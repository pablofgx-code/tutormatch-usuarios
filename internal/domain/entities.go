package domain

import (
	"strings"
	"time"
)

type Ramo struct {
	Codigo string
	Nombre string
}

type Usuario struct {
	ID             string
	Nombre         string
	Email          string
	PasswordHash   string
	EsTutor        bool
	RamosAprobados []Ramo
	CreadoEn       time.Time
}

func NormalizarEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
