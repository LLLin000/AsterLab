# Batch 18 — Histology & IHC: Fixation / Embedding / Sectioning / H&E / IHC Staining / Slide Imaging

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 组织/类器官切片湿实验 only:固定、包埋 (石蜡 + HistoGel/明胶-蔗糖冷冻)、切片/贴片、脱蜡复水、H&E 染色、IHC (抗原修复/封闭/一抗二抗/显色或荧光/复染/封片)、玻片原始成像。临床诊断判读、病理分级、图像分割/定量、统计一律 out-of-scope,不建操作。B06 已覆盖培养细胞爬片 IF,本批只做切片 (tissue/organoid section) 链,不复拆 B06 步骤。
- 规则: 现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;每条数值/行为证据只引本文件 §1 的直接读来源;检索摘要一律 `search-summary-only` 不作证据;固定/通透/封闭/染色通用模块禁建 (审计 `#23`/`#26`,冻结 §3);不新增公共模块除非四问 + 晋升四门全过;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用固定时间、通用切片厚度、通用抗体稀释度、通用 DAB 显色时间;所有温度/时间/体积/浓度均为所引协议的绑定值,不得跨样本/抗体复制。DAB 为疑似致癌物,操作按厂商 SDS,不在本文件立安全模块。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Thermo Fisher BestProtocols, Immunohistochemical Staining of FFPE Tissues (steps 1–23 + Materials) | 厂商手册 (当前页) | 切片贴片/烘片/脱蜡复水/HIER 或酶修复/洗/内源封闭/血清封闭/一抗 2 h/二抗 1 h/avidin-HRP 30 min/DAB/苏木素复染/脱水透明/Permount 封片 | 直接读:全文转写。Histoclear II 3×5 min;乙醇 100% 2×5 + 90%/70% 各 5 min + 水 5 min;烘片 50–60 °C 20 min (禁超 60 °C);HIER 微波煮沸后维持 15 min + 柠檬酸盐冷却 ~20 min;或 Proteinase K/Trypsin 10–20 min 37 °C;PBS 5 min×3;0.3% H2O2 15–40 min RT;10% 血清 1 h RT + Parafilm/湿盒;一抗 2 h RT (低丰度至 12 h 4 °C);DAB 500 µL stock + 2.5 µL 30% H2O2/50 mL PBS,镜下监控;脱水反向 70→100% + Histoclear,3 滴 Permount,干 1–2 h |
| S2a | Abcam H&E Stain Kit ab245880 protocol book v3b (2026-04-21) | 厂商试剂盒手册 (PDF 全文) | 石蜡或冰冻切片/细胞涂片;Mayer's (Lillie's 改良) 苏木素 + Bluing + 改良醇溶 Eosin Y | 直接读:全文转写。苏木素盖片 5 min → 两换水洗 → Bluing 10–15 s → 两换水洗 → 无水乙醇浸蘸吸干 → Eosin Y 2–3 min → 无水乙醇漂洗 → 三换无水乙醇脱水 → 透明 + 合成树脂封片。脱蜡/复水步写 "deparaffinize if necessary, hydrate in distilled water",exact 级数未给 |
| S2b | UC San Diego CPC Histopathology, H&E Stain (paraffin + frozen, Simone LCM 变体) | 机构核心 protocol (PDF 全文) | 石蜡切片 10 步;冰冻切片 8 步;LCM 变体 7 步 | 直接读:全文转写。石蜡:2× 二甲苯各 10 min;100% 乙醇 5 min×2 + 95% 5 min×2;水 5 min;Gill II 苏木素 3 min;蓝化 30 s;95% 乙醇过一下;Eosin 3 min + 水快蘸;95% 乙醇 1 min + 100% 乙醇 10 dips×3;Citrosol 2 min×3;Cytoseal 60 + 玻璃盖片 |
| S3 | Kollmann et al. 2023, Front Cell Dev Biol 11:1223032, §2.3.3 immunostaining (human intestinal organoid, paraffin + 2D monolayer) | 同行评议论文方法节 | organoid 收获→3.5% PFA 10 min RT→HistoGel 包埋→石蜡;切片二甲苯脱蜡→降序乙醇→10 mM 柠檬酸钠煮沸 10 min 修复;0.1% Triton 10 min;3% BSA + 1% NGS 1 h RT;一抗 4 °C overnight;Alexa488/Cy3 二抗;DAPI;升序乙醇 + 二甲苯;Entellan 封片 | 直接读:方法节全文转写 (数值见表)。WB/TER/活力/统计为下游/分析,不拆 |
| S4 | Lancaster & Knoblich, Nat Protoc 2014 (PMC4160653), Box 1 cryosection prep + Table 1 antibody panel | 同行评议 protocol (PMC 全文) | organoid 4% PFA 15 min 4 °C→PBS 洗×3→30% 蔗糖 4 °C overnight→7.5% 明胶/10% 蔗糖包埋→异戊烷/干冰 −50~−30 °C 速冻 1–2 min→−80 °C 存→cryostat 20 µm 切片 | 直接读:Box 1 步骤 1–14 + Table 1 (15 个抗体/稀释度,仅记存在)。2D/3D 神经分化主体归 B19,本批只用 Box 1 切片链 |
| R0 | B06 (流式/IF)、B11 (3D 换液 lossy)、审计 `#23/#26`、冻结表 `#4/#5/#9` | 既有仓库记录 | 只记身份,不复述契约,不作证据 | R1 `op.liquid_transfer`;R2 `op.sample_aliquot`;R3 `op.incubation` (elapsed only);R4 `op.washing` 保留 + lossy 变体;R5 `op.centrifugation`;R6 `FLUO_IMAGE` (raw;明场为 modality 绑定,审计 §6.2) |

