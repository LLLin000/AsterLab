# Batch 19 — Organoid Differentiation: Cerebral (EB → Neural Induction → Matrigel → Agitation) / Intestinal (WNT-Depletion) — Passaging, Media Switching, Maturation Readouts

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 类器官定向分化湿实验 only:传代/拆分、培养基切换 (维持 → 分化)、成熟维持、湿端原始读出 (形态门控 + 染色/发光/电阻读出的存在性)。B11 已覆盖 ULA/悬滴/spheroid sheet/ALI,本批不复拆;Matrigel dome 建立已由 Bio-protocol 4298 (小鼠肠道) 覆盖,本批只记分化/传代/切换链,不重立 dome 点胶。固定/包埋/切片/染色执行链归 B18,本批只记 "何时固定/测什么 marker" 的方法要求,不复拆染色步骤 (题设禁止组织学与类器官培养合并)。共培养分选、organ-on-chip、生物打印、磁悬浮无来源,一律不建模。
- 规则: 现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;每条数值/行为证据只引本文件 §1 的直接读来源;取材/伦理/临床信息 out-of-scope (S3 从手术切除样本起步,本批自 crypt 分离后起记,不记任何患者数据);图像分割/定量、曲线拟合、统计一律 out-of-scope,不建操作;现有 `op.*` 只记身份和差异,不复述契约;不新增公共模块除非四问 + 晋升四门全过;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用接种密度、通用分化天数、通用 split 比例、通用成熟 marker 阈值;所有温度/时间/体积/浓度/离心条件均为所引协议的绑定值,不得跨谱系/细胞系复制。脑/肠分化机理互不合并 (神经外胚层自组织 vs WNT 撤除驱动成熟,输入/切换/确认均不同)。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| P1 | Lancaster & Knoblich, Nat Protoc 2014 (PMC4160653) 全文:EB 制作 (Option A/B) + 喂养 (steps 2–4) + 神经诱导 (steps 5–7) + Matrigel 滴 (steps 8–17) + 静置扩张 (steps 18–19) + 生物反应器/摇床生长 (steps 20–21) + Box 1/Anticipated Results | 同行评议 protocol (PMC 开放全文) | day 0 EB → day 6 神经诱导 → day 11 Matrigel → day 15 搅拌 → 长期维持 | 直接读:全文转写。9,000 活细胞/150 µL (96 孔 U 底 ULA);隔天半量换液;ROCKi 50 µM (1:100) + 低 bFGF 4 ng/mL 至变亮或 350–400 µm (通常前 4 天);350–600 µm 窗喂养 (去 ROCKi/bFGF);~500–600 µm 变亮光滑边转 24 孔 500 µL 神经诱导液;48 h 后补 500 µL;4–5 天见放射状假复层神经上皮即包埋;30 µL Matrigel/滴 (4×4 网格/60 mm 皿);37 °C 20–30 min 聚合 + 5 mL 无维生素 A 分化液;4 天静置 (24 h 后喂一次);day 15 转 125 mL 旋瓶 75–100 mL (≤32 个/瓶) 或摇床 85 rpm,换含维生素 A 分化液;摇床每 3–4 天 / 旋瓶每周全换 |
| P2 | Kollmann et al. 2023, Front Cell Dev Biol 11:1223032, §2.2 人肠 organoid 培养/传代/2D/分化 + §2.3.4–2.3.6 读出方法 | 同行评议论文方法节 | IntestiCult 维持 → 传代 (TrypLE double-tip) → 2D 转化 (Geltrex 10%) → WNT 撤除分化 4 天;读出:CellTiter-Glo (2D/3D)、Dispase 解离、TER | 直接读:方法节全文转写。包埋 50% BMM + 50% IntestiCult,5–10 µL 滴,37 °C 倒置 20–30 min,Y-27632 10 µM,每 2–3 天换 IntestiCult;传代:自带培养基冲散 + PBS 补洗 → 300×g 3 min → BM 洗 → 1 mL TrypLE + double-tip 强吹 10–20× → BM 终止 → 300×g 3 min → BM 洗 → 重包埋;2D:TrypLE 37 °C ~5 min 至单细胞 + 10% Geltrex/BM 室温 30 min 包被晾干后接种;分化液 = BM + R-Spondin-1 500 ng/mL + Noggin 100 ng/mL + EGF 50 ng/mL + 烟酰胺 10 mM + SB202190 10 µM + Gastrin 10 nM + A83-01 500 nM + LY2157299 500 nM,4 天。取材/伦理段 (§2.2.1–2.2.2 手术来源) out-of-scope,自 crypt 沉淀重悬起记 |
| R0 | Bio-protocol 4298 小鼠肠 organoid Matrigel dome (96 孔 10 µL/滴,聚合 37 °C 30 min,换液 2–3 天,传代 500×g/1:2–1:3/吹打 20–40×);B11 (ULA/悬滴/片/ALI);B18 (切片/染色执行链);审计 `#24` 解离/`#30` dome/`#31` ULA/`#32` lossy 换液 | 既有仓库记录 | 只记身份,不复述契约,不作证据 | R1 `op.liquid_transfer`;R2 取样/分样;R3 `op.incubation` (elapsed only);R4 `op.washing` 保留 + lossy 变体;R5 `op.centrifugation`;R6 细胞浓度计数;R7 `FLUO_IMAGE` (raw);缺口 `G-1` 发光板读 (domain-shared) |

