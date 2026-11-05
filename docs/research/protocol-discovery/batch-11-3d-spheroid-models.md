# Batch 11 — 3D Spheroid Models Other Than Matrigel Dome: ULA Spheroid Formation, Hanging Drop, Spheroid Sheet, Air-Liquid Interface

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: Matrigel dome 之外的 3D 模型湿实验 only:ULA 圆底板球体形成、悬滴法 (经典 lid 法 + InSphero Akura 商業平台)、spheroid sheet (微孔板成球 + 网格夹层融合成膜)、气液界面 ALI 分化 (Transwell 双腔 → 去顶液)。只记录湿实验步骤与原始读数;图像分割/定量算法、剂量曲线拟合、统计推断、TEER 数值解释、组织学判读一律 out-of-scope,记为 ToolAction/方法终点,不建操作。
- 规则: 不建通用 `3D culture` 操作;ULA / hanging drop / ALI 机理互不合并 (重力聚集 vs 防粘聚集 vs 极化分化,输入/保留机制/确认方式不同);现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;每条数值/行为证据只引本文件 §1 的直接读来源,检索摘要一律 `search-summary-only` 不作证据;不新增公共模块除非四问 + 晋升四门全过,domain-shared 允许但本批无晋升;不做代码/UI/DB 变更。
- 禁止声明: 本文件不含任何通用接种密度、通用球体直径、通用换液体积、通用 ALI 天数;所有温度/时间/体积/浓度/残留量均为所引协议的绑定值,不得跨细胞系/耗材复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| P1 | Corning Spheroid Microplates CLS-AN-235 + CLS-AN-446 + Sigma spheroid microplate 页 | 厂商应用指南/协议页 | ULA 圆底板 (96/384/1536) 球体形成、体积/残留/密度/添加剂/离心助聚 | 检索摘要级 (`search-summary-only`,非证据):Corning PDF 直读被 403 拦截、Sigma 页超时,仅转写检索摘要中的体积/残留/密度窗;凡数值以 UNKNOWN 方法绑定记,不得作 Compatible 证据 |
| P2a | Foty 2011, A Simple Hanging Drop Cell Culture Protocol for Generation of 3D Spheroids, JoVE 2720 (PMC3197119) | 同行评议 protocol (JoVE 视频 + 全文) | §1 单细胞悬液 + §2 悬滴形成 + §3 Notes + Representative Results 全文 | 直接读:页面全文转写;90% 汇合、2× PBS 洗、2 mL 0.05% trypsin-1 mM EDTA/100 mm、2 mL 完全培养基终止、40 µL 10 mg/mL DNase 5 min RT、200×g 5 min、1 mL 洗×2、2 mL 重悬、2.5×10⁶/mL、60 mm 皿底 5 mL PBS 水合腔、盖内面 20 µL 枪打 10 µL 滴 (≥20 滴/皿)、37 °C/5% CO₂/95% 湿度、每日镜检、成片后转圆底玻璃摇瓶 3 mL 摇床水浴成球 |
| P2b | InSphero Akura PLUS Hanging Drop System Quick Start Guide (+ Product Manual 检索摘要佐证) | 厂商快查指南 (PDF) | 40 µL 滴/孔、SureDrop 密封、70 µL 转移、250 RCF 沉降、Akura 96 板换液 | 直接读:Quick Start 全文转写; humidifier pad + 20 mL 0.5× PBS 浸泡 ~5 min、40 µL/滴、250–500/滴 (长期生长) / 2,500+/滴 (非增殖/大球)、均质化吹打、盖 + 37 °C CO₂、~4 天收紧、Akura 96 预湿 40 µL、3 针定位、<10 µL/s 加 70 µL 落球、250 RCF 2 min、定容 70 µL、换液沿 ledge <30 µL/s 吸留 5–7 µL + <50 µL/s 加 70 µL |
| P3 | Le et al. 2025, Spheroid Sheets: A Scalable Platform for Producing Tissue Membrane Constructs, Bio-protocol 15(22): e5501 (Mater Today Bio 31:101526 验证) | 同行评议 protocol (开放获取全文) | Procedure A hMSC 扩增 + B 微孔板成球 + C 网格装配/软骨分化 14–21 天全文 | 直接读:PDF 全文转写;复苏/传代密度/50% 换液/90% 传代/倍增公式、Elplasia/AggreWell 成球 (例 2,000/球 × 2,885/孔 = 8.88 M/孔)、1 mL 预湿 + 300×g 1 min 除泡、TrypLE 3–5 min + 300×g 3–5 min、1 mL 滴加 + 100×g 1 min 慢加减速 + 沿壁补 3 mL、过夜–3 天收获 (禁久置)、1 mL 枪冲洗收获 + 10–20 s 沉降重用上清、500 µL 通透测试、~500 球/cm² (300 µm 球 monolayer)、双尼龙网 (150×200 µm) + 框沉底培养、每 2–3 天换液、2–3 天融合/≥10 天可剥/14 天内易剥 |
| P4 | Propagenix, Air-Liquid Interface (ALI) Differentiation Protocol for Human Bronchial Epithelial Cells (EpiX 扩增 → PneumaCult-ALI) | 实验室 protocol (厂商培养基体系引用) | 24 孔 Transwell submerged → day 0 airlift → 3–4 周分化全文 (2 页) | 直接读:PDF 全文转写;80–90% 收获、Modified EpiX (+75 µL 2 M CaCl₂/100 mL → 1.5 mM)、24 孔顶 200–250 µL / 底 800 µL–1 mL、300 µL PBS 预湿 5–10 min (禁触膜)、500,000/mL × 250 µL = 125,000/孔 (~400,000/cm²)、submerged 至完全单层 (隔天双腔换液)、day 0 去双腔液 + 底腔 PneumaCult-ALI 800 µL–1 mL (顶腔不再加液)、底腔每 2–3 天换液、每周 300 µL PBS 顶腔漂洗 mucin、EVOM2/ENDOHM-6 + 空白对照可选、21 天多纤毛/产粘液假复层 (H&E/acetylated-tubulin/alcian blue 切片仅记存在) |
| R0 | Promega CellTiter-Glo 3D (TM412) + Assay Guidance Manual 3D spheroid 章 | 厂商手册/指南 (读出参照) | 1:1 加样、5 min 剧烈裂解、25–60 min 稳定、0.25–1 s/孔 top-read | 检索摘要级 (`search-summary-only`,非证据):室温平衡 30 min、~450 rpm、白壁板、终点破坏性、无空间分辨、>350 µm/ATP>10 µM 线性外;仅作读出边界对照,不拆表 |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 取样/分样 sampling atomic (父/子谱系);R3 `op.incubation` (elapsed/条件 only,不断言聚集/融合/分化/生长);R4 `op.washing` preserving + lossy 变体 (3D 审计 `#5` 部分换液);R5 `op.centrifugation`;R6 细胞浓度计数 (count only);R7 `FLUO_IMAGE` (raw 采集 only);R8 `ABS_READ` (raw OD only);缺口 `G-1` 发光板读 (domain-shared candidate,本批复用其边界);审计 `#24` 解离、`#30` dome/composite、`#31` ULA 形成/composite、`#32` 3D 换液/lossy、`#34` ToolAction 均同类处理。