检索摘要 (`search-summary-only`,非证据):Novus IHC paraffin 页、protocols.io IHC paraffin v3、Thermo 高/低 pH IHC 变体页、STAR organoid 分化/CRISPR 页 (403 未直读) 仅用于立项定位;数值以 §1 直接读为准。

---

## 2. P-PREP — 固定/包埋/切片/贴片 (S1 steps 1–2 + S3 收获包埋 + S4 Box 1)

绑定摘要:石蜡链 — 切片贴 Superfrost Plus (S1 step 1),干 oven 50–60 °C 20 min 助贴 + 软化石蜡 (禁超 60 °C 保抗原, S1 step 2);S3 organoid 链:收获洗后 3.5% PFA 10 min RT → 300×g 3 min → PBS 洗 → 37 °C 预热 HistoGel 包被 4 °C 玻片,1 min 固化后转包埋盒 → 再固定 → 石蜡包埋。冷冻链 (S4 Box 1):24 孔板内 organoid → 1 mL PBS 洗 → 1 mL 4% PFA 4 °C 15 min → PBS 10 min×3 → 30% 蔗糖 4 °C overnight (下沉) → 预聚明胶/蔗糖底层 (4 °C) → 37 °C 平衡 15 min → 称量皿内定位排列 (≤6 个/块,早期可多) → 室温 2–3 min 初固 → 覆盖 → 4 °C 15–20 min → 切块 → 异戊烷/干冰 −50~−30 °C 1–2 min 速冻 → −80 °C 存 (可数月) → cryostat 20 µm 切片收 Ultra Plus 片。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 切片贴片;烘片 50–60 °C 20 min (禁超 60 °C) | method-local (`SEC-MOUNT`,切片贴附前置) + `op.incubation` 参数化 (定时加热保持) | 石蜡块 → 已贴片待脱蜡 | 方法绑定:50–60 °C、20 min、上限硬约束 | S1 steps 1–2 |
| organoid 收获→3.5% PFA 10 min RT→离心→HistoGel 包埋→石蜡 (S3) | method-local (`FIX-PARAFFIN`,固定包埋分支) + `op.centrifugation`(R5)/`op.liquid_transfer`(R1) 参数化 | 3D organoid → 石蜡块 | 方法绑定:3.5% PFA、10 min RT、300×g 3 min、HistoGel 37 °C/1 min | S3 §2.3.3 段 1 |
| PFA 4 °C 15 min→PBS×3→30% 蔗糖 overnight→明胶/蔗糖包埋→速冻→20 µm 切片 (S4) | method-local (`FIX-CRYO`,冷冻包埋分支) + 保持/转移参数化 | organoid → 冷冻块 → 20 µm 切片 | 方法绑定:4% PFA 15 min 4 °C;30% 蔗糖 overnight;−50~−30 °C 1–2 min;20 µm | S4 Box 1 steps 1–14 |
| 切片厚度/包埋朝向/排列密度 (≤6/块) | 方法前置 (非操作) | — → 可染切片 | 方法绑定;均匀性判据 UNKNOWN | S4 Box 1 steps 1/7/13 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (PFA/蔗糖/明胶/包埋液加除) | 切片链 (S1/S3/S4) + 既有多家族 | 复用 R1 | 体积/试剂全方法绑定 |
| 孵育/保持 (烘片/固定时钟/蔗糖沉降/速冻前聚合) | 切片链 | 复用 R3 elapsed-only;固定/聚合完成不断言 | 温度/时长各自绑定 |
| 离心 (organoid 收获 300×g) | 切片链 (S3) + 既有 | 复用 R5 | 300×g 3 min 方法绑定 |
| 通用 "fixation/embedding/sectioning" | 仅切片链;石蜡 vs 冷冻机理/试剂/终点不同 | 不建通用模块;两分支 method-local 分立 | 冻结 §3/审计 `#26` 同理;G2/G4 不过 |

