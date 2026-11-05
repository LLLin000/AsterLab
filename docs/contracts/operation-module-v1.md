# OperationModule Contract v1(K1 契约草案)

- 日期:2026-11-05
- 阶段:K1;契约与最小 Rust 测试,**未接入 CLI/Preview/Run/Slint**。
- 代码:[`crates/asterlab-core/src/operation_module.rs`](../../crates/asterlab-core/src/operation_module.rs)
- K0:[知识覆盖审计](../research/2026-11-05-k0-knowledge-coverage-audit.md)
- 状态:待审查。审查通过前不进入 K2–K5。

## 0. 一句话

`OperationModule` 是 `ProtocolBlockVersion` 的**契约视图**,不是新的数据库表、不是第二套步骤运行时;它把“一步操作需要什么、产生什么、哪些值有来源、什么仪器能力足够、审核到哪一级”写成可校验结构。

## 1. 边界

| 是 | 不是 |
|---|---|
| 可复用操作的版本化定义 | 新的持久化对象或表 |
| 输入/输出、参数、时间作用域、样本影响、仪器能力要求 | 自由 JSON 工作流 |
| 编译到现有 `ProtocolBlockVersion` / `BlockUse` / `ParameterBinding` | 替代现有步骤块或 ProtocolVersion |
| 结构化知识 + 审核状态 | 自动生成科学结论 |
| K1 的结构校验 | K2 的仪器兼容性引擎、K3 的 Catalog Compiler |

**本轮代码不改** `src/cli.rs`、`ProtocolPreview`、`RunWorkspace`、任何 `.slint` 或正式数据库。

## 2. 顶层结构

```rust
pub struct OperationModule {
    pub schema_version: u32,
    pub identity: ModuleIdentity,
    pub semantics: ModuleSemantics,
    pub parameters: Vec<OperationParameter>,
    pub execution: ExecutionSpec,
    pub knowledge: ModuleKnowledge,
}
```

`OPERATION_MODULE_SCHEMA_VERSION = 1`。当前为草案;旧记录没有该结构时保持 legacy,不推断。

### 2.1 Identity

```rust
pub struct ModuleIdentity {
    pub module_id: String,
    pub version: u32,
    pub title: String,
    pub action_type: ActionType,
    pub applicability: Applicability,
    pub variant_of: Option<ModuleRef>,
}
```

| 字段 | 规则 |
|---|---|
| `module_id` | 稳定 ID;小写字母开头,只含 `a-z 0-9 . _ -`,≤120 |
| `version` | ≥1;版本不可原地修改 |
| `action_type` | 原子动作枚举;不是仪器型号 |
| `applicability` | assay family / sample kind / container kind / variant / exclusion |
| `variant_of` | 可选;引用**确切**的父模块版本 |

`ModuleRef` 只允许精确匹配:

```rust
pub struct ModuleRef { pub module_id: String, pub version: u32 }
```

不支持 `latest`、范围、模糊标题匹配。不同版本不能被静默替换。

### 2.2 Semantics

```rust
pub struct ModuleSemantics {
    pub purpose: String,
    pub preconditions: Vec<String>,
    pub inputs: Vec<PortSpec>,
    pub outputs: Vec<PortSpec>,
    pub sample_effects: Vec<SampleEffect>,
}
```

`PortSpec` 使用明确角色,不用自由字符串:

```rust
pub struct PortSpec {
    pub port_id: String,
    pub role: PortRole,          // source_sample / reagent / destination_container / ...
    pub object_type: PortObjectType, // sample / reagent / container / measurement / data / ...
    pub required: bool,
    pub cardinality: Cardinality, // one / many
    pub description: String,
}
```

`SampleEffect`:

```text
preserved | aliquoted | consumed | terminal | state_changed | unknown
```

规则:

- `unknown` 不能和已知效果同时出现;否则模块在自相矛盾。
- `terminal` 不是“破坏性”布尔值;终末、可分样、可回收是不同语义。
- 输出“原始测量”与“派生结果”分开;不能把计划输出写成已产生。

### 2.3 Parameters

```rust
pub struct OperationParameter {
    pub key: String,
    pub label: String,
    pub value_type: ParameterValueType,
    pub unit: Option<String>,
    pub required: bool,
    pub required_at: RequiredAt,
    pub value_source: ValueSource,
    pub constraints: Vec<ParameterConstraint>,
    pub description: String,
    pub evidence: Vec<EvidenceRef>,
}
```

