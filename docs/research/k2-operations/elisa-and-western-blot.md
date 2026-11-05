# ELISA and Western Blot — Method Records

- Date: 2026-11-05
- `review_status: research_draft`
- Scope: two separate method records (`METHOD-ELISA`, `METHOD-WB`). Each records method-specific steps, parameter bindings, controls, and limits, and references generic operation modules instead of redefining them.
- Generic operations referenced (not duplicated): liquid transfer / pipetting, `wash`, `incubation` (timed hold), terminal readout (`ABS_READ`-class absorbance; luminescence / fluorescence readout). See `liquid-transfer.md`, `washing-and-sampling.md`, `inoculation-and-incubation.md`, `absorbance-and-fluorescence-readout.md`.
- Rule respected: sandwich / direct / competitive ELISA are recorded as separate variants, not one universal SOP. Chemiluminescent / fluorescent (/ colorimetric) Western detection are recorded as separate variants, not one universal SOP.
- No value in this file is a concentration/amount claim from raw signal. OD / RLU / RFU / band intensity are raw; any conversion to concentration/amount is a separate downstream derivation requiring same-plate standards and curve fit, out of scope here.

## Relation to generic operations (read first)

| Method step | Generic module it binds | What the method record adds |
|---|---|---|
| Any reagent addition to wells / membranes | liquid transfer | assay-specific reagent, volume, buffer, order |
| Any wash between steps | `wash` (sample identity conserved, liquid → waste) | assay-specific cycles, buffer, soak, no-dry rule |
| Any timed hold (coat, block, bind, develop) | `incubation` (timed hold) | assay-specific duration, temperature, shaking, light protection |
| Plate signal acquisition | absorbance / luminescence / fluorescence readout | assay-specific wavelength/mode, plate colour, read window |
| Membrane signal acquisition | imaging readout (CCD / film / fluorescence imager) | variant-specific substrate, exposure, channel |

This file does not redefine transfer/wash/incubation/readout mechanics.

---

# Method Record 1 — ELISA

## 1.1 Identity

| Field | Value |
|---|---|
| Catalog id | `METHOD-ELISA` |
| Version | `v1` (`research_draft`) |
| Title | Enzyme-linked immunosorbent assay — sandwich, direct/indirect, competitive variants |
| `action_type` | method template (binds liquid-transfer + wash + incubation + readout); not a single executable op |
| Scope | 96-well polystyrene plate immunoassay: coat/capture → block → sample bind → detect → enzyme/substrate develop → stop (colorimetric only) → read |
| Exclusions | Sample preparation/lysis; antibody manufacturing/QC; standard-curve fitting math (downstream); ELISPOT; in-cell ELISA; lateral flow; kit lot release |

## 1.2 Purpose

Detect the presence of a soluble target (peptide/protein/antibody/hormone) immobilised in microplate wells via specific antibody–antigen binding coupled to an enzyme (or fluorescent tag), producing a measurable product whose raw signal is proportional to captured antigen under controlled conditions. What changes: soluble analyte → surface-bound immune complex → developed soluble product + raw optical signal. No biology interpretation (no concentration call) happens in this record.

## 1.3 Operation chain by variant (references generic modules)

Core chain (all variants): `coat → [wash] → block → sample/standard bind (incubation) → wash → detection → wash → [conjugate → wash] → substrate develop → [stop] → readout`.

| Variant | Capture | Detection path | Signal direction | Source locator |
|---|---|---|---|---|
| Sandwich (most used; highest sensitivity/specificity per E1) | Capture antibody coated on well; antigen bound between capture and detection antibodies recognising two different non-overlapping epitopes | Direct: HRP-conjugated detection Ab. Indirect: biotinylated detection Ab + streptavidin-HRP, or unlabelled detection Ab + enzyme-labelled secondary | Signal increases with antigen | E1 "What are the common ELISA formats"; E2 sandwich procedures |
| Direct (antigen immobilisation) | No capture antibody; antigen-containing sample coated directly in coating buffer | Labelled primary (or same detection + conjugate + substrate chain as sandwich) | Signal increases with antigen | E2 "Performing direct antigen immobilization"; E1 direct-vs-indirect table |
| Indirect | Antigen coated directly (same as direct capture) | Unlabelled primary + enzyme-labelled secondary (or biotin–streptavidin); signal amplified vs direct | Signal increases with antigen | E1 "indirect ELISA … antigen–primary complex followed by enzyme-linked secondary"; E2 "Using an enzyme-labeled secondary antibody" |
| Competitive / inhibition | Limited antibody on plate; sample (unlabelled) antigen competes with labelled reference antigen | Labelled antigen or labelled antibody; measured signal is interference | Signal **decreases** as sample antigen increases | E1 "Competitive ELISA and other formats" (purified labelled antigen competes with sample; decrease vs labelled-alone wells) |

Variant-specific notes (do not merge):
- Sandwich requires a validated "matched pair" (capture + detection bind simultaneously without interference); using the same antibody for capture and detection limits dynamic range/sensitivity (E1 "Primary antibodies for ELISA").
- Indirect detection: secondary must be specific for the detection primary only, not the capture antibody; achieved by different host species (e.g. mouse IgG + rabbit IgG) and cross-adsorbed secondaries (E1 "direct vs indirect" section).
- Competitive is the stated strategy when the antigen is small / single-epitope (E1); plate is NOT coated with excess capture protein "with the exception of competition ELISAs" (E1 "ELISA plates").
- Direct detection is quick / no secondary cross-reactivity but minimal amplification, labelling may harm immunoreactivity, few commercial conjugates (E1 comparison table). Indirect is versatile / amplified but adds a step and secondary cross-reactivity risk (E1 table).

