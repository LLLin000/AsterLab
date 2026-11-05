# 组件体系与 LabwareProfile 设计评审

- 日期:2026-11-06
- 状态:评审通过;C3 架构基线(待实现),不是实现记录。
- 基线:提交 `1ba6189`;C0/C1.1/C2-A/C2-B1/C2-B2.1 已有实现。
- 范围:评审“少量通用组件 + 可扩展专业规格 + Agent 自由组合”的组件体系,重点看表格、孔板、计算器和四类关系。

## 结论

方向通过,可以作为 C3 架构基线,但不能把 12 个 Definition 直接当成已实现组件,也不能在计划容器契约之前先做全部孔板/计算器。

修订后的关键决定:

1. **通用表达**:`content.table` 分 authored / bound;`tool.calculator` 由 `ToolActionSpec` / `tool_ref` 选择算法;不为每种实验内容新建组件类型。
2. **单一规格权威**:有计划容器时,`LabwareProfile` 由 `PlannedContainer` 决定;`lab.well_plate` 不得另选一个与计划容器不一致的 Profile。无计划容器时,View 可以临时选 Profile 仅作预览。
3. **计划容器进入 DesignVersion**:`PlannedContainer / Position / PositionAssignment` 是科学计划快照的一部分,Run 创建时冻结。
4. **孔位可多目标**:`PositionAssignment` 支持多个 typed target,但必须有 role、relationship、source 和计数规则;多个关联对象不自动等于多个独立 n。
5. **物理包含独立**:`ContainerPlacement` 表示爬片、Insert、管等进入另一容器/位置,不能简化为一个 Sample 标签。
6. **Profile 字段有知识状态**:`known / unknown / not_applicable / conflicted`,字段级来源;96 孔不能推导表面、面积、裙边或仪器兼容性。
7. **Authored Table 可预览计算**:可使用未确认内容做 draft/unverified 计算,但不能自动提升为科学事实;正式写入需解析、差异预览、确认和新 DesignVersion。
8. **顺序反转**:先冻结 C3-0/C3-1 契约与计划容器模型,再实现 `content.table`、`tool.calculator`、`lab.well_plate` 三个组件,最后做三案例。

因此:三层架构通过;12 个 Definition 只作为目录;C3-1 的 LabwareProfile + Container/Position/Assignment/Placement 必须先冻结。

## 1. 现有实现与提案的对应关系

| 提案 | 现有实现 | 判断 |
|---|---|---|
| Component Definition | `ComponentDefinition` + `ComponentRegistry` + `ComponentFamily` | 可扩展,方向正确 |
| Component Instance | `ComponentInstance` + `instance_id` + `model_ref` + `slots` | 基础可用 |
| Workflow | `WorkflowGraph` + visual edges | 已明确“视图组织不等于 Protocol 顺序” |
| Content / Layout / Tool | 目前只有 `table` / `well_plate` / `dilution` / `note` | 不足 12 个,但可通过 Definition 扩展 |
| Scientific refs | `ModelRef` 支持 DesignVersion / ProtocolVersion / BlockVersion / OperationUse / Run | **缺 LabwareProfile / PlannedContainer / PositionAssignment** |
| Typed binding | 组件端口、ModelPath、Contextual Validation | 可用于表格/计算器,但无孔位分配类型 |
| Agent patch | `ViewChangeSet` 原子批量修改 | 可编辑展示层,尚不能改科学事实 |

## 2. 三层分离是对的,但边界要写死

提案中的三层应明确为:

```text
Component Definition
  我怎样表达和交互:workflow / operation / layout.section / content.table ...

Profile / Template / Content
  我使用哪种规格、预置或内容:96-well cell culture / PCR plate / table rows ...

Scientific Core
  实验事实:DesignVersion / Group / SampleAllocation / PlannedContainer /
  PositionAssignment / ProtocolVersion / BlockUse / Run
```

关键约束:

- Definition 不知道“96 孔”“CCK-8”“浓度表”;它只知道 props schema、ports、slots、renderer、operations。
- Profile 不是组件实例;它是可复用、可版本化、带来源的规格。`LabwareProfile` 不能成为 `ComponentDefinition` 的兄弟组件。
- Content 不是科学事实;Markdown 表格、备注、图片只是 View 内容。
- Template 不是新权威;它只能保存“Definition 引用 + 默认 props/slots + 绑定预设”,不能复制 Protocol/Run。

## 3. 表格:通用 `content.table` 合适,但必须分 authored / bound

当前 `layout.table` 只有 `inputs.data` / `outputs.values` 和 `props.rows`。它能表达简单数据,但没有明确“作者写的表格”和“引用科学数据的表格”的区别。

建议:

```text
content.table
  props:
    mode: authored | bound
    markdown?: string
    columns?: [{key,label,align}]
    rows?: [...]
  inputs:
    data: Data (仅 bound)
  outputs:
    values: Data
```

规则:

- `authored`:内容由用户/Agent 填写;不是实验设计事实,不能被 Protocol 自动消费。
- `bound`:内容从 DataBinding 生成;可配置显示列、格式、排序,但不能反写科学数据。
- 从 authored 到正式实验组/孔位/剂量,必须经过显式“提升/导入”命令并做 Core 校验。
- 不为“浓度表 / 分组表 / 结果表”分别建组件;它们只是不同内容/绑定。
- P0 支持行列/Markdown 导入导出;合并单元格等复杂表格延后。

## 4. 孔板:ProfileRef 的单一权威

组件、Profile、计划容器、实际容器必须分开,但 `lab.well_plate` 不应该自己决定 Profile。

```text
lab.well_plate (ComponentDefinition)
  └─ model_ref: PlannedContainer / ActualContainer
  └─ profile_override?: LabwareProfileRef   // 仅预览,不适用于已绑定计划容器
```

规则:

- 有 `PlannedContainer` 时,Profile 由 `PlannedContainer.profile_ref` 决定。
- 组件不能选择与计划容器不一致的 Profile;UI 只读显示该 Profile。
- 没有计划容器时,组件可以用 `profile_override` 做预告/预览,但必须标注“未绑定计划容器”,不能生成计划孔位。
- `ActualContainer` 的 Profile 由实际使用记录或计划容器冻结值决定,不能由 View 改写。

这样避免“Core 认为是 PCR 板,界面按细胞培养板显示”的第二权威。

## 5. PlannedContainer / Position / PositionAssignment

最小模型:

```text
PlannedContainer {
    id,
    profile_ref,
    label,
    source,
    evidence?
}

Position {
    container_id,
    position_id,        // A1, B2...
    // 合法集合由 profile 的 coordinate_system 生成或校验;
    // 不必把 96 个空孔都存成记录。
}

PositionAssignment {
    container_id,
    position_id,
    targets: Vec<AssignedTarget>
}

AssignedTarget {
    target_ref,         // Sample / Reaction / Material / Data ...
    role,               // primary | auxiliary | control | ...
    relationship,       // contains | uses | produced_from | ...
    source,
    counts_as_independent_unit: bool
}
```

规则:

- `PositionAssignment` 属于 DesignVersion 计划快照;Run 创建时冻结。
- 一个孔可以关联多个 typed target,但必须有角色和关系。
- `counts_as_independent_unit` 明确计数;多个关联对象不自动增加 n。
- PCR 的 A1 可关联 `Reaction-R01`;模板、引物、组分作为该 Reaction 的关联,不强行变成孔内并列主对象。
- 计划孔位不等于实际孔位;实际使用由 Run/ExecutionEvent 记录。
- 一个 `PlannedContainer` 可被接种、加药、孵育、检测多步骤共用;View 多处显示只是引用。

## 6. ContainerPlacement:容器内部包含

需要独立于孔位分配:

```text
ContainerPlacement {
    parent_container_id,
    position_id,
    child_container_id,
    relation,          // placed_in | inserted_into | mounted_on ...
    source
}
```

例子:

- 六孔板 A1 放置 Coverslip-01;
- Transwell Insert-01 放入 A1,具有 upper/lower chamber 与 membrane;
- 培养瓶/离心管/管架中的物理包含。

规则:

- 这是 Scientific Core 关系,不是 View slot。
- 不发展成三维场景系统;只记录容器/位置/关系/来源。
- 一个位置可以有放置关系,但不因此自动获得 Sample 或 Reaction。
- Transwell 的膜面积、孔径、上下室关系属于 Profile/Insert 规格,不能从六孔板推断。

## 7. 四类关系重新确认

| 关系 | 例子 | 权威 |
|---|---|---|
| 视觉包含 | 药物处理详情显示 Plate-01 | ViewDocument slot tree |
| 实验使用 | 加药步骤 uses Plate-01 | Protocol/BlockUse + ContainerUse |
| 物理包含 | Plate-01 A1 放 Coverslip-01 | PlannedContainer + ContainerPlacement |
| 样本/反应分配 | A1 → Control/S01/Reaction-R01 | DesignVersion PositionAssignment |
| 实际执行 | Run 的 ActualPlate/Well | Run/ExecutionEvent |

`lab.well_plate` 只是显示；它不拥有以上任何科学事实。

## 8. LabwareProfile 字段与知识状态

字段分组保持:

```text
geometry: rows, columns, well_shape, bottom_shape, coordinate_system,
          footprint, height, pitch
physical: nominal_volume, working_volume_range, culture_area,
          surface_treatment, material
use: sterility, temperature_range, optical_properties, sealing_type
compatibility: compatible_instruments, compatible_centrifuges,
               plate_reader/thermal_cycler constraints
provenance: manufacturer, catalog_no, evidence[], profile_version,
            review_status
```

关键属性必须是字段级状态,不能只靠空值:

```text
ProfileField<T> {
    state: known | unknown | not_applicable | conflicted
    value?: T
    evidence: []       // standard | vendor | local | project
    note?
}
```

- `known`:有明确值 + 来源。
- `unknown`:应有但未核实。
- `not_applicable`:该耗材不适用(如 PCR 板的 culture_area)。
- `conflicted`:多来源矛盾,保留冲突,不选一个“看起来合理”的值。
- 96 孔不能推导培养面积、表面处理、裙边、孔底、热循环兼容性。
- 标准尺寸(ANSI/SLAS)与厂商属性分开;标准不证明具体产品属性。
- Profile 版本不可变;项目自定义标 `local`,不能冒充已审核。

