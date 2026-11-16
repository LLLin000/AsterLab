use super::*;

pub(crate) fn parse_input<T: DeserializeOwned>(path: &str) -> Result<T, CliError> {
    let mut raw = String::new();
    if path == "-" {
        io::stdin()
            .take(INPUT_LIMIT + 1)
            .read_to_string(&mut raw)
            .map_err(|e| CliError::input(e.to_string()))?;
    } else {
        let meta =
            fs::metadata(path).map_err(|e| CliError::input(format!("无法读取输入文件：{e}")))?;
        if !meta.is_file() {
            return Err(CliError::input("输入路径必须是文件"));
        }
        if meta.len() > INPUT_LIMIT {
            return Err(CliError::input("JSON 输入超过 2 MiB 上限"));
        }
        raw = fs::read_to_string(path)
            .map_err(|e| CliError::input(format!("输入必须为 UTF-8 文本：{e}")))?;
    }
    if raw.len() as u64 > INPUT_LIMIT {
        return Err(CliError::input("JSON 输入超过 2 MiB 上限"));
    }
    serde_json::from_str(&raw)
        .map_err(|e| CliError::input(format!("JSON 输入不符合该工具的结构：{e}")))
}
#[derive(Debug)]
pub(crate) struct Invocation<'a> {
    pub(crate) input: &'a str,
    /// `--diagnostics json` was requested; envelopes then carry a structured
    /// `diagnostics` array.
    pub(crate) diagnostics: bool,
}

pub(crate) fn parse_args(args: &[String]) -> Result<Invocation<'_>, CliError> {
    let mut input: Option<&str> = None;
    let mut diagnostics = false;
    let mut index = 0;
    while index < args.len() {
        match args[index].as_str() {
            "--input" => {
                let Some(value) = args.get(index + 1) else {
                    return Err(CliError::input("--input 需要 JSON 文件路径或 -"));
                };
                if input.replace(value.as_str()).is_some() {
                    return Err(CliError::input("--input 只能出现一次"));
                }
                index += 2;
            }
            "--diagnostics" => {
                let Some(value) = args.get(index + 1) else {
                    return Err(CliError::input("--diagnostics 需要参数：json"));
                };
                if value != "json" {
                    return Err(CliError::input(format!(
                        "--diagnostics 只支持 json，收到：{value}"
                    )));
                }
                diagnostics = true;
                index += 2;
            }
            other => {
                return Err(CliError::input(format!(
                    "未知参数：{other}；格式：labworkbench <tool> --input <json-file|-> [--diagnostics json]"
                )));
            }
        }
    }
    let Some(input) = input else {
        return Err(CliError::input("命令格式：labworkbench <tool> --input <json-file|->；用 `labworkbench tools --json` 查看工具，用 `labworkbench schema <tool>` 查看字段定义"));
    };
    Ok(Invocation { input, diagnostics })
}
pub(crate) fn output(tool: &str, result: Value, diagnostics: bool) {
    println!("{}", success_envelope(tool, result, diagnostics));
}

pub(crate) fn success_envelope(tool: &str, result: Value, diagnostics: bool) -> Value {
    let derived = if diagnostics {
        result_diagnostics(&result)
    } else {
        Vec::new()
    };
    let mut envelope = json!({"ok":true,"tool":tool,"result":result});
    if diagnostics {
        envelope["diagnostics"] = Value::Array(derived);
    }
    envelope
}

