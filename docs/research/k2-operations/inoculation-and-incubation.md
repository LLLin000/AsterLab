# Cell Inoculation/Distribution and Incubation — Operation Modules

`review_status: research_draft`

> Draft only. Numeric values below are quoted examples/ranges from the
> cited manufacturer/culture-guide sources, not validated defaults.
> Nothing here is `SourceChecked`. Cell-line Product Information Sheets
> override every example density, split ratio, and schedule in this file.
> Do NOT use this file to auto-fill densities, volumes, durations, or
> incubator setpoints.

## 1. Identity

| Field | Value |
|---|---|
| Module A | `OP-INOC` — Cell inoculation / distribution (seeding / plating / splitting into vessels) |
| Module B | `OP-INCUB` — Incubation (controlled-environment hold between handling steps) |
| Relationship | Two separate authorities with one handoff artifact. They are NOT one merged operation. |
| Handoff artifact | A closed/vented, labeled seeded vessel plus its seeding record (vessel type, volume, density/count, medium, time, operator). `OP-INOC` produces it; `OP-INCUB` receives it. |
| Boundary rule | `OP-INOC` owns density, volume, vessel choice, and distribution uniformity. `OP-INCUB` owns chamber conditions, placement, duration hold, and retrieval checks. Neither module may silently set the other's parameters. |
| Scope | Mammalian adherent and suspension culture in standard vessels and CO₂ incubators. Insect/amphibian lines, spinner/shaker culture agitation speeds, hypoxia, and organoid/3D matrices are out of scope except where explicitly noted. |
| Companion instrument record | `../k2-instruments/co2-incubator-heracell-vios-160i.md` (`research_draft`) holds the Heracell VIOS 160i capability profile. This file does NOT duplicate its spec table; it references it as the example chamber `OP-INCUB` can run in. |

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator used in this draft |
|---|---|---|---|
| S1 | Thermo Fisher, "Subculturing Adherent Cells" (Gibco cell-culture protocol) | https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/subculturing-adherent-cells.html | Full page sections: viability check, wash, dissociation, neutralization, count, dilute-to-density, seed, gas exchange |
| S2 | Thermo Fisher, "Subculturing Suspension Cells" (Gibco protocol) | https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/subculturing-suspension-cells.html | Full page sections: log-phase rule, centrifuge/resuspend/count, dilution to recommended density, shaker/spinner cap handling |
| S3 | Thermo Fisher, "Useful Numbers for Cell Culture" | https://www.thermofisher.com/us/en/home/references/gibco-cell-culture-basics/cell-culture-protocols/cell-culture-useful-numbers.html | Vessel table: surface area, example seed numbers, Versene/trypsin/medium volumes |
| S4 | Corning, "Recommended Medium Volumes for Corning Cell Culture Vessels" (CLS-AN-209) | https://www.corning.com/catalog/cls/documents/application-notes/CLS-AN-209.pdf | Medium-per-area rule, per-vessel growth area / working-volume / yield rows |
| S5 | ATCC, "Animal Cell Culture Guide" (web + PDF) | https://www.atcc.org/resources/culture-guides/animal-cell-culture-guide and https://www.atcc.org/-/media/resources/culture-guides/animal-cell-culture-guide.pdf | Incubation optimum; subculture timing; trypsin workflow; suspension seed range; feeding schedule |
| S6 | ATCC, "Guide to Subculturing Cell Line Monolayers" | https://www.atcc.org/resources/technical-documents/guide-to-subculturing-cell-line-monolayers | Dissociation monitoring, ~95% single-cell target, 35–37 °C / 5–10% CO₂ statement, gas-permeable/ loose-cap rule, proportional volume scaling |
| S7 | Thermo Scientific, Steri-Cycle CO₂ Incubator Operating Manual 7010370 (Model 370/380) + Forma Steri-Cycle brochure | https://documents.thermofisher.com/TFS-Assets/LED/manuals/forma-stericycle-co2-incubator-usermanual-7010370.pdf | Set/Run/Cal modes; temperature range; CO₂ range; humidity-pan setup; sterilization cycle (generic incubator-operation reference only) |
| S8 | Sibling K2 record: Heracell VIOS 160i capability profile | `docs/research/k2-instruments/co2-incubator-heracell-vios-160i.md` | Referenced, not re-verified here |

