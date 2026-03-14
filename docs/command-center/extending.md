# Extending the Command Center

## Adding a new tab to the detail panel

1. Add a tab label to the `Tab` union type in `agent_detail_panel.tsx`.
2. Add a tab button in the tabs render.
3. Add tab content.

```tsx
// 1. Extend the union type
type Tab = 'active' | 'history' | 'tree' | 'memory' | 'settings' | 'analytics';

// 2. Add the button
{(['active', 'history', 'tree', 'memory', 'settings', 'analytics'] as Tab[]).map((t) => (
    <button
        key={t}
        className={`acc-detail-panel__tab${tab === t ? ' acc-detail-panel__tab--active' : ''}`}
        onClick={() => setTab(t)}
    >
        {t === 'analytics' ? 'Analytics' : t.charAt(0).toUpperCase() + t.slice(1)}
    </button>
))}

// 3. Add content
{tab === 'analytics' && <AnalyticsTab agentId={definition.id} />}
```

## Adding a new tile footer stat

Tile footer stats come from `AgentMetrics`. To surface a custom stat:

1. Add a field to `AgentMetrics` in `server/public/model/agent_task.go`.
2. Update the atomic counter in `server/platform/services/agentruntime/observability.go`.
3. Add a `<FooterStat/>` call in `agent_tile.tsx`.

```tsx
// In agent_tile.tsx footer section
<FooterStat label="P95 Latency" value={`${metrics?.p95_latency_ms ?? 0}ms`} />
```

## Custom dispatch modal

The "New Task" button in `AgentStatusBar` calls `onNewTask`. Wire it to Mattermost's `ModalController`:

```tsx
// In a parent component with Redux dispatch access
<AgentStatusBar
    onNewTask={() => dispatch(openModal({
        modalId: ModalIdentifiers.MY_DISPATCH_MODAL,
        dialogType: MyDispatchModal,
    }))}
    // ...
/>
```

## Adding a new built-in tool

1. Create a function in `server/platform/services/agentruntime/mmtools.go` (or a new file).
2. Register it in the `ToolRegistry` in `service.go`:

```go
registry.Register(AgentTool{
    Name:        "my_custom_tool",
    Description: "Does something useful",
    Parameters: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "query": map[string]any{"type": "string"},
        },
        "required": []string{"query"},
    },
    Execute: myCustomToolExecute,
})
```

3. Add it to the `tools` array in any `AgentDefinition` that should have access.

## Extending the tile via Module Federation

To inject a custom component into the tile grid from a plugin, register a module via Mattermost's `moduleRegistry`:

```typescript
// In your plugin's index.ts
window.registerPlugin('my-plugin', {
    agentTileExtension: (props: AgentTileExtensionProps) => MyTileExtension,
});
```

The command center checks `moduleRegistry` for `agentTileExtension` entries and renders them inside each tile's body section.
