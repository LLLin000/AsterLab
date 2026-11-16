# Experiment Processor Contract v1 (EXP-14)

- Status: `v1 draft for review` — semantic mapping only, no runtime change.
- Date: 2026-11-16. Card: EXP-14 (Contract), risk L1.
- Authority order: `CONTEXT.md` (terms) > `AGENT_API.md` (callable fields) >
  `docs/contracts/readout-schema-v1.md` + `docs/contracts/operation-module-v1.md` +
  `docs/architecture.md` (storage/projection boundaries).
- Research inputs (all `research_draft`, NOT executable):
  `docs/research/k2-operations/viability-and-cytotoxicity-methods.md` (CCK-8/LDH/LIVE-DEAD),
  `docs/research/k2-operations/qpcr-and-nucleic-acid-qc.md` (R1/R2/R3),
  `docs/research/k2-operations/biofilm-cv-cfu.md` (culture/wash/CV/dispersal/CFU).
- Non-goals: no new readout table, no runtime schema, no generic processor
  framework, no second execution engine; no promotion of `research_draft`
  catalog JSON to executable modules; no change to ReviewRecord/CompilePolicy
  gates; no Scratch/image, Flow/FCS, DOE, power analysis, complex statistics,
  or wet-lab validation.

## 0. What this contract is

A reviewable mapping from three existing research records onto AsterLab's
**already-implemented** carriers:

1. Definition side: `ProtocolBlockVersion` (+ `BlockUse` + `ParameterBinding`)
   composed into a `ProtocolVersion`, referenced per-Assay by a `DesignVersion`.
2. Execution side: a `Run` freezing that `DesignVersion` + protocol manifest,
   with facts recorded only as append-only `execution_event` records.
3. Readout side: typed `Readout` values validated by `readout-schema-v1`
   (`ReadoutSchema` + `ReadoutRegistry`; builtins `cck8_od450`,
   `rna_concentration`, `rna_purity`, `cfu_count`) and projected to legacy
   `readouts[]` only where `legacy_form()` allows it.

Nothing here creates a processor runtime. "Processor" in this document means
"which existing block / event / readout carries which research datum", plus
the checks that reject an incomplete claim. Any computation (blank
subtraction, viability %, ΔCq, CFU/mL) is a `ToolAction` (pure) or downstream
derivation recorded as a `derived` readout with `method` + `derived_from` —
never a new step type, never a silent rewrite of a raw value.

Reuse vocabulary (one label per mapped step, cf. operation-module-v1 §5):

- `direct` — use the existing block/event/readout as-is.
- `parameterized` — same block with method-specific binding (wavelength,
  volume, time, solvent).
- `variant` — an existing sibling variant covers it (record which one).
- `composite` — a fixed combination of existing blocks (parent only holds
  order + bindings, cf. `BlockUse`).
- `method template` — research-side combination (`MT-*` / `OP-*`); compiles to
  `composite` + `ToolAction`, never to a new atom.
- `ToolAction` — deterministic calculation/preview, `writes_data=false`,
  no `execution_event` of its own.
- `Custom` — reserved for a genuinely new stain/material/process with an
  unstable boundary (none needed for the three families below).
- `Unknown` — explicitly unplanned; carried as a gap, never zero-filled.

## 1. Authoritative carriers (already implemented; this contract adds none)

