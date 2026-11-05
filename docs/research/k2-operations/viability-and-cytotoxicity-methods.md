# Viability and Cytotoxicity Methods — CCK-8, LDH Release, LIVE/DEAD (Microscopy)

`review_status: research_draft`

> Do NOT treat as `SourceChecked`. S1–S3 (manufacturer manuals/protocol pages),
> S6 (Bio-protocol preprint), S7 (PMC author manuscript of a Current Protocols
> unit), S8 (mirror copy of a Cold Spring Harbor Protocol), and S9 (mirror copy
> of a Current Protocols in Cytometry unit) were read directly on 2026-11-05
> at the sections cited. The original CSH Protocols URL returned HTTP 403, so S8
> is an identically-paginated mirror copy — verify against the publisher
> (doi:10.1101/pdb.prot095497) before promotion. All other values are
> search-summary reports or explicitly UNKNOWN. No numeric value in this file is
> invented. No second-person review done. No `SourceChecked` / `ExpertReviewed` /
> `LabValidated` claim is made. Existing repo operations are used ONLY for
> the Reuse Map (§8); no repo file is used as evidence for any step/value.

## 1. Identity

| Field | Value |
|---|---|
| Document | Method templates for three non-interchangeable viability/cytotoxicity readouts: CCK-8 (metabolic), LDH release (membrane integrity), LIVE/DEAD microscopy (esterase + membrane integrity, imaged) |
| Role | K2 method-template research (planning + execution bindings and controls); not instrument profiles, not runtime JSON |
| Records | `MT-CCK8-DEHYDROGENASE` · `MT-LDH-RELEASE` · `MT-LIVEDEAD-MICROSCOPY` |
| Scope | Wet-lab biological experiments ONLY. Assay-specific reagents, volumes, incubation bindings, controls, and readout boundaries only. Generic transfer / incubation / wash / absorbance / imaging mechanics are NOT redefined here. For imaging, only wet-lab acquisition is recorded; no analysis algorithm is defined |
| Out of scope | Clinical/patient studies; epidemiology; bioinformatics; image segmentation/analysis; statistics or data-processing pipelines; MTT/XTT/MTS/WST-1 variants; 51Cr release; flow-cytometry LIVE/DEAD gating; ATP-luminescence; colony-based survival; instrument operating manuals; per-lot dye/antibody sheets |

### 1.1 Granularity decision

| Record ID | Type | Why it is not an atomic operation |
|---|---|---|
| `MT-CCK8-DEHYDROGENASE` | Method Template (`ProtocolVersion` + block combination) | Binds generic transfer + incubation + absorbance-readout with CCK-8-specific reagent, time, wavelength, and control bindings; adds a downstream derivation (blank subtraction, viability %) that is a ToolAction, not an operation |
| `MT-LDH-RELEASE` | Method Template | Binds generic transfer/aliquot + incubation + absorbance-readout with LDH-specific lysis, supernatant-transfer, enzymatic-development, and control bindings; % cytotoxicity is a ToolAction |
| `MT-LIVEDEAD-MICROSCOPY` | Method Template | Binds generic transfer + incubation + fluorescence-image-acquisition with L3224-specific dye, filter, and no-fixation bindings; live/dead calling and counting are downstream analysis, not imaging |

These IDs are provisional research IDs. They MUST NOT be converted directly to `OperationModule` JSON. Atomic sub-operations belong to the existing catalog records referenced in §8.

