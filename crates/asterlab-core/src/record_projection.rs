//! Single read-only `RecordProjection` for the Record surface.
//!
//! Record (Notebook) and Run history (audit) read the *same* projection:
//! frozen design + planned/actual parameters + image slot matrix + timers +
//! located assets + readings + Markdown notes. Nothing here writes a record,
//! redefines a group, invents a slot or recomputes a reading.
//!
//! Two views project the same facts:
//!
//! - [`ProjectionView::Unblinded`] renders the frozen group identity;
//! - [`ProjectionView::Blind`] drops group id/name/role/condition and well
//!   group labels, because the Run bench must record without exposing the
//!   treatment assignment. Blind masking removes identity, it never
//!   substitutes a made-up label.
//!
//! Notes are Markdown with dynamic references (`@matrix(6h,12h,24h)`,
//! `@params(tube)`, `@timeline`). The projection resolves every reference into
//! structured data so a PDF export can flatten it; an unknown or malformed
//! reference stays visible as an unresolved reference plus an issue.
//!
//! Signs and archives are pure state transitions over a supplied input. Core
//! never reads a clock, copies a file or writes an archive; the manifest lists
//! the SHA256 checksum of every archived member.

use crate::assets::{AssetLink, AssetRef};
use crate::design_map::{DesignAssay, DesignGroup, FrozenDesignView};
use crate::execution_timeline::{
    query_timeline, RetryRecord, TimelineEvent, TimelineIssue, TimelineQuery,
};
use crate::readout_records::{
    query_readout_records, ReadoutRecord, ReadoutRecordQuery, ReadoutRecordQueryResult,
};
use crate::reading_facts::{
    query_readings, time_key, ReadingFact, ReadingIssue, ReadingQuery,
};
use crate::run_workspace::{ExecutionEventView, RunPlateView, RunWorkspace};
use crate::timer::{TimerMode, TimerRecord, TimerStatus};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet, HashMap};

/// Schema version of [`RecordProjection`] and [`ImageSlotMatrix`].
pub const RECORD_PROJECTION_SCHEMA_VERSION: u32 = 1;

/// Schema version of [`ArchiveManifest`].
pub const ARCHIVE_MANIFEST_SCHEMA_VERSION: u32 = 1;

/// Which group identity the projection may expose.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ProjectionView {
    /// Bench view: group id, name, role, condition and well group labels are
    /// removed.
    Blind,
    /// Record/Notebook view: the frozen group identity is rendered.
    Unblinded,
}

impl ProjectionView {
    pub fn is_blind(self) -> bool {
        matches!(self, Self::Blind)
    }
}

/// One problem found while projecting. `code` is stable; `path` locates the
/// note, asset or slot that caused it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordIssue {
    pub code: String,
    pub message: String,
    pub path: Option<String>,
}

impl RecordIssue {
    fn new(code: &str, message: impl Into<String>, path: Option<String>) -> Self {
        Self {
            code: code.to_owned(),
            message: message.into(),
            path,
        }
    }
}

/// One recorded actual parameter value.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ActualParameter {
    pub value: Value,
    pub unit: Option<String>,
    pub event_id: String,
    pub recorded_at: Option<String>,
    pub source: Option<String>,
}

/// How the actual records relate to the planned binding.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ParameterStatus {
    /// Every recorded actual value equals the planned value.
    Matches,
    /// At least one recorded actual value differs from the planned value.
    Differs,
    /// Planned but never recorded.
    MissingActual,
    /// Recorded but not planned in the frozen Protocol.
    Unplanned,
}

/// Planned Protocol binding next to the actual Run values for the same slot.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ParameterComparison {
    pub scope_id: String,
    pub step_path: Option<String>,
    pub slot: String,
    pub label: Option<String>,
    pub unit: Option<String>,
    pub planned: Option<Value>,
    pub actual: Vec<ActualParameter>,
    pub status: ParameterStatus,
}

/// Timer state with the measured elapsed time at the projection watermark.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TimerObservation {
    pub timer_id: String,
    pub task_key: String,
    pub phase_index: u32,
    pub mode: TimerMode,
    pub status: TimerStatus,
    pub planned_s: Option<u64>,
    /// `None` when the timer is running and no watermark was supplied.
    pub elapsed_s: Option<u64>,
    pub remaining_s: Option<u64>,
    pub round_index: u32,
    pub round_count: Option<u32>,
}

/// One planned image slot. `asset_id` stays `None` until an asset fills it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ImageSlot {
    pub timepoint: Option<String>,
    pub fov_index: u32,
    pub asset_id: Option<String>,
}

/// One well row of the completion matrix.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ImageWellRow {
    pub coordinate: String,
    /// Removed in the blind view.
    pub group_label: Option<String>,
    pub slots: Vec<ImageSlot>,
    pub filled_slots: u32,
    pub planned_slots: u32,
}

/// One plate of the completion matrix.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ImagePlateRow {
    pub plate_id: String,
    pub name: String,
    pub timepoints: Vec<Option<String>>,
    pub wells: Vec<ImageWellRow>,
    pub planned_slots: u32,
    pub filled_slots: u32,
}

/// Planned slots are derived from the frozen plate layout and the assay plan
/// times `fov_per_well`; they are never derived from already uploaded assets.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ImageSlotMatrix {
    pub fov_per_well: u32,
    pub plates: Vec<ImagePlateRow>,
    pub planned_slots: u32,
    pub filled_slots: u32,
    pub missing_slots: u32,
    /// Assets that name no known well coordinate.
    pub unlocated_asset_ids: Vec<String>,
    /// Assets that fill no planned slot (unknown FOV index or no plan row).
    pub unplanned_asset_ids: Vec<String>,
}

/// A located asset, keeping the physical/image coordinates it was linked with.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordAsset {
    pub asset_id: String,
    pub task_key: String,
    pub sample_id: Option<String>,
    pub well_id: Option<String>,
    pub timepoint: Option<String>,
    pub fov_index: Option<u32>,
    /// Stable locator shared with navigation, `None` without a well.
    pub coordinate_id: Option<String>,
}

/// Frozen group identity. The blind view returns `group_id: None` and an empty
/// name instead of a placeholder.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordGroupView {
    pub group_id: Option<String>,
    pub name: String,
    pub role: Option<String>,
    pub condition: Option<String>,
    pub independent_n: Option<u32>,
    pub technical_repeats: Option<u32>,
    pub blinded: bool,
}

/// Frozen assay identity. Group references are dropped in the blind view.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordAssayView {
    pub assay_id: String,
    pub title: String,
    pub endpoint: String,
    pub method: String,
    pub unit: String,
    pub group_ids: Vec<String>,
    pub timepoints: Vec<String>,
    pub timepoint_relation: Option<String>,
    pub protocol_version_id: Option<String>,
}

/// Frozen design snapshot; the Record view never reads the current Design.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordDesignSummary {
    pub design_id: String,
    pub design_version_id: String,
    pub design_revision: i64,
    pub title: String,
    pub objective: String,
    pub groups: Vec<RecordGroupView>,
    pub assays: Vec<RecordAssayView>,
}

/// Timeline counts for the Record audit renderer.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordTimelineSummary {
    pub events: Vec<TimelineEvent>,
    pub failures: Vec<TimelineEvent>,
    pub retries: Vec<RetryRecord>,
    pub issues: Vec<TimelineIssue>,
    pub skipped_by_rule: u32,
    pub first_recorded_at: Option<String>,
    pub last_recorded_at: Option<String>,
}

/// Kind of a dynamic note reference.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReferenceKind {
    /// Image slot matrix, optionally filtered by timepoint.
    Matrix,
    /// Planned vs actual parameter rows.
    Params,
    /// Execution timeline counts.
    Timeline,
    /// A name Core does not resolve.
    Unknown,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReferenceStatus {
    Resolved,
    Unresolved,
}

/// One `@kind(args)` reference resolved against this projection.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ResolvedReference {
    pub raw: String,
    pub kind: ReferenceKind,
    pub args: Vec<String>,
    pub status: ReferenceStatus,
    /// Structured payload used by an exporter to flatten the reference.
    pub payload: Option<Value>,
    pub message: Option<String>,
}

/// Markdown note as stored. Prose only; facts stay in Run events.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordNote {
    pub note_id: String,
    pub title: Option<String>,
    pub markdown: String,
    pub updated_at_utc: String,
}

/// Note with its references resolved against this projection.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordNoteView {
    pub note_id: String,
    pub title: Option<String>,
    pub markdown: String,
    pub updated_at_utc: String,
    pub references: Vec<ResolvedReference>,
}

/// Lifecycle of a Run record: executing, awaiting summary, signed, archived.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RecordStatus {
    Recording,
    Summarizing,
    Signed,
    Archived,
}

