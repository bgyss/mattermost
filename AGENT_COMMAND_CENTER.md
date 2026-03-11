# Agent Command Center

The Agent Command Center is a full-screen, real-time control panel for the Mattermost multi-agent orchestration platform. It surfaces every agent definition in a resizable tile grid, streams live token output and tool calls per tile, and provides tabbed deep-dive panels for tasks, delegation trees, memory, and settings — all without leaving the browser.

---

## Navigation

| Method | How |
|--------|-----|
| URL | `http://localhost:8065/agents` |
| Global header | Click the **Agents** button in the top-right navigation bar |

When the page mounts, the team sidebar, left sidebar, and right sidebar are hidden and the command center fills the entire viewport. The normal channel view is restored when you navigate away.

---

## Architecture overview

```
/agents route (root.tsx)
    └── AgentCommandCenter (index.tsx)           ← body class toggle, layout shell
          ├── AgentStatusBar                     ← top ribbon: metrics + workgroup filter
          └── acc-main (flex row)
                ├── AgentGrid                   ← CSS Grid of AgentTile components
                │     └── AgentTile × N         ← per-agent tile
                └── AgentDetailPanel (optional) ← right drawer, opens on tile click
                      ├── Active Tasks tab
                      ├── History tab
                      ├── Delegation Tree tab
                      ├── Memory tab
                      └── Settings tab
```

### Full-screen layout

The `AgentCommandCenter` component toggles `body.agent-command-center` on mount and removes it on unmount. The corresponding SCSS (in `sass/base/_agent_command_center.scss`) collapses the standard channel layout:

```scss
body.agent-command-center {
    #TeamSidebar, .sidebar--left, .sidebar--right { display: none !important; }
    .agent-command-center-root { flex: 1; display: flex; flex-direction: column; }
}
```

---

## Component reference

### `AgentCommandCenter` (`index.tsx`)

The route entry point. Responsibilities:

- Mount/unmount the `agent-command-center` body class for full-screen layout.
- Own `selectedAgentId` and `filterWorkgroupId` state.
- Render `AgentStatusBar`, `AgentGrid`, and conditionally `AgentDetailPanel`.

**Props:** none (route component).

---

### `AgentStatusBar` (`agent_status_bar.tsx`)

Top ribbon showing system-wide metrics, a workgroup filter, and a "New Task" button.

**Features:**
- Polls `GET /api/v4/agents/metrics` every **30 seconds** via Redux action `getAgentMetrics()`.
- Derives four aggregate stats from all agent metrics: Tasks Today, Tokens, Active agents, Failed tasks.
- Workgroup selector populated from `state.entities.workgroups.byId`.

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `selectedWorkgroupId` | `string` | Currently filtered workgroup (`''` = all) |
| `onWorkgroupChange` | `(id: string) => void` | Called when workgroup selector changes |
| `onNewTask` | `() => void` | Called when "New Task" button is clicked |

---

### `AgentGrid` (`agent_grid.tsx`)

CSS Grid container for all agent tiles.

**Features:**
- Fetches `GET /api/v4/agents/definitions` on mount via `getAgentDefinitions()` action.
- Filters by `filterWorkgroupId` when set.
- Column count is automatic based on agent count: 1 agent → 1 col, 2–4 → 2 cols, 5+ → 3 cols.
- Delegates dispatch calls to `Client4.submitAgentTask()`.

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `filterWorkgroupId` | `string` | Only show agents in this workgroup |
| `selectedAgentId` | `string` | Highlighted tile ID |
| `onSelectAgent` | `(id: string) => void` | Called when a tile is clicked |

---

### `AgentTile` (`agent_tile.tsx`)

The core component — one tile per agent definition.

**Sections:**

| Section | Content |
|---------|---------|
| **Header** | Avatar (initials), display name, workgroup badge, status pill |
| **Body** | Live token stream (last 200 chars of `activeTask.output.text`) |
| **Tool badges** | Recent tool call names from task output metadata |
| **Footer** | Task count, total tokens, avg latency from `AgentMetrics`; **Dispatch** button |
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
| `selected` | `boolean` | Whether the detail panel is open for this tile |
| `onSelect` | `() => void` | Toggle detail panel |
| `onDispatch` | `(agentId, input) => void` | Submit a task |

---

### `AgentDetailPanel` (`agent_detail_panel.tsx`)

Right-side drawer that opens when a tile is selected.

**Tabs:**

#### Active Tasks
- Dispatches `getActiveAgentTasks(agentId)` on mount.
- Shows `status`, `input.text`, `tokens_used`, and `latency_ms` for each running/pending task.

#### History
- Reads completed/failed tasks from `state.entities.agentTasks.byId` filtered by `agent_id`.
- Most recent first.

#### Delegation Tree
- Finds the root task for this agent and renders `<AgentDelegationTree rootTaskId={…}/>`.
- Renders the DAG of parent → child subtask delegation.

