# Automated liquid handling — InstrumentRecords (research draft)

`review_status: research_draft` — NOT source-checked. No `ReviewRecord` exists.
Do NOT claim `SourceChecked`. Every numeric value carries a source locator;
anything not verified from a fetched source is marked `search-summary-only`
or UNKNOWN with a reason (`source_missing` / `configuration_dependent` /
`method_dependent` / `device_state` / `not_applicable`).

Retrieval date for all sources: 2026-11-05.

Scope: wet-lab biological liquid handling only. No clinical/patient studies,
epidemiology, bioinformatics, image analysis, statistics, or data pipelines.
Operation semantics (aspirate/dispense/mix/blow-out/touch-tip/air-gap,
complex-command behaviour) are NOT duplicated here — see
`docs/research/k2-operations/liquid-transfer.md` (reference implementation:
manual piston-stroke pipette; automation mapping: Opentrons building-block +
complex commands, §§6/11) and `catalog/operations/liquid-transfer.v1.json`.
This file records only instrument platform vs. installed-configuration
capability: pipettes, labware, tip handling, deck constraints, liquid-class
hooks, and raw run outputs.

Granularity declaration: this file contains three **InstrumentRecord**
objects (platform profiles, not installed devices, not Run state):
`auto-liquid-opentrons-flex` (Opentrons Flex platform),
`auto-liquid-opentrons-ot2` (Opentrons OT-2 platform),
`auto-liquid-hamilton-star-class` (generic Hamilton Microlab STAR-line class).
No new Atomic Operation is proposed (see Reuse Map).

## 0. Reuse Map (inventory taken 2026-11-05)

Catalog inventory (`catalog/operations/*.json`): `liquid-transfer.v1.json`,
`washing.v1.json`, `incubation.v1.json`, `sample-aliquot.v1.json`,
`absorbance-measurement.v1.json`. Operations-research inventory
(`docs/research/k2-operations/`): `liquid-transfer.md`,
`record-liquid-transfer.md`, `washing-and-sampling.md`, plus method templates
(qPCR, cell culture, ELISA/WB, flow/IF, biofilm) — none redefined here as
evidence; referenced only for reuse mapping per workflow instruction.

Concrete wet-lab illustration used for mapping (NOT evidence):
PCR / mastermix setup assembled on an automated handler — source/method
samples + mastermix combined into a reaction plate, then handed to an
on-deck or off-deck thermocycler. Repository:
Opentrons Protocol Library,
https://docs.opentrons.com/flex/protocols/library/ (`search-summary-only`;
protocol cards expose name, verification badges, time estimate, robot model,
"What you'll need", steps, deck map, liquids list — exact card/section not
paginated here) and versioned sources at
https://github.com/Opentrons/Protocols (each protocol = folder with
README.md + single `.py` + optional `fields.json`; `search-summary-only`).
No single named protocol version was pinned in this task; every step-level
volume/count below is therefore UNKNOWN (`source_missing`) and MUST NOT be
encoded as defaults.

| Illustrative step | Existing operation / template | Disposition |
|---|---|---|
| Aspirate / dispense / mix / blow-out / touch-tip / air-gap / push-out; `transfer` vs `distribute` vs `consolidate` choice | `liquid-transfer` (`catalog/operations/liquid-transfer.v1.json`; `k2-operations/liquid-transfer.md` §§6/11) | **Parameterized reuse.** All volume, rate, clearance, mix, blow-out-location, disposal-volume, new-tip parameters already belong to that module. No new operation. |
| Tip pickup / eject / return; `new_tip` once/always/never/per-source/per-destination; trash vs return | `liquid-transfer` T5 + §6 `new_tip`/`trash` rows | **Direct reuse** (automation conveniences with stated preconditions). Manual single-use rule maps to `always`-family policies. |
| Liquid presence / height check, meniscus-relative pipetting, `load_liquid` bookkeeping | `liquid-transfer` §8 completion checks (Flex-only gates) | **Direct reuse** as completion check / precondition, not an operation. Requires clean dry tip discipline (precondition, method/run-bound). |
| Mastermix volume arithmetic, run preview / deck-conflict preview | — | **ToolAction**, not an operation. Deterministic calculation/preview; never an Atomic Operation candidate. |
| Gripper labware transport (pipette-to-staging moves, module loading) | No existing operation in catalog inventory | **Missing → Custom Step** (boundary unstable, protocol-specific). Do NOT mint a `labware-transport` atomic module from this task. |
| Thermocycling of the assembled plate | Existing thermocycler / incubation records (outside this file) | **Out of scope**; referenced only as downstream handoff. |

No new Atomic Operation candidate is created: every liquid step is
expressible as `liquid-transfer` + parameters; the only gap (gripper
transport) is a Custom Step; calculations are ToolActions.

---

## InstrumentRecord 1 — Opentrons Flex (platform profile)

### 1A. Identity

| Field | Value |
|---|---|
| profile id (proposed) | `auto-liquid-opentrons-flex` |
| version | research draft v0 |
| class | automated liquid handler / gantry pipetting robot (platform) |
| Manufacturer / model | Opentrons Labworks Inc. / Flex, model `FLEX12SUV` (all Flex robots) |
| Configuration covered | Base platform: 1–2 pipette mounts + optional gripper + optional on-deck modules. Exact installed pipette(s), tip racks, modules, deck fixtures, gripper present/absent are configuration-dependent (§1E). NOT a single installed device. |

