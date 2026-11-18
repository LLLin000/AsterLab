# AsterLab v2 通用模块架构演进复盘与全表面实图验收档案

状态：已实现（`feat/v2-generic-modules` · 2026-11-18）

**初版审计基准**：远端主干 `origin/main` @ `6010ab9`（2026-11-17，累计 488 Commits / PR #188）+ 待合入分支 `origin/agent/CLOSE-07-entries-deeplink` @ `6923fd4`（第 1–6 节保留当日复盘全文）。  
**全量交付基线（2026-11-18）**：已在分支 `feat/v2-generic-modules`（基于 `5fcd0c1`，保留安全回退标签 `v0.1.0-beta-legacy-bench`）完成 **P0 基础设施 + `MOD-01..MOD-06` + 前端科研文案治理 + 多维正交孔板标注 (`plate@1`) + Notebook ELN 双分栏重塑 + 7 项通用引擎闭环 +「孔板速排」独立设计器（先选孔板 8 规格、复用悬浮单框选孔标注、SVG/PNG 矢量与高清导出、两列 `孔板历史` 卡片加载与导出）与 Windows 构建发布**（见第 7–10 节）。

> [!NOTE]
> **执行进度更新（第 1–6 节与第 7–10 节的关系）**：下方第 1–6 节完整保留初版代码复盘的现状取证与架构诊断；在此基础上，我们已经接续完成 P0、`MOD-01..MOD-06`、前端科研文案治理、多维正交孔板与 Notebook ELN 改造，以及 **21 个 SDK 模块逐一截屏与「孔板速排」全链路验收**（详见 [v2-module-visual-catalog.md](./v2-module-visual-catalog.md)）。第 7–10 节记录了全量验证账本、实图巡检与 Windows 校验哈希。

---

## 一、核心复盘定论：你的方向判断完全命中当前代码的根本病灶

对照 `origin/main` 全部 39 个 Rust Core 模块、17 个 AsterML 模块规范与 MyGo 六大桌面台面源码，**你提出的反思不仅在产品理念上完全正确，而且精准击中了过去一周（PR #95 ~ #188）代码演进中最严重的架构跑偏**。

可以用一句话概括 AsterLab 的现状：

**底层科学诚信地基（`Plan ≠ Fact`、不可变冻结、审计溯源、单样本状态机、离线持久化）打得非常扎实；但在模块层与 UI 台面层，团队在 HUVEC、RT-qPCR、WB、CFU 等案例压力下走上了「来一个实验、塞一套专用字段与专用面板」的歧路，且设计模型分裂为五套互不打通的结构，导致目前既无法在步骤内自由组合通用模块，也背上了大量实验硬编码包袱。**

---

## 二、AsterLab 演进脉络与现状全景（488 Commits · 5 个阶段）

### 2.1 五阶段演进复盘

| 阶段 | 周期与区间 | 核心成果 | 留下的结构性隐患 |
| :--- | :--- | :--- | :--- |
| **1. 科学契约奠基** | 11-02 ~ 11-05 | 对标 Cell Planner，在 [CONTEXT.md](../../CONTEXT.md) 确立 `Plan ≠ Fact`、独立样本 `n` ≠ 孔数、`ProtocolBlock` 无环图与冻结 `Run` 快照。 | 过早铺开 22 批湿实验调研，在 [crates/asterlab-core/src](../../crates/asterlab-core/src) 留下 `semantic_component.rs` / `semantic_composition.rs` / `instrument_matching.rs` 等至今未接线的孤岛抽象。 |
| **2. 四代前端更迭** | 11-06 ~ 11-10 | 6 天内经历 Vanilla Web（[web/](../../web)）→ Slint（[apps/desktop/](../../apps/desktop)）→ Rune → **MyGo + Onevra + PureGo `Runs.dll` C-ABI**（[prototypes/mygo-run/](../../prototypes/mygo-run)），跑通纯原生无 WebView 桌面安装包。 | 目录至今叫 `prototypes/mygo-run`（被 [native_generator_hashes.json](../../scripts/ci/native_generator_hashes.json) 与反向 `include_str!` 锁死）；旧 Web 文档 [DELIVERY_ACCEPTANCE.md](../../docs/DELIVERY_ACCEPTANCE.md) 未更新。 |
| **3. AsterML SDK v1** | 11-10 ~ 11-12 | 建立 `.asterml` Markdown 语法与 Go/Rust 共享的 [modules.json](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json)，实现 `asterdoc_version` 不可变版本与冻结编译。 | **埋下双轨制分裂**：AsterDoc 是叠加在旧 `Design` JSON + `mygo.*` 侧车之上的单向有损编译器，且 `step@3` 强制外链数据库动态生成的 `ProtocolBlockVersion` UUID。 |
| **4. 案例驱动大扩张** | 11-13 ~ 11-16 | Kandev 双泳道自动化冲刺（140+ 卡片），围绕 [UI-REDESIGN.md](../../docs/sdk/huvec-tube-formation-case/UI-REDESIGN.md) 与 [cross-assay-primitives-audit.md](../../docs/sdk/cross-assay-primitives-audit.md) 交付 Timer、Pantry、盲法、样本排除、`RecordProjection` 与嵌图归档 PDF。 | **把具体实验长相当成交互功能开发**：`plate@1` 塞入 GelMA/LAP 字段、造出 6 套专用计算器、Run 台面堆叠 5 套实验专用录入块、孔板网格硬编码仅支持 3 个实验名。 |
| **5. Beta RC 与紧急收口** | 11-17 | 打出 `v0.1.0-beta` 候选包，连开 `CLOSE-01..07` 修补前后端断头路与入口泛滥。 | 单文件 scope 隔离导致多处“单测全绿、主链断路”（如实测派生 Design 在 AsterDoc 主轨上 100% 保存报错）。 |

---

## 三、代码级铁证：当前系统偏离「通用模块自由组合」的四大症结

### 症结 1：大量「实验专用逻辑」硬编码进了通用模块、计算器与界面

| 污染位置（`origin/main` @ `6010ab9`） | 具体硬编码表象 | 违背的原则与后果 |
| :--- | :--- | :--- |
| **通用孔板模块被 HUVEC 配方污染**<br>[modules.json](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json) `L69-158`<br>[modules.go](../../prototypes/mygo-run/internal/asterdoc/modules.go) `L59-300` | 在通用 `kind: plate` 中硬塞了 **8 个水凝胶/细胞专属输入**（`mix_gelma_percent_by_group`, `mix_lap_stock_percent`, `mix_lap_final_percent`, `mix_cell_density`, `mix_factor_*`）及专属 C-ABI `MixPlanService`。 | 孔板组件只应负责**二维几何与分组/样本坐标映射**；换成海藻酸钠、Matrigel 或抑菌圈实验，`mix_gelma_*` 立即沦为荒谬的专用遗留字段。 |
| **同一套四则稀释算术拆成 6+ 个专用计算器**<br>`crates/asterlab-core/src/calculators.rs:12-749`<br>`src/cli/dispatch.rs:331-480` | 除 `dilution@1` 外，单独开发了 `rna_reagent_usage@1`（[modules.json](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json) `L912-1028`）、`gel_mastermix`、`cell_seeding`、`sample_normalize_batch`、`in_vivo_dosing`、`calculator.biofilm_cfu`。 | 计算器可插拔性建在了**实验名词级**而非**公式能力级**；每换一种试剂配制就要改 Rust Core 与 JSON Schema。 |
| **Assay 读数网格写死仅支持 3 个实验名**<br>[layouts.go](../../prototypes/mygo-run/internal/assays/layouts.go) `L34-64`<br>[schema.go](../../prototypes/mygo-run/internal/assays/schema.go) `L140-308`<br>`crates/asterlab-app/src/lib.rs:191-225` | `layoutRegistry` 只认识 `cck8`、`qpcr`、`western_blot`（其中 `western_blot` 为绕过 `AppCore::save_readouts_batch` 两项白名单，竟硬编码冒用 `SchemaID: "qpcr.sample_gene.cq.v1"`！）。 | 任何其他实验（连默认骨架 [starter.asterml](../../prototypes/mygo-run/starter.asterml) 的 `assay-main` 或 HUVEC 的 `assay-tube-formation`）调 `BuildSchema` 均报 `unsupported assay`，Run 无法生成录入网格。 |
| **Run 执行台面堆叠实验专用表单，旁路了模块渲染**<br>[run_page.go](../../prototypes/mygo-run/run_page.go) `L30-35`<br>`internal/runworkspace/bench.go:293-332`<br>`capture.go:185-219` / `readout.go:295-385` | `a.runWS != nil` 永久短路了原本支持模块渲染的 `runui.View`；`bench.go` 硬摆 12孔×6视野采图条（专读 `per_well_fov_count`）、逗号手敲样本排除框、以及要求实验员**手填 `wb_grade` 和 JSON Schema 文本**的分类读数框；步骤绑定的模块退化为一行静态文本芯片 `moduleChip`。 | Run 不再消费 Design 步骤里的模块组合，而是把过去测过的 4 个实验控件硬拼在同一个台面上。 |
| **记录投影顶层硬编码显微多视野图板**<br>`crates/asterlab-core/src/record_projection.rs:174-204`<br>`notebook_page.go:1309` | `RecordProjection` 顶层写死 `images: ImageSlotMatrix` 与 `fov_per_well`；Notebook 图板写死 `for fov := 1; fov <= 6; fov++` 切换按钮。 | 非显微多视野实验（如涂板菌落照片、抑菌圈单皿照片、qPCR 曲线）无法自然复用通用媒体/表格投影。 |

