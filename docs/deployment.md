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

`payer-core` runs `apps/migrate` before deploy, so the database schema and seed data are applied automatically.

For the first deployment, `payer-core` also runs the async job processor in embedded mode via:

```text
ASHN_EMBED_WORKER=true
```

This avoids needing a separate paid Render background worker while keeping async auth/adjudication behavior live.

### Render Setup Steps

1. In Render, choose **New + → Blueprint**.
2. Connect `orieken/adventure-society-health-network`.
3. Select the root `render.yaml`.
4. Create the Blueprint.
5. Wait for `ashn-payer-core` to finish its pre-deploy migration.
6. Confirm `ashn-api-gateway` health:

```text
https://ashn-api-gateway.onrender.com/v1/health
```

If Render assigns different service hostnames, update these env vars in Render:

- `PAYER_CORE_URL`
- `PROVIDER_SERVICE_URL`
- `EDI_INTAKE_URL`

Then update Netlify's `VITE_ASHN_API_URL`.

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

## Notes

- This is still a demo/learning deployment, not a HIPAA-ready production deployment.
- Render free web services may sleep when inactive; the first request can be slow.
- If you later want a dedicated async worker, add `apps/tx-worker` as a Render background worker and remove `ASHN_EMBED_WORKER=true`.
