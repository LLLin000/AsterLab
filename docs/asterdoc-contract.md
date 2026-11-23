# AsterDoc / AsterML v2 Module Contract

Status: AsterML v2 (`sdkVersion: "2.0.0"`), self-contained composable step modules with unified Design → Run → Notebook rendering.

## 0. Hard Architectural Rules

1. **No assay-specific modules, fields, or dedicated UI panels**: If a requirement can be expressed by composing the 10 generic Layer-1 content modules (`prose`, `group_table`, `param_table`, `figure`, `data_table`, `checklist`, `calculator`, `timer`, `attachment`, `subflow`) + standard container (`plate@1`) + Layer-2 scientific primitives (`experiment`, `group`, `assay`, `material`, `sample`, `readout`, `reference`, `instrument_profile`), adding an experiment-specific kind/field/UI panel is forbidden.
2. **Figures (`figure@1`) are supplementary visual expression only**: Diagrams, hand-drawn Petri dish quadrants, and agent-generated SVGs live in `figure@1` for human comprehension. Structured experimental groups, parameters, and measurements belong exclusively in structured modules (`group_table`, `param_table`, `plate`, `data_table`, `readout`) and Run records — never parsed out of SVG/image text.
3. **Self-contained step composition**: `step@1` owns an ordered `blocks: [...]` array of Layer-1 content modules plus optional `block_refs: [...]` to shared top-level modules (`plate`, `group_table`, `calculator`). A self-contained AsterML v2 document compiles and freezes directly into a Run via `CreateRunFromAsterDoc` without pre-registering `ProtocolBlockVersion` UUIDs.

## 1. Source syntax (AsterML v2.1 Markdown-First)

An AsterDoc is ordinary Markdown with **YAML Frontmatter**, **orthogonal step headings (`#@` / `##@`)**, and **fenced `asterlab` interactive cards** only where structured calculation, layout, timing, or tabular readout is needed:

````markdown
---
title: 实验标题
scheduled_date: 2026-11-24
stage: planned
---

#@ 1. 实验处理与分组 {#t0}

- [ ] 准备实验材料，将菌液 OD600 校正至 0.5 AU
- [ ] 按四象限完成分组加样后 37 °C 倒置培养 16 h

```asterlab
kind: param_table
id: pt-02
schema: 1
title: 实验条件
rows:
  - {key: od600, label: OD600, value: 0.5, unit: AU}
```

```asterlab
kind: timer
id: timer-02
schema: 1
title: 37 °C 倒置培养
mode: countdown
status: idle
duration: {value: 16, unit: h}
```
````

Rules:

- **Canonical step authoring**: New documents, built-in SOP templates, `/step` commands, and AI Agents MUST author steps using orthogonal Markdown headings (`#@ 主步骤` / `##@ 子步骤`), plain Markdown prose, and `- [ ]` checklists. This unlocks unified outline/progress tracking, SciGraph context, desktop `Alt+Hover`, and CLI `asterlab asterdoc.suggest` compilation.
- **Legacy container compatibility**: Fenced `step@1`, `workflow@1`, `prose@1`, and `subflow@1` blocks remain registered in `spec/modules.json` strictly for backward-compatible parsing and frozen Run replay; they are NOT exposed in foreground templates or slash menus.
- `kind` and `schema` select a registered module specification from `prototypes/mygo-run/internal/asterdoc/spec/modules.json`.
- `id` is stable and unique across all top-level blocks and inline `step.blocks[]` items in one document.
- Unknown `kind`, unknown `schema`, or unknown UCUM unit in `spec/units.json` is rejected by both Go and Rust Core (`TestAsterDocGoRustParity`).
- Plain Markdown outside fences is preserved verbatim on semantic patches.

## 2. Three-Layer Module Architecture

- **Layer 1 · Generic Content Modules (10 composable blocks + standard `plate`)**:
  `prose@1`, `group_table@1`, `param_table@1` (default UI heading `实验条件`), `figure@1` (resolves relative paths against `View.BaseDir` and stores user-picked/dropped assets cleanly into `./assets/<file>` for folder portability), `data_table@1` (supports `layout: rows | matrix` aligned to `container_ref` wells with whole-plate TSV/CSV matrix paste), `checklist@1`, `calculator@1` (`group_mastermix` accepts `container_ref` or `plate_ref`; Design mode persists edited formula/normalization/dilution parameters back to `.asterml`), `timer@1` (`window` mode resolves both `offset` and `duration`), `attachment@1` (displays clickable `container_grid` well-slot progress chips), `subflow@1`, and `plate@1` (`4`/`6`/`12`/`24`/`48`/`96`/`384` standard grids plus `rack_1d` vertical gel lanes / PCR strips, circular `dish` quadrants, and `cohort`; supports single-input unified annotation with serial gradient shorthand `浓度:100/2 uM`, order-independent orthogonal `factors`, and deterministic factor-scoped technical replicate numbering across `(sample_id, canonical_factor_signature, replicate)`).
