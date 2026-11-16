use super::*;

pub(crate) fn readout_issue_error(
    action: &str,
    issues: Vec<asterlab_core::reading_facts::ReadingIssue>,
) -> CliError {
    let detail = issues
        .first()
        .map(|issue| format!("：{}", issue.message))
        .unwrap_or_default();
    let mut error = CliError::input(format!("{action}校验失败{detail}"));
    error.issues = issues
        .into_iter()
        .map(|issue| json!({"code":issue.code,"message":issue.message}))
        .collect();
    error
}

pub(crate) fn readout_submission_issue_error(
    action: &str,
    issues: Vec<ReadoutSubmissionIssue>,
) -> CliError {
    let detail = issues
        .first()
        .map(|issue| format!("：{}", issue.message))
        .unwrap_or_default();
    let mut error = CliError::input(format!("{action}校验失败{detail}"));
    error.issues = issues
        .into_iter()
        .map(|issue| json!({"code":issue.code,"path":issue.path,"message":issue.message}))
        .collect();
    error
}

/// One typed `readout.submit`: a batch of v1 readouts recorded through the
/// shared schema pipeline. Unlike the scalar form it can carry `table`,
/// `categorical`, `plate` and `file` values, and each readout names its schema.
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct TypedReadoutSubmit {
    pub(crate) run_id: String,
    pub(crate) step_path: String,
    pub(crate) protocol_version_id: String,
    #[serde(default)]
    pub(crate) assay_id: Option<String>,
    #[serde(default)]
    pub(crate) timepoint: Option<String>,
    #[serde(default)]
    pub(crate) batch_id: Option<String>,
    #[serde(default)]
    pub(crate) occurred_at: Option<String>,
    #[serde(default)]
    pub(crate) operator: Option<String>,
    pub(crate) readouts: Vec<Readout>,
    /// Schemas this submission relies on, on top of the builtin registry. A
    /// definition is checked by `ReadoutRegistry::insert` and never replaces a
    /// different definition of the same `schema_id`.
    #[serde(default)]
    pub(crate) readout_schemas: Vec<ReadoutSchema>,
}

/// Stable idempotency key over the submission's identity. Values are content,
/// not identity, so a replay returns the first event instead of overwriting it.
pub(crate) fn typed_readout_key(submission: &ReadoutSubmission) -> String {
    let mut hasher = Sha256::new();
    let parts = json!([
        submission.run_id.trim(),
        submission.step_path.trim(),
        submission.assay_id,
        submission.timepoint,
        submission.readouts,
    ]);
    hasher.update(serde_json::to_vec(&parts).unwrap_or_default());
    let digest: String = hasher
        .finalize()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    format!("readout:typed:{digest}")
}

/// `readout.submit` entry point: route the scalar form to the frozen numeric
/// path and the typed form through `plan_readout_submission`.
pub(crate) fn submit_readout_input(
    db: &mut Connection,
    input: Value,
) -> Result<Value, CliError> {
    if input.get("readouts").is_some() {
        let typed: TypedReadoutSubmit = serde_json::from_value(input)
            .map_err(|error| CliError::input(format!("读数提交字段无效：{error}")))?;
        return submit_typed_readouts(db, &typed);
    }
    let request: ReadoutSubmitRequest = serde_json::from_value(input)
        .map_err(|error| CliError::input(format!("读数提交字段无效：{error}")))?;
    submit_readout(db, &request)
}

