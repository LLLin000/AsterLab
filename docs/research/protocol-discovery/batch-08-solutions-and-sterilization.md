# Batch-08 Solutions and Sterilization — Weighing / Dissolution / pH / Filtration / Autoclaving / Storage

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`；没有 `ReviewRecord`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 范围: 基础溶液/缓冲液/培养基配制与灭菌的湿实验步骤与原始约束：称量、溶解/混匀、pH 调节、过滤除菌、高压蒸汽灭菌、分装/储存。至少覆盖一种细胞培养基（S1/S2 DMEM）与一种缓冲液（S3/S4 PBS）。
- 不在范围（out-of-scope，一律非操作）: 下游细胞培养使用、灭菌验证（生物指示剂/过滤器完整性/灭菌柜确认）、稀释换算与浓度计算、统计与生物学解释。
- 规则: 现有仓库记录只用于 Reuse Map 身份对照，不作步骤/数值证据；每条数值/行为证据只引本文件 §1 的直接读来源；称量、溶解/混匀、pH 调节、过滤、高压灭菌、储存各自独立成行，不得合并为一个 "media prep" 模块；灭菌验证无证据不得立为通用操作；不写 PortSpec/Transition 全文；不做代码/UI/DB 变更。
- 禁止声明: 本文件不含任何通用称量精度、通用 pH 目标、通用灭菌循环、通用货架期；出现的所有温度/时间/浓度/孔径均为所引来源的协议绑定值，不得跨协议复制。

## 1. 来源索引

| # | Protocol | 权威类型 | 版本/定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Thermo Fisher Scientific, Cell Culture Media Preparation from Powder（Gibco 培养基/试剂配制通用规程 + 灭菌/添加节） | 厂商手册 | 通用粉末培养基配制步骤 1–7 + Table 1 DMEM 行 + 粉末 DPBS 配制步骤 1–5 + 液体浓缩物配制 + Media sterilization and supplementation 节全文 | 直接读：页面全文转写 |
| S2 | Morimoto Lab, DMEM Media Preparation（p. II.A.12） | 实验室 protocol（任务指定可采纳） | Materials + To prepare 1x liquid medium 步骤 1–8 + 储存注记全文 | 直接读：PDF 全文转写 |
| S3 | MP Biomedicals, PBS Tablets Dulbecco's Formula 1X 技术资料（Cat. 2810301/305/306/307） | 厂商手册 | p.1 Directions for Tablets + Formulation 表全文 | 直接读：PDF 全文转写 |
| S4 | Fadanka / Minette / Mowoh, Preparation of PBS Solution, protocols.io V.1（2022-06-14, Protocol ID 62341） | 开放 protocol（working 状态） | Abstract + Materials + 步骤 1–6（PPE/称量溶解/pH/定容/灭菌储存）全文 | 直接读：PDF 全文转写 |

既有复用对照（只记身份，不复述契约，不作证据）：
R1 `op.liquid_transfer`；R2 `op.sample_aliquot`（conditional，谱系注记）；R3 `op.incubation`（elapsed/条件 only，本批储存不得复用，见 §8 G-7）；R4 `op.centrifugation`（本批未出现）；R5 `op.washing`（本批未出现）；审计 `#24` 解离、`#23` 通用染色、`G-1` 发光读数与本批无关。

发现用检索摘要（`search-summary-only`，非证据）：CSH PBS 1X/10X、CSH LB 液体培养基、CSH Tris-HCl 1M、Merck Millipore 无菌过滤选型（0.22 µm 灭菌级 / 0.45 µm 澄清 / 0.1 µm 支原体截留）的 web 检索摘要仅用于立项定位，不引任何数值；凡数值均以 S1–S4 直接读为准。

---

## 2. P-DMEM-POWDER-GIBCO（S1；通用粉末培养基规程，以 DMEM 高糖为例绑定）

