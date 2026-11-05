# Absorbance Measurement & Fluorescence Imaging — Reusable Readout Modules

`review_status: research_draft`

> Rule: this draft records raw readout only. Optical density (OD) and
> fluorescence intensity / images MUST NOT be stored or reported as
> viability, concentration, or count. Derivation (blank subtraction,
> standard curve, gating, segmentation) is a separate downstream operation
> with its own controls and is out of scope here.

## 1. Identity

| Field | Value |
|---|---|
| Module A | `ABS_READ` — absorbance (endpoint / spectral scan) measurement of a staged plate |
| Module B | `FLUO_IMAGE` — fluorescence imaging of a staged sample (widefield; preserved or fixed) |
| Role | Terminal readout (data-producing); does not transform the biological state except as noted in §6 |
| Companion instrument profiles | `docs/research/k2-instruments/plate-reader-multiskan-skyhigh.md` (`research_draft`); `docs/research/k2-instruments/microscope-leica-dmi8.md` (`research_draft`) |
| Companion assay references | CCK-8 (Dojindo CK04), TMB-ELISA (Thermo), LIVE/DEAD L3224 (Thermo), IF intracellular-antigen / fix-perm-block (Thermo) — all `research_draft`, web-summary level (see §2) |

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator / what was used |
|---|---|---|---|
| S1 | Thermo Multiskan SkyHigh spec sheet PDF (COL33729 0720) | https://documents.thermofisher.com/TFS-Assets/BID/Flyers/multiskan-sky-microplate-spectrophotometer-specifications.pdf | Spec table: wavelength, bandwidth, plates, modes, range, linearity/accuracy/precision, incubation, shaking, speed |
| S2 | Thermo Multiskan SkyHigh product page, Specifications | https://www.thermofisher.com/us/en/home/life-science/lab-equipment/microplate-instruments/plate-readers/models/multiskan-skyhigh.html | Same table + config/footnote wording (incubation + weight discrepancies) |
| S3 | Leica DMi8 spec table | https://www.leica-microsystems.com/products/light-microscopes/p/dmi8/specification/ | Imaging modes, contrast list, filter cubes, EFW, illumination, detectors, focus, stage |
| S4 | Leica DMi8 FAQ | https://www.leica-microsystems.com/products/light-microscopes/p/dmi8/faq/ | Infinity Port, T-House, LID/RFID, ratiometric, hood use, weight ~30 kg, configuration-dependence |
| S5 | Leica DMi8 Product Brochure EN (MC-0009257, 2024) | https://downloads.leica-microsystems.com/DMi8/Brochure%20or%20Flyer/DMi8_Product_Brochure_EN.pdf | FOV 22 mm, Synapse triggering, Navigator/Sample Finder, THUNDER, spinning-disk/GSD/TIRF paths |
| S6 | Dojindo CCK-8 protocol + CK04 manual | https://www.dojindo.com/products/pdf/Protocol/Cell_Counting_Kit-8.pdf ; https://www.dojindo.com/manual/CK04/ | WST-8 → formazan, 450 nm, 10 µL/well (96-well), 1–4 h at 37 °C 5% CO₂, ≥600 nm background correction, 430–490 nm fallback, no-bubble, SDS/HCl stop option |
| S7 | Thermo TMB Substrate Kit UG (MAN0011199) + N301/N600/SS03 pages | https://documents.thermofisher.com/TFS-Assets%2FLSG%2Fmanuals%2FMAN0011199_TMB_Subs_UG.pdf ; https://www.thermofisher.com/order/catalog/product/N301 ; https://www.thermofisher.com/order/catalog/product/SS03 | TMB+peroxide 1:1 mix, 100 µL/well, 15–30 min RT, stop with equal-volume acid (2 M H₂SO₄ or kit stop), blue→yellow, read 450 nm (unstopped blue ~605 nm), read stopped plate within 60 min |
| S8 | Thermo LIVE/DEAD L3224 manual (MP03224) + mammalian-cell protocol + L3224 catalog | https://tools.thermofisher.com/content/sfs/manuals/mp03224.pdf ; https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/live-dead-viability-cytotoxicity-kit-for-mammalian-cells.html | Calcein AM (live, green ~494/517, FITC/GFP filter) + EthD-1 (dead, red ~528/617, RFP/PI/Texas Red filter); 5 µL Calcein AM + 20 µL EthD-1 / 10 mL DPBS; 100–200 µL on cells; 30 min at 20–25 °C; wash to cut serum-esterase background; use fresh ≤1 day; do not fix |
| S9 | Thermo IF protocols (intracellular-antigen BestProtocols; fix-perm-block; growing-staining PDF) | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/immunofluorescence-staining-intracellular-antigens-cultured-cells.html ; https://www.thermofisher.com/us/en/home/life-science/cell-analysis/cell-analysis-learning-center/molecular-probes-school-of-fluorescence/imaging-basics/protocols-troubleshooting/protocols/fix-perm-block.html | 4% formaldehyde/PFA or 100% ice-cold MeOH fix; Triton X-100 permeabilization; serum/BSA block; fluorophore-conjugated secondary; protect from light; mount (Fluoromount-G / Fluoro-Gel) |

