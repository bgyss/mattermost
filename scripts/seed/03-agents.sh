#!/usr/bin/env bash
# seed/03-agents.sh — provision CEO + Sales demo agents via REST API.
#
# Skipped automatically if MM_AGENTS_OPENCLAW_TOKEN is unset (no LLM configured).
# Can also be skipped explicitly with MM_SKIP_AGENTS=1 in ci-up.sh.
set -euo pipefail

BASE_URL="${MM_BASE_URL:-http://localhost:8065}"
ADMIN_EMAIL="${MM_ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${MM_ADMIN_PASSWORD:-Admin1234!}"
TEAM_NAME="${MM_TEAM_NAME:-mattermost-dev}"
LLM_SERVICE="${MM_AGENTS_LLM_SERVICE:-openclaw}"

# Skip gracefully if no LLM token is configured
if [ -z "${MM_AGENTS_OPENCLAW_TOKEN:-}${MM_AGENTS_ANTHROPIC_API_KEY:-}${MM_AGENTS_OPENAI_API_KEY:-}" ]; then
  echo "   [seed/03] No LLM token configured — skipping agent provisioning."
  echo "   Set MM_AGENTS_OPENCLAW_TOKEN (or ANTHROPIC/OPENAI key) in .env.agents to enable."
  exit 0
fi

echo "   [seed/03] Provisioning demo agents (llm_service=$LLM_SERVICE)..."

# Login (re-auth; 02-team.sh export doesn't cross process boundaries)
TOKEN=$(curl -sf -X POST "$BASE_URL/api/v4/users/login" \
  -H 'Content-Type: application/json' \
  -d "{\"login_id\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}" \
  -D - | grep -i '^token:' | awk '{print $2}' | tr -d '\r')

TEAM_ID=$(curl -sf "$BASE_URL/api/v4/teams/name/$TEAM_NAME" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.id')

ADMIN_USER_ID=$(curl -sf "$BASE_URL/api/v4/users/me" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.id')

RESPONSE=$(curl -sf -X POST "$BASE_URL/api/v4/agents/demo-setup" \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{
    \"team_id\":           \"$TEAM_ID\",
    \"observer_user_ids\": [\"$ADMIN_USER_ID\"],
    \"llm_service_id\":    \"$LLM_SERVICE\"
  }")

EXEC_AGENT_ID=$(echo "$RESPONSE"  | jq -r '.executive_agent_id // empty')
SALES_HEAD_ID=$(echo "$RESPONSE"  | jq -r '.sales_head_agent_id // empty')
COORD_CH_ID=$(echo "$RESPONSE"    | jq -r '.coordination_channel_id // empty')

if [ -z "$EXEC_AGENT_ID" ]; then
  echo "   Warning: demo-setup returned unexpected response:"
  echo "   $RESPONSE"
  echo "   Agents may not be available — continuing without them."
  exit 0
fi

echo "   CEO agent:        $EXEC_AGENT_ID"
echo "   Sales Head agent: $SALES_HEAD_ID"
echo "   Coordination ch:  $COORD_CH_ID"
