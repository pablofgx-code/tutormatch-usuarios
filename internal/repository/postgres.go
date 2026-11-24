package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tutormatch/usuarios/internal/domain"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

type rowScanner interface {
	Scan(dest ...any) error
}

// escanearUsuario centraliza (DRY) la lectura de columnas de la tabla usuarios.
func escanearUsuario(s rowScanner) (*domain.Usuario, error) {
	u := &domain.Usuario{}
	if err := s.Scan(&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.EsTutor, &u.CreadoEn); err != nil {
		return nil, err
	}
	return u, nil
}

func (r *PostgresRepository) CrearUsuario(ctx context.Context, u *domain.Usuario) error {
	query := `
		INSERT INTO usuarios (id, nombre, email, password_hash, es_tutor, creado_en)
		VALUES (gen_random_uuid(), $1, $2, $3, false, $4)
		RETURNING id, es_tutor
	`
	emailNormalizado := domain.NormalizarEmail(u.Email)

	err := r.db.QueryRowContext(ctx, query, u.Nombre, emailNormalizado, u.PasswordHash, u.CreadoEn).Scan(&u.ID, &u.EsTutor)
	if err != nil {
		if esErrorDuplicado(err) {
			return domain.ErrYaExiste
		}
		return fmt.Errorf("error al crear usuario en db: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ObtenerPorEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	query := `
		SELECT id, nombre, email, password_hash, es_tutor, creado_en
		FROM usuarios
		WHERE lower(email) = lower($1)
	`
	u, err := escanearUsuario(r.db.QueryRowContext(ctx, query, domain.NormalizarEmail(email)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNoEncontrado
		}
		return nil, fmt.Errorf("error al obtener usuario por email: %w", err)
	}

	ramos, err := r.obtenerRamosDeUsuario(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	u.Ramos = ramos

	return u, nil
}

func (r *PostgresRepository) ObtenerPorID(ctx context.Context, id string) (*domain.Usuario, error) {
	query := `
		SELECT id, nombre, email, password_hash, es_tutor, creado_en
		FROM usuarios
		WHERE id = $1
	`
	u, err := escanearUsuario(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || esErrorUUIDInvalido(err) {
			return nil, domain.ErrNoEncontrado
		}
		return nil, fmt.Errorf("error al obtener usuario por id: %w", err)
	}

	ramos, err := r.obtenerRamosDeUsuario(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	u.Ramos = ramos

	return u, nil
}

// ObtenerPorIDs evita el problema N+1: 1 consulta de usuarios + 1 consulta en lote de ramos (O(N+M)).
func (r *PostgresRepository) ObtenerPorIDs(ctx context.Context, ids []string) ([]*domain.Usuario, error) {
	if len(ids) == 0 {
		return []*domain.Usuario{}, nil
	}

	query := `
		SELECT id, nombre, email, password_hash, es_tutor, creado_en
		FROM usuarios
		WHERE id = ANY($1)
	`
	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("error al consultar usuarios en lote: %w", err)
	}
	defer rows.Close()

	usuarios := make([]*domain.Usuario, 0, len(ids))
	for rows.Next() {
		u, err := escanearUsuario(rows)
		if err != nil {
			return nil, err
		}
		usuarios = append(usuarios, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando usuarios en lote: %w", err)
	}
	// Cerramos rows antes de la segunda consulta para devolver la conexión al pool de inmediato.
	rows.Close()

	if err := r.cargarRamosEnLote(ctx, usuarios); err != nil {
		return nil, err
	}

	return usuarios, nil
}

func (r *PostgresRepository) AgregarRamosTutorTx(ctx context.Context, usuarioID string, ramos []domain.InscripcionRamoInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queryInsert := `
		INSERT INTO tutor_ramos (usuario_id, ramo_codigo, nota_aprobacion, periodo_aprobacion, activo)
		VALUES ($1, $2, $3, $4, true)
	`
	for _, rm := range ramos {
		_, err := tx.ExecContext(ctx, queryInsert, usuarioID, rm.RamoCodigo, rm.NotaAprobacion, rm.PeriodoAprobacion)
		if err != nil {
			if esErrorDuplicado(err) {
				return domain.ErrYaExiste
			}
			if esErrorLlaveForanea(err) || esErrorUUIDInvalido(err) {
				return domain.ErrNoEncontrado
			}
			return fmt.Errorf("error al inscribir ramo %s: %w", rm.RamoCodigo, err)
		}
	}

	if err := r.recalcularEsTutor(ctx, tx, usuarioID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresRepository) CambiarEstadoRamoTx(ctx context.Context, usuarioID, ramoCodigo string, activo bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE tutor_ramos SET activo = $3 WHERE usuario_id = $1 AND ramo_codigo = $2`,
		usuarioID, ramoCodigo, activo,
	)
	if err != nil {
		if esErrorUUIDInvalido(err) {
			return domain.ErrNoEncontrado
		}
		return fmt.Errorf("error al cambiar estado del ramo: %w", err)
	}

	filas, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if filas == 0 {
		return domain.ErrNoEncontrado
	}

	if err := r.recalcularEsTutor(ctx, tx, usuarioID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PostgresRepository) ListarRamos(ctx context.Context) ([]domain.Ramo, error) {
	query := `SELECT codigo, nombre FROM ramos ORDER BY codigo ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error al listar ramos: %w", err)
	}
	defer rows.Close()

	var ramos []domain.Ramo
	for rows.Next() {
		var rm domain.Ramo
		if err := rows.Scan(&rm.Codigo, &rm.Nombre); err != nil {
			return nil, err
		}
		ramos = append(ramos, rm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando el catálogo de ramos: %w", err)
	}

	return ramos, nil
}

// ListarTutoresPorRamo resuelve todos los tutores y sus ramos en 2 consultas fijas (sin N+1).
func (r *PostgresRepository) ListarTutoresPorRamo(ctx context.Context, ramoCodigo string) ([]*domain.Usuario, error) {
	query := `
		SELECT u.id, u.nombre, u.email, u.password_hash, u.es_tutor, u.creado_en
		FROM usuarios u
		JOIN tutor_ramos tr ON u.id = tr.usuario_id
		WHERE tr.ramo_codigo = $1 AND tr.activo = true AND u.es_tutor = true
	`
	rows, err := r.db.QueryContext(ctx, query, ramoCodigo)
	if err != nil {
		return nil, fmt.Errorf("error al listar tutores por ramo: %w", err)
	}
	defer rows.Close()

	var tutores []*domain.Usuario
	for rows.Next() {
		u, err := escanearUsuario(rows)
		if err != nil {
			return nil, err
		}
		tutores = append(tutores, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando tutores por ramo: %w", err)
	}
	rows.Close()

	if err := r.cargarRamosEnLote(ctx, tutores); err != nil {
		return nil, err
	}

	return tutores, nil
}

// ValidarTutorRamo verifica en una sola consulta SQL gracias a la FK tutor_ramos.usuario_id -> usuarios.id.
func (r *PostgresRepository) ValidarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, error) {
	var activo bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM tutor_ramos WHERE usuario_id = $1 AND ramo_codigo = $2 AND activo = true)`,
		tutorID, ramoCodigo,
	).Scan(&activo)
	if err != nil {
		if esErrorUUIDInvalido(err) {
			return false, nil
		}
		return false, err
	}

	return activo, nil
}

// recalcularEsTutor aplica las reglas 2 y 3 dentro de la misma transacción.
func (r *PostgresRepository) recalcularEsTutor(ctx context.Context, tx *sql.Tx, usuarioID string) error {
	query := `
		UPDATE usuarios
		SET es_tutor = EXISTS (
			SELECT 1 FROM tutor_ramos WHERE usuario_id = $1 AND activo = true
		)
		WHERE id = $1
	`
	_, err := tx.ExecContext(ctx, query, usuarioID)
	return err
}

func (r *PostgresRepository) obtenerRamosDeUsuario(ctx context.Context, usuarioID string) ([]domain.RamoTutor, error) {
	query := `
		SELECT r.codigo, r.nombre, tr.nota_aprobacion, tr.periodo_aprobacion, tr.activo, tr.verificado_en
		FROM tutor_ramos tr
		JOIN ramos r ON r.codigo = tr.ramo_codigo
		WHERE tr.usuario_id = $1
		ORDER BY r.codigo ASC
	`
	rows, err := r.db.QueryContext(ctx, query, usuarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ramos []domain.RamoTutor
	for rows.Next() {
		var rt domain.RamoTutor
		if err := rows.Scan(
			&rt.Ramo.Codigo,
			&rt.Ramo.Nombre,
			&rt.NotaAprobacion,
			&rt.PeriodoAprobacion,
			&rt.Activo,
			&rt.VerificadoEn,
		); err != nil {
			return nil, err
		}
		ramos = append(ramos, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando ramos del usuario: %w", err)
	}

	return ramos, nil
}

// cargarRamosEnLote trae los ramos de un conjunto de usuarios en una única query y los cruza en O(1) por fila.
func (r *PostgresRepository) cargarRamosEnLote(ctx context.Context, usuarios []*domain.Usuario) error {
	if len(usuarios) == 0 {
		return nil
	}

	ids := make([]string, 0, len(usuarios))
	porID := make(map[string]*domain.Usuario, len(usuarios))
	for _, u := range usuarios {
		ids = append(ids, u.ID)
		porID[u.ID] = u
	}

	query := `
		SELECT tr.usuario_id, r.codigo, r.nombre, tr.nota_aprobacion, tr.periodo_aprobacion, tr.activo, tr.verificado_en
		FROM tutor_ramos tr
		JOIN ramos r ON r.codigo = tr.ramo_codigo
		WHERE tr.usuario_id = ANY($1)
		ORDER BY tr.usuario_id ASC, r.codigo ASC
	`
	rows, err := r.db.QueryContext(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("error cargando ramos en lote: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var usuarioID string
		var rt domain.RamoTutor
		if err := rows.Scan(
			&usuarioID,
			&rt.Ramo.Codigo,
			&rt.Ramo.Nombre,
			&rt.NotaAprobacion,
			&rt.PeriodoAprobacion,
			&rt.Activo,
			&rt.VerificadoEn,
		); err != nil {
			return err
		}
		if u, ok := porID[usuarioID]; ok {
			u.Ramos = append(u.Ramos, rt)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterando ramos en lote: %w", err)
	}

	return nil
}

func esErrorDuplicado(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

func esErrorLlaveForanea(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "23503") || strings.Contains(msg, "foreign key constraint")
}

func esErrorUUIDInvalido(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "22P02") || strings.Contains(msg, "invalid input syntax for type uuid")
}
