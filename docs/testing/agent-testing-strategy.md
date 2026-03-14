# Agent Testing Strategy

This document covers the research and recommended strategy for testing the Mattermost multi-agent orchestration platform — specifically the OpenClaw-backed agent runtime — across both CI smoke testing and live LLM integration testing.

---

## TL;DR

The industry-wide consensus for AI agent CI is a **two-tier split**:

- **Tier 1 — Smoke (every PR, $0, <2 min):** Mock LLM server returning canned responses. Tests orchestration logic, delegation chains, tool call schemas, memory operations, WebSocket dispatch. Zero API calls, zero flakiness.
- **Tier 2 — Live (on-demand / nightly, real $, ~30 min):** Real LLM providers. Tests end-to-end task completion, prompt quality regression, agent behavior under real model outputs. Gated behind human approval in CI.

This is explicitly OpenClaw's own model: their test suite has `pnpm test` (deterministic, CI), `pnpm test:e2e` (gateway smoke, optional CI), and `pnpm test:live` — documented as "not CI-stable by design."

---

## OpenClaw — Current State and Testing Model

### What OpenClaw Is

OpenClaw is a self-hosted personal AI assistant gateway. It routes LLM calls across 20+ messaging platforms (Slack, Mattermost, WhatsApp, Telegram, Discord, etc.) and exposes an OpenAI-compatible HTTP chat completions endpoint at `http://127.0.0.1:18789/v1/chat/completions`.

The Mattermost agent runtime uses OpenClaw as its default LLM backend:

- Agent routing via `x-openclaw-agent-id: <agentDefinition.Id>` request header
- Token: `MM_AGENTS_OPENCLAW_TOKEN` or `OPENCLAW_GATEWAY_TOKEN` env var
- Config: `gateway.http.endpoints.chatCompletions.enabled: true`

The internal workflow engine ("Lobster") handles sequencing, retry counting, and conditional routing in YAML. LLMs handle content generation; Lobster handles deterministic control flow.

### OpenClaw's Three-Tier Test Architecture

| Tier | Command | What It Tests | CI | Cost |
|------|---------|--------------|-----|------|
| Unit/Integration | `pnpm test` | In-process logic, deterministic | Yes | $0 |
| E2E / Gateway Smoke | `pnpm test:e2e` | Multi-instance WS/HTTP, no credentials | Optional | $0 |
| Live | `pnpm test:live` | Full pipeline: gateway → agent → model → tools | No | Real $ |

Live tests are narrowed by the `OPENCLAW_LIVE_MODELS` env var. OpenClaw explicitly documents: **"live tests do not run in CI — not CI-stable by design."**

### OpenClaw + Promptfoo Integration

Promptfoo (acquired by OpenAI in March 2026) has native OpenClaw support with four provider types:

```yaml
providers:
  - id: openclaw:main              # Chat completions
  - id: openclaw:responses:main    # OpenResponses API
  - id: openclaw:agent:main        # Full WebSocket agent + streaming
  - id: openclaw:tools:search_posts  # Direct tool invocation
```

This makes promptfoo the natural eval layer sitting above the OpenClaw gateway for Tier 2 live testing.

### Deterministic Multi-Agent Pipelines in OpenClaw

OpenClaw community case studies demonstrate that reliable delegation chains keep LLMs out of routing decisions entirely. The pattern:

- **LLMs:** Content generation only
- **Lobster YAML:** Sequencing, retry counting, conditional routing
- **Structured output:** LLM responses pass through JSON schema (`{approved: bool, feedback: string}`) evaluated by shell conditions
- **Loop construct:** `sub-lobster` steps with `loop.maxIterations` + `loop.condition` for iterative workflows

**Testing implication:** If delegation logic lives in deterministic orchestration code (not LLM decisions), it can be tested without any LLM at all.

---

## Tier 1: Deterministic / Smoke Testing (CI-Safe)

### The Core Tool: llmock

