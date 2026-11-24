package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tutormatch/usuarios/internal/domain"
)

// Driver SQL liviano en memoria para probar PostgresRepository sin requerir Docker.
type fakeHandlers struct {
	onQuery func(query string, args []driver.NamedValue) (driver.Rows, error)
	onExec  func(query string, args []driver.NamedValue) (driver.Result, error)
}

var (
	fakeMu       sync.RWMutex
	fakeRegistry = map[string]*fakeHandlers{}
)

func init() {
	sql.Register("fake_pg_usuarios", &fakeDriver{})
}

type fakeDriver struct{}

func (d *fakeDriver) Open(dsn string) (driver.Conn, error) {
	fakeMu.RLock()
	h := fakeRegistry[dsn]
	fakeMu.RUnlock()
	return &fakeConn{h: h}, nil
}

type fakeConn struct{ h *fakeHandlers }

func (c *fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("no soportado") }
func (c *fakeConn) Close() error                        { return nil }
func (c *fakeConn) Begin() (driver.Tx, error)           { return &fakeTx{}, nil }
func (c *fakeConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return &fakeTx{}, nil
}
func (c *fakeConn) CheckNamedValue(*driver.NamedValue) error { return nil }

func (c *fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.h != nil && c.h.onQuery != nil {
		return c.h.onQuery(query, args)
	}
	return &fakeRows{}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.h != nil && c.h.onExec != nil {
		return c.h.onExec(query, args)
	}
	return driver.RowsAffected(1), nil
}

type fakeTx struct{}

func (t *fakeTx) Commit() error   { return nil }
func (t *fakeTx) Rollback() error { return nil }

type fakeRows struct {
	cols []string
	data [][]driver.Value
	idx  int
}

func (r *fakeRows) Columns() []string { return r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.idx])
	r.idx++
	return nil
}

func abrirRepoFake(t *testing.T, h *fakeHandlers) *PostgresRepository {
	t.Helper()
	dsn := t.Name()
	fakeMu.Lock()
	fakeRegistry[dsn] = h
	fakeMu.Unlock()

	db, err := sql.Open("fake_pg_usuarios", dsn)
	if err != nil {
		t.Fatalf("error abriendo db fake: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewPostgresRepository(db)
}

func TestPostgresRepository_CrearYObtenerSinNMasUno(t *testing.T) {
	ahora := time.Now().UTC()
	const uid1 = "11111111-1111-4111-8111-111111111111"
	const uid2 = "22222222-2222-4222-8222-222222222222"
	consultasLoteRamos := 0

	repo := abrirRepoFake(t, &fakeHandlers{
		onQuery: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			switch {
			case strings.Contains(query, "INSERT INTO usuarios"):
				if args[1].Value == "duplicado@ucn.cl" {
					return nil, errors.New("SQLSTATE 23505 duplicate key")
				}
				return &fakeRows{
					cols: []string{"id", "es_tutor"},
					data: [][]driver.Value{{uid1, false}},
				}, nil
			case strings.Contains(query, "WHERE tr.usuario_id = ANY($1)"):
				consultasLoteRamos++
				return &fakeRows{
					cols: []string{"usuario_id", "codigo", "nombre", "nota", "periodo", "activo", "verificado_en"},
					data: [][]driver.Value{
						{uid1, "MAT1610", "Cálculo I", 6.5, "2025-2", true, ahora},
						{uid2, "FIS1514", "Dinámica", 5.8, "2025-1", true, ahora},
					},
				}, nil
			case strings.Contains(query, "WHERE tr.usuario_id = $1"):
				return &fakeRows{
					cols: []string{"codigo", "nombre", "nota", "periodo", "activo", "verificado_en"},
					data: [][]driver.Value{{"MAT1610", "Cálculo I", 6.5, "2025-2", true, ahora}},
				}, nil
			case strings.Contains(query, "WHERE lower(email)"):
				if args[0].Value == "nadie@ucn.cl" {
					return &fakeRows{cols: []string{"id", "nombre", "email", "password_hash", "es_tutor", "creado_en"}}, nil
				}
				return &fakeRows{
					cols: []string{"id", "nombre", "email", "password_hash", "es_tutor", "creado_en"},
					data: [][]driver.Value{{uid1, "Ana", "ana@ucn.cl", "hash", true, ahora}},
				}, nil
			case strings.Contains(query, "WHERE id = $1"):
				if args[0].Value == "mal-uuid" {
					return nil, errors.New("SQLSTATE 22P02 invalid input syntax for type uuid")
				}
				return &fakeRows{
					cols: []string{"id", "nombre", "email", "password_hash", "es_tutor", "creado_en"},
					data: [][]driver.Value{{uid1, "Ana", "ana@ucn.cl", "hash", true, ahora}},
				}, nil
			case strings.Contains(query, "WHERE id = ANY($1)"):
				return &fakeRows{
					cols: []string{"id", "nombre", "email", "password_hash", "es_tutor", "creado_en"},
					data: [][]driver.Value{
						{uid1, "Ana", "ana@ucn.cl", "hash", true, ahora},
						{uid2, "Beto", "beto@ucn.cl", "hash", true, ahora},
					},
				}, nil
			}
			return &fakeRows{}, nil
		},
	})

	ctx := context.Background()
	u := &domain.Usuario{Nombre: "Ana", Email: "ana@ucn.cl", PasswordHash: "hash", CreadoEn: ahora}
	if err := repo.CrearUsuario(ctx, u); err != nil || u.ID != uid1 {
		t.Fatalf("CrearUsuario falló: id=%q err=%v", u.ID, err)
	}
	if err := repo.CrearUsuario(ctx, &domain.Usuario{Email: "duplicado@ucn.cl"}); !errors.Is(err, domain.ErrYaExiste) {
		t.Fatalf("esperaba ErrYaExiste, obtuve %v", err)
	}

	obtenido, err := repo.ObtenerPorEmail(ctx, "ana@ucn.cl")
	if err != nil || len(obtenido.Ramos) != 1 {
		t.Fatalf("ObtenerPorEmail falló: %+v err=%v", obtenido, err)
	}
	if _, err := repo.ObtenerPorEmail(ctx, "nadie@ucn.cl"); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado, obtuve %v", err)
	}

	if _, err := repo.ObtenerPorID(ctx, uid1); err != nil {
		t.Fatalf("ObtenerPorID falló: %v", err)
	}
	if _, err := repo.ObtenerPorID(ctx, "mal-uuid"); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado para 22P02, obtuve %v", err)
	}

	if vacio, err := repo.ObtenerPorIDs(ctx, nil); err != nil || len(vacio) != 0 {
		t.Fatalf("ObtenerPorIDs vacío falló: %v", err)
	}

	// Verifica que 2 usuarios cargan sus ramos con UNA sola consulta en lote (sin N+1)
	lote, err := repo.ObtenerPorIDs(ctx, []string{uid1, uid2})
	if err != nil || len(lote) != 2 || len(lote[0].Ramos) != 1 || len(lote[1].Ramos) != 1 || consultasLoteRamos != 1 {
		t.Fatalf("ObtenerPorIDs lote inesperado: len=%d consultasRamos=%d err=%v", len(lote), consultasLoteRamos, err)
	}
}

