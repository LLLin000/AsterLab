# Semantic Component Contract v1(S0)

- 日期:2026-11-05
- 阶段:S0;语义契约与映射,不是 Rust 实现,不是 UI/JSON 批量生成。
- 目标:让已有操作、方法、工具和资源拥有统一的**语义接口、引用机制与组合规则**,但不新建第二套组件运行时。
- 状态:研究契约;所有相关记录仍为 `research_draft`,没有 `SourceChecked`。

## 0. 一句话

统一的是**语义能力**,不是存储结构:

```text
ComponentDefinition  ──引用/版本──>  ComponentInstance
        │                              │
        ├─ Operation                   ├─ BlockUse
        ├─ Composition                 ├─ ParameterBinding
        ├─ ToolAction                  ├─ Typed Binding / Reference
        └─ Resource                    └─ Run freeze
```

- 不新建 `component` / `component_instance` 数据库表。
- 不建立第二套 ProtocolBlock 或 Workflow 引擎。
- `ComponentDefinition` 是现有对象的**语义视图**;`ComponentInstance` 是现有 `BlockUse` / 绑定 / 引用的**语义视图**。
- Design / Protocol / Run 三层权威边界不变。

## 1. 五个语义能力

| 能力 | 回答 | 现有对象 | 是否新建持久化 |
|---|---|---|---|
| Operation | 一步实验操作是什么 | `OperationModule` / `ProtocolBlockVersion` 的单步骤 | 否 |
| Composition | 多个操作如何嵌套/排序/依赖 | `ProtocolVersion` + 递归 `BlockUse` | 否 |
| ToolAction | 确定性计算/检查/预览 | 当前只有文档;需 Core 类型 | 否(先契约) |
| Resource | 仪器/材料/容器能力 | `InstrumentProfile`;材料/容器待补 | 否 |
| Typed Binding / Reference | 参数、样本、数据、结果、资源如何连接 | `ParameterBinding` / 端口 / TargetRef | 否(扩展语义) |

### 1.1 Operation

`OperationModule` 已定义操作身份、端口、参数、时间作用域、样本转换和仪器要求。S0 只把它视为 `ComponentDefinition` 的一种,不改变其字段。

### 1.2 Composition

一个 `ComponentDefinition` 可以是:

- 原子 Operation;
- 由多个 `BlockUse` 递归组成的复合块/ProtocolVersion;
- 方法模板(按顺序组合原子操作与条件步骤)。

组合只使用现有 `ProtocolBlockVersion` 的 `steps` 与 `BlockUse`;不另建 DAG 存储。

### 1.3 ToolAction

ToolAction 不是 Operation:

- 输入/输出 schema;
- 确定性算法/版本;
- 副作用级别(pure / database_write / external_effect);
- 测试用例。

当前 Core 没有 `ToolActionSpec`;S0 先冻结语义,不在本轮实现执行器。

### 1.4 Resource

Resource 是能力声明,不是操作:

- `InstrumentProfile` 已存在(K2);
- 材料/容器/耗材将来用 typed reference,不复制名称;
- 资源选择不等于校准/当前状态已确认。

### 1.5 Typed Binding / Reference

绑定不是把值塞进 JSON,而是:

```text
BindingRef {
  owner: Design | Protocol | Run,
  target: parameter | port | sample | data | resource,
  source: literal | design_ref | protocol_ref | run_ref | tool_output,
  unit, identity/lineage, version, provenance,
  state: resolved | unresolved | not_applicable
}
```

绑定必须明确归属,不能在 Design/Protocol/Run 之间互相覆盖。

## 2. 统一语义接口(不统一存储)

### 2.1 ComponentDefinition

```text
ComponentDefinition {
  kind: operation | composition | tool_action | resource,
  id, version,
  title,
  inputs[], outputs[],
  parameters[],
  constraints[],
  evidence[],
  review_status
}
```

映射:

| 语义字段 | OperationModule | ProtocolBlockVersion | ToolActionSpec | InstrumentProfile |
|---|---|---|---|---|
| id/version | identity.catalog | block_id/version | action_id/version | profile_id/version |
| inputs/outputs | semantics.inputs/outputs | 子步骤的端口汇总(待补) | input/output schema | capabilities/container |
| parameters | parameters | parameter_slots | input schema | capability parameters |
| constraints | execution/constraints | validation/ports(待补) | schema | constraints |
| evidence | knowledge.evidence | 待补 | 待补 | evidence |
| review | knowledge.review_status | 待补 | 待补 | review_status |

