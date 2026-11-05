# Batch 22 — Cell Line QC: Mycoplasma / STR Authentication / Sterility / Endotoxin

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 4 个湿实验 protocol 组:支原体检测 (P-MYCO,PCR 法主程序 + 发光法/培养法注记)、STR 身份鉴定 (P-STR,多重 PCR + 毛细管电泳外送注记)、无菌检查 (P-STER,直接接种/薄膜过滤 + 环境监测注记)、内毒素鲎试验 (P-ENDO,凝胶法/动态浊度/显色基质注记)。只记录湿实验步骤与原始读数;谱型比对算法、抑菌验证统计、放行判定的质量体系流程为方法约束/分析,不建操作。
- 规则: 四组分立记录,不合并为通用 "QC test";STR 分型 PCR 的热循环记为 domain-shared candidate `ThermalProgram` 复用证据 (C01 §3),毛细管电泳外送只记存在;发光法支原体读数记为 `luminescence_plate_read` candidate 复用证据,不另建模块;现有仓库记录只作 Reuse Map 身份对照;不新增公共模块除非四问 + 晋升四门全过;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用支原体判定阈值、通用 STR 匹配率、通用无菌培养时间、通用内毒素限值;所有引物/培养基/孵育/读数条件均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| S1a | Lonza MycoAlert / MycoAlert PLUS 说明书 (发光法,比值判读) | 厂商手册 | 试剂加样/读数/比值程序 | 检索转写级:样品 + 试剂 5 min→读 A→底物加样 10 min→读 B→B/A 比;exact 阈值按 lot 说明,本批 UNKNOWN |
| S1b | 通用支原体 PCR 检测程序 (16S rRNA 引物 + 阳性对照 DNA + 内参注记,多厂商程序转写) | 厂商手册/方法论文 | 样品制备/PCR/电泳判读 | 检索转写级:上清煮沸/提取→通用引物 PCR→琼脂糖电泳条带判读;阳性对照必备;exact 引物序列/循环数按试剂盒,本批 UNKNOWN |
| S2 | ATCC STR Database 程序 + Promega GenePrint / ABI Identifiler 多重 PCR 说明书 (外送毛细管电泳) | 厂商手册/库程序 | 多重 PCR → 外送电泳 → 谱型比对 | 检索转写级:基因组提取→多重 PCR (8–16 位点 + gender)→外送毛细管电泳→等位基因 calls→80% 匹配阈值注记 (ASN-0002);exact 位点 panel 按试剂盒,本批 UNKNOWN |
| S3 | USP <71> 无菌检查 (直接接种/薄膜过滤) + 环境监测注记 (沉降/浮游菌,本批只记存在) | 药典/指南 | 培养基/接种/培养/判读 | 检索转写级:FTM/SCDM 双培养基→直接接种或薄膜过滤→14 天培养 (温度分支)→每日观察;exact 温度/接种量按药典,本批 UNKNOWN |
| S4 | USP <85>/ChP 鲎试验 (凝胶限度/动态浊度/动态显色三分支,本批主记凝胶 + 另两分支注记) | 药典/指南 | 鲎试剂/标准内毒素/阳性对照程序 | 检索转写级:样品 + 鲎试剂→37 °C 60 min→凝胶判读 (翻转不流动);定量分支标准曲线 ToolAction;干扰试验必备;exact 灵敏度 λ 按试剂,本批 UNKNOWN |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 sampling;R3 `op.incubation` (elapsed only);R4 `op.washing`;R5 `op.centrifugation`;R7 `absorbance_measurement` (显色分支);C01 `ThermalProgram` domain-shared candidate (PCR 程序) + `luminescence_plate_read` candidate (发光比值) + 电泳 deferred candidate。

检索摘要 (`search-summary-only`,非 `SourceChecked` 证据):各试剂盒 lot 片、药典全文、STR 数据库条目仅用于立项定位;凡数值以 §1 直接转写为准,未见值一律 UNKNOWN。

---

## 2. P-MYCO — 支原体检测:PCR 主程序 + 发光/培养注记 (S1a/S1b)

