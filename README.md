# ASHN — Adventure Society Health Network

ASHN is a lore-themed healthcare EDI simulator that makes X12-style payer/provider workflows visible. It now supports JSON/XML/raw X12 intake, trading partner validation, raw X12 generation, acknowledgments, async authorization/adjudication jobs, claim and prior-auth attachments, 275 document-vault receipts, export/replay tools, and a dashboard for learning how the transaction chain fits together.

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

This target requires Docker to be running because it starts Postgres before executing the integration suite. It applies the migration, verifies schema/seed reset behavior, runs the payer-core Postgres workflow, and exercises intake through the gateway/intake path.

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

The stack also starts `edi-intake` on `http://localhost:8083`; canonical XML/JSON intake is available through the gateway at `POST /v1/x12/transactions`, `POST /v1/x12/xml` is kept as the XML compatibility route, `POST /v1/x12/raw` accepts first-pass raw X12 `834`/`820`/`270`/`276`/`278`/`837`/`835`/`275` text, and `POST /v1/x12/batch` accepts multipart XML/JSON/X12 demo files.
Trading partner profiles and routing rules are available at `GET /v1/x12/trading-partners`; those profiles now enforce partner-specific `275` attachment rules plus `837` diagnosis/procedure rules before accepted intake is forwarded to `payer-core`.
Transaction details can be exported from `GET /v1/transactions/{id}/export?format=json|xml|x12` and replayed with `POST /v1/transactions/{id}/replay`.
Intake audit records can be exported from `GET /v1/x12/messages/{id}/export?format=xml|json` and replayed with `POST /v1/x12/messages/{id}/replay`.
The dashboard XML Intake tab includes an operational rejection console that trends failed partner submissions, groups them by partner, transaction type, and validation reason, and offers one-click drilldown, inspect, and replay controls for demos and debugging.
The dashboard Workflow tab also includes executable and exportable demo scenarios. Each scenario can run a repeatable workflow, stream step results into Live Session Events, download a versioned JSON runbook, and export a JSON evidence bundle with step results, transaction IDs, and artifact hints for stakeholder walkthroughs.

API authentication is opt-in. Set `ASHN_API_KEYS` on `api-gateway` to a comma-separated list of accepted keys; protected `/v1` routes then accept either `Authorization: Bearer <key>` or `X-ASHN-API-Key: <key>`. `GET /v1/health` stays public for health checks. If the dashboard talks to an authenticated gateway, set `VITE_ASHN_API_KEY` to the same demo key at build/runtime.

The gateway also rate-limits public/demo traffic with `ASHN_RATE_LIMIT_REQUESTS` and `ASHN_RATE_LIMIT_WINDOW`. Buckets are keyed by API key when present, otherwise by caller IP; health checks and preflight requests stay exempt.

Every service echoes and propagates `X-Request-ID` and `X-Correlation-ID`. Clients can provide either header; otherwise the receiving service creates IDs and forwards them through gateway, intake, provider, payer, and worker health boundaries.

Services also accept, emit, and propagate W3C `traceparent` and `tracestate` headers for basic OpenTelemetry-compatible trace context. Each service creates a local span ID, forwards trace context downstream, and includes `traceId`, `spanId`, and `parentSpanId` in structured request logs.

Service logs are emitted as structured JSON events with stable fields such as `time`, `level`, `msg`, `service`, `requestId`, `correlationId`, transaction/job IDs, and relevant status/error details. This keeps local demos readable while making Render/Docker logs easier to filter.

`make demo-reset` drops the local Postgres volume, starts a fresh database, reapplies `000001_init.up.sql`, and restores known seed data such as six providers and three trading partners.

## What It Exposes For Learning

- How payer, provider, gateway, intake, worker, and ledger service boundaries fit together.
- How common healthcare EDI transaction types relate: `834`, `270/271`, `278`, `275`, `837`, `276/277`, `835`, `999`, and `277CA`.
- How XML/JSON intake, trading partner validation, acknowledgments, raw X12, and durable audit trails can coexist in one workflow.
- How `837` diagnoses, service lines, partner companion-guide profiles, and `835` remittance details connect from intake through adjudication.
- How asynchronous review and adjudication change transaction state over time instead of completing every workflow immediately.
- How documentation requests, per-document review, deficiency follow-up, and resubmission work in a 275 attachment flow.
- How external 275 document references are resolved as safe vault receipts without server-side fetching arbitrary URLs.
- How opt-in gateway API keys protect partner-facing routes without blocking public health checks.
- How gateway rate limiting protects public/demo endpoints while leaving health and CORS checks available.
- How request and correlation IDs make multi-service EDI flows traceable.
- How OpenTelemetry-compatible trace context follows gateway, intake, payer, provider, and worker requests.
- How structured JSON logs expose operational events across gateway, intake, payer, provider, worker, and migration services.

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
