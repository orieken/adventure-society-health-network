# ASHN — Adventure Society Health Network

ASHN is a lore-themed EDI healthcare transaction simulator. Phase 1 uses structured JSON payloads that mirror X12 transaction flow.

## Quick Start

```sh
cp .env.example .env
make build
make test
```

## Database

Start Postgres and apply the schema/seed data:

```sh
make db
make migrate
```

Run services with persistence:

```sh
export DATABASE_URL="postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable"
go run ./apps/payer-core
go run ./apps/provider-service
go run ./apps/api-gateway
```

If `DATABASE_URL` is unset or unreachable, services fall back to in-memory state and seeded providers.

Run services in separate terminals:

```sh
go run ./apps/payer-core
go run ./apps/provider-service
go run ./apps/api-gateway
go run ./apps/ashn-cli providers list
```
