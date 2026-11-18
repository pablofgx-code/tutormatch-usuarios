package grpc

import (
	"context"

	usuariosv1 "github.com/tutormatch/usuarios/gen/usuariosv1/usuarios/v1"
	"github.com/tutormatch/usuarios/internal/domain"
	"github.com/tutormatch/usuarios/internal/service"
)

type UsuarioHandler struct {
	usuariosv1.UnimplementedUsuariosServiceServer
	svc *service.UsuarioService
}

func NewUsuarioHandler(svc *service.UsuarioService) *UsuarioHandler {
	return &UsuarioHandler{svc: svc}
}

func (h *UsuarioHandler) RegistroUsuario(ctx context.Context, req *usuariosv1.RegistroUsuarioRequest) (*usuariosv1.LoginResponse, error) {
	token, usuario, err := h.svc.RegistroUsuario(ctx, req.GetNombre(), req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, mapError(err)
	}

	return &usuariosv1.LoginResponse{
		Token:   token,
		Usuario: toProtoUsuario(usuario),
	}, nil
}

func (h *UsuarioHandler) Login(ctx context.Context, req *usuariosv1.LoginRequest) (*usuariosv1.LoginResponse, error) {
	token, usuario, err := h.svc.Login(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, mapError(err)
	}

	return &usuariosv1.LoginResponse{
		Token:   token,
		Usuario: toProtoUsuario(usuario),
	}, nil
}

func (h *UsuarioHandler) ObtenerUsuario(ctx context.Context, req *usuariosv1.ObtenerUsuarioRequest) (*usuariosv1.Usuario, error) {
	usuario, err := h.svc.ObtenerUsuario(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}

	return toProtoUsuario(usuario), nil
}

func (h *UsuarioHandler) ObtenerUsuarios(ctx context.Context, req *usuariosv1.ObtenerUsuariosRequest) (*usuariosv1.ObtenerUsuariosResponse, error) {
	usuarios, err := h.svc.ObtenerUsuarios(ctx, req.GetIds())
	if err != nil {
		return nil, mapError(err)
	}

	return &usuariosv1.ObtenerUsuariosResponse{
		Usuarios: toProtoUsuarios(usuarios),
	}, nil
}

func (h *UsuarioHandler) AgregarRamosTutor(ctx context.Context, req *usuariosv1.AgregarRamosTutorRequest) (*usuariosv1.Usuario, error) {
	inscripciones := make([]domain.InscripcionRamoInput, 0, len(req.GetRamos()))
	for _, r := range req.GetRamos() {
		inscripciones = append(inscripciones, domain.InscripcionRamoInput{
			RamoCodigo:        r.GetRamoCodigo(),
			NotaAprobacion:    r.GetNotaAprobacion(),
			PeriodoAprobacion: r.GetPeriodoAprobacion(),
		})
	}

	usuario, err := h.svc.AgregarRamosTutor(ctx, req.GetUsuarioId(), inscripciones)
	if err != nil {
		return nil, mapError(err)
	}

	return toProtoUsuario(usuario), nil
}

func (h *UsuarioHandler) QuitarRamoTutor(ctx context.Context, req *usuariosv1.CambiarRamoTutorRequest) (*usuariosv1.Usuario, error) {
	usuario, err := h.svc.QuitarRamoTutor(ctx, req.GetUsuarioId(), req.GetRamoCodigo())
	if err != nil {
		return nil, mapError(err)
	}

	return toProtoUsuario(usuario), nil
}

func (h *UsuarioHandler) ReactivarRamoTutor(ctx context.Context, req *usuariosv1.CambiarRamoTutorRequest) (*usuariosv1.Usuario, error) {
	usuario, err := h.svc.ReactivarRamoTutor(ctx, req.GetUsuarioId(), req.GetRamoCodigo())
	if err != nil {
		return nil, mapError(err)
	}

	return toProtoUsuario(usuario), nil
}

func (h *UsuarioHandler) ListarRamos(ctx context.Context, _ *usuariosv1.ListarRamosRequest) (*usuariosv1.ListarRamosResponse, error) {
	ramos, err := h.svc.ListarRamos(ctx)
	if err != nil {
		return nil, mapError(err)
	}

	ramosProto := make([]*usuariosv1.Ramo, 0, len(ramos))
	for _, r := range ramos {
		ramosProto = append(ramosProto, &usuariosv1.Ramo{
			Codigo: r.Codigo,
			Nombre: r.Nombre,
		})
	}

	return &usuariosv1.ListarRamosResponse{
		Ramos: ramosProto,
	}, nil
}

func (h *UsuarioHandler) ListarTutoresPorRamo(ctx context.Context, req *usuariosv1.ListarTutoresPorRamoRequest) (*usuariosv1.ListarTutoresPorRamoResponse, error) {
	tutores, err := h.svc.ListarTutoresPorRamo(ctx, req.GetRamoCodigo())
	if err != nil {
		return nil, mapError(err)
	}

	return &usuariosv1.ListarTutoresPorRamoResponse{
		Tutores: toProtoUsuarios(tutores),
	}, nil
}

func (h *UsuarioHandler) ValidarTutorRamo(ctx context.Context, req *usuariosv1.ValidarTutorRamoRequest) (*usuariosv1.ValidarTutorRamoResponse, error) {
	valido, err := h.svc.ValidarTutorRamo(ctx, req.GetTutorId(), req.GetRamoCodigo())
	if err != nil {
		return nil, mapError(err)
	}

	return &usuariosv1.ValidarTutorRamoResponse{
		Valido: valido,
	}, nil
}
