# Candidate Recheck — PCR Thermal Cycling / Electrophoresis Separation / Raw Flow Acquisition / Luminescence Raw Acquisition

-日期: 2026-11-05
- 状态: `research_draft`; not `SourceChecked`; no `ReviewRecord`. Second-person field-by-field review required before any promotion to JSON/runtime.
- Scope: Wet-lab-only re-evaluation of four candidates for public-module promotion: (1) PCR thermal cycling, (2) electrophoresis separation, (3) raw flow acquisition, (4) luminescence raw acquisition.
- Rules in force: at least two concrete independent method sources required; method-name variants do not count as independent families. If evidence is insufficient, keep method-local/gap. Luminescence plate read vs imaging MUST NOT be merged. No clinical, animal in vivo, data processing, image analysis, statistics or bioinformatics content. No Rust/CLI/Slint/database changes. No `SourceChecked` claim.
- Existing repo records (B04, B06, record-standard, granularity-freeze, catalog-promotion-audit) are Reuse Map context only; claims below cite primary protocols actually read (or search summaries where stated). This file proposes no new public module unless all four promotion gates pass.
- Promotion gates (catalog-promotion-audit §1): G1 ≥2 independent method families with real documented use; G2 semantic stability (same purpose, ports, sample effect, completion-check type); G3 abstraction benefit; G4 acceptable branch cost (parameters only, or at most one explicitly declared variant). Fail any gate → method-local or gap.

## Source index (primary protocols actually read for this recheck)

| ID | Protocol | Type | Version / locator | Read degree (2026-11-05) |
|---|---|---|---|---|
| T1 | NEB Protocol for a Routine Taq PCR Reaction (E5000S v4.0) | Vendor manual | 50 µL reaction table + cycling table (95 °C 30 s; 30× [95 °C 15–30 s / 45–68 °C 15–60 s / 68 °C 1 min/kb]; 68 °C 5 min; 4 °C hold) | Direct read of reaction/cycling tables |
| T2 | NEB Phusion High-Fidelity PCR Protocol (M0530) | Vendor manual | Reaction table + thermocycling conditions (98 °C 30 s; 25–35× [98 °C 5–10 s / 45–72 °C 10–30 s / 72 °C 15–30 s/kb]; 72 °C 5–10 min; 4–10 °C hold; 2-step variant) | Direct read of cycling table |
| T3 | Bio-Rad Quantitative PCR Basics / QIAGEN QuantiNova SYBR Green PCR Kit protocol | Vendor manual/how-to | 3-step (95 °C ~10 s / 60 °C ~30 s / 72 °C 5–30 s, ~40 cycles) and 2-step (95 °C 5 s / 60 °C 10 s, acquisition during anneal/extension) variants | Direct read of cycling procedures |
| A1 | Herman Lab (Univ. Nebraska) Agarose Gel Electrophoresis protocol | University lab protocol | 1% agarose in 0.5× TBE, 0.5× TBE immersion, ~92 V 60–90 min, loading dye, ladder, gel imager | Direct read |
| A2 | Keiser et al., Agarose Gel Electrophoresis for the Separation of DNA Fragments (PMC4846332) | Published method | 0.5–2% agarose, TAE/TBE + 1 mM EDTA, 1–5 V/cm, EtBr 0.5 µg/mL, UV visualization | Direct read |
| A3 | Rockland SDS-PAGE Protocol; Bio-Rad Performing Protein Electrophoresis / Laemmli PAGE-SDS | Vendor + university lab protocol | SDS denaturing polyacrylamide: 4–20% / 10–15% gels, ~150 V 45–90 min (or 200 V 31–39 min), reducing sample buffer, Coomassie/silver or Western downstream | Direct read of run/stain steps |
| F1 | Fred Hutch FERO Flow Cytometry protocol (Flow.pdf) | Facility/core protocol | Acquire = All, Event count = 10,000, Storage = All, Resolution = 1024, FSC threshold 50, 200–1000 events/s | Direct read |
| F2 | HKU CPOS FACSAria/NovoCyte Quanteon Standard Operation Protocol (2019) | Facility SOP | Plate/sample manager, stop conditions Events 1–10^7 / Time 0–60 min / Volume 5–5000 µL, flow rates Slow 14 / Medium 35 / Fast 66 µL/min | Direct read |
| L1 | Cayman ATP Detection Assay Kit — Luminescence (insert 700410) | Vendor kit insert | 100 µL reaction mix + 10 µL standard/sample, 15–20 min RT dark, RLU plate read within 5 min, 12 fmol–10 pmol range | Direct read |
| L2 | Promega Luciferase Assay System Technical Bulletin TB281 (plate-reading luminometer) | Vendor technical bulletin | 20 µL lysate + 100 µL reagent, 2 s delay / 10 s read, ~1 min stability window, t½ ~10 min, ambient temperature | Direct read |
| L3 | Promega ENLITEN ATP Assay GloMax 96 Microplate Luminometer Method | Vendor application note | 10 µL ATP standard/sample + 100 µL rL/L, ATP standards 10^-13–10^-7 M, pH ~7.73, 23–25 °C, fresh daily standard curve | Direct read |
| I1 | Center for Molecular Imaging IVIS Spectrum Bioluminescence Protocol; Technion IVIS 200 Protocol; LTK-RES-6-B-EN SOP | Facility SOPs (in vivo imaging) | Luciferin 150 mg/kg IP, ~10 min distribution, isoflurane anesthesia, exposure 30–60 s, 600–60,000 count target | Direct read — excluded from wet-lab scope (animal in vivo); used only to document the imaging-side separation boundary |
| — | B04 (batch-04-molecular-basics.md), B06 (batch-06-flow-and-if.md), record-standard, granularity-freeze, catalog-promotion-audit | Repo research records | As cited in header | Reuse Map context only; not step/value evidence. B04 S3 = T1 (same NEB Taq manual); B04 P4 ≈ A1-family (different lab, same agarose QA method — counts as one family, see §3); B06 S1–S5 (Thermo staining) for flow prep context only |

