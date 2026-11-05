# Liquid transfer — reusable operation module (research draft)

`review_status: research_draft` — NOT SourceChecked. No values invented. Every
numeric or behavioural claim below carries its source locator; anything not
verified from a fetched source is marked UNKNOWN.

## 1. Identity

- **Operation:** `liquid-transfer` — move a stated volume of liquid from a source
  to a destination using a piston-stroke pipette + disposable tip, optionally
  with mix / blow-out / touch-tip / air-gap steps.
- **Scope:** manual pipetting is the reference implementation. Automated liquid
  handling (Opentrons Python API) is noted as a mapping in §11, not as the
  default.
- **Granularity:** one transfer = aspirate (+ optional pre-steps) → dispense
  (+ optional post-steps). Multi-well loops (`transfer` / `distribute` /
  `consolidate`) are compositions of this operation, not separate operations.
- **Out of scope:** sample creation/destruction bookkeeping, incubation,
  centrifugation, counting, and scheduling belong to the calling Protocol, not
  to this module.

## 2. Sources (retrieved 2026-11-05)

| # | Source | Locator used |
|---|--------|--------------|
| S1 | Opentrons docs, "Python API: Liquid Control" https://docs.opentrons.com/python-api/building-block-commands/liquids/ | Aspirate, Dispense, Push out after dispense, Blow out, Touch tip, Mix, Dynamic mix, Air gap, Detect/Require/Measure liquids sections (fetched full text, this task) |
| S2 | Opentrons docs, "Python API: Complex Command Parameters" https://docs.opentrons.com/python-api/complex-commands/parameters/ | Tip handling, Mix before, Disposal volume, Touch tip, Air gap, Mix after, Blowout behaviour table, Trash tips sections incl. `transfer`/`distribute`/`consolidate` behaviour rows and `blow_out=True` + `blowout_location` rule (fetched full text, this task) |
| S3 | Eppendorf Research plus operating manual, doc 3120 900.012-06/042013, via mirror https://pipette.com/mm5/pdfs/manuals-brochures/Operating-manual-Research-plus.pdf — **via** `docs/research/k2-instruments/manual-pipette-eppendorf-research-plus.md` (that file verified S1 §§1–8, pp. 5–35; this task did NOT re-fetch the PDF) | Family span 0.1 µL–10 mL (§1.1); air-cushion principle + first-stop/blow-out stroke (§1.1); tip rules incl. LoRetention + 5/10 mL filter rules (§§2.2, 3.2); aqueous-like liquids, temperature/viscosity warnings, pre-wet 1–3× (§§2.2, 5.5.1); test conditions ISO 8655-1/-2/-6, distilled water ISO 3696, 10 determinations (p. 31); tips single-use (§§2.2, 3.4) |
| S4 | ATCC, "Guide to Subculturing Cell Line Monolayers" https://www.atcc.org/resources/technical-documents/guide-to-subculturing-cell-line-monolayers | "Steps for harvesting a cell monolayer" 1–5 (rinse 5 mL; add 2–3 mL dissociating solution; check every 5 min; add 6–8 mL growth medium; collect/count/dispense; optional 5 min at 125 × g); medium 12–15 mL per 75-cm²; trypsin 0.05–0.25 %, 10–15 min; seed 10³–10⁴ cells/cm²; re-incubate 35–37 °C, 5–10 % CO₂, pH 7.0–7.6, medium change 2–3×/week; troubleshooting (vigorous pipetting → DNA/clumps/viability loss) (fetched full text, this task) |
| S5 | ISO 8655:2022 context (secondary, NOT fully verified here) — Eppendorf "The High Art of Pipette Calibration" https://www.eppendorf.com/gb-en/lab-academy/life-science/next-generation-sequencing/the-high-art-of-pipette-calibration/ ; Sartorius white paper https://www.sartorius.com/en/products/pipetting/pipetting-resources/gravimetric-calibration-testing-pova-iso8655-white-paper | Cited only for "ISO 8655-6 is the gravimetric reference procedure for piston-operated volumetric apparatus (POVA)". Full standard text is paywalled; 2022-revision deltas (condition windows, tip-exchange rules, reporting) are UNKNOWN from my verified sources — see §12 |

## 3. Operation contract

