# Redux & State

## Reducers

Three new reducer keys were added to `state.entities.*`:

| Reducer key | State shape | Source file |
|-------------|-------------|-------------|
| `agentDefinitions` | `{ list: AgentDefinition[] }` | `reducers/entities/agent_definitions.ts` |
| `agentMetrics` | `{ list: AgentMetrics[] }` | `reducers/entities/agent_definitions.ts` |
| `agentMemory` | `{ byAgent: Record<string, AgentMemory[]> }` | `reducers/entities/agent_definitions.ts` |

Existing reducers extended:

| Reducer key | Extension |
|-------------|-----------|
| `agentTasks` | `{ byId: Record<string, AgentTask> }` — upserted by WS events |
| `workgroups` | `{ byId: Record<string, Workgroup> }` — fetched on workgroup selector load |

## Action creators

Defined in `mattermost-redux/src/actions/agents.ts`:

| Function | HTTP call | Dispatches |
|----------|-----------|-----------|
| `getAgentDefinitions()` | `GET /api/v4/agents/definitions` | `RECEIVED_AGENT_DEFINITIONS` |
| `getAgentMetrics()` | `GET /api/v4/agents/metrics` | `RECEIVED_AGENT_METRICS` |
| `getActiveAgentTasks(agentId)` | `GET /api/v4/agents/{id}/tasks/active` | `RECEIVED_ACTIVE_TASKS` |
| `patchAgentDefinition(id, patch)` | `PATCH /api/v4/agents/{id}` | `RECEIVED_AGENT_DEFINITION` |

## Action types

Defined in `mattermost-redux/src/action_types/agents.ts`:

```typescript
RECEIVED_AGENT_DEFINITIONS
RECEIVED_AGENT_DEFINITION
RECEIVED_AGENT_METRICS
RECEIVED_ACTIVE_TASKS
RECEIVED_AGENT_TASK_UPDATE
RECEIVED_AGENT_TOKEN_STREAM
RECEIVED_AGENT_DELEGATION
RECEIVED_AGENT_THINKING
RECEIVED_AGENT_MEMORY
```

## Selectors

Defined in `mattermost-redux/src/selectors/entities/agent_tasks.ts`:

| Selector | Returns |
|----------|---------|
| `getAllAgentTasks(state)` | All tasks indexed by ID |
| `getTasksByAgent(state, agentId)` | All tasks for one agent |
| `getActiveTaskForAgent(state, agentId)` | The current running/pending task |
| `getTaskEventsByTask(state, taskId)` | Ordered events for streaming |
| `getAgentDefinitionList(state)` | All agent definitions |
| `getAgentDefinitionById(state, id)` | One definition by ID |
| `getAgentMetricsById(state, agentId)` | Metrics object for one agent |

## Client4 methods

Added to `webapp/platform/client/src/client4.ts`:

| Method | HTTP |
|--------|------|
| `getAgentDefinitions()` | `GET /api/v4/agents/definitions` |
| `getActiveAgentTasks(agentId)` | `GET /api/v4/agents/{id}/tasks/active` |
| `getAgentMetrics()` | `GET /api/v4/agents/metrics` |
| `patchAgentDefinition(id, patch)` | `PATCH /api/v4/agents/{id}` |
| `getAgentTaskTree(taskId)` | `GET /api/v4/agents/tasks/{id}/tree` |
| `getAgentMemory(agentId)` | `GET /api/v4/agents/{id}/memory` |
| `submitAgentTask(req)` | `POST /api/v4/agents/tasks` |
