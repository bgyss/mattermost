# Component Reference

All components live in `webapp/channels/src/components/agent_command_center/`.

---

## `AgentCommandCenter` (`index.tsx`)

Route entry point. Responsibilities:

- Toggle `body.agent-command-center` on mount/unmount for full-screen layout.
- Own `selectedAgentId` and `filterWorkgroupId` state.
- Render `AgentStatusBar`, `AgentGrid`, and (conditionally) `AgentDetailPanel`.

**Props:** none (route component).

---

## `AgentStatusBar` (`agent_status_bar.tsx`)

Top ribbon showing system-wide metrics, workgroup filter, and "New Task" button.

- Polls `GET /api/v4/agents/metrics` every **30 seconds** via `getAgentMetrics()`.
- Derives four aggregate stats from all agent metrics.

| Metric | Calculation |
|--------|-------------|
| **Tasks Today** | Sum of `tasks_submitted` across all agents |
| **Tokens** | Sum of `tokens_total` (formatted as k/M) |
| **Active** | Count of agents with at least one un-completed task |
| **Failed** | Sum of `tasks_failed` |

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `selectedWorkgroupId` | `string` | Currently filtered workgroup (`''` = all) |
| `onWorkgroupChange` | `(id: string) => void` | Called when workgroup selector changes |
| `onNewTask` | `() => void` | Called when "+ New Task" button clicked |

---

## `AgentGrid` (`agent_grid.tsx`)

CSS Grid container for all agent tiles.

- Fetches `GET /api/v4/agents/definitions` on mount via `getAgentDefinitions()`.
- Filters by `filterWorkgroupId` when set.
- Column count: 1 agent → 1 col; 2–4 → 2 cols; 5+ → 3 cols.

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `filterWorkgroupId` | `string` | Only show agents in this workgroup |
| `selectedAgentId` | `string` | Highlighted tile ID |
| `onSelectAgent` | `(id: string) => void` | Called when a tile is clicked |

---

## `AgentTile` (`agent_tile.tsx`)

The core component — one per agent definition.

**Sections:**

| Section | Content |
|---------|---------|
| **Header** | Avatar (initials), display name, workgroup badge, status pill |
| **Body** | Live token stream (last 200 chars of `activeTask.output.text`) |
| **Tool badges** | Recent tool call names from task output metadata |
| **Footer** | Task count, tokens, avg latency from `AgentMetrics`; **Dispatch** button |
| **Dispatch form** | Inline textarea — submit with Cmd/Ctrl+Enter or button |

**Status pill states:**

| State | Color | Condition |
|-------|-------|-----------|
| Idle | Gray | No active task |
| Pending / Claimed | Amber | `status` is `pending`, `claimed`, or `awaiting_subtask` |
| Running | Green (pulsing dot) | `status === 'running'` |
| Complete | Blue | `status === 'complete'` |
| Failed | Red | `status === 'failed'` |

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `definition` | `AgentDefinition` | Agent definition object |
| `workgroupName` | `string?` | Display name of the owning workgroup |
| `selected` | `boolean` | Whether the detail panel is open |
| `onSelect` | `() => void` | Toggle detail panel |
| `onDispatch` | `(agentId, input) => void` | Submit a task |

---

## `AgentDetailPanel` (`agent_detail_panel.tsx`)

Right-side drawer opened by clicking a tile.

### Active Tasks tab

- Calls `getActiveAgentTasks(agentId)` on mount.
- Shows `status`, `input.text`, `tokens_used`, and `latency_ms` per task.

### History tab

- Reads completed/failed tasks from `state.entities.agentTasks.byId` filtered by `agent_id`.
- Most recent first.

### Delegation Tree tab

- Finds the root task for this agent and renders `<AgentDelegationTree rootTaskId={…}/>`.
- Renders the parent → child subtask DAG.

### Memory tab

- Calls `Client4.getAgentMemory(agentId)` directly (live fetch).
- Shows `key`, `value_text`, and `scope` for each entry.

### Settings tab

- Form for `display_name`, `model_id`, `max_concurrency`, `system_prompt`.
- Saves via `patchAgentDefinition(id, patch)` → `PATCH /api/v4/agents/{id}`.

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `definition` | `AgentDefinition` | Agent being inspected |
| `onClose` | `() => void` | Close the panel |
