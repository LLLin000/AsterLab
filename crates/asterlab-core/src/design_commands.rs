//! Design command slice for creating and revising a versioned experiment design.
//!
//! This module defines the shared command intent, draft shape and deterministic
//! validation. Persistence is an adapter concern; CLI, HTTP and the desktop
//! application must call the same Application Service.

use crate::design_map::FrozenDesignView;
use crate::record_projection::{compare_parameters, ParameterComparison, ParameterStatus};
use crate::run_workspace::RunWorkspace;
use crate::timer::TimerRecord;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::collections::HashSet;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignDraft {
    pub title: String,
    #[serde(default)]
    pub objective: String,
    #[serde(default)]
    pub experimental_unit: Option<String>,
    #[serde(default)]
    pub groups: Vec<DesignGroupDraft>,
    #[serde(default)]
    pub assays: Vec<DesignAssayDraft>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignGroupDraft {
    pub id: String,
    pub name: String,
    #[serde(default)]
    pub role: Option<String>,
    #[serde(default)]
    pub condition: Option<String>,
    #[serde(default)]
    pub independent_n: Option<u32>,
    #[serde(default)]
    pub technical_repeats: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignAssayDraft {
    pub id: String,
    pub title: String,
    #[serde(default)]
    pub endpoint: String,
    #[serde(default)]
    pub method: String,
    #[serde(default)]
    pub unit: String,
    #[serde(default)]
    pub group_ids: Vec<String>,
    #[serde(default)]
    pub timepoints: Vec<String>,
    #[serde(default)]
    pub timepoint_relation: Option<String>,
    #[serde(default)]
    pub protocol_version_id: Option<String>,
    #[serde(default)]
    pub container_type: Option<String>,
    #[serde(default)]
    pub container_format: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CreateDesign {
    pub project_id: String,
    pub draft: DesignDraft,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ReviseDesign {
    pub design_id: String,
    pub expected_revision: i64,
    pub draft: DesignDraft,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignSaved {
    pub design_id: String,
    pub revision: i64,
    pub design_version_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct DesignCommandError {
    pub code: String,
    pub path: Option<String>,
    pub message: String,
}

impl DesignCommandError {
    fn invalid(path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            code: "INVALID_DESIGN".to_owned(),
            path: Some(path.into()),
            message: message.into(),
        }
    }
}

pub trait DesignCommands {
    fn create_design(&self, command: CreateDesign) -> Result<DesignSaved, DesignCommandError>;
    fn revise_design(&self, command: ReviseDesign) -> Result<DesignSaved, DesignCommandError>;
}

/// Validate only structural/scientific-reference constraints. Missing n,
/// sample relations, doses or timepoints remain valid draft unknowns.
impl DesignDraft {
    /// Map the editor draft to the current CLI/HTTP design payload.
    /// Unknown fields are intentionally not invented.
    pub fn to_design_json(&self) -> serde_json::Value {
        let group_names = self
            .groups
            .iter()
            .map(|group| (group.id.as_str(), group.name.as_str()))
            .collect::<std::collections::HashMap<_, _>>();
        serde_json::json!({
            "title": self.title,
            "objective": self.objective,
            "experimental_unit": self.experimental_unit,
            "group_specs": self.groups.iter().map(|group| serde_json::json!({
                "id": group.id,
                "name": group.name,
                "role": group.role,
                "condition": group.condition,
                "independent_n": group.independent_n,
                "technical_repeats": group.technical_repeats
            })).collect::<Vec<_>>(),
            "assays": self.assays.iter().map(|assay| serde_json::json!({
                "id": assay.id,
                "title": assay.title,
                "endpoint": assay.endpoint,
                "method": assay.method,
                "unit": assay.unit,
                "group_ids": assay.group_ids,
                "group_names": assay.group_ids.iter().filter_map(|id| group_names.get(id.as_str()).copied()).collect::<Vec<_>>(),
                "timepoints": assay.timepoints,
                "timepoint_relation": assay.timepoint_relation,
                "protocol_version_id": assay.protocol_version_id,
                "container_type": assay.container_type,
                "container_format": assay.container_format
            })).collect::<Vec<_>>()
        })
    }

    pub fn from_design_json(data: &serde_json::Value) -> Self {
        let groups = data["group_specs"]
            .as_array()
            .map(|items| {
                items
                    .iter()
                    .map(|group| DesignGroupDraft {
                        id: group["id"].as_str().unwrap_or_default().to_owned(),
                        name: group["name"].as_str().unwrap_or_default().to_owned(),
                        role: group["role"].as_str().map(str::to_owned),
                        condition: group["condition"].as_str().map(str::to_owned),
                        independent_n: group["independent_n"]
                            .as_u64()
                            .and_then(|value| u32::try_from(value).ok()),
                        technical_repeats: group["technical_repeats"]
                            .as_u64()
                            .and_then(|value| u32::try_from(value).ok()),
                    })
                    .collect()
            })
            .unwrap_or_default();
        let assays = data["assays"]
            .as_array()
            .map(|items| {
                items
                    .iter()
                    .map(|assay| DesignAssayDraft {
                        id: assay["id"].as_str().unwrap_or_default().to_owned(),
                        title: assay["title"].as_str().unwrap_or_default().to_owned(),
                        endpoint: assay["endpoint"].as_str().unwrap_or_default().to_owned(),
                        method: assay["method"].as_str().unwrap_or_default().to_owned(),
                        unit: assay["unit"].as_str().unwrap_or_default().to_owned(),
                        group_ids: assay["group_ids"]
                            .as_array()
                            .map(|ids| ids.iter().filter_map(|id| id.as_str().map(str::to_owned)).collect())
                            .unwrap_or_default(),
                        timepoints: assay["timepoints"]
                            .as_array()
                            .map(|points| points.iter().filter_map(|point| point.as_str().map(str::to_owned)).collect())
                            .unwrap_or_default(),
                        timepoint_relation: assay["timepoint_relation"].as_str().map(str::to_owned),
                        protocol_version_id: assay["protocol_version_id"].as_str().map(str::to_owned),
                        container_type: assay["container_type"].as_str().map(str::to_owned),
                        container_format: assay["container_format"].as_str().map(str::to_owned),
                    })
                    .collect()
            })
            .unwrap_or_default();
        Self {
            title: data["title"].as_str().unwrap_or_default().to_owned(),
            objective: data["objective"].as_str().unwrap_or_default().to_owned(),
            experimental_unit: data["experimental_unit"].as_str().map(str::to_owned),
            groups,
            assays,
        }
    }
}

pub fn validate_draft(draft: &DesignDraft) -> Result<(), DesignCommandError> {
    if draft.title.trim().is_empty() {
        return Err(DesignCommandError::invalid("title", "设计名称不能为空"));
    }
    if draft.title.chars().count() > 160 {
        return Err(DesignCommandError::invalid("title", "设计名称不能超过 160 个字符"));
    }
    if draft.objective.chars().count() > 5000 {
        return Err(DesignCommandError::invalid("objective", "研究问题不能超过 5000 个字符"));
    }

    let mut group_ids = HashSet::new();
    for (index, group) in draft.groups.iter().enumerate() {
        if group.id.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("groups[{}].id", index),
                "组别必须有稳定 ID",
            ));
        }
        if !group_ids.insert(group.id.as_str()) {
            return Err(DesignCommandError::invalid(
                format!("groups[{}].id", index),
                "组别 ID 重复",
            ));
        }
        if group.name.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("groups[{}].name", index),
                "组别名称不能为空",
            ));
        }
    }

    let mut assay_ids = HashSet::new();
    for (index, assay) in draft.assays.iter().enumerate() {
        if assay.id.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("assays[{}].id", index),
                "检测必须有稳定 ID",
            ));
        }
        if !assay_ids.insert(assay.id.as_str()) {
            return Err(DesignCommandError::invalid(
                format!("assays[{}].id", index),
                "检测 ID 重复",
            ));
        }
        if assay.title.trim().is_empty() {
            return Err(DesignCommandError::invalid(
                format!("assays[{}].title", index),
                "检测名称不能为空",
            ));
        }
        for group_id in &assay.group_ids {
            if !group_ids.contains(group_id.as_str()) {
                return Err(DesignCommandError::invalid(
                    format!("assays[{}].group_ids", index),
                    format!("引用了未定义的组别 ID：{group_id}"),
                ));
            }
        }
        let mut timepoints = HashSet::new();
        for timepoint in &assay.timepoints {
            if timepoint.trim().is_empty() {
                return Err(DesignCommandError::invalid(
                    format!("assays[{}].timepoints", index),
                    "时间点不能为空",
                ));
            }
            if !timepoints.insert(timepoint.as_str()) {
                return Err(DesignCommandError::invalid(
                    format!("assays[{}].timepoints", index),
                    format!("时间点重复：{timepoint}"),
                ));
            }
        }
    }
    Ok(())
}

