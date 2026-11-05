# Batch 12 — Live-Cell Time-Lapse Imaging and Cell Death / Cycle Assays: Annexin V, TUNEL, EdU, PI Cell-Cycle Flow

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 活细胞/time-lapse 原始采集与环境保持;Annexin V 流式凋亡;Click-iT TUNEL 原位成像;Click-iT EdU 流式 + 成像增殖;乙醇固定 PI 细胞周期流式。只记湿实验步骤与原始读数;设门/补偿/群体统计/图像分割定量/曲线拟合一律 out-of-scope,不建操作。
- 规则: 现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;每条数值/行为证据只引本文件 §1 的原始 protocol;成像原始采集与流式原始采集分开,染色/固定/通透/采集/分析分开;不建通用 stain/fix/perm/block 模块;不新增公共模块除非四问 + 晋升四门全过,domain-shared 允许但本批只增复用证据;不做代码/UI/DB 变更。
- 禁止声明: 本文件不含任何通用 Annexin V 用量、通用 TUNEL 时间、通用 EdU 浓度、通用乙醇固定时长、通用 time-lapse 间隔/曝光;所有温度/时间/体积/浓度/通道均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| P1 | Thermo Fisher BestProtocols: Annexin V Staining Protocol for Flow Cytometry (A 基本 / B FVD / C 表面+胞内) | 厂商手册 | 当前页;General notes + A 步骤 1–10 + B 步骤 1–13 + C 步骤 1–14 + 材料表 | 直接读:页面全文转写;A 主绑定,B/C 只记分支与路由,不展开 C 的表面/胞内引用的完整滴定 |
| P2 | Click-iT TUNEL Alexa Fluor Imaging Assay User Guide, MAN0002004 Rev. A.0 (C10245/46/47) | 厂商手册 (试剂盒说明书) | Coverslip 步骤 1–7 + 96 孔 步骤 1–7 + 表 2–8 + 兼容表 1 | 直接读:PDF 全文转写;HeLa + 0.5 µM staurosporine 4 h 为开发语境,不作通用诱导条件 |
| P3 | Thermo Fisher Click-iT EdU Protocol for Flow Cytometry (C10418/24/25) + Click-iT EdU Imaging Protocol (C10337/38/39/40) | 厂商手册 | Flow:试剂配制 1–5 + 标记 1–4 + 表面染 1–6 + 固定通透 1–5 + Click 反应表 1 + DNA 染表 2 + 采集;Imaging:配液 1–6 + 标记 1–6 + 固定通透 1–6 + Click 表 + 附加标记 + 成像滤光表 | 直接读:两页全文转写;Jurkat + 10 µM 1–2 h 为开发起点,不作通用浓度 |
| P4 | UNC Flow Cytometry Facility: Propidium Iodide Staining for DNA Content in Ethanol Fixed Cells | 大学 facility SOP | 步骤 1–10 + 注记 (浓度/内标/低压/双体) 全文 | 直接读:PDF 全文转写;胰酶/胶原酶解离为前置语境,不拆解 |
| P5 | Burke & Orth 2016, Through the Looking Glass (JoVE 53994, PMC4942147) §§1–8 + Thermo BioProbes 70: Image Live Cells Through Time and Space | 同行评议 protocol + 厂商成像指南 (环境/光毒部分) | JoVE 步骤 1–9 全文 (HT1080 FUCCI 例) + BioProbes 光/环境/EVOS Onstage + phototoxicity tips 全文 | 直接读:两文全文转写;纵向追踪/定量 (§9, ImageJ ROI/FUCCI/BP1-2) 只记存在,不拆解 (out-of-scope) |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 取样/分样 sampling atomic(谱系,守恒缺口);R3 `op.incubation`(elapsed/条件 only,不断言结合/通透/生长);R4 保留语义洗涤 `op.washing` + lossy 变体;R5 `op.centrifugation`;R6 `FLUO_IMAGE` 原始荧光图像采集 (固定/活分终点注记);R7 流式原始采集 domain-shared candidate (原始 FCS only,分选/设门/补偿另立);R8 细胞浓度计数 readout;审计 `#23` 通用染色、`#24` 解离、`#26` 固定/通透/封闭、`#33` 发光读数、`#34` ToolAction 与本批方法内染色/固定/分析的对应关系见 §§7–10。

检索摘要(`search-summary-only`,非证据):Abcam Annexin V 页、FxCycle PI/RNase 产品页、PubCompare/Facility PI 变体页、JoVE 微生物延时页仅用于立项定位;凡数值以 §1 直接读为准。

---

## 2. P1 — Annexin V 流式 (Thermo BestProtocols A/B/C)

