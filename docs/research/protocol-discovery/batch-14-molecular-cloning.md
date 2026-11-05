# Batch 14 — Molecular Cloning Chain: Plasmid Miniprep / Restriction Digest / Ligation / Transformation / Gel Extraction-Cleanup

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 分子克隆链 5 个湿实验 protocol 组:质粒小提 (QIAprep spin 碱裂解柱式)、限制性酶切 (Addgene 通用程序)、T4 连接 (Addgene 标准 insert+vector)、化学感受态热激转化 (Addgene)、凝胶回收/纯化 (QIAquick 硅胶柱,Davidson + Kerr 双来源)。止于洗脱 DNA / 酶切反应液 / 连接产物 / 过夜平板单菌落 / 回收 DNA;下游测序验证/酶切鉴定电泳判读/浓度换算/CFU 拟合一律 out-of-scope 或 ToolAction,不建操作。
- 规则: 五链分立记录,不合并;制备与酶切/连接/转化/回收互不合并;磷酸酶/平端补齐/电转仅记存在,不拆执行;现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;不新增公共模块除非四问 + 晋升四门全过,domain-shared 允许但本批无晋升;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用洗脱体积、通用酶量、通用连接比、通用热激时长、通用溶解温度;所有温度/时间/体积/浓度/离心条件均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| S1 | QIAGEN QIAprep Miniprep Handbook June 2005, spin protocol pp.22–23 + Important Notes pp.15–17 + elution notes p.17 | 厂商手册 | 1–5 mL LB 过夜高拷贝 ≤20 µg;250/250/350 µL 体系;13,000 rpm 结合/洗/洗脱 | 直接读:PDF 全文转写;离心机转 vacuum/5 mL 管变体仅记存在,不作第二证据;large/low-copy 附录 p.44 仅记存在 |
| S2 | Addgene Protocol — Restriction Digest of Plasmid DNA (Last Upload 2016-10-11) | 实验室 protocol | 选酶/缓冲/配制/37 °C 1 h/热失活或纯化/电泳 | 直接读:页面全文转写;例示 1 µg DNA + 1 µL 酶 + 3 µL 10×Buffer (+BSA) 至 30 µL,10–50 µL 总体积窗 |
| S3 | Addgene Protocol — DNA Ligation (Standard Insert + Vector) | 实验室 protocol | 3:1 摩尔比/~100 ng 总量/10 µL 体系/RT 2 h 或 16 °C overnight | 直接读:页面全文转写;ATP 分装注记 + 对照表 + 比例优化节全文 |
| S4 | Addgene Protocol — Bacterial Transformation (chemically competent, heat shock; Last Update 2017-11-13) | 实验室 protocol | 融化/混合/冰/42 °C 热激/复苏/铺板/过夜 | 直接读:页面全文转写;Equipment + Reagents + Procedure 1–9 + Tips/FAQ 全文;电转/自制感受态仅记存在 |
| S5a | Davidson College Biology — Gel Purification of DNA using Qiaquick Spin System (QIAquick) | 大学 lab protocol | 切胶/称重/3 vol QG/50 °C 10 min/异丙醇分支/结合/洗/洗脱 | 直接读:页面全文转写;步骤 1–14 全文;低百分胶/低 UV 注记为例 |
| S5b | Kerr Lab UW — QIA Quick Gel Purification (Eshelman 2012-09-13, revised from Quick-Start) | 大学 lab protocol | 同 QIAquick 体系;55 °C ~10 min/NaOAc 变色/复离/热 MQ 洗脱 | 直接读:页面全文转写;S5a/S5b 差异保留,不合并为通用值 |
| — | NEB Restriction Digest Protocol + T4 DNA Ligase M0202 页;NEB Cloning Guide/酶兼容表 | 厂商页 | 仅立项定位 | `search-summary-only`,非证据;NEB 页 403 未直读,凡数值以 S2/S3 直接读为准 |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 取样/分样 sampling atomic (谱系,守恒缺口);R3 `op.incubation` (elapsed/条件 only,不断言裂解/酶切/连接/摄入完成);R4 `op.washing` (保留语义) + lossy 变体;R5 `op.centrifugation`;R6 连续稀释=Composite;R7 涂布/划线接种 narrow/variant + 菌落计数 readout;R8 `absorbance_measurement` (raw OD only,本批仅监测语境);R9 `FLUO_IMAGE` (凝胶成像仅下游入口,本批不拆);审计 `#24` 解离、`#26` 固定/通透/封闭、`#27` 定量不单原子 (blank/单位/化学不同,作方法绑定读数)、`#33` 发光读数与本批无关。电泳分离为 C01 deferred candidate,本批只作下游入口。B10 热激转化为第二证据方向 (S4 为主,S2/S3 转化为连接产物去向)。

