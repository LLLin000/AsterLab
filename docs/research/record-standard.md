# AsterLab 操作与仪器记录规范 v1

- 日期:2026-11-05
- 状态:研究记录规范;不是运行时 Schema,不替代 K1 `OperationModule` 或 K2 `InstrumentProfile`
- 适用:后续所有操作、方法、仪器来源记录
- 原则:先记录事实、来源、适用条件和未知,再决定能否进入 JSON/运行时

## 1. 两类记录,不要混用

| 记录 | 回答 | 机器契约 |
|---|---|---|
| OperationRecord | 做什么、为什么、输入/输出、样本变化、参数、时间、检查、失败模式 | `OperationModule` |
| InstrumentRecord | 哪类型号/配置、在什么条件下能做什么、量程/模式/限制、原始输出 | `InstrumentProfile` |

- 方法记录(ELISA、qPCR、WB)不是另一种运行时对象;它记录方法特有的步骤、参数绑定、对照和限制,并引用通用操作模块。
- 一个研究文档可以覆盖多个对象,但每个可复用操作/仪器必须有独立 ID 和边界。
- 不要为方法另写一套“通用操作”正文;通用操作的原理只研究一次。

## 1.1 粒度判定:先决定它是不是一个操作

一个对象要成为 `OperationModule`,必须同时满足:

1. **一个主要动作**:加液、取样、孵育、洗涤、离心、读取、成像等;不是一组动作。
2. **一个主要对象变化**:能用一条 transition 或条件 transition 表达;不是多个独立测量/判断。
3. **一个完成状态**:有明确的“这一步完成”条件;不包含多个决策点。
4. **跨方法可复用**:换一个 Assay/Protocol 仍保持同一含义;不依赖某试剂盒或某菌种。
5. **边界可封闭**:输入/输出端口能列出;不把方法解释藏在自由文本里。

不满足时,按以下类型记录:

| 类型 | 含义 | 例子 |
|---|---|---|
| `atomic_operation` | 单一可复用操作,可映射 `OperationModule` | 液体转移、取样、孵育、基本洗涤、离心、吸光度读取 |
| `composite_block` | 多个操作的固定顺序,可映射组合 `BlockUse` | 细胞传代、静态生物膜培养、CV 染色流程 |
| `method_template` | 一项检测方法的完整变体,含参数绑定/对照/分析 | CCK-8、ELISA、Western blot、qPCR、CV 生物量、CFU |
| `assay_model` | 科学问题/检测类别,不是步骤 | “生物膜形成”“细胞活力” |
| `instrument_profile` | 仪器能力,不是操作 | Multiskan SkyHigh、DMi8 |

**一个文件可以记录多个对象,但每个对象必须声明类型;只有 `atomic_operation` 能生成 `OperationModule` JSON。**

## 2. OperationRecord 必填结构

| 区块 | 必填内容 |
|---|---|
| identity | catalog id、version、标题、action_type、适用范围、排除项 |
| sources | URL/手册、页码/表格/章节、检索日期、读取程度 |
| purpose | 一步操作改变什么,不写“按常规” |
| ports | 输入/输出端口、对象类型、角色、必填、基数 |
| sample transitions | 输入→输出、effect、条件、备注;不得只写一个全局布尔 |
| parameters | key、类型、单位、必填阶段、来源分类、约束、逐参数证据 |
| time scope | 默认/允许的 once/per_timepoint/per_assay/unresolved |
| preconditions | 进入该步前必须成立的事实 |
| completion checks | deterministic/operational/advisory;是否阻断 |
| instrument requirements | 能力类别+能力约束;不绑定具体型号 |
| controls | 方法/步骤依赖的对照;引用 assay,不是实际组 ID |
| failure modes | 现象、原因/影响、处理、记录 |
| readout | raw/derived/not_implied;没有读数则 N/A |
| unknowns | 具体缺什么、为什么缺、需要谁确认 |
| review status | `research_draft`;没有 `ReviewRecord` 不得 `SourceChecked` |

禁止:

- 用通用操作记录写死某试剂盒的波长、体积、时间或空白。
- 把某型号规格写成通用能力。
- 把计划值写成实际值。
- 把“未找到”写成“没有/不支持”。

## 3. InstrumentRecord 必填结构

| 区块 | 必填内容 |
|---|---|
| identity | profile id、version、class、厂商/型号、配置/版本 |
| sources | 厂商手册/规格表 URL、页码/表格/章节、检索日期 |
| capabilities | key、value/range/set/text、unit、conditions、逐能力证据 |
| container support | `unspecified/partial/complete/explicitly_unsupported`;不能把部分列表当完整列表 |
| actual state | 校准、维护、实际安装组件属于 Run/设备状态,不写入静态 profile |
| raw outputs | 原始文件/读数/元数据;不写科学结论 |
| unknowns | 型号/配置/软件/校准/附件缺什么 |
| review | `ReviewRecord`(审核者、时间、对象版本、范围、证据、outcome) |

平台能力 ≠ 单台配置。可选项必须写成条件或 variant,不能写成每台都有。

## 4. 证据等级与措辞

| 标记 | 含义 | 能否用于 Compatible |
|---|---|---|
| `contract` | K1/K2 Schema 定义,不是科学断言 | 否 |
| `agent-verified` | 记录者读过来源并转写 | 否,仍待第二人复核 |
| `search-summary-only` | 只看到检索摘要/二手描述 | 否 |
| `UNKNOWN` | 未取得来源或配置不明确 | 否 |
| `N/A` | 该字段不适用于此对象 | 否 |
| `ResearchDraft` | 记录状态 | 否 |
| `SourceChecked` | 第二人逐字段复核后 | 可参与,仍需 ReviewRecord |
| `ExpertReviewed` | 具备实验经验的人审查 | 可参与 |
| `LabValidated` | 本实验室流程验证 | 可参与 |

要求:

- 每个数值/行为有来源定位;没有就 UNKNOWN,不能猜。
- 区分厂商规格、方法要求、实验室 SOP、单台设备状态、Run 实际设置。
- 来源冲突保留双方,说明取舍,不合并成一个“平均”值。
- `UNKNOWN` 要进一步分类:`source_missing`、`configuration_dependent`、`method_dependent`、`device_state`、`not_applicable`。
- 另一名 AI 只能做预审,不能替代第二人科学审核。

## 5. JSON 转换规则

- 只有通过结构校验的 OperationRecord 才能生成 `OperationModule` JSON。
- 只有通过结构校验的 InstrumentRecord 才能生成 `InstrumentProfile` JSON。
- 记录中的 `UNKNOWN` 不要伪造成空值;在 JSON 中省略该字段或保留为研究文档中的 UNKNOWN,并确保结构校验仍通过。
- 记录中的长解释不进入 JSON;JSON 只保留机器可读字段和证据引用。
- 契约不能表达的关系(数值守恒、谱系生成、条件执行)写入对照矩阵的“不能表达”,不要临时加字段。
- 所有 JSON 保持 `research_draft`,直到有 `ReviewRecord`。

## 6. 文件与命名

- 操作研究:`docs/research/k2-operations/<topic>.md`
- 仪器研究:`docs/research/k2-instruments/<topic>.md`
- 机器草稿:`catalog/operations/<module-id>.v<version>.json`、`catalog/instruments/<profile-id>.v<version>.json`
- 交叉矩阵:`docs/research/k2-operations/<topic>-matrix.md`
- 一个文件覆盖多个对象时,每个对象必须有独立章节和 ID。

## 7. 审核清单

- [ ] 每个数值有来源定位,或明确 UNKNOWN。
- [ ] 方法要求与仪器规格分开。
- [ ] 样本身份/状态变化有端口与 transition。
- [ ] 时间作用域不是从 Assay 名称推断。
- [ ] 参数没有隐含默认值。
- [ ] 部分列表未被当成完整列表。
- [ ] 计划/实际/原始/派生分开。
- [ ] 没有把 AI 预审写成人工审核。
- [ ] JSON 只通过结构校验,未进入运行时。