[llmock](https://github.com/CopilotKit/llmock) is a deterministic mock LLM server that runs as a real HTTP process. It is the recommended tool for Go server testing.

**Why llmock over other options:**

- Works **cross-process** — child processes inherit `OPENAI_BASE_URL` env var, so the real Go server under test calls the mock just like it would call real OpenAI/Anthropic
- Supports OpenAI (Chat Completions + Responses API), Anthropic Claude (Messages), and Google Gemini wire formats
- SSE streaming in correct provider wire format with configurable chunk size and latency simulation
- Full tool call support — critical for testing agent tool invocations
- Zero Node.js dependencies; 92 stars, active development

**Usage pattern for Go server tests:**

```go
// TestMain sets up llmock once for the whole package
func TestMain(m *testing.M) {
    mock := llmock.New()
    mock.AddFixture("task_delegation", llmock.Fixture{
        SystemPromptContains: "You are the CEO agent",
        Response: llmock.Response{
            Content: `I'll delegate this to the Sales team.`,
            ToolCalls: []llmock.ToolCall{{
                Name: "delegate_task",
                Arguments: `{"agent_id":"sales-head","message":"Prepare Q3 pipeline summary"}`,
            }},
        },
    })
    mock.Start()
    os.Setenv("OPENAI_BASE_URL", mock.URL())
    // Also override OpenClaw gateway URL to point at llmock
    os.Setenv("OPENCLAW_GATEWAY_URL", mock.URL())
    os.Exit(m.Run())
    mock.Stop()
}
```

### Cassette / JSON Fixture Pattern (Go)

Since no Go-native VCR library exists, the recommended pattern is a hand-rolled `TestProvider` that loads JSON cassette files:

```
server/platform/services/agentruntime/testdata/cassettes/
  task_submission.json
  delegation_ceo_to_sales.json
  memory_recall.json
  tool_search_posts.json
  error_retry.json
```

Each cassette captures:
```json
{
  "scenario": "delegation_ceo_to_sales",
  "request": {
    "model": "gpt-4o",
    "system_prompt_contains": "You are the CEO agent",
    "messages": [...]
  },
  "response": {
    "content": "I'll delegate this to the Sales team.",
    "tool_calls": [
      {"name": "delegate_task", "arguments": {...}}
    ],
    "usage": {"prompt_tokens": 412, "completion_tokens": 38}
  }
}
```

**Record mode** (`RECORD=true` env var): call real LLM, serialize to cassette file.
**Replay mode** (CI default): load cassette, return without API call.

### What Tier 1 Tests Cover

| Test Area | Mechanism | What's Validated |
|-----------|-----------|-----------------|
| Agent executor loop | llmock canned response | Correct tool call parsing, context construction, post patching |
| Delegation routing | Cassette with tool_calls fixture | `delegate_task` / `route_to_capability` invoked with correct args |
| Memory tools | Direct unit tests (no LLM) | `Remember`, `Recall`, `RecallAll`, `BuildMemoryContext`, expiry |
| Tool schema validation | llmock → tool registry | Missing required args, unknown tool names, malformed JSON |
| Retry behavior | llmock returning errors | Exponential backoff, max retry counts, error propagation |
| WebSocket dispatch | Go test + mock WS client | `agent_token_stream`, `agent_thinking`, `agent_task_*` events emitted in correct order |
| Capability router | Unit test (no LLM) | Least-loaded routing, atomic counter behavior, concurrent access |
| Digest job | Mock `AppIface` | Correct channel posted to, feature flag respected, missing job data no-op |

### Frontend Tier 1 (Jest + MSW)

[MSW (Mock Service Worker)](https://mswjs.io/) supports WebSocket mocking in jsdom environments for testing React components:

```typescript
// Test Redux reducer handles agent_token_stream event
const server = setupServer(
  ws.link('ws://localhost:8065/api/v4/websocket').on('connection', ({ client }) => {
    client.send(JSON.stringify({
      event: 'agent_token_stream',
      data: { task_id: 'task-123', token: 'Delegating', done: false }
    }));
  })
);
```

Tests to add in `webapp/channels/src/components/agent_command_center/__tests__/`:
- Token stream renders in real-time
- Task status badge transitions: `pending → running → complete`
- Delegation tree renders correct parent/child edges
- Metrics dashboard fetches and displays agent metrics

---

## Tier 2: Live LLM Testing (On-Demand / Nightly)

### GitHub Actions: Still Viable for Live Tests

GitHub Actions is suitable for live LLM tests with the right gate mechanism:

**Environment Protection Rules** (the recommended approach):

1. Create a GitHub Environment named `live-agent-tests`
2. Add required reviewers (team members who approve live test runs)
3. Scope `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `MM_AGENTS_OPENCLAW_TOKEN` secrets to this environment only
4. Jobs referencing `environment: live-agent-tests` pause for human approval before executing

