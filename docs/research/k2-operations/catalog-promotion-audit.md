# Catalog Promotion Audit — Atomic Operation candidates

- Date: 2026-11-05
- Status: `research_draft`; **document-level audit, NOT `SourceChecked`** — no `ReviewRecord` exists. No code, UI, database, or JSON changes. This file promotes nothing to runtime; it advises what a future public Catalog may contain after second-person review.
- Scope: Candidate atomic operations from `granularity-candidates.md`, `granularity-freeze.md` (20-item freeze table), `granularity-candidate-review.md`, `granularity-scenarios.md`, and operation records (`record-liquid-transfer.md`, `washing-and-sampling.md`, `inoculation-and-incubation.md`, `cell-culture-and-separation.md`, `absorbance-and-fluorescence-readout.md`, `biofilm-cv-cfu.md`, `wound-healing-and-migration.md`, `three-d-and-organoid-operations.md`, `viability-and-cytotoxicity-methods.md`, `qpcr-and-nucleic-acid-qc.md`, `elisa-and-western-blot.md`).
- Method: Catalog promotion is **not** atomicity. Each candidate is tested against four promotion gates (§1). Reuse evidence cited here is document-level (Reuse Maps and scenario decompositions); it is not source verification and does not satisfy record-standard §4.

## 1. Promotion gate (all four required)

| # | Gate | Pass means |
|---|---|---|
| G1 | Real reuse | Documented at document level in ≥2 independent method families (e.g., cell culture vs microbial vs molecular vs imaging), not 2 variants of the same method. |
| G2 | Semantic stability | Same purpose, ports, sample effect, and completion-check type across those families. Parameters may vary; effect must not. |
| G3 | Abstraction benefit | One public definition + parameter binding replaces repeated per-method redefinition. |
| G4 | Acceptable branch cost | Variants (if any) differ in parameter values only, or in at most one explicitly declared variant with its own loss/terminality note. Divergent effects hidden behind flags = fail. |

A candidate failing any gate is classified as `method_local_step` (keep the atomic step inside its method record), `composite_block`, `method_template`, or `tool_action` per record-standard §1.2 — it is not deleted, it is not public.

## 2. Decision table

Shared = `shared_operation` (promote to public Catalog, §5). Streak is promoted as a narrow exception; it remains separate from spread. All other classes stay private to their method/composite/template/tooling.

