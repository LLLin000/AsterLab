# 从步骤到实验：协议知识、软件参照与 AsterLab 的边界

- 日期：2026-11-05
- 状态：研究笔记，不是 UI 规范、不是验收结论、不是用户测试结论。
- 基线：仓库 HEAD `8e14130`；本轮只读源码、研究资料和厂商/标准文档，没有改 UI、没有启动应用、没有运行测试。
- 范围：回答“软件怎样才算真的懂一步实验”，而不是声称能替研究者设计所有实验。

## 0. 先否定一个不成立的前提

没有人能专业地设计所有实验。实验类型、细胞/材料、仪器型号、实验室 SOP 和科学假设都在变化；把“懂所有实验”当成产品目标，最可能得到一份看起来完整、实际不能执行的 SOP。

可以把“专业理解”降成一个可检查的标准：

1. 知道这一步**为什么存在**，而不只是知道动作名称。
2. 知道它的**输入、输出和完成标准**。
3. 知道哪些参数有单位、范围、来源，哪些必须由实验室确认。
4. 知道这一步的**对照、干扰、失败模式和判定点**。
5. 知道读数的含义边界：什么能说，什么不能从读数直接推出。
6. 知道它属于哪个 Protocol/Assay 变体，不能把别的变体参数硬套过来。

软件要做的不是把知识全部塞进模型，而是让每条知识**有来源、有版本、可被专家审核、未知时不被猜填**。AsterLab 的现有方向——设计、Protocol、Run 分开，未知不伪装成事实——比“AI 生成完整实验方案”更可靠。

## 1. “专业理解”的检查表

每个步骤块至少应能回答以下问题；回答不了的项目不是“稍后补”，而是明确的待确认项。

| 维度 | 要回答的问题 | 不能接受的替代 |
|---|---|---|
| 目的 | 这一步改变什么、为什么需要 | “按常规”“充分混匀” |
| 动作 | 对什么材料做什么 | 只有仪器名，没有动作 |
| 输入 | 样本、试剂、容器、耗材、仪器 | 用“细胞”代替具体样本/孔 |
| 输出 | 处理后的材料、数据、状态 | 把计划输出写成已产生 |
| 参数 | 值、单位、范围、来源、所属层 | 无单位数字；把示例值当默认 |
| 时间 | 时长、温度、CO2、锚点 | “过夜”“适当时间”无定义 |
| 对照 | 空白、阴性、阳性、未染色、标准曲线 | 一套对照套所有检测 |
| 判定点 | 何时继续、何时停止、何时重做 | 隐藏在选择里的经验 |
| 失败模式 | 气泡、蒸发、抑制、污染、偏倚 | 只写“注意操作” |
| 读数含义 | 原始值、派生值、能/不能推出什么 | 把 OD 直接叫“活力” |
| 变体 | 96/384、贴壁/悬浮、显微镜/流式 | 把一种方法当唯一标准 |
| 证据 | 厂商手册、同行评议、实验室 SOP | 模型记忆当来源 |

这张表可以直接变成步骤块的字段，而不是产品说明。

## 2. 步骤契约：把“懂”写成结构

现有 `ProtocolBlock` 已经有名称、用途、输入/输出、参数槽位和来源；还需要把“专业理解”显式化：

```text
StepDefinition（可复用、版本化）
├─ name / aliases / version / variant_of
├─ purpose                   为什么做，产生什么变化
├─ stage                     接种/处理/洗涤/分离/培养/测量/成像/取样
├─ action                    明确动词 + 对象
├─ inputs[]                  样本、材料、容器、仪器、文件
├─ outputs[]                 材料、状态、数据
├─ parameters[]              key/type/unit/range/required/source/layer
├─ constraints[]             容量、兼容性、温度、时间、校准、生物安全
├─ controls[]                类型、用途、适用 assay、是否必需
├─ decision_points[]         问题、证据、分支、停止/重做条件
├─ checks[]                  deterministic / operational / advisory
├─ failure_modes[]           现象、原因、影响、处理
├─ readout_meaning           原始值、派生值、不能推出什么
├─ sample_use                终末/可复测/可分样/需固定
├─ instrument_class/profile  仪器能力，不是具体机器状态
├─ evidence[]                URL、手册版本、SOP、验证记录
└─ execution_fields[]        Run 中要记录的实际值/文件/偏差
```

使用层：

- `ProtocolTemplate`：有序步骤 + 参数绑定 + 对照 + 版本。
- `BlockUse`：某次插入位置；同一块用两次是两个使用，不复制内容。
- `Run`：冻结 Protocol/步骤版本；实际设置、操作者、时间、偏差、原始文件。
- `InstrumentProfile`：仪器能力档案；未知型号时只显示通用字段。

