.PHONY: dev test migrate build

dev:
	docker compose -f infra/docker-compose.yml up -d

test:
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go test ./...

migrate:
	@echo "Run golang-migrate against infra/migrations with DATABASE_URL"

build:
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build ./...