检索摘要 (`search-summary-only`,非证据):STAR human intestinal organoid 分化/CRISPR 页 (403 未直读)、STEMCELL/R&D/Tocris 脑 organoid 概述页、MiMB WNT 撤除综述摘仅用于立项定位;凡数值以 §1 直接读为准,摘要值禁引作证据。

---

## 2. P-CEREBRAL — 脑 organoid 分化 (P1;hPSC → EB → 神经诱导 → Matrigel → 搅拌成熟)

绑定摘要:起始质控 — 6 孔板 70–80% 汇合、无分化 pluripotent 形态 colonies (P1 step 1 + Fig 2a 门控);一孔约产一整 96 孔板 EB。Feeder-dependent 路径:Dispase 1 mL/孔 20–30 min (至边缘卷起,至多 40 min) → 1 mL 低 bFGF 培养基重拍打脱落 (不带 MEF) → 15 mL 管静置 ~1 min 自然沉降 colonies,吸弃单细胞/MEF 上清 (×2) → 1 mL Trypsin/EDTA 37 °C 2 min + 1 mL 抑制剂,1 mL 枪头吹散至浑浊 → 2×5 µL 计数 (Trypan blue,平均) + 8 mL 低 bFGF 培养基。Feeder-free 路径:0.5 mM EDTA 600 µL/孔 4 min → 弃 EDTA 加 1 mL Accutase 再 4 min → 1 mL mTeSR1 喷打脱落 → 2 mL/15 mL 管吹散 → 计数 + 3 mL mTeSR1。汇合:270×g 5 min → 1 mL 低 bFGF + ROCKi (1:100,终 50 µM) 重悬单细胞 → 补体积至 9,000 活细胞/150 µL → 96 孔 U 底 ULA 每孔 150 µL。喂养:24 h 后镜检 (小 EB + 周围死细胞为正常);隔天轻轻吸去约半量 (不扰 EB) + 补 150 µL 新液 (终体积不苛求);ROCKi + 低 bFGF 至 EB 变亮或 >350–400 µm (通常前 4 天);350–600 µm 窗内改纯 hES 培养基 (去 ROCKi/bFGF) 隔天喂。神经诱导:~500–600 µm、变亮、边缘光滑 (通常 day 6) → 切口 1–1.5 mm 的 200 µL 枪头 (禁铲/刮) 逐个转 24 孔 ULA 500 µL 神经诱导液 (DMEM-F12 + 1% N2 + 1% Glutamax + 1% NEAA + 肝素终 1 µg/mL);48 h 后再加 500 µL;再 2 天后镜检:外圈更亮 = 神经外胚层,见放射状假复层上皮即达包埋窗口 (4–5 天);非放射状透亮芽可再留 1–2 天,但已放射区会收缩,禁 overdue。包埋:Matrigel 冰上 4 °C 1–2 h 融 (500 µL 管);Parafilm 覆 200 µL 枪头盒压 4×4 凹坑,剪方块置 60 mm 皿 ( glove + 70% 乙醇消毒,不可高压灭菌);切口 1.5–2 mm 枪头逐个转组织至凹坑 → 未切口 200 µL 枪头吸净余液 (枪口后垫组织防吸入) → 每组织滴 ~30 µL Matrigel (一次做 16 个,防干) → 10 µL 枪头即刻居中 (室温即凝) → 37 °C 20–30 min 聚合 → 加 5 mL 无维生素 A 脑分化液 → 翻转 Parafilm 抖落液滴入皿。静置扩张:24 h 后镜检,1–3 天见透亮神经上皮芽 + 液腔 (迁移出的成纤维样细胞为正常,不扰芽);再 24 h 后喂无维生素 A 分化液 (倾斜沉滴,尽量吸净,补 5 mL),再静置 48 h。搅拌成熟:4 天后切口 ~3 mm 的 1 mL 枪头转 125 mL 旋瓶 (75–100 mL 含维生素 A 分化液,CO₂ 箱内磁力搅拌台) 或 60 mm 皿换含维生素 A 液放 85 rpm 摇床;每瓶 ≤2 皿 (≤32 个,防融合);摇床每 3–4 天 / 旋瓶每周全换;维持至多 1 年以上 (2 个月停长,5–6 个月后缩小)。成熟门控 (Anticipated Results,方法要求):day 5–6 外圈透亮;神经诱导后 60–80% 出神经外胚层;Matrigel 后 30–80% 出神经上皮芽 (批间变异大,须同批对照);day 15–20 太大转解剖镜;早期 (12–20 天) Sox2/Pax6 + 偶见神经元,1 个月后 Tuj1/DCX,后见 FOXG1/TTR/PROX1/FZD9/NKX2.1/Reelin/TBR1/CTIP2/SATB2 分区 (Table 1 panel 仅记存在,染色执行归 B18)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 起始 colonies 质控 (70–80%,无分化形态) | 方法门控 (人工判断,非操作) | hPSC 培养 → 可起 EB 群体 | 方法绑定;分化 >5–10% 禁起 | P1 step 1 + Fig 2a |
| Dispase/EDTA + Accutase/Trypsin 解离 → 计数调 9,000/150 µL → U 底 ULA 接种 | method-local (解离,审计 `#24`) + 细胞浓度计数(R6) + `op.liquid_transfer`(R1) | 贴壁 colonies → 接种用单细胞悬液 → 已接种 EB 板 | 方法绑定:Dispase 20–30 min / EDTA 4 min + Accutase 4 min / Trypsin 2 min;270×g 5 min;ROCKi 50 µM;9,000/150 µL | P1 step 1A/1B |
| 隔天半量换液 (ROCKi + 低 bFGF → 去添加,350–600 µm 窗) | `op.liquid_transfer`(R1,换液) + 方法门控 (变亮/粒径判定) | 已接种 EB → 维持中 EB | 方法绑定:半量/隔天;350–400 µm 与 350–600 µm 双阈;完成=镜检,非 elapsed | P1 steps 2–4 |
| ~500–600 µm 转 24 孔 500 µL 神经诱导液;48 h 补 500 µL;4–5 天放射状上皮窗口 | `op.liquid_transfer`(R1,转移/补液) + `op.incubation`(R3,保持) + 方法门控 (诱导完成判定) | EB → 神经外胚层聚集体 | 方法绑定:500 µL + 500 µL;肝素 1 µg/mL;窗口判据为人工镜检 | P1 steps 5–7 |
| Matrigel 滴包埋 (~30 µL/滴,居中,37 °C 20–30 min,加 5 mL 无 VitA 液) | method-local (dome 包埋,审计 `#30`/B4298 同类) + 保持参数化 | 神经聚集体 → Matrigel 内组织 | 方法绑定:30 µL、16 滴/皿、20–30 min、无 VitA | P1 steps 8–17 |
| 静置 4 天扩张 (24 h 镜检 + 喂一次 + 48 h) | `op.incubation`(R3) + `op.liquid_transfer`(R1,喂养) + 方法门控 (出芽判定) | 包埋组织 → 出芽组织 | 方法绑定;出芽 30–80% 为批间变异注记,非承诺值 | P1 steps 18–19 |
| 转旋瓶/摇床 + 切换含 VitA 液;节律全换;长期维持 | method-local (搅拌切换,独立候选见 §6) + `op.liquid_transfer`(R1) | 出芽组织 → 成熟中 organoid | 方法绑定:75–100 mL/瓶、≤32 个/瓶、85 rpm、3–4 天/每周;VitA 切换为硬性方法分支 | P1 steps 20–21 |
| Box 1 固定/包埋/切片/染色;Table 1 panel | 交接 B18 执行链 (本批不拆);panel 只记存在 | 成熟 organoid → 切片数据集 | 方法要求;厚度 20 µm 等值归 B18 | P1 Box 1/Table 1 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (解离/接种/半量换液/转移/补液/Matrigel/全换) | 脑分化 (P1) + 肠分化 (P2) | 复用 R1;稀释/半量整体=Composite | 体积/节律全方法绑定 |
| 孵育/保持 (EB/诱导/聚合/扩张/长期维持) | 脑分化 (P1) + 肠分化 (P2) | 复用 R3 elapsed-only;聚集/诱导/出芽/成熟结果不在内 | 完成=人工门控 |
| 离心 (270×g 汇合) | 脑分化 (P1) + 肠分化 (P2 300×g) | 复用 R5;值各自绑定,不合并 | 270×g 5 min vs 300×g 3 min |
| 细胞浓度计数 (9,000/150 µL 门控) | 脑分化 (P1) + 肠分化 (P2) | 复用 R6 计数 only | Trypan/TC10 方法绑定 |
| ULA 接种/EB 形成本身 | 仅脑分化一起点 | method-local/composite;不晋升 | 冻结 §5.3/审计 `#31` 同理 |
| Matrigel 滴包埋 | 脑分化 (P1) + B4298 鼠肠 dome | method-local/composite;不晋升 | 审计 `#30`;体积/皿型分支大 |
| 搅拌切换 (旋瓶 vs 摇床 + VitA 切换) | 仅脑分化 | method-local;不晋升 (§6) | 单家族;与换液 intent 不同 |
| 成熟 marker panel/分区判读 | 方法终点/分析 out-of-scope | 非操作;染色执行归 B18 | Table 1 只记存在 |