Note on independence counting: polymerase-brand variants (Taq vs Phusion vs Platinum Taq) within routine PCR, and 2-step vs 3-step cycling within qPCR, are method-name/parameter variants of the same cycling family, not independent families. Agarose (DNA) vs SDS-PAGE (protein) are different separation families (different matrix, buffer, sample effect) and do count separately for a generic "electrophoresis" claim — but that also means a generic merge must survive G2 across both, which it does not (§2).

## 1. (1) PCR thermal cycling

### Independent-family count

| Family | Sources | Variant or independent? |
|---|---|---|
| Conventional endpoint PCR (Taq) | T1; B04 S3 (same manual) | One family |
| Conventional endpoint PCR (Phusion HF) | T2 | Variant — same cycling semantics, enzyme/temperature values differ |
| Real-time qPCR (SYBR Green, 2-/3-step) | T3 | Same thermal-cycling semantics + per-cycle fluorescence acquisition; chemistry/analysis differ |
| RT-PCR / hot-start / gradient PCR | T1/T2 optimization sections | Variants (pre-step or optimizer), same cycling core |

Independent families with real documented use: 1 (thermal-cycling PCR). qPCR adds fluorescence acquisition during cycling, which is a different completion-check/readout type — it does not create a second family for a bare "thermal cycling" op; it creates a conflict for merging (G2).

### Comparison