## 1.4 Ports

| Port | Object type | Role | Required | Cardinality |
|---|---|---|---|---|
| `plate_in` | 96-well (or 384-well) polystyrene assay plate, not tissue-culture treated | consumable vessel | yes | 1 per run |
| `capture_ab_in` / `antigen_coat_in` | coating solution (variant-dependent) | reagent | yes (one of the two; sandwich/competitive use capture Ab; direct uses antigen coat) | 1 lot per run |
| `samples_in` | biological fluid / culture medium / lysate dilutions | tracked samples | yes | 1..N wells |
| `standards_in` | recombinant standard dilution series | calibrators (downstream fit only) | yes for any concentration claim; recorded but not fitted here | series per plate |
| `detection_ab_in`, `conjugate_in`, `substrate_in`, `stop_in` | assay reagents | reagents | variant-dependent (see §1.7) | 1 lot each per run |
| `plate_out` | developed plate (post-stop or post-substrate) | preserved vessel, post-reagent state | yes | 1 |
| `data_out` | raw well→signal matrix + wavelength/mode + timestamps + instrument + blank-well IDs | raw data | yes | 1 set per read |

## 1.5 Sample transitions

| Input → Output | Effect | Condition | Notes |
|---|---|---|---|
| `plate_empty → plate_coated` | coat (passive adsorption via hydrophobic interactions) | coating buffer, time/temp per §1.7 | Coated plates may be used immediately or dried/stored at 4 °C depending on coated-protein stability (E1) |
| `plate_coated → plate_blocked` | block remaining surface sites | blocking buffer 1 h RT (E2); blocker chosen empirically for best signal/noise (E1) | Inadequate blocker → background; excess blocker may mask epitopes / inhibit enzyme (E1) |
| `samples + plate_blocked → plate_bound` | affinity capture of analyte | sample incubation per §1.7 | Unbound material removed by subsequent wash; not a purification claim |
| `plate_bound → plate_developed` | enzyme/substrate product formed | substrate + time/temp/light per §1.7 | Overdevelopment → green (TMB); chemiluminescent signal decays after substrate exhaustion (E1) |
| `plate_developed → plate_out + data_out` | terminal optical readout, non-consumptive optically but post-reagent chemically | read window per §1.7 | Re-entry to culture or to pre-reagent state is NOT a transition offered |

No `consumed/preserved` global boolean is used; transitions above carry identity.

## 1.6 Parameters (assay-specific bindings only; nothing invented)

Coating (E1 "ELISA plates"; E2 procedures):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `coat_protein_conc` | number, µg/mL | E1 | plan + run | 2–10 µg/mL protein in alkaline buffer as common starting point (E1); fluorescent E2 procedure narrows to 5–10 µg/mL — keep both, do not average; per-protein optimisation required (E1) |
| `coat_buffer` | enum (PBS pH 7.4 / carbonate-bicarbonate pH 9.4) | E1; E2 materials | plan + run | 50 mM carbonate pH 9.4 or 10 mM phosphate pH 7.4 (E2) |
| `coat_volume` | number, µL/well | E2 | run | 100 µL/well (colorimetric/chemiluminescent); 50–100 µL/well fluorescent (E2) |
| `coat_time_temp` | string | E2 | run | 1 h RT or overnight 12–18 h at 2–8 °C (E2); "several hours to overnight at 4–37 °C" general range (E1) — record actual |
| `plate_type` | enum | E1; E2 materials | plan + run | Clear polystyrene flat-bottom for colorimetric; white opaque (chemi; black also allowed, white higher signal) / black (fluorescent); min binding capacity 400 ng/cm², CV < 5% preferred; inspect for scratches (E1). Pre-coated affinity options (Protein A/G, streptavidin, GST/His) are variants, not defaults (E1) |

Blocking / washing (E1 "Blocking buffers and wash buffers"; E2 procedures; E3 troubleshooting):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `block_buffer` | string (lot + product) | E2 | run | e.g. SuperBlock (TBS/PBS) or StartingBlock T20 (TBS/PBS) per E2 procedure used; ideal blocker binds all nonspecific sites without obscuring epitopes — must be tested, no universal blocker (E1) |
| `block_volume_time` | number + string | E2 | run | 300 µL/well, 1 h RT (all three E2 procedures) |
| `wash_buffer` | string | E2; E1 | run | TBS or PBS + 0.05% Tween-20 (E2); high-purity detergents (E1) |
| `wash_cycles` | integer + volume | E2; E3 | run | 1× post-coat (>300 µL/well, tap dry); 5× (>300 µL/well) post-sample, post-detection, post-conjugate (E2). Generic wash procedure: ≥400 µL fill, 15–30 s soak, aspirate without scratching, tap dry; usually 3–4× (E3 tips) — record actual cycles used |
| `plate_sealer` | boolean | E3 | run | Fresh sealer each opening; prevents cross-contamination, evaporation, edge effects (E3) |

