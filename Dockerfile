# Etapa 1: Compilación del binario
FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /usuarios-server ./cmd/server/main.go

# Etapa 2: Imagen final ligera de ejecución
FROM alpine:3.20

WORKDIR /app
COPY --from=builder /usuarios-server .

EXPOSE 50051

CMD ["./usuarios-server"]