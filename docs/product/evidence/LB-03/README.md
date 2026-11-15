# LB-03 — Notebook: prose paper + read-only Run timeline from one source

Card `707f834d-9c05-480a-a660-6d9b40a660f4`, lane Windows Gate, V2 track.
Rule ids: `UX-IA-4`, `UX-06`/`UX-STATE`, `UX-P1`, `UX-LAYOUT`, `UX-DONE`.

## What changed

`UI-REDESIGN §2.3` asks the Record block to render two projections of one
source. The Notebook now does exactly that:

- **Prose paper** (left / first tab) — the editable interpretation, max-width
  820, unchanged modes 实时 · 编辑 · 阅读 · 文档.
- **运行记录** (right / second tab) — a read-only timeline built by
  `notebook.Build` from the *same* `RunSnapshot`. One row per stored entry, in
  recorded order, each showing its stored time, its source (`execution_event <id>`) and
  its trace state. No third history store and no fact-writing control.

An `@` citation that names a stored event now resolves to the timeline entry it
projects (`notebook.CitationTargetOf`) and can jump there (**定位到运行记录**); the
entry is highlighted and scrolled into view. A citation carries identity only —
it is a pointer into the authoritative record, never a second copy of it.

The toolbar names the save state in the contract's words — `未保存` / `已保存`,
via `notebook.SaveStateLabel` — instead of a bare colour dot (`UX-06`).

Layout (`UX-P1`/`UX-LAYOUT`): at ≥1100 the two projections sit side by side;
below 1100 (covers 900×600) they **switch** as two tabs, so the prose keeps a
readable measure and Save stays in the first viewport.

## Build stamp

All screenshots come from `prototypes/mygo-run/dist/mygo-run.exe` built at
repository HEAD `34489e0` (`BUILD_INFO.txt` → `commit 34489e0`, matching
`git rev-parse --short HEAD`).

```text
AsterLab MyGo Prototype (UNSIGNED)
commit 34489e0
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 3C0E3C3C8FC96CE5B54346D2E2BBCA846A418EF4CB92EBC28614DA19DE48DE91
mygo-run.exe SHA256 2891EEE9842AF721E847B54BBF902F2AB6FCAF666844959E2F701E0668CC58C3
```

## Evidence (real application windows, light, isolated data)

| Shot | What it shows | Rule id |
|---|---|---|
| [1360×880](lb03-notebook-dual-1360x880-light.png) | prose paper and 运行记录 side by side, 4 timeline entries, `只读` badge | UX-IA-4, UX-LAYOUT |
| [900×600 · 笔记](lb03-notebook-switch-900x600-light.png) | narrow: sections switch as tabs; Save in the first viewport | UX-P1, UX-LAYOUT |
| [900×600 · 运行记录](lb03-notebook-timeline-900x600-light.png) | switched timeline projection | UX-IA-4 |
| [citation before](lb03-citation-before-900x600-light.png) | prose with `@OD450 读数` citation and 定位到运行记录 | UX-IA-4 |
| [citation after](lb03-citation-after-900x600-light.png) | jumped to and highlighted 读数提交：OD450 | UX-IA-4 |

Raw accessibility snapshots sit beside each PNG (`*.txt`), so the exact
window text is auditable without re-running the app.

## Seed

Isolated data `%TEMP%\lb03-gui\data`, Run `a2dad20c-c504-44b9-9443-a2871e69bbae`.
Seeded through the CLI only: `project.create` → `protocol.block.create` /
`protocol.create` → `design.create` → `asterdoc.save` / `asterdoc.create_run` →
one real `readout.submit` (OD450 0.62 AU, Control-1) → one completion
`execution_event.create` → one notebook-stage observation carrying a real
`cite://event/<id>` token. Nothing was fabricated: the timeline entries are
the stored events.

## Tests

`go test ./... -count=1` → exit 0. New coverage:

- `internal/notebook`:
  - `TestTimelineProjectsEventsInRecordedOrder` — the projection lists stored
    events in recorded order.
  - `TestCitationResolvesToTimelineEntryId` — an event/batch/step citation
    resolves to a projected entry id; an absent one does not.
  - `TestCitationTargetCopiesNoContent` — resolving changes no projection and
    leaves exactly one entry per event.
  - `Entry.DisplayTime` — one place words the stored/absent time.
- `notebook_page_test.go`: wide shows both projections; narrow switches and
  starts on prose; state word is explicit; a citation jump sets the
  highlight.

## NOT VERIFIED

- **Citation jump from the raw `@` popover on the wide layout** — the seeded
  note already carried the citation, so the captured jump is from the 引用快照
  action at 900×600. The wide 定位到运行记录 path renders the same control but
  was not separately photographed.
- **Asset citation jump** — `CitationTargetOf` maps a file asset to the task
  entry it is linked to; no seeded file asset was linked, so that branch has
  unit-level coverage only, no screenshot.

## Artifact hashes (SHA256)

```text
cdcb799791f6122b99c32a9839e84974ba59509fa583152bce806d445aefa01f  lb03-citation-after-900x600-light.png
eea1b8b20fffb3c966655f4fbcd9efdcb833af1e8371eac4dec3376edb48622a  lb03-citation-before-900x600-light.png
e0361c88ce2ccb572e70e8200fc315ef32eb76fc2fd407c21fcb87ed27a822e5  lb03-notebook-dual-1360x880-light.png
eea1b8b20fffb3c966655f4fbcd9efdcb833af1e8371eac4dec3376edb48622a  lb03-notebook-switch-900x600-light.png
fdae423602cde9bff841e0d4b0a8c037d1ca8cbc57336b3df361d21c2803e234  lb03-notebook-timeline-900x600-light.png
```

The citation "before" shot is the same prose view as the 900×600 switch shot — the jump starts from that state, so the two captures are byte-identical; both are kept because they document different moments (tab switch vs pre-jump).
