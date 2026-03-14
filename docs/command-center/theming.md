# CSS & Theming

## Full-screen layout

`AgentCommandCenter` toggles `body.agent-command-center` on mount and removes it on unmount.

The SCSS at `webapp/channels/src/sass/base/_agent_command_center.scss` collapses the standard layout:

```scss
body.agent-command-center {
    #TeamSidebar, .sidebar--left, .sidebar--right { display: none !important; }
    .agent-command-center-root { flex: 1; display: flex; flex-direction: column; }
}
```

## CSS class reference

| Class | Element |
|-------|---------|
| `.acc-status-bar` | Top metrics ribbon |
| `.acc-grid` | Tile grid container |
| `.acc-tile` | Individual agent tile |
| `.acc-tile--selected` | Highlighted tile (detail panel open) |
| `.acc-tile__header` | Tile header row |
| `.acc-tile__avatar` | Initials avatar |
| `.acc-tile__status-pill` | Status pill base |
| `.acc-tile__status-pill--idle` | Gray idle state |
| `.acc-tile__status-pill--pending` | Amber pending state |
| `.acc-tile__status-pill--running` | Green pulsing state |
| `.acc-tile__status-pill--complete` | Blue complete state |
| `.acc-tile__status-pill--failed` | Red failed state |
| `.acc-tile__body` | Live token stream area |
| `.acc-tile__tool-badges` | Tool call badge row |
| `.acc-tile__footer` | Metrics + dispatch button row |
| `.acc-dispatch-form` | Inline task submission form |
| `.acc-detail-panel` | Right drawer container |
| `.acc-detail-panel__tabs` | Tab button row |
| `.acc-detail-panel__tab` | Individual tab button |
| `.acc-detail-panel__tab--active` | Active tab |
| `.acc-settings-form` | Settings form in detail panel |

## Custom properties

All colors use Mattermost theme CSS custom properties. To override for the command center only, target `.agent-command-center-root`:

```scss
.agent-command-center-root {
    --acc-tile-bg:       var(--center-channel-bg);
    --acc-tile-border:   var(--center-channel-color-16);
    --acc-running-color: #3fc380;
    --acc-pending-color: #f4a733;
    --acc-failed-color:  var(--error-text);
    --acc-complete-color: var(--link-color);
}
```

## Running dot animation

The "Running" pulse is a CSS animation on `.acc-tile__status-pill--running::before`:

```scss
@keyframes acc-pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50%       { opacity: 0.4; transform: scale(1.4); }
}

.acc-tile__status-pill--running::before {
    content: '';
    display: inline-block;
    width: 8px; height: 8px;
    border-radius: 50%;
    background: var(--acc-running-color);
    animation: acc-pulse 1.4s ease-in-out infinite;
    margin-right: 6px;
}
```