impl RecordStatus {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Recording => "recording",
            Self::Summarizing => "summarizing",
            Self::Signed => "signed",
            Self::Archived => "archived",
        }
    }

    /// Archived is terminal for notes: the Notebook becomes read-only.
    pub fn locks_notes(self) -> bool {
        matches!(self, Self::Archived)
    }
}

/// Sign-off evidence. `signed_at_utc` is supplied by the caller so Core stays
/// clock-free and deterministic.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ArchiveSignature {
    pub signed_by: String,
    pub signed_at_utc: String,
    pub statement: Option<String>,
}

/// One archived member with its checksum.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ArchiveEntry {
    /// `asset` | `note` | `projection`.
    pub kind: String,
    /// Logical path only: root/relative for assets, `notes/<id>.md`, or
    /// `projection.json`. Never an absolute path.
    pub logical_path: String,
    pub sha256: String,
    pub size: Option<u64>,
}

/// Archive manifest: the checksum list of a complete archive package.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ArchiveManifest {
    pub schema_version: u32,
    pub run_id: String,
    pub design_version_id: String,
    pub generated_at_utc: String,
    pub entries: Vec<ArchiveEntry>,
    /// SHA256 over the canonical `"<sha256>  <logical_path>\n"` lines.
    pub manifest_sha256: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordArchiveState {
    pub status: RecordStatus,
    pub signature: Option<ArchiveSignature>,
    pub manifest: Option<ArchiveManifest>,
}

impl Default for RecordArchiveState {
    fn default() -> Self {
        Self::recording()
    }
}

impl RecordArchiveState {
    pub fn recording() -> Self {
        Self {
            status: RecordStatus::Recording,
            signature: None,
            manifest: None,
        }
    }

    pub fn notes_locked(&self) -> bool {
        self.status.locks_notes()
    }
}

/// Command error for note edits and archive transitions.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordCommandError {
    pub code: String,
    pub message: String,
}

impl RecordCommandError {
    fn new(code: &str, message: impl Into<String>) -> Self {
        Self {
            code: code.to_owned(),
            message: message.into(),
        }
    }
}

impl std::fmt::Display for RecordCommandError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for RecordCommandError {}

/// Inputs that are not stored in the Run workspace itself.
#[derive(Debug, Clone, PartialEq)]
pub struct RecordProjectionInput {
    pub view: ProjectionView,
    /// Planned FOV slots per well. A plan value, not an asset count.
    pub fov_per_well: u32,
    pub notes: Vec<RecordNote>,
    pub archive: RecordArchiveState,
    /// Watermark for running timers. Without it a running timer has no measured
    /// elapsed time; paused and completed timers still report `accumulated_s`.
    pub now_utc: Option<String>,
}

impl Default for RecordProjectionInput {
    fn default() -> Self {
        Self {
            view: ProjectionView::Unblinded,
            fov_per_well: 1,
            notes: Vec::new(),
            archive: RecordArchiveState::recording(),
            now_utc: None,
        }
    }
}

/// The single read-only projection both Record renderers consume.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RecordProjection {
    pub schema_version: u32,
    pub run_id: String,
    pub view: ProjectionView,
    pub design: RecordDesignSummary,
    pub parameters: Vec<ParameterComparison>,
    pub timers: Vec<TimerObservation>,
    pub images: ImageSlotMatrix,
    pub assets: Vec<RecordAsset>,
    pub readings: Vec<ReadingFact>,
    pub reading_issues: Vec<ReadingIssue>,
    pub readouts: Vec<ReadoutRecord>,
    pub timeline: RecordTimelineSummary,
    pub notes: Vec<RecordNoteView>,
    pub notes_locked: bool,
    pub archive: RecordArchiveState,
    pub issues: Vec<RecordIssue>,
}

/// Build the Record projection from the frozen Run workspace.
///
/// Every input is explicit: asset links and timer records come from the Run
/// store, notes and archive state from the Record store. The function only
/// reads; it never mutates the workspace or an asset.
pub fn build_record_projection(
    workspace: &RunWorkspace,
    links: &[AssetLink],
    timers: &[TimerRecord],
    input: &RecordProjectionInput,
) -> RecordProjection {
    let mut issues = Vec::new();
    let view = input.view;
    let design = project_design(&workspace.frozen_design, view);
    let parameters = compare_parameters(workspace);
    let timers = project_timers(timers, input.now_utc.as_deref(), &mut issues);
    let images = build_image_matrix(workspace, links, view, input.fov_per_well, &mut issues);
    let assets = links
        .iter()
        .map(|link| RecordAsset {
            asset_id: link.asset_id.clone(),
            task_key: link.task_key.clone(),
            sample_id: link.sample_id.clone(),
            well_id: link.well_id.clone(),
            timepoint: link.timepoint.clone(),
            fov_index: link.fov_index,
            coordinate_id: link.coordinate_id(),
        })
        .collect();
    let readings = query_readings(workspace, &ReadingQuery::default());
    let readouts = query_readout_records(workspace, &ReadoutRecordQuery::default());
    let timeline = project_timeline(workspace);
    let mut projection = RecordProjection {
        schema_version: RECORD_PROJECTION_SCHEMA_VERSION,
        run_id: workspace.run_id.clone(),
        view,
        design,
        parameters,
        timers,
        images,
        assets,
        readings: readings.facts.clone(),
        reading_issues: readings.issues.clone(),
        readouts: readouts.records.clone(),
        timeline,
        notes: Vec::new(),
        notes_locked: input.archive.notes_locked(),
        archive: input.archive.clone(),
        issues,
    };
    let (notes, note_issues) = resolve_notes(&projection, &input.notes);
    projection.notes = notes;
    projection.issues.extend(note_issues);
    projection.issues.extend(readout_issues(&readouts));
    projection
}

fn readout_issues(result: &ReadoutRecordQueryResult) -> Vec<RecordIssue> {
    result
        .issues
        .iter()
        .map(|issue| RecordIssue::new(&issue.code, issue.message.clone(), issue.step_path.clone()))
        .collect()
}

/// Planned Protocol bindings next to recorded actual parameters.
///
/// A slot is keyed by `(scope, slot)`; every actual value recorded for that key
/// is kept, including repeated recordings. Actual values never overwrite the
/// planned binding.
pub fn compare_parameters(workspace: &RunWorkspace) -> Vec<ParameterComparison> {
    let mut rows: BTreeMap<(String, String), ParameterComparison> = BTreeMap::new();
    for scope in &workspace.scopes {
        for parameter in &scope.resolved_parameters {
            rows.entry((scope.id.clone(), parameter.slot.clone()))
                .or_insert_with(|| ParameterComparison {
                    scope_id: scope.id.clone(),
                    step_path: parameter.path.clone(),
                    slot: parameter.slot.clone(),
                    label: Some(parameter.label.clone()),
                    unit: parameter.unit.clone(),
                    planned: Some(parameter.value.clone()),
                    actual: Vec::new(),
                    status: ParameterStatus::MissingActual,
                });
        }
    }

    let mut ordered: Vec<(usize, &ExecutionEventView)> =
        workspace.events.iter().enumerate().collect();
    ordered.sort_by_key(|(index, event)| {
        (
            time_key(
                event
                    .recorded_at
                    .as_deref()
                    .or(event.occurred_at.as_deref())
                    .unwrap_or_default(),
            ),
            *index,
        )
    });

    for (_, event) in ordered {
        let scope_id = event
            .assay_id
            .clone()
            .or_else(|| event.planned_output_id.clone())
            .unwrap_or_else(|| "global".to_owned());
        for entry in &event.actual_parameters {
            let Some(key) = entry["key"]
                .as_str()
                .map(str::trim)
                .filter(|key| !key.is_empty())
            else {
                continue;
            };
            let row = rows
                .entry((scope_id.clone(), key.to_owned()))
                .or_insert_with(|| ParameterComparison {
                    scope_id: scope_id.clone(),
                    step_path: event.protocol_step_path.clone(),
                    slot: key.to_owned(),
                    label: None,
                    unit: entry["unit"].as_str().map(str::to_owned),
                    planned: None,
                    actual: Vec::new(),
                    status: ParameterStatus::Unplanned,
                });
            row.actual.push(ActualParameter {
                value: entry["value"].clone(),
                unit: entry["unit"].as_str().map(str::to_owned),
                event_id: event.event_id.clone(),
                recorded_at: event
                    .recorded_at
                    .clone()
                    .or_else(|| event.occurred_at.clone()),
                source: entry["source"].as_str().map(str::to_owned),
            });
        }
    }

    let mut comparisons: Vec<ParameterComparison> = rows.into_values().collect();
    for row in &mut comparisons {
        row.status = match (&row.planned, row.actual.is_empty()) {
            (None, _) => ParameterStatus::Unplanned,
            (Some(_), true) => ParameterStatus::MissingActual,
            (Some(planned), false) => {
                if row.actual.iter().all(|actual| &actual.value == planned) {
                    ParameterStatus::Matches
                } else {
                    ParameterStatus::Differs
                }
            }
        };
    }
    comparisons
}

