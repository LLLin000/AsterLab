# Nucleic-Acid Extraction/QC, Reverse Transcription & qPCR Acquisition/Analysis — Operation Records

`review_status: research_draft`

> Rule: no `SourceChecked` claim is made. All values below carry a source
> locator or are marked `UNKNOWN` with a reason. No cycling program, reference
> gene, threshold, or efficiency value in this file is universal.

## 1. Identity (three separate records in one file)

| Record | Catalog ID / version | Title | `action_type` | Scope | Exclusions |
|---|---|---|---|---|---|
| R1 | `NA_EXTRACT_QC` v0.1 | Nucleic-acid extraction and pre-RT QC | extract + QC (non-amplifying) | Total RNA (or DNA where stated) isolated for downstream RT-qPCR; QC before RT | RT chemistry, qPCR, sequencing library prep, FFPE restoration protocols |
| R2 | `RT_SYNTH` v0.1 | Reverse transcription (RNA → cDNA) | synthesize (first-strand cDNA) | Discrete RT step (two-step workflow) and RT phase of one-step RT-qPCR | qPCR cycling/detection itself (R3); RNA extraction (R1) |
| R3 | `QPCR_ACQ` v0.1 | qPCR acquisition and Cq-level analysis | amplify + detect (terminal readout + derived quantities) | SYBR Green I and probe chemistries; one-step and two-step workflows; relative and absolute quantification setups | Post-Cq biological interpretation; standard-curve fitting as a separate derivation step is described here only as derived readout, not executed |

Generic mechanics (pipetting, plate handling, thermal incubation, optical
readout principles) are NOT duplicated here. They are referenced, not restated:

- liquid transfer → `docs/research/k2-operations/liquid-transfer.md` and
  `record-liquid-transfer.md`
- incubation → `docs/research/k2-operations/inoculation-and-incubation.md`
- optical readout discipline (raw vs derived) →
  `docs/research/k2-operations/absorbance-and-fluorescence-readout.md`

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator / what was used | Reading depth |
|---|---|---|---|---|
| S1 | Bustin et al. 2009, MIQE guidelines, Clin Chem 55(4):611–622 | https://pubmed.ncbi.nlm.nih.gov/19246619 ; https://academic.oup.com/clinchem/article/55/4/611/5631762 | Minimum-information concept; Essential (E) vs Desirable (D) checklist; RDML submission recommendation | `search-summary-only` — abstract/secondary summaries only; full Table 1 not verified against publisher PDF |
| S2 | RDML Consortium, MIQE checklist page | https://rdml.org/miqe.html | Full checklist table transcribed: experimental design, sample, nucleic-acid extraction, RT, qPCR target, oligonucleotides, protocol, validation, data analysis sections; footnotes on no-RT (`*`) and probe disclosure (`**`) | `agent-verified` — page read 2026-11-05; checklist rows transcribed |
| S3 | QIAGEN FAQ: "What testing should be performed to assess the quality of an RNA sample?" | https://www.qiagen.com/us/resources/faq/2660 | Spectrophotometric block (A260 conc., A260/A280, A260/A230), Bioanalyzer/gel integrity block (28S/18S, RIN), optional RT2 QC array mention | `search-summary-only` — search-result summary only; QIAGEN page not opened |
| S4 | NEB usage guideline: "Assessing RNA (NEB #T2110)" | https://www.neb.com/en/tools-and-resources/usage-guidelines/assessing-rna-neb-t2110 | Purity vs integrity framing; blanking/buffer-pH cautions; fluorometric vs spectrophotometric quantitation; RIN/RINe caveats by sample type | `search-summary-only` — search-result summary only; page fetch failed (HTTP 403) |
| S5 | Thermo Fisher: "Quantitating RNA" (Ambion tech note) | https://www.thermofisher.com/us/en/home/references/ambion-tech-support/rna-isolation/tech-notes/quantitating-rna.html | UV A260/A280 quantitation principle | `search-summary-only` — search-result summary only; page not opened |

