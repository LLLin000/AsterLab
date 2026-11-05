# OperationModule Contract v1.1(K1.1 契约加固)

- 日期:2026-11-05
- 阶段:K1.1;契约与最小 Rust 测试,**未接入 CLI/Preview/Run/Slint**。
- 代码:[`crates/asterlab-core/src/operation_module.rs`](../../crates/asterlab-core/src/operation_module.rs)
- K0:[知识覆盖审计](../research/2026-11-05-k0-knowledge-coverage-audit.md)
- K2 接口提案:[仪器匹配 v1 提案](instrument-matching-v1.md)
- 状态:K1.1 待审查;审查通过前不进入 K2 实现。

## 0. 一句话

`OperationModule` 是知识目录中的**版本化操作定义**;项目里的 `ProtocolBlockVersion` 是它的本地实例。两者身份分开,知识只有一份权威来源,样本变化通过具体的输入/输出端口关系表达。

## 1. 边界

| 是 | 不是 |
|---|---|
| 目录级操作定义 + 项目级绑定关系 | 新的数据库表或第二套步骤运行时 |
| 输入/输出端口、样本转移、参数来源、仪器能力要求 | 自由 JSON 工作流 |
| 编译到现有 `ProtocolBlockVersion` / `BlockUse` / `ParameterBinding` | 替代现有步骤块或 ProtocolVersion |
| 结构校验 + 证据边界 | 自动科学审核、仪器兼容引擎 |

本轮未改 `src/cli.rs`、`ProtocolPreview`、`RunWorkspace`、任何 `.slint` 或正式数据库。

## 2. 两层身份

### 2.1 目录身份

```rust
pub struct CatalogModuleRef {
    pub module_id: String, // 跨项目稳定的知识 ID,如 op.liquid_transfer
    pub version: u32,      // 不可变版本
}
```

- 只支持精确匹配;不支持 `latest`、范围、标题模糊匹配。
- `CatalogModuleRef` 不是数据库 ID,不能直接当 `ProtocolBlock.id`。

### 2.2 项目内实例

```rust
pub struct ProtocolBlockVersionRef {
    pub block_id: String,   // 项目数据库生成
    pub version_id: String, // 不可变版本记录
}

pub enum ModuleOrigin {
    Imported,
    Derived,
    LocalCustom,
}

pub struct ModuleBinding {
    pub catalog: Option<CatalogModuleRef>,
    pub block_version: ProtocolBlockVersionRef,
    pub origin: ModuleOrigin,
    pub based_on: Option<CatalogModuleRef>,
}
```

| Origin | 必须 | 不能 |
|---|---|---|
| `Imported` | `catalog`, `block_version` | `based_on` |
| `Derived` | `based_on`, `block_version` | `catalog`(本地派生不是目录定义) |
| `LocalCustom` | `block_version` | `catalog` / `based_on` |

同一 `CatalogModuleRef` 可以被两个项目导入为两个不同的 `ProtocolBlockVersionRef`;它们共享知识定义,不共享数据库 ID。用户改版必须产生 `Derived`/`LocalCustom` 绑定并保留来源,不能冒充原目录版本。

## 3. 顶层结构

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

`OPERATION_MODULE_SCHEMA_VERSION = 1`;K1.1 仍使用 1,字段变更会在 K3 前明确升版。

### 3.1 Identity

```rust
pub struct ModuleIdentity {
    pub catalog: CatalogModuleRef,
    pub title: String,
    pub action_type: ActionType,
    pub applicability: Applicability,
    pub variant_of: Option<CatalogModuleRef>,
}
```

- `catalog` 是目录身份;项目本地身份在 `ModuleBinding`,不在本结构。
- `variant_of` 引用确切父版本;不能引用自己。
- `action_type` 是原子动作,不是仪器型号。

### 3.2 Semantics

```rust
pub struct ModuleSemantics {
    pub purpose: String,
    pub preconditions: Vec<String>,
    pub inputs: Vec<PortSpec>,
    pub outputs: Vec<PortSpec>,
    pub sample_transitions: Vec<SampleTransition>,
}
```

`PortSpec` 角色:

```text
source_sample | reagent | destination_sample | destination_container |
consumable | instrument | raw_measurement | derived_result | reference_data
```