fn project_design(frozen: &FrozenDesignView, view: ProjectionView) -> RecordDesignSummary {
    let blind = view.is_blind();
    RecordDesignSummary {
        design_id: frozen.design_id.clone(),
        design_version_id: frozen.design_version_id.clone(),
        design_revision: frozen.design_revision,
        title: frozen.content.title.clone(),
        objective: frozen.content.objective.clone(),
        groups: frozen
            .content
            .groups
            .iter()
            .map(|group| group_view(group, blind))
            .collect(),
        assays: frozen
            .content
            .assays
            .iter()
            .map(|assay| assay_view(assay, blind))
            .collect(),
    }
}

fn group_view(group: &DesignGroup, blind: bool) -> RecordGroupView {
    if blind {
        return RecordGroupView {
            group_id: None,
            name: String::new(),
            role: None,
            condition: None,
            independent_n: None,
            technical_repeats: None,
            blinded: true,
        };
    }
    RecordGroupView {
        group_id: Some(group.id.clone()),
        name: group.name.clone(),
        role: group.role.clone(),
        condition: group.condition.clone(),
        independent_n: group.independent_n,
        technical_repeats: group.technical_repeats,
        blinded: false,
    }
}

fn assay_view(assay: &DesignAssay, blind: bool) -> RecordAssayView {
    RecordAssayView {
        assay_id: assay.id.clone(),
        title: assay.title.clone(),
        endpoint: assay.endpoint.clone(),
        method: assay.method.clone(),
        unit: assay.unit.clone(),
        group_ids: if blind {
            Vec::new()
        } else {
            assay.group_ids.clone()
        },
        timepoints: assay.timepoints.clone(),
        timepoint_relation: assay.timepoint_relation.clone(),
        protocol_version_id: assay.protocol_version_id.clone(),
    }
}

fn project_timers(
    timers: &[TimerRecord],
    now_utc: Option<&str>,
    issues: &mut Vec<RecordIssue>,
) -> Vec<TimerObservation> {
    timers
        .iter()
        .map(|timer| {
            // A running timer has no measured elapsed time without a
            // watermark; reporting `accumulated_s` (often 0) would invent one.
            let measured = match (timer.status, now_utc) {
                (TimerStatus::Running, None) => Err(
                    "running timer requires a projection watermark".to_owned(),
                ),
                (_status, Some(now)) => timer.elapsed_s(now),
                // Non-running timers ignore the watermark entirely.
                (_status, None) => timer.elapsed_s("1970-01-01T00:00:00Z"),
            };
            let (elapsed_s, remaining_s) = match measured {
                Ok(elapsed) => (
                    Some(elapsed),
                    timer.planned_s.map(|planned| planned.saturating_sub(elapsed)),
                ),
                Err(message) => {
                    issues.push(RecordIssue::new(
                        "timer_watermark_missing",
                        format!("Timer {} 无法计算实测耗时：{message}", timer.timer_id),
                        Some(timer.timer_id.clone()),
                    ));
                    (None, None)
                }
            };
            TimerObservation {
                timer_id: timer.timer_id.clone(),
                task_key: timer.task_key.clone(),
                phase_index: timer.phase_index,
                mode: timer.mode,
                status: timer.status,
                planned_s: timer.planned_s,
                elapsed_s,
                remaining_s,
                round_index: timer.round_index,
                round_count: timer.round_count,
            }
        })
        .collect()
}

fn project_timeline(workspace: &RunWorkspace) -> RecordTimelineSummary {
    let result = query_timeline(workspace, &TimelineQuery::default());
    let first = result.events.first().and_then(|event| {
        event
            .recorded_at
            .clone()
            .or_else(|| event.occurred_at.clone())
    });
    let last = result.events.last().and_then(|event| {
        event
            .recorded_at
            .clone()
            .or_else(|| event.occurred_at.clone())
    });
    RecordTimelineSummary {
        events: result.events,
        failures: result.failures,
        retries: result.retries,
        issues: result.issues,
        skipped_by_rule: workspace
            .events
            .iter()
            .filter(|event| event.skipped_by_rule)
            .count() as u32,
        first_recorded_at: first,
        last_recorded_at: last,
    }
}

fn assay_timepoints(workspace: &RunWorkspace, plate: &RunPlateView) -> Vec<Option<String>> {
    let planned = plate
        .assay_id
        .as_deref()
        .and_then(|assay_id| {
            workspace
                .frozen_design
                .content
                .assays
                .iter()
                .find(|assay| assay.id == assay_id)
        })
        .map(|assay| assay.timepoints.clone())
        .unwrap_or_default();
    if !planned.is_empty() {
        return planned.into_iter().map(Some).collect();
    }
    if let Some(timepoint) = plate.timepoint.as_deref().filter(|tp| !tp.is_empty()) {
        return vec![Some(timepoint.to_owned())];
    }
    vec![None]
}

fn build_image_matrix(
    workspace: &RunWorkspace,
    links: &[AssetLink],
    view: ProjectionView,
    fov_per_well: u32,
    issues: &mut Vec<RecordIssue>,
) -> ImageSlotMatrix {
    let fov_per_well = fov_per_well.max(1);
    let blind = view.is_blind();

    let mut coordinate_plates: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for plate in &workspace.plates {
        for well in &plate.wells {
            coordinate_plates
                .entry(well.coordinate.to_ascii_uppercase())
                .or_default()
                .push(plate.plate_id.clone());
        }
    }

    // Planned slot key -> asset id. Keys are (plate, well, timepoint, fov).
    let mut slot_assets: HashMap<(String, String, Option<String>, u32), String> = HashMap::new();
    let mut unlocated = Vec::new();
    let mut unplanned = Vec::new();
    let mut ambiguous_reported: BTreeSet<String> = BTreeSet::new();

    for link in links {
        let Some(well_id) = link
            .well_id
            .as_deref()
            .map(str::trim)
            .filter(|well| !well.is_empty())
        else {
            unlocated.push(link.asset_id.clone());
            issues.push(RecordIssue::new(
                "asset_without_well",
                format!("资产 {} 没有孔位坐标，无法进入完成度矩阵", link.asset_id),
                Some(link.asset_id.clone()),
            ));
            continue;
        };
        let coordinate = well_id.to_ascii_uppercase();
        let Some(plate_ids) = coordinate_plates.get(&coordinate) else {
            unlocated.push(link.asset_id.clone());
            issues.push(RecordIssue::new(
                "unknown_well",
                format!("资产 {} 的孔位 {coordinate} 不在冻结板图中", link.asset_id),
                Some(link.asset_id.clone()),
            ));
            continue;
        };
        if plate_ids.len() > 1 && ambiguous_reported.insert(coordinate.clone()) {
            issues.push(RecordIssue::new(
                "ambiguous_well",
                format!(
                    "孔位 {coordinate} 同时出现在 {} 块板中，资产坐标不唯一",
                    plate_ids.len()
                ),
                Some(coordinate.clone()),
            ));
        }
        let plate_id = plate_ids[0].clone();
        let timepoint = link
            .timepoint
            .as_deref()
            .map(str::trim)
            .filter(|tp| !tp.is_empty())
            .map(str::to_owned);
        let fov = link.fov_index.unwrap_or(0);
        if fov >= fov_per_well {
            unplanned.push(link.asset_id.clone());
            issues.push(RecordIssue::new(
                "fov_out_of_range",
                format!(
                    "资产 {} 的视野序号 {fov} 超出计划的 {fov_per_well} 个视野",
                    link.asset_id
                ),
                Some(link.asset_id.clone()),
            ));
            continue;
        }
        let key = (plate_id, coordinate, timepoint, fov);
        if let Some(previous) = slot_assets.get(&key) {
            issues.push(RecordIssue::new(
                "duplicate_asset_slot",
                format!(
                    "同一槽位被重复挂载：{} 与 {}；保留先到的资产",
                    previous, link.asset_id
                ),
                Some(link.asset_id.clone()),
            ));
            continue;
        }
        slot_assets.insert(key, link.asset_id.clone());
    }

    let mut plates = Vec::new();
    let mut planned_total = 0u32;
    let mut filled_total = 0u32;
    for plate in &workspace.plates {
        let timepoints = assay_timepoints(workspace, plate);
        let mut wells = Vec::new();
        let mut plate_planned = 0u32;
        let mut plate_filled = 0u32;
        for well in &plate.wells {
            let coordinate = well.coordinate.to_ascii_uppercase();
            let mut slots = Vec::new();
            let mut filled = 0u32;
            for timepoint in &timepoints {
                for fov in 0..fov_per_well {
                    let asset_id = slot_assets
                        .get(&(
                            plate.plate_id.clone(),
                            coordinate.clone(),
                            timepoint.clone(),
                            fov,
                        ))
                        .cloned();
                    if asset_id.is_some() {
                        filled += 1;
                    }
                    slots.push(ImageSlot {
                        timepoint: timepoint.clone(),
                        fov_index: fov,
                        asset_id,
                    });
                }
            }
            let planned = slots.len() as u32;
            plate_planned += planned;
            plate_filled += filled;
            wells.push(ImageWellRow {
                coordinate: well.coordinate.clone(),
                group_label: if blind {
                    None
                } else {
                    well.group_label.clone()
                },
                slots,
                filled_slots: filled,
                planned_slots: planned,
            });
        }
        planned_total += plate_planned;
        filled_total += plate_filled;
        plates.push(ImagePlateRow {
            plate_id: plate.plate_id.clone(),
            name: plate.name.clone(),
            timepoints,
            wells,
            planned_slots: plate_planned,
            filled_slots: plate_filled,
        });
    }

    ImageSlotMatrix {
        fov_per_well,
        plates,
        planned_slots: planned_total,
        filled_slots: filled_total,
        missing_slots: planned_total.saturating_sub(filled_total),
        unlocated_asset_ids: unlocated,
        unplanned_asset_ids: unplanned,
    }
}

