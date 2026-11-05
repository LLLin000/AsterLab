# qPCR / Flow Cytometry / Spectrophotometry / Thermomixer — Instrument Capability Records

`review_status: research_draft` (all four records below)

> Reading statement (record author, 2026-11-05): I directly read the two
> official Thermo Fisher sources for the NanoDrop One (§R2 spec-sheet PDF,
> both pages incl. the full Technical Specifications table), the official BD
> FACSCanto II technical-specifications PDF (all 4 pages), and the official
> Thermo Fisher QuantStudio 3/5 product page (Features + Specifications table).
> For the Eppendorf ThermoMixer C I read the official product page text and
> extracted its Technical Data values; the operating-manual PDF returned
> HTTP 403 and was NOT read. QuantStudio ramp-rate / reaction-volume /
> optical-filter rows that I could not open in a primary source are marked
> `search-summary-only` or UNKNOWN, not verified. Do NOT claim SourceChecked
> for any record below — no second-person review has occurred.

## 0. Sources and retrieval log

| # | Source | URL | Locator | Retrieved | Reading degree |
|---|---|---|---|---|---|
| S1 | Thermo Fisher QuantStudio 3/5 product page, Specifications table | https://www.thermofisher.com/us/en/home/life-science/pcr/real-time-pcr/real-time-pcr-instruments/quantstudio-systems/models/quantstudio-3-5.html | "Specifications" table (Format / Efficiency and productivity rows); Features list | 2026-11-05 | Agent-read (page text) |
| S2 | QuantStudio 3/5 specification sheet (FisherSci mirror) and related spec summaries | https://assets.fishersci.com/TFS-Assets/GSD/Reference-Materials/quantstudio-3-5-specification-sheet.pdf (fetch failed: binary/aborted); values via search-result summaries only | Spec-sheet rows as surfaced in search summaries | 2026-11-05 | `search-summary-only` — primary PDF not opened |
| S3 | BD FACSCanto II Flow Cytometer Technical Specifications (doc 23-10918-00) | https://www.bdbiosciences.com/content/dam/bdb/marketing-documents/facscanto_techspecs.pdf | pp. 2–4: Optics/Fluidics table, Detector Bands, Data Management table, Cytometer Options, Installation Requirements | 2026-11-05 | Agent-read (full PDF text) |
| S4 | BD FACSCanto II Filter Guide | https://www.bdbiosciences.com/content/dam/bdb/marketing-documents/BD_FACSCanto_II_FilterGuide.pdf | Configuration/filter rows | 2026-11-05 | `search-summary-only` — PDF not opened, search summary only |
| S5 | NanoDrop One Microvolume UV-Vis Spectrophotometers spec sheet (PS52754_E 0220) | https://assets.thermofisher.com/TFS-Assets/CAD/Specification-Sheets/NanoDrop-One-Specifications.pdf | p. 2, full Technical Specifications table | 2026-11-05 | Agent-read (full PDF text) |
| S6 | Eppendorf ThermoMixer C product page | https://www.eppendorf.com/us-en/Products/Temperature-Control-and-Mixing/Instruments/Eppendorf-ThermoMixerC-p-PF-19703 | Marketing text + Technical Data values (orbit, speed, temp range, dimensions, weight, power) | 2026-11-05 | Agent-read (page text; Technical Data table partially extracted) |
| S7 | Eppendorf ThermoMixer C operating manual (doc 47748) | https://www.eppendorf.com/product-media/doc/en/47748/Sample-Preparation_Operating-manual_ThermoMixer-C.pdf | Technical-data section (§8 approx.) | 2026-11-05 | NOT read — HTTP 403; values attributed to it are `search-summary-only` |

Regulatory/footnote common to S1, S5: "For Research Use Only. Not for use in diagnostic procedures."
S3 states "For In Vitro Diagnostic Use" on p. 2 (7- and 8-color violet-laser assays require user validation for IVD use; see IR2 conditions).

---

## IR1. Real-time PCR — Applied Biosystems QuantStudio 5 (documented configuration: 96-well 0.2 mL block)

### IR1.1 Identity