参数必须标注归属：`protocol_fixed`、`design_variable`、`run_actual`。同一个数字如果在不同层出现，不能互相覆盖。例如“10 µL CCK-8”是 Protocol 示例参数；“每孔 5,000 细胞”是设计/方法参数；“实际加入 9.8 µL”是 Run 实际值。

## 3. 代表性实验的步骤知识

以下不是“唯一正确 Protocol”，而是展示每一步的目的、关键参数、对照和边界。具体值来自某一来源，采用前必须按实验室 SOP 复核。

### 3.1 细胞培养、传代与接种

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 弃培养基、洗涤 | 去除血清/抑制剂，为解离做准备 | 无 Ca²⁺/Mg²⁺ 平衡盐溶液；体积覆盖细胞层 |
| 加解离试剂 | 让贴壁细胞脱离 | 例如 0.5 mL/10 cm²；室温约 2 min，按细胞系调整 |
| 观察脱离 | 避免过度消化 | 若 <90% 脱离，延长/轻拍；不要只看时间 |
| 加完全培养基 | 中和/稀释解离试剂 | 约为解离试剂体积的 2 倍 |
| 离心 | 去除解离试剂/碎片 | 例如 200×g，5–10 min；按细胞类型调整 |
| 重悬、计数 | 知道活细胞密度 | 台盼蓝/自动计数；活率通常应 >90% |
| 接种 | 进入下一步实验 | 按推荐密度/体积；不是所有细胞同一密度 |

**不能推断**：汇合度不是细胞数；传代比例不是独立重复；细胞计数不是处理组分配；不同细胞系的解离时间/离心条件不能互相套用。

来源：Thermo Fisher 贴壁细胞传代与解离协议。

### 3.2 CCK-8 细胞活力/增殖

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 铺板 | 建立可比较的细胞基数 | 例如 96 孔板 100 µL/孔、5,000 cells/well（仅示例）；预培养 |
| 处理 | 施加实验条件 | 处理浓度/时间来自设计；不是 CCK-8 固定值 |
| 孵育 | 让处理产生效应 | 6–48 h 视实验；不能统一写“过夜” |
| 加 CCK-8 | WST-8 被活细胞脱氢酶还原 | 例如 10 µL/孔；避免气泡；混匀 |
| 显色孵育 | 形成可读吸光度 | 37°C、5% CO2、1–4 h；可按动力学选时间 |
| 读板 | 得到原始吸光度 | 450 nm（430–490 nm 可接受）；空白/背景校正 |
| 计算 | 相对活力/标准曲线 | 需要空白、对照、标准曲线；不能把 OD 直接叫活细胞数 |

**对照**：无细胞空白（培养基 + CCK-8）、未处理对照、载体/溶剂对照（如适用）。空白不是“0 细胞”这么简单，它同时反映试剂本底和培养基颜色。

**失败模式**：气泡、边缘蒸发、培养基/药物颜色、还原性物质、细胞过密超出线性、孵育过短/过长、CCK-8 未混匀。若背景高，用 ≥600 nm 参考波长校正；具体按手册和仪器。

**读数边界**：CCK-8 测的是脱氢酶活性，不是直接计数，也不等于“毒性”或“增殖”单独一项；需要标准曲线/时间过程才能解释。

来源：Dojindo CCK-8 技术手册 CK04e。

### 3.3 LIVE/DEAD 荧光活力

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 培养/处理 | 形成待测细胞 | 贴壁/悬浮、容器按成像方式 |
| 配染色液 | 同时标记活/死细胞 | 示例：5 µL calcein AM + 20 µL EthD-1 加到 10 mL DPBS |
| 去培养基、加染液 | 让染料接触细胞 | 100–200 µL/孔或覆盖细胞层 |
| 孵育 | 活细胞酯酶切割 calcein AM；死细胞膜通透让 EthD-1 进入 | 20–25°C、30 min、避光；浓度需按细胞优化 |
| 成像 | 获得双通道图像 | calcein ~494/517（GFP/FITC）；EthD-1 ~528/617（RFP） |
| 判定 | 区分活/死 | 需要死细胞对照、未染色对照；不能只凭颜色 |

**关键边界**：染料不是固定/通透化后仍可用的通用染色；固定会改变结果。死细胞对照可用热杀死（例如 60°C、20 min）或膜破坏，但具体按试剂盒。图像视野不是独立 n；同一孔多个视野是技术采样。

