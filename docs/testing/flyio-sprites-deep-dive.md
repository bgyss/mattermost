# Fly.io Sprites — Deep Dive

Fly.io Sprites are **persistent, hardware-isolated Linux virtual machines** built on Firecracker microVMs. Launched in January 2026, they are designed for AI coding agents, sandboxed code execution, and bursty development workloads that benefit from scale-to-zero billing.

This document covers architecture, pricing, implementation details, and an honest assessment of current limitations — specifically in the context of using Sprites as an always-on Mattermost test environment for agent CI.

---

## Architecture

### VM Technology

Sprites run on **Firecracker microVMs** (the same technology behind AWS Lambda). Each Sprite is a full Linux VM, not a container — providing hardware-level isolation with a dedicated kernel.

Key architectural detail: Fly.io uses "inside-out orchestration" — most management code runs in the VM's root namespace, and user code runs in an inner container. VMs can bounce without full reboots.

### How Sprites Differ from Fly.io Machines

| Feature | Fly.io Machines | Fly.io Sprites |
|---------|----------------|----------------|
| Image format | Docker/OCI images | Bare Linux (no Dockerfile) |
| Root filesystem | Ephemeral (use Volumes for persistence) | Persistent 100GB ext4 |
| State model | Stateless by default | Stateful by default |
| Deployment | `fly deploy` with Dockerfile | `sprite exec` with shell commands |
| Checkpoint/restore | No | Yes (~300ms checkpoint, <1s restore) |
| Auto-idle | Yes (scale-to-zero) | Yes (auto-sleep after ~30s inactivity) |
| GPU support | Yes (A10, L40S, A100) | No |

### Storage Architecture

Sprites use a **tiered storage system** inspired by JuiceFS:

- **Hot tier:** Fast local NVMe cache on the host machine
- **Cold tier:** Durable S3-compatible object storage backend
- **Metadata:** SQLite, made durable via Litestream replication
- **Copy-on-write:** Checkpoints capture only changed blocks (~300ms)
- **TRIM-friendly:** Deleting files actually reduces storage costs

The root partition starts at 100GB (auto-scalable). You are billed only for blocks actually written, not the full allocation.

!!! warning "Not suitable for hot PostgreSQL"
    Fly.io explicitly acknowledges that object storage performance is insufficient for production PostgreSQL workloads. For a CI test database with small datasets, this is likely acceptable.

### Networking

- Each Sprite gets a **unique public HTTPS URL** proxied through Fly's anycast network
- Default exposed port: **8080**
- **Layer 3 egress restrictions by domain** — configurable network policies via REST API (default allows LLM provider domains)
- Service discovery via Corrosion (gossip-based)

!!! warning "WebSocket proxy issues"
    Community reports indicate the proxy doesn't always handle WebSocket connections correctly. This is relevant for Mattermost's WebSocket API — testing would need to verify WS functionality works through the Fly proxy.

### Compute Limits

- **Maximum:** 8 CPUs, 16GB RAM per Sprite
- **No GPU support** — CPU-only
- **Pre-installed:** Python 3.13, Node.js 22.20, Go, Git, Claude Code
- **No Docker/OCI** — dependencies installed manually or captured via checkpoints

### Auto-Idle Lifecycle

```
Running (billed for CPU + RAM + hot storage)
    ↓ ~30s inactivity
Warm (reduced billing — storage only)
    ↓ ~10 minutes
Suspended (cold storage billing only — ~$0.02/GB-month)
    ↓ HTTP request arrives
Running (~1s wake time)
```

Up to 5 checkpoints are retained per Sprite. Destroying a Sprite permanently deletes all data and stops all billing.

---

## Pricing

### Per-Second Compute Rates

| Resource | Rate | Monthly Equivalent (24/7) |
|----------|------|--------------------------|
| CPU | $0.07/CPU-hour | ~$51/CPU-month |
| Memory | $0.04375/GB-hour | ~$32/GB-month |
| Hot storage (NVMe) | $0.000683/GB-hour | ~$0.50/GB-month |
| Cold storage (S3) | $0.000027/GB-hour | ~$0.02/GB-month |

### Real-World Cost Examples

