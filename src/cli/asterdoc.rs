use super::*;

// --- AsterDoc document store -------------------------------------------------

pub(crate) const ASTERDOC_SCHEMA_VERSION: u64 = 1;
pub(crate) const ASTERDOC_COMPILER_VERSION: &str = "asterdoc-core-0.1.0";

/// save_asterdoc_version appends one immutable document revision. Parsing and
/// validation happen before the transaction writes; the design link and its
/// project must already exist.
pub(crate) fn save_asterdoc_version(db: &mut Connection, input: &Value) -> Result<Value, CliError> {
    let design_id = input.get("design_id").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("design_id is required"))?;
    let source = input.get("source").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("source is required"))?;
    if source.len() > 400_000 {
        return Err(CliError::input("AsterDoc source cannot exceed 400000 bytes"));
    }
    let expected = input.get("expected_revision").and_then(Value::as_u64).unwrap_or(u64::MAX);
    let operation_id = input.get("operation_id").and_then(Value::as_str)
        .filter(|v| !v.trim().is_empty());
    let actor_id = input.get("actor_id").and_then(Value::as_str)
        .filter(|v| !v.trim().is_empty());
    validate_asterdoc_source(source)?;

    let (kind, project, _, design_revision, _) =
        kind_id(db, design_id)?.ok_or_else(|| CliError::missing("实验设计不存在"))?;
    if kind != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let project = project.ok_or_else(|| CliError::internal("实验设计缺少项目归属"))?;

    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;

    if let Some(op) = operation_id {
        let existing: Option<Value> = {
            let mut stmt = tx.prepare(
                "SELECT data_json FROM records WHERE kind='asterdoc_version' AND parent_id=?1",
            ).map_err(|e| CliError::internal(e.to_string()))?;
            let rows = stmt.query_map([design_id], |row| row.get::<_, String>(0))
                .map_err(|e| CliError::internal(e.to_string()))?;
            let mut found = None;
            for row in rows.flatten() {
                let data: Value = serde_json::from_str(&row).unwrap_or(Value::Null);
                if data.get("operation_id").and_then(Value::as_str) == Some(op) {
                    found = Some(data);
                    break;
                }
            }
            found
        };
        if let Some(data) = existing {
            if data.get("source").and_then(Value::as_str) != Some(source) {
                return Err(CliError::conflict(
                    "operation_id 已用于另一份 AsterDoc 源码".to_owned(),
                ));
            }
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            let mut out = data;
            out["design_id"] = json!(design_id);
            return Ok(out);
        }
    }

    let current: u64 = tx
        .query_row(
            "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='asterdoc_version' AND parent_id=?1",
            [design_id],
            |row| row.get::<_, i64>(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))? as u64;
    if expected != u64::MAX && expected != current {
        return Err(CliError::conflict(format!(
            "AsterDoc 版本已从 v{expected} 更新为 v{current}；请先重新读取。"
        )));
    }
    let revision = current + 1;
    let hash = format!("{:x}", Sha256::digest(source.as_bytes()));
    let created_at = stamp();
    let data = json!({
        "revision": revision,
        "design_id": design_id,
        "design_revision": design_revision,
        "source": source,
        "source_hash": hash,
        "schema_version": ASTERDOC_SCHEMA_VERSION,
        "compiler_version": ASTERDOC_COMPILER_VERSION,
        "operation_id": operation_id,
        "actor_id": actor_id,
        "created_at": created_at
    });
    insert(&tx, "asterdoc_version", Some(&project), Some(design_id), revision as i64, &data)?;
    if let Ok(compiled) = compile_asterdoc_to_design(source) {
        if let Ok(compiled_json) = serde_json::to_string(&compiled) {
            tx.execute(
                "UPDATE records SET data_json=?2,updated_at=?3 WHERE id=?1",
                params![design_id, compiled_json, created_at],
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
        }
    }
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    mirror_asterdoc_to_disk(db, design_id, source);
    Ok(data)
}

fn mirror_asterdoc_to_disk(db: &Connection, design_id: &str, source: &str) {
    let Some(db_path) = db.path().filter(|p| !p.is_empty() && *p != ":memory:") else {
        return;
    };
    let Some(data_dir) = std::path::Path::new(db_path).parent() else {
        return;
    };
    let clean_id = design_id.trim();
    if clean_id.is_empty() || clean_id.contains(['/', '\\', '.']) {
        return;
    }
    let exp_dir = data_dir.join("experiments").join(clean_id);
    if std::fs::create_dir_all(exp_dir.join("assets")).is_err() {
        return;
    }
    let file_path = exp_dir.join("experiment.asterml");
    let mut final_body = source.to_owned();
    if let Ok(prev) = std::fs::read_to_string(&file_path) {
        // Preserve any trailing asterlab-exec / asterlab-record envelope blocks when updating design from CLI.
        let mut suffix_start: Option<usize> = None;
        for marker in ["\n```asterlab-exec", "\n````asterlab-record"] {
            if let Some(pos) = prev.find(marker) {
                suffix_start = Some(match suffix_start {
                    Some(cur) => cur.min(pos),
                    None => pos,
                });
            }
        }
        if let Some(pos) = suffix_start {
            let trimmed_src = source.trim_end();
            final_body = format!("{trimmed_src}\n{}", &prev[pos..]);
        }
    }
    let tmp_path = exp_dir.join(".experiment.asterml.tmp");
    if std::fs::write(&tmp_path, final_body.as_bytes()).is_ok() {
        let _ = std::fs::rename(&tmp_path, &file_path);
    }
}

/// load_asterdoc_at returns one exact immutable revision, for Run freezing and
/// audit. A missing revision is an explicit error, never a fallback to latest.

/// apply_asterdoc_command applies one semantic edit to an exact document
/// revision and then saves through the authoritative revision path.
pub(crate) fn apply_asterdoc_command(db: &mut Connection, input: &Value) -> Result<Value, CliError> {
    let design_id = input.get("design_id").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("design_id is required"))?;
    let expected = input.get("expected_revision").and_then(Value::as_u64)
        .ok_or_else(|| CliError::input("expected_revision is required"))?;
    let command = input.get("command").cloned().unwrap_or(Value::Null);
    let source = if expected == 0 {
        "".to_owned()
    } else {
        load_asterdoc_at(db, design_id, expected)?["source"].as_str().unwrap_or("").to_owned()
    };
    if command.get("type").and_then(Value::as_str) == Some("remove_step") {
        let step_id = command.get("moduleId").and_then(Value::as_str).unwrap_or("");
        let mut statement = db.prepare(
            "SELECT data_json FROM records WHERE kind='run' AND parent_id=?1"
        ).map_err(|error| CliError::internal(error.to_string()))?;
        let rows = statement.query_map([design_id], |row| row.get::<_, String>(0))
            .map_err(|error| CliError::internal(error.to_string()))?;
        for row in rows {
            let data: Value = serde_json::from_str(&row.map_err(|error| CliError::internal(error.to_string()))?)
                .map_err(|error| CliError::internal(error.to_string()))?;
            let frozen_revision = data["asterdoc"]["revision"].as_u64().unwrap_or(0);
            if frozen_revision == 0 { continue; }
            let frozen = load_asterdoc_at(db, design_id, frozen_revision)?;
            if asterdoc_blocks(frozen["source"].as_str().unwrap_or(""))?.iter()
                .any(|block| yaml_kind(&block.value) == "step" && yaml_id(&block.value) == step_id)
            {
                return Err(step_reference_error(step_id));
            }
        }
    }
    let next = apply_asterdoc_patch(&source, &command)?;
    save_asterdoc_version(db, &json!({
        "design_id": design_id,
        "source": next,
        "expected_revision": expected,
        "operation_id": input.get("operation_id"),
        "actor_id": input.get("actor_id")
    }))
}

pub(crate) struct AsterBlock {
    pub(crate) body_start: usize,
    pub(crate) body_end: usize,
    pub(crate) fence_start: usize,
    pub(crate) fence_end: usize,
    pub(crate) value: serde_yaml::Value,
}

fn leading_backticks(trimmed: &str) -> usize {
    let n = trimmed.bytes().take_while(|&b| b == b'`').count();
    if n >= 3 { n } else { 0 }
}

fn is_closing_backtick_fence(trimmed: &str, min_ticks: usize) -> bool {
    min_ticks >= 3 && trimmed.len() >= min_ticks && trimmed.bytes().all(|b| b == b'`')
}

fn frontmatter_end_offset(source: &str) -> usize {
    let mut iter = source.split_inclusive('\n');
    let first = match iter.next() {
        Some(l) => l,
        None => return 0,
    };
    if first.trim_start_matches('\u{feff}').trim() != "---" {
        return 0;
    }
    let mut offset = first.len();
    for line in iter {
        offset += line.len();
        let tr = line.trim();
        if tr == "---" || tr == "..." {
            return offset;
        }
        if tr.starts_with("```") {
            return 0;
        }
    }
    0
}

pub(crate) fn asterdoc_blocks(source: &str) -> Result<Vec<AsterBlock>, CliError> {
    let mut blocks = Vec::new();
    let fm_end = frontmatter_end_offset(source);
    let mut offset = fm_end;
    let mut in_fence = false;
    let mut in_other_fence = false;
    let mut fence_ticks = 0usize;
    let mut body_start = 0usize;
    let mut body = String::new();
    let mut fence_start = 0usize;
    for line in source[fm_end..].split_inclusive('\n') {
        let trimmed = line.trim();
        if !in_fence && !in_other_fence {
            let ticks = leading_backticks(trimmed);
            if ticks >= 3 {
                let lang = trimmed[ticks..].trim();
                if ticks == 3 && lang == "asterlab" {
                    in_fence = true;
                    fence_ticks = ticks;
                    fence_start = offset;
                    body_start = offset + line.len();
                    body.clear();
                } else {
                    in_other_fence = true;
                    fence_ticks = ticks;
                }
            }
        } else if in_other_fence {
            if is_closing_backtick_fence(trimmed, fence_ticks) {
                in_other_fence = false;
                fence_ticks = 0;
            }
        } else if is_closing_backtick_fence(trimmed, fence_ticks) {
            let value: serde_yaml::Value = serde_yaml::from_str(&body)
                .map_err(|e| CliError::input(format!("asterlab block: {e}")))?;
            blocks.push(AsterBlock {
                body_start,
                body_end: offset,
                fence_start,
                fence_end: offset + line.len(),
                value,
            });
            in_fence = false;
            fence_ticks = 0;
            body.clear();
        } else {
            body.push_str(line);
        }
        offset += line.len();
    }
    Ok(blocks)
}

pub(crate) fn yaml_id(value: &serde_yaml::Value) -> String {
    value
        .get("id")
        .and_then(|id| id.as_str())
        .unwrap_or("")
        .to_owned()
}

pub(crate) fn yaml_kind(value: &serde_yaml::Value) -> String {
    value
        .get("kind")
        .and_then(|kind| kind.as_str())
        .unwrap_or("")
        .to_owned()
}

pub(crate) fn append_asterdoc_module(source: &str, module: &serde_yaml::Value) -> Result<String, CliError> {
    let id = yaml_id(module);
    if asterdoc_blocks(source)?.iter().any(|block| yaml_id(&block.value) == id) {
        return Err(CliError::input(format!("module id {id} already exists")));
    }
    let raw = serde_yaml::to_string(module).map_err(|e| CliError::internal(e.to_string()))?;
    let mut next = source.to_owned();
    if !next.is_empty() && !next.ends_with('\n') {
        next.push('\n');
    }
    next.push_str("\n```asterlab\n");
    next.push_str(&raw);
    next.push_str("```\n");
    Ok(next)
}

pub(crate) fn apply_replacements(mut source: String, mut replacements: Vec<(usize, usize, String)>) -> String {
    replacements.sort_by(|a, b| b.0.cmp(&a.0));
    for (start, end, text) in replacements {
        if start <= end && end <= source.len() {
            source.replace_range(start..end, &text);
        }
    }
    source
}

pub(crate) fn step_path(value: &serde_yaml::Value) -> String {
    value.get("path").and_then(|path| path.as_str()).unwrap_or("").trim().to_owned()
}

pub(crate) fn step_path_cli_error(error: ast::StepPathError) -> CliError {
    CliError::input(error.message)
}

/// Apply the path rewrite for one step and, when the step already had a path,
/// for its descendants. A step without a path has no identifiable descendants,
/// so only that block is rewritten.
pub(crate) fn rewrite_step_subtree(
    source: &str,
    step_id: &str,
    old_path: &str,
    new_path: &str,
    extra_paths: &[(String, String)],
) -> Result<String, CliError> {
    let blocks = asterdoc_blocks(source)?;
    if !blocks.iter().any(|block| yaml_kind(&block.value) == "step" && yaml_id(&block.value) == step_id) {
        return Err(CliError::missing(format!("step {step_id} does not exist")));
    }
    let old_path = old_path.trim();
    if new_path.is_empty() {
        return Err(CliError::input(format!(
            "step {step_id} has no target path; retry nest_step or promote_step"
        )));
    }
    let mut replacements = Vec::new();
    for block in &blocks {
        if yaml_kind(&block.value) != "step" {
            continue;
        }
        let id = yaml_id(&block.value);
        let path = step_path(&block.value);
        let replacement = if id == step_id {
            Some(new_path.to_owned())
        } else if !old_path.is_empty()
            && (path == old_path
                || path.strip_prefix(old_path).is_some_and(|suffix| suffix.starts_with('/')))
        {
            Some(format!("{new_path}{}", &path[old_path.len()..]))
        } else {
            extra_paths.iter()
                .find(|(extra_id, _)| *extra_id == id)
                .map(|(_, extra_path)| extra_path.clone())
        };
        let Some(replacement) = replacement else { continue };
        let mut value = block.value.clone();
        let mapping = value.as_mapping_mut()
            .ok_or_else(|| CliError::input(format!("step {id} module must be a mapping")))?;
        mapping.insert(
            serde_yaml::Value::String("path".to_owned()),
            serde_yaml::Value::String(replacement),
        );
        let raw = serde_yaml::to_string(&value).map_err(|e| CliError::internal(e.to_string()))?;
        replacements.push((block.body_start, block.body_end, raw));
    }
    let next = apply_replacements(source.to_owned(), replacements);
    let entries: Vec<(String, String)> = asterdoc_blocks(&next)?.iter()
        .filter(|block| yaml_kind(&block.value) == "step")
        .map(|block| (yaml_id(&block.value), step_path(&block.value)))
        .collect();
    if let Some(error) = ast::find_step_path_conflict(&entries) {
        return Err(step_path_cli_error(error));
    }
    Ok(next)
}

pub(crate) fn contains_step_reference(value: &serde_yaml::Value, step_id: &str, target_path: &str) -> bool {
    match value {
        serde_yaml::Value::Sequence(values) => values.iter()
            .any(|value| contains_step_reference(value, step_id, target_path)),
        serde_yaml::Value::Mapping(values) => values.iter().any(|(key, value)| {
            let key = key.as_str().unwrap_or("");
            let reference_key = matches!(
                key,
                "ref" | "step_ref" | "step_refs" | "step_id" | "stepId" | "step_ids" | "stepIds"
                    | "parent_step_id" | "parentStepId" | "sub_steps" | "subSteps"
            ) || key.ends_with("_step_id") || key.ends_with("_step_ids")
                || key.ends_with("_ref") || key.ends_with("_refs");
            (reference_key && (value.as_str() == Some(step_id)
                || value.as_sequence().is_some_and(|items| items.iter().any(|item| item.as_str() == Some(step_id)))))
                || (key == "step_path" && value.as_str() == Some(target_path))
                || contains_step_reference(value, step_id, target_path)
        }),
        _ => false,
    }
}

pub(crate) fn step_reference_error(step_id: &str) -> CliError {
    CliError {
        code: "STEP_REFERENCED",
        message: format!("step {step_id} is referenced and cannot be removed"),
        exit: 2,
        path: Some(format!("step:{step_id}")),
        current_version: None,
        current_version_id: None,
        issues: vec![json!({"code":"STEP_REFERENCED","step_id":step_id})],
    }
}

