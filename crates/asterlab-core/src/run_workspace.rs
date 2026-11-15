//! Read-model projection for a frozen Run.
//!
//! Steps are definitions, tasks are plans, and occurrences are actual execution
//! instances. This module never reads the current Design.
//!
//! Two entry points project the same facts: [`build`] keeps the unblinded view
//! every existing caller already gets, and [`build_for_view`] accepts a
//! [`ProjectionView`]. A blind view removes frozen group identity — id, name,
//! role, condition, group counts, well group labels and the legacy `mygo`
//! mirror that repeats them — but only when the frozen Design records a
//! `blinding` basis; the view alone never blanks a design that declared none.
//!
//! [`PlannedTask::schedule`] is the plan side of the timeline: it shifts the
//! Run's `scheduled_date` by the frozen assay `time_anchor_event` and
//! `timepoint_timing`. It is not a [`StepOccurrence`]; occurrences are read back
//! from events that carry an `occurrence_id`, never synthesized from a plan.

use crate::design_map::FrozenDesignView;
use crate::pantry::CalendarDate;
use crate::protocol_preview::{OperationStep, ResolvedParameter, StepTimeScope, UnresolvedParameter};
use crate::record_projection::ProjectionView;
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunWorkspace {
    pub run_id: String,
    pub design_id: String,
    pub design_version_id: String,
    pub design_revision: i64,
    pub frozen_design: FrozenDesignView,
    pub protocol_version_id: Option<String>,
    pub status: String,
    pub scopes: Vec<RunScope>,
    pub steps: Vec<RunStep>,
    pub tasks: Vec<PlannedTask>,
    pub occurrences: Vec<StepOccurrence>,
    pub materials: Vec<MaterialManifestView>,
    pub plates: Vec<RunPlateView>,
    /// Linked assets with the coordinates they were recorded with.
    pub linked_assets: Vec<LinkedAssetView>,
    /// Grouping basis for the capture-gap check, one row per well × timepoint.
    pub asset_coverage: Vec<AssetCoverageGroup>,
    pub events: Vec<ExecutionEventView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunScope {
    pub kind: String, // assay | planned_output
    pub id: String,
    pub title: String,
    pub protocol_version_id: Option<String>,
    pub protocol_title: Option<String>,
    pub protocol_version: Option<u64>,
    pub protocol_ready: bool,
    pub resolved_parameters: Vec<ResolvedParameter>,
    pub unresolved_parameters: Vec<UnresolvedParameter>,
    pub step_paths: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunStep {
    pub view_id: String,
    pub scope: StepScope,
    pub order: u32,
    pub step: OperationStep,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PlannedTask {
    pub task_key: String,
    pub step_path: String,
    pub scope: StepScope,
    pub timepoint: Option<String>,
    pub target: TargetSelection,
    #[serde(default = "default_task_status")]
    pub status: String,
    /// Planned execution window; absent when the Run date or the frozen timing
    /// cannot place this task on a calendar.
    #[serde(default)]
    pub schedule: Option<TaskSchedule>,
}

fn default_task_status() -> String {
    "not_started".to_owned()
}

/// Planned execution window of one [`PlannedTask`].
///
/// The stored plan carries a calendar date, not a clock time, so `window_start`
/// and `window_end` are RFC3339 UTC instants anchored at `00:00` of
/// `anchor_date`. They describe when the task is planned, not when it happened;
/// actual times live in [`ExecutionEventView`].
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct TaskSchedule {
    pub timepoint: String,
    /// Frozen `time_anchor_event` the offset is measured from.
    pub anchor_event: String,
    /// Run `scheduled_date`, read as the anchor's day.
    pub anchor_date: String,
    pub offset_value: f64,
    /// `min`, `h` or `day`, as stored.
    pub unit: String,
    pub window_before: Option<f64>,
    pub window_after: Option<f64>,
    pub window_start: String,
    pub window_end: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct StepOccurrence {
    pub occurrence_id: String,
    pub context: StepOccurrenceContext,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct StepOccurrenceContext {
    pub run_id: String,
    pub protocol_version_id: String,
    pub scope: StepScope,
    pub step_path: String,
    pub timepoint: Option<String>,
    pub target: TargetSelection,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum StepScope {
    Assay { assay_id: String },
    PlannedOutput { planned_output_id: String },
    Unscoped,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum TargetRef {
    Sample { sample_id: String },
    Container { container_id: String },
    Well { plate_id: String, coordinate: String },
    Animal { animal_id: String },
    Subject { subject_id: String },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum TargetSelection {
    One { target: TargetRef },
    Many { targets: Vec<TargetRef> },
    NotApplicable,
    Unresolved,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct MaterialManifestView {
    pub planned_output_id: Option<String>,
    pub title: String,
    pub producer_protocol_version_id: Option<String>,
    pub consumer_assay_ids: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunPlateView {
    pub plate_id: String,
    pub name: String,
    pub format: String,
    pub rows: u32,
    pub columns: u32,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub wells: Vec<RunWellView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunWellView {
    pub coordinate: String,
    pub group_label: Option<String>,
    pub sample_id: Option<String>,
    pub note: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ExecutionEventView {
    pub event_id: String,
    pub event_type: Option<String>,
    pub content: Option<String>,
    pub occurred_at: Option<String>,
    pub recorded_at: Option<String>,
    pub stage: Option<String>,
    pub timepoint: Option<String>,
    pub step_status: Option<String>,
    pub protocol_step_path: Option<String>,
    pub protocol_version_id: Option<String>,
    pub assay_id: Option<String>,
    pub planned_output_id: Option<String>,
    pub task_key: Option<String>,
    pub occurrence_id: Option<String>, // legacy events have None
    pub retry_of_event_id: Option<String>,
    /// Explicit sample-attempt identity carried by the write; legacy events have
    /// `None` and readers derive the attempt from retry links.
    pub attempt: Option<u32>,
    pub operator: Option<String>,
    pub sample_ids: Vec<String>,
    pub readouts: Vec<Value>,
    pub measurements: Vec<Value>,
    pub checks: Vec<Value>,
    pub materials: Vec<Value>,
    pub deviation: Option<String>,
    pub check_ack: bool,
    pub controls: Vec<Value>,
    pub lineage: Vec<Value>,
    pub actual_parameters: Vec<Value>,
    pub batch_id: Option<String>,
    pub schema_id: Option<String>,
    pub schema_version: Option<u32>,
    pub result_cells: Vec<Value>,
    pub ad_hoc_step: bool,
    pub skipped_by_rule: bool,
}
/// One linked asset as the Run read model exposes it.
///
/// Every coordinate stays `None` when the stored link never recorded it; the
/// read model fills no default and infers no well from a task or a sample.
/// `coordinate_id` is derived from the recorded coordinates by the same rule
/// `asset.link` uses, so Run and Record navigation share one locator.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct LinkedAssetView {
    pub asset_id: String,
    pub task_key: String,
    pub sample_id: Option<String>,
    pub well_id: Option<String>,
    pub timepoint: Option<String>,
    pub fov_index: Option<u32>,
    pub coordinate_id: Option<String>,
}

/// Capture-gap grouping basis: one row per `(well_id, timepoint)`.
///
/// Only links that recorded a `well_id` form a group. A link without a well
/// stays in [`RunWorkspace::linked_assets`] and never invents a group.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct AssetCoverageGroup {
    pub well_id: String,
    pub timepoint: Option<String>,
    /// Distinct recorded FOV indexes, ascending; empty when none was recorded.
    pub fov_indices: Vec<u32>,
    pub asset_ids: Vec<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct RunWorkspaceError(pub String);

impl std::fmt::Display for RunWorkspaceError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.0)
    }
}

impl std::error::Error for RunWorkspaceError {}

fn text(value: &Value) -> String {
    value.as_str().unwrap_or_default().to_owned()
}

fn opt_text(value: &Value) -> Option<String> {
    value
        .as_str()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
}

/// Reads stored `asset_link` payloads. Coordinates are copied as recorded:
/// a missing or zero FOV index stays `None` instead of becoming "FOV 0".
fn linked_asset_views(link_data: &[Value]) -> Vec<LinkedAssetView> {
    link_data
        .iter()
        .map(|item| {
            let link = &item["link"];
            let well_id = opt_text(&link["wellId"]);
            let timepoint = opt_text(&link["timepoint"]);
            let fov_index = link["fovIndex"].as_u64().map(|v| v as u32).filter(|v| *v > 0);
            let coordinate_id = crate::assets::AssetLink {
                asset_id: text(&item["id"]),
                run_id: opt_text(&link["runId"]).unwrap_or_default(),
                task_key: opt_text(&link["taskKey"]).unwrap_or_default(),
                sample_id: opt_text(&link["sampleId"]),
                well_id: well_id.clone(),
                timepoint: timepoint.clone(),
                fov_index,
            }
            .coordinate_id();
            LinkedAssetView {
                asset_id: text(&item["id"]),
                task_key: opt_text(&link["taskKey"]).unwrap_or_default(),
                sample_id: opt_text(&link["sampleId"]),
                well_id,
                timepoint,
                fov_index,
                coordinate_id,
            }
        })
        .collect()
}

fn asset_coverage(links: &[LinkedAssetView]) -> Vec<AssetCoverageGroup> {
    let mut groups: std::collections::BTreeMap<(String, Option<String>), AssetCoverageGroup> =
        std::collections::BTreeMap::new();
    for link in links {
        let Some(well_id) = link.well_id.as_deref() else {
            continue;
        };
        let group = groups
            .entry((well_id.to_owned(), link.timepoint.clone()))
            .or_insert_with(|| AssetCoverageGroup {
                well_id: well_id.to_owned(),
                timepoint: link.timepoint.clone(),
                fov_indices: Vec::new(),
                asset_ids: Vec::new(),
            });
        if let Some(fov) = link.fov_index {
            if !group.fov_indices.contains(&fov) {
                group.fov_indices.push(fov);
            }
        }
        if !group.asset_ids.contains(&link.asset_id) {
            group.asset_ids.push(link.asset_id.clone());
        }
    }
    groups
        .into_values()
        .map(|mut group| {
            group.fov_indices.sort_unstable();
            group.asset_ids.sort();
            group
        })
        .collect()
}


fn scope_for_manifest(entry: &Value) -> StepScope {
    if let Some(id) = opt_text(&entry["assay_id"]) {
        StepScope::Assay { assay_id: id }
    } else if let Some(id) = opt_text(&entry["planned_output_id"]) {
        StepScope::PlannedOutput {
            planned_output_id: id,
        }
    } else {
        StepScope::Unscoped
    }
}

fn scope_id(scope: &StepScope) -> String {
    match scope {
        StepScope::Assay { assay_id } => assay_id.clone(),
        StepScope::PlannedOutput {
            planned_output_id,
        } => planned_output_id.clone(),
        StepScope::Unscoped => "global".to_owned(),
    }
}

fn parse_manifest_parameters<T: serde::de::DeserializeOwned>(
    entry: &Value,
    field: &str,
    scope: &str,
) -> Result<Vec<T>, RunWorkspaceError> {
    let Some(items) = entry[field].as_array() else {
        return Ok(Vec::new());
    };
    items
        .iter()
        .enumerate()
        .map(|(index, value)| {
            serde_json::from_value(value.clone()).map_err(|error| {
                RunWorkspaceError(format!(
                    "RunProtocol scope {} {}[{}] 无效：{}",
                    scope, field, index, error
                ))
            })
        })
        .collect()
}

fn append_manifest(
    entry: &Value,
    scope: StepScope,
    title: &str,
    scopes: &mut Vec<RunScope>,
    steps: &mut Vec<RunStep>,
    order: &mut u32,
) -> Result<(), RunWorkspaceError> {
    let scope_label = scope_id(&scope);
    let raw_steps = entry["steps"].as_array().cloned().unwrap_or_default();
    let mut step_paths = Vec::new();
    for (index, raw) in raw_steps.iter().enumerate() {
        let step: OperationStep = serde_json::from_value(raw.clone()).map_err(|error| {
            RunWorkspaceError(format!(
                "RunProtocol scope {} step[{}] 无法解析：{}",
                scope_label, index, error
            ))
        })?;
        *order += 1;
        step_paths.push(step.path.clone());
        let view_id = serde_json::to_string(&serde_json::json!({
            "scope": &scope,
            "path": &step.path
        }))
        .unwrap_or_else(|_| format!("{}:{}", scope_label, step.path));
        steps.push(RunStep {
            view_id,
            scope: scope.clone(),
            order: *order,
            step,
        });
    }
    let protocol_ready = entry["protocol_ready"].as_bool().unwrap_or(false);
    let resolved_parameters = parse_manifest_parameters::<ResolvedParameter>(
        entry,
        "resolved_parameters",
        &scope_label,
    )?;
    let unresolved_parameters = parse_manifest_parameters::<UnresolvedParameter>(
        entry,
        "unresolved_parameters",
        &scope_label,
    )?;
    scopes.push(RunScope {
        kind: match scope {
            StepScope::Assay { .. } => "assay".to_owned(),
            StepScope::PlannedOutput { .. } => "planned_output".to_owned(),
            StepScope::Unscoped => "global".to_owned(),
        },
        id: scope_label,
        title: title.to_owned(),
        protocol_version_id: opt_text(&entry["protocol_version_id"]),
        protocol_title: opt_text(&entry["protocol_title"]),
        protocol_version: entry["protocol_version"].as_u64(),
        protocol_ready,
        resolved_parameters,
        unresolved_parameters,
        step_paths,
    });
    Ok(())
}


fn assay_timepoints(design: &crate::design_map::DesignMapContent, assay_id: &str) -> Vec<String> {
    design
        .assays
        .iter()
        .find(|assay| assay.id == assay_id)
        .map(|assay| assay.timepoints.clone())
        .unwrap_or_default()
}

fn explicit_step_timepoints(design_snapshot: &Value) -> std::collections::HashMap<String, Vec<String>> {
    let mut out: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    for task in design_snapshot["mygo"]["stepTasks"].as_array().into_iter().flatten() {
        let path = task["path"].as_str().unwrap_or("").trim();
        let timepoint = task["timepoint"].as_str().unwrap_or("").trim();
        if path.is_empty() || timepoint.is_empty() {
            continue;
        }
        let entry = out.entry(path.to_owned()).or_default();
        if !entry.iter().any(|existing| existing == timepoint) {
            entry.push(timepoint.to_owned());
        }
    }
    out
}

const SECONDS_PER_HOUR: f64 = 3_600.0;
const SECONDS_PER_DAY: f64 = 86_400.0;

fn time_unit_seconds(unit: &str) -> Option<f64> {
    match unit {
        "min" => Some(60.0),
        "h" => Some(SECONDS_PER_HOUR),
        "day" => Some(SECONDS_PER_DAY),
        _ => None,
    }
}

fn finite_number(value: &Value) -> Option<f64> {
    value.as_f64().filter(|value| value.is_finite())
}

/// Planned execution window of one task.
///
/// The window is the Run's `scheduled_date` shifted by the frozen timepoint
/// offset and widened by its allowed window. The stored plan carries a calendar
/// date, not a clock time, so both bounds anchor at `00:00` of that date rather
/// than inventing a time of day. Anything the plan does not state — no date, a
/// timepoint without timing, no `time_anchor_event`, an offset-less window —
/// leaves the task unscheduled instead of producing a guessed window.
fn task_schedule(
    design: &crate::design_map::DesignMapContent,
    scheduled_date: Option<CalendarDate>,
    scope: &StepScope,
    timepoint: &str,
) -> Option<TaskSchedule> {
    let base = scheduled_date?;
    let StepScope::Assay { assay_id } = scope else {
        return None;
    };
    let assay = design.assays.iter().find(|assay| assay.id == *assay_id)?;
    let anchor_event = assay
        .time_anchor_event
        .clone()
        .filter(|anchor| !anchor.trim().is_empty())?;
    let timing = assay
        .timepoint_timing
        .as_array()?
        .iter()
        .find(|timing| timing["timepoint"].as_str() == Some(timepoint))?;
    let offset_value = finite_number(&timing["offset_value"])?;
    let unit = timing["unit"].as_str()?;
    let scale = time_unit_seconds(unit)?;
    let window_before = match timing.get("window_before") {
        None | Some(Value::Null) => None,
        Some(value) => Some(finite_number(value).filter(|value| *value >= 0.0)?),
    };
    let window_after = match timing.get("window_after") {
        None | Some(Value::Null) => None,
        Some(value) => Some(finite_number(value).filter(|value| *value >= 0.0)?),
    };
    let offset_s = offset_value * scale;
    let before_s = window_before.unwrap_or(0.0) * scale;
    let after_s = window_after.unwrap_or(0.0) * scale;
    Some(TaskSchedule {
        timepoint: timepoint.to_owned(),
        anchor_event,
        anchor_date: base.to_iso(),
        offset_value,
        unit: unit.to_owned(),
        window_before,
        window_after,
        window_start: instant(base, offset_s - before_s)?,
        window_end: instant(base, offset_s + after_s)?,
    })
}

/// `base` at `00:00:00Z` plus a signed second offset, rounded to whole seconds.
fn instant(base: CalendarDate, offset_s: f64) -> Option<String> {
    if !offset_s.is_finite() {
        return None;
    }
    let seconds = offset_s.round();
    if seconds.abs() >= 1e15 {
        return None;
    }
    let seconds = seconds as i64;
    let date = base.checked_add_days(seconds.div_euclid(SECONDS_PER_DAY as i64))?;
    let of_day = seconds.rem_euclid(SECONDS_PER_DAY as i64);
    Some(format!(
        "{}T{:02}:{:02}:{:02}Z",
        date.to_iso(),
        of_day / 3_600,
        (of_day % 3_600) / 60,
        of_day % 60
    ))
}

/// Reads actual execution instances back from stored events.
///
/// Only events that recorded an `occurrence_id` produce a [`StepOccurrence`];
/// legacy events stay history. An event that names an occurrence but no
/// Protocol version cannot be placed in the frozen Run, so it is not promoted
/// either. The first event of an instance fixes its context; later events of
/// the same instance add facts, not a second instance.
fn occurrences_from_events(run_id: &str, events: &[ExecutionEventView]) -> Vec<StepOccurrence> {
    let mut seen = std::collections::HashSet::new();
    let mut occurrences = Vec::new();
    for event in events {
        let Some(occurrence_id) = event.occurrence_id.as_deref() else {
            continue;
        };
        let Some(protocol_version_id) = event.protocol_version_id.clone() else {
            continue;
        };
        if !seen.insert(occurrence_id.to_owned()) {
            continue;
        }
        let scope = if let Some(assay_id) = event.assay_id.clone() {
            StepScope::Assay { assay_id }
        } else if let Some(planned_output_id) = event.planned_output_id.clone() {
            StepScope::PlannedOutput { planned_output_id }
        } else {
            StepScope::Unscoped
        };
        let target = if event.sample_ids.is_empty() {
            TargetSelection::Unresolved
        } else {
            TargetSelection::Many {
                targets: event
                    .sample_ids
                    .iter()
                    .map(|sample_id| TargetRef::Sample {
                        sample_id: sample_id.clone(),
                    })
                    .collect(),
            }
        };
        occurrences.push(StepOccurrence {
            occurrence_id: occurrence_id.to_owned(),
            context: StepOccurrenceContext {
                run_id: run_id.to_owned(),
                protocol_version_id,
                scope,
                step_path: event.protocol_step_path.clone().unwrap_or_default(),
                timepoint: event.timepoint.clone(),
                target,
            },
        });
    }
    occurrences
}

/// A blind view only masks a design that recorded a blinding basis; asking for
/// blindness on an unblinded design must not blank its group identity.
fn design_declares_blinding(design_snapshot: &Value) -> bool {
    opt_text(&design_snapshot["blinding"]).is_some()
}

/// Removes frozen group identity from a workspace built for the blind view.
///
/// Identity is removed, never replaced: names and ids become empty and the
/// group counts are dropped, matching [`ProjectionView::Blind`] in the Record
/// projection. The legacy `mygo` mirror repeats assay control and calibrator
/// group names, so the blind view drops the whole mirror instead of guessing
/// which of its keys are identity.
fn mask_group_identity(frozen: &mut FrozenDesignView, plates: &mut [RunPlateView]) {
    frozen.mygo = None;
    for group in &mut frozen.content.groups {
        group.id.clear();
        group.name.clear();
        group.role = None;
        group.condition = None;
        group.independent_n = None;
        group.technical_repeats = None;
    }
    for assay in &mut frozen.content.assays {
        assay.group_ids.clear();
        assay.group_names.clear();
    }
    for link in &mut frozen.content.sample_links {
        link.group_id = None;
        link.group_name.clear();
    }
    for plate in plates {
        for well in &mut plate.wells {
            well.group_label = None;
        }
    }
}

/// Build a frozen Run workspace from stored JSON.
///
/// `design_snapshot` must be the payload of the Run's exact DesignVersion,
/// never the current Design record. This is the unblinded entry point every
/// existing caller uses; see [`build_for_view`] for the blind view.
pub fn build(
    run_id: &str,
    design_id: &str,
    design_version_id: &str,
    design_revision: i64,
    design_snapshot: &Value,
    run_data: &Value,
    plate_data: &[Value],
    event_data: &[Value],
    link_data: &[Value],
) -> Result<RunWorkspace, RunWorkspaceError> {
    build_for_view(
        run_id,
        design_id,
        design_version_id,
        design_revision,
        design_snapshot,
        run_data,
        plate_data,
        event_data,
        link_data,
        ProjectionView::Unblinded,
    )
}

/// Build a frozen Run workspace for an explicit [`ProjectionView`].
///
/// Arguments and validation are identical to [`build`]; only the exposed group
/// identity differs. Blind masking requires the frozen Design to declare a
/// `blinding` basis, so a blind request never blanks a design that declared
/// none.
pub fn build_for_view(
    run_id: &str,
    design_id: &str,
    design_version_id: &str,
    design_revision: i64,
    design_snapshot: &Value,
    run_data: &Value,
    plate_data: &[Value],
    event_data: &[Value],
    link_data: &[Value],
    view: ProjectionView,
) -> Result<RunWorkspace, RunWorkspaceError> {
    let blind = view.is_blind() && design_declares_blinding(design_snapshot);
    let scheduled_date = opt_text(&run_data["scheduled_date"])
        .and_then(|raw| CalendarDate::parse(&raw));
    let frozen = FrozenDesignView::from_version(
        design_id,
        design_version_id,
        design_revision,
        design_snapshot,
    )
    .map_err(|error| RunWorkspaceError(error.to_string()))?;

    let mut scopes: Vec<RunScope> = Vec::new();
    let mut steps: Vec<RunStep> = Vec::new();
    let mut tasks: Vec<PlannedTask> = Vec::new();
    let mut order = 0u32;

    if let Some(entries) = run_data["protocol_manifest"].as_array() {
        for entry in entries {
            let scope = scope_for_manifest(entry);
            let title = opt_text(&entry["assay_title"])
                .or_else(|| opt_text(&entry["protocol_title"]))
                .unwrap_or_else(|| "Protocol".to_owned());
            append_manifest(entry, scope, &title, &mut scopes, &mut steps, &mut order)?;
        }
    }
    if let Some(entries) = run_data["material_manifest"].as_array() {
        for entry in entries {
            if let Some(producer) = entry.get("producer_protocol") {
                let scope = StepScope::PlannedOutput {
                    planned_output_id: opt_text(&entry["planned_output_id"])
                        .unwrap_or_default(),
                };
                let title = opt_text(&entry["title"]).unwrap_or_else(|| "共享材料".to_owned());
                append_manifest(producer, scope, &title, &mut scopes, &mut steps, &mut order)?;
            }
        }
    }

    // Explicit design stepTasks are the author's path-level time binding. They
    // resolve a step whose Protocol block did not repeat a time_scope, without
    // guessing from a title or expanding every assay timepoint.
    let explicit_timepoints = explicit_step_timepoints(design_snapshot);
    let mut task_keys = std::collections::HashSet::new();
    // Tasks are only expanded per timepoint for explicit per-timepoint steps.
    // Missing step-level time scope stays a single task with no claimed timepoint.
    for step in &steps {
        let timepoints = match (&step.scope, step.step.time_scope) {
            (StepScope::Assay { assay_id }, Some(StepTimeScope::PerTimepoint)) => {
                assay_timepoints(&frozen.content, assay_id)
            }
            (StepScope::Assay { .. }, None) => explicit_timepoints
                .get(&step.step.path)
                .cloned()
                .unwrap_or_default(),
            _ => Vec::new(),
        };
        if timepoints.is_empty() {
            let key = serde_json::to_string(&serde_json::json!({
                "scope": &step.scope,
                "path": &step.step.path,
                "timepoint": null
            }))
            .unwrap_or_else(|_| format!("{}:{}", scope_id(&step.scope), step.step.path));
            if !task_keys.insert(key.clone()) {
                return Err(RunWorkspaceError(format!("duplicate task key {key}")));
            }
            tasks.push(PlannedTask {
                task_key: key,
                step_path: step.step.path.clone(),
                scope: step.scope.clone(),
                timepoint: None,
                target: TargetSelection::Unresolved,
                status: default_task_status(),
                schedule: None,
            });
        } else {
            for timepoint in timepoints {
                let key = serde_json::to_string(&serde_json::json!({
                    "scope": &step.scope,
                    "path": &step.step.path,
                    "timepoint": &timepoint
                }))
                .unwrap_or_else(|_| {
                    format!("{}:{}:{}", scope_id(&step.scope), step.step.path, timepoint)
                });
                if !task_keys.insert(key.clone()) {
                    return Err(RunWorkspaceError(format!("duplicate task key {key}")));
                }
                let schedule =
                    task_schedule(&frozen.content, scheduled_date, &step.scope, &timepoint);
                tasks.push(PlannedTask {
                    task_key: key,
                    step_path: step.step.path.clone(),
                    scope: step.scope.clone(),
                    timepoint: Some(timepoint),
                    target: TargetSelection::Unresolved,
                    status: default_task_status(),
                    schedule,
                });
            }
        }
    }

    let materials = run_data["material_manifest"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .map(|item| MaterialManifestView {
                    planned_output_id: opt_text(&item["planned_output_id"]),
                    title: text(&item["title"]),
                    producer_protocol_version_id: item["producer_protocol"]["protocol_version_id"]
                        .as_str()
                        .map(str::to_owned),
                    consumer_assay_ids: item["consumers"]
                        .as_array()
                        .map(|consumers| {
                            consumers
                                .iter()
                                .filter_map(|consumer| opt_text(&consumer["assay_id"]))
                                .collect()
                        })
                        .unwrap_or_default(),
                })
                .collect()
        })
        .unwrap_or_default();

    let plates = plate_data
        .iter()
        .map(|plate| RunPlateView {
            plate_id: text(&plate["id"]),
            name: text(&plate["name"]),
            format: text(&plate["format"]),
            rows: plate["rows"].as_u64().unwrap_or(0) as u32,
            columns: plate["columns"].as_u64().unwrap_or(0) as u32,
            assay_id: opt_text(&plate["assay_id"]),
            timepoint: opt_text(&plate["timepoint"]),
            wells: plate["wells"]
                .as_array()
                .map(|wells| {
                    wells
                        .iter()
                        .map(|well| RunWellView {
                            coordinate: text(&well["well_id"]),
                            group_label: opt_text(&well["group_label"]),
                            sample_id: opt_text(&well["sample_id"]),
                            note: opt_text(&well["note"]),
                        })
                        .collect()
                })
                .unwrap_or_default(),
        })
        .collect();

    let events: Vec<ExecutionEventView> = event_data
        .iter()
        .map(|event| ExecutionEventView {
            event_id: text(&event["id"]),
            event_type: opt_text(&event["event_type"]),
            content: opt_text(&event["content"]),
            occurred_at: opt_text(&event["occurred_at"]),
            recorded_at: opt_text(&event["recorded_at"]),
            stage: opt_text(&event["stage"]),
            timepoint: opt_text(&event["timepoint"]),
            step_status: opt_text(&event["step_status"]),
            protocol_step_path: opt_text(&event["protocol_step_path"]),
            protocol_version_id: opt_text(&event["protocol_version_id"]),
            assay_id: opt_text(&event["assay_id"]),
            planned_output_id: opt_text(&event["planned_output_id"]),
            task_key: opt_text(&event["task_key"]),
            occurrence_id: opt_text(&event["occurrence_id"]),
            retry_of_event_id: opt_text(&event["retry_of_event_id"]),
            attempt: event["attempt"]
                .as_u64()
                .map(|attempt| attempt as u32)
                .filter(|attempt| *attempt >= 1),
            operator: opt_text(&event["operator"]),
            sample_ids: event["sample_ids"]
                .as_array()
                .map(|items| items.iter().filter_map(Value::as_str).map(str::to_owned).collect())
                .unwrap_or_default(),
            readouts: event["readouts"].as_array().cloned().unwrap_or_default(),
            measurements: event["measurements"].as_array().cloned().unwrap_or_default(),
            checks: event["checks"].as_array().cloned().unwrap_or_default(),
            materials: event["materials"].as_array().cloned().unwrap_or_default(),
            deviation: opt_text(&event["deviation"]),
            check_ack: event["check_ack"].as_bool().unwrap_or(false),
            controls: event["controls"].as_array().cloned().unwrap_or_default(),
            lineage: event["lineage"].as_array().cloned().unwrap_or_default(),
            actual_parameters: event["actual_parameters"].as_array().cloned().unwrap_or_default(),
            batch_id: opt_text(&event["batch_id"]),
            schema_id: opt_text(&event["schema_id"]),
            schema_version: event["schema_version"].as_u64().map(|v| v as u32),
            result_cells: event["result_cells"].as_array().cloned().unwrap_or_default(),
            ad_hoc_step: event["ad_hoc_step"].as_bool().unwrap_or(false),
            skipped_by_rule: opt_text(&event["event_type"]).as_deref() == Some("skipped_by_rule"),
        })
        .collect();

    // Old events do not have occurrence_id. Do not synthesize an instance.
    let occurrences = occurrences_from_events(run_id, &events);
    let linked_assets = linked_asset_views(link_data);
    let asset_coverage = asset_coverage(&linked_assets);

    let mut workspace = RunWorkspace {
        run_id: run_id.to_owned(),
        design_id: design_id.to_owned(),
        design_version_id: design_version_id.to_owned(),
        design_revision,
        frozen_design: frozen,
        protocol_version_id: opt_text(&run_data["protocol_version_id"]),
        status: text(&run_data["status"]),
        scopes,
        steps,
        tasks,
        occurrences,
        materials,
        plates,
        linked_assets,
        asset_coverage,
        events,
    };
    for task in &mut workspace.tasks {
        let mut latest: Option<&ExecutionEventView> = None;
        for event in &workspace.events {
            if !event_matches_task(task, event) || event.step_status.is_none() {
                continue;
            }
            let at = event.recorded_at.as_deref().or(event.occurred_at.as_deref()).unwrap_or("");
            let prev = latest
                .map(|e| e.recorded_at.as_deref().or(e.occurred_at.as_deref()).unwrap_or(""))
                .unwrap_or("");
            if latest.is_none() || at >= prev {
                latest = Some(event);
            }
        }
        task.status = latest
            .and_then(|event| event.step_status.clone())
            .unwrap_or_else(|| "not_started".to_owned());
    }
    if blind {
        mask_group_identity(&mut workspace.frozen_design, &mut workspace.plates);
    }
    Ok(workspace)
}

fn event_matches_task(task: &PlannedTask, event: &ExecutionEventView) -> bool {
    if let Some(key) = event.task_key.as_deref() {
        return task.task_key == key;
    }
    if event.protocol_step_path.as_deref() != Some(task.step_path.as_str()) {
        return false;
    }
    if event.timepoint.as_deref().unwrap_or("") != task.timepoint.as_deref().unwrap_or("") {
        return false;
    }
    match &task.scope {
        StepScope::Assay { assay_id } => event.assay_id.as_deref() == Some(assay_id.as_str()),
        StepScope::PlannedOutput { planned_output_id } => event.planned_output_id.as_deref() == Some(planned_output_id.as_str()),
        StepScope::Unscoped => event.assay_id.is_none() && event.planned_output_id.is_none(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn design() -> Value {
        json!({
            "title":"d","objective":"o","experimental_unit":"well",
            "group_specs":[{"id":"g1","name":"Control","role":"control","independent_n":3}],
            "assays":[{"id":"cck8","title":"CCK-8","timepoints":["D1","D3"],
                "group_ids":["g1"],"timepoint_relation":"same_physical_sample_repeated"}],
            "sample_allocations":[],"planned_outputs":[],"evidence_conflicts":[]
        })
    }

    fn run() -> Value {
        json!({
            "status":"planned",
            "protocol_manifest":[{
                "assay_id":"cck8","assay_title":"CCK-8","protocol_version_id":"pv1",
                "steps":[
                    {"path":"root/a","use_id":"a","title":"Add","instruction":"add",
                     "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1","time_scope":"per_timepoint"},
                    {"path":"root/b","use_id":"b","title":"Read","instruction":"read",
                     "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1","time_scope":"per_timepoint"}
                ]
            }],
            "material_manifest":[]
        })
    }

    #[test]
    fn tasks_use_explicit_timepoints_without_targets() {
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &[], &[]).unwrap();
        assert_eq!(workspace.steps.len(), 2);
        assert_eq!(workspace.tasks.len(), 4);
        assert!(workspace.tasks.iter().all(|task| task.target == TargetSelection::Unresolved));
        assert!(workspace.tasks.iter().any(|task| task.timepoint.as_deref() == Some("D1")));
        assert!(workspace.occurrences.is_empty());
    }

    #[test]
    fn same_step_path_in_different_scopes_has_distinct_view_ids() {
        let mut run = run();
        run["protocol_manifest"] = json!([
            {"assay_id":"cck8","assay_title":"CCK-8","protocol_version_id":"pv1","steps":[{
                "path":"root/seed","use_id":"seed","title":"Seed","instruction":"seed",
                "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1"
            }]},
            {"assay_id":"live","assay_title":"Live/Dead","protocol_version_id":"pv2","steps":[{
                "path":"root/seed","use_id":"seed","title":"Seed","instruction":"seed",
                "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b2"
            }]}
        ]);
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run, &[], &[], &[]).unwrap();
        assert_eq!(workspace.steps.len(), 2);
        assert_ne!(workspace.steps[0].view_id, workspace.steps[1].view_id);
    }

    #[test]
    fn missing_step_time_scope_does_not_expand_timepoints() {
        let mut run = run();
        run["protocol_manifest"][0]["steps"][0]
            .as_object_mut()
            .unwrap()
            .remove("time_scope");
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run, &[], &[], &[]).unwrap();
        assert_eq!(workspace.tasks.len(), 3);
        assert!(workspace
            .tasks
            .iter()
            .filter(|task| task.step_path == "root/a")
            .all(|task| task.timepoint.is_none()));
        assert_eq!(
            workspace
                .tasks
                .iter()
                .filter(|task| task.step_path == "root/b")
                .count(),
            2
        );
    }

    #[test]
    fn duplicate_task_keys_are_rejected_not_silently_merged() {
        let mut run = run();
        run["protocol_manifest"][0]["steps"][1]["path"] = json!("root/a");
        assert!(build("r1", "d1", "dv1", 1, &design(), &run, &[], &[], &[]).is_err());
    }

    #[test]
    fn frozen_design_carries_sdk_modules() {
        let mut design = design();
        design["asterlab"] = json!({"foundationVersion":1,"sdkVersion":"1.0.0",
            "modules":[{"kind":"dilution","id":"d1","schema":1}]});
        let workspace = build("r1", "d1", "dv1", 1, &design, &run(), &[], &[], &[]).unwrap();
        assert_eq!(workspace.frozen_design.asterlab.as_ref().unwrap()["modules"][0]["kind"], "dilution");
    }

    #[test]
    fn explicit_design_step_tasks_bind_timepoints_without_protocol_scope() {
        let mut design = design();
        design["mygo"] = json!({
            "stepTasks": [
                {"stepId":"s1","path":"root/a","timepoint":"D1","assayId":"cck8"}
            ]
        });
        let mut run = run();
        run["protocol_manifest"][0]["steps"][0]
            .as_object_mut()
            .unwrap()
            .remove("time_scope");
        let workspace = build("r1", "d1", "dv1", 1, &design, &run, &[], &[], &[]).unwrap();
        let root_a: Vec<_> = workspace.tasks.iter().filter(|task| task.step_path == "root/a").collect();
        assert_eq!(root_a.len(), 1);
        assert_eq!(root_a[0].timepoint.as_deref(), Some("D1"));
    }

    #[test]
    fn invalid_step_is_an_error_not_a_silent_drop() {
        let mut run = run();
        run["protocol_manifest"][0]["steps"][0]
            .as_object_mut()
            .unwrap()
            .remove("path");
        let error = build("r1", "d1", "dv1", 1, &design(), &run, &[], &[], &[]).unwrap_err();
        assert!(error.to_string().contains("step[0]"), "{error}");
    }

    #[test]
    fn old_events_remain_legacy_without_occurrence_id() {
        let events = vec![json!({"id":"e1","event_type":"operation","timepoint":"D1",
            "protocol_step_path":"root/a","protocol_version_id":"pv1","assay_id":"cck8"})];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &events, &[]).unwrap();
        assert!(workspace.occurrences.is_empty());
        assert_eq!(workspace.events[0].occurrence_id, None);
    }
    #[test]
    fn projects_rule_skips_and_ad_hoc_step_marker_without_blocking_task() {
        let events = vec![json!({
            "id":"e1","event_type":"skipped_by_rule","content":"No bubbles",
            "recorded_at":"2026-11-15T10:00:00Z","protocol_step_path":"root/a",
            "timepoint":"D1",
            "protocol_version_id":"pv1","assay_id":"cck8","step_status":"skipped",
            "ad_hoc_step":true
        })];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &events, &[]).unwrap();
        assert!(workspace.events[0].skipped_by_rule);
        assert!(workspace.events[0].ad_hoc_step);
        assert_eq!(workspace.tasks[0].status, "skipped");
    }
    #[test]
    fn skipped_by_rule_is_projected_as_a_non_blocking_step_status() {
        let events = vec![json!({"id":"skip-1","event_type":"skipped_by_rule",
            "step_status":"skipped","protocol_step_path":"root/a","protocol_version_id":"pv1",
            "timepoint":"D1",
            "assay_id":"cck8","recorded_at":"2026-11-15T10:00:00Z"})];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &events, &[]).unwrap();
        assert_eq!(workspace.tasks[0].status, "skipped");
        assert!(workspace.events[0].skipped_by_rule);
    }

    fn link(id: &str, well: Option<&str>, timepoint: Option<&str>, fov: Option<u32>) -> Value {
        json!({
            "id": id,
            "link": {
                "runId": "r1",
                "taskKey": "root/b",
                "sampleId": "sample-1",
                "wellId": well,
                "timepoint": timepoint,
                "fovIndex": fov
            }
        })
    }

    #[test]
    fn linked_assets_carry_their_coordinates_and_group_by_well_and_timepoint() {
        let links = vec![
            link("asset-2", Some("A1"), Some("t6h"), Some(2)),
            link("asset-1", Some("A1"), Some("t6h"), Some(1)),
            link("asset-3", Some("A1"), Some("t12h"), Some(1)),
            link("asset-4", Some("B1"), None, None),
        ];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &[], &links).unwrap();
        assert_eq!(workspace.linked_assets.len(), 4);
        assert_eq!(workspace.linked_assets[0].coordinate_id.as_deref(), Some("A1_t6h_fov02"));
        assert_eq!(workspace.linked_assets[3].coordinate_id.as_deref(), Some("B1"));
        assert_eq!(
            workspace.asset_coverage,
            vec![
                AssetCoverageGroup {
                    well_id: "A1".into(),
                    timepoint: Some("t12h".into()),
                    fov_indices: vec![1],
                    asset_ids: vec!["asset-3".into()],
                },
                AssetCoverageGroup {
                    well_id: "A1".into(),
                    timepoint: Some("t6h".into()),
                    fov_indices: vec![1, 2],
                    asset_ids: vec!["asset-1".into(), "asset-2".into()],
                },
                AssetCoverageGroup {
                    well_id: "B1".into(),
                    timepoint: None,
                    fov_indices: vec![],
                    asset_ids: vec!["asset-4".into()],
                },
            ]
        );
    }

    #[test]
    fn legacy_links_keep_coordinates_empty_and_group_nothing() {
        let links = vec![
            json!({"id":"asset-old","link":{"runId":"r1","taskKey":"root/b"}}),
            json!({"id":"asset-zero","link":{"runId":"r1","taskKey":"root/b",
                "wellId":"C1","timepoint":"","fovIndex":0}}),
        ];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &[], &links).unwrap();
        let old = &workspace.linked_assets[0];
        assert_eq!(old.well_id, None);
        assert_eq!(old.timepoint, None);
        assert_eq!(old.fov_index, None);
        assert_eq!(old.coordinate_id, None);
        // An empty timepoint and a zero FOV index are absent, not "0".
        let zero = &workspace.linked_assets[1];
        assert_eq!(zero.well_id.as_deref(), Some("C1"));
        assert_eq!(zero.timepoint, None);
        assert_eq!(zero.fov_index, None);
        assert_eq!(zero.coordinate_id.as_deref(), Some("C1"));
        assert_eq!(workspace.asset_coverage.len(), 1);
        assert!(workspace.asset_coverage[0].fov_indices.is_empty());
    }

    fn timed_design() -> Value {
        json!({
            "title":"d","objective":"o","experimental_unit":"well",
            "blinding":"研究者与操作者均盲，按笼号编号给药",
            "group_specs":[{"id":"g1","name":"Control","role":"control",
                "condition":"vehicle","independent_n":3,"technical_repeats":2}],
            "assays":[{"id":"cck8","title":"CCK-8","endpoint":"OD450","method":"absorbance",
                "unit":"AU","timepoints":["D0","D3","D6"],
                "time_anchor_event":"treatment_start",
                "timepoint_timing":[
                    {"timepoint":"D0","offset_value":0.0,"unit":"day"},
                    {"timepoint":"D3","offset_value":3.0,"unit":"day",
                     "window_before":0.5,"window_after":0.5},
                    {"timepoint":"D6","offset_value":144.0,"unit":"h"}
                ],
                "group_ids":["g1"],"group_names":["Control"],
                "timepoint_relation":"same_physical_sample_repeated"}],
            "sample_allocations":[{"assay_id":"cck8","group_id":"g1","group_name":"Control",
                "physical_slots_per_group":3}],
            "planned_outputs":[],"evidence_conflicts":[],
            "mygo":{"stepTasks":[]}
        })
    }

    fn timed_run() -> Value {
        json!({
            "status":"planned",
            "scheduled_date":"2026-11-20",
            "protocol_manifest":[{
                "assay_id":"cck8","assay_title":"CCK-8","protocol_version_id":"pv1",
                "steps":[{
                    "path":"root/dose","use_id":"dose","title":"Dose","instruction":"dose",
                    "parameters":[],"unresolved_parameter_refs":[],
                    "source_block_version_id":"b1","time_scope":"per_timepoint"
                }]
            }],
            "material_manifest":[]
        })
    }

    fn labeled_plates() -> Vec<Value> {
        vec![json!({
            "id":"p1","name":"Plate","format":"96","rows":8,"columns":12,
            "assay_id":"cck8","wells":[
                {"well_id":"A1","group_label":"Control","sample_id":"s1"}
            ]
        })]
    }

    #[test]
    fn blind_view_masks_frozen_group_identity_and_well_labels() {
        let workspace = build_for_view(
            "r1", "d1", "dv1", 1, &timed_design(), &timed_run(),
            &labeled_plates(), &[], &[], ProjectionView::Blind,
        )
        .unwrap();
        let group = &workspace.frozen_design.content.groups[0];
        assert_eq!(group.id, "");
        assert_eq!(group.name, "");
        assert_eq!(group.role, None);
        assert_eq!(group.condition, None);
        assert_eq!(group.independent_n, None);
        assert_eq!(group.technical_repeats, None);
        let assay = &workspace.frozen_design.content.assays[0];
        assert!(assay.group_ids.is_empty());
        assert!(assay.group_names.is_empty());
        let link = &workspace.frozen_design.content.sample_links[0];
        assert_eq!(link.group_id, None);
        assert_eq!(link.group_name, "");
        // The legacy mygo mirror embeds control/calibrator group names; a blind
        // view drops the mirror instead of guessing which of its keys leak.
        assert_eq!(workspace.frozen_design.mygo, None);
        assert_eq!(workspace.plates[0].wells[0].group_label, None);
        assert_eq!(workspace.plates[0].wells[0].sample_id.as_deref(), Some("s1"));
        // Blinding removes identity, not the schedule or the timepoints.
        assert_eq!(assay.timepoints, vec!["D0", "D3", "D6"]);
        assert_eq!(workspace.tasks.len(), 3);
    }

    #[test]
    fn explicit_unblinded_view_keeps_group_plaintext_for_a_blind_design() {
        let workspace = build_for_view(
            "r1", "d1", "dv1", 1, &timed_design(), &timed_run(),
            &labeled_plates(), &[], &[], ProjectionView::Unblinded,
        )
        .unwrap();
        let group = &workspace.frozen_design.content.groups[0];
        assert_eq!(group.id, "g1");
        assert_eq!(group.name, "Control");
        assert_eq!(group.role.as_deref(), Some("control"));
        assert_eq!(group.condition.as_deref(), Some("vehicle"));
        assert_eq!(workspace.plates[0].wells[0].group_label.as_deref(), Some("Control"));
        assert!(workspace.frozen_design.mygo.is_some());
        // The default entry point stays unblinded so existing callers are
        // unchanged.
        let default = build(
            "r1", "d1", "dv1", 1, &timed_design(), &timed_run(),
            &labeled_plates(), &[], &[],
        )
        .unwrap();
        assert_eq!(default.frozen_design.content.groups[0].name, "Control");
    }

    #[test]
    fn blind_view_without_a_declared_blinding_basis_changes_nothing() {
        for declaration in [json!(""), json!(null)] {
            let mut design = timed_design();
            design["blinding"] = declaration;
            let workspace = build_for_view(
                "r1", "d1", "dv1", 1, &design, &timed_run(),
                &labeled_plates(), &[], &[], ProjectionView::Blind,
            )
            .unwrap();
            assert_eq!(workspace.frozen_design.content.groups[0].name, "Control");
            assert_eq!(
                workspace.plates[0].wells[0].group_label.as_deref(),
                Some("Control")
            );
        }
    }

    #[test]
    fn occurrences_are_projected_from_events_that_carry_an_occurrence_id() {
        let events = vec![
            json!({"id":"e1","event_type":"operation","timepoint":"D0",
                "protocol_step_path":"root/dose","protocol_version_id":"pv1",
                "assay_id":"cck8","occurrence_id":"occ-1","sample_ids":["s1","s2"],
                "recorded_at":"2026-11-20T09:00:00Z"}),
            json!({"id":"e2","event_type":"measurement","timepoint":"D0",
                "protocol_step_path":"root/dose","protocol_version_id":"pv1",
                "assay_id":"cck8","occurrence_id":"occ-1",
                "recorded_at":"2026-11-20T10:00:00Z"}),
            json!({"id":"e3","event_type":"operation","timepoint":"D3",
                "protocol_step_path":"root/dose","protocol_version_id":"pv1",
                "assay_id":"cck8"}),
        ];
        let workspace =
            build("r1", "d1", "dv1", 1, &timed_design(), &timed_run(), &[], &events, &[]).unwrap();
        assert_eq!(workspace.occurrences.len(), 1);
        let occurrence = &workspace.occurrences[0];
        assert_eq!(occurrence.occurrence_id, "occ-1");
        assert_eq!(occurrence.context.run_id, "r1");
        assert_eq!(occurrence.context.protocol_version_id, "pv1");
        assert_eq!(
            occurrence.context.scope,
            StepScope::Assay { assay_id: "cck8".to_owned() }
        );
        assert_eq!(occurrence.context.step_path, "root/dose");
        assert_eq!(occurrence.context.timepoint.as_deref(), Some("D0"));
        assert_eq!(
            occurrence.context.target,
            TargetSelection::Many {
                targets: vec![
                    TargetRef::Sample { sample_id: "s1".to_owned() },
                    TargetRef::Sample { sample_id: "s2".to_owned() },
                ]
            }
        );
        // A legacy event without the id is history, not a new instance.
        assert_eq!(workspace.events[2].occurrence_id, None);
    }

    #[test]
    fn occurrence_id_without_a_protocol_version_is_not_synthesized() {
        let events = vec![json!({"id":"e1","event_type":"operation","timepoint":"D0",
            "protocol_step_path":"root/dose","assay_id":"cck8","occurrence_id":"occ-1"})];
        let workspace =
            build("r1", "d1", "dv1", 1, &timed_design(), &timed_run(), &[], &events, &[]).unwrap();
        assert!(workspace.occurrences.is_empty());
    }

    #[test]
    fn periodic_timepoints_project_execution_windows_from_the_run_date() {
        let workspace =
            build("r1", "d1", "dv1", 1, &timed_design(), &timed_run(), &[], &[], &[]).unwrap();
        assert_eq!(workspace.tasks.len(), 3);
        let schedule = |label: &str| {
            workspace
                .tasks
                .iter()
                .find(|task| task.timepoint.as_deref() == Some(label))
                .and_then(|task| task.schedule.as_ref())
                .unwrap_or_else(|| panic!("no schedule for {label}"))
        };
        let d0 = schedule("D0");
        assert_eq!(d0.timepoint, "D0");
        assert_eq!(d0.anchor_event, "treatment_start");
        assert_eq!(d0.anchor_date, "2026-11-20");
        assert_eq!(d0.unit, "day");
        assert_eq!(d0.window_before, None);
        assert_eq!(d0.window_after, None);
        assert_eq!(d0.window_start, "2026-11-20T00:00:00Z");
        assert_eq!(d0.window_end, "2026-11-20T00:00:00Z");
        let d3 = schedule("D3");
        assert_eq!(d3.offset_value, 3.0);
        assert_eq!(d3.window_before, Some(0.5));
        assert_eq!(d3.window_after, Some(0.5));
        assert_eq!(d3.window_start, "2026-11-22T12:00:00Z");
        assert_eq!(d3.window_end, "2026-11-23T12:00:00Z");
        let d6 = schedule("D6");
        assert_eq!(d6.unit, "h");
        assert_eq!(d6.offset_value, 144.0);
        assert_eq!(d6.window_start, "2026-11-26T00:00:00Z");
        assert_eq!(d6.window_end, "2026-11-26T00:00:00Z");
    }

    #[test]
    fn a_task_without_a_run_date_timing_or_anchor_gets_no_window() {
        let mut undated = timed_run();
        undated.as_object_mut().unwrap().remove("scheduled_date");
        let workspace =
            build("r1", "d1", "dv1", 1, &timed_design(), &undated, &[], &[], &[]).unwrap();
        assert_eq!(workspace.tasks.len(), 3);
        assert!(workspace.tasks.iter().all(|task| task.schedule.is_none()));

        let mut untimed = timed_design();
        untimed["assays"][0]["timepoint_timing"] = json!([]);
        let workspace =
            build("r1", "d1", "dv1", 1, &untimed, &timed_run(), &[], &[], &[]).unwrap();
        assert!(workspace.tasks.iter().all(|task| task.schedule.is_none()));

        let mut unanchored = timed_design();
        unanchored["assays"][0]
            .as_object_mut()
            .unwrap()
            .remove("time_anchor_event");
        let workspace =
            build("r1", "d1", "dv1", 1, &unanchored, &timed_run(), &[], &[], &[]).unwrap();
        assert!(workspace.tasks.iter().all(|task| task.schedule.is_none()));
    }

    #[test]
    fn linked_asset_outside_any_well_stays_visible_without_a_group() {
        let links = vec![
            link("asset-1", None, Some("t6h"), Some(1)),
            link("asset-2", Some("A1"), Some("t6h"), Some(1)),
        ];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &[], &links).unwrap();
        assert_eq!(workspace.linked_assets.len(), 2);
        assert_eq!(workspace.linked_assets[0].coordinate_id, None);
        assert_eq!(workspace.asset_coverage.len(), 1);
        assert_eq!(workspace.asset_coverage[0].asset_ids, vec!["asset-2".to_owned()]);
    }
}
