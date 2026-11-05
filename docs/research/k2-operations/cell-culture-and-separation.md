# Cell culture and separation operations (research draft)

`review_status: research_draft` — NOT SourceChecked. No second-person review.
No `ReviewRecord` exists. Every numeric or behavioural claim below carries a
source locator; anything not verified from a fetched source is marked UNKNOWN.
No values are invented.

## 0. File identity and scope

- **File:** `docs/research/k2-operations/cell-culture-and-separation.md`
- **Covers four independent operation records** (separate IDs, separate boundaries):
  1. `cell-passaging-adherent` — adherent mammalian cell dissociation / passaging.
  2. `cell-counting-viability` — manual hemocytometer count + trypan blue viability.
  3. `centrifugation-cell-pelleting` — low-speed pelleting of mammalian cell suspensions.
  4. `cryopreservation-storage` — freezing + cryogenic storage of mammalian cell suspensions.
- **Out of scope for this file:** suspension-cell subculture specifics, insect-cell
  passaging variants, thaw/recovery as a standalone module (handoff only),
  assay plate controls, instrument model profiles, JSON machine drafts.
- **Reuse rule:** generic liquid handling, washing, and incubation mechanics are NOT
  re-taught here. Method-specific bindings and differences are recorded; the
  generic modules are referenced:
  - liquid transfer: `docs/research/k2-operations/liquid-transfer.md`
    (`liquid-transfer` module; aspirate/dispense/mix semantics).
  - washing/sampling: `docs/research/k2-operations/washing-and-sampling.md`.
  - incubation: `docs/research/k2-operations/inoculation-and-incubation.md`
    (incubator hold conditions; dissociation/flask holds are short RT/37 °C
    dwells, not growth incubation — see R1).

## 1. Sources (retrieved 2026-11-05)

| ID | Source | Locator used | Reading depth |
|---|---|---|---|
| T-ADH | Thermo Fisher, "Adherent Cell Culture Protocol" https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/subculturing-adherent-cells.html | Steps 1–12 (wash step 3, dissociation steps 5–8, spin step 9, count/reseed steps 10–12); Materials list; insect-cell section excluded | `agent-verified` (full page text fetched in-task) |
| T-TB | Thermo Fisher, "Trypan Blue Exclusion" https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/trypan-blue-exclusion.html | Steps 1–6 (0.4 %, 10 µL + 10 µL 1:1, load 10 µL, examine immediately, viability formula, ×10⁴ with dilution correction, ≥95 % healthy log-phase criterion) | `agent-verified` (full page text fetched in-task) |
| T-HEMO | Thermo Fisher, "Manually Counting Cells in a Hemocytometer" https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/counting-cells-in-a-hemacytometer.html | Grid description (9 squares, 1 mm² each; Neubauer 0.1 mm³ = 1×10⁻⁴ mL); protocol steps 1–4 (10 µL load, 10× objective, central grid zig-zag, ×10⁴, duplicates averaged); formulas C = N×D×10³, V = N/T×100 % | `agent-verified` (full page text fetched in-task) |
| T-FREEZE | Thermo Fisher, "Cell Freezing Protocols" https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/freezing-cells.html | "Medium for freezing cells" formulations; "How to freeze cells" steps 1–11 (characterize/log-phase/≥90 % viability/low passage; cold freezing medium 2–8 °C; spin 100–400 × g 5–10 min; ~1 °C/min; −80 °C overnight; gas-phase LN₂ below −135 °C; biohazard gas-phase rule) | `agent-verified` (full page text fetched in-task) |
| A-CRYO | ATCC, "Cryogenic Storage of Animal Cells" https://www.atcc.org/resources/technical-documents/cryogenic-storage-of-animal-cells | Supplies; Procedure §1 (log phase, medium change 24 h prior, contamination/identity check, antibiotic-free 1–2 wk note); §2 (T-75 volumes: rinse 5–10 mL CMF-PBS, 3–5 mL trypsin, 37 °C, +5 mL serum medium, ~100 × g 5 min soft pellet); §3 (5 % DMSO or 10 % glycerol; DMSO 5–10 %, glycerol 5–15 %, serum up to 90–95 %; 2–5×10⁶ viable cells/mL; 1.0–1.8 mL/vial); §4 (−1 to −3 °C/min; CoolCell/−70 to −90 °C; fast dry-ice transfer); §5 (below −130 °C; vapour-phase preferred; labelling/records/alarms/two locations); §6 (37 °C ~2 min thaw; 70 % ethanol; dilute to 4–5 mL flask / 10 mL dish; medium change 8–24 h; stepwise 1–2 mL additions) | `agent-verified` (full page text fetched in-task) |
| C-SUB | Corning, "Subculturing Monolayer Cell Culture Protocol" (Fishersci mirror PDF) | Search-result summary only: monolayer subculture incl. hemacytometer-with-vital-stain counting section | `search-summary-only` — NOT fetched in-task; no parameter is grounded on it |
| C-AN523 | Corning, CLS-AN-523 Cell Counter app note | Search-result summary only: TrypLE/Trypan-blue 1:1 auto-correction, live/dead gating | `search-summary-only` — NOT fetched in-task; automated-counter claims NOT recorded |

Corning sources are context only. All recorded parameters come from
T-ADH / T-TB / T-HEMO / T-FREEZE / A-CRYO.

---

## Record 1 — `cell-passaging-adherent` v1

### 1.1 Identity

- **catalog id:** `cell-passaging-adherent` v1.
- **action_type:** detach adherent mammalian cells from cultureware and re-plate
  an aliquot as a new culture (subculture/split).
- **Scope:** mammalian adherent monolayer dissociation with trypsin/TrypLE-type
  reagent + wash + quench + collect; T-ADH steps 1–8 + 12.
  The spin (step 9) and count (steps 10–11) inside the Thermo page are
  **handoffs** to `centrifugation-cell-pelleting` (R3) and
  `cell-counting-viability` (R2), not part of this module.
- **Exclusions:** suspension cells; insect-cell variants (T-ADH insect section
  explicitly excluded); thaw/recovery; seeding-density selection rationale;
  contamination testing; automation mappings.