impl DesignDraft {
    /// Rebuild the editable draft from the frozen DesignVersion a Run points at.
    ///
    /// Fields the frozen map does not carry (`group_names`, `controls`,
    /// `timepoint_timing`) are not invented; the adapter keeps them in the
    /// stored design JSON.
    pub fn from_frozen_design(frozen: &FrozenDesignView) -> Self {
        Self {
            title: frozen.content.title.clone(),
            objective: frozen.content.objective.clone(),
            experimental_unit: frozen.content.experimental_unit.clone(),
            groups: frozen
                .content
                .groups
                .iter()
                .map(|group| DesignGroupDraft {
                    id: group.id.clone(),
                    name: group.name.clone(),
                    role: group.role.clone(),
                    condition: group.condition.clone(),
                    independent_n: group.independent_n,
                    technical_repeats: group.technical_repeats,
                })
                .collect(),
            assays: frozen
                .content
                .assays
                .iter()
                .map(|assay| DesignAssayDraft {
                    id: assay.id.clone(),
                    title: assay.title.clone(),
                    endpoint: assay.endpoint.clone(),
                    method: assay.method.clone(),
                    unit: assay.unit.clone(),
                    group_ids: assay.group_ids.clone(),
                    timepoints: assay.timepoints.clone(),
                    timepoint_relation: assay.timepoint_relation.clone(),
                    protocol_version_id: assay.protocol_version_id.clone(),
                    container_type: assay.container_type.clone(),
                    container_format: assay.container_format.clone(),
                })
                .collect(),
        }
    }
}

