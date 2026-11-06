# Agent 设计实验：三类 Protocol 审计

- 日期：2026-11-06
- 范围：为外部 Agent 设计实验的工作流挑选三个典型 Protocol，审计“文字 Protocol → 可视化 View”是否丢信息，以及每一步需要什么组件
- 结论：现有 `workflow + operation + detail slots` 足以表达流程结构；不足以让人**不查资料直接执行**。缺口是执行详情/检查表/原始结果，不是新的科学对象。

## 1. 选择的三个 Protocol

| Protocol | 为什么选 |
|---|---|
| CCK-8 活力/细胞毒性 | 最典型的铺板→处理→孵育→加试剂→读板→扣除空白；覆盖 Plate、Table、Calculator |
| 两步法 RT-qPCR | 样本/提取/QC→RT→qPCR 配置→扩增→Cq 分析；参数、耗材、仪器、对照和原始数据最密集 |
| 静态生物膜 CV + CFU | 培养→清洗→CV→脱色读板，以及分散→稀释→铺板→计数；覆盖有损清洗、分支和两种不可互换的 readout |

## 2. 审计结果

### CCK-8

| 步骤 | 人执行时必须看到 | 当前可承载 | 缺口 |
|---|---|---|---|
| 铺板/预处理/加药 | 细胞类型、板型、体积、密度、预处理/暴露时间、组别与孔位 | workflow、operation、well plate、table | 执行指令、材料/来源、组别与孔位绑定 |
| 空白 | 同板无细胞 medium+CCK-8、空白孔 ID | well plate、table | blank/control 角色与后续扣除绑定 |
| 加 CCK-8 | 试剂身份、10 µL/well 或 10% 体积、目标孔 | operation、well plate | 体积/目标孔/来源的显式字段 |
| 孵育 | 1–4 h、37°C、5% CO₂、避光 | operation | 时间/温度/气体/光照的显式字段 |
| 混匀/检查 | 轻柔混匀、无气泡 | note | 执行检查/验收条件 |
| 读板 | 450 nm，430–490 fallback，≥600 reference，实际 wavelength | table | 采集设置、原始 OD 与孔位链接 |
| 延迟读取 | SDS 或 HCl 可选分支、稳定 24 h | workflow/note | 可选分支与样品状态变化 |
| 计算 | 原始 OD、空白 ID、公式来源 | table、calculator | CCK-8 blank subtraction/viability ToolAction；不能猜通用公式 |

### 两步 qPCR

| 阶段 | 人执行时必须看到 | 缺口 |
|---|---|---|
| 提取/QC | 样本处理、提取 kit/修改、DNase/RNase、浓度/纯度/完整性/抑制 | 结构化 QC 字段、证据/轨迹、SOP 接受条件 |
| RT | RNA 量、体积、引物策略、酶、温度/时间、±RT 对照、cDNA 存储 | 反应配方、热程序、对照结果、cDNA 输出契约 |
| qPCR 配置 | target/amplicon/引物、探针、mix、板/管、通道、孔图、NTC/NRT/标准品/重复 |  assay/oligo 元数据、plate→sample/control/replicate 绑定 |
| 扩增 | 完整 cycling program、仪器/block/channel、采集软件版本 | 热程序、仪器 profile、run/acquisition 契约 |
| Cq 分析 | 原始曲线/Cq、Cq 方法、NTC、specificity、outlier、±RT、reference、efficiency、LOD | 原始/派生结果契约、分析 provenance、claim gate |

### 静态生物膜 CV + CFU

| 步骤 | 人执行时必须看到 | 缺口 |
|---|---|---|
| 培养/接种 | 菌株、预培养、medium、接种比例、体积/well、非处理板、温度/时间、重复、attachment zone | 接种/培养条件、attached/planktonic 两相状态 |
| 清洗 | wash 液体、体积、次数、丢弃浮游相、可能脱附损失；CV 与 CFU 的 wash 变体不同 | lossy wash/fraction loss 契约 |
| CV | 染色浓度/体积/时间、洗/干燥、拍照顺序、control/blank | stain/terminal-state/photo-order |
| 脱色/读板 | 溶剂/体积、时间、λ、blank、原始 OD、生物量语义 | readout/raw OD 契约、biomass ≠ viability |
| 分散 | PBS 交换、well→tube lineage、超声/刮取、kill control、最终体积 | lineage、control gate、residual attached cells |
| 稀释/铺板/计数 | 稀释因子、体积、agar、温度/时间、30–300 计数窗、重复、CFU 公式 | 稀释/CFU readout 契约、原始/派生结果 |

## 3. 共同缺口

1. **执行说明与来源**：`ProtocolPreview.step.instruction` 没有稳定显示在每个步骤详情；`time_scope`、source/version、unresolved 没有形成执行卡。
2. **材料/试剂/仪器**：缺少通用 materials/equipment 视图；现在只能写进 note/table，字段和来源不稳定。
3. **控制与验收**：blank/NTC/±RT/kill control 等没有明确的 control 角色、预期和 actual 记录。
4. **原始 vs 派生**：OD/Cq/CFU 原始数据与派生结果没有分离视图；只能靠表格单元格。
5. **样本 lineage**：well→tube→dilution→plate 没有通用 lineage 展示；plate 不能代替样本关系。
6. **可选分支/人工 gate**：workflow 是 DAG，不是条件执行器；需要显式的“人工检查/通过/阻塞/跳过”展示，不能假装自动执行。

## 4. 最小扩展建议（不改 Scientific Core）

**不新增 assay-specific 组件。** 新增或完善 2 个**展示层**能力：

1. `operation` detail preset：
   - 指令、时间范围、参数值/单位/来源、未解析项、来源版本
   - 由 ProtocolPreview 解析，不在 View 复制科学事实
2. 通用 `checklist/gate` presentation：
   - 人工前置条件、验收项、阻塞/跳过、实际记录
   - 不参与自动执行，不创建新的科学规则

其余继续复用：

- `lab.well_plate`：计划孔位/组别/对照
- `content.table`：materials、equipment、controls、raw readout、derived
- `tool.calculator`：只接已注册 ToolAction；不把 qPCR/CFU 计算硬塞进 reagent total
- `workflow`：顺序/嵌套，不承担条件执行
- `content.note/markdown`：长说明和 caveat

## 5. 设计判断

- View 是 Protocol/Design 的投影；不应把完整 SOP 只存进 View props。
- 人类执行时应从 **Run workspace** 读取同一份执行包：instruction、params、materials、equipment、controls、readout、actual。
- Agent 新建 Draft 时先生成 Protocol/Design/View；人类审阅后冻结；Agent 不能自行创建 Run。
- 文字 Protocol 不应被 View 取代；View 必须保留对原始版本、来源和 unresolved 的引用。

## 6. 待确认

1. 是否接受“View 不复制 SOP，执行时从 Protocol/Run 解析”的边界？
2. 是否批准先实现 `operation` 执行详情 preset + 通用 checklist/gate，而不新增 qPCR/CFU/CV 专用组件？
3. 第一轮真实验收是否按 CCK-8 → qPCR → 生物膜顺序推进？