### 1.2 Purpose

Convert one attached culture (vessel + spent medium + adherent monolayer) into a
single-cell suspension aliquot re-plated at a cell-line-specific seeding density,
so log-phase growth continues without contact-inhibition deterioration
(T-ADH steps 1, 12; Thermo maintaining-cultures guidance is background, not a
recorded parameter).

### 1.3 Ports / inputs / outputs

| Port | Kind | Required at | Notes + source |
|---|---|---|---|
| `source_culture` | input sample (attached cells + spent medium in vessel) | run | Must be log-phase, viability > 90 % (T-ADH step 1). |
| `source_vessel` | input container | run | Tissue-culture-treated flask/plate/dish (T-ADH Materials). |
| `wash_buffer` | input material | run | Ca/Mg-free balanced salt solution (T-ADH step 3). |
| `dissociation_reagent` | input material | run | Pre-warmed trypsin or TrypLE (T-ADH step 5). Reagent choice is cell-line-dependent. |
| `quench_medium` | input material | run | Pre-warmed complete growth medium, 2 volumes of dissociation reagent (T-ADH step 8). |
| `destination_vessels` | input containers (pre-existing) | run | Fresh vessels with medium; caps loosened unless vented flasks (T-ADH step 12). |
| `detached_suspension` | output sample | run | Quenched single-cell suspension handed to R3/R2/R1-reseed. |
| `new_culture` | output sample | run | Diluted suspension re-plated at line-specific seeding density (T-ADH step 12). |
| `waste` | output (implicit) | conditional | Spent medium (step 2) + wash (step 4). Exact waste path is protocol-level — UNKNOWN to this module. |
| `instrument.incubator` | input instrument | run | 37 °C humidified 5 % CO₂ for mammalian hold (T-ADH Materials). |
| `instrument.microscope` | input instrument | run | Inverted microscope for detachment check (T-ADH step 7). |
| `instrument.bsc` | input instrument | run | Laminar flow hood + sterile technique (T-ADH pre-step). |

Generic aspirate/dispense/mix mechanics: see `liquid-transfer`; wash mechanics:
see `washing-and-sampling`. This record binds only the volumes/sequence below.

### 1.4 Sample transitions

| # | From → to | Effect | Condition / source |
|---|---|---|---|
| T1 | `source_culture --spent-medium--> source_culture(rinsed)` | Spent medium removed and discarded | Step 2. |
| T2 | `source_culture --wash--> source_culture(washed)` | Serum/Ca/Mg traces removed that would inhibit dissociation | Step 3; gentle addition to side opposite cell layer + rocking. |
| T3 | `source_culture(washed) --dissociate--> detached_suspension(crude)` | Attached → detached; ≥ 90 % detached threshold | Steps 5–7. |
| T4 | `detached_suspension(crude) --quench--> detached_suspension` | Enzyme action stopped by serum/complete medium; dispersed by pipetting over layer | Step 8. |
| T5 | `detached_suspension --split--> new_culture` | Aliquot diluted to line-specific seeding density and re-plated | Step 12. Handoff volumes/concentrations from R2. |
| T6 | `source_culture --residue--> waste` | Spent medium + used wash leave the sample path | Steps 2, 4. |

### 1.5 Parameters (method-specific bindings only)

| Parameter | Type | Unit | Required at | Source class | Constraints (sourced only) + evidence |
|---|---|---|---|---|---|
| `prestatus.viability_min` | float | % | run (gate) | manufacturer_protocol | > 90 % at subculture; log phase. T-ADH step 1. |
| `wash_volume_per_area` | float | mL / 10 cm² | run | manufacturer_protocol | ≈ 2 per 10 cm²; Ca/Mg-free; gentle side-addition + rocking. T-ADH step 3. |
| `dissociation_volume_per_area` | float | mL / 10 cm² | run | manufacturer_protocol | ≈ 0.5 per 10 cm², enough to cover layer; pre-warmed; rock to cover. T-ADH step 5. |
| `dissociation_reagent` | enum | — | run | manufacturer_protocol | trypsin or TrypLE. T-ADH step 5. Concentration/grade: method_dependent UNKNOWN (not stated on this page). |
| `dissociation_dwell` | float | min | run | manufacturer_protocol | ≈ 2 at room temperature; **actual time varies with cell line**. T-ADH step 6. |
| `detachment_check_interval` | float | s | conditional | manufacturer_protocol | Recheck every 30 s if < 90 % detached; may extend + tap vessel. T-ADH step 7. |
| `detachment_threshold` | float | % | run (gate) | manufacturer_protocol | Proceed when ≥ 90 % detached; tilt vessel minimal time to drain. T-ADH steps 7–8. |
| `quench_volume` | float (derived) | mL | run | manufacturer_protocol | 2 volumes of complete pre-warmed medium relative to dissociation reagent volume; disperse by pipetting over layer. T-ADH step 8. |
| `reseed_seeding_density` | float | cells / area or / mL | run | method_dependent | Dilute to line-recommended density. T-ADH step 12. Numeric value UNKNOWN (cell-line/product-insert-dependent). |
| `vessel_venting` | enum | — | run | manufacturer_protocol | Loosen flask caps unless vented/gas-permeable flasks. T-ADH step 12. |

No implicit defaults: dwell time, reagent concentration, seeding density MUST be
supplied per cell line; the ≈ values above are Thermo example bindings, not
universal defaults.

### 1.6 Time scope

`once` per subculture event. The dissociation dwell (≈ 2 min + 30 s rechecks) is
a bounded step dwell, not a growth incubation; growth incubation of the new
culture belongs to `inoculation-and-incubation`. Repeated passages are separate
invocations (frequency cell-line-dependent, UNKNOWN here).

### 1.7 Preconditions

- Solutions/equipment contacting cells are sterile; work in laminar flow hood
  with sterile technique (T-ADH pre-step).
- Culture in log phase, viability > 90 % (T-ADH step 1).
- Complete growth medium and dissociation reagent pre-warmed (37 °C medium per
  Materials; pre-warmed reagent per step 5). Exact pre-warm method/time UNKNOWN.
