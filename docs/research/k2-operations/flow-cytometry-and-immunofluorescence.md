# Flow Cytometry & Immunofluorescence — Reusable Operation Records

`review_status: research_draft`

> Do NOT treat as `SourceChecked`. No second-person review done. No `ReviewRecord`
> exists. Numeric values below are transcribed only from sources marked
> `agent-verified` in §2; everything marked `search-summary-only` or `UNKNOWN`
> MUST NOT be used for planning until verified. No value is invented.

## 1. Identity

| Field | Value |
|---|---|
| Document | Reusable operation definitions for flow-cytometry sample prep + acquisition and immunofluorescence fixation / staining / imaging |
| Role | K2 operation contracts (planning + execution preconditions/checks), not instrument profiles |
| Records | `FLOW-PREP` (stain for flow, incl. viability) · `FLOW-ACQ-ANALYSIS` (analytical acquisition, no sorting) · `FLOW-SORT` (sorting acquisition, separate record) · `IF-FIXPERMBLOCK` (fix / perm / block) · `IF-STAIN` (primary / secondary / mount) · `IF-IMAGE-FIXED` (fixed-sample imaging only) |
| Scope | Cultured-cell / suspension inputs; surface + intracellular flow staining; fixed-cell IF on slides/chambers/dishes |
| Out of scope | See §11 (does-not-cover list) |

## 2. Sources (retrieved 2026-11-05)

| # | Source | URL | Locator / what was used | Read status |
|---|---|---|---|---|
| T1 | Thermo Fisher, "BestProtocols: Immunofluorescent Staining of Intracellular Antigens on Cultured Cells" | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/immunofluorescence-staining-intracellular-antigens-cultured-cells.html | Materials + Experimental Procedure steps 1–15 (fixation options, 3× washes, perm, block, primary 2 h, secondary 1 h, streptavidin 30 min, mounting) | Read directly 2026-11-05 (`agent-verified`) |
| T2 | Thermo Fisher, "Fix, Perm, & Block" (Molecular Probes School of Fluorescence) | https://www.thermofisher.com/us/en/home/life-science/cell-analysis/cell-analysis-learning-center/molecular-probes-school-of-fluorescence/imaging-basics/protocols-troubleshooting/protocols/fix-perm-block.html | Steps 1–10 table (1 mL 4% formaldehyde / 1 mL 0.5% Triton / 2 mL 3% BSA volumes; 15 min / 15 min / ≥60 min; storage note) | Read directly 2026-11-05 (`agent-verified`) |
| T3 | Thermo Fisher, "BestProtocols: Staining Intracellular Antigens for Flow Cytometry" (Protocols A/B/C) | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/staining-intracellular-antigens-flow-cytometry.html | Protocol A (IC Fixation 20–60 min; perm spins 400–600×g; stain 20–60 min), Protocol B (Foxp3 30–60 min, mouse ≤18 h), Protocol C (methanol + MeOH-resistant/sensitive table) | Read directly 2026-11-05 (`agent-verified`) |
| T4 | Thermo Fisher, "BestProtocols: Viability Staining Protocol for Flow Cytometry" (A/B/C) | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/viability-staining-flow-cytometry.html | Protocol A (PI/7-AAD 5 µL/100 µL, 5–15 min, no wash, 4 h), Protocol B (calcein 30 min), Protocol C1/C2/C4/C5 (FVD 1 µL/mL, 30 min 2–8 °C; 1:1000 plate working stock) | Read directly 2026-11-05 (`agent-verified`) |
| T5 | Thermo Fisher, "BestProtocols: Staining Cell Surface Targets for Flow Cytometry" (A/B) | https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/staining-cell-surface-targets-flow-cytometry.html | Protocol A (50 µL cells + 50 µL antibody = 100 µL; 30 min 2–8 °C; wash 2 mL/200 µL 400–600×g ×2; Fc-block and polymer-dye buffer amounts; storage 100+100 µL IC Fixation ≤3 d) | Read directly 2026-11-05 (`agent-verified`) |
| B1 | BD Biosciences, "Flow Cytometry Controls — Tips for Better Data" (blog) | https://www.bdbiosciences.com/en-us/learn/science-thought-leadership/blogs/flow-cytometry-controls-tips-for-better-data | Instrument setup controls (alignment, delay, sensitivity, gain, spillover, compensation, CS&T beads); single-stain / FMO / isotype-limits / biological controls | `search-summary-only` — page body not directly transcribed; do NOT plan numeric values from it |
| F1 | Columbia Univ., "FACSAria Sorting Guide" (PDF); BCCHR "Sample Preparation for Sorting on the FACS Aria" (PDF); BD FACSAria Fusion User's Guide; MSU "Sorting Best Practices 2025" / "SOP-BD-FACS-Aria-IIu"; Harvard Immunology sorting guidelines; Stanford Aria User Guide; PAVIR Aria Training Guide | (see search session; facility PDFs + vendor UG excerpts) | Sorting-vs-analysis distinction; single-cell suspension + mesh filtration; sort concentrations; viability-dye trigger; unstained + compensation controls; sterile collection; ≤4 populations; drop-delay/Accudrop; sort layout gates→destinations | `search-summary-only` — facility/vendor summaries only; no per-build configuration verified |
| G1 | Generic companion records in this repo | `docs/research/k2-operations/washing-and-sampling.md` (`wash`), `docs/research/k2-operations/inoculation-and-incubation.md`, `docs/research/k2-operations/absorbance-and-fluorescence-readout.md` (`FLUO_IMAGE`, Module B) | Wash semantics (identity conserved, no drying); incubation envelope; live-vs-fixed `FLUO_IMAGE` contract referenced, not duplicated | Repo context, not a scientific source |

