# Batch 21 — Reporter / ROS / Mitochondrial Function / Calcium Readouts

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 4 个湿实验 protocol 组:荧光素酶报告基因 (P-LUC,单/双报告)、ROS 探针读数 (P-ROS,DCFH-DA/CellROX/MitoSOX 注记)、线粒体功能 (P-MITO,膜电位 JC-1/TMRM + ATP 发光 + Seahorse 存在注记)、钙信号读数 (P-CA,Fluo-4/Rhod-2/Fura-2 注记)。只记录湿实验步骤与原始读数;剂量拟合/动力学建模/图像分割/统计一律 out-of-scope 或 ToolAction/分析。
- 规则: 四组分立记录,不合并为通用 "redox/mito assay";板式发光读数统一记为 domain-shared candidate `luminescence_plate_read` (C01 §6),不另建模块;荧光板读/流式/成像各守边界,不合并;现有仓库记录只作 Reuse Map 身份对照;不新增公共模块除非四问 + 晋升四门全过;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用报告基因活性、通用 ROS 水平、通用膜电位、通用钙浓度;所有底物/探针浓度/孵育/读数条件均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Promega Luciferase Assay System TB281 (单报告) + Dual-Luciferase Reporter Assay E1910 技术手册 (萤火虫/海肾双报告) | 厂商手册 | 裂解/底物/读数/淬灭-读二轮程序 | 检索转写级:被动裂解 15 min→底物 LAR II 加样→2 s 延迟 10 s 积分→Stop&Glo 淬灭→海肾底物第二轮;exact 延迟/积分按仪器,本批 UNKNOWN |
| S2 | Thermo DCFH-DA / CellROX Green + MitoSOX Red 说明书程序 (ROS 通用流程) | 厂商手册 | 探针负载/洗/刺激/读数 | 检索转写级:无血清培养基探针负载 (DCFH-DA ~10 µM 30 min 37 °C 注记)→洗→刺激/处理→板读/流式/成像;MitoSOX 线粒体分支 5 µM 10 min;exact 浓度按 lot/系优化,本批 UNKNOWN |
| S3a | 膜电位 JC-1 / TMRM 通用程序 (Thermo 说明书 +  facility SOP 转写) | 厂商手册/SOP | 探针负载/阳性对照 CCCP/FCCP/读数比 | 检索转写级:JC-1 负载 15–30 min 37 °C→红/绿比;TMRM 非淬灭模式低浓度→阳性对照去极化;exact 浓度/通道按仪器,本批 UNKNOWN |
| S3b | Cayman ATP Bioluminescence insert + Seahorse XF 存在注记 | 厂商手册/注记 | ATP 发光板读;Seahorse 只记存在 | 检索转写级:ATP 裂解→荧光素酶底物→板读 RLU→标准曲线 ToolAction;Seahorse OCR/ECAR 本批不拆执行 |
| S4 | Thermo Fluo-4 / Fura-2 / Rhod-2 钙指示剂程序 (AM 酯负载 + Pluronic/去酯化注记) | 厂商手册 | 负载/去酯化/基线/刺激/动态读数 | 检索转写级:AM 酯 + Pluronic 负载 30 min 37 °C→洗→去酯化 30 min RT→基线读→刺激→动态荧光;Fura-2 比值 340/380;exact 负载浓度/采样率按仪器,本批 UNKNOWN |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 sampling;R3 `op.incubation` (elapsed only);R4 `op.washing` (保留/lossy);R5 `op.centrifugation`;R7 `absorbance_measurement` (本批无关,仅 ATP 背景对照);C01 `luminescence_plate_read` domain-shared candidate (板式积分 RLU,优先级高,不是 gap);`FLUO_IMAGE` raw only;流式原始采集 domain-shared candidate。

检索摘要 (`search-summary-only`,非 `SourceChecked` 证据):各探针 lot 片、Seahorse 试剂盒页、IVIS 成像页仅用于立项定位;凡数值以 §1 直接转写为准,未见值一律 UNKNOWN。

