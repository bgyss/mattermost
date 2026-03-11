# Tutorial: Using the Agent Command Center

This tutorial walks you through the Agent Command Center from first launch to running real agent tasks, watching live token streams, and tuning agents through the settings panel.

**Prerequisites:** A running Mattermost server with at least one workgroup provisioned. See [AGENTS_DEMO.md](./AGENTS_DEMO.md) for workgroup setup.

---

## Part 1 — Opening the command center

### Option A — URL

Go directly to `http://localhost:8065/agents`.

### Option B — Global header button

1. Log in to Mattermost.
2. Look in the top-right header bar for the **Agents** button.
3. Click it.

The team sidebar disappears and the command center fills the screen. You'll see:

```
┌──────────────────────────────────────────────────────────────────────┐
│  Tasks Today: 0   Tokens: 0   Active: 0   Failed: 0  [All Wkgrps ▾] │  ← Status bar
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │  CEO                       Executive ●  Idle                  │  │  ← Agent tile
│  │  No active task                                               │  │
│  │  Tasks: 0   Tokens: 0             [Dispatch]                  │  │
│  └────────────────────────────────────────────────────────────────┘  │
│  ... more tiles ...                                                   │
└──────────────────────────────────────────────────────────────────────┘
```

If the grid is empty, your server has no agent definitions yet — go to Part 0 below.

---

## Part 0 — Provisioning agents (if starting fresh)

If you have no agents, provision the demo setup first:

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

Refresh `/agents` — you should now see tiles for the CEO, Sales Head, and specialist agents.

---

## Part 2 — Reading the status bar

The top ribbon aggregates stats across every agent:

| Metric | What it means |
|--------|--------------|
| **Tasks Today** | Total `tasks_submitted` across all agents |
| **Tokens** | Sum of all `tokens_total` (formatted as k/M for large numbers) |
| **Active** | Number of agents with at least one submitted, un-completed task |
| **Failed** | Sum of `tasks_failed` — investigate these first |

The status bar refreshes every **30 seconds** automatically.

### Filtering by workgroup

Use the **workgroup selector** (right side of the ribbon) to narrow the grid to a single department. Select "All Workgroups" to see everything.

---

## Part 3 — Reading an agent tile

Each tile is divided into four sections:

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

**Status pill color guide:**

- ⚫ **Gray / Idle** — no active task, agent is waiting
- 🟡 **Amber / Pending** — task is queued, not yet claimed
- 🟢 **Green (pulsing) / Running** — agent is actively processing
- 🔵 **Blue / Complete** — last task finished successfully
- 🔴 **Red / Failed** — last task errored

The body text is the **live output stream** — it shows the last 200 characters of whatever the agent is writing, updating in real time via WebSocket.

---

## Part 4 — Dispatching a task from a tile

1. Click **Dispatch** in the tile footer. An inline text area slides open.
2. Type your instruction, e.g. `Identify the top 3 prospects in the fintech vertical`.
3. Press **Cmd+Enter** (Mac) or **Ctrl+Enter** (Windows/Linux) to submit, or click **Send**.

Watch the tile body immediately start filling with the agent's output. The status pill changes to "Running" (green, pulsing).

> **Tip:** The dispatch form sends `POST /api/v4/agents/tasks` with the agent ID you chose. If you want to specify a channel for the response post, use the REST API directly (see [AGENTS_DEMO.md](./AGENTS_DEMO.md)).

---

## Part 5 — Opening the detail panel

**Single-click** any tile to open its detail panel on the right side. The tile gets a blue border highlight and the panel slides in.

Click the tile again (or the **×** button in the panel header) to close it.

### Active Tasks tab

Shows all tasks currently running or queued for this agent. Each row shows:
- Task status badge
- Input text (truncated)
- Token count and latency

If you dispatched a task in Part 4, you should see it here with a "Running" badge.

### History tab

All completed and failed tasks for this agent, most recent first. Good for auditing what the agent has done.

### Delegation Tree tab

If the agent has delegated subtasks to other agents, you'll see the parent → child chain here, rendered as an indented tree. This is powered by the `root_task_id` and `parent_task_id` fields on each `AgentTask`.

Example tree for a CEO task that delegated to Sales Head:

```
CEO task (complete)
  └── Sales Head task (complete)
        ├── Prospecting Specialist task (complete)
        └── Proposal Writer task (running)
```

### Memory tab

Shows all entries in the agent's memory store — key/value pairs tagged with a scope (`global`, `task`, or `user`). This is the persistent context that carries between tasks.

Useful for understanding why an agent behaves a certain way — if it has remembered something incorrect, you can see it here (and clear it via `DELETE /api/v4/agents/{id}/memory`).

### Settings tab

A live-edit form for the agent's core configuration:

