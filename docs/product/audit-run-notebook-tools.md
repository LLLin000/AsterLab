# Audit: Run, Notebook, Tools & Materials

Date: 2026-11-12. Scope: `internal/runworkspace`, `internal/runui`, `internal/notebook`, `internal/tools`, `internal/assets`, plus `main.go` wiring. No code changed. Viewports referenced: 1360×880 and 900×600.

Screenshots used: `prototypes/mygo-run/internal/runui/evidence-runui-900x600-light.png`, `evidence-runui-1200x760-light.png`, `evidence-notebook.png`, `evidence-gui-notebook-page.png`, `evidence-tools.png`, `internal/tools/evidence-tools.png`.

## 1. Page inventory

| Page | Entry point | Primary action | Secondary actions | States | Reads / writes |
|---|---|---|---|---|---|
| 实验运行 (Run) | `main.go:2577` (`shell.Run` case) → `runworkspace.View` at `main.go:2591` (fallback `runui.View` at `main.go:2594`) | Record a readout for the selected task and save checks (`runworkspace/recording.go:71-162`, `view.go:851-869`) | Select task in flow rail (`view.go:376-410`); confirm execution checks (`view.go:809-842`); attach/relocate file (`main.go:2472-2487`); complete task (`main.go:2419-2452`); open original plate (`runui.go:889-892`) | `runui.StateEmpty/Ready/Saved/Blocked/Error` (`main.go:285-294`, `1096-1100`, `1370`, `1383`, `1396-1398`); `runworkspace.State` sync (`view.go:127-149`) | Reads `runs.RunSnapshot`, frozen AsterDoc (`main.go:476`), assets (`main.go:354-377`). Writes measurements/checks/completion via Core callbacks (`main.go:2374-2452`); asset links (`main.go:418-471`) |
| Notebook | `main.go:2599` (`shell.Notebook` case) → `notebookPage` at `main.go:1779` | Write/edit Markdown note and save (`main.go:1860-1864`, `saveNotebook` `main.go:2107-2137`) | Switch mode 实时/编辑/阅读/文档/时间线 (`main.go:1849-1856`); format toolbar (`main.go:1823-1840`); insert plate module in 文档 mode (`main.go:1841-1846`); export PDF (`main.go:1803`); open original plate from timeline (`main.go:1980-1985`) | `noteMode 0-4` (`main.go:1853`); `noteDirty` (`main.go:1857-1859`); `noteErr` (`main.go:1867-1872`); noteRunID reload gate (`main.go:1789-1791`) | Reads `snap` (`notebook.Build` `main.go:285`, `1092`). Writes notebook Markdown + appends one notebook-stage event (`main.go:2107-2137`); never mutates Run events (`renderNotebookTimeline` `main.go:1945-1946`) |
| 资料与工具 (Tools) | `main.go:2601` (`shell.Tools` case) → `toolsPage` at `main.go:1756` | Edit a tool draft (plate/dilution/parameters) then hand off to Designer (`tools.go:253,273,292,312`) | Switch tool via left list (`tools.go:190-199`); import structured block (`tools.go:316-325`); browse component library (`tools.go:297-315`) | `Active Tool` (`tools.go:36-48`); `Error/Status` (`tools.go:214-218`) | Reads block definitions (`blocks.AllTypes()` `tools.go:300`). Writes in-memory drafts only until handoff (`tools.go:35`); handoff calls `Callbacks` into Designer (`tools.go:87-116`); no direct Run write |

## 2. Run vs Notebook division of labour

| Capability | Run (experiment execution) | Notebook (notes/timeline) | Can the other reach it? |
|---|---|---|---|
| Record a readout (measurements) | Yes — `recordingLayer` + `SaveReadout` (`main.go:2184-2290`, `view.go:851-869`) | No — no measurement input anywhere in `notebookPage` (`main.go:1779-1943`) | Notebook cannot |
| Complete a step | Yes — `CompleteTask` callback (`main.go:2322-2360`) | No | Notebook cannot |
| Read events (history) | Yes, task-scoped, collapsible (`view.go:871-945`), includes deviation/checks/materials/parameters/lineage/measurements (`view.go:897-939`) | Yes, run-scoped, read-only timeline (`main.go:1947-2006`), with per-event "打开原始孔板" (`main.go:1980-1985`) | Both read; Run is task-scoped, Notebook is run-scoped |
| Attach a file | Yes — `OnLinkFile` → `linkRunAsset` (`main.go:2484-2490`, `main.go:418-471`); gallery view (`recording.go:279-306`) | No attach action | Notebook cannot |
| Write prose | No prose editor in Run surface | Yes — 4 editable modes: live rich, source, reading, document (`main.go:1881-1941`) | Run cannot |
| Link evidence | Yes — asset link to task/sample (`assets.NewLink` `assets.go:162-206`); open batch plate (`runui.go:889-892`) | Read-only open of original plate from timeline (`main.go:1980-1985`); cannot create links | Notebook can only open, not create |

Decisive summary: Run owns facts (measurements, checks, completion, files); Notebook owns interpretation (prose, run-wide timeline view). Neither can do the other's primary job.

## 3. Usability defects (ranked)

