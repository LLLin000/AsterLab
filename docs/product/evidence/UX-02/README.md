# UX-02 — 四态恢复证据

真实桌面窗口尺寸为 1360×880 和 900×600，浅色主题；当前 Windows 显示缩放下，PNG 像素尺寸分别为 2018×1308 和 1328×888。空态数据在隔离的 `LABWORKBENCH_DATA_DIR` 中创建。
Core 拒绝复现：隔离目录 `D:/.ci-cache/ux02-data` 中创建冻结 Run；CLI `asset.link` 以 `root_dir=D:/.ci-cache/qa-lb06-assets`、路径 `D:/.ci-cache/qa-lb06/qa01-1-plate-1360x880-light.png` 请求关联，Core 返回 `INVALID_INPUT: asset is outside its root`。GUI 截图记录了同一越界文件关联失败后的恢复提示。

| 状态 | 截图 | 下一步动作 |
|---|---|---|
| 无项目 — 尚无项目 | [1360×880](ux02-no-project-1360x880-light.png) · [900×600](ux02-no-project-900x600-light.png) | 新建设计 |
| 空项目 — 尚无实验设计 | [1360×880](ux02-empty-project-1360x880-light.png) · [900×600](ux02-empty-project-900x600-light.png) | 新建设计或导入设计 |
| 关联文件不可用 — 文件路径不可访问 | [1360×880](ux02-unavailable-asset-1360x880-light.png) · [900×600](ux02-unavailable-asset-900x600-light.png) | 重新定位 |
| Core 拒绝写请求 — 关联失败并显示原因 | [1360×880](core-rejection-1360x880-light.png) · [900×600](core-rejection-900x600-light.png) | 重新关联 |

## 构建与接线

- 构建戳：`b2acf22`（`dist/BUILD_INFO.txt` 中的 `commit` 与 `git rev-parse --short HEAD` 一致）。
- 错误写入状态：`prototypes/mygo-run/main.go:2988-2995`。
- 错误传入记录视图：`prototypes/mygo-run/main.go:2529-2541`。
- 横幅与重试按钮：`prototypes/mygo-run/internal/runworkspace/recording.go:464-473`。
- 本 PR 无代码改动；仅更新本目录中的证据。
