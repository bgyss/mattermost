# LLM Providers

## Provider selection

The provider used for each agent is determined by `AgentDefinition.LLMServiceId`. The runtime resolves this in `llm_service.go`:

| `llm_service_id` | Provider | Env var |
|------------------|----------|---------|
| `"openclaw"` (default) | Local OpenClaw gateway | `MM_AGENTS_OPENCLAW_TOKEN` |
| `"anthropic"` | Anthropic API directly | `MM_AGENTS_ANTHROPIC_API_KEY` |
| `"openai"` | OpenAI API | `MM_AGENTS_OPENAI_API_KEY` |

`AgentDefinition.LLMServiceId` defaults to `"openclaw"` in `PreSave()`.

## OpenClaw (recommended for local dev)

OpenClaw is a local LLM gateway that exposes an OpenAI-compatible `/v1/chat/completions` endpoint and proxies requests to your upstream provider.

### Configuration

```bash
# In .env.agents
MM_AGENTS_OPENCLAW_TOKEN=your-token
OPENCLAW_GATEWAY_URL=http://127.0.0.1:18789   # default
```

### Agent routing

The server sends `x-openclaw-agent-id: <agentDefinition.Id>` as a request header so OpenClaw can apply per-agent routing rules (e.g. different models per department).

### Required OpenClaw config

```yaml
gateway:
  http:
    endpoints:
      chatCompletions:
        enabled: true
```

## Anthropic (direct)

```bash
# In .env.agents
MM_AGENTS_ANTHROPIC_API_KEY=sk-ant-...
MM_AGENTS_LLM_SERVICE=anthropic
```

Pass `"llm_service_id": "anthropic"` in the demo setup request:

```bash
curl -X POST .../api/v4/agents/demo-setup \
  -d '{"llm_service_id": "anthropic", ...}'
```

## OpenAI

```bash
MM_AGENTS_OPENAI_API_KEY=sk-...
```

Pass `"llm_service_id": "openai"`.

## Switching providers per agent

Each `AgentDefinition` can target a different provider. After provisioning, patch an agent:

```bash
curl -X PATCH http://localhost:8065/api/v4/agents/$AGENT_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"llm_service_id": "anthropic", "model_id": "claude-opus-4-6"}'
```

Or use the **Settings** tab in the Agent Command Center.

## Adding a custom provider

Implement the `LLMService` interface in `server/platform/services/agentruntime/llm_service.go`:

```go
type LLMService interface {
    ChatCompletion(ctx context.Context, req ChatCompletionRequest) (<-chan ChatEvent, error)
}
```

Register it in `NewLLMService()` with a new service ID string.
