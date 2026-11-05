# Wound Healing and Transwell Migration — Method Records (wet-lab only)

`review_status: research_draft`

> Do NOT treat as `SourceChecked`. P1–P4 below were read directly (full-page
> fetch) on 2026-11-05; B1–B3 were NOT readable and are marked explicitly.
> No numeric value in this file is invented, but only §2-listed locators count
> as personally verified. No second-person review done. No `SourceChecked` /
> `ExpertReviewed` / `LabValidated` claim is made.
>
> Wet-lab scope only. This file stops at raw image acquisition. Downstream
> image quantification (area segmentation, edge finding, cell counting),
> statistics, and data-processing pipelines are NOT recorded here — no
> analysis algorithm is defined. Invasion through ECM is a variant note only
> (§6.6), not a recorded procedure.

## 1. Identity

| Field | Value |
|---|---|
| Document | Method records for 2D gap-closure (scratch, culture-insert) and Transwell migration wet-lab work |
| Role | K2 method templates (assay-specific steps, bindings, controls, limits); generic operations are referenced, not re-taught |
| Records | `MT-SCRATCH-WOUND` (mechanical scratch gap closure) · `MT-INSERT-GAP` (culture-insert defined gap) · `MT-TRANSWELL-MIGRATION` (two-chamber pore migration; adherent + non-adherent paths) |
| Scope | Wet-lab only: adherent seeding → gap creation / gradient assembly → incubation → rinse / apical clear → fix/stain → raw image acquisition |
| Out of scope | ECM/Matrigel invasion procedure; 3D / spheroid / organoid migration; microfluidic chemotaxis; single-cell tracking metrics; any image-analysis algorithm, statistics, or pipeline; any clinical/epidemiologic/bioinformatic content; any drug-efficacy claim |
| Granularity | **Not a single atomic operation.** These are method templates composed of atomic operations plus two **method-local steps** (see §1.1 and §3). |

### 1.1 Granularity decision

| Record ID | Type | Why it is not an atomic operation |
|---|---|---|
| `MT-SCRATCH-WOUND` | method template | seed + starve + mechanical wound + wash + proliferation block + incubation + raw imaging |
| `MT-INSERT-GAP` | method template | seed around barrier + attachment hold + barrier removal + raw imaging |
| `MT-TRANSWELL-MIGRATION` | method template | culture + upper seed + gradient assembly + timed incubation + apical clear + fix/stain + raw imaging |
| `CAND-GAP-CREATE` (scratch / insert-removal variants) | **method-local step**; not promoted to Catalog | mechanism differs between scratch and insert removal; no cross-method reuse evidence sufficient for a shared operation |
| `CAND-APICAL-CLEAR` (swab wipe of non-migrated cells) | **method-local step**; not promoted to Catalog | Transwell-specific selective removal; washing cannot express it, but it is not shared across method families |

These are method-internal steps. They remain `ProtocolStep::Text`-style
records in the method template and MUST NOT be converted to global
`OperationModule` JSON. Promotion requires >1 independent method family,
semantic stability and a second-person review.

Core rule (read first): **gap closure is migration + proliferation unless
proliferation is blocked and controlled.** A closure number without a
proliferation control (mitomycin C / serum-free design + control arms) is not
a migration measurement. Scratch gaps, insert gaps, and Transwell counts are
NOT commensurable — each record keeps its own measurand (§8).

## 2. Protocol sources (authoritative repositories and manufacturer manuals)

Four concrete protocols anchor this file. Each row gives title, authors,
DOI/URL, version/date, and the step-level locators used. Repo-internal
operation records were NOT used as protocol evidence.