检索摘要(`search-summary-only`,非证据):NEB 酶量/时间/热失活摘要、QIAGEN 柱结合/洗脱摘要、protocols.io QIAprep/QIAquick 条目仅用于立项定位。

---

## 2. P-PREP — QIAprep Spin 质粒小提 (S1)

绑定摘要:1–5 mL LB 过夜 (12–16 h,37 °C 剧烈振荡;>16 h 不推荐);>8,000 rpm 3 min 收菌 (或 15 mL 管 5,400×g 10 min 4 °C),倒净残液。250 µL P1 (RNase A 已加,可选 LyseBlue 1:1,000) 重悬至无菌块 → 250 µL P2 温和颠倒 4–6× (禁涡旋,≤5 min,至黏稠微清) → 350 µL N3 立即充分颠倒 4–6× (≥5 mL 可至 10×,至浑浊;LyseBlue 褪至无色) → 13,000 rpm (~17,900×g) 10 min,紧实白沉淀 → 上清转柱 → 30–60 s 离心弃穿流 → 可选 0.5 mL PB 洗 30–60 s (endA+ 必需,XL-1 Blue/DH5α 可省) → 0.75 mL PE 洗 30–60 s → 弃穿流后再空甩 1 min (残留乙醇抑制下游) → 洁净 1.5 mL 管,50 µL EB (10 mM Tris·Cl pH 8.5) 或水 (pH 7.0–8.5) 滴膜中央,静置 1 min,离心 1 min;平均得液 ~48 µL。TE 可洗脱但 EDTA 抑制酶反应;水洗脱 −20 °C 久存降解风险。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 1–5 mL 过夜培养;收菌 >8,000 rpm 3 min RT,倒净残液 | `op.centrifugation` 参数化 + 方法内培养前置 | 培养物 → 菌沉淀 | 方法绑定:1–5 mL;12–16 h;转速/管型分支;残液去除为完成检查 | S1 pp.15,22 steps 前 |
| 250 µL P1 重悬至无菌块 (RNase A 已加;LyseBlue 可选) | `op.liquid_transfer` 参数化 + 方法内重悬备注 | 菌沉淀 + P1 → 重悬液 | 方法绑定:250 µL;涡旋/吹打至无块;LyseBlue 1:1,000 可选 | S1 p.22 step 1 + p.14/16 |
| 250 µL P2,颠倒 4–6×,≤5 min,禁涡旋 | `op.liquid_transfer` 参数化 + method-local (`ALK-LYSIS`,碱裂解) | 重悬液 → 裂解液 | 方法绑定:250 µL;次数/时长上限;禁涡旋为防基因组剪切约束 | S1 p.22 step 2 |
| 350 µL N3 立即充分颠倒 4–6×,至浑浊/无色 | `op.liquid_transfer` 参数化 + method-local (`ALK-NEUT`,中和沉淀) | 裂解液 → 中和混合物 + 沉淀 | 方法绑定:350 µL;立即混匀防局部沉淀;大体积至 10× | S1 p.22 step 3 |
| 13,000 rpm 10 min;上清转柱 (倾倒/移液) | `op.centrifugation` + `op.liquid_transfer` (转柱) 参数化 | 中和混合物 → 澄清上清 (柱上) + 白沉淀 | 方法绑定:13,000 rpm 10 min;exact 转子/RCF 换算 UNKNOWN | S1 p.22 steps 4–5 |
| 30–60 s 离心弃穿流 (结合) | 方法内硅胶膜结合分支 (`PREP-BIND`) + 离心参数化 | 澄清上清 → 已结合柱 + 废穿流 | 方法绑定:30–60 s;结合 effect 留方法内,不并入普通洗涤 | S1 p.23 step 6 |
| 可选 0.5 mL PB 洗 30–60 s (endA+ 必需) | 方法内柱洗分支 (`PREP-WASH-PB`) | 已结合柱 → PB 已洗柱 | 方法绑定:0.5 mL;菌株依赖分支 | S1 p.23 step 7 |
| 0.75 mL PE 洗 30–60 s;弃穿流后空甩 1 min | 方法内柱洗分支 (`PREP-WASH-PE`) + 离心参数化 | 已洗柱 → 干燥柱 | 方法绑定:0.75 mL;空甩阻断性完成检查 (残留乙醇抑制下游) | S1 p.23 steps 8–9 |
| 50 µL EB/水滴膜中央,静置 1 min,离心 1 min | `op.liquid_transfer` + `op.incubation` (静置) + `op.centrifugation` 参数化 + method-local (`PREP-ELUTE`,浓度/总量权衡) | 干燥柱 → 洗脱液 + 废柱 | 方法绑定:50 µL;pH 7.0–8.5 窗;100 µL 增产/减浓分支为方法注记 | S1 p.23 step 10 + p.13/17 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (P1/P2/N3/转柱/洗液/EB) | 多家族 | 复用 R1 | 体积全方法绑定 |
| 离心 (收菌/澄清/结合/洗/洗脱) | 多家族 | 复用 R5 | 8,000 rpm/13,000 rpm/时长绑定;转子 UNKNOWN |
| 孵育/保持 (洗脱静置 1 min) | 多家族 | 复用 R3 elapsed-only | 只断言 elapsed,不推断洗脱完全 |
| 碱裂解–中和–硅胶膜结合–洗–洗脱整体 | 仅质粒柱提家族 | method-local,不新增 | G1 不过 (本批仅一家族);G2 不过 (与基因组柱提/凝胶柱 effect 不兼容,见 §7) |
| 通用 "DNA extraction/purification" | 表面多见 | 不建模块 | 柱提 vs 有机抽提 vs 磁珠 effect/端口异质 |

