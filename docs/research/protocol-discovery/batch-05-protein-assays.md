# Batch 05 — Protein Assays: ELISA Variants / Western Blot Detection Modes

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`；没有 `ReviewRecord`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 范围: 4 个湿实验 protocol 的步骤与原始读数：sandwich ELISA（colorimetric / chemiluminescent / fluorescent 三读出）、indirect 与 competitive ELISA（信号方向不同，独立变体）、Western blot 通用程序（transfer check → block → probe → wash → develop → image）、WB 三检测模式（colorimetric / chemiluminescent / fluorescent + chemifluorescence 邻接）。只记录方法绑定与差异，不重教通用操作原理。
- 规则: ELISA 各变体不合并（sandwich 递增 vs competitive 递减 vs indirect 直接包被+二抗放大）；WB 三检测模式不合并（沉淀终点 vs 时间关键发光 vs 稳定荧光）；标准曲线拟合、densitometry、统计一律记为 ToolAction 或 out-of-scope，不建操作；新模块须过四维粒度判定 + 晋升门槛（≥2 独立方法家族），否则留 method-local。本文件不提出任何新公共模块。
- 现有仓库记录（`k2-operations/elisa-and-western-blot.md` 等）仅用于 Reuse Map，不作步骤/数值证据；每条数值证据只引本文件 §1 的原始 protocol/厂商手册。

## 1. 来源索引与实际读取深度

| # | 来源 | 权威类型 | 定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Thermo Fisher, Basic Sandwich ELISA Protocols（colorimetric / chemiluminescent / fluorescent 三程序 + Antibody dilution tables + AP/PNPP 与 direct-immobilisation / labelled-secondary 变体注记）https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/elisa-protocol/general-elisa-protocol/basic-sandwich-elisa-protocol.html | 厂商手册 | 全页 Materials + 三程序 Procedure 逐步 + dilution tables | 直接读：colorimetric 步骤 1–17、chemi 步骤 1–17、fluoro 步骤 1–19、dilution 表两张 |
| S2 | Bio-Rad Antibodies, Sandwich ELISA with Streptavidin-biotin Detection（Method 步骤 1–7 + Reagents 1–3）https://www.bio-rad-antibodies.com/sandwich-elisa-protocol-with-streptavidin-biotin-detection.html | 厂商手册 | Reagents + Method 1–7 全文 | 直接读全文 |
| S3 | Bio-Rad Antibodies, Indirect ELISA（Method 1–6）https://www.bio-rad-antibodies.com/indirect-elisa-protocol.html；Competitive ELISA（Method 1–7）https://www.bio-rad-antibodies.com/competitive-elisa-protocol-inhibition.html | 厂商手册 | 两页 Method 全文 | 直接读两页全文；S3 主证据为 competitive 递减方向，indirect 作对照变体 |
| S4 | Bio-Rad Antibodies, Western Blotting Protocol（Reagents + Method 1–8）https://www.bio-rad-antibodies.com/western-blot-protocol.html；Bio-Rad Detection Methods（colorimetric / chemiluminescence / fluorescence / other 机制 + 灵敏度表）https://www.bio-rad.com/en-us/applications-technologies/detection-methods?ID=LUSQ6KKG4 | 厂商手册 + 厂商机制页 | WB 程序 1–8 + Detection 全节机制/表/曝光约束 | 直接读：WB 程序全文；Detection 页 colorimetric 表、chemi 表（曝光 5 s–5 min、工作液 24 h RT、信号 6–8 h/24 h）、fluorescence（multiplex/10× 动态范围/存档数周）、chemifluorescence 邻接段 |

未用作证据：BioLegend sandwich 页（403 未读到正文）、CSH Immunoblotting 系列（仅检索摘要，未读正文，不引用）、任何本地 SOP/已跑数据。

## 2. P-ELISA-SW (S1 Thermo sandwich 三读出；S2 Bio-Rad streptavidin-biotin 佐证)

绑定摘要：clear 96 孔（colorimetric）/ white opaque（chemi；black 背景高时备选）/ black（fluoro)。包被 100 µL/孔（fluoro 50–100 µL），1 h RT 或 overnight 12–18 h 2–8 °C；洗 1×（>300 µL/孔，tap dry）→ 300 µL/孔 block 1 h RT → 100 µL/孔 standards（duplicate）+ samples，1–2 h RT ~500 rpm 振荡 → 洗 5×（>300 µL/孔）→ 100 µL/孔 detection Ab 2 h RT ~500 rpm（fluoro 限定 0.05–0.1 µg/mL in wash buffer，1–2 h）→ biotin 路径 100 µL/孔 streptavidin-HRP 1 h RT ~500 rpm（colorimetric 1:5,000；chemi 1:5,000–1:20,000 经验滴定；fluoro 0.05–0.1 µg/mL，50–100 µL/孔）→ 洗 5× → 显色/读数三分支：TMB 100 µL/孔 30 min RT + 100 µL stop（0.16 M H₂SO₄）→ 450 nm ≤30 min 内读数；chemi 100 µL/孔 1:1 luminol:peroxide 1 min RT → 1–5 min 内 RLU（~425 nm）；QuantaBlu 9:1 substrate:peroxide 100 µL/孔 1.5–90 min RT/37 °C + 100 µL stop → RFU ex 325/em 420 nm（可用 315–340/370–470）。S2 佐证：包被 1–10 µg/mL 100 µL/孔 overnight 4 °C → 200 µL/孔 block 60 min RT → 100 µL 样品/标准 90 min RT（triplicate 优先）→ 生物素检测 Ab 1 h RT → 酶-streptavidin 60 min RT → 底物 ~30 min → 50 µL stop 混匀 30 min 内读数；coating buffer pH 9.6 碳酸盐；wash PBS + 0.05% Tween-20；TMB/0.2 M H₂SO₄ vs pNPP/1 M NaOH 两套。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 包被 100 µL/孔（fluoro 50–100 µL）；1 h RT 或 12–18 h 2–8 °C | `op.liquid_transfer` 参数化 + `op.incubation` 参数化 | 空板 → 已包被板 | 方法绑定：体积/浓度（fluoro 5–10 µg/mL；colorimetric/chemi 按稀释表/说明书）/缓冲液 pH 9.4 或 7.4/时间温度二选一 | S1 三程序步骤 1–2；S2 Method 1 |
| 洗 1×（>300 µL/孔，tap dry） | `op.washing` 参数化 | 已包被板 → 已洗板 | 方法绑定：>300 µL，1× post-coat；tap dry 完成检查 | S1 步骤 3；S2 Method 1（3× PBS 变体，冲突保留见 §7） |
| Block 300 µL/孔 1 h RT（S2 200 µL 60 min） | `op.liquid_transfer` + `op.incubation` 参数化 | 已洗板 → 已封闭板 | 方法绑定：300 µL vs 200 µL 冲突保留，各守其 SOP，不平均；SuperBlock/StartingBlock vs PBS+1% BSA 品种绑定 | S1 步骤 4；S2 Method 2 |
| Standards（duplicate/triplicate）+ samples 100 µL/孔；1–2 h RT ~500 rpm（S2 90 min RT） | `op.liquid_transfer` + `op.incubation` 参数化 | 已封闭板 + 样品 → 已结合板 | 方法绑定：100 µL/孔；duplicate（S1）vs triplicate 优先（S2）冲突保留；振荡 ~500 rpm；稀释液 blocking/wash buffer 按程序 | S1 步骤 6–7；S2 Method 3 |
| 洗 5×（>300 µL/孔，tap dry）；S2 全程 3× | `op.washing` 参数化 | 已结合板 → 已洗结合板 | 方法绑定：5×（S1 assay steps）vs 3×（S2）冲突保留，不平均；E3 通用 3–4× 仅参照 | S1 步骤 8；S2 Method 3–5 |
| Detection Ab 100 µL/孔 2 h RT ~500 rpm（fluoro 0.05–0.1 µg/mL，1–2 h） | `op.liquid_transfer` + `op.incubation` 参数化 | 已洗板 → 已检测板 | 方法绑定：浓度按说明书/稀释表（polyclonal serum 5–15 coat/1–10 detect；affinity-purified 1–12/0.5–5 µg/mL 均为起点，需优化）；HRP 直标则跳过 conjugate 步 | S1 步骤 9–10 + dilution 表；S2 Method 4 |
| Biotin 路径 streptavidin-HRP 100 µL/孔 1 h RT（1:5,000；chemi 至 1:20,000；fluoro 0.05–0.1 µg/mL） | `op.liquid_transfer` + `op.incubation` 参数化 | 已检测板 → 已偶联板 | 方法绑定（条件：仅 biotinylated detection）：稀释度/浓度/体积按读出分支；exact factor UNKNOWN 需滴定 | S1 步骤 11–13（三分支值不同）；S2 Method 5 |
| 底物显色：TMB 100 µL 30 min / chemi 1:1 100 µL 1 min / QuantaBlu 9:1 100 µL 1.5–90 min；AP 备选 PNPP 15–30 min | `op.liquid_transfer` + `op.incubation`（定时保持）参数化 | 已偶联板 → 已显色板 | 方法绑定：体积/比例/时长/温度/避光按底物分支；无通用显色值 | S1 步骤 15 + AP/PNPP 备选段；S2 Method 6 |
| Stop：TMB 100 µL 0.16 M（S2 50 µL 0.2 M）/ PNPP 50 µL 2 N NaOH / QuantaBlu 100 µL stop | `op.liquid_transfer` 参数化 | 已显色板 → 已终止板 | 方法绑定（条件：colorimetric/AP/quanti-fluoro 才有 stop；chemi 无 stop）：molarity/体积按 lot 记录，冲突保留 | S1 步骤 16 + AP 段；S2 Method 7 |
| 读数：450 nm ≤30 min / RLU ~425 nm 1–5 min / RFU 325/420 | `ABS_READ`（colorimetric）/ luminescence-raw / fluorescence-raw 参数化 | 已终止/显色板 → 原始信号矩阵 | 方法绑定：λ/模式/板色/读数窗；chemi 延迟即衰减（严格时钟） | S1 步骤 17 + 三分支读数行；S2 Method 7 |
| log-log / 4-parameter 拟合；S2 triplicate 均值语境 | ToolAction（确定性计算） | 原始矩阵 + 同板标准 → 派生浓度 | 方法模板内公式，非操作；无拟合不作浓度声明 | S1 步骤 18（Calculate results 指针）；本批不定义算法 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（包被/封闭/样品/检测/偶联/底物/stop） | 细胞培养、微生物、分子、生化板系 | shared（`op.liquid_transfer`） | S1 步骤 2/4/7/10/12/15/16；S2 Method 1–7；本批仅绑定体积/浓度 |
| 孵育/保持（包被、封闭、结合、偶联、显色） | 多家族 | shared（`op.incubation`） | S1 各段时长/温度/振荡绑定；只断言 elapsed/条件，不推断结合完成 |
| 洗涤（1× post-coat；5×/3× post-step） | 多家族 | shared（`op.washing`） | S1 5× vs S2 3× 冲突保留；tap dry/无划伤为完成检查 |
| 吸光度读取（原始 OD） | 生化/免疫读数 | shared（`ABS_READ`） | S1 TMB 450 nm + ≤30 min 硬时钟 |
| 发光/荧光原始读数 | 免疫/细胞读数 | luminescence-raw / fluorescence-raw 按分支绑定，不并入 `ABS_READ` | S1 RLU 1–5 min / RFU 325-420；物理/仪器不同（审计 #4.4） |
| 标准曲线/浓度换算 | —（计算） | ToolAction | S1 log-log/4-parameter 指针；跨试剂盒不可比，未建模 |
| 通用 "conjugate"/"develop" | 表面多见但 effect 不稳定 | 不建通用模块 | biotin–streptavidin vs 二抗-酶增益不同（S1 labelled-secondary 注记）；留方法内条件分支 |

## 3. P-ELISA-IND/COMP (S3 Bio-Rad indirect + competitive；S1 direct-immobilisation 注记佐证)

绑定摘要：两变体均直接包被抗原 100 µL/孔 1–10 µg/mL coating buffer，overnight 4 °C，PBS 洗 3× → 200 µL/孔 block（PBS+1% BSA）1 h RT，wash buffer 洗 3×。Indirect：100 µL unconjugated detection Ab 90 min RT → 洗 3× → 100 µL enzyme-conjugated secondary 90 min RT → 洗 3× → 底物 100 µL ~30 min/至显色 → stop 50 µL 混匀 30 min 内读数。Competitive：50 µL 抗原 + 50 µL 抗体混合（wash buffer 系列稀释）1 h RT 预孵 → 100 µL/孔混合物 1 h RT → 洗 3× → 100 µL enzyme-conjugated secondary 1 h RT → 洗 3× → 底物/读数同 indirect。信号方向：indirect 随样品抗原递增；competitive 随样品抗原递减（vs labelled-alone 孔）。S1 佐证 direct-immobilisation 变体：样品直接包被替代 capture Ab，其余链不变；S1 labelled-secondary 注记：非生物素检测 + 酶标二抗增益略低于 biotin–streptavidin，可能需更高 conjugate 浓度（exact factor UNKNOWN）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 抗原直接包被 100 µL/孔 1–10 µg/mL，overnight 4 °C，PBS 洗 3× | `op.liquid_transfer` + `op.incubation` + `op.washing` 参数化 | 空板 → 抗原包被板 | 方法绑定：浓度窗 1–10 µg/mL；S2 sandwich 包被为 capture Ab（对照差异，不合并） | S3 两页 Method 1 |
| Block 200 µL/孔 1 h RT，wash 3× | 同上三操作参数化 | 抗原包被板 → 已封闭板 | 方法绑定：PBS+1% BSA（BUF032–034 备选）；S1 300 µL 冲突保留 | S3 两页 Method 2 |
| Indirect：unconjugated detection Ab 100 µL 90 min RT | `op.liquid_transfer` + `op.incubation` 参数化 | 已封闭板 → 已检测板（indirect 分支） | 方法绑定：稀释按说明书；二抗宿主/交叉吸附计划另立（method_local） | S3 Indirect Method 3 |
| Competitive：50+50 µL 抗原抗体混合 1 h RT 预孵 → 100 µL/孔 1 h RT | `op.liquid_transfer`×2 + `op.incubation`×2 参数化（预孵为方法内条件分支） | 样品/抗体 → 混合物 → 已竞争板 | 方法绑定：系列稀释 wash buffer；预孵 1 h + 板上 1 h 两时钟分离 | S3 Competitive Method 3–4 |
| Enzyme-conjugated secondary 100 µL（indirect 90 min / competitive 1 h RT） | `op.liquid_transfer` + `op.incubation` 参数化 | 已检测/竞争板 → 已偶联板 | 方法绑定：时长按变体；direct-HRP 路径无此步（条件） | S3 Indirect Method 4；Competitive Method 5 |
| 底物 100 µL ~30 min/至显色；stop 50 µL 混匀 30 min 内读数 | `op.liquid_transfer` + 定时保持 + `ABS_READ` 参数化 | 已偶联板 → 原始 OD 矩阵 | 方法绑定：TMB/0.2 M H₂SO₄ vs pNPP/NaOH 按偶联酶；S1 0.16 M/100 µL 冲突保留 | S3 两页 Method 6–7 |
| 竞争方向判读（递减 vs labelled-alone） | 方法模板内解释，非操作 | 原始 OD → 定性方向 | 方法绑定；不得套 sandwich 递增解释 | S3 Competitive 标题/format 指针；本批不定义 cutoff 公式 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/洗涤/吸光度读取 | 多家族（见 §2） | shared（同 §2 绑定） | S3 两页 Method 1–7；仅体积/时长值不同 |
| 抗原直接包被 | ELISA 内多变体（S1 注记 + S3 两页） | method-local（`MT-ELISA-DIRECT-COAT` 内步骤） | 同一方法内分支，不跨家族，不晋升（G1 不过） |
| 竞争预孵混合（50+50 µL） | 仅 competitive | method-local 条件分支 | 单变体步骤；信号方向 effect 不同，不并入 sandwich 结合 |
| 间接二抗放大 | 仅 indirect（含 S1 labelled-secondary 注记） | method-local 条件分支 | 增益/交叉反应与 biotin–streptavidin 不可比（审计 #4.3 类比）；不建通用 "amplify" 模块 |

## 4. P-WB-PROC (S4 Bio-Rad Western Blotting Protocol，步骤 1–8)

绑定摘要：SDS-PAGE 后按 manufacturer 说明转膜（本批 out-of-scope，只记入口）→ Ponceau S 染色 1 min，水完全脱色（light-sensitive）→ blocking solution 2 h RT 或 overnight 4 °C → wash buffer 短 rins → primary（wash buffer 稀释，起点 1–10 µg/mL，按 datasheet）2 h RT 或 overnight 4 °C → 3×10 min wash（gentle agitation）→ enzyme-conjugated secondary（wash buffer 稀释）1 h RT → 4×5 min wash buffer + 3×5 min PBST + 2×5 min PBS（gentle agitation）→ 按 manufacturer 底物显色成像。Wash buffer = blocking buffer + 0.1% Tween-20；block = Block Ace（BUF029）水溶（cleaner 首选）或 5% 脱脂奶 PBS；phospho 按 datasheet 指定封闭。No-primary 样品对照必备（判断二抗非特异）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 1 转膜（按 manufacturer） | 上游入口，本批不拆 | gel → membrane（blotted） | UNKNOWN（电压/时间/缓冲液/膜选择均未给，按仪器说明） | S4 WB Method 1 |
| 2 Ponceau S 1 min，水完全脱色 | method-local `WB-TRANSFER-CHECK`（可逆总蛋白 QC） | 已转膜 → 已质检膜 | 方法绑定：配方（0.1 g/95 mL 水/5 mL 乙酸）+ 1 min + 脱色至净；light-sensitive | S4 WB Reagents（Ponceau S）+ Method 2 |
| 3 Block 2 h RT 或 overnight 4 °C | `op.liquid_transfer`（加封闭液）+ `op.incubation` 参数化 | 已质检膜 → 已封闭膜 | 方法绑定：Block Ace vs 5% 奶二选一；phospho 按 datasheet；时长温度二选一 | S4 WB Reagents（Blocking Buffer）+ Method 3 |
| 4 短 rins + primary 2 h RT/overnight 4 °C（起点 1–10 µg/mL） | `op.washing`（rins）+ `op.liquid_transfer` + `op.incubation` 参数化 | 已封闭膜 → 已一抗膜 | 方法绑定：稀释按 datasheet（exact clone/host UNKNOWN）；gentle agitation 全程 | S4 WB Method 4 |
| 5 3×10 min wash（gentle agitation） | `op.washing` 参数化 | 已一抗膜 → 已洗一抗膜 | 方法绑定：cycles×时长×缓冲液；实际执行记录 | S4 WB Method 5 |
| 6 Secondary 1 h RT（wash buffer 稀释） | `op.liquid_transfer` + `op.incubation` 参数化 | 已洗膜 → 已二抗膜 | 方法绑定：稀释按 datasheet；host 匹配另立 | S4 WB Method 6 |
| 7 终洗 4×5 wash + 3×5 PBST + 2×5 PBS | `op.washing` 参数化（三段缓冲液序列） | 已二抗膜 → 待显色膜 | 方法绑定：三段序列为本方法值，不通用 | S4 WB Method 7 |
| 8 底物显色成像（按 manufacturer） | 方法内显色分支（见 §5 三模式）+ 原始图像采集 | 待显色膜 → 已显影膜 + 原始图像 | 方法绑定：底物/曝光/通道按 §5 模式；chemi 计时严格 | S4 WB Method 8 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（封闭/抗体/底物加样） | 多家族 | shared | S4 Method 3/4/6/8；仅试剂/体积绑定 |
| 孵育/保持（block/一抗/二抗/显色） | 多家族 | shared | S4 2 h RT vs overnight 4 °C 二选一；只断言 elapsed/条件 |
| 洗涤（rins/3×10/终洗三段） | 多家族 | shared（保留语义；脱落场景 lossy 显式） | S4 Method 4/5/7；cycles/缓冲液方法绑定 |
| 转膜后 Ponceau 质检 | 仅 WB | method-local（`WB-TRANSFER-CHECK`） | 单方法 QC；G1 不过，不晋升 |
| 通用 "probe"/"block" | 表面多见但 effect 不稳定 | 不建通用模块 | 封闭剂/抗体/膜/磷酸化效应异质（审计 #23/#26 类比）；留方法内 |

## 5. P-WB-DETECT (S4 Detection Methods：colorimetric / chemiluminescent / fluorescent + chemifluorescence 邻接)

绑定摘要（三模式严格区分，不合并）：Colorimetric：HRP 底物 4CN/DAB（500 pg）、Opti-4CN（100 pg）、Amplified Opti-4CN（5 pg）；AP 底物 BCIP/NBT（100 pg，不褪色）。肉眼监测、按需终止；HRP/4CN 见光褪色；azide 抑制 HRP；AP 遇酸失活。Chemiluminescent：HRP/luminol（Immun-Star HRP 1–3 pg，30 s 曝光，信号 6–8 h）/ WesternC femtogram（CCD 优化，信号至 24 h）；AP/CDP-Star 10 pg（30 s–5 min 曝光，信号至 24 h，可再激活）。典型 CCD 曝光 5 s–5 min；工作液 RT 稳定 24 h；延迟采集即衰减；azide 禁忌同上；AP 内源磷酸酶假阳性。Fluorescent：荧光基团标记一抗/二抗，无液体底物步；需匹配激发/发射（quantum yield / extinction / Stokes / spectra 四要素）；multiplex 要求不同宿主一抗 + 交叉吸附二抗 + 谱不重叠；动态范围 ~10× chemi；blot 存档数周/数月可重拍；strip/reprobe 兼容（膜不干）。Chemifluorescence（邻接，不并入）：酶促生成荧光产物，荧光成像仪事后扫描，稳定、可 strip/reprobe；bioluminescence/autoradiography/immunogold 仅存在注记，本批不拆。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| Colorimetric 底物加样 + 肉眼监测终止 | 方法内显色分支（colorimetric）+ `op.incubation`（定时保持） | 待显色膜 → 沉淀膜（终点） | 方法绑定：底物按表（4CN/DAB/Opti-4CN/BCIP-NBT）；终止点操作者判断 | S4 Detection colorimetric 节 + 灵敏度表 |
| Chemiluminescent 底物加样（luminol/peroxide 或 CDP-Star）+ CCD/film 捕获（例 5 min 底物 + 5 s 成像） | 方法内显色分支（chemi）+ 原始图像采集（chemi-clock） | 待显色膜 → 发光膜 + 原始图像 | 方法绑定：底物按表；曝光 5 s–5 min 滴定；substrate→capture 延迟必记 | S4 Detection chemi 节 + 表 + WesternC 图注 |
| Fluorescent 标记抗体孵育（§4 已记）后直接荧光采集，无底物步 | 原始荧光图像采集（`FLUO_IMAGE` WB 模态绑定） | 已探针膜 → 原始多通道图像 | 方法绑定：激发/发射/滤块/相机按仪器；通道/曝光逐帧记 | S4 Detection fluorescence 节（optics 四要素 + multiplex 图） |
| Chemifluorescence 酶促荧光产物 + 荧光扫描 | 邻接分支（本批只记存在，不拆执行） | — | 方法绑定；需独立 protocol 才可执行 | S4 Other Detection Methods（chemifluorescence 段） |
| 同一 blot 跨模式比较/定量 | （分析） | 图像 → 派生 | out-of-scope；非操作（模式 measurand/灵敏度/时间窗不可比） | S4 三节 + 灵敏度表（本批不定义归一化） |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/洗涤（探针链） | 多家族 | shared（见 §4） | S4 WB Method 3–7 |
| 原始荧光图像采集 | 成像多家族（LIVE/DEAD、IF、WB fluoro） | shared（`FLUO_IMAGE` 模态参数化） | S4 fluorescence 节；只到原始图像，判读/定量另立 |
| 发光原始采集（chemi-clock） | 免疫/发光读数家族 | method-local 缺口（G-2，见 §8） | 单家族证据（本批 WB chemi）；不并入 `ABS_READ`，不硬套荧光采集 |
| 比色终点目视 | 仅 WB colorimetric | method-local（显色分支内完成检查） | 单方法；G1 不过 |
| densitometry/归一化/灵敏度换算 | —（分析） | 非操作（out-of-scope） | 三模式量程/稳定性不同（fluoro 10× chemi；HRP 褪色 vs AP 不褪） |

## 6. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 包被/封闭/样品/检测/偶联/底物/stop 加样（ELISA 三页） | `op.liquid_transfer` | 参数化复用；绑定体积/浓度/缓冲液/板色（§§2–3） |
| 包被/封闭/结合/偶联/显色保持（ELISA + WB block/探针） | `op.incubation` | 参数化复用；绑定时长/温度/振荡/避光；只断言 elapsed/偏差 |
| 板洗（1×/5×/3×，tap dry）/ 膜洗（rins/3×10/终洗三段） | `op.washing` | 参数化复用；cycles/缓冲液/soak 方法绑定；脱落场景 lossy 显式 |
| TMB/PNPP 终止板读数 | `ABS_READ` | 参数化复用；绑定 λ（450/405）+ 读数窗（≤30 min）；派生另立 |
| QuantaBlu/发光板读数 | luminescence-raw / fluorescence-raw | 按分支绑定（RLU 1–5 min / RFU 325-420）；不并入 `ABS_READ` |
| WB 荧光/多通道采集 | `FLUO_IMAGE`（模态参数） | 参数化复用；只到原始图像；通道/曝光逐帧记 |
| WB chemi 采集 | 缺口（method-local） | 单家族证据；不建公共模块（见 §8 G-2） |
| Ponceau 转膜质检 | method-local（`WB-TRANSFER-CHECK`） | 单方法 QC；不晋升 |
| 抗原直接包被 / 竞争预孵 / 二抗放大分支 | method-local 条件分支 | 留 ELISA 方法模板内；不合并、不晋升 |
| 标准曲线拟合（log-log/4-parameter）、竞争 cutoff、densitometry/归一化、跨模式比较 | ToolAction / out-of-scope | 一律非操作；无拟合不作浓度/表达声明 |

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | ELISA-SW colorimetric (S1/S2) | ELISA-SW chemi (S1) | ELISA-SW fluoro (S1) | ELISA-IND (S3) | ELISA-COMP (S3) | WB 程序 (S4) | WB 检测三模式 (S4) | 备注 |
|---|---|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 包被/封闭/样品/检测/偶联/TMB/stop | ● 同左（偶联至 1:20,000） | ● 包被 50–100 µL/偶联 50–100 µL/QuantaBlu 9:1 | ● 包被/封闭/一抗/二抗/底物 | ● 包被/封闭/50+50 混合/二抗/底物 | ● 封闭/一抗/二抗/底物 | ● 底物加样（模式分支） | 最高复用；全参数化 |
| `op.incubation` | ● 包被/封闭/结合/偶联/显色 | ● 同左（1 min 底物） | ● 同左（1.5–90 min） | ● 包被/封闭/90+90 min | ● 预孵 1 h + 板上 1 h + 二抗 1 h | ● block/一抗/二抗 | ● 显色保持（模式时钟） | 只断言 elapsed/偏差 |
| `op.washing` | ● 1× + 5×（S2 3× 冲突保留） | ● 同左 | ● 同左 | ● 3× 全程 | ● 3× 全程 | ● rins/3×10/终洗三段 | —（探针链已记） | cycles/缓冲液方法绑定 |
| `ABS_READ` | ● 450 nm ≤30 min | — | —（PNPP 405 nm 备选） | ● 适当 λ 30 min 内 | ● 适当 λ 30 min 内 | — | — | 原始 OD 止 |
| 发光/荧光原始读数 | — | ● RLU 1–5 min | ● RFU 325/420 | — | — | — | ● chemi-clock / fluoro 存档 | 物理/时钟不同，不合并 |
| `FLUO_IMAGE`（WB 模态） | — | — | — | — | — | — | ● 多通道原始采集 | 只到原始图像 |
| method-local（包被变体/竞争/质检/模式分支） | ○ capture vs direct | ○ | ○（直标 2–4 µg/mL 备选） | ● 直接包被 + 二抗放大 | ● 预孵竞争（递减） | ● Ponceau QC | ● 三模式分支 | 各留方法内；不合并 |
| ToolAction/分析 | ● 曲线拟合 | ● 曲线拟合 | ● 曲线拟合 | ● 曲线拟合 | ● 方向判读 + 拟合 | ● densitometry（域外） | ● 跨模式不可比 | 全部非操作 |

●=主路径使用；○=方法内备选分支；—=本方法无此步。

方向警示：sandwich/indirect 递增 vs competitive 递减——同一 OD 值在不同变体下含义相反，不得共用解释。读数警示：TMB-OD（≤30 min 终点）vs RLU（1–5 min 衰减时钟）vs RFU（稳定）vs 条带沉淀（终点）vs chemi 光（耗尽即止）vs 荧光（存档数周）——六者相互不可换算。

## 8. 候选与四问判定（本批结论：零新公共模块）

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用（≥2 独立家族） | 结论 |
|---|---|---|---|---|---|
| 抗原直接包被 | ✓ | ✓ | ✓ | ✗（仅 ELISA 内分支） | method-local，不新增 |
| 竞争预孵混合 | ✓ | ✓ | ✓ | ✗（仅 competitive） | method-local 条件分支，不新增 |
| 二抗-酶放大（vs biotin–streptavidin） | ✓ | ✓ | ✓ | ✗（effect/增益不稳定，审计 #4.3 类比） | method-local，不建通用 "amplify" |
| Ponceau 转膜质检 | ✓ | ✓ | ✓（脱色至净） | ✗（仅 WB） | method-local，不新增 |
| WB chemi 原始采集 | ✓ | ✓ | ✓（曝光/延迟记） | ✗（本批仅 WB 一家族；发光板读数 RLU 另有家族但仪器/时钟不同，需独立记录） | 缺口 G-2，不新增、不硬套 |
| 通用 "develop"/"probe"/"block" | ✓ | ✗（底物/抗体/膜效应异质） | △ | ✗（G2/G4 不过） | 不建通用模块 |

结论：本批无候选同时通过“四问 + 公共晋升（G1–G4 + ≥2 独立方法家族）”双门；不新增任何公共模块。

## 9. 缺口清单（不能表达/UNKNOWN；分类按 record-standard §4）

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | ELISA：同板标准系列（duplicate/triplicate）+ blank + 阴性/阳性 + vehicle/matrix；indirect 需 secondary-only 推理；competitive 需 labelled-alone 孔定向；WB：marker/WesternC + no-primary + loading/total-protein + 阳性/阴性裂解液 + multiplex 单色参照。任一缺即无意义 |
| G-2 | 发光原始采集公共证据不足 | `source_missing`（单家族） | WB chemi-clock（5 s–5 min 曝光、工作液 24 h RT、信号 6–8 h/24 h）本批仅 WB 一家族；不得并入 `ABS_READ` 或硬套 `FLUO_IMAGE`；需独立发光记录补第二家族证据 |
| G-3 | 派生公式归属 | ToolAction，非操作 | log-log/4-parameter 拟合、blank 扣除、竞争 cutoff、densitometry/loading 归一化、跨模式比较——全部下游；无拟合不作浓度/表达声明 |
| G-4 | 洗涤/包被值冲突 | 方法绑定（冲突保留） | S1 5× vs S2/S3 3×；S1 block 300 µL vs S2/S3 200 µL；S1 stop 100 µL 0.16 M vs S2 50 µL 0.2 M；S1 duplicate vs S2 triplicate；fluoro 包被 5–10 µg/mL + 50–100 µL vs 通用 1–10 µg/mL + 100 µL——各自守其 SOP，不平均 |
| G-5 | 系别/批依赖值 | `method_dependent` + `configuration_dependent` | 抗体起点浓度（稀释表各源不同，需逐对优化）；streptavidin-HRP exact 稀释（chemi 1:5,000–1:20,000 经验滴定）；labelled-secondary exact 增益 UNKNOWN；phospho 封闭按 datasheet；AP 精确动力学（PNPP 15–30 min 外）UNKNOWN |
| G-6 | 力学/手法量 | `source_missing` | 洗板 aspirate 力度/洗头不触底、tap dry 力度、sealer 贴合、膜 agitation 强度、气泡处理（fluoro 离心/挑泡）——原文定性，归 SOP 操作者判断 |
| G-7 | 固定后不可逆/终点约束 | 跃迁约束 | 终止后读数窗硬时钟（TMB ≤30 min；chemi 1–5 min；S1 60-min 稳定性注记从严取 30 min）；HRP/4CN 见光褪色 vs AP/BCIP-NBT 不褪；azide 禁用于 HRP；AP 遇酸失活；chemi 底物耗尽即止（fresh substrate 再激活另立程序）；LIVE 类固定禁用不属本批 |
| G-8 | 时间锚点 | 锚点缺口 | ELISA `t₀`=样品加入（包被/封闭为前置保持）；显色 `per_assay` + 读数秒/分级；WB `t₀`=一抗加入（转膜/Ponceau/block 为前置）；chemi substrate→capture 延迟必记；孵育不断言结合/生长完成 |
| G-9 | 范围外（误建模块风险） | `not_applicable` | 样品制备/裂解、SDS-PAGE/转膜优化、strip/reprobe 程序、ELISPOT/in-cell/Luminex、bioluminescence/autoradiography/immunogold 执行程序、图像分割/统计管线——本批一律不记录、不建模 |

## 10. 核查与状态

- 已验证（直接读，2026-11-05，§1 定位节）：S1 三程序 + 稀释表 + AP/direct/labelled-secondary 注记；S2 Method 1–7 + Reagents；S3 indirect/competitive 两页 Method 全文；S4 WB Method 1–8 + Reagents + Detection 三节/两表/曝光与稳定性注记。单人 `agent-verified`，仍需第二人逐字段复核；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：BioLegend sandwich 页（403）、CSH immunoblot 全文、任何 per-lot 片（抗体/标准/底物动力学/曝光序列）、仪器操作手册、实验室 SOP 改编、已跑数据派生应用。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器规格分离（§§2–5 表 1 vs 读数/仪器绑定行）；样本跃迁有端口（板/膜/原始矩阵）；时间锚 `t₀` 显式；部分列表标部分（板色/底物选项）；plan/actual/raw/derived 分离（原始 OD/RLU/RFU/图像保留，派生下游）；无 JSON；UNKNOWN 永不填零。
