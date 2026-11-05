# Heracell VIOS 160i CO2 Incubator — Capability Profile

`review_status: research_draft`

> Personally verified: every row in §3 was read by me from the three official
> Thermo Fisher sources in §2 (product page, brochure spec table, operating
> manual). I did NOT verify service/calibration intervals or the SkanIt-style
> PC utility beyond what the manual states. Do NOT claim SourceChecked.

## 1. Identity

| Field | Value |
|---|---|
| Instrument | Thermo Scientific Heracell VIOS 160i CO2 Incubator |
| Role | Direct-heat CO2 incubator for cell/tissue culture (165 L class) |
| Manufacturer | Thermo Fisher Scientific (Thermo Electron LED GmbH, Langenselbold) |
| Configurations | Single 165 L chamber; interior electropolished stainless steel OR 100% copper; CO2 sensor TC180 (thermal conductivity + rH compensation) OR IR180Si (single-beam dual-wavelength IR); optional O2 control 1–21% or 5–90%; optional 3-segment gas-tight inner doors; stackable ×2 same-type with adapter |
| Reference catalog no. | 51030400 (single 165 L, stainless, IR sensor, 120 V North America) |
| Control software | iCan touchscreen on door; USB data/error logging; optional 4–20 mA output |
| Research-use note | Product page states "intended for General Laboratory Use"; customer responsible for suitability to application |

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator |
|---|---|---|---|
| S1 | Product page "Heracell VIOS 160i CO2 Incubator, 165 L, Electropolished Stainless Steel", cat. 51030400 | https://www.thermofisher.com/order/catalog/product/51030400 | "Specifications" table (full page) |
| S2 | Brochure "Heracell VIOS CO2 Incubator" BRCO2VIOSNA-EN 1020 | https://documents.thermofisher.com/TFS-Assets/LPD/brochures/Heracell%20VIOS%20CO2%20Incubator%20Brochure%20BRCO2VIOSNA-EN%201020%20Web.pdf | pp. 15–16, "| specifications |" table, 160i column |
| S3 | User/Operating Manual HERACELL VIOS 160i / 250i, doc 50162755, March 2020 (= 50144132 Rev A) | https://bme.usc.edu/wp-content/uploads/2024/08/TFS-Heracell-VIOS-CO2-Incubator-User-Manual-50144132-A-EN.pdf (mirror of official Thermo Fisher manual; doc no. on p. 1) | Ch. 3 (device description), Ch. 6 (handling/control + troubleshooting pp. 6-45–6-48), Ch. 11 Specifications pp. 11-2–11-5 |

## 3. Capability table