Verification note: §2 URLs were resolved and section summaries read on
2026-11-05. No paginated line-by-line audit against the PDF manuals was
performed for this draft. Every value carries its source tag so a second
reviewer can promote individual rows to `source_checked`.

## 3. Operation contracts

### 3A. `OP-INOC` — Inoculation / distribution

| Slot | Contract |
|---|---|
| Purpose | Convert a counted cell suspension into one or more seeded culture vessels at a defined density and volume, ready for the incubator. Covers adherent reseeding and suspension splitting; dissociation itself (trypsin/TrypLE) is a prerequisite step, not part of the seeding authority. |
| Inputs | Counted single-cell suspension with viability; target vessel(s); fresh complete medium (pre-warmed); line-specific seeding instruction (Product Sheet / SOP) |
| Outputs | Seeded vessel(s): cells + medium at recorded density/volume; seeding record (line, passage, count method, viability, density, volume per well/flask, time anchor, operator) |
| Preconditions | Viability > 90% before subculturing (S1); suspension work done in log phase before confluency (S2); adherent cultures at 70–90% confluence at split time (S5); sterile technique in laminar-flow hood; solutions/equipment sterile (S1); reagents/medium pre-warmed, excitement-free handling to avoid clumping (S1, S6) |
| Completion checks | Target cell number delivered per vessel per S3/S4-derived plan; suspension confirmed mostly single cells (~95%, S6); adherent detachment ≥ 90% before harvest (S1); vessel capped/vented for gas exchange (S1, S2, S6); vessel labeled; seeding record complete; vessel handed to `OP-INCUB` |
| Time scope | A handling step: minutes per vessel set. Ends at handoff. Growth happens under `OP-INCUB`, not here. |
| Instrument requirements | Hemocytometer with Trypan blue or automated counter (S1, S2); centrifuge (typical settings only — see §5); biosafety cabinet; calibrated pipettes; qualified vessels (see §5 vessel row) |

### 3B. `OP-INCUB` — Incubation

| Slot | Contract |
|---|---|
| Purpose | Hold seeded vessels in a controlled chamber (temperature, CO₂/pH, humidity) for a defined duration anchored to the seeding (or treatment) event, then return them for the next handling or readout step. |
| Inputs | Handoff artifact from `OP-INOC` (seeded, vented, labeled vessel + seeding record); incubator with free, qualified slot; line-specific setpoints |
| Outputs | Incubated culture + hold record (chamber ID, setpoints vs. actuals, load time, unload time, door-opening deviations, observations: attachment, confluence, pH color, contamination flags) |
| Preconditions | Chamber at setpoint and stabilized before loading (S7 startup/auto-start; S8 companion profile for VIOS stabilization behavior); humidity reservoir filled per manual (S7); gas supply connected with correct inlet pressure (S7); free shelf space with air circulation (companion S8: load to max ~70% of surface for circulation); vessel vent path open (caps loosened one full turn or gas-permeable caps for shaker/spinner work, S2; gas-permeable or loosely capped vessels generally, S6) |
| Completion checks | Duration elapsed per plan; attachment/growth checked the day after seeding (S5, S6); medium replenished 2–3×/week for actively growing cultures (S5, S6); no contamination, no desiccation, pH in range; retrieval logged with time anchor |
| Time scope | A hold step: hours to days, line- and assay-dependent. No universal duration exists — Product Sheet / protocol schedule governs. |
| Instrument requirements | Direct-heat CO₂ incubator (example: Heracell VIOS 160i per S8; generic operating model per S7). No incubator setpoint in this file is a substitute for the chamber's own qualified profile. |