---

## 3. P-INTESTINAL — 人肠 organoid 维持/传代/2D 转化/WNT 撤除分化 (P2;自 crypt 沉淀起记,取材上游 out-of-scope)

绑定摘要:维持 — crypt 沉淀重悬于 4 °C 50% BMM + 50% IntestiCult,5–10 µL 滴种 Cellstar 板,37 °C 倒置 20–30 min 固化后翻正,加 37 °C IntestiCult + Y-27632 10 µM;每 2–3 天换 IntestiCult (8 系,多 passage,本批不记供体信息)。传代 — 自带培养基冲散凝胶滴收 organoid (+ PBS 补洗大残胶) → 300×g 3 min → 弃上清 → 1 mL BM 洗 → 300×g 3 min → 沉淀重悬 1 mL TrypLE + double-tip (10 µL 空头套 1,000 µL 头上) 强吹 10–20× 机械拆分 → 1 mL BM 终止 → 300×g 3 min → BM 洗 → 按维持法重包埋 (传代节律/比例原文未给通用值,UNKNOWN)。2D 转化 — 同上收集洗涤 → 1 mL TrypLE 37 °C ~5 min (反复镜检) + double-tip 强吹至单细胞 → 1 mL BM 终止洗涤 → 重悬 IntestiCult/Y-27632 → 接种于 10% Geltrex/BM 室温 30 min 包被晾干后的 chamber/coverslip/电极/小室/多孔板 (顶腔加培养基培养,刺激时双腔加)。分化 — 增殖 organoid 换分化液 (WNT 撤除):BM + hR-Spondin-1 500 ng/mL + mNoggin 100 ng/mL + hEGF 50 ng/mL + 烟酰胺 10 mM + SB202190 10 µM + [Leu15]-Gastrin I 10 nM + A83-01 500 nM + LY2157299 500 nM,4 天。成熟门控 (结果节,方法要求):薄壁囊泡 → 柱状上皮 → 隐窝-绒毛样 (2 天/4 天镜检);Ki-67/LGR5/CHGA/CLDN2 下降,IAP/MUC2/DSC2 上升,LYZ 不变;2D TER 9 h 起升,24 h 达 6.47±1.07 倍基线后平台;Dispase 解离碎片降至 0.39 倍未分化对照。读出方法 (§2.3.4–2.3.6,本批只记湿端存在):CellTiter-Glo 2.0 (2D)/3D (3D organoid),96 孔,振荡 10 min 避光 + RT 静置 25 min,GENios Pro 测 ATP 发光,归一化对照 (发光读数缺口 `G-1`);Dispase-II 2.4 U/mL HBSS 37 °C 30 min + 上下吹 5× 剪切,拍照计数 (ToolAction);TER 电极/小室,汇合后起测,归一化基线。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| crypt 沉淀 → 50% BMM + 50% IntestiCult 重悬 → 5–10 µL 滴种 → 倒置固化 20–30 min → 加 IntestiCult/Y-27632 10 µM | method-local (dome 包埋,同 P1/B4298 类,值另绑) + `op.liquid_transfer`(R1) + `op.incubation`(R3) | crypt 沉淀 → 维持中 3D organoid | 方法绑定:50/50、5–10 µL、20–30 min、10 µM | P2 §2.2.2 (沉淀后段) |
| 每 2–3 天换 IntestiCult | `op.washing` lossy 变体(R4/`#32`) | 维持中 organoid → 维持中 organoid + 废液 | 方法绑定:2–3 天节律;残留/吸位 UNKNOWN | P2 §2.2.2 末句 |
| 传代:冲散收 → 300×g 3 min → BM 洗 → TrypLE + double-tip 10–20× → BM 终止 → 300×g → BM 洗 → 重包埋 | method-local (机械拆分,审计 `#24` 类) + `op.centrifugation`(R5)/`op.liquid_transfer`(R1) 参数化 | 维持中 organoid → 拆分后悬液 → 新包埋 organoid | 方法绑定:300×g 3 min;10–20×;终止比 1:1;split 比例/节律 UNKNOWN | P2 §2.2.3 |
| 2D 转化:TrypLE 37 °C ~5 min + double-tip 至单细胞 → 10% Geltrex 包被 (RT 30 min) → IntestiCult/Y-27632 接种 | method-local (2D 转化分支,独立候选见 §6) + 保持/转移参数化 | 3D organoid → 2D 单层 | 方法绑定:~5 min、10% Geltrex、30 min;汇合判据人工 | P2 §2.2.4 |
| 切换分化液 (WNT 撤除配方) 4 天 | method-local (培养基切换,独立候选见 §6) | 增殖 organoid → 分化中 organoid | 方法绑定:8 组分浓度 + 4 天;完成=门控非 elapsed | P2 §2.2.5 |
| 成熟门控:形态演变 + marker 升降 + TER 平台 + 碎片下降 | 方法门控 (人工/仪器门控,非操作) | 分化中 organoid → 分化成熟判定 | 方法要求;阈值禁作通用 | P2 §3.2/Fig 2 |
| CellTiter-Glo 2.0/3D 活力;Dispase 解离;TER | 方法模板读出 (湿端存在性) + ToolAction/分析 out-of-scope | 分化 organoid → 原始发光/碎片图像/电阻迹 | 方法绑定;发光缺口 `G-1` | P2 §2.3.4–2.3.6 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (包埋/换液/终止/重悬/包被/接种/切换) | 肠分化 (P2) + 脑分化 (P1) | 复用 R1 | 体积/配方全方法绑定 |
| 孵育/保持 (固化/维持/拆分时钟/分化 4 天/包被) | 肠分化 (P2) + 脑分化 | 复用 R3 elapsed-only;分化/成熟不在内 | 完成=门控 |
| 离心 (300×g 收/洗/终止后收) | 肠分化 (P2) + 脑分化 (P1 270×g) | 复用 R5;值各自绑定 | 300×g 3 min RT |
| 取样/分样 (传代等分谱系) | 肠分化 (P2) | 复用 R2;守恒 UNKNOWN | split 比例 UNKNOWN |
| 3D 换液 | 肠分化 (P2) + 脑分化喂养 | 复用 lossy 变体语境 | 残留/吸位方法绑定 |
| 发光活力读数 | 肠分化读出语境 | 缺口 `G-1` domain-shared;Custom 占位 | 不复用 `ABS_READ` |
| 机械拆分/2D 转化/培养基切换 | 仅肠分化 (拆分与脑吹散机理不同) | method-local;不晋升 (§6) | 单家族;G1 不过 |
| TER/碎片计数/统计 | 方法终点/ToolAction/分析 | 一律非操作 | §2.3.6–2.3.7 不拆 |

