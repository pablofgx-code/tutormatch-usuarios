package service

import (
	"context"
	"errors"
	"testing"

	"github.com/tutormatch/usuarios/internal/domain"
)

const testUUID = "11111111-1111-4111-8111-111111111111"

type mockRepo struct {
	usuarios map[string]*domain.Usuario
}

func newMockRepo() *mockRepo {
	return &mockRepo{usuarios: make(map[string]*domain.Usuario)}
}

func (m *mockRepo) CrearUsuario(_ context.Context, u *domain.Usuario) error {
	for _, existente := range m.usuarios {
		if existente.Email == domain.NormalizarEmail(u.Email) {
			return domain.ErrYaExiste
		}
	}
	u.ID = testUUID
	u.Email = domain.NormalizarEmail(u.Email)
	m.usuarios[u.ID] = u
	return nil
}

func (m *mockRepo) ObtenerPorEmail(_ context.Context, email string) (*domain.Usuario, error) {
	for _, u := range m.usuarios {
		if u.Email == domain.NormalizarEmail(email) {
			return u, nil
		}
	}
	return nil, domain.ErrNoEncontrado
}

func (m *mockRepo) ObtenerPorID(_ context.Context, id string) (*domain.Usuario, error) {
	u, ok := m.usuarios[id]
	if !ok {
		return nil, domain.ErrNoEncontrado
	}
	return u, nil
}

func (m *mockRepo) ObtenerPorIDs(_ context.Context, ids []string) ([]*domain.Usuario, error) {
	var res []*domain.Usuario
	for _, id := range ids {
		if u, ok := m.usuarios[id]; ok {
			res = append(res, u)
		}
	}
	return res, nil
}

func (m *mockRepo) AgregarRamosTutorTx(_ context.Context, usuarioID string, ramos []domain.InscripcionRamoInput) error {
	u, ok := m.usuarios[usuarioID]
	if !ok {
		return domain.ErrNoEncontrado
	}
	for _, in := range ramos {
		u.Ramos = append(u.Ramos, domain.RamoTutor{
			Ramo:              domain.Ramo{Codigo: in.RamoCodigo, Nombre: "Ramo Mock"},
			NotaAprobacion:    in.NotaAprobacion,
			PeriodoAprobacion: in.PeriodoAprobacion,
			Activo:            true,
		})
	}
	u.EsTutor = true
	return nil
}

func (m *mockRepo) CambiarEstadoRamoTx(_ context.Context, usuarioID, ramoCodigo string, activo bool) error {
	u, ok := m.usuarios[usuarioID]
	if !ok {
		return domain.ErrNoEncontrado
	}
	encontrado := false
	tieneActivos := false
	for i := range u.Ramos {
		if u.Ramos[i].Ramo.Codigo == ramoCodigo {
			u.Ramos[i].Activo = activo
			encontrado = true
		}
		if u.Ramos[i].Activo {
			tieneActivos = true
		}
	}
	if !encontrado {
		return domain.ErrNoEncontrado
	}
	u.EsTutor = tieneActivos
	return nil
}

func (m *mockRepo) ListarRamos(_ context.Context) ([]domain.Ramo, error) {
	return []domain.Ramo{{Codigo: "MAT1610", Nombre: "Cálculo I"}}, nil
}

func (m *mockRepo) ListarTutoresPorRamo(_ context.Context, ramoCodigo string) ([]*domain.Usuario, error) {
	var res []*domain.Usuario
	for _, u := range m.usuarios {
		for _, r := range u.Ramos {
			if r.Ramo.Codigo == ramoCodigo && r.Activo && u.EsTutor {
				res = append(res, u)
			}
		}
	}
	return res, nil
}

func (m *mockRepo) ValidarTutorRamo(_ context.Context, tutorID, ramoCodigo string) (bool, bool, error) {
	u, ok := m.usuarios[tutorID]
	if !ok {
		return false, false, nil
	}
	for _, r := range u.Ramos {
		if r.Ramo.Codigo == ramoCodigo && r.Activo {
			return true, true, nil
		}
	}
	return true, false, nil
}

type mockHasher struct{}

func (h *mockHasher) Hash(pw string) (string, error) { return "hashed_" + pw, nil }
func (h *mockHasher) Compare(hash, pw string) bool   { return hash == "hashed_"+pw }

type mockToken struct{}

