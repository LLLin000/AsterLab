# Centrifuge / Cell Counter / Plate Washer — InstrumentRecords (documented profiles)

`review_status: research_draft`

> Evidence status: I did NOT open or read any full manual/spec-sheet PDF for
> this draft. Every value below comes from search-result summaries only (see
> §5) and is marked `search-summary-only`. No value is `agent-verified`.
> Do NOT claim `SourceChecked`. No `ReviewRecord` exists. All numeric values
> (rpm/RCF, count ranges, wash volumes) MUST be re-verified against the cited
> official manual before any compatibility planning or JSON conversion.

Retrieval date for all sources: 2026-11-05.

## InstrumentRecord 1 — Refrigerated microcentrifuge

### 1A. Identity

| Field | Value |
|---|---|
| profile id (proposed) | `centrifuge-eppendorf-5424r` |
| version | research draft v0 |
| class | centrifuge / refrigerated microcentrifuge |
| Manufacturer / model | Eppendorf Centrifuge 5424 R (`search-summary-only`) |
| Configuration covered | Base refrigerated 24-place unit; rotor/adapter-dependent variants (see §1C). Exact installed rotor/adapters UNKNOWN |

### 1B. Sources

| # | Source | Locator |
|---|---|---|
| S1 | Eppendorf operating manual PDF, Centrifuge 5424 R | https://www.eppendorf.com/product-media/doc/en/330723/Centrifugation_Operating-manual_Centrifuge-5424-R.pdf — `search-summary-only`; exact pages/tables not verified |
| S2 | Eppendorf product page "Centrifuge 5424 R" | https://www.eppendorf.com/de-en/Products/Centrifugation/Microcentrifuges/Centrifuge-5424R-p-PF-240997 — `search-summary-only`; Specifications section not paginated here |

### 1C. Capabilities (`search-summary-only` throughout)

| Capability | Value (unverified, re-verify before use) | Evidence |
|---|---|---|
| Max speed | 15,000 rpm / 21,130 × g, rotor- and adapter-dependent | S1 summary |
| Speed range / increments | 100–15,000 rpm, 50 rpm increments (`search-summary-only`) | S1/S2 summaries |
| Capacity | 24 × 1.5/2.0 mL tubes or spin columns with appropriate rotor | S1 summary |
| Named rotors (partial list) | FA-45-24-11, FA-45-24-11-Special, F-45-18-11-Kit, FA-45-32-5-PCR | S1 summary |
| Temperature range | −10 °C to +40 °C; FastTemp pre-cool function (`search-summary-only`) | S1/S2 summaries |
| Run-time programming | 30 s to 9 h 59 min + continuous/infinity (`search-summary-only`) | S1 summary |
| Sample density limit | Max 1.2 g/mL (`search-summary-only`) | S1 summary |
| Noise | < 54 dB(A) (`search-summary-only`) | S1 summary |

### 1D. Container / rotor / adapter support

Status: `partial` — four rotor names appear in the search summary; full
rotor/adapter compatibility matrix (which tube volumes per adapter, per-rotor
max RCF, lid-open restrictions) was NOT verified. MUST NOT treat this partial
list as complete. Swing-bucket or 5 mL support: UNKNOWN (`source_missing`).

### 1E. Conditions

- Max RCF values are per-rotor/per-adapter; operator MUST NOT exceed the sample
  vessel's rated g-force (`search-summary-only`, per S1 summary note).
- Refrigeration performance (e.g. holding 4 °C at max speed) and
  acceleration/deceleration ramp options: UNKNOWN (`source_missing` — only
  summary mentions exist).
- Power (350 W max per summary), dimensions, ECO shut-off: `search-summary-only`,
  not verified here.

### 1F. Raw outputs

No digital assay readout. Raw observables: set vs. actual rpm/RCF, set vs.
chamber temperature, elapsed time, rotor ID (display-dependent).
Display toggle (rpm vs. RCF) and error/log export: UNKNOWN (`source_missing`).