Sample / detection incubations (E2):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `sample_volume` | number, µL/well | E2 | run | 100 µL/well standards (in duplicate) + samples (E2) |
| `sample_incubation` | duration + temp + shaking | E2 | run | 1–2 h RT with gentle shaking ~500 rpm (all E2) |
| `detection_ab` + `detection_incubation` | string (lot, dilution) + duration | E2 | run | Dilution per manufacturer; colorimetric/chemi: add 100 µL/well, 2 h RT ~500 rpm. Fluorescent E2 narrows: 0.05–0.1 µg/mL in wash buffer, 1–2 h RT ~500 rpm; fluorophore-conjugated direct alternative 2–4 µg/mL, read directly after wash (E2). Starting ranges by antibody source in E2 dilution table (polyclonal serum/crude ascites 5–15 coat / 1–10 detect; affinity-purified mono/poly 1–12 coat / 0.5–5 detect µg/mL) — optimisation required |
| `streptavidin_hrp` + `conjugate_incubation` | dilution + duration | E2 | run (conditional: only if detection is biotinylated) | 1:5,000 colorimetric (2 µL + 9.998 mL per plate); 1:5,000–1:20,000 chemiluminescent (optimise empirically); fluorescent 0.05–0.1 µg/mL, 50–100 µL/well. 1 h RT ~500 rpm (E2) |
| `secondary_enzyme_conjugate` | string (conditional) | E2 | run (conditional: only if non-biotinylated detection + labelled secondary path) | Slightly less amplification than biotin–streptavidin; may need higher conjugate concentration (E2). Exact factor UNKNOWN — titrate |

Substrate / stop / read (E1 detection strategies; E2 procedures; sibling readout record for TMB stability):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `substrate_system` | enum (TMB/HRP colorimetric; luminol/HRP chemiluminescent; QuantaBlu fluorogenic; PNPP/AP) | E1; E2 | plan + run | HRP: TMB/OPD/ABTS; AP: PNPP. Chemi HRP: luminol + peroxide. Fluoro: QuantaRed/QuantaBlu. Choice sets plate colour + reader (E1 table) |
| `substrate_volume_time` | number + duration | E2 | run | TMB: 100 µL/well, 30 min RT or until desired colour. Chemi: 100 µL/well 1:1 luminol:peroxide, 1 min RT. QuantaBlu: 9:1 substrate:peroxide, 100 µL/well, 1.5–90 min RT or 37 °C (E2) |
| `stop_solution` | string + volume | E2 | run (colorimetric only) | TMB: 100 µL 0.16 M H₂SO₄ (N600) (E2 colorimetric). AP/PNPP: 50 µL 2 N NaOH (E2). Kit-lot molarity varies — record lot (sibling record notes 2 M vs 0.16 M variants) |
| `read_wavelength_mode` | number/mode | E2; E1 | run | TMB stopped: 450 nm within 30 min of stop (E2; sibling TMB UG notes 60-min stability — enforce the stricter 30-min plan clock, record actual delay). PNPP: 405 nm after 15–30 min RT. Chemi: RLU (~425 nm) on luminometer between 1–5 min after substrate (later reads fade). QuantaBlu: RFU ex 325 nm / em 420 nm (usable 315–340 / 370–470) (E2) |
| `standard_curve_fit` | downstream method reference | E2 | plan (conditional) | Log-log or 4-parameter fit over same-plate standard series (E2 "Calculate results"). Fitting itself is out of scope; without it no concentration claim |

## 1.7 Time scope

- `coat`: once per plate (or per batch if pre-coated/stored).
- `block`, `sample bind`, `detection`, `conjugate`, `substrate develop`: once per assay run, sequential holds; each is an independent timed node (do not collapse into one "ELISA incubation").
- `readout`: terminal instrument time only (minutes); development clocks above are upstream holds.
- Stability windows are plan constraints: stopped-TMB read ≤30 min (E2); chemiluminescent read 1–5 min (E2).

## 1.8 Preconditions

- Correct plate chemistry for the signal (clear vs white/black opaque) and undamaged wells (E1).
- Reagents at RT at start (15–20 min bench equilibration); kit stored 2–8 °C, unexpired (E3).
- Matched antibody pair validated for sandwich; host-species / cross-adsorption plan for indirect (E1).
- Blank wells + standard series on the same plate for any quantitation (E2; E3 standard-curve rows).
- Reader set to the substrate's wavelength/mode (E3).

## 1.9 Completion checks

| Check | Level | Blocking? |
|---|---|---|
| Every addressed well returned a value in instrument range; no error flag | deterministic | yes — block downstream fit on failure |
| Wavelength/mode, plate type, timestamps, blank-well IDs recorded in `data_out` | deterministic | yes |
| Wash completeness: no residual liquid (tap-dry), no scratched wells (E2/E3) | operational | yes — flag affected wells |
| Signal plausibility (not saturated/clipped; stopped-TMB not green/overdeveloped) | operational | no — flag, do not auto-rescue by re-scaling |
| Incubation times/temps/shaking recorded per hold | operational | no |

## 1.10 Instrument requirements (capability, no model binding)

- Absorbance reader covering 450 nm (TMB) / 405 nm (PNPP) for colorimetric; luminometer (~425 nm RLU) for chemiluminescent; fluorometer with correct excitation/emission filters for fluorescent/chemifluorescent (E1 table; E2 materials).
- Plate capacity: 96-well (384-well per E1 platform note); shaking ~500 rpm capability where E2 shaking is planned — record actual capability, do not assume.
- Multiskan SkyHigh-class absorbance-only readers MUST NOT be planned for luminescence/fluorescence reads (see sibling readout record).

