#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${MM_BASE_URL:-http://localhost:8065}"
ADMIN_EMAIL="${MM_ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${MM_ADMIN_PASSWORD:?MM_ADMIN_PASSWORD must be set in .env.agents}"
TEAM_NAME="${MM_TEAM_NAME:-acme}"
LLM_SERVICE="${MM_AGENTS_LLM_SERVICE:-openclaw}"

echo "→ Logging in as $ADMIN_EMAIL..."
TOKEN=$(curl -sf -X POST "$BASE_URL/api/v4/users/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login_id\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" \
  -D - | grep -i '^token:' | awk '{print $2}' | tr -d '\r')

echo "→ Getting team ID for '$TEAM_NAME'..."
TEAM_ID=$(curl -sf "$BASE_URL/api/v4/teams/name/$TEAM_NAME" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.id')

echo "→ Getting admin user ID..."
ADMIN_USER_ID=$(curl -sf "$BASE_URL/api/v4/users/me" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.id')

echo "→ Provisioning demo (llm_service_id=$LLM_SERVICE)..."
DEMO=$(curl -sf -X POST "$BASE_URL/api/v4/agents/demo-setup" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"team_id\": \"$TEAM_ID\",
    \"observer_user_ids\": [\"$ADMIN_USER_ID\"],
    \"llm_service_id\": \"$LLM_SERVICE\"
  }")

EXEC_AGENT_ID=$(echo "$DEMO" | jq -r '.executive_agent_id')
SALES_HEAD_ID=$(echo "$DEMO" | jq -r '.sales_head_agent_id')
COORD_CH_ID=$(echo "$DEMO"   | jq -r '.coordination_channel_id')

echo ""
echo "✓ Demo provisioned!"
echo "  CEO agent ID:        $EXEC_AGENT_ID"
echo "  Sales Head agent ID: $SALES_HEAD_ID"
echo "  Coordination ch ID:  $COORD_CH_ID"
echo ""
echo "Next steps:"
echo "  1. Open http://localhost:8065 → log in as admin"
echo "  2. Go to #executive-general and type:"
echo "     @ceo Please ask the Sales team to prepare a Q3 pipeline summary."
echo "  3. Watch delegation happen live at http://localhost:8065/agents"
