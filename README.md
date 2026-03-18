# beads-pane (`bp`)

Terminal dashboard for monitoring [beads](https://github.com/steveyegge/beads) issue trackers across all your repos. Read-only, zero-config after first run.

```
┌────────────────────────────────────────────────────────────────────────┐
│ BEADS-PANE ◆ Agent Control Pane    8 repos │ poll 30s │ light │ 17:04 │
├──────────────────────┬─────────────────────────────────────────────────┤
│ Repositories         │ Issues · arche [254]                            │
│                      │  ID           Title              Status Pri Type│
│ ▸ arche         254  │  arche-l9f0   Security: JWT..    Open   P0  bug│
│   opencompany    41  │  arche-84jx   Security: Rem..    Open   P0  bug│
│   sticker_gen    12  │  arche-n9z9   Security: Mov..    Open   P0  bug│
│                      ├─────────────────────────────────────────────────┤
│ Aggregate            │ Detail                                          │
│  Total       323     │  arche-l9f0 ◆ Security: Implement proper JWT.. │
│  Open         34     │  Open │ P0 Critical │ bug                       │
│  In Prog      21     │  Created: 2026-01-15                            │
│  Blocked      10     ├─────────────────────────────────────────────────┤
│  Ready        22     │ Blocked [7]                                     │
│  Lead: 89.7h avg     │  opencompany  cuq.2  Add DB..     ←cuq.1       │
│                      ├────────────────────┬────────────────────────────┤
│ Priority             │ Epics              │ Daemons                    │
│  ■ P0 Critical    3  │  MVP chat runtime  │  ✓ opencompany  PID 45015 │
│  ■ P1 High       15  │  ████░░░░  0/9     │    17h45m  v0.24.0        │
│  ■ P2 Medium     12  │                    │                            │
├──────────────────────┴────────────────────┴────────────────────────────┤
│ q:quit  r:refresh  t:theme  Tab:pane  ↑↓:nav                          │
└────────────────────────────────────────────────────────────────────────┘
```

## Install

```bash
go install github.com/t3nsed/beads-pane@latest
```

Binary compiles as `bp`. Requires `bd` CLI: `brew install beads` or see https://github.com/steveyegge/beads

## Usage

```bash
bp                # starts in light mode (default)
bp --theme dark   # starts in dark mode
```

Press `t` at any time to toggle between light and dark themes.

On first run, you'll be asked where to scan for repos (defaults to `$HOME`). Config is saved to:
- macOS: `~/Library/Application Support/beads-pane/config.json`
- Linux: `~/.config/beads-pane/config.json`

## What it shows

- **Repositories**: All discovered repos with `.beads` directories, sorted by issue count
- **Aggregate stats**: Total/open/in-progress/blocked/closed/ready counts, average lead time
- **Priority distribution**: P0-P4 breakdown across all active issues
- **Labels**: Top labels across all repos by count
- **Issues table**: Non-closed issues for the selected repo
- **Issue detail**: Full detail for the selected issue
- **Blocked**: All blocked issues across every repo with their blockers
- **Epics**: Epic progress bars with child completion status
- **Daemons**: Running `bd` daemon processes with uptime and status

## Keys

| Key | Action |
|-----|--------|
| `q` | Quit |
| `r` | Force refresh |
| `t` | Toggle light/dark theme |
| `Tab` / `Shift+Tab` | Cycle focus between panels |
| `↑` `↓` | Navigate within focused panel |
| `Enter` | Select item |

## Config

Edit `config.json` to change:

```json
{
  "scan_roots": ["/home/you"],
  "poll_interval_seconds": 30,
  "max_scan_depth": 6
}
```

All `bd` access is strictly read-only (`--sandbox --no-auto-flush --no-auto-import`).

## License

MIT