| Field | Value |
|---|---|
| profile id | `inst-qpcr-quantstudio5-96w02ml` |
| version | `v0.1.0-research_draft` |
| class | real-time PCR thermocycler with fluorescence detection |
| manufacturer / model | Thermo Fisher Scientific (Applied Biosystems) QuantStudio 5 Real-Time PCR System |
| documented configuration | Fixed-block instrument; this record covers the **96-well 0.2 mL block** configuration only. The 96-well 0.1 mL (Fast) and 384-well blocks are separate variants and are NOT covered (see IR1.7 UNKNOWNs) |
| control software | Full-function touchscreen (on-board) and/or PC/cloud via QuantStudio Connect; 21 CFR Part 11 assistance package (security, auditing, e-signature) on QS5 (S1) |

### IR1.2 Capabilities (only where a source was read; 96-well 0.2 mL unless noted)

| Key | Value / range / set | Unit | Conditions | Evidence |
|---|---|---|---|---|
| block format | fixed 96-well, 0.2 mL consumables (also accepts 0.1 mL consumables per S1 Features text; block itself is fixed per S1) | — | Configuration chosen at purchase; blocks are not interchangeable in the field per S1 "Fixed blocks" | `agent-verified` (S1 Specifications + Features) |
| multiplexing | up to 6 targets (96-well); 384-well up to 5 (variant, listed for boundary only) | targets/run | QS5 96-well per S1 Specifications table | `agent-verified` (S1) |
| temperature control | VeriFlex, 6 independent zones (96-well blocks only) | zones | QS5 only; QS3 has 3 zones — do not transfer | `agent-verified` (S1) |
| discrimination | differences in target quantity as low as 1.5-fold detectable | fold | As stated on S1 overview; chemistry/assay-dependent | `agent-verified` (S1) |
| on-board memory | >2,000 runs | runs | S1 Specifications table | `agent-verified` (S1) |
| cloud connectivity | direct connection to cloud-enabled system (QuantStudio Connect), remote monitoring | — | S1 Specifications + Features | `agent-verified` (S1) |
| dimensions | 40 × 27 × 50 cm (H × W × D) | cm | S1 Specifications table | `agent-verified` (S1) |
| reaction volume | 10–100 µL | µL | 96-well 0.2 mL block | `search-summary-only` (S2) — NOT verified in an opened primary source |
| max block ramp rate | 6.5 °C/s (0.2 mL block) | °C/s | Block- and chemistry-dependent; 0.1 mL and 384-well values differ and are out of scope | `search-summary-only` (S2) — NOT verified in an opened primary source |
| average ramp rate | ~3.66 °C/s (0.2 mL block) | °C/s | Same conditions as above | `search-summary-only` (S2) |
| temperature uniformity | 0.4 °C | °C | Block uniformity claim | `search-summary-only` (S2) |
| optical filters | 96-well: 6 decoupled filters / up to 21 filter combinations (per S2 summary); 384-well: 5 coupled filters | — | Format-dependent; exact excitation/emission band table NOT verified | `search-summary-only` (S2) |
| dynamic range | ~10 log units; single-copy singleplex detection claimed | log units | Assay/chemistry-dependent | `search-summary-only` (S2) |
| factory-calibrated dye set | FAM/SYBR, VIC, ABY, NED, TAMRA, JUN, ROX, Mustang Purple, Cy5 (per S2 summary) | — | Exact per-channel filter mapping NOT verified | `search-summary-only` (S2) |

### IR1.3 Container / consumable support — `partial`

- Supported (agent-verified, S1): 96-well 0.2 mL consumables; 96-well 0.1 mL consumables also listed for QS5 in S1 Features ("96-well (0.1-mL block), 96-well (0.2-mL block), and 384-well options").
- 384-well consumables belong to the 384-well block variant, not this configuration.
- Plate/part numbers, seal types, 2D-barcode support: UNKNOWN (not verified).

### IR1.4 Conditions

