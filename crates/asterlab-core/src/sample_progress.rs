//! Sample-level execution identity and per-step sample progress.
//!
//! A Run answers "what actually happened" for each sample, not only for each
//! step. The execution identity of one *attempt* is:
//!
//! ```text
//! run_id + step_path + sample_id + attempt
//! ```
//!
//! Attempts are append-only: the first record of an identity is attempt 1, and
//! an explicit retry appends attempt `n + 1`. Older attempts stay queryable so a
//! retry never rewrites or hides the failed record. Progress is derived from the
//! stored execution events; this module owns no store and writes nothing.
//!
//! A step is only `completed` when every sample in the matrix is completed.
//! Mixed or partial matrices stay `in_progress`, and a failure anywhere keeps
//! the step `failed` even when other samples succeeded.

use crate::run_workspace::{ExecutionEventView, RunWorkspace};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashMap};

/// Schema version of the progress read model. Bump only for a breaking change.
pub const SAMPLE_PROGRESS_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SampleStatus {
    NotStarted,
    InProgress,
    Completed,
    Failed,
    Skipped,
}

/// One immutable attempt of one sample on one frozen step.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SampleExecutionIdentity {
    pub run_id: String,
    pub step_path: String,
    pub sample_id: String,
    pub attempt: u32,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ProgressIssue {
    pub code: String,
    pub message: String,
    pub step_path: Option<String>,
    pub sample_id: Option<String>,
    pub event_id: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default, deny_unknown_fields)]
pub struct SampleProgressQuery {
    /// Restrict the matrix to one frozen step path.
    pub step_path: Option<String>,
    /// Restrict the matrix to one sample.
    pub sample_id: Option<String>,
    /// Additional samples that must appear even before their first event.
    pub sample_ids: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default, deny_unknown_fields)]
