# AsterLab Product UX Contract v1

Status: binding for all UI work from 2026-11-12. A UI change that cannot cite a
rule id below is not reviewable. This contract governs product behaviour; it is
deliberately separate from the module SDK manifest.

## 1. Information architecture

Six surfaces. Each surface has exactly one job, one primary action, and an
explicit list of what does not belong there.

| # | Surface | One job | Primary action | Does not belong here |
|---|---|---|---|---|
| 1 | 项目工作区 | Find the experiment to continue | Continue the last task / open a design | Editing parameters, recording results, reading protocol text |
| 2 | 实验设计 | Author and understand the plan | Create Run (after preflight) | Recording readouts, writing prose notes, browsing assets |
| 3 | 实验运行 | Execute a step and record facts | Record the readout / complete the step | Authoring the plan, writing long-form prose |
| 4 | Notebook | Interpret and narrate | Write and save prose | Recording measurements, completing steps, attaching execution files |
| 5 | 实验方案 (Protocol) | Read the frozen plan as a document | Read a step, export the frozen document | Editing anything |
| 6 | 资料与工具 | Prepare reusable material | Hand a prepared draft to Design or Run | Owning experiment state |

Decisions taken:

- **Protocol becomes surface 5** (`internal/shell/shell.go:115` currently lists
  five pages; `protocolPage` is buried under Tools, `main.go:1761`). It is a
  read-only projection of the frozen plan, so it must never gain an edit action.
- **Notebook keeps prose only.** The run-wide event timeline moves to Run as a
  second tab (`run / history`), because Run already owns the authoritative
  events (`internal/notebook` reads the same `runs.RunSnapshot`). Notebook must
  not present a second, differently-shaped history.
- **Tools stays a page** and gains a second handoff: "send to current Run task"
  in addition to "send to Designer". Drafts persist for the session.
- **Project workspace never hides designs.** The `6 - len(inProgress)` cap
  (`internal/shell/shell.go:331-337`) is removed.

Write boundaries (no exceptions):

- Facts (measurements, checks, completion, asset links) are written only from
  Run through Core commands.
- Prose is written only from Notebook.
- Plan structure and module values are written only from Design.
- Protocol and the Run history are read-only.

### 1.1 Protocol: frozen vs preview

Protocol renders two different objects and must never conflate them:

- **已冻结** — a Run exists; the text is the exact frozen AsterDoc revision. Show
  the Run id and the frozen revision.
- **预览** — no Run yet; the text is a read-only projection of an explicit
  Design revision. Show the design revision and the word `预览`, never `冻结`.

A preview must not be exportable as a frozen record, and a frozen document must
not silently follow later design edits.

### 1.2 Notebook: prose may cite facts, never create them

Notebook writes interpretation. It may reference existing Run events, files,
measurement ids and sample ids, and those references must resolve to the
authoritative record. It must not provide any control that creates a
measurement, completes a step, or attaches an execution file. A note that cites
a fact is a citation, not a second copy of the fact.

### 1.3 Tools → Run: drafts, not facts

"Send to current Run task" sends a structured draft or suggestion. The Run
surface shows it as unconfirmed; nothing is written until the operator confirms
it there, and the write goes through the same Core command as manual entry.
Tools never writes execution facts directly.

## 2. Enforceable UX principles

Each principle has a test that fails the change, not a slogan.

**P1 Task first.** Exactly one primary action is visible per surface without
scrolling at 900×600. *Test:* at 900×600 the primary control is in the initial
viewport; everything else may be below the fold.

**P2 Progressive disclosure.** Default view shows only what is needed to decide
the next action. Parameters, versions, block ids and provenance live in the
inspector, a detail section, or an expanded row. *Test:* no raw identifier
(`uuid`, `step-id`, `block-version-id`) is visible in a default view; each is
reachable in ≤1 click.

