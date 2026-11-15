//! Unified reading/observation facts and their single query.
//!
//! Facts are a read model derived from execution events; they are never written
//! back as a second store. A fact is keyed by `run + step + target + endpoint`,
//! so the result table and the plate view both consume the same
//! [`ReadingQueryResult`] instead of maintaining local copies.

use crate::run_workspace::{ExecutionEventView, RunPlateView, RunWorkspace};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::collections::{HashMap, HashSet};

/// Schema version of the fact read model. Bump only for a breaking field change.
pub const READING_FACT_SCHEMA_VERSION: u32 = 1;

/// Where a reading was taken.
///
/// A well coordinate is only unique inside one plate, so `Well` always carries
/// the plate id. Readings that name a sample or experimental unit but no
/// container position stay on the unit axis.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ReadingTarget {
    Well { plate_id: String, coordinate: String },
    Unit { unit_id: String },
}

/// The identity of one reading axis: one step, one endpoint, one target.
#[derive(Debug, Clone, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingKey {
    pub step_path: String,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub endpoint: String,
    pub target: ReadingTarget,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReadingStatus {
    Raw,
    Missing,
}

/// One raw reading. `value` stays `None` for an explicitly missing cell; a
/// missing reading is never coerced to `0`.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingFact {
    pub run_id: String,
    pub step_path: String,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub endpoint: String,
    pub target: ReadingTarget,
    pub value: Option<f64>,
    pub unit: Option<String>,
    pub status: ReadingStatus,
    pub source_event_id: String,
    /// 1-based revision of this key. A re-recorded value increments it.
    pub revision: u32,
    /// Event whose fact this revision replaced, `None` for revision 1.
    pub supersedes_event_id: Option<String>,
    pub recorded_at: Option<String>,
}

impl ReadingFact {
    pub fn key(&self) -> ReadingKey {
        ReadingKey {
            step_path: self.step_path.clone(),
            assay_id: self.assay_id.clone(),
            timepoint: self.timepoint.clone(),
            endpoint: self.endpoint.clone(),
            target: self.target.clone(),
        }
    }
}

/// A later fact replaced an earlier one. The earlier event is kept; nothing is
/// mutated. `replacement_event_id` is `None` while a write plan is still
/// pending and `Some` once the replacement event is recorded.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingOverwrite {
    pub key: ReadingKey,
    pub previous_event_id: String,
    pub previous_revision: u32,
    pub replacement_event_id: Option<String>,
    pub replacement_revision: u32,
    pub recorded_at: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingIssue {
    pub code: String,
    pub message: String,
    pub event_id: Option<String>,
    pub step_path: Option<String>,
    pub target: Option<ReadingTarget>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default, deny_unknown_fields)]
