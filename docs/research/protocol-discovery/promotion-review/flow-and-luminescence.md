# Flow and Luminescence Promotion Review v2 — FlowAcquisition + LuminescencePlateRead

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 目的: 对两个 domain-shared candidate 做跨方法语义对照,决定是否值得进入 Catalog、应缩小到什么范围
- 范围: 湿实验 only。流式:已染色悬液 → 原始事件/FCS;发光:待读混合物 → 孔/管积分 RLU。分析/统计/设门/补偿/群体拟合/图像分割一律 out-of-scope,不建操作
- 规则: 现有仓库记录只作 Reuse Map 身份对照,不作证据;每条数值/行为证据只引所列原始 protocol;不生成 JSON;不做 Runtime/Slint/CLI/DB 变更

## 1. 来源索引

| ID | 原始 Protocol | 覆盖方法 | 读取深度 |
|---|---|---|---|
| F-SURF | Thermo BestProtocols: Staining Cell Surface Targets (Protocol A) | 表面标记流式 | batch-06 §2 (S1-A) 直接读转写 |
| F-IC | Thermo BestProtocols: Staining Intracellular Antigens (A/B/C) | 胞内流式 (胞质/核/Foxp3/MeOH) | batch-06 §3 (S2) 直接读转写 |
| F-VIAB | Thermo BestProtocols: Viability Staining (PI/7-AAD; calcein; FVD C1–C5) | 活力流式 | batch-06 §4 (S3) 直接读转写 |
| F-ANN | Thermo BestProtocols: Annexin V Staining for Flow Cytometry (A/B/C) | 凋亡流式 | batch-12 §2 (P1) 直接读转写 |
| F-EDU | Thermo Click-iT EdU Protocol for Flow Cytometry | 增殖 EdU 流式 | batch-12 §4 (P3 Flow) 直接读转写 |
| F-CYCLE | UNC Flow Cytometry Facility: PI Staining for DNA Content in Ethanol Fixed Cells | 周期 PI 流式 | batch-12 §5 (P4) 直接读转写 |
| F-SOP | Fred Hutch flow SOP / HKU FACSAria/NovoCyte SOP | 采集仪器设置语境 | candidate-recheck C01 §5;本轮仅作边界注记,不引数值 |
| L-LUC | Promega Luciferase Assay TB281 + Dual-Luciferase E1910 | 单/双报告基因发光 | batch-21 §2 (S1) 检索转写 |
| L-ATP | Cayman ATP Bioluminescence insert | ATP 发光 | batch-21 §4 (S3b) 检索转写 |
| L-GLO | Promega ENLITEN ATP/GloMax | ATP/报告基因仪器语境 | candidate-recheck C01 §6;本轮仅作边界注记,不引数值 |
| I-IVIS | IVIS bioluminescence SOP | 成像边界对照 | 仅用于证明成像边界;animal in vivo out-of-scope,不拆解 |

本轮未新开任何外部 PDF/手册;凡数值以 batch-06/12/21 已转写绑定值为准,未见值一律 UNKNOWN。

## 2. FlowAcquisition — 跨方法对照

### 2.1 独立方法家族 (G1 证据,同一流式领域内 ≥5 独立方法)

| 方法 | 上游染色语义 (method-local,不合并) | 采集前输入 | 原始输出 | 定位 |
|---|---|---|---|---|
| 表面标记 | Fc/染料配伍封闭 + 抗体,30 min 2–8 °C | 已染悬液 | 原始 FCS | F-SURF;batch-06 §2 |
| 胞内标记 | IC/Foxp3/MeOH 固定通透 + 胞内染 | 已固定通透染色悬液 | 原始 FCS | F-IC;batch-06 §3 |
| 活力标记 | PI/7-AAD (禁洗禁固定) / calcein / FVD (固定兼容) 三分立 | 死染/活染/FVD 标记悬液 | 原始 FCS | F-VIAB;batch-06 §4 |
| Annexin V 凋亡 | 钙依赖 PS 结合 + PI/7-AAD 或 FVD 路由 | 已 Annexin/活力标记悬液 | 原始 FCS | F-ANN;batch-12 §2 |
| EdU 增殖 (流分支) | EdU 脉冲 + Click + 可选 DNA 含量 | 已 Click 标记悬液 | 原始 FCS (DNA 线性 / EdU 对数) | F-EDU;batch-12 §4 |
| PI 周期 | 乙醇固定 + RNase + PI 化学计量染色 | PI 定量悬液 | 原始 FCS (DNA 含量) | F-CYCLE;batch-12 §5 |