// ---------------------------------------------------------------------------
// Dynamic note references
// ---------------------------------------------------------------------------

struct ParsedReference {
    raw: String,
    name: String,
    args: Vec<String>,
}

fn is_word_char(character: char) -> bool {
    character.is_alphanumeric() || character == '_'
}

/// Find `@name` and `@name(a, b)` tokens. An `@` inside a word (an email
/// address, for example) is not a reference.
fn parse_references(markdown: &str) -> Vec<ParsedReference> {
    let chars: Vec<char> = markdown.chars().collect();
    let mut references = Vec::new();
    let mut index = 0usize;
    while index < chars.len() {
        if chars[index] != '@' {
            index += 1;
            continue;
        }
        if index > 0 && is_word_char(chars[index - 1]) {
            index += 1;
            continue;
        }
        let mut cursor = index + 1;
        let name_start = cursor;
        while cursor < chars.len()
            && (chars[cursor].is_ascii_lowercase()
                || chars[cursor].is_ascii_digit()
                || chars[cursor] == '_')
        {
            cursor += 1;
        }
        if cursor == name_start {
            index += 1;
            continue;
        }
        let name: String = chars[name_start..cursor].iter().collect();
        let mut args = Vec::new();
        let mut end = cursor - 1;
        if cursor < chars.len() && chars[cursor] == '(' {
            let args_start = cursor + 1;
            let mut close = None;
            for (offset, character) in chars.iter().enumerate().skip(args_start) {
                if *character == ')' {
                    close = Some(offset);
                    break;
                }
                if *character == '(' {
                    break;
                }
            }
            let Some(close) = close else {
                index += 1;
                continue;
            };
            let raw_args: String = chars[args_start..close].iter().collect();
            args = raw_args
                .split(',')
                .map(str::trim)
                .filter(|arg| !arg.is_empty())
                .map(str::to_owned)
                .collect();
            end = close;
        }
        let raw: String = chars[index..=end].iter().collect();
        references.push(ParsedReference { raw, name, args });
        index = end + 1;
    }
    references
}

fn resolve_notes(
    projection: &RecordProjection,
    notes: &[RecordNote],
) -> (Vec<RecordNoteView>, Vec<RecordIssue>) {
    let mut views = Vec::with_capacity(notes.len());
    let mut issues = Vec::new();
    let mut seen = BTreeSet::new();
    for note in notes {
        if !seen.insert(note.note_id.as_str()) {
            issues.push(RecordIssue::new(
                "duplicate_note_id",
                format!("笔记 ID 重复：{}", note.note_id),
                Some(note.note_id.clone()),
            ));
            continue;
        }
        let mut references = Vec::new();
        for parsed in parse_references(&note.markdown) {
            let resolved = resolve_reference(projection, &parsed);
            if let (ReferenceStatus::Unresolved, Some(message)) = (resolved.status, resolved.message.clone()) {
                issues.push(RecordIssue::new(
                    "unresolved_reference",
                    message,
                    Some(format!("{}#{}", note.note_id, parsed.raw)),
                ));
            }
            references.push(resolved);
        }
        views.push(RecordNoteView {
            note_id: note.note_id.clone(),
            title: note.title.clone(),
            markdown: note.markdown.clone(),
            updated_at_utc: note.updated_at_utc.clone(),
            references,
        });
    }
    (views, issues)
}

fn unresolved(parsed: &ParsedReference, kind: ReferenceKind, message: impl Into<String>) -> ResolvedReference {
    ResolvedReference {
        raw: parsed.raw.clone(),
        kind,
        args: parsed.args.clone(),
        status: ReferenceStatus::Unresolved,
        payload: None,
        message: Some(message.into()),
    }
}

fn resolve_reference(
    projection: &RecordProjection,
    parsed: &ParsedReference,
) -> ResolvedReference {
    let kind = match parsed.name.as_str() {
        "matrix" => ReferenceKind::Matrix,
        "params" => ReferenceKind::Params,
        "timeline" => ReferenceKind::Timeline,
        _ => {
            return unresolved(
                parsed,
                ReferenceKind::Unknown,
                format!("未知的动态引用 @{}", parsed.name),
            )
        }
    };
    let payload = match kind {
        ReferenceKind::Matrix => matrix_payload(projection, &parsed.args),
        ReferenceKind::Params => params_payload(projection, &parsed.args),
        ReferenceKind::Timeline => timeline_payload(projection, &parsed.args),
        ReferenceKind::Unknown => unreachable!(),
    };
    match payload {
        Ok(payload) => ResolvedReference {
            raw: parsed.raw.clone(),
            kind,
            args: parsed.args.clone(),
            status: ReferenceStatus::Resolved,
            payload: Some(payload),
            message: None,
        },
        Err(message) => unresolved(parsed, kind, message),
    }
}

fn matrix_timepoints(projection: &RecordProjection) -> Vec<Option<String>> {
    let mut timepoints: Vec<Option<String>> = Vec::new();
    for plate in &projection.images.plates {
        for timepoint in &plate.timepoints {
            if !timepoints.contains(timepoint) {
                timepoints.push(timepoint.clone());
            }
        }
    }
    timepoints
}

fn matrix_payload(projection: &RecordProjection, args: &[String]) -> Result<Value, String> {
    let available = matrix_timepoints(projection);
    let requested = args
        .iter()
        .map(|arg| Some(arg.clone()))
        .collect::<Vec<_>>();
    let (selected, unknown) = if args.is_empty() {
        (available.clone(), Vec::new())
    } else {
        let known = requested
            .iter()
            .filter(|timepoint| available.contains(timepoint))
            .cloned()
            .collect::<Vec<_>>();
        if known.is_empty() {
            return Err(format!("时间点不存在：{}", args.join(", ")));
        }
        let unknown = requested
            .iter()
            .filter(|timepoint| !available.contains(timepoint))
            .filter_map(|timepoint| timepoint.clone())
            .collect::<Vec<_>>();
        (known, unknown)
    };
    let mut wells = Vec::new();
    for plate in &projection.images.plates {
        for well in &plate.wells {
            let mut cells = Vec::new();
            for timepoint in &selected {
                let slots = well
                    .slots
                    .iter()
                    .filter(|slot| &slot.timepoint == timepoint)
                    .collect::<Vec<_>>();
                if slots.is_empty() {
                    continue;
                }
                let asset_ids = slots
                    .iter()
                    .filter_map(|slot| slot.asset_id.clone())
                    .collect::<Vec<_>>();
                let mut cell = json!({
                    "timepoint": timepoint,
                    "planned": slots.len(),
                    "filled": asset_ids.len(),
                    "asset_ids": asset_ids,
                });
                if !projection.view.is_blind() {
                    cell["group_label"] = json!(well.group_label);
                }
                cells.push(cell);
            }
            if cells.is_empty() {
                continue;
            }
            wells.push(json!({
                "plate_id": plate.plate_id,
                "coordinate": well.coordinate,
                "cells": cells,
            }));
        }
    }
    let mut payload = json!({
        "kind": "image_matrix",
        "timepoints": selected,
        "wells": wells,
    });
    if !unknown.is_empty() {
        payload["unknown_timepoints"] = json!(unknown);
    }
    Ok(payload)
}