绑定摘要:钙依赖 Annexin V–PS 结合;禁 EDTA/钙螯合剂;膜完整才是凋亡标记,破膜后胞内 PS 误结合;FVD eFluor 450 禁配 Annexin V 试剂盒;A:1X 结合缓冲 (10X 1:9 水) → 收获 → 1X PBS 洗一次 + 1X 结合缓冲洗一次 → 1–5×10⁶/mL → 5 µL Annexin V / 100 µL 细胞 → 10–15 min RT 避光 → 2 mL 结合缓冲 400–600×g 5 min RT 弃上清 → 200 µL 重悬 → 5 µL PI 或 7-AAD → 5–15 min 冰/RT,染后禁洗 (染料须留存),≤4 h 内分析,2–8 °C 避光保存。B (FVD):无叠氮无蛋白 PBS 洗两次 → 1–10×10⁶/mL → 1 µL FVD / mL 立即涡旋 → 30 min 2–8 °C 避光 → 染色缓冲洗两次 + 结合缓冲洗一次 → 回 A 的 Annexin 段 (1–5×10⁶/mL, 5 µL/100 µL, 10–15 min RT, 2 mL 洗离心, 200 µL 重悬) → 直接上机 (无 PI/7-AAD 步)。C:表面染 (路由 B06 P-FLOW-SURFACE) → B 的 FVD 段 → A 的 Annexin 段 (结合缓冲洗一次) → 胞内染 (路由 B06 P-FLOW-IC A/B)。PerCP-eFluor 710 试剂盒不含活力染料,须配 FVD 660/506/780。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 配 1X 结合缓冲 1:9 | `op.liquid_transfer` 参数化 (缓冲配制语境) | 10X + 水 → 1X 结合缓冲 | 方法绑定:1:9 | P1 A-1 / B-1 / C-1 |
| 收获细胞 | 前置条件 (解离/收获另立,本批不拆) | 培养物 → 悬液 | 方法绑定 | P1 A-2 |
| A:1X PBS 洗一次 + 结合缓冲洗一次;B:无叠氮无蛋白 PBS 洗两次 | `op.washing`(R4 保留语义) + `op.centrifugation`(R5) 参数化 | 悬液 → 待染悬液 | 方法绑定:次数/缓冲种类;离心值见 A-7 档 | P1 A-3 / B-2 |
| 重悬至 1–5×10⁶/mL (A) / 1–10×10⁶/mL (B FVD 前) | `op.liquid_transfer` 参数化 (浓度调整语境) | 洗后细胞 → 目标浓度悬液 | 方法绑定:两档浓度 | P1 A-4 / B-3 |
| 5 µL Annexin V / 100 µL 悬液 | method-local `stain-AnnexinV` + `op.liquid_transfer` | 待染悬液 → 已加 Annexin V | 方法绑定:5 µL/100 µL;缀合物按试剂盒 (FITC/PE/APC 等) | P1 A-5 / B-9 / C-10 |
| 10–15 min RT 避光 | `op.incubation` 参数化 (结合时钟) | 已加 Annexin V → 已孵育 | 方法绑定:10–15 min RT 避光 | P1 A-6 / B-10 / C-11 |
| 2 mL 结合缓冲 + 400–600×g 5 min RT,弃上清 | `op.washing` + `op.centrifugation` 参数化 | 已孵育 → 已洗管 | 方法绑定:2 mL;400–600×g 5 min RT | P1 A-7 / B-11 |
| 200 µL 结合缓冲重悬 | `op.liquid_transfer` 参数化 | 已洗 → 待活力染/待测悬液 | 方法绑定:200 µL | P1 A-8 / B-12 / C 前 |
| A:5 µL PI 或 7-AAD,5–15 min 冰/RT,禁洗,≤4 h 分析,2–8 °C 避光存 | method-local `stain-PI/7-AAD` (染料留存硬约束) + `op.incubation` 参数化 | 待测悬液 → 死染标记悬液 (染料留存) | 方法绑定:5 µL;5–15 min;禁洗;≤4 h 时钟 | P1 A-9–10 + 注记 |
| B:1 µL FVD / mL 立即涡旋,30 min 2–8 °C 避光,染色缓冲洗两次 | method-local `stain-FVD` + `op.incubation`/`op.washing` 参数化 | 悬液 → FVD 标记悬液 | 方法绑定:1 µL/mL;30 min 2–8 °C;FVD 450 禁用 | P1 B-4–6 + 注记 |
| B/C:染色缓冲洗两次 + 结合缓冲洗一次后进 Annexin 段 | `op.washing` 参数化 (缓冲切换) | FVD/表面染后 → Annexin 待染 | 方法绑定:2+1 次 | P1 B-6–7 / C-7–8 |
| C:表面染路由 + 胞内染路由 | 路由 B06 P-FLOW-SURFACE / P-FLOW-IC | — → 已多染悬液 | 方法绑定:克隆/缓冲按胞内靶点 A/B 选 | P1 C-2 / C-13 |
| 上机分析 | R7 流式原始采集 (方法模板内步骤);设门/补偿 out-of-scope | 已染悬液 → 原始 FCS | UNKNOWN:事件数/流速/激光-滤光-增益-阈值;逐机构绑定 | P1 A-10 / B-13 / C-14 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (配缓冲/调浓度/加 Annexin/加染料/重悬) | 多家族 | 复用 R1 | 体积/浓度全方法绑定 |
| 洗涤 (PBS/结合缓冲/染色缓冲) | 多家族 | 复用 R4 保留语义 | 去液力度 UNKNOWN;PI/7-AAD 后禁洗为硬约束 |
| 离心 400–600×g 5 min RT | 传代/流式/Transwell | 复用 R5 | 转子/RPM 映射 UNKNOWN |
| 孵育 (10–15 min / 30 min / ≤4 h 窗) | 多家族 | 复用 R3 elapsed-only | 不断言 PS 结合完成 |
| Annexin V / PI/7-AAD / FVD 染色 | 仅凋亡流式分支 | method-local (三分立,不合并) | 钙依赖/膜完整/固定兼容/禁洗差异;G2 不过 |
| 通用 "viability stain" | — | 不建模块 | PI/7-AAD (禁固定+禁洗) vs FVD (固定兼容) effect 不同 |
| 流式获取 | 流式领域内多方法 | R7 domain-shared 复用证据增量;设门/补偿另立 | 仪器值 UNKNOWN |

---

## 3. P2 — Click-iT TUNEL 原位成像 (MAN0002004 coverslip + 96 孔)

