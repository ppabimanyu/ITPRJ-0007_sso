SHELL := /bin/sh

.PHONY: dev build test lint migrate seed bootstrap web-build clean

# Rebuild images, apply migrations, and start the whole local stack.
dev:
	docker compose up -d postgres redis mailhog
	docker compose --profile migrate run --rm migrate up
	docker compose up -d --build app caddy

build:
	go build -trimpath -o bin/server ./cmd/server
	go build -trimpath -o bin/migration ./cmd/migration

test:
	go test ./...

lint:
	golangci-lint run

# Runs against DATABASE_URL from the shell or .env.
migrate:
	go run ./cmd/migration up

# Pipes the password from stdin. Example:
#   printf '%s\n' 'Dev-Only-Passw0rd!' | make seed
seed:
	go run ./cmd/migration seed --password-stdin

# Requires EMAIL. Example:
#   EMAIL=admin@example.com make bootstrap
bootstrap:
	@test -n "$(EMAIL)" || (echo "EMAIL is required: EMAIL=admin@example.com make bootstrap" && exit 1)
	go run ./cmd/migration bootstrap --email "$(EMAIL)" --password-stdin

# Build the SPA and embed it in the Go binary. Requires Node.js 22 or newer.
web-build:
	cd web && npm ci
	cd web && npm run build
	@find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
	@cp -R web/dist/. internal/webui/dist/

clean:
	rm -rf bin web/dist
	@find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