fn params_payload(projection: &RecordProjection, args: &[String]) -> Result<Value, String> {
    let rows = projection
        .parameters
        .iter()
        .filter(|row| {
            args.is_empty()
                || args.iter().any(|arg| arg == &row.scope_id || arg == &row.slot)
        })
        .map(|row| {
            json!({
                "scope_id": row.scope_id,
                "step_path": row.step_path,
                "slot": row.slot,
                "label": row.label,
                "unit": row.unit,
                "planned": row.planned,
                "actual": row.actual.iter().map(|actual| json!({
                    "value": actual.value,
                    "unit": actual.unit,
                    "event_id": actual.event_id,
                    "recorded_at": actual.recorded_at,
                })).collect::<Vec<_>>(),
                "status": row.status,
            })
        })
        .collect::<Vec<_>>();
    if rows.is_empty() {
        return Err(format!("参数不存在：{}", args.join(", ")));
    }
    Ok(json!({ "kind": "parameters", "rows": rows }))
}

fn timeline_payload(projection: &RecordProjection, args: &[String]) -> Result<Value, String> {
    if !args.is_empty() {
        return Err(format!("@timeline 不接受参数：{}", args.join(", ")));
    }
    Ok(json!({
        "kind": "timeline",
        "events": projection.timeline.events.len(),
        "failures": projection.timeline.failures.len(),
        "retries": projection.timeline.retries.len(),
        "skipped_by_rule": projection.timeline.skipped_by_rule,
        "first_recorded_at": projection.timeline.first_recorded_at,
        "last_recorded_at": projection.timeline.last_recorded_at,
    }))
}

// ---------------------------------------------------------------------------
// Note edits, sign-off and archive
// ---------------------------------------------------------------------------

/// Upsert one Markdown note.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct NoteEdit {
    pub note_id: String,
    pub title: Option<String>,
    pub markdown: String,
    pub updated_at_utc: String,
}

/// Apply a note edit in place.
///
/// An archived Run locks the Notebook: the edit is rejected and the stored
/// notes stay untouched. Signed but not archived records remain editable.
pub fn apply_note_edit(
    notes: &mut Vec<RecordNote>,
    state: &RecordArchiveState,
    edit: &NoteEdit,
) -> Result<(), RecordCommandError> {
    if state.notes_locked() {
        return Err(RecordCommandError::new(
            "note_locked",
            "Run 已签核归档，笔记只读",
        ));
    }
    let note_id = edit.note_id.trim();
    if note_id.is_empty() {
        return Err(RecordCommandError::new("missing_note_id", "笔记必须有稳定 ID"));
    }
    if !is_utc_timestamp(&edit.updated_at_utc) {
        return Err(RecordCommandError::new(
            "invalid_timestamp",
            format!("笔记更新时间不是 RFC3339：{}", edit.updated_at_utc),
        ));
    }
    match notes
        .iter_mut()
        .find(|note| note.note_id.as_str() == note_id)
    {
        Some(note) => {
            note.title = edit.title.clone();
            note.markdown = edit.markdown.clone();
            note.updated_at_utc = edit.updated_at_utc.clone();
        }
        None => notes.push(RecordNote {
            note_id: note_id.to_owned(),
            title: edit.title.clone(),
            markdown: edit.markdown.clone(),
            updated_at_utc: edit.updated_at_utc.clone(),
        }),
    }
    Ok(())
}

/// `recording -> summarizing`. Run execution is over; the record awaits summary.
pub fn summarize_record(
    state: &RecordArchiveState,
) -> Result<RecordArchiveState, RecordCommandError> {
    if state.status != RecordStatus::Recording {
        return Err(RecordCommandError::new(
            "invalid_transition",
            format!(
                "只有 recording 可以进入 summarizing，当前为 {}",
                state.status.as_str()
            ),
        ));
    }
    Ok(RecordArchiveState {
        status: RecordStatus::Summarizing,
        signature: None,
        manifest: None,
    })
}

/// `summarizing -> signed`.
pub fn sign_record(
    state: &RecordArchiveState,
    signature: &ArchiveSignature,
) -> Result<RecordArchiveState, RecordCommandError> {
    if state.status != RecordStatus::Summarizing {
        return Err(RecordCommandError::new(
            "invalid_transition",
            format!(
                "只有 summarizing 可以签核，当前为 {}",
                state.status.as_str()
            ),
        ));
    }
    validate_signature(signature)?;
    Ok(RecordArchiveState {
        status: RecordStatus::Signed,
        signature: Some(signature.clone()),
        manifest: None,
    })
}

/// `signed -> archived`. The manifest digest is verified before the state
/// becomes terminal.
pub fn archive_record(
    state: &RecordArchiveState,
    manifest: &ArchiveManifest,
) -> Result<RecordArchiveState, RecordCommandError> {
    if state.status == RecordStatus::Archived {
        return Err(RecordCommandError::new("already_archived", "Run 已归档"));
    }
    if state.status != RecordStatus::Signed {
        return Err(RecordCommandError::new(
            "invalid_transition",
            format!(
                "只有 signed 可以归档，当前为 {}",
                state.status.as_str()
            ),
        ));
    }
    verify_archive_manifest(manifest)?;
    Ok(RecordArchiveState {
        status: RecordStatus::Archived,
        signature: state.signature.clone(),
        manifest: Some(manifest.clone()),
    })
}

/// Sign and archive in one step: `summarizing -> signed -> archived`.
///
/// The manifest fingerprints every linked asset, every note and the projection
/// itself. Generation time is the signature time, so Core needs no clock.
pub fn sign_and_archive(
    state: &RecordArchiveState,
    signature: &ArchiveSignature,
    projection: &RecordProjection,
    links: &[AssetLink],
    assets: &[AssetRef],
) -> Result<RecordArchiveState, RecordCommandError> {
    if state.status == RecordStatus::Archived {
        return Err(RecordCommandError::new("already_archived", "Run 已归档"));
    }
    let signed = sign_record(state, signature)?;
    let manifest = build_archive_manifest(signature, projection, links, assets)?;
    archive_record(&signed, &manifest)
}

fn build_archive_manifest(
    signature: &ArchiveSignature,
    projection: &RecordProjection,
    links: &[AssetLink],
    assets: &[AssetRef],
) -> Result<ArchiveManifest, RecordCommandError> {
    let by_id: HashMap<&str, &AssetRef> = assets
        .iter()
        .map(|asset| (asset.id.as_str(), asset))
        .collect();
    let mut entries: BTreeMap<String, ArchiveEntry> = BTreeMap::new();

    let mut missing = Vec::new();
    for link in links {
        let Some(asset) = by_id.get(link.asset_id.as_str()) else {
            missing.push(format!("{}（无 AssetRef）", link.asset_id));
            continue;
        };
        if !is_sha256(&asset.sha256) {
            missing.push(format!("{}（无 SHA256）", link.asset_id));
            continue;
        }
        if asset.root_id.trim().is_empty() || asset.relative_path.trim().is_empty() {
            missing.push(format!("{}（无逻辑路径）", link.asset_id));
            continue;
        }
        let logical_path = format!("{}/{}", asset.root_id, asset.relative_path);
        entries.insert(
            logical_path.clone(),
            ArchiveEntry {
                kind: "asset".to_owned(),
                logical_path,
                sha256: asset.sha256.to_ascii_lowercase(),
                size: Some(asset.size),
            },
        );
    }
    if !missing.is_empty() {
        return Err(RecordCommandError::new(
            "asset_checksum_missing",
            format!("归档缺少资产校验信息：{}", missing.join("、")),
        ));
    }

    for note in &projection.notes {
        let logical_path = format!("notes/{}.md", note.note_id);
        entries.insert(
            logical_path.clone(),
            ArchiveEntry {
                kind: "note".to_owned(),
                logical_path,
                sha256: sha256_hex(note.markdown.as_bytes()),
                size: Some(note.markdown.len() as u64),
            },
        );
    }

    let projection_json = serde_json::to_vec(projection).unwrap_or_default();
    entries.insert(
        "projection.json".to_owned(),
        ArchiveEntry {
            kind: "projection".to_owned(),
            logical_path: "projection.json".to_owned(),
            sha256: sha256_hex(&projection_json),
            size: Some(projection_json.len() as u64),
        },
    );

    let manifest = ArchiveManifest {
        schema_version: ARCHIVE_MANIFEST_SCHEMA_VERSION,
        run_id: projection.run_id.clone(),
        design_version_id: projection.design.design_version_id.clone(),
        generated_at_utc: signature.signed_at_utc.clone(),
        manifest_sha256: String::new(),
        entries: entries.into_values().collect(),
    };
    let digest = manifest_digest(&manifest.entries);
    Ok(ArchiveManifest {
        manifest_sha256: digest,
        ..manifest
    })
}