---

## 3. P-DIG — 限制性酶切 (S2)

绑定摘要:按序列选酶 (Sequence Analyzer);按酶说明书定缓冲,双酶切查兼容表。1.5 mL 管:DNA + 酶 + Buffer (+BSA 如推荐) + dH2O 至总体积;例示 1 µg DNA + 各 1 µL 酶 + 3 µL 10×Buffer (+3 µL 10×BSA) 至 30 µL,总体积窗 10–50 µL;诊断性 ~500 ng,克隆用 1 µg;单位定义 1 U = 50 µL 体系 1 h 切 1 µg DNA,实际多加;0.2–0.5 µL 加样下限多为过量。轻弹/吹打混匀。按酶温 (常 37 °C) 保温 1 h;应用分支 45 min–过夜,诊断 1–2 h,>1 µg 克隆 ≥4 h。酶须冰上保存,离 freezer 立即回冰。不进凝胶纯化而换液时按说明书热失活 (如 70 °C 15 min) 或柱纯化;可视化走凝胶电泳 (下游入口,本批不拆)。Master Mix (除 DNA 外预混) 为效率备注。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 选酶 (序列分析);定缓冲 (双酶切查兼容表) | 方法设计/对照,非操作 | 质粒序列 → 酶/缓冲方案 | 方法绑定:酶按说明书;exact 兼容组合 UNKNOWN | S2 steps 1–2 |
| 配制:DNA + 酶 + Buffer (+BSA) + dH2O 至 10–50 µL (例 1 µg/1 µL/3 µL/至 30 µL) | `op.liquid_transfer` 参数化 | DNA + 试剂 → 酶切反应液 | 方法绑定:例示体积;诊断 ~500 ng vs 克隆 1 µg 分支;exact 酶单位按 DNA 量算 | S2 step 3 + Pro-Tips |
| 轻弹/吹打混匀;酶离 freezer 立即置冰 | 方法内手法备注 + `op.incubation` 前置条件 | 反应液 → 已混匀反应液 | 方法绑定:禁剧烈涡旋为隐含手法;exact 混匀次数 UNKNOWN | S2 step 4 + Tips |
| 按酶温保温 (常 37 °C) 1 h (分支 45 min–过夜;克隆大体系 ≥4 h) | `op.incubation` 参数化 + method-local (`DIG-CUT`,酶切) | 反应液 → 已酶切液 | 方法绑定:温度/时长按酶;只断言 elapsed,不推断切透 | S2 step 5 + Pro-Tip |
| 按需热失活 (如 70 °C 15 min) 或柱纯化后换酶/下游 | 方法内分支 (失活 vs 纯化二选一) | 已酶切液 → 失活液/纯化 DNA | 方法绑定:按酶说明书;exact 失活温度按酶表,本批 UNKNOWN | S2 step 5 注记 |
| 凝胶电泳可视化 | 下游入口,本批不拆 (B04/C01 边界) | 已酶切液 → 条带判读 (out-of-scope) | 方法绑定 | S2 step 6 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育 (配制/保温) | 多家族 | 复用 R1/R3 | 体积/温度/时长全绑定 |
| 限制性切割 (序列特异性磷酸二酯键断裂) | 仅酶切家族 | method-local (`DIG-CUT`),不新增 | G1 不过 (单家族);与非特异核酸酶/PCR 扩增 effect 不兼容 |
| 通用 "digest" | 表面多见 | 不建模块 | 限制酶 vs 蛋白酶 vs 组织解离酶机理/底物异质 (见 §7) |
| Master Mix 预混 | 效率备注 | 非操作 | 一致性手法,不建模块 |

