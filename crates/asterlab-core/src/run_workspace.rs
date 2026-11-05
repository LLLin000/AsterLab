//! Read-model projection for a frozen Run.
//!
//! Steps are definitions, tasks are plans, and occurrences are actual execution
//! instances. This module never reads the current Design.

use crate::design_map::FrozenDesignView;
use crate::protocol_preview::OperationStep;
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
    pub events: Vec<ExecutionEventView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunScope {
    pub kind: String, // assay | planned_output
    pub id: String,
    pub title: String,
    pub protocol_version_id: Option<String>,
    pub step_paths: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct RunStep {
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
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ExecutionEventView {
    pub event_id: String,
    pub event_type: Option<String>,
    pub content: Option<String>,
    pub occurred_at: Option<String>,
    pub stage: Option<String>,
    pub timepoint: Option<String>,
    pub step_status: Option<String>,
    pub protocol_step_path: Option<String>,
    pub protocol_version_id: Option<String>,
    pub assay_id: Option<String>,
    pub planned_output_id: Option<String>,
    pub occurrence_id: Option<String>, // legacy events have None
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

fn parse_steps(entry: &Value) -> Vec<OperationStep> {
    entry["steps"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .filter_map(|step| serde_json::from_value(step.clone()).ok())
                .collect()
        })
        .unwrap_or_default()
}

fn assay_timepoints(design: &crate::design_map::DesignMapContent, assay_id: &str) -> Vec<String> {
    design
        .assays
        .iter()
        .find(|assay| assay.id == assay_id)
        .map(|assay| assay.timepoints.clone())
        .unwrap_or_default()
}

/// Build a frozen Run workspace from stored JSON.
///
/// `design_snapshot` must be the payload of the Run's exact DesignVersion,
/// never the current Design record.
pub fn build(
    run_id: &str,
    design_id: &str,
    design_version_id: &str,
    design_revision: i64,
    design_snapshot: &Value,
    run_data: &Value,
    plate_data: &[Value],
    event_data: &[Value],
) -> Result<RunWorkspace, RunWorkspaceError> {
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

    let mut add_manifest = |entry: &Value, scope: StepScope, title: &str| {
        let protocol_version_id = opt_text(&entry["protocol_version_id"]);
        let step_paths = parse_steps(entry)
            .into_iter()
            .map(|step| {
                order += 1;
                let path = step.path.clone();
                let scope_for_step = scope.clone();
                steps.push(RunStep {
                    scope: scope_for_step,
                    order,
                    step,
                });
                path
            })
            .collect::<Vec<_>>();
        scopes.push(RunScope {
            kind: match scope {
                StepScope::Assay { .. } => "assay".to_owned(),
                StepScope::PlannedOutput { .. } => "planned_output".to_owned(),
                StepScope::Unscoped => "global".to_owned(),
            },
            id: scope_id(&scope),
            title: title.to_owned(),
            protocol_version_id,
            step_paths,
        });
    };

    if let Some(entries) = run_data["protocol_manifest"].as_array() {
        for entry in entries {
            let scope = scope_for_manifest(entry);
            let title = opt_text(&entry["assay_title"])
                .or_else(|| opt_text(&entry["protocol_title"]))
                .unwrap_or_else(|| "Protocol".to_owned());
            add_manifest(entry, scope, &title);
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
                add_manifest(producer, scope, &title);
            }
        }
    }

    // Planned tasks are only derived from explicit step/timepoint pairs.
    // Targets stay Unresolved; no plate/well/sample Cartesian product.
    for step in &steps {
        let timepoints = match &step.scope {
            StepScope::Assay { assay_id } => assay_timepoints(&frozen.content, assay_id),
            _ => Vec::new(),
        };
        if timepoints.is_empty() {
            tasks.push(PlannedTask {
                task_key: format!("{}:{}", scope_id(&step.scope), step.step.path),
                step_path: step.step.path.clone(),
                scope: step.scope.clone(),
                timepoint: None,
                target: TargetSelection::Unresolved,
            });
        } else {
            for timepoint in timepoints {
                tasks.push(PlannedTask {
                    task_key: format!(
                        "{}:{}:{}",
                        scope_id(&step.scope),
                        step.step.path,
                        timepoint
                    ),
                    step_path: step.step.path.clone(),
                    scope: step.scope.clone(),
                    timepoint: Some(timepoint),
                    target: TargetSelection::Unresolved,
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
            assay_id: opt_text(&plate["assay_id"]),
            timepoint: opt_text(&plate["timepoint"]),
        })
        .collect();

    let events = event_data
        .iter()
        .map(|event| ExecutionEventView {
            event_id: text(&event["id"]),
            event_type: opt_text(&event["event_type"]),
            content: opt_text(&event["content"]),
            occurred_at: opt_text(&event["occurred_at"]),
            stage: opt_text(&event["stage"]),
            timepoint: opt_text(&event["timepoint"]),
            step_status: opt_text(&event["step_status"]),
            protocol_step_path: opt_text(&event["protocol_step_path"]),
            protocol_version_id: opt_text(&event["protocol_version_id"]),
            assay_id: opt_text(&event["assay_id"]),
            planned_output_id: opt_text(&event["planned_output_id"]),
            occurrence_id: opt_text(&event["occurrence_id"]),
        })
        .collect();

    // Old events do not have occurrence_id. Do not synthesize an instance.
    let occurrences = Vec::new();

    Ok(RunWorkspace {
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
        events,
    })
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
                     "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1"},
                    {"path":"root/b","use_id":"b","title":"Read","instruction":"read",
                     "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1"}
                ]
            }],
            "material_manifest":[]
        })
    }

    #[test]
    fn tasks_use_explicit_timepoints_without_targets() {
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &[]).unwrap();
        assert_eq!(workspace.steps.len(), 2);
        assert_eq!(workspace.tasks.len(), 4);
        assert!(workspace.tasks.iter().all(|task| task.target == TargetSelection::Unresolved));
        assert!(workspace.tasks.iter().any(|task| task.timepoint.as_deref() == Some("D1")));
        assert!(workspace.occurrences.is_empty());
    }

    #[test]
    fn old_events_remain_legacy_without_occurrence_id() {
        let events = vec![json!({"id":"e1","event_type":"operation","timepoint":"D1",
            "protocol_step_path":"root/a","protocol_version_id":"pv1","assay_id":"cck8"})];
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &events).unwrap();
        assert!(workspace.occurrences.is_empty());
        assert_eq!(workspace.events[0].occurrence_id, None);
    }
}
