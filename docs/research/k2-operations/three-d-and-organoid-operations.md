# 3D spheroid / organoid boundary operations (research draft)

- 日期: 2026-11-05
- 状态: `research_draft`; NOT SourceChecked. No `ReviewRecord` exists.
- 范围: 仅湿实验 (wet-lab) 生物学操作: 基质/支架处理、换液、球体形成、(湿端)成像采集、活力读数。
  不含临床/患者研究、流行病学、生物信息学、图像分割与分析算法、统计与数据处理管线。
  成像仅记录湿端原始采集;分析 (设门/活死判读/定量算法) 不在此文件定义,归 method/ToolAction。
- 原则: 稀疏边界记录。引用通用操作,不为每种模型发明 protocol;
  凡边界不稳定、一再随细胞系/基质/仪器变化的内容,记为 Custom Step 或缺口,不硬建成原子操作。
- 证据说明: 本任务仅见到检索摘要/二手描述,未拉取全文 PDF;以下所有外部数值与行为
  均标 `search-summary-only`,不得用于 Compatible 判定。凡未核实者一律 UNKNOWN。

## 0. Reuse Map (先查复用)

已有可复用对象仅指 `catalog/operations/*.v1.json` 的 module_id
与 `docs/research/k2-operations/` 的已冻结记录;它们只说明"库里有什么",
步骤与数值证据一律以外部权威来源为准,不复制库内记录作证据。

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 转移细胞悬液/培养基/试剂 (含冷 Matrigel) | `op.liquid_transfer` | 参数化复用;绑定体积/冷操作/宽口吸头/防气泡备注 |
| ULA 板接种单细胞悬液 | `op.liquid_transfer` + `op.sample_aliquot` (分样谱系) | 参数化复用;形成本身不是转移,归复合块 |
| 37 °C / 5% CO₂ 培养、Matrigel 聚合 (gelation) | `op.incubation` | 参数化复用;仅断言 elapsed/温度条件,不推断生长或成胶完成度 |
| 3D 换液 (部分换液、保留球体/类器官) | `op.washing` | **变体需要 (variant needed)**:基础洗涤语义 (保留样本的液体交换) 可用,但必须声明 lossy 风险;残留体积、吸液位置为方法绑定,不能进通用默认 |
| 离心辅助聚集/沉降 (如 ~300 × g 短暂离心) | 通用离心记录 (`cell-culture-and-separation.md`) | 参数化复用;RCF/时间/转子为方法绑定 |
| 球体解离为单细胞悬液 (Accutase/Trypsin/TrypLE/EDTA) | 通用传代/解离复合块 (`cell-culture-and-separation.md`) | 参数化复用;试剂与时间完全方法依赖 |
| 明场/荧光原始图像采集 | 冻结表 #9 荧光图像采集原子 (raw image only) | 直接复用语义;3D 侧只需绑定焦平面/多层采集备注,不另建模块 |
| 计数/活力判读、图像定量、剂量曲线拟合 | — | **ToolAction / method**,不是操作 (record-standard §1.5);本文件不定义 |
| 发光活力读数 (CellTiter-Glo 3D) | `op.absorbance_measurement` | **不能复用**:吸光度 ≠ 发光 (luminescence);读数原子缺失,记为缺口,方法模板内以 Custom Step 占位 |
| 冷链 Matrigel 分装/解冻/ dome 点胶与聚合 | 无完全对应者 | 复合块 + Custom Step (R1);聚合确认仅操作性检查,不建新通用原子除非边界稳定后复审 |

判定小结: 本文件**不新建任何 Atomic Operation 候选**。
唯一候选 (`matrix_embedding` / gelation 单列原子) 未通过四问中的"稳定边界"
(不同基质、浓度、dome/embed/sandwich 做法的输入输出与确认方式均不同),
故按 granularity-freeze 规则降为 Composite Block + Custom Step。

## 1. 通用记录约定

