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
    tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
    Ok(data)
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

pub(crate) fn asterdoc_blocks(source: &str) -> Result<Vec<AsterBlock>, CliError> {
    let mut blocks = Vec::new();
    let mut offset = 0usize;
    let mut in_fence = false;
    let mut body_start = 0usize;
    let mut body = String::new();
    let mut fence_start = 0usize;
    for line in source.split_inclusive('\n') {
        let trimmed = line.trim();
        if !in_fence {
            if trimmed.strip_prefix("```").map(str::trim) == Some("asterlab") {
                in_fence = true;
                fence_start = offset;
                body_start = offset + line.len();
                body.clear();
            }
        } else if trimmed == "```" {
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
        let mut linked = false;
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
            linked = true;
            break;
        }
        if !linked {
            let workflow: serde_yaml::Value = serde_yaml::from_str(&format!(
                "kind: workflow\nid: workflow-main\nschema: 1\nsteps: [{}]\nedges: []\nbusiness_order: [{}]\n",
                step_id, step_id
            )).map_err(|e| CliError::internal(e.to_string()))?;
            next = append_asterdoc_module(&next, &workflow)?;
        }
        next = apply_replacements(next, replacements);
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
                for field in ["samples", "replicates"] {
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
    let mut in_fence = false;
    let mut body = String::new();
    let mut line_no = 0usize;
    for line in source.lines() {
        line_no += 1;
        let trimmed = line.trim();
        if !in_fence {
            if let Some(lang) = trimmed.strip_prefix("```") {
                if lang.trim() == "asterlab" {
                    in_fence = true;
                    body.clear();
                }
            }
            continue;
        }
        if trimmed == "```" {
            validate_asterdoc_module(&body, line_no)?;
            parse_asterdoc_header(&body, &mut ids, line_no)?;
            let module: serde_yaml::Value = serde_yaml::from_str(&body)
                .map_err(|e| CliError::input(format!("asterlab block near line {line_no}: {e}")))?;
            modules.push(module);
            in_fence = false;
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
    let mut groups = std::collections::HashSet::new();
    let mut assays = std::collections::HashSet::new();
    let mut steps = std::collections::HashSet::new();
    for module in modules {
        match kind(module).as_str() {
            "group" => { groups.insert(id(module)); }
            "assay" => { assays.insert(id(module)); }
            "step" => { steps.insert(id(module)); }
            _ => {}
        }
    }
    let mut ids_by_kind: std::collections::HashMap<String, std::collections::HashSet<String>> =
        std::collections::HashMap::new();
    let mut all_ids = std::collections::HashSet::new();
    for module in modules {
        let module_kind = kind(module);
        let module_id = id(module);
        ids_by_kind.entry(module_kind).or_default().insert(module_id.clone());
        all_ids.insert(module_id);
    }
    for module in modules {
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
        modules.iter().map(|module| (id(module), module)).collect();
    for module in modules {
        if kind(module) != "step" {
            continue;
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
    for module in modules {
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
    "plate-wells",
    "plate-geometry-wells",
    "reference-ref",
    "group-fields",
    "assay-fields",
    "step-fields",
    "step-execution",
    "step-uses",
    "workflow-fields",
    "dilution-fields",
    "instrument-profile-fields",
    "material-fields",
    "sample-fields",
    "container-fields",
    "measurement-profile-fields",
    "rna-reagent-fields",
    "plate-mix-fields",
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

pub(crate) fn asterdoc_unit(symbol: &str) -> Option<(String, f64)> {
    asterdoc_units()["units"].as_array()?.iter().find_map(|unit| {
        if unit["symbol"].as_str() != Some(symbol) {
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

pub(crate) fn asterdoc_plate_mix_fields(mapping: &serde_yaml::Mapping) -> Result<(), CliError> {
    let field = |key: &str| mapping.get(serde_yaml::Value::String(key.to_owned()));
    let present = |key: &str| field(key).is_some_and(|value| !value.is_null());
    let ratio = |key: &str| -> Result<Option<f64>, CliError> {
        let Some(value) = field(key) else {
            return Ok(None);
        };
        if value.is_null() {
            return Ok(None);
        }
        let number = value
            .as_f64()
            .ok_or_else(|| CliError::input(format!("{key} must be a number")))?;
        if !number.is_finite() || number < 0.0 {
            return Err(CliError::input(format!(
                "{key} must be a non-negative finite number"
            )));
        }
        Ok(Some(number))
    };
    let quantity = |key: &str, dimension: &str| -> Result<f64, CliError> {
        let value = field(key)
            .ok_or_else(|| CliError::input(format!("mix plan needs {key}")))?;
        let (number, unit, actual, factor) = asterdoc_quantity_parts(value)?;
        if actual != dimension {
            return Err(CliError::input(format!(
                "{key} unit {unit} is {actual}, not {dimension}"
            )));
        }
        Ok(number * factor)
    };

    let gel_plan = present("mix_gelma_percent_by_group")
        || present("mix_lap_stock_percent")
        || present("mix_lap_final_percent");
    let cell_plan = present("mix_cell_density")
        || present("mix_factor_stock_concentration")
        || present("mix_factor_final_concentration");
    if !gel_plan && !cell_plan {
        if present("mix_volume_per_well") || present("mix_excess_ratio") {
            return Err(CliError::input(
                "mix_volume_per_well and mix_excess_ratio need a gel or cell plan",
            ));
        }
        return Ok(());
    }
    if !present("mix_volume_per_well") {
        return Err(CliError::input("mix plan needs mix_volume_per_well"));
    }
    let volume_per_well_ul = quantity("mix_volume_per_well", "volume")? / 1e-6;
    if volume_per_well_ul <= 0.0 {
        return Err(CliError::input("mix_volume_per_well must be positive"));
    }
    let excess_ratio = ratio("mix_excess_ratio")?.unwrap_or(0.0);

    let groups = field("groups")
        .and_then(|value| value.as_mapping())
        .ok_or_else(|| CliError::input("plate needs groups"))?;
    let mut wells_by_group = std::collections::BTreeMap::new();
    for (group, wells) in groups {
        let name = group.as_str().unwrap_or("").trim().to_owned();
        if name.is_empty() {
            return Err(CliError::input("plate group name cannot be empty"));
        }
        let count = wells.as_sequence().map(|items| items.len()).unwrap_or(0);
        if count == 0 {
            return Err(CliError::input(format!(
                "plate group {name} has no wells"
            )));
        }
        wells_by_group.insert(name, count as u32);
    }
    if wells_by_group.is_empty() {
        return Err(CliError::input("mix plan needs at least one plate group"));
    }

    if gel_plan {
        let percents = field("mix_gelma_percent_by_group")
            .and_then(|value| value.as_mapping())
            .ok_or_else(|| CliError::input("mix_gelma_percent_by_group must be a mapping"))?;
        let stock_percent = ratio("mix_lap_stock_percent")?
            .ok_or_else(|| CliError::input("gel plan needs mix_lap_stock_percent"))?;
        let final_percent = ratio("mix_lap_final_percent")?
            .ok_or_else(|| CliError::input("gel plan needs mix_lap_final_percent"))?;
        if stock_percent <= 0.0 {
            return Err(CliError::input("mix_lap_stock_percent must be positive"));
        }
        if final_percent > stock_percent {
            return Err(CliError::input(
                "mix_lap_final_percent cannot exceed mix_lap_stock_percent",
            ));
        }
        let mut gel_groups = Vec::with_capacity(wells_by_group.len());
        for (name, wells) in &wells_by_group {
            let raw = percents
                .get(serde_yaml::Value::String(name.clone()))
                .ok_or_else(|| {
                    CliError::input(format!(
                        "mix_gelma_percent_by_group is missing group {name}"
                    ))
                })?;
            let percent = raw.as_f64().ok_or_else(|| {
                CliError::input(format!("group {name} GelMA percent must be a number"))
            })?;
            if !percent.is_finite() || percent <= 0.0 || percent > 100.0 {
                return Err(CliError::input(format!(
                    "group {name} GelMA percent must be in (0, 100]"
                )));
            }
            gel_groups.push(GelMastermixGroup {
                name: name.clone(),
                gelma_percent_wv: percent,
                wells: *wells,
            });
        }
        for key in percents.keys() {
            let name = key.as_str().unwrap_or("");
            if !wells_by_group.contains_key(name) {
                return Err(CliError::input(format!(
                    "mix_gelma_percent_by_group names unknown group {name}"
                )));
            }
        }
        gel_mastermix_calc(&GelMastermixInput {
            volume_per_well_ul,
            excess_ratio,
            lap_stock_percent_wv: stock_percent,
            lap_final_percent_wv: final_percent,
            groups: gel_groups,
        })
        .map_err(CliError::input)?;
    }

    if cell_plan {
        if !present("mix_cell_density") {
            return Err(CliError::input("cell plan needs mix_cell_density"));
        }
        let density = quantity("mix_cell_density", "cells_per_well")?;
        if density <= 0.0 {
            return Err(CliError::input("mix_cell_density must be positive"));
        }
        if present("mix_factor_stock_concentration")
            != present("mix_factor_final_concentration")
        {
            return Err(CliError::input(
                "factor stock and final concentration must be provided together",
            ));
        }
        let stock = if present("mix_factor_stock_concentration") {
            quantity("mix_factor_stock_concentration", "mass_concentration")?
        } else {
            0.0
        };
        let final_concentration = if present("mix_factor_final_concentration") {
            quantity("mix_factor_final_concentration", "mass_concentration")?
        } else {
            0.0
        };
        if stock <= 0.0 && final_concentration > 0.0 {
            return Err(CliError::input("factor stock concentration must be positive"));
        }
        if stock > 0.0 && final_concentration > stock {
            return Err(CliError::input(
                "factor final concentration cannot exceed stock concentration",
            ));
        }
        let groups = wells_by_group
            .iter()
            .map(|(name, wells)| CellSeedingGroup {
                name: name.clone(),
                wells: *wells,
            })
            .collect();
        cell_seeding_calc(&CellSeedingInput {
            density_cells_per_well: density,
            volume_per_well_ul,
            excess_ratio,
            factor_stock_mg_per_ml: stock,
            factor_final_mg_per_ml: final_concentration,
            groups,
        })
        .map_err(CliError::input)?;
    }
    Ok(())
}

pub(crate) fn validate_asterdoc_module(body: &str, line_no: usize) -> Result<(), CliError> {
    let value: serde_yaml::Value = serde_yaml::from_str(body)
        .map_err(|e| CliError::input(format!("asterlab block near line {line_no}: {e}")))?;
    let mapping = value
        .as_mapping()
        .ok_or_else(|| CliError::input("asterlab block must be a YAML mapping"))?;
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
            "plate-wells" | "plate-geometry-wells" => {
                let format = if rule.as_str() == Some("plate-wells") {
                    96
                } else {
                    get("format").and_then(|value| value.as_i64()).unwrap_or(0)
                };
                let Some((rows, cols)) = plate_geometry(format) else {
                    return Err(CliError::input(format!(
                        "plate format {format} is not registered"
                    )));
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
                        let valid = well.len() >= 2
                            && well.as_bytes()[0].is_ascii_uppercase()
                            && (well.as_bytes()[0] - b'A') < rows as u8
                            && well[1..]
                                .parse::<u32>()
                                .map(|col| (1..=cols as u32).contains(&col))
                                .unwrap_or(false);
                        if !valid {
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
                        let valid = well.len() >= 2
                            && well.as_bytes()[0].is_ascii_uppercase()
                            && (well.as_bytes()[0] - b'A') < rows as u8
                            && well[1..]
                                .parse::<u32>()
                                .map(|col| (1..=cols as u32).contains(&col))
                                .unwrap_or(false);
                        if !valid {
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
                let mut seen_replicates: std::collections::HashMap<String, String> =
                    std::collections::HashMap::new();
                if let Some(replicates) = get("replicates").and_then(|value| value.as_mapping()) {
                    for (well_key, replicate_value) in replicates {
                        let well = well_key.as_str().unwrap_or("").trim().to_uppercase();
                        let valid = well.len() >= 2
                            && well.as_bytes()[0].is_ascii_uppercase()
                            && (well.as_bytes()[0] - b'A') < rows as u8
                            && well[1..]
                                .parse::<u32>()
                                .map(|col| (1..=cols as u32).contains(&col))
                                .unwrap_or(false);
                        if !valid {
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
                        let key = format!("{sample_id}\u{0}{replicate}");
                        if let Some(previous) = seen_replicates.get(&key) {
                            return Err(CliError::input(format!(
                                "sample {sample_id} replicate {replicate} is assigned to both {previous} and {well}"
                            )));
                        }
                        seen_replicates.insert(key, well);
                    }
                }
            }
            "plate-mix-fields" => asterdoc_plate_mix_fields(mapping)?,
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
            "step-fields" => {
                if text("assay").is_empty() {
                    return Err(CliError::input("step needs an assay"));
                }
            }
            "step-execution" => match text("execution") {
                "executable" if text("assay").is_empty() => {
                    return Err(CliError::input("executable step needs an assay"));
                }
                "executable" => {}
                "organizational" => {
                    for field in ["assay", "timepoints", "path", "checks", "tasks", "uses"] {
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
            },
            "step-uses" => {
                let mut seen = std::collections::HashSet::new();
                for use_item in get("uses").and_then(|value| value.as_sequence()).into_iter().flatten() {
                    let ref_id = asterdoc_map_get(use_item, "ref").and_then(|value| value.as_str()).unwrap_or("").trim();
                    let role = asterdoc_map_get(use_item, "role").and_then(|value| value.as_str()).unwrap_or("").trim();
                    if ref_id.is_empty() || role.is_empty() {
                        return Err(CliError::input("step use needs ref and role"));
                    }
                    if !seen.insert(ref_id.to_owned()) {
                        return Err(CliError::input(format!("step use repeats {ref_id}")));
                    }
                }
            }
            "workflow-fields" => {
                if get("steps").and_then(|value| value.as_sequence()).is_none_or(|items| items.is_empty()) {
                    return Err(CliError::input("workflow needs at least one step"));
                }
            }
            "dilution-fields" => {
                let stock = asterdoc_quantity_parts(get("stock").ok_or_else(|| CliError::input("dilution needs stock"))?)?;
                let target = asterdoc_quantity_parts(get("target").ok_or_else(|| CliError::input("dilution needs target"))?)?;
                let final_volume = asterdoc_quantity_parts(get("final_volume").ok_or_else(|| CliError::input("dilution needs final_volume"))?)?;
                if stock.2 != "mass_concentration" || target.2 != "mass_concentration" || final_volume.2 != "volume" {
                    return Err(CliError::input("dilution needs concentration/concentration/volume"));
                }
                if stock.0 <= 0.0 || target.0 <= 0.0 || final_volume.0 <= 0.0 {
                    return Err(CliError::input("dilution quantities must be positive"));
                }
                let stock_base = stock.0 * stock.3;
                let target_base = target.0 * target.3;
                if target_base > stock_base {
                    return Err(CliError::input("target concentration cannot exceed stock concentration"));
                }
                let stock_volume = target_base * final_volume.0 * final_volume.3 / stock_base;
                if stock_volume > final_volume.0 * final_volume.3 {
                    return Err(CliError::input("stock volume exceeds final volume"));
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
            "container-fields" => {
                if text("format").is_empty() {
                    return Err(CliError::input("container needs a format"));
                }
                if get("wells").and_then(|v| v.as_i64()).unwrap_or(0) <= 0 {
                    return Err(CliError::input("container needs positive wells"));
                }
            }
            "measurement-profile-fields" => {
                if text("endpoint").is_empty() {
                    return Err(CliError::input("measurement profile needs an endpoint"));
                }
                if text("unit").is_empty() {
                    return Err(CliError::input("measurement profile needs a unit"));
                }
            }
            "rna-reagent-fields" => {
                let sample_count = get("sample_count")
                    .and_then(|v| v.as_i64())
                    .ok_or_else(|| CliError::input("rna reagent needs sample_count"))?;
                if sample_count <= 0 {
                    return Err(CliError::input("rna reagent sample_count must be positive"));
                }
                let (vol_value, vol_unit, vol_dim, _vol_factor) = asterdoc_quantity_parts(get("volume_per_sample").ok_or_else(|| CliError::input("rna reagent needs volume_per_sample"))?)?;
                let (stock_value, stock_unit, stock_dim, stock_factor) = asterdoc_quantity_parts(get("stock_concentration").ok_or_else(|| CliError::input("rna reagent needs stock_concentration"))?)?;
                let (target_value, target_unit, target_dim, target_factor) = asterdoc_quantity_parts(get("target_concentration").ok_or_else(|| CliError::input("rna reagent needs target_concentration"))?)?;
                if vol_dim != "volume" || stock_dim != "mass_concentration" || target_dim != "mass_concentration" {
                    return Err(CliError::input("rna reagent needs volume/concentration/concentration"));
                }
                let _ = (vol_unit, stock_unit, target_unit);
                if vol_value <= 0.0 || stock_value <= 0.0 || target_value <= 0.0 {
                    return Err(CliError::input("rna reagent quantities must be positive"));
                }
                let extra_ratio = match get("extra_ratio") {
                    Some(v) => v
                        .as_f64()
                        .ok_or_else(|| CliError::input("rna reagent extra_ratio must be a number"))?,
                    None => 0.0,
                };
                if extra_ratio < 0.0 {
                    return Err(CliError::input("rna reagent extra_ratio must be non-negative"));
                }
                let stock_base = stock_value * stock_factor;
                let target_base = target_value * target_factor;
                if target_base > stock_base {
                    return Err(CliError::input("target concentration cannot exceed stock concentration"));
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
    let mut in_fence = false;
    let mut body = String::new();
    for line in source.lines() {
        let trimmed = line.trim();
        if !in_fence {
            if trimmed.strip_prefix("```").map(str::trim) == Some("asterlab") {
                in_fence = true;
                body.clear();
            }
            continue;
        }
        if trimmed == "```" {
            let module: serde_yaml::Value = serde_yaml::from_str(&body)
                .map_err(|e| CliError::input(format!("asterlab block: {e}")))?;
            modules.push(module);
            in_fence = false;
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
    let mut groups = Vec::new();
    let mut assays = Vec::new();
    let mut steps = Vec::new();
    let mut workflows = Vec::new();
    let mut plates = Vec::new();
    for module in modules {
        match ytext(module, "kind").as_str() {
            "experiment" => experiments.push(module),
            "group" => groups.push(module),
            "assay" => assays.push(module),
            "step" => steps.push(module),
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
    let group_index: std::collections::HashMap<String, &serde_yaml::Value> = groups
        .iter()
        .map(|group| (ytext(group, "id"), *group))
        .collect();
    let assay_index: std::collections::HashMap<String, &serde_yaml::Value> = assays
        .iter()
        .map(|assay| (ytext(assay, "id"), *assay))
        .collect();
    // Reference modules name the ProtocolBlock version a design step reuses.
    // The compiled design carries that identity so a frozen task, which stores
    // the producing block version, can be joined to its design step even
    // though the design step path and the block's own step paths differ.
    let reference_index: std::collections::HashMap<String, String> = modules
        .iter()
        .filter(|module| ytext(module, "kind") == "reference" && ytext(module, "view") == "protocol_block")
        .map(|module| (ytext(module, "id"), ytext(module, "ref")))
        .filter(|(id, target)| !id.is_empty() && !target.is_empty())
        .collect();

    let group_specs: Vec<Value> = groups
        .iter()
        .map(|group| {
            let mut spec = json!({
                "id": ytext(group, "id"),
                "name": ytext(group, "name"),
            });
            let role = ytext(group, "role");
            if !role.is_empty() {
                spec["role"] = json!(match role.as_str() {
                    "control" | "treatment" | "comparator" | "reference" => role,
                    _ => "other".to_owned(),
                });
            }
            if !ytext(group, "condition").is_empty() {
                spec["condition"] = json!(ytext(group, "condition"));
            }
            if let Some(n) = group.get("independent_n").and_then(|v| v.as_u64()) {
                spec["independent_n"] = json!(n);
            }
            if let Some(n) = group.get("technical_repeats").and_then(|v| v.as_u64()) {
                spec["technical_repeats"] = json!(n);
            }
            spec
        })
        .collect();

    let assay_plans: Vec<Value> = assays
        .iter()
        .map(|assay| {
            let group_ids: Vec<String> = ystrings(assay, "group_refs");
            let group_names: Vec<String> = group_ids
                .iter()
                .map(|id| {
                    group_index
                        .get(id)
                        .map(|group| ytext(group, "name"))
                        .unwrap_or_default()
                })
                .collect();
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
                "control_group_ids": [],
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

    let mut mygo_assays = serde_json::Map::new();
    for assay in &assays {
        let assay_id = ytext(assay, "id");
        let assay_timepoints = ystrings(assay, "timepoints");
        let control_ref = ytext(assay, "control_group_ref");
        let control_group = group_index
            .get(&control_ref)
            .map(|group| ytext(group, "name"))
            .unwrap_or_default();
        let calibrator_ref = ytext(assay, "calibrator_group_ref");
        let calibrator_group = group_index
            .get(&calibrator_ref)
            .map(|group| ytext(group, "name"))
            .unwrap_or_default();
        let mut wells = Vec::new();
        for plate in &plates {
            if ytext(plate, "assay") != assay_id {
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
                assay_timepoints.clone()
            } else {
                vec![plate_timepoint]
            };
            for (group_value, wells_value) in groups_map {
                let group_id = group_value.as_str().unwrap_or("").to_owned();
                if !group_index.contains_key(&group_id) {
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
                "technicalReplicates": assay.get("technical_replicates").and_then(|v| v.as_u64()).unwrap_or(0),
                "controlGroup": control_group,
                "calibratorGroup": calibrator_group,
                "referenceGenes": ystrings(assay, "reference_genes"),
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
    for (index, step) in steps.iter().enumerate() {
        let id = ytext(step, "id");
        let assay = ytext(step, "assay");
        let title = ytext(step, "title");
        let schema = step.get("schema").and_then(|value| value.as_i64()).unwrap_or(0);
        let execution = if schema >= 3 { ytext(step, "execution") } else { "executable".to_owned() };
        let is_executable = execution == "executable";
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
        let timepoints = ystrings(step, "timepoints");
        let path = ytext(step, "path");
        let checks: Vec<Value> = step.get("checks")
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
    if !sdk_modules.is_empty() {
        design["asterlab"] = json!({
            "foundationVersion": 1,
            "sdkVersion": asterdoc_spec()["sdkVersion"].as_str().unwrap_or("1.0.0"),
            "modules": sdk_modules
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