| Capability | Value | Source |
|---|---|---|
| Chamber volume | 165 L total (5.8 cu ft); useable ≈ 100 L | S2 spec table; S3 p. 11-2 ("Inner space volume, of which useable volume: approx. 165 / approx. 100") |
| Chamber dimensions (W×H×D) | Interior 470 × 607 × 576 mm (18.5 × 23.9 × 22.7 in); exterior 637 × 905 × 790 mm (S3) / 637 × 900 × 880 mm (S2; depth differs, see §4) | S3 p. 11-2; S2 spec table; S1 same interior dims |
| Chamber material | Electropolished stainless steel OR 100% solid copper; air duct/shelves match interior material | S1 specs ("Chamber Material"); S3 p. 3-17 |
| Temperature control range | Room temperature + 3 °C to 55 °C (room must be ≥ 18 °C) | S1 ("Ambient +3° to 55°C"); S2 spec table ("Range 3°C above ambient to 55°C"); S3 p. 11-2 + p. 3-6 |
| Temperature control deviation (time, DIN 12880-2, 37 °C) | ± 0.1 °C | S2 spec table ("Control ±0.1°C"); S3 p. 11-2 |
| Temperature uniformity (spatial, DIN 12880-2, 37 °C) | ≤ ± 0.3 °C (S3) / < ± 0.3 °C (S2); standard feature set, see calibration instructions | S3 p. 11-2; S2 spec table |
| Ambient operating range | +18…34 °C; +18…28 °C when stacked; max 80% rH ambient; site ≤ 2000 m | S3 p. 11-2; S2 spec table (ambient row) |
| Temperature tracking alarm | ± 1 °C (actual > set + 1 °C or < set − 1 °C after 45 min post-door / 159 min post-setpoint; field-service adjustable) | S2 spec table ("Tracking alarm ±1°C"); S3 troubleshooting pp. 6-46–6-47 + footnotes |
| Overtemperature protection | Independent dual probes + Class 3.1 controller per DIN 12880:2007-05; response at set + 1 °C with visual + audible alarm, auto-reduce to setpoint | S1 ("Dual temperature probes for over temperature protection"); S3 pp. 3-11–3-12, p. 11-2 |
| CO2 measurement/control range | 0–20 vol% | S2 spec table; S3 pp. 11-4–11-5 |
| CO2 control deviation (time) | ± 0.1 vol% | S2 spec table; S3 p. 11-4 |
| CO2 accuracy (37 °C, 5% CO2) | ± 0.3% CO2 | S2 spec table (160i column); S3 p. 11-4 |
| CO2 tracking alarm | ± 1% (actual > set + 1% or < set − 1%) | S2 spec table ("Tracking alarm ±1%"); S3 p. 6-47 troubleshooting |
| CO2 sensor options | TC180 thermal-conductivity with rH compensation, OR IR180Si single-beam dual-wavelength IR with MEMS emitter (temp-resistant, stays in chamber during Steri-Run) | S1 ("Choice of TC with rh compensation or IR"); S3 p. 3-11; S2 brochure p. 6 |
| O2 control ranges (optional) | 1–21% (hypoxic, N2-displaced) or 5–90% (hyperoxic, O2-fed); zirconium-oxide sensor | S1 ("O2 Concentration Range: Optional 1 to 21% or 5 to 90%"); S2 spec table; S3 p. 11-4; sensor type S2 brochure p. 5 |
| O2 control deviation / accuracy | ± 0.2 vol% over time; accuracy at 37 °C/21% O2: ± 0.5% (1–21% option), ± 2.0% (5–90% option) | S2 spec table; S3 p. 11-4 |
| O2 tracking alarm | ± 1% | S2 spec table; S3 p. 6-48 troubleshooting |
| Humidity method | Directly heated covered 3 L reservoir (max 3 L / min 0.5 L), no standing water in chamber; pre-humidified process gases; low-humidity mode lowers setpoint ≈ 93% → ≈ 90% rH (permanent setting, extended adaptation) | S1 ("Humidity Source: Covered integrated water reservoir"); S3 pp. 3-6–3-7, p. 11-2 |
| Humidity level at 37 °C | ≈ 93% rH high-humidity mode; ≈ 90% rH low-humidity mode | S2 spec table ("RH >_93% @ 37°C"); S3 p. 11-3; S1 "<93% at 37°C" |
| Water quality requirement | Sterile distilled or equivalent; conductivity 1–20 µS/cm (resistivity 50 kΩ·cm–1 MΩ·cm); pH 7–9; NO chlorinated tap water, NO chloride additives, NO ultrapure/Type I outside range (warranty void) | S3 pp. 3-6–3-7 (two CAUTION boxes) |
| Gas supply requirements | Purity ≥ 99.5% or medical grade; inlet pressure 0.8–1.0 bar (11.6–14.5 psi), must not exceed 1 bar; CO2 + O2/N2 via 1/8 in. hose barbs with HEPA-grade inlet filters (99.998% @ 0.3 µm); optional internal gas guards / external auto-changeover | S2 spec table ("Inlet pressure 12-15 PSI (0.8-1.0 bar)", "Gas purity min. 99.5", "1/8\" hose (barbed)"); S3 pp. 3-15, 4-11–4-12 |
| HEPA filtration | In-chamber HEPA, 99.998% @ 0.3 µm; full chamber volume filtered every 60 s; ISO Class 5 within 5 min of 30 s door opening | S1 ("ISO 5 HEPA Filtration"); S2 brochure pp. 4, 12; S3 pp. 3-7–3-8 |
| Decontamination (Steri-Run) | 180 °C on all interior surfaces, automatic overnight cycle < 12 h total (≈2 h heat-up + ≈90 min hold + ≈8 h cool-down); electromechanical door lock engages > 65 °C | S1 ("Steri-Run … 180°C"); S3 pp. 8-7–8-8, p. 3-24 |
| Steri-Run lethality claim | Brochure: 12-log SAL, third-party validated (180 °C/45 min eliminates all test organisms incl. spores/mycobacteria). Manual: 6-log reduction per ISO 11138. Discrepancy — see §4 | S2 brochure p. 11 table; S3 p. 8-7 |
| Shelves (160i) | Perforated adjustable, 423 × 465 mm; 3 supplied, max 11; 10 kg per shelf / 30 kg total; rails perforated every 42 mm; bottom insert has refill cut-out | S2 spec table ("Number standard/maximum 3/10 … Max. load 10/30 kg"); S3 pp. 3-23, 11-2 (3 supplied / 11 max) |
| Inner doors (optional) | 3-segment gas-tight screen (160i); 6-door Cell Locker variant | S1 ordering options (S2 pp. 18–19 list 51901144 3-door screen); S3 p. 3-18 + Appendix A |
| Documented alarms (control-loop error + alarm relay + audible + log) | Door open > 30 s (message) / > 10 min (alarm+relay); sensor breakage (T/CO2/O2/rH); actual high/low (T ±1 °C, CO2/O2 ±1%); calibration out-of-range; sensor/gas-changeover comms errors; cylinder A/B empty; no gas; rH no-water at 0.5 L; fan error; ADC error; EEPROM/mainboard error; data-logger error; Steri-Run / auto-start errors; power-down during Steri-Run | S3 troubleshooting tables pp. 6-45–6-48; alarm relay spec p. 4-18; S2 spec table tracking-alarm rows |
| QC / data outputs | USB (virtual COM, 9600–57600 baud) with command protocol + PC utility (*.CSV error/data logs, *.SRF service file); data logger up to 10 000 events; optional 4–20 mA (T 0–200 °C, CO2 0–20%, O2 0–100%); potential-free alarm contact; remote alarm contacts standard | S1 ("Data Outputs: USB, and optional 4-20mA"); S2 spec table ("remote alarm contacts, USB, optional 4-20mA"); S3 pp. 3-15–3-16, Ch. 12 |
| Electrical (120 V NA) | 120 V 50/60 Hz; incubation 0.55 kW / 4.6 A; decontamination 1.01 kW / 8.3 A; 16 A branch breaker; heat to room 0.06 kWh/h @37 °C (0.26 avg / 0.78 heat-up / 0.59 hold during Steri-Run) | S1 ("120 V, 50/60 Hz"); S3 pp. 11-4–11-5 (rated current/power rows) |
| Safety/EMC ratings | CE, CSA; IP20; protection class I; overvoltage cat. II; pollution degree 2; EMC class B | S1 ("Certifications/Compliance CE, CSA"); S3 pp. 11-4–11-5 |
| Weight / noise | 83 kg operating (78.4 kg without accessories); < 50 dB(A) per DIN 45635-1 | S2 spec table ("183 lbs"); S3 pp. 11-2–11-3 |

