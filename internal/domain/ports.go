package domain

import "context"

// Puertos segregados por responsabilidad (ISP) para permitir decoradores y pruebas focalizadas.
type CuentaRepository interface {
	CrearUsuario(ctx context.Context, u *Usuario) error
	ObtenerPorEmail(ctx context.Context, email string) (*Usuario, error)
	ObtenerPorID(ctx context.Context, id string) (*Usuario, error)
	ObtenerPorIDs(ctx context.Context, ids []string) ([]*Usuario, error)
}

type TutorRamoRepository interface {
	AgregarRamosTutorTx(ctx context.Context, usuarioID string, ramos []InscripcionRamoInput) error
	CambiarEstadoRamoTx(ctx context.Context, usuarioID, ramoCodigo string, activo bool) error
	ListarTutoresPorRamo(ctx context.Context, ramoCodigo string) ([]*Usuario, error)
	ValidarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, error)
}

type CatalogoRepository interface {
	ListarRamos(ctx context.Context) ([]Ramo, error)
}

// UsuarioRepository compone las capacidades completas de persistencia del microservicio.
type UsuarioRepository interface {
	CuentaRepository
	TutorRamoRepository
	CatalogoRepository
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	// Compare verifica la contraseña; con hash vacío realiza trabajo equivalente (anti timing-attack) y devuelve false.
	Compare(hash, password string) bool
}

type TokenProvider interface {
	GenerateToken(usuarioID string) (string, error)
}