- QS5 6-zone VeriFlex applies to 96-well blocks only (S1, `agent-verified`).
- 21 CFR Part 11 "assistance" features (security, auditing, e-signature) are software/package features, not instrument accuracy claims (S1).
- Fast vs. standard chemistries, run-time claims ("<30 min"), and calibration-kit part numbers (S1 lists A26332-series etc.) are ordering/workflow information, not capability bounds — not transferred into capability rows.

### IR1.5 Raw outputs

Amplification data per well/channel, melt-curve data, allelic-discrimination (genotyping) data, run files stored on-board (>2,000 runs) and exportable via cloud/USB; RDML export described as MIQE-friendly (S2, `search-summary-only`). No scientific conclusions are recorded here.

### IR1.6 Actual-device boundary

Static profile only. The following are Run/device state and MUST NOT be written into the profile: installed block serial, calibration/verification-kit status and dates, dye-calibration currency, software/firmware version, 21 CFR Part 11 mode on/off, cloud account linkage, service history.

### IR1.7 UNKNOWNs

| Item | Status | Reason / UNKNOWN class |
|---|---|---|
| Exact optical filter excitation/emission bands per channel | UNKNOWN | `source_missing` — S2 search summary gives ranges (450–680 nm excitation) but the band table was never opened |
| Ramp rates, uniformity, reaction volumes as committed values | UNKNOWN (provisional `search-summary-only` rows above) | `source_missing` — primary spec-sheet PDF not opened; do not use for Compatible decisions |
| 384-well and 96-well 0.1 mL block capabilities | UNKNOWN | `configuration_dependent` — separate variants, deliberately out of scope |
| Temperature accuracy (°C) | UNKNOWN | `source_missing` — value seen only in Dx-model summary (0.25 °C), wrong model; not transferred |
| Hold/gradient programming limits, heated-lid temperature | UNKNOWN | `source_missing` |
| Calibration procedure and interval | UNKNOWN | `source_missing` — kit part numbers listed (S1) but procedure manual not read |
| Export file formats / LIMS interface details | UNKNOWN | `source_missing` |

---

## IR2. Flow cytometer — BD FACSCanto II (documented configuration: standard 2-laser 4-2; options as conditional variants)

### IR2.1 Identity

| Field | Value |
|---|---|
| profile id | `inst-flow-facscanto2-2laser42` |
| version | `v0.1.0-research_draft` |
| class | benchtop flow cytometer (hydrodynamic-focusing cell analyzer) |
| manufacturer / model | BD Biosciences BD FACSCanto II |
| documented configuration | Standard **2-laser (488 nm + 633 nm), 6-PMT 4-2** configuration. The 8-color 3-laser (405 nm) and 8-color 2-laser (5-3) options and the FACS Loader / HTS options are **conditional variants**, not part of the base configuration (S3) |
| control software | BD FACSDiva; BD FACSCanto clinical software (S3) |
| regulatory note | "For In Vitro Diagnostic Use" (S3 p. 2); 7- and 8-color assays using the violet laser are Research Use Only / require user validation for IVD (S3 p. 3) |

### IR2.2 Capabilities (standard 2-laser 4-2 configuration; S3 `agent-verified` throughout)

