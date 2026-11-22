-include .env
export

TAILWIND ?= tailwindcss
CSS_IN  := web/static/css/input.css
CSS_OUT := web/static/css/app.css

.PHONY: dev db migrate gen css build test lint audit

dev: db migrate
	go tool air

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

test:
	go test -race ./...

lint:
	golangci-lint run

audit:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go run github.com/securego/gosec/v2/cmd/gosec@latest ./...
