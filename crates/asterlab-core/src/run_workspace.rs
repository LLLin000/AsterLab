//! Read-model projection for a frozen Run.
//!
//! Steps are definitions, tasks are plans, and occurrences are actual execution
//! instances. This module never reads the current Design.

use crate::design_map::FrozenDesignView;
use crate::protocol_preview::{OperationStep, ResolvedParameter, StepTimeScope, UnresolvedParameter};
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
}

fn default_task_status() -> String {
    "not_started".to_owned()
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
                tasks.push(PlannedTask {
                    task_key: key,
                    step_path: step.step.path.clone(),
                    scope: step.scope.clone(),
                    timepoint: Some(timepoint),
                    target: TargetSelection::Unresolved,
                    status: default_task_status(),
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

    let events = event_data
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
        })
        .collect();

    // Old events do not have occurrence_id. Do not synthesize an instance.
    let occurrences = Vec::new();

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
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run(), &[], &[]).unwrap();
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
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run, &[], &[]).unwrap();
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
        let workspace = build("r1", "d1", "dv1", 1, &design(), &run, &[], &[]).unwrap();
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
        assert!(build("r1", "d1", "dv1", 1, &design(), &run, &[], &[]).is_err());
    }

    #[test]
    fn frozen_design_carries_sdk_modules() {
        let mut design = design();
        design["asterlab"] = json!({"foundationVersion":1,"sdkVersion":"1.0.0",
            "modules":[{"kind":"dilution","id":"d1","schema":1}]});
        let workspace = build("r1", "d1", "dv1", 1, &design, &run(), &[], &[]).unwrap();
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
        let workspace = build("r1", "d1", "dv1", 1, &design, &run, &[], &[]).unwrap();
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
        let error = build("r1", "d1", "dv1", 1, &design(), &run, &[], &[]).unwrap_err();
        assert!(error.to_string().contains("step[0]"), "{error}");
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
