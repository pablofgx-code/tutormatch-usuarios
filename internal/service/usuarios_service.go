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

	if nombre == "" || !domain.EsEmailValido(email) || password == "" {
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

// Login verifica credenciales y devuelve el token JWT (HS256, 24h) más el usuario.
func (s *UsuarioService) Login(ctx context.Context, email, password string) (string, *domain.Usuario, error) {
	email = domain.NormalizarEmail(email)
	if email == "" || password == "" {
		return "", nil, domain.ErrArgumentoInvalido
	}

	usuario, err := s.repo.ObtenerPorEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNoEncontrado) {
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

func (s *UsuarioService) ObtenerUsuarios(ctx context.Context, ids []string) ([]*domain.Usuario, error) {
	validos := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if domain.EsUUIDValido(id) {
			validos = append(validos, id)
		}
	}
	if len(validos) == 0 {
		return []*domain.Usuario{}, nil
	}
	return s.repo.ObtenerPorIDs(ctx, validos)
}

// AgregarRamosTutor valida el formulario (nota 4.0..7.0 y periodo YYYY-1/2) e inscribe en transacción.
func (s *UsuarioService) AgregarRamosTutor(ctx context.Context, usuarioID string, ramos []domain.InscripcionRamoInput) (*domain.Usuario, error) {
	usuarioID = strings.TrimSpace(usuarioID)
	if usuarioID == "" || len(ramos) == 0 {
		return nil, domain.ErrArgumentoInvalido
	}

	// Validamos formato y que no vengan códigos duplicados en la misma petición.
	vistos := make(map[string]bool, len(ramos))
	for i := range ramos {
		ramos[i].RamoCodigo = strings.TrimSpace(ramos[i].RamoCodigo)
		if err := ramos[i].Validar(); err != nil {
			return nil, err
		}
		if vistos[ramos[i].RamoCodigo] {
			return nil, domain.ErrArgumentoInvalido
		}
		vistos[ramos[i].RamoCodigo] = true
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
	usuarioID = strings.TrimSpace(usuarioID)
	ramoCodigo = strings.TrimSpace(ramoCodigo)
	if usuarioID == "" || ramoCodigo == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	if !domain.EsUUIDValido(usuarioID) {
		return nil, domain.ErrNoEncontrado
	}

	if err := s.repo.CambiarEstadoRamoTx(ctx, usuarioID, ramoCodigo, false); err != nil {
		return nil, err
	}

	return s.repo.ObtenerPorID(ctx, usuarioID)
}

// ReactivarRamoTutor vuelve a activar un ramo previamente inscrito y recalcula es_tutor.
func (s *UsuarioService) ReactivarRamoTutor(ctx context.Context, usuarioID, ramoCodigo string) (*domain.Usuario, error) {
	usuarioID = strings.TrimSpace(usuarioID)
	ramoCodigo = strings.TrimSpace(ramoCodigo)
	if usuarioID == "" || ramoCodigo == "" {
		return nil, domain.ErrArgumentoInvalido
	}
	if !domain.EsUUIDValido(usuarioID) {
		return nil, domain.ErrNoEncontrado
	}

	if err := s.repo.CambiarEstadoRamoTx(ctx, usuarioID, ramoCodigo, true); err != nil {
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

	existe, activo, err := s.repo.ValidarTutorRamo(ctx, tutorID, ramoCodigo)
	if err != nil {
		return false, err
	}

	return existe && activo, nil
}
