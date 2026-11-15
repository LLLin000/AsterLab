# UX-06 — 引用选择器改用主题字号

## 已关闭缺陷

- 缺陷 A（链接失败错误可见）：已由 LB-17（#140，`b2acf22`）关闭。`main.go:2988-2995` 保存 Core 链接失败原因并使界面刷新；`internal/runworkspace/recording.go:464-473` 显示 `关联失败：…` 与 `重新关联` 重试按钮。
- 缺陷 B（Notebook 状态词）：已由 LB-15（#139，`b70a0f6`）关闭。`notebook_page.go:786-800` 按实际状态显示 `保存中`、`未保存`、`已保存`、`草稿`。

## 本次改动

引用选择器及引用快照区域的 7 处固定 `FontSize(<数字>)` 改用 `tokens.TextMeta` / `tokens.TextData`，字号遵循主题文字比例（UX-P3）。

真实窗口截图：[`citation-selector-1360x880-light.png`](citation-selector-1360x880-light.png)，1360×880 浅色主题；`@` 引用选择器已展开。

## 构建戳

- 构建 commit：`b2acf22`
- mygo pin：`v0.8.4-0.20261108220316-c4197b7fe562`
- `Runs.dll` SHA256：`8D23FD899295DB49DAE6BAEBD4F85952FB4BC8C8E24B892B98857873EA7CDE26`
- `mygo-run.exe` SHA256：`B1FF08E419C516B79E9B549936FCF395444EAA4DD3C612712B753FE45370E434`
- `go build ./...`：通过
- `go test ./... -count=1`：通过（23 packages ok，2 no tests）；首次因 Runs 原生模块缓存过期失败，刷新模块后通过。
