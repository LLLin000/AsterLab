# Batch-04 Molecular Basics — DNA Extraction / RNA Extraction-QC / PCR / Agarose Gel Electrophoresis

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`；没有 `ReviewRecord`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 范围: 4 个基础分子湿实验的步骤与原始读数；只记录差异与绑定，不重教通用操作原理。
- 规则: 提取、PCR、扩增后电泳三者互不合并；定量读数（A260/A280、Qubit/Quant-iT）与条带判读/浓度换算一律记为方法模板绑定或 ToolAction/out-of-scope，不建操作；新模块须过四维粒度判定 + 晋升四门（≥2 独立方法家族）否则留 method-local。本文件不提出任何新公共模块。
- 现有仓库记录仅用于 Reuse Map，不作步骤/数值证据；每条数值证据只引本文件 §1 的原始 protocol/厂商手册。

## 1. 来源索引

| # | Protocol | 权威类型 | 版本/定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | QIAGEN DNeasy Blood & Tissue Handbook, Protocol: Purification of Total DNA from Animal Tissues (Spin-Column) | 厂商手册 | 07/2006 版 PDF pp.1–3，步骤 1–8 + Things to do before starting | 直接读：步骤 1–8 全文 + 注意（ATL/AL 结晶、56 °C 预热、RNase A 可选、乙醇干燥关键） |
| S2 | Thermo Fisher TRIzol Reagent User Guide, Pub. MAN0001271 Rev. D (2025-03-31) | 厂商手册 | pp.1–3：Lyse samples and separate phases 步骤 1–7；Precipitate/Wash/Solubilize RNA；Determine RNA yield；Table 4 典型产量 | 直接读：裂解/分相/沉淀/洗涤/溶解/定量节全文；蛋白/透析节仅范围浏览不作证据 |
| S3 | NEB Taq PCR Kit Instruction Manual E5000S v4.0_1/20, Protocol for a Routine PCR Reaction + PCR Optimization | 厂商手册 | pp.2–3：50 µL 反应表 + Cycling 表 + Optimization §1–11；Troubleshooting 仅参照 | 直接读：反应配制表、循环条件表、优化 §1–11 全文 |
| P4 | Faircloth-lab protocols, Gel Visualization (Carl Oliveros, CC-BY), gel-visualization.rst | 大学 lab protocol | Steps 1–11：1.25% agarose / TBE / GelRed / 110–120 V ~1 h / 成像 | 直接读：步骤 1–11 全文；适用范围仅限“DNA 提取物/PCR 产物/文库的 QA 可视化”，不冒充普适标准方法 |
| — | CSH Protocols prot100404（Agarose Gel Electrophoresis）/ prot095117（Basic PCR）摘要；Morimoto lab GitHub 组织页；Rice Bennett Lab Wiki | 参照/排除项，非证据 | — | 未用作证据：CSH 全文 403 未读到正文，仅检索摘要；Morimoto 组织页无 DNA/PCR/电泳 protocol；Rice Wiki 未直接读。凡本文件数值均不引自这些页 |

既有复用对照（仅身份，不复述契约，不作证据）：
R1 `op.liquid_transfer`；R2 取样/分样 `op.sample_aliquot`（谱系注记，conditional）；R3 `op.incubation`（elapsed-only）；R4 `op.centrifugation`；R5 `ABS_READ`（仅生化比色原始 OD，本批核酸定量不复用它）；R6 `FLUO_IMAGE`（仅原始图像采集）；R7 细胞浓度计数（本批未用，仅对照）；R8 连续稀释 = Composite；R9 固定/染色/通用定量不建模块（freeze §3 + audit §4.3/§27）。

---

## 2. P-DNA（S1；动物组织 DNeasy 柱式）

绑定摘要：≤25 mg 组织（脾 ≤10 mg；鼠尾 0.4–0.6 cm）+ 180 µL Buffer ATL；+ 20 µL proteinase K，56 °C 消化至完全（组织常 1–3 h，鼠尾 6–8 h，可过夜；可选 RNase A 4 µL 100 mg/mL RT 2 min 去 RNA）；涡旋 15 s 后 + 200 µL Buffer AL 混匀 + 200 µL 乙醇（96–100%）混匀成均一液；全部转入 DNeasy Mini 柱，≥6000×g（8000 rpm）1 min，弃流穿；换管 + 500 µL AW1，≥6000×g 1 min，弃流穿；换管 + 500 µL AW2，20,000×g（14,000 rpm）3 min 干燥膜（残留乙醇干扰下游；勿让柱接触流穿，否则再空甩 1 min）；换干净 1.5/2 mL 管，200 µL AE 直接加膜上，RT 1 min，≥6000×g 1 min 洗脱（100 µL 浓度高但总量低；推荐重复洗脱一次合并或分管；1.5 mL 管单次勿超 200 µL）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 剪碎 ≤25 mg 组织入 1.5 mL 管 + 180 µL ATL（冻存须回温；脾 ≤10 mg） | `method_local`（组织取样/分装前处理）+ `op.liquid_transfer` 参数化（加 ATL） | 组织块 → 含 ATL 组织管 | 方法绑定：25/10 mg 上限、180 µL ATL；研磨/匀浆为可选分支 | S1 Things to do + 步骤 1 |
| + 20 µL proteinase K，涡旋，56 °C 至完全裂解（1–3 h / 鼠尾 6–8 h / 可过夜）；可选 RNase A 4 µL RT 2 min | `op.liquid_transfer`（加酶）+ `op.incubation` 参数化（56 °C 保持）+ `method_local`（蛋白酶裂解消化，终点人工判断） | 含 ATL 组织管 → 裂解液（可粘稠、不可胶冻） | 方法绑定：20 µL、56 °C、时长窗；胶冻分支见 Troubleshooting（本批不展开） | S1 步骤 2 |
| 涡旋 15 s；+ 200 µL AL 混匀 + 200 µL 乙醇混匀成均一液（可预混；白色沉淀不干扰） | `op.liquid_transfer` 参数化 + `method_local`（变性/结合条件化，AL+乙醇体系） | 裂解液 → 结合相溶液 | 方法绑定：200+200 µL；均一性为完成检查 | S1 步骤 3 |
| 全部转柱，≥6000×g 1 min，弃流穿+收集管 | `op.liquid_transfer`（转柱）+ `op.centrifugation` 参数化 + `method_local`（硅胶膜结合-弃流穿，effect 与细胞洗涤不兼容） | 结合相溶液 → 已结合柱 + 废流穿 | 方法绑定：≥6000×g 1 min；结合 effect 留方法内 | S1 步骤 4 |
| 换管 + 500 µL AW1，≥6000×g 1 min，弃流穿 | 同上（加液转移 + 离心 + 方法内柱洗分支 1） | 已结合柱 → AW1 已洗柱 | 方法绑定：500 µL AW1、≥6000×g 1 min | S1 步骤 5 |
| 换管 + 500 µL AW2，20,000×g 3 min 干燥膜；勿接触流穿 | 同上（方法内柱洗分支 2 + 干燥确认）；离心复用 R4 | AW1 已洗柱 → 干燥柱 | 方法绑定：500 µL AW2、20,000×g 3 min；干燥为阻断性完成检查 | S1 步骤 6 |
| 换干净管，200 µL AE 直加膜上，RT 1 min，≥6000×g 1 min 洗脱；推荐重复一次 | `op.liquid_transfer` + `op.centrifugation` 参数化 + `method_local`（洗脱回收，浓度/总量权衡） | 干燥柱 → 洗脱液（含 DNA）+ 废柱 | 方法绑定：200 µL（100 µL 变体）、RT 1 min、≥6000×g 1 min；重复洗脱分支 | S1 步骤 7–8 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（加 ATL/酶/AL/乙醇/转柱/加洗液/加 AE） | 多家族（R1 已有） | shared（`op.liquid_transfer` 参数化） | S1 步骤 1–8 全部体积绑定；本批仅新增分子柱式家族证据 |
| 离心（结合/洗涤/干燥/洗脱） | 多家族（R4 已有） | shared（`op.centrifugation` 参数化） | S1 ≥6000×g / 20,000×g 绑定；转子/RCF 换算缺口见 §7 |
| 孵育/保持（56 °C 裂解、RT 1 min 洗脱） | 多家族（R3 已有） | shared（elapsed/条件 only；裂解完成不断言） | S1 步骤 2/7；完全裂解靠人工判断 |
| 硅胶膜结合-洗涤-干燥-洗脱整体 | 仅核酸柱提家族 | method-local（不建模块） | G1 不过（本批仅 1 分子家族）；G2 不过（固相保留-DNA effect 与细胞洗涤不兼容，不并入 `op.washing`） |
| 组织剪碎/匀浆、蛋白酶消化终点 | 仅本方法 | method-local / Custom Step | 终点人工判断；研磨粒度 UNKNOWN |

---

## 3. P-RNA（S2；TRIzol 抽提 + QC）

绑定摘要：1 mL TRIzol / 50–100 mg 组织 / 单层 1×10⁵–1×10⁷（3.5 cm 皿）/ 悬浮 5–10×10⁶（动物/植物/酵母）或 1×10⁷（细菌）；样本体积 ≤ 裂解体积 25%；高脂可选 12,000×g 4–10 °C 5 min 去脂；5 min 室温解离；+ 0.2 mL 氯仿 / 1 mL TRIzol，剧烈摇匀，2–3 min；12,000×g 4 °C 15 min 分相；45° 吸水相（禁带中间层/有机相）；小样本（<10⁶ 细胞 / <10 mg）可选 5–10 µg RNase-free glycogen；+ 0.5 mL 异丙醇 / 1 mL TRIzol，室温 10 min；12,000×g 4 °C 10 min 白色胶状沉淀；1 mL 75% 乙醇 / 1 mL TRIzol 重悬，涡旋，7500×g 4 °C 5 min；弃上清，晾干 5–10 min（禁真空离心干燥、禁过干；过干 A230/280 <1.6 警告）；20–50 µL RNase-free 水 / 0.1 mM EDTA / 0.5% SDS 吹打溶解（酶促下游禁 SDS），55–60 °C 10–15 min；–70 °C 存或下游；75% 乙醇中 –20 °C ≥1 年 / 4 °C ≥1 周。QC：A260 定量（A260 × 稀释 × 40 = µg/mL，NanoDrop 免稀释），A260/280 ~2 为纯；或 Qubit/Quant-iT 荧光定量（只测完整 RNA）。Table 4 例：10⁶ 上皮 8–15 µg、成纤维 5–7 µg、肝 6–10 µg/mg。本批有机相 DNA/蛋白路径不记录。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 按样加 TRIzol 匀浆裂解（组织/单层去培养基直接加/悬浮离心去上清后加；禁预洗；高 RNase 用冷 TRIzol；RNase-free 耗材+手套） | `op.liquid_transfer` 参数化 + `method_local`（酚-异硫氰酸胍裂解匀浆） | 组织/细胞 → 裂解匀浆 | 方法绑定：1 mL 配比三档 + 25% 体积上限；停止点 4 °C 过夜 / –20 °C 1 年 | S2 Lyse 步骤 1 + Procedural guidelines |
| 高脂可选 12,000×g 5 min 取上清；室温 5 min 解离核蛋白 | `op.centrifugation` + `op.incubation` 参数化 | 裂解匀浆 → 已解离匀浆 | 方法绑定：12,000×g 4–10 °C 5 min；5 min | S2 步骤 2–3 |
| + 0.2 mL 氯仿 / 1 mL TRIzol，盖紧剧烈摇匀，2–3 min；12,000×g 4 °C 15 min 分相 | `op.liquid_transfer` + `op.centrifugation` 参数化 + `method_local`（氯仿分相，effect 方法特定） | 已解离匀浆 → 三相（水相 RNA / 中间层 / 有机相 DNA-蛋白） | 方法绑定：0.2 mL 配比、2–3 min、12,000×g 4 °C 15 min | S2 步骤 4–6 |
| 45° 吸水相入新管（禁带中间层/有机相；有机相 4 °C 过夜存） | `op.sample_aliquot` 参数化（父保留 + 子样本；谱系注记） | 三相管 → 水相子管 + 母管（中间层/有机相存） | 方法绑定：45° 手法；污染即失败模式 | S2 步骤 7 |
| 可选 glycogen 5–10 µg（小样本）；+ 0.5 mL 异丙醇 / 1 mL TRIzol，室温 10 min；12,000×g 4 °C 10 min | `op.liquid_transfer` + `op.incubation` + `op.centrifugation` 参数化 + `method_local`（异丙醇沉淀，effect 方法特定） | 水相子管 → 白色胶状 RNA 沉淀 + 废上清 | 方法绑定：5–10 µg、0.5 mL 配比、10 min、12,000×g 10 min | S2 Precipitate 步骤 1–4 |
| 1 mL 75% 乙醇重悬，涡旋，7500×g 4 °C 5 min；弃上清；晾干 5–10 min（禁真空离心、禁过干） | `op.liquid_transfer` + `op.centrifugation` 参数化 + `method_local`（乙醇洗-晾干，干燥度人工判断） | RNA 沉淀 → 已洗沉淀 | 方法绑定：1 mL 配比、7500×g 5 min、5–10 min；过干为失败模式 | S2 Wash 步骤 1–4 |
| 20–50 µL RNase-free 水/EDTA/SDS 吹打溶解；55–60 °C 10–15 min；–70 °C 存 | `op.liquid_transfer` + `op.incubation` 参数化 | 已洗沉淀 → 可用 RNA 溶液 | 方法绑定：20–50 µL、55–60 °C 10–15 min；SDS 禁用分支（酶促下游） | S2 Solubilize 步骤 1–3 |
| A260/A280 定量（稀释测或 NanoDrop）；荧光试剂盒定量；Table 4 产量参照 | 方法模板内读数绑定 + ToolAction（浓度换算） | RNA 溶液 → 原始吸光/荧光值 → 派生浓度 | 方法绑定：×40 系数、~2 纯度判据、Table 4 产量 | S2 Determine yield + Table 4 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（加 TRIzol/氯仿/异丙醇/乙醇/溶解液） | 多家族 | shared（`op.liquid_transfer` 参数化） | S2 全流程体积配比绑定 |
| 离心（去脂/分相/沉淀/洗涤） | 多家族 | shared（`op.centrifugation` 参数化） | S2 12,000×g / 7500×g / 温度绑定 |
| 孵育/保持（5 min 解离、2–3 min、10 min 沉淀、55–60 °C 溶解） | 多家族 | shared（elapsed only） | S2 四段保持分别记时；解离/沉淀完成不断言为生物学完成 |
| 水相 harvest | 细胞上清、LDH harvest、微生物取样 + 本批水相 | shared conditional（`op.sample_aliquot` + 谱系注记） | S2 步骤 7；禁带层为完成检查；守恒仍 UNKNOWN |
| 酚-氯仿分相 / 异丙醇沉淀 / 乙醇洗-晾干 | 仅 TRIzol 家族 | method-local（不建模块） | G1 不过；G2 不过（沉淀-固相 effect 与保样洗涤不兼容） |
| 核酸定量（A260/A280、荧光） | —（读数绑定） | 不建通用模块（freeze §3 + audit #27） | blank/单位/完整性判据方法依赖；换算为 ToolAction |

---

## 4. P-PCR（S3；NEB Taq 常规反应）

绑定摘要：50 µL 体系：5 µL 10× Standard Taq Buffer（→1×：10 mM Tris-HCl pH 8.3、50 mM KCl、1.5 mM MgCl₂）+ 1 µL 10 mM dNTP（→200 µM each）+ 上/下游引物各 1 µL 10 µM 储备（→0.2 µM each，范围 0.05–1.0，常用 0.1–0.5）+ 模板（质粒/病毒 1 pg–1 ng，基因组 1 ng–1 µg）+ 0.25 µL Taq（1.25 U/50 µL，范围 0.25–2.5 U；难吸取可用 Diluent F 稀释）+ 水至 50 µL；冰上配制、酶最后加、预热 95 °C 即转入；无热盖加矿物油；多反应做 master mix。循环：95 °C 30 s ×1；95 °C 15–30 s / 退火 45–68 °C 15–60 s / 68 °C 1 min-per-kb（<1 kb 取 45–60 s）×30；68 °C 5 min ×1；4 °C hold。优化分支：GC/难模板初始变性 2–4 min、菌落 PCR 5 min；Mg²⁺ 1.5–2.0 起 0.5–1.0 步进；引物 20–30 nt、GC 40–60%、Tm >45 °C 且对内 ≤5 °C 差、退火取低 Tm–5 °C 常用 50–60 °C；dNTP >500 µM 禁用。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 50 µL 配制：buffer/dNTP/引物/模板/酶/水（master mix，多反应分装） | `op.liquid_transfer` 参数化（多次加样 = 重复转移；连续稀释不适用） | 储备液 → 已配制反应管 | 方法绑定：5/1/1/1 µL、0.25 µL 酶、模板两档量、终浓度全表 | S3 Routine 步骤 1 + Optimization §1–5/§8 |
| 冰上配制、酶最后加、轻混离心；无热盖加矿物油；预热 95 °C 即转入 | 父操作备注/前置条件（防非特异、防蒸发），不是新操作 | 已配制反应管 → 待循环反应管 | 方法绑定；Hot Start 聚合酶为例外分支 | S3 步骤 2 + Optimization §6–7 |
| 热循环：初始变性 + 30×三步 + 终延伸 + hold（含 GC/菌落变体） | `method_template` 内程序化保持序列（不拆为多次 `op.incubation`；循环语义不同） | 待循环管 → 扩增产物管 | 方法绑定：95/45–68/68 °C、15 s–5 min、×30；退火/延伸按引物/长度定 | S3 Cycling 表 + Optimization §9–11 |
| 阳性/阴性对照、污染控制（正压/滤芯头、master mix 减误差） | 方法对照矩阵（非操作） | — | 方法绑定 | S3 Optimization §1/§7 |
| 无产物/非特异 troubleshooting（加循环、调 Mg/退火、换引物、纯化模板） | 方法分支/人工判断（非操作） | — | 方法绑定 | S3 Troubleshooting 表 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（反应配制/master mix 分装） | 多家族 | shared（`op.liquid_transfer` 参数化） | S3 反应表；本批新增 PCR 配制家族证据 |
| 热循环三步/终延伸/hold | 仅 PCR 家族 | 方法模板内程序（不复用 `op.incubation`） | 语义不同：程序化变温循环 ≠ 等温保持；合并将丢失退火/延伸区分，G2 不过 |
| 引物/Tm/退火选择、Mg/dNTP 优化 | —（设计/计算） | 方法绑定 + ToolAction（Tm Calculator） | S3 §2–4/§10；不建操作 |
| 扩增成败判读/ troubleshooting | —（判断） | 非操作（人工判断 + Custom Step 出口） | S3 Troubleshooting 表；循环数/特异性权衡 |

---

## 5. P-GEL（P4；Faircloth 凝胶可视化 QA）

绑定摘要：小胶 0.5 g / 40 mL 1× TBE（50 mL 瓶），大胶 2.0 g / 160 mL（250 mL 瓶）→ 1.25% agarose；+ GelRed 1 µL（小）/ 2 µL（大），旋匀；微波至均一（小 ~1 min / 大 ~2.5 min，需 2–3 轮微波-旋匀）；冷至 ~60 °C（水浴/冷水/静置至可触）；摆平胶床、放好梳子；温胶倒入，凝固 15–20 min；每样 + 1 µL loading dye；拔梳，胶连床入槽，DNA 由负（黑）向正（红），必要时补 1× TBE 浸没；上样 + ladder；盖盖接电，110–120 V ~1 h；三楼公用 imager 拍照。目的仅限 QA 可视化（提取物/扩增子/文库）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 称量配 1.25% agarose / TBE（0.5 g/40 mL；2.0 g/160 mL）+ GelRed（1/2 µL）旋匀 | `op.liquid_transfer` 参数化（量取缓冲）+ `method_local`（称量-溶胶配制） | 粉/缓冲/染料 → 待加热溶胶 | 方法绑定：1.25%、两档体积、GelRed 两档量 | P4 步骤 1–2 |
| 微波至均一（2–3 轮微波-旋匀；小 ~1 min / 大 ~2.5 min） | `method_local`（加热溶解；设备条件化） | 待加热溶胶 → 均一热胶 | 方法绑定；微波功率 UNKNOWN（`configuration_dependent`） | P4 步骤 3 |
| 冷至 ~60 °C；摆平胶床放好梳子；温胶倒入，凝固 15–20 min | `method_local`（铸胶-凝固；温度/水平人工判断）+ `op.incubation` 不适用（凝固 ≠ 培养保持，不复用） | 热胶 → 已凝固胶床 | 方法绑定：~60 °C、15–20 min | P4 步骤 4–6 |
| 每样 + 1 µL loading dye；拔梳入槽（黑→红方向），补 TBE 浸没 | `op.liquid_transfer` 参数化（加染料/补缓冲） | DNA 样 + 胶床 → 待跑胶 | 方法绑定：1 µL/样；方向为阻断性前置 | P4 步骤 7–8 |
| 上样 + ladder；110–120 V ~1 h | `op.liquid_transfer`（上样）+ `method_local`（电泳分离-迁移，effect 方法特定） | 待跑胶 → 已分离胶 | 方法绑定：110–120 V、~1 h；浓度/电压/时间互依 | P4 步骤 9–10 |
| 公用 imager 拍照存档 | `FLUO_IMAGE` 参数化（仅原始图像采集；模态/通道绑定） | 已分离胶 → 原始胶图 + 已成像胶（光漂白/染料安全警告） | 方法绑定：imager 型号/曝光 UNKNOWN | P4 步骤 11 |
| 条带判读（有无/大小/质量） | （分析） | 胶图 → 派生判定 | out-of-scope；非操作 | P4 Purpose（QA）隐含；本批不定义算法 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（量缓冲/加染料/上样/补缓冲） | 多家族 | shared（`op.liquid_transfer` 参数化） | P4 步骤 1/7–10 |
| 原始图像采集（胶图拍照） | 成像多家族（R6 已有） | shared（`FLUO_IMAGE` 参数化，模态绑定） | P4 步骤 11；只到原始图，判读另立 |
| 溶胶-铸胶-电泳分离整体 | 仅本 QA 电泳家族 | method-local（不建模块） | G1 不过（本批仅 1 电泳家族）；G2 不过（基质固化 + 电场迁移 effect 与孵育/转移不兼容） |
| 条带判读/定量 | —（分析） | 非操作（out-of-scope） | 与细胞活力 call 同理（audit #12/#34） |

---

## 6. Reuse Map 汇总（本批）

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 加 ATL/酶/AL/乙醇/转柱/加洗液/加 AE（DNA）；加 TRIzol/氯仿/异丙醇/乙醇/溶解液（RNA）；反应配制/master mix（PCR）；量缓冲/加染料/上样/补缓冲（GEL） | `op.liquid_transfer` | 参数化复用；绑定体积/配比/孔板-管（§§2–5） |
| 56 °C 裂解、RT 洗脱（DNA）；解离/分相等待/沉淀/55–60 °C 溶解（RNA） | `op.incubation` | 参数化复用；只断言 elapsed/条件；裂解/沉淀完成不断言 |
| 结合/洗涤/干燥/洗脱离心（DNA）；去脂/分相/沉淀/洗涤离心（RNA） | `op.centrifugation` | 参数化复用；绑定 RCF/时间/温度；转子/RCF 换算缺口 |
| 水相 harvest（RNA） | `op.sample_aliquot` | 参数化复用（conditional + 谱系注记）；禁带层为完成检查 |
| 胶图拍照（GEL） | `FLUO_IMAGE` | 参数化复用（模态绑定）；只到原始图 |
| 硅胶膜结合-洗-干-洗脱；酚-氯仿分相/醇沉/乙醇洗-晾干；PCR 热循环程序；溶胶-铸胶-电泳分离 | — | 全部 method-local；不晋升（§7 G-3/G-4） |
| A260/A280、Qubit/Quant-iT 换算、Table 4 产量参照；条带判读；PCR troubleshooting/对照 | — | 方法模板绑定 + ToolAction / out-of-scope；一律非操作 |

---

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | P-DNA 柱提 | P-RNA TRIzol-QC | P-PCR Taq | P-GEL QA 电泳 | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 全流程体积 | ● 全流程配比 | ● 反应配制 | ● 缓冲/染料/上样 | 最高复用；全参数化 |
| `op.centrifugation` | ● 结合/洗/干/洗脱 | ● 去脂/分相/沉淀/洗 | — | — | RCF/温度方法绑定 |
| `op.incubation` | ● 56 °C / RT 1 min | ● 解离/等待/溶解 | —（循环不复用） | —（凝固不复用） | 只断言 elapsed/偏差 |
| `op.sample_aliquot` | — | ● 水相 harvest | ○ 模板分装语境 | ○ 样品取样语境 | 谱系关键步；守恒 UNKNOWN |
| `FLUO_IMAGE`（原始） | — | — | — | ● 胶图拍照 | 判读另立 |
| `op.washing` | —（柱洗不并入） | —（乙醇洗不并入） | — | — | 本批无保样细胞洗；合并即 G2 失败 |
| `ABS_READ` | — | —（核酸定量不并入） | — | — | 比色 OD ≠ A260/荧光；另立 |
| method-local 分离/扩增 | ● 硅胶结合-洗脱 | ● 分相-沉淀 | ● 热循环程序 | ● 电泳迁移 | 四者互不合并 |
| ToolAction/分析 | ○ 浓度/总量权衡 | ● A260/荧光换算 | ● Tm/对照/排错 | ● 条带判读 | 全部非操作 |

●=主路径使用；○=语境提及/变体；—=本批未使用。

三者互不可比（§8 G-6）：柱提（固相保留-洗脱）/ TRIzol（液-液分相 + 沉淀）/ PCR（指数扩增）/ 电泳（基质电迁移）的作用对象与完成判据完全不同；定量三通道（A260 吸光 / 荧光 / 胶带亮度）亦不可互换。

---

## 8. 缺口清单（不能表达/UNKNOWN；分类按 record-standard §4）

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | DNA：起始量上限 + RNase 可选 + 干燥确认 + 洗脱浓度/总量权衡；RNA：RNase-free 前置 + 禁带层 + 禁过干/禁真空干燥 + SDS 禁用分支 + A260/280 纯度判据；PCR：阳/阴对照 + 污染控制 + 模板质量门控；GEL：ladder + 方向 + 浸没 + 浓度-电压-时间互依。任一缺失即下游不可信 |
| G-2 | 派生归属 | ToolAction，非操作 | A260×稀释×40、A260/280 ~2、Qubit/Quant-iT 换算、Table 4 产量参照；PCR Tm/退火选择、Mg/dNTP 优化；胶带判读。全部下游，不建模块 |
| G-3 | 力学/手法量 | `source_missing` | 组织剪碎粒度、涡旋/摇匀强度、45° 吸液手法、梳子拔出力、胶床水平度；S1–P4 均无量化，归 SOP 操作者判断 |
| G-4 | 样本/体系依赖值 | `method_dependent` | 组织类型-时长（1–3 h / 6–8 h / 过夜）、脾 10 mg 上限、TRIzol 三档配比、PCR 模板两档量 × 退火 45–68 °C × 延伸 1 min/kb、凝胶 1.25%/110–120 V/~1 h（P4 体系值，不通用） |
| G-5 | 设备/试剂态 | `configuration_dependent` / `device_state` | 微波功率、离心机转子/RCF 换算、热循环仪块均匀性、imager 曝光/分辨、per-lot 缓冲/染料/酶活、移液校准、56 °C/55–60 °C 箱均匀性 |
| G-6 | 方法边界 | 模型边界 | 提取≠扩增≠分离≠定量：柱洗≠细胞洗、醇沉≠保样洗、热循环≠孵育、凝固≠孵育、A260/荧光≠`ABS_READ`、胶图判读≠计数。合并即 G2 失败 |
| G-7 | 不可逆跃迁 | 跃迁约束 | 弃流穿/弃上清/弃有机相一律显式 loss；过干 RNA 难溶；SDS 溶解后禁酶促；PCR 扩增后产物污染风险（前置对照）；GelRed/EB 安全处理（本批不展开 S1/S2 安全页外的值） |
| G-8 | 时间锚点 | 锚点缺口 | DNA `t₀`=加 ATL/酶；RNA `t₀`=加 TRIzol（停止点 4 °C 过夜/–20 °C 1 年）；PCR `t₀`=循环启动（hold 4 °C 为终结保持）；GEL `t₀`=上电（15–20 min 凝固为前置）。孵育不断言裂解/沉淀/扩增/分离完成 |
| G-9 | 范围外（误建模块风险） | `not_applicable` | 临床/язы动物在体、DNA 蛋白 co-prep 展开、有机相 DNA/蛋白回收、RT/qPCR、测序/文库构建、图像分割/定量、统计/生物信息——本批一律不记录、不建模 |

---

## 9. 晋升审查结论：零新模块

- 四问（独立目的/稳定边界/独立确认/有意义复用）下，本批真正的“新语义”只有四个：硅胶膜结合-洗脱、酚-氯仿分相-醇沉、PCR 热循环程序、电泳溶胶-铸胶-迁移。它们各自只出现在 1 个独立方法家族（分子柱提 / TRIzol / PCR / QA 电泳），G1 不过；且前两者与 `op.washing`、热循环与 `op.incubation`、核酸定量与 `ABS_READ` 的 effect/端口/完成检查均不兼容，G2 不过。
- 因此全部留在 method-local（方法模板内步骤 / Custom Step 出口），不进入公共 Catalog，不建 PortSpec/Transition 全文，不做代码/UI/DB 变更。
- 本批对公共 Catalog 的唯一贡献是复用证据增量：`op.liquid_transfer`（+PCR 配制/电泳上样家族）、`op.centrifugation`（+柱提/TRIzol 家族）、`op.sample_aliquot`（+水相 harvest）、`op.incubation`（+裂解/溶解保持）、`FLUO_IMAGE`（+胶图原始采集）。

## 10. 核查与状态

- 已验证（直接读，2026-11-05，§1 定位节）：S1 步骤 1–8；S2 裂解/分相/沉淀/洗涤/溶解/定量节 + Table 4；S3 反应表 + 循环表 + 优化 §1–11；P4 步骤 1–11 全文。单人 `agent-verified`，仍需第二人逐字段复核；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：S1 Troubleshooting/固定组织前处理、S2 蛋白/透析路径与 FAQ 库、S3 引物设计实例与 per-lot 酶活、P4 外的凝胶浓度-电压体系、任何仪器操作手册/单机构建表、per-lot 片、实验室 SOP 改编、系别/种属优化数、任何已跑数据的派生应用。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器规格分离（§§2–5 仅能力 + 用过型号作例）；样本跃迁有端口（§§2–5 表 1）；时间锚 `t₀` 显式；工作值标协议归属；部分列表标部分；plan/actual/raw/derived 分离（原始吸光/荧光/胶图保留，派生下游）；无 JSON；UNKNOWN 永不填零。
