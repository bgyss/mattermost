#!/usr/bin/env bash
# seed/02-team.sh — create the default team and add the admin user.
#
# The creating user is automatically added as team admin by Mattermost.
set -euo pipefail

BASE_URL="${MM_BASE_URL:-http://localhost:8065}"
ADMIN_EMAIL="${MM_ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${MM_ADMIN_PASSWORD:-Admin1234!}"
TEAM_NAME="${MM_TEAM_NAME:-mattermost-dev}"
TEAM_DISPLAY_NAME="${MM_TEAM_DISPLAY_NAME:-Mattermost Dev}"

echo "   [seed/02] Logging in as $ADMIN_EMAIL..."
TOKEN=$(curl -sf -X POST "$BASE_URL/api/v4/users/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login_id\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" \
  -D - | grep -i '^token:' | awk '{print $2}' | tr -d '\r')

if [ -z "$TOKEN" ]; then
  echo "   Error: could not obtain session token for $ADMIN_EMAIL"
  exit 1
fi

echo "   [seed/02] Creating team '$TEAM_NAME'..."
RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v4/teams" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"name\":         \"$TEAM_NAME\",
    \"display_name\": \"$TEAM_DISPLAY_NAME\",
    \"type\":         \"O\"
  }")

TEAM_ID=$(echo "$RESPONSE" | jq -r '.id // empty')
if [ -z "$TEAM_ID" ]; then
  echo "   Error: unexpected response from /api/v4/teams:"
  echo "   $RESPONSE"
  exit 1
fi

echo "   Team created: id=$TEAM_ID name=$TEAM_NAME"

# Persist token for downstream seed scripts (03-agents.sh etc.)
export MM_CI_TOKEN="$TOKEN"
export MM_CI_TEAM_ID="$TEAM_ID"
