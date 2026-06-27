.PHONY: dev dev-stack demo demo-reset db db-wait db-reset db-status migrate test build

dev:
	docker compose -f infra/docker-compose.yml up -d

dev-stack:
	./scripts/dev-stack.sh

demo:
	./scripts/demo-flow.sh

demo-reset: db-reset db migrate
	@echo "Start services with 'make dev-stack' in another terminal, then run 'make demo'."

db:
	docker compose -f infra/docker-compose.yml up -d postgres

db-wait:
	./scripts/wait-for-postgres.sh

test:
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go test ./...

migrate:
	./scripts/wait-for-postgres.sh
	docker compose -f infra/docker-compose.yml exec -T postgres psql -U ashn_user -d ashn -f /migrations/000001_init.up.sql

db-status:
	./scripts/db-status.sh

db-reset:
	docker compose -f infra/docker-compose.yml down -v
	docker compose -f infra/docker-compose.yml up -d postgres
	./scripts/wait-for-postgres.sh

build:
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go build ./...