---

### 症结 2：Design 步骤内部完全不支持「内容模块自由组合」，且强迫所有步骤填同一套死字段

在当前活跃主轨 `asterDesignPage`（[design_page.go](../../prototypes/mygo-run/design_page.go) `L181-732`，所有 UI 新建/导入的设计 100% 走此路径）中：

1. **「流程」Tab 强迫每一步只填同样的 4 个单行文本框**：
   - 右侧抽屉 `stepPropertiesDrawer`（`design_page.go:645-732`）写死只有 **标题**、**操作摘要（仅改 `tasks[0].title`，若初始无 `tasks` 连框都不显示）**、**检测 ID**、**时间点** 4 个文本框；甚至没有“新建步骤”按钮，且原本 1,040 行的节点画布 `prototypes/mygo-run/design_canvas.go` 调用次数为 **0**（已成孤岛死代码，现已移除）。
2. **「文档」Tab 把所有复合结构封死为 `"read-only"`**：
   - `GenericModuleEditor`（[view.go](../../prototypes/mygo-run/internal/asterdoc/view.go) `L949-1010`）对 `checks`、`tasks`、`uses`、`table`、`object-list`、`well-map`、`artifact` 全部直接打印静态灰字 `"read-only"`，且 UI 内没有 `.asterml` 源码编辑器。
3. **AsterML 语法本身缺少「步骤级内容块容器」**：
   - 当前 `.asterml` 把所有模块平铺在文档顶层，`step@3` 只是扁平字典，只能通过 `uses: [{ref, role}]` 外链资源，**无法在某个步骤下按任意顺序自由排列「参数表 → SVG 示意图 → 稀释计算器 → 检查清单 → 计时器」**。

---

### 症结 3：仓库内并存「五套设计/模块模型」，造成多处主链路断裂

```mermaid
flowchart LR
    M1["1. AsterDoc (.asterml 源码)"] -->|"仅在 CreateRun 时单向有损编译"| M2["2. Classic Design JSON (+ mygo 侧车)"]
    M1 -.->|"step@3 强制外链预建 UUID"| M3["3. SQLite ProtocolBlock (+ OperationSpec v2)"]
    M4["4. Rust ComponentFramework (ViewDocument)"] -.->|"仅接旧 Web 孤岛"| WEB["Legacy Web (已退役)"]
    M5["5. Go internal/blocks (mygo.instances)"] -.->|"仅接旧 JSON 画布"| LEG["Legacy Designer (架空)"]
```

五套模型并存直接引发了当前主线上的 **4 个严重断层**：
1. **「由实测派生新版 Design」在主轨 100% 失败**：Notebook 点击保存派生设计时调 `SaveDesign`，被 Rust Core `revise_design_record`（`src/cli/design_run.rs:324-335`）直接报错拦截（*“该设计由 AsterDoc 文档管理”*）。
2. **静态 `.asterml` 无法独立运行**：[huvec-tube-formation.asterml](../../docs/sdk/huvec-tube-formation-case/huvec-tube-formation.asterml) `L335-396` 被迫塞入 7 段 `{{protocol_block_version:...}}` 模板占位符，必须靠外部 Python 脚本先写库拿 UUID 再做字符串替换才能创建 Run。
3. **顶部预算条与多阶段计时编辑在 AsterDoc 主轨失效**：`asterDesignPage` 调 `designer.BudgetBar` 读的是旧 JSON `Nodes`，导致 AsterDoc 设计恒显示 `0 步骤`；`CLOSE-05` 的计时编辑器也只挂在旧轨画布上。
4. **`Tools` 页 5 个工具与主流程断联**：除 Pantry 台账外，`Tools` 页的稀释计算器、参数表、96 孔板等回调 `UseInDesigner` / `CreateDraft` 全为 `nil`（`main.go:2755-2783`）。

---

## 四、对照你的「三层模块体系」：逐层现状审计与关键架构补强

### 4.1 三层能力完整度对照表

| 层级 | 模块 / 原语 | 当前代码真实状态（`origin/main` 审计） | 改造动作 |
| :--- | :--- | :--- | :--- |
| **第一层**<br>通用内容模块 | **富文本 / 提示块 (`prose` / `callout`)** | 顶层支持 Markdown，步骤内仅单行 `instruction` 文本 | 补齐步骤内富文本段落与注意事项块 |
| | **参数表 (`param_table`)** | **缺失**（AsterML 无此模块；旧 `blocks` 孤岛未打通） | **新建核心模块**：Design 填计划值+单位，Run 可选记录实际值 |
| | **通用表格 (`data_table`)** | **缺失**（`GenericModuleEditor` 对 `table` 只读；无 CSV 粘贴） | **新建核心模块**：支持自定义列类型/单位、CSV 粘贴、公式列、Run 填数 |
| | **图片 / SVG (`figure`)** | **完全缺失**（Design 无插图模块；Run 图库不解码缩略图） | **新建核心模块**：支持 PNG/JPG/SVG 引用与预览缩放，严守不承载结构化数据边界 |
| | **检查清单 (`checklist`)** | 半残缺（`step.checks` 在 UI 只读，不支持数值范围规则） | 升级为独立可穿插模块，支持勾选确认与简单阈值提示规则（如 `30..300`） |
| | **计算器 (`calculator`)** | 分裂为 6+ 个实验专用计算器 | **收口为单一通用计算器模块**：支持标量配方模式 + 逐样本动态加样表模式 |
| | **计时器 (`timer`)** | Core `timer.rs` 引擎成熟，但 AsterML 无模块声明 | 新增通用 `timer` 模块（倒计时 / 正计时秒表 / 多阶段循环 / `t0+Δt` 时间窗） |
| | **文件附件 (`attachment`)** | 仅有 Run 期 `asset.link`，无 Design 附件与通用采集槽块 | 统一为通用附件模块（Design 挂参考 SOP/图谱，Run 按步骤或容器坐标采图/挂原始文件） |
| | **子流程 (`subflow`)** | `step.path` 组织节点与 `ProtocolBlock` 双轨割裂 | 支持步骤内直接嵌套子步骤列表，可选引用版本化共享子流程（取消强制预建 UUID） |
| | **标准容器 (`plate` / `rack`)** | `plate@1` 被 HUVEC `mix_*` 污染；`container@1` 重复且未编译 | **剥离 `plate` 内全部 `mix_*` 配方字段**；合并 `plate` 与 `container`（孔板/管架/单皿/笼位） |
| **第二层**<br>通用科学数据与逻辑 | **分组 (`group`) & 样本 (`sample`)** | `group@1` 与 `sample_progress.rs`（10 态 + 血缘）**成熟优秀** | 保留 Core 内核；Design 提供**分组表视图**直接增删改分组并自动同步全局 |
| | **试剂 (`material` ↔ `pantry`)** | `pantry.rs` 成熟，但与 AsterML `material@1` 未打通 | 允许参数表/材料引用可选关联 Pantry 批号，保持零强制 |
| | **单位 (`units`) & 公式 (`formula`)** | **分裂为 4 套互不相通的单位表**；`formula` 仅为死字符串 | 合并为**唯一 Core UCUM 单位注册表**（补齐 `% w/v`, `mol/L`, `CFU/mL` 等）+ 安全算术求值器 |
| | **读数契约 (`ReadoutSchema`)** | `readout_schema.rs`（5 类值 + 缺失限定词 + `raw/derived` 溯源）**优秀** | 保留并设为 Run 表格/容器录入的唯一驱动源，**废弃 `assays/layouts.go` 实验名硬编码** |
| **第三层**<br>计划、运行与记录 | **Design → Run → Notebook** | 快照冻结已实现，但**同源模块渲染被 `bench.go` 旁路**，派生写回断裂 | 打通**「同一棵步骤模块树」在 Design（编辑）→ Run（执行态）→ Notebook（记录投影）的同源渲染** |