- Destination vessels prepared before harvest (implied by step 12 ordering).

### 1.8 Completion checks

| Check | Type | Blocks? | Source |
|---|---|---|---|
| ≥ 90 % detached under microscope before quench | operational | yes — do not quench early | T-ADH step 7–8 |
| Suspension dispersed (pipetted over layer), no large clumps carried into split | operational | yes | T-ADH step 8 |
| New vessels in 37 °C 5 % CO₂ incubator, caps vented correctly | operational | yes | T-ADH step 12 + Materials |
| Viability/count obtained via R2 before final dilution | operational | yes (handoff) | T-ADH steps 10–11 |

### 1.9 Instrument requirements (capabilities, not models)

- Aseptic work capability (laminar flow hood or equivalent) — sterile-contact
  condition (T-ADH pre-step).
- Humidified 37 °C / 5 % CO₂ incubation for mammalian cultures (T-ADH Materials).
  Non-mammalian setpoints excluded.
- Inverted phase-contrast microscopy for detachment staging (T-ADH step 7).
- Piston pipettes + sterile tips/conical tubes (mechanics in `liquid-transfer`;
  15 mL conical tube named in T-ADH step 9 handoff).
- Automated counter named as alternative to hemocytometer (T-ADH step 11) is an
  allowed variant, not a requirement; its performance envelope is UNKNOWN here.

### 1.10 Controls

No plate/biological assay controls belong to this operation. Operational
controls: microscopic detachment staging (step 7); pre-passage viability gate
(step 1); post-detachment count/viability via R2 (steps 10–11). Reference only;
no group IDs recorded.

### 1.11 Failure modes

| Phenomenon | Cause / impact | Handling + record (sourced where stated) |
|---|---|---|
| Incomplete detachment (< 90 %) | Under-dwell / line-dependent adhesion | Extend minutes, recheck every 30 s, tap vessel (T-ADH step 7). Record dwell actually used. |
| Over-digestion damage | Excess dwell (implied by "varies with cell line", step 6) | Stop at ≥ 90 %; quench immediately. Quantitative over-trypsinization limit UNKNOWN (`method_dependent`). |
| Wash-disturbed monolayer | Wash added directly onto layer | Add to side opposite layer, rock gently (T-ADH step 3). |
| Poor post-split growth | Split past confluence / wrong density / deviated medium (Thermo maintaining-cultures background) | Follow product-insert medium/density; exact rescue UNKNOWN. |
| Contamination | Non-sterile contact | Sterile solutions/equipment + hood (T-ADH pre-step). |

### 1.12 Readout

`not_implied`. This module produces no measurement; concentration/viability come
from R2. Do not encode a count result here.

### 1.13 UNKNOWNs

| UNKNOWN | Class | Why / who confirms |
|---|---|---|
| Trypsin/TrypLE concentration, EDTA inclusion, phenol-red status | `method_dependent` | Not on T-ADH page; confirm per cell-line product insert. |
| Seeding density numeric | `method_dependent` | Line-specific; confirm per product insert. |
| Exact dissociation dwell per line | `method_dependent` | "Varies with cell line" (step 6). |
| Pre-warm method/time for reagents | `source_missing` | "Pre-warmed" stated, no method/time. |
| Flask-size scaling table | `source_missing` | Page gives per-10 cm² bindings only. |
| Insect/suspension variants | `not_applicable` (excluded) | Deliberately out of scope. |

---

## Record 2 — `cell-counting-viability` v1

### 2.1 Identity

- **catalog id:** `cell-counting-viability` v1.
- **action_type:** determine total/viable cell concentration and percent viability
  from a cell-suspension aliquot by manual hemocytometer count with trypan blue
  dye exclusion.
- **Scope:** manual Neubauer/Improved-Neubauer hemocytometer + 0.4 % trypan blue,
  T-TB + T-HEMO. Automated counters are a noted variant, not recorded here.
- **Exclusions:** dissociation/harvest mechanics (R1); pelleting (R3);
  automated-counter algorithms/gating (C-AN523, `search-summary-only`);
  downstream seeding math beyond the two stated formulas.

### 2.2 Purpose

Produce a derived concentration (cells/mL) and viability (%) for one suspension
sample so passaging (R1), freezing (R4), or assay plating can dose correctly;
live cells exclude dye, dead cells with compromised membranes stain blue (T-TB
principle).

### 2.3 Ports

| Port | Kind | Required at | Notes + source |
|---|---|---|---|
| `aliquot` | input sample | run | Suspension aliquot; resuspend gently but thoroughly before sampling (T-HEMO step 2). |
| `stain` | input material | run | Trypan blue 0.4 % (T-TB step 2). |
| `stained_mix` | intermediate sample | run | 1:1 mix, examined immediately (T-TB steps 2–3). |
| `chamber` | input instrument consumable | run | Hemocytometer + coverslip, cleaned with alcohol, dried, fixed (T-HEMO step 1). |
| `instrument.microscope` | input instrument | run | Inverted microscope, 10× objective, phase contrast (T-HEMO step 3). |
| `report.concentration` | output readout (derived) | run | Cells/mL via ×10⁴ or C = N×D×10³ (T-HEMO; T-TB step 6). |
| `report.viability_pct` | output readout (derived) | run | [1 − blue/total] × 100 (T-TB step 5); or V = N/T × 100 % (T-HEMO). |

### 2.4 Sample transitions

| # | From → to | Effect | Source |
|---|---|---|---|
| T1 | `aliquot + stain --mix--> stained_mix` | 1:1 stained mixture; dilution factor 2 | T-TB step 2 |
| T2 | `stained_mix --load--> chamber` | 10 µL loaded, not overfilled | T-HEMO step 2; T-TB step 3 |
| T3 | `chamber --observe--> report.*` | Counts → derived concentration + viability; sample consumed as measurement | T-HEMO step 4; T-TB steps 4–6 |

Counting is destructive to the aliquot only; the source suspension is unchanged
except for the removed aliquot volume.

### 2.5 Parameters

