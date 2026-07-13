.PHONY: build test dev clean

build:
	cd frontend && npm run build
	go build -o bin/pantry ./cmd/server

test:
	go test ./...
	cd frontend && npm run build

dev:
	go run ./cmd/server -data ./data

clean:
	rm -rf bin frontend/dist