| # | Protocol (title / authors / DOI-URL / version-date) | Step-level locators used in this file | Read status 2026-11-05 |
|---|---|---|---|
| P1 | Varankar & Bapat, "Migratory Metrics of Wound Healing: A Quantification Approach for *in vitro* Scratch Assays", Front Oncol 2018, doi:10.3389/fonc.2018.00633, https://www.frontiersin.org/journals/oncology/articles/10.3389/fonc.2018.00633/full (peer-reviewed PROTOCOLS article). Wet-lab steps only (§"Step-Wise Procedures" steps 1–5 + Materials/Equipment/Reagents/Equipment Setup). Analysis steps 6–21 (Fiji/MTrack2/BioVoxxel/MATLAB/Prism) are out of scope and NOT recorded. | Materials (cell lines A4/OVCA420/OVCAR3; MEM/RPMI; TE/TPVG; 24-well Corning Costar CLS3527; Neubauer chamber; Thermo 4141 incubator; Leica TCS SP5); Reagents Setup (PA1 medium; RPMI+10%FBS; 0.5 mg/mL mitomycin C stock; 1×PBS recipe); Equipment Setup (5%CO₂ 37 °C; scope equilibration ≥30 min); steps 1–5 (passage / seed / starve / wound / acquire) | Read directly, full-page fetch |
| P2 | ibidi, "Culture-Insert 2 Well — Instruction Manual", v3.4, 2026-10-14, https://ibidi.com/img/cms/downloads/in/IN_8XXXX_ci_2well.pdf (manufacturer validation manual; products 80206/81176/80242/80209) | Material Geometry table (specs p.1); Surface + Handling (p.2); Seeding Cells steps 1–5 (p.2); removal/fill/wash steps 5–8 (pp.2–3); TIP on partial detachment (p.3) | Read directly, full-PDF fetch |
| P3 | ibidi, "Application Note 21: Wound Healing Assay Using the ibidi Culture-Insert 2 Well in a µ-Dish 35 mm, high", v4.0, 2024-07-15, https://ibidi.com/img/cms/downloads/an/AN21_Wound_Healing_Assay.pdf (official application note; worked MCF-7 example) | §1 Material (MCF-7 HTB-22; 81176; RPMI R8758+10%FCS F0804; trypsin 59418C); §2.1 Cell Seeding steps 1–5 (p.2–3); §2.2 Gap Formation steps 1–4 (p.3); §2.3 Image Acquisition incl. worked 20 h/30 min series (p.4). §2.4 analysis (AN67 pointer) NOT recorded. | Read directly, full-PDF fetch |
| P4 | Justus, Marie, Sanderlin & Yang, "Transwell *In Vitro* Cell Migration and Invasion Assays", Methods Mol Biol 2023;2644:349–359, doi:10.1007/978-1-0716-3052-5_22, PMC10335869, https://pmc.ncbi.nlm.nih.gov/articles/PMC10335869/ (Springer Protocols / Methods in Molecular Biology chapter) | §2 Materials (§§2.1–2.5: cells, media, migration buffer, inserts, attractants, equipment); §3.1 Cell Culture steps 1–11; §3.2 Migration steps 1–11; §3.4 Crystal Violet steps 1–5; §3.5 DAPI steps 1–5; §3.7 non-adherent collection; §4 Notes 1–22. §3.3 invasion + §3.6 counting-tool detail NOT recorded (variant / analysis, out of scope). | Read directly, full-page fetch |

Blocked or unreadable — marked explicitly, never used as evidence, no steps invented from them:

| # | Source attempted | What happened | Fallback |
|---|---|---|---|
| B1 | Bio-protocol (authoritative repository) | Page fetch returned empty webkit shell (`noindex, nofollow`); protocol body not accessible without login/rendering — treated as blocked | P1 used as the readable peer-reviewed scratch protocol instead |
| B2 | Corning Transwell migration/invasion guide + AN-188 (manufacturer) | `https://www.corning.com/...ultimate-guide-from-setup-to-analysis.html` → HTTP 403 on direct fetch | P4 (Springer Protocols Transwell chapter) used as the readable primary protocol instead |
| B3 | Abcam wound-healing (scratch) protocol page | Direct fetch timed out; no page content retrieved | Not cited for any step or parameter; P1 covers the scratch path |

## 3. Reuse Map (inventory of existing operations first)

Inventory checked 2026-11-05: `catalog/operations/` holds `op.liquid_transfer`,
`op.washing`, `op.sample_aliquot`, `op.incubation`, `op.absorbance_measurement`;
`docs/research/k2-operations/` holds `OP-INOC` / `OP-INCUB`, `wash` /
`sample_aliquot`, `ABS_READ` / `FLUO_IMAGE`, R1–R4 cell-culture records,
`IF-FIXPERMBLOCK` / `IF-STAIN` / `IF-IMAGE-FIXED`, CV/CFU templates. Existing
records are referenced here for reuse ONLY — no step or value is copied from
them as evidence; every binding below cites P1–P4.

| Experiment step | Existing operation / template | Verdict |
|---|---|---|
| Adherent seeding to wells / inserts / upper chamber (density, volume, vessel) | `OP-INOC` | Parameterized reuse (bind density, volume, vessel = well/insert/membrane) |
| Hold for attachment / growth / migration window | `op.incubation` / `OP-INCUB` | Direct reuse (bind temp, atmosphere, duration, time anchor) |
| Reagent addition (medium, mitomycin C, attractant, stain, fixative, PBS) | `op.liquid_transfer` | Direct reuse (bind volume, reagent, destination) |
| Post-scratch debris rinse; Transwell membrane rinse | `op.washing` (`wash`) | Parameterized reuse (gentle PBS rinse; bind liquid, cycles, vigor) |
| Serum starvation / medium exchange | `op.washing` + `op.liquid_transfer` | Parameterized reuse (exchange to serum-free; no new op) |
| Fixation before staining (Transwell) | `IF-FIXPERMBLOCK` (fixation part) | Parameterized reuse (bind fixative, time; permeabilisation N/A unless stated) |
| Staining of migrated cells on membrane | biofilm `OP-CV-BIOMASS` stain sub-step | Parameterized reuse (bind dye, conc., time; solubilisation N/A — read is on-membrane) |
| Gap / membrane raw imaging (phase, brightfield, fluorescence) | `FLUO_IMAGE` (raw image acquisition) | Variant needed (modality parameter: phase/brightfield vs fluorescence; acquisition contract otherwise identical) |
| Pre-seed viability / count gate | cell-culture R2 `cell-counting-viability` | Direct reuse |
| Sampling / aliquoting for seeding suspension | `op.sample_aliquot` | Direct reuse where a parent-to-child split is recorded |
| Scratch wound creation (tip-driven monolayer disruption + debris) | none — liquid transfer / wash / inoculation cannot express mechanical cell removal with wound edges | **Method-local step** `CAND-GAP-CREATE` scratch variant (§4) |
| Insert removal gap reveal (barrier lift, no cell killing) | none — effect differs from scratch (no kill, no debris); cannot share one default | **Method-local step** `CAND-GAP-CREATE` insert-removal variant (§5) |
| Apical wipe of non-migrated cells (cotton swab) | none — washing exchanges liquid around a retained sample; wiping selectively removes one face's cells | **Method-local step** `CAND-APICAL-CLEAR` (§6) |
| Gap-closure metrics / migrated-cell counts from images | none in wet-lab scope — deterministic downstream derivation | **Out of scope here** (no analysis algorithm defined; record stops at raw images, §8) |