---

## 3. P-HE — H&E 染色 (S2a + S2b)

绑定摘要 (S2a,试剂盒盖染法):脱蜡 (if necessary) → 水合 → 足量 Mayer's 苏木素盖片 5 min → 两换蒸馏水洗 → Bluing 10–15 s → 两换水洗 → 无水乙醇浸蘸吸干 → 足量 Eosin Y 2–3 min → 无水乙醇漂洗 → 三换无水乙醇脱水 → 透明 → 合成树脂封片;结果判读 (胞质淡粉/胶原粉/肌肉粉红/红细胞粉红/核蓝) 为方法终点,不建操作。绑定摘要 (S2b,浸染法,石蜡):2× 二甲苯各 10 min → 100% 乙醇 5 min×2 → 95% 5 min×2 → 水 5 min → Gill II 3 min → 水洗 → 蓝化剂 30 s → 水洗 → 95% 乙醇 brief → Eosin 3 min → 水快蘸 → 95% 乙醇 1 min → 100% 乙醇 10 dips×3 → Citrosol 2 min×3 → Cytoseal 60 + 玻璃盖片。S2b 另有冰冻 (10% 缓冲 formalin 20 min 前固定) 与 LCM 快染 (Mayer's 30 s/eosin 90 s/90–100% 乙醇各 10 s×2/二甲苯 30 s) 变体,各留方法内,不合并值。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 脱蜡:2× 二甲苯各 10 min (S2b);S2a "deparaffinize if necessary" exact 级数未给 | `op.washing` 参数化 (保留语义;溶剂置换) | 已贴片 → 脱蜡片 | 方法绑定:2×10 min;S2a 级数 UNKNOWN | S2b paraffin step 1;S2a Assay Procedure |
| 复水:100% 5 min×2 + 95% 5 min×2 + 水 5 min | `op.washing` 参数化 | 脱蜡片 → 水合片 | 方法绑定;系列/时长逐法 | S2b paraffin steps 2–3 |
| 苏木素:盖染 5 min (S2a) / 浸染 Gill II 3 min (S2b) | method-local (`STAIN-H`,核染分支) + `op.incubation` 参数化 | 水合片 → 核染片 | 方法绑定:5 min vs 3 min;染料配方按 kit/lot | S2a Assay;S2b step 4 |
| 水洗 → 蓝化 10–15 s (S2a) / 30 s (S2b) → 水洗 | method-local (`STAIN-H` 内步) + `op.washing` 参数化 | 核染片 → 蓝化片 | 方法绑定:10–15 s vs 30 s;不合并值 | S2a Assay;S2b step 5 |
| 95%/无水乙醇过渡 → Eosin 2–3 min (S2a) / 3 min + 水快蘸 (S2b) | method-local (`STAIN-E`,浆染分支) + `op.incubation` 参数化 | 蓝化片 → 复染片 | 方法绑定:2–3 min vs 3 min;醇过渡为方法注记 | S2a Assay;S2b steps 6–7 |
| 脱水:95% 1 min + 100% 10 dips×3 (S2b) / 三换无水乙醇 (S2a) | `op.washing` 参数化 | 复染片 → 脱水片 | 方法绑定;手法 (dips vs 换液) 为父操作备注 | S2a Assay;S2b step 8 |
| 透明 Citrosol 2 min×3 + Cytoseal 60 盖片 (S2b) / 透明 + 合成树脂 (S2a) | method-local (`MOUNT`,封片存档) | 脱水片 → 存档片 | 方法绑定;固化时间 UNKNOWN | S2a Assay;S2b steps 9–10 |
| 冰冻/LCM 变体 (前固定/快染时序) | 同上各变体 (method-local) | 同上 | 方法绑定;禁与石蜡值合并 | S2b frozen/LCM 节 |
| 免疫染色后禁 eosin;水性显色配 Mayer's (非 Gill's) | 方法约束 (非步骤) | — | 方法绑定 | S2b 页首注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/洗涤 (脱蜡/复水/水洗/脱水/透明,各系列) | H&E 双来源 + IHC (S1/S3) | 复用 R1/R4;溶剂/级数/时长全方法绑定 | S2a 级数 UNKNOWN 段 vs S2b 全绑定,不合并 |
| 孵育/保持 (苏木素/蓝化/eosin 时钟) | H&E 双来源 | 复用 R3 elapsed-only;结合/显色完成不断言 | 5 min vs 3 min vs 10–15 s vs 30 s 各自绑定 |
| 核染/浆染/蓝化/封片 | 仅 H&E | method-local;不晋升 | 染料化学/effect 不同;G1/G2 不过 |
| 通用 "H&E stain" 原子 | 表面多见但盖染 vs 浸染 vs LCM 快染分支大 | 不建通用模块 | 方法模板内固定顺序;G4 不过 |
| 颜色判读/核浆比 | 方法终点 | 非操作 (out-of-scope) | 无 |

