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

The original screenshot set above was captured from
`prototypes/mygo-run/dist/mygo-run.exe` built at HEAD `82f90b3`. The new
wide-layout and citation-jump captures below use build commit `c55ae66`.

```text
AsterLab MyGo Prototype (UNSIGNED)
commit c55ae66
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 220E9C9CB0550C54F7E6C85EDDF698F04313FC8190C014BF070BAA94BB3BE238
mygo-run.exe SHA256 5E0F8D2BC32761A694B2B378FD2A4ED61BA190319ADBBAD9D39CA716A0348793
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

Isolated data `%TEMP%\lb03-gui\data`, Run `369ec46b-ab25-4820-8fb0-25b1b762de51`.
Seeded through CLI: project/protocol/design/AsterDoc creation, one real
`readout.submit` (OD450 0.62 AU, Control-1), one completion event, a local
`asset.link`, and Notebook observations containing event and file citations.
The file citation jump was exercised in the real window; no content was
fabricated.

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

## Real-window evidence addendum

Captured 2026-11-15 from launched `prototypes/mygo-run/dist/mygo-run.exe`.
The outer window was resized to 1360×880; the screenshot client area is
1344×871. Each image has an adjacent accessibility snapshot (`*.txt`).

Run `369ec46b-ab25-4820-8fb0-25b1b762de51` was seeded through the CLI with
one measurement task and one OD450 readout. CLI `asset.link` linked the local
file `linked-protocol-note.txt` (`asset-a9856d627eadfe8b70add39c21c3b576`,
SHA256 `b900d311046c76b8342ee28dcf16119d8eee24315a4fed3909847cecd7e6d379`);
the Notebook note cites it with `cite://asset/<id>`.

| Real-window image | Window size | Theme | What it shows | PNG SHA256 |
|---|---:|---|---|---|
| `lb03-real-notebook-dual-1360x880-light.png` | 1360×880 | light | Notebook prose and read-only timeline side by side | `87bce953bf4fd47260f244db41b14d922bb2779d01bd461a66eca9f07e62d5f3` |
| `lb03-real-citation-event-before-1360x880-light.png` | 1360×880 | light | before readout citation jump; `定位到运行记录` visible | `87bce953bf4fd47260f244db41b14d922bb2779d01bd461a66eca9f07e62d5f3` |
| `lb03-real-citation-event-after-1360x880-light.png` | 1360×880 | light | after readout citation jump; timeline row highlighted | `a916f05d2e5cf3f8107938906409f9edaba9dd1eb295c43a58aff009c4158333` |
| `lb03-real-citation-asset-before-1360x880-light.png` | 1360×880 | light | before local-file citation jump | `a916f05d2e5cf3f8107938906409f9edaba9dd1eb295c43a58aff009c4158333` |
| `lb03-real-citation-asset-after-1360x880-light.png` | 1360×880 | light | after local-file citation jump; task timeline row highlighted | `28a12b02f8c32bb974a18c259a55947bf77463495f271f99c440ce4bcf5a6e86` |

The initial Notebook and event-before captures are identical. The event-after
and asset-before captures are identical because both citations resolve to the
same task row; the asset-after capture shows that row highlighted after its jump.

## Artifact hashes (SHA256)

```text
5b26fa8e67297d3966c0e72777efc09f8bf5f884c726fd42420100ed78c00cb1  lb03-citation-after-900x600-light.png
0f279058e14c2e2d5778e246ac04839bcc477aaf24814d24779162b8c5f1abdd  lb03-citation-before-900x600-light.png
e3dbcd6b3da5299681f2e49e4e81c4eb73c2c7adffa68f07b7cdfbd6685c2c89  lb03-notebook-dual-1360x880-light.png
0f279058e14c2e2d5778e246ac04839bcc477aaf24814d24779162b8c5f1abdd  lb03-notebook-switch-900x600-light.png
20014d9917b84efce59e247531e1d34cb17332b7c459100bf978c9697a2515d5  lb03-notebook-timeline-900x600-light.png
```

The citation "before" shot is the same prose view as the 900×600 switch shot — the jump starts from that state, so the two captures are byte-identical; both are kept because they document different moments (tab switch vs pre-jump).
