package grpc

import (
	"context"
	"errors"
	"testing"

	usuariosv1 "github.com/tutormatch/usuarios/gen/usuariosv1/usuarios/v1"
	"github.com/tutormatch/usuarios/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testUsuarioID = "11111111-1111-4111-8111-111111111111"

type stubUseCase struct {
	usuario *domain.Usuario
	err     error
	valido  bool
}

func (s *stubUseCase) RegistroUsuario(_ context.Context, _, _, _ string) (string, *domain.Usuario, error) {
	return "tok-1", s.usuario, s.err
}
func (s *stubUseCase) Login(_ context.Context, _, _ string) (string, *domain.Usuario, error) {
	return "tok-1", s.usuario, s.err
}
func (s *stubUseCase) ObtenerUsuario(_ context.Context, _ string) (*domain.Usuario, error) {
	return s.usuario, s.err
}
func (s *stubUseCase) ObtenerUsuarios(_ context.Context, _ []string) ([]*domain.Usuario, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []*domain.Usuario{s.usuario}, nil
}
func (s *stubUseCase) AgregarRamosTutor(_ context.Context, _ string, _ []domain.InscripcionRamoInput) (*domain.Usuario, error) {
	return s.usuario, s.err
}
func (s *stubUseCase) QuitarRamoTutor(_ context.Context, _, _ string) (*domain.Usuario, error) {
	return s.usuario, s.err
}
func (s *stubUseCase) ReactivarRamoTutor(_ context.Context, _, _ string) (*domain.Usuario, error) {
	return s.usuario, s.err
}
func (s *stubUseCase) ListarRamos(_ context.Context) ([]domain.Ramo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []domain.Ramo{{Codigo: "MAT1610", Nombre: "Cálculo I"}}, nil
}
func (s *stubUseCase) ListarTutoresPorRamo(_ context.Context, _ string) ([]*domain.Usuario, error) {
	if s.err != nil {
		return nil, s.err
	}
	return []*domain.Usuario{s.usuario}, nil
}
func (s *stubUseCase) ValidarTutorRamo(_ context.Context, _, _ string) (bool, error) {
	return s.valido, s.err
}

func TestHandler_TodosLosRPC(t *testing.T) {
	u := &domain.Usuario{
		ID:      testUsuarioID,
		Nombre:  "Ana",
		Email:   "ana@ucn.cl",
		EsTutor: true,
		Ramos:   []domain.RamoTutor{{Ramo: domain.Ramo{Codigo: "MAT1610", Nombre: "Cálculo I"}, Activo: true}},
	}
	stub := &stubUseCase{usuario: u, valido: true}
	h := NewUsuarioHandler(stub)
	ctx := context.Background()

	if reg, err := h.RegistroUsuario(ctx, &usuariosv1.RegistroUsuarioRequest{Nombre: "Ana", Email: "ana@ucn.cl", Password: "secret"}); err != nil || reg.GetToken() == "" || reg.GetUsuario().GetId() != testUsuarioID {
		t.Fatalf("RegistroUsuario falló: %+v err=%v", reg, err)
	}
	if _, err := h.Login(ctx, &usuariosv1.LoginRequest{Email: "ana@ucn.cl", Password: "secret"}); err != nil {
		t.Fatalf("Login falló: %v", err)
	}
	if res, err := h.ObtenerUsuario(ctx, &usuariosv1.ObtenerUsuarioRequest{Id: testUsuarioID}); err != nil || res.GetNombre() != "Ana" {
		t.Fatalf("ObtenerUsuario falló: %+v err=%v", res, err)
	}
	if lote, err := h.ObtenerUsuarios(ctx, &usuariosv1.ObtenerUsuariosRequest{Ids: []string{testUsuarioID}}); err != nil || len(lote.GetUsuarios()) != 1 {
		t.Fatalf("ObtenerUsuarios falló: %+v err=%v", lote, err)
	}
	if res, err := h.AgregarRamosTutor(ctx, &usuariosv1.AgregarRamosTutorRequest{
		UsuarioId: testUsuarioID,
		Ramos:     []*usuariosv1.InscripcionRamo{{RamoCodigo: "MAT1610", NotaAprobacion: 6.5, PeriodoAprobacion: "2025-2"}},
	}); err != nil || !res.GetEsTutor() {
		t.Fatalf("AgregarRamosTutor falló: %+v err=%v", res, err)
	}
	if _, err := h.QuitarRamoTutor(ctx, &usuariosv1.CambiarRamoTutorRequest{UsuarioId: testUsuarioID, RamoCodigo: "MAT1610"}); err != nil {
		t.Fatalf("QuitarRamoTutor falló: %v", err)
	}
	if _, err := h.ReactivarRamoTutor(ctx, &usuariosv1.CambiarRamoTutorRequest{UsuarioId: testUsuarioID, RamoCodigo: "MAT1610"}); err != nil {
		t.Fatalf("ReactivarRamoTutor falló: %v", err)
	}
	if ramos, err := h.ListarRamos(ctx, &usuariosv1.ListarRamosRequest{}); err != nil || len(ramos.GetRamos()) != 1 {
		t.Fatalf("ListarRamos falló: %+v err=%v", ramos, err)
	}
	if tutores, err := h.ListarTutoresPorRamo(ctx, &usuariosv1.ListarTutoresPorRamoRequest{RamoCodigo: "MAT1610"}); err != nil || len(tutores.GetTutores()) != 1 {
		t.Fatalf("ListarTutoresPorRamo falló: %+v err=%v", tutores, err)
	}
	if val, err := h.ValidarTutorRamo(ctx, &usuariosv1.ValidarTutorRamoRequest{TutorId: testUsuarioID, RamoCodigo: "MAT1610"}); err != nil || !val.GetValido() {
		t.Fatalf("ValidarTutorRamo falló: %+v err=%v", val, err)
	}

	// Errores propagados por el handler
	stub.err = domain.ErrNoEncontrado
	if _, err := h.ObtenerUsuario(ctx, &usuariosv1.ObtenerUsuarioRequest{Id: "x"}); status.Code(err) != codes.NotFound {
		t.Fatalf("esperaba NotFound, obtuve %v", err)
	}
	stub.err = domain.ErrCredenciales
	if _, err := h.Login(ctx, &usuariosv1.LoginRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("esperaba Unauthenticated, obtuve %v", err)
	}
}

func TestMapperYRecoveryInterceptor(t *testing.T) {
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
		{errors.New("error sql crudo"), codes.Internal},
	}

	for _, tc := range casos {
		if got := status.Code(mapError(tc.err)); got != tc.code {
			t.Fatalf("para %v esperaba %v, obtuve %v", tc.err, tc.code, got)
		}
	}

	// RecoveryInterceptor: paso normal y captura de panic
	info := &grpc.UnaryServerInfo{FullMethod: "/tutorias.usuarios.v1.UsuariosService/Login"}
	resp, err := RecoveryInterceptor(context.Background(), nil, info, func(_ context.Context, _ any) (any, error) {
		return "ok", nil
	})
	if err != nil || resp != "ok" {
		t.Fatalf("ejecución normal falló: resp=%v err=%v", resp, err)
	}

	_, err = RecoveryInterceptor(context.Background(), nil, info, func(_ context.Context, _ any) (any, error) {
		panic("fallo crítico simulado")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("esperaba codes.Internal tras panic, obtuve %v", err)
	}
}