绑定摘要：混合容器取接近终体积；950 mL 蒸馏水（终体积 1 L 语境）；室温（15–30 °C）水、轻柔搅拌，加粉末，不加热； rinsing 包装内残粉并入；按 Table 1 加碳酸氢钠（粉末或 7.5% 溶液）；用 1 N NaOH / 1 N HCl 缓慢调 pH 至比目标工作 pH 低 0.2–0.3（过滤后 pH 回升 0.1–0.3）；蒸馏水定容；立即用 0.2-µm 膜正压过滤进无菌容器。DMEM 高糖粉末行：NaHCO₃ 3.7 g/L 或 49.3 mL/L 7.5% 溶液；工作 pH 7.0–7.4。粉末 DPBS（with Ca/Mg 版）：950 mL 蒸馏水 + 室温轻柔搅拌 + 包装 rinsing；CaCl₂ 单独包装时加 0.1 g/L；定容后立即 0.2-µm 正压过滤（无 pH 调节步骤）。灭菌节：高压灭菌 = 高温高压、冷却至室温后才加添加物（维生素/氨基酸热敏）；已有热敏组分时用过滤；0.2 µm 为灭菌级，可去除细菌及更大微生物。添加物：7.5% 碳酸氢钠、L-谷氨酰胺、抗生素、血清。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 取接近终体积的混合容器 + 950 mL 蒸馏水 | `op.liquid_transfer` 参数化（加水） | 空容器 → 含水容器 | 方法绑定：950 mL（1 L 语境） | S1 Prepare cell culture media powders 步骤 1 |
| 粉末加入室温（15–30 °C）水，轻柔搅拌；不加热；rinsing 包装并入 | method-local（溶解/混匀，独立候选，见 §9）+ 父操作备注（rinsing 手法） | 干粉 + 水 → 未调 pH 混悬/溶液 | 方法绑定：15–30 °C、轻柔、不加热；水量 UNKNOWN（粉末规格按目录号） | S1 步骤 2–3 |
| 按 Table 1 加 NaHCO₃（粉末或 7.5% 溶液；DMEM 高糖 3.7 g/L 或 49.3 mL/L） | `op.liquid_transfer` 参数化（加料）+ method-local（碳酸氢盐形态选择，assay 依赖） | 未调 pH 溶液 → 含碳酸氢盐溶液 | 方法绑定：3.7 g/L / 49.3 mL/L；粉末 vs 溶液为本方法选项 | S1 步骤 4 + Table 1 DMEM 行 |
| 1 N NaOH / 1 N HCl 缓慢调 pH 至目标低 0.2–0.3；过滤回升 0.1–0.3 | method-local（pH 调节，独立候选，见 §9） | 含碳酸氢盐溶液 → 已调 pH 待滤液 | 方法绑定：0.2–0.3 offset、0.1–0.3 回升、1 N 滴定剂；目标工作 pH 7.0–7.4（Table 1） | S1 步骤 5 + Table 1 |
| 蒸馏水定容 | `op.liquid_transfer` 参数化 | 已调 pH 液 → 终体积待滤液 | 方法绑定：至终体积（数值=容器规格） | S1 步骤 6 |
| 立即 0.2-µm 膜正压过滤进无菌容器 | method-local（过滤除菌，独立候选，见 §9） | 待滤液 → 无菌培养基 + 废滤器 | 方法绑定：0.2 µm、正压；膜材质/面积/压力 UNKNOWN | S1 步骤 7 |
| 粉末 DPBS：950 mL + 室温搅拌 + rinsing；CaCl₂ 单独包装时 0.1 g/L；定容后立即 0.2-µm 正压过滤 | 同上（称量/溶解/过滤分立；无 pH 行） | 干粉 + 水 → 无菌 DPBS | 方法绑定：950 mL、0.1 g CaCl₂/L（条件分支）、0.2 µm 正压 | S1 Prepare powdered DPBS 步骤 1–5 |
| 液体浓缩物：100 mL 10X + ~850 mL 水无菌稀释；加 7.5% NaHCO₃；1 N HCl/NaOH 调 pH；定容；分装无菌容器、盖紧、按标签温度存 | `op.liquid_transfer` 参数化（无菌稀释/分装）+ method-local（pH 调节）+ 方法储存约束（非操作） | 10X 浓缩物 → 1X 无菌分装 | 方法绑定：100 mL + ~850 mL、Table 2/3 量；储存温度=产品标签（本页无通用值） | S1 Prepare media from liquid concentrate 步骤 1–5 + Table 2/3 |
| 灭菌后添加（碳酸氢钠/L-谷氨酰胺/抗生素/血清）；高压灭菌后冷至室温才加 | 方法模板内添加规则 + 方法约束（非操作） | 无菌基础培养基 → 完全培养基 | 方法绑定；冷却要求定性（无时间/温度值） | S1 Media sterilization and supplementation 节 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 称量（粉末/盐质量测定） | 本批 S1/S2/S4 + 历史称量语境 | ToolAction（质量读数）+ method-local（天平使用）；不建模块 | §9：原子性可过但 G3 不过（天平技能无抽象收益）；粉末规格量 UNKNOWN（按目录号） |
| 溶解/混匀（搅拌溶解、rinsing、不加热/不过混） | 培养基配制（S1/S2）+ 缓冲液配制（S4） | method-local（独立候选，见 §9）；不并入转移/洗涤 | S1 步骤 2–3/DPBS；搅拌强度/终点判据 UNKNOWN |
| pH 调节（滴定 + pH 计） | 培养基（S1/S2）+ PBS（S4）；S3 片剂无此步 | method-local（独立候选）；不建通用模块 | S1 步骤 5；目标 pH 均为方法绑定；温度依赖 UNKNOWN（§8 G-4） |
| 过滤除菌（0.2 µm 正压） | 培养基配制域内 S1 + S2（§9 G1 ∈域内成立） | method-local；domain-shared deferred，不晋升 | S1 步骤 7；膜材质/完整性测试无证据（§8 G-5）；与 S2 真空路线分支成本未解 |
| 高压灭菌 | S1 仅灭菌节一般陈述（无循环值） | 本协议内为陈述/分支条件，非可复用操作证据 | 无循环参数；不可 autoclave 培养基禁并入（§8 G-5） |
| 分装/储存（无菌容器分装 + 标签温度） | 多家族（R1/R2 已有） | `op.liquid_transfer`（分装）参数化；储存=方法约束非操作 | S1 各规程末步；货架期本页无值（§8 G-6） |