---

## 2. P-LUC — 荧光素酶报告基因:单/双报告 (S1)

绑定摘要:转染后处理→被动裂解液 15 min RT (摇)→裂解物转管/板→LAR II 底物加样→2 s 延迟 10 s 积分读萤火虫→Stop&Glo 淬灭→海肾底物第二轮读数→萤火虫/海肾比为 ToolAction。ATP 发光 (S3b) 共享同一读出物理但底物/裂解/对照不同,不合并方法。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 被动裂解 15 min RT + 摇 | `op.incubation` 参数化 + 转移 | 贴壁/悬浮细胞 → 裂解物 | 方法绑定:15 min;只 elapsed,不推断裂解完全 | S1 裂解行 |
| 底物加样 (LAR II / Stop&Glo/海肾底物) | `op.liquid_transfer` 参数化 | 裂解物 + 底物 → 待读混合物 | 方法绑定:加样体积/顺序;exact µL 按管/板程序 | S1 加样行 |
| 发光板读/管读:2 s 延迟 10 s 积分,逐轮 | domain-shared candidate `luminescence_plate_read` (C01 §6) | 待读混合物 → 原始 RLU | 方法绑定:延迟/积分窗;仪器型号 UNKNOWN | S1 读数行 |
| 萤火虫/海肾比 + 空载/未转染对照归一 | ToolAction + 方法对照 | 原始 RLU → 派生比值 | 方法模板内公式;无对照不作活性声明 | S1 数据处理节 |
| 动物在体 IVIS 成像 | out-of-scope (animal in vivo) | — | 本轮排除;仅 C01 边界注记 | C01 I1 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/保持/取样 | 多家族 | shared (R1/R3/R2) | 仅条件绑定 |
| 发光板式读数 (LUC + ATP) | 域内多方法 (报告基因/ATP, C01 L1–L3) | domain-shared candidate,不新增 | 共享积分 RLU 物理;底物/对照方法绑定;成像另立 |
| 通用 "reporter activity" | 表面多见 | 不建通用模块 | 底物/酶/淬灭/归一对照异质,G2/G4 不过 |
| 比值/归一/拟合 | — (计算) | ToolAction,非操作 | 无对照不声明 |

---

## 3. P-ROS — ROS 探针读数 (S2)

绑定摘要:贴壁/悬浮细胞→无血清培养基 DCFH-DA (~10 µM) 30 min 37 °C 负载 (CellROX 同类;MitoSOX 5 µM 10 min 线粒体分支)→洗→处理/刺激 (阳性对照 TBHP/Rosup 注记)→板读/流式/成像原始荧光。探针氧化不可逆,光/酯酶/外排为方法约束。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 探针负载:无血清培养基 + 探针,30 min 37 °C (MitoSOX 10 min) | `op.incubation` + `op.liquid_transfer` 参数化 | 细胞 → 已负载细胞 | 方法绑定:浓度/时长分支;exact µM 按系优化 UNKNOWN | S2 负载行 |
| 洗去胞外探针 | `op.washing` 参数化 (lossy 显式,贴壁脱落/悬浮丢失) | 已负载细胞 → 待测细胞 | 方法绑定:洗液/次数;exact 体积 UNKNOWN | S2 洗涤行 |
| 处理/刺激 + 阳性对照并行 | 方法内处理分支 + 对照 | 待测细胞 → 已处理细胞 | 方法绑定:刺激品种/时长;对照缺即无意义 | S2 处理注记 |
| 原始荧光读数:板读 / 流式 / 成像三选一 | 方法绑定读出 (板读=荧光强度;流式=原始采集 candidate;成像=`FLUO_IMAGE` raw) | 已处理细胞 → 原始荧光 | 方法绑定:激发/发射/通道;三者不互换 | S2 读数行 |
| MFI/阳性率/倍数变化 | ToolAction/分析 | 原始荧光 → 派生指标 | 非操作;设门/分割 out-of-scope | S2 数据节 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/洗涤/取样 | 多家族 | shared (R1/R3/R4/R2) | 仅条件绑定 |
| 荧光板读 vs 流式 vs 成像 | 域内三读出 | 各守边界,不合并 | 端口/样本消耗/分辨率不同,G2 不过 |
| 通用 "ROS level" | 表面多见 (DCF/CellROX/MitoSOX) | 不建通用模块 | 探针定位/化学/可逆性/对照异质,G2/G4 不过 |
| 阳性对照归一 | 方法对照 | 非操作 | 缺对照不作水平声明 |