No JSON is emitted. Candidates stay provisional until a second person reviews them.

## 4. Record A — `MT-SCRATCH-WOUND` (protocol: P1)

### 4.1 Purpose

Create a linear cell-free wound in a confluent adherent monolayer and track
its recolonisation. The tool path kills/removes on-path cells and leaves
debris plus two wound edges; closure mixes migration and proliferation unless
the design blocks proliferation (P1 steps 3–4).

### 4.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Counted suspension (P1 steps 1–2); 24-well tissue-culture plate (P1: Corning Costar CLS3527); sterile 1× PBS (P1 PBS recipe); serum-free medium; mitomycin C working solution from 0.5 mg/mL stock (P1 Reagents Setup); 200 µL tip (P1 step 4) |
| Outputs | Same plate with wounded monolayer + debris (transient) → rinsed wound → incubated series + raw images at recorded timepoints |
| Ports | `suspension_in` → `monolayer_out` (`OP-INOC` reuse); `monolayer_in` → `wounded_out` (`CAND-GAP-CREATE` scratch variant); `waste_out` (debris/detached cells, sink) |
| Sample transition | `MONOLAYER_CONFLUENT → MONOLAYER_WOUNDED(+DEBRIS) → WOUND_RINSED → IMAGED_SERIES`. Wounding is destructive for on-path cells; rinsing discards debris by design (lossy step, never generic lossless wash). |
| Time scope | `per_assay` wound-to-readout block; time anchor `t₀` = wound creation (P1 step 4); starvation clock 24 h pre-wound (P1 step 3); acquisition 24 h post-wound (P1 step 5) |

### 4.3 Parameters (P1 bindings only; P1-silence = UNKNOWN, §11)

| Parameter | Value in P1 | `required_at` | Locator |
|---|---|---|---|
| `vessel` | 24-well tissue-culture plate, Corning Costar CLS3527 | plan | P1 Equipment (General Consumables) |
| `passage_reagents` | 0.5% TE (A4) / TPVG (OVCAR3, OVCA420); 500 µL; 37 °C, 2 min (TE) / 1 min (TPVG); quench with equal volume of the line medium; Neubauer count | run | P1 step 1 + CRITICAL note (trypsin criteria line-dependent, must be optimised) |
| `seed_target` | Confluent monolayer after 24 h at 37 °C; absolute number line/plate-size/doubling-time dependent, constant per line for reproducibility | plan | P1 step 2 + CRITICAL note |
| `serum_starve` | Wash with sterile 1× PBS → serum-free medium → 24 h at 37 °C acclimatisation | plan | P1 step 3 + CRITICAL notes (removes undefined growth factors; complete deprivation may affect viability — optimisation may be necessary) |
| `wound_tool` | 200 µL pipette tip, straight-line scrape | run | P1 step 4 |
| `debris_rinse` | Wash with 500 µL sterile 1× PBS, gentle, minimal wound-edge disturbance | run | P1 step 4 + CRITICAL note; peeling-cell caution for extensive cell–cell contacts |
| `proliferation_block` | Serum-free medium containing mitomycin C 10 µg/mL (from 0.5 mg/mL aqueous stock); growth factors/compounds added in the same serum-free medium where studied | plan | P1 Reagents Setup + step 4 + CRITICAL note (optimal treatment = minimal viability loss with maximal division inhibition) |
| `wound_comparability` | Similar widths across samples (comparison requirement) | run | P1 step 4 CRITICAL note |
| `acquisition` | 24-well plate in on-stage CO₂ chamber; 10× objective; defined positions + focal planes per scratch; 30 min interval over 24 h; `.lif` capture | run | P1 step 5 + CRITICAL notes (equilibrate CO₂/temp ≥30 min before setup; chamber slides / coverslip-bottom plates allow higher magnification; fluorescent labels allowed for resolution) |
| `incubation_between` | 5% CO₂, 37 °C throughout culture and imaging (line-dependent criteria noted) | plan | P1 Equipment Setup (CO₂ incubator; Live Cell Imaging Microscope Setup) |
| `passaging_time` / `wound_time` / `acquisition_time` | ~15 min per line (passage); ~15 min (wound induction); 24 h (acquisition) | plan | P1 step headers |
| `pause_points` | Harvested cells on ice ≤1 h; data processing per user convenience (processing itself out of scope) | run | P1 PAUSE POINTs |