/// Recompute the manifest digest and validate every checksum shape.
pub fn verify_archive_manifest(manifest: &ArchiveManifest) -> Result<(), RecordCommandError> {
    if manifest.schema_version != ARCHIVE_MANIFEST_SCHEMA_VERSION {
        return Err(RecordCommandError::new(
            "unsupported_manifest",
            format!("不支持的归档清单版本 {}", manifest.schema_version),
        ));
    }
    for entry in &manifest.entries {
        if !is_sha256(&entry.sha256) {
            return Err(RecordCommandError::new(
                "invalid_checksum",
                format!("{} 的 SHA256 无效", entry.logical_path),
            ));
        }
        if entry.logical_path.starts_with('/') || entry.logical_path.contains(":\\") {
            return Err(RecordCommandError::new(
                "absolute_path",
                format!("归档路径必须是逻辑路径：{}", entry.logical_path),
            ));
        }
    }
    let expected = manifest_digest(&manifest.entries);
    if expected != manifest.manifest_sha256 {
        return Err(RecordCommandError::new(
            "manifest_digest_mismatch",
            "归档清单摘要与条目不一致",
        ));
    }
    Ok(())
}

fn validate_signature(signature: &ArchiveSignature) -> Result<(), RecordCommandError> {
    if signature.signed_by.trim().is_empty() {
        return Err(RecordCommandError::new("missing_signer", "签核必须记录签核人"));
    }
    if !is_utc_timestamp(&signature.signed_at_utc) {
        return Err(RecordCommandError::new(
            "invalid_timestamp",
            format!("签核时间不是 RFC3339：{}", signature.signed_at_utc),
        ));
    }
    Ok(())
}

fn manifest_digest(entries: &[ArchiveEntry]) -> String {
    let mut canonical = String::new();
    for entry in entries {
        canonical.push_str(&entry.sha256);
        canonical.push_str("  ");
        canonical.push_str(&entry.logical_path);
        canonical.push('\n');
    }
    sha256_hex(canonical.as_bytes())
}

fn sha256_hex(bytes: &[u8]) -> String {
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    format!("{:x}", hasher.finalize())
}

fn is_sha256(value: &str) -> bool {
    value.len() == 64 && value.chars().all(|character| character.is_ascii_hexdigit())
}