/// Request the next Design revision derived from one finished Run.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DeriveDesignCommand {
    pub run_id: String,
    /// Optional replacement title; the frozen title is reused when absent.
    #[serde(default)]
    pub title: Option<String>,
    /// Optional replacement research question.
    #[serde(default)]
    pub objective: Option<String>,
    pub note: Option<String>,
}

/// A measured parameter that should become the next version's binding.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DerivedParameterUpdate {
    pub scope_id: String,
    pub step_path: Option<String>,
    pub parameter_key: String,
    pub label: Option<String>,
    pub unit: Option<String>,
    pub planned: Option<Value>,
    pub actual: Value,
    pub event_id: String,
}

/// A measured duration that should become the next version's time budget input.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DerivedDurationUpdate {
    pub timer_id: String,
    pub task_key: String,
    pub phase_index: u32,
    pub planned_s: Option<u64>,
    pub actual_s: u64,
}

/// Next Design version payload derived from one Run.
///
/// The frozen Run and its DesignVersion are only read: the caller persists this
/// as a new revision through [`ReviseDesign`].
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct DerivedDesign {
    pub design_id: String,
    pub source_design_version_id: String,
    pub source_design_revision: i64,
    /// Revision the adapter must pass as `ReviseDesign.expected_revision`.
    pub expected_revision: i64,
    pub source_run_id: String,
    pub draft: DesignDraft,
    pub parameter_updates: Vec<DerivedParameterUpdate>,
    pub duration_updates: Vec<DerivedDurationUpdate>,
    pub note: Option<String>,
}

impl DerivedDesign {
    pub fn to_design_json(&self) -> Value {
        let mut data = self.draft.to_design_json();
        data["derived_from"] = json!({
            "design_version_id": self.source_design_version_id,
            "design_revision": self.source_design_revision,
            "run_id": self.source_run_id,
            "parameter_updates": self.parameter_updates.iter().map(|update| json!({
                "scope_id": update.scope_id,
                "step_path": update.step_path,
                "parameter_key": update.parameter_key,
                "label": update.label,
                "unit": update.unit,
                "planned": update.planned,
                "actual": update.actual,
                "event_id": update.event_id,
            })).collect::<Vec<_>>(),
            "duration_updates": self.duration_updates.iter().map(|update| json!({
                "timer_id": update.timer_id,
                "task_key": update.task_key,
                "phase_index": update.phase_index,
                "planned_s": update.planned_s,
                "actual_s": update.actual_s,
            })).collect::<Vec<_>>(),
            "note": self.note,
        });
        data
    }
}