### 1B. Sources

| # | Source | Locator (retrieved 2026-11-05) | Reading depth |
|---|---|---|---|
| S1 | Opentrons docs, "Opentrons Flex: System Specifications" https://docs.opentrons.com/flex/system-description/specs/ | Tables: General specifications; Environmental specifications; Certifications; Model and serial numbers | fetched full text this task (`agent-verified` transcription) |
| S2 | Opentrons docs, "Opentrons Flex: Pipettes" https://docs.opentrons.com/flex/system-description/pipettes/ | Sections: Pipettes (mount occupancy); Pipette specifications (1-/8-/96-channel tables); calibration; tip rack adapter; partial tip pickup; sensors (capacitance/optical/pressure); firmware updates | fetched full text this task (`agent-verified` transcription) |
| S3 | Opentrons docs, "Python API: Deck Slots" https://docs.opentrons.com/python-api/deck-slots/ | Sections: Physical deck labels; API deck labels (Flex↔OT-2 equivalence table); Deck configuration (staging/trash/waste-chute); Deck conflicts | fetched full text this task (`agent-verified` transcription) |
| S4 | Opentrons docs, "Python API: Liquid Control" https://docs.opentrons.com/python-api/building-block-commands/liquids/ | Sections fetched: Aspirate (+by well/location, flow rates); Dispense (+by well/location, flow rates); Push out after dispense. Remainder (blow-out, touch-tip, mix, air-gap, presence/height sub-pages) NOT fetched here | partial fetch (`agent-verified` for sections seen; rest UNKNOWN `source_missing`) |
| S5 | Opentrons docs, "Protocol Library" https://docs.opentrons.com/flex/protocols/library/ + https://github.com/Opentrons/Protocols | Protocol cards, "What you'll need", steps, deck map, liquids list; repo layout (folder = README.md + `.py` + optional `fields.json`) | `search-summary-only`; no single protocol pinned |
| S6 | Opentrons docs, "Complex Command Parameters" https://docs.opentrons.com/python-api/complex-commands/parameters/ (via `k2-operations/liquid-transfer.md` S2) | `transfer`/`distribute`/`consolidate` tip/mix/disposal/blow-out/trash semantics | NOT re-fetched here; cited via operation record, treat as `search-summary-only` for this file |

### 1C. Capabilities

Platform values (agent-verified from S1/S2 unless flagged):

