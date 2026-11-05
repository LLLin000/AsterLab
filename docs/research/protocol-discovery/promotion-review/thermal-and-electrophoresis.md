# Catalog Promotion Review v2 — ThermalProgram + Electrophoresis Separation

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`。第二人复核前不得晋升、不得进入 JSON/运行时。
- 目的: 对两个 domain-shared candidate 做窄边界跨方法语义对照,决定是否进入 Catalog、应缩小到什么范围。
- 规则: 至少 2 个独立方法;比较目的/端口/样本变化/完成检查/仪器要求/差异成本;不足则保留 candidate/deferred/method-local。不生成 JSON。
- 范围: 仅湿实验步骤与原始读数。临床、动物在体、数据处理、图像分析、统计、生物信息一律 out-of-scope。
- 边界禁令: 荧光采集不并入 ThermalProgram;样品制备/基质制备/染色/成像不并入电泳分离。
- 既有记录用法: `record-standard`、`granularity-freeze`、`catalog-promotion-audit`、`candidate-recheck`、B04/B07/B05 只作 Reuse Map 身份对照与边界参照;数值/行为证据只引本文件 §1 的原始 protocol 直接读节。B08/B12/B21 只用于边界排除(缓冲液配制/原始图像/发光读出各守其域),不作本轮晋升证据。

## 1. 来源索引

| ID | Protocol | 权威类型 | 定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| T1 | NEB Taq PCR Kit E5000S, Routine PCR + Cycling 表 + Optimization §1–11 | 厂商手册 | B04 §1 S3 / §4 | 直接读:50 µL 反应表 + 循环表 + 优化全文 |
| T2 | PowerUp SYBR Green Master Mix MAN0013511 Rev. G, Ch.2 + App.B/C | 厂商手册 | B07 §1 S3 / §4 | 直接读:配制/上机/循环 Tables 2–4 + 熔解 Table 5 + 质控门槛 |
| T3 | TaqPath 1-Step RT-qPCR Master Mix CG MAN0007959 Rev. 2.0, Ch.2 + App.B | 厂商手册 | B07 §1 S4 / §5 | 直接读:双轨配制表 + 双循环表 + 对照矩阵 |
| R1 | SuperScript IV RT User Guide (SSIV 单酶两步法) | 厂商手册 | B07 §1 S1 / §2 | 直接读:退火表 + RT 混合表 + 温育/灭活/可选 RNase H |
| R2 | Bio-Rad iScript cDNA Synthesis Kit 4106228C | 厂商手册 | B07 §1 S2 / §3 | 直接读:配制表 + 三段程序 + 建议 |
| A1 | Faircloth-lab Gel Visualization (agarose QA, 1.25% / TBE / GelRed / 110–120 V ~1 h) | 大学 lab protocol | B04 §1 P4 / §5 | 直接读:步骤 1–11 全文;适用范围仅限 DNA 提取物/扩增子/文库 QA |
| A2 | Keiser et al. agarose method; Herman Lab agarose steps | 同行/大学 protocol | C01 §2 A1–A2 | C01 转引存在;本文件未重读正文,不引数值 |
| A3 | Rockland / Bio-Rad SDS-PAGE run + stain steps | 厂商手册 | C01 §2 A3;B05 §4 步骤 1 (上游入口 out-of-scope) | 本文件未取得可直接读的 run 参数表;只作存在注记,不引电压/时间值 |
| X1 | B05 WB 程序 (transfer check → block → probe → wash → develop → image) + Detection 三模式 | 厂商手册 | B05 §§4–5 | 边界参照:SDS-PAGE 本体在 B05 为上游入口,不拆;WB 显色/成像不并入电泳分离 |
| X2 | B08 溶液/灭菌 (DMEM/PBS 配制、过滤/高压) | 厂商手册/实验室 protocol | B08 §§2–5 | 边界参照:缓冲液/凝胶试剂配制是上游,不并入分离 |
| X3 | B12 / B21 读出边界 (FLUO_IMAGE raw only; luminescence_plate_read; 流式 raw FCS) | 研究记录 | B12 §§2–6;B21 §§2–5 | 边界参照:原始采集各守其域,不并入本轮两候选 |

证据缺口声明: A3 的 SDS-PAGE 运行参数表本轮没有直接读证据;凡涉及 SDS-PAGE 电压/电流/时长/缓冲体系的具体值一律 UNKNOWN,不作断言。这是电泳 deferred 的独立理由之一。

## 2. ThermalProgram (PCR / qPCR / RT-PCR 共用的程序化热暴露)

### 2.1 比较边界

只比较“已密封反应体系 → 按程序化温度阶段/循环执行 → 热处理完成”。以下不进入本候选:

- 反应配制/master mix 分装/模板加样 → `op.liquid_transfer` 参数化 (T1/T2/T3/R1/R2 全覆盖)。
- 两步法 RT 的等温退火/合成/灭活保持 (SSIV 65 °C 5 min;23/50–55/80 °C; iScript 25/42/85 °C) → `op.incubation` 参数化,elapsed/条件 only (B07 §7 G-3)。
- 实时荧光原始采集 (逐循环 raw fluorescence + 采集元数据) 与熔解原始数据 → qPCR 方法模板内读数,不并入本候选 (B07 §7 G-5;题设禁令)。
- Cq/Ct (`instrument_reported_derived` + baseline/threshold 方法 + 软件版本)、ΔCq/ΔΔCq、效率/标准曲线、引物设计/Tm 选择 → ToolAction / out-of-scope。

一步法 (T3) 的同孔 UDG/RT/激活相是程序内阶段,其热暴露部分属于本候选的 stage 条目;cDNA 合成是否完成、cDNA 收率/均一性不断言(与两步法 RT 的 `op.incubation` 规则一致)。

### 2.2 跨方法对照 (窄边界)

| 维度 | T1 常规 PCR (NEB Taq) | T2 两步 SYBR qPCR (PowerUp) | T3 一步探针 RT-qPCR (TaqPath) |
|---|---|---|---|
| 目的 | 程序化变温驱动变性/退火/延伸循环 + 终延伸 + hold | 同左 + UDG/激活前置;40× 两档时间;可选三步 (Tm<60) | 同左 + 同孔 UDG/RT/激活前置;40× 双轨 (Fast/Standard) |
| 端口 | 待循环管 → 已扩增管 (+ 控制器日志) | 待上机板 → 已扩增板 (+ 实时荧光 sidecar,另立) | RNA/DNA 待上机板 → 已扩增板 (+ 实时荧光 sidecar,另立;无存档 cDNA 中间体) |
| 样本变化 | 指数扩增;终延伸补平;hold 4 °C 终结保持 | 同左;UDG 去污染(方法绑定);熔解为独立方法内程序,不并入本候选 | 同左;RT 相在同一程序内完成 RNA→cDNA 热暴露;UNG/激活同左 |
| 完成检查 | 控制器走完程序无报错并到达 hold;扩增成败由对照/下游另判 | 同左;NTC/质控门槛为方法对照,不由本候选断言 | 同左;no-RT/IPC/NAC/NTC 为方法对照矩阵 |
| 仪器要求 | 热循环仪:块均匀性 + 热盖 + 体积档;具体型号/均匀性值 UNKNOWN | qPCR 仪:上表能力 + 通道-染料匹配 + Fast/Standard 模式;7900HT/7500 只用 Standard 约束 | 同左 + RT 相温度窗 (50 °C,最适 48–55 °C);20 µL/fast vs 50 µL/standard 体积-模式绑定 |
| 代表绑定值 | 95 °C 30 s ×1;95/退火/68 °C ×30;68 °C 5 min;4 °C hold;退火 45–68 °C 按引物;延伸 1 min/kb | UDG 50 °C 2 min;激活 95 °C 2 min;40× (95 °C 1/3 s 或 15 s + 60 °C 30 s 或 1 min;Tm<60 加 55–60 °C 15 s + 72 °C 1 min);gDNA 推荐 Standard | UNG 25 °C 2 min;RT 50 °C 15 min;激活 95 °C 2 min;40× (fast 3 s+30 s / standard 15 s+60 s);退火 56–62 °C 可调 |

三个方法在“程序化阶段 + 持续时间 + 循环结构”上语义一致;分歧全部落在 stage 条目值与可选前置/后置相,未改变端口与完成检查类型。

### 2.3 拟晋升合同 (domain scope,不写 JSON)

- identity: `DomainSharedCandidate ThermalProgram`;适用域:核酸扩增热暴露 (PCR/qPCR/RT-PCR);等温 RT 保持与荧光采集明确排除。
- ports: `in: sealed_reaction_vessel (管/板,已配制)`;`in: program_spec (stage 表)`;`out: thermally_processed_vessel`;`out: controller_log (完成/报错)`;qPCR 荧光 sidecar 明确不是本端口产物。
- parameters (全方法绑定,无通用程序): `stages[]: {name, temp_C, duration_s, cycles}`;`lid_temp_C`;`volume_mode_uL`;`ramp_rate` UNKNOWN (`configuration_dependent`);`hold_step`。Fast/Standard、两步/三步、UDG/RT/激活有无均为 `stages[]` 取值,不另立 variant。
- sample effect: 热驱动的酶促转化暴露 (按 stage 化学:UDG/RT/变性/退火/延伸/激活);密封体系无物质转移;不 Glas 断言合成/扩增完成。
- completion checks: deterministic:控制器走完无报错并到达 hold (阻断);operational:程序-耗材-模式匹配 (板/膜/体积档,阻断);advisory:均匀性/漂移备注。扩增成功、Cq 有无、熔解峰型一律不由本候选断言。
- instrument requirements (能力类别,不绑定型号):可编程热循环能力 + 块均匀性 + 热盖 + 体积模式;qPCR 另需通道-染料匹配与 Fast/Standard 模式支持。具体均匀性/校准/软件版本 UNKNOWN。
- failure modes:程序中断/报错 → 重跑判据留方法;耗材-模式错配 (7900HT/7500 误用 Fast) → 方法前置阻断;污染风险 (扩增后开盖) 为方法约束。

### 2.4 晋升四门

- G1 复用广度:✓。T1/T2/T3 是同一技术域内三个独立方法家族 (常规终点 PCR / 两步 SYBR / 一步探针),不是同一方法变体。
- G2 语义稳定:✓ (窄边界下)。目的/端口/完成检查类型一致;循环参数与可选相均为参数值,不改变 effect。合并为多次 `op.incubation` 则 G2 失败,故明确不合并 (B04 §7 G-6;B07 §7 G-3)。
- G3 抽象收益:✓。一个 stage-list 定义 + 方法绑定替代三套重复的循环表转写。
- G4 分支成本:✓。差异 = stage 表取值 + 可选相有无,不需要按化学 (SYBR vs 探针) 或按仪器立分支;熔解、探针化学、引物设计已排除在外,故无隐藏 effect 分支。

决定:**PROMOTE 为 Domain-Shared Operation (域:核酸扩增热暴露)**。荧光采集、熔解、两步等温 RT 保持、Cq 派生明确留在方法模板/读数/ToolAction,不随本候选晋升。

## 3. Electrophoresis Separation (准备完成体系 → 施加电场 → 分离完成)

### 3.1 比较边界 (本轮重审起点)

只比较“已上样、已就位的电泳体系 → 施加电场 → 迁移完成”。以下明确不进入本候选 (题设禁令):

- 凝胶/缓冲液配制与铸胶凝固 (称量-溶胶-铸胶,微波功率/凝固判据) → 方法内步骤;B08 缓冲液配制类比,不合并。
- 样品制备/变性/还原/loading dye 加入/上样体积 → `op.liquid_transfer` + 方法内条件 (变性状态改变 analyte 路径)。
- 染色 (考马斯/银染/荧光染料) 与成像/判读 (胶图原始采集 `FLUO_IMAGE` + 条带判读分析) → 读数/分析另立。
- Transfer (WB 转膜)、Ponceau 质检 → WB 方法内步骤 (B05)。

### 3.2 跨家族对照 (窄边界仍分歧)

| 维度 | F-AGAROSE (DNA, A1 直接读) | F-SDSPAGE (蛋白, A3 存在注记,无直接读值) | 对齐结论 |
|---|---|---|---|
| 目的 | 电场驱动天然双链 DNA 按大小分离 (QA 可视化) | 电场驱动 SDS-变性多肽按分子量分离 (下游转膜/染色前置) | 意图同为“电场分离”,但 analyte 状态与下游不同 |
| 端口 | 待跑胶 (已铸 1.25% agarose/TBE + 已上样 DNA/ladder,浸没,黑→红极性) → 已分离胶 + run 日志 | 待跑胶 (已浇 PAGE + 已变性上样 + 不连续缓冲) → 已分离胶 + run 日志 | 端口名可对齐,但前置体系 (基质/缓冲/变性) 全部方法绑定且不可互换 |
| 样本变化 | 基质内空间重分布;质量守恒;天然状态 | 同为空间重分布,但 SDS 包被 + 还原/加热变性是不可逆前置跃迁;effect 含变性态 | 前置跃迁不同;分离 effect 的可比性依赖变性前提,本候选内无法表达 |
| 完成检查 | 程序时钟 (110–120 V ~1 h,浓度-电压-时间互依) + 染料前沿/ ladder 分离目视;条带身份另判 | 染料前沿 + marker 分离 + 电流/电压模式 (恒压/恒流) + 缓冲耗尽判据;具体值 UNKNOWN | 检查类型相似但阈值/模式/缓冲约束不兼容;恒压 vs 恒流分支未解 |
| 仪器要求 | 电泳槽 + 电源 (恒压能力) + TBE 缓冲;电压/均匀性 UNKNOWN | 同类电源 + 槽,但缓冲体系 (Tris-Glycine-SDS 等)、胶浓度梯度、冷却要求不同;全部 UNKNOWN | 能力类别可对齐 (电源 + 槽),但缓冲/胶/散热约束全部分叉 |
| 代表绑定 | 1.25% agarose/TBE;GelRed 两档量为上游值;110–120 V ~1 h (P4 体系值,不通用) | 运行参数无直接读值,不引用;B05 只确认 SDS-PAGE 为 WB 上游入口 | agarose 有单体系值;SDS-PAGE 无可比值 → G1 证据不对称 |

### 3.3 拟候选合同草案 (deferred,不定稿)

- ports: `in: loaded_gel_system (已铸胶 + 已上样 + 缓冲浸没 + 极性确认)`;`out: separated_gel`;`out: run_log (电压/电流/时长)`。
- parameters: `mode (constant_V/constant_mA)`;`field (V 或 mA,按 mode)`;`duration`;`polarity_check`;`buffer_system_ref (前置引用,不取值)`;`gel_system_ref (前置引用,不取值)`。
- sample effect: 基质内空间重分布;质量守恒;变性/天然状态为前置属性,不由本步改变。
- completion: operational:染料前沿到达 + ladder/marker 分离 (阻断,阈值方法绑定);deterministic:电源按设定走完无报警 (阻断)。条带身份/定量另判。
- instrument:电源恒压/恒流能力 + 槽/电极 + 缓冲兼容;不绑定型号;均匀性/校准 UNKNOWN。
- 差异成本:对齐需要同时分支——基质 (agarose vs PAGE)、缓冲 (TBE vs SDS 不连续)、analyte 态 (天然 vs 变性还原)、电气模式 (恒压 vs 恒流)、完成阈值 (QA 目视 vs marker + 转膜相容)。分支数 = 另建一个操作,违反 G4。

### 3.4 晋升四门

- G1 复用广度:△。名义上有 agarose / SDS-PAGE 两个家族,但 SDS-PAGE 运行参数无直接读证据,第二家族证据不对称;且 B04/B05 本批各只记录一个电泳家族 (B04 §9 G1 不过)。
- G2 语义稳定:✗。目的可对齐,但 analyte 前置态 (天然 vs SDS 变性)、缓冲/基质体系、电气模式与完成阈值不兼容;窄边界仍需把体系差异藏进条件分支。
- G3 抽象收益:✗。统一定义省下的转写量小于为两家族维护的条件分支与前置引用成本。
- G4 分支成本:✗。分歧 effect 藏在 flag 后 (变性/缓冲/模式),不是纯参数值差异。

决定:**DEFERRED,暂缓晋升;不永久归为 method-local**。理由:意图级共享真实存在 (电场驱动分离),但完成检查/场强-缓冲-样本态耦合仍大,且 SDS-PAGE 运行表缺直接读证据。下一步需用“准备完成 → 电场 → 分离”窄边界补第二家族直接读 (恒压/恒流、缓冲体系、marker/前沿判据、极性与浸没检查),再重审;此期间两家族分离步骤各留方法模板内,不建公共模块,不合并样品制备/基质/染色/成像。

## 4. 汇总决定

| 候选 | 分类 | 理由 | 下一步 |
|---|---|---|---|
| ThermalProgram | Promote (Domain-Shared, 核酸扩增热暴露域) | G1–G4 全过 (窄边界下);stage-list 参数化覆盖 T1/T2/T3;荧光/熔解/等温 RT 已排除,无隐藏分支 | 以 §2.3 合同立研究记录;荧光采集另立读数;等温 RT 仍复用 `op.incubation` |
| Electrophoresis separation | Deferred candidate (不晋升,不永久 method-local) | G2/G4 不过 + SDS-PAGE 直接读缺失;窄边界仍需多体系分支 | 补 SDS-PAGE 运行直接读后重审;期间留方法模板内 |

## 5. 缺口清单 (UNKNOWN 分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 热循环仪/定量 PCR 仪能力链 | `configuration_dependent` + `device_state` | 块均匀性、热盖温度、体积档、ramp、通道-滤光-染料匹配、ROX 归一行为、Cq 软件版本——T1/T2/T3 均无通用值,单机构建未读 |
| G-2 | 循环值跨 assay 复制禁令 | 方法绑定缺口 | 退火/延伸/循环数/UDG/RT/激活/熔解全部为所引手册绑定值;T1/T2/T3 值互不可比,禁携值 |
| G-3 | 电泳第二家族证据 | `source_missing` | SDS-PAGE 运行参数表未直接读;恒压/恒流、缓冲体系、marker/前沿判据全 UNKNOWN |
| G-4 | 电泳体系耦合 | `method_dependent` | 凝胶浓度-电压-时间互依 (A1 1.25%/110–120 V/~1 h 为单体系值);缓冲/变性/极性/浸没检查各守其法 |
| G-5 | 读数/派生归属 | 模型边界 | 实时荧光/Cq/熔解峰、胶图原始采集、条带判读/定量、ΔCq/曲线拟合一律不并入本轮两候选;荧光/发光/吸光读出各守其物理 (B12/B21 边界沿用) |
| G-6 | 力学/手法量 | `source_missing` | 上样手法、梳子拔出力、胶床水平、密封/气泡处理——A1/T1–T3 均无量化,归 SOP/操作者判断 |
| G-7 | 时间锚点 | 锚点缺口 | Thermal `t₀`=程序启动;熔解为程序后时钟;电泳 `t₀`=上电;铸胶凝固与储存为独立前置/保持窗。保持/程序不断言合成/扩增/分离的生物学完成 |
| G-8 | 范围外 | `not_applicable` | 样品制备/基质制备/染色/转膜/成像分割/定量、数字 PCR/等温扩增、临床/动物在体——本轮一律不记录、不建模 |

## 6. 核查与状态

- 已验证 (直接读,2026-11-05,§1 定位节):T1 反应表 + 循环表 + 优化 §1–11;T2 反应表 + Tables 2–5 + App.B/C;T3 双轨配制表 + 双循环表 + 对照矩阵;R1/R2 温育程序;A1 步骤 1–11。单人 `agent-verified`,仍需第二人逐字段复核。
- 未验证:A3 SDS-PAGE 运行参数表、任何仪器操作手册/单机构建表、转子/通道映射、per-lot 片、实验室 SOP 改编、已跑数据派生应用;检索摘要一律未转写为证据。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离;样本跃迁有端口 (§§2–3 表);时间锚显式;部分列表标部分;plan/actual/raw/derived 分离 (本轮只有 plan + 方法约束,无 run 值);无 JSON;无代码/UI/DB/Slint 变更;UNKNOWN 永不填零。
- 本文件不作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
