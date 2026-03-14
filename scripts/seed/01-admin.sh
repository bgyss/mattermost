#!/usr/bin/env bash
# seed/01-admin.sh — create the first system admin account via REST API.
#
# Mattermost automatically grants system_admin to the first user created when
# there are no existing accounts, so no special flag is required.
set -euo pipefail

BASE_URL="${MM_BASE_URL:-http://localhost:8065}"
ADMIN_EMAIL="${MM_ADMIN_EMAIL:-admin@example.com}"
ADMIN_USERNAME="${MM_ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${MM_ADMIN_PASSWORD:-Admin1234!}"

echo "   [seed/01] Creating admin user ($ADMIN_EMAIL)..."

RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v4/users" \
  -H 'Content-Type: application/json' \
  -d "{
    \"email\":            \"$ADMIN_EMAIL\",
    \"username\":         \"$ADMIN_USERNAME\",
    \"password\":         \"$ADMIN_PASSWORD\",
    \"allow_marketing\":  false
  }" 2>&1) || {
    echo "   Error creating admin user. Response: $RESPONSE"
    exit 1
  }

USER_ID=$(echo "$RESPONSE" | jq -r '.id // empty')
if [ -z "$USER_ID" ]; then
  echo "   Error: unexpected response from /api/v4/users:"
  echo "   $RESPONSE"
  exit 1
fi

echo "   Admin user created: id=$USER_ID username=$ADMIN_USERNAME"