pub struct ProgressEvent {
    pub event_id: String,
    pub step_path: Option<String>,
    pub sample_ids: Vec<String>,
    /// Explicit attempt carried by the write; `None` is derived from retries.
    pub attempt: Option<u32>,
    pub retry_of_event_id: Option<String>,
    pub event_type: Option<String>,
    pub step_status: Option<String>,
    pub deviation: bool,
    pub recorded_at: Option<String>,
    pub occurred_at: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AttemptProgress {
    pub attempt: u32,
    pub status: SampleStatus,
    pub event_ids: Vec<String>,
    pub last_event_at: Option<String>,
    pub deviation: bool,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SampleProgress {
    pub sample_id: String,
    pub status: SampleStatus,
    /// Latest attempt number, or `None` before the first event.
    pub attempt: Option<u32>,
    pub attempt_count: u32,
    pub last_event_at: Option<String>,
    /// Every attempt, oldest first; old attempts are never dropped.
    pub attempts: Vec<AttemptProgress>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct StepProgress {
    pub step_path: String,
    pub status: SampleStatus,
    /// True only when every sample is completed.
    pub complete: bool,
    pub total_samples: u32,
    pub completed_samples: u32,
    pub failed_samples: u32,
    pub skipped_samples: u32,
    pub in_progress_samples: u32,
    pub not_started_samples: u32,
    pub last_event_at: Option<String>,
    pub samples: Vec<SampleProgress>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct WorkflowProgress {
    pub schema_version: u32,
    pub run_id: String,
    pub sample_ids: Vec<String>,
    pub steps: Vec<StepProgress>,
    pub issues: Vec<ProgressIssue>,
}

/// Project the sample matrix of one Run from its frozen steps and events.
pub fn build(
    run_id: &str,
    step_paths: &[String],
    events: &[ProgressEvent],
    query: &SampleProgressQuery,
) -> WorkflowProgress {
    let mut issues = Vec::new();
    let mut known_steps: Vec<String> = Vec::new();
    for path in step_paths {
        let path = path.trim();
        if path.is_empty() || known_steps.iter().any(|known| known == path) {
            continue;
        }
        known_steps.push(path.to_owned());
    }
    let step_filter = query
        .step_path
        .as_deref()
        .map(str::trim)
        .filter(|path| !path.is_empty());
    let steps: Vec<String> = known_steps
        .iter()
        .filter(|path| step_filter.is_none_or(|filter| filter == path.as_str()))
        .cloned()
        .collect();

    let resolved = resolve_events(events, &mut issues);

    let mut samples: Vec<String> = Vec::new();
    for event in &resolved {
        for sample in &event.event.sample_ids {
            let sample = sample.trim();
            if sample.is_empty() || samples.iter().any(|known| known == sample) {
                continue;
            }
            samples.push(sample.to_owned());
        }
    }
    for sample in &query.sample_ids {
        let sample = sample.trim();
        if sample.is_empty() || samples.iter().any(|known| known == sample) {
            continue;
        }
        samples.push(sample.to_owned());
    }
    if let Some(filter) = query
        .sample_id
        .as_deref()
        .map(str::trim)
        .filter(|sample| !sample.is_empty())
    {
        if !samples.iter().any(|known| known == filter) {
            samples.push(filter.to_owned());
        }
        samples.retain(|sample| sample == filter);
    }

    for event in &resolved {
        let event = event.event;
        let Some(step) = event.step_path.as_deref().map(str::trim).filter(|s| !s.is_empty()) else {
            issues.push(issue(
                "missing_step",
                "执行事件没有关联冻结步骤，不能进入样本矩阵",
                None,
                None,
                Some(&event.event_id),
            ));
            continue;
        };
        if step_filter.is_some_and(|filter| filter != step) {
            continue;
        }
        if !known_steps.iter().any(|known| known == step) {
            issues.push(issue(
                "unknown_step",
                &format!("执行事件引用了本次冻结 Run 之外的步骤 {step}"),
                Some(step),
                None,
                Some(&event.event_id),
            ));
        }
    }

    let steps = steps
        .into_iter()
        .map(|step_path| step_progress(&step_path, &samples, &resolved))
        .collect();

    WorkflowProgress {
        schema_version: SAMPLE_PROGRESS_SCHEMA_VERSION,
        run_id: run_id.to_owned(),
        sample_ids: samples,
        steps,
        issues,
    }
}

fn step_progress(
    step_path: &str,
    samples: &[String],
    events: &[ResolvedEvent<'_>],
) -> StepProgress {
    let step_events: Vec<(usize, u32)> = events
        .iter()
        .enumerate()
        .filter(|(_, event)| {
            event
                .event
                .step_path
                .as_deref()
                .map(str::trim)
                == Some(step_path)
        })
        .map(|(index, event)| (index, event.attempt))
        .collect();

    let last_event_at = step_events.last().and_then(|(index, _)| event_time(&events[*index].event));

    let samples: Vec<SampleProgress> = samples
        .iter()
        .map(|sample_id| sample_progress(sample_id, events, &step_events))
        .collect();

    let total = samples.len() as u32;
    let count = |status: SampleStatus| {
        samples
            .iter()
            .filter(|sample| sample.status == status)
            .count() as u32
    };
    let completed = count(SampleStatus::Completed);
    let failed = count(SampleStatus::Failed);
    let skipped = count(SampleStatus::Skipped);
    let in_progress = count(SampleStatus::InProgress);
    let not_started = count(SampleStatus::NotStarted);

    let status = if total == 0 {
        SampleStatus::NotStarted
    } else if completed == total {
        SampleStatus::Completed
    } else if failed > 0 {
        SampleStatus::Failed
    } else if skipped == total {
        SampleStatus::Skipped
    } else if not_started == total {
        SampleStatus::NotStarted
    } else {
        SampleStatus::InProgress
    };

    StepProgress {
        step_path: step_path.to_owned(),
        status,
        complete: total > 0 && completed == total,
        total_samples: total,
        completed_samples: completed,
        failed_samples: failed,
        skipped_samples: skipped,
        in_progress_samples: in_progress,
        not_started_samples: not_started,
        last_event_at,
        samples,
    }
}

fn sample_progress(
    sample_id: &str,
    events: &[ResolvedEvent<'_>],
    step_events: &[(usize, u32)],
) -> SampleProgress {
    let mut attempts: BTreeMap<u32, AttemptProgress> = BTreeMap::new();
    let mut last_event_at = None;
    for (index, attempt) in step_events {
        let event = events[*index].event;
        if !event.sample_ids.iter().any(|sample| sample.trim() == sample_id) {
            continue;
        }
        let status = event_status(event);
        let deviation = event.deviation || event.event_type.as_deref() == Some("deviation");
        let entry = attempts.entry(*attempt).or_insert_with(|| AttemptProgress {
            attempt: *attempt,
            status,
            event_ids: Vec::new(),
            last_event_at: None,
            deviation: false,
        });
        entry.status = status;
        entry.deviation |= deviation;
        if !event.event_id.is_empty() && !entry.event_ids.iter().any(|id| id == &event.event_id) {
            entry.event_ids.push(event.event_id.clone());
        }
        entry.last_event_at = event_time(event);
        last_event_at = event_time(event);
    }

    let current = attempts.values().next_back();
    SampleProgress {
        sample_id: sample_id.to_owned(),
        status: current.map(|attempt| attempt.status).unwrap_or(SampleStatus::NotStarted),
        attempt: current.map(|attempt| attempt.attempt),
        attempt_count: attempts.len() as u32,
        last_event_at,
        attempts: attempts.into_values().collect(),
    }
}

fn event_status(event: &ProgressEvent) -> SampleStatus {
    match event.step_status.as_deref().map(str::trim) {
        Some("completed") => SampleStatus::Completed,
        Some("blocked") => SampleStatus::Failed,
        Some("skipped") => SampleStatus::Skipped,
        Some("in_progress") => SampleStatus::InProgress,
        _ if event.event_type.as_deref() == Some("deviation") || event.deviation => {
            SampleStatus::Failed
        }
        _ => SampleStatus::InProgress,
    }
}

fn event_time(event: &ProgressEvent) -> Option<String> {
    event
        .recorded_at
        .as_deref()
        .or(event.occurred_at.as_deref())
        .map(str::trim)
        .filter(|time| !time.is_empty())
        .map(str::to_owned)
}

fn event_time_key(event: &ProgressEvent) -> String {
    crate::reading_facts::time_key(event_time(event).as_deref().unwrap_or_default())
}

/// Project the sample matrix of a frozen Run workspace.
pub fn build_from_workspace(
    workspace: &RunWorkspace,
    query: &SampleProgressQuery,
) -> WorkflowProgress {
    let steps: Vec<String> = workspace
        .steps
        .iter()
        .map(|step| step.step.path.clone())
        .collect();
    let events: Vec<ProgressEvent> = workspace.events.iter().map(ProgressEvent::from_view).collect();
    build(&workspace.run_id, &steps, &events, query)
}

impl ProgressEvent {
    pub fn from_view(view: &ExecutionEventView) -> Self {
        ProgressEvent {
            event_id: view.event_id.clone(),
            step_path: view.protocol_step_path.clone(),
            sample_ids: view.sample_ids.clone(),
            attempt: view.attempt,
            retry_of_event_id: view
                .retry_of_event_id
                .clone()
                .or_else(|| retry_link(&view.lineage)),
            event_type: view.event_type.clone(),
            step_status: view.step_status.clone(),
            deviation: view.deviation.is_some(),
            recorded_at: view.recorded_at.clone(),
            occurred_at: view.occurred_at.clone(),
        }
    }

    /// Read one stored `execution_event` data object (plus its `id`).
    pub fn from_stored(value: &Value) -> Self {
        ProgressEvent {
            event_id: str_field(value, "id").or_else(|| str_field(value, "event_id")).unwrap_or_default(),
            step_path: str_field(value, "protocol_step_path"),
            sample_ids: value["sample_ids"]
                .as_array()
                .map(|items| {
                    items
                        .iter()
                        .filter_map(Value::as_str)
                        .map(str::trim)
                        .filter(|sample| !sample.is_empty())
                        .map(str::to_owned)
                        .collect()
                })
                .unwrap_or_default(),
            attempt: value["attempt"].as_u64().map(|attempt| attempt as u32),
            retry_of_event_id: str_field(value, "retry_of_event_id").or_else(|| {
                value["lineage"]
                    .as_array()
                    .and_then(|links| retry_link(links))
            }),
            event_type: str_field(value, "event_type"),
            step_status: str_field(value, "step_status"),
            deviation: value["deviation"].as_str().is_some_and(|value| !value.trim().is_empty()),
            recorded_at: str_field(value, "recorded_at"),
            occurred_at: str_field(value, "occurred_at"),
        }
    }
}

struct ResolvedEvent<'a> {
    event: &'a ProgressEvent,
    attempt: u32,
}

/// Order events by recorded time (never caller array order) and resolve the
/// attempt of each one: an explicit `attempt` wins, otherwise an explicit retry
/// link to an earlier event of the same step adds one attempt.
fn resolve_events<'a>(
    events: &'a [ProgressEvent],
    issues: &mut Vec<ProgressIssue>,
) -> Vec<ResolvedEvent<'a>> {
    let mut ordered: Vec<usize> = (0..events.len()).collect();
    ordered.sort_by_key(|index| (event_time_key(&events[*index]), *index));
    let mut by_id: HashMap<&str, usize> = HashMap::new();
    for (position, index) in ordered.iter().enumerate() {
        let id = events[*index].event_id.as_str();
        if !id.is_empty() {
            by_id.entry(id).or_insert(position);
        }
    }
    let mut cache: Vec<Option<u32>> = vec![None; ordered.len()];
    let mut stack: Vec<usize> = Vec::new();
    for position in 0..ordered.len() {
        resolve_attempt(
            position,
            events,
            &ordered,
            &by_id,
            &mut cache,
            &mut stack,
            issues,
        );
    }
    ordered
        .iter()
        .enumerate()
        .map(|(position, index)| ResolvedEvent {
            event: &events[*index],
            attempt: cache[position].unwrap_or(1),
        })
        .collect()
}

fn resolve_attempt(
    position: usize,
    events: &[ProgressEvent],
    ordered: &[usize],
    by_id: &HashMap<&str, usize>,
    cache: &mut [Option<u32>],
    stack: &mut Vec<usize>,
    issues: &mut Vec<ProgressIssue>,
) -> u32 {
    if let Some(attempt) = cache[position] {
        return attempt;
    }
    let event = &events[ordered[position]];
    if stack.contains(&position) {
        issues.push(issue(
            "retry_cycle",
            "重试链形成循环，无法确定 attempt",
            event.step_path.as_deref(),
            None,
            Some(&event.event_id),
        ));
        return 1;
    }
    let attempt = if let Some(explicit) = event.attempt {
        if explicit == 0 {
            issues.push(issue(
                "invalid_attempt",
                "attempt 必须从 1 开始",
                event.step_path.as_deref(),
                None,
                Some(&event.event_id),
            ));
            1
        } else {
            explicit
        }
    } else if let Some(target_id) = event
        .retry_of_event_id
        .as_deref()
        .map(str::trim)
        .filter(|id| !id.is_empty())
    {
        match by_id.get(target_id) {
            Some(&target) if same_step(&events[ordered[target]], event) => {
                stack.push(position);
                let base = resolve_attempt(target, events, ordered, by_id, cache, stack, issues);
                stack.pop();
                base.saturating_add(1)
            }
            _ => 1,
        }
    } else {
        1
    };
    cache[position] = Some(attempt);
    attempt
}

fn same_step(left: &ProgressEvent, right: &ProgressEvent) -> bool {
    left.step_path.as_deref().map(str::trim) == right.step_path.as_deref().map(str::trim)
}

fn issue(
    code: &str,
    message: &str,
    step_path: Option<&str>,
    sample_id: Option<&str>,
    event_id: Option<&str>,
) -> ProgressIssue {
    ProgressIssue {
        code: code.to_owned(),
        message: message.to_owned(),
        step_path: step_path.map(str::to_owned),
        sample_id: sample_id.map(str::to_owned),
        event_id: event_id.map(str::to_owned),
    }
}

/// Validate an explicit attempt claim and expand it to one identity per sample.
///
/// The attempt must be `>= 1` and at most one step beyond the highest attempt
/// already recorded for that sample, so a retry appends `n + 1` instead of
/// creating a parallel identity.
pub fn plan_event_identity(
    run_id: &str,
    step_path: Option<&str>,
    sample_ids: &[String],
    attempt: Option<u32>,
    existing: &[ProgressEvent],
) -> Result<Vec<SampleExecutionIdentity>, Vec<ProgressIssue>> {
    let Some(attempt) = attempt else {
        return Ok(Vec::new());
    };
    let mut issues = Vec::new();
    let run_id = run_id.trim();
    if run_id.is_empty() {
        issues.push(issue("missing_run", "attempt 必须关联 Run", None, None, None));
    }
    let step_path = step_path.map(str::trim).filter(|path| !path.is_empty());
    let step_path = match step_path {
        Some(path) => Some(path),
        None => {
            issues.push(issue(
                "missing_step",
                "attempt 必须关联冻结的 Protocol 步骤",
                None,
                None,
                None,
            ));
            None
        }
    };
    let mut samples: Vec<&str> = Vec::new();
    for sample in sample_ids {
        let sample = sample.trim();
        if sample.is_empty() || samples.iter().any(|known| *known == sample) {
            continue;
        }
        samples.push(sample);
    }
    if samples.is_empty() {
        issues.push(issue(
            "missing_sample",
            "attempt 必须关联至少一个样本；步骤级事件不带样本时不应声明 attempt",
            step_path,
            None,
            None,
        ));
    }
    if attempt == 0 {
        issues.push(issue(
            "invalid_attempt",
            "attempt 必须从 1 开始",
            step_path,
            None,
            None,
        ));
    }
    if !issues.is_empty() {
        return Err(issues);
    }
    let step_path = step_path.expect("validated above");

    let resolved = resolve_events(existing, &mut Vec::new());
    let mut identities = Vec::with_capacity(samples.len());
    for sample in samples {
        let observed: Vec<u32> = resolved
            .iter()
            .filter(|event| {
                event
                    .event
                    .step_path
                    .as_deref()
                    .map(str::trim)
                    == Some(step_path)
                    && event
                        .event
                        .sample_ids
                        .iter()
                        .any(|id| id.trim() == sample)
            })
            .map(|event| event.attempt)
            .collect();
        let highest = observed.iter().copied().max().unwrap_or(0);
        if attempt > highest + 1 {
            issues.push(issue(
                "attempt_gap",
                &format!(
                    "样本 {sample} 的下一 attempt 只能是 {}，收到 {attempt}",
                    highest + 1
                ),
                Some(step_path),
                Some(sample),
                None,
            ));
            continue;
        }
        identities.push(SampleExecutionIdentity {
            run_id: run_id.to_owned(),
            step_path: step_path.to_owned(),
            sample_id: sample.to_owned(),
            attempt,
        });
    }
    if issues.is_empty() {
        Ok(identities)
    } else {
        Err(issues)
    }
}

fn str_field(value: &Value, key: &str) -> Option<String> {
    value[key]
        .as_str()
        .map(str::trim)
        .filter(|text| !text.is_empty())
        .map(str::to_owned)
}

fn retry_link(links: &[Value]) -> Option<String> {
    links.iter().find_map(|link| {
        let relation = link["relation"].as_str()?;
        if relation != "retry" || link["from_kind"].as_str() != Some("execution_event") {
            return None;
        }
        link["from_id"]
            .as_str()
            .map(str::trim)
            .filter(|id| !id.is_empty())
            .map(str::to_owned)
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn step(path: &str) -> String {
        path.to_owned()
    }

    fn event(id: &str, step: &str, samples: &[&str], at: &str) -> ProgressEvent {
        ProgressEvent {
            event_id: id.to_owned(),
            step_path: Some(step.to_owned()),
            sample_ids: samples.iter().map(|s| (*s).to_owned()).collect(),
            recorded_at: Some(at.to_owned()),
            ..ProgressEvent::default()
        }
    }

    fn status(event: ProgressEvent, status: &str) -> ProgressEvent {
        ProgressEvent {
            step_status: Some(status.to_owned()),
            ..event
        }
    }

    fn find<'a>(progress: &'a WorkflowProgress, step_path: &str) -> &'a StepProgress {
        progress
            .steps
            .iter()
            .find(|step| step.step_path == step_path)
            .expect("step must be present")
    }

    fn sample<'a>(step: &'a StepProgress, sample_id: &str) -> &'a SampleProgress {
        step.samples
            .iter()
            .find(|sample| sample.sample_id == sample_id)
            .expect("sample must be present")
    }

    #[test]
    fn sample_progress_keeps_retry_attempts_separate() {
        let first = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "blocked");
        let retry = status(
            ProgressEvent {
                retry_of_event_id: Some("e1".to_owned()),
                ..event("e2", "root/seed", &["S1"], "2026-11-13T11:00:00Z")
            },
            "completed",
        );

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[first, retry],
            &SampleProgressQuery::default(),
        );

