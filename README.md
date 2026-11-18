# TutorMatch - Microservicio de Usuarios (gRPC)

Microservicio en Go bajo Arquitectura Limpia y PostgreSQL encargado de la gestión de cuentas, autenticación JWT (HS256), catálogo de asignaturas y verificación de tutores.

## Requisitos previos

* Docker y Docker Compose (para levantar el entorno completo).
* Go y Protocol Buffers (`protoc`) únicamente si se desea compilar de forma local fuera de Docker.

## Ejecución rápida con Docker (Recomendado para el equipo)

Para levantar la base de datos PostgreSQL (con las tablas y el catálogo inicial de ramos) junto con el servidor gRPC en un solo paso:

```bash
docker compose up -d --build
```

* **Servidor gRPC:** `localhost:50051` (con Server Reflection activado para Postman o `grpcurl`).
* **Base de datos PostgreSQL:** `localhost:5432` (usuario: `tutormatch`, contraseña: `secretpassword`, base de datos: `usuarios_db`).

Para detener los contenedores:

```bash
docker compose down
```

## Variables de entorno

| Variable | Descripción | Valor por defecto (Desarrollo) |
| :--- | :--- | :--- |
| `PORT` | Puerto TCP del servidor gRPC | `50051` |
| `DATABASE_URL` | Cadena de conexión a PostgreSQL | `postgres://tutormatch:secretpassword@localhost:5432/usuarios_db?sslmode=disable` |
| `JWT_SECRET` | Secreto compartido con el BFF para firmar tokens HS256 | `dev-secret-cambiar-en-produccion` |

## Comandos de desarrollo local

* **Iniciar el servidor sin Docker (requiere PostgreSQL activo):**
  ```bash
  go run ./cmd/server/main.go
  ```
* **Ejecutar pruebas unitarias y cobertura:**
  ```bash
  go test -v -cover ./internal/...
  ```
* **Regenerar código gRPC desde el submódulo de contratos:**
  ```bash
  protoc --proto_path=contracts/proto --go_out=gen/usuariosv1 --go_opt=paths=source_relative --go-grpc_out=gen/usuariosv1 --go-grpc_opt=paths=source_relative usuarios/v1/usuarios.proto
  ```

## Endpoints gRPC implementados (`tutorias.usuarios.v1.UsuariosService`)

* `RegistroUsuario`: Crea un usuario con contraseña encriptada (`bcrypt`) y devuelve su token JWT inicial.
* `Login`: Valida credenciales y emite un token JWT firmado con HS256 válido por 24 horas.
* `ObtenerUsuario` / `ObtenerUsuarios`: Consulta perfiles individuales o en lote a partir de sus UUIDs.
* `AgregarRamosTutor`: Registra asignaturas aprobadas (nota 4.0 a 7.0 y semestre `YYYY-1` o `YYYY-2`) y activa `es_tutor = true`.
* `QuitarRamoTutor` / `ReactivarRamoTutor`: Desactiva o reactiva ramos conservando el historial y recalculando `es_tutor` en transacción.
* `ListarRamos`: Devuelve el catálogo oficial de asignaturas.
* `ListarTutoresPorRamo`: Filtra tutores activos que imparten una asignatura específica.
* `ValidarTutorRamo`: Confirma a los microservicios de Sesiones y Material si un usuario existe y tiene activo un ramo.