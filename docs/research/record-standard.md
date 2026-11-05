# AsterLab 操作与仪器记录规范 v1

- 日期:2026-11-05
- 状态:研究记录规范;不是运行时 Schema,不替代 K1 `OperationModule` 或 K2 `InstrumentProfile`
- 适用:后续所有操作、方法、仪器来源记录
- 原则:先记录事实、来源、适用条件和未知,再决定能否进入 JSON/运行时
- 本轮范围:**体外/细胞/微生物/分子湿实验**。临床研究、动物在体实验、统计/生物信息/图像分割与分析暂不纳入;只记录湿实验步骤和原始读数,不定义数据处理算法或科学结论。

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

## 1.5 具体实验的拆分流程(必须先查复用)

收到一个具体实验时,按以下顺序做,不要先写新模块:

1. **识别层级**:它回答什么科学问题?是 Assay Model、Method Template,还是单个操作?
2. **列出现有操作**:在 `catalog/operations/` 和操作记录里查找可复用的 Atomic Operation(液体转移、取样、孵育、洗涤、离心、读取、成像、接种、计数等)。
3. **画复用映射**:实验步骤 → 已有操作/复合块/方法模板;标出“直接复用/参数化复用/需要变体/缺失”。
4. **判断缺失类型**:
   - 同一目的、端口、effect,只是值不同 → 用参数/绑定,不建新模块。
   - 样本 effect/意图改变 → 建 variant 或新 Atomic Operation;先过四维粒度判定。
   - 只是多个操作的固定顺序 → Composite Block/Method Template。
   - 只是确定性计算/预览/换算 → ToolAction,不是操作。
   - 没有稳定边界/新方法 → Custom Step,允许 Unknown。
5. **只对真正缺失的原子操作建候选**,并记录它为什么不能由已有操作+参数表达。
6. **来源/证据**:每个参数和关键行为有来源;没有就 UNKNOWN。
7. **不进入 JSON/运行时**,除非候选通过粒度审查、结构校验和第二人复核。

每个具体实验记录必须有 **Reuse Map**:

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 加处理液 | `op.liquid_transfer` | 直接复用;绑定体积/模式 |
| 洗涤 | `op.washing` | 复用;指定 lossy/Preserving 变体 |
| 孵育 | `op.incubation` | 复用;绑定温度/时间/锚点 |
| 新步骤 X | 无 | 建候选/ Custom Step;说明原因 |

**工具(ToolAction)不是操作**:稀释计算、板容量检查、时间估算、图像计数等确定性动作不建 OperationModule;只在步骤上下文调用。

## 1.6 原子性不等于应进入公共 Catalog

**最小可审计操作**是步骤建模粒度;**是否进入全局 Catalog**是另一项复用决策。原子性本身不授权新建公共模块。

### Method-local Step

方法内部步骤不是新的持久化实体;默认继续使用 `ProtocolStep::Text`:

- 有说明、输入、完成检查、来源;
- 不属于全局 Catalog;
- 方法模板可以引用它,但其他方法不能默认复用;
- 边界稳定且出现跨方法复用后,再走晋升审查。

适合 method-local:
- 划痕/Insert 的“形成间隙”;
- Transwell 的“清除上膜未迁移细胞”;
- 3D 基质 dome 点胶;
- 特定方法的专用染色/固定变体。

### 公共模块晋升门槛

候选公共 OperationModule 必须回答:

| 维度 | 标准 |
|---|---|
| 复用广度 | 至少 2 个独立方法家族有真实使用场景(同一方法的变体不算两个) |
| 语义稳定 | 换方法后核心输入/输出/作用不变 |
| 抽象收益 | 复用后确实减少重复定义/校验/记录 |
| 差异成本 | 不需要大量专用条件分支才能统一 |

决策路径:

```text
发现新步骤
  → 已有操作 + 参数/变体能否表达?
      能 → 复用,不新增
      否 → 是否在多个独立方法中有相同科学语义?
             否 → Method-local Step
             是 → 候选公共 OperationModule
                    → 契约/证据/测试/复核
```

例外:涉及关键样本谱系、安全性或必须独立审计的操作,即使短期只有一个方法,也可以作为受控例外晋升;复用次数是优先级规则,不是绝对定律。

**第一版 Catalog 应小而有真实复用证据,不是覆盖所有已知操作。**

## 1.7 Protocol Discovery 轻量记录

持续研究 Protocol 时,不需要每篇都写完整 OperationRecord。每篇先交两张表:

1. **步骤拆解表**:原文步骤 → 操作语义/现有模块/候选 → 输入/输出 → 参数归属 → 来源定位。
2. **公共操作复用表**:候选 → 出现过的独立方法家族 → 处理决定 → 证据/缺口。

只有当一个候选要通过公共 Catalog 晋升时,才展开完整 PortSpec/SampleTransition/参数证据。详见 [protocol-discovery](protocol-discovery/README.md)。

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

## 3.1 按断言类型选择来源

不存在“平台越权威就越优先”的统一排序。来源优先级由**断言类型和适用范围**决定:

| 断言 | 首选来源 |
|---|---|
| 公开方法/步骤是什么 | 该方法的原始 protocol/方法论文/Protocol 仓库条目 |
| 某试剂盒的操作条件 | 对应版本/批次的厂商说明书 |
| 某型号/配置的仪器能力 | 该型号+配置的手册/规格表 |
| 本实验室实际采用的操作 | 经审核的本地 SOP/设备记录 |
| 统计/报告要求 | 对应指南/标准 |

规则:

- 同一实验可以同时引用“公开 protocol”和“本地 SOP”;它们是不同适用范围的证据,不互相覆盖。
- 试剂体系/版本不同,不能拿另一版本的厂商条件替换。
- 本地 SOP 是执行依据,但不能冒充“公开标准方法”。
- 来源冲突时按适用范围保留双方,说明软件应选哪一个,不取平均值。
- 不能为了遵循公开 protocol 而覆盖实验室已选定的试剂体系或已审核 SOP。

本地仓库的研究记录只用于 Reuse Map 和差异对比,不是标准步骤或数值证据。

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