---

## 4. P-MITO — 线粒体功能:膜电位 + ATP + Seahorse 注记 (S3a/S3b)

绑定摘要:JC-1 负载 15–30 min 37 °C→红 (聚集体,健康) / 绿 (单体,去极化) 比;TMRM 低浓度非淬灭模式→去极化对照 CCCP/FCCP 并行;ATP 发光 (裂解→底物→板读 RLU→标准曲线);Seahorse OCR/ECAR 本批只记存在,不拆执行程序。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| JC-1/TMRM 负载 + 保持 | `op.incubation` + `op.liquid_transfer` 参数化 | 细胞 → 已负载细胞 | 方法绑定:浓度/时长;淬灭/非淬灭模式按程序 | S3a 负载行 |
| 去极化对照 CCCP/FCCP 并行 | 方法对照,非操作 | — | 方法绑定;缺即无意义 | S3a 对照行 |
| 膜电位原始荧光读数 (比值/强度) | 方法绑定读出 (板读/流式/成像三选一,各守边界) | 已负载细胞 → 原始荧光 | 方法绑定:通道/模式;比值计算为 ToolAction | S3a 读数行 |
| ATP 裂解 + 底物 + 板读 RLU | `luminescence_plate_read` candidate + 转移/保持参数化 | 细胞 → 原始 RLU | 方法绑定:裂解/底物/延迟积分;与 LUC 底物不同 | S3b ATP 行 |
| ATP 标准曲线 → 浓度 | ToolAction | 原始 RLU → 派生浓度 | 方法模板内公式 | S3b 数据节 |
| Seahorse OCR/ECAR | 存在注记,本批不拆 | — | 需独立 protocol;仪器配置 UNKNOWN | S3b 注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/洗涤 | 多家族 | shared | 仅条件绑定 |
| 发光板读 (ATP + LUC) | 域内多方法 | domain-shared candidate,不新增 | 同一读出物理;底物/裂解方法绑定 |
| 膜电位比值 (JC-1/TMRM) | 仅本家族 | method-local,不新增 | G1 不过;淬灭模式/通道异质 |
| 通用 "mitochondrial function" | 表面多见 | 不建通用模块 | 膜电位 vs ATP vs OCR 机理/端口异质,G2/G4 不过 |
| Seahorse 运行 | 未拆家族 | 不建模 | 缺独立 protocol |

---

## 5. P-CA — 钙信号读数 (S4)

绑定摘要:AM 酯 + Pluronic 负载 30 min 37 °C→洗→去酯化 30 min RT→基线读→刺激 (激动剂/去极化)→动态荧光采集 (Fluo-4 单波长强度;Fura-2 340/380 比值;Rhod-2 线粒体分支)。采样率/刺激时序按仪器,动力学拟合为分析。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| AM 酯负载 (+Pluronic) 30 min 37 °C | `op.incubation` + `op.liquid_transfer` 参数化 | 细胞 → 已负载细胞 | 方法绑定:浓度/助溶剂;exact µM UNKNOWN | S4 负载行 |
| 洗 + 去酯化 30 min RT | `op.washing` + `op.incubation` 参数化 | 已负载细胞 → 待测细胞 | 方法绑定:去酯化时长;只 elapsed | S4 洗/保持行 |
| 基线读 → 刺激加样 → 动态采集 | 方法内动态读出分支 (`CA-KINETIC`) + 转移 | 待测细胞 → 原始荧光时间序列 | 方法绑定:刺激时序/采样率;仪器 UNKNOWN | S4 采集行 |
| 峰值/达峰时间/AUC/比值换算 | ToolAction/分析 | 时间序列 → 派生指标 | 非操作;无基线不作浓度声明 | S4 数据节 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 转移/保持/洗涤 | 多家族 | shared | 仅条件绑定 |
| 动态荧光采集 | 仅本家族 (+ ROS 板读远亲) | method-local,不新增 | G1 不过;时序/刺激绑定太强 |
| 通用 "calcium level" | 表面多见 (Fluo-4/Fura-2/Rhod-2) | 不建通用模块 | 单波长 vs 比值 vs 线粒体定位异质,G2 不过 |