**`workflow_dispatch` trigger** for manual runs:

```yaml
on:
  workflow_dispatch:
    inputs:
      llm_provider:
        description: 'LLM provider to test against'
        required: true
        default: 'anthropic'
        type: choice
        options: [anthropic, openai, openclaw]
      scenario:
        description: 'Test scenario to run'
        default: 'all'
```

**GitHub Actions limitations to know:**

- Hard 6-hour per-job limit on hosted runners (cannot be extended)
- For long-running delegation chain tests (hour+), use self-hosted runners or Buildkite (see below)
- Non-determinism is inherent — use `continue-on-error: true` and score thresholds rather than pass/fail
- Fork PRs cannot access secrets — natural guardrail preventing untrusted code from triggering API calls

### Promptfoo for Live Eval CI

```yaml
# .github/workflows/agent-live-eval.yml
name: Live Agent Evaluation

on:
  workflow_dispatch:
    inputs:
      provider: {default: openclaw, type: choice, options: [openclaw, anthropic, openai]}
  schedule:
    - cron: '0 2 * * *'  # nightly at 2am

jobs:
  eval:
    runs-on: ubuntu-latest
    environment: live-agent-tests   # requires reviewer approval
    steps:
      - uses: actions/checkout@v4
      - uses: cachix/install-nix-action@v27
      - name: Start CI environment
        run: nix develop --command bash scripts/ci-up.sh

      - name: Run promptfoo evals
        run: |
          nix develop --command bash -c "
            npx promptfoo eval \
              --config evals/agent-behaviors.yaml \
              --output evals/results/$(date +%Y%m%d).json
          "
        env:
          OPENCLAW_GATEWAY_URL: http://localhost:18789
          MM_AGENTS_OPENCLAW_TOKEN: ${{ secrets.MM_AGENTS_OPENCLAW_TOKEN }}
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

      - name: Post results to Mattermost
        if: always()
        run: bash scripts/post-eval-results.sh
```

### Live Test Scenarios

What Tier 2 tests that Tier 1 cannot:

| Scenario | Why It Needs a Real LLM | Acceptance Threshold |
|----------|------------------------|---------------------|
| CEO → Sales Head delegation | Requires LLM to choose `delegate_task` tool unprompted | Tool called in ≥80% of runs |
| Multi-turn task refinement | LLM must maintain context across turns | Task completed in ≤5 turns, 75% success rate |
| Memory recall influences response | LLM must use recalled memories in output | Memory context referenced in ≥70% of runs |
| Capability routing by tag | LLM response must trigger correct capability | Correct agent selected ≥85% of runs |
| Error recovery | LLM must handle tool failure gracefully | No infinite loop in 100% of runs |

### Go Live Test Build Tag Pattern

Mirrors OpenClaw's own `*.live.test.ts` approach, using Go build tags:

```go
//go:build live
// +build live

package agentruntime_test

// TestCEODelegationLive tests the full CEO→Sales delegation chain
// against a real LLM provider.
// Run: go test -tags=live ./platform/services/agentruntime/... -v
func TestCEODelegationLive(t *testing.T) {
    // ...
}
```

CI runs `go test ./...` (no `-tags=live`); live tests run `go test -tags=live ./...`.

---

## Tier 3: Long-Running / Nightly Benchmarks

For multi-step delegation chains that run for minutes or hours, GitHub Actions' 6-hour limit may not be sufficient. Options:

### Option A: GitHub Actions Self-Hosted Runners

- No per-job timeout limit
- Run on your own infrastructure (EC2, bare metal)
- Full access to long-running processes
- Setup: register runner with `gh` CLI; use `runs-on: self-hosted` in workflow

### Option B: Buildkite

