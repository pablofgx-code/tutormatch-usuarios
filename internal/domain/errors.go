package domain

import "errors"

var (
	ErrNoEncontrado      = errors.New("recurso no encontrado")
	ErrYaExiste          = errors.New("el recurso ya existe")
	ErrCredenciales      = errors.New("email o contraseña incorrectos")
	ErrArgumentoInvalido = errors.New("datos de entrada inválidos")
)