---

## 4. P-LIG — T4 DNA 连接 (S3)

绑定摘要:先算量:常用 3 insert:1 vector 摩尔比 (~100 ng 总 DNA;DNA 末端摩尔数按长度折算,calculator 辅助;低浓度可放大体系,buffer 同比)。10 µL 例:vector + insert + Ligase Buffer (10× 按 1 µL/10 µL,5× 按 2 µL/10 µL) + 0.5–1 µL T4 DNA Ligase + H2O 至 10 µL;buffer 含 ATP,冻融降解,建议分装 −20 °C 单次 thaw。RT 2 h 或 16 °C overnight (按厂商);高浓度 ligase 常 RT 5 min 即够;退火 oligo 疑难可试 37 °C。必设对照:vector-alone+ligase (主对照,查酶切彻底/磷酸酶效果;应少菌落);另 uncut vector−ligase (感受态/抗性)、cut vector−ligase (未切本底)、insert/water+ligase (污染)。连后直接转化 (路由 §5)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 算量:3:1 摩尔比,~100 ng 总量,按长度折算末端摩尔数 | ToolAction (摩尔换算) + 方法设计 | 浓度/长度 → 加样量方案 | 方法绑定:calculator 品种 UNKNOWN;exact ng 按片段,本批 UNKNOWN | S3 Background + step 前 |
| 配制 10 µL:vector + insert + buffer + 0.5–1 µL ligase + H2O | `op.liquid_transfer` 参数化 | 载体/插入/buffer/酶 → 连接反应液 | 方法绑定:buffer/酶体积;放大体系同比分支 | S3 step 1 + Notes |
| 保温:RT 2 h 或 16 °C overnight (高浓酶 5 min / oligo 37 °C 分支) | `op.incubation` 参数化 + method-local (`LIG-JOIN`,磷酸二酯键形成) | 连接反应液 → 连接产物 | 方法绑定:时长/温度分支;只 elapsed,不推断连接效率 | S3 step 2 + Note |
| 对照矩阵 (vector-alone+ligase 主对照 + 3 辅助对照) | 方法对照,非操作 | — | 方法绑定:缺主对照即无意义 | S3 Tips 对照表 |
| 连后转化 | 路由 §5 P-TF | 连接产物 → 转化混合物 | 方法绑定 | S3 step 3 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育 (配制/保温) | 多家族 | 复用 R1/R3 | 体积/时长/温度绑定 |
| 连接 (T4 phosphodiester 形成) | 仅连接家族 | method-local (`LIG-JOIN`),不新增 | G1 不过;与酶切/聚合 effect 方向相反,不合并 (见 §7) |
| 通用 "clone/ligate" | 表面多见 | 不建模块 | sticky vs blunt/单酶 CIP/SAP/末端补齐分支异质 |
| 摩尔比换算/对照判读 | ToolAction/方法 QC | 非操作 | 计算与判读另立 |

---

## 5. P-TF — 化学感受态热激转化 (S4)

