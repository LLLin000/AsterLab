//! Execution event vocabulary and the Run/step timeline.
//!
//! Events are append-only records under a frozen Run. This module owns three
//! things and no store:
//!
//! - the explicit event vocabulary (`intent`, `contribution`, `outcome`);
//! - a write plan that validates an [`EventDraft`] against the frozen Run and
//!   returns the exact `execution_event.create` payload;
//! - a query that classifies stored events into a chronological Run/step
//!   timeline, including failed and retried records.
//!
//! A retry is recorded through the existing `lineage` array, so the stored
//! event schema does not need a new field:
//!
//! ```json
//! {"from_kind":"execution_event","from_id":"<retried event>",
//!  "to_kind":"step","to_id":"<step path>","relation":"retry"}
//! ```
//!
//! A stored `retry_of_event_id` is also honoured when an adapter writes it
//! directly. The query never guesses a retry from "the latest event of the
//! same step"; the link must be explicit.

use crate::run_workspace::{ExecutionEventView, RunScope, RunWorkspace, StepScope};
use serde::{Deserialize, Serialize};
use serde_json::{json, Map, Value};
use std::collections::{HashMap, HashSet};

/// Schema version of the timeline read model. Bump only for a breaking field
/// change.
pub const EXECUTION_TIMELINE_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EventIntent {
    Operation,
    MaterialUse,
    Measurement,
    Check,
    Completion,
    Failure,
    Observation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EventContribution {
    Operation,
    MaterialUse,
    Measurement,
    Check,
    Completion,
    Deviation,
    Observation,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum EventOutcome {
    Recorded,
    Succeeded,
    Failed,
    Blocked,
    Skipped,
}

impl EventOutcome {
    pub fn is_failure(self) -> bool {
        matches!(self, EventOutcome::Failed | EventOutcome::Blocked)
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EventDraft {
    pub run_id: String,
    pub intent: EventIntent,
    pub content: String,
    #[serde(default)]
    pub step_path: Option<String>,
    #[serde(default)]
    pub protocol_version_id: Option<String>,
    #[serde(default)]
    pub assay_id: Option<String>,
    #[serde(default)]
    pub planned_output_id: Option<String>,
    #[serde(default)]
    pub timepoint: Option<String>,
    #[serde(default)]
    pub task_key: Option<String>,
    #[serde(default)]
    pub operator: Option<String>,
    #[serde(default)]
    pub occurred_at: Option<String>,
    #[serde(default)]
    pub step_status: Option<String>,
    #[serde(default)]
    pub sample_ids: Vec<String>,
    #[serde(default)]
    pub checks: Vec<CheckInput>,
    #[serde(default)]
    pub measurements: Vec<MeasurementInput>,
    #[serde(default)]
    pub materials: Vec<MaterialInput>,
    #[serde(default)]
    pub deviation: Option<String>,
    #[serde(default)]
    pub retry_of_event_id: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CheckInput {
    pub check_id: String,
    pub status: String,
    #[serde(default)]
    pub note: Option<String>,
    #[serde(default)]
    pub checked_by: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MeasurementInput {
    pub endpoint: String,
    pub value: f64,
    #[serde(default)]
    pub unit: Option<String>,
    #[serde(default)]
    pub sample_id: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MaterialInput {
    pub material: String,
    #[serde(default)]
    pub planned_output_id: Option<String>,
    #[serde(default)]
    pub amount: Option<f64>,
    #[serde(default)]
    pub unit: Option<String>,
}

/// Payload for one `execution_event.create` call.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EventAppendPlan {
    pub payload: Value,
    /// Set when this event retries another one.
    pub retry_of_event_id: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CheckView {
    pub check_id: String,
    pub status: String,
    pub note: Option<String>,
    pub checked_by: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TimelineEvent {
    pub event_id: String,
    pub event_type: Option<String>,
    pub contributions: Vec<EventContribution>,
    pub outcome: EventOutcome,
    pub content: Option<String>,
    pub step_path: Option<String>,
    pub assay_id: Option<String>,
    pub planned_output_id: Option<String>,
    pub timepoint: Option<String>,
    pub task_key: Option<String>,
    pub occurrence_id: Option<String>,
    pub operator: Option<String>,
    pub occurred_at: Option<String>,
    pub recorded_at: Option<String>,
    pub checks: Vec<CheckView>,
    pub measurements: Vec<Value>,
    pub materials: Vec<Value>,
    pub deviation: Option<String>,
    pub retry_of_event_id: Option<String>,
    /// 1 for a first attempt; incremented across explicit retry links.
    pub attempt: u32,
}

impl TimelineEvent {
    pub fn is_failure(&self) -> bool {
        self.outcome.is_failure()
    }

    pub fn has_contribution(&self, contribution: EventContribution) -> bool {
        self.contributions.contains(&contribution)
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RetryRecord {
    pub event_id: String,
    pub retry_of_event_id: String,
    pub step_path: Option<String>,
    pub task_key: Option<String>,
    pub attempt: u32,
    pub previous_outcome: EventOutcome,
    pub recorded_at: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TimelineIssue {
    pub code: String,
    pub message: String,
    pub event_id: Option<String>,
    pub step_path: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default, deny_unknown_fields)]
pub struct TimelineQuery {
    pub step_path: Option<String>,
    pub task_key: Option<String>,
    pub assay_id: Option<String>,
    pub occurrence_id: Option<String>,
    /// Empty means every contribution.
    pub kinds: Vec<EventContribution>,
    pub failures_only: bool,
    pub retries_only: bool,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TimelineQueryResult {
    pub schema_version: u32,
    pub run_id: String,
    /// Chronological by recorded time, not caller array order.
    pub events: Vec<TimelineEvent>,
    /// Failed and blocked records from `events`.
    pub failures: Vec<TimelineEvent>,
    /// Explicit retry links whose target exists and failed.
    pub retries: Vec<RetryRecord>,
    pub issues: Vec<TimelineIssue>,
}

/// Validate a draft against the frozen Run and return the storage payload.
pub fn plan_event(
    draft: &EventDraft,
    workspace: &RunWorkspace,
) -> Result<EventAppendPlan, Vec<TimelineIssue>> {
    let mut issues = Vec::new();
    let run_id = draft.run_id.trim();
    if run_id.is_empty() {
        issues.push(issue("missing_run", "事件缺少 run_id", None, None));
    } else if run_id != workspace.run_id {
        issues.push(issue(
            "wrong_run",
            "事件 run_id 与 workspace 不一致",
            None,
            None,
        ));
    }

    let content = draft.content.trim();
    if content.is_empty() {
        issues.push(issue("missing_content", "事件内容不能为空", None, None));
    } else if content.chars().count() > 5000 {
        issues.push(issue(
            "content_too_long",
            "事件内容不能超过 5000 个字符",
            None,
            None,
        ));
    }

    if let Some(at) = draft.occurred_at.as_deref() {
        if !at.trim().is_empty() && !is_rfc3339(at.trim()) {
            issues.push(issue(
                "invalid_occurred_at",
                "occurred_at 必须是带时区的 RFC 3339 时间；不确定时留空",
                None,
                None,
            ));
        }
    }

    let step_path = trim_opt(&draft.step_path);
    let protocol_version_id = trim_opt(&draft.protocol_version_id);
    let assay_id = trim_opt(&draft.assay_id);
    let planned_output_id = trim_opt(&draft.planned_output_id);
    let timepoint = trim_opt(&draft.timepoint);
    let task_key = trim_opt(&draft.task_key);
    let operator = trim_opt(&draft.operator);
    let retry_of_event_id = trim_opt(&draft.retry_of_event_id);
    let deviation = trim_opt(&draft.deviation);

    let mut step_status = trim_opt(&draft.step_status);
    if let Some(status) = step_status.as_deref() {
        if !["in_progress", "completed", "blocked", "skipped"].contains(&status) {
            issues.push(issue(
                "invalid_step_status",
                "step_status 必须是 in_progress、completed、blocked 或 skipped",
                None,
                step_path.as_deref(),
            ));
            step_status = None;
        }
    }

    if let Some(operator) = operator.as_deref() {
        if operator.chars().count() > 160 {
            issues.push(issue(
                "invalid_operator",
                "记录人不能超过 160 个字符",
                None,
                None,
            ));
        }
    }

    if step_path.is_none() {
        let has_step_fields = protocol_version_id.is_some()
            || assay_id.is_some()
            || planned_output_id.is_some()
            || timepoint.is_some()
            || task_key.is_some()
            || step_status.is_some()
            || !draft.checks.is_empty();
        if has_step_fields {
            issues.push(issue(
                "field_requires_step",
                "未关联冻结 Protocol 步骤时不能提供步骤字段、检查或任务键",
                None,
                None,
            ));
        }
        if retry_of_event_id.is_some() {
            issues.push(issue(
                "retry_requires_step",
                "重试必须关联到被重试事件所在的 Protocol 步骤",
                None,
                None,
            ));
        }
    }

    if assay_id.is_some() && planned_output_id.is_some() {
        issues.push(issue(
            "ambiguous_scope",
            "步骤事件只能关联一个检测或一个共享计划产物",
            None,
            step_path.as_deref(),
        ));
    }

    let mut resolved_step: Option<&crate::run_workspace::RunStep> = None;
    if let Some(path) = step_path.as_deref() {
        let matching: Vec<&crate::run_workspace::RunStep> = workspace
            .steps
            .iter()
            .filter(|step| {
                step.step.path == path
                    && scope_matches(
                        &step.scope,
                        assay_id.as_deref(),
                        planned_output_id.as_deref(),
                    )
            })
            .collect();
        match matching.len() {
            0 => issues.push(issue(
                "unknown_step",
                &format!("步骤 {path} 不属于本次 Run 的冻结 Protocol 或作用域"),
                None,
                Some(path),
            )),
            1 => resolved_step = Some(matching[0]),
            _ => issues.push(issue(
                "ambiguous_step",
                &format!("步骤 {path} 在多个作用域中出现；必须给出 assay_id 或 planned_output_id"),
                None,
                Some(path),
            )),
        }

        match protocol_version_id.as_deref() {
            None => issues.push(issue(
                "missing_protocol_version",
                "关联 Protocol 步骤时必须提供 protocol_version_id",
                None,
                Some(path),
            )),
            Some(version) => {
                if let Some(step) = resolved_step {
                    let expected = workspace
                        .scopes
                        .iter()
                        .find(|scope| run_scope_matches(&step.scope, scope))
                        .and_then(|scope| scope.protocol_version_id.as_deref());
                    if expected != Some(version) {
                        issues.push(issue(
                            "unknown_protocol_version",
                            "protocol_version_id 与本次 Run 冻结的步骤作用域不一致",
                            None,
                            Some(path),
                        ));
                    }
                }
            }
        }

        if let Some(timepoint) = timepoint.as_deref() {
            if assay_id.is_none() {
                issues.push(issue(
                    "timepoint_without_assay",
                    "时间点必须同时给出检测；Core 不按名称猜检测",
                    None,
                    Some(path),
                ));
            } else if !workspace.tasks.iter().any(|task| {
                task.step_path == path
                    && task.timepoint.as_deref() == Some(timepoint)
                    && scope_matches(
                        &task.scope,
                        assay_id.as_deref(),
                        planned_output_id.as_deref(),
                    )
            }) {
                issues.push(issue(
                    "unknown_timepoint",
                    &format!("时间点 {timepoint} 不是该步骤本次 Run 的计划时间点"),
                    None,
                    Some(path),
                ));
            }
        }

        if let Some(key) = task_key.as_deref() {
            match workspace.tasks.iter().find(|task| task.task_key == key) {
                None => issues.push(issue(
                    "unknown_task",
                    &format!("任务 {key} 不属于本次 Run"),
                    None,
                    Some(path),
                )),
                Some(task) => {
                    let scope_ok = scope_matches(
                        &task.scope,
                        assay_id.as_deref(),
                        planned_output_id.as_deref(),
                    );
                    if task.step_path != path
                        || task.timepoint != timepoint
                        || !scope_ok
                    {
                        issues.push(issue(
                            "task_mismatch",
                            "task_key 与事件的步骤、作用域或时间点不一致",
                            None,
                            Some(path),
                        ));
                    }
                }
            }
        }
    }

    // Intent-specific payload requirements.
    match draft.intent {
        EventIntent::Check => {
            if step_path.is_none() {
                issues.push(issue(
                    "missing_step",
                    "检查记录必须关联冻结的 Protocol 步骤",
                    None,
                    None,
                ));
            }
            if draft.checks.is_empty() {
                issues.push(issue(
                    "missing_checks",
                    "检查事件至少需要一项检查结果",
                    None,
                    step_path.as_deref(),
                ));
            }
        }
        EventIntent::Completion => {
            if step_path.is_none() {
                issues.push(issue(
                    "missing_step",
                    "完成事件必须关联冻结的 Protocol 步骤",
                    None,
                    None,
                ));
            }
            if let Some(status) = step_status.as_deref() {
                if status != "completed" {
                    issues.push(issue(
                        "step_status_conflict",
                        "完成事件的 step_status 只能是 completed",
                        None,
                        step_path.as_deref(),
                    ));
                }
            }
            step_status = Some("completed".to_owned());
        }
        EventIntent::Measurement => {
            if step_path.is_none() {
                issues.push(issue(
                    "missing_step",
                    "测量事件必须关联冻结的 Protocol 步骤",
                    None,
                    None,
                ));
            }
            if draft.measurements.is_empty() {
                issues.push(issue(
                    "missing_measurements",
                    "测量事件至少需要一项数值读数",
                    None,
                    step_path.as_deref(),
                ));
            }
        }
        EventIntent::MaterialUse => {
            if draft.materials.is_empty() {
                issues.push(issue(
                    "missing_materials",
                    "材料使用事件至少需要一项材料",
                    None,
                    step_path.as_deref(),
                ));
            }
        }
        EventIntent::Failure => {
            if deviation.is_none() {
                issues.push(issue(
                    "missing_deviation",
                    "失败事件必须说明实际偏差内容",
                    None,
                    step_path.as_deref(),
                ));
            }
            if step_path.is_some() {
                if let Some(status) = step_status.as_deref() {
                    if status != "blocked" {
                        issues.push(issue(
                            "step_status_conflict",
                            "带步骤的失败事件 step_status 只能是 blocked",
                            None,
                            step_path.as_deref(),
                        ));
                    }
                }
                step_status = Some("blocked".to_owned());
            }
        }
        EventIntent::Operation | EventIntent::Observation => {}
    }

    if draft.intent != EventIntent::MaterialUse && !draft.materials.is_empty() {
        issues.push(issue(
            "materials_not_allowed",
            "材料使用请使用 material_use 事件",
            None,
            step_path.as_deref(),
        ));
    }
    if draft.intent != EventIntent::Measurement && !draft.measurements.is_empty() {
        issues.push(issue(
            "measurements_not_allowed",
            "数值读数请使用 measurement 事件",
            None,
            step_path.as_deref(),
        ));
    }
    if !matches!(
        draft.intent,
        EventIntent::Check | EventIntent::Completion | EventIntent::Failure
    ) && !draft.checks.is_empty()
    {
        issues.push(issue(
            "checks_not_allowed",
            "检查结果请使用 check 或 completion 事件",
            None,
            step_path.as_deref(),
        ));
    }

    if draft.sample_ids.len() > 100 {
        issues.push(issue(
            "too_many_samples",
            "样本编号最多 100 个",
            None,
            step_path.as_deref(),
        ));
    }
    let mut sample_ids = Vec::with_capacity(draft.sample_ids.len());
    for sample in &draft.sample_ids {
        let sample = sample.trim();
        if sample.is_empty() || sample.chars().count() > 120 {
            issues.push(issue(
                "invalid_sample",
                "样本编号不能为空且不能超过 120 个字符",
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        sample_ids.push(sample.to_owned());
    }

    if draft.checks.len() > 100 {
        issues.push(issue(
            "too_many_checks",
            "同一事件最多 100 项检查",
            None,
            step_path.as_deref(),
        ));
    }
    let mut check_ids = HashSet::new();
    let mut checks = Vec::with_capacity(draft.checks.len());
    for check in &draft.checks {
        let check_id = check.check_id.trim();
        let status = check.status.trim();
        if check_id.is_empty() || check_id.chars().count() > 80 {
            issues.push(issue(
                "invalid_check",
                "检查项 ID 不能为空且不能超过 80 个字符",
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if !check_ids.insert(check_id.to_owned()) {
            issues.push(issue(
                "duplicate_check",
                &format!("同一事件不能重复记录检查项：{check_id}"),
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if !CHECK_STATUSES.contains(&status) {
            issues.push(issue(
                "invalid_check_status",
                &format!("检查项 {check_id} 的状态必须是 passed、failed、blocked 或 skipped"),
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if let Some(note) = check.note.as_deref() {
            if note.chars().count() > 2000 {
                issues.push(issue(
                    "invalid_check_note",
                    "检查说明不能超过 2000 个字符",
                    None,
                    step_path.as_deref(),
                ));
            }
        }
        if let Some(checked_by) = check.checked_by.as_deref() {
            if checked_by.trim().is_empty() || checked_by.chars().count() > 160 {
                issues.push(issue(
                    "invalid_checked_by",
                    "检查人不能为空且不能超过 160 个字符",
                    None,
                    step_path.as_deref(),
                ));
            }
        }
        checks.push(json!({
            "check_id": check_id,
            "status": status,
            "note": check.note,
            "checked_by": check.checked_by,
        }));
    }

    if draft.intent == EventIntent::Completion {
        if let Some(step) = resolved_step {
            for check in step.step.checks.iter().filter(|check| check.blocking) {
                let provided = checks.iter().find(|value| {
                    value["check_id"].as_str() == Some(check.check_id.as_str())
                });
                let passed = provided
                    .and_then(|value| value["status"].as_str())
                    == Some("passed");
                if !passed {
                    issues.push(issue(
                        "completion_blocked",
                        &format!(
                            "完成步骤前，blocking 检查项 {} 必须为 passed",
                            check.check_id
                        ),
                        None,
                        step_path.as_deref(),
                    ));
                }
            }
        }
    }

    if draft.measurements.len() > 100 {
        issues.push(issue(
            "too_many_measurements",
            "同一事件最多 100 项测量",
            None,
            step_path.as_deref(),
        ));
    }
    let mut measurements = Vec::with_capacity(draft.measurements.len());
    for measurement in &draft.measurements {
        let endpoint = measurement.endpoint.trim();
        if endpoint.is_empty() || endpoint.chars().count() > 300 {
            issues.push(issue(
                "invalid_measurement",
                "测量终点不能为空且不能超过 300 个字符",
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if !measurement.value.is_finite() {
            issues.push(issue(
                "invalid_measurement",
                &format!("测量 {endpoint} 的值必须是有限数值"),
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if measurement
            .unit
            .as_deref()
            .is_some_and(|unit| unit.chars().count() > 40)
        {
            issues.push(issue(
                "invalid_measurement",
                &format!("测量 {endpoint} 的单位不能超过 40 个字符"),
                None,
                step_path.as_deref(),
            ));
        }
        if measurement
            .sample_id
            .as_deref()
            .is_some_and(|sample| sample.chars().count() > 120)
        {
            issues.push(issue(
                "invalid_measurement",
                &format!("测量 {endpoint} 的样本编号不能超过 120 个字符"),
                None,
                step_path.as_deref(),
            ));
        }
        measurements.push(json!({
            "endpoint": endpoint,
            "value": measurement.value,
            "unit": measurement.unit,
            "sample_id": measurement.sample_id,
        }));
    }

    if draft.materials.len() > 100 {
        issues.push(issue(
            "too_many_materials",
            "同一事件最多 100 项材料",
            None,
            step_path.as_deref(),
        ));
    }
    let mut materials = Vec::with_capacity(draft.materials.len());
    for material in &draft.materials {
        let name = material.material.trim();
        if name.is_empty() || name.chars().count() > 160 {
            issues.push(issue(
                "invalid_material",
                "材料名称不能为空且不能超过 160 个字符",
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if let Some(amount) = material.amount {
            if !amount.is_finite() || amount < 0.0 {
                issues.push(issue(
                    "invalid_material",
                    &format!("材料 {name} 的用量必须是有限的非负数"),
                    None,
                    step_path.as_deref(),
                ));
                continue;
            }
        }
        if material.amount.is_some()
            != material
                .unit
                .as_deref()
                .is_some_and(|unit| !unit.trim().is_empty())
        {
            issues.push(issue(
                "invalid_material",
                &format!("材料 {name} 填写用量时必须同时填写单位"),
                None,
                step_path.as_deref(),
            ));
            continue;
        }
        if let Some(output_id) = material.planned_output_id.as_deref() {
            if !workspace
                .materials
                .iter()
                .any(|output| output.planned_output_id.as_deref() == Some(output_id))
            {
                issues.push(issue(
                    "unknown_planned_output",
                    &format!("planned_output_id 不属于本次 Run 的材料清单：{output_id}"),
                    None,
                    step_path.as_deref(),
                ));
            }
        }
        materials.push(json!({
            "material": name,
            "planned_output_id": material.planned_output_id,
            "amount": material.amount,
            "unit": material.unit,
        }));
    }

    if let Some(retry_id) = retry_of_event_id.as_deref() {
        validate_retry_target(
            retry_id,
            step_path.as_deref(),
            task_key.as_deref(),
            workspace,
            &mut issues,
        );
    }

    if !issues.is_empty() {
        return Err(issues);
    }

    let event_type = match draft.intent {
        EventIntent::Operation | EventIntent::Check | EventIntent::Completion => "operation",
        EventIntent::MaterialUse => "material_use",
        EventIntent::Measurement => "measurement",
        EventIntent::Failure => "deviation",
        EventIntent::Observation => "observation",
    };

    let mut payload = Map::new();
    payload.insert("run_id".to_owned(), json!(workspace.run_id));
    payload.insert("event_type".to_owned(), json!(event_type));
    payload.insert("content".to_owned(), json!(content));
    if let Some(at) = draft.occurred_at.as_deref().map(str::trim).filter(|at| !at.is_empty()) {
        payload.insert("occurred_at".to_owned(), json!(at));
    }
    if let Some(status) = step_status.as_deref() {
        payload.insert("step_status".to_owned(), json!(status));
    }
    if let Some(path) = step_path.as_deref() {
        payload.insert("protocol_step_path".to_owned(), json!(path));
        if let Some(version) = protocol_version_id.as_deref() {
            payload.insert("protocol_version_id".to_owned(), json!(version));
        }
        if let Some(assay) = assay_id.as_deref() {
            payload.insert("assay_id".to_owned(), json!(assay));
        }
        if let Some(output) = planned_output_id.as_deref() {
            payload.insert("planned_output_id".to_owned(), json!(output));
        }
        if let Some(timepoint) = timepoint.as_deref() {
            payload.insert("timepoint".to_owned(), json!(timepoint));
        }
    }
    if let Some(key) = task_key.as_deref() {
        payload.insert("task_key".to_owned(), json!(key));
    }
    if !sample_ids.is_empty() {
        payload.insert("sample_ids".to_owned(), json!(sample_ids));
    }
    if !materials.is_empty() {
        payload.insert("materials".to_owned(), json!(materials));
    }
    if !measurements.is_empty() {
        payload.insert("measurements".to_owned(), json!(measurements));
    }
    if !checks.is_empty() {
        payload.insert("checks".to_owned(), json!(checks));
    }
    if let Some(deviation) = deviation.as_deref() {
        payload.insert("deviation".to_owned(), json!(deviation));
    }
    if let Some(operator) = operator.as_deref() {
        payload.insert("operator".to_owned(), json!(operator));
    }
    if let Some(retry_id) = retry_of_event_id.as_deref() {
        payload.insert(
            "lineage".to_owned(),
            json!([{
                "from_kind": "execution_event",
                "from_id": retry_id,
                "to_kind": "step",
                "to_id": step_path,
                "relation": "retry"
            }]),
        );
    }

    Ok(EventAppendPlan {
        payload: Value::Object(payload),
        retry_of_event_id,
    })
}

/// Query the Run timeline, optionally narrowed to one step or task.
pub fn query_timeline(workspace: &RunWorkspace, query: &TimelineQuery) -> TimelineQueryResult {
    // Stored read models may hand events newest-first. Revision/attempt order
    // follows recorded time, never caller array order.
    let mut ordered: Vec<(usize, &ExecutionEventView)> =
        workspace.events.iter().enumerate().collect();
    ordered.sort_by_key(|(index, event)| {
        (
            crate::reading_facts::time_key(
                event
                    .recorded_at
                    .as_deref()
                    .or(event.occurred_at.as_deref())
                    .unwrap_or_default(),
            ),
            *index,
        )
    });

    let mut issues = Vec::new();
    let mut events = Vec::with_capacity(ordered.len());
    for (_, event) in ordered {
        let mut classified = classify_event(event);
        issues.append(&mut classified.issues);
        events.push(classified.event);
    }

    let mut id_to_index: HashMap<String, usize> = HashMap::new();
    for (index, event) in events.iter().enumerate() {
        if event.event_id.is_empty() {
            continue;
        }
        if id_to_index
            .insert(event.event_id.clone(), index)
            .is_some()
        {
            issues.push(issue(
                "duplicate_event_id",
                &format!("事件 ID {} 在本次 Run 中重复", event.event_id),
                Some(&event.event_id),
                event.step_path.as_deref(),
            ));
        }
    }

    let mut retries = Vec::new();
    let mut attempts = Vec::new();
    for index in 0..events.len() {
        let Some(target_id) = events[index].retry_of_event_id.clone() else {
            continue;
        };
        let Some(&target_index) = id_to_index.get(target_id.as_str()) else {
            issues.push(issue(
                "unknown_retry_target",
                &format!("重试事件指向不存在的事件 {target_id}"),
                Some(&events[index].event_id),
                events[index].step_path.as_deref(),
            ));
            continue;
        };
        if retry_chain_has_cycle(&events, &id_to_index, index) {
            issues.push(issue(
                "retry_cycle",
                &format!("事件 {} 的重试链形成循环", events[index].event_id),
                Some(&events[index].event_id),
                events[index].step_path.as_deref(),
            ));
            continue;
        }
        let attempt = retry_depth(&events, &id_to_index, index) + 1;
        attempts.push((index, attempt));
        let target = &events[target_index];
        if !target.outcome.is_failure() {
            issues.push(issue(
                "retry_target_not_failed",
                &format!(
                    "事件 {} 重试的目标 {} 没有失败或阻塞记录",
                    events[index].event_id, target_id
                ),
                Some(&events[index].event_id),
                events[index].step_path.as_deref(),
            ));
            continue;
        }
        if let (Some(retry_step), Some(target_step)) = (
            events[index].step_path.as_deref(),
            target.step_path.as_deref(),
        ) {
            if retry_step != target_step {
                issues.push(issue(
                    "retry_step_mismatch",
                    &format!(
                        "重试事件 {} 与目标 {} 不属于同一步骤",
                        events[index].event_id, target_id
                    ),
                    Some(&events[index].event_id),
                    events[index].step_path.as_deref(),
                ));
                continue;
            }
        }
        retries.push(RetryRecord {
            event_id: events[index].event_id.clone(),
            retry_of_event_id: target_id,
            step_path: events[index].step_path.clone(),
            task_key: events[index].task_key.clone(),
            attempt,
            previous_outcome: target.outcome,
            recorded_at: events[index].recorded_at.clone(),
        });
    }
    for (index, attempt) in attempts {
        events[index].attempt = attempt;
    }

    let matches = |event: &TimelineEvent| -> bool {
        if let Some(step) = query.step_path.as_deref() {
            if event.step_path.as_deref() != Some(step) {
                return false;
            }
        }
        if let Some(task) = query.task_key.as_deref() {
            if event.task_key.as_deref() != Some(task) {
                return false;
            }
        }
        if let Some(assay) = query.assay_id.as_deref() {
            if event.assay_id.as_deref() != Some(assay) {
                return false;
            }
        }
        if let Some(occurrence) = query.occurrence_id.as_deref() {
            if event.occurrence_id.as_deref() != Some(occurrence) {
                return false;
            }
        }
        if !query.kinds.is_empty()
            && !query
                .kinds
                .iter()
                .any(|kind| event.has_contribution(*kind))
        {
            return false;
        }
        if query.failures_only && !event.is_failure() {
            return false;
        }
        if query.retries_only && event.retry_of_event_id.is_none() {
            return false;
        }
        true
    };

    let events: Vec<TimelineEvent> = events.into_iter().filter(|event| matches(event)).collect();
    let selected: HashSet<&str> = events.iter().map(|event| event.event_id.as_str()).collect();
    let failures = events
        .iter()
        .filter(|event| event.is_failure())
        .cloned()
        .collect();
    retries.retain(|record| selected.contains(record.event_id.as_str()));
    issues.retain(|issue| match issue.event_id.as_deref() {
        Some(event_id) => selected.contains(event_id),
        None => query
            .step_path
            .as_deref()
            .map_or(true, |step| issue.step_path.as_deref() == Some(step)),
    });

    TimelineQueryResult {
        schema_version: EXECUTION_TIMELINE_SCHEMA_VERSION,
        run_id: workspace.run_id.clone(),
        events,
        failures,
        retries,
        issues,
    }
}

/// Convenience for one step path; same result as a [`TimelineQuery`] filter.
pub fn query_step_timeline(workspace: &RunWorkspace, step_path: &str) -> TimelineQueryResult {
    query_timeline(
        workspace,
        &TimelineQuery {
            step_path: Some(step_path.to_owned()),
            ..TimelineQuery::default()
        },
    )
}

const CHECK_STATUSES: [&str; 4] = ["passed", "failed", "blocked", "skipped"];

struct ClassifiedEvent {
    event: TimelineEvent,
    issues: Vec<TimelineIssue>,
}

fn classify_event(event: &ExecutionEventView) -> ClassifiedEvent {
    let mut issues = Vec::new();
    let event_id = event.event_id.trim().to_owned();
    if event_id.is_empty() {
        issues.push(issue(
            "missing_event_id",
            "执行事件缺少 event_id",
            None,
            event.protocol_step_path.as_deref(),
        ));
    }
    let step_path = event
        .protocol_step_path
        .as_deref()
        .map(str::trim)
        .filter(|step| !step.is_empty())
        .map(str::to_owned);
    let event_type = event
        .event_type
        .as_deref()
        .map(str::trim)
        .filter(|kind| !kind.is_empty())
        .map(str::to_owned);

    let mut contributions = Vec::new();
    match event_type.as_deref() {
        Some("operation") => contributions.push(EventContribution::Operation),
        Some("material_use") => contributions.push(EventContribution::MaterialUse),
        Some("measurement") => contributions.push(EventContribution::Measurement),
        Some("deviation") => contributions.push(EventContribution::Deviation),
        Some("observation") => contributions.push(EventContribution::Observation),
        Some(other) => issues.push(issue(
            "unknown_event_type",
            &format!("未知的执行事件类型：{other}"),
            Some(&event_id),
            step_path.as_deref(),
        )),
        None => issues.push(issue(
            "missing_event_type",
            "执行事件缺少 event_type，未按操作/检查/测量分类",
            Some(&event_id),
            step_path.as_deref(),
        )),
    }

    let mut checks = Vec::new();
    for raw in &event.checks {
        let check_id = raw["check_id"].as_str().unwrap_or_default().trim();
        let status = raw["status"].as_str().unwrap_or_default().trim();
        if check_id.is_empty() || !CHECK_STATUSES.contains(&status) {
            issues.push(issue(
                "invalid_check_status",
                &format!("检查项 {check_id:?} 的状态无效，未参与结果判定"),
                Some(&event_id),
                step_path.as_deref(),
            ));
            continue;
        }
        checks.push(CheckView {
            check_id: check_id.to_owned(),
            status: status.to_owned(),
            note: raw["note"].as_str().map(str::to_owned),
            checked_by: raw["checked_by"].as_str().map(str::to_owned),
        });
    }
    if !checks.is_empty() {
        contributions.push(EventContribution::Check);
        if step_path.is_none() {
            issues.push(issue(
                "check_without_step",
                "检查记录没有关联 Protocol 步骤",
                Some(&event_id),
                None,
            ));
        }
    }
    let step_status = event
        .step_status
        .as_deref()
        .map(str::trim)
        .filter(|status| !status.is_empty());
    if step_status == Some("completed") {
        contributions.push(EventContribution::Completion);
        if step_path.is_none() {
            issues.push(issue(
                "completion_without_step",
                "完成记录没有关联 Protocol 步骤",
                Some(&event_id),
                None,
            ));
        }
    }
    contributions.sort();
    contributions.dedup();

    let deviation = event
        .deviation
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned);
    let outcome = classify_outcome(&checks, step_status, event_type.as_deref(), deviation.as_deref());
    let retry_of_event_id = retry_target(event, &event_id, step_path.as_deref(), &mut issues);

    ClassifiedEvent {
        event: TimelineEvent {
            event_id,
            event_type,
            contributions,
            outcome,
            content: event.content.clone(),
            step_path,
            assay_id: event.assay_id.clone(),
            planned_output_id: event.planned_output_id.clone(),
            timepoint: event.timepoint.clone(),
            task_key: event.task_key.clone(),
            occurrence_id: event.occurrence_id.clone(),
            operator: event.operator.clone(),
            occurred_at: event.occurred_at.clone(),
            recorded_at: event.recorded_at.clone(),
            checks,
            measurements: event.measurements.clone(),
            materials: event.materials.clone(),
            deviation,
            retry_of_event_id,
            attempt: 1,
        },
        issues,
    }
}

fn classify_outcome(
    checks: &[CheckView],
    step_status: Option<&str>,
    event_type: Option<&str>,
    deviation: Option<&str>,
) -> EventOutcome {
    if checks.iter().any(|check| check.status == "failed") {
        return EventOutcome::Failed;
    }
    if checks.iter().any(|check| check.status == "blocked")
        || step_status == Some("blocked")
    {
        return EventOutcome::Blocked;
    }
    if event_type == Some("deviation") || deviation.is_some() {
        return EventOutcome::Failed;
    }
    if step_status == Some("completed") {
        return EventOutcome::Succeeded;
    }
    if step_status == Some("skipped")
        || (!checks.is_empty() && checks.iter().all(|check| check.status == "skipped"))
    {
        return EventOutcome::Skipped;
    }
    EventOutcome::Recorded
}

fn retry_target(
    event: &ExecutionEventView,
    event_id: &str,
    step_path: Option<&str>,
    issues: &mut Vec<TimelineIssue>,
) -> Option<String> {
    if let Some(explicit) = event
        .retry_of_event_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        return Some(explicit.to_owned());
    }
    let mut edges = Vec::new();
    for edge in &event.lineage {
        if edge["relation"].as_str() != Some("retry") {
            continue;
        }
        if edge["from_kind"].as_str() != Some("execution_event") {
            issues.push(issue(
                "invalid_retry_edge",
                "retry lineage 的 from_kind 必须是 execution_event",
                Some(event_id),
                step_path,
            ));
            continue;
        }
        let from_id = edge["from_id"].as_str().unwrap_or_default().trim();
        if from_id.is_empty() {
            issues.push(issue(
                "invalid_retry_edge",
                "retry lineage 缺少 from_id",
                Some(event_id),
                step_path,
            ));
            continue;
        }
        edges.push(from_id.to_owned());
    }
    if edges.len() > 1 {
        issues.push(issue(
            "multiple_retry_edges",
            "同一事件只能声明一个被重试事件",
            Some(event_id),
            step_path,
        ));
    }
    edges.into_iter().next()
}

fn retry_chain_has_cycle(
    events: &[TimelineEvent],
    id_to_index: &HashMap<String, usize>,
    start: usize,
) -> bool {
    let mut seen = HashSet::new();
    let mut current = start;
    loop {
        if !seen.insert(current) {
            return true;
        }
        let Some(target) = events[current]
            .retry_of_event_id
            .as_deref()
            .and_then(|target| id_to_index.get(target).copied())
        else {
            return false;
        };
        current = target;
    }
}

fn retry_depth(
    events: &[TimelineEvent],
    id_to_index: &HashMap<String, usize>,
    start: usize,
) -> u32 {
    let mut depth = 0;
    let mut current = start;
    while let Some(target) = events[current]
        .retry_of_event_id
        .as_deref()
        .and_then(|target| id_to_index.get(target).copied())
    {
        depth += 1;
        current = target;
        if depth > events.len() as u32 {
            break;
        }
    }
    depth
}

fn validate_retry_target(
    retry_id: &str,
    step_path: Option<&str>,
    task_key: Option<&str>,
    workspace: &RunWorkspace,
    issues: &mut Vec<TimelineIssue>,
) {
    let mut classified = Vec::with_capacity(workspace.events.len());
    for event in &workspace.events {
        classified.push(classify_event(event).event);
    }
    let Some(target_index) = classified.iter().position(|event| event.event_id == retry_id)
    else {
        issues.push(issue(
            "missing_retry_target",
            &format!("被重试事件 {retry_id} 不属于本次 Run"),
            None,
            step_path,
        ));
        return;
    };
    let target = &classified[target_index];
    if !target.outcome.is_failure() {
        issues.push(issue(
            "retry_target_not_failed",
            &format!("被重试事件 {retry_id} 没有失败或阻塞记录"),
            None,
            step_path,
        ));
    }
    if let (Some(retry_step), Some(target_step)) = (step_path, target.step_path.as_deref()) {
        if retry_step != target_step {
            issues.push(issue(
                "retry_step_mismatch",
                "重试事件必须与被重试事件属于同一步骤",
                None,
                step_path,
            ));
        }
    }
    if let (Some(retry_task), Some(target_task)) = (task_key, target.task_key.as_deref()) {
        if retry_task != target_task {
            issues.push(issue(
                "retry_task_mismatch",
                "重试事件必须与被重试事件属于同一任务",
                None,
                step_path,
            ));
        }
    }
    if classified.iter().any(|event| {
        event.retry_of_event_id.as_deref() == Some(retry_id)
            && event.outcome == EventOutcome::Succeeded
    }) {
        issues.push(issue(
            "already_retried",
            &format!("被重试事件 {retry_id} 已经存在成功的重试记录"),
            None,
            step_path,
        ));
    }
}

fn trim_opt(value: &Option<String>) -> Option<String> {
    value
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
}

fn is_rfc3339(value: &str) -> bool {
    let bytes = value.as_bytes();
    let number = |index: usize, count: usize| -> Option<u32> {
        let part = bytes.get(index..index + count)?;
        if !part.iter().all(u8::is_ascii_digit) {
            return None;
        }
        std::str::from_utf8(part).ok()?.parse().ok()
    };
    let year = number(0, 4);
    let month = number(5, 2);
    let day = number(8, 2);
    let hour = number(11, 2);
    let minute = number(14, 2);
    let second = number(17, 2);
    if !(bytes.get(4) == Some(&b'-')
        && bytes.get(7) == Some(&b'-')
        && matches!(bytes.get(10), Some(b'T') | Some(b't'))
        && bytes.get(13) == Some(&b':')
        && bytes.get(16) == Some(&b':'))
    {
        return false;
    }
    let (Some(_year), Some(month), Some(day), Some(hour), Some(minute), Some(second)) =
        (year, month, day, hour, minute, second)
    else {
        return false;
    };
    if !(1..=12).contains(&month) || day == 0 || hour > 23 || minute > 59 || second > 60 {
        return false;
    }
    let leap = year.is_some_and(|year| year % 4 == 0 && (year % 100 != 0 || year % 400 == 0));
    let max_day = match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if leap => 29,
        2 => 28,
        _ => return false,
    };
    if day > max_day {
        return false;
    }
    let mut index = 19;
    if bytes.get(index) == Some(&b'.') {
        index += 1;
        let start = index;
        while bytes.get(index).is_some_and(u8::is_ascii_digit) {
            index += 1;
        }
        if !(1..=9).contains(&(index - start)) {
            return false;
        }
    }
    match bytes.get(index) {
        Some(b'Z') | Some(b'z') => index + 1 == bytes.len(),
        Some(b'+') | Some(b'-') => {
            let offset_hour = number(index + 1, 2);
            let offset_minute = number(index + 4, 2);
            bytes.get(index + 3) == Some(&b':')
                && offset_hour.is_some_and(|value| value <= 23)
                && offset_minute.is_some_and(|value| value <= 59)
                && index + 6 == bytes.len()
        }
        _ => false,
    }
}

fn issue(
    code: &str,
    message: &str,
    event_id: Option<&str>,
    step_path: Option<&str>,
) -> TimelineIssue {
    TimelineIssue {
        code: code.to_owned(),
        message: message.to_owned(),
        event_id: event_id.map(str::to_owned),
        step_path: step_path.map(str::to_owned),
    }
}


fn run_scope_matches(scope: &StepScope, run_scope: &RunScope) -> bool {
    match scope {
        StepScope::Assay { assay_id } => {
            run_scope.kind == "assay" && run_scope.id == *assay_id
        }
        StepScope::PlannedOutput { planned_output_id } => {
            run_scope.kind == "planned_output" && run_scope.id == *planned_output_id
        }
        StepScope::Unscoped => run_scope.kind == "global",
    }
}

fn scope_matches(scope: &StepScope, assay_id: Option<&str>, planned_output_id: Option<&str>) -> bool {
    match scope {
        StepScope::Assay {
            assay_id: scope_assay,
        } => planned_output_id.is_none() && assay_id == Some(scope_assay.as_str()),
        StepScope::PlannedOutput {
            planned_output_id: scope_output,
        } => assay_id.is_none() && planned_output_id == Some(scope_output.as_str()),
        StepScope::Unscoped => assay_id.is_none() && planned_output_id.is_none(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::run_workspace;
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
                    {"path":"root/seed","use_id":"seed","title":"Seed","instruction":"seed",
                     "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1","time_scope":"once"},
                    {"path":"root/read","use_id":"read","title":"Read","instruction":"read",
                     "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1","time_scope":"per_timepoint",
                     "checks":[{"check_id":"signal","label":"Signal","blocking":true}]}
                ]
            }],
            "material_manifest":[]
        })
    }

    fn workspace(events: Vec<Value>) -> RunWorkspace {
        run_workspace::build("r1", "d1", "dv1", 1, &design(), &run(), &[], &events).unwrap()
    }

    fn step_event(id: &str, recorded_at: &str, event_type: &str) -> Value {
        json!({
            "id": id,
            "event_type": event_type,
            "content": id,
            "recorded_at": recorded_at,
            "protocol_step_path": "root/read",
            "protocol_version_id": "pv1",
            "assay_id": "cck8",
            "timepoint": "D1"
        })
    }

    fn check(status: &str) -> Value {
        json!([{"check_id":"signal","status":status}])
    }

    fn base_draft(intent: EventIntent) -> EventDraft {
        EventDraft {
            run_id: "r1".into(),
            intent,
            content: "event".into(),
            step_path: Some("root/read".into()),
            protocol_version_id: Some("pv1".into()),
            assay_id: Some("cck8".into()),
            planned_output_id: None,
            timepoint: Some("D1".into()),
            task_key: None,
            operator: None,
            occurred_at: None,
            step_status: None,
            sample_ids: Vec::new(),
            checks: Vec::new(),
            measurements: Vec::new(),
            materials: Vec::new(),
            deviation: None,
            retry_of_event_id: None,
        }
    }

    #[test]
    fn timeline_orders_events_by_recorded_time_not_array_order() {
        let later = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        let earlier = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        let result = query_timeline(
            &workspace(vec![later, earlier]),
            &TimelineQuery::default(),
        );
        let ids: Vec<&str> = result.events.iter().map(|e| e.event_id.as_str()).collect();
        assert_eq!(ids, vec!["e1", "e2"]);
    }

    #[test]
    fn check_completion_and_measurement_are_explicit_contributions() {
        let mut completion = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        completion["checks"] = check("passed");
        completion["step_status"] = json!("completed");
        let mut measurement = step_event("e2", "2026-01-01T00:00:02Z", "measurement");
        measurement["measurements"] = json!([{"endpoint":"od450","value":1.2,"unit":"AU"}]);
        let result = query_timeline(
            &workspace(vec![completion, measurement]),
            &TimelineQuery::default(),
        );
        assert!(result.events[0].has_contribution(EventContribution::Operation));
        assert!(result.events[0].has_contribution(EventContribution::Check));
        assert!(result.events[0].has_contribution(EventContribution::Completion));
        assert_eq!(result.events[0].outcome, EventOutcome::Succeeded);
        assert!(result.events[1].has_contribution(EventContribution::Measurement));
    }

    #[test]
    fn failed_check_and_blocked_step_are_failure_records() {
        let mut failed = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        failed["checks"] = check("failed");
        let mut blocked = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        blocked["step_status"] = json!("blocked");
        let result = query_timeline(
            &workspace(vec![failed, blocked]),
            &TimelineQuery::default(),
        );
        assert_eq!(result.events[0].outcome, EventOutcome::Failed);
        assert_eq!(result.events[1].outcome, EventOutcome::Blocked);
        assert_eq!(result.failures.len(), 2);
    }

    #[test]
    fn retry_edge_links_to_failed_event_and_counts_attempt() {
        let mut failed = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        failed["checks"] = check("failed");
        let mut retry = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        retry["lineage"] = json!([{
            "from_kind":"execution_event","from_id":"e1",
            "to_kind":"step","to_id":"root/read","relation":"retry"
        }]);
        let result = query_timeline(
            &workspace(vec![failed, retry]),
            &TimelineQuery::default(),
        );
        assert_eq!(result.events[1].retry_of_event_id.as_deref(), Some("e1"));
        assert_eq!(result.events[1].attempt, 2);
        assert_eq!(result.retries.len(), 1);
        assert_eq!(result.retries[0].previous_outcome, EventOutcome::Failed);
        assert_eq!(result.retries[0].attempt, 2);
    }

    #[test]
    fn step_filter_keeps_only_that_step_failures() {
        let mut read = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        read["checks"] = check("failed");
        let mut seed = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        seed["protocol_step_path"] = json!("root/seed");
        seed["timepoint"] = Value::Null;
        let result = query_step_timeline(&workspace(vec![read, seed]), "root/read");
        assert_eq!(result.events.len(), 1);
        assert_eq!(result.events[0].event_id, "e1");
        assert_eq!(result.failures.len(), 1);
    }

    #[test]
    fn unknown_retry_target_is_reported_not_guessed() {
        let mut retry = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        retry["lineage"] = json!([{
            "from_kind":"execution_event","from_id":"missing",
            "to_kind":"step","to_id":"root/read","relation":"retry"
        }]);
        let result = query_timeline(&workspace(vec![retry]), &TimelineQuery::default());
        assert!(result.retries.is_empty());
        assert!(result
            .issues
            .iter()
            .any(|issue| issue.code == "unknown_retry_target"));
        assert_eq!(result.events[0].attempt, 1);
    }

    #[test]
    fn retry_cycle_is_reported() {
        let mut first = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        first["lineage"] = json!([{
            "from_kind":"execution_event","from_id":"e2",
            "to_kind":"step","to_id":"root/read","relation":"retry"
        }]);
        let mut second = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        second["lineage"] = json!([{
            "from_kind":"execution_event","from_id":"e1",
            "to_kind":"step","to_id":"root/read","relation":"retry"
        }]);
        let result = query_timeline(
            &workspace(vec![first, second]),
            &TimelineQuery::default(),
        );
        assert!(result.retries.is_empty());
        assert!(result.issues.iter().any(|issue| issue.code == "retry_cycle"));
    }

    #[test]
    fn plan_check_event_produces_scoped_operation_payload() {
        let mut draft = base_draft(EventIntent::Check);
        draft.checks = vec![CheckInput {
            check_id: "signal".into(),
            status: "passed".into(),
            note: None,
            checked_by: None,
        }];
        let plan = plan_event(&draft, &workspace(Vec::new())).unwrap();
        assert_eq!(plan.payload["event_type"], "operation");
        assert_eq!(plan.payload["protocol_step_path"], "root/read");
        assert_eq!(plan.payload["assay_id"], "cck8");
        assert_eq!(plan.payload["checks"][0]["check_id"], "signal");
        assert!(plan.payload.get("step_status").is_none());
    }

    #[test]
    fn plan_completion_requires_blocking_check_passed() {
        let mut draft = base_draft(EventIntent::Completion);
        let error = plan_event(&draft, &workspace(Vec::new())).unwrap_err();
        assert!(error.iter().any(|issue| issue.code == "completion_blocked"));
        draft.checks = vec![
            CheckInput {
                check_id: "signal".into(),
                status: "passed".into(),
                note: None,
                checked_by: None,
            },
        ];
        let plan = plan_event(&draft, &workspace(Vec::new())).unwrap();
        assert_eq!(plan.payload["event_type"], "operation");
        assert_eq!(plan.payload["step_status"], "completed");
    }

    #[test]
    fn plan_measurement_requires_numeric_readings_and_frozen_step() {
        let mut valid = base_draft(EventIntent::Measurement);
        valid.measurements = vec![MeasurementInput {
            endpoint: "od450".into(),
            value: 1.2,
            unit: Some("AU".into()),
            sample_id: None,
        }];
        let plan = plan_event(&valid, &workspace(Vec::new())).unwrap();
        assert_eq!(plan.payload["event_type"], "measurement");
        assert_eq!(plan.payload["measurements"][0]["endpoint"], "od450");

        let mut unknown = valid.clone();
        unknown.step_path = Some("root/missing".into());
        let error = plan_event(&unknown, &workspace(Vec::new())).unwrap_err();
        assert!(error.iter().any(|issue| issue.code == "unknown_step"));
    }

    #[test]
    fn plan_failure_records_deviation_and_blocked_step() {
        let mut draft = base_draft(EventIntent::Failure);
        draft.deviation = Some("incubator overheated".into());
        let plan = plan_event(&draft, &workspace(Vec::new())).unwrap();
        assert_eq!(plan.payload["event_type"], "deviation");
        assert_eq!(plan.payload["step_status"], "blocked");
        assert_eq!(plan.payload["deviation"], "incubator overheated");
    }

    #[test]
    fn plan_retry_emits_lineage_and_round_trips_through_query() {
        let mut failed = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        failed["checks"] = check("failed");
        let mut draft = base_draft(EventIntent::Operation);
        draft.retry_of_event_id = Some("e1".into());
        let plan = plan_event(&draft, &workspace(vec![failed.clone()])).unwrap();
        let edge = &plan.payload["lineage"][0];
        assert_eq!(edge["from_id"], "e1");
        assert_eq!(edge["relation"], "retry");

        let mut stored = plan.payload.clone();
        stored["id"] = json!("e2");
        stored["recorded_at"] = json!("2026-01-01T00:00:02Z");
        let result = query_timeline(
            &workspace(vec![failed, stored]),
            &TimelineQuery::default(),
        );
        assert_eq!(result.retries.len(), 1);
        assert_eq!(result.retries[0].retry_of_event_id, "e1");
        assert_eq!(result.events[1].attempt, 2);
    }

    #[test]
    fn plan_rejects_retry_of_succeeded_event() {
        let succeeded = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        let mut draft = base_draft(EventIntent::Operation);
        draft.retry_of_event_id = Some("e1".into());
        let error = plan_event(&draft, &workspace(vec![succeeded])).unwrap_err();
        assert!(error
            .iter()
            .any(|issue| issue.code == "retry_target_not_failed"));
    }

    #[test]
    fn plan_rejects_timepoint_without_assay_and_unknown_task() {
        let mut draft = base_draft(EventIntent::Operation);
        draft.assay_id = None;
        let error = plan_event(&draft, &workspace(Vec::new())).unwrap_err();
        assert!(error
            .iter()
            .any(|issue| issue.code == "timepoint_without_assay"));

        let mut draft = base_draft(EventIntent::Operation);
        draft.task_key = Some("missing".into());
        let error = plan_event(&draft, &workspace(Vec::new())).unwrap_err();
        assert!(error.iter().any(|issue| issue.code == "unknown_task"));
    }

    #[test]
    fn plan_rejects_step_fields_without_a_step() {
        let mut draft = base_draft(EventIntent::Operation);
        draft.step_path = None;
        let error = plan_event(&draft, &workspace(Vec::new())).unwrap_err();
        assert!(error
            .iter()
            .any(|issue| issue.code == "field_requires_step"));
    }

    #[test]
    fn retry_chain_reports_each_attempt_explicitly() {
        let mut first = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        first["checks"] = check("failed");
        let mut second = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        second["checks"] = check("failed");
        second["lineage"] = json!([{
            "from_kind":"execution_event","from_id":"e1",
            "to_kind":"step","to_id":"root/read","relation":"retry"
        }]);
        let mut third = step_event("e3", "2026-01-01T00:00:03Z", "operation");
        third["retry_of_event_id"] = json!("e2");
        let result = query_timeline(
            &workspace(vec![first, second, third]),
            &TimelineQuery::default(),
        );
        assert_eq!(result.retries.len(), 2);
        assert_eq!(result.events[1].attempt, 2);
        assert_eq!(result.events[2].attempt, 3);
        assert_eq!(result.events[2].retry_of_event_id.as_deref(), Some("e2"));
    }

    #[test]
    fn plan_rejects_wrong_protocol_version_and_invalid_timestamp() {
        let mut draft = base_draft(EventIntent::Operation);
        draft.protocol_version_id = Some("pv2".into());
        draft.occurred_at = Some("2026-13-40T00:00:00Z".into());
        let error = plan_event(&draft, &workspace(Vec::new())).unwrap_err();
        assert!(error
            .iter()
            .any(|issue| issue.code == "unknown_protocol_version"));
        assert!(error
            .iter()
            .any(|issue| issue.code == "invalid_occurred_at"));
    }

    #[test]
    fn plan_rejects_unknown_check_status_and_duplicate_check() {
        let mut draft = base_draft(EventIntent::Check);
        draft.checks = vec![
            CheckInput {
                check_id: "signal".into(),
                status: "maybe".into(),
                note: None,
                checked_by: None,
            },
            CheckInput {
                check_id: "signal".into(),
                status: "passed".into(),
                note: None,
                checked_by: None,
            },
        ];
        let error = plan_event(&draft, &workspace(Vec::new())).unwrap_err();
        assert!(error
            .iter()
            .any(|issue| issue.code == "invalid_check_status"));
        assert!(error.iter().any(|issue| issue.code == "duplicate_check"));
    }

    #[test]
    fn query_filters_failures_and_retries_only() {
        let mut failed = step_event("e1", "2026-01-01T00:00:01Z", "operation");
        failed["checks"] = check("failed");
        let ok = step_event("e2", "2026-01-01T00:00:02Z", "operation");
        let mut retry = step_event("e3", "2026-01-01T00:00:03Z", "operation");
        retry["lineage"] = json!([{
            "from_kind":"execution_event","from_id":"e1",
            "to_kind":"step","to_id":"root/read","relation":"retry"
        }]);
        let ws = workspace(vec![failed, ok, retry]);
        let failures = query_timeline(
            &ws,
            &TimelineQuery {
                failures_only: true,
                ..TimelineQuery::default()
            },
        );
        assert_eq!(failures.events.len(), 1);
        assert_eq!(failures.events[0].event_id, "e1");
        let retries = query_timeline(
            &ws,
            &TimelineQuery {
                retries_only: true,
                ..TimelineQuery::default()
            },
        );
        assert_eq!(retries.events.len(), 1);
        assert_eq!(retries.events[0].event_id, "e3");
        assert_eq!(retries.retries.len(), 1);
    }

    #[test]
    fn legacy_event_without_type_is_flagged() {
        let legacy = json!({"id":"e1","recorded_at":"2026-01-01T00:00:01Z",
            "protocol_step_path":"root/read"});
        let result = query_timeline(&workspace(vec![legacy]), &TimelineQuery::default());
        assert!(result.events[0].contributions.is_empty());
        assert!(result
            .issues
            .iter()
            .any(|issue| issue.code == "missing_event_type"));
    }
}