Personally verified 2026-11-05: S2 checklist rows only. NOT verified:
publisher PDF of S1, any kit manual (extraction/RT/qPCR), any instrument
operator manual, any per-lot oligonucleotide sheet, any laboratory SOP
adaptation. Numerical QC cutoffs in §5/§8 from S3–S5 are second-hand ranges,
NOT laboratory acceptance criteria, and MUST NOT be planned as pass/fail
without the primary manual.

## 3. Purpose (what each step changes)

- R1 `NA_EXTRACT_QC`: biological sample → isolated nucleic acid + QC metrics
  (concentration, purity ratios, integrity score, contamination/inhibition
  flags). No amplification, no sequence-specific detection.
- R2 `RT_SYNTH`: RNA (+ primers + RT enzyme + dNTPs + buffer) → single-stranded
  cDNA archive. Changes the analyte class; efficiency is priming- and
  sample-dependent and is NOT assumed complete or uniform.
- R3 `QPCR_ACQ`: cDNA/DNA (+ primers/probe + polymerase + buffer) →
  fluorescence amplification curves + per-well Cq (raw) → normalized/derived
  quantities (ΔCq, ΔΔCq / efficiency-corrected ratio, or absolute copy number
  via standard curve). Detection only; biological "up/down-regulation" is
  downstream interpretation, not this record's readout.

## 4. Workflow variants that MUST NOT be collapsed

| Distinction | What differs | Recording consequence |
|---|---|---|
| One-step vs two-step | One-step: RT and qPCR in the same well/reaction without an archived cDNA intermediate. Two-step: R2 produces storable cDNA; R3 consumes an aliquot | One-step: no `cDNA_out` port; RT conditions are a phase inside R3. Two-step: R2 `cDNA_out` → R3 `cDNA_in`; cDNA storage conditions recorded per S2 (D) |
| SYBR Green I (dsDNA-binding dye) vs probe (sequence-specific) | SYBR binds any dsDNA (primer-dimers included); probe reports only probe-hybridized amplicon | SYBR runs REQUIRE melt/digest/gel/sequence specificity evidence and reporting of the Cq of the no-template control, NTC (S2, E). Probe runs: probe-sequence disclosure is strongly encouraged but not E when vendors withhold it (S2, D `**`) |
| Relative vs absolute quantification | Relative: target normalized to reference gene(s) and a calibrator group, efficiency-aware. Absolute: interpolated against a standard series of known copy number | Relative REQUIRES reference-gene justification + normalisation-method description (S2, E); absolute REQUIRES standard-curve slope/y-intercept, efficiency from slope, r², linear dynamic range, LOD evidence (S2, E). Never plan one without its companion data |

No reference gene is named or endorsed here. Any gene symbol appearing in a
future plan is `method_dependent` UNKNOWN until justified per S2.

No cycling program is given here. "Complete thermocycling parameters" is an
E item that MUST be recorded per run (S2); any temperature/time in a plan is
assay-kit-dependent, not a default from this file.

## 5. Operation contracts

### 5.1 R1 `NA_EXTRACT_QC` — extraction and pre-RT QC