绑定摘要:−80 °C 感受态冰上融 20–30 min;含抗生素 LB 平板 4 °C 取出回温至 RT (可选 37 °C 预热)。1–5 µL DNA (常 10 pg–100 ng) + 20–50 µL 感受态,指弹轻混 (禁涡旋语境)。冰 20–30 min → 42 °C 水浴热激 30–60 s (管底 1/2–2/3 入水;45 s 常优,按细胞品种) → 冰 2 min → 250–1,000 µL LB/SOC (无抗生素) 37 °C 摇 45 min 复苏 (Amp 可短/省,其他抗生素建议 ≥20–30 min;抗性蛋白表达窗口)。分两板:50 µL + 剩余;液多可温和离心浓缩重悬防平板积液。37 °C 过夜。连接产物效率约低 10×;高浓度感受态 DNA 宜稀释 (1:5/1:10),少即是多;大质粒 (>10 kb)/BAC 改电转 (存在注记,不拆)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 冰上融 20–30 min;平板回温 RT (可选 37 °C) | `op.incubation` 参数化 (定时冷/温保持) | 冻存感受态 + 冷平板 → 待用感受态/平板 | 方法绑定:20–30 min;exact 细胞品种按说明书 | S4 steps 1–2 |
| 1–5 µL DNA (10 pg–100 ng) + 20–50 µL 感受态,指弹轻混 | `op.liquid_transfer` 参数化 + 方法内轻混备注 | DNA + 感受态 → 转化混合物 | 方法绑定:体积/量窗;连接产物稀释分支 | S4 step 3 + TIP |
| 冰 20–30 min → 42 °C 30–60 s (45 s 常优) → 冰 2 min | `op.incubation` 参数化 + method-local (`TF-UPTAKE`,热激摄入) | 转化混合物 → 已热激混合物 | 方法绑定:三段时长/温度;只 elapsed,不推断摄入/效率 | S4 steps 4–6 |
| 250–1,000 µL LB/SOC (无抗) 37 °C 摇 45 min 复苏 | `op.liquid_transfer` + `op.incubation` 参数化 | 已热激混合物 → 已复苏菌液 | 方法绑定:体积窗/时长;Amp 短省分支 | S4 step 7 + Pro-Tip |
| 分两板 (50 µL + 剩余);液多温和离心浓缩;37 °C 过夜 | 涂布接种 (R7 variant) + `op.incubation` 参数化 | 已复苏菌液 → 过夜平板 (单菌落) | 方法绑定:分板规则;exact 离心 UNKNOWN;单菌落判读定性 | S4 steps 8–9 + Pro-Tips |
| 阳性对照质粒/抗生素匹配检查;无转化子排查 | 方法对照/QC,非操作 | — | 方法绑定 | S4 FAQ |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/离心 (混合/保温/复苏/浓缩) | 多家族 (B02/B10 划线/涂布/液体培养) | 复用 R1/R3/R5 | 体积/时长/温度绑定;浓缩离心值 UNKNOWN |
| 涂布接种/菌落 (分板/过夜) | 微生物家族 (B02/B10) | 复用 R7 variant + readout;本批新增转化后铺板证据 | 单菌落判读定性;CFU 换算另立 |
| 热激摄入 (膜通透摄入) | 仅化学转化家族 | method-local (`TF-UPTAKE`),不新增 | G1 不过 (单家族);与电转/转染/转导机理不同,不合并 (见 §7) |
| 通用 "transformation/transfection" | 表面多见 | 不建模块 | 化学热激 vs 电穿孔 vs 脂质体 vs 病毒 effect/设备异质 |

---

## 6. P-GEL — 凝胶回收/PCR-酶切纯化 (S5a/S5b)

