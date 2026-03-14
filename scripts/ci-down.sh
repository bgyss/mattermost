#!/usr/bin/env bash
# ci-down.sh — stop all CI processes and wipe ephemeral state (postgres data dir, logs, pids).
#
# Safe to run even if environment is partially up — every step is best-effort.
# Usage:  mise run ci:down
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PGDATA="${CI_PGDATA:-$ROOT_DIR/.devenv/state/postgres}"
PG_PORT="${MM_PG_PORT:-5432}"
PG_HOST="${MM_PG_HOST:-127.0.0.1}"
SERVER_PID_FILE="$ROOT_DIR/.devenv/server.pid"

echo "→ Stopping Mattermost server..."
if [ -f "$SERVER_PID_FILE" ]; then
  SERVER_PID=$(cat "$SERVER_PID_FILE")
  if kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    # Wait up to 10s for graceful shutdown
    for i in $(seq 1 10); do
      kill -0 "$SERVER_PID" 2>/dev/null || break
      sleep 1
    done
    kill -9 "$SERVER_PID" 2>/dev/null || true
  fi
  rm -f "$SERVER_PID_FILE"
fi
# Catch any stray processes (e.g. if PID file was missing)
pkill -f "bin/mattermost" 2>/dev/null || true
pkill -f "cmd/mattermost" 2>/dev/null || true

echo "→ Stopping PostgreSQL..."
if command -v pg_ctl &>/dev/null && [ -d "$PGDATA" ]; then
  pg_ctl -D "$PGDATA" stop -m fast 2>/dev/null || true
fi
# Fallback: kill by port
if command -v lsof &>/dev/null; then
  lsof -ti tcp:"$PG_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
fi

echo "→ Wiping ephemeral state..."
rm -rf "$PGDATA"
rm -f  "$ROOT_DIR/.devenv/server.log" \
       "$ROOT_DIR/.devenv/postgres.log" \
       "$ROOT_DIR/.devenv/server.pid"

echo "✓ CI environment torn down"
