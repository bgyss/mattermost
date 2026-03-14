# CI Platform Comparison for Live Agent Testing

GitHub Actions is the right choice for smoke/deterministic tests and for gated live tests that complete within 6 hours. For long-running benchmarks and always-on test environments, other platforms are worth considering. This document provides a detailed comparison.

---

## GitHub Actions (Current Platform)

### Strengths
- Already in use; zero migration cost
- Native `workflow_dispatch` for manual live test triggers
- **Environment Protection Rules** — require human reviewer approval before secrets are exposed and live tests run. This is the cleanest gating mechanism available.
- Fork PRs cannot access secrets by default — natural guardrail preventing untrusted code from triggering API calls
- 2,000 free minutes/month; competitive pricing for short jobs

### Limitations
- **Hard 6-hour per-job limit** on hosted runners — cannot be extended
- Non-deterministic LLM tests will appear "flaky" to GitHub's flaky test detection
- No native LLM eval result display in the PR UI (requires third-party like Braintrust)

### Recommended Gate Pattern

```yaml
jobs:
  live-agent-eval:
    runs-on: ubuntu-latest
    environment: live-agent-tests  # pauses until reviewer approves
    steps:
      - ...
```

Create the `live-agent-tests` GitHub Environment with:

1. **Required reviewers** — add team members who should approve live test runs
2. **Branch restrictions** — only allow from `main` or `release/*`
3. **Secrets scoped to this environment** — `ANTHROPIC_API_KEY`, `MM_AGENTS_OPENCLAW_TOKEN`

### Verdict

✅ **Use for:** Smoke tests (Tier 1), gated live evals completing under 6 hours (Tier 2), nightly promptfoo eval schedules

---

## Buildkite (Self-Hosted)

### What It Is

Commercial CI/CD platform with both hosted and self-hosted agents. Self-hosted agents have no per-job timeout restrictions. Buildkite invested heavily in "Agentic CI" features in 2025:

- Dynamic pipeline generation — LLM agents can modify pipeline steps at runtime
- Claude/Codex/Bedrock-powered build summaries
- SDK for composing pipeline definitions in JS, TS, Python, or Go at runtime

### Strengths
- **No timeout limit** on self-hosted agents — run delegation chain tests for hours
- Go SDK for pipeline composition is natural for this Go-based project
- Self-healing pipeline pattern: agent fixes CI failure, re-runs tests
- Can run on your own GPU hardware if needed

### Limitations
- Commercial cost + infrastructure cost (running your own machines)
- Requires setting up and maintaining self-hosted agent infrastructure
- More operational overhead than GH Actions

### Verdict

✅ **Use for:** Long-running delegation chain benchmarks (Tier 3) that exceed 6 hours; teams with existing self-hosted runner infrastructure

---

## Fly.io Sprites (Always-On Test Environment)

### What It Is