---

## 4. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 解离/接种/半量喂养/转移/补液/包埋/终止/重悬/包被/全换/切换 (P1–P2) | `op.liquid_transfer` | 参数化复用;绑定体积/配方/节律 (§§2–3 表 1) |
| EB/诱导/聚合/扩张/固化/维持/分化保持 (P1 day 0–15+;P2 2–3 天节律 + 4 天分化) | `op.incubation` | 参数化复用;绑定时长/温度/锚点;聚集/诱导/出芽/分化/成熟结果一律不在内 |
| 维持/分化期换液 (P1 喂养/全换;P2 2–3 天换液) | `op.washing` lossy 变体(R4/`#32`) | 复用变体语境;残留/吸位/防扰动为方法绑定 |
| 汇合/收获/洗涤沉降 (P1 270×g 5 min;P2 300×g 3 min) | `op.centrifugation` | 参数化复用;RCF/时间/转子各自绑定,不合并 |
| 计数调密度 (P1 9,000/150 µL;Trypan/TC10) | 细胞浓度计数(R6) | 复用 (计数 only);活力/形成/分化判定归门控 |
| 传代等分/滴种谱系 | `sampling`(R2) | 参数化复用;split 比例/数量守恒 UNKNOWN (沿用缺口) |
| Box 1/Table 1 染色 panel;S3 IF 染色执行 | 交接 B18 执行链 | 本批只记存在 + 方法要求 (何时固定/测什么);不复拆步骤 |
| 形态演变/Ki-67/LGR5/IAP/MUC2/TER 平台/碎片计数/统计 | 方法门控/ToolAction/分析 out-of-scope | 一律非操作 |
| 发光活力 (CTG 2.0/3D 10 min + 25 min) | 缺口 `G-1` (domain-shared),方法模板内 Custom 占位 | 不复用 `ABS_READ`;本批不立读数模块 |
| EB 形成/Matrigel 滴包埋/机械拆分/2D 转化/培养基切换/搅拌切换 | method-local | 各留方法模板内;脑/肠不合并 (§6 防合并线) |