## 1.11 Controls (assay companions; readout alone proves nothing)

| Control | Why (sourced) |
|---|---|
| Same-plate standard series in duplicate (quantitation only) | Curve fit needs it; poor dilutions → poor curve (E2/E3) |
| Blank (substrate + stop, no sample) + negative control (no target) | Defines background / nonspecific binding; positive control (known amount) confirms reagents work (E1 FAQs; E3) |
| Vehicle / matrix control | Sample matrix and blocker/detergent alter background (E1 blocker discussion) |
| Secondary-only reasoning for indirect path | Secondary cross-reactivity is the known indirect failure (E1 table) — full-stain vs secondary-only comparison required before specificity claims |
| Replicate wells (duplicates minimum, E2) | Poor replicates trace to washing/coating/sealer failures (E3) |

## 1.12 Readout meaning — what the numbers are and are NOT

- A stopped-TMB OD at 450 nm is stopped-chromogen amount under that development time — NOT analyte concentration. A luminometer RLU / fluorometer RFU is photons under that substrate/exposure — NOT amount. Proportionality to captured antigen holds only when the assay was constructed and developed properly (E1), and conversion requires a same-plate standard series fitted downstream (E2).
- Competitive direction is inverted: lower signal = more sample antigen vs labelled-alone wells (E1). Do not apply sandwich (increasing) interpretation to competitive plates.
- An OD without blank/standard/controls is background plus signal; high background, edge gradients, and overdevelopment are documented confounders (E3), not biology.

## 1.13 Failure modes

| # | Phenomenon | Cause / effect | Handling / record |
|---|---|---|---|
| F-E1 | Weak or no signal | Cold reagents; wrong storage/expired; wrong order/dilution; capture Ab never bound (wrong plate — tissue-culture instead of ELISA; wrong buffer/time); too little detector Ab; scratched wells; wrong read wavelength (E3) | Verify RT equilibration, storage, dilutions, plate chemistry, wavelength; re-run; record lot/expiry/actual conditions |
| F-E2 | Too much signal / saturated | Insufficient washing; no/reused sealer (cross-contamination); wrong dilutions; over-incubation (E3) | Enforce wash + fresh-sealer + clock discipline; re-run at adjusted load |
| F-E3 | High background | Insufficient washing (+30 s soak increments); light-exposed substrate; over-incubation; bad standard dilutions (E3). Blocker too lean → background; too rich → masks epitopes/inhibits enzyme (E1) | Re-titrate blocker; protect substrate from light; record wash/soak actually done |
| F-E4 | Poor standard curve | Dilution/pipetting error; capture failed to bind (E3) | Check calculations + technique; verify coating step; re-run curve |
| F-E5 | Poor replicates / assay-to-assay drift | Washing variance; coating variance; sealer reuse; incubation temperature fluctuation (E3) | Replicates minimum duplicates; fresh sealer; centred/unstacked plate; calibrated washer tips not touching well bottoms (E3) |
| F-E6 | Edge effects (outer wells high) | Uneven temperature / evaporation / stacked plates (E3) | Seal; centre plate; never stack; record incubator position |
| F-E7 | Coating "hooking" (trapped detection, high background, compressed range) | Coating above optimal density traps proteins and blocks effective washing (E1) | Lower coating concentration experimentally; do not rescue by boosting detection |
| F-E8 | Chemiluminescent fade across plates | Signal decays once substrate exhausts; late plates read low and mimic low abundance (E1) | Fix read order/timing (1–5 min); optimise substrate; record substrate-to-read delay per plate |
| F-E9 | High-dose analyte underestimation ("hook/prozone"-type) | `search-summary-only` (Bio-Rad ELISA hints; Thermo D19554/TR0065 summaries): excess analyte saturates antibodies, signal falls | Mitigation per summaries (multiple dilutions, linear-range check) is NOT verified here — treat as UNKNOWN procedure; record dilutions run and flag non-monotonic curves for second-person review |

## 1.14 UNKNOWNs (ELISA)

| Item | Status | Reason / who confirms |
|---|---|---|
| Per-kit/per-lot antibody concentrations, standard concentrations, stop molarity, substrate brand kinetics | UNKNOWN (`method_dependent`, `configuration_dependent`) | Kit insert + lot datasheet; record lot, do not default |
| Optimal blocker/detergent per sample–antibody combination | UNKNOWN (`method_dependent`) | Empirical signal/noise test required (E1); lab SOP confirms |
| AP-system exact kinetics beyond PNPP 15–30 min / 405 nm (E2) | UNKNOWN (`source_missing`) | AP substrate user guide; second-person read |
| Kinetic/continuous read mode; exact washer pressures/volumes; incubator uniformity | UNKNOWN (`source_missing`/`device_state`) | Instrument manuals + build sheet; see sibling readout UNKNOWNs |
| True high-dose hook dilution protocol for a given kit | UNKNOWN (`method_dependent`) | Kit validation data + lab titration; F-E9 summaries not sufficient |

---

# Method Record 2 — Western Blot

## 2.1 Identity

