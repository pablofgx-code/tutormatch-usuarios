package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTProvider implementa la interfaz domain.TokenProvider
type JWTProvider struct {
	secretKey []byte
}

func NewJWTProvider(secret string) *JWTProvider {
	return &JWTProvider{secretKey: []byte(secret)}
}

// GenerateToken crea un JWT con el algoritmo HS256, el ID en "sub" y 24h de expiración
func (p *JWTProvider) GenerateToken(usuarioID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": usuarioID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secretKey)
}
