# Rune Surface Migration — first pass

Status: **four surfaces visible; Run is the only writable surface.**

| Surface | Current Rune state | Not yet |
|---|---|---|
| 实验设计 | read-only design list (title, assay count) | four-step editor, matrix, inspector |
| Protocol | read-only protocol list (title, revision) | block tree, parameters, version editing |
| 实验运行 | writable Run slice (task/check/measurement/results) | full parity, restart verification |
| 资料与工具 | read-only record list (formula/table/note) | resources, calculators, instruments |

Shell:

- custom 40pt titlebar; no Windows native frame
- 48–56px icon rail; active page uses violet
- collapsible project/design sidebar; `侧栏` icon in titlebar
- project as section, design as file-like row
- 900px: sidebar can be collapsed manually; Run readiness card scrolls below the record card

Screenshots:

- `rune-gate2-1200.png` — Run, 2/3 + 1/3
- `rune-page-design.png`
- `rune-page-protocol.png`
- `rune-page-tools.png`

This is a migration scaffold, not parity. Do not claim the Web UI has been retired.