---

## 3. P-DMEM-MORIMOTO（S2；DMEM 高糖 1X 液体配制：粉末 → 真空过滤 → 4 °C 暗存）

绑定摘要：Gibco cat# 12100-046（高糖 + L-谷氨酰胺 + 吡哆辛盐酸盐，无丙酮酸钠、无碳酸氢钠，10×1 L 包装）+ dH₂O + NaCO₃（原文拼写 sic）+ 无菌培养瓶（45 mm 颈）+ Millipore GP Express PLUS 真空 0.22-µm 滤头（150 mL SCGPTO1RE / 500 mL SCGPT05RE，原文型号拼写照录）。1X 配制：量取比目标少 5% 的无菌 dH₂O；室温无菌 dH₂O + 轻柔搅拌加粉（不加热）；rinsing 包装并入；每升加 3.7 g NaCO₃；加 dH₂O 至目标体积、搅至溶解（不过混）；终 pH 应 7.4，调至比工作 pH 低 0.2–0.3（1 N NaOH/HCl 缓慢加、搅拌；过滤回升 0.1–0.3；调后密闭至过滤）；立即 Millipore 真空 0.22-µm 滤头灭菌；4 °C 暗存至多 12 个月（粉末按 Invitrogen 技术服务：4 °C 暗/干至多 36 个月；配好液体 4 °C 暗至少 12 个月）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 量取比目标少 5% 的无菌 dH₂O | `op.liquid_transfer` 参数化 | 无菌水源 → 混合容器 | 方法绑定：−5%（终体积相对值） | S2 步骤 1 |
| 室温无菌 dH₂O + 轻柔搅拌加粉，不加热；rinsing 包装并入 | method-local（溶解/混匀）+ 父操作备注（rinsing 手法） | 干粉 + 水 → 未调 pH 混合液 | 方法绑定：室温（无数值）、轻柔、不加热 | S2 步骤 2–3 |
| 每升加 3.7 g NaCO₃ | `op.liquid_transfer` 参数化（加料；称量见 ToolAction） | 混合液 → 含碳酸氢盐液 | 方法绑定：3.7 g/L（NaCO₃ 原文拼写） | S2 步骤 4 |
| 加 dH₂O 至目标体积，搅至溶解，不过混 | method-local（溶解/混匀）+ `op.liquid_transfer`（定容） | 含盐液 → 终体积待调 pH 液 | 方法绑定：不过混（定性）；终体积=目标 | S2 步骤 5 |
| 终 pH 应 7.4；调至低 0.2–0.3（1 N NaOH/HCl 缓慢搅拌加；过滤回升 0.1–0.3；调后密闭） | method-local（pH 调节） | 待调液 → 已调 pH 待滤液 | 方法绑定：7.4、0.2–0.3、0.1–0.3、1 N | S2 步骤 6 |
| 立即真空 0.22-µm 滤头灭菌 | method-local（过滤除菌） | 待滤液 → 无菌培养基 | 方法绑定：真空、0.22 µm、Millipore 滤头型号（照录） | S2 步骤 7 |
| 4 °C 暗存至多 12 个月（粉末 36 个月 4 °C 暗/干；液体至少 12 个月 4 °C 暗） | 方法储存约束（非操作） | 无菌培养基 → 库存培养基 | 方法绑定：4 °C、暗、12/36 个月（Invitrogen 技术服务转述） | S2 步骤 8 + 注记 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 称量（3.7 g/L NaCO₃；粉末整包量） | 培养基配制（S1/S2）+ 缓冲液（S4） | ToolAction + method-local；不建模块 | S2 步骤 4；天平型号/精度 UNKNOWN |
| 溶解/混匀（轻柔搅拌、不加热、不过混、密闭待滤） | 培养基配制（S1 + S2 独立二家族） | method-local（独立候选）；不晋升 | S2 步骤 2–3/5–6；与 S1 同语义但 G3/G4 不过（§9） |
| pH 调节（7.4 目标 + 0.2–0.3 offset + 1 N 滴定） | 培养基（S1 + S2）+ PBS（S4） | method-local；不建通用模块 | S2 步骤 6；pH 计型号/校准 UNKNOWN |
| 过滤除菌（真空 0.22 µm） | 培养基配制域内 S1（正压 0.2 µm）+ S2（真空 0.22 µm） | method-local；deferred（正压/真空分支未解） | S2 步骤 7；压力/通量/完整性 UNKNOWN（§8 G-5） |
| 分装/储存（无菌瓶 + 4 °C 暗 + 期限） | 多家族 | `op.liquid_transfer`（分装语境）+ 储存约束非操作 | S2 步骤 7–8；开封后稳定性 UNKNOWN |