## 4. Ports

Ports are the only legal way material crosses a module boundary.

| Port | Direction | Object | Carries |
|---|---|---|---|
| `P1` suspension-in | into `OP-INOC` | Counted cell suspension | Cell count, viability, passage, medium, volume |
| `P2` vessel-in | into `OP-INOC` | Empty qualified vessel | Vessel type, surface treatment, sterility |
| `P3` medium-in | into `OP-INOC` | Fresh complete medium | Formulation, pre-warm state, volume |
| `P4` seeded-vessel | `OP-INOC` → `OP-INCUB` (the handoff) | Seeded, vented, labeled vessel + seeding record | Line, density/count per well, volume per well, vessel ID, seed time `t₀`, operator |
| `P5` chamber-slot | into `OP-INCUB` | Incubator shelf position | Chamber ID, setpoints, available space |
| `P6` incubated-culture | out of `OP-INCUB` | Culture after hold + hold record | Load/unload times, condition log, observations |

Connection rule: `P4` is mandatory. `OP-INCUB` MUST NOT accept an
unlabeled vessel, a vessel without a seeding record, or a sealed vessel
with no vent path. `OP-INOC` MUST NOT command chamber setpoints.

## 5. Parameters (only when sourced — otherwise UNKNOWN)

`required_at`: `protocol` = fixed by method version; `design` = set per
study; `run` = recorded at execution.

### 5A. `OP-INOC` parameters

| Parameter | Type / unit | Source | Required_at | Constraints / notes (sourced) |
|---|---|---|---|---|
| Seeding density (adherent, per area or per well) | decimal, cells/cm² or cells/well | S3 (example numbers by vessel); S5/S6 (line-specific Product Sheet governs) | design | No universal value. S3 examples only (e.g. 60 mm dish ~0.8×10⁶ cells; 24-well ~0.05×10⁶/well; T-75 ~2.1×10⁶) — examples, NOT defaults. S6: some lines need minimum numbers; fast growers need lower densities. |
| Seeding density (suspension) | decimal, viable cells/mL | S5 | design | S5 range 2×10⁴–5×10⁵ viable cells/mL at seed, up to ~2×10⁶ cells/mL at saturation. Line-specific sheet overrides. |
| Split ratio (maintenance shortcut) | string/decimal, e.g. 1:2 | S6 | run | S6 notes 1:2 splits are common for routine maintenance where exact density tracking is less critical. Not a substitute for counted seeding in designed experiments. |
| Medium volume per vessel | decimal, mL (per well / flask / dish) | S4; S3 | design | S4 rule: 0.2–0.3 mL medium per cm² growth area. Per-vessel rows in S4 (e.g. 6-well ~1.9–2.9 mL/well; 12-well ~0.76–1.14; 24-well ~0.38–0.57; 25 cm² flask 5–7.5 mL; 75 cm² 15–22.5 mL). Microplate/Transwell volumes run higher against meniscus/evaporation; deeper medium slows O₂ diffusion (S4). Scale proportionally for non-75 cm² vessels (S6). |
| Average cell yield at confluence | decimal, cells/cm² | S4 | design (planning only) | S4 planning figure ~1×10⁵ cells/cm² monolayer average; actual varies by line and conditions, higher or lower. Do NOT use as a seeding input. |
| Vessel identity | enum (plate/flask/dish/Transwell + well count + surface treatment) | S4 | design | Growth areas are engineering-drawing approximations; cells growing up vessel sides alter usable area (S4). S4/S6 surface options: TC-treated, CellBIND, ULA, non-treated. |
| Viability gate | decimal, % | S1 | run | Confirm > 90% viability before subculturing (S1). Method: Trypan blue hemocytometer or automated counter (S1, S2). |
| Single-cell quality gate | decimal, % single cells | S6 (ATCC monolayers); S1 (≥90% detached) | run | Aim ~95% single-cell suspension before seeding (S6); S1: proceed once detachment ≥ 90%, else incubate longer / tap gently. Disperse clusters gently (S5). |
| Centrifuge settings | decimal, ×g / min | S1, S2 | run | S1 typical 200×g, 5–10 min (may vary by cell type). S2: 800×g 10 min for harvest; gentle 100×g 5–10 min wash every ~3 weeks for debris. All are typical values, NOT fixed — cell type governs. |
| Neutralization / dilution ratio | ratio | S1 | run | After detachment, remove dissociation reagent and add ~2× its volume of pre-warmed complete medium (S1). Serum must be removed before trypsin (trypsin inhibited otherwise, S6). |
| Reagent pre-warm state | boolean + °C | S1, S5, S6 | run | Pre-warm dissociation reagent and medium (usually 37 °C bath); pre-warm enzyme to reduce exposure time (S6). Exact bath time is UNKNOWN. |
| Vent state at handoff | enum (vented cap / loosened cap / gas-permeable seal) | S1, S2, S6 | run | Ensure proper gas exchange (S1). Shaker flasks: loosen cap one full turn or use gas-permeable cap; spinner: loosen side-arm caps one full turn (S2). Generally: gas-permeable or loosely capped vessels (S6). |
| Seed time anchor `t₀` | datetime | — (record-keeping) | run | UNKNOWN as a sourced format. Record actual seed time per vessel set; downstream durations anchor to it. |
| Distribution uniformity method | UNKNOWN | — | — | UNKNOWN. No sourced quantitative acceptance criterion (e.g. CV of cells/well) found in S1–S6. Gentle handling / avoid vigorous agitation (S6) is qualitative only. |
| Edge-well / evaporation policy | UNKNOWN | — | — | UNKNOWN. S4 notes meniscus/evaporation effects in microplates but gives no fill/exclusion pattern. |