/// Read-only validations (`agent_edit.validate`, `semantic.validate`, …) return
/// their findings inside a successful result. Surface the same entries as
/// structured diagnostics so one flag covers both envelopes.
pub(crate) fn result_diagnostics(result: &Value) -> Vec<Value> {
    result
        .get("issues")
        .and_then(Value::as_array)
        .map(|issues| {
            issues
                .iter()
                .map(Diagnostic::from_legacy_json)
                .filter_map(|diagnostic| serde_json::to_value(diagnostic).ok())
                .collect()
        })
        .unwrap_or_default()
}
pub(crate) fn ensure_project(db: &Connection, id: &str) -> Result<(), CliError> {
    if !has_kind(db, id, "project")? {
        return Err(CliError::missing(format!("项目不存在：{id}")));
    }
    Ok(())
}
pub(crate) fn update_data(
    db: &Connection,
    id: &str,
    expected: i64,
    expected_kind: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let (kind, _, _, revision, _) =
        kind_id(db, id)?.ok_or_else(|| CliError::missing(format!("记录不存在：{id}")))?;
    if kind != expected_kind {
        return Err(CliError::input(format!("目标必须是 {expected_kind} 记录")));
    }
    if revision != expected {
        return Err(CliError::conflict(format!(
            "记录版本已从 v{expected} 更新为 v{revision}；请先重新读取。"
        )));
    }
    let next = revision + 1;
    let tx = db
        .unchecked_transaction()
        .map_err(|e| CliError::internal(e.to_string()))?;
    let changed = tx
        .execute(
            "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1 AND revision=?5",
            params![
                id,
                next,
                serde_json::to_string(data).map_err(|e| CliError::internal(e.to_string()))?,
                stamp(),
                revision
            ],
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    if changed != 1 {
        return Err(CliError::conflict("记录已被其他操作更新；请重新读取。"));
    }
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    get(db, id)
}
pub(crate) fn set_status(
    db: &Connection,
    i: StatusUpdate,
    kind: &str,
    allowed: &[&str],
) -> Result<Value, CliError> {
    if !allowed.contains(&i.status.as_str()) {
        return Err(CliError::input(format!(
            "status 不属于该记录允许的状态：{}",
            allowed.join(", ")
        )));
    }
    let (actual, _, _, revision, raw) =
        kind_id(db, &i.id)?.ok_or_else(|| CliError::missing(format!("记录不存在：{}", i.id)))?;
    if actual != kind {
        return Err(CliError::input(format!("目标必须是 {kind} 记录")));
    }
    if revision != i.expected_revision {
        return Err(CliError::conflict(format!(
            "记录版本已从 v{} 更新为 v{revision}；请先重新读取。",
            i.expected_revision
        )));
    }
    let mut data: Value =
        serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    data["status"] = json!(i.status);
    update_data(db, &i.id, i.expected_revision, kind, &data)
}
pub(crate) fn concentration_factor(unit: &str) -> Result<f64, CliError> {
    match unit {
        "M" => Ok(1.0),
        "mM" => Ok(1e-3),
        "µM" => Ok(1e-6),
        "nM" => Ok(1e-9),
        _ => Err(CliError::input("浓度单位只能是 M、mM、µM、nM")),
    }
}
/// Concentration units in one of the two dimensions the CLI speaks.
///
/// Molarity and mass concentration never convert into each other here: a
/// molecular weight is missing, and `calculator.solution` is the tool that owns
/// that conversion. `Dilution` therefore only allows a same-dimension pair.
pub(crate) enum ConcentrationScale {
    Molar(f64),
    Mass(f64),
}

impl ConcentrationScale {
    /// Factor to the dimension's base unit (`M` or `ng/µL`).
    pub(crate) fn factor(&self) -> f64 {
        match self {
            Self::Molar(factor) | Self::Mass(factor) => *factor,
        }
    }

    pub(crate) fn is_mass(&self) -> bool {
        matches!(self, Self::Mass(_))
    }
}

pub(crate) fn concentration_scale(unit: &str) -> Result<ConcentrationScale, CliError> {
    if let Ok(factor) = concentration_factor(unit) {
        return Ok(ConcentrationScale::Molar(factor));
    }
    match unit.parse::<MassConcentrationUnit>() {
        Ok(mass) => Ok(ConcentrationScale::Mass(mass.ng_per_ul_factor())),
        Err(_) => Err(CliError::input(
            "浓度单位只能是 M、mM、µM、nM 或 mg/mL、µg/mL、ng/µL",
        )),
    }
}

pub(crate) fn mass_concentration_unit(unit: &str) -> Result<MassConcentrationUnit, CliError> {
    unit.parse::<MassConcentrationUnit>()
        .map_err(CliError::input)
}
pub(crate) fn volume_factor(unit: &str) -> Result<f64, CliError> {
    match unit {
        "L" => Ok(1.0),
        "mL" => Ok(1e-3),
        "µL" => Ok(1e-6),
        _ => Err(CliError::input("体积单位只能是 L、mL、µL")),
    }
}

// Derived mix values are exact f64 arithmetic; round only at the CLI boundary
// so JSON reads as 2030 instead of 2030.0000000000002. 1e-6 uL is 1 nL.
pub(crate) fn display_quantity(value: f64) -> f64 {
    (value * 1e6).round() / 1e6
}

pub(crate) fn create_project(db: &Connection, name: &str) -> Result<Value, CliError> {
    let tx = db.unchecked_transaction().map_err(|e| CliError::internal(e.to_string()))?;
    let r = insert(&tx, "project", None, None, 1, &json!({"name": name, "description": ""}))?;
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    Ok(r)
}

pub(crate) fn list_designs(db: &Connection, project_id: Option<&str>) -> Result<Value, CliError> {
    Ok(json!({"records": list(db, Some("design"), project_id)?}))
}

pub(crate) fn load_design(db: &Connection, design_id: &str) -> Result<Value, CliError> {
    let r = get(db, design_id)?;
    if r["kind"] != "design" {
        return Err(CliError::input("指定记录不是实验设计"));
    }
    let revision = r["revision"]
        .as_i64()
        .ok_or_else(|| CliError::internal("设计版本号无效"))?;
    let version_id: String = db
        .query_row(
            "SELECT id FROM records WHERE kind='design_version' AND parent_id=?1 AND revision=?2",
            params![design_id, revision],
            |row| row.get(0),
        )
        .optional()
        .map_err(|e| CliError::internal(e.to_string()))?
        .ok_or_else(|| CliError::internal("当前设计版本快照缺失"))?;
    Ok(json!({"design": r, "version": get(db, &version_id)?}))
}

pub(crate) fn take_mygo_extension(value: &mut Value) -> Option<Value> {
    value.as_object_mut()?.remove("mygo")
}

pub(crate) fn create_design_record(
    db: &Connection,
    project_id: &str,
    design: &Value,
) -> Result<Value, CliError> {
    ensure_project(db, project_id)?;
    let mut input = design.clone();
    let mygo = take_mygo_extension(&mut input);
    let design_fields: DesignFields = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("设计字段无效：{e}")))?;
    let tx = db
        .unchecked_transaction()
        .map_err(|e| CliError::internal(e.to_string()))?;
    let registry = load_labware_registry(db)?;
    let mut r = create_design_tx(&tx, project_id, &design_fields, &registry)?;
    if let Some(mygo) = mygo {
        let id = r["design"]["id"].as_str().unwrap_or_default();
        let mut stored = r["design"]["data"].clone();
        stored["mygo"] = mygo;
        let raw = serde_json::to_string(&stored).map_err(|e| CliError::internal(e.to_string()))?;
        tx.execute("UPDATE records SET data_json=?2 WHERE id=?1", params![id, raw])
            .map_err(|e| CliError::internal(e.to_string()))?;
        let version_id = r["version"]["id"].as_str().unwrap_or_default();
        let mut version = r["version"]["data"].clone();
        version["snapshot"] = stored.clone();
        let version_raw = serde_json::to_string(&version).map_err(|e| CliError::internal(e.to_string()))?;
        tx.execute("UPDATE records SET data_json=?2 WHERE id=?1", params![version_id, version_raw])
            .map_err(|e| CliError::internal(e.to_string()))?;
        r["design"]["data"] = stored;
    }
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    Ok(r)
}