---

## 4. P-PBS-TABLET（S3；PBS 片剂 → 溶解 → 115 °C 高压 → pH 可调）

绑定摘要：1 片溶于 100 mL 蒸馏水；115 °C 高压 10 min；pH 7.3–7.5，可用稀 HCl/NaOH 调。1X 配方：KCl 200 mg（2.68 mM）、KH₂PO₄ 200 mg（1.47 mM）、NaCl 8000 mg（136.89 mM）、Na₂HPO₄ 1150 mg（8.10 mM)；无钙镁。依据 Dulbecco & Vogt 1954；Paul 1959。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 1 片溶于 100 mL 蒸馏水 | method-local（溶解/混匀：片剂崩解溶解）+ `op.liquid_transfer`（量水） | 干片 + 水 → 待灭菌 PBS | 方法绑定：1 片/100 mL | S3 Directions for Tablets |
| 115 °C 高压 10 min | method-local（高压灭菌，独立候选，见 §9） | 待灭菌 PBS → 无菌 PBS | 方法绑定：115 °C、10 min；压力/排气/装载 UNKNOWN | S3 Directions for Tablets |
| pH 7.3–7.5；稀 HCl/NaOH 可调 | method-local（pH 调节条件分支）+ 方法验收窗 | 无菌 PBS → 可用 PBS | 方法绑定：7.3–7.5、稀滴定剂（无浓度值） | S3 Directions for Tablets |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 溶解/混匀（片剂溶解） | 缓冲液（S3 + S4 独立二家族） | method-local（独立候选）；不晋升 | S3 片剂 vs S4 散盐：崩解机理不同，G2 不过 |
| 高压灭菌（115 °C 10 min） | 缓冲液域内 S3 + S4（§9 G1 ∈域内成立） | method-local；deferred（循环值互不兼容） | S3 115 °C/10 min；装载/验证 UNKNOWN（§8 G-5） |
| pH 验收/微调 | 仅本片剂家族为验收窗（S4 为滴定目标） | method-local；不与滴定调节合并 | 稀酸碱无浓度；pH 计 UNKNOWN |

