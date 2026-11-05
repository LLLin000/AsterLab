# Manual Pipette — Eppendorf Research plus (research draft)

`review_status: research_draft` — NOT source-checked. Every numeric capability below
carries its source locator; anything not verified is marked UNKNOWN. No values invented.

## 1. Identity

- **Manufacturer:** Eppendorf AG, Hamburg, Germany.
- **Model family:** Eppendorf Research® plus — mechanical piston-stroke pipette,
  air-cushion principle (§1.1: pressing the control button moves the piston; upward
  stroke aspirates into the tip, downward stroke dispenses; stroke past the first stop
  = blow-out).
- **Documented configurations in the verified manual (2013 ed.):**
  single-channel variable volume, single-channel fixed volume, 8-channel variable,
  12-channel variable. Manual states overall dispensable range 0.1 µL–10 mL depending
  on model (§1.1). The 8→12-channel lower part is convertible within the same volume
  range (§1.3.2).
- **Configurations existing on the current Eppendorf product page but NOT in the
  verified 2013 manual:** 16-channel, 24-channel (4.5 mm cone distance, for 384-well
  plates), and Move It® adjustable-tip-spacing variants. Their volume ranges and error
  limits are UNKNOWN from my verified sources (see §6).

## 2. Sources (retrieved 2026-11-05)

| # | Source | Locator used |
|---|--------|--------------|
| S1 | Operating manual, Eppendorf Research plus, doc 3120 900.012-06/042013, © 2013 Eppendorf AG, via mirror https://pipette.com/mm5/pdfs/manuals-brochures/Operating-manual-Research-plus.pdf | §§1–8, pp. 5–35 of the PDF (technical data §6 pp. 29–31; ordering info §7 pp. 33–34; ambient §6.4 p. 31; autoclaving §5.2.1 p. 16; adjustment §5.5 pp. 23–26; operation §§3.1–3.4 pp. 11–12; materials §1.4 p. 8) |
| S2 | Eppendorf product page, "Looking for the original Research pipette?" https://www.eppendorf.com/us-en/Products/Liquid-Handling/All-Pipettes-Dispensers-Automated-Liquid-Handlers/Eppendorf-Research-plus-p-PF-534798 | channel-availability statements (1-, 8-, 12-, 16-, 24-channel + Move It), 16/24-channel 4.5 mm cone-distance listings |
| S3 (attempted, NOT verified) | Official Eppendorf PDFs https://www.eppendorf.com/product-media/doc/en/186591/…Operating-manual…pdf and …/672922/…Performance-Data-Sheet.pdf | Direct fetch returned HTTP 403 on 2026-11-05; content verified only via S1 mirror of the identical document (same doc number). Current performance-data-sheet values UNKNOWN. |

## 3. Capability table — variants documented in S1

Increments, test tips, and error limits are Eppendorf AG factory error limits per
S1 §6 (test conditions: ISO 8655-1/-2/-6, fine balance with evaporation protection,
10 determinations, distilled water per ISO 3696, 20–25 °C ± 0.5 °C, pre-wetted tip,
dispensing against tube wall — S1 p. 31).

### 3a. Single-channel, variable volume (S1 §6.1 p. 29 + §7.1 p. 33)

