# ASHN — Adventure Society Health Network

ASHN is a lore-themed healthcare EDI simulator that makes X12-style payer/provider workflows visible. It now supports JSON/XML intake, trading partner validation, raw X12 generation, acknowledgments, async authorization/adjudication jobs, claim and prior-auth attachments, a 275 documentation workbench, export/replay tools, and a dashboard for learning how the transaction chain fits together.

Docs:

- `docs/elevator-pitch.md` — presentation-ready project summary
- `docs/x12-workflow.md` — how ASHN maps healthcare X12 into the demo workflow
- `docs/future-enhancements.md` — completed foundation plus remaining hardening backlog
- `docs/presentations/` — elevator pitch and technical deep-dive presentation decks
- `docs/deployment.md` — Render + Netlify deployment guide

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
4. `275` documentation attachment when payer review needs supporting records
5. `837` claim submission
6. `276 → 277` claim status
7. `835` payment/remittance

The stack also starts `tx-worker`, which asynchronously reviews queued `278` authorizations and claim adjudication jobs so statuses can change over time in the dashboard.

Service logs are written under `.dev/logs/` while `make dev-stack` is running.

The stack also starts `edi-intake` on `http://localhost:8083`; canonical XML/JSON intake is available through the gateway at `POST /v1/x12/transactions`, with `POST /v1/x12/xml` kept as the XML compatibility route.
Trading partner profiles and routing rules are available at `GET /v1/x12/trading-partners`.
Transaction details can be exported from `GET /v1/transactions/{id}/export?format=json|xml|x12` and replayed with `POST /v1/transactions/{id}/replay`.
XML intake audit records can be exported from `GET /v1/x12/messages/{id}/export?format=xml|json` and replayed with `POST /v1/x12/messages/{id}/replay`.

## What It Exposes For Learning

- How payer, provider, gateway, intake, worker, and ledger service boundaries fit together.
- How common healthcare EDI transaction types relate: `834`, `270/271`, `278`, `275`, `837`, `276/277`, `835`, `999`, and `277CA`.
- How XML/JSON intake, trading partner validation, acknowledgments, raw X12, and durable audit trails can coexist in one workflow.
- How asynchronous review and adjudication change transaction state over time instead of completing every workflow immediately.
- How documentation requests, per-document review, deficiency follow-up, and resubmission work in a 275 attachment flow.

## Docker Compose Backend

Start the containerized backend stack with service health checks:

```sh
make dev
```

This starts Postgres, `payer-core`, `provider-service`, `edi-intake`, `tx-worker`, and `api-gateway`. Compose waits on each upstream health check before starting dependent services, and the gateway is available at `http://localhost:8080`.

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