---

## 4. P-IHC — IHC 染色 (S1 steps 3–23 + S3 染色段)

绑定摘要 (S1, DAB 三步法主路径):脱蜡复水 — Histoclear II 3×5 min → 100% 乙醇 2×5 min → 90%/70% 各 5 min → ddH2O 5 min,换液时上下提插排泡,此后禁干燥 (S1 step 3)。抗原修复二选一 (须按抗原优化,新抗原建议修复/不修复对照):HIER — 修复液浸没,微波煮沸后维持 15 min,柠檬酸盐内冷至 RT ~20 min (S1 step 4);或酶消化 — Proteinase K (20 µg/mL TE pH 8.0) / Trypsin (0.05% + 0.1% CaCl pH 7.8),10–20 min 37 °C 后冷至 RT (S1 step 5)。PBS 洗 5 min (coplin + 摇床低速,换液×2, S1 step 6)。过氧化物酶显色终点时 0.3% H2O2 15–40 min RT 封闭内源酶 (组织依赖优化, S1 step 7) → PBS 3×5 min (S1 step 8)。10% 正常血清 (二抗宿主种属) 100–150 µL/片盖组织,Parafilm 覆 (不拉伸、不触组织、排泡),湿盒 RT 1 h (S1 step 9) → 去 Parafilm,PBS 1×5 min (S1 step 10)。一抗 (厂商推荐稀释,封闭液配) 盖片 + Parafilm,湿盒 RT 2 h (低丰度至 12 h 4 °C, S1 step 11) → PBS 3× (S1 step 12)。 unconjugated 走三步法:生物素化二抗 (推荐稀释) 1 h RT 湿盒 (S1 step 15) → PBS 3× (S1 step 16) → avidin-HRP (推荐稀释) 30 min RT 湿盒 (S1 step 17) → PBS 3× (S1 step 18);生物素化一抗走二步法 (S1 steps 14/15 注记)。DAB:500 µL DAB stock + 2.5 µL 30% H2O2 / 50 mL PBS pH 7.4,镜下监控至棕色出现 (S1 step 19) → ddH2O 终止 (S1 step 20)。可选苏木素复染,镜下监控,流水洗去浮色 (禁损组织, S1 step 21)。脱水:反向 70%→100% 乙醇 + Histoclear (3×5 min each, S1 step 22) → 3 滴 Permount 盖片,干 1–2 h 后明场观察 (S1 step 23)。S3 organoid 荧光分支:0.1% Triton/PBS 10 min 通透 → 3% BSA + 1% NGS 1 h RT → 一抗 4 °C overnight → Alexa488/Cy3 二抗 → DAPI 复染 → 升序乙醇 + 二甲苯 → Entellan 封片;抗体 panel 见 Supplementary Table S1 (仅记存在)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 脱蜡复水 Histoclear II 3×5 + 乙醇系列 + 水 5 min;禁干燥 | `op.washing` 参数化 (保留语义) | 已贴片 → 水合片 | 方法绑定;禁干为硬约束 | S1 step 3 |
| HIER 煮沸 15 min + 冷却 ~20 min / 酶消化 10–20 min 37 °C (二选一,按抗原) | method-local (`RETRIEVE`,修复分支二选) + `op.incubation` 参数化 | 水合片 → 已修复片 | 方法绑定;修复/不修复对照为方法要求;exact 缓冲液按 lot | S1 steps 4–5 + 注记 |
| PBS 洗 5 min (换液×2,摇床低速,排泡) | `op.washing` 参数化 | 已修复片 → 待封闭片 | 方法绑定:5 min/次;手法为备注 | S1 step 6 |
| 0.3% H2O2 15–40 min RT (过氧化物酶终点才做) | method-local (`BLOCK-ENDO`,条件分支) | 待封闭片 → 已灭活片 | 方法绑定:15–40 min 组织依赖 | S1 step 7 |
| PBS 3×5 min | `op.washing` 参数化 | 已灭活片 → 已洗片 | 方法绑定 | S1 step 8 |
| 10% 血清 100–150 µL/片 1 h RT + Parafilm/湿盒/排泡 | method-local (`BLOCK-SERUM`) + `op.incubation` 参数化 | 已洗片 → 已封闭片 | 方法绑定:体积/血清种属/1 h | S1 step 9 |
| 一抗 (推荐稀释) 2 h RT,低丰度至 12 h 4 °C | method-local (`STAIN-1ST`) + `op.incubation` 参数化 | 已封闭片 → 已一抗片 | 方法绑定;稀释按 lot TDS | S1 step 11 + 注记 |
| 二抗路由:三步法 (生物素化二抗 1 h + avidin-HRP 30 min) / 二步法 (生物素化一抗直连) | method-local (`STAIN-2ND`/`STAIN-AVIDIN`,路由分立) + 保持参数化 | 已一抗片 → 已标记片 | 方法绑定:1 h / 30 min;路由错配 = 重做 | S1 steps 13–18 |
| DAB 显色 (500 µL + 2.5 µL/50 mL),镜下监控至棕色 | method-local (`DEVELOP-DAB`,终点分支) + 人工判断门控 | 已标记片 → 已显色片 | 方法绑定;完成=人工镜检,非 elapsed | S1 step 19 + 注记 |
| 苏木素复染 (可选,镜下监控) + 流水洗 | method-local (`STAIN-H` 切片语境,复用 §3 语义,值另绑) | 已显色片 → 已复染片 | 方法绑定 | S1 step 21 |
| 反向脱水 + Histoclear;Permount 3 滴;干 1–2 h | `op.washing` 参数化 + method-local (`MOUNT`) | 已复染片 → 存档片 | 方法绑定 | S1 steps 22–23 |
| S3 荧光分支:Triton 0.1% 10 min→BSA/NGS 1 h→一抗 overnight 4 °C→荧光二抗→DAPI→升序醇 + 二甲苯→Entellan | method-local (通透/封闭/一抗/二抗/复染/封片,各留方法内) + 保持/洗涤参数化 | 石蜡片/单层 → 已标记片 | 方法绑定:10 min/1 h/overnight;抗体按 Table S1 | S3 §2.3.3 段 2–3 |
| 一抗缺失/同型/内源对照 + 抗原修复优化矩阵 | 方法模板内对照 | 同批片 → 可比基线 | 方法绑定;任一缺即判读无效 | S1 steps 4–5 注记;B06 G-1 同理 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/洗涤/孵育 (脱蜡/修复/洗/封闭/抗体/显色/脱水全链) | IHC 双来源 + H&E + 切片链 | 复用 R1/R4/R3;体积/系列/时长全方法绑定 | S1/S3 步骤号见表 1 |
| 修复/内源封闭/血清封闭/一抗/二抗/亲和素/DAB/复染/封片 | 仅 IHC | method-local (各步分立);不晋升 | effect/终点/可逆性异质;审计 `#23/#26` |
| 通用 "IHC stain"/"antigen retrieval" | 表面多见但 HIER vs 酶 vs 不修复分支大 | 不建通用模块 | G2/G4 不过;修复条件逐抗原优化 |
| DAB 显色完成/复染终点 | 人工镜检门控 | 方法门控,非操作确认 | elapsed 不断言显色完成 |

