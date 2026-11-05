# Biofilm Culture, Crystal-Violet Biomass, and CFU Viability — Operation Records

`review_status: research_draft`

> Do NOT treat as `source_checked`. S1–S3 below were read directly on
> 2026-11-05. All other values are search-summary reports or explicitly
> UNKNOWN. No numeric value in this file is invented, but only
> §9-listed rows count as personally verified. No second-person review done.
> No `SourceChecked` / `ExpertReviewed` / `LabValidated` claim is made.

## 1. Identity

| Field | Value |
|---|---|
| Document | Operation records for static biofilm work: culture, washing, CV biomass, dispersal, and CFU viability |
| Role | K2 operation contracts (planning + execution preconditions/checks), not instrument profiles |
| Records | `OP-BIOFILM-STATIC-CULTURE` (biofilm growth) · `OP-BIOFILM-WASH` (planktonic removal) · `OP-CV-BIOMASS` (crystal-violet stain + solubilize + OD read) · `OP-BIOFILM-DISPERSAL` (harvest + disaggregate to suspension) · `OP-CFU-DILUTION-PLATE` (serial dilution + plating + colony count → CFU) |
| Scope | Static microtiter-dish biofilm formation and two non-interchangeable readouts (CV total biomass vs CFU viable count) |
| Out of scope | Flow-cell / chemostat mature-biofilm systems; colony-based biofilm system; air-liquid-interface microscopy assay; metabolic / fluorescent viability stains; MBEC / Calgary device; antimicrobial susceptibility interpretation |
| Granularity | NOT a single atomic operation. See [granularity-and-biofilm.md](granularity-and-biofilm.md): these are assay model / method templates / composite blocks, not `OperationModule`s. |

### 1.1 Granularity decision (provisional)

| Record ID | Type | Why it is not an atomic operation |
|---|---|---|
| `OP-BIOFILM-STATIC-CULTURE` | composite block / method phase | inoculation + incubation + optional medium exchange; attached and planktonic fractions coexist |
| `OP-BIOFILM-WASH` | lossy washing variant | discards planktonic fraction and may detach weakly adherent biofilm; not a generic sample-preserving wash |
| `OP-CV-BIOMASS` | method template | wash + stain + wash + solubilize + absorbance; CV measures total adherent material, not viable cells |
| `OP-BIOFILM-DISPERSAL` | composite/device-dependent step | scraping/sonication/vortex combinations; needs kill-control and detachment check |
| `OP-CFU-DILUTION-PLATE` | method template | disperse + dilute + plate + incubate + count + CFU calculation |

These record IDs are provisional research IDs. They MUST NOT be converted directly to `OperationModule` JSON. Atomic sub-operations (inoculation, incubation, lossy wash, stain, solubilize, absorbance read, colony count) belong in separate records/modules.

Core rule (read first): **CV and CFU are not interchangeable.** CV absorbance
reports total surface-associated dye-binding material (cells + matrix +
anything adherent); CFU reports viable culturable cells that form colonies
after dispersal. Either readout alone misrepresents the other compartment.
Washing is **not** a universal sample-preserving operation in biofilm work:
every wash removes planktonic cells by design and can detach weakly adherent
biofilm, causing loss/underestimation.

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator / what was used | Read status |
|---|---|---|---|---|
| S1 | O'Toole 2011, "Microtiter Dish Biofilm Formation Assay", J Vis Exp 47:2437 (PMC3182663) | https://pmc.ncbi.nlm.nih.gov/articles/PMC3182663/ | Protocol §1 steps 1–4 (growth); §2 steps 1–6 (stain); §3 steps 1–4 (quantify); Discussion (replicates, controls, organism-dependence) | Read directly 2026-11-05 |
| S2 | Merritt / Kadouri / O'Toole, "Growing and Analyzing Static Biofilms", Curr Protoc Microbiol 2005 (PMC4568995) | https://pmc.ncbi.nlm.nih.gov/articles/PMC4568995/ | Basic Protocol 1 steps 1a–9 + Table 1B.1.1 (solvents per organism); Alternate Protocol 1 incl. intro paragraph + steps 1–5 (direct enumeration, sonication check) | Read directly 2026-11-05 |
| S3 | Reynolds, "Serial Dilution Protocols", ASM (peer-reviewed CUE 2005) | https://asm.org/asm/media/protocol-images/serial-dilution-protocols.pdf | Standard formula p.2; countable-plate rule + dilution-factor product pp.2–3; worked example p.4 (51 colonies, 1/10², 0.1 mL → 5.1×10⁴ CFU/mL); Comments & Tips (30–300 error rationale, sterile pipettes, agitation) | Read directly 2026-11-05 |
| S4 | ASM "Preparing Spread Plates Protocols" + "Aseptic Laboratory Techniques: Plating Methods" (PMC4846335) | https://asm.org/asm/media/protocol-images/preparing-spread-plates-protocols.pdf ; https://pmc.ncbi.nlm.nih.gov/articles/PMC4846335/ | Spread-plate handling (agar volume, drying, 0.1 mL spread, pour-vs-spread distinction, 30–300 range) | Search summary only — NOT personally verified |
| S5 | Biofilm dispersal methods: biofilmbook "Harvesting and Dispersing of Cells from Biofilms"; JoVE 62390 "Harvesting and Disaggregation" | https://biofilmbook.hypertextbookshop.com/public_version/contents/appendices/appendix001/pages/page009.html ; https://doi.org/10.3791/62390 | Scraping / vortexing / sonication / homogenization combos (e.g. 45 kHz 100% 30 s; ~20,500 rpm ~30 s reported) as alternatives to S2 sonication | Search summary only — NOT personally verified |