| Item | Contract |
|---|---|
| Function | Isolate nucleic acid by a documented procedure/kit and emit QC metrics before any RT is planned. |
| Input port `sample_in` | Biological sample with MIQE sample block recorded: description (E); processing procedure (E); micro/macrodissection where applicable (E); freeze/fix details with speed (E); storage conditions + duration, especially FFPE (E); volume/mass processed (D) (all S2). |
| Output port `na_out` | Isolated nucleic acid, identified by extraction procedure/instrumentation (E) + kit name with modifications (E) + source of extra reagents (D) (S2). DNase/RNase treatment details (E) travel with the output (S2). |
| Output port `qc_out` | QC record only (no biology call): quantification incl. instrument + method (E); purity A260/A280 (D); yield (D); integrity method/instrument (E); RIN/RQI or 3′/5′ Cq (E); electrophoresis traces (D); contamination assessment DNA-or-RNA (E); inhibition testing by dilution/spike/other (E) (all S2). |
| Preconditions | Sample block complete to E level; extraction procedure identified before the run; QC instruments available (spectrophotometer/fluorometer, integrity platform) or their absence recorded as a gap. |
| Completion checks | `deterministic`: E-level MIQE extraction rows present (procedure, kit+modifications, DNase/RNase, contamination, quantification+method, integrity method+score, inhibition). `operational`: purity/yield scores recorded where measured (D rows are recorded-if-available, never defaulted). `advisory`: second-hand QC ranges (see §8) attached as context, not gates. All blocking logic is SOP-defined, not defined by this draft. |
| Time scope | `per_assay` for the extraction batch; QC reads are `once` per eluate. Storage duration (sample and eluate) is a separate clock and MUST be recorded, not inferred from assay name. |
| Instrument capability required | Liquid-handling per generic transfer record; separation device per kit (centrifuge/magnetic stand — kit-manual-dependent, UNKNOWN here); quantification reader (UV spectrophotometer and/or fluorometer); integrity platform (capillary electrophoresis or gel). No model, wavelength, or cutoff is set by this draft. |

### 5.2 R2 `RT_SYNTH` — reverse transcription

| Item | Contract |
|---|---|
| Function | Convert an RNA aliquot to first-strand cDNA under fully recorded conditions. Two-step: archived intermediate. One-step: phase inside R3 with no archived cDNA. |
| Input port `rna_in` | QC-passed-by-SOP RNA aliquot; amount + reaction volume recorded (S2, E). |
| Input port `reagents_in` | Priming oligonucleotide identity + concentration, when gene-specific priming is used (E); reverse transcriptase identity + concentration (E); temperature + time (E); reagent manufacturer + catalogue numbers (D) (all S2). Primer strategy class (oligo-dT / random / gene-specific) is recorded as given by the kit protocol; no strategy is recommended here. |
| Output port `cdna_out` | cDNA + storage conditions (D) (S2). Two-step only. |
| Output port `rt_qc_out` | Cqs with and without RT (D, essential-at-validation per S2 footnote `*`: no-RT assay is essential when RNA is first validated as DNA-free, desirable thereafter). |
| Preconditions | R1 `qc_out` exists for the input RNA; DNase treatment status known (S2, E in R1); RT enzyme/primer strategy chosen per assay protocol, not defaulted. |
| Completion checks | `deterministic`: complete reaction conditions recorded (S2, E). `operational`: ±RT Cq pair present at validation stage. `advisory`: cDNA storage conditions noted. RT efficiency is NOT a completion check in this draft (no sourced universal value). |
| Time scope | `per_assay` (per RT batch); RT temperature/time belong to R2, not to qPCR cycling. |
| Instrument capability required | Controlled thermal device covering the assay's RT temperature/time (capability, not a model or program); generic transfer for setup. One-step variant additionally requires the R3 fluorescence-thermal capability during the RT phase. |

### 5.3 R3 `QPCR_ACQ` — qPCR acquisition and Cq-level analysis