| Capability | Value / range / set | Conditions | Evidence |
|---|---|---|---|
| Footprint / weight | 87 × 69 × 84 cm (W×D×H); 88.5 kg (195 lb) | — | S1 General specifications table |
| Deck, working area | 12 ANSI/SLAS-compatible slots, pipette-accessible, coordinates A1–D3 | Slots removable, corner-clipped; reconfigurable via fixtures (§1E) | S1 + S3 Physical deck labels |
| Deck, staging area | 4 slots A4–D4, gripper-only; pipettes CANNOT reach | Requires staging-area fixture in deck configuration (robot ≥ v7.1.0); incompatible with powered module in same row of column 3 | S3 Staging area slots |
| Trash / waste fixtures | Movable trash bin (slots A1–D1, A3–D3); waste chute fixed at D3, 4 configs (cover × staging-adapter); 96-channel dispense/blow-out/drop or gripper labware-drop into chute requires cover REMOVED | API ≥ v2.16 must `load_trash_bin()` / `load_waste_chute()`; v2.15 single bin at A3 only | S3 Trash bin / Waste chute |
| Deck conflict gate | App/touchscreen blocks run start until physical-vs-protocol deck configuration matches | Flex robot system ≥ v7.1.0 | S3 Deck conflicts |
| Pipette mounts | Left + right mounts; 1-/8-channel occupy ONE mount; 96-channel occupies BOTH | 96-channel + second pipette cannot co-install | S2 Pipettes |
| Pipette models (6) | 1-ch 1–50 µL; 1-ch 5–1000 µL; 8-ch 1–50 µL; 8-ch 5–1000 µL; 96-ch 1–200 µL; 96-ch 5–1000 µL | Family span 1–1000 µL; see accuracy table per model+tip+volume | S2 Pipettes |
| Accuracy/precision (selected; full tables in S2) | 1-ch 1–50 µL: 1 µL/20 µL-tip 3.5 %CV ±8.0 %D … 50 µL/50 µL-tip 0.40 %CV ±1.25 %D; 1-ch 5–1000 µL: 1000 µL 0.15 %CV ±0.50 %D; 8-ch 1–50 µL: 1 µL up to 8 %CV ±10 %D, 50 µL 0.60 %CV ±1.25 %D; 8-ch 5–1000 µL: 1000 µL 0.15 %CV ±0.70 %D; 96-ch 1–200 µL: 1 µL up to 6 %CV ±10 %D, 200 µL 1.0 %CV ±1.0 %D; 96-ch 5–1000 µL: 1000 µL 1.5 %CV ±1.5 %D | Per pipette-type × tip-size × test-volume; "use smallest capacity tips that meet needs" (S2 Tip). Volume calibration NOT performed — positional calibration only (probe on magnetic holder) | S2 specification tables + Tip note + calibration section |
| Partial tip pickup | 96-ch: column / row / single tip; 8-ch: partial column (2–7 consecutive) or single tip; default = full head | Partial pickup: rack directly on deck, NOT in tip-rack adapter | S2 Partial tip pickup |
| 96-ch tip adapter | Ships with 4 aluminium tip-rack adapters; required for full-rack pickup (pull-on action protects deck) | Adapters hold Flex 50/200/1000 µL tip racks | S2 Tip rack adapter |
| Sensors | Capacitance (1-ch ×1; 8-ch on ch 1+8 = A1/H1; 96-ch on ch 1+96 = A1/H12); optical tip-presence (×1 per pipette, all heads); pressure (1-ch ×1; 8-ch ch 1+8; 96-ch ch 1+96 — other channels CANNOT detect liquid) | Capacitance used for probe/deck calibration; pressure for liquid presence/level/height | S2 Pipette sensors |
| Default pipetting geometry | Bare-well aspirate/dispense 1 mm above well-bottom centre; tunable via `well_bottom_clearance`, `top()`/`bottom()`/`meniscus()` | Meniscus targeting (Flex only, API v2.21–2.23) needs `load_liquid` volume or `measure_liquid_height` first; `end_location` multi-location aspirate/dispense Flex-only (v2.27) | S4 Aspirate/Dispense by well or location |
| Flow-rate control | Default µL/s per pipette; `rate` multiplier (best practice ≤ 3×) XOR absolute `flow_rate` (v2.24+); specifying both raises | Applies to aspirate/dispense/air-gap (per operation record S1/S2) | S4 flow-rate sections |
| Push-out | Flex pipettes push out by default when fully emptying tip (default/max depend on pipette+tip — table NOT fetched → UNKNOWN `source_missing`); `push_out=0` disables | v2.15+ | S4 Push out after dispense |
| Over-aspirate guard | API ≥ v2.17: `dispense(volume)` > aspirated raises (pre-v2.16 silently dispensed `current_volume`) | version-dependent | S4 Dispense note |
| Connectivity / compute | Wi-Fi 802.11ac/a/b/g/n dual-band; Ethernet 100 Mbps; USB 2.0 (9×A + 1×B); 2 MP camera; App on Win 10+, macOS 11+, Ubuntu 22.04+ | — | S1 General specifications |
| Power / environment | 100–240 VAC, 50–60 Hz, 1φ 4.0 A (115 V) / 2.0 A (230 V); mains ±10 % V, ±5 % f; TN-S; 6.3 A short-circuit; indoor only; +20 to +25 °C recommended; 40–60 % RH non-condensing; pollution degree 2; ≥ 20 cm wall clearance | Recommended (not validated sterile); enclosure = polycarbonate, NOT a sterile environment (`search-summary-only` enclosure note — S1 gives window composition only) | S1 tables |
| Certifications / identity | CE, ETL, FCC, ISO 9001 complete; IVD/GMP NOT certified/validated; serial on back sticker / touchscreen rear / App path Devices > Flex > Robot settings > Advanced | — | S1 Certifications + Model/serial |

### 1D. Container support

Status: `partial`. Verified only at the category level below; MUST NOT treat
any example list as a complete compatibility matrix.

- Deck accepts ANSI/SLAS-footprint labware, tip racks, modules, consumables
  in working-area slots (S1, S3). Exact per-slot/per-module/per-pipette
  compatibility matrix: UNKNOWN (`source_missing` — labware-library catalogue
  not enumerated here).
- Every item needs a labware definition JSON (schema v2, namespace/metadata,
  shape, well geometry, gripper offsets, stacking offsets, tip length, …);
  non-Library items are "custom" and need a Creator-made definition;
  small dimensional variation affects runs (`search-summary-only` via
  Labware Definitions/Concepts summaries — pages not fetched full text).
- Flex Tips / Flex Filter Tips recommended; OT-2 pipettes/tips NOT compatible
  with Flex (`search-summary-only` — product-page note, not verified in S1/S2
  fetched text).
- 96-channel full-rack pickup REQUIRES tip-rack adapter (S2, agent-verified);
  partial pickup forbids the adapter (S2, agent-verified).
- Reservoirs / trash / 6- or 24-well touch-tip restriction (API ≥ v2.28
  errors): cited via operation record, NOT re-fetched → `search-summary-only`.

### 1E. Configuration conditions (platform ≠ device)

- Installed pipette model(s) + mount occupancy determine volume span,
  accuracy row, and whether a 96-channel second mount is free (§1C).
- Presence/absence of gripper determines staging-area (A4–D4) usability and
  labware moves; pipettes alone cannot use column 4 (S3).
- Installed deck fixtures (staging slots A3–D3, trash bin position, waste
  chute ± cover ± staging adapter) must match the protocol's
  `load_*` calls or the deck-conflict gate blocks the run (S3).
- Powered on-deck modules (heater-shaker, temperature, thermocycler,
  magnetic block) occupy column-3 caddies and conflict with same-row staging
  slots (S3 example C3/C4).