Evidence status: T1–T5 `agent-verified` (single reader, still needs second-person check before any `SourceChecked` claim). B1/F1 `search-summary-only`. No `ExpertReviewed`, no `LabValidated`.

## 3. Non-merge rules (read first)

1. **Analysis ≠ sorting.** `FLOW-ACQ-ANALYSIS` consumes the sample optically and returns data only; `FLOW-SORT` additionally creates new sorted child samples with lineage. They MUST NOT share one default operation (same reason as wash-vs-aliquot in G1: one conserves identity, the other forks it).
2. **Live imaging ≠ fixed imaging.** This file covers **fixed** IF imaging only (`IF-IMAGE-FIXED`). Live-cell imaging (e.g. LIVE/DEAD-style unfixed work) follows the `FLUO_IMAGE (live)` contract in G1 and is NOT merged here: fixation is terminal for the cells, and some viability readouts are destroyed by fixation (T4-A vs T1/T2).
3. **Platform capability ≠ configured machine.** Laser / filter / cube / LED / detector / nozzle / stage lists in vendor envelopes are maxima. A plan MUST check the per-build configuration (which lasers fitted, which cubes mounted, which nozzle installed) — see §9 and UNKNOWNs.
4. Generic mechanics are **referenced, not restated**: liquid exchange → `wash` (G1); timed holds → incubation record; fixed-image acquisition mechanics → `FLUO_IMAGE` contract (G1 §3.2), with IF-specific channels/controls added here.

---

## 4. `FLOW-PREP` — staining a suspension for flow cytometry (surface ± intracellular ± viability)