**失败模式**：染料浓度过高/过低、背景、光漂白、细胞外酯酶、固定、孔边缘、曝光不一致。

来源：Thermo Fisher LIVE/DEAD L3224 协议与 FAQ。

### 3.4 划痕/伤口愈合

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 铺板 | 形成汇合单层 | 例如 ibidi insert：3×10⁵ cells/mL、70 µL/well、24 h；按细胞生长调整 |
| 确认汇合 | 划痕前基线一致 | 显微镜检查；不汇合则延长/换液 |
| 去插入物/划痕 | 形成无细胞区域 | 插入物可重复；枪头划痕变异大、可能损伤 ECM |
| 洗涤 | 去除碎片/脱落细胞 | 温和 PBS/无血清培养基；避免冲散单层 |
| 加低血清/处理 | 观察迁移而非增殖 | 常用低血清；是否加增殖抑制剂需说明 |
| 0 h 成像 | 记录起始缺口 | 同一位置/同一视野 ID；低倍宽视野 |
| 后续成像 | 计算闭合 | 6/12/24 h 等按实验；同一位置、同一条件 |

**不能推断**：缺口闭合不等于“迁移增加”，也可能来自增殖、细胞变大或碎片；没有同一视野 ID 不能声称追踪同一视野；划痕宽度不是细胞数。

来源：ibidi 伤口愈合应用说明与实验流程。

### 3.5 ELISA

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 包被 | 固定捕获抗体 | 例如 100 µL/孔，4°C 过夜；缓冲液/浓度按抗体 |
| 洗涤 | 去除未结合 | 例如 400 µL/孔，重复；拍干 |
| 封闭 | 降低非特异结合 | 例如 300 µL/孔、室温 60 min |
| 加标准品/样本 | 让抗原被捕获 | 100 µL/孔、室温 2 h；标准曲线/稀释 |
| 加检测抗体 | 形成夹心 | 100 µL/孔、室温 2 h；抗体浓度按试剂盒 |
| 加底物 | 酶促显色 | 100 µL/孔、室温 20–30 min、避光 |
| 终止/读板 | 固定信号 | 50 µL 终止液；HRP/TMB 450 nm，AP/PNPP 405 nm |

**对照**：标准曲线、空白、阴性/阳性、复孔；样本基质/稀释线性。不能只用一个“对照孔”。

**失败模式**：钩状效应、洗涤不足、交叉反应、边缘蒸发、气泡、底物过显、样本稀释不当。

**读数边界**：OD 不是浓度；浓度来自标准曲线和稀释倍数；不同试剂盒/抗体不能直接比。

来源：R&D Systems 夹心 ELISA 协议。

### 3.6 Western blot

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 裂解 | 提取可溶蛋白 | 冷裂解液 + 蛋白酶抑制剂；体积按沉淀大小 |
| 定量 | 等量上样 | BCA 等；标准曲线；空白校正 |
| SDS-PAGE | 按分子量分离 | 胶浓度按目标蛋白；上样量/对照 |
| 转膜 | 把蛋白转到膜 | PVDF/硝酸纤维素；恒流/恒压按设备 |
| 封闭 | 降低背景 | 5% 脱脂奶/BSA in TBST，约 1 h |
| 一抗 | 识别目标 | 例如 1–10 µg/mL，4°C 过夜；按抗体优化 |
| 洗涤 | 去未结合 | 例如 3×5 min TBST |
| 二抗 | 信号放大 | HRP/荧光二抗，约 1 h；避光（荧光） |
| 检测 | 成像 | 化学发光/荧光；避免过曝 |

**对照**：分子量 ladder、loading control（如 actin/GAPDH，但需确认在条件下稳定）、阳性/阴性样本、转膜效率。不能把“有条带”直接当目标蛋白，需分子量/抗体验证。

**失败模式**：降解、上样不均、转膜不全、封闭不匹配、抗体非特异、洗涤不足、剥离/重新探测损伤。

来源：Addgene、Abcam、Sigma 的 Western blot 协议。

### 3.7 RT-qPCR

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 样本/RNA 提取 | 获得可扩增核酸 | RNase-free；产量/纯度/完整性 |
| RNA 质量 | 排除抑制/降解 | A260/280、凝胶/RIN 等；按样本类型 |
| 逆转录 | RNA → cDNA | 随机引物/oligo(dT)、RT 酶、RT 条件；±RT 对照 |
| qPCR 反应 | 扩增目标 | 引物/探针、master mix、体积、通道 |
| 扩增 | 实时荧光 | 循环条件、退火温度、延伸；熔解曲线 |
| 分析 | Cq/相对或绝对量 | 标准曲线效率、参考基因、NTC/无 RT 对照 |

