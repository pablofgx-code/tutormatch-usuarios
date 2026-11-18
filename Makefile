.PHONY: proto run test docker-up docker-down

proto:
	protoc --proto_path=contracts/proto --go_out=gen/usuariosv1 --go_opt=paths=source_relative --go-grpc_out=gen/usuariosv1 --go-grpc_opt=paths=source_relative usuarios/v1/usuarios.proto

run:
	go run ./cmd/server/main.go

test:
	go test -v -cover ./internal/...

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down