| Block | Contract |
|---|---|
| catalog id / version | `FLOW-PREP.v1` · `research_draft` · action_type `stain` |
| Scope / exclusions | Cell suspensions (tubes or U/V-bottom plates) stained for flow analysis or sorting. Covers surface staining (T5-A), intracellular Protocols A/B/C (T3), viability Protocols A/B/C (T4), Fc/polymer-dye blocking (T5). Excludes RBC-lysed whole-blood workflow detail (T5-B noted only), cell preparation/dissociation upstream, sorting itself (`FLOW-SORT`), acquisition (`FLOW-ACQ-*`), gating/compensation math (downstream). |
| Purpose | Convert a live single-cell suspension into a stained suspension (optionally fixed) whose per-cell fluorescence reports surface/intracellular markers + viability state. Changes antibody occupancy and viability-dye state; fixation (if used) ends live-cell usability. |
| Ports | `cells_in` (required, single-cell suspension) → `cells_out` (same identity, stained; fixed-flag set iff fixation applied); `antibody_mix_in`, `viability_dye_in` (consumables); `waste_out` (supernatants, untracked). No aliquot IDs created here. |
| Sample transitions | `cells_live → [surface stain] → cells_stained_live` (reversible to sorter/acquisition); `cells_stained_live → [fix→perm→intracellular stain] → cells_fixed_stained` (**terminal**: no return to culture; T3/T5). PI/7-AAD-stained cells MUST NOT enter any fixation step (T4-A: incompatible). |
| Parameters (sourced only) | Surface (T5-A): 50 µL cell suspension + antibody mix to 100 µL final; 30 min 2–8 °C dark (purified/biotinylated primary 1 h); wash 2 mL/tube or 200 µL/well, 400–600×g 5 min RT, ×2; secondary ≥30 min 2–8 °C. Fc block: mouse 0.5–1 µg anti-CD16/32 per 100 µL, 10–20 min 2–25 °C; human 20 µL Fc-inhibitor per 100 µL; or 2 µL 2% serum per 100 µL, 15 min, no pre-wash. Polymer dyes: 5 µL Super Bright Complete *or* 50 µL Brilliant Stain Buffer (*or* 10 µL Plus variant) per sample; 5 µL CellBlox (Plus) per sample whenever NovaFluor/cyanine dyes used (T5 table). Intracellular (T3): A — fix 100 µL IC Fixation Buffer (tubes) / 200 µL (plate), 20–60 min RT dark; perm wash 2 mL / 200 µL, 400–600×g 5 min, ×2; stain in perm buffer 20–60 min RT (plate ≥30 min); optional 2% serum block 2 µL, 15 min. B (Foxp3/nuclear) — 1 mL working solution/tube, 200 µL/well; 30–60 min 2–8 °C or RT dark; mouse ≤18 h 2–8 °C. C (phospho/methanol) — fix by adding equal volume IC Fixation Buffer, 10–60 min RT dark; 1 mL ice-cold 90–100% methanol, ≥30 min 2–8 °C; spins 400–600×g 5 min (plate 600×g 4–5 min). Viability (T4): PI/7-AAD — 5 µL per 100 µL cells post-surface-stain, 5–15 min ice/RT, **do not wash**, keep dye present during acquisition, analyze ≤4 h (2–8 °C dark). Calcein — 1–5×10⁶ cells in 0.1–1 mL, dye per lot sheet (titrate), 30 min RT dark, wash ×2. FVD — protein/azide-free PBS, 1–10×10⁶/mL, ≥0.5 mL, 1 µL FVD per 1 mL cells (plate: 1:1000 working stock, 100 µL/well; cocktail: 0.5–1 µL/sample, minimize cocktail dwell), 30 min 2–8 °C dark, wash 1–2×; <5% dead expected → heat-kill aliquot 65 °C 1 min + ice 1 min, mix 1:1 for a positive peak. MeOH fluorochrome rule (T3-C table): resistant Alexa 488 / eFluor 660 / Alexa 647 / eFluor 450 / FITC; sensitive PE(+tandems) / PerCP(+tandems) / APC(+tandems) — do not plan sensitive dyes through methanol. |
| Time scope | Within-step, `once` per stained tube/well. Surface ≈30 min + washes; intracellular adds 20–60 min fix + ≥30 min stain; FVD 30 min. Exact totals are sums of the sourced holds above; no separate incubation node needed for the holds listed here. Post-stain storage (T5: 100 µL sample + 100 µL IC Fixation Buffer or 2 mL 1-step Fix/Lyse, ≤3 d 2–8 °C dark) is a hold with its own window, not part of staining time. |
| Preconditions | Single-cell suspension confirmed (prep upstream done); antibodies spun, not vortexed, stored 2–8 °C / 4 °C dark, never frozen (T3/T5 notes); correct buffer set for target class (cytoplasmic → T3-A; nuclear → T3-B; phospho → T3-C — buffer choice is target-dependent, T3 intro); FVD in protein/azide-free PBS for maximum brightness (T4-C); light protection from first fluorochrome step (T3–T5). |
| Completion checks | All panel antibodies + viability dye applied at recorded dilutions; wash count met (surface ×2; intracellular ×2 post-fix + ×2 post-stain); PI/7-AAD tubes explicitly flagged no-wash with dye present (C1); fixed-flag recorded; storage buffer + clock recorded if held (C2). Deterministic: missing wash or wrong buffer set blocks acquisition. |
| Instrument requirements | No instrument beyond pipettors/centrifuge (400–600×g, refrigerated to 2–8 °C for Foxp3/FVD holds)/dark incubation. Centrifuge ×g-vs-RPM rotor mapping is configuration-dependent (T3 states ×g; T5 states ×g — rotor UNKNOWN, §10). |
| Controls (required companions) | Unstained; **single-stain** per fluorochrome for spillover/compensation (cells, or beads per B1 — beads `search-summary-only`); **FMO** per gated channel for background/gate placement (B1); viability dye in every run (FVD if any fixation follows; PI/7-AAD only for unfixed surface-only runs, T4-A); Fc-block verification populations where relevant (T5); isotype controls only with stated limits (B1: do not account for spillover — never a substitute for FMO). See §7 for control→claim mapping. |
| Failure modes | Wrong buffer set for target class → lost epitope (T3 intro) — re-run with correct set, do not rescue by boosting antibody. Methanol-sensitive dye through Protocol C → dead channel (T3-C table) — re-panel. PI/7-AAD then fixed → uninterpretable viability (T4-A) — re-stain with FVD. FVD in protein/azide buffer → dim dead peak / high live background (T4-C4 note) — re-stain in PBS. Polymer-dye interaction without blocker → undercompensated look (T5) — re-run with blocker. Vortexed antibody / frozen conjugate → degraded staining (T3/T5 notes). |
| Readout | N/A (no data produced; stained suspension only). Raw flow data belongs to `FLOW-ACQ-*` §6. |
| UNKNOWNs | Antibody clones/dilutions per target; polymer-dye incompatibility matrix beyond "use blocker"; FVD lot-specific µM; rare-event cell-input equation precision (T5 cites r=(100/CV)×2 — formula `agent-verified` as cited, inputs method-dependent). See §10. |

