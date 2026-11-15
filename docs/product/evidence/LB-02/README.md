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

`prototypes/mygo-run/dist/BUILD_INFO.txt` produced by `scripts/mygo-build.ps1`:

```
AsterLab MyGo Prototype (UNSIGNED)
commit f624c8a
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 1BB5B9C4E13103F617412511D6F457CD3B5F3EF9EA9D7D2F4F0E910D8420182C
mygo-run.exe SHA256 C93CF8A9853C243AC5E40FA94C18F847170AB5C92ED1CC400ABB94DEBDE7C0E0
```

`git rev-parse --short HEAD` at capture time was `f624c8a`, so the build stamp equals HEAD.

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

Regenerate with:

```
cd prototypes/mygo-run
LB02_EVIDENCE=1 go test ./internal/runworkspace -run TestLB02Evidence -count=1
```
