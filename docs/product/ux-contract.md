# AsterLab Product UX Contract v1

Status: binding for all UI work from 2026-11-12. A UI change that cannot cite a
rule id below is not reviewable. This contract governs product behaviour; it is
deliberately separate from the module SDK manifest.

## 1. Information architecture

Obsidian-grade single-vault local experiment workbench organizing a minimalist `40px` left **Ribbon** + transparent Vault **File Tree** (`.asterlab/`, `assets/`, `templates/`, `<ProjectFolder>/*.asterml`) alongside a recursive **Peer Split-Pane Tree** (`PaneNode` hosting `TabCalendar`, `TabFile`, `TabPantry`), Back/Forward history, a 24px **Desktop Status Bar**, a slim collapsible 3-tab **Right Companion Drawer** (`步骤大纲` | `速算与计时` | `关联与诊断`), `Ctrl+K/O` floating Command Palette, and `Ctrl+Shift+F` **Vault Global Search Modal**.

| # | Surface | One job | Primary action | Does not belong here |
|---|---|---|---|---|
| 1 | 实验日历（`TabCalendar`） | Orthogonal date view (`月` / `周`) over project folders via frontmatter `date` + drag-to-schedule & day Popover | ＋ 新建实验 / double-click empty day / click day item | Editing module tables, 4-tool clutter |
| 2 | 实验文档（`TabFile` · `编辑 \| 阅读 \| 源码`） | Single-source-of-truth `.asterml` Live-Preview workbench where Design & inline Execution progress live in one file | Inline edit/execute (`checklist`, `timer`, `param_table`, `plate`, `calculator`, `data_table`, `figure`) + `⋯` Export | Separate frozen `运行`/`记录` copies, manual Save blockers |
| 3 | 储藏室货架（`TabPantry` · `tools.ViewPantry`） | Standalone reagent shelf, BioRender box/rack maps (`A1..L12`), lot/expiry & manuals | Quick locate from `Ctrl+K` or `param_table` coordinate badge | Nested 4-tool switcher chrome (`Plate Studio`/`Bench Calc`/`SOP Library`) |

Decisions taken:

- **Obsidian Vault & Peer Split-Pane Shell (`internal/shell/` & `internal/vault/`)**:
  - **40px Ribbon + File Tree (`file_tree.go`)**: hosts `实验日历`, `储藏室货架`, `+ 新建实验`, `+ 新建项目文件夹`, `库设置`, and a true-filesystem tree (`F2` inline rename, drag-to-move/split/schedule, full native `ContextMenu`). Project ownership = folder path; schedule = YAML frontmatter `date`.
  - **Peer Split-Pane Tree (`tabs.go` & `shell.go`)**: every leaf pane is equal; supports 4-way edge drag-to-split (`left`/`right`/`up`/`down`), middle-click tab close, opening the same `.asterml` file side-by-side in `编辑` and `阅读` sharing one `*vault.LiveDocument` with unified 100-step `Ctrl+Z/Y` Undo/Redo, plus `workspace.json` session restore.
  - **Motion System (`internal/theme/motion.go`)**: frequency-budgeted 4-tier springs (`MotionMicro`, `MotionSnappy`, `MotionFluid`, `MotionHero`), `NumericRoll`, `FloatingBarTransition`, `LiquidSplitTransition`, `HeroFlyTransition`, `Oklab` color interpolation, and `ApplyNudge` sponge shake.
  - **UnifiedLive Document Canvas (`experiment_page.go` & `internal/asterdoc/`)**: strips frontmatter into `# Title` + 1-line property pills (`📅 日期` / `📁 项目` / `状态`), renders frameless `prose` & `checklist`/`timer` interactive in both `编辑` and `阅读`, reveals hover `⋮⋮` block grips + `+` insert gaps (`/` slash menu), layer-pill `plate@1` (`[分组]` + layers + `单值 | 平铺列表 | 等比序列` + right-click well ops), `data_table@1` header `Popover`, `figure@1` `Ctrl+V` clipboard PNG paste into `./assets/`, and `param_table@1` inline `记偏离` deviation input.
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
| `设计中` | design missing objective/steps/modules/title | warning pill naming exact missing gap |
| `方案就绪` | objective completeness rules pass | success pill; quick `开始实验` enabled |
| `实时保存` / `实时已保存` | Obsidian-style instant sync active & persisted | quiet metadata / success pill; no manual Save |
| `未保存` | offline/failed write with local draft preserved | warning pill + error cause + retry |
| `校验中` | Core assessment running | disable Create Run, keep content readable |
| `进行中` / `已完成` / `未开始` / `阻塞` | frozen task or run status | task row, calendar chip & header pill |
| `未解析` | a required binding is missing | warning; never silently substituted |
| `已冻结` / `方案快照已锁定` | immutable snapshot | lock affordance; no plan edit controls rendered |
| `记录已锁定` | signed/sealed final run notes | opens in `阅读`; edit modes locked |