## 5. `FLOW-ACQ-ANALYSIS` — analytical acquisition (no sorting)

| Block | Contract |
|---|---|
| catalog id / version | `FLOW-ACQ-ANALYSIS.v1` · `research_draft` · action_type `acquire` |
| Scope / exclusions | Passing a stained suspension through an analyzer flow cell and recording per-event scatter + fluorescence (FCS). **No droplets charged, no collection, no sorted children.** Excludes sorting (`FLOW-SORT`), gating/compensation/statistics (downstream derivation). |
| Purpose | Record raw event list. Does not change biology except photobleaching/sheath-fluid consumption; fixed cells stay fixed, live cells are consumed as measured sample (not returned to culture by this op — re-entry, if any, is a separate transfer, not modeled here). |
| Ports | `cells_stained_in` → `cells_measured_out` (same identity, consumed/held as waste per lab SOP) + `data_out` (raw FCS: per-event FSC/SSC/fluorescence + laser/filter/gain metadata + timestamps + instrument ID). |
| Sample transitions | `cells_stained → [acquire] → cells_consumed + raw_FCS`. No new sample IDs. |
| Parameters | Event count / stop condition (e.g. target gated events — value method-dependent, UNKNOWN as default); flow rate; laser/filter/gain per channel **as configured on this machine** (not the platform envelope); threshold/trigger channel; QC-bead check per B1 (`search-summary-only`). No sourced numeric defaults carried here. |
| Time scope | Instrument time only (`once` per tube/well). Staining/storage clocks belong to `FLOW-PREP`. |
| Preconditions | Stained per `FLOW-PREP` with correct fixed//live flag; single-stain + FMO + viability controls of the same run available (B1); laser/filter set covers every panel dye (per-build check); QC (alignment/delay/sensitivity per B1) passed. |
| Completion checks | Every addressed tube/well returns an FCS file with channel metadata + instrument ID (C1); control files present (C2); no instrument error flag. Values NOT interpreted here. |
| Instrument requirements | Analyzer-class cytometer: hydrodynamic flow cell + per-dye laser excitation + filtered detectors + acquisition software (capability categories; B1). Required configuration record per run: laser wavelengths fitted, optical filter set per detector, gain/PMT voltages, threshold. No model named; no envelope value treated as per-machine fact. |
| Controls | Same-run unstained + single-stain set + FMOs + viability (see §7). |
| Failure modes | Clogged/clumped sample → event-rate collapse (filter single-cell suspension upstream); wrong filter for dye → missing channel (per-build check failure — block run); gain set without single-stain refs → uncompensatable data (B1). |
| Readout | **Raw**: FCS event list + metadata. **Derived** (NOT this op): compensation matrix, gates, %positive, MFI — downstream with FMO/single-stain justification. |
| UNKNOWNs | Per-build laser/filter/gain/threshold values; event-count targets; QC acceptance limits; software/procedure versions. All `configuration_dependent` or `method_dependent` (§10). |

## 6. `FLOW-SORT` — sorting acquisition (separate record)