- 每个记录独立 ID 与粒度声明 (§2–§5)。
- `time_scope` 默认 `once`;连续培养中的换液/观察为 `per_timepoint`。
- 所有具体数值 (密度、体积、时间、浓度) 均为**方法绑定示例**,不进入通用操作默认;
  通用侧只固定目的、端口、transition 语义、完成检查类型与失败模式。
- 失败模式与 UNKNOWN 分类 (`source_missing` / `method_dependent` /
  `configuration_dependent` / `device_state` / `not_applicable`) 逐记录列出。

## 2. 记录 R1: 冷基质处理与包埋 (Matrix handling + embedding)

- **ID:** `3d.matrix-embed.v1` (`ResearchDraft`)
- **粒度:** Composite Block = 参数化 `op.liquid_transfer` (冷操作) +
  Custom Step (dome/包埋点胶) + 参数化 `op.incubation` (聚合) ;聚合步骤本身暂为
  Custom Step,不单列原子操作。
- **适用:** Matrigel 类温度敏感基质的 dome / embedded / sandwich 包埋;
  排除非温敏支架 (其处理另行记录,见 §6 缺口)。
- **来源:** S4、S5 (Matrigel 指南与处理要点, `search-summary-only`);
  具体实验锚定 S6 (Bio-protocol, PMC4996657)、S7 (Bio-protocol 4298) 的 dome/包埋做法。

### 2.1 Purpose

将单细胞/隐窝/类器官片段悬液与冰冷液态基质混合,以 dome 或包埋层形式点入培养器皿,
经 37 °C 聚合成支撑 3D 生长的凝胶结构;样本身份保留,物理状态改变
(悬浮 → 基质包埋)。

### 2.2 Ports

| Port | Kind | Required | Notes + source |
|---|---|---|---|
| `cell_input` | input sample | required at run | 细胞/隐窝/类器官片段悬液 (S6、S7 做法, `search-summary-only`) |
| `cold_matrix` | input material | required at run | 冰冷液态 Matrigel;至少 50% 混合物占比 (S4, `search-summary-only`) |
| `culture_vessel` | input container | required at run | TC-treated 器皿;有做法要求 37 °C 预孵过夜 (S4, `search-summary-only`, 方法绑定) |
| `embedded_sample` | output sample | produced | 包埋于凝胶 dome/层中的样本 |
| `overlay_medium` | input material | required at run | 聚合后加的室温培养基,避开 dome 扰动 (S4, `search-summary-only`) |

### 2.3 Sample transitions

| # | From → to | Effect |
|---|---|---|
| T1 | `cell_input + cold_matrix --> embedded_sample (liquid)` | 混合;基质 `consumed`,样本 `state_changed` (悬浮 → 基质悬浮) |
| T2 | `embedded_sample (liquid) --gelation--> embedded_sample (gel)` | 聚合;身份保留,状态改变;完成度需操作性确认 (§2.6),孵育本身不自动断言 |
| T3 | `overlay_medium --> embedded_sample` | 覆盖培养基加入;体积与时机为方法绑定 |

### 2.4 Parameters (全为方法绑定,无通用默认)

| key | 类型/单位 | 必填阶段 | 来源分类与证据 |
|---|---|---|---|
| `matrix_fraction` | fraction | 方法绑定 | S4 称至少 50% Matrigel (`search-summary-only`);具体比例 UNKNOWN (`method_dependent`) |
| `dome_volume` | µL/well | 方法绑定 | S4 称 5–50 µL/well;S7 称按孔板查表 (`search-summary-only`);具体值 UNKNOWN (`method_dependent`) |
| `gel_layer_density` | µL/cm² | 方法绑定 | thick gel / embedded 做法 150–200 µL/cm² (S4, `search-summary-only`) |
| `cold_chain` | text | run | 全程冰上、预冷吸头/管,2–8 °C 过夜解冻;Matrigel 约 10 °C 开始胶凝、22 °C 上快速胶凝 (S5, `search-summary-only`) |
| `polymerization` | temp/time | 方法绑定 | S4: dome 通常 ≥10 min,thick/embedded ≥30 min,37 °C (S4, `search-summary-only`) |
| `bubble_avoidance` | flag | run | 混匀避免气泡 (S4/S7 做法, `search-summary-only`);姿势备注,不进 Custom Step |

