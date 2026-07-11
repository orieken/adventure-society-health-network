# ASHN Dashboard

React/Vite dashboard for the Adventure Society Health Network.

## Run

```sh
npm install
npm run dev
```

The dashboard runs on `http://localhost:9300` and talks to `VITE_ASHN_API_URL`, defaulting to `http://localhost:8080`.

If `api-gateway` has `ASHN_API_KEYS` enabled, set `VITE_ASHN_API_KEY` so dashboard API calls and exports include `X-ASHN-API-Key`.