**不要求这些对象共享同一 Rust struct。** S0 只要求它们能被同一个语义查询投影。

### 2.2 ComponentInstance

```text
ComponentInstance {
  definition_ref,
  instance_id/use_id,
  path,
  bindings[],
  condition?,
  owner: Protocol | Run
}
```

映射:

- `BlockUse.use_id` = instance id;
- `BlockUse.block_version_id` = definition ref(精确版本);
- `BlockUse.bindings` = 当前参数绑定;
- `ProtocolVersion` / `protocol_manifest` = 实例所在上下文;
- Run 的冻结 manifest = 实例的冻结快照。

不新建表;不复制操作正文。

### 2.3 Composition

```text
Composition {
  parent_ref,
  children[],
  contains: parent → child,
  precedes: child → child,
  consumes/produces: port refs,
  binds: parameter/resource refs,
  conditional: condition ref
}
```

**权威顺序**仍是 `ProtocolBlockVersion.steps` 的数组顺序;`contains` 是结构视图,`precedes` 默认由数组顺序派生。只有当真实 Protocol 需要跨分支依赖时才提出显式 `precedes` 扩展;不为每个组合存一张边表。

### 2.4 Typed Binding

当前 `ParameterBinding` 只有 `parameter_key/value/unit/source/source_reference/confirmed`。S0 建议扩展为:

```text
Binding {
  binding_id,
  owner_scope: design | protocol | run,
  target: { kind: parameter|port|sample|data|resource, key/path },
  source: { kind: literal|design_ref|protocol_ref|run_ref|tool_output, ref, path },
  value_type, unit,
  identity: { sample_id? lineage? },
  provenance[],
  state: resolved | unresolved | not_applicable,
  frozen: bool
}
```

规则:

- 单位精确匹配;不做隐式换算(除非未来有显式 ToolAction)。
- 样本身份/谱系是引用,不是字符串复制。
- 计划值、实际值、原始读数、派生结果分开。
- 未解决用 `unresolved`,不能写 0/null 冒充。
- Run 只能冻结已确认绑定;修订产生新 Run/新版本。

## 3. 现有 Core 对象映射

| 语义对象 | 现有对象 | 映射方式 | 缺口 |
|---|---|---|---|
| Operation definition | `OperationModule` | catalog id + version | 未进入 ProtocolBlock |
| Block definition | `ProtocolBlockVersion` | block_id/version/parameter_slots/steps | 缺 inputs/outputs/condition |
| Block instance | `BlockUse` | use_id + exact block_version_id + bindings | 无 typed sample/data refs |
| Composition | `ProtocolVersion` + nested `BlockUse` | recursive steps | 无 explicit dependency/condition |
| Parameter binding | `ParameterBinding` | value/source/confirmed | 无 owner/port/sample/data refs |
| Tool action | 文档中的 `ToolActionSpec` | 待 Core 类型 | 无实现 |
| Resource | `InstrumentProfile` | profile_id/version/capabilities | 材料/容器未统一 |
| Run freeze | `RunWorkspace` / `protocol_manifest` | frozen design/protocol/steps | 绑定冻结语义待补 |
| Sample ref | `TargetRef::Sample` / `SampleLink` | 引用 | 与 Design sample allocation 未打通 |
| Data ref | 目前只有 `ResolvedParameter`/measurement | 需扩展 | 无统一 DataRef |

## 4. 语义命令(Agent 只操作语义,不写 JSON/SQL/UI)

| 命令 | 作用 | 是否改定义 | 是否改实例 |
|---|---|---|---|
| `inspect` | 读取 Design/Protocol/Run 语义结构 | 否 | 否 |
| `search` | 查找可复用 Operation/Composition/Tool/Resource | 否 | 否 |
| `compose` | 提议在父组合中插入/移动子块 | 是(需新版本) | 否 |
| `bind` | 绑定参数/样本/数据/资源 | 否 | 是 |
| `validate` | 结构/引用/单位/依赖/冻结检查 | 否 | 否 |
| `propose_patch` | 生成可审阅变更集,带 expected version | 否 | 否 |
| `apply_patch` | 事务提交变更集 | 视 patch 类型 | 视 patch 类型 |

规则:

- 修改已审核 Operation 的科学定义 = 新版本 + 重新审核;不能通过 `bind` 或 `patch` 偷改 `module_contract`。
- 修改本次实验参数 = `bind`,生成新 ProtocolVersion/Run 绑定。
- Agent 不直接写 SQLite;不直接改 Slint;不绕过 `propose_patch` 的版本检查。

