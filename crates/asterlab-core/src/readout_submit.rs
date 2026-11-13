//! `readout.submit` / `readout.batch` write planning.
//!
//! A single readout and a table/plate batch both map onto one frozen
//! `execution_event.create` payload, so recording a readout never needs a new
//! record kind or a second fact table:
//!
//! - `readout.submit` builds one `measurement` event with one `readouts[]`
//!   entry (and one derived `measurements[]` entry when a number was recorded).
//! - `readout.batch` reuses [`crate::reading_facts::plan_reading_batch`], which
//!   validates every cell against this Run's plate and emits a single event.
//!
//! Both planners are pure: they validate, compute a submission idempotency key
//! and return the payload. The CLI wraps the actual insert in one transaction,
//! so a rejected row leaves no partial event behind.
//!
//! # Idempotency
//!
//! `readout.submit` keys on the identity tuple from the SDK-05 contract:
//! `(run_id, step_path, endpoint, sample_id, timepoint)`. `readout.batch` keys
//! on the same tuple per cell plus the cell position (`plate_id, row, column`),
//! because several wells of one plate legitimately share endpoint, sample and
//! timepoint. Recorded numbers are content, not identity: a replayed
//! submission returns the first event instead of writing a second one.
//! `run_id` is part of every key, so identical readouts in two Runs stay
//! separate records.

use crate::reading_facts::{plan_reading_batch, ReadingBatch, ReadingCellInput, ReadingIssue};
use crate::run_workspace::RunWorkspace;
use crate::readout_schema::MissingReason;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

/// One `readout.submit` request.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutSubmitRequest {
    pub run_id: String,
    pub step_path: String,
    pub endpoint: String,
    pub protocol_version_id: String,
    #[serde(default)]
    pub assay_id: Option<String>,
    #[serde(default)]
    pub planned_output_id: Option<String>,
    #[serde(default)]
    pub timepoint: Option<String>,
    #[serde(default)]
    pub sample_id: Option<String>,
    /// Exactly one of `value` / `qualifier` is recorded.
    #[serde(default)]
    pub value: Option<f64>,
    #[serde(default)]
    pub qualifier: Option<MissingReason>,
    #[serde(default)]
    pub unit: Option<String>,
    #[serde(default)]
    pub content: Option<String>,
    #[serde(default)]
    pub occurred_at: Option<String>,
    #[serde(default)]
    pub operator: Option<String>,
}

/// One `readout.batch` request. Exactly one of `cells` / `csv` must be given.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutBatchRequest {
    pub run_id: String,
    pub step_path: String,
    pub protocol_version_id: String,
    pub plate_id: String,
    pub schema_id: String,
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    #[serde(default)]
    pub assay_id: Option<String>,
    #[serde(default)]
    pub timepoint: Option<String>,
    #[serde(default)]
    pub batch_id: Option<String>,
    #[serde(default)]
    pub content: Option<String>,
    #[serde(default)]
    pub occurred_at: Option<String>,
    #[serde(default)]
    pub operator: Option<String>,
    #[serde(default)]
    pub cells: Vec<ReadingCellInput>,
    #[serde(default)]
    pub csv: Option<String>,
}

fn default_schema_version() -> u32 {
    1
}

/// A validated submission: the exact `execution_event.create` payload, the
/// idempotency key that payload is stored under, and how many readouts it
/// records.
#[derive(Debug, Clone, PartialEq)]
pub struct ReadoutPlan {
    pub event: Value,
    pub idempotency_key: String,
    pub readout_count: usize,
}

fn issue(code: &str, message: impl Into<String>) -> ReadingIssue {
    ReadingIssue {
        code: code.to_owned(),
        message: message.into(),
        event_id: None,
        step_path: None,
        target: None,
    }
}

fn blank(value: &str) -> bool {
    value.trim().is_empty()
}

fn optional_text(value: &Option<String>) -> Option<String> {
    value
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
}