**关键边界**：Cq 不是表达量；相对表达依赖参考基因和效率；绝对定量依赖标准曲线；无 RT/NTC 对照用于排除污染/基因组 DNA；MIQE 要求报告引物、扩增效率、参考基因、逆转录和统计等。

**失败模式**：RNA 降解/抑制剂、引物二聚体、非特异扩增、效率差、参考基因不稳定、孔间污染、错误基线/阈值。

**变体**：一步法 vs 两步法；SYBR vs 探针；相对 vs 绝对定量。不能把一套循环条件套所有引物。

来源：MIQE 指南、Bio-Rad qPCR 学习中心与 qPCR 对照资料。

### 3.8 流式细胞术免疫表型

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 样本制备 | 获得单细胞悬液 | 组织消化/血液裂解；避免团块/死细胞 |
| Fc 阻断 | 降低非特异结合 | 按物种/样本；不是所有面板都必须 |
| 活力染料 | 排除死细胞 | 膜完整/胺反应染料；按通道 |
| 表面染色 | 标记表面抗原 | 抗体滴定；冰上/避光；按 panel |
| 固定/通透/胞内染色 | 标记胞内抗原 | 固定/通透剂必须与染料/抗体兼容 |
| 补偿 | 扣除溢漏 | 单染补偿微球/细胞；不能只用未染 |
| 采集 | 获取事件 | 流速、事件数、阈值、CS&T/QC |
| 分析 | 设门/百分比/MFI | FMO、同型、生物学对照；双联体排除 |

**关键边界**：事件数不是细胞数；百分比依赖设门；MFI 不是绝对分子数（除非定量微球）；补偿不等于 FMO；分析型不等于分选型。

**失败模式**：补偿不当、死细胞、双联体、非特异结合、panel 溢漏、低事件数、固定后抗原丢失。

来源：BD FACSCanto II 技术规格、BD/Bio-Rad 流式对照资料。

### 3.9 免疫荧光/显微成像

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 接种/贴片 | 让细胞可成像 | 玻底皿/盖玻片；密度按视野 |
| 固定 | 保存形态/抗原 | 4% PFA 15 min（示例）；甲醇是另一种变体 |
| 通透 | 让抗体进入 | 0.1% Triton 10 min（示例）；按抗原定位 |
| 封闭 | 降低背景 | 1 h 室温；BSA/血清 |
| 一抗 | 识别目标 | 4°C 过夜或室温 1–2 h；按抗体 |
| 洗涤 | 去未结合 | PBS 3×5 min |
| 二抗 | 荧光标记 | 1–2 h 室温、避光；种属匹配 |
| 复染/封片 | 核定位/保存 | DAPI；封片剂；避光 |
| 成像 | 采集 | 物镜、通道、曝光、Z-stack、位置 |

**对照**：仅二抗、同型、阳性组织/细胞、未染色（自发荧光）。不能把“有荧光”直接当目标表达；需抗体验证/对照。

**失败模式**：固定/通透不兼容、抗体浓度、背景、光漂白、串色、离焦、视野选择偏倚。

来源：Cell Signaling Technology、Proteintech、Sigma 的 IF/ICC 协议。

### 3.10 生物膜 CV / CFU

| 步骤 | 目的 | 关键参数/判定 |
|---|---|---|
| 培养生物膜 | 形成附着群落 | 96/24 孔；培养基/时间/温度按菌种 |
| 去浮游菌 | 只测附着生物膜 | 轻柔弃液；洗涤次数/力度一致 |
| CV 染色 | 测总生物量 | 例如 0.1% CV 10–15 min；洗涤去多余染料 |
| 溶解/读板 | 定量染料 | 30% 乙酸或乙醇；590–600 nm |
| 或 CFU：刮取/超声 | 分散附着细胞 | 刮壁/超声；验证分散效率 |
| 系列稀释/点板 | 计数活菌 | 10 倍稀释；计 5–50 个菌落/点；CFU/mL |
| 分析 | 生物量 vs 活菌 | CV 与 CFU 不可互换 |

**对照**：无菌培养基、未处理生物膜、空白孔、分散效率对照。CV 会被培养基/材料/染料结合干扰；CFU 受分散/稀释/计数范围影响。

**失败模式**：洗涤冲散生物膜、CV 结合非细胞成分、超声过度/不足、稀释不在计数范围、菌液聚集。

来源：PMC 静态生物膜/CV 协议与 CV+CFU 方法论文。

### 3.11 读板机测量（通用边界）