| Item | Value |
|---|---|
| Operation key | `liquid-transfer` (suggested; not a CLI claim) |
| Reference implementation | Manual piston-stroke pipette + disposable tip (S3) |
| Automation mapping | Opentrons building-block commands (`aspirate`, `dispense`, `blow_out`, `mix`, `touch_tip`, `air_gap`, `push_out`) composed by complex commands (`transfer`, `distribute`, `consolidate`) (S1, S2) |
| Example protocol binding | ATCC monolayer subculture harvest steps (S4) — volumes below are example bindings for a 75-cm² flask, NOT operation defaults |
| Calibration basis | Gravimetric verification per ISO 8655-6 against manufacturer limits (basis per S3 p. 31; current ISO text per S5 UNKNOWN) |
| Status | `review_status: research_draft`, NOT SourceChecked |

## 4. Ports (inputs / outputs)

| Port | Kind | Required | Notes + source |
|---|---|---|---|
| `source_sample` | input sample | required at run | Liquid-bearing sample aspirated from. Opentrons `source` = well or list of wells (S2). ATCC example: culture medium to discard, trypsinised cell suspension to collect (S4 steps 1, 5). |
| `source_container` | input container | required at run | Well / flask / reservoir holding `source_sample`. Positioned via well or `Location` (`top`/`bottom` offsets, meniscus on Flex) (S1). |
| `destination_sample` | output sample | required at run | Sample created or augmented by the dispense. ATCC example: cells resuspended in 6–8 mL growth medium; aliquots dispensed into prepared vessels (S4 steps 4–5, "Plate cultures"). |
| `destination_container` | input container (pre-existing) | required at run | Fresh / prepared vessel. ATCC: "prepare fresh culture medium … label the culture vessels" before harvest; "adding concentrated cell suspensions to empty culture vessels can result in uneven attachment" (S4). |
| `reagent` | input material | optional | Transfer liquid itself when it is a reagent (dissociating solution, growth medium). ATCC example bindings in §6. |
| `consumable.tip` | input consumable | required at run | Disposable tip, single-use (S3 §§2.2, 3.4). Must be picked up before aspirate; Opentrons `new_tip` policies in §6. |
| `instrument.pipette` | input instrument | required at run | Piston-stroke air-cushion pipette (S3 §1.1). Capability requirements in §10. |
| `waste` | output (implicit) | conditional | Discarded medium, used tips. ATCC step 1 "Remove and discard the culture medium"; Opentrons trash / trash bin / waste chute (S1 "Blow out", S2 "Trash tips"). Exact waste path is protocol-defined — container identity UNKNOWN to this module. |

## 5. Sample transitions

| # | From → to | Effect |
|---|---|---|
| T1 | `source_sample --volume--> destination_sample` | Destination gains the transferred volume; source loses it. Volumes are example-bound at run time (e.g. S4: 6–8 mL suspension collected, aliquots dispensed). No concentration change is implied by this module. |
| T2 | `reagent --> destination_sample` | Reagent becomes (part of) destination sample (e.g. 2–3 mL dissociating solution added; 6–8 mL growth medium added to suspension) (S4). |
| T3 | `destination_sample --mix--> destination_sample` | Optional in-place homogenisation (`mix_after`, `mix`); identity unchanged, uniformity changed (S1 "Mix", S2 "Mix after"). |
| T4 | `source_sample --mix--> source_sample` | Optional pre-aspirate homogenisation (`mix_before`) (S2). `consolidate()` ignores `mix_before` (S2). |
| T5 | `consumable.tip: clean --> used` | Tips are single-use (S3); Opentrons tip policies (`once`/`always`/`never`/`per source`/`per destination`) control reuse across wells within a command (S2). |
| T6 | `source_sample --> waste` | Discard path (e.g. spent medium removed in S4 step 1). Volume accounting for waste is protocol-level, UNKNOWN to this module. |

## 6. Parameters — only where sourced

Manual reference behaviour first; automation-only extras flagged [AUTO].

