# ASHN Testkit

Feature files describe the core ASHN transaction flows. The Go test file contains lightweight executable contract checks.

Run unit/contract checks:

```sh
go test ./...
```

Run the full stack manually:

```sh
go run ./apps/payer-core
go run ./apps/provider-service
go run ./apps/api-gateway
ASHN_API_URL=http://localhost:8080 go test ./packages/testkit
```

