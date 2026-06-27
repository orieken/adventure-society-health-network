#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

docker compose -f infra/docker-compose.yml exec -T postgres psql -U ashn_user -d ashn <<'SQL'
SELECT 'adventurers' AS table_name, count(*) FROM adventurers
UNION ALL SELECT 'providers', count(*) FROM providers
UNION ALL SELECT 'transactions', count(*) FROM transactions
UNION ALL SELECT 'claims', count(*) FROM claims
UNION ALL SELECT 'enrollments', count(*) FROM enrollments
UNION ALL SELECT 'premium_payments', count(*) FROM premium_payments
UNION ALL SELECT 'auth_requests', count(*) FROM auth_requests
ORDER BY table_name;
SQL
