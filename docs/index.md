# Mattermost Agent Platform

Mattermost includes a built-in **multi-agent orchestration platform** that lets you deploy, coordinate, and observe autonomous AI agents directly inside your team's workspace.

## What's included

| Component | Description |
|-----------|-------------|
| **Agent Runtime** | Go service (`agentruntime`) that drives the agentic loop — LLM calls, tool execution, streaming, delegation |
| **Workgroups** | Logical departments (Executive, Sales, Marketing…) that own agents and channels |
| **Agent Command Center** | Full-screen browser UI at `/agents` — live tile grid, token streams, delegation trees |
| **Built-in Tools** | `search_posts`, `create_post`, `get_channel`, `list_channels`, `get_user`, `delegate_task`, `route_to_capability`, `remember`, `recall` |
| **OpenClaw Integration** | Local LLM gateway (default); Anthropic and OpenAI as fallback providers |
| **Observable Coordination** | `ChannelTypeAgentDirect` channels that let admins watch agent-to-agent communication |

## Quick navigation

<div class="grid cards" markdown>

- :rocket: **[Quickstart](getting-started/quickstart.md)** — spin up the full demo in four `mise run` commands
- :building_construction: **[Architecture](architecture/overview.md)** — how the platform is structured end to end
- :computer: **[Command Center](command-center/index.md)** — reference for the `/agents` UI
- :books: **[Tutorial](tutorial/command-center.md)** — guided walkthrough from first launch to live delegation
- :test_tube: **[Demo Setup](demo/local-setup.md)** — provision CEO + Sales agents and run scenarios
- :cloud: **[Cloud Env](cloud/cursor-cloud.md)** — Cursor Cloud dual-repo enterprise setup

</div>

## Architecture at a glance

```
Human (Admin)
     │  @mention or API task submission
     ▼
AgentRuntimeService (Go)
     │  agentic loop: LLM → tools → stream → DB
     ├── OpenClaw / Anthropic / OpenAI
     ├── Built-in tools (search, post, delegate)
     ├── Memory manager (per-agent persistent KV)
     └── Dispatcher (queue + per-agent semaphores)
           │  delegate_task / route_to_capability
           ▼
     Sub-agents (same runtime, different definitions)
           │  posts results
           ▼
     Mattermost channels (observable by admins)
           │  WebSocket events
           ▼
     Agent Command Center (/agents)
```

## Delegation flow

```
CEO agent
  └── delegate_task → Sales Head agent
        ├── route_to_capability → Prospecting Specialist
        └── route_to_capability → Proposal Writer
```

Each level posts in its own channel; coordination channels let admins observe cross-workgroup traffic in real time.
