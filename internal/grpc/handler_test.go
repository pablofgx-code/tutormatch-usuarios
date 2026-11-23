package grpc

import (
	"context"
	"errors"
	"testing"

	usuariosv1 "github.com/tutormatch/usuarios/gen/usuariosv1/usuarios/v1"
	"github.com/tutormatch/usuarios/internal/domain"
	"github.com/tutormatch/usuarios/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testUsuarioID = "11111111-1111-4111-8111-111111111111"

type stubRepo struct {
	usuarios map[string]*domain.Usuario
}

func newStubRepo() *stubRepo {
	return &stubRepo{usuarios: make(map[string]*domain.Usuario)}
}

func (m *stubRepo) CrearUsuario(_ context.Context, u *domain.Usuario) error {
	for _, ex := range m.usuarios {
		if ex.Email == domain.NormalizarEmail(u.Email) {
			return domain.ErrYaExiste
		}
	}
	u.ID = testUsuarioID
	u.Email = domain.NormalizarEmail(u.Email)
	m.usuarios[u.ID] = u
	return nil
}

func (m *stubRepo) ObtenerPorEmail(_ context.Context, email string) (*domain.Usuario, error) {
	for _, u := range m.usuarios {
		if u.Email == domain.NormalizarEmail(email) {
			return u, nil
		}
	}
	return nil, domain.ErrNoEncontrado
}

func (m *stubRepo) ObtenerPorID(_ context.Context, id string) (*domain.Usuario, error) {
	u, ok := m.usuarios[id]
	if !ok {
		return nil, domain.ErrNoEncontrado
	}
	return u, nil
}

func (m *stubRepo) ObtenerPorIDs(_ context.Context, ids []string) ([]*domain.Usuario, error) {
	var res []*domain.Usuario
	for _, id := range ids {
		if u, ok := m.usuarios[id]; ok {
			res = append(res, u)
		}
	}
	return res, nil
}

func (m *stubRepo) AgregarRamosTutorTx(_ context.Context, usuarioID string, ramos []domain.InscripcionRamoInput) error {
	u, ok := m.usuarios[usuarioID]
	if !ok {
		return domain.ErrNoEncontrado
	}
	for _, r := range ramos {
		u.Ramos = append(u.Ramos, domain.RamoTutor{
			Ramo:   domain.Ramo{Codigo: r.RamoCodigo, Nombre: "Cálculo I"},
			Activo: true,
		})
	}
	u.EsTutor = true
	return nil
}

func (m *stubRepo) CambiarEstadoRamoTx(_ context.Context, usuarioID, ramoCodigo string, activo bool) error {
	u, ok := m.usuarios[usuarioID]
	if !ok {
		return domain.ErrNoEncontrado
	}
	encontrado := false
	activos := false
	for i := range u.Ramos {
		if u.Ramos[i].Ramo.Codigo == ramoCodigo {
			u.Ramos[i].Activo = activo
			encontrado = true
		}
		if u.Ramos[i].Activo {
			activos = true
		}
	}
	if !encontrado {
		return domain.ErrNoEncontrado
	}
	u.EsTutor = activos
	return nil
}

func (m *stubRepo) ListarRamos(_ context.Context) ([]domain.Ramo, error) {
	return []domain.Ramo{{Codigo: "MAT1610", Nombre: "Cálculo I"}}, nil
}

func (m *stubRepo) ListarTutoresPorRamo(_ context.Context, ramoCodigo string) ([]*domain.Usuario, error) {
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

func (m *stubRepo) ValidarTutorRamo(_ context.Context, tutorID, ramoCodigo string) (bool, bool, error) {
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

type stubHasher struct{}

func (stubHasher) Hash(pw string) (string, error) { return "h:" + pw, nil }
func (stubHasher) Compare(h, pw string) bool      { return h == "h:"+pw }

type stubToken struct{}

func (stubToken) GenerateToken(id string) (string, error) { return "tok:" + id, nil }

func TestHandler_TodosLosRPC(t *testing.T) {
	svc := service.NewUsuarioService(newStubRepo(), stubHasher{}, stubToken{})
	h := NewUsuarioHandler(svc)
	ctx := context.Background()

	// 1. RegistroUsuario
	reg, err := h.RegistroUsuario(ctx, &usuariosv1.RegistroUsuarioRequest{
		Nombre: "Ana", Email: "ana@ucn.cl", Password: "secret",
	})
	if err != nil || reg.GetToken() == "" || reg.GetUsuario().GetId() != testUsuarioID {
		t.Fatalf("RegistroUsuario falló: resp=%+v err=%v", reg, err)
	}

	// 2. Login exitoso y fallido
	if _, err := h.Login(ctx, &usuariosv1.LoginRequest{Email: "ana@ucn.cl", Password: "secret"}); err != nil {
		t.Fatalf("Login falló: %v", err)
	}
	if _, err := h.Login(ctx, &usuariosv1.LoginRequest{Email: "ana@ucn.cl", Password: "mala"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperaba Unauthenticated, obtuve %v", err)
	}

	// 3. ObtenerUsuario y ObtenerUsuarios
	if u, err := h.ObtenerUsuario(ctx, &usuariosv1.ObtenerUsuarioRequest{Id: testUsuarioID}); err != nil || u.GetNombre() != "Ana" {
		t.Fatalf("ObtenerUsuario falló: %+v err=%v", u, err)
	}
	if _, err := h.ObtenerUsuario(ctx, &usuariosv1.ObtenerUsuarioRequest{Id: "no-existe"}); status.Code(err) != codes.NotFound {
		t.Fatalf("esperaba NotFound, obtuve %v", err)
	}
	lote, err := h.ObtenerUsuarios(ctx, &usuariosv1.ObtenerUsuariosRequest{Ids: []string{testUsuarioID, "otro"}})
	if err != nil || len(lote.GetUsuarios()) != 1 {
		t.Fatalf("ObtenerUsuarios falló: %+v err=%v", lote, err)
	}

	// 4. AgregarRamosTutor, QuitarRamoTutor, ReactivarRamoTutor
	uTutor, err := h.AgregarRamosTutor(ctx, &usuariosv1.AgregarRamosTutorRequest{
		UsuarioId: testUsuarioID,
		Ramos: []*usuariosv1.InscripcionRamo{
			{RamoCodigo: "MAT1610", NotaAprobacion: 6.5, PeriodoAprobacion: "2025-2"},
		},
	})
	if err != nil || !uTutor.GetEsTutor() || len(uTutor.GetRamos()) != 1 {
		t.Fatalf("AgregarRamosTutor falló: %+v err=%v", uTutor, err)
	}

	uQuitado, err := h.QuitarRamoTutor(ctx, &usuariosv1.CambiarRamoTutorRequest{
		UsuarioId: testUsuarioID, RamoCodigo: "MAT1610",
	})
	if err != nil || uQuitado.GetEsTutor() {
		t.Fatalf("QuitarRamoTutor falló: %+v err=%v", uQuitado, err)
	}

	uReactivado, err := h.ReactivarRamoTutor(ctx, &usuariosv1.CambiarRamoTutorRequest{
		UsuarioId: testUsuarioID, RamoCodigo: "MAT1610",
	})
	if err != nil || !uReactivado.GetEsTutor() {
		t.Fatalf("ReactivarRamoTutor falló: %+v err=%v", uReactivado, err)
	}

	// 5. ListarRamos, ListarTutoresPorRamo, ValidarTutorRamo
	ramos, err := h.ListarRamos(ctx, &usuariosv1.ListarRamosRequest{})
	if err != nil || len(ramos.GetRamos()) != 1 {
		t.Fatalf("ListarRamos falló: %+v err=%v", ramos, err)
	}

	tutores, err := h.ListarTutoresPorRamo(ctx, &usuariosv1.ListarTutoresPorRamoRequest{RamoCodigo: "MAT1610"})
	if err != nil || len(tutores.GetTutores()) != 1 {
		t.Fatalf("ListarTutoresPorRamo falló: %+v err=%v", tutores, err)
	}

	val, err := h.ValidarTutorRamo(ctx, &usuariosv1.ValidarTutorRamoRequest{
		TutorId: testUsuarioID, RamoCodigo: "MAT1610",
	})
	if err != nil || !val.GetValido() {
		t.Fatalf("ValidarTutorRamo falló: %+v err=%v", val, err)
	}
}

func TestMapper_ErroresYNil(t *testing.T) {
	if toProtoUsuario(nil) != nil {
		t.Fatal("toProtoUsuario(nil) debe devolver nil")
	}
	if mapError(nil) != nil {
		t.Fatal("mapError(nil) debe devolver nil")
	}

	casos := []struct {
		err  error
		code codes.Code
	}{
		{domain.ErrNoEncontrado, codes.NotFound},
		{domain.ErrYaExiste, codes.AlreadyExists},
		{domain.ErrCredenciales, codes.Unauthenticated},
		{domain.ErrArgumentoInvalido, codes.InvalidArgument},
		{errors.New("desconocido"), codes.Internal},
	}

	for _, tc := range casos {
		if got := status.Code(mapError(tc.err)); got != tc.code {
			t.Fatalf("para %v esperaba %v, obtuve %v", tc.err, tc.code, got)
		}
	}
}
