# api-gateway

Public ASHN HTTP API facade. Routes `/v1/*` requests to `payer-core`, `provider-service`, and `edi-intake`.

## Optional API Key Auth

Set `ASHN_API_KEYS` to a comma-separated list of accepted keys to protect `/v1` routes:

```sh
ASHN_API_KEYS=dev-secret,partner-secret go run ./apps/api-gateway
```

Clients can authenticate with either header:

```text
Authorization: Bearer dev-secret
X-ASHN-API-Key: dev-secret
```

`GET /v1/health` and CORS preflight requests stay public so Render, Docker Compose, and browser clients can still perform health checks.

## Rate Limiting

The gateway applies an in-memory fixed-window rate limit to `/v1` routes. Requests are grouped by API key when `Authorization: Bearer <key>` or `X-ASHN-API-Key` is present, otherwise by remote IP address. `GET /v1/health` and CORS preflight requests are exempt.

```sh
ASHN_RATE_LIMIT_REQUESTS=300 ASHN_RATE_LIMIT_WINDOW=1m go run ./apps/api-gateway
```

Set `ASHN_RATE_LIMIT_REQUESTS=0` to disable the limiter for local stress testing. Limited responses return `429`, `Retry-After`, and `X-RateLimit-*` headers.

## Request Tracing

The gateway accepts or creates `X-Request-ID` and `X-Correlation-ID`, returns both headers to the caller, and forwards them to downstream services. Provide `X-Correlation-ID` when grouping several calls into one demo or replay workflow.

The gateway also accepts and propagates W3C `traceparent` and `tracestate` headers. If a caller does not provide `traceparent`, ASHN creates a trace ID and per-service span IDs for basic OpenTelemetry-compatible request tracing.

Logs are JSON events. Request logs include `service`, `method`, `path`, `requestId`, and `correlationId`; domain/service logs include IDs and status fields relevant to the event.