- **Layer 2 · Scientific Data & Logic**:
  `experiment@1`, `group@1` (and `group_table@1`), `assay@1`, `material@1` (with optional `pantry_code`), `sample@1`, `readout@1` (dedicated `readout.viewer` document card), `reference@1`, `instrument_profile@1`.
- **Layer 3 · Plan, Run & Record (Single Source of Truth)**:
  - **Design**: Edit any step's `blocks[]` freely (insert, update, reorder, remove via `asterdoc.command`: `add_step`, `insert_step_block`, `update_step_block`, `move_step_block`, `remove_step_block`), edit global entities, or edit raw `.asterml` source.
  - **Run**: `CreateRunFromAsterDoc` atomically freezes the `asterdoc_version`, compiles `DesignVersion` + auto-materialized `ProtocolBlockVersion`/`ProtocolVersion` if not already bound, and renders each frozen step's `blocks[]` in `ModeRun` (live timer controls, interactive checklist & `skipped_by_rule`, actual parameter recording, interactive `data_table` with CSV paste / formula columns / range rules -> `readout.batch` / `readout.submit`, and photo/file `attachment` slots -> `asset.link`).
  - **Notebook / Record**: `RecordProjection` aggregates frozen plan + recorded Run facts/assets into unified views and exports; `DeriveDesignFromRun` produces a new AsterDoc revision (`asterdoc_source`) saved via `SaveAsterDoc`.

## 3. Shared Registry & Parity

`prototypes/mygo-run/internal/asterdoc/spec/modules.json` (21 `kind@schema` across 21 `kind`s, `sdkVersion: "2.0.0"`) and `spec/units.json` are embedded at compile time by both Go (`internal/asterdoc`) and Rust (`src/cli/asterdoc.rs`). `TestAsterDocGoRustParity` pins identical validation outcomes across both languages.

## 4. AsterML Single-Source Document & Envelope Compatibility (`envelope.go`, `doc_ops.go`)

An `.asterml` file inside the Obsidian-style Vault is a **single unified source of truth** where experimental design and inline execution progress coexist seamlessly inside one human-readable file:

1. **YAML Frontmatter (`--- ... ---` at byte 0)**:
   - Parsed by `ScanFrontmatter` (`asterdoc/envelope.go`) and updated via `SetFrontmatterMetadata`.
   - Declares `title`, `date` / `scheduled_date` (`YYYY-MM-DD`, empty for unscheduled experiments or templates), and `status` / `stage` (`planned | in_progress | completed`).
2. **Unified Inline Execution Fields (Single Source of Truth)**:
   - `checklist@1.items[]`: optional `done: bool` and `note: string`.
   - `param_table@1.rows[]`: optional `actual: number|string` and `note: string` alongside `value`.
   - `timer@1`: optional `status` (`idle|running|paused|completed`), `started_at`, `completed_at`, `elapsed_sec`, `phase_index`, `round_index`.
   - `plate@1`: optional `layers: map[layerName]map[well]value` and `excluded_wells: []well`.
   - `calculator@1`: supports both legacy 5 modes and unified `rows: [{key, label, value, formula, unit, note}]` with `@plate.wells` / `@param.<key>` references.
   - `data_table@1`: supports `source_ref` (`@groups` or `@plate.<id>`) alongside inline `rows` and `matrix` layout.
   - `experiment@1`: optional `status`, `conclusion`, `outcome`.
3. **Legacy 3-Part Envelope Auto-Flattening**:
   - Older files containing `asterlab-exec` / `asterlab-record` envelopes are transparently normalized on load by `FlattenEnvelopeToSingleSource` into the unified inline representation without losing recorded data.
4. **Atomic Writes, Templates & Archive Interchange**:
   - `WriteFileAtomic` persists via temp file + `Sync` + atomic `os.Rename`.
   - `ClearExecutionDataForTemplate` creates clean date-free `.asterml` templates under `<Vault>/templates/`.
   - Planned archival interchange builds on the open RO-Crate `.eln` (`application/vnd.eln+zip`) profile alongside `./assets/`.
