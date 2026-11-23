package domain

import (
	"errors"
	"testing"
)

func TestValidacionesDominio(t *testing.T) {
	if got := NormalizarEmail("  PABLO@UCN.CL  "); got != "pablo@ucn.cl" {
		t.Fatalf("email normalizado inesperado: %q", got)
	}

	if !EsEmailValido("ana@uc.cl") || EsEmailValido("sin-arroba") || EsEmailValido("a @b.cl") {
		t.Fatal("EsEmailValido devolvió resultado inesperado")
	}

	if !EsUUIDValido("11111111-1111-4111-8111-111111111111") || EsUUIDValido("no-es-uuid") {
		t.Fatal("EsUUIDValido devolvió resultado inesperado")
	}

	casos := []struct {
		nombre string
		in     InscripcionRamoInput
		errEsp error
	}{
		{"valido", InscripcionRamoInput{"MAT1610", 6.5, "2025-2"}, nil},
		{"ramo vacio", InscripcionRamoInput{"  ", 6.0, "2025-1"}, ErrArgumentoInvalido},
		{"nota baja", InscripcionRamoInput{"MAT1610", 3.9, "2025-1"}, ErrArgumentoInvalido},
		{"nota alta", InscripcionRamoInput{"MAT1610", 7.1, "2025-1"}, ErrArgumentoInvalido},
		{"periodo invalido", InscripcionRamoInput{"MAT1610", 5.5, "2025-3"}, ErrArgumentoInvalido},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			if err := tc.in.Validar(); !errors.Is(err, tc.errEsp) {
				t.Fatalf("esperaba %v, obtuve %v", tc.errEsp, err)
			}
		})
	}
}