绑定摘要 (coverslip;96 孔括号注差异):去培养基, PBS 洗一次 (脱落风险时跳过直固定) → 4% PFA 覆盖,15 min RT → 去固定液 → 0.25% Triton X-100 PBS 覆盖,20 min RT → 2× 水洗 → 可选 DNase 阳性 (每盖片 89 µL 水 + 10 µL 缓冲 + 1 µL DNase,禁涡旋,100 µL/片 30 min RT,水洗一次) → 100 µL TdT 缓冲 equilibration 10 min RT (孔板 50 µL/孔) → TdT 反应液 (每盖片 94 µL 缓冲 + 2 µL EdUTP + 4 µL TdT;每孔 47/1/2 µL,总量 50 µL/孔) 100 µL/片 (50 µL/孔),60 min 37 °C 湿盒 (可 RT 过夜分支),去液 → 2× 3% BSA PBS 洗 2 min (孔板 1× 5 min) → Click 反应液 (盖片 97.5 µL 缓冲 + 2.5 µL 添加剂;孔板同比,总量 100 µL→按孔数表) 100 µL/片 (50 µL/孔),30 min RT 避光,配后 15 min 内用 → 3% BSA 洗 5 min → 可选抗体染 (BSA 封闭,按厂商) → Hoechst 1:5,000 PBS,100 µL/片 (孔) 15 min RT 避光 → 2× PBS 洗 → 湿封/干封兼容,按 Alexa 488/594/647 + Hoechst 滤光成像。HeLa + 0.5 µM staurosporine 4 h 为开发例;兼容表:Qdot 须 Click 后;荧光蛋白用有机染料/抗 GFP/ riten;phalloidin 禁 Click 同做 (铜催化不兼容,改抗 tubulin);有机染料完全兼容;TC-FlAsH/ReAsH 须 Click 前。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 去培养基 + PBS 洗一次 (脱落风险跳过) | `op.washing` 参数化 + 条件分支 | 贴壁样 → 待固定样 | 方法绑定:跳过分支为防丢失约束 | P2 §1.1 |
| 4% PFA 覆盖,15 min RT | method-local `fix-TUNEL` + `op.liquid_transfer` + `op.incubation` 参数化 | 待固定 → 已固定 | 方法绑定:4% PFA;15 min RT;70% 乙醇为注记备选 | P2 §1.2–1.4 |
| 0.25% Triton PBS 覆盖,20 min RT,2× 水洗 | method-local `perm-TUNEL` + `op.incubation`/`op.washing` 参数化 | 已固定 → 已通透 | 方法绑定:0.25%;20 min;水洗两次 | P2 §1.5–1.6 |
| 可选 DNase 阳性 100 µL/片 (89/10/1),禁涡旋,30 min RT | method-local (阳性对照制备) | 已通透 → 阳性对照样 | 方法绑定:配比;30 min;~18 阳性/kit | P2 §2.1–2.4 + 表 2/6 |
| TdT 缓冲 equilibration 100 µL/片 (50 µL/孔) 10 min RT,去液 | `op.liquid_transfer` + `op.incubation` 参数化 | 已通透 → 已平衡 | 方法绑定:100/50 µL;10 min | P2 §3.1–3.3 |
| TdT 反应液 100 µL/片 (50 µL/孔),60 min 37 °C 湿盒 (RT 过夜分支) | method-local `TdT-incorporation` + `op.incubation` 参数化 | 已平衡 → EdUTP 掺入样 | 方法绑定:94/2/4 (片),47/1/2 (孔);60 min 37 °C;酶用后回 ≤−20 °C | P2 §3.4–3.6 + 表 3/7 |
| 2× 3% BSA PBS 洗 2 min (孔 1× 5 min) | `op.washing` 参数化 | 掺入样 → 待 Click | 方法绑定:3% BSA;次数/分钟 | P2 §3.6 |
| Click 添加剂配制 (625 µL 水 → E) + 反应液 (97.5/2.5),100 µL/片 (50 µL/孔) 30 min RT 避光,15 min 内用 | method-local `click-label` + `op.liquid_transfer` + `op.incubation` 参数化 | 掺入样 → 荧光标记样 | 方法绑定:配比;30 min;15 min 使用窗 | P2 §4.1–4.4 + 表 4/8 |
| 3% BSA 洗 5 min | `op.washing` 参数化 | 已标记 → 待复染/待采 | 方法绑定:5 min | P2 §4.5 |
| 可选抗体染 (BSA 封闭 + 一/二抗按厂商,避光) | 路由方法模板内 (按厂商绑定) | 已标记 → 已复染 | 方法绑定:按抗体 TDS | P2 §5 |
| Hoechst 1:5,000,100 µL 15 min RT 避光,2× PBS 洗 | method-local `stain-DNA` + `op.incubation`/`op.washing` 参数化 | 已标记 → 已核复染 | 方法绑定:1:5,000;15 min | P2 §6 |
| 原始荧光采集 (每视野×通道) | `FLUO_IMAGE`(R6) 参数化;定量 out-of-scope | 已封片 → 原始图像集 | 方法绑定:通道=实际 Alexa/Hoechst;曝光/物镜逐帧记 | P2 §7 + 表 5 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/洗涤 | 多家族 | 复用 R1/R3/R4 | 体积/时间/缓冲全方法绑定 |
| 固定/通透 (PFA/Triton) | 仅 TUNEL/IF 内 | method-local;与 B06 IF 不合并 | 浓度/时间/终点不可逆差异;G2/G4 不过 |
| TdT 掺入 + Click 标记 | 仅 TUNEL 家族 | method-local (两步分立) | 酶促掺入 vs 铜催化标记 effect 不同;不与 EdU Click 合并 (底物/酶/目的不同,见 §9) |
| DNase 阳性对照 | 仅 TUNEL | 方法模板内对照 | ~18 份/kit 为本试剂盒值 |
| 荧光图像原始采集 | LIVE/DEAD/IF/EdU/TUNEL/划痕 | 复用 R6 | 只到原始帧;HCS/定量另立 |

---

## 4. P3 — Click-iT EdU 增殖:流式 + 成像 (同一家族两读出)