---

## 5. P-IMG — 玻片原始成像 (S1 step 23 + S3 Axio Imager + S4 Table 1)

绑定摘要:明场 — DAB/复染片干 1–2 h 后 brightfield 镜检,显色中即用 brightfield 监控 (S1 steps 19/23)。荧光 — Axio Imager M2 + ApoTome.2 采代表性 IF 图,Fiji 后续 (S3 §2.3.3 段末);只到原始帧,通道/物镜/曝光/cube/仪器 ID 逐帧记录,定量/共定位/统计 out-of-scope。S4 Table 1 的 15 个抗体 (Pax6/Sox2/P-Vim/Tbr2/Tuj1/DCX/Foxg1/Ttr/Prox1/Fzd9/Nkx2.1/Reelin/Tbr1/Ctip2/Satb2 + 稀释度) 仅记 panel 存在,不作推荐值。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| DAB 显色中 brightfield 监控;干 1–2 h 后明场观察 | `FLUO_IMAGE` 明场 modality 绑定 (raw 采集 only) | 存档片 → 原始明场帧 | 方法绑定;曝光/白平衡 UNKNOWN | S1 steps 19/23 |
| Axio Imager M2 + ApoTome.2 代表性 IF 采集 | `FLUO_IMAGE` 参数化 (固定终点注记) | 已标记片 → 原始荧光帧集 | 方法绑定;通道 = 实际缀合物 (Alexa488/Cy3/DAPI) | S3 §2.3.3 段末 |
| Fiji/定量/统计/病理判读 | ToolAction + 分析 out-of-scope | 原始帧 → 派生结论 | 方法要求 | S3 §2.3.6–2.3.7 (不拆) |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 原始图像采集 (明场 modality + 荧光) | 切片链 + B06 IF + 既有 | 复用 R6;只到原始帧 | 曝光/物镜/cube 逐帧记录,无默认值 |
| 定量/判读/统计 | 方法终点/分析 | 一律非操作 | S3 统计节不拆 |

