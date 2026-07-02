.PHONY: dev dev-stack demo demo-reset db db-wait db-reset db-status migrate test test-integration test-coverage test-e2e build

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

test-integration: db migrate
	mkdir -p .gocache
	ASHN_INTEGRATION=1 DATABASE_URL="postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable" GOCACHE=$(PWD)/.gocache go test ./apps/payer-core -run Integration -count=1 -v
	DATABASE_URL="postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable" ./scripts/xml-intake-integration.sh

test-coverage: db migrate
	mkdir -p .gocache
	GOCACHE=$(PWD)/.gocache go test ./... -coverprofile=coverage.out
	ASHN_INTEGRATION=1 DATABASE_URL="postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable" GOCACHE=$(PWD)/.gocache go test ./apps/payer-core -run Integration -count=1 -coverprofile=coverage.integration.out
	{ head -n 1 coverage.out; tail -n +2 coverage.out; tail -n +2 coverage.integration.out; } > coverage.merged.out
	GOCACHE=$(PWD)/.gocache go tool cover -func=coverage.merged.out | tail -n 1

test-e2e:
	npm run test:e2e

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