## 3. Record A — `OP-BIOFILM-STATIC-CULTURE`

### 3.1 Purpose

Grow surface-attached biofilm on the wall and/or bottom of a microtiter well
under static batch conditions, for early-stage attachment / microcolony readout.
S1 Abstract: static batch growth does not produce flow-cell-mature biofilms but
identifies initiation factors and develops some mature traits (e.g. antibiotic
tolerance). S2 intro: static assays suit early events (adherence, microcolony;
in some cases <60 min) but nutrients may limit and mature biofilms may not form.

### 3.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Inoculum culture (stationary/overnight); fresh biofilm medium; microtiter plate + lid |
| Outputs | Same plate with two fractions: `biofilm_attached` (surface-associated) + `planktonic_free` (suspension above it) |
| Ports | `inoculum_in` (required) → consumed into wells; `plate_in` (required) → `plate_cultured_out` (same identity, now holds both fractions) |
| Sample transition | `PLANKTONIC_INOCULUM → ATTACHED_BIOFILM + PLANKTONIC_COFRACTION` (co-existence, not conversion; the planktonic fraction is a real output that the wash step then discards) |
| Time scope | `per_assay` incubation block, bounded by organism-specific duration (see parameters); not inferable from an assay name |

### 3.3 Parameters (only sourced values)

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `preculture` | text | S1 §1.1 (overnight LB); S2 §1a (3–5 mL to stationary phase) | plan | Medium is organism-dependent; LB / rich medium cited only for the S1 *P. aeruginosa* example. |
| `dilution_into_assay` | ratio | S1 §1.2; S2 §2a (both 1:100) | plan | 1:100 is the verified microtiter value, not a universal inoculation rule. |
| `assay_medium` | text | S1 §1.2 | plan | S1 example: M63 + MgSO₄ + glucose + casamino acids; arginine as sole C-source boosts robust biofilm with less planktonic growth. Organism-specific; S1 Discussion: medium must be determined empirically per microbe. |
| `volume_per_well` | number, µL | S1 §1.3; S2 §2a/§1b (all 100) | plan | 100 µL/well (96-well). No other format verified. |
| `replicates_per_condition` | integer | S1 §1.3 (4–8); S2 §2a/§9-note (4, increasable) | plan | S1 recommends 4–8; S2 uses 4 as the worked default. Take max(protocol): plan 4–8 unless the SOP fixes it. |
| `incubation_temp` | number, °C | S1 §1.4 (37); S2 Table 1B.1.1 (organism rows) | plan | S1 example 37 °C; S2: *P. aeruginosa* 25–37, *E. coli* 25, *S. aureus* 37, *V. cholerae* 25–30, etc. Method-dependent; no default. |
| `incubation_time` | number, h | S1 §1.4 (4–24); S2 §2a-note (commonly within 48) | plan | Range only; S1 Discussion + S2: time course must be determined empirically per organism. |
| `plate_type` | text | S2 Materials | plan | 96-well, **not tissue-culture-treated** (S2 names BD 353911 + lid 353913 as the used product, not a requirement). Other formats: UNKNOWN. |
| `lid_reuse_cleaning` | text | S2 §2a note | run | Lids may be reused after cleaning with 70% (v/v) ethanol + air-dry. |
| `attachment_zone` | enum | S1 Fig.1 + Discussion | plan | Motile → walls and/or bottom (air-liquid interface); non-motile → bottom. Determines where the biofilm is looked for, not a settable parameter. |

### 3.4 Preconditions / completion checks

Preconditions: (P1) inoculum at specified growth stage; (P2) fresh assay medium
per organism SOP; (P3) correct non-treated plate + clean lid; (P4) incubator at
organism temperature. Completion: (C1) all addressed wells received inoculum at
`volume_per_well`; (C2) plate covered and incubated for the planned window;
(C3) replicate count met; (C4) no new sample IDs — well identity conserved,
fractions distinguished by state, not by ID fork.

### 3.5 Instrument requirements