        let step = find(&progress, "root/seed");
        let sample = sample(step, "S1");
        assert_eq!(sample.status, SampleStatus::Completed);
        assert_eq!(sample.attempt_count, 2);
        assert_eq!(sample.attempt, Some(2));
        let attempts: Vec<(u32, SampleStatus)> = sample
            .attempts
            .iter()
            .map(|attempt| (attempt.attempt, attempt.status))
            .collect();
        assert_eq!(
            attempts,
            vec![
                (1, SampleStatus::Failed),
                (2, SampleStatus::Completed)
            ]
        );
        assert_eq!(
            sample.attempts[0].event_ids,
            vec!["e1".to_owned()],
            "the failed attempt keeps its events"
        );
        assert_eq!(step.status, SampleStatus::Completed);
        assert!(step.complete);
    }

    #[test]
    fn sample_progress_does_not_report_step_complete_for_partial_samples() {
        let done = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "completed");

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[done],
            &SampleProgressQuery {
                sample_ids: vec!["S1".to_owned(), "S2".to_owned()],
                ..SampleProgressQuery::default()
            },
        );

        let step = find(&progress, "root/seed");
        assert_eq!(step.status, SampleStatus::InProgress);
        assert!(!step.complete, "one completed sample does not complete the step");
        assert_eq!(step.total_samples, 2);
        assert_eq!(step.completed_samples, 1);
        assert_eq!(step.not_started_samples, 1);
        assert_eq!(sample(step, "S2").status, SampleStatus::NotStarted);
        assert_eq!(sample(step, "S2").attempt, None);
        assert!(sample(step, "S2").attempts.is_empty());
    }

    #[test]
    fn sample_progress_marks_failed_sample_and_step() {
        let done = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "completed");
        let failed = status(event("e2", "root/seed", &["S2"], "2026-11-13T10:30:00Z"), "blocked");

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[done, failed],
            &SampleProgressQuery::default(),
        );

        let step = find(&progress, "root/seed");
        assert_eq!(step.status, SampleStatus::Failed);
        assert!(!step.complete);
        assert_eq!(step.completed_samples, 1);
        assert_eq!(step.failed_samples, 1);
        assert_eq!(sample(step, "S2").status, SampleStatus::Failed);
    }

    #[test]
    fn sample_progress_uses_last_event_time_per_step() {
        let early = status(
            event("e1", "root/readout", &["S1"], "2026-11-13T10:00:00Z"),
            "in_progress",
        );
        let late = status(
            event("e2", "root/readout", &["S2"], "2026-11-13T12:30:00Z"),
            "completed",
        );

        let progress = build(
            "run-1",
            &[step("root/readout")],
            &[early, late],
            &SampleProgressQuery::default(),
        );

        let step = find(&progress, "root/readout");
        assert_eq!(step.last_event_at.as_deref(), Some("2026-11-13T12:30:00Z"));
        assert_eq!(sample(step, "S1").status, SampleStatus::InProgress);
        assert_eq!(sample(step, "S1").last_event_at.as_deref(), Some("2026-11-13T10:00:00Z"));
    }

    #[test]
    fn sample_progress_reports_skipped_matrix_without_claiming_completion() {
        let skipped = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "skipped");
        let skipped_two = status(
            event("e2", "root/seed", &["S2"], "2026-11-13T10:05:00Z"),
            "skipped",
        );

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[skipped, skipped_two],
            &SampleProgressQuery::default(),
        );

        let step = find(&progress, "root/seed");
        assert_eq!(step.status, SampleStatus::Skipped);
        assert!(!step.complete, "a skipped step is not a completed step");
        assert_eq!(step.skipped_samples, 2);
    }

    #[test]
    fn sample_progress_honours_explicit_attempt_field() {
        let first = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "blocked");
        let second = status(
            ProgressEvent {
                attempt: Some(2),
                ..event("e2", "root/seed", &["S1"], "2026-11-13T11:00:00Z")
            },
            "completed",
        );

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[first, second],
            &SampleProgressQuery::default(),
        );

        let sample = sample(find(&progress, "root/seed"), "S1");
        assert_eq!(sample.attempt_count, 2);
        assert_eq!(sample.attempts[1].attempt, 2);
        assert_eq!(sample.status, SampleStatus::Completed);
    }

    #[test]
    fn sample_progress_rejects_attempt_zero_and_gaps() {
        let existing = vec![status(
            event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"),
            "blocked",
        )];
        let zero = plan_event_identity("run-1", Some("root/seed"), &["S1".to_owned()], Some(0), &existing)
            .expect_err("attempt 0 is not a valid identity");
        assert_eq!(zero[0].code, "invalid_attempt");

        let gap = plan_event_identity("run-1", Some("root/seed"), &["S1".to_owned()], Some(3), &existing)
            .expect_err("attempt 3 skips attempt 2");
        assert_eq!(gap[0].code, "attempt_gap");

        let ok = plan_event_identity("run-1", Some("root/seed"), &["S1".to_owned()], Some(2), &existing)
            .expect("next attempt is accepted");
        assert_eq!(
            ok,
            vec![SampleExecutionIdentity {
                run_id: "run-1".to_owned(),
                step_path: "root/seed".to_owned(),
                sample_id: "S1".to_owned(),
                attempt: 2,
            }]
        );

        let missing_sample =
            plan_event_identity("run-1", Some("root/seed"), &[], Some(1), &existing)
                .expect_err("attempt without a sample has no identity");
        assert_eq!(missing_sample[0].code, "missing_sample");
    }

    #[test]
    fn sample_progress_reports_unknown_step_events() {
        let known = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "completed");
        let unknown = status(event("e2", "root/gone", &["S1"], "2026-11-13T10:05:00Z"), "completed");

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[known, unknown],
            &SampleProgressQuery::default(),
        );

        assert_eq!(progress.steps.len(), 1);
        assert!(progress
            .issues
            .iter()
            .any(|issue| issue.code == "unknown_step" && issue.event_id.as_deref() == Some("e2")));
        assert_eq!(progress.sample_ids, vec!["S1".to_owned()]);
    }

    #[test]
    fn sample_progress_reads_stored_retry_lineage() {
        let first = status(event("e1", "root/seed", &["S1"], "2026-11-13T10:00:00Z"), "blocked");
        let second = status(
            ProgressEvent {
                retry_of_event_id: Some("e1".to_owned()),
                ..event("e2", "root/seed", &["S1"], "2026-11-13T11:00:00Z")
            },
            "completed",
        );
        let stored = json!({
            "id": "e2",
            "protocol_step_path": "root/seed",
            "sample_ids": ["S1"],
            "step_status": "completed",
            "lineage": [{
                "from_kind": "execution_event",
                "from_id": "e1",
                "to_kind": "step",
                "to_id": "root/seed",
                "relation": "retry"
            }]
        });
        let restored = ProgressEvent::from_stored(&stored);
        assert_eq!(restored.retry_of_event_id.as_deref(), Some("e1"));

        let progress = build(
            "run-1",
            &[step("root/seed")],
            &[first, second],
            &SampleProgressQuery::default(),
        );
        assert_eq!(sample(find(&progress, "root/seed"), "S1").attempt_count, 2);
    }
}

