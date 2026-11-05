# Batch-07 RT-qPCR — Wet-Lab Discovery (RT setup / qPCR setup / plate / cycling / raw acquisition)

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`；没有 `ReviewRecord`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 范围: 仅湿实验步骤与原始读数：RNA 上样/QC 门槛引用、RT 配制与温育、qPCR 反应配制、板布局（含对照/重复/标准品位置类型）、热循环（含 RT 相与 UDG/激活相）、实时荧光原始采集（含熔解曲线原始数据）。
- 不在范围（out-of-scope，一律非操作）: ΔCq / ΔΔCq / 归一化方法 / 效率校正计算 / 统计检验 / 生物学解释。Cq 只记录到“原始 Cq + 判定方法 + 软件版本”，不做任何派生。
- 规则: 现有仓库记录只用于 Reuse Map 身份对照，不作步骤/数值证据；每条数值/行为证据只引本文件 §1 的原始厂商手册；不提出任何新公共模块；不写 PortSpec/Transition 全文；不做代码/UI/DB 变更。
- 禁止声明: 本文件不含任何通用循环程序、通用 reference gene、通用阈值、通用效率值；出现的所有温度/时间/浓度均为所引手册的 assay-kit 绑定值，不得跨 assay 复制。

## 1. 来源索引

| # | Protocol | 权威类型 | 版本/定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | SuperScript IV Reverse Transcriptase User Guide（SSIV RT 单酶版） | 厂商手册 | 2015-02-03 版 PDF pp.1–2：20 µL 反应表 + RT protocol 步骤 1–6 全文 | 直接读：退火表 + RT 混合表 + 温育/灭活/可选 RNase H 全文转写 |
| S2 | Bio-Rad iScript cDNA Synthesis Kit Instruction Manual 4106228C | 厂商手册 | 2008 版 2 页：Reaction Setup 表 + Reaction Protocol + 存放/含量/建议全文 | 直接读：全文转写（含沉淀注记、放大规则、下游 1/10 量规则、lot 1/2 不兼容注记） |
| S3 | Applied Biosystems PowerUp SYBR Green Master Mix User Guide, MAN0013511 Rev. G (2026-03-09) | 厂商手册 | Ch.1 产品/存放/耗材 + Ch.2 配制/上机/分析 + App.B 组分/化学/两步法 + App.C 模板质控全文 | 直接读：pp.5–24 全文转写；故障表只记存在，不引排错值作证据 |
| S4 | Applied Biosystems TaqPath 1-Step RT-qPCR Master Mix CG User Guide, MAN0007959 Rev. 2.0 (2023-02-02) | 厂商手册 | Ch.1 产品/起始模板/存放 + Ch.2 设计/配制（fast/standard 双轨）/上机/分析 + App.B 引物探针设计流程全文 | 直接读：pp.5–21 全文转写；故障表只记存在 |

既有复用对照（只记身份，不复述契约，不作证据）：
R1 `op.liquid_transfer`；R2 `op.sample_aliquot`（conditional，谱系注记）；R3 `op.incubation`（elapsed/条件 only）；R4 `op.centrifugation`；R5 `op.washing`（本批出现 PCR 前 RNA 提取语境外引用，不直接复用）；R6 `FLUO_IMAGE`（仅原始静态图像；本批实时荧光不并入，见 §7 G-5）。

发现用检索摘要（`search-summary-only`，非证据）：立项阶段的 web 检索摘要仅用于定位手册，不引任何数值；凡数值均以 S1–S4 直接读页为准。

---

## 2. P-RT-SSIV（S1；单酶两步法 RT，上游引物策略自选）

绑定摘要：单管 20 µL。退火：1 µL 引物（三选一：50 µM oligo(dT)₂₀ / 50 µM random hexamers / 2 µM 基因特异性反向引物）+ 1 µL 10 mM dNTP（each）+ 模板 RNA（10 pg–5 µg total RNA 或 10 pg–500 ng mRNA，至多 11 µL）+ DEPC/无核酸酶水至 13 µL；混匀短离，65 °C 5 min，冰上 ≥1 min。RT 混合：5× SSIV Buffer 使用前回温至室温、涡旋+短离；4 µL 5× Buffer + 1 µL 100 mM DTT + 1 µL RNaseOUT（40 U/µL）+ 1 µL SSIV RT（200 U/µL）；加盖混匀短离后与退火 RNA 合并。多反应对公共组分做 master mix。温育：random hexamer 先 23 °C 10 min 再继续；oligo(dT)₂₀/GSP 直接 50–55 °C 10 min；80 °C 10 min 灭活。可选（>1 kb 靶标扩增需要时）：1 µL 大肠杆菌 RNase H，37 °C 20 min 去 RNA。产物直接做 PCR 或 −20 °C 存；PCR 起始建议 RT 产物占 PCR 总体积约 10%。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 引物+dNTP+RNA+水至 13 µL（引物三选一；RNA 量窗两档） | `op.liquid_transfer` 参数化 + method-local（引物策略选择，assay 依赖） | 储备液/RNA → 退火前混合管 | 方法绑定：1/1 µL、RNA 10 pg–5 µg / 10 pg–500 ng、上限 11 µL、至 13 µL；终浓度 2.5 µM（dT/hexamer）/ 0.1 µM（GSP）、dNTP 0.5 mM each | S1 p.1 反应表 + p.2 步骤 1a |
| 混匀短离；65 °C 5 min；冰上 ≥1 min | `op.incubation` 参数化（退火保持，elapsed/条件 only）+ 父操作备注（短离为手法） | 混合管 → 已退火 RNA | 方法绑定：65 °C 5 min、冰 ≥1 min；退火完成不断言为 cDNA 合成完成 | S1 p.2 步骤 1b–c |
| Buffer 回温+涡旋短离；4/1/1/1 µL 配 RT 混合，加盖混匀短离 | `op.liquid_transfer` 参数化 | 储备液 → RT 混合管 | 方法绑定：4 µL 5×（→1×）、1 µL 100 mM DTT（→5 mM）、1 µL RNaseOUT（→2.0 U/µL）、1 µL SSIV RT（200 U/µL） | S1 p.1 反应表 + p.2 步骤 2 |
| RT 混合加入已退火 RNA；多反应 master mix | `op.liquid_transfer` 参数化 | RT 混合 + 已退火 RNA → 待温育 RT 管 | 方法绑定；master mix 为减误差手法，非操作 | S1 p.2 步骤 3 + 表头注记 |
| random hexamer 23 °C 10 min（前置）；全体 50–55 °C 10 min；80 °C 10 min 灭活 | `op.incubation` 参数化（三段保持；elapsed/条件 only） | 待温育管 → cDNA 产物管 | 方法绑定：23 °C 10 min（hexamer 专用分支）、50–55 °C 10 min、80 °C 10 min；RT 效率/均一性不断言 | S1 p.2 步骤 4 |
| 可选 RNase H 1 µL，37 °C 20 min（>1 kb 需要时） | method-local（RNA 去除分支，靶标长度依赖） | cDNA-RNA 双链 → 已去 RNA cDNA | 方法绑定：1 µL、37 °C 20 min；是否需要由下游扩增决定 | S1 p.2 步骤 5 |
| 直接 PCR 或 −20 °C 存；RT 产物约占 PCR 体积 10% | 方法模板内流转规则 + ToolAction（体积换算） | cDNA → PCR 输入 / 存档 | 方法绑定：−20 °C；10% 为起始建议非定值 | S1 p.2 步骤 6 + 注记 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（退火配制/RT 混合/合并/master mix） | 细胞培养、微生物、ELISA、流式/IF、分子柱提/TRIzol/PCR（R1 已有） | shared（`op.liquid_transfer` 参数化） | S1 全部体积绑定；本批新增单酶 RT 家族证据 |
| 孵育/保持（65 °C 退火、23/50–55/80 °C 三段、37 °C 可选） | 多家族（R3 已有） | shared（elapsed/条件 only；合成完成不断言） | S1 步骤 1/4/5；引物策略分支留方法内 |
| 引物策略选择（dT/random/GSP）/ RNase H 去留 | 仅 RT 家族 | method-local | G1 不过；策略改变 analyte 路径，G2 不兼容转移/洗涤 |
| cDNA 浓度/效率/储存稳定性判读 | —（派生/判断） | 非操作（out-of-scope） | 不建模块；效率无来源值 |

---

## 3. P-RT-ISCRIPT（S2；预混两步法 RT，引物共混）

绑定摘要：单管 20 µL：4 µL 5× iScript reaction mix + 1 µL iScript RT（RNase 抑制剂已预混；RNase H+ 改良 MMLV）+ RNA（100 fg–1 µg total RNA）+ 无核酸酶水至 20 µL。5× mix 融化后可能有沉淀，属正常，须充分重悬。温育：25 °C 5 min → 42 °C 30 min → 85 °C 5 min → 可选 4 °C hold。引物为 mix 内预混 oligo(dT)+random hexamer，优化 <1 kb 靶标。RNA >1 µg 时按比例放大（2 µg→40 µL；5 µg→100 µL）。下游 PCR 建议 cDNA 用量 ≤ 反应体积 1/10（典型 2 µL）。−20 °C 存（恒温非无霜），≥1 年；水可室温。lot 首位 2 与首位 1 不兼容。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 融化后沉淀充分重悬；4/1 µL + RNA + 水至 20 µL（RNA 100 fg–1 µg） | `op.liquid_transfer` 参数化 + 父操作备注（重悬手法） | 储备/RNA → 待温育 RT 管 | 方法绑定：4 µL 5×、1 µL RT、RNA 量窗、至 20 µL | S2 p.2 Reaction Setup 表 + 注记 |
| 25 °C 5 min → 42 °C 30 min → 85 °C 5 min → 可选 4 °C hold | `op.incubation` 参数化（程序化保持序列；elapsed/条件 only） | 待温育管 → cDNA 产物管 | 方法绑定：25/5、42/30、85/5；合成效率不断言 | S2 p.2 Reaction Protocol |
| >1 µg 按比例放大（40/100 µL 例）；下游 ≤1/10 体积（典型 2 µL） | 方法模板内流转规则 + ToolAction（体积换算） | cDNA → 放大体系 / PCR 输入 | 方法绑定：40 µL/2 µg、100 µL/5 µg、1/10、2 µL 典型 | S2 p.2 Recommendations |
| lot 首位 2 vs 1 不兼容；−20 °C 存放 | 前置条件/储存约束（非操作） | — | 方法绑定；混 lot = 失败模式 | S2 p.1 Storage/Note + Kit Contents |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（预混配制/放大/下游取样） | 多家族（R1 已有） | shared（`op.liquid_transfer` 参数化） | S2 体积/量窗绑定；本批新增预混 RT 家族证据 |
| 孵育/保持（三段 + hold） | 多家族（R3 已有） | shared（elapsed/条件 only） | S2 Protocol；与 PCR 热循环不合并（见 §7 G-3） |
| 预混引物共混组成（dT+hexamer，<1 kb 优化） | 仅本试剂盒 | method-local | G1 不过；组成改变 priming 路径，禁作通用参数 |
| cDNA 用量/效率/兼容性判读 | — | 非操作 | lot 规则留方法前置；无效率值 |

---

## 4. P-SYBR-2STEP（S3；两步法 SYBR qPCR：cDNA/gDNA → 实时扩增 + 熔解）

绑定摘要：输入 1–10 ng 单链 cDNA 或 10–100 ng gDNA / 反应；每反应 4 重复；配制加 10% overage；<10 µL 不推荐；按需成比例放大。10 µL/孔：5 µL 2× mix + 正/反引物（各 300–800 nM，Fast/Standard 通用）+ 模板+水；20 µL/孔：10 µL 2× mix + 同上。流程：swirl 混匀 mix；DNA/引物冰上融化、涡旋+短离；组分混匀+短离去泡；分装至光学板；光学膜密封+短离集液去泡；室温避光至多 72 h 内可上机。NTC：除样品外全组分，应无 Ct。ROX 为内含被动参比；Bio-Rad/Roche 系不支持 ROX 归一化（可检出但不归一，分析时不套用）。循环（三选一，依引物 Tm/模板/仪器）：Fast（Tm≥60）：UDG 50 °C 2 min → 激活 95 °C 2 min → 40×（95 °C 1 s〔QuantStudio/ViiA 7/CFX Opus/LC PRO〕或 3 s〔7500 Fast/7900HT Fast/StepOne/StepOnePlus〕+ 60 °C 30 s）。Standard（Tm≥60）：UDG 50/2 → 激活 95/2 → 40×（95/15 s + 60/1 min）。Standard（Tm<60）：UDG 50/2 → 激活 95/2 → 40×（95/15 s + 退火 55–60 °C（取引物 Tm）15 s + 72/1 min）。gDNA 推荐 Standard；7900HT/7500 只用 Standard。熔解（dissociation）：95 °C 15 s（1.6 °C/s）→ 60 °C 1 min（1.6 °C/s）→ 95 °C 15 s（0.15 °C/s，continuous）；避光 72 h 内 / 见光 24 h 内可补做。分析只用各仪器原生软件；禁跨平台直接比较绝对 Ct（趋势/相对差异可比）。RNA 上游门槛（App.C，引用为前置）：0.002–0.2 µg/µL、gDNA <0.005 wt%、无 RT/PCR 抑制剂、PCR 相容缓冲、无 RNase（怀疑时 RT 加抑制剂至 1.0 U/µL）、不预变性（可能降产）；DNA 质控：单带电泳 + A260/280 1.8–2.0。兼容性警告：SSIV VILO 合成 cDNA 在部分 assay/样品中高背景，S3 推荐换用 High-Capacity 系（§“Required materials”）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| swirl mix；DNA/引物冰上融化涡旋短离；按表配制（+10% overage；4 重复；<10 µL 禁） | `op.liquid_transfer` 参数化 | 储备/模板 → 反应混合液 | 方法绑定：5/10 µL 2×、引物各 300–800 nM、模板 1–10 ng cDNA / 10–100 ng gDNA、10%、4 重复 | S3 Ch.2 Set up the PCR reactions + Guidelines |
| 混匀短离去泡；分装光学板；密封+短离；室温避光 ≤72 h 可上机 | `op.liquid_transfer`（分装）+ `op.centrifugation` 参数化（集液/去泡，非沉淀） | 混合液 → 待上机板 | 方法绑定：光学板/膜、短离、≤72 h、避光 | S3 Ch.2 步骤 2–4 |
| 板文档配置（实验类型/染料/被动参比/体积/模式） | 方法模板内上机前置（非操作） | 待上机板 → 已编程仪器 | 方法绑定：SYBR/SYBR/None/ROX、Fast/Standard 与耗材匹配；Bio-Rad/Roche 不套 ROX 归一 | S3 Ch.2 Set up plate document + p.9 染料兼容 |
| 三选一热循环（Fast / Standard Tm≥60 / Standard Tm<60，gDNA 与 7900HT/7500 约束） | 方法模板内程序（不拆为多次 `op.incubation`；循环语义不同） | 待上机板 → 已扩增板 + 实时荧光原始曲线 | 方法绑定：UDG 50/2、激活 95/2、40×（1 s/3 s vs 15 s；60 °C 30 s vs 1 min；Tm<60 三步 55–60/15 s + 72/1 min） | S3 Ch.2 Set up and run + Tables 2–4 |
| 熔解三步（1.6/1.6/0.15 °C/s；continuous） | method-local（特异性证据采集，SYBR 化学依赖） | 已扩增板 → 熔解原始数据 | 方法绑定：95/15 s + 60/1 min + 95/15 s；72 h 暗 / 24 h 光窗口 | S3 Ch.2 Table 5 + App.B Dissociation |
| 原生软件看扩增图、算 baseline/Ct、查熔解峰、NTC 核验 | 方法模板内原始读数（只到原始 Ct + 判定方法 + 软件版本） | 已扩增板 → 原始 Ct/图/熔解峰 | 方法绑定；跨平台绝对 Ct 禁比；相对/绝对定量下游 out-of-scope | S3 Ch.2 Analyze results + App.B |
| RNA/DNA 上游质控门槛引用（浓度窗、gDNA 占比、抑制剂、RNase、A260/280、电泳单带） | 方法前置（非操作；本批不执行提取） | 上游样品 → 合格待转录/扩增样品 | 方法绑定：0.002–0.2 µg/µL、<0.005%、1.0 U/µL、1.8–2.0 | S3 App.C + RNA guidelines |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（配制/master mix 分装/模板加样） | 多家族（R1 已有；B04 P-PCR 家族同类） | shared（`op.liquid_transfer` 参数化） | S3 反应表；本批新增 SYBR-qPCR 家族证据 |
| 板短离集液/去泡 | 传代/计数/流式离心 + 本批板集液 | shared conditional（`op.centrifugation` 参数化，集液注记；非沉淀分离） | S3 步骤 2/4；RCF/转子 UNKNOWN（`configuration_dependent`） |
| 热循环程序（UDG/激活/40×/熔解） | 仅 PCR 家族 | 方法模板内程序（不复用 `op.incubation`） | B04 P-PCR 规则延续；合并即 G2 失败 |
| SYBR 熔解特异性证据 | 仅 SYBR 家族 | method-local | G1 不过；染料化学依赖，不并入通用读数 |
| 实时荧光原始采集（扩增曲线/Ct） | 仅 qPCR 家族 | 方法模板内读数（不并入 `FLUO_IMAGE`） | 见 §7 G-5；Cq 判定软件版本依赖 |
| 引物设计/浓度优化/基线阈值调试/定量换算 | —（设计/计算/判断） | ToolAction 或 out-of-scope | S3 App.D/ troubleshooting；不建操作 |

---

## 5. P-1STEP-TAQPATH（S4；一步法探针 RT-qPCR：RNA/DNA → 同孔 RT + 扩增）

绑定摘要：4× CG mix（含 Fast DNA 聚合酶、热稳定 MMLV RT、UNG、dUTP 系 dNTP、RNase 抑制剂、ROX、抑制剂耐受缓冲；粘稠，混匀靠翻转非涡旋；−25~−15 °C 存，不冻结可凝胶、淡黄正常）。双轨配制：Fast（≤30 µL/反应，示例 20 µL）：5 µL 4× mix + 1 µL 20× ppmix（引物 400–900 nM、探针 100–250 nM）+ 样品（按需至体积上限）+ 水至 20 µL。Standard（>30 µL，示例 50 µL）：12.5 µL 4× + 2.5 µL 20× ppmix + 样品 + 水至 50 µL。10% overage。冰上操作：试剂冰上融化；assay 涡旋+短离；样品轻柔翻转/弹 3–5× + 短离；确认无凝胶块。Bulk 混合（mix+assay+水，不含样品）翻转 ≥10× + 短离；按（20−样品）或（50−样品）分装光学板孔；加样；光学膜密封后翻转 3–5×（孔内液体 seal↔底往返）+ 150×g 1 min 去泡。循环：Fast（≤30 µL）：UNG 25 °C 2 min → RT 50 °C 15 min（最适 48–55 °C）→ 激活 95 °C 2 min（兼 RT 灭活/预变性/热启动）→ 40×（95 °C 3 s + 60 °C 30 s）。Standard（>30 µL）：UNG 25/2 → RT 50/15 → 激活 95/2 → 40×（95/15 s + 60/60 s）。孔类型：unknown / IPC（外源内标）/ NAC（无扩增对照）/ NTC / 标准品稀释系列（AQ 用，可省标只收 Ct）/ ≥3 重复。no-RT 对照：分装 mix 95 °C 5 min 热杀 RT 后再配（PCR 热启动室温后恢复）。扩增子设计（App.B，引用为前置）：50–150 bp、跨内含子、探针优先设计。分析：软件看扩增图、设 baseline/threshold、去离群、读各孔/重复组 Ct；AQ 读斜率/效率/R²/截距（下游派生，out-of-scope）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义（现有模块/候选） | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 冰上融化；assay 涡旋短离；样品翻转/弹 3–5×短离；无凝胶块确认 | `op.liquid_transfer` 参数化 + 父操作备注（翻转/涡旋手法） | 储备/样品 → 待配制组分 | 方法绑定；4× 粘稠翻转>涡旋；凝胶块为阻断性完成检查 | S4 Ch.2 Prepare RT-qPCR reaction mix 步骤 1–4 + 注记 |
| 按轨配制（20 µL：5/1 µL；50 µL：12.5/2.5 µL；+10% overage；引物/探针浓度窗） | `op.liquid_transfer` 参数化（多次加样 = 重复转移） | 储备 → bulk 混合液（不含样品） | 方法绑定：5/1、12.5/2.5 µL；引物 400–900 nM、探针 100–250 nM；20× ppmix 换算 | S4 Ch.2 Fast/Standard 表 |
| Bulk 翻转 ≥10× + 短离；按（总体积−样品）分装；加样；密封翻转 3–5× + 150×g 1 min | `op.liquid_transfer` + `op.centrifugation` 参数化（150×g 1 min 板集液/去泡） | bulk 液 → 已加样密封板 | 方法绑定：≥10×、3–5× seal↔底、150×g 1 min | S4 Ch.2 步骤 6–9（双轨同文） |
| 板文档/实验配置（fast/standard 模式 + 体积 + 循环表） | 方法模板内上机前置 | 密封板 → 已编程仪器 | 方法绑定：20 µL/fast vs 50 µL/standard；ramp/采集取默认 | S4 Ch.2 Set up plate document |
| 同孔两相：UNG 25/2 → RT 50/15 → 激活 95/2 → 40×扩增（fast 3 s+30 s / standard 15 s+60 s） | 方法模板内程序（RT 相 + 循环；不拆 `op.incubation`；无存档 cDNA 中间体） | RNA/DNA 样品 → 已扩增板 + 实时荧光原始曲线 | 方法绑定：25/2、50/15（48–55 最适）、95/2、40×两档时间；退火 56–62 °C 可调（Tm 一致） | S4 Ch.2 双循环表 + RT-qPCR guidelines |
| 孔类型布局（unknown/IPC/NAC/NTC/标准系列/≥3 重复；no-RT 热杀分支） | 方法对照矩阵（非操作；存在性记录） | — | 方法绑定；NAC/NTC 应无靶扩增、仅 IPC 可扩增 | S4 Ch.2 Design the experiment 表 + No-RT 注记 |
| 软件看图、设 baseline/threshold、去离群、读 Ct；AQ 读曲线参数 | 方法模板内原始读数 + 下游派生（out-of-scope） | 已扩增板 → 原始 Ct（+判定方法/软件版本） | 方法绑定；阈值/效率/统计一律下游，不建操作 | S4 Ch.2 Analyze the data |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（bulk 配制/分装/加样/overage） | 多家族（R1 已有） | shared（`op.liquid_transfer` 参数化） | S4 双轨表；本批新增一步法家族证据 |
| 板翻转混匀 + 150×g 集液/去泡 | 多家族离心 + 本批板集液 | shared conditional（`op.centrifugation` 参数化，集液注记） | S4 150×g 1 min；转子/适配器 UNKNOWN |
| UDG/RT/激活/40× 同孔程序 | 仅一步法家族 | 方法模板内程序（不复用 `op.incubation`） | G1 不过；RT 相无存档 cDNA（one-step 无 `cDNA_out` 端口，见 k2 R2/R3 区分） |
| 探针化学/多重/IPC/NAC 设计 | 仅探针家族 | method-local + ToolAction（浓度换算） | S4 App.B；G1/G2 不过 |
| 实时荧光原始采集 | 仅 qPCR 家族 | 方法模板内读数 | 见 §7 G-5 |
| 效率/标准曲线/统计 | — | out-of-scope，非操作 | S4 Analyze；禁携值 |

---

## 6. Reuse Map 汇总（本批）

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 退火配制/RT 混合/合并/master mix（SSIV）；预混配制/放大/下游取样（iScript）；SYBR 配制/分装/加模板；一步法 bulk/分装/加样 | `op.liquid_transfer` | 参数化复用；绑定体积/浓度/量窗/overage（§§2–5） |
| 退火 65 °C、RT 三段/程序化保持、储存窗（RT 两协议） | `op.incubation` | 参数化复用；只断言 elapsed/条件；合成/退火完成不断言 |
| 板短离集液/去泡（SYBR；一步法 150×g 1 min） | `op.centrifugation` | 参数化复用（集液注记，非沉淀分离）；转子/RCF 换算缺口 |
| cDNA/反应混合分装到管/孔（含 (V−样品) 分装语境） | `op.sample_aliquot` | 参数化复用（conditional + 谱系注记）；守恒 UNKNOWN |
| 热循环 40× / RT 同孔相 / UDG / 激活 / 熔解程序 | — | 全部 method-local（方法模板内程序）；不晋升（§7 G-3） |
| 引物策略/预混组成/探针/IPC/扩增子设计 | — | method-local + ToolAction；不晋升（§7 G-4） |
| 实时荧光原始采集（扩增曲线/原始 Ct/熔解峰/NTC 结果） | — | 方法模板内读数；不并入 `FLUO_IMAGE`（§7 G-5） |
| ΔCq/ΔΔCq/效率/标准曲线拟合/统计/生物学解释 | — | out-of-scope；一律非操作 |

---

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | P-RT-SSIV | P-RT-iScript | P-SYBR-2STEP | P-1STEP-TaqPath | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 退火/混合/合并 | ● 预混/放大/取样 | ● 配制/分装/加模板 | ● bulk/分装/加样 | 最高复用；全参数化 |
| `op.incubation` | ● 65/23/50–55/80/37 °C 段 | ● 25/42/85/4 °C 段 | ○ UDG/激活为程序内相（不拆） | ○ UDG/RT/激活为程序内相（不拆） | 仅等温 RT/退火保持复用；循环内相不拆 |
| `op.centrifugation` | ○ 短离手法（备注） | ○ — | ● 板短离集液 | ● 150×g 1 min 集液 | 集液≠沉淀；RCF 绑定仅一步法有值 |
| `op.sample_aliquot` | ○ cDNA→PCR 取样语境 | ○ 下游 1/10 取样 | ○ 模板分装语境 | ● (V−样品) 分装 | 谱系关键步；守恒 UNKNOWN |
| method-local 扩增/熔解/探针 | — | — | ● 40×三选一 + 熔解 | ● 同孔 RT+40×双轨 + 探针 | 四者互不合并（化学/相结构不同） |
| ToolAction/前置/分析 | ○ 10% 换算 | ○ 体积换算 | ● 引物窗/质控门槛/基线阈值 | ● ppmix 换算/扩增子设计/基线阈值 | 全部非操作或前置 |

●=主路径使用；○=语境提及/程序内相；—=本批未使用。

三者互不可比：单酶 RT（策略自选）/ 预混 RT（共混引物）/ SYBR 两步（染料非特异）/ 一步探针（同孔 RT+特异探针）的作用对象与完成判据不同；SYBR 熔解峰 ≠ 探针特异性，实时 Ct ≠ 终点胶带亮度，相对/绝对定量互不可比（k2 R3 规则延续）。

---

## 8. 缺口清单（不能表达/UNKNOWN；分类按 record-standard §4）

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | 两步：NTC（SYBR 必须，S3 E 级语境）+ 模板质控门槛；一步：IPC/NAC/NTC/标准系列/≥3 重复/no-RT 热杀分支（S4）。任一缺失即对应声明不可信 |
| G-2 | 派生归属 | ToolAction / out-of-scope，非操作 | ΔCq/ΔΔCq、效率（斜率来源）、标准曲线拟合、归一化描述、统计方法、Ct 离群处置——全部下游；本批只到原始 Ct + 判定方法 + 软件版本 |
| G-3 | 方法边界 | 模型边界 | 等温 RT 保持可复用 `op.incubation`；但 40× 变温循环 / 同孔 RT 相 / UDG / 激活 / 熔解整体≠孵育：合并即 G2 失败（端口/effect/完成检查不兼容），一律方法模板内程序 |
| G-4 | 试剂/设计依赖值 | `method_dependent` | 引物 300–800 nM（SYBR）vs 400–900 nM + 探针 100–250 nM（TaqPath）为各手册绑定值，不通用；扩增子 50–150 bp、跨内含子、探针优先为设计前置；per-lot 寡核苷酸单未读 |
| G-5 | 读数边界 | 模型边界 | 实时荧光原始采集（曲线/Ct/熔解）不并入 `FLUO_IMAGE`（静态图像）或 `ABS_READ`（终点 OD）：动力学/通道/基线阈值 effect 不同；发光类比（audit G-1）同样禁并入 |
| G-6 | 样本/体系依赖值 | `method_dependent` | RNA 量窗（10 pg–5 µg / 100 fg–1 µg）、cDNA/gDNA 上样窗（1–10 ng / 10–100 ng）、RT 温度窗（50–55 vs 48–55 最适）、退火 56–62 °C、反应体积（10/20 vs 20/50 µL）——均为协议归属值 |
| G-7 | 设备/试剂态 | `configuration_dependent` / `device_state` / `source_missing` | 循环仪块均匀性/体积档、通道-滤光-染料匹配、ROX 归一行为（Bio-Rad/Roche 例外）、Cq 软件版本、板/膜耗材目录、移液校准、−25~−15 °C 实际箱温——S1–S4 均无通用值，单机构建未读 |
| G-8 | 力学/手法量 | `source_missing` | swirl/涡旋/翻转强度、≥10×/3–5× 次数外强度、短离 RCF（除 150×g 外）、残留气泡判据、凝胶块触觉标准——原文无量化，归 SOP/操作者判断 |
| G-9 | 不可逆跃迁 | 跃迁约束 | 扩增后孔为终点（含 UDG 后残留产物风险，S3/S4 良好规范语境）；cDNA 不回转为 RNA；一步法无存档 cDNA 中间体；污染处置 SOP 门控 |
| G-10 | 时间锚点 | 锚点缺口 | RT `t₀`=混合合并完成；qPCR `t₀`=循环启动；UNG/RT/激活为程序内相时钟；熔解为程序后时钟；储存（−20 °C cDNA / 板 ≤72 h / mix+assay ≤4 周）为独立保持窗。孵育不断言合成/扩增完成 |
| G-11 | 范围外（误建模块风险） | `not_applicable` | 临床/动物在体、数字 PCR、等温扩增、一步-两步效率比较、FFPE 修复、测序 QC（RIN-for-RNA-seq/DIN）、图像分割/定量、统计/生物信息——本批一律不记录、不建模 |

---

## 9. 晋升审查结论：零新模块

- 四问（独立目的/稳定边界/独立确认/有意义复用）下，本批真正的“新语义”只有四个：单酶 RT 合成、预混 RT 合成、SYBR 变温扩增+熔解、一步法同孔 RT-扩增。它们各自只出现在 1 个手册家族内（SSIV / iScript / PowerUp / TaqPath），G1 不过；且任一与 `op.incubation`/`op.washing`/`FLUO_IMAGE`/`ABS_READ` 的 effect/端口/完成检查均不兼容，G2 不过。
- 因此全部留在 method-local（方法模板内步骤 / Custom Step 出口），不进入公共 Catalog，不建 PortSpec/Transition 全文，不做代码/UI/DB 变更。
- 本批对公共 Catalog 的唯一贡献是复用证据增量：`op.liquid_transfer`（+RT/两种 qPCR 配制家族）、`op.incubation`（+退火/RT 等温保持）、`op.centrifugation`（+板集液/去泡语境，conditional）、`op.sample_aliquot`（+cDNA/反应分装谱系）。

## 10. 核查与状态

- 已验证（直接读，2026-11-05，§1 定位节）：S1 退火表 + RT 表 + 步骤 1–6；S2 配制表 + 三段程序 + 建议；S3 反应表 + Tables 2–5 + App.B/C；S4 双轨配制表 + 双循环表 + 对照矩阵 + App.B。单人 `agent-verified`，仍需第二人逐字段复核；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：任何仪器操作手册/单机构建表、转子映射、per-lot 寡核苷酸单/片、实验室 SOP 改编、试剂批间差、引物二聚体实例、任何已跑数据的派生应用。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器规格分离（§§2–5 仅能力 + 用过体积作例）；样本跃迁有端口（§§2–5 表 1）；时间锚显式；工作值标协议归属；部分列表标部分；plan/actual/raw/derived 分离（原始曲线/Ct 保留，派生下游）；无 JSON；UNKNOWN 永不填零。
