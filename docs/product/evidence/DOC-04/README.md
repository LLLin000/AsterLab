# DOC-04 Evidence — Notebook @Mention Catalog, Citations & Staleness

## Baseline & Scope
- Repository: `LLLin000/AsterLab`, base `origin/main`, isolated `ovnfs` checkout `O:\doc04`.
- Scope files modified:
  - `prototypes/mygo-run/internal/notebook/citations.go`
  - `prototypes/mygo-run/internal/notebook/citations_test.go`
  - `prototypes/mygo-run/notebook_page.go`
- Zero Core write-path or schema changes (`crates/` untouched).

## Verification
1. Go unit & full suite tests (`go test ./... -count=1`): 21 packages passed, 0 failed.
   - `TestCitationInsertionAndSnapshotResolution`
   - `TestCitationStalenessAndMissingAssets`
   - `TestRemoveCitationOnlyChangesMarkdown`
   - `TestAtMentionTriggerCatalogAndRichtextRoundTrip`
2. Real Windows GUI verification (`dist/mygo-run.exe` + `Runs.dll` + `labworkbench.exe`):
   - Typed `实验总结 @` in live Notebook editor -> hierarchical `@` dropdown opened automatically showing `项目 · DOC-04 Notebook 引用验收`, category filter tabs (`全部 / 板图 / 统计结果 / 总结语句 / 步骤板块 / 文件 / 读数`), and grouped entries (`统计结果`, `总结语句 / 事件`, `步骤板块`, `关联文件`, `读数明细`).
   - Inserted event, asset, and readout citations as Markdown-safe `[@...](cite://...)` tokens (`doc04-citations-1360x880-light.png`) and saved the notebook.
   - Mutated underlying measurement (`0.82 -> 0.91`) and unlinked `plate-raw.csv` in Core -> restarted GUI and confirmed `已过期` / `已过期 / 原记录不可用` warnings (`doc04-stale-1360x880-dark.png`, `doc04-stale-900x600-light.png`).
   - Clicked `移除引用` and saved -> confirmed citation token removed from Markdown while Core events (4) and measurements (2) remained intact (`doc04-removed-900x600-light.png`, `doc04-900x600-dark.png`).

## Screenshots
- `doc04-at-dropdown-1360x880-light.png`
- `doc04-citations-1360x880-light.png`
- `doc04-stale-1360x880-dark.png`
- `doc04-stale-900x600-light.png`
- `doc04-removed-900x600-light.png`
- `doc04-900x600-dark.png`