| Parameter | Type | Unit | Source | Required at | Constraints (sourced only) |
|---|---|---|---|---|---|
| `volume` | float or list of float | µL | S2 (`transfer` signature) | run | Must be within fitted pipette model range. Family span 0.1 µL–10 mL depending on model (S3 §1.1); per-model ranges/increments in the instrument file (S3 §6). `0` volume skips that pair (search-sighted Opentrons reference; behaviour table in S2 — treat exact zero-skip as secondary, not verified in fetched excerpt → confirm before encoding). |
| `source` / `dest` | well / list of wells | — | S2 | run | `dest` count may exceed `source` count (multi-dispense); exact 1:1/1:N/N:1 expansion rules per command are S2 "Sources and Destinations" page, NOT fetched here → UNKNOWN, do not encode. |
| `new_tip` | enum | — | S2 | run (optional; default `"once"` per S2) | One of `"once"`, `"always"`, `"never"`, `"per source"`, `"per destination"`. `"once"`/`"always"` require an associated tip rack and no tip already attached, else error; `"never"` requires a tip already attached, else error (S2). `distribute`/`consolidate` pick up only one tip even with `"always"` (S2 example). |
| `mix_before` | tuple (repetitions, volume) | count, µL | S2 | optional | e.g. `(3, 50)` mixes 50 µL 3× before each aspiration (S2). Ignored by `consolidate()` (S2). |
| `mix_after` | tuple (repetitions, volume) | count, µL | S2 | optional | e.g. `(3, 50)` mixes 50 µL 3× after each dispense (S2). `distribute()` ignores `mix_after` (S2). |
| `blow_out` | boolean | — | S2 | optional | Opt-in. MUST also be `True` for `blowout_location` to take effect on `transfer()` (S2, new in v2.8). |
| `blowout_location` | enum | — | S2 | optional (with `blow_out=True`) | `transfer()`: `"trash"`, `"source well"`, `"destination well"`; `distribute()`: `"trash"`, `"source well"`; `consolidate()`: `"trash"`, `"destination well"`. Default trash; shifts to source/destination well if tip already held liquid before the command with `new_tip="never"` (S2). |
| `disposal_volume` | float | µL | S2 | optional (`distribute` only) | Default = pipette minimum volume; `0` skips extra aspiration/blow-out (S2). `transfer()` aspirates no extra liquid when set but performs a very small blow-out after each dispense; `consolidate()` ignores it entirely (S2). |
| `air_gap` | float | µL | S1, S2 | optional | Aspirated air to keep liquid off tip bottom (S2 purpose note). `transfer()`: air gap after each aspiration, pipette empty after dispense; `distribute()`: also after dispense if not empty; `consolidate()`: after each aspiration, may stack multiple gaps (S2 table). If air gap would exceed pipette max, command splits via tip-refilling strategy (S2 example). |
| `touch_tip` (complex) | boolean | — | S2 | optional | `True` = touch after every aspirate and dispense incl. tip-refill aspirations (S2). For one-sided or tuned touches use building-block `touch_tip` (S2). |
| `touch_tip` building-block tuning [AUTO] | `radius` (fraction) XOR `mm_from_edge` (mm); `speed` mm/s; `v_offset` mm | various | S1 | optional | Speed default 60, min 1, max 80 mm/s (S1). `radius` and `mm_from_edge` mutually exclusive; do NOT set `radius > 1.0` or negative `mm_from_edge` (wall-crash risk) (S1). API ≥ v2.28 errors on `touch_tip` in reservoirs/trash/6- or 24-well plates (S1). |
| `trash` (legacy) / `trash_location` (liquid-class) [AUTO] | boolean / labware ref | — | S2 | optional (default `True` = drop in trash) | `trash=False` returns tips to rack (S2). Liquid-class commands use `trash_location` (e.g. trash bin) (S2, new in v2.24). |
| `rate` / `flow_rate` [AUTO] | multiplier / float | — / µL/s | S1 | optional | `rate` multiplies default flow rate (best practice ≤ 3×); `flow_rate` sets absolute µL/s; mutually exclusive — specifying both raises (S1). Applies to `aspirate`, `dispense`, `air_gap` (S1). Absolute `flow_rate` added v2.24 (S1). |
| `push_out` | float | µL | S1 | optional | Extra plunger travel past aspirate start on `dispense()`; Flex pipettes push out by default when fully emptying the tip (volumes depend on pipette+tip — table values NOT verified here → UNKNOWN); `push_out=0` disables; new in v2.15 (S1). Manual analogue: stroke past first stop = blow-out (S3 §1.1). |
| `well_bottom_clearance.aspirate` / `.dispense` [AUTO] | float | mm | S1 | optional | Default 1 mm above well-bottom centre when given a bare well (S1). Settable per direction (S1 examples set 2 mm). |
| `mix` repetitions/volume/delays [AUTO] | int / float / float | count / µL / s | S1 | optional | `mix(repetitions, volume, location)`; `aspirate_flow_rate`, `dispense_flow_rate`, `aspirate_delay`, `dispense_delay`, `final_push_out` added v2.24; pipette stays in well during mix since v2.3; `dynamic_mix` (Flex only, v2.27) adds start/end locations (S1). |
| `pre_wet` (manual) | count | — | S3 | conditional | Pre-wet tip 1–3× for liquids differing from water; verify by weighing (S3 §§2.2, 5.5.1). No automated equivalent parameter verified here → UNKNOWN mapping. |
| ATCC example bindings (NOT defaults) | float | mL / % / min | S4 | run (protocol-level) | 75-cm² flask: rinse 5 mL dissociating solution then remove; add 2–3 mL; check every 5 min; trypsin 0.05–0.25 %, 10–15 min, 37 °C optional; add 6–8 mL growth medium; optional centrifuge 5 min at 125 × g; seed vessels with ~12–15 mL medium per 75 cm²; seed density 10³–10⁴ cells/cm². Scale proportionally for other vessels (S4). |

