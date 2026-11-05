# Unified Component Framework v1(C0/C1)

- 日期:2026-11-05
- 状态:C0 + C1.1 + C2-A 已实现;不是 UI 规格。
- 代码:[`component_framework.rs`](../../crates/asterlab-core/src/component_framework.rs)
- 边界:不创建第二套 Design/Protocol/Run,不创建 Component 数据库,不替代科学 Core。

## 1. 分层

```text
Unified Component Framework
  component registry · DSL · recursive composition · typed binding · semantic patch
        │ model_ref / bind
        ▼
Scientific Core
  Design · OperationModule · ProtocolBlockVersion · Protocol · Run
```

组件框架描述“实验方案由什么组成、如何呈现、如何嵌套、如何被 Agent 修改”。科学 Core 仍回答操作、参数、样本、版本和冻结。

## 2. 组件家族与实例机制

| 组件 | 实例机制 | 说明 |
|---|---|---|
| `workflow` | graph + `detail` slot | 流程图节点/边;详情是布局树 |
| `operation` | `operation` instance + OperationUse | 同时引用 BlockVersion 定义和 QualifiedStepRef 使用位置 |
| `layout.table` | instance + props | 表格内容;可绑定 Design 数据 |
| `lab.well_plate` | instance + props | 孔板布局;引用真实设计/样本绑定 |
| `tool.dilution` | instance + inputs/outputs | 纯计算工具;不创建 ProtocolStep |
| `content.note` | instance + props | 普通说明内容 |

`BlockUse` 只负责 Operation/Workflow 的科学实例,不强行承载表格、孔板、计算器和详情布局。

## 3. 统一对象

```text
ComponentDefinition {
  id, version, family, title,
  props_schema, capabilities,
  inputs[], outputs[], slots[],
  presentation?, agent_operations[]
}

ComponentFamily =
  workflow | operation | layout | tool | content | data | visualization

// id 是具体组件身份,例如 lab.well_plate,不是核心枚举。
// slot.accepts 接受 family;slot.accepts_definitions 可选精确 id 白名单。

ComponentInstance {
  instance_id,
  definition: { id, version },
  model_ref?: DesignVersion | ProtocolVersion | BlockVersion
             | OperationUse { block_version_id, step: QualifiedStepRef }
             | Run,
  props,
  slots: { slot_id: [ComponentInstance] },
  workflow?: { nodes[], edges[] },
  visual?
}

ViewDocument {
  schema_version,
  document_id,
  revision,
  root: ComponentInstance,
  bindings[]
}

ViewChangeSet {
  changeset_id, document_id, expected_revision,
  operations: ViewPatch[], provenance[]
}

DataBinding {
  binding_id,
  source: Model(path) | ComponentPort(instance, port),
  target: ComponentPort(instance, port),
  value_type
}

ViewPatch =
  UpdateProps | AddSlotChild | MoveSlotChild | AddWorkflowNode
  | RemoveInstance | Connect | Disconnect
  | AddBinding | ReplaceBinding | RemoveBinding
```

`WorkflowGraph` 的 node/edge 只是视图组织:它可以表达“图上怎么连”,不能改变 Protocol 执行顺序。科学顺序、条件、样本流必须通过正式 Protocol/Composition 命令修改。

`OperationUse` 把 BlockVersion 定义身份与 `QualifiedStepRef` 使用位置分开;View 的 `instance_id` 不是科学步骤身份。

## 4. DSL 示例

```yaml
schema_version: 1
document_id: experiment_main
root:
  type: workflow
  id: experiment_main
  model_ref: { kind: protocol_version, id: protocol-v1 }
  workflow:
    nodes:
      - id: step_drug
        type: operation
        model_ref: { kind: block_version, id: drug-treatment-v1 }
        slots:
          detail:
            - type: layout.table
              id: concentration_table
              bind: design.concentrations
            - type: lab.well_plate
              id: plate_layout
              props: { format: "96" }
            - type: tool.dilution
              id: dilution_tool
              inputs: { targets: concentration_table.values }
            - type: workflow
              id: drug_subflow
              model_ref: { kind: block_version, id: drug-prep-v1 }
```

正式表示使用 `ViewDocument` 的 JSON 结构;上面的 YAML 仅用于阅读。所有 `model_ref` 最后解析为现有 Core 的精确版本引用,不靠自由文本猜测。

## 5. 校验

- Definition 必须精确 id + version 命中注册表;同版本不可被不同内容覆盖。
- `ComponentFamily` 固定;新组件通过注册具体 `id` 扩展,不需要修改核心枚举。
- `instance_id` 在整份文档内唯一;递归 slot 与 workflow node 共享命名空间。
- `props` 必须满足 Definition 的 schema 子集。
- slot 必须声明;`one` 最多一个;required slot 非空;child kind 必须在 accepts 中。
- workflow node id 唯一;edge 必须指向节点;禁止自连和环。
- binding target 必须是声明 input;source 必须是声明 output 或 model path;类型必须一致;一个 input 只接受一个 binding。
- ComponentPort 之间的数据依赖环必须拒绝。
- Structural Validation 只证明端口/类型/路径形状;`validate_with_context` 才解析 ModelPath 与 OperationUse。
- 未解析/未知 ModelPath 保持 unresolved,不生成假数据。
- view 层不验证科学参数;参数合法性仍由 S1/S3 的 Block/Operation 校验负责。

## 6. C1.1 / C2-A Fixture

测试文档包含:

- workflow 根节点,引用 ProtocolVersion;
- Operation 节点,引用同一 BlockVersion 的 `OperationUse` 与独立 QualifiedStepRef;
- 表格、96 孔板、稀释计算器、说明卡片;
- Operation 的 detail 中嵌套 Workflow;
- 子 Workflow 中两个 Operation 复用同一 BlockVersion、使用不同 step path;
- Design 数据到表格、表格到计算器的 typed binding。

测试覆盖:

- 保存/JSON round-trip/重开;
- 递归 slot 与嵌套 workflow;
- 重复 instance、错误 slot family、缺失定义版本;
- workflow 环与 binding 环;
- `UpdateProps`、`AddSlotChild`、`MoveSlotChild`、`RemoveInstance`、`Connect/Disconnect`、`Add/Replace/RemoveBinding`;
- 批量 changeset 的 expected_revision、原子失败、删除清理绑定;
- 自定义组件 `lab.solution_calculator` 通过注册扩展;
- Structural / Contextual 绑定校验分离。

## 7. C2-B1/B2 持久化与 Resolver

已增加正式命令:

```text
view.create  创建 view + view_document v1
view.get     读取 view 与全部不可变版本
view.revise  按 expected_revision 应用 ViewChangeSet
```

- 复用现有 `records` 表,不新增组件表。
- 每次成功修订生成新的 `view_document` 版本,旧版本不改。
- 修订事务内检查 `ViewChangeSet.expected_revision`。
- `view.create` / `view.revise` 都执行结构校验和真实 Contextual Validation。

最小 Resolver `DbComponentContext`:

- 校验 DesignVersion / ProtocolVersion / BlockVersion / Run 记录类型。
- ModelPath 遍历 DesignVersion snapshot 字段;缺失为 Missing,无法确定为 Unknown。
- OperationUse 至少验证 BlockVersion 与 ProtocolVersion 存在。
- 尚未验证 OperationUse 的 step path 是否与 BlockUse 精确一致;这是 C2-B2 的剩余缺口。
- 未接入 Frozen Run 的完整历史语义快照解析。

## 8. 非目标

- 不实现 Slint/SVG/前端画布。
- 不建立 Component 数据库、Workflow Runtime 或第二套 Protocol。
- 不把 `purpose` 当 SOP,不从 UI 反向修改科学定义。
- 不让 Agent 直接编辑任意 JSON 路径。
- 本阶段不声称 ViewDocument 与 Scientific Core 的跨层修改已经原子提交。

## 9. C3-0 / C3-1（当前）

C3-0 目录:

- 12 个 Definition 已注册:`layout.section`、`content.markdown`、`content.table`、`lab.container_view`、`lab.rack_layout`、`tool.calculator`、`data.reference`、`visual.chart`、`media.figure`,以及原有 workflow/operation/well_plate。
- 旧 `layout.table`、`content.note`、`tool.dilution` 保留只读兼容。
- `content.table` 支持 authored/bound 的 props 结构;`tool.calculator` 通过 `tool_ref` 选择算法。
- 这只是目录/契约,不代表 12 个组件都有完整渲染器。

C3-1 计划容器:

- `LabwareProfile` + `ProfileField` 的 known/unknown/not_applicable/conflicted 状态;注册时校验 state/value/evidence。
- `PlannedContainer`、`PositionAssignment`、`AssignedTarget`、`ContainerPlacement`。
- `ContainerPlan` 校验包含环、重复 child、重复位置、重复分配、多个 primary target、非法/Unicode 孔位。
- `ContainerPlacement` 区分 compatible / incompatible / unknown;unknown 可以保存草案,不能宣称已通过物理兼容。
- DesignVersion 增加 `container_plan`,并可通过 `design.revise` 替换生成新版本。
- Run 创建时把该计划冻结进 run data。
- `labware.define` 持久化版本化 Profile;Design 校验从持久化 Registry 加载。
- `ModelRef::PlannedContainer { design_version_id, container_id }` 可精确引用同一计划容器。
- 默认 Profile 为 `research_draft`,物理字段未核实;不提供推测体积/表面/兼容性。

剩余:

- Run 只保存 ProfileRef,尚未保存 Profile 内容摘要/完整解释快照。
- `OperationUse.scope` 仍未与真实 Assay scope 精确比较。
- Generic Table / Calculator / WellPlate 完整数据适配未实现。

## 10. C2-B2.1 与首个视图切片

- `DbComponentContext` 携带项目范围,所有 ModelRef 检查记录类型、项目归属和精确版本。
- OperationUse 通过真实 ProtocolPreview 展开步骤校验 `step_path + source_block_version_id`,不再只检查两个记录存在。
- ModelPath 对 Design/Protocol/Block/Run 都执行字段路径解析;不存在为 Missing,未知为 Unknown。
- `ContextValidationMode::AllowUnresolved` 允许草稿保存 Unknown,但 Missing/类型冲突仍拒绝。
- `component.define` 持久化自定义 Definition;View 创建/修订从注册表加载。
- `view.project` 从 ProtocolPreview 生成只读初始 ViewDocument。
- `web/view.html` 提供最小交互切片:
  - Workflow 节点与箭头;
  - 选中节点查看 detail;
  - Note 文本编辑通过 `ViewChangeSet` 保存;
  - 嵌套 Workflow 进入/面包屑导航。
- UI 目前只读/展示 + Note 编辑,不修改科学参数。

## 11. 下一步

C2-B Persistence & Scientific Core Bridge:

- ViewDocument 正式保存/修订适配;
- 把涉及科学参数的修改路由到正式领域 Command;
- 跨 ViewDocument 与 Scientific Core 的统一变更评估与提交边界;
- 接入真实 Design/Protocol/Run Context 的 ModelPath/OperationUse 解析。

C3 再接入孔板、计算器真实数据;C4 才做 presentation adapter 与 UI。
