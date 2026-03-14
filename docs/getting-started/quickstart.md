# Quickstart

The fastest path from zero to a running multi-agent demo. Tooling is managed by **Nix + devenv** — no Docker required.

## 1. Enter the Nix dev shell

```bash
nix develop     # downloads all tools (Go, Node, Python, postgres…) — first run ~2 min
# or equivalently:
devenv shell
```

Everything — Go 1.24, Node 24, Python 3.12, golangci-lint, mockery, uv, jujutsu — is provided by `devenv.nix`.

## 2. Install Python/docs deps (optional)

```bash
mise run docs:install   # creates .venv and installs mkdocs-material via uv
```

## 3. Configure your credentials

```bash
cp .env.agents.example .env.agents
$EDITOR .env.agents
```

Minimum required values:

```bash
MM_ADMIN_PASSWORD=your-mattermost-admin-password
MM_AGENTS_OPENCLAW_TOKEN=your-openclaw-token  # or set an Anthropic key instead
```

See [Prerequisites](prerequisites.md) for details on obtaining each value.

## 4. Start the stack

=== "Option A — All at once"

    ```bash
    devenv up   # starts PostgreSQL + server + webapp together
    ```

=== "Option B — Three terminals"

    ```bash
    # Terminal A — PostgreSQL (Nix-managed, no Docker)
    mise run agents:db
    # Terminal A (after postgres is up) — Server
    mise run agents:server
    ```

=== "Terminal B — Webapp"

    ```bash
    mise run agents:webapp   # webpack dev server with hot reload
    ```

=== "Terminal C — Provision + Verify"

    ```bash
    # Wait for "Server is listening" in terminal A, then:
    mise run agents:provision  # provisions CEO + Sales agents (one-time per DB)
    mise run agents:status     # verify 4 agents are live
    ```

## 5. Run the demo

1. Open `http://localhost:8065` — log in as admin.
2. Navigate to `http://localhost:8065/agents` (or click **Agents** in the header).
3. In `#executive-general`, send:
   ```
   @ceo Please ask the Sales team to prepare a Q3 pipeline summary.
   ```
4. Watch the CEO tile go green → Sales Head activates → specialist tiles fire.
5. Click the Sales Head tile → **Delegation Tree** to see the full chain.

## Mise task reference

| Task | Description |
|------|-------------|
| `devenv up` | Start PostgreSQL + server + webapp (all via Nix) |
| `mise run agents:db` | Start PostgreSQL only (via devenv) |
| `mise run agents:server` | Run Mattermost server |
| `mise run agents:webapp` | Start webpack dev server |
| `mise run agents:provision` | Provision CEO + Sales demo workgroups |
| `mise run agents:status` | List all agent definitions (requires server running) |
| `mise run docs:serve` | Live-preview these docs at `http://localhost:8000` |
| `mise run docs:build` | Build static docs to `site/` |