Core rule (read first): **the three readouts are not interchangeable.** CCK-8 reports
dehydrogenase-dependent formazan (metabolic activity under controlled conditions);
LDH reports membrane-compromised enzyme leakage normalized to a maximum-lysis
control; LIVE/DEAD microscopy reports per-cell esterase activity (green) and
membrane permeability (red) as images. No readout proves "viability %" alone.

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator / what was used | Read status |
|---|---|---|---|---|
| S1 | Dojindo CK04 manual, Cell Counting Kit-8 (Rev. Apr 14, 2026) | https://www.dojindo.com/manual/CK04/ | General Protocol (Cell Number Determination; Cell Proliferation and Cytotoxicity Assay); Precautions 1–2, 5–6; FAQ 1, 3, 5 | Read directly 2026-11-05 |
| S2 | Promega TB163, CytoTox 96 Non-Radioactive Cytotoxicity Assay (G1780, Rev. 8/25) | https://www.promega.com/-/media/files/resources/protocols/technical-bulletins/0/cytotox-96-nonradioactive-cytotoxicity-assay-protocol.pdf | §4.A controls; §4.B compound-cytotoxicity protocol steps 1–8; §4.C calculation; §5.B–5.C cell-mediated controls/formula (reference only); §6 total-cell-number variant; §7.A–7.B background/controls; Fig.1 principle | Read directly 2026-11-05 |
| S3 | Thermo Fisher, LIVE/DEAD Viability/Cytotoxicity Kit for Mammalian Cells Protocol (L3224, microscopy) | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/live-dead-viability-cytotoxicity-kit-for-mammalian-cells.html | Protocol steps 3, 5, 6; Spectral information and storage table; Protocol tips; "should not be used for flow cytometry" scope note | Read directly 2026-11-05 |
| S4 | Thermo MP03224 manual + mammalian-cell protocol summaries (calcein AM / EthD-1 titration, serum-esterase wash, dead-cell controls) | https://tools.thermofisher.com/content/sfs/manuals/mp03224.pdf | Dye-titration procedure; wash-to-reduce-serum-esterase note; dead-control suggestions (heat/ethanol); use-fresh-≤1-day note | Search summary only — NOT personally verified |
| S5 | Dojindo CK12 (LDH-WST) and Roche LDH kit pages (wavelength cross-check) | https://www.dojindo.com/manual/CK12/ ; https://elabdoc-prod.roche.com/LifeScience/Document/350f7444-c3ef-e311-98a1-00215a9b0ba8 | WST-LDH 490 nm readout; Roche LDH 490–492 nm + >600 nm reference convention | Search summary only — NOT personally verified, used only to note kit-family variation |
| S6 | Bio-protocol Preprint: Che, Wang & Ren, "CCK-8 cell viability assay" (Dec 23, 2021), abbrev. of eLife 2020 (doi:10.7554/eLife.52570) | https://bio-protocol.org/exchange/preprintdetail?id=1481&type=3 | Protocol steps 1–6 (5000 NP cells/well in 200 µL; 90 µL fresh medium + 10 µL CCK-8; blank = 90 µL + 10 µL no cells; 3 h 37 °C dark; read 450 nm after shaker mix; As−Ab) | Read directly 2026-11-05 |
| S7 | Current Protocols in Toxicology unit (author manuscript): Kaja, Payne, Naumchuk & Koulen, "Quantification of lactate dehydrogenase for cell viability testing using cell lines and primary cultured astrocytes", Curr Protoc Toxicol. 2017;72:2.26.1–2.26.10 (doi:10.1002/cptx.21) | https://pmc.ncbi.nlm.nih.gov/articles/PMC5501254/ | Abstract + Strategic Planning; Basic Protocol 1 steps 1–7 + annotations (50 µL transfer; 50 µL reagent; 60 min dark RT; 50 µL 1 M acetic acid stop; OD490); Alternate Protocol 1 total-LDH lysis (10 µL 10X per 100 µL); Support Protocol 1 seeding (25,000 neurons/well, 100 µL) | Read directly 2026-11-05 |
| S8 | Cold Spring Harbor Protocols: Kumar, Nagarajan & Uchil, "Analysis of Cell Viability by the Lactate Dehydrogenase Assay", Cold Spring Harb Protoc. 2018; doi:10.1101/pdb.prot095497 (mirror copy; original URL http://cshprotocols.cshlp.org/content/2018/6/pdb.prot095497 returned HTTP 403) | https://yenepoya.res.in/database/LTN_Datta_Lab/OGD-Prot_Systematic-Review/LDH.pdf | MATERIALS + Box 1 (Phenol Red/serum background; Spontaneous/Medium/Maximum controls); Optimization steps 1–8 (0–20,000 cells/100 µL; 15 µL lysis; 250×g 4 min; 50 µL + 50 µL substrate; 15–30 min 37 °C; 100 µL DMF/SDS stop; 490 nm ref 690 nm); Cytotoxicity steps 9–14 + Recipes + Discussion | Read directly 2026-11-05 (mirror pagination pp.1–4) |
| S9 | Current Protocols in Cytometry Unit 9.2: Johnson, Nguyen & Coder, "Assessment of Cell Viability", Curr Protoc Cytom. 2013;64:9.2.1–9.2.26 (doi:10.1002/0471142956.cy0902s64) — Alternate Protocol 8, microscopy arm (obtained via hosted PDF mirror) | https://microscopist.co.uk/files/wp-content/uploads/2017/04/johnson-et-al_cp-in-cytometry-2013.pdf | Alternate Protocol 8 steps 1–6 (2 µM calcein AM + 1 µM EthD-1 + 10 µM Hoechst 33342 in prewarmed medium; 1 mL/chamber; dead control 100 µM digitonin or 50% Triton; wash/destain/re-incubate; FITC/Texas Red/DAPI imaging); Fig.9.2.8 non-overlap caveat; "Methods for microscopy" timing (image calcein/EthD-1 within 30 min; replace dye with fresh medium for delayed reads) | Read directly 2026-11-05 (mirror pagination) |

## 3. Record A — `MT-CCK8-DEHYDROGENASE`

### 3.1 Purpose

Quantify dehydrogenase-dependent WST-8 → water-soluble formazan signal as a
proxy for living-cell number under controlled conditions (S1: "amount of formazan
dye generated by dehydrogenases in cells is directly proportional to the number
of living cells"). S1 Precaution 1 bounds the claim: conditions/chemicals that
affect dehydrogenase activity cause discrepancy between actual and CCK-8-read
viable-cell number.

### 3.2 Assay-specific bindings (reference, do not redefine, generic ops in §8)

| Binding | Value | Source |
|---|---|---|
| Culture + dosing | 96-well: 100 µL cell suspension/well; proliferation example 5,000 cells/well, 24 h pre-incubation, then test substance, then 6/12/24/48 h exposure | S1 General Protocol, cytotoxicity steps 1–3, read 2026-11-05 |
| Reagent addition | 10 µL CCK-8 solution per well (96-well); other formats: 10% of total volume | S1 General Protocol steps 2/4; FAQ 1, read 2026-11-05 |
| Development hold | 1–4 h in incubator (e.g. 37 °C, 5% CO₂); leukocytes may need up to 4 h or ~10⁵ cells/well | S1 General Protocol steps 3/5; Precaution 5, read 2026-11-05 |
| Readout binding | Absorbance at 450 nm (best sensitivity); fallback 430–490 nm if no 450 nm filter; reference subtraction at ≥600 nm (Fig. 5 uses 650 nm) when turbidity high | S1 General Protocol step 4/6; Precaution 6; FAQ 5, read 2026-11-05 |
| Delayed-read option | Add 10 µL 1% w/v SDS or 0.1 M HCl per well, cover, protect from light, RT; stable 24 h | S1 General Protocol step 4/6 note, read 2026-11-05 |
| Seeding window (96-well) | Adherent ≥1,000 cells/well; leukocytes ≥2,500 cells/well; recommended max 25,000 cells/well (per 100 µL medium) | S1 FAQ 1, read 2026-11-05 |
| Compatibles | Phenol-red medium usable (blank-subtracted); CCK-8 low toxicity — same cells may proceed to CV/neutral-red/DNA assays | S1 FAQ 3–4, read 2026-11-05 |
| Repository exemplar (Bio-protocol) | NP cells 5,000/well in 200 µL 96-well (center wells preferred — outer ring evaporates); after treatment, replace with 90 µL fresh medium + 10 µL CCK-8; blank = 90 µL + 10 µL, no cells; 3 h 37 °C dark; gentle shaker mix before 450 nm read; proliferation = As−Ab | S6 steps 1–6, read 2026-11-05 |

### 3.3 Ports and sample transitions

`cells_cultured → [dose] → cells_dosed → [CCK-8 addition + development] → plate_developed(formazan) → [ABS_READ] → plate_preserved + raw_OD_matrix`. Optically non-consumptive read, but wells are post-reagent (formazan ± SDS/HCl-stabilized); never return to pre-reagent state or to an unstained-cell state. Time scope: dosing clock belongs to the exposure SOP; development is a `per_assay` hold; read is instrument-seconds.

### 3.4 Preconditions / completion checks

Preconditions: (P1) cells seeded in the verified window with pre-incubation done;
(P2) 450 nm (or recorded 430–490 nm fallback) reader capability; (P3) same-plate
cell-free medium + CCK-8 blank planned; (P4) no bubbles (S1). Completion: (C1)
every addressed well received CCK-8 at the 10 µL (or 10%) binding; (C2) developed
for the recorded 1–4 h; (C3) every well returned OD + λ (+ reference λ where
used) with blank-well IDs. Background subtraction and viability-% derivation are
downstream ToolActions (§7), not completion of the read.

## 4. Record B — `MT-LDH-RELEASE`

### 4.1 Purpose

Quantify membrane-integrity loss via LDH released into medium, coupled through
lactate + NAD⁺ + INT/diaphorase to red formazan proportional to lysed-cell
number (S2 Fig.1 + §1). S2 bounds the window: released-LDH half-life ≈ 9 h in
medium, so exposure timing must be optimized — long delays underestimate killing.

### 4.2 Assay-specific bindings (compound-cytotoxicity path, S2 §4)

| Binding | Value | Source |
|---|---|---|
| Culture + dosing | Cells in 96-well; test/vehicle to final 100–150 µL/well (384-well: 25–30 µL) | S2 §4.B steps 1–2, read 2026-11-05 |
| Exposure hold | 37 °C for the SOP's exposure period (LDH half-life ≈ 9 h caveat) | S2 §4.B step 3, read 2026-11-05 |
| Maximum-lysis binding | 10 µL 10X Lysis Solution per 100 µL original volume, 45 min before reagent (≈0.8% Triton X-100 in the cell-mediated setup) | S2 §4.B step 4 + §5.B setup, read 2026-11-05 |
| Supernatant harvest | Transfer 50 µL aliquots (384-well: 12.5 µL) from every test/control well to a fresh flat clear-bottom enzymatic plate | S2 §4.B step 5, read 2026-11-05 |
| Enzymatic development | Equal-volume CytoTox 96 Reagent (50 µL / 12.5 µL), foil/cover, 30 min at room temperature protected from light | S2 §4.B step 6, read 2026-11-05 |
| Stop + read | Equal-volume Stop Solution; pop large bubbles with needle; read 490 or 492 nm within 1 h | S2 §4.B steps 7–8, read 2026-11-05 |
| Repository path (Current Protocols Toxicol., astrocyte/line exemplar) | 50 µL sample → Assay Plate + 50 µL Assay Reagent (Buffer A+B, MPMS), 300–500 rpm 15 s mix; 60 min dark RT; stop with 50 µL 1 M acetic acid, 15 s mix; OD490; total-LDH wells: 10 µL 10X lysis per 100 µL supernatant, shake ≥3 min, microscopy-confirmed lysis; neuroprotection seeding exemplar 25,000 E18 neurons/well in 100 µL coated 96-well | S7 Basic/Alternate/Support Protocols, read 2026-11-05 |
| Repository path (CSH Protocols, formulation-explicit) | Optimization: 0–20,000 cells/100 µL + 15 µL lysis, 250×g 4 min; 50 µL supernatant + 50 µL substrate, 15–30 min 37 °C dark; stop 100 µL DMF/SDS (pH 4.7) — or 1 N HCl, DMF/SDS preferred with Phenol Red; read 490 nm, ref 690 nm, within 1 h; Box 1 controls (Spontaneous/Medium/Maximum); serum ≤5% to cut background; substrate recipe (0.054 M lactate / 1.3 mM NAD⁺ / 0.66 mM INT / 0.28 mM MPMS in 0.2 M Tris-HCl pH 8.2) | S8 Optimization steps 1–8 + Box 1 + Recipes, read 2026-11-05 (mirror pp.1–4; verify against doi:10.1101/pdb.prot095497) |
| Total-cell-number variant (S2 kit path) | Lyse ALL wells (15 µL 10X per 100 µL, 45–60 min at 37 °C), then harvest + develop as above; signal ∝ cell number | S2 §6, read 2026-11-05 |
### 4.3 Ports and sample transitions

`cells_cultured → [dose/exposure] → cells_exposed (+ supernatant_LDH) → [aliquot] → enzymatic_plate(supernatant child samples) → [develop + stop] → plate_stopped + raw_OD_matrix`. The culture plate is consumed as a scored timepoint (supernatant removed); the enzymatic plate is terminal (stopped chemistry). Time scope: exposure is `per_assay`; lysis 45 min and development 30 min are separate hold nodes; stop→read ≤ 1 h is a hard clock (S2).

### 4.4 Preconditions / completion checks

Preconditions: (P1) all §5 control wells plated per assay path (compound §4.A vs
cell-mediated §5.B — do not mix); (P2) lysis reagent available if % cytotoxicity
is claimed; (P3) fresh flat clear-bottom enzymatic plate ready; (P4) reader
covers 490/492 nm. Completion: (C1) every test/control well contributed its
aliquot to the mapped enzymatic well; (C2) reagent + stop added at equal volume
with light protection and timestamps; (C3) every enzymatic well returned OD +
λ within 1 h of stop. Derivation (§7) is downstream.

## 5. Record C — `MT-LIVEDEAD-MICROSCOPY`

### 5.1 Purpose

Distinguish live vs dead mammalian cells in place by two probes imaged in two
channels: intracellular-esterase calcein (green, live) + membrane-compromised
EthD-1 DNA binding (red, dead) (S3 principle + spectral table). Microscopy path
only — S3 scope note: "should not be used for flow cytometry."

### 5.2 Assay-specific bindings

| Binding | Value | Source |
|---|---|---|
| Staining solution | 5 µL calcein AM (Comp. A) + 20 µL EthD-1 (Comp. B) into 10 mL DPBS | S3 Protocol step 3, read 2026-11-05 |
| Application | Remove medium; add 100–200 µL staining solution directly to cells | S3 Protocol steps 4–5, read 2026-11-05 |
| Stain hold | 30 min at 20–25 °C | S3 Protocol step 6, read 2026-11-05 |
| Channels | Calcein ~494/517 nm, FITC/GFP (EVOS GFP); EthD-1 ~528/617 nm, RFP (EVOS RFP) | S3 spectral table, read 2026-11-05 |
| Handling | Thaw vials before prep; use stock solutions within 1 day; titrate per cell type, highest dye with minimal background; protect from light (generic fluorophore handling) | S3 steps 2 + tips, read 2026-11-05 |
| Repository path (Current Protocols Cytometry Alt. Protocol 8) | 2 µM calcein AM + 1 µM EthD-1 + 10 µM Hoechst 33342 in prewarmed medium, 1 mL per chambered-coverglass well; dead control 100 µM digitonin (or 50% Triton stock); wash → destain → re-incubate in dark; image FITC (calcein) / Texas Red (EthD-1) / DAPI (Hoechst); Hoechst required for total count because calcein/EthD-1 are slightly non-overlapping; image calcein/EthD-1 within 30 min, or replace dye with fresh medium for delayed reads | S9 Alt. Protocol 8 steps 1–6 + Fig.9.2.8 + "Methods for microscopy", read 2026-11-05 |
| Pre-wash + dead control | Wash to reduce serum-esterase background; dead-control by heat/ethanol; ~2 µM / ~4 µM working-concentration exemplars | S4, search summary only — NOT verified |

### 5.3 Ports and sample transitions

`cells_live → [medium removal + stain] → cells_stained_live → [FLUO_IMAGE] → cells_imaged_live + raw_images`. Preserved through imaging (phototoxicity/bleaching caveat); fixation after staining is a forbidden transition (S3). No transition back to unstained culture. Time scope: stain 30 min is a `per_assay` hold; acquisition is per-frame.

### 5.4 Preconditions / completion checks

Preconditions: (P1) cells in a microscopy-compatible vessel; (P2) FITC/GFP +
RFP cubes/illumination confirmed on this build; (P3) fresh (≤1 day) working
solution at the recorded titration; (P4) unfixed/unpermeabilized state. Completion:
(C1) every requested (position, channel) pair has an image with channel/cube/
exposure/objective recorded; (C2) no saturated-to-clip or fully dark frame
accepted without operator note; (C3) single-stain + unstained controls imaged
where a live/dead call will be made (§6). Counting/calling is downstream (§7).

## 6. Controls (required companions; readout alone proves nothing)

| Readout | Required controls (same run) | Why (sourced) |
|---|---|---|
| CCK-8 OD | Cell-free medium + CCK-8 blank (same plate); vehicle control; per-cell-type cell-number standard curve for any count claim; reducing-agent background check | S1 FAQ 3 (phenol-red/medium background subtractable); S1 Precautions 1–2 (dehydrogenase dependence; reducing-agent formazan); S6-lineage curve requirement carried by the generic readout record |
| LDH, compound path | No-Cell (medium background); Vehicle-Only cells; Maximum LDH Release (lysis 45 min before reagent) whenever % cytotoxicity is computed | S2 §4.A + §4.C, read 2026-11-05 |
| LDH, cell-mediated path | Above + Effector Spontaneous + Target Spontaneous + Target Maximum + Volume Correction (lysis-in-medium) + Culture Medium Background; LDH Positive Control optional (bovine-heart LDH function check) | S2 §5.B–5.C, read 2026-11-05 |
| LDH practice | Triplicate/quadruplicate per condition; target-cell-number optimization pre-experiment for signal-to-noise (S2: e.g. 0–20,000 cells/100 µL; S8: ≥2× medium background) | S2 §5.A–5.B + S8 Optimization steps 1–7, read 2026-11-05 |
| LDH, repository control sets (S7/S8 paths) | Medium-only + Spontaneous-release + Maximum-release (S8 Box 1 triad); S7 path: 4 technical replicates/condition, control-condition normalization without complex math, LDHr/LDHt ratio for % cytotoxicity | S8 Box 1 + §4.C-equivalent + Discussion, read 2026-11-05; S7 Step 7 + annotations, read 2026-11-05 |
| LIVE/DEAD microscopy | Unstained control; single-stain (calcein-only, EthD-1-only) controls; per-cell-type dye titration from low end; Hoechst/total-nuclei counterstain required for % denominators (S9); dead-cell reference — 100 µM digitonin or 50% Triton stock (S9) preferred over S4 heat/ethanol exemplars | S3 tips (titration, minimal background); S9 Alt. Protocol 8 + Fig.9.2.8, read 2026-11-05; S4 search-summary-only for serum-esterase pre-wash |

## 7. Readout meaning — what the numbers/images are and are NOT

- **CCK-8 OD at 450 nm** is formazan absorbance under the recorded development
  time — NOT viability, NOT cell number, NOT concentration. Proportionality to
  living-cell number requires same-cell-type, same-incubation controls (S1
  principle + Precaution 1); reducing agents and turbidity break the mapping
  (S1 Precautions 2, 6). Derivation (`blank subtraction → viability % vs
  vehicle/standard curve`) is a deterministic **ToolAction**, not an operation.
- **LDH OD at 490/492 nm** is developed-formazan amount from supernatant LDH —
  NOT % dead without the control arithmetic. Compound path (S2 §4.C): subtract
  medium background, then `100 × Experimental / Maximum`. Cell-mediated path
  (S2 §5.C): subtract medium background everywhere, subtract Volume Correction
  from Target Maximum, then `100 × (Experimental − Effector Spont − Target
  Spont) / (Target Max − Target Spont)`. Repository paths: S7 normalizes to the
  control condition (no complex math), or `% = OD490(LDHr)/OD490(LDHt) × 100`
  when the total-LDH arm is run; S8 compound-style equation is `Experimental /
  Maximum` (steps 13–14, 490 nm with 690 nm reference subtracted). Cross-kit
  commensurability (INT vs WST, stopped vs kinetic) is NOT established here
  (S5 search-summary-only). No image segmentation, statistics, or pipeline is
  defined — wet-lab acquisition only.
- **LIVE/DEAD frames** are photons through (cube/filter, illumination,
  exposure, objective) at (position, time) — NOT a live/dead call and NOT a
  percentage. The call requires thresholds, single-stain references, per-cell-type
  titration, and — per S9 — a Hoechst/total-nuclei denominator, because
  calcein/EthD-1 are slightly non-overlapping (Fig.9.2.8: low-dose permeabilization
  yields cells staining for neither; define viability cutoffs a priori).
  Fixation after staining voids the readout (S3). No counting/scoring algorithm
  is defined here; derivation is a downstream step outside this wet-lab record.
- Cross-method comparison (CCK-8 OD vs LDH % vs LIVE/DEAD images) is NOT
  commensurable inside these templates: different measurands, different
  controls, different time windows (metabolic 1–4 h vs leakage half-life ~9 h
  vs 30 min stain snapshot).

## 8. Reuse Map (existing operations first; no new atomic operation proposed)

Inventory consulted: `catalog/operations/` (liquid-transfer, incubation,
washing, sample-aliquot, absorbance-measurement .v1 JSONs, all
`research_draft`) + `docs/research/k2-operations/` (liquid-transfer,
inoculation-and-incubation, washing-and-sampling,
absorbance-and-fluorescence-readout incl. `FLUO_IMAGE`, plus README/granularity
records). Verdicts: `direct` = use as-is; `parameterized` = use with
method-specific binding; `variant` = existing variant covers it; `missing`
= no existing op + ToolAction/Custom Step marking (no code changes made).

| Experiment step | Existing operation / template | Verdict | Binding / note |
|---|---|---|---|
| CCK-8: seed cells, add test substance | `op.liquid_transfer` (catalog + liquid-transfer.md) | parameterized | Volume = 100 µL/well + 10 µL dosing (S1); tip/mixing per SOP |
| CCK-8: add CCK-8 reagent | `op.liquid_transfer` | parameterized | 10 µL/well (96-well) or 10% total volume (S1); no-bubble constraint |
| CCK-8: 1–4 h development hold | `op.incubation` (catalog + inoculation-and-incubation.md) | parameterized | 37 °C 5% CO₂ 1–4 h (S1); cell-type-dependent, no default |
| CCK-8: 450 nm read | `op.absorbance_measurement` / `ABS_READ` | parameterized | λ = 450 nm (fallback 430–490), reference ≥600 nm (S1) |
| LDH: dose + exposure hold | `op.liquid_transfer` + `op.incubation` | parameterized | Final 100–150 µL/well; 37 °C exposure, LDH half-life ~9 h caveat (S2) |
| LDH: maximum-lysis addition | `op.liquid_transfer` + `op.incubation` | parameterized | 10 µL 10X per 100 µL, 45 min before reagent (S2); NOT a new "lyse" atom |
| LDH: supernatant to enzymatic plate | `op.sample_aliquot` (catalog + washing-and-sampling.md) | parameterized | 50 µL (96-well) / 12.5 µL (384-well) child samples to fresh flat clear-bottom plate (S2); lineage parent → enzymatic wells |
| LDH: 30 min RT development (+ stop) | `op.incubation` (timed hold) + `op.liquid_transfer` (reagent/stop addition) | parameterized | Equal-volume reagent, dark 30 min RT; equal-volume stop; read ≤1 h (S2) |
| LDH: 490/492 nm read | `op.absorbance_measurement` / `ABS_READ` | parameterized | λ = 490 or 492 nm (S2); record actual λ |
| LIVE/DEAD: remove medium, add stain | `op.liquid_transfer` (×2: to waste, then reagent in) | parameterized | 100–200 µL staining solution (S3); serum-esterase pre-wash per S4 is `op.washing` parameterized — S4 unverified |
| LIVE/DEAD: 30 min stain hold | `op.incubation` | parameterized | 20–25 °C, 30 min (S3) |
| LIVE/DEAD: two-channel imaging | `FLUO_IMAGE` (absorbance-and-fluorescence-readout.md) | parameterized | Calcein FITC/GFP + EthD-1 RFP bindings (S3); per-frame exposure/objective recorded |
| CCK-8 viability %; LDH % cytotoxicity; blank subtractions | — (deterministic calculation/preview) | missing → **ToolAction** | NOT operations; no new atoms proposed |
| Any new stain/material/process with unstable boundary | — | missing → **Custom Step** | None needed for these three kits; reserved per granularity-freeze |

**No new atomic operation candidate.** Every physical step above is expressed by
an existing operation + method binding. Every missing item is a ToolAction
(calculation) or a future Custom Step, never a new atom.

## 9. UNKNOWNs (do not use for planning; classified per record-standard §4)

| Item | Status | Reason |
|---|---|---|
| CCK-8 exact cell-density linearity per cell type; per-lot background; SDS/HCl-stop compatibility with downstream assays | UNKNOWN | `method_dependent` (S1: cell-type/timing dependent; lot sheets not read) |
| CCK-8 non-96-well geometry (volumes beyond the 10% rule; edge effects) | UNKNOWN | `source_missing` (S1 FAQ 1 gives only the 10% rule; no plate-specific validation read) |
| LDH serum-LDH background per serum lot; phenol-red contribution magnitude | UNKNOWN | `method_dependent` + `configuration_dependent` (S2 §7 notes dependence; no lot values read) |
| LDH exposure-time optimum per cell line/compound; late-read underestimation curve | UNKNOWN | `method_dependent` (S2: must be optimized; half-life ≈ 9 h only) |
| LDH 384-well evaporation/edge behavior; automated-washer handling | UNKNOWN | `source_missing` (S2 gives 384 volumes only; no washer SOP read) |
| L3224 exact µM working concentrations; per-cell-type titration endpoint; unmixing thresholds | UNKNOWN | `method_dependent` (S3: titrate per cell type; S4 unverified; S9 gives 2 µM / 1 µM / 10 µM Hoechst as its protocol's fixed point, not a universal optimum) |
| L3224 serum-esterase wash buffer/cycles; dead-control kill parameters | UNKNOWN | `source_missing` (S4 search-summary-only; S3 page does not state them; S9 dead control = 100 µM digitonin or 50% Triton stock — verified as S9's choice, still cell-type-dependent) |
| CSH publisher-version pagination/wording beyond the mirror copy | UNKNOWN | `source_missing` (original URL HTTP 403 on 2026-11-05; mirror pp.1–4 read; verify against doi:10.1101/pdb.prot095497 before promotion) |
| Exposure/focus/stage/incubation-per-build imaging values | UNKNOWN | `configuration_dependent` / `device_state` (per-build sheets, LAS X, SkanIt not read) |
| Cross-kit LDH scale equivalence (Promega INT vs Dojindo WST vs Roche) | UNKNOWN | `source_missing` (S5 search-summary-only; no head-to-head protocol read) |

## 10. Verified vs NOT verified

- Verified (read 2026-11-05 at the locators in §2): CCK-8 bindings (§3.2) from S1 + Bio-protocol exemplar S6 steps 1–6; LDH bindings (§4.2) from S2 §4–§7 + Current Protocols Toxicol. S7 (Basic/Alternate/Support Protocols) + CSH Protocols S8 (Optimization steps 1–8, Box 1, Recipes, Discussion); LIVE/DEAD bindings (§5.2) from S3 + Current Protocols Cytometry Alt. Protocol 8 S9 (steps 1–6, Fig.9.2.8, microscopy timing).
- NOT verified (do not treat as checked): S4 titration/wash/dead-control details; S5 cross-kit comparability; CSH publisher-version wording beyond the S8 mirror copy; any operating manual (SkanIt, LAS X, build sheets), per-lot dye/LDH sheets, laboratory SOP adaptations, or any derivation (curve fit, % formulas applied to real data). Second-person review against the §2 URLs/locators is still required before promotion. This file stays `research_draft`.
