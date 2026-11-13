# AsterDoc SDK 模块目录 v1

> 权威来源：`prototypes/mygo-run/internal/asterdoc/spec/modules.json`（foundationVersion 1，sdkVersion 1.0.0，17 个 `kind@schema`，对应 15 个 `kind`；`step` 的 schema 1/2/3 是同一 kind 的三个版本）。本文只描述已实现的事实，不描述未实现能力。

## 1. 已注册模块总表

下表按 `kind@schema` 列出 17 条；`step@1`、`step@2`、`step@3` 不是三个 kind，而是 `step` 的三个 schema 版本（15 个 kind、17 个 `kind@schema`：其余 14 个 kind 各 1 个 schema，`step` 有 3 个）。计数与 `spec/modules.json` 的 `modules[]` 长度一致（`python3 -c "...print(len(...))"` → 17）。

| kind@schema | category | capabilities | permissions | Design / Document / Run / Static | rules |
|---|---|---|---|---|---|
| plate@1 | labware | well-plate, well-assignment, plate-geometry | document.edit | plate.editor / plate.viewer / plate.run / — | plate-geometry-wells |
| reference@1 | document | typed-reference | (none) | generic.editor / reference.viewer / generic.run / — | reference-ref |
| experiment@1 | workflow | experiment-plan | (none) | generic.editor / experiment.viewer / generic.run / — | (none) |
| group@1 | workflow | group-definition | (none) | generic.editor / group.viewer / generic.run / — | group-fields |
| assay@1 | measurement | assay-plan, measurement-endpoint | (none) | generic.editor / assay.viewer / generic.run / — | assay-fields |
| step@1 | workflow | step-definition, workflow-edge | (none) | generic.editor / step.viewer / generic.run / — | step-fields |
| workflow@1 | workflow | workflow-graph | (none) | — (Workspace canvas) / workflow.viewer / generic.run / — | workflow-fields |
| dilution@1 | measurement | quantity, deterministic-calculation, derived-output | (none) | generic.editor / dilution.viewer / generic.run / — | dilution-fields |
| instrument_profile@1 | resource | instrument-profile, capability-declaration | (none) | generic.editor / instrument-profile.viewer / generic.run / — | instrument-profile-fields |
| step@2 | workflow | step-definition, workflow-edge | (none) | generic.editor / step.viewer / generic.run / — | step-fields, step-uses |
| step@3 | workflow | step-definition, workflow-edge | (none) | generic.editor / step.viewer / generic.run / — | step-execution, step-uses |
| material@1 | resource | material, role-declaration | (none) | generic.editor / material.viewer / generic.run / — | material-fields |
| sample@1 | resource | sample, sample-tracking | (none) | generic.editor / sample.viewer / generic.run / — | sample-fields |
| container@1 | labware | container, well-map | (none) | generic.editor / container.viewer / generic.run / — | container-fields |
| measurement_profile@1 | measurement | measurement-profile, calculator-reference | (none) | generic.editor / measurement_profile.viewer / generic.run / — | measurement-profile-fields |
| rna_reagent_usage@1 | measurement | quantity, deterministic-calculation, derived-output | (none) | generic.editor / rna_reagent_usage.viewer / generic.run / — | rna-reagent-fields |
| readout@1 | measurement | readout-schema, value-kind, run-record | run.readout | generic.editor / generic.editor / generic.run / — | (none) |

说明：`renderers.static` 当前未声明；静态预览走 `StaticPreview`/`StaticDocumentPreview`。

`readout@1` 是唯一「manifest 已声明、Go 原型未注册实现」的模块：Rust Core 从同一 `modules.json`
校验 `readout` 文档（输入/端口/量纲），Go `BuiltinRegistry()` 没有对应 `ModuleSpec`，因此 MyGo 侧没有
`readout` 的解析器、编辑器或 Run 视图；这是明确的能力缺口，不是回退猜测。读数本身不进入 AsterDoc
文档：形状由 `readout@1` 声明，值只经 Core 写入 Run 事实（`docs/contracts/readout-schema-v1.md`）。

