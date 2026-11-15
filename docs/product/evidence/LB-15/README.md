# LB-15 证据：Notebook 写入失败提示、草稿持久保留与中文状态表达

## 构建戳与二进制摘要（与截图采集时 `git rev-parse --short HEAD` 一致）

- Capture `git rev-parse --short HEAD`: `815a436`
- `O:\lb15\prototypes\mygo-run\dist\BUILD_INFO.txt`:

```text
AsterLab MyGo Prototype (UNSIGNED)
commit 815a436
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 CEF48D96A28635B26E871C2A9D6430D1FCEDA066A6FF9C97F2CB486B5A153CE4
mygo-run.exe SHA256 E7F7605CE331ADC87B6128B9ABF550311B1FBF23AD10D9B654EDF2B347FB9241
```

## 修改与 §3 状态词对齐说明

1. **保存失败可见且支持重试**（`prototypes/mygo-run/notebook_page.go`）：
   - `notebookToolbarState` 根据真实运行状态渲染工具栏状态词：
     - `保存中`（进行中保存，对应 `a.noteState == "saving"`）
     - `未保存`（草稿未写入或写入失败，对应 `a.noteDirty || a.noteState == "error"`）
     - `已保存`（已写入 Core 且无未保存更改，对应 `a.noteState == "saved"`）
     - `草稿`（空白新建未写入 Core 的初始状态）
   - 依据 `docs/product/ux-contract.md` §3 状态词表与 `internal/notebook/notebook.go` (`SaveStateLabel`)，产品内统一使用 `已保存` / `未保存` / `草稿` / `保存中`（不使用与之冲突的同义词 `已提交`）。
   - 保存失败时顶部横幅展示 `未保存（<Core 映射错误原因>）` 与 `重试` 按钮，保留当前编辑草稿并在磁盘持久化；点击 `重试` 重新调用 `a.saveNotebook()`。
2. **切换 Run 与重启不丢草稿**（`prototypes/mygo-run/notebook_page.go`、`prototypes/mygo-run/main.go`）：
   - 未保存草稿实时同步至内存 `a.noteDrafts[runID]` 并以原子临时文件+重命名写入 `LABWORKBENCH_DATA_DIR/drafts/notebook-<fnv>.json`；切换 Run、发生写入失败或重启应用后重新进入该 Run，自动恢复未保存草稿并显示 `已从本地草稿恢复未保存修改。` 与 `丢弃草稿` 操作。
   - 在实窗测试中同时发现并修复了 `main.go` 中 `applySnap` 跨 Run 切换未重置 `a.guard = state.StaleGuard{}` 的问题（此前从高 Revision 的 Run B 切回低 Revision 的 Run A 会被错误识别为“忽略过期快照”）。
3. **全中文界面标签、可读时间线摘要与渐进披露**：
   - 时间线头部从 `Document Revision: %d` / `Design rev %d` 统一为 `设计版本 %d · %d 条`；只读空态显示 `未开始` 徽标与 `尚未记录笔记内容，切换到“实时”或“编辑”开始撰写。`。
   - 时间线条目默认展示中文可读来源摘要（`创建 Run`、`测量记录`、`实验运行 · 创建时有 N 条警告 · 操作者 ...`），并将原始 `SourceType/SourceID` 放入悬停 `.Tooltip` 与一键展开的 `components.Collapsible` `"来源详情"` 中。

## 真实窗口截图（`1360×880` 浅色主题，隔离 `LABWORKBENCH_DATA_DIR`）

所有截图均由 `mygo-automation` 驱动真实 `mygo-run.exe`（`815a436`）与 Core SQLite 采集：

1. [notebook-chinese-states-1360x880-light.png](notebook-chinese-states-1360x880-light.png)
   - 中文状态词 `草稿`、`设计版本 2 · 2 条`、`运行记录 [只读]`，以及经 `execution_event.create` 写入的 `测量记录`（无裸 `Document Revision` / `execution_event` / UUID 暴露），原始来源收拢于 `来源详情`。
2. [notebook-readonly-empty-1360x880-light.png](notebook-readonly-empty-1360x880-light.png)
   - 只读“阅读”标签页空态：显示 §3 状态徽标 `未开始` 与引导文案 `尚未记录笔记内容，切换到“实时”或“编辑”开始撰写。`。
3. [notebook-save-error-retry-1360x880-light.png](notebook-save-error-retry-1360x880-light.png)
   - 隔离 SQLite 中临时移除目标 Run 后点击 `保存笔记`：工具栏状态切换为红色 `未保存`，横幅显示 `未保存（INTERNAL: 实验运行不存在）` 与 `重试` 按钮且正文保留。随后在自动化脚本中恢复 SQLite 行并点击 `重试`，验证二次提交成功转为 `已保存`。
4. [notebook-draft-switch-run-1360x880-light.png](notebook-draft-switch-run-1360x880-light.png)
   - 在 Run A 编辑未保存草稿（`Run A 未保存草稿 — 切换 Run 与重启后仍保留`）后切至已保存笔记的 Run B（显示 `Run B 已保存实验笔记。` 与 `已保存`），再切回 Run A：Run A 草稿完整保留并显示 `未保存` 与恢复横幅；随后彻底退出并重启 `mygo-run.exe`，验证重启后草稿仍从磁盘恢复。