绑定摘要:PCR 法:培养上清 (抗生素去除 2–3 天注记)→煮沸/提取→通用 16S 引物 PCR→琼脂糖电泳条带判读,阳性对照 + 内参必备。发光法 (MycoAlert):样品 + 试剂 5 min→读 A→底物 10 min→读 B→B/A 比,阈值按 lot。培养法只记存在。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 取样:培养上清 (抗生素去除分支) | `sampling` (R2) + 方法约束 | 培养物 → 待测上清 | 方法绑定:去除时长注记;exact 天数 UNKNOWN | S1b 取样行 |
| 样品制备:煮沸/提取 | 方法内制备分支 (`MYCO-PREP`) | 待测上清 → PCR 模板 | 方法绑定:煮沸/提取二选一;exact 条件 UNKNOWN | S1b 制备行 |
| 支原体 PCR (通用引物 + 阳性对照 + 内参) | domain-shared candidate `ThermalProgram` (C01 §3) + 方法内程序绑定 | PCR 模板 → 扩增产物 | 方法绑定:引物/循环按试剂盒;对照缺即无意义 | S1b 扩增行 |
| 琼脂糖电泳条带判读 | deferred candidate (电泳) + 方法绑定判读 | 扩增产物 → 原始条带 | 方法绑定;污染/引物二聚体为方法约束 | S1b 判读行 |
| 发光法分支:试剂加样/两次读数/比值 | `luminescence_plate_read` candidate + `op.liquid_transfer` + ToolAction (比值) | 待测上清 → 原始 RLU → 派生比 | 方法绑定:5/10 min;阈值按 lot UNKNOWN | S1a 程序行 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 取样/转移/保持 | 多家族 | shared (R2/R1/R3) | 仅条件绑定 |
| PCR 热循环 (支原体 + STR) | 域内多方法 (C01 T1–T3 + 本批) | domain-shared candidate 复用证据,不新增 | 程序参数方法绑定 |
| 发光读数 (支原体 + LUC/ATP) | 域内多方法 | domain-shared candidate 复用证据,不新增 | 底物/阈值方法绑定 |
| 电泳判读 | deferred (C01 §4) | 维持 deferred,不新增 | 边界待重审 |
| 通用 "mycoplasma test" | 仅本家族 (三分支) | method-local,不新增 | G1 不过 (单方法内分支);PCR vs 发光 vs 培养不合并 |

---

## 3. P-STR — STR 身份鉴定:多重 PCR + 外送电泳 (S2)

绑定摘要:基因组提取 (B04 方法)→多重 PCR (8–16 位点 + gender marker)→外送毛细管电泳→等位基因 calls→与库谱型比对 (≥80% 匹配为人源系阈值注记 ASN-0002)→污染/漂移判读为分析。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 基因组提取 | B04 方法/composite,本批不重拆 | 细胞 → gDNA | 方法绑定;见 B04 | S2 提取行 |
| 多重 PCR (位点 panel + gender) | `ThermalProgram` candidate + 方法内程序绑定 | gDNA → 扩增产物 | 方法绑定:panel/循环按试剂盒,exact 值 UNKNOWN | S2 扩增行 |
| 外送毛细管电泳 | 存在注记 (外送),本批不拆执行 | 扩增产物 → 原始峰图 (外部) | 外部服务;仪器/聚合物 UNKNOWN | S2 外送行 |
| 谱型比对/匹配率/污染判读 | 分析/ToolAction + 方法约束 | 峰图 → 派生谱型/匹配率 | 非操作;80% 阈值注记,非本批断言 | S2 比对注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/取样 | 多家族 | shared | 仅条件绑定 |
| 热循环 (STR + 支原体 + 常规 PCR) | 域内多方法 | domain-shared candidate 复用证据,不新增 | panel/循环方法绑定 |
| 通用 "authentication" | 仅本家族 | method-local,不新增 | G1 不过;比对算法为分析 |

---

## 4. P-STER — 无菌检查:直接接种/薄膜过滤 (S3)

绑定摘要:样品→FTM (厌氧/需氧) + SCDM (真菌) 双培养基→直接接种或薄膜过滤 (冲洗液冲洗)→培养 14 天 (温度分支)→每日观察浑浊/菌落→阳性对照 + 阴性对照必备。抑菌验证/放行为质量体系流程,非操作。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 接种:直接接种 / 薄膜过滤 + 冲洗 | 方法内接种分支 (`STER-INOC`) + 转移参数化 | 样品 → 已接种培养基/膜 | 方法绑定:接种量/冲洗体积按药典,exact 值 UNKNOWN | S3 接种节 |
| 14 天培养 (温度分支 FTM/SCDM) | `op.incubation` 参数化 | 已接种物 → 已培养物 | 方法绑定:时长/温度;只 elapsed,不推断无菌 | S3 培养行 |
| 每日观察浑浊/菌落 (原始记录) | 方法绑定观察 (raw readout) | 已培养物 → 原始观察记录 | 方法绑定;判读为方法约束 | S3 观察行 |
| 阳性/阴性对照并行 | 方法对照,非操作 | — | 方法绑定;缺即无意义 | S3 对照行 |
| 环境监测 (沉降/浮游菌) | 存在注记,本批不拆 | — | 需独立 SOP | S3 注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/取样 | 多家族 | shared | 仅条件绑定 |
| 无菌接种/过滤 | 仅本家族 | method-local,不新增 | G1 不过;药典约束强 |
| 通用 "sterility" | 仅本家族 | method-local,不新增 | 单方法;观察判读非原子操作 |