| Field | Value |
|---|---|
| Catalog id | `METHOD-WB` |
| Version | `v1` (`research_draft`) |
| Title | Western blot immunodetection — colorimetric, chemiluminescent, fluorescent (incl. multiplex) variants |
| `action_type` | method template (binds transfer + wash + incubation + imaging readout); not a single executable op |
| Scope | Post-SDS-PAGE membrane immunodetection: transfer check → block → primary → wash → secondary/conjugate → wash → develop → image |
| Exclusions | Cell lysis / sample prep recipes; SDS-PAGE running conditions; transfer physics/buffer optimisation; stripping/reprobing recipes; total-protein normalisation math; band-quantitation fitting (downstream) |

## 2.2 Purpose

Identify specific proteins blotted from SDS-PAGE onto a membrane via antibody labelling and variant-specific detection chemistry (precipitate / emitted light / fluorophore emission). What changes: separated proteins on gel → membrane-bound proteins → antibody–enzyme/fluorophore complex → visible bands / digital image. No amount/expression claim happens in this record.

## 2.3 Operation chain by variant (references generic modules)

Shared upstream (variant-independent): `SDS-PAGE (out of scope, referenced) → transfer to membrane (per manufacturer) → transfer check (Ponceau S) → block (incubation) → primary (incubation) → wash → secondary/conjugate (incubation) → wash → develop → image`.

| Variant | Label + substrate/product | Capture | Key constraints | Source locator |
|---|---|---|---|---|
| Colorimetric (HRP: 4CN/DAB/Opti-4CN; AP: BCIP/NBT) | Enzyme converts substrate to coloured precipitate on the blot; reaction monitored by eye and stopped at desired signal/background | Eye or basic imager; stable product (AP/BCIP/NBT non-fading; HRP/4CN fades in light) | Medium sensitivity (table: HRP/4CN 500 pg; DAB 500 pg; Opti-4CN 100 pg; amplified Opti-4CN 5 pg; AP/BCIP/NBT 100 pg) | W1 "Colorimetric Detection" + sensitivity table |
| Chemiluminescent (HRP/luminol; AP/CDP-Star) | Enzyme–substrate reaction emits light; captured on film or CCD (e.g. ChemiDoc-class) | CCD imager or film; typical exposures seconds–minutes | Femtogram–low-pg sensitivity (table: HRP/luminol 1–3 pg; WesternC femtogram; AP/CDP-Star 10 pg); signal lasts hours (6–8 h / up to 24 h), working solution 24 h RT; azide inhibits HRP; blots reactivatable with fresh substrate | W1 "Chemiluminescence Detection" + table + Immun-Star notes |
| Fluorescent (incl. multiplex) | Primary or secondary labelled with fluorophore; excitation → emission captured by fluorescence-capable imager; no liquid substrate step for direct fluorophore path | Fluorescence imager with matched excitation/emission; distinct spectra per channel | Multiplex simultaneous targets; ~10× dynamic range vs chemiluminescence; blots re-imageable weeks/months; needs species-separated primaries + cross-absorbed secondaries + non-overlapping spectra | W1 "Fluorescence Detection" + multiplex figure |
| Chemifluorescence (adjacent, not merged) | Enzymatic conversion to fluorescent product; scanned later on fluorescence imager | Fluorescence imager | Stable product; compatible with stripping/reprobing | W1 "Other Detection Methods — Chemifluorescence" |

Do not merge: chemiluminescence is time-critical light emission (substrate exhaustion ends signal); fluorescence is stable re-imageable emission; colorimetric is an endpoint precipitate. Plate/reader, timing, and quantitation limits differ per variant.

## 2.4 Ports

| Port | Object type | Role | Required | Cardinality |
|---|---|---|---|---|
| `gel_in` | SDS-PAGE gel with separated proteins | input vessel | yes | 1 |
| `membrane_in` | nitrocellulose or PVDF membrane | consumable vessel | yes | 1 per blot |
| `marker_in` | molecular-weight / WesternC standards | calibrators for position (not amount) | yes | 1 lane minimum |
| `primary_ab_in`, `secondary_conjugate_in`, `substrate_in` | antibody / conjugate / substrate lots | reagents | yes | 1 lot each per run |
| `membrane_out` | developed membrane (precipitate / reacted / labelled) | preserved consumable | yes | 1 |
| `image_out` | band image set + channel/exposure metadata + instrument ID | raw data | yes | 1+ frames per blot |

## 2.5 Sample transitions

| Input → Output | Effect | Condition | Notes |
|---|---|---|---|
| `gel_loaded → membrane_blotted` | transfer proteins gel→membrane | per manufacturer instructions (W2 step 1) | Transfer apparatus/buffer/time are build- and protocol-dependent — UNKNOWN here |
| `membrane_blotted → membrane_qc` | reversible total-protein stain check | Ponceau S 1 min, destain with water (W2 step 2); light-sensitive reagent | QC only; complete destain before blocking |
| `membrane_qc → membrane_blocked` | occupy unbound sites | blocking solution 2 h RT or overnight 4 °C (W2 step 3) | Blocker choice membrane- and target-dependent; phospho-proteins follow datasheet (W2 reagents note) |
| `membrane_blocked → membrane_probed` | primary + secondary/conjugate binding with interleaved washes | primary 2 h RT or overnight 4 °C; 3×10 min wash; secondary 1 h RT; 4×5 min wash buffer + 3×5 min PBST + 2×5 min PBS (W2 steps 4–7) | Dilutions in wash buffer; gentle agitation throughout |
| `membrane_probed → membrane_out + image_out` | variant-specific develop + image | substrate per manufacturer; image before signal fades (chemi) or archivally (fluoro/color AP) | Membrane is terminal for the probed state; stripping/reprobing is a separate, unrecorded procedure |