| Parameter | Type | Unit | Required at | Source class | Constraints + evidence |
|---|---|---|---|---|---|
| `stain_concentration` | float | % (w/v) | run | manufacturer_protocol | 0.4 % trypan blue. T-TB step 2. |
| `stain_ratio` | ratio | — | run | manufacturer_protocol | 1:1 — 10 µL stain : 10 µL cells. T-TB step 2. |
| `load_volume` | float | µL | run | manufacturer_protocol | 10 µL into chamber; do not overfill. T-HEMO step 2; T-TB step 3. |
| `read_timing` | enum | — | run | manufacturer_protocol | Examine immediately under microscope at low magnification. T-TB step 3. Max delay UNKNOWN. |
| `objective` | float | × | run | manufacturer_protocol | 10×, phase contrast to distinguish cells. T-HEMO step 3. |
| `count_pattern` | enum | — | run | manufacturer_protocol | Central large gridded square (1 mm²), zig-zag down each row; duplicates averaged. T-HEMO step 4. |
| `chamber_volume_per_square` | float | mm³ (mL equiv.) | design (fixed) | manufacturer_protocol | Standard Neubauer square 0.1 mm³ = 1×10⁻⁴ mL → ×10⁴ (×10,000) factor. T-HEMO "Why multiply by 10,000". Non-Neubauer grids UNKNOWN. |
| `dilution_factor_D` | float | — | run | manufacturer_protocol | Correct for stain (≥ 2 for 1:1) plus any prior dilution. T-TB step 6 ("remember to correct"). |
| `healthy_viability_gate` | float | % | advisory | manufacturer_protocol | ≥ 95 % for healthy log-phase cultures (T-TB step 4); > 90 % gate at subculture (T-ADH step 1). Advisory, not a block. |

Formulas (recorded verbatim, not derived): viability % = [1.00 − (blue ÷ total)]
× 100 (T-TB step 5); viable cells × 10⁴ = cells/mL culture after dilution
correction (T-TB step 6); C = N × D × 10³ with N over 10 subgrids
(1.0 mm³), V = N/T × 100 % (T-HEMO calc section).

### 2.6 Time scope

`once` per sampled aliquot. Duplicate loads are replicate reads within one
invocation (T-HEMO step 4), not separate timepoints. Time-series viability is a
protocol-level composition, UNKNOWN here.

### 2.7 Preconditions

- Single-cell suspension available (adherent cells already dissociated per R1;
  T-HEMO notes dissociation/collection as prerequisite steps).
- Chamber + coverslip cleaned with alcohol, dried, fixed (T-HEMO step 1).
- Suspension resuspended gently but thoroughly immediately before loading
  (T-HEMO step 2).

### 2.8 Completion checks

| Check | Type | Blocks? | Source |
|---|---|---|---|
| Duplicate samples prepared and averaged | operational | yes | T-HEMO step 4 |
| Blue + total counted; viability computed | deterministic | yes | T-TB steps 4–5 |
| Dilution factor applied to cells/mL | deterministic | yes | T-TB step 6 |
| ≥ 95 % advisory for healthy log-phase noted | advisory | no | T-TB step 4 |

### 2.9 Instrument requirements

- Hemocytometer (Neubauer/Improved Neubauer reference grid; T-HEMO). Grid
  variant beyond Neubauer: UNKNOWN.
- Coverslip + alcohol cleaning (T-HEMO step 1).
- Inverted microscope, 10× objective, phase contrast (T-HEMO step 3).
- Micropipette for 10 µL loads (mechanics in `liquid-transfer`).
- Automated counter (e.g. Countess) is an allowed alternative workflow
  (T-ADH step 11; T-HEMO "Additional protocols") with UNKNOWN performance
  envelope — not recorded.

### 2.10 Controls

Duplicate-sample averaging is the built-in replicate control (T-HEMO step 4).
No biological assay controls belong here. Stain-only/blank controls are not
stated in T-TB/T-HEMO — UNKNOWN (`source_missing`).

### 2.11 Failure modes

| Phenomenon | Cause / impact | Handling + record |
|---|---|---|
| Overfilled chamber | Volume ≠ calibrated 0.1 mm³/square → biased ×10⁴ | Reload 10 µL, do not overfill (T-HEMO step 2). |
| Delayed read | Uptake/settling drift (mechanism not quantified in source) | Examine immediately (T-TB step 3). Max delay UNKNOWN. |
| Clumped suspension | Under-dispersed pellet → uncountable grid | Gently but thoroughly resuspend before loading (T-HEMO step 2). |
| Low viability (< 90–95 %) | Unhealthy/over-confluent culture | Do not passage/freeze on a bad gate; see R1/R4 gates. Rescue UNKNOWN. |

### 2.12 Readout

- `raw`: blue count, total/viable count per grid(s) (T-TB step 4; T-HEMO step 4).
- `derived`: cells/mL (×10⁴ with dilution correction; or C = N×D×10³),
  viability % ([1 − blue/total] × 100; or V = N/T × 100 %).
- No instrument file output is implied for the manual path.

### 2.13 Readout limits

