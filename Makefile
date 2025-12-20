.PHONY: proto build test clean run-server run-client

proto:
	protoc --proto_path=./proto \
		--go_out=./pkg/pb --go-grpc_out=./pkg/pb \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		./proto/*.proto
build:
	go build -o bin/server ./cmd/server/
	go build -o bin/client ./cmd/client/

test:
	echo "Running tests..."

run-server:
	go run ./cmd/server/

run-client:
	go run ./cmd/client/

clean:
	rm -rf pkg/pb/*
	rm -rf bin/*