---

## 5. P-ENDO — 内毒素鲎试验:凝胶 + 定量分支注记 (S4)

绑定摘要:样品 (干扰试验必备)→鲎试剂加样→37 °C 60 min→凝胶翻转判读 (不流动=阳性);动态浊度/显色分支:标准内毒素系列→动力学读数→标准曲线 ToolAction→EU/mL。灵敏度 λ/限值按试剂/药典,本批 UNKNOWN。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 加样:样品/标准/阳性对照 + 鲎试剂 | `op.liquid_transfer` 参数化 | 样品 + 试剂 → 待测管/板 | 方法绑定:体积/λ按试剂;exact 值 UNKNOWN | S4 加样行 |
| 37 °C 60 min 保持 (不振荡) | `op.incubation` 参数化 | 待测物 → 已作用物 | 方法绑定:60 min;只 elapsed | S4 保持行 |
| 凝胶翻转判读 (原始) | 方法绑定判读 (raw readout) | 已作用物 → 原始凝胶结果 | 方法绑定;振荡即失效为约束 | S4 判读行 |
| 定量分支:浊度/显色动力学 + 标准曲线 | 方法绑定读数 + ToolAction (曲线) | 已作用物 → 原始 OD/时间 → 派生 EU/mL | 方法绑定;干扰试验缺即无意义 | S4 定量注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/吸光度读数 (显色分支) | 多家族 | shared (R1/R3/R7) | 仅条件绑定 |
| 鲎凝胶判读 | 仅本家族 | method-local,不新增 | G1 不过 |
| 通用 "endotoxin test" | 仅本家族 (三分支) | method-local,不新增 | 单方法内分支;凝胶 vs 定量不合并为通用模块 |

---

## 6. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 取样/试剂/底物/培养基加样 | `op.liquid_transfer` + sampling | 参数化复用;绑定体积/比例 (§§2–5) |
| PCR 程序 | `ThermalProgram` domain-shared candidate | 复用证据 (+支原体/STR);循环/引物方法绑定 |
| 培养/保持 (接种/鲎/发光) | `op.incubation` | 参数化复用;只 elapsed,不推断无菌/阳性 |
| 发光读数 (MycoAlert) | `luminescence_plate_read` candidate | 复用证据;阈值按 lot |
| 电泳/外送/观察/凝胶判读 | deferred/外送/方法绑定读出 | 不新增;各守边界 |
| 谱型比对/比值/曲线/放行 | ToolAction/分析/体系流程 | 一律非操作 |

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | P-MYCO (S1a/b) | P-STR (S2) | P-STER (S3) | P-ENDO (S4) | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer`/sampling | ● 取样/试剂 | ● 提取/PCR | ● 接种/过滤 | ● 加样 | 全参数化 |
| `op.incubation` | ● PCR 程序/发光保持 | ● 多重 PCR 程序 | ● 14 天培养 | ● 37 °C 60 min | 只 elapsed |
| `ThermalProgram` candidate | ● 支原体 PCR | ● STR 多重 PCR | — | — | 域共享证据 |
| `luminescence_plate_read` | ● MycoAlert 比值 | — | — | — | 域共享证据 |
| `absorbance_measurement` | — | — | — | ○ 显色分支 | 原始值止 |
| 方法绑定读出/外送 | ● 电泳条带 | ● 外送峰图 | ● 观察记录 | ● 凝胶翻转 | 各守边界 |
| ToolAction/分析 | ● 阈值判读 | ● 比对/匹配率 | ● 抑菌/放行 | ● 曲线 EU/mL | 全部非操作 |

●=主路径使用;○=分支;—=本方法无此步。

方向警示:PCR 条带 (定性存在) vs 发光比值 (定量阈值) vs 培养观察 (14 天生长)——同一 "支原体阴性" 词在不同原理下对检测限/盲区含义相反,不得共用解释。鉴定警示:STR 谱型 (身份) vs 支原体 (污染) vs 无菌 (生长) vs 内毒素 (热原)——四者回答不同 QC 问题,不得合并为通用 "QC pass"。

## 8. 候选与四问判定 + 晋升审查 (本批结论:零新公共模块)

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用 (≥2 独立家族) | 结论 |
|---|---|---|---|---|---|
| 支原体 PCR/发光检测 | ✓ | ✓ | ✓ | ✗ (仅 QC 一家族;读出层已由 candidate 覆盖) | method-local,不新增 |
| STR 分型程序 | ✓ | ✓ | ✓ | ✗ (仅 QC 一家族;热循环层已由 candidate 覆盖) | method-local,不新增 |
| 无菌接种/培养观察 | ✓ | ✓ | ✓ | ✗ (仅本家族) | method-local,不新增 |
| 鲎凝胶/定量判读 | ✓ | ✓ | ✓ | ✗ (仅本家族) | method-local,不新增 |
| 通用 "cell line QC" | ✓ | ✗ (身份 vs 污染 vs 生长 vs 热原) | △ | ✗ (G2/G4 不过) | **拒绝合并**;四组分立 (题设要求) |

晋升四门:本批无候选同时通过。四组不合并的独立理由:目的不同 (身份/污染/无菌/热原)、端口不同 (gDNA/上清/样品/鲎试剂)、完成检查与对照不同 (谱型库 vs 阳性 DNA vs 双培养基对照 vs 干扰试验)、有效期不同 (STR 建库一次性 vs 支原体/无菌批放行 vs 内毒素限值)。合并将隐藏 QC 语义差异,违反 G2/G4。

结论:本批零新公共模块;对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer`、sampling、`op.incubation` + `ThermalProgram` candidate (+支原体/STR 双绑定) + `luminescence_plate_read` candidate (+MycoAlert 绑定)。

