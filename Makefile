.PHONY: proto build test clean run-server run-client

proto:
	protoc --proto_path=./proto \
		--go_out=./pkg/pb --go-grpc_out=./pkg/pb \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		./proto/*.proto

build: proto
	make -j3 build-server build-client build-control

test:
	@echo "Running tests..."

build-server:
	go build -o bin/server ./cmd/server/

build-client:
	go build -o bin/client ./cmd/client/

build-control:
	go build -o bin/control ./cmd/control/

run-server:
	go run ./cmd/server/ $(ARGS)

# Pass additional arguments to the client via ARGS variable -
# e.g., make run-client ARGS="--head localhost:9000 --tail localhost:9002"
run-client:
	go run ./cmd/client/ $(ARGS)

run-control:
	go run ./cmd/control/ $(ARGS)

clean:
	rm -rf pkg/pb/*
	rm -rf bin/*