---

## 5. P-PBS-FROM-SALTS（S4；散盐称量 → 搅拌溶解 → pH 滴定 → 定容 → 高压 → RT/4 °C 存）

绑定摘要：1X 组成 137 mM NaCl、10 mM Na₂HPO₄、1.8 mM KH₂PO₄，pH 7.2–7.4。500 mL 例：NaCl 4.0 g（8.0 g/L）、Na₂HPO₄ 0.71 g（1.42 g/L）、KH₂PO₄ 0.12 g（0.24 g/L)。器具：500 mL 烧杯、100 mL 量筒、蒸馏水、pH 计、天平、称量舟、磁力搅拌子/搅拌器、冰箱、高压釜、500/1000 mL Duran 瓶。流程：PPE + 台面消毒（漂白剂 + 70% 酒精）；天平称盐入称量舟；转入 ≥500 mL 烧杯；100 mL 无菌蒸馏水 rinsing 称量舟并入；旋摇 + 磁力搅拌 ~3 min 溶清；pH 计测，NaOH/HCl 调至 7.2/7.4；无菌蒸馏水定容至 500 mL；转 Duran 瓶高压；冷却、盖紧，室温或 4 °C 冰箱存。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| PPE + 台面消毒（漂白剂 + 70% 酒精）；器具物料齐套检查 | 方法前置（非操作） | — → 就绪工作区 | 方法绑定；消毒接触时间 UNKNOWN | S4 步骤 1 + Safety/Before start |
| 天平称三种盐入称量舟（4.0/0.71/0.12 g per 500 mL） | ToolAction（质量读数）+ method-local（称量绑定） | 盐源 → 已称量盐 | 方法绑定：4.0/0.71/0.12 g；天平精度 UNKNOWN | S4 步骤 2 + 组分表 |
| 转入烧杯；100 mL 无菌蒸馏水 rinsing 称量舟并入；旋摇 + 磁力搅拌 ~3 min 溶清 | method-local（溶解/混匀）+ 父操作备注（rinsing 手法） | 已称量盐 + 水 → 澄清盐溶液 | 方法绑定：100 mL rinsing、~3 min；转速/终点标准（澄清目视） | S4 步骤 2–4（concentration 表） |
| pH 计测，NaOH/HCl 调至 7.2/7.4 | method-local（pH 调节） | 盐溶液 → 已调 pH 溶液 | 方法绑定：7.2/7.4；滴定剂浓度（NaOH Cat #795429 除外）UNKNOWN | S4 步骤 3（Protocol materials） |
| 无菌蒸馏水定容至 500 mL 1X PBS | `op.liquid_transfer` 参数化 | 已调 pH 溶液 → 终体积待灭菌液 | 方法绑定：至 500 mL | S4 步骤 3 |
| 转 Duran 瓶高压灭菌 | method-local（高压灭菌；循环值 UNKNOWN） | 待灭菌液 → 无菌 PBS | 方法绑定：容器（Duran 瓶）；温度/时间/压力 UNKNOWN（原文只写 autoclave） | S4 步骤 4 |
| 冷却、盖紧，室温或 4 °C 存 | 方法储存约束（非操作） | 无菌 PBS → 库存 PBS | 方法绑定：室温 or 4 °C；期限 UNKNOWN | S4 步骤 5 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 称量（三种盐克级称量） | S1/S2/S4 三家族 | ToolAction + method-local；不建模块 | S4 组分表；天平/称量舟/刮刀规格 UNKNOWN |
| 溶解/混匀（磁力搅拌 ~3 min + rinsing） | 缓冲液（S3 + S4）+ 培养基（S1/S2） | method-local；不晋升 | S4 步骤 2；与片剂崩解 G2 不兼容 |
| pH 调节（7.2/7.4 双目标） | S1/S2/S4 | method-local；S3 验收窗不并入 | S4 步骤 3；电极/温度补偿 UNKNOWN |
| 高压灭菌（循环值未写） | S3（115 °C/10 min）+ S4（未写值） | method-local；deferred | S4 步骤 4；循环值 UNKNOWN 即 G2 不可判 → 不晋升 |
| 定容/分装/储存 | 多家族 | `op.liquid_transfer`（定容/转移）+ 储存约束非操作 | S4 步骤 3–5；期限 UNKNOWN |