func (t *mockToken) GenerateToken(id string) (string, error) { return "jwt_para_" + id, nil }

func TestRegistroYLogin(t *testing.T) {
	svc := NewUsuarioService(newMockRepo(), &mockHasher{}, &mockToken{})
	ctx := context.Background()

	// Email inválido debe rechazarse
	_, _, err := svc.RegistroUsuario(ctx, "Pablo", "sin-arroba", "123456")
	if !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("se esperaba ErrArgumentoInvalido por email inválido, obtuve %v", err)
	}

	// Registro exitoso
	tok, u, err := svc.RegistroUsuario(ctx, "Pablo", "PABLO@ucn.cl", "123456")
	if err != nil || tok == "" || u.Email != "pablo@ucn.cl" {
		t.Fatalf("se esperaba registro exitoso, obtuve err=%v", err)
	}

	// Email duplicado
	_, _, err = svc.RegistroUsuario(ctx, "Otro", "pablo@ucn.cl", "123456")
	if !errors.Is(err, domain.ErrYaExiste) {
		t.Fatalf("se esperaba ErrYaExiste, obtuve %v", err)
	}

	// Login exitoso, contraseña errónea, usuario inexistente y datos vacíos
	if _, _, err = svc.Login(ctx, "pablo@ucn.cl", "123456"); err != nil {
		t.Fatalf("se esperaba login exitoso, obtuve %v", err)
	}
	if _, _, err = svc.Login(ctx, "pablo@ucn.cl", "mala_clave"); !errors.Is(err, domain.ErrCredenciales) {
		t.Fatalf("se esperaba ErrCredenciales, obtuve %v", err)
	}
	if _, _, err = svc.Login(ctx, "nadie@ucn.cl", "123456"); !errors.Is(err, domain.ErrCredenciales) {
		t.Fatalf("se esperaba ErrCredenciales para email inexistente, obtuve %v", err)
	}
	if _, _, err = svc.Login(ctx, "", ""); !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("se esperaba ErrArgumentoInvalido en login vacío, obtuve %v", err)
	}
}

func TestFlujoRamosTutor(t *testing.T) {
	svc := NewUsuarioService(newMockRepo(), &mockHasher{}, &mockToken{})
	ctx := context.Background()

	_, u, _ := svc.RegistroUsuario(ctx, "Pablo", "pablo@ucn.cl", "123456")

	// Nota fuera de escala chilena
	_, err := svc.AgregarRamosTutor(ctx, u.ID, []domain.InscripcionRamoInput{
		{RamoCodigo: "MAT1610", NotaAprobacion: 3.9, PeriodoAprobacion: "2025-1"},
	})
	if !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("se esperaba ErrArgumentoInvalido, obtuve %v", err)
	}

	// Ramos repetidos en el mismo request
	_, err = svc.AgregarRamosTutor(ctx, u.ID, []domain.InscripcionRamoInput{
		{RamoCodigo: "MAT1610", NotaAprobacion: 6.0, PeriodoAprobacion: "2025-1"},
		{RamoCodigo: "MAT1610", NotaAprobacion: 6.0, PeriodoAprobacion: "2025-1"},
	})
	if !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("se esperaba ErrArgumentoInvalido por duplicados, obtuve %v", err)
	}

	// UUID mal formado devuelve ErrNoEncontrado
	_, err = svc.AgregarRamosTutor(ctx, "no-es-uuid", []domain.InscripcionRamoInput{
		{RamoCodigo: "MAT1610", NotaAprobacion: 6.0, PeriodoAprobacion: "2025-1"},
	})
	if !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("se esperaba ErrNoEncontrado para UUID inválido, obtuve %v", err)
	}

	// Inscripción válida
	uActualizado, err := svc.AgregarRamosTutor(ctx, u.ID, []domain.InscripcionRamoInput{
		{RamoCodigo: "MAT1610", NotaAprobacion: 6.5, PeriodoAprobacion: "2025-1"},
	})
	if err != nil || !uActualizado.EsTutor {
		t.Fatalf("se esperaba EsTutor=true, obtuve err=%v", err)
	}

	// Quitar y reactivar
	uSinRamos, err := svc.QuitarRamoTutor(ctx, u.ID, "MAT1610")
	if err != nil || uSinRamos.EsTutor {
		t.Fatalf("se esperaba EsTutor=false al quitar ramo, obtuve err=%v", err)
	}
	uReactivado, err := svc.ReactivarRamoTutor(ctx, u.ID, "MAT1610")
	if err != nil || !uReactivado.EsTutor {
		t.Fatalf("se esperaba EsTutor=true al reactivar ramo, obtuve err=%v", err)
	}

	// Errores de validación en quitar/reactivar
	if _, err = svc.QuitarRamoTutor(ctx, "", "MAT1610"); !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("esperaba ErrArgumentoInvalido, obtuve %v", err)
	}
	if _, err = svc.QuitarRamoTutor(ctx, "mal-id", "MAT1610"); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado, obtuve %v", err)
	}
	if _, err = svc.ReactivarRamoTutor(ctx, "", "MAT1610"); !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("esperaba ErrArgumentoInvalido, obtuve %v", err)
	}
	if _, err = svc.ReactivarRamoTutor(ctx, "mal-id", "MAT1610"); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado, obtuve %v", err)
	}
}

