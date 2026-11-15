//! Deterministic arithmetic derived from a plate plan, a batch of measured
//! sample concentrations, or an in vivo dose schedule.
//!
//! These functions are pure: no store, no transaction, no Run fact. Callers
//! keep the authored document as the source of truth and render the returned
//! numbers as derived values. Measurements and targets stay separate fields, so
//! a dropdown default never becomes a reported measurement.

/// One GelMA mastermix group: a group name, its target % w/v and its actual
/// well count (technical replicates included).
#[derive(Debug, Clone, PartialEq)]
pub struct GelMastermixGroup {
    pub name: String,
    pub gelma_percent_wv: f64,
    pub wells: u32,
}

/// Inputs for `gel_mastermix`. Volumes are microlitres; percents are % w/v
/// (g per 100 mL).
#[derive(Debug, Clone, PartialEq)]
pub struct GelMastermixInput {
    pub volume_per_well_ul: f64,
    pub excess_ratio: f64,
    pub lap_stock_percent_wv: f64,
    pub lap_final_percent_wv: f64,
    pub groups: Vec<GelMastermixGroup>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct GelMastermixGroupOutput {
    pub name: String,
    pub wells: u32,
    pub gelma_percent_wv: f64,
    pub volume_ul: f64,
    pub gelma_mass_mg: f64,
    pub lap_stock_volume_ul: f64,
    pub solvent_volume_ul: f64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct GelMastermixOutput {
    pub groups: Vec<GelMastermixGroupOutput>,
    pub volume_ul: f64,
    pub gelma_mass_mg: f64,
    pub lap_stock_volume_ul: f64,
    pub solvent_volume_ul: f64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CellSeedingGroup {
    pub name: String,
    pub wells: u32,
}

/// Inputs for `cell_seeding`. `factor_*_mg_per_ml` may both be zero, which
/// means no factor is added.
#[derive(Debug, Clone, PartialEq)]
pub struct CellSeedingInput {
    pub density_cells_per_well: f64,
    pub volume_per_well_ul: f64,
    pub excess_ratio: f64,
    pub factor_stock_mg_per_ml: f64,
    pub factor_final_mg_per_ml: f64,
    pub groups: Vec<CellSeedingGroup>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CellSeedingGroupOutput {
    pub name: String,
    pub wells: u32,
    pub total_cells: u64,
    pub suspension_volume_ul: f64,
    pub factor_stock_volume_ul: f64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct CellSeedingOutput {
    pub groups: Vec<CellSeedingGroupOutput>,
    pub total_cells: u64,
    pub suspension_volume_ul: f64,
    pub factor_stock_volume_ul: f64,
}

/// One plate group: its authored name and its actual well count.
#[derive(Debug, Clone, PartialEq)]
pub struct PlateMixGroup {
    pub name: String,
    pub wells: u32,
}

/// One group's target from `mix_gelma_percent_by_group`. `percent` is `None`
/// when the document value is not a number, so Core reports it instead of the
/// caller guessing.
#[derive(Debug, Clone, PartialEq)]
pub struct PlateMixGelPercent {
    pub name: String,
    pub percent: Option<f64>,
}

/// A plate's GelMA plan. `None` on a lap field means the document omitted it.
#[derive(Debug, Clone, PartialEq)]
pub struct PlateMixGelPlan {
    pub lap_stock_percent_wv: Option<f64>,
    pub lap_final_percent_wv: Option<f64>,
    pub percent_by_group: Vec<PlateMixGelPercent>,
}

/// A plate's cell seeding plan. Both factor concentrations absent means no
/// factor is added; exactly one present is an error.
#[derive(Debug, Clone, PartialEq)]
pub struct PlateMixCellPlan {
    pub density_per_well: Option<f64>,
    pub factor_stock_mg_per_ml: Option<f64>,
    pub factor_final_mg_per_ml: Option<f64>,
}

/// The plate `mix_*` fields after unit normalisation. `None` means the field is
/// absent from the document: Core owns every default and every rejection, so a
/// caller never has to invent `0.0`.
#[derive(Debug, Clone, PartialEq)]
pub struct PlateMixInput {
    pub volume_per_well_ul: Option<f64>,
    pub excess_ratio: Option<f64>,
    pub gel: Option<PlateMixGelPlan>,
    pub cells: Option<PlateMixCellPlan>,
    pub groups: Vec<PlateMixGroup>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct PlateMixOutput {
    pub gel: Option<GelMastermixOutput>,
    pub cells: Option<CellSeedingOutput>,
}

/// Inputs for `runtime_normalize`: recompute the live mix volume when the
/// actual well count differs from the frozen plan.
#[derive(Debug, Clone, PartialEq)]
pub struct RuntimeNormalizeInput {
    pub planned_wells: u32,
    pub actual_wells: u32,
    pub volume_per_well_ul: f64,
    pub excess_ratio: f64,
    pub dead_volume_ul: f64,
    pub prepared_volume_ul: f64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct RuntimeNormalizeOutput {
    pub planned_volume_ul: f64,
    pub required_volume_ul: f64,
    pub dead_volume_ul: f64,
    /// Extra volume to prepare when the actual plate needs more than planned.
    pub additional_volume_ul: f64,
    /// Prepared volume left unused when the actual plate needs less.
    pub unused_volume_ul: f64,
}

fn finite(value: f64, field: &str) -> Result<f64, String> {
    if !value.is_finite() {
        return Err(format!("{field} must be a finite number"));
    }
    Ok(value)
}

fn positive(value: f64, field: &str) -> Result<f64, String> {
    if !(finite(value, field)? > 0.0) {
        return Err(format!("{field} must be positive"));
    }
    Ok(value)
}

fn non_negative(value: f64, field: &str) -> Result<f64, String> {
    if !(finite(value, field)? >= 0.0) {
        return Err(format!("{field} must be non-negative"));
    }
    Ok(value)
}

fn checked_names<'a>(names: impl Iterator<Item = &'a str>, noun: &str) -> Result<(), String> {
    let mut seen = std::collections::HashSet::new();
    for name in names {
        let name = name.trim();
        if name.is_empty() {
            return Err(format!("{noun} name cannot be empty"));
        }
        if !seen.insert(name.to_owned()) {
            return Err(format!("{noun} {name} is duplicated"));
        }
    }
    Ok(())
}

fn checked_groups<'a, T>(
    groups: &'a [T],
    name: impl Fn(&T) -> &str,
    wells: impl Fn(&T) -> u32,
) -> Result<(), String> {
    if groups.is_empty() {
        return Err("at least one group is required".into());
    }
    checked_names(groups.iter().map(&name), "group")?;
    for group in groups {
        if wells(group) == 0 {
            return Err(format!(
                "group {} needs at least one well",
                name(group).trim()
            ));
        }
    }
    Ok(())
}

/// Compute the GelMA mastermix per group.
///
/// `% w/v` means grams per 100 mL, so `mass_mg = volume_ul * percent / 100`.
/// The LAP stock volume is `volume * final_percent / stock_percent`. Solvent
/// volume balances the tube: total volume minus the LAP stock. Powder volume
/// is treated as negligible, which the caller must state as an assumption.
pub fn gel_mastermix(input: &GelMastermixInput) -> Result<GelMastermixOutput, String> {
    let volume_per_well = positive(input.volume_per_well_ul, "volume_per_well_ul")?;
    let excess = non_negative(input.excess_ratio, "excess_ratio")?;
    let lap_stock = positive(input.lap_stock_percent_wv, "lap_stock_percent_wv")?;
    let lap_final = non_negative(input.lap_final_percent_wv, "lap_final_percent_wv")?;
    if lap_final > lap_stock {
        return Err("lap_final_percent_wv cannot exceed lap_stock_percent_wv".into());
    }
    checked_groups(
        &input.groups,
        |group| group.name.as_str(),
        |group| group.wells,
    )?;

    let mut groups = Vec::with_capacity(input.groups.len());
    let mut total_volume_ul = 0.0;
    let mut total_gelma_mass_mg = 0.0;
    let mut total_lap_stock_volume_ul = 0.0;
    let mut total_solvent_volume_ul = 0.0;
    for group in &input.groups {
        let percent = positive(group.gelma_percent_wv, "gelma_percent_wv")?;
        if percent > 100.0 {
            return Err(format!(
                "group {} gelma_percent_wv cannot exceed 100",
                group.name
            ));
        }
        let volume_ul = volume_per_well * f64::from(group.wells) * (1.0 + excess);
        let gelma_mass_mg = volume_ul * percent / 100.0;
        let lap_stock_volume_ul = volume_ul * lap_final / lap_stock;
        let solvent_volume_ul = volume_ul - lap_stock_volume_ul;
        total_volume_ul += volume_ul;
        total_gelma_mass_mg += gelma_mass_mg;
        total_lap_stock_volume_ul += lap_stock_volume_ul;
        total_solvent_volume_ul += solvent_volume_ul;
        groups.push(GelMastermixGroupOutput {
            name: group.name.clone(),
            wells: group.wells,
            gelma_percent_wv: percent,
            volume_ul,
            gelma_mass_mg,
            lap_stock_volume_ul,
            solvent_volume_ul,
        });
    }
    Ok(GelMastermixOutput {
        groups,
        volume_ul: total_volume_ul,
        gelma_mass_mg: total_gelma_mass_mg,
        lap_stock_volume_ul: total_lap_stock_volume_ul,
        solvent_volume_ul: total_solvent_volume_ul,
    })
}

fn factor_volume(
    suspension_volume_ul: f64,
    stock_mg_per_ml: f64,
    final_mg_per_ml: f64,
) -> Result<f64, String> {
    if stock_mg_per_ml <= 0.0 {
        return Ok(0.0);
    }
    if final_mg_per_ml > stock_mg_per_ml {
        return Err("factor_final_mg_per_ml cannot exceed factor_stock_mg_per_ml".into());
    }
    Ok(suspension_volume_ul * final_mg_per_ml / stock_mg_per_ml)
}

/// Compute the cell suspension and factor addition per group. Cell counts are
/// rounded up because a partial cell cannot be seeded.
pub fn cell_seeding(input: &CellSeedingInput) -> Result<CellSeedingOutput, String> {
    let density = positive(input.density_cells_per_well, "density_cells_per_well")?;
    let volume_per_well = positive(input.volume_per_well_ul, "volume_per_well_ul")?;
    let excess = non_negative(input.excess_ratio, "excess_ratio")?;
    let stock = non_negative(input.factor_stock_mg_per_ml, "factor_stock_mg_per_ml")?;
    let final_concentration = non_negative(input.factor_final_mg_per_ml, "factor_final_mg_per_ml")?;
    if final_concentration > 0.0 && stock == 0.0 {
        return Err("factor_stock_mg_per_ml is required when a factor is added".into());
    }
    checked_groups(
        &input.groups,
        |group| group.name.as_str(),
        |group| group.wells,
    )?;

    let mut groups = Vec::with_capacity(input.groups.len());
    let mut total_cells = 0_u64;
    let mut total_suspension_volume_ul = 0.0;
    let mut total_factor_stock_volume_ul = 0.0;
    for group in &input.groups {
        let wells = f64::from(group.wells);
        let suspension_volume_ul = volume_per_well * wells * (1.0 + excess);
        let cells = (density * wells * (1.0 + excess)).ceil();
        if cells > u32::MAX as f64 {
            return Err(format!("group {} cell count is too large", group.name));
        }
        let factor_stock_volume_ul = factor_volume(suspension_volume_ul, stock, final_concentration)?;
        total_cells += cells as u64;
        total_suspension_volume_ul += suspension_volume_ul;
        total_factor_stock_volume_ul += factor_stock_volume_ul;
        groups.push(CellSeedingGroupOutput {
            name: group.name.clone(),
            wells: group.wells,
            total_cells: cells as u64,
            suspension_volume_ul,
            factor_stock_volume_ul,
        });
    }
    Ok(CellSeedingOutput {
        groups,
        total_cells,
        suspension_volume_ul: total_suspension_volume_ul,
        factor_stock_volume_ul: total_factor_stock_volume_ul,
    })
}

/// Compute a plate's plan from its authored `mix_*` fields.
///
/// This is the single authority for the plate mix numbers: the GelMA mastermix
/// and the cell seeding rows are the same `gel_mastermix`/`cell_seeding`
/// results, and every document rule (a missing lap percent, a group without a
/// target, a factor above its stock) is a rejection here. A plate that names no
/// mix field and sets no volume returns no plan.
pub fn plate_mix_plan(input: &PlateMixInput) -> Result<PlateMixOutput, String> {
    if input.gel.is_none() && input.cells.is_none() {
        if input.volume_per_well_ul.is_some() || input.excess_ratio.is_some() {
            return Err("mix_volume_per_well and mix_excess_ratio need a gel or cell plan".into());
        }
        return Ok(PlateMixOutput {
            gel: None,
            cells: None,
        });
    }
    let volume_per_well_ul = positive(
        input
            .volume_per_well_ul
            .ok_or("mix plan needs mix_volume_per_well")?,
        "mix_volume_per_well",
    )?;
    let excess_ratio = match input.excess_ratio {
        Some(value) => non_negative(value, "mix_excess_ratio")?,
        None => 0.0,
    };
    checked_groups(
        &input.groups,
        |group| group.name.as_str(),
        |group| group.wells,
    )?;

    let gel = match &input.gel {
        Some(plan) => Some(plate_gel_mastermix(plan, volume_per_well_ul, excess_ratio, &input.groups)?),
        None => None,
    };
    let cells = match &input.cells {
        Some(plan) => Some(plate_cell_seeding(plan, volume_per_well_ul, excess_ratio, &input.groups)?),
        None => None,
    };
    Ok(PlateMixOutput { gel, cells })
}

fn plate_gel_mastermix(
    plan: &PlateMixGelPlan,
    volume_per_well_ul: f64,
    excess_ratio: f64,
    groups: &[PlateMixGroup],
) -> Result<GelMastermixOutput, String> {
    let lap_stock_percent_wv = plan
        .lap_stock_percent_wv
        .ok_or("gel plan needs mix_lap_stock_percent")?;
    let lap_final_percent_wv = plan
        .lap_final_percent_wv
        .ok_or("gel plan needs mix_lap_final_percent")?;
    // The two guards below repeat the arithmetic checks inside `gel_mastermix`
    // so the message names the document field the author edits.
    if !(lap_stock_percent_wv > 0.0) {
        return Err("mix_lap_stock_percent must be positive".into());
    }
    if lap_final_percent_wv > lap_stock_percent_wv {
        return Err("mix_lap_final_percent cannot exceed mix_lap_stock_percent".into());
    }
    let mut gel_groups = Vec::with_capacity(groups.len());
    for group in groups {
        let name = group.name.trim();
        let entry = plan
            .percent_by_group
            .iter()
            .find(|entry| entry.name.trim() == name)
            .ok_or_else(|| format!("mix_gelma_percent_by_group is missing group {name}"))?;
        let percent = entry
            .percent
            .ok_or_else(|| format!("group {name} GelMA percent must be a number"))?;
        if !(percent.is_finite() && percent > 0.0 && percent <= 100.0) {
            return Err(format!("group {name} GelMA percent must be in (0, 100]"));
        }
        gel_groups.push(GelMastermixGroup {
            name: group.name.clone(),
            gelma_percent_wv: percent,
            wells: group.wells,
        });
    }
    for entry in &plan.percent_by_group {
        let name = entry.name.trim();
        if !groups.iter().any(|group| group.name.trim() == name) {
            return Err(format!(
                "mix_gelma_percent_by_group names unknown group {name}"
            ));
        }
    }
    gel_mastermix(&GelMastermixInput {
        volume_per_well_ul,
        excess_ratio,
        lap_stock_percent_wv,
        lap_final_percent_wv,
        groups: gel_groups,
    })
}

fn plate_cell_seeding(
    plan: &PlateMixCellPlan,
    volume_per_well_ul: f64,
    excess_ratio: f64,
    groups: &[PlateMixGroup],
) -> Result<CellSeedingOutput, String> {
    let density_cells_per_well = plan
        .density_per_well
        .ok_or("cell plan needs mix_cell_density")?;
    // Same reason as the gel plan: name the document field the author edits.
    if !(density_cells_per_well > 0.0) {
        return Err("mix_cell_density must be positive".into());
    }
    let (factor_stock_mg_per_ml, factor_final_mg_per_ml) =
        match (plan.factor_stock_mg_per_ml, plan.factor_final_mg_per_ml) {
            (Some(stock), Some(final_concentration)) => (stock, final_concentration),
            (None, None) => (0.0, 0.0),
            _ => {
                return Err(
                    "factor stock and final concentration must be provided together".into(),
                )
            }
        };
    if factor_stock_mg_per_ml <= 0.0 && factor_final_mg_per_ml > 0.0 {
        return Err("factor stock concentration must be positive".into());
    }
    if factor_stock_mg_per_ml > 0.0 && factor_final_mg_per_ml > factor_stock_mg_per_ml {
        return Err("factor final concentration cannot exceed stock concentration".into());
    }
    let groups = groups
        .iter()
        .map(|group| CellSeedingGroup {
            name: group.name.clone(),
            wells: group.wells,
        })
        .collect();
    cell_seeding(&CellSeedingInput {
        density_cells_per_well,
        volume_per_well_ul,
        excess_ratio,
        factor_stock_mg_per_ml,
        factor_final_mg_per_ml,
        groups,
    })
}

/// Recompute the live mix volume from the actual well count. The dead volume
/// is added once per prepared tube; excess applies only to the per-well
/// working volume, never to the dead volume.
pub fn runtime_normalize(
    input: &RuntimeNormalizeInput,
) -> Result<RuntimeNormalizeOutput, String> {
    let volume_per_well = positive(input.volume_per_well_ul, "volume_per_well_ul")?;
    let excess = non_negative(input.excess_ratio, "excess_ratio")?;
    let dead_volume = non_negative(input.dead_volume_ul, "dead_volume_ul")?;
    let prepared = non_negative(input.prepared_volume_ul, "prepared_volume_ul")?;
    let working = |wells: u32| {
        volume_per_well * f64::from(wells) * (1.0 + excess) + dead_volume
    };
    let planned_volume_ul = working(input.planned_wells);
    let required_volume_ul = working(input.actual_wells);
    let delta = required_volume_ul - prepared;
    Ok(RuntimeNormalizeOutput {
        planned_volume_ul,
        required_volume_ul,
        dead_volume_ul: dead_volume,
        additional_volume_ul: delta.max(0.0),
        unused_volume_ul: (-delta).max(0.0),
    })
}

/// A mass concentration unit.
///
/// This is a different dimension from the molar units the dilution tool speaks
/// (`M`/`mM`/`µM`/`nM`): nothing here converts between the two, and the parser
/// rejects every molar spelling, so a mass concentration can never be fed into
/// the molar `C1V1 = C2V2` path as if the two were interchangeable.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MassConcentrationUnit {
    MgPerMl,
    UgPerMl,
    NgPerUl,
}

impl MassConcentrationUnit {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::MgPerMl => "mg/mL",
            Self::UgPerMl => "µg/mL",
            Self::NgPerUl => "ng/µL",
        }
    }

    /// How many `ng/µL` one unit of this concentration is.
    ///
    /// `1 mg/mL = 10^6 ng / 10^3 µL = 1000 ng/µL` and
    /// `1 µg/mL = 10^3 ng / 10^3 µL = 1 ng/µL`.
    pub fn ng_per_ul_factor(self) -> f64 {
        match self {
            Self::MgPerMl => 1000.0,
            Self::UgPerMl | Self::NgPerUl => 1.0,
        }
    }
}

impl std::str::FromStr for MassConcentrationUnit {
    type Err = String;

    fn from_str(text: &str) -> Result<Self, Self::Err> {
        match text.trim() {
            "mg/mL" => Ok(Self::MgPerMl),
            "µg/mL" => Ok(Self::UgPerMl),
            "ng/µL" => Ok(Self::NgPerUl),
            other => Err(format!(
                "{other} is not a mass concentration unit (mg/mL, µg/mL, ng/µL); \
                 molar units are a separate dimension"
            )),
        }
    }
}

/// A measured or specified mass concentration. `value` is in `unit`, never in
/// moles: the molar conversion needs a molecular weight and belongs to another
/// tool.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct MassConcentration {
    pub value: f64,
    pub unit: MassConcentrationUnit,
}

impl MassConcentration {
    pub fn ng_per_ul(&self) -> Result<f64, String> {
        let value = non_negative(self.value, "mass concentration")?;
        Ok(value * self.unit.ng_per_ul_factor())
    }
}

/// One sample's measured mass concentration, as its own value: a measurement
/// is not the target concentration.
#[derive(Debug, Clone, PartialEq)]
pub struct SampleMeasurement {
    pub name: String,
    pub concentration: MassConcentration,
}

/// Inputs for `sample_normalize_batch`. `target_mass_ng` is the mass every
/// sample must contribute to one reaction; `reaction_volume_ul` is the total
/// volume that reaction may occupy, so it caps the sample volume.
#[derive(Debug, Clone, PartialEq)]
pub struct SampleNormalizeBatchInput {
    pub target_mass_ng: f64,
    pub reaction_volume_ul: f64,
    pub samples: Vec<SampleMeasurement>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct SampleNormalizeRow {
    pub name: String,
    pub concentration_ng_per_ul: f64,
    /// Volume the target mass needs at this concentration; it may exceed the
    /// reaction volume, which is what `insufficient` reports.
    pub required_sample_volume_ul: f64,
    /// Volume actually pipetted: the required volume, capped by the reaction.
    pub sample_volume_ul: f64,
    pub diluent_volume_ul: f64,
    /// Mass the pipetted volume delivers; it falls below `target_mass_ng`
    /// exactly when `insufficient` is set.
    pub achieved_mass_ng: f64,
    pub insufficient: bool,
}

#[derive(Debug, Clone, PartialEq)]
pub struct SampleNormalizeBatchOutput {
    pub rows: Vec<SampleNormalizeRow>,
}

/// Balance every sample to the same mass in the same reaction volume.
///
/// Unequal measured concentrations mean unequal sample volumes; the diluent
/// makes up the rest. A sample too dilute to reach the target mass inside the
/// reaction volume is reported with `insufficient`, not silently rounded up:
/// the row then carries the capped volume and the mass it can actually deliver.
pub fn sample_normalize_batch(
    input: &SampleNormalizeBatchInput,
) -> Result<SampleNormalizeBatchOutput, String> {
    let target_mass = positive(input.target_mass_ng, "target_mass_ng")?;
    let reaction_volume = positive(input.reaction_volume_ul, "reaction_volume_ul")?;
    if input.samples.is_empty() {
        return Err("at least one sample is required".into());
    }
    checked_names(
        input.samples.iter().map(|sample| sample.name.as_str()),
        "sample",
    )?;

    let mut rows = Vec::with_capacity(input.samples.len());
    for sample in &input.samples {
        let concentration = sample.concentration.ng_per_ul()?;
        if concentration <= 0.0 {
            return Err(format!(
                "sample {} concentration must be positive",
                sample.name
            ));
        }
        let required_sample_volume_ul = target_mass / concentration;
        let insufficient = required_sample_volume_ul > reaction_volume;
        let sample_volume_ul = required_sample_volume_ul.min(reaction_volume);
        rows.push(SampleNormalizeRow {
            name: sample.name.clone(),
            concentration_ng_per_ul: concentration,
            required_sample_volume_ul,
            sample_volume_ul,
            diluent_volume_ul: reaction_volume - sample_volume_ul,
            achieved_mass_ng: sample_volume_ul * concentration,
            insufficient,
        });
    }
    Ok(SampleNormalizeBatchOutput { rows })
}

/// Inputs for `in_vivo_dosing`. Weights are grams; the dose is `mg/kg`; the
/// dose volume is `mL/kg`; `overage_ratio` is the dead-volume margin prepared
/// on top of the summed injections; `stock_concentration` is the stock the
/// working solution is drawn from.
#[derive(Debug, Clone, PartialEq)]
pub struct InVivoDosingInput {
    pub body_weights_g: Vec<f64>,
    pub dose_mg_per_kg: f64,
    pub dose_volume_ml_per_kg: f64,
    pub overage_ratio: f64,
    pub stock_concentration: MassConcentration,
}

#[derive(Debug, Clone, PartialEq)]
pub struct InVivoDoseRow {
    /// 1-based position in the submitted weight list; animals carry no names.
    pub index: usize,
    pub body_weight_g: f64,
    pub dose_mass_mg: f64,
    pub injection_volume_ul: f64,
}

#[derive(Debug, Clone, PartialEq)]
pub struct InVivoDosingOutput {
    pub rows: Vec<InVivoDoseRow>,
    pub total_body_weight_g: f64,
    pub total_dose_mass_mg: f64,
    pub total_injection_volume_ul: f64,
    /// Totals after `overage_ratio`, i.e. what the working solution must hold.
    pub prepared_dose_mass_mg: f64,
    pub preparation_volume_ul: f64,
    pub stock_volume_ul: f64,
    pub diluent_volume_ul: f64,
}

/// Compute each animal's injection from its body weight and the preparation
/// the summed doses need.
///
/// `weight_g × mL/kg` is already microlitres: `g × 10^-3 = kg` and
/// `kg × mL/kg = mL = 10^3 µL`, so the factors cancel. Overage scales the
/// prepared tube, never an individual injection. A stock too dilute to fit the
/// prepared mass into the prepared volume is a rejection, because diluting it
/// would break the `mL/kg` dose volume.
pub fn in_vivo_dosing(input: &InVivoDosingInput) -> Result<InVivoDosingOutput, String> {
    if input.body_weights_g.is_empty() {
        return Err("at least one body weight is required".into());
    }
    let dose_mg_per_kg = positive(input.dose_mg_per_kg, "dose_mg_per_kg")?;
    let dose_volume_ml_per_kg =
        positive(input.dose_volume_ml_per_kg, "dose_volume_ml_per_kg")?;
    let overage = non_negative(input.overage_ratio, "overage_ratio")?;
    let stock_ng_per_ul = input.stock_concentration.ng_per_ul()?;
    if stock_ng_per_ul <= 0.0 {
        return Err("stock concentration must be positive".into());
    }

    let mut rows = Vec::with_capacity(input.body_weights_g.len());
    let mut total_body_weight_g = 0.0;
    let mut total_dose_mass_mg = 0.0;
    let mut total_injection_volume_ul = 0.0;
    for (offset, weight) in input.body_weights_g.iter().enumerate() {
        let body_weight_g = positive(*weight, "body_weight_g")?;
        let dose_mass_mg = body_weight_g / 1000.0 * dose_mg_per_kg;
        let injection_volume_ul = body_weight_g * dose_volume_ml_per_kg;
        total_body_weight_g += body_weight_g;
        total_dose_mass_mg += dose_mass_mg;
        total_injection_volume_ul += injection_volume_ul;
        rows.push(InVivoDoseRow {
            index: offset + 1,
            body_weight_g,
            dose_mass_mg,
            injection_volume_ul,
        });
    }

    let prepared_dose_mass_mg = total_dose_mass_mg * (1.0 + overage);
    let preparation_volume_ul = total_injection_volume_ul * (1.0 + overage);
    let stock_volume_ul = prepared_dose_mass_mg * 1_000_000.0 / stock_ng_per_ul;
    if stock_volume_ul > preparation_volume_ul {
        return Err(
            "stock concentration is too low: the dose does not fit the prepared volume".into(),
        );
    }
    Ok(InVivoDosingOutput {
        rows,
        total_body_weight_g,
        total_dose_mass_mg,
        total_injection_volume_ul,
        prepared_dose_mass_mg,
        preparation_volume_ul,
        stock_volume_ul,
        diluent_volume_ul: preparation_volume_ul - stock_volume_ul,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn gelma_groups() -> Vec<GelMastermixGroup> {
        vec![
            GelMastermixGroup {
                name: "3%".into(),
                gelma_percent_wv: 3.0,
                wells: 3,
            },
            GelMastermixGroup {
                name: "5%".into(),
                gelma_percent_wv: 5.0,
                wells: 3,
            },
            GelMastermixGroup {
                name: "7%".into(),
                gelma_percent_wv: 7.0,
                wells: 3,
            },
        ]
    }

    fn seeding_groups() -> Vec<CellSeedingGroup> {
        ["3%", "5%", "7%"]
            .into_iter()
            .map(|name| CellSeedingGroup {
                name: name.into(),
                wells: 3,
            })
            .collect()
    }

    fn close(actual: f64, expected: f64) {
        assert!(
            (actual - expected).abs() < 1e-9,
            "expected {expected}, got {actual}"
        );
    }

    #[test]
    fn huvec_gel_mastermix_scales_with_wells_and_excess() {
        let output = gel_mastermix(&GelMastermixInput {
            volume_per_well_ul: 200.0,
            excess_ratio: 0.1,
            lap_stock_percent_wv: 0.5,
            lap_final_percent_wv: 0.05,
            groups: gelma_groups(),
        })
        .expect("valid HUVEC plan");

        assert_eq!(output.groups.len(), 3);
        let first = &output.groups[0];
        close(first.volume_ul, 660.0);
        close(first.gelma_mass_mg, 19.8);
        close(first.lap_stock_volume_ul, 66.0);
        close(first.solvent_volume_ul, 594.0);
        close(output.groups[2].gelma_mass_mg, 46.2);
        close(output.volume_ul, 1980.0);
        close(output.gelma_mass_mg, 99.0);
        close(output.lap_stock_volume_ul, 198.0);
        close(output.solvent_volume_ul, 1782.0);
    }

    #[test]
    fn gel_mastermix_rejects_bad_percent_and_duplicate_groups() {
        let bad_percent = GelMastermixInput {
            volume_per_well_ul: 200.0,
            excess_ratio: 0.0,
            lap_stock_percent_wv: 0.5,
            lap_final_percent_wv: 0.05,
            groups: vec![GelMastermixGroup {
                name: "3%".into(),
                gelma_percent_wv: 0.0,
                wells: 3,
            }],
        };
        assert!(gel_mastermix(&bad_percent).is_err());

        let duplicate = GelMastermixInput {
            groups: vec![
                GelMastermixGroup {
                    name: "5%".into(),
                    gelma_percent_wv: 5.0,
                    wells: 3,
                },
                GelMastermixGroup {
                    name: "5%".into(),
                    gelma_percent_wv: 5.0,
                    wells: 3,
                },
            ],
            ..bad_percent
        };
        assert!(gel_mastermix(&duplicate).is_err());

        let lap = GelMastermixInput {
            lap_final_percent_wv: 0.6,
            ..duplicate
        };
        assert!(gel_mastermix(&lap).is_err());
    }

    #[test]
    fn cell_seeding_counts_cells_and_factor_volume() {
        let output = cell_seeding(&CellSeedingInput {
            density_cells_per_well: 20_000.0,
            volume_per_well_ul: 200.0,
            excess_ratio: 0.1,
            factor_stock_mg_per_ml: 0.01,
            factor_final_mg_per_ml: 0.00005,
            groups: seeding_groups(),
        })
        .expect("valid HUVEC seeding plan");

        close(output.groups[0].suspension_volume_ul, 660.0);
        assert_eq!(output.groups[0].total_cells, 66_000);
        close(output.groups[0].factor_stock_volume_ul, 3.3);
        assert_eq!(output.total_cells, 198_000);
        close(output.suspension_volume_ul, 1980.0);
        close(output.factor_stock_volume_ul, 9.9);
    }

    #[test]
    fn cell_seeding_without_factor_and_rejects_factor_above_stock() {
        let output = cell_seeding(&CellSeedingInput {
            density_cells_per_well: 5_000.0,
            volume_per_well_ul: 100.0,
            excess_ratio: 0.0,
            factor_stock_mg_per_ml: 0.0,
            factor_final_mg_per_ml: 0.0,
            groups: vec![CellSeedingGroup {
                name: "control".into(),
                wells: 6,
            }],
        })
        .expect("no-factor seeding");
        assert_eq!(output.total_cells, 30_000);
        close(output.factor_stock_volume_ul, 0.0);

        let above_stock = CellSeedingInput {
            factor_stock_mg_per_ml: 0.001,
            factor_final_mg_per_ml: 0.01,
            ..CellSeedingInput {
                density_cells_per_well: 5_000.0,
                volume_per_well_ul: 100.0,
                excess_ratio: 0.0,
                factor_stock_mg_per_ml: 0.0,
                factor_final_mg_per_ml: 0.0,
                groups: vec![CellSeedingGroup {
                    name: "control".into(),
                    wells: 6,
                }],
            }
        };
        assert!(cell_seeding(&above_stock).is_err());
    }

    fn plate_mix_groups() -> Vec<PlateMixGroup> {
        ["3%", "5%", "7%"]
            .into_iter()
            .map(|name| PlateMixGroup {
                name: name.into(),
                wells: 3,
            })
            .collect()
    }

    /// The shared HUVEC fixture: 3 groups x 3 wells on a 12-well plate,
    /// 200 uL/well, 10% excess, 0.5% LAP stock diluted to 0.05%, 20k cells/well
    /// and a factor dosed from a 10 ug/mL stock to 50 ng/mL. The Go parity test
    /// (`internal/asterdoc`) pins the same numbers through the native bridge.
    fn huvec_plate_mix() -> PlateMixInput {
        PlateMixInput {
            volume_per_well_ul: Some(200.0),
            excess_ratio: Some(0.1),
            gel: Some(PlateMixGelPlan {
                lap_stock_percent_wv: Some(0.5),
                lap_final_percent_wv: Some(0.05),
                percent_by_group: vec![
                    PlateMixGelPercent {
                        name: "3%".into(),
                        percent: Some(3.0),
                    },
                    PlateMixGelPercent {
                        name: "5%".into(),
                        percent: Some(5.0),
                    },
                    PlateMixGelPercent {
                        name: "7%".into(),
                        percent: Some(7.0),
                    },
                ],
            }),
            cells: Some(PlateMixCellPlan {
                density_per_well: Some(20_000.0),
                factor_stock_mg_per_ml: Some(0.01),
                factor_final_mg_per_ml: Some(0.00005),
            }),
            groups: plate_mix_groups(),
        }
    }

    #[test]
    fn plate_mix_plan_derives_the_huvec_rows() {
        let output = plate_mix_plan(&huvec_plate_mix()).expect("valid HUVEC plate");

        let gel = output.gel.expect("gel plan");
        assert_eq!(gel.groups.len(), 3);
        close(gel.groups[0].volume_ul, 660.0);
        close(gel.groups[0].gelma_mass_mg, 19.8);
        close(gel.groups[0].lap_stock_volume_ul, 66.0);
        close(gel.groups[0].solvent_volume_ul, 594.0);
        close(gel.volume_ul, 1980.0);
        close(gel.gelma_mass_mg, 99.0);
        close(gel.lap_stock_volume_ul, 198.0);
        close(gel.solvent_volume_ul, 1782.0);

        let cells = output.cells.expect("cell plan");
        assert_eq!(cells.groups.len(), 3);
        assert_eq!(cells.groups[0].total_cells, 66_000);
        assert_eq!(cells.total_cells, 198_000);
        close(cells.suspension_volume_ul, 1980.0);
        close(cells.factor_stock_volume_ul, 9.9);
    }

    /// The acceptance case: 3%/5%/7%, nine wells, 0.5 mL per well, 10% excess.
    #[test]
    fn plate_mix_plan_covers_the_half_millilitre_plate() {
        let output = plate_mix_plan(&PlateMixInput {
            volume_per_well_ul: Some(500.0),
            cells: Some(PlateMixCellPlan {
                density_per_well: Some(20_000.0),
                factor_stock_mg_per_ml: None,
                factor_final_mg_per_ml: None,
            }),
            ..huvec_plate_mix()
        })
        .expect("valid 0.5 mL plate");

        let gel = output.gel.expect("gel plan");
        close(gel.groups[0].volume_ul, 1650.0);
        close(gel.groups[1].gelma_mass_mg, 82.5);
        close(gel.groups[2].gelma_mass_mg, 115.5);
        close(gel.volume_ul, 4950.0);
        close(gel.gelma_mass_mg, 247.5);
        close(gel.lap_stock_volume_ul, 495.0);
        close(gel.solvent_volume_ul, 4455.0);

        let cells = output.cells.expect("cell plan");
        assert_eq!(cells.groups[1].total_cells, 66_000);
        assert_eq!(cells.total_cells, 198_000);
        close(cells.suspension_volume_ul, 4950.0);
        close(cells.factor_stock_volume_ul, 0.0);
    }

    #[test]
    fn plate_mix_plan_stays_empty_without_mix_fields() {
        let output = plate_mix_plan(&PlateMixInput {
            volume_per_well_ul: None,
            excess_ratio: None,
            gel: None,
            cells: None,
            groups: plate_mix_groups(),
        })
        .expect("plate without a mix plan");
        assert!(output.gel.is_none());
        assert!(output.cells.is_none());
    }

    #[test]
    fn plate_mix_plan_rejects_incomplete_and_inconsistent_plans() {
        let volume_without_plan = PlateMixInput {
            gel: None,
            cells: None,
            ..huvec_plate_mix()
        };
        assert!(plate_mix_plan(&volume_without_plan).is_err());

        let mut missing_percent = huvec_plate_mix();
        missing_percent
            .gel
            .as_mut()
            .unwrap()
            .percent_by_group
            .pop();
        assert!(plate_mix_plan(&missing_percent).is_err());

        let mut unknown_group = huvec_plate_mix();
        unknown_group
            .gel
            .as_mut()
            .unwrap()
            .percent_by_group
            .push(PlateMixGelPercent {
                name: "9%".into(),
                percent: Some(9.0),
            });
        assert!(plate_mix_plan(&unknown_group).is_err());

        let mut non_numeric_percent = huvec_plate_mix();
        non_numeric_percent.gel.as_mut().unwrap().percent_by_group[0].percent = None;
        assert!(plate_mix_plan(&non_numeric_percent).is_err());

        let mut lap_above_stock = huvec_plate_mix();
        lap_above_stock.gel.as_mut().unwrap().lap_final_percent_wv = Some(0.6);
        assert!(plate_mix_plan(&lap_above_stock).is_err());

        let mut missing_lap_stock = huvec_plate_mix();
        missing_lap_stock.gel.as_mut().unwrap().lap_stock_percent_wv = None;
        assert!(plate_mix_plan(&missing_lap_stock).is_err());

        let mut factor_above_stock = huvec_plate_mix();
        factor_above_stock
            .cells
            .as_mut()
            .unwrap()
            .factor_final_mg_per_ml = Some(0.02);
        assert!(plate_mix_plan(&factor_above_stock).is_err());

        let mut factor_without_final = huvec_plate_mix();
        factor_without_final
            .cells
            .as_mut()
            .unwrap()
            .factor_final_mg_per_ml = None;
        assert!(plate_mix_plan(&factor_without_final).is_err());

        let mut missing_density = huvec_plate_mix();
        missing_density.cells.as_mut().unwrap().density_per_well = None;
        assert!(plate_mix_plan(&missing_density).is_err());

        let mut non_finite_volume = huvec_plate_mix();
        non_finite_volume.volume_per_well_ul = Some(f64::NAN);
        assert!(plate_mix_plan(&non_finite_volume).is_err());

        let mut no_groups = huvec_plate_mix();
        no_groups.groups.clear();
        assert!(plate_mix_plan(&no_groups).is_err());
    }

    #[test]
    fn runtime_normalize_recomputes_from_actual_wells_and_dead_volume() {
        let under = runtime_normalize(&RuntimeNormalizeInput {
            planned_wells: 9,
            actual_wells: 7,
            volume_per_well_ul: 200.0,
            excess_ratio: 0.1,
            dead_volume_ul: 50.0,
            prepared_volume_ul: 2000.0,
        })
        .expect("valid runtime plan");
        close(under.planned_volume_ul, 2030.0);
        close(under.required_volume_ul, 1590.0);
        close(under.additional_volume_ul, 0.0);
        close(under.unused_volume_ul, 410.0);

        let over = runtime_normalize(&RuntimeNormalizeInput {
            actual_wells: 10,
            prepared_volume_ul: 2030.0,
            ..RuntimeNormalizeInput {
                planned_wells: 9,
                actual_wells: 9,
                volume_per_well_ul: 200.0,
                excess_ratio: 0.1,
                dead_volume_ul: 50.0,
                prepared_volume_ul: 0.0,
            }
        })
        .expect("valid runtime plan");
        close(over.required_volume_ul, 2250.0);
        close(over.additional_volume_ul, 220.0);
        close(over.unused_volume_ul, 0.0);
    }

    #[test]
    fn runtime_normalize_rejects_non_finite_and_negative_inputs() {
        let invalid = RuntimeNormalizeInput {
            planned_wells: 9,
            actual_wells: 9,
            volume_per_well_ul: f64::NAN,
            excess_ratio: 0.0,
            dead_volume_ul: 0.0,
            prepared_volume_ul: 0.0,
        };
        assert!(runtime_normalize(&invalid).is_err());
        let negative_dead = RuntimeNormalizeInput {
            volume_per_well_ul: 200.0,
            dead_volume_ul: -1.0,
            ..invalid
        };
        assert!(runtime_normalize(&negative_dead).is_err());
    }

    fn mass(value: f64, unit: MassConcentrationUnit) -> MassConcentration {
        MassConcentration { value, unit }
    }

    fn batch_samples(concentrations: &[(&str, f64)]) -> Vec<SampleMeasurement> {
        concentrations
            .iter()
            .map(|(name, value)| SampleMeasurement {
                name: (*name).into(),
                concentration: mass(*value, MassConcentrationUnit::NgPerUl),
            })
            .collect()
    }

    #[test]
    fn mass_concentration_units_convert_to_ng_per_ul() {
        close(
            mass(1.0, MassConcentrationUnit::MgPerMl)
                .ng_per_ul()
                .expect("finite"),
            1000.0,
        );
        close(
            mass(2.5, MassConcentrationUnit::UgPerMl)
                .ng_per_ul()
                .expect("finite"),
            2.5,
        );
        close(
            mass(750.0, MassConcentrationUnit::NgPerUl)
                .ng_per_ul()
                .expect("finite"),
            750.0,
        );
        assert_eq!(
            "mg/mL".parse::<MassConcentrationUnit>(),
            Ok(MassConcentrationUnit::MgPerMl)
        );
        assert_eq!(
            "µg/mL".parse::<MassConcentrationUnit>(),
            Ok(MassConcentrationUnit::UgPerMl)
        );
        assert_eq!(
            "ng/µL".parse::<MassConcentrationUnit>(),
            Ok(MassConcentrationUnit::NgPerUl)
        );
        assert_eq!(MassConcentrationUnit::MgPerMl.as_str(), "mg/mL");
    }

    /// Molar units are a different dimension: the mass unit parser must never
    /// hand back a unit the dilution math would treat as moles.
    #[test]
    fn mass_concentration_rejects_molar_units_and_invalid_values() {
        for molar in ["M", "mM", "µM", "nM", "mg/L", ""] {
            let error = molar
                .parse::<MassConcentrationUnit>()
                .expect_err("not a mass concentration unit");
            assert!(
                error.contains("mass concentration"),
                "unexpected message for {molar}: {error}"
            );
        }
        assert!(mass(-1.0, MassConcentrationUnit::NgPerUl)
            .ng_per_ul()
            .is_err());
        assert!(mass(f64::INFINITY, MassConcentrationUnit::MgPerMl)
            .ng_per_ul()
            .is_err());
        assert!(mass(f64::NAN, MassConcentrationUnit::MgPerMl)
            .ng_per_ul()
            .is_err());
    }

    #[test]
    fn sample_normalize_batch_balances_each_sample_to_the_target_mass() {
        let output = sample_normalize_batch(&SampleNormalizeBatchInput {
            target_mass_ng: 1000.0,
            reaction_volume_ul: 20.0,
            samples: batch_samples(&[("A1", 50.0), ("A2", 200.0), ("A3", 100.0)]),
        })
        .expect("valid batch");

        assert_eq!(output.rows.len(), 3);
        let boundary = &output.rows[0];
        close(boundary.required_sample_volume_ul, 20.0);
        close(boundary.sample_volume_ul, 20.0);
        close(boundary.diluent_volume_ul, 0.0);
        close(boundary.achieved_mass_ng, 1000.0);
        assert!(!boundary.insufficient);

        close(output.rows[1].sample_volume_ul, 5.0);
        close(output.rows[1].diluent_volume_ul, 15.0);
        close(output.rows[1].achieved_mass_ng, 1000.0);
        assert!(!output.rows[1].insufficient);

        close(output.rows[2].sample_volume_ul, 10.0);
        close(output.rows[2].diluent_volume_ul, 10.0);
        close(output.rows[2].achieved_mass_ng, 1000.0);
    }

    #[test]
    fn sample_normalize_batch_flags_a_low_concentration_sample() {
        let output = sample_normalize_batch(&SampleNormalizeBatchInput {
            target_mass_ng: 1000.0,
            reaction_volume_ul: 20.0,
            samples: batch_samples(&[("low", 25.0), ("ok", 100.0)]),
        })
        .expect("valid batch");

        let low = &output.rows[0];
        close(low.concentration_ng_per_ul, 25.0);
        close(low.required_sample_volume_ul, 40.0);
        close(low.sample_volume_ul, 20.0);
        close(low.diluent_volume_ul, 0.0);
        close(low.achieved_mass_ng, 500.0);
        assert!(low.insufficient);

        assert!(!output.rows[1].insufficient);
        close(output.rows[1].achieved_mass_ng, 1000.0);
    }

    #[test]
    fn sample_normalize_batch_converts_mixed_units_before_balancing() {
        let output = sample_normalize_batch(&SampleNormalizeBatchInput {
            target_mass_ng: 1000.0,
            reaction_volume_ul: 20.0,
            samples: vec![
                SampleMeasurement {
                    name: "mg".into(),
                    concentration: mass(0.05, MassConcentrationUnit::MgPerMl),
                },
                SampleMeasurement {
                    name: "ug".into(),
                    concentration: mass(200.0, MassConcentrationUnit::UgPerMl),
                },
            ],
        })
        .expect("valid batch");

        close(output.rows[0].concentration_ng_per_ul, 50.0);
        close(output.rows[0].sample_volume_ul, 20.0);
        close(output.rows[0].diluent_volume_ul, 0.0);
        close(output.rows[1].concentration_ng_per_ul, 200.0);
        close(output.rows[1].sample_volume_ul, 5.0);
        close(output.rows[1].diluent_volume_ul, 15.0);
    }

    #[test]
    fn sample_normalize_batch_rejects_invalid_inputs() {
        let base = SampleNormalizeBatchInput {
            target_mass_ng: 1000.0,
            reaction_volume_ul: 20.0,
            samples: batch_samples(&[("A1", 50.0)]),
        };
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            target_mass_ng: 0.0,
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            target_mass_ng: f64::NAN,
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            reaction_volume_ul: -1.0,
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            samples: Vec::new(),
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            samples: batch_samples(&[("A1", 50.0), ("A1", 60.0)]),
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            samples: batch_samples(&[("  ", 50.0)]),
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            samples: batch_samples(&[("A1", 0.0)]),
            ..base.clone()
        })
        .is_err());
        assert!(sample_normalize_batch(&SampleNormalizeBatchInput {
            samples: batch_samples(&[("A1", -5.0)]),
            ..base
        })
        .is_err());
    }