检索摘要(`search-summary-only`,非证据):CLS-AN-235/446 要点、Sigma ULA 页、Perfecta3D 384 孔板摘要、Akura Product Manual/FAQ 摘要、PneumaCult-ALI PIS 摘要、CTG3D TM412/Quick Protocol 摘要仅用于立项定位;凡数值以 §1 直接读为准,P1/R0 数值禁引作证据。

---

## 2. P1 — ULA 圆底板球体形成 (Corning CLS-AN-235 谱系;摘要级)

绑定摘要(仅摘要可转写,全部 `search-summary-only`):ULA 共价亲水惰性涂层 + 圆底 (96/384/1536,不透光壁/清底) 同板生成-培养-读数,免转移;单细胞悬液 (可选 40 µm 过滤) 多道/自动分装,禁划伤涂层;37 °C/5% CO₂;多数系 ~24 h 成球,部分需延长/短暂离心助聚;每日镜检;feeding 沿壁加/吸,残留 96/384 孔 10–20 µL、1536 孔 ~5 µL (半量换液可减扰动);起始体积 96 孔 75–200 (max 300)、384 孔 25–75 (max 90)、1536 孔 5–10 (max 14) µL;96 孔测试窗 40–10,000/100 µL (0/24/48/72 h + CellTiter-Glo 3D 例);添加剂甲基纤维素增粘 / Matrigel overlay 促收紧 (逐系);离心 ~300×g 助聚/居中 (ECM 语境);宽口吸头转移/包埋;Matrigel 回收冷却/冷缓冲稀释/Cell Recovery;解离 Accutase/Trypsin/TrypLE/EDTA。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 制单细胞悬液 (可选 40 µm 过滤/滤帽管) | `sampling`(R2) + 细胞浓度计数(R6)语境,细节 UNKNOWN | 培养 → 接种用悬液 | 方法绑定;过滤为可选分支 | P1 摘要;数值禁引 |
| 分装至 ULA 圆底孔 (手动多道/自动;禁划伤涂层) | `op.liquid_transfer`(R1) + 方法前置(涂层保护,非操作) | 悬液 + ULA 板 → 已接种孔 | 方法绑定;体积/密度逐系,本批无通用值 | P1 摘要 |
| 37 °C/5% CO₂ 保持;~24 h 聚集 (部分系 48 h/仍松散) | `op.incubation`(R3,仅 elapsed) + method-local(聚集结果,非孵育断言) | 已接种孔 → 含球体孔 | 方法绑定;时间到≠形成完成 | P1 摘要 |
| 可选短暂离心助聚/居中 (~300×g,ECM 语境) | `op.centrifugation`(R5)条件分支 | 已接种孔 → 居中聚集体 | 方法绑定;RCF/时间 UNKNOWN (摘要值禁引) | P1 摘要 (CLS-AN-446 语境) |
| 每日镜检确认形成/紧密度;未形成→延长/助聚/换条件 | 方法门控(人工判断,非操作) | 含球体孔 → 形成判定 | 方法绑定 | P1 摘要 |
| 沿壁部分换液,留残留 (96/384 10–20 µL;1536 ~5 µL) | `op.washing` lossy 变体(R4/`#32`) | 含球体孔 → 保留球体 + 废液 | 方法绑定;残留为关键参数,禁通用默认 | P1 摘要 |
| 宽口吸头转移/包埋/回收;Matrigel overlay 语境冷却/稀释 | `op.liquid_transfer`(R1) + method-local(基质处理归 R1/R3 组合,本批不展开) | 含球体孔 → 转移后样本 | 方法绑定 | P1 摘要 |
| 终点 CellTiter-Glo 3D / 荧光/发光板读 (同板免转移) | Custom 站位 (发光读数缺口 `G-1`,R0 边界) + `FLUO_IMAGE`(R7 raw 语境) | 含球体孔 → 原始读数 | 方法绑定;读数解释归方法/ToolAction | P1 摘要 + R0 摘要 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (接种/换液/转移) | ULA(P1);悬滴/片/ALI (P2–P4 直接读) | 复用 R1;稀释整体=Composite | P1 值全 UNKNOWN,仅直接读家族作证据 |
| 孵育/保持 | ULA(P1);全部直接读家族 | 复用 R3 elapsed-only;聚集/融合/分化不在内 | 时间到≠形成完成 (冻结 §6.3) |
| 3D 部分换液 | ULA(P1);片/ALI basal 换液 | 复用 lossy 变体语境;残留/吸位方法绑定 | 不建 "3D 洗涤" 新原子 (审计 `#32`) |
| 离心助聚 | ULA(P1 摘要);片 B2/B6 (P3 直接读 100–300×g) | 条件复用 R5;值各自绑定 | P1 值禁引,证据只记 P3 |
| ULA 球体形成本身 | 仅 ULA 一类 | method-local/composite;不晋升 (§8) | G1/G3 不过;不与悬滴/ALI 合并 |
| 活力/成像读数 | ULA 终点语境 | 发光缺口 `G-1`;成像复用 R7 raw | 吸光度禁代发光 (审计 §4.4) |