绑定摘要:EdU 为胸苷类似物,S 期掺入,铜催化叠氮-炔 Click 检出,免 BrdU 的 DNA 变性。Flow:EdU 储备 10 mM (4 mL DMSO/水 → A),叠氮工作液 (130 µL DMSO → B),1X saponin 洗液 (E 1:10 于 1% BSA PBS),10X 添加剂 (2 mL 水 → G) → 1X (1:10 水,现配);Jurkat 开发起点 10 µM 1–2 h (长时低浓度/短时高浓度,按系滴定,设 EdU 阴性对照);可选表面染 (3 mL BSA 洗,1×10⁷/mL,100 µL/管,PE/串联/Qdot 禁 Click 前加,待 Click 后补);固定 100 µL Component D (4% PFA) 15 min RT 避光;3 mL BSA 洗 (红细胞/血红蛋白重复洗净);100 µL 1X saponin 15 min 或直进 Click;Click 液 (表 1:每反应 438 µL PBS + 10 µL CuSO₄ + 2.5 µL 染料叠氮 + 50 µL 添加剂,0.5 mL/管) 30 min RT 避光,配后 15 min 内用;3 mL saponin 洗 → 100 µL saponin (进胞内/DNA) 或 500 µL saponin (直上机);可选 DNA 含量 (RNase + 染料,兼容矩阵表 2:Pacific Blue×FxCycle Violet 不兼容、Alexa488×PI 不兼容、Alexa647×FxCycle Far Red 不兼容等);采集:DNA 线性、EdU 对数,传统机低流速,Attune <10,000 events/s 同实验同速同浓度。Imaging:EdU 储备 10 mM (2 mL DMSO/水 → A),反应缓冲 (36 mL 水 → D),叠氮 (70 µL DMSO → B),10X 添加剂 (2 mL 水 → F) → 1X (1:10,8 h 内用),Hoechst 1:2,000;盖片过夜贴壁 → 20 µM 标记液 (10 µL 储备/5 mL 培养基,10 片量) 半量换液终 10 µM 2 h (慢生长延长);每孔 1 mL 3.7% 甲醛 15 min RT → 2× 1 mL 3% BSA 洗 → 1 mL 0.5% Triton 20 min RT → 2× BSA 洗 → Click 液 (每孔 430 µL 缓冲 + 20 µL CuSO₄ + 1.2 µL 叠氮 + 50 µL 1X 添加剂,0.5 mL/孔,按表,按序加) 30 min RT 避光摇匀 → 1× BSA 洗 → 可选抗体 → 1 mL Hoechst 1X 30 min RT 避光 → 2× PBS 洗 → 湿封/包埋,按 Hoechst/Alexa 488/555/594/647 滤光 (DAPI/FITC/RFP/TRITC/Cy5) 成像。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 储备/工作液配制 (EdU 10 mM;叠氮;1X saponin;10X→1X 添加剂;Hoechst 稀释) | `op.liquid_transfer` 参数化 (配制语境) | 干粉/浓缩 → 工作液 | 方法绑定:4 mL/130 µL/1:10/2 mL/1:2,000(成像)/1:5,000(TUNEL 另) | P3 Flow 配制 1–5;Imaging 配液 1–6 |
| EdU 标记 (Flow 10 µM 1–2 h;Imaging 半量换液终 10 µM 2 h) | method-local `pulse-EdU` + `op.liquid_transfer` + `op.incubation` 参数化 | 培养细胞 → EdU 脉冲细胞 | 方法绑定:10 µM/1–2 h/2 h;阴性对照 (未加 EdU 同群) | P3 Flow 标记 1–4;Imaging 标记 1–6 |
| 可选表面染 (Flow;PE/串联/Qdot 禁 Click 前) | 路由 B06 P-FLOW-SURFACE + 硬约束 | EdU 脉冲 → 已表面染 | 方法绑定:1×10⁷/mL;100 µL/管;禁染料表 | P3 Flow 表面染 1–6 + 注记 |
| 固定 (Flow 100 µL D 15 min RT;Imaging 1 mL 3.7% 甲醛 15 min RT) | method-local `fix-EdU` + `op.incubation` 参数化 | 脉冲细胞 → 已固定 | 方法绑定:100 µL/1 mL;15 min RT;甲醇/saponin 为注具备选 | P3 Flow 固定 1–4;Imaging 固定 1–4 |
| 通透/洗 (Flow saponin 100 µL 15 min + 3 mL BSA 洗;Imaging 0.5% Triton 1 mL 20 min RT + BSA 洗) | method-local `perm-EdU` + `op.washing` 参数化 | 已固定 → 已通透 | 方法绑定:100 µL/1 mL;15/20 min;红细胞重复洗净 | P3 Flow 固定 4–5;Imaging 通透 5–6 |
| Click 反应 (Flow 0.5 mL/管 438/10/2.5/50;Imaging 0.5 mL/孔 430/20/1.2/50,按序加,30 min RT 避光,15 min 内用) | method-local `click-EdU` + `op.incubation` 参数化 | 已通透 → EdU 荧光标记 | 方法绑定:配比/0.5 mL/30 min;添加顺序硬约束 | P3 Flow Click 1–5 + 表 1;Imaging Click 1–7 + 表 |
| saponin/BSA 洗 (Flow 3 mL;Imaging 1 mL) | `op.washing` 参数化 | 已标记 → 待测/待采 | 方法绑定:体积/缓冲 | P3 Flow Click 5;Imaging Click 7 |
| 可选 DNA 含量 (RNase + 染料,兼容矩阵) | method-local (染料路由) + 方法结果 | 已标记 → DNA 复染 | 方法绑定:表 2 兼容 (3 不兼容格);RNase 按染料 | P3 Flow DNA 表 2 |
| Flow 上机 | R7 流式原始采集;DNA 线性/EdU 对数;设门另立 | 待测悬液 → 原始 FCS | 方法绑定:低流速/<10k/s;同速同浓度;激发/滤光按叠氮 (405/488/633) | P3 Flow 采集 |
| 成像 (Hoechst 1X 1 mL 30 min RT 避光,2× PBS 洗,滤光成像) | method-local `stain-Hoechst` + `FLUO_IMAGE`(R6) | 已标记 → 原始图像集 | 方法绑定:1:2,000;30 min;DAPI/FITC/RFP/TRITC/Cy5 | P3 Imaging 附加标记 + 成像表 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/洗涤/离心 | 多家族 | 复用 R1/R3/R4/R5 | 体积/时间/缓冲全方法绑定 |
| EdU 脉冲标记 | 仅增殖家族 (流/像两读出算同一家族) | method-local | 脉冲时长/浓度系别依赖;BrdU 另立 (变性 antibody vs Click,见 §9) |
| Click 标记 (EdU) | 仅 EdU | method-local;不与 TUNEL Click 合并 | 底物 (掺入 EdU vs TdT-EdUTP) 与前处理不同;G2 不过 |
| 固定/通透 (PFA/saponin/Triton) | 仅 EdU 内两变体 | method-local (变体分立) | 100 µL vs 1 mL;15 vs 20 min;红细胞裂解语境 (Flow) |
| 表面/胞内/DNA 复染路由 | 路由既有模板 | 方法模板内分支 | PE/Qdot 禁 Click 前;DNA 兼容矩阵 3 格不兼容 |
| 原始流式采集 / 原始图像采集 | 领域内复用 | R7 / R6 各复用,互不合并 | 线性 vs 对数;流速/事件率绑定 |

