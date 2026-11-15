//! Readout schema registry and the one submission path shared by plate,
//! single-value and table readouts.
//!
//! A [`ReadoutRegistry`] holds validated [`ReadoutSchema`] definitions; a
//! [`ReadoutSubmission`] names one Run step and carries any mix of readout
//! shapes. [`plan_readout_submission`] validates every readout through the
//! same pipeline (`validate_schema` → `validate_readout` →
//! `check_derived_sources`) and returns one `execution_event.create` payload,
//! so no experiment gets its own record table.
//!
//! Storage layout inside that single event (`result_cells` is a free-form
//! array, see `plan_reading_batch`):
//!
//! - `{"kind": "cell", …}` entries carry numeric plate/single values and are
//!   the cells `query_readings` turns into reading facts;
//! - `{"kind": "record", "readout": …}` entries carry the complete v1
//!   [`Readout`]. Table, categorical and file readouts have no lossless
//!   `readouts[]` form (contract §2.2), so this is where they live; numeric and
//!   plate readouts get a record too, so the stored readout round-trips exactly.
//!
//! `query_readings` ignores `record` entries; [`query_readout_records`] reads
//! them back.

use crate::reading_facts::{
    parse_coordinate, query_readings, resolve_plate_for, time_key, ReadingIssue, ReadingKey,
    ReadingOverwrite, ReadingQuery, ReadingTarget,
};
use crate::readout_schema::{
    check_derived_sources, to_legacy_readouts, validate_readout, validate_schema, LegacyReadout,
    Readout, ReadoutIssue, ReadoutSchema, ReadoutValue, SourceKind, ValueKind,
};
use crate::run_workspace::RunWorkspace;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::collections::{BTreeMap, HashMap, HashSet};

/// Schema version of the stored readout record shape in `result_cells`.
pub const READOUT_RECORD_SCHEMA_VERSION: u32 = 1;

/// The `kind` value marking a stored readout record (as opposed to a numeric
/// cell) inside `execution_event.result_cells`.
pub const READOUT_RECORD_KIND: &str = "record";

/// The three entry forms named by the recording card, plus the two remaining
/// value kinds.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RecordForm {
    /// Well-plate grid; each cell is one numeric reading.
    Plate,
    /// One scalar value.
    Single,
    /// Multi-column table; no per-cell identity, so it is stored as a record.
    Table,
    Categorical,
    File,
}

impl RecordForm {
    pub fn of(kind: ValueKind) -> Self {
        match kind {
            ValueKind::Plate => Self::Plate,
            ValueKind::Numeric => Self::Single,
            ValueKind::Table => Self::Table,
            ValueKind::Categorical => Self::Categorical,
            ValueKind::File => Self::File,
        }
    }

    /// Whether entries of this form become numeric reading facts.
    pub fn records_cells(self) -> bool {
        matches!(self, Self::Plate | Self::Single)
    }
}

/// One problem found while validating a submission. `code` is either a
/// contract code from [`ReadoutIssueCode`] or a submission code listed in the
/// module docs (`unknown_schema`, `wrong_run`, …).
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutSubmissionIssue {
    pub code: String,
    pub path: String,
    pub message: String,
}

impl ReadoutSubmissionIssue {
    fn new(code: &str, path: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            code: code.to_owned(),
            path: path.into(),
            message: message.into(),
        }
    }

    fn from_contract(path: &str, issue: ReadoutIssue) -> Self {
        let code = serde_json::to_value(issue.code)
            .ok()
            .and_then(|value| value.as_str().map(str::to_owned))
            .unwrap_or_else(|| "invalid_readout".to_owned());
        Self::new(&code, path, issue.message)
    }
}

/// Validated schema definitions, keyed by `schema_id`.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct ReadoutRegistry {
    schemas: BTreeMap<String, ReadoutSchema>,
}

impl ReadoutRegistry {
    pub fn new() -> Self {
        Self::default()
    }

    /// The four schemas the recording card names: CCK-8 OD450 (plate), RNA
    /// concentration and A260/A280 purity, and CFU counting.
    pub fn builtin() -> Self {
        let mut registry = Self::new();
        for schema in builtin_readout_schemas() {
            registry
                .insert(schema)
                .expect("builtin readout schemas are valid");
        }
        registry
    }

    /// Register one schema. Its own invariants must pass, and a `schema_id`
    /// may only be re-inserted with an identical definition: the same id may
    /// never silently change meaning.
    pub fn insert(&mut self, schema: ReadoutSchema) -> Result<(), Vec<ReadoutSubmissionIssue>> {
        let path = format!("schema:{}", schema.schema_id);
        let issues = validate_schema(&schema);
        if !issues.is_empty() {
            return Err(issues
                .into_iter()
                .map(|issue| ReadoutSubmissionIssue::from_contract(&path, issue))
                .collect());
        }
        match self.schemas.get(&schema.schema_id) {
            Some(existing) if existing == &schema => Ok(()),
            Some(_) => Err(vec![ReadoutSubmissionIssue::new(
                "duplicate_schema",
                path,
                format!("{} 已注册为不同定义；同 id 不得改语义", schema.schema_id),
            )]),
            None => {
                self.schemas.insert(schema.schema_id.clone(), schema);
                Ok(())
            }
        }
    }

    pub fn get(&self, schema_id: &str) -> Option<&ReadoutSchema> {
        self.schemas.get(schema_id)
    }

    pub fn len(&self) -> usize {
        self.schemas.len()
    }

    pub fn is_empty(&self) -> bool {
        self.schemas.is_empty()
    }

    pub fn ids(&self) -> impl Iterator<Item = &str> {
        self.schemas.keys().map(String::as_str)
    }
}