| Model (volume range, button color) | Increment | Test tip (epT.I.P.S., color / range / length) | Test volume → systematic / random (Eppendorf limits) |
|---|---|---|---|
| 0.1–2.5 µL, dark gray | 0.002 µL | 0.1–10 µL, 34 mm | 0.1 µL → ±48 % / ±0.048 µL, ±12 % / ±0.012 µL; 0.25 µL → ±12 % / ±0.03 µL, ±6 % / ±0.015 µL; 1.25 µL → ±2.5 % / ±0.031 µL, ±1.5 % / ±0.019 µL; 2.5 µL → ±1.4 % / ±0.035 µL, ±0.7 % / ±0.018 µL |
| 0.5–10 µL, medium gray | 0.01 µL | 0.1–20 µL, 40 mm | 0.5 µL → ±8 % / ±0.04 µL, ±5 % / ±0.025 µL; 1 µL → ±2.5 % / ±0.025 µL, ±1.8 % / ±0.018 µL; 5 µL → ±1.5 % / ±0.075 µL, ±0.8 % / ±0.04 µL; 10 µL → ±1.0 % / ±0.1 µL, ±0.4 % / ±0.04 µL |
| 2–20 µL, light gray | 0.02 µL | 0.5–20 µL L, 46 mm | 2 µL → ±5 % / ±0.1 µL, ±1.5 % / ±0.03 µL; 10 µL → ±1.2 % / ±0.12 µL, ±0.6 % / ±0.06 µL; 20 µL → ±1.0 % / ±0.2 µL, ±0.3 % / ±0.06 µL |
| 2–20 µL, yellow | 0.02 µL | 2–200 µL, 53 mm | 2 µL → ±5 % / ±0.1 µL, ±1.5 % / ±0.03 µL; 10 µL → ±1.2 % / ±0.12 µL, ±0.6 % / ±0.06 µL; 20 µL → ±1.0 % / ±0.2 µL, ±0.3 % / ±0.06 µL |
| 10–100 µL, yellow | 0.1 µL | 2–200 µL, 53 mm | 10 µL → ±3 % / ±0.3 µL, ±1 % / ±0.1 µL; 50 µL → ±1 % / ±0.5 µL, ±0.3 % / ±0.15 µL; 100 µL → ±0.8 % / ±0.8 µL, ±0.2 % / ±0.2 µL |
| 20–200 µL, yellow | 0.2 µL | 2–200 µL, 53 mm | 20 µL → ±2.5 % / ±0.5 µL, ±0.7 % / ±0.14 µL; 100 µL → ±1 % / ±1 µL, ±0.3 % / ±0.3 µL; 200 µL → ±0.6 % / ±1.2 µL, ±0.2 % / ±0.4 µL |
| 30–300 µL, orange | 0.2 µL | 20–300 µL, 55 mm | 30 µL → ±2.5 % / ±0.75 µL, ±0.7 % / ±0.21 µL; 150 µL → ±1 % / ±1.5 µL, ±0.3 % / ±0.45 µL; 300 µL → ±0.6 % / ±1.8 µL, ±0.2 % / ±0.6 µL |
| 100–1000 µL, blue | 1 µL | 50–1000 µL, 71 mm | 100 µL → ±3 % / ±3 µL, ±0.6 % / ±0.6 µL; 500 µL → ±1 % / ±5 µL, ±0.2 % / ±1 µL; 1000 µL → ±0.6 % / ±6 µL, ±0.2 % / ±2 µL |
| 0.5–5 mL, violet | 0.005 mL | 0.1–5 mL, 120 mm | 0.5 mL → ±2.4 % / ±12 µL, ±0.6 % (random µL value truncated in extraction — UNKNOWN, see §6); 2.5 mL → ±1.2 % / ±30 µL, ±0.25 %; 5.0 mL → ±0.6 % / ±30 µL, ±0.15 % |
| 1–10 mL, turquoise | 0.01 mL | 1–10 mL, 165 mm | 1.0 mL → ±3 % / ±30 µL, ±0.6 %; 5.0 mL → ±0.8 % / ±40 µL, ±0.2 %; 10.0 mL → ±0.6 % / ±60 µL, ±0.15 % |

Order numbers (S1 §7.1 p. 33, Intl / North America): 3120 000.011/3120000011
(0.1–2.5 µL) … 3120 000.089/3120000089 (1–10 mL). Full list in S1; numbers may be
superseded — verify against current Eppendorf shop before ordering.

### 3b. Multi-channel, variable volume — 8- and 12-channel (S1 §6.2 p. 30 + §7.3 p. 34)

| Model (per channel count) | Increment | Test tip | Test volume → systematic / random |
|---|---|---|---|
| 0.5–10 µL, medium gray (8-ch 3122 000.019; 12-ch 3122 000.027) | 0.01 µL | 0.1–20 µL, 40 mm | 0.5 µL → ±12 % / ±0.06 µL, ±8.0 %; 1 µL → ±8.0 % / ±0.08 µL, ±5.0 %; 5 µL → ±4.0 % / ±0.2 µL, ±2.0 %; 10 µL → ±2.0 % / ±0.2 µL, ±1.0 % |
| 10–100 µL, yellow (8-ch 3122 000.035; 12-ch 3122 000.043) | 0.1 µL | 2–200 µL, 53 mm | 10 µL → ±3.0 % / ±0.3 µL, ±2.0 %; 50 µL → ±1.0 % / ±0.5 µL, ±0.8 %; 100 µL → ±0.8 % / ±0.8 µL, ±0.3 % |
| 30–300 µL, orange (8-ch 3122 000.051; 12-ch 3122 000.060) | 0.2 µL | 20–300 µL, 55 mm | 30 µL → ±3.0 % / ±0.9 µL, ±1.0 %; 150 µL → ±1.0 % / ±1.5 µL, ±0.5 %; 300 µL → ±0.6 % / ±1.8 µL, ±0.3 % |