## 4. Conditions and footnotes (from sources)

- CO2 control is disabled at setpoint 0% (air ≈ 0% CO2); O2 control/monitoring disabled at setpoint 21% (S3 p. 6-7 factory presets: 37 °C / 5.0% CO2 / 21.0% O2).
- Lower temperature bound follows the room: at 21 °C room, control starts at 24 °C; in-chamber heat loads shift it further (S3 p. 3-22 table: +5 W → RT+6.5 °C … +20 W → RT+16 °C). Loading surface should be used to max 70% for air circulation (S3 p. 5-4).
- Stacking: max two units of the same type with stacking adapter; stacked ambient limited to +18…28 °C; above 28 °C an overtemperature alarm triggers on the upper unit during Steri-Run (S3 pp. 2-3–2-7, p. 11-2).
- Steri-Run lethality discrepancy: brochure claims 12-log SAL / "6-log in half the time" (S2 p. 11); manual certifies 6-log reduction per ISO 11138 (S3 p. 8-7). Use the manual's 6-log figure for conservative planning.
- Exterior-depth discrepancy: S2 lists 880 mm depth vs S3's 790 mm (width/height agree). Interior dims and weight agree across all three sources; treat depth as configuration/packaging-dependent, UNKNOWN for tight-fit planning.
- HEPA cartridge is NOT autoclavable, max 60 °C — must be removed before Steri-Run; airbox (thermoresistant plastic) stays in (S3 pp. 3-17, 8-2). Pre-filter stays in during Steri-Run but is removed for refilling (S3 pp. 3-7, 5-2).
- Access port Ø 42 mm with plug + separate pressure-compensation opening (must not be connected to exhaust); silicone plug moved to outer side before Steri-Run (S3 p. 3-22).
- Auto-start routine to 37 °C at 20 °C ambient takes 5–10 h (cold unit); validates sensors and establishes atmosphere before loading (S3 p. 11-2, pp. 5-3–5-4).

## 5. UNKNOWNs (not in verified sources — do not use for planning)

| Item | Status | Reason |
|---|---|---|
| Recovery time as certified spec | UNKNOWN | Brochure cites < 10 min for all parameters after 30 s door opening from *internal testing* (S2 pp. 4–5); not a DIN-rated spec value |
| CO2/O2 sensor calibration interval | UNKNOWN | Manual gives procedures (Ch. 9) but no fixed interval read in this draft |
| HEPA / prefilter / gas-inlet-filter replacement interval | UNKNOWN | Manual Ch. 9 replacement procedures exist; interval reminder is user-programmable (S3 p. 6-39); no fixed hours stated in verified pages |
| Preventive-maintenance / service interval | UNKNOWN | Not stated in verified pages |
| 100 V model power/current for 160i | Verified present in table but not transcribed (230/220/120 V rows used) | S3 p. 11-4 contains the row; omitted as out-of-scope for NA 120 V profile |
| Stacked-unit thermal-separation accessory part no. | UNKNOWN | Required adapter noted (S3 p. 2-7) but part number not verified |
| Firmware / PC-utility OS compatibility beyond driver note | UNKNOWN | Manual driver note lists WIN 7/8/2000/XP/Vista era (S3 p. 12-1); current compatibility unverified |

## 6. What was verified vs. not

- Personally verified (read 2026-11-05): every row in §3 against S1 (product-page spec table), S2 (brochure pp. 15–16 spec table + pp. 4–12 feature pages), and S3 (manual Ch. 3, Ch. 6 control/troubleshooting, Ch. 11 spec tables pp. 11-2–11-5, Ch. 12 interfaces).
- NOT verified: service/calibration/filter intervals (manual Ch. 9 only skimmed for procedure titles), 100 V electrical row, Cell Locker appendix beyond scope note, current PC-utility OS support, pricing/availability.