---

## 3. P2 — 悬滴法 (P2a Foty 经典 lid 法 + P2b Akura PLUS 平台;二者不合并为一值)

绑定摘要(P2a):指数/贴壁 90% 汇合起步;2× PBS 洗、2 mL 0.05% trypsin-1 mM EDTA/100 mm、37 °C 至脱壁、2 mL 完全培养基终止 + 5 mL 枪吹散、15 mL 管;40 µL 10 mg/mL DNase 5 min RT、vortex、200×g 5 min;1 mL 洗×2、2 mL 重悬;计数调 2.5×10⁶/mL (细胞大小逐系调);60 mm 皿底 5 mL PBS 水合腔、盖内面 20 µL 枪打 10 µL 滴 (≥20 滴/皿,间隔防连)、倒扣水合腔、37 °C/5% CO₂/95% 湿度、每日镜检至成片/聚集体 (stereo 镜);成片后转圆底玻璃摇瓶 3 mL 完全培养基、 shaking 水浴 37 °C/5% CO₂ 至成球。Notes:膜染料预染、1:1 (或他比) 双群共培养 (癌/间质、胚层、粘附工程系)、0.05% trypsin/2 mM calcium 保 cadherin 消化。结果:多系 24 h (MLL 18 h 例 + MEK 抑制收紧例),胚肝 18 h + 摇瓶 48 h 成球;双群 sorting-out (sphere-within-sphere) 仅记存在,机制解释 out-of-scope。
绑定摘要(P2b):20 mL 0.5× PBS 浸 humidifier pad ~5 min 置底盘;40 µL/滴、250–500/滴 (长期生长) / 2,500+/滴 (非增殖/大球,逐系试浓度)、吹打均质化;紧压 SureDrop 密封;盖 + 37 °C CO₂、~4 天收紧;长期培养/检测必须转移至 Akura 96:预湿 40 µL/孔 (上下吹打后临种前吸除);3 针定位、<10 µL/s 经 inlet 加 70 µL 落球;250 RCF 2 min 沉降/除泡;吸除重加 70 µL 定容;换液 tip 置 ledge、<30 µL/s 吸 (留 5–7 µL) + <50 µL/s 加 70 µL。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 90% 汇合 2× PBS 洗;2 mL trypsin-EDTA 37 °C 脱壁;2 mL 完全培养基终止 + 吹散至 15 mL 管 (P2a) | method-local(解离,审计 `#24`) + `op.liquid_transfer`(R1) | 贴壁培养 → 悬液 | 方法绑定:90%、2 mL/2 mL、0.05%/1 mM | P2a §1.1 |
| 40 µL 10 mg/mL DNase 5 min RT;vortex;200×g 5 min;1 mL 洗×2;2 mL 重悬 (P2a) | `op.centrifugation`(R5) + `op.liquid_transfer`(R1) | 悬液 → 已洗沉淀 → 重悬液 | 方法绑定:40 µL、5 min、200×g、1 mL | P2a §1.2–1.3 |
| 计数调 2.5×10⁶/mL (大小逐系调) (P2a) / 计数配 250–500 或 2,500+/40 µL + 均质化吹打 (P2b) | 细胞浓度计数(R6,计数 only) + `op.liquid_transfer`(配制) | 重悬液 → 接种悬液 | 方法绑定:2.5×10⁶/mL;40 µL、两档细胞数 | P2a §1.4;P2b Seeding |
| 60 mm 皿底 5 mL PBS 水合腔;盖内面 10 µL 滴×≥20 (P2a) / pad + 20 mL 0.5× PBS ~5 min + 40 µL SureDrop 密封 (P2b) | method-local(悬滴布放,独立候选,见 §8) | 接种悬液 + 皿/板 → 悬滴阵列 | 方法绑定:5 mL、10 µL、≥20/皿;40 µL、20 mL、~5 min | P2a §2.1–2.2;P2b Prep/Seeding |
| 倒扣孵育 37 °C/5% CO₂/95% 湿度,每日镜检至成片/聚集 (P2a) / 盖 + 37 °C CO₂ ~4 天收紧 (P2b) | `op.incubation`(R3) + 方法门控(形成判定,非操作) | 悬滴 → 聚集体/成片 | 方法绑定;完成=人工镜检,非 elapsed | P2a §2.3;P2b Step 4–5 |
| 成片后转圆底玻璃摇瓶 3 mL 摇床水浴至成球 (P2a) / 3 针定位 + <10 µL/s 加 70 µL 落球 + 250 RCF 2 min + 重加 70 µL 定容 (P2b) | method-local(悬滴→悬浮转移,独立候选,见 §8) | 悬滴聚集体 → 摇瓶/接收板球体 | 方法绑定:3 mL;70 µL、<10 µL/s、250 RCF 2 min | P2a §2.4;P2b Transfer 1–4 |
| Akura 96 换液:ledge 位 <30 µL/s 吸留 5–7 µL + <50 µL/s 加 70 µL (P2b) | `op.washing` lossy 变体(R4) | 接收板球体 → 保留球体 + 废液 | 方法绑定:5–7 µL、70 µL、两档流速 | P2b Medium Exchange |
| 膜染料预染/双群 1:1 共培养/钙保粘附消化 (P2a Notes) | 方法分支/前置(非操作) | — → 标记/混合悬液 | 方法绑定 | P2a §3 Notes 2–4 |
| 聚集体尺寸 ImageJ threshold/binary/particle + t-test (P2a Fig 1–2) | ToolAction + 分析 (out-of-scope);**不是操作** | 图像 → 派生尺寸/检验 | 方法要求 | P2a Representative Results |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (终止/洗/重悬/配制/转板) | 悬滴(P2);片/ALI (P3/P4) | 复用 R1 | 体积/滴数全方法绑定 |
| 离心 (200×g 洗涤沉淀;250 RCF 落球沉降) | 悬滴(P2);片(P3) | 复用 R5;意图不同但同为分离/沉降,值各自绑定 | 200×g 5 min vs 250 RCF 2 min 不合并值 |
| 孵育 (悬滴保持/摇瓶成球/4 天收紧) | 悬滴(P2);ULA/片/ALI | 复用 R3 elapsed-only;聚集/收紧结果不在内 | 完成=镜检门控 |
| 3D 部分换液 (Akura 96 留 5–7 µL) | 悬滴平台(P2b);ULA/片 | 复用 lossy 变体语境 | 流速/残留方法绑定 |
| 细胞浓度计数 | 悬滴(P2a/P2b);片(P3)/ALI(P4) | 复用 R6 计数 only;活力/形成判定另立 | 工具 (hemacytometer/TC10) 方法绑定 |
| 悬滴布放 (lid 10 µL vs SureDrop 40 µL) | 仅悬滴家族 | method-local;不晋升 (§8) | 同一家族两变体≠两独立家族;不与 ULA 接种合并 |
| 悬滴→悬浮/接收板转移 (摇瓶 vs 落球) | 仅悬滴家族 | method-local;不晋升 (§8) | 机理与 ULA 宽口转移不同 |
| 尺寸定量/检验 | 悬滴结果语境 | ToolAction + 分析 out-of-scope | 阈值/检验归下游 |