| 参数 | 要确认什么 |
|---|---|
| 波长/滤光片 | 与染料/底物匹配；不是所有孔板都同一波长 |
| 板型 | 6/48/96/384；透明/白色/黑色；底高 |
| 读数模式 | 终点/动力学/光谱；时间点/间隔 |
| 空白/背景 | 无细胞/无分析物/仅试剂 |
| 线性范围 | 吸光度/荧光是否饱和；稀释 |
| 温度/振荡 | 是否需要；对动力学的影响 |
| 光程/体积 | 微量/低体积需要校正 |

**读数边界**：原始 OD/荧光不是浓度/活力/菌数；必须通过标准曲线、对照和已知稀释解释。

来源：Multiskan SkyHigh 规格表。

### 3.12 动物与临床研究的边界

动物实验不是“带孔板的细胞实验”：

- ARRIVE 2.0 要求报告样本量、纳入/排除、随机化、盲法、结果测量、伦理等；这些属于设计和报告，不是 Protocol 步骤的替代。
- 随机化单位是动物/笼/窝，不是孔；样本量估算依赖效应量和变异，不是 UI 默认值。
- 影像/组织学步骤需要麻醉、成像、取材、病理、盲法读片等独立模块；软件不能从细胞 Protocol 直接推导。
- 临床队列/MRI 研究涉及受试者、伦理、知情同意、标签定义、训练/验证按患者拆分；不应把孔板/动物模块硬套。

本轮不展开动物/临床专属字段；只明确它们需要独立的设计对象和验证边界。

### 3.13 覆盖边界

已覆盖：细胞培养/传代、CCK-8、LIVE/DEAD、划痕、ELISA、Western、RT-qPCR、流式、IF、CV/CFU、读板机，以及动物/临床边界。

未覆盖但不应用孔板模型硬套：分子克隆/构建、测序建库、质谱、类器官/3D 培养、微流控、动物手术/行为学、临床试验、影像组学。它们需要各自的步骤块和安全/伦理字段，不能靠通用“实验模板”假装理解。

## 4. 跨实验的五个硬结论

### 4.1 对照是检测的一部分，不是全局字段

CCK-8 需要无细胞空白；LIVE/DEAD 需要死细胞/未染色对照；ELISA 需要标准曲线/空白；qPCR 需要 NTC、无 RT、参考基因；流式需要单染、FMO、活力；IF 需要仅二抗/同型；CFU 需要无菌/未处理/分散对照。

因此 `controls[]` 必须绑定到 Assay/Protocol，并说明用途。没有“通用对照清单”这回事。

### 4.2 样本使用政策决定时间线

- CCK-8 在部分 Protocol 中可在同一孔做多个时间点（例如测后吸液补液），但这依赖试剂、细胞状态和实验室验证；不能自动假设所有后续检测都适合。
- LIVE/DEAD 通常是终末/成像检测；固定后染料可能失效。
- 划痕需要同一视野追踪；没有位置 ID 就没有同视野证据。
- ELISA/CFU/qPCR 往往消耗/破坏样本；同一孔不能同时服务多个终末检测。
- 流式染色/固定会改变样本，后续培养/分选需明确。

现有 `sample_use` 不能只有“破坏性/非破坏性”布尔值，至少要有：可重复测量、终末、可分样、需固定/通透、需活细胞、可回收。适用性来自 Protocol/实验室规则/用户确认，不由软件猜。

### 4.3 时间锚点属于每个检测/步骤

“0 h”可能是处理开始、划痕完成、染色开始、成像开始、 sampling 开始。CCK-8 的 D1/D3/D7 与活死染色的 D1/D3/D7 可能不是同一批样本，也不是同一锚点。时间不能只存一份全局数组。

### 4.4 读数不是结论

| 读数 | 能说 | 不能直接说 |
|---|---|---|
| OD450（CCK-8） | 该孔在特定条件下的脱氢酶活性信号 | 活细胞数、毒性、增殖 |
| 荧光（LIVE/DEAD） | 染料通道下的膜完整性/酯酶信号 | 绝对活率、所有细胞 |
| 划痕闭合率 | 图像上无细胞区域缩小 | 迁移增强（可能增殖/变形） |
| ELISA OD | 标准曲线下的结合信号 | 样本浓度（需曲线/稀释） |
| Western 条带 | 目标分子量处的抗体信号 | 绝对蛋白量（需标准/对照） |
| Cq | 扩增曲线到达阈值的 cycle | 表达量（需效率/参考基因） |
| 流式百分比 | 设门内事件比例 | 绝对细胞数、体内比例 |
| IF 荧光 | 抗体定位/强度 | 蛋白绝对量、功能 |
| CV OD595 | 染料结合的总生物量 | 活菌数 |
| CFU | 可培养活菌数 | 总生物量、所有活细胞 |