### 2.5 Time scope

- 点胶+聚合 `once`;包埋后的换液/传代为 `per_timepoint`,归 R3/缺口 (§6)。

### 2.6 Preconditions / completion checks

- Preconditions: 基质液态且冰冷;器皿为 TC-treated;操作台维持冷链 (S4、S5, `search-summary-only`)。
- Completion (operational, 阻断): dome/胶层成形、未塌陷、未被覆盖培养基冲散
  (S4 "避免扰动", `search-summary-only`)。聚合是否充分**不能**仅由时间断言;
  需人工判断 (dome 形态),属允许的人工判断 (record-standard §1.3)。
- Advisory: 气泡检查。

### 2.7 Instrument requirements (能力类别,不绑型号)

- 可调移液器 + 预冷一次性吸头 (宽口吸头为方法备注, S7 类做法, `search-summary-only`)。
- 37 °C 恒温箱 (聚合用,复用 `op.incubation` 能力约束)。
- 可选加热平台维持点胶温度 (S4, `search-summary-only`, `configuration_dependent`)。

### 2.8 Controls / readout

- Controls: 基质批次/浓度记录为方法对照上下文;本操作不定义对照组。
- Readout: N/A (无仪器读数;形态判断为人工观察,非 readout 原子)。

### 2.9 Failure modes

| 现象 | 原因/影响 | 处理 | 记录 |
|---|---|---|---|
| 提前胶凝/拉丝 | 离冰过久、吸头未预冷 | 废弃重配,检查冷链 | run 记录 |
| Dome 塌陷/被冲散 | 聚合不足、加液冲击 | 延长聚合,沿壁缓慢加液 | run 记录 |
| 气泡包裹样本 | 剧烈混匀 | 轻柔混匀,备注吸头类型 | run 记录 |

### 2.10 Unknowns

- 各模型最优 Matrigel 浓度/占比: UNKNOWN (`method_dependent`)。
- 非 Matrigel 支架 (胶原、合成水凝胶、切片支架): UNKNOWN (`source_missing`),见 §6。
- 聚合充分性的定量判据: UNKNOWN (`method_dependent`);当前仅人工形态判断。

## 3. 记录 R2: ULA 球体形成 (Spheroid formation)

- **ID:** `3d.spheroid-formation-ula.v1` (`ResearchDraft`)
- **粒度:** Composite Block = 参数化 `op.liquid_transfer` (接种) +
  参数化 `op.incubation` (37 °C / 5% CO₂) + 可选离心辅助 + 人工形成判断;
  "形成"本身不是转移或孵育可表达的,故整块为复合,不拆新原子。
- **适用:** ULA 圆底板的 scaffold-free 球体形成;排除需基质 overlay 的做法
  (归 R1 组合) 与 hanging drop (见 §6 缺口)。
- **来源:** S1 (Corning CLS-AN-235 用户指南)、S2 (CLS-AN-446 考量)、S3 (Sigma/Corning protocol 页),
  均为 `search-summary-only`。

### 3.1 Purpose

将单细胞悬液接种至 ULA 表面,经培养使细胞自聚集为单个多细胞球体;
样本身份保留,空间组织改变 (分散 → 聚集体)。

### 3.2 Ports

| Port | Kind | Required | Notes + source |
|---|---|---|---|
| `single_cell_suspension` | input sample | required at run | 健康单细胞悬液;有做法用 40 µm 过滤 (S3, `search-summary-only`, 方法绑定) |
| `ula_plate` | input container | required at run | ULA 圆底板 (96/384/1536-well);共价亲水惰性涂层,防划伤 (S1, `search-summary-only`) |
| `spheroid` | output sample | produced | 每孔单个球体 (单球体/孔为目标,非保证) |
| `medium_only_control` | input sample | 方法绑定 | S3 做法设培养基对照 (`search-summary-only`) |

### 3.3 Sample transitions