pub(crate) fn apply_asterdoc_patch(source: &str, command: &Value) -> Result<String, CliError> {
    let kind = command.get("type").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("command.type is required"))?;
    let module_id = command.get("moduleId").and_then(Value::as_str).unwrap_or("");
    if kind == "add_step" {
        let module = command.get("module").cloned().unwrap_or(Value::Null);
        if module.is_null() {
            return Err(CliError::input("add_step needs module"));
        }
        let mut module_yaml: serde_yaml::Value = serde_json::from_value(module)
            .map_err(|e| CliError::input(format!("module is invalid: {e}")))?;
        let step_id = yaml_id(&module_yaml);
        if step_id.is_empty() {
            return Err(CliError::input("step needs an id"));
        }
        if yaml_kind(&module_yaml) == "step" && module_yaml.get("schema").is_none() {
            if let Some(map) = module_yaml.as_mapping_mut() {
                map.insert(serde_yaml::Value::String("schema".to_owned()), serde_yaml::Value::Number(1.into()));
            }
        }
        // A step without a path is unreachable in the workflow tree, so the
        // writer derives root/<step-id> instead of storing an empty path.
        if yaml_kind(&module_yaml) == "step" && step_path(&module_yaml).is_empty() {
            let derived = ast::derive_step_path(&step_id).map_err(step_path_cli_error)?;
            module_yaml.as_mapping_mut()
                .ok_or_else(|| CliError::input(format!("step {step_id} module must be a mapping")))?
                .insert(
                    serde_yaml::Value::String("path".to_owned()),
                    serde_yaml::Value::String(derived),
                );
        }
        let mut next = append_asterdoc_module(source, &module_yaml)?;
        let blocks = asterdoc_blocks(&next)?;
        let workflow_id = command.get("workflowId").and_then(Value::as_str).unwrap_or("");
        let mut replacements: Vec<(usize, usize, String)> = Vec::new();
        for block in &blocks {
            if yaml_kind(&block.value) != "workflow" {
                continue;
            }
            if !workflow_id.is_empty() && yaml_id(&block.value) != workflow_id {
                continue;
            }
            let mut value = block.value.clone();
            let mapping = value.as_mapping_mut()
                .ok_or_else(|| CliError::input("workflow module must be a mapping"))?;
            let steps_key = serde_yaml::Value::String("steps".to_owned());
            let steps = mapping.entry(steps_key).or_insert_with(|| serde_yaml::Value::Sequence(Vec::new()));
            if let Some(sequence) = steps.as_sequence_mut() {
                if !sequence.iter().any(|item| item.as_str() == Some(step_id.as_str())) {
                    sequence.push(serde_yaml::Value::String(step_id.clone()));
                }
            }
            let order_key = serde_yaml::Value::String("business_order".to_owned());
            let order = mapping.entry(order_key).or_insert_with(|| serde_yaml::Value::Sequence(Vec::new()));
            if let Some(sequence) = order.as_sequence_mut() {
                if !sequence.iter().any(|item| item.as_str() == Some(step_id.as_str())) {
                    sequence.push(serde_yaml::Value::String(step_id.clone()));
                }
            }
            let raw = serde_yaml::to_string(&value).map_err(|e| CliError::internal(e.to_string()))?;
            replacements.push((block.body_start, block.body_end, raw));
            break;
        }
        if !replacements.is_empty() {
            next = apply_replacements(next, replacements);
        }
        return Ok(next);
    }
    if kind == "reorder_steps" {
        let blocks = asterdoc_blocks(source)?;
        let workflow = blocks.iter().find(|block|
            yaml_kind(&block.value) == "workflow" && yaml_id(&block.value) == module_id
        ).ok_or_else(|| CliError::missing(format!("workflow {module_id} does not exist")))?;
        let declared: Vec<String> = workflow.value.get("steps").and_then(|value| value.as_sequence())
            .into_iter().flatten().filter_map(|value| value.as_str().map(str::to_owned)).collect();
        let requested: Vec<String> = command.get("order").and_then(Value::as_array)
            .into_iter().flatten().filter_map(Value::as_str).map(str::to_owned).collect();
        let declared_set: HashSet<_> = declared.iter().collect();
        let requested_set: HashSet<_> = requested.iter().collect();
        if requested.len() != declared.len() || requested_set.len() != requested.len()
            || requested_set != declared_set {
            return Err(CliError::input("order must be a complete permutation of workflow.steps"));
        }
        let mut value = workflow.value.clone();
        value.as_mapping_mut().ok_or_else(|| CliError::input("workflow module must be a mapping"))?
            .insert(serde_yaml::Value::String("business_order".to_owned()),
                serde_yaml::to_value(requested).map_err(|e| CliError::internal(e.to_string()))?);
        let raw = serde_yaml::to_string(&value).map_err(|e| CliError::internal(e.to_string()))?;
        return Ok(apply_replacements(source.to_owned(), vec![(workflow.body_start, workflow.body_end, raw)]));
    }
    if kind == "nest_step" || kind == "promote_step" {
        let blocks = asterdoc_blocks(source)?;
        let child = blocks.iter().find(|block| yaml_kind(&block.value) == "step" && yaml_id(&block.value) == module_id)
            .ok_or_else(|| CliError::missing(format!("step {module_id} does not exist")))?;
        let child_id = module_id.to_owned();
        let child_path = step_path(&child.value);
        let mut extra_paths: Vec<(String, String)> = Vec::new();
        let new_path = if kind == "nest_step" {
            let parent_id = command.get("parentId").and_then(Value::as_str).unwrap_or("");
            if parent_id.is_empty() {
                return Err(CliError::input(
                    "nest_step needs a parentId naming the step that becomes the parent; pass the target step id"
                ));
            }
            if parent_id == module_id {
                return Err(CliError::input(format!(
                    "nest_step cannot nest step {module_id} under itself; pass a different parentId"
                )));
            }
            let parent = blocks.iter().find(|block| yaml_kind(&block.value) == "step" && yaml_id(&block.value) == parent_id)
                .ok_or_else(|| CliError::missing(format!("parent step {parent_id} does not exist")))?;
            let parent_path = step_path(&parent.value);
            let resolved_parent = ast::resolve_step_path(parent_id, &parent_path)
                .map_err(step_path_cli_error)?;
            // A missing parent path is materialized in the same revision;
            // otherwise the child would point at a path no step actually has.
            if parent_path.is_empty() {
                extra_paths.push((parent_id.to_owned(), resolved_parent.clone()));
            }
            ast::nest_step_path(&child_id, &child_path, parent_id, &resolved_parent)
                .map_err(step_path_cli_error)?
        } else if child_path.is_empty() {
            // Nothing to promote out of: materialize the derived top-level path.
            ast::resolve_step_path(&child_id, "")
                .map_err(step_path_cli_error)?
        } else {
            ast::promote_step_path(&child_id, &child_path).map_err(step_path_cli_error)?
        };
        return rewrite_step_subtree(source, &child_id, &child_path, &new_path, &extra_paths);
    }
    if kind == "remove_step" {
        let blocks = asterdoc_blocks(source)?;
        let index = blocks.iter().position(|block| yaml_id(&block.value) == module_id)
            .ok_or_else(|| CliError::missing(format!("step {module_id} does not exist")))?;
        let target = &blocks[index].value;
        let target_path = step_path(target);
        if target.get("protect_references").and_then(|value| value.as_bool()) == Some(true)
            || blocks.iter().enumerate().any(|(other_index, candidate)| {
                if other_index == index { return false; }
                let path = step_path(&candidate.value);
                (yaml_kind(&candidate.value) == "step"
                    && path.starts_with(&format!("{target_path}/")))
                    || contains_step_reference(&candidate.value, module_id, &target_path)
            })
        {
            return Err(step_reference_error(module_id));
        }
        let block = &blocks[index];
        let mut replacements: Vec<(usize, usize, String)> = vec![(
            block.fence_start,
            block.fence_end,
            String::new(),
        )];
        for workflow in &blocks {
            if yaml_kind(&workflow.value) != "workflow" {
                continue;
            }
            let mut value = workflow.value.clone();
            let mapping = value.as_mapping_mut()
                .ok_or_else(|| CliError::input("workflow module must be a mapping"))?;
            for key in ["steps", "business_order"] {
                if let Some(sequence) = mapping
                    .get_mut(serde_yaml::Value::String(key.to_owned()))
                    .and_then(|item| item.as_sequence_mut())
                {
                    sequence.retain(|item| item.as_str() != Some(module_id));
                }
            }
            if let Some(edges) = mapping
                .get_mut(serde_yaml::Value::String("edges".to_owned()))
                .and_then(|item| item.as_sequence_mut())
            {
                edges.retain(|edge| {
                    !(edge.get("from").and_then(|v| v.as_str()) == Some(module_id)
                        || edge.get("to").and_then(|v| v.as_str()) == Some(module_id))
                });
            }
            let raw = serde_yaml::to_string(&value).map_err(|e| CliError::internal(e.to_string()))?;
            replacements.push((workflow.body_start, workflow.body_end, raw));
        }
        return Ok(apply_replacements(source.to_owned(), replacements));
    }
    if kind == "add_module" {
        let module = command.get("module").cloned().unwrap_or(Value::Null);
        if module.is_null() {
            return Err(CliError::input("add_module needs module"));
        }
        let module_yaml: serde_yaml::Value = serde_json::from_value(module)
            .map_err(|e| CliError::input(format!("module is invalid: {e}")))?;
        let id = yaml_id(&module_yaml);
        if id.is_empty() {
            return Err(CliError::input("module needs an id"));
        }
        if asterdoc_blocks(source)?.iter().any(|block| yaml_id(&block.value) == id) {
            return Err(CliError::input(format!("module id {id} already exists")));
        }
        let raw = serde_yaml::to_string(&module_yaml)
            .map_err(|e| CliError::internal(e.to_string()))?;
        let mut next = source.to_owned();
        if !next.is_empty() && !next.ends_with('\n') {
            next.push('\n');
        }
        next.push_str("\n```asterlab\n");
        next.push_str(&raw);
        next.push_str("```\n");
        return Ok(next);
    }

    if matches!(kind, "insert_step_block" | "update_step_block" | "move_step_block" | "remove_step_block") {
        let step_id = command.get("stepId").and_then(Value::as_str).filter(|s| !s.is_empty()).unwrap_or(module_id);
        let blocks = asterdoc_blocks(source)?;
        let step_block = blocks.iter().find(|block| yaml_kind(&block.value) == "step" && yaml_id(&block.value) == step_id)
            .ok_or_else(|| CliError::missing(format!("step {step_id} does not exist")))?;
        let mut value = step_block.value.clone();
        let mapping = value.as_mapping_mut()
            .ok_or_else(|| CliError::input(format!("step {step_id} module must be a mapping")))?;
        let blocks_key = serde_yaml::Value::String("blocks".to_owned());
        let seq = mapping.entry(blocks_key).or_insert_with(|| serde_yaml::Value::Sequence(Vec::new()))
            .as_sequence_mut()
            .ok_or_else(|| CliError::input(format!("step {step_id}.blocks must be a list")))?;
        match kind {
            "insert_step_block" => {
                let raw_block = command.get("block").cloned().unwrap_or(Value::Null);
                if raw_block.is_null() {
                    return Err(CliError::input("insert_step_block needs block"));
                }
                let mut block_yaml: serde_yaml::Value = serde_json::from_value(raw_block)
                    .map_err(|e| CliError::input(format!("step block is invalid: {e}")))?;
                let bid = yaml_id(&block_yaml);
                if bid.is_empty() {
                    return Err(CliError::input("step block needs an id"));
                }
                for top in &blocks {
                    if yaml_id(&top.value) == bid {
                        return Err(CliError::input(format!("module id {bid} already exists")));
                    }
                    for nested in top.value.get("blocks").and_then(|v| v.as_sequence()).into_iter().flatten() {
                        if yaml_id(nested) == bid {
                            return Err(CliError::input(format!("module id {bid} already exists")));
                        }
                    }
                }
                if block_yaml.get("schema").is_none() {
                    if let Some(bm) = block_yaml.as_mapping_mut() {
                        bm.insert(serde_yaml::Value::String("schema".to_owned()), serde_yaml::Value::Number(1.into()));
                    }
                }
                let idx = command.get("index").and_then(Value::as_u64).map(|i| i as usize);
                if let Some(i) = idx.filter(|i| *i < seq.len()) {
                    seq.insert(i, block_yaml);
                } else {
                    seq.push(block_yaml);
                }
            }
            "update_step_block" => {
                let block_id = command.get("blockId").and_then(Value::as_str).unwrap_or("");
                let properties = command.get("properties").and_then(Value::as_object)
                    .ok_or_else(|| CliError::input("update_step_block needs properties"))?;
                let target = seq.iter_mut().find(|item| yaml_id(item) == block_id)
                    .ok_or_else(|| CliError::missing(format!("step {step_id} block {block_id} does not exist")))?;
                let target_map = target.as_mapping_mut()
                    .ok_or_else(|| CliError::input("step block must be a mapping"))?;
                for (key, prop) in properties {
                    if key == "id" || key == "kind" || key == "schema" {
                        continue;
                    }
                    let yv: serde_yaml::Value = serde_json::from_value(prop.clone())
                        .map_err(|e| CliError::input(format!("property {key}: {e}")))?;
                    target_map.insert(serde_yaml::Value::String(key.clone()), yv);
                }
            }
            "move_step_block" => {
                let block_id = command.get("blockId").and_then(Value::as_str).unwrap_or("");
                let pos = seq.iter().position(|item| yaml_id(item) == block_id)
                    .ok_or_else(|| CliError::missing(format!("step {step_id} block {block_id} does not exist")))?;
                let target_pos = if let Some(to) = command.get("toIndex").and_then(Value::as_u64) {
                    (to as usize).min(seq.len().saturating_sub(1))
                } else {
                    match command.get("direction").and_then(Value::as_str).unwrap_or("") {
                        "up" => pos.saturating_sub(1),
                        "down" => (pos + 1).min(seq.len().saturating_sub(1)),
                        other => return Err(CliError::input(format!("move_step_block direction must be up or down, got {other}"))),
                    }
                };
                if pos != target_pos {
                    let item = seq.remove(pos);
                    seq.insert(target_pos, item);
                }
            }
            "remove_step_block" => {
                let block_id = command.get("blockId").and_then(Value::as_str).unwrap_or("");
                let before = seq.len();
                seq.retain(|item| yaml_id(item) != block_id);
                if seq.len() == before {
                    return Err(CliError::missing(format!("step {step_id} block {block_id} does not exist")));
                }
            }
            _ => {}
        }
        let raw = serde_yaml::to_string(&value).map_err(|e| CliError::internal(e.to_string()))?;
        return Ok(apply_replacements(source.to_owned(), vec![(step_block.body_start, step_block.body_end, raw)]));
    }

    let blocks = asterdoc_blocks(source)?;
    let index = blocks.iter().position(|block| yaml_id(&block.value) == module_id)
        .ok_or_else(|| CliError::missing(format!("module {module_id} does not exist")))?;
    let block = &blocks[index];

    if kind == "remove_module" {
        let mut next = source.to_owned();
        next.replace_range(block.fence_start..block.fence_end, "");
        return Ok(next);
    }

    let mut value = block.value.clone();
    match kind {
        "assign_wells" | "clear_wells" => {
            let group = command.get("group").and_then(Value::as_str).unwrap_or("").trim();
            let wells: Vec<String> = command.get("wells").and_then(Value::as_array)
                .into_iter().flatten()
                .filter_map(|well| well.as_str().map(|well| well.trim().to_uppercase()))
                .collect();
            if wells.is_empty() {
                return Err(CliError::input("wells cannot be empty"));
            }
            let mapping = value.as_mapping_mut()
                .ok_or_else(|| CliError::input("plate module must be a mapping"))?;
            let groups_key = serde_yaml::Value::String("groups".to_owned());
            let groups = mapping.entry(groups_key).or_insert_with(|| {
                serde_yaml::Value::Mapping(serde_yaml::Mapping::new())
            });
            let groups = groups.as_mapping_mut()
                .ok_or_else(|| CliError::input("plate.groups must be a mapping"))?;
            for (_, existing) in groups.iter_mut() {
                if let Some(sequence) = existing.as_sequence_mut() {
                    sequence.retain(|well| {
                        well.as_str()
                            .map(|well| !wells.iter().any(|remove| remove == &well.trim().to_uppercase()))
                            .unwrap_or(true)
                    });
                }
            }
            if kind == "assign_wells" {
                if group.is_empty() {
                    return Err(CliError::input("assign_wells needs a group"));
                }
                let key = serde_yaml::Value::String(group.to_owned());
                let target = groups.entry(key).or_insert_with(|| {
                    serde_yaml::Value::Sequence(Vec::new())
                });
                let sequence = target.as_sequence_mut()
                    .ok_or_else(|| CliError::input("plate group must be a list"))?;
                for well in &wells {
                    if !sequence.iter().any(|existing| existing.as_str() == Some(well.as_str())) {
                        sequence.push(serde_yaml::Value::String(well.clone()));
                    }
                }
                let mut sorted: Vec<String> = sequence.iter()
                    .filter_map(|well| well.as_str().map(str::to_owned))
                    .collect();
                sorted.sort();
                *sequence = sorted.into_iter().map(serde_yaml::Value::String).collect();
            } else {
                for field in ["samples", "replicates", "roles"] {
                    let key = serde_yaml::Value::String(field.to_owned());
                    if let Some(assignments) = mapping.get_mut(&key).and_then(|value| value.as_mapping_mut()) {
                        let remove: Vec<serde_yaml::Value> = assignments
                            .iter()
                            .filter_map(|(key, _)| {
                                let assigned = key.as_str()?.trim().to_uppercase();
                                wells.iter().any(|well| well == &assigned).then(|| key.clone())
                            })
                            .collect();
                        for key in remove {
                            assignments.remove(&key);
                        }
                    }
                }
                let factors_key = serde_yaml::Value::String("factors".to_owned());
                if let Some(factors) = mapping.get_mut(&factors_key).and_then(|value| value.as_mapping_mut()) {
                    let mut empty_dims = Vec::new();
                    for (dim_key, dim_val) in factors.iter_mut() {
                        if let Some(dim_map) = dim_val.as_mapping_mut() {
                            let remove: Vec<serde_yaml::Value> = dim_map
                                .iter()
                                .filter_map(|(wk, _)| {
                                    let assigned = wk.as_str()?.trim().to_uppercase();
                                    wells.iter().any(|well| well == &assigned).then(|| wk.clone())
                                })
                                .collect();
                            for wk in remove {
                                dim_map.remove(&wk);
                            }
                            if dim_map.is_empty() {
                                empty_dims.push(dim_key.clone());
                            }
                        }
                    }
                    for dk in empty_dims {
                        factors.remove(&dk);
                    }
                }
            }
        }
        "connect_steps" | "disconnect_steps" => {
            let from = command.get("from").and_then(Value::as_str).unwrap_or("");
            let to = command.get("to").and_then(Value::as_str).unwrap_or("");
            if from.is_empty() || to.is_empty() {
                return Err(CliError::input("connect/disconnect needs from and to"));
            }
            let mapping = value.as_mapping_mut()
                .ok_or_else(|| CliError::input("workflow module must be a mapping"))?;
            let edges_key = serde_yaml::Value::String("edges".to_owned());
            let edges = mapping.entry(edges_key).or_insert_with(|| {
                serde_yaml::Value::Sequence(Vec::new())
            });
            let sequence = edges.as_sequence_mut()
                .ok_or_else(|| CliError::input("workflow.edges must be a list"))?;
            sequence.retain(|edge| {
                !(edge.get("from").and_then(|v| v.as_str()) == Some(from)
                    && edge.get("to").and_then(|v| v.as_str()) == Some(to))
            });
            if kind == "connect_steps" {
                let mut edge = serde_yaml::Mapping::new();
                edge.insert(
                    serde_yaml::Value::String("id".to_owned()),
                    serde_yaml::Value::String(format!("edge-{from}-{to}")),
                );
                edge.insert(
                    serde_yaml::Value::String("from".to_owned()),
                    serde_yaml::Value::String(from.to_owned()),
                );
                edge.insert(
                    serde_yaml::Value::String("to".to_owned()),
                    serde_yaml::Value::String(to.to_owned()),
                );
                sequence.push(serde_yaml::Value::Mapping(edge));
            }
        }
        "update_module" => {
            let properties = command.get("properties").and_then(Value::as_object)
                .ok_or_else(|| CliError::input("update_module needs properties"))?;
            let mapping = value.as_mapping_mut()
                .ok_or_else(|| CliError::input("module must be a mapping"))?;
            for (key, property) in properties {
                if key == "id" || key == "kind" || key == "schema" {
                    continue;
                }
                let yaml_value: serde_yaml::Value = serde_json::from_value(property.clone())
                    .map_err(|e| CliError::input(format!("property {key}: {e}")))?;
                mapping.insert(serde_yaml::Value::String(key.clone()), yaml_value);
            }
        }
        _ => return Err(CliError::input(format!("unsupported command type {kind}"))),
    }
    let raw = serde_yaml::to_string(&value)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut next = source.to_owned();
    next.replace_range(block.body_start..block.body_end, &raw);
    Ok(next)
}

pub(crate) fn load_asterdoc_at(db: &Connection, design_id: &str, revision: u64) -> Result<Value, CliError> {
    let (kind, _, _, _, _) =
        kind_id(db, design_id)?.ok_or_else(|| CliError::missing("实验设计不存在"))?;
    if kind != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let row: Option<String> = db
        .query_row(
            "SELECT data_json FROM records WHERE kind='asterdoc_version' AND parent_id=?1 AND revision=?2",
            params![design_id, revision as i64],
            |row| row.get(0),
        )
        .optional()
        .map_err(|e| CliError::internal(e.to_string()))?;
    let raw = row.ok_or_else(|| CliError::missing(format!(
        "AsterDoc revision v{revision} does not exist"
    )))?;
    let mut data: Value = serde_json::from_str(&raw)
        .map_err(|e| CliError::internal(e.to_string()))?;
    data["design_id"] = json!(design_id);
    Ok(data)
}

/// load_asterdoc returns the latest immutable document revision for a design.
pub(crate) fn load_asterdoc(db: &Connection, design_id: &str) -> Result<Value, CliError> {
    let (kind, _, _, _, _) =
        kind_id(db, design_id)?.ok_or_else(|| CliError::missing("实验设计不存在"))?;
    if kind != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let row: Option<String> = db
        .query_row(
            "SELECT data_json FROM records WHERE kind='asterdoc_version' AND parent_id=?1 ORDER BY revision DESC LIMIT 1",
            [design_id],
            |row| row.get(0),
        )
        .optional()
        .map_err(|e| CliError::internal(e.to_string()))?;
    let Some(raw) = row else {
        return Ok(json!({
            "design_id": design_id,
            "revision": 0,
            "source": "",
            "source_hash": "",
            "schema_version": ASTERDOC_SCHEMA_VERSION,
            "compiler_version": ASTERDOC_COMPILER_VERSION
        }));
    };
    let mut data: Value = serde_json::from_str(&raw)
        .map_err(|e| CliError::internal(e.to_string()))?;
    data["design_id"] = json!(design_id);
    Ok(data)
}

