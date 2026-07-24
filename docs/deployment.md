# ASHN Deployment Guide

ASHN deploys cleanly as:

- **Render** for Go services and Postgres
- **Netlify** for the Vite dashboard

## Render

Use the root-level `render.yaml` Blueprint.

It creates:

- `ashn-api-gateway`
- `ashn-payer-core`
- `ashn-provider-service`
- `ashn-edi-intake`
- `ashn-postgres`

`payer-core` applies the initial migration on startup when `ASHN_AUTO_MIGRATE=true`, so the database schema and seed data are applied automatically on Render's free tier.

For the first deployment, `payer-core` also runs the async job processor in embedded mode via:

```text
ASHN_AUTO_MIGRATE=true
ASHN_EMBED_WORKER=true
```

This avoids needing a separate paid Render background worker while keeping async auth/adjudication behavior live.

### Render Setup Steps

1. In Render, choose **New + → Blueprint**.
2. Connect `orieken/adventure-society-health-network`.
3. Select the root `render.yaml`.
4. Create the Blueprint.
5. Wait for `ashn-payer-core` to start and apply its startup migration.
6. Confirm `ashn-api-gateway` health:

```text
https://ashn-api-gateway.onrender.com/v1/health
```

If Render assigns different service hostnames, update these env vars in Render:

- `PAYER_CORE_URL`
- `PROVIDER_SERVICE_URL`
- `EDI_INTAKE_URL`
- `ASHN_RATE_LIMIT_REQUESTS`
- `ASHN_RATE_LIMIT_WINDOW`

Then update Netlify's `VITE_ASHN_API_URL`.

The dashboard polls several read-only endpoints together, so the demo Render gateway uses a larger rate-limit budget than the local default. If the Netlify dashboard shows missing service signals while `/v1/health` is healthy, check for `429 rate limit exceeded` responses and raise `ASHN_RATE_LIMIT_REQUESTS` or slow the dashboard polling interval.

## Netlify

Use the root-level `netlify.toml`.

It configures:

- base directory: `apps/dashboard`
- build command: `npm ci && npm run build`
- publish directory: `dist`
- API URL: `https://ashn-api-gateway.onrender.com`

### Netlify Setup Steps

1. In Netlify, choose **Add new site → Import an existing project**.
2. Connect `orieken/adventure-society-health-network`.
3. Netlify should read `netlify.toml`.
4. Confirm the production env var:

```text
VITE_ASHN_API_URL=https://ashn-api-gateway.onrender.com
```

5. Deploy the site.

## Smoke Test

After both are deployed:

1. Open the Netlify dashboard URL.
2. Confirm Gateway health shows `ok`.
3. Enroll an adventurer.
4. Submit eligibility, auth, claim, and payment.
5. Open a transaction detail.
6. Test export as JSON/XML/X12.
7. Test replay transaction or XML intake replay.

## Synthetic Monitoring

ASHN includes a scheduled GitHub Actions monitor in `.github/workflows/synthetic-monitor.yml`.

It runs every six hours and checks the deployed service surface:

- OpenAPI roots
- Service health endpoints
- Gateway pagination
- Request ID, correlation ID, and trace header propagation

Readiness and metrics contracts are available as a manual post-deploy check. Use this after Render finishes deploying gateway changes that include `/v1/system/readiness` and `/v1/metrics/summary`.

To enable dashboard smoke checks on the schedule, add this repository variable:

```text
ASHN_DASHBOARD_URL=https://your-netlify-site.netlify.app
```

The Render URLs default to the public demo services, but you can override them with repository variables:

```text
ASHN_API_URL=https://ashn-api-gateway.onrender.com
ASHN_PAYER_CORE_URL=https://ashn-payer-core.onrender.com
ASHN_PROVIDER_SERVICE_URL=https://ashn-provider-service.onrender.com
ASHN_EDI_INTAKE_URL=https://ashn-edi-intake.onrender.com
```

Use **Actions → Synthetic Monitor → Run workflow** to trigger an on-demand smoke check. Set `run_operations=true` to include readiness/metrics checks. Set `run_mutating=true` only for release validation or demo environments because it creates ledger, intake, and partner-management records.

## Notes

- This is still a demo/learning deployment, not a HIPAA-ready production deployment.
- Render free web services may sleep when inactive; the first request can be slow.
- If you later want a dedicated async worker, add `apps/tx-worker` as a Render background worker and remove `ASHN_EMBED_WORKER=true`.
