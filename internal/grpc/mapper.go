package grpc

import (
	"errors"
	"log"

	usuariosv1 "github.com/tutormatch/usuarios/gen/usuariosv1/usuarios/v1"
	"github.com/tutormatch/usuarios/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mapError traduce los errores puros de dominio a códigos gRPC sin filtrar detalles internos al cliente.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, domain.ErrNoEncontrado):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrYaExiste):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrCredenciales):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrArgumentoInvalido):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		log.Printf("[gRPC] Error interno inesperado: %v", err)
		return status.Error(codes.Internal, "error interno del servidor")
	}
}

// toProtoUsuario convierte la entidad de dominio al mensaje Usuario de Protobuf.
func toProtoUsuario(u *domain.Usuario) *usuariosv1.Usuario {
	if u == nil {
		return nil
	}

	ramosProto := make([]*usuariosv1.RamoTutor, 0, len(u.Ramos))
	for _, rt := range u.Ramos {
		ramosProto = append(ramosProto, &usuariosv1.RamoTutor{
			Ramo: &usuariosv1.Ramo{
				Codigo: rt.Ramo.Codigo,
				Nombre: rt.Ramo.Nombre,
			},
			Activo: rt.Activo,
		})
	}

	return &usuariosv1.Usuario{
		Id:      u.ID,
		Nombre:  u.Nombre,
		Email:   u.Email,
		EsTutor: u.EsTutor,
		Ramos:   ramosProto,
	}
}

// toProtoUsuarios convierte una lista de usuarios de dominio a Protobuf.
func toProtoUsuarios(usuarios []*domain.Usuario) []*usuariosv1.Usuario {
	res := make([]*usuariosv1.Usuario, 0, len(usuarios))
	for _, u := range usuarios {
		res = append(res, toProtoUsuario(u))
	}
	return res
}
