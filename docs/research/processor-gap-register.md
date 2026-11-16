# Scratch/image 与 Flow/FCS 缺口登记（EXP-16）

- 日期：2026-11-16
- 状态：`research_draft`；不是运行时契约，不替代任何 OperationModule / InstrumentProfile / readout schema。
- 目的：明确 Scratch/image 与 Flow/FCS 当前**可记录边界**、未定义的输入/输出/格式/算法/阈值/验证与禁止推断项，防止后续把未知能力误写成可执行处理器。
- 范围依据：`docs/research/record-standard.md`（记录规范）、`docs/research/coverage-ledger.md`（覆盖账本）、`k2-operations/flow-cytometry-and-immunofluorescence.md`（Flow/IF）、`k2-operations/viability-and-cytotoxicity-methods.md`（活力/毒性）、`k2-operations/wound-healing-and-migration.md`（划痕/迁移）。
- 冲突记录：见 §8（scope 引用的处理器契约路径不存在）。

## 1. 当前可记录边界（湿实验止于原始采集）

两类对象当前均只记录到**湿实验原始采集**为止，下游一律未定义：

| 对象 | 已有方法模板（research_draft） | 记录止于 |
|---|---|---|
| Scratch 划痕 | `MT-SCRATCH-WOUND`（P1：Varankar & Bapat scratch protocol） | 原始 image 序列（`t₀` 创伤 + 同位点时序帧） |
| Insert 间隙 | `MT-INSERT-GAP`（P2/P3：ibidi manual + AN21） | barrier 移除后 `t₀` 起的原始 image 序列 |
| Transwell 迁移 | `MT-TRANSWELL-MIGRATION`（P4：Justus et al. 迁移部分） | 膜上固定/染色后的原始 image，或下室收集液计数 |
| Flow 染色 | `FLOW-PREP`（T3/T4/T5：表面/胞内/活力染色） | 染色后悬液（固定标志已记录），无数据 |
| Flow 采集 | `FLOW-ACQ-ANALYSIS`（分析型采集） | 原始 FCS 事件表 + 通道元数据 |
| Flow 分选 | `FLOW-SORT`（分选型采集，F1 摘要级） | 原始 FCS + 分选布局记录（新生样本带谱系） |
| 固定 IF | `IF-FIXPERMBLOCK` / `IF-STAIN` / `IF-IMAGE-FIXED`（T1/T2） | 封片制备 + 原始荧光 image 帧 |

已有复用结论（沿用，不重建）：划痕/迁移的物理步骤全部由现有操作 + 方法绑定表达（`op.liquid_transfer`、`op.washing`、`op.incubation`、`FLUO_IMAGE` 等）；Flow 染色的通用机械引用 `wash` 与孵育记录。**没有任何新原子操作候选获得批准。**

## 2. Scratch/image 缺口

### 2.1 已知候选承载方式（method-local，不得晋升）

- `CAND-GAP-CREATE`（划痕变体 / insert 移除变体）：方法内部步骤，机制不同（机械杀伤 vs 无杀伤揭除），不得合并为一个公共操作，更不得转为 `OperationModule` JSON。
- `CAND-APICAL-CLEAR`（Transwell 上室棉签擦除）：Transwell 特异的选择性去除，洗涤语义无法表达，但仍是方法内部步骤，不跨方法家族复用。
- 晋升门槛（record-standard §1.6）：≥2 独立方法家族的真实复用证据 + 语义稳定 + 第二人复核；三者全缺，维持 `ProtocolStep::Text` 级别。

### 2.2 缺失的格式 / 字段 / 算法 / 阈值

