# K0 知识覆盖审计:实验、步骤、仪器与证据缺口

- 日期:2026-11-05
- 阶段:K0;只做审计,不改 UI、不新增运行时、不导入模板。
- 审计范围:仓库现有研究文档、Core 契约、`src/cli.rs`、`crates/asterlab-core` 与已有测试说明。
- 重要限制:本轮没有逐页打开所有厂商 PDF,也没有实验室专家复核。除代码事实外,科学内容一律**不能**标为 `Source Checked`;现有研究结论只算 `Research Draft`。

## 0. 结论

研究材料覆盖了实验类型、步骤目的、对照、失败模式和仪器规格,但还没有形成可编译、可验证的知识模块。当前最准确的描述是:

> 有领域调研和产品规格草案;没有逐步骤的机器可读 OperationModule,没有版本化 InstrumentProfile,没有 Catalog Compiler,也没有任何 Lab Validated 模板。

现有实现的真实能力是:Design、ProtocolPreview、RunWorkspace 三条只读链路;参数槽、步骤块、递归组合、Run 冻结已存在。缺口不是数据库表,而是**契约、编译/校验、证据分级和测试样例**。

## 1. 代码事实(已核对)

| 事实 | 位置 | 对知识体系的影响 |
|---|---|---|
| `ParameterSlot` 只有 `key/label/value_type/unit/required` | `src/cli.rs:488` | 没有 `source`、`required_at`、范围、枚举、证据;不能区分协议固定值与示例值 |
| `ParameterValueType` 只有 text/number/integer/boolean | `src/cli.rs:482` | 没有量纲、枚举、文件/实体引用;单位只做字符串完全匹配 |
| `ProtocolStep` 只有 `Text` 与 `BlockUse` | `src/cli.rs:522` | 没有 input/output/port、sample effect、instrument requirement、review status |
| `ProtocolStep::Text` 有 `time_scope` | `src/cli.rs:533` | 时间作用域已有基础;但没有步骤级语义和来源 |
| `ProtocolBlockVersionData` 只有 summary/parameter_slots/steps | `src/cli.rs:591` | 没有模块身份、适用性、审核状态、证据 |
| `validate_block_content` 只校验 ID、参数引用、子块版本和绑定 | `src/cli.rs:694` | 不校验科学语义、能力兼容、证据完整性 |
| `expand_block` 复制 title/instruction/parameters/visualization/time_scope | `src/cli.rs:851` | 没有 `meta`;没有模块契约展开 |
| `ProtocolPreview` / `OperationStep` 已有类型化投影 | `crates/asterlab-core/src/protocol_preview.rs:11` | 读模型基础可用;仍缺步骤语义 |
| `RunWorkspace` 已分离 steps/tasks/occurrences | `crates/asterlab-core/src/run_workspace.rs:12` | 计划/实际边界已有;没有 instrument actual 字段 |
| `DesignAssayDraft` 有 method/endpoint/unit/timepoints | `crates/asterlab-core/src/design_commands.rs:38` | 检测方法仍是字符串,没有 variant/Protocol 绑定结构 |
| 没有 `StepMeta`、`InstrumentProfile`、`ToolAction`、`OperationModule` 的 Rust 类型 | 全仓库 | 这些目前只是文档概念,不是已实现能力 |

## 2. 实验覆盖矩阵

状态定义:

- `R0` Research Draft:有文档整理,但未逐字段对照一手来源。
- `R1` Source Linked:至少有一个可打开的一手/权威来源链接;尚未逐页核对。
- `S0` Source Checked:逐字段对照来源、记录版本/页码/适用条件。**当前没有任何条目达到。**
- `E0` Expert Reviewed / `L0` Lab Validated:**当前没有任何条目达到。**

| 实验/流程 | 现有内容 | 变体是否分开 | 逐步骤契约 | 现有来源 | 状态 | 主要缺口 |
|---|---|---|---|---|---|---|
| 细胞培养/传代/接种 | 弃液、洗涤、解离、离心、重悬、计数、接种 | 部分(贴壁为主) | 无 | Thermo Fisher 细胞培养协议 | R1 | 细胞系差异、解离时间、计数/活率判定未结构化 |
| CCK-8 | 铺板、处理、孵育、加试剂、显色、读板、计算 | 部分(96/384、终点/动力学提到) | 无 | Dojindo CK04e;现有研究引用 | R1 | 试剂体系/细胞类型/时间点未形成变体矩阵;450 nm 等值未逐字段核对 |
| LIVE/DEAD | 配液、染色、孵育、成像、判定 | 部分(显微镜/流式提到) | 无 | Thermo Fisher L3224 | R1 | 染料套件、固定兼容性、视野/独立 n 边界未结构化 |
| 划痕/伤口愈合 | 铺板、划痕、洗涤、低血清、0 h/后续成像 | 部分(枪头/插入物) | 无 | ibidi 应用说明 | R1 | 同视野 ID、增殖混杂、时间锚点未结构化 |
| ELISA | 包被、洗涤、封闭、标准品/样本、检测抗体、底物、终止/读板 | 部分(夹心;HRP/AP 提到) | 无 | R&D Systems 夹心 ELISA | R1 | 曲线拟合、稀释线性、基质效应、钩状效应未结构化 |
| Western blot | 裂解、定量、SDS-PAGE、转膜、封闭、一抗、洗涤、二抗、检测 | 部分(化学发光/荧光) | 无 | Addgene/Abcam/Sigma | R1 | 抗体验证、上样对照、转膜效率、曝光边界未结构化 |
| RT-qPCR | 提取、质量、逆转录、qPCR、扩增、分析 | 部分(一步/两步;SYBR/探针) | 无 | MIQE;Bio-Rad | R1 | 引物/效率/参考基因/对照未形成必填契约 |
| 流式免疫表型 | 样本制备、Fc 阻断、活力、表面/胞内染色、补偿、采集、分析 | 部分(分析/分选) | 无 | BD FACSCanto II;BD 对照资料 | R1 | panel、激光/滤光片、补偿/FMO、事件数边界未结构化 |
| IF/显微成像 | 接种、固定、通透、封闭、一抗、洗涤、二抗、复染、成像 | 部分(固定/通透/活细胞) | 无 | CST/Proteintech | R1 | 通道/曝光/物镜/视野元数据未结构化 |
| 生物膜 CV/CFU | 培养、去浮游菌、CV 染色/溶解/读板;或刮取/超声/稀释/点板 | 部分(CV vs CFU) | 无 | PMC 静态生物膜协议 | R1 | 分散效率、计数范围、CV/CFU 不可互换未结构化 |
| 读板机测量 | 波长、板型、模式、空白、线性、温度/振荡 | 通用边界 | 无 | Multiskan SkyHigh 规格 | R1 | 型号能力与协议要求未分离 |
| 动物/临床 | 只写了边界,没有步骤模板 | 不适用 | 无 | ARRIVE 2.0 等 | R0 | 不能复用孔板模型;需要独立对象与伦理/随机化字段 |

## 3. 操作模块覆盖矩阵

| 模块 | 现有文档内容 | 机器可读定义 | 参数契约 | 正/反例测试 | 状态 |
|---|---|---|---|---|---|
| 液体转移 | 体积、通道、吸头、源/目标、混匀 | 无 | 无 | 无 | R1 |
| 接种/分配 | 密度、体积、板型、孔数 | 无 | 无 | 无 | R1 |
| 孵育/培养 | 温度、CO2、湿度、时间、振荡、锚点 | 无 | 无 | 无 | R1 |
| 洗涤/吸液 | 洗液、体积、循环、浸泡、残留 | 无 | 无 | 无 | R1 |
| 取样/分样 | 体积、来源、目标、样本去向 | 无 | 无 | 无 | R0/R1 |
| 离心 | 转子、RCF/rpm、时间、温度、配平 | 无 | 无 | 无 | R1 |
| 吸光度测量 | 波长、模式、板型、空白、温度 | 无 | 无 | 无 | R1 |
| 荧光/发光测量 | 激发/发射、滤光片、增益、板型 | 无 | 无 | 无 | R1 |
| 显微成像 | 物镜、通道、曝光、Z-stack、位置 ID | 无 | 无 | 无 | R1 |
| 洗板 | 板型、体积、循环、浸泡、残留 | 无 | 无 | 无 | R1 |
| qPCR | 板型、体积、通道、循环、熔解 | 无 | 无 | 无 | R1 |
| 流式 | 激光、检测器、panel、补偿、事件数 | 无 | 无 | 无 | R1 |
| 分光光度 | 波长、blank、体积、检测限 | 无 | 无 | 无 | R0/R1 |
| 涡旋/加热/恒温混匀 | 温度、转速/振幅、时间 | 无 | 无 | 无 | R0 |
| 样本存储/冻存/交接 | 温度、容器、降温、位置、交接 | 无 | 无 | 无 | R0 |
| 非仪器步骤 | 固定、通透、封闭、染色、裂解等 | 无 | 无 | 无 | R0/R1 |

