# Microscope — Leica DMi8 (inverted, modular platform)

review_status: research_draft

> Source rule: every numeric/factual claim below carries an official Leica source + locator + retrieval date. Anything configuration-dependent or not found on an official page is marked UNKNOWN. No values invented.

## 1. Identity

- Instrument: Leica DMi8 inverted microscope (life-science research platform; distinct from the industrial DMi8 M/C/A line).
- Architecture: inverted (optics below stage, sample on top of objectives); fully modular stand ("House"/T-House) configurable manual → motorized, non-fluorescence → fluorescence, with post-purchase upgrades.
- Control software: Leica Application Suite X (LAS X); microscope SDK available for third-party control.
- Regulatory: IVD-certified variant exists (contact Leica partner for details); EC DoC / UKCA / NRTL certificates listed on the official downloads page.
- Document: Leica DMi8 Product Brochure EN, MC-0009257, copyright 2024.

## 2. Sources personally verified (read 2026-11-05)

1. Leica DMi8 Tech Specs table — https://www.leica-microsystems.com/products/light-microscopes/p/dmi8/specification/ — spec table rows. Retrieved 2026-11-05.
2. Leica DMi8 FAQ — https://www.leica-microsystems.com/products/light-microscopes/p/dmi8/faq/ — Q&A answers ("Which contrast methods…", "What is an Infinity Port?", "What is a T-House?", "What is Closed Loop Focus?", "What is Adaptive Focus Control?", "What is LID?", sample weight, ratio/TIRF/super-resolution answers). Retrieved 2026-11-05.
3. Leica DMi8 Downloads page — https://www.leica-microsystems.com/products/light-microscopes/p/dmi8/downloads/ — "Brochure or flyer" table (DMi8 Product Brochure EN, PDF 15 MB) and "Certificates" table (EC DoC / UKCA / NRTL entries). Retrieved 2026-11-05.
4. Leica DMi8 Product Brochure EN (PDF) — https://downloads.leica-microsystems.com/DMi8/Brochure%20or%20Flyer/DMi8_Product_Brochure_EN.pdf — pp. 4–6, 10–14, 16. Retrieved 2026-11-05.

NOT verified by me (do not treat as checked): the industrial DMi8 M/C/A spec page, the ObjectiveFinder catalog, third-party mirrors of the brochure (jhtechnologies, dmimedicalusa, synercon.ru), fpbase.org spectra, Okolab/Tokai Hit manuals. No SourceChecked claim is made.

## 3. Capability table (official values only)

| Capability | Documented value | Source + locator (retrieved 2026-11-05) |
|---|---|---|
| Platform / stand | DMi8 modular inverted stand; T-House module carries Infinity Port connections, fluorescence-illumination interface, stage and transmitted-light devices | FAQ "What is a T-House?"; brochure p. 14 |
| Base imaging mode | Widefield | Spec table "Imaging modes" row |
| Transmitted-light contrast (spec table) | BF, Darkfield, IPH, IMC, DIC | Spec table "Transmitted light contrast & method" row |
| Contrast methods (FAQ, broader list) | BF, DF, DIC, IMC, Polarization, Phase Contrast, IPH, Fluorescence "all available" — availability is configuration-dependent; spec table omits phase/polarization, so confirm per build | FAQ "Which contrast methods can I use with the Leica DMi8?" |
| Fluorescence path | Optional; Infinity Port gives direct access to fluorescence light path for illumination sources / photomanipulation / DMD / TIRF modules; fluorescence module optional (manual or motorized, liquid light guide, field + aperture plane access); system can be ordered without fluorescence/Infinity Port and retrofitted later | FAQ "What is an Infinity Port?", "I don't need fluorescence…", "How can I make use of an Infinity Port?", "Can I do TIRF experiments?" |
| Fluorescence turret / cube ID | All DMi8 filter cubes carry RFID (LID); optional LID-equipped turret auto-detects cubes in LAS X | FAQ "What is LID?" |
| Filter cubes | "From single to quad"; dye spectra via fpbase.org/spectra link (no per-cube bands on the spec page) | Spec table "Filter cubes" row |
| External filter wheel | Optional, up to 8 dyes, 24 ms switch time | Spec table "External filter wheel (EFW)" row |
| Fluorescence illumination sources | EL6000; pE300 up to pE800 MB or pE800 fura | Spec table "Fluorescence illumination" row |
| Detectors | "From K3M to K8, Hamamatsu Fusion or Kinetix22 (22 mm FOV)" — model/family names only, no sensor specs on this page | Spec table "Detection" row |
| Ratiometric (FURA) option | FURA objectives maximized for UV detection + camera image splitter to capture two fluorescent channels simultaneously (FURA/ratio configuration details beyond this not on FAQ) | FAQ "Can I do ratio imaging?" |
| High-speed fluorescence support | Fast external filter wheel, AFC, high-speed sCMOS/EMCCD compatibility, LAS X control (no fps numbers stated) | FAQ "I need to perform a high speed fluorescence experiment…?" |
| Field of view | 22 mm FOV optics; ">30% fewer tiles" vs 19 mm for same area; enables large-format cameras (Teledyne Kinetix22 named as example) | Brochure p. 6; spec table "(22mm FOV)" |
| Focus | Manual to motorized, closed-loop, up to AFC (hardware focus for fast focusing + drift correction) | Spec table "Focus" row |
| Closed-loop focus precision | Precision/reproducibility better than 20 nm for a given value (iterative measurement); 12 mm focus travel range; reduces gear slippage/hysteresis; independent of step size/optics/specimen | FAQ "What is Closed Loop Focus?" |
| AFC principle | Infrared LED measures distance between reflective surface at image plane and objective; actively holds focus under changing environmental conditions | FAQ "What is Adaptive Focus Control?" |
| Stage | Fixed, manual, or motorized up to QUANTUM high-speed stage, real-time controlled with Synapse (no travel/speed/repeatability numbers on spec page) | Spec table "Stage" row |
| Hardware triggering | Synapse integrated real-time controller, microsecond-precision synchronization of hardware components | Brochure pp. 6, 14 |
| Sample formats | Multi-position experiments on slides, dishes, multi-well plates "and more" via Navigator templates; overview-to-multichannel workflow; full Sample Finder requires scanning stage | Brochure p. 4; spec table footnote |
| Sample weight limit | Max ~30 kg depending on stage used; "in theory" no height limitation (objectives/turret below stage) | FAQ "How big and heavy can samples be…?" |
| Incubation | Optional; cage systems and stage-top incubators, e.g. Okolab, Tokai Hit (no T/CO₂/humidity ranges on Leica pages — see unknowns) | Spec table "Incubation" row; FAQ live-cell answer |
| Immersion handling | Adaptive Immersion: sensor cap continuously monitors/maintains immersion; enables switching to immersion objectives (water immersion emphasized) without removing sample; pairs with Navigator/Sample Finder ROI workflow and SmartCORR | Brochure pp. 4–5; spec table upgrade-path row |
| Objective correction aid | SmartCORR auto-optimizes objective correction-collar settings to reduce spherical aberration; uses MotCORR-compatible objective adjustment | Brochure p. 8; spec table upgrade-path row |
| Computational clearing | THUNDER (2D/3D) opto-digital Computational Clearing for widefield and spinning-disk data; THUNDER Live real-time parameter optimization; Aivia AI analysis | Spec table "Optional or enhanced image clarity…" row; brochure pp. 7, 9 |
| Confocal / super-resolution paths | CICERO Spinning Disk Confocal upgrade; GSD 3D super-resolution system built on DMi8; TIRF via Infinity Port (Leica AM TIRF or adapted own system), down to single molecule stated qualitatively | Spec table upgrade-path row; FAQ TIRF + super-resolution answers; brochure pp. 7, 14 |
| Light-path expansion | Cage System 30 mm Infinity Port Connector for homebuilt devices (photomanipulation, optogenetics, microspectroscopy, etc.); micromanipulation via Leica/Narishige/Eppendorf solutions | FAQ "How can I use the Leica DMi8 to build my own research solution?", "Is there a solution for microinjection?" |
| Hood / biohazard use | Can be configured exclusively for digital imaging (remote monitoring via software) | FAQ "I want to use my microscope in a hood…?" |