Personally verified 2026-11-05 at the level of the instrument-profile drafts (S1–S5 fully transcribed there) and web-summary level for S6–S9. NOT verified: instrument operating manuals, SkanIt manual, LAS X manual, per-build DMi8 configuration, per-lot antibody/dye data sheets.

## 3. Operation contracts

### 3.1 `ABS_READ` — absorbance measurement

| Item | Contract |
|---|---|
| Function | Record per-well absorbance (OD, dimensionless absorbance units) at a specified wavelength, or a spectrum over a range, for a staged plate. No biology interpretation. |
| Input port `plate_in` | Microplate already developed for colorimetric readout (CCK-8 formazan; stopped TMB ELISA). Accepted formats per S1/S2: 6- to 48-, 96-, 384-well; µDrop/µDrop Duo; max height with lid 19.5 mm. Cuvette only on cuvette-equipped configurations (H 40–58 × W 12.5 × D 12.5 mm, beam center 8.5 mm, window ≥ 2 mm). |
| Output port `plate_out` | Same physical plate, preserved (measurement is optical and non-consumptive). Reagent-added wells are NOT returned to a pre-reagent state. |
| Output port `data_out` | Raw matrix: well ID → OD value(s) + wavelength(s) + timestamp + instrument ID + blank-well IDs. Raw only; blank subtraction and curve fitting happen downstream. |
| Preconditions | Plate seated correctly; lid handling per instrument limit; correct plate-type selected in software; wavelength/bandwidth within instrument envelope; blank wells present on the same plate (assay protocols S6/S7 assume same-plate background); no bubbles in wells (S6); TMB stopped before 450 nm read and not overdeveloped/green (S7). |
| Completion checks | Every addressed well returned a value within readout range (≤ 4 Abs per S1/S2); no instrument error flag; wavelength and plate-type recorded in `data_out`; blank-well values present and plausible (recorded, not auto-subtracted). |
| Time scope | Instrument time only: 6 s per 96-well plate, 10 s per 384-well plate (A1→A1, S1/S2); spectral scan ≤ 10 s/well (S1). Development/incubation time (CCK-8 1–4 h; TMB 15–30 min; stopped-TMB stability 60 min) belongs to the upstream assay step, not this module. |
| Instrument capability required | Monochromator absorbance reader covering the assay wavelength; xenon flash lamp; 200–1,000 nm in 1 nm steps; bandwidth < 2.5 nm (all S1/S2). Multiskan SkyHigh is absorbance-only: fluorescence/luminescence modes MUST NOT be assumed (instrument-profile UNKNOWN). Linearity/accuracy/precision claims valid only for 96-well flat-bottom plates and quartz semimicro cuvettes w/ 4 mm beam window (S1/S2 footnote). |