第一批应只做:液体转移、接种/分配、孵育、洗涤、取样、吸光度测量、荧光成像;以及 CCK-8、LIVE/DEAD 两个完整模板。其余保持研究状态,不批量导入。

## 4. 仪器档案覆盖矩阵

| 档案/型号 | 文档中的能力 | 代码中的 profile | 证据定位 | 状态 | 缺口 |
|---|---|---|---|---|---|
| `plate_reader.absorbance.multiskan_skyhigh` | 200–1000 nm;6/48/96/384;终点/动力学/光谱 | 无 | 厂商规格摘录 | R1 | 未逐页核对;未记录滤光片/板盖/校准;无兼容性规则 |
| `centrifuge.eppendorf_5424r` | 100–15,000 rpm;最高约 21,130×g;−10–40°C;24×1.5/2.0 mL | 无 | 厂商规格摘录 | R1 | 转子/适配器/RCF 关系未建模;未核对具体转子 |
| `cell_counter.countess_3` | 10 µL;1×10⁴–1×10⁷ cells/mL;4–60 µm | 无 | 厂商规格摘录 | R1 | 染色/稀释/聚集判定未结构化 |
| `washer.biotek_405ts` | 96/384;1–250 循环;25–3000 µL/孔;残留 ≤2 µL | 无 | 厂商规格摘录 | R1 | manifold/板型兼容未建模 |
| `microscope.leica_dmi8` | 倒置;明场/DIC/相差/荧光;物镜/滤光片;stage incubator | 无 | 厂商规格摘录 | R1 | 物镜工作距离/通道/容器兼容未建模 |
| `qpcr.quantstudio_5` | 96 孔 0.1/0.2 mL;6 滤光片/21 组合 | 无 | 厂商规格摘录 | R1 | block/反应体积/通道兼容未建模 |
| `flow.bd_facscanto_ii` | 488/633/405 nm;最多 8 色;10–120 µL/min;10k events/s | 无 | 厂商规格摘录 | R1 | 激光/滤光片/panel 兼容未建模 |
| `pipette.manual` / `liquid_handler.generic` | 体积/通道/吸头/labware | 无 | 通用能力 | R0 | 无型号、无量程、无吸头匹配 |
| `incubator.co2` | 温度/CO2/湿度/时间 | 无 | 通用能力 | R0 | 无容器/透气盖/校准 |
| `thermomixer.generic` | 温度/转速/时间 | 无 | 通用能力 | R0 | 无加热块/容器适配 |

## 5. 文档与实现冲突

| 编号 | 冲突/风险 | 处理决定 |
|---|---|---|
| D1 | 附件把 Operation Module、Assay Protocol Template、Instrument Profile、ToolAction 称为“四种能力”;`CONTEXT.md` 规定步骤块是唯一可复用操作对象 | OperationModule 是 `ProtocolBlockVersion` 的**契约视图**,不是新持久化对象;Assay Protocol Template 映射 `ProtocolVersion`;InstrumentProfile / ToolAction 独立但不成为步骤运行时 |
| D2 | `instrument-step-modules.md` 写“仪器/操作模块 = 一个步骤类型”;`architecture.md` 写 InstrumentProfile 是 `meta.instrument` 语义 | 保留前者作为用户插入形态;后者作为存储归属。仪器能力不等于步骤,步骤通过 `meta.instrument` 引用 profile |
| D3 | `2026-11-05-step-editor-implementation-plan.md` 把 `StepMeta` 放在 `ProtocolStep::Text`;`architecture.md` 说 P4 前不实现 | K1 只定义模块契约;StepMeta 仍为 P4 候选,不提前写入 CLI |
| D4 | 现有 `ParameterSlot` 不能表达来源/约束/必填阶段;文档却要求 `protocol_fixed`、`design_variable` 等 | K1 定义 `OperationParameter`,映射到未来 `ParameterSlot` 扩展;当前不改 `src/cli.rs` |
| D5 | 文档称“仪器能力检查”可确定;代码没有 InstrumentProfile/兼容性引擎 | K0 标为未实现;K2 才建立 profile 与兼容性规则 |
| D6 | `RunWorkspace` 已实现 steps/tasks/occurrences,但 `ExecutionEvent` 没有仪器设置/原始文件字段 | K1 不改 Run;记录字段作为契约候选,Run 实际值仍在 K4/K5 评估 |
| D7 | 现有研究把“常用值”和“厂商推荐值”并列;没有来源状态 | K1 强制 `value_source`;无来源值只能是 `unknown` 或 `example_only`,不能自动填充 |
| D8 | 文档覆盖十余类实验,但没有变体矩阵;容易把 CCK-8 一个流程当唯一 SOP | K0 要求每个 assay 至少保留一个来源协议 + 一个实验室 SOP 变体;首批只做 CCK-8/LIVE-DEAD |