/// Record one batch of typed readouts as a single measurement event.
pub(crate) fn submit_typed_readouts(
    db: &mut Connection,
    input: &TypedReadoutSubmit,
) -> Result<Value, CliError> {
    if input.readouts.is_empty() {
        return Err(CliError::input("readouts 至少需要一个读数"));
    }
    let mut registry = ReadoutRegistry::builtin();
    for schema in &input.readout_schemas {
        registry
            .insert(schema.clone())
            .map_err(|issues| readout_submission_issue_error("读数 schema", issues))?;
    }
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let workspace = run_workspace_typed(&tx, &input.run_id)?;
    // Same scope rule as the scalar and batch forms: the step must belong to
    // this Run's frozen manifest, never to a step the Run does not execute.
    let step_path = input.step_path.trim();
    if !workspace
        .steps
        .iter()
        .any(|step| step.step.path.trim() == step_path)
    {
        return Err(CliError::input(format!(
            "步骤 {step_path} 不属于本次 Run 的冻结清单"
        )));
    }
    let submission = ReadoutSubmission {
        run_id: input.run_id.trim().to_owned(),
        step_path: step_path.to_owned(),
        batch_id: input
            .batch_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned),
        assay_id: input
            .assay_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned),
        timepoint: input
            .timepoint
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_owned),
        readouts: input.readouts.clone(),
    };
    let plan = plan_readout_submission(&registry, &submission, &workspace)
        .map_err(|issues| readout_submission_issue_error("读数提交", issues))?;
    let key = typed_readout_key(&submission);
    let accepted = plan.readouts.len().max(input.readouts.len());
    let mut event = plan.event;
    // The typed planner builds the shared measurement payload; the CLI adds the
    // frozen Protocol scope the other readout tools already require.
    event["protocol_version_id"] = json!(input.protocol_version_id.trim());
    if let Some(at) = input.occurred_at.as_deref().filter(|v| !v.trim().is_empty()) {
        event["occurred_at"] = json!(at.trim());
    }
    if let Some(operator) = input
        .operator
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        event["operator"] = json!(operator);
    }
    event["operation_id"] = json!(key);
    event["operation_payload"] = serde_json::to_value(&input.readouts).unwrap_or(Value::Null);
    if let Some(record) = find_readout_submission(&tx, &input.run_id, &key)? {
        tx.commit()
            .map_err(|error| CliError::internal(error.to_string()))?;
        return Ok(json!({
            "record": record,
            "idempotency_key": key,
            "replayed": true,
            "accepted": accepted
        }));
    }
    let record = create_execution_event_tx(&tx, &input.run_id, &event)?;
    tx.commit()
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(json!({
        "record": record,
        "idempotency_key": key,
        "replayed": false,
        "accepted": accepted
    }))
}

/// JSON `cells[]` columns, in CSV order. `qualifier` is the reason a cell has
/// no number; the core CSV parser is the one validation path for it.
pub(crate) const BATCH_CELL_COLUMNS: [&str; 10] = [
    "row", "column", "field", "value", "unit", "sample_id", "group", "replicate", "status",
    "qualifier",
];

pub(crate) fn csv_field(value: &str) -> String {
    let escaped = value.replace('"', "\"\"");
    format!("\"{escaped}\"")
}

pub(crate) fn batch_cell_text(cell: &Value, column: &str) -> Result<String, &'static str> {
    let Some(value) = cell.get(column) else {
        return Ok(String::new());
    };
    if value.is_null() {
        return Ok(String::new());
    }
    match column {
        "value" => value
            .as_f64()
            .map(|number| number.to_string())
            .ok_or("必须是有限数值"),
        "row" | "column" | "replicate" => value
            .as_u64()
            .map(|number| number.to_string())
            .ok_or("必须是非负整数"),
        _ => value.as_str().map(str::to_owned).ok_or("必须是字符串"),
    }
}

/// Re-encode JSON `cells[]` as the CSV form so a `qualifier` cell goes through
/// the same parser, `Undetermined` mapping and status rules as a CSV upload.
pub(crate) fn cells_json_as_csv(cells: &[Value]) -> Result<String, CliError> {
    let mut csv = String::new();
    csv.push_str(&BATCH_CELL_COLUMNS.join(","));
    csv.push('\n');
    for (index, cell) in cells.iter().enumerate() {
        let mut fields = Vec::with_capacity(BATCH_CELL_COLUMNS.len());
        for column in BATCH_CELL_COLUMNS {
            let text = batch_cell_text(cell, column)
                .map_err(|expectation| {
                    CliError::input(format!("cells[{index}].{column} {expectation}"))
                })?;
            fields.push(csv_field(&text));
        }
        csv.push_str(&fields.join(","));
        csv.push('\n');
    }
    Ok(csv)
}

/// Accept a `qualifier` in JSON `cells[]` by handing the batch to the CSV path.
/// Cells without a qualifier keep the plain JSON route, so their error paths and
/// messages do not change.
pub(crate) fn readout_batch_request_from_json(input: Value) -> Result<ReadoutBatchRequest, CliError> {
    let has_csv = input.get("csv").is_some_and(|value| !value.is_null());
    let cells = input.get("cells").and_then(Value::as_array).cloned();
    let qualified = cells.as_deref().is_some_and(|cells| {
        cells
            .iter()
            .any(|cell| cell.get("qualifier").is_some_and(|value| !value.is_null()))
    });
    if !qualified || has_csv {
        if qualified && has_csv {
            return Err(CliError::input(
                "读数批次的 cells 与 csv 只能提供一种",
            ));
        }
        let mut input = input;
        // An explicit `"qualifier": null` is the same as omitting it; strip it
        // so the closed cell shape keeps accepting the request.
        if let Some(cells) = input.get_mut("cells").and_then(Value::as_array_mut) {
            for cell in cells.iter_mut() {
                if let Some(object) = cell.as_object_mut() {
                    if object.get("qualifier").is_some_and(Value::is_null) {
                        object.remove("qualifier");
                    }
                }
            }
        }
        return serde_json::from_value(input)
            .map_err(|error| CliError::input(format!("读数批次字段无效：{error}")));
    }
    let csv = cells_json_as_csv(cells.as_deref().unwrap_or_default())?;
    let mut object = match input {
        Value::Object(object) => object,
        _ => {
            return Err(CliError::input(
                "读数批次输入必须是 JSON 对象",
            ))
        }
    };
    object.remove("cells");
    object.insert("csv".to_owned(), Value::String(csv));
    serde_json::from_value(Value::Object(object))
        .map_err(|error| CliError::input(format!("读数批次字段无效：{error}")))
}