`renderers` 是实际执行的契约。`Registry.RegisterRenderer` 将回调按 key 注册一次，多个 kind 可以共享；
`generic.editor` 和只读 `generic.run` 是共享实现。`plate` 使用独立的 `plate.editor`、`plate.viewer`、
`plate.run`。缺少某个 surface 时显示能力缺口，不回退猜测。`workflow` 的增删/排序/连线属于 Workspace
画布命令，因此它没有模块级 Design renderer。

## 1.1 模块就绪矩阵（独立维度，如实陈述）

注册 ≠ 可用编辑器 / Run 视图 / 记录闭环。下表按四个独立维度拆开，不把“已注册”混同为“可操作”。

| kind@schema | schema/校验 | Design 编辑器 | Run 视图 | 记录闭环 |
|---|---|---|---|---|
| plate@1 | Go+Rust parity；格式 6/12/24/48/96/384 | **96 孔交互式分配/清空**；6/12/24/48/384 为忠实只读矩阵，非完整专用编辑器 | 只读 `plate.run` 摘要 | 仅 CCK-8 结果网格 / Run Core 路径 |
| reference@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无 |
| experiment@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无 |
| group@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无 |
| assay@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无 |
| step@1 / step@2 / step@3 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无 |
| workflow@1 | Go+Rust | Workspace 画布（非模块级 Design renderer） | 只读摘要 | 无 |
| dilution@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无（derived，不写 Run 事实） |
| instrument_profile@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无；无设备控制 |
| material@1 / sample@1 / container@1 / measurement_profile@1 | Go+Rust | 通用编辑器仅标量字段；复合字段只读 | 只读摘要 | 无 |
| readout@1 | 仅 Rust（`src/cli.rs` 读 `modules.json`）；Go 未注册 | 无 | 无 | Core：`ReadoutRegistry` + `plan_readout_submission` 记录孔板/单值/表格读数，`query_readout_records` 读回 |

说明：
- 通用编辑器（`GenericModuleEditor`）对 `list/object/table/object-list/well-map/geometry` 字段为 read-only；复合字段目前无法在通用编辑器内组装。
- 模块级 Run 视图均为只读摘要；记录写入只通过 Core 命令（`run.workspace` / `execution_event.create` 等）。
- 本 pass 仅 CCK-8 结果网格 / Run Core 路径具备读数闭环；仪器控制未实现。

## 2. 端口（inputs / outputs）明细


### plate@1
- inputs: kind string required；id string required；schema int required；format int required（6/12/24/48/96/384）；assay reference→assay；timepoint timepoint；groups well-map required；samples object（well→显式 sample ID，可选）；replicates object（well→正整数技术重复编号，可选，须有对应 sample ID）。未填写的身份保持未解析，不根据组别或孔位顺序推断。
- outputs: rows int view；columns int view；wells well-map view；samples object view；replicates object view。clear_wells 同时清除目标孔位的组别、样本和重复映射。

### reference@1
- inputs: kind string required；id string required；schema int required；ref string required；view string。
- outputs: target string view。

### experiment@1
- inputs: kind/id/schema/title string|int required；objective、research_question、hypothesis、experimental_unit、controls、analysis_plan、sample_size_rationale、randomization、blinding、inclusion_criteria、exclusion_criteria string；factors object-list；timepoints string[]；primary_endpoint、endpoint_method、endpoint_unit string；planned_run_count int；planned_run_relationship string。
- outputs: design_fields object view。

### group@1
- inputs: kind/id/schema string|int required；name string required；role、condition string；independent_n、technical_repeats int。
- outputs: group_spec object view。

### assay@1
- inputs: kind/id/schema/title string|int required；endpoint、method、unit string；group_refs list(ref→group)；timepoints string[]；protocol_version_id string；controls string[]；timepoint_relation string；technical_replicates int；control_group_ref reference→group；calibrator_group_ref reference→group；reference_genes string[]。
- outputs: assay_plan object view。

### step@1
- inputs: kind/id/schema/title string|int required；assay reference→assay；timepoints string[]；path string；order int；checks object-list；tasks object-list。
- outputs: step_task object view。

### workflow@1
- inputs: kind/id/schema string|int required；steps list(ref→step) required；edges object-list；business_order string[]。
- outputs: workflow_graph object view。

