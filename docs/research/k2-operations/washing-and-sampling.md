# Washing and Sampling/Aliquot Operations — Reusable Contract Draft

`review_status: research_draft`

> Do NOT treat as `source_checked`. Three sources below were read directly on
> 2026-11-05 (§2: S1–S3). All other values are search-summary reports or
> explicitly UNKNOWN. No numeric value in this file is invented, but only
> §8-listed rows count as personally verified. No second-person review done.

## 1. Identity

| Field | Value |
|---|---|
| Document | Reusable operation definitions for washing and sampling/aliquoting |
| Role | K2 operation contracts (planning + execution preconditions/checks), not instrument profiles |
| Operations | `wash` (liquid exchange, sample preserved) and `sample_aliquot` (sample split, child aliquot created) |
| Scope | ELISA plate washing, adherent cell-culture vessel washing, culture-supernatant sampling/aliquoting |
| Out of scope | Dissociation/trypsinization, centrifugation as a standalone op, filtration, freezing/storage ops, bioreactor in-process sampling hardware |

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator | Read status |
|---|---|---|---|---|
| S1 | Thermo Fisher, "Adherent Cell Culture Protocol" (Gibco Cell Culture Basics) | https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/subculturing-adherent-cells.html | "Protocol for passaging adherent cells", steps 1–4 | Read directly 2026-11-05 |
| S2 | Arbor Assays, "Plate Washing Instructions" v.231213 | https://www.arborassays.com/wp-content/uploads/2023/12/Plate-Washing-Instruction_231213.pdf | Full 2-page document | Read directly 2026-11-05 |
| S3 | Stanford HIMC, "Cell culture supernatant preparation and normalization" (2024-03-05) | https://iti.stanford.edu/content/dam/sm/iti/documents/himc/Cell%20culture%20supernatant%20preparation%20and%20normalization%203-5-24.pdf | Full 1-page document | Read directly 2026-11-05 |
| S4 | Abcam, "SimpleStep Ignite ELISA: Tips for Optimal Assay Performance" | https://www.abcam.com/en-us/technical-resources/troubleshooting/simplestep-ignite-elisa-tips | Wash tips section | Search summary only — NOT personally verified |
| S5 | Corning, "Optimizing the Separation Step on 96-well Microplates for ELISA Assays" (CLS-DD-AN-457) | https://www.corning.com/catalog/cls/documents/application-notes/CLS-DD-AN-457.pdf | Separation-step guidance | Search summary only — NOT personally verified (direct fetch HTTP 403) |
| S6 | CSH Protocols, "Collecting and Storing Hybridoma Tissue Culture Supernatants" (2020) | https://cshprotocols.cshlp.org/content/2020/10/pdb.prot103317.full | Clarify / sterile-filter / aliquot / store steps | Search summary only — NOT personally verified (direct fetch HTTP 403) |
| S7 | FAU, "SOP: untargeted metabolomics" v3-final (2026-05-29) | https://www.pharmakologie.med.fau.de/files/2026/06/SOP-metabolomics_20260529_v3-final.pdf | Supernatant sampling § | Search summary only — NOT personally verified |

## 3. Core distinction (read first)

Washing and sampling **MUST NOT share one default operation**. They have
opposite sample effects:

| Aspect | `wash` | `sample_aliquot` |
|---|---|---|
| Sample identity | Conserved: same vessel/sample ID before and after | Split: parent keeps ID, each aliquot gets a **new child sample ID** with a lineage link to the parent |
| What leaves the vessel | Waste liquid (discarded or aspirated to waste; never a tracked sample) | A tracked aliquot (volume, destination, label, storage condition) |
| Volume conservation | Vessel working volume is **replaced**, not conserved (spent liquid → waste; fresh buffer/medium → vessel) | Parent volume is **conserved**: `parent_remaining + Σ aliquots = parent_before` (within pipetting tolerance) |
| Effect on downstream assay | Removes unbound material / serum traces; insufficient washing → high background, reduced sensitivity, increased variability (S2) | No purification; an aliquot is representative of the parent at sampling time (S3 normalization; S6/S7 per summaries) |
| Repeatability semantics | Cycles are idempotent-ish repeats of the same exchange; extra cycles beyond protocol **may reduce sensitivity** (S2) | Each draw is a new split event; re-sampling the same parent is a new child ID, never a "cycle" |

## 4. Operation contract tables

### 4.1 `wash`