/// Look up an earlier readout submission by its idempotency key. The key is
/// stored as `operation_id`; the query is scoped to this Run, so the same key
/// in another Run is a different record.
pub(crate) fn find_readout_submission(
    db: &Connection,
    run_id: &str,
    operation_id: &str,
) -> Result<Option<Value>, CliError> {
    let id: Option<String> = db
        .query_row(
            "SELECT id FROM records WHERE kind='execution_event' AND parent_id=?1 \
             AND json_extract(data_json,'$.operation_id')=?2 ORDER BY created_at LIMIT 1",
            params![run_id, operation_id],
            |row| row.get(0),
        )
        .optional()
        .map_err(|error| CliError::internal(error.to_string()))?;
    match id {
        Some(id) => Ok(Some(get(db, &id)?)),
        None => Ok(None),
    }
}

/// `readout.submit`: validate one readout and append exactly one measurement
/// event. Re-submitting the same identity tuple returns the first record.
pub(crate) fn submit_readout(
    db: &mut Connection,
    input: &ReadoutSubmitRequest,
) -> Result<Value, CliError> {
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let workspace = run_workspace_typed(&tx, &input.run_id)?;
    let plan = plan_readout_submit(input, &workspace)
        .map_err(|issues| readout_issue_error("读数提交", issues))?;
    if let Some(record) = find_readout_submission(&tx, &input.run_id, &plan.idempotency_key)? {
        tx.commit()
            .map_err(|error| CliError::internal(error.to_string()))?;
        return Ok(json!({
            "record": record,
            "idempotency_key": plan.idempotency_key,
            "replayed": true
        }));
    }
    let record = create_execution_event_tx(&tx, &input.run_id, &plan.event)?;
    tx.commit()
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(json!({
        "record": record,
        "idempotency_key": plan.idempotency_key,
        "replayed": false
    }))
}

/// `readout.batch`: validate every cell of a table/plate batch and append one
/// measurement event in a single transaction. Any invalid row aborts the whole
/// batch; no earlier row is written.
pub(crate) fn submit_readout_batch(
    db: &mut Connection,
    input: &ReadoutBatchRequest,
) -> Result<Value, CliError> {
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let workspace = run_workspace_typed(&tx, &input.run_id)?;
    let plan = plan_readout_batch(input, &workspace)
        .map_err(|issues| readout_issue_error("读数批次", issues))?;
    if let Some(record) = find_readout_submission(&tx, &input.run_id, &plan.idempotency_key)? {
        tx.commit()
            .map_err(|error| CliError::internal(error.to_string()))?;
        return Ok(json!({
            "record": record,
            "idempotency_key": plan.idempotency_key,
            "replayed": true,
            "accepted": plan.readout_count
        }));
    }
    let record = create_execution_event_tx(&tx, &input.run_id, &plan.event)?;
    tx.commit()
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(json!({
        "record": record,
        "idempotency_key": plan.idempotency_key,
        "replayed": false,
        "accepted": plan.readout_count
    }))
}
pub(crate) fn merge_design_checks(run_data: &Value, frozen: &mut Value, step_path: &str) {
    let Some(design_checks) = run_data.get("design_checks").and_then(Value::as_array) else {
        return;
    };
    let Some(checks) = frozen.get_mut("checks").and_then(Value::as_array_mut) else {
        frozen["checks"] = json!([]);
        let Some(checks) = frozen.get_mut("checks").and_then(Value::as_array_mut) else { return; };
        for entry in design_checks {
            if entry.get("path").and_then(Value::as_str) != Some(step_path) { continue; }
            for check in entry.get("checks").and_then(Value::as_array).into_iter().flatten() {
                if !checks.iter().any(|existing| existing.get("check_id") == check.get("check_id")) {
                    checks.push(check.clone());
                }
            }
        }
        return;
    };
    for entry in design_checks {
        if entry.get("path").and_then(Value::as_str) != Some(step_path) { continue; }
        for check in entry.get("checks").and_then(Value::as_array).into_iter().flatten() {
            if !checks.iter().any(|existing| existing.get("check_id") == check.get("check_id")) {
                checks.push(check.clone());
            }
        }
    }
}