/// Derive the next Design version from a Run's measured parameters and timers.
///
/// Pure: the frozen Run workspace is only read, and nothing is persisted here.
/// Only measured drift enters `parameter_updates`; planned-but-unrecorded slots
/// and matching values stay out so the next version does not lose unknowns.
pub fn derive_design_from_run(
    workspace: &RunWorkspace,
    timers: &[TimerRecord],
    command: &DeriveDesignCommand,
) -> Result<DerivedDesign, DesignCommandError> {
    if command.run_id.trim() != workspace.run_id {
        return Err(DesignCommandError {
            code: "WRONG_RUN".to_owned(),
            path: Some("run_id".to_owned()),
            message: format!(
                "Run {} 与冻结工作区 {} 不一致",
                command.run_id, workspace.run_id
            ),
        });
    }

    let frozen = &workspace.frozen_design;
    let mut draft = DesignDraft::from_frozen_design(frozen);
    if let Some(title) = command.title.as_deref() {
        let title = title.trim();
        if title.is_empty() {
            return Err(DesignCommandError::invalid("title", "设计名称不能为空"));
        }
        draft.title = title.to_owned();
    }
    if let Some(objective) = command.objective.as_deref() {
        draft.objective = objective.to_owned();
    }
    validate_draft(&draft).map_err(|error| DesignCommandError {
        code: "INVALID_DERIVED_DESIGN".to_owned(),
        path: error.path,
        message: error.message,
    })?;

    let parameter_updates = derived_parameter_updates(&compare_parameters(workspace));
    let duration_updates = timers
        .iter()
        .filter(|timer| timer.accumulated_s > 0)
        .filter(|timer| timer.status != crate::timer::TimerStatus::Running)
        .map(|timer| DerivedDurationUpdate {
            timer_id: timer.timer_id.clone(),
            task_key: timer.task_key.clone(),
            phase_index: timer.phase_index,
            planned_s: timer.planned_s,
            actual_s: timer.accumulated_s,
        })
        .collect();

    Ok(DerivedDesign {
        design_id: frozen.design_id.clone(),
        source_design_version_id: frozen.design_version_id.clone(),
        source_design_revision: frozen.design_revision,
        expected_revision: frozen.design_revision,
        source_run_id: workspace.run_id.clone(),
        draft,
        parameter_updates,
        duration_updates,
        note: command.note.clone(),
    })
}

fn derived_parameter_updates(comparisons: &[ParameterComparison]) -> Vec<DerivedParameterUpdate> {
    let mut updates = Vec::new();
    for row in comparisons {
        if matches!(row.status, ParameterStatus::Matches | ParameterStatus::MissingActual) {
            continue;
        }
        // `actual` is chronological; the last recording is the Run's final value.
        let Some(actual) = row.actual.last() else {
            continue;
        };
        updates.push(DerivedParameterUpdate {
            scope_id: row.scope_id.clone(),
            step_path: row.step_path.clone(),
            parameter_key: row.slot.clone(),
            label: row.label.clone(),
            unit: actual.unit.clone().or_else(|| row.unit.clone()),
            planned: row.planned.clone(),
            actual: actual.value.clone(),
            event_id: actual.event_id.clone(),
        });
    }
    updates
}

#[cfg(test)]
mod tests {
    use super::*;

    fn blank() -> DesignDraft {
        DesignDraft {
            title: "New design".into(),
            objective: "Compare conditions".into(),
            experimental_unit: None,
            groups: vec![],
            assays: vec![],
        }
    }

    #[test]
    fn empty_groups_and_assays_are_valid_draft_unknowns() {
        assert!(validate_draft(&blank()).is_ok());
    }