## 6. 证据缺口与审核计划

### 6.1 现有证据的缺口

1. 多数来源只有链接,没有页码/章节/检索日期/版本。
2. 厂商推荐参数没有与“实验室 SOP”分开;不能当默认值。
3. 没有来源到字段的逐项映射;无法判断某个数值来自手册、论文还是 Agent 记忆。
4. 没有变体矩阵:同一 assay 的容器、细胞类型、试剂盒、读数模式没有分支。
5. 没有专家复核记录,也没有真实实验室验证记录。
6. 部分厂商站点对自动请求返回 403;现有链接未全部人工打开复核。
7. 没有 unit/dimension 校验;单位只是字符串。
8. 没有“示例值/推荐值/协议固定值”的运行时区分。

### 6.2 K0 审核计划(下一轮执行)

每个首批模块都必须完成:

1. 选一份一手来源(厂商手册/标准/同行评议协议),记录版本、页码/章节、URL、检索日期。
2. 选一份实验室 SOP;没有 SOP 时明确写“无实验室 SOP”,不得用厂商流程冒充。
3. 逐字段对照:参数名、值、单位、适用范围、限制、是否必需。
4. 建立变体矩阵:容器、样本类型、检测模式、试剂体系、时间锚点。
5. 找 1–2 名实际做该实验的人做专家复核;记录未回答项。
6. 只有完成 1–5 才能从 `Research Draft` 升到 `Source Checked`/`Expert Reviewed`。
7. `Lab Validated` 只能在真实实验室流程验证后使用。

## 7. K0 交付清单

- [x] 代码事实与行号。
- [x] 实验覆盖矩阵(12 类 + 动物/临床边界)。
- [x] 操作模块矩阵(16 类)。
- [x] 仪器档案矩阵(11 类/型号)。
- [x] 文档冲突清单(D1–D8)。
- [x] 证据缺口与审核计划。
- [x] 明确当前没有任何 `Source Checked` / `Expert Reviewed` / `Lab Validated` 条目。
- [ ] 逐页核对手册/标准(下一轮 K0+ 执行,不在本轮伪造)。
- [ ] 专家复核与实验室验证(后续阶段)。

## 8. 来源

沿用现有研究已列来源,不新增未核对断言:

- Thermo Fisher 细胞培养/解离协议:https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/subculturing-adherent-cells.html
- Dojindo CCK-8 CK04e:https://dojindo.com/products/manual/CK04e.pdf
- Thermo Fisher LIVE/DEAD L3224:https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/live-dead-viability-cytotoxicity-kit-for-mammalian-cells.html
- ibidi 伤口愈合:https://ibidi.com/img/cms/downloads/an/AN21_Wound_Healing_Assay.pdf
- R&D Systems 夹心 ELISA:https://www.rndsystems.com/resources/protocols/elisa-direct-sandwich-protocol
- Addgene Western blot:https://www.addgene.org/protocols/western-blot/
- MIQE:https://academic.oup.com/clinchem/article/55/4/611/5631762
- Bio-Rad qPCR:https://www.bio-rad.com/en-us/applications-technologies/real-time-pcr-qpcr-learning-center
- BD FACSCanto II:https://www.bdbiosciences.com/content/dam/bdb/marketing-documents/facscanto_techspecs.pdf
- Cell Signaling IF:https://www.cellsignal.com/learn-and-support/protocols/protocol-if-cell-based-imaging
- 静态生物膜/CV/CFU:https://pmc.ncbi.nlm.nih.gov/articles/PMC4568995/
- Multiskan SkyHigh 规格:https://etconanalytical.com/wp-content/uploads/2020/10/Multiskan-SkyHigh-Microplate-Spectrophotometer-Specifications-.pdf
- Opentrons Protocol Designer:https://docs.opentrons.com/protocol-designer/
- ISA Abstract Model:https://isa-specs.readthedocs.io/en/latest/isamodel.html
- SiLA 2:https://sila-standard.com/standards/
- OBI:https://obi-ontology.org/docs/core-classes/

这些链接的版本、页码和适用条件仍需人工复核;本轮不把它们当作已核验事实。