### 4.5 变体不能合并

- CCK-8：96/384 孔、贴壁/悬浮、终点/动力学。
- LIVE/DEAD：显微镜/流式、不同染料套件。
- 划痕：枪头 vs 插入物；低血清 vs 增殖抑制剂。
- ELISA：夹心/直接/竞争；HRP/AP；不同样本基质。
- Western：化学发光/荧光；湿转/半干。
- qPCR：一步/两步；SYBR/探针；相对/绝对。
- 流式：分析/分选；表面/胞内；不同激光/滤光片。
- IF：固定/通透/活细胞；不同抗体。
- CFU/CV：不同菌种/培养基/清洗/分散。

软件应把这些作为 Protocol 变体/参数，而不是一个“万能 assay”加自由文本。

## 5. 相关项目：它们强在哪，缺在哪

| 项目 | 对象模型 | 步骤/执行 | 可借鉴 | 不能照搬 |
|---|---|---|---|---|
| Opentrons Protocol Designer | deck/labware/pipette/module + protocol steps | 无代码步骤：transfer/mix/pause/module/move；直接生成机器人可执行协议 | 步骤类型化、模块步骤、参数表单、仿真/运行 | 只服务 Opentrons 机器人；不表达科学设计、对照、n、样本关系 |
| protocols.io | protocol + materials + steps/substeps + runs | 协议编辑、材料清单、 checklist run、计时器、版本/DOI | 步骤+材料+Run 分层；Run 是执行记录；可分享/引用 | Run 是 checklist，不是实验设计/样本分配；不解决仪器能力 |
| Benchling | Notebook entries/protocols/templates；Runs 是仪器/数据 pipeline | Notebook 模板/子模板；结构化表；Runs 处理仪器文件/结果 | 模板复用、结构化表、Run 输入/输出/文件、审计 | Benchling Runs 主要是 instrument/data run，不是湿实验步骤执行；不替代设计 |
| SciNote | Project/Experiment/Task；Protocol metadata + steps | 步骤含 title/description/checklist/table/attachment；步骤↔结果链接 | 细步骤、checklist 审计、步骤-结果关联、开源 | 步骤语义偏文档；仪器/参数模型弱；不解决科学设计 |
| Labguru | Protocol 模板 → Experiment 复制；ELN/LIMS | 协议含步骤、表单、样本、设备、板设计；实验从协议复制 | 模板实例化、样本/设备/板设计结合 | 复制式模板；版本/参数语义弱；不解决设计关系 |
| eLabFTW | Experiments/Resources/Templates；API | 实验记录、模板、步骤、审计/修订、锁定/时间戳 | 开源、审计、API、模板、权限 | 不是 protocol-first 执行引擎；步骤模型轻 |
| RSpace | Documents/Notebooks/Forms/Templates/Inventory | 文档/表单/模板、 inventory、provenance、RO-Crate 导出 | 结构化表单、 inventory、导出/ provenance | 不是步骤级实验协议；仪器执行弱 |
| openBIS | Space→Project→Experiment→Object→Dataset；Methods | 样本/方法/数据集、父子关系、实验记录 | 强 lineage、方法库、数据集关联 | 重、偏数据管理；不适合直接做桌面交互 |

结论：现有产品要么强在**执行/记录**（Opentrons、protocols.io、Benchling Runs），要么强在**文档/库存/审计**（ELN/LIMS），很少同时把“科学设计关系 + 可执行步骤 + 仪器能力 + 未知/版本”放在一个轻量界面里。AsterLab 的机会不是再做一个 ELN，而是把设计对象和 Protocol 步骤真正连起来。

## 6. 标准：借模式，不借整套本体