func TestPostgresRepository_RamosYValidacionUnicaQuery(t *testing.T) {
	ahora := time.Now().UTC()
	const uid = "11111111-1111-4111-8111-111111111111"

	repo := abrirRepoFake(t, &fakeHandlers{
		onExec: func(query string, args []driver.NamedValue) (driver.Result, error) {
			switch {
			case strings.Contains(query, "INSERT INTO tutor_ramos"):
				if args[1].Value == "DUP" {
					return nil, errors.New("23505 unique constraint")
				}
				if args[1].Value == "NOEXISTE" {
					return nil, errors.New("23503 foreign key constraint")
				}
				return driver.RowsAffected(1), nil
			case strings.Contains(query, "UPDATE tutor_ramos"):
				if args[1].Value == "NOINSCRITO" {
					return driver.RowsAffected(0), nil
				}
				return driver.RowsAffected(1), nil
			default:
				return driver.RowsAffected(1), nil
			}
		},
		onQuery: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			switch {
			case strings.Contains(query, "FROM ramos ORDER BY"):
				return &fakeRows{
					cols: []string{"codigo", "nombre"},
					data: [][]driver.Value{{"MAT1610", "Cálculo I"}},
				}, nil
			case strings.Contains(query, "JOIN tutor_ramos tr ON u.id"):
				return &fakeRows{
					cols: []string{"id", "nombre", "email", "password_hash", "es_tutor", "creado_en"},
					data: [][]driver.Value{{uid, "Ana", "ana@ucn.cl", "hash", true, ahora}},
				}, nil
			case strings.Contains(query, "WHERE tr.usuario_id = ANY($1)"):
				return &fakeRows{
					cols: []string{"usuario_id", "codigo", "nombre", "nota", "periodo", "activo", "verificado_en"},
					data: [][]driver.Value{{uid, "MAT1610", "Cálculo I", 6.5, "2025-2", true, ahora}},
				}, nil
			case strings.Contains(query, "FROM tutor_ramos WHERE usuario_id = $1"):
				if args[0].Value == "no-existe" {
					return &fakeRows{cols: []string{"exists"}, data: [][]driver.Value{{false}}}, nil
				}
				if args[0].Value == "mal-uuid" {
					return nil, errors.New("22P02")
				}
				return &fakeRows{cols: []string{"exists"}, data: [][]driver.Value{{true}}}, nil
			}
			return &fakeRows{}, nil
		},
	})

	ctx := context.Background()

	if err := repo.AgregarRamosTutorTx(ctx, uid, []domain.InscripcionRamoInput{
		{RamoCodigo: "MAT1610", NotaAprobacion: 6.5, PeriodoAprobacion: "2025-2"},
	}); err != nil {
		t.Fatalf("AgregarRamosTutorTx falló: %v", err)
	}
	if err := repo.AgregarRamosTutorTx(ctx, uid, []domain.InscripcionRamoInput{{RamoCodigo: "DUP"}}); !errors.Is(err, domain.ErrYaExiste) {
		t.Fatalf("esperaba ErrYaExiste, obtuve %v", err)
	}
	if err := repo.AgregarRamosTutorTx(ctx, uid, []domain.InscripcionRamoInput{{RamoCodigo: "NOEXISTE"}}); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado, obtuve %v", err)
	}

	if err := repo.CambiarEstadoRamoTx(ctx, uid, "MAT1610", false); err != nil {
		t.Fatalf("CambiarEstadoRamoTx falló: %v", err)
	}
	if err := repo.CambiarEstadoRamoTx(ctx, uid, "NOINSCRITO", false); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado, obtuve %v", err)
	}

	if ramos, err := repo.ListarRamos(ctx); err != nil || len(ramos) != 1 {
		t.Fatalf("ListarRamos falló: %v", err)
	}
	if tutores, err := repo.ListarTutoresPorRamo(ctx, "MAT1610"); err != nil || len(tutores) != 1 || len(tutores[0].Ramos) != 1 {
		t.Fatalf("ListarTutoresPorRamo falló: %+v err=%v", tutores, err)
	}

	if activo, err := repo.ValidarTutorRamo(ctx, uid, "MAT1610"); err != nil || !activo {
		t.Fatalf("ValidarTutorRamo esperado true, obtuve %v err=%v", activo, err)
	}
	if activo, err := repo.ValidarTutorRamo(ctx, "no-existe", "MAT1610"); err != nil || activo {
		t.Fatalf("ValidarTutorRamo esperado false, obtuve %v err=%v", activo, err)
	}
	if activo, err := repo.ValidarTutorRamo(ctx, "mal-uuid", "MAT1610"); err != nil || activo {
		t.Fatalf("ValidarTutorRamo para 22P02 esperado false, obtuve %v err=%v", activo, err)
	}
}

