#!/usr/bin/env bash
# Disposable verification stack for ITPRJ-0007 SSO.
#
#   stack.sh up       start the stack and wait for /healthz
#   stack.sh down     stop the server this helper started and drop its database
#   stack.sh doctor   read-only health report
#   stack.sh status   print the recorded port, pid, database, and log path
#
# The stack itself comes from the repo's scripts/e2e-stack.sh; this wrapper only
# records what it started so cleanup never kills a process it does not own.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../../../.." && pwd)"
state_dir="$repo_root/.audit/verify"
state_file="$state_dir/stack.env"
default_port="${E2E_PORT:-8093}"

die() {
  echo "verify-stack: $*" >&2
  exit 1
}

load_state() {
  PORT=""
  PID=""
  DB=""
  LOG=""
  if [ -f "$state_file" ]; then
    # shellcheck disable=SC1090
    . "$state_file"
  fi
}

healthy() {
  [ -n "${1:-}" ] && curl -fsS --max-time 2 "http://localhost:$1/healthz" >/dev/null 2>&1
}

cmd_up() {
  load_state
  if [ -n "$PORT" ] && healthy "$PORT"; then
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
      echo "verify stack already up on http://localhost:$PORT (pid $PID, db $DB)"
      return 0
    fi
    echo "port $PORT serves /healthz but the recorded process is gone; run: $0 down" >&2
    exit 1
  fi

  PORT="$default_port"
  if healthy "$PORT"; then
    die "port $PORT is already serving; choose another with E2E_PORT=<port>"
  fi

  mkdir -p "$state_dir"
  DB="sso_verify_${PORT}_$(date +%s)"
  LOG="$state_dir/stack-${PORT}.log"
  : >"$LOG"

  echo "starting verify stack on :$PORT with database $DB"
  cd "$repo_root"
  E2E_PORT="$PORT" E2E_DB="$DB" nohup bash scripts/e2e-stack.sh >>"$LOG" 2>&1 &
  PID=$!
  printf 'PORT=%s\nPID=%s\nDB=%s\nLOG=%s\n' "$PORT" "$PID" "$DB" "$LOG" >"$state_file"

  for _ in $(seq 1 600); do
    if healthy "$PORT"; then
      echo "verify stack ready on http://localhost:$PORT (pid $PID, db $DB, log $LOG)"
      return 0
    fi
    if ! kill -0 "$PID" 2>/dev/null; then
      tail -n 40 "$LOG" >&2 || true
      die "stack exited before becoming healthy (log: $LOG)"
    fi
    sleep 1
  done
  tail -n 40 "$LOG" >&2 || true
  die "stack did not become healthy within 600s (log: $LOG)"
}

cmd_down() {
  load_state
  if [ -z "$PORT" ]; then
    echo "no recorded verify stack"
    return 0
  fi
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    for _ in $(seq 1 20); do
      kill -0 "$PID" 2>/dev/null || break
      sleep 0.5
    done
    if kill -0 "$PID" 2>/dev/null; then
      kill -9 "$PID" 2>/dev/null || true
    fi
  fi
  if [ -n "$DB" ]; then
    (
      cd "$repo_root"
      docker compose exec -T postgres psql -U sso -d postgres \
        -c "DROP DATABASE IF EXISTS \"$DB\" WITH (FORCE)" >/dev/null 2>&1
    ) || true
  fi
  rm -f "$state_file"
  echo "verify stack stopped; database ${DB:-none} dropped; evidence in $state_dir remains"
}

cmd_doctor() {
  load_state
  if [ -z "$PORT" ]; then
    echo "no verify stack recorded; run: $0 up" >&2
    exit 1
  fi
  local health ready pid_state
  echo "port: $PORT"
  if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    pid_state="alive"
  else
    pid_state="dead"
  fi
  echo "pid: ${PID:-?} ($pid_state)"
  echo "db: ${DB:-?}"
  echo "log: ${LOG:-?}"
  health="$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "http://localhost:$PORT/healthz" || true)"
  ready="$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "http://localhost:$PORT/readyz" || true)"
  echo "healthz: ${health:-unreachable}"
  echo "readyz: ${ready:-unreachable}"
  echo "port owner:"
  lsof -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null || echo "  (nobody is listening)"
  echo "containers:"
  (
    cd "$repo_root"
    docker compose ps postgres redis mailhog 2>/dev/null
  ) || echo "  (docker unavailable)"
  echo "setup state:"
  ls "$repo_root/web/e2e/.auth" 2>/dev/null || echo "  (none; the Playwright setup project recreates it)"

  case "$health" in
    200)
      case "$ready" in
        200) return 0 ;;
        *) echo "verify-stack: readyz is ${ready:-unreachable}; the app cannot reach Postgres or Redis" >&2; exit 1 ;;
      esac
      ;;
    *)
      echo "verify-stack: healthz is ${health:-unreachable}; run: $0 down && $0 up" >&2
      exit 1
      ;;
  esac
}

cmd_status() {
  load_state
  if [ -z "$PORT" ]; then
    echo "no verify stack recorded"
    return 0
  fi
  echo "PORT=$PORT"
  echo "PID=$PID"
  echo "DB=$DB"
  echo "LOG=$LOG"
}

case "${1:-}" in
  up) cmd_up ;;
  down) cmd_down ;;
  doctor) cmd_doctor ;;
  status) cmd_status ;;
  *)
    echo "usage: $0 {up|down|doctor|status}" >&2
    exit 2
    ;;
esac