---

### 4.2 在落地你的方案时，必须守住的 3 条关键架构细化（避免掉入“纯富文本无结构”的新坑）

> [!IMPORTANT]
> **自由组合 ≠ 无语义的 Notion 富文本**。AsterLab 之所以比普通电子实验记录本（ELN）更有价值，在于它能区分计划与事实、追踪样本血缘并驱动结构化计算。因此在重构模块体系时，需要明确以下三条边界：

1. **第一层内容模块内部要明确区分「纯表达块」与「数据绑定块」**：
   - **纯表达块**（`prose` 富文本、`figure` 设计示意图/SVG）：不产生科学实体，Agent 或研究者拖入四象限培养皿手绘 SVG 即可，绝不解析图片文字。
   - **数据绑定块**（分组表、`param_table`、`calculator`、`plate/rack`、`data_table`、`timer`、`checklist`）：它们在步骤中的**出现位置与组合顺序完全自由**，但其内部字段通过显式 ID（`@group`, `@param`, `@sample`, `@readout`）读写 **第二层科学数据模型**。例如手写笔记里的「HAMA、Van、溶菌酶四组」，在界面上是一个自由插入的**分组表模块**，底层写入的是全局 `Group[]` 实体，这样后续的参数表、孔板、计算器和盲法揭盲都能直接引用这四组，而不会变成一张死表格。
2. **彻底终结「AsterDoc vs ProtocolBlock vs Design JSON」五轨脑裂，收敛为单一自包含的 AsterML v2 文档树**：
   - 一份 `.asterml` 文件直接包含 **顶层实验元数据（分组、试剂、容器、读数定义）** + **有序步骤树（每个 `step` 内部直接拥有有序的 `blocks: [...]` 模块列表）**。
   - 取消“每个可执行步骤必须先去数据库建一个 `ProtocolBlockVersion` UUID 才能编译”的反人类约束；共享子流程改为**可选引用**，普通步骤自包含即可一键冻结为 `Run`。
   - 「从实测派生新版 Design」直接生成新修订号的 AsterDoc 源码（替换实测参数/耗时），彻底修复当前 100% 保存报错的断路。
3. **Run 台面从「实验控件大杂烩」回归「冻结模块树的同源运行态渲染」**：
   - 删掉 `bench.go` 中常驻堆叠的专属控件和 `assays/layouts.go` 的实验名白名单。
   - 当实验员在 Run 中点开「步骤 02」时，界面直接按冻结顺序渲染该步骤下的模块列表：
     - `param_table` → 左列显示冻结计划值，右列按需开放「记录实际值/批号」；
     - `figure` → 原样展示设计期的四象限培养皿示意图（只读参考）；
     - `calculator` → 若引用上游实测值（如 Nanodrop 浓度或体重），现场算出逐样本加样表；
     - `timer` → 渲染为可点击启动/暂停的持久化计时条（挂入顶部全局时钟栏）；
     - `checklist` → 渲染为可勾选步骤项或带规则豁免（`skipped_by_rule`）的检查门；
     - `data_table` / `plate` / `attachment` → 渲染为直接写回 Core `execute_command("readout.submit" | "readout.batch" | "asset.link")` 的实测记录区；样本剔除/重做直接作为表格行或孔位的右键/行级状态操作，不再单摆一个逗号输入框。

```mermaid
flowchart TD
    subgraph L2["第二层：通用科学数据内核 (Rust Core 单一权威)"]
        G["分组 Group / 样本 Sample (10态+血缘)"]
        U["统一 UCUM 单位制 + 安全公式引擎"]
        C["通用容器拓扑 Container (孔板 / 1D管架 / 单皿 / 笼位)"]
        R["通用读数契约 ReadoutSchema (数值/表/板/分类/文件 + Raw/Derived)"]
        T["持久化状态机 Timer + 外部指纹资产 AssetLink + Pantry"]
    end

    subgraph L1["第一层：十个通用内容模块 (步骤内任意顺序自由组合)"]
        B1["富文本 Prose"]
        B2["分组表 / 参数表 ParamTable"]
        B3["图片与SVG Figure"]
        B4["通用表格 DataTable"]
        B5["通用计算器 Calculator"]
        B6["检查清单 Checklist"]
        B7["计时器 TimerBlock"]
        B8["文件与采图槽 Attachment"]
        B9["标准容器视图 Plate / Rack"]
        B10["内联/共享子流程 Subflow"]
    end

    subgraph L3["第三层：同源三态工作面 (同一棵步骤模块树)"]
        D["Design 设计态：自由增删重排模块 + 编辑计划值"]
        RUN["Run 运行态：读取冻结模块树 + 原位记录实测/计时/图片"]
        NB["Notebook 记录态：RecordProjection 自动汇总实测对照 + 研判归档"]
    end

    L1 <-->|"显式 ID 引用与受控写入"| L2
    D -->|"不可变冻结 Freeze"| RUN
    RUN -->|"追加式 ExecutionEvent"| NB
    NB -.->|"一键生成新版 AsterML"| D
```

---

## 五、用你的三个实验（抑菌圈 / 涂板 CFU / HUVEC）验证：零专用组件即可完整表达

立下硬规则 **「能用现有通用模块组合实现的需求，不允许新增实验专用模块」** 后，三个实验全部由上述 10 个基础模块拼装完成：

| 实验场景 | 步骤内组合的通用模块序列 | Design 态表现 | Run 态表现（同源渲染） |
| :--- | :--- | :--- | :--- |
| **1. 抑菌圈实验**<br>（HAMA/Van/溶菌酶 + 手绘四象限皿） | `分组表` → `param_table`（OD、浓度、体积） → `figure`（手绘或 Agent 生成的四象限 SVG） → `checklist` → `data_table`（抑菌圈直径 mm） + `attachment`（单皿实拍图） | 自由排列参数、插图与操作清单；SVG 仅作视觉示意，4 组信息存于 `Group[]`。 | 对照四象限示意图加样，勾选清单，在通用表格填入各组各象限直径（mm）并拖入培养皿实拍照片。 |
| **2. 涂板菌落计数 (CFU)**<br>（梯度稀释 + 30–300 判读） | `calculator`（梯度稀释公式） → `subflow`（稀释涂板步骤） → `timer`（培养时间窗） → `data_table`（稀释度、涂板体积、菌落数 + `30..300` 检查规则 + 公式列 `CFU/mL`） | 定义稀释梯度与表格列公式 `count / (dilution * vol)` 及有效计数区间规则。 | 填入原始菌落数，超出 `30..300` 自动提示标记 `below_lod` / `above_uloq`，纯函数自动派生 `CFU/mL` 并保留原始计数溯源。 |
| **3. HUVEC 体外成管**<br>（GelMA 配液 + VEGF 稀释 + 循环消毒 + 162 张显微图） | Step 01: `calculator`（分组配液表，引用 `@plate.group_wells`） + `timer`（3 轮冷热交替） + `checklist`（无气泡豁免）<br>Step 02: `plate(format:12)` + `timer(stopwatch)` + `param_table`（实测固化秒数）<br>Step 04: `calculator`（VEGF 稀释） + `timer(anchor: t0)`<br>Step 05: `attachment`（绑定 `@plate` 坐标，`slots_per_cell: 6`） | 无需 `mix_gelma_*` 专用字段，直接用通用分组配方计算器引用 12 孔板各组孔数。 | 按步骤依次展开循环倒计时、秒表与 `9孔 × 6槽` 通用坐标采图格；Notebook 自动从通用媒体槽生成分组×时间点对比图板。 |

---

## 六、分阶段实施路径回顾（初版归档，核心项均已落地）

> [!TIP]
> **切勿推倒重写 Core！** `asterlab-core` 中的 `ReadoutSchema`、`SampleProgress`、`Timer`、`Assets`、`Pantry`、`RecordProjection` 与 `ProjectionCache` 已经是高质量资产。正确的做法是：**「保住 Core 科学事实内核，做一次外科手术式的去专用化瘦身，重塑 AsterML v2 步骤模块容器与同源渲染层」**。