| Block | Contract |
|---|---|
| catalog id / version | `FLOW-SORT.v1` · `research_draft` · action_type `sort` |
| Scope / exclusions | Deflecting gated droplets into collection vessels for downstream use/culture. Covers sterile collection setup + sort layout (gates→destinations) + drop-delay management. Excludes analysis-only runs, downstream culture of sorted cells, sort-purity re-analysis (separate `FLOW-ACQ-ANALYSIS` on an aliquot). |
| Purpose | Fork one stained suspension into ≥1 sorted child populations (new sample IDs with lineage → parent) plus waste. Only this flow record creates samples. |
| Ports | `cells_stained_in` (required) → `sorted_out[i]` (new ID each, lineage → parent) + `waste_out` (untracked, incl. abort/waste stream) + `data_out` (raw FCS + sort-layout record: gates→destinations, drop delay, counters). |
| Sample transitions | `cells_stained(n) → [sort] → SORTED(1..k, new IDs) + waste`. Parent identity conserved as source; volume/cell conservation is method-dependent (yield < input by design — abort + waste). |
| Parameters | All `search-summary-only` (F1) — do NOT plan on these until verified: single-cell suspension filtered (<70 µm reported; 35 µm mesh reported); sort concentration bands reported (order 5–20×10⁶/mL); viability-dye gating reported (7-AAD/PI trigger when viability <75% reported); negative unstained aliquot (≥0.3 mL reported) + per-fluor compensation controls; acquisition window (10k–30k gated events reported); ≤4 simultaneous populations reported; sterile collection tubes with serum/antibiotic medium (20% FCS / 50% FBS variants reported); drop-delay via Accudrop-type calibration; sort layout gates→destinations with counters. Each is UNKNOWN-for-planning until the facility SOP + build sheet are read. |
| Time scope | `once` per sort session; rate/yield are device_state + method_dependent (no verified throughput carried). |
| Preconditions | Sterile sorted-path + collection vessels with medium (F1, unverified detail); single-cell suspension; controls as in `FLOW-ACQ-ANALYSIS` PLUS sort-layout review (gate→destination mapping checked — mis-mapping mis-sorts); operator/booking per facility (F1). |
| Completion checks | Each destination has collected material with parent-link + gate record (C1); counters/drop-delay logged (C2); post-sort viability/purity check dispositioned (re-analysis is a new `FLOW-ACQ-ANALYSIS`, not implied). |
| Instrument requirements | Sorter-class cytometer: flow cell + droplet breakoff + charging/deflection + collection chamber (capability categories). Configuration per run: nozzle size, pressure, drop delay, laser/filter set, collection-device map. None verified here. |
| Controls | Same-run unstained + single-stain + FMO + viability (B1/F1); sort-purity control (post-sort re-acquisition aliquot) where claimed. |
| Failure modes | Aggregates → clog/abort storm (re-filter upstream); wrong drop delay → mis-sort (re-calibrate, discard affected fraction per SOP); non-sterile path → contaminated sorted culture (sterility precondition failure); gate→destination mismatch → swapped populations (layout check C-gate). All mitigations facility-SOP-dependent. |
| Readout | **Raw**: FCS + sort-layout log. **Derived** (NOT this op): purity %, yield %, post-sort viability — downstream. |
| UNKNOWNs | Every numeric above; nozzle/pressure/drop-delay per build; collection-medium formulation; purity acceptance criteria. See §10. |

## 7. Panel / channel conditions and control→claim mapping (flow)

- **Panel condition.** A panel is valid for a run only if: every dye is excitable/detectable by the **fitted** laser/filter set (per-build check, not platform envelope); methanol-exposed panels use only MeOH-resistant dyes (T3-C table); polymer-dye panels include the matching blocker (T5 table); fixable viability dye is used whenever fixation precedes acquisition (T4). Panel design (dye assignment, titrations) is method-dependent and carries no default here.
- **Channel condition.** A channel is reportable only with its chain recorded: dye → laser line → filter → gain → single-stain reference → FMO boundary (B1) or lot-matched equivalent. Unrecorded gain/filter = unreportable channel.
- **Control→claim mapping.** Unstained → autofluorescence/background floor. Single-stain per fluorochrome → spillover/compensation (B1). FMO per gated channel → gate boundary where spillover background ends (B1). Viability (FVD for fixed runs; PI/7-AAD only for unfixed runs, T4-A) → dead-cell exclusion; no viability claim without it (dead cells bind antibody nonspecifically, T4 intro). Isotype → limited nonspecific-binding context only, never a gate setter alone (B1). Biological (unstimulated/negative) → expression call context (B1/T3-C untreated control).

## 8. `IF-FIXPERMBLOCK` + `IF-STAIN` + `IF-IMAGE-FIXED` — immunofluorescence on fixed cells

### 8.1 `IF-FIXPERMBLOCK.v1` (`research_draft`, action_type `fix_permeabilize_block`)

Covers adherent cells fixed → permeabilized → blocked as a terminal preparation step (T1 steps 2–7; T2 steps 1–10). Volumes in T2 are for one well of a 6-well plate / one 35 mm dish; T1 chamber-slide volume is 100 µL/chamber (8-chamber slide). **T1 and T2 disagree on perm strength and are both kept** — T1: 1% Triton X-100, 37 °C 30 min + 10 min RT; T2: 0.5% Triton X-100, 15 min RT. No merged "standard" is offered; the target-antigen optimization note governs (T1: "optimal fixation (both reagent and time) depends on the antigen … must be optimized").

