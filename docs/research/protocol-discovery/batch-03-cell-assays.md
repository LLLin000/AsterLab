# Batch-03 Cell Assays — CCK-8 / LDH / LIVE-DEAD / Scratch / Transwell

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 范围: 5 个基础细胞 assay 的湿实验步骤与原始读数；只记录差异与绑定，不重教通用操作原理。
- 规则: 三种活力读数不合并；图像分析/统计一律记为 ToolAction 或 out-of-scope，不建操作；新模块须过四维粒度判定 + ≥2 独立方法家族，否则留 method-local。本文件不提出任何新公共模块。
- 现有仓库记录仅用于 Reuse Map，不作步骤/数值证据；每条数值证据只引本文件 §1 的原始 protocol/厂商手册。

## 1. 来源索引

| # | Protocol | 权威类型 | 版本/定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Dojindo Cell Counting Kit-8 手册 CK04 (General Protocol: Cell Number Determination; Proliferation and Cytotoxicity Assay; Precautions 1–2, 5–6; FAQ 1, 3, 5) https://www.dojindo.com/manual/CK04/ | 厂商手册 | Rev. Apr 14, 2026 | 直接读：步骤 1–6 + 注意事项/FAQ 定位节 |
| S2 | Promega CytoTox 96 Non-Radioactive Cytotoxicity Assay TB163 (G1780, Rev. 8/25) §4.A 对照；§4.B 复方毒性步骤 1–8；§4.C 计算；§5.B–5.C 细胞介导对照/公式(仅参照)；§6 总细胞数变体；§7.A–7.B 背景/对照；Fig.1 原理 https://www.promega.com/-/media/files/resources/protocols/technical-bulletins/0/cytotox-96-nonradioactive-cytotoxicity-assay-protocol.pdf | 厂商手册 | Rev. 8/25 | 直接读：§4–§7 + Fig.1 |
| S3 | Thermo LIVE/DEAD Viability/Cytotoxicity Kit for Mammalian Cells Protocol (L3224, 显微镜路径) 步骤 3, 5, 6；光谱/储存表；Protocol tips；"不用于流式"范围注 https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/live-dead-viability-cytotoxicity-kit-for-mammalian-cells.html | 厂商手册 | L3224 当前页 | 直接读：步骤 3/5/6 + 光谱表 + tips |
| P1 | Varankar & Bapat, "Migratory Metrics of Wound Healing…in vitro Scratch Assays", Front Oncol 2018, doi:10.3389/fonc.2018.00633 (PROTOCOLS 文章) — 湿实验步骤 1–5 + Materials/Equipment/Reagents/Equipment Setup；分析步骤 6–21 不记录 https://www.frontiersin.org/journals/oncology/articles/10.3389/fonc.2018.00633/full | 同行评议 protocol 文章 | 2018 | 直接读：Step-Wise Procedures 步骤 1–5 全文 |
| P4 | Justus, Marie, Sanderlin & Yang, "Transwell In Vitro Cell Migration and Invasion Assays", Methods Mol Biol 2023;2644:349–359, doi:10.1007/978-1-0716-3052-5_22, PMC10335869 — §2 材料；§3.1 培养步骤 1–11；§3.2 迁移步骤 1–11；§3.4 CV 步骤 1–5；§3.5 DAPI 步骤 1–5；§3.7 非贴壁收集；§4 Notes 1–22；§3.3 侵袭 + §3.6 计数工具不记录 https://pmc.ncbi.nlm.nih.gov/articles/PMC10335869/ | Springer Protocols / Methods in Molecular Biology 章节 | 2023 | 直接读：§2–§4 定位节 |
| A1–A3 (参照变体, 非主证据) | Che, Wang & Ren, Bio-protocol Preprint "CCK-8 cell viability assay" (2021-12-23, 步骤 1–6)；Kaja 等, Curr Protoc Toxicol 2017 LDH (Basic/Alternate/Support Protocols)；Kumar 等, CSH Protocols 2018 LDH (Optimization 步骤 1–8 + Box 1 + Recipes, 镜像页 pp.1–4, 原链 403 待出版商核对 doi:10.1101/pdb.prot095497)；Johnson 等, Curr Protoc Cytom 2013 Alt. Protocol 8 (显微镜臂, 步骤 1–6) | 仓库型 protocol (Bio-protocol / Current Protocols / CSH) | 各自原文 | 直接读(本批记录为变体参照，主绑定仍以 S1/S2/S3/P1/P4 为准) |