/// Stable, namespaced submission key over the canonical JSON of `parts`.
fn submission_key(scope: &str, parts: &Value) -> String {
    let mut hasher = Sha256::new();
    hasher.update(serde_json::to_vec(parts).unwrap_or_default());
    let digest: String = hasher
        .finalize()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    format!("readout:{scope}:{digest}")
}

fn check_scope(
    run_id: &str,
    step_path: &str,
    protocol_version_id: &str,
    workspace: &RunWorkspace,
    issues: &mut Vec<ReadingIssue>,
) {
    if blank(run_id) {
        issues.push(issue("missing_run", "缺少 run_id"));
    } else if run_id.trim() != workspace.run_id {
        issues.push(issue(
            "wrong_run",
            format!("run_id {} 与当前 Run 不一致", run_id.trim()),
        ));
    }
    if blank(step_path) {
        issues.push(issue("missing_step", "缺少 Protocol 步骤路径"));
    } else if !workspace
        .steps
        .iter()
        .any(|step| step.step.path.trim() == step_path.trim())
    {
        issues.push(issue(
            "unknown_step",
            format!("步骤 {} 不属于本次 Run 的冻结清单", step_path.trim()),
        ));
    }
    if blank(protocol_version_id) {
        issues.push(issue(
            "missing_protocol_version",
            "关联 Protocol 步骤时必须提供 protocol_version_id",
        ));
    }
}

/// Validate one readout and build the `execution_event.create` payload it
/// appends. Structure follows the frozen `measurements[]` / `readouts[]` form:
/// the number lands in both, an explicit missing reason only in `readouts[]`.
pub fn plan_submit(
    request: &ReadoutSubmitRequest,
    workspace: &RunWorkspace,
) -> Result<ReadoutPlan, Vec<ReadingIssue>> {
    let mut issues = Vec::new();
    check_scope(
        &request.run_id,
        &request.step_path,
        &request.protocol_version_id,
        workspace,
        &mut issues,
    );

    let endpoint = request.endpoint.trim();
    if endpoint.is_empty() {
        issues.push(issue("missing_endpoint", "读数缺少 endpoint"));
    } else if endpoint.chars().count() > 300 {
        issues.push(issue("invalid_endpoint", "endpoint 不能超过 300 个字符"));
    }
    check_optional_len(request.unit.as_deref(), 40, "invalid_unit", "单位", &mut issues);
    check_optional_len(
        request.sample_id.as_deref(),
        120,
        "invalid_sample",
        "样本编号",
        &mut issues,
    );
    if request.value.is_some() == request.qualifier.is_some() {
        issues.push(issue(
            "readout_value_or_qualifier",
            "必须且只能填写 value 或 qualifier 之一",
        ));
    }
    if request.value.is_some_and(|value| !value.is_finite()) {
        issues.push(issue("non_finite_value", "读数必须是有限数值"));
    }
    if request.timepoint.is_some() && request.assay_id.is_none() {
        issues.push(issue(
            "timepoint_without_assay",
            "时间点必须同时给出检测；Core 不会按名称猜检测",
        ));
    }
    if request.assay_id.is_some() && request.planned_output_id.is_some() {
        issues.push(issue(
            "ambiguous_scope",
            "读数只能关联一个检测或一个共享计划产物",
        ));
    }
    if !issues.is_empty() {
        return Err(issues);
    }

    let sample_id = optional_text(&request.sample_id);
    let qualifier = request.qualifier.map(|reason| {
        serde_json::to_value(reason).unwrap_or(Value::Null)
    });
    let mut readout = json!({
        "endpoint": endpoint,
        "unit": request.unit,
        "sample_id": sample_id,
        "result_kind": "raw",
        "derived_from": [],
    });
    if let Some(value) = request.value {
        readout["value"] = json!(value);
    }
    if let Some(qualifier) = qualifier {
        readout["qualifier"] = qualifier;
    }
    let measurements = match request.value {
        Some(value) => vec![json!({
            "endpoint": endpoint,
            "value": value,
            "unit": request.unit,
            "sample_id": sample_id,
            "status": "raw",
            "field": endpoint,
        })],
        None => Vec::new(),
    };

    let idempotency_key = submission_key(
        "single",
        &json!([
            request.run_id.trim(),
            request.step_path.trim(),
            endpoint,
            sample_id,
            request.timepoint,
        ]),
    );
    let event = json!({
        "run_id": request.run_id.trim(),
        "event_type": "measurement",
        "content": request
            .content
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned)
            .unwrap_or_else(|| format!("读数提交：{endpoint}")),
        "occurred_at": request.occurred_at,
        "timepoint": request.timepoint,
        "protocol_step_path": request.step_path.trim(),
        "protocol_version_id": request.protocol_version_id.trim(),
        "assay_id": request.assay_id,
        "planned_output_id": request.planned_output_id,
        "operator": request.operator,
        "measurements": measurements,
        "readouts": [readout],
        "operation_id": idempotency_key,
        "operation_payload": serde_json::to_value(request).unwrap_or(Value::Null),
    });

    Ok(ReadoutPlan {
        event,
        idempotency_key,
        readout_count: 1,
    })
}