### 1G. Actual-device-vs-profile boundary

This record describes the manufacturer platform capability only. It is NOT the
installed machine: installed rotor(s), adapters on hand, calibration state,
maintenance history, and actual temperature performance belong to Run/device
state and are UNKNOWN here (`device_state`). Optional rotors listed above MUST
be written as variants/conditions, not as present on every unit.

### 1H. Unknowns

| Item | Status | Reason |
|---|---|---|
| Per-rotor/per-adapter max RCF table | UNKNOWN | `source_missing` (summary only, no page/table verified) |
| Installed rotor/adapters, calibration, service state | UNKNOWN | `device_state` |
| Temperature accuracy/uniformity, pre-cool timing | UNKNOWN | `source_missing` |
| Acceleration/deceleration rate options | UNKNOWN | `source_missing` |
| Aerosol-tight lids / biosafety options | UNKNOWN | `source_missing` |

---

## InstrumentRecord 2 — Automated cell counter

### 2A. Identity

| Field | Value |
|---|---|
| profile id (proposed) | `cell-counter-invitrogen-countess-3` |
| version | research draft v0 |
| class | automated cell counter / brightfield suspension-cell counter |
| Manufacturer / model | Invitrogen Countess 3 Automated Cell Counter, Thermo Fisher Scientific (`search-summary-only`) |
| Configuration covered | Brightfield-only Countess 3 (NOT Countess 3 FL fluorescence variant) |

### 2B. Sources

| # | Source | Locator |
|---|---|---|
| S3 | Countess 3 User Guide PDF (MAN0019566) | https://documents.thermofisher.com/TFS-Assets/LSG/manuals/MAN0019566_Countess_3_Auto_Cell_Counter_UG.pdf — `search-summary-only`; pages/sections not verified |
| S4 | Thermo Fisher product page "Countess 3 Automated Cell Counter" | https://www.thermofisher.com/us/en/home/life-science/cell-analysis/cell-analysis-instruments/automated-cell-counters/models/countess-3.html — `search-summary-only`; Specifications section not paginated here |

### 2C. Capabilities (`search-summary-only` throughout)

| Capability | Value (unverified, re-verify before use) | Evidence |
|---|---|---|
| Method | Brightfield automated counting + viability with trypan blue (0.4%) or SafeCount stains | S4 summary |
| Sample concentration range | 1 × 10⁴ to 1 × 10⁷ cells/mL | S3/S4 summaries |
| Particle/cell diameter range | 4–60 µm | S3/S4 summaries |
| Sample/chamber volume | 10 µL | S3/S4 summaries |
| Processing time | < 30 s per sample | S4 summary |
| Optics | Single-channel brightfield; 5 MP camera; 2.5× optical magnification (`search-summary-only`) | S3/S4 summaries |
| Counting area | 3.82 mm² (2.26 × 1.69 mm) (`search-summary-only`) | S4 summary |
| Data storage/export | USB drive (32 GB; FAT32/exFAT/NTFS per summary); Thermo Fisher Connect import/export | S3 summary |
| Operating environment | 10–40 °C; humidity < 80% non-condensing (`search-summary-only`) | S3 summary |

### 2D. Container support

Status: `partial` — compatible with Countess chamber slides and reusable slides
per summaries; slide dimensions 25 × 75 mm PMMA (`search-summary-only`). Full
consumable part-number list and reusable-slide cleaning lifetime: UNKNOWN
(`source_missing`). MUST NOT treat named slides as the complete list.

### 2E. Conditions

- Viability stains named (trypan blue / SafeCount) are consumable/method
  requirements, not instrument capabilities; gating/threshold parameters live in
  the counting protocol.
- Autofocus/lighting automation, histogram/scatter gating behavior, 21 CFR
  Part 11 software option: `search-summary-only`, sections not verified.

### 2F. Raw outputs