---

## 4. P3 — Spheroid sheet (Le Bio-protocol 5501;微孔板成球 + 网格夹层)

绑定摘要:扩增 P2 购入 → P4 冻存 → P4 复苏至 P6/P7 分化;复苏后 5,000–6,000/m²、传代后 3,000–4,000/cm²,每 2–3 天 50% 换液、90% 传代;倍增 (t·ln2)/ln(Nt/N₀),年轻供体 <50 h,异常 >70 h 可去抗生素 (经验)。成球:细胞数/球 × 微孔数/孔 (例 2,000 × 2,885 = 8.88 M/孔,Elplasia 6 孔);day 1 径 ~200 µm (1,000/球)–~350 µm (5,000/球),Elplasia 上限 250 µm,网格孔 150×200 µm 故球 >150 µm;1 mL 培养基/孔 + parafilm 包 + 300×g 1 min 除泡,37 °C 待用;TrypLE 5 mL/T75 37 °C 3–5 min、轻敲、5–10 mL 扩增培养基中和 + 5 mL 二洗、300×g 3–5 min;即刻重悬软骨培养基、计数配 1 mL (例 8.88 M/mL)、BSC 内逐滴加、parafilm 包 + 100×g 1 min (慢加减速 1)、沿壁补 3 mL;37 °C 过夜–3 天收获,禁久置 (换液冲脱致随机融合)。装配:框 70% 乙醇 ~1 h + 2× DPBS 洗存 DPBS、网/滤纸/钢具高压灭菌 (滤纸建议 60 °C 烘干过夜);1 mL 枪同液冲洗收球至 50 mL 管、10–20 s 沉降重用上清、倾板先冲上区 2–3 遍;分装 (例 300 µm 球 ~500/cm² = packed monolayer ~300 µm 厚,沉降快须混匀后分);滤纸 + 单网 + 500 µL 测试通透 (+500 µL 复测),可选 Whatman mask;单次吸尽沉底球、mask 内匀铺、刮刀调、去 mask 盖第二网;钳 transfer 上框 (摩擦/卡扣/螺丝)、入 6 孔/10 cm 皿没顶、软骨培养基 14–21 天每 2–3 天换液;2–3 天融合、≥10 天可剥、14 天内易剥/14–21 天小心可剥;新球 (<24 h) 粘头→预湿枪头 (~10× 吸打) 防损。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| P4 冻存→P6/P7 扩增;5,000–6,000/cm² (复苏) / 3,000–4,000/cm² (传代);2–3 天 50% 换液;90% 传代;倍增公式 | Composite(传代,见 B01) + `op.liquid_transfer`(R1,换液) + ToolAction(倍增换算) | 冻存管 → 可成球 hMSC | 方法绑定 | P3 A.1–3 + Note |
| 定量:细胞数/球 × 微孔数/孔 (例 8.88 M/孔);径窗/孔板上限/网孔下限 | ToolAction(乘法) + 方法约束(选型门控,非操作) | 目标球径 → 接种数 | 方法绑定:2,000/球、2,885/孔 | P3 B.1 + Notes 1–2 |
| 1 mL/孔 + parafilm + 300×g 1 min 除泡;37 °C 待用 | method-local(微孔除泡前置) | 干微孔 → 可接种孔 | 方法绑定:1 mL、300×g 1 min | P3 B.2 |
| TrypLE 5 mL/T75 3–5 min;5–10 mL 中和 + 5 mL 二洗;300×g 3–5 min;即刻重悬软骨培养基计数配 1 mL | method-local(解离 `#24`) + `op.centrifugation`(R5) + `op.liquid_transfer`(R1) + 细胞浓度计数(R6) | 贴壁 hMSC → 接种悬液 | 方法绑定:5 mL、3–5 min、300×g | P3 B.3–B.4 |
| 1 mL 逐滴匀铺 + parafilm + 100×g 1 min (慢速 1);沿壁补 3 mL;37 °C 过夜–3 天 (禁久置) | `op.liquid_transfer`(R1,接种) + `op.centrifugation`(R5,落孔) + `op.incubation`(R3) + 方法约束(禁久置) | 接种悬液 → 微孔内球体 | 方法绑定:1 mL、100×g 1 min、3 mL | P3 B.5–B.8 |
| 1 mL 枪同液冲洗收球至 50 mL 管;10–20 s 沉降重用上清;倾板 2–3 遍 (预湿枪头 ~10× 防粘) | method-local(球体收获,独立候选,见 §8) | 微孔内球体 → 管内球体悬液 | 方法绑定;手法为父操作备注 | P3 C.2 + Troubleshooting 2 |
| 分装至各管 (例 ~500/cm²);混匀后分 (沉降快) | `op.liquid_transfer`(R1,分装) + `sampling`(R2,等分谱系) | 管内悬液 → 网上样等分 | 方法绑定:~500/cm²、~300 µm 厚 | P3 C.3 + Note |
| 滤纸 + 单网 + 500 µL (+500 µL) 通透测试;可选 mask | 方法前置(装配准备,非操作) | 网/框 → 可接种网 | 方法绑定:500 µL | P3 C.4 |
| 单次吸尽匀铺 mask 内;刮刀调;去 mask 盖第二网;上框 (摩擦/卡扣/螺丝) 没顶培养 | method-local(网格夹层装配,独立候选,见 §8) | 球体等分 + 双网框 → 夹层片 | 方法绑定;框尺寸/卡扣为装置备注 | P3 C.5–C.6 + Supplies 4/Fig 1 |
| 14–21 天软骨分化,每 2–3 天换液;2–3 天融合/≥10 天可剥/14 天内易剥 | `op.incubation`(R3,保持) + `op.liquid_transfer`(R1,换液) + 方法门控(融合/可剥判定) | 夹层片 → 融合片膜 | 方法绑定:14–21 天、2–3 天节律 | P3 C.6c + General notes 1–2 |
| H&E/Alcian blue/collagen II 横切 (Fig 2f–h) | 方法终点/分析 out-of-scope,仅记存在 | 片膜 → 切片数据集 | 方法要求 | P3 Fig 2 + Validation |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (换液/中和/接种/补液/分装/换液) | 片(P3);悬滴/ALI (P2/P4) | 复用 R1 | 体积/节律全方法绑定 |
| 离心 (300×g 收 cell/除泡语境;100×g 落孔) | 片(P3);悬滴(P2) | 复用 R5;除泡≠分离但同为受控离心,值各自绑定 | 300×g 3–5 min vs 1 min vs 100×g 1 min 不合并值 |
| 孵育 (过夜–3 天成球;14–21 天分化) | 片(P3);悬滴/ULA/ALI | 复用 R3 elapsed-only;融合/分化/可剥不在内 | 完成=人工判定 |
| 取样/分样 (管内等分上网) | 片(P3);悬滴/ALI/微生物既有 | 复用 R2;沉降快混匀为备注 | 数量守恒 UNKNOWN (沿用缺口) |
| 细胞浓度计数 | 片(P3);悬滴/ALI | 复用 R6 计数 only | >90% 活力门控为方法前置 |
| 微孔除泡/球体收获/网格装配 | 仅片家族 | method-local;不晋升 (§8) | 单家族;网孔/框型分支大 |
| 倍增/接种量/厚度换算 | 片内 | ToolAction;**永不建模块** | 确定性派生 |