/// Validate a table/plate batch and build the one event it appends. Every cell
/// is checked before a payload exists, so a bad row produces no event at all.
pub fn plan_batch(
    request: &ReadoutBatchRequest,
    workspace: &RunWorkspace,
) -> Result<ReadoutPlan, Vec<ReadingIssue>> {
    let mut issues = Vec::new();
    check_scope(
        &request.run_id,
        &request.step_path,
        &request.protocol_version_id,
        workspace,
        &mut issues,
    );
    if blank(&request.plate_id) {
        issues.push(issue(
            "missing_plate",
            "批次缺少孔板 ID；孔位坐标在孔板内才唯一",
        ));
    }
    if blank(&request.schema_id) {
        issues.push(issue("missing_schema", "批次缺少结果 schema_id"));
    }
    if request.schema_version != 1 {
        issues.push(issue(
            "invalid_schema_version",
            format!("schema_version={}，当前只接受 1", request.schema_version),
        ));
    }
    let csv = request
        .csv
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty());
    let cells = match (&request.cells[..], csv) {
        (_, Some(_)) if !request.cells.is_empty() => {
            issues.push(issue(
                "ambiguous_cells",
                "cells 与 csv 必须且只能提供一种",
            ));
            Vec::new()
        }
        (cells, _) if !cells.is_empty() => cells.to_vec(),
        (_, Some(text)) => match parse_csv_cells(text) {
            Ok(cells) => cells,
            Err(csv_issues) => {
                issues.extend(csv_issues);
                Vec::new()
            }
        },
        _ => {
            issues.push(issue("empty_batch", "批次至少需要一个读数单元或 csv"));
            Vec::new()
        }
    };
    if !issues.is_empty() {
        return Err(issues);
    }

    let batch = ReadingBatch {
        run_id: request.run_id.trim().to_owned(),
        step_path: request.step_path.trim().to_owned(),
        plate_id: request.plate_id.trim().to_owned(),
        schema_id: request.schema_id.trim().to_owned(),
        schema_version: request.schema_version,
        assay_id: optional_text(&request.assay_id),
        timepoint: optional_text(&request.timepoint),
        batch_id: optional_text(&request.batch_id),
        operation_id: None,
        cells: cells.clone(),
    };
    let plan = plan_reading_batch(&batch, workspace)?;

    let mut identities: Vec<Value> = cells
        .iter()
        .map(|cell| {
            json!([
                request.plate_id.trim(),
                cell.row,
                cell.column,
                cell.field.trim(),
                optional_text(&cell.sample_id),
            ])
        })
        .collect();
    identities.sort_by_key(|value| value.to_string());
    let idempotency_key = submission_key(
        "batch",
        &json!([
            request.run_id.trim(),
            request.step_path.trim(),
            optional_text(&request.assay_id),
            optional_text(&request.timepoint),
            identities,
        ]),
    );

    let mut event = plan.event;
    event["protocol_version_id"] = json!(request.protocol_version_id.trim());
    event["operator"] = json!(request.operator);
    event["occurred_at"] = json!(request.occurred_at);
    event["operation_id"] = json!(idempotency_key);
    event["operation_payload"] = serde_json::to_value(request).unwrap_or(Value::Null);
    if let Some(content) = request
        .content
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        event["content"] = json!(content);
    }

    Ok(ReadoutPlan {
        event,
        idempotency_key,
        readout_count: cells.len(),
    })
}