| Research need | AsterLab carrier | Notes |
|---|---|---|
| Reusable operation definition | `ProtocolBlockVersion` (immutable; combination blocks pin exact sub-block versions) | Research `MT-*`/`OP-*` IDs MUST NOT be converted to block IDs; they are provisional. |
| One use of a definition | `BlockUse` (`use_id` + exact `block_version_id` + `bindings`) | Same block in two assays = two uses, two bindings, no shared physical execution implied. |
| Per-use inputs | `ParameterSlot` + `ParameterBinding` | Unit must match exactly; no implicit conversion (readout-schema-v1 §1.3, AGENT_API `calculator.*`). |
| Top-level method shown in an Assay | `ProtocolVersion` (`root_block_version_id` + `root_bindings`) | `Design.assays[].protocol_version_id` points at it. |
| Planned science | `Design` / `DesignVersion` (groups, assays, endpoints, timepoints, `planned_outputs`, `sample_allocations`) | Time scope of a *step* (`once`/`per_timepoint`/…) never invents a *design* timepoint. |
| One real execution | `Run` + frozen manifest (`design_version_id`, `protocol_manifest`, `material_manifest`) | `run.workspace` reads the frozen snapshot; missing snapshot is an error, never a fallback to current Design. |
| Something actually happened | `execution_event` (`operation`/`material_use`/`measurement`/`observation`/`deviation`, optional `occurrence_id`, `step_path` + `protocol_version_id` required for measurements) | Steps do not prove execution; plans do not prove material sharing. |
| Typed readout | `Readout` + `ReadoutSchema` v1 (`value_kind`: `numeric`/`plate`/`table`/`categorical`/`file`; `source`: `raw`/`derived`; `missing`: `not_detected`/`below_lod`/`above_uloq`/`invalid`/`other`) | Raw carries no `method`/`derived_from`; derived requires both and may only cite same-Run raw readouts. Builtins cover `cck8_od450` (plate, AU), `rna_concentration` (numeric, ng/µL), `rna_purity` (numeric, ratio), `cfu_count` (numeric, CFU). |
| Deterministic math | `ToolAction` / `calculator.*` | Pure preview; writing the result is a separate `derived` readout with lineage. |

Global checks that already exist and stay in force:

- `readout.batch` must resolve to exactly one plate of this Run
  (`unresolved_plate` / `ambiguous_plate` / `unknown_plate`), else reject.
- `plate` cells carry per-cell `number | missing`; top-level `missing` on a
  plate/table/file is `top_level_missing_not_applicable`.
- Scalar `readout.submit` is `value` XOR `qualifier`; `qualifier` rows never
  enter `measurements[]` and are never written as `0`.
- qPCR CSV may spell the value `Undetermined` → stored as `not_detected`.
- `well_role` (`unknown`/`ntc`/`standard`/`positive_control`) and
  `dilution_factor` on `well.set` are plan annotations, not readouts.
- Unknown enum spellings (e.g. `above_ulog`) are deserialization errors, not
  issues to be aliased (readout-schema-v1 §8).

## 2. CCK-8 (`MT-CCK8-DEHYDROGENASE`, method template)

Source: viability doc §§3–4, S1 (Dojindo CK04) + S6 (Bio-protocol exemplar).
Claim boundary: OD450 formazan signal under a recorded 1–4 h development is
proportional to living-cell number only under same-cell-type, same-incubation
controls; it is NOT viability, NOT cell number, NOT concentration by itself.

### 2.1 Raw vs derived

| Layer | Carrier | Fields |
|---|---|---|
| Raw | `Readout` (`cck8_od450`, `plate`, unit `AU`, `source.kind=raw`, no `method`/`derived_from`) projected per cell to legacy `readouts[]` (`endpoint=od450`, `result_kind=raw`) | `cells[]`: `coordinate` + `number` (450 nm, recorded λ; reference ≥600 nm where used) or per-cell `missing` (`above_uloq`, `not_detected`, …). Unrecorded cell = no `number` + no `missing` (≠ 0, ≠ `not_detected`). |
| Blank subtraction | `ToolAction` (pure) | Inputs: same-plate cell-free medium + CCK-8 blank well IDs (required companion, §2.2). Output re-recorded as derived, not overwritten. |
| Viability % / cell-number-mapped value | `Readout` (`source.kind=derived`, `method=<tool@version>`, `derived_from=[<raw readout_ids>]`) | `derived_from` cites only same-Run **raw** OD readouts; citing another derived readout is `derived_source_not_raw`. |

### 2.2 Required companions (same Run; readout alone proves nothing)

- Same-plate blank: cell-free medium + CCK-8, with well IDs recorded
  (phenol-red/medium background is subtractable, not ignorable).