### Phase 0：工作区与基线整顿（已完成：见安全标签 `v0.1.0-beta-legacy-bench`）
1. 收口当前基线为 `v0.1.0-beta-legacy-bench`，切换干净分支 `feat/v2-generic-modules`。
2. 遵循硬规则：**禁止在 `modules.json`、`calculators.rs`、`assays/`、`runworkspace/` 新增任何绑定具体实验/试剂名称的模块、字段或专用面板**。

### Phase 1：模型统一与「去实验专用化」大扫除（P0 核心已落地）
1. 升级 `step@1` 契约支持有序子块 `blocks[]` 与内联自包含冻结，取消强制预建 `ProtocolBlockVersion` UUID；修复 `persistDerivedDesign()` / `SaveAsterDoc` 派生写回断路。
2. 清理 `plate` 与专用计算器/读数白名单污染，收口为统一 `sdk 2.0.0` 通用模块清单。

### Phase 2 & 3：通用模块渲染、Design 自由编排与 Run/Notebook 同源执行（P0 工作台与核心闭环已落地，细化增强转入明日卡片）
见下方第 7 节至第 10 节。

---

## 七、P0 通用模块基础设施与 UI 工作台已完成交付总览（2026-11-18）

今晚在 `feat/v2-generic-modules` 分支（未推送远端、未建 Kandev 卡片）按计划完成并收敛了 P0 核心基础设施工序与 UI/UX 全面重构，涉及四大层面：

### 7.1 AsterML v2 (`sdk 2.0.0`) 单一通用模块规范与 Rust Core 契约