`ParameterValueType` 当前只映射现有四类:`text|number|integer|boolean`。枚举/文件/实体引用不在 v1 假装支持;如果后续需要,先改统一参数契约。

`RequiredAt`:

```text
draft | protocol_freeze | run_freeze | execution
```

`required_at` 表示“最迟在哪个阶段必须确认”,不是“现在必须填”。草案可保留未知。

`ValueSource`:

| 值 | 含义 | 自动绑定 |
|---|---|---|
| `protocol_fixed` | 所选且已确认的 Protocol 版本规定 | 可在适用条件满足时作为计划值 |
| `design_variable` | 本实验由研究者设定 | 不自动填 |
| `lab_sop` | 已选实验室 SOP 版本 | 可建议,检查适用性 |
| `vendor_recommended` | 厂商推荐 | 只作建议 |
| `example_only` | 文档示例 | 不自动写入 |
| `run_actual` | 实际执行记录 | 只能来自 Run |
| `unknown` | 未确定 | 保持未知 |

`ParameterConstraint` v1:

```text
min / max / one_of / max_length / non_empty
```

规则:

- 单位只允许用于 number/integer;当前是字符串精确匹配,没有量纲换算。
- `min/max` 只能用于数值;必须有限。
- `one_of` 必须非空。
- 约束缺失不等于“无限制”;缺失表示“尚未结构化”,不能据此自动放行。

### 2.4 Execution

```rust
pub struct ExecutionSpec {
    pub time_scope: StepTimeScope,
    pub instrument_requirements: Vec<InstrumentRequirement>,
    pub completion_checks: Vec<CompletionCheck>,
    pub record_fields: Vec<RecordField>,
}
```

- `time_scope` 复用现有 `StepTimeScope`:`once|per_timepoint|per_assay|unresolved`。
- 只有 `per_timepoint` 才允许按 Assay 时间点展开任务;缺失/`unresolved` 不等于每个时间点。
- `completion_checks` 分 `deterministic|operational|advisory`;只有 deterministic 才能阻断当前操作,advisory 不能伪装成错误。
- `record_fields` 是 Run 中应记录的字段契约,不是实际值。

仪器要求:

```rust
pub struct InstrumentRequirement {
    pub requirement_id: String,
    pub class: String,
    pub profile_id: Option<String>,
    pub profile_version: Option<u32>,
    pub required: bool,
    pub capabilities: Vec<CapabilityRequirement>,
}
```

- 优先声明能力,不是品牌型号。
- 有 `profile_id` 必须同时有 `profile_version`;K1 校验/编译时拒绝“只有型号没有版本”的精确引用。
- 型号未知时 `profile_id/profile_version` 为空,只保留 `class`。
- `CapabilityConstraint` v1:`exists|min|max|range|one_of`;单位随约束声明,不做换算。
- K1 不实现“兼容/不兼容/未知”判定;那是 K2。

### 2.5 Knowledge

```rust
pub struct ModuleKnowledge {
    pub controls: Vec<ControlSpec>,
    pub decision_points: Vec<DecisionPoint>,
    pub failure_modes: Vec<FailureMode>,
    pub readout: Option<ReadoutMeaning>,
    pub evidence: Vec<EvidenceRef>,
    pub review_status: ReviewStatus,
}
```

- `controls` 绑定方法要求(空白/阴性/阳性/标准/载体/未处理/未染色),不是实际组别 ID。
- `readout` 明确 raw / derived / not_implied;不能把 OD、Cq、荧光强度直接叫活细胞数、表达量或浓度。
- `decision_points` 只表达继续/停止/重做的依据,不是自动流程引擎。
- `evidence` 至少含 citation;url/locator/version/retrieved_at 可空但缺失会降低可审核性。
- `review_status`:`research_draft < source_checked < expert_reviewed < lab_validated`;只有 `research_draft` 允许没有 evidence。
- 状态是单调的;软件不会因为字段齐全就把模板升级。

## 3. 与现有模型的映射

### 3.1 `OperationModule` → `ProtocolBlockVersion`

一个原子 `OperationModule` 映射到一个只含一个 `ProtocolStep::Text` 的 `ProtocolBlockVersion`;组合块继续用现有递归 `BlockUse`,不再为组合块造第二种模块类型。