### 5B. `OP-INCUB` parameters

| Parameter | Type / unit | Source | Required_at | Constraints / notes (sourced) |
|---|---|---|---|---|
| Temperature setpoint | decimal, °C | S5, S6, S7, S8 | design (line) / run (actual) | Most animal lines 37 °C (S5). S6: typically 35–37 °C. Insect/amphibian/sensitive lines differ (e.g. ~28 °C, S5). A few hours > 2 °C above optimum can harm growth (S5). Chamber capability is instrument-specific (S7/S8), not repeated here. |
| CO₂ setpoint | decimal, vol% | S5, S6, S7 | design (line) / run (actual) | Typical 5% CO₂ (S5). S6 states generally 5–10%. Buffer/pH system must match CO₂ availability, especially in closed systems (S6). Chamber range is instrument-specific (S7/S8). |
| Humidity | decimal, % rH | S5 (culture need); S7/S8 (chamber capability) | design / run | Cultures: high humidity; S5-family guidance 85–95% for typical mammalian work. Chamber example (S8): ≈93% rH high-humidity mode / ≈90% low-humidity mode at 37 °C via heated reservoir. Reservoir water quality and fill are chamber-manual items (S8; S7 pan fill). |
| O₂ setpoint | decimal, vol% or UNKNOWN | S8 | design only if line requires | UNKNOWN for standard culture (normoxia assumed, no sourced setpoint). Optional O₂ control exists on some chambers (S8: 1–21% or 5–90% options); only specify when the line/protocol demands it. |
| Medium / buffer system | enum (bicarbonate/HEPES etc.) | S6 (principle only) | design | S6 principle: ensure buffer maintains pH at available CO₂. No sourced formulation table — UNKNOWN per medium. |
| Duration + time anchor | duration + datetime | S5, S6 (schedule cues only) | design (planned) / run (actual) | No universal duration. Sourced cues: check attachment/growth the day after seeding (S5, S6); change medium 2–3×/week as needed (S5, S6); subculture adherent lines at 70–90% confluence to hold exponential growth (S5). Exact hours/days per line: UNKNOWN. |
| Shelf position / load map | slot reference | S8 (principle) | run | Load to max ~70% of surface for air circulation (S8). Exact slot map format: UNKNOWN. |
| Door-opening / recovery policy | UNKNOWN | — | — | UNKNOWN as a plannable spec. S8 records recovery time as UNKNOWN (brochure internal-testing figure only). Door-open alarms exist (S8) but no sourced minute budget for plans. |
| Agitation (shaker/spinner speed) | UNKNOWN | S2 (principle only) | — | S2: keep spinner speed within cell-line-recommended range to minimize shear; add Pluronic F-68 for insect lines to reduce shear. No sourced rpm table — UNKNOWN per line. |