## 2.6 Parameters (assay-specific bindings only; nothing invented)

Transfer check + block (W2 reagents + method):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `transfer_check` | stain + time | W2 | run | Ponceau S (0.1 g / 95 mL water / 5 mL acetic acid — W2 recipe), 1 min, destain with water; light-sensitive |
| `block_solution` | string (lot + formula) | W2 | run | Block Ace (BUF029) in water, or 5% non-fat dried milk in PBS. Block Ace preferred for cleaner blots; phospho-protein blocking per product datasheet (W2). BSA/gelatin alternatives per W1 blotting guide exist but exact formulations here are UNKNOWN |
| `block_time_temp` | duration + temp | W2 | run | 2 h RT or overnight 4 °C |
| `pbs_pbst_recipe` | string | W2 | run | PBS and PBST (PBS + 1.0 mL/L Tween-20) recipes transcribed in W2; use lot-recorded buffer |

Antibodies + washes (W2 steps 4–7):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `primary_ab` + `primary_incubation` | dilution (µg/mL or ratio) + duration/temp | W2 | run | Diluted in wash buffer; generally 1–10 µg/mL starting point, precise per datasheet; 2 h RT or overnight 4 °C with gentle agitation. Exact clone/host/dilution UNKNOWN until lot sheet |
| `primary_wash` | cycles × duration | W2 | run | 3×10 min wash buffer, gentle agitation |
| `secondary_conjugate` + `secondary_incubation` | dilution + duration/temp | W2 | run | Enzyme-conjugated secondary in wash buffer; 1 h RT with agitation. TidyBlot-type alternative 1/40–1/400 with titration is `search-summary-only` — do not plan from it |
| `final_wash` | cycles × duration × buffer | W2 | run | 4×5 min wash buffer; 3×5 min PBST; 2×5 min PBS, gentle agitation |
| `no_primary_control` | boolean (lane present) | W2 | plan + run | Include a no-primary sample to assess secondary nonspecific binding (W2 step 8 note) |

Develop + image (W1 detection page; W2 step 8):

| Key | Type / unit | Source | `required_at` | Constraint / notes |
|---|---|---|---|---|
| `substrate_system` | enum (HRP-4CN/DAB/Opti-4CN; AP-BCIP/NBT; HRP-luminol/WesternC; AP-CDP-Star; fluorophore label) | W1 | plan + run | Sensitivity table values in §2.3 are per W1 table; per-sample sensitivity varies with protein/antibody/substrate affinity (W1) |
| `develop_time` | duration | W1; W2 | run | Colorimetric: monitor and stop at desired signal/background (W1). Chemi CCD: typically 5 s–5 min (W1); example WesternC 5 min substrate + 5 s image (W1 figure). Exact exposure UNKNOWN until titrated |
| `imager_mode` | enum (eye / film / CCD / fluorescence channels) | W1 | plan + run | Colorimetric eye/basic capture; chemi film or CCD; fluorescent needs matched light source + filters + camera (quantum yield / extinction / Stokes / spectra per W1). Channel/exposure recorded per frame |
| `multiplex_set` | array of {target, host species, fluorophore, ex/em} | W1 | plan + run (multiplex only) | Different host primaries + cross-absorbed secondaries + spectrally distinct fluorophores to avoid cross-reactivity and crosstalk (W1). Exact dyes UNKNOWN until imager/antibody sheets |

## 2.7 Time scope

- `transfer`, `block`, `primary`, `secondary`, `develop` are each once-per-blot timed holds with their own windows (not one "WB incubation").
- `wash` cycles are per-step repeats (3×10 min; 4×5 + 3×5 + 2×5 min) — record actuals.
- `image` is terminal instrument time; chemiluminescent clock is substrate-to-capture delay (minutes; signal depletes), fluorescent/color-AP is archival.

## 2.8 Preconditions

- Transfer completed per manufacturer; Ponceau check passed and fully destained (W2).
- Correct membrane chemistry seated; phospho-target blocking confirmed against datasheet (W2).
- Primary/secondary host-species plan avoids cross-reactivity (esp. multiplex) (W1).
- Imager has the required mode: CCD/film for chemi; fluorescence excitation/emission paths for fluorescent (W1).
- No azide with HRP systems (W1 table disadvantage).

## 2.9 Completion checks

| Check | Level | Blocking? |
|---|---|---|
| Marker lane visible; every addressed lane has an image file with channel/exposure/instrument recorded | deterministic | yes |
| No-primary control lane present and recorded | operational | yes for specificity claims |
| Exposure not saturated-to-clip and not blank without operator note; chemi substrate-to-image delay recorded | operational | no — flag, re-expose per titration rather than rescaling |
| Wash sequence actually executed recorded (cycles × buffers) | operational | no — flag deviations |

## 2.10 Instrument requirements (capability, no model binding)