- **Manifest 统一收敛**：[modules.json](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json) 已收敛为 `sdk 2.0.0`，包含 **21 个模块 / 21 个 kind**（Go/Rust 共享 `20 rules`、`21 port types`、`6 categories`、`20 dispatch arms`），通过 `python3 scripts/sdk/check.py` 双向强校验。
- **核心通用能力模块**：`prose`、`group_table`、`param_table`、`figure`、`data_table`、`checklist`、`calculator`、`timer`、`attachment`、`subflow`、`plate`，且步骤内 `step@1.blocks[]` 支持任意顺序自由组合，亦可引用组织型子节点（[run_read.rs](../../src/cli/run_read.rs) 与 [workflowtree](../../prototypes/mygo-run/internal/workflowtree/tree.go) 已移除 `schema == 3` 硬约束，统一按 `execution == "organizational"` 识别并支持 `block_refs` 解析）。
- **自包含 `step@1` 自动物化**：Run 创建与评估链路（[cli.rs](../../src/cli.rs#L645-L785) `assess_and_run_creation_reject_unmaterialized_steps`、`self_contained_step_v2_auto_materializes_on_run_creation`）已实现无需预先手动生成外部 `ProtocolBlockVersion` UUID 即可直接编译冻结为不可变 Run；并补齐稀释计算器单位异常报错语义对齐与模式单测（[asterdoc.rs](../../src/cli/asterdoc.rs#L2031)、[cli.rs](../../src/cli.rs#L1064-L1102)）。
- **Parity 夹具升级**：更新 [generate_fixtures.py](../../tests/sdk-parity/generate_fixtures.py)（对独立非 `experiment` 夹具自动前置 `experiment-1`）并重生成 [fixtures.json](../../tests/sdk-parity/fixtures.json)，Go/Rust 67 组双向一致性夹具零差异通过。
- **Native Bridge 修复与测试夹具同步**：修复 [modules/runs/rust/src/lib.rs](../../prototypes/mygo-run/modules/runs/rust/src/lib.rs#L523) `pantry_event_notice` 与未使用导入；同步升级 [testdata/rna-case/labworkbench.sqlite](../../prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite) 中的 `asterdoc_version` 与 `run.asterdoc`（`plate` `rack_1d` + `step@1` 及重算 SHA256 `source_hash`）。

### 7.2 Design 工作台重构（复用 `onevra/components` + `mygo/ui/editor`）

- **3-Tab 清晰工作台**：在 [design_page.go](../../prototypes/mygo-run/design_page.go) 重构为三大视图：
  - **Tab 0「步骤与模块画布」**：左侧流程步骤树 + 顶部「实验总览 & 步骤头折叠状态徽章（移除冗余重复步骤头，开启 `HideStepHeader = true`）」 + 中右画布区以通用 `StepBlockComposer` 流式渲染并自由增删重排步骤内任意通用模块；
  - **Tab 1「全局资源定义」**：统一查看与编辑 `group` / `material` / `plate` / `readout` 等跨步骤科学原语；
  - **Tab 2「AsterML v2 源码」**：集成 `mygo/ui/editor` 源码编辑器（含行号、语法对照与脏状态检查），并修复 Windows `\r\n` vs `\n` 换行符标准化导致的假「未保存」问题（`design_page.go:335`）。
- **模态框与设计系统严格合规**：将 [main.go](../../prototypes/mygo-run/main.go#L3761-L3960) 中的预检确认、新建设计、插入模块（提供 10 类 v2 起手模板选项）与导入对话框全面迁至 `theme.GetTokens(c.Theme())`；全仓库 `prototypes/mygo-run/` 扫描 `ui.Hex(` 仅允许存在于 `internal/theme/theme.go` 与 `internal/theme/theme_test.go`。
- **细节视觉打磨（5 项精修）**：
  1. 拓宽 [view.go](../../prototypes/mygo-run/internal/asterdoc/view.go#L1002) `GenericModuleEditor` 字段输入框宽度；
  2. 为 [view.go](../../prototypes/mygo-run/internal/asterdoc/view.go) / [composer.go](../../prototypes/mygo-run/internal/asterdoc/composer.go) / [design_page.go](../../prototypes/mygo-run/design_page.go#L724-L755) / [runworkspace/view.go](../../prototypes/mygo-run/internal/runworkspace/view.go) 统一启用 `HideStepHeader` 消除卡片套娃标题；
  3. 拓宽 [widget_tables.go](../../prototypes/mygo-run/internal/asterdoc/widget_tables.go#L507) `data_table` 列名输入宽度；
  4. 调整四象限培养皿内置 SVG 标注坐标（`viewBox="0 0 420 264"`，将象限文字外移至圆外，避免压线遮挡，见 [inhibition-zone.asterml](../../prototypes/mygo-run/internal/asterdoc/testdata/inhibition-zone.asterml) 与 [v2_modules_test.go](../../prototypes/mygo-run/internal/asterdoc/v2_modules_test.go#L74)）；
  5. 为 [runworkspace/view.go](../../prototypes/mygo-run/internal/runworkspace/view.go#L943) `procedureLayer` 标题头增加换行兜底防止状态胶囊挤换行。

### 7.3 Run 同源执行 & Notebook 通用记录投影

- **Run `ModeRun` 同源渲染**：[internal/runworkspace/view.go](../../prototypes/mygo-run/internal/runworkspace/view.go) 与 [internal/runworkspace/model.go](../../prototypes/mygo-run/internal/runworkspace/model.go) 新增 `FrozenDoc` 与 `parseAsterDocSource`（兼容内联 `blocks[]` 与 `block_refs`），`renderFrozenStepBlocks` 直接复用 `asterdoc.StepBlockComposer`（`ModeRun` + `HideStepHeader = true`）并挂接 Core 实测回调；`plannedDocFOV()`、`recordFOV()`、`FOVPerWellForDoc`、`FOVPerWellForStep` 与 `frozenPlannedWells()` 已解除对显微 6 视野的硬编码上限，自适应单皿/无视野/多视野容器。
- **Notebook 动态图板与 AsterDoc 派生写回**：[notebook_page.go](../../prototypes/mygo-run/notebook_page.go#L1519) 使用动态视野上限（`limit := max(6, a.plannedDocFOV())` / `maxFOV := max(6, max(int(proj.Images.FOVPerWell), a.plannedDocFOV()))`），并在 `persistDerivedDesign()` 中同步调用 `SaveAsterDoc`，打通 Run 实测派生新版 Design 写回闭环。

### 7.4 三组旗舰实验样板 & 权威文档同步更新

- 已在通用模块测试夹具中落盘三组零专用代码的完整实验样板（含手绘风四象限培养皿 SVG、CFU `30..300` 规则与稀释公式、HUVEC 12 孔板循环计时与多视野采图），通过视觉与无头 CLI 全链路测试回归。
- 已在仓库 6 份权威文档同步固化 AsterML v2 架构铁律、模块目录、`AGENT_API` 契约、设计系统组件规范与发布边界（未在仓库内新建冗余 TODO/roadmap 文件，严格遵守 [AGENTS.md](../../AGENTS.md) §1）：
  - [AGENTS.md](../../AGENTS.md)：确立模块体系与组合铁律（禁止新增实验专用模块/字段/UI 面板、`figure@1` 仅作表达、步骤内自由组合与同源渲染）；
  - [asterdoc-contract.md](../../docs/asterdoc-contract.md) & [module-catalog-v1.md](../../docs/sdk/module-catalog-v1.md)：对齐 `sdk 2.0.0` 模块契约、21 个通用注册表项与 `step@1.blocks[]` 自包含规范；
  - [AGENT_API.md](../../AGENT_API.md)：新增 **AsterDoc v2 模块与步骤内编排接口**（补齐 `insert_step_block` / `update_step_block` / `move_step_block` / `remove_step_block`、`asterdoc.inspect.budget` 与 `ReadoutValue::Table` 类型化表格提交及 `.table` 保留 ID 规避规则）；
  - [design-system.md](../../docs/product/design-system.md)：更新 **第五节「AsterML v2 通用模块组件映射表（2026-11-18）」**（涵盖拖拽排序、Lightbox 放大、表格粘贴/导航、`已按实测值换算` / `数据已记录` 徽章规范，以及 `**前端科研文案规范（强制）**`）。
  - [UI-REDESIGN.md](../../docs/sdk/huvec-tube-formation-case/UI-REDESIGN.md)：更新 **第四节「AsterML v2 通用模块基础设施落地状态与后续边界（2026-11-18）」** 为 `MOD-01..MOD-06` 全量收口状态。

### 7.5 `MOD-01..MOD-06` 全量实测联动与 UX 收口（提交 `5c27673`，`+3284/-463` across `24 files`）

在 P0 提交 `b704537` 之上，紧接着完成了原规划的 `MOD-01..MOD-06` 全部工程项：

- **`MOD-01` Run 期实测水合与计算器联动**：
  - [internal/runworkspace/model.go](../../prototypes/mygo-run/internal/runworkspace/model.go) 与 [view.go](../../prototypes/mygo-run/internal/runworkspace/view.go) 解析 Run 事件的 `DataJSON`，完整水合 `actual_parameters`、`measurements`、`result_cells`（已保存表格读数）及样本排除/重做状态（`qc_dropped` → `QC剔除`、`censored_dead` → `删失`、`rework` → `重做`）；
  - [internal/asterdoc/calc_eval.go](../../prototypes/mygo-run/internal/asterdoc/calc_eval.go) 新增 `EvalCalculatorWithActuals`，支持 `dilution` 的 `@actual.*`/`@param.*` `stock_ref` 自动替换、`sample_normalize` 自动过滤已剔除/删失样本并换算 `ng/uL`/`ug/mL`/`mg/mL`（低浓度超出反应体系上限按科研文案标注，见 7.6 节）、以及 `formula` 实测表达式精确变量代入，在界面渲染绿色 `已按实测值换算` 徽章（[widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go)）。
- **`MOD-02` 步骤内拖拽重排与 `data_table` 粘贴/键盘/Core 闭环**：
  - [internal/asterdoc/composer.go](../../prototypes/mygo-run/internal/asterdoc/composer.go) 与 [asterdoc.go](../../prototypes/mygo-run/internal/asterdoc/asterdoc.go) 新增手柄（`lucide.GripVertical`）`ui.Drag`/`DragOver`/`Drop` 与 `MoveStepBlockToIndex`；
  - [internal/asterdoc/widget_tables.go](../../prototypes/mygo-run/internal/asterdoc/widget_tables.go) 实现 TSV/CSV 多单元格剪贴板粘贴（自动跳过中文/英文表头并自动扩行）、`FocusBind`/`OnTextKey` 方向键与 `Tab`/`Enter` 移焦、按 Core `TypedReadoutSubmit` 严格契约构造 `ReadoutValue::Table` JSON 载荷（内置标量 ID 自动后缀 `.table` 避让，分类列保留字符串），提交成功后显示 `数据已记录` 徽章；
  - [internal/notebook/record/projection.go](../../prototypes/mygo-run/internal/notebook/record/projection.go) 与 [notebook_page.go](../../prototypes/mygo-run/notebook_page.go) 新增 `DecodeReadouts` 与 `recordTypedReadoutsSection`，在记录图板原样渲染已保存的结构化表格与分类读数。
- **`MOD-03` 媒体真图异步解码、Lightbox 缩放与文件拖入**：
  - [internal/asterdoc/widget_figure.go](../../prototypes/mygo-run/internal/asterdoc/widget_figure.go) 与 [widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go) 注册 `golang.org/x/image/tiff` 解码器，支持宿主窗口开启后后台线程异步解码（`AsyncDecode` + `mygo.RunOnMain` + `OnAsyncInvalidate`）、`components.Dialog` + `ui.ScrollBoth` Lightbox（`75%/100%/150%/200%` 缩放与系统外部打开），以及 Design 模式下 `FileDragOver`/`DroppedFiles` 直接拖入本地图像或 SVG。
- **`MOD-04` & `MOD-05` Rust CLI 预算、计算器 Parity 与三实验无头端到端**：
  - [src/cli/asterdoc.rs](../../src/cli/asterdoc.rs) 新增与 Go 对齐的 `compute_asterdoc_budget` 与 `eval_asterdoc_calculator_with_actuals`；[src/cli/dispatch.rs](../../src/cli/dispatch.rs) 为 `asterdoc.inspect` 返回 `budget` 并支持按 `module_id` 检索嵌套 `step.blocks[]`，同时在 `asterdoc.command` JSON Schema 注册 `insert_step_block` / `update_step_block` / `move_step_block` / `remove_step_block`；
  - [tests/sdk-parity/run.py](../../tests/sdk-parity/run.py) 补齐 Windows UTF-8 解码与 `.exe` 识别；[src/cli.rs](../../src/cli.rs) 新增 `asterdoc_budget_and_calculator_actuals_parity` 与 `v2_three_flagship_experiments_end_to_end_cli`（抑菌圈 / CFU / HUVEC 完整跑通 `validate → save → command → inspect → create_run → run.workspace → actual_parameters → readout.submit(Table) → record_projection → derive_asterdoc_source_from_run`）。
- **`MOD-06` 扩展视觉门禁与 Windows 构建打包**：
  - [v2_workbench_visual_test.go](../../prototypes/mygo-run/v2_workbench_visual_test.go) 扩充 `960x640` / `2560x1440` 多分辨率、150% Lightbox 缩放弹窗、Run 实测表格保存与合成培养皿实拍图、HUVEC 计算器实测代入以及 Notebook 记录图板表格截图，并同步完成 Windows `Runs.dll`、`mygo-run.exe`、`.zip` 与 NSIS `-setup.exe` 构建产出。

### 7.6 前端科研化表达全面治理（提交 `674139f`，`+203/-92` across `30 files`）

按科研人员视角对整个 MyGo 前端（Design / Run / Notebook / Shell / Protocol / Designer / AgentEdit）进行了全量文案排查与去工程化改写，确保界面不暴露实现层概念：

- **消除后端/架构术语**：全面移除用户可见的 `Core`、`Rust Core`、`后端校验`、`校验成功`、`readout.submit`、`schema ID`、`同源执行`、`insufficient`、`AsterDoc rev`、`OperationSpec v2` 等工程字样，全部转为科研语义：
  - 表格与计算器：`保存本表数据`（原“提交表格读数”）、`数据已记录`（原“已保存至 Core”）、`已按实测值换算 · `（原“已代入实测 · ”）、`浓度偏低，超出单孔反应体积上限`（原含 `insufficient`）。
  - 设计校验与步骤容器：`未命名实验方案` / `实验方案无法读取` / `实验方案核查未完成：` / `方案格式有误` / `方案格式正常`（替代原 `AsterDoc 设计`、`Core 校验失败：`、`AsterML v2 语法有效`）；`本步实验操作与记录` / `%d 项模块`（替代 `步骤模块 · 同源执行` / `%d 个内容块`）；步骤耗时未配置提示改为 `当前步骤尚未配置预计耗时`（[step_list.go](../../prototypes/mygo-run/internal/designer/step_list.go)）。
  - Run / 版本头栏与报错同步：[runworkspace/view.go](../../prototypes/mygo-run/internal/runworkspace/view.go) 条件展示 `冻结方案第 %d 版`，[runui.go#L1128](../../prototypes/mygo-run/internal/runui/runui.go#L1128) 改为 `实验记录 v%d · 方案第 %d 版 · 冻结流程 #%d`；[main.go](../../prototypes/mygo-run/main.go) 将 `"实验数据已保存，但界面同步未完成："` 等状态提示科研化，并同步修正 [recording.go#L771](../../prototypes/mygo-run/internal/runworkspace/recording.go#L771) 的错误分类匹配，避免把“数据已保存但同步失败”误报为 `未保存 · `。
- **原始技术 ID 去噪与语义化映射**：在 [widget_tables.go](../../prototypes/mygo-run/internal/asterdoc/widget_tables.go) 导出 `IsRawIdentifier` 辅助判定，在 Design 卡片与全局块引用、Run 角色标签及 Notebook 记录图板中自动替换为人类可读科研标题（如 `琼脂扩散抑菌圈测定`、`读数`/`计时`/`采图/附件` 等角色标签、`实验测定数据表`/`定性观测记录`/`定量测定读数`、`实测记录步骤`、`待核实：缺水印`）；步骤底层技术 ID 收纳进可折叠的 `步骤高级选项与技术标识`。
- **文档规范与测试门禁固化**：在 [design-system.md](../../docs/product/design-system.md) 正式补充 `**前端科研文案规范（强制）**`，并同步更新 [v2_workbench_visual_test.go](../../prototypes/mygo-run/v2_workbench_visual_test.go) 及相关单测断言以锁死前端零后端术语泄露。

### 7.7 多维正交孔板标注体系 (`plate@1`) 与 RT-qPCR 第四旗舰样板（提交 `4d5391c`）

面向 RT-qPCR、多靶点与多因子交叉铺板场景，在不破坏原有单因子细胞/抑菌圈铺板契约的前提下，为 `plate@1` 补齐正交实验维度、多维着色、批量复孔布板与专用样板：

- **正交因子映射与技术复孔作用域**：
  - 在 [modules.json](../../prototypes/mygo-run/internal/asterdoc/spec/modules.json)、[src/cli/asterdoc.rs](../../src/cli/asterdoc.rs) 与 [src/cli.rs](../../src/cli.rs) 中为 `plate@1` 补充可选 `factors`（`dim -> {well -> level}`）输入/输出，扩展 `clear_wells` 同步清理 `roles` 与 `factors`；将技术复孔唯一性校验收敛至规范因子签名作用域 `(sample_id, canonical_factor_signature, replicate)`，使同一样本（如 `cDNA-C1`）可跨越 `GAPDH` / `VEGFA` / `CD31` 等靶点合理复用 `1..3` 复孔编号（已由 Rust 单测 `plate_factors_scope_replicates_across_targets` 与 Go 单测 `TestPlateFactorsScopeReplicatesAndBatchLayout` 验证）。
- **5 模可视化着色、孔内徽标与放大浮层编辑器**：
  - [internal/theme/theme.go](../../prototypes/mygo-run/internal/theme/theme.go) 新增 `CategoricalPalette`、`CategoricalColor`、`WellRoleColor` 语义调色板；
  - [internal/uikit/plate/plate.go](../../prototypes/mygo-run/internal/uikit/plate/plate.go) 扩展 `WellState.Factors`、`Props.{ColorBy, FactorKey, Blind}`、`WellFillColor`、`RoleLabel`、孔内居中微徽标、内层多维环、自动换行图例及悬停详情 HUD；
  - [internal/asterdoc/{asterdoc.go, modules.go, view.go, composer.go}](../../prototypes/mygo-run/internal/asterdoc/modules.go) 新增 `Plate.Factors`、`ApplyPlateSamplesWithFactors`、`ApplyPlateBatchSamples`、`ApplyPlateFactor`、`ApplyPlateRole`、5 模（`按分组` / `按样本` / `按因子` / `按复孔` / `按孔角色` + 动态因子键胶囊）切换、两行标注工具栏及导出 `RenderPlateOverlay`（在 [design_page.go](../../prototypes/mygo-run/design_page.go) 与 [runworkspace/view.go](../../prototypes/mygo-run/internal/runworkspace/view.go) 统一接入）。
- **RT-qPCR 96 孔多靶点旗舰样板**：升级 [rt-qpcr.asterml](../../prototypes/mygo-run/internal/asterdoc/testdata/rt-qpcr.asterml) 为 3 步骤 96 孔（`GAPDH` / `VEGFA` / `CD31` + `Std` / `NTC`）多因子相对定量方案，并在 [main.go](../../prototypes/mygo-run/main.go) `renderNewDesignDialog` 嵌入提供一键新建。

### 7.8 Notebook ELN 体验重构与通用 v2 模块同源复用（提交 `4d5391c`）

参照 Benchling / Craft / Notion 的科研电子实验记录体验重构 [notebook_page.go](../../prototypes/mygo-run/notebook_page.go)，彻底打通 Design → Run → Notebook 同源组件复用：

- **共享冻结模块水合器与样式收口**：在 [internal/runworkspace/view.go](../../prototypes/mygo-run/internal/runworkspace/view.go) 导出 `HydrateReadOnlyAsterDocView` 与共享 `populateAsterViewState`，并补全 `moduleRoleLabel` 对 `"parameters"` / `"group"` / `"other"` 的中文映射（消除右侧步骤资源标签英文泄露）；在 [internal/asterdoc/{modules.go, widget_tables.go}](../../prototypes/mygo-run/internal/asterdoc/modules.go) 升级 `renderStepSummary`（`theme.Tokens` 配色、解析 `block_refs` 与隐藏原始 ID）、修复只读 `param_table` 单位重复后缀并收窄 `group_table` 最小列宽适配分栏面板。
- **Craft 风格编辑/阅读纸面与双视图右侧伴侣栏**：清理顶栏孤立 `[B]` 按钮；Mode 3 新增 11 类 AsterML v2 通用模块插入下拉菜单（`DefaultStarterBlock`）；Paper 顶栏新增试验标题信头与科研起手模板芯片；Mode 1 优化 Markdown 段落/引用排版；Mode 2 支持直接原位渲染 ```` ```asterlab ```` 通用参数/分组/表格模块卡与 `@matrix(...)` 动态矩阵卡；基于 `components.SplitPercent` 搭建右侧伴侣栏（`时间线` | `实验模块`），直接调用 `HydrateReadOnlyAsterDocView` 完整复用 `group_table`、`plate`、`param_table` 等漂亮组件卡片。
- **双栏卡片化 `图板` 仪表盘**：新增 5 格 KPI 概览条（`步骤进度` / `实测读数` / `采图视野` / `质控排除` / `计时核查`）、代表视野网格、`计划 vs 实际` 中文步骤名核对表、`实验方案与实测模块总览` 与右轨治理卡（`签核与归档` / `派生设计 (源自实测)` / `动态引用 @matrix(...)`）。

### 7.9 全仓视觉与功能补齐：`Tools` 工作台、实验归档导出与首页样板（提交 `4d5391c`）

- **`Tools` 工具台升级与 Design 闭环**：基于 `onevra/components` 与科研中文重构 [internal/tools/tools.go](../../prototypes/mygo-run/internal/tools/tools.go)（储藏室台账、96 孔板速排、稀释配液换算、实验参数表、通用模块模板、结构化数据导入），移除 [main.go](../../prototypes/mygo-run/main.go) `toolsPage` 冗余外层标题，并将 `UseInDesigner` 接通 `applyToolInstanceToDesign` 实现“工具计算/排板 → 一键带入当前设计”。
- **归档导出补充解码表与读数**：在 [internal/protocol/archive.go](../../prototypes/mygo-run/internal/protocol/archive.go) Markdown 与 PDF 导出中追加 `## 实验测定数据与读数` 章节（消费 `proj.Readouts` 并新增 `TestArchiveIncludesDecodedTableReadouts`），同时清理 [protocol_page.go](../../prototypes/mygo-run/protocol_page.go) 残余英文技术提示。
- **首页运行标题与 v2 样板卡片**：在 [internal/shell/shell.go](../../prototypes/mygo-run/internal/shell/shell.go) 将 `"Run · "` 本地化为 `"实验运行 · "`，并在 `homeTemplates` 中接入 v2 旗舰模板。
- **契约与设计文档同步**：同步更新 [design-system.md](../../docs/product/design-system.md)、[asterdoc-contract.md](../../docs/asterdoc-contract.md) 与 [AGENT_API.md](../../AGENT_API.md)。

---

## 八、Light / Dark 实测界面交付巡检图集（视觉门禁已验证）

已通过 [v2_workbench_visual_test.go](../../prototypes/mygo-run/v2_workbench_visual_test.go)（`TestV2WorkbenchVisualGate`）渲染输出并人工读图巡检了全部桌面窗口截图，覆盖浅色 / 深色双主题、`960x640` / `1360x880` / `2560x1440` 三档视口、Lightbox 放大、Run 实测代入与 Notebook 记录图板表格：

### 8.1 P0 核心工作台双主题图集（9 帧）

**1. Design 抑菌圈 Tab 0（Light 主题：步骤模块流 + 四象限皿 SVG + 参数/清单组合）**

![Design 抑菌圈 Tab 0（Light 主题：步骤模块流 + 四象限皿 SVG + 参数/清单组合）](./evidence/v2-visual-catalog/design_iz_tab0_light.png)

**2. Design 抑菌圈 Tab 0（Dark 主题）**

![Design 抑菌圈 Tab 0（Dark 主题）](./evidence/v2-visual-catalog/design_iz_tab0_dark.png)

**3. Design 涂板 CFU Tab 0（Light 主题：通用计算器 + 计时器 + 30–300 规则表格）**

![Design 涂板 CFU Tab 0（Light 主题：通用计算器 + 计时器 + 30–300 规则表格）](./evidence/v2-visual-catalog/design_cfu_tab0_light.png)

**4. Design HUVEC 体外成管 Tab 0（Light 主题：分组配方计算器 + 12 孔板 + 多视野采图槽）**

![Design HUVEC 体外成管 Tab 0（Light 主题：分组配方计算器 + 12 孔板 + 多视野采图槽）](./evidence/v2-visual-catalog/design_huvec_tab0_light.png)

**5. Design 抑菌圈 Tab 1（Light 主题：全局分组/试剂/容器/读数定义视图）**

![Design 抑菌圈 Tab 1（Light 主题：全局分组/试剂/容器/读数定义视图）](./evidence/v2-visual-catalog/design_iz_tab1_global_light.png)

**6. Design 抑菌圈 Tab 2（Light 主题：`mygo/ui/editor` AsterML v2 源码编辑器与换行符无误报状态）**

![Design 抑菌圈 Tab 2（Light 主题：mygo/ui/editor AsterML v2 源码编辑器与换行符无误报状态）](./evidence/v2-visual-catalog/design_iz_tab2_source_light.png)

**7. Design 抑菌圈 Tab 2（Dark 主题：`mygo/ui/editor` AsterML v2 源码编辑器）**

![Design 抑菌圈 Tab 2（Dark 主题：mygo/ui/editor AsterML v2 源码编辑器）](./evidence/v2-visual-catalog/design_iz_tab2_source_dark.png)

**8. Run 抑菌圈同源执行态 ModeRun（Light 主题：冻结模块树原位渲染与胶囊防裁切头栏）**

![Run 抑菌圈同源执行态 ModeRun（Light 主题：冻结模块树原位渲染与胶囊防裁切头栏）](./evidence/v2-visual-catalog/run_iz_mode_run_light.png)

**9. Run 抑菌圈同源执行态 ModeRun（Dark 主题）**

![Run 抑菌圈同源执行态 ModeRun（Dark 主题）](./evidence/v2-visual-catalog/run_iz_mode_run_dark.png)

### 8.2 `MOD-01..MOD-06` 扩展能力实图巡检（8 帧新增高亮场景）

**1. Design Lightbox 150% 放大检视：四象限培养皿 SVG 与缩放/关闭控制栏**

![Design Lightbox 150% 放大检视：四象限培养皿 SVG 与缩放/关闭控制栏](./evidence/v2-visual-catalog/design_iz_figure_zoom_light.png)

**2. Run 抑菌圈实测表格提交：`data_table` 公式列实时求值 + `数据已记录` 徽章（Light）**

![Run 抑菌圈实测表格提交：data_table 公式列实时求值 + 数据已记录徽章（Light）](./evidence/v2-visual-catalog/run_iz_actuals_and_table_light.png)

**3. Run 抑菌圈实测表格提交（Dark 主题）**

![Run 抑菌圈实测表格提交（Dark 主题）](./evidence/v2-visual-catalog/run_iz_actuals_and_table_dark.png)

**4. Run 培养皿实物照片挂接：`attachment@1` 缩略图解码预览**

![Run 培养皿实物照片挂接：attachment@1 缩略图解码预览](./evidence/v2-visual-catalog/run_iz_dish_photo_light.png)

**5. Run HUVEC 12孔板计算器实时代入实测母液浓度：`已按实测值换算 · vegf_stock=200 ug/mL`**

![Run HUVEC 12孔板计算器实时代入实测母液浓度：已按实测值换算 · vegf_stock=200 ug/mL](./evidence/v2-visual-catalog/run_huvec_calc_actuals_light.png)

**6. Notebook 记录图板：解码渲染四象限抑菌圈实验测定数据表（Light）**

![Notebook 记录图板：解码渲染四象限抑菌圈实验测定数据表（Light）](./evidence/v2-visual-catalog/notebook_record_board_table_light.png)

**7. Notebook 记录图板表格读数视图（Dark）**

![Notebook 记录图板表格读数视图（Dark）](./evidence/v2-visual-catalog/notebook_record_board_table_dark.png)

**8. Design HUVEC `2560x1440` 宽屏适配门禁视图**

![Design HUVEC 2560x1440 宽屏适配门禁视图](./evidence/v2-visual-catalog/design_huvec_2560_light.png)

### 8.3 多维孔板 (`RT-qPCR`)、Notebook ELN 与 `Tools` 工作台实图巡检（8 帧，另同步复检 `Run ModeRun` 参数标签修复）

**1. Design RT-qPCR 96 孔多靶点排板：按靶点基因着色 + 孔内微徽标（Light）**

![Design RT-qPCR 96 孔多靶点排板（Light：按靶点基因着色与 5 模/因子维度切换）](./evidence/v2-visual-catalog/design_qpcr_plate_multilayer_light.png)

**2. Design RT-qPCR 96 孔多靶点排板（Dark 主题）**

![Design RT-qPCR 96 孔多靶点排板（Dark 主题）](./evidence/v2-visual-catalog/design_qpcr_plate_multilayer_dark.png)

**3. Design RT-qPCR 放大孔板编辑浮层：按样本复孔着色 + 批量复孔与自定义因子标注栏（Light）**

![Design RT-qPCR 放大孔板编辑浮层（Light：样本复孔着色与多维批标注栏）](./evidence/v2-visual-catalog/design_qpcr_plate_overlay_light.png)

**4. Notebook ELN 双分栏工作面：左侧 Markdown 研判 + 同源参数卡，右侧 `实验模块` 伴侣栏（Light）**

![Notebook ELN 双分栏工作面（Light：同源参数卡与右侧冻结分组/培养皿模块）](./evidence/v2-visual-catalog/notebook_eln_split_light.png)

**5. Notebook ELN 双分栏工作面（Dark 主题）**

![Notebook ELN 双分栏工作面（Dark 主题）](./evidence/v2-visual-catalog/notebook_eln_split_dark.png)

**6. Notebook 双栏卡片化 `图板`：5 格 KPI 条 + 视野矩阵 + 汉字步骤 `计划 vs 实际` + 右轨治理卡（Light）**

![Notebook 双栏卡片化图板（Light：5格KPI指标条与计划实测对比卡）](./evidence/v2-visual-catalog/notebook_record_board_table_light.png)

**7. Notebook 双栏卡片化 `图板`（Dark 主题）**

![Notebook 双栏卡片化图板（Dark 主题）](./evidence/v2-visual-catalog/notebook_record_board_table_dark.png)

**8. `资料与工具` 工作台：`onevra/components` 边栏 + 储藏室台账与纯科研中文表达（Light）**

![资料与工具工作台（Light：onevra 边栏与储藏室台账列表）](./evidence/v2-visual-catalog/tools_workbench_light.png)

**9. `资料与工具 -> 孔板速排`：先选孔板 8 种规格 → 复用悬浮单框选孔标注 + SVG/PNG 导出 + 两列 `孔板历史` 组件墙（Light）**

![孔板速排工作台与两列孔板历史（Light：8规格先选后排、悬浮单框选孔、SVG/PNG导出与点击载入缩略历史）](./evidence/v2-visual-catalog/comp_15b_tools_plate_legacy.png)

---

## 九、全量验证账本与 Windows 构建校验（`feat/v2-generic-modules`）

> [!IMPORTANT]
> 下表所有条目均在最后一次代码改动完成后重新实测通过；鉴于宿主工作目录 `D:\` 存在历史外部沙盒遗传的 NTFS `Low Mandatory Level` 属性，Rust 构建命令统一通过 Cargo 官方环境变量 `CARGO_TARGET_DIR=%TEMP%\asterlab-cargo-target` 与 `CARGO_BUILD_BUILD_DIR=%TEMP%\asterlab-cargo-build`（以及 `CARGO_PROFILE_DEV_DEBUG=0`、`CARGO_PROFILE_TEST_DEBUG=0`）在标准 Medium-IL `%TEMP%` 编译并运行全部测试二进制。

### 9.1 全量测试与契约门禁

| 验证维度 | 命令 | 实测输出摘要 | 状态 |
| :--- | :--- | :--- | :--- |
| **SDK 契约强校验** | `python scripts/sdk/check.py` | `21 modules, 21 kinds, sdk 2.0.0` · `20 rules, 21 port types, 6 categories and 20 dispatch arms` -> `sdk check: ok` | PASS |
| **Native Hash 守门** | `python scripts/ci/native_generator_hash.py --check` | `runs_native.go` & `mygo.rs` 均 `drift none` · `OK golden` | PASS |
| **Token 颜色约束** | `rg -n "ui\.Hex\(" prototypes/mygo-run --glob "!*_test.go"` | 仅命中 `internal/theme/theme.go`（零调用点泄露） | PASS |
| **Rust Core 全套** | `cargo test --locked -p asterlab-core` | 单元测试 `473 passed; 0 failed` (`0.06s`) + `catalog_drafts` `3 passed` (`0.00s`) + `operation_surface` `6 passed` (`0.00s`) + Doc-tests `2 passed` (`1.12s`) | PASS |
| **Rust App 单测 (`--lib`)** | `cargo test --locked --manifest-path crates/asterlab-app/Cargo.toml --lib` | `89 passed; 0 failed; 0 ignored` | PASS |
| **Rust CLI & 3实验 E2E** | `cargo test --locked -p labworkbench --bins` | `75 passed; 0 failed; 0 ignored` (`1.02s`，含全套 CLI E2E) | PASS |
| **Go / Rust Parity** | `python tests/sdk-parity/run.py` | `cases=67 pass=67 fail=0 differences=0`（`isolated_data_sha256=271714845b115f0c1460eddd3138cd8b3abdb1312ec8acfe3d70824534b0681e`） | PASS |
| **MyGo PureGo 桥接** | `go test -count=1 ./modules/runs` | `ok asterlab-mygo-prototype/modules/runs 4.468s` | PASS |
| **Go Internal 全包** | `go test -count=1 ./internal/...` | 21 个有测试的 `internal/*` 包全部 `ok`（含 `internal/tools 0.523s` 含 `TestPlateStudioWorkflowHistoryAndExport`、`internal/uikit/plate 3.649s` 含 `TestPlateExportSVGAndPNG`、`internal/asterdoc 1.522s`）+ `internal/uikit/workflownode [no test files]` | PASS |
| **Go 完整 E2E & 视觉门禁** | `go test -tags=e2e -count=1 .` | 全包 E2E 与全套实图视觉门禁全数通过，`ok asterlab-mygo-prototype 7.793s` | PASS |

*(全部验证完成后，清理临时构建与测试副产物。)*

### 9.2 Windows 构建与未签名发布产物（当前头提交构建）

| 产物 | 路径 | 体积 | SHA256 |
| :--- | :--- | ---: | :--- |
| Native Rust CDylib | `prototypes/mygo-run/dist/Runs.dll` | 13,794,816 bytes | `213A3FC23113EDDAD82464659C039F30D32CA1580D226E3586740975779A2749` |
| 桌面可执行文件 | `prototypes/mygo-run/dist/mygo-run.exe` | 32,154,624 bytes | `43317BE8DEAC2F38A8EB465BF4AFEC770A8E82A8609B0E3366CA2B72F3337F87` |
| 便携压缩包 | `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64.zip` | 21,884,745 bytes | `2C1E1C780B76E4E1F6D9715A42796A52A2F791E3C97C5D006D784DC4D5D39ABF` |
| NSIS 安装包 | `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe` | 18,122,878 bytes | `B5BE27D27BAEED85AD91B5E294C07B86D059DE73607714538A4340D0CDBED8E3` |

---

## 十、收口现状与分支推送说明

> [!NOTE]
> **分支与远端同步范围**：遵循要求，已在分支 `feat/v2-generic-modules` 归档两份带真实渲染图鉴与相对路径图片证据的权威笔记（[v2-module-visual-catalog.md](./v2-module-visual-catalog.md) 与 [v2-architecture-retrospective-and-evidence.md](./v2-architecture-retrospective-and-evidence.md)）并推送至 `origin/feat/v2-generic-modules` 便于后续更新对照；**未向 `main` 直接推送，未移动 `sdk-v1-rc3`，亦未操作 Kandev 看板**。

### 10.1 交付总清单：核心闭环与组件体验已全部落地

| 任务 / 阶段 | 核心目标 | 完成批次 |
| :--- | :--- | :--- |
| **P0 基础设施** | AsterML v2 (`sdk 2.0.0`) 单一通用模块、3-Tab Design 工作台、Run `ModeRun` 同源渲染与三旗舰样板 | `b704537` |
| **MOD-01** | Run 期实测水合 + `calculator@1` 实测代入 + `qc_dropped`/`censored_dead`/`rework` 联动 | `5c27673` |
| **MOD-02** | 步骤内拖拽排序 + `data_table` CSV/TSV 粘贴、键盘导航与 `ReadoutValue::Table` 回写/Notebook 呈现 | `5c27673` |
| **MOD-03** | `attachment@1`/`figure@1` 多格式异步缩略图解码（含 TIFF）、75%–200% Lightbox 缩放与 Design 拖入 | `5c27673` |
| **MOD-04** | `AGENT_API.md` v2 文档对齐与抑菌圈/CFU/HUVEC 三旗舰实验无头 CLI E2E 回归 | `5c27673` |
| **MOD-05** | Rust/Go `ComputeBudgetSummary` 与多模块 `RecordProjection` 审计回归 | `5c27673` |
| **MOD-06** | `960x640` / `1360x880` / `2560x1440` 多分辨率视觉门禁 + Windows 构建流水线 | `5c27673` |
| **前端科研文案治理** | 全量替换 `Core` / `后端` / `readout.submit` / `schema` / `同源执行` / `insufficient` / 原始 ID 为科研人员自然表达 | `674139f` |
| **多维孔板 + Notebook ELN + 全仓补齐** | `plate@1.factors` 多因子复孔作用域与 5 模着色、RT-qPCR 第四旗舰样板、Notebook ELN 纸面/双分栏 `实验模块` 同源复用与 5-KPI `图板`、`Tools` 工作台、归档解码导出 | `4d5391c` |
| **7 项引擎/交互闭环与 21 模块打磨** | 卡片内联改标题、`.asterml`+`./assets/` 双向回写、`format: 4` 与全自适应孔径（4→384/圆形皿/`rack_1d`）、单输入框悬浮选孔标注（含梯级 `100/2 uM` 与无序复孔重算）、`data_table` 2D 矩阵视图与整板粘贴、`readout.viewer`、全仓统一 `plateui.View`、`实验条件` 默认标题与 17 张组件实图修正 | `18ba9e9` |
| **「孔板速排」工作台与双图鉴 GitHub 归档** | `孔板速排` 先选 8 种规格后排板、完整复用 `asterdoc.View` 拖拽框选/悬浮标注/放大浮层、`RenderSVG` + 2× `RenderPNG` 导出、两列 `孔板历史` 卡片加载/导出/持久化、`GroupLabels` 图例映射及两份带 47 张真实实图的 `docs/sdk/` 归档推送 GitHub | 本次交付 (`feat/v2-generic-modules`) |

### 10.2 后续交互试跑与参考入口

1. **交互式实窗体验**：运行最新构建的 `prototypes/mygo-run/dist/mygo-run.exe`（或安装 `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe`），在左侧进入「资料与工具 → **孔板速排**」体验先选规格 → 框选悬浮标注 → `导出 SVG` / `导出 PNG` → 下方两列「孔板历史」点击载入编辑与 `应用到当前设计`。
2. **本机工作区权限整洁（可选）**：因本机 `D:\L\...` 祖先目录残留了外部历史 `Low Mandatory Level` ACL 继承，若今后希望在本目录不设置 `CARGO_TARGET_DIR` / `CARGO_BUILD_BUILD_DIR` 直接跑 `cargo test`，可由你自己以管理员身份清理该文件夹继承完整性标签。