| 标准 | 它模型什么 | 可借 | 代价/边界 |
|---|---|---|---|
| ISA Abstract Model | Investigation/Study/Assay；Process→Material/Data 的 DAG；参数值/执行者/日期 | 样本 lineage、过程输入/输出、参数值属于过程 | 偏元数据交换，不是 UI/执行；全套实现重 |
| OBI | Planned process、plan specification、protocol、input/output | “过程/计划/输入输出”的精确语义 | 本体工程重；不适合直接当产品 schema |
| EXPO/EXACT | 实验动作、目标与“怎么做”分离 | 动作分类/目标导向；不要把动作名当目的 | 研究性强、采用少 |
| SiLA 2 | Feature/Command/Parameter/Response；仪器能力 | 仪器能力档案、命令参数、可观察执行 | 需要仪器支持；不是实验设计标准 |
| AnIML / Allotrope ADF | 分析仪器数据/结果/审计 | 原始文件元数据、审计/校验和 | 文件格式重；不是 Protocol 模型 |
| OME-NGFF/XML | 生物成像数据/元数据 | 图像维度、通道、位置、像素元数据 | 只管成像数据，不管实验步骤 |
| RO-Crate | 研究对象打包 + 元数据/ provenance | 导出/归档、数据+代码+文档打包 | 不是运行时模型 |
| W3C PROV | Entity/Activity/Agent | 审计、谁在何时用什么产生了什么 | 通用 provenance，不含实验语义 |
| MIQE | qPCR 最低报告信息 | qPCR 对照、效率、参考基因、报告字段 | 是 checklist，不是数据格式 |
| ARRIVE 2.0 | 动物研究最低报告/设计信息 | 随机化、盲法、样本量、伦理、排除 | 是报告指南，不是动物 Protocol 引擎 |

推荐：内部用“小核心”表达 `Process/Input/Output/Parameter/Provenance`；需要交换时映射到 ISA/RO-Crate/PROV；需要仪器集成时用 SiLA 2 能力模型；需要报告时用 MIQE/ARRIVE 检查表。不要一开始就实现 OBI/EXPO 全套。

## 7. AsterLab 应该怎么落地

### 7.1 先建“步骤知识库”，不是“万能生成器”

- 每个步骤块有 `purpose`、`inputs/outputs`、`parameters`、`controls`、`decision_points`、`failure_modes`、`readout_meaning`、`sample_use`、`evidence`。
- 首批覆盖：细胞培养/传代、CCK-8、LIVE/DEAD、划痕、ELISA、Western、RT-qPCR、流式、IF、CFU/CV、读板。
- 每个 assay 至少保留一个“来源协议”和一个“实验室 SOP 变体”；不合并变体。
- 没有来源的参数留空；示例值只能作为“示例”，不能成为默认。

### 7.2 步骤插入仍然是唯一入口

仪器/操作为步骤模块，插入 Protocol 的位置；不弹“是否使用某仪器”，不先跳到工具中心。详见 [`instrument-step-modules.md`](../prototypes/instrument-step-modules.md)。

### 7.3 三层校验

| 层 | 例子 | 行为 |
|---|---|---|
| 结构完整性 | 类型/单位/版本/引用损坏 | 拒绝写入，字段级错误 |
| 操作可行性 | 孔体积超限、板型不匹配、缺仪器 | 可保存草案；阻止受影响的冻结/Run |
| 科学建议 | 缺对照、n 依据不清、处理与批次混杂 | 给依据/影响；用户可确认、暂不处理或不适用 |

科学建议不能因为“软件觉得不对”就禁止保存新实验。现有产品原则“未知 ≠ 否/零/不适用”要继续保持。

### 7.4 计划/实际分开

- Protocol 参数、设计变量、Run 实际值分别存储。
- 仪器模块的原始文件/图像/板文件进入 Run，派生结果单独存。
- 执行偏差链接到具体步骤/样本/结果；不能覆盖计划。
- 完成状态来自实际记录，不从“有 Protocol”推导。

### 7.5 对“懂所有实验”的产品约束

- 不自动补剂量、时间、温度、转速、波长、细胞密度。
- 不把一种 Protocol 当所有实验室的标准。
- 不把读数自动翻译成生物结论。
- 不把技术重复当独立 n。
- 不把计划输出写成实际材料。
- 不因未知/不适用/新实验而拒绝保存草案。

## 8. 验证计划（下一步，不是已做）

1. **桌面专家评审**：每个 assay 找 1–2 名实际做该实验的人，逐步骤问“目的/参数/对照/失败模式”，记录无法回答的字段；不把评审当用户测试结论。
2. **真实 Protocol 走查**：用实验室现有 SOP + 一份厂商协议，比较步骤差异；确认软件是否错误合并变体。
3. **干运行**：用 L929 CCK-8、活死、划痕、SA 铜梯度、生物膜案例，检查步骤插入、参数缺失、样本使用、Run 记录是否说得清。
4. **执行影子**：跟随 1–2 次真实实验，记录实际偏差/临时操作；验证软件是否允许记录而不是阻止。
5. **指标**：缺失参数数量、错误对照数量、变体混淆次数、需要专家纠正的步骤数、Run 记录完整度；不以“生成速度”作为主要指标。

## 9. 非目标

