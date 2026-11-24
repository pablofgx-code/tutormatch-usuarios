package security

import (
	"github.com/tutormatch/usuarios/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher implementa domain.PasswordHasher con protección contra timing attacks.
type BcryptHasher struct {
	dummyHash []byte
}

func NewBcryptHasher() *BcryptHasher {
	dummy, _ := bcrypt.GenerateFromPassword([]byte("tutormatch-timing-guard"), bcrypt.DefaultCost)
	return &BcryptHasher{dummyHash: dummy}
}

// Hash encripta la contraseña con bcrypt validando el límite nativo de 72 bytes.
func (h *BcryptHasher) Hash(password string) (string, error) {
	if !domain.EsPasswordValida(password) {
		return "", domain.ErrArgumentoInvalido
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Compare verifica la contraseña; si hash está vacío compara contra dummyHash para igualar tiempos.
func (h *BcryptHasher) Compare(hash, password string) bool {
	if !domain.EsPasswordValida(password) {
		return false
	}
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(h.dummyHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
