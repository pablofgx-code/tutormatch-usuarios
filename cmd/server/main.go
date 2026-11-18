package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	usuariosv1 "github.com/tutormatch/usuarios/gen/usuariosv1/usuarios/v1"
	grpcadapter "github.com/tutormatch/usuarios/internal/grpc"
	"github.com/tutormatch/usuarios/internal/repository"
	"github.com/tutormatch/usuarios/internal/security"
	"github.com/tutormatch/usuarios/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dbURL := getEnv("DATABASE_URL", "postgres://tutormatch:secretpassword@localhost:5432/usuarios_db?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "dev-secret-cambiar-en-produccion")
	port := getEnv("PORT", "50051")

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("error abriendo conexión a postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("no se pudo conectar a postgres: %v", err)
	}
	log.Println("Conexión a PostgreSQL establecida correctamente")

	// Inyección de dependencias siguiendo Clean Architecture
	repo := repository.NewPostgresRepository(db)
	hasher := security.NewBcryptHasher()
	tokenProvider := security.NewJWTProvider(jwtSecret)
	svc := service.NewUsuarioService(repo, hasher, tokenProvider)
	handler := grpcadapter.NewUsuarioHandler(svc)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("error al escuchar en el puerto %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	usuariosv1.RegisterUsuariosServiceServer(grpcServer, handler)

	// Habilita introspección para probar desde Postman o grpcurl sin cargar el archivo proto a mano
	reflection.Register(grpcServer)

	// Apagado elegante (Graceful Shutdown) cuando se detiene el contenedor o se presiona Ctrl+C
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("Apagando servidor gRPC...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Servidor gRPC de Usuarios escuchando en el puerto %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("error ejecutando servidor gRPC: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
