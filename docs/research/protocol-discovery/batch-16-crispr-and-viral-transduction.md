# Batch-16 CRISPR and Viral Transduction — gRNA Delivery / Selection / Single-Cell Cloning / Lentiviral / AAV Transduction

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`；没有 `ReviewRecord`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 范围: 哺乳动物贴壁细胞的湿实验步骤与原始约束：CRISPR gRNA/RNP 递送（脂质体/电穿孔语境）、慢病毒转导、AAV 转导、抗生素筛选、编辑后单细胞克隆分离。只记录湿实验步骤与原始读数；编辑效率、转导效率、存活/单克隆/敲除判定一律记为方法前置或方法门控，不建操作。
- 不在范围（out-of-scope，一律非操作）: 病毒包装/滴度测定（TCID50/p24/qPCR 滴定）、gRNA/载体序列设计、脱靶分析/测序验证、Western/流式设门与定量、图像分割计数与统计、动物在体转导、临床。
- 规则: CRISPR 递送与病毒转导不得合并为一个"递送/转导"模块（输入物、生物安全要求、完成确认互不兼容）；慢病毒与 AAV 不得合并（整合 vs 非整合、MOI 定义域/受体要求、筛选逻辑不同）；复合物配制一律拆为 `op.liquid_transfer`（合并）+ `op.incubation`（复合保持），不立新原子；筛选存活与单克隆/编辑判定是人工/方法门控，不建操作；B09 筛选/有限稀释只作 Reuse Map 身份对照，不复用其数值；每条数值/行为证据只引本文件 §1 的直接读来源；不写 PortSpec/Transition 全文；不做代码/UI/DB 变更。
- 禁止声明: 本文件不含任何通用 MOI、通用 polybrene 浓度、通用抗生素工作浓度、通用电穿孔电压、通用编辑效率；出现的所有体积/浓度/时间/汇合度均为所引来源的协议绑定值，不得跨细胞系/试剂批次/病毒批次复制。

## 1. 来源索引

| # | Protocol | 权威类型 | 版本/定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | IDT Alt-R CRISPR-Cas9 RNP lipofection guide（HEK293/HeLa 贴壁语境；crRNA:tracrRNA 双链 + Cas9 复合 + RNAiMAX 递送节） | 厂商指南 | RNP 复合步骤 + 脂质体转染步骤 + 检测前培养节 | 间接读：公开指南步骤大纲；`search-summary-only`，未逐字段复核 |
| S2 | Addgene lentiCRISPRv2 / Lentiviral transduction + puromycin selection（贴壁细胞慢病毒转导 + 药筛语境） | 开放 protocol（机构维护） | 转导步骤（polybrene 条件分支）+ 换液 + puromycin 筛选 + 维持节 | 间接读：步骤大纲；`search-summary-only` |
| S3 | Addgene AAV transduction guide（AAV2/常用血清型贴壁细胞转导语境；不含包装/滴定） | 开放 protocol（机构维护） | 转导步骤（MOI/血清条件）+ 换液 + 表达观察节 | 间接读：步骤大纲；`search-summary-only` |
| S4 | Synthego single-cell cloning after editing（有限稀释/96 孔单细胞分离语境；编辑后克隆起点） | 厂商指南 | 单细胞悬液制备 + 有限稀释铺板 + 7–14 天克隆生长 + 单集落判定节 | 间接读：步骤大纲；`search-summary-only`；数值以 B09 S5b 为对照，不互相覆盖 |
| S5 | Thermo puromycin kill curve（筛选浓度预实验语境；与 B09 S5a 同类，独立来源） | 厂商指南 | kill curve 分皿 + 系列浓度 + 换液节律 + 存活判读节 | 间接读：步骤大纲；`search-summary-only` |

既有复用对照（只记身份，不复述契约，不作证据）：
R1 `op.liquid_transfer`；R2 `op.sample_aliquot`（conditional，谱系注记）；R3 `op.incubation`（elapsed/偏差 only，不断言转导/编辑/筛选结果）；R4 `op.washing`（preserving；lossy 变体另立）；R5 `op.centrifugation`；R6 细胞浓度计数；R7 `FLUO_IMAGE`（raw 采集 only）。审计 `#12` 活力/效率判定、`#17` 连续稀释、`#24` 解离、`#34` ToolAction 与本批派生（MOI 换算、编辑效率%、kill curve 拟合、Poisson）同类处理。B09 P-STABLE-CLONE 为筛选/有限稀释的家族内对照，不跨批复制数值。