#### Memory
- Calls `Client4.getAgentMemory(agentId)` directly (live fetch, not cached in Redux).
- Shows `key`, `value_text`, and `scope` for each memory entry.

#### Settings
- Form for `display_name`, `model_id`, `max_concurrency`, and `system_prompt`.
- Saves via `patchAgentDefinition(id, patch)` Redux action → `PATCH /api/v4/agents/{id}`.

**Props:**

| Prop | Type | Description |
|------|------|-------------|
| `definition` | `AgentDefinition` | Agent being inspected |
| `onClose` | `() => void` | Close the panel |

---

## Redux state

### New reducers (added in this phase)

| Reducer key | State shape | Source action types |
|-------------|-------------|---------------------|
| `entities.agentDefinitions` | `{ list: AgentDefinition[] }` | `RECEIVED_AGENT_DEFINITIONS`, `RECEIVED_AGENT_DEFINITION` |
| `entities.agentMetrics` | `{ list: AgentMetrics[] }` | `RECEIVED_AGENT_METRICS` |
| `entities.agentMemory` | `{ byAgent: Record<string, AgentMemory[]> }` | `RECEIVED_AGENT_MEMORY` |

### New action creators (`mattermost-redux/actions/agents`)

| Function | HTTP call | Dispatches |
|----------|-----------|-----------|
| `getAgentDefinitions()` | `GET /api/v4/agents/definitions` | `RECEIVED_AGENT_DEFINITIONS` |
| `getAgentMetrics()` | `GET /api/v4/agents/metrics` | `RECEIVED_AGENT_METRICS` |
| `getActiveAgentTasks(agentId)` | `GET /api/v4/agents/{id}/tasks/active` | `RECEIVED_ACTIVE_TASKS` |
| `patchAgentDefinition(id, patch)` | `PATCH /api/v4/agents/{id}` | `RECEIVED_AGENT_DEFINITION` |

### New selectors (`mattermost-redux/selectors/entities/agent_tasks`)

| Selector | Returns |
|----------|---------|
| `getAllAgentTasks(state)` | All tasks indexed by ID |
| `getTasksByAgent(state, agentId)` | All tasks for one agent |
| `getActiveTaskForAgent(state, agentId)` | The current running/pending task |
| `getTaskEventsByTask(state, taskId)` | Ordered events for streaming |
| `getAgentDefinitionList(state)` | All agent definitions |
| `getAgentDefinitionById(state, id)` | One definition by ID |
| `getAgentMetricsById(state, agentId)` | Metrics object for one agent |

---

## WebSocket event handling

The command center reacts to live events dispatched by the server:

| WS Event | Handler | Effect |
|----------|---------|--------|
| `agent_task_submitted` | `handleAgentTaskUpdate` | Upsert task in `byId` |
| `agent_task_complete` | `handleAgentTaskUpdate` | Update task status |
| `agent_task_failed` | `handleAgentTaskUpdate` | Update task status |
| `agent_token_stream` | `handleAgentTokenStream` | Append token to `task.output.text` |
| `agent_delegation` | `handleAgentDelegation` | Insert partial task into `byId` for DAG rendering |
| `agent_thinking` | `handleAgentThinking` | Insert `AgentTaskEvent` for thinking panel |

The token stream is the mechanism by which the tile body updates in real time — each `agent_token_stream` event appends one token chunk to the task's output text in Redux state.

---

## REST API additions

### `GET /api/v4/agents/definitions`

Lists all non-deleted `AgentDefinition` records across all workgroups. Ordered by `create_at` ascending.

**Auth:** Session required.

**Response:** `AgentDefinition[]`

```json
[
  {
    "id": "def1abc...",
    "workgroup_id": "wg1...",
    "role": "head",
    "bot_user_id": "bot1...",
    "display_name": "Sales Head",
    "system_prompt": "You are the head of the Sales department...",
    "llm_service_id": "openclaw",
    "model_id": "claude-sonnet-4-6",
    "tools": ["search_posts", "create_post", "delegate_task"],
    "capabilities": ["sales", "account_management"],
    "max_concurrency": 5,
    "pool_size": 3,
    "create_at": 1741392000000,
    "update_at": 1741392000000
  }
]
```

---

## File index

### New files

| File | Purpose |
|------|---------|
| `webapp/channels/src/components/agent_command_center/index.tsx` | Route entry point, layout shell |
| `webapp/channels/src/components/agent_command_center/agent_status_bar.tsx` | Top metrics ribbon |
| `webapp/channels/src/components/agent_command_center/agent_grid.tsx` | Grid of tiles |
| `webapp/channels/src/components/agent_command_center/agent_tile.tsx` | Per-agent tile |
| `webapp/channels/src/components/agent_command_center/agent_detail_panel.tsx` | Right drawer |
| `webapp/channels/src/sass/base/_agent_command_center.scss` | All command center CSS |
| `webapp/channels/src/packages/mattermost-redux/src/reducers/entities/agent_definitions.ts` | agentDefinitions, agentMetrics, agentMemory reducers |
| `webapp/channels/src/packages/mattermost-redux/src/selectors/entities/agent_tasks.ts` | Selectors for tasks, definitions, metrics, memory |