pub struct ReadingQuery {
    pub step_path: Option<String>,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub endpoint: Option<String>,
    pub plate_id: Option<String>,
    /// Keep superseded revisions in `history`.
    pub include_history: bool,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingQueryResult {
    pub schema_version: u32,
    pub run_id: String,
    /// Latest revision per key.
    pub facts: Vec<ReadingFact>,
    /// Every matching revision in event order; superseded facts only when the
    /// request asked for history.
    pub history: Vec<ReadingFact>,
    pub overwrites: Vec<ReadingOverwrite>,
    pub issues: Vec<ReadingIssue>,
}

/// One cell rendered by either projection. Both projections produce the exact
/// same cells from the same query result.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingTableCell {
    pub coordinate: String,
    pub row: u32,
    pub column: u32,
    pub endpoint: String,
    pub value: Option<f64>,
    pub unit: Option<String>,
    pub status: ReadingStatus,
    pub source_event_id: String,
    pub revision: u32,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingTable {
    pub plate_id: String,
    pub rows: u32,
    pub columns: u32,
    pub cells: Vec<ReadingTableCell>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingPlateWell {
    pub coordinate: String,
    pub row: u32,
    pub column: u32,
    pub cells: Vec<ReadingTableCell>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingPlate {
    pub plate_id: String,
    pub rows: u32,
    pub columns: u32,
    pub wells: Vec<ReadingPlateWell>,
}

/// One cell of a batch write. Blank stays explicit: `status = "missing"` with
/// no value.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingCellInput {
    pub row: u32,
    pub column: u32,
    pub field: String,
    pub value: Option<f64>,
    #[serde(default)]
    pub unit: Option<String>,
    #[serde(default)]
    pub sample_id: Option<String>,
    #[serde(default)]
    pub group: Option<String>,
    #[serde(default)]
    pub replicate: Option<u32>,
    #[serde(default)]
    pub status: Option<String>,
}

/// A readout batch. Saving it appends exactly one execution event and never
/// mutates an earlier event, so a same-well overwrite becomes a new event.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingBatch {
    pub run_id: String,
    pub step_path: String,
    pub plate_id: String,
    pub schema_id: String,
    pub schema_version: u32,
    #[serde(default)]
    pub assay_id: Option<String>,
    #[serde(default)]
    pub timepoint: Option<String>,
    #[serde(default)]
    pub batch_id: Option<String>,
    #[serde(default)]
    pub operation_id: Option<String>,
    pub cells: Vec<ReadingCellInput>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadingBatchPlan {
    /// Payload for the single `execution_event.create` call.
    pub event: Value,
    /// Facts this event will replace. The query reports them again once the
    /// event is recorded.
    pub overwrites: Vec<ReadingOverwrite>,
}

/// Build the one fact table from a frozen Run. Events are consumed in
/// recorded-time order (not caller array order); a later fact for the same key
/// is a new revision and never rewrites the earlier event.
pub fn query_readings(workspace: &RunWorkspace, request: &ReadingQuery) -> ReadingQueryResult {
    let mut history: Vec<ReadingFact> = Vec::new();
    let mut latest: HashMap<ReadingKey, usize> = HashMap::new();
    let mut overwrites: Vec<ReadingOverwrite> = Vec::new();
    let mut issues: Vec<ReadingIssue> = Vec::new();

    // The stored read model may hand events newest-first; revisions must follow
    // recorded time, not array position. `time_key` normalizes RFC 3339
    // fractional digits so string comparison is chronological.
    let mut events: Vec<&ExecutionEventView> = workspace.events.iter().collect();
    events.sort_by_key(|event| {
        time_key(
            event
                .recorded_at
                .as_deref()
                .or(event.occurred_at.as_deref())
                .unwrap_or_default(),
        )
    });
    for event in events {
        let has_readings = !event.measurements.is_empty()
            || !event.result_cells.is_empty()
            || !event.readouts.is_empty();
        if !has_readings {
            continue;
        }
        let Some(step_path) = event
            .protocol_step_path
            .as_deref()
            .map(str::trim)
            .filter(|step| !step.is_empty())
        else {
            issues.push(issue(
                "missing_step",
                "读数事件没有关联 Protocol 步骤，不能进入事实表",
                event,
                None,
            ));
            continue;
        };
        let step_path = step_path.to_owned();

        // A batch event records both `result_cells` and the derived
        // `measurements`; they describe the same cells, so structured arrays
        // win and legacy `readouts` are only used when both are absent.
        let structured = !event.result_cells.is_empty() || !event.measurements.is_empty();
        let mut seen: HashSet<(ReadingTarget, String)> = HashSet::new();
        let mut plate_choice: Option<Result<&RunPlateView, &'static str>> = None;
        let mut raw = Vec::new();

        if structured {
            for cell in &event.result_cells {
                // `kind: "record"` entries hold a complete Readout (table,
                // categorical, file). They carry no cell identity and must not
                // become numeric facts; `readout_records` reads them back.
                if cell["kind"].as_str() == Some(crate::readout_records::READOUT_RECORD_KIND) {
                    continue;
                }
                let Some(endpoint) = endpoint_of(cell) else {
                    issues.push(issue(
                        "missing_endpoint",
                        "结构化读数缺少 field 或 endpoint，未写入事实表",
                        event,
                        Some(&step_path),
                    ));
                    continue;
                };
                let value = cell["value"].as_f64();
                let status = match status_of(cell, value) {
                    Ok(status) => status,
                    Err((code, message)) => {
                        issues.push(issue(code, &message, event, Some(&step_path)));
                        continue;
                    }
                };
                raw.push(RawReading {
                    endpoint,
                    position: position_of(cell),
                    unit_id: text_of(cell, "sample_id"),
                    value,
                    unit: text_of(cell, "unit"),
                    status,
                });
            }
            for measurement in &event.measurements {
                let Some(endpoint) = endpoint_of(measurement) else {
                    issues.push(issue(
                        "missing_endpoint",
                        "数值读数缺少 field 或 endpoint，未写入事实表",
                        event,
                        Some(&step_path),
                    ));
                    continue;
                };
                let value = measurement["value"].as_f64();
                let status = match status_of(measurement, value) {
                    Ok(status) => status,
                    Err((code, message)) => {
                        issues.push(issue(code, &message, event, Some(&step_path)));
                        continue;
                    }
                };
                raw.push(RawReading {
                    endpoint,
                    position: position_of(measurement),
                    unit_id: text_of(measurement, "sample_id"),
                    value,
                    unit: text_of(measurement, "unit"),
                    status,
                });
            }
        } else {
            for readout in &event.readouts {
                let Some(endpoint) = endpoint_of(readout) else {
                    continue;
                };
                let result_kind = readout["result_kind"].as_str().unwrap_or("raw");
                if result_kind != "raw" {
                    // Derived results are computed from raw observations; they
                    // are not facts of their own and must not enter the raw
                    // fact table.
                    continue;
                }
                let value = readout["value"].as_f64();
                if value.is_none() {
                    // A non-numeric observation is not a numeric reading fact;
                    // it stays in the event text instead of being coerced.
                    continue;
                }
                raw.push(RawReading {
                    endpoint,
                    position: None,
                    unit_id: text_of(readout, "sample_id"),
                    value,
                    unit: text_of(readout, "unit"),
                    status: ReadingStatus::Raw,
                });
            }
        }

        for reading in raw {
            let target = match reading.position {
                Some((row, column)) => {
                    let plate = match plate_choice.get_or_insert_with(|| {
                        resolve_plate_for(
                            &workspace.plates,
                            event.assay_id.as_deref(),
                            event.timepoint.as_deref(),
                        )
                    }) {
                        Ok(plate) => *plate,
                        Err(code) => {
                            issues.push(issue(
                                code,
                                "读数有行列位置，但本次 Run 不能唯一确定孔板；未生成孔位事实",
                                event,
                                Some(&step_path),
                            ));
                            continue;
                        }
                    };
                    if (plate.rows > 0 && row >= plate.rows)
                        || (plate.columns > 0 && column >= plate.columns)
                    {
                        issues.push(issue(
                            "invalid_position",
                            &format!(
                                "位置 row={row} column={column} 超出孔板 {}×{}",
                                plate.rows, plate.columns
                            ),
                            event,
                            Some(&step_path),
                        ));
                        continue;
                    }
                    let Some(coordinate) = coordinate_for(row, column) else {
                        issues.push(issue(
                            "invalid_position",
                            &format!("位置 row={row} column={column} 无法转换为孔位坐标"),
                            event,
                            Some(&step_path),
                        ));
                        continue;
                    };
                    ReadingTarget::Well {
                        plate_id: plate.plate_id.clone(),
                        coordinate,
                    }
                }
                None => match reading.unit_id {
                    Some(unit_id) => ReadingTarget::Unit { unit_id },
                    None => {
                        issues.push(issue(
                            "missing_target",
                            "读数既无孔位也无样本/单位，不能进入事实表",
                            event,
                            Some(&step_path),
                        ));
                        continue;
                    }
                }
            };
            if !seen.insert((target.clone(), reading.endpoint.clone())) {
                continue;
            }

            let key = ReadingKey {
                step_path: step_path.clone(),
                assay_id: event.assay_id.clone(),
                timepoint: event.timepoint.clone(),
                endpoint: reading.endpoint.clone(),
                target: target.clone(),
            };
            let recorded_at = event
                .recorded_at
                .clone()
                .or_else(|| event.occurred_at.clone());
            let (revision, supersedes_event_id) = match latest.get(&key) {
                Some(&index) => {
                    let previous = &history[index];
                    let previous_event_id = previous.source_event_id.clone();
                    let previous_revision = previous.revision;
                    overwrites.push(ReadingOverwrite {
                        key: key.clone(),
                        previous_event_id: previous_event_id.clone(),
                        previous_revision,
                        replacement_event_id: Some(event.event_id.clone()),
                        replacement_revision: previous_revision + 1,
                        recorded_at: recorded_at.clone(),
                    });
                    (previous_revision + 1, Some(previous_event_id))
                }
                None => (1, None),
            };
            latest.insert(key, history.len());
            history.push(ReadingFact {
                run_id: workspace.run_id.clone(),
                step_path: step_path.clone(),
                assay_id: event.assay_id.clone(),
                timepoint: event.timepoint.clone(),
                endpoint: reading.endpoint,
                target,
                value: reading.value,
                unit: reading.unit,
                status: reading.status,
                source_event_id: event.event_id.clone(),
                revision,
                supersedes_event_id,
                recorded_at,
            });
        }
    }

    let mut facts = Vec::new();
    for (index, fact) in history.iter().enumerate() {
        if latest.get(&fact.key()) != Some(&index) || !fact_matches(fact, request) {
            continue;
        }
        facts.push(fact.clone());
    }
    let history = if request.include_history {
        history
            .into_iter()
            .filter(|fact| fact_matches(fact, request))
            .collect()
    } else {
        facts.clone()
    };
    let overwrites = overwrites
        .into_iter()
        .filter(|overwrite| key_matches(&overwrite.key, request))
        .collect();
    issues.retain(|issue| issue_matches(issue, request));

    ReadingQueryResult {
        schema_version: READING_FACT_SCHEMA_VERSION,
        run_id: workspace.run_id.clone(),
        facts,
        history,
        overwrites,
        issues,
    }
}

/// Flat table projection over the query result.
pub fn project_table(result: &ReadingQueryResult, plate: &RunPlateView) -> ReadingTable {
    ReadingTable {
        plate_id: plate.plate_id.clone(),
        rows: plate.rows,
        columns: plate.columns,
        cells: well_cells(result, &plate.plate_id),
    }
}

/// Spatial plate projection over the same query result. `wells` is ordered by
/// row then column; each well keeps its endpoint cells.
pub fn project_plate(result: &ReadingQueryResult, plate: &RunPlateView) -> ReadingPlate {
    let mut wells: Vec<ReadingPlateWell> = Vec::new();
    for cell in well_cells(result, &plate.plate_id) {
        match wells.last_mut() {
            Some(well) if well.row == cell.row && well.column == cell.column => {
                well.cells.push(cell);
            }
            _ => wells.push(ReadingPlateWell {
                coordinate: cell.coordinate.clone(),
                row: cell.row,
                column: cell.column,
                cells: vec![cell],
            }),
        }
    }
    ReadingPlate {
        plate_id: plate.plate_id.clone(),
        rows: plate.rows,
        columns: plate.columns,
        wells,
    }
}

/// Validate a readout batch and build the single execution event it appends.
/// A cell that already has a fact is reported as an overwrite; the earlier
/// event is never mutated. The batch must resolve to exactly one plate of this
/// Run; otherwise its facts would be invisible to [`query_readings`], so the
/// batch is rejected instead of written.
pub fn plan_reading_batch(
    batch: &ReadingBatch,
    workspace: &RunWorkspace,
) -> Result<ReadingBatchPlan, Vec<ReadingIssue>> {
    let mut issues: Vec<ReadingIssue> = Vec::new();
    if batch.run_id.trim().is_empty() {
        issues.push(blocking("missing_run", "批次缺少 run_id"));
    } else if batch.run_id.trim() != workspace.run_id {
        issues.push(blocking("wrong_run", "批次 run_id 与 workspace 不一致"));
    }
    if batch.step_path.trim().is_empty() {
        issues.push(blocking("missing_step", "批次缺少 Protocol 步骤"));
    }
    if batch.plate_id.trim().is_empty() {
        issues.push(blocking("missing_plate", "批次缺少孔板 ID；孔位坐标在孔板内才唯一"));
    }
    if batch.schema_id.trim().is_empty() {
        issues.push(blocking("missing_schema", "批次缺少结果 schema"));
    }
    if batch.cells.is_empty() {
        issues.push(blocking("empty_batch", "批次至少需要一个读数单元"));
    }
    if batch.timepoint.is_some() && batch.assay_id.is_none() {
        issues.push(blocking(
            "timepoint_without_assay",
            "时间点必须同时给出检测；Core 不会按名称猜检测",
        ));
    }

    let plate = match resolve_plate_for(
        &workspace.plates,
        batch.assay_id.as_deref(),
        batch.timepoint.as_deref(),
    ) {
        Ok(plate) => {
            if !batch.plate_id.trim().is_empty() && batch.plate_id.trim() != plate.plate_id {
                issues.push(blocking(
                    "unknown_plate",
                    &format!(
                        "批次孔板 {} 不能由本次 Run 的检测/时间点唯一确定",
                        batch.plate_id.trim()
                    ),
                ));
            }
            Some(plate)
        }
        Err(code) => {
            issues.push(blocking(
                code,
                "本次 Run 不能唯一确定孔板；写入后事实表无法归属孔位",
            ));
            None
        }
    };

    let mut seen = HashSet::new();
    let mut cells = Vec::with_capacity(batch.cells.len());
    for (index, cell) in batch.cells.iter().enumerate() {
        let field = cell.field.trim();
        if field.is_empty() {
            issues.push(blocking(
                "missing_field",
                &format!("cell {index}: 读数缺少 field"),
            ));
        }
        if !seen.insert((cell.row, cell.column)) {
            issues.push(blocking(
                "duplicate_position",
                &format!("cell {index}: 同一行列为 {} 重复出现", display_position(cell.row, cell.column)),
            ));
        }
        let status = cell
            .status
            .as_deref()
            .unwrap_or(if cell.value.is_some() { "raw" } else { "missing" })
            .to_owned();
        if !matches!(status.as_str(), "raw" | "missing") {
            issues.push(blocking(
                "invalid_status",
                &format!("cell {index}: status 必须是 raw 或 missing"),
            ));
        } else if (status == "raw") != cell.value.is_some() {
            issues.push(blocking(
                "status_value_mismatch",
                &format!("cell {index}: status 与 value 不一致"),
            ));
        }
        if let Some(plate) = plate {
            if (plate.rows > 0 && cell.row >= plate.rows)
                || (plate.columns > 0 && cell.column >= plate.columns)
            {
                issues.push(blocking(
                    "invalid_position",
                    &format!("cell {index}: 位置超出孔板 {}×{}", plate.rows, plate.columns),
                ));
                continue;
            }
        }
        let Some(coordinate) = coordinate_for(cell.row, cell.column) else {
            issues.push(blocking(
                "invalid_position",
                &format!("cell {index}: 行列无法转换为孔位坐标"),
            ));
            continue;
        };
        cells.push((cell, field.to_owned(), status, coordinate));
    }
    if !issues.is_empty() {
        return Err(issues);
    }

    let current = query_readings(workspace, &ReadingQuery::default());
    let mut overwrites = Vec::new();
    let mut seen_keys: HashSet<ReadingKey> = HashSet::new();
    for (_, field, _, coordinate) in &cells {
        let key = ReadingKey {
            step_path: batch.step_path.trim().to_owned(),
            assay_id: batch.assay_id.clone(),
            timepoint: batch.timepoint.clone(),
            endpoint: field.clone(),
            target: ReadingTarget::Well {
                plate_id: batch.plate_id.trim().to_owned(),
                coordinate: coordinate.clone(),
            },
        };
        if !seen_keys.insert(key.clone()) {
            continue;
        }
        if let Some(fact) = current.facts.iter().find(|fact| fact.key() == key) {
            overwrites.push(ReadingOverwrite {
                key,
                previous_event_id: fact.source_event_id.clone(),
                previous_revision: fact.revision,
                replacement_event_id: None,
                replacement_revision: fact.revision + 1,
                recorded_at: None,
            });
        }
    }

    let measurements: Vec<Value> = cells
        .iter()
        .filter(|(_, _, status, _)| status.as_str() == "raw")
        .map(|(cell, field, _, _)| {
            json!({
                "endpoint": field,
                "value": cell.value,
                "unit": cell.unit.clone().unwrap_or_default(),
                "sample_id": cell.sample_id.clone(),
                "row": cell.row,
                "column": cell.column,
                "group": cell.group.clone(),
                "replicate": cell.replicate,
                "status": "raw",
                "field": field,
            })
        })
        .collect();
    let result_cells: Vec<Value> = cells
        .iter()
        .map(|(cell, field, status, _)| {
            json!({
                "row": cell.row,
                "column": cell.column,
                "field": field,
                "value": cell.value,
                "unit": cell.unit.clone(),
                "sample_id": cell.sample_id.clone(),
                "group": cell.group.clone(),
                "replicate": cell.replicate,
                "status": status,
            })
        })
        .collect();
    let event = json!({
        "run_id": batch.run_id,
        "event_type": "measurement",
        "content": format!("结构化读数批次：{} 个孔位", cells.len()),
        "protocol_step_path": batch.step_path.trim(),
        "assay_id": batch.assay_id,
        "timepoint": batch.timepoint,
        "batch_id": batch.batch_id,
        "schema_id": batch.schema_id,
        "schema_version": batch.schema_version,
        "operation_id": batch.operation_id,
        "measurements": measurements,
        "result_cells": result_cells,
    });

    Ok(ReadingBatchPlan { event, overwrites })
}

struct RawReading {
    endpoint: String,
    position: Option<(u32, u32)>,
    unit_id: Option<String>,
    value: Option<f64>,
    unit: Option<String>,
    status: ReadingStatus,
}

pub(crate) fn resolve_plate_for<'a>(
    plates: &'a [RunPlateView],
    assay_id: Option<&str>,
    timepoint: Option<&str>,
) -> Result<&'a RunPlateView, &'static str> {
    let mut candidates: Vec<&RunPlateView> = plates
        .iter()
        .filter(|plate| match (assay_id, plate.assay_id.as_deref()) {
            (Some(assay), Some(plate_assay)) => assay == plate_assay,
            (None, _) => true,
            (Some(_), None) => false,
        })
        .collect();
    if candidates.len() > 1 {
        if let Some(timepoint) = timepoint {
            let filtered: Vec<&RunPlateView> = candidates
                .iter()
                .copied()
                .filter(|plate| plate.timepoint.as_deref() == Some(timepoint))
                .collect();
            if !filtered.is_empty() {
                candidates = filtered;
            }
        }
    }
    match candidates.len() {
        1 => Ok(candidates[0]),
        0 => Err("unresolved_plate"),
        _ => Err("ambiguous_plate"),
    }
}

fn endpoint_of(item: &Value) -> Option<String> {
    text_of(item, "field").or_else(|| text_of(item, "endpoint"))
}

fn text_of(item: &Value, field: &str) -> Option<String> {
    item[field]
        .as_str()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_owned)
}

