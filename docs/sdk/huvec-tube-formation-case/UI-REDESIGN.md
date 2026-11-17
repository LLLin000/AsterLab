# HUVEC 成管案例驱动的三板块 UI/UX 与同源架构重设计方案

- 关联文件：`DESIGN.md`、`huvec-tube-formation.asterml`
- 目的：保存针对当前 AsterLab 三大核心工作面（**实验设计 Design → 实验运行 Run → 记录板块 Record [Notebook + 审计流水]**）的交互重排、Timer 状态机、同源双渲染架构与多端屏幕适配规范。

---

## 一、核心架构原则：Design 为唯一真相源（SSOT）

```text
[1. Design 可编辑真相源]
   分组/孔板 · 配液公式 · 步骤/循环/条件 · 耗时预算 · 采集槽定义
          │
          ▼  Freeze (design_version_id)
[Frozen Blueprint 不可变蓝图]
          ├───────────────────────────────────────────┐
          ▼                                           ▼
[2. Run 台面执行]                           [3. RecordProjection 统一只读投影]
 计时实测 · 实际参数 · 条件跳过 · 偏差        (Blueprint + Facts + Assets + Notes)
 12孔×6视野 162张照片 (LinkedAsset)                   ├──▶ 渲染 A：Run 审计流水（查漏/对账）
                                                      └──▶ 渲染 B：Notebook 实验本（自动摘要+3×3图板+Markdown研判）
```

**强约束**：Run 和记录板块都不重新定义分组、孔位、计划参数或步骤结构；Run 只往冻结蓝图派生的槽位里写权威事实，Notebook 与 Run 历史读同一份 `RecordProjection`。

---

## 二、各板块重设计细则

### 1. 实验设计（Design）
- **去掉冗余顶栏与空画布感**：合并多层浮条为单一顶栏（视图切换 `[工作台 | 源码]` + 主动作 `创建 Run`）；新增全局**时间与孔板预算条**（显示总跨度 `~27.5h`、净占手时间 `~2.5h`、已选孔数 `9/12`、可并行空窗 `Step 03 ≥2h`）。
- **步骤卡片必须承载关键操作事实**：直接显示 `序号 + 标题 + 结构化参数摘要 + 耗时/类型标签`（如 `80°C 5m ↔ 冰 10m ×3 · 占手 45m`），彻底消灭 `未填写参数` 占位。
- **12 孔板 ↔ 配液单实时联动**：多选或取消孔位时，计算器所需孔数自动同步，GelMA 称重（含黏稠余量）与 HUVEC 细胞总数实时重算。
- **技术绑定退居二线**：`Protocol path`、`reference` UUID 默认折叠在属性区底部。

### 2. 实验运行（Run）
- **操作台心智与“按步骤变身”**：
  1. **循环计时型（Step 01）**：显示 `Round 1/3 → 2/3 → 3/3` 自动接力倒计时；条件步骤（`2000 rpm 离心`）并排提供 `[执行并记参数]` 与 `[无气泡 · 按规则跳过]`（写入 `skipped_by_rule`，区别于异常 `deviation`）。
  2. **分组差异记录型（Step 02）**：12 孔板转为分组底图，支持整板记一次或按 `3% / 5% / 7%` 分别跑正计时秒表，记录各组实际 405nm 固化秒数。
  3. **长空窗与时间锚点型（Step 03 & Step 04）**：Step 04 完成锁定绝对时间戳 $t_0$，自动推算 `6h / 12h / 24h` 目标时间窗；长窗口挂到顶栏常驻时钟条。
  4. **多视野采图型（Step 05a/b/c）**：12 孔板按 `rows × cols` 自适应尺寸不溢出；每孔显示 `0/6 → 6/6`，支持批量拖入按序填槽，**传满 6 张自动跳至下一未满孔**。
- **可恢复 Timer 持久化契约**：
  Timer 作为 Run 事实持久化，不依赖前端内存倒计时：
  `{timer_id, run_id, task_key, phase_index, mode(countdown|stopwatch|window), planned_s, started_at_utc, accumulated_s, status}`。
  任意端打开或软件重启均按 `accumulated_s + (now - started_at_utc)` 重算剩余/已用时间。
- **扩展资产坐标（`LinkedAsset`）**：在 `{RunID, TaskKey, SampleID}` 基础上增加 `WellID`、`Timepoint`、`FOVIndex`（支持 162 张显微图精确定位与缺漏检查）。

### 3. 记录板块（Notebook + Run 历史双渲染）
- **消灭空白记事本**：Notebook 打开即从 `RecordProjection` 自动铺出：
  1. 计划 vs 实际执行摘要（含 Timer 实测耗时、405nm 各组固化秒数、$t_0$ 与条件跳过）；
  2. **`3% / 5% / 7% × 6h / 12h / 24h` 代表视野对比矩阵**（默认取首视野，点击可切换 1..6 视野）；
  3. 右侧/下方为研究者 Markdown 研判区，支持插入可解析的动态引用块（如 `@matrix(6h,12h,24h)`），导出 PDF 自动展平为实拍图板与参数对照。