## 7. Preconditions

1. A clean compatible tip is attached; `new_tip="never"` requires a tip already on, `"once"`/`"always"` require a tip rack and no tip on (S2). Tips single-use (S3).
2. Requested `volume` is within the fitted pipette model's range and above its minimum/disposal accounting (family 0.1 µL–10 mL per model, S3; Opentrons tip-refilling splits over-capacity commands, S2).
3. Source holds ≥ requested volume (+ `disposal_volume` + `air_gap` headroom where used) (S2 capacity/refill behaviour).
4. Destination has headroom for the dispense (implied by S2 refill/capacity logic; exact deck/labware capacity UNKNOWN to this module).
5. Pipette is calibrated/adjusted for the liquid class in use; factory adjustment is for aqueous-like liquids, user (temporary) adjustment + gravimetric verification per EN ISO 8655-2/-6 is required for deviating liquids, high altitude, or changed lower parts (S3 §§4.1, 5.4–5.5 via instrument file).
6. For cell culture: vessels pre-labelled, fresh medium prepared, pH equilibrated (S4 "Prepare medium"); work aseptically. Enclosure class (e.g. BSC class) is UNKNOWN from verified sources here (S4 fetched text states no cabinet class).
7. [AUTO] Flex-only presence/height gates (`require_liquid_presence`, `measure_liquid_height`, `load_liquid`) require a clean dry tip picked up immediately before and aspirate/drop immediately after (S1).

## 8. Completion checks

| Check | How | Source |
|---|---|---|
| Tip emptied, no hanging droplet | Visual; touch-tip to knock off clinging droplets (S1 "Touch tip"); manual blow-out stroke past first stop (S3 §1.1) | S1, S3 |
| Full dispense accepted by API | v2.17+: `dispense(volume)` greater than aspirated raises instead of silently dispensing `current_volume` (S1 "Dispense" note) | S1 |
| Gravimetric verification (offline QC) | Weigh delivered distilled water (ISO 3696 quality-3) per ISO 8655-6 reference procedure, 10 determinations per test volume (S3 p. 31 test conditions). Pass = within manufacturer systematic/random limits for the fitted model (S3 §6 via instrument file) | S3 |
| [AUTO] Liquid presence / height | `detect_liquid_presence()` (bool, no error on empty) vs `require_liquid_presence()` (errors/pauses on empty); `measure_liquid_height()` for meniscus-relative pipetting (S1) | S1 |
| Cell-culture outcome | Suspension ≥ 95 % single cells on inverted-microscope check, else more (gentle) pipetting (S4 step 4); reattachment + active growth checked next day (S4 "Re-incubate") | S4 |

## 9. Failure modes

