package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/tutormatch/usuarios/internal/domain"
)

type UsuarioService struct {
	repo          domain.UsuarioRepository
	hasher        domain.PasswordHasher
	tokenProvider domain.TokenProvider
}

func NewUsuarioService(repo domain.UsuarioRepository, hasher domain.PasswordHasher, tokenProvider domain.TokenProvider) *UsuarioService {
	return &UsuarioService{
		repo:          repo,
		hasher:        hasher,
		tokenProvider: tokenProvider,
	}
}

// RegistroUsuario crea la cuenta y devuelve el JWT junto con el usuario para dejar la sesión iniciada.
func (s *UsuarioService) RegistroUsuario(ctx context.Context, nombre, email, password string) (string, *domain.Usuario, error) {
	nombre = strings.TrimSpace(nombre)
	email = domain.NormalizarEmail(email)

	if !domain.EsNombreValido(nombre) || !domain.EsEmailValido(email) || !domain.EsPasswordValida(password) {
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

	token, err := s.tokenProvider.GenerateToken(usuario.ID)
	if err != nil {
		return "", nil, err
	}

	return token, usuario, nil
}

// Login verifica credenciales con mitigación de timing attack y devuelve el JWT (HS256, 24h).
func (s *UsuarioService) Login(ctx context.Context, email, password string) (string, *domain.Usuario, error) {
	email = domain.NormalizarEmail(email)
	if !domain.EsEmailValido(email) || !domain.EsPasswordValida(password) {
		return "", nil, domain.ErrArgumentoInvalido
	}

	usuario, err := s.repo.ObtenerPorEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNoEncontrado) {
			// Ejecuta trabajo equivalente a bcrypt para evitar enumeración de correos por tiempo.
			s.hasher.Compare("", password)
			return "", nil, domain.ErrCredenciales
		}
		return "", nil, err
	}

	if !s.hasher.Compare(usuario.PasswordHash, password) {
		return "", nil, domain.ErrCredenciales
	}

	token, err := s.tokenProvider.GenerateToken(usuario.ID)
	if err != nil {
		return "", nil, err
	}

	return token, usuario, nil
}

func (s *UsuarioService) ObtenerUsuario(ctx context.Context, id string) (*domain.Usuario, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	if !domain.EsUUIDValido(id) {
		return nil, domain.ErrNoEncontrado
	}
	return s.repo.ObtenerPorID(ctx, id)
}

// ObtenerUsuarios filtra UUIDs válidos, deduplica en O(N) y acota el tamaño máximo del lote.
func (s *UsuarioService) ObtenerUsuarios(ctx context.Context, ids []string) ([]*domain.Usuario, error) {
	if len(ids) > domain.MaxBatchIDs {
		return nil, domain.ErrArgumentoInvalido
	}

	vistos := make(map[string]struct{}, len(ids))
	validos := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if !domain.EsUUIDValido(id) {
			continue
		}
		if _, dup := vistos[id]; dup {
			continue
		}
		vistos[id] = struct{}{}
		validos = append(validos, id)
	}
	if len(validos) == 0 {
		return []*domain.Usuario{}, nil
	}
	return s.repo.ObtenerPorIDs(ctx, validos)
}

// AgregarRamosTutor valida el formulario (nota 4.0..7.0 y periodo YYYY-1/2) e inscribe en transacción.
func (s *UsuarioService) AgregarRamosTutor(ctx context.Context, usuarioID string, ramos []domain.InscripcionRamoInput) (*domain.Usuario, error) {
	usuarioID = strings.TrimSpace(usuarioID)
	if usuarioID == "" || len(ramos) == 0 || len(ramos) > domain.MaxRamosPorSolicitud {
		return nil, domain.ErrArgumentoInvalido
	}

	vistos := make(map[string]struct{}, len(ramos))
	for i := range ramos {
		ramos[i].RamoCodigo = strings.TrimSpace(ramos[i].RamoCodigo)
		if err := ramos[i].Validar(); err != nil {
			return nil, err
		}
		if _, dup := vistos[ramos[i].RamoCodigo]; dup {
			return nil, domain.ErrArgumentoInvalido
		}
		vistos[ramos[i].RamoCodigo] = struct{}{}
	}

	if !domain.EsUUIDValido(usuarioID) {
		return nil, domain.ErrNoEncontrado
	}

	if err := s.repo.AgregarRamosTutorTx(ctx, usuarioID, ramos); err != nil {
		return nil, err
	}

	return s.repo.ObtenerPorID(ctx, usuarioID)
}

// QuitarRamoTutor desactiva un ramo conservando los datos de verificación y recalcula es_tutor.
func (s *UsuarioService) QuitarRamoTutor(ctx context.Context, usuarioID, ramoCodigo string) (*domain.Usuario, error) {
	return s.cambiarEstadoRamo(ctx, usuarioID, ramoCodigo, false)
}

// ReactivarRamoTutor vuelve a activar un ramo previamente inscrito y recalcula es_tutor.
func (s *UsuarioService) ReactivarRamoTutor(ctx context.Context, usuarioID, ramoCodigo string) (*domain.Usuario, error) {
	return s.cambiarEstadoRamo(ctx, usuarioID, ramoCodigo, true)
}

func (s *UsuarioService) cambiarEstadoRamo(ctx context.Context, usuarioID, ramoCodigo string, activo bool) (*domain.Usuario, error) {
	usuarioID = strings.TrimSpace(usuarioID)
	ramoCodigo = strings.TrimSpace(ramoCodigo)
	if usuarioID == "" || ramoCodigo == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	if !domain.EsUUIDValido(usuarioID) {
		return nil, domain.ErrNoEncontrado
	}

	if err := s.repo.CambiarEstadoRamoTx(ctx, usuarioID, ramoCodigo, activo); err != nil {
		return nil, err
	}

	return s.repo.ObtenerPorID(ctx, usuarioID)
}

func (s *UsuarioService) ListarRamos(ctx context.Context) ([]domain.Ramo, error) {
	return s.repo.ListarRamos(ctx)
}

func (s *UsuarioService) ListarTutoresPorRamo(ctx context.Context, ramoCodigo string) ([]*domain.Usuario, error) {
	ramoCodigo = strings.TrimSpace(ramoCodigo)
	if ramoCodigo == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	return s.repo.ListarTutoresPorRamo(ctx, ramoCodigo)
}

// ValidarTutorRamo devuelve true únicamente si el usuario existe y tiene el ramo activo.
func (s *UsuarioService) ValidarTutorRamo(ctx context.Context, tutorID, ramoCodigo string) (bool, error) {
	tutorID = strings.TrimSpace(tutorID)
	ramoCodigo = strings.TrimSpace(ramoCodigo)
	if tutorID == "" || ramoCodigo == "" {
		return false, domain.ErrArgumentoInvalido
	}
	if !domain.EsUUIDValido(tutorID) {
		return false, nil
	}

	return s.repo.ValidarTutorRamo(ctx, tutorID, ramoCodigo)
}