---

## 6. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| PFA/蔗糖/明胶/包埋液/HistoGel/脱蜡/复水/洗液/封闭/抗体/亲和素/DAB/苏木素/eosin/脱水/透明剂加除 (§§2–4) | `op.liquid_transfer` | 参数化复用;绑定体积/试剂/系列/孔板 (§§2–4 表 1) |
| 烘片/固定/蔗糖沉降/聚合/修复/封闭/抗体/显色/苏木素/eosin 时钟 + 干片/−80 °C 存窗 | `op.incubation` | 参数化复用;只断言 elapsed/条件;固定/聚合/修复/显色完成不断言 |
| 脱蜡/复水/PBS 洗/水洗/脱水/透明 (Histoclear/乙醇/水系列) | `op.washing` (保留语义) | 参数化复用;禁干 (IHC) 为硬约束;脱落 loss 显式 |
| organoid 收获/澄清 (300×g 3 min) | `op.centrifugation` | 参数化复用;转子/RPM 映射 UNKNOWN |
| 切片分样/块内多样本 (≤6/块) | `op.sample_aliquot` | 条件复用;父→块谱系注记 |
| 明场/荧光原始采集 (§5) | `FLUO_IMAGE` (明场为 modality 绑定) | 参数化复用;只到原始帧;通道/曝光/物镜逐帧记录 |
| 固定包埋分支 (石蜡/HistoGel vs 明胶-蔗糖冷冻)/贴片/速冻/修复二选/内源封闭/血清封闭/一抗/二抗/亲和素/DAB/核染/浆染/蓝化/封片 | method-local | 各留方法内;石蜡 vs 冷冻、HIER vs 酶、DAB vs 荧光不合并、不晋升 |
| 颜色判读/抗体 panel 选择/修复优化矩阵/图像定量/统计/临床诊断 | ToolAction / 方法终点 / out-of-scope | 一律非操作 |

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | P-PREP 切片制备 (S1/S3/S4) | P-HE (S2a/S2b) | P-IHC (S1/S3) | P-IMG (S1/S3) | 既有家族证据 | 结论 |
|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 包埋/固定液 | ● 盖染/浸染/系列 | ● 修复/封闭/抗体/DAB | — | B01–B13 全线 | shared,最高复用 |
| `op.incubation` | ● 烘片/固定/沉降 | ● 染料时钟 | ● 修复/封闭/抗体/显色 | — | B01–B13 | shared elapsed-only |
| `op.washing` | ● 收获洗/PBS | ● 脱蜡/复水/水洗/脱水 | ● 脱蜡/复水/PBS/脱水 | — | B01–B13 | shared;禁干硬约束 |
| `op.centrifugation` | ● 300×g 收获 | — | — | — | 多家族 | shared,值绑定 |
| `op.sample_aliquot` | ● 块内分样 | — | — | — | qPCR/培养/流式 | 条件复用 |
| `FLUO_IMAGE` (含明场 modality) | — | — | ○ 显色监控语境 | ● 明场/荧光原始帧 | B06/3D/既有 | shared raw |
| method-local (固定/修复/染色/封片) | ● 两分支分立 | ● H/E/蓝化分立 | ● 修复/封闭/抗体/DAB | — | 无 | 不合并 (显式拒绝) |
| ToolAction/终点 | — | ● 颜色判读 | ● 对照矩阵/优化 | ● 定量/统计/诊断 | — | 非操作 |