pub(crate) fn frozen_step_for_event(run_data: &Value, event: &ExecutionEventCreate) -> Option<Value> {
    let step_path = event.protocol_step_path.as_deref()?;
    let protocol_version_id = event.protocol_version_id.as_deref()?;
    if event.planned_output_id.is_none() {
        for manifest in run_data
            .get("protocol_manifest")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            if manifest.get("protocol_version_id").and_then(Value::as_str) != Some(protocol_version_id) {
                continue;
            }
            if manifest.get("assay_id").and_then(Value::as_str) != event.assay_id.as_deref() {
                continue;
            }
            if let Some(step) = manifest
                .get("steps")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .find(|step| step.get("path").and_then(Value::as_str) == Some(step_path))
            {
                let mut frozen = step.clone();
                merge_design_checks(run_data, &mut frozen, step_path);
                return Some(frozen);
            }
        }
    } else {
        for manifest in run_data
            .get("material_manifest")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            if manifest.get("planned_output_id").and_then(Value::as_str) != event.planned_output_id.as_deref() {
                continue;
            }
            if manifest
                .get("producer_protocol")
                .and_then(|p| p.get("protocol_version_id"))
                .and_then(Value::as_str)
                != Some(protocol_version_id)
            {
                continue;
            }
            if let Some(step) = manifest
                .get("producer_protocol")
                .and_then(|p| p.get("steps"))
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .find(|step| step.get("path").and_then(Value::as_str) == Some(step_path))
            {
                return Some(step.clone());
            }
        }
    }
    None
}
pub(crate) fn refresh_run_status_in_transaction(tx: &Transaction<'_>, run_id: &str) -> Result<(), CliError> {
    let workspace = run_workspace(tx, run_id)?;
    let tasks = workspace["tasks"].as_array().cloned().unwrap_or_default();
    if tasks.is_empty() {
        return Ok(());
    }
    let blocked = tasks.iter().any(|task| task["status"].as_str() == Some("blocked"));
    let all_done = tasks.iter().all(|task| task["status"].as_str() == Some("completed"));
    let target = if blocked && !all_done {
        "blocked"
    } else if all_done {
        "completed"
    } else {
        "in_progress"
    };
    let run = get(tx, run_id)?;
    if run["data"]["status"].as_str() == Some(target) {
        return Ok(());
    }
    let mut data = run["data"].clone();
    data["status"] = json!(target);
    let revision = run["revision"].as_i64().unwrap_or(0);
    tx.execute(
        "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1 AND revision=?5",
        params![run_id, revision + 1, serde_json::to_string(&data).map_err(|error| CliError::internal(error.to_string()))?, stamp(), revision],
    )
    .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(())
}

pub(crate) fn validate_run_assay_reference(
    db: &Connection,
    run_id: &str,
    assay_id: &str,
    timepoint: Option<&str>,
) -> Result<(), CliError> {
    let (kind, _, _, _, raw) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id 必须指向实验运行"));
    }
    let run: Value = serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    let design_version_id = run["design_version_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("运行缺少冻结设计版本"))?;
    let version = get(db, design_version_id)?;
    let snapshot = &version["data"]["snapshot"];
    let assays = snapshot["assays"]
        .as_array()
        .ok_or_else(|| CliError::internal("冻结设计版本缺少检测列表"))?;
    let assay = assays
        .iter()
        .find(|assay| assay["id"].as_str() == Some(assay_id))
        .ok_or_else(|| CliError::input("assay_id 必须属于该 Run 冻结设计中的检测"))?;
    if assay_requires_timepoint(snapshot, assay)
        && assay["timepoints"]
            .as_array()
            .is_some_and(|points| !points.is_empty())
        && timepoint.is_none()
    {
        return Err(CliError::input(
            "该检测每个时间点使用新样本；实际板图必须标记对应 timepoint",
        ));
    }
    if let Some(timepoint) = timepoint {
        if !assay["timepoints"]
            .as_array()
            .is_some_and(|points| points.iter().any(|point| point.as_str() == Some(timepoint)))
        {
            return Err(CliError::input(
                "板图 timepoint 必须属于该检测在 Run 冻结设计中的时间点",
            ));
        }
    }
    Ok(())
}

pub(crate) fn assay_requires_timepoint(snapshot: &Value, assay: &Value) -> bool {
    assay["timepoint_relation"] == "independent_sample_per_timepoint"
        || assay["sample_use_policy"] == "new_per_timepoint"
        || snapshot["sample_allocations"]
            .as_array()
            .is_some_and(|allocations| {
                allocations.iter().any(|allocation| {
                    allocation["assay_id"] == assay["id"]
                        && (allocation["timepoint_relation"] == "independent_sample_per_timepoint"
                            || allocation["relation"] == "new_per_timepoint")
                })
            })
}