### dilution@1
- inputs: kind/id/schema string|int required；stock quantity (dimension mass_concentration) required；target quantity (mass_concentration) required；final_volume quantity (volume) required。
- outputs: stock_volume quantity (volume) derived calculator dilution.v1；diluent_volume quantity (volume) derived calculator dilution.v1。

### instrument_profile@1
- inputs: kind/id/schema string|int required；class string required；model string；capabilities list(object) required；constraints list(object)；source enum{built_in,lab_imported,unknown} required；evidence list(artifact)。
- outputs: profile_view object view。

### step@2
- 在 step@1 基础上新增 inputs.uses list(object)（每项 {ref, role}）；rules 增加 step-uses。step@1 语义不变。

### step@3
- 在 `step@2` 基础上新增必填 `execution` enum{executable,organizational}。可执行步骤需要 assay 和唯一的 `reference.view: protocol_block`；Core 再确认精确 BlockVersion 属于该 assay 的冻结 ProtocolVersion。
- `organizational` 不得声明 assay、timepoints、path、checks、tasks 或 uses，不生成 Run Task。`step@1`/`step@2` 始终按可执行步骤解释；缺少 Block 不推断为组织节点。

### material@1
- inputs: kind/id/schema string|int required；name string required；role enum{control,treatment} required；condition string；concentration quantity (mass_concentration)；source enum{built_in,lab_imported,unknown} required。
- outputs: material_view object view。

### sample@1
- inputs: kind/id/schema string|int required；sample_id string required；group_ref reference→group required；material_refs list(ref→material)；amount quantity (volume) required。
- outputs: sample_view object view。

### container@1
- inputs: kind/id/schema string|int required；format string required；wells int required；geometry geometry；sample_map well-map。
- outputs: container_view object view。

### measurement_profile@1
- inputs: kind/id/schema string|int required；endpoint string required；unit string required；readout_schema object；calculator_refs list(string)。
- outputs: profile_view object view。

### rna_reagent_usage@1
- inputs: kind/id/schema string|int required；sample_count int required；volume_per_sample quantity (volume) required；stock_concentration quantity (mass_concentration) required；target_concentration quantity (mass_concentration) required；extra_ratio decimal optional。
- outputs: stock_volume quantity (volume) derived calculator rna.reagent_usage.v1；diluent_volume quantity (volume) derived calculator rna.reagent_usage.v1；total_volume quantity (volume) derived calculator rna.reagent_usage.v1。

### readout@1
- inputs: kind/id/schema string|int required；schema_id string required（`cck8_od450`、`rna_concentration`、`rna_purity`、`cfu_count` 等）；endpoint string required；value_kind enum{numeric,table,plate,categorical,file} required；unit string；missing_reasons list(string)；null_allowed bool；source_kinds list(string)；table_columns object-list；plate_format enum{6,12,24,48,96,384,custom}。
- outputs: readout_shape object view；record_form enum{plate,single,table,categorical,file} view。
- 记录路径：`crates/asterlab-core::readout_records`（`ReadoutRegistry`、`plan_readout_submission`、`query_readout_records`）；三种记录形态共用一次 schema 校验与一个 `execution_event` 写入，不新增记录表。

## 3. 模块如何被设计：12 维度与落点

`docs/sdk/module-contract-v1.md` §1 列出 12 维度；本节只说明每维在何处落地：

| 维度 | 落点 |
|---|---|
| 1 Identity | `modules.json` 字段 `kind/schema/title/sdk`；Go `Descriptor`（`registry.go`）；Rust `asterdoc_module_spec`（`src/cli.rs`） |
| 2 Inputs | `modules.json` `inputs[]`；Go `Port`/`Descriptor.Inputs`；Rust `spec["inputs"]` |
| 3 Outputs | `modules.json` `outputs[]`（含 `origin`、`calculator`）；Go `Port.Origin/Calculator` |
| 4 References | `inputs[].type=reference` + `target`；Go `Port.Target`；Rust 引用校验 `validate_asterdoc_references`（`src/cli.rs`） |
| 5 Validation | `rules[]` + Go `ModuleSpec.Validate`（`specValidator`，`spec.go`）+ Rust `validate_asterdoc_module`（`src/cli.rs`） |
| 6 Actions | `modules.json` `actions[]`（含 `permission/writes`） |
| 7 Permissions | `modules.json` `permissions[]` + `actions[].permission`；Go `Perm*` 常量与 `View.Can` |
| 8 Bindings | renderer/runtime 集成（下文 §5） |
| 9 Renderers | `modules.json` `renderers.design/document/run/static`；Go `Renderers`；回调注册于 `Registry` |
| 10 Lifecycle | `modules.json` `lifecycle.states/freeze` |
| 11 Provenance | `modules.json` `provenance.origin/record_boundary`；Go `Provenance` |
| 12 Diagnostics | 共享 shape（Foundation v1），CLI 错误信封暴露稳定顶层 code |