- **审计流水（Run 历史）**：同一份数据的审计渲染，支持按 `孔位 / 时间点 / 状态` 筛选，核查漏图与精确操作时间戳。

---

## 三、多屏幕响应式布局规范

| 板块 | 手机端（`390×844` 竖屏单列） | 16 寸全屏（`1440×900` 16:10） | 27 寸全屏（`2560×1440` 16:9） |
|---|---|---|---|
| **实验设计 (Design)** | 放弃 2D 画布，改为纵向步骤卡 + 顶部预算条；`流程 / 12孔板 / 配液单` 分段切换；底抽屉编辑 | 左流程链 + 中 `12孔板↔配液联动` + 右精简属性（底层 ID 折叠） | 流程画布 + 12 孔板 + 配液计算器 + 属性同屏全展开，无需切 tab |
| **实验运行 (Run)** | **纯单步台面模式**：顶栏锁住计时器，大字展示当前参数，底部拇指区主按钮；不渲染大矩阵图槽 | 三栏联动：左步骤轨 + 中大字操作台（含 Timer） + 右动态采集区（Step 5 才展 12 孔板） | 四栏工作台：中操作台**限宽 ~560px 防拉扁**，右放大 6 视野直接判读，最右停靠**多实验并行时钟 + 实时流水** |
| **记录 (Record)** | 上半区自动摘要 + 按时间点横滑 3 组代表图；下半区短笔记 | 双栏分屏：左自动摘要 + `3×3` 图板，右 `max-width 760px` Markdown 研判 | 居中双栏大画幅（总宽限制 ~1700px）：左展开多视野高清对比与偏差表，右同步撰写与插入动态图块 |

---

## 四、AsterML v2 通用模块基础设施落地状态与后续边界（2026-11-18）

基于三层模块架构与硬规则（“能用通用模块组合实现的需求，严禁新增实验专用模块或面板”），P0 基础设施与核心 UI 工作台已在本分支（`feat/v2-generic-modules`）落地：

1. **AsterML v2 契约与核心收口 (`sdk 2.0.0`)**：
   - `step@1` 通过有序 `blocks[]`（与可选 `block_refs`）自由组合 10 类通用内容模块（`prose`、`group_table`、`param_table`、`figure`、`data_table`、`checklist`、`calculator`、`timer`、`attachment`、`subflow`）及标准容器 `plate@1`。
   - 彻底移除 `plate@1` 的 HUVEC 专属 `mix_*` 字段与实验名硬编码（`internal/assays/schema.go` 增加通用 `genericAssaySchema` 回退），单一 `calculator@1` 收口 5 种通用能力模式（`dilution`、`group_mastermix`、`serial_dilution`、`sample_normalize`、`formula`），统一 UCUM 单位表与纯函数公式引擎。
   - **自包含自动物化冻结**：`step@1` 不再强制预先创建外部 `ProtocolBlockVersion` UUID；创建 Run 时由 Core 自动物化未绑定步骤并冻结同一份模块树。

2. **Design 三标签页工作台 (`prototypes/mygo-run/design_page.go`)**：
   - 顶部保留单一工作流顶栏 + 实时 `ComputeBudgetSummary` 资源预算条（步骤数、分组数、孔位/皿数、预计占手与总跨度），废除零调用孤岛画布 `design_canvas.go`。
   - **步骤工作台**：左侧可折叠/调节的步骤大纲（`components.Split`）+ 右侧自由编排流（`StepBlockComposer` + `components.MenuButton` 插入 10 类模块与标准孔板）。
   - **实验全局设定**：实验档案、分组表、标准容器与仪器/试剂清单集中编辑，无实验硬编码字段。
   - **AsterML 源码**：基于 `components.SplitPercent` 与 `mygo/ui/editor` 的 Markdown 源码编辑，右侧实时联动 `ModeProtocol` 预览。

3. **Run 同源执行与 Notebook 闭环**：
   - `internal/runworkspace` 按步骤加载冻结 `step@1.blocks[]` 并以 `asterdoc.ModeRun` 同源渲染，参数实测记录、检查清单确认/按规则跳过、计时器启停、数据表读数提交均回写 Core 不可变事实；`capture.go` 动态读取 `attachment@1`（`scope: container_grid`）的 `slots_per_cell`。
   - Notebook 动态适配计划/实测 `FOV` 层数，并在实测派生新设计（`DeriveDesignFromRun`）保存后同步写回新版 AsterML 文档修订（`SaveAsterDoc`）。

4. **后续工作方向（Phase 2 / Phase 3）**：
   - 完整接通步骤工作台内 `Run` 实际值对照回溯、`attachment@1` 真机文件缩略图异步解码缓存与批注缩放；
   - 补齐 `AGENT_API` 一步生成典型湿实验文档的自动化 E2E 回归（抑菌圈 / CFU / HUVEC）；
   - Windows 桌面打包与多分辨率实窗人工验收（`Windows Gate` 泳道）。
