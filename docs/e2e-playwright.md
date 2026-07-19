# Playwright E2E and Contract Tests

ASHN uses `@orieken/saturday-playwright` as the root TypeScript test harness for API contract checks and dashboard smoke coverage.

## Commands

```bash
npm run typecheck:e2e
npm run test:e2e
npm run test:e2e:services
npm run test:e2e:ui
```

`test:e2e:services` is safe by default: it checks OpenAPI docs, service health, gateway pagination, and request/correlation header propagation.

## Environment

The tests default to the Render service URLs:

```bash
ASHN_API_URL=https://ashn-api-gateway.onrender.com
ASHN_PAYER_CORE_URL=https://ashn-payer-core.onrender.com
ASHN_PROVIDER_SERVICE_URL=https://ashn-provider-service.onrender.com
ASHN_EDI_INTAKE_URL=https://ashn-edi-intake.onrender.com
```

Set `ASHN_DASHBOARD_URL` to run browser smoke coverage against the Netlify dashboard or a local Vite instance.

```bash
ASHN_DASHBOARD_URL=http://localhost:9300 npm run test:e2e:ui
```

## Mutating Demo Flows

Enrollment, eligibility, claim submission, XML/JSON/raw X12 intake, transaction export, intake audit replay, batch intake, trading partner profile management, dental XML workflows, and async adjudication tests create ledger data. They are skipped unless explicitly enabled:

```bash
ASHN_RUN_MUTATING_E2E=1 npm run test:e2e:services
```

Use the mutating flow for demos, release checks, and seeded integration environments. Keep it disabled for passive production smoke checks.

## Current Coverage Map

- **Service contracts:** OpenAPI roots, health contracts, gateway pagination, and distributed request/correlation headers.
- **Core transaction path:** `834`, `270/271`, `278`, `275`, `837`, `277`, `277CA`, `820`, and `835`-adjacent async adjudication ledger checks.
- **X12 intake:** Canonical XML, canonical JSON representation route, raw `837`, multipart batch intake, accepted audit visibility, rejected audit visibility, audit export, and audit replay.
- **Attachments:** Claim and prior-auth `275`, embedded document content download, external document reference validation, packets, and attachment review status.
- **Partner/dental workflows:** Trading partner create/update/delete and dental `278` predetermination plus `837D` claim service-line details.