未用作证据：Bio-protocol 需登录渲染体、Corning Transwell 指南(403)、Abcam 划痕页(超时)——均未读到正文，不引用。

## 2. P-CCK8 (S1；变体参照 A1/Bio-protocol S6)

绑定摘要：96 孔 100 µL/孔铺板(增殖例 5,000/孔，24 h 预孵) → 加药暴露(例 6/12/24/48 h) → 10 µL CCK-8/孔(他板型为总体积 10%) → 孵箱 1–4 h 显色 → 450 nm 读数(无 450 滤光片时 430–490 nm；浑浊加 ≥600 nm 参比，例 650 nm)；延迟读数可选 10 µL 1% SDS 或 0.1 M HCl/孔、避光 RT、稳定 24 h。Bio-protocol 变体：NP 细胞 5,000/孔/200 µL (只用中圈孔)；处理后换 90 µL 新鲜培养基 + 10 µL CCK-8；blank = 90 µL + 10 µL 无细胞；3 h 37 °C 避光；上机前轻柔摇匀；增殖 = As−Ab。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 铺板 100 µL/孔；例 5,000/孔 + 24 h 预孵；加药至终体积 | `op.liquid_transfer` 参数化 | 悬液 → 已铺板/已给药孔板 | 方法绑定：100 µL/孔，5,000/孔(例)，24 h 预孵，暴露 6/12/24/48 h(例) | S1 General Protocol 步骤 1–3 |
| 加 10 µL CCK-8/孔(他板型 10%) | `op.liquid_transfer` 参数化 | 已给药孔 → 已加试剂孔 | 方法绑定：10 µL/孔(96 孔)/10% 总体积；无气泡前置 | S1 步骤 2/4；FAQ 1 |
| 孵箱显色 1–4 h (37 °C 5% CO₂；白细胞可达 4 h/~10⁵/孔) | `op.incubation` 参数化 | 已加试剂孔 → 已显色孔(甲臜) | 方法绑定：1–4 h，细胞类型依赖，无默认值 | S1 步骤 3/5；Precaution 5 |
| 450 nm 读数(备选 430–490；参比 ≥600)；延迟读加 SDS/HCl | `ABS_READ` 参数化 + `op.liquid_transfer`(稳定剂，可选) | 已显色孔 → 孔板保留 + 原始 OD 矩阵 | 方法绑定：λ=450(备选 430–490)，参比 ≥600(例 650)；稳定剂 10 µL 1% SDS 或 0.1 M HCl | S1 步骤 4/6；Precaution 6；FAQ 5 |
| Bio-protocol 变体：90 µL 换液 + 10 µL CCK-8；blank 同配方无细胞；3 h 暗；摇匀后 450 nm | 同上三行，参数值不同 | 同上 | 方法绑定(变体)：90+10 µL，3 h 37 °C 暗，As−Ab | A1 步骤 1–6 |
| 空白扣除 → 活力% (S1 Fig.5 语境；S6 As−Ab) | ToolAction (确定性计算) | OD 矩阵 → 派生值 | 方法模板内公式，非操作 | S1/S6 推导节；本批不定义算法 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移(铺板/给药/加试剂) | 细胞培养、微生物、分子、生化读数板系 | shared (`op.liquid_transfer`) | S1 步骤 1–4；本批仅绑定体积 |
| 孵育/保持(预孵、暴露、显色) | 同上多家族 | shared (`op.incubation`) | S1 1–4 h 显色绑定；生长推断不在孵育内 |
| 吸光度读取(原始 OD) | 生化/免疫/微生物读数 | shared (`ABS_READ`) | S1 λ=450 + 参比绑定；计数/活力% 不在读数内 |
| 活力% 派生 | —(计算) | ToolAction | S6 As−Ab；标准曲线/还原剂干扰见 §7 |

## 3. P-LDH (S2 §4 化合物毒性路径；变体参照 A2/A3 + S2 §6)