/// validate_asterdoc_source is Core's authority: the GUI may preview more
/// language features, but a saved revision must pass these checks.
pub(crate) fn validate_asterdoc_source(source: &str) -> Result<(), CliError> {
    let mut ids = std::collections::HashSet::new();
    let mut modules: Vec<serde_yaml::Value> = Vec::new();
    let fm_end = frontmatter_end_offset(source);
    let mut in_fence = false;
    let mut in_other_fence = false;
    let mut fence_ticks = 0usize;
    let mut body = String::new();
    let mut line_no = source[..fm_end].lines().count();
    for line in source[fm_end..].lines() {
        line_no += 1;
        let trimmed = line.trim();
        if !in_fence && !in_other_fence {
            let ticks = leading_backticks(trimmed);
            if ticks >= 3 {
                let lang = trimmed[ticks..].trim();
                if ticks == 3 && lang == "asterlab" {
                    in_fence = true;
                    fence_ticks = ticks;
                    body.clear();
                } else {
                    in_other_fence = true;
                    fence_ticks = ticks;
                }
            }
            continue;
        }
        if in_other_fence {
            if is_closing_backtick_fence(trimmed, fence_ticks) {
                in_other_fence = false;
                fence_ticks = 0;
            }
            continue;
        }
        if is_closing_backtick_fence(trimmed, fence_ticks) {
            let module: serde_yaml::Value = serde_yaml::from_str(&body)
                .map_err(|e| CliError::input(format!("asterlab block near line {line_no}: {e}")))?;
            validate_asterdoc_module_value(&module, line_no)?;
            parse_asterdoc_header(&body, &mut ids, line_no)?;
            if module.get("kind").and_then(|k| k.as_str()) == Some("step") {
                for nested in module.get("blocks").and_then(|v| v.as_sequence()).into_iter().flatten() {
                    let bid = yaml_id(nested);
                    let bid = bid.trim();
                    if bid.is_empty() {
                        return Err(CliError::input("inline step block needs a stable id"));
                    }
                    if !ids.insert(bid.to_owned()) {
                        return Err(CliError::input(format!("asterlab module id {bid} is duplicated")));
                    }
                }
            }
            modules.push(module);
            in_fence = false;
            fence_ticks = 0;
            continue;
        }
        body.push_str(line);
        body.push('\n');
    }
    if in_fence {
        return Err(CliError::input("asterlab block is not closed"));
    }
    validate_asterdoc_references(&modules)
}

pub(crate) fn validate_asterdoc_references(modules: &[serde_yaml::Value]) -> Result<(), CliError> {
    let text = |value: &serde_yaml::Value, key: &str| {
        value
            .get(key)
            .and_then(|item| item.as_str())
            .map(str::trim)
            .unwrap_or("")
            .to_owned()
    };
    let strings = |value: &serde_yaml::Value, key: &str| -> Vec<String> {
        value
            .get(key)
            .and_then(|item| item.as_sequence())
            .into_iter()
            .flatten()
            .filter_map(|item| item.as_str().map(str::to_owned))
            .collect()
    };
    let kind = |value: &serde_yaml::Value| text(value, "kind");
    let id = |value: &serde_yaml::Value| text(value, "id");
    let mut flat_modules: Vec<&serde_yaml::Value> = Vec::with_capacity(modules.len());
    for module in modules {
        flat_modules.push(module);
        if kind(module) == "step" {
            for nested in module.get("blocks").and_then(|v| v.as_sequence()).into_iter().flatten() {
                flat_modules.push(nested);
            }
        }
    }
    let mut groups = std::collections::HashSet::new();
    let mut assays = std::collections::HashSet::new();
    let mut steps = std::collections::HashSet::new();
    let mut ids_by_kind: std::collections::HashMap<String, std::collections::HashSet<String>> =
        std::collections::HashMap::new();
    let mut all_ids = std::collections::HashSet::new();
    let mut top_ids = std::collections::HashSet::new();
    for module in modules {
        top_ids.insert(id(module));
    }
    for module in &flat_modules {
        let module_kind = kind(module);
        let module_id = id(module);
        match module_kind.as_str() {
            "group" => { groups.insert(module_id.clone()); }
            "group_table" => {
                for row in module.get("rows").and_then(|v| v.as_sequence()).into_iter().flatten() {
                    let gid = text(row, "id");
                    if !gid.is_empty() {
                        groups.insert(gid.clone());
                        ids_by_kind.entry("group".to_owned()).or_default().insert(gid);
                    }
                }
            }
            "assay" => { assays.insert(module_id.clone()); }
            "step" => { steps.insert(module_id.clone()); }
            _ => {}
        }
        ids_by_kind.entry(module_kind).or_default().insert(module_id.clone());
        all_ids.insert(module_id);
    }
    for module in &flat_modules {
        let module_kind = kind(module);
        let module_id = id(module);
        let schema = module.get("schema").and_then(|value| value.as_i64()).unwrap_or(0);
        let Some(spec) = asterdoc_module_spec(&module_kind, schema) else {
            continue;
        };
        for input in spec["inputs"].as_array().into_iter().flatten() {
            let port_type = input["type"].as_str().unwrap_or("");
            let item_type = input["item_type"].as_str().unwrap_or("");
            if port_type != "reference" && !(port_type == "list" && item_type == "reference") {
                continue;
            }
            let name = input["name"].as_str().unwrap_or("");
            let target = input["target"].as_str().unwrap_or("");
            let values: Vec<String> = if port_type == "reference" {
                let value = text(module, name);
                if value.is_empty() { Vec::new() } else { vec![value] }
            } else {
                strings(module, name)
            };
            for value in values {
                let found = if target == "module" {
                    all_ids.contains(&value)
                } else {
                    ids_by_kind.get(target).is_some_and(|ids| ids.contains(&value))
                };
                if !found {
                    let label = if target == "module" { "module" } else { target };
                    return Err(CliError::input(format!(
                        "{module_kind} {module_id} field {name} references missing {label} {value}"
                    )));
                }
            }
        }
    }
    let by_id: std::collections::HashMap<String, &serde_yaml::Value> =
        flat_modules.iter().map(|module| (id(module), *module)).collect();
    for module in modules {
        if kind(module) != "step" {
            continue;
        }
        for ref_id in strings(module, "block_refs") {
            let ref_id = ref_id.trim();
            if !ref_id.is_empty() && !top_ids.contains(ref_id) && !all_ids.contains(ref_id) {
                return Err(CliError::input(format!(
                    "step {} block_refs references missing module {ref_id}",
                    id(module)
                )));
            }
        }
        for use_item in module.get("uses").and_then(|value| value.as_sequence()).into_iter().flatten() {
            let ref_id = asterdoc_map_get(use_item, "ref").and_then(|value| value.as_str()).unwrap_or("").trim();
            let role = asterdoc_map_get(use_item, "role").and_then(|value| value.as_str()).unwrap_or("").trim();
            if ref_id.is_empty() {
                continue;
            }
            let target = by_id.get(ref_id).ok_or_else(|| CliError::input(format!(
                "step {} uses missing module {ref_id}",
                id(module)
            )))?;
            let target_kind = kind(target);
            let target_schema = target.get("schema").and_then(|value| value.as_i64()).unwrap_or(0);
            let spec = asterdoc_module_spec(&target_kind, target_schema).ok_or_else(|| {
                CliError::input(format!("step {} uses unregistered module {ref_id}", id(module)))
            })?;
            if !role_allowed_for_module(role, spec) {
                return Err(CliError::input(format!(
                    "step {} use {ref_id} has role {role} incompatible with {target_kind}",
                    id(module)
                )));
            }
        }
    }
    for module in &flat_modules {
        match kind(module).as_str() {
            "assay" => {
                for reference in strings(module, "group_refs") {
                    if !groups.contains(&reference) {
                        return Err(CliError::input(format!(
                            "assay {} references missing group {reference}",
                            id(module)
                        )));
                    }
                }
            }
            "step" => {
                let assay = text(module, "assay");
                if !assay.is_empty() && !assays.contains(&assay) {
                    return Err(CliError::input(format!(
                        "step {} references missing assay {assay}",
                        id(module)
                    )));
                }
            }
            "workflow" => {
                let declared: std::collections::HashSet<String> =
                    strings(module, "steps").into_iter().collect();
                for step in &declared {
                    if !steps.contains(step) {
                        return Err(CliError::input(format!(
                            "workflow {} references missing step {step}",
                            id(module)
                        )));
                    }
                }
                for edge in module.get("edges").and_then(|value| value.as_sequence()).into_iter().flatten() {
                    let from = text(edge, "from");
                    let to = text(edge, "to");
                    if !declared.contains(&from) || !declared.contains(&to) {
                        return Err(CliError::input(format!(
                            "workflow {} edge {from}->{to} does not join declared steps",
                            id(module)
                        )));
                    }
                }
                for step in strings(module, "business_order") {
                    if !declared.contains(&step) {
                        return Err(CliError::input(format!(
                            "workflow {} business_order references {step} outside steps",
                            id(module)
                        )));
                    }
                }
            }
            "plate" => {
                let assay = text(module, "assay");
                if !assay.is_empty() && !assays.contains(&assay) {
                    return Err(CliError::input(format!(
                        "plate {} references missing assay {assay}",
                        id(module)
                    )));
                }
                for group in module.get("groups").and_then(|value| value.as_mapping()).into_iter().flatten() {
                    let group = group.0.as_str().unwrap_or("");
                    if !groups.contains(group) {
                        return Err(CliError::input(format!(
                            "plate {} references missing group {group}",
                            id(module)
                        )));
                    }
                }
            }
            _ => {}
        }
    }
    Ok(())
}

pub(crate) fn parse_asterdoc_header(
    body: &str,
    ids: &mut std::collections::HashSet<String>,
    line_no: usize,
) -> Result<(), CliError> {
    let value: serde_yaml::Value = serde_yaml::from_str(body)
        .map_err(|e| CliError::input(format!("asterlab block near line {line_no}: {e}")))?;
    let mapping = value.as_mapping()
        .ok_or_else(|| CliError::input("asterlab block must be a YAML mapping"))?;
    let text = |key: &str| mapping
        .get(serde_yaml::Value::String(key.to_owned()))
        .and_then(|value| value.as_str())
        .map(str::trim)
        .unwrap_or("");
    let id = text("id");
    if id.is_empty() {
        return Err(CliError::input("asterlab module needs a stable id"));
    }
    if !ids.insert(id.to_owned()) {
        return Err(CliError::input(format!("asterlab module id {id} is duplicated")));
    }
    Ok(())
}

pub(crate) const ASTERDOC_MODULE_SPEC: &str = include_str!(
    "../../prototypes/mygo-run/internal/asterdoc/spec/modules.json"
);

pub(crate) fn asterdoc_spec() -> &'static Value {
    use std::sync::OnceLock;
    static SPEC: OnceLock<Value> = OnceLock::new();
    SPEC.get_or_init(|| {
        let spec: Value = serde_json::from_str(ASTERDOC_MODULE_SPEC)
            .expect("embedded AsterDoc module spec must be valid JSON");
        validate_asterdoc_spec(&spec).expect("embedded AsterDoc module spec must satisfy the SDK contract");
        spec
    })
}

/// Every rule name the Rust validator implements. A manifest may only use
/// these names; `validate_asterdoc_spec` rejects anything else at startup.
pub(crate) const ASTERDOC_RULES: &[&str] = &[
    "plate-geometry-wells",
    "reference-ref",
    "group-fields",
    "assay-fields",
    "step-execution",
    "step-blocks",
    "workflow-fields",
    "instrument-profile-fields",
    "material-fields",
    "sample-fields",
    "prose-fields",
    "group_table-fields",
    "param_table-fields",
    "figure-fields",
    "data_table-fields",
    "checklist-fields",
    "calculator-fields",
    "timer-fields",
    "attachment-fields",
    "subflow-fields",
];

pub(crate) fn validate_asterdoc_spec(spec: &Value) -> Result<(), String> {
    if spec["foundationVersion"].as_u64() != Some(1) {
        return Err("foundationVersion must be 1".into());
    }
    let sdk = spec["sdkVersion"].as_str().ok_or("sdkVersion is required")?;
    let categories = ["workflow", "resource", "labware", "measurement", "document", "device"];
    let types = [
        "string", "enum", "bool", "boolean", "int", "integer", "number", "decimal",
        "quantity", "reference", "list", "object", "table", "geometry", "artifact",
        "duration", "timepoint", "relative_time",
        "well-map", "string[]", "object-list",
    ];
    let mut keys = std::collections::HashSet::new();
    for module in spec["modules"].as_array().into_iter().flatten() {
        let kind = module["kind"].as_str().unwrap_or("");
        let schema = module["schema"].as_i64().unwrap_or(0);
        if kind.is_empty() || schema <= 0 {
            return Err("module needs kind and schema".into());
        }
        if !keys.insert(format!("{kind}@{schema}")) {
            return Err(format!("duplicate module {kind}@{schema}"));
        }
        if module["sdk"].as_str().is_some_and(|value| value != sdk) {
            return Err(format!("{kind}@{schema} declares another sdk"));
        }
        if module["category"].as_str().is_some_and(|value| !categories.contains(&value)) {
            return Err(format!("{kind}@{schema} has an unknown category"));
        }
        let permissions: std::collections::HashSet<&str> = module["permissions"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|value| value.as_str())
            .collect();
        for action in module["actions"].as_array().into_iter().flatten() {
            let permission = action["permission"].as_str().unwrap_or("");
            if permission.is_empty() || !permissions.contains(permission) {
                return Err(format!("{kind}@{schema} action names an undeclared permission"));
            }
        }
        // A rule declared in the manifest must have an implementation here.
        // Without this check a manifest-only addition surfaces later as an
        // "unknown rule" internal error on a user's document instead of failing
        // at startup (module-catalog-v1.md §4.1).
        for rule in module["rules"].as_array().into_iter().flatten() {
            let rule = rule.as_str().unwrap_or("");
            if !ASTERDOC_RULES.contains(&rule) {
                return Err(format!(
                    "{kind}@{schema} declares unregistered rule {rule}; \
                     register it in ASTERDOC_RULES and validate_asterdoc_module"
                ));
            }
        }
        for port in module["inputs"].as_array().into_iter().flatten()
            .chain(module["outputs"].as_array().into_iter().flatten())
        {
            let port_type = port["type"].as_str().unwrap_or("");
            if !types.contains(&port_type) {
                return Err(format!("{kind}@{schema} port has an unknown type {port_type}"));
            }
            if port["origin"].as_str() == Some("derived") && port["calculator"].as_str().unwrap_or("").is_empty() {
                return Err(format!("{kind}@{schema} derived output lacks calculator"));
            }
            let reference = port_type == "reference" || port["item_type"].as_str() == Some("reference");
            if reference && port["target"].as_str().unwrap_or("").is_empty() {
                return Err(format!("{kind}@{schema} reference lacks target"));
            }
            if let Some(dimension) = port["dimension"].as_str().filter(|value| !value.is_empty()) {
                let known = asterdoc_units()["units"].as_array().into_iter().flatten()
                    .any(|unit| unit["dimension"].as_str() == Some(dimension));
                if !known {
                    return Err(format!("{kind}@{schema} port has an unknown dimension {dimension}"));
                }
            }
        }
    }
    Ok(())
}

pub(crate) const ASTERDOC_UNITS: &str = include_str!(
    "../../prototypes/mygo-run/internal/asterdoc/spec/units.json"
);

pub(crate) fn asterdoc_units() -> &'static Value {
    use std::sync::OnceLock;
    static UNITS: OnceLock<Value> = OnceLock::new();
    UNITS.get_or_init(|| {
        serde_json::from_str(ASTERDOC_UNITS)
            .expect("embedded AsterDoc unit registry must be valid JSON")
    })
}

pub(crate) fn normalize_asterdoc_unit(symbol: &str) -> String {
    symbol
        .trim()
        .replace('µ', "u")
        .replace('μ', "u")
        .replace("°C", "Cel")
}

pub(crate) fn asterdoc_unit(symbol: &str) -> Option<(String, f64)> {
    let normalized = normalize_asterdoc_unit(symbol);
    asterdoc_units()["units"].as_array()?.iter().find_map(|unit| {
        if unit["symbol"].as_str() != Some(normalized.as_str()) {
            return None;
        }
        Some((
            unit["dimension"].as_str()?.to_owned(),
            unit["factor"].as_f64()?,
        ))
    })
}

pub(crate) fn asterdoc_map_get<'a>(value: &'a serde_yaml::Value, key: &str) -> Option<&'a serde_yaml::Value> {
    value
        .as_mapping()?
        .get(serde_yaml::Value::String(key.to_owned()))
}

pub(crate) fn asterdoc_quantity(value: &serde_yaml::Value, dimension: &str) -> Result<(), CliError> {
    let number = asterdoc_map_get(value, "value")
        .and_then(|value| value.as_f64())
        .ok_or_else(|| CliError::input("quantity value must be a number"))?;
    if !number.is_finite() {
        return Err(CliError::input("quantity value must be finite"));
    }
    let unit = asterdoc_map_get(value, "unit")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|unit| !unit.is_empty())
        .ok_or_else(|| CliError::input("quantity needs a UCUM unit"))?;
    let (actual, _) = asterdoc_unit(unit)
        .ok_or_else(|| CliError::input(format!("unit {unit} is not in the Foundation v1 UCUM subset")))?;
    if !dimension.is_empty() && actual != dimension {
        return Err(CliError::input(format!(
            "unit {unit} is {actual}, not {dimension}"
        )));
    }
    Ok(())
}

pub(crate) fn asterdoc_quantity_parts(value: &serde_yaml::Value) -> Result<(f64, String, String, f64), CliError> {
    let number = asterdoc_map_get(value, "value")
        .and_then(|value| value.as_f64())
        .ok_or_else(|| CliError::input("quantity value must be a number"))?;
    if !number.is_finite() {
        return Err(CliError::input("quantity value must be finite"));
    }
    let unit = asterdoc_map_get(value, "unit")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|unit| !unit.is_empty())
        .ok_or_else(|| CliError::input("quantity needs a UCUM unit"))?
        .to_owned();
    let (dimension, factor) = asterdoc_unit(&unit)
        .ok_or_else(|| CliError::input(format!("unit {unit} is not in the Foundation v1 UCUM subset")))?;
    Ok((number, unit, dimension, factor))
}

pub(crate) fn asterdoc_artifact(value: &serde_yaml::Value) -> Result<(), CliError> {
    let uri = asterdoc_map_get(value, "uri")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|uri| !uri.is_empty())
        .ok_or_else(|| CliError::input("artifact needs a uri"))?;
    if uri.to_ascii_lowercase().starts_with("data:") {
        return Err(CliError::input("artifact must reference external bytes, not inline data"));
    }
    if let Some(hash) = asterdoc_map_get(value, "sha256").and_then(|value| value.as_str()) {
        if !hash.is_empty() && (hash.len() != 64 || !hash.chars().all(|c| c.is_ascii_hexdigit())) {
            return Err(CliError::input("artifact sha256 must be 64 hex characters"));
        }
    }
    Ok(())
}

