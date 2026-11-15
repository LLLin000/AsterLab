# AsterLab — Component Contract

- 状态：**Native UI Direction Freeze / 2026-11-08**
- 本文定义 AsterLab 自己的组件语义和 Rune 映射；不复制 Rune/OpenCode 的默认视觉。
- 迁移期间 Web 组件与 Rune 组件共享同一数据和状态契约；不共享实现。

## Ownership

```text
AsterLab theme      → semantic tokens
AsterLab kit        → AsterLab components (wraps rune / rune-kit)
AsterLab screens    → composition only
Rune / rune-kit     → rendering, layout, input, accessibility primitives
```

规则：

- 页面不直接拼 `rune-kit` 的 Button/Table/Input；通过 AsterLab kit 调用。
- 组件不读取数据库、不发 HTTP、不判断科学规则；只接收 view model 和意图回调。
- 组件不把“计划值”显示成“实际值”；不把未知显示成 0、空或通过。
- 所有颜色、间距、字体、圆角、动效来自 token，不写裸值。

## Token contract

来源：[DESIGN.md](DESIGN.md) 的 frontmatter。

| 类别 | 必须存在 | 用途 |
|---|---|---|
| Color | canvas, sidebar, surface, surface-subtle, surface-inset, hairline, ink, ink-muted, ink-subtle, accent, accent-soft, accent-tint, warn, danger, success | 表面、文字、状态、当前动作 |
| Type | display, title, headline, body, body-strong, secondary, meta, caption | 七级文字阶梯 |
| Space | 4, 8, 12, 16, 24, 32, 48 | 8pt 邻接节奏 |
| Radius | 4, 6, 10, 12, pill | 控件/面板/状态 |
| Motion | hover, selection, expand, reduced | 120/200/320ms；reduced 直接到终态 |

Rune 使用 OKLCH 颜色；从 token 生成 Rune theme，不手写第二份颜色。Web 与 Rune 都从同一个语义 token 源生成。

## Core components

### Shell

| Component | 责任 | 状态 | Rune mapping |
|---|---|---|---|
| `AppShell` | 顶栏、rail、内容区、全局选择器 | normal, loading, disconnected | `col/row` + `rune_kit::shell::SplitView`（仅需要侧栏/检查器时） |
| `Rail` | 四个主标签 | selected, hover, focus | `Sidebar` / custom keyed rows |
| `ContextBar` | 项目 → 设计 → Run | normal, stale, missing snapshot | `Popover` + `Text` |
| `StatusChip` | 状态文本 + 语义色 | done, current, blocked, unknown, warning | `Badge` / custom |
| `PageHeader` | 标题、副标题、主/次操作 | normal, stale, error | `Toolbar` + `Button` |

### Schedule

| Component | 责任 | 状态 | Rune mapping |
|---|---|---|---|
| `ScheduleStrip` | 时间点/阶段/步骤进度 | timepoints, phases, linear, empty | custom keyed row; no chart library |
| `TaskChip` | 单个任务入口 | done, current, pending, blocked | `Button`/custom row |
| `TaskHeader` | 当前步骤标题、说明、参数、作用域 | normal, no parameters, unresolved | `Text` + metadata rows |

ScheduleStrip 只消费结构化任务；阶段来自 Protocol/任务字段，不来自标题正则。

### Run workspace

| Component | 责任 | 状态 | Rune mapping |
|---|---|---|---|
| `TaskWorkspace` | 左侧记录、右侧就绪栏 | editable, readonly, blocked | `row` + `col` |
| `RecordTable` | 动态列、行内输入、添加行 | empty, partial, valid, invalid, saved | `rune_kit::Table` or custom table |
| `RecordCell` | 数值/文本/单位输入 | focus, error, disabled | `TextInput` / `NumberInput` |
| `ReadinessList` | 材料、仪器、检查紧凑列表 | bound, unbound, missing, blocked | `List` / custom rows |
| `CheckField` | 检查项状态 | unset, passed, failed, not_applicable | `Select` / `SegmentedControl` |
| `DeviationField` | 偏差/实际变化 | empty, filled | `TextArea` |
| `CompletionAction` | 完成本步 | enabled, blocked, saving, saved | `Button` |
| `MeasurementAction` | 保存读数 | enabled, invalid, saving, saved | `Button` |