- Vehicle control wells.
- Cell-number standard curve (same cell type) for ANY count claim.
- Development record: 10 µL/well (96-well) or 10% total volume; 1–4 h at the
  recorded incubator condition; λ = 450 nm (recorded 430–490 nm fallback) +
  reference λ where used; no-bubble precondition.
- Interpretation caveat carried as note, not as a gate: dehydrogenase
  affectors and reducing agents break the OD↔cell-number mapping.

### 2.3 Lineage

`cells_cultured → [dose] → cells_dosed → [CCK-8 addition + development] →
plate_developed → [absorbance read] → raw_OD_matrix (+ optional derived)`.
Optically non-consumptive read, but wells are post-reagent — never return to a
pre-reagent state. Time scope: exposure clock belongs to the dosing SOP;
development is a `per_assay` hold; the read itself is instrument-seconds.

## 3. RT-qPCR (`NA_EXTRACT_QC` / `RT_SYNTH` / `QPCR_ACQ`, method templates)

Source: qPCR doc §§5–11, MIQE checklist S2 (agent-verified rows only).
Claim boundary: raw Cq is a detection-cycle number under stated
threshold/baseline rules — NOT expression, NOT copy number, NOT fold-change.
One-step vs two-step, SYBR vs probe, relative vs absolute MUST NOT be
collapsed (§4 of the research file).

### 3.1 Raw vs derived

