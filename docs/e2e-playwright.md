# Playwright E2E and Contract Tests

ASHN uses `@orieken/saturday-playwright` as the root TypeScript test harness for API contract checks and dashboard smoke coverage.

## Commands

```bash
npm run typecheck:e2e
npm run test:e2e
npm run test:e2e:services
npm run test:e2e:ui
```

`test:e2e:services` is safe by default: it checks OpenAPI docs, service health, and gateway read contracts.

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

Enrollment, eligibility, claim submission, and XML intake tests create ledger data. They are skipped unless explicitly enabled:

```bash
ASHN_RUN_MUTATING_E2E=1 npm run test:e2e:services
```

Use the mutating flow for demos, release checks, and seeded integration environments. Keep it disabled for passive production smoke checks.
