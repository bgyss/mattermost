# Prerequisites

## Required tools

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.24.13 | `mise install` (from `.mise.toml`) |
| Node.js | 24.11 | `mise install` |
| Docker | any recent | [docs.docker.com](https://docs.docker.com/get-docker/) |
| jq | any | `brew install jq` / `apt install jq` |
| Python | 3.11+ | System or `mise install python` — needed only for MkDocs |

## OpenClaw gateway

OpenClaw is a local LLM gateway that proxies agent requests to an upstream provider (Anthropic, OpenAI, Ollama, etc.).

- Default URL: `http://127.0.0.1:18789`
- Required config: `gateway.http.endpoints.chatCompletions.enabled: true`
- Token: set `MM_AGENTS_OPENCLAW_TOKEN` in `.env.agents`

The server auto-routes via OpenClaw when an agent's `llm_service_id` is `"openclaw"` (the default).

## Alternative: Anthropic direct

If you don't have OpenClaw, use Anthropic directly:

```bash
# In .env.agents:
MM_AGENTS_ANTHROPIC_API_KEY=sk-ant-...
MM_AGENTS_LLM_SERVICE=anthropic
```

Pass `"llm_service_id": "anthropic"` when calling `agents:provision` or `POST /api/v4/agents/demo-setup`.

## Mattermost admin account

`agents:provision` logs in as the admin user to call the provisioning API. Set:

```bash
MM_ADMIN_EMAIL=admin@example.com   # default
MM_ADMIN_PASSWORD=your-password
MM_TEAM_NAME=acme                  # team to provision agents into
```

The first user created via the setup wizard gets `system_admin` automatically.

## Environment file

Copy the template and fill in your values:

```bash
cp .env.agents.example .env.agents
```

`mise` auto-loads `.env.agents` (via `_.file = ".env.agents"` in `.mise.toml`) whenever any task runs, so you never need to `export` vars manually.

!!! warning "Never commit `.env.agents`"
    `.env.agents` is gitignored. Commit only `.env.agents.example` with placeholder values.