---

## 5. P4 — PI 细胞周期 (乙醇固定,UNC Facility)

绑定摘要:单细胞悬液前置 (贴壁胰酶 ±EDTA,禁刮;实体瘤胶原酶/蛋白酶解离语境) → PBS 洗,1–2×10⁶/5 mL PBS → 70% 乙醇 4 °C (每 5 mL 悬液配 4.5 mL 乙醇管,冰上;−20 °C 预冷数小时) → 染色液现配 (10 mL 0.1% Triton PBS + 2 mg DNase-free RNase + 200 µg PI (1 mg/mL 储备),每天现配;RNase 可购 DNase-free 或 100 °C 5 min 自制;PI 1 mg/mL 4 °C 避光存数月) → 悬液离心弃上清 → 0.5 mL PBS 完全重悬 → 加入乙醇管混匀 → 乙醇 ≥2 h (−20 °C 存数月) → 离心弃净乙醇 (管沿触纸巾,禁 Pasteur 吸) → 5 mL PBS 静置 1–2 min 再离心 (难沉系加 0.5% BSA) → 1 mL 染色液重悬 → 37 °C 15 min 或 RT 30 min (可过夜) → 冰上避光至上机。关键约束:全批同浓度 (结合为浓度律,非量律;染后稀释须用染色液或同 PI 浓度 PBS,禁纯 PBS 稀释);可选鸡/鳟 RBC 内标 (二倍体/非整倍体并存时);swing-bucket 离心,禁微量离心机/Eppendorf 管;上机极低压 + 双体排除 (PI area vs height)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 单细胞悬液制备 (胰酶/酶解离;禁刮) | 前置条件,不拆解 | 培养物/组织 → 单细胞悬液 | 方法绑定:禁刮硬约束 | P4 前言 |
| PBS 洗,1–2×10⁶/5 mL PBS | `op.washing` + `op.sample_aliquot`(R2 谱系) 参数化 | 悬液 → 定量悬液 | 方法绑定:1–2×10⁶/5 mL | P4 前言 |
| 70% 乙醇预冷 (−20 °C 数小时),4.5 mL/管冰上 | `op.liquid_transfer` 参数化 (固定剂准备) | 乙醇 → 预冷乙醇管 | 方法绑定:70%;4.5 mL/管;−20 °C/冰上 | P4 步骤 1 |
| 染色液现配 (10 mL 0.1% Triton PBS + 2 mg RNase + 200 µg PI) | `op.liquid_transfer` 参数化 (染色液配制) | 储备 → 当日染色液 | 方法绑定:配比;每日现配;RNase DNase-free 要求 | P4 步骤 2 |
| 离心弃上清 → 0.5 mL PBS 完全重悬 → 加乙醇管混匀 | `op.centrifugation` + `op.liquid_transfer` 参数化 | 定量悬液 → 乙醇固定悬液 | 方法绑定:0.5 mL PBS;混匀防团聚 (隐含) | P4 步骤 3–5 |
| 乙醇 ≥2 h (−20 °C 存数月) | method-local `fix-ethanol` + `op.incubation` 参数化 | 悬液 → 乙醇固定细胞 | 方法绑定:≥2 h;−20 °C;甲醛禁代 (染料可及性注记) | P4 步骤 6 + 前言 |
| 离心弃净乙醇 (触纸巾,禁 Pasteur) + 5 mL PBS 1–2 min 再离心 (0.5% BSA 可选) | `op.washing` + `op.centrifugation` 参数化 | 固定细胞 → 复水细胞 | 方法绑定:5 mL;1–2 min;0.5% BSA 条件分支 | P4 步骤 7–8 |
| 1 mL 染色液重悬 → 37 °C 15 min 或 RT 30 min (可过夜) → 冰上避光 | method-local `stain-PI/RNase` + `op.incubation` 参数化 | 复水细胞 → PI 标记悬液 | 方法绑定:1 mL;37 °C 15 min / RT 30 min;避光 | P4 步骤 9–10 |
| 同浓度约束 + 稀释规则 + 内标 + 低压 + 双体排除 | 方法约束 + 模板内采集条件;分析 out-of-scope | — → 可比 FCS | 方法绑定:同浓度硬约束;鸡/鳟 RBC 可选;极低压;area vs height | P4 注记 3 段 |
| 上机 | R7 流式原始采集 (DNA 含量);周期拟合 out-of-scope | PI 悬液 → 原始 FCS | 方法绑定:极低压;同浓度;同 PI 浓度稀释 | P4 注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/洗涤/离心/孵育 | 多家族 | 复用 R1/R4/R5/R3 | 体积/离心/时间全方法绑定;swing-bucket 禁微量机为硬约束 |
| 乙醇固定 | 仅周期家族 | method-local;不与 PFA/甲醇合并 | 甲醛致染色质位阻注记;effect 不同 |
| PI/RNase 染色 | 仅周期家族 | method-local;不与活力 PI (P1) 合并 | RNase 必需 + 乙醇通透前提 + 定量化学计量 intent 不同 (见 §9) |
| 同浓度/稀释/内标/双体 | 周期方法 | 方法约束 + ToolAction 语境,非操作 | 浓度律 vs 量律;拟合 (ModFit 类) out-of-scope |

---

## 6. P5 — 活细胞 time-lapse (Burke & Orth JoVE + BioProbes 环境/光毒)

