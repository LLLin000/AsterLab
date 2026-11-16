use super::*;

pub(crate) fn patch_run(
    db: &Connection,
    run_id: &str,
    expected_revision: i64,
    changes: &Value,
) -> Result<Value, CliError> {
    let patch: RunPatch = serde_json::from_value(changes.clone())
        .map_err(|e| CliError::input(format!("运行修订字段无效：{e}")))?;
    if patch.title.is_none() && patch.scheduled_date.is_none() && patch.status.is_none() {
        return Err(CliError::input(
            "至少提供 title、scheduled_date 或 status 中的一项",
        ));
    }
    let (kind, _, _, revision, raw) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("id 必须指向实验运行"));
    }
    if revision != expected_revision {
        return Err(CliError::conflict(format!(
            "运行版本已从 v{expected_revision} 更新为 v{revision}；请先重新读取。"
        )));
    }
    let mut data: Value = serde_json::from_str(&raw)
        .map_err(|e| CliError::internal(format!("运行快照无法读取：{e}")))?;
    if let Some(title) = patch.title {
        check_text(&title, "运行名称", 160)?;
        data["title"] = json!(title.trim());
    }
    if let Some(scheduled_date) = patch.scheduled_date {
        if let Some(date) = scheduled_date.as_deref() {
            validate_date(date, "计划开始日期")?;
        }
        data["scheduled_date"] = json!(scheduled_date);
    }
    if let Some(status) = patch.status {
        if ![
            "planned",
            "in_progress",
            "completed",
            "blocked",
            "cancelled",
        ]
        .contains(&status.as_str())
        {
            return Err(CliError::input(
                "运行状态必须是 planned、in_progress、completed、blocked 或 cancelled",
            ));
        }
        if status == "in_progress" {
            ensure_run_protocol_ready(db, run_id)?;
        }
        data["status"] = json!(status);
    }
    update_data(db, run_id, expected_revision, "run", &data)
}