## 5. 跨方法公共操作覆盖矩阵

● 直接使用;△ 条件/分支使用;○ 刻意不用 (防合并);— 本方法无此步。

| 操作 | P-CEREBRAL 脑分化 (P1) | P-INTESTINAL 肠分化 (P2) | 既有家族证据 (B4298/B11/B18) | 结论 |
|---|---|---|---|---|
| `op.liquid_transfer` | ● 解离/接种/喂养/转移/包埋/全换 | ● 包埋/换液/终止/包被/切换 | B01–B13 全线 | shared,最高复用 |
| `op.incubation` | ● EB/诱导/聚合/扩张/长期 | ● 固化/维持/分化 4 天 | B01–B13 | shared elapsed-only |
| `op.washing` lossy | ● 喂养/全换 (防扰动) | ● 2–3 天换液 | 生物膜/3D 既有 | shared variant |
| `op.centrifugation` | ● 270×g 汇合 | ● 300×g 收/洗 | 传代/Transwell/转化 | shared,值不合并 |
| 细胞浓度计数 | ● 9,000/150 µL | ○ (收获语境) | B01/R6 | shared 计数 only |
| `sampling` 谱系 | — | ● 传代等分 | qPCR/培养/微生物 | 条件复用 |
| `FLUO_IMAGE` raw | ○ 门控观察语境 | ○ 形态门控语境 | B06/B18/3D | 按需复用 raw |
| 发光读数 `G-1` | — | ○ CTG 占位 | ATP/reporter 域共享 | domain-shared,不建模 |
| EB 形成/滴包埋 method-local | ● | ○ (鼠 dome 参照,不合并值) | B4298/B11 | 不合并 |
| 机械拆分/2D 转化/切换 | ○ (吹散≠拆分) | ● | 无 | 不合并 |
| 搅拌切换 (旋瓶/摇床 + VitA) | ● | — | 无 | 不合并 |
| B18 染色执行链 | ○ 交接 (Box 1/Table 1) | ○ 交接 (S3 段) | B18 主责 | 本批不拆 (防合并) |