| Block | Contract |
|---|---|
| Purpose | Immobilize cellular structures (fixation), grant antibody access (permeabilization), suppress nonspecific binding (blocking). Cells are dead after fixation; nothing here returns to culture. |
| Ports | `cells_live_in` (adherent, 60–80% confluency per T1, e.g. 30,000 cells/chamber) → `cells_blocked_out` (same vessel identity, state FIXED+BLOCKED); `reagent_in` (fixative/perm/block formulations); `waste_out` (untracked). |
| Sample transitions | `cells_live → [fix] → cells_fixed → [perm] → cells_permeabilized → [block] → cells_blocked`. Fixation is the terminal point for live-cell identity. |
| Parameters (sourced) | Fix option A: 4% formaldehyde/PFA in PBS, 15 min RT (T1: 4% formaldehyde; T2: 1 mL 4% w/v formaldehyde in PBS). Fix option B: 100% ice-cold MeOH, 10 min at −20 °C (T1). Washes between stages: 3× PBS, 5 min/wash, dropper + aspiration (T1); T2: 3× PBS by side-wall pipetting/swish/tilt. Anti-drying rule: never let wells dry from first wash on (T1 note); aspirate ≤2 wells at a time (T1). Block: T1 — 10% normal serum (secondary-host species) in PBS or TBS + 0.1% Triton X-100, 1 h RT humidified; T2 — 2 mL 3% w/v BSA in PBS, ≥60 min RT (up to overnight); 30-min shortening only for robust systems (T2 tip). Humidified/Parafilm incubation for perm + block (T1). Fixed-sample hold: several days at 5 °C (T2) — a hold window, not a culture state. |
| Time scope | `once` per vessel. Fix 10–15 min + perm 15–40 min + block 0.5 h–overnight per the option chosen; summed from sourced holds, no hidden default. |
| Preconditions | Correct confluency/vessel seated; fixative choice matched to antigen (T1 optimization note); serum species matched to secondary host (T1); PBS/TBS formulations recorded (T1 gives PBS 137 mM NaCl / 2.7 mM KCl / 10 mM Na₂HPO₄ / 1.76 mM KH₂PO₄ pH 7.4; TBS 50 mM Tris / 150 mM NaCl pH 7.4). |
| Completion checks | 3× washes completed per stage (C1); no drying interval (C2 — advisory-to-blocking: dried wells invalidate background); block applied at recorded formulation/time (C3). |
| Instrument requirements | None beyond manual pipetting/aspiration + RT incubation + humidified chamber. No microscope needed at this stage. |
| Controls | Fix/perm optimization per antigen (T1 note) — the "control" here is a condition matrix, not a tube; unstained/secondary-only companions are defined in §8.2. |
| Failure modes | Wrong fixative/time for antigen → lost epitope or no access (T1) — re-optimize, do not rescue by boosting exposure later. Drying → high background, uninterpretable (T1) — discard. Wrong serum species → background (T1 formulation). Over/under-permeabilization → no signal vs destroyed morphology (T1/T2 strengths kept separate). |
| Readout | N/A. |
| UNKNOWNs | Antigen-specific fix/perm choice; Triton %/time per cell type beyond the two sourced options; block-cure/mount-cure times; storage-day limit ("several days" unquantified, T2). §10. |

### 8.2 `IF-STAIN.v1` (`research_draft`, action_type `immunostain`)

Covers primary → secondary (± streptavidin 3-step) → mount per T1 steps 8–15. References `wash` (G1) for the 3× 5-min exchanges and `FLUO_IMAGE` companion conventions for light protection.

