# K2.1 Field Evidence Matrix — two representative modules

- 日期:2026-11-05
- 状态:agent-verified draft; **not** SourceChecked, not ExpertReviewed.
- Scope: only `op.liquid_transfer` and `op.absorbance_measurement`, plus their two draft instrument profiles.
- Rule: every substantive field must point to a source locator or be explicitly UNKNOWN. `contract` means the field is defined by the K1 schema, not a scientific claim.

## 1. `op.liquid_transfer` v1

| Field | Value in draft | Source / locator | Applicability | Status |
|---|---|---|---|---|
| `action_type` | `liquid_transfer` | K1 contract; Opentrons Liquid Control | manual reference; automated is mapping | contract |
| `inputs.source` | sample | Opentrons Liquid Control; ATCC subculture | any liquid source | agent-verified |
| `inputs.destination` | container | Opentrons Liquid Control; ATCC subculture | tube/plate/flask | agent-verified |
| `inputs.tip` | consumable | Eppendorf Research plus manual §§2.2, 3.4 (via instrument audit) | single-use tip | agent-verified |
| `outputs.transferred` | sample | K1 transition model; ATCC dispense step | no concentration change implied | contract |
| `sample_transition` | source → transferred, `aliquoted` | K1 transition model; ATCC harvest/dispense | source loses volume; destination gains portion | contract + agent-verified |
| `parameter.volume` | number, µL, min 0.1, max 10000 | Eppendorf Research plus family span 0.1 µL–10 mL (manual §1.1) | family-level; per-model range must be checked | research-draft |
| `precondition.tip` | clean, compatible, single-use | Eppendorf manual §§2.2, 3.4; Opentrons tip policy | manual and automated | agent-verified |
| `precondition.volume` | within fitted pipette range | Eppendorf manual §6; Opentrons transfer capacity | per model | agent-verified |
| `failure.cross_contamination` | aerosol/carryover | Eppendorf manual; ATCC troubleshooting; Opentrons tip policy | all transfers | research-draft |
| `failure.volume_error` | delivered ≠ requested | Eppendorf manual §§5.5, 6; ISO 8655-6 test basis | all transfers | research-draft |
| `instrument.requirement` | pipette, volume_range exists; actual volume checked per use | Eppendorf manual; K1 capability requirement | class-level, not a model; no family-wide range claim | contract |
| `data.delivered_volume` | record field | K1 `record_fields`; Opentrons/ISO 8655 | run record; not a plan default | contract |

### UNKNOWN for liquid transfer

- Actual pipette model, serial number, calibration status and date.
- Actual tip lot and fit; low-retention need for a specific liquid.
- Liquid density/viscosity/surface tension and temperature.
- Automated liquid-handler deck/labware mapping.
- Whether the lab SOP overrides the manufacturer's error limits.

## 2. `op.absorbance_measurement` v1

| Field | Value in draft | Source / locator | Applicability | Status |
|---|---|---|---|---|
| `action_type` | `absorbance_measurement` | K1 contract; Multiskan SkyHigh spec | endpoint/spectral absorbance only | contract |
| `inputs.plate_in` | prepared plate | Multiskan spec (plates); Dojindo/TMB assay protocols | 96/384 draft | agent-verified |
| `outputs.plate_out` | same plate, preserved | Multiskan spec; optical measurement | not returned to pre-reagent state | agent-verified |
| `outputs.raw_measurement` | raw OD matrix | Multiskan spec; K1 readout | no derived biology | contract + agent-verified |
| `sample_transition` | plate_in → plate_out, `preserved` | Multiskan spec; assay protocols | optical non-consumptive | agent-verified |
| `parameter.wavelength` | number, nm, 200–1000 | Multiskan spec, wavelength range | instrument envelope; assay selects exact λ | agent-verified |
| `parameter.plate_format` | 96 or 384 | Multiskan spec, plates | draft profile; other formats UNKNOWN | agent-verified |
| `parameter.blank_wells` | optional text | Dojindo CCK-8 protocol (blank/background) | same-plate blank; subtraction downstream | research-draft |
| `control.blank_wells` | required blank | Dojindo CCK-8; TMB ELISA | assay-specific; not universal | research-draft |
| `readout.raw` | absorbance | Multiskan spec; K1 readout | raw OD only | agent-verified |
| `readout.not_implied` | viability/cell number/concentration | Dojindo CCK-8; TMB protocol | must not be auto-derived | agent-verified |
| `failure.out_of_range` | saturated/non-linear | Multiskan linearity at 450 nm; Dojindo | 96-well flat-bottom caveat | research-draft |
| `instrument.requirement` | plate_reader; wavelength, plate format | Multiskan spec; K1 capability requirement | class-level, not a model | contract |
| `data.instrument_id` | record field | K1 record_fields | run record; not a plan default | contract |

### UNKNOWN for absorbance

- Shaking speed/amplitude/duration; incubation uniformity.
- Kinetic/continuous-read timing.
- SkanIt software version; calibration/QC status.
- Exact assay-specific blank treatment and standard curve.
- Whether the actual lab plate reader is the SkyHigh model/configuration.

## 3. Draft instrument profiles

### 3.1 `pipette.eppendorf_research_plus.single_channel.100_1000` v1

| Field | Value | Source | Applicability | Status |
|---|---|---|---|---|
| model | Research plus, single-channel, 100–1000 µL | Eppendorf manual §6.1/§7.1 | one specific variant | agent-verified |
| volume range | 100–1000 µL | manual §6.1 | this variant only | agent-verified |
| channels | 1 | manual §6.1 | this variant only | agent-verified |
| tip family | epT.I.P.S. 50–1000 µL | manual §6.1 test-tip column | this variant | agent-verified |
| accuracy/precision | not included | manual §6.1 table | needs separate typed fields | UNKNOWN / not in JSON |
| calibration | not included | lab equipment record | per physical unit/date | N/A (Run/equipment) |
| container support | unspecified | no support list in draft | any call returns Unknown if requested | UNKNOWN |

### 3.2 `plate_reader.thermo_multiskan_skyhigh` v1

| Field | Value | Source | Applicability | Status |
|---|---|---|---|---|
| wavelength | 200–1000 nm | spec table | instrument envelope | agent-verified |
| plate formats | 6/12/24/48/96/384 | spec table | draft partial support list | agent-verified |
| readout range | up to 4 Abs | spec table | instrument range | agent-verified |
| linearity 450 | 0–2.5 Abs | spec table + footnote | 96-well flat-bottom / quartz cuvette | agent-verified |
| conditions | linearity caveat | spec table footnote | profile capability condition | agent-verified |
| shaking | unknown | spec says linear only | no numeric value | UNKNOWN |
| software/calibration | unknown | manual not read | per instrument | UNKNOWN |
| container support | partial | spec plate list | absence = Unknown, not Incompatible | agent-verified |

## 4. Review status rule

- These rows are **agent-verified**, not independently human-verified.
- A second person must check each row against the URL/locator before any status can move toward `SourceChecked`.
- `Compatible` remains impossible for both JSON drafts because:
  - `review_status` is `research_draft`;
  - no `ReviewRecord` is present;
  - the plate reader has a capability condition.
- The K2 engine tests prove this behavior.

## 5. Gaps that still require a model change

- The JSON profiles do not yet type accuracy/precision, calibration, actual serial number or configuration.
- The liquid-transfer profile is one variant; the catalog must not merge the full Research plus family into one range.
- The absorbance profile's plate support is partial; do not convert absence into incompatibility.
- `OperationModule` `conditions` are strings; typed condition satisfaction is future work.