    fn in_vivo_input() -> InVivoDosingInput {
        InVivoDosingInput {
            body_weights_g: vec![20.0, 25.0, 30.0],
            dose_mg_per_kg: 10.0,
            dose_volume_ml_per_kg: 10.0,
            overage_ratio: 0.2,
            stock_concentration: mass(5.0, MassConcentrationUnit::MgPerMl),
        }
    }

    #[test]
    fn in_vivo_dosing_computes_each_injection_and_the_preparation_totals() {
        let output = in_vivo_dosing(&in_vivo_input()).expect("valid dosing plan");

        assert_eq!(output.rows.len(), 3);
        close(output.rows[0].body_weight_g, 20.0);
        close(output.rows[0].dose_mass_mg, 0.2);
        close(output.rows[0].injection_volume_ul, 200.0);
        close(output.rows[1].dose_mass_mg, 0.25);
        close(output.rows[1].injection_volume_ul, 250.0);
        close(output.rows[2].dose_mass_mg, 0.3);
        close(output.rows[2].injection_volume_ul, 300.0);

        close(output.total_body_weight_g, 75.0);
        close(output.total_dose_mass_mg, 0.75);
        close(output.total_injection_volume_ul, 750.0);
        close(output.prepared_dose_mass_mg, 0.9);
        close(output.preparation_volume_ul, 900.0);
        close(output.stock_volume_ul, 180.0);
        close(output.diluent_volume_ul, 720.0);
    }