绑定摘要：96 孔终体积 100–150 µL/孔(384 孔 25–30 µL) → 37 °C 暴露(LDH 半衰期 ~9 h 警告) → 最大裂解孔：试剂前 45 min 加 10 µL 10X/100 µL → 每孔取 50 µL 上清(384 孔 12.5 µL)至新平底透明酶标板 → 等体积 CytoTox 96 Reagent，避光 RT 30 min → 等体积 Stop，大气泡针挑破，1 h 内 490/492 nm 读数。仓库变体：CPT Toxicol 路径 50 µL 样品 + 50 µL 试剂(300–500 rpm 15 s 混匀)，60 min 暗 RT，50 µL 1 M 乙酸终止，OD490，总 LDH 臂 10 µL 10X/100 µL 上清摇 ≥3 min 镜检确认裂解，神经元铺板例 25,000/孔/100 µL；CSH 路径优化 0–20,000/100 µL + 15 µL 裂解、250×g 4 min、50+50 µL 底物 15–30 min 37 °C 暗、100 µL DMF/SDS (pH 4.7) 终止、490/参比 690 nm、血清 ≤5%、底物配方 0.054 M 乳酸/1.3 mM NAD⁺/0.66 mM INT/0.28 mM MPMS 于 0.2 M Tris-HCl pH 8.2；S2 §6 总细胞数变体：全部孔裂解(15 µL 10X/100 µL，45–60 min 37 °C)后同流程 harvesting + 显色。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 铺板 + 加药至 100–150 µL/孔；37 °C 暴露 | `op.liquid_transfer` + `op.incubation` 参数化 | 悬液 → 已暴露培养板 | 方法绑定：终体积 100–150 µL(384 孔 25–30 µL)；暴露时长归暴露 SOP，半衰期 ~9 h 警告 | S2 §4.B 步骤 1–3 + §1 |
| 最大裂解孔加 10 µL 10X/100 µL，试剂前 45 min | `op.liquid_transfer` + `op.incubation` 参数化 | 已暴露孔 → 含最大裂解孔的板 | 方法绑定：10 µL 10X/100 µL，45 min；不是新 "lyse" 原子 | S2 §4.B 步骤 4 + §5.B |
| 每孔取 50 µL 上清至新酶标板(384 孔 12.5 µL) | `op.sample_aliquot` 参数化 | 培养板 → 酶标板(子样本) + 培养板(已取样时间点终结) | 方法绑定：50/12.5 µL；新鲜平底透明板；谱系父→酶标孔 | S2 §4.B 步骤 5 |
| 等体积 Reagent，避光 RT 30 min；等体积 Stop | `op.liquid_transfer` + `op.incubation`(定时保持) 参数化 | 酶标板 → 已显色 → 已终止板 | 方法绑定：等体积；暗 30 min RT；1 h 内读数硬时钟 | S2 §4.B 步骤 6–7 |
| 490/492 nm 读数 | `ABS_READ` 参数化 | 已终止板 → 原始 OD 矩阵 | 方法绑定：λ=490 或 492，记录实际 λ | S2 §4.B 步骤 8 |
| % 细胞毒性推导(化合物路径 §4.C；细胞介导 §5.C；仓库 S7/S8 式) | ToolAction | OD 矩阵 + 对照 → 派生% | 方法模板内公式，非操作 | S2 §4.C/§5.C；S7/S8 对应节 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移(给药/裂解液/试剂/终止液) | 多家族 | shared | S2 步骤 4/6/7 等体积绑定 |
| 取样/分样(上清 harvest) | 细胞、微生物取样 | shared conditional (`op.sample_aliquot` + 谱系注记) | S2 步骤 5；培养板作时间点终结 |
| 孵育/保持(暴露、裂解 45 min、显色 30 min) | 多家族 | shared | S2 三段保持分离记时；stop→read ≤1 h |
| 吸光度读取 | 多家族 | shared | S2 λ=490/492 |
| % 细胞毒性/总细胞数换算 | —(计算) | ToolAction | 跨试剂盒(INT/WST、终止/动力学)不可比，未建模 |

## 4. P-LIVEDEAD 显微镜 (S3；变体参照 A3/Curr Protoc Cytometry Alt. 8)

