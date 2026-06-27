# ASHN — Adventure Society Health Network

ASHN is a lore-themed EDI healthcare transaction simulator. Phase 1 uses structured JSON payloads that mirror X12 transaction flow.

## Quick Start

```sh
cp .env.example .env
make build
make test
```

## Run The Whole Stack

Start Postgres, apply migrations, run all services, and launch the dashboard:

```sh
make dev-stack
```

Open the dashboard:

```text
http://localhost:9300
```

In a second terminal, run the full ASHN workflow:

```sh
make demo
```

The demo performs:

1. `834` adventurer enrollment
2. `270 → 271` eligibility verification
3. `278` prior authorization
4. `837` claim submission
5. `276 → 277` claim status
6. `835` payment/remittance

Service logs are written under `.dev/logs/` while `make dev-stack` is running.

## Database

Start Postgres and apply the schema/seed data:

```sh
make db
make migrate
```

Inspect row counts:

```sh
make db-status
```

Reset the local database volume and reapply migrations:

```sh
make db-reset
make migrate
```

For a clean demo database:

```sh
make demo-reset
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
