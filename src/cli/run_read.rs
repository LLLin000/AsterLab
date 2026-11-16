use super::*;

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct PlateCreate {
    pub(crate) run_id: String,
    pub(crate) name: String,
    pub(crate) format: String,
    pub(crate) rows: u32,
    pub(crate) columns: u32,
    #[serde(default)]
    pub(crate) assay_id: Option<String>,
    #[serde(default)]
    pub(crate) timepoint: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct WellSet {
    pub(crate) plate_id: String,
    pub(crate) well_id: String,
    #[serde(default)]
    pub(crate) expected_revision: Option<i64>,
    #[serde(default)]
    pub(crate) group_label: String,
    #[serde(default)]
    pub(crate) sample_id: String,
    #[serde(default)]
    pub(crate) note: String,
    /// Planned detection target label for this position.
    #[serde(default)]
    pub(crate) target: Option<String>,
    /// Planned role of this position in the detection layout.
    #[serde(default)]
    pub(crate) well_role: Option<WellRole>,
    /// Planned dilution factor; unitless and positive.
    #[serde(default)]
    pub(crate) dilution_factor: Option<f64>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct NoteCreate {
    pub(crate) run_id: String,
    pub(crate) content: String,
    #[serde(default)]
    pub(crate) event_time: Option<String>,
    #[serde(default)]
    pub(crate) observation: String,
    #[serde(default)]
    pub(crate) result: String,
    #[serde(default)]
    pub(crate) deviation: String,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventMaterialUse {
    pub(crate) material: String,
    #[serde(default)]
    pub(crate) planned_output_id: Option<String>,
    #[serde(default)]
    pub(crate) amount: Option<f64>,
    #[serde(default)]
    pub(crate) unit: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventMeasurement {
    pub(crate) endpoint: String,
    pub(crate) value: f64,
    #[serde(default)]
    pub(crate) unit: Option<String>,
    #[serde(default)]
    pub(crate) sample_id: Option<String>,
    #[serde(default)]
    pub(crate) row: Option<usize>,
    #[serde(default)]
    pub(crate) column: Option<usize>,
    #[serde(default)]
    pub(crate) group: Option<String>,
    #[serde(default)]
    pub(crate) replicate: Option<usize>,
    #[serde(default)]
    pub(crate) status: Option<String>,
    #[serde(default)]
    pub(crate) field: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventReadout {
    #[serde(default)]
    pub(crate) readout_id: Option<String>,
    pub(crate) endpoint: String,
    #[serde(default)]
    pub(crate) value: Option<f64>,
    #[serde(default)]
    pub(crate) qualifier: Option<String>,
    #[serde(default)]
    pub(crate) unit: Option<String>,
    #[serde(default)]
    pub(crate) sample_id: Option<String>,
    #[serde(default = "raw_result_kind")]
    pub(crate) result_kind: String,
    #[serde(default)]
    pub(crate) method: Option<String>,
    #[serde(default)]
    pub(crate) derived_from: Vec<String>,
}
pub(crate) fn raw_result_kind() -> String { "raw".into() }
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventCheck {
    pub(crate) check_id: String,
    pub(crate) status: String,
    #[serde(default)]
    pub(crate) note: Option<String>,
    #[serde(default)]
    pub(crate) checked_by: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventControl {
    pub(crate) role: String,
    #[serde(default)]
    pub(crate) subtype: Option<String>,
    pub(crate) status: String,
    #[serde(default)]
    pub(crate) control_id: Option<String>,
    #[serde(default)]
    pub(crate) sample_id: Option<String>,
    #[serde(default)]
    pub(crate) experimental_unit: Option<String>,
    #[serde(default)]
    pub(crate) expected: Option<String>,
    #[serde(default)]
    pub(crate) observed: Option<String>,
    #[serde(default)]
    pub(crate) note: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventActualParameter {
    pub(crate) key: String,
    pub(crate) value: Value,
    #[serde(default)]
    pub(crate) unit: Option<String>,
    #[serde(default)]
    pub(crate) source: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EventLineage {
    pub(crate) from_kind: String,
    pub(crate) from_id: String,
    pub(crate) to_kind: String,
    pub(crate) to_id: String,
    pub(crate) relation: String,
    #[serde(default)]
    pub(crate) note: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ExecutionEventCreate {
    pub(crate) run_id: String,
    pub(crate) event_type: String,
    pub(crate) content: String,
    #[serde(default)]
    pub(crate) occurred_at: Option<String>,
    #[serde(default)]
    pub(crate) stage: Option<String>,
    #[serde(default)]
    pub(crate) timepoint: Option<String>,
    #[serde(default)]
    pub(crate) step_status: Option<String>,
    #[serde(default)]
    pub(crate) ad_hoc_step: bool,
    #[serde(default)]
    pub(crate) protocol_step_path: Option<String>,
    #[serde(default)]
    pub(crate) protocol_version_id: Option<String>,
    #[serde(default)]
    pub(crate) assay_id: Option<String>,
    #[serde(default)]
    pub(crate) planned_output_id: Option<String>,
    /// Optional pantry ledger reference. Absent keeps the event unchanged;
    /// present must resolve to a stored item, and only produces an advisory.
    #[serde(default)]
    pub(crate) pantry_item_id: Option<String>,
    #[serde(default)]
    pub(crate) task_key: Option<String>,
    #[serde(default)]
    pub(crate) sample_ids: Vec<String>,
    /// Parent sample the `sample_ids` were derived from. Omitted for events
    /// without sample lineage; does not gate any existing write.
    #[serde(default)]
    pub(crate) parent_sample_id: Option<String>,
    /// Sample this event's `sample_ids` were aliquoted from. Separate from
    /// `parent_sample_id` because a parent remains the source across aliquots.
    #[serde(default)]
    pub(crate) aliquot_from: Option<String>,
    /// Marks the event as recorded under a blind view. `None` only for legacy
    /// rows; the read model treats a missing flag as "not masked".
    #[serde(default)]
    pub(crate) blind_masked: Option<bool>,
    /// Sample-level attempt for one (step, sample) identity. Omitted for
    /// step-level or legacy events; the read model then derives the attempt.
    #[serde(default)]
    pub(crate) attempt: Option<u32>,
    #[serde(default)]
    pub(crate) materials: Vec<EventMaterialUse>,
    #[serde(default)]
    pub(crate) measurements: Vec<EventMeasurement>,
    #[serde(default)]
    pub(crate) batch_id: Option<String>,
    #[serde(default)]
    pub(crate) schema_id: Option<String>,
    #[serde(default)]
    pub(crate) schema_version: Option<u32>,
    #[serde(default)]
    pub(crate) result_cells: Vec<Value>,
    #[serde(default)]
    pub(crate) readouts: Vec<EventReadout>,
    #[serde(default)]
    pub(crate) checks: Vec<EventCheck>,
    #[serde(default)]
    pub(crate) controls: Vec<EventControl>,
    #[serde(default)]
    pub(crate) lineage: Vec<EventLineage>,
    #[serde(default)]
    pub(crate) actual_parameters: Vec<EventActualParameter>,
    #[serde(default)]
    pub(crate) operator: Option<String>,
    #[serde(default)]
    pub(crate) deviation: Option<String>,
    #[serde(default)]
    pub(crate) operation_id: Option<String>,
    #[serde(default)]
    pub(crate) operation_payload: Option<Value>,
    // Internal marker for the native SaveTaskChecks/CompleteTask path. The
    // public execution_event.create schema deliberately does not expose it.
    #[serde(default)]
    pub(crate) check_ack: bool,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct FormulaCreate {
    pub(crate) project_id: String,
    pub(crate) title: String,
    pub(crate) expression: String,
    #[serde(default)]
    pub(crate) variables: String,
    #[serde(default)]
    pub(crate) notes: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct FormulaRevise {
    pub(crate) id: String,
    pub(crate) expected_revision: i64,
    #[serde(default)]
    pub(crate) title: Option<String>,
    #[serde(default)]
    pub(crate) expression: Option<String>,
    #[serde(default)]
    pub(crate) variables: Option<String>,
    #[serde(default)]
    pub(crate) notes: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct TableCreate {
    pub(crate) project_id: String,
    pub(crate) name: String,
    pub(crate) columns: Vec<String>,
    pub(crate) rows: Vec<Vec<String>>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct TableRevise {
    pub(crate) id: String,
    pub(crate) expected_revision: i64,
    pub(crate) name: String,
    pub(crate) columns: Vec<String>,
    pub(crate) rows: Vec<Vec<String>>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct DilutionCalc {
    pub(crate) stock_concentration: f64,
    pub(crate) stock_unit: String,
    pub(crate) target_concentration: f64,
    pub(crate) target_unit: String,
    pub(crate) final_volume: f64,
    pub(crate) volume_unit: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SolutionCalc {
    pub(crate) concentration_mol_l: f64,
    pub(crate) final_volume: f64,
    pub(crate) volume_unit: String,
    pub(crate) molecular_weight_g_mol: f64,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SerialDilutionCalc {
    pub(crate) initial_concentration: f64,
    pub(crate) concentration_unit: String,
    pub(crate) transfer_volume: f64,
    pub(crate) diluent_volume: f64,
    pub(crate) steps: u32,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ReactionComponent {
    pub(crate) name: String,
    pub(crate) volume_ul: f64,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ReactionVolumeCalc {
    pub(crate) target_volume_ul: f64,
    pub(crate) components: Vec<ReactionComponent>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct GelMastermixCalcGroup {
    pub(crate) name: String,
    pub(crate) gelma_percent_wv: f64,
    pub(crate) wells: u32,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct GelMastermixCalc {
    pub(crate) volume_per_well_ul: f64,
    #[serde(default)]
    pub(crate) excess_ratio: f64,
    pub(crate) lap_stock_percent_wv: f64,
    pub(crate) lap_final_percent_wv: f64,
    pub(crate) groups: Vec<GelMastermixCalcGroup>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct CellSeedingCalcGroup {
    pub(crate) name: String,
    pub(crate) wells: u32,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct CellSeedingCalc {
    pub(crate) density_cells_per_well: f64,
    pub(crate) volume_per_well_ul: f64,
    #[serde(default)]
    pub(crate) excess_ratio: f64,
    #[serde(default)]
    pub(crate) factor_stock_mg_per_ml: f64,
    #[serde(default)]
    pub(crate) factor_final_mg_per_ml: f64,
    pub(crate) groups: Vec<CellSeedingCalcGroup>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RuntimeNormalizeCalc {
    pub(crate) planned_wells: u32,
    pub(crate) actual_wells: u32,
    pub(crate) volume_per_well_ul: f64,
    #[serde(default)]
    pub(crate) excess_ratio: f64,
    pub(crate) dead_volume_ul: f64,
    pub(crate) prepared_volume_ul: f64,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SampleNormalizeCalcSample {
    pub(crate) name: String,
    pub(crate) concentration: f64,
    pub(crate) unit: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SampleNormalizeBatchCalc {
    pub(crate) target_mass_ng: f64,
    pub(crate) reaction_volume_ul: f64,
    pub(crate) samples: Vec<SampleNormalizeCalcSample>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct InVivoDosingCalc {
    pub(crate) body_weights_g: Vec<f64>,
    pub(crate) dose_mg_per_kg: f64,
    pub(crate) dose_volume_ml_per_kg: f64,
    #[serde(default)]
    pub(crate) overage_ratio: f64,
    pub(crate) stock_concentration: f64,
    pub(crate) stock_unit: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct BiofilmCfuPlate {
    pub(crate) raw_id: String,
    pub(crate) sample_id: String,
    pub(crate) colonies: f64,
    pub(crate) dilution_fractions: Vec<f64>,
    pub(crate) plated_volume: f64,
    pub(crate) plated_unit: String,
    pub(crate) suspension_volume: f64,
    pub(crate) suspension_unit: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct BiofilmCfuCalc {
    pub(crate) plates: Vec<BiofilmCfuPlate>,
    pub(crate) countable_min: f64,
    pub(crate) countable_max: f64,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct StudyImport {
    pub(crate) project: ProjectCreate,
    #[serde(default)]
    pub(crate) designs: Vec<DesignFields>,
    #[serde(default)]
    pub(crate) protocols: Vec<ProtocolBody>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolBody {
    pub(crate) title: String,
    pub(crate) body: String,
}

pub(crate) fn stamp() -> String {
    Utc::now().to_rfc3339()
}
pub(crate) fn make_id() -> String {
    Uuid::new_v4().to_string()
}
pub(crate) fn check_text(value: &str, label: &str, max: usize) -> Result<(), CliError> {
    let s = value.trim();
    if s.is_empty() {
        return Err(CliError::input(format!("{label}不能为空")));
    }
    if s.chars().count() > max {
        return Err(CliError::input(format!("{label}不能超过 {max} 个字符")));
    }
    Ok(())
}
pub(crate) fn validate_groups(groups: &[GroupSpec]) -> Result<(), CliError> {
    let mut names = std::collections::HashSet::new();
    let mut ids = std::collections::HashSet::new();
    for g in groups {
        check_text(&g.id, "组别 ID", 80)?;
        check_text(&g.name, "组别名称", 120)?;
        if !ids.insert(g.id.as_str()) {
            return Err(CliError::input(format!("组别 ID 重复：{}", g.id)));
        }
        if !names.insert(g.name.trim().to_lowercase()) {
            return Err(CliError::input(format!("组别名称重复：{}", g.name)));
        }
        if g.independent_n == Some(0) || g.technical_repeats == Some(0) {
            return Err(CliError::input("n 和技术重复数只能是正整数或留空"));
        }
        if g.condition
            .as_ref()
            .is_some_and(|v| v.chars().count() > 500)
        {
            return Err(CliError::input("处理条件不能超过 500 个字符"));
        }
    }
    Ok(())
}

pub(crate) fn legacy_timepoint_relation(relation: &LegacySampleRelation) -> Option<TimepointRelation> {
    match relation {
        LegacySampleRelation::NewPerTimepoint => {
            Some(TimepointRelation::IndependentSamplePerTimepoint)
        }
        // The old label did not distinguish an experimental unit from a physical sample.
        // Preserve that uncertainty instead of silently choosing one interpretation.
        LegacySampleRelation::SameSampleRepeated => Some(TimepointRelation::Unknown),
        LegacySampleRelation::Unknown
        | LegacySampleRelation::SplitFromAssay
        | LegacySampleRelation::SequentialAfterAssay => None,
    }
}

pub(crate) fn legacy_source_relation(relation: &LegacySampleRelation) -> SourceRelation {
    match relation {
        LegacySampleRelation::SplitFromAssay => SourceRelation::SplitFromAssay,
        LegacySampleRelation::SequentialAfterAssay => SourceRelation::SequentialAfterAssay,
        _ => SourceRelation::Unknown,
    }
}

pub(crate) fn normalize_design(d: &mut DesignFields, previous: Option<&DesignFields>) {
    if d.group_specs.is_empty() && !d.groups.trim().is_empty() {
        d.group_specs = d
            .groups
            .lines()
            .filter(|name| !name.trim().is_empty())
            .map(|name| GroupSpec {
                id: String::new(),
                name: name.trim().to_owned(),
                role: None,
                condition: None,
                independent_n: None,
                technical_repeats: None,
            })
            .collect();
    }

    let mut used_ids = std::collections::HashSet::new();
    for group in &mut d.group_specs {
        if group.id.trim().is_empty() {
            let matches: Vec<&GroupSpec> = previous
                .into_iter()
                .flat_map(|old| old.group_specs.iter())
                .filter(|old| old.name == group.name && !old.id.trim().is_empty())
                .collect();
            if matches.len() == 1 && !used_ids.contains(matches[0].id.as_str()) {
                group.id = matches[0].id.clone();
            } else {
                group.id = make_id();
            }
        }
        used_ids.insert(group.id.clone());
    }

    let groups_by_id: std::collections::HashMap<String, String> = d
        .group_specs
        .iter()
        .map(|group| (group.id.clone(), group.name.clone()))
        .collect();
    let groups_by_name: std::collections::HashMap<String, String> = d
        .group_specs
        .iter()
        .map(|group| (group.name.clone(), group.id.clone()))
        .collect();
    for assay in &mut d.assays {
        if assay.timepoint_relation.is_none() {
            assay.timepoint_relation = Some(
                assay
                    .sample_use_policy
                    .as_ref()
                    .and_then(legacy_timepoint_relation)
                    .unwrap_or(TimepointRelation::Unknown),
            );
        }
        assay.sample_use_policy = None;
        if assay.group_ids.is_empty() {
            if let Some(ids) = assay
                .group_names
                .iter()
                .map(|name| groups_by_name.get(name).cloned())
                .collect::<Option<Vec<_>>>()
            {
                assay.group_ids = ids;
            }
        }
        if !assay.group_ids.is_empty() {
            assay.group_names = assay
                .group_ids
                .iter()
                .filter_map(|id| groups_by_id.get(id).cloned())
                .collect();
        }
    }
    let assay_timepoint_relations: std::collections::HashMap<String, TimepointRelation> = d
        .assays
        .iter()
        .filter_map(|assay| {
            assay
                .timepoint_relation
                .as_ref()
                .map(|relation| (assay.id.clone(), relation.clone()))
        })
        .collect();
    for allocation in &mut d.sample_allocations {
        if allocation.timepoint_relation.is_none() {
            allocation.timepoint_relation = Some(
                legacy_timepoint_relation(&allocation.relation)
                    .or_else(|| assay_timepoint_relations.get(&allocation.assay_id).cloned())
                    .unwrap_or(TimepointRelation::Unknown),
            );
        }
        if allocation.source_relation.is_none() {
            allocation.source_relation = Some(legacy_source_relation(&allocation.relation));
        }
        if allocation.group_id.is_none() {
            allocation.group_id = groups_by_name.get(&allocation.group_name).cloned();
        }
        if allocation.physical_slots_per_group.is_some()
            && allocation.physical_slots_scope.is_none()
        {
            allocation.physical_slots_scope = Some(PhysicalSlotsScope::Unknown);
        }
        if let Some(name) = allocation
            .group_id
            .as_ref()
            .and_then(|id| groups_by_id.get(id))
        {
            allocation.group_name = name.clone();
        }
        allocation.relation = LegacySampleRelation::Unknown;
    }
}

pub(crate) fn load_labware_registry(db: &Connection) -> Result<LabwareRegistry, CliError> {
    let mut registry = default_labware_registry();
    for record in list(db, Some("labware_profile"), None)? {
        let profile: LabwareProfile = serde_json::from_value(record["data"].clone())
            .map_err(|e| CliError::internal(format!("LabwareProfile 数据无效：{e}")))?;
        registry
            .register(profile)
            .map_err(|error| CliError::input(error))?;
    }
    Ok(registry)
}

pub(crate) fn normalize_validate_design_data(
    data: Value,
    previous: Option<&Value>,
) -> Result<Value, CliError> {
    normalize_validate_design_data_with_registry(data, previous, &default_labware_registry())
}

pub(crate) fn normalize_validate_design_data_with_registry(
    data: Value,
    previous: Option<&Value>,
    registry: &LabwareRegistry,
) -> Result<Value, CliError> {
    let mut design: DesignFields = serde_json::from_value(data)
        .map_err(|e| CliError::input(format!("设计数据结构无效：{e}")))?;
    let previous_design = previous
        .map(|value| serde_json::from_value::<DesignFields>(value.clone()))
        .transpose()
        .map_err(|e| CliError::internal(format!("现有设计数据无效：{e}")))?;
    normalize_design(&mut design, previous_design.as_ref());
    validate_design_with_registry(&design, registry)?;
    serde_json::to_value(design).map_err(|e| CliError::internal(e.to_string()))
}

pub(crate) fn run_workspace(db: &Connection, run_id: &str) -> Result<Value, CliError> {
    run_workspace_for_view(
        db,
        run_id,
        asterlab_core::record_projection::ProjectionView::Unblinded,
    )
}

/// Parse the optional `view` selector shared by the CLI and the native bridge.
/// Empty or absent keeps the unblinded record view; `blind` masks group
/// identity only for a frozen Design that declared a blinding basis.
pub(crate) fn parse_projection_view(
    raw: Option<&str>,
) -> Result<asterlab_core::record_projection::ProjectionView, CliError> {
    match raw.map(str::trim).unwrap_or("") {
        "" | "unblinded" => Ok(asterlab_core::record_projection::ProjectionView::Unblinded),
        "blind" => Ok(asterlab_core::record_projection::ProjectionView::Blind),
        other => Err(CliError::input(format!(
            "view 只能是 blind 或 unblinded，收到 {other}"
        ))),
    }
}

pub(crate) fn run_workspace_for_view(
    db: &Connection,
    run_id: &str,
    view: asterlab_core::record_projection::ProjectionView,
) -> Result<Value, CliError> {
    let run = get(db, run_id)?;
    let workspace = run_workspace_typed_for_view(db, run_id, view)?;
    let mut value =
        serde_json::to_value(workspace).map_err(|error| CliError::internal(error.to_string()))?;
    value["creation_audit"] = run["data"]
        .get("creation_audit")
        .cloned()
        .unwrap_or(Value::Null);
    value["asterdoc"] = run["data"]
        .get("asterdoc")
        .cloned()
        .unwrap_or(Value::Null);
    value["design_checks"] = run["data"]
        .get("design_checks")
        .cloned()
        .unwrap_or_else(|| json!([]));
    Ok(value)
}

/// Typed counterpart of [`run_workspace`], used by commands that validate
/// against the frozen Run before writing.
pub(crate) fn run_workspace_typed(
    db: &Connection,
    run_id: &str,
) -> Result<asterlab_core::run_workspace::RunWorkspace, CliError> {
    run_workspace_typed_for_view(
        db,
        run_id,
        asterlab_core::record_projection::ProjectionView::Unblinded,
    )
}

/// Derived-only cache helpers for [`run_workspace_typed_for_view`].
///
/// `projection_cache` never stores authority: rows are keyed by
/// `(entity_id, source_revision, projection_kind)` where the revision is a
/// fingerprint over every Run-scoped input row (the Run record itself plus
/// its events, asset links, plates and wells). Any write to those inputs
/// changes the fingerprint, so a stale row never matches. Every cache path
/// falls back to live computation: a missing table (pre-migration database),
/// a missing or mismatched row, or an unparseable payload all recompute.
pub(crate) fn projection_cache_table_exists(db: &Connection) -> bool {
    db.query_row(
        "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='projection_cache')",
        [],
        |row| row.get::<_, bool>(0),
    )
    .unwrap_or(false)
}

pub(crate) fn projection_source_revision(db: &Connection, run_id: &str) -> Option<i64> {
    let mut stmt = db
        .prepare(
            "SELECT id, revision, updated_at FROM records WHERE id=?1 OR parent_id=?1 OR parent_id IN (SELECT id FROM records WHERE kind='plate' AND parent_id=?1) ORDER BY id",
        )
        .ok()?;
    let rows: Vec<(String, i64, String)> = stmt
        .query_map([run_id], |row| {
            Ok((row.get(0)?, row.get(1)?, row.get(2)?))
        })
        .ok()?
        .filter_map(|row| row.ok())
        .collect();
    if rows.is_empty() {
        return None;
    }
    use sha2::{Digest, Sha256};
    let mut hasher = Sha256::new();
    for (id, revision, updated_at) in &rows {
        hasher.update(id.as_bytes());
        hasher.update([0]);
        hasher.update(revision.to_le_bytes());
        hasher.update([0]);
        hasher.update(updated_at.as_bytes());
        hasher.update([0]);
    }
    let digest = hasher.finalize();
    let mut bytes = [0u8; 8];
    bytes.copy_from_slice(&digest[..8]);
    Some((i64::from_le_bytes(bytes) & i64::MAX) ^ (rows.len() as i64))
}

fn read_cached_workspace(
    db: &Connection,
    run_id: &str,
    source_revision: i64,
    view: asterlab_core::record_projection::ProjectionView,
) -> Option<asterlab_core::run_workspace::RunWorkspace> {
    use asterlab_core::projection_cache::{validate_cached, ProjectionCacheKey, ProjectionKind};
    let kind = match view {
        asterlab_core::record_projection::ProjectionView::Blind => return None,
        asterlab_core::record_projection::ProjectionView::Unblinded => ProjectionKind::RunWorkspace,
    };
    let requested = ProjectionCacheKey::new(run_id, source_revision, kind);
    let (stored_revision, stored_kind, stored_hash, payload_raw): (i64, String, String, String) =
        db.query_row(
            "SELECT source_revision, projection_kind, payload_hash, payload_json FROM projection_cache WHERE entity_id=?1 AND source_revision=?2 AND projection_kind=?3",
            rusqlite::params![run_id, source_revision, kind.as_str()],
            |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?)),
        )
        .ok()?;
    let payload: Value = serde_json::from_str(&payload_raw).ok()?;
    validate_cached(
        &requested,
        run_id,
        stored_revision,
        &stored_kind,
        &stored_hash,
        payload.clone(),
    )?;
    serde_json::from_value(payload).ok()
}

fn write_cached_workspace(
    db: &Connection,
    run_id: &str,
    source_revision: i64,
    workspace: &asterlab_core::run_workspace::RunWorkspace,
) {
    use asterlab_core::projection_cache::{cache_entry, ProjectionCacheKey, ProjectionKind};
    let Ok(payload) = serde_json::to_value(workspace) else {
        return;
    };
    let key = ProjectionCacheKey::new(run_id, source_revision, ProjectionKind::RunWorkspace);
    let entry = cache_entry(key, payload, stamp());
    let payload_raw = serde_json::to_string(&entry.payload).unwrap_or_default();
    if payload_raw.is_empty() {
        return;
    }
    let _ = db.execute(
        "INSERT INTO projection_cache(entity_id, source_revision, projection_kind, payload_json, payload_hash, updated_at) VALUES(?1,?2,?3,?4,?5,?6) ON CONFLICT(entity_id, source_revision, projection_kind) DO UPDATE SET payload_json=excluded.payload_json, payload_hash=excluded.payload_hash, updated_at=excluded.updated_at",
        rusqlite::params![
            entry.key.entity_id,
            entry.key.source_revision,
            entry.key.kind.as_str(),
            payload_raw,
            entry.payload_hash,
            entry.updated_at,
        ],
    );
}
pub(crate) fn run_workspace_typed_for_view(
    db: &Connection,
    run_id: &str,
    view: asterlab_core::record_projection::ProjectionView,
) -> Result<asterlab_core::run_workspace::RunWorkspace, CliError> {
    let run = get(db, run_id)?;
    if run["kind"] != "run" {
        return Err(CliError::input("指定记录不是实验运行"));
    }
    let design_id = run["parent_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("执行批次缺少所属设计"))?;
    let version_id = run["data"]["design_version_id"]
        .as_str()
        .filter(|value| !value.is_empty())
        .ok_or_else(|| CliError::input("执行批次没有冻结设计版本；不能建立 RunWorkspace"))?;
    let version = get(db, version_id)?;
    if version["kind"] != "design_version" {
        return Err(CliError::internal("冻结设计版本记录类型无效"));
    }
    if version["parent_id"].as_str() != Some(design_id) {
        return Err(CliError::input("冻结设计版本不属于本执行批次的设计"));
    }
    if version["project_id"].as_str() != run["project_id"].as_str() {
        return Err(CliError::input("冻结设计版本与执行批次不属于同一项目"));
    }
    let revision = version["data"]["design_revision"]
        .as_i64()
        .ok_or_else(|| CliError::internal("冻结设计版本号无效"))?;
    let snapshot = &version["data"]["snapshot"];
    let project = run["project_id"].as_str();

    let mut events = Vec::new();
    for record in list(db, Some("execution_event"), project)? {
        if record["parent_id"].as_str() != Some(run_id) {
            continue;
        }
        let mut data = record["data"].clone();
        data["id"] = record["id"].clone();
        events.push(data);
    }
    let mut asset_links = Vec::new();
    for record in list(db, Some("asset_link"), project)? {
        if record["parent_id"].as_str() != Some(run_id) {
            continue;
        }
        asset_links.push(record["data"].clone());
    }
    let mut plates = Vec::new();
    for record in list(db, Some("plate"), project)? {
        if record["parent_id"].as_str() != Some(run_id) {
            continue;
        }
        let mut data = record["data"].clone();
        data["id"] = record["id"].clone();
        plates.push(data);
    }
    let plate_ids: HashSet<String> = plates
        .iter()
        .filter_map(|plate| plate["id"].as_str().map(str::to_owned))
        .collect();
    let mut wells_by_plate: HashMap<String, Vec<Value>> = HashMap::new();
    for record in list(db, Some("well"), project)? {
        let Some(plate_id) = record["parent_id"].as_str() else {
            continue;
        };
        if !plate_ids.contains(plate_id) {
            continue;
        }
        let mut data = record["data"].clone();
        if data["well_id"].as_str().is_none() {
            data["well_id"] = record["id"].clone();
        }
        wells_by_plate
            .entry(plate_id.to_owned())
            .or_default()
            .push(data);
    }
    for plate in &mut plates {
        let plate_id = plate["id"].as_str().unwrap_or_default().to_owned();
        plate["wells"] = json!(wells_by_plate.remove(&plate_id).unwrap_or_default());
    }

    let cached_revision = if projection_cache_table_exists(db) {
        projection_source_revision(db, run_id)
    } else {
        None
    };
    if let Some(source_revision) = cached_revision {
        if let Some(cached) = read_cached_workspace(db, run_id, source_revision, view) {
            return Ok(cached);
        }
    }
    let workspace = asterlab_core::run_workspace::build_for_view(
        run_id,
        design_id,
        version_id,
        revision,
        snapshot,
        &run["data"],
        &plates,
        &events,
        &asset_links,
        view,
    )
    .map_err(|error| CliError::internal(error.to_string()))?;
    if let Some(source_revision) = cached_revision {
        if matches!(
            view,
            asterlab_core::record_projection::ProjectionView::Unblinded
        ) {
            write_cached_workspace(db, run_id, source_revision, &workspace);
        }
    }
    Ok(workspace)
}

pub(crate) fn inspect_workflow_tree(input: &InspectTreeInput, data_dir: &Path) -> Result<Value, CliError> {
    let db = open_db(data_dir)?;
    workflow_tree_for_db(
        &db,
        Some(&input.design_id),
        input.revision,
        input.run_id.as_deref(),
    )
}

/// workflow_tree_for_db builds the read-only hierarchy projection for a
/// Design or a frozen Run. Native services call this directly so the GUI and
/// CLI share one Core-driven implementation.
pub(crate) fn workflow_tree_for_db(
    db: &Connection,
    design_id: Option<&str>,
    requested_revision: Option<u64>,
    run_id: Option<&str>,
) -> Result<Value, CliError> {
    let (design_id, revision, source, tasks, design) = if let Some(run_id) = run_id {
        let workspace = run_workspace(db, run_id)?;
        let frozen_design_id = workspace["design_id"]
            .as_str()
            .ok_or_else(|| CliError::internal("Run 缺少设计 ID"))?
            .to_owned();
        if let Some(requested) = design_id {
            if frozen_design_id != requested {
                return Err(CliError::input("design_id 必须等于 Run 的所属设计"));
            }
        }
        let revision = workspace["asterdoc"]["revision"]
            .as_u64()
            .filter(|revision| *revision > 0)
            .or_else(|| workspace["design_revision"].as_u64())
            .ok_or_else(|| CliError::internal("Run 缺少冻结 AsterDoc 版本号"))?;
        if let Some(requested) = requested_revision {
            if requested != revision {
                return Err(CliError::conflict(format!(
                    "Run 冻结的 AsterDoc revision 为 {revision}，请求为 {requested}"
                )));
            }
        }
        let asterdoc = &workspace["asterdoc"];
        let frozen_asterdoc = asterdoc
            .get("revision")
            .and_then(Value::as_u64)
            .is_some_and(|revision| revision > 0)
            || asterdoc
                .get("source")
                .and_then(Value::as_str)
                .is_some_and(|source| !source.trim().is_empty());
        if !frozen_asterdoc {
            // A classic `design.create` Run freezes a DesignVersion but never
            // stores an AsterDoc document. There is no revision to load, so
            // returning an empty tree with an explicit status is the honest
            // answer; only a Run that names a revision must be loadable.
            return Ok(json!({
                "designId": frozen_design_id,
                "revision": revision,
                "runId": run_id,
                "design": false,
                "roots": [],
                "order": [],
                "precedes": {},
                "issues": [format!(
                    "Run 由经典 design.create 冻结，没有 AsterDoc 文档；不存在 AsterDoc revision v{revision}，层级树为空"
                )]
            }));
        }
        let document = load_asterdoc_at(db, &frozen_design_id, revision)?;
        let frozen_hash = workspace["asterdoc"]["source_hash"].as_str().unwrap_or("");
        let stored_hash = document["source_hash"].as_str().unwrap_or("");
        if !frozen_hash.is_empty() && !stored_hash.is_empty() && frozen_hash != stored_hash {
            return Err(CliError::conflict(format!(
                "Run 冻结的 AsterDoc source_hash 与 revision v{revision} 不一致"
            )));
        }
        let source = document["source"].as_str().unwrap_or("").to_owned();
        let tasks = workflow_tasks_from_workspace(&workspace);
        (frozen_design_id, revision, source, tasks, false)
    } else {
        let design_id = design_id.ok_or_else(|| CliError::input("design_id 必填"))?;
        let document = if let Some(revision) = requested_revision {
            load_asterdoc_at(db, design_id, revision)?
        } else {
            load_asterdoc(db, design_id)?
        };
        let revision = document["revision"].as_u64().unwrap_or(0);
        let source = document["source"].as_str().unwrap_or("").to_owned();
        (design_id.to_owned(), revision, source, Vec::new(), true)
    };

    let document = parse_workflow_document(&source)?;
    let blocks = load_workflow_blocks(db, &document)?;
    let mut value = if design {
        asterlab_core::workflow_tree::build_design(&document, &blocks)
    } else {
        asterlab_core::workflow_tree::build_run(&document, &blocks, &tasks)
    };
    value["designId"] = json!(design_id);
    value["revision"] = json!(revision);
    if let Some(run_id) = run_id {
        value["runId"] = json!(run_id);
    }
    Ok(value)
}

pub(crate) fn parse_workflow_document(source: &str) -> Result<Document, CliError> {
    let mut document = Document::default();
    let mut workflow_seen = false;
    for block in asterdoc_blocks(source)? {
        let value = &block.value;
        let kind = value.get("kind").and_then(|value| value.as_str()).unwrap_or("");
        match kind {
            "reference" => {
                let view = value.get("view").and_then(|value| value.as_str()).unwrap_or("");
                if view == "protocol_block" {
                    let id = value.get("id").and_then(|value| value.as_str()).unwrap_or("");
                    let reference = value.get("ref").and_then(|value| value.as_str()).unwrap_or("");
                    if !id.is_empty() && !reference.is_empty() {
                        document
                            .reference_blocks
                            .insert(id.to_owned(), reference.to_owned());
                    }
                }
            }
            "step" => {
                let id = value.get("id").and_then(|value| value.as_str()).unwrap_or("");
                if id.is_empty() {
                    continue;
                }
                let schema = value.get("schema").and_then(|value| value.as_i64()).unwrap_or(0);
                let execution = value
                    .get("execution")
                    .and_then(|value| value.as_str())
                    .unwrap_or("");
                let organizational = schema == 3 && execution == "organizational";
                let mut path = value
                    .get("path")
                    .and_then(|value| value.as_str())
                    .unwrap_or("")
                    .to_owned();
                if organizational && path.is_empty() {
                    path = id.to_owned();
                }
                let mut refs = Vec::new();
                if let Some(uses) = value.get("uses").and_then(|value| value.as_sequence()) {
                    for item in uses {
                        if let Some(reference) =
                            item.get("ref").and_then(|value| value.as_str())
                        {
                            if !reference.is_empty() {
                                refs.push(reference.to_owned());
                            }
                        }
                    }
                }
                document.steps.push(DesignStep {
                    id: id.to_owned(),
                    title: value
                        .get("title")
                        .and_then(|value| value.as_str())
                        .unwrap_or("")
                        .to_owned(),
                    path,
                    order: value.get("order").and_then(|value| value.as_i64()).unwrap_or(0),
                    organizational,
                    refs,
                });
            }
            "workflow" if !workflow_seen => {
                workflow_seen = true;
                if let Some(order) = value
                    .get("business_order")
                    .and_then(|value| value.as_sequence())
                {
                    document.order = order
                        .iter()
                        .filter_map(|value| value.as_str().map(str::to_owned))
                        .collect();
                }
                if let Some(edges) = value.get("edges").and_then(|value| value.as_sequence()) {
                    for edge in edges {
                        let from = edge.get("from").and_then(|value| value.as_str()).unwrap_or("");
                        let to = edge.get("to").and_then(|value| value.as_str()).unwrap_or("");
                        if !from.is_empty() && !to.is_empty() {
                            document.edges.push((from.to_owned(), to.to_owned()));
                        }
                    }
                }
            }
            _ => {}
        }
    }
    Ok(document)
}

pub(crate) fn load_workflow_blocks(
    db: &Connection,
    document: &Document,
) -> Result<HashMap<String, Result<BlockDefinition, String>>, CliError> {
    let mut versions = HashSet::new();
    for step in &document.steps {
        for reference in &step.refs {
            if let Some(version) = document.reference_blocks.get(reference) {
                versions.insert(version.clone());
            }
        }
    }
    let mut blocks = HashMap::new();
    for version in versions {
        let parsed = block_preview(db, &version, &[])
            .map_err(|error| error.message)
            .and_then(workflow_block_definition);
        blocks.insert(version, parsed);
    }
    Ok(blocks)
}

pub(crate) fn workflow_block_definition(value: Value) -> Result<BlockDefinition, String> {
    let steps = value
        .get("steps")
        .and_then(Value::as_array)
        .ok_or_else(|| "ProtocolPreview 缺少 steps".to_owned())?;
    let mut definition = BlockDefinition::default();
    for step in steps {
        let parameters = step
            .get("parameters")
            .and_then(Value::as_array)
            .map(|parameters| {
                parameters
                    .iter()
                    .map(|parameter| BlockParameter {
                        key: parameter
                            .get("slot")
                            .and_then(Value::as_str)
                            .unwrap_or("")
                            .to_owned(),
                        label: parameter
                            .get("label")
                            .and_then(Value::as_str)
                            .unwrap_or("")
                            .to_owned(),
                        value: parameter.get("value").cloned().unwrap_or(Value::Null),
                        unit: parameter
                            .get("unit")
                            .and_then(Value::as_str)
                            .map(str::to_owned),
                    })
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();
        let unresolved_parameters = step
            .get("unresolved_parameter_refs")
            .and_then(Value::as_array)
            .map(|values| {
                values
                    .iter()
                    .filter_map(|value| value.as_str().map(str::to_owned))
                    .collect()
            })
            .unwrap_or_default();
        definition.steps.push(BlockStep {
            use_id: step
                .get("use_id")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            path: step
                .get("path")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            title: step
                .get("title")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            instruction: step
                .get("instruction")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            source_block_version_id: step
                .get("source_block_version_id")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            parameters,
            unresolved_parameters,
        });
    }
    Ok(definition)
}

pub(crate) fn workflow_tasks_from_workspace(workspace: &Value) -> Vec<RunTask> {
    let mut step_details: HashMap<(String, String), (String, String)> = HashMap::new();
    for step in workspace["steps"].as_array().into_iter().flatten() {
        let inner = &step["step"];
        let path = inner["path"].as_str().unwrap_or("").to_owned();
        let assay_id = step["scope"]["assay_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        let title = inner["title"].as_str().unwrap_or("").to_owned();
        let source = inner["source_block_version_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        step_details.insert((path, assay_id), (title, source));
    }
    let mut protocol_versions: HashMap<String, String> = HashMap::new();
    for scope in workspace["scopes"].as_array().into_iter().flatten() {
        let assay_id = scope["id"].as_str().unwrap_or("").to_owned();
        let version = scope["protocol_version_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        if !assay_id.is_empty() {
            protocol_versions.insert(assay_id, version);
        }
    }
    let global_protocol = workspace["protocol_version_id"]
        .as_str()
        .unwrap_or("")
        .to_owned();
    let mygo = &workspace["frozen_design"]["mygo"];
    let step_tasks = mygo["stepTasks"].as_array().cloned().unwrap_or_default();
    let explicit_links = mygo
        .as_object()
        .is_some_and(|mygo| mygo.contains_key("stepTasks"));
    let mut step_blocks: HashMap<String, Vec<String>> = HashMap::new();
    for (step, blocks) in mygo["stepBlocks"]
        .as_object()
        .into_iter()
        .flat_map(|blocks| blocks.iter())
    {
        let versions = blocks
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|value| value.as_str().map(str::to_owned))
            .collect::<Vec<_>>();
        if !versions.is_empty() {
            step_blocks.insert(step.clone(), versions);
        }
    }
    let mut steps_by_assay: HashMap<String, Vec<String>> = HashMap::new();
    for (step, assay) in mygo["stepAssays"]
        .as_object()
        .into_iter()
        .flat_map(|assays| assays.iter())
    {
        if let Some(assay) = assay.as_str().filter(|assay| !assay.is_empty()) {
            steps_by_assay
                .entry(assay.to_owned())
                .or_default()
                .push(step.clone());
        }
    }
    let mut paths_per_pair: HashMap<(String, String), HashSet<String>> = HashMap::new();
    for task in workspace["tasks"].as_array().into_iter().flatten() {
        let assay = task["scope"]["assay_id"].as_str().unwrap_or("");
        if assay.is_empty() {
            continue;
        }
        paths_per_pair
            .entry((
                assay.to_owned(),
                task["timepoint"].as_str().unwrap_or("").to_owned(),
            ))
            .or_default()
            .insert(task["step_path"].as_str().unwrap_or("").to_owned());
    }

    let mut tasks = Vec::new();
    for task in workspace["tasks"].as_array().into_iter().flatten() {
        let path = task["step_path"].as_str().unwrap_or("").to_owned();
        let assay_id = task["scope"]["assay_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        let timepoint = task["timepoint"].as_str().unwrap_or("").to_owned();
        let (title, source) = step_details
            .get(&(path.clone(), assay_id.clone()))
            .cloned()
            .unwrap_or_default();
        let design_step_id = if explicit_links {
            let pair = step_tasks
                .iter()
                .filter(|link| {
                    link["assayId"].as_str() == Some(assay_id.as_str())
                        && link["timepoint"].as_str() == Some(timepoint.as_str())
                })
                .collect::<Vec<_>>();
            let by_path = pair
                .iter()
                .copied()
                .filter(|link| link["path"].as_str() == Some(path.as_str()))
                .collect::<Vec<_>>();
            let mut chosen = if by_path.len() == 1 {
                by_path[0]["stepId"].as_str().map(str::to_owned)
            } else if by_path.is_empty() {
                let pair_paths = paths_per_pair
                    .get(&(assay_id.clone(), timepoint.clone()))
                    .map(HashSet::len)
                    .unwrap_or(0);
                let unqualified = pair
                    .iter()
                    .copied()
                    .filter(|link| {
                        link.get("path")
                            .and_then(Value::as_str)
                            .map(str::is_empty)
                            .unwrap_or(true)
                    })
                    .collect::<Vec<_>>();
                if pair_paths == 1 && unqualified.len() == 1 {
                    unqualified[0]["stepId"].as_str().map(str::to_owned)
                } else {
                    None
                }
            } else {
                None
            };
            if chosen.is_none() && !source.is_empty() {
                let mut candidates = step_blocks
                    .iter()
                    .filter(|(_, blocks)| blocks.iter().any(|block| block == &source))
                    .map(|(step, _)| step.clone())
                    .collect::<Vec<_>>();
                candidates.sort_unstable();
                if candidates.len() == 1 {
                    chosen = candidates.pop();
                }
            }
            chosen.unwrap_or_default()
        } else {
            steps_by_assay
                .get(&assay_id)
                .filter(|steps| steps.len() == 1)
                .map(|steps| steps[0].clone())
                .unwrap_or_default()
        };
        tasks.push(RunTask {
            key: task["task_key"].as_str().unwrap_or("").to_owned(),
            path,
            title: task["title"]
                .as_str()
                .map(str::to_owned)
                .unwrap_or_else(|| title.clone()),
            status: task["status"].as_str().unwrap_or("").to_owned(),
            protocol_version_id: protocol_versions
                .get(&assay_id)
                .cloned()
                .unwrap_or_else(|| global_protocol.clone()),
            source_block_version_id: source,
            assay_id,
            timepoint,
            design_step_id,
        });
    }
    tasks
}

pub(crate) fn design_map(db: &Connection, design_id: &str) -> Result<Value, CliError> {
    let record = get(db, design_id)?;
    if record["kind"] != "design" {
        return Err(CliError::input("指定记录不是实验设计"));
    }
    let revision = record["revision"]
        .as_i64()
        .ok_or_else(|| CliError::internal("设计版本号无效"))?;
    let map = asterlab_core::design_map::build_current(design_id, revision, &record["data"])
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(map.to_flat_json())
}

pub(crate) fn validate_design(d: &DesignFields) -> Result<(), CliError> {
    validate_design_with_registry(d, &default_labware_registry())
}

pub(crate) fn validate_design_with_registry(
    d: &DesignFields,
    registry: &LabwareRegistry,
) -> Result<(), CliError> {
    check_text(&d.title, "设计名称", 160)?;
    for (value, label, max) in [
        (&d.objective, "研究问题", 5000),
        (&d.experimental_unit, "实验单位", 500),
        (&d.primary_endpoint, "主要检测终点", 1000),
        (&d.endpoint_method, "检测方法", 1000),
        (&d.endpoint_unit, "终点单位", 100),
        (&d.sample_size_rationale, "样本量依据", 5000),
        (&d.randomization, "随机化方法", 2000),
        (&d.blinding, "盲法", 2000),
        (&d.inclusion_criteria, "纳入标准", 5000),
        (&d.exclusion_criteria, "排除标准", 5000),
        (&d.analysis_plan, "分析计划", 5000),
        (&d.controls, "对照设置", 2000),
    ] {
        if value.chars().count() > max {
            return Err(CliError::input(format!("{label}不能超过 {max} 个字符")));
        }
    }
    if let Some(question) = &d.research_question {
        if question.chars().count() > 5000 {
            return Err(CliError::input("研究问题不能超过 5000 个字符"));
        }
    }
    if d.factors.len() > 50 {
        return Err(CliError::input("因素数量最多 50 个"));
    }
    let mut factor_names = std::collections::HashSet::new();
    for factor in &d.factors {
        check_text(&factor.name, "因素名称", 120)?;
        if !factor_names.insert(factor.name.trim().to_lowercase()) {
            return Err(CliError::input(format!("因素名称重复：{}", factor.name)));
        }
        if factor.levels.is_empty() {
            return Err(CliError::input("因素必须至少有一个水平"));
        }
        if factor.levels.len() > 50 {
            return Err(CliError::input("每个因素的水平数量最多 50 个"));
        }
        for level in &factor.levels {
            check_text(level, "因素水平", 500)?;
        }
    }
    validate_groups(&d.group_specs)?;
    d.container_plan
        .validate(registry)
        .map_err(|issues| CliError::input(format!("容器计划无效：{}", issues.join("；"))))?;
    if d.group_specs.len() > 100 {
        return Err(CliError::input("组别数量最多 100 个"));
    }
    if d.timepoints.len() > 100 {
        return Err(CliError::input("时间点数量最多 100 个"));
    }
    for t in &d.timepoints {
        check_text(t, "时间点", 100)?;
    }
    if d.planned_run_count.is_some_and(|n| n == 0 || n > 1000) {
        return Err(CliError::input("计划执行轮次必须为 1–1000；未知时留空"));
    }
    if d.assays.len() > 100 || d.sample_allocations.len() > 10000 {
        return Err(CliError::input(
            "检测最多 100 项；样本分配关系最多 10000 条",
        ));
    }
    let mut assay_ids = std::collections::HashSet::new();
    let group_ids: std::collections::HashSet<&str> =
        d.group_specs.iter().map(|g| g.id.as_str()).collect();
    let group_names: std::collections::HashSet<&str> =
        d.group_specs.iter().map(|g| g.name.as_str()).collect();
    for assay in &d.assays {
        check_text(&assay.id, "检测 ID", 80)?;
        check_text(&assay.title, "检测名称", 160)?;
        if !assay_ids.insert(assay.id.as_str()) {
            return Err(CliError::input(format!("检测 ID 重复：{}", assay.id)));
        }
        for t in &assay.timepoints {
            check_text(t, "检测时间点", 100)?;
        }
        if assay.timepoint_timing.len() > 100 {
            return Err(CliError::input("单项检测的相对时间最多 100 项"));
        }
        if !assay.timepoint_timing.is_empty() {
            if assay.timepoint_timing.len() != assay.timepoints.len()
                || assay
                    .timepoint_timing
                    .iter()
                    .zip(&assay.timepoints)
                    .any(|(timing, label)| timing.timepoint != *label)
            {
                return Err(CliError::input(
                    "相对时间必须与测量时间点逐项对应且顺序一致",
                ));
            }
            if assay
                .time_anchor_event
                .as_deref()
                .is_none_or(|anchor| anchor.trim().is_empty())
            {
                return Err(CliError::input(
                    "填写相对时间前必须明确时间起算事件；未知时先不填写相对时间",
                ));
            }
            for timing in &assay.timepoint_timing {
                if !["min", "h", "day"].contains(&timing.unit.as_str()) {
                    return Err(CliError::input("相对时间单位只能是 min、h 或 day"));
                }
                if timing.offset_value.is_some_and(|v| !v.is_finite())
                    || timing
                        .window_before
                        .is_some_and(|v| !v.is_finite() || v < 0.0)
                    || timing
                        .window_after
                        .is_some_and(|v| !v.is_finite() || v < 0.0)
                {
                    return Err(CliError::input(
                        "相对时间偏移必须为有限数值；允许时间窗不能小于 0",
                    ));
                }
                if timing.offset_value.is_none()
                    && (timing.window_before.is_some() || timing.window_after.is_some())
                {
                    return Err(CliError::input("填写允许时间窗时必须同时填写相对时间偏移"));
                }
            }
        }
        let mut selected_group_ids = std::collections::HashSet::new();
        for group_id in &assay.group_ids {
            check_text(group_id, "检测适用组别 ID", 80)?;
            if !group_ids.contains(group_id.as_str())
                || !selected_group_ids.insert(group_id.as_str())
            {
                return Err(CliError::input("检测适用组别必须引用已定义的唯一组别 ID"));
            }
        }
        for group_name in &assay.group_names {
            check_text(group_name, "检测适用组别", 120)?;
            if !group_names.contains(group_name.as_str()) {
                return Err(CliError::input("检测适用组别必须引用已定义的组别"));
            }
        }
        let mut control_ids = std::collections::HashSet::new();
        for group_id in &assay.control_group_ids {
            check_text(group_id, "对照组 ID", 80)?;
            if !group_ids.contains(group_id.as_str()) || !control_ids.insert(group_id.as_str()) {
                return Err(CliError::input("检测对照必须引用已定义的唯一组别 ID"));
            }
        }
        if assay.timepoints.len() > 100 || assay.controls.len() > 100 {
            return Err(CliError::input("单项检测的时间点或对照最多 100 项"));
        }
    }
    let mut evidence_ids = std::collections::HashSet::new();
    for evidence in &d.source_evidence {
        check_text(&evidence.id, "来源证据 ID", 80)?;
        check_text(&evidence.source_document, "来源文档", 500)?;
        check_text(&evidence.claim, "来源记录内容", 2000)?;
        if let Some(locator) = &evidence.source_locator {
            check_text(locator, "来源定位", 500)?;
        }
        if !evidence_ids.insert(evidence.id.as_str()) {
            return Err(CliError::input(format!(
                "来源证据 ID 重复：{}",
                evidence.id
            )));
        }
    }
    for allocation in &d.sample_allocations {
        if !assay_ids.contains(allocation.assay_id.as_str()) {
            return Err(CliError::input(format!(
                "样本分配引用了不存在的检测：{}",
                allocation.assay_id
            )));
        }
        if !group_names.contains(allocation.group_name.as_str()) {
            return Err(CliError::input(format!(
                "样本分配引用了不存在的组别：{}",
                allocation.group_name
            )));
        }
        if allocation
            .group_id
            .as_deref()
            .is_none_or(|id| !group_ids.contains(id))
        {
            return Err(CliError::input("样本分配必须引用已定义的组别 ID"));
        }
        if allocation
            .physical_slots_per_group
            .is_some_and(|n| n == 0 || n > 100000)
        {
            return Err(CliError::input("每组容器位数量必须为 1–100000；未知时留空"));
        }
        let needs_source = matches!(
            allocation.source_relation.as_ref(),
            Some(SourceRelation::SplitFromAssay | SourceRelation::SequentialAfterAssay)
        );
        if needs_source && allocation.source_assay_id.is_none() {
            return Err(CliError::input("分样或顺序复用关系必须指定来源检测"));
        }
        if !needs_source && allocation.source_assay_id.is_some() {
            return Err(CliError::input("只有分样或顺序复用关系可以填写来源检测"));
        }
        if let Some(source) = &allocation.source_assay_id {
            if !assay_ids.contains(source.as_str()) || source == &allocation.assay_id {
                return Err(CliError::input("样本分配的来源检测必须是另一项已定义检测"));
            }
        }
        if allocation
            .evidence_id
            .as_ref()
            .is_some_and(|id| !evidence_ids.contains(id.as_str()))
        {
            return Err(CliError::input("样本分配引用的来源证据不存在"));
        }
        if allocation
            .planned_output_id
            .as_ref()
            .is_some_and(|id| !d.planned_outputs.iter().any(|output| &output.id == id))
        {
            return Err(CliError::input("样本分配引用了不存在的计划产物"));
        }
    }
    if d.planned_outputs.len() > 1000 {
        return Err(CliError::input("计划产物最多 1000 项"));
    }
    let mut output_ids = std::collections::HashSet::new();
    for output in &d.planned_outputs {
        check_text(&output.id, "计划产物 ID", 80)?;
        check_text(&output.title, "计划产物名称", 160)?;
        check_text(&output.protocol_version_id, "产物制备 Protocol 版本 ID", 80)?;
        if !output_ids.insert(output.id.as_str()) {
            return Err(CliError::input(format!("计划产物 ID 重复：{}", output.id)));
        }
        let mut slots = std::collections::HashSet::new();
        for binding in &output.root_bindings {
            check_text(&binding.slot, "产物参数槽位", 100)?;
            if !slots.insert(binding.slot.as_str()) {
                return Err(CliError::input(format!(
                    "计划产物参数重复：{}",
                    binding.slot
                )));
            }
        }
        if output
            .evidence_id
            .as_ref()
            .is_some_and(|id| !evidence_ids.contains(id.as_str()))
        {
            return Err(CliError::input("计划产物引用的来源证据不存在"));
        }
    }
    for conflict in &d.evidence_conflicts {
        check_text(&conflict.id, "冲突 ID", 80)?;
        check_text(&conflict.topic, "冲突主题", 500)?;
        let mut affected_assay_ids = std::collections::HashSet::new();
        for assay_id in &conflict.affected_assay_ids {
            check_text(assay_id, "冲突关联检测 ID", 80)?;
            if !assay_ids.contains(assay_id.as_str()) || !affected_assay_ids.insert(assay_id) {
                return Err(CliError::input(
                    "冲突关联检测必须是已定义的检测，且不能重复",
                ));
            }
        }
        if conflict.affects_container_estimate && conflict.affected_assay_ids.is_empty() {
            return Err(CliError::input("影响容器位估算的冲突必须关联至少一项检测"));
        }
        if conflict.evidence_ids.len() < 2
            || conflict
                .evidence_ids
                .iter()
                .any(|id| !evidence_ids.contains(id.as_str()))
        {
            return Err(CliError::input(
                "每项证据冲突必须引用至少两条已保存的来源证据",
            ));
        }
        if matches!(conflict.status, ConflictStatus::Resolved)
            && conflict
                .resolution
                .as_deref()
                .unwrap_or("")
                .trim()
                .is_empty()
        {
            return Err(CliError::input(
                "已确认的冲突解释必须写明依据；否则应保留为未解决",
            ));
        }
    }
    Ok(())
}
pub(crate) fn validate_date(value: &str, label: &str) -> Result<(), CliError> {
    if NaiveDate::parse_from_str(value, "%Y-%m-%d").is_err() {
        return Err(CliError::input(format!("{label}必须使用 YYYY-MM-DD 格式")));
    }
    Ok(())
}