绑定摘要：5 µL calcein AM (A) + 20 µL EthD-1 (B) → 10 mL DPBS 现配(≤1 天用完)；弃培养基，加 100–200 µL 染色液直接覆盖细胞；20–25 °C 30 min；calcein ~494/517 nm (FITC/GFP)、EthD-1 ~528/617 nm (RFP) 双通道成像；小瓶使用前融化；逐细胞类型从低端滴定、取背景最小的最高浓度；避光。仓库变体：2 µM calcein AM + 1 µM EthD-1 + 10 µM Hoechst 33342 于预热培养基，1 mL/腔室；死对照 100 µM digitonin (或 50% Triton 储备)；洗涤→脱色→暗再孵；FITC/Texas Red/DAPI 成像；calcein/EthD-1 30 min 内成像否则换新鲜培养基延迟读；Hoechst 为总数分母(双染轻微不重叠)。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 现配染色液 5 µL A + 20 µL B / 10 mL DPBS | `op.liquid_transfer` 参数化 | 染料储备 → 工作液(≤1 天) | 方法绑定：5 µL + 20 µL / 10 mL；滴定逐系优化 | S3 步骤 3 + tips |
| 弃培养基；加 100–200 µL 染色液 | `op.liquid_transfer`×2 (去废 + 加试剂)；血清酯酶预洗为 `op.washing` 参数化(未验证细节) | 活细胞 → 已染色细胞 | 方法绑定：100–200 µL 直接覆盖 | S3 步骤 4–5 |
| 20–25 °C 30 min 染色保持 | `op.incubation` 参数化 | 已染色 → 待采集成像 | 方法绑定：30 min，20–25 °C | S3 步骤 6 |
| 双通道原始采集(FITC/GFP + RFP) | `FLUO_IMAGE` 参数化 | 待采集 → 原始双通道图像 + 已成像细胞(光毒/漂白警告) | 方法绑定：滤块/激发/曝光/物镜逐帧记录；不固定(固定为禁用跃迁) | S3 光谱表 |
| 仓库变体：三染 + 死对照 + 洗涤/脱色 + 30 min 内成像 | 同上 + `op.washing` 参数化 | 同上 | 方法绑定(变体)：2 µM/1 µM/10 µM Hoechst，1 mL/腔，100 µM digitonin 死对照 | A3 Alt. 8 步骤 1–6 |
| 活/死判定与计数 | (分析) | 图像 → 派生判定 | out-of-scope；非操作 | S3 原理 + A3 Fig.9.2.8 非重叠警告 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移(弃液/加染料/工作液配制) | 多家族 | shared | S3 步骤 3–5 |
| 孵育/保持(30 min 染色) | 多家族 | shared | S3 步骤 6 |
| 荧光图像采集(原始) | 成像多家族 | shared (`FLUO_IMAGE`) | S3 双通道绑定；判读另立 |
| 洗涤(血清酯酶背景预洗) | 多家族 | shared 参数化(细节未验证) | S4 级细节缺口，见 §9 |
| 活/死 call 与计数 | —(分析) | 非操作(out-of-scope) | 阈值/单染参照/总数分母见 §7 |

## 5. P-SCRATCH (P1 步骤 1–5)

