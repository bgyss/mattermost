# Temporal — Deep Dive

[Temporal](https://temporal.io/) is a **durable workflow orchestration engine** that records every step of a workflow as an event, enabling automatic recovery from failures without re-executing completed work. Originally forked from Uber's Cadence project, it has become the dominant open-source workflow orchestration platform, with 183,000+ weekly active developers and 2,500+ cloud customers.

This document covers architecture, Go SDK usage, pricing, testing capabilities, and an honest assessment of fit for orchestrating Mattermost's multi-agent delegation chains.

---

## Architecture

### Server Components

Temporal's server comprises **four independently scalable gRPC services**, discovering each other through Ringpop (hash ring membership protocol):

| Service | Port | Role |
|---------|------|------|
| **Frontend** | 7233 | Stateless API gateway — rate limiting, auth, validation, routing |
| **History** | 7234 | Persists workflow state, manages event history, runs transfer/timer queues |
| **Matching** | 7235 | Hosts task queues, matches Workers to Tasks |
| **Worker** | 6939 | Internal system workflows, replication |

**History Shards** are the fundamental scaling unit. Each shard maps to a single persistence partition with sequential operation guarantees. The shard count is **immutable after initial configuration** — must be set correctly at deployment.

Supported persistence backends: **PostgreSQL**, **MySQL**, or **Apache Cassandra** (for greater scale).

### Event Sourcing & Deterministic Replay

Temporal uses durable event sourcing with idempotent execution:

1. The Temporal Service tracks progress by appending **Events** to an Event History
2. Workflows generate **Commands** (start timer, schedule activity, etc.) which become Events
3. On failure recovery, the Worker requests the Event History and **replays** it — running the workflow code again to produce Commands that are compared against existing Events
4. If Commands match, execution resumes from where it left off
5. If a generated Command doesn't match, a **non-deterministic error** is raised

### Workflows vs Activities

- **Workflows**: Deterministic orchestration code. Must produce the same Commands in the same sequence given the same input. Cannot use `time.Now()`, random numbers, or direct HTTP calls.
- **Activities**: Non-deterministic work (API calls, database queries, LLM inference). Automatically retried on failure. Results are recorded in Event History so they aren't re-executed on replay.

This separation is what makes Temporal powerful for agent orchestration — the LLM calls (non-deterministic) run as Activities, while the orchestration logic (which agent to call next, how to aggregate results) runs as a deterministic Workflow.

### 2025–2026 Platform Updates

- **Temporal Nexus** (GA): Connect Temporal Applications across isolated Namespaces
- **Worker Versioning APIs** (Pre-release): Pin Workflows to specific code versions, "rainbow deployments"
- **Activity Operations** (Public Preview): Pause, unpause, reset, update live Activities without redeployment
- **Multi-region Replication** (GA): 99.99% SLA with automatic failover
- **Google Cloud Availability** (GA)

---

## Go SDK

### Defining Workflows

```go
type DelegationInput struct {
    AgentID   string
    TaskText  string
    TeamID    string
}

type DelegationResult struct {
    Response   string
    TokensUsed int
    SubTasks   []string
}

func CEODelegationWorkflow(ctx workflow.Context, input DelegationInput) (*DelegationResult, error) {
    activityOptions := workflow.ActivityOptions{
        StartToCloseTimeout: 120 * time.Second,
        HeartbeatTimeout:    30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{
            InitialInterval:    2 * time.Second,
            BackoffCoefficient: 2.0,
            MaximumInterval:    5 * time.Minute,
            MaximumAttempts:    5,
        },
    }
    ctx = workflow.WithActivityOptions(ctx, activityOptions)

    // Step 1: LLM decides delegation plan
    var plan DelegationPlan
    err := workflow.ExecuteActivity(ctx, LLMPlanActivity, input).Get(ctx, &plan)
    if err != nil {
        return nil, err
    }

    // Step 2: Fan out to child workflows for each delegate
    var futures []workflow.ChildWorkflowFuture
    for _, delegation := range plan.Delegations {
        childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
            ParentClosePolicy: enums.PARENT_CLOSE_POLICY_TERMINATE,
        })
        f := workflow.ExecuteChildWorkflow(childCtx, AgentTaskWorkflow, delegation)
        futures = append(futures, f)
    }

    // Step 3: Collect results
    var results []string
    for _, f := range futures {
        var result DelegationResult
        if err := f.Get(ctx, &result); err != nil {
            results = append(results, fmt.Sprintf("error: %v", err))
            continue
        }
        results = append(results, result.Response)
    }

    // Step 4: LLM synthesizes final response
    var finalResult DelegationResult
    err = workflow.ExecuteActivity(ctx, LLMSynthesizeActivity, results).Get(ctx, &finalResult)
    return &finalResult, err
}
```

First parameter must be `workflow.Context`. Best practice: pass a single struct parameter. Struct methods for workflows are strongly discouraged — use struct methods only for Activities.

### Defining Activities

```go
type LLMActivities struct {
    OpenClawClient *openclaw.Client
    Store          store.AgentStore
}

func (a *LLMActivities) LLMPlanActivity(ctx context.Context, input DelegationInput) (*DelegationPlan, error) {
    // Non-deterministic: calls OpenClaw LLM gateway
    resp, err := a.OpenClawClient.ChatCompletion(ctx, openclaw.Request{
        Messages: []openclaw.Message{{Role: "user", Content: input.TaskText}},
        AgentID:  input.AgentID,
    })
    if err != nil {
        return nil, err
    }

    // Heartbeat during streaming (for long-running LLM calls)
    activity.RecordHeartbeat(ctx, "planning")

    return parsePlan(resp), nil
}
```

When you `RegisterActivity()` for a struct, the Worker gets access to all exported methods.

### Worker Setup

```go
temporalClient, _ := client.Dial(client.Options{
    HostPort: "localhost:7233",
})

w := worker.New(temporalClient, "agent-tasks", worker.Options{})
w.RegisterWorkflow(CEODelegationWorkflow)
w.RegisterWorkflow(AgentTaskWorkflow)
w.RegisterActivity(&LLMActivities{
    OpenClawClient: openclawClient,
    Store:          agentStore,
})
w.Run(worker.InterruptCh())
```

### Child Workflows (Delegation)

```go
childWorkflowOptions := workflow.ChildWorkflowOptions{
    // ABANDON: child continues if parent fails/cancels
    // TERMINATE: child killed when parent ends (default)
    // REQUEST_CANCEL: child receives cancellation signal
    ParentClosePolicy: enums.PARENT_CLOSE_POLICY_TERMINATE,
}
ctx = workflow.WithChildOptions(ctx, childWorkflowOptions)
err := workflow.ExecuteChildWorkflow(ctx, ChildWorkflowDef, params).Get(ctx, &result)
```

### Signals, Queries, and Updates

```go
// Signals — async, modify workflow state (e.g., cancel delegation)
signalChan := workflow.GetSignalChannel(ctx, "cancel-delegation")
signalChan.Receive(ctx, &cancelData)

// Queries — sync, read-only (e.g., get agent status for Command Center)
workflow.SetQueryHandler(ctx, "getStatus", func() (string, error) {
    return currentStatus, nil
})

// Updates — sync, modify state AND return result
workflow.SetUpdateHandlerWithOptions(ctx, "updatePriority", handler,
    workflow.UpdateHandlerOptions{Validator: validatorFunc})
```

### Continue-As-New

Prevents unbounded event history growth for long-running agent loops:

```go
if eventCount > 10000 {
    return workflow.NewContinueAsNewError(ctx, AgentLoopWorkflow, updatedState)
}
```

---

## Pricing

### Temporal Cloud Plans

| Plan | Monthly Base | Included Actions | Active Storage | P0 Response |
|------|-------------|-----------------|---------------|-------------|
| **Essentials** | Greater of $100 or 5% usage | 1M | 1 GB | 1 business day |
| **Business** | Greater of $500 or 10% usage | 2.5M | 2.5 GB | 2 business hours |
| **Enterprise** | Annual contract | 10M | 10 GB | 24/7, 30 min |
| **Mission Critical** | Annual contract | 10M | 10 GB | Dedicated |

### Pay-As-You-Go Actions Pricing

| Tier (actions/month) | Price per million |
|---------------------|-------------------|
| First 5M | $50 |
| 5M–10M | $45 |
| 10M–20M | $40 |
| 20M–50M | $35 |
| 50M–100M | $30 |
| 100M–200M | $25 |
| 200M+ | Contact Sales |

### What Counts as an Action

Billed: Workflow starts, timer starts, signals, queries, activity starts/retries, heartbeats reaching server, child workflow starts (2 actions each), schedule executions (3 actions each).

Not billed: Failed/deduplicated workflow starts, search attributes during workflow start, deduplicated updates.

### Storage Pricing

- **Active**: $0.042/GBh (~$30/GB-month)
- **Retained**: $0.00105/GBh (~$0.76/GB-month)

### Real-World Cost Estimates

| Scenario | Monthly Cost | Actions/Month |
|----------|-------------|---------------|
| Dev/testing | <$25 | ≤1M |
| CI agent test orchestration | ~$100–200 | ~2–4M |
| Production agent platform | ~$500–1,000 | ~10–20M |
| High-scale SaaS platform | $10,000+ | 200M+ |

### Self-Hosted Costs

The open-source server is free (MIT license). Infrastructure costs:

- 5+ services to run (Frontend, History, Matching, Worker, Web UI)
- Database (PostgreSQL/MySQL/Cassandra) + optional Elasticsearch
- One real-world example: **~$1,500/month** for EKS nodes + Aurora PostgreSQL supporting 200M+ workflows/month

Requires dedicated operations team for 24/7 on-call, security patches, and disaster recovery.

### Incentives

- **$1,000 free credits** for new users
- **$6,000 free credits** for startups under $30M funding
- Commitment pricing: prepay for 1–3 year terms with volume/duration discounts

---

## Testing Capabilities

### Time-Skipping Test Framework

The Go SDK includes an **in-memory Temporal Server implementation** that automatically skips time. A workflow that would take days in production completes in milliseconds in tests.

```go
type AgentWorkflowTestSuite struct {
    suite.Suite
    testsuite.WorkflowTestSuite
    env *testsuite.TestWorkflowEnvironment
}

func (s *AgentWorkflowTestSuite) SetupTest() {
    s.env = s.NewTestWorkflowEnvironment()
}

func (s *AgentWorkflowTestSuite) AfterTest(suiteName, testName string) {
    s.env.AssertExpectations(s.T())
}
```

`workflow.Sleep()` is automatically skipped. `time.Sleep()` is NOT (and should never be used in workflows).

### Mocking LLM Activities

```go
// Fixed response (cassette pattern)
s.env.OnActivity(LLMPlanActivity, mock.Anything, mock.Anything).Return(
    &DelegationPlan{Delegations: []Delegation{{AgentID: "sales-head", Task: "Q3 summary"}}},
    nil,
)

// Error simulation (rate limiting)
s.env.OnActivity(LLMPlanActivity, mock.Anything, mock.Anything).Return(
    nil, errors.New("429: rate limited"),
).Once()

// Custom logic (conditional responses)
s.env.OnActivity(LLMPlanActivity, mock.Anything, mock.Anything).Return(
    func(ctx context.Context, input DelegationInput) (*DelegationPlan, error) {
        if strings.Contains(input.TaskText, "sales") {
            return &DelegationPlan{Delegations: salesDelegations}, nil
        }
        return &DelegationPlan{Delegations: defaultDelegations}, nil
    },
)
```

### Mid-Workflow Assertions

```go
s.env.RegisterDelayedCallback(func() {
    res, err := s.env.QueryWorkflow("getStatus")
    var status string
    res.Get(&status)
    s.Equal("delegating", status)
}, 10*time.Second+time.Millisecond)
```

### Replay Testing for Determinism

```go
replayer := worker.NewWorkflowReplayer()
replayer.RegisterWorkflow(CEODelegationWorkflow)
err := replayer.ReplayWorkflowHistory(nil, historicalEventHistory)
// If err != nil, a code change broke determinism
```

Capture event histories from production and replay against new code before deploying — catches incompatible changes in CI.

### Activity Testing in Isolation

```go
env := s.NewTestActivityEnvironment()
env.RegisterActivity(&LLMActivities{OpenClawClient: mockClient})
result, err := env.ExecuteActivity(LLMPlanActivity, testInput)
```

---

## Implementation: Mattermost Agent Delegation as Temporal Workflows

### Architecture Mapping

```
Current Architecture               →  Temporal Architecture
─────────────────────              →  ─────────────────────
AgentRuntimeService.SubmitTask()   →  temporalClient.ExecuteWorkflow()
executor.go agentic loop           →  AgentTaskWorkflow (deterministic)
LLM API calls                      →  LLMCallActivity (non-deterministic)
delegate_task tool                 →  workflow.ExecuteChildWorkflow()
In-memory dispatcher queue         →  Temporal Task Queue
Atomic observability counters      →  Temporal Metrics + Event History
AgentTask DB polling               →  Temporal Visibility API / Queries
```

### CEO → Sales Head → Specialists

```
CEODelegationWorkflow (Parent)
├── ExecuteActivity(LLMPlanActivity)           // CEO decides to delegate
├── ExecuteChildWorkflow(SalesHeadWorkflow)     // Sales Head child workflow
│   ├── ExecuteActivity(LLMAnalyzeActivity)    // Sales Head plans
│   ├── ExecuteChildWorkflow(ProspectingSpec)   // Specialist grandchild
│   │   └── Activity loop: LLM calls + tool execution
│   └── ExecuteChildWorkflow(ProposalWriter)    // Specialist grandchild
│       └── Activity loop: LLM calls + tool execution
└── ExecuteActivity(LLMSynthesizeActivity)      // CEO aggregates results
```

### Inter-Agent Communication

| Pattern | Use Case |
|---------|----------|
| **Child Workflows** | Delegation (CEO → Sales Head → Specialists) |
| **Signals** | Async notifications between agents (e.g., "task priority changed") |
| **Queries** | Command Center reads agent status without side effects |
| **Updates** | Sync state modifications (e.g., "update system prompt mid-run") |

### Heartbeating for Long-Running LLM Calls

```go
func LLMStreamActivity(ctx context.Context, prompt string) (string, error) {
    // Resume from checkpoint on retry
    if activity.HasHeartbeatDetails(ctx) {
        var partial string
        activity.GetHeartbeatDetails(ctx, &partial)
        // resume from partial result
    }

    var result strings.Builder
    for chunk := range streamLLMResponse(ctx, prompt) {
        result.WriteString(chunk)
        activity.RecordHeartbeat(ctx, result.String())
    }
    return result.String(), nil
}
```

### Observability Integration

- Temporal UI provides chronological event history for every agent interaction
- Every decision, tool call, and delegation is recorded
- Signal delivery timestamps, Activity results, retry counts, error traces all visible
- Integrations available: Langfuse (tracing), Braintrust, Datadog

---

## Customer Case Studies

### Scale

- **2,500+ customers** globally
- **183,000+ weekly active** open-source developers
- **7 million+ unique Temporal clusters** deployed
- Peak throughput: **300K executions/second**
- Revenue grew **4.4x in 18 months**; Net Dollar Retention **184%**
- **$5B valuation** (Series D, February 2026); total raised ~$650M

### Notable Production Users

**Netflix** — Foundation for next-gen CI/CD system used by nearly every Netflix developer. Migrated from complex homegrown orchestration. Deployment failure rate dropped to **0.0001%**. Adoption doubling year-over-year.

> "It's a system that just works. Even when it fails, things will just pick up right back where they started."

**Snap** — Every Snap story uses Temporal. Cross-organization orchestration for services and infrastructure deployments.

**Coinbase** — Every Coinbase transaction uses Temporal.

**Stripe** — Millions of Temporal workflows daily.

**Datadog** — 100+ internal Temporal users for database reliability and developer experience.

**Twilio** — Every message on Twilio uses Temporal.

**Replit** — Powers Replit Agent at massive scale.

> "Temporal gives us a lot more confidence to build the product."

**Retool** — Thousands of agent runs per day. Shipped Agents product in months with 10 engineers.

> "It just wouldn't have been possible without Temporal."

**Lindy** — 2.5M Temporal actions daily for AI agent orchestration.

**Vinted** — 10–12 million workflows/activities daily for payment flow orchestration.

**VEED.IO** — 4 million daily Temporal activities for video workflow orchestration.

**Attentive** — Saved $30,000/month migrating from self-hosted to Cloud.

**Other notable users**: OpenAI, Cloudflare, GitLab, DoorDash, Duolingo, HashiCorp, Alaska Airlines, GoDaddy, Qualtrics, Vodafone, Deloitte, Checkr, Instacart, Box, Comcast.

---

## Limitations and Risks

### Hard Limits (Temporal Cloud)

| Limit | Value |
|-------|-------|
| Event History | **51,200 events or 50 MB** (warns at 10,240 / 10 MB) |
| Payload size per request | **2 MB** |
| gRPC message size | **4 MB** |
| Signals per workflow | **10,000** |
| Concurrent operations per workflow | 2,000 (recommended: 500) |
| Retention period | 1–90 days (default 30) |
| Namespaces per account | 10 default (auto-increases to 100) |

### Determinism Constraints

Operations that must maintain strict sequencing in workflow code:

- Starting/cancelling Timers, Activities, Child Workflows
- Signalling external Workflows
- Ending Workflow Executions
- SideEffect/MutableSideEffect calls

**Forbidden in workflow code**: `time.Now()`, `rand.*`, direct HTTP calls, goroutines outside `workflow.Go()`, any operation that varies between executions.

**Code change risk**: Modifying workflow logic for in-flight workflows causes non-deterministic errors. Must use Worker Versioning or Patching APIs for safe rollouts.

### Operational Complexity (Self-Hosted)

- 5+ services to run and scale independently
- Database administration (PostgreSQL/MySQL/Cassandra) + optional Elasticsearch
- History Shard count immutable after deployment — must estimate correctly upfront
- Requires dedicated 24/7 on-call team
- Security patches, upgrades, disaster recovery planning

### Learning Curve

- Determinism requirement is counterintuitive — developers must unlearn habits like calling `time.Now()`
- Debugging deeply nested parent-child workflow hierarchies is difficult
- Go and Java SDKs have best maturity; Python/TypeScript are newer
- "Heavyweight code-only DSL" — steep maintainability curve for simpler use cases

### Vendor Lock-In

- Open-source (MIT license) — self-hosting avoids cloud lock-in
- However, workflows are deeply coupled to Temporal SDK APIs
- Migrating away requires rewriting all workflow/activity code
- No standard "portable workflow format" exists

---

## Comparison to Alternatives

| Dimension | Temporal | AWS Step Functions | Plain Goroutines + DB |
|-----------|---------|-------------------|----------------------|
| **Definition** | Code-first (Go, Java, Python, TS) | JSON/YAML (ASL) | Application code |
| **Execution** | Self-managed workers | Fully managed AWS | Your infrastructure |
| **Max duration** | Unlimited (Continue-As-New) | 1 year (Standard), 5 min (Express) | Unlimited |
| **State recovery** | Automatic (event replay) | Automatic | Manual checkpointing |
| **Cost at scale** | ~$1,500/mo self-hosted for 200M+ workflows | Grows linearly with transitions | Infrastructure only |
| **Timer precision** | Sub-second | ~1 minute minimum | Application-dependent |
| **Learning curve** | High (determinism, SDK concepts) | Medium (AWS knowledge) | Low (familiar patterns) |
| **Debugging** | Event history + Temporal UI | CloudWatch + visual timeline | Custom logging |
| **Cloud lock-in** | Cloud-agnostic | AWS-only | None |
| **Saga/compensation** | Built-in | Manual | Manual |
| **Parent-child** | Native child workflows | Nested state machines | Application code |

---

## Verdict for This Project

Temporal is the **most architecturally principled** platform for multi-agent delegation chain orchestration, but requires significant investment.

**Pros:**

- Parent-child workflow model maps perfectly onto CEO → Sales Head → Specialist delegation hierarchy
- Time-skipping test framework enables testing 30-day delegation chains in milliseconds
- Every LLM call, tool invocation, and delegation is durably recorded — complete audit trail
- Automatic retry with heartbeating for flaky LLM API calls
- Go SDK is first-class; `$1,000` free credits for evaluation
- Signals and Queries map naturally onto the Agent Command Center's real-time status updates
- Production-proven for AI agent orchestration (Replit, Retool, Lindy all use it for agents)

**Cons:**

- Significant refactoring required — `agentruntime` must be rewritten as Temporal Activities + Workflows
- Determinism constraints add cognitive overhead for all contributors
- Self-hosted: 5+ services + database + operational burden; Cloud: $100+/month minimum
- History event limit (51,200) means long-running agent loops need Continue-As-New
- No portable workflow format — deep SDK coupling makes future migration expensive

**Recommendation:** Temporal is a **long-term investment** worth pursuing when agent orchestration complexity grows substantially — specifically when delegation chains become deep (3+ levels), long-running (hours/days), or when the audit trail becomes a product requirement. For the current state of the platform, the existing in-memory dispatcher + PostgreSQL approach is sufficient and far simpler.

**Evaluation path:**

1. Start with `$1,000` free Temporal Cloud credits
2. Prototype a single `AgentTaskWorkflow` that wraps the existing `executor.go` agentic loop
3. Validate that the time-skipping test framework delivers the promised CI speedup
4. If successful, incrementally migrate delegation chains to child workflows

---

## References

| Resource | URL |
|----------|-----|
| Temporal Documentation | https://docs.temporal.io/ |
| Go SDK Guide | https://docs.temporal.io/develop/go |
| Go SDK Testing Suite | https://docs.temporal.io/develop/go/testing-suite |
| Cloud Pricing | https://temporal.io/pricing |
| Cloud Pricing Update Blog | https://temporal.io/blog/temporal-cloud-pricing-update |
| Actions Reference | https://docs.temporal.io/cloud/actions |
| System Limits | https://docs.temporal.io/cloud/limits |
| Building AI Agents with Temporal | https://temporal.io/blog/of-course-you-can-build-dynamic-ai-agents-with-temporal |
| Multi-Agent Architectures | https://temporal.io/blog/using-multi-agent-architectures-with-temporal |
| OpenAI Agents SDK Integration | https://temporal.io/blog/announcing-openai-agents-sdk-integration |
| Orchestrating Ambient Agents | https://temporal.io/blog/orchestrating-ambient-agents-with-temporal |
| AI Solutions Page | https://temporal.io/solutions/ai |
| Replay 2025 Announcements | https://temporal.io/blog/replay-2025-product-announcements |
| Customer Case Studies | https://temporal.io/in-use |
| Netflix Case Study | https://temporal.io/resources/case-studies/netflix-increases-developer-productivity |
| Self-Hosted Deployment | https://docs.temporal.io/self-hosted-guide/deployment |
| GitHub Repository | https://github.com/temporalio/temporal |
| Series D Announcement | https://www.geekwire.com/2026/temporal-raises-300m-hits-5b-valuation/ |