    #[test]
    fn draft_round_trips_through_design_json() {
        let mut draft = blank();
        draft.groups = vec![DesignGroupDraft {
            id: "g1".into(), name: "Control".into(), role: Some("control".into()),
            condition: None, independent_n: None, technical_repeats: Some(2),
        }];
        draft.assays = vec![DesignAssayDraft {
            id: "a1".into(), title: "CCK-8".into(), endpoint: "OD450".into(),
            method: "CCK-8".into(), unit: "absorbance".into(),
            group_ids: vec!["g1".into()], timepoints: vec!["D1".into()],
            timepoint_relation: None, protocol_version_id: None,
            container_type: None, container_format: None,
        }];
        let json = draft.to_design_json();
        assert_eq!(json["assays"][0]["group_names"][0], "Control");
        assert_eq!(DesignDraft::from_design_json(&json), draft);
    }

    #[test]
    fn duplicate_group_ids_are_rejected() {
        let mut draft = blank();
        draft.groups = vec![
            DesignGroupDraft { id: "g1".into(), name: "A".into(), role: None, condition: None, independent_n: None, technical_repeats: None },
            DesignGroupDraft { id: "g1".into(), name: "B".into(), role: None, condition: None, independent_n: None, technical_repeats: None },
        ];
        assert!(validate_draft(&draft).is_err());
    }

    #[test]
    fn assay_cannot_reference_unknown_group() {
        let mut draft = blank();
        draft.assays = vec![DesignAssayDraft {
            id: "a1".into(),
            title: "CCK-8".into(),
            endpoint: "OD450".into(),
            method: "CCK-8".into(),
            unit: "absorbance".into(),
            group_ids: vec!["missing".into()],
            timepoints: vec!["D1".into()],
            timepoint_relation: None,
            protocol_version_id: None,
            container_type: None,
            container_format: None,
        }];
        assert!(validate_draft(&draft).is_err());
    }

    #[test]
    fn duplicate_timepoints_are_rejected() {
        let mut draft = blank();
        draft.groups = vec![DesignGroupDraft { id: "g1".into(), name: "Control".into(), role: None, condition: None, independent_n: None, technical_repeats: None }];
        draft.assays = vec![DesignAssayDraft {
            id: "a1".into(), title: "CCK-8".into(), endpoint: "".into(), method: "".into(), unit: "".into(),
            group_ids: vec!["g1".into()], timepoints: vec!["D1".into(), "D1".into()],
            timepoint_relation: None, protocol_version_id: None, container_type: None, container_format: None,
        }];
        assert!(validate_draft(&draft).is_err());
    }

    fn derive_design() -> Value {
        serde_json::json!({
            "title":"HUVEC 成管",
            "objective":"比较 3%/5%/7% GelMA",
            "experimental_unit":"well",
            "group_specs":[
                {"id":"g3","name":"3%","role":"treatment","condition":"3% GelMA","technical_repeats":6},
                {"id":"g5","name":"5%","role":"treatment","condition":"5% GelMA"}
            ],
            "assays":[{
                "id":"tube","title":"Tube formation","endpoint":"tube_length",
                "method":"microscopy","unit":"um","group_ids":["g3","g5"],
                "timepoints":["6h","12h","24h"],
                "timepoint_relation":"same_physical_sample_repeated",
                "protocol_version_id":"pv1"
            }],
            "sample_allocations":[],"planned_outputs":[],"evidence_conflicts":[]
        })
    }

    fn derive_run() -> Value {
        serde_json::json!({
            "status":"completed",
            "protocol_manifest":[{
                "assay_id":"tube","assay_title":"Tube formation","protocol_version_id":"pv1",
                "resolved_parameters":[
                    {"slot":"curing_s","label":"405nm 固化秒数","value":45,"unit":"s","provenance":[]},
                    {"slot":"seeding_density","label":"接种密度","value":10000,"unit":"cells/well","provenance":[]}
                ],
                "unresolved_parameters":[],
                "steps":[
                    {"path":"root/seed","use_id":"seed","title":"Seed","instruction":"seed",
                     "parameters":[],"unresolved_parameter_refs":[],
                     "source_block_version_id":"b1","time_scope":"once"}
                ]
            }],
            "material_manifest":[]
        })
    }

    fn derive_workspace(events: Vec<Value>) -> RunWorkspace {
        crate::run_workspace::build(
            "r1", "d1", "dv1", 4, &derive_design(), &derive_run(), &[], &events, &[],
        )
        .expect("workspace")
    }