绑定摘要：0.5% TE (A4) / TPVG (OVCAR3, OVCA420) 500 µL，37 °C 2 min (TE)/1 min (TPVG)，等体积系别培养基终止，Neubauer 计数(胰酶条件系别依赖须优化) → 铺 24 孔 (Corning Costar CLS3527)，37 °C 24 h 至汇合(绝对数系别/孔板/倍增时间依赖，同系恒定) → 无菌 1×PBS 洗 → 无血清培养基 37 °C 24 h 饥饿驯化(完全剥夺或影响活力须优化) → 200 µL 枪头直线划痕 → 500 µL 无菌 1×PBS 轻柔洗去碎片(最小扰动 wound edge；高接触系小心 peeling) → 含 10 µg/mL mitomycin C 的无血清培养基(含研究因子同配，0.5 mg/mL 水储备配制；最小活力损失 + 最大分裂抑制为优化目标) → 台式 CO₂ 腔 24 孔板，10×，每划痕固定位置/焦平面，30 min 间隔 24 h，`.lif` 保存；CO₂/温度提前 ≥30 min 平衡；传代 ~15 min/系，划痕 ~15 min，采集 24 h；收获细胞冰上 ≤1 h。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 传代收获 + 计数(TE/TPVG，500 µL，37 °C 1–2 min，等体积终止) | 复用细胞培养传代复合块内操作 + 细胞浓度计数 | 源培养 → 已计数悬液 | 方法绑定：试剂/体积/时间/终止液系别化；计数方法恒定 | P1 步骤 1 + CRITICAL |
| 铺板 24 h 至汇合 | `OP-INOC` 参数化 + `op.incubation` | 悬液 → 汇合单层 | 方法绑定：24 孔 CLS3527；同系恒定数；24 h 37 °C | P1 步骤 2 + CRITICAL |
| PBS 洗 → 无血清 24 h 饥饿 | `op.washing` + `op.liquid_transfer` 参数化 + `op.incubation` | 单层 → 饥饿单层 | 方法绑定：1×PBS 洗；无血清；24 h 37 °C | P1 步骤 3 + CRITICAL |
| 200 µL 枪头直线划痕 | method-local `CAND-GAP-CREATE` (划痕变体) | 汇合单层 → 划伤单层(+碎片/双 edge) | 方法绑定：200 µL 枪头；可比宽度要求；压力/速度 UNKNOWN | P1 步骤 4 |
| 500 µL PBS 碎片洗 | `op.washing` lossy 参数化 | 划伤单层(+碎片) → 已洗伤口(碎片弃) | 方法绑定：500 µL 无菌 PBS，轻柔，edge 扰动最小 | P1 步骤 4 + CRITICAL |
| 加含 mitomycin C 10 µg/mL 的无血清培养基 | `op.liquid_transfer` 参数化 | 已洗伤口 → 已阻断伤口 | 方法绑定：10 µg/mL (0.5 mg/mL 储备)；因子同配 | P1 Reagents Setup + 步骤 4 |
| 24 h 延时成像(10×，30 min 间隔，固定位/焦平面) | `FLUO_IMAGE` 变体(明场/相差模态参数) + `op.incubation`(腔内保持) | 已阻断伤口 → 原始图像序列 | 方法绑定：10×，30 min×24 h，`.lif`；CO₂/温度预平衡 ≥30 min | P1 步骤 5 |
| 闭合度量/迁移速率 | (分析) | 图像序列 → 派生值 | out-of-scope；非操作 | P1 步骤 6–21(本批不记录) |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/接种/孵育/洗涤/计数/成像 | 多家族 | shared/参数化 | P1 步骤 1–5；见 §8 矩阵 |
| 划痕形成(机械去细胞 + edge) | 仅伤口闭合家族 | method-local (`CAND-GAP-CREATE` 划痕变体) | P1 步骤 4；G1/G2/G4 不过(与 insert 机制 effect 不兼容，不合并) |
| 闭合度量 | —(分析) | 非操作 | P1 步骤 6–21 出范围 |

## 6. P-TRANSWELL 迁移 (P4 §3.1/§3.2/§3.4/§3.5/§3.7；侵袭 §3.3 仅存在注记)

