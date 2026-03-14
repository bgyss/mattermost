# Buildkite — Deep Dive

Buildkite is a hybrid CI/CD platform: a fully managed SaaS control plane paired with self-hosted build agents running on your infrastructure. Founded in Melbourne (2013), bootstrapped profitably, ~$47M total funding, ~140 employees. Used by Shopify, Reddit, Airbnb, Block (Square), Spotify, Uber, Pinterest, Slack, and others.

This document covers architecture, pricing, the "Agentic CI" features, and how Buildkite would fit into the Mattermost agent testing strategy — specifically for long-running live LLM tests that exceed GitHub Actions' 6-hour limit.

---

## Core Architecture

### The Hybrid Model

Buildkite's key differentiator from GitHub Actions:

- **Control plane (SaaS):** Handles orchestration, scheduling, pipeline visualization, the web UI. Fully managed by Buildkite.
- **Build environment (your infra):** Agents run on your machines — EC2, bare metal, Kubernetes, your laptop. Source code and secrets never leave your environment.
- **Or hosted agents:** Buildkite also offers managed compute (Linux/macOS) if you don't want to manage infrastructure.

```
┌─────────────────────────────────────┐
│        Buildkite Control Plane      │
│  (pipeline scheduling, web UI,      │
│   status checks, artifact index)    │
└──────────────┬──────────────────────┘
               │ poll (outbound only)
               │
    ┌──────────┴──────────┐
    │   Your Infrastructure│
    │  ┌─────┐  ┌─────┐   │
    │  │Agent│  │Agent│   │
    │  │ #1  │  │ #2  │   │
    │  └─────┘  └─────┘   │
    │  (EC2, K8s, bare)    │
    └──────────────────────┘
```

Agents **poll outbound** to the Buildkite API — no inbound connections required. This makes it firewall-friendly and means Buildkite never sees your code.

### Pipeline Format

Pipelines are defined in `.buildkite/pipeline.yml`:

```yaml
steps:
  - command: "make build"
    key: "build"

  - wait   # fan-in: wait for build to complete

  - command: "make test-unit"
    parallelism: 10              # fan-out: 10 parallel instances
  - command: "make test-integration"
  - command: "make test-e2e"

  - wait   # fan-in: wait for all tests

  - command: "make deploy"
    branches: "main"

  - block: "Deploy to Production?"  # manual approval gate
    branches: "main"

  - command: "make deploy-prod"
```

Key difference from GitHub Actions: **all steps run in parallel by default** unless explicitly sequenced with `wait`, `depends_on`, or `block` steps. In GH Actions, steps within a job are sequential.

### Queue-Based Routing

Agents are assigned to queues via tags. Pipeline steps target specific queues:

```yaml
steps:
  - command: "go test -tags=live ./..."
    agents:
      queue: "llm-testing"       # route to agents with LLM access
      os: "linux"
    timeout_in_minutes: 480      # 8 hours — no limit on Pro/Enterprise
```

This maps naturally to agent testing: a `llm-testing` queue runs on agents that have `ANTHROPIC_API_KEY` and `OPENCLAW_GATEWAY_URL` configured, while a `default` queue runs smoke tests on cheap instances.

### Agent Hooks

Hooks intercept the build lifecycle at specific points. Defined in `/etc/buildkite-agent/hooks/` (global) or `.buildkite/hooks/` (per-repo):

| Hook | When | Use Case |
|------|------|----------|
| `environment` | Before checkout | Inject secrets from Vault/AWS Secrets Manager |
| `pre-command` | Before the step runs | Fetch LLM API keys, start postgres |
| `post-command` | After the step | Upload eval results, post to Mattermost |
| `pre-exit` | Before agent cleanup | Tear down test environment |

### Timeouts

- **Per-step:** `timeout_in_minutes` in pipeline YAML
- **Personal plan:** 4-hour hard limit
- **Pro and Enterprise plans:** No plan-level limit — **jobs can run 8+ hours**
- Jobs can extend their timeout by up to 60 minutes beyond the initial value at runtime
- Self-hosted agents have no inherent timeout — controlled entirely by pipeline config

This is the primary reason to consider Buildkite for Tier 3 agent benchmarks.

---

## Agentic CI (2025-2026)

Buildkite has invested heavily in AI-powered CI workflows. Their thesis: CI must become "composable, adaptive, and programmable" for AI-generated workloads.

### Components

| Component | What It Does |
|-----------|-------------|
| **MCP Server** | Open-source MCP server exposing pipeline, build, and job data to AI tools (Claude Desktop, Codex CLI, Gemini CLI) |
| **Model Providers** | Connect to frontier models (Claude, etc.) directly through Buildkite using your own or Buildkite-managed API keys |
| **Pipeline Triggers** | Inbound webhooks for GitHub and Linear events; AI agents can respond to external signals |
| **Pipeline SDK** | Define pipelines in Go, TypeScript, Python, Ruby — AI agents can generate these programmatically |
| **AI Plugins** | Claude/Codex/Bedrock-powered plugins for annotating CI jobs with rich build summaries |