| Dimension | Conventional PCR (T1/T2) | qPCR (T3) |
|---|---|---|
| Purpose | Programmatic denature–anneal–extend repetitions to exponentially amplify target; endpoint product | Same amplification, plus per-cycle fluorescence to track amplification in real time |
| Ports | In: assembled reaction (tube/plate well). Out: amplified reaction, same vessel | In: assembled reaction with dye/probe. Out: same vessel + per-cycle fluorescence trace (FAM/SYBR channel) |
| Sample effect | Nucleic-acid copy number increased; template chemistry consumed | Same amplification effect; reporter chemistry consumed per cycle |
| Completion checks | Program ran to end; hold step reached; product presence (downstream gel) | Program ran and fluorescence acquired at prescribed step each cycle; Cq callable (threshold in log-linear phase) |
| Instrument requirements | Programmable thermal cycler: 3 held temperatures, ramp rates, heated lid/mineral oil if unheated | Same + optical fluorescence unit, per-cycle acquisition timing, passive reference dye support |
| Difference cost | — | Merging bare cycling with qPCR-cycling hides the acquisition/readout difference behind a flag → G4 fails; splitting re-creates two ops for one family → G1 fails either way |

### Gate verdict

- G1 FAIL. One independent family (PCR cycling). Phusion/Taq/qPCR/RT are variants of the same method family, not independent families.
- G2 FAIL for any merge with qPCR-acquisition or with `op.incubation` (programmed cycling ≠ isothermal hold; B04 §4 already ruled: 程序化变温循环 ≠ 等温保持).
- Decision: method-local inside PCR/qPCR method templates. No new public module. The cycling program stays a Method Template binding (temperatures/times/cycle counts per T1/T2/T3); reaction assembly reuses `op.liquid_transfer`; product QC (gel, melt curve) stays downstream method/analysis.

Gap note: ramp-rate/overshoot tolerances, block uniformity, heated-lid vs mineral-oil effect are `configuration_dependent` / `source_missing` in T1–T3.

## 2. (2) Electrophoresis separation

### Independent-family count

| Family | Sources | Variant or independent? |
|---|---|---|
| Agarose gel electrophoresis (DNA, native, QA of extracts/PCR products) | A1, A2; B04 P4 (Faircloth, same QA method) | One family (A1/A2/B04-P4 are lab-to-lab instances, not independent families) |
| SDS-PAGE (protein, denaturing, polyacrylamide) | A3 | Independent family — different matrix, buffer, sample state, effect |

Independent families: 2 (agarose-DNA vs SDS-PAGE-protein). G1 is met for the word "electrophoresis" — but G2 decides whether one shared op can carry both.

### Comparison

| Dimension | Agarose-DNA (A1/A2) | SDS-PAGE protein (A3) |
|---|---|---|
| Purpose | Size-separate native DNA by electric field through agarose matrix | Size-separate SDS-denatured, uniformly negatively charged proteins by MW through polyacrylamide |
| Ports | In: DNA sample + agarose gel in TAE/TBE bath. Out: separated gel + used buffer | In: reduced/denatured protein sample + polyacrylamide gel in Tris-glycine-SDS bath. Out: separated gel + used buffer |
| Sample effect | DNA migrates negative→positive; smaller fragments faster; no denaturation of bulk sample chemistry | Proteins denatured in sample buffer (heat + reducing agent); migration governed by MW, not native charge |
| Completion checks | Dye-front/loading-dye position; bands separated; gel imaged | Dye front reaches bottom; bands separated; gel stained or blotted |
| Instrument requirements | DC power supply (92 V or 1–5 V/cm), gel box, imager (UV/blue-light) | Vertical chamber + DC supply (150–200 V), cassette/plates, imager or blotter; buffer temp ~20 °C |
| Difference cost | To unify: buffer (TBE vs Tris-glycine-SDS), matrix (% agarose vs % acrylamide), sample prep (loading dye vs reducing buffer + boil), voltage regime (V/cm vs constant V per gel), downstream (imager vs stain/blot). Every fixed field branches → G4 fails | — |

### Gate verdict