## 4. Operating conditions & constraints (documented only)

- Sample weight ≤ ~30 kg depending on stage (FAQ, retrieved 2026-11-05). No per-stage load table on the pages read — UNKNOWN per stage.
- Full Sample Finder functionality requires a scanning stage (spec-table footnote).
- IVD use: certified, details via Leica partner (FAQ) — certificate numbers/editions not transcribed; see downloads-page Certificates table.
- Fluorescence, Infinity Port, incubation, AFC, THUNDER, spinning disk, TIRF are optional/configuration-dependent — a given DMi8 may have none of them. The spec page describes the platform envelope, not a fixed configuration.

## 5. UNKNOWN (not on the official pages read; do not fill plausibly)

- Objective catalog for the DMi8 build: magnifications, NA, immersion media, working distances, correction classes, part numbers — UNKNOWN (a 63x objective appears only as an image credit in brochure p. 11, not a specification).
- Objective turret capacity/positions, condenser models, DIC prism/objective pairing, phase-ring alignment details — UNKNOWN.
- Filter-cube part numbers, excitation/emission bands, dichroic specs, number of simultaneously mounted cubes — UNKNOWN (only "single to quad" + fpbase link documented).
- Fluorescence channel count, LED/laser wavelengths and power, EFW dye list — UNKNOWN.
- Camera sensor specs (resolution, pixel size, QE, frame rate, cooling) for K3M–K8 / Fusion / Kinetix22 on the DMi8 — UNKNOWN.
- Stage travel range, speed, repeatability, accuracy; focus speed/resolution beyond the 20 nm closed-loop reproducibility claim — UNKNOWN.
- Incubation performance (temperature/CO₂/O₂/humidity ranges, uniformity, control accuracy) — UNKNOWN; consult Okolab/Tokai Hit manuals for the fitted unit.
- TIRF penetration depth, TIRF wavelengths/angles, GSD resolution claims, spinning-disk pinhole/speed specs — UNKNOWN on pages read.
- Working-distance and container constraints (plate/dish thickness, cover-glass #1.5 requirement, condenser WD limits such as the 70 mm S-condenser figure seen only on third-party mirrors) — UNKNOWN from official pages read; verify against the operating manual for the exact build.
- Operating-manual procedures (safety, maintenance intervals, decontamination, transport/storage environment) — manual not read; UNKNOWN here.

## 6. Verification log (exactly what I checked)

- Read the four sources in §2 and transcribed only values visible there; did not open the brochure's linked flyers, manuals, or third-party PDFs.
- Cross-checked the contrast-method lists between the spec table and FAQ and recorded the discrepancy explicitly rather than merging them.
- Left every numeric performance field without an official figure as UNKNOWN.