---

## 2. P-CRISPR-RNP（S1；RNP 脂质体递送）

绑定摘要：转染时 60–80% 汇合铺板（来源绑定例；他系另行绑定）。crRNA + tracrRNA 等摩尔退火成 gRNA，双链程序为例 95 °C 5 min 后室温冷却（本来源绑定值；他仪器/体系另行绑定）。gRNA + Cas9 按来源摩尔比室温复合 10–20 min 成 RNP；RNP + RNAiMAX/同类脂质体在 Opti-MEM 中再复合 10–20 min 后加到细胞上。37 °C 孵 24–72 h 后取样做编辑检测（T7E1/测序归下游 out-of-scope）。复合物现配现用；效率/毒性权衡逐系优化。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 按转染时 60–80% 汇合铺板 | `op.liquid_transfer` 参数化 + 方法前置（汇合度门控，非操作） | 悬液 → 已铺板细胞 | 方法绑定：汇合度窗；接种数逐系 | S1 转染前培养节 |
| crRNA:tracrRNA 等摩尔混合退火（95 °C 5 min → RT 冷却为例） | method-local（gRNA 退火，独立候选，见 §7）+ `op.incubation`（退火保持部分） | oligo 对 → 双链 gRNA | 方法绑定：温度/时间/摩尔比；冷链程序归方法 | S1 gRNA 制备节 |
| gRNA + Cas9 室温复合 10–20 min 成 RNP | `op.liquid_transfer`（合并）+ `op.incubation`（复合保持）参数化 | gRNA + Cas9 → RNP | 方法绑定：摩尔比/10–20 min/RT；复合终点判据 UNKNOWN | S1 RNP 复合节 |
| RNP + 脂质体在 Opti-MEM 中 10–20 min 再复合，加到细胞上 | `op.liquid_transfer` + `op.incubation` 参数化 | RNP + 脂质体 → 可转染复合物 → 已递送培养 | 方法绑定：两段时钟各自绑定；脂质体配方 proprietary | S1 转染节 |
| 37 °C 孵 24–72 h 后取样检测 | `op.incubation` 参数化 + 方法终点（取样为 `op.sample_aliquot`，判定归下游） | 已递送培养 → 待测样品 | 方法绑定：24–72 h；T7E1/测序 out-of-scope | S1 培养/检测节 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（铺板/合并/加复合物） | 多家族（R1） | shared（`op.liquid_transfer`） | S1 全流程；仅绑定体积/顺序 |
| 复合保持（gRNA–Cas9；RNP–脂质体两段） | S1 内两段 + B09 S1/S2/S3 同语义 | `op.incubation` 参数化复用；不立"RNP 形成"新原子 | 两段时钟不合并通用值（§7） |
| 转染后保持（24–72 h） | 多家族（R3） | `op.incubation`；编辑结果不在内 | 完成=elapsed only |
| gRNA 退火（95 °C 程序） | 仅 CRISPR 家族 | method-local（独立候选）；不晋升 | 单家族；程序逐体系（§7） |
| 编辑效率判定（T7E1/测序/表型） | —（方法门控/下游） | 非操作 | 读数与分析 out-of-scope（§6 G-5） |

---

## 3. P-LENTI（S2；慢病毒转导 + puromycin 筛选）