## 5. 两个合成示例

### 5.1 多步骤 CCK-8 流程(示例数据,不是实验默认值)

```text
Design: assay CCK-8, groups G1/G2, timepoints D1/D3/D7, sample relation per assay
ProtocolVersion: CCK-8 / root composition
  C1 seed cells        -> Operation: inoculation / liquid_transfer
  C2 add treatment     -> Operation: liquid_transfer (dose binding)
  C3 incubate          -> Operation: incubation (time anchor = treatment)
  C4 add CCK-8         -> Operation: liquid_transfer
  C5 develop           -> Operation: incubation
  C6 read absorbance   -> Operation: absorbance_measurement (raw OD)
  C7 derive            -> ToolAction: blank/curve (not a step)
```

Bindings:

- volume: `literal` or `protocol_ref`; unit exact;
- plate/timepoint: `design_ref` to assay/group/timepoint;
- instrument: `resource_ref` to `plate_reader` profile; not a hard-coded model;
- raw OD vs derived viability: separate `DataRef`.

The same `op.liquid_transfer` appears in C1/C2/C4 but each is a different `BlockUse` with its own bindings.

### 5.2 同一操作模块在不同方法中复用

| 方法 | 使用 `op.liquid_transfer` | 差异 |
|---|---|---|
| CCK-8 | seed / add treatment / add CCK-8 | volumes, plate, timepoint |
| ELISA | coat / block / sample / detection / substrate | volumes, wash buffer, plate |
| biofilm CFU | dilution / plating | dilution factor, plate, sample lineage |

The operation definition is the same; the **bindings** and **method template** differ. No copied operation text.

## 6. 语义缺口(只提出最小扩展)

| # | 缺口 | 影响 | 最小扩展 |
|---|---|---|---|
| G1 | `BlockUse` 没有 typed inputs/outputs/condition | 组合只能靠顺序和文本 | 增加可选 `ports`/`condition` 语义字段,不建图引擎 |
| G2 | `ParameterBinding` 没有 owner/target/source ref | 参数、样本、数据无法统一连接 | 扩展 Binding 语义类型;先只读投影 |
| G3 | 无 `ToolActionSpec` Rust 类型 | 计算不能作为语义节点 | 先定义类型与 pure 校验 |
| G4 | `InstrumentProfile` 已有,但无 operation→resource 绑定 | 仪器能力无法在组合中解析 | 增加 `resource_ref` 绑定,不新建 profile 表 |
| G5 | 无 sample/data ref 统一类型 | 样本谱系/原始/派生数据断裂 | 引入 `SampleRef`/`DataRef`,复用 `TargetRef` |
| G6 | 无 composition cycle/dependency validation | 递归组合可能循环 | `validate` 增加 DAG/深度检查 |
| G7 | Run 冻结只覆盖 manifest,未覆盖 semantic bindings | 冻结后绑定漂移 | Run freeze 增加 binding snapshot(后续) |
| G8 | 无 `propose_patch` 事务契约 | Agent 修改不可审阅 | 先定义 change-set schema,不实现 UI |

## 7. 非目标

- 不新建一套 Component 数据库或运行时。
- 不实现通用 Workflow/DAG/事件总线。
- 不实现 SVG/布局树/低代码编辑器。
- 不批量生成 Operation JSON。
- 不修改 Slint/CLI/数据库。
- 不把 Design/Protocol/Run 合并成一个对象。

## 8. S0 验收

- [x] 五个语义能力与现有对象逐项映射。
- [x] 明确 Definition/Instance/Composition/Binding 不新建持久化。
- [x] Typed Binding 的归属、单位、身份、来源、冻结规则。
- [x] Composition 的 contains/precedes/consumes/produces/binds/conditional 区分。
- [x] 语义命令与定义/实例修改的边界。
- [x] 两个合成示例。
- [x] 语义缺口与最小扩展。
- [ ] 架构/科学审查通过。
- [ ] 才进入 S1 Rust 实现。

## 9. 来源

- [Core Contract v1](core-v1.md)
- [OperationModule v1.1](operation-module-v1.md)
- [Instrument Matching v1](instrument-matching-v1.md)
- [Protocol Discovery](../research/protocol-discovery/README.md)
- [Record standard](../research/record-standard.md)
- [Granularity freeze](../research/k2-operations/granularity-freeze.md)