fn check_optional_len(
    value: Option<&str>,
    max: usize,
    code: &str,
    label: &str,
    issues: &mut Vec<ReadingIssue>,
) {
    if value.is_some_and(|value| value.chars().count() > max) {
        issues.push(issue(code, format!("{label}不能超过 {max} 个字符")));
    }
}

/// Parse the `cells` CSV form used by `readout.batch`.
///
/// Header names are case-insensitive: `row`, `column`, `field` are required;
/// `value`, `unit`, `sample_id`, `group`, `replicate`, `status` are optional.
/// Rows are 0-based, matching `reading_facts::ReadingCellInput`. Unknown or
/// duplicated headers are rejected instead of ignored.
pub fn parse_csv_cells(csv: &str) -> Result<Vec<ReadingCellInput>, Vec<ReadingIssue>> {
    let records = split_csv(csv);
    let mut issues = Vec::new();
    let mut records = records.into_iter().filter(|record| {
        !(record.len() == 1 && record[0].trim().is_empty())
    });
    let Some(header) = records.next() else {
        return Err(vec![issue("empty_csv", "csv 至少需要表头和一行读数")]);
    };
    let header: Vec<String> = header
        .iter()
        .map(|name| name.trim().trim_start_matches('\u{feff}').to_lowercase())
        .collect();
    let mut columns = std::collections::HashMap::new();
    for (index, name) in header.iter().enumerate() {
        if !matches!(
            name.as_str(),
            "row"
                | "column"
                | "field"
                | "value"
                | "unit"
                | "sample_id"
                | "group"
                | "replicate"
                | "status"
        ) {
            issues.push(issue(
                "csv_unknown_column",
                format!("csv 列 {name} 不受支持"),
            ));
            continue;
        }
        if columns.insert(name.clone(), index).is_some() {
            issues.push(issue(
                "csv_duplicate_column",
                format!("csv 列 {name} 重复"),
            ));
        }
    }
    for required in ["row", "column", "field"] {
        if !columns.contains_key(required) {
            issues.push(issue(
                "csv_missing_header",
                format!("csv 缺少必需列 {required}"),
            ));
        }
    }
    if !issues.is_empty() {
        return Err(issues);
    }

    let field_of = |record: &[String], name: &str| -> Option<String> {
        columns.get(name).and_then(|index| record.get(*index)).and_then(|value| {
            let value = value.trim();
            if value.is_empty() {
                None
            } else {
                Some(value.to_owned())
            }
        })
    };
    let mut cells = Vec::new();
    for (offset, record) in records.enumerate() {
        let line = offset + 2;
        if record.len() != header.len() {
            issues.push(issue(
                "csv_row_width",
                format!("csv 第 {line} 行有 {} 列，表头有 {} 列", record.len(), header.len()),
            ));
            continue;
        }
        let row = match field_of(&record, "row").map(|value| value.parse::<u32>()) {
            Some(Ok(row)) => row,
            _ => {
                issues.push(issue(
                    "csv_invalid_row",
                    format!("csv 第 {line} 行 row 必须是非负整数"),
                ));
                continue;
            }
        };
        let column = match field_of(&record, "column").map(|value| value.parse::<u32>()) {
            Some(Ok(column)) => column,
            _ => {
                issues.push(issue(
                    "csv_invalid_column",
                    format!("csv 第 {line} 行 column 必须是非负整数"),
                ));
                continue;
            }
        };
        let field = field_of(&record, "field").unwrap_or_default();
        let value = match field_of(&record, "value") {
            Some(raw) => match raw.parse::<f64>() {
                Ok(value) if value.is_finite() => Some(value),
                _ => {
                    issues.push(issue(
                        "csv_invalid_value",
                        format!("csv 第 {line} 行 value 必须是有限数值"),
                    ));
                    continue;
                }
            },
            None => None,
        };
        let replicate = match field_of(&record, "replicate") {
            Some(raw) => match raw.parse::<u32>() {
                Ok(replicate) => Some(replicate),
                Err(_) => {
                    issues.push(issue(
                        "csv_invalid_replicate",
                        format!("csv 第 {line} 行 replicate 必须是非负整数"),
                    ));
                    continue;
                }
            },
            None => None,
        };
        cells.push(ReadingCellInput {
            row,
            column,
            field,
            value,
            unit: field_of(&record, "unit"),
            sample_id: field_of(&record, "sample_id"),
            group: field_of(&record, "group"),
            replicate,
            status: field_of(&record, "status"),
        });
    }
    if issues.is_empty() && cells.is_empty() {
        issues.push(issue("empty_csv", "csv 表头之后没有读数行"));
    }
    if !issues.is_empty() {
        return Err(issues);
    }
    Ok(cells)
}