Manual counting is time-consuming and subject to human error; automated counters
are described as the emerging standard (T-HEMO "Additional cell counting
protocols"). Quantitative error bounds (CV, LoD, counting range) are UNKNOWN
(`source_missing` — not stated on T-TB/T-HEMO pages).

### 2.14 UNKNOWNs

| UNKNOWN | Class | Why / who confirms |
|---|---|---|
| Max stain-to-read delay | `source_missing` | "Immediately" only; confirm per SOP/counter manual. |
| Validated counting range / error bounds | `source_missing` | Not on fetched pages. |
| Non-Neubauer grid volumes | `configuration_dependent` | Page notes other grids exist with varying volumes. |
| Cell-type exclusions (clumping lines) | `method_dependent` | Not enumerated. |
| Automated-counter performance | `configuration_dependent` | C-AN523 is `search-summary-only`; counter model manual needed. |

---

## Record 3 — `centrifugation-cell-pelleting` v1

### 3.1 Identity

- **catalog id:** `centrifugation-cell-pelleting` v1.
- **action_type:** pellet a mammalian cell suspension at low RCF and separate
  pellet from supernatant for resuspension in fresh medium.
- **Scope:** the spin step as invoked from passaging (T-ADH step 9) and freezing
  workflows (T-FREEZE step 7; A-CRYO §2.5). This record covers ONLY the spin +
  supernatant removal; harvest, wash, quench, count, freeze belong to R1/R2/R4.
- **Exclusions:** lysate clarification spins, rpm↔g conversion tables, rotor
  models, temperature control, braking profiles.

### 3.2 Purpose

Concentrate detached cells into a soft pellet so dissociation reagent / spent or
freeze-depleted medium (supernatant) can be removed and replaced with a minimal
volume of fresh/complete medium without vigorous handling that harms recovery
(T-ADH step 9–10; T-FREEZE step 7; A-CRYO §2.5 + §3.1).

### 3.3 Ports

| Port | Kind | Required at | Notes + source |
|---|---|---|---|
| `cell_suspension` | input sample | run | Quenched/detached suspension in 15 mL conical tube (T-ADH step 9; A-CRYO §2.5). |
| `tube` | input container | run | Sterile 15 mL conical (T-ADH step 9); T-FREEZE allows 15 or 50 mL. |
| `instrument.centrifuge` | input instrument | run | Low-speed cell-capable centrifuge with RCF control (capability; model UNKNOWN). |
| `pellet` | output sample | run | Soft cell pellet (A-CRYO §2.5 "soft cell pellet"). |
| `supernatant` | output (to waste or decant) | run | Dissociation/spent medium withdrawn aseptically to smallest volume without disturbing pellet, then discarded (T-FREEZE step 7). |

### 3.4 Sample transitions

| # | From → to | Effect | Source |
|---|---|---|---|
| T1 | `cell_suspension --spin--> pellet + supernatant` | One suspension resolves into pellet + liquid phases | T-ADH step 9; T-FREEZE step 7; A-CRYO §2.5 |
| T2 | `supernatant --> waste` | Supernatant withdrawn/discarded aseptically | T-FREEZE step 7 ("withdraw … discard"); A-CRYO §3.1 ("remove and discard the supernatant") |
| T3 | `pellet + fresh_medium --resuspend--> washed_suspension` | Pellet resuspended in minimal/cold fresh medium (handoff to R1-step-10 / R2 / R4-step-8) | T-ADH step 10; T-FREEZE step 8; resuspension mechanics via `liquid-transfer` |

### 3.5 Parameters

| Parameter | Type | Unit | Required at | Source class | Constraints + evidence |
|---|---|---|---|---|---|
| `rcf` | float (range) | × g | run | manufacturer_protocol | **Sources disagree — both retained, no averaging.** T-ADH step 9: 200 × g. T-FREEZE step 7: ≈ 100–400 × g. A-CRYO §2.5: ≈ 100 × g. All state speed/time vary by cell type. Binding MUST be chosen per cell type; no universal default. |
| `duration` | float (range) | min | run | manufacturer_protocol | T-ADH step 9: 5–10. T-FREEZE step 7: 5–10. A-CRYO §2.5: 5. Same method-dependent rule as `rcf`. |
| `tube_size` | enum | — | run | manufacturer_protocol | 15 mL conical (T-ADH step 9; A-CRYO §2); 15 or 50 mL (T-FREEZE Materials). |
| `supernatant_residual` | enum | — | run | manufacturer_protocol | Withdraw to smallest volume without disturbing pellet (T-FREEZE step 7). Numeric residual UNKNOWN. |

`rpm`, rotor radius, temperature, brake/decel profile: UNKNOWN (not stated for
these cell spins; do not convert RCF↔rpm without rotor data).

### 3.6 Time scope

`once` per spin invocation. Repeat spins are separate invocations. Storage or
incubation while waiting is out of scope.

### 3.7 Preconditions

- Cells already detached/quenched and transferred to tube (T-ADH step 9 ordering).
- Sterile tube + aseptic handling for supernatant withdrawal (T-FREEZE step 7).
- RCF/time selected for the cell type (all three sources: "vary / varies
  depending on the cell type").

### 3.8 Completion checks

| Check | Type | Blocks? | Source |
|---|---|---|---|
| Visible soft pellet; supernatant separable | operational | yes | A-CRYO §2.5 ("soft cell pellet") |
| Supernatant removed to minimal residual without disturbing pellet | operational | yes | T-FREEZE step 7 |
| Pellet promptly resuspended in fresh/cold medium (no dry hold stated) | operational | yes (handoff) | T-ADH step 10; T-FREEZE step 8 |

### 3.9 Instrument requirements

- Low-speed centrifuge capable of 100–400 × g with timed holds and 15/50 mL
  conical carriers (range covers all three sourced bindings; NOT a claim that
  every unit spans it — verify per unit).
- No rotor type, refrigeration, or brake claim is recorded (UNKNOWN —
  `device_state` / `configuration_dependent`).

### 3.10 Controls

None. Pellet presence + clear supernatant is the operational check (§3.8), not
an assay control.

### 3.11 Failure modes

| Phenomenon | Cause / impact | Handling + record |
|---|---|---|
| No/loose pellet | RCF/time too low for line; disturbed during handling | Re-spin per line-validated binding; record actual RCF/time. Quantitative rescue UNKNOWN. |
| Pellet loss on decant | Over-aspiration / vigorous handling | Withdraw to smallest volume without disturbing pellet (T-FREEZE step 7); pipette, don't pour. |
| Clumps / DNA strings on resuspension | Vigorous pipetting during harvest (ATCC monolayer-guide background, consistent with A-CRYO "be as gentle as possible" §2) | Gentle resuspension; record handling. Enzymatic rescue UNKNOWN. |
| Cross-line over/under-spin | Applying one source's value as universal (200 × g vs 100 × g vs 100–400 × g) | Treat as method-dependent; confirm per line. Conflict retained, not averaged. |

### 3.12 Readout

`not_implied`. No measurement is produced; downstream concentration/viability via R2.

### 3.13 UNKNOWNs

| UNKNOWN | Class | Why / who confirms |
|---|---|---|
| Per-line RCF/time table | `method_dependent` | All sources defer to cell type; confirm per product insert. |
| Rotor type / radius / rpm equivalent | `configuration_dependent` + `device_state` | Not stated; confirm per centrifuge manual + installed rotor. |
| Temperature / brake settings | `device_state` | Not stated for these spins. |
| Max hold time as pellet | `source_missing` | Prompt resuspension implied, no time given. |

---

## Record 4 — `cryopreservation-storage` v1

### 4.1 Identity

- **catalog id:** `cryopreservation-storage` v1.
- **action_type:** formulate a concentrated viable-cell aliquot in cryoprotective
  medium, freeze at a controlled rate, and hold below vitrification-relevant
  cryogenic temperature for indefinite storage.
- **Scope:** mammalian cultures; freeze medium formulation + aliquoting +
  controlled-rate freeze + LN₂ storage (T-FREEZE steps 1–11; A-CRYO §§1–5).
  Thaw/recovery (A-CRYO §6; T-FREEZE "Next protocol: Thawing Cells") is a
  **handoff summary only**, not a recorded thaw module.
- **Exclusions:** stem-cell-specific formulations beyond what T-FREEZE names;
  program-freezer programs; inventory/location bookkeeping schema; shipping.

### 4.2 Purpose

Arrest biological aging/genetic drift by holding living cells below −130 °C so a
seed/working stock survives indefinitely and can re-establish culture later
(T-FREEZE "Why freeze"; A-CRYO intro + §5).

### 4.3 Ports

| Port | Kind | Required at | Notes + source |
|---|---|---|---|
| `culture` | input sample (log-phase cells) | run | Characterized, contamination-checked, log phase, ≥ 90 % viability, lowest possible passage (T-FREEZE step 1; A-CRYO §1). |
| `freeze_medium` | input material | run | Cold cryoprotective medium (T-FREEZE step 2: store 2–8 °C until use). Formulations §4.5. |
| `cryovial` | input container | run | Sterile cryogenic storage vials (T-FREEZE Materials); 2 mL screw-capped plastic vials for cryogenic use (A-CRYO Supplies). |
| `controlled_rate_unit` | input instrument | run | Controlled-rate freezer or isopropanol chamber / CoolCell-type unit (T-FREEZE step 10; A-CRYO §4). |
| `ln2_store` | input instrument (facility) | storage | LN₂ container; gas phase above LN₂ below −135 °C (T-FREEZE step 11) / below −130 °C optimum (A-CRYO §5). |
| `frozen_stock[]` | output samples | run | 1.0–1.8 mL aliquots per vial (A-CRYO §3.2) at line-specific viable density. |
| `waste` | output (implicit) | conditional | Depleted supernatant from pre-freeze spin (T-FREEZE step 7). |

Detachment/count/spin mechanics inside the freeze workflow are handoffs to
R1 (`following the procedure used during subculture`, T-FREEZE step 3), R2
(T-FREEZE step 5), R3 (T-FREEZE step 7).

### 4.4 Sample transitions

| # | From → to | Effect | Source |
|---|---|---|---|
| T1 | `culture --harvest--> cell_suspension` | Adherent cells detached gently; suspension cells collected (handoff R1) | T-FREEZE steps 3–4; A-CRYO §2 ("be as gentle as possible") |
| T2 | `cell_suspension --spin/remove--> pellet` | Depleted medium removed (handoff R3) | T-FREEZE step 7; A-CRYO §3.1 |
| T3 | `pellet + freeze_medium --resuspend--> frozen_stock(liquid)` | Pellet at target viable density in cold medium; mixed often to stay homogeneous while aliquoting | T-FREEZE steps 6, 8–9; A-CRYO §3.1–3.2 |
| T4 | `frozen_stock(liquid) --controlled-freeze--> frozen_stock(frozen)` | ≈ −1 °C/min to −80 °C overnight, then LN₂ | T-FREEZE step 10; A-CRYO §4 |
| T5 | `frozen_stock(frozen) --store--> frozen_stock(archived)` | Indefinite hold below −130 °C (−135 °C Thermo), gas phase; no identity change implied | T-FREEZE step 11; A-CRYO §5 |
| T6 | `frozen_stock(archived) --thaw--> recovery_culture` | **Handoff only:** rapid 37 °C ~2 min thaw, ethanol decontamination, dilution into complete medium; medium change 8–24 h; stepwise additions for sensitive lines | A-CRYO §6; T-FREEZE next-protocol pointer |

### 4.5 Parameters

| Parameter | Type | Unit | Required at | Source class | Constraints + evidence |
|---|---|---|---|---|---|
| `prestatus.viability_min` | float | % | run (gate) | manufacturer_protocol | ≥ 90 % at freeze; log phase; low passage; contamination-checked/characterized. T-FREEZE step 1. A-CRYO §1 adds: medium changed 24 h prior (ideal); mycoplasma/identity check recommended; antibiotic-free 1–2 wk before freezing uncovers cryptic contaminants. |
| `freeze_medium_base` | enum/text | — | run | manufacturer_protocol | Follow cell-line/product-insert instructions (T-FREEZE "Medium for freezing cells" preamble). Sourced options (NOT defaults — pick per line): (a) complete medium + 10 % glycerol; (b) complete medium + 10 % DMSO; (c) 50 % conditioned + 50 % fresh with 10 % glycerol or DMSO; serum-free: 50/50 conditioned/fresh serum-free + 7.5 % DMSO, or fresh serum-free + 7.5 % DMSO + 10 % culture-grade BSA; or ready-to-use Recovery / Synth-a-Freeze media (T-FREEZE formulations). ATCC: complete medium + 5 % DMSO **or** 10 % glycerol (A-CRYO Supplies + §3); DMSO 5–10 %, glycerol 5–15 %, serum up to 90–95 % for difficult lines (A-CRYO §3). Conflict retained: ATCC typical 5 % DMSO vs Thermo 10 % DMSO — line-dependent, do not average. |
| `freeze_medium_storage` | float (range) | °C | prep | manufacturer_protocol | Prepared medium held at 2–8 °C until use. T-FREEZE step 2. |
| `freeze_density` | float (range) | viable cells / mL | run | manufacturer_protocol | A-CRYO §3.1: 2–5×10⁶ viable cells/mL. Thermo: "recommended viable cell density for the specific cell type" (T-FREEZE step 8) — numeric UNKNOWN beyond ATCC range; confirm per insert. |
| `aliquot_volume` | float (range) | mL / vial | run | manufacturer_protocol | 1.0–1.8 per vial (A-CRYO §3.2). Vial size 2 mL screw-capped cryogenic (A-CRYO Supplies). Fill/headspace rule UNKNOWN. |
| `prefreeze_spin` | handoff | × g / min | run | manufacturer_protocol | ≈ 100–400 × g, 5–10 min, cell-type-dependent (T-FREEZE step 7); ≈ 100 × g 5 min soft pellet (A-CRYO §2.5). Executed as R3. |
| `cooling_rate` | float (range) | °C / min | run | manufacturer_protocol | T-FREEZE step 10: ≈ 1/min. A-CRYO §4: −1 to −3/min for most animal cultures. Programmable freezer preferred; CoolCell/isopropanol chamber at −70 to −90 °C overnight acceptable (A-CRYO §4; T-FREEZE step 10). Polystyrene-box substitute works for many lines but is uncontrolled/unreproducible — not for valuable cultures (A-CRYO §4.1). |
| `interim_hold` | text | — | run | manufacturer_protocol | −80 °C overnight in chamber (T-FREEZE step 10); −70 to −90 °C (A-CRYO §4). Duration beyond "overnight" UNKNOWN. |
| `storage_temperature` | float (limit) | °C | storage | manufacturer_protocol | Below −130 °C optimum (A-CRYO §5); below −135 °C gas phase (T-FREEZE step 11). Higher temps may survive short-term but viability declines (A-CRYO §5). |
| `storage_phase` | enum | — | storage | manufacturer_protocol | Gas phase above LN₂ (T-FREEZE step 11; A-CRYO §5 vapour-phase preferred for safety). Biohazardous materials MUST be gas-phase (T-FREEZE Safety Note). Liquid-phase leak→explosion hazard on thaw (A-CRYO §6). |
| `transfer_practice` | enum | — | run | manufacturer_protocol | Fast efficient transfer to final store; dry-ice staging if delayed; avoid transient warming — "major cause of variation" (A-CRYO §4.2). Time limit UNKNOWN. |
| `vial_labelling` | text | — | run | manufacturer_protocol | At minimum cell-line name + date; system must suit permanent cryogenic storage (A-CRYO §3.2 + §5). Label technology UNKNOWN. |
| `thaw_handoff` | handoff summary | — | recovery | manufacturer_protocol | Rapid ~2 min 37 °C agitated thaw, cap/O-ring out of water; 70 % ethanol decontamination; aseptic dilution (flask 4–5 mL / dish 10 mL per A-CRYO §6.2); pre-equilibrate medium pH ≥ 15 min; medium change 8–24 h (cryoprotectant removal) except sensitive lines spun out; stepwise 1–2 mL additions over 10–20 min at 20–37 °C for osmotic-shock-prone lines (A-CRYO §6). Full thaw module NOT recorded. |

DMSO handling: powerful solvent, skin penetration; avoid contact, dispose per
local rules; some commercial lots may contain toxics — do not use unqualified
DMSO for freezing (A-CRYO §3 WARNING + T-FREEZE Safety Note). Autoclaving,
filtration, or grade requirements beyond "cell culture-grade albumin/BSA"
(T-FREEZE; A-CRYO §3) UNKNOWN.

### 4.6 Time scope

Freezing is `once` per banked lot; storage is continuous/indefinite below limit
(T-FREEZE "indefinite period"; A-CRYO §5). "Overnight" interim hold + "8–24 h"
post-thaw medium change are bounded dwells, not assay timepoints. Re-banking
from working stocks is a new invocation (T-FREEZE seed/working-stock note).

### 4.7 Preconditions

- Log-phase, characterized, contamination-screened culture; medium changed 24 h
  prior ideally (T-FREEZE step 1; A-CRYO §1).
- Cold freeze medium ready (2–8 °C); sterile cryovials labelled; count/spin
  capacity available (T-FREEZE Materials + steps 2, 5–7).
- Controlled-rate unit qualified or chamber + −70/−80/−90 °C freezer available;
  LN₂ store with alarm/monitoring and (for valuable stocks) second location
  (T-FREEZE steps 10–11; A-CRYO §§4–5).

### 4.8 Completion checks

| Check | Type | Blocks? | Source |
|---|---|---|---|
| Pre-freeze viability ≥ 90 %, count recorded, density math done | deterministic | yes | T-FREEZE steps 1, 5–6; A-CRYO §2.5 (count while tube spins) |
| Homogeneous suspension maintained during aliquoting (gentle frequent mixing) | operational | yes | T-FREEZE step 9 |
| Controlled-rate profile executed; fast transfer to LN₂ | operational | yes | T-FREEZE step 10; A-CRYO §4.2 |
| Vials in gas phase below limit; locations + history recorded | operational | yes | T-FREEZE step 11; A-CRYO §5 |
| Post-thaw viability/attachment via R2 (recovery QC) | operational | yes (handoff) | A-CRYO §§3/6 context; T-FREEZE "viability and recovery" media claims |

### 4.9 Instrument requirements

- Controlled-rate freezer (programmable preferred) OR passive
  isopropanol/CoolCell-type container + −70 to −90 °C freezer (T-FREEZE step 10;
  A-CRYO §4 incl. ACS-6000 example as instance, not requirement).
- LN₂ storage container holding gas phase below −135 °C/−130 °C with monitoring
  / alarms (T-FREEZE step 11; A-CRYO §5).
- 37 °C water bath for thaw handoff; 70 % ethanol decontamination (A-CRYO §6).
- Sterile 2 mL cryogenic screw-cap vials; 15/50 mL conicals; hemocytometer or
  counter for R2 (T-FREEZE Materials; A-CRYO Supplies).
- No model, capacity, or calibration-interval claim is made (UNKNOWN —
  `device_state` / `configuration_dependent`).

### 4.10 Controls

Contamination + identity verification pre-freeze (A-CRYO §1) and pre-freeze
viability gate (T-FREEZE step 1) are the lot-release controls. No assay plate
controls belong here. Post-thaw growth/viability comparison is recovery QC via
R2, not a control well.

### 4.11 Failure modes

| Phenomenon | Cause / impact | Handling + record (sourced) |
|---|---|---|
| Poor recovery from harsh harvest | Damaged cells rarely survive freeze–thaw additive injury | Harvest as gently as possible (A-CRYO §2). |
| Residual trypsin carried into freeze | Dissociating agent not removed/inactivated → poor recovery | Inactivate with serum medium or remove via spin (A-CRYO §2.4; T-FREEZE step 7). |
| Ice injury from fast/uncontrolled freeze | No controlled rate | ≈ −1 °C/min (T-FREEZE) / −1 to −3 °C/min (A-CRYO); don't trust foam-box for valuable stocks. |
| Transient warming on transfer | Viability variation | Fast transfer; dry-ice staging (A-CRYO §4.2). |
| Vial explosion on thaw | LN₂ ingress during submerged liquid-phase storage | Prefer vapour/gas phase; PPE + face protection on thaw (A-CRYO §6 + Safety Precaution). |
| DMSO toxicity / contamination | Skin contact; bad DMSO lot; prolonged DMSO dwell | Avoid contact, compliant disposal (A-CRYO WARNING); cell-culture-dedicated DMSO bottle opened only in hood (T-FREEZE Materials). Max DMSO dwell time UNKNOWN. |
| Osmotic shock on recovery | Bulk fresh-medium addition to sensitive lines | Stepwise 1–2 mL additions over 10–20 min (A-CRYO §6.2). |
| Alkaline shock on recovery | Cold/un-equilibrated medium | Pre-incubate vessel + medium ≥ 15 min to normal pH (A-CRYO §6.2). |

### 4.12 Readout

`not_implied` for the freeze/store itself. Inputs from R2 (viability/density);
recovery QC via R2. No freezer log schema is recorded.

### 4.13 UNKNOWNs

| UNKNOWN | Class | Why / who confirms |
|---|---|---|
| Per-line DMSO/glycerol % + serum % + density | `method_dependent` | Ranges retained; exact = product insert. |
| Max DMSO dwell before freeze; max "overnight" hours | `source_missing` | Not timed in sources. |
| Fill volume/headspace; vial part/material spec | `configuration_dependent` | "2 mL cryogenic vials" only. |
| Freezer program steps; LN₂ capacity/alarms/calibration | `device_state` | Facility/unit-dependent. |
| Full thaw protocol as module | `not_applicable` (handoff) | Deliberately not recorded; needs its own record if required. |
| Ready-to-use media (Recovery/Synth-a-Freeze) exact ratios | `source_missing` | Named as options; ratios on product inserts, not fetched. |

---

## 5. Cross-record evidence status

- `agent-verified`: all parameters in R1–R4 grounded on T-ADH / T-TB / T-HEMO /
  T-FREEZE / A-CRYO full-page fetches in-task (§1 table). No numeric value in
  this file comes from a search snippet.
- `search-summary-only`: C-SUB, C-AN523 — context only; zero parameters depend
  on them. Automated-counter behaviour is therefore NOT recorded.
- `UNKNOWN`: listed per record (§§1.13/2.14/3.13/4.13) with reason classes
  `source_missing` / `configuration_dependent` / `method_dependent` /
  `device_state` / `not_applicable`. No UNKNOWN is silently defaulted in JSON
  terms; a future module draft MUST omit the field or carry the UNKNOWN forward.
- Conflicts retained without averaging: passaging spin 200 × g (T-ADH) vs
  100–400 × g (T-FREEZE) vs ~100 × g (A-CRYO); freeze DMSO 5 % typical (A-CRYO)
  vs 10 % formulations (T-FREEZE); storage below −130 °C (A-CRYO) vs below
  −135 °C gas phase (T-FREEZE); viability gate > 90 % at subculture (T-ADH) vs
  ≥ 95 % healthy log-phase (T-TB) vs ≥ 90 % at freeze (T-FREEZE). Each is
  method/cell-line-dependent; the strictest gate is NOT promoted to a default.
- `review_status: research_draft`. No SourceChecked, ExpertReviewed, LabValidated,
  or ReviewRecord claim is made. A second person must verify each value against
  the cited page/section before any Compatible use.

## 6. What this record does not cover

- Suspension and insect culture variants, primary-cell specifics, and any
  cell-line product-insert numbers (densities, media, dwell times).
- Automated cell counters, image cytometers, and their gating/dilution software.
- Centrifuge rotor/rpm physics, refrigeration, braking, and unit qualification.
- Thaw/recovery as an executable module; LN₂ facility operations, alarms,
  inventory, and shipping.
- Biological assay controls, contamination assays, or authentication methods.
- Runtime JSON (`OperationModule` / `InstrumentProfile`) — no JSON was drafted;
  when drafted it stays `research_draft` until a `ReviewRecord` exists.

## 7. Audit checklist (per `record-standard.md` §7)

- [x] Every value has a source locator or explicit UNKNOWN.
- [x] Method requirements and instrument specs separated (§§1.9/2.9/3.9/4.9 are capability classes, no model claims).
- [x] Sample identity/state changes have ports + transitions (R1–R4 §.3/§.4).
- [x] Time scope is per-invocation (`once` + continuous store for R4), not inferred from assay names.
- [x] No implicit defaults (example bindings labelled; line-specific values UNKNOWN).
- [x] No partial list presented as complete (Corning marked `search-summary-only`; ready-media ratios not recorded).
- [x] Plan/actual/raw/derived separated (R2 §2.12; R1/R3/R4 `not_implied`).
- [x] No AI pre-review presented as human review; no SourceChecked claim.