| Key | Value / range / set | Unit | Conditions | Evidence |
|---|---|---|---|---|
| lasers | 488 nm solid-state 20 mW; 633 nm HeNe 17 mW; spatially separated beams, 9 × 65 µm elliptical spots | nm / mW | Fixed alignment; no operator alignment required | `agent-verified` (S3 p. 2) |
| flow cell | 180 × 430 µm rectangular quartz | µm | Fixed-alignment flow cell | `agent-verified` (S3 p. 2) |
| collection optics | optical-gel coupled 1.2 NA lens; reflective optics with single transmission filter per PMT | — | S3 p. 2 | `agent-verified` (S3 p. 2) |
| FSC / SSC resolution | 1.0 µm / 0.5 µm | µm | S3 p. 2 | `agent-verified` (S3 p. 2) |
| FSC / SSC detectors | FSC: photodiode 488/10 BP; SSC: PMT 488/10 BP | — | S3 p. 2 | `agent-verified` (S3 p. 2) |
| fluorescence detectors | 6 PMTs, 4-2 arrangement | PMTs | Standard configuration | `agent-verified` (S3 p. 2) |
| blue-laser dyes | FITC, PE, PerCP or PerCP-Cy5.5, PE-Cy7 (525, 575, 678/695, 785 nm) | — | S3 p. 2 | `agent-verified` (S3 p. 2) |
| red-laser dyes | APC, APC-Cy7 (660, 785 nm) | — | S3 p. 2 | `agent-verified` (S3 p. 2) |
| detector bands (standard) | Blue 530/30, 585/42, >670, 780/60 nm; Red 660/20, 780/60 nm | nm | Keyed filters, no tools required | `agent-verified` (S3 p. 2) |
| MESF threshold sensitivities | FITC <100; PE <50 | MESF | Fluorescence threshold sensitivities row, S3 p. 2 | `agent-verified` (S3 p. 2) |
| setup-bead resolution minima | FITC >15; PE >80; PerCP >9; PerCP-Cy5.5 >25; PE-Cy7 >120; APC >40; APC-Cy7 >16 | (bead score) | BD FACS 7-color setup beads; higher = better separation (S3 p. 2). NOT the same quantity as MESF above — do not merge | `agent-verified` (S3 p. 2) |
| sample flow rate | 10–120 | µL/min | S3 p. 2 | `agent-verified` (S3 p. 2) |
| max particle size | 50 | µm | S3 p. 2 | `agent-verified` (S3 p. 2) |
| acquisition rate | up to 10,000 events/s (6 compensated fluorescence + 2 scatter parameters) | events/s | S3 p. 2 | `agent-verified` (S3 p. 2) |
| sample dead volume | 30 (BD Falcon 12 × 75 mm tubes) | µL | S3 p. 2 | `agent-verified` (S3 p. 2) |
| carryover | ≤0.1 | % | Standard configuration (HTS variant: ≤1%, S3 p. 3) | `agent-verified` (S3 p. 2) |
| sheath consumption | 1.10 L/h normal; <1 mL/h standby | L/h | Integrated fluidics cart + compressor, automated startup/shutdown/cleaning | `agent-verified` (S3 p. 2) |
| signal processing | 18-bit dynamic range, IEEE 32-bit floating-point resolution | bits | S3 p. 3 | `agent-verified` (S3 p. 3) |
| parameters recorded | Area, Width, Height for all channels + up to 2 ratios + Time | — | S3 p. 3 | `agent-verified` (S3 p. 3) |
| threshold | single parameter (any channel) or logical combinations (any/all) | — | S3 p. 3 | `agent-verified` (S3 p. 3) |
| compensation | full inter-beam matrix, during or post acquisition | — | S3 p. 3 | `agent-verified` (S3 p. 3) |
| installation | cytometer 61 × 91 × 64 cm, 145 kg; cart 61 × 79 × 64 cm, 51 kg; 100/115/230 VAC 50–60 Hz; 16–30 °C, 20–80% RH noncondensing; 1,843 BTU/h with Loader | — | S3 p. 4 | `agent-verified` (S3 p. 4) |

Conditional variants (platform capability, NOT base configuration — use only with the option installed):

| Variant | Key deltas | Evidence |
|---|---|---|
| 8-color, 3-laser | +405 nm diode 30 mW; 8 PMTs 4-2-2; violet dyes Pacific Blue, AmCyan; violet bands 450/50, 502–525 nm; 40-tube carousel Loader option | `agent-verified` (S3 p. 3) |
| 8-color, 2-laser | 8 PMTs 5-3; blue adds PE-Texas Red; red adds Alexa Fluor 700; red bands 660/20, 712/21, 780/60 nm | `agent-verified` (S3 p. 3) |
| HTS option | 96/384-well plates; <15 min per 96-well plate HTS mode (2 s acquisition); carryover ≤1% | `agent-verified` (S3 p. 3) |

### IR2.3 Container / consumable support — `partial`

- Supported (agent-verified, S3): BD Falcon 12 × 75 mm tubes (dead-volume reference); 40-tube carousel with Loader option; 96/384-well plates with HTS option; BD FACSFlow sheath (20 L), FACSClean (5 L), shutdown solution (5 L), 10 L waste tank.
- Tube-vendor compatibility beyond BD Falcon, plate-brand compatibility, minimum well volumes: UNKNOWN (partial list only — do not treat as complete).