### Practical Examples from Buildkite's Blog

**1. Self-Healing Builds:** Claude uses the MCP server to query build logs, find root causes, clone the repo, implement fixes, push a branch, create a PR, and wait for the fix build to pass before posting a summary.

**2. AI Code Review:** Pipeline step runs Claude Code to evaluate PRs and post GitHub review comments with structured feedback.

**3. Issue Analysis:** Linear issue creation triggers an agentic analysis pipeline that generates either a solution or implementation recommendations.

### Go SDK for Dynamic Pipelines

The Go SDK (`github.com/buildkite/pipeline-sdk/sdk/go`) enables generating pipeline steps programmatically:

```go
package main

import (
    bk "github.com/buildkite/pipeline-sdk/sdk/go"
)

func run() error {
    pipeline := bk.NewStepBuilder()

    // Always run smoke tests
    pipeline.AddCommand(&bk.Command{
        Commands: []string{"go test ./platform/services/agentruntime/... -count=1"},
        Label:    "Agent Smoke Tests",
    })

    // Conditionally add live tests if on main branch
    if bk.Environment.BUILDKITE_BRANCH() == "main" {
        pipeline.AddCommand(&bk.Command{
            Commands: []string{"go test -tags=live ./platform/services/agentruntime/... -v"},
            Label:    "Agent Live Tests",
            Agents:   map[string]string{"queue": "llm-testing"},
            TimeoutInMinutes: 120,
        })
    }

    return pipeline.Print()
}
```

This is powerful for agent testing: an AI agent could generate test pipeline steps based on which agent components changed in a PR.

!!! note "SDK Maturity"
    The Pipeline SDK is pre-release (v0.0.0, no tagged version). Types are auto-generated from Buildkite's Pipeline Schema. Expect API changes.

---

## Pricing

### Plan Comparison

| Plan | Cost | Concurrent Self-Hosted Agents | Hosted Minutes/Month |
|------|------|------------------------------|---------------------|
| **Personal** | Free | 3 jobs max | 500 |
| **Pro** | $30/user/month | 10 included, $2.50/additional | 2,000 |
| **Enterprise** | Custom | Unlimited | Custom |

### Hosted Agent Compute Rates

| Instance | vCPU | RAM | $/minute |
|----------|------|-----|----------|
| Linux Small | 2 | 4 GB | $0.013 |
| Linux Medium | 4 | 16 GB | $0.026 |
| Linux Large | 8 | 32 GB | $0.052 |
| Mac M4 Medium | 6 | 28 GB | $0.18 |
| Mac M4 Large | 12 | 56 GB | $0.36 |

Billed per-second. No rounding, no minimum charge. Charges apply only to command execution time.

### Cost Scenarios for Agent Testing

| Scenario | Platform | Monthly Cost |
|----------|----------|-------------|
| Smoke tests only (GH Actions) | GitHub Actions (free tier) | $0 |
| Smoke + gated live evals | GitHub Actions (Team plan) | ~$4/user/month |
| **Buildkite Pro (5 engineers)** | Buildkite | **$150/month** + infra |
| Self-hosted agent on EC2 spot (c5.xlarge) | AWS | ~$50/month (spot) |
| **Total: Buildkite + 1 self-hosted agent** | Combined | **~$200/month** |

### vs. GitHub Actions

For the specific use case of "run live agent tests for 2-8 hours":

- **GitHub Actions:** Free for <6h jobs; self-hosted runners have no timeout but you still pay for GH Teams ($4/user/month). No per-minute agent fee.
- **Buildkite Pro:** $30/user/month + infrastructure. No timeout limit on Pro/Enterprise. More expensive in license fees, potentially cheaper in compute (spot instances + queue-based routing).

**Bottom line:** If your team is <20 people and live tests complete under 6 hours, **GitHub Actions self-hosted runners are cheaper**. Buildkite becomes cost-competitive at ~75+ engineers or when you need its advanced features (dynamic pipelines, queue routing, agentic CI).

---

## Self-Hosted Agent Setup

### Installation

```bash
# Ubuntu/Debian
sh -c 'echo deb https://apt.buildkite.com/buildkite-agent stable main > \
  /etc/apt/sources.list.d/buildkite-agent.list'
apt-get update && apt-get install -y buildkite-agent

# Configure
cat > /etc/buildkite-agent/buildkite-agent.cfg <<EOF
token="YOUR_AGENT_TOKEN"
name="llm-test-agent-%spawn"
tags="queue=llm-testing,os=linux"
spawn=2
EOF

# Start
systemctl enable buildkite-agent && systemctl start buildkite-agent
```

