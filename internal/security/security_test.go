package security

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestBcryptHasher(t *testing.T) {
	h := NewBcryptHasher()

	hash, err := h.Hash("secreta123")
	if err != nil || hash == "" || hash == "secreta123" {
		t.Fatalf("hash inválido: %q err=%v", hash, err)
	}
	if !h.Compare(hash, "secreta123") {
		t.Fatal("se esperaba coincidencia con la contraseña correcta")
	}
	if h.Compare(hash, "otra-clave") {
		t.Fatal("no debía coincidir con una contraseña distinta")
	}
}

func TestJWTProvider(t *testing.T) {
	secret := "secreto-prueba-hs256"
	p := NewJWTProvider(secret)

	tokenStr, err := p.GenerateToken("11111111-1111-4111-8111-111111111111")
	if err != nil || tokenStr == "" {
		t.Fatalf("error generando JWT: %v", err)
	}

	parsed, err := jwt.Parse(tokenStr, func(tok *jwt.Token) (any, error) {
		if tok.Method != jwt.SigningMethodHS256 {
			t.Fatalf("algoritmo inesperado: %v", tok.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token inválido: %v", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || claims["sub"] != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("claim sub inesperado: %+v", claims)
	}
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	if int64(exp-iat) != 24*60*60 {
		t.Fatalf("se esperaba expiración de 24h, diferencia=%v", exp-iat)
	}
}