fn position_of(item: &Value) -> Option<(u32, u32)> {
    let row = item["row"].as_u64()?;
    let column = item["column"].as_u64()?;
    Some((row as u32, column as u32))
}

fn status_of(item: &Value, value: Option<f64>) -> Result<ReadingStatus, (&'static str, String)> {
    let status = item["status"]
        .as_str()
        .map(str::trim)
        .filter(|status| !status.is_empty());
    match (status, value) {
        (None, Some(_)) | (Some("raw"), Some(_)) => Ok(ReadingStatus::Raw),
        (None, None) | (Some("missing"), None) => Ok(ReadingStatus::Missing),
        (Some("raw"), None) => Err((
            "status_value_mismatch",
            "status=raw 但缺少数值；读数被跳过".to_owned(),
        )),
        (Some("missing"), Some(_)) => Err((
            "status_value_mismatch",
            "status=missing 但带数值；读数被跳过".to_owned(),
        )),
        (Some(other), _) => Err((
            "unsupported_cell_status",
            format!("不支持的读数状态 {other}；读数被跳过"),
        )),
    }
}

pub(crate) fn coordinate_for(row: u32, column: u32) -> Option<String> {
    if row >= 26 {
        return None;
    }
    let letter = char::from(b'A' + row as u8);
    Some(format!("{letter}{}", column + 1))
}

