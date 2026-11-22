-include .env
export

TAILWIND ?= tailwindcss
CSS_IN  := web/static/css/input.css
CSS_OUT := web/static/css/app.css

.PHONY: dev db migrate gen css build build-prod deploy test lint audit

dev:
	docker compose up app

db:
	docker compose up -d --wait db

migrate:
	go tool goose -dir migrations postgres "$(MIGRATIONS_URL)" up

gen:
	go tool templ generate
	sqlc generate

css:
	$(TAILWIND) -i $(CSS_IN) -o $(CSS_OUT) --minify

build: gen css
	go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

# Artefactos Linux amd64 para producción; goose corre migraciones al iniciar el servicio.
build-prod: gen css
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