/// Accepts `YYYY-MM-DDTHH:MM:SS` with an optional fractional part and a `Z` or
/// numeric offset. Core compares nothing; it only rejects unparseable stamps.
fn is_utc_timestamp(value: &str) -> bool {
    let value = value.trim();
    if value.len() < 20 || !value.is_ascii() {
        return false;
    }
    let bytes = value.as_bytes();
    let digits = [0usize, 1, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17, 18];
    if bytes[4] != b'-' || bytes[7] != b'-' || bytes[10] != b'T' {
        return false;
    }
    for index in digits {
        if !bytes[index].is_ascii_digit() {
            return false;
        }
    }
    if bytes[13] != b':' || bytes[16] != b':' {
        return false;
    }
    let rest = &value[19..];
    rest == "Z"
        || rest.starts_with('.')
        || rest.starts_with('+')
        || rest.starts_with('-')
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::assets::{Availability, MediaType};
    use crate::run_workspace;
    use serde_json::json;

    fn design_snapshot() -> Value {
        json!({
            "title":"HUVEC 成管",
            "objective":"比较 3%/5%/7% GelMA",
            "experimental_unit":"well",
            "group_specs":[
                {"id":"g3","name":"3%","role":"treatment","condition":"3% GelMA","technical_repeats":6},
                {"id":"g5","name":"5%","role":"treatment","condition":"5% GelMA"},
                {"id":"g7","name":"7%","role":"treatment","condition":"7% GelMA"}
            ],
            "assays":[{
                "id":"tube","title":"Tube formation","endpoint":"tube_length",
                "method":"microscopy","unit":"um","group_ids":["g3","g5","g7"],
                "timepoints":["6h","12h","24h"],
                "timepoint_relation":"same_physical_sample_repeated",
                "protocol_version_id":"pv1"
            }],
            "sample_allocations":[],"planned_outputs":[],"evidence_conflicts":[]
        })
    }

    fn design_snapshot_without_timepoints() -> Value {
        let mut design = design_snapshot();
        design["assays"][0]["timepoints"] = json!([]);
        design
    }

    fn run_data() -> Value {
        json!({
            "status":"recording",
            "protocol_manifest":[{
                "assay_id":"tube","assay_title":"Tube formation","protocol_version_id":"pv1",
                "resolved_parameters":[{
                    "slot":"curing_s","label":"405nm 固化秒数","value":45,"unit":"s","provenance":[]
                }],
                "unresolved_parameters":[],
                "steps":[
                    {"path":"root/seed","use_id":"seed","title":"Seed","instruction":"seed",
                     "parameters":[],"unresolved_parameter_refs":[],
                     "source_block_version_id":"b1","time_scope":"once"},
                    {"path":"root/image","use_id":"image","title":"Image","instruction":"image",
                     "parameters":[],"unresolved_parameter_refs":[],
                     "source_block_version_id":"b1","time_scope":"per_timepoint"}
                ]
            }],
            "material_manifest":[]
        })
    }

    fn plate(wells: Vec<Value>) -> Value {
        json!({
            "id":"plate-1","name":"12 孔板","format":"12_well","rows":3,"columns":4,
            "assay_id":"tube",
            "wells":wells
        })
    }

    fn well(coordinate: &str, group: &str) -> Value {
        json!({
            "well_id":coordinate,
            "group_label":group,
            "sample_id":format!("S-{coordinate}")
        })
    }

    fn workspace_with(
        design: &Value,
        plate_data: Vec<Value>,
        events: Vec<Value>,
    ) -> RunWorkspace {
        run_workspace::build("r1", "d1", "dv1", 1, design, &run_data(), &plate_data, &events)
            .expect("workspace")
    }

    fn twelve_well_workspace() -> RunWorkspace {
        let wells = vec![
            well("A1", "3%"),
            well("A2", "3%"),
            well("B1", "5%"),
            well("B2", "7%"),
        ];
        workspace_with(&design_snapshot(), vec![plate(wells)], vec![])
    }

    fn link(asset_id: &str, well_id: Option<&str>, timepoint: Option<&str>, fov: Option<u32>) -> AssetLink {
        AssetLink {
            asset_id: asset_id.to_owned(),
            run_id: "r1".to_owned(),
            task_key: "task-image".to_owned(),
            sample_id: None,
            well_id: well_id.map(str::to_owned),
            timepoint: timepoint.map(str::to_owned),
            fov_index: fov,
        }
    }

    fn asset_ref(id: &str, path: &str, checksum: &str) -> AssetRef {
        AssetRef {
            id: id.to_owned(),
            root_id: "run-r1".to_owned(),
            relative_path: path.to_owned(),
            media_type: MediaType::Image,
            size: 1024,
            mod_time: "2026-11-15T00:00:00Z".to_owned(),
            sha256: checksum.to_owned(),
            availability: Availability::Available,
        }
    }

    fn checksum(seed: &str) -> String {
        sha256_hex(seed.as_bytes())
    }

    fn note(markdown: &str) -> RecordNote {
        RecordNote {
            note_id: "n1".to_owned(),
            title: Some("研判".to_owned()),
            markdown: markdown.to_owned(),
            updated_at_utc: "2026-11-15T09:00:00Z".to_owned(),
        }
    }

    #[test]
    fn blind_projection_masks_group_identity_and_keeps_slots() {
        let workspace = twelve_well_workspace();
        let links = vec![link("a1", Some("A1"), Some("6h"), Some(0))];
        let unblinded = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput::default(),
        );
        assert_eq!(unblinded.view, ProjectionView::Unblinded);
        assert_eq!(unblinded.design.groups[0].group_id.as_deref(), Some("g3"));
        assert_eq!(unblinded.design.groups[0].name, "3%");
        assert_eq!(unblinded.design.assays[0].group_ids.len(), 3);
        assert_eq!(
            unblinded.images.plates[0].wells[0].group_label.as_deref(),
            Some("3%")
        );

        let blind_input = RecordProjectionInput {
            view: ProjectionView::Blind,
            fov_per_well: 1,
            notes: Vec::new(),
            archive: RecordArchiveState::recording(),
            now_utc: None,
        };
        let blind = build_record_projection(&workspace, &links, &[], &blind_input);
        assert!(blind.design.groups.iter().all(|group| group.blinded));
        assert!(blind
            .design
            .groups
            .iter()
            .all(|group| group.group_id.is_none() && group.name.is_empty()));
        assert!(blind.design.assays[0].group_ids.is_empty());
        assert_eq!(blind.images.plates[0].wells[0].group_label, None);
        // Slots, wells and filled assets are execution facts, not group facts.
        assert_eq!(blind.images.filled_slots, 1);
        let serialized = serde_json::to_string(&blind).expect("serialize");
        // No group id, name, role or condition survives the mask.
        assert!(!serialized.contains("\"g3\""));
        assert!(!serialized.contains("\"condition\":\""));
        assert!(!serialized.contains("\"group_label\":\""));
        assert_eq!(serialized.matches("\"blinded\":true").count(), 3);
    }

    #[test]
    fn image_matrix_plans_162_slots_and_counts_filled_per_well() {
        // 27 wells x 6 FOV without assay timepoints = the 162-photo plan.
        let coordinates = [
            "A1", "A2", "A3", "A4", "A5", "A6", "A7", "A8", "A9", "B1", "B2", "B3", "B4",
            "B5", "B6", "B7", "B8", "B9", "C1", "C2", "C3", "C4", "C5", "C6", "C7", "C8",
            "C9",
        ];
        let wells = coordinates
            .iter()
            .map(|coordinate| well(coordinate, "3%"))
            .collect::<Vec<_>>();
        let workspace = workspace_with(
            &design_snapshot_without_timepoints(),
            vec![plate(wells)],
            vec![],
        );
        let links = (0..6)
            .map(|fov| link(&format!("img-{fov}"), Some("A1"), None, Some(fov)))
            .collect::<Vec<_>>();
        let projection = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput {
                fov_per_well: 6,
                ..RecordProjectionInput::default()
            },
        );
        assert_eq!(projection.images.planned_slots, 162);
        assert_eq!(projection.images.filled_slots, 6);
        assert_eq!(projection.images.missing_slots, 156);
        let a1 = &projection.images.plates[0].wells[0];
        assert_eq!(a1.coordinate, "A1");
        assert_eq!(a1.filled_slots, 6);
        assert_eq!(a1.planned_slots, 6);
        let a2 = &projection.images.plates[0].wells[1];
        assert_eq!(a2.filled_slots, 0);
        assert_eq!(a2.planned_slots, 6);
        assert!(projection.issues.is_empty());
    }

    #[test]
    fn duplicate_slot_keeps_first_asset_and_partial_timepoints_are_reported() {
        let workspace = twelve_well_workspace();
        let links = vec![
            link("first", Some("A1"), Some("6h"), Some(0)),
            link("second", Some("A1"), Some("6h"), Some(0)),
        ];
        let projection = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput {
                notes: vec![note("@matrix(6h, 48h)")],
                ..RecordProjectionInput::default()
            },
        );
        assert_eq!(projection.images.filled_slots, 1);
        assert_eq!(
            projection.images.plates[0].wells[0].slots[0]
                .asset_id
                .as_deref(),
            Some("first")
        );
        assert!(projection
            .issues
            .iter()
            .any(|issue| issue.code == "duplicate_asset_slot"));

        let payload = projection.notes[0].references[0]
            .payload
            .as_ref()
            .expect("matrix payload");
        assert_eq!(payload["timepoints"], json!(["6h"]));
        assert_eq!(payload["unknown_timepoints"], json!(["48h"]));
    }

    #[test]
    fn unlocated_and_out_of_plan_assets_are_reported() {
        let workspace = twelve_well_workspace();
        let links = vec![
            link("no-well", None, Some("6h"), Some(0)),
            link("bad-well", Some("Z9"), Some("6h"), Some(0)),
            link("fov-9", Some("A1"), Some("6h"), Some(9)),
        ];
        let projection = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput {
                fov_per_well: 6,
                ..RecordProjectionInput::default()
            },
        );
        assert_eq!(
            projection.images.unlocated_asset_ids,
            vec!["no-well".to_owned(), "bad-well".to_owned()]
        );
        assert_eq!(projection.images.unplanned_asset_ids, vec!["fov-9".to_owned()]);
        assert_eq!(projection.images.filled_slots, 0);
        let codes = projection
            .issues
            .iter()
            .map(|issue| issue.code.as_str())
            .collect::<Vec<_>>();
        assert!(codes.contains(&"asset_without_well"));
        assert!(codes.contains(&"unknown_well"));
        assert!(codes.contains(&"fov_out_of_range"));
    }

    #[test]
    fn parameters_keep_planned_binding_next_to_actual_values() {
        let events = vec![
            json!({
                "id":"e1","event_type":"measurement","protocol_step_path":"root/seed",
                "assay_id":"tube","recorded_at":"2026-11-15T08:00:00Z",
                "actual_parameters":[{"key":"curing_s","value":52,"unit":"s"}]
            }),
            json!({
                "id":"e2","event_type":"measurement","protocol_step_path":"root/seed",
                "assay_id":"tube","recorded_at":"2026-11-15T08:05:00Z",
                "actual_parameters":[{"key":"curing_s","value":52,"unit":"s"},
                                     {"key":"rpm","value":2000,"unit":"rpm"}]
            }),
        ];
        let workspace = workspace_with(&design_snapshot(), vec![], events);
        let parameters = compare_parameters(&workspace);
        let curing = parameters
            .iter()
            .find(|row| row.slot == "curing_s")
            .expect("curing row");
        assert_eq!(curing.planned, Some(json!(45)));
        assert_eq!(curing.status, ParameterStatus::Differs);
        assert_eq!(curing.actual.len(), 2);
        assert_eq!(curing.actual[1].event_id, "e2");
        let rpm = parameters
            .iter()
            .find(|row| row.slot == "rpm")
            .expect("unplanned row");
        assert_eq!(rpm.planned, None);
        assert_eq!(rpm.status, ParameterStatus::Unplanned);
    }

    #[test]
    fn planned_parameter_without_actual_stays_missing() {
        let workspace = twelve_well_workspace();
        let parameters = compare_parameters(&workspace);
        assert_eq!(parameters.len(), 1);
        assert_eq!(parameters[0].status, ParameterStatus::MissingActual);
        assert!(parameters[0].actual.is_empty());
    }

    #[test]
    fn timers_report_measured_elapsed_at_the_watermark() {
        let timers = vec![
            TimerRecord {
                timer_id: "t1".to_owned(),
                run_id: "r1".to_owned(),
                task_key: "task-countdown".to_owned(),
                phase_index: 0,
                mode: TimerMode::Countdown,
                planned_s: Some(45),
                started_at_utc: Some("2026-11-15T08:00:00Z".to_owned()),
                accumulated_s: 0,
                status: TimerStatus::Running,
                round_index: 0,
                round_count: Some(3),
                anchor_event_id: None,
                anchor_task_key: None,
            },
            TimerRecord {
                timer_id: "t2".to_owned(),
                run_id: "r1".to_owned(),
                task_key: "task-images".to_owned(),
                phase_index: 0,
                mode: TimerMode::Stopwatch,
                planned_s: None,
                started_at_utc: None,
                accumulated_s: 52,
                status: TimerStatus::Completed,
                round_index: 0,
                round_count: None,
                anchor_event_id: None,
                anchor_task_key: None,
            },
        ];
        let workspace = twelve_well_workspace();
        let projection = build_record_projection(
            &workspace,
            &[],
            &timers,
            &RecordProjectionInput {
                now_utc: Some("2026-11-15T08:00:30Z".to_owned()),
                ..RecordProjectionInput::default()
            },
        );
        assert_eq!(projection.timers[0].elapsed_s, Some(30));
        assert_eq!(projection.timers[0].remaining_s, Some(15));
        assert_eq!(projection.timers[1].elapsed_s, Some(52));

        let no_watermark = build_record_projection(
            &workspace,
            &[],
            &timers,
            &RecordProjectionInput::default(),
        );
        assert_eq!(no_watermark.timers[0].elapsed_s, None);
        assert_eq!(no_watermark.timers[1].elapsed_s, Some(52));
        assert!(no_watermark
            .issues
            .iter()
            .any(|issue| issue.code == "timer_watermark_missing"));
    }

    #[test]
    fn notes_resolve_matrix_and_params_references_and_flag_unknown_ones() {
        let workspace = twelve_well_workspace();
        let links = vec![
            link("a1", Some("A1"), Some("6h"), Some(0)),
            link("a2", Some("A1"), Some("12h"), Some(0)),
        ];
        let markdown = "6h 组形成最好。 @matrix(6h, 12h) 与 @params(curing_s) 见上；@bogus 未知。";
        let projection = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput {
                notes: vec![note(markdown)],
                ..RecordProjectionInput::default()
            },
        );
        let references = &projection.notes[0].references;
        assert_eq!(references.len(), 3);
        assert_eq!(references[0].kind, ReferenceKind::Matrix);
        assert_eq!(references[0].status, ReferenceStatus::Resolved);
        let payload = references[0].payload.as_ref().expect("matrix payload");
        assert_eq!(payload["timepoints"], json!(["6h", "12h"]));
        let a1 = payload["wells"]
            .as_array()
            .expect("wells")
            .iter()
            .find(|well| well["coordinate"] == "A1")
            .expect("A1 row");
        assert_eq!(a1["cells"][0]["filled"], 1);
        assert_eq!(a1["cells"][1]["filled"], 1);
        assert_eq!(a1["cells"][0]["group_label"], json!("3%"));

        assert_eq!(references[1].kind, ReferenceKind::Params);
        assert_eq!(references[1].status, ReferenceStatus::Resolved);
        assert_eq!(references[1].payload.as_ref().unwrap()["rows"][0]["slot"], "curing_s");

        assert_eq!(references[2].kind, ReferenceKind::Unknown);
        assert_eq!(references[2].status, ReferenceStatus::Unresolved);
        assert!(projection
            .issues
            .iter()
            .any(|issue| issue.code == "unresolved_reference"
                && issue.path.as_deref().unwrap_or_default().contains("@bogus")));

        // Blind view must not leak the group label into the matrix payload.
        let blind = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput {
                view: ProjectionView::Blind,
                notes: vec![note("@matrix(6h)")],
                ..RecordProjectionInput::default()
            },
        );
        let cell = &blind.notes[0].references[0].payload.as_ref().unwrap()["wells"][0]["cells"][0];
        assert!(cell.get("group_label").is_none());
    }

    #[test]
    fn unknown_timepoint_reference_is_unresolved_not_guessed() {
        let workspace = twelve_well_workspace();
        let projection = build_record_projection(
            &workspace,
            &[],
            &[],
            &RecordProjectionInput {
                notes: vec![note("@matrix(48h)")],
                ..RecordProjectionInput::default()
            },
        );
        let reference = &projection.notes[0].references[0];
        assert_eq!(reference.status, ReferenceStatus::Unresolved);
        assert_eq!(reference.payload, None);
    }

    #[test]
    fn sign_and_archive_locks_notes_and_fingerprints_every_member() {
        let workspace = twelve_well_workspace();
        let links = vec![link("a1", Some("A1"), Some("6h"), Some(0))];
        let assets = vec![asset_ref("a1", "fov/A1_fov00.png", &checksum("image-bytes"))];
        let notes = vec![note("@matrix(6h)")];
        let projection = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput {
                notes: notes.clone(),
                archive: RecordArchiveState {
                    status: RecordStatus::Summarizing,
                    signature: None,
                    manifest: None,
                },
                ..RecordProjectionInput::default()
            },
        );
        let signature = ArchiveSignature {
            signed_by: "Lin".to_owned(),
            signed_at_utc: "2026-11-15T12:00:00Z".to_owned(),
            statement: Some("数据完整".to_owned()),
        };
        let archived = sign_and_archive(
            &projection.archive,
            &signature,
            &projection,
            &links,
            &assets,
        )
        .expect("sign and archive");

        assert_eq!(archived.status, RecordStatus::Archived);
        let manifest = archived.manifest.as_ref().expect("manifest");
        assert_eq!(manifest.run_id, "r1");
        assert_eq!(manifest.design_version_id, "dv1");
        assert_eq!(manifest.generated_at_utc, "2026-11-15T12:00:00Z");
        assert_eq!(manifest.entries.len(), 3);
        let asset_entry = manifest
            .entries
            .iter()
            .find(|entry| entry.kind == "asset")
            .expect("asset entry");
        assert_eq!(asset_entry.logical_path, "run-r1/fov/A1_fov00.png");
        assert_eq!(asset_entry.sha256, checksum("image-bytes"));
        assert!(manifest
            .entries
            .iter()
            .any(|entry| entry.logical_path == "notes/n1.md"));
        assert!(manifest
            .entries
            .iter()
            .any(|entry| entry.logical_path == "projection.json"));
        verify_archive_manifest(manifest).expect("manifest verifies");

        let mut stored = notes;
        let edit = NoteEdit {
            note_id: "n1".to_owned(),
            title: None,
            markdown: "改主意了".to_owned(),
            updated_at_utc: "2026-11-15T13:00:00Z".to_owned(),
        };
        let error = apply_note_edit(&mut stored, &archived, &edit).expect_err("locked");
        assert_eq!(error.code, "note_locked");
        assert_eq!(stored[0].markdown, "@matrix(6h)");

        let again = sign_and_archive(
            &archived,
            &signature,
            &projection,
            &links,
            &assets,
        )
        .expect_err("already archived");
        assert_eq!(again.code, "already_archived");
    }

    #[test]
    fn archive_requires_summary_state_and_real_checksums() {
        let workspace = twelve_well_workspace();
        let links = vec![link("a1", Some("A1"), Some("6h"), Some(0))];
        let assets = vec![asset_ref("a1", "fov/A1_fov00.png", "")];
        let projection = build_record_projection(
            &workspace,
            &links,
            &[],
            &RecordProjectionInput::default(),
        );
        let signature = ArchiveSignature {
            signed_by: "Lin".to_owned(),
            signed_at_utc: "2026-11-15T12:00:00Z".to_owned(),
            statement: None,
        };
        let early = sign_and_archive(
            &projection.archive,
            &signature,
            &projection,
            &links,
            &assets,
        )
        .expect_err("recording cannot sign");
        assert_eq!(early.code, "invalid_transition");

        let summarizing = RecordArchiveState {
            status: RecordStatus::Summarizing,
            signature: None,
            manifest: None,
        };
        let missing = sign_and_archive(&summarizing, &signature, &projection, &links, &assets)
            .expect_err("missing checksum");
        assert_eq!(missing.code, "asset_checksum_missing");

        let mut unsigned = signature.clone();
        unsigned.signed_by = "  ".to_owned();
        let no_signer =
            sign_and_archive(&summarizing, &unsigned, &projection, &links, &assets)
                .expect_err("missing signer");
        assert_eq!(no_signer.code, "missing_signer");
    }

    #[test]
    fn tampered_manifest_is_rejected() {
        let workspace = twelve_well_workspace();
        let projection = build_record_projection(
            &workspace,
            &[],
            &[],
            &RecordProjectionInput::default(),
        );
        let signature = ArchiveSignature {
            signed_by: "Lin".to_owned(),
            signed_at_utc: "2026-11-15T12:00:00Z".to_owned(),
            statement: None,
        };
        let summarizing = RecordArchiveState {
            status: RecordStatus::Summarizing,
            signature: None,
            manifest: None,
        };
        let archived = sign_and_archive(&summarizing, &signature, &projection, &[], &[])
            .expect("archive without assets");
        let mut manifest = archived.manifest.clone().expect("manifest");
        manifest.entries[0].sha256 = checksum("tampered");
        let error = verify_archive_manifest(&manifest).expect_err("digest mismatch");
        assert_eq!(error.code, "manifest_digest_mismatch");
    }

    #[test]
    fn note_edit_upserts_and_validates_fields() {
        let state = RecordArchiveState::recording();
        let mut notes = Vec::new();
        let edit = NoteEdit {
            note_id: "n1".to_owned(),
            title: Some("结果".to_owned()),
            markdown: "第一版".to_owned(),
            updated_at_utc: "2026-11-15T09:00:00Z".to_owned(),
        };
        apply_note_edit(&mut notes, &state, &edit).expect("insert");
        assert_eq!(notes.len(), 1);
        apply_note_edit(
            &mut notes,
            &state,
            &NoteEdit {
                markdown: "第二版".to_owned(),
                updated_at_utc: "2026-11-15T10:00:00Z".to_owned(),
                ..edit.clone()
            },
        )
        .expect("update");
        assert_eq!(notes.len(), 1);
        assert_eq!(notes[0].markdown, "第二版");
        assert_eq!(notes[0].updated_at_utc, "2026-11-15T10:00:00Z");

        let blank = apply_note_edit(
            &mut notes,
            &state,
            &NoteEdit {
                note_id: " ".to_owned(),
                title: None,
                markdown: String::new(),
                updated_at_utc: "2026-11-15T10:00:00Z".to_owned(),
            },
        )
        .expect_err("blank id");
        assert_eq!(blank.code, "missing_note_id");
        let bad_time = apply_note_edit(
            &mut notes,
            &state,
            &NoteEdit {
                updated_at_utc: "yesterday".to_owned(),
                ..edit
            },
        )
        .expect_err("bad timestamp");
        assert_eq!(bad_time.code, "invalid_timestamp");
    }

    #[test]
    fn projection_serializes_round_trip() {
        let workspace = twelve_well_workspace();
        let projection = build_record_projection(
            &workspace,
            &[],
            &[],
            &RecordProjectionInput {
                notes: vec![note("@timeline")],
                ..RecordProjectionInput::default()
            },
        );
        let json = serde_json::to_string(&projection).expect("serialize");
        let back: RecordProjection = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(back, projection);
        assert_eq!(
            projection.notes[0].references[0].payload.as_ref().unwrap()["events"],
            json!(0)
        );
    }
}
