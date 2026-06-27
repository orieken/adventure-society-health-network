#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ATTEMPTS="${1:-60}"

cd "$ROOT_DIR"

for _ in $(seq 1 "$ATTEMPTS"); do
  if docker compose -f infra/docker-compose.yml exec -T postgres pg_isready -U ashn_user -d ashn >/dev/null 2>&1; then
    echo "[ASHN] Postgres is ready"
    exit 0
  fi
  sleep 1
done

echo "[ASHN] Timed out waiting for Postgres" >&2
exit 1