| Block | Contract |
|---|---|
| Purpose | Bind primary antibody to intracellular antigen, then reveal with fluorophore-conjugated (2-step) or biotinylated + streptavidin-fluorophore (3-step) secondary; mount as archivable preparation. |
| Ports | `cells_blocked_in` → `preparation_mounted_out` (same vessel/slide identity, state STAINED+MOUNTED, archivable); `antibody_in` (lots + dilutions recorded). |
| Sample transitions | `cells_blocked → [primary] → cells_primary → [secondary ± streptavidin] → cells_labeled → [mount] → preparation_archived`. |
| Parameters (sourced, T1) | Primary at manufacturer-recommended dilution in blocking reagent, 2 h RT humidified (low-abundance up to 12 h 4 °C). Wash 3× TBS 5 min. Secondary at recommended dilution in blocking reagent, 1 h RT humidified; **if fluorophore-conjugated: protect from light AND omit Triton X-100 from blocking reagent** (T1 step 10). Streptavidin (3-step only): eFluor 605NC / 650NC / FITC conjugates in 10% normal serum/TBS without Triton, 30 min RT; start 1:100 for 605NC. Wash 3× TBS 5 min. Mount: Fluoromount-G or Fluoro-Gel; store slides 4 °C protected from light. |
| Time scope | `once` per vessel/slide. Primary 2 h (≤12 h cold variant) + secondary 1 h (+ 30 min streptavidin if 3-step); washes 3×5 min each round. |
| Preconditions | Block completed (`IF-FIXPERMBLOCK` C3); antibody dilutions from lot data sheets (no dilution invented here); light protection available for any fluorophore step; TBS preferred with eFluor nanocrystals for maximum intensity (T1 note, conditional). |
| Completion checks | Primary lot + dilution + time/temp recorded (C1); secondary format recorded (direct-fluor vs biotin + streptavidin conjugate, C2); light-protection + Triton-omission flags verified for fluor secondaries (C3); mount medium + storage recorded (C4). |
| Instrument requirements | None (manual). Imaging requirements belong to §8.3. |
| Controls (required companions, same run) | **Secondary-only** (no-primary) control; **unstained/autofluorescence** control (same fix/perm/block, no antibodies); per-antigen fix/perm optimization series (T1). No "positive" localization claim without both (see G1 §5/§8 IF rows). |
| Failure modes | Triton left in fluor-secondary block → degraded signal path (T1) — re-stain. Light exposure of fluorophores → fading (T1/G1-F7) — re-stain, re-record exposure history. Streptavidin applied in 2-step (direct-fluor) workflow → mis-protocol (T1 step 9 routing) — route by primary format. |
| Readout | N/A (preparation only; photons belong to §8.3). |
| UNKNOWNs | All antibody dilutions/times per target; Triton-omission scope beyond fluor secondaries; mount curing time; per-lot background. §10. |

### 8.3 `IF-IMAGE-FIXED.v1` (`research_draft`, action_type `image_fixed`)

Fixed-preparation imaging only. Mechanics (per-frame metadata, focus modes, completion checks) follow the `FLUO_IMAGE` contract (G1 §3.2/§6/§7); this record adds IF-specific channel/controls and the fixed-terminal transition. **Live imaging is NOT covered here** — use G1 `FLUO_IMAGE (live)`.

| Block | Contract |
|---|---|
| Purpose | Record raw fluorescence frames (position × channel) from a mounted fixed IF preparation. No counting, no expression call. |
| Ports | `preparation_mounted_in` → `preparation_archived_out` (same identity, preserved) + `data_out` (raw image set: position → channel → exposure/objective/cube/filter metadata + timestamp + instrument ID). |
| Sample transitions | `preparation_mounted → [image] → preparation_archived + raw_images`. Terminal for cells; the mount is the archive. No transition back to culture exists. |
| Parameters | `channels`: per secondary/streptavidin conjugate actually on the preparation (dye + excitation/emission + filter/cube IDs from the build sheet — no band invented here). `exposure` per channel (recorded per frame, no default carried). `objective` actually used (ID/mag/NA/immersion). `position_ids` (slide/chamber/dish fields; Navigator-type multi-position only where a scanning stage is fitted, G1 §3.2). `focus_mode` per build (manual/motorized/closed-loop/AFC where fitted). Mount medium carried from §8.2. |
| Time scope | Acquisition time only (`once` per (position, channel)). All fix/perm/block/stain clocks belong to §8.1–§8.2. |
| Preconditions | Mounted preparation labeled with channel map; fluorescence path present on **this** build (module + cubes + illumination + detector — per-build check, G1 §3.2); correct cubes/illumination for the dyes on board; focus strategy set; light protection observed (T1). |
| Completion checks | Every requested (position, channel) has a file with channel/cube/exposure/objective recorded (C1); focus-lock status recorded where applicable (C2); no saturated-to-clip or fully dark frame accepted without operator note (C3, per G1). |
| Instrument requirements | Inverted widefield (or equivalent) fluorescence platform **as configured**: Infinity Port / fluorescence module only if fitted; filter cubes (single-to-quad) with part/band record; illumination per build; detector per build; 22 mm-class FOV optics; focus device per build; scanning stage required for multi-position templates. Platform envelope maxima MUST NOT stand in for the build sheet (§9). |
| Controls | Secondary-only + unstained frames acquired with identical settings (companion to §8.2 controls); exposure/focus held comparable across control and full-stain frames or the comparison is void. |
| Failure modes | Wrong-cube/wrong-LED configuration (G1-F5) — block run, reassign. Photobleaching across previews (G1-F7) — minimize preview, record history. Focus drift without AFC / missing positions without scanning stage (G1-F9) — require capability check at plan. Fix/perm mismatch (G1-F8) — re-optimize upstream, never rescue by exposure boost. |
| Readout | **Raw**: frames + metadata. **Derived** (NOT this op): intensity quantification, colocalization, cell counts — downstream with secondary-only/unstained justification. |
| UNKNOWNs | Per-build objective catalog, cube bands, LED λ/power, camera specs, stage travel/speed, incubation ranges, LAS/software procedures — all `configuration_dependent` (§10). |