| Item | Value |
|---|---|
| Inputs | Target vessel(s) with attached/bound sample; wash buffer (specified formulation, e.g. Ca²⁺/Mg²⁺-free balanced salt solution per S1; 1X Wash Buffer diluted from concentrate per S2); waste path |
| Outputs | Same vessel(s), same sample ID, with exchanged liquid; waste liquid (untracked) |
| Ports | `vessel_in` (required) → `vessel_out` (same identity); `buffer_in` (consumable); `waste_out` (sink, untracked) |
| Sample transition | `PRESENT → PRESENT` (no new sample; vessel ID conserved) |
| Completion state | Vessel contains post-wash liquid at protocol volume; proceeds **directly** to next step without drying (S2: "Proceed directly to the next assay step to prevent wells from drying out") |

### 4.2 `sample_aliquot`

| Item | Value |
|---|---|
| Inputs | Parent sample (vessel/tube/plate well); destination consumables (tubes/vials/plates); optional clarification step (centrifugation) |
| Outputs | Parent sample (reduced volume, same ID) + ≥1 child aliquots (new IDs, lineage → parent) |
| Ports | `parent_in` (required) → `parent_out` (same ID, reduced volume); `aliquot_out[i]` (new ID each); optional `background_aliquot_out` (media-only control per S3) |
| Sample transition | `PRESENT(n) → PRESENT(n − Σ aliquots) + ALIQUOT(1..k)` with lineage records |
| Completion state | Each aliquot labeled, volumed, and dispositioned (use immediately or specified storage per S3); parent returned to culture/storage or advanced per protocol |

## 5. Parameters

### 5.1 `wash` parameters

| Parameter | Type | Unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|---|
| `wash_buffer_formulation` | enum/string | — | S1, S2 | plan | S1: balanced salt solution **without calcium and magnesium**; S2: 1X Wash Buffer diluted from concentrate per kit protocol. Formulation is protocol-specific; no universal default. |
| `wash_volume_per_area` (cell culture) | number | mL per 10 cm² | S1 step 3 | plan | S1 states "approximately 2 mL per 10 cm² culture surface area". Approximate — vessel geometry dependent. |
| `wash_volume_per_well` (ELISA plate) | number | µL/well | S2 | plan | S2: "at least 300 µL" 1X Wash Buffer per well. Kit protocols set exact value/repeats. |
| `wash_volume_per_well` (SimpleStep ELISA) | number | µL/well | S4 | plan | 350 µL/well reported — NOT personally verified. Keep kit-specific, not a global default. |
| `cycles` | integer ≥ 1 | count | S2 (repeats "as described in the assay protocol"); S5 (3–5 reported) | plan | Kit protocol owns the count. S2 warns extra washes beyond recommendation may reduce sensitivity. S5's 3–5 range is NOT personally verified. |
| `soak_time` | number | s or min | S4 (1–2 min reported); S5 (5-min final soak reported) | run | Neither personally verified — see UNKNOWNs. Must be kit-specified, never defaulted. |
| `removal_method` | enum: `aspirate` \| `invert_decant` \| `automated_washer` | — | S2 | plan | S2 allows aspirate or inversion + blot; automated washer → follow washer manufacturer's instructions. |
| `blot_step` | boolean | — | S2 | run | S2: invert + firmly blot on lint-free tissue; fresh tissue for final blot; change tissues frequently. |
| `aspiration_height` / tip clearance | number | mm | — | run | UNKNOWN — no source states a value (see §7). |
| `dispense_force` | qualitative | — | S5 (reported) | run | "Equal, gentle dispensing"; avoid vigorous jets that strip bound protein — NOT personally verified. No numeric value exists in verified sources. |
| `prevent_drying` | boolean (MUST be true) | — | S1 (rock gently, avoid disturbing layer), S2, S4 | run | S2: proceed directly to next step. Drying is a failure mode, not a parameter to relax. |
| `sterility` | constraint | — | S1 | run | S1: solutions/equipment contacting cells MUST be sterile; work in laminar flow hood. Applies to cell-culture wash only. |

### 5.2 `sample_aliquot` parameters