---

## 5. P4 — ALI 分化 (Propagenix;EpiX submerged → PneumaCult-ALI airlift)

绑定摘要:80–90% 收获 (EpiX 扩增规程另行,本批不拆);Modified EpiX = EpiX + 75 µL 2 M CaCl₂/100 mL (终 1.5 mM);ALI 期 PneumaCult-ALI 按供应商配;24 孔 Transwell 顶 200–250 µL / 底 800 µL–1 mL;300 µL PBS 预湿顶腔 5–10 min 后不触膜吸除;500,000/mL × 250 µL = 125,000/孔 (~400,000/cm²) 加顶腔 + 底腔 Modified EpiX 800 µL–1 mL;37 °C/5% CO₂ submerged 至完全单层,双腔隔天换液;完全单层后去双腔液 + 底腔 PneumaCult-ALI 800 µL–1 mL 即 day 0 airlift,顶腔不再加液;底腔每 2–3 天换液;每周 300 µL PBS 轻柔漂洗顶腔去 mucin;可选 EVOM2/ENDOHM-6 + 空白 insert 对照;21 天 (3–4 周) 分化为多纤毛/产粘液假复层,石蜡 5 µm 切片 H&E/acetylated-tubulin/alcian blue 仅记存在,不拆判读。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 配 Modified EpiX (75 µL 2 M CaCl₂/100 mL → 1.5 mM);按供应商配 PneumaCult-ALI | 方法试剂配制(非操作,本批不立候选) | 基础培养基 → 双相培养基 | 方法绑定:75 µL、1.5 mM | P4 Reagents + Steps 1–2 |
| 300 µL PBS 预湿顶腔 5–10 min,不触膜吸除 | 方法前置(膜准备,非操作) | 干膜 → 可接种膜 | 方法绑定:300 µL、5–10 min | P4 Step 3 |
| 收获计数;500,000/mL × 250 µL (125,000/孔) 加顶腔 + 底腔 800 µL–1 mL Modified EpiX | 细胞浓度计数(R6) + `op.liquid_transfer`(R1,双腔加样) | 悬液 + Transwell → submerged 双腔培养 | 方法绑定:125,000/孔、双腔体积 | P4 Steps 4–5 |
| Submerged 至完全单层;双腔隔天换 Modified EpiX | `op.incubation`(R3) + `op.liquid_transfer`(R1,双腔换液) + 方法门控(单层判定) | 接种膜 → 完全单层 | 方法绑定:隔天节律;完成=人工镜检 | P4 Step 6 |
| 去双腔液 + 底腔 PneumaCult-ALI 800 µL–1 mL 即 day 0;顶腔不再加液 | method-local(airlift 建立,独立候选,见 §8) | 完全单层 → ALI day 0 | 方法绑定:800 µL–1 mL | P4 Step 7 |
| 底腔每 2–3 天换 PneumaCult-ALI;每周 300 µL PBS 顶腔漂洗 mucin | `op.liquid_transfer`(R1,底腔换液/顶腔漂洗,lossy 语境) | ALI 培养 → 维持中 ALI | 方法绑定:2–3 天、300 µL/周 | P4 Step 8 |
| 可选 TEER (EVOM2/ENDOHM-6 + 空白对照) | 方法终点 readout (非操作,本批仅记存在;定量解释 out-of-scope) | ALI 培养 → 电阻数据集 | 方法绑定 | P4 Step 9 |
| 21 天 (3–4 周) 分化终点;切片染色 trio | 方法终点/分析 out-of-scope,仅记存在 | ALI 培养 → 终点数据集 | 方法要求 | P4 Examples |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (双腔加样/换液/漂洗/去液) | ALI(P4);悬滴/片(P2/P3) | 复用 R1;顶腔漂洗为 lossy 语境 | 双腔体积/节律全方法绑定 |
| 孵育 (submerged 生长/ALI 分化保持) | ALI(P4);全部 3D + 既有 | 复用 R3 elapsed-only;单层/分化不在内 | 完成=单层/分化门控 |
| 细胞浓度计数 (收获计数) | ALI(P4);悬滴/片 | 复用 R6 计数 only | 80–90% 收获门控为方法前置 |
| Airlift (去顶液极化) | 仅 ALI 家族 | method-local;不晋升 (§8) | 单家族;与部分换液 intent 不同 |
| TEER/切片终点 | ALI 方法终点 | 非操作/分析 out-of-scope | 仪器型号仅记用过,不作 profile |