pub(crate) fn validate_asterdoc_port(kind: &str, input: &Value, value: &serde_yaml::Value) -> Result<(), CliError> {
    let name = input["name"].as_str().unwrap_or("");
    let port_type = input["type"].as_str().unwrap_or("");
    let field = |message: String| CliError::input(format!("{kind}.{name} {message}"));
    match port_type {
        "string" if value.as_str().is_none() => Err(field("must be a string".into())),
        "enum" => {
            let text = value
                .as_str()
                .ok_or_else(|| field("must be an enum string".into()))?;
            if let Some(values) = input["values"].as_array() {
                if !values.iter().any(|candidate| candidate.as_str() == Some(text)) {
                    let allowed = values.iter().filter_map(|v| v.as_str()).collect::<Vec<_>>().join(", ");
                    return Err(field(format!("must be one of {allowed}")));
                }
            }
            Ok(())
        }
        "int" | "integer" if value.as_i64().is_none() => Err(field("must be an integer".into())),
        "number" | "decimal" if !value.is_number() => Err(field("must be a number".into())),
        "bool" | "boolean" if value.as_bool().is_none() => Err(field("must be a boolean".into())),
        "well-map" | "object" | "geometry" if value.as_mapping().is_none() => {
            Err(field("must be a mapping".into()))
        }
        "string[]" => {
            let items = value
                .as_sequence()
                .ok_or_else(|| field("must be a string list".into()))?;
            if items.iter().any(|item| item.as_str().is_none()) {
                return Err(field("must be a string list".into()));
            }
            Ok(())
        }
        "object-list" | "table" => {
            let items = value
                .as_sequence()
                .ok_or_else(|| field("must be an object list".into()))?;
            if items.iter().any(|item| item.as_mapping().is_none()) {
                return Err(field("must be an object list".into()));
            }
            Ok(())
        }
        "list" => {
            let items = value
                .as_sequence()
                .ok_or_else(|| field("must be a list".into()))?;
            for item in items {
                validate_asterdoc_list_item(kind, input, item)?;
            }
            Ok(())
        }
        "quantity" => asterdoc_quantity(value, input["dimension"].as_str().unwrap_or(""))
            .map_err(|error| field(error.message)),
        "duration" => {
            asterdoc_quantity(value, "time").map_err(|error| field(error.message))
        }
        "reference" => value
            .as_str()
            .map(str::trim)
            .filter(|text| !text.is_empty())
            .map(|_| ())
            .ok_or_else(|| field("must be a non-empty reference".into())),
        "artifact" => asterdoc_artifact(value).map_err(|error| field(error.message)),
        "timepoint" => {
            if value.as_str().is_some() {
                Ok(())
            } else if asterdoc_map_get(value, "label")
                .and_then(|label| label.as_str())
                .is_some_and(|label| !label.trim().is_empty())
            {
                Ok(())
            } else {
                Err(field("must be a timepoint string or label mapping".into()))
            }
        }
        "relative_time" => {
            if asterdoc_map_get(value, "anchor")
                .and_then(|anchor| anchor.as_str())
                .is_none_or(|anchor| anchor.trim().is_empty())
            {
                return Err(field("relative time needs an anchor".into()));
            }
            let offset = asterdoc_map_get(value, "offset")
                .ok_or_else(|| field("relative time needs an offset".into()))?;
            asterdoc_quantity(offset, "time").map_err(|error| field(error.message))
        }
        _ => Ok(()),
    }
}

pub(crate) fn validate_asterdoc_list_item(kind: &str, input: &Value, item: &serde_yaml::Value) -> Result<(), CliError> {
    let name = input["name"].as_str().unwrap_or("");
    let item_type = input["item_type"].as_str().unwrap_or("");
    let field = |message: String| CliError::input(format!("{kind}.{name} {message}"));
    match item_type {
        "" | "object" if item.as_mapping().is_none() => Err(field("must be a list of objects".into())),
        "string" if item.as_str().is_none() => Err(field("must be a list of strings".into())),
        "reference" if item.as_str().map(str::trim).is_none_or(|text| text.is_empty()) => {
            Err(field("must be a list of non-empty references".into()))
        }
        "quantity" => asterdoc_quantity(item, input["dimension"].as_str().unwrap_or(""))
            .map_err(|error| field(error.message)),
        "artifact" => asterdoc_artifact(item).map_err(|error| field(error.message)),
        _ => Ok(()),
    }
}

pub(crate) fn asterdoc_module_spec(kind: &str, schema: i64) -> Option<&'static Value> {
    asterdoc_spec()["modules"].as_array()?.iter().find(|module| {
        module["kind"].as_str() == Some(kind) && module["schema"].as_i64() == Some(schema)
    })
}

pub(crate) fn asterdoc_supported() -> String {
    asterdoc_spec()["modules"]
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(|module| {
            Some(format!(
                "{}@{}",
                module["kind"].as_str()?,
                module["schema"].as_i64()?
            ))
        })
        .collect::<Vec<_>>()
        .join(", ")
}

pub(crate) fn plate_geometry(format: i64) -> Option<(i64, i64)> {
    match format {
        4 => Some((2, 2)),
        6 => Some((2, 3)),
        12 => Some((3, 4)),
        24 => Some((4, 6)),
        48 => Some((6, 8)),
        96 => Some((8, 12)),
        384 => Some((16, 24)),
        _ => None,
    }
}

pub(crate) fn role_allowed_for_module(role: &str, spec: &Value) -> bool {
    let required: Vec<&str> = if let Some(values) = asterdoc_spec()["roleCapabilities"][role].as_array() {
        values.iter().filter_map(|value| value.as_str()).collect()
    } else if role.contains(':') {
        vec![role]
    } else {
        return false;
    };
    if required.is_empty() {
        return true;
    }
    let declared: Vec<&str> = spec["capabilities"].as_array().into_iter().flatten()
        .filter_map(|value| value.as_str()).collect();
    required.iter().any(|required| declared.contains(required))
}

pub(crate) fn validate_formula_expr(expr: &str) -> Result<(), String> {
    #[derive(Clone, Debug, PartialEq)]
    enum Tok {
        Num,
        Ident,
        Plus,
        Minus,
        Star,
        Slash,
        LParen,
        RParen,
    }
    let trimmed = expr.trim();
    if trimmed.is_empty() {
        return Err("formula cannot be empty".into());
    }
    let bytes = trimmed.as_bytes();
    let mut tokens = Vec::new();
    let mut i = 0usize;
    while i < bytes.len() {
        let b = bytes[i];
        if b.is_ascii_whitespace() {
            i += 1;
            continue;
        }
        match b {
            b'+' => { tokens.push(Tok::Plus); i += 1; }
            b'-' => { tokens.push(Tok::Minus); i += 1; }
            b'*' => { tokens.push(Tok::Star); i += 1; }
            b'/' => { tokens.push(Tok::Slash); i += 1; }
            b'(' => { tokens.push(Tok::LParen); i += 1; }
            b')' => { tokens.push(Tok::RParen); i += 1; }
            b'0'..=b'9' | b'.' => {
                let start = i;
                while i < bytes.len() && (bytes[i].is_ascii_digit() || bytes[i] == b'.') {
                    i += 1;
                }
                trimmed[start..i]
                    .parse::<f64>()
                    .map_err(|_| format!("invalid number in formula: {}", &trimmed[start..i]))?;
                tokens.push(Tok::Num);
            }
            b'a'..=b'z' | b'A'..=b'Z' | b'_' => {
                let start = i;
                while i < bytes.len() && (bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_') {
                    i += 1;
                }
                let _ = &trimmed[start..i];
                tokens.push(Tok::Ident);
            }
            _ => return Err(format!("unexpected character '{}' in formula", b as char)),
        }
    }
    fn parse_expr(tokens: &[Tok], pos: &mut usize) -> Result<(), String> {
        parse_term(tokens, pos)?;
        while matches!(tokens.get(*pos), Some(Tok::Plus | Tok::Minus)) {
            *pos += 1;
            parse_term(tokens, pos)?;
        }
        Ok(())
    }
    fn parse_term(tokens: &[Tok], pos: &mut usize) -> Result<(), String> {
        parse_factor(tokens, pos)?;
        while matches!(tokens.get(*pos), Some(Tok::Star | Tok::Slash)) {
            *pos += 1;
            parse_factor(tokens, pos)?;
        }
        Ok(())
    }
    fn parse_factor(tokens: &[Tok], pos: &mut usize) -> Result<(), String> {
        match tokens.get(*pos) {
            Some(Tok::Plus | Tok::Minus) => {
                *pos += 1;
                parse_factor(tokens, pos)
            }
            Some(Tok::Num | Tok::Ident) => {
                *pos += 1;
                Ok(())
            }
            Some(Tok::LParen) => {
                *pos += 1;
                parse_expr(tokens, pos)?;
                if !matches!(tokens.get(*pos), Some(Tok::RParen)) {
                    return Err("unclosed '(' in formula".into());
                }
                *pos += 1;
                Ok(())
            }
            _ => Err("unexpected end of formula expression".into()),
        }
    }
    let mut pos = 0usize;
    parse_expr(&tokens, &mut pos)?;
    if pos != tokens.len() {
        return Err("trailing tokens in formula".into());
    }
    Ok(())
}

pub(crate) fn valid_plate_well(well: &str, topology: &str, rows: i64, cols: i64) -> bool {
    let w = well.trim().to_uppercase();
    if w.is_empty() {
        return false;
    }
    if w.len() >= 2 && w.as_bytes()[0].is_ascii_uppercase() {
        let r = (w.as_bytes()[0] - b'A') as i64;
        if r >= 0 && r < rows {
            if let Ok(col) = w[1..].parse::<i64>() {
                return (1..=cols).contains(&col);
            }
        }
    }
    if topology != "plate" {
        if let Ok(idx) = w.parse::<i64>() {
            return idx >= 1 && idx <= rows.saturating_mul(cols);
        }
    }
    false
}

pub(crate) fn validate_asterdoc_module(body: &str, line_no: usize) -> Result<(), CliError> {
    let value: serde_yaml::Value = serde_yaml::from_str(body)
        .map_err(|e| CliError::input(format!("asterlab block near line {line_no}: {e}")))?;
    validate_asterdoc_module_value(&value, line_no)
}