| # | Candidate | Class | G1 | G2 | G3 | G4 | Decision |
|---|---|---|---|---|---|---|---|
| 1 | Liquid transfer (`op.liquid_transfer`) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote. |
| 2 | Sample aliquot / sampling (`op.sample_aliquot`) | `shared_operation` (conditional, §6.1) | ✓ | ~(lineage gap) | ✓ | ✓ | Promote with lineage note. |
| 3 | Incubation / hold (`op.incubation`) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote (elapsed/deviation only). |
| 4 | Washing, sample-preserving (`op.washing`) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote. |
| 5 | Washing, lossy variant | `shared_operation` variant of #4 | ✓ | ✓ | ✓ | ✓ | Promote as explicit variant, never default. |
| 6 | Centrifugation | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote. |
| 7 | Post-centrifugation supernatant removal | `method_local_step` (reuse of #2) | ✓ | ✓ | ✗ (no benefit over #2) | ✓ | Do not create separate public op; express as `sample_aliquot`/transfer after centrifugation. |
| 8 | Absorbance read, raw OD (`ABS_READ`) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote. |
| 9 | Fluorescence image acquisition, raw (`FLUO_IMAGE`) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote (raw acquisition only). |
| 10 | Phase/brightfield image acquisition | `shared_operation` (modality parameter of #9) | ✓ | ✓ | ✓ | ✓ | Promote as modality binding, not new op. |
| 11 | Cell concentration count (hemocytometer/automated count) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote (count only). |
| 12 | Viability % / viability call | `tool_action` + `method_template` | — | ✗ (method-dependent measurand) | ✗ | ✗ | Not an operation. Live/dead gating, CCK-8 % etc. are derivation inside method templates. See §4.2. |
| 13 | Streak inoculation | `shared_operation` (narrow exception) | ~ (microbial isolation + purity check; treated as one family) | ✓ | ✓ | ✓ | Promote as a separate, narrow operation — never merged with spread. See §4.1. |
| 14 | Spread inoculation | `shared_operation` (narrow) | ✓ (enumeration families) | ✓ | ✓ | ✓ | Promote. See §4.1. |
| 15 | Colony count (30–300 readout) | `shared_operation` | ✓ | ✓ | ✓ | ✓ | Promote (count + image only), held out of first set per §7 G-2. |
| 16 | CFU/mL derivation | `tool_action` | — | — | — | — | Never an operation; deterministic calculation per method template. |
| 17 | Serial / continuous dilution | `composite_block` | ✓ | ✓ | ✓ | ✓ | Composite of repeated #1; not an atomic op. |
| 18 | Cell passaging (adherent) | `composite_block` | ✓ | ✓ | ✓ | ✓ | Composite; not public atomic. |
| 19 | Static biofilm culture | `composite_block` | — | — | — | — | Composite/method phase. |
| 20 | Biofilm CV (wash–stain–solubilize–read) | `method_template` | — | — | — | — | Template; internals reuse #4/#5/#8. |
| 21 | Biofilm CFU | `method_template` | — | — | — | — | Template; internals reuse #5/#14/#15. |
| 22 | CCK-8, ELISA, Western blot, qPCR, flow cytometry, IF | `method_template` | — | — | — | — | Templates; not operations. |
| 23 | Generic "staining" | `method_local_step` | ✓ (occurs widely) | ✗ (CV dye vs antibody label vs viability dye: different effects) | ✗ | ✗ | No generic stain op. Per-dye steps live in methods. See §4.3. |
| 24 | Dissociation (enzymatic/mechanical dispersal) | `method_local_step` | ✓ | ✗ (enzyme vs sonication vs scraping effects differ) | ✗ | ✗ | Method-local; never fold into washing (#4) or transfer (#1). |
| 25 | Solubilization (e.g., CV elution) | `method_local_step` | ✗ | — | ✗ | — | Method-local sub-step. |
| 26 | Fixation / permeabilization / blocking | `method_local_step` | ✓ | ✗ (terminality and next-step consequences differ) | ✗ | ✗ | Do not make a generic "fix" op; method-local. |
| 27 | Nucleic-acid / protein quantification | `method_template` (binding) | ✗ (A260 vs BCA: different blanks/units) | ✗ | ✗ | ✗ | No shared op. See freeze §3. |
| 28 | `CAND-GAP-CREATE` (scratch + insert removal) | `method_local_step` | ✗ (1 assay family) | ✗ (destructive vs barrier-reveal) | ✗ | ✗ | **Downgrade — §3.1.** |
| 29 | `CAND-APICAL-CLEAR` (Transwell swab wipe) | `method_local_step` | ✗ (1 method) | ✓ | ✗ | ✓ | **Downgrade — §3.2.** |
| 30 | 3D matrix dome / embedding | `composite_block` + Custom Step | ✗ | ✗ (dome/embed/sandwich; matrix-dependent) | ✗ | ✗ | **Downgrade — §3.3.** |
| 31 | ULA spheroid formation | `composite_block` | ✗ (1 model class) | ✓ | ✗ | ✓ | **Downgrade — §3.4.** |
| 32 | 3D partial media exchange | `shared_operation` variant of #4 (lossy) | ✓ | ✓ | ✓ | ✓ | Covered by #5; do not create a "3D wash" op. |
| 33 | Luminescence read (e.g., ATP-glow) | `method_local_step` (gap) | ✗ (1 family documented) | — | — | — | Gap, §7. Do not shoehorn into #8. |
| 34 | Gap-closure metrics, migrated-cell counts, dose–response fits, ΔCq/ΔΔCq, dilution/CFU calculations, plate-capacity checks | `tool_action` | — | — | — | — | Deterministic derivations; never operations. |

## 3. Rejected / downgraded list (explicit)

### 3.1 `CAND-GAP-CREATE` — downgrade to `method_local_step` (never shared)

Two proposed variants have incompatible effects: scratch (tip-driven destruction, debris, lossy) vs insert-removal (barrier lift, no kill, no debris). Sharing one public op would hide the effect difference behind a variant flag — G2 and G4 fail; reuse is one assay family (wound/migration) — G1 fails. Keep each variant as a step inside `MT-SCRATCH-WOUND` / `MT-INSERT-GAP` respectively. Gap closure, proliferation blocking (mitomycin C), and closure metrics stay in the method template / tooling.

### 3.2 `CAND-APICAL-CLEAR` — downgrade to `method_local_step`

Documented in exactly one method (Transwell migration). Selective removal of one membrane face preserving the other cannot be expressed by washing, but that proves method locality, not sharedness: G1 fails (no second independent family), G3 fails (no reuse benefit beyond Transwell). Keep inside `MT-TRANSWELL-MIGRATION`.

### 3.3 3D matrix dome (and embedding generally) — downgrade to `composite_block` + Custom Step

Dome vs embedded vs sandwich inputs, confirmations, and failure modes differ by matrix, concentration, and geometry; gelation completion cannot be asserted by incubation time. Fails G1 (one model class), G2 (unstable boundary), G4 (branch cost = whole new op per matrix). The 3D record (`three-d-and-organoid-operations.md` R1) already rules this way: parameterized liquid transfer (cold) + Custom Step (dome dispensing) + parameterized incubation (polymerization).

### 3.4 ULA spheroid formation — downgrade to `composite_block`

Formation is not transfer, not incubation, and not countable — it is an aggregation outcome with a daily-microscopy judgment gate and branch-on-failure. It decomposes into liquid transfer (seeding) + incubation (hold) + optional centrifugation + human formation call. Promoting "spheroid formation" would enshrine a cell-line-specific outcome as a general operation. Fails G1 and G3.

## 4. Improper-merging checks

### 4.1 Streak vs spread — keep separate; both are public, narrow operations

Both are atomic (one spatial-distribution act, not split per stroke/motion). They answer different questions: streak isolates pure colonies (spatial separation); spread enumerates (fixed-volume surface distribution into a dilution series). Merging them into "inoculation" or "surface plating" would lose the measurand. Spread has ≥2 enumeration families. Streak is narrower (microbial isolation/purity); it is promoted as an explicit narrow exception, not because it is interchangeable with spread. They must not share one public definition.

### 4.2 Count vs viability — count promotes; viability is not an operation

A hemocytometer/automated count returns a concentration readout (atomic, reusable across seeding, passaging, seeding-density gates). Viability % requires a dye/method (trypan blue vs LIVE/DEAD vs CCK-8 dehydrogenase), thresholds, and controls — the measurand changes with the method. Merging "cell counting/viability" into one public op (as in the initial candidates) violates G2 — freeze §3 splits them, and this audit concurs: viability calls are `tool_action` derivations inside method templates (CCK-8, LIVE/DEAD, IF), never a shared operation.

### 4.3 Stain vs incubation — never merge

Incubation is a controlled hold guaranteeing only elapsed time and conditions; staining is a chemical binding/labeling effect with dye-, concentration-, and time-specific consequences. A stain step that "incubates" in a dye is still a stain (effect = labeling/binding, readout potential), not a hold. Merging them would let a method silently claim a generic hold where a labeling result is needed. Generic "stain" also fails G2: CV total-biomass dye, DAPI nucleic stain, and antibody immunolabeling have different effects and controls → `method_local_step`, each owned by its method template.

### 4.4 Other merges refused

- Washing vs dissociation: pipetting/blowing that mechanically disperses cells is a different intent from liquid exchange — dissociation stays method-local, never a washing parameter.
- Washing (preserving) vs lossy wash: promoting only the preserving default would hide biofilm/3D detachment loss. The lossy form is an explicit variant with declared loss, not a parameter tweak.
- Absorbance vs luminescence: different physics, instruments, and linearity claims. No shared "plate read" op.
- Post-centrifugation supernatant removal vs centrifugation: separation and taking are distinct intents; the take is a reuse of transfer/aliquot, not a new public op.

## 5. First public Catalog set (small)

Promote these 11 definitions to the public Catalog (as `research_draft` records first; second-person `ReviewRecord` required before JSON/runtime). Each item lists two document-level Reuse Map examples. Example entries map experiment steps to the public object; they are **document-level** and imply no source verification.

| # | Public operation | Reuse Map example A | Reuse Map example B |
|---|---|---|---|
| 1 | Liquid transfer | Cell seeding: suspension → well (bind volume, vessel) | Reagent addition in ELISA/wash cycles (bind reagent, destination) |
| 2 | Sample aliquot (conditional, §6.1) | Eluate/cDNA split in qPCR (parent retained + child aliquot) | Culture-supernatant sample for assay (parent retained) |
| 3 | Incubation / hold | Post-seeding growth hold (bind T/CO₂/duration) | Dye-development hold in CCK-8 (elapsed only; no growth claim) |
| 4 | Washing (preserving) | Post-scratch debris rinse (gentle, bind liquid/cycles) | Pre-stain rinse in IF workflow (bind liquid, vigor) |
| 5 | Washing, lossy variant | Biofilm pre-stain wash (discards planktonic + detachment loss declared) | 3D partial media exchange (retained sphere + residual volume bound) |
| 6 | Centrifugation | Low-speed cell pelleting before count (bind RCF/time/rotor) | Pellet-wash in Transwell harvest (bind RCF/time) |
| 7 | Absorbance read (raw OD) | CCK-8 plate read 450 nm + reference (raw matrix only) | Biofilm-CV solubilizate read 550 nm (raw OD only; biomass derivation downstream) |
| 8 | Fluorescence image acquisition (raw) | LIVE/DEAD stained sample, per-channel frames (raw images) | Fixed IF preparation, per-channel frames (terminal; no gating) |
| 9 | Spread inoculation | Spread-plate enumeration into countable dilution series | Biofilm-CFU plating of dispersed suspension |
| 10 | Cell concentration count | Pre-seed count gate for passaging (count + dilution) | Count-adjust of Transwell seeding suspension to target density |
| 11 | Streak inoculation (narrow) | Microbial isolation from a mixed/contaminated sample | Purity/subculture check before a downstream assay |

Explicitly **not** in the first set: colony count (promotable, held out per §7 G-2). Streak is included as a narrow, separate operation; it is not merged with spread.

## 6. Exceptions and conditions

### 6.1 Sample aliquot — conditional promotion

The independent review argued for downgrading aliquot into liquid transfer (lineage/quantity conservation unmodeled). This audit keeps it in the first set **conditionally**: its distinct purpose (parent retained, child sample created) is real and reused across qPCR, culture, and assay sampling, but the Catalog entry must carry an explicit note that parent-remaining + Σ aliquots conservation and child ID generation are **not** contract-expressed (record-fields/notes only; see scenario §"cannot express" 1–2). If the second-person reviewer does not accept the conditional note, aliquot drops to `method_local_step` without blocking the other 9.

### 6.2 Variants that must be declared, not parameterized away

- Lossy wash (biofilm, 3D exchange): variant with `loss` note, loss magnitude UNKNOWN. Never a parameter of preserving wash.
- Imaging modality (phase/brightfield vs fluorescence): parameter binding on the same raw-acquisition op; fixed-vs-live sample terminality note required.

### 6.3 Confirmation downgrade accepted

Incubation completes on elapsed/deviation evidence only; attachment, growth, gelation, or spheroid tightness are method preconditions/results, never incubation assertions. Same rule applies to any promoted op whose completion check would infer biology.

## 7. Gaps (not in the Catalog, with the reason)

| ID | Gap | Status |
|---|---|---|
| G-1 | Luminescence (ATP-glow) raw read | Missing atomic readout; do not reuse absorbance op. Needs its own record before any promotion. |
| G-2 | Colony-count cross-family evidence | Promotable in principle; held out of the first set until a second family beyond enumeration is documented. |
| G-3 | Dissociation chemistry | Method-local; no stable cross-method boundary (enzyme, sonication, scraping differ). |
| G-4 | Lineage / quantity conservation, conditional execution, typed `loss` effect, time-anchored II | Contract gaps (scenario §"cannot express"); remain notes/Custom Steps, not new fields. |
| G-5 | 3D/organoid specifics (passaging/splitting, hanging drop, non-Matrigel scaffolds, real-time viability) | `source_missing`/`method_dependent`; Custom Step until a lab protocol binds them. |
| G-6 | Invasion (ECM Transwell) | Existence note only; needs its own method record before any step is planned. |

## 8. Verification statement

This audit uses only repository documents listed in §Scope. No external protocol was opened for this audit; no numeric value is asserted; no `SourceChecked` / `ExpertReviewed` / `LabValidated` claim is made. Every reuse statement is **document-level** (an existing Reuse Map proposes reuse; it does not verify the reused step against its protocol). No code, UI, database, or JSON was changed.