| Item | Contract |
|---|---|
| Function | Amplify target(s) under fully recorded chemistry + cycling, acquire fluorescence curves, determine per-well Cq by a stated method, apply stated replicate/outlier/statistics rules, and emit derived quantities only with their companions (efficiency, references, standards). |
| Input port `template_in` | cDNA aliquot (two-step) with amount recorded (S2, E: reaction volume + amount of cDNA/DNA), or RNA aliquot entering a one-step reaction (RT phase conditions recorded under R2 rules). |
| Input port `assay_in` | Target block (S2): sequence accession (E); amplicon length (E); in-silico specificity screen (E); exon/intron location of each primer where applicable (E); splice-variant targeting (E); amplicon location (D); pseudogene/homolog check (D); alignment (D); secondary-structure analysis (D); multiplex efficiency + LOD per assay where multiplexed (E). Oligonucleotide block (S2): primer sequences (E); modification location/identity (E); probe sequences (D, strongly encouraged, `**`); RTPrimerDB ID (D); manufacturer (D); purification method (D). Protocol block (S2): complete reaction conditions (E); primer/probe/Mg++/dNTP concentrations (E); polymerase identity + concentration (E); buffer/kit identity + manufacturer (E); buffer constitution (D); additives incl. SYBR Green I, DMSO (E); plate/tube manufacturer + catalogue (D); complete thermocycling parameters (E); setup manual/robotic (D); instrument manufacturer (E). |
| Output port `data_raw_out` | Raw: per-well amplification curves + Cq + Cq-determination method (S2, E: analysis program source/version + Cq method) + outlier rule + disposition (S2, E) + NTC results (S2, E). Raw Cq is NOT normalized, NOT efficiency-corrected, NOT a fold-change. |
| Output port `data_derived_out` | Derived ONLY with companions: validation block (optimisation evidence D; specificity by gel/sequence/melt/digest E; SYBR NTC Cq E; standard-curve slope + y-intercept E; efficiency from slope E; efficiency CI/SE D; r² E; linear dynamic range E; Cq variation at lower limit E; CI through range D; LOD evidence E; multiplex efficiency+LOD E) + analysis block (reference-gene number/choice justification E; normalisation-method description E; biological-replicate number + concordance D; technical-replicate number + stage RT-or-qPCR E; intra-assay repeatability E; inter-assay reproducibility %CV D; power analysis D; statistical methods E; software source/version E; RDML submission D) (all S2). |
| Preconditions | Assay validation companions exist for the claimed quantification mode (§4 table); channel/chemistry match confirmed (SYBR vs probe); reference-gene justification and normalisation method stated before any "relative expression" is emitted; standard series + LOD stated before any "copy number" is emitted. |
| Completion checks | `deterministic`: every E row above recorded or explicitly gapped as UNKNOWN. `operational`: NTC/NRT/reference/standard companions present per §6. `advisory`: RDML export, power analysis, reproducibility %CV. A run missing an E companion is recorded as incomplete for that claim, not rescued by statistics. |
| Time scope | `per_assay` for the plate/run; cycling/RT-phase clocks are run-level. Biological timepoints upstream are NOT inferred from the plate name. |
| Instrument capability required | Real-time thermal cycler with fluorescence detection: block format matching plates/tubes; channel(s)/filters matched to the chemistry on board; documented Cq-determination software (source + version); uniform thermal control across the block (ranges are device- and assay-dependent, UNKNOWN here). No brand, model, channel count, or program is specified. |

## 6. Controls (MIQE-relevant; readout alone proves nothing)

