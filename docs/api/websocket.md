# WebSocket Events

The server broadcasts agent events over the Mattermost WebSocket connection. Connect to `ws://localhost:8065/api/v4/websocket` with a valid session token.

## Event list

| Event name | When fired | Payload fields |
|------------|-----------|----------------|
| `agent_task_submitted` | Task inserted into DB | `task_id`, `agent_id`, `status: "pending"` |
| `agent_task_complete` | Task executor finishes successfully | `task_id`, `agent_id`, `status: "complete"`, `tokens_used`, `latency_ms` |
| `agent_task_failed` | Task executor hits an error | `task_id`, `agent_id`, `status: "failed"`, `error` |
| `agent_token_stream` | Each LLM output chunk | `task_id`, `agent_id`, `token` |
| `agent_thinking` | Internal reasoning step or tool call | `task_id`, `agent_id`, `event_type`, `payload` |
| `agent_delegation` | Agent calls `delegate_task` or `route_to_capability` | `parent_task_id`, `child_task_id`, `agent_id` |

## Payload examples

### `agent_token_stream`

```json
{
  "event": "agent_token_stream",
  "data": {
    "task_id":  "task123...",
    "agent_id": "def456...",
    "token":    " enterprise"
  }
}
```

### `agent_thinking`

```json
{
  "event": "agent_thinking",
  "data": {
    "task_id":    "task123...",
    "agent_id":   "def456...",
    "event_type": "tool_call",
    "payload": {
      "tool_name": "search_posts",
      "args":      {"query": "Q3 pipeline"}
    }
  }
}
```

### `agent_delegation`

```json
{
  "event": "agent_delegation",
  "data": {
    "parent_task_id": "task123...",
    "child_task_id":  "task789...",
    "agent_id":       "sales_head_def..."
  }
}
```

## Go constants

Defined in `server/public/model/websocket_message.go`:

```go
const (
    WebsocketEventAgentTaskSubmitted = "agent_task_submitted"
    WebsocketEventAgentTaskComplete  = "agent_task_complete"
    WebsocketEventAgentTaskFailed    = "agent_task_failed"
    WebsocketEventAgentTokenStream   = "agent_token_stream"
    WebsocketEventAgentThinking      = "agent_thinking"
    WebsocketEventAgentDelegation    = "agent_delegation"
)
```

## TypeScript constants

Defined in `webapp/platform/client/src/websocket_events.ts`:

```typescript
export const AgentTaskSubmitted = 'agent_task_submitted';
export const AgentTaskComplete  = 'agent_task_complete';
export const AgentTaskFailed    = 'agent_task_failed';
export const AgentTokenStream   = 'agent_token_stream';
export const AgentThinking      = 'agent_thinking';
export const AgentDelegation    = 'agent_delegation';
```