| OperationModule | 现有对象 | 处理 |
|---|---|---|
| `identity.module_id` | `ProtocolBlock.id` | 稳定 ID;不是新表主键 |
| `identity.version` | `ProtocolBlockVersion.version` | 版本不可变 |
| `identity.title` | `ProtocolBlock.title` | 名称 |
| `parameters[]` | `ParameterSlot[]` | 扩展可选 `description/source/required_at/constraints/evidence`;K1 不改现有结构 |
| `execution.time_scope` | `ProtocolStep::Text.time_scope` | 已有字段;直接映射 |
| `semantics.inputs/outputs` | 尚无字段 | K3 编译为 block-level optional `module_contract`;不塞进任意文本 |
| `semantics.sample_effects` | 尚无字段 | 同上 |
| `execution.instrument_requirements` | `StepMeta.instrument`(P4 候选) | 只存能力引用,不存型号设置 |
| `knowledge` | 尚未落地 | K3 作为 block-level contract sidecar;StepMeta 只投影步骤级子集 |
| `review_status/evidence` | 尚无字段 | K3 随 contract 保存;不能从名称推断 |

**存储选择:** 模块级字段放在 `ProtocolBlockVersionData` 的可选 `module_contract` sidecar,而不是每个步骤的 `meta`。理由:

- `purpose`、`applicability`、`inputs/outputs`、`sample_effects` 属于模块整体,不属于某一个文字步骤。
- 一个组合块可以有多个步骤;把模块级知识塞进任意一个步骤会漂移。
- `StepMeta`(P4)仍可存在,只放步骤级目的/判定/失败模式等子集;两者不是平行引擎。
- 该 sidecar 是 JSON 字段,不新增表;K1 只定义形状,不改 `deny_unknown_fields` 的现有 CLI。

### 3.2 `BlockUse` / `ParameterBinding`

- `BlockUse.block_version_id` 指向确切 `ProtocolBlockVersion`;因此也固定了 `module_id + version`。
- 同一模块插入两次 = 两个 `BlockUse`;参数绑定互相隔离。
- `ParameterBinding` 只改本次值,不复制步骤块,不改模块定义。
- 父块通过 `source_slot` 映射参数时,仍要求类型/单位完全一致;K1 不引入自动换算。

### 3.3 Protocol / Run

- Assay Protocol Template 映射现有 `ProtocolVersion`(可引用根块 + root bindings),不是新对象。
- Run 创建时冻结**确切**的 block version + module contract + 参数绑定;旧 Run 不重新解析“最新模块”。
- 模块 contract 只定义要求;Run 的仪器设置、序列号、校准、原始文件仍属于执行记录,不能由 contract 补造。

### 3.4 InstrumentProfile / ToolAction

- `InstrumentProfile` 是独立能力档案(K2),不被本契约包含;本契约只引用 `class + profile_id + profile_version`。
- `ToolAction` 是确定性计算/预览/外部动作,不是步骤;本契约不把计算器做成 OperationModule。
- K1 不实现 profile 注册、兼容性引擎或 ToolAction 执行。

## 4. 兼容规则

1. **旧块可读**:没有 `module_contract` 的 `ProtocolBlockVersion` 继续按 legacy 处理;不推断语义,不自动升级。
2. **新字段可选**:未来扩展字段必须可缺省;旧记录不被迁移。
3. **不允许静默替换版本**:引用必须是确切版本;`latest`/范围/标题匹配都不合法。
4. **旧 Run 不变**:冻结的 contract 与 block version 不重新解析;新代码读取旧 Run 时按 legacy 展示。
5. **schema_version**:`OperationModule` 用 `schema_version=1`;未来变更先升版本,不原地扩字段。
6. **拒绝未知字段**:契约类型使用 `serde(deny_unknown_fields)`,错字不能静默丢失;但读取 legacy 块不受此约束。
7. **不承诺降级**:单机升级整体替换;不保证旧二进制读新 contract。

## 5. K1 结构校验

`validate_contract()` 只做结构检查;不判断科学正确性、不判断仪器是否真的兼容。