`SampleTransition` 把样本变化绑定到具体端口:

```rust
pub struct SampleTransition {
    pub transition_id: String,
    pub input_port_id: String,
    pub output_port_id: Option<String>,
    pub effect: SampleEffect,
    pub note: Option<String>,
}
```

`SampleEffect`:

```text
preserved | moved | aliquoted | consumed | terminal | state_changed | unknown
```

规则:

- `transition_id` 唯一;输入必须引用 input port,输出必须引用 output port。
- `moved` / `aliquoted` / `state_changed` 必须有输出;`consumed` / `terminal` 可以没有输出。
- transition 可带 `applies_when { parameter_key, value }` 条件;当前只做结构校验,不执行模式选择。
- `unknown` 不能和已知 effect 混用。
- 液体转移、接种、取样必须至少有一个材料输出;空输出的液体转移被拒绝。
- 样本/试剂/材料输入必须被至少一个 transition 引用;样本/材料输出也必须被引用。
- 容器输入/输出不强制做 sample transition;它们不是样本本身。

这样可区分:

- 部分取样后保留的来源样本;
- 产生的新分样;
- 被消耗的试剂;
- 固定/染色等状态变化;
- 终末处理。

### 3.3 Parameters

```rust
pub struct OperationParameter {
    pub key: String,
    pub label: String,
    pub value_type: ParameterValueType, // text | number | integer | boolean
    pub unit: Option<String>,
    pub required: bool,
    pub required_at: RequiredAt,
    pub value_source: ValueSource,
    pub constraints: Vec<ParameterConstraint>,
    pub description: String,
    pub evidence: Vec<EvidenceRef>,
}
```

`required_at`:`draft | protocol_freeze | run_freeze | execution`。

`value_source` 只是**参数允许来源分类**:

```text
protocol_fixed | design_variable | lab_sop |
vendor_recommended | example_only | run_actual | unknown
```

实际绑定必须另有证明:

```rust
pub struct ParameterBinding {
    pub parameter_key: String,
    pub value: serde_json::Value,
    pub unit: Option<String>,
    pub source: ValueSource,
    pub source_reference: Option<String>,
    pub confirmed: bool,
}
```

- `protocol_fixed` / `lab_sop` 绑定必须有具体 `source_reference`;只有来源类别不能授权计划绑定。
- `is_plan_ready()` 还要求 `confirmed = true`;`LabSop` 类别本身不等于用户已选择并确认某份 SOP。
- `unknown` 绑定只能携带 `null`,不能有伪造值。
- 单位必须与参数声明完全一致;K1 不做换算。
- `vendor_recommended` / `example_only` 可作为建议保存,但不会自动成为计划值。

约束:

```text
min | max | one_of | max_length | non_empty | positive
```

- `min <= max`;多个 min/max 交叉检查。
- `one_of` 不得为空;值必须能按参数类型解析。
- `max_length` 只用于 text;整数范围用 min/max。
- `non_empty` 只用于 text;`positive` 只用于 number/integer 且绑定值必须 >0。

### 3.4 Execution

```rust
pub struct ExecutionSpec {
    pub default_time_scope: StepTimeScope,
    pub allowed_time_scopes: Vec<StepTimeScope>,
    pub instrument_requirements: Vec<InstrumentRequirement>,
    pub completion_checks: Vec<CompletionCheck>,
    pub record_fields: Vec<RecordField>,
}
```

- `default_time_scope` 只是默认;编译后的 Protocol 步骤只保存一个有效 `time_scope`,不会两个字段同时生效。
- `allowed_time_scopes` 为空时只允许默认值;非空时默认值必须在集合内,且不能重复。
- 只有 `per_timepoint` 才按 Assay 时间点展开任务。

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

- 有 `profile_id` 必须有 `profile_version`;精确引用,不静默替换。
- 型号未知时只保留 `class`,不填转速/波长/通道。
- `CapabilityConstraint` v1:`exists | min | max | range | one_of`;range 必须 `min <= max`,边界有限。
- K1 只校验结构;兼容性判定在 K2。

### 3.5 Knowledge

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