| # | From → to | Effect |
|---|---|---|
| T1 | `single_cell_suspension --> ula_plate well` | 接种;体积为方法绑定 (S1: 96 孔 75–200 µL,最大 300 µL;384 孔 25–75 µL;1536 孔 5–10 µL — `search-summary-only`) |
| T2 | `dispersed cells --aggregation--> spheroid` | 聚集;通常 24 h 内形成,部分细胞系需 48 h 或紧密度不足 (S1/S3, `search-summary-only`);时间到不等于形成完成 |
| T3 (可选) | `seeded well --brief_centrifugation--> centered aggregate` | 短暂离心助聚集/居中 (S2, `search-summary-only`);RCF/时间方法绑定, centrifugation 本身复用通用离心 |

### 3.4 Parameters (方法绑定)

| key | 类型/单位 | 证据 |
|---|---|---|
| `seeding_density` | cells/well | S3 测试 40–10 000 cells/well (`search-summary-only`);最优值完全细胞系依赖,UNKNOWN (`method_dependent`) |
| `medium_additives` | text | 甲基纤维素增黏、Matrigel overlay 等按细胞系优化 (S2, `search-summary-only`) |
| `formation_time` | hours/days | 24 h 松散聚集、48 h 多数收紧 (S3, `search-summary-only`);MCF7 等例外;终点 UNKNOWN (`method_dependent`) |
| `residual_volume` | µL | 与 R3 共享;换液残留见 §4 |

### 3.5 Preconditions / completion checks

- Preconditions: 单细胞悬液活力良好;ULA 涂层完好 (禁划伤, S1, `search-summary-only`)。
- Completion (operational + 人工判断,阻断): 每日镜检确认球体形成与紧密度
  (S1/S2, `search-summary-only`);未形成 → 条件分支 (延长培养/离心辅助/换条件),
  属 record-standard §1.3 允许的条件分支。