/// Contract §4 readout schemas, verbatim.
pub fn builtin_readout_schemas() -> Vec<ReadoutSchema> {
    let examples = [
        json!({
            "schema_id": "cck8_od450",
            "schema_version": 1,
            "endpoint": "od450",
            "value_kind": "plate",
            "unit": {"canonical": "AU", "required": true},
            "missing": {"allowed": ["not_detected", "above_uloq"], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": true, "sample_required": false},
            "plate": {"format": "96", "rows": 8, "columns": 12}
        }),
        json!({
            "schema_id": "rna_concentration",
            "schema_version": 1,
            "endpoint": "concentration",
            "value_kind": "numeric",
            "unit": {"canonical": "ng/µL", "required": true},
            "missing": {"allowed": ["below_lod"], "null_allowed": false},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": true, "sample_required": true}
        }),
        json!({
            "schema_id": "rna_purity",
            "schema_version": 1,
            "endpoint": "a260_a280",
            "value_kind": "numeric",
            "unit": {"canonical": "ratio", "required": true},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": true},
            "link": {"timepoint_required": true, "sample_required": true}
        }),
        json!({
            "schema_id": "cfu_count",
            "schema_version": 1,
            "endpoint": "cfu",
            "value_kind": "numeric",
            "unit": {"canonical": "CFU", "required": true},
            "missing": {"allowed": ["not_detected", "above_uloq"], "null_allowed": false},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": true, "sample_required": true}
        }),
    ];
    examples
        .into_iter()
        .map(|example| {
            serde_json::from_value(example).expect("builtin readout schema is a valid example")
        })
        .collect()
}

/// One batch of readouts recorded against one Run step.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutSubmission {
    pub run_id: String,
    pub step_path: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub batch_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub assay_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timepoint: Option<String>,
    pub readouts: Vec<Readout>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutSubmissionPlan {
    /// Payload for the single `execution_event.create` call.
    pub event: Value,
    /// Lossless `readouts[]` projection; table/categorical/file contribute
    /// nothing here (contract §2.2).
    pub readouts: Vec<LegacyReadout>,
    /// Record form of each submitted readout, in submission order.
    pub forms: Vec<RecordForm>,
    /// Facts this event will replace once recorded.
    pub overwrites: Vec<ReadingOverwrite>,
}

/// One readout read back from a stored event.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutRecord {
    pub readout: Readout,
    pub event_id: String,
    pub step_path: String,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub recorded_at: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default, deny_unknown_fields)]
