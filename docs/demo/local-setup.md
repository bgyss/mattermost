# Local Demo Setup (mise)

The full multi-agent demo can be spun up with `mise` tasks. No manual `export` or curl sequences needed.

## One-time setup

```bash
# 1. Copy the env template and fill in your credentials
cp .env.agents.example .env.agents
$EDITOR .env.agents
```

Minimum required values in `.env.agents`:

```bash
MM_ADMIN_PASSWORD=your-mattermost-admin-password
MM_AGENTS_OPENCLAW_TOKEN=your-openclaw-gateway-token
MM_TEAM_NAME=acme
```

See [Prerequisites](../getting-started/prerequisites.md) for details.

## Starting the stack

=== "All at once"

    ```bash
    devenv up   # starts PostgreSQL (Nix) + server + webapp together
    ```

=== "Separate terminals"

    **Terminal A:**
    ```bash
    mise run agents:db     # start PostgreSQL via devenv (no Docker)
    mise run agents:server # compile + run Mattermost
    ```

Wait for:
```
{"level":"info","msg":"Server is listening","address":":8065"}
```

**Terminal B:**
```bash
mise run agents:webapp   # webpack dev server with hot reload at http://localhost:8065
```

## Provisioning agents (one time per fresh DB)

**Terminal C:**
```bash
mise run agents:provision
```

This script (`scripts/provision-demo.sh`):

1. Logs in as admin → gets session token
2. Resolves team ID by `$MM_TEAM_NAME`
3. Gets admin user ID
4. Calls `POST /api/v4/agents/demo-setup` with `llm_service_id=$MM_AGENTS_LLM_SERVICE`
5. Prints agent IDs and next steps

## Verify agents are live

```bash
mise run agents:status
```

Expected output (4 agents):

```json
[
  {"id": "...", "display_name": "CEO", "role": "executive", "llm_service_id": "openclaw"},
  {"id": "...", "display_name": "Sales Head", "role": "head", "llm_service_id": "openclaw"},
  {"id": "...", "display_name": "Prospecting Specialist", "role": "specialist", "llm_service_id": "openclaw"},
  {"id": "...", "display_name": "Proposal Writer", "role": "specialist", "llm_service_id": "openclaw"}
]
```

## Channel structure after provisioning

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

Because you passed your user ID in `observer_user_ids`, you are automatically a member of all channels including the coordination channel.

## Agent architecture

| Agent | Role | Capabilities | Default channel |
|-------|------|-------------|----------------|
| CEO | executive | strategy, coordination, delegation | `#executive-general` |
| Sales Head | head | sales, account_management, pipeline | `#sales-general` |
| Prospecting Specialist | specialist | lead_generation, prospecting, outreach | `#sales-internal` |
| Proposal Writer | specialist | proposal_writing, rfp_response, pricing | `#sales-internal` |

## Mise task reference

| Task | Description |
|------|-------------|
| `devenv up` | Start PostgreSQL + server + webapp (Nix, no Docker) |
| `mise run agents:db` | Start PostgreSQL only (via devenv) |
| `mise run agents:server` | Run Mattermost server |
| `mise run agents:webapp` | Start webpack dev server |
| `mise run agents:provision` | Provision CEO + Sales demo |
| `mise run agents:status` | List all agent definitions |

## Environment variables loaded automatically

`mise` loads `.env.agents` automatically (via `_.file = ".env.agents"` in `.mise.toml`):

| Variable | Default | Purpose |
|----------|---------|---------|
| `MM_AGENTS_OPENCLAW_TOKEN` | — | OpenClaw gateway token |
| `OPENCLAW_GATEWAY_URL` | `http://127.0.0.1:18789` | Gateway URL |
| `MM_FEATUREFLAGS_ENABLEAIAGENTS` | `true` | Feature flag |
| `MM_ADMIN_EMAIL` | `admin@example.com` | Admin login |
| `MM_ADMIN_PASSWORD` | — | Admin password |
| `MM_TEAM_NAME` | `acme` | Team to provision into |
| `MM_AGENTS_LLM_SERVICE` | `openclaw` | Provider for demo agents |
