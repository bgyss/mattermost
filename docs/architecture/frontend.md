# Frontend Architecture (React/TypeScript)

## Component tree

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

## New files added

### Components (`webapp/channels/src/components/`)

| File | Purpose |
|------|---------|
| `agent_command_center/index.tsx` | Route entry point, layout shell, state owner |
| `agent_command_center/agent_status_bar.tsx` | Top ribbon — metrics + workgroup selector |
| `agent_command_center/agent_grid.tsx` | CSS Grid container for tiles |
| `agent_command_center/agent_tile.tsx` | Per-agent tile with live token stream |
| `agent_command_center/agent_detail_panel.tsx` | Right drawer with 5 tabs |
| `agent_task_status/index.tsx` | Post badge (status + token count) |
| `agent_thinking_panel/index.tsx` | Live tool call viewer (right sidebar) |
| `agent_delegation_tree/index.tsx` | DAG visualization component |
| `agent_task_metrics/index.tsx` | Admin metrics page |

### Redux (`mattermost-redux/src/`)

| File | Purpose |
|------|---------|
| `reducers/entities/agent_definitions.ts` | `agentDefinitions`, `agentMetrics`, `agentMemory` reducers |
| `selectors/entities/agent_tasks.ts` | 7 selectors for tasks, definitions, metrics |
| `actions/agents.ts` | 4 action creators hitting agent REST endpoints |
| `action_types/agents.ts` | 9 action type string constants |

### Types & Client (`webapp/platform/`)

| File | Purpose |
|------|---------|
| `types/src/agent_tasks.ts` | `AgentTask`, `AgentDefinition`, `AgentMetrics`, `AgentMemory` |
| `types/src/workgroups.ts` | `Workgroup`, `WorkgroupTemplate` |
| `client/src/websocket_events.ts` | 6 new agent WebSocket event names |
| `client/src/client4.ts` | 7 new API methods (getAgentDefinitions, submitAgentTask, etc.) |

## Redux state shape

```typescript
state.entities.agentDefinitions = {
  list: AgentDefinition[]
}

state.entities.agentMetrics = {
  list: AgentMetrics[]
}

state.entities.agentMemory = {
  byAgent: Record<string, AgentMemory[]>
}

state.entities.agentTasks = {
  byId: Record<string, AgentTask>
}

state.entities.workgroups = {
  byId: Record<string, Workgroup>
}
```

## Selectors

| Selector | Returns |
|----------|---------|
| `getAllAgentTasks(state)` | All tasks indexed by ID |
| `getTasksByAgent(state, agentId)` | All tasks for one agent |
| `getActiveTaskForAgent(state, agentId)` | The current running/pending task |
| `getTaskEventsByTask(state, taskId)` | Ordered events for streaming |
| `getAgentDefinitionList(state)` | All agent definitions |
| `getAgentDefinitionById(state, id)` | One definition by ID |
| `getAgentMetricsById(state, agentId)` | Metrics object for one agent |

## WebSocket event handlers

Added to `webapp/channels/src/actions/websocket_actions.ts`:

| WS Event | Handler | Effect |
|----------|---------|--------|
| `agent_task_submitted` | `handleAgentTaskUpdate` | Upsert task in `byId` |
| `agent_task_complete` | `handleAgentTaskUpdate` | Update task status |
| `agent_task_failed` | `handleAgentTaskUpdate` | Update task status |
| `agent_token_stream` | `handleAgentTokenStream` | Append token to `task.output.text` |
| `agent_delegation` | `handleAgentDelegation` | Insert partial task for DAG rendering |
| `agent_thinking` | `handleAgentThinking` | Insert `AgentTaskEvent` for thinking panel |

## Modified files

| File | Change |
|------|--------|
| `channels/src/components/root/root.tsx` | Added `/agents` route |
| `channels/src/components/global_header/right_controls/right_controls.tsx` | Added "Agents" nav link |
| `channels/src/actions/websocket_actions.ts` | Wired 6 agent WS event handlers |
| `channels/src/packages/mattermost-redux/src/reducers/entities/index.ts` | Wired 3 new reducers |
| `channels/src/sass/base/_module.scss` | Imported `_agent_command_center.scss` |
