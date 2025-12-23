.PHONY: proto build test clean run-server run-client

proto:
	protoc --proto_path=./proto \
		--go_out=./pkg/pb --go-grpc_out=./pkg/pb \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		./proto/*.proto

build: proto
	make -j2 build-server build-client

test:
	echo "Running tests..."

build-server:
	go build -o bin/server ./cmd/server/

build-client:
	go build -o bin/client ./cmd/client/

run-server:
	go run ./cmd/server/

run-client:
	go run ./cmd/client/

clean:
	rm -rf pkg/pb/*
	rm -rf bin/*
