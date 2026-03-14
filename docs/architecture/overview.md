# Platform Overview

## Repository structure

```
mattermost/
├── server/
│   ├── public/model/               # Shared types: AgentDefinition, AgentTask, Workgroup…
│   ├── channels/
│   │   ├── app/workgroups.go       # Business logic: Workgroup CRUD, ProvisionWorkgroup
│   │   ├── app/agent_hooks.go      # @mention trigger → SubmitTask
│   │   ├── api4/agents.go          # REST handlers for agents + tasks
│   │   ├── api4/workgroups.go      # REST handlers for workgroups
│   │   ├── store/sqlstore/         # PostgreSQL: agent_store.go
│   │   ├── db/migrations/          # 000156_create_agent_tables.*
│   │   └── jobs/agent_digest/      # Hourly metrics digest job
│   └── platform/services/
│       └── agentruntime/           # Core agent runtime (see below)
└── webapp/
    ├── platform/types/src/         # TypeScript types (AgentDefinition, AgentTask…)
    ├── platform/client/src/        # Client4 API methods + WebSocket event names
    └── channels/src/
        ├── components/agent_command_center/   # /agents UI
        ├── components/agent_task_status/      # Post badge component
        ├── components/agent_thinking_panel/   # Live tool call viewer
        ├── components/agent_delegation_tree/  # DAG visualization
        ├── components/agent_task_metrics/     # Admin metrics page
        ├── sass/base/_agent_command_center.scss
        └── packages/mattermost-redux/src/
            ├── reducers/entities/             # agentDefinitions, agentMetrics, agentMemory
            ├── selectors/entities/agent_tasks.ts
            └── actions/agents.ts
```

## Data model

```
Workgroup (1)
  └── AgentDefinition (N)        — one bot user per definition
        └── AgentTask (N)        — submitted work items
              ├── AgentTaskEvent (N)  — streaming events (thinking, tool calls, tokens)
              └── root_task_id / parent_task_id  — delegation DAG

AgentMemory                      — per-agent KV store, keyed by (agent_id, key, scope)
```

## Request lifecycle

```
1. User @mentions agent in channel
        ↓
2. app/agent_hooks.go detects bot user ID → calls AgentRuntimeService.SubmitTask()
        ↓
3. AgentTask inserted into DB with status=pending
        ↓
4. Dispatcher (in-memory queue) dequeues + acquires per-agent semaphore
        ↓
5. Executor runs agentic loop:
   a. Build system prompt + memory context
   b. POST /v1/chat/completions to OpenClaw (streaming)
   c. Parse tool calls → execute in parallel via ToolRegistry
   d. Stream token chunks → WS event agent_token_stream
   e. Repeat until no tool calls in response
        ↓
6. Final output posted to channel via create_post tool (or direct post)
        ↓
7. AgentTask updated to status=complete; WS event agent_task_complete
```

## Delegation flow

When an agent calls `delegate_task` or `route_to_capability`:

1. Tool creates a child `AgentTask` with `parent_task_id` set
2. Child task is submitted to the runtime synchronously (polling until complete)
3. Child result is returned as the tool's output
4. Parent agent continues with the delegated result

`route_to_capability` selects the least-loaded agent that advertises the requested capability tag using atomic in-memory counters.

## Key design decisions

| Decision | Rationale |
|----------|-----------|
| In-process dispatcher (not a message queue) | Simpler dev setup; swap in Redis/NATS for production scale |
| Per-agent semaphore (not global) | Allows burst concurrency per agent while preventing overload |
| `ChannelTypeAgentDirect` ("A") | Makes agent-to-agent coordination observable without polluting regular channel lists |
| OpenClaw as default provider | Keeps API keys local; compatible with any OpenAI-spec backend |
| Memory stored in DB | Survives server restarts; queryable by admins |