### IR2.4 Conditions

- Sensitivity numbers are bead- and fluorochrome-dependent; MESF thresholds and bead-resolution scores are different quantities (S3 p. 2) — they MUST NOT be merged or averaged.
- Filter-change procedure is keyed/no-tools (S3); actual installed filter set is device state.
- Throughput figures (56/66 min per carousel with Multi-check controls) are application- and control-dependent (S3 p. 3), not instrument-rate guarantees.

### IR2.5 Raw outputs

Per-event listmode data (FSC/SSC + fluorescence Area/Width/Height, ratios, Time), compensation matrices, carousel/worklist metadata, QC bead results. No gating outcomes or biological conclusions are recorded here.

### IR2.6 Actual-device boundary

Static profile only. The following are Run/device state: installed laser/filter configuration, PMT voltages, installed options (Loader/HTS/405 nm), fluidics levels and cleaning status, calibration-bead currency, software version, service history.

### IR2.7 UNKNOWNs

| Item | Status | Reason / UNKNOWN class |
|---|---|---|
| Per-channel PMT spectral response / gain ranges | UNKNOWN | `source_missing` — not in S3; Filter Guide (S4) only `search-summary-only` |
| Exact per-fluorochrome spillover matrix | UNKNOWN | `method_dependent` — panel- and voltage-dependent; full inter-beam matrix is computed per run |
| Maximum logical gate regions (numeric cap) | UNKNOWN | `source_missing` — S3 says "limited only by system memory (2 GB RAM)", no number |
| CPU/monitor and OS rows (HP Xw4600, Win XP Pro) | UNKNOWN (currency) | `source_missing` — S3 (2009) lists obsolete IT; current workstation config unverified |
| Loader carousel throughput for user panels | UNKNOWN | `method_dependent` — S3 figures are for specified control/application combos only |
| Violet-laser dye/filter mapping for 4-2-2 | UNKNOWN (provisional `search-summary-only` via S4) | `source_missing` — Filter Guide PDF not opened |
| Service/calibration interval | UNKNOWN | `source_missing` |

---

## IR3. Microvolume spectrophotometer — Thermo Scientific NanoDrop One (documented both variants: pedestal-only ND-ONE and pedestal + cuvette ND-ONEC)

### IR3.1 Identity

| Field | Value |
|---|---|
| profile id | `inst-spectro-nanodrop-one` |
| version | `v0.1.0-research_draft` |
| class | microvolume UV-Vis spectrophotometer (pedestal sample-retention; cuvette position on OneC variant only) |
| manufacturer / model | Thermo Fisher Scientific NanoDrop One (ND-ONE, pedestal only) / NanoDrop OneC (ND-ONEC, pedestal + cuvette) |
| documented configuration | Both variants documented in ONE record with cuvette rows explicitly conditioned on OneC; pedestal rows apply to both (S5) |
| control | 7-inch 1280 × 800 multipoint capacitive touchscreen, gloved-lab compatible; Android + ARM Cortex-A9; 32 GB flash; PC software (Windows) for data management (S5) |

### IR3.2 Capabilities (S5 `agent-verified` throughout unless noted)

