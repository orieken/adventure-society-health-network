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

## Request Tracing

The gateway accepts or creates `X-Request-ID` and `X-Correlation-ID`, returns both headers to the caller, and forwards them to downstream services. Provide `X-Correlation-ID` when grouping several calls into one demo or replay workflow.
