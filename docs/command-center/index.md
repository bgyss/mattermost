# Agent Command Center

The Agent Command Center is a full-screen, real-time control panel for the multi-agent platform. It surfaces every agent in a tile grid, streams live token output per tile, and provides deep-dive panels for tasks, delegation trees, memory, and settings.

## Opening the command center

| Method | How |
|--------|-----|
| URL | `http://localhost:8065/agents` |
| Global header | Click **Agents** in the top-right navigation bar |

On load, the team sidebar, left sidebar, and right sidebar are hidden; the command center fills the viewport. Normal channel view is restored when you navigate away.

## Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Tasks Today: 12   Tokens: 48k   Active: 2   Failed: 0  [Sales ▾] [+ Task] │  ← Status bar
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌───────────────────────────┐  ┌───────────────────────────┐               │
│  │ CEO             ● Running │  │ Sales Head      ● Pending │               │  ← Tile grid
│  │ …generating summary…      │  │ Waiting for CEO…          │               │
│  │ [search_posts]            │  │                           │               │
│  │ Tasks: 5  Tokens: 12k  [Dispatch] │  │ Tasks: 8  Tokens: 31k  [Dispatch] │ │
│  └───────────────────────────┘  └───────────────────────────┘               │
│                                                              ┌─────────────┐ │
│                                                              │ Sales Head  │ │  ← Detail panel
│                                                              │ Active Tasks│ │
│                                                              │ History     │ │
│                                                              │ Tree        │ │
│                                                              │ Memory      │ │
│                                                              │ Settings    │ │
│                                                              └─────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Quick links

- [Component Reference](components.md) — props and behaviour of each component
- [Redux & State](redux.md) — reducers, action creators, selectors
- [WebSocket Events](websocket.md) — real-time event handling
- [CSS & Theming](theming.md) — classes and custom properties
- [Extending](extending.md) — adding tabs, tile stats, custom dispatch modals
- [Tutorial](../tutorial/command-center.md) — guided walkthrough

## Verification checklist

- [ ] `/agents` loads → full-screen, sidebars hidden
- [ ] Agent tiles appear with names and workgroup badges
- [ ] Submit task → tile transitions idle → running, token stream updates
- [ ] Click tile → detail panel, Active Tasks tab shows running task
- [ ] Task completes → status pill turns blue, metrics update in footer
- [ ] Delegation Tree tab → shows parent→child chain when delegation occurred
- [ ] Memory tab → shows agent memory keys
- [ ] Settings tab → edit system prompt → PATCH saves → persists on reload
- [ ] Workgroup selector → only filtered agents shown
- [ ] **Agents** header link navigates to `/agents`
- [ ] Navigating away → body class removed, normal view restored