| Control / companion | Required with | Why (S2) | Notes |
|---|---|---|---|
| No-template control, NTC (incl. Cq for SYBR, E) | Every R3 run; Essential for SYBR | Detects contamination / primer-dimer signal | A late NTC Cq is data, not an auto-pass; disposition by stated outlier rule |
| No-RT control (±RT Cq pair) | RNA validation stage essential (`*`); desirable thereafter | Tests for genomic-DNA carryover into the RNA-validated sample | Lives at the R1/R2 boundary; once the sample is validated DNA-free, continued inclusion is D |
| Inhibition test (dilution series, spike, or other, E) | R1 QC | Polymerase/RT inhibitors survive extraction | Method recorded; no universal ΔCq-shift cutoff is carried in this draft |
| Contamination assessment (DNA-or-RNA, E) | R1 QC | Cross-nucleic-acid carryover confounds RT-qPCR | DNase/RNase treatment details are the companion record |
| Specificity evidence (gel, sequence, melt, or digest, E) | Every R3 assay; melt mandatory in practice for SYBR | SYBR reports any dsDNA | Primer-dimers and off-targets are the failure, not the dye |
| Reference gene(s) + justification + normalisation method (all E) | Any relative-quantification claim | Single unvalidated reference is the classic MIQE failure | Number, choice rationale, and stability evidence required; no gene endorsed here |
| Standard curve (slope, y-intercept, efficiency, r², linear range, LOD) | Any absolute-quantification or efficiency-corrected claim | Efficiency comes from the slope of that curve, not from a textbook value | Efficiency CI/SE, CI through range: D; multiplex needs per-assay efficiency + LOD |
| Biological + technical replicates (counts E/D; stage RT-or-qPCR E) | Every claim | Separates biological variation from pipetting/cycling variation | Repeatability (intra, E) vs reproducibility (inter, %CV, D) recorded separately |
| Positive / calibrator sample | Relative runs (calibrator group per experimental design E) | ΔΔCq needs a stated calibrator | Experimental/control group definitions are E (S2) |

## 7. Plate / channel setup (record, do not standardize)

- Plate/tube manufacturer + catalogue (D), reaction volume + template amount
  (E), primer/probe/Mg++/dNTP concentrations (E), polymerase + buffer/kit
  identity (E), additives (E), setup manual/robotic (D): all per S2, recorded
  per run. No volume or concentration is defaulted here.
- Channel/dye assignment (SYBR vs each probe fluorophore) MUST match the
  instrument's fitted filters; multiplex requires per-assay efficiency and LOD
  (S2, E). Filter part numbers, excitation/emission bands, and passive-reference
  choices are `configuration_dependent` UNKNOWNs (no instrument manual read).
- Well map (NTC, NRT, standards, references, unknowns, replicate layout) is a
  plan-level artifact; this draft requires its existence, not its content.
- Complete thermocycling parameters (E) are recorded per run; this file carries
  no denaturation/annealing/extension triple and no cycle count.

## 8. Parameters (only sourced requirements; values are per-assay UNKNOWNs)

### 8.1 R1 parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `sample_block` (description, processing, freeze/fix speed, storage) | record | S2 (E/D rows) | plan + run | No content defaulted |
| `extraction_procedure` + `kit_name` + `modifications` | text | S2 (E) | plan + run | Kit-manual-dependent; no kit endorsed |
| `dnase_rnase_treatment` | text | S2 (E) | run | Absence is data; record, do not assume |
| `quant_method` + `quant_instrument` | text | S2 (E) | run | UV vs fluorometric distinguished; instrument ID recorded |
| `purity_a260_a280` | ratio, dimensionless | S2 (D); ranges second-hand S3/S4 | run (if measured) | Reported ranges (S3: 1.8–2.0 RNA; S4: ≈1.9–2.1; S5: principle only) are `search-summary-only` context, NOT gates. Blanking buffer/pH recorded per S4 caution |
| `purity_a260_a230` | ratio, dimensionless | S2 context; ranges S3/S4 | run (if measured) | S3 second-hand: >1.7; S4 second-hand: ≈2.0–2.2. Same status as above |
| `yield_conc` | conc. (ng/µL) + volume | S2 (D: yield) | run (if measured) | No target carried; low-concentration spectrophotometry limits per S4 noted qualitatively only |
| `integrity_method` + `integrity_score` (RIN/RQI or 3′/5′ Cq) | text + score | S2 (E) | run | S3 second-hand: RIN ≥ 7.0 with 28S:18S ≈ 2:1; S4 second-hand: RIN > 7 often good but application-dependent. Context only, NOT pass/fail |
| `electrophoresis_traces` | file reference | S2 (D) | run (if available) | Raw traces archived, not summarized away |
| `contamination_assessment` | text/result | S2 (E) | run | DNA-in-RNA or RNA-in-DNA per workflow |
| `inhibition_test` | method + result | S2 (E) | run | Dilution/spike/other; no universal cutoff |