### 4.4 Preconditions / completion checks

Preconditions: (P1) sub-confluent growing source culture, authenticated and
mycoplasma-checked (P1 CAUTION); (P2) counted suspension by a consistent
method; (P3) 1× PBS + serum-free medium + mitomycin C working solution ready;
(P4) on-stage CO₂/temp equilibrated ≥30 min before acquisition. Completion:
(C1) confluent monolayer confirmed before wounding; (C2) one comparable wound
per addressed well imaged at `t₀`; (C3) debris rinsed without edge scouring
(operator judgment; F-S2); (C4) every planned timepoint re-imaged at the same
positions/focal planes with raw files kept — derivation is downstream and out
of scope, never in-module.

### 4.5 Instrument requirements

Calibrated pipettors + 200 µL tips (capability: straight monolayer disruption;
model UNSPECIFIED); CO₂ incubator holding 37 °C / 5% CO₂ (P1: Thermo 4141 as
the used chamber, not a requirement); microscope with time-lapse CO₂ stage
(capability: 10× time series over 24 h; P1: Leica TCS SP5 as the used
microscope, not a requirement). No absorbance reader or analysis software is
implied by this record.

## 5. Record B — `MT-INSERT-GAP` (protocols: P2, P3)

### 5.1 Purpose

Form a defined cell-free gap without mechanical wounding by seeding around a
removable silicone barrier and lifting it after attachment. No on-path cell
killing and no wound debris by design (P2 p.1; P3 §2.2). P2 states insert-gap
results are not directly comparable to conventional scratch assays (P2 p.2).

### 5.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Counted suspension (P2 Seeding step 1; P3 §2.1 steps 1–3); Culture-Insert 2 Well on a flat, clean, dry surface (P2 Handling); sterile tweezers; cell-free medium or PBS for rinse; pre-warmed cell-free fill medium |
| Outputs | Same vessel with barrier-defined gap(s) → raw image series from `t₀` = insert removal |
| Ports | `suspension_in` → `seeded_insert_out` (`OP-INOC`, one port per insert well); `seeded_insert_in` → `gap_out` (`CAND-GAP-CREATE` insert-removal variant) |
| Sample transition | `SEEDED_AROUND_BARRIER → GAP_DEFINED(+NO_DEBRIS) → IMAGED_SERIES`. Barrier lift reveals the gap; partial detachment on lift is loss (F-I2), never modelled as lossless. |
| Time scope | `per_assay` seed-to-readout block; attachment hold ~24 h (line-dependent, P2/P3); `t₀` = removal; acquisition from `t₀` (P3 §2.3: 30–60 min frames over ~24 h; worked MCF-7 series 20 h at 30 min) |

### 5.3 Parameters (P2/P3 bindings only; silence = UNKNOWN, §11)

| Parameter | Value | `required_at` | Locator |
|---|---|---|---|
| `insert_format` | Culture-Insert 2 Well (P2 products; 2 wells separated by one wall) | plan | P2 p.1 (table: 80206/81176/80242/80209) |
| `gap_width_nominal` | 500 µm wall → 500 µm cell-free gap, ±100 µm | plan | P2 p.1 (Material Geometry + Specifications table) |
| `well_geometry` | Outer 8.4 × 8.4 × 5 mm; 70 µL/well; growth area 0.22 cm²/well; coating area 0.82 cm²/well; silicone, single-use, autoclavable/alcohol-compatible but one-time-use only | plan | P2 p.1 Specifications table + material note |
| `surface` | Flat, clean, dry surface; bottom side only is adhesive; ibiTreat hydrophilic polymer coverslip in 81176/80206/80242; no extra coating needed for adherent cells; ECM coatings allowed without restriction where required | plan | P2 p.2 (Surface); P2 Handling (no wet/damp/uneven/dusty surfaces; press fully into contact) |
| `seed_density` | 3–7×10⁵ cells/mL → confluent layer within 24 h (line-dependent); P3 worked value 3×10⁵ cells/mL for most lines | plan | P2 p.2 (Seeding step 1, incl. centrifugation to remove dead cells/debris); P3 §2.1 step 3 + AN30 pointer for optimisation |
| `seed_volume_per_well` | 70 µL per insert well; no shaking (inhomogeneous distribution otherwise) | run | P2 p.2 (Seeding step 2); P3 §2.1 step 4 |
| `surrounding_fill` | Optionally fill surrounding area with medium/suspension/buffer: vessel recommended volume minus 200 µL | run | P2 p.2 (Seeding step 3) |
| `attachment_hold` | Incubate lidded vessel as usual (e.g. 37 °C, 5% CO₂; P3: ≥24 h, ≥90% humidity against evaporation); extend + monitor, change medium if needed when not confluent | plan | P2 p.2 (step 4); P3 §2.1 step 5 + §2.2 step 1 |
| `removal` | Grip one corner with sterile tweezers, lift gently after sufficient attachment (~24 h, line-dependent) | run | P2 p.2 (step 5); P3 §2.2 step 2 |
| `rinse` | Gentle rinse with cell-free medium or PBS to remove debris / non-attached cells | run | P2 p.3 (step 7); P3 §2.2 step 3 |
| `fill_medium` | Cell-free medium at recommended vessel volume (e.g. 2 mL pre-warmed 37 °C for µ-Dish 35 mm, high), pipetted slowly from one dish edge; inhibitors/enhancers added here where studied | run | P2 p.3 (step 6); P3 §2.2 step 4 |
| `detachment_guard` | If the layer partially detaches on removal → reduce seeding density or incubation time (avoid overgrowth) | run | P2 p.3 TIP; P3 scope note (100% optical confluence target, not multilayer) |
| `imaging` | Begin immediately after gap creation; 500 µm gap in one FOV at 4×/10×; orient gap horizontally/vertically for analysis; same gap position throughout (stage-top incubator); brightfield/phase for unlabelled cells, fluorescence with live stain | run | P3 §2.3 (incl. FOV/orientation/magnification guidance) |
| `vessel_optics` | Thin-bottom optical vessels for high-resolution imaging; immersion-oil compatibility table must be respected where oil objectives are used | plan | P2 p.3 (Immersion Oil table); P3 §2.3 (widest-FOV recommendation) |