Multi-channel random-error absolute (µL) values for the 10/100/300 µL heads were
rendered ambiguously in extraction (S1 p. 30 lower block); treat absolute-µL random
errors for multi-channel as UNKNOWN — use the verified % values above. Cone spacing
for 8/12-channel: UNKNOWN in S1 (current page states 9 mm for 96-well use, S2 —
not verified from S1; listed as S2-observed, not a spec claim).

### 3c. Single-channel, fixed volume (S1 §6.3 p. 31 + §7.2 p. 33)

| Nominal | Test tip | Systematic | Random |
|---|---|---|---|
| 10 µL (medium gray, 0.1–20 µL/40 mm) | epT.I.P.S. 0.1–20 µL | ±1.2 % / ±0.12 µL | ±0.6 % / ±0.06 µL |
| 20 µL (light gray, 0.5–20 µL L/46 mm) | epT.I.P.S. 0.5–20 µL L | ±0.8 % / ±0.16 µL | ±0.3 % / ±0.06 µL |
| 10/20/25/50/100/200 µL (yellow, 2–200 µL/53 mm) | epT.I.P.S. 2–200 µL | 10 µL ±1.2 %/±0.12; 20 µL ±1.0 %/±0.2; 25 µL ±1.0 %/±0.25; 50 µL ±0.7 %/±0.35; 100 µL ±0.6 %/±0.6; 200 µL ±0.6 %/±1.2 µL | 10 µL ±0.6 %/±0.06; 20 µL ±0.3 %/±0.06; 25 µL ±0.3 %/±0.08; 50 µL ±0.3 %/±0.15; 100 µL ±0.2 %/±0.2; 200 µL ±0.2 %/±0.4 µL |
| 200/250/500/1000 µL (blue, 50–1000 µL/71 mm) | epT.I.P.S. 50–1000 µL | 200 µL ±0.6 %/±1.2; 250 µL ±0.6 %/±1.5; 500 µL ±0.6 %/±3; 1000 µL ±0.6 %/±6 µL | 200 µL ±0.2 %/±0.4; 250 µL ±0.2 %/±0.5; 500 µL ±0.2 %/±1; 1000 µL ±0.2 %/±2 µL |

## 4. Tips, liquids, calibration

- **Recommended tips:** epT.I.P.S. recommended throughout (S1 §§1.3.1, 1.3.2, 3.2).
  Control-button color matches the epT.I.P.S. tray color (S1 §1.3.1). Special tip
  epT.I.P.S. LoRetention recommended for liquids whose surface tension is lower than
  water (e.g. wetting agents) (S1 §3.2 p. 11). 5 mL/10 mL pipettes: use WITH protection
  filter for standard (non-filter) tips; REMOVE the pipette's protection filter when
  using 5 mL/10 mL ep Dualfilter T.I.P.S. or third-party 5/10 mL filter tips (S1 §§2.2, 3.2).
- **Applicable liquids:** General laboratory liquid transfer with aqueous-like liquids
  under the ISO 8655 test conditions above. S1 warns: solutions differing greatly from
  water (density, viscosity, surface tension, vapor pressure) or temperature differences
  between pipette/tip/liquid cause incorrect volumes — keep 20–25 °C ± 0.5 °C, avoid
  gradients, pre-wet tip 1–3×, verify by weighing (S1 §§2.2, 5.5.1). Density example:
  ±10 % aqueous density change ≈ ±0.2 % volume change (S1 §5.5.1 p. 23). High-altitude
  use requires adjustment to ambient air pressure (S1 §§4.1, 5.5.1). In-vivo use:
  UNKNOWN (S1 §2.1 defers intended use to a separate enclosed document I did not verify).
