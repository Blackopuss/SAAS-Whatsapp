-include .env
export

.PHONY: dev db migrate gen web build build-prod deploy test lint audit

dev:
	docker compose up app

db:
	docker compose up -d --wait db

migrate:
	go tool goose -dir migrations postgres "$(MIGRATIONS_URL)" up

gen:
	go tool templ generate
	sqlc generate

# Interfaz (React + Astryx) en frontend/; sale a web/static/app/, que sirve Go.
web:
	cd frontend && npm ci && npm run typecheck && npm run build && npm run budget

build: gen web
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

# Artefactos Linux amd64 para producción; goose corre migraciones al iniciar el servicio.
build-prod: gen web
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/createadmin ./cmd/createadmin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/createstore ./cmd/createstore
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/goose github.com/pressly/goose/v3/cmd/goose

deploy:
	bash ./deploy/deploy.sh

test:
	go test -race ./...

lint:
	golangci-lint run

audit:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go run github.com/securego/gosec/v2/cmd/gosec@latest -exclude-generated ./...