### 3.2 `FLUO_IMAGE` — fluorescence imaging

| Item | Contract |
|---|---|
| Function | Acquire fluorescence image(s) at specified channel(s) and positions from a stained sample. No cell counting, no viability call. |
| Input port `sample_in` | Stained sample in a compatible vessel: chamber slide / dish / multi-well plate on the stage (S5 Navigator templates: "slides, dishes, multi-well plates and more"). Two sub-cases: (a) live/unfixed (LIVE/DEAD S8); (b) fixed + immunolabeled (IF S9). |
| Output port `sample_out` | (a) Preserved: LIVE/DEAD-stained live sample returned unfixed (S8: dyes do not survive fixation — fixation after staining is a failure, not a transition). (b) Terminal-or-archived: fixed IF sample; cells are dead by fixation and the coverslipped/mounted preparation is the archive (S9). Neither output re-enters culture. |
| Output port `data_out` | Raw image set: position ID → channel → exposure/objective/cube/filter metadata + timestamp + instrument ID. No segmentation, no intensity-to-biology mapping. |
| Preconditions | Correct vessel seated within stage load limits; fluorescence path present on this build (optional per S4 — MUST confirm per machine); correct cubes/illumination for the dyes on board; focus strategy set (manual / motorized / closed-loop / AFC per S3–S4); light protection for fluorophores (S9); LIVE/DEAD: cells washed to reduce serum-esterase background, fresh working solution ≤1 day, no prior fix/perm (S8); IF: fix→perm→block completed per S9 before antibody steps. |
| Completion checks | Every requested (position, channel) pair has an image file; channel/cube/exposure/objective/position recorded per frame; no saturated-to-clip or completely dark frame accepted without operator note; focus-lock status recorded where applicable. |
| Time scope | Acquisition time only (per-frame exposure + motion + filter-switch 24 ms where EFW fitted, S3). Stain incubation (LIVE/DEAD 30 min at 20–25 °C, S8) and IF fix/perm/block/antibody incubations (tens of minutes to hours, S9) belong to upstream staining steps. |
| Instrument capability required | Inverted widefield platform with configuration-matched fluorescence: Infinity Port / fluorescence module (manual or motorized, liquid light guide) only if fitted (S4); filter cubes "single to quad" with RFID/LID auto-detect where turret supports it (S4); optional EFW up to 8 dyes (S3); illumination per build (EL6000 / pE300–pE800 family, S3); detector per build (K3M–K8 / Fusion / Kinetix22 family names only, S3); 22 mm FOV optics (S5); closed-loop focus reproducibility < 20 nm / 12 mm travel and AFC drift correction only where fitted (S4); Synapse microsecond triggering only where fitted (S5); scanning stage required for full Sample Finder multi-position (S5 footnote). |

## 4. Parameters (only sourced values; nothing invented)

### 4.1 `ABS_READ` parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `wavelength` | number, nm | S1/S2 (range 200–1,000, 1 nm steps); S6/S7 (450 nm assays) | plan + run | CCK-8: 450 nm (fallback 430–490 nm per S6, 450 nm best sensitivity). Stopped TMB ELISA: 450 nm (S7); unstopped blue ~605 nm (N301 page, S7). Spectral scan: any sub-range of 200–1,000 nm (S1). |
| `reference_wavelength` | number, nm | S6 | run (conditional) | CCK-8 background correction: subtract OD ≥ 600 nm when turbidity high or reducing agents present. Not a second biology signal. |
| `bandwidth` | number, nm | S1/S2 | capability check | Instrument spectral bandwidth < 2.5 nm. Not user-settable per sources read — record as capability, UNKNOWN whether adjustable. |
| `plate_format` | enum (6/12/24/48/96/384, µDrop) | S1/S2 | plan + run | Must match physical plate; photometric specs verified only for 96-well flat-bottom + quartz semimicro cuvette (S1/S2 footnote). |
| `blank_well_ids` | string[] (well IDs) | S6/S7 (same-plate background assumed) | run | Same-plate, same-matrix wells. Module records them; subtraction is downstream. |
| `read_mode` | enum (endpoint, spectral_scan) | S1 | plan + run | SkyHigh per S1: endpoint + spectral scanning. Kinetic/continuous mode: UNKNOWN (not in sources). |
| `incubation_before_read` | boolean + assay time | S6/S7 for assay side; S1/S2 for instrument range | plan | Instrument incubates ambient→45 °C (S1) / ambient+2 °C→45 °C (S2) — discrepancy kept, use S2 conservatively. Shaking: "linear" only, no speed/time (UNKNOWN). CCK-8 1–4 h at 37 °C 5% CO₂; TMB 15–30 min RT; read stopped TMB within 60 min (SS03, S7). |