- **Chemical compatibility:** Check material compatibility before organic solvents /
  aggressive chemicals (S1 §§1.4, 5). User-accessible materials: upper part PP/PC/PEI;
  lower parts PP, PVDF, PEI, PPS, PEEK, PTFE, EPDM, silicone, stainless/spring steel;
  tips PP; dual-filter PE (S1 §1.4 p. 8). Never clean with acetone; no corrosive fluids,
  strong solvents, abrasives, or sharp objects (S1 §5.1).
- **Calibration/adjustment:** Factory-adjusted (gray ADJ seal, display "0"); per-unit
  Eppendorf Certificate with delivery errors enclosed; changing factory adjustment voids
  the certificate (S1 §§5.5.1, 5.5.3). User (temporary) adjustment via supplied Allen key
  + red seal for specific liquids, with gravimetric verification per EN ISO 8655-2/-6;
  fixed-volume models support user adjustment only (S1 §§5.5.2–5.5.3). Eppendorf PICASO
  software (≥ v2.3) + SOP on website/CD for error determination and periodic checks
  (S1 §4.1 p. 13). Gravimetric re-test required after any lower-part exchange or
  reassembly (S1 §§5.4.1–5.4.3).
- **Sterility/maintenance:** Fully steam-autoclavable, whole unit or lower part removed
  (do not disassemble lower part): 121 °C, 20 min, 1 bar overpressure; no added
  disinfectants/NaOCl during autoclaving; single-use protection filter for 5/10 mL
  replaced after autoclaving; no re-test needed if procedure followed (S1 §5.2.1 p. 16).
  Surface disinfection: wipe with disinfectant/DNA-RNA agent/70 % isopropanol; tip cone
  + ejector sleeve may see 4 % NaOCl then thorough rinse with demineralized water;
  temporary UV ≥ 254 nm tolerated (S1 §5.2.2 p. 17). Tips are single-use (S1 §§2.2, 3.4).
  100 µL/300 µL multi-channel lower parts use replaceable O-rings (S1 §5.3).

## 5. Operating conditions (S1 §6.4 p. 31)

| Condition | Value |
|---|---|
| Operating | 5–40 °C, 10–95 % relative humidity |
| Storage (without transport packaging) | −5 to 45 °C, 10–95 % relative humidity |
| Gravimetric test climate | 20–25 °C constant ± 0.5 °C (S1 §§2.2, 5.5.1, 6.3) |

## 6. Explicit unknowns

1. **16-/24-channel and Move It volume ranges, increments, error limits, cone-spacing
   specs** — exist per S2 product page but absent from verified manual S1; UNKNOWN.
2. **Current order numbers / 1200 µL or additional newer variants** — S1 (2013) may be
   outdated; UNKNOWN, verify at eppendorf.com before use.
3. **Multi-channel absolute-µL random errors** for 10/100/300 µL heads — ambiguous in
   extraction; % values verified, absolute µL UNKNOWN.
4. **0.5–5 mL random error in µL at 0.5 mL** — truncated in extraction; UNKNOWN.
5. **Current performance-data-sheet (S3) values** — official PDF blocked (403);
   UNKNOWN whether limits changed since 2013.
6. **In-vivo / clinical applicability** — UNKNOWN (intended-use sheet not verified).
7. **Weight, length, force specifications** — not stated with figures in S1; UNKNOWN.

## 7. Fields I personally verified (no SourceChecked claim)

- Family identity, air-cushion principle, 0.1 µL–10 mL family span, 8/12-channel +
  fixed/single-variable configuration set, 8↔12-channel convertibility: S1 §§1.1, 1.3.2.
- All §3a/§3b/§3c volume ranges, increments, tip color/range/length pairings, and error
  limits except the items flagged UNKNOWN in §6: S1 §6 pp. 29–31.
- ISO 8655 test conditions, operating/storage climate: S1 §§6.3–6.4 pp. 31.
- Tip recommendation incl. LoRetention + 5/10 mL filter rules: S1 §§3.2, 2.2.
- Autoclave cycle, disinfection/UV rules, materials list, adjustment mechanism,
  certificate-void rule, PICASO/SOP references: S1 §§1.4, 4.1, 5.1–5.5.
- Existence (not specs) of 16/24-channel + Move It: S2 product page.
- I did NOT verify S3 official PDFs (403) and claim no SourceChecked status.