绑定摘要：转导前一天低密度铺板，转导时 30–60% 汇合（来源绑定例）。病毒按 MOI（来源例 0.5–5；仅本来源绑定，不得通用化）加到含 polybrene（来源例 4–8 µg/mL；仅条件分支，他系另行杀伤预试）的培养基中；spinfection 为可选分支（来源例 800–1000×g 30–60 min；不做默认）。转导后 12–24 h 换正常培养基；48 h 后进 puromycin 筛选（浓度由 S5 kill curve 逐系×逐批号定）。维持 3–7 天至对照全死后扩增。BSL-2 操作约束为方法前置，不建模。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 前一天低密度铺板，转导时 30–60% 汇合 | `op.liquid_transfer` 参数化 + 方法前置 | 悬液 → 已铺板细胞 | 方法绑定：汇合度窗 | S2 转导前节 |
| 病毒按 MOI 稀释加入（含 polybrene 条件分支 4–8 µg/mL 例） | `op.liquid_transfer` 参数化 + ToolAction（MOI→体积换算） | 病毒储备 + 培养基 → 转导培养 | 方法绑定：MOI/体积/polybrene 三者逐批；滴度值归病毒 lot | S2 转导节 |
| spinfection 可选分支（800–1000×g 30–60 min 例） | `op.centrifugation` 参数化（条件分支，非默认） | 转导培养 → 已离心转导培养 | 方法绑定；RCF/时间/温度逐系 | S2 可选节 |
| 12–24 h 后换正常培养基；37 °C 保持至 48 h | `op.liquid_transfer`（换液）+ `op.incubation` 参数化 | 转导培养 → 恢复培养 | 方法绑定：换液时钟 | S2 转导后节 |
| 48 h 后进 puromycin 筛选 3–7 天，对照全死为终点 | `op.liquid_transfer`（换药）+ `op.incubation`（筛选保持）+ 方法门控（存活，非操作） | 恢复培养 → 筛选中培养 → 存活池 | 方法绑定：浓度来自 S5；3–7 天逐系 | S2 筛选节 |
| BSL-2 约束（二级防护/废液消毒/离心杯密封为例） | 方法前置/安全约束（非操作） | — | 方法绑定；不进操作定义 | S2 安全注 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（铺板/加病毒/换液/换药） | 多家族（R1） | shared | S2 全流程 |
| 筛选保持（3–7 天） | 多家族（R3） | `op.incubation`；存活结果不在内 | 对照全死为方法门控 |
| spinfection 离心 | 多家族（R5） | `op.centrifugation` 条件分支复用 | 与 B09 收 cell 同一操作不同绑定 |
| MOI→体积换算 | ToolAction | 非操作 | 滴度/活病毒分数 UNKNOWN（§6 G-2） |
| BSL-2 安全约束 | 方法前置 | 不建模块 | 实验室状态依赖（§6 G-6） |

---

## 4. P-AAV（S3；AAV 转导，筛选另行）

绑定摘要：与 P-LENTI 不合并。转导时 50–70% 汇合（来源绑定例）；病毒按 vg/cell（来源例 10³–10⁵；仅本来源绑定）加入；血清存在与否按血清型/来源绑定（有血清 vs 低血清预饥饿两分支，不合并默认值）。转导后 24–72 h 不换液或半换液（来源绑定），之后正常维持至表达峰（3–7 天例；荧光报告读数归下游）。无抗生素筛选为默认分支；稳定维持另行建系（引用 B09，不在本节展开）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 按 50–70% 汇合铺板 | `op.liquid_transfer` 参数化 + 方法前置 | 悬液 → 已铺板细胞 | 方法绑定 | S3 转导前节 |
| 按 vg/cell 稀释加病毒（血清分支二选一） | `op.liquid_transfer` 参数化 + ToolAction（vg→体积换算） | 病毒储备 → 转导培养 | 方法绑定：vg/cell、血清条件逐血清型 | S3 转导节 |
| 24–72 h 保持（不换液/半换液分支），后正常维持 3–7 天 | `op.incubation` 参数化 + `op.liquid_transfer`（条件换液） | 转导培养 → 待测培养 | 方法绑定；换液分支归方法 | S3 维持节 |
| 表达观察（荧光报告为例） | 方法终点（`FLUO_IMAGE` raw 采集可复用；效率%归方法） | 待测培养 → 表达判定 | 方法绑定；定量归下游 | S3 检测节 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/换液 | 多家族（R1） | shared | S3 全流程 |
| 转导后保持 | 多家族（R3） | `op.incubation`；表达结果不在内 | 3–7 天为方法绑定 |
| vg→体积换算 | ToolAction | 非操作 | 衣壳/基因组滴度口径 UNKNOWN（§6 G-2） |
| 血清条件分支 | 方法约束 | 不建模块 | 血清型依赖（§6 G-3） |