绑定摘要：培养至 80–90% 汇合、>95% 活力门控、团块过筛 → 贴壁收获：2× 5 mL RT 无 Ca²⁺/Mg²⁺ PBS 洗，2 mL 0.25% Trypsin-EDTA 旋匀 37 °C/5% CO₂ 3 min (经验性，限时，trypsin 敏感受体换非酶缓冲) + 3 mL 10% FBS 暖培养基轻柔单细胞重悬 → 335×g 5 min，两轮 PBS 洗后转迁移缓冲 (DMEM + 10 mM HEPES + 0.1% BSA, pH 7.4) → 15 µL + 15 µL 台盼蓝 96 孔混匀 4–5×，10 µL 上血球计，调至 1×10⁶/mL → 上室：100 µL (1×10⁵) 不触膜接种，37 °C/5% CO₂ 静置 10 min；下室：600 µL 迁移缓冲 + 趋化剂(例 5 ng/mL C5a 或 10% NIH3T3 条件培养基)，缓慢加无气泡、膜液直接接触 → 2–5 h 37 °C/5% CO₂ (系别依赖) → 非贴壁路径：镊取 insert，600 µL 下室液按 §6.5 计数；贴壁路径：棉签轻柔擦净上表面(重复至无残留) → 新孔 1 mL 70% 乙醇 RT 10–15 min 固定、空孔晾干 → CV 0.2% 水溶液 RT 3–5 min，2× PBS/水洗晾干，10×/20× 多代表视野(苏木精备选；避上表面 edge 残留区)；或 DAPI 1 µg/mL (1 mg/mL 储备 1 µL → 1000 µL PBS)，600 µL/孔暗 10 min，3× 1 mL PBS 洗，10×/20× 荧光代表视野(Hoechst 33358 备选)。无菌边界：贴壁 §3.2 步骤 5 / 非贴壁步骤 3 之前须无菌，之后 clear/fix/stain 不再要求。侵袭变体(§3.3)：上膜先铺 ECM (Matrigel 1:1–1:2，50–100 µL，37 °C/5% CO₂ 固化 30 min–1 h，窗口 16–24 h)——本批不绑定、不计划，只标记边界。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 培养至 80–90% + >95% 活力门控 + 收获/洗涤/重悬/计数调至 1×10⁶/mL | 传代复合块内操作 + 细胞浓度计数(门控) | 源培养 → 1×10⁶/mL 单细胞迁移缓冲悬液 | 方法绑定：80–90%，>95%，2×5 mL PBS，0.25% TE 3 min，335×g 5 min，缓冲配方，15+15 µL 台盼蓝 | P4 §3.1 步骤 1–11 + Notes 5–7 |
| 上室接种 100 µL + 下室 600 µL 趋化剂组装梯度 | `OP-INOC` + `op.liquid_transfer` 参数化 | 悬液/趋化剂 → 上室已种 + 下室梯度 | 方法绑定：6.5 mm PC 膜 insert，5.0/8.0 µm 孔(试点定)，100 µL/600 µL，C5a 5 ng/mL 或 10% 条件培养基(例)，无气泡膜液接触 | P4 §3.2 步骤 1–2 + Notes 2/8–12 |
| 2–5 h 迁移保持 | `op.incubation` 参数化 | 已组装梯度 → 已迁移 insert | 方法绑定：2–5 h 37 °C/5% CO₂，系别依赖 | P4 §3.2 步骤 3 + Note 13 |
| 棉签擦净上表面未迁移细胞 | method-local `CAND-APICAL-CLEAR` | 已迁移 insert → 上表面已清除 + 下表面保留 | 方法绑定：棉签轻柔擦拭、重复至净；力/遍数 UNKNOWN | P4 §3.2 步骤 7 |
| 70% 乙醇 10–15 min 固定 | fix 方法内步骤参数化(`IF-FIXPERMBLOCK` 固定部分) | 已清除 insert → 已固定膜 | 方法绑定：1 mL 70% 乙醇，RT 10–15 min | P4 §3.2 步骤 8–10 |
| CV 0.2% 3–5 min / DAPI 1 µg/mL 暗 10 min + 洗涤 | 方法内染色步骤参数化 + `op.washing` 参数化 | 已固定膜 → 已染色膜 | 方法绑定：CV 0.2% 3–5 min + 2×洗；DAPI 1 µg/mL 600 µL 暗 10 min + 3×1 mL PBS 洗 | P4 §3.4 步骤 1–5；§3.5 步骤 1–5 |
| 10×/20× 多代表视野原始采集 / 非贴壁下室液收集计数 | `FLUO_IMAGE` 变体(明场/荧光) / 收集 + 细胞浓度计数 | 已染色膜 → 原始图像；下室液 → 计数 | 方法绑定：10×/20× 代表视野(避 edge 残留)；600 µL 体积规则 | P4 §3.4–§3.5；§3.7 |
| 迁移细胞计数/剂量拟合 | (分析) | 图像/计数 → 派生值 | out-of-scope；非操作 | P4 §3.6(本批不记录) |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/接种/孵育/洗涤/固定/染色/成像/计数 | 多家族 | shared/参数化(染色/固定为方法内步骤) | P4 §§3.1–3.2/3.4–3.5/3.7 |
| 上表面清除(棉签擦拭，分面命运分叉) | 仅 Transwell | method-local (`CAND-APICAL-CLEAR`) | P4 §3.2 步骤 7；G1/G3 不过(单方法)，洗涤不能表达但不晋升 |
| 通用 "staining"/"fixation" | 表面多见但 effect 不稳定 | 不建通用模块 | CV/抗体/活力染料 effect 不同；DAPI/CV 留方法内 |
| 侵袭 ECM 铺胶 | 仅侵袭 | 不建(本批 out-of-scope) | P4 §3.3 存在注记，需独立记录 |

