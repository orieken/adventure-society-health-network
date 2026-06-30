# ASHN — Adventure Society Health Network

ASHN is a lore-themed EDI healthcare transaction simulator. Phase 1 uses structured JSON payloads that mirror X12 transaction flow.

Docs:

- `docs/elevator-pitch.md` — presentation-ready project summary
- `docs/x12-workflow.md` — how ASHN maps healthcare X12 into the demo workflow
- `docs/future-enhancements.md` — prioritized backlog, including the proposed XML EDI intake service

## Quick Start

```sh
cp .env.example .env
make build
make test
```

Run DB-backed integration tests:

```sh
make test-integration
```

This target requires Docker to be running because it starts Postgres before executing the integration suite.

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

The stack also starts `edi-intake` on `http://localhost:8083`; the public XML endpoint is available through the gateway at `POST /v1/x12/xml`.

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
go run ./apps/edi-intake
go run ./apps/api-gateway
```

If `DATABASE_URL` is unset or unreachable, services fall back to in-memory state and seeded providers.

Run services in separate terminals:

```sh
go run ./apps/payer-core
go run ./apps/provider-service
go run ./apps/edi-intake
go run ./apps/api-gateway
go run ./apps/ashn-cli providers list
```