绑定摘要:HT1080 FUCCI 例:MEM-EBSS + 10% FBS + 1% Pen/Strep + 丙酮酸 + NEAA,37 °C 5% CO₂ 80% 湿度培养 → 成像前 2 d 12 孔 #1.5 玻璃底皿 50,000/孔 (~60% 汇合起点,按系调,禁塑料成像,难贴壁系多聚赖氨酸/胶原包被备选) → 环境腔 (~80% 湿度,样品位 37 °C,5% CO₂ 或预混气/CO₂非依赖培养基/FluoroBrite 低背景,储水无菌水,配 dummy 水皿 30 min–1 h 预平衡,石蜡膜耦合防漂) → 倒置宽场/转盘共聚焦 (例 200 W 金属卤 + 20X 0.70 NA,另例 40X 0.75 NA),低位物镜,样品置物镜上 → 快速转运 (保温袋,防 CO₂/温度骤变) → 入腔密封开气 (无密封盖时矿物油/透气石蜡膜/二次加湿,防冷凝) → 热漂平衡 30 min (按皿尺寸经验) → 成像条件:10–20 min/帧 (例 10 min;伤口 2 h/70 h),FITC 50 ms / TexasRed 40 ms / 明场 20 ms 2×2 bin,限通道数,先定参数,荧光禁作自动聚焦 (改明场聚焦,10 µm 范围;信标/硬件连续聚焦备选,玻璃厚度波动可失锁) → 半量换暖药 (37 °C,例 DMSO / 1 µM selinexor / 10 µM PD0332991,可暂停-换液-重启做给药前后纵向,代谢/降解用条件培养基回接验证) → 启动,巡检聚焦/湿度,长时补 37 °C 无菌水 → 结束存盘 (多软件须显式保存) → 生物危害处置。追踪/定量 (§9 ROI/FUCCI/BP1-2/ImageJ) out-of-scope。BioProbes 约束:哺乳细胞光敏 + 环境 (pH/渗透/温/气) 脆弱,离孵育数分钟即漂;碳酸盐缓冲离箱 1 h 内 pH 升,LCIS  ambient 4 h,HEPES + 强光产 H₂O₂ 禁长时依赖;LED 近无 UV;探针不过量 (S/N > 绝对信号,亮探针配稀疏靶点);EVOS Onstage ±0.1 °C + 自动聚焦 (明场/信标/粗细调);建像前布光即光剂量,须最小化。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 培养至 ~60% (50,000/孔,前 2 d,#1.5 玻璃底,计数控密度) | 前置条件 (培养),不拆解 | 细胞系 → 待采样品 | 方法绑定:50,000/孔;~60%;#1.5 玻璃 | P5 JoVE §§3.1–3.2 |
| 环境腔预平衡 (~80% 湿度,样品 37 °C,5% CO₂,储水,dummy 水皿 30 min–1 h) | method-local `chamber-equilibration` + `op.incubation` (保持窗) 参数化 | 空腔 → 已平衡腔 | 方法绑定:80%;37 °C;5%;30 min–1 h | P5 JoVE §§4.1–4.3 |
| 显微镜就位 (倒置宽场,20X 0.70 NA,低位物镜,样品置物镜上) | 仪器能力类别 (不绑定单型号) | 已平衡腔 → 待采光路 | UNKNOWN:具体机型/光源/相机按实验室绑定 | P5 JoVE §§1/5 |
| 快速转运 + 入腔密封开气 (保温,防漂耦合,矿物油/石蜡膜备选) | method-local (转运/密封) | 培养皿 → 腔内样品 | 方法绑定:密封/防蒸发手法 | P5 JoVE §§6.1–6.3 |
| 热漂平衡 30 min | `op.incubation` 参数化 (漂移窗) | 腔内样品 → 热稳定样品 | 方法绑定:30 min (按皿经验) | P5 JoVE §6.4 |
| 成像程序 (x/y/z + 波长,10–20 min/帧;例 50/40/20 ms 2×2 bin;限通道;建像前最小布光) | `FLUO_IMAGE`(R6 live 注记) + 明场/相差模态绑定 + 方法模板时间程序 | 热稳定样品 → 原始 time-lapse 栈 | 方法绑定:间隔/曝光/bin/通道;光预算为硬约束 | P5 JoVE §§7.1–7.2 + BioProbes 光控表/贴士 |
| 自动聚焦 (明场,10 µm;信标/硬件备选;荧光聚焦禁例行) | 方法模板内聚焦策略 (非操作) | — → 聚焦栈 | 方法绑定:10 µm;荧光聚焦 ≈10× 光量警告 | P5 JoVE §7.3 + BioProbes 贴士 |
| 半量换暖药 (37 °C;DMSO/1 µM/10 µM 例) + 暂停-重启分支 | `op.liquid_transfer` 参数化 (给药语境) + 条件分支 | 稳定样品 + 暖药 → 给药样品 | 方法绑定:半量;37 °C;药/浓度为例值 | P5 JoVE §7.4 |
| 运行巡检 (聚焦/湿度,暂停调焦,补 37 °C 水) + 结束存盘处置 | 方法完成检查 + 前置/后置 | 栈采集中 → 已存档栈 + 废弃样品 | 方法绑定:巡检;显式保存要求 | P5 JoVE §§7.6–8 |
| 纵向追踪/定量 (ROI/FUCCI/BP1-2/存活曲线) | out-of-scope (分析);**永不建模块** | 原始栈 → 派生数据 | 方法要求 | P5 JoVE §9 (只记存在) |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (半量换药/暖药) | 多家族 | 复用 R1 | 半量/温度方法绑定 |
| 孵育/保持 (预平衡/热漂/腔保持) | 多家族 | 复用 R3 elapsed-only | 不断言生长/贴附/周期 |
| 原始荧光/明场采集 (逐帧) | LIVE/DEAD/IF/EdU/TUNEL/划痕 | 复用 R6 (live 注记) | 只到原始栈;间隔/曝光/通道方法绑定 |
| 时间程序 (间隔×时长×位点) | 仅延时家族 | 方法模板内程序,非操作 | 光预算 vs 追踪粒度的权衡,无通用值 |
| 环境腔平衡/密封/转运 | 仅活细胞 | method-local | 机型/气路/湿度配置依赖;平台≠单机构建 |
| 追踪/定量/存活曲线 | — | out-of-scope;ToolAction/分析另立 | 与原始采集严格分开 |

---