关键约束：
- derived 输出缺少 `calculator` 被 manifest 校验器拒绝（`validateSpecModule` in `spec.go`：`port.Origin == "derived" && port.Calculator == ""`）；Rust 端 `validate_asterdoc_spec` 同样校验。
- `record_boundary` 固定为 `run`；document 模块不得写 Run 事实。
- 量纲/UCUM：`prototypes/mygo-run/internal/asterdoc/spec/units.json` 仅 36 个符号、9 个维度（mass_concentration、mass、amount_concentration、length、time、volume、amount、dimensionless、temperature）；`units.go` 注释明确是"命名 UCUM 子集"，转换显式且不重写原值。
- 引用：`reference.ref` 是非类型化的 `string`（`reference@1` 的 `ref` 字段为 `string`，仅 `target` 在引用端口声明）；typed reference 通过 `inputs[].type=reference + target` 实现，`ref` 本身不带类型。
- Go 注册：`BuiltinRegistry()` 注册 16 个 `kind@schema`（14 个 kind）；`Register` 验证所声明的 renderer key 有已注册 callback。`readout@1` 由 manifest 声明、Rust Core 消费，Go 侧尚未注册（见 §1 说明）。
- Rust 镜像：`src/cli.rs` 嵌入同一 `modules.json` 并校验；`asterdoc_module_spec(kind, schema)` 查找。

## 4. 新增模块：精确文件清单

1. `prototypes/mygo-run/internal/asterdoc/spec/modules.json`：在 `modules[]` 追加模块对象（含 inputs/outputs/actions/rules/sdk/category/capabilities/permissions/renderers/lifecycle/provenance）。
2. Go：在 `modules.go` 增加 `xxxSpec() ModuleSpec`（Descriptor 用 `builtinDescriptor`/`descriptorFor`，Validate 用 `specValidator`，Render 用 UI 代码），并在 `registry.go` 的 `BuiltinRegistry()` 中 `Register`。
3. Rust：若新增 rule 名，必须在 `src/cli.rs` 的 `validate_asterdoc_module` 的 `match rule` 分支中增加对应 arm；未注册的 rule 名落入 `other => Err(internal(...))`（见 §4.1）。
4. `prototypes/mygo-run/asterdoc_store_test.go`：在 `TestAsterDocGoRustParity` 的 `samples` 中追加 parity 样本（valid/invalid 各至少一个），保持 Go/Rust 接受拒绝一致。
5. 测试夹具：`prototypes/mygo-run/internal/asterdoc/testdata/`（若存在）或 `readTestDoc` 引用的 fixture。
6. 文档：更新 `docs/sdk/module-contract-v1.md`、`module-development-guide.md`、本文件。

### 4.1 Rule-name parity 危险

Rust 侧 `validate_asterdoc_module` 的 rule 匹配是穷举的：已知 rule 有显式 arm，`other => return Err(CliError::internal("unknown asterlab module rule {other}"))`。因此在 `modules.json` 中声明一个 Go 侧已实现但 Rust 侧 switch 未注册的 rule 名，Rust 校验器会返回 `internal` 错误，导致 Go/Rust parity 失败（`TestAsterDocGoRustParity` 中 `(goErr==nil) != (coreErr==nil)`）。新增 rule 必须同步两端。

## 5. 消费 SDK 模块的界面