pub(crate) fn validate_asterdoc_module_value(value: &serde_yaml::Value, line_no: usize) -> Result<(), CliError> {
    let mapping = value
        .as_mapping()
        .ok_or_else(|| CliError::input(format!("asterlab block near line {line_no} must be a YAML mapping")))?;
    let get = |key: &str| mapping.get(serde_yaml::Value::String(key.to_owned()));
    let text = |key: &str| {
        get(key)
            .and_then(|value| value.as_str())
            .map(str::trim)
            .unwrap_or("")
    };
    let kind = text("kind");
    let schema = get("schema").and_then(|value| value.as_i64()).unwrap_or(0);
    let spec = asterdoc_module_spec(kind, schema).ok_or_else(|| {
        CliError::input(format!(
            "unsupported asterlab module {kind}@{schema}; supported: {}",
            asterdoc_supported()
        ))
    })?;
    let mut allowed_keys: Vec<&str> = Vec::new();
    for input in spec["inputs"].as_array().into_iter().flatten() {
        if let Some(name) = input["name"].as_str() {
            allowed_keys.push(name);
        }
    }
    for (raw_key, _) in mapping {
        let Some(k) = raw_key.as_str().map(str::trim) else {
            return Err(CliError::input(format!("{kind} module has non-string field key")));
        };
        if !allowed_keys.contains(&k) {
            return Err(CliError::input(format!(
                "{kind} module {} has unknown field {k}; allowed fields: {}",
                text("id"),
                allowed_keys.join(", ")
            )));
        }
    }
    for input in spec["inputs"].as_array().into_iter().flatten() {
        let name = input["name"].as_str().unwrap_or("");
        let present = get(name).is_some_and(|value| !value.is_null());
        if input["required"].as_bool().unwrap_or(false) && !present {
            return Err(CliError::input(format!(
                "{} module needs {name}",
                kind
            )));
        }
        if !present {
            continue;
        }
        validate_asterdoc_port(kind, input, get(name).expect("present field"))?;
    }
    for rule in spec["rules"].as_array().into_iter().flatten() {
        match rule.as_str().unwrap_or("") {
            "plate-geometry-wells" => {
                let topology = if text("topology").is_empty() { "plate" } else { text("topology") };
                let format = get("format").and_then(|value| value.as_i64()).unwrap_or(0);
                let (rows, cols) = if topology != "plate" {
                    let custom_r = get("rows").and_then(|v| v.as_i64()).unwrap_or(0);
                    let custom_c = get("columns").and_then(|v| v.as_i64()).unwrap_or(0);
                    if custom_r > 0 && custom_c > 0 {
                        if custom_r > 16 || custom_c > 24 {
                            return Err(CliError::input("plate custom rows/columns must fit within 16x24"));
                        }
                        (custom_r, custom_c)
                    } else if let Some(geom) = plate_geometry(format) {
                        geom
                    } else if (1..=1536).contains(&format) {
                        (1, format)
                    } else {
                        return Err(CliError::input(format!(
                            "plate format {format} is not valid for topology {topology}"
                        )));
                    }
                } else {
                    let Some(geom) = plate_geometry(format) else {
                        return Err(CliError::input(format!(
                            "plate format {format} is not registered"
                        )));
                    };
                    geom
                };
                let groups = get("groups")
                    .and_then(|value| value.as_mapping())
                    .ok_or_else(|| CliError::input("plate needs groups"))?;
                let mut seen = std::collections::HashSet::new();
                for (group, wells) in groups {
                    let group = group.as_str().unwrap_or("").trim();
                    if group.is_empty() {
                        return Err(CliError::input("plate group name cannot be empty"));
                    }
                    let wells = wells.as_sequence().ok_or_else(|| {
                        CliError::input(format!("plate group {group} must be a list"))
                    })?;
                    for well in wells {
                        let well = well.as_str().unwrap_or("").trim().to_uppercase();
                        if !valid_plate_well(&well, topology, rows, cols) {
                            return Err(CliError::input(format!(
                                "well {well} is outside {rows}x{cols}"
                            )));
                        }
                        if !seen.insert(well.clone()) {
                            return Err(CliError::input(format!(
                                "well {well} is assigned twice"
                            )));
                        }
                    }
                }
                let mut sample_by_well: std::collections::HashMap<String, String> =
                    std::collections::HashMap::new();
                if let Some(samples) = get("samples").and_then(|value| value.as_mapping()) {
                    for (well_key, sample_value) in samples {
                        let well = well_key.as_str().unwrap_or("").trim().to_uppercase();
                        if !valid_plate_well(&well, topology, rows, cols) {
                            return Err(CliError::input(format!(
                                "sample well {well} is outside {rows}x{cols}"
                            )));
                        }
                        if !seen.contains(&well) {
                            return Err(CliError::input(format!(
                                "sample well {well} has no group assignment"
                            )));
                        }
                        let sample_id = sample_value.as_str().map(str::trim).unwrap_or("").to_owned();
                        if sample_id.is_empty() {
                            return Err(CliError::input(format!(
                                "sample ID for well {well} cannot be empty"
                            )));
                        }
                        if sample_by_well.contains_key(&well) {
                            return Err(CliError::input(format!(
                                "sample assignment for well {well} is duplicated"
                            )));
                        }
                        sample_by_well.insert(well, sample_id);
                    }
                }
                let mut factor_pairs_by_well: std::collections::HashMap<String, Vec<(String, String)>> =
                    std::collections::HashMap::new();
                if let Some(factors_val) = get("factors") {
                    let factors = factors_val.as_mapping().ok_or_else(|| {
                        CliError::input("plate.factors must be a mapping")
                    })?;
                    for (dim_key, dim_raw) in factors {
                        let dim = dim_key.as_str().unwrap_or("").trim().to_owned();
                        if dim.is_empty() {
                            return Err(CliError::input("plate factor dimension name cannot be empty"));
                        }
                        let dim_map = dim_raw.as_mapping().ok_or_else(|| {
                            CliError::input(format!("plate factor {dim} must be a mapping"))
                        })?;
                        for (well_key, val_raw) in dim_map {
                            let well = well_key.as_str().unwrap_or("").trim().to_uppercase();
                            if !valid_plate_well(&well, topology, rows, cols) {
                                return Err(CliError::input(format!(
                                    "factor {dim} well {well} is outside {rows}x{cols}"
                                )));
                            }
                            if !seen.contains(&well) {
                                return Err(CliError::input(format!(
                                    "factor {dim} well {well} has no group assignment"
                                )));
                            }
                            let val_str = val_raw.as_str().map(str::trim).unwrap_or("").to_owned();
                            if val_str.is_empty() {
                                return Err(CliError::input(format!(
                                    "factor {dim} value for well {well} cannot be empty"
                                )));
                            }
                            factor_pairs_by_well.entry(well).or_default().push((dim.clone(), val_str));
                        }
                    }
                }
                let factor_sig = |well: &str| -> String {
                    let Some(pairs) = factor_pairs_by_well.get(well) else {
                        return String::new();
                    };
                    let mut sorted = pairs.clone();
                    sorted.sort();
                    sorted
                        .into_iter()
                        .map(|(k, v)| format!("{k}={v}"))
                        .collect::<Vec<_>>()
                        .join("\u{1f}")
                };
                let mut seen_replicates: std::collections::HashMap<String, String> =
                    std::collections::HashMap::new();
                if let Some(replicates) = get("replicates").and_then(|value| value.as_mapping()) {
                    for (well_key, replicate_value) in replicates {
                        let well = well_key.as_str().unwrap_or("").trim().to_uppercase();
                        if !valid_plate_well(&well, topology, rows, cols) {
                            return Err(CliError::input(format!(
                                "replicate well {well} is outside {rows}x{cols}"
                            )));
                        }
                        let sample_id = sample_by_well.get(&well).cloned().unwrap_or_default();
                        if sample_id.is_empty() {
                            return Err(CliError::input(format!(
                                "replicate well {well} has no sample ID"
                            )));
                        }
                        let replicate = replicate_value.as_i64().unwrap_or(0);
                        if replicate < 1 {
                            return Err(CliError::input(format!(
                                "replicate for well {well} must be positive"
                            )));
                        }
                        let sig = factor_sig(&well);
                        let key = format!("{sample_id}\u{0}{sig}\u{0}{replicate}");
                        if let Some(previous) = seen_replicates.get(&key) {
                            return Err(CliError::input(format!(
                                "sample {sample_id} replicate {replicate} is assigned to both {previous} and {well}"
                            )));
                        }
                        seen_replicates.insert(key, well);
                    }
                }
                if let Some(roles) = get("roles").and_then(|value| value.as_mapping()) {
                    for (well_key, role_value) in roles {
                        let well = well_key.as_str().unwrap_or("").trim().to_uppercase();
                        if !valid_plate_well(&well, topology, rows, cols) {
                            return Err(CliError::input(format!(
                                "role well {well} is outside {rows}x{cols}"
                            )));
                        }
                        let role = role_value.as_str().unwrap_or("").trim();
                        if !matches!(
                            role,
                            "sample" | "blank" | "vehicle" | "positive_control" | "negative_control" | "standard" | "ntc"
                        ) {
                            return Err(CliError::input(format!(
                                "invalid well role {role} for {well}"
                            )));
                        }
                    }
                }
            }
            "reference-ref" => {
                if text("ref").is_empty() {
                    return Err(CliError::input("reference needs a ref"));
                }
            }
            "group-fields" => {
                if text("name").is_empty() {
                    return Err(CliError::input("group needs a name"));
                }
            }
            "assay-fields" => {
                if get("group_refs").and_then(|value| value.as_sequence()).is_some_and(|items| items.is_empty()) {
                    return Err(CliError::input("assay needs at least one group_ref"));
                }
            }
            "step-execution" => {
                let execution = if get("execution").is_none_or(|value| value.is_null()) {
                    "executable"
                } else {
                    text("execution")
                };
                match execution {
                    "executable" => {}
                    "organizational" => {
                        for field in ["assay", "timepoints", "path", "checks", "tasks", "uses", "blocks", "block_refs"] {
                            let Some(value) = get(field) else { continue };
                            let present = match value {
                                serde_yaml::Value::Null => false,
                                serde_yaml::Value::String(value) => !value.trim().is_empty(),
                                serde_yaml::Value::Sequence(values) => !values.is_empty(),
                                _ => true,
                            };
                            if present {
                                return Err(CliError::input(format!(
                                    "organizational step cannot declare {field}"
                                )));
                            }
                        }
                    }
                    _ => {
                        return Err(CliError::input(
                            "step execution must be executable or organizational",
                        ));
                    }
                }
            }
            "step-blocks" => {
                if let Some(blocks_val) = get("blocks") {
                    let items = blocks_val
                        .as_sequence()
                        .ok_or_else(|| CliError::input("step.blocks must be a list"))?;
                    for item in items {
                        let item_kind = item
                            .as_mapping()
                            .and_then(|m| m.get(serde_yaml::Value::String("kind".to_owned())))
                            .and_then(|v| v.as_str())
                            .map(str::trim)
                            .unwrap_or("");
                        if !matches!(
                            item_kind,
                            "prose" | "group_table" | "param_table" | "figure" | "data_table"
                                | "checklist" | "calculator" | "timer" | "attachment" | "subflow" | "plate"
                        ) {
                            return Err(CliError::input(format!(
                                "step.blocks cannot embed module kind {item_kind}"
                            )));
                        }
                        validate_asterdoc_module_value(item, line_no)?;
                    }
                }
            }
            "workflow-fields" => {
                if get("steps").and_then(|value| value.as_sequence()).is_none_or(|items| items.is_empty()) {
                    return Err(CliError::input("workflow needs at least one step"));
                }
            }
            "instrument-profile-fields" => {
                if text("class").is_empty() {
                    return Err(CliError::input("instrument profile needs a class"));
                }
                let capabilities = get("capabilities")
                    .and_then(|value| value.as_sequence())
                    .ok_or_else(|| CliError::input("instrument profile needs capabilities"))?;
                if capabilities.is_empty() {
                    return Err(CliError::input("instrument profile needs at least one capability"));
                }
                for capability in capabilities {
                    if asterdoc_map_get(capability, "name")
                        .and_then(|value| value.as_str())
                        .is_none_or(|name| name.trim().is_empty())
                    {
                        return Err(CliError::input("instrument capability needs a name"));
                    }
                }
            }
            "material-fields" => {
                if text("name").is_empty() {
                    return Err(CliError::input("material needs a name"));
                }
                if text("role").is_empty() {
                    return Err(CliError::input("material needs a role"));
                }
                if text("source").is_empty() {
                    return Err(CliError::input("material needs a source"));
                }
            }
            "sample-fields" => {
                if text("sample_id").is_empty() {
                    return Err(CliError::input("sample needs a sample_id"));
                }
            }
            "prose-fields" => {
                if text("body").is_empty() {
                    return Err(CliError::input("prose needs non-empty body"));
                }
            }
            "group_table-fields" => {
                let rows = get("rows")
                    .and_then(|v| v.as_sequence())
                    .ok_or_else(|| CliError::input("group_table needs rows"))?;
                if rows.is_empty() {
                    return Err(CliError::input("group_table needs at least one group row"));
                }
                let mut seen_ids = std::collections::HashSet::new();
                for row in rows {
                    let row_id = asterdoc_map_get(row, "id").and_then(|v| v.as_str()).unwrap_or("").trim();
                    let name = asterdoc_map_get(row, "name").and_then(|v| v.as_str()).unwrap_or("").trim();
                    if row_id.is_empty() || name.is_empty() {
                        return Err(CliError::input("group_table row needs non-empty id and name"));
                    }
                    if !seen_ids.insert(row_id.to_owned()) {
                        return Err(CliError::input(format!("group_table repeats id {row_id}")));
                    }
                }
            }
            "param_table-fields" => {
                let rows = get("rows")
                    .and_then(|v| v.as_sequence())
                    .ok_or_else(|| CliError::input("param_table needs rows"))?;
                for row in rows {
                    let key = asterdoc_map_get(row, "key").and_then(|v| v.as_str()).unwrap_or("").trim();
                    let label = asterdoc_map_get(row, "label").and_then(|v| v.as_str()).unwrap_or("").trim();
                    if key.is_empty() && label.is_empty() {
                        return Err(CliError::input("param_table row needs non-empty key or label"));
                    }
                    if asterdoc_map_get(row, "value").is_none_or(serde_yaml::Value::is_null) {
                        return Err(CliError::input("param_table row needs value"));
                    }
                    if let Some(unit) = asterdoc_map_get(row, "unit").and_then(|v| v.as_str()).map(str::trim).filter(|u| !u.is_empty()) {
                        if asterdoc_unit(unit).is_none() {
                            return Err(CliError::input(format!("param_table unit {unit} is not in UCUM subset")));
                        }
                    }
                }
            }
            "figure-fields" => {
                let src = text("src");
                let svg = text("svg");
                if src.is_empty() && svg.is_empty() {
                    return Err(CliError::input("figure needs src or svg"));
                }
                if !svg.is_empty() && !svg.contains("<svg") {
                    return Err(CliError::input("figure svg must contain <svg"));
                }
                if src.to_ascii_lowercase().starts_with("data:") && src.len() > 65536 {
                    return Err(CliError::input("figure data URI exceeds 65536 bytes"));
                }
                if let Some(h) = get("max_height").and_then(|v| v.as_i64()) {
                    if h < 40 {
                        return Err(CliError::input("figure max_height must be >= 40"));
                    }
                }
            }
            "data_table-fields" => {
                let cols = get("columns")
                    .and_then(|v| v.as_sequence())
                    .ok_or_else(|| CliError::input("data_table needs columns"))?;
                if cols.is_empty() {
                    return Err(CliError::input("data_table needs at least one column"));
                }
                let mut col_keys = std::collections::HashSet::new();
                for col in cols {
                    let ckey = asterdoc_map_get(col, "key").and_then(|v| v.as_str()).unwrap_or("").trim();
                    let clabel = asterdoc_map_get(col, "label").and_then(|v| v.as_str()).unwrap_or("").trim();
                    let vkind = asterdoc_map_get(col, "value_kind").and_then(|v| v.as_str()).unwrap_or("").trim();
                    if ckey.is_empty() || clabel.is_empty() {
                        return Err(CliError::input("data_table column needs key and label"));
                    }
                    if !col_keys.insert(ckey.to_owned()) {
                        return Err(CliError::input(format!("data_table repeats column {ckey}")));
                    }
                    if !matches!(vkind, "text" | "number" | "categorical" | "formula") {
                        return Err(CliError::input(format!("data_table column {ckey} has invalid value_kind {vkind}")));
                    }
                    if let Some(unit) = asterdoc_map_get(col, "unit").and_then(|v| v.as_str()).map(str::trim).filter(|u| !u.is_empty()) {
                        if asterdoc_unit(unit).is_none() {
                            return Err(CliError::input(format!("data_table column {ckey} unit {unit} is not in UCUM subset")));
                        }
                    }
                    if let (Some(min), Some(max)) = (
                        asterdoc_map_get(col, "min").and_then(|v| v.as_f64()),
                        asterdoc_map_get(col, "max").and_then(|v| v.as_f64()),
                    ) {
                        if min > max {
                            return Err(CliError::input(format!("data_table column {ckey} min exceeds max")));
                        }
                    }
                    if vkind == "formula" {
                        let formula = asterdoc_map_get(col, "formula").and_then(|v| v.as_str()).unwrap_or("");
                        validate_formula_expr(formula)
                            .map_err(|e| CliError::input(format!("data_table column {ckey} {e}")))?;
                    }
                }
            }
            "checklist-fields" => {
                let items = get("items")
                    .and_then(|v| v.as_sequence())
                    .ok_or_else(|| CliError::input("checklist needs items"))?;
                if items.is_empty() {
                    return Err(CliError::input("checklist needs at least one item"));
                }
                let mut seen_ids = std::collections::HashSet::new();
                for item in items {
                    let cid = asterdoc_map_get(item, "id").and_then(|v| v.as_str()).unwrap_or("").trim();
                    let label = asterdoc_map_get(item, "label").and_then(|v| v.as_str()).unwrap_or("").trim();
                    if cid.is_empty() || label.is_empty() {
                        return Err(CliError::input("checklist item needs non-empty id and label"));
                    }
                    if !seen_ids.insert(cid.to_owned()) {
                        return Err(CliError::input(format!("checklist repeats item id {cid}")));
                    }
                }
            }
            "calculator-fields" => {
                match text("mode") {
                    "dilution" => {
                        let stock = asterdoc_quantity_parts(get("stock").ok_or_else(|| CliError::input("calculator dilution needs stock"))?)?;
                        let target = asterdoc_quantity_parts(get("target").ok_or_else(|| CliError::input("calculator dilution needs target"))?)?;
                        let final_vol = asterdoc_quantity_parts(get("final_volume").ok_or_else(|| CliError::input("calculator dilution needs final_volume"))?)?;
                        if stock.0 <= 0.0 || target.0 <= 0.0 || final_vol.0 <= 0.0 {
                            return Err(CliError::input("calculator dilution quantities must be positive"));
                        }
                        if final_vol.2 != "volume" {
                            return Err(CliError::input("calculator dilution final_volume must have volume unit"));
                        }
                        if stock.2 != target.2 {
                            return Err(CliError::input("calculator dilution stock and target must have compatible units"));
                        }
                        if target.0 * target.3 > stock.0 * stock.3 {
                            return Err(CliError::input("calculator dilution target exceeds stock"));
                        }
                    }
                    "group_mastermix" => {
                        let vpu = asterdoc_quantity_parts(get("volume_per_unit").ok_or_else(|| CliError::input("calculator group_mastermix needs volume_per_unit"))?)?;
                        if vpu.2 != "volume" || vpu.0 <= 0.0 {
                            return Err(CliError::input("calculator group_mastermix volume_per_unit must be positive volume"));
                        }
                        if let Some(ex) = get("excess_ratio").and_then(|v| v.as_f64()) {
                            if ex < 0.0 || !ex.is_finite() {
                                return Err(CliError::input("calculator excess_ratio must be non-negative"));
                            }
                        }
                        let has_comps = get("components").and_then(|v| v.as_sequence()).is_some_and(|s| !s.is_empty());
                        let has_plate = !text("plate_ref").is_empty() || !text("container_ref").is_empty();
                        if !has_comps && !has_plate {
                            return Err(CliError::input("calculator group_mastermix needs components or plate_ref"));
                        }
                        for comp in get("components").and_then(|v| v.as_sequence()).into_iter().flatten() {
                            if asterdoc_map_get(comp, "name").and_then(|v| v.as_str()).is_none_or(|n| n.trim().is_empty()) {
                                return Err(CliError::input("calculator component needs name"));
                            }
                        }
                    }
                    "serial_dilution" => {
                        let df = get("dilution_factor").and_then(|v| v.as_f64()).unwrap_or(0.0);
                        let steps = get("steps_count").and_then(|v| v.as_i64()).unwrap_or(0);
                        if !df.is_finite() || df <= 1.0 {
                            return Err(CliError::input("calculator serial_dilution dilution_factor must be > 1"));
                        }
                        if steps < 1 {
                            return Err(CliError::input("calculator serial_dilution steps_count must be >= 1"));
                        }
                    }
                    "sample_normalize" => {
                        let count = get("sample_count").and_then(|v| v.as_i64()).unwrap_or(0);
                        let has_vol = get("volume_per_unit").is_some_and(|v| asterdoc_quantity_parts(v).is_ok_and(|p| p.2 == "volume" && p.0 > 0.0));
                        if count < 1 && !has_vol {
                            return Err(CliError::input("calculator sample_normalize needs positive sample_count or volume_per_unit"));
                        }
                    }
                    "formula" => {
                        if let Some(rows) = get("rows").and_then(|v| v.as_sequence()).filter(|s| !s.is_empty()) {
                            for row in rows {
                                let rk = asterdoc_map_get(row, "key").and_then(|v| v.as_str()).unwrap_or("").trim();
                                let rl = asterdoc_map_get(row, "label").and_then(|v| v.as_str()).unwrap_or("").trim();
                                if rk.is_empty() && rl.is_empty() {
                                    return Err(CliError::input("calculator row needs key or label"));
                                }
                                if asterdoc_map_get(row, "kind").and_then(|v| v.as_str()) == Some("formula") {
                                    let expr = asterdoc_map_get(row, "formula").and_then(|v| v.as_str()).unwrap_or("");
                                    validate_formula_expr(expr).map_err(CliError::input)?;
                                }
                            }
                        } else {
                            validate_formula_expr(text("formula")).map_err(CliError::input)?;
                        }
                    }
                    bad_mode => {
                        return Err(CliError::input(format!("unsupported calculator mode {bad_mode}")));
                    }
                }
            }
            "timer-fields" => {
                match text("mode") {
                    "countdown" => {
                        let d = asterdoc_quantity_parts(get("duration").ok_or_else(|| CliError::input("countdown timer needs duration"))?)?;
                        if d.2 != "time" || d.0 <= 0.0 {
                            return Err(CliError::input("countdown timer duration must be positive time"));
                        }
                    }
                    "stopwatch" => {
                        if let Some(dur) = get("duration").filter(|v| !v.is_null()) {
                            asterdoc_quantity(dur, "time")?;
                        }
                    }
                    "cycle" => {
                        let phases = get("phases").and_then(|v| v.as_sequence())
                            .ok_or_else(|| CliError::input("cycle timer needs phases"))?;
                        if phases.is_empty() {
                            return Err(CliError::input("cycle timer needs at least one phase"));
                        }
                        if let Some(rounds) = get("rounds").and_then(|v| v.as_i64()) {
                            if rounds < 1 {
                                return Err(CliError::input("cycle timer rounds must be >= 1"));
                            }
                        }
                        for phase in phases {
                            let pname = asterdoc_map_get(phase, "name").and_then(|v| v.as_str()).unwrap_or("").trim();
                            if pname.is_empty() {
                                return Err(CliError::input("cycle timer phase needs name"));
                            }
                            let pmode = asterdoc_map_get(phase, "mode").and_then(|v| v.as_str()).unwrap_or("").trim();
                            if pmode != "open" || asterdoc_map_get(phase, "duration").is_some_and(|v| !v.is_null()) {
                                let dval = asterdoc_map_get(phase, "duration")
                                    .ok_or_else(|| CliError::input(format!("cycle timer phase {pname} needs duration")))?;
                                let d = asterdoc_quantity_parts(dval)?;
                                if d.2 != "time" || d.0 <= 0.0 {
                                    return Err(CliError::input(format!("cycle timer phase {pname} duration must be positive time")));
                                }
                            }
                        }
                    }
                    "window" => {
                        if text("anchor").is_empty() && text("schedule_window").is_empty() {
                            return Err(CliError::input("window timer needs anchor or schedule_window"));
                        }
                        if let Some(off) = get("offset").filter(|v| !v.is_null()) {
                            let d = asterdoc_quantity_parts(off)?;
                            if d.2 != "time" || d.0 < 0.0 {
                                return Err(CliError::input("window timer offset must be non-negative time"));
                            }
                        }
                    }
                    bad_mode => {
                        return Err(CliError::input(format!("unsupported timer mode {bad_mode}")));
                    }
                }
            }
            "attachment-fields" => {
                if let Some(slots) = get("slots_per_cell").and_then(|v| v.as_i64()) {
                    if slots < 1 {
                        return Err(CliError::input("attachment slots_per_cell must be >= 1"));
                    }
                }
                if text("scope") == "container_grid" && text("container_ref").is_empty() {
                    return Err(CliError::input("container_grid attachment needs container_ref"));
                }
            }
            "subflow-fields" => {
                if text("title").is_empty() {
                    return Err(CliError::input("subflow needs title"));
                }
                let items = get("items").and_then(|v| v.as_sequence())
                    .ok_or_else(|| CliError::input("subflow needs items"))?;
                for item in items {
                    let sid = asterdoc_map_get(item, "id").and_then(|v| v.as_str()).unwrap_or("").trim();
                    let stitle = asterdoc_map_get(item, "title").and_then(|v| v.as_str()).unwrap_or("").trim();
                    if sid.is_empty() || stitle.is_empty() {
                        return Err(CliError::input("subflow item needs non-empty id and title"));
                    }
                }
            }
            other => {
                return Err(CliError::internal(format!(
                    "unknown asterlab module rule {other}"
                )))
            }
        }
    }
    Ok(())
}


// --- AsterDoc compiler -------------------------------------------------------

/// compile_asterdoc_to_design turns the authored modules into the legacy
/// Design projection. The source remains authoritative; this projection is
/// written only when a Run is frozen.
pub(crate) fn compile_asterdoc_to_design(source: &str) -> Result<Value, CliError> {
    validate_asterdoc_source(source)?;
    let modules = parse_asterdoc_modules(source)?;
    compile_modules_to_design(&modules)
}

pub(crate) fn parse_asterdoc_modules(source: &str) -> Result<Vec<serde_yaml::Value>, CliError> {
    let mut modules = Vec::new();
    let fm_end = frontmatter_end_offset(source);
    let mut in_fence = false;
    let mut in_other_fence = false;
    let mut fence_ticks = 0usize;
    let mut body = String::new();
    for line in source[fm_end..].lines() {
        let trimmed = line.trim();
        if !in_fence && !in_other_fence {
            let ticks = leading_backticks(trimmed);
            if ticks >= 3 {
                let lang = trimmed[ticks..].trim();
                if ticks == 3 && lang == "asterlab" {
                    in_fence = true;
                    fence_ticks = ticks;
                    body.clear();
                } else {
                    in_other_fence = true;
                    fence_ticks = ticks;
                }
            }
            continue;
        }
        if in_other_fence {
            if is_closing_backtick_fence(trimmed, fence_ticks) {
                in_other_fence = false;
                fence_ticks = 0;
            }
            continue;
        }
        if is_closing_backtick_fence(trimmed, fence_ticks) {
            let module: serde_yaml::Value = serde_yaml::from_str(&body)
                .map_err(|e| CliError::input(format!("asterlab block: {e}")))?;
            modules.push(module);
            in_fence = false;
            fence_ticks = 0;
            continue;
        }
        body.push_str(line);
        body.push('\n');
    }
    Ok(modules)
}

pub(crate) fn ytext(module: &serde_yaml::Value, key: &str) -> String {
    module
        .get(key)
        .and_then(|value| value.as_str())
        .map(str::trim)
        .unwrap_or("")
        .to_owned()
}

pub(crate) fn ystrings(module: &serde_yaml::Value, key: &str) -> Vec<String> {
    module
        .get(key)
        .and_then(|value| value.as_sequence())
        .into_iter()
        .flatten()
        .filter_map(|value| value.as_str().map(str::to_owned))
        .collect()
}

pub(crate) fn ywells(module: &serde_yaml::Value, group: &str) -> Vec<String> {
    module
        .get("groups")
        .and_then(|value| value.as_mapping())
        .and_then(|groups| groups.get(serde_yaml::Value::String(group.to_owned())))
        .and_then(|value| value.as_sequence())
        .into_iter()
        .flatten()
        .filter_map(|value| value.as_str().map(|well| well.trim().to_uppercase()))
        .collect()
}