参考:[SLAS Microplate Standards](https://www.slas.org/education/ansi-slas-microplate-standards/)、[ANSI/SLAS 1-2004 footprint](https://www.slas.org/slas/assets/file/public/standards/ansi_slas_1-2004_footprintdimensions.pdf)、[ANSI/SLAS 2-2004 height](https://www.slas.org/SLAS/assets/File/public/standards/ANSI_SLAS_2-2004_HeightDimensions.pdf)。

## 9. Generic Table:authored 可预览,不可自动提升

```text
content.table
  props:
    mode: authored | bound
    markdown?: string
    columns?: [{key,label,align}]
    rows?: [...]
  inputs:
    data: Data (仅 bound)
  outputs:
    values: Data
```

- `authored`:用户/Agent 填写;不是科学事实;可用于 **draft/unverified** 计算预览。
- `bound`:从 DataBinding 生成;可配置显示列/格式/排序;不反写科学数据。
- authored 表格进入正式组别、剂量、孔位、反应:必须走正式 Core 命令:
  `解析 → 差异预览 → 用户确认 → 新 DesignVersion`。
- 计算器对 authored 输入的输出必须标记 `draft / unverified`;不能覆盖正式计划。
- 不为浓度表/分组表/结果表分别建组件。

## 10. Calculator:一个组件,多个算法

```text
tool.calculator
  props:
    tool_ref: ToolActionSpec@version
    preset?: {...}
  inputs:
    values: Data / scalar ports
  outputs:
    result: Data
```

- 稀释、铺板、Master Mix、试剂总量、梯度、单位换算复用同一个组件。
- 算法/单位/来源/确定性由 `ToolActionSpec` / `PureTool` 决定。
- 不新增每类计算器的 View 组件。
- 计算器输出是派生数据;不能反向覆盖孔板计划或科学参数。
- Authored table 可作临时输入,但结果保持 draft/unverified。

## 11. 12 个 Definition 的目录定位

12 个组件作为 **目录**,不等于 12 个都已实现/可渲染:

| ID | Family | 备注 |
|---|---|---|
| `workflow` | Workflow | 保留 |
| `operation` | Operation | 保留,OperationUse |
| `layout.section` | Layout | 新增;分组/折叠 |
| `content.markdown` | Content | 新增;替代/补充 note |
| `content.table` | Content | 新增;authored/bound |
| `lab.well_plate` | Layout | 保留;profile 由容器决定 |
| `lab.container_view` | Layout | 新增;皿/瓶/管 |
| `lab.rack_layout` | Layout | 新增;管架/冻存盒 |
| `tool.calculator` | Tool | 新增;替代 dilution |
| `data.reference` | Data | 新增 |
| `visual.chart` | Visualization | 新增 |
| `media.figure` | Visualization | 新增 |

- `ComponentFamily` 不加 12 个枚举;只加 Definition。
- 旧 `layout.table` / `content.note` / `tool.dilution` 保留只读重开;新编辑迁移到通用 ID。
- C3-3 只先实现 `content.table`、`tool.calculator`、`lab.well_plate` 三个;其余目录可先注册 schema,不宣称可用。

## 12. 修订后的开发顺序

| 阶段 | 工作 | 验收 |
|---|---|---|
| C3-0 | 冻结组件目录、Table authored/bound、Calculator、ProfileRef 契约 | Definition + 契约 |
| C3-1 | 冻结 LabwareProfile、PlannedContainer、PositionAssignment、ContainerPlacement | 科学计划模型 |
| C3-2 | 接入 DesignVersion、Profile Registry、Run 冻结 | 计划/执行分离 |
| C3-3 | 实现 `content.table`、`tool.calculator`、`lab.well_plate` | 三个可用组件 |
| C3-4 | CCK-8、PCR、爬片/Transwell 三案例 | 端到端引用/计算/来源 |

先冻结底层契约,再扩充 Definition;不先做 12 个空组件。

## 13. 现有代码的 scope 缺口

`1ba6189` 的 `DbComponentContext::operation_use_exists()` 已比较 `step_path` 与 `source_block_version_id`,但:

- 尚未比较 `QualifiedStepRef.scope`;
- `view.project` 统一生成 `ProtocolScope::Unscoped`。

因此多 Assay 场景下,View 节点还不能宣称具有精确作用域身份。应在 C3-1 前后做小型回归修复:让投影/解析携带真实 Assay/PlannedOutput scope,并比较 scope;不要为它新建一套作用域系统。

## 14. 已决定的问题

1. `PlannedContainer / PositionAssignment` 进入 DesignVersion,Run 创建时冻结。
2. 一个孔位允许多个 typed target;必须有 role、relationship、source、counting。
3. `LabwareProfile` 允许项目自定义,但标 `local`;未核实字段保持 unknown/conflicted,不能冒充已审核事实。
4. authored 表格提升为科学事实必须走正式 Core 命令 + 差异预览 + 确认 + 新 DesignVersion。
5. 旧组件保留完整只读兼容;新编辑使用通用 Definition,迁移显式进行。

## 15. 评审判断

- 三层架构、通用表格、通用计算器、Profile 化孔板:通过。
- 12 个 Definition 直接冻结为全部实现:不通过;只能作为目录。
- 计划容器/位置/包含关系先于孔板联动:必须。
- “组件显示多次 = 多个实体”/“自动补 Profile 默认值”/“authored 表格自动成为事实”:禁止。
- 下一步:先提交 C3-0/C3-1 契约修订与测试,不立即实现全部 C3。

本文件是 C3 架构基线草案;Labware 物理属性正式使用前仍需按具体厂商/标准逐项核对。
