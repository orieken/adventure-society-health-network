#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATABASE_URL="${DATABASE_URL:-postgres://ashn_user:ashn_password@localhost:5432/ashn?sslmode=disable}"
LOG_DIR="$ROOT_DIR/.dev/integration-logs"
API_URL="http://localhost:18080"

mkdir -p "$LOG_DIR" "$ROOT_DIR/.gocache"

PIDS=()

cleanup() {
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
  for _ in $(seq 1 60); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      echo "[ASHN] $name is ready"
      return 0
    fi
    sleep 0.5
  done
  echo "[ASHN] Timed out waiting for $name at $url" >&2
  return 1
}

sql_value() {
  docker compose -f "$ROOT_DIR/infra/docker-compose.yml" exec -T postgres \
    psql -U ashn_user -d ashn -Atc "$1"
}

echo "[ASHN] Resetting schema for XML intake integration..."
docker compose -f "$ROOT_DIR/infra/docker-compose.yml" exec -T postgres \
  psql -U ashn_user -d ashn -f /migrations/000001_init.down.sql >/dev/null
docker compose -f "$ROOT_DIR/infra/docker-compose.yml" exec -T postgres \
  psql -U ashn_user -d ashn -f /migrations/000001_init.up.sql >/dev/null

echo "[ASHN] Starting integration services..."
(
  cd "$ROOT_DIR"
  env DATABASE_URL="$DATABASE_URL" PAYER_CORE_ADDR=":18081" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/payer-core
) >"$LOG_DIR/payer-core.log" 2>&1 &
PIDS+=("$!")

(
  cd "$ROOT_DIR"
  env DATABASE_URL="$DATABASE_URL" PAYER_CORE_URL="http://localhost:18081" EDI_INTAKE_ADDR=":18083" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/edi-intake
) >"$LOG_DIR/edi-intake.log" 2>&1 &
PIDS+=("$!")

(
  cd "$ROOT_DIR"
  env PAYER_CORE_URL="http://localhost:18081" PROVIDER_SERVICE_URL="" EDI_INTAKE_URL="http://localhost:18083" API_GATEWAY_ADDR=":18080" GOCACHE="$ROOT_DIR/.gocache" go run ./apps/api-gateway
) >"$LOG_DIR/api-gateway.log" 2>&1 &
PIDS+=("$!")

wait_for_url "http://localhost:18081/health" "payer-core"
wait_for_url "http://localhost:18083/health" "edi-intake"
wait_for_url "$API_URL/v1/health" "api-gateway"

accepted_xml="$(mktemp)"
rejected_xml="$(mktemp)"
cat >"$accepted_xml" <<'XML'
<AshnX12Transaction type="834">
  <Sender id="partner-greenstone" />
  <Receiver id="Adventure Society" />
  <Enrollment>
    <Name>XML Integration Farros</Name>
    <Rank>Iron</Rank>
    <Guild>Grim Foundations</Guild>
    <Region>Greenstone</Region>
  </Enrollment>
</AshnX12Transaction>
XML

cat >"$rejected_xml" <<'XML'
<AshnX12Transaction type="270">
  <EligibilityInquiry>
    <AdventurerId>missing-provider</AdventurerId>
  </EligibilityInquiry>
</AshnX12Transaction>
XML

accepted_response="$(mktemp)"
accepted_status="$(curl -sS -o "$accepted_response" -w "%{http_code}" \
  -H "Content-Type: application/xml" \
  --data-binary "@$accepted_xml" \
  "$API_URL/v1/x12/xml")"
if [[ "$accepted_status" != "201" ]]; then
  echo "[ASHN] Expected accepted XML status 201, got $accepted_status" >&2
  cat "$accepted_response" >&2
  exit 1
fi

rejected_response="$(mktemp)"
rejected_status="$(curl -sS -o "$rejected_response" -w "%{http_code}" \
  -H "Content-Type: application/xml" \
  --data-binary "@$rejected_xml" \
  "$API_URL/v1/x12/xml")"
if [[ "$rejected_status" != "400" ]]; then
  echo "[ASHN] Expected rejected XML status 400, got $rejected_status" >&2
  cat "$rejected_response" >&2
  exit 1
fi

accepted_count="$(sql_value "SELECT count(*) FROM inbound_messages WHERE transaction_type = '834' AND status = 'accepted' AND downstream_status = 201 AND raw_payload LIKE '%XML Integration Farros%';")"
rejected_count="$(sql_value "SELECT count(*) FROM inbound_messages WHERE transaction_type = '270' AND status = 'rejected' AND error LIKE 'missing field%' AND raw_payload LIKE '%missing-provider%';")"
transaction_count="$(sql_value "SELECT count(*) FROM transactions WHERE type = '834' AND status = 'Accepted';")"
adventurer_count="$(sql_value "SELECT count(*) FROM adventurers WHERE name = 'XML Integration Farros';")"

if [[ "$accepted_count" != "1" || "$rejected_count" != "1" || "$transaction_count" != "1" || "$adventurer_count" != "1" ]]; then
  echo "[ASHN] XML intake integration assertions failed" >&2
  echo "accepted_count=$accepted_count rejected_count=$rejected_count transaction_count=$transaction_count adventurer_count=$adventurer_count" >&2
  exit 1
fi

messages_response="$(mktemp)"
messages_status="$(curl -sS -o "$messages_response" -w "%{http_code}" "$API_URL/v1/x12/messages?limit=10&status=accepted&type=834&q=Farros")"
if [[ "$messages_status" != "200" ]]; then
  echo "[ASHN] Expected XML messages status 200, got $messages_status" >&2
  cat "$messages_response" >&2
  exit 1
fi

node - "$messages_response" <<'NODE'
const fs = require("fs");
const payload = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));
if (!payload.page || payload.page.count !== 1) {
  throw new Error(`expected one accepted XML audit message, got ${JSON.stringify(payload.page)}`);
}
const [message] = payload.data;
if (message.transactionType !== "834" || message.status !== "accepted" || !message.rawPayload.includes("XML Integration Farros")) {
  throw new Error(`unexpected accepted XML audit message: ${JSON.stringify(message)}`);
}
NODE

rejected_messages_response="$(mktemp)"
rejected_messages_status="$(curl -sS -o "$rejected_messages_response" -w "%{http_code}" "$API_URL/v1/x12/messages?limit=10&status=rejected&type=270&q=missing-provider")"
if [[ "$rejected_messages_status" != "200" ]]; then
  echo "[ASHN] Expected rejected XML messages status 200, got $rejected_messages_status" >&2
  cat "$rejected_messages_response" >&2
  exit 1
fi

node - "$rejected_messages_response" <<'NODE'
const fs = require("fs");
const payload = JSON.parse(fs.readFileSync(process.argv[2], "utf8"));
if (!payload.page || payload.page.count !== 1) {
  throw new Error(`expected one rejected XML audit message, got ${JSON.stringify(payload.page)}`);
}
const [message] = payload.data;
if (message.transactionType !== "270" || message.status !== "rejected" || !message.error.includes("missing field")) {
  throw new Error(`unexpected rejected XML audit message: ${JSON.stringify(message)}`);
}
NODE

echo "[ASHN] XML intake integration passed"