### 5.4 Preconditions / completion checks

Preconditions: (P1) insert fully attached (entire adhesive bottom in contact;
P2 Handling check); (P2) suspension counted by a consistent method, dead
cells/debris centrifuged out; (P3) sterile tweezers + rinse + pre-warmed fill
medium ready. Completion: (C1) 100% optically confluent single layer confirmed
before removal (P3 §§2.1–2.2); (C2) insert lifted without tearing the layer
(adhesion check, F-I2); (C3) gap imaged at `t₀` at a recorded position/orientation;
(C4) same positions re-imaged per schedule with raw files kept.

### 5.5 Instrument requirements

Culture inserts + compatible optical vessels (capability: flat, clean, dry,
adhesive-compatible surface; P2 catalogues as the used vessels, not a
requirement); sterile tweezers; CO₂ incubator with humidity control; inverted
microscope, optionally with stage-top incubation (capabilities only: 4×–10×
phase/brightfield time series; P3: ibidi Stage Top Incubators as the used
example, not a requirement). No reader, washer model, or analysis software is
implied by this record.

## 6. Record C — `MT-TRANSWELL-MIGRATION` (protocol: P4; migration only)

### 6.1 Purpose

Measure directional migration through membrane pores toward a lower-chamber
chemoattractant. Cells are seeded above, attractant below; after the migration
window, non-migrated apical cells are removed and migrated basal cells
(adherent path) are fixed, stained, and imaged — or collected from the lower
chamber (non-adherent path) (P4 §§3.2/3.7, Fig.1).

### 6.2 Ports and sample transitions

| Item | Value |
|---|---|
| Inputs | Single-cell suspension at 1×10⁶ cells/mL in migration buffer (P4 §3.1 step 11); Transwell inserts (P4 §2.3); migration buffer ± chemoattractant for the lower chamber (P4 §2.2/§2.4); cotton-tipped applicators; 70% ethanol; crystal violet or DAPI stain; PBS |
| Outputs | Insert with `APICAL_CLEARED + BASAL_RETAINED` faces → fixed/stained membrane + raw images (adherent); or collected lower-chamber medium + hemocytometer count (non-adherent) |
| Ports | `suspension_in` → `upper_out` (`OP-INOC` into insert); `attractant_in` → `lower_out` (`op.liquid_transfer`); `insert_in` → `cleared_out` (`CAND-APICAL-CLEAR`); `cleared_in` → `fixed_stained_out` (fix/stain reuse) |
| Sample transition | `SEEDED_UPPER + ATTRACTANT_LOWER → INCUBATED_GRADIENT → APICAL_CLEARED + BASAL_RETAINED → FIXED_STAINED + RAW_IMAGES` (adherent) or `→ COLLECTED_MEDIUM + COUNT` (non-adherent). The apical wipe forks the sample fate by membrane face; washing semantics MUST NOT be used for this step. |
| Time scope | `per_assay` seed-to-readout block; `t₀` = seeding/gradient assembly (P4 §3.2 steps 1–2); migration window 2–5 h at 37 °C / 5% CO₂ (P4 §3.2 step 3; line-dependent per Note 13) |

### 6.3 Parameters (P4 bindings only; silence = UNKNOWN, §11)

