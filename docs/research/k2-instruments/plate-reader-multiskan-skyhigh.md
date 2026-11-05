# Multiskan SkyHigh Absorbance Microplate Reader — Capability Profile

`review_status: research_draft`

> Personally verified: all 13 spec-table rows below were read by me from the two
> official Thermo Fisher sources listed in §2 (spec-sheet PDF + product page).
> I did NOT verify an operating/user manual or the SkanIt software manual
> version compatibility (listed as UNKNOWN). Do NOT claim SourceChecked.

## 1. Identity

| Field | Value |
|---|---|
| Instrument | Thermo Scientific Multiskan SkyHigh Microplate Spectrophotometer |
| Role | UV/Vis absorbance microplate spectrophotometer (monochromator-based) |
| Manufacturer | Thermo Fisher Scientific |
| Configurations | 3: touchscreen stand-alone; touchscreen + cuvette; SkanIt PC-software-only (no touchscreen) |
| Control software | 7 in. touchscreen (stand-alone) and/or Thermo Scientific SkanIt Software (PC) |
| Research-use note | For Research Use Only. Not for use in diagnostic procedures. |

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator |
|---|---|---|---|
| S1 | Spec sheet PDF (COL33729 0720), "Technical specification for the Thermo Scientific Multiskan SkyHigh Microplate Spectrophotometer" | https://documents.thermofisher.com/TFS-Assets/BID/Flyers/multiskan-sky-microplate-spectrophotometer-specifications.pdf | Full 1-page spec table |
| S2 | Product page "Multiskan SkyHigh Microplate Spectrophotometer", Specifications section | https://www.thermofisher.com/us/en/home/life-science/lab-equipment/microplate-instruments/plate-readers/models/multiskan-skyhigh.html | "Specifications" table (Technical + General specifications) |

S1 and S2 agree on every overlapping value except the two noted discrepancies
(incubation wording, weight) — see Conditions §4.

## 3. Capability table

| Capability | Value | Source |
|---|---|---|
| Wavelength selection / optics | Monochromator; xenon flash lamp | S1 spec table rows 1–2; S2 same rows |
| Wavelength range | 200–1,000 nm, 1 nm steps | S1 "Wavelength range" row; S2 same row |
| Bandwidth (spectral) | < 2.5 nm | S1 "Bandwidth" row; S2 same row |
| Plate formats | 6- to 48-, 96-, and 384-well plates; Thermo Scientific µDrop / µDrop Duo plates | S1 "Plates" row; S2 same row |
| Reading modes | Absorbance endpoint + spectral scanning (full spectrum 200–1,000 nm in ≤10 s/well); cuvette reading on cuvette-equipped configurations only | S1 "Spectral scanning speed" row + Features/config table; S2 "Available options" config table + Features list |
| Readout range | Up to 4 absorbance units | S1 "Readout range" row; S2 same row |
| Linear range (450 nm) | 0–2.5 Abs, 2% | S1 "Linearity at 450 nm" row; S2 same row |
| Accuracy (450 nm) | 1.0% + 0.003 Abs (0–2.0 Abs); 2.0% (2.0–2.5 Abs) | S1 "Accuracy at 450 nm" row; S2 same row |
| Precision (450 nm) | SD < 0.003 Abs or CV < 1.0% | S1 "Precision at 450 nm" row; S2 same row |
| Incubation / temperature range | S1: "From ambient temperature to 45 °C". S2: "From ambient temperature +2C to 45 °C" | S1 "Incubation range" row; S2 "Incubation range" row (wording differs; see §4) |
| Shaking | Linear plate shaking (no speed/duration given) | S1 "Plate shaking" row; S2 same row |
| Reading speed | 6 s per 96-well plate; 10 s per 384-well plate (A1 back to A1) | S1 "Measurement speed" row; S2 same row |
| Supported consumables | Microplates (max height with lid 19.5 mm); µDrop/µDrop Duo plates (up to 32 low-volume samples); cuvette on cuvette models — H 40–58 × W 12.5 × D 12.5 mm, beam center 8.5 mm, beam window ≥ 2 mm | S1 "Cuvette" row + plate footnote; S2 same + "Available options" table |
| Interfaces / connectivity | 1× USB-B (PC), 1× Ethernet, 3× USB-A (memory, Wi-Fi dongle); touchscreen models add cloud (Thermo Fisher Connect, OneDrive) | S1 "Connections" + "User interfaces" rows; S2 same + Features list |
| Dimensions / weight / power | 265 × 295 × 445 mm; 11.3 kg per S1 (S2: 10.6–11.5 kg model-dependent); 100–240 V, < 110 W (S2 only) | S1 bottom rows; S2 bottom rows |

## 4. Conditions and footnotes (from sources)

- Photometric specs (linearity/accuracy/precision) are valid for 96-well flat-bottom microplates and quartz semimicro cuvettes with a 4 mm beam window (S1/S2 footnote `*`).
- Maximum plate height with lid is 19.5 mm (S1/S2 footnote `**`/`***`).
- Incubation wording discrepancy: S1 (spec sheet, 2020) says "From ambient temperature to 45 °C"; S2 (live product page) says "From ambient temperature +2C to 45 °C". Treat the lower bound as configuration/ambient-dependent; use the S2 wording for conservative planning.
- Weight discrepancy: S1 states 11.3 kg; S2 states 10.6–11.5 kg depending on model (touchscreen/cuvette options). Both official; range reflects configurations.

## 5. UNKNOWNs (not in verified sources — do not use for planning)

| Item | Status | Reason |
|---|---|---|
| Shaking speed / amplitude / duration ranges | UNKNOWN | Sources state "Linear" only, no numeric range |
| Incubation uniformity / control accuracy | UNKNOWN | No value in S1 or S2 |
| Kinetic / continuous-read mode timing | UNKNOWN | Only endpoint + spectral scan speeds stated |
| Fluorescence / luminescence modes | UNKNOWN | Absorbance-only instrument per sources; no such modes claimed |
| Filter wheel / cut-off filters | UNKNOWN | Monochromator selection; no filters specified |
| Temperature calibration certificate | UNKNOWN | Not stated in sources |
| Plate-height/offset auto-adjustment | UNKNOWN | Only max-height limit stated |
| SkanIt software version compatibility | UNKNOWN | User manual (N21873) and SkanIt manual not verified in this draft |
| Service/calibration interval | UNKNOWN | Not stated in sources |

## 6. What was verified vs. not

- Personally verified (read 2026-11-05): every row in §3 against S1 and S2, including both footnoted discrepancies in §4.
- NOT verified: operating manual N21873 contents, SkanIt software version requirements, µDrop Duo plate part-number compatibility beyond the "Available options" bundle table, regulatory/CE status.