---

## 6. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 底物/探针/刺激加样 | `op.liquid_transfer` | 参数化复用;绑定体积/浓度/顺序 (§§2–5) |
| 裂解/负载/去酯化/保持 | `op.incubation` | 参数化复用;绑定时长/温度;只 elapsed,不推断负载/裂解完成 |
| 探针洗/去酯化洗 | `op.washing` | 参数化复用;lossy 显式 |
| 集菌/澄清 (悬浮分支) | `op.centrifugation` | 参数化复用;绑定 RCF/时长 |
| 裂解物/细胞分样 | sampling atomic | 参数化复用;谱系保留 |
| 发光板读/管读 (LUC/ATP) | `luminescence_plate_read` domain-shared candidate | 复用读出物理;底物/延迟/积分方法绑定;成像另立 |
| 荧光板读/流式/成像 | 方法绑定读出,各守边界 | 不合并;通道/模式/分辨率异质 |
| 比值/归一/曲线/动力学/统计 | ToolAction/分析 | 一律非操作 |

## 7. 跨方法公共操作覆盖矩阵

| 公共操作 | P-LUC (S1) | P-ROS (S2) | P-MITO (S3a/S3b) | P-CA (S4) | 备注 |
|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 裂解/底物 | ● 探针/刺激 | ● 探针/底物 | ● 酯/刺激 | 全参数化 |
| `op.incubation` | ● 裂解 15 min | ● 负载 30/10 min | ● 负载 15–30 min | ● 负载/去酯化 30+30 min | 只 elapsed |
| `op.washing` | — | ● 去胞外探针 | ○ 洗 (分支) | ● 洗 + 去酯化 | lossy 显式 |
| `luminescence_plate_read` | ● 双轮 RLU | — | ● ATP RLU | — | 域共享候选;底物不同 |
| 荧光读出 (板/流/像) | — | ● 三选一 | ● 三选一 | ● 动态序列 | 各守边界,不合并 |
| method-local (底物/探针/模式) | ● LAR/Stop&Glo | ● DCF/MitoSOX | ● JC-1/TMRM/ATP | ● AM/比值/刺激 | 各留方法内 |
| ToolAction/分析 | ● 比值归一 | ● MFI/倍数 | ● 比值/曲线 | ● 峰/AUC | 全部非操作 |

●=主路径使用;○=分支;—=本方法无此步。

方向警示:发光积分 (无激发,RLU,延迟/积分窗) vs 荧光强度 (有激发/发射,光漂白/淬灭) vs 比值 (双通道归一,仪器/通道绑定)——同一 "读数" 词在不同物理下对定量含义相反,不得共用模块。探针警示:胞质 ROS vs 线粒体 ROS vs 膜电位 vs 胞质钙——定位/化学/可逆性不同,不得共用 "redox/mito" 解释。