| Scenario | Active Time | Cost |
|----------|-------------|------|
| 4-hour coding session (2 CPU, 4GB) | 4 hours | ~$0.44 |
| CI test env, 2 hours/day active | ~60 hrs/month | ~$15/month |
| Always-on Mattermost test instance (1 CPU, 2GB) | 24/7 | ~$115/month |
| Full capacity sustained (8 CPU, 16GB) | 24/7 | ~$655/month |

### Subscription Plans

| Plan | $/month | Max Concurrent Sprites | CPU Hours Included |
|------|---------|----------------------|-------------------|
| Adventurer | $20 | 20 | 450 |
| Veteran | $50 | 50 | 800 |
| Hero | $100 | 100 | 1,200 |
| Champion | $200 | 200 | 1,800 |

Usage beyond plan allowances billed at standard per-unit rates. Trial: **$30 in credits** (~500 Sprites worth of short sessions).

### Network Egress

- North America/Europe: $0.02/GB
- Asia Pacific/Oceania: $0.04/GB

### Cost Comparison

| Platform | Always-on 1 CPU / 2GB | Bursty 2 hrs/day |
|----------|----------------------|-------------------|
| **Fly.io Sprites** | ~$115/month | ~$15/month |
| **Hetzner Cloud CX22** | ~€4/month ($4.40) | ~€4/month (no scale-to-zero) |
| **EC2 t3.small** | ~$15/month | ~$15/month (with stop/start) |
| **Railway** | ~$5-20/month | ~$5-20/month |
| **Render** | ~$7/month | ~$7/month |

**Takeaway:** Sprites are cost-competitive only for **bursty, intermittent workloads** that benefit from scale-to-zero. For an always-on test Mattermost instance, a Hetzner VPS is 25x cheaper. The value proposition is **zero-ops persistence + instant wake** — not raw cost efficiency.

---

## Implementation: Mattermost Test Environment on Sprites

### How It Would Work

```
CI Job (GitHub Actions)
    │
    ├── Sprite wakes from idle on first API call (~1s)
    │
    ├── Mattermost server already running (state persisted from last run)
    │   ├── PostgreSQL data dir intact
    │   ├── Admin user exists
    │   ├── Agents provisioned
    │   └── Channels configured
    │
    ├── CI runs tests against https://mm-test.sprites.dev
    │
    └── Sprite auto-idles after ~30s of no traffic
```

### Setup Script (One-Time)

```bash
# Create a Sprite for the Mattermost test environment
sprite create mm-test-env

# Install Nix + devenv (or install Go/Node/PostgreSQL directly)
sprite exec mm-test-env -- bash -c '
  curl --proto "=https" --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh
  # ... clone repo, run ci-up.sh ...
'

# Checkpoint after initial setup (saves ~300ms snapshot)
sprite checkpoint mm-test-env --name "clean-state"
```

### Reset Between CI Runs

```bash
# Option A: Restore to clean checkpoint (~1s)
sprite restore mm-test-env --checkpoint "clean-state"

# Option B: Run reset script (wipe DB, re-seed)
sprite exec mm-test-env -- bash scripts/ci-reset.sh
```

### GitHub Actions Integration

```yaml
- name: Run agent tests against Sprite
  env:
    SPRITE_TOKEN: ${{ secrets.SPRITE_TOKEN }}
    MM_BASE_URL: https://mm-test-env.sprites.dev
  run: |
    # Wake the Sprite (auto-starts on HTTP request)
    curl -sf "$MM_BASE_URL/api/v4/system/ping" --retry 5 --retry-delay 2
    # Run tests against the persistent environment
    go test -tags=live ./platform/services/agentruntime/... -v
```

### CLI and SDK Reference

```bash
# CLI
sprite create <name>              # Create new Sprite
sprite exec <name> -- <command>   # Execute command
sprite console <name>             # Interactive SSH-like session
sprite checkpoint <name>          # Take snapshot
sprite restore <name>             # Restore from checkpoint
sprite destroy <name>             # Permanent delete
sprite org auth                   # Authenticate
```

```go
// Go SDK: github.com/superfly/sprites-go
client := sprites.NewClient(os.Getenv("SPRITE_TOKEN"))
sprite, _ := client.Create(ctx, "mm-test-env")
output, _ := sprite.Exec(ctx, "curl -sf localhost:8065/api/v4/system/ping")
```