| Parameter | Value in P4 | `required_at` | Locator |
|---|---|---|---|
| `culture_gate` | Culture to 80–90% confluence before starting (P4 §3.1 step 1); initiate only if >95% viable (Note 7); strain out clumps where present | plan | P4 §3.1 steps 1–2 + Note 7 |
| `harvest` (adherent) | 2× 5 mL RT Ca²⁺/Mg²⁺-free PBS washes; 2 mL 0.25% Trypsin-EDTA, swirl, 37 °C/5% CO₂ 3 min or until detached; +3 mL warmed 10% FBS medium, gentle single-cell resuspension into 15 mL tube. Trypsin %/time empirical; limit exposure (receptor cleavage); non-enzymatic buffer where the receptor is trypsin-sensitive | run | P4 §3.1 steps 3–6 + Notes 5–6 |
| `pellet_wash` | 335×g 5 min; aspirate, flick-loosen; 5 mL PBS resuspension; repeat spin; resuspend in migration buffer | run | P4 §3.1 steps 7–10 |
| `migration_buffer` | DMEM + 10 mM HEPES + 0.1% BSA, pH 7.4 | plan | P4 §2.2 |
| `count_adjust` | 15 µL cells + 15 µL trypan blue in 96-well, mix 4–5×, 10 µL to hemocytometer; resuspend viable cells to 1×10⁶ cells/mL in migration buffer | run | P4 §3.1 step 11 |
| `insert` | 6.5 mm diameter, tissue-culture-treated polycarbonate membrane, sterile PS plates; 5.0 or 8.0 µm pores for the worked assay (pore smaller than the suspended cell diameter as a rule; pilot required) | plan | P4 §2.3 + Notes 2/8 |
| `seed_upper` | 100 µL cell solution (1×10⁵ cells) onto the insert membrane without touching it with the tip; 10 min at 37 °C/5% CO₂ to settle | run | P4 §3.2 step 1 + Notes 8–9 |
| `attractant_lower` | 600 µL migration buffer + attractant below the insert (e.g. 5 ng/mL C5a or 10% NIH3T3 fibroblast conditioned medium); add slowly, no bubbles, membrane in direct contact with the liquid | plan + run | P4 §3.2 step 2 + Notes 10–12 (concentration cell-type dependent, empirically determined) |
| `migration_window` | 2–5 h at 37 °C / 5% CO₂ (cell-type dependent) | plan | P4 §3.2 step 3 + Note 13 |
| `nonadherent_path` | Remove insert with forceps; quantify migrated cells in the 600 µL bottom medium per §6.5 | run | P4 §3.2 step 4 + §3.7 |
| `apical_clear` (adherent) | Cotton-tipped applicator, gentle wipe of the apical face; repeat to leave no non-migrated cells | run | P4 §3.2 step 7 |
| `fix` (adherent) | 1 mL 70% ethanol in a fresh well; insert in, RT 10–15 min; dry in an empty well | run | P4 §3.2 steps 8–10 |
| `stain_cv` | 0.2% (w/v) crystal violet acetate in water, 3–5 min RT; 2× PBS/water wash; dry; image at 10×/20×, multiple representative FOVs | plan + run | P4 §2.2 (solution) + §3.4 steps 1–5 + Notes 19–20 (hematoxylin alternative; avoid apical-edge remnants) |
| `stain_dapi` | 1 µg/mL DAPI (1 µL of 1 mg/mL stock into 1000 µL PBS); 600 µL/well, 10 min dark; 3× 1 mL PBS washes; fluorescence at 10×/20× representative FOVs | plan + run | P4 §3.5 steps 1–5 + Note 21 (Hoechst 33358 alternative) |
| `sterility_boundary` | Sterile technique required through §3.2 step 5 (adherent) / step 3 (non-adherent); no longer required after plate removal for clear/fix/stain | run | P4 Note 14 |

### 6.4 Preconditions / completion checks

Preconditions: (P1) pore size chosen by pilot for the cell type (Note 2);
(P2) gradient assembled with membrane–liquid contact and no trapped bubbles
(Notes 10–11); (P3) swabs + ethanol + stain + counting path ready. Completion:
(C1) window elapsed per plan; (C2) apical face cleared with basal face intact
(spot-check by imaging before fixation where the SOP requires; avoid
apical-edge remnant fields per Note 20); (C3) every addressed insert returned
raw images (adherent) or a collected-volume count (non-adherent) with
attractant, window, pore, stain, and field-sampling metadata recorded.

### 6.5 Instrument requirements

Transwell inserts + compatible plates (capability: stated pore size +
membrane; P4 §2.3 catalogues as the used inserts, not a requirement); CO₂
incubator; fixation/staining handling; inverted colorimetric microscope
(P4: DISCOVER ECHO Revolve 3 as used) and/or fluorescence microscope
(P4: EVOS Fl as used) at 10×/20× — capabilities only, no model required;
hemocytometer (non-adherent path), optionally flow cytometer / automated
counter (P4 §3.7). No plate reader, washer model, or analysis software is
implied by this record.

### 6.6 Invasion variant (existence note only — NOT a recorded procedure)

P4 §3.3 describes a Transwell invasion assay adding an ECM layer (Matrigel,
diluted 1:1–1:2, 50–100 µL, solidified 30 min–1 h at 37 °C/5% CO₂, 16–24 h
window) on the apical membrane before seeding. Listed here solely so the
migration/invasion boundary is explicit. No invasion parameter is bound, no
invasion step is planned from this file; invasion needs its own record.

## 7. Controls (method companions; readout alone proves nothing)