| Layer | Carrier | Fields |
|---|---|---|
| Raw curves + Cq | `Readout` (`numeric`, `source.kind=raw`); batch CSV path via `readout.batch` (`field=cq`) | Per-well curve artifact: `categorical`/`file` readout or `asset.link` citation (method[this contract] = record the reference, do not invent a curve math). Per-well Cq: numeric value + `cq_method` + analysis program source/version recorded on the event (`content`/`_asset` citation). `Undetermined` → `not_detected` (never 0, never null-without-reason where the schema forbids it). |
| `not_detected` and sibling qualifiers | `qualifier` (`not_detected`/`below_lod`/`above_uloq`/`invalid`/`other`) | `value` XOR `qualifier`; qualifier rows stay in `readouts[]`, out of `measurements[]`. A late NTC Cq is data with a stated outlier disposition, not an auto-pass. |
| ΔCq | `derived` readout (`method` + `derived_from=[target raw Cq, reference raw Cq(s)]`) | Requires justified reference gene(s) + stated normalisation method (both E). |
| ΔΔCq / efficiency-corrected ratio | `derived` readout | Additionally requires calibrator-group definition + measured efficiencies (from that run's standard curve slope, never a textbook value). |
| Absolute copy number | `derived` readout | Requires same-run standard series: slope, y-intercept, efficiency, r², linear dynamic range, LOD evidence. |
| Curve fitting / efficiency-from-slope / normalisation / statistics | `ToolAction` + versioned analysis software (source/version recorded) | Mathematics lives in the software, not in this contract; results re-enter only as `derived` readouts with full companions. |

### 3.2 Required companions

- Every R3 run: NTC incl. Cq for SYBR (E); specificity evidence
  (gel/sequence/melt/digest; melt mandatory in practice for SYBR).
- Validation stage: ±RT (`cq_with_rt` / `cq_no_rt`) pair for gDNA-carryover;
  contamination assessment; inhibition test (dilution/spike/other, no universal
  cutoff carried here).
- Relative claims: reference-gene number/choice justification + normalisation
  description; calibrator group; replicate counts with stage (RT vs qPCR).
- Absolute/efficiency-corrected claims: full standard-curve block (§3.1).
- Run record: complete reaction conditions (volumes, primer/probe/Mg++/dNTP,
  polymerase, buffer/kit, additives), thermocycling program per run (never
  copied between assays), plate/tube catalogue, channel/dye assignment matched
  to fitted filters, well map (NTC/NRT/standards/references/unknowns).
- No reference gene, cycling program, threshold, or efficiency value is
  defaulted by this contract — all are per-assay `Unknown` until recorded.

### 3.3 Lineage

- Two-step: `rna → [RT_SYNTH] → cdna_archived (+ rt_qc) → [QPCR_ACQ aliquot] →
  consumed_well + raw_curves/Cq (+ derived)`.
- One-step: `rna → [RT-phase inside QPCR_ACQ] → amplicon_signal`; no archived
  cDNA port exists.
- Amplified wells are terminal (post-PCR product); they MUST NOT re-enter
  culture, extraction, or a second qPCR as template without an explicit
  documented dilution/transfer.

## 4. Biofilm CV + CFU (five `OP-*` records, composite/method-template family)

Source: biofilm doc §§3–9, S1 (O'Toole JoVE) + S2 (Merritt/CurrProtoc) + S3
(ASM serial dilution). Core rule: **CV and CFU are not interchangeable.**
CV = total surface-associated dye-binding material (cells + matrix +
anything adherent); CFU = viable culturable cells forming colonies after
dispersal. Converting one into the other is a modeling error, not a gap.

### 4.1 CV biomass (`OP-CV-BIOMASS`, method template)

| Layer | Carrier |
|---|---|
| Raw | `Readout` (`plate`-shaped where full-plate; else per-well `numeric` raw, unit `AU`-equivalent with recorded λ) + same-solvent blank well IDs + recorded λ (S1: 550 nm; S2: 500–600 nm reader range — record actual λ) + solvent ID. Solubilized-dye OD only; qualitative dry-well photo (optional, pre-solubilization) is a `file` readout / `asset.link`, never a number. |
| Derived | Blank-subtracted OD and replicate mean ± SD are `derived` readouts (`method` + `derived_from` = raw well ODs). Cross-organism / cross-solvent CV values are NOT commensurable (solvents differ by organism: 30% acetic acid default; DMSO/ethanol variants per Table 1B.1.1). |

Companions: same-solvent blank; medium-only wells carried through wash +
stain (background); positive-control strain + negative control per plate;
4–8 replicates per condition; recorded stain binding (0.1% CV, 125 µL/well,
10–15 min RT) and solubilization binding (solvent + volume + 10–15 min;
S1 125 µL vs S2 200 µL + 125 µL transfer conflict kept — SOP owns the
choice, never mix volume of one with transfer of the other).

### 4.2 CFU (`OP-CFU-DILUTION-PLATE`, method template)

| Layer | Carrier |
|---|---|
| Raw | Per-plate colony count as `Readout` (`cfu_count`, `numeric`, unit `CFU`, `source.kind=raw`), linked to the plated child sample + well lineage (§4.4). Only the 30–300 plate counts; duplicates averaged (S3). |
| Derived | `CFU/mL = colony count ÷ (total dilution of plated tube × volume plated in mL)` (S3 p.2 + p.4 worked example: 51 colonies × 1/10² × 0.1 mL → 5.1×10⁴ CFU/mL); `CFU/well = CFU/mL × suspension volume` (S2 Alt: 2.0 mL total per tube). Both are `derived` readouts (`method=s3_cfu_formula@1` or the SOP's stated variant + `derived_from=[raw count readout(s)]`). Recording only the derived CFU/mL while dropping raw count + dilution + volume is rejected at review (F-P3). |

Companions: kill-control (planktonic before/after identical sonication —
gates the whole CFU batch); detachment check (CV re-stain of carrier,
optional but recommended); diluent/sterility control per SOP; sterile fresh
pipette per dilution + tube agitation (technique constraint, not a number).

### 4.3 Lossy wash (`OP-BIOFILM-WASH`, lossy washing variant)

Washing MUST NOT reuse a generic sample-preserving wash default. Model as
`ATTACHED + PLANKTONIC → ATTACHED_RETAINED + PLANKTONIC_REMOVED +
DETACHMENT_LOSS(?)` where the loss magnitude is always possible and UNKNOWN
without a same-organism control. Wash liquid (tap-water tubs vs sterile PBS
for the CFU path), cycle counts (S1 vs S2 differ — SOP owns the count), and
vigor are recorded on the `execution_event` (`content`); loss magnitude is
carried as an interpretation caveat, never netted against background effects
(over-vigorous wash lowers both CV and CFU; post-stain rinse deficit raises CV
background without touching CFU — asymmetric, §9 of the research file).

### 4.4 Well → tube → plate lineage (CFU path)

`well (plate_cultured, both fractions) → [Alt PBS wash 6×] → well (retained)
→ [cut well + 100 µL PBS into 1.9 mL PBS = 2.0 mL total] → suspension tube
(new child sample ID, lineage → well) → [serial dilution] →
dilution_series_out[i] (child IDs per tube) → [plating] → plate_out[j]
(child IDs per plate) → colony-bearing plates (terminal, biohazard-disposed)
+ derived_CFU dataset`. Each well maps 1:1 to a labeled tube; dispersal
energy (S2 example: microtip 8 s at 40% — device-relative, needs its own
kill-control on any other device) is recorded, not generalized. The stained
CV plate is terminal for biology (dye-fixed, dried, solubilized — never
returns to culture).

## 5. Reuse Map (existing operations first; no new atom proposed)

Inventory consulted: `catalog/operations/` (+ `docs/research/k2-operations/`
generic records) as cited by each research file's §8. Verdicts use the §0
vocabulary. Bindings below are planning-time `ParameterBinding`s on existing
blocks, not new modules.

| Experiment step | Existing operation / carrier | Verdict | Binding / note |
|---|---|---|---|
| CCK-8: seed cells, add test substance | `op.liquid_transfer` | parameterized | 100 µL/well + dosing per S1; tip/mixing per SOP |
| CCK-8: add CCK-8 reagent | `op.liquid_transfer` | parameterized | 10 µL/well (96-well) or 10% total volume; no-bubble check |
| CCK-8: 1–4 h development hold | `op.incubation` | parameterized | 37 °C 5% CO₂ 1–4 h; cell-type-dependent, no default |
| CCK-8: 450 nm read | `op.absorbance_measurement` / `ABS_READ` → `cck8_od450` raw plate readout | parameterized | λ = 450 nm (fallback 430–490 recorded), reference ≥600 nm |
| CCK-8: blank subtraction → viability % | `ToolAction` → `derived` readout | ToolAction | NOT an operation; `derived_from` cites raw ODs |
| LDH-context lysis / aliquot / develop pattern (reference only; LDH execution out of scope for this card's three families) | `op.liquid_transfer` + `op.incubation` + `op.sample_aliquot` | variant | Same atoms as CCK-8 path; kept to show no new "lyse" atom is needed |
| LIVE/DEAD-context stain + image pattern (reference only) | `op.liquid_transfer` + `op.incubation` + `FLUO_IMAGE` | variant | Microscopy path only; counting/calling is downstream, not imaging |
| NA extraction batch (R1) | `op.liquid_transfer` + `op.incubation` (generic mechanics) + R1 MIQE record ports | composite | Extraction kit chemistry stays SOP text; QC metrics are `numeric` raw readouts (conc. ng/µL, ratios dimensionless, RIN/score) + `file` traces |
| RT setup + thermal step (R2) | `op.liquid_transfer` + `op.incubation` | composite | RT T/t belongs to R2; one-step variant is a phase inside R3 with no `cdna_out` port |
| qPCR setup + cycling + acquisition (R3) | `op.liquid_transfer` + `op.incubation` (thermal) + fluorescence acquisition | composite | Curves/Cq are raw readouts; chemistry/channel matching is a pre-run capability check |
| qPCR ΔCq / ΔΔCq / absolute copy | `ToolAction` → `derived` readout | ToolAction | Each tier needs its §3.2 companions or the claim is recorded incomplete |
| Biofilm static culture | inoculation + incubation generic ops | composite | 1:100 into 100 µL/well (96-well verified only); 4–8 replicates; medium/temp/time organism-empirical |
| Biofilm wash (pre- and post-stain; Alt 6× PBS for CFU) | lossy washing variant | variant | NOT `direct` generic wash; loss modeled per §4.3 |
| CV stain + solubilize + OD read | stain + `op.liquid_transfer` (reagent/solvent) + `op.incubation` (holds) + `op.absorbance_measurement` → raw OD | method template | Compiles to composite + ToolAction; CV ≠ cells |
| Biofilm dispersal to suspension | harvest + disaggregate composite (device-dependent) | composite | Kill-control gates plating; well→tube 1:1 lineage with new child IDs |
| Serial dilution + plating + count → CFU/mL | dilute + plate + incubate + count + S3-formula ToolAction | method template | Raw count (`cfu_count`) + derived `CFU/mL` + derived `CFU/well` kept together |
| Any new stain/material/process with unstable boundary | — | Custom | None needed for CCK-8 / qPCR / CV+CFU; reserved per granularity freeze |
| Per-build imaging values, reader photometric envelope, absolute sonication energy, colony-incubation SOP | — | Unknown | Capabilities recorded, never defaulted; gaps stay gaps |

No new atomic operation candidate. Every physical step above resolves to an
existing operation + binding; every missing item is a `ToolAction`
(calculation), a downstream derivation, or a reserved `Custom`/`Unknown`.

## 6. Explicitly deferred (no algorithm carried)

- `Scratch` / image-based wound-healing readouts: no segmentation, tracking,
  or closure-rate math is defined here; image acquisition (if any) would be a
  `file` readout + `asset.link`, analysis stays out of scope.
- `Flow/FCS`: no gating, compensation, or population-call semantics carried;
  FCS files would be `file` readouts with instrument capability references
  only (no Cytometry microscopy path is affected — that stays in the
  LIVE/DEAD research file, itself out of execution scope for this card).
- DOE / factorial layouts, power / sample-size derivations, complex
  statistics (mixed models, multiplicity, error propagation beyond the S3
  CFU/mL arithmetic and MIQE companion rules): not derived here; statistics
  plans are SOP + versioned-software business recorded by source/version.
- LDH-release and LIVE/DEAD-microscopy execution bindings: cited in the Reuse
  Map only as `variant` references so reviewers can see no new atom is
  needed; their full runtime mapping belongs to a future card, not this one.

## 7. `research_draft` → executable threshold (unchanged)

All three research files stay `research_draft`. Promotion is NOT part of
this card and requires, per file:

1. Second-person verification of every sourced row against the cited
   manual/page/section (S1–S9 locators in each file; S4/S5 search-only rows
   and the S8 mirror copy need primary-source reads first).
2. A `ReviewRecord` (reviewer, date, object versions, scope, evidence,
   outcome) — absent in all three files today.
3. The lab's CompilePolicy gate (SOP acceptance cutoffs, reference-gene
   justification policy, kill-control acceptance, replicate policy) — none is
   set by a research draft; second-hand QC ranges (A260/A280, RIN, 28S:18S)
   are context, never gates.
4. Schematization (if ever): UNKNOWNs omitted, never zero-filled; long
   rationale stays out of machine fields; provisional `MT-*`/`OP-*` IDs are
   replaced by real `ProtocolBlockVersion` references via `ModuleBinding`
   (`Imported`/`Derived`/`LocalCustom`), never converted 1:1 to
   `OperationModule` JSON.

## 8. Reviewable synthetic examples (illustrative IDs, not fixtures)

CCK-8 raw plate cell (event fragment; full envelope per
`execution_event.create` + `readout.batch`):

```json
{
  "run_id": "<run>",
  "step_path": "root/cck8/read",
  "protocol_version_id": "<pv>",
  "assay_id": "<cck8-assay>",
  "timepoint": "处理后 24 h",
  "readouts": [
    {"readout_id": "ro-demo-cck8-1",
     "schema_id": "cck8_od450", "schema_version": 1,
     "value": {"kind": "plate", "plate_id": "plate-1",
       "rows": 8, "columns": 12,
       "cells": [
         {"coordinate": "A1", "number": 1.234},
         {"coordinate": "A2", "missing": "above_uloq"},
         {"coordinate": "B1", "missing": "not_detected"}
       ]},
     "unit": "AU",
     "source": {"kind": "raw", "derived_from": []},
     "link": {"timepoint": "处理后 24 h"}}
  ]
}
```

CCK-8 derived viability (separate event; method + lineage required):

```json
{"readout_id": "ro-demo-via-1", "schema_id": "cck8_viability_demo",
 "schema_version": 1,
 "value": {"kind": "numeric", "number": 0.92},
 "unit": "ratio",
 "source": {"kind": "derived", "method": "blank_subtract_viability@1",
   "derived_from": ["ro-demo-cck8-1"]},
 "link": {"timepoint": "处理后 24 h"}}
```

qPCR raw Cq with `not_detected` + derived ΔCq (same-Run raw only):

```json
{"readout_id": "ro-demo-cq-1", "schema_id": "qpcr_cq_demo",
 "schema_version": 1,
 "value": {"kind": "numeric", "number": 24.7},
 "unit": null,
 "source": {"kind": "raw", "derived_from": []},
 "link": {"timepoint": "处理后 24 h", "sample_id": "S-1"}}
{"readout_id": "ro-demo-cq-ntc-1", "schema_id": "qpcr_cq_demo",
 "schema_version": 1,
 "value": {"kind": "numeric"},
 "missing": "not_detected",
 "source": {"kind": "raw", "derived_from": []},
 "link": {"timepoint": "处理后 24 h", "sample_id": "NTC-1"}}
{"readout_id": "ro-demo-dcq-1", "schema_id": "qpcr_dcq_demo",
 "schema_version": 1,
 "value": {"kind": "numeric", "number": 2.1},
 "source": {"kind": "derived", "method": "delta_cq@1",
   "derived_from": ["ro-demo-cq-1", "ro-demo-cq-ref-1"]},
 "link": {"timepoint": "处理后 24 h", "sample_id": "S-1"}}
```

CFU raw count + derived CFU/mL (S3 formula kept auditable):

```json
{"readout_id": "ro-demo-cfu-1", "schema_id": "cfu_count",
 "schema_version": 1,
 "value": {"kind": "numeric", "number": 51.0},
 "unit": "CFU",
 "source": {"kind": "raw", "derived_from": []},
 "link": {"timepoint": "处理后 48 h", "sample_id": "SUSP-A1",
   "target": {"kind": "well", "plate_id": "plate-cfu", "coordinate": "B2"}}}
{"readout_id": "ro-demo-cfu-ml-1", "schema_id": "cfu_density_demo",
 "schema_version": 1,
 "value": {"kind": "numeric", "number": 51000.0},
 "unit": "CFU/mL",
 "source": {"kind": "derived", "method": "s3_cfu_formula@1",
   "derived_from": ["ro-demo-cfu-1"]},
 "link": {"timepoint": "处理后 48 h", "sample_id": "SUSP-A1"}}
```

`CFU/mL` here is 51 ÷ (1/10² × 0.1 mL) = 5.1×10⁴ CFU/mL, mirroring the S3
worked example; the raw count, the total dilution, and the plated volume are
all retained alongside the derived value.

## 9. Unresolved boundaries (carried, not solved)

- CCK-8 per-cell-type linearity, per-lot background, SDS/HCl-stop downstream
  compatibility; non-96-well geometry beyond the 10% rule.
- qPCR per-assay programs, primer/probe sequences, channel/filter bands,
  efficiency/r²/LOD numbers, reference-gene identity, well-map content.
- Biofilm colony-incubation SOP, pour/spread hardware detail, absolute
  sonication energy, suspension/solubilized hold windows, reader photometric
  envelope, wash-loss magnitude, medium recipes beyond the S1 example.
- Cross-kit LDH scale equivalence; cross-chemistry / cross-method
  commensurability (SYBR vs probe, one-step vs two-step, CV vs CFU,
  CCK-8 vs LDH vs imaging) — all refused, not deferred.
- `table` per-cell reasons, `file`↔asset linkage finalization,
  `readout_id` generation rule, mixed-coordinate plates (readout-schema-v1 §8).