[Fly.io Sprites](https://fly.io/docs/machines/) are persistent VMs that:
- Auto-idle when inactive (billing pauses at ~$0.002/min when idle)
- Preserve disk state between idle periods (PostgreSQL data, agent configs, channel history)
- Boot from idle in ~2 seconds on first HTTP request
- Cost: ~$4-6/month for a 1 CPU / 1GB RAM machine

### Use Case for Agent Testing

Instead of spinning up a fresh Mattermost instance for every CI run (30-60 second startup), keep a **permanent test Mattermost instance** running on Fly.io with:
- Admin user pre-created
- Team and channels configured
- Agent definitions provisioned
- OpenClaw gateway connected

CI live test jobs authenticate against this always-on instance rather than running `scripts/ci-up.sh`. Tests complete faster because infrastructure startup cost is paid once, not per run.

```yaml
# In .github/workflows/agent-live-eval.yml
env:
  MM_BASE_URL: https://mm-test.your-org.fly.dev
  MM_ADMIN_EMAIL: ${{ secrets.FLY_TEST_ADMIN_EMAIL }}
  MM_ADMIN_PASSWORD: ${{ secrets.FLY_TEST_ADMIN_PASSWORD }}
```

### Reset Strategy

For a clean slate: call a reset endpoint or run `scripts/ci-reset.sh` against the Fly.io instance via SSH. No need to tear down and rebuild the whole VM.

### Verdict

✅ **Use for:** Persistent Tier 2/3 test environment that survives between CI runs; especially useful when startup time is a bottleneck for live agent tests

---

## Modal (Python Workloads)

### What It Is

[Modal](https://modal.com/) is a serverless compute platform for Python ML workloads. Pay-per-second, GPU available, functions run up to configurable hour limits.

### Relevance to This Project

Limited — the Mattermost server is Go, and Modal's SDK is Python-only. The pattern would require:
1. Python test orchestration calling the Mattermost REST API
2. Modal functions that invoke LLM evals against the running Mattermost server

Possible for Python-based eval workloads (e.g., running DeepEval or LangSmith evals as Modal functions), but introduces a Python layer not currently in the stack.

### Verdict

⚠️ **Low priority** — Python-only SDK adds friction for a Go-primary project. Consider if you adopt Python eval frameworks (DeepEval, LangSmith).

---

## AWS Step Functions + CodeBuild (Enterprise Scale)

### What It Is

The most principled architecture for complex multi-step agent test orchestration:

- **CodeBuild** — containerized CI builds, no local Docker required
- **Step Functions** — state machine orchestration with no timeout limit (runs up to 1 year)
- **Bedrock** — native Step Functions integration for LLM calls within state machines
- **IAM** — role-based secret management (no env var secrets)

### Pattern for Delegation Chain Testing

```
State Machine: AgentDelegationChainTest
  → State: StartMattermostServer (CodeBuild job)
  → State: WaitForHealthCheck (HTTP poller, retries)
  → State: SubmitAgentTask (HTTP call to /api/v4/agents/tasks)
  → State: PollForCompletion (loop with 5s wait)
  → State: EvaluateResult (Bedrock LLM-as-judge)
  → State: PostResultsToChannel (HTTP call to Mattermost webhook)
  → State: Teardown (CodeBuild job)
```

### Strengths
- No timeout limits
- Full audit trail of every state transition
- Parallel fan-out across multiple test scenarios simultaneously
- Native Bedrock integration for LLM-as-judge scoring
- Battle-tested for production workloads (not just CI)

### Limitations
- Significant AWS expertise required
- High setup complexity
- Tightly coupled to AWS ecosystem

### Verdict

✅ **Use for:** Teams already on AWS; production-grade long-running agent benchmark orchestration; when you need full auditability of test execution

---

## Temporal (Workflow Orchestration)

### What It Is

[Temporal](https://temporal.io/) is a durable workflow orchestration engine used by OpenAI, Replit, and others for production AI agents. Go SDK available.

### Why It's Relevant

Temporal's programming model maps naturally onto multi-agent delegation chain testing:
- Each **agent** becomes a Temporal **Activity** (with configurable retries, timeouts)
- Each **delegation** becomes a **child Workflow** invocation
- Test environment: Temporal's testing SDK replaces Activity implementations with deterministic stubs

```go
// In production: CEO agent Activity calls real OpenClaw
// In tests: replaced with stub returning cassette response
testEnv.RegisterActivity(agentruntime.CEOAgentActivity)
testEnv.OnActivity(agentruntime.CEOAgentActivity, mock.Anything).Return(
    cassettes.Load("ceo_delegation_response"), nil,
)
testEnv.ExecuteWorkflow(agentruntime.DelegationChainWorkflow, taskInput)
```

### Strengths
- **Time-skipping** in tests: test 30-day delegation chains without waiting
- Step-debug execution for complex workflows
- Complete event history for every execution
- Parent-child workflow model mirrors agent delegation hierarchy exactly

### Limitations
- Significant adoption investment (new infrastructure dependency)
- Requires refactoring `agentruntime` to use Temporal Activities + Workflows
- Self-hosted Temporal cluster or Temporal Cloud subscription

### Verdict

⚠️ **Long-term consideration** — the most principled architecture for delegation chain testing, but requires significant refactoring. Revisit if agent orchestration complexity grows substantially.

---

## Recommendation Matrix

| Use Case | Recommended Platform |
|----------|---------------------|
| Every PR smoke tests | GitHub Actions (free tier) |
| Gated live evals, <6h | GitHub Actions + Environment protection rules |
| Always-on test instance | Fly.io Sprites (~$5/month) |
| Long-running benchmarks (>6h) | GitHub Actions self-hosted runners or Buildkite |
| Enterprise scale, AWS teams | AWS Step Functions + CodeBuild |
| Complex delegation chain testing (future) | Temporal |
| Python eval workloads | Modal |

---

## Summary

For the immediate future, **GitHub Actions is sufficient** with proper environment protection rules for live tests. The main constraint is the 6-hour limit — for Tier 3 long-running benchmarks, the simplest solution is self-hosted GH Actions runners on a cheap EC2 instance or a persistent Fly.io machine that CI jobs SSH into.

The platform to invest in as the agent platform matures is likely **Buildkite** (for Go-native pipeline composition and self-hosted runners without limits) or **Temporal** (if the agent runtime is refactored to use durable workflows as a first-class primitive).