绑定摘要 (S5a;S5b 差异括号注):跑胶拍照 (MW marker;尽量低百分胶) → UV 下剃刀切带,贴带切小,尽量减胶体积,低 UV 短时,护目/护肤 → 无色管称重,3 vol QG : 1 vol 胶 (100 mg ≈ 100 µL) → 50 °C 水浴 10 min (S5b:55 °C 烘箱 ~10 min) 至完全溶解,每 2–3 min 涡旋助溶;溶液须黄,橙/紫加 10 µL 3 M NaOAc pH 5.0 转黄 (S5b) → 异丙醇:S5a 仅 <500 bp 或 >4 kb 加 1 胶体积涡旋 30 s,否则跳过;S5b 加 ~1 胶体积 (~100 µL) 混匀 (差异保留,不调和) → QIAquick 柱置 2 mL 管,≤800 µL/次结合,满速 1 min (S5b:8,000 rpm 30 s;可回倒重离一次;低速短时减损注记) → 弃穿流 → 0.75 mL PE 洗,满速 1 min (S5b:静置 2–5 min 脱盐后 8,000 rpm 30 s) → 弃穿流后空甩 1 min 13,000 rpm (残留乙醇不弃穿流即去不净) → 洁净 1.5 mL 管,30 µL EB 滴膜中央静置 1 min 满速 1 min (S5b:30 µL 65 °C 热 MQ,静置 2–5 min,13,000 rpm 2 min;EB 盐抑制连接/测序故用水为 S5b 方法选择);−20 °C 存 (水洗脱无缓冲降解注记) → 取 1–2 µL NanoDrop 定量 (读数入口,换算另立)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 跑胶拍照 (marker;低百分胶);UV 切带贴带切小,低 UV 短时 | 下游入口 (B04 电泳) + 方法内切胶分支 (`GEL-EXCISE`) | 胶 (条带) → 胶块 | 方法绑定:减体积/低 UV 约束;exact 胶% 按分辨,UNKNOWN | S5a 1–2;S5b 1–2 |
| 称重;3 vol QG : 1 vol 胶 (100 mg ≈ 100 µL) | `op.liquid_transfer` 参数化 (缓冲配比语境) | 胶块 + QG → 待溶混合物 | 方法绑定:3:1;exact 称重皮重按天平 | S5a 3;S5b 3–4 |
| 50 °C 10 min (S5b 55 °C ~10 min) 至完全溶解,涡旋 2–3 min 助溶;黄色检查,橙/紫 +NaOAc 转黄 | 方法内溶胶分支 (`GEL-DISSOLVE`) + `op.incubation` 参数化 | 待溶混合物 → 已溶胶液 | 方法绑定:温度/时长/涡旋节律;颜色为完成检查;NaOAc 10 µL 仅 S5b | S5a 4–5;S5b 4 |
| 异丙醇:S5a 条件分支 (<500 bp/>4 kb 加 1 vol 涡旋 30 s) vs S5b 常加 ~1 vol | 方法内分支差异,各守其 SOP | 已溶胶液 → 待结合液 | 方法绑定:分支条件不同,不调和;exact 体积按胶重 | S5a 6;S5b 5 |
| 上柱结合 ≤800 µL/次,满速 1 min (S5b 8,000 rpm 30 s,可回倒重离) | 方法内硅胶结合分支 (`GEL-BIND`) + 离心参数化 | 待结合液 → 已结合柱 + 废穿流 | 方法绑定:体积上限/转速/时长;低速减损为 S5b 注记 | S5a 7–9;S5b 6–9 |
| 0.75 mL PE 洗 (S5b 静置 2–5 min),离心;弃穿流后空甩 1 min 13,000 rpm | 方法内柱洗分支 (`GEL-WASH`) + 离心参数化 | 已结合柱 → 干燥柱 | 方法绑定:0.75 mL;静置有无按来源;空甩阻断性检查 | S5a 10–11;S5b 10–12 |
| 30 µL EB 滴膜中央静置 1 min 离心 1 min (S5b 30 µL 65 °C 热 MQ 静置 2–5 min 13,000 rpm 2 min) | `op.liquid_transfer` + `op.incubation` + `op.centrifugation` 参数化 + method-local (`GEL-ELUTE`) | 干燥柱 → 回收 DNA + 废柱 | 方法绑定:体积/温度/时长按来源;水 vs EB 为方法选择 (盐抑制注记) | S5a 12–13;S5b 13 |
| 1–2 µL NanoDrop 定量 | 方法模板内读数绑定 + ToolAction (浓度换算) | 回收 DNA → 原始吸光值 + 派生浓度 | 方法绑定;blank/线性按仪器,本批 UNKNOWN | S5a 14 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/离心/孵育 (QG/异丙醇/上柱/洗/洗脱) | 多家族 | 复用 R1/R5/R3 | 体积/转速/时长绑定;RCF 换算 UNKNOWN |
| 硅胶膜结合–洗–洗脱 (凝胶体系) | 凝胶回收一家族 (+质粒柱提为另一化学) | method-local,不新增 | G1 不过 (单家族);G2 不过:QG 溶胶体系 vs P1/P2/N3 碱裂解体系 effect 不兼容,不与 §2 合并 (见 §7) |
| 通用 "cleanup" | 表面多见 | 不建模块 | 凝胶溶解 vs 酶反应直接纯化 (加 QG/异丙醇变体) vs 乙醇沉淀异质 |
| 颜色/干燥/体积完成检查 | 方法 QC | 非操作 | 黄色/无乙醇残留/得液量为阻断检查 |

---

## 7. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| P1/P2/N3/QG/PE/EB/酶/buffer/感受态/DNA 加样 | `op.liquid_transfer` | 参数化复用;绑定体积/配比/edish (§§2–6) |
| 收菌/澄清/结合/洗/洗脱/浓缩离心 | `op.centrifugation` | 参数化复用;绑定 RCF/时长 (8,000 rpm 3 min;13,000 rpm 10 min/1 min;30–60 s 结合/洗) |
| 裂解/酶切/连接/热激/复苏/溶胶/洗脱静置保持 | `op.incubation` | 参数化复用;绑定时长/温度;只 elapsed,不推断裂解/切透/连接/摄入/溶解完成 |
| 转化后分板涂布/过夜平板菌落 | 涂布接种 variant + 菌落计数 readout | 参数化复用;单菌落判读定性;CFU 换算另立 |
| 碱裂解/中和/膜结合/酶切/连接/热激摄入/溶胶/柱洗/洗脱 | method-local (`ALK-LYSIS`/`ALK-NEUT`/`PREP-BIND`/`PREP-WASH`/`PREP-ELUTE`/`DIG-CUT`/`LIG-JOIN`/`TF-UPTAKE`/`GEL-EXCISE`/`GEL-DISSOLVE`/`GEL-BIND`/`GEL-WASH`/`GEL-ELUTE`) | 各留方法模板内;不合并、不晋升 (G1/G2 不过) |
| 摩尔比/浓度/得率/效率/纯度、NanoDrop/A260 换算、条带判读、对照判读 | ToolAction / 下游 out-of-scope | 一律非操作;无曲线/无对照不作浓度/效率声明 |

