# AsterDoc SDK 模块目录 v2

> 权威来源：`prototypes/mygo-run/internal/asterdoc/spec/modules.json`（foundationVersion 1，sdkVersion 2.0.0，21 个 `kind@schema`，对应 21 个 `kind`）。本文只描述 AsterML v2 通用模块契约。

## 1. 已注册模块总表

下表按 `kind@schema` 列出 21 条（21 个 `kind` 均只有 `schema: 1`，无版本跳跃）：

| kind@schema | category | capabilities | permissions | Design / Document / Run / Static | rules |
|---|---|---|---|---|---|
| plate@1 | labware | well-plate, well-assignment, plate-geometry | document.edit | plate.editor / plate.viewer / plate.run / — | plate-geometry-wells |
| prose@1 | document | rich-text, callout | document.edit | prose.editor / prose.viewer / prose.run / — | prose-fields |
| group_table@1 | workflow | group-definition | document.edit | group_table.editor / group_table.viewer / group_table.run / — | group_table-fields |
| param_table@1 | measurement | parameter-table, quantity | document.edit | param_table.editor / param_table.viewer / param_table.run / — | param_table-fields |
| figure@1 | document | figure, svg-illustration | document.edit | figure.editor / figure.viewer / figure.run / — | figure-fields |
| data_table@1 | measurement | data-table, readout-schema | document.edit, run.readout | data_table.editor / data_table.viewer / data_table.run / — | data_table-fields |
| checklist@1 | workflow | checklist, rule-skip | document.edit | checklist.editor / checklist.viewer / checklist.run / — | checklist-fields |
| calculator@1 | measurement | quantity, deterministic-calculation, derived-output | document.edit | calculator.editor / calculator.viewer / calculator.run / — | calculator-fields |
| timer@1 | workflow | timer, time-schedule | document.edit | timer.editor / timer.viewer / timer.run / — | timer-fields |
| attachment@1 | document | attachment, asset-link | document.edit | attachment.editor / attachment.viewer / attachment.run / — | attachment-fields |
| subflow@1 | workflow | subflow, step-definition | document.edit | subflow.editor / subflow.viewer / subflow.run / — | subflow-fields |
| reference@1 | document | typed-reference | (none) | generic.editor / reference.viewer / generic.run / — | reference-ref |
| experiment@1 | workflow | experiment-plan | (none) | generic.editor / experiment.viewer / generic.run / — | (none) |
| group@1 | workflow | group-definition | (none) | generic.editor / group.viewer / generic.run / — | group-fields |
| assay@1 | measurement | assay-plan, measurement-endpoint | (none) | generic.editor / assay.viewer / generic.run / — | assay-fields |
| step@1 | workflow | step-definition, workflow-edge | document.edit | generic.editor / step.viewer / generic.run / — | step-execution, step-blocks |
| workflow@1 | workflow | workflow-graph | (none) | — / workflow.viewer / generic.run / — | workflow-fields |
| instrument_profile@1 | resource | instrument-profile, capability-declaration | (none) | generic.editor / instrument-profile.viewer / generic.run / — | instrument-profile-fields |
| material@1 | resource | material, role-declaration | (none) | generic.editor / material.viewer / generic.run / — | material-fields |
| sample@1 | resource | sample, sample-tracking | (none) | generic.editor / sample.viewer / generic.run / — | sample-fields |
| readout@1 | measurement | readout-schema, value-kind, run-record | run.readout | generic.editor / generic.editor / generic.run / — | (none) |