Per summaries: total/live/dead counts, viability %, mean size, captured image,
histogram/scatter data, saved protocol files (`search-summary-only`). Exact
file formats, single-cell data export schema, and OQ report contents: UNKNOWN
(`source_missing`).

### 2G. Actual-device-vs-profile boundary

Platform capability ≠ installed machine. Firmware/software version, calibration
(OQ) status, installed options (Wi-Fi dongle, 21 CFR Part 11 licence), and
reusable-slide wear belong to device state: all UNKNOWN here (`device_state`).

### 2H. Unknowns

| Item | Status | Reason |
|---|---|---|
| Accuracy/precision vs. hemocytometer, linearity evidence | UNKNOWN | `source_missing` (app notes not verified) |
| Focus/exposure/gating parameter definitions | UNKNOWN | `source_missing` |
| Fluorescence channels | N/A for Countess 3 (FL variant is a different instrument) | `not_applicable` — variant boundary |
| Software/firmware version compatibility | UNKNOWN | `configuration_dependent` |
| Calibration/OQ interval and procedure | UNKNOWN | `device_state` / `source_missing` |

---

## InstrumentRecord 3 — Microplate washer

### 3A. Identity

| Field | Value |
|---|---|
| profile id (proposed) | `plate-washer-agilent-biotek-405ts` |
| version | research draft v0 |
| class | microplate washer/dispenser |
| Manufacturer / model | Agilent BioTek 405 TS Microplate Washer (`search-summary-only`) |
| Configuration covered | Base washer; manifolds, buffer-switching, vacuum-filtration, magnetic-separation, and ultrasonic options are configuration-dependent variants |

### 3B. Sources

| # | Source | Locator |
|---|---|---|
| S5 | Agilent technical-details PDF (5994-5501EN) | https://www.agilent.com/cs/library/specifications/public/405-TS-technical-details-5994-5501EN-agilent.pdf — `search-summary-only`; pages/tables not verified |
| S6 | Agilent product page "BioTek 405 TS Washer" | https://www.agilent.com/en/product/cell-analysis/microplate-automation-detection/microplate-washers-dispensers/biotek-405-ts-washer-1623265 — `search-summary-only`; section not paginated here |

### 3C. Capabilities (`search-summary-only` throughout)

| Capability | Value (unverified, re-verify before use) | Evidence |
|---|---|---|
| Plate formats | 96- and 384-well, low-profile and standard height; solid or filter-bottom (optional) | S5 summary |
| Manifolds | 96-tube straight; 96-tube dual-action (20°); 192-tube dual-action (7°) for 384-well | S5 summary |
| Dispense volume per tube | 96-well manifold 50–3,000 µL/tube; 384-well manifold 25–3,000 µL/tube, 1 µL increments | S5 summary |
| Wash cycles | 1–250 | S5 summary |
| Dispense precision | < 3% CV at 300 µL/well (96-well); < 4% CV at 80 µL/well (384-well) | S5 summary |
| Residual volume | ≤ 2 µL/well | S5 summary |
| Soak / shake | Programmable up to 60 min each | S5 summary |
| Buffer switching | Optional 4-buffer auto-switch; 4 L or 10 L supply bottles | S5 summary |
| Vacuum filtration (optional) | −38 to −506 mmHg; 5–999 s | S5 summary |
| Control | Onboard software + LHC2 PC software (21 CFR Part 11 option); SiLA driver option | S5 summary |
| Power / size / weight | 100–240 VAC 50–60 Hz; 14 × 17 × 10 in; 30–36 lb by configuration | S5 summary |

### 3D. Container / plate support

Status: `partial` — only plate classes (96/384, low/standard height) are
summarized. Brand-specific plate-height/adapter tables, deep-well support, and
filter-plate vacuum compatibility: UNKNOWN (`source_missing`). MUST NOT treat
the two plate classes as a complete labware list.

### 3E. Conditions

