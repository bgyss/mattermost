# Agent Demo: Executive + Sales Department with Observable Coordination

This guide walks you through running a minimal two-workgroup OpenClaw agent demo on your local machine.

> **New:** Once agents are provisioned, use the **[Agent Command Center](./AGENT_COMMAND_CENTER.md)** (`/agents`) for a real-time visual overview of all agents, live token streams, delegation trees, and task management — no terminal required. See the [tutorial](./AGENT_COMMAND_CENTER_TUTORIAL.md) for a guided walkthrough.

## Architecture

```
Human (Admin)
     │ talks in
     ▼
#executive-general          ← send directives to the CEO agent here
     │
     │ delegate_task / route_to_capability
     ▼
#agentcoord-{id}-{id}       ← ChannelTypeAgentDirect — OBSERVABLE by admins
     │                          CEO ↔ Sales Head coordination channel
     │
     ▼
#sales-general              ← Sales Head agent works here
     │
     │ delegate_task
     ├──────────────────────────────────┐
     ▼                                  ▼
#sales-internal                 Prospecting Specialist / Proposal Writer
  (subagents)                        (spawned automatically)
```

### Agents provisioned

| Agent | Role | Capabilities | Default channel |
|---|---|---|---|
| CEO | executive | strategy, coordination, delegation | `#executive-general` |
| Sales Head | head | sales, account_management, pipeline | `#sales-general` |
| Prospecting Specialist | specialist | lead_generation, prospecting, outreach | `#sales-internal` |
| Proposal Writer | specialist | proposal_writing, rfp_response, pricing | `#sales-internal` |

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.24.13+ | see `server/go.mod` |
| Node.js | 24.11 | see `.nvmrc` |
| Docker | any recent | PostgreSQL via `docker compose` |
| OpenClaw gateway | any | local agent gateway on port 18789 |

### OpenClaw setup

OpenClaw is a local LLM gateway that routes agent requests to your chosen backend (Anthropic, OpenAI, Ollama, etc.). The server auto-detects it when `LLMServiceId = "openclaw"`.

```bash
# Environment variables (add to .env or export before starting the server)
export MM_AGENTS_OPENCLAW_TOKEN="your-openclaw-token"
export OPENCLAW_GATEWAY_URL="http://127.0.0.1:18789"   # default if unset
```

If you want to use Anthropic directly instead:

```bash
export MM_AGENTS_ANTHROPIC_API_KEY="sk-ant-..."
# Then pass "anthropic" as llm_service_id in the demo-setup request
```

---

## Step 1 — Start the server

```bash
cd server
make run-server
```

Or with Docker Compose for Postgres:

```bash
cd server
docker compose up -d  # starts postgres
make run-server
```

Wait for the log line:
```
{"level":"info","msg":"Server is listening","address":":8065"}
```

---

## Step 2 — Create a team and get your admin token

If this is a fresh install, complete the setup wizard at `http://localhost:8065` and create a team (e.g. **Acme**).

Retrieve your admin session token — easiest via the browser dev-tools **Network** tab after logging in, or:

```bash
TOKEN=$(curl -s -X POST http://localhost:8065/api/v4/users/login \
  -H 'Content-Type: application/json' \
  -d '{"login_id":"admin@example.com","password":"your-password"}' \
  -D - | grep -i '^token:' | awk '{print $2}' | tr -d '\r')

echo "Token: $TOKEN"
```

Get your team ID:

```bash
TEAM_ID=$(curl -s http://localhost:8065/api/v4/teams/name/acme \
  -H "Authorization: Bearer $TOKEN" | jq -r '.id')

echo "Team: $TEAM_ID"
```

---

## Step 3 — Provision the demo (one call)

`POST /api/v4/agents/demo-setup` provisions both workgroups, wires all coordination channels, and adds you (the admin) as an observer on every agent channel.

```bash
ADMIN_USER_ID=$(curl -s http://localhost:8065/api/v4/users/me \
  -H "Authorization: Bearer $TOKEN" | jq -r '.id')

DEMO=$(curl -s -X POST http://localhost:8065/api/v4/agents/demo-setup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"team_id\": \"$TEAM_ID\",
    \"observer_user_ids\": [\"$ADMIN_USER_ID\"],
    \"llm_service_id\": \"openclaw\"
  }")

echo "$DEMO" | jq .
```

Sample response:

```json
{
  "executive_workgroup": {
    "id": "wg_exec...",
    "name": "executive",
    "channel_ids": {
      "general":  "ch_exec_general...",
      "internal": "ch_exec_internal...",
      "reports":  "ch_exec_reports...",
      "coordination_channels": {
        "wg_sales...": "ch_coord..."
      }
    }
  },
  "sales_workgroup": {
    "id": "wg_sales...",
    "name": "sales",
    ...
  },
  "coordination_channel_id": "ch_coord...",
  "executive_agent_id":      "def_ceo...",
  "sales_head_agent_id":     "def_saleshead...",
  "sales_specialist_agent_ids": ["def_prospecting...", "def_proposal..."]
}
```

Save the IDs:

```bash
EXEC_AGENT_ID=$(echo "$DEMO" | jq -r '.executive_agent_id')
SALES_HEAD_ID=$(echo "$DEMO" | jq -r '.sales_head_agent_id')
COORD_CH_ID=$(echo "$DEMO"   | jq -r '.coordination_channel_id')
EXEC_GENERAL=$(echo "$DEMO"  | jq -r '.executive_workgroup.channel_ids.general')
SALES_GENERAL=$(echo "$DEMO" | jq -r '.sales_workgroup.channel_ids.general')
```

---

## Step 4 — Open the Mattermost client

```bash
# Start webapp dev server (hot reload)
cd webapp
make dev
```

Then open `http://localhost:8065` in your browser. Log in as admin.

You will see new sidebar sections:

```
EXECUTIVE OFFICE
  ▸ executive-general       ← talk to CEO here
  ▸ executive-internal
  ▸ executive-reports

SALES
  ▸ sales-general           ← talk to Sales Head here
  ▸ sales-internal
  ▸ sales-reports

OTHER CHANNELS
  ▸ agentcoord-{...}-{...}  ← CEO ↔ Sales Head coordination (observable)
```

Because you passed your user ID in `observer_user_ids`, you are already a member of all channels including the coordination channel.

---

## Step 5 — Run the demo scenarios

### Scenario A — Talk directly to the CEO

In `#executive-general`, type:

```
@ceo Please ask the Sales team to prepare a Q3 pipeline summary and identify the top 5 prospects for enterprise outreach.
```

**What happens:**
1. CEO agent receives the message via `@mention` trigger
2. CEO calls `delegate_task` targeting the Sales Head agent
3. A delegation notice appears in `#agentcoord-*` (visible to you)
4. Sales Head picks up the task in `#sales-general`
5. Sales Head calls `route_to_capability` → spawns the Prospecting Specialist for lead identification
6. Sales Head calls `route_to_capability` → spawns the Proposal Writer for pipeline report formatting
7. Each specialist posts results in `#sales-internal`
8. Sales Head synthesises and responds to CEO
9. CEO posts the final summary in `#executive-general`

### Scenario B — Talk directly to the Sales Head

In `#sales-general`, type:

```
@sales-head Draft a personalised outreach email for Acme Corp, a 500-person SaaS company focused on data analytics.
```

**What happens:**
1. Sales Head delegates to Prospecting Specialist for company research
2. Prospecting Specialist delegates to Proposal Writer for draft copy
3. Completed email is posted back in `#sales-general`

### Scenario C — Submit a task via API (non-interactive)

```bash
curl -s -X POST http://localhost:8065/api/v4/agents/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"agent_id\":   \"$EXEC_AGENT_ID\",
    \"channel_id\": \"$EXEC_GENERAL\",
    \"text\":       \"What is our current sales pipeline health? Ask the sales team.\",
    \"priority\":   50
  }" | jq .
```

Poll task status:

```bash
TASK_ID="<id from above response>"

curl -s http://localhost:8065/api/v4/agents/tasks/$TASK_ID \
  -H "Authorization: Bearer $TOKEN" | jq '{id, status, output}'
```

Get the full delegation tree:

```bash
curl -s http://localhost:8065/api/v4/agents/tasks/$TASK_ID/tree \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {id, agent_id, status, parent_task_id}]'
```

Watch task events (inner monologue + tool calls):

```bash
curl -s "http://localhost:8065/api/v4/agents/tasks/$TASK_ID/events?per_page=100" \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {event_type, payload}]'
```

---

## Step 6 — Admin visibility features

### List all observable coordination channels

```bash
curl -s "http://localhost:8065/api/v4/agents/coordination-channels?team_id=$TEAM_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {id, display_name, name}]'
```

Returns all `ChannelTypeAgentDirect` channels — these are the agent-to-agent observable channels.

### Per-agent metrics

```bash
curl -s http://localhost:8065/api/v4/agents/metrics \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Inspect Sales Head's global memory

```bash
curl -s "http://localhost:8065/api/v4/agents/$SALES_HEAD_ID/memory?scope=global" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

### Active tasks for Sales Head

