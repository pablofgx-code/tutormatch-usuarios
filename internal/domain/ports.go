package domain

import "context"

type UsuarioRepository interface {
	CrearUsuario(ctx context.Context, u *Usuario) error
	ObtenerPorEmail(ctx context.Context, email string) (*Usuario, error)
	ObtenerPorID(ctx context.Context, id string) (*Usuario, error)
	ObtenerPorIDs(ctx context.Context, ids []string) ([]*Usuario, error)

	// Reemplazan a ActualizarRamos:
	AgregarRamosTutorTx(ctx context.Context, usuarioID string, ramos []InscripcionRamoInput) error
	CambiarEstadoRamoTx(ctx context.Context, usuarioID, ramoCodigo string, activo bool) error

	ListarRamos(ctx context.Context) ([]Ramo, error)
	ListarTutoresPorRamo(ctx context.Context, ramoCodigo string) ([]*Usuario, error)
	ValidarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, bool, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type TokenProvider interface {
	GenerateToken(usuarioID string) (string, error)
}
