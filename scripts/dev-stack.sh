#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="$ROOT_DIR/.dev/logs"
DATABASE_URL="${DATABASE_URL:-postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable}"
ASHN_API_URL="${ASHN_API_URL:-http://localhost:8080}"

mkdir -p "$LOG_DIR"

PIDS=()

cleanup() {
  echo
  echo "[ASHN] Shutting down local services..."
  for pid in "${PIDS[@]:-}"; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
  wait >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

wait_for_url() {
  local url="$1"
  local name="$2"
  local attempts="${3:-40}"

  for _ in $(seq 1 "$attempts"); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "[ASHN] $name is ready"
      return 0
    fi
    sleep 0.5
  done

  echo "[ASHN] Timed out waiting for $name at $url" >&2
  return 1
}

start_service() {
  local name="$1"
  shift
  echo "[ASHN] Starting $name..."
  (
    cd "$ROOT_DIR"
    "$@"
  ) >"$LOG_DIR/$name.log" 2>&1 &
  PIDS+=("$!")
}

echo "[ASHN] Starting Postgres..."
(
  cd "$ROOT_DIR"
  make db
  make db-wait
  make migrate
)

start_service payer-core env DATABASE_URL="$DATABASE_URL" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/payer-core
start_service provider-service env DATABASE_URL="$DATABASE_URL" PAYER_CORE_URL="http://localhost:8081" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/provider-service
start_service edi-intake env PAYER_CORE_URL="http://localhost:8081" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/edi-intake
start_service api-gateway env PAYER_CORE_URL="http://localhost:8081" PROVIDER_SERVICE_URL="http://localhost:8082" EDI_INTAKE_URL="http://localhost:8083" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/api-gateway
start_service dashboard npm --prefix apps/dashboard run dev

wait_for_url "http://localhost:8081/health" "payer-core"
wait_for_url "http://localhost:8082/health" "provider-service"
wait_for_url "http://localhost:8083/health" "edi-intake"
wait_for_url "$ASHN_API_URL/v1/health" "api-gateway"
wait_for_url "http://localhost:9300" "dashboard"

cat <<EOF

[ASHN] Local stack is ready.
  Dashboard:   http://localhost:9300
  API Gateway: $ASHN_API_URL
  Logs:        $LOG_DIR

Run the demo in another terminal:
  make demo

Press Ctrl-C to stop services. Postgres remains running in Docker.
EOF

while true; do
  sleep 3600
done