- Cell-friendly behavior (angled dispense, adjustable dispense/aspiration rates,
  side-wall dispensing) is named in summaries but without numeric
  flow-rate/height values here: UNKNOWN (`source_missing`).
- Ultrasonic Advantage cleaning, Verify clog detection, biomagnetic separation:
  optional/configuration-dependent — MUST be written as variants, not as
  present on every unit.
- Dead/prime volumes (e.g. third-party summary figures) are NOT recorded here:
  they came from a non-manufacturer mirror (`source_missing` as to official
  manual pages).

### 3F. Raw outputs

No analytical readout. Raw artifacts: executed wash protocol parameters,
Verify/clog-check results, error flags (`search-summary-only`). Log file
formats and export paths: UNKNOWN (`source_missing`).

### 3G. Actual-device-vs-profile boundary

This record is the configurable platform, not an installed washer. Installed
manifold(s), buffer-switch module, vacuum/magnet/ultrasonic options, pump
conditioning, and manifold calibration state belong to device state: all
UNKNOWN here (`device_state`).

### 3H. Unknowns

| Item | Status | Reason |
|---|---|---|
| Installed manifold and option modules | UNKNOWN | `configuration_dependent` |
| Per-plate validated wash protocols (volumes, cycles, soak) | UNKNOWN | `method_dependent` (ELISA vs. cell assays differ) |
| Flow-rate / aspiration-height numeric ranges | UNKNOWN | `source_missing` |
| Residual-volume test conditions | UNKNOWN | `source_missing` |
| Maintenance/sterilization procedure and intervals | UNKNOWN | `source_missing` |
| Automation-interface (BioStack/BioSpa/BenchCel) requirements | UNKNOWN | `configuration_dependent` |

---

## §5. Consolidated source table

| # | Official source | URL | Reading depth |
|---|---|---|---|
| S1 | Eppendorf Centrifuge 5424 R operating manual PDF | https://www.eppendorf.com/product-media/doc/en/330723/Centrifugation_Operating-manual_Centrifuge-5424-R.pdf | `search-summary-only` |
| S2 | Eppendorf Centrifuge 5424 R product page | https://www.eppendorf.com/de-en/Products/Centrifugation/Microcentrifuges/Centrifuge-5424R-p-PF-240997 | `search-summary-only` |
| S3 | Invitrogen Countess 3 User Guide MAN0019566 | https://documents.thermofisher.com/TFS-Assets/LSG/manuals/MAN0019566_Countess_3_Auto_Cell_Counter_UG.pdf | `search-summary-only` |
| S4 | Thermo Fisher Countess 3 product page | https://www.thermofisher.com/us/en/home/life-science/cell-analysis/cell-analysis-instruments/automated-cell-counters/models/countess-3.html | `search-summary-only` |
| S5 | Agilent BioTek 405 TS technical details 5994-5501EN | https://www.agilent.com/cs/library/specifications/public/405-TS-technical-details-5994-5501EN-agilent.pdf | `search-summary-only` |
| S6 | Agilent BioTek 405 TS product page | https://www.agilent.com/en/product/cell-analysis/microplate-automation-detection/microplate-washers-dispensers/biotek-405-ts-washer-1623265 | `search-summary-only` |

## §6. Evidence status and next verification steps

- Evidence status for every capability row: `search-summary-only`. Nothing in
  this file is `agent-verified`, `SourceChecked`, `ExpertReviewed`, or
  `LabValidated`.
- No `ReviewRecord` exists (no reviewer, no date, no scope, no outcome).
- Before any Compatible-use or JSON conversion: (1) open S1/S3/S5 PDFs and
  record exact page/table/section per value; (2) resolve 5424 R per-rotor RCF
  table, Countess 3 gating/file-format sections, and 405 TS flow-rate/prime-
  volume sections; (3) confirm installed configurations (rotors, manifolds,
  software versions) as device state; (4) submit for second-person review.