- `controls` 是方法要求的对照类型,不是实际组别 ID。
- `readout` 必须写清 raw / derived / not_implied。
- `evidence` 是模块级证据;参数证据在 `OperationParameter.evidence`。
- `review_status` 单调:`research_draft < source_checked < expert_reviewed < lab_validated`。

## 4. 唯一知识来源

| 场景 | 权威来源 |
|---|---|
| 目录原子操作块 | `module_contract`(本契约) |
| 普通自定义单步骤,无模块契约 | 将来的可选 `StepMeta` |
| 多 `BlockUse` 复合块 | 子块各自的契约;父块只定义组合说明与绑定 |
| 已冻结 Run | 冻结 Manifest;不重新查询当前目录 |

- 同一字段不得在 `module_contract` 和 `StepMeta` 各存一份权威值。
- `StepMeta` 只能存模块契约未覆盖的步骤级补充;若冲突,以模块契约为准。
- 修改模块科学知识 = 新模块版本 + 重新审核;不能直接改已审核版本。
- `time_scope` 只在编译后的 Protocol 步骤保留一份有效值;模块只给默认/允许集合。
- `applies_when` 条件转移是结构表达;执行器必须选择具体分支,不能把全部条件转移当成同时发生。

## 5. 与现有模型的映射

| OperationModule / 绑定 | 现有对象 | 处理 |
|---|---|---|
| `CatalogModuleRef` | 无 | 知识目录身份;不是数据库 ID |
| `ModuleBinding.block_version` | `ProtocolBlockVersion` | 项目内实例;`block_id + version_id` |
| `ModuleIdentity.catalog` | 无 | 编译进 block-level `module_contract` sidecar |
| `OperationParameter` | `ParameterSlot` | 未来扩展可选 `description/source/required_at/constraints/evidence` |
| `ParameterBinding` | 现有 `ParameterBinding` | 复用绑定语义;增加来源证明字段 |
| `default_time_scope` | `ProtocolStep::Text.time_scope` | 编译后只保留步骤的最终值 |
| `InstrumentRequirement` | `StepMeta.instrument`(P4 候选) | 只存能力引用,不存型号设置 |
| `ModuleKnowledge` | 无 | K3 随 `module_contract` sidecar 保存 |

**存储选择:** 模块级契约放在 `ProtocolBlockVersionData` 的可选 `module_contract` sidecar;不在每个步骤重复。`StepMeta` 仍可存在,但只放步骤级子集。

`BlockUse` / `ParameterBinding` 继续引用确切 block version;同一模块插入两次是两个使用位置,绑定互相隔离。Run 冻结确切版本与参数,不重新解析最新模块。

## 6. 兼容规则

1. 旧块可读:没有 `module_contract` 的块按 legacy 处理,不推断语义。
2. 新字段可选;旧记录不迁移。
3. 引用必须精确版本;拒绝 `latest`/范围/标题匹配。
4. 旧 Run 不变;冻结内容不重新解析。
5. 契约类型使用 `deny_unknown_fields`;错字不能静默丢失。
6. 不承诺旧二进制读新 contract;单机升级整体替换。
7. `review_status` 的结构校验不等于科学审核;审核边界见第 7 节。

## 7. 审核边界

`validate_contract()` 只检查结构合法性;它**不是**审核通过。

| 机制 | 检查 |
|---|---|
| `validate_contract()` | 类型、ID、引用、范围、单位、来源字段、样本转移、时间作用域 |
| `audit_evidence()`(K3 提案) | 证据定位、适用条件、逐字段映射、审核者/时间/内容摘要 |
| 专家评审 | 由具备实验经验的人确认科学内容 |
| 实验室验证 | 真实 SOP/流程验证 |

- `SourceChecked` 目前只要求至少一条 evidence,是结构门槛,不是科学审核。
- 一个 citation 存在不能自动升级;正式审核记录应绑定不可变模块版本。
- 模块内容改变后,旧审核不自动延续到新版本。

## 8. K1.1 结构校验