/// Normalize an RFC 3339 timestamp to a fixed-width, lexicographically
/// comparable key. Fractional seconds are padded to nanoseconds and a `Z` zone
/// becomes `+00:00`, so stored UTC timestamps order correctly. Non-timestamp
/// text is returned as is.
pub(crate) fn time_key(value: &str) -> String {
    let Some(time_at) = value.find('T') else {
        return value.to_owned();
    };
    let zone_at = value[time_at + 1..]
        .find(['Z', '+', '-'])
        .map(|offset| time_at + 1 + offset)
        .unwrap_or(value.len());
    let (body, zone) = value.split_at(zone_at);
    let zone = if zone == "Z" { "+00:00" } else { zone };
    let (head, fraction) = match body.find('.') {
        Some(dot) => (&body[..dot], &body[dot + 1..]),
        None => (body, ""),
    };
    let mut fraction = fraction.to_owned();
    fraction.truncate(9);
    while fraction.len() < 9 {
        fraction.push('0');
    }
    format!("{head}.{fraction}{zone}")
}

pub(crate) fn parse_coordinate(coordinate: &str) -> Option<(u32, u32)> {
    let letters: String = coordinate
        .chars()
        .take_while(|value: &char| value.is_ascii_alphabetic())
        .collect();
    let digits: String = coordinate.chars().skip(letters.len()).collect();
    if letters.is_empty() || digits.is_empty() {
        return None;
    }
    let mut row = 0u32;
    for byte in letters.bytes() {
        let value = (byte.to_ascii_uppercase() as u32).checked_sub('A' as u32)? + 1;
        row = row.checked_mul(26)?.checked_add(value)?;
    }
    let column = digits.parse::<u32>().ok()?.checked_sub(1)?;
    Some((row - 1, column))
}