---

## 5. P-CLONE（S4 + S5；编辑/转导后单细胞克隆 + kill curve）

绑定摘要：起点为已筛选存活池（S2 筛选后或 S1 编辑后富集池）。kill curve 先行（S5）：汇合皿按 1:5–1:10 分到系列 puromycin 浓度，孵 3–7 天（每 2–3 天换药），存活判读定筛选浓度；浓度逐系×逐批号，不得复用。单细胞分离沿 S4：胰酶消化 + 吹打/40 µm 筛制单细胞悬液（终点人工判定），计数后有限稀释至 5 cells/mL（100 µL/孔 → 0.5/孔例；Poisson 归 ToolAction），96 孔静置 7–14 天，~7 天首检，全板扫描单集落孔保留、多集落孔弃用，扩增后做敲除验证（测序/Western out-of-scope）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| kill curve：1:5–1:10 分皿进系列浓度；3–7 天每 2–3 天换药；存活判读定浓度 | `op.liquid_transfer` + `op.incubation` 参数化 + ToolAction（曲线拟合/浓度选择） | 汇合培养 → 筛选浓度绑定值 | 方法绑定；浓度逐系×逐批号 | S5 kill curve 节 |
| 制单细胞悬液（胰酶 + 吹打/40 µm 筛；计数） | method-local（解离，审计 `#24`）+ 细胞浓度计数（R6） | 存活池 → 已计数单细胞悬液 | 方法绑定；分散终点人工判定 | S4 悬液节 |
| 有限稀释至 5 cells/mL；100 µL/孔（0.5/孔例） | `op.liquid_transfer` 参数化（连续稀释复合，审计 `#17`）+ ToolAction（稀释/Poisson 换算） | 悬液 → 96 孔单细胞板 | 方法绑定；分布归 ToolAction | S4 铺板节 |
| 静置 7–14 天（~7 天首检）；全板扫描单集落保留 | `op.incubation` 参数化 + 方法门控（单克隆判定） | 单细胞板 → 单克隆候选孔 | 方法绑定：7–14 天；判定人工 | S4 培养/判定节 |
| 扩增后敲除验证（测序/Western/流式） | 方法终点（非操作） | 候选孔 → 克隆系 + 验证判定 | 方法绑定；分析 out-of-scope | S4 验证节 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（分皿/换药/稀释/铺板/扩增） | 多家族（R1） | shared | S4/S5 全流程 |
| 保持（kill curve 3–7 天 / 克隆 7–14 天） | 多家族（R3） | `op.incubation`；存活/生长不在内 | 两段时钟分离 |
| 细胞浓度计数 | 多家族（R6） | shared（计数 only） | 活力/单分散判读归方法 |
| 单克隆/编辑判定 | —（方法门控） | 非操作 | 成像存档/二次确证原文未要求（§6 G-7） |

---

## 6. Reuse Map 汇总（本批）

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 铺板/稀释/合并/加 RNP/加病毒/换液/换药/有限稀释/扩增（S1–S5） | `op.liquid_transfer` | 参数化复用；绑定体积/容器/顺序备注（RNP 两段复合、勿吹散、防交叉） |
| 复合保持（S1 gRNA–Cas9 10–20 min / RNP–脂质体 10–20 min） | `op.incubation` | 参数化复用；两段各自绑定，不合并通用值 |
| 转导/转染后/筛选/克隆生长保持（S1 24–72 h / S2 12–24 h+48 h+3–7 天 / S3 24–72 h+3–7 天 / S4 7–14 天 / S5 3–7 天） | `op.incubation` | 参数化复用；完成=elapsed only；转导/编辑/存活/生长结果一律不在内 |
| spinfection（S2 可选） | `op.centrifugation` | 条件分支复用；RCF/时间逐系 |
| 取样/计数（S1 检测取样 / S4 单细胞悬液计数 / S5 存活计数） | `op.sample_aliquot`（取样）/ 细胞浓度计数（R6） | 复用（计数 only；谱系注记按 R2）；活力/单分散判读归方法 |
| gRNA 退火（S1 95 °C 程序） | 无（method-local 独立候选） | 不晋升（§7） |
| CRISPR vs 慢病毒 vs AAV 递送 | 无（三分立 method-local/模板分支） | 一律不合并；跨分支禁抄值（§7） |
| MOI/vg 换算/kill curve 拟合/Poisson 分布/编辑效率% | ToolAction | 一律非操作 |
| 存活/单集落/敲除判定（S1–S5 全部状态门控） | 无（方法门控，非操作） | 方法-local 或门控，不建操作 |
| 表达/编辑读数（荧光/T7E1/测序/Western/流式） | `FLUO_IMAGE` raw 采集可复用；判定与定量归方法/下游 | 不建"转导/编辑效率"操作（审计 `#12` 同理） |