- Transfer apparatus compatible with the gel/membrane sizes used (capability exists; voltage/time/buffer UNKNOWN — do not plan from this record).
- CCD imager or film path for chemiluminescence; fluorescence-capable imager (matched excitation, emission filters, camera) for fluorescent/multiplex; basic capture for colorimetric (W1).
- Ponceau/light-protection handling (reagent light-sensitive, W2).

## 2.11 Controls (assay companions; image alone proves nothing)

| Control | Why (sourced) |
|---|---|
| Molecular-weight / WesternC standards lane | Position reference; WesternC co-detected on chemi CCD path (W1 figure) |
| No-primary-antibody sample | Determines secondary/conjugate nonspecific binding (W2) |
| Loading / total-protein reference (e.g. housekeeping or total-protein stain) | Single-band intensity is not load-normalised without it; normalisation math itself is downstream and out of scope |
| Positive / negative lysate where available | Confirms antibody works and signal is target-dependent (general W2 "appropriate controls"; exact lysates `method_dependent`) |
| Single-colour / unstained references for multiplex | Required to assign channels before any "positive" call (W1 crosstalk discussion) |

## 2.12 Readout meaning — what the bands/images are and are NOT

- A band is detected label at (apparent MW, lane) under that block/antibody/wash/substrate/exposure — NOT protein amount, NOT expression level, NOT modification stoichiometry.
- Chemiluminescent intensity is light emitted during the enzyme reaction only; it ends when substrate exhausts, so intensity depends on timing as well as protein (W1). Fluorescence has wider linear range and archival stability, but channel assignment needs crosstalk controls (W1). Colorimetric precipitate is an endpoint stopped by eye (W1) — NOT a linear quantity.
- Cross-blot comparisons (chemi vs fluoro vs colorimetric; different exposures/antibodies) are NOT commensurable inside this record: different measurands, sensitivities, and time windows.

## 2.13 Failure modes

| # | Phenomenon | Cause / effect | Handling / record |
|---|---|---|---|
| F-W1 | Blank blot / weak bands | Transfer failed (caught at Ponceau check, W2); antibody too dilute; expired/cold reagents; wrong secondary/host; azide-poisoned HRP; acidic inactivation of AP (W1/W2) | Verify Ponceau before proceeding; check datasheet dilutions, host match, azide-free buffers; record actual lots/dilutions |
| F-W2 | High background / nonspecific bands | Under-blocking or wrong blocker for membrane/target; insufficient washes; endogenous phosphatase with AP substrates → false positives (W1 table) | Re-optimise blocker; enforce full wash sequence; phospho/AP paths need datasheet-specific blocking |
| F-W3 | Saturated / blown-out bands with invisible weak bands | Overexposure or overdevelopment; chemiluminescence narrower linear range than fluorescence (W1) | Re-image at shorter exposure; record exposure series; do not clip-and-interpret |
| F-W4 | Faded record (HRP colorimetric) | 4CN/DAB product fades in light (W1 limitation) | Protect from light; AP/BCIP/NBT noted non-fading alternative (W1); record storage |
| F-W5 | Multiplex crosstalk / misassignment | Overlapping spectra; cross-reactive secondaries (W1) | Species-separated primaries + cross-absorbed secondaries + distinct spectra; single-colour controls |
| F-W6 | Signal decay before capture (chemi) | Delayed imaging after substrate exhaustion (W1) | Enforce substrate→capture clock; reactivate only with fresh substrate per kit note (W1) |
| F-W7 | Stripping/reprobing artefacts; transfer-buffer optimisation; lysis-dependent band patterns | Procedures not verified in sources read | UNKNOWN (`source_missing`/`method_dependent`) — do not plan; needs protocol-sheet + lab SOP confirmation |

## 2.14 UNKNOWNs (Western)

| Item | Status | Reason / who confirms |
|---|---|---|
| Transfer voltage/time/buffer/membrane choice (nitrocellulose vs PVDF); gel %/run conditions; RIPA/lysis recipe + inhibitors | UNKNOWN (`source_missing`, `method_dependent`) | Gel/transfer-system manuals + sample-prep SOP; W2 defers to manufacturer instructions |
| Per-lot antibody clone/host/dilution, conjugate dilution, substrate kinetics, exposure series | UNKNOWN (`method_dependent`) | Lot datasheets + titration; W2 gives only 1–10 µg/mL starting point |
| Phospho-protein blocking formulation | UNKNOWN (`method_dependent`) | Product datasheet + phospho protocol sheet (W2 pointer) |
| Stripping/reprobing, total-protein normalisation, housekeeping validation | UNKNOWN (`source_missing`) | Dedicated protocol + second-person review; not covered here |
| Film vs CCD calibration, fluorescence channel/bandpass part numbers, stage/imager specs | UNKNOWN (`configuration_dependent`) | Instrument profile + build sheet |

---

## 3. Sources, evidence status, retrieval

Retrieved 2026-11-05. "Read directly" = record author opened the page/PDF text in-session and transcribed; "search-summary-only" = seen only as a search-result summary, never used for a value.