| Key | Value / range / set | Unit | Conditions | Evidence |
|---|---|---|---|---|
| minimum sample volume | 1 | µL | Pedestal | `agent-verified` (S5 spec table) |
| consumables for pedestal | none — pipette directly onto pedestal sample-retention system | — | Pedestal only | `agent-verified` (S5 "No consumables needed") |
| wavelength range | 190–850 | nm | S5 spec table | `agent-verified` (S5) |
| wavelength accuracy | ±1 | nm | S5 spec table | `agent-verified` (S5) |
| spectral bandwidth | ≤1.8 (FWHM at Hg 254 nm) | nm | S5 spec table | `agent-verified` (S5) |
| photometric range | pedestal 0–550 A; cuvette 0–1.5 A (10 mm equivalent) | A | Cuvette row applies to OneC only | `agent-verified` (S5) |
| photometric accuracy | 3% at 0.97 A, 302 nm | — | Footnote 2: absorbance expressed per Abs/mm at 25 °C | `agent-verified` (S5 + footnote) |
| repeatability | typical 0.002 A (1.0 mm path) or 1% CV, whichever greater | — | Footnote 1: SD of 10 individual measurements at 0.97 A | `agent-verified` (S5 + footnote) |
| pathlength | 0.030–1.0 mm auto-ranging | mm | Pedestal | `agent-verified` (S5) |
| dsDNA detection limit | pedestal 2.0 ng/µL; cuvette 0.2 ng/µL | ng/µL | Cuvette value OneC only | `agent-verified` (S5) |
| BSA (IgG) detection limit | pedestal 0.06 (0.03) mg/mL; cuvette 0.006 (0.003) mg/mL | mg/mL | Parentheses as printed in S5 | `agent-verified` (S5) |
| dsDNA maximum | pedestal 27,500 ng/µL | ng/µL | Extended auto-range pathlength | `agent-verified` (S5) |
| BSA (IgG) maximum | pedestal 820 (400) mg/mL | mg/mL | Parentheses as printed in S5 | `agent-verified` (S5) |
| measurement + processing time | 8 | s | S5 spec table | `agent-verified` (S5) |
| light source / detector | xenon flash lamp; 2048-element CMOS linear image sensor | — | S5 spec table | `agent-verified` (S5) |
| stirring | 9 speeds | speeds | Cuvette (OneC) only | `agent-verified` (S5) |
| cuvette temperature control | 37 | °C | Cuvette (OneC) only; single setpoint listed | `agent-verified` (S5) |
| connectivity | 3× USB-A, Ethernet, Bluetooth, Wi-Fi (models with Wi-Fi/BT support only, footnote 3); DYMO LabelWriter/keyboard/mouse/barcode support | — | Footnote 3: Wi-Fi/BT only on supporting models | `agent-verified` (S5 + footnote) |
| dimensions / weight / power | 20 × 25.4 × 32.3 cm; 3.6 kg; 12 V DC, 12–18 W operating, 5 W standby | — | S5 spec table | `agent-verified` (S5) |
| pre-programmed methods | Nucleic Acid A260/A280/A230 + labeled; Protein A280/A205, Pierce 660, Bradford, BCA, Lowry, labeled; OD600, Kinetics, UV-Vis, Custom | — | S5 Applications Support row | `agent-verified` (S5) |

### IR3.3 Container / consumable support — `partial`

- Supported (agent-verified, S5): pedestal measurement with no consumable; cuvette position (OneC only) with stirring and 37 °C control; PR-1 reconditioning kit and PV-1 verification solution listed as accessories.
- Cuvette dimensional spec (pathlength/material/volume), OD600 culture-vessel compatibility: UNKNOWN — S5 gives no cuvette mechanical spec; partial list only.

### IR3.4 Conditions

- Photometric and repeatability specs carry footnotes 1–2 (measurement conditions at 0.97 A / 25 °C) — they are not blanket guarantees across the range.
- Wi-Fi/Bluetooth connectivity is model-dependent (footnote 3); do not assume for every unit.
- Acclaro contaminant-analysis flags are software guidance, not accuracy claims — recorded here as application behavior only.

### IR3.5 Raw outputs

Full UV-Vis spectra, A260/A280 and A260/230 ratios, labeled-nucleic-acid/protein method results, kinetics traces, OD600 readings, Acclaro sample-intelligence flags, on-board stored results transferable via USB/network. No concentration-validity conclusions are recorded here.

### IR3.6 Actual-device boundary

Static profile only. The following are Run/device state: pedestal reconditioning (PR-1) currency, PV-1 verification currency and results, cuvette actually installed, stir-bar presence, software version, audit/21 CFR Part 11 mode, network linkage.

### IR3.7 UNKNOWNs