六者上游染色 effect/可逆性/固定兼容互不相同,一律留在 method-local (沿用 audit #23/#26);共享的只是“已染悬液 → 原始事件/FCS”这一读出物理。

### 2.2 拟议合同 (narrow: raw acquisition only)

- 身份: `FlowAcquisition`,domain-shared,适用域:流式细胞术
- 目的: 对已染色单细胞悬液做逐事件荧光/散射询问,记录原始事件,不做任何群体解释
- 端口: 输入 = 待测悬液 (管/孔,必填,基数 1);输出 = 原始 FCS/事件集 (必填) + 废液/已通过细胞 (lossy,显式);无分选组分输出
- 参数 (全方法绑定,无通用默认值): 事件数目标、流速/压力档、阈值/触发通道、激光–滤光–检测器映射、增益/电压;逐 run 按 facility 构建表绑定,本轮全 UNKNOWN (`configuration_dependent`)
- 样本 effect: 悬液经 interrogation 后消耗,不返回可培养物;不产生新细胞群谱系 (与分选区别点);固定/染色状态不变,仅位置/存量变化
- 时间: `per_assay`;PI/7-AAD 加样 → 上机 ≤4 h 硬时钟为例值,属方法绑定,不写入通用默认
- 完成检查 (deterministic): FCS 文件已记录 + 事件数达到方法绑定目标;不推断结合完成/阳性率/周期分布
- 仪器要求: 能力类别 = 流式细胞仪 (激光/滤光/检测器/阈值/流速能力);不绑定具体型号/配置;喷嘴/分选流路/收集器不在本操作要求内
- 对照: unstained + 每荧光单染 + FMO + 每 run 活力对照为方法模板对照,非本操作参数;缺一判读无效,但不阻塞采集文件存在性
- 失败模式: 阈值不当致碎片淹没;流速过高致 coincidence;浓度过稀致事件不足;堵塞/气泡致中断 — 现象记入 run,处理回方法 SOP
- readout: raw only;MFI/阳性%/周期拟合/纯度/得率全为下游 ToolAction/分析
- unknowns: 事件数/流速/激光-滤光-增益-阈值/质控限 (`configuration_dependent` + `source_missing`,F-SOP 未读到构建表)

### 2.3 与分选/设门/补偿的严格分离

| 动作 | 输入 → 输出 | 样本变化 | 归属 | 合并判定 |
|---|---|---|---|---|
| FlowAcquisition (本候选) | 已染悬液 → 原始 FCS + 废液 | 消耗,无新谱系 | domain-shared candidate | — |
| 流式分选 (FACS) | 已染悬液 → 分选亚群 + 分选后 FCS + 废液 | 产生目标细胞群与父子谱系,纯度/得率/无菌路径 | 方法模板内独立分支 | **拒绝合并**: 端口 (收集管/板/收集液)、effect (新活细胞群)、完成检查 (纯度/得率/无菌) 全不同;G2 不过 |
| 设门/补偿/群体统计 | 原始 FCS → 门/补偿矩阵/阳性%/MFI | 无湿样本变化,纯数据变换 | out-of-scope / ToolAction | **拒绝合并**: 无样本端口;G1/G2 不过 |
| 上游染色/固定/通透 | 悬液 → 已染悬液 | 标记/固定/通透 effect | method-local (三分立/多分立) | **拒绝合并**: effect/可逆性/固定兼容不稳定;沿用 audit #23/#26 |

## 3. LuminescencePlateRead — 跨方法对照

### 3.1 独立方法家族 (G1 证据,板式发光领域内 ≥2 独立 assay)

| 方法 | 上游语义 (method-local) | 采集前输入 | 原始输出 | 定位 |
|---|---|---|---|---|
| 单报告基因 (firefly) | 被动裂解 15 min + LAR II 底物 | 待读混合物 (裂解物+底物) | 孔/管积分 RLU (2 s 延迟 10 s 积分,例值) | L-LUC;batch-21 §2 |
| 双报告基因 (firefly + Renilla) | 上轮 + Stop&Glo 淬灭 + 海肾底物第二轮 | 淬灭后待读混合物 | 第二轮积分 RLU + 萤火虫/海肾比 (比值为 ToolAction) | L-LUC;batch-21 §2 |
| ATP 发光 | ATP 裂解 + 荧光素酶底物 | 待读混合物 | 孔/管积分 RLU → 标准曲线浓度 (曲线为 ToolAction) | L-ATP;batch-21 §4 |

三者底物/裂解/淬灭/对照互不相同,留在方法内;共享的只是“孔/管积分光信号”读出物理。

### 3.2 拟议合同 (narrow: plate/tube integrated RLU only)

- 身份: `LuminescencePlateRead`,domain-shared,适用域:板式发光检测
- 目的: 对已加底物的裂解/反应混合物做无激发积分光测量,记录原始 RLU,不做任何活性/浓度解释
- 端口: 输入 = 待读混合物 (孔/管,必填,基数 1);输出 = 原始 RLU 矩阵 (必填) + 已读耗尽混合物 (终点,显式);无空间分辨图像输出
- 参数 (全方法绑定): 延迟/积分窗、加样体积/顺序、裂解/底物体系;exact 延迟/积分/浓度按 lot/仪器,本轮 UNKNOWN (`method_dependent` + `configuration_dependent`);`t₀`=底物加入
- 样本 effect: 裂解终点不可逆;混合物经读取后衰减/耗尽,不返回可用样本;延迟漂移为方法约束
- 完成检查 (deterministic): 每孔/管 RLU 已记录 + 延迟/积分窗与方法绑定一致;不推断报告基因活性/ATP 浓度
- 仪器要求: 能力类别 = 发光检测器 (无激发光路,积分模式);不绑定具体型号;滤光片/激发模块不在要求内
- 对照: 空载/未转染归一、阳性对照、标准曲线、未负载空白为方法模板对照,非本操作参数
- 失败模式: 底物衰减致延迟漂移;裂解不完全致偏低;串孔/本底致偏高 — 回方法 SOP,不改读出定义
- readout: raw RLU only;比值/归一/标准曲线浓度/MFI/倍数全为下游 ToolAction
- unknowns: exact 底物 µM、延迟/积分/采样、通道/增益、仪器型号 (`method_dependent` + `source_missing`,L-GLO 未读到手册)

### 3.3 与成像/吸光度的严格分离

| 动作 | 输入 → 输出 | 区别点 | 归属 | 合并判定 |
|---|---|---|---|---|
| LuminescencePlateRead (本候选) | 待读混合物 → 积分 RLU (无空间分辨) | 无激发/无波长,动力学由延迟/积分窗定义 | domain-shared candidate | — |
| 发光成像 (`luminescence_image_acquisition`) | 整体样品/动物 → 空间分辨图像 | 有空间分辨率,曝光/视野/定位语义;IVIS 在体 out-of-scope | 独立分支,另立 | **拒绝合并**: 端口/分辨率/适用域不同;G2/G4 不过 |
| 吸光度读取 (`ABS_READ`,已有) | 溶解物 → 原始 OD (有波长/参比) | 有激发/波长/线性声称,仪器与物理不同 | 已有 shared op,各守边界 | **拒绝合并**: 物理/检测器/动力学不同;沿用 audit §4.4 |
| 荧光板读/流式/成像 | 已负载细胞 → 荧光强度/事件/图像 | 有激发/发射,光漂白/淬灭约束 | 方法绑定读出,各守边界 | **拒绝合并**: batch-21 §7 方向警示;G2 不过 |

## 4. 晋升判定 (G1–G4,沿用 freeze §5.2 / audit §1)

| 候选 | G1 复用广度 | G2 语义稳定 | G3 抽象收益 | G4 分支成本 | 决定 |
|---|---|---|---|---|---|
| FlowAcquisition | ✓ (表面/胞内/活力/Annexin/EdU-流/周期六方法,域内成立) | ✓ (narrow 到 raw FCS 后端口/effect/完成检查一致;染色差异已排除在外) | ✓ (一定义 + 参数绑定替代每方法重述采集) | ✓ (分选/设门分离后无隐藏分支;激光-滤光值走配置绑定) | **Promote 为 domain-shared,不做 global op**;适用域注记:流式细胞术;分选/设门永不并入 |
| LuminescencePlateRead | ✓ (报告基因 + ATP,域内多 assay;C01 L1–L3) | ✓ (narrow 到积分 RLU 后一致;活性/浓度解释已排除为 ToolAction) | ✓ (一定义 + 延迟/积分绑定替代每 assay 重述读出) | ✓ (成像另立后无隐藏分支;底物/裂解走方法绑定) | **Promote 为 domain-shared,不做 global op**;适用域注记:板式发光检测;成像/吸光度永不并入 |
| 通用 "stain/fix/perm/block" | ✓ | ✗ | ✗ | ✗ | 维持 method-local,不晋升 |
| 流式分选 | ✗ (本轮无第二独立分选家族证据) | — | — | — | 方法模板内分支,不立公共操作 |
| 发光成像 | ✗ (仅 IVIS 边界注记,在体 out-of-scope) | — | — | — | 不建模,另立分支时需独立 protocol |
| 比值/归一/曲线/MFI/拟合/统计 | — | — | — | — | ToolAction/分析,永不建模块 |

## 5. 缺口清单 (UNKNOWN 分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 流式仪器值 | `configuration_dependent` + `source_missing` | 激光线/滤光/增益/阈值/事件数/流速/质控限;F-SOP 构建表未读;plan 前须逐 run 绑定 |
| G-2 | 分选值 | `source_missing` | 喷嘴/压力/drop-delay/收集液/纯度标准/无菌路径;本轮未拆,不作采集参数 |
| G-3 | 发光 exact 条件 | `method_dependent` + `configuration_dependent` | 底物/探针 exact µM、延迟/积分/增益、仪器型号;按 lot/仪器绑定 |
| G-4 | 光/化学约束 | `method_dependent` | 底物衰减漂移、裂解不完全、串孔本底;原文定性,归操作者判断 |
| G-5 | 派生归属 | ToolAction/分析,非操作 | 萤火虫/海肾比、标准曲线浓度、MFI/阳性率、周期拟合、设门补偿 — 全部下游 |
| G-6 | 不可逆/终点约束 | 跃迁约束 | 固定终结回培养;PI/7-AAD 染后禁固定;裂解终点不可逆;FVD 逢固定必选 |
| G-7 | 时间锚点 | 锚点缺口 | 流式 `t₀`=染色/加样,PI ≤4 h 窗独立;发光 `t₀`=底物加入;保持不断言结合/反应完成 |
| G-8 | 范围外 | `not_applicable` | 分选执行、IVIS 在体成像、Seahorse 运行、图像分割、动力学建模、统计/生物信息 — 本轮一律不记录、不建模 |

## 6. 核查与状态

- 已验证: 本轮未新开来源;事实断言以上表定位节 batch-06 §§2–4、batch-12 §§2/4/5、batch-21 §§2/4、candidate-recheck C01 §§5–6 为准。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证: 任何流式/发光仪器手册、单机构建表、转子映射、per-lot 片、实验室 SOP 改编、滴定终点、已跑数据派生应用。
- 记录清单: 每值有定位或标 UNKNOWN;方法要求与仪器规格分离;样本跃迁有端口 (§§2–3);时间锚显式;plan/actual/raw/derived 分离 (本轮只有 plan + 方法约束);无 JSON;UNKNOWN 永不填零。