### 4.2 `FLUO_IMAGE` parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `channels` | array of { dye, ex_nm, em_nm, filter_set } | S8 (L3224 values); S9 (fluorophore-conjugated secondary, no fixed bands) | plan + run | L3224: calcein ~494/517 (FITC/GFP); EthD-1 ~528/617 (RFP; PI/Texas Red also cited; catalog lists 517/617 variant — record the lot manual's pair, do not merge). IF: per secondary-antibody data sheet; simultaneous calcein+EthD-1 viewable with fluorescein longpass; individually with bandpass sets (S8). Dye list for EFW / cube part numbers / LED wavelengths: UNKNOWN (S3–S4 list families only). |
| `exposure` | number, ms (per channel) | General imaging parameter — no sourced value | run | UNKNOWN default. MUST be recorded per frame; no recommended value carried in this draft. |
| `objective` | string (ID / mag / NA / immersion) | S3–S5 list no catalog | plan + run | UNKNOWN catalog. Record objective ID actually used; FURA objectives called out only for UV/ratio work (S4). Correction-collar/SmartCORR and Adaptive Immersion apply only where fitted (S5). |
| `position_ids` | string[] (well/slide-field IDs) | S5 (Navigator multi-position) | run | Slides, dishes, multi-well plates supported via Navigator templates; full Sample Finder needs scanning stage (S5). Stage travel/speed/repeatability: UNKNOWN. |
| `focus_mode` | enum (manual, motorized, closed-loop, AFC) | S3/S4 | plan + run | Closed-loop < 20 nm reproducibility, 12 mm travel; AFC = IR-LED surface tracking (S4). Availability is per-build. |
| `incubation_during_imaging` | object (T/CO₂/humidity) | S3 ("Incubation" row, no ranges) | plan (conditional) | UNKNOWN ranges; consult fitted Okolab/Tokai Hit manual. Live-cell work without a fitted unit is a planning failure. |
| `mounting_medium` | string | S9 | run (IF only) | Fluoromount-G or Fluoro-Gel (S9). Coverslip/mount recorded, not chosen by this module. |

## 5. Controls (required companions; readout alone proves nothing)

| Readout | Required controls (same run) | Why (sourced) |
|---|---|---|
| CCK-8 absorbance | Cell-free medium + CCK-8 blank; vehicle control; (for quantitation) cell-number standard curve — curve fitting itself is downstream, not this module | Medium/reducing agents/phenol red contribute background (S6); dehydrogenase dependence means signal ≠ count without a same-cell-type curve (S6) |
| TMB ELISA absorbance | Blank (substrate+stop, no sample); negative control; standard series for any concentration claim (downstream) | Color development is time/temperature/HRP-amount dependent; overdevelopment → green (S7); stopped signal stable only 60 min (S7) |
| LIVE/DEAD imaging | Unstained control; single-stain (calcein-only, EthD-1-only) controls; dye-titration per cell type starting low (S8) | Serum esterases raise extracellular calcein background — wash first (S8); EthD-1 intensity varies with concentration 0.1–10 µM (S8 summary); bleed-through/assignment needs single-stain references |
| IF imaging | Secondary-only (no-primary) control; unstained/autofluorescence control; per-antigen fix/perm optimization (S9) | Fixation/perm conditions are antigen-dependent (S9); block (serum/BSA) and light protection determine background vs specific signal (S9) |

