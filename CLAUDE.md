# CLAUDE.md - Contexto del Proyecto `tutormatch-usuarios`

## Visión general
Este repositorio contiene el microservicio de **Usuarios** de la plataforma **TutorMatch**, desarrollado en **Go** con **PostgreSQL 16** y expuesto mediante **gRPC**. Forma parte de un sistema distribuido en Kubernetes junto con un BFF (NestJS/GraphQL), Sesiones (Spring Boot), Material (NestJS/MongoDB) y un Agente de IA con MCP.

## Arquitectura y estructura de paquetes
El proyecto aplica Arquitectura Limpia (Clean Architecture) e Inversión de Dependencias:
* `cmd/server/main.go`: Punto de entrada, configuración de variables de entorno, conexión a PostgreSQL e inyección de dependencias.
* `contracts/`: Submódulo Git (`tutormatch-contracts`) con la definición oficial en `proto/usuarios/v1/usuarios.proto`.
* `gen/usuariosv1/`: Código Go autogenerado por `protoc` (no editar manualmente).
* `internal/domain/`: Entidades puras (`Usuario`, `Ramo`, `RamoTutor`), errores de dominio y puertos (interfaces `UsuarioRepository`, `PasswordHasher`, `TokenProvider`).
* `internal/service/`: Lógica de negocio y casos de uso (`usuarios_service.go`) junto con sus pruebas unitarias con mocks (`usuarios_service_test.go`).
* `internal/repository/`: Implementación de persistencia en PostgreSQL (`postgres.go`) usando `database/sql` y el driver `pgx/v5/stdlib`.
* `internal/security/`: Adaptadores de infraestructura para encriptación (`bcrypt_hasher.go`) y firma de tokens JWT HS256 (`jwt_provider.go`).
* `internal/grpc/`: Adaptador de red gRPC (`handler.go`) y traductor de entidades/errores a códigos de estado gRPC (`mapper.go`).
* `db/init.sql`: Esquema inicial de base de datos (`usuarios`, `ramos`, `tutor_ramos`) y catálogo semilla de asignaturas.

## Reglas de negocio e invariantes críticas
1. **Cálculo automático de `es_tutor`**: El campo `es_tutor` nunca se modifica de forma directa desde un endpoint. Se recalcula automáticamente dentro de la misma transacción SQL (`recalcularEsTutor`) cada vez que se ejecutan `AgregarRamosTutor`, `QuitarRamoTutor` o `ReactivarRamoTutor`. Es `true` si el usuario tiene al menos un ramo con `activo = true`.
2. **Borrado lógico de ramos**: `QuitarRamoTutor` no elimina filas de `tutor_ramos`; actualiza `activo = false` conservando la nota y el semestre para permitir `ReactivarRamoTutor` de forma idempotente.
3. **Validaciones académicas**: Las notas de aprobación deben estar en la escala chilena (`4.0` a `7.0`) y el periodo debe cumplir el formato `YYYY-1` o `YYYY-2` (ej. `2025-2`).
4. **Autenticación**: `RegistroUsuario` y `Login` devuelven un JWT firmado con `HS256` que contiene el UUID del usuario en el claim `sub` y expira en 24 horas (`exp`), usando la variable de entorno `JWT_SECRET`.
5. **Mapeo de errores gRPC**:
   * `domain.ErrNoEncontrado` se traduce a `codes.NotFound`
   * `domain.ErrYaExiste` se traduce a `codes.AlreadyExists`
   * `domain.ErrCredenciales` se traduce a `codes.Unauthenticated`
   * `domain.ErrArgumentoInvalido` se traduce a `codes.InvalidArgument`

## Comandos principales
* **Levantar entorno completo en Docker:** `docker compose up -d --build`
* **Ejecutar servidor localmente:** `go run ./cmd/server/main.go`
* **Ejecutar tests y cobertura (mínimo exigido 60%):** `go test -v -cover ./internal/...`
* **Análisis estático:** `go vet ./cmd/... ./internal/...`
* **Regenerar código Protobuf:**
  `protoc --proto_path=contracts/proto --go_out=gen/usuariosv1 --go_opt=paths=source_relative --go-grpc_out=gen/usuariosv1 --go-grpc_opt=paths=source_relative usuarios/v1/usuarios.proto`