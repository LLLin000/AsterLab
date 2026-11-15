# UI-PAN-10 储藏室台账证据

## 截图

- `1360x880-light.png`：浅色主题，窗口尺寸 1360×880。
- `900x600-light.png`：浅色主题，窗口尺寸 900×600。
- 两张均为 `mygo-automation` 从真实窗口捕获；系统显示缩放使 PNG 像素尺寸大于窗口尺寸。
- 截图构建戳：`dist/BUILD_INFO.txt` 的 `commit 64ca174`；截图前核对 `git rev-parse --short HEAD` 同为 `64ca174`。

## Core 桥接与调用

`main.go` 的 `app.newToolsWorkspace` 将台账操作接到 `runSvc`：`ListPantry` 读取清单、`UpsertPantry` 新增/更新、`DeletePantry` 删除、`ExportPantryLabels` 生成插件标签清单；均传入 `context.Background()` 和默认 `PantryQuery{}`。UI 的 CRUD 与导出均经这些桥接方法，不在 Go 侧重算 Core 事实。

## 临期与导出规则

- 分组依据 Core 返回的 `PantryItem.Status`：仅 `near_expiry` 进入“临期”组，其余进入“其他”；临期使用主题 `Warning` token 高亮，过期/用尽使用 `Danger` token。
- “导出贴标清单”调用 Core 的 `ExportPantryLabels`。只有 Core 返回成功才显示成功提示，并将 Core 返回的 manifest JSON 展示为插件清单；失败展示原因和“重试”入口。

## 门禁说明

官方 `scripts/mygo-build.ps1` 在本 checkout 不重写 `mygo-plugin.json`；只有额外手动运行 `mygo native build -target windows-amd64` 才会重写它。