●=主路径使用;○=变体/监控语境使用;—=本批未使用。

防合并线(显式拒绝):石蜡包埋 ≠ 冷冻包埋 (试剂/可逆性/终点不同);HIER ≠ 酶消化 (热 vs 蛋白酶 effect 不同,逐抗原优化);DAB 显色 ≠ 荧光标记 (酶沉淀 vs 光子发射,读出链不同);盖染 vs 浸染 vs LCM 快染 (值禁合并);切片 H&E ≠ 培养细胞 IF 染色 (B06 边界);明场 ≠ 荧光定量 (审计 §6.2 modality 绑定,非两操作)。

## 8. 缺口清单(不能表达 / UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | IHC:修复/不修复对照 + 一抗缺失/同型 + 内源酶验证;H&E:良好固定对照组织/细胞涂片 (S2a)。任一缺即判读无效 |
| G-2 | 固定/取材上游未拆 | `source_missing` | 离体到固定间隔、固定液体积比/时长 (S3 只给 10 min RT 一例)、脱钙/大组织包埋程序——本批无来源,不补写 |
| G-3 | 切片/贴片力学量 | `source_missing` | 切片机型号/刀角/展片水温/贴片干燥条件——原文无量化,归 SOP 操作者判断 |
| G-4 | 修复优化逐抗原依赖 | `method_dependent` | HIER pH/时长 vs 酶种类/浓度/时长 vs 不修复;S1 只给起点,须逐抗体滴定;S3 柠檬酸钠 10 min 一例禁作通用 |
| G-5 | DAB/复染终点主观性 | 方法门控缺口 | "镜下监控至棕色/ desired staining"无 OD 阈值;elapsed 不断言显色完成;批次间可比性 UNKNOWN |
| G-6 | 抗体 lot 依赖 | `method_dependent` + `configuration_dependent` | 克隆/稀释/TDS、Table 1/S1 稀释度按 lot;荧光通道与 DAPI/Entellan 配伍逐批验证 |
| G-7 | 成像仪器值 | `configuration_dependent` + `source_missing` | 明场曝光/白平衡、荧光物镜/曝光/cube/仪器 ID——S1/S3/S4 均无值;plan 前须逐 run 绑定 |
| G-8 | 时间锚点 | 锚点缺口 | 烘片/固定/PFA 时钟各自 `per_assay`;储存 (−80 °C 数月/S1 干片) 为独立保持窗;孵育不断言固定/显色完成 |
| G-9 | 范围外 (误建模块风险) | `not_applicable` | 临床诊断/病理分级、电镜/特殊染色 (PAS/Masson/银染)、多重 IHC/空间组学、图像分割/定量/统计、通用 "fix/embed/stain/section" 模块——一律不建模 |

