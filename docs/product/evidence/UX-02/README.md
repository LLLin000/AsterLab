# UX-02 — 恢复状态证据（Core 拒绝未验证）

真实桌面窗口尺寸为 1360×880 和 900×600，浅色主题；当前 Windows 显示缩放下，PNG 像素尺寸分别为 2018×1308 和 1328×888。空态数据在隔离的 `LABWORKBENCH_DATA_DIR` 中创建。
Core CLI 复现（非 GUI）：在隔离目录 `D:/.ci-cache/ux02-data` 中创建冻结 Run；`asset.link` 使用 `root_dir=D:/.ci-cache/qa-lb06-assets` 和路径 `D:/.ci-cache/qa-lb06/qa01-1-plate-1360x880-light.png`，Core 返回 `INVALID_INPUT: asset is outside its root`。GUI 截图里的同类失败由 Go-side 本地路径校验触发；它没有证明该写请求到达 Core。

| 状态 | 截图 | 下一步动作 |
|---|---|---|
| 无项目 — 尚无项目 | [1360×880](ux02-no-project-1360x880-light.png) · [900×600](ux02-no-project-900x600-light.png) | 新建设计 |
| 空项目 — 尚无实验设计 | [1360×880](ux02-empty-project-1360x880-light.png) · [900×600](ux02-empty-project-900x600-light.png) | 新建设计或导入设计 |
| 关联文件不可用 — 文件路径不可访问 | [1360×880](ux02-unavailable-asset-1360x880-light.png) · [900×600](ux02-unavailable-asset-900x600-light.png) | 重新定位 |
| GUI 本地路径校验失败 — 显示原因和恢复动作；非 Core 响应证据 | [1360×880](core-rejection-1360x880-light.png) · [900×600](core-rejection-900x600-light.png) | 重新关联 |

## 未验证的验收状态

本 PR 证明 GUI 本地校验失败时会显示原因与“重新关联”动作，也单独记录了 Core CLI 拒绝越界路径。尚未捕获“GUI 发出写请求后收到 Core 拒绝”的真实窗口状态；UX-02 第四态仍未验证，不应据此将任务标为完成。

## 构建与接线

- 构建戳：`b2acf22`（`dist/BUILD_INFO.txt` 中的 `commit` 与 `git rev-parse --short HEAD` 一致）。
- 错误写入状态：`prototypes/mygo-run/main.go:2988-2995`。
- 错误传入记录视图：`prototypes/mygo-run/main.go:2529-2541`。
- 横幅与重试按钮：`prototypes/mygo-run/internal/runworkspace/recording.go:464-473`。
- 本 PR 无代码改动；仅更新本目录中的证据。
