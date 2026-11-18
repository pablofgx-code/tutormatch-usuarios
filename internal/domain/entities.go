package domain

import (
	"regexp"
	"strings"
	"time"
)

var periodoRegex = regexp.MustCompile(`^[0-9]{4}-[12]$`)

type Ramo struct {
	Codigo string
	Nombre string
}

// RamoTutor representa la inscripción histórica de un tutor en un ramo
type RamoTutor struct {
	Ramo              Ramo
	NotaAprobacion    float64
	PeriodoAprobacion string
	Activo            bool // false si el tutor lo quitó (permite reactivar)
	VerificadoEn      time.Time
}

type Usuario struct {
	ID           string
	Nombre       string
	Email        string
	PasswordHash string
	EsTutor      bool
	Ramos        []RamoTutor // Reemplaza a RamosAprobados
	CreadoEn     time.Time
}

// InscripcionRamoInput son los datos puros que vienen del formulario del front
type InscripcionRamoInput struct {
	RamoCodigo        string
	NotaAprobacion    float64
	PeriodoAprobacion string
}

func NormalizarEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Validar verifica la escala chilena y el formato de periodo antes de tocar BD
func (i InscripcionRamoInput) Validar() error {
	if strings.TrimSpace(i.RamoCodigo) == "" {
		return ErrArgumentoInvalido
	}
	if i.NotaAprobacion < 4.0 || i.NotaAprobacion > 7.0 {
		return ErrArgumentoInvalido
	}
	if !periodoRegex.MatchString(i.PeriodoAprobacion) {
		return ErrArgumentoInvalido
	}
	return nil
}
