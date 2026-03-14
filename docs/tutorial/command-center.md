# Tutorial: Agent Command Center

A guided walkthrough from first launch to live agent delegation.

**Prerequisites:** Server running with at least one workgroup provisioned. See [Local Demo Setup](../demo/local-setup.md).

---

## Part 0 — Provision agents (if starting fresh)

If you have no agents, run:

```bash
mise run agents:provision
```

Or via the API manually:

```bash
TOKEN="your-admin-token"
TEAM_ID="your-team-id"
ADMIN_USER_ID="your-user-id"

curl -s -X POST http://localhost:8065/api/v4/agents/demo-setup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"team_id\": \"$TEAM_ID\",
    \"observer_user_ids\": [\"$ADMIN_USER_ID\"],
    \"llm_service_id\": \"openclaw\"
  }" | jq .
```

Refresh `/agents` — you should see tiles for CEO, Sales Head, and specialist agents.

---

## Part 1 — Opening the command center

=== "Via URL"

    Navigate to `http://localhost:8065/agents`.

=== "Via header button"

    1. Log in to Mattermost.
    2. Click the **Agents** button in the top-right header bar.

The sidebars disappear and the command center fills the screen.

---

## Part 2 — Reading the status bar

The top ribbon aggregates stats across every agent:

| Metric | What it means |
|--------|--------------|
| **Tasks Today** | Total `tasks_submitted` across all agents |
| **Tokens** | Sum of all `tokens_total` |
| **Active** | Agents with at least one un-completed task |
| **Failed** | Sum of `tasks_failed` — investigate these first |

The status bar refreshes every **30 seconds**.

Use the **workgroup selector** on the right to filter the grid to a single department.

---

## Part 3 — Reading a tile

```
┌─────────────────────────────────────────────────────┐
│ [SH]  Sales Head   [Sales] ●●● Running              │  ← Header
├─────────────────────────────────────────────────────┤
│ …identifying top 5 enterprise prospects from        │  ← Live token stream
│ the Salesforce data. Running search_posts tool now  │
├─────────────────────────────────────────────────────┤
│ [search_posts] [create_post]                        │  ← Tool badges
├─────────────────────────────────────────────────────┤
│ Tasks: 12   Tokens: 48,210   Avg Lat: 2.1s [Dispatch]│  ← Footer
└─────────────────────────────────────────────────────┘
```

**Status pill guide:**

| Color | State | Condition |
|-------|-------|-----------|
| ⚫ Gray | Idle | No active task |
| 🟡 Amber | Pending | Task queued, not yet claimed |
| 🟢 Green (pulsing) | Running | Agent actively processing |
| 🔵 Blue | Complete | Last task finished successfully |
| 🔴 Red | Failed | Last task errored |

The body text is the **live output stream** — last 200 characters, updated per WebSocket token event.

---

## Part 4 — Dispatching a task

1. Click **Dispatch** in a tile footer — an inline textarea slides open.
2. Type your instruction.
3. Press **Cmd+Enter** (Mac) or **Ctrl+Enter** (Windows/Linux), or click **Send**.

Watch the tile body fill with the agent's output. Status pill → Running (green, pulsing).

!!! tip
    The dispatch form calls `POST /api/v4/agents/tasks`. For advanced options (channel ID, priority), use the REST API directly.

---

## Part 5 — Opening the detail panel

**Single-click** any tile to open its detail panel on the right side.

### Active Tasks tab

Shows all tasks currently running or queued. Each row: status badge, input text, token count, latency.

### History tab

All completed and failed tasks, most recent first. Good for auditing.

### Delegation Tree tab

If the agent has delegated subtasks, you'll see the parent → child chain:

```
CEO task (complete)
  └── Sales Head task (complete)
        ├── Prospecting Specialist task (complete)
        └── Proposal Writer task (running)
```

Powered by `root_task_id` and `parent_task_id` fields on each `AgentTask`.

### Memory tab

All entries in the agent's memory store — key/value pairs with scope (`global`, `task`, `user`). This is the persistent context between tasks.

To clear stale memory:

```bash
curl -X DELETE http://localhost:8065/api/v4/agents/{AGENT_ID}/memory \
  -H "Authorization: Bearer $TOKEN"
```

### Settings tab

| Field | Effect |
|-------|--------|
| **Display Name** | Changes agent identity in tile header |
| **Model ID** | Switches the underlying LLM (e.g. `claude-haiku-4-5-20251001`) |
| **Max Concurrency** | Simultaneous tasks this agent handles |
| **System Prompt** | Instruction shaping the agent's behaviour |

Click **Save Changes** — takes effect on the next submitted task.

---

## Part 6 — Watching live delegation

1. Dispatch to the CEO tile:
   ```
   Ask the Sales team for a current pipeline summary.
   ```
2. CEO tile: status → Running, token stream shows reasoning.
3. After a few seconds, Sales Head tile also goes Running.
4. Click the Sales Head tile → **Delegation Tree** tab.
   You should see the CEO's root task with the Sales Head's subtask nested under it.
5. Specialist tiles activate if Sales Head further delegates.
6. All tiles return to Idle/Complete when done; metrics bar updates.

---

## Part 7 — Filtering and focus mode

When you have 5+ agents, the grid switches to 3 columns automatically. To focus on one department:

1. Use the **workgroup selector** in the status bar.
2. Pick e.g. "Sales" → only Sales Head and specialists are shown.
3. The metrics bar still reflects the full system.

---

## Part 8 — Admin tasks

### Swapping a model

1. Click tile → Settings tab.
2. Change **Model ID** (e.g. `claude-haiku-4-5-20251001` for lower cost).
3. Save.

### Checking system health

- **Failed > 0** → open failed agent's detail panel → History tab → check error.
- **Active doesn't decrease** → tasks may be hung → check `GET /api/v4/agents/{id}/tasks/active`.

---

## Keyboard reference

| Key | Action |
|-----|--------|
| `Esc` (in dispatch form) | Close dispatch form |
| `Cmd/Ctrl + Enter` (in dispatch form) | Submit task |
| Click tile again | Close detail panel |
| Click × button | Close detail panel |

---

## Troubleshooting

**Empty grid** — confirm `GET /api/v4/agents/definitions` returns non-empty; check browser console.

**Status pill always Idle** — check browser console for WebSocket errors; confirm `agent_task_submitted` fires in the WS inspector.

**Token stream not updating** — confirm `MM_AGENTS_OPENCLAW_TOKEN` is set and the gateway is running; look for `"agent runtime started"` in server logs.

**Settings tab doesn't save** — check network tab for `PATCH /api/v4/agents/{id}` returning 200.

**Sidebars not restored after navigation** — force page reload or navigate to a channel URL like `/team-name/channels/town-square`.

---

## What's next

| Goal | How |
|------|-----|
| Add a new department | `POST /api/v4/workgroups` with a template ID |
| Connect to Claude directly | Set `MM_AGENTS_ANTHROPIC_API_KEY` + `llm_service_id: "anthropic"` |
| Export task history | `GET /api/v4/agents/tasks/{id}/events` piped to a data pipeline |
| Add a custom tile tab | See [Extending the Command Center](../command-center/extending.md) |
| Write E2E tests | See `e2e-tests/playwright/CLAUDE.OPTIONAL.md` |
