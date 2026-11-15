# File ownership and wave sequencing

Purpose: make parallel work impossible to conflict. One production file has one
writer at a time; routing files belong to the Integration Owner only.

## 1. Why the previous plan conflicted

The wave plan named slices, not files. In practice:

- 1A (Protocol page) and 1C (project/tools) both needed `internal/shell/shell.go`.
- 1A and 1B both needed `main.go` (page routing and page bodies live in the same file).
- 1B (Notebook split) and 2A (Run recording) both needed `internal/runworkspace`.

Two writers on one file in parallel worktrees always ends in a manual merge.

## 2. Rule

1. **Integration Owner owns navigation and routing.** Only it edits
   `internal/shell/shell.go`, the page-routing part of `main.go`, and
   `internal/state`.
2. **Page bodies are extracted before they are owned.** Wave 1 is a mechanical
   extraction with no behaviour change:
   - `notebookPage` → `notebook_page.go`
   - `protocolPage` → `protocol_page.go`
   - `designPage`/`asterDesignPage` → `design_page.go`
   - run page wiring → `run_page.go`
   `main.go` keeps only construction and routing.
3. After extraction, each owner edits only its own files. A change that needs a
   navigation or routing change is a request to the Integration Owner, not an
   edit.
4. Cross-page state (selected project, design, run, task, unsaved drafts) is
   defined once by the Integration Owner in `internal/state`; pages read it and
   emit intents, they never own it.

## 3. Ownership map

| Owner | Exclusive files | May request, never edit |
|---|---|---|
| Integration | `internal/shell/shell.go`, `main.go` (routing/construction), `internal/state/*`, `docs/product/ownership.md` | — |
| Run | `internal/runworkspace/*`, `internal/runui/*`, `run_page.go`, `run_recording_page.go` | `internal/state` (add a field), routing |
| Design | `design_canvas.go`, `design_page.go`, `internal/designer/*` | `internal/state`, routing |
| Notebook / Protocol | `internal/notebook/*`, `internal/protocol/*`, `notebook_page.go`, `protocol_page.go` | `internal/state`, routing, `internal/runworkspace` |
| SDK | `internal/asterdoc/spec/modules.json`, `internal/asterdoc/{spec,modules,registry,view}.go`, `src/cli.rs`, `docs/sdk/*` | `internal/state`, routing, page files |

`src/cli.rs` stays single-writer (SDK Owner). A page that needs a new Core
command requests it; it does not add one.

## 4. Sequencing

**Wave 0.5 — environment gate (blocks everything).** Disk report, protected-path
list, build-cache relocation decision. No parallel builds until free space is
adequate.

**Wave 1 — Integration only (serial, one owner).**
1. Extract page bodies (mechanical, tests unchanged).
2. Add Protocol as surface 6 and route it.
3. Remove the project-workspace design cap.
4. Define cross-page context in `internal/state`.
Exit: navigation works, Protocol reachable, no page owner has started.

**Wave 2 — three owners in parallel.** Run, Design, Notebook/Protocol, each on
its own files. Notebook removes its timeline; Run adds the history tab in the
same wave but in `internal/runworkspace` (Run Owner), so no file is shared.

**Wave 3 — SDK Owner, serial.** Tooling, composite-field editing, static
renderer, parity fixture. Then one real module (RNA reagent calculator) with the
change-impact table.

**Wave 4 — Integration.** Full suites, native module check, CCK-8 GUI E2E, RNA
multi-step Run, six-page screenshots at 1360×880 and 900×600 in both themes,
restart and unsaved-state recovery.

## 5. Merge discipline

- One logical commit per slice, conventional prefix (`feat`, `fix`, `docs`,
  `refactor`).
- A slice is mergeable only when its own narrow check passes; the full suite runs
  once at Wave 4.
- No slice reformats files it does not own.
- No slice changes `sdk-v1-rc3` or rewrites a frozen Run.

## 6. Windows Gate

Windows-host concurrency, ovnfs checkout isolation, and GUI/E2E sequencing
are defined in
[`cross-platform-builds.md` §10](../engineering/cross-platform-builds.md#10-windows-gate-task-execution).
The one-writer-per-production-file rule in this document remains binding:
overlapping write scopes serialize regardless of the Kandev WIP cap.
