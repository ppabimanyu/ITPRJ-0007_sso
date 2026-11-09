#!/usr/bin/env bash
# Runs the Playwright smoke suite against a private stack and always tears it down.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

lock_dir="${TMPDIR:-/tmp}/sso-e2e.lock"
for _ in $(seq 1 240); do
  if mkdir "$lock_dir" 2>/dev/null; then
    echo $$ >"$lock_dir/pid"
    break
  fi
  owner="$(cat "$lock_dir/pid" 2>/dev/null || true)"
  if [ -n "$owner" ] && ! kill -0 "$owner" 2>/dev/null; then
    rm -rf "$lock_dir"
    continue
  fi
  sleep 0.5
done
if [ ! -d "$lock_dir" ]; then
  echo "another e2e run is holding $lock_dir" >&2
  exit 1
fi

export E2E_PORT="${E2E_PORT:-8091}"
export E2E_DB="${E2E_DB:-sso_e2e_$(date +%s)_$$}"
stack_log="${TMPDIR:-/tmp}/sso-e2e-stack.log"
: >"$stack_log"

stack_pid=""
cleanup() {
  if [ -n "$stack_pid" ] && kill -0 "$stack_pid" 2>/dev/null; then
    kill "$stack_pid" 2>/dev/null || true
    wait "$stack_pid" 2>/dev/null || true
  fi
  docker compose exec -T postgres psql -U sso -d postgres \
    -c "DROP DATABASE IF EXISTS \"$E2E_DB\" WITH (FORCE)" >/dev/null 2>&1 || true
  rm -rf "$lock_dir"
}
trap cleanup EXIT INT TERM

bash scripts/e2e-stack.sh >"$stack_log" 2>&1 &
stack_pid=$!

for _ in $(seq 1 360); do
  if curl -fsS "http://localhost:${E2E_PORT}/healthz" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$stack_pid" 2>/dev/null; then
    wait "$stack_pid" || true
    tail -n 60 "$stack_log" >&2
    echo "e2e stack exited before becoming healthy" >&2
    exit 1
  fi
  sleep 0.5
done
if ! curl -fsS "http://localhost:${E2E_PORT}/healthz" >/dev/null 2>&1; then
  tail -n 60 "$stack_log" >&2
  echo "e2e stack did not become healthy on port ${E2E_PORT}" >&2
  exit 1
fi

cd web
E2E_BASE_URL="http://localhost:${E2E_PORT}" npx playwright test "$@"
