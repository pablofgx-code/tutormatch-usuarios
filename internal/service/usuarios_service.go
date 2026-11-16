package service

import (
	"context"
	"time"

	"github.com/tutormatch/usuarios/internal/domain"
)

type UsuarioService struct {
	repo          domain.UsuarioRepository
	hasher        domain.PasswordHasher
	tokenProvider domain.TokenProvider
}

// NewUsuarioService inyecta las dependencias necesarias cumpliendo con el principio de inversión de dependencias
func NewUsuarioService(repo domain.UsuarioRepository, hasher domain.PasswordHasher, tokenProvider domain.TokenProvider) *UsuarioService {
	return &UsuarioService{
		repo:          repo,
		hasher:        hasher,
		tokenProvider: tokenProvider,
	}
}

// Registrar crea un nuevo usuario, encripta su contraseña y le genera su token de sesión inicial
func (s *UsuarioService) Registrar(ctx context.Context, nombre, email, password string) (string, *domain.Usuario, error) {
	if nombre == "" || email == "" || password == "" {
		return "", nil, domain.ErrArgumentoInvalido
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return "", nil, err
	}

	usuario := &domain.Usuario{
		Nombre:       nombre,
		Email:        email,
		PasswordHash: hash,
		CreadoEn:     time.Now().UTC(),
	}

	if err := s.repo.CrearUsuario(ctx, usuario); err != nil {
		return "", nil, err
	}

	// Generamos el JWT de inmediato para que quede con la sesión iniciada (como exige el contrato)
	token, err := s.tokenProvider.GenerateToken(usuario.ID)
	if err != nil {
		return "", nil, err
	}

	return token, usuario, nil
}

// IniciarSesion verifica credenciales y emite el token JWT
func (s *UsuarioService) IniciarSesion(ctx context.Context, email, password string) (string, *domain.Usuario, error) {
	if email == "" || password == "" {
		return "", nil, domain.ErrArgumentoInvalido
	}

	usuario, err := s.repo.ObtenerPorEmail(ctx, email)
	if err != nil {
		if err == domain.ErrNoEncontrado {
			return "", nil, domain.ErrCredenciales
		}
		return "", nil, err
	}

	// Comparamos la contraseña encriptada con bcrypt
	if !s.hasher.Compare(usuario.PasswordHash, password) {
		return "", nil, domain.ErrCredenciales
	}

	token, err := s.tokenProvider.GenerateToken(usuario.ID)
	if err != nil {
		return "", nil, err
	}

	return token, usuario, nil
}

// ObtenerUsuario busca un usuario por su ID
func (s *UsuarioService) ObtenerUsuario(ctx context.Context, id string) (*domain.Usuario, error) {
	if id == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	return s.repo.ObtenerPorID(ctx, id)
}

// ObtenerUsuarios resuelve la consulta en lote para el BFF
func (s *UsuarioService) ObtenerUsuarios(ctx context.Context, ids []string) ([]*domain.Usuario, error) {
	return s.repo.ObtenerPorIDs(ctx, ids)
}

// ActualizarRamos reemplaza los ramos de un tutor y recalcula su es_tutor
func (s *UsuarioService) ActualizarRamos(ctx context.Context, usuarioID string, ramoCodigos []string) (*domain.Usuario, error) {
	if usuarioID == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	return s.repo.ActualizarRamos(ctx, usuarioID, ramoCodigos)
}

// BuscarTutores busca tutores activos que impartan un ramo
func (s *UsuarioService) BuscarTutores(ctx context.Context, ramoCodigo string) ([]*domain.Usuario, error) {
	if ramoCodigo == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	return s.repo.BuscarTutoresPorRamo(ctx, ramoCodigo)
}

// ListarRamos devuelve el catálogo oficial
func (s *UsuarioService) ListarRamos(ctx context.Context) ([]domain.Ramo, error) {
	return s.repo.ListarRamos(ctx) // Asumiendo firma limpia
}

// VerificarTutorRamo valida para Sesiones si el tutor existe y aprobó el ramo
func (s *UsuarioService) VerificarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, bool, error) {
	if tutorID == "" || ramoCodigo == "" {
		return false, false, domain.ErrArgumentoInvalido
	}
	return s.repo.VerificarTutorRamo(ctx, tutorID, ramoCodigo)
}
