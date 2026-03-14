#!/usr/bin/env bash
# ci-up.sh — spin up a clean, fully-seeded Mattermost environment for CI or local testing.
#
# Requires: pg_ctl, psql, initdb, go — all provided by `devenv shell` / `nix develop`.
# Usage:    mise run ci:up
#           MM_SKIP_AGENTS=1 mise run ci:up   # skip agent provisioning
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PGDATA="${CI_PGDATA:-$ROOT_DIR/.devenv/state/postgres}"
PG_PORT="${MM_PG_PORT:-5432}"
PG_HOST="${MM_PG_HOST:-127.0.0.1}"
PG_LOG="$ROOT_DIR/.devenv/postgres.log"
SERVER_LOG="$ROOT_DIR/.devenv/server.log"
SERVER_PID_FILE="$ROOT_DIR/.devenv/server.pid"

BASE_URL="${MM_BASE_URL:-http://localhost:8065}"

# ── Pre-flight checks ─────────────────────────────────────────────────────────

for cmd in pg_ctl psql initdb go; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "Error: '$cmd' not found. Enter the devenv shell first:"
    echo "  direnv allow   # or: devenv shell"
    exit 1
  fi
done

if [ -f "$SERVER_PID_FILE" ] && kill -0 "$(cat "$SERVER_PID_FILE")" 2>/dev/null; then
  echo "Error: Mattermost server is already running (PID $(cat "$SERVER_PID_FILE"))."
  echo "Run 'mise run ci:down' first."
  exit 1
fi

if pg_isready -h "$PG_HOST" -p "$PG_PORT" &>/dev/null; then
  echo "Error: PostgreSQL is already accepting connections on $PG_HOST:$PG_PORT."
  echo "Run 'mise run ci:down' first, or stop the existing postgres."
  exit 1
fi

# ── 1. PostgreSQL ─────────────────────────────────────────────────────────────

echo "→ [1/5] Initialising PostgreSQL data directory..."
mkdir -p "$(dirname "$PGDATA")"
initdb -D "$PGDATA" --username=postgres --auth-local=trust --auth-host=trust -q

echo "→ [1/5] Starting PostgreSQL..."
pg_ctl -D "$PGDATA" -l "$PG_LOG" -o "-h $PG_HOST -p $PG_PORT" start -w

echo "→ [1/5] Creating mmuser + mattermost_test..."
psql -h "$PG_HOST" -p "$PG_PORT" -U postgres -c \
  "CREATE USER mmuser WITH PASSWORD 'mostest' CREATEDB;" >/dev/null
psql -h "$PG_HOST" -p "$PG_PORT" -U postgres -c \
  "CREATE DATABASE mattermost_test OWNER mmuser;" >/dev/null
psql -h "$PG_HOST" -p "$PG_PORT" -U postgres -d mattermost_test -c \
  "GRANT ALL ON SCHEMA public TO mmuser; ALTER DATABASE mattermost_test OWNER TO mmuser;" >/dev/null

# ── 2. Build server binary ────────────────────────────────────────────────────

echo "→ [2/5] Building Mattermost server binary..."
(cd "$ROOT_DIR/server" && go build -trimpath \
  -ldflags '-X github.com/mattermost/mattermost/server/public/model.BuildNumber=ci' \
  -o bin/mattermost ./cmd/mattermost)

# ── 3. Start server ───────────────────────────────────────────────────────────

echo "→ [3/5] Starting Mattermost server..."
export MM_SQLSETTINGS_DATASOURCE="postgres://mmuser:mostest@${PG_HOST}/mattermost_test?sslmode=disable&connect_timeout=10&binary_parameters=yes"
export MM_SQLSETTINGS_DRIVERNAME="postgres"
export MM_SERVICESETTINGS_SITEURL="$BASE_URL"
export MM_SERVICESETTINGS_ENABLELOCALMODE="true"
export MM_EMAILSETTINGS_REQUIREEMAILVERIFICATION="false"
export MM_EMAILSETTINGS_SMTPSERVER=""
export MM_EMAILSETTINGS_SENDPUSHNOTIFICATIONS="false"
export MM_LOGSETTINGS_CONSOLELEVEL="INFO"
export MM_FEATUREFLAGS_ENABLEAIAGENTS="true"

nohup "$ROOT_DIR/server/bin/mattermost" > "$SERVER_LOG" 2>&1 &
echo $! > "$SERVER_PID_FILE"

echo "→ [3/5] Waiting for server to accept connections..."
for i in $(seq 1 60); do
  if curl -sf "$BASE_URL/api/v4/system/ping" 2>/dev/null | grep -q '"status":"OK"'; then
    echo "   Server ready (${i}×2s)"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "Error: server did not start within 120s. Logs:"
    tail -20 "$SERVER_LOG"
    exit 1
  fi
  sleep 2
done

# ── 4. Seed fixtures ──────────────────────────────────────────────────────────

echo "→ [4/5] Seeding fixtures..."
bash "$SCRIPT_DIR/seed/01-admin.sh"
bash "$SCRIPT_DIR/seed/02-team.sh"

if [ "${MM_SKIP_AGENTS:-0}" != "1" ]; then
  bash "$SCRIPT_DIR/seed/03-agents.sh"
else
  echo "   Skipping agent provisioning (MM_SKIP_AGENTS=1)"
fi

# ── 5. Done ───────────────────────────────────────────────────────────────────

echo ""
echo "✓ CI environment ready"
echo "  Web client:  $BASE_URL"
echo "  Admin:       ${MM_ADMIN_EMAIL:-admin@example.com} / ${MM_ADMIN_PASSWORD:-Admin1234!}"
echo "  Team:        ${MM_TEAM_NAME:-mattermost-dev}"
echo "  Server log:  $SERVER_LOG"
echo ""
echo "  Run 'mise run ci:down' to tear everything down."