- Per-pipette default/max push-out volumes, default flow rates (µL/s),
  liquid-class definitions (`transfer_with_liquid_class`), `load_liquid` /
  `measure_liquid_height` behaviour: UNKNOWN here (`source_missing` — S4
  remainder and liquid-class pages not fetched; do not encode).
- Calibration state, firmware sync, maintenance history: device state (§1G).

### 1F. Raw outputs

- Protocol source: versioned Python `.py` file (+ optional `fields.json`
  inputs); Protocol Designer JSON convertible to Python
  (`search-summary-only` via S5 adaptation docs).
- Run-time observables: deck-conflict pass/block (S3, agent-verified);
  per-command aspirate/dispense volumes and locations are protocol-level
  values, not instrument constants.
- Run artefacts (run log, deck map render, liquids list, supporting data):
  existence `search-summary-only` via S5; file schemas/fields UNKNOWN
  (`source_missing`).
- No scientific conclusion is produced by the handler itself: N/A (readout
  belongs to downstream assay, e.g. thermocycler/qPCR records).

### 1G. Actual-device-vs-profile boundary

This record is the Flex PLATFORM. It is NOT any installed unit: installed
pipette serials/models, tip stocks, module complement, deck-fixture layout,
gripper presence, firmware/robot-system versions, positional-calibration
results, service history, and actual ambient/lab conditions belong to
Run/device state and are UNKNOWN here (`device_state`). Optional items
(gripper, modules, chute, staging slots) MUST be written as conditions /
variants, never as present on every unit.

### 1H. Unknowns

| Item | Status | Reason |
|---|---|---|
| Per-pipette default/max push-out table, default flow rates (µL/s) | UNKNOWN | `source_missing` (S4 references tables not fetched) |
| Blow-out / touch-tip / mix / dynamic-mix / air-gap / liquid-presence parameter pages | UNKNOWN in this file | `source_missing` (S4 remainder not fetched; operation record covers semantics — do not re-encode) |
| Opentrons-verified liquid-class definitions + `transfer_with_liquid_class` behaviour | UNKNOWN | `source_missing` |
| Complete labware-library catalogue; per-labware/per-module compatibility; third-party tip/labware performance | UNKNOWN / `partial` container list | `source_missing` + `configuration_dependent` |
| Flex Tips dimensional / sterility / filter specs; 20/50/200/1000 µL tip volume ceilings | UNKNOWN | `source_missing` |
| Run-log / deck-map / liquids-list schemas; error-code / recovery export | UNKNOWN | `source_missing` |
| IVD/GMP-adjacent validation, sterile-enclosure performance, HEPA/UV accessory | UNKNOWN (base unit not sterile; NOT IVD/GMP per S1) | `source_missing` / `not_applicable` to base platform |
| Installed pipettes/modules/gripper/fixtures, calibration, firmware, service state | UNKNOWN | `device_state` |
| Named Protocol Library protocol pinned to a version/DOI with step-level volumes | UNKNOWN | `source_missing` (no single protocol opened; S5 generic only) |

---

## InstrumentRecord 2 — Opentrons OT-2 (platform profile)

### 2A. Identity

| Field | Value |
|---|---|
| profile id (proposed) | `auto-liquid-opentrons-ot2` |
| version | research draft v0 |
| class | automated liquid handler / gantry pipetting robot (platform, compact) |
| Manufacturer / model | Opentrons Labworks Inc. / OT-2 |
| Configuration covered | Base platform with left+right pipette mounts (single- and/or 8-channel GEN2), 11-slot deck + fixed trash. Exact installed pipette pair, tip stocks, optional modules: configuration-dependent (§2E). |

### 2B. Sources

| # | Source | Locator (retrieved 2026-11-05) | Reading depth |
|---|---|---|---|
| S7 | Opentrons docs, "OT-2: System Specifications" https://docs.opentrons.com/ot-2/system-description/specs/ | Full table: dimensions, weight, deck slots, Wi-Fi/Ethernet/USB, camera, power supply/consumption, frame/windows, ventilation, PC requirements | fetched full text (`agent-verified`) |
| S8 | Opentrons docs, "OT-2: Pipettes" https://docs.opentrons.com/ot-2/pipettes/ | Sections: GEN1 vs GEN2 identification; install/replace; pickup/eject mechanism; single-channel table; multi-channel (8-ch) table; Tip + Note callouts | fetched full text (`agent-verified`) |
| S3 | Same deck-slots page as §1B | OT-2 numeric labels 1–11 + fixed trash; Flex↔OT-2 equivalence row | fetched full text (`agent-verified`) |
| S9 | OT-2 product/brochure pages (secondary) https://opentrons.com/robots/ot-2 ; brochure PDF | Pipette configuration mix (2×8-ch / 2×single / mixed); 1–1000 µL span; ANSI/SLAS + up-to-50 mL tube compatibility; 22 s 96-well fill claim | `search-summary-only` (summaries only, not fetched) |

### 2C. Capabilities

