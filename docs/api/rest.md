# REST API Reference

All endpoints require a valid session token: `Authorization: Bearer <token>`.

Base URL: `http://localhost:8065/api/v4`

---

## Agents

### `GET /agents/definitions`

List all non-deleted agent definitions across all workgroups.

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

### `GET /agents/{agentId}`

Get a single agent definition.

### `PATCH /agents/{agentId}`

Update agent definition fields.

**Body:** `PatchAgentDefinition`

```json
{
  "display_name": "Sales Director",
  "model_id": "claude-opus-4-6",
  "max_concurrency": 10,
  "system_prompt": "Updated instructions..."
}
```

### `DELETE /agents/{agentId}`

Soft-delete an agent definition.

---

## Tasks

### `POST /agents/tasks`

Submit a task to an agent.

**Body:**

```json
{
  "agent_id":   "def1abc...",
  "channel_id": "ch1...",
  "text":       "Prepare a Q3 pipeline summary",
  "priority":   50
}
```

**Response:**

```json
{
  "id":       "task1...",
  "agent_id": "def1abc...",
  "status":   "pending",
  "input":    {"text": "Prepare a Q3 pipeline summary"},
  "create_at": 1741392000000
}
```

### `GET /agents/tasks/{taskId}`

Get a task by ID.

### `GET /agents/{agentId}/tasks/active`

Get all running or pending tasks for an agent.

### `GET /agents/tasks/{taskId}/tree`

Get the full delegation tree rooted at this task.

```json
[
  {"id": "task1...", "agent_id": "ceo...", "parent_task_id": null, "status": "complete"},
  {"id": "task2...", "agent_id": "sales_head...", "parent_task_id": "task1...", "status": "complete"},
  {"id": "task3...", "agent_id": "specialist...", "parent_task_id": "task2...", "status": "running"}
]
```

### `GET /agents/tasks/{taskId}/events`

Stream/list events for a task (thinking steps, tool calls, token chunks).

**Query params:** `per_page=100`, `page=0`

---

## Metrics

### `GET /agents/metrics`

Get per-agent metrics.

```json
[
  {
    "agent_id":        "def1...",
    "tasks_submitted": 42,
    "tasks_completed": 40,
    "tasks_failed":    2,
    "tokens_total":    120000,
    "avg_latency_ms":  2100
  }
]
```

---

## Memory

### `GET /agents/{agentId}/memory`

Get memory entries for an agent.

**Query params:** `scope=global|task|user`

```json
[
  {"key": "last_prospect", "value_text": "Acme Corp", "scope": "global"},
  {"key": "email_style", "value_text": "concise and direct", "scope": "global"}
]
```

### `DELETE /agents/{agentId}/memory`

Clear all memory for an agent.

---

## Workgroups

### `GET /workgroups`

List all workgroups.

### `POST /workgroups`

Create a workgroup from a template.

```json
{
  "template_id": "marketing",
  "team_id": "team1..."
}
```

### `GET /workgroups/{workgroupId}`

Get a workgroup by ID.

### `DELETE /workgroups/{workgroupId}`

Delete a workgroup.

---

## Demo setup

### `POST /agents/demo-setup`

Provision Executive + Sales workgroups, coordination channels, and bot users in one call.

**Body:**

```json
{
  "team_id":           "team1...",
  "observer_user_ids": ["admin_user_id..."],
  "llm_service_id":    "openclaw"
}
```

**Response:**

```json
{
  "executive_workgroup":       { "id": "wg_exec...", "channel_ids": {...} },
  "sales_workgroup":           { "id": "wg_sales...", "channel_ids": {...} },
  "coordination_channel_id":   "ch_coord...",
  "executive_agent_id":        "def_ceo...",
  "sales_head_agent_id":       "def_saleshead...",
  "sales_specialist_agent_ids": ["def_prospecting...", "def_proposal..."]
}
```

---

## Coordination channels

### `GET /agents/coordination-channels`

List all `ChannelTypeAgentDirect` ("A") channels for a team.

**Query params:** `team_id=<id>`

Returns all observable agent-to-agent coordination channels.
