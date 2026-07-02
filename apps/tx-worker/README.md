# tx-worker

Asynchronous EDI workflow processor for ASHN.

The worker polls the Postgres-backed `transaction_jobs` table and moves long-running workflow decisions off the request path.

Current jobs:

- `auth_review` — reviews queued `278` prior authorization requests and updates their transaction/auth status.
- `claim_adjudication` — moves submitted claims into `Pending` review.
- `claim_finalization` — completes claim review as `Approved` or `Denied` and emits a related `277` status transaction.

## Run

```sh
export DATABASE_URL="postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable"
go run ./apps/tx-worker
```

`make dev-stack` starts this worker automatically.
