# Backend Architecture (Go)

## AgentRuntimeService

**Location:** `server/platform/services/agentruntime/`

The runtime is started as part of `app.Server` and wired in via `AgentAppIface` (defined in `executor.go`) to avoid circular imports.

### Files

| File | Purpose |
|------|---------|
| `service.go` | Service struct, `Start()` / `Stop()`, `SubmitTask()` entrypoint |
| `executor.go` | Core agentic loop — context → LLM → stream → tool calls → repeat |
| `dispatcher.go` | In-memory task queue + per-agent semaphores + 10-second DB recovery poller |
| `tool_registry.go` | `AgentTool` interface + parallel tool execution |
| `mmtools.go` | 5 built-in Mattermost tools: `search_posts`, `get_channel`, `create_post`, `get_user`, `list_channels` |
| `delegation_tools.go` | `delegate_task` + `route_to_capability` (polls DB synchronously) |
| `capability_router.go` | Least-loaded routing by capability tag using atomic counters |
| `memory.go` | `MemoryManager` — `Remember` / `Recall` / `RecallAll` / `BuildMemoryContext`; hourly expiry cleanup |
| `memory_tools.go` | `remember`, `recall`, `recall_all` tools |
| `llm_service.go` | `LLMService` interface + OpenClaw / Anthropic / OpenAI adapters |
| `observability.go` | Per-agent atomic metrics (submitted / completed / failed / tokens / latency) |
| `context_keys.go` | `contextKeyAgentID`, `contextKeyTaskID`, `contextKeyUserID` |
| `util.go` | `getEnvOrDefault` helper |

### Agentic loop (executor.go)

```go
for {
    resp, err := llmService.ChatCompletion(ctx, messages, tools)
    streamTokens(resp, wsHub)            // → WS agent_token_stream
    if len(resp.ToolCalls) == 0 { break }
    results := toolRegistry.Execute(ctx, resp.ToolCalls)  // parallel
    messages = append(messages, resp, results...)
}
```

### Built-in tools

| Tool | Description |
|------|-------------|
| `search_posts` | Search channel posts by keyword |
| `get_channel` | Fetch channel details by name |
| `create_post` | Post a message to a channel |
| `get_user` | Look up a user by username |
| `list_channels` | List channels in the team |
| `delegate_task` | Assign a task to a specific agent by ID |
| `route_to_capability` | Route to the least-loaded agent with a given capability |
| `remember` | Store a key-value pair in agent memory |
| `recall` | Retrieve a specific memory key |
| `recall_all` | Retrieve all memory entries for the agent |

## Store layer

**Interface:** `server/channels/store/store.go` — `Agent() AgentStore`

**Implementation:** `server/channels/store/sqlstore/agent_store.go`

### AgentStore interface

```go
type AgentStore interface {
    // Workgroup CRUD
    SaveWorkgroup(wg *model.Workgroup) (*model.Workgroup, error)
    GetWorkgroup(id string) (*model.Workgroup, error)
    UpdateWorkgroup(wg *model.Workgroup) (*model.Workgroup, error)
    DeleteWorkgroup(id string) error
    GetWorkgroupsByTeam(teamID string) ([]*model.Workgroup, error)

    // Agent definition CRUD
    SaveAgentDefinition(def *model.AgentDefinition) (*model.AgentDefinition, error)
    GetAgentDefinition(id string) (*model.AgentDefinition, error)
    GetAgentDefinitionByBotUserId(botUserId string) (*model.AgentDefinition, error)
    PatchAgentDefinition(id string, patch *model.PatchAgentDefinition) (*model.AgentDefinition, error)
    DeleteAgentDefinition(id string) error
    ListAllAgentDefinitions() ([]*model.AgentDefinition, error)

    // Task CRUD
    SaveAgentTask(task *model.AgentTask) (*model.AgentTask, error)
    GetAgentTask(id string) (*model.AgentTask, error)
    UpdateAgentTask(task *model.AgentTask) (*model.AgentTask, error)
    GetActiveTasksForAgent(agentID string) ([]*model.AgentTask, error)
    GetTaskTree(rootTaskID string) ([]*model.AgentTask, error)

    // Task events
    SaveAgentTaskEvent(event *model.AgentTaskEvent) error
    GetTaskEvents(taskID string, page, perPage int) ([]*model.AgentTaskEvent, error)

    // Memory
    UpsertAgentMemory(m *model.AgentMemory) error
    GetAgentMemory(agentID, scope string) ([]*model.AgentMemory, error)
    DeleteAgentMemory(agentID string) error
    DeleteExpiredMemory(before int64) error
}
```

## Database migrations

**File:** `server/channels/db/migrations/postgres/000156_create_agent_tables.up.sql`

Tables created:
- `Workgroups` — workgroup definitions with channel_ids JSONB
- `AgentDefinitions` — agent configuration (system prompt, model, tools, capabilities)
- `AgentTasks` — task instances with status, input, output, parent/root IDs
- `AgentTaskEvents` — streaming events (thinking steps, tool calls, token chunks)
- `AgentMemory` — persistent KV store per agent

## App layer

**File:** `server/channels/app/workgroups.go`

Contains all business logic:
- `ProvisionWorkgroup(template, teamID)` — creates workgroup + bot users + channels
- `ProvisionDemoSetup(req)` — provisions Executive + Sales workgroups + coordination channel
- `SeedDefaultWorkgroupsIfNeeded(teamID)` — idempotent bootstrap
- `ListAllAgentDefinitions()` — delegates to store
- All CRUD methods for workgroups and agent definitions

## WebSocket events

Defined in `server/public/model/websocket_message.go`:

| Constant | Value |
|----------|-------|
| `WebsocketEventAgentTaskSubmitted` | `"agent_task_submitted"` |
| `WebsocketEventAgentTaskComplete` | `"agent_task_complete"` |
| `WebsocketEventAgentTaskFailed` | `"agent_task_failed"` |
| `WebsocketEventAgentTokenStream` | `"agent_token_stream"` |
| `WebsocketEventAgentThinking` | `"agent_thinking"` |
| `WebsocketEventAgentDelegation` | `"agent_delegation"` |

## Background jobs

**`channels/jobs/agent_digest/`** — hourly digest that posts agent metrics to a designated channel. Gated by the `EnableAIAgents` feature flag.