## 8. 跨方法公共操作覆盖矩阵

| 公共操作 | P-PREP 质粒小提 (S1) | P-DIG 酶切 (S2) | P-LIG 连接 (S3) | P-TF 转化 (S4) | P-GEL 回收 (S5a/b) | 备注 |
|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● P1/P2/N3/洗液/EB | ● DNA/酶/buffer/BSA | ● vector/insert/buffer/ligase | ● DNA/感受态/复苏液 | ● QG/异丙醇/PE/EB | 最高复用;全参数化 |
| `op.incubation` | ● 洗脱静置 1 min | ● 37 °C 1 h (+分支) | ● RT 2 h/16 °C overnight | ● 冰/42 °C/冰/37 °C 45 min | ● 50/55 °C 溶胶 + 洗脱静置 | 只 elapsed/偏差 |
| `op.centrifugation` | ● 收菌/澄清/结合/洗/洗脱 | — | — | ○ 液多浓缩 | ● 结合/洗/空甩/洗脱 | RCF/转子方法绑定 |
| 涂布/菌落 | — | — | — | ● 分两板/过夜 | — | B02/B10 证据延续 |
| sampling | ● 上清转柱/穿流弃留 | — | — | — | ● 结合分次/穿流弃留 | 守恒缺口沿用 |
| method-local (裂解/切/连/摄入/溶胶/柱化学) | ● 碱裂解/结合/洗/洗脱 | ● 序列特异切割 | ● 末端连接 | ● 热激摄入 | ● 切胶/溶胶/结合/洗/洗脱 | 各留方法内;拒绝合并 |
| ToolAction/下游 | ● 得率/纯度/A260 | ● Master Mix/电泳判读 | ● 摩尔比/对照判读 | ● 效率/电转分支 | ● NanoDrop/得率 | 全部非操作 |

●=主路径使用;○=条件分支;—=本方法无此步。

方向警示:碱裂解 (变性释放) vs 酶切 (序列特异断裂) vs 连接 (末端共价接合)——同一 "DNA 操作" 词下方向相反,不得共用模块。捕获警示:质粒柱 (P1/P2/N3 碱裂解体系,PB 去核酸酶) vs 凝胶柱 (QG 溶胶体系,异丙醇分支,黄色检查)——同一 "bind-wash-elute" 词下化学/完成检查不同,不得共用解释。转化警示:化学热激 (42 °C 30–60 s + 复苏) vs 电转 (场强,存在注记)——机理/设备不同,不得共用模块。

## 9. 候选与四问判定 + 晋升审查 (本批结论:零新公共模块)

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用 (≥2 独立家族) | 结论 |
|---|---|---|---|---|---|
| 碱裂解–中和–硅胶结合–洗–洗脱 (质粒) | ✓ | ✓ | ✓ (穿流/得液/下游酶切可确认) | ✗ (仅质粒柱提一家族;凝胶柱化学不同) | method-local,不新增 |
| 限制性切割 | ✓ | ✓ | ✓ (电泳可确认) | ✗ (仅酶切一家族) | method-local,不新增 |
| T4 连接 | ✓ | ✓ | ✓ (转化菌落/对照可确认) | ✗ (仅连接一家族) | method-local,不新增 |
| 热激摄入 | ✓ | ✓ | ✓ (平板菌落可确认) | ✗ (仅化学转化一家族;B10 同为细菌转化,不算第二独立家族) | method-local,不新增 |
| 溶胶–结合–洗–洗脱 (凝胶) | ✓ | ✓ | ✓ (回收量/下游连接可确认) | ✗ (仅凝胶回收一家族) | method-local,不新增 |
| 通用 "digest/ligate/transform/purify" 合并 | ✓ | ✗ (底物/键/方向/设备异质) | △ | ✗ (G2/G4 不过) | **拒绝合并**;五链分立 (题设要求) |

