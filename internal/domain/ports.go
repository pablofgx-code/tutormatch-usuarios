package domain

import "context"

type UsuarioRepository interface {
	CrearUsuario(ctx context.Context, u *Usuario) error
	ObtenerPorEmail(ctx context.Context, email string) (*Usuario, error)
	ObtenerPorID(ctx context.Context, id string) (*Usuario, error)
	ObtenerPorIDs(ctx context.Context, ids []string) ([]*Usuario, error)
	ActualizarRamos(ctx context.Context, usuarioID string, ramoCodigos []string) (*Usuario, error)
	BuscarTutoresPorRamo(ctx context.Context, ramoCodigo string) ([]*Usuario, error)
	ListarRamos(ctx context.Context) ([]Ramo, error)
	VerificarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, bool, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type TokenProvider interface {
	GenerateToken(usuarioID string) (string, error)
}