- G1 PASS (2 families) but G2 FAIL (different sample effect: native migration vs denaturing separation; different ports: agarose/TBE vs polyacrylamide/SDS buffers; different completion checks: band separation on imager vs dye-front + stain/blot) and G4 FAIL (unification cost = one branch per core field).
- Decision: method-local in each method template (agarose QA electrophoresis; SDS-PAGE/Western). This confirms B04 §9 (溶胶-铸胶-电泳分离整体 … method-local). No new public module.
- What is shared (already in Catalog): `op.liquid_transfer` (buffer/sample addition), gel imaging via `FLUO_IMAGE` (raw image only), and — for protein gels — staining/blotting steps that stay method-local per audit #23/#26.

Gap notes: gel % ↔ fragment/MW-range mapping is `method_dependent` (A1/A2/A3 disagree by design); voltage↔V/cm depends on electrode spacing (`configuration_dependent`); stain choice (EtBr/GelRed/SYBR/Coomassie/silver) and safety handling are method bindings, not op parameters.

## 3. (3) Raw flow acquisition

### Independent-family count

| Family | Sources | Variant or independent? |
|---|---|---|
| Flow cytometry acquisition (analysis, FCS file) | F1 (Fred Hutch), F2 (HKU/NovoCyte), B06 flow prep (Thermo staining protocols) | One acquisition family; F1/F2 are two facility SOPs for the same instrument class |
| Flow sorting (sterile sort, FACSAria) | F2 discusses sort hardware; B06 notes sort lineage gap | Same instrument class, different downstream act (collect subpopulations) |

Independent families: 1 (single-particle flow acquisition). Sort is a destination branch of the same acquisition, not a second family for "raw acquisition."

### Comparison (vs existing readout ops)

| Dimension | Flow acquisition (F1/F2) | `ABS_READ` (raw OD) | `FLUO_IMAGE` (raw image) |
|---|---|---|---|
| Purpose | Record per-event light scatter/fluorescence as an FCS table | Record absorbance at a wavelength per well | Record pixel intensities per (field, channel) |
| Ports | In: stained suspension in tube/plate. Out: FCS file (raw events) + sample tube (often volume-depleted) | In: plate well. Out: raw OD matrix + plate | In: slide/well. Out: raw image set + sample |
| Sample effect | Cells aspirated/consumed or collected; sample volume lost; shear/flow exposure | Non-destructive (well retained) | Non-destructive (fixed slide retained; live imaging light exposure) |
| Completion checks | Expected event count / volume / time reached; FCS file saved; QC beads within limits | Plate read finished; raw OD exported | Per-channel frames captured with exposure metadata |
| Instrument requirements | Flow cytometer: lasers, filters, PMT voltages, threshold, flow rate, sheath fluid, sample loader | Plate reader: lamp/filter, linear range | Microscope/imager: objective, cube, exposure, camera |
| Difference cost | No overlap in ports/effect/checks with ABS_READ or FLUO_IMAGE; merging any pair is a G2/G4 failure | — | — |

### Gate verdict