### 8.2 R2 parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `rna_amount` + `reaction_volume` | mass (ng/µg) + volume (µL) | S2 (E) | plan + run | No default |
| `priming_strategy` + `primer_conc` | enum/text + conc. | S2 (E when GSP) | plan + run | Strategy recorded as kit-given; no strategy recommended |
| `rt_enzyme` + `rt_enzyme_conc` | text + conc./units | S2 (E) | plan + run | No enzyme endorsed |
| `rt_temp` + `rt_time` | °C + min | S2 (E) | run | No program given; kit-manual-dependent |
| `reagent_manufacturer` + `catalogue_nos` | text | S2 (D) | plan | Traceability only |
| `cdna_storage` | conditions (T/duration) | S2 (D) | run (two-step) | Recorded, not assumed stable |
| `cq_with_rt` / `cq_no_rt` | Cq pair | S2 (D `*`) | validation stage | Essential at first validation, D thereafter |

### 8.3 R3 parameters

| Parameter | Type / unit | Source | `required_at` | Constraints / notes |
|---|---|---|---|---|
| `target_block` (accession, amplicon length, specificity screen, exon/intron, splice variants) | record | S2 (E/D rows) | plan | No sequence carried; per-assay record |
| `primer_sequences` + `modifications` | sequence text | S2 (E) | plan + run | Full disclosure required |
| `probe_sequences` | sequence text | S2 (D `**`) | plan + run | Strongly encouraged; vendor-withheld is recorded as such, not reconstructed |
| `reaction_conditions` (volumes, concs, polymerase, buffer/kit, additives) | record | S2 (E/D rows) | plan + run | No concentration defaulted |
| `thermocycling_params` | program (T/t/cycles) | S2 (E) | plan + run | Per-assay UNKNOWN until the kit protocol is read; NEVER copy between assays |
| `pcr_efficiency` (from slope) + `slope` + `y_intercept` + `r2` + `linear_range` + `lod` | number/record | S2 (E; CI/SE D) | analysis | Computed from that run's curve, not assumed; formulae live in the analysis software (source/version recorded) |
| `cq_method` + `analysis_program` (source, version) | text | S2 (E) | run + analysis | Threshold/baseline rules are software-version-dependent |
| `reference_genes` + `justification` + `normalisation_method` | record | S2 (E) | plan + analysis | No gene named here; stability evidence required by SOP |
| `replicates_bio` + `replicates_tech` (+ stage) | counts | S2 (D/E) | plan + run | Stage (RT vs qPCR) recorded for tech replicates |
| `statistics` + `software` | text | S2 (E) | analysis | Significance method + software version recorded |

## 9. Sample transitions (ports + terminality)

- R1: `sample → [NA_EXTRACT_QC] → nucleic_acid + qc_record`. Eluate may
  re-enter storage (clock recorded) or proceed to R2. Failed-QC eluate is
  quarantined per SOP, not auto-excluded by this draft.
- R2 (two-step): `rna → [RT_SYNTH] → cdna_archived (+ rt_qc)`. cDNA is a
  storable intermediate; aliquots enter R3. RT is directional; cDNA does not
  revert to RNA.
- R2 (one-step): `rna → [RT-phase inside R3] → amplicon_signal`. No archived
  cDNA transition exists; the well is consumed by amplification.
- R3: `template → [QPCR_ACQ] → consumed_well + raw_curves/Cq (+ derived)`.
  Amplified wells are terminal (post-PCR product); they MUST NOT re-enter
  culture, extraction, or a second qPCR as template without an explicit
  documented dilution/transfer (contamination risk noted, handling SOP-gated).

## 10. Failure modes