---

## 6. Reuse Map 汇总（本批）

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 加水/定容/稀释/分装进无菌容器（S1 各规程；S2 步骤 1/5；S3 量水；S4 定容/转瓶） | `op.liquid_transfer` | 参数化复用；绑定体积/容器/无菌语境 |
| 反应/培养基分装到小体积（S1 无菌分装；S2 无菌瓶；S4 Duran 瓶） | `op.sample_aliquot` | 条件复用语境（出现即记谱系；守恒 UNKNOWN） |
| 溶解/混匀（粉末搅拌、不加热/不过混、片剂崩解、磁力搅拌、rinsing） | 无（method-local 独立候选） | 四行各自独立，不并入转移/洗涤；不晋升（§9） |
| pH 调节（offset 滴定、终 pH 目标、片剂验收窗） | 无（method-local 独立候选） | 三种 pH 语义互不合并；不晋升（§9） |
| 过滤除菌（S1 正压 0.2 µm；S2 真空 0.22 µm） | 无（method-local，deferred） | 域内二家族但分支/验证缺口 → 不晋升（§9） |
| 高压灭菌（S3 115 °C/10 min；S4 未写值；S1 仅一般陈述） | 无（method-local，deferred） | 循环值互不兼容 + 验证无证据 → 不晋升（§9） |
| 储存（S1 标签温度；S2 4 °C 暗 12 个月；S4 室温 or 4 °C） | 无（方法约束，非操作） | 不复用 `op.incubation`（§8 G-7） |
| 称量/读数/换算（克级称量、5% 水量、1 N 滴定、片剂计数） | ToolAction + method-local | 一律非操作；天平/pH 计规格 UNKNOWN |
| 灭菌验证（指示剂/完整性/柜确认） | 无证据 | 不建操作（§8 G-5） |

---

## 7. 跨方法公共操作覆盖矩阵