Incubator holding the organism temperature (capability: temperature control
covering at least 25–37 °C; accuracy/uniformity UNKNOWN); manual pipetting for
100 µL dispensing; biosafety handling per organism (S2 CAUTION cites Unit 1A.1 /
Appendix 1B — not read here). No model bound.

## 4. Record B — `OP-BIOFILM-WASH` (planktonic removal; LOSSY by design)

### 4.1 Purpose

Remove planktonic bacteria and media components that would otherwise stain and
inflate background (S1 §2.2; S2 §4), **at the cost of possible detachment of
weakly adherent biofilm**. This operation MUST NOT reuse a generic
"sample-preserving wash" default: the biofilm wash discards one real fraction
(planktonic) and erodes the retained fraction to an unquantified degree.

### 4.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Cultured plate (`biofilm_attached` + `planktonic_free`); wash liquid (tap water per S1/S2 Basic Protocol, or sterile PBS per S2 Alternate Protocol — protocol-owned choice) |
| Outputs | Same plate, planktonic-depleted; wash waste (untracked) |
| Ports | `plate_cultured_in` → `plate_washed_out` (same identity); `wash_liquid_in` (consumable); `waste_out` (sink) |
| Sample transition | `ATTACHED + PLANKTONIC → ATTACHED_RETAINED + PLANKTONIC_REMOVED + DETACHMENT_LOSS(?)`. `DETACHMENT_LOSS ≥ 0` is always possible; its magnitude is UNKNOWN without a same-organism control. Never model as `PRESENT → PRESENT` lossless. |

### 4.3 Parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `wash_liquid` | enum/text | S1 §2.2 (tub water); S2 §3–4 (tap water); S2 Alt §2 (sterile PBS) | plan | Basic-protocol wash uses tap-water tubs; direct-enumeration wash uses sterile PBS (100 µL/well via multichannel). Choice is protocol-owned; do not substitute silently (sterility matters downstream of Alt wash). |
| `pre_stain_cycles` | integer | S1 §2.2 (dump + 2 submerge/shake-outs); S2 §4 (shake-out + 1 submerge/shake-out, replace water when cloudy) | plan | S1 and S2 differ; keep the SOP's own count. No universal cycle value. |
| `post_stain_cycles` | integer | S1 §2.5 (3–4 submerge rinses + vigorous blot); S2 §6 (two successive tray washes + shake-out) | plan | Same conflict handling: SOP owns the count. Purpose per S2 §6: remove non-specific CV. |
| `agitation` | qualitative | S1 §2.2 ("gently submerge"); S2 §4 ("vigorously shake out"); Alt §2 ("vigorously shaking out") | run | Gentle-in, vigorous-out in verified text; no numeric force/speed exists → UNKNOWN magnitude. Vigor is the detachment-loss lever (see failure F-W3). |
| `blot_dry` | boolean + method | S1 §2.5 (blot vigorously on paper towels); S2 §7 (invert + tap vigorously, air-dry) | run | Required post-rinse liquid removal. S2: post-stain air-dry; stained plates then stable weeks at RT. Pre-stain drying interval: UNKNOWN — never insert one silently. |
| `alt_wash_cycles` (CFU path) | integer | S2 Alt §2 (6×: 100 µL PBS in, shake out) | plan | Six PBS exchanges, multichannel-added. Applies only to the direct-enumeration path. |

### 4.4 Preconditions / completion checks

Preconditions: (P1) cultured plate present with both fractions; (P2) wash trays
/ PBS volume sufficient for `cycles × plates` (S2: 1–2 inches water; replace
when cloudy/dark or efficiency drops); (P3) waste tray + paper towels available.
Completion (operational, non-blocking for loss magnitude): (C1) spent/planktonic
liquid removed; (C2) prescribed cycles executed; (C3) no visible pooling before
stain; (C4) well identity unchanged. Loss magnitude is NOT a completion check —
it is an UNKNOWN carried into readout interpretation.

## 5. Record C — `OP-CV-BIOMASS` (stain + solubilize + OD read)

### 5.1 Purpose

Render total adherent biomass visible (qualitative photo of dry wells, S1 §2.7)
and semiquantitative (solubilized-dye absorbance, S1 §3 / S2 §8–9). S2
Alternate-Protocol intro states the boundary explicitly: CV stains essentially
any adherent material including matrix, so CV may **overestimate** adherent
bacteria.

### 5.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Washed plate (`biofilm_attached` retained); 0.1% (w/v) CV in water; solubilization solvent; optically clear flat-bottom plate (for transfer variant) |
| Outputs | Same stained plate (terminal for biology: dye-fixed, dried, then chemically solubilized — never returns to culture) + `raw_OD_matrix` |
| Ports | `plate_washed_in` → `plate_stained_out` (same identity, terminal state); `data_out` (well → OD + λ + solvent blank ID + timestamp) |
| Sample transition | `ATTACHED_RETAINED → ATTACHED_STAINED → SOLUBILIZED_LYSATE`. Destructive/terminal for the biofilm as a biological sample. |
| Time scope | `per_assay` stain + solubilize block; read within the same block (no verified stability window for solubilized CV — UNKNOWN). |