| Readout / step | Required controls (same run) | Source locator |
|---|---|---|
| Scratch closure (migration claim) | Proliferation block (P1: 10 µg/mL mitomycin C in serum-free) + serum-free background + comparable wound widths; gentle edge handling | P1 step 4 + CRITICAL notes |
| Scratch comparability | Similar wound widths; constant seeding number per line; same positions/focal planes across timepoints | P1 step 2 CRITICAL + step 4 CRITICAL + step 5 |
| Insert gap | Confluence-matched inserts; same-gap-position tracking; adhesion check after removal (reduce density/time on detachment) | P3 §2.3 (same position); P2 p.3 TIP |
| Transwell migration | Chemoattractant-concentration pilot per cell type (Notes 2/12–13); gradient-integrity check (contact, no bubbles, Notes 10–11); >95% viability gate (Note 7); density titration against over/under-saturation | P4 Notes 2/7/10–13 |
| Stain background | Unstained / diluent-only carrier through fix/stain (practice-level; detail UNKNOWN, §11) | General control principle; no P1–P4 locator — marked UNKNOWN |
| Imaging comparability | Same fields, same magnification/exposure across timepoints/FOVs; representative (not edge-remnant) fields | P1 step 5; P3 §2.3; P4 Notes 20/22-adjacent (representative FOV) |

## 8. Readout meaning — what the raw products are and are NOT

- **Scratch/insert raw image series** is the optical state of that
  wound/gap under its geometry, seeding, medium, block, and schedule —
  nothing more. It is NOT a migration rate, NOT a cell count, NOT
  proliferation-free unless the §7 controls hold. Scratch and insert gaps
  are NOT commensurable (P2 p.2: different creation physics and surface).
- **Transwell raw images / collected medium** show stained basal-face cells
  (adherent) or suspended bottom-chamber cells (non-adherent) for that pore,
  density, attractant, window, and field/volume rule — NOT total motility,
  NOT invasion, NOT viability.
- **No derivation lives here.** Closure arithmetic, edge advance, and
  per-FOV counts are downstream analysis and are deliberately NOT defined in
  this file (wet-lab scope). Any downstream use MUST keep raw images +
  plan metadata (attractant, window, pore, stain, positions, timestamps)
  alongside the derived value; re-running with different thresholds/fields
  is a new derivation, not a correction.

## 9. Failure modes

| # | Record | Failure | Effect | Locator |
|---|---|---|---|---|
| F-S1 | scratch | Subconfluent wounding; non-constant seeding | Ragged gaps; non-reproducible closure | P1 step 2 CRITICAL |
| F-S2 | scratch | Excess pressure / plastic residue / harsh PBS wash / peeling of high-contact monolayers | Torn/widened wound, false-slow closure; debris background | P1 step 4 CRITICAL + CAUTION |
| F-S3 | scratch | Skipped or ineffective proliferation block while claiming migration | Proliferation-driven closure misread as migration (the core §1 error) | P1 step 4 CRITICAL |
| F-S4 | scratch | Unequal wound widths; positions/focal planes not fixed | Non-comparable series; apparent treatment effects from geometry/drift | P1 step 4 CRITICAL + step 5 |
| F-S5 | scratch | Imaging without CO₂/temp equilibration (≥30 min) or pH maintenance | Drift/stress artefacts over 24 h live series | P1 Equipment Setup + step 5 CRITICAL |
| F-I1 | insert | Wet/damp/uneven/dusty surface; weak press-down; shaking after seeding | Leaky barrier, inhomogeneous monolayer, non-uniform gaps | P2 Handling + Seeding step 2 |
| F-I2 | insert | Removal before confluence / overgrown multilayer / aggressive lift | Partial detachment, torn edges, false-open gap | P2 steps 4–5 + p.3 TIP |
| F-I3 | insert | Wrong density for the line | No confluence at 24 h, or piling multilayer closing by stacking rather than migration | P2 Seeding step 1; P3 §2.1 step 3 |
| F-T1 | transwell | Wrong pore size (no pilot) | No transit (too small) or drop-through (too large) | P4 Notes 2/8 |
| F-T2 | transwell | Over/under-seeding; damaged or low-viability cells; trypsin overexposure | Saturated membrane (uncountable) or silent wells; cleaved receptors distort chemotaxis | P4 Notes 6–8 |
| F-T3 | transwell | Gradient failure (attractant omitted/misplaced, bubbles, no membrane contact) | Random transit misread as chemotaxis | P4 Notes 10–11 |
| F-T4 | transwell | Incomplete apical clear or basal scouring | Residual apical cells inflate signal; scoured basal face deflates it — same wipe fails both ways | P4 §3.2 step 7 + Note 20 |
| F-T5 | transwell | Edge-remnant fields imaged as representative | Apical leftovers counted as migrated | P4 Note 20 |
| F-Q1 | all | Dropping raw images + plan metadata, keeping only a derived number | Recomputation and audit impossible | Record-keeping rule (§8); no P1–P4 numeric locator needed |

## 10. Time scope and instrument summary

