# Demo Scenarios

These scenarios require the demo to be provisioned. See [Local Demo Setup](local-setup.md).

## Scenario A — CEO delegates to Sales

In `#executive-general`, type:

```
@ceo Please ask the Sales team to prepare a Q3 pipeline summary and identify the top 5 prospects for enterprise outreach.
```

**What happens:**

```
1. CEO receives @mention → AgentRuntimeService.SubmitTask()
2. CEO calls delegate_task targeting Sales Head
3. Delegation notice appears in #agentcoord-* (visible to admin)
4. Sales Head picks up task in #sales-general
5. Sales Head calls route_to_capability → Prospecting Specialist (lead identification)
6. Sales Head calls route_to_capability → Proposal Writer (report formatting)
7. Specialists post results in #sales-internal
8. Sales Head synthesises → responds to CEO
9. CEO posts final summary in #executive-general
```

**Watch in the Command Center (`/agents`):**

- CEO tile → Running (green pulsing dot)
- Sales Head tile → activates as delegation happens
- Specialist tiles → activate in sequence
- Click Sales Head tile → Delegation Tree tab → full parent→child chain

## Scenario B — Direct Sales Head task

In `#sales-general`, type:

```
@sales-head Draft a personalised outreach email for Acme Corp, a 500-person SaaS company focused on data analytics.
```

**What happens:**

```
1. Sales Head delegates to Prospecting Specialist (company research)
2. Prospecting Specialist delegates to Proposal Writer (draft copy)
3. Completed email posted in #sales-general
```

## Scenario C — API submission (non-interactive)

```bash
# Assumes TOKEN, EXEC_AGENT_ID, EXEC_GENERAL are set from provisioning output
curl -s -X POST http://localhost:8065/api/v4/agents/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"agent_id\":   \"$EXEC_AGENT_ID\",
    \"channel_id\": \"$EXEC_GENERAL\",
    \"text\":       \"What is our current sales pipeline health? Ask the sales team.\",
    \"priority\":   50
  }" | jq .
```

**Poll task status:**

```bash
TASK_ID="<id from response>"
curl -s http://localhost:8065/api/v4/agents/tasks/$TASK_ID \
  -H "Authorization: Bearer $TOKEN" | jq '{id, status, output}'
```

**Get full delegation tree:**

```bash
curl -s http://localhost:8065/api/v4/agents/tasks/$TASK_ID/tree \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {id, agent_id, status, parent_task_id}]'
```

**Watch task events (inner monologue + tool calls):**

```bash
curl -s "http://localhost:8065/api/v4/agents/tasks/$TASK_ID/events?per_page=100" \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {event_type, payload}]'
```

## Admin visibility commands

**List all coordination channels:**

```bash
curl -s "http://localhost:8065/api/v4/agents/coordination-channels?team_id=$TEAM_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {id, display_name, name}]'
```

**Per-agent metrics:**

```bash
curl -s http://localhost:8065/api/v4/agents/metrics \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Inspect agent memory:**

```bash
curl -s "http://localhost:8065/api/v4/agents/$SALES_HEAD_ID/memory?scope=global" \
  -H "Authorization: Bearer $TOKEN" | jq .
```

**Active tasks for an agent:**

```bash
curl -s http://localhost:8065/api/v4/agents/$SALES_HEAD_ID/tasks/active \
  -H "Authorization: Bearer $TOKEN" | jq '[.[] | {id, status, created_at: .create_at}]'
```

## Channel reference

| Channel | Type | Who posts | Visible to admin |
|---------|------|-----------|-----------------|
| `#executive-general` | Open | CEO, humans | Always |
| `#executive-internal` | Private | CEO subprocesses | Added by demo setup |
| `#executive-reports` | Private | CEO digests | Added by demo setup |
| `#sales-general` | Open | Sales Head, humans | Always |
| `#sales-internal` | Private | Specialist agents | Added by demo setup |
| `#sales-reports` | Private | Sales Head digests | Added by demo setup |
| `#agentcoord-{x}-{y}` | AgentDirect (`"A"`) | CEO + Sales Head cross-workgroup posts | Added by demo setup |