```javascript
// JavaScript SDK: @fly/sprites
import { SpritesClient } from '@fly/sprites';
const client = new SpritesClient({ token: process.env.SPRITE_TOKEN });
const sprite = await client.create('mm-test-env');
const result = await sprite.exec('curl -sf localhost:8065/api/v4/system/ping');
```

---

## Limitations and Risks

### Reliability Concerns (as of Early 2026)

Community forums document several stability issues:

- **Creation failures:** Users reported ~1 in 5 sprite creations/executions succeeding during programmatic workloads, with the rest timing out. Improvements noted after February 2026.
- **Progressive degradation:** Sprites becoming slower over time; simple commands like `git status` taking minutes before complete I/O timeout. Root cause: I/O lockup in earlier runtime versions (fixed in rc35+).
- **No automatic updates:** Existing Sprites do not auto-update to fixed runtime versions. Requires manual intervention by Fly.io support.
- **Filesystem data loss:** At least one user reported home directory contents disappearing weekly (cause unconfirmed).

These are early-product issues and may be resolved by the time we adopt, but they warrant monitoring.

### Feature Gaps

- **No reboot command** — only checkpoint-restore or destroy-recreate
- **No SFTP/directory mounting** for IDE integration
- **No SSH agent forwarding** for private repo access
- **No usage dashboard** — billing visibility is poor
- **No region selection** — Sprites are auto-placed
- **No native secrets management** (unlike Fly Machines' `fly secrets set`)
- **No horizontal scaling** — each Sprite is a single VM
- **WebSocket proxy issues** — relevant for Mattermost WS API

### Billing Gotchas

- Sprites that fail to suspend properly continue consuming credits with no visible indicator
- Cold storage is always billed until the Sprite is destroyed
- Name reuse can cause hangs — generate unique names when rapidly cycling

---

## Comparison to Alternatives for This Use Case

For the specific use case of "persistent Mattermost test instance that CI jobs point at":

| Option | Monthly Cost | Wake Time | Persistence | Ops Overhead |
|--------|-------------|-----------|-------------|-------------|
| **Fly.io Sprite** | ~$5-15 (bursty) | ~1s | Full (100GB root) | Low |
| **Hetzner CX22** | ~$4.40 | N/A (always on) | Full | Medium (manage yourself) |
| **EC2 t3.small + stop/start** | ~$5-15 | ~30s (instance start) | EBS persists | Medium |
| **Railway** | ~$5-20 | ~5s (cold start) | Full | Low |
| **Render** | ~$7 | ~30s (free tier) | Full | Low |

### Verdict for This Project

Sprites are a **reasonable but not compelling** choice for the Mattermost CI test environment:

**Pros:**

- Zero-ops scale-to-zero saves cost vs. always-on VMs when CI runs are infrequent
- Checkpoint/restore enables instant clean-slate reset between test suites
- Go SDK available for programmatic control from CI

**Cons:**

- Early product with documented reliability issues (I/O lockups, creation failures)
- WebSocket proxy issues are concerning for Mattermost
- No native secrets management
- More expensive than Hetzner/EC2 for always-on workloads
- No GPU (irrelevant for us, but limits future LLM self-hosting)

**Recommendation:** Wait 3-6 months for stability to mature. In the meantime, a $4.40/month Hetzner VPS with a cron job that runs `scripts/ci-up.sh` achieves 90% of the same outcome with proven reliability. Revisit Sprites when the WebSocket proxy and reliability issues are resolved.

---

## References

| Resource | URL |
|----------|-----|
| Sprites Official Site | https://sprites.dev/ |
| Design & Implementation Blog | https://fly.io/blog/design-and-implementation/ |
| Launch Blog Post | https://fly.io/blog/code-and-let-live/ |
| Fly.io Pricing | https://fly.io/docs/about/pricing/ |
| CLI Reference | https://docs.sprites.dev/cli/commands/ |
| Go SDK | https://github.com/superfly/sprites-go |
| JS SDK | npm: @fly/sprites |
| Community: Stability Issues | https://community.fly.io/t/trouble-with-sprites-stability/27104 |
| Community: I/O Degradation | https://community.fly.io/t/sprites-becoming-non-responsive-after-usage/27137 |
| Simon Willison's Analysis | https://simonwillison.net/2026/Jan/9/sprites-dev/ |
| HN Discussion | https://news.ycombinator.com/item?id=46561089 |