## 8. 候选与四问判定 + 晋升审查 (本批结论:零新公共模块)

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用 (≥2 独立家族) | 结论 |
|---|---|---|---|---|---|
| 发光板读 (LUC/ATP) | ✓ | ✓ | ✓ | ✓ (报告基因 + ATP,域内多方法) | domain-shared candidate (C01 已立),本批不新增 |
| 荧光板读 (ROS/钙/膜电位) | ✓ | ✓ | ✓ | △ (读出物理共享,但通道/模式/定位绑定强) | 维持方法绑定读出;是否立 domain-shared 候选需下一轮跨家族比较,本批不新增 |
| 动态钙采集 | ✓ | ✓ | ✓ | ✗ (仅钙一家族) | method-local,不新增 |
| 通用 "ROS/mito/calcium assay" | ✓ | ✗ (探针/定位/物理异质) | △ | ✗ (G2/G4 不过) | **拒绝合并**;四组分立 (题设要求) |
| 通用 "reporter activity" | ✓ | ✗ (酶/底物/归一异质) | △ | ✗ | 不建通用模块 |

晋升四门:本批无候选同时通过 (发光板读已由 C01 立为 candidate,本批只是复用证据增量,不重复晋升)。四组不合并的独立理由:端口不同 (裂解物+底物 vs 活细胞+探针 vs 线粒体探针 vs AM 酯+刺激时序)、样本 effect 不同 (裂解终点 vs 可逆/不可逆负载 vs 膜电位动态 vs 钙瞬变)、完成检查与对照不同 (双报告归一 vs 阳性对照 vs 去极化对照 vs 基线+刺激)。合并将隐藏物理/化学差异,违反 G2/G4。

结论:本批零新公共模块;对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer`、`op.incubation`、`op.washing`、sampling + `luminescence_plate_read` candidate (+LUC/ATP 双绑定)。

## 9. 缺口清单 (不能表达/UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | LUC:空载/未转染 + 海肾归一;ROS:阳性对照 + 未负载空白;膜电位:CCCP/FCCP 去极化对照;钙:基线 + 溶剂对照。任一缺即无意义 |
| G-2 | exact 条件未取 | `source_missing` | 底物/探针 exact µM、延迟/积分/采样率、通道/滤光片、仪器型号——按 lot/仪器,本批 UNKNOWN |
| G-3 | 光/化学约束 | `method_dependent` | 探针光漂白/淬灭、外排/去酯化不完全、底物衰减 kinetics——原文定性,归操作者判断;读数延迟漂移 |
| G-4 | 派生归属 | ToolAction/分析,非操作 | 比值/归一、标准曲线、MFI/阳性率、峰/AUC、动力学拟合、统计——全部下游;无对照/基线不作活性/浓度声明 |
| G-5 | 系别/批依赖值 | `method_dependent` + `configuration_dependent` | 探针浓度/负载时长按系滴定;读数增益/通道按仪器;刺激剂量按系 |
| G-6 | 不可逆/终点约束 | 跃迁约束 | DCF 氧化不可逆;裂解终点不可逆;AM 酯去酯化后外排;延迟读数漂移;成像光毒性另记 |
| G-7 | 时间锚点 | 锚点缺口 | LUC `t₀`=底物加入;ROS `t₀`=探针负载开始;膜电位 `t₀`=探针加入;钙 `t₀`=刺激加入。保持不断言负载/反应完成 |
| G-8 | 范围外 (误建模块风险) | `not_applicable` | IVIS 在体成像、Seahorse 运行、图像分割、动力学建模、统计、通用 "redox/mito/calcium" 模块——本批一律不建模 |
| G-9 | 损失/守恒未量化 | 合同缺口 (沿用) | 洗涤探针损失、悬浮离心丢失、裂解不完全——量级 UNKNOWN;lossy 显式 |

## 10. 核查与状态

- 已验证 (检索转写,2026-11-05,§1 定位节):S1 双轮读数程序;S2 负载/洗/读三选一 + MitoSOX 分支;S3a 负载/对照/比值 + S3b ATP 板读 + Seahorse 注记;S4 负载/去酯化/基线/刺激/动态。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何 PDF 全文逐页、per-lot 片 (底物/探针/抗体)、仪器操作手册、实验室 SOP 改编、已跑数据派生应用、Seahorse 执行程序。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离;样本跃迁有端口;时间锚显式 (§9 G-7);部分列表标部分;plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
