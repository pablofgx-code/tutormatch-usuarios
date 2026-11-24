# Etapa 1: Compilación del binario
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/usuarios-server ./cmd/server/main.go

# Etapa 2: Imagen final ligera de ejecución
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata \
    && adduser -D -H -u 10001 appuser

WORKDIR /app
COPY --from=builder /app/usuarios-server /app/usuarios-server

USER 10001:10001
EXPOSE 50051 9090

ENTRYPOINT ["/app/usuarios-server"]