## 9. 晋升审查结论:零新公共模块

四问 (独立目的/稳定边界/独立确认/有意义复用) + 晋升四门 (G1 复用广度 ≥2 独立方法,域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| 通用 "histology/IHC" 操作 | ✗ (意图混杂) | ✗ | ✗ | ✗ | ✗ | **禁止新建** (题设要求组织学与培养不合并;制备与检测意图不同) |
| 通用 fixation/embedding/sectioning | ✓✓✓△ | ✗ (石蜡 vs 冷冻两分支 effect/终点不同,同一家族内变体≠两家族) | ✗ | ✗ | ✗ | method-local 两分支分立;不晋升 |
| 脱蜡/复水/脱水系列新原子 | ✓✓✓△ | ✓ | ✓ | ✗ (`op.washing` 已覆盖液体交换,溶剂系列为参数) | ✓ | 不新增,直接复用 `op.washing`;系列/时长方法绑定 |
| 通用 antigen retrieval | ✓✓✓△ | ✗ | ✗ (HIER vs 酶 vs 不修复后果不同) | ✗ | ✗ | method-local;不晋升 |
| 通用 H&E / IHC stain | ✓✓✓△ | ✗ (H/E/DAB/荧光 effect 各异) | ✗ | ✗ | ✗ | method-local;审计 `#23` 同理 |
| 明场玻片成像新原子 | ✓✓✓✓ | ✓ | ✓ | ✗ (审计 §6.2 已裁决为 modality 绑定) | ✓ | 不新增,复用 `FLUO_IMAGE` 明场绑定 |

因此本批全部候选留在 method-local / 既有操作参数化复用 / 方法门控 / ToolAction,不进入公共 Catalog,不写 PortSpec/Transition 全文,不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer` (+切片制备/脱蜡系列/抗体链)、`op.incubation` (+烘片/固定/修复/抗体/DAB 时钟)、`op.washing` (+Histoclear/乙醇系列 + 禁干约束)、`op.centrifugation` (+organoid 收获 300×g)、`op.sample_aliquot` (+块内分样)、`FLUO_IMAGE` (+明场 modality + 切片荧光终点)。

## 10. 核查与状态

- 已验证(直接读,2026-11-05,§1 定位节):S1 Thermo IHC 全文 (Materials + steps 1–23);S2a Abcam v3b 全文 (Overview + Assay Procedure);S2b UCSD 石蜡/冰冻/LCM 全文;S3 Kollmann §2.3.3 全文;S4 Lancaster Box 1 + Table 1 全文。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:Novus/protocols.io IHC 全文、任何 per-lot 片 (抗体/修复液/DAB)、切片机/成像仪手册、实验室 SOP 改编、已跑数据派生应用;检索摘要一律未转写为证据。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离 (§§2–5 表 1 vs 读数/仪器绑定行);样本跃迁有端口 (§§2–5 表 1);时间锚显式 (§8 G-8);部分列表标部分 (S4 Table 1 仅记存在);plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
