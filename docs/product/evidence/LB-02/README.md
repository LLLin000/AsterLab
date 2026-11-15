# LB-02 Evidence — Run Workspace Three-Panel Layout (V2)

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, base `origin/main` (`f624c8a`), isolated `ovnfs` checkout `O:\lb02`.
- Branch: `agent/LB-02-run-panel`.
- Scope files modified:
  - `prototypes/mygo-run/internal/runworkspace/model.go`
  - `prototypes/mygo-run/internal/runworkspace/view.go`
  - `prototypes/mygo-run/internal/runworkspace/recording.go`
  - `prototypes/mygo-run/internal/runworkspace/view_test.go`
  - `prototypes/mygo-run/internal/runworkspace/lb02_evidence_test.go`
  - `docs/product/evidence/LB-02/`

## UX Contract Rule Citations
- `UX-LAYOUT` — at `≥1100` the Run surface is three columns: left vertical flow chain
  (hierarchy, progress, timepoint, status, selected task highlighted), middle record
  projection (`RecordingView`: plate/table/files), right compact properties. Below
  `1100` it is one column with the recording first and the procedure metadata collapsed.
- `UX-P1` (Task first) — at `900×600` the record projection is the first content block;
  the procedure metadata is behind the collapsed `操作说明与元数据` disclosure.
- `UX-P2` (Progressive disclosure) — the default view shows no raw `step`/`block`/`protocol`
  identifier. Paths, step ids, protocol versions and module ids sit in the `技术属性（只读）`
  disclosure, one click away.
- `UX-STATE` — every status prints the contract word `未开始 / 进行中 / 已完成 / 阻塞`;
  the raw Core values (`planned`, `in_progress`, `completed`, `blocked`, `running`) never
  reach the surface.
- `UX-P3` (One component language) — spacing, section tabs, chips and headers use the
  existing theme tokens and Onevra components; the change adds no literal colour or font
  size to the run surface.
- `UX-P5` (Recoverable state) — switching tasks keeps each task's recording draft; the
  selected task, tab and disclosure state live in `State`, and per-task drafts are keyed
  by task key (`State.RecordingDirty` / `taskDrafts`).

## Automated Checks
- `cd prototypes/mygo-run && go test ./... -count=1` → exit 0.
- Focused coverage in `internal/runworkspace`:
  - `TestWideLayoutPlacesRecordingBetweenFlowAndProperties` — flow < recording < properties
    at `1360×880`.
  - `TestDefaultViewHidesRawIdentifiers` — no raw identifier or raw status at `1360×880`
    or `900×600`.
  - `TestStatusLabelContractVocabulary` — `planned/in_progress/completed/blocked` map to
    `未开始/进行中/已完成/阻塞`.
  - `TestViewNarrowStacks` — at `900×600` the recording precedes the procedure metadata.
  - `TestProcedureShowsFrozenProtocolReadOnly` — frozen instruction/parameters stay visible;
    raw identifiers stay behind `技术属性（只读）` and appear after expansion.
- `./scripts/ci/local-gate.ps1 -Pr <n>` → `LOCAL_GATE: PASS`.

## Build Stamp

`scripts/mygo-build.ps1` writes `prototypes/mygo-run/dist/BUILD_INFO.txt` from
`git rev-parse --short HEAD` and the built artifact hashes:

```
AsterLab MyGo Prototype (UNSIGNED)
commit <short HEAD>
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 ...
mygo-run.exe SHA256 ...
```

The Windows gate rebuilds at the PR head, so `commit` in `BUILD_INFO.txt` equals
`git rev-parse --short HEAD` for the exact head that was gated.

## Screenshots

Rendered from the same `runworkspace.View` that the app mounts, with a plate/table
`RecordingView` callback.

| File | Size | Theme | Shows |
|---|---|---|---|
| `lb02-1360x880-light.png` | 1360×880 | light | three columns: flow chain / record projection / compact properties |
| `lb02-1360x880-dark.png` | 1360×880 | dark | three columns, dark tokens |
| `lb02-900x600-light.png` | 900×600 | light | single column: record first, procedure metadata collapsed, flow chain below |
| `lb02-900x600-dark.png` | 900×600 | dark | single column, dark tokens |
| `lb02-properties-collapsed-1360x880-light.png` | 1360×880 | light | before: `技术属性（只读）` collapsed, no raw identifiers |
| `lb02-properties-expanded-1360x880-light.png` | 1360×880 | light | after: paths, step id, protocol version and module ids revealed in one click |

## Real-window evidence addendum

Captured 2026-11-15 by launching `prototypes/mygo-run/dist/mygo-run.exe` and
resizing its real window with `MoveWindow`. The recorded window sizes are the
outer-window sizes; the screenshot tool captures the client area (1344×871 at
1360×880, 884×591 at 900×600). Run `369ec46b-ab25-4820-8fb0-25b1b762de51`
contains one frozen task with a measurement plan and one saved OD450 readout
(0.62 AU, Control-1), seeded through the AsterDoc CLI.

Build commit: `c55ae66`.

```text
Runs.dll SHA256 220E9C9CB0550C54F7E6C85EDDF698F04313FC8190C014BF070BAA94BB3BE238
mygo-run.exe SHA256 5E0F8D2BC32761A694B2B378FD2A4ED61BA190319ADBBAD9D39CA716A0348793
```

| Real-window image | Window size | Theme | State | PNG SHA256 |
|---|---:|---|---|---|
| `lb02-real-1360x880-light-properties-collapsed.png` | 1360×880 | light | properties collapsed | `a50b1c56c24ebc39eee84b160203320ab588973a209d074d29e2786935cecff4` |
| `lb02-real-1360x880-light-properties-expanded.png` | 1360×880 | light | properties expanded | `0a11ce58bd9268e25a9586a3f198e2256e6071bb1ab7b86a26c03b71ebe187ae` |
| `lb02-real-1360x880-dark-properties-collapsed.png` | 1360×880 | dark | properties collapsed | `7969047ea165c185881e2421be9e1b2a6ee0469bdea1decbb20883c8fea94879` |
| `lb02-real-900x600-light-properties-collapsed.png` | 900×600 | light | properties collapsed | `71499a189696a5fbace27653ccacc2b3834fb3be38c7dedcaf1267d265cfcebd` |
| `lb02-real-900x600-dark-properties-collapsed.png` | 900×600 | dark | properties collapsed | `86e0d374171896c3ad4fe34d06dd757404b1b9216fedf389a31db08caacdba6e` |

Regenerate with:

```
cd prototypes/mygo-run
LB02_EVIDENCE=1 go test ./internal/runworkspace -run TestLB02Evidence -count=1
```