## 7. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 铺板/给药/加试剂(CCK-8, LDH, LIVE/DEAD, 划痕, Transwell) | `op.liquid_transfer` | 参数化复用；绑定体积/试剂/孔板(§§2–6) |
| 显色/暴露/染色/迁移/贴附保持 | `op.incubation` | 参数化复用；绑定温度/时长/锚点(`t₀` 见各表)；生长/迁移完成不断言 |
| 饥饿换液/碎片洗/膜洗/预洗 | `op.washing`(保留语义)；碎片/脱落场景用 lossy 变体 | 参数化复用；划痕碎片弃、insert 部分脱落 loss 显式 |
| LDH 上清 harvest | `op.sample_aliquot` | 参数化复用；父→酶标孔谱系；培养板作时间点终结 |
| CCK-8/LDH 读数 | `ABS_READ` | 参数化复用；绑定 λ(450 / 490-492 / 参比)；派生另立 |
| LIVE/DEAD 双通道、划痕/Transwell 膜成像 | `FLUO_IMAGE`(模态参数：荧光/明场/相差) | 参数化复用；只到原始图像；判读/计数 out-of-scope |
| 计数门控(Transwell 调浓度、划痕/Transwell 种前计数) | 细胞浓度计数 | 直接复用；活力判读归方法 |
| 固定/染色(CV/DAPI/乙醇) | 方法内固定/染色步骤 | 方法内参数化；不建通用 stain/fix 模块 |
| 划痕形成 | `CAND-GAP-CREATE` 划痕变体 | method-local；不晋升(G1/G2/G4 不过) |
| 上表面清除 | `CAND-APICAL-CLEAR` | method-local；不晋升(G1/G3 不过) |
| 活力%/毒性%/闭合度/迁移计数/拟合 | — | ToolAction 或 out-of-scope；一律非操作 |

## 8. 跨方法公共操作覆盖矩阵

| 公共操作 | CCK-8 | LDH | LIVE/DEAD 镜检 | Scratch | Transwell 迁移 | 备注 |
|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 铺板/给药/10 µL 试剂 | ● 给药/裂解/试剂/终止 | ● 配液/弃液/加染液 | ● 铺板/换液/加 MMC | ● 上室/下室/固定/染色 | 最高复用；全参数化 |
| `op.incubation` | ● 预孵/暴露/显色 1–4 h | ● 暴露/裂解 45 min/显色 30 min | ● 染色 30 min | ● 饥饿 24 h/成像间保持 | ● 静置 10 min/迁移 2–5 h | 只断言 elapsed/偏差 |
| `op.washing`(含 lossy 变体) | ○ (换液语境) | ○ (收获前洗，仓库路径) | ●/○ 预洗(细节缺) | ● 碎片洗 lossy | ● 膜洗 | 去碎片/脱落一律 lossy 显式 |
| `op.sample_aliquot` | — | ● 50/12.5 µL 上清 | — | ○ 种前分样 | ○ 种前分样 | LDH 为谱系关键步 |
| `ABS_READ` | ● 450(+参比) | ● 490/492 | — | — | — | 原始 OD 止；派生另立 |
| `FLUO_IMAGE`(含明场/相差模态) | — | — | ● 双通道 | ● 延时序列 | ● 膜多视野/荧光 | 原始采集止；分析另立 |
| 细胞浓度计数 | ○ (密度窗语境) | ○ (优化语境) | — | ● 种前门控 | ● 调 1×10⁶/mL | 计数≠活力 call |
| method-local 划痕/清除 | — | — | — | ● 划痕 | ● 上表面清除 | 各留方法内；不合并、不晋升 |
| ToolAction/分析 | ● 活力% | ● 毒性% | ● 活死判读 | ● 闭合度量 | ● 迁移计数 | 全部非操作 |

●=主路径使用；○=变体/门控语境使用；—=本批未使用。

三读数互不可比(§9 G-5)：CCK-8=脱氢酶甲臜(代谢，1–4 h 窗)；LDH=膜泄漏酶(半衰期 ~9 h，上清时间点终结)；LIVE/DEAD=逐细胞酯酶/通透性快照(30 min 染色)。对照系各自独立(§9 G-1)，不得跨读数借用分母。