- 不建立通用工作流/脚本引擎。
- 不实现完整 OBI/EXPO/ISA 本体。
- 不覆盖所有实验类型；先覆盖有真实案例和来源的 assay。
- 不把动物/临床研究塞进孔板模型。
- 不声称已做用户测试或已证明可用性提升。

## 10. 来源

### 实验协议与仪器

- Thermo Fisher：贴壁细胞传代、解离、细胞计数。
  - https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/subculturing-adherent-cells.html
  - https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/cell-dissociation-protocol.html
- Dojindo CCK-8 技术手册 CK04e：https://dojindo.com/products/manual/CK04e.pdf
- Thermo Fisher LIVE/DEAD L3224：https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/live-dead-viability-cytotoxicity-kit-for-mammalian-cells.html
- ibidi 伤口愈合应用说明：https://ibidi.com/img/cms/downloads/an/AN21_Wound_Healing_Assay.pdf
- R&D Systems 夹心 ELISA：https://www.rndsystems.com/resources/protocols/elisa-direct-sandwich-protocol
- Addgene Western blot：https://www.addgene.org/protocols/western-blot/
- Abcam Western blot：https://www.abcam.com/en-us/technical-resources/protocols/western-blot
- Bio-Rad qPCR 学习中心：https://www.bio-rad.com/en-us/applications-technologies/real-time-pcr-qpcr-learning-center
- MIQE：https://academic.oup.com/clinchem/article/55/4/611/5631762
- BD FACSCanto II 技术规格：https://www.bdbiosciences.com/content/dam/bdb/marketing-documents/facscanto_techspecs.pdf
- BD 流式对照：https://www.bdbiosciences.com/en-eu/learn/science-thought-leadership/blogs/flow-cytometry-controls-tips-for-better-data
- Cell Signaling IF：https://www.cellsignal.com/learn-and-support/protocols/protocol-if-cell-based-imaging
- Proteintech IF：https://www.ptglab.com/support/immunofluorescence-protocol/if-staining-protocols/
- 静态生物膜/CV/CFU：https://pmc.ncbi.nlm.nih.gov/articles/PMC4568995/
- Multiskan SkyHigh 规格：https://etconanalytical.com/wp-content/uploads/2020/10/Multiskan-SkyHigh-Microplate-Spectrophotometer-Specifications-.pdf

### 软件与标准

- Opentrons Protocol Designer：https://docs.opentrons.com/protocol-designer/
- Opentrons transfer steps：https://docs.opentrons.com/protocol-designer/steps/transfer/
- Opentrons module steps：https://docs.opentrons.com/protocol-designer/steps/module/
- protocols.io 编辑器：https://www.protocols.io/help/editor
- protocols.io Run：https://www.protocols.io/help/ongoing-research-work/run
- Benchling 模板：https://help.benchling.com/hc/en-us/articles/39939099777933-Configure-Notebook-templates-and-Template-Collections
- Benchling Runs：https://help.benchling.com/hc/en-us/articles/38247110160269-How-to-use-Runs
- SciNote 协议结构：https://www.scinote.net/blog/how-to-structure-a-protocol/
- SciNote 步骤-结果链接：https://knowledgebase.scinote.net/en/knowledge/how-to-link-protocol-steps-and-results
- Labguru 协议：https://help.labguru.com/en/articles/5469376-creating-and-using-protocols
- eLabFTW 文档：https://doc.elabftw.net/
- eLabFTW API：https://doc.elabftw.net/api/v2/
- RSpace 文档：https://documentation.researchspace.com/l/en
- openBIS 数据模型：https://openbis.readthedocs.io/en/latest/user-documentation/advance-features/openbis-data-modelling.html
- ISA Abstract Model：https://isa-specs.readthedocs.io/en/latest/isamodel.html
- OBI core classes：https://obi-ontology.org/docs/core-classes/
- EXACT/EXPO：https://www.aber.ac.uk/en/cs/research/cb/dss/exact/
- SiLA 2 标准：https://sila-standard.com/standards/
- AnIML：https://animl.org/
- Allotrope Framework：https://docs.allotrope.org/
- OME-NGFF：https://ngff.openmicroscopy.org/specifications/
- RO-Crate：https://www.researchobject.org/ro-crate/
- W3C PROV：https://www.w3.org/TR/prov-overview/
- ARRIVE 2.0：https://arriveguidelines.org/arrive-guidelines

链接核对：部分厂商站点对自动请求返回 403 或超时（Thermo、BD、Bio-Rad、Benchling、Labguru、eLabFTW、RSpace、AnIML、R&D、Abcam 等）；这些页面在研究中已通过检索摘要或页面内容核对，正式采用前仍需人工打开复核版本与许可。