## 9. Instrument capability vs configured machine

| Capability class | Platform envelope (what the class CAN do) | Per-run configured machine (what THIS machine does today) — MUST record |
|---|---|---|
| Flow analyzer | Hydrodynamic flow cell, multi-laser excitation, filtered detectors, FCS output (B1, `search-summary-only`) | Laser lines fitted + filter set per detector + gains + threshold + software version |
| Flow sorter | Analyzer capabilities + droplet breakoff/charging/deflection + collection chamber (F1, `search-summary-only`) | Above + nozzle size, pressure, drop delay, collection-device map, sterile-path status |
| IF microscope | Widefield fluorescence path IF FITTED: Infinity Port/module, single-to-quad cubes, optional filter wheel, EL6000/pE-family-class illumination, K3M–K8/Fusion/Kinetix22-class detectors, 22 mm-class FOV, closed-loop/AFC focus, scanning stage (G1 S3–S5, transcribed in companion drafts) | Module fitted? which cubes (part + bands) + illumination λ/power + detector ID + objective used + focus device + stage fitted? + incubation fitted? |

Rule: a plan citing only the left column is rejected. Every acquisition/sort/image node needs the right column filled from the build sheet, or an explicit UNKNOWN with `configuration_dependent` reason (§10).

## 10. UNKNOWNs (do not use for planning)

| Item | Status | Reason class |
|---|---|---|
| Per-build cytometer lasers / filters / gains / thresholds; sorter nozzle / pressure / drop delay / collection map; microscope objectives / cube bands / LED λ+power / camera / stage / incubation ranges | UNKNOWN | `configuration_dependent` — no build sheet read |
| Antibody clones, dilutions, incubation tweaks per target; polymer-dye interaction matrix; FVD lot µM; calcein working concentration; titration endpoints; spectral-unmixing thresholds | UNKNOWN | `method_dependent` — lot sheets / titrations not read |
| Sort concentrations, event counts, collection-medium formulation, throughput, purity criteria; QC-bead limits; PMT-setting procedure | UNKNOWN | `source_missing` — F1/B1 only `search-summary-only`, no facility SOP or user guide transcribed |
| Absolute cell-input numbers for rare-event precision (T5 formula inputs) | UNKNOWN | `method_dependent` — assay-defined |
| Mount curing time; fixed-sample storage-day limit ("several days", T2); Triton %/time per cell type beyond T1/T2 options; SDS/HCl-stop compatibility | UNKNOWN | `source_missing` — no source states a value |
| T3-C plate methanol-storage temperature ("≤20 °C" as printed vs tubes "<−20 °C") | UNKNOWN (conflict kept) | `source_missing` — probable missing minus sign; do not resolve by guessing, check kit/lot manual |
| Centrifuge rotor ×g↔RPM mapping where a protocol states one unit | UNKNOWN | `device_state` — rotor-dependent |
| Kinetic/continuous flow-readout timing; shaking speed/amplitude; service intervals | UNKNOWN | `not_applicable` to these records (no kinetic claim made) |

## 11. What this record does NOT cover

- Cell preparation/dissociation upstream of staining (single-cell suspension is a precondition, not a step here).
- RBC-lysed whole-blood staining detail (T5-B inventoried in §2 only), tissue-section IF, methanol-free phospho alternatives beyond T3-C, live-cell imaging (G1 `FLUO_IMAGE (live)`).
- Compensation math, gating strategy, %positive/MFI derivation, image segmentation/quantification — all downstream derivations with their own controls.
- JSON generation: every UNKNOWN above stays omitted/UNKNOWN in any `OperationModule` draft; long explanations here do not enter JSON; all JSON stays `research_draft` until a `ReviewRecord` exists.
- Any claim of `SourceChecked`, human review, or lab validation — none made (§12).

## 12. Review

| Field | Value |
|---|---|
| Status | `research_draft` |
| Transcription | Single AI agent, 2026-11-05, from T1–T5 (direct reads) + B1/F1 (summaries, not verified) + G1 (repo context) |
| Second-person check | Not done — `SourceChecked` MUST NOT be claimed |
| Human/expert/lab review | None — `ExpertReviewed` / `LabValidated` MUST NOT be claimed |
| Promotion requirement | Second-person field-by-field verification against §2 locators + facility build sheets + lot data sheets, recorded in a `ReviewRecord`, before any Compatible/JSON use |
