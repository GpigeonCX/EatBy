.PHONY: build test dev clean

build:
	cd frontend && npm run build
	GOCACHE=$${GOCACHE:-/tmp/eatby-go-cache} go build -o bin/pantry ./cmd/server

test:
	GOCACHE=$${GOCACHE:-/tmp/eatby-go-cache} go test ./...
	cd frontend && npm test
	cd frontend && npm run build

dev:
	EATBY_ADMIN_USERNAME=$${EATBY_ADMIN_USERNAME:-admin} EATBY_ADMIN_PASSWORD=$${EATBY_ADMIN_PASSWORD:-change-this-password} go run ./cmd/server -data ./dev-data

clean:
	rm -rf bin frontend/dist