### 5.3 Parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `cv_concentration` | %, w/v in water | S1 §2.3; S2 §5 (both 0.1%) | plan | 0.1% CV in water. Safety note (S1): gloves + coat; CV powder hygroscopic, stains skin/clothing. |
| `cv_volume_per_well` | number, µL | S1 §2.3; S2 §5 (both 125) | plan | 125 µL/well. |
| `stain_time` | number, min, at RT | S1 §2.4 (10–15); S2 §5 (10) | plan | Take the SOP's value inside 10–15 min; S2's flat 10 min is compatible, not contradictory. |
| `solvent` | text | S1 §3.1 (30% acetic acid); S2 §8 + Table 1B.1.1 | plan | Default verified: 30% (v/v) acetic acid in water. S2 Table 1B.1.1 organism variants: DMSO 100% (*Agrobacterium*, *V. cholerae* rows), 80% ethanol/20% acetone (*E. coli*), 95% ethanol (*P. aeruginosa* / *P. fluorescens* / *S. mutans* rows), 30% acetic acid (*P. aeruginosa* Zegans row), 33% glacial acetic acid (*S. aureus*). S2 §8 note: 30% acetic acid solubilizes more efficiently than 95% ethanol across more microbes, but solvents should still be tested empirically per organism. |
| `solvent_volume` | number, µL | S1 §3.1 (125); S2 §8 (200) | plan | Conflict kept: S1 adds 125 µL and reads/transfers from it; S2 adds 200 µL, incubates, mixes by pipetting, transfers 125 µL to a clear plate. Use the SOP's own volume; never mix S1 volume with S2 transfer. |
| `solubilize_time` | number, min, at RT | S1 §3.2; S2 §8 (both 10–15) | plan | 10–15 min covered + RT, per both. |
| `transfer_step` | boolean + volume | S1 §3.3; S2 §9 (both transfer 125 µL to new flat-bottom clear plate) | run | Required where the assay plate optics are unsuitable; S2 specifies optically clear flat-bottom nonsterile plate. |
| `wavelength` | number, nm | S1 §3.4 (550); S2 §9 (500–600 range) | plan + run | S1 fixes 550 nm; S2 allows 500–600 nm for reader variation. Plan 550 nm when the reader supports it; record actual λ always. |
| `blank_well_ids` | string[] | S1 §3.4 (30% acetic acid blank) | run | Same-solvent blank (S1: 30% acetic acid in water). Subtraction is downstream, not in-module. |
| `qualitative_photo` | boolean | S1 §2.7 | run (conditional) | Dry-well photograph before solubilization, optional. Once solubilized, the photo state is gone. |

### 5.4 Preconditions / completion checks

Preconditions: (P1) plate washed per Record B; (P2) CV solution prepared with
protective handling; (P3) solvent + clear transfer plate available;
(P4) reader covering the planned λ. Completion (deterministic): (C1) every
addressed well stained for the planned time; (C2) excess dye washed out per
Record B post-stain cycles; (C3) solubilization transferred where specified;
(C4) every addressed well returned a value with λ, solvent, blank IDs recorded.
Advisory: photograph dry wells before solubilizing if qualitative comparison
is wanted (S1 §2.7).

### 5.5 Instrument requirements

Absorbance reader covering 500–600 nm (capability: photometric readout in the
visible band; model UNSPECIFIED — the verified sources name no reader model).
Optically clear flat-bottom plate for the transfer variant. No fluorescence,
kinetic, or incubation capability is implied by this record.

## 6. Record D — `OP-BIOFILM-DISPERSAL` (harvest + disaggregate)

### 6.1 Purpose