| Item | Status | Reason / UNKNOWN class |
|---|---|---|
| Cuvette mechanical/optical spec (pathlength, material, min volume) | UNKNOWN | `source_missing` — no cuvette dimensions in S5 |
| 37 °C control accuracy/uniformity/stability | UNKNOWN | `source_missing` — single setpoint only |
| Stirring speed mapping (rpm per "speed") | UNKNOWN | `source_missing` — "9 speeds" with no units |
| Baseline drift / lamp-life spec | UNKNOWN | `source_missing` |
| PC software version compatibility | UNKNOWN | `source_missing` — Windows 7/10 64-bit listed; currency unverified |
| Calibration/service interval | UNKNOWN | `source_missing` |

---

## IR4. Heated mixer — Eppendorf ThermoMixer C (base unit; blocks and lids as conditional variants)

### IR4.1 Identity

| Field | Value |
|---|---|
| profile id | `inst-thermomixer-eppendorf-c` |
| version | `v0.1.0-research_draft` |
| class | dry-block heater/cooler with orbital mixing (Peltier-cooled) |
| manufacturer / model | Eppendorf ThermoMixer C |
| documented configuration | Base unit only; SmartBlocks (tube/plate/DWP/PCR formats), ThermoTop heated lid, and SmartExtender are interchangeable accessories and are recorded as conditional variants, not base capabilities |
| control | On-device display (time/temperature/speed/program); USB interface; multi-step programmable programs (S6/S7 summaries) |

### IR4.2 Capabilities

| Key | Value / range / set | Unit | Conditions | Evidence |
|---|---|---|---|---|
| temperature range | 15 °C below ambient (RT) to 100 | °C | Settable 1–100 °C in 1 °C increments per S7 summary; effective minimum is ambient-dependent | `agent-verified` for range framing (S6 Technical Data: "15 °C below RT, max: 100 °C"); increment/step detail `search-summary-only` (S7) |
| temperature accuracy | ±0.5 (20–45 °C); ±1.0 outside 20–45 °C | °C | Block/position-dependent; per-block calibration per S7 summary | Range framing `agent-verified` (S6 highlights); numeric bands `search-summary-only` (S7) — treat as provisional |
| mixing frequency | 300–3,000 | rpm | Block- and vessel-dependent; some blocks/lids cap lower (e.g. 2,000 with lid / deepwell, per S7 summary) | `agent-verified` for 300–3,000 framing (S6); per-format caps `search-summary-only` (S7) |
| mixing orbit | 3 (2D Mix-Control) | mm | S6 | `agent-verified` (S6) |
| heating rate | up to 6.0–7.0 (block-dependent; SmartBlock 1.5 mL ≈ 6.0) | °C/min | Refers to BLOCK rate, not sample temperature; varies with vessel fill | `search-summary-only` (S7) — NOT verified in an opened primary source |
| cooling rate | up to 2.5 (100 °C → RT) | °C/min | Same block-vs-sample caveat | `search-summary-only` (S7) |
| vessel/formats | 0.1 µL–50 mL working range claimed; tubes, MTP/DWP/PCR plates, cryovials via SmartBlocks; PCR 96/384-well; DWP 96-well 500/1000 µL | — | Each format requires its matching SmartBlock; polystyrene >70 °C melt and polypropylene DWP >80 °C deformation limits per S7 summary | `agent-verified` for format classes (S6); volume extremes and plastic limits `search-summary-only` (S7) |
| programs | up to 20 slots / 5 quick-access keys (variant wording across S6/S7 summaries) | — | Exact slot/key split unverified | `search-summary-only` (S7) |
| dimensions / weight / power | 20.6 × 30.4 × 13.6 cm; 6.3 kg; 100–130 or 220–240 V, 50/60 Hz, max 200 W | — | S6 Technical Data | `agent-verified` (S6) |
| data interface | USB | — | S6/S7 summaries | `search-summary-only` (S7) |

### IR4.3 Container / consumable support — `partial`

- Supported classes (agent-verified, S6): tubes, microplates, deepwell plates, PCR plates, cryovials — via interchangeable SmartBlocks with tool-free exchange and block recognition.
- Exact SmartBlock catalog (well counts, tube diameters 11.0–11.9 mm, 15/50 mL conical, 2.0 mL cryovessel rows seen in S7 summary): `search-summary-only`, partial list — do not treat as complete or committed.

