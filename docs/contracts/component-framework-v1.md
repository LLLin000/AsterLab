# Unified Component Framework v1(C0/C1)

- 日期:2026-11-05
- 状态:C0 契约 + C1 Headless Framework 已实现;不是 UI 规格。
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
| `operation` | `operation` instance + BlockUse ref | 引用真实 BlockVersion,不复制操作定义 |
| `layout.table` | instance + props | 表格内容;可绑定 Design 数据 |
| `lab.well_plate` | instance + props | 孔板布局;引用真实设计/样本绑定 |
| `tool.dilution` | instance + inputs/outputs | 纯计算工具;不创建 ProtocolStep |
| `content.note` | instance + props | 普通说明内容 |

`BlockUse` 只负责 Operation/Workflow 的科学实例,不强行承载表格、孔板、计算器和详情布局。

## 3. 统一对象

```text
ComponentDefinition {
  id, version, kind, title,
  props_schema, capabilities,
  inputs[], outputs[], slots[],
  presentation?, agent_operations[]
}

ComponentInstance {
  instance_id,
  definition: { id, version },
  model_ref?: DesignVersion | ProtocolVersion | BlockVersion | Run,
  props,
  slots: { slot_id: [ComponentInstance] },
  workflow?: { nodes[], edges[] },
  visual?
}

ViewDocument {
  schema_version,
  document_id,
  root: ComponentInstance,
  bindings[]
}

DataBinding {
  binding_id,
  source: Model(path) | ComponentPort(instance, port),
  target: ComponentPort(instance, port),
  value_type
}

ViewPatch =
  UpdateProps | AddSlotChild | RemoveInstance | Connect | AddBinding
```

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

- Definition 必须精确 id + version 命中注册表。
- `instance_id` 在整份文档内唯一;递归 slot 与 workflow node 共享命名空间。
- `props` 必须满足 Definition 的 schema 子集。
- slot 必须声明;`one` 最多一个;required slot 非空;child kind 必须在 accepts 中。
- workflow node id 唯一;edge 必须指向节点;禁止自连和环。
- binding target 必须是声明 input;source 必须是声明 output 或 model path;类型必须一致;一个 input 只接受一个 binding。
- view 层不验证科学参数;参数合法性仍由 S1/S3 的 Block/Operation 校验负责。

## 6. C1 Fixture

测试文档包含:

- workflow 根节点;
- operation 节点;
- 表格、96 孔板、稀释计算器、说明卡片;
- 嵌套 workflow;
- Design 数据到表格、表格到计算器的 typed binding。

测试覆盖:

- 保存/JSON round-trip/重开;
- 递归 slot 与嵌套 workflow;
- 重复 instance、错误 slot kind、缺失定义版本;
- workflow 环;
- `UpdateProps` 与 `AddSlotChild` 的语义 Patch;
- 所有修改后重新完整校验。

## 7. 非目标

- 不实现 Slint/SVG/前端画布。
- 不实现 C2 的全套 add/move/remove/connect 编辑器。
- 不建立 Component 数据库、Workflow Runtime 或第二套 Protocol。
- 不把 `purpose` 当 SOP,不从 UI 反向修改科学定义。
- 不让 Agent 直接编辑任意 JSON 路径。

## 8. 下一步

C2 Semantic Editing API:在 `ViewPatch` 上增加 move/connect/bind 的精确语义、expected version、影响评估与事务提交;C3 再接入孔板/计算器真实数据;C4 才做 presentation adapter 与 UI。