`组织节点` (organizational step) is a node type, not a task state: it renders
without a status dot and without counts.

## 4. Layout contract

Breakpoints: **≥1360 wide**, **1100–1359 medium**, **<1100 narrow** (covers
900×600). Layout must be correct at 1360×880 and 900×600; other sizes are
derived.

- **Obsidian Shell (`internal/shell/`):** 40px Ribbon + Vault File Tree (`file_tree.go`), peer `PaneNode` split tree with per-pane tab bars & 4-way edge drag-to-split overlays (`LiquidSplitTransition`), Back/Forward chrome + `Ctrl+O/K` palette + `Ctrl+Shift+F` Vault Global Search, 24px bottom Desktop Status Bar, and slim 3-tab Right Companion Drawer (`步骤大纲`, `速算与计时`, `关联与诊断`).
- **Experiment Calendar (`实验日程`):** minimal top bar (`YYYY 年 MM 月`, `‹ 今天 ›`, `月 | 周`, search, `＋ 新建实验`), seamless month cells or `08:00–20:00` Week time grid, circular Today pill, drag-to-schedule/reschedule drop targets, double-click cell create dialog (project folder + template + date), and day Popover.
- **UnifiedLive Experiment Canvas (`experiment_page.go`):** 1-row breadcrumb header (`项目 / 标题` + Read/Edit toggle + `Ctrl+F` in-page find + copy + `⋯`), unframed `# Title` + property pills (`日期` / `项目` / `状态`), Live-Preview step stream with collapsible step headers, hover `⋮⋮` drag handles, `+` insert gaps, `/` slash menu, and collapsible bottom `科学分析汇总 (AsterCore)` projection card; `checklist`, `subflow` & `timer` remain interactive in both `编辑` and `阅读` modes, persisting directly to `.asterml` alongside File Recovery snapshots (`.asterlab/snapshots/`).
- **Standalone Pantry (`tools.ViewPantry`):** renders directly inside `TabPantry` without legacy 4-tool switcher chrome.

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

## 8. Design system

`docs/product/design-system.md` is the enforceable visual contract behind P3: the
single token source, the 4-multiple spacing scale, the three radii, the
requirement→component table (Do/Don't) and the copy rules. A UI task must read it
before writing code.

Binding on every UI task:

- Generic colors, radii, and control heights come from `Tokens`; typography
  uses text tokens or theme-scaled `t.Rem`, and spacing uses spacing tokens or
  `t.Space`. Scientific semantic colors may only use named palettes registered
  in `docs/product/design-system.md` §1; call sites must not contain literals.
  Violations fail P3.
- Pick the component from the table first; restyle later, never the reverse.
- A new component is allowed only when neither `oneo-me/onevra/source/components`
  nor an existing package has an equivalent, and it must be registered in the
  table with a reason.
- The PR carries a `## 设计一致性` section: tokens used, components reused, new
  components (if any) and why. `UX-DESIGN` is the rule id for these items.

## 9. Obsidian Settings, Context Menus & Mobile Adaptive Layout

- **Settings (`Ctrl+,` / gear icon, `internal/shell/settings.go`)**: Built on `onevra/components.Settings` with real-time search across 6 categories (`实验库与文件`、`外观与主题`、`文档与实验默认`、`快捷键`、`移动端与触控适配`、`关于实验库与数据`), persisting theme preset (`obsidian` default / `zinc` / `warm`), `LiquidGlass` toggle, mobile pull-down action, auto-hide dock, and edit toolbar preferences to `<Vault>/.asterlab/config.json`.
- **Context-Menu-First Ergonomics (`UX-CLEAN-UI`)**: Secondary operations (file tree rename/move/duplicate/template/delete, pane split/close/pin, calendar quick-create, step block cut/copy/paste/reorder, plate invert/edge-select/outlier-mark/export CSV&PNG) live in native `components.ContextMenu` popups rather than cluttering rows with permanent buttons.
- **Obsidian iOS 26 Mobile Shell & Dual-Mode Liquid Glass (`internal/shell/mobile.go`, `internal/theme/theme.go`)**: Below `680dp` viewport width or on `android`/`ios` (or `ForceMobilePreview`), collapses to single-pane focus with open top center (`ScrollEdgeSoft` top feather, `40dp` circular drawer orb on left, twin-icon mode/`⋯` glass pill on right), borderless `Tok.Elevated` cards (`实验属性`, compact `实验分组`, day agenda), centered icon-only Liquid Glass bottom capsule (`<` · `>` · `🔍` · `+` · `[N]` · `≡`) that auto-hides on downward scroll and morphs into an icon-only Edit Toolbar in `ModeEdit`, 2-column Tab Switcher card grid, pull-down Quick Action, and left/right edge-swipe `Dragged()`/`DragVelocity()` drawers with Obsidian's 3-tier bottom control island. Light glass uses milky `glass.Regular` (76%); Dark glass prevents Glass-on-Glass washout and uses smoked `glass.Clear` (84%) + `rgba(255,255,255,0.14)` specular rim.