pub(crate) fn compile_modules_to_design(modules: &[serde_yaml::Value]) -> Result<Value, CliError> {
    let mut experiments = Vec::new();
    let mut group_modules = Vec::new();
    let mut group_table_rows: Vec<(String, serde_yaml::Value)> = Vec::new();
    let mut assays = Vec::new();
    let mut steps = Vec::new();
    let mut workflows = Vec::new();
    let mut plates = Vec::new();
    let mut by_id: std::collections::HashMap<String, &serde_yaml::Value> = std::collections::HashMap::new();
    for module in modules {
        let mid = ytext(module, "id");
        if !mid.is_empty() {
            by_id.insert(mid, module);
        }
        match ytext(module, "kind").as_str() {
            "experiment" => experiments.push(module),
            "group" => group_modules.push(module),
            "group_table" => {
                for row in module.get("rows").and_then(|v| v.as_sequence()).into_iter().flatten() {
                    let gid = ytext(row, "id");
                    if !gid.is_empty() {
                        group_table_rows.push((gid, row.clone()));
                    }
                }
            }
            "assay" => assays.push(module),
            "step" => {
                steps.push(module);
                for nested in module.get("blocks").and_then(|v| v.as_sequence()).into_iter().flatten() {
                    if ytext(nested, "kind") == "group_table" {
                        for row in nested.get("rows").and_then(|v| v.as_sequence()).into_iter().flatten() {
                            let gid = ytext(row, "id");
                            if !gid.is_empty() {
                                group_table_rows.push((gid, row.clone()));
                            }
                        }
                    }
                }
            }
            "workflow" => workflows.push(module),
            "plate" => plates.push(module),
            _ => {}
        }
    }
    if experiments.len() != 1 {
        return Err(CliError::input(
            "a design document needs exactly one experiment module",
        ));
    }
    let experiment = experiments[0];
    let mut group_name_by_id: std::collections::HashMap<String, String> = std::collections::HashMap::new();
    let mut group_specs: Vec<Value> = Vec::new();
    let mut seen_group_ids: std::collections::HashSet<String> = std::collections::HashSet::new();
    let push_group = |val: &serde_yaml::Value,
                      specs: &mut Vec<Value>,
                      names: &mut std::collections::HashMap<String, String>,
                      seen: &mut std::collections::HashSet<String>| {
        let gid = ytext(val, "id");
        let name = ytext(val, "name");
        if gid.is_empty() || !seen.insert(gid.clone()) {
            return;
        }
        names.insert(gid.clone(), name.clone());
        let mut spec = json!({
            "id": gid,
            "name": name,
        });
        let role = ytext(val, "role");
        if !role.is_empty() {
            spec["role"] = json!(match role.as_str() {
                "control" | "treatment" | "comparator" | "reference" => role,
                "blank" | "vehicle" | "positive_control" | "negative_control" => "control".to_owned(),
                "standard" | "ntc" => "reference".to_owned(),
                _ => "other".to_owned(),
            });
        }
        if !ytext(val, "condition").is_empty() {
            spec["condition"] = json!(ytext(val, "condition"));
        }
        if let Some(n) = val.get("independent_n").and_then(|v| v.as_u64()) {
            spec["independent_n"] = json!(n);
        }
        if let Some(n) = val.get("technical_repeats").and_then(|v| v.as_u64()) {
            spec["technical_repeats"] = json!(n);
        }
        specs.push(spec);
    };
    for group in &group_modules {
        push_group(group, &mut group_specs, &mut group_name_by_id, &mut seen_group_ids);
    }
    for (_, row) in &group_table_rows {
        push_group(row, &mut group_specs, &mut group_name_by_id, &mut seen_group_ids);
    }

    let reference_index: std::collections::HashMap<String, String> = modules
        .iter()
        .filter(|module| ytext(module, "kind") == "reference" && ytext(module, "view") == "protocol_block")
        .map(|module| (ytext(module, "id"), ytext(module, "ref")))
        .filter(|(id, target)| !id.is_empty() && !target.is_empty())
        .collect();

    let all_group_ids: Vec<String> = group_specs
        .iter()
        .filter_map(|g| g["id"].as_str().map(str::to_owned))
        .collect();

    let mut assay_plans: Vec<Value> = assays
        .iter()
        .map(|assay| {
            let group_ids: Vec<String> = ystrings(assay, "group_refs");
            let group_names: Vec<String> = group_ids
                .iter()
                .map(|id| group_name_by_id.get(id).cloned().unwrap_or_default())
                .collect();
            let mut control_group_ids: Vec<String> = Vec::new();
            let ctrl_ref = ytext(assay, "control_group_ref");
            if !ctrl_ref.is_empty() {
                control_group_ids.push(ctrl_ref);
            }
            let mut plan = json!({
                "id": ytext(assay, "id"),
                "title": ytext(assay, "title"),
                "endpoint": ytext(assay, "endpoint"),
                "method": ytext(assay, "method"),
                "unit": ytext(assay, "unit"),
                "timepoints": ystrings(assay, "timepoints"),
                "group_ids": group_ids,
                "group_names": group_names,
                "controls": ystrings(assay, "controls"),
                "control_group_ids": control_group_ids,
                "timepoint_timing": [],
            });
            if !ytext(assay, "protocol_version_id").is_empty() {
                plan["protocol_version_id"] = json!(ytext(assay, "protocol_version_id"));
            }
            let relation = ytext(assay, "timepoint_relation");
            if !relation.is_empty() {
                plan["timepoint_relation"] = json!(relation);
            }
            plan
        })
        .collect();

    if assay_plans.is_empty() {
        let title = {
            let t = ytext(experiment, "title");
            if t.is_empty() { "主实验".to_owned() } else { t }
        };
        let endpoint = {
            let e = ytext(experiment, "primary_endpoint");
            if e.is_empty() { "观测指标".to_owned() } else { e }
        };
        let method = {
            let m = ytext(experiment, "endpoint_method");
            if m.is_empty() { "通用记录".to_owned() } else { m }
        };
        let group_names: Vec<String> = all_group_ids
            .iter()
            .map(|id| group_name_by_id.get(id).cloned().unwrap_or_default())
            .collect();
        assay_plans.push(json!({
            "id": "assay-main",
            "title": title,
            "endpoint": endpoint,
            "method": method,
            "unit": ytext(experiment, "endpoint_unit"),
            "timepoints": ystrings(experiment, "timepoints"),
            "group_ids": all_group_ids,
            "group_names": group_names,
            "controls": [],
            "control_group_ids": [],
            "timepoint_timing": [],
        }));
    }

    let default_assay_id = assay_plans
        .first()
        .and_then(|a| a["id"].as_str())
        .unwrap_or("assay-main")
        .to_owned();

    let mut mygo_assays = serde_json::Map::new();
    for (assay_idx, assay_plan) in assay_plans.iter().enumerate() {
        let assay_id = assay_plan["id"].as_str().unwrap_or("").to_owned();
        let assay_yaml = assays.iter().find(|a| ytext(a, "id") == assay_id).copied();
        let assay_timepoints: Vec<String> = assay_plan["timepoints"]
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|v| v.as_str().map(str::to_owned))
            .collect();
        let control_ref = assay_yaml.map(|a| ytext(a, "control_group_ref")).unwrap_or_default();
        let control_group = group_name_by_id.get(&control_ref).cloned().unwrap_or_default();
        let tech_reps = assay_yaml
            .and_then(|a| a.get("technical_replicates"))
            .and_then(|v| v.as_u64())
            .unwrap_or(0);
        let mut wells = Vec::new();
        for plate in &plates {
            let plate_assay = ytext(plate, "assay");
            let matches_assay = plate_assay == assay_id || (plate_assay.is_empty() && assay_idx == 0);
            if !matches_assay {
                continue;
            }
            let mut sample_lookup: std::collections::HashMap<String, String> =
                std::collections::HashMap::new();
            let mut replicate_lookup: std::collections::HashMap<String, i64> =
                std::collections::HashMap::new();
            if let Some(samples) = plate.get("samples").and_then(|v| v.as_mapping()) {
                for (key, value) in samples {
                    let well = key.as_str().unwrap_or("").trim().to_uppercase();
                    let sample = value.as_str().map(str::trim).unwrap_or("").to_owned();
                    sample_lookup.insert(well, sample);
                }
            }
            if let Some(replicates) = plate.get("replicates").and_then(|v| v.as_mapping()) {
                for (key, value) in replicates {
                    let well = key.as_str().unwrap_or("").trim().to_uppercase();
                    replicate_lookup.insert(well, value.as_i64().unwrap_or(0));
                }
            }
            let groups_map = plate.get("groups")
                .and_then(|value| value.as_mapping())
                .cloned()
                .unwrap_or_default();
            let plate_timepoint = ytext(plate, "timepoint");
            let plate_timepoints = if plate_timepoint.is_empty() {
                if assay_timepoints.is_empty() { vec![String::new()] } else { assay_timepoints.clone() }
            } else {
                vec![plate_timepoint]
            };
            for (group_value, wells_value) in groups_map {
                let group_id = group_value.as_str().unwrap_or("").to_owned();
                if !group_name_by_id.contains_key(&group_id) {
                    continue;
                }
                let mut coordinates: Vec<String> = wells_value
                    .as_sequence()
                    .into_iter()
                    .flatten()
                    .filter_map(|value| value.as_str().map(|well| well.trim().to_uppercase()))
                    .collect();
                coordinates.sort_by_key(|well| {
                    parse_well_coordinate(well).unwrap_or(usize::MAX)
                });
                for coordinate in &coordinates {
                    for timepoint in &plate_timepoints {
                        wells.push(json!({
                            "wellId": coordinate,
                            "sampleId": sample_lookup.get(coordinate).cloned().unwrap_or_default(),
                            "groupId": group_id,
                            "replicate": replicate_lookup.get(coordinate).copied().unwrap_or(0),
                            "timepoint": timepoint
                        }));
                    }
                }
            }
        }
        mygo_assays.insert(
            assay_id,
            json!({
                "timepoints": assay_timepoints,
                "technicalReplicates": tech_reps,
                "controlGroup": control_group,
                "calibratorGroup": "",
                "referenceGenes": [],
                "wells": wells
            }),
        );
    }

    let mut nodes = Vec::new();
    let mut step_assays = serde_json::Map::new();
    let mut step_execution = serde_json::Map::new();
    let mut step_tasks = Vec::new();
    let mut step_checks = Vec::new();
    let mut step_details = Vec::new();
    let mut step_uses = serde_json::Map::new();
    let mut step_blocks = serde_json::Map::new();
    let mut asterlab_steps = serde_json::Map::new();
    let mut asterlab_step_params = serde_json::Map::new();
    let mut asterlab_step_timers = serde_json::Map::new();
    let mut asterlab_step_tables = serde_json::Map::new();
    let mut asterlab_step_capture = serde_json::Map::new();

    for (index, step) in steps.iter().enumerate() {
        let id = ytext(step, "id");
        let title = ytext(step, "title");
        let execution = {
            let e = ytext(step, "execution");
            if e.is_empty() { "executable".to_owned() } else { e }
        };
        let is_executable = execution == "executable";
        let assay = {
            let a = ytext(step, "assay");
            if a.is_empty() && is_executable { default_assay_id.clone() } else { a }
        };
        let path = {
            let p = ytext(step, "path");
            if p.is_empty() && is_executable {
                ast::derive_step_path(&id).unwrap_or_else(|_| format!("root/{id}"))
            } else {
                p
            }
        };
        nodes.push(json!({
            "id": id,
            "kind": "flask",
            "title": title,
            "pos": {"X": index as f64 * 240.0, "Y": 0.0}
        }));
        step_assays.insert(id.clone(), json!(assay));
        step_execution.insert(id.clone(), json!(execution));
        if let Some(uses) = step.get("uses").and_then(|value| value.as_sequence()) {
            if !uses.is_empty() {
                let blocks: Vec<String> = uses
                    .iter()
                    .filter_map(|use_item| use_item.get("ref").and_then(|value| value.as_str()))
                    .filter_map(|reference| reference_index.get(reference).cloned())
                    .collect();
                if !blocks.is_empty() {
                    step_blocks.insert(id.clone(), serde_json::to_value(blocks).unwrap_or(Value::Null));
                }
                step_uses.insert(id.clone(), serde_json::to_value(uses).unwrap_or(Value::Null));
            }
        }

        let mut checks: Vec<Value> = step.get("checks")
            .and_then(|value| value.as_sequence())
            .into_iter()
            .flatten()
            .filter_map(|check| {
                let mapping = check.as_mapping()?;
                let get = |key: &str| mapping.get(serde_yaml::Value::String(key.to_owned()))
                    .and_then(|value| value.as_str()).unwrap_or("").to_owned();
                Some(json!({
                    "check_id": get("checkId"),
                    "label": get("title"),
                    "blocking": mapping.get(serde_yaml::Value::String("blocking".to_owned()))
                        .and_then(|value| value.as_bool()).unwrap_or(false)
                }))
            })
            .collect();

        let mut resolved_blocks: Vec<Value> = Vec::new();
        for inline in step.get("blocks").and_then(|v| v.as_sequence()).into_iter().flatten() {
            if let Ok(v) = serde_json::to_value(inline) {
                resolved_blocks.push(v);
            }
        }
        for ref_id in ystrings(step, "block_refs") {
            if let Some(target) = by_id.get(&ref_id) {
                if let Ok(v) = serde_json::to_value(target) {
                    resolved_blocks.push(v);
                }
            }
        }
        if !resolved_blocks.is_empty() {
            asterlab_steps.insert(id.clone(), Value::Array(resolved_blocks.clone()));
            let mut params_acc = Vec::new();
            let mut timers_acc = Vec::new();
            let mut tables_acc = Vec::new();
            let mut capture_acc = Vec::new();
            for blk in &resolved_blocks {
                match blk["kind"].as_str().unwrap_or("") {
                    "checklist" => {
                        for item in blk["items"].as_array().into_iter().flatten() {
                            let cid = item["id"].as_str().unwrap_or("").trim();
                            let label = item["label"].as_str().unwrap_or("").trim();
                            if !cid.is_empty() && !label.is_empty() {
                                checks.push(json!({
                                    "check_id": cid,
                                    "label": label,
                                    "blocking": item["blocking"]
                                        .as_bool()
                                        .or_else(|| item["required"].as_bool())
                                        .unwrap_or(false),
                                    "allow_rule_skip": item["allow_rule_skip"].as_bool().unwrap_or(false),
                                    "skip_rule": item["skip_rule"].as_str().unwrap_or("")
                                }));
                            }
                        }
                    }
                    "param_table" => {
                        for row in blk["rows"].as_array().into_iter().flatten() {
                            params_acc.push(row.clone());
                        }
                    }
                    "timer" => timers_acc.push(blk.clone()),
                    "data_table" => tables_acc.push(blk.clone()),
                    "attachment" => {
                        let scope = blk["scope"].as_str().unwrap_or("");
                        if scope == "container_grid" || scope == "step_evidence" {
                            capture_acc.push(json!({
                                "id": blk["id"],
                                "scope": scope,
                                "container_ref": blk["container_ref"],
                                "slots_per_cell": blk["slots_per_cell"].as_u64().unwrap_or(1),
                                "media_hint": blk["media_hint"],
                            }));
                        }
                    }
                    _ => {}
                }
            }
            if !params_acc.is_empty() {
                asterlab_step_params.insert(id.clone(), Value::Array(params_acc));
            }
            if !timers_acc.is_empty() {
                asterlab_step_timers.insert(id.clone(), Value::Array(timers_acc));
            }
            if !tables_acc.is_empty() {
                asterlab_step_tables.insert(id.clone(), Value::Array(tables_acc));
            }
            if !capture_acc.is_empty() {
                asterlab_step_capture.insert(id.clone(), Value::Array(capture_acc));
            }
        }

        if !checks.is_empty() {
            step_checks.push(json!({"stepId": id, "path": path, "checks": checks}));
        }
        if let Some(tasks) = step.get("tasks").and_then(|value| value.as_sequence()) {
            if !tasks.is_empty() {
                let tasks: Vec<Value> = tasks.iter().map(|task| {
                    serde_json::to_value(task).unwrap_or(Value::Null)
                }).collect();
                step_details.push(json!({"stepId": id, "tasks": tasks}));
            }
        }
        let timepoints = ystrings(step, "timepoints");
        if is_executable {
            if timepoints.is_empty() {
                step_tasks.push(json!({"stepId": id, "assayId": assay, "timepoint": "", "path": path}));
            } else {
                for timepoint in timepoints {
                    step_tasks.push(json!({"stepId": id, "assayId": assay, "timepoint": timepoint, "path": path}));
                }
            }
        }
    }
    let workflow = workflows.first();
    let business_order = workflow
        .map(|workflow| ystrings(workflow, "business_order"))
        .filter(|order| !order.is_empty())
        .unwrap_or_else(|| steps.iter().map(|step| ytext(step, "id")).collect());
    let edges: Vec<Value> = workflow
        .and_then(|workflow| workflow.get("edges"))
        .and_then(|value| value.as_sequence())
        .into_iter()
        .flatten()
        .enumerate()
        .map(|(index, edge)| json!({
            "ID": format!("e{}", index + 1),
            "From": ytext(edge, "from"),
            "To": ytext(edge, "to")
        }))
        .collect();
    let mygo = json!({
        "workflow": {"Nodes": nodes, "Edges": edges, "BusinessOrder": business_order},
        "stepExecution": step_execution,
        "stepAssays": step_assays,
        "stepTasks": step_tasks,
        "stepChecks": step_checks,
        "stepDetails": step_details,
        "stepUses": step_uses,
        "stepBlocks": step_blocks,
        "assays": mygo_assays
    });

    let mut design = json!({
        "title": ytext(experiment, "title"),
        "objective": ytext(experiment, "objective"),
        "research_question": if ytext(experiment, "research_question").is_empty() { Value::Null } else { json!(ytext(experiment, "research_question")) },
        "hypothesis": ytext(experiment, "hypothesis"),
        "experimental_unit": ytext(experiment, "experimental_unit"),
        "factors": experiment.get("factors")
            .and_then(|value| value.as_sequence())
            .map(|items| items.iter().filter_map(|item| serde_json::to_value(item).ok()).collect::<Vec<_>>())
            .unwrap_or_default(),
        "timepoints": ystrings(experiment, "timepoints"),
        "primary_endpoint": ytext(experiment, "primary_endpoint"),
        "endpoint_method": ytext(experiment, "endpoint_method"),
        "endpoint_unit": ytext(experiment, "endpoint_unit"),
        "controls": ytext(experiment, "controls"),
        "analysis_plan": ytext(experiment, "analysis_plan"),
        "sample_size_rationale": ytext(experiment, "sample_size_rationale"),
        "randomization": ytext(experiment, "randomization"),
        "blinding": ytext(experiment, "blinding"),
        "inclusion_criteria": ytext(experiment, "inclusion_criteria"),
        "exclusion_criteria": ytext(experiment, "exclusion_criteria"),
        "planned_run_count": experiment.get("planned_run_count").and_then(|v| v.as_u64()),
        "planned_run_relationship": if ytext(experiment, "planned_run_relationship").is_empty() { "unknown".to_owned() } else { ytext(experiment, "planned_run_relationship") },
        "group_specs": group_specs,
        "assays": assay_plans,
        "mygo": mygo
    });
    let core_kinds = ["experiment", "group", "assay", "step", "workflow", "plate", "reference"];
    let sdk_modules: Vec<Value> = modules
        .iter()
        .filter(|module| !core_kinds.contains(&ytext(module, "kind").as_str()))
        .filter_map(|module| serde_json::to_value(module).ok())
        .collect();
    if !sdk_modules.is_empty() || !asterlab_steps.is_empty() {
        design["asterlab"] = json!({
            "foundationVersion": 1,
            "sdkVersion": asterdoc_spec()["sdkVersion"].as_str().unwrap_or("2.0.0"),
            "modules": sdk_modules,
            "steps": asterlab_steps,
            "stepParameters": asterlab_step_params,
            "stepTimers": asterlab_step_timers,
            "stepTables": asterlab_step_tables,
            "stepCapture": asterlab_step_capture
        });
    }
    if design["research_question"].is_null() {
        design.as_object_mut().unwrap().remove("research_question");
    }
    let mut validation = design.clone();
    if let Some(object) = validation.as_object_mut() {
        object.remove("mygo");
        object.remove("asterlab");
    }
    let parsed: DesignFields = serde_json::from_value(validation)
        .map_err(|e| CliError::input(format!("AsterDoc projection is not a valid design: {e}")))?;
    validate_design(&parsed)?;
    Ok(design)
}