[Buildkite](https://buildkite.com/) supports self-hosted agents with no timeout restrictions. Active investments in "agentic CI" (2025): dynamic pipeline generation where agents modify pipeline steps at runtime, Claude/Codex/Bedrock-powered build summaries.

```yaml
# buildkite pipeline for long-running agent benchmarks
steps:
  - label: "Agent Benchmark Suite"
    command: "mise run agents:benchmark"
    timeout_in_minutes: 480  # 8 hours
    agents:
      queue: gpu-runners  # your own infra
```

### Option C: Fly.io Sprites (Persistent Test Environment)

[Fly.io Sprites](https://fly.io/docs/machines/) are persistent VMs that auto-idle when inactive (billing pauses) but preserve state between runs. ~$4-6/month.

**Pattern:** Keep a permanent Mattermost test instance running on Fly.io with agents pre-configured. CI live tests point at this always-on environment instead of spinning up a new one. Tests run in seconds (no startup cost).

```
fly launch --vm-memory 2048 --region iad
# Machine idles when no CI job is running
# Wakes on first HTTP request (~2s cold start)
# State (PostgreSQL, agent configs) persists across CI runs
```

### Option D: AWS Step Functions (Production-Grade)

For teams already on AWS, Step Functions + CodeBuild is the most principled model for multi-step delegation chain tests:

- **No timeout limits** (Step Functions state machines can run for up to 1 year)
- **Native Bedrock integration** for invoking LLM calls within state machine steps
- **Full audit trail** — every state transition is logged
- **Parallel execution** — fan-out across multiple agent test scenarios simultaneously

---

## Eval Platforms Compared

| Platform | Best For | Pricing | GitHub Actions Integration | OpenClaw Support |
|----------|----------|---------|--------------------------|-----------------|
| **promptfoo** | Eval configs, A/B prompt testing, red teaming | Free OSS + paid cloud | Native GitHub Action | Native (4 provider types) |
| **Braintrust** | Score regression, PR comments, LLM-as-judge | Usage-based | `braintrustdata/eval-action@v1` | Via OpenAI-compatible endpoint |
| **LangSmith** | Multi-turn trajectory analysis, dataset management | Free tier + paid | Control Plane API | Via LangChain integration |
| **DeepEval** | pytest-compatible, agent trajectory scoring | Free OSS + paid cloud | `deepeval test run` | Via OpenAI-compatible endpoint |

**Recommendation:** Start with **promptfoo** (native OpenClaw support, acquired by OpenAI, best-in-class CI integration) for Tier 2. Add **Braintrust** PR comment integration when you want score regression gating.

---

## Testing Agent Memory in Isolation

Memory testing is challenging because quality depends on the whole system. Recommended layers:

**Layer 1 — Store unit tests (no LLM):**
Test `MemoryManager.Remember()` / `Recall()` / `RecallAll()` directly against the test PostgreSQL instance. Verify: round-trip persistence, expiry cleanup at scheduled intervals, bank isolation by agent ID + user ID.

**Layer 2 — Context construction unit tests (no LLM):**
Given a fixed set of pre-seeded memories, assert `BuildMemoryContext()` produces the expected context string with memories in the correct order and format.

**Layer 3 — Memory influence integration tests (llmock):**
Pre-seed memories → run agent executor against llmock → inspect the HTTP request llmock recorded → assert the prompt sent to the LLM includes the expected memory context. Tests the remember→recall loop without needing real LLM responses.

**Layer 4 — Live memory persistence (Tier 2):**
Run a multi-session agent interaction with real LLM. Session 1 establishes facts ("The Q3 target is $2M"). Session 2 asks a question requiring recall. Assert recalled fact appears in response.

**CI isolation pattern:** Each test creates unique memory bank IDs (UUID-based) so parallel test runs don't contaminate each other.

---

## Testing WebSocket Streaming

The Mattermost agent runtime emits these WebSocket events during task execution:

```
agent_task_submitted → agent_thinking (0..N) → agent_token_stream (0..N) → agent_task_complete/failed
```

**Go server-side testing:**

```go
// Use gorilla/websocket test helpers or Mattermost's TestHelper
// llmock streams SSE tokens; assert Go server forwards them as WS events

func TestAgentTokenStreamForwarding(t *testing.T) {
    mock := llmock.New()
    mock.AddStreamingFixture("ceo_response", []string{
        "I'll ", "delegate ", "this ", "to ", "Sales.",
    })
    // ... connect WS client, submit task, assert events received in order
}
```

**Frontend testing (MSW):**

```typescript
// Assert AgentCommandCenter tile shows streaming tokens in real-time
it('streams tokens into tile during task execution', async () => {
  server.use(
    ws.link('ws://localhost:8065').on('connection', ({ client }) => {
      client.send(event('agent_task_submitted', { task_id: 'T1' }));
      client.send(event('agent_token_stream', { task_id: 'T1', token: 'Delegating...' }));
      client.send(event('agent_task_complete', { task_id: 'T1' }));
    })
  );
  // render <AgentCommandCenter /> and assert token appears in tile
});
```

---

## Implementation Roadmap

The following is the planned implementation sequence. None of this is implemented yet — this section documents intent for future development.

### Phase 1: Tier 1 Foundation (Next Sprint)

- [ ] Add llmock dependency to server test setup (`TestMain` in `agentruntime` package)
- [ ] Create cassette directory: `server/platform/services/agentruntime/testdata/cassettes/`
- [ ] Write `TestProvider` struct with record/replay mode driven by `RECORD` env var
- [ ] Cassettes for: task_submission, delegation, memory_recall, tool_error, retry
- [ ] Add `//go:build live` tag pattern to separate live tests from unit tests
- [ ] Add `test:agent:smoke` mise task: `go test ./platform/services/agentruntime/... -count=1`
- [ ] Frontend: add MSW WebSocket handlers for agent events in Jest setup

### Phase 2: Live Eval Workflow (Following Sprint)

- [ ] Create `evals/` directory with promptfoo YAML configs for each agent scenario
- [ ] Create `.github/workflows/agent-live-eval.yml` with `workflow_dispatch` + environment gate
- [ ] Create GitHub Environment `live-agent-tests` with required reviewer
- [ ] Scope `MM_AGENTS_OPENCLAW_TOKEN`, `ANTHROPIC_API_KEY` to that environment
- [ ] Add `scripts/post-eval-results.sh` — post promptfoo JSON results to a Mattermost channel
- [ ] Add `test:agent:live` mise task: `go test -tags=live ./platform/services/agentruntime/... -v`

### Phase 3: Score Regression (Future)

- [ ] Integrate Braintrust `eval-action` for PR comment score diffs
- [ ] Define acceptance thresholds per scenario (table in Tier 2 section above)
- [ ] Add nightly scheduled eval run with results posted to `#agent-ci` channel
- [ ] Explore Fly.io Sprites for persistent always-on test Mattermost instance

### Phase 4: Long-Running Benchmarks (Future)

- [ ] Evaluate Buildkite self-hosted runners vs GH Actions self-hosted for >6h tests
- [ ] Implement delegation chain benchmarks (CEO→Sales→Specialist full round-trip)
- [ ] Multi-session memory persistence tests
- [ ] Capability router load tests (concurrent agent task submission)

---

## Key References

| Resource | URL | Notes |
|----------|-----|-------|
| OpenClaw Testing Docs | https://open-claw.bot/docs/help/development/testing/ | Three-tier model, live test rationale |
| OpenClaw Chat Completions API | https://docs.openclaw.ai/gateway/openai-http-api | `x-openclaw-agent-id` header usage |
| Promptfoo OpenClaw Provider | https://www.promptfoo.dev/docs/providers/openclaw/ | 4 provider types, YAML config examples |
| Promptfoo CI/CD Integration | https://www.promptfoo.dev/docs/integrations/ci-cd/ | GitHub Actions action, quality gates |
| llmock (CopilotKit) | https://github.com/CopilotKit/llmock | Cross-process mock LLM server |
| Block Engineering Testing Pyramid | https://engineering.block.xyz/blog/testing-pyramid-for-ai-agents | Four-layer pyramid pattern |
| Braintrust eval-action | https://github.com/braintrustdata/eval-action | PR score diff comments |
| DeepEval CI/CD | https://deepeval.com/docs/evaluation-unit-testing-in-ci-cd | pytest-compatible agent evals |
| BAML VCR | https://github.com/gr-b/baml_vcr | Cassette recording for streaming |
| Fly.io Sprites | https://fly.io/docs/machines/ | Persistent VMs for always-on test env |
| Buildkite Agentic CI | https://buildkite.com/resources/blog/building-ai-powered-ci-workflows-three-practical-examples/ | LLM-aware pipeline features |
| Temporal AI Agents | https://temporal.io/solutions/ai | Durable workflow for delegation testing |
| Deterministic Multi-Agent in OpenClaw | https://dev.to/ggondim/how-i-built-a-deterministic-multi-agent-dev-pipeline-inside-openclaw | Lobster workflow engine pattern |
