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

## 1.1 粒度判定:最小可审计科学操作

**不是拆到最小物理动作,而是拆到“最小有独立科学意义、可复用、可验证的操作单元”。**

一个候选要成为 `atomic_operation`,回答四个问题:

| 维度 | 判断问题 |
|---|---|
| 独立目的 | 这个动作是否有明确、独立的实验意图? |
| 稳定边界 | 输入、输出及可能发生的样本变化是否能说明白? |
| 独立确认 | 是否可以判断操作完成、失败或需要复核? |
| 有意义的复用 | 换一个方法后,这个操作的基本含义是否保持不变? |

停止拆分规则:

> 如果继续拆分只会产生设备动作、操作姿势、或没有独立科学意义的子动作,就停止。

例子:

| 操作 | 是否继续拆 | 理由 |
|---|---|---|
| 移液 | 不拆吸液/排液 | 一次物质转移是整体 |
| 划线接种 | 不拆每一笔 | 一次空间分布操作 |
| 涂布接种 | 不拆每次涂布器移动 | 一次表面接种 |
| 离心 | 不拆启动/加速/减速 | 一次分离操作 |
| 离心后取上清 | 与离心分开 | 新的操作意图和物质转移 |
| 菌种传代 | 拆 | 转接、培养、谱系更新等多个阶段 |
| 生物膜 CV | 拆 | 洗涤、染色、溶解、测量等多个操作 |

原子性不是“动作数量最少”,而是科学事务边界足够清楚。

### 1.2 四种粒度 + Custom Step

| 层级 | 负责什么 | 示例 | 机器表达 |
|---|---|---|---|
| Atomic Operation | 一个可独立确认的科学操作 | 移液、划线、孵育、离心、测量 | `OperationModule` |
| Composite Block | 多个操作构成可复用流程 | 菌种传代、连续稀释、细胞传代 | 递归 `BlockUse` |
| Method Template | 具有明确科学终点和方法约束 | 生物膜 CV、CFU、CCK-8、ELISA | `ProtocolVersion` + 组合块 |
| Assay Model / Design | 定义测量什么、比较什么及样本关系 | 生物膜形成、细胞活力、蛋白表达 | Design/Assay |

前三层继续使用现有 `ProtocolBlockVersion` 和递归 `BlockUse`,不需要第二套执行引擎。

**Custom Step 是必须保留的出口。** 当没有合适模块时,允许创建一个普通自定义步骤:

- 有标题、说明、时间、参数、目标、来源;
- 样本变化可标 `Unknown`;
- 不强行套用不合适的 OperationModule;
- 反复出现且边界稳定后,再提炼为正式模块。

### 1.3 不能假设所有实验都可无损拆解

以下情况必须允许条件分支、人工判断和未知:

- 处理后根据观察决定继续/重复/换方法;
- 判断细胞状态、取样区域、样本是否适合后续操作;
- 洗涤/分散造成未知程度的样本损失;
- 新方法尚无稳定模块。

正确表达是:`已定义操作 + 条件分支 + 人工判断 + Custom Step`,而不是要求所有实验由已有模块 100% 覆盖。

### 1.4 原子操作不是“无变量”

Atomic Operation 固定的是**科学语义与边界**,不是所有数值:

- 固定:目的、端口、主要样本变化、完成检查、失败模式;
- 可变:试剂/缓冲液、体积、次数、浸泡、搅拌/吹打方式、去除方式、温度等;
- 变量应进入参数槽/本次绑定,而不是复制出许多模块。

用洗涤为例:

| 内容 | 归属 |
|---|---|
| 目的:交换液体、去除未结合物,样本身份保留 | 原子操作定义 |
| 输入:样本、容器、洗液、废液路径 | 原子操作端口 |
| 主要变化:样本 `state_changed`;洗液 `consumed` | 原子操作 transition |
| 洗液种类、体积、循环数、浸泡时间、搅拌/吹打方式、吸液方式 | 参数槽 |
| 某方法要求的 3 次 PBS、5 分钟浸泡、轻柔吹打 | Method Template 绑定 |
| 实际用了多少体积、几次、是否吹打 | Run 记录 |
| 洗涤造成样本损失/生物膜脱落 | lossy wash 变体或 loss 缺口,不能塞进普通参数假装无损 |

判定规则:

- 同一目的、端口、effect、完成检查,只是值不同 → **参数**,不建新模块。
- 样本 effect/损失/身份发生变化 → **variant**。
- 主要意图改变(洗涤 → 吹打分散/解离)→ **另一个操作**。
- 方法特定的默认值、对照、读数解释 → **Method Template**,不写进通用操作默认。

因此“用什么试剂洗、洗多少次、要不要吹打”本身不会推翻洗涤的原子性;它们应被参数化并由方法/运行绑定。只有“吹打”从混合洗液变成机械分散/解离时,才不再是洗涤。

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