**关键规则**

- `RecordTable` 不自动生成列；列由 Assay/Protocol schema 提供。
- `RecordCell` 在失焦或提交时校验；错误显示在单元格附近和顶部摘要。
- `CompletionAction` 只发出 `complete_task` 意图；阻断判断在 Rust 应用层。
- `MeasurementAction` 只发出 `record_measurement` 意图；与完成事件分离。
- `ReadinessList` 的“未绑定”是文本状态，不是空白或禁用假象。

### Results

| Component | 责任 | 状态 | Rune mapping |
|---|---|---|---|
| `ResultTable` | 事件、测量、读数、检查 | empty, partial, complete, error | `Table` |
| `MissingEvidenceList` | 已完成但无读数 | normal, empty | `List` |
| `BulkEntry` | 批量粘贴/解析预览 | empty, parsed, error, saved | `TextArea` + preview |

Results 与 Run 消费同一个结果记录契约；不能各自拼 `measurements[]`。

### Design / Protocol / Tools

| Component | 责任 | 状态 |
|---|---|---|
| `DesignStepRail` | 四步草稿 | step, complete, incomplete |
| `GroupTable` | 组别、条件、独立 n | normal, selected, unresolved |
| `AssayMatrix` | 组别 × 检测覆盖 | empty, partial, complete |
| `RelationSummary` | 样本/分样/共用摘要 | unknown, partial, complete |
| `ProtocolTree` | 步骤块树/版本/参数 | collapsed, expanded, stale |
| `ParameterField` | 参数、单位、来源 | resolved, unresolved, invalid |
| `ResourceCard` | 材料/资料表/公式/仪器 | normal, selected, missing |
| `CalculatorPanel` | 输入 → 预览 → 应用 | idle, calculating, preview, applied, error |
| `PlateMap` | 孔位状态 | empty, mapped, selected, actual |

## Interaction contract

- **键盘**：Tab 顺序符合阅读顺序；Enter 触发当前主操作；Escape 关闭浮层；表格方向键移动单元格；焦点环始终可见。
- **目标尺寸**：桌面主要控件 ≥ 28px 高；触屏/移动 ≥ 44px；表格行高 ≥ 36px。
- **焦点管理**：打开抽屉/模态后焦点进入；关闭后回到触发点。
- **保存语义**：显式保存；未保存离开提示；失败保留输入；成功有可感知但不过度的确认。
- **动效**：只用于状态变化；120–200ms；`prefers-reduced-motion`/Rune reduce motion 下直接到终态。
- **无障碍**：每个图标按钮有 label；表格有行列角色；状态文本不只靠颜色；错误与字段关联。
- **国际化**：中文为第一语言；英文标识保留；长文本可换行，不截断关键值。

## Rune mapping notes

- Rune 的 `Table` 支持虚拟化、排序、列宽、键盘导航；AsterLab 需要在其上增加**可编辑单元格、动态列、单位、行校验**。若 `rune-kit::Table` 的编辑能力不足，允许在 AsterLab kit 内实现表格，不修改上游。
- Rune 的 `TextInput` 支持 IME；仍必须在 Windows 实测中文候选、组合输入、复制粘贴和失焦提交。
- Rune 的主题 token 用 OKLCH；从 AsterLab token 生成，不直接使用 `rune_theme_opencode`。
- Rune 的 `Button`/`Select`/`Popover` 等仅在 AsterLab kit 内部使用；屏幕层只认 AsterLab 组件。
- Rune 的动画是运行时弹簧；AsterLab 只使用 calm/snappy 两档，不新增装饰性循环动画。
- Rune 的 headless snapshot 是验收工具，不是视觉设计依据；最终以真实 Windows 窗口和真实任务走查为准。

## Component acceptance

每个组件迁移必须提供：

1. 状态表（normal / hover / focus / disabled / error / loading / empty）。
2. 键盘路径和焦点顺序。
3. 1440/1920/390 或对应窗口尺寸截图。
4. 中文长文本、数字、单位、未知值的 fixture。
5. Web 与 Rune 同一 fixture 的对照。
6. Impeccable 组件级 critique；*不通过则不进页面。*