| Aspect | Scratch (P1) | Insert gap (P2/P3) | Transwell (P4) |
|---|---|---|---|
| Time scope | 24 h starve pre-wound; `t₀` = wound; 30 min frames over 24 h; passage ~15 min/line, wounding ~15 min | Seed → ~24 h attach (extend if needed); `t₀` = removal; frames 30–60 min over hours (~24 h; worked 20 h/30 min MCF-7) | `t₀` = seed/gradient; 10 min settle; 2–5 h migration; fix 10–15 min RT; stain 3–5 min (CV) / 10 min dark (DAPI) |
| Instruments | Pipettors + 200 µL tips; 37 °C/5% CO₂ incubator; time-lapse microscope with CO₂ stage (10×) | Inserts + optical vessels; tweezers; humidified CO₂ incubator (± stage-top); 4×–10× inverted microscope | Inserts + plates (stated pore/membrane); CO₂ incubator; fix/stain handling; 10×/20× colorimetric and/or fluorescence microscope; hemocytometer (± flow/automated counter) |

## 11. UNKNOWNs (do not use for planning)

| Item | Status | Reason (`UNKNOWN` class) |
|---|---|---|
| Line-specific seeding numbers (P1: "depends on cell type, size, doubling time"); line-specific attractant concentrations, migration windows beyond the worked ranges | UNKNOWN | `method_dependent` — P1 step 2 / P4 Notes 12–13 assign these to per-line optimisation/pilot |
| Mitomycin C viability impact per line; serum-deprivation tolerance per line | UNKNOWN | `method_dependent` — P1 CRITICAL notes flag optimisation without giving numbers |
| Tip pressure/speed, rinse/swab force/strokes, insert press/lift force | UNKNOWN | `source_missing` — no P1–P4 locator quantifies them; operator-dependent until an SOP fixes them |
| Pore-density tables, membrane-material choice, plate-drying/spreader detail | UNKNOWN | `source_missing` — P4 names PC membranes without a density table; S7-type detail not read |
| Stain-background control detail (unstained-carrier rule) | UNKNOWN | `source_missing` — no P1–P4 locator; practice-level only |
| Reader/imager envelope (resolution, linearity, stage stability, exposure) | UNKNOWN | `configuration_dependent` — no instrument manual read (P1/P3/P4 name used models only as examples) |
| Pipette calibration, incubator uniformity, per-lot dye/membrane/insert data | UNKNOWN | `device_state` |
| Invasion coating parameters and windows | UNKNOWN | `not_applicable` — out of scope (§6.6); needs its own record |
| Any analysis threshold, field-count rule, or statistical test | UNKNOWN | `not_applicable` — analysis not recorded in this file (§8) |
| B1–B3 full texts (Bio-protocol body, Corning guide, Abcam page) | UNKNOWN | `source_missing` — blocked/timed-out fetches, marked in §2, never used as evidence |

## 12. Verified vs NOT verified

- Verified by direct reading (2026-11-05): P1 wet-lab steps 1–5 + materials/setup locators in §4; P2 geometry/surface/handling/seeding/removal locators in §5; P3 worked MCF-7 seeding/gap/imaging locators in §5; P4 culture/migration/stain/notes locators in §6. `agent-verified` only — single reader, still needs second-person check before any `SourceChecked` claim.
- NOT verified: B1–B3 (§2, blocked); any instrument operating manual or per-build sheet; per-lot reagents/membranes/inserts; laboratory SOP adaptations; any line-specific optimisation numbers; any downstream quantification. No `SourceChecked` claim is made; this file stays `research_draft`.

## 13. What this record does not cover

- ECM/Matrigel invasion procedure, 3D/spheroid/organoid migration, microfluidic chemotaxis, single-cell speed/persistence metrics.
- Any image-analysis algorithm, cell-counting software step, statistic, or pipeline — the record stops at raw images / collected medium.
- Conversion between gap-closure readouts, migrated distance, and Transwell counts — refused by §8, not deferred.
- Catalogue- or kit-fixed numbers beyond the P2–P4 worked examples; catalogue-specific values need their own record. P1/P3/P4 name used models/catalogues as examples only, never as requirements.
- JSON cutover: all IDs stay `research_draft`; UNKNOWNs are omitted (never zero-filled) if ever schematized, and rationale in §1/§8 stays out of machine fields.

## 14. Review checklist (per `record-standard.md` §7)

- [x] Every bound value carries an external locator (P1–P4 + page/section/table), or is explicitly UNKNOWN (§11). No repo-internal record is cited as evidence.
- [x] Method requirements (density, medium, regimen, pore, window) separated from instrument specs (§10 names capabilities and used examples, never a required model).
- [x] Sample identity/state changes have ports + transitions (§§4.2/5.2/6.2); scratch debris and apical-wipe fate fork modelled as effects, not a global boolean.
- [x] Time scopes are per-record blocks anchored to explicit `t₀` events, never inferred from an assay name (§§4–6, §10).
- [x] No implicit defaults: worked values labelled per-protocol (MCF-7, J774A.1/BMDM/B16-F10, A4/OVCA lines); conflicts and line-dependent items assigned to SOP/pilot.
- [x] Partial lists (insert formats; membrane/pore options; stain alternatives) labelled as partial/used-examples, not complete capability.
- [x] Plan / actual / raw / derived separated (§8; raw images + plan metadata kept; derivation out of scope).
- [x] No AI pre-review presented as human review; status stays `research_draft`.
- [x] No JSON emitted; cutover rule recorded in §13.