1. **Primary action competes with metadata (Run).** The recording panel sits after procedure metadata, checks, stat chips, measurements preview, "下一步" banner, modules and frozen info (`view.go:606-727`), so "在下方实验记录中录入读数" (`view.go:700`) is below the fold at 900×600. Symptom: user scrolls a long procedure card before reaching the input. `view.go:600-869`.
2. **User cannot tell where to do X (Notebook modes).** Mode tabs are icon-only with terse labels 实时/编辑/阅读/文档/时间线 (`main.go:1851-1853`); save is silently disabled in modes 2 and 4 (`main.go:1861`) with only a dirty dot (`main.go:1857-1859`), no reason. Symptom: user does not know which tab writes prose vs reads events, or why Save is greyed. `main.go:1849-1864`.
3. **Primary action competes with metadata (Notebook toolbar).** The floating toolbar mixes undo/redo, 8 formatting commands, mode switch, dirty indicator and Save in one row (`main.go:1816-1865`); at narrow width the mode switch and Save compete for space. Symptom: Save can be pushed off-screen on 900×600. `main.go:1816-1865`.
4. **User cannot tell where to do X (Run history).** Operation history is collapsed by default (`view.go:872`, `ShowHistory` zero value `view.go:54-56`); no affordance indicates events exist. Symptom: evidence of recorded steps is hidden; user thinks nothing was recorded. `view.go:871-873`.
5. **User cannot tell where to do X (Tools handoff).** Every tool ends with "Use … in Designer / create draft" (`tools.go:253,273,292,312`) and drafts are in-memory only (`tools.go:35-48`), with no Run linkage or persistence. Symptom: user expects a tool to feed the current Run but it only creates a Designer draft; lost on navigation. `tools.go:87-116`, `tools.go:177-222`.

## 4. Layout options and recommendations

### Run (view.go 3-layer task workspace)

- Option A: Keep 3 layers stacked (procedure → recording → history collapsed) as now. Tradeoff: minimal change; recording stays below fold on small screens.
- Option B: Make recording the top layer, procedure collapsible below, history as tab. Tradeoff: fastest path to primary action; loses procedure-first reading order.
- Option C: Two-column at ≥1360 (procedure left, recording right), single column at 900. Tradeoff: best use of wide space; more layout code.
- Recommendation: Option B for ≤900, Option C for ≥1360 (responsive switch already exists `view.go:311-321` `shouldStack`).

### Notebook

- Option A: Keep 5 modes in one toolbar. Tradeoff: one surface; mode confusion remains.
- Option B: Split into two pages: "笔记" (prose modes) and "运行时间线" (read-only events), with timeline link from Run. Tradeoff: clearer intent; loses single-sheet feel.
- Option C: Keep one page but group modes into "写" (实时/编辑/阅读/文档) and "查" (时间线) with a segmented switch and inline help. Tradeoff: moderate; preserves Obsidian single sheet.
- Recommendation: Option B (split). Notebook currently mixes editable prose and a read-only event timeline over the same Run (`main.go:1945-1946`); splitting removes the mode-switch ambiguity and the disabled-Save surprise. Merge/split verdict: split the timeline out into a Run sub-view; keep Notebook prose-only.

### Tools (tools.go sidebar + draft)

- Option A: Keep sidebar tool list + handoff to Designer. Tradeoff: unchanged; no Run linkage.
- Option B: Add a "send to current Run task" handoff alongside Designer handoff. Tradeoff: requires new callback; better fit for execution workflow.
- Option C: Convert tools to a modal invoked from Designer/Run instead of a top-level page. Tradeoff: removes a page; less discoverability.
- Recommendation: Option B; keep page, add Run-task handoff, persist drafts to session.

## 5. Acceptance criteria

### Run at 1360×880
- [ ] Recording input (plate/table/form) is visible without scrolling past procedure metadata on first render (`view.go:851-869`).
- [ ] History layer can be expanded and shows at least one event with type, content and timestamp (`view.go:871-945`).
- [ ] Attach-file control is reachable from the recording panel and opens a dialog (`main.go:2477-2483`).

### Run at 900×600
- [ ] Layout stacks to single column (`view.go:311-321` `shouldStack`) with no horizontal overflow.
- [ ] Primary Save/Complete action remains visible while scrolling the recording panel (`recording.go:86-88` "Save lives in the header").

### Notebook at 1360×880
- [ ] Mode switch shows text labels (not icon-only) for all 5 modes (`main.go:1851-1853`).
- [ ] Save button state has a tooltip/reason when disabled (`main.go:1860-1864`).
- [ ] Timeline mode renders event list with "打开原始孔板" for events with BatchID (`main.go:1979-1986`).

### Notebook at 900×600
- [ ] Paper max width 820 does not overflow (`main.go:1875`, `1906`).
- [ ] Toolbar wraps or scrolls without hiding Save (`main.go:1816-1865`).

### Tools at 1360×880 / 900×600
- [ ] Left tool list (190px) does not collapse below 900px (`tools.go:188`).
- [ ] Handoff button label states destination (Designer vs Run task) (`tools.go:253` etc.).
- [ ] Error/Status message visible after a failed handoff (`tools.go:214-218`).