| 代码 | 规则 |
|---|---|
| `schema_version` | 必须为 1 |
| `stable_id` | catalog/module/port/transition/binding ID 格式 |
| `version` | ≥1 |
| `variant_self_reference` | 不能引用自身 |
| `parameter_key` | 小写字母开头,字母/数字/下划线 |
| `duplicate_parameter` | 参数 key 唯一 |
| `unit_on_non_numeric` | 只有 number/integer 可声明单位 |
| `parameter_range` | min 不得大于 max |
| `one_of_empty` / `one_of_type` | 枚举非空且符合参数类型 |
| `max_length_constraint` | 只用于 text |
| `binding_unit` / `binding_type` / `binding_min` / `binding_max` / `binding_one_of` | 绑定值与参数定义一致 |
| `binding_source_reference` | protocol_fixed/lab_sop 绑定必须有具体来源 |
| `unknown_with_value` | unknown 不能携带伪造值 |
| `transition_input` / `transition_output` | transition 必须引用存在的端口 |
| `transition_output_required` | moved/aliquoted/state_changed 必须有输出 |
| `material_output_required` | 液体转移/接种/取样必须有材料输出 |
| `unreferenced_input` / `unreferenced_output` | 样本/试剂/材料端口必须被 transition 引用 |
| `sample_effect_ambiguous` | unknown 不能和已知 effect 混用 |
| `time_scope_not_allowed` | 默认值必须在允许集合内 |
| `profile_version` | profile_id 与 profile_version 同时出现 |
| `capability_range` | 能力范围 min ≤ max |
| `evidence_required` / `evidence_citation` | 非 research_draft 必须有 evidence;citation 非空 |

不校验:具体波长是否适用于某细胞、某型号是否真的支持某板型、对照是否科学充分。这些进入 K2/K3/专家审核。

## 9. Rust 测试

代码位置:`crates/asterlab-core/src/operation_module.rs`。当前 26 个测试,覆盖:

- 最小契约、精确版本引用;
- 同目录模块跨项目绑定不同 block version;
- 派生绑定必须有来源;
- 未知样本效果不能混用;
- 空输出液体转移被拒绝;
- transition 端口引用;
- 时间作用域默认/允许集合;
- 证据门槛、单位规则、重复参数;
- min/max、one_of 类型、capability range;
- profile 版本、建议值不自动成为计划值;
- LabSop 类别本身不授权绑定;
- 已确认来源绑定才可计划;
- 单位/范围绑定检查;
- 序列化保留来源与时间作用域;
- 审核等级单调。

运行:

```bash
cargo test -p asterlab-core operation_module
```

## 10. 未决项

1. `module_contract` sidecar 在 `ProtocolBlockVersionData` 的具体字段名;K3 实现。
2. `ModuleBinding` 是否随 sidecar 保存,还是由 `ProtocolBlockVersion` 外部映射表维护;K3 决定。
3. `StepMeta` 最终字段边界;K1.1 只冻结“不得覆盖 module_contract 权威字段”。
4. 参数单位量纲/换算;K2 只做“未知/待核”,不猜兼容。
5. 正式审核记录(审核者、时间、内容摘要)的存储;K3。
6. 仪器 profile 的版本/导入/审批流程;K2。

## 11. 完成条件

- [x] 两层身份:catalog ID / project block version ID。
- [x] 导入、派生、本地自定义的来源关系。
- [x] 唯一知识来源与时间作用域规则。
- [x] 端口级样本转移与反例。
- [x] 参数结构校验与绑定来源证明。
- [x] 审核边界说明。
- [x] 26 个 Rust 测试。
- [ ] K1.1 审查通过。
- [ ] 确定 sidecar 字段与 StepMeta 边界。
- [ ] 才进入 K2 实现。

## 12. 来源

- Core Contract:[`core-v1.md`](core-v1.md)
- K0 审计:[`../research/2026-11-05-k0-knowledge-coverage-audit.md`](../research/2026-11-05-k0-knowledge-coverage-audit.md)
- 协议知识:[`../research/2026-11-05-protocol-knowledge-and-related-projects.md`](../research/2026-11-05-protocol-knowledge-and-related-projects.md)
- ISA Abstract Model:https://isa-specs.readthedocs.io/en/latest/isamodel.html
- SiLA 2:https://sila-standard.com/standards/
- OBI:https://obi-ontology.org/docs/core-classes/

这些标准只借 Process/Input/Output/Parameter/Capability 的模式;不实现完整本体。