## 6. Sample transitions (ports + terminality)

- `ABS_READ`: `plate_developed → [ABS_READ] → plate_preserved + raw_OD_matrix`. Non-destructive optically, but the plate is post-reagent: CCK-8 wells contain formazan (optionally SDS/HCl-stabilized up to 24 h RT per S6); ELISA wells contain stopped acid-yellow product (read within 60 min per S7). Re-reads are permitted within stability windows; re-entry to culture or to a pre-reagent state is NOT a transition this module offers.
- `FLUO_IMAGE` (live, e.g. LIVE/DEAD): `cells_live → [stain] → cells_stained_live → [FLUO_IMAGE] → cells_imaged_live + raw_images`. Preserved through imaging (phototoxicity/bleaching caveat in §7); fixation after L3224 staining destroys the readout (S8) — model as forbidden transition.
- `FLUO_IMAGE` (fixed, e.g. IF): `cells_live → [fix→perm→block→label→mount] → preparation_fixed → [FLUO_IMAGE] → preparation_archived + raw_images`. Fixation is terminal for the cells; the mounted preparation is the archivable output. No transition back to live culture exists.

## 7. Failure modes

| # | Failure | Detection (in-module) | Mitigation / disposition |
|---|---|---|---|
| F1 | Bubbles in wells → scatter spikes | Visual check pre-read; single-well outlier vs replicates (S6 warns) | Re-seat/de-bubble per SOP; re-read; flag well if persists |
| F2 | Signal outside linear/readout range (CCK-8 saturates; ELISA overdeveloped green; > 4 Abs) | Value ≥ 2.5 Abs treated as outside linearity (S1/S2 at 450 nm); green TMB noted (S7) | Dilute/re-run upstream with adjusted cell/HRP load or shorter development; record, do not clip-and-interpret |
| F3 | Turbidity / reducing-agent / phenol-red background (CCK-8) | Elevated blank; protocol calls for ≥600 nm subtraction (S6) | Record both wavelengths; subtraction downstream with same-plate blank |
| F4 | Stopped-TMB read too late (> 60 min) or read unstopped at wrong wavelength | Timestamp vs stop-time; 450 vs ~605 mismatch (S7) | Re-develop plate; enforce stop→read clock in plan |
| F5 | Wrong fluorescence configuration (no fluorescence module / wrong cube / LED missing for dye) | Pre-run capability check against build sheet (S4: fluorescence optional) | Block run; reassign to a fitted build |
| F6 | Serum-esterase background (extracellular calcein) | High background in blank/unstaineed fields (S8) | Wash step upstream; re-titrate dye from low end (S8) |
| F7 | Photobleaching / light exposure of fluorophores | Fading across repeat frames; S9 requires light protection | Minimize preview exposure; record exposure history; re-stain if protocol allows |
| F8 | Fix/perm mismatch for antigen (lost epitope or no access) | Secondary-only vs full-stain comparison; S9 antigen-dependence note | Re-optimize fixative (PFA vs MeOH) and Triton conditions upstream; do not rescue by boosting exposure |
| F9 | Focus drift / missed positions (no AFC, no scanning stage) | Missing position IDs; soft frames; S4–S5 fit-dependence | Require focus-mode + stage-capability check at plan; record lock status per frame |
| F10 | Condensation / lid / plate-height violation (19.5 mm limit, S1/S2) | Instrument error or edge-well artifacts | Re-seat without lid or on a compliant plate; flag affected wells |

## 8. Readout meaning — what the numbers/images are and are NOT