- **image 输入格式未定**：P1 用 `.lif` 时序采集、P3 用明场/相差单 FOV、P4 用 10×/20× 代表性 FOV；尚无统一的原始 image 文件格式、位深、通道命名与元数据字段约定。
- **image 分割与定量算法未定义**：gap 面积分割、边缘检测、细胞计数等图像分割与图像定量**均无本项目已批准算法**。P1 的分析步骤（Fiji/MTrack2/BioVoxxel/MATLAB/Prism）明确在记录范围之外，不得转写为本项目算法。
- **判定阈值未定义**：何种闭合率/迁移细胞数构成结论、增殖对照的合格线、代表性 FOV 的采样数，均无阈值。P1 要求 10 µg/mL mitomycin C + 血清对照只是实验条件，不是结论阈值。
- **不可比性**：Scratch 缺口、insert 缺口、Transwell 计数三者量纲不同，P2 明确 insert 结果不可直接与划痕比较；任何跨模板归一化公式当前都不存在，不得推断。

## 3. Flow/FCS 缺口

### 3.1 已有边界（沿用）

- 分析（`FLOW-ACQ-ANALYSIS`）与分选（`FLOW-SORT`）不得共用一个默认操作：前者只产出数据，后者分叉出新样本 ID。
- `FLOW-PREP` 的 PI/7-AAD 染色管不得进入任何固定步骤；固定后活力判读作废，重染 FVD。
- 有效 panel 的条件：染料可被**本机构装配**的激光/滤光片覆盖（平台包络不作数）、甲醇暴露 panel 只用 MeOH 耐受染料、聚合物染料配对应 blocker、可固定活力染料先于采集。

### 3.2 缺失的格式 / 字段 / 算法 / 阈值

- **FCS 输出字段未定**：原始 FCS 事件表（FSC/SSC/荧光/激光/增益/时间戳/仪器 ID）只是采集契约的 `data_out` 描述，尚无本项目认可的字段表、版本与校验规则；不得写入 readout schema 或执行事件结构。
- **gating 与 compensation 算法未定义**：设门策略、补偿矩阵计算、FMO 边界判定**均无本项目已批准算法**。single-stain（spillover/补偿）、FMO（门边界）、活力染料（死细胞排除）只是必需对照清单，不是算法；isotype 仅作有限背景参考，永不单独设门。
- **阈值未定义**：阳性率判定线、MFI 比较基线、稀有事件细胞投入量、分选纯度验收标准，均无阈值。T5 公式的稀有事件方程输入是方法依赖的，不携带默认值。
- **按机构配置缺失**（`configuration_dependent`，全部 UNKNOWN）：实际装配的激光线、每通道滤光片、增益/PMT、阈值、分选喷嘴/压力/drop delay、收集装置映射；引用平台包络的计划一律驳回。
- **分选数值全部待验证**：`FLOW-SORT` 的浓度带、事件数、收集液配方、通量、质控限均来自 `search-summary-only` 的 F1，在机构 SOP 与装机表读到之前不得用于计划。

## 4. 输入 / 输出 / 格式总表

| # | 缺口 | 现状 | 状态 |
|---|---|---|---|
| G-01 | image 原始文件格式与元数据字段 | P1/P3/P4 各用各的采集，无统一约定 | 未定义 |
| G-02 | image 分割 / 定量算法 | 无批准算法；外部工具链不在记录范围 | 未定义 |
| G-03 | image 结论阈值（闭合率、迁移数、FOV 采样） | 无阈值 | 未定义 |
| G-04 | FCS 字段表与版本 | 仅有 `data_out` 自然语言描述 | 未定义 |
| G-05 | gating / compensation 算法 | 无批准算法；只有对照清单 | 未定义 |
| G-06 | Flow 阳性 / MFI / 纯度阈值 | 无阈值 | 未定义 |
| G-07 | 按机构装机配置（激光/滤光片/增益/喷嘴） | 全部 UNKNOWN | 待机构装机表 |
| G-08 | 分选浓度 / 事件数 / 收集液 / 通量 | F1 摘要级，未验证 | 待机构 SOP |
| G-09 | 跨模板可比性（划痕 vs insert vs Transwell；CCK-8 vs LDH vs LIVE/DEAD image） | 已知不可比，无归一化公式 | 不得建立 |
| G-10 | 可执行处理器 / readout 类型 / CLI-API 字段 | 本轮非目标，零新增 | 禁止 |

