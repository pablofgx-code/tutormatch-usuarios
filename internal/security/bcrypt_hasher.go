package security

import "golang.org/x/crypto/bcrypt"

// BcryptHasher implementa la interfaz domain.PasswordHasher
type BcryptHasher struct{}

func NewBcryptHasher() *BcryptHasher {
	return &BcryptHasher{}
}

// Hash encripta la contraseña usando el algoritmo bcrypt con el costo por defecto
func (h *BcryptHasher) Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Compare verifica si la contraseña en texto plano coincide con el hash de la base de datos
func (h *BcryptHasher) Compare(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