type stubCatalogoRepo struct {
	domain.UsuarioRepository
	llamadas int
	err      error
}

func (s *stubCatalogoRepo) ListarRamos(_ context.Context) ([]domain.Ramo, error) {
	s.llamadas++
	if s.err != nil {
		return nil, s.err
	}
	return []domain.Ramo{{Codigo: "MAT1610", Nombre: "Cálculo I"}}, nil
}

func TestCachedCatalogoRepository_HitTTLYError(t *testing.T) {
	ahora := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	base := &stubCatalogoRepo{}
	cached := NewCachedCatalogoRepository(base, 5*time.Minute)
	cached.now = func() time.Time { return ahora }

	ctx := context.Background()
	if _, err := cached.ListarRamos(ctx); err != nil {
		t.Fatalf("primera llamada falló: %v", err)
	}
	if _, err := cached.ListarRamos(ctx); err != nil {
		t.Fatalf("segunda llamada falló: %v", err)
	}
	if base.llamadas != 1 {
		t.Fatalf("esperaba 1 consulta (cache hit), obtuve %d", base.llamadas)
	}

	ahora = ahora.Add(6 * time.Minute)
	if _, err := cached.ListarRamos(ctx); err != nil {
		t.Fatalf("recarga tras TTL falló: %v", err)
	}
	if base.llamadas != 2 {
		t.Fatalf("esperaba 2 consultas tras expirar TTL, obtuve %d", base.llamadas)
	}

	baseErr := &stubCatalogoRepo{err: errors.New("db caída")}
	cachedErr := NewCachedCatalogoRepository(baseErr, 0)
	if _, err := cachedErr.ListarRamos(ctx); err == nil {
		t.Fatal("se esperaba error propagado")
	}
}
