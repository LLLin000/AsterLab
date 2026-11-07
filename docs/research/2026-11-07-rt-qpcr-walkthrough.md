# RT-qPCR design/execution walkthrough

- Date: 2026-11-07
- Purpose: test the new four-tab shell with a complex non-CCK-8 experiment
- Data: synthetic two-step RT-qPCR; not a real experiment

## Scenario

- 5 steps: RNA extraction/QC → RT → qPCR setup → cycling → Cq analysis.
- Groups: Control / Treatment; timepoints 0 h / 24 h; NTC and NRT checks.
- Parameters: RNA input, RT volume, cDNA input, cycles, annealing temperature.
- View: operation nodes plus a qPCR 96-well plate component on the setup step.
- Run: 96-well plate A1/A2/B1/B2 plus an extraction check event and raw Cq/not-detected readouts.

## What is visible

| Page | Visible | Assessment |
|---|---|---|
| 实验设计 (Workflow) | 5 operation nodes; step detail; brief; checks; qPCR plate on step 3 | Order correct; plate component is present |
| Protocol | protocol/version selector; 5-step strip; instruction; parameters; checks | Selection works; materials/instruments still say 未列出 because binding is absent |
| 实验运行 | Assay qpcr; step 1/5; instruction; RNA input; check; record form; event; plate | Execution identity now correct; plate appears, event order correct |
| 资料与工具 | common instruments, tools, materials catalog; project/records/backup | Catalog exists; not yet bound to a Protocol step |

## Problems observed

1. Protocol “材料与试剂 / 仪器与设备” remain 未列出 even though the common resource catalog exists.
   The catalog is available, but no typed binding from Protocol step to instrument/material.
2. Run events still expose raw values (`operation`, `passed`, check IDs) in places; technical fields should be collapsed/localized.
3. The qPCR plate is only visible after selecting the setup step; that is semantically correct, but the step should show a small “plate attached” indicator in the overview.
4. Protocol identity selector is usable, but the page still has duplicated small headings and a secondary edit link.
5. Design and Protocol pages are read-only; edit still jumps to the old workspace.

## Next fixes

- Add typed `instrument_ref` / `material_ref` / `recipe_ref` to Protocol step detail and render “required / available / unresolved”.
- Add a plate/container badge to steps with a `lab.well_plate` child.
- Localize event/checks status labels and keep IDs in `<details>`.
- Remove old-workspace primary edit paths after inline editors land.
