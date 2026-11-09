#!/usr/bin/env bash
# Starts a clean, deterministic stack for the Playwright smoke suite.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

E2E_USER_PASSWORD="${E2E_USER_PASSWORD:-E2E-User-Passw0rd!}"
E2E_ADMIN_PASSWORD="${E2E_ADMIN_PASSWORD:-E2E-Admin-Passw0rd!}"
export E2E_USER_PASSWORD E2E_ADMIN_PASSWORD

E2E_PORT="${E2E_PORT:-8091}"
E2E_DB="${E2E_DB:-sso_e2e}"
export E2E_PORT E2E_DB

export APP_ENV=dev
export HTTP_ADDR=":${E2E_PORT}"
export EXTERNAL_URL="http://localhost:${E2E_PORT}"
export LOG_LEVEL="${LOG_LEVEL:-info}"
export METRICS_ADDR="${METRICS_ADDR:-:9090}"
export DATABASE_URL="postgres://sso:dev-only-postgres@localhost:5433/${E2E_DB}?sslmode=disable"
export REDIS_URL="redis://localhost:6380/1"
export SSO_MASTER_KEYS="${SSO_MASTER_KEYS:-{\"dev-only\":\"FglevJnTk5aiM1Gmb5Epn2V6wQ1e5ZvxYHUNMBGp/sA=\"}}"
export SSO_MASTER_KEY_ID="${SSO_MASTER_KEY_ID:-dev-only}"
export ALLOWED_ORIGINS="${ALLOWED_ORIGINS:-http://localhost:${E2E_PORT},http://localhost:5173}"
export PASSWORD_MIN_LENGTH=12
export AUTH_LOGIN_IP_LIMIT="${AUTH_LOGIN_IP_LIMIT:-100}"
export SMTP_HOST="${SMTP_HOST:-localhost}"
export SMTP_PORT="${SMTP_PORT:-1025}"
export SMTP_FROM="${SMTP_FROM:-no-reply@example.com}"
export SMTP_TLS="${SMTP_TLS:-false}"
export TRUSTED_PROXY_CIDRS=""

export POSTGRES_PORT=5433
export REDIS_PORT=6380

if curl -fsS "http://localhost:${E2E_PORT}/healthz" >/dev/null 2>&1; then
  echo "port ${E2E_PORT} is already serving; stop that process before running e2e" >&2
  exit 1
fi

docker compose up -d --wait postgres redis mailhog

docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U sso -d postgres \
  -c "DROP DATABASE IF EXISTS \"${E2E_DB}\" WITH (FORCE)" \
  -c "CREATE DATABASE \"${E2E_DB}\" OWNER sso"

docker compose exec -T redis redis-cli -n 1 FLUSHDB >/dev/null

go run ./cmd/migration up
printf '%s\n' "$E2E_USER_PASSWORD" | go run ./cmd/migration seed --password-stdin
printf '%s\n' "$E2E_ADMIN_PASSWORD" | go run ./cmd/migration bootstrap \
  --email admin@example.com --password-stdin

# The consent screen only renders for third-party clients. The seeded SPA client
# stays first-party in dev; e2e flips it here so the suite pins ConsentView.
docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U sso -d "$E2E_DB" \
  -c "UPDATE client SET third_party = true WHERE client_id = 'sample-spa'"

if [ ! -d web/node_modules ]; then
  (cd web && npm ci)
fi
(cd web && npm run build)

mkdir -p internal/webui/dist
find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
cp -R web/dist/. internal/webui/dist/

go build -trimpath -o bin/e2e-server ./cmd/server
exec ./bin/e2e-server