---

## 6. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 接种/稀释/双腔加样/换液/中和/补液/分装/落球定容/底腔换液 (P1–P4) | `op.liquid_transfer` | 参数化复用;稀释/分装整体=Composite,不建"稀释"原子 |
| 收获计数/接种计数/悬液计数 (P2a/P2b/P3/P4) | 细胞浓度计数(R6) | 复用 (计数 only);活力/单层/形成/分化判定归方法门控 |
| 悬滴/成球/分化/筛选保持 (P1 ~24 h / P2a 18 h–48 h+摇瓶 / P2b ~4 天 / P3 过夜–3 天 + 14–21 天 / P4 submerged + ALI 3–4 周) | `op.incubation` | 复用 elapsed-only;聚集/融合/分化/可剥结果一律不在内 |
| ULA 沿壁部分换液 (留 10–20/~5 µL,摘要)/Akura 96 (留 5–7 µL)/顶腔漂洗/片分化期换液 | `op.washing` lossy 变体(R4/`#32`) | 复用变体语境;残留/吸位/流速为方法绑定,不进通用默认 |
| 洗涤沉淀 200×g / 落球沉降 250 RCF / 收 cell 300×g / 除泡 300×g / 落孔 100×g | `op.centrifugation` | 参数化复用;RCF/时间/转子各自绑定,不合并值 |
| 管内等分上网/悬液配制谱系 | `sampling`(R2) | 参数化复用;数量守恒 UNKNOWN (沿用缺口) |
| 同板荧光/明场观察 | `FLUO_IMAGE`(R7 raw 采集 only) | 按需复用 raw 语境;通道/焦面/定量归下游 |
| 倍增公式/接种量乘法/厚度估算/125,000 换算/Poisson (P2b 0.5/孔类推禁用,本批未用)/TEER/切片判读/尺寸 t-test | ToolAction + 方法终点 | **永不建模块** |
| ULA 球体形成/悬滴布放/悬滴转移/微孔除泡/球体收获/网格装配/airlift 建立 (P1–P4 各自) | 无 → `method_local` | 方法模板自有步骤;不晋升 (§8) |
| 发光活力 (CTG3D 1:1/5 min/25–60 min/0.25–1 s) | 缺口 `G-1` (domain-shared candidate),方法模板内 Custom 占位 | 不复用 `ABS_READ`;本批不立读数模块 |

