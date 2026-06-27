.PHONY: dev dev-stack demo db migrate test build

dev:
	docker compose -f infra/docker-compose.yml up -d

dev-stack:
	./scripts/dev-stack.sh

demo:
	./scripts/demo-flow.sh

db:
	docker compose -f infra/docker-compose.yml up -d postgres

test:
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go test ./...

migrate:
	docker compose -f infra/docker-compose.yml exec -T postgres psql -U ashn_user -d ashn -f /migrations/000001_init.up.sql

build:
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build ./...
