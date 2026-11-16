# UX-02 — Recoverable empty and error states

Required capture format: real desktop window, light theme, 1360×880 and 900×600. Seed each state in an isolated data directory; do not fabricate records to make an empty state look populated.

| State | What the user sees | Next action | Rule id | Real-GUI evidence |
|---|---|---|---|---|
| 未开始 — no project | Empty workspace says “未开始 · 尚无项目” | 新建设计 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-no-project-1360x880-light.png) · [900×600](ux02-no-project-900x600-light.png) |
| 未开始 — project has no designs/runs | Empty project says “未开始 · 还没有实验设计” | 新建设计 or 导入设计 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-empty-project-1360x880-light.png) · [900×600](ux02-empty-project-900x600-light.png) |
| 未解析 — linked asset unavailable | Original Run asset path is retained and marked unavailable | 重新定位 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-unavailable-asset-1360x880-light.png) · [900×600](ux02-unavailable-asset-900x600-light.png) |
| 关联失败 — Core rejects a stale Run | Persistent banner retains the Core reason when a loaded Run disappears | 重新关联 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-core-rejected-run-1360x880-light.png) · [900×600](ux02-core-rejected-run-900x600-light.png) |

## Seed and failure record

- No-project: empty `LABWORKBENCH_DATA_DIR`.
- Empty project: `project.create` seeded project `UX-02 Empty Project`, with no designs or Runs.
- Missing asset: created a real Run through `asterdoc.create_run`, linked an existing local file with `asset.link`, then deleted that file. The Run workspace rendered `未解析 · 文件不可用` and `重新定位`.
- Core-rejected write: build stamp `ad8ed02`; isolated `scripts/rc3-fixture.py` Run `6899ae62-089a-47d9-9805-e1279322b51e`. Selected `接种细胞` at `A1 / D1 / FOV 1`, then chose the existing in-root PNG `ux02-no-project-1360x880-light.png`. The app kept its loaded Run snapshot while a separate `sqlite3` process changed the disposable test DB.
- Deleted exactly the Run record with `DELETE FROM records WHERE id='6899ae62-089a-47d9-9805-e1279322b51e' AND kind='run'`. Output: `before|1`, `deleted_rows|1`, `after|0`. Local path/root validation passed; Core `SaveAssetLink` rejected the missing Run with `INTERNAL: 实验运行不存在`. The link dialog showed the reason; after closing it, the page retained `关联失败：INTERNAL: 实验运行不存在` and `重新关联`. The Core write did not succeed.
- `APPDATA`/`LOCALAPPDATA` were isolated under the unique temp case directory. `mygo-automation windows --json` reported bounds `1360×880` and `900×600`; screenshots are `2018×1308` and `1328×888` device pixels, captured without resampling.
- `scripts/mygo-build.ps1` produced `BUILD_INFO` at `ad8ed02`, MyGo pin `v0.8.4-0.20261108220316-c4197b7fe562`, `Runs.dll` SHA256 `CCD4D8B2AA237A765D37080FE8F5C37E2E727F3A4845CA64C2AE17C55C774977`, and app SHA256 `8CC226C4FD38AA6CE7985E173F4D95C71C85F73382ED6CED2133CA4114624014`.