- Design 文档编辑：`asterdoc.View`，`Mode=ModeDesign`，`Allowed={PermDocumentEdit:true}`，`OnAssignWells/OnClearWells/OnUpdateModule` 为 Core 命令 seam（`prototypes/mygo-run/main.go` ~1220-1245）。权限 seam 为 `View.Can(permission)`（`view.go`），`Mode`/`ReadOnly` 从不单独决定写权限。
- 冻结 Run 只读：`renderFrozenAsterDoc` 创建 `&asterdoc.View{..., Mode: mode, ReadOnly: true}`（`main.go` ~2026）；`runworkspace` 任务面板中 `frozen := &asterdoc.View{Mode: ModeRun, ReadOnly: true}`（`prototypes/mygo-run/internal/runworkspace/view.go` ~190 附近，`main.go` ~2026）。
- Notebook AsterML 模式：`a.noteAster = &asterdoc.View{..., Mode: ModeNotebook, Allowed:{PermDocumentEdit:true}, OnChange:...}`（`main.go` ~1512）。
- 静态预览/PDF：`StaticPreview(m)` / `StaticDocumentPreview(doc)`（`static.go`）；`pdf.go` ~100 调用 `StaticDocumentPreview(doc)` 作为 PDF 导出的只读数据源。`StaticPreview` 从 `BuiltinRegistry()` 查找 spec，未注册 kind 则退化为按键名打印。

## 5.1 `plate@1` 单一 AsterDoc kind 与迁移

- AsterDoc 侧只有一种孔板模块 kind：`plate@1`，通过 `format`（6/12/24/48/96/384）表达板型。不存在重复的 `plate96` AsterDoc module kind。
- 旧 `kind: plate96` 源必须显式迁移为 `kind: plate` + `format: 96`；不可变历史修订不被重写，不保留兼容别名。
- `cck8.plate96.od450.v1` 是 CCK-8 结果 schema ID，`blocks.TypePlate96` 是 legacy block 路径类型；两者是独立契约，不是重复的 AsterDoc module kind，不得重命名或删除。

## 6. 限制（如实陈述）

- 无动态/第三方加载：模块是 Go 代码（`*Spec()` + `Register`），不是插件 ABI；contract 语言中立，但 v1 不是可插拔扩展。
- 无设备控制/SiLA：`instrument_profile` 仅能力声明，不连接硬件、不校准、不记录执行结果。
- 量纲仅命名 UCUM 子集（36 符号/9 维度），非完整 UCUM。
- 无代码生成：Go 与 Rust 校验器并行维护；`TestAsterDocGoRustParity` 覆盖 plate、引用、稀释、仪器档案和字段校验样本，但不逐一覆盖每个模块的每个字段。
- 无 manifest/schema 校验或代码生成 CLI；目前的工具 `sdk.modules`、`sdk.describe` 从嵌入的 manifest 发现/描述模块，`asterdoc.validate` 校验候选 AsterDoc 并尝试编译。Go 与 Rust 校验器仍并行维护。
- 无模板市场：模板 shape 在 Foundation schema 中定义；实例化已在 `internal/asterdoc/template.go` 实现（`InstantiateTemplate` / `TemplateManifest`），但无模板市场、无远程下载、无模板管理 CLI。
- 通用编辑器对 `list/object/table/object-list/well-map/geometry` 字段显示为 read-only（`GenericModuleEditor`，`view.go`）；因此复合字段目前无法在通用编辑器内组装，需要专用编辑器或后续的结构化控件。
- 无模块生命周期（register/activate/dispose）：模块是纯函数，加载即全部注册，失败即暴露 `BuiltinError`。
- `step@1`/`step@2` 语义不变；`step@2` 新增 `uses`；`step@3` 显式声明执行/组织语义并要求可执行步骤有精确 ProtocolBlock 来源。
- `reference.ref` 是非类型化字符串，typed reference 通过端口 `type=reference + target` 实现。
- `renderers.static` 字段未在 manifest 中声明；静态渲染走 `StaticPreview` 路径。
- 模块 specific migration 版本化；Foundation v1 不静默迁移旧字段。
- 全诊断数组为已声明 gap，CLI 错误信封仅暴露稳定顶层 code + 可读消息。
- 模板实例化仅提供 Go API（`InstantiateTemplate`），宿主通过 `CreateAsterDocDesign`/`SaveAsterDoc` 落地；无 CLI 模板命令。