fn display_position(row: u32, column: u32) -> String {
    coordinate_for(row, column).unwrap_or_else(|| format!("row={row} column={column}"))
}

fn well_cells(result: &ReadingQueryResult, plate_id: &str) -> Vec<ReadingTableCell> {
    let mut cells: Vec<ReadingTableCell> = result
        .facts
        .iter()
        .filter_map(|fact| {
            let ReadingTarget::Well {
                plate_id: fact_plate,
                coordinate,
            } = &fact.target
            else {
                return None;
            };
            if fact_plate != plate_id {
                return None;
            }
            let (row, column) = parse_coordinate(coordinate)?;
            Some(ReadingTableCell {
                coordinate: coordinate.clone(),
                row,
                column,
                endpoint: fact.endpoint.clone(),
                value: fact.value,
                unit: fact.unit.clone(),
                status: fact.status,
                source_event_id: fact.source_event_id.clone(),
                revision: fact.revision,
            })
        })
        .collect();
    cells.sort_by(|left, right| {
        (left.row, left.column)
            .cmp(&(right.row, right.column))
            .then_with(|| left.endpoint.cmp(&right.endpoint))
    });
    cells
}

fn fact_matches(fact: &ReadingFact, request: &ReadingQuery) -> bool {
    key_matches(
        &ReadingKey {
            step_path: fact.step_path.clone(),
            assay_id: fact.assay_id.clone(),
            timepoint: fact.timepoint.clone(),
            endpoint: fact.endpoint.clone(),
            target: fact.target.clone(),
        },
        request,
    )
}