## 7. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 配缓冲/调浓度/加抗体/加 Annexin/加染料/重悬/换药/配 Click/TdT/染色液 | `op.liquid_transfer` | 参数化复用;绑定体积/试剂/顺序 (Click 按序加,DNase 禁涡旋为备注) |
| Annexin 前分装/周期定量悬液 | `op.sample_aliquot` | 参数化复用;父→管谱系;数量守恒 UNKNOWN |
| 结合/标记/固定/通透/Click/TdT/储存/腔平衡/热漂时钟 + PI ≤4 h 窗 + Click 15 min 使用窗 | `op.incubation` | 复用 elapsed-only;通透/结合/生长完成不断言 |
| PBS/结合缓冲/染色缓冲/BSA/saponin/水洗 (管/孔/片) | `op.washing` (保留语义) | 参数化复用;禁洗 (PI/7-AAD)、禁干 (IF 沿用)、纯 PBS 禁稀释 (P4) 为硬约束;脱落 loss 显式 |
| 400–600×g 档离心;乙醇/复水离心 (swing-bucket) | `op.centrifugation` | 参数化复用;转子/RPM 映射 UNKNOWN;微量机禁令为方法约束 |
| TUNEL/EdU/周期 Hoechst/PI 原始荧光采集 (固定终点) | `FLUO_IMAGE` | 参数化复用;只到原始帧/片;通道/曝光/物镜逐帧记录 |
| 活细胞逐帧采集 (荧光 + 明场/相差) | `FLUO_IMAGE` (live 注记,模态绑定) | 参数化复用;只到原始栈;间隔/曝光/位点为方法模板时间程序 |
| Annexin/EdU/周期流式获取 | R7 流式原始采集 (domain-shared) | 模板内步骤复用证据增量;只到原始 FCS;分选/设门/补偿另立 |
| 表面/胞内/FVD/Annexin/PI-周期/RNase/TdT/Click/Hoechst/乙醇固定/saponin-Triton/腔平衡/密封/聚焦策略 | 无 → `method_local` | 各留方法内;跨 Annexin×周期×TUNEL×EdU×IF 不合并、不晋升 (§10) |
| 阴性/阳性对照 (EdU 未标记群;TUNEL DNase;周期内标 RBC;活力热杀沿用 B06) | 方法模板内对照 | 非操作;缺一判读无效 |
| 浓度换算/兼容矩阵查表/同浓度律/双体排除/周期拟合/设门补偿/追踪定量/存活曲线/统计 | ToolAction 或 out-of-scope | **永不建模块** |

---

## 8. 跨方法公共操作覆盖矩阵

● 直接使用;△ 条件/分支使用;○ 刻意不用 (防合并);— 本方法无此步。

| 操作 | P1 Annexin V | P2 TUNEL | P3 EdU 流/像 | P4 PI 周期 | P5 延时活采 | 既有家族证据 | 结论 |
|---|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 缓冲/抗体/染料 | ● TdT/Click/抗体 | ● 脉冲/固定/Click | ● 乙醇/染色液 | ● 半量换药 | B02/B06/B10 | shared,最多复用 |
| `op.sample_aliquot` | ○ 分装语境 (B/C) | — | ○ 管/孔分装 | ● 定量悬液 | — | B06/B10 | shared conditional |
| `op.incubation` | ● 10–15/30 min/≤4 h | ● 15/20/60 min/过夜分支 | ● 脉冲/15 min/30 min | ● ≥2 h/15–30 min | ● 预平衡/热漂/腔保持 | B06/B10 | shared elapsed-only |
| `op.washing` | ● PBS/结合/染色 (禁洗一次) | ● BSA/水洗 | ● BSA/saponin | ● PBS/复水 | — (贴壁/腔内无洗) | B06/B10 | shared;硬约束保留 |
| `op.centrifugation` | ● 400–600×g 5 min | — (贴壁无离心) | ● 同左档 | ● swing-bucket | — | B06/B10 | shared;映射 UNKNOWN |
| `FLUO_IMAGE` 原始 | — | ● 固定片多通道 | ● (成像分支) | — | ● 活栈 + 明场 | B03/B06 | shared;活/固定为终点注记,不合并语义 |
| 流式原始采集 R7 | ● FCS | — | ● (流分支) | ● DNA-FCS | — | candidate-recheck §5 | domain-shared 证据增量 |
| method-local 染色/固定/通透/Click | ● Annexin/FVD/PI | ● TdT/Click/PFA-Triton | ● EdU脉冲/Click/PFA-saponin-Triton | ● 乙醇/PI-RNase | ● 腔/聚焦/给药 | B06 | 各留方法内 |
| 对照/派生/分析 | ● 阴性/FMO 沿用 | ● DNase 阳性 | ● 阴性/兼容矩阵 | ● 内标/双体/拟合 | ● 追踪/存活 | B06 | 模板/ToolAction/out-of-scope |

防合并线 (显式拒绝):Annexin-PS 结合 ≠ 活力 PI 通透标记 ≠ 周期 PI 化学计量 (钙依赖/膜完整/RNase/乙醇前提不同);TUNEL TdT-EdUTP ≠ EdU 脉冲掺入 (酶促断裂标记 vs S 期掺入,底物/目的不同);TUNEL/EdU 双 Click 反应液不合并 (缓冲/配比/前处理不同);乙醇固定 ≠ PFA/甲醛/甲醇固定 (PI 可及性 vs 形态保持 intent 不同);流式 FCS ≠ 图像栈 (事件级 vs 空间分辨);延时时间程序 ≠ 孵育 elapsed (程序为模板,单步仍 elapsed-only);saponin 通透 ≠ Triton 通透 (红细胞裂解/维持散射 vs 贴壁通透);Hoechst 核复染 (P2/P3) 不立通用 DNA 染模块 (稀释/时间/缓冲各守其法)。

---