---

## 7. 跨方法公共操作覆盖矩阵

| 公共操作/候选 | P-CRISPR-RNP (S1) | P-LENTI (S2) | P-AAV (S3) | P-CLONE (S4/S5) | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 铺板/合并/加复合物 | ● 铺板/加病毒/换液/换药 | ● 铺板/加病毒/换液 | ● 分皿/换药/稀释/铺板/扩增 | 最高复用；全参数化 |
| `op.incubation`（复合保持） | ● 两段 10–20 min | — | — | — | S1 独有；与 B09 同语义不同值 |
| `op.incubation`（转导后/筛选/生长保持） | ● 24–72 h | ● 12–24 h + 3–7 天 | ● 24–72 h + 3–7 天 | ● 3–7 天 + 7–14 天 | 完成=elapsed only |
| `op.centrifugation` | — | ○ spinfection 可选 | — | — | 条件分支；不作默认 |
| 细胞浓度计数 | — | — | — | ● 悬液/存活计数 | 计数 only |
| gRNA 退火 method-local | ● 95 °C 程序 | — | — | — | 单家族 |
| CRISPR/慢病毒/AAV 分立 | ● RNP | ● 整合+药筛 | ● 非整合+无筛选默认 | —（起点为存活池） | 三者互不可比：RNP（蛋白–RNA 复合+脂质体）/ 慢病毒（RNA 病毒+polybrene+药筛+BSL-2）/ AAV（DNA 病毒+vg/cell+血清型分支）的作用对象与完成判据不同；MOI≠vg/cell 不可互换 |
| 状态门控（汇合/存活/单集落/敲除） | ● 60–80% + 编辑判定 | ● 30–60% + 对照全死 | ● 50–70% + 表达判定 | ● 单集落 + 验证判定 | 全系人工/方法判定；非操作 |

●=主路径使用；○=条件/替代分支；—=本批未使用。

---

## 8. 缺口清单（不能表达/UNKNOWN；类别按 record-standard §4）

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 脂质体/缓冲液专有配方 | `source_missing` | RNAiMAX/Opti-MEM 同类配方 proprietary；禁把任一递送体系的量外推到另一体系 |
| G-2 | 病毒滴度口径与活病毒分数 | `source_missing` + `method_dependent` | MOI（感染性）vs vg/cell（基因组）口径不同；滴定方法（功能滴度 vs qPCR）归包装 lot；本批无滴定步骤证据，换算输入值视为 lot 绑定 |
| G-3 | polybrene/血清条件分支 | `method_dependent` | S2 polybrene 4–8 µg/mL 仅例；毒性窗逐系预试；S3 血清分支逐血清型；不得默认加入他方法 |
| G-4 | spinfection 适用窗 | `method_dependent` | S2 例 800–1000×g 30–60 min；贴壁脱落/毒性 trade-off 原文未量化；非默认分支 |
| G-5 | 编辑/转导效率判据 | 方法边界 | 最优=高效率+低毒性权衡，原文无阈值；T7E1/测序/荧光定量与设门归下游 out-of-scope；不建效率操作 |
| G-6 | 筛选浓度无通用值 | `method_dependent` | puromycin 工作浓度=kill curve 逐系×逐批号测定；禁携值跨系/跨批使用；BSL-2 约束为实验室状态依赖 |
| G-7 | 单克隆/敲除确证等级 | 方法边界 | S4 以全板扫描单集落为克隆证据；成像存档/二次确证/敲除纯合性声明的证据等级归方法模板 |
| G-8 | 解离终点与损失 | `source_missing` | 吹打/过筛力度与单分散终点为操作者判断；洗涤/消化损失未量化（lossy 变体证据不足，不立项） |
| G-9 | 时间锚点 | 锚点缺口 | RNP `t₀`=复合物加细胞；慢病毒 `t₀`=病毒加入（spinfection 起点另行）；AAV `t₀`=病毒加入；筛选 `t₀`=进药时刻；克隆 `t₀`=单细胞铺板。防漂移约束为方法注记 |
| G-10 | 范围外（误建模块风险） | `not_applicable` | 病毒包装/滴定、gRNA 设计、脱靶/测序分析、流式设门与分选、通用"递送/转导/筛选/挑克隆"模块——本批一律不建模 |
| G-11 | BSL 等级差异 | 方法前置 | 慢病毒 BSL-2 vs RNP/AAV（含血清型）实验室生物安全要求不同；禁把任一分支的安全默认复制到另一分支 |

