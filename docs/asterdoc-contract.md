# AsterDoc / AsterML v2 Module Contract

Status: AsterML v2 (`sdkVersion: "2.0.0"`), self-contained composable step modules with unified Design → Run → Notebook rendering.

## 0. Hard Architectural Rules

1. **No assay-specific modules, fields, or dedicated UI panels**: If a requirement can be expressed by composing the 10 generic Layer-1 content modules (`prose`, `group_table`, `param_table`, `figure`, `data_table`, `checklist`, `calculator`, `timer`, `attachment`, `subflow`) + standard container (`plate@1`) + Layer-2 scientific primitives (`experiment`, `group`, `assay`, `material`, `sample`, `readout`, `reference`, `instrument_profile`), adding an experiment-specific kind/field/UI panel is forbidden.
2. **Figures (`figure@1`) are supplementary visual expression only**: Diagrams, hand-drawn Petri dish quadrants, and agent-generated SVGs live in `figure@1` for human comprehension. Structured experimental groups, parameters, and measurements belong exclusively in structured modules (`group_table`, `param_table`, `plate`, `data_table`, `readout`) and Run records — never parsed out of SVG/image text.
3. **Self-contained step composition**: `step@1` owns an ordered `blocks: [...]` array of Layer-1 content modules plus optional `block_refs: [...]` to shared top-level modules (`plate`, `group_table`, `calculator`). A self-contained AsterML v2 document compiles and freezes directly into a Run via `CreateRunFromAsterDoc` without pre-registering `ProtocolBlockVersion` UUIDs.

## 1. Source syntax

An AsterDoc is ordinary Markdown. A fenced block whose language is `asterlab` declares one module:

````markdown
## 实验步骤 02：处理与分组

```asterlab
kind: step
id: step-02-treatment
schema: 1
title: 02 实验处理与分组
execution: executable
blocks:
  - kind: param_table
    id: pt-02
    schema: 1
    title: 实验参数
    rows:
      - {key: od600, label: OD600, value: 0.5, unit: AU}
  - kind: figure
    id: fig-02
    schema: 1
    title: 培养皿四象限分区示意图
    svg: "<svg viewBox='0 0 200 200'>...</svg>"
    caption: 示意：四象限分区（仅视觉辅助，分组真值在 group_table）
  - kind: checklist
    id: chk-02
    schema: 1
    items:
      - {id: prep, label: 准备实验材料, blocking: true}
      - {id: assign, label: 按四象限完成分组加样, blocking: true}
```
````

Rules:

- `kind` and `schema` select a registered module specification from `prototypes/mygo-run/internal/asterdoc/spec/modules.json`.
- `id` is stable and unique across all top-level blocks and inline `step.blocks[]` items in one document.
- Unknown `kind`, unknown `schema`, or unknown UCUM unit in `spec/units.json` is rejected by both Go and Rust Core (`TestAsterDocGoRustParity`).
- Plain Markdown outside fences is preserved verbatim on semantic patches.

## 2. Three-Layer Module Architecture

- **Layer 1 · Generic Content Modules (10 composable blocks + standard `plate`)**:
  `prose@1`, `group_table@1`, `param_table@1`, `figure@1`, `data_table@1`, `checklist@1`, `calculator@1`, `timer@1`, `attachment@1`, `subflow@1`, and `plate@1` (`plate` / `rack_1d` / `dish` / `cohort`, stripped of all assay-specific `mix_*` fields; supports universal orthogonal factor annotations via `factors: { dimension -> { well -> value } }` and factor-scoped technical replicate uniqueness `(sample_id, canonical_factor_signature, replicate)` so multi-target qPCR / dose-response plates share sample IDs across targets while still rejecting true collisions).
- **Layer 2 · Scientific Data & Logic**:
  `experiment@1`, `group@1` (and `group_table@1`), `assay@1`, `material@1` (with optional `pantry_code`), `sample@1`, `readout@1`, `reference@1`, `instrument_profile@1`.
- **Layer 3 · Plan, Run & Record (Single Source of Truth)**:
  - **Design**: Edit any step's `blocks[]` freely (insert, update, reorder, remove via `asterdoc.command`: `add_step`, `insert_step_block`, `update_step_block`, `move_step_block`, `remove_step_block`), edit global entities, or edit raw `.asterml` source.
  - **Run**: `CreateRunFromAsterDoc` atomically freezes the `asterdoc_version`, compiles `DesignVersion` + auto-materialized `ProtocolBlockVersion`/`ProtocolVersion` if not already bound, and renders each frozen step's `blocks[]` in `ModeRun` (live timer controls, interactive checklist & `skipped_by_rule`, actual parameter recording, interactive `data_table` with CSV paste / formula columns / range rules -> `readout.batch` / `readout.submit`, and photo/file `attachment` slots -> `asset.link`).
  - **Notebook / Record**: `RecordProjection` aggregates frozen plan + recorded Run facts/assets into unified views and exports; `DeriveDesignFromRun` produces a new AsterDoc revision (`asterdoc_source`) saved via `SaveAsterDoc`.

## 3. Shared Registry & Parity

`prototypes/mygo-run/internal/asterdoc/spec/modules.json` (21 `kind@schema` across 21 `kind`s, `sdkVersion: "2.0.0"`) and `spec/units.json` are embedded at compile time by both Go (`internal/asterdoc`) and Rust (`src/cli/asterdoc.rs`). `TestAsterDocGoRustParity` pins identical validation outcomes across both languages.