## 9. 缺口清单 (不能表达/UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | 支原体:阳性 DNA + 内参;STR:等位基因 ladder + 阴性对照;无菌:阳性/阴性 + 抑菌验证;内毒素:标准系列 + 阳性产品对照 + 干扰试验。任一缺即无意义 |
| G-2 | exact 条件未取 | `source_missing` | 引物序列/循环数、发光阈值、STR panel/位点、接种量/温度、鲎 λ/限值——按 lot/药典版本,本批 UNKNOWN |
| G-3 | 操作手法/观察主观性 | `source_missing` | 凝胶翻转力度、浑浊目视阈值、电泳条带弱阳性判读——归有经验操作者/SOP 判断 |
| G-4 | 派生/体系归属 | ToolAction/分析/体系,非操作 | 匹配率、比值阈值、EU/mL 曲线、抑菌验证统计、放行判定——全部下游/体系;无对照不作合格声明 |
| G-5 | 样品/批依赖值 | `method_dependent` + `configuration_dependent` | 抗生素去除时长、提取产量、培养基批、鲎试剂批灵敏度、外送平台 panel |
| G-6 | 不可逆/终点约束 | 跃迁约束 | PCR/发光终点;凝胶振荡即失效;14 天培养中途判阳即终止;STR 外送不可逆 |
| G-7 | 时间锚点 | 锚点缺口 | 支原体 `t₀`=模板加入/试剂加入;STR `t₀`=PCR 启动;无菌 `t₀`=接种;内毒素 `t₀`=鲎试剂加入。保持不断言检测完成 |
| G-8 | 范围外 (误建模块风险) | `not_applicable` | 谱型算法、放行 SOP、动物/临床样本、通用 "QC" 模块——本批一律不建模 |
| G-9 | 损失/守恒未量化 | 合同缺口 (沿用) | 提取损失、过滤截留、接种存活率——量级 UNKNOWN;lossy 显式 |

## 10. 核查与状态

- 已验证 (检索转写,2026-11-05,§1 定位节):S1a 发光比值程序 + S1b PCR/电泳主程序;S2 多重 PCR/外送/比对链;S3 双培养基/接种分支/14 天;S4 凝胶/定量分支 + 干扰试验。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何药典/手册全文逐页、per-lot 片 (引物/探针/鲎试剂灵敏度)、外送平台 SOP、实验室 QC 放行记录、已跑数据派生应用。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离;样本跃迁有端口;时间锚显式 (§9 G-7);部分列表标部分;plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