## 5. 不得推断（禁止推断项）

以下各项**不得推断**，违者视为把未知能力写成可执行处理器：

1. 不得推断任何 image 分割、边缘检测、细胞计数算法；原始 image 不是迁移率、不是细胞数。
2. 不得推断任何 gating 门位置、补偿矩阵、FMO 边界数值；对照齐备不等于算法存在。
3. 不得推断任何结论阈值（阳性率、MFI 差、闭合率、纯度线）；实验条件（如 mitomycin C 浓度）不是结论阈值。
4. 不得推断跨模板可比性；不同创伤物理、不同量纲之间不换算、不取平均。
5. 不得推断按机构配置（激光/滤光片/增益/喷嘴/压力）；平台能力包络永不替代装机表。
6. 不得推断试剂批号特异性数值（抗体稀释、FVD 浓度、滴定终点）；以批单证与滴定为准，未读到即 UNKNOWN。
7. 不得推断 `search-summary-only`（B1/F1/S4/S5）中的任何数值为计划依据。
8. 不得推断固定后样本可回培养、PI/7-AAD 管可固定、染色后可回未染色态等违背样本转移终末性的路径。

## 6. 本轮 deferred

- DOE（实验设计优化）、功效分析（power/sample-size）、复杂统计检验、多组学/生物信息流程：本轮全部记为 **deferred**，不进入操作模块、不进入阈值表。
- 图像分析软件链、质谱/测序下游：覆盖账本已列为未覆盖，本轮同样 **deferred**。
- 解冻条件：上述任一方向需先有独立立项、公开指南或厂商手册级别的来源、以及合成/真实数据验证方案，方可另行提案；提案本身不在本卡范围，不得在本卡创建解析器或实现卡。

## 7. 另行提案的验证门槛

任一缺口（G-01–G-08）要从“未定义”转为可提案，必须齐备：

1. **外部资料**：对应断言类型的首选来源（公开 protocol / 试剂盒说明书 / 装机手册 / 本地已审核 SOP，见 record-standard §3.1），且关键参数有逐字段第二人复核记录（`ReviewRecord`）。
2. **机构配置**（Flow/成像）：本机构装机表（激光/滤光片/增益/物镜/相机/喷嘴）+ 质控程序版本，逐项填写 §3.1 的 per-run 配置记录。
3. **合成验证**：用固定夹具走通“原始采集 → 派生计算 → 读数”的完整路径，输出可重现（本轮不建夹具，只定门槛）。
4. **真实验证**：至少一批真实试剂/样本的端到端记录，偏差与失败模式入库；统计类提案另需功效分析前置（仍 deferred，不得并入本提案）。
5. 提案载体是新卡（Contract/Core），不得复用本登记文档直接转实现；算法与阈值在提案通过前仍视为未定义。

## 8. 来源与缺口审核记录（含冲突）

- 已读来源：`record-standard.md`、`coverage-ledger.md`、`k2-operations/flow-cytometry-and-immunofluorescence.md`（T1–T5 直接读，B1/F1 摘要）、`k2-operations/viability-and-cytotoxicity-methods.md`（S1–S3/S6–S9 直接读，S4/S5 摘要）、`k2-operations/wound-healing-and-migration.md`（P1–P4 直接读，B1–B3 阻断）、`contracts/operation-module-v1.md`、`contracts/readout-schema-v1.md`。
- 全部来源条目仍为 `research_draft`；无 `ReviewRecord`，无 `SourceChecked` / `ExpertReviewed` / `LabValidated` 主张。
- **冲突记录**：本卡 `scope.read` 列出 `docs/contracts/experiment-processor-v1.md`，但该路径在仓库中不存在（`docs/contracts/` 下无此文件）。按 Docs 工作方法先记录、不擅改范围：本登记未引用该契约的任何字段；如该契约实际存在于他处，需建卡修正 scope 后再补登记。