fn key_matches(key: &ReadingKey, request: &ReadingQuery) -> bool {
    request
        .step_path
        .as_ref()
        .is_none_or(|step| &key.step_path == step)
        && request
            .assay_id
            .as_ref()
            .is_none_or(|assay| key.assay_id.as_ref() == Some(assay))
        && request
            .timepoint
            .as_ref()
            .is_none_or(|timepoint| key.timepoint.as_ref() == Some(timepoint))
        && request
            .endpoint
            .as_ref()
            .is_none_or(|endpoint| &key.endpoint == endpoint)
        && request.plate_id.as_ref().is_none_or(|plate| match &key.target {
            ReadingTarget::Well { plate_id, .. } => plate_id == plate,
            ReadingTarget::Unit { .. } => false,
        })
}

fn issue_matches(issue: &ReadingIssue, request: &ReadingQuery) -> bool {
    if let Some(step) = request.step_path.as_ref() {
        if issue.step_path.as_ref() != Some(step) {
            return false;
        }
    }
    if let Some(plate) = request.plate_id.as_ref() {
        if let Some(ReadingTarget::Well { plate_id, .. }) = issue.target.as_ref() {
            if plate_id != plate {
                return false;
            }
        }
    }
    true
}

fn issue(
    code: &str,
    message: &str,
    event: &ExecutionEventView,
    step_path: Option<&str>,
) -> ReadingIssue {
    ReadingIssue {
        code: code.to_owned(),
        message: message.to_owned(),
        event_id: Some(event.event_id.clone()),
        step_path: step_path.map(str::to_owned),
        target: None,
    }
}