pub struct ReadoutRecordQuery {
    pub step_path: Option<String>,
    pub assay_id: Option<String>,
    pub timepoint: Option<String>,
    pub schema_id: Option<String>,
    pub value_kind: Option<ValueKind>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutRecordQueryResult {
    pub schema_version: u32,
    pub run_id: String,
    pub records: Vec<ReadoutRecord>,
    /// Stored entries that cannot be read back as a [`Readout`].
    pub issues: Vec<ReadingIssue>,
}

/// Validate a readout submission and build the single execution event it
/// appends. Every readout shape goes through the same schema lookup and the
/// same `validate_schema` / `validate_readout` / `check_derived_sources`
/// pipeline; only the storage projection differs by form.
pub fn plan_readout_submission(
    registry: &ReadoutRegistry,
    submission: &ReadoutSubmission,
    workspace: &RunWorkspace,
) -> Result<ReadoutSubmissionPlan, Vec<ReadoutSubmissionIssue>> {
    let mut issues: Vec<ReadoutSubmissionIssue> = Vec::new();

    if submission.run_id.trim().is_empty() {
        issues.push(ReadoutSubmissionIssue::new(
            "missing_run",
            "run_id",
            "提交缺少 run_id",
        ));
    } else if submission.run_id.trim() != workspace.run_id {
        issues.push(ReadoutSubmissionIssue::new(
            "wrong_run",
            "run_id",
            "提交 run_id 与 workspace 不一致",
        ));
    }
    if submission.step_path.trim().is_empty() {
        issues.push(ReadoutSubmissionIssue::new(
            "missing_step",
            "step_path",
            "读数必须关联冻结 Protocol 步骤",
        ));
    }
    if submission.readouts.is_empty() {
        issues.push(ReadoutSubmissionIssue::new(
            "empty_submission",
            "readouts",
            "提交至少需要一个读数",
        ));
    }
    if submission.timepoint.is_some() && submission.assay_id.is_none() {
        issues.push(ReadoutSubmissionIssue::new(
            "timepoint_without_assay",
            "timepoint",
            "时间点必须同时给出检测；Core 不会按名称猜检测",
        ));
    }
    match (&submission.assay_id, &submission.timepoint) {
        (Some(assay_id), timepoint) => {
            match workspace
                .frozen_design
                .content
                .assays
                .iter()
                .find(|assay| &assay.id == assay_id)
            {
                Some(assay) => {
                    if let Some(timepoint) = timepoint {
                        if !assay.timepoints.iter().any(|known| known == timepoint) {
                            issues.push(ReadoutSubmissionIssue::new(
                                "unknown_timepoint",
                                "timepoint",
                                format!("时间点 {timepoint} 不在冻结设计 {assay_id} 的时间点中"),
                            ));
                        }
                    }
                }
                None => issues.push(ReadoutSubmissionIssue::new(
                    "unknown_assay",
                    "assay_id",
                    format!("检测 {assay_id} 不在冻结设计中"),
                )),
            }
        }
        (None, _) => {}
    }

    let mut schema_cache: HashMap<String, Vec<ReadoutIssue>> = HashMap::new();
    let mut assigned: Vec<Readout> = Vec::with_capacity(submission.readouts.len());
    let mut forms: Vec<RecordForm> = Vec::with_capacity(submission.readouts.len());
    let mut seen_ids: HashSet<String> = HashSet::new();
    let label = submission
        .batch_id
        .as_deref()
        .filter(|value| !value.trim().is_empty())
        .unwrap_or("batch");

    for (index, readout) in submission.readouts.iter().enumerate() {
        let path = format!("readouts[{index}]");
        let Some(schema) = registry.get(&readout.schema_id) else {
            issues.push(ReadoutSubmissionIssue::new(
                "unknown_schema",
                format!("{path}.schema_id"),
                format!("{} 未在 ReadoutRegistry 注册", readout.schema_id),
            ));
            forms.push(RecordForm::of(readout.value.value_kind()));
            continue;
        };

        let schema_issues = schema_cache
            .entry(schema.schema_id.clone())
            .or_insert_with(|| validate_schema(schema));
        if !schema_issues.is_empty() {
            let reported = issues
                .iter()
                .any(|issue| issue.code == "invalid_schema" && issue.path == path);
            if !reported {
                for issue in schema_issues.clone() {
                    issues.push(ReadoutSubmissionIssue::new(
                        "invalid_schema",
                        format!("{path}.schema"),
                        format!("{}: {}", issue.message, schema.schema_id),
                    ));
                }
            }
            forms.push(RecordForm::of(readout.value.value_kind()));
            continue;
        }

        let mut readout = readout.clone();
        let id = match readout.readout_id.as_deref().map(str::trim) {
            Some(id) if !id.is_empty() => id.to_owned(),
            _ => format!("ro-{label}-{index}"),
        };
        if !seen_ids.insert(id.clone()) {
            issues.push(ReadoutSubmissionIssue::new(
                "duplicate_readout_id",
                format!("{path}.readout_id"),
                format!("readout_id {id} 重复"),
            ));
        }
        readout.readout_id = Some(id);

        if let (Some(link_timepoint), Some(timepoint)) =
            (readout.link.timepoint.as_deref(), submission.timepoint.as_deref())
        {
            if link_timepoint != timepoint {
                issues.push(ReadoutSubmissionIssue::new(
                    "timepoint_mismatch",
                    format!("{path}.link.timepoint"),
                    format!("读数时间点 {link_timepoint} 与提交时间点 {timepoint} 不一致"),
                ));
            }
        }

        for issue in validate_readout(schema, &readout) {
            issues.push(ReadoutSubmissionIssue::from_contract(
                &format!("{path}.{}", issue.path),
                issue,
            ));
        }

        forms.push(RecordForm::of(readout.value.value_kind()));
        assigned.push(readout);
    }

    // Derived readouts may only cite raw readouts that already exist: either
    // earlier in this submission or in an event of this Run.
    let (mut raw_ids, mut derived_ids) = stored_readout_ids(workspace);
    for readout in &assigned {
        let Some(id) = readout.readout_id.clone() else {
            continue;
        };
        match readout.source.kind {
            SourceKind::Raw => raw_ids.push(id),
            SourceKind::Derived => derived_ids.push(id),
        }
    }
    let raw_refs: Vec<&str> = raw_ids.iter().map(String::as_str).collect();
    let derived_refs: Vec<&str> = derived_ids.iter().map(String::as_str).collect();
    for (index, readout) in assigned.iter().enumerate() {
        if readout.source.kind != SourceKind::Derived {
            continue;
        }
        for issue in check_derived_sources(readout, &raw_refs, &derived_refs) {
            issues.push(ReadoutSubmissionIssue::from_contract(
                &format!("readouts[{index}].{}", issue.path),
                issue,
            ));
        }
    }

    if !issues.is_empty() {
        return Err(issues);
    }

    let assay_id = submission.assay_id.as_deref();
    let timepoint = submission.timepoint.as_deref();
    let mut cells: Vec<Value> = Vec::new();
    let mut measurements: Vec<Value> = Vec::new();
    let mut legacy: Vec<LegacyReadout> = Vec::new();
    let mut forms_count: BTreeMap<&str, usize> = BTreeMap::new();

    for (index, readout) in assigned.iter().enumerate() {
        let path = format!("readouts[{index}]");
        let form = forms[index];
        *forms_count.entry(form_name(form)).or_default() += 1;
        let schema = registry
            .get(&readout.schema_id)
            .expect("schema was resolved before validation");

        cells.push(json!({
            "kind": READOUT_RECORD_KIND,
            "readout": readout,
        }));

        match (&readout.value, form) {
            (ReadoutValue::Plate { plate_id, rows, columns, cells: plate_cells }, RecordForm::Plate) => {
                let plate = match resolve_plate_for(&workspace.plates, assay_id, timepoint) {
                    Ok(plate) => plate,
                    Err(code) => {
                        issues.push(ReadoutSubmissionIssue::new(
                            code,
                            &path,
                            "本次 Run 不能唯一确定孔板；孔位事实无法归属",
                        ));
                        continue;
                    }
                };
                if plate.plate_id != *plate_id {
                    issues.push(ReadoutSubmissionIssue::new(
                        "unknown_plate",
                        format!("{path}.value.plate_id"),
                        format!("读数孔板 {plate_id} 不是本次 Run 的 {}", plate.plate_id),
                    ));
                    continue;
                }
                if plate.rows != *rows || plate.columns != *columns {
                    issues.push(ReadoutSubmissionIssue::new(
                        "plate_shape_mismatch",
                        format!("{path}.value"),
                        format!(
                            "读数板型 {rows}×{columns} 与 Run 孔板 {}×{} 不一致",
                            plate.rows, plate.columns
                        ),
                    ));
                    continue;
                }
                for cell in plate_cells {
                    let Some((row, column)) = parse_coordinate(&cell.coordinate) else {
                        issues.push(ReadoutSubmissionIssue::new(
                            "invalid_position",
                            format!("{path}.value.cells"),
                            format!("孔位 {} 不是有效坐标", cell.coordinate),
                        ));
                        continue;
                    };
                    let status = if cell.number.is_some() { "raw" } else { "missing" };
                    cells.push(json!({
                        "kind": "cell",
                        "readout_id": readout.readout_id,
                        "plate_id": plate.plate_id,
                        "row": row,
                        "column": column,
                        "field": schema.endpoint,
                        "value": cell.number,
                        "unit": readout.unit,
                        "sample_id": readout.link.sample_id,
                        "status": status,
                    }));
                    if status == "raw" {
                        measurements.push(json!({
                            "endpoint": schema.endpoint,
                            "value": cell.number,
                            "unit": readout.unit.clone().unwrap_or_default(),
                            "sample_id": readout.link.sample_id,
                            "row": row,
                            "column": column,
                            "status": status,
                            "field": schema.endpoint,
                        }));
                    }
                }
            }
            (ReadoutValue::Numeric { number }, RecordForm::Single) => {
                let (position, unit_id) = match numeric_target(readout) {
                    Ok(target) => target,
                    Err(issue) => {
                        issues.push(ReadoutSubmissionIssue::new(
                            issue.0,
                            format!("{path}.link"),
                            issue.1,
                        ));
                        continue;
                    }
                };
                if let Some((plate_id, coordinate)) = &position {
                    let plate = match resolve_plate_for(&workspace.plates, assay_id, timepoint) {
                        Ok(plate) => plate,
                        Err(code) => {
                            issues.push(ReadoutSubmissionIssue::new(
                                code,
                                &path,
                                "数值读数带孔位，但本次 Run 不能唯一确定孔板",
                            ));
                            continue;
                        }
                    };
                    if plate.plate_id != *plate_id {
                        issues.push(ReadoutSubmissionIssue::new(
                            "unknown_plate",
                            format!("{path}.link.target"),
                            format!("读数孔位属于 {plate_id}，不是本次 Run 的 {}", plate.plate_id),
                        ));
                        continue;
                    }
                    if !coordinate_is_inside(plate, coordinate) {
                        issues.push(ReadoutSubmissionIssue::new(
                            "invalid_position",
                            format!("{path}.link.target"),
                            format!(
                                "孔位 {coordinate} 超出孔板 {}×{}",
                                plate.rows, plate.columns
                            ),
                        ));
                        continue;
                    }
                }
                let status = if number.is_some() { "raw" } else { "missing" };
                let sample_id = readout.link.sample_id.clone().or(unit_id);
                let mut cell = json!({
                    "kind": "cell",
                    "readout_id": readout.readout_id,
                    "field": schema.endpoint,
                    "value": number,
                    "unit": readout.unit,
                    "sample_id": sample_id,
                    "status": status,
                });
                if let Some((plate_id, coordinate)) = &position {
                    cell["plate_id"] = json!(plate_id);
                    let (row, column) = parse_coordinate(coordinate)
                        .expect("numeric target coordinates were checked above");
                    cell["row"] = json!(row);
                    cell["column"] = json!(column);
                    if status == "raw" {
                        measurements.push(json!({
                            "endpoint": schema.endpoint,
                            "value": number,
                            "unit": readout.unit.clone().unwrap_or_default(),
                            "sample_id": sample_id,
                            "row": row,
                            "column": column,
                            "status": status,
                            "field": schema.endpoint,
                        }));
                    }
                } else if status == "raw" {
                    measurements.push(json!({
                        "endpoint": schema.endpoint,
                        "value": number,
                        "unit": readout.unit.clone().unwrap_or_default(),
                        "sample_id": sample_id,
                        "status": status,
                        "field": schema.endpoint,
                    }));
                }
                cells.push(cell);
            }
            _ => {}
        }

        if matches!(form, RecordForm::Plate | RecordForm::Single) {
            match to_legacy_readouts(schema, readout) {
                Ok(entries) => legacy.extend(entries),
                Err(issue) => issues.push(ReadoutSubmissionIssue::from_contract(&path, issue)),
            }
        }
    }

    if !issues.is_empty() {
        return Err(issues);
    }

    let current = query_readings(workspace, &ReadingQuery::default());
    let mut overwrites = Vec::new();
    let mut seen_keys: HashSet<ReadingKey> = HashSet::new();
    for cell in &cells {
        if cell["kind"].as_str() != Some("cell") {
            continue;
        }
        let Some(endpoint) = cell["field"].as_str() else {
            continue;
        };
        let target = match (cell["row"].as_u64(), cell["column"].as_u64()) {
            (Some(row), Some(column)) => {
                let (Some(plate_id), Some(coordinate)) = (
                    cell["plate_id"].as_str(),
                    crate::reading_facts::coordinate_for(row as u32, column as u32),
                ) else {
                    continue;
                };
                ReadingTarget::Well {
                    plate_id: plate_id.to_owned(),
                    coordinate,
                }
            }
            _ => match cell["sample_id"].as_str() {
                Some(unit_id) => ReadingTarget::Unit {
                    unit_id: unit_id.to_owned(),
                },
                None => continue,
            },
        };
        let key = ReadingKey {
            step_path: submission.step_path.trim().to_owned(),
            assay_id: submission.assay_id.clone(),
            timepoint: submission.timepoint.clone(),
            endpoint: endpoint.to_owned(),
            target,
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

    let count = assigned.len();
    let mut summary: Vec<String> = forms_count
        .iter()
        .map(|(form, count)| format!("{form} {count}"))
        .collect();
    summary.sort();
    let single_schema = assigned
        .first()
        .map(|readout| (&readout.schema_id, readout.schema_version))
        .filter(|(id, version)| {
            assigned
                .iter()
                .all(|readout| &readout.schema_id == *id && readout.schema_version == *version)
        });

    let mut event = json!({
        "run_id": submission.run_id,
        "event_type": "measurement",
        "content": format!("通用读数批次：{count} 条（{}）", summary.join(" / ")),
        "protocol_step_path": submission.step_path.trim(),
        "assay_id": submission.assay_id,
        "timepoint": submission.timepoint,
        "batch_id": submission.batch_id,
        "measurements": measurements,
        "result_cells": cells,
        "readouts": legacy,
    });
    if let Some((schema_id, schema_version)) = single_schema {
        event["schema_id"] = json!(schema_id);
        event["schema_version"] = json!(schema_version);
    }

    Ok(ReadoutSubmissionPlan {
        event,
        readouts: legacy.clone(),
        forms,
        overwrites,
    })
}

/// Read back every readout record stored in a Run's events, newest revision of
/// each `readout_id` only.
pub fn query_readout_records(
    workspace: &RunWorkspace,
    request: &ReadoutRecordQuery,
) -> ReadoutRecordQueryResult {
    let mut events: Vec<(String, &crate::run_workspace::ExecutionEventView)> = workspace
        .events
        .iter()
        .map(|event| {
            (
                time_key(
                    event
                        .recorded_at
                        .as_deref()
                        .or(event.occurred_at.as_deref())
                        .unwrap_or_default(),
                ),
                event,
            )
        })
        .collect();
    events.sort_by(|left, right| left.0.cmp(&right.0));

    let mut latest: HashMap<String, usize> = HashMap::new();
    let mut records: Vec<ReadoutRecord> = Vec::new();
    let mut issues: Vec<ReadingIssue> = Vec::new();
    for (_, event) in events {
        let step_path = event
            .protocol_step_path
            .as_deref()
            .map(str::trim)
            .filter(|path| !path.is_empty())
            .unwrap_or_default()
            .to_owned();
        for entry in &event.result_cells {
            if entry["kind"].as_str() != Some(READOUT_RECORD_KIND) {
                continue;
            }
            let readout: Readout = match serde_json::from_value(entry["readout"].clone()) {
                Ok(readout) => readout,
                Err(error) => {
                    issues.push(ReadingIssue {
                        code: "invalid_readout_record".to_owned(),
                        message: format!("存储的读数记录无法解析：{error}"),
                        event_id: Some(event.event_id.clone()),
                        step_path: Some(step_path.clone()),
                        target: None,
                    });
                    continue;
                }
            };
            if !record_matches(&readout, event, request) {
                continue;
            }
            let recorded_at = event.recorded_at.clone().or_else(|| event.occurred_at.clone());
            let record = ReadoutRecord {
                readout: readout.clone(),
                event_id: event.event_id.clone(),
                step_path: step_path.clone(),
                assay_id: event.assay_id.clone(),
                timepoint: event.timepoint.clone(),
                recorded_at,
            };
            match readout.readout_id {
                Some(id) => match latest.get(&id) {
                    Some(index) => records[*index] = record,
                    None => {
                        latest.insert(id, records.len());
                        records.push(record);
                    }
                },
                None => records.push(record),
            }
        }
    }

    ReadoutRecordQueryResult {
        schema_version: READOUT_RECORD_SCHEMA_VERSION,
        run_id: workspace.run_id.clone(),
        records,
        issues,
    }
}

fn record_matches(
    readout: &Readout,
    event: &crate::run_workspace::ExecutionEventView,
    request: &ReadoutRecordQuery,
) -> bool {
    if let Some(step_path) = &request.step_path {
        if event.protocol_step_path.as_deref() != Some(step_path.as_str()) {
            return false;
        }
    }
    if let Some(assay_id) = &request.assay_id {
        if event.assay_id.as_deref() != Some(assay_id.as_str()) {
            return false;
        }
    }
    if let Some(timepoint) = &request.timepoint {
        if event.timepoint.as_deref() != Some(timepoint.as_str()) {
            return false;
        }
    }
    if let Some(schema_id) = &request.schema_id {
        if &readout.schema_id != schema_id {
            return false;
        }
    }
    if let Some(value_kind) = request.value_kind {
        if readout.value.value_kind() != value_kind {
            return false;
        }
    }
    true
}

/// Readout ids already stored in this Run's events, split by provenance.
fn stored_readout_ids(workspace: &RunWorkspace) -> (Vec<String>, Vec<String>) {
    let mut raw = Vec::new();
    let mut derived = Vec::new();
    for event in &workspace.events {
        for entry in &event.result_cells {
            if entry["kind"].as_str() != Some(READOUT_RECORD_KIND) {
                continue;
            }
            let Ok(readout) = serde_json::from_value::<Readout>(entry["readout"].clone()) else {
                continue;
            };
            let Some(id) = readout.readout_id else {
                continue;
            };
            match readout.source.kind {
                SourceKind::Raw => raw.push(id),
                SourceKind::Derived => derived.push(id),
            }
        }
        for entry in &event.readouts {
            let Some(id) = entry["readout_id"].as_str().filter(|id| !id.is_empty()) else {
                continue;
            };
            match entry["result_kind"].as_str().unwrap_or("raw") {
                "raw" => raw.push(id.to_owned()),
                _ => derived.push(id.to_owned()),
            }
        }
    }
    (raw, derived)
}

/// `(plate_id, coordinate)` for a well target, else the unit/sample identity.
fn numeric_target(readout: &Readout) -> Result<(Option<(String, String)>, Option<String>), (&'static str, String)> {
    match &readout.link.target {
        Some(ReadingTarget::Well { plate_id, coordinate }) => {
            Ok((Some((plate_id.clone(), coordinate.clone())), None))
        }
        Some(ReadingTarget::Unit { unit_id }) => {
            let unit_id = unit_id.trim();
            if unit_id.is_empty() {
                return Err((
                    "missing_target",
                    "unit 目标缺少 unit_id；读数无法进入事实表".to_owned(),
                ));
            }
            Ok((None, readout.link.sample_id.clone().or_else(|| Some(unit_id.to_owned()))))
        }
        None => match readout.link.sample_id.as_deref().map(str::trim) {
            Some(sample_id) if !sample_id.is_empty() => Ok((None, Some(sample_id.to_owned()))),
            _ => Err((
                "missing_target",
                "读数既无孔位也无样本；写入后事实表无法归属".to_owned(),
            )),
        },
    }
}

fn coordinate_is_inside(plate: &crate::run_workspace::RunPlateView, coordinate: &str) -> bool {
    match parse_coordinate(coordinate) {
        Some((row, column)) => {
            (plate.rows == 0 || row < plate.rows) && (plate.columns == 0 || column < plate.columns)
        }
        None => false,
    }
}

fn form_name(form: RecordForm) -> &'static str {
    match form {
        RecordForm::Plate => "孔板",
        RecordForm::Single => "单值",
        RecordForm::Table => "表格",
        RecordForm::Categorical => "分类",
        RecordForm::File => "文件",
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::run_workspace::build;
    use serde_json::json;

    fn design() -> Value {
        json!({
            "title":"d","objective":"o","experimental_unit":"well",
            "group_specs":[],
            "assays":[
                {"id":"cck8","title":"CCK-8","timepoints":["D1"],
                 "group_ids":[],"timepoint_relation":"same_physical_sample_repeated"},
                {"id":"rna","title":"RNA","timepoints":["裂解后立即"],
                 "group_ids":[],"timepoint_relation":"same_physical_sample_repeated"}
            ],
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

    fn run_workspace(events: &[Value]) -> RunWorkspace {
        build("r1", "d1", "dv1", 1, &design(), &run(), &[plate()], events, &[]).unwrap()
    }

    fn registry() -> ReadoutRegistry {
        let mut registry = ReadoutRegistry::builtin();
        registry
            .insert(
                serde_json::from_value(json!({
                    "schema_id": "rna_quality",
                    "schema_version": 1,
                    "endpoint": "rna_quality",
                    "value_kind": "table",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": true, "sample_required": true},
                    "table": {"columns": [
                        {"key": "a260_a280", "label": "A260/A280", "value_kind": "numeric", "unit": "ratio"},
                        {"key": "a260_a230", "label": "A260/A230", "value_kind": "numeric", "unit": "ratio"}
                    ]}
                }))
                .unwrap(),
            )
            .expect("table schema is valid");
        registry
            .insert(
                serde_json::from_value(json!({
                    "schema_id": "colony_count_loose",
                    "schema_version": 1,
                    "endpoint": "colonies",
                    "value_kind": "numeric",
                    "unit": {"canonical": "count", "required": true},
                    "missing": {"allowed": ["not_detected"], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false}
                }))
                .unwrap(),
            )
            .expect("loose numeric schema is valid");
        registry
    }

    fn cck8_readout() -> Readout {
        serde_json::from_value(json!({
            "readout_id": "ro-cck8-1",
            "schema_id": "cck8_od450",
            "schema_version": 1,
            "value": {
                "kind": "plate",
                "plate_id": "plate-96",
                "rows": 8,
                "columns": 12,
                "cells": [
                    {"coordinate": "A1", "number": 1.234},
                    {"coordinate": "A2", "missing": "above_uloq"},
                    {"coordinate": "A3"}
                ]
            },
            "unit": "AU",
            "source": {"kind": "raw", "derived_from": []},
            "link": {"timepoint": "D1"}
        }))
        .unwrap()
    }

    fn cfu_readout() -> Readout {
        serde_json::from_value(json!({
            "readout_id": "ro-cfu-1",
            "schema_id": "cfu_count",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 143.0},
            "unit": "CFU",
            "source": {"kind": "raw", "derived_from": []},
            "link": {
                "timepoint": "D1",
                "sample_id": "S-3",
                "target": {"kind": "well", "plate_id": "plate-96", "coordinate": "B2"}
            }
        }))
        .unwrap()
    }

    fn table_readout() -> Readout {
        serde_json::from_value(json!({
            "readout_id": "ro-rna-table-1",
            "schema_id": "rna_quality",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["a260_a280", "a260_a230"],
                "rows": [[2.06, 1.98]]
            },
            "source": {"kind": "raw", "derived_from": []},
            "link": {"timepoint": "D1", "sample_id": "S-1"}
        }))
        .unwrap()
    }

    fn submission(readouts: Vec<Readout>) -> ReadoutSubmission {
        ReadoutSubmission {
            run_id: "r1".into(),
            step_path: "root/read".into(),
            batch_id: Some("b1".into()),
            assay_id: Some("cck8".into()),
            timepoint: Some("D1".into()),
            readouts,
        }
    }

    fn codes(issues: &[ReadoutSubmissionIssue]) -> Vec<String> {
        issues.iter().map(|issue| issue.code.clone()).collect()
    }

    #[test]
    fn readout_submit_registry_keeps_schema_identity() {
        let mut registry = ReadoutRegistry::builtin();
        assert_eq!(registry.len(), 4);
        assert!(registry.get("cck8_od450").is_some());
        assert!(registry.get("rna_purity").is_some());
        assert!(registry.get("missing_schema").is_none());

        let first = registry.get("cfu_count").cloned().unwrap();
        registry.insert(first.clone()).expect("identical re-insert is idempotent");
        assert_eq!(registry.len(), 4);

        let mut changed = first;
        changed.endpoint = "cfu_per_ml".into();
        let error = registry.insert(changed).unwrap_err();
        assert_eq!(codes(&error), vec!["duplicate_schema"]);

        let mut invalid = serde_json::from_value::<ReadoutSchema>(json!({
            "schema_id": "broken_plate",
            "schema_version": 1,
            "endpoint": "od450",
            "value_kind": "plate",
            "unit": {"canonical": "AU", "required": true},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false}
        }))
        .unwrap();
        invalid.plate = None;
        let error = registry.insert(invalid).unwrap_err();
        assert_eq!(codes(&error), vec!["kind_spec_missing"]);
    }

    #[test]
    fn readout_submit_plate_single_table_share_one_event() {
        let workspace = run_workspace(&[]);
        let plan = plan_readout_submission(
            &registry(),
            &submission(vec![cck8_readout(), cfu_readout(), table_readout()]),
            &workspace,
        )
        .expect("all three forms are valid");

        assert_eq!(
            plan.forms,
            vec![RecordForm::Plate, RecordForm::Single, RecordForm::Table]
        );
        let event = &plan.event;
        assert_eq!(event["event_type"], "measurement");
        assert_eq!(event["protocol_step_path"], "root/read");
        assert!(
            event.get("schema_id").is_none(),
            "a mixed submission has no single schema"
        );

        let cells = event["result_cells"].as_array().unwrap();
        let records = cells
            .iter()
            .filter(|cell| cell["kind"] == "record")
            .count();
        assert_eq!(records, 3, "every submitted readout is stored verbatim");
        let numeric_cells: Vec<&Value> = cells
            .iter()
            .filter(|cell| cell["kind"] == "cell")
            .collect();
        assert_eq!(numeric_cells.len(), 4, "three plate cells plus one count");
        assert_eq!(numeric_cells[0]["status"], "raw");
        assert_eq!(numeric_cells[1]["status"], "missing");
        assert_eq!(numeric_cells[2]["status"], "missing");
        assert_eq!(numeric_cells[3]["field"], "cfu");

        // Table readouts never enter the legacy numeric projection.
        assert_eq!(plan.readouts.len(), 4);
        assert!(plan
            .readouts
            .iter()
            .all(|readout| readout.endpoint != "rna_quality"));
        assert_eq!(plan.readouts[0].readout_id.as_deref(), Some("ro-cck8-1-A1"));
        assert_eq!(plan.overwrites.len(), 0);
    }

    #[test]
    fn readout_submit_written_event_is_queryable() {
        let workspace = run_workspace(&[]);
        let plan = plan_readout_submission(
            &registry(),
            &submission(vec![cck8_readout(), cfu_readout(), table_readout()]),
            &workspace,
        )
        .unwrap();

        let mut event = plan.event.clone();
        event["id"] = json!("e1");
        event["recorded_at"] = json!("2026-01-01T00:00:00Z");
        let recorded = run_workspace(&[event]);

        let facts = query_readings(&recorded, &ReadingQuery::default());
        assert!(facts.issues.is_empty(), "{:?}", facts.issues);
        assert_eq!(facts.facts.len(), 4, "plate cells and the count are facts");
        assert!(facts
            .facts
            .iter()
            .all(|fact| fact.endpoint != "rna_quality"));

        let stored = query_readout_records(&recorded, &ReadoutRecordQuery::default());
        assert!(stored.issues.is_empty(), "{:?}", stored.issues);
        assert_eq!(stored.records.len(), 3);
        let table = stored
            .records
            .iter()
            .find(|record| record.readout.schema_id == "rna_quality")
            .expect("table readout round-trips");
        assert_eq!(table.readout, table_readout());

        let plate_only = query_readout_records(
            &recorded,
            &ReadoutRecordQuery {
                value_kind: Some(ValueKind::Plate),
                ..ReadoutRecordQuery::default()
            },
        );
        assert_eq!(plate_only.records.len(), 1);
        assert_eq!(plate_only.records[0].readout, cck8_readout());
    }

    #[test]
    fn readout_submit_reports_contract_codes_for_invalid_readouts() {
        let workspace = run_workspace(&[]);
        let mut unknown = cfu_readout();
        unknown.readout_id = Some("ro-unknown".into());
        unknown.schema_id = "not_registered".into();
        let mut wrong_unit = cfu_readout();
        wrong_unit.readout_id = Some("ro-unit".into());
        wrong_unit.unit = Some("OD".into());
        let mut unrecorded = cfu_readout();
        unrecorded.readout_id = Some("ro-null".into());
        unrecorded.value = ReadoutValue::Numeric { number: None };

        let error = plan_readout_submission(
            &registry(),
            &submission(vec![unknown, wrong_unit, unrecorded]),
            &workspace,
        )
        .unwrap_err();
        assert_eq!(
            codes(&error),
            vec!["unknown_schema", "unit_mismatch", "missing_without_reason"]
        );
        assert_eq!(error[1].path, "readouts[1].unit");
        assert_eq!(error[2].path, "readouts[2].value");
    }

    #[test]
    fn readout_submit_derived_readout_must_cite_raw_readout() {
        let workspace = run_workspace(&[]);
        let ratio = |id: &str, sources: Vec<&str>| -> Readout {
            serde_json::from_value(json!({
                "readout_id": id,
                "schema_id": "rna_purity",
                "schema_version": 1,
                "value": {"kind": "numeric", "number": 2.06},
                "unit": "ratio",
                "source": {"kind": "derived", "method": "a260_a280_ratio@1", "derived_from": sources},
                "link": {"timepoint": "D1", "sample_id": "S-1"}
            }))
            .unwrap()
        };
        let concentration: Readout = serde_json::from_value(json!({
            "readout_id": "ro-rna-conc-1",
            "schema_id": "rna_concentration",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 214.5},
            "unit": "ng/µL",
            "source": {"kind": "raw", "derived_from": []},
            "link": {"timepoint": "D1", "sample_id": "S-1"}
        }))
        .unwrap();

        let plan = plan_readout_submission(
            &registry(),
            &submission(vec![
                concentration,
                ratio("ro-rna-ratio-1", vec!["ro-rna-conc-1"]),
            ]),
            &workspace,
        )
        .expect("a derived readout may cite a raw readout from the same submission");
        assert!(plan.readouts.iter().any(|readout| {
            readout.readout_id.as_deref() == Some("ro-rna-ratio-1")
                && readout.result_kind == SourceKind::Derived
        }));

        let error = plan_readout_submission(
            &registry(),
            &submission(vec![ratio(
                "ro-rna-ratio-2",
                vec!["ro-rna-conc-1", "ro-rna-ratio-1"],
            )]),
            &workspace,
        )
        .unwrap_err();
        assert_eq!(
            codes(&error),
            vec!["derived_source_unknown", "derived_source_unknown"]
        );
    }

    #[test]
    fn readout_submit_assigns_ids_and_rejects_duplicates() {
        let workspace = run_workspace(&[]);
        let mut first = cfu_readout();
        first.readout_id = None;
        let mut second = cfu_readout();
        second.readout_id = Some(String::new());

        let plan = plan_readout_submission(
            &registry(),
            &submission(vec![first, second]),
            &workspace,
        )
        .expect("blank ids are assigned, not rejected");
        assert_eq!(
            plan.readouts
                .iter()
                .map(|readout| readout.readout_id.clone().unwrap())
                .collect::<Vec<_>>(),
            vec!["ro-b1-0".to_owned(), "ro-b1-1".to_owned()]
        );

        let mut duplicate = cfu_readout();
        duplicate.readout_id = Some("ro-cfu-1".into());
        let error = plan_readout_submission(
            &registry(),
            &submission(vec![cfu_readout(), duplicate]),
            &workspace,
        )
        .unwrap_err();
        assert_eq!(codes(&error), vec!["duplicate_readout_id"]);
    }

    #[test]
    fn readout_submit_plate_must_match_the_run_plate() {
        let workspace = run_workspace(&[]);
        let mut foreign = cck8_readout();
        if let ReadoutValue::Plate { plate_id, .. } = &mut foreign.value {
            *plate_id = "plate-cfu".into();
        }
        let error = plan_readout_submission(&registry(), &submission(vec![foreign]), &workspace)
            .unwrap_err();
        assert_eq!(codes(&error), vec!["unknown_plate"]);

        let mut wrong_shape = cck8_readout();
        if let ReadoutValue::Plate { rows, columns, .. } = &mut wrong_shape.value {
            *rows = 4;
            *columns = 6;
        }
        let error =
            plan_readout_submission(&registry(), &submission(vec![wrong_shape]), &workspace)
                .unwrap_err();
        assert!(
            codes(&error).iter().any(|code| code == "invalid_plate_shape"),
            "{:?}",
            codes(&error)
        );
    }

    #[test]
    fn readout_submit_numeric_needs_a_target() {
        let workspace = run_workspace(&[]);
        let untargeted: Readout = serde_json::from_value(json!({
            "readout_id": "ro-loose-1",
            "schema_id": "colony_count_loose",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 12.0},
            "unit": "count",
            "source": {"kind": "raw", "derived_from": []},
            "link": {}
        }))
        .unwrap();
        let error = plan_readout_submission(&registry(), &submission(vec![untargeted]), &workspace)
            .unwrap_err();
        assert_eq!(codes(&error), vec!["missing_target"]);
        assert_eq!(error[0].path, "readouts[0].link");

        let mut no_sample = cfu_readout();
        no_sample.link.sample_id = None;
        let error = plan_readout_submission(&registry(), &submission(vec![no_sample]), &workspace)
            .unwrap_err();
        assert_eq!(codes(&error), vec!["sample_missing"]);

        let mut outside = cfu_readout();
        outside.link.target = Some(ReadingTarget::Well {
            plate_id: "plate-96".into(),
            coordinate: "Z99".into(),
        });
        let error = plan_readout_submission(&registry(), &submission(vec![outside]), &workspace)
            .unwrap_err();
        assert!(
            codes(&error).iter().any(|code| code == "invalid_position"),
            "{:?}",
            codes(&error)
        );
    }

    #[test]
    fn readout_submit_reports_overwrites_for_recorded_wells() {
        let workspace = run_workspace(&[]);
        let plan = plan_readout_submission(&registry(), &submission(vec![cck8_readout()]), &workspace)
            .unwrap();
        let mut event = plan.event.clone();
        event["id"] = json!("e1");
        event["recorded_at"] = json!("2026-01-01T00:00:00Z");
        let recorded = run_workspace(&[event]);

        let again = plan_readout_submission(&registry(), &submission(vec![cck8_readout()]), &recorded)
            .unwrap();
        assert_eq!(again.overwrites.len(), 3, "one overwrite per recorded cell");
        assert_eq!(again.overwrites[0].previous_event_id, "e1");
        assert_eq!(again.overwrites[0].replacement_revision, 2);
        assert!(again.overwrites[0].replacement_event_id.is_none());
    }

    #[test]
    fn readout_submit_rejects_empty_and_foreign_run() {
        let workspace = run_workspace(&[]);
        let error = plan_readout_submission(&registry(), &submission(vec![]), &workspace)
            .unwrap_err();
        assert_eq!(codes(&error), vec!["empty_submission"]);

        let mut foreign = submission(vec![cfu_readout()]);
        foreign.run_id = "other-run".into();
        foreign.step_path = "  ".into();
        let error = plan_readout_submission(&registry(), &foreign, &workspace).unwrap_err();
        assert_eq!(codes(&error), vec!["wrong_run", "missing_step"]);
    }

    #[test]
    fn readout_submit_checks_timepoint_against_the_frozen_design() {
        let workspace = run_workspace(&[]);
        let mut wrong_timepoint = submission(vec![cfu_readout()]);
        wrong_timepoint.timepoint = Some("处理后 48 h".into());
        let error = plan_readout_submission(&registry(), &wrong_timepoint, &workspace).unwrap_err();
        assert!(
            codes(&error).iter().any(|code| code == "unknown_timepoint"),
            "{:?}",
            codes(&error)
        );
        assert!(
            codes(&error).iter().any(|code| code == "timepoint_mismatch"),
            "{:?}",
            codes(&error)
        );

        let mut unknown_assay = submission(vec![cfu_readout()]);
        unknown_assay.assay_id = Some("missing-assay".into());
        let error = plan_readout_submission(&registry(), &unknown_assay, &workspace).unwrap_err();
        assert_eq!(codes(&error), vec!["unknown_assay"]);
    }
}
