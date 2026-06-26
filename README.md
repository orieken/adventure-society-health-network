# ASHN — Adventure Society Health Network

ASHN is a lore-themed EDI healthcare transaction simulator. Phase 1 uses structured JSON payloads that mirror X12 transaction flow.

## Quick Start

```sh
cp .env.example .env
make build
make test
```

Run services in separate terminals:

```sh
go run ./apps/payer-core
go run ./apps/provider-service
go run ./apps/api-gateway
go run ./apps/ashn-cli providers list
```