| Parameter | Type | Unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|---|
| `parent_sample_id` | string (ref) | — | contract | run | Required. Parent identity conserved. |
| `aliquot_volume` | number | µL or mL | protocol-specific | plan | No universal default. S7 reports 1 mL/well collection and 0.5 mL aliquots — NOT personally verified. Must come from the assay SOP. |
| `aliquot_count` | integer ≥ 1 | count | contract | plan | Includes optional background-media aliquot (S3: "submit an aliquot of the media for background level"). |
| `destination_type` | enum/string | — | S3 | plan | S3: clean polypropylene microcentrifuge tubes; low-binding tubes/vials or 96-well low-binding polypropylene plates for submission. |
| `clarification` (optional pre-step) | struct: `speed, time, temp` | rpm or ×g; min; °C | S3 | plan | S3: 1400 RPM, 10 min, 4 °C to remove particles before aliquoting. Unit is RPM as written (rotor-dependent ×g UNKNOWN). S6/S7 report 1000 ×g variants — NOT verified. |
| `sterile_technique` | constraint | — | S1, S6 (reported) | run | S1 sterile setup for any cell-contacting step. S6 sterile-filter + small-vial aliquoting reported, not verified. |
| `label_traceability` | constraint | — | S3 | run | S3: readable labels matching a plate/vial list (Excel list for submission). Every aliquot needs parent-link + index. |
| `disposition` | enum: `use_immediately` \| `store` | — | S3 | run | S3: use immediately or store at −80 °C; avoid multiple freeze/thaw cycles. S6 reports 4 °C short-term / −20 °C long-term — NOT verified. |
| `homogeneity_assumption` | flag + method | — | S3 | plan | S3 normalizes by equal plating / end-point live-cell counts. Whether the parent is mixed before drawing is protocol-defined; never assume homogeneity silently. |

## 6. Preconditions and completion checks

### 6.1 `wash`

Preconditions:

1. Target vessel present at the wash station with correct identity (P1).
2. Wash buffer prepared to protocol formulation and volume for `cycles × vessels` (P2; S2: dilute concentrate to 1X per assay protocol).
3. Waste path available; lint-free tissues / aspiration available per `removal_method` (P3; S2).
4. Cell-culture wash only: sterile buffer, sterile contact path, hood (P4; S1).
5. Bound/attached fraction confirmed present where applicable (e.g. cells ≥ attachment threshold before a pre-dissociation wash) — else washing an empty vessel is a protocol error, not a no-op (P5; derived from S1 steps 1–3, no value invented).

Completion checks:

1. Spent liquid removed and fresh buffer exchanged for all `cycles` (C1).
2. Residual liquid removed per method (aspirated/blotted) without scratching well interiors (C2; S2: "take care not to scratch the inside of the well").
3. No drying interval before the next step (C3; S2).
4. No new sample IDs created; vessel ID unchanged (C4; contract §3).

### 6.2 `sample_aliquot`

Preconditions:

1. Parent sample present, identified, and sufficient volume for `Σ aliquots + dead volume` (P1; contract).
2. Destination consumables present, correct type, pre-labeled (P2; S3).
3. Clarification (if specified) completed before drawing (P3; S3 ordering).
4. Sterile contact path where the parent is a live culture (P4; S1).

Completion checks:

1. Each aliquot volumed into its destination; volume conservation holds within tolerance (C1; §3).
2. Each aliquot labeled with parent link + index; list matches physical labels (C2; S3).
3. Disposition applied: immediate use handoff or specified storage (C3; S3).
4. Parent returned to correct next state (incubator / next step), not left at bench indefinitely (C4; derived handling rule — no time value stated, see UNKNOWNs).

## 7. Failure modes

| # | Operation | Failure | Effect | Source / status |
|---|---|---|---|---|
| F1 | wash | Insufficient washing (too few cycles / too little volume) | High background, reduced sensitivity, increased variability | S2, verified |
| F2 | wash | Extra washes beyond protocol | May reduce assay sensitivity | S2, verified |
| F3 | wash | Wells dry out between steps | Assay degradation; next-step chemistry compromised | S2 ("prevent wells from drying out"), verified |
| F4 | wash | Scratch well interior during aspiration | Damages bound layer; variability | S2, verified |
| F5 | wash | Serum/Ca²⁺/Mg²⁺ carryover into dissociation step | Inhibits dissociation reagent | S1 step 3 rationale, verified |
| F6 | wash | Disturbing attached cell layer (harsh dispense / wrong side) | Cell loss / detachment before intended step | S1 ("gently add … opposite the attached cell layer"), verified |
| F7 | wash | Non-sterile buffer/contact path (cell culture) | Contamination | S1, verified |
| F8 | sample_aliquot | Drawing from unsettled/non-uniform parent without a mixing rule | Unrepresentative aliquots; normalization invalid | Derived from S3 normalization requirement; mixing rule itself UNKNOWN |
| F9 | sample_aliquot | Wrong/low-binding consumable not used | Analyte loss to tube walls | S3 (low-binding requirement), verified |
| F10 | sample_aliquot | Label/list mismatch | Untraceable aliquots; submission rejected | S3, verified |
| F11 | sample_aliquot | Repeated freeze/thaw of aliquots | Degradation (analyte- and assay-dependent) | S3 ("avoid multiple freeze/thaw"), verified |
| F12 | sample_aliquot | Insufficient parent volume (dead volume ignored) | Short-filled or missing aliquots | Contract derivation; dead-volume value UNKNOWN |
| F13 | either | Vessel/sample identity confusion (wash keeps ID, aliquot splits ID) | Lineage corruption if the two ops share one code path | Contract §3; the reason they MUST NOT share a default |