pub(crate) fn create_execution_event_tx(
    tx: &Transaction<'_>,
    run_id: &str,
    input: &Value,
) -> Result<Value, CliError> {
    let mut payload = input.clone();
    payload["run_id"] = json!(run_id);
    let event: ExecutionEventCreate = serde_json::from_value(payload)
        .map_err(|e| CliError::input(format!("执行事件字段无效：{e}")))?;
    let (kind, project, _, _, raw) =
        kind_id(tx, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("parent_id 必须指向实验运行"));
    }
    if ![
        "operation",
        "material_use",
        "measurement",
        "deviation",
        "observation",
        "skipped_by_rule",
    ]
    .contains(&event.event_type.as_str())
    {
        return Err(CliError::input(
            "event_type 必须是 operation、material_use、measurement、deviation、observation 或 skipped_by_rule",
        ));
    }
    check_text(&event.content, "事件内容", 5000)?;
    if let Some(at) = &event.occurred_at {
        if DateTime::parse_from_rfc3339(at).is_err() {
            return Err(CliError::input(
                "occurred_at 必须是带时区的 RFC 3339 时间；不确定时留空",
            ));
        }
    }
    if event
        .stage
        .as_ref()
        .is_some_and(|v| v.chars().count() > 160)
    {
        return Err(CliError::input("操作阶段不能超过 160 个字符"));
    }
    if event
        .timepoint
        .as_ref()
        .is_some_and(|v| v.trim().is_empty() || v.chars().count() > 100)
    {
        return Err(CliError::input("timepoint 不能为空且不能超过 100 个字符"));
    }
    if event.step_status.as_deref().is_some_and(|status| {
        ![
            "in_progress",
            "completed",
            "blocked",
            "skipped",
            "qc_dropped",
            "censored_dead",
            "rework",
        ]
        .contains(&status)
    }) {
        return Err(CliError::input(
            "step_status 必须是 in_progress、completed、blocked、skipped、qc_dropped、censored_dead 或 rework",
        ));
    }
    if event.step_status.is_some() && event.protocol_step_path.is_none() {
        return Err(CliError::input("步骤状态只能关联 Protocol 步骤"));
    }
    if let Some(timepoint) = event.timepoint.as_deref() {
        let assay_id = event
            .assay_id
            .as_deref()
            .ok_or_else(|| CliError::input("检测步骤的 timepoint 必须同时提供 assay_id"))?;
        if event.protocol_step_path.is_none() {
            return Err(CliError::input("步骤 timepoint 必须关联 Protocol 步骤"));
        }
        validate_run_assay_reference(tx, run_id, assay_id, Some(timepoint))?;
    }
    for (value, label) in [
        (event.protocol_version_id.as_deref(), "Protocol 版本 ID"),
        (event.assay_id.as_deref(), "检测 ID"),
        (event.planned_output_id.as_deref(), "计划产物 ID"),
        (event.parent_sample_id.as_deref(), "父样本编号"),
        (event.aliquot_from.as_deref(), "分样来源编号"),
    ] {
        if let Some(value) = value {
            check_text(value, label, 120)?;
        }
    }
    if event.sample_ids.len() > 100 {
        return Err(CliError::input("样本编号最多 100 个"));
    }
    for sample_id in &event.sample_ids {
        check_text(sample_id, "样本编号", 120)?;
    }
    if event.materials.len() > 100 || event.measurements.len() > 100 {
        return Err(CliError::input("单个事件的材料使用或测量最多 100 项"));
    }
    let run_data: Value =
        serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    if let Some(step_path) = &event.protocol_step_path {
        check_text(step_path, "Protocol 步骤路径", 500)?;
        if event.assay_id.is_some() && event.planned_output_id.is_some() {
            return Err(CliError::input(
                "步骤事件只能关联一个检测或一个共享计划产物",
            ));
        }
        let protocol_version_id = event
            .protocol_version_id
            .as_deref()
            .ok_or_else(|| CliError::input("关联 Protocol 步骤时必须提供 protocol_version_id"))?;
        let in_protocols = run_data
            .get("protocol_manifest")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
            .any(|manifest| {
                manifest["protocol_version_id"].as_str() == Some(protocol_version_id)
                    && event.planned_output_id.is_none()
                    && manifest["assay_id"].as_str() == event.assay_id.as_deref()
                    && (event.ad_hoc_step
                        || manifest["steps"].as_array().is_some_and(|steps| {
                            steps.iter().any(|step| step["path"].as_str() == Some(step_path.as_str()))
                        }))
            });
        let in_materials = run_data
            .get("material_manifest")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
            .any(|manifest| {
                manifest["planned_output_id"].as_str() == event.planned_output_id.as_deref()
                    && event.assay_id.is_none()
                    && manifest["producer_protocol"]["protocol_version_id"].as_str()
                        == Some(protocol_version_id)
                    && manifest["producer_protocol"]["steps"]
                        .as_array()
                        .is_some_and(|steps| {
                            steps
                                .iter()
                                .any(|step| step["path"].as_str() == Some(step_path.as_str()))
                        })
            });
        if !in_protocols && !in_materials {
            return Err(CliError::input(
                "Protocol 步骤引用必须同时匹配本次运行冻结的 Protocol 版本、检测或计划产物及步骤路径",
            ));
        }
    } else if event.protocol_version_id.is_some()
        || event.assay_id.is_some()
        || event.planned_output_id.is_some()
    {
        return Err(CliError::input(
            "未关联 protocol_step_path 时不能提供 Protocol 步骤引用字段",
        ));
    }
    if let Some(task_key) = event.task_key.as_deref() {
        check_text(task_key, "执行任务键", 1000)?;
    }
    if !event.checks.is_empty() {
        let step_path = event
            .protocol_step_path
            .as_deref()
            .ok_or_else(|| CliError::input("检查结果必须关联冻结的 Protocol 步骤"))?;
        let protocol_version_id = event
            .protocol_version_id
            .as_deref()
            .ok_or_else(|| CliError::input("检查结果必须提供 protocol_version_id"))?;
        let mut frozen_step: Option<&Value> = None;
        for manifest in run_data
            .get("protocol_manifest")
            .and_then(Value::as_array)
            .into_iter()
            .flatten()
        {
            if manifest["protocol_version_id"].as_str() != Some(protocol_version_id)
                || event.planned_output_id.is_some()
                || manifest["assay_id"].as_str() != event.assay_id.as_deref()
            {
                continue;
            }
            frozen_step = manifest["steps"].as_array().and_then(|steps| {
                steps
                    .iter()
                    .find(|step| step["path"].as_str() == Some(step_path))
            });
            if frozen_step.is_some() {
                break;
            }
        }
        if frozen_step.is_none() {
            for manifest in run_data
                .get("material_manifest")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
            {
                if manifest["planned_output_id"].as_str()
                    != event.planned_output_id.as_deref()
                    || event.assay_id.is_some()
                    || manifest["producer_protocol"]["protocol_version_id"].as_str()
                        != Some(protocol_version_id)
                {
                    continue;
                }
                frozen_step = manifest["producer_protocol"]["steps"]
                    .as_array()
                    .and_then(|steps| {
                        steps
                            .iter()
                            .find(|step| step["path"].as_str() == Some(step_path))
                    });
                if frozen_step.is_some() {
                    break;
                }
            }
        }
        let frozen_step = frozen_step
            .ok_or_else(|| CliError::input("检查结果引用的 Protocol 步骤不属于本次冻结清单"))?;
        let mut allowed_checks: std::collections::HashSet<&str> = frozen_step["checks"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|check| check["check_id"].as_str())
            .collect();
        for entry in run_data["design_checks"].as_array().into_iter().flatten() {
            if entry["path"].as_str() != Some(step_path) {
                continue;
            }
            for check in entry["checks"].as_array().into_iter().flatten() {
                if let Some(check_id) = check["check_id"].as_str() {
                    allowed_checks.insert(check_id);
                }
            }
        }
        let mut seen = std::collections::HashSet::new();
        for check in &event.checks {
            if !seen.insert(check.check_id.as_str()) {
                return Err(CliError::input(format!(
                    "同一事件不能重复记录检查项：{}",
                    check.check_id
                )));
            }
            if !allowed_checks.contains(check.check_id.as_str()) {
                return Err(CliError::input(format!(
                    "检查项 {} 不属于冻结步骤 {}",
                    check.check_id, step_path
                )));
            }
        }
    }
    let planned_output_ids: std::collections::HashSet<&str> = run_data
        .get("material_manifest")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(|item| item.get("planned_output_id").and_then(Value::as_str))
        .collect();
    for material in &event.materials {
        check_text(&material.material, "材料名称", 160)?;
        if let Some(output_id) = &material.planned_output_id {
            if !planned_output_ids.contains(output_id.as_str()) {
                return Err(CliError::input(format!(
                    "planned_output_id 不属于本次运行的材料清单：{output_id}"
                )));
            }
        }
        if material
            .amount
            .is_some_and(|amount| !amount.is_finite() || amount < 0.0)
        {
            return Err(CliError::input("材料用量必须是有限的非负数"));
        }
        if material.amount.is_some()
            != material
                .unit
                .as_ref()
                .is_some_and(|unit| !unit.trim().is_empty())
        {
            return Err(CliError::input("填写材料用量时必须同时填写单位"));
        }
        if material
            .unit
            .as_ref()
            .is_some_and(|unit| unit.chars().count() > 40)
        {
            return Err(CliError::input("材料单位不能超过 40 个字符"));
        }
    }
    for measurement in &event.measurements {
        check_text(&measurement.endpoint, "测量终点", 300)?;
        if !measurement.value.is_finite() {
            return Err(CliError::input("测量值必须是有限数值"));
        }
        if let Some(sample_id) = &measurement.sample_id {
            check_text(sample_id, "测量样本编号", 120)?;
        }
        if measurement
            .unit
            .as_ref()
            .is_some_and(|unit| unit.chars().count() > 40)
        {
            return Err(CliError::input("测量单位不能超过 40 个字符"));
        }
    }
    if event.readouts.len() > 100 || event.checks.len() > 100 || event.controls.len() > 100 || event.lineage.len() > 100 {
        return Err(CliError::input("单个事件的读数、检查、对照或 lineage 最多 100 项"));
    }
    if let Some(operator) = &event.operator {
        check_text(operator, "记录人", 160)?;
    }
    let mut known_readouts: HashMap<String, String> = HashMap::new();
    if event
        .readouts
        .iter()
        .any(|readout| readout.readout_id.is_some() || readout.result_kind == "derived")
    {
        let mut stmt = tx
            .prepare("SELECT data_json FROM records WHERE kind='execution_event' AND parent_id=?1")
            .map_err(|e| CliError::internal(e.to_string()))?;
        let rows = stmt
            .query_map([run_id], |row| row.get::<_, String>(0))
            .map_err(|e| CliError::internal(e.to_string()))?;
        for row in rows {
            let raw = row.map_err(|e| CliError::internal(e.to_string()))?;
            let value: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            for readout in value["readouts"].as_array().into_iter().flatten() {
                if let (Some(id), Some(kind)) = (
                    readout["readout_id"].as_str(),
                    readout["result_kind"].as_str(),
                ) {
                    known_readouts.insert(id.to_owned(), kind.to_owned());
                }
            }
        }
    }
    let mut current_readout_ids = HashSet::new();
    for readout in &event.readouts {
        check_text(&readout.endpoint, "读数终点", 300)?;
        if !["raw", "derived"].contains(&readout.result_kind.as_str()) {
            return Err(CliError::input("readout.result_kind 必须是 raw 或 derived"));
        }
        if let Some(value) = readout.value {
            if !value.is_finite() {
                return Err(CliError::input("readout.value 必须是有限数值"));
            }
        }
        let has_qualifier = readout.qualifier.as_deref().is_some_and(|value| !value.trim().is_empty());
        if readout.value.is_some() == has_qualifier {
            return Err(CliError::input("readout 必须且只能填写数值或 qualifier 之一"));
        }
        if let Some(readout_id) = &readout.readout_id {
            check_text(readout_id, "读数 ID", 160)?;
            if !current_readout_ids.insert(readout_id.as_str()) {
                return Err(CliError::input(format!("同一事件不能重复使用读数 ID：{readout_id}")));
            }
            if known_readouts.contains_key(readout_id) {
                return Err(CliError::input(format!("读数 ID 已属于本次 Run：{readout_id}")));
            }
            known_readouts.insert(readout_id.clone(), readout.result_kind.clone());
        }
        if let Some(qualifier) = &readout.qualifier {
            if !["not_detected", "below_lod", "above_uloq", "invalid", "other"].contains(&qualifier.as_str()) {
                return Err(CliError::input("readout.qualifier 不受支持"));
            }
        }
        if readout.result_kind == "raw" && !readout.derived_from.is_empty() {
            return Err(CliError::input("原始读数不能填写 derived_from"));
        }
        if readout.result_kind == "derived" {
            if readout.method.as_deref().is_none_or(|value| value.trim().is_empty()) {
                return Err(CliError::input("派生读数必须填写计算方法"));
            }
            if readout.derived_from.is_empty() {
                return Err(CliError::input("派生读数必须列出来源读数或事件"));
            }
            for source in &readout.derived_from {
                match known_readouts.get(source) {
                    Some(kind) if kind == "raw" => {}
                    Some(_) => {
                        return Err(CliError::input(format!(
                            "派生读数的来源必须是原始读数：{source}"
                        )))
                    }
                    None => {
                        return Err(CliError::input(format!(
                            "派生读数的来源不存在或不属于本次 Run：{source}"
                        )))
                    }
                }
            }
        }
        if let Some(method) = &readout.method {
            check_text(method, "读数方法", 300)?;
        }
        if readout.derived_from.len() > 100 {
            return Err(CliError::input("派生读数的来源最多 100 项"));
        }
        for source in &readout.derived_from {
            check_text(source, "读数来源", 160)?;
        }
        if let Some(unit) = &readout.unit {
            if unit.chars().count() > 40 {
                return Err(CliError::input("读数单位不能超过 40 个字符"));
            }
        }
        if let Some(sample_id) = &readout.sample_id {
            check_text(sample_id, "读数样本编号", 120)?;
        }
    }
    if event.actual_parameters.len() > 100 {
        return Err(CliError::input("实际参数最多 100 项"));
    }
    for parameter in &event.actual_parameters {
        check_text(&parameter.key, "实际参数键", 120)?;
        if parameter.value.is_null() {
            return Err(CliError::input("实际参数值不能为 null"));
        }
        if let Some(unit) = &parameter.unit {
            if unit.chars().count() > 40 {
                return Err(CliError::input("实际参数单位不能超过 40 个字符"));
            }
        }
        if let Some(source) = &parameter.source {
            if source.chars().count() > 300 {
                return Err(CliError::input("实际参数来源不能超过 300 个字符"));
            }
        }
    }
    for check in &event.checks {
        check_text(&check.check_id, "检查项 ID", 80)?;
        if !["passed", "failed", "blocked", "skipped"].contains(&check.status.as_str()) {
            return Err(CliError::input("check.status 必须是 passed、failed、blocked 或 skipped"));
        }
        if let Some(note) = &check.note {
            if note.chars().count() > 2000 {
                return Err(CliError::input("检查说明不能超过 2000 个字符"));
            }
        }
        if let Some(checked_by) = &check.checked_by {
            check_text(checked_by, "检查人", 160)?;
        }
    }
    for control in &event.controls {
        if !["blank", "negative", "positive", "standard", "vehicle", "untreated", "unstained", "other"].contains(&control.role.as_str()) {
            return Err(CliError::input("control.role 不受支持"));
        }
        if !["passed", "failed", "blocked", "skipped", "not_run"].contains(&control.status.as_str()) {
            return Err(CliError::input("control.status 不受支持"));
        }
        for value in [&control.subtype, &control.control_id, &control.sample_id, &control.experimental_unit, &control.expected, &control.observed, &control.note] {
            if let Some(value) = value {
                if value.chars().count() > 500 {
                    return Err(CliError::input("对照字段不能超过 500 个字符"));
                }
            }
        }
    }
    for edge in &event.lineage {
        for value in [&edge.from_kind, &edge.from_id, &edge.to_kind, &edge.to_id, &edge.relation] {
            check_text(value, "lineage 字段", 120)?;
        }
        if let Some(note) = &edge.note {
            if note.chars().count() > 500 {
                return Err(CliError::input("lineage 说明不能超过 500 个字符"));
            }
        }
    }
    if event.event_type != "measurement" && (!event.readouts.is_empty()) {
        return Err(CliError::input("结构化读数请使用 measurement 事件类型"));
    }
    if event.event_type == "skipped_by_rule" {
        if event.step_status.as_deref() != Some("skipped") {
            return Err(CliError::input("skipped_by_rule event requires step_status=skipped"));
        }
        if event.protocol_step_path.as_deref().is_none_or(str::is_empty) {
            return Err(CliError::input("skipped_by_rule event requires protocol_step_path"));
        }
    }
    match event.event_type.as_str() {
        "material_use" if event.materials.is_empty() => {
            return Err(CliError::input("材料使用事件至少需要一项材料"));
        }
        "measurement" if event.measurements.is_empty() && event.readouts.is_empty() && event.result_cells.is_empty() => {
            return Err(CliError::input("测量事件至少需要一项数值读数或结构化读数"));
        }
        "deviation"
            if event
                .deviation
                .as_deref()
                .is_none_or(|v| v.trim().is_empty()) =>
        {
            return Err(CliError::input("偏差事件必须说明实际偏差内容"));
        }
        _ => {}
    }
    if event.event_type != "measurement" && !event.measurements.is_empty() {
        return Err(CliError::input("数值读数请使用 measurement 事件类型"));
    }
    if event.event_type != "material_use" && !event.materials.is_empty() {
        return Err(CliError::input("材料使用请使用 material_use 事件类型"));
    }
    if let Some(deviation) = &event.deviation {
        if deviation.chars().count() > 5000 {
            return Err(CliError::input("偏差说明不能超过 5000 个字符"));
        }
    }
    if event.step_status.as_deref() == Some("completed") {
        if let Some(frozen_step) = frozen_step_for_event(&run_data, &event) {
            let blocking_checks: Vec<&str> = frozen_step
                .get("checks")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .filter(|check| {
                    check
                        .get("blocking")
                        .and_then(Value::as_bool)
                        .unwrap_or(false)
                })
                .filter_map(|check| check.get("check_id").and_then(Value::as_str))
                .collect();
            if !blocking_checks.is_empty() {
                let mut provided: std::collections::HashMap<&str, &str> =
                    std::collections::HashMap::new();
                for check in &event.checks {
                    provided.insert(check.check_id.as_str(), check.status.as_str());
                }
                for check_id in blocking_checks {
                    match provided.get(check_id) {
                        Some(&"passed") => {}
                        Some(_) => {
                            return Err(CliError::input(format!(
                                "完成步骤前，blocking 检查项 {check_id} 必须为 passed"
                            )))
                        }
                        None => {
                            return Err(CliError::input(format!(
                                "完成步骤前，缺少 blocking 检查项 {check_id}"
                            )))
                        }
                    }
                }
            }
        }
    }
    if event.step_status.as_deref() == Some("completed") {
        let mut stmt = tx
            .prepare(
                "SELECT id,project_id,parent_id,revision,data_json,created_at,updated_at \
                 FROM records WHERE kind='execution_event' AND parent_id=?1",
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
        let rows = stmt
            .query_map([run_id], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, Option<String>>(1)?,
                    row.get::<_, Option<String>>(2)?,
                    row.get::<_, i64>(3)?,
                    row.get::<_, String>(4)?,
                    row.get::<_, String>(5)?,
                    row.get::<_, String>(6)?,
                ))
            })
            .map_err(|e| CliError::internal(e.to_string()))?;
        for row in rows {
            let (id, project, parent, revision, raw, created, updated) =
                row.map_err(|e| CliError::internal(e.to_string()))?;
            let data: Value = serde_json::from_str(&raw)
                .map_err(|e| CliError::internal(e.to_string()))?;
            if data.get("step_status").and_then(Value::as_str) != Some("completed") {
                continue;
            }
            if data.get("protocol_step_path").and_then(Value::as_str)
                != event.protocol_step_path.as_deref()
            {
                continue;
            }
            if data.get("timepoint").and_then(Value::as_str) != event.timepoint.as_deref() {
                continue;
            }
            if data.get("assay_id").and_then(Value::as_str) != event.assay_id.as_deref() {
                continue;
            }
            if data.get("planned_output_id").and_then(Value::as_str)
                != event.planned_output_id.as_deref()
            {
                continue;
            }
            if let Some(task_key) = &event.task_key {
                if data.get("task_key").and_then(Value::as_str) != Some(task_key.as_str()) {
                    continue;
                }
            }
            // A completion belongs to samples and one attempt, not to the whole
            // step: completing S1 must not suppress the record for S2.
            let mut stored_samples: Vec<&str> = data["sample_ids"]
                .as_array()
                .into_iter()
                .flatten()
                .filter_map(Value::as_str)
                .map(str::trim)
                .collect();
            let mut new_samples: Vec<&str> = event
                .sample_ids
                .iter()
                .map(String::as_str)
                .map(str::trim)
                .collect();
            stored_samples.sort_unstable();
            new_samples.sort_unstable();
            if stored_samples != new_samples {
                continue;
            }
            if data.get("attempt").and_then(Value::as_u64) != event.attempt.map(u64::from) {
                continue;
            }
            return Ok(json!({
                "id": id,
                "kind": "execution_event",
                "project_id": project,
                "parent_id": parent,
                "revision": revision,
                "data": data,
                "created_at": created,
                "updated_at": updated
            }));
        }
    }
    if let Some(attempt) = event.attempt {
        let mut stmt = tx
            .prepare("SELECT id, data_json FROM records WHERE kind='execution_event' AND parent_id=?1")
            .map_err(|e| CliError::internal(e.to_string()))?;
        let rows = stmt
            .query_map([run_id], |row| {
                Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))
            })
            .map_err(|e| CliError::internal(e.to_string()))?;
        let mut existing = Vec::new();
        for row in rows {
            let (id, raw) = row.map_err(|e| CliError::internal(e.to_string()))?;
            let mut data: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            data["id"] = json!(id);
            existing.push(asterlab_core::sample_progress::ProgressEvent::from_stored(
                &data,
            ));
        }
        asterlab_core::sample_progress::plan_event_identity(
            run_id,
            event.protocol_step_path.as_deref(),
            event.timepoint.as_deref(),
            &event.sample_ids,
            Some(attempt),
            &existing,
        )
        .map_err(|issues| {
            CliError::input(format!(
                "样本执行身份无效：{}",
                issues
                    .iter()
                    .map(|issue| issue.message.as_str())
                    .collect::<Vec<_>>()
                    .join("；")
            ))
        })?;
    }
    let occurred_at = event.occurred_at.clone().unwrap_or_else(stamp);
    let data = json!({
        "event_type":event.event_type,
        "content":event.content,
        "occurred_at":occurred_at,
        "recorded_at":stamp(),
        "stage":event.stage,
        "timepoint":event.timepoint,
        "step_status":event.step_status,
        "ad_hoc_step":event.ad_hoc_step,
        "protocol_step_path":event.protocol_step_path,
        "protocol_version_id":event.protocol_version_id,
        "assay_id":event.assay_id,
        "planned_output_id":event.planned_output_id,
        "pantry_item_id":event.pantry_item_id,
        "task_key":event.task_key,
        "sample_ids":event.sample_ids,
        "parent_sample_id":event.parent_sample_id,
        "aliquot_from":event.aliquot_from,
        "blind_masked":event.blind_masked,
        "attempt":event.attempt,
        "materials":event.materials.iter().map(|m|json!({"material":m.material,"planned_output_id":m.planned_output_id,"amount":m.amount,"unit":m.unit})).collect::<Vec<_>>(),
        "measurements":event.measurements.iter().map(|m|json!({"endpoint":m.endpoint,"value":m.value,"unit":m.unit,"sample_id":m.sample_id,"row":m.row,"column":m.column,"group":m.group,"replicate":m.replicate,"status":m.status,"field":m.field})).collect::<Vec<_>>(),
        "batch_id":event.batch_id,
        "schema_id":event.schema_id,
        "schema_version":event.schema_version,
        "result_cells":event.result_cells,
        "readouts":event.readouts.iter().map(|r|json!({"readout_id":r.readout_id,"endpoint":r.endpoint,"value":r.value,"qualifier":r.qualifier,"unit":r.unit,"sample_id":r.sample_id,"result_kind":r.result_kind,"method":r.method,"derived_from":r.derived_from})).collect::<Vec<_>>(),
        "checks":event.checks.iter().map(|c|json!({"check_id":c.check_id,"status":c.status,"note":c.note,"checked_by":c.checked_by})).collect::<Vec<_>>(),
        "check_ack":event.check_ack,
        "controls":event.controls.iter().map(|c|json!({"role":c.role,"subtype":c.subtype,"status":c.status,"control_id":c.control_id,"sample_id":c.sample_id,"experimental_unit":c.experimental_unit,"expected":c.expected,"observed":c.observed,"note":c.note})).collect::<Vec<_>>(),
        "lineage":event.lineage.iter().map(|l|json!({"from_kind":l.from_kind,"from_id":l.from_id,"to_kind":l.to_kind,"to_id":l.to_id,"relation":l.relation,"note":l.note})).collect::<Vec<_>>(),
        "actual_parameters":event.actual_parameters.iter().map(|p|json!({"key":p.key,"value":p.value,"unit":p.unit,"source":p.source})).collect::<Vec<_>>(),
        "operator":event.operator,
        "deviation":event.deviation,
        "operation_id":event.operation_id,
        "operation_payload":event.operation_payload
    });
    let event_record = insert(
        tx,
        "execution_event",
        project.as_deref(),
        Some(run_id),
        1,
        &data,
    )?;
    refresh_run_status_in_transaction(tx, run_id)?;
    Ok(event_record)
}