## 6. Sample transitions

```text
[P1 suspension] + [P2 empty vessel] + [P3 medium]
        │  OP-INOC: count → dilute → dispense → vent → label
        ▼
[P4 SEEDED VESSEL + seeding record]   ←── authority boundary ── handoff
        │  OP-INCUB: place → hold (T/CO₂/RH) → monitor → retrieve
        ▼
[P6 INCUBATED CULTURE + hold record] → next step (treatment / feed / assay / split)
```

| Transition | From → to | Owning authority | Guard |
|---|---|---|---|
| T-INOC | Counted suspension → seeded vessel | `OP-INOC` | Density/volume per S3/S4-derived plan; viability > 90% (S1); ~95% single cells (S6); vent path open (S1/S2/S6) |
| T-HANDOFF | Seeded vessel → chamber slot | Boundary | Label + seeding record complete; chamber at setpoint and stabilized (S7); slot free with circulation (S8) |
| T-HOLD | Loaded vessel → incubated culture | `OP-INCUB` | Setpoints per line (S5/S6); duration per schedule (S5/S6 cues); attachment check next day; feeds 2–3×/week (S5/S6) |
| T-EXIT | Incubated culture → next operation | Boundary | Hold record closed (load/unload times, deviations); contamination/desiccation/pH flags cleared or escalated |

Container-state note (S4, S6): vessel identity (format + surface) is
fixed at `OP-INOC` and MUST NOT change under `OP-INCUB`. If the plan
needs a different surface (e.g. ULA vs TC-treated), that is a new
`OP-INOC` instance, not an incubation adjustment.

## 7. Failure modes

| # | Failure | Detectable at | Sourced signal / mitigation |
|---|---|---|---|
| F1 | Contamination (bacterial/fungal/mycoplasma) | `OP-INCUB` hold checks | Sterile technique + hood discipline (S1); Steri-Run/HEPA chamber hygiene (S7/S8); ATCC sterility-testing and banking practice (S5 family). Turbid medium / pH crash / clumping in suspension signals confluency-or-contamination check (S2). |
| F2 | Seeding density wrong (too sparse → no growth; too dense → premature confluence) | Next-day attachment check; confluence timing | S6: some lines need minimum numbers; fast growers need lower densities. Fix is re-seed per Product Sheet, not incubator adjustment. |
| F3 | Cell clumping / uneven distribution | Pre-seed microscopy; next-day field check | Target ~95% single cells (S6); disperse gently (S5); avoid vigorous agitation during detachment (S6); tap gently only (S1). Quantitative uniformity criterion UNKNOWN (§5A). |
| F4 | Wrong vessel / surface / volume | `OP-INOC` completion | S4 area/volume rows; S4 O₂-diffusion warning on overfilled wells; S6 proportional scaling. Seeding a dense suspension into an empty vessel without fresh medium equilibration is flagged by ATCC practice (S5: equilibrate fresh medium to pH/temperature; transfer 12–15 mL fresh medium to new flask in the 75 cm²-flask workflow). |
| F5 | Vent path closed (sealed cap, wrong seal) → pH drift / poor growth | `OP-INCUB` entry + pH color | S1/S2/S6 vent rules (§5A). Closed-system buffering UNKNOWN per medium (§5B). |
| F6 | Temperature excursion (> 2 °C above optimum for hours) | Chamber alarms / hold record | S5 harm threshold; S7/S8 tracking/overtemperature alarms. Recovery-time budget UNKNOWN — log the deviation, do not assume recovery. |
| F7 | Desiccation / edge evaporation (microplates) | Hold checks, volume inspection | S4 meniscus/evaporation note. No sourced exclusion/fill policy — UNKNOWN; flag, do not invent a pattern. |
| F8 | Over-confluence before split (missed 70–90% window) | Confluence check | S5 70–90% subculture rule; S2 log-phase-before-confluency rule. Phenotype-drift risk noted qualitatively (S1: conditions critical to avoid aberrant phenotypes). |
| F9 | Dissociation damage (over-trypsinization) | Detachment monitoring | S1 ~2 min starting exposure, extend only by observation; S5/S6 5–15 min typical with ~5 min monitoring cadence. Exact minutes per line UNKNOWN. |
| F10 | Record gap (no `t₀`, no density, no chamber actuals) | Handoff / exit guards | `P4`/`P6` mandatory fields (§4). Missing values block downstream scheduling; never back-filled. |