防合并线(显式拒绝):脑神经诱导 ≠ 肠 WNT 撤除 (自组织 vs 信号撤除,切换配方/确认 marker 均不同);EB 悬浮聚集 ≠ dome 包埋聚集 (容器/保留机制/确认不同);Trypsin/Accutase 单细胞化 ≠ TrypLE double-tip 机械拆分 (终点组织量级不同,审计 `#24`);2D 转化 (3D→单层,Geltrex 包被) ≠ 传代重包埋 (谱系方向相反);搅拌切换 (VitA + 通气) ≠ 部分换液 (分化 intent vs 液体交换);旋瓶 ≠ 摇床 ≠ 静置 (通气/剪切分支,值禁合并);发光 ≠ 吸光度 (审计 §4.4);染色执行 (B18) ≠ 分化决策 (B19)——题设禁止合并,本批只记交接点。

## 6. 缺口清单(不能表达 / UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G1 | 传代节律/比例 UNKNOWN (P2) | `source_missing` | P2 未给肠 organoid 传代间隔/split 比例通用值;P1 脑无常规传代 (长期整养);B4298 鼠 1:2–1:3 禁跨种复制 |
| G2 | 切换窗口逐系/批依赖 | `method_dependent` | 脑:EB 500–600 µm + 变亮 + 放射状三判据须同现,overdue 收缩;肠:4 天 vs 文献 3–5/7 天分支 (STAR 摘要未直读);供体/系/代次偏移未量化 |
| G3 | VitA/添加剂切换因果 | `method_dependent` | P1 无 VitA → 含 VitA 为硬性分支,浓度/时序按配方;禁作通用 "maturation switch" 默认 |
| G4 | 搅拌分支未归一 | `configuration_dependent` | 旋瓶 (75–100 mL/周换/≤32 个) vs 摇床 (85 rpm/3–4 天) vs 静置;剪切/融合/蒸发差异未量化;设备准入 (CO₂ 箱内搅拌台) 逐 lab 绑定 |
| G5 | 拆分力度/损失 | `source_missing` | double-tip 10–20×、吹打 20/40× (B4298)、单细胞确认 (反复镜检) 均为定性;viability/回收率未声明;lossy 显式,不塞普通参数 |
| G6 | 成熟完成无通用阈值 | 方法门控缺口 | 透亮/出芽率 (30–80%)、marker 升降、TER 6.47 倍平台、碎片 0.39 倍均为所引实验的观测值,禁作放行标准;须同批对照 (P1 Anticipated Results 明确要求) |
| G7 | 发光读数原子缺失 | 缺口 `G-1` | CTG 2.0/3D 10 min + 25 min + GENios Pro 仅湿端存在性;量程/积分/仪器能力 `configuration_dependent` + `source_missing`;方法模板内 Custom 占位 |
| G8 | 时间锚点 | 锚点缺口 | 脑 `t₀`=EB 接种 day 0,诱导/包埋/搅拌各为独立切换锚;肠 `t₀`=分化液切换;储存 (−80 °C 块数月, P1 Box 1) 为独立保持窗;孵育不断言分化/成熟完成 |
| G9 | 范围外 (误建模块风险) | `not_applicable` | 手术取材/伦理/患者数据、共培养/CRISPR 工程 (STAR 未直读)、organ-on-chip/生物打印/磁悬浮/悬滴重立 (B11 已覆)、移植/动物/临床、电镜/空间组学、图像分割/定量/统计、通用 "differentiate/mature/passage" 模块——一律不建模 |