pub(crate) fn parse_well_coordinate(well: &str) -> Option<usize> {
    let mut chars = well.chars();
    let row = chars.next()? as usize;
    if !(b'A' as usize..=b'Z' as usize).contains(&row) {
        return None;
    }
    let col: usize = chars.as_str().parse().ok()?;
    Some(((row - b'A' as usize) + 1) * 1000 + col)
}


/// create_asterdoc_design creates the Design record and its first immutable
/// AsterDoc revision. The Design projection is created later by the compiler.
pub(crate) fn create_asterdoc_design(db: &mut Connection, input: &Value) -> Result<Value, CliError> {
    let project_id = input.get("project_id").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("project_id is required"))?;
    let title = input.get("title").and_then(Value::as_str).unwrap_or("新实验").trim();
    let source = input.get("source").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("source is required"))?;
    validate_asterdoc_source(source)?;
    let created = create_design_record(db, project_id, &json!({"title": title}))?;
    let design_id = created["design"]["id"].as_str()
        .ok_or_else(|| CliError::internal("created design has no id"))?;
    let mut doc = save_asterdoc_version(db, &json!({
        "design_id": design_id,
        "source": source,
        "expected_revision": 0,
        "operation_id": input.get("operation_id"),
        "actor_id": input.get("actor_id")
    }))?;
    doc["design_id"] = json!(design_id);
    Ok(doc)
}

pub(crate) fn derive_asterdoc_source_from_run(
    source: &str,
    run_id: &str,
    parameter_updates: &[asterlab_core::design_commands::DerivedParameterUpdate],
    duration_updates: &[asterlab_core::design_commands::DerivedDurationUpdate],
) -> String {
    let Ok(blocks) = asterdoc_blocks(source) else {
        return source.to_owned();
    };
    let mut replacements: Vec<(usize, usize, String)> = Vec::new();
    for block in &blocks {
        let bkind = yaml_kind(&block.value);
        if bkind != "step" && bkind != "param_table" && bkind != "timer" {
            continue;
        }
        let mut updated = block.value.clone();
        let mut changed = false;
        let patch_param_table = |tbl: &mut serde_yaml::Value, step_path: &str| -> bool {
            let mut tbl_changed = false;
            let Some(rows) = tbl.get_mut("rows").and_then(|v| v.as_sequence_mut()) else {
                return false;
            };
            for row in rows {
                let key = asterdoc_map_get(row, "key").and_then(|v| v.as_str()).unwrap_or("").trim().to_owned();
                let label = asterdoc_map_get(row, "label").and_then(|v| v.as_str()).unwrap_or("").trim().to_owned();
                if let Some(upd) = parameter_updates.iter().find(|u| {
                    let u_sp = u.step_path.as_deref().unwrap_or("");
                    let u_lbl = u.label.as_deref().unwrap_or("");
                    (u_sp == step_path || step_path.is_empty() || u_sp.ends_with(&format!("/{step_path}")))
                        && (u.parameter_key == key || (!label.is_empty() && u_lbl == label))
                }) {
                    if let Some(rmap) = row.as_mapping_mut() {
                        if let Ok(val_yaml) = serde_json::from_value::<serde_yaml::Value>(upd.actual.clone()) {
                            rmap.insert(serde_yaml::Value::String("value".to_owned()), val_yaml);
                            if let Some(unit) = upd.unit.as_deref().filter(|s| !s.is_empty()) {
                                rmap.insert(serde_yaml::Value::String("unit".to_owned()), serde_yaml::Value::String(unit.to_owned()));
                            }
                            tbl_changed = true;
                        }
                    }
                }
            }
            tbl_changed
        };
        if bkind == "param_table" {
            changed |= patch_param_table(&mut updated, "");
        } else if bkind == "step" {
            let spath = step_path(&block.value);
            let sid = yaml_id(&block.value);
            if let Some(nested) = updated.get_mut("blocks").and_then(|v| v.as_sequence_mut()) {
                for blk in nested {
                    let nk = yaml_kind(blk);
                    if nk == "param_table" {
                        changed |= patch_param_table(blk, if spath.is_empty() { &sid } else { &spath });
                    } else if nk == "timer" && yaml_kind(blk) == "timer" {
                        let bid = yaml_id(blk);
                        if let Some(du) = duration_updates.iter().find(|d| d.timer_id == bid) {
                            if let Some(tm) = blk.as_mapping_mut() {
                                if let Ok(dur) = serde_yaml::from_str::<serde_yaml::Value>(&format!("value: {}\nunit: s", du.actual_s)) {
                                    tm.insert(serde_yaml::Value::String("duration".to_owned()), dur);
                                    changed = true;
                                }
                            }
                        }
                    }
                }
            }
        }
        if changed {
            if let Ok(raw) = serde_yaml::to_string(&updated) {
                replacements.push((block.body_start, block.body_end, raw));
            }
        }
    }
    let mut next = if replacements.is_empty() {
        source.to_owned()
    } else {
        apply_replacements(source.to_owned(), replacements)
    };
    let provenance = format!("> 派生自 Run {run_id}");
    if !next.contains(&provenance) {
        if !next.ends_with('\n') {
            next.push('\n');
        }
        next.push('\n');
        next.push_str(&provenance);
        next.push('\n');
    }
    next
}

pub(crate) fn flatten_asterdoc_modules<'a>(modules: &'a [serde_yaml::Value]) -> Vec<&'a serde_yaml::Value> {
    let mut flat = Vec::with_capacity(modules.len());
    for module in modules {
        flat.push(module);
        if yaml_kind(module) == "step" {
            if let Some(blocks) = module.get("blocks").and_then(|v| v.as_sequence()) {
                for item in blocks {
                    if item.is_mapping() {
                        flat.push(item);
                    }
                }
            }
        }
    }
    flat
}

fn yaml_num_f64(value: Option<&serde_yaml::Value>) -> Option<f64> {
    let v = value?;
    v.as_f64()
        .or_else(|| v.as_i64().map(|n| n as f64))
        .or_else(|| v.as_u64().map(|n| n as f64))
        .filter(|n| n.is_finite())
}

fn yaml_quantity_parts(value: Option<&serde_yaml::Value>) -> Option<(f64, String, String, f64)> {
    asterdoc_quantity_parts(value?).ok()
}

fn timer_quantity_seconds(value: Option<&serde_yaml::Value>) -> i64 {
    if let Some((num, _, dim, factor)) = yaml_quantity_parts(value) {
        if dim == "time" && num > 0.0 {
            return (num * factor + 0.5) as i64;
        }
    }
    0
}

/// compute_asterdoc_budget mirrors Go's ComputeBudgetSummary for structural counts and timer budgets.
pub(crate) fn compute_asterdoc_budget(modules: &[serde_yaml::Value]) -> Value {
    use std::collections::HashSet;
    let flat = flatten_asterdoc_modules(modules);
    let mut groups_seen: HashSet<String> = HashSet::new();
    let mut steps: i64 = 0;
    let mut executable_steps: i64 = 0;
    let mut wells: i64 = 0;
    let mut hands_on_sec: i64 = 0;
    let mut total_span_sec: i64 = 0;
    let mut has_open_phase = false;

    for m in &flat {
        match yaml_kind(m).as_str() {
            "group" => {
                let id = yaml_id(m);
                if !id.is_empty() {
                    groups_seen.insert(id);
                }
            }
            "group_table" => {
                if let Some(rows) = m.get("rows").and_then(|v| v.as_sequence()) {
                    for row in rows {
                        let gid = ytext(row, "id");
                        if !gid.is_empty() {
                            groups_seen.insert(gid);
                        }
                    }
                }
            }
            "plate" => {
                let mut plate_wells: HashSet<String> = HashSet::new();
                if let Some(groups) = m.get("groups").and_then(|v| v.as_mapping()) {
                    for (k, v) in groups {
                        if let Some(gname) = k.as_str().map(str::trim).filter(|s| !s.is_empty()) {
                            groups_seen.insert(gname.to_owned());
                        }
                        if let Some(seq) = v.as_sequence() {
                            for item in seq {
                                if let Some(w) = item.as_str().map(|s| s.trim().to_uppercase()).filter(|s| !s.is_empty()) {
                                    plate_wells.insert(w);
                                }
                            }
                        }
                    }
                }
                wells += plate_wells.len() as i64;
            }
            "step" => {
                steps += 1;
                let exec = ytext(m, "execution");
                if exec.is_empty() || exec == "executable" {
                    executable_steps += 1;
                }
            }
            "timer" => match ytext(m, "mode").as_str() {
                "countdown" => {
                    let s = timer_quantity_seconds(m.get("duration"));
                    hands_on_sec += s;
                    total_span_sec += s;
                }
                "stopwatch" => {
                    let s = timer_quantity_seconds(m.get("duration"));
                    hands_on_sec += s;
                    total_span_sec += s;
                    if s == 0 {
                        has_open_phase = true;
                    }
                }
                "cycle" => {
                    let rounds = yaml_num_f64(m.get("rounds"))
                        .filter(|r| *r >= 1.0)
                        .map(|r| r as i64)
                        .unwrap_or(1);
                    let mut per_round = 0i64;
                    if let Some(phases) = m.get("phases").and_then(|v| v.as_sequence()) {
                        for ph in phases {
                            if ytext(ph, "mode") == "open" {
                                has_open_phase = true;
                            }
                            per_round += timer_quantity_seconds(ph.get("duration"));
                        }
                    }
                    let tot = per_round * rounds;
                    hands_on_sec += tot;
                    total_span_sec += tot;
                }
                "window" => {
                    total_span_sec += timer_quantity_seconds(m.get("offset"));
                }
                _ => {}
            },
            _ => {}
        }
    }

    json!({
        "steps": steps,
        "executable_steps": executable_steps,
        "groups": groups_seen.len() as i64,
        "wells": wells,
        "hands_on_sec": hands_on_sec,
        "total_span_sec": total_span_sec,
        "has_open_phase": has_open_phase
    })
}

#[derive(Clone, Debug)]
pub(crate) struct CalcSampleInput {
    pub sample_id: String,
    pub value: f64,
    pub unit: String,
    pub endpoint: String,
}

#[derive(Clone, Debug, Default)]
pub(crate) struct CalcActualContext {
    pub params: std::collections::BTreeMap<String, f64>,
    pub param_units: std::collections::BTreeMap<String, String>,
    pub samples: Vec<CalcSampleInput>,
}

fn round4(v: f64) -> f64 {
    (v * 10_000.0).round() / 10_000.0
}

fn fmt_calc_num(v: f64) -> String {
    let r = round4(v);
    if (r - r.round()).abs() < 1e-9 {
        format!("{}", r.round() as i64)
    } else {
        let s = format!("{:.4}", r);
        s.trim_end_matches('0').trim_end_matches('.').to_owned()
    }
}

fn doc_planned_param_float(flat: &[&serde_yaml::Value], key: &str) -> Option<(f64, String)> {
    if key.trim().is_empty() {
        return None;
    }
    for m in flat {
        if yaml_kind(m) != "param_table" {
            continue;
        }
        if let Some(rows) = m.get("rows").and_then(|v| v.as_sequence()) {
            for row in rows {
                let rk = ytext(row, "key");
                let rl = ytext(row, "label");
                if rk.eq_ignore_ascii_case(key) || rl.eq_ignore_ascii_case(key) {
                    if let Some(n) = yaml_num_f64(row.get("value")) {
                        return Some((n, ytext(row, "unit")));
                    }
                }
            }
        }
    }
    None
}

fn eval_formula_with_vars(expr: &str, vars: &std::collections::BTreeMap<String, f64>) -> Result<f64, CliError> {
    validate_formula_expr(expr).map_err(CliError::input)?;
    #[derive(Clone, Debug)]
    enum Tok {
        Num(f64),
        Ident(String),
        Plus,
        Minus,
        Star,
        Slash,
        LParen,
        RParen,
    }
    let bytes = expr.as_bytes();
    let mut tokens = Vec::new();
    let mut i = 0usize;
    while i < bytes.len() {
        let b = bytes[i];
        if b.is_ascii_whitespace() {
            i += 1;
            continue;
        }
        match b {
            b'+' => { tokens.push(Tok::Plus); i += 1; }
            b'-' => { tokens.push(Tok::Minus); i += 1; }
            b'*' => { tokens.push(Tok::Star); i += 1; }
            b'/' => { tokens.push(Tok::Slash); i += 1; }
            b'(' => { tokens.push(Tok::LParen); i += 1; }
            b')' => { tokens.push(Tok::RParen); i += 1; }
            b'0'..=b'9' | b'.' => {
                let start = i;
                while i < bytes.len() && (bytes[i].is_ascii_digit() || bytes[i] == b'.') {
                    i += 1;
                }
                let val = expr[start..i]
                    .parse::<f64>()
                    .map_err(|_| CliError::input(format!("invalid number in formula: {}", &expr[start..i])))?;
                tokens.push(Tok::Num(val));
            }
            b'a'..=b'z' | b'A'..=b'Z' | b'_' => {
                let start = i;
                while i < bytes.len() && (bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_') {
                    i += 1;
                }
                tokens.push(Tok::Ident(expr[start..i].to_owned()));
            }
            _ => return Err(CliError::input(format!("unexpected character '{}' in formula", b as char))),
        }
    }
    fn parse_expr(tokens: &[Tok], pos: &mut usize, vars: &std::collections::BTreeMap<String, f64>) -> Result<f64, CliError> {
        let mut left = parse_term(tokens, pos, vars)?;
        while matches!(tokens.get(*pos), Some(Tok::Plus | Tok::Minus)) {
            let plus = matches!(tokens.get(*pos), Some(Tok::Plus));
            *pos += 1;
            let right = parse_term(tokens, pos, vars)?;
            left = if plus { left + right } else { left - right };
        }
        Ok(left)
    }
    fn parse_term(tokens: &[Tok], pos: &mut usize, vars: &std::collections::BTreeMap<String, f64>) -> Result<f64, CliError> {
        let mut left = parse_factor(tokens, pos, vars)?;
        while matches!(tokens.get(*pos), Some(Tok::Star | Tok::Slash)) {
            let mul = matches!(tokens.get(*pos), Some(Tok::Star));
            *pos += 1;
            let right = parse_factor(tokens, pos, vars)?;
            if mul {
                left *= right;
            } else {
                if right == 0.0 {
                    return Err(CliError::input("division by zero in formula"));
                }
                left /= right;
            }
        }
        Ok(left)
    }
    fn parse_factor(tokens: &[Tok], pos: &mut usize, vars: &std::collections::BTreeMap<String, f64>) -> Result<f64, CliError> {
        match tokens.get(*pos).cloned() {
            Some(Tok::Plus) => {
                *pos += 1;
                parse_factor(tokens, pos, vars)
            }
            Some(Tok::Minus) => {
                *pos += 1;
                Ok(-parse_factor(tokens, pos, vars)?)
            }
            Some(Tok::Num(v)) => {
                *pos += 1;
                Ok(v)
            }
            Some(Tok::Ident(name)) => {
                *pos += 1;
                vars.get(&name)
                    .copied()
                    .ok_or_else(|| CliError::input(format!("missing formula variable {name}")))
            }
            Some(Tok::LParen) => {
                *pos += 1;
                let v = parse_expr(tokens, pos, vars)?;
                if !matches!(tokens.get(*pos), Some(Tok::RParen)) {
                    return Err(CliError::input("unclosed '(' in formula"));
                }
                *pos += 1;
                Ok(v)
            }
            _ => Err(CliError::input("unexpected token in formula")),
        }
    }
    let mut pos = 0usize;
    let out = parse_expr(&tokens, &mut pos, vars)?;
    if !out.is_finite() {
        return Err(CliError::input("formula result is not finite"));
    }
    Ok(out)
}