## 8. The boundary, stated once

`OP-INOC` ends when the seeded, vented, labeled vessel and its seeding
record exist. `OP-INCUB` begins when that vessel is placed into a
stabilized chamber slot. Density, volume, vessel, and uniformity
questions go to `OP-INOC` and its line-specific sources (S1–S6).
Temperature, CO₂, humidity, duration-hold, and retrieval questions go to
`OP-INCUB` and its line + chamber sources (S5–S8). A growth problem is
never fixed by re-tuning the other module's parameters without evidence.

## 9. UNKNOWNs (do not use for planning)

| Item | Status | Reason |
|---|---|---|
| Universal seeding density / split ratio | UNKNOWN | Line-dependent by every source (S1, S2, S5, S6). S3 numbers are vessel examples, not prescriptions. |
| Exact dissociation minutes per line | UNKNOWN | S1 ~2 min start / S5–S6 5–15 min are typical bands with observation cadence, not fixed recipes. |
| Quantitative uniformity acceptance (cells/well CV) | UNKNOWN | No criterion in S1–S6. |
| Edge-well / evaporation fill policy | UNKNOWN | S4 notes the physics; no pattern sourced. |
| Incubation duration per line/assay | UNKNOWN | Only schedule cues sourced (next-day check; 2–3 feeds/week; 70–90% split window). |
| Closed-system buffer formulation table | UNKNOWN | S6 states the principle only. |
| Shaker/spinner rpm per line | UNKNOWN | S2 gives shear-avoidance principle only. |
| Door-opening minute budget / recovery spec | UNKNOWN | Per S8: brochure figure is internal testing, not a DIN-rated spec. |
| Chamber calibration / filter / service intervals | UNKNOWN | Deferred to the instrument record (S8 lists them UNKNOWN). |
| Contamination-rate or growth-rate expectations | UNKNOWN | No sourced rates in S1–S6. |

## 10. What was verified vs. not

- Read and cited 2026-11-05 (§2): section-level content of S1–S7 via
  official pages/manuals plus the sibling S8 profile. Every numeric
  example in §5 names its source; anything without a source is marked
  UNKNOWN.
- NOT verified: paginated PDF line audit of S4/S5-PDF/S7; second-person
  source check; any wet-lab confirmation; lab-SOP alignment; exact
  Product Sheet values for any specific cell line; current chamber
  firmware/calibration state.
- Promotion rule: a row leaves `research_draft` only with (a) source URL
  + page/table/section, (b) retrieval date, (c) vendor-spec vs SOP vs
  chamber-configuration distinction, and (d) second-reviewer sign-off —
  per `../k2-instruments/README.md` minimum bar.