    #[test]
    fn in_vivo_dosing_rejects_invalid_inputs() {
        assert!(in_vivo_dosing(&InVivoDosingInput {
            body_weights_g: Vec::new(),
            ..in_vivo_input()
        })
        .is_err());
        assert!(in_vivo_dosing(&InVivoDosingInput {
            body_weights_g: vec![0.0],
            ..in_vivo_input()
        })
        .is_err());
        assert!(in_vivo_dosing(&InVivoDosingInput {
            body_weights_g: vec![20.0, f64::NAN],
            ..in_vivo_input()
        })
        .is_err());
        assert!(in_vivo_dosing(&InVivoDosingInput {
            dose_mg_per_kg: 0.0,
            ..in_vivo_input()
        })
        .is_err());
        assert!(in_vivo_dosing(&InVivoDosingInput {
            dose_volume_ml_per_kg: -1.0,
            ..in_vivo_input()
        })
        .is_err());
        assert!(in_vivo_dosing(&InVivoDosingInput {
            overage_ratio: -0.1,
            ..in_vivo_input()
        })
        .is_err());
        assert!(in_vivo_dosing(&InVivoDosingInput {
            stock_concentration: mass(0.0, MassConcentrationUnit::MgPerMl),
            ..in_vivo_input()
        })
        .is_err());
        // The stock is too dilute to fit the dose inside the injection volume.
        assert!(in_vivo_dosing(&InVivoDosingInput {
            stock_concentration: mass(0.5, MassConcentrationUnit::MgPerMl),
            ..in_vivo_input()
        })
        .is_err());
    }
}