fn blocking(code: &str, message: &str) -> ReadingIssue {
    ReadingIssue {
        code: code.to_owned(),
        message: message.to_owned(),
        event_id: None,
        step_path: None,
        target: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

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

    fn plate(assay: &str, timepoint: &str) -> Value {
        json!({"id":"plate-96","name":"96 孔板","format":"96","rows":8,"columns":12,
            "assay_id":assay,"timepoint":timepoint,"wells":[]})
    }

    fn cell_event(id: &str, recorded_at: &str, value: f64) -> Value {
        json!({
            "id": id, "event_type":"measurement", "content":"结构化读数批次",
            "recorded_at": recorded_at, "protocol_step_path":"root/read",
            "assay_id":"cck8", "timepoint":"D1",
            "result_cells":[
                {"row":0,"column":0,"field":"od450","value":value,"unit":"AU","status":"raw"}
            ],
            "measurements":[
                {"endpoint":"od450","value":value,"unit":"AU","row":0,"column":0,"status":"raw","field":"od450"}
            ]
        })
    }

    fn workspace(plates: &[Value], events: &[Value]) -> RunWorkspace {
        crate::run_workspace::build("r1", "d1", "dv1", 1, &design(), &run(), plates, events, &[])
            .unwrap()
    }

    fn plate_of(workspace: &RunWorkspace) -> &RunPlateView {
        &workspace.plates[0]
    }

    #[test]
    fn table_and_plate_projections_return_the_same_cells() {
        let events = vec![json!({
            "id":"e1","event_type":"measurement","content":"read",
            "recorded_at":"2026-01-01T00:00:00Z","protocol_step_path":"root/read",
            "assay_id":"cck8","timepoint":"D1",
            "result_cells":[
                {"row":0,"column":0,"field":"od450","value":1.23,"unit":"AU","status":"raw"},
                {"row":0,"column":1,"field":"od450","status":"missing"}
            ]
        })];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert!(result.issues.is_empty(), "{:?}", result.issues);
        assert_eq!(result.facts.len(), 2);

        let table = project_table(&result, plate_of(&workspace));
        let drawn = project_plate(&result, plate_of(&workspace));
        let mut from_plate: Vec<ReadingTableCell> =
            drawn.wells.iter().flat_map(|well| well.cells.clone()).collect();
        from_plate.sort_by_key(|cell| (cell.row, cell.column, cell.endpoint.clone()));
        let mut from_table = table.cells.clone();
        from_table.sort_by_key(|cell| (cell.row, cell.column, cell.endpoint.clone()));

        assert_eq!(from_table, from_plate);
        assert_eq!(table.cells.len(), 2);
        assert_eq!(drawn.wells.len(), 2);
        let a1 = &drawn.wells[0];
        assert_eq!(a1.coordinate, "A1");
        assert_eq!(a1.cells[0].value, Some(1.23));
        let a2 = &drawn.wells[1];
        assert_eq!(a2.coordinate, "A2");
        assert_eq!(a2.cells[0].value, None);
        assert_eq!(a2.cells[0].status, ReadingStatus::Missing);
    }

    #[test]
    fn re_recording_a_well_writes_a_new_event_and_keeps_history() {
        let events = vec![
            cell_event("e1", "2026-01-01T00:00:00Z", 1.0),
            cell_event("e2", "2026-01-02T00:00:00Z", 2.0),
        ];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let current = query_readings(&workspace, &ReadingQuery::default());
        assert_eq!(current.facts.len(), 1, "measurement and result cell are one fact");
        let fact = &current.facts[0];
        assert_eq!(fact.value, Some(2.0));
        assert_eq!(fact.source_event_id, "e2");
        assert_eq!(fact.revision, 2);
        assert_eq!(fact.supersedes_event_id.as_deref(), Some("e1"));
        assert_eq!(current.history.len(), 1, "history is opt-in");
        assert_eq!(current.overwrites.len(), 1);
        assert_eq!(current.overwrites[0].previous_event_id, "e1");
        assert_eq!(current.overwrites[0].replacement_event_id.as_deref(), Some("e2"));

        let with_history = query_readings(
            &workspace,
            &ReadingQuery {
                include_history: true,
                ..ReadingQuery::default()
            },
        );
        assert_eq!(with_history.history.len(), 2);
        assert_eq!(with_history.history[0].value, Some(1.0));
        assert_eq!(with_history.history[0].revision, 1);

        let table = project_table(&current, plate_of(&workspace));
        let drawn = project_plate(&current, plate_of(&workspace));
        assert_eq!(table.cells.len(), 1);
        assert_eq!(table.cells[0].value, Some(2.0));
        assert_eq!(drawn.wells[0].cells[0].value, Some(2.0));
        assert_eq!(drawn.wells[0].cells[0].source_event_id, "e2");
    }

    #[test]
    fn ambiguous_plate_is_reported_instead_of_guessed() {
        let events = vec![cell_event("e1", "2026-01-01T00:00:00Z", 1.0)];
        let mut second = plate("cck8", "D1");
        second["id"] = json!("plate-2");
        let workspace = workspace(&[plate("cck8", "D1"), second], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert!(result.facts.is_empty());
        assert!(result.issues.iter().any(|issue| issue.code == "ambiguous_plate"));
    }

    #[test]
    fn missing_step_is_reported_not_folded_into_an_anonymous_key() {
        let events = vec![json!({
            "id":"e1","event_type":"measurement","content":"read",
            "recorded_at":"2026-01-01T00:00:00Z","assay_id":"cck8","timepoint":"D1",
            "result_cells":[{"row":0,"column":0,"field":"od450","value":1.0,"status":"raw"}]
        })];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert!(result.facts.is_empty());
        assert!(result.issues.iter().any(|issue| issue.code == "missing_step"));
    }

    #[test]
    fn sample_readout_without_a_well_stays_on_the_unit_axis() {
        let events = vec![json!({
            "id":"e1","event_type":"observation","content":"Cq",
            "recorded_at":"2026-01-01T00:00:00Z","protocol_step_path":"root/read",
            "assay_id":"cck8","timepoint":"D1",
            "readouts":[{"endpoint":"cq","value":21.5,"unit":"Cq","sample_id":"s1"}]
        })];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert!(result.issues.is_empty(), "{:?}", result.issues);
        assert_eq!(result.facts.len(), 1);
        assert_eq!(
            result.facts[0].target,
            ReadingTarget::Unit {
                unit_id: "s1".to_owned()
            }
        );
        assert!(project_table(&result, plate_of(&workspace)).cells.is_empty());
    }

    #[test]
    fn derived_readouts_never_become_raw_facts() {
        let events = vec![json!({
            "id":"e1","event_type":"observation","content":"fold change",
            "recorded_at":"2026-01-01T00:00:00Z","protocol_step_path":"root/read",
            "assay_id":"cck8","timepoint":"D1",
            "readouts":[{"endpoint":"fold_change","value":2.0,"result_kind":"derived",
                "derived_from":["r1"]}]
        })];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert!(result.facts.is_empty());
        assert!(result.history.is_empty());
        assert!(result.issues.is_empty(), "{:?}", result.issues);
    }

    #[test]
    fn unknown_cell_status_is_reported_not_coerced_to_raw() {
        let events = vec![json!({
            "id":"e1","event_type":"measurement","content":"read",
            "recorded_at":"2026-01-01T00:00:00Z","protocol_step_path":"root/read",
            "assay_id":"cck8","timepoint":"D1",
            "result_cells":[{"row":0,"column":0,"field":"od450","value":1.0,
                "status":"excluded"}]
        })];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert!(result.facts.is_empty());
        assert!(result
            .issues
            .iter()
            .any(|issue| issue.code == "unsupported_cell_status"));
    }

    #[test]
    fn filters_apply_to_facts_and_overwrites_together() {
        let events = vec![
            cell_event("e1", "2026-01-01T00:00:00Z", 1.0),
            cell_event("e2", "2026-01-02T00:00:00Z", 2.0),
        ];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let other = query_readings(
            &workspace,
            &ReadingQuery {
                endpoint: Some("ct".to_owned()),
                include_history: true,
                ..ReadingQuery::default()
            },
        );
        assert!(other.facts.is_empty());
        assert!(other.history.is_empty());
        assert!(other.overwrites.is_empty());

        let matching = query_readings(
            &workspace,
            &ReadingQuery {
                plate_id: Some("plate-96".to_owned()),
                ..ReadingQuery::default()
            },
        );
        assert_eq!(matching.facts.len(), 1);
    }

    #[test]
    fn revisions_follow_recorded_time_not_array_position() {
        // The stored read model can return newest-first.
        let events = vec![
            cell_event("e2", "2026-01-02T00:00:00Z", 2.0),
            cell_event("e1", "2026-01-01T00:00:00Z", 1.0),
        ];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        assert_eq!(workspace.events[0].event_id, "e2");
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert_eq!(result.facts.len(), 1);
        assert_eq!(result.facts[0].source_event_id, "e2");
        assert_eq!(result.facts[0].value, Some(2.0));
        assert_eq!(result.facts[0].revision, 2);
        assert_eq!(result.facts[0].supersedes_event_id.as_deref(), Some("e1"));
    }

    #[test]
    fn overwrite_is_derived_from_appending_events_not_from_mutating_them() {
        let events = vec![
            cell_event("e1", "2026-01-01T00:00:00Z", 1.0),
            cell_event("e2", "2026-01-02T00:00:00Z", 2.0),
        ];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let result = query_readings(&workspace, &ReadingQuery::default());
        assert_eq!(workspace.events.len(), 2);
        assert_eq!(workspace.events[0].measurements[0]["value"], 1.0);
        assert_eq!(result.facts[0].revision, 2);
    }

    fn batch(value: f64) -> ReadingBatch {
        ReadingBatch {
            run_id: "r1".to_owned(),
            step_path: "root/read".to_owned(),
            plate_id: "plate-96".to_owned(),
            schema_id: "cck8.plate96.od450.v1".to_owned(),
            schema_version: 1,
            assay_id: Some("cck8".to_owned()),
            timepoint: Some("D1".to_owned()),
            batch_id: None,
            operation_id: Some("op-1".to_owned()),
            cells: vec![ReadingCellInput {
                row: 0,
                column: 0,
                field: "od450".to_owned(),
                value: Some(value),
                unit: Some("AU".to_owned()),
                sample_id: Some("s1".to_owned()),
                group: Some("g1".to_owned()),
                replicate: Some(1),
                status: Some("raw".to_owned()),
            }],
        }
    }

    #[test]
    fn write_plan_emits_one_measurement_event_and_reports_the_replaced_fact() {
        let events = vec![cell_event("e1", "2026-01-01T00:00:00Z", 1.0)];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let plan = plan_reading_batch(&batch(9.0), &workspace).unwrap();
        assert_eq!(plan.event["event_type"], "measurement");
        assert_eq!(plan.event["protocol_step_path"], "root/read");
        assert_eq!(plan.event["schema_id"], "cck8.plate96.od450.v1");
        assert_eq!(plan.event["measurements"].as_array().unwrap().len(), 1);
        assert_eq!(plan.event["measurements"][0]["value"], 9.0);
        assert_eq!(plan.event["measurements"][0]["row"], 0);
        assert_eq!(plan.event["result_cells"].as_array().unwrap().len(), 1);
        assert_eq!(plan.overwrites.len(), 1);
        assert_eq!(plan.overwrites[0].previous_event_id, "e1");
        assert_eq!(plan.overwrites[0].replacement_event_id, None);
        assert_eq!(plan.overwrites[0].replacement_revision, 2);
    }

    #[test]
    fn write_plan_rejects_duplicate_positions_and_status_mismatch() {
        let events = vec![cell_event("e1", "2026-01-01T00:00:00Z", 1.0)];
        let workspace = workspace(&[plate("cck8", "D1")], &events);

        let mut duplicate = batch(1.0);
        duplicate.cells.push(duplicate.cells[0].clone());
        let issues = plan_reading_batch(&duplicate, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "duplicate_position"));

        let mut mismatch = batch(1.0);
        mismatch.cells[0].status = Some("missing".to_owned());
        let issues = plan_reading_batch(&mismatch, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "status_value_mismatch"));
    }

    #[test]
    fn write_plan_rejects_a_batch_whose_plate_cannot_be_resolved() {
        let events = vec![cell_event("e1", "2026-01-01T00:00:00Z", 1.0)];
        let workspace = workspace(&[plate("cck8", "D1")], &events);
        let mut wrong = batch(1.0);
        wrong.plate_id = "plate-other".to_owned();
        let issues = plan_reading_batch(&wrong, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "unknown_plate"));

        let mut out_of_range = batch(1.0);
        out_of_range.cells[0].row = 8;
        let issues = plan_reading_batch(&out_of_range, &workspace).unwrap_err();
        assert!(issues.iter().any(|issue| issue.code == "invalid_position"));
    }
}