/// RFC 4180-style record split: commas separate fields, `""` escapes a quote,
/// CRLF and LF end records.
fn split_csv(csv: &str) -> Vec<Vec<String>> {
    let mut records = Vec::new();
    let mut record = Vec::new();
    let mut field = String::new();
    let mut quoted = false;
    let mut chars = csv.chars().peekable();
    while let Some(ch) = chars.next() {
        if quoted {
            if ch == '"' {
                if chars.peek() == Some(&'"') {
                    chars.next();
                    field.push('"');
                } else {
                    quoted = false;
                }
            } else {
                field.push(ch);
            }
            continue;
        }
        match ch {
            '"' => quoted = true,
            ',' => record.push(std::mem::take(&mut field)),
            '\r' => {
                if chars.peek() == Some(&'\n') {
                    chars.next();
                }
                record.push(std::mem::take(&mut field));
                records.push(std::mem::take(&mut record));
            }
            '\n' => {
                record.push(std::mem::take(&mut field));
                records.push(std::mem::take(&mut record));
            }
            _ => field.push(ch),
        }
    }
    if quoted || !field.is_empty() || !record.is_empty() {
        record.push(field);
        records.push(record);
    }
    records
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::run_workspace::RunWorkspace;
    use serde_json::json;

    fn design() -> Value {
        json!({
            "title":"d","objective":"o","experimental_unit":"well",
            "group_specs":[],
            "assays":[{"id":"cck8","title":"CCK-8","timepoints":["D1"],
                "group_ids":[],"timepoint_relation":"same_physical_sample_repeated"}],
            "sample_allocations":[],"planned_outputs":[],"evidence_conflicts":[]
        })
    }

    fn run() -> Value {
        json!({
            "status":"in_progress",
            "protocol_manifest":[{
                "assay_id":"cck8","assay_title":"CCK-8","protocol_version_id":"pv1",
                "steps":[{"path":"root/read","use_id":"read","title":"Read","instruction":"read",
                    "parameters":[],"unresolved_parameter_refs":[],
                    "source_block_version_id":"b1","time_scope":"per_timepoint"}]
            }],
            "material_manifest":[]
        })
    }

    fn plate() -> Value {
        json!({"id":"plate-96","name":"96 孔板","format":"96","rows":8,"columns":12,
            "assay_id":"cck8","timepoint":"D1","wells":[]})
    }

    fn ws(run_id: &str) -> RunWorkspace {
        crate::run_workspace::build(run_id, "d1", "dv1", 1, &design(), &run(), &[plate()], &[])
            .unwrap()
    }

    fn submit(value: f64) -> ReadoutSubmitRequest {
        ReadoutSubmitRequest {
            run_id: "r1".to_owned(),
            step_path: "root/read".to_owned(),
            endpoint: "od450".to_owned(),
            protocol_version_id: "pv1".to_owned(),
            assay_id: Some("cck8".to_owned()),
            planned_output_id: None,
            timepoint: Some("D1".to_owned()),
            sample_id: Some("S-1".to_owned()),
            value: Some(value),
            qualifier: None,
            unit: Some("AU".to_owned()),
            content: None,
            occurred_at: None,
            operator: Some("tester".to_owned()),
        }
    }

    fn cell(row: u32, column: u32, value: Option<f64>) -> ReadingCellInput {
        ReadingCellInput {
            row,
            column,
            field: "od450".to_owned(),
            value,
            unit: Some("AU".to_owned()),
            sample_id: Some("S-1".to_owned()),
            group: Some("g1".to_owned()),
            replicate: Some(1),
            status: None,
        }
    }

    fn batch(cells: Vec<ReadingCellInput>) -> ReadoutBatchRequest {
        ReadoutBatchRequest {
            run_id: "r1".to_owned(),
            step_path: "root/read".to_owned(),
            protocol_version_id: "pv1".to_owned(),
            plate_id: "plate-96".to_owned(),
            schema_id: "cck8.plate96.od450.v1".to_owned(),
            schema_version: 1,
            assay_id: Some("cck8".to_owned()),
            timepoint: Some("D1".to_owned()),
            batch_id: Some("batch-1".to_owned()),
            content: None,
            occurred_at: None,
            operator: None,
            cells,
            csv: None,
        }
    }

    #[test]
    fn readout_submit_maps_a_single_readout_to_measurement_and_readouts() {
        let workspace = ws("r1");
        let plan = plan_submit(&submit(1.234), &workspace).unwrap();
        assert_eq!(plan.event["event_type"], "measurement");
        assert_eq!(plan.event["protocol_step_path"], "root/read");
        assert_eq!(plan.event["readouts"][0]["endpoint"], "od450");
        assert_eq!(plan.event["readouts"][0]["value"], 1.234);
        assert_eq!(plan.event["readouts"][0]["unit"], "AU");
        assert_eq!(plan.event["readouts"][0]["sample_id"], "S-1");
        assert_eq!(plan.event["readouts"][0]["result_kind"], "raw");
        assert_eq!(plan.event["measurements"][0]["value"], 1.234);
        assert_eq!(plan.event["measurements"][0]["field"], "od450");
        assert_eq!(plan.event["operation_id"], plan.idempotency_key.as_str());
        assert_eq!(plan.readout_count, 1);
    }

    #[test]
    fn readout_submit_keeps_missing_reasons_out_of_measurements() {
        let workspace = ws("r1");
        let mut request = submit(0.0);
        request.value = None;
        request.qualifier = Some(MissingReason::AboveUloq);
        let plan = plan_submit(&request, &workspace).unwrap();
        assert_eq!(plan.event["readouts"][0]["qualifier"], "above_uloq");
        assert!(plan.event["readouts"][0].get("value").is_none());
        assert!(plan.event["measurements"].as_array().unwrap().is_empty());
    }

    #[test]
    fn readout_submit_key_is_stable_and_run_scoped() {
        let workspace = ws("r1");
        let first = plan_submit(&submit(1.0), &workspace).unwrap();
        let second = plan_submit(&submit(2.5), &workspace).unwrap();
        assert_eq!(first.idempotency_key, second.idempotency_key);

        let other = ws("r2");
        let mut request = submit(1.0);
        request.run_id = "r2".to_owned();
        let elsewhere = plan_submit(&request, &other).unwrap();
        assert_ne!(first.idempotency_key, elsewhere.idempotency_key);

        request.run_id = "r3".to_owned();
        let issues = plan_submit(&request, &other).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "wrong_run"));
    }

    #[test]
    fn readout_submit_rejects_ambiguous_values_and_unknown_steps() {
        let workspace = ws("r1");
        let mut request = submit(1.0);
        request.qualifier = Some(MissingReason::Invalid);
        let issues = plan_submit(&request, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "readout_value_or_qualifier"));

        let mut request = submit(1.0);
        request.value = None;
        let issues = plan_submit(&request, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "readout_value_or_qualifier"));

        let mut request = submit(1.0);
        request.step_path = "root/other".to_owned();
        let issues = plan_submit(&request, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "unknown_step"));

        let mut request = submit(1.0);
        request.timepoint = Some("D1".to_owned());
        request.assay_id = None;
        let issues = plan_submit(&request, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "timepoint_without_assay"));
    }

    #[test]
    fn readout_submit_batch_parses_csv_and_json_to_the_same_plan() {
        let workspace = ws("r1");
        let json_plan = plan_batch(&batch(vec![cell(0, 0, Some(1.25)), cell(0, 1, None)]), &workspace)
            .unwrap();
        let mut csv_request = batch(Vec::new());
        csv_request.csv = Some(
            "row,column,field,value,unit,sample_id,group,replicate\n\
             0,0,od450,1.25,AU,S-1,g1,1\n\
             0,1,od450,,AU,S-1,g1,1\n"
                .to_owned(),
        );
        let csv_plan = plan_batch(&csv_request, &workspace).unwrap();
        assert_eq!(json_plan.event["result_cells"], csv_plan.event["result_cells"]);
        assert_eq!(json_plan.idempotency_key, csv_plan.idempotency_key);
        assert_eq!(csv_plan.readout_count, 2);
        assert_eq!(csv_plan.event["schema_id"], "cck8.plate96.od450.v1");
        assert_eq!(csv_plan.event["protocol_version_id"], "pv1");
    }

    #[test]
    fn readout_submit_batch_rejects_bad_rows_before_an_event_exists() {
        let workspace = ws("r1");
        let mut duplicate = batch(vec![cell(0, 0, Some(1.0)), cell(0, 0, Some(2.0))]);
        duplicate.batch_id = None;
        let issues = plan_batch(&duplicate, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "duplicate_position"));

        let mut mismatch = batch(vec![cell(0, 0, Some(1.0))]);
        mismatch.cells[0].status = Some("missing".to_owned());
        let issues = plan_batch(&mismatch, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "status_value_mismatch"));

        let mut both = batch(vec![cell(0, 0, Some(1.0))]);
        both.csv = Some("row,column,field\n0,0,od450\n".to_owned());
        let issues = plan_batch(&both, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "ambiguous_cells"));
    }

    #[test]
    fn readout_submit_batch_key_covers_cell_identity_not_values() {
        let workspace = ws("r1");
        let first = plan_batch(&batch(vec![cell(0, 0, Some(1.0)), cell(1, 0, Some(2.0))]), &workspace)
            .unwrap();
        let corrected = plan_batch(&batch(vec![cell(0, 0, Some(9.0)), cell(1, 0, Some(2.0))]), &workspace)
            .unwrap();
        assert_eq!(first.idempotency_key, corrected.idempotency_key);

        let different_well = plan_batch(&batch(vec![cell(0, 0, Some(1.0)), cell(2, 0, Some(2.0))]), &workspace)
            .unwrap();
        assert_ne!(first.idempotency_key, different_well.idempotency_key);

        let other = ws("r2");
        let mut request = batch(vec![cell(0, 0, Some(1.0)), cell(1, 0, Some(2.0))]);
        request.run_id = "r2".to_owned();
        let elsewhere = plan_batch(&request, &other).unwrap();
        assert_ne!(first.idempotency_key, elsewhere.idempotency_key);
    }

    #[test]
    fn readout_submit_csv_rejects_unknown_headers_and_short_rows() {
        let issues = parse_csv_cells("row,column,field,sensitivty\n0,0,od450,1\n").unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "csv_unknown_column"));

        let issues = parse_csv_cells("row,column,field,value\n0,0,od450\n").unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "csv_row_width"));

        let issues = parse_csv_cells("row,column,field,value\n0,0,od450,abc\n").unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "csv_invalid_value"));

        let cells = parse_csv_cells("\u{feff}row,column,field,value\n0,0,\"od,450\",1.5\n").unwrap();
        assert_eq!(cells[0].field, "od,450");
        assert_eq!(cells[0].value, Some(1.5));
    }
}