pub(crate) fn revise_design_record(
    db: &Connection,
    id: &str,
    expected_revision: i64,
    changes: &Value,
) -> Result<Value, CliError> {
    let (kind, project, _parent, revision, raw) = kind_id(db, id)?
        .ok_or_else(|| CliError::missing(format!("设计不存在：{id}")))?;
    if kind != "design" {
        return Err(CliError::input("指定记录不是实验设计"));
    }
    let document_versions: i64 = db
        .query_row(
            "SELECT COUNT(*) FROM records WHERE kind='asterdoc_version' AND parent_id=?1",
            [id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    if document_versions > 0 {
        return Err(CliError::input(
            "该设计由 AsterDoc 文档管理；请保存新的 AsterDoc 版本，而不是修改派生设计字段",
        ));
    }
    if revision != expected_revision {
        return Err(CliError::conflict(format!(
            "设计版本已从 v{} 更新为 v{}；先重新读取后再提交。",
            expected_revision, revision
        )));
    }
    let mut value: Value =
        serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    let mut mygo = take_mygo_extension(&mut value);
    let previous: DesignFields = serde_json::from_value(value.clone())
        .map_err(|e| CliError::internal(format!("现有设计无法读取：{e}")))?;
    let mut change_value = changes.clone();
    if let Some(extension) = take_mygo_extension(&mut change_value) {
        mygo = Some(extension);
    }
    let changeset: DesignChanges = serde_json::from_value(change_value)
        .map_err(|e| CliError::input(format!("changes 无效：{e}")))?;
    merge_design(&mut value, &changeset);
    let mut d: DesignFields = serde_json::from_value(value)
        .map_err(|e| CliError::input(format!("修改后设计无效：{e}")))?;
    normalize_design(&mut d, Some(&previous));
    validate_design_with_registry(&d, &load_labware_registry(db)?)?;
    let next = revision + 1;
    let tx = db
        .unchecked_transaction()
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut stored = serde_json::to_value(&d).map_err(|e| CliError::internal(e.to_string()))?;
    if let Some(mygo) = mygo {
        stored["mygo"] = mygo;
    }
    let newdata = serde_json::to_string(&stored).map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute(
        "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1",
        params![id, next, newdata, stamp()],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    let snapshot = insert(
        &tx,
        "design_version",
        project.as_deref(),
        Some(id),
        next,
        &json!({"design_id": id, "design_revision": next, "snapshot": stored}),
    )?;
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    let r = get(db, id)?;
    Ok(json!({"design": r, "version": snapshot}))
}

pub(crate) fn create_run_for_design(
    db: &mut Connection,
    design_id: &str,
    title: Option<&str>,
    operation_id: Option<&str>,
) -> Result<Value, CliError> {
    let design = get(db, design_id)?;
    if design["kind"] != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let default_title = design["data"]["title"]
        .as_str()
        .unwrap_or("Run")
        .to_string()
        + " run";
    let title_str = title.unwrap_or(&default_title);
    let op = operation_id.filter(|s| !s.trim().is_empty()).map(str::to_owned);

    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;

    if let Some(op_id) = &op {
        let found: Option<(String, String)> = {
            let mut stmt = tx
                .prepare("SELECT id, data_json FROM records WHERE kind='run' AND json_extract(data_json, '$.operation_id')=?1")
                .map_err(|e| CliError::internal(e.to_string()))?;
            let rows = stmt
                .query_map([op_id], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?)))
                .map_err(|e| CliError::internal(e.to_string()))?;
            let mut out = None;
            for row in rows {
                let (id, raw) = row.map_err(|e| CliError::internal(e.to_string()))?;
                out = Some((id, raw));
                break;
            }
            out
        };
        if let Some((existing_id, raw)) = found {
            let existing_data: Value = serde_json::from_str(&raw).unwrap_or(Value::Null);
            let existing_payload = existing_data.get("operation_payload").cloned().unwrap_or(Value::Null);
            let request_payload = json!({"design_id": design_id, "title": title_str});
            if existing_payload == request_payload {
                tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
                return get(db, &existing_id);
            }
            tx.rollback().ok();
            return Err(CliError::conflict("operation id conflict: same operation_id with different payload"));
        }
    }

    let record = create_run_in_transaction(
        &tx,
        design_id,
        title_str,
        None,
        None,
        None,
        None,
        "independent",
        op.as_deref(),
    )?;
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    Ok(record)
}


/// create_run_from_asterdoc compiles one immutable document revision into the
/// Design projection, freezes that projection, and creates the Run in one
/// transaction. The document version and source hash are stored on the Run.

/// assess_asterdoc returns the Core-derived design readiness. It never trusts
/// a GUI-only rule: the same parser, compiler and run audit are used.
pub(crate) fn assess_asterdoc(db: &Connection, input: &Value) -> Result<Value, CliError> {
    let design_id = input.get("design_id").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("design_id is required"))?;
    let revision = input.get("revision").and_then(Value::as_u64).unwrap_or(0);
    let doc = if revision > 0 {
        load_asterdoc_at(db, design_id, revision)?
    } else {
        load_asterdoc(db, design_id)?
    };
    let source = doc["source"].as_str().unwrap_or("");
    let mut errors: Vec<String> = Vec::new();
    let mut warnings: Vec<String> = Vec::new();
    if source.trim().is_empty() {
        errors.push("文档还是空的".to_owned());
    } else if let Err(error) = validate_asterdoc_source(source) {
        errors.push(error.message);
    } else {
        match compile_asterdoc_to_design(source) {
            Err(error) => errors.push(error.message),
            Ok(design) => {
                match run_creation_audit(&design, 0, "") {
                    Err(error) => errors.push(error.message),
                    Ok(audit) => {
                        for warning in audit["warnings"].as_array().into_iter().flatten() {
                            if let Some(warning) = warning.as_str() {
                                warnings.push(warning.to_owned());
                            }
                        }
                    }
                }
                let assays = design["assays"].as_array().cloned().unwrap_or_default();
                if assays.is_empty() {
                    errors.push("还没有检测（assay）".to_owned());
                }
                for assay in &assays {
                    let id = assay["id"].as_str().unwrap_or("");
                    if assay["protocol_version_id"].as_str().unwrap_or("").is_empty() {
                        errors.push(format!("检测 {id} 没有绑定 protocol 版本，Run 无法生成可执行任务"));
                    }
                }
                match asterdoc_step_materialization_issues(db, design_id, &design) {
                    Ok(issues) => errors.extend(issues),
                    Err(error) => errors.push(error.message),
                }
                let groups = design["group_specs"].as_array().cloned().unwrap_or_default();
                if groups.is_empty() {
                    errors.push("还没有分组".to_owned());
                }
            }
        }
    }
    errors.sort();
    errors.dedup();
    warnings.sort();
    warnings.dedup();
    let state = if errors.is_empty() && warnings.is_empty() {
        "ready"
    } else if errors.is_empty() {
        "warning"
    } else {
        "incomplete"
    };
    Ok(json!({
        "design_id": design_id,
        "revision": doc["revision"].as_u64().unwrap_or(0),
        "source_hash": doc["source_hash"].as_str().unwrap_or(""),
        "state": state,
        "errors": errors,
        "warnings": warnings
    }))
}