| Field | Effect |
|-------|--------|
| **Display Name** | Changes how the agent identifies itself in the tile header |
| **Model ID** | Switches the underlying LLM (e.g. `claude-opus-4-6` → `claude-haiku-4-5`) |
| **Max Concurrency** | How many tasks this agent handles simultaneously |
| **System Prompt** | The instruction that shapes the agent's behavior and persona |

Click **Save Changes** — the change takes effect on the next task (running tasks continue with the old prompt).

---

## Part 6 — Watching a live delegation

This scenario requires the demo setup from [AGENTS_DEMO.md](./AGENTS_DEMO.md) with at least two workgroups.

1. In the command center, dispatch to the CEO tile:
   ```
   Ask the Sales team for a current pipeline summary.
   ```
2. Watch the CEO tile: status → "Running", token stream shows reasoning.
3. After a few seconds, the Sales Head tile also goes "Running".
4. Click the Sales Head tile → open the detail panel → **Delegation Tree** tab.
   You should see the CEO's root task with the Sales Head's subtask nested under it.
5. Specialist tiles may also activate if Sales Head further delegates.
6. When all tasks complete, all tiles return to "Idle" (or "Complete") and the metrics bar updates.

---

## Part 7 — Filtering and focus mode

When you have many agents (5+), the grid switches to 3 columns automatically. To focus on one department:

1. Use the **workgroup selector** in the status bar.
2. Pick e.g. "Sales" → only the Sales Head and specialists are shown.
3. The status bar metrics still reflect the full system, not just the filtered view.

---

## Part 8 — Admin tasks via the command center

### Checking system health

Look at the status bar every morning:
- **Failed > 0** — open each failed agent's detail panel → History tab, check the error in the task row.
- **Active doesn't decrease** — tasks may be stuck. Check `GET /api/v4/agents/{id}/tasks/active` for hung tasks.

### Swapping a model for one agent

1. Click a tile → Settings tab.
2. Change **Model ID** to a cheaper/faster model (e.g. `claude-haiku-4-5-20251001`).
3. Save.

### Updating a system prompt

1. Click the tile → Settings tab.
2. Edit the **System Prompt** text area.
3. Save.

Changes take effect immediately on the next submitted task.

### Inspecting agent memory

1. Click the tile → Memory tab.
2. Review key/value entries.
3. If you see stale or incorrect data, clear it:
   ```bash
   curl -X DELETE http://localhost:8065/api/v4/agents/{AGENT_ID}/memory \
     -H "Authorization: Bearer $TOKEN"
   ```

---

## Keyboard reference

| Key | Action |
|-----|--------|
| Enter (on tile) | Open detail panel |
| Esc (in dispatch form) | Close dispatch form |
| Cmd/Ctrl + Enter (in dispatch form) | Submit task |
| Click tile again | Close detail panel |
| Click × button | Close detail panel |

---

## Troubleshooting

### The grid is empty

- Confirm at least one workgroup is provisioned: `GET /api/v4/agents/definitions` should return a non-empty array.
- Check browser console for network errors on the `/definitions` fetch.

### Status pill is always "Idle" even during active tasks

- WebSocket may not be connected. Check the browser console for WebSocket errors.
- The tile only shows live data if the task's agent ID matches the definition. Confirm the task was dispatched to the correct agent.

### Token stream isn't updating

- The `agent_token_stream` WebSocket event requires the server's `AgentRuntimeService` to be running. Check server logs for `"agent runtime started"`.
- Confirm `MM_AGENTS_OPENCLAW_TOKEN` (or the relevant provider key) is set.

### Detail panel Settings tab doesn't save

- Check the browser network tab — the `PATCH /api/v4/agents/{id}` call should return `200`.
- The current user must have appropriate permissions (currently any session is accepted; add role checks as needed).

### Navigating away doesn't restore the sidebar

- This is a lifecycle bug. Force a page reload or navigate to a team URL like `/team-name/channels/town-square`.
- The `useEffect` cleanup in `AgentCommandCenter/index.tsx` removes the `agent-command-center` body class on unmount, but React's unmount may not fire if navigation is aborted. This is a known edge case.

---

## What's next

| Goal | How |
|------|-----|
| Add a new department | `POST /api/v4/workgroups` with a template ID, then refresh the command center |
| Connect to Claude directly | Set `MM_AGENTS_ANTHROPIC_API_KEY` and use `llm_service_id: "anthropic"` |
| Add agent E2E tests | See [e2e-tests/playwright/CLAUDE.OPTIONAL.md](./e2e-tests/playwright/CLAUDE.OPTIONAL.md) for Playwright patterns |
| Build a plugin that extends a tile | Use Mattermost's `moduleRegistry` and Module Federation to inject into the tile grid |
| Export task history | Query `GET /api/v4/agents/tasks/{id}/events` and pipe to a data pipeline |