### Multiple Agents Per Machine

Use the `spawn` config option:
```
spawn=5   # One process, 5 concurrent job slots
```

### Auto-Scaling on AWS

Buildkite provides a one-click **Elastic CI Stack for AWS** (CloudFormation template):

- AgentScaler Lambda runs every minute
- Adjusts ASG capacity based on Buildkite job queue depth (not CPU usage — scales based on demand)
- Supports EC2 Spot Instances (up to 90% savings)
- Scales to zero when no jobs are queued
- Grace period for in-flight jobs before termination (default 3,600s)

```bash
# Deploy the stack
aws cloudformation create-stack \
  --stack-name buildkite-agents \
  --template-url https://s3.amazonaws.com/buildkite-aws-stack/latest/aws-stack.yml \
  --parameters \
    ParameterKey=BuildkiteAgentToken,ParameterValue=YOUR_TOKEN \
    ParameterKey=BuildkiteQueue,ParameterValue=llm-testing \
    ParameterKey=InstanceType,ParameterValue=c5.xlarge \
    ParameterKey=SpotPrice,ParameterValue=0.08 \
    ParameterKey=MinSize,ParameterValue=0 \
    ParameterKey=MaxSize,ParameterValue=5
```

Kubernetes auto-scaling is also supported via `agent-stack-k8s` with a Buildscaler controller.

---

## GitHub Integration

Buildkite integrates with GitHub via the GitHub App (recommended) or OAuth:

- Reports build status directly on PRs via GitHub Checks API
- Supports GitHub required status checks for branch protection
- Per-step annotations visible in GitHub PR UI
- Can coexist with GitHub Actions — both report independent status checks

### Migration from GitHub Actions

