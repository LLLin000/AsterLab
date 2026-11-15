//! Deterministic mix arithmetic derived from a plate plan.
//!
//! These functions are pure: no store, no transaction, no Run fact. Callers
//! keep the authored plate document as the source of truth and render the
//! returned numbers as derived values.

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

fn checked_groups<'a, T>(
    groups: &'a [T],
    name: impl Fn(&T) -> &str,
    wells: impl Fn(&T) -> u32,
) -> Result<(), String> {
    if groups.is_empty() {
        return Err("at least one group is required".into());
    }
    let mut seen = std::collections::HashSet::new();
    for group in groups {
        let group_name = name(group).trim();
        if group_name.is_empty() {
            return Err("group name cannot be empty".into());
        }
        if !seen.insert(group_name.to_owned()) {
            return Err(format!("group {group_name} is duplicated"));
        }
        if wells(group) == 0 {
            return Err(format!("group {group_name} needs at least one well"));
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
}