| Capability | Value / range / set | Conditions | Evidence |
|---|---|---|---|
| Footprint / weight | 66 × 57 × 63 cm; 48 kg (105 lb) | Bench placement, ≥ 20 cm (8") ventilation clearance | S7 |
| Deck | 11 ANSI/SLAS slots (working area) + 1 fixed trash slot (back-right) | Numeric labels 1 (front-left) … 11 (back-centre); trash fixed, not a labware slot | S7 + S3 Physical deck labels |
| Pipette mounts | Left + right carriage mounts; one single- or multi-channel pipette per mount (max 2 pipettes) | GEN2 current; GEN1 discontinued/obsolete (black housing, drop logo) vs GEN2 (longer, black/silver, marked GEN2 + capacity) | S8 GEN1/GEN2 + Installing |
| GEN2 models | Single: P20 (1–20 µL), P300 (20–300 µL), P1000 (100–1000 µL); Multi (8-ch): P20 (1–20 µL), P300 (20–300 µL). Family span 1–1000 µL | "For best results, use smallest capacity tips that meet needs" (S8 Tip) | S8 model list + Tip |
| Accuracy/precision single (agent-verified rows) | P20: 1 µL ±15 %/0.15 µL, CV 5 %/0.05; 10 µL ±2 %/0.2, CV 1 %/0.1; 20 µL ±1.5 %/0.3, CV 0.8 %/0.16. P300: 20 µL ±4 %/0.8, CV 2.5 %/0.5; 150 µL ±1 %/1.5, CV 0.4 %/0.6; 300 µL ±0.6 %/1.8, CV 0.3 %/0.9. P1000: 100 µL ±2 %/2.0, CV 1 %/1; 500 µL ±1 %/5.0, CV 0.2 %/1; 1000 µL ±0.7 %/7.0, CV 0.15 %/1.5 | Per-pipette test-volume rows; flow-rate table lives on a characteristics page NOT fetched → defaults UNKNOWN | S8 single-channel table |
| Accuracy/precision 8-ch (agent-verified rows) | P20 8-ch: 1 µL ±20 %/0.2, CV 10 %/0.1; 10 µL ±3 %/0.3, CV 2 %/0.2; 20 µL ±2.2 %/0.44, CV 1.5 %/0.3. P300 8-ch: 20 µL ±10 %/2.0, CV 4 %/0.8; 150 µL ±2.5 %/3.75, CV 0.8 %/1.2; 300 µL ±1.5 %/4.5, CV 0.5 %/1.5 | 8 channels, no independent per-channel volume control implied by head design (`search-summary-only` for the "no independent control" phrasing — table verified, control claim not in fetched text) | S8 multi-channel table |
| Tip attach/detach | Press-on pickup (force scales with tip count); ejector pushes tips off / returns to rack | Positional calibration only; NO volume calibration before use | S8 pickup section + Note |
| Calibration | Positional only (see S8 Note; install/calibrate procedure on Installation page not fetched) | — | S8 Note |
| Modules (optional) | Magnetic, Temperature, Thermocycler, Heater-Shaker, HEPA filter (`search-summary-only`) | Configuration-dependent; load-name migration needed when adapting OT-2→Flex protocols | S9 summaries |
| Power/compute/env | External Mean Well GST220A; idle ~90–120 W, active ~100–180 W (movement/lights/pipette-dependent); Wi-Fi 2.4 GHz b/g/n, Ethernet 100 Mbps, USB 2.0, 2 MP camera; steel/aluminium frame, polycarbonate window; App on Win 10+, macOS 11+, Ubuntu 22.04+ | Power varies with run state — not a fixed rating | S7 |
| Cross-platform notes | Flex pipettes/tips NOT compatible with OT-2 and vice versa (`search-summary-only` — compatibility note seen only in Flex-side summaries) | Platform lock-in for pipette/tip stocks | S9-side summary; re-verify before procurement |

### 2D. Container support

Status: `partial`. 11-slot ANSI/SLAS working area + fixed trash verified
(S7/S3). Reagent-agnostic / ANSI/SLAS-plate + up-to-50 mL-tube claims are
`search-summary-only` (S9). Per-labware/per-module/per-pipette matrix,
third-party tip performance, and tube-rack adapters: UNKNOWN
(`source_missing`). MUST NOT present the 11-slot count as a labware list.

### 2E. Configuration conditions

- Installed pipette pair (2× single / 2× 8-ch / mixed) fixes mounted volume
  spans and applicable accuracy rows; empty mount = lost channel option.
- Optional modules and their slot occupancy are per-device choices; adapting
  an OT-2 Python protocol to Flex requires `robotType: "Flex"`, apiLevel
  ≥ 2.15, pipette/tip-rack renaming, coordinate slot labels, and module
  load-name updates (`search-summary-only` — adaptation page summary, not
  fetched).
- Default flow rates, liquid classes, `push_out` (OT-2 has none per operation
  record — NOT re-verified here): UNKNOWN in this file (`source_missing`).
- Calibration/maintenance state: device state (§2G).

### 2F. Raw outputs

- Versioned Python protocol (`.py`, `def run(context):`) + optional
  `fields.json`; Protocol Designer drag-and-drop source
  (`search-summary-only` via S9/repo summaries).
- No per-instrument numeric readout beyond run execution; assay readout is
  downstream (N/A for this profile).
- Run-log / error-export schemas: UNKNOWN (`source_missing`).

### 2G. Actual-device-vs-profile boundary

Platform profile only. Installed pipette pair + GEN generation, tip stocks,
module complement, calibration results, service history, lab ambient: Run /
device state, UNKNOWN here (`device_state`). GEN1 units still in the field
MUST be recorded as a variant (obsolete, less accurate, unsupported) — never
merged into GEN2 rows.

### 2H. Unknowns

| Item | Status | Reason |
|---|---|---|
| Default flow rates (µL/s) per OT-2 pipette | UNKNOWN | `source_missing` (characteristics page not fetched) |
| OT-2 liquid-class / `load_liquid` / presence-sensing support | UNKNOWN | `source_missing` (OT-2 lacks Flex pressure/meniscus stack — exact boundary not verified here) |
| Per-labware compatibility matrix; third-party tip/labware performance | UNKNOWN | `source_missing` |
| Module load names, thermocycler/heater-shaker integration details | UNKNOWN | `source_missing` (`search-summary-only` existence only) |
| 22 s 96-well fill / throughput claims | UNKNOWN (do not use) | `search-summary-only` brochure claim, no test conditions verified |
| Installed pair, calibration, service, firmware | UNKNOWN | `device_state` |

---

## InstrumentRecord 3 — Hamilton Microlab STAR line (generic class)

### 3A. Identity

| Field | Value |
|---|---|
| profile id (proposed) | `auto-liquid-hamilton-star-class` |
| version | research draft v0 |
| class | automated liquid handler / configurable multi-channel pipetting platform (generic class, NOT one model) |
| Manufacturer / line | Hamilton Company / Microlab® STAR Line (STAR / STARlet / STARplus siblings; STAR IVD variant exists) |
| Configuration covered | Class-level envelope only: 1 mL / 5 mL / X1 channel counts, MPH options, deck-track envelope, CO-RE tool family, VENUS software, monitoring technologies. EVERYTHING countable (channel count, heads, deck size, options fitted) is configuration-dependent (§3E). |

### 3B. Sources

| # | Source | Locator (retrieved 2026-11-05) | Reading depth |
|---|---|---|---|
| S10 | Hamilton product page, "Microlab STAR Line" https://www.hamiltoncompany.com/microlab-star | Sections fetched: platform intro; channel/MPH options ("up to sixteen 1 mL / eight 5 mL / 96 + 384 MPH"); deck "up to 55+ ANSI/SLAS positions" / "up to 66 tracks"; CO-RE II; MAD / Dual LLD / TADM / ADC / LFC / DPS / Autoload / CO-RE Gripper + Plate Gripper / capacitive LLD / clot detection / GUI; VENUS software; IVD/HEPA optional notes | partial fetch lines 1–300 of 646 (`agent-verified` for sections seen; remainder UNKNOWN) |
| S11 | Hamilton spec sheet, "Microlab STAR" https://www.hamiltoncompany.com/product-specs/Microlab%20STAR | Tables: System Dimensions (H 90.3 cm / W 1.66 m / D 78.1 cm–1.01 m); General (145 kg); Transport Tools (CO-RE Gripper + IPG optional); Platform (IVDR No; 55 ANSI/SLAS; 180° integration); Sample loading/tracking (Auto continuous, Manual standard, 1D/2D Autoload optional); Pipetting Options (1 mL ≤ 16, 5 mL ≤ 8, X1-1000 µL ≤ 16, MagPip n/a, MPH 96 and/or 384, range 1 µL–5000 µL "Depending on Configuration") | fetched text (`agent-verified`) |
| S12 | Hamilton STAR brochure / VANTAGE CO-RE summaries (secondary) https://info.hamiltoncompany.com/view/449349329 ; VANTAGE spec PDF | 1000 µL channels 1.0–1000 µL + 5 mL channels 50–5000 µL; asymmetric spreading; CO-RE Gripper/FlipTube/Suction tools; STARlet 30 / STAR 54 deck tracks; STAR < 65 dBA; operating +15…+35 °C (JP STAR-line page) | `search-summary-only` (not fetched full) |
| S13 | CO-RE 96 Probe Head verification procedure (secondary) https://www.laboratory-equipment.com/…Hamilton…verification-procedure.pdf | Gravimetric (5-digit balance) + photometric verification, 3 pipettings × 96 channels, jet vs surface mode split at 50 µL, 20 ± 2 °C / 50 ± 5 % RH, wind shield, ≥ 1 h warm-up | `search-summary-only` (method-existence only; acceptance numbers NOT transcribed) |

### 3C. Capabilities (class envelope — every row is conditional on fitted configuration)

| Capability | Value / envelope | Conditions | Evidence |
|---|---|---|---|
| System size / weight | H 90.3 cm × W 1.66 m × D 78.1 cm–1.01 m; 145 kg | Depth range = configuration-dependent | S11 System Dimensions + General |
| Deck envelope | STAR spec row: 55 ANSI/SLAS positions; marketing: "up to 55+" / "up to 66 tracks"; integration 180° left+right | STAR vs STARlet vs STARplus deck-track counts differ (30/54/71 cited `search-summary-only` via S12); positions hold Hamilton + third-party consumables/devices | S11 Platform; S12 for sibling split |
| Pipetting channels | 1 mL CO-RE ≤ 16 ch; 5 mL CO-RE ≤ 8 ch; Fluid-Motion X1-1000 µL ≤ 16 ch; MagPip n/a; MPH 96 and/or 384 optional | Counts are maxima ("Up To"); fitted count per installation | S11 Pipetting Options |
| Volume envelope | 1 µL – 5000 µL per channel, "Depending on Configuration" | Low end needs 1 mL/X1 channels; high end needs 5 mL channels; per-channel/per-tip accuracy rows NOT transcribed here → UNKNOWN | S11 Pipetting Range fn 1; S12 for 1.0–1000 / 50–5000 splits (`search-summary-only`) |
| Tip attachment | CO-RE / CO-RE II compressed-o-ring expansion: tight, aligned, airtight lock; gentle aerosol-free ejection | Channel-specific; tools (Gripper, FlipTube, Suction) ride CO-RE channels | S10 CO-RE II + ejection sections |
| Monitoring stack (existence agent-verified, thresholds UNKNOWN) | MAD (per-channel aspiration verification); Dual LLD — capacitive (aqueous/conductive) + pressure (non-conductive/viscous/volatile); TADM real-time aspirate+dispense pressure trace vs expected + digital audit trail; ADC anti-droplet dosing compensation; LFC layered-liquid interface detection; clot detection; capacitive LLD via black conductive CO-RE tips | Feature availability is per-channel/per-configuration; pressure curves / expected-value bands not verified → do not encode | S10 Featured Technologies |
| Motion / tools | DPS simultaneous multi-channel pipetting incl. non-uniform patterns; Fluid Motion drive; CO-RE Gripper (labware transport on pipette channels) + Internal Plate Gripper optional; iSWAP/off-deck integration; multi-flex modules (30+ `search-summary-only`) | Optional tools fitted per installation | S10 transport + DPS sections; S11 Transport Tools |
| Sample tracking | Automatic continuous loading + manual standard; Autoload 1D/2D barcode ID optional (carriers, tips, tubes, plates, troughs; LIMS handoff) | Scanners optional, not base | S11 Sample Loading; S10 Autoload |
| Software | VENUS: drag-and-drop + Power Steps for routine pipetting; GUI run control; LIMS bidirectional; 21 CFR Part 11 tooling (`search-summary-only`) | Version/licence per installation | S10 VENUS + Explore §4; S12 for Part 11 (`search-summary-only`) |
| Contamination control | UV light + HEPA hood optional; Deckwatch camera + CELSORA T/RH monitoring optional (`search-summary-only`) | Optional; base IVDR compliance = No (S11) | S10 Optional Process Integrity; S11 IVDR row |

What is deliberately NOT in this table: per-volume accuracy/precision
(%D/CV) numbers, per-tip-volume pairings, LLD/TADM pressure thresholds,
pipetting speeds, deck-layout drawings, VENUS step parameters — all UNKNOWN
(`source_missing`; S13 acceptance values never transcribed).

### 3D. Container support

Status: `partial`. 55 ANSI/SLAS positions (STAR row) verified (S11);
SBS-microplate 1–1536-well and common-tube support is `search-summary-only`
(S12). Carrier/adapter matrix (which tubes/plates per carrier, per-channel
spreading limits, MPH plate compatibility), third-party integration frames,
and lid-handling limits: UNKNOWN (`source_missing` +
`configuration_dependent`). MUST NOT present "55 positions" as a labware list.

### 3E. Configuration conditions (class ≠ installation)

- Channel census (how many 1 mL / 5 mL / X1; MPH 96 and/or 384 fitted or
  absent) fixes the installation's volume envelope and throughput; quote
  only the fitted subset, never the class maxima, for a real device.
- Deck size/track count is a sibling-model choice (STARlet/STAR/STARplus);
  device-integration sides, Autoload 1D/2D, grippers, UV/HEPA, Deckwatch/
  CELSORA are all optional — each MUST be a variant flag.
- CO-RE tip family stocked (volumes, conductive-black vs standard, sterile
  ISO 11137 options) and carrier complement are installation choices
  (`search-summary-only` tip-range hints via S12; sterile/quality-system
  claims not verified).
- Liquid-class / TADM-reference-curve tuning per reagent (aqueous vs
  viscous/volatile), LLD mode selection, ADC compensation: method- and
  installation-dependent; no parameter transcribed → UNKNOWN
  (`method_dependent` + `source_missing`).
- Operating climate (S12: +15…+35 °C cited `search-summary-only`; storage/
  transport, humidity, noise < 65 dBA likewise) needs the installation
  manual before use.

### 3F. Raw outputs

- VENUS run control artefacts + TADM per-channel pressure traces with
  digital audit trail: existence agent-verified (S10 TADM/MAD sections);
  file formats, export paths, LIMS message schemas: UNKNOWN
  (`source_missing`).
- Gravimetric/photometric verification records (balance + photometer +
  plate-shaker procedure, S13): method-existence `search-summary-only`;
  acceptance tables and per-installation verification results: UNKNOWN.
- No scientific conclusion inheres in the handler: N/A (downstream assay owns
  readout).

### 3G. Actual-device-vs-profile boundary

This is a GENERIC CLASS, two removes from a bench: (1) sibling model +
fitted channels/heads/options, (2) installation state (carriers, tips,
VENUS version/project, TADM references, calibration, PM history, lab
ambient, LIMS binding). All of (1)–(2) are UNKNOWN here
(`configuration_dependent` + `device_state`). NEVER present class maxima
(16 ch, 5000 µL, 55+ positions) as one machine's capability.

### 3H. Unknowns

| Item | Status | Reason |
|---|---|---|
| Per-volume accuracy/precision tables; per-tip pairings | UNKNOWN | `source_missing` (S13 values not transcribed; S12 CV hints are summary-only) |
| TADM/MAD/LLD/ADC/LFC thresholds, pressure bands, alarm logic | UNKNOWN | `source_missing` |
| VENUS project parameters, Power-Step definitions, file/export schemas | UNKNOWN | `source_missing` |
| Carrier/adapter/labware matrix; MPH partial-plate rules; tube gripping limits | UNKNOWN | `source_missing` + `configuration_dependent` |
| Tip catalogue (volumes, conductive, sterile, quality-system claims) | UNKNOWN | `source_missing` |
| Sibling-model envelope split (STARlet/STAR/STARplus dimensions, tracks, weight, power) | UNKNOWN beyond S11 STAR row | `source_missing` (`search-summary-only` hints only) |
| Environmental/storage/transport windows, noise, power draw per configuration | UNKNOWN | `source_missing` |
| Sibling protocol-repository example with step-level locators (e.g. STAR NGS/PCR application note) | UNKNOWN — none pinned | `source_missing` (no Hamilton application-note protocol opened this task; do not invent) |
| Fitted channels/heads/options, tip/carrier stocks, calibration/PM/LIMS state | UNKNOWN | `device_state` + `configuration_dependent` |

---

## 4. Cross-record notes

- **Liquid classes.** Opentrons: `transfer_with_liquid_class` + versioned
  liquid-class definitions exist per the operation record (S2 there) but no
  definition content was fetched in THIS task → UNKNOWN (`source_missing`);
  do not encode. Manual analogue is per-liquid user adjustment + gravimetric
  verification (Eppendorf manual §§5.5, via instrument file
  `manual-pipette-eppendorf-research-plus.md`). Hamilton: per-reagent tuning
  via LLD mode + ADC + TADM references; same UNKNOWN status.
- **Tip handling (all platforms).** Single-use disposable tips are the
  reference rule (manual file §§2.2/3.4; Opentrons `new_tip` policies in the
  operation record). Full-rack vs partial-rack adapter rules (Flex) and
  press-on/eject mechanics (OT-2) are the only tip claims verified here
  (S2/S8); everything else (tip catalogues, sterility, filter specs,
  third-party equivalence) is UNKNOWN.
- **Run files (all platforms).** Versioned protocol source + deck
  configuration + run-time gate (Flex deck-conflict) are verified concepts;
  exact log/deck-map/liquids schemas, error-code catalogues, and audit-trail
  exports are UNKNOWN for JSON conversion — omit, do not null-fill.
- **Throughput / timing claims** (22 s plate fill, walkaway time, NGS
  turnarounds): excluded — `search-summary-only` marketing without test
  conditions; time scope stays `unresolved`.

## 5. Fields personally verified (no SourceChecked claim)

- Flex system envelope, environment, certifications, model/serial scheme: S1
  tables (§1C).
- Flex 6-model pipette list, mount occupancy, full accuracy/precision tables,
  adapter/partial-pickup/sensor rules, positional-only calibration: S2 (§1C).
- Flex working/staging/trash/chute slots, A1–D4 coordinates incl.
  Flex↔OT-2 equivalence, fixture rules, deck-conflict gate: S3 (§§1C/2C).
- Flex aspirate/dispense defaults (1 mm), clearance/meniscus/end-location
  mechanics, rate-vs-flow_rate exclusivity, push-out default, v2.17
  over-dispense guard: S4 sections seen (§1C).
- OT-2 envelope, 11+trash deck, GEN1/GEN2 distinction, 5-model list,
  single + 8-ch accuracy tables, press-on/eject + positional-only
  calibration: S7/S8 (§2C).
- Hamilton STAR class envelope (145 kg, 55 ANSI/SLAS, ≤16×1 mL / ≤8×5 mL /
  ≤16×X1, MPH 96/384, 1 µL–5000 µL conditional, IVDR No, optional
  Grippers/Autoload) + monitoring/software feature existence: S10/S11
  (§3C).
- I did NOT verify: any full paper-manual PDF bytes; S4 remainder;
  liquid-class contents; labware-library catalogue; tip catalogues; run-log
  schemas; any named versioned wet-lab protocol's step values; Hamilton
  acceptance numbers. All marked above. No second-person review performed.

## 6. Review

`review_status: research_draft`. No `ReviewRecord` (reviewer, date, object
version, scope, evidence, outcome) exists. A second person MUST re-verify
every field against the cited URL + table/section before any
`Compatible` use or JSON conversion; `agent-verified` above means
"record author read the page", never an approval.