Convert `ATTACHED_RETAINED` into a uniform single-cell suspension countable by
plating, because aggregates plate as one colony and matrix-embedded cells do
not separate spontaneously. No single dispersal method suits all biofilms
(S5 search-summary; consistent with S2's verification-first design).

### 6.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Washed biofilm plate (PBS-washed per Alt path); sterile PBS; sonication/homogenization vessel |
| Outputs | Cell suspension tube(s), one per well (new child sample IDs with lineage → well) |
| Ports | `well_in` (required, consumed as a physical carrier) → `suspension_out[i]` (new ID each); `diluent_in` (PBS) |
| Sample transition | `ATTACHED_RETAINED → SUSPENSION_SINGLECELL(?) + RESIDUAL_ATTACHED(?)`. Both `?` terms are method- and organism-dependent: incomplete detachment leaves `RESIDUAL_ATTACHED` (undercount); over-aggressive energy kills cells (undercount the other way). CV re-stain of the carrier checks the first; planktonic kill-control checks the second (S2 Alt §5 notes). |
| Time scope | `per_assay` harvest block immediately after Alt wash; dispersal energy applied per-tube. |

### 6.3 Parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `carrier_handling` | text | S2 Alt §3–4 | run | Cut each well out with scissors; add 100 µL PBS to the well; drop well + contents into 8 mL tube holding 1.9 mL PBS (final liquid 2.0 mL). Wells are too small to sonicate in — transfer is required, not optional. |
| `sonication_regimen` | energy × time | S2 Alt §5: microtip, 8 s at 40% power (example device Sonics VC-505) | plan | Verified only as the S2 example regimen. Power % is device-relative; absolute watts/frequency UNKNOWN (device_state). Any other device needs re-verification via the kill-control below. |
| `kill_control` | procedure | S2 Alt §5 note | plan | Mandatory companion: viable-count a planktonic culture before vs after the same sonication; if viability drops, adjust regimen. Without this control the CFU number is uninterpretable. |
| `detachment_check` (optional) | procedure | S2 Alt §5 note | run | CV-stain the sonicated carrier per Record C; absent CV ring ⇒ cells removed. Optional but recommended. |
| `alternative_regimens` | text | S5 | plan | Scraping + vortex + sonication combos; homogenizer-probe; reported exemplars (45 kHz/100%/30 s; ~20,500 rpm/~30 s) — all search-summary-only, NOT verified. Any non-S2 regimen needs its own kill-control + detachment check before planning. |
| `final_volume` | number, mL | S2 Alt §4 (2.0) | run | 2.0 mL total per tube; enters the CFU denominator (CFU per well = CFU/mL × 2.0 mL). |

### 6.4 Preconditions / completion checks

Preconditions: (P1) wells PBS-washed 6× per Alt §2; (P2) sonicator/homogenizer
available with a verified regimen + kill-control on file; (P3) sterile PBS and
labeled tubes ready. Completion: (C1) each well mapped 1:1 to a labeled tube
(C2) dispersal applied per regimen; (C3, operational) kill-control within
acceptance before plating the batch — block plating if it fails (deterministic
gate); (C4) suspension held for immediate dilution (hold-time limit UNKNOWN).

## 7. Record E — `OP-CFU-DILUTION-PLATE` (serial dilution + plating + count)

### 7.1 Purpose

Derive viable concentration (CFU/mL of suspension; back-calculated to CFU/well)
by diluting into the countable window and counting colonies. S3 formula:

> CFU/mL = colony count ÷ (total dilution of plated tube × volume plated in mL)

S3 worked example (p.4): 51 colonies from the 1/10² tube, 0.1 mL plated →
51 ÷ (1/10² × 1/10) = 5.1×10⁴ CFU/mL.

### 7.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Dispersed suspension (Record D output); sterile diluent + tubes; agar plates |
| Outputs | Colony-bearing plates (terminal: incubated, counted, biohazard-disposed) + `derived_CFU` dataset |
| Ports | `suspension_in` → `dilution_series_out[i]` (child IDs per tube) → `plate_out[j]` (child IDs per plate); `data_out` (plate → colony count + dilution + volume + computed CFU/mL) |
| Sample transition | `SUSPENSION → DILUTION_SERIES → COLONIES_ON_PLATE + DERIVED_CFU`. Dilution is conservative (cells neither created nor destroyed if technique is correct); plating + incubation is terminal for the aliquot. |
| Time scope | `per_assay` dilution-to-count block; colony incubation clock belongs to the organism/medium SOP (UNKNOWN here). |

### 7.3 Parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `dilution_factor_per_tube` | ratio | S3 pp.2–3: transfer ÷ (transfer + diluent) | plan | Per-tube factor; total dilution = product of tube + all previous tubes. No default step (1:10 typical in S3, SOP-owned). |
| `volume_plated` | number, mL | S3 p.4 example (0.1) | run | Stated per procedure; S3: "nothing to calculate … value will be stated". 0.1 mL is the worked-example value, not a global default. S4 reports ~0.1 mL spread-volume convention — NOT verified. |
| `countable_window` | integer range, colonies/plate | S3 Step 1 + Comments (30–300) | run | Count the plate with 30–300 colonies. <30: air-contaminant / sampling error dominates; >300: overlap/counting error (S3). S4-reported 25–250 / 20–300 variants NOT verified — keep 30–300. |
| `replicate_averaging` | rule | S3 Step 1 | run | Duplicate plates from one dilution at equal volume: average the counts. |
| `plating_method` | enum: `spread` \| `pour` | S4 (search-summary) | plan | Spread = surface colonies, retrievable for subculture; pour = surface + embedded (agar-locked). Choice affects colony position/counting, NOT the S3 formula. Method details (agar 15–20 mL/plate, plate drying, no-edge spreading, 10–20 min rest) are S4 search-summary-only. |
| `pipetting_discipline` | constraint | S3 Comments & Tips | run | Sterile pipettes; different pipette between dilutions (carryover otherwise); adequate agitation of each tube. Accuracy of the CFU claim rests on this (S3). Pipette calibration values: UNKNOWN (device_state). |
| `agar_medium` / `incubation` for colonies | text | — | plan | UNKNOWN (`method_dependent`): organism/medium/temperature/time per SOP. S5 reports R2A / 36±2 °C / 24 h for one coupon workflow — search-summary-only, do not generalize. |
| `labeling` | constraint | S3 via spread-plate summary (name, date, medium, organism, dilution) | run | Search-summary-only detail; keep as SOP labeling rule, not a verified verbatim. |

### 7.4 Preconditions / completion checks

Preconditions: (P1) uniform suspension (Record D complete; homogeneity assumed
only post-dispersal + agitation — never assume for undisrupted biofilm);
(P2) sterile diluent/tubes/pipettes sufficient for the series; (P3) agar plates
prepared per SOP. Completion: (C1) at least one plate inside 30–300 with its
tube dilution + plated volume recorded; (C2) duplicates averaged where present;
(C3) CFU/mL computed by the S3 formula with scientific-notation + raw-count
forms kept; (C4) plates + tubes disposed to biohazard (S3 Safety).

### 7.5 Instrument requirements

Calibrated pipettors covering the transfer + plating volumes (capability, not
model); vortex/agitation for tube mixing; incubator for colony growth at
SOP temperature; colony-counting aid (eye / counter — capability only).
Agar-plate preparation/drying hardware per S4: UNKNOWN (search-summary-only).

## 8. Controls (method/step companions; readout alone proves nothing)

| Readout / step | Required controls (same run) | Why (sourced) |
|---|---|---|
| Biofilm culture | Positive-control strain + negative control (medium-only or known non-former) on each plate; 4–8 replicates per condition | S1 Discussion: include positive and, if possible, negative control on each plate; S2 §2a/§9-note: replicate averaging + SD per strain |
| CV biomass | Same-solvent blank (S1 §3.4: 30% acetic acid); medium-only wells carried through wash + stain (background from media components, S1 §2.2) | Blank + background wells separate dye-in-solvent from adherent signal; subtraction downstream |
| CV detachment check (Record D) | Pre-dispersal stained reference or planktonic-only control | S2 Alt §5 note: lack of CV ring ⇒ removal; without a reference, "clean carrier" is ambiguous |
| Dispersal kill-control | Planktonic culture counted before vs after identical sonication | S2 Alt §5 note: adjust regimen if viability drops; gates the whole CFU batch |
| CFU count | Diluent-only / sterility control per SOP (practice, UNKNOWN detail here); duplicate plates per dilution for averaging (S3) | S3: average duplicates; contamination/low-count error rationale in Comments |

## 9. Readout meaning — what the numbers are and are NOT

- **CV OD (550 nm, or recorded 500–600 nm)** is light attenuation by
  solubilized dye from that well — nothing more. It is NOT cell number, NOT
  viability, NOT concentration. Under controlled same-organism conditions it is
  a semiquantitative proxy for total adherent biomass (S1 §3–4; S2 §9:
  average ± SD across replicates). Cross-organism or cross-solvent CV values
  are NOT commensurable (S2 Table 1B.1.1: solvents differ by organism).
- **CFU (colonies → CFU/mL → CFU/well)** is a viable-culturable count under
  that dispersal + medium + incubation. It is NOT total cells (misses dead and
  viable-but-nonculturable), NOT biomass (misses matrix), and one colony may
  arise from an incompletely dispersed aggregate (S2 Alt intro + dispersal
  rationale §6.1).
- **CV vs CFU direction of disagreement is informative, not contradictory:**
  high CV + low CFU ⇒ matrix-rich / largely nonviable adherent material or
  dispersal kill; low CV + high CFU ⇒ efficient dispersal of a thin biofilm or
  wash-stripped biomass with retained viable cells. Planning MUST keep both
  measurands with their own controls; converting one into the other is a
  modeling error.
- **Wash loss propagates asymmetrically:** over-vigorous washing lowers CV
  (biomass stripped) and lowers CFU (cells discarded with wash) — but a
  post-stain rinse deficit raises CV background without touching CFU. Never net
  these effects against each other.

## 10. Failure modes

| # | Record | Failure | Effect | Source / status |
|---|---|---|---|---|
| F-W1 | wash | Under-washing (too few cycles, cloudy water reused) | Stained planktonic/media background inflates CV; S2: replace water when cloudy/dark | S1 §2.2; S2 §4/§6, verified |
| F-W2 | wash | Post-stain rinse deficit | Non-specific CV retained; high background, false-high biomass | S2 §6 purpose-note, verified |
| F-W3 | wash | Over-vigorous shake-out / harsh handling | Detachment of weakly adherent biofilm → false-low CV and false-low CFU; the lossy-wash core risk | Derived from S1 "gently submerge" vs "shake out" + S2 "vigorously shake out"; magnitude UNKNOWN |
| F-W4 | wash | Wrong wash liquid (non-sterile water on the CFU path) | Contamination of the viable-count suspension | S2 Alt §2 (sterile PBS required), verified as requirement; contamination rate UNKNOWN |
| F-C1 | culture | Wrong plate (tissue-culture-treated) or dirty reused lid | Altered attachment; contamination | S2 Materials + lid note, verified; effect size UNKNOWN |
| F-C2 | culture | Medium/temp/time not empirically set for the organism | No biofilm, or planktonic-only growth mistaken for biofilm | S1 Discussion; S2 §2a-note, verified |
| F-CV1 | CV | CV staining before planktonic removal | Planktonic cells stained as false biomass | S1 §2.2 rationale, verified |
| F-CV2 | CV | Wrong solubilization solvent for the organism | Incomplete dye release → false-low OD | S2 §8 + Table 1B.1.1 (test empirically), verified |
| F-CV3 | CV | Volume-protocol mismatch (S1 125 µL vs S2 200 µL + transfer) | Concentration shift misread as biomass difference | S1 §3.1 vs S2 §8–9 conflict, verified |
| F-CV4 | CV | Reading at wrong λ / unstated λ | Non-comparable ODs across runs | S1 §3.4 vs S2 §9, verified |
| F-CV5 | CV | Photographing after solubilization (skipped dry-well photo) | Qualitative record unrecoverable | S1 §2.7 ordering, verified |
| F-D1 | dispersal | Sonicating inside the microtiter well | Unsuitable vessel; incomplete/uneven dispersal | S2 Alt §4 rationale, verified |
| F-D2 | dispersal | Unverified energy (wrong device, % power copied across models) | Kill (false-low CFU) or clumps (false-low CFU, high variance) | S2 Alt §5 kill-control requirement, verified; absolute-energy mapping UNKNOWN |
| F-D3 | dispersal | Skipping kill-control | Uninterpretable CFU; batch must be gated | S2 Alt §5 note, verified |
| F-P1 | CFU | Counting outside 30–300 | High error both directions (S3 Comments) | S3, verified |
| F-P2 | CFU | Same pipette across dilutions / no agitation | Carryover bias; non-uniform tubes | S3 Comments & Tips, verified |
| F-P3 | CFU | Recording only the derived CFU/mL, dropping raw count + dilution + volume | Recomputation / audit impossible | S3 formula needs all three inputs, verified |
| F-P4 | CFU | Pour/spread confusion (expecting surface colonies from a pour, or embedded-inclusive count from a spread) | Mislocated colonies; subculture failures | S4 search-summary-only — treat as caution, not verified fact |

## 11. Time scope and instrument summary

| Aspect | Culture | Wash | CV | Dispersal | CFU |
|---|---|---|---|---|---|
| Time scope | Hours (4–24 S1; ≤48 S2) organism block | Minutes, cycles-bound; no verified durations | Stain 10–15 min + solubilize 10–15 min, both RT; read same block | Seconds of energy (8 s S2) + handling; hold-time limit UNKNOWN | Dilution minutes + colony incubation per SOP (UNKNOWN here) |
| Instruments | Incubator (25–37 °C capability); pipettor; non-treated plates | Trays / multichannel pipettor; PBS where CFU path | Absorbance reader (500–600 nm); clear flat-bottom plates | Sonicator with microtip (capability: controllable ultrasonic energy; model as example only); tubes, scissors, PBS | Pipettors; vortex; incubator; agar plates; biohazard disposal |

## 12. UNKNOWNs (do not use for planning)

| Item | Status | Reason (`UNKNOWN` class) |
|---|---|---|
| Absolute sonication energy (W, kHz, tip amplitude) behind "40% power" | UNKNOWN | `device_state` — % is device-relative; S2 names VC-505 as example only |
| Hold time: dispersed suspension → first dilution; solubilized CV → read | UNKNOWN | `source_missing` — no verified window in S1–S3 |
| Colony incubation (medium, temp, time) for biofilm-derived CFU | UNKNOWN | `method_dependent` — organism/medium SOP owns it; S5's R2A/36±2 °C/24 h unverified here |
| Pour/spread hardware details (agar mL/plate, plate drying, spreader technique) | UNKNOWN | `source_missing` (S4 not directly read) |
| Reader linearity/accuracy/precision envelope for CV OD | UNKNOWN | `configuration_dependent` — no reader model read; S1/S2 give no photometric spec |
| Wash-loss magnitude per organism/vigor | UNKNOWN | `method_dependent` — requires same-organism detachment controls; never default to zero |
| Tissue-culture-treated vs non-treated quantitative attachment delta | UNKNOWN | `source_missing` — S2 requires non-treated but gives no delta |
| Biofilm-medium recipes beyond the S1 *P. aeruginosa* example | UNKNOWN | `method_dependent` — empirical per organism (S1 Discussion) |
| Anaerobic / microaerophilic static-biofilm adaptations | UNKNOWN | `source_missing` — S1/S2 assume aerobic static incubation |
| Contamination-rate / sterility-failure handling for the CFU path | UNKNOWN | `source_missing` — S3 Safety covers disposal only |
| Mature-biofilm / flow-cell parameters | UNKNOWN | `not_applicable` — explicitly out of scope (§1); S1 Abstract + S2 intro state the static limitation |

## 13. Verified vs NOT verified

Verified by direct reading (2026-11-05):

- S1: overnight → 1:100 → 100 µL/well → 4–8 replicates → 4–24 h 37 °C (*P. aeruginosa* example); dump + 2 tub rinses pre-stain; 125 µL 0.1% CV 10–15 min RT; 3–4 rinses + vigorous blot; dry hours–overnight; photo dry wells; 125 µL 30% acetic acid 10–15 min; transfer 125 µL; OD 550 nm vs acetic-acid blank; positive + ideally negative control per plate; medium/temp/time empirical per microbe; motile→walls/bottom, non-motile→bottom; static batch ≠ mature flow-cell biofilm.
- S2: non-treated 96-well + 70%-ethanol lid reuse; 3–5 mL stationary → 1:100 → 100 µL/well × 4 wells (small-N) or manifold from overnight plate (screens); trays with 1–2 inches water, replace when cloudy; 125 µL 0.1% CV 10 min; two-tray post-stain wash; invert/tap/air-dry, stable weeks RT; 200 µL 30% acetic acid (more efficient than 95% ethanol across microbes) 10–15 min; mix, transfer 125 µL to clear plate; OD 500–600 nm; Table 1B.1.1 solvents; CV stains matrix too (overestimates cells); Alt: 6× 100 µL PBS wash, cut wells, 100 µL PBS + well into 1.9 mL PBS (2.0 mL total), sonicate 8 s 40% microtip, viable counts; planktonic kill-control + optional CV detachment check.
- S3: CFU/mL = count ÷ (total dilution × volume plated); total dilution = product of tube factors; count the 30–300 plate; average duplicates; 0.1 mL worked example → 5.1×10⁴ CFU/mL; >300/<30 high error; sterile + fresh pipettes between dilutions; agitate tubes; biohazard disposal; no mouth pipetting.

NOT verified (do not use for planning until read directly):

- S4 spread/pour hardware numbers, S5 dispersal regimens (45 kHz/100%/30 s; ~20,500 rpm/30 s; R2A/36±2 °C/24 h) — search summaries only.
- Any plate-reader operating manual, sonicator manual, per-build calibration, per-lot CV/agar data sheets, laboratory SOP adaptations, colony-incubation SOPs, anaerobic adaptations.

## 14. What this record does not cover

- Flow-cell / chemostat / Calgary-device biofilm growth, MBEC values, or mature-biofilm architecture metrics.
- Air-liquid-interface microscopy, colony-biofilm death monitoring, or live/dead fluorescent staining (no `FLUO_IMAGE` parameters carried here).
- Conversion between CV OD and cell number, or between CFU and biomass — both conversions are refused by §9, not deferred.
- Kit-fixed wavelengths, volumes, or blanks: every CV number here is protocol-text, never a kit insert; kit-specific values need their own record.
- JSON cutover: all five IDs stay `research_draft`; UNKNOWNs are omitted (never zero-filled) if these records are ever schematized, and long rationale in §9 stays out of machine fields.

## 15. Review checklist (per `record-standard.md` §7)

- [x] Every numeric value carries a source locator, or is explicitly UNKNOWN (§12).
- [x] Method requirements (medium, solvent, regimen) separated from instrument specs (§11 names capabilities, never a required model).
- [x] Sample identity/state changes have ports + transitions (§3.2, §4.2, §5.2, §6.2, §7.2); wash loss modeled as a possible effect, not a global boolean.
- [x] Time scopes are per-record blocks, never inferred from an assay name (§11).
- [x] No implicit defaults: conflicts (S1 vs S2 volumes/cycles/λ) kept side-by-side with SOP-owns-the-choice notes.
- [x] Partial lists (Table 1B.1.1 solvents; S5 alternative regimens) labeled as partial/search-only, not complete capability.
- [x] Plan / actual / raw / derived separated (§9; CFU keeps raw count + inputs alongside the derived value).
- [x] No AI pre-review presented as human review; status stays `research_draft`.
- [x] No JSON emitted; cutover rule recorded in §14.