pub(crate) fn create_run_from_asterdoc(
    db: &mut Connection,
    design_id: &str,
    document_revision: u64,
    operation_id: Option<&str>,
    title: Option<&str>,
    acknowledged_warnings: &[String],
) -> Result<Value, CliError> {
    let document = load_asterdoc_at(db, design_id, document_revision)?;
    let source = document["source"].as_str().unwrap_or("");
    let source_hash = document["source_hash"].as_str().unwrap_or("").to_owned();
    let compiler_version = document["compiler_version"].as_str().unwrap_or("").to_owned();
    let schema_version = document["schema_version"].as_u64().unwrap_or(0);
    let design = compile_asterdoc_to_design(source)?;
    let design = serde_json::to_string(&design)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let op = operation_id.filter(|value| !value.trim().is_empty());

    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;

    if let Some(op_id) = op {
        let found: Option<(String, String)> = {
            let mut stmt = tx
                .prepare("SELECT id, data_json FROM records WHERE kind='run' AND json_extract(data_json, '$.operation_id')=?1")
                .map_err(|e| CliError::internal(e.to_string()))?;
            let rows = stmt
                .query_map([op_id], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?)))
                .map_err(|e| CliError::internal(e.to_string()))?;
            let mut out = None;
            for row in rows.flatten() {
                out = Some(row);
                break;
            }
            out
        };
        if let Some((existing_id, raw)) = found {
            let existing: Value = serde_json::from_str(&raw).unwrap_or(Value::Null);
            let same = existing["asterdoc"]["revision"].as_u64() == Some(document_revision)
                && existing["asterdoc"]["source_hash"].as_str() == Some(source_hash.as_str());
            if same {
                tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
                return get(db, &existing_id);
            }
            return Err(CliError::conflict(
                "operation id conflict: same operation_id with another AsterDoc revision",
            ));
        }
    }

    let (kind, project, _, revision, _) =
        kind_id(&tx, design_id)?.ok_or_else(|| CliError::missing("实验设计不存在"))?;
    if kind != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let project = project.ok_or_else(|| CliError::internal("实验设计缺少项目归属"))?;
    let next = revision + 1;
    tx.execute(
        "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1",
        params![design_id, next, design, stamp()],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    insert(
        &tx,
        "design_version",
        Some(&project),
        Some(design_id),
        next,
        &json!({"design_id": design_id, "design_revision": next, "snapshot": serde_json::from_str::<Value>(&design).unwrap_or(Value::Null)}),
    )?;
    let default_title = serde_json::from_str::<Value>(&design)
        .ok()
        .and_then(|value| value["title"].as_str().map(str::to_owned))
        .unwrap_or_else(|| "Run".to_owned())
        + " run";
    let title_str = title.unwrap_or(&default_title);
    let run = create_run_in_transaction(
        &tx,
        design_id,
        title_str,
        None,
        None,
        None,
        None,
        "independent",
        op,
    )?;
    let run_id = run["id"].as_str().unwrap_or_default().to_owned();
    let run_raw: String = tx
        .query_row("SELECT data_json FROM records WHERE id=?1", [&run_id], |row| row.get(0))
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut run_data: Value = serde_json::from_str(&run_raw)
        .map_err(|e| CliError::internal(e.to_string()))?;
    if let Some(audit) = run_data.get_mut("creation_audit") {
        let warnings: Vec<String> = audit["warnings"].as_array().into_iter().flatten()
            .filter_map(|value| value.as_str().map(str::to_owned)).collect();
        let acknowledged: Vec<String> = warnings.iter()
            .filter(|warning| acknowledged_warnings.contains(*warning))
            .cloned().collect();
        if !warnings.is_empty() && acknowledged.len() == warnings.len() {
            audit["decision"] = json!("acknowledged_warnings");
            audit["acknowledged_warnings"] = json!(acknowledged);
        }
    }
    run_data["asterdoc"] = json!({
        "design_id": design_id,
        "revision": document_revision,
        "source_hash": source_hash,
        "schema_version": schema_version,
        "compiler_version": compiler_version,
        "acknowledged_warnings": acknowledged_warnings
    });
    tx.execute(
        "UPDATE records SET data_json=?2,updated_at=?3 WHERE id=?1",
        params![run_id, serde_json::to_string(&run_data).unwrap_or_default(), stamp()],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    get(db, &run_id)
}

pub(crate) fn create_design_tx(
    tx: &Transaction<'_>,
    project_id: &str,
    design: &DesignFields,
    registry: &LabwareRegistry,
) -> Result<Value, CliError> {
    let raw = normalize_validate_design_data_with_registry(
        serde_json::to_value(design).map_err(|e| CliError::internal(e.to_string()))?,
        None,
        registry,
    )?;
    let record = insert(tx, "design", Some(project_id), Some(project_id), 1, &raw)?;
    let version = insert(
        tx,
        "design_version",
        Some(project_id),
        record["id"].as_str(),
        1,
        &json!({"design_id":record["id"],"design_revision":1,"snapshot":raw}),
    )?;
    Ok(json!({"design":record,"version":version}))
}
pub(crate) fn merge_design(base: &mut Value, c: &DesignChanges) {
    macro_rules! put {
        ($field:ident) => {
            if let Some(v) = &c.$field {
                base[stringify!($field)] = json!(v)
            }
        };
    }
    put!(title);
    put!(objective);
    put!(research_question);
    put!(factors);
    put!(experimental_unit);
    put!(group_specs);
    put!(timepoints);
    put!(primary_endpoint);
    put!(endpoint_method);
    put!(endpoint_unit);
    put!(sample_size_rationale);
    put!(randomization);
    put!(blinding);
    put!(inclusion_criteria);
    put!(exclusion_criteria);
    put!(analysis_plan);
    put!(controls);
    put!(assays);
    put!(sample_allocations);
    put!(planned_outputs);
    put!(container_plan);
    put!(source_evidence);
    put!(evidence_conflicts);
    put!(planned_run_count);
    put!(planned_run_relationship);
    put!(groups);
    put!(hypothesis);
    put!(replicate_plan);
    put!(independent_variables);
}