Buildkite provides an [interactive migration tool](https://buildkite.com/docs/pipelines/migration/tool/github-actions) that converts GH Actions YAML to Buildkite pipeline YAML. Also available as a CLI tool (`buildkite/migration` on GitHub, alpha).

### Coexistence Pattern

During migration, you can use a GitHub Action to trigger Buildkite pipelines:

```yaml
# .github/workflows/trigger-buildkite.yml
- uses: buildkite/trigger-pipeline-action@v2
  with:
    buildkite_api_access_token: ${{ secrets.BUILDKITE_TOKEN }}
    pipeline: "my-org/agent-live-tests"
    branch: ${{ github.head_ref }}
```

This lets you keep smoke tests on GitHub Actions while routing live tests to Buildkite.

---

## Implementation: Agent Testing on Buildkite

### Pipeline for Mattermost Agent Tests

```yaml
# .buildkite/pipeline.agent-tests.yml

steps:
  # ── Tier 1: Smoke tests (default queue, any agent) ──────────────
  - label: ":test_tube: Agent Smoke Tests"
    command: |
      mise run ci:up
      go test ./platform/services/agentruntime/... -count=1
      mise run ci:down
    agents:
      queue: "default"
    timeout_in_minutes: 30

  - wait

  # ── Tier 2: Live tests (LLM queue, agents with API keys) ────────
  - label: ":brain: Live Agent Eval (Anthropic)"
    command: |
      mise run ci:up
      go test -tags=live ./platform/services/agentruntime/... -v
      npx promptfoo eval --config evals/agent-behaviors.yaml
      mise run ci:down
    agents:
      queue: "llm-testing"
    timeout_in_minutes: 120
    soft_fail:
      - exit_status: 1     # non-deterministic tests — soft fail, don't block

  - wait: ~
    continue_on_failure: true

  # ── Post results ────────────────────────────────────────────────
  - label: ":mega: Post Results to Mattermost"
    command: "bash scripts/post-eval-results.sh"
    agents:
      queue: "default"

  # ── Tier 3: Long-running benchmarks (manual gate) ──────────────
  - block: ":lock: Run delegation chain benchmark?"
    branches: "main"

  - label: ":chains: CEO→Sales→Specialist Delegation Benchmark"
    command: |
      mise run ci:up
      go test -tags=benchmark ./platform/services/agentruntime/... -v -timeout 6h
      mise run ci:down
    agents:
      queue: "llm-testing"
    timeout_in_minutes: 480    # 8 hours — only possible on Pro/Enterprise
```

### Agent Configuration for LLM Testing Queue

On the self-hosted agent machine:

```bash
# /etc/buildkite-agent/hooks/environment
export ANTHROPIC_API_KEY="sk-ant-..."
export MM_AGENTS_OPENCLAW_TOKEN="..."
export OPENCLAW_GATEWAY_URL="http://127.0.0.1:18789"
export MM_ADMIN_PASSWORD="Admin1234!"
```

These secrets never leave your machine — Buildkite's control plane never sees them.

---

## Customer Case Studies

| Company | Scale | Key Result |
|---------|-------|------------|
| **Shopify** | Thousands of engineers | <5 min builds; grew engineering 300% without CI bottlenecks |
| **Reddit** | Migrated from 6,000-line YAML | 30% faster Android/iOS builds; queue times from minutes to ~5 seconds |
| **Intercom** | 150 daily deployments | Test time reduced from 25 min to 3 min |
| **PagerDuty** | Enterprise scale | 20% faster incident resolution through faster deploys |
| **REA Group** | Large team | Setup time reduced from weeks to days |

Other notable users: Airbnb, Block (Square), Canva, Pinterest, Spotify, Uber, Slack, Tinder, Venmo, Wayfair, Cruise, Peloton, Retool, Hasura.

---

## Limitations

### Pricing Concerns

Community feedback notes pricing has become "kind of wild" — a 60-person startup reported $1,800/month in license fees alone ($30/user). A Buildkite founding engineer publicly acknowledged this and announced plans for more accessible tiers.

For a small team (5-10 engineers), GitHub Actions self-hosted runners are significantly cheaper for the same capability.

### Compared to GitHub Actions

| Missing in Buildkite | Available in GH Actions |
|---------------------|------------------------|
| No native matrix strategy | `strategy.matrix` |
| No built-in dependency caching | `actions/cache` |
| No generous free compute for OSS | Unlimited minutes for public repos |
| Requires separate GitHub integration setup | Built-in |
| Smaller plugin ecosystem | Large marketplace |

### Operational Overhead

- Self-hosted agents require infrastructure management (patching, monitoring, scaling)
- AWS Elastic CI Stack mitigates this but adds AWS complexity
- Queue configuration and agent tag management adds cognitive overhead
- If the Buildkite SaaS goes down, no new builds can be scheduled (running builds complete)

---

## Verdict for This Project

### When to Adopt Buildkite

Buildkite is worth adopting when **any** of these are true:

1. Live agent tests regularly exceed GitHub Actions' 6-hour limit
2. The team grows past ~20 engineers and CI queue times become a bottleneck
3. You need GPU agents for self-hosted LLM inference testing
4. You want AI-powered dynamic pipelines (Buildkite SDK + MCP server)

### When to Stay on GitHub Actions

Stay on GitHub Actions if:

1. Live tests complete under 6 hours (use GH Actions self-hosted runners for no timeout)
2. Team is <20 engineers
3. You don't need queue-based routing or dynamic pipeline generation
4. Budget is a primary concern ($0 vs. $30/user/month)

### Recommended Adoption Path

1. **Now:** Keep smoke tests on GitHub Actions. Use GH Actions `workflow_dispatch` + environment protection for gated live tests.
2. **If live tests hit 6h limit:** Add a single self-hosted GH Actions runner on a cheap EC2 instance. This is free (no per-minute platform fee for self-hosted).
3. **If team grows past 20+ or needs dynamic pipelines:** Evaluate Buildkite Pro. Use the GH Actions → Buildkite trigger action during migration so both systems coexist.
4. **Long-term:** Buildkite's Go SDK + MCP server + agentic CI features make it the natural choice for a Go-based agent platform at scale.

---

## References

| Resource | URL |
|----------|-----|
| Buildkite Homepage | https://buildkite.com/ |
| Pricing | https://buildkite.com/pricing/ |
| Pipeline Architecture | https://buildkite.com/docs/pipelines/architecture |
| Agent Documentation | https://buildkite.com/docs/agent |
| Agent Hooks | https://buildkite.com/docs/agent/hooks |
| Build Timeouts | https://buildkite.com/docs/pipelines/configure/build-timeouts |
| Elastic CI Stack for AWS | https://github.com/buildkite/elastic-ci-stack-for-aws |
| Pipeline SDK (Go) | https://pkg.go.dev/github.com/buildkite/pipeline-sdk |
| MCP Server | https://github.com/buildkite/buildkite-mcp-server |
| Agentic CI Blog Post | https://buildkite.com/resources/blog/building-ai-powered-ci-workflows-three-practical-examples/ |
| What AI Teaches Us About CI | https://buildkite.com/resources/blog/what-ai-is-teaching-us-about-ci/ |
| Migration from GH Actions | https://buildkite.com/docs/pipelines/migration/from-githubactions |
| vs. GitHub Actions | https://buildkite.com/compare/github-actions |
| Reddit Case Study | https://buildkite.com/resources/case-studies/reddit/ |
| Shopify Case Study | https://buildkite.com/resources/case-studies/ |
| GitHub Integration | https://buildkite.com/docs/pipelines/source-control/github |
| Dynamic Pipelines | https://buildkite.com/docs/pipelines/configure/dynamic-pipelines |
