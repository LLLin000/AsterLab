# Operation Granularity Scenarios — Five Worked Examples

- Date: 2026-11-05
- Status: `research_draft`; NOT `SourceChecked`. No `ReviewRecord` exists.
- Standard: [record-standard §1.1–§1.3](../record-standard.md#11-粒度判定最小可审计科学操作) (four layers + Custom Step).
- Method: each scenario is decomposed into Assay model → Method Template(s) → Composite Block(s) → Atomic Operation(s) → Custom Step / conditional / manual-judgment points. Existing records are context only; nothing here promotes a draft or invents a field.
- Context records used (no values re-verified here): `cell-culture-and-separation.md` (R1 passaging, R2 counting, R3 pelleting), `inoculation-and-incubation.md` (`OP-INOC` / `OP-INCUB`), `washing-and-sampling.md` (`wash` / `sample_aliquot`), `record-liquid-transfer.md` (`op.liquid_transfer`), `absorbance-and-fluorescence-readout.md` (`ABS_READ` / `FLUO_IMAGE`), `biofilm-cv-cfu.md` (OP-BIOFILM-*, OP-CV-BIOMASS, OP-CFU-DILUTION-PLATE), `qpcr-and-nucleic-acid-qc.md` (R1/R2/R3), `elisa-and-western-blot.md` (`METHOD-ELISA` / `METHOD-WB`), `granularity-candidates.md`, `granularity-and-biofilm.md`, `k2.2-cross-operation-matrix.md`.

Notation: `A` = Assay model, `M` = Method Template, `C` = Composite Block, `O` = Atomic Operation, `X` = Custom Step / gate. `UNKNOWN` classes per record-standard §4 (`source_missing`, `method_dependent`, `configuration_dependent`, `device_state`, `not_applicable`).

---

## Scenario 1 — L929 CCK-8 cytotoxicity / viability

Assay question: does the test condition reduce L929 viable-cell dehydrogenase activity vs control?

```text
A  cell viability (L929 cytotoxicity)
└─ M  CCK-8 viability assay (seed → treat → develop → read → derive)
    ├─ C  cell seeding + growth hold (OP-INOC → OP-INCUB)
    ├─ C  treatment / dosing (serial compound dilution + addition)
    ├─ C  CCK-8 development + absorbance read
    │   └─ O  ABS_READ (450 nm + ≥600 nm reference, same-plate blank)
    └─ X  derivation (blank subtraction, standard curve, viability %) — downstream, NOT in readout
```

| Layer | Content (this scenario) | Source record |
|---|---|---|
| A Assay model | Cell viability: measurand is dehydrogenase-proportional formazan signal, interpreted as viability only with same-cell-type curve + controls | readout §8; candidates D |
| M Method Template | CCK-8 assay: seeding density, treatment schedule, CCK-8 dose (10 µL/well 96-well example), 1–4 h 37 °C 5% CO₂ develop, 450 nm read, blank/vehicle/standard-curve companions | readout S6, §§3.1/4.1/5 |
| C Composite | C1 seed+hold: `OP-INOC` counted seeding → `OP-INCUB` hold with next-day attachment check, 2–3×/week feed cue. C2 dosing: serial dilution (repeated `liquid_transfer`) + addition to wells. C3 develop+read: CCK-8 addition → timed hold → `ABS_READ` | inoculation §§3–5; liquid-transfer §5; readout §9 |
| O Atomic | `liquid_transfer` (seed, dose, CCK-8 add); `wash` only if medium exchange specified (sample-preserving variant); `incubation` hold; `cell-counting-viability` (pre-seed gate >90%); `ABS_READ` raw OD only | cell-culture R2; washing §4.1; readout §3.1 |
| X / conditional / judgment | Confluence at seed/treat time (70–90% cue, operator call); edge-well/evaporation flag (no sourced fill policy); OD outside linearity (≥2.5 Abs caution) → re-run at adjusted load, never clip-and-interpret; turbidity/reducing-agent background → ≥600 nm correction recorded, subtraction downstream | inoculation F2/F7; readout F2/F3; readout §5 |

Line-specific unknowns (do not plan): L929 seeding density (`method_dependent`, Product Sheet governs); treatment concentrations/schedule (`method_dependent`); per-cell-type OD–count linearity (`method_dependent`); CCK-8 lot background (`configuration_dependent`).

---

## Scenario 2 — Adherent cell passaging (maintenance, not a readout assay)

Assay question: none — this is lineage maintenance. Assay slot records the design intent (keep line in log-phase, preserve identity), not a measurement.

```text
A  cell-line maintenance (log-phase expansion, identity preserved)
└─ M  adherent subculture (wash → dissociate → quench → collect → count → re-seed → hold)
    ├─ C  harvest (wash + dissociate + quench + disperse)
    ├─ C  handoff (pellet [optional] + count/viability → dilute-to-density)
    └─ O  OP-INOC → OP-INCUB (re-seed + hold)
```

| Layer | Content (this scenario) | Source record |
|---|---|---|
| A Assay model | Maintenance / expansion: no measurand; output is a new passage with recorded lineage (line, passage no., seed record, hold record) | inoculation §6 (T-HANDOFF/T-EXIT); candidates D (excluded from assay list) |
| M Method Template | `cell-passaging-adherent` + `OP-INOC`/`OP-INCUB` binding: per-10 cm² wash/dissociation volumes, 2-vol quench, line-specific density | cell-culture R1 §§1.1–1.5; inoculation §5A |
| C Composite | Harvest block (T2–T4 transitions); spin+count handoff (`centrifugation-cell-pelleting` → `cell-counting-viability`); re-seed block (`OP-INOC` density/volume/vessel/vent + label) | cell-culture R1 §§1.3–1.4; inoculation §6 |
| O Atomic | `wash` (Ca/Mg-free, gentle side-addition, ~2 mL/10 cm² example); `liquid_transfer` (reagent add, quench, split); `centrifugation` (low-speed pellet, settings line-dependent); `cell-counting-viability` (0.4% TB 1:1, 10 µL load, ×10⁴, duplicates); `incubation` hold | cell-culture R1–R3; washing §5.1; liquid-transfer §5 |
| X / conditional / judgment | Detachment staging gate (≥90% before quench; recheck q30s + tap); viability gate (>90% to passage, ≥95% healthy advisory); dissociation "varies with cell line" dwell — extend-by-observation, never by default table; destination-vessel choice when surface changes (new `OP-INOC`, not an incubation tweak) | cell-culture R1 §§1.5/1.8/1.11; inoculation F9 |

Unknowns: trypsin/TrypLE concentration, exact dwell, seeding density numeric, flask-size scaling (`method_dependent` / `source_missing` per R1 §1.13).

---

## Scenario 3 — Bacterial streak isolation + spread-plate enumeration

Two method templates share atomics but answer different questions — isolation (pure culture) vs concentration (CFU).

```text
A1  pure-culture isolation        A2  viable enumeration (CFU)
└─ M  streak isolation            └─ M  spread-plate + count
     ├─ C  single-colony pick/sub      ├─ C  serial dilution (repeated liquid_transfer)
     └─ O  streak inoculation          ├─ O  spread inoculation (0.1 mL example, NOT a default)
                                       ├─ O  incubation (organism/medium T/t)
                                       └─ O  colony count (30–300 window) + CFU derivation
```

| Layer | Content (this scenario) | Source record |
|---|---|---|
| A Assay model | A1 isolation (presence/purity of colonies); A2 viable concentration (CFU/mL via S3 formula, back-calculated per volume) | biofilm S3 formula; candidates D |
| M Method Template | M-streak: loop drawing across agar quadrants to separate cells spatially. M-spread: dilute-to-window → spread fixed volume → incubate → count → compute; spread vs pour choice (surface vs embedded) stays variant-level | candidates B/C; biofilm §§7.2–7.3 |
| C Composite | Serial dilution (product of per-tube factors; sterile-pipette-per-transfer discipline); colony-incubation hold (organism/medium-dependent) | biofilm §7.3 (`dilution_factor_per_tube`, `pipetting_discipline`) |
| O Atomic | `streak inoculation` (one spatial-distribution act; do NOT split per stroke); `spread inoculation` (one surface-distribution act; do NOT split per spreader motion); `incubation`; `colony count` (readout only — count + image, 30–300 window, duplicates averaged); `liquid_transfer` per dilution step | candidates A (streak/spread atomicity rule); biofilm §§7.3–7.4; record-standard §1.1 examples |
| X / conditional / judgment | Single well-isolated colony present? (pick vs re-streak); which plate falls in 30–300 (operator selection, duplicates averaged); contamination call (air-contaminant <30 rationale); agar-label rule (name/date/medium/organism/dilution, search-summary-only detail) | biofilm S3 Comments & Tips; candidates E |

Unknowns: agar volume/drying/rest time, incubation T/t per organism, pour-vs-spread SOP choice (`method_dependent`; S4 search-summary-only, not verified).

---

## Scenario 4 — Biofilm formation: CV biomass vs CFU viability (two non-interchangeable templates)

Read first: CV (total adherent dye-binding material) and CFU (viable culturable cells after dispersal) MUST NOT be merged into one "biofilm assay" module.

```text
A  biofilm formation (attached + planktonic fractions co-exist)
├─ M  biofilm_cv (wash → stain → wash → solubilize → read; total biomass)
└─ M  biofilm_cfu (wash → disperse → dilute → plate → incubate → count → derive; viable count)
```

| Layer | Content (this scenario) | Source record |
|---|---|---|
| A Assay model | Static microtiter biofilm formation; input `PLANKTONIC_INOCULUM`, output `ATTACHED + PLANKTONIC` co-fractions; organism/medium/time empirically determined per microbe | biofilm §§3.2–3.3; granularity-and-biofilm §2 |
| M Method Template | `OP-CV-BIOMASS`: 0.1% CV 125 µL 10–15 min → post-stain rinse → organism solvent (S2 Table 1B.1.1) → 550 nm (500–600 reader range) + same-solvent blank. `OP-CFU-DILUTION-PLATE`: Alt-PBS wash → dispersal → dilution series → spread/pour → count 30–300 → S3 formula (worked 5.1×10⁴ CFU/mL example) | biofilm §§5.3/7.3 |
| C Composite | `OP-BIOFILM-STATIC-CULTURE` (inoculate 1:100, 100 µL/well 96-well, 4–8 replicates +/− controls); `OP-BIOFILM-DISPERSAL` (cut-well → 2.0 mL PBS tube → sonication regimen, e.g. S2 microtip 8 s 40%); serial-dilution block | biofilm §§3.3/6.3 |
| O Atomic | `inoculation` (into wells); `incubation` hold; `lossy wash` variant (pre-stain dump+2 submerge/shake-outs; Alt 6× PBS; planktonic removal + unquantified detachment — NEVER generic `wash`); `stain` (CV); `solubilize`; `absorbance read` (raw OD); `liquid_transfer` (dilution steps); `colony count` | biofilm §§4/5/7; granularity-and-biofilm §§2.1/5 |
| X / conditional / judgment | Kill-control gate (planktonic ±sonication viability; block plating batch on fail — deterministic gate); detachment check (CV re-stain of carrier, optional); solvent choice per organism (empirical); <30/>300 plate selection; hold-time limit post-dispersal (`UNKNOWN`); non-sonication regimens need own kill-control + detachment check | biofilm §§6.3–6.4/8 |

Unknowns: detachment-loss magnitude (always ≥0, unquantified without same-organism control); sonication absolute energy (`device_state`); agar/incubation per organism (`method_dependent`); post-stain dry interval, solubilized-CV stability window (`source_missing`).

---

## Scenario 5 — qPCR relative quantification (two-step, SYBR; ELISA noted as the alternate M)

Chosen 5th scenario: qPCR. ELISA (`METHOD-ELISA`: coat→block→bind→detect→develop→stop→read; sandwich/direct/indirect/competitive variants; stopped-TMB 450 nm ≤30 min / chemi 1–5 min) follows the same four-layer shape and is not worked here to avoid duplicating the chain.

```text
A  relative transcript abundance (target vs reference genes, calibrator group)
└─ M  two-step SYBR RT-qPCR (extract → QC → RT → amplify/detect → derive)
    ├─ C  NA extraction + pre-RT QC batch
    ├─ C  RT batch → archived cDNA (one-step variant: RT phase inside R3, NO cDNA port)
    └─ C  qPCR plate run (standards/references/NTC/NRT layout → curves → Cq → ΔCq/ΔΔCq)
```

| Layer | Content (this scenario) | Source record |
|---|---|---|
| A Assay model | Relative expression: measurand is Cq under stated threshold/baseline rules; "up/down-regulation" is downstream interpretation, not readout | qpcr §§3/11 |
| M Method Template | SYBR two-step relative workflow; one-step vs two-step, SYBR vs probe, relative vs absolute MUST NOT be collapsed (different ports/companions) | qpcr §4 |
| C Composite | R1 extraction+QC batch (`per_assay` + eluate storage clock); R2 RT batch (recorded T/t, priming strategy, enzyme); R3 plate run (complete reaction conditions + thermocycling program + well map recorded per run; cycling program never copied between assays) | qpcr §§5/7/12 |
| O Atomic | `liquid_transfer` (setup dilutions, plate loading); `sample_aliquot` (eluate/cDNA aliquots with lineage); `incubation` thermal holds (RT T/t, cycling); qPCR fluorescence acquisition (raw curves + Cq + method + program version + outlier rule + NTC Cq) | qpcr §§5/7; washing §4.2 (aliquot lineage); liquid-transfer §4 |
| X / conditional / judgment | No-RT (±RT pair, essential at RNA validation); inhibition test (dilution/spike); specificity evidence (melt/gel/sequence — mandatory in practice for SYBR); reference-gene justification + normalisation method (blocks relative claims); standard-curve companions for any efficiency-corrected claim; Cq-method/software-version break (re-analyze or record break; cross-method ΔCq not commensurable) | qpcr §§6/10, F1–F11 |

Unknowns (selection): extraction kit volumes/yields; universal A260/A280, A260/A230, RIN cutoffs (all `method_dependent` — S3/S4 ranges are search-summary-only context, never gates); RT enzyme/priming/T-t; primer/probe sequences and concs; any thermocycling triple; channel/filter bands and multiplex sets (`configuration_dependent`); efficiency/r²/LOD/reference identity (measured/justified per study).

---

## What the current contract cannot express (no new fields invented)

Per record-standard §5 and `k2.2-cross-operation-matrix.md` §4 (`能表达/不能表达`):

1. **Numerical conservation** — parent-remaining + Σ aliquots = parent-before (sampling, dilution series); wash residual volume; CFU back-calculation (CFU/well = CFU/mL × 2.0 mL). Contract holds structure (`Positive` > 0) but no arithmetic across ports.
2. **Lineage / ID generation** — parent→child sample IDs, well→tube 1:1 maps (biofilm dispersal), passage numbers, cDNA aliquot genealogy. Currently record-fields/notes only.
3. **Conditional execution** — `transfer_mode` (`partial`/`full`/`add_to_existing`), countable-window plate selection, kill-control gate blocking the CFU batch, detachment-threshold gating. `applies_when` records branches; runtime choice is unimplemented.
4. **Actual state change typing** — attached vs planktonic fractions in one well; ≥90% detached; contamination/desiccation/pH flags; fixation terminality (LIVE/DEAD must-not-fix rule). `state_changed` is untyped.
5. **Loss as an effect** — biofilm wash detachment (`DETACHMENT_LOSS ≥ 0`), dispersal kill vs residual-attached undercount, over-wash sensitivity loss. No loss/quantity effect exists; do not overload `state_changed`.
6. **Cross-field / cross-module validation** — pipette volume-range vs planned volume; reader λ/bandwidth vs assay λ; fluorescence build (module/cube/illumination/detector/focus/stage) vs dye panel; washer/plate/λ compatibility. Documented as capability gates, not JSON types.
7. **Assay-companion layering** — blanks, standard series, NTC/NRT, reference genes, kill-controls, secondary-only lanes. Required companions live in method records; the generic operation JSON has no slot for them.
8. **Time-anchored II** — `t₀` seeding anchors, stop→read clocks (TMB ≤30 min plan), substrate-exhaustion fade (chemi 1–5 min), cDNA/eluate storage clocks, door-opening/recovery budgets. Time scope sets (`once`/`per_assay`) do not compute anchors.

Cutover rule (unchanged): keep every UNKNOWN omitted-or-explicit in JSON, keep all five scenario IDs out of `OperationModule` JSON, keep this file `research_draft` until a second-person `ReviewRecord` exists.

## Sources (this file verifies nothing new)

- Record standard: `docs/research/record-standard.md` §§1.1–1.3, §5.
- Matrix: `k2.2-cross-operation-matrix.md` §§1/4.
- Biofilm: `biofilm-cv-cfu.md` S1–S3 (agent-verified reads) + S4–S5 (search-summary-only); `granularity-and-biofilm.md`.
- Cell: `cell-culture-and-separation.md` T-ADH/T-TB/T-HEMO/T-FREEZE/A-CRYO (agent-verified); `inoculation-and-incubation.md` S1–S8 (section-level, S8 sibling profile).
- Readout: `absorbance-and-fluorescence-readout.md` S1–S5 (transcribed) + S6–S9 (web-summary).
- qPCR: `qpcr-and-nucleic-acid-qc.md` S2 (agent-verified checklist); S1/S3–S5 search-summary-only.
- ELISA/WB alternate: `elisa-and-western-blot.md` E1–E3/W1–W2 (method-record level).
- No second-person review; no `SourceChecked` / `ExpertReviewed` / `LabValidated` claim in this file.