### Modified files

| File | Change |
|------|--------|
| `webapp/channels/src/components/root/root.tsx` | Added `/agents` route |
| `webapp/channels/src/components/global_header/right_controls/right_controls.tsx` | Added "Agents" nav link |
| `webapp/channels/src/actions/websocket_actions.ts` | Added `handleAgentThinking`, wired `AgentThinking` WS event |
| `webapp/channels/src/packages/mattermost-redux/src/actions/agents.ts` | Added 4 new action creators |
| `webapp/channels/src/packages/mattermost-redux/src/action_types/agents.ts` | Added 9 new action type keys |
| `webapp/channels/src/packages/mattermost-redux/src/reducers/entities/index.ts` | Wired 3 new reducers |
| `webapp/channels/src/sass/base/_module.scss` | Imported new SCSS |
| `webapp/platform/client/src/client4.ts` | Added 7 new API client methods |
| `server/channels/store/store.go` | Added `ListAllAgentDefinitions()` to `AgentStore` interface |
| `server/channels/store/sqlstore/agent_store.go` | SQL implementation of `ListAllAgentDefinitions()` |
| `server/channels/store/storetest/mocks/AgentStore.go` | Mock for `ListAllAgentDefinitions()` |
| `server/channels/app/workgroups.go` | App layer `ListAllAgentDefinitions()` |
| `server/channels/api4/agents.go` | New `GET /api/v4/agents/definitions` route + handler |

---

## CSS theming

All command center styles use CSS custom properties from the Mattermost theme system. To override colors for the command center only, target the `.agent-command-center-root` scope:

```scss
.agent-command-center-root {
    --acc-tile-bg: var(--center-channel-bg);
    --acc-running-color: #3fc380;
    --acc-failed-color: var(--error-text);
}
```

Key CSS classes:

| Class | Element |
|-------|---------|
| `.acc-status-bar` | Top ribbon |
| `.acc-grid` | Tile grid container |
| `.acc-tile` | Individual agent tile |
| `.acc-tile__status-pill--{state}` | Status pill variant |
| `.acc-detail-panel` | Right drawer |
| `.acc-detail-panel__tab` | Tab button |
| `.acc-settings-form` | Settings form in detail panel |
| `.acc-dispatch-form` | Inline task dispatch form |

---

## Extending the command center

### Adding a new tab to the detail panel

1. Add a tab label to the `Tab` type in `agent_detail_panel.tsx`.
2. Add a tab button in the tabs `map`.
3. Add a tab content component.

```tsx
// 1. extend the union
type Tab = 'active' | 'history' | 'tree' | 'memory' | 'settings' | 'analytics';

// 2. add the button (in the tabs map)
t === 'analytics' ? 'Analytics' : ...

// 3. add the content
{tab === 'analytics' && <AnalyticsTab agentId={definition.id}/>}
```

### Adding a new tile footer stat

Tile footer stats come from `AgentMetrics`. To surface a custom stat, add a field to the `AgentMetrics` type in `server/public/model/agent_task.go`, update the observability counters in `agentruntime/observability.go`, and add a `<FooterStat/>` call in `agent_tile.tsx`.

### Plugging in a custom dispatch modal

The "Agents" status bar has a `onNewTask` prop that fires when the "+ New Task" button is clicked. Pass a handler that opens any modal registered with Mattermost's `ModalController`:

```tsx
// in a parent component that has access to dispatch:
onNewTask={() => dispatch(openModal({
    modalId: ModalIdentifiers.MY_DISPATCH_MODAL,
    dialogType: MyDispatchModal,
}))}
```

---

## Verification checklist

- [ ] Navigate to `/agents` → full-screen view loads, sidebars hidden
- [ ] With agent definitions in DB → tiles appear with correct names and workgroup badges
- [ ] Submit a task → tile transitions from "idle" to "running", token stream appears in tile body
- [ ] Click a tile → detail panel opens, Active Tasks tab shows the running task
- [ ] Task completes → status pill turns green, token count + latency shown in footer
- [ ] Delegation Tree tab → shows parent→child chain when delegation occurred
- [ ] Memory tab → shows agent memory keys
- [ ] Settings tab → edit system prompt → PATCH saves → value persists on reload
- [ ] Workgroup selector → filter to one workgroup → only those agents shown
- [ ] "Agents" link in global header navigates to `/agents`
- [ ] Navigating away → body class removed, normal channel view restored
