# AG-04 Evidence — MyGo Agent Proposal Preview UI

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, base `origin/main` (`e2162bb`), isolated `ovnfs` checkout `O:\ag04`.
- Scope files modified:
  - `prototypes/mygo-run/internal/agentedit/client.go`
  - `prototypes/mygo-run/internal/agentedit/view.go`
  - `prototypes/mygo-run/internal/agentedit/agentedit_test.go`
  - `prototypes/mygo-run/design_page.go`
  - `docs/product/evidence/AG-04/`

## UX Contract Rule Citations
- `UX-P1` (Task first): `Agent 提案` remains `components.ButtonSecondary` in `design_page.go`, secondary to `创建 Run`.
- `UX-P2` (Progressive disclosure): Field-level diffs are grouped by `path` inside a scrollable review dialog (`params.volume_per_sample`, `params.sample_count`), showing declared `before`, Core authoritative `current`, and `requested` values plus `confirmation_prompt`.
- `UX-P3` (One component language): Built with `components.Dialog`, `components.Button`, `components.Checkbox`, `ui.TextInput`, `ui.Scroll`, and `c.Theme()` tokens.
- `UX-P5` (Explicit, recoverable state): Reject (`拒绝`) closes the dialog and makes zero write calls; blocked/conflict states surface exact `AG-01` error codes (`VERSION_CONFLICT`, `UNIT_MISMATCH`, `INVALID_PROPOSAL`, etc.) and disable `接受并提交`.
- `UX-LAYOUT` / `UX-DONE`: Verified at `1360×880` and `900×600` in both light and dark themes.

## Automated Checks
- `cd prototypes/mygo-run && go test ./internal/agentedit -v` → exit 0 (`TestGroupedDiffAndRejectNoCommit`, `TestBlockedCodesAreSurfaced`, `TestViewRendersWideAndNarrow`)
- `cd prototypes/mygo-run && go test ./... -count=1` → exit 0 (`21 packages ok, 4 no tests`)

## Real Windows GUI Walkthrough (Isolated Data Dir)
Using built `dist/mygo-run.exe` + `target/debug/labworkbench.exe` against isolated `labworkbench.sqlite`:
1. **Conflict (`VERSION_CONFLICT`)**: Loaded `proposal-conflict.json` (`expected_version: 99`) → `校验提案` surfaced `VERSION_CONFLICT` and kept `接受并提交` disabled (`ag04-conflict-1360x880-light.png`).
2. **Unit Mismatch (`UNIT_MISMATCH`)**: Loaded `proposal-unit-err.json` (`unit: mL` on `uL` slot) → surfaced `blocking · UNIT_MISMATCH · params.volume_per_sample.unit` (`ag04-preview-900x600-dark.png`).
3. **Grouped Diff Preview & Reject (0 writes)**: Loaded `proposal-valid.json` → displayed `params.volume_per_sample` (`200 uL -> 250 uL`) and `params.sample_count` (`4 -> 6`) grouped by path (`ag04-preview-1360x880-light.png`, `ag04-preview-1360x880-dark.png`, `ag04-preview-900x600-light.png`). Clicked `拒绝` → dialog closed, DB SHA-256 unchanged:
   - Before reject: `C54DA0C7EF2453E3971B2AA0B21E712F12468BDE2CCA12A274396129535F2551`
   - After reject: `C54DA0C7EF2453E3971B2AA0B21E712F12468BDE2CCA12A274396129535F2551`
4. **Accept & Commit (`agent_edit.commit`)**: Clicked `接受并提交` → surfaced `已保存 · new_version_id: a657bf07-badb-40e8-ab9b-5d9fd690877d · new_revision: 2` (`ag04-committed-900x600-light.png`); `labworkbench record.list` confirmed immutable Protocol v1 (`80594501-...`, `200 uL / 4`) preserved alongside new Protocol v2 (`a657bf07-...`, `250 uL / 6`).