晋升四门 (G1 复用广度 ≥2 独立方法,域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):本批无候选同时通过。五链不合并的独立理由:端口不同 (菌沉淀/反应液/末端/感受态/胶块)、样本 effect 不同 (变性释放/特异断裂/共价接合/膜摄入/基质溶解)、完成检查不同 (浑浊-白沉淀/电泳条带/对照菌落差/过夜单菌落/黄色-干燥-得液)。合并将隐藏化学与方向差异,违反 G2/G4。

结论:本批零新公共模块;对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer` (+裂解/酶切/连接/转化/溶胶五类绑定)、`op.incubation` (+静置/酶切/连接/热激/复苏/溶胶保持)、`op.centrifugation` (+收菌/澄清/结合/洗/洗脱/浓缩)、涂布接种/菌落计数 (+转化后分板证据)。

## 10. 缺口清单 (不能表达/UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | 连接:vector-alone+ligase 主对照 + 3 辅助对照;转化:阳性对照质粒 + 抗生素匹配;小提:下游酶切/测序参照;回收:marker/投入量参照。任一缺即无意义 |
| G-2 | exact 配方/量未取 | `source_missing` | 酶单位按 DNA 量折算 exact 值、连接 exact ng 按片段长度、转化 exact 感受态品种/效价、回收 exact 胶重/QG 倍数按实测、PB/PE exact 盐组成按 lot——本批 UNKNOWN,按说明书/datasheet |
| G-3 | 力学/手法量 | `source_missing` | 颠倒力度、指弹轻混 vs 涡旋禁令、切胶贴带余量、回倒重离手法、平板晾干程度——原文定性,归 SOP 操作者判断 |
| G-4 | 派生公式归属 | ToolAction,非操作 | 摩尔比/末端数折算、浓度/得率/效率换算、CFU、条带判读——全部下游;无对照不作效率声明 |
| G-5 | 系别/批依赖值 | `method_dependent` + `configuration_dependent` | 酶温/缓冲/热失活按酶表;连接时长按 ligase 浓度;热激 30–60 s 按感受态品种;溶胶 50 vs 55 °C、异丙醇条件 vs 常加按来源 SOP;洗脱水 vs EB 按下游抑制约束 |
| G-6 | 不可逆/终点约束 | 跃迁约束 | P2 ≤5 min 过裂解剪切基因组;N3 延迟致局部沉淀;PE 残留乙醇抑制酶/连接;水洗脱久存降解;UV 过曝损伤 DNA;酶离 freezer 久置失活;ATP 反复冻融失效 |
| G-7 | 时间锚点 | 锚点缺口 | 小提 `t₀`=P2 加入;酶切 `t₀`=保温开始;连接 `t₀`=ligase 加入;转化 `t₀`=热激开始 (复苏为后置);回收 `t₀`=QG 加入。孵育不断言裂解/切透/连接/摄入/溶解完成 |
| G-8 | 范围外 (误建模块风险) | `not_applicable` | 测序/序列分析、凝胶图像分割定量、生长曲线/药敏 (B10)、蛋白酶/组织解离酶 (B15)、通用 "clone/purify/transform" 模块——本批一律不建模 |
| G-9 | 损失/守恒未量化 | 合同缺口 (沿用) | 穿流弃留损失、结合容量超载 (≤800 µL/次,超量分次)、分板取样守恒、切胶 UV 损伤量——量级 UNKNOWN;lossy 显式,不塞进普通洗涤参数 |

## 11. 核查与状态

- 已验证 (直接读,2026-11-05,§1 定位节):S1 spin 250/250/350 µL + 颠倒/离心/洗/50 µL 洗脱三元组;LyseBlue/干燥/残留乙醇注记;S2 选酶/缓冲/例示配制/37 °C 1 h/热失活分支;S3 3:1/~100 ng/10 µL/RT 2 h 或 16 °C overnight + 对照表;S4 融化/混合/冰-热激-冰/复苏/分板/过夜全链;S5a/S5b 切胶/3 vol QG/溶胶温度/异丙醇分支差异/结合/洗/空甩/30 µL 洗脱对偶。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何 PDF 全文逐页之外的 lot 片 (酶活/感受态效价/QG 批)、仪器操作手册 (离心机转子/水浴均一性)、实验室 SOP 改编、电转执行程序、磷酸酶/平端补齐执行、已跑数据派生应用。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离 (§§2–6 表 1 vs 读数/仪器绑定行);样本跃迁有端口 (裂解液/反应液/产物/菌液/胶液/原始值);时间锚显式 (§10 G-7);部分列表标部分 (缓冲/酶/胶型选项);plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
