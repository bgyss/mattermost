# Customising the Demo

## Use Anthropic instead of OpenClaw

In `.env.agents`:

```bash
MM_AGENTS_ANTHROPIC_API_KEY=sk-ant-...
MM_AGENTS_LLM_SERVICE=anthropic
```

Re-run `mise run agents:provision` — it will pass `"llm_service_id": "anthropic"` to the setup endpoint.

Or patch an existing agent definition:

```bash
curl -s -X PATCH http://localhost:8065/api/v4/agents/$AGENT_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"llm_service_id": "anthropic", "model_id": "claude-opus-4-6"}'
```

## Override agent system prompts

Patch any agent definition after provisioning:

```bash
curl -s -X PATCH http://localhost:8065/api/v4/agents/$SALES_HEAD_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "system_prompt": "You are a highly aggressive closer. Prioritise big-ticket enterprise deals above all else."
  }'
```

Or use the **Settings** tab in the Agent Command Center (`/agents`).

## Add a second department workgroup

Any additional workgroup provisioned into the same team gets a coordination channel with the Executive workgroup automatically:

```bash
# Marketing
curl -s -X POST http://localhost:8065/api/v4/workgroups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"template_id\": \"marketing\", \"team_id\": \"$TEAM_ID\"}"
```

**Available template IDs:**

| Template | Department |
|----------|-----------|
| `executive` | CEO / Executive Office |
| `marketing` | Marketing |
| `engineering` | Engineering |
| `sales` | Sales |
| `finance` | Finance |
| `research` | Research & Development |
| `product` | Product Management |
| `manufacturing` | Manufacturing |
| `supply_chain` | Supply Chain |

## Run with a different model per agent

After provisioning, patch individual agents to use different models:

```bash
# Expensive model for CEO
curl -X PATCH .../api/v4/agents/$CEO_ID \
  -d '{"model_id": "claude-opus-4-6"}'

# Cheaper model for specialists
curl -X PATCH .../api/v4/agents/$SPECIALIST_ID \
  -d '{"model_id": "claude-haiku-4-5-20251001"}'
```

## Clear agent memory

If an agent is behaving unexpectedly due to stale memory:

```bash
curl -X DELETE http://localhost:8065/api/v4/agents/$AGENT_ID/memory \
  -H "Authorization: Bearer $TOKEN"
```

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Agents don't respond | `MM_AGENTS_OPENCLAW_TOKEN` set? Gateway running on port 18789? |
| `"agent runtime not available"` errors | Look for `"agent runtime started"` in server logs |
| Tasks stuck at `pending` | Dispatcher DB poller recovers abandoned tasks every 10 seconds — check dispatcher logs |
| No posts in coordination channel | Verify both workgroups' `ChannelIds.CoordinationChannels` is populated via `GET /api/v4/workgroups/{id}` |
| Coordination channel not visible | Confirm demo-setup was run with your user ID in `observer_user_ids` |
