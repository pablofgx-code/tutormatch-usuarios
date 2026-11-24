package repository

import (
	"context"
	"sync"
	"time"

	"github.com/tutormatch/usuarios/internal/domain"
)

// CachedCatalogoRepository aplica el Patrón Decorator sobre domain.UsuarioRepository
// para cachear únicamente el catálogo estático de ramos con TTL (OCP).
type CachedCatalogoRepository struct {
	domain.UsuarioRepository
	ttl      time.Duration
	now      func() time.Time
	mu       sync.RWMutex
	ramos    []domain.Ramo
	expiraEn time.Time
}

func NewCachedCatalogoRepository(base domain.UsuarioRepository, ttl time.Duration) *CachedCatalogoRepository {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &CachedCatalogoRepository{
		UsuarioRepository: base,
		ttl:               ttl,
		now:               time.Now,
	}
}

func (c *CachedCatalogoRepository) ListarRamos(ctx context.Context) ([]domain.Ramo, error) {
	c.mu.RLock()
	if len(c.ramos) > 0 && c.now().Before(c.expiraEn) {
		copia := make([]domain.Ramo, len(c.ramos))
		copy(copia, c.ramos)
		c.mu.RUnlock()
		return copia, nil
	}
	c.mu.RUnlock()

	ramos, err := c.UsuarioRepository.ListarRamos(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.ramos = make([]domain.Ramo, len(ramos))
	copy(c.ramos, ramos)
	c.expiraEn = c.now().Add(c.ttl)
	c.mu.Unlock()

	copia := make([]domain.Ramo, len(ramos))
	copy(copia, ramos)
	return copia, nil
}