- 孵育确认仅 elapsed/偏差 (granularity-freeze #5),不推断生长。

### 3.6 Failure modes

| 现象 | 原因/影响 | 处理 |
|---|---|---|
| 多球体/卫星团 | 密度过高、板面扰动 | 优化密度,减少移动 |
| 不聚集/松散 (如 MCF7 类) | 细胞系特性 | 加 ECM overlay / 甲基纤维素 / 短暂离心 (S2, `search-summary-only`) |
| 缺氧核心 | 球体过大、培养过久 | 降密度、缩短培养,平衡终点 (S2, `search-summary-only`) |
| 涂层划伤致贴壁 | 手动操作损伤 | 废弃该孔,沿壁操作 (S1, `search-summary-only`) |

### 3.7 Unknowns

- 各细胞系最优密度/添加剂/终点时间: UNKNOWN (`method_dependent`),必须由实验室 protocol 绑定。
- 球体大小/紧密度定量判据: UNKNOWN;本文件不定义图像分析算法 (范围排除)。

## 4. 记录 R3: 3D 换液 (Partial media exchange, lossy 变体)

- **ID:** `3d.media-exchange.v1` (`ResearchDraft`)
- **粒度:** Atomic variant — 以 `op.washing` 为基础 (保留样本的液体交换),
  声明为 **lossy 变体**:球体/类器官可能被吸走或 dome 被扰动,损失量 UNKNOWN。
- **适用:** ULA 球体孔内部分换液、dome/包埋培养的覆盖培养基更换;
  排除全量换液与传代式重铺板 (归缺口 §6)。
- **来源:** S1、S2 (`search-summary-only`)。

### 4.1 Purpose

在保留球体/类器官 (及 dome 完整) 的前提下更换全部或部分培养基;
与 2D 洗涤目的相同,但保留语义是有条件的,必须显式声明损失风险。

### 4.2 Ports / transitions

| Port | Kind | Required | Notes |
|---|---|---|---|
| `culture_well` | input container | required | 含球体/类器官 ± dome |
| `fresh_medium` | input material | required | 预温培养基为方法绑定 |
| `spent_medium` | output (waste) | produced | 吸出的旧液;体积记账归 protocol 层 |
| `retained_sample` | output sample | produced | 保留的球体/类器官;`state_changed` (培养基环境改变),身份保留 |

Transition effect: `culture_well --partial_exchange--> retained_sample + spent_medium`;
lossy 语义: `retained_sample` 可能发生未知程度损失,不能默认无损
(granularity-freeze #5 规则)。

### 4.3 Parameters (方法绑定)

| key | 证据 |
|---|---|
| `residual_volume` | S1/S2: 换液留 10–20 µL (96/384 孔)、5 µL (1536 孔) (`search-summary-only`);半量换液可减扰动 |
| `aspiration_site` | 沿侧壁吸/排,避开球体与 dome,勿触 ULA 表面 (S1/S2, `search-summary-only`) |
| `frequency` | 按需/每 2–4 天类节奏 (S6/S7 做法, `search-summary-only`);UNKNOWN (`method_dependent`) |

### 4.4 Completion / failure

- Completion (operational): 残留体积符合绑定,球体仍在孔内、dome 未破裂;建议镜检确认。
- Failure: 球体被吸走 (用宽口吸头/留残液, S2, `search-summary-only`);
  dome 破裂 (小心 aspirate,空孔加 PBS 防干, S4, `search-summary-only`);
  交叉污染/蒸发边沿效应为 run 记录项。

### 4.5 与 2D 假设的差异 (本记录核心断言)

1. **不能假设贴壁保留**:2D 洗涤以贴壁为保留机制;3D 无贴壁,保留全靠残留液 + 吸液位置。
2. **不能假设全量换液**:3D 常规为部分/半量换液;全量 aspirate 几乎必丢样本。
3. **不能复用 2D 体积默认**:残留体积是 3D 方法绑定的关键参数,不是可省略的细节。
4. **dome 完整性是额外确认项**:2D 洗涤无此项。

## 5. 记录 R4: 3D 活力读数 (CellTiter-Glo 3D, 方法模板)

- **ID:** `3d.viability-ctg3d.v1` (`ResearchDraft`)
- **粒度:** Method Template — 绑定 3D 裂解型 ATP 发光法的试剂、混合、孵育与读数要求;
  发光读数原子库内缺失,以 Custom Step 占位,不自创读数模块。
- **适用:** ULA 球体/Matrigel 包埋样本的终点总 ATP 活力;排除需空间分辨
  (坏死核心 vs 存活边缘) 与实时动态监测的需求 (见 §6)。
- **来源:** S9 (Promega TM412 技术手册)、S10 (JoVE 59714, 需订阅 — 仅见摘要,访问受限已注明),
  均为 `search-summary-only`。

### 5.1 Purpose 与 Reuse 映射

| 方法步骤 | 复用对象 | 处理 |
|---|---|---|
| 板与试剂回温 (~30 min 室温平衡) | `op.incubation` (保持) | 参数化复用 |
| 等体积加 3D 试剂 | `op.liquid_transfer` | 参数化复用;1:1 比例为方法绑定 (S9, `search-summary-only`) |
| 剧烈混匀 ~5 min 裂解 | 无稳定原子 (3D 裂解混合非普通混匀) | Custom Step;~450 rpm/5 min 为方法绑定 (S9, `search-summary-only`) |
| 室温孵育 ~25–30 min (至 60 min) 稳定发光 | `op.incubation` | 参数化复用 |
| 发光读数 (0.25–1 s/孔) | 库内缺失 | Custom Step 占位;需 top-read luminometer,无滤光片 (S9, `search-summary-only`) |
| 剂量/活力计算 | — | ToolAction,本文件不定义 |

### 5.2 关键方法绑定 (非通用默认,全部 `search-summary-only`)

- 试剂 4 °C 过夜解冻,室温平衡,轻柔混匀防 ATP 污染 (S9)。
- 大/致密球体 (>350 µm) 裂解慢:延长孵育、加强振荡;发光与 ATP/细胞数相关,
  3D 试剂比经典试剂多回收 20–40% ATP (S9 摘要说法,未核全文)。
- Matrigel/基质干扰:设基质对照评估背景与裂解效率 (S9)。
- 读板:白色不透光板;透明板培养则转板读数 (S9)。
- 总 ATP 法无空间分辨;破坏性终点,不做连续监测 (S9)。

### 5.3 与 2D 假设的差异

1. 2D 的 CCK-8/MTT/经典 CTG 条件**不能**直接套用:3D 需更强裂解与更长稳定时间。
2. 2D 吸光度读数模块 (`op.absorbance_measurement`) **不能**表达发光读数。
3. 以 DNA/蛋白归一化的换算为 ToolAction,不在本操作内。
4. 读数解释 (活力定义、抑制剂干扰、ATP spike 对照) 归 method/assay 层。

### 5.4 Unknowns

- 发光读数原子的量程/积分时间/仪器能力: UNKNOWN (`configuration_dependent` + `source_missing`)。
- 各球体尺寸的最小裂解条件: UNKNOWN (`method_dependent`)。
- JoVE 59714 全文步骤: UNKNOWN (订阅墙,本任务未访问)。

## 6. 不覆盖 / 需实验室 protocol (Needs lab protocol)

以下各项**有意不记录步骤**,需具体实验室 protocol 绑定后方可执行;
若反复出现且边界稳定,再按 granularity-freeze 规则提炼,当前一律 Custom Step 或缺口:

1. 具体细胞系接种密度、培养基配方、添加剂、终点时间 (`method_dependent`)。
2. Matrigel 浓度/占比、dome 体积/孔板映射、聚合时间微调。
3. 类器官传代/拆分 (机械吹打 vs EDTA/酶解)、split 比例 (如 1:2–1:4 类做法, S6/S7, `search-summary-only`)、
   传代时机 (坏死核心出现等人工判断)。
4. 球体解离为单细胞悬液的具体试剂与时间;冻存/复苏流程。
5. Hanging drop、磁悬浮、生物打印、微流控芯片等其他 3D 构建方式 — 无来源, UNKNOWN (`source_missing`)。
6. 非 Matrigel 支架 (胶原、合成水凝胶、脱细胞支架) 的处理 — UNKNOWN (`source_missing`)。
7. 共培养、定向分化、移植前 QC — 方法依赖,本文件不覆盖。
8. 3D 培养的成像定量 (大小/圆度/生长曲线算法)、空间活力 (live/dead 设门)、组学终点
   — 属分析/统计管线,范围排除;湿端仅采原始图像。
9. 非终点/实时活力监测 (如 RealTime-Glo 类) — 无来源, UNKNOWN (`source_missing`)。
10. Protocol 库全文: protocols.io `10.17504/protocols.io.ewov1qmqkgr2/v1` (小鼠肠类器官) 仅见摘要,
    全文是否需登录 UNKNOWN;JoVE 59714 需订阅,未访问。引用二者须待全文核实。

## 7. 2D 板假设对照表 (什么不能复用)

| 2D 假设 | 在 3D 中为何失效 | 本文件处理 |
|---|---|---|
| 贴壁 = 样本保留 | 球体悬浮、类器官在 dome 内;无贴壁机制 | R3 强制 lossy 变体 + 残留体积绑定 |
| 全量吸弃 + 重加 | 全量 aspirate 丢样本/冲散 dome | 仅允许部分/半量换液语义 |
| 均匀浓度、可直接计数 | 球体有梯度 (缺氧核心);完整球体不可直接计数 | 计数归 readout/method;R4 测总 ATP,不承诺单细胞数 |
| 2D 活力试剂条件通用 | 3D 裂解不足则 ATP 低估 | R4 独立方法模板,不复用 2D 条件 |
| 吸光度读数模块通用 | 发光 vs 吸光物理量不同 | 明确缺口,不挪用 `op.absorbance_measurement` |
| 孵育时间到 = 培养完成 | 形成/聚合需形态学人工确认 | 完成检查仅 elapsed + 人工判断 |
| 沿壁加液是礼貌问题 | 在 3D 是样本存亡问题 | R1/R3 的吸液位置为阻断性完成检查 |

## 8. 来源表 (Source table, 检索日期 2026-11-05)

| # | 来源 | 定位器 (URL + 章节/表格/页) | 读取程度 | 用途 |
|---|---|---|---|---|
| S1 | Corning Spheroid Microplates 用户指南 CLS-AN-235 | https://www.corning.com/catalog/cls/documents/protocols/CLS-AN-235.pdf ;接种/培养/换液/体积章节 | `search-summary-only` | R2、R3 |
| S2 | Corning 3D 培养考量 CLS-AN-446 | https://www.corning.com/catalog/cls/documents/application-notes/CLS-AN-446_DL.pdf ;形成优化/换液/离心/回收章节 | `search-summary-only` | R2、R3 |
| S3 | Sigma-Aldrich/Corning Spheroid Microplates protocol 页 | https://www.sigmaaldrich.com/US/en/technical-documents/protocol/cell-culture-and-cell-culture-analysis/3d-cell-culture/corning-spheroid-microplates ;密度/时间点/CTG-3D 章节 | `search-summary-only` | R2 |
| S4 | Corning Matrigel Organoid Culture Guidelines for Use | https://www.corning.com/catalog/cls/documents/protocols/Matrigel_Matrix_Organoid_Culture_Guidelines_for_Use.pdf ;解冻/dome/thick/embedded/聚合章节 | `search-summary-only` | R1、R3 |
| S5 | Corning Matrigel 处理要点 (At the Bench) | https://www.corning.com/worldwide/en/products/life-sciences/resources/stories/at-the-bench/top-tips-for-handling-corning-matrigel-matrix.html ;胶凝温度/分装章节 | `search-summary-only` | R1 |
| S6 | Bio-protocol: 小鼠肠干细胞分离/培养/维持 (PMC4996657) | https://pmc.ncbi.nlm.nih.gov/articles/PMC4996657/ ; dome/换液/传代/冻存章节 | `search-summary-only` | R1 锚定、§6 |
| S7 | Bio-protocol 4298: 小鼠结肠/小肠类器官制备培养 | https://en.bio-protocol.org/pdf/Bio-protocol4298.pdf ; dome 体积表/培养/解离章节 | `search-summary-only` | R1 锚定、§6 |
| S8 | protocols.io: 小鼠肠类器官制备培养 v1 | DOI 10.17504/protocols.io.ewov1qmqkgr2/v1 ;全文是否需登录 UNKNOWN | 未读 (仅检索摘要) | 候选锚定,待全文 |
| S9 | Promega CellTiter-Glo 3D 技术手册 TM412 | https://www.promega.com/-/media/files/resources/protocols/technical-manuals/101/celltiter-glo-3d-cell-viability-assay-protocol.pdf ;试剂/混合/孵育/读数章节 | `search-summary-only` | R4 |
| S10 | JoVE 59714: 3D 球体活力/死亡评估 | https://www.jove.com/t/59714/assessing-cell-viability-death-3d-spheroid-cultures-cancer ;需订阅,本任务未访问 | 未读 (访问受限) | R4 候选,待全文 |

## 9. 审核状态

- `review_status: research_draft`。无 `ReviewRecord`,**不声明 SourceChecked**。
- 待第二人复核:全部 `search-summary-only` 条目需逐字段对全文复核;
  R3 的 lossy 变体需确认 typed `loss` effect 是否进入模型 (granularity-freeze §7 未决);
  发光读数原子缺口需独立记录 (`k2-instruments` 侧或新操作候选复审)。
- 审核清单自查:数值均标摘要级来源或 UNKNOWN;方法要求与通用语义分开;
  transition 逐端口列出;时间作用域未从 assay 名推断;无隐含默认;
  部分列表 (如 dome 体积) 未作完整列表;计划/实际/原始/派生分开 (R4 读数为原始,解释归 method);
  AI 预审未写成人工审核;无 JSON 改动 (本次零运行时变更)。