    fn completed_timer(accumulated_s: u64) -> TimerRecord {
        TimerRecord {
            timer_id: "t1".into(),
            run_id: "r1".into(),
            task_key: "task-seed".into(),
            phase_index: 0,
            mode: crate::timer::TimerMode::Countdown,
            planned_s: Some(45),
            started_at_utc: None,
            accumulated_s,
            status: crate::timer::TimerStatus::Completed,
            round_index: 0,
            round_count: None,
            anchor_event_id: None,
            anchor_task_key: None,
        }
    }

    #[test]
    fn derive_design_copies_frozen_draft_and_measured_drift() {
        let events = vec![serde_json::json!({
            "id":"e1","event_type":"measurement","protocol_step_path":"root/seed",
            "assay_id":"tube","recorded_at":"2026-11-15T08:00:00Z",
            "actual_parameters":[
                {"key":"curing_s","value":52,"unit":"s"},
                {"key":"seeding_density","value":10000,"unit":"cells/well"}
            ]
        })];
        let workspace = derive_workspace(events);
        let before = serde_json::to_string(&workspace).expect("snapshot");

        let derived = derive_design_from_run(
            &workspace,
            &[completed_timer(52)],
            &DeriveDesignCommand {
                run_id: "r1".into(),
                title: Some("HUVEC 成管 v2".into()),
                objective: Some("按实测固化时间比较".into()),
                note: Some("固化实测 52s".into()),
            },
        )
        .expect("derive");

        assert_eq!(derived.design_id, "d1");
        assert_eq!(derived.source_design_version_id, "dv1");
        assert_eq!(derived.source_design_revision, 4);
        assert_eq!(derived.expected_revision, 4);
        assert_eq!(derived.source_run_id, "r1");
        assert_eq!(derived.draft.title, "HUVEC 成管 v2");
        assert_eq!(derived.draft.objective, "按实测固化时间比较");
        assert_eq!(derived.draft.experimental_unit.as_deref(), Some("well"));
        assert_eq!(derived.draft.groups.len(), 2);
        assert_eq!(derived.draft.groups[0].technical_repeats, Some(6));
        assert_eq!(derived.draft.assays[0].group_ids, vec!["g3", "g5"]);
        assert_eq!(derived.draft.assays[0].timepoints, vec!["6h", "12h", "24h"]);

        // Only the drifted slot becomes an update; the matching one is dropped.
        assert_eq!(derived.parameter_updates.len(), 1);
        let update = &derived.parameter_updates[0];
        assert_eq!(update.parameter_key, "curing_s");
        assert_eq!(update.planned, Some(serde_json::json!(45)));
        assert_eq!(update.actual, serde_json::json!(52));
        assert_eq!(update.event_id, "e1");
        assert_eq!(derived.duration_updates.len(), 1);
        assert_eq!(derived.duration_updates[0].actual_s, 52);
        assert_eq!(derived.duration_updates[0].planned_s, Some(45));

        // The frozen Run is untouched by the derivation.
        assert_eq!(
            serde_json::to_string(&workspace).expect("snapshot"),
            before
        );
    }

    #[test]
    fn derive_design_rejects_a_foreign_run_id() {
        let workspace = derive_workspace(Vec::new());
        let error = derive_design_from_run(
            &workspace,
            &[],
            &DeriveDesignCommand {
                run_id: "other".into(),
                title: None,
                objective: None,
                note: None,
            },
        )
        .expect_err("wrong run");
        assert_eq!(error.code, "WRONG_RUN");
        assert_eq!(error.path.as_deref(), Some("run_id"));
    }

    #[test]
    fn derived_design_json_round_trips_the_draft() {
        let workspace = derive_workspace(Vec::new());
        let derived = derive_design_from_run(
            &workspace,
            &[],
            &DeriveDesignCommand {
                run_id: "r1".into(),
                title: None,
                objective: None,
                note: None,
            },
        )
        .expect("derive");
        let data = derived.to_design_json();
        assert_eq!(DesignDraft::from_design_json(&data), derived.draft);
        assert_eq!(data["derived_from"]["run_id"], "r1");
        assert_eq!(data["derived_from"]["design_version_id"], "dv1");
        assert!(derived.parameter_updates.is_empty());
        assert!(derived.duration_updates.is_empty());
    }
}
