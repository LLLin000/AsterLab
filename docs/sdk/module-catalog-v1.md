# AsterDoc SDK 模块目录 v1

> 权威来源：`prototypes/mygo-run/internal/asterdoc/spec/modules.json`（foundationVersion 1，sdkVersion 1.0.0，15 个已注册模块）。本文只描述已实现的事实，不描述未实现能力。

## 1. 已注册模块总表

| kind@schema | category | capabilities | permissions | renderers design/document/static | rules |
|---|---|---|---|---|---|
| plate96@1 | labware | well-plate, well-assignment, plate-geometry | document.edit | plate96.editor / plate96.viewer / — | plate-wells |
| plate@1 | labware | well-plate, well-assignment, plate-geometry | document.edit | plate.editor / plate.viewer / — | plate-geometry-wells |
| reference@1 | document | typed-reference | (none) | reference.editor / reference.viewer / — | reference-ref |
| experiment@1 | workflow | experiment-plan | (none) | experiment.editor / experiment.viewer / — | (none) |
| group@1 | workflow | group-definition | (none) | group.editor / group.viewer / — | group-fields |
| assay@1 | measurement | assay-plan, measurement-endpoint | (none) | assay.editor / assay.viewer / — | assay-fields |
| step@1 | workflow | step-definition, workflow-edge | (none) | step.editor / step.viewer / — | step-fields |
| workflow@1 | workflow | workflow-graph | (none) | workflow.editor / workflow.viewer / — | workflow-fields |
| dilution@1 | measurement | quantity, deterministic-calculation, derived-output | (none) | dilution.editor / dilution.viewer / — | dilution-fields |
| instrument_profile@1 | resource | instrument-profile, capability-declaration | (none) | instrument-profile.editor / instrument-profile.viewer / — | instrument-profile-fields |
| step@2 | workflow | step-definition, workflow-edge | (none) | step.editor / step.viewer / — | step-fields, step-uses |
| material@1 | resource | material, role-declaration | (none) | material.editor / material.viewer / — | material-fields |
| sample@1 | resource | sample, sample-tracking | (none) | sample.editor / sample.viewer / — | sample-fields |
| container@1 | labware | container, well-map | (none) | container.editor / container.viewer / — | container-fields |
| measurement_profile@1 | measurement | measurement-profile, calculator-reference | (none) | measurement_profile.editor / measurement_profile.viewer / — | measurement-profile-fields |

说明：`renderers.static` 在当前 manifest 中未声明（均为 `—`）；静态预览通过 `StaticPreview`/`StaticDocumentPreview`（`prototypes/mygo-run/internal/asterdoc/static.go`）按 registry 路径读取，不依赖该字段。

`renderers` 现在是**被执行**的契约：`Register` 校验每个声明的 key 都有实现（`ModuleSpec.Renders`），跨 kind 复用同一 key 被拒绝；宿主按 `Descriptor.RendererKey(mode)` 分派，声明但未实现时渲染显式的能力缺口提示（`View.rendererGap`），不回退猜测。`plate`/`plate96`/`container` 保留专用编辑器；字段型模块通过 `generic-editor` capability 显式选择通用编辑器。

## 2. 端口（inputs / outputs）明细

### plate96@1
- inputs: kind string required；id string required；schema int required；assay string；groups well-map required。
- outputs: wells well-map view；assigned int view。

### plate@1
- inputs: kind string required；id string required；schema int required；format int required（6/12/24/48/96/384）；assay reference→assay；timepoint timepoint；groups well-map required。
- outputs: rows int view；columns int view；wells well-map view。

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
| 9 Renderers | `modules.json` `renderers.design/document/static`；Go `Renderers`；渲染代码在 `modules.go` |
| 10 Lifecycle | `modules.json` `lifecycle.states/freeze` |
| 11 Provenance | `modules.json` `provenance.origin/record_boundary`；Go `Provenance` |
| 12 Diagnostics | 共享 shape（Foundation v1），CLI 错误信封暴露稳定顶层 code |

关键约束：
- derived 输出缺少 `calculator` 被 manifest 校验器拒绝（`validateSpecModule` in `spec.go`：`port.Origin == "derived" && port.Calculator == ""`）；Rust 端 `validate_asterdoc_spec` 同样校验。
- `record_boundary` 固定为 `run`；document 模块不得写 Run 事实。
- 量纲/UCUM：`prototypes/mygo-run/internal/asterdoc/spec/units.json` 仅 36 个符号、9 个维度（mass_concentration、mass、amount_concentration、length、time、volume、amount、dimensionless、temperature）；`units.go` 注释明确是"命名 UCUM 子集"，转换显式且不重写原值。
- 引用：`reference.ref` 是非类型化的 `string`（`reference@1` 的 `ref` 字段为 `string`，仅 `target` 在引用端口声明）；typed reference 通过 `inputs[].type=reference + target` 实现，`ref` 本身不带类型。
- Go 注册：`BuiltinRegistry()`（`registry.go`）`sync.Once` 注册 15 个 `*Spec()`；`Register` 拒绝重复 kind@schema、缺 Validate/Render。
- Rust 镜像：`src/cli.rs` 嵌入同一 `modules.json`（`ASTERDOC_MODULE_SPEC`），`asterdoc_spec()` OnceLock 解析并 `validate_asterdoc_spec` 校验；`asterdoc_module_spec(kind, schema)` 查找。

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

## 6. 限制（如实陈述）

- 无动态/第三方加载：模块是 Go 代码（`*Spec()` + `Register`），不是插件 ABI；contract 语言中立，但 v1 不是可插拔扩展。
- 无设备控制/SiLA：`instrument_profile` 仅能力声明，不连接硬件、不校准、不记录执行结果。
- 量纲仅命名 UCUM 子集（36 符号/9 维度），非完整 UCUM。
- 无代码生成：Go 与 Rust 校验器并行维护，漂移仅由 `TestAsterDocGoRustParity` 采样覆盖（`TestAsterDocGoRustParity` 现含 23 个 valid/invalid 样本，覆盖 plate/plate96/reference/dilution/instrument_profile/step@2/material/sample/container/measurement_profile，未逐一覆盖 experiment/group/assay/workflow 每字段）。
- 无 `sdk new/validate/manifest` CLI：CLI 仅 `validate_asterdoc_source`（`src/cli.rs`）供 AsterML 编译使用。
- 无模板市场：模板 shape 在 Foundation schema 中定义；实例化已在 `internal/asterdoc/template.go` 实现（`InstantiateTemplate` / `TemplateManifest`），但无模板市场、无远程下载、无模板管理 CLI。
- 通用编辑器对 `list/object/table/object-list/well-map/geometry` 字段显示为 read-only（`GenericModuleEditor`，`view.go`）；因此复合字段目前无法在通用编辑器内组装，需要专用编辑器或后续的结构化控件。
- 无模块生命周期（register/activate/dispose）：模块是纯函数，加载即全部注册，失败即暴露 `BuiltinError`。
- `step@1` 语义不变；`step@2` 仅新增 `uses` 绑定。
- `reference.ref` 是非类型化字符串，typed reference 通过端口 `type=reference + target` 实现。
- `renderers.static` 字段未在 manifest 中声明；静态渲染走 `StaticPreview` 路径。
- 模块 specific migration 版本化；Foundation v1 不静默迁移旧字段。
- 全诊断数组为已声明 gap，CLI 错误信封仅暴露稳定顶层 code + 可读消息。
- 模板实例化仅提供 Go API（`InstantiateTemplate`），宿主通过 `CreateAsterDocDesign`/`SaveAsterDoc` 落地；无 CLI 模板命令。
