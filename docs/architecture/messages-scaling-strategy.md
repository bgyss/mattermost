# Messages Database Scaling Strategy

> **Status:** Deferred — not urgent for current deployment scales. Review when a single channel exceeds ~10M posts, total posts table exceeds ~500M rows, or p99 channel load latency exceeds 200ms.
>
> **Inspiration:** [How Discord Stores Trillions of Messages](https://discord.com/blog/how-discord-stores-trillions-of-messages) — their evolution from MongoDB → Cassandra → ScyllaDB is the reference arc for this strategy.

---

## Current state

Mattermost stores all messages in a single PostgreSQL `posts` table with no partitioning:

```sql
posts (
    id          VARCHAR(26) PRIMARY KEY,   -- Mattermost 26-char ID (not a Snowflake)
    createat    BIGINT,                    -- Unix ms timestamp (separate from ID)
    updateat    BIGINT,
    deleteat    BIGINT,                    -- Soft-delete: 0 = live
    channelid   VARCHAR(26),
    userid      VARCHAR(26),
    rootid      VARCHAR(26),              -- Thread root (empty = root post itself)
    message     VARCHAR(65535),
    type        VARCHAR(26),
    props       JSONB,
    ...
)
```

**Primary read pattern** — channel history, newest-first, offset-paginated:

```sql
SELECT * FROM posts
WHERE channelid = ? AND deleteat = 0
ORDER BY createat DESC
LIMIT 25 OFFSET 0;   -- page 0
LIMIT 25 OFFSET 975; -- page 39 — full table scan to skip 975 rows
```

**Critical index:** `(channelid, deleteat, createat)` — this composite index is the load-bearing structure for almost every channel history query.

### What breaks at scale

| Problem | Trigger | Why |
|---------|---------|-----|
| Slow deep pagination | `OFFSET N` grows large | Postgres must scan and discard N rows even with an index |
| Index bloat | Table > ~100M rows | B-tree index pages grow; random I/O increases |
| VACUUM pressure | High write/soft-delete volume | Dead tuple accumulation slows all scans |
| Hot partition | Single very active channel | All reads/writes contend on the same index leaf pages |
| Thread parent lookups | Subquery per page | `getParentsPosts` runs a correlated subquery on every page load |
| `props` JSONB | Unstructured, unbounded | Large props blobs inflate row size; no per-field index |

---

## What Discord teaches us

Discord's message storage went through three generations. Each transition was driven by a specific failure mode at scale:

### Generation 1: MongoDB (single node, working set exceeded RAM)
The problem: MongoDB stored everything in RAM-indexed B-trees. Once the message working set exceeded available RAM, every cache miss became a disk seek. Random I/O killed latency.

**Lesson:** Message storage must be designed for write-heavy workloads where the "hot" window is much smaller than total data.

### Generation 2: Cassandra (LSM trees, time-bucketed partitions)
The key architectural insight was the **data model**:

```
Partition key:  (channel_id, bucket)
Clustering key: message_id DESC
```

Where `bucket` is a time-based integer — e.g., `floor(createat / BUCKET_WINDOW_MS)`. Each partition holds roughly 10 days of messages for one channel. This means:

- No partition ever grows unboundedly — a dormant channel's partition is small; an active channel's current partition is bounded by the window
- Reads for "latest messages" hit only the current bucket (single partition scan)
- Old messages are in cold partitions that never need to be touched
- Linear horizontal scale: add nodes, partitions distribute automatically

LSM (Log-Structured Merge) trees flip the I/O profile: all writes are sequential appends to memtables flushed to SSTables. Random reads become range scans on sorted files. This is ideal for the message access pattern: append-heavy writes, range reads by time.

**Lesson:** Time-bucketed partition keys + LSM trees are the right primitive for an append-heavy time-series workload.

### Generation 3: ScyllaDB (C++ Cassandra, per-core scheduling, no GC)
Cassandra's JVM GC caused latency spikes at p99/p999. ScyllaDB is API-compatible but written in C++ with the Seastar async framework, giving each CPU core its own scheduler and eliminating GC pauses.

Additionally, Discord built a **Rust data service** layer in front of ScyllaDB to:
- Cache hot reads in process memory
- Coalesce concurrent reads for the same partition (request deduplication)
- Handle schema evolution without touching the DB layer

**Lesson:** At trillion-message scale, GC latency is observable. Per-core async IO and a caching service layer matter. For Mattermost's scale targets, Cassandra is likely sufficient before ScyllaDB becomes necessary.

---

## ScyllaDB deep-dive

ScyllaDB ([scylladb.com](https://www.scylladb.com), [github.com/scylladb/scylladb](https://github.com/scylladb/scylladb)) is the recommended choice for Step 4. It deserves its own section.

### Architecture

ScyllaDB uses the **Seastar** async framework: each CPU core runs its own event loop with its own memory arena, network queues, and storage queues. There is no shared memory between shards — all inter-shard communication is explicit message-passing. This means:

- **No GC pauses** — C++, no JVM
- **No lock contention** — shards don't share state
- **Predictable tail latency** — p99 and p999 are close to p50 because one slow shard doesn't block others
- **Linear core scaling** — adding CPUs directly increases throughput

On the same hardware, ScyllaDB typically achieves 10× the throughput of Apache Cassandra at equivalent latency.

### Key features relevant to message storage

**Tablets (ScyllaDB 6.0+):** The traditional Cassandra/ScyllaDB approach uses consistent hashing to assign token ranges to nodes. Rebalancing after adding a node requires streaming large amounts of data. Tablets are a finer-grained unit — each tablet is a sub-range of a partition's token space. Adding a node moves individual tablets rather than entire token ranges, making rebalancing faster and less disruptive.

**Workload prioritisation:** ScyllaDB allows tagging queries as `USING SERVICE_LEVEL` so that, for example, real-time channel reads are prioritised over background analytics queries on the same cluster.

**Change Data Capture (CDC):** ScyllaDB has native CDC that writes a delta log for every mutation. This enables:
- Streaming message changes to Elasticsearch for full-text search without dual-writes from the application
- Event sourcing patterns for the agent task system
- Audit logs without additional instrumentation

**Raft consensus:** Newer ScyllaDB versions use Raft for schema changes and topology operations (replacing the older gossip-based approach). This means schema migrations are strongly consistent — important for a multi-node production deployment.

**Alternator:** ScyllaDB's DynamoDB-compatible API. If DynamoDB is already in use (e.g., for a cloud-hosted Mattermost), Alternator means self-hosted ScyllaDB can serve as a drop-in replacement without changing application code.

### ScyllaDB vs Apache Cassandra

| | Apache Cassandra | ScyllaDB |
|--|--|--|
| Language | Java (JVM) | C++ (Seastar) |
| GC pauses | Yes — p99 latency spikes | No |
| Throughput (same HW) | Baseline | ~10× higher |
| CQL compatibility | Native | Full (driver-level compatible) |
| Maturity | Very high (2008) | High (2015, production at Discord, Comcast, etc.) |
| Managed cloud | Datastax Astra | ScyllaDB Cloud / Serverless |
| Tablets (elastic rebalance) | No | Yes (v6.0+) |
| CDC | External (Debezium) | Native |
| nixpkgs | `pkgs.cassandra` | `pkgs.scylladb` |

**Recommendation:** Start with Cassandra in devenv (simpler setup, no AVX requirement) and deploy ScyllaDB in production. The CQL wire protocol and driver (`gocql`) work identically with both.

### ScyllaDB deployment modes

| Mode | When to use |
|------|-------------|
| Self-hosted cluster | On-prem or dedicated cloud VMs; full control |
| ScyllaDB Cloud | Managed, serverless billing; fastest to production |
| ScyllaDB Serverless | Pay-per-request; ideal for variable/bursty workloads |
| Embedded (single node) | Local dev via devenv; also suitable for small self-hosted deployments |

### Go driver

Use [`gocql`](https://github.com/gocql/gocql) — the standard CQL driver for Go, compatible with both Cassandra and ScyllaDB. For ScyllaDB specifically, the [`gocql` shard-aware driver](https://github.com/scylladb/gocql) (a fork maintained by ScyllaDB) routes each request directly to the shard that owns the data, avoiding an extra intra-node hop. Use the ScyllaDB fork in production.

```go
import "github.com/scylladb/gocql"

cluster := gocql.NewCluster("scylladb-host:9042")
cluster.Keyspace = "mattermost_messages"
cluster.Consistency = gocql.Quorum
session, _ := cluster.CreateSession()
```

---

## Alternatives comparison

Before committing to ScyllaDB/Cassandra, these alternatives are worth evaluating for specific constraints:

### Apache Cassandra
**Best for:** Teams already operating Cassandra; when ScyllaDB's C++ dependency or AVX CPU requirement is a constraint.
**Avoid if:** p99 latency matters and hardware is shared; ScyllaDB is strictly better on the same workload.

### CockroachDB
**Best for:** If you want PostgreSQL-compatible SQL with horizontal scale — CockroachDB uses the PostgreSQL wire protocol. Transactions, joins, and foreign keys work. It uses Raft for distributed consensus and auto-sharding.
**Trade-off:** Significantly higher latency than ScyllaDB for the message write pattern (every write goes through Raft consensus). Better fit for metadata (users, channels, permissions) than for the high-throughput append-only message stream. Worth considering as a **replacement for PostgreSQL** rather than as a message store alternative to ScyllaDB.

### YugabyteDB
**Best for:** PostgreSQL-compatible distributed SQL, similar positioning to CockroachDB but with both a PostgreSQL-compatible API (YSQL) and a Cassandra-compatible API (YCQL) in one system. Could serve both the relational (metadata) and time-series (messages) workloads on a single cluster.
**Trade-off:** More operationally complex; less mature than either PostgreSQL or Cassandra. The dual-API approach is appealing but means running one cluster at twice the complexity.

### FoundationDB + Record Layer
**Best for:** If you need ACID transactions across arbitrary key ranges with horizontal scale. Apple uses FoundationDB for iCloud. The Record Layer (open-sourced by Apple/FoundationDB) provides a structured record abstraction on top.
**Trade-off:** Steep operational learning curve; smaller community than Cassandra. Key-value model requires more application-level structure. Not a natural fit for the Mattermost codebase.

### ClickHouse
**Best for:** **Analytics and search** on message data — not primary storage. ClickHouse is a columnar OLAP database optimised for aggregation queries over large datasets. It would be an excellent store for message analytics (message volume by channel, active user counts, search-adjacent features) but is not suitable as the primary read/write message store.
**Use case here:** Could sit alongside ScyllaDB as an analytics replica — CDC from ScyllaDB → ClickHouse for admin dashboards and search features.

### Amazon DynamoDB
**Best for:** Cloud-only Mattermost deployments on AWS where operational simplicity trumps cost. DynamoDB's data model is essentially Cassandra's (partition key + sort key + LSM storage).
**Trade-off:** Vendor lock-in; expensive at high throughput; no self-hosted option. ScyllaDB Alternator provides the same API self-hosted if lock-in is a concern.

### Summary table

| Database | Model | Best role | Self-hosted | Notes |
|----------|-------|-----------|-------------|-------|
| **ScyllaDB** | Wide-column (CQL) | Message hot store | Yes | Recommended for Step 4 |
| Apache Cassandra | Wide-column (CQL) | Message hot store | Yes | Easier devenv; lower prod throughput |
| CockroachDB | Distributed SQL | Replace PostgreSQL for metadata | Yes | Not for message stream |
| YugabyteDB | Distributed SQL + CQL | Both tiers on one cluster | Yes | Higher operational complexity |
| FoundationDB | KV + Record Layer | Transactional KV at scale | Yes | Steep learning curve |
| ClickHouse | Columnar OLAP | Analytics replica | Yes | Complements, not replaces ScyllaDB |
| DynamoDB | Wide-column (proprietary) | Cloud-only message store | No (Alternator for self-host) | Lock-in risk |

---

## Recommended strategy for Mattermost

The path below is incremental. Each step independently improves the situation without requiring the next step.

### Step 1: Cursor-based pagination (replace OFFSET) — low effort, high impact

Replace `LIMIT N OFFSET M` with keyset (cursor) pagination using `(createat, id)`:

```sql
-- Instead of:
SELECT * FROM posts WHERE channelid = ? AND deleteat = 0
ORDER BY createat DESC LIMIT 25 OFFSET 975;

-- Use:
SELECT * FROM posts WHERE channelid = ? AND deleteat = 0
  AND (createat, id) < (?, ?)   -- cursor from last item of previous page
ORDER BY createat DESC, id DESC LIMIT 25;
```

The existing `idx_posts_create_at_id` index on `(createat, id)` already supports this. The API surface change is: `page` parameter becomes an opaque cursor token (base64-encoded `(createat, id)` pair). This eliminates deep-pagination scans entirely.

**Files:** `server/channels/store/sqlstore/post_store.go` — `GetPostsOptions.Page` → `GetPostsOptions.AfterCursor`.

---

### Step 2: Declarative time partitioning on the posts table — medium effort

Partition `posts` by `createat` using PostgreSQL declarative range partitioning. This reduces index size for queries on recent data, allows old partitions to be archived to cheaper storage, and enables partition-level VACUUM.

```sql
-- New DDL (migration)
CREATE TABLE posts_partitioned (
    LIKE posts INCLUDING ALL
) PARTITION BY RANGE (createat);

-- Monthly partitions (adjust granularity to deployment size)
CREATE TABLE posts_2024_01 PARTITION OF posts_partitioned
    FOR VALUES FROM (1704067200000) TO (1706745600000);
CREATE TABLE posts_2024_02 PARTITION OF posts_partitioned
    FOR VALUES FROM (1706745600000) TO (1709251200000);
-- ... automated partition management job creates future partitions weekly
```

A background job creates the next partition window before it's needed and can detach + compress partitions older than a configurable retention window (e.g., 2 years online, archived beyond that).

**Key constraint:** The partition key (`createat`) must be part of every index. The existing `(channelid, deleteat, createat)` index already satisfies this.

---

### Step 3: Mattermost ID → Snowflake-style sortable ID

Mattermost uses a custom 26-char base-62 ID. It encodes a timestamp but was not designed to be monotonically sortable. Discord's Snowflake IDs are 64-bit integers where the top bits are a millisecond timestamp — this means `ORDER BY id` and `ORDER BY createat` are equivalent, removing the need for a separate `createat` column in range queries.

Migrating IDs is a large breaking change. The near-term alternative: ensure `(channelid, createat, id)` is used as the canonical sort for all channel queries (already approximately true) so the id suffix acts as a tiebreaker. This is good enough until an ID redesign is warranted.

---

### Step 4: Introduce a Cassandra/ScyllaDB hot message store

This is the Discord-pattern inflection point. At very large scale, move recent messages (e.g., last 90 days) to a Cassandra-compatible store while keeping PostgreSQL as the source of truth for metadata, user data, channels, and historical archives.

**Data model mirroring Discord's:**

```
Table: messages
Partition key:  (channel_id, bucket)
Clustering key: message_id DESC

bucket = floor(createat_ms / 604800000)  -- 7-day windows
```

**Read path:**
1. Compute which buckets overlap the requested time range
2. Query Cassandra for those bucket partitions
3. Fall back to PostgreSQL for messages older than the Cassandra retention window

**Write path:**
1. Write to PostgreSQL (source of truth, async replication or dual-write)
2. Write to Cassandra (primary fast-path for recent reads)

**Why this works:**
- Recent messages (the vast majority of reads) are served from Cassandra's LSM-tree with sequential I/O
- PostgreSQL handles everything else: search indexing, user data joins, admin queries, historical exports
- The two stores have complementary strengths — no need to replace PostgreSQL entirely

**Infrastructure:** ScyllaDB is available in nixpkgs (`pkgs.scylladb`) and can be added to `devenv.nix` as a service for local development.

---

### Step 5: Rust data service layer (Discord's "Superdisk")

Discord's final architecture added a Rust service between their API servers and ScyllaDB:

- **Request coalescing:** Multiple concurrent requests for the same channel page are deduplicated — only one DB query runs, result is fanned out to all waiters
- **In-process hot cache:** The most recent page of each active channel is kept in memory, invalidated on new posts via WebSocket event
- **Schema translation:** The service owns the Cassandra schema, hiding it from API servers — schema migrations don't require coordinated API deploys

For Mattermost, this maps to a `MessageService` that sits between `api4/` and `store/`:

```
api4/post.go
    → app/post.go
        → MessageService (new)
            → hot cache (in-process)
            → Cassandra (recent, via bucket queries)
            → PostgreSQL (historical, metadata joins)
```

This is the final form. It's significant infrastructure investment and only justified at Discord-like message volumes.

---

## Priority summary

| Step | Effort | Impact | When |
|------|--------|--------|------|
| 1. Cursor pagination | Low | High — eliminates deep-scan | Posts table > 50M rows |
| 2. Time partitioning | Medium | High — VACUUM, archive, cold storage | Posts table > 200M rows |
| 3. Snowflake IDs | High | Medium — cleaner sort, smaller indexes | ID redesign milestone |
| 4. Cassandra hot store | High | Very high — horizontal scale | Single channel > 10M posts or p99 > 200ms |
| 5. Rust data service | Very high | Very high — coalescing, cache | Discord-tier message volumes |

Steps 1 and 2 are purely PostgreSQL work and should be done first — they buy substantial headroom before any new infrastructure is needed.

---

## What not to do

- **Don't replace PostgreSQL for metadata.** Cassandra has no joins, no transactions, no flexible querying. User profiles, channel membership, permissions, threads, reactions, and search all need a relational model. The right split is: PostgreSQL for everything relational, Cassandra/ScyllaDB for the append-only message stream.
- **Don't use offset pagination as a cursor.** Encoding `page=39` in an API and computing `OFFSET 975` will always degrade — fix this before adding infrastructure.
- **Don't partition without cursor pagination.** Partition pruning requires the partition key in the `WHERE` clause. If queries still use `OFFSET`, the planner may scan multiple partitions anyway.
- **Don't add Elasticsearch as the message store.** Elasticsearch is a search index, not a primary store. It's already used (optionally) in Mattermost for full-text search. Keep it as a secondary index, not a read path for channel history.

---

## devenv / local development

ScyllaDB can be added to `devenv.nix` when Step 4 is reached:

```nix
# devenv.nix addition for Cassandra/ScyllaDB local dev
packages = with pkgs; [ cassandra ... ];  # or scylladb if in nixpkgs
processes.cassandra = {
  exec = "cassandra -f";  # foreground mode
};
```

The Go driver to use is `gocql` (widely used, battle-tested with both Cassandra and ScyllaDB). The Mattermost store layer's interface pattern (`store.PostStore`) makes it straightforward to add a second implementation backed by `gocql` without touching the app or API layers.