---

## 7. 跨方法公共操作覆盖矩阵

● 直接使用;△ 条件/分支使用;○ 刻意不用 (防合并);— 本方法无此步。

| 操作 | ULA P1 (摘要) | 悬滴 P2a/b | 片 P3 | ALI P4 | 既有家族证据 | 结论 |
|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 接种/换液/转移 | ● 终止/洗/配制/落球/定容 | ● 中和/接种/补液/分装/换液 | ● 双腔加样/换液/漂洗 | B01–B10 全线 | shared,最高复用 |
| 细胞浓度计数 | ● (摘要语境) | ● 2.5×10⁶/mL;两档/40 µL | ● 配 8.88 M/mL | ● 125,000/孔 | B01/R6 | shared readout (计数 only) |
| `op.incubation` | ● 保持 (elapsed) | ● 悬滴/摇瓶/4 天 | ● 过夜–3 天 + 14–21 天 | ● submerged + ALI | B01–B10 | shared elapsed-only |
| `op.washing` lossy | ● 留残液换液 | ● Akura 96 留 5–7 µL | ● 分化期换液 | ● 底腔换液/顶腔漂洗 | 生物膜/3D 既有 | shared variant |
| `op.centrifugation` | ○ (摘要值禁引) | ● 200×g;250 RCF | ● 300×g;100×g | — | 传代/Transwell/转化 | shared,值不合并 |
| `sampling` 谱系 | — | — | ● 管内等分上网 | — | qPCR/培养/微生物 | 条件复用 |
| `FLUO_IMAGE` raw | ○ 观察语境 | ○ 镜检门控 | ○ 融合观察 | — | LIVE/DEAD/IF/划痕 | 按需复用 raw |
| 发光读数 `G-1` | ○ 缺口占位 | — | — | — | ATP/reporter 域共享候选 | domain-shared,不建模 |
| ULA 形成 method-local | ● | ○ (≠ULA) | ○ (微孔≠ULA) | ○ (≠聚集) | 无 | 不合并 (显式拒绝) |
| 悬滴布放/转移 | ○ | ● lid/Akura 双变体 | ○ (≠悬滴) | ○ | 无 | 不合并 |
| 球体收获/网格装配 | — | ○ | ● | — | 无 | 不合并 |
| Airlift 建立 | — | — | — | ● 去顶液极化 | 无 | 不合并 |

防合并线(显式拒绝):ULA 防粘聚集 ≠ 悬滴重力聚集 (保留机制/容器/确认不同);悬滴 lid 10 µL ≠ Akura SureDrop 40 µL 落球转移 (同一家族两变体,不算跨家族复用);微孔成球 + 网格融合 ≠ ULA 单孔成球 (收获/装配链不同);ALI 去顶液极化 ≠ 部分换液 (分化 intent vs 液体交换,顶腔"不再加液"不是残留体积);摇瓶成球 ≠ 振荡培养 (聚集结果 vs 通气保持);发光 ≠ 吸光度 (审计 §4.4);离心沉降 ≠ 孵育收紧 (elapsed 不断言聚集)。

---