- An OD value is light attenuation at (λ, bandwidth) for that well, nothing more. It is NOT viability, NOT cell number, NOT concentration. CCK-8 formazan OD at 450 nm is proportional to dehydrogenase activity under controlled conditions (S6) — proportionality to living-cell count requires a same-cell-type standard curve and same-incubation controls, which live downstream of this module.
- A stopped-TMB OD at 450 nm is stopped-chromogen amount under that development time (S7) — NOT analyte concentration without a same-plate standard series fitted downstream.
- A fluorescence frame is photons through (cube/filter, illumination, exposure, objective) at (position, time) — NOT a live/dead call and NOT expression level. L3224 assigns live = intracellular-esterase calcein (green) and dead = membrane-compromised EthD-1 DNA binding (red) (S8), but the call requires controls, thresholds, and per-cell-type titration defined outside this module. IF localization requires secondary-only and unstained controls (S9) before any "positive" statement.
- Cross-assay comparisons (e.g., CCK-8 OD vs LIVE/DEAD images vs IF) are NOT commensurable inside these modules: different measurands, different controls, different time windows.

## 9. Time scope and instrument capability summary

- In-module clock: seconds per plate (absorbance) and per-frame acquisition (imaging) only. All incubation/development/staining clocks (CCK-8 1–4 h; TMB 15–30 min + 60 min stability; LIVE/DEAD 30 min/20–25 °C; IF tens of minutes to hours incl. possible overnight primary) are upstream steps and MUST appear as separate plan nodes with their own windows.
- Capability gates: `ABS_READ` needs an absorbance reader with the assay λ in 200–1,000 nm (1 nm steps, < 2.5 nm bandwidth) and a compliant plate/cuvette (S1/S2); Multiskan SkyHigh MUST NOT be planned for fluorescence or kinetic reads on current sources. `FLUO_IMAGE` needs a per-build fluorescence check (module, cubes, illumination, detector, focus, stage, incubation) — the DMi8 envelope (S3–S5) is a platform maximum, never a single machine's capability.

## 10. UNKNOWNs (do not plan on these)

| Item | Status | Reason |
|---|---|---|
| Kinetic/continuous absorbance timing; shaking speed/amplitude/duration; incubation uniformity/accuracy; SkanIt version; service interval | UNKNOWN | Not in S1/S2 (see instrument profile §5) |
| DMi8 per-build: objective catalog, cube part numbers + bands, LED/laser λ + power, camera sensor specs, stage travel/speed/repeatability, incubation ranges, TIRF/GSD/spinning-disk numbers, condenser WD / cover-glass rules, LAS X procedures | UNKNOWN | Not on S3–S5 pages read (see instrument profile §5) |
| CCK-8 exact cell-density linearity per cell type; per-lot background; SDS/HCl-stop compatibility with downstream steps | UNKNOWN | Cell-type dependent (S6); lot sheets not read |
| ELISA antibody pairs, coating/blocking/wash details, standard concentrations, per-kit stop molarity (2 M H₂SO₄ vs 0.16 M N600 vs SS03 ready-to-use differ by product) | UNKNOWN | Kit-lot dependent; S7 covers substrate/stop/readout only |
| L3224 exact µM working concentrations; per-cell-type titration endpoint; spectral unmixing thresholds | UNKNOWN | Optimization per cell type required (S8); lot manual is authoritative |
| IF antibody dilutions, incubation times per target, Triton %/time per cell type (0.5% vs 1% variants in S9), mount curing time | UNKNOWN | Antigen/cell-type dependent (S9); antibody data sheets not read |

## 11. Verified vs NOT verified

- Verified (transcribed 2026-11-05): instrument envelopes in the two `k2-instruments` profiles from S1–S5; assay readout cores (λ, volumes, times, filter guidance, blank/control requirements, stability windows, terminality-relevant notes) from S6–S9 at web-summary level.
- NOT verified (do not treat as checked): operating/user manuals (Multiskan N21873, DMi8 build manual, LAS X, SkanIt), per-machine build sheets, per-lot dye/antibody/TMB data sheets, laboratory SOP adaptations, any concentration/viability/count derivation. No `SourceChecked` claim is made; this file stays `research_draft`.
- Anti-fabrication note: every numeric value above carries its source row; any parameter without one is labeled UNKNOWN with the reason. Second-person review against the listed URLs/locators is still required before promotion.
