.PHONY: dev-api dev-web build test

dev-api:
	go run ./cmd/server

dev-web:
	cd web && npm run dev

build:
	cd web && npm ci && npm run build
	go build -o bin/goose-canvas ./cmd/server

test:
	go test ./...
	cd web && npm test
	cd web && npm run build
