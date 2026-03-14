# Cursor Cloud Environment

This page covers the dual-repo enterprise setup used in Cursor Cloud agents.

## Repository layout

| Repository | Location | Purpose |
|------------|----------|---------|
| `mattermost/mattermost` | `/workspace` | Primary monorepo (Go server + React webapp) |
| `mattermost/enterprise` | `$HOME/enterprise` | Private enterprise code (Go, linked via `go.work`) |
| `mattermost/mattermost-plugin-agents` | `$HOME/mattermost-plugin-agents` | AI plugin for validation/testing |

PostgreSQL 14 is the only required external dependency, run via Docker Compose.

## Starting services

1. **Start Docker daemon:**
   ```bash
   sudo dockerd &>/tmp/dockerd.log &
   # Wait a few seconds, then verify:
   docker info
   ```

2. **Start server + webapp:**
   ```bash
   cd /workspace/server && \
     MM_LICENSE="$TEST_LICENSE" \
     MM_PLUGINSETTINGS_ENABLEUPLOADS=true \
     MM_PLUGINSETTINGS_ENABLE=true \
     MM_SERVICESETTINGS_SITEURL=http://localhost:8065 \
     make BUILD_ENTERPRISE_DIR="$HOME/enterprise" run
   ```

3. **Restart server after code changes:**
   ```bash
   cd /workspace/server && \
     MM_LICENSE="$TEST_LICENSE" \
     make BUILD_ENTERPRISE_DIR="$HOME/enterprise" restart-server
   ```

!!! warning "Always pass BUILD_ENTERPRISE_DIR"
    Every `make` command (`run`, `restart-server`, `check-style`, `test-server`) **must** include `BUILD_ENTERPRISE_DIR="$HOME/enterprise"`. Without it the build silently falls back to team edition.

## Enterprise verification

The server logs `"Enterprise Build", enterprise_build: true` when enterprise code is loaded. If you see "TEAM EDITION" in the webapp, it means the license isn't loaded — pass `MM_LICENSE="$TEST_LICENSE"`.

Check via API:
```bash
curl -s http://localhost:8065/api/v4/config/client?format=old | jq '.BuildEnterpriseReady'
# Should return "true"
```

## Agents plugin

```bash
cd $HOME/mattermost-plugin-agents
MM_SERVICESETTINGS_SITEURL=http://localhost:8065 make deploy
```

Configure via the Mattermost config API. The `config` field under `mattermost-ai` must be a JSON **object** (not a string):

```python
import json, os

config = {
    "PluginSettings": {
        "Plugins": {
            "mattermost-ai": {
                "config": {   # MUST be an object, NOT json.dumps(...)
                    "services": [{
                        "id":                    "anthropic-svc-001",
                        "name":                  "Anthropic Claude",
                        "type":                  "anthropic",
                        "apiKey":                os.environ["ANTHROPIC_API_KEY"],
                        "defaultModel":          "claude-sonnet-4-6",
                        "tokenLimit":            200000,
                        "outputTokenLimit":      16000,
                        "streamingTimeoutSeconds": 300
                    }],
                    "bots": [{
                        "id":                "claude-bot-001",
                        "name":              "claude",
                        "displayName":       "Claude Assistant",
                        "serviceID":         "anthropic-svc-001",
                        "customInstructions": "You are a helpful AI assistant.",
                        "enableVision":      True,
                        "disableTools":      False,
                        "channelAccessLevel": 0,
                        "userAccessLevel":   0,
                        "reasoningEnabled":  True,
                        "thinkingBudget":    1024
                    }],
                    "defaultBotName": "claude"
                }
            }
        }
    }
}
# Write to temp file, then PATCH:
# curl -X PUT http://localhost:8065/api/v4/config/patch \
#   -H "Authorization: Bearer $TOKEN" -d @config.json
```

!!! danger "Never log the API key"
    The `apiKey` field contains a secret. Never print or log it.

## Supported service types

`openai`, `openaicompatible`, `azure`, `anthropic`, `asage`, `cohere`, `bedrock`, `mistral`

## Lint, test, build

All commands from `/workspace/server/`, always include `BUILD_ENTERPRISE_DIR`:

| Task | Command |
|------|---------|
| Run | `make BUILD_ENTERPRISE_DIR="$HOME/enterprise" run` |
| Restart | `make BUILD_ENTERPRISE_DIR="$HOME/enterprise" restart-server` |
| Lint | `make BUILD_ENTERPRISE_DIR="$HOME/enterprise" check-style` |
| Tests | `make BUILD_ENTERPRISE_DIR="$HOME/enterprise" test-server` |
| Quick tests | `go test ./public/model/...` |

**Webapp** (from `/workspace/webapp/`):

```bash
npm run check        # lint
npm run test         # Jest
npm run check-types  # TypeScript
npm run build        # production build
```

## Key gotchas

- **"TEAM EDITION"** means no license (not no enterprise code). Fix: pass `MM_LICENSE="$TEST_LICENSE"`.
- Server auto-generates `config/config.json` pointing to `postgres://mmuser:mostest@localhost/mattermost_test` (matches Docker Compose defaults).
- The first user created via `/api/v4/users` gets `system_admin` automatically.
- SMTP errors and plugin directory warnings on startup are expected in dev — non-blocking.
- The enterprise repo must be on a compatible branch with the main repo.
- **Preferred VCS is `jj`.** Use `jj git fetch` / `jj git push` for remote sync. See `CLAUDE.md` for the full command reference.

## Browser automation

`agent-browser` (Vercel) is installed globally in the Cursor Cloud environment. It provides CLI browser automation: navigation, clicking, typing, screenshots, accessibility snapshots, and visual diffs.

```bash
agent-browser <command>
```

See the agent-browser skill for the full command reference.
