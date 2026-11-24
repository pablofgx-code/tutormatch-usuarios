package grpc

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryInterceptor atrapa panics inesperados en cualquier RPC para evitar la caída del proceso.
func RecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			metodo := "desconocido"
			if info != nil {
				metodo = info.FullMethod
			}
			log.Printf("[gRPC] Panic recuperado en %s: %v", metodo, r)
			err = status.Error(codes.Internal, "error interno del servidor")
		}
	}()

	return handler(ctx, req)
}