/// eval_asterdoc_calculator_with_actuals evaluates one calculator@1 module deterministically
/// with optional Run-stage measured parameters and upstream sample concentrations.
pub(crate) fn eval_asterdoc_calculator_with_actuals(
    modules: &[serde_yaml::Value],
    calc_id: &str,
    actuals: Option<&CalcActualContext>,
) -> Result<Value, CliError> {
    use std::collections::BTreeMap;
    let flat = flatten_asterdoc_modules(modules);
    let calc = flat
        .iter()
        .copied()
        .find(|m| yaml_kind(m) == "calculator" && yaml_id(m) == calc_id)
        .ok_or_else(|| CliError::missing(format!("calculator {calc_id} not found")))?;
    let mode = ytext(calc, "mode");
    let mut rows: Vec<Value> = Vec::new();
    let mut used_actuals: Vec<String> = Vec::new();
    let summary: String;

    match mode.as_str() {
        "dilution" => {
            let (mut sv, mut su, mut sdim, mut sfac) = asterdoc_quantity_parts(
                calc.get("stock").ok_or_else(|| CliError::input("calculator dilution needs stock"))?,
            )?;
            let stock_ref = ytext(calc, "stock_ref");
            if !stock_ref.is_empty() {
                let key = stock_ref
                    .strip_prefix("@actual.")
                    .or_else(|| stock_ref.strip_prefix("@param."))
                    .unwrap_or(&stock_ref);
                if let Some(ctx) = actuals {
                    if let Some(&av) = ctx.params.get(key).filter(|v| **v > 0.0) {
                        sv = av;
                        if let Some(au) = ctx.param_units.get(key).filter(|u| !u.trim().is_empty()) {
                            let (dim, fac) = asterdoc_unit(au)
                                .ok_or_else(|| CliError::input(format!("unknown unit {au}")))?;
                            su = au.clone();
                            sdim = dim;
                            sfac = fac;
                        }
                        used_actuals.push(format!("{key}={} {su}", fmt_calc_num(sv)));
                    }
                } else if let Some((pv, pu)) = doc_planned_param_float(&flat, key).filter(|(v, _)| *v > 0.0) {
                    sv = pv;
                    if !pu.is_empty() {
                        if let Some((dim, fac)) = asterdoc_unit(&pu) {
                            su = pu;
                            sdim = dim;
                            sfac = fac;
                        }
                    }
                }
            }
            let (tv, tu, tdim, tfac) = asterdoc_quantity_parts(
                calc.get("target").ok_or_else(|| CliError::input("calculator dilution needs target"))?,
            )?;
            let (fv, fu, fdim, _) = asterdoc_quantity_parts(
                calc.get("final_volume").ok_or_else(|| CliError::input("calculator dilution needs final_volume"))?,
            )?;
            if sdim != tdim || fdim != "volume" {
                return Err(CliError::input("dilution quantities have incompatible units"));
            }
            if sv <= 0.0 || tv <= 0.0 || fv <= 0.0 {
                return Err(CliError::input("dilution quantities must be positive"));
            }
            if tv * tfac > sv * sfac {
                return Err(CliError::input("target concentration exceeds stock concentration"));
            }
            let stock_vol = (tv * tfac / (sv * sfac)) * fv;
            let diluent_vol = fv - stock_vol;
            summary = format!(
                "{} {fu} 母液 + {} {fu} 稀释液",
                fmt_calc_num(stock_vol),
                fmt_calc_num(diluent_vol)
            );
            rows.push(json!({"label": "母液体积 (V1)", "amount": round4(stock_vol), "unit": fu, "note": format!("从 {} {su} 稀释至 {} {tu}", fmt_calc_num(sv), fmt_calc_num(tv))}));
            rows.push(json!({"label": "稀释液体积", "amount": round4(diluent_vol), "unit": fu, "note": "补足至目标终体积"}));
            rows.push(json!({"label": "目标总体积 (V2)", "amount": round4(fv), "unit": fu, "note": "C1·V1 = C2·V2"}));
        }
        "group_mastermix" => {
            let (vpu_val, vpu_unit, vpu_dim, vpu_fac) = asterdoc_quantity_parts(
                calc.get("volume_per_unit").ok_or_else(|| CliError::input("group_mastermix needs volume_per_unit"))?,
            )?;
            if vpu_dim != "volume" || vpu_val <= 0.0 {
                return Err(CliError::input("volume_per_unit must use positive volume unit"));
            }
            let excess = yaml_num_f64(calc.get("excess_ratio")).unwrap_or(0.0).max(0.0);
            let mut group_counts: BTreeMap<String, i64> = BTreeMap::new();
            let mut plate_ref = ytext(calc, "plate_ref");
            if plate_ref.is_empty() {
                plate_ref = ytext(calc, "container_ref");
            }
            if !plate_ref.is_empty() {
                if let Some(plate) = flat.iter().copied().find(|m| yaml_kind(m) == "plate" && yaml_id(m) == plate_ref) {
                    if let Some(groups) = plate.get("groups").and_then(|v| v.as_mapping()) {
                        for (gk, gv) in groups {
                            if let (Some(gname), Some(seq)) = (gk.as_str(), gv.as_sequence()) {
                                if !seq.is_empty() {
                                    group_counts.insert(gname.to_owned(), seq.len() as i64);
                                }
                            }
                        }
                    }
                }
            }
            if group_counts.is_empty() {
                let sc = yaml_num_f64(calc.get("sample_count")).filter(|v| *v >= 1.0).unwrap_or(1.0) as i64;
                group_counts.insert("总体系".to_owned(), sc);
            }
            let mut total_units = 0i64;
            for (g, count) in &group_counts {
                total_units += *count;
                let gvol = (*count as f64) * vpu_val * (1.0 + excess);
                rows.push(json!({
                    "label": format!("{g} 体系 ({count} 孔/份)"),
                    "amount": round4(gvol),
                    "unit": vpu_unit,
                    "note": format!("含 {}% 余量", (excess * 100.0).round() as i64)
                }));
            }
            let total_vol = (total_units as f64) * vpu_val * (1.0 + excess);
            if let Some(comps) = calc.get("components").and_then(|v| v.as_sequence()) {
                for comp in comps {
                    let cname = ytext(comp, "name");
                    if cname.is_empty() {
                        continue;
                    }
                    if let Some(ratio) = yaml_num_f64(comp.get("ratio")).filter(|r| *r > 0.0) {
                        rows.push(json!({"label": cname, "amount": round4(total_vol * ratio), "unit": vpu_unit, "note": format!("配比系数 {}", fmt_calc_num(ratio))}));
                    } else if let Some(target_pct) = yaml_num_f64(comp.get("target_percent")) {
                        let stock_pct = yaml_num_f64(comp.get("stock_percent")).unwrap_or(0.0);
                        if stock_pct > 0.0 {
                            rows.push(json!({"label": cname, "amount": round4(total_vol * (target_pct / stock_pct)), "unit": vpu_unit, "note": format!("{}% → {}%", fmt_calc_num(stock_pct), fmt_calc_num(target_pct))}));
                        } else {
                            let mass_mg = (total_vol * vpu_fac * 1000.0) * (target_pct * 10.0);
                            rows.push(json!({"label": cname, "amount": round4(mass_mg), "unit": "mg", "note": format!("终浓度 {}% w/v", fmt_calc_num(target_pct))}));
                        }
                    } else if let Some((qv, qu, _, _)) = yaml_quantity_parts(comp.get("volume_per_unit")).filter(|p| p.0 > 0.0) {
                        rows.push(json!({"label": cname, "amount": round4(qv * (total_units as f64) * (1.0 + excess)), "unit": qu, "note": format!("单份 {} {qu}", fmt_calc_num(qv))}));
                    } else {
                        let note = ytext(comp, "note");
                        rows.push(json!({"label": cname, "amount": round4(total_vol), "unit": vpu_unit, "note": if note.is_empty() { "按分组浓度配置".to_owned() } else { note }}));
                    }
                }
            }
            summary = format!(
                "共 {total_units} 份 · 含 {}% 余量 · 总配液 {} {vpu_unit}",
                (excess * 100.0).round() as i64,
                fmt_calc_num(total_vol)
            );
        }
        "serial_dilution" => {
            let df = yaml_num_f64(calc.get("dilution_factor")).filter(|v| *v > 1.0)
                .ok_or_else(|| CliError::input("serial_dilution needs dilution_factor > 1"))?;
            let steps = yaml_num_f64(calc.get("steps_count")).filter(|v| *v >= 1.0)
                .ok_or_else(|| CliError::input("serial_dilution needs steps_count >= 1"))? as i64;
            let (t_val, t_unit) = yaml_quantity_parts(calc.get("volume_per_unit"))
                .filter(|p| p.0 > 0.0)
                .map(|p| (p.0, p.1))
                .unwrap_or((100.0, "uL".to_owned()));
            let d_val = t_val * (df - 1.0);
            let mut factor = 1.0;
            for step in 1..=steps {
                factor *= df;
                rows.push(json!({
                    "label": format!("第 {step} 级 (1:{})", fmt_calc_num(factor)),
                    "amount": round4(t_val),
                    "unit": t_unit,
                    "note": format!("吸取 {} {t_unit} + 稀释液 {} {t_unit}", fmt_calc_num(t_val), fmt_calc_num(d_val))
                }));
            }
            summary = format!("{steps} 级 {} 倍梯度稀释（总稀释至 1:{}）", fmt_calc_num(df), fmt_calc_num(factor));
        }
        "sample_normalize" => {
            let rxn = yaml_quantity_parts(calc.get("volume_per_unit"))
                .filter(|p| p.0 > 0.0)
                .or_else(|| yaml_quantity_parts(calc.get("final_volume")).filter(|p| p.0 > 0.0));
            let target_mass_ng = if let Some((mv, _, mdim, mfac)) = yaml_quantity_parts(calc.get("target_mass")).filter(|p| p.0 > 0.0) {
                if mdim != "mass" {
                    return Err(CliError::input("sample_normalize target_mass must have mass unit"));
                }
                mv * mfac * 1e9
            } else if let Some(ng) = yaml_num_f64(calc.get("target_mass_ng")).filter(|v| *v > 0.0) {
                ng
            } else {
                500.0
            };

            let mut samples: Vec<CalcSampleInput> = Vec::new();
            let mut from_actual = false;
            let wanted_endpoint = ytext(calc, "upstream_endpoint");
            if let Some(ctx) = actuals {
                for s in &ctx.samples {
                    if !wanted_endpoint.is_empty() && !s.endpoint.is_empty() && !s.endpoint.eq_ignore_ascii_case(&wanted_endpoint) {
                        continue;
                    }
                    if s.value > 0.0 {
                        samples.push(s.clone());
                    }
                }
                if !samples.is_empty() {
                    from_actual = true;
                }
            }
            if samples.is_empty() {
                if let Some(seq) = calc.get("samples").and_then(|v| v.as_sequence()) {
                    for rm in seq {
                        let sid = {
                            let s = ytext(rm, "sample_id");
                            if s.is_empty() { ytext(rm, "name") } else { s }
                        };
                        if sid.is_empty() {
                            continue;
                        }
                        let mut val_opt = yaml_num_f64(rm.get("concentration"));
                        let mut unit = ytext(rm, "unit");
                        if val_opt.is_none() {
                            if let Some((qv, qu, _, _)) = yaml_quantity_parts(rm.get("concentration")) {
                                val_opt = Some(qv);
                                unit = qu;
                            }
                        }
                        if let Some(v) = val_opt.filter(|v| *v > 0.0) {
                            samples.push(CalcSampleInput {
                                sample_id: sid,
                                value: v,
                                unit: if unit.is_empty() { "ng/uL".to_owned() } else { unit },
                                endpoint: String::new(),
                            });
                        }
                    }
                }
            }
            if !samples.is_empty() {
                if let Some((rv, _, rdim, rfac)) = rxn {
                    if rdim != "volume" {
                        return Err(CliError::input("sample_normalize volume unit is invalid"));
                    }
                    let rxn_ul = rv * rfac * 1e6;
                    let norm_unit = "uL";
                    let mut insufficient_count = 0i64;
                    for s in &samples {
                        let eff_unit = if s.unit.trim().is_empty() { "ng/uL" } else { s.unit.as_str() };
                        let conc_ng_per_ul = asterdoc_unit(eff_unit)
                            .filter(|(dim, _)| dim == "mass_concentration")
                            .map(|(_, fac)| s.value * fac * 1000.0);
                        match conc_ng_per_ul {
                            Some(conc) if conc > 0.0 => {
                                let req_ul = target_mass_ng / conc;
                                let (sample_ul, note) = if req_ul > rxn_ul {
                                    insufficient_count += 1;
                                    (
                                        rxn_ul,
                                        format!("实测 {} {eff_unit} · 需 {} {norm_unit}（浓度偏低 · 超反应体积上限 insufficient）", fmt_calc_num(s.value), fmt_calc_num(req_ul)),
                                    )
                                } else {
                                    let diluent_ul = rxn_ul - req_ul;
                                    (
                                        req_ul,
                                        format!("实测 {} {eff_unit} · 补稀释液 {} {norm_unit}", fmt_calc_num(s.value), fmt_calc_num(diluent_ul)),
                                    )
                                };
                                rows.push(json!({"label": format!("{} 加样量", s.sample_id), "amount": round4(sample_ul), "unit": norm_unit, "note": note}));
                                if from_actual {
                                    used_actuals.push(format!("{}={} {eff_unit}", s.sample_id, fmt_calc_num(s.value)));
                                }
                            }
                            _ => {
                                rows.push(json!({"label": s.sample_id, "amount": 0.0, "unit": norm_unit, "note": format!("实测 {} {eff_unit} 单位暂不支持换算", fmt_calc_num(s.value))}));
                            }
                        }
                    }
                    summary = format!(
                        "{} 个样本归一化至 {} ng / {} {norm_unit}（偏低 {insufficient_count} 项）",
                        samples.len(),
                        fmt_calc_num(target_mass_ng),
                        fmt_calc_num(rxn_ul)
                    );
                } else {
                    summary = format!("归一化 {} 个样本至目标浓度", samples.len());
                }
            } else {
                let count = yaml_num_f64(calc.get("sample_count")).filter(|v| *v >= 1.0).unwrap_or(1.0) as i64;
                let excess = yaml_num_f64(calc.get("excess_ratio")).unwrap_or(0.0).max(0.0);
                if let Some((rv, ru, _, _)) = rxn {
                    let tot = (count as f64) * rv * (1.0 + excess);
                    summary = format!("{count} 个样本 · 单样 {} {ru} · 总配液 {} {ru}", fmt_calc_num(rv), fmt_calc_num(tot));
                    rows.push(json!({"label": "单样反应体积", "amount": round4(rv), "unit": ru, "note": format!("样本数 {count}")}));
                    rows.push(json!({"label": "含余量总体系", "amount": round4(tot), "unit": ru, "note": format!("含 {}% 余量", (excess * 100.0).round() as i64)}));
                } else {
                    summary = format!("归一化 {count} 个样本至目标浓度");
                }
            }
        }
        "formula" => {
            let expr = ytext(calc, "formula");
            validate_formula_expr(&expr).map_err(CliError::input)?;
            let mut vars: BTreeMap<String, f64> = BTreeMap::new();
            if let Some(inputs) = calc.get("inputs").and_then(|v| v.as_mapping()) {
                for (k, v) in inputs {
                    let Some(kname) = k.as_str().map(str::to_owned) else { continue };
                    if let Some(raw_ref) = v.as_str().map(str::trim) {
                        if raw_ref.starts_with("@actual.") || raw_ref.starts_with("@param.") {
                            let ref_key = raw_ref
                                .strip_prefix("@actual.")
                                .or_else(|| raw_ref.strip_prefix("@param."))
                                .unwrap_or(raw_ref);
                            if let Some(ctx) = actuals {
                                if let Some(&av) = ctx.params.get(ref_key) {
                                    vars.insert(kname.clone(), av);
                                    used_actuals.push(format!("{kname}={}", fmt_calc_num(av)));
                                    continue;
                                }
                            }
                            if let Some((pv, _)) = doc_planned_param_float(&flat, ref_key) {
                                vars.insert(kname, pv);
                            }
                        }
                    } else if let Some(num) = yaml_num_f64(Some(v)) {
                        vars.insert(kname, num);
                    } else if let Some((qv, _, _, _)) = yaml_quantity_parts(Some(v)) {
                        vars.insert(kname, qv);
                    }
                }
            }
            if let Some(ctx) = actuals {
                let bytes = expr.as_bytes();
                let mut i = 0usize;
                while i < bytes.len() {
                    if bytes[i].is_ascii_alphabetic() || bytes[i] == b'_' {
                        let start = i;
                        while i < bytes.len() && (bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_') {
                            i += 1;
                        }
                        let ident = &expr[start..i];
                        if !vars.contains_key(ident) {
                            if let Some(&av) = ctx.params.get(ident) {
                                vars.insert(ident.to_owned(), av);
                                used_actuals.push(format!("{ident}={}", fmt_calc_num(av)));
                            }
                        }
                    } else {
                        i += 1;
                    }
                }
            }
            used_actuals.sort();
            if vars.is_empty() {
                summary = format!("公式：{expr}（Run 时代入实测值）");
            } else {
                let val = eval_formula_with_vars(&expr, &vars)?;
                summary = format!("{expr} = {}", fmt_calc_num(val));
                rows.push(json!({"label": "计算结果", "amount": round4(val), "unit": "", "note": expr}));
            }
        }
        other => return Err(CliError::input(format!("unsupported calculator mode {other}"))),
    }

    Ok(json!({
        "module_id": calc_id,
        "mode": mode,
        "summary": summary,
        "rows": rows,
        "used_actuals": used_actuals
    }))
}

/// asterdoc_cross_module_warnings checks quantitative consistency across modules
/// (such as calculator.sample_count vs plate_ref wells and group independent_n vs plate wells).
pub(crate) fn asterdoc_cross_module_warnings(source: &str) -> Vec<String> {
    use std::collections::{BTreeMap, HashMap, HashSet};
    let Ok(blocks) = asterdoc_blocks(source) else {
        return Vec::new();
    };
    let values: Vec<serde_yaml::Value> = blocks.into_iter().map(|b| b.value).collect();
    let flat = flatten_asterdoc_modules(&values);

    struct PlateStats {
        total_wells: usize,
        has_factors: bool,
        by_group: BTreeMap<String, usize>,
    }
    let mut plates: HashMap<String, PlateStats> = HashMap::new();
    for m in &flat {
        if yaml_kind(m) != "plate" {
            continue;
        }
        let pid = yaml_id(m);
        let has_factors = m.get("factors")
            .and_then(|v| v.as_mapping())
            .is_some_and(|fm| !fm.is_empty());
        let mut by_group: BTreeMap<String, usize> = BTreeMap::new();
        let mut all_wells: HashSet<String> = HashSet::new();
        if let Some(groups) = m.get("groups").and_then(|v| v.as_mapping()) {
            for (k, v) in groups {
                let Some(gkey) = k.as_str().map(str::trim).filter(|s| !s.is_empty()) else {
                    continue;
                };
                let mut gset: HashSet<String> = HashSet::new();
                if let Some(seq) = v.as_sequence() {
                    for w in seq {
                        if let Some(ws) = w.as_str().map(|s| s.trim().to_uppercase()).filter(|s| !s.is_empty()) {
                            gset.insert(ws.clone());
                            all_wells.insert(ws);
                        }
                    }
                }
                if !gset.is_empty() {
                    *by_group.entry(gkey.to_owned()).or_insert(0) += gset.len();
                }
            }
        }
        plates.insert(pid, PlateStats { total_wells: all_wells.len(), has_factors, by_group });
    }

    let mut warnings = Vec::new();
    for m in &flat {
        if yaml_kind(m) != "calculator" {
            continue;
        }
        let cid = yaml_id(m);
        let pref = {
            let p = ytext(m, "plate_ref");
            if p.is_empty() { ytext(m, "container_ref") } else { p }
        };
        let sc = m.get("sample_count").and_then(|v| v.as_i64()).unwrap_or(0);
        if !pref.is_empty() && sc > 0 {
            if let Some(ps) = plates.get(&pref) {
                if ps.total_wells > 0 && sc as usize != ps.total_wells {
                    warnings.push(format!(
                        "calculator {cid} sample_count ({sc}) does not match referenced plate {pref} assigned wells ({})",
                        ps.total_wells
                    ));
                }
            }
        }
    }

    let mut check_group = |row: &serde_yaml::Value| {
        let gid = ytext(row, "id");
        let gname = ytext(row, "name");
        let ind_n = row.get("independent_n").and_then(|v| v.as_u64()).unwrap_or(0) as usize;
        if ind_n == 0 {
            return;
        }
        let tech_rep = row.get("technical_repeats").and_then(|v| v.as_u64()).map(|n| n as usize);
        let mut matched_wells: usize = 0;
        let mut has_factors = false;
        for ps in plates.values() {
            if !gid.is_empty() {
                if let Some(&cnt) = ps.by_group.get(&gid) {
                    matched_wells += cnt;
                    has_factors |= ps.has_factors;
                    continue;
                }
            }
            if !gname.is_empty() {
                if let Some(&cnt) = ps.by_group.get(&gname) {
                    matched_wells += cnt;
                    has_factors |= ps.has_factors;
                }
            }
        }
        if matched_wells == 0 {
            return;
        }
        let label = if gname.is_empty() { &gid } else { &gname };
        if matched_wells < ind_n {
            warnings.push(format!(
                "group {gid} ({label}) declares independent_n={ind_n}, but plate assigns only {matched_wells} well(s)"
            ));
        } else if let Some(rep) = tech_rep.filter(|&r| r >= 1) {
            let expected = ind_n * rep;
            let ok = if has_factors { matched_wells >= expected && matched_wells % expected == 0 } else { matched_wells == expected };
            if !ok {
                warnings.push(format!(
                    "group {gid} ({label}) declares independent_n={ind_n} x technical_repeats={rep} ({expected} wells), but plate assigns {matched_wells} well(s)"
                ));
            }
        } else if matched_wells % ind_n != 0 {
            warnings.push(format!(
                "group {gid} ({label}) declares independent_n={ind_n}, which does not evenly divide plate assigned {matched_wells} well(s)"
            ));
        }
    };

    for m in &flat {
        match yaml_kind(m).as_str() {
            "group" => check_group(m),
            "group_table" => {
                if let Some(rows) = m.get("rows").and_then(|v| v.as_sequence()) {
                    for row in rows {
                        check_group(row);
                    }
                }
            }
            _ => {}
        }
    }

    warnings.sort();
    warnings.dedup();
    warnings
}

