//! Typed projection of a recursively expanded Protocol preview.
//!
//! This module does not load blocks or validate parameter bindings; the CLI
//! expansion produces these types. Keeping the shape here prevents adapters
//! from inventing their own step/parameter JSON.

use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ProtocolPreview {
    pub block_version_id: String,
    pub block_id: String,
    pub title: String,
    pub version: i64,
    pub ready: bool,
    pub steps: Vec<OperationStep>,
    pub resolved_parameters: Vec<ResolvedParameter>,
    pub unresolved_parameters: Vec<UnresolvedParameter>,
    pub block_version_ids: Vec<String>,
    #[serde(default)]
    pub module_contracts: Vec<Value>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum StepTimeScope {
    Once,
    PerTimepoint,
    PerAssay,
    Unresolved,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct OperationStep {
    pub path: String,
    pub use_id: String,
    pub title: String,
    pub instruction: String,
    pub parameters: Vec<ResolvedParameter>,
    pub unresolved_parameter_refs: Vec<String>,
    pub source_block_version_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub visualization: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub time_scope: Option<StepTimeScope>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub checks: Vec<StepCheck>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepCheck {
    pub check_id: String,
    pub label: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instruction: Option<String>,
    #[serde(default)]
    pub blocking: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResolvedParameter {
    /// Present for global resolved parameters; omitted for step parameters.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    pub slot: String,
    pub label: String,
    pub value: Value,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub provenance: Vec<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct UnresolvedParameter {
    pub path: String,
    pub slot: String,
    pub label: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ProtocolTimeBudget {
    pub total_span_s: f64,
    pub hands_on_s: f64,
    pub parallel_windows: Vec<ParallelWindow>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ParallelWindow {
    pub step_index: usize,
    pub start_s: f64,
    pub end_s: f64,
    pub duration_s: f64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub time_anchor_id: Option<String>,
}

/// Projects declared step times into a serial, deterministic budget. Wait and
/// timed-endpoint durations contribute to span, not hands-on time.
pub fn project_time_budget(
    specs: &[crate::operation_spec::OperationSpec],
) -> Result<ProtocolTimeBudget, String> {
    let mut budget = ProtocolTimeBudget {
        total_span_s: 0.0,
        hands_on_s: 0.0,
        parallel_windows: Vec::new(),
    };

    for (step_index, spec) in specs.iter().enumerate() {
        let repeat_count = spec.repeat.as_ref().map_or(1, |repeat| repeat.count);
        let schedule = spec.schedule.as_ref();
        let schedule_count = schedule.map_or(1, |rule| rule.occurrences);
        if repeat_count == 0 || schedule_count == 0 {
            return Err(format!("step[{step_index}] repetition count must be positive"));
        }
        let interval_s = if let Some(schedule) = schedule {
            if spec.time_anchor_id.as_deref().unwrap_or("").trim().is_empty() {
                return Err(format!("step[{step_index}] periodic schedule requires a time anchor"));
            }
            let interval = schedule.interval.seconds().ok_or_else(|| {
                format!("step[{step_index}] schedule interval must use a time unit")
            })?;
            if !interval.is_finite() || interval <= 0.0 {
                return Err(format!("step[{step_index}] schedule interval must be finite and positive"));
            }
            Some(interval)
        } else {
            None
        };
        let Some(timing) = &spec.timing else {
            if let Some(interval) = interval_s {
                budget.total_span_s += interval * f64::from(schedule_count - 1);
                if !budget.total_span_s.is_finite() {
                    return Err("projected time budget is not finite".into());
                }
            }
            continue;
        };

        let duration = timing
            .duration
            .seconds()
            .ok_or_else(|| format!("step[{step_index}] timing duration must use a time unit"))?;
        if !duration.is_finite() || duration <= 0.0 {
            return Err(format!(
                "step[{step_index}] timing duration must be finite and positive"
            ));
        }
        let occurrence_duration = duration * f64::from(repeat_count);
        if !occurrence_duration.is_finite() {
            return Err(format!("step[{step_index}] expanded duration is not finite"));
        }

        for occurrence in 0..schedule_count {
            let start_s = budget.total_span_s;
            budget.total_span_s += occurrence_duration;
            if timing.kind == crate::operation_spec::OperationTimeKind::HandsOn {
                budget.hands_on_s += occurrence_duration;
            } else {
                budget.parallel_windows.push(ParallelWindow {
                    step_index,
                    start_s,
                    end_s: budget.total_span_s,
                    duration_s: occurrence_duration,
                    time_anchor_id: spec.time_anchor_id.clone(),
                });
            }
            if occurrence + 1 < schedule_count {
                budget.total_span_s += interval_s.unwrap_or(0.0);
            }
        }
        if !budget.total_span_s.is_finite() || !budget.hands_on_s.is_finite() {
            return Err("projected time budget is not finite".into());
        }
    }
    Ok(budget)
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn time_budget_separates_hands_on_span_and_parallel_wait_windows() {
        use crate::operation_spec::OperationSpec;

        let specs: Vec<OperationSpec> = serde_json::from_value(json!([
            {
                "schema_version": 2,
                "operation": { "kind": "measure", "endpoint_id": "capture", "unit": "count" },
                "timing": { "kind": "hands_on", "duration": { "value": 30.0, "unit": "s" } },
                "repeat": { "count": 2 }
            },
            {
                "schema_version": 2,
                "operation": { "kind": "incubate", "temperature": { "value": 4.0, "unit": "C" },
                    "duration": { "value": 2.0, "unit": "h" } },
                "timing": { "kind": "wait", "duration": { "value": 2.0, "unit": "h" } },
                "time_anchor_id": "cold_start"
            },
            {
                "schema_version": 2,
                "operation": { "kind": "measure", "endpoint_id": "readout", "unit": "AU" },
                "timing": { "kind": "timed_endpoint", "duration": { "value": 10.0, "unit": "s" } },
                "schedule": { "interval": { "value": 1.0, "unit": "d" }, "occurrences": 3 },
                "time_anchor_id": "t0"
            }
        ]))
        .unwrap();

        let budget = project_time_budget(&specs).unwrap();
        assert_eq!(budget.hands_on_s, 60.0);
        assert_eq!(budget.total_span_s, 180_090.0);
        assert_eq!(budget.parallel_windows.len(), 4);
        assert_eq!(
            budget.parallel_windows[1..]
                .iter()
                .map(|window| (window.start_s, window.end_s, window.duration_s))
                .collect::<Vec<_>>(),
            [(7_260.0, 7_270.0, 10.0), (93_670.0, 93_680.0, 10.0), (180_080.0, 180_090.0, 10.0)]
        );
    }

    #[test]
    fn step_parameter_omits_path_but_global_keeps_it() {
        let step_param = ResolvedParameter {
            path: None,
            slot: "wavelength".into(),
            label: "波长".into(),
            value: json!(450),
            unit: Some("nm".into()),
            provenance: vec![],
        };
        let global = ResolvedParameter {
            path: Some("root/read_od/wavelength".into()),
            ..step_param.clone()
        };
        let step_json = serde_json::to_value(&step_param).unwrap();
        let global_json = serde_json::to_value(&global).unwrap();
        assert!(!step_json.as_object().unwrap().contains_key("path"));
        assert_eq!(global_json["path"], "root/read_od/wavelength");
    }

    #[test]
    fn visualization_is_omitted_when_absent() {
        let step = OperationStep {
            path: "root/a".into(),
            use_id: "a".into(),
            title: "A".into(),
            instruction: "do".into(),
            parameters: vec![],
            unresolved_parameter_refs: vec![],
            source_block_version_id: "v".into(),
            visualization: None,
            time_scope: None,
            checks: vec![],
        };
        assert!(!serde_json::to_value(step)
            .unwrap()
            .as_object()
            .unwrap()
            .contains_key("visualization"));
    }
}