---

## 9. 晋升审查结论：零新公共模块

四问（独立目的/稳定边界/独立确认/有意义复用）+ 晋升四门（G1 复用广度 ≥2 独立方法、域内成立；G2 语义稳定；G3 抽象收益；G4 分支成本）：

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| RNP/gRNA 复合保持 | ✓✓✓✓ | ✓（S1 两段 + B09 三家族同语义） | ✓ | ✗（`op.incubation` 参数化已覆盖） | ✓ | 不新增，直接复用 `op.incubation` |
| gRNA 退火（95 °C 程序） | ✓✓✓✓ | ✗（仅 CRISPR 一家族） | △（oligo 体系/程序逐设计） | ✗ | ✗ | method-local；不晋升 |
| 病毒转导递送（MOI/polybrene/spinfection vs vg/血清型） | ✓✓✓✓ | ✗（慢病毒与 AAV 各自单家族；两病毒互不复用） | ✗（整合 vs 非整合、筛选 vs 无筛选 effect 不同） | ✗ | ✗ | 三分立 method-local/模板分支；不晋升，不合并 |
| 筛选保持/单克隆分离 | 复合（稀释复合 + 转移 + 保持 + 人工判定） | ✗（单家族；B09 同类不算第二家族独立证据） | — | — | — | Composite + 方法门控；不建原子（审计 `#17` 同理） |
| 转导/编辑效率状态 | ✗（无操作转化意图） | — | — | — | — | 方法-local 或门控，不建操作 |

因此本批全部候选留在 method-local / 既有操作参数化复用 / 方法门控 / ToolAction，不进入公共 Catalog，不写 PortSpec/Transition 全文，不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量：`op.liquid_transfer`（+RNP 复合/病毒加入/有限稀释家族）、`op.incubation`（+两段复合保持/转导后保持/筛选保持/克隆生长绑定）、`op.centrifugation`（+spinfection 条件分支）、细胞浓度计数（+单细胞悬液/存活计数）。

## 10. 核查与状态

- 已验证（间接读大纲，2026-11-05，§1 定位节）：S1 RNP 复合/转染节；S2 转导/polybrene/换液/筛选节；S3 vg 转导/血清分支/维持节；S4 单细胞铺板/判定节；S5 kill curve 节。单人 `agent-verified`（大纲级），仍需第二人逐字段复核全文；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：任何病毒 lot 滴度值、polybrene 毒性窗、spinfection 最优 RCF、puromycin 实测浓度、编辑/转导原始效率数据、gRNA 退火完整程序、测序/Western 原始数据；S1–S5 数值一律视为来源绑定例，不跨系/跨批引用。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器/安全规格分离；样本跃迁有端口（§§2–5 表 1）；时间锚显式（§8 G-9）；工作值标协议归属；plan/actual/raw/derived 分离（本批只有 plan + 方法约束，无 run 值）；无 JSON；UNKNOWN 永不填零。
