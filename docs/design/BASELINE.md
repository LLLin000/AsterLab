# Web Run Baseline

- 日期：2026-11-08
- 服务：`http://127.0.0.1:19000`
- Fixture：`L929 closure run` / Run `61b8961c-e2b7-43e1-bcd9-84aa6c427c57`
- 浏览器：Chromium，1440×1000 与 390×844
- 说明：这是**实现现状**，不是用户测试结论，也不是已批准的目标设计。

## Screenshots

| 文件 | Route / state | 观察 |
|---|---|---|
| `baseline/workflow-1440.png` | `/?page=workflow` | 实验设计入口与设计上下文 |
| `baseline/protocols-1440.png` | `/?page=protocols` | Protocol 列表/工作区 |
| `baseline/runs-current-1440.png` | `/?page=runs&run=...` D7，当前任务 Stain LIVE/DEAD | 当前任务、三个未绑定模块、确认/记录 |
| `baseline/runs-record-1440.png` | 选中 CCK-8 read | 结构化读数已进入确认/记录，但表格位于首屏下方 |
| `baseline/runs-results-1440.png` | Run → 结果 | 事件/测量结果视图 |
| `baseline/tools-1440.png` | `/?page=tools` | 快速入口、资源卡、工具 |
| `baseline/runs-current-390.png` | 390px，Run 当前任务 | 手机上时间条、任务、模块、记录顺序 |
| `baseline/runs-current-light-1440.png` | 强制 `data-bs-theme=light` | DESIGN.md 的暖灰/浅色 token 与默认 dark 的差异 |

## Current state inventory

| # | Surface | State | Evidence | Gap |
|---|---|---|---|---|
| 1 | Shell | 项目/设计下拉可用 | screenshots | 上下文切换的未保存保护未在基线中验证 |
| 2 | Design | 设计列表/入口 | `workflow-1440.png` | 与 Run 的视觉密度不一致 |
| 3 | Protocol | 列表 | `protocols-1440.png` | 未覆盖未解析参数状态 |
| 4 | Run | 当前任务，无测量 schema | `runs-current-1440.png` | 三个“未绑定”卡片等重；记录区在下方 |
| 5 | Run | 当前任务，有测量 schema | `runs-record-1440.png` | 记录字段已内联，但 1000px 高首屏看不到完整表格/主操作 |
| 6 | Run | 已完成任务回看 | 交互走查 | 选中已完成任务后，后续任务显示“未开始”；需确认这是选择态还是执行态 |
| 7 | Run | 检查阻断 | 交互走查 | 需固定阻断时不能完成；当前未在基线中显示 |
| 8 | Results | 有测量事件 | `runs-results-1440.png` | 事件、测量、当前任务状态的单一来源需在 Rust 契约中固定 |
| 9 | Results | 无测量 | 数据 fixture | “已完成但无读数”文案存在；未做批量边界 |
| 10 | Tools | 资源卡/计算器 | `tools-1440.png` | 卡片数量与空状态未覆盖 |
| 11 | Mobile | 390px Run | `runs-current-390.png` | 无横向溢出；记录表在手机上需要横向/堆叠策略 |
| 12 | Retired | `?page=records` | 自动化测试 | 已回退到实验运行；不允许再出现第二个记录入口 |

## Observed baseline facts

- 默认文档是 **dark**：`<html data-bs-theme="dark">`，背景 `#101318`，不是 DESIGN.md 的暖灰 `#f7f6f3`。
- 强制 `data-bs-theme="light"` 后，背景是 `#f7f8fa`、ink `#202938`；tokens 仍是冷灰/蓝，不是 DESIGN.md 的暖灰/紫。
- DESIGN.md 的 accent 是 `#5b45c9`；实际 light token 是 `#2563eb`，dark 是 `#6d8cff`。这是设计系统漂移，必须在迁移前收口。
- Run 页当前任务、材料/仪器/检查、确认/记录、历史记录是多个等权卡片。
- 已有结构化记录表，但仅在确认/记录卡片内部；在 1440×1000 下，表格底部超出首屏。
- 三个未绑定模块在空状态下占据整行，视觉重量高于实际信息。
- Schedule 当前由时间点驱动；阶段/线性 Protocol 的 fallback 尚未在 UI 中验证。
- 手机 390px 无横向滚动；但记录表可用性未完成验证。

## Impeccable live detector — Run baseline

Command:

```bash
"C:/Users/Lin/.agents/skills/impeccable/scripts/impeccable.cmd" detect --json \
  'http://127.0.0.1:19000/?page=runs&run=61b8961c-e2b7-43e1-bcd9-84aa6c427c57'
```

Result: **10 findings — 8 warnings, 2 advisory**.

| Severity | Finding | Evidence |
|---|---|---|
| warning | Undersized functional text ×7 | `通用准备`, `D7 任务`, `已完成`, `当前`, `未开始` at 10px |
| warning | Low contrast primary action | white on `#6d8cff` = 3.1:1 |
| advisory | Brand contrast | white on `#7873ff` = 3.7:1 |
| advisory | Textarea placeholder | `#757575` on `#3b3b3b` = 2.4:1 |

Static scan of `web/` also reports 12 warnings / 133 advisories; many are in legacy/workspace CSS and must not be treated as Run-specific. The live Run scan is the migration baseline.

## Acceptance use

Use this baseline to compare the Rune Run slice:

- Same Run id, same selected task, same timepoint.
- Same completion/measurement states.
- Same unknown labels.
- Same data after restart.
- Visual differences are allowed only when approved via `CRITIQUE.md`; factual differences are failures.