- G1 FAIL for a new public op: one acquisition family. (The wet-lab prep — stain/wash/FVD — is already method-local per B06 §§2–4 and audit #23/#26.)
- G2 FAIL vs ABS_READ/FLUO_IMAGE: different physics, ports, sample effect (consumptive aspiration vs non-destructive read), completion checks.
- Decision: method template internal step (raw FCS acquisition), not a public op. Sorting stays a separate downstream branch (collection + purity), never merged into acquisition (B06 §8 already: 获取/分选 … 方法模板内步骤).
- Instrument settings (event count 10,000 in F1; flow rates in F2; PMT/threshold in both) are `configuration_dependent` facility/build bindings, not method defaults — consistent with B06 G-2 (`source_missing` for laser/filter/gain/threshold values).

Gap note: absolute-counting dead-volume effect (F2) and autosampler bead-normalization are method/facility bindings; gating/compensation/statistics are out-of-scope (analysis).

## 4. (4) Luminescence raw acquisition — plate read, NOT imaging

Boundary enforcement (explicit): luminescence plate read (well-integrated RLU) and luminescence imaging (spatial pixel map, e.g. IVIS) are different ops with different ports/effects/checks and MUST NOT be merged. This file evaluates plate read only. The IVIS sources (I1) are cited solely to document the imaging side of the boundary; they are animal in vivo and out of wet-lab scope for op modeling.

### Independent-family count (plate read)

| Family | Sources | Variant or independent? |
|---|---|---|
| ATP luciferase plate read (viability/ATP quantification) | L1 (Cayman ATP), L3 (Promega ENLITEN/GloMax) | One family (same luciferase chemistry, different kits) |
| Luciferase reporter plate read (promoter activity) | L2 (Promega TB281) | Same detection physics (luciferase + luminometer), different measurand/upstream (lysis of reporter cells) — same plate-read family |
| Dual-luciferase sequential read | L2-adjacent TB281/Dual-Glo (search summary) | Variant (two reads + stop reagent), same op shape |

Independent families for the readout physics: 1 (luciferase luminometry). ATP vs reporter are different assays sharing one detector — the reuse is at instrument level, not at "two independent method families using one op." G1 for a new public op therefore fails; audit #33 already classifies luminescence as `method_local_step (gap)`.

### Comparison

| Dimension | Luminescence plate read (L1–L3) | `ABS_READ` (absorbance) | Luminescence imaging (I1, for boundary only) |
|---|---|---|---|
| Purpose | Integrate total emitted light per well → RLU | Measure transmitted light attenuation per well → OD | Map emitted light per pixel over animal/plate surface → counts/image |
| Ports | In: plate well + reagent/luciferase substrate. Out: RLU per well (sample retained in well) | In: plate well. Out: OD per well | In: animal/plate in chamber + luciferin. Out: image frames (animal retained, chamber light exposure) |
| Sample effect | Luciferase reaction consumes substrate; signal decays (flash t½ ~10 min L2 / glow read-within-5-min L1) | Non-destructive photometry | Non-destructive photon collection; substrate distribution kinetics in vivo |
| Completion checks | Read within stability window; RLU exported; standard/blank present | Read finished; OD exported | Exposure/binning within 600–60,000 counts; distribute ~10 min post-dose |
| Instrument requirements | Luminometer/plate reader in luminescence mode: injector, integration time (~1–10 s), gain; ambient T | Plate reader: absorbance optics, wavelength filter | IVIS/imager: CCD, f/stop, binning, anesthesia/light-tight chamber |
| Difference cost | Cannot be parameterized from ABS_READ (no excitation, no wavelength, different detector mode, reaction kinetics differ) → G4 fails if shoehorned | — | Cannot be parameterized from plate read (spatial vs integrated, exposure vs integration, dosing kinetics) → MUST NOT merge |

### Gate verdict

- G1 FAIL (one plate-read family). G2 FAIL vs `ABS_READ` (different physics/instrument/completion checks). G4 FAIL (shoehorning requires mode/kinetics carve-outs). Audit §4.4 and #33 concur: "Absorbance vs luminescence: different physics, instruments, and linearity claims. No shared 'plate read' op."
- Decision: method-local / gap (`method_local_step (gap)` per audit #33). The raw RLU capture stays a method-template readout binding inside ATP/reporter templates; ATP concentration derivation (standard curve) is a ToolAction/downstream derivation (L1/L3), not an operation. Plate-read vs imaging kept as two distinct template readout bindings; imaging itself is out of this recheck's wet-lab scope (in vivo).

Gap notes: kinetics type (flash vs glow) is `method_dependent` per kit (L1 glow-read-in-5-min vs L2 flash-read-immediately); pH/temperature windows (`method_dependent`, L3 pH ~7.73, 23–25 °C); gain/integration are `configuration_dependent`.

## 5. Promotion summary

| Candidate | G1 (≥2 independent families) | G2 (semantic stability) | G3 (benefit) | G4 (branch cost) | Decision |
|---|---|---|---|---|---|
| (1) PCR thermal cycling | FAIL (1 family; Phusion/Taq/qPCR are variants) | FAIL vs incubation & vs qPCR-acquisition | FAIL | FAIL | Method-local in PCR/qPCR templates |
| (2) Electrophoresis separation | PASS (agarose vs SDS-PAGE) | FAIL (native vs denaturing effect/ports/checks) | FAIL | FAIL (branch per core field) | Method-local per method (agarose QA; SDS-PAGE/WB) |
| (3) Raw flow acquisition | FAIL (1 family) | FAIL vs ABS_READ / FLUO_IMAGE | FAIL | FAIL | Method template internal step (raw FCS) |
| (4) Luminescence raw acquisition (plate read) | FAIL (1 family) | FAIL vs ABS_READ; imaging MUST NOT merge | FAIL | FAIL | Method-local / gap (audit #33) |

Zero new public modules proposed. Each candidate fails at least G1 or G2 and G4; no candidate passes all four gates. This file adds no PortSpec/Transition, no JSON, no code/UI/DB.

## 6. What remains shared (Reuse Map additions only)

- `op.liquid_transfer` — reaction assembly (T1–T3), gel loading/mixing (A1–A3), stain/wash volumes feeding flow (F1/F2 via B06), substrate addition before luminescence read (L1–L3).
- `op.incubation` — PCR/cycling is not incubation; use only for true holds (luciferase equilibration, staining, glow wait). Lysis/incubation before read stays method-bound.
- `op.centrifugation` / `op.washing` — flow prep (B06) and protein sample clarification (A3) parameterize existing ops; no new washing variant created.
- `FLUO_IMAGE` — gel imaging (A1/A2) and fixed-slide fluorescence (B06) parameterize raw acquisition; band/gel/FCS/RLU interpretation stays downstream (analysis/ToolAction).
- `ABS_READ` — unchanged; luminescence is explicitly not folded into it (audit §4.4).

## 7. Gaps and UNKNOWNs (classified per record-standard §4)

| ID | Gap | Class | Affects |
|---|---|---|---|
| G-1 | Thermal cycler ramp/overshoot/block uniformity; heated-lid vs oil evaporation effect | `configuration_dependent` / `source_missing` | (1) |
| G-2 | Gel % ↔ fragment/MW range; V/cm ↔ constant-V equivalence; stain safety handling expansion beyond source scope | `method_dependent` / `source_missing` | (2) |
| G-3 | Flow cytometer laser/filter/gain/threshold/flow-rate values; facility build sheet; sort purity/sterility spec | `configuration_dependent` / `source_missing` (B06 G-2) | (3) |
| G-4 | Luminescence kinetics (flash vs glow) per kit; pH/T windows; gain/integration limits of specific luminometers | `method_dependent` / `configuration_dependent` | (4) |
| G-5 | All second-family uses beyond the primary protocols read here (e.g., gradient/digital PCR, native PAGE, capillary/flow-imaging) | `source_missing` — not counted until a primary protocol is read | (1)–(4) |
| G-6 | In vivo luminescence imaging is out of wet-lab scope; imaging-side op modeling deferred to a separate in vivo record if ever needed | `not_applicable` (this file) | (4) boundary |

## 8. Verification statement

- Primary protocols T1–T3, A1–A3, F1–F2, L1–L3, I1 were read directly (tables/steps as located in the source index) on 2026-11-05; searches surfaced additional vendor/facility SOPs that were used only as context, not as numeric evidence, where so noted.
- B04/B06 and the standard/freeze/audit docs were used as Reuse Map context only; no value in this file is sourced from them.
- No `SourceChecked` / `ExpertReviewed` / `LabValidated` claim is made. Status remains `research_draft`. No code, CLI, UI, database, or JSON was changed. No clinical, animal in vivo (except the I1 boundary citation), data-processing, image-analysis, statistics, or bioinformatics modeling is included.
- Files written: `docs/research/protocol-discovery/candidate-recheck.md` only.