| Failure | Cause | Detection | Mitigation + source |
|---|---|---|---|
| Volume error (systematic/random out of limits) | Wrong model for volume; unadjusted liquid (density/viscosity/surface-tension/vapour-pressure deviation); temperature gradient pipette/tip/liquid; altitude; worn/incorrect tip | Gravimetric check per §8 (S3) | Fit correct model; user-adjust + gravimetric verify for the liquid (S3 §5.5); pre-wet 1–3×, equilibrate 20–25 °C ± 0.5 °C test climate (S3); use LoRetention tips for low-surface-tension liquids (S3 §3.2) |
| Droplet retention / incomplete dispense | Insufficient push-out/blow-out; wrong flow rate for viscosity | Visual; [AUTO] silent-short-dispense behaviour pre-v2.17 (S1) | Increase `push_out` (S1); enable `blow_out` with explicit `blowout_location` (S2); manual: full stroke past first stop (S3) |
| Cross-contamination between wells | Tip reused across sources/destinations | Procedural | `new_tip="always"` (or `"per source"`/`"per destination"` for `transfer`) + dry run to confirm pickup/drop behaviour (S2); single-use tips (S3); [AUTO] raise `well_bottom_clearance`, customise liquid class, or drop to building-block commands (S2 workaround list) |
| Tip-refill surprise (extra aspirate/dispense cycles) | `volume + disposal_volume + air_gap` exceeds tip capacity | Run-preview step list (S2 order-of-operations reference, not fetched → preview claim secondary) | Reduce `disposal_volume`/`air_gap` or split command; note `distribute` with refill blows out disposal volume per aspiration (S2) |
| `blowout_location` silently ignored | `blow_out` not set `True` (required since v2.8 for `transfer`) | Review params | Always pair `blowout_location` with `blow_out=True` (S2) |
| `disposal_volume` has no effect | Used on `transfer` (aspirates nothing extra) or `consolidate` (ignored) | Review params | Use only with `distribute`; on `transfer` expect only a very small blow-out (S2) |
| `mix_before`/`mix_after` silently ignored | `mix_before` on `consolidate`; `mix_after` on `distribute` | Review params | Restructure command (S2 notes) |
| Touch-tip crash / bent tip | `radius > 1.0` or negative `mm_from_edge`; touch in large labware | Visual / API error (≥ v2.28 in reservoirs/trash/6-/24-well) (S1) | Keep `radius ≤ 1.0`, `mm_from_edge ≥ 0`, lower speed, test in one well first (S1) |
| Cell clumping / poor reattachment / low viability | Over-harsh dissociation or over-vigorous pipetting; residual serum inhibiting trypsin; over-confluence; DNA gel from lysed cells; too-hard centrifugation | Microscope check (< 95 % single cells); next-day reattachment check (S4) | Rinse serum fully before trypsin; lowest effective trypsin, shortest exposure, 37 °C/prewarm to shorten it; do NOT hit/shake flask; gentle pipetting; gentle spin 5 min at 125 × g; DNase drop for DNA gels; chill suspension if delayed (S4 troubleshooting) |
| Chemical incompatibility / tip damage | Organic solvents / aggressive chemicals vs pipette materials; acetone cleaning | Material list (S3 §1.4) | Check compatibility first; never acetone/corrosives/abrasives on pipette (S3 §5.1) |

## 10. Instrument capability requirements

- Piston-stroke air-cushion pipette with settable volume and first-stop/blow-out
  stroke distinction (S3 §1.1).
- Compatible tip system matched to the model (colour/range/length pairings per
  S3 §6 via instrument file; 5/10 mL filter rules per S3 §§2.2, 3.2).
- Calibration regime: factory certificate voided by changing factory adjustment;
  user adjustment allowed with gravimetric verification per EN ISO 8655-2/-6;
  re-test after any lower-part exchange/reassembly (S3 §§5.4–5.5).
- Maintenance envelope: full-unit or removed-lower-part steam autoclave
  121 °C / 20 min / 1 bar overpressure (5/10 mL protection filter replaced
  after); surface wipe (70 % isopropanol / DNA-RNA agent; tip cone + ejector
  may see 4 % NaOCl then thorough demineralised-water rinse); temporary
  UV ≥ 254 nm tolerated (S3 §§5.2.1–5.2.2).
- Operating climate 5–40 °C, 10–95 % RH; gravimetric test climate 20–25 °C
  constant ± 0.5 °C (S3 §6.4 + p. 31).
- [AUTO] backends additionally provide: trash bin/waste chute (v2.16+),
  absolute `flow_rate` (v2.24+), meniscus-relative pipetting (v2.21–2.23),
  liquid presence/height sensing + `load_liquid` (v2.20–2.22), `dynamic_mix`
  and multi-location aspirate/dispense (v2.27), absolute blow-out `flow_rate`
  (v2.28) (S1, S2 version notes).

## 11. Where automated liquid handling differs (manual → Opentrons map)

