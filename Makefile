.PHONY: build test dev dev-reset-admin clean

build:
	cd frontend && npm run build
	GOCACHE=$${GOCACHE:-/tmp/eatby-go-cache} go build -o bin/pantry ./cmd/server

test:
	GOCACHE=$${GOCACHE:-/tmp/eatby-go-cache} go test ./...
	cd frontend && npm test
	cd frontend && npm run build

dev:
	set -a; . ./.env; set +a; GOCACHE=$${GOCACHE:-/tmp/eatby-go-cache} go run ./cmd/server -data ./dev-data

dev-reset-admin:
	set -a; . ./.env; set +a; GOCACHE=$${GOCACHE:-/tmp/eatby-go-cache} go run ./cmd/server -data ./dev-data -reset-admin-password

clean:
	rm -rf bin frontend/dist