**P3 One component language.** Use existing Onevra/MyGo components and the
theme tokens. No new visual vocabulary per page. Scientific widgets (plate,
workflow canvas, result grid) may be specialised but must share spacing, type
scale, state colours and focus behaviour. *Test:* the page uses no literal
colour or font size outside the theme; new widgets reuse `components.*`.

**P4 Content decides layout.** A plate is a matrix, a workflow is a graph, a
record is a table, a protocol is a document. Do not wrap everything in the same
card. *Test:* the surface's main region is the shape of its content, not a
stack of cards.

**P5 State is explicit and recoverable.** Every surface names the state in the
same words, and every failed or unsaved state offers a way back. *Test:* the
state word comes from §3; a failure shows cause + retry; navigation and restart
do not silently discard a draft.

## 3. State vocabulary

One word per state, used identically in every surface:

| State | Meaning | Surface treatment |
|---|---|---|
| `草稿` | editable, not yet valid | neutral; primary action enabled only if the surface can validate locally |
| `未保存` | user changed something | warning dot + explicit Save; never auto-discard |
| `已保存` | persisted, no pending change | success, transient |
| `校验中` | Core assessment running | disable Create Run, keep content readable |
| `不完整` | Core assessment returned errors | danger banner listing each error, each pointing at the offending step/module |
| `待确认` | warnings require acknowledgement | warning banner + explicit acknowledgement before Run creation |
| `就绪` | no errors, no warnings | success |
| `未开始` / `进行中` / `已完成` / `阻塞` | frozen task status | task row + rail |
| `未解析` | a required binding is missing | warning; never silently substituted |
| `已冻结` | immutable snapshot | lock affordance; no edit controls rendered |

`组织节点` (organizational step) is a node type, not a task state: it renders
without a status dot and without counts.

## 4. Layout contract

Breakpoints: **≥1360 wide**, **1100–1359 medium**, **<1100 narrow** (covers
900×600). Layout must be correct at 1360×880 and 900×600; other sizes are
derived.

- **Design:** persistent right inspector (≥320px) at ≥1100; bottom sheet at
  <1100. The inspector never overlays the primary action.
- **Run:** recording panel is the first content block at ≤900; two columns
  (procedure | recording) at ≥1360. Procedure metadata collapses by default
  below the recording panel.
- **Protocol:** step list (224px) + detail side by side; document mode max-width
  820 centered.
- **Notebook:** prose paper max-width 820; toolbar wraps rather than hides Save.
- **Tools:** left list 190px, never collapses.
- **Project workspace:** grid without a hard item cap; explicit "show all".

Primary-action budget: one accent-filled control per surface. Everything else is
secondary or icon-with-tooltip, and every icon-only control has a text tooltip.

## 5. Required input for any UI task

A UI task is not actionable until all five are present:

1. Real-window screenshot of the current surface at 1360×880 and 900×600.
2. The surface's job, quoted from §1.
3. The user scenario ("who is doing what, coming from where, going where").
4. The component and layout constraints from §2 and §4.
5. Acceptance checks from §6.

An agent must propose 2–3 layout options with tradeoffs and a recommendation
before writing production code. Direct audit→code is not allowed.

## 6. Definition of done for a page

A page change is done only when all hold:

- [ ] Primary action visible without scrolling at 900×600 (P1).
- [ ] No raw identifier in the default view (P2).
- [ ] No literal colour/size outside the theme (P3).
- [ ] Main region matches content shape (P4).
- [ ] Every state uses §3 vocabulary; failure has cause + retry (P5).
- [ ] Draft survives navigation away and back.
- [ ] 1360×880 and 900×600 screenshots attached to the change.
- [ ] Existing page-level tests still pass; new behaviour has one focused test.

## 7. Rule ids

Cite as `UX-IA-<n>`, `UX-P<1-5>`, `UX-STATE`, `UX-LAYOUT`, `UX-DONE`. Example:
"Run recording moved above procedure — UX-P1, UX-LAYOUT".