### IR4.4 Conditions

- Minimum temperature is ambient-relative (15 °C below RT), not an absolute setpoint — lab ambient is a precondition.
- Heating/cooling rates describe the BLOCK, not the sample; sample profiles lag and depend on fill, vessel, and ThermoTop use (per S7 summary guidance).
- Maximum speed is block-capped by automatic block recognition; lid/deepwell formats cap lower — never state 3,000 rpm as universal.

### IR4.5 Raw outputs

No analytical readout. Device state available to the operator: set vs. displayed block temperature, mixing speed, elapsed/remaining time, active program step, ThermoTop status. Export/logging details UNKNOWN.

### IR4.6 Actual-device boundary

Static profile only. The following are Run/device state: which SmartBlock is mounted, ThermoTop on/off, calibration-certificate currency, IQ/OQ status, ambient temperature at run time, actual sample temperature, service history.

### IR4.7 UNKNOWNs

| Item | Status | Reason / UNKNOWN class |
|---|---|---|
| Operating-manual technical-data table (exact rows) | UNKNOWN | `source_missing` — manual PDF (S7) returned HTTP 403, never opened |
| Per-block temperature accuracy/homogeneity table | UNKNOWN | `source_missing` — only summary bands available |
| Per-format maximum rpm table | UNKNOWN | `configuration_dependent` + `source_missing` — caps vary by block/lid |
| Timer/run-time range (3:00–5:15 seen in one summary) | UNKNOWN | `source_missing` — single-summary value, unit/scope unclear, not transferred |
| Temperature-setting increment (1 °C) | UNKNOWN (provisional `search-summary-only`) | `source_missing` — manual not opened |
| USB protocol / export format | UNKNOWN | `source_missing` |
| Calibration certificate scope and interval | UNKNOWN | `device_state` + `source_missing` |
| Ambient operating envelope (5–40 °C seen in summary) | UNKNOWN (provisional) | `source_missing` — manual not opened |

---

## Evidence status and UNKNOWN rollup

| Record | Agent-verified rows | Search-summary-only rows | UNKNOWNs | Compatible? |
|---|---|---|---|---|
| IR1 QuantStudio 5 (96-well 0.2 mL) | format, multiplexing, VeriFlex zones, discrimination, memory, cloud, dimensions | reaction volume, ramp rates, uniformity, filters, dynamic range, dye set | optics bands, accuracy, 384/0.1 mL variants, lid, calibration, exports | NO — `research_draft`, no ReviewRecord |
| IR2 FACSCanto II (2-laser 4-2) | all base-configuration rows (S3 pp. 2–4) + conditional variants | violet-dye mapping detail (S4) | PMT curves, spillover, gate cap, IT currency, service | NO — `research_draft`, no ReviewRecord |
| IR3 NanoDrop One/OneC | entire spec table (S5) | none | cuvette mechanics, 37 °C accuracy, stir rpm, lamp life, SW versions, service | NO — `research_draft`, no ReviewRecord |
| IR4 ThermoMixer C | range framing, orbit, speed framing, format classes, dimensions/power | accuracy bands, rates, volume extremes, programs, USB, ambient | manual table, per-block tables, timer, calibration | NO — `research_draft`, no ReviewRecord |

- No record above claims `SourceChecked`: per record-standard §4, that requires a second person to re-verify every field against the source plus a ReviewRecord. None exists.
- Platform-vs-device discipline: QS5 block variants, FACSCanto laser/filter/Loader/HTS options, NanoDrop OneC cuvette position, and ThermoMixer SmartBlocks/ThermoTop are recorded as conditional variants or UNKNOWNs — none are stated as present on any actual unit.
- Review status for all four: `research_draft`. Next step toward `source_checked`: open the QS spec-sheet PDF (S2), the Filter Guide (S4), and the ThermoMixer C manual (S7) directly, replace every `search-summary-only` row with a paginated citation or a classified UNKNOWN, and submit all four records with a ReviewRecord for second-person verification.