| # | Failure | Detection (in-module) | Mitigation / disposition |
|---|---|---|---|
| F1 | Genomic-DNA carryover (no/incomplete DNase) | ±RT Cq pair close; contamination assessment flag (S2) | Re-DNase or re-extract upstream; record, do not subtract away |
| F2 | Inhibitor carryover (salts, phenol, ethanol) | Inhibition test; depressed A260/A230 context (S3/S4 second-hand); shifted/delayed Cq in dilution series (S2) | Re-purify/dilute per SOP; record method + result |
| F3 | Degraded input (low integrity) | RIN/RQI or 3′/5′ Cq; traces (S2) | Re-extract or SOP-gated proceed-with-flag; never silently pool with intact samples |
| F4 | Wrong quantitation (blank/pH error; spec at low conc.; DNA counted as RNA) | S4 cautions: blank, buffer pH, low-ng/µL limits, dye-vs-UV mismatch | Re-blank in correct buffer; confirm by fluorometric or RNA-specific method; record both |
| F5 | RT priming bias / incomplete RT | ±RT gap; 3′/5′ Cq spread (S2) | Change priming strategy per kit guidance upstream; record strategy, do not pool strategies in one analysis |
| F6 | Primer-dimer / off-target (esp. SYBR) | Melt/gel/sequence; early NTC Cq (S2, E for SYBR) | Re-optimize/redesign upstream; SYBR run without specificity evidence is incomplete |
| F7 | Contamination (template in NTC; amplicon carryover) | NTC Cq present (S2, E) | Invalidate affected layout per SOP; decontaminate workflow; one-step and post-PCR handling separation is SOP business |
| F8 | Efficiency assumed instead of measured | Missing slope/y-intercept/r²/range/LOD (S2, E) | Block absolute/efficiency-corrected claims until the curve exists |
| F9 | Single unvalidated reference gene | Missing justification/normalisation (S2, E) | Block relative claims until references are justified |
| F10 | Channel/chemistry mismatch; multiplex cross-talk | Pre-run capability check; per-assay efficiency+LOD missing (S2, E for multiplex) | Block run; reassign channels or split to singleplex |
| F11 | Cq-method drift (threshold/baseline/software version changed mid-study) | `cq_method` + program version mismatch across runs (S2, E) | Re-analyze under one stated method or record the break; cross-run ΔCq across methods is NOT commensurable |

## 11. Readout meaning — raw vs derived

- Raw (this module produces): per-well fluorescence curves, per-well Cq with
  its determination method + program version, NTC/NRT Cqs, melt/gel/sequence
  specificity artifacts, standard-series Cqs, reference-gene Cqs. Raw Cq is a
  detection-cycle number under stated threshold/baseline rules — NOT
  expression, NOT copy number, NOT fold-change.
- Derived (only with companions): ΔCq (target − reference, needs justified
  references + normalisation description); ΔΔCq / efficiency-corrected ratio
  (needs calibrator group + measured efficiencies); absolute copy number
  (needs same-run standard curve + linear range + LOD). Curve fitting,
  efficiency computation, normalisation, and statistics are downstream
  derivations with their own E companions (§5.3); they MUST NOT be stored as
  if they were raw Cq.
- Cross-chemistry and cross-method comparisons (SYBR Cq vs probe Cq; one-step
  Cq vs two-step Cq; relative vs absolute numbers) are NOT commensurable
  inside these records.

## 12. Time scope and instrument capability summary

- In-module clocks: extraction batch (`per_assay`), eluate/sample storage
  duration (recorded separately), RT temperature/time (R2), qPCR cycling +
  acquisition (R3). Upstream biological timepoints and downstream analysis
  time are separate nodes and MUST NOT be inferred from plate/run names.
- Capability gates (no models): R1 needs extraction separation + UV and/or
  fluorometric quantitation + integrity platform; R2 needs a controlled RT
  thermal step; R3 needs a real-time fluorescence thermal cycler with
  chemistry-matched channels and versioned Cq software. Any brand, model,
  block size, channel count, or temperature range is a per-device UNKNOWN
  until its manual is read.

