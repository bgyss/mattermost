# WebSocket Events

The command center reacts in real time to server-pushed WebSocket events. All handlers live in `webapp/channels/src/actions/websocket_actions.ts`.

## Event reference

| WS Event | Handler | Redux Effect |
|----------|---------|--------|
| `agent_task_submitted` | `handleAgentTaskUpdate` | Upsert task in `agentTasks.byId` |
| `agent_task_complete` | `handleAgentTaskUpdate` | Update task status to `complete` |
| `agent_task_failed` | `handleAgentTaskUpdate` | Update task status to `failed` |
| `agent_token_stream` | `handleAgentTokenStream` | Append token chunk to `task.output.text` |
| `agent_delegation` | `handleAgentDelegation` | Insert partial child task for DAG rendering |
| `agent_thinking` | `handleAgentThinking` | Insert `AgentTaskEvent` for thinking panel |

## Token stream

`agent_token_stream` is the mechanism by which tile bodies update in real time. Each event carries:

```typescript
{
  task_id: string,
  token:   string,    // one chunk of text
  agent_id: string
}
```

The handler appends to `state.entities.agentTasks.byId[task_id].output.text`. The tile body re-renders and shows the last 200 characters.

## Delegation events

`agent_delegation` carries partial task data before the child task is persisted to DB:

```typescript
{
  parent_task_id: string,
  child_task_id:  string,
  agent_id:       string,
  status:         'pending'
}
```

This allows the Delegation Tree tab to render in real time as delegation happens, rather than waiting for the full DB write.

## Server-side event publishing

Events are published in `server/platform/services/agentruntime/executor.go` via the WebSocket hub:

```go
hub.BroadcastWebSocketEvent(model.WebsocketEventAgentTokenStream, map[string]any{
    "task_id":  task.Id,
    "agent_id": task.AgentId,
    "token":    chunk,
}, "")
```

## Troubleshooting

**Token stream not updating:**
- Check browser console for WebSocket errors.
- Confirm the `AgentRuntimeService` is running — look for `"agent runtime started"` in server logs.
- Confirm `MM_AGENTS_OPENCLAW_TOKEN` is set.

**Status pill stuck on Idle:**
- Confirm the task was submitted to the correct agent (matching `agent_id`).
- Verify `agent_task_submitted` event fires in browser dev tools → Network → WS.
