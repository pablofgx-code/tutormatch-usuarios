package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
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

const (
	defaultDevJWTSecret = "dev-secret-cambiar-en-produccion"
	minProdJWTSecretLen = 32
	maxGRPCMsgBytes     = 1 << 20 // 1 MB
)

func main() {
	appEnv := strings.ToLower(getEnv("APP_ENV", "development"))
	dbURL := getEnv("DATABASE_URL", "postgres://tutormatch:secretpassword@localhost:5432/usuarios_db?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", defaultDevJWTSecret)
	port := getEnv("PORT", "50051")

	if appEnv == "production" {
		if jwtSecret == "" || jwtSecret == defaultDevJWTSecret || len(jwtSecret) < minProdJWTSecretLen {
			log.Fatal("ERROR FATAL: en APP_ENV=production JWT_SECRET es obligatorio, no puede usar el valor de desarrollo y debe tener al menos 32 caracteres")
		}
	} else if jwtSecret == defaultDevJWTSecret {
		log.Println("ADVERTENCIA: usando JWT_SECRET de desarrollo; configurar Secret seguro en producción")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("error abriendo conexión a postgres: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("no se pudo conectar a postgres: %v", err)
	}
	log.Println("Conexión a PostgreSQL establecida correctamente")

	// Inyección de dependencias + Decorator de caché TTL para el catálogo de ramos
	pgRepo := repository.NewPostgresRepository(db)
	repo := repository.NewCachedCatalogoRepository(pgRepo, 10*time.Minute)
	hasher := security.NewBcryptHasher()
	tokenProvider := security.NewJWTProvider(jwtSecret)
	svc := service.NewUsuarioService(repo, hasher, tokenProvider)
	handler := grpcadapter.NewUsuarioHandler(svc)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("error al escuchar en el puerto %s: %v", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxGRPCMsgBytes),
		grpc.UnaryInterceptor(grpcadapter.RecoveryInterceptor),
	)
	usuariosv1.RegisterUsuariosServiceServer(grpcServer, handler)

	// Server Reflection solo fuera de producción para reducir superficie de exposición.
	if appEnv != "production" {
		reflection.Register(grpcServer)
	}

	// Apagado elegante (Graceful Shutdown)
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("Apagando servidor gRPC...")
		grpcServer.GracefulStop()
	}()

	log.Printf("Servidor gRPC de Usuarios escuchando en el puerto %s (entorno: %s)", port, appEnv)
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