## 13. UNKNOWNs (do not plan on these)

| Item | Status | Reason class |
|---|---|---|
| Extraction kit choice, lysis/binding/wash/elution volumes, yields per sample type; centrifuge/magnet parameters | UNKNOWN | `method_dependent` + `source_missing` — no kit manual read |
| Universal A260/A280, A260/A230, RIN/RQI cutoffs; 28S:18S rule; DV200 rules; per-application thresholds (e.g. FFPE vs fresh) | UNKNOWN as gates | `method_dependent` — S3/S4 ranges are `search-summary-only` context; acceptance is SOP- and application-defined |
| Spectrophotometer/fluorometer models, blanking SOP, detection limits; integrity platform model + ladder + RIN algorithm version | UNKNOWN | `configuration_dependent` + `device_state` — no instrument manual or build sheet read |
| RT enzyme choice, priming strategy per target, RT T/t program, cDNA stability duration | UNKNOWN | `method_dependent` — kit-manual-dependent; no program carried here |
| Primer/probe sequences, concentrations, Mg++/dNTP levels, polymerase/buffer identity, additives, plate/tube catalogue | UNKNOWN | `method_dependent` — per-assay record, never defaulted |
| Thermocycling program (any denaturation/annealing/extension triple, cycle count, ramp, hot-start, one-step RT-phase program) | UNKNOWN | `method_dependent` — S2 requires per-run recording; this draft sets none |
| Channel/filter bands, multiplex dye sets, passive reference, crosstalk compensation; block uniformity/accuracy specs; Cq software behavior | UNKNOWN | `configuration_dependent` — no qPCR instrument manual read |
| Efficiency value, r² target, LOD copy number, reference-gene identity/count, normalisation formula, statistical test choice | UNKNOWN | `method_dependent` — measured/justified per study; no textbook value adopted |
| Per-well positions of controls/standards/replicates (well map content) | UNKNOWN | `not_applicable` to this generic record — plan-level artifact |

## 14. Evidence status

- `agent-verified`: S2 MIQE checklist structure and E/D assignments as
  transcribed in §5–§8 (read 2026-11-05 at https://rdml.org/miqe.html).
- `search-summary-only`: S1 (MIQE paper framing), S3 (QIAGEN QC ranges), S4
  (NEB purity/integrity guidance + caveats), S5 (Thermo UV principle). These
  MUST NOT ground `Compatible` decisions or acceptance gates.
- No `SourceChecked`, no `ExpertReviewed`, no `LabValidated`. Promotion
  requires a second person to re-verify each E/D row against the S1 publisher
  PDF + the actual kit/instrument manuals, recorded in a `ReviewRecord`
  (reviewer, date, object versions, scope, evidence, outcome) — absent here.

## 15. What this record does not cover

- Brand/model selection or per-device profiles (belongs in
  `docs/research/k2-instruments/`; no instrument profile is created here).
- Kit-level SOPs: actual extraction volumes, RT programs, primer designs,
  cycling programs, threshold settings (belong in per-assay protocols citing
  the manufacturer manual page/table/section).
- Derivation mathematics: efficiency-from-slope computation, ΔΔCq/error
  propagation, standard-curve regression, statistical testing (belong in the
  versioned analysis software + statistics plan, recorded by source/version).
- Digital PCR, isothermal amplification, one-step-vs-two-step efficiency
  comparisons, FFPE restoration, and sequencing QC (RIN/DV200-for-RNA-seq
  gates, DIN for DNA) beyond the MIQE pre-RT scope noted here.
- Laboratory decisions: QC pass/fail cutoffs, reference-gene choice,
  calibrator-group definition, replicate-count justification, RDML-submission
  policy. This draft records what MUST be written down; it does not make the
  call.
