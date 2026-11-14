# UX-02 — Recoverable empty and error states

Required capture format: real desktop window, light theme, 1360×880 and 900×600. Seed each state in an isolated data directory; do not fabricate records to make an empty state look populated.

| State | What the user sees | Next action | Rule id | Real-GUI evidence |
|---|---|---|---|---|
| 未开始 — no project | Empty workspace says “未开始 · 尚无项目” | 新建设计 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-no-project-1360x880-light.png) · [900×600](ux02-no-project-900x600-light.png) |
| 未开始 — project has no designs/runs | Empty project says “未开始 · 还没有实验设计” | 新建设计 or 导入设计 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-empty-project-1360x880-light.png) · [900×600](ux02-empty-project-900x600-light.png) |
| 未解析 — linked asset unavailable | Original Run asset path is retained and marked unavailable | 重新定位 | UX-P5, UX-STATE, UX-DONE | [1360×880](ux02-unavailable-asset-1360x880-light.png) · [900×600](ux02-unavailable-asset-900x600-light.png) |
| NOT VERIFIED — Core-rejected write | No recovery state could be captured. The host `OnLinkFile` error path only emits a toast; see failure below. | Required host callback must expose the error to the internal view before a retry action can be rendered. | UX-P5, UX-STATE, UX-DONE | NOT VERIFIED |

## Seed and failure record

- No-project: empty `LABWORKBENCH_DATA_DIR`.
- Empty project: `project.create` seeded project `UX-02 Empty Project`, with no designs or Runs.
- Missing asset: created a real Run through `asterdoc.create_run`, linked an existing local file with `asset.link`, then deleted that file. The Run workspace rendered `未解析 · 文件不可用` and `重新定位`.
- Core-write failure: attempted a write with the isolated data directory made non-writable using `icacls`. The desktop app disconnected before the UI error could be captured; the exact failed command was `mygo-automation press @e52 --settle` (`idle: the app did not answer: read unix @->C:\\Users\\Lin\\AppData\\Local\\Temp\\ux02-evidence\\core-rejection.sock: wsarecv: An existing connection was forcibly closed by the remote host`). The `icacls` operation also reported `拒绝访问` for `labworkbench.sqlite`.
- `asset.link` with a root ID not previously registered was accepted by Core, so it did not produce the requested rejection.
- App host call site `prototypes/mygo-run/main.go:1923-1926` catches linked-asset write errors with a toast and does not pass the error to `internal/runworkspace.RecordingView`; the requested UI recovery treatment cannot be completed within this card's write scope.