| Manual concept | Automated counterpart | Note + source |
|---|---|---|
| Aspirate / dispense by hand to first stop | `aspirate(volume, location[, rate\|flow_rate])` / `dispense(...)` | Default 1 mm above well-bottom centre; tunable via `well_bottom_clearance` and `top()`/`bottom()`/`meniscus()` locations (S1) |
| Blow-out stroke | `blow_out(location[, flow_rate])` or complex-command `blow_out` + `blowout_location` | Location- and command-dependent table in §6 (S2); manual has no "trash" concept — waste path is protocol-defined |
| Final purge tuning | `push_out` on `dispense()` | Flex defaults to push-out when fully emptying tip; OT-2 has none (S1) |
| Tip touch-off / droplet knock-off | `touch_tip` (complex bool or tuned building-block) | Tunable radius/edge/speed; restricted labware ≥ v2.28 (S1) |
| Pre-wet 1–3× | No verified equivalent parameter | UNKNOWN mapping — do not assume `mix_before` equals pre-wetting (S2 `mix_before` is in-well mixing, not a wetting stroke) |
| Homogenise before/after | `mix_before` / `mix_after` / `mix()` / `dynamic_mix()` | With the `consolidate`/`dist substrates` ignore-rules in §6 (S1, S2) |
| One-to-many / many-to-one transfers | `transfer` (general) vs `distribute` (1→N + disposal volume) vs `consolidate` (N→1) | Different blow-out, disposal, mix, and tip-pickup semantics per command — see §6 (S2). Choice of command is a protocol decision, not interchangeable |
| Tip-change discipline | `new_tip` + `trash`/`trash_location` | Manual single-use rule (S3) maps to `"always"`-family policies; `"once"`/`"never"` are automation conveniences with strict preconditions (S2) |
| Volume bookkeeping | `load_liquid` + `measure_liquid_height` + meniscus targeting [Flex only] | No manual equivalent; requires clean dry tip discipline (S1) |
| Liquid-class behaviour | `transfer_with_liquid_class` + Opentrons-verified liquid-class definitions | Liquid class determines mix/blow-out/air-gap/disposal behaviour (S2); manual equivalent is user adjustment per liquid (S3 §5.5) |

## 12. Explicit UNKNOWNs

1. Per-model volume/increment/error-limit numbers — deferred to the instrument file (`k2-instruments/manual-pipette-eppendorf-research-plus.md`); not duplicated here to avoid drift.
2. Full ISO 8655:2022 text (parts 1/2/6/7): paywalled; 2022 deltas beyond S3's cited test conditions (condition windows, tip-exchange-per-replicate, reporting fields, Part 7 routine-check vs Part 6 reference-method boundary) are UNKNOWN from verified sources (S5 secondary only).
3. S2 "Sources and Destinations" expansion rules (1:1/1:N/N:1) and "Order of Operations" step sequences — pages referenced but not fetched; do not encode.
4. Opentrons pipette max volumes, default flow rates (µL/s), Flex default/max push-out table — referenced by S1 but numeric table not fetched; UNKNOWN.
5. Zero-volume skip behaviour of `transfer()` — search-sighted only; treat as unconfirmed.
6. Manual↔auto equivalence of pre-wetting vs `mix_before`, and of manual blow-out air volume vs `push_out` µL — UNKNOWN; no conversion factor sourced.
7. Waste-container identity, transfer duration/time scope, shear-stress magnitudes, carry-over volumes — UNKNOWN; ATCC gives only outcome-level limits (≥ 95 % single cells, gentle handling, 5 min at 125 × g).
8. 16-/24-channel and Move It variants, current order numbers, in-vivo applicability — per instrument file §6, UNKNOWN.

## 13. Verified vs not (no SourceChecked claim)

- **I verified from fetched text in this task:** Opentrons building-block set and their key parameters/version notes (S1); complex-command tip/mix/disposal/air-gap/blow-out/trash semantics incl. per-command location tables and the `blow_out=True` gating rule (S2); ATCC harvest steps, example volumes, culture conditions, and troubleshooting rules quoted in §6/§8/§9 (S4).
- **I did NOT verify in this task:** the Eppendorf PDF bytes (rely on the instrument file's prior verification — S3); any ISO 8655 full-text clause (S5 is secondary context only); Opentrons Sources/Destinations and Order-of-Operations pages; numeric pipette/flow/push-out tables.
- **Status restated:** `review_status: research_draft`. No SourceChecked claim is made.