| ID | Source | URL | Locator used | Read level |
|---|---|---|---|---|
| E1 | Thermo Fisher, "ELISA Assay Technique" (overview: formats, plates, antibodies, blockers, detection strategies, FAQs) | https://www.thermofisher.com/us/en/home/life-science/protein-biology/protein-biology-learning-center/protein-biology-resource-library/pierce-protein-methods/overview-elisa.html | Core steps; formats diagram + comparison tables; plates (2–10 µg/mL, pH 7.4/9.4, 400 ng/cm², CV <5%, clear vs black/white); matched-pair + hooking; blocker/wash guidance; detection table; positive/negative control FAQ | Read directly 2026-11-05 |
| E2 | Thermo Fisher, "Basic Sandwich ELISA Protocols" (colorimetric / chemiluminescent / fluorescent procedures + antibody dilution tables) | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/elisa-protocol/general-elisa-protocol/basic-sandwich-elisa-protocol.html | Materials + stepwise procedures (volumes, times, temps, shaking, wash cycles, substrate/stop, read modes/windows); AP/PNPP alternates; direct-immobilisation + labelled-secondary variants; dilution tables | Read directly 2026-11-05 |
| E3 | Thermo Fisher, "ELISA Troubleshooting Guide" + pipetting/washing tips | https://www.thermofisher.com/us/en/home/life-science/protein-biology/protein-biology-learning-center/protein-biology-resource-library/pierce-protein-methods/overview-elisa/elisa-troubleshooting-guide.html | Weak/no-signal, too-much-signal, background, curve, replicate, assay-variability, edge-effect tables; wash procedure (≥400 µL, 15–30 s soak, 3–4×, tap dry); sealer discipline | Read directly 2026-11-05 |
| W1 | Bio-Rad, "Detection Methods" (colorimetric / chemiluminescent / fluorescent / other mechanisms + sensitivity tables) | https://www.bio-rad.com/en-us/applications-technologies/detection-methods?ID=LUSQ6KKG4 | Mechanism figure; HRP/AP colorimetric substrates + pg table; chemi substrates + duration/exposure + table; fluorescence optics + multiplex/dynamic-range/stability; chemifluorescence/autoradiography/immunogold adjacency | Read directly 2026-11-05 |
| W2 | Bio-Rad Antibodies, "Western Blotting Protocol" (general procedure + reagent recipes) | https://www.bio-rad-antibodies.com/western-blot-protocol.html | Reagents (Block Ace vs 5% milk; phospho pointer; wash = block + 0.1% Tween; Ponceau/PBS/PBST recipes); steps 1–8 (transfer→Ponceau 1 min→block 2 h RT/overnight 4 °C→primary 1–10 µg/mL 2 h RT/overnight 4 °C→3×10 min wash→secondary 1 h RT→4×5/3×5/2×5 min washes→substrate); no-primary control note | Read directly 2026-11-05 |
| — | Bio-Rad "Helpful ELISA hints" (hook/edge); Thermo TR0065/D19554 app notes; Bio-Rad Bulletins 2895/6390/6570/3429/6166; TidyBlot protocol; phospho-PrecisionAb protocol | various (see session search output) | Referenced only to mark F-E9 and WB titration paths as unverified | Search summary only — NOT used for any value; do not cite as evidence |

Evidence labels used in §§1–2: values tagged E1/E2/E3/W1/W2 are `agent-verified` (transcribed from a directly-read source, still needing second-person review). Anything else is `UNKNOWN` with a reason, or explicitly `search-summary-only`. No `SourceChecked`, `ExpertReviewed`, or `LabValidated` claim is made.

Source conflicts retained (not averaged): stopped-TMB read window 30 min (E2) vs 60-min stability in the sibling readout record's TMB UG; fluorescent coat 5–10 µg/mL + 50–100 µL (E2-fluoro) vs general 2–10 µg/mL + 100 µL (E1/E2-colorimetric); wash 5× (E2 assay steps) vs 3–4× general tips (E3). In each case the plan records the procedure actually followed and the stricter stability clock.

## 4. What this record does not cover

- SDS-PAGE, transfer optimisation, lysis/RIPA, sample-prep matrices (WB upstream) and sample-matrix preparation (ELISA upstream).
- Standard-curve fitting (log-log / 4-parameter), blank subtraction, total-protein / loading normalisation, and any concentration/amount/expression derivation — all downstream derivations with their own controls.
- Stripping/reprobing, ELISPOT, in-cell ELISA, multiplex Luminex, autoradiography/immunogold as executable procedures (mentioned only as adjacent methods).
- Reusable-operation mechanics (transfer/wash/incubation/readout internals), instrument capabilities per build, JSON conversion (`OperationModule` / `InstrumentProfile`), K3 compilation, run-time state, or laboratory SOP adaptations.
- Per-kit/per-lot numbers (antibody concentrations, standard concentrations, substrate kinetics, exposure series) — intentionally UNKNOWN until lot sheets are read.

## 5. Review status and audit checklist

- Status: `research_draft`. No `ReviewRecord` exists; no second-person field-by-field check done. This file MUST NOT be treated as `SourceChecked` and MUST NOT enter runtime/Compatible use.
- Checklist against `record-standard.md` §7: every numerate carries a source row or an UNKNOWN tag (§§1.6/2.6/1.14/2.14); method requirements and instrument specs are separated (§§1.10/2.10 vs parameter tables); ports + transitions replace any global boolean (§§1.4–1.5/2.4–2.5); no Assay-name-derived time scope (§§1.7/2.7); no implied defaults (variant conditionals explicit); partial lists (plate options, substrate options) labelled as options; plan/actual/raw/derived kept apart (§§1.12/2.12); no AI pre-review presented as human review (§3).