| 公共操作/候选 | P-DMEM-Gibco (S1) | P-DMEM-Morimoto (S2) | P-PBS-Tablet (S3) | P-PBS-Salts (S4) | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer`（加水/定容/分装） | ● 950 mL/定容/正压分装 | ● −5%水/定容/滤入无菌瓶 | ○ 100 mL 量水 | ● 定容 500 mL/转瓶 | 最高复用；全参数化 |
| `op.sample_aliquot`（谱系语境） | ○ 无菌分装 | ○ 无菌瓶分装 | —（单瓶无分样） | ○ Duran 瓶转移 | 守恒 UNKNOWN |
| 称量 ToolAction | ● 粉末/碳酸氢盐 | ● 3.7 g/L | ○ 1 片计数（非称量） | ● 4.0/0.71/0.12 g | 均为方法绑定；不建模块 |
| 溶解/混匀 method-local | ● 轻柔/不加热/rinsing | ● 轻柔/不加热/不过混 | ● 片剂崩解溶解 | ● 磁力 ~3 min/rinsing | 四者机理/约束不同，不合并 |
| pH 调节 method-local | ● offset 0.2–0.3/回升 | ● 7.4 + offset/回升 | ○ 7.3–7.5 验收窗 | ● 7.2/7.4 双目标 | 滴定目标 ≠ 验收窗，不合并 |
| 过滤除菌 method-local | ● 正压 0.2 µm | ● 真空 0.22 µm | —（高压路线） | —（高压路线） | 正压 vs 真空分支未解 |
| 高压灭菌 method-local | ○ 一般陈述无值 | —（过滤路线） | ● 115 °C/10 min | ● 未写值 | 循环值互不兼容 |
| 储存约束（非操作） | ○ 标签温度 | ● 4 °C 暗 12 个月 | —（期限未写） | ● RT or 4 °C（期限 UNKNOWN） | 不复用 `op.incubation` |

●=主路径使用；○=提及/条件分支；—=本批未使用。

四者互不可比：通用粉末规程（多培养基 Table 绑定）/ 单一 DMEM 实验室 SOP（真空滤头 + 确切货架期）/ 片剂缓冲液（崩解 + 固定循环）/ 散盐 PBS（磁力搅拌 + 双 pH 目标）的作用对象与完成判据不同；0.2 µm 正压 ≠ 0.22 µm 真空，115 °C/10 min ≠ 未写值循环，pH offset 滴定 ≠ pH 验收窗。

---

## 8. 缺口清单（不能表达/UNKNOWN；分类按 record-standard §4）

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 粉末规格量 | `method_dependent` | S1 粉末包质量按目录号（Table 1 只给 NaHCO₃ 与 pH）；S2 整包粉末量未写；禁携值跨目录号使用 |
| G-2 | 溶解终点与力学量 | `source_missing` | 轻柔/澄清目视/不过混/≥10×式翻转强度（S1–S4 均无转速、时间除外 S4 ~3 min、澄清标准）；残粉/结块判据为操作者判断 |
| G-3 | pH 测量链 | `configuration_dependent` + `device_state` | pH 计型号/电极/校准/温度补偿 S1–S4 全 UNKNOWN；S4 NaOH Cat #795429 外滴定剂浓度 UNKNOWN；S3 稀酸碱无浓度 |
| G-4 | pH 温度依赖 | `source_missing` | Tris 式温度系数（检索摘要见 −0.03/°C 量级）S1–S4 均未写；本批任何 pH 值不得做温度外推 |
| G-5 | 灭菌分支与验证 | `source_missing` + 方法边界 | 过滤：膜材质/面积/压力/通量/完整性测试 S1–S2 全 UNKNOWN，正压 vs 真空等价性无证据；高压：S4 循环值 UNKNOWN，装载/排气/验证（BI/柜确认）S3–S4 全无；禁立通用"灭菌验证"操作 |
| G-6 | 货架期与添加物依赖 | `method_dependent` | S2 12/36 个月为 Invitrogen 转述值；S1 储存=标签温度；S4 期限 UNKNOWN；L-谷氨酰胺/抗生素/血清加入后货架期 S1–S2 均未给（S1 只列种类） |
| G-7 | 储存 ≠ 孵育 | 模型边界 | 4 °C 暗 / RT / 冰箱存是稳定性保持，不断言生长/合成；一律不复用 `op.incubation`（elapsed/条件 only 亦不断言稳定性） |
| G-8 | 水质与无菌链 | `configuration_dependent` | 蒸馏水 vs 无菌蒸馏水 vs 组织培养级水 S1–S4 混用原文照录；无菌容器/盖/滤器无菌状态为前置假设，无验证值 |
| G-9 | 碳酸氢盐形态与浓缩禁令 | 方法绑定缺口 | 粉末 vs 7.5% 溶液互换当量 S1 Table 1 给出但溶解动力学 UNKNOWN；浓缩配制沉淀风险 S1/S2 只作禁令（S1 粉末禁浓缩语境见检索摘要，S1 直接读内为 Table 语境），无阈值 |
| G-10 | 钙镁与沉淀 | `method_dependent` | S1 DPBS 的 CaCl₂ 0.1 g/L 条件分支；S3 无钙镁；PBS 补钙镁配方（检索摘要 1 mM/0.5 mM）未直接读，不引值；沉淀/复溶条件 UNKNOWN |
| G-11 | 时间锚点 | 锚点缺口 | 配制 `t₀`=定容/pH 完成；过滤/高压为灭菌时钟；储存为独立保持窗。调后密闭至过滤（S2）为防漂移约束，非孵育 |
| G-12 | 范围外（误建模块风险） | `not_applicable` | 临床/动物在体、发酵罐级配制、纯化水系统验证、灭菌柜 PQ、培养基性能测试（促生长试验）、通用"称量/调 pH/灭菌"模块——本批一律不建模 |

---

## 9. 晋升审查结论：零新模块

四问（独立目的/稳定边界/独立确认/有意义复用）+ 晋升四门（G1 复用广度 ≥2 独立方法、域内成立；G2 语义稳定；G3 抽象收益；G4 分支成本）：

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| 称量 | ✓✓✓✓（原子可测） | ✓（S1/S2/S4） | ✓ | ✗（天平技能，无复用收益） | ✓ | ToolAction + method-local；不晋升 |
| 溶解/混匀 | ✓✓✓✓ | ✓（培养基 S1/S2 + 缓冲液 S4，跨域） | ✗（粉末搅拌 vs 片剂崩解 vs 磁力搅拌 effect 不同） | ✗ | ✗ | method-local；不晋升 |
| pH 调节 | ✓✓✓✓ | ✓（S1/S2/S4） | ✗（offset 滴定 vs 验收窗 vs 双目标；端口/完成检查不兼容） | ✗ | ✗ | method-local；不晋升 |
| 过滤除菌 | ✓✓✓✓ | △（S1+S2，同一技术域内成立，跨域无第二家族） | △（0.2 µm 正压 vs 0.22 µm 真空；完整性检查缺失） | ✓ | ✗（分支=新操作/域，膜/压力/设备全 UNKNOWN） | method-local；domain-shared deferred，不晋升 |
| 高压灭菌 | ✓✓✓✓ | △（S3+S4，缓冲液域内成立） | ✗（115 °C/10 min vs 未写值；培养基禁高压后果不同） | ✗ | ✗ | method-local；deferred，不晋升 |
| 储存 | ✗（无独立科学转化意图，稳定性不断言） | — | — | — | — | 方法约束，非操作；禁并入 `op.incubation` |
| 灭菌验证 | 无证据 | ✗ | — | — | — | 不建操作（§8 G-5） |

因此本批六个分立候选全部留在 method-local / ToolAction / 方法约束，不进入公共 Catalog，不写 PortSpec/Transition 全文，不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量：`op.liquid_transfer`（+粉末/缓冲液配制与无菌分装家族）、`op.sample_aliquot`（+无菌分装谱系语境）。

## 10. 核查与状态

- 已验证（直接读，2026-11-05，§1 定位节）：S1 通用粉末步骤 1–7 + DMEM Table 1 行 + DPBS 粉末步骤 + 浓缩物步骤 + 灭菌/添加节；S2 Materials + 步骤 1–8 + 储存注记；S3 片剂用法 + 配方表；S4 组分表 + 步骤 1–6。单人 `agent-verified`，仍需第二人逐字段复核；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：任何高压釜操作手册/灭菌柜确认记录、pH 计校准、转子/滤器映射、per-lot 盐/粉末单、实验室 SOP 改编、试剂批间差、任何已跑数据的派生应用；CSH PBS/LB/Tris 与 Millipore 选型检索摘要一律未转写为证据。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器规格分离（§§2–5 仅能力 + 用过体积作例）；样本跃迁有端口（§§2–5 表 1）；时间锚显式；工作值标协议归属；部分列表标部分；plan/actual/raw/derived 分离（本批只有 plan + 方法约束，无 run 值）；无 JSON；UNKNOWN 永不填零。