## 7. 晋升审查结论:零新公共模块

四问 (独立目的/稳定边界/独立确认/有意义复用) + 晋升四门 (G1 复用广度 ≥2 独立方法,域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| 通用 "organoid differentiation/maturation" | ✗ (脑/肠意图混杂) | ✗ | ✗ | ✗ | ✗ | **禁止新建** (题设要求;两谱系分立记录) |
| EB 形成/神经诱导/Matrigel 滴/搅拌切换 | ✓✓✓△ | ✗ (仅脑一家族) | ✓ (各自内) | ✗ | △ | method-local/composite;不晋升 |
| 机械拆分/2D 转化/培养基切换 (WNT 撤除) | ✓✓✓✓ | ✗ (仅肠一家族) | ✓ (各自内) | ✗ | ✓ | method-local;不晋升 |
| 3D 换液/喂养新原子 | ✓✓✓△ | ✓ | ✓ | ✗ (`op.washing` lossy 已覆盖) | ✓ | 不新增,直接复用 lossy 变体 (审计 `#32`) |
| 发光成熟读数 | ✓✓✓✓ | ✓ (ATP/reporter 域内) | ~ (板读 vs 成像须切分) | ✓ | ✓ (切分后) | domain-shared candidate (`G-1`);本批 Custom 占位,不停留立项 |
| 成熟 marker panel/TER/碎片判读 | ✓△✓✗ (阈值不稳) | ✗ | ✗ | ✗ | ✗ | 方法门控/ToolAction;**永不建模块** |

因此本批全部候选留在 method-local / 既有操作参数化复用 / 方法门控 / ToolAction,不进入公共 Catalog,不写 PortSpec/Transition 全文,不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer` (+EB 接种/半量喂养/Matrigel/切换家族)、`op.incubation` (+诱导/扩张/分化 4 天/长期维持绑定)、`op.washing` lossy (+喂养防扰动/2–3 天换液语境)、`op.centrifugation` (+270×g/300×g 双档语境)、细胞浓度计数 (+9,000/150 µL 门控语境)、`sampling` (+传代等分谱系)。

## 8. 核查与状态

- 已验证(直接读,2026-11-05,§1 定位节):P1 PMC 全文 (EB A/B + 喂养 + 诱导 + 包埋 + 静置 + 搅拌 + Box 1/Table 1/Anticipated Results);P2 Kollmann §2.2.2–2.2.5 + §2.3.3–2.3.6 全文 (维持/传代/2D/分化/读出方法)。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:P2 §2.2.1–2.2.2 取材上游 (out-of-scope,未转写为证据)、STAR 分化/CRISPR 全文 (403)、任何 per-lot 片 (BMM/Matrigel/Geltrex/抗体/分化因子)、旋瓶/摇床/读板机手册、实验室 SOP 改编、已跑数据派生应用;检索摘要一律未转写为证据。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离 (§§2–3 表 1 vs 读数/仪器绑定行);样本跃迁有端口 (§§2–3 表 1);时间锚显式 (§6 G-8);部分列表标部分 (Table 1 panel/STAR 摘要仅记存在);plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);患者数据零记录;无 JSON;UNKNOWN 永不填零。