func TestConsultasYValidaciones(t *testing.T) {
	svc := NewUsuarioService(newMockRepo(), &mockHasher{}, &mockToken{})
	ctx := context.Background()

	_, u, _ := svc.RegistroUsuario(ctx, "Pablo", "pablo@ucn.cl", "123456")
	_, _ = svc.AgregarRamosTutor(ctx, u.ID, []domain.InscripcionRamoInput{
		{RamoCodigo: "MAT1610", NotaAprobacion: 6.0, PeriodoAprobacion: "2025-1"},
	})

	// ObtenerUsuario: válido, vacío y mal formado
	if obtenido, err := svc.ObtenerUsuario(ctx, u.ID); err != nil || obtenido.ID != u.ID {
		t.Fatalf("error en ObtenerUsuario: %v", err)
	}
	if _, err := svc.ObtenerUsuario(ctx, ""); !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("esperaba ErrArgumentoInvalido, obtuve %v", err)
	}
	if _, err := svc.ObtenerUsuario(ctx, "no-es-uuid"); !errors.Is(err, domain.ErrNoEncontrado) {
		t.Fatalf("esperaba ErrNoEncontrado, obtuve %v", err)
	}

	// ObtenerUsuarios: omite IDs inválidos e inexistentes sin fallar
	lista, err := svc.ObtenerUsuarios(ctx, []string{u.ID, "id-inexistente"})
	if err != nil || len(lista) != 1 {
		t.Fatalf("se esperaba 1 usuario en lote, obtuve %d (err=%v)", len(lista), err)
	}
	vacia, err := svc.ObtenerUsuarios(ctx, []string{"no-es-uuid"})
	if err != nil || len(vacia) != 0 {
		t.Fatalf("se esperaba lista vacía sin error, obtuve %d (err=%v)", len(vacia), err)
	}

	// ListarRamos y ListarTutoresPorRamo
	if ramos, err := svc.ListarRamos(ctx); err != nil || len(ramos) == 0 {
		t.Fatalf("se esperaban ramos en el catálogo, err=%v", err)
	}
	if tutores, err := svc.ListarTutoresPorRamo(ctx, "MAT1610"); err != nil || len(tutores) != 1 {
		t.Fatalf("se esperaba 1 tutor para MAT1610, obtuve %d (err=%v)", len(tutores), err)
	}
	if _, err := svc.ListarTutoresPorRamo(ctx, ""); !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("esperaba ErrArgumentoInvalido, obtuve %v", err)
	}

	// ValidarTutorRamo: válido, otro ramo, UUID inválido y vacío
	if valido, err := svc.ValidarTutorRamo(ctx, u.ID, "MAT1610"); err != nil || !valido {
		t.Fatalf("se esperaba tutor válido, err=%v", err)
	}
	if invalido, err := svc.ValidarTutorRamo(ctx, u.ID, "FIS1514"); err != nil || invalido {
		t.Fatalf("se esperaba false para ramo no inscrito, err=%v", err)
	}
	if invalido, err := svc.ValidarTutorRamo(ctx, "no-es-uuid", "MAT1610"); err != nil || invalido {
		t.Fatalf("se esperaba false sin error para UUID inválido, err=%v", err)
	}
	if _, err := svc.ValidarTutorRamo(ctx, "", "MAT1610"); !errors.Is(err, domain.ErrArgumentoInvalido) {
		t.Fatalf("esperaba ErrArgumentoInvalido, obtuve %v", err)
	}
}