| 代码 | 规则 |
|---|---|
| `schema_version` | 必须为 1 |
| `stable_id` | module/port/requirement/control ID 格式 |
| `version` | ≥1 |
| `title` / `purpose` | 非空 |
| `sample_effect` | 至少一个 |
| `sample_effect_ambiguous` | `unknown` 不能与已知效果混用 |
| `duplicate_parameter` | 参数 key 唯一 |
| `profile_version` | `profile_id` 与 `profile_version` 必须同时出现 |
| `unit_on_non_numeric` | 只有 number/integer 可声明单位 |
| `numeric_constraint` | min/max 只能用于数值 |
| `max_length_constraint` | 只用于 text/integer,且 >0 |
| `one_of_empty` | one_of 不能为空 |
| `duplicate_port` / `duplicate_control` / `duplicate_completion_check` | 同类 ID 唯一 |
| `evidence_required` | `source_checked` 及以上必须有 evidence |
| `evidence_citation` | evidence citation 非空 |

明确不做:K1 不校验“450 nm 是否适用于该细胞”“该型号是否支持该板型”“该对照是否科学充分”。这些进入 K2/K3/专家审核。

## 6. 最小 Rust 测试

代码位置:`crates/asterlab-core/src/operation_module.rs`。当前 10 个测试:

| 测试 | 证明 |
|---|---|
| `minimal_contract_is_structurally_valid` | 完整最小模块可通过结构校验 |
| `module_references_are_exact_version_only` | 同 ID 不同版本不相等;拒绝隐式升级 |
| `unknown_sample_effect_cannot_be_mixed_with_known_effect` | 未知不能被伪装成已知样本影响 |
| `source_checked_requires_evidence` | 审核状态不能无证据升级 |
| `unit_is_rejected_on_text_parameter` | 单位不能挂在文本参数上 |
| `duplicate_parameter_keys_are_rejected` | 参数 key 唯一 |
| `profile_id_requires_exact_profile_version` | 型号档案不能只给 ID 不给版本 |
| `suggestions_are_not_planned_bindings` | 厂商推荐/示例值不能当作已确认计划值 |
| `serde_round_trip_preserves_source_and_time_scope` | 来源与时间作用域不被序列化丢失 |
| `review_status_is_monotonic` | 审核等级单调,不能回退冒充 |

运行:

```bash
cargo test -p asterlab-core operation_module
```

## 7. 未决项(审查时决定)

1. `module_contract` sidecar 放在 `ProtocolBlockVersionData` 的哪个字段;K3 再改 `src/cli.rs`。
2. 一个 block 是否允许包含多个 `OperationModule` 语义;v1 只支持“原子模块 = 单步骤块”,组合用 `BlockUse`。
3. `ParameterValueType` 是否增加 `enum/file/entity_ref`;v1 不增加。
4. 量纲/单位换算是否进入 K2;v1 只做字符串精确匹配。
5. `InstrumentRequirement.profile_version` 是否必填;当前“有 profile_id 必须有 profile_version”的规则由编译器/K2 检查,不在结构校验中补默认值。
6. `ReviewStatus` 的审批人和时间是否记录;v1 只有枚举,没有审批日志。
7. `StepMeta` 与 block-level contract 的最终边界;K1 只给出原则,不写 P4 Schema。

## 8. K1 完成条件

- [x] 类型与字段边界。
- [x] 参数来源/确认阶段/约束。
- [x] 输入输出、样本影响、时间作用域。
- [x] 仪器能力要求(不实现兼容引擎)。
- [x] 审核状态与证据。
- [x] 与 `ProtocolBlockVersion`/`BlockUse`/`ParameterSlot`/`ProtocolVersion`/Run 的映射。
- [x] 兼容规则。
- [x] 10 个最小 Rust 测试。
- [x] 未接入 CLI/Preview/Run/Slint。
- [ ] 架构/科学审查通过。
- [ ] 决定 sidecar 字段与 StepMeta 边界。
- [ ] 才进入 K2。

## 9. 来源

- 现有 Core Contract:[`docs/contracts/core-v1.md`](core-v1.md)
- 现有架构:[`docs/architecture.md`](../architecture.md)
- K0 审计:[`docs/research/2026-11-05-k0-knowledge-coverage-audit.md`](../research/2026-11-05-k0-knowledge-coverage-audit.md)
- ISA Abstract Model:https://isa-specs.readthedocs.io/en/latest/isamodel.html
- SiLA 2:https://sila-standard.com/standards/
- OBI:https://obi-ontology.org/docs/core-classes/
- 现有协议知识研究:[`docs/research/2026-11-05-protocol-knowledge-and-related-projects.md`](../research/2026-11-05-protocol-knowledge-and-related-projects.md)

这些标准只借 Process/Input/Output/Parameter/Capability 的模式;不实现完整本体。