## 9. 缺口清单(不能表达 / UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 影响步骤 | 现状 |
|---|---|---|---|
| G-1 | 钙/螯合剂链:结合缓冲钙浓度未量化;EDTA 残留阈值 UNKNOWN | P1 全表 | `method_dependent` + `source_missing`;只记禁令,不立通用值 |
| G-2 | 膜完整前提:破膜/胰酶过度致胞内 PS 暴露的验收标准 UNKNOWN | P1 | 方法前置 (单细胞完整性确认);误判为凋亡的风险注记 |
| G-3 | 缀合物/电压链:Annexin 荧光素选择、FVD 搭配、补偿/FMO、事件数/阈值全 UNKNOWN | P1/P3/P4 采集 | `configuration_dependent`;facility 构建表未读;plan 前须逐 run 绑定 |
| G-4 | TUNEL 分支未解:37 °C 60 min vs RT 过夜等效性;70% 乙醇备选的可比性;phalloidin/荧光蛋白/Qdot 不兼容的逐系验证 | P2 | `method_dependent`;各守其法,不平均 |
| G-5 | EdU 脉冲链:浓度×时长×系倍增曲面;BrdU 变性对照缺失;PE/Qdot 禁 Click 前的完整禁表 | P3 | `method_dependent`;10 µM/Jurkat 只作起点;DNA 兼容 3 格不兼容硬保留 |
| G-6 | 周期化学计量链:RNase 活性/批;PI 储备衰减;同浓度律的浓度窗;BSA 复水系别差异;微量机禁令的转子映射 | P4 | `method_dependent` + `device_state`;稀释规则为硬约束,拟合 out-of-scope |
| G-7 | 光预算链:曝光×间隔×通道×位点的累积剂量;激发波长选择 (长波优先);增益/合并/LED 功率的 S/N 权衡 | P5 | `configuration_dependent`;无通用间隔/曝光值 |
| G-8 | 环境链:腔温/湿度/CO₂ 均匀性;碳酸盐 pH 漂;LCIS/FluoroBrite/HEPES 适用窗 (HEPES + 光产 H₂O₂);矿物油/石蜡膜/冷凝/漂移 | P5 | `configuration_dependent` + `device_state`;平台≠单机构建 |
| G-9 | 终点不可逆/不兼容:固定终结回培养;PI/7-AAD 染后禁固定;FVD 逢固定必选;MeOH/铜催化敏感染料 routing;荧光聚焦 ≈10× 光量 | P1–P5 | 跃迁约束;路由错误=重做 |
| G-10 | 时间锚点:Annexin 加样→上机 ≤4 h;TUNEL TdT+Click 累加时钟 + Click 15 min 使用窗;EdU 脉冲→固定直进;周期乙醇→染色独立窗;延时 t₀=给药/换液,热漂 30 min 不计入生物学时钟 | 全部 | 锚点缺口;孵育不断言生物学完成 |
| G-11 | 范围外 (误建模块风险) | — | 设门/补偿/周期拟合/图像分割/共定位/计数/统计/临床 in vivo 一律 `not_applicable`,本批不记录、不建模 |

---

## 10. 晋升审查结论:零新模块

四问 (独立目的/稳定边界/独立确认/有意义复用) + 晋升四门 (G1 复用广度 ≥2 独立方法、域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| 通用染色 (Annexin/PI/FVD/EdU-Click/TUNEL-Click/Hoechst) | ✓✓✓△ | ✓ (表面多见) | ✗ (钙依赖 vs 通透 vs 化学计量 vs 酶促 effect 不同) | ✗ | ✗ | method-local;不晋升 (沿用审计 `#23`) |
| 通用固定/通透/封闭 (PFA/乙醇/甲醇/Triton/saponin) | ✓✓✓△ | ✓ | ✗ (终点不可逆 + 下游后果不同) | ✗ | ✗ | method-local;不晋升 (沿用审计 `#26`) |
| TdT 掺入 / Click 标记 (TUNEL) | ✓✓✓✓ | ✗ (仅 TUNEL) | ✓ | ✗ | ✓ | method-local;不与 EdU Click 合并 |
| EdU 脉冲 / Click 标记 (EdU) | ✓✓✓✓ | △ (流/像同一家族两读出,不算两独立方法) | ✓ | ✗ | ✓ | method-local;BrdU 另立 |
| 乙醇固定 + PI/RNase | ✓✓✓✓ | ✗ (仅周期) | ✓ | ✗ | ✓ | method-local;不与活力 PI 合并 |
| 活细胞环境保持/延时程序 | ✓✓✓✓ | ✗ (仅延时) | △ (机型/气路依赖) | ✗ | ✗ | method-local + 方法模板时间程序 |
| 原始图像采集 (活/固定) | ✓✓✓✓ | ✓ | ✓ (raw only,终点注记) | ✓ | ✓ | 已有 R6 复用,本批只增证据,不新立 |
| 原始流式采集 | ✓✓✓✓ | ✓ (Annexin/EdU-流/周期三方法,域内成立) | ✓ (raw FCS only) | ✓ | ✓ (分选/设门分离) | 已有 R7 domain-shared candidate,本批只增三方法证据,不新立 |

因此本批全部候选留在 method-local / 方法模板 / ToolAction / out-of-scope,不进入公共 Catalog,不写 PortSpec/Transition 全文,不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer` (+ 结合缓冲/Click/TdT/乙醇/换药语境)、`op.washing` (+ 禁洗/水洗/saponin 语境)、`op.centrifugation` (+ Annexin/周期 swing-bucket 语境)、`op.incubation` (+ 脉冲/TdT/Click/腔平衡语境)、`FLUO_IMAGE` (+ TUNEL/EdU-像/活栈三语境)、R7 流式原始采集 (+ Annexin V / EdU-流 / PI-周期三方法域内证据)。

---

## 11. 核查与状态

- 已验证 (直接读,2026-11-05,§1 定位节):P1 A/B/C 全文;MAN0002004 coverslip + 96 孔 + 表 1–8;P3 Flow/Imaging 两页全文 + 表 1–2 + 滤光表;P4 UNC 步骤 1–10 + 注记;P5 JoVE §§1–9 + BioProbes 光/环境/贴士全文。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何流式/显微镜/环境腔手册、单机构建表、转子映射、per-lot 片、实验室 SOP 改编、滴定终点、已跑数据的派生应用;检索摘要一律未转写为证据。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离 (§§2–6 仅能力类别 + 用过体积作例);样本跃迁有端口 (§§2–6 表 1);时间锚显式;部分列表标部分;plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,原始 FCS/图像保留,派生下游分离);无 JSON;UNKNOWN 永不填零。