## 8. 缺口清单(不能表达 / UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 影响步骤 | 现状 |
|---|---|---|---|
| G1 | P1 全文未读:CLS-AN-235 PDF 403、Sigma 页超时;密度/体积/残留/添加剂/离心值全部 UNKNOWN | P1 全表 | `source_missing`;摘要任何数值禁引作证据;复核须取 Corning PDF 全文 |
| G2 | ULA 最优密度/添加剂/终点逐系依赖 (MCF7 类松散例);紧密度定量判据无 | P1 形成判定 | `method_dependent`;本文件不定义图像算法 (范围排除) |
| G3 | 悬滴浓度窗:2.5×10⁶/mL (P2a) vs 250–500/40 µL vs 2,500+/40 µL (P2b) 互不可比;细胞大小/增殖状态分支未解 | P2 接种 | `method_dependent`;禁跨平台复制值 |
| G4 | 悬滴蒸发/融合/掉滴:水合腔 (5 mL vs 20 mL 0.5× PBS pad)、滴间距、95% 湿度、 lid vs SureDrop 密封差异 | P2 保持 | `configuration_dependent`;掉滴损失未量化 (lossy 证据不足,仅记手法) |
| G5 | 球体收获损失:冲洗力度/枪头粘附 (<24 h 新球)、沉降重用上清的携带、2–3 遍完整性 | P3 C.2 | UNKNOWN;预湿枪头为备注,损失量未声明 |
| G6 | 网格装配分支:网孔 (150×200 µm) vs 球径 (>150 µm)、框型/卡扣/螺丝、spacer 厚度、outgrowth 致难剥 (14–21 天) | P3 C.4–C.6 | `method_dependent` + `configuration_dependent`;通用 "sheet" 模块不可立 |
| G7 | ALI 分化逐系/供体依赖:submerged 天数 (完全单层判据)、21 vs 28 天、mucin 漂洗频率、TEER 阈值 | P4 全表 | `method_dependent`;TEER/切片判读 out-of-scope |
| G8 | 发光读数原子缺失:CTG3D 1:1/5 min/25–60 min/0.25–1 s 仅摘要,量程/积分/仪器能力 `configuration_dependent` + `source_missing` | 终点读数 | 缺口 `G-1`;方法模板内 Custom 占位,不立模块 |
| G9 | 解离/消化终点与损失:TrypLE/Trypsin 力度/时间、吹打分散终点 (变圆目视)、洗涤造成损失未量化 | P2a/P3 | `source_missing`;审计 `#24` 同理,不并入洗涤 |
| G10 | 时间锚点:悬滴 `t₀`=布滴时刻、摇瓶 `t₀`=转移时刻、片 `t₀`=100×g 落孔、ALI `t₀`=去顶液 day 0;储存/预湿为独立保持窗 | 全部 | 锚点缺口;调后密闭/沉降/晾置为防漂移约束,非孵育断言 |
| G11 | 范围外 (误建模块风险) | — | 磁悬浮/生物打印/organ-on-chip/类器官定向分化/共培养分选、长时实时活力、组学终点、动物/临床一律不建模 (`not_applicable`) |

---

## 9. 晋升审查结论:零新模块

四问(独立目的/稳定边界/独立确认/有意义复用) + 晋升四门(G1 复用广度 ≥2 独立方法、域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| 通用 "3D 培养" 操作 | ✗ (意图混杂) | ✗ | ✗ | ✗ | ✗ | **禁止新建** (题设要求);四机制分立记录 |
| ULA 球体形成 | ✓✓✓△ | ✗ (仅 ULA 一类) | ✓ | ✗ | ✓ | method-local/composite;不晋升 (冻结 §5.3/审计 `#31` 同理) |
| 悬滴布放 (lid / SureDrop) | ✓✓✓✓ | ✗ (仅悬滴家族;两变体≠两家族) | △ (10 µL 开放滴 vs 40 µL 密封滴 effect 分支) | ✗ | ✗ | method-local;不晋升;不与 ULA 接种合并 |
| 悬滴→悬浮/接收板转移 (摇瓶 / 70 µL 落球) | ✓✓✓✓ | ✗ | ✗ (摇瓶振荡 vs 落球沉降) | ✗ | ✗ | method-local;不晋升 |
| 微孔除泡/球体收获/网格装配 | ✓✓✓✓ | ✗ (仅片家族) | ✓ (各自内) | ✗ | ✗ | method-local/composite;不晋升 |
| Airlift 建立 (去顶液极化) | ✓✓✓✓ | ✗ (仅 ALI) | ✓ | ✗ | ✓ | method-local;不晋升;不与部分换液合并 |
| 3D 部分换液新原子 | ✓✓✓△ | ✓ | ✓ | ✗ (`op.washing` lossy 已覆盖) | ✓ | 不新增,直接复用 lossy 变体 (审计 `#32`) |
| 发光活力读数 | ✓✓✓✓ | ✓ (ATP/reporter 域内) | ~ (板读 vs 成像须切分) | ✓ | ✓ (切分后) | domain-shared candidate (`G-1`);本批不停留,直接复用其边界,Custom 占位 |

因此本批全部候选留在 method-local / 既有操作参数化复用 / 方法门控 / ToolAction,不进入公共 Catalog,不写 PortSpec/Transition 全文,不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer`( + 悬滴布滴/落球/双腔/分装家族)、细胞浓度计数( + 2.5×10⁶/mL 与 40 µL 两档语境)、`op.incubation`( + 悬滴聚集/片融合/ALI 分化三类保持绑定)、`op.centrifugation`( + 200×g/250 RCF/300×g/100×g 四档语境)、`op.washing` lossy( + Akura 5–7 µL/ALI 顶漂/片分化换液语境)。

---

## 10. 核查与状态

- 已验证(直接读,2026-11-05,§1 定位节):P2a JoVE 全文 (悬液/悬滴/Notes/结果);P2b Akura Quick Start 全文 (准备/接种/转移/换液);P3 Bio-protocol 5501 全文 (A/B/C + Recipes + Fig 2 + Notes/Troubleshooting);P4 Propagenix ALI 全文 (试剂 + 步骤 1–9 + Examples)。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:P1 CLS-AN-235/446 全文数值、Sigma 页体积表、Perfecta3D/GravityPLUS 数值、PneumaCult PIS 全文断点、CTG3D TM412 全文量程、任何仪器手册/批间差/已跑数据派生;检索摘要一律未转写为证据。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离 (型号仅记用过,不作 profile);样本跃迁有端口 (§§2–5 表 1);时间锚显式 (§8 G10);部分列表标部分 (P2b Product Manual 仅佐证);plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