## 9. 缺口清单(不能表达/UNKNOWN；分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | CCK-8：同板无细胞 blank + vehicle + 系别标准曲线 + 还原剂背景核查；LDH 化合物路径：No-Cell + Vehicle + Maximum(试剂前 45 min 裂解)，细胞介导另加 Effector/Target Spontaneous + Target Maximum + Volume Correction + Medium Background(＋可选阳性对照)，三/四复孔 + 靶细胞数预优化；LIVE/DEAD：未染色 + 单染 + 逐系滴定 + Hoechst 总数分母 + 死对照(digitonin/Triton，S9 值)；划痕：MMC 阻断 + 无血清背景 + 可比宽度；Transwell：浓度试点 + 梯度完整 + >95% 活力门控。任一读数缺对照即无意义 |
| G-2 | 派生公式归属 | ToolAction，非操作 | CCK-8 As−Ab/活力%；LDH 化合物 `100×Exp/Max`(背景扣除后)，细胞介导 `100×(Exp−EffSpont−TgtSpont)/(TgtMax−TgtSpont)`，仓库 S7 对照归一或 LDHr/LDHt，S8 Exp/Max (690 参比扣除)；LIVE/DEAD call 需阈值 + 单染参照 + 总数分母(双染轻微不重叠)；闭合/迁移计数全部下游 |
| G-3 | 力学/手法量 | `source_missing` | 枪头压力/速度、PBS 洗/棉签力·遍数、insert 按压/揭起力；P1/P4 均无量化，归 SOP 操作者判断 |
| G-4 | 系别依赖值 | `method_dependent` | 铺板数(CCK-8 1,000–25,000/孔窗内仍逐系定；P1 汇合数；Transwell 吸引剂浓度/窗口 2–5 h)；MMC 活力影响；饥饿耐受；L3224 µM 工作浓度/滴定终点；LDH 暴露最优/延迟低估曲线 |
| G-5 | 读数不可比性 | 模型边界 | CCK-8/LDH/LIVE-DEAD、划痕/insert/Transwell 相互不可换算；insert 与划痕creation physics 不同(P2 p.2)；跨试剂盒 LDH (INT/WST、终止/动力学) 等价性未建立 |
| G-6 | 试剂/耗材批与设备态 | `configuration_dependent` / `device_state` | 血清 LDH 背景/酚红贡献； per-lot 染料/膜/insert；读数/成像包络(分辨/线性/台稳定/曝光)；移液校准/孵箱均匀性；S5 跨试剂盒、S4 预洗/死对照细节未验证 |
| G-7 | 固定后不可逆 | 跃迁约束 | L3224 染色后固定禁用；LDH 酶标板终止后终结；CCK-8 加试剂后不可回退 unstained；划痕碎片弃、insert 部分脱落、上表面清除分面分叉一律显式 loss/effect，不套无损洗涤 |
| G-8 | 时间锚点 | 锚点缺口 | CCK-8 显色 `per_assay` + 读数秒级，暴露时钟归暴露 SOP；LDH stop→read ≤1 h 硬时钟；LIVE/DEAD 30 min `per_assay` + 逐帧采集；划痕 `t₀`=划痕(饥饿 24 h 前置，采集 24 h)；Transwell `t₀`=接种/梯度组装；孵育不断言生长/迁移完成 |
| G-9 | 范围外(误建模块风险) | `not_applicable` | 图像分割/edge/计数软件、统计/拟合/管线、MTT/XTT/MTS/WST-1、⁵¹Cr、流式 LIVE/DEAD 设门、ATP 发光、集落存活、ECM 侵袭铺胶程序、3D/微流控/单细胞追踪——本批一律不记录、不建模 |

## 10. 核查与状态

- 已验证(直接读，2026-11-05，§1 定位节)：S1 CCK-8 绑定；S2 LDH §4–§7 + Fig.1；S3 L3224 步骤 3/5/6 + 光谱表 + tips；P1 湿实验步骤 1–5 + 材料/设置；P4 §2–§3/§4 Notes；A1–A3 仓库变体(主证据仍为 S1/S2/S3/P1/P4)。单人 `agent-verified`，仍需第二人逐字段复核；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：S4 滴定/洗/死对照细节、S5 跨试剂盒等价、CSH 出版商版措辞(镜像 pp.1–4 外)、任何仪器操作手册/单机构建表、per-lot 片、实验室 SOP 改编、系别优化数、任何已跑数据的派生应用。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器规格分离(§§5–6 仅能力 + 用过型号作例)；样本跃迁有端口(§§2–6 表 1)；时间锚 `t₀` 显式；工作值标协议归属；部分列表标部分；plan/actual/raw/derived 分离(原始 OD/图像保留，派生下游)；无 JSON；UNKNOWN 永不填零。