## 8. What was verified vs. not

Verified by direct reading (2026-11-05):

- S1: spent-media removal → Ca²⁺/Mg²⁺-free balanced-salt wash at ~2 mL/10 cm² → discard; gentle addition opposite the cell layer; rocking; sterile technique + hood. (No soak time, no aspiration height, no cycle count stated.)
- S2: purpose = remove unbound material; under-wash → background/sensitivity/variability; over-wash → may reduce sensitivity; ≥300 µL/well; aspirate-or-invert + blot on lint-free tissue; don't scratch wells; automated washer → manufacturer's instructions; never let dry.
- S3: centrifuge 1400 RPM × 10 min × 4 °C; aliquot clarified medium into clean polypropylene tubes; use immediately or −80 °C; avoid multiple freeze/thaw; sub-aliquot for future assays; background media aliquot; low-binding tubes/plates; labeled list.

NOT verified (do not use for planning until read directly):

- S4/S5 numeric wash details (350 µL/well, 1–2 min soak, 3–5 cycles, 5-min final soak, gentle-dispense rationale).
- S6 hybridoma workflow (1000 ×g clarification, sterile filtration, Tris buffering, azide, 4 °C/−20 °C storage).
- S7 metabolomics volumes (1 mL/well, 0.5 mL aliquots, 1000 ×g, NaCl wash ratio).
- Any plate-washer manufacturer's instructions (S2 defers to them; none read here).

## 9. UNKNOWNs (do not use for planning)

| Item | Status | Reason |
|---|---|---|
| Aspiration height / tip clearance (wash) | UNKNOWN | No verified source states a height in mm. |
| Soak time values | UNKNOWN | Only unverified S4/S5 reports; no verified number. Kit-specified. |
| Cycle counts | UNKNOWN (globally) | Kit/protocol-specified; S2 owns repeats to the assay protocol. |
| Wash buffer formulations beyond "Ca/Mg-free salt" / "kit 1X" | UNKNOWN | pH, detergent, ionic strength are kit-specific; none verified here. |
| Vortex/shake speed during soak or wash | UNKNOWN | No verified value. |
| Centrifuge ×g equivalent of S3's 1400 RPM | UNKNOWN | Rotor-dependent; S3 states RPM only. |
| Dead/touch volume for aliquoting | UNKNOWN | Pipette- and tip-dependent; no source read. |
| Bench-time limit for parent during aliquoting | UNKNOWN | No source states a time limit. |
| Mixing rule before drawing (invert / swirl / none) | UNKNOWN | S3 implies representative sampling via normalization but states no mixing action. |
| Analyte-specific storage condition (beyond S3 −80 °C-or-immediate) | UNKNOWN | Assay-dependent; S6 alternatives unverified. |
| Automated washer model parameters | UNKNOWN | Deferred to washer manufacturer manuals (S2); none read. |
| Suspension-culture wash (centrifuge-resuspend) parameters | UNKNOWN | Only adherent wash (S1) verified; suspension handling not covered here. |

## 10. Time scope and instrument requirements

| Aspect | `wash` | `sample_aliquot` |
|---|---|---|
| Time scope | Within-step liquid exchange; bounded by `cycles × (dispense + soak + remove)`; no verified durations — planner MUST NOT assume a duration | Within-step split event; bounded by draws + labeling; no verified durations |
| Instruments | Manual: multi/repeat pipettor or squirt bottle + aspiration (S2); automated: plate washer per its manual (S2); cell culture: biosafety cabinet for sterility (S1) | Pipettor + tubes/plates (S3); centrifuge if clarification specified (S3); freezer (−80 °C) if stored (S3) |
| Calibration dependence | Washer alignment/calibration matters per S4-report (unverified); dispense accuracy matters for ≥300 µL/well (S2) | Pipetting accuracy determines volume conservation (contract); centrifuge speed/temp per S3 |

## 11. Implications for the operation model

1. Model `wash` and `sample_aliquot` as **separate operation types** with separate parameter schemas; a shared "liquid handling" base MUST NOT supply default cycles, volumes, or soak times.
2. `wash` conserves sample identity; `sample_aliquot` forks it. Lineage tables, volume bookkeeping, and completion checks differ per §3–§6 and failure F13.
3. Every numeric parameter (volume, cycles, soak, speed, time, temp) requires a protocol/kit source at plan time; UNKNOWNs in §9 are rejection reasons for auto-planning, not fill-in-with-typical-value slots.
4. Sterility (S1) and no-drying (S2) are hard constraints on the wash path, not preferences.
