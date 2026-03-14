# PostgreSQL Scaling Strategy

> **Status:** Deferred — not urgent for current demo scale. Review when agent concurrency exceeds ~20 simultaneous tasks or when `AgentTaskEvents` table exceeds 10M rows.

## Current architecture

Mattermost is tightly coupled to a single PostgreSQL instance:

- **Driver:** `pgx` via `database/sql`, connection pool controlled by `SqlSettings.MaxOpenConns / MaxIdleConns`
- **Query builder:** `squirrel` (raw SQL, not an ORM)
- **Migrations:** `morph` — agent tables added in `000156_create_agent_tables.*`
- **No built-in read replica routing** — all queries hit one DSN

In production, Mattermost deployments place **PgBouncer** in front to keep raw connection counts manageable across multiple app nodes.

---

## The problem: write amplification in AgentTaskEvents

The agent tables have very different write profiles:

| Table | Write pattern | Risk at scale |
|-------|--------------|--------------|
| `AgentDefinitions` | Rare (config changes) | None |
| `AgentTasks` | Moderate (status transitions) | Low |
| `AgentMemory` | Moderate (upserts per task) | Low |
| `AgentTaskEvents` | **One row per token chunk** | **High** |

At ~30 tokens/sec per agent, 10 concurrent agents = **~300 DB writes/second** just for token events. At 100 agents this is untenable on a single node.

The root mismatch: **token chunks are ephemeral** (only needed live during task execution) but are persisted to a durable, ACID-transactional database. This is expensive for what is fundamentally a pub/sub problem.

### What's already good

The metrics system (`agentruntime/observability.go`) uses **atomic in-memory counters** — no DB queries for `GET /api/v4/agents/metrics`. This is the right pattern. Token streaming should follow the same philosophy.

---

## Recommended work, in priority order

### 1. Replace token streaming with Redis pub/sub or NATS (highest leverage)

**Problem:** Every LLM token chunk writes a row to `AgentTaskEvents`.
**Fix:** Broadcast token chunks via Redis pub/sub or NATS. The WebSocket hub subscribes and fans out to browsers. Only write **final task output** and **durable events** (tool calls, errors) to Postgres.

```
Current:  LLM token → DB write → WS broadcast
Target:   LLM token → Redis pub/sub → WS broadcast
                                    └→ DB write (final output only, on task complete)
```

This eliminates the dominant write load without changing the external API.

**Files to touch:**
- `server/platform/services/agentruntime/executor.go` — publish to Redis instead of writing `AgentTaskEvent` rows for token chunks
- `server/channels/wsapi/` or hub — subscribe to Redis channel, broadcast `agent_token_stream` WS events
- Keep `AgentTaskEvents` for durable event types: `tool_call`, `thinking`, `error`, `delegation`

---

### 2. Partition AgentTaskEvents by time

**Problem:** `AgentTaskEvents` grows unboundedly; vacuuming and index scans get slow.
**Fix:** Declarative table partitioning by week. Old partitions can be detached and archived without locking.

```sql
-- Migration: convert to partitioned table
CREATE TABLE AgentTaskEvents (
    ...
    create_at BIGINT NOT NULL
) PARTITION BY RANGE (create_at);

CREATE TABLE AgentTaskEvents_2026_w01 PARTITION OF AgentTaskEvents
    FOR VALUES FROM (1735689600000) TO (1736294400000);
-- Add a job to create partitions weekly and detach/archive partitions older than N weeks
```

---

### 3. TTL cleanup job for AgentTaskEvents

**Problem:** No automatic cleanup — the table grows forever.
**Fix:** Extend the existing `agent_digest` job (or add a separate job) to delete task events older than a configurable retention window (default: 30 days).

We already have `DeleteExpiredMemory` in `AgentMemory` — follow the same pattern.

---

### 4. Add PgBouncer to the devenv setup

**Problem:** devenv Postgres has no connection pooler, which doesn't match production topology.
**Fix:** Add `services.pgbouncer` (or a manual process) in `devenv.nix` pointing at the local Postgres instance. This lets developers catch connection exhaustion bugs locally before they hit production.

```nix
# devenv.nix addition
packages = with pkgs; [ pgbouncer ... ];
processes.pgbouncer = {
  exec = "pgbouncer ${config.devenv.root}/.devenv/pgbouncer.ini";
  process-compose.depends_on.postgres.condition = "process_healthy";
};
```

---

### 5. Distributed task dispatcher (when running multiple Mattermost nodes)

**Problem:** The current dispatcher is in-process (in-memory queue + per-agent semaphores). With multiple Mattermost nodes, each node has its own queue — tasks aren't shared fairly.
**Fix:** Move to one of:

- **Postgres `SKIP LOCKED`** — simplest, no new infrastructure. Workers `SELECT ... FOR UPDATE SKIP LOCKED` on `AgentTasks WHERE status = 'pending'`. Postgres handles coordination.
- **Redis lists** — fast, low-latency, already likely present in the stack.
- **NATS JetStream** — best fit if token streaming moves to NATS anyway (see item 1).

`SKIP LOCKED` is the recommended first step — it requires no new services and handles the multi-node case correctly.

```sql
-- Dispatcher query with SKIP LOCKED
SELECT * FROM AgentTasks
WHERE status = 'pending'
ORDER BY priority DESC, create_at ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;
```

---

### 6. pgvector for semantic AgentMemory search

**Problem:** `AgentMemory` is a flat KV store — recall is by exact key only.
**Fix:** Add a `embedding vector(1536)` column to `AgentMemory` and use `pgvector` for semantic recall. This keeps the semantic search inside Postgres (no separate vector DB) and leverages the existing connection pool.

```sql
ALTER TABLE AgentMemory ADD COLUMN embedding vector(1536);
CREATE INDEX ON AgentMemory USING ivfflat (embedding vector_cosine_ops);
```

Recall becomes: embed the query → nearest-neighbour search → return top-K memories.

This is where Postgres has a structural advantage: the vector index sits next to the data, so joins are free.

---

### 7. Read replica routing for metrics and history queries

**Problem:** Analytics-style queries (task history, metrics aggregation, delegation tree traversal) contend with write-heavy task execution on the same DB node.
**Fix:** Route read-only queries to a replica. Mattermost's store layer supports a `ReplicaLagSettings` pattern. Agent store read methods (`GetAgentTask`, `GetTaskTree`, `GetAgentMemory`) should use the replica connection when available.

---

## What NOT to do

- **Don't switch away from Postgres** for the primary store. Mattermost's entire store layer, migrations, and tooling are Postgres-native. The right path is optimising Postgres usage, not replacing it.
- **Don't add a separate time-series DB** (InfluxDB, Prometheus remote write) for agent metrics. In-memory atomic counters (already implemented) are sufficient at the scales this platform targets. Export to Prometheus via a scrape endpoint if dashboards are needed.
- **Don't use a separate vector DB** (Pinecone, Weaviate). pgvector covers the use case and avoids operational complexity.

---

## devenv Postgres notes (local dev)

The devenv-managed instance is suitable for development but:

- Data lives in `.devenv/state/postgres` — local only, not backed up
- Default Postgres config (not tuned) — for accurate local perf testing, set `shared_buffers = 256MB`, `work_mem = 16MB`, `max_connections = 200` in a `postgresql.conf` override in `devenv.nix`
- No WAL archiving or replication

Production deployments should follow the [Mattermost PostgreSQL configuration guide](https://docs.mattermost.com/install/software-hardware-requirements.html).