```bash
curl -s http://localhost:8065/api/v4/agents/$SALES_HEAD_ID/tasks/active \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {id, status, created_at: .create_at}]'
```

---

## Step 7 — Use the Agent Command Center (UI)

All of the visibility commands above are also available in the browser. After provisioning:

1. Open `http://localhost:8065/agents` (or click **Agents** in the top-right header).
2. You will see a tile for every provisioned agent — CEO, Sales Head, and specialists.
3. Send the CEO task from Step 5 Scenario A, then watch the tiles update in real time:
   - CEO tile → "Running" (green, pulsing dot)
   - Sales Head tile → activates as soon as delegation happens
   - Specialist tiles → activate if Sales Head further delegates
4. Click the Sales Head tile → **Delegation Tree** tab to see the full parent→child chain.
5. Click any tile → **Memory** tab to inspect what the agent has remembered between tasks.
6. Click any tile → **Settings** tab to live-patch the system prompt or model.

For the full Agent Command Center guide see [AGENT_COMMAND_CENTER.md](./AGENT_COMMAND_CENTER.md).
For a step-by-step tutorial see [AGENT_COMMAND_CENTER_TUTORIAL.md](./AGENT_COMMAND_CENTER_TUTORIAL.md).

---

## Channel reference

| Channel | Type | Who posts | Visible to admin |
|---|---|---|---|
| `#executive-general` | Open | CEO agent, humans | ✅ always |
| `#executive-internal` | Private | CEO agent subprocesses | ✅ (added by demo setup) |
| `#executive-reports` | Private | CEO agent digests | ✅ (added by demo setup) |
| `#sales-general` | Open | Sales Head, humans | ✅ always |
| `#sales-internal` | Private | Specialist agents | ✅ (added by demo setup) |
| `#sales-reports` | Private | Sales Head digests | ✅ (added by demo setup) |
| `#agentcoord-{x}-{y}` | **AgentDirect** | CEO + Sales Head cross-workgroup posts | ✅ (added by demo setup + type-A bypass) |

`ChannelTypeAgentDirect` (`"A"`) is a new channel type introduced in this implementation. System admins can query all type-A channels via `GET /api/v4/agents/coordination-channels`, and the demo-setup provisioner automatically adds observer users to them.

---

## Customisation

### Use Anthropic instead of OpenClaw

```bash
curl -s -X POST http://localhost:8065/api/v4/agents/demo-setup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"team_id\": \"$TEAM_ID\",
    \"observer_user_ids\": [\"$ADMIN_USER_ID\"],
    \"llm_service_id\": \"anthropic\"
  }"
```

Make sure `MM_AGENTS_ANTHROPIC_API_KEY` is set before starting the server.

### Override agent system prompts

After provisioning, patch any agent definition:

```bash
curl -s -X PATCH http://localhost:8065/api/v4/agents/$SALES_HEAD_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "system_prompt": "You are a highly aggressive closer. Prioritise big-ticket enterprise deals above all else."
  }'
```

### Add a second coordination workgroup

Provision any additional department workgroup (e.g. Marketing) and a coordination channel will be created automatically with the Executive workgroup when provisioned into the same team:

```bash
curl -s -X POST http://localhost:8065/api/v4/workgroups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"template_id\": \"marketing\", \"team_id\": \"$TEAM_ID\"}"
```

---

## Running tests

```bash
# Model layer (no Docker needed)
cd server/public && go test ./model/... -run "TestAgent|TestWorkgroup|TestChannelTypeAgent|TestDemoSetup" -v

# Agentruntime service (no Docker needed)
cd server && go test ./platform/services/agentruntime/... -v

# Digest job (no Docker needed)
cd server && go test ./channels/jobs/agent_digest/... -v

# All agent-related quick tests
cd server && go test ./platform/services/agentruntime/... ./channels/jobs/agent_digest/... && \
  cd public && go test ./model/... -run "TestAgent|TestWorkgroup|TestChannelTypeAgent"
```

---

## Troubleshooting

| Symptom | Check |
|---|---|
| Agents don't respond | Verify `MM_AGENTS_OPENCLAW_TOKEN` is set and the gateway is running on port 18789 |
| "agent runtime not available" API errors | Confirm the server started with `AgentRuntimeService` — look for `"agent runtime started"` in server logs |
| Tasks stuck at `pending` | Check dispatcher logs; the DB poller recovers abandoned tasks every 10 seconds |
| No posts in coordination channel | Verify both workgroups' `ChannelIds.CoordinationChannels` map is populated (check via `GET /api/v4/workgroups/{id}`) |
| Coordination channel not visible | Confirm you ran demo-setup with your user ID in `observer_user_ids`, or manually join the channel via the Mattermost UI |
