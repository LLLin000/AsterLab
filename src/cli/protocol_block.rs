use super::*;

#[derive(Default)]
pub(crate) struct ComposeAccumulator {
    pub(crate) steps: Vec<OperationStep>,
    pub(crate) resolved_parameters: Vec<ResolvedParameter>,
    pub(crate) unresolved_parameters: Vec<UnresolvedParameter>,
    pub(crate) block_version_ids: Vec<String>,
    pub(crate) module_contracts: Vec<Value>,
    pub(crate) unresolved_keys: HashSet<String>,
}
pub(crate) fn block_version_data(
    db: &Connection,
    version_id: &str,
) -> Result<(Value, ProtocolBlockVersionData), CliError> {
    let record = get(db, version_id)?;
    if record["kind"] != "protocol_block_version" {
        return Err(CliError::input(
            "block_version_id 必须指向 protocol_block_version",
        ));
    }
    let data = serde_json::from_value::<ProtocolBlockVersionData>(record["data"].clone())
        .map_err(|e| CliError::internal(format!("步骤块版本数据无效：{e}")))?;
    Ok((record, data))
}
pub(crate) fn validate_parameter_value(
    slot: &ParameterSlot,
    value: &Value,
    unit: Option<&str>,
) -> Result<(), CliError> {
    let valid = match slot.value_type {
        ParameterValueType::Text => value.as_str().is_some(),
        ParameterValueType::Number => value.as_f64().is_some_and(f64::is_finite),
        ParameterValueType::Integer => value.as_i64().is_some() || value.as_u64().is_some(),
        ParameterValueType::Boolean => value.as_bool().is_some(),
    };
    if !valid {
        return Err(CliError::input(format!(
            "参数 {} 的值类型应为 {}",
            slot.key,
            match slot.value_type {
                ParameterValueType::Text => "text",
                ParameterValueType::Number => "number",
                ParameterValueType::Integer => "integer",
                ParameterValueType::Boolean => "boolean",
            }
        )));
    }
    if slot.unit.as_deref() != unit {
        return Err(CliError::input(format!(
            "参数 {} 的单位必须与槽位声明完全一致（{}）",
            slot.key,
            slot.unit.as_deref().unwrap_or("无单位")
        )));
    }
    Ok(())
}
pub(crate) fn validate_parameter_slots(
    slots: &[ParameterSlot],
) -> Result<HashMap<String, ParameterSlot>, CliError> {
    if slots.len() > 100 {
        return Err(CliError::input("单个步骤块最多定义 100 个参数槽位"));
    }
    let mut by_key = HashMap::new();
    for slot in slots {
        let mut chars = slot.key.chars();
        let first_ok = chars.next().is_some_and(|c| c.is_ascii_alphabetic());
        if !first_ok || !chars.all(|c| c.is_ascii_alphanumeric() || c == '_') || slot.key.len() > 80
        {
            return Err(CliError::input(format!(
                "参数 key 无效：{}（使用字母开头的字母、数字和下划线）",
                slot.key
            )));
        }
        check_text(&slot.label, "参数显示名", 120)?;
        if let Some(unit) = &slot.unit {
            check_text(unit, "参数单位", 40)?;
            if !matches!(
                slot.value_type,
                ParameterValueType::Number | ParameterValueType::Integer
            ) {
                return Err(CliError::input(format!(
                    "非数值参数 {} 不能声明单位",
                    slot.key
                )));
            }
        }
        if by_key.insert(slot.key.clone(), slot.clone()).is_some() {
            return Err(CliError::input(format!("参数 key 重复：{}", slot.key)));
        }
    }
    Ok(by_key)
}
pub(crate) fn validate_block_content(
    db: &Connection,
    project_id: &str,
    slots: &[ParameterSlot],
    steps: &[ProtocolStep],
) -> Result<(), CliError> {
    if steps.is_empty() || steps.len() > 100 {
        return Err(CliError::input("步骤块必须包含 1–100 个步骤"));
    }
    let parent_slots = validate_parameter_slots(slots)?;
    let mut use_ids = HashSet::new();
    for (index, step) in steps.iter().enumerate() {
        let use_id = match step {
            ProtocolStep::Text { use_id, .. } | ProtocolStep::BlockUse { use_id, .. } => use_id,
        };
        check_text(use_id, "步骤/使用 ID", 80)?;
        if !use_id
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || c == '_' || c == '-')
        {
            return Err(CliError::input(format!(
                "步骤/使用 ID 只能包含字母、数字、下划线或连字符：{use_id}"
            )));
        }
        if !use_ids.insert(use_id.as_str()) {
            return Err(CliError::input(format!(
                "同一块内的步骤/使用 ID 重复：{use_id}"
            )));
        }
        match step {
            ProtocolStep::Text {
                title,
                instruction,
                parameter_refs,
                operation_spec,
                checks,
                ..
            } => {
                if let Some(spec) = operation_spec {
                    let spec_path = format!("steps[{index}].operation_spec");
                    if !spec.is_supported_schema_version() {
                        return Err(CliError::input(format!(
                            "{spec_path}.schema_version: unsupported OperationSpec schema_version {}",
                            spec.schema_version
                        )));
                    }
                    if let Err(issues) = spec.validate() {
                        let details = issues
                            .into_iter()
                            .map(|issue| {
                                format!(
                                    "{spec_path}.{}: {} ({})",
                                    issue.path,
                                    issue.message,
                                    issue.code.as_str()
                                )
                            })
                            .collect::<Vec<_>>()
                            .join("; ");
                        return Err(CliError::input(details));
                    }
                }
                check_text(title, "步骤标题", 200)?;
                check_text(instruction, "步骤说明", 4000)?;
                if parameter_refs.len() > 100 {
                    return Err(CliError::input("单个步骤最多引用 100 个参数"));
                }
                let mut refs = HashSet::new();
                for key in parameter_refs {
                    if !parent_slots.contains_key(key) {
                        return Err(CliError::input(format!(
                            "步骤 {use_id} 引用了未定义的参数槽位：{key}"
                        )));
                    }
                    if !refs.insert(key.as_str()) {
                        return Err(CliError::input(format!(
                            "步骤 {use_id} 重复引用参数：{key}"
                        )));
                    }
                }
                if checks.len() > 50 {
                    return Err(CliError::input("单个步骤最多定义 50 个检查项"));
                }
                let mut check_ids = HashSet::new();
                for check in checks {
                    check_text(&check.check_id, "检查项 ID", 80)?;
                    check_text(&check.label, "检查项名称", 200)?;
                    if let Some(instruction) = &check.instruction {
                        check_text(instruction, "检查项说明", 2000)?;
                    }
                    if !check_ids.insert(check.check_id.as_str()) {
                        return Err(CliError::input(format!(
                            "步骤 {use_id} 的检查项 ID 重复：{}",
                            check.check_id
                        )));
                    }
                }
            }
            ProtocolStep::BlockUse {
                block_version_id,
                bindings,
                ..
            } => {
                let (child_record, child) = block_version_data(db, block_version_id)?;
                if child_record["project_id"].as_str() != Some(project_id) {
                    return Err(CliError::input("被引用的步骤块版本必须属于同一项目"));
                }
                let child_slots = validate_parameter_slots(&child.parameter_slots)?;
                let mut mapped = HashSet::new();
                for binding in bindings {
                    let child_slot = child_slots.get(&binding.slot).ok_or_else(|| {
                        CliError::input(format!("子块不存在参数槽位：{}", binding.slot))
                    })?;
                    if !mapped.insert(binding.slot.as_str()) {
                        return Err(CliError::input(format!(
                            "子块参数 {} 被重复绑定",
                            binding.slot
                        )));
                    }
                    match (&binding.value, &binding.source_slot) {
                        (Some(value), None) => {
                            validate_parameter_value(child_slot, value, binding.unit.as_deref())?
                        }
                        (None, Some(source_key)) => {
                            if binding.unit.is_some() {
                                return Err(CliError::input(
                                    "source_slot 绑定不能另填 unit；单位来自父参数槽位",
                                ));
                            }
                            let parent_slot = parent_slots.get(source_key).ok_or_else(|| {
                                CliError::input(format!("父步骤块不存在参数槽位：{source_key}"))
                            })?;
                            if parent_slot.value_type != child_slot.value_type
                                || parent_slot.unit != child_slot.unit
                            {
                                return Err(CliError::input(format!("父槽位 {source_key} 与子槽位 {} 的类型或单位不一致；不会自动换算", binding.slot)));
                            }
                        }
                        _ => {
                            return Err(CliError::input(format!(
                                "参数 {} 必须且只能设置 value 或 source_slot 之一",
                                binding.slot
                            )))
                        }
                    }
                }
                if let Some(missing) = child
                    .parameter_slots
                    .iter()
                    .find(|s| s.required && !mapped.contains(s.key.as_str()))
                {
                    return Err(CliError::input(format!(
                        "子块必填参数 {} 尚未绑定；请映射到父槽位或明确给值",
                        missing.key
                    )));
                }
            }
        }
    }
    Ok(())
}
pub(crate) fn validate_root_bindings(
    slots: &[ParameterSlot],
    bindings: &[RootParameterValue],
) -> Result<HashMap<String, BoundValue>, CliError> {
    let slots = validate_parameter_slots(slots)?;
    let mut values = HashMap::new();
    for binding in bindings {
        let slot = slots
            .get(&binding.slot)
            .ok_or_else(|| CliError::input(format!("步骤块不存在根参数槽位：{}", binding.slot)))?;
        validate_parameter_value(slot, &binding.value, binding.unit.as_deref())?;
        if values
            .insert(
                binding.slot.clone(),
                BoundValue {
                    value: binding.value.clone(),
                    unit: binding.unit.clone(),
                    provenance: vec![json!({"kind":"preview_input","slot":binding.slot})],
                },
            )
            .is_some()
        {
            return Err(CliError::input(format!("根参数重复绑定：{}", binding.slot)));
        }
    }
    Ok(values)
}
pub(crate) fn add_unresolved(acc: &mut ComposeAccumulator, path: &str, slot: &ParameterSlot) {
    let key = format!("{path}/{}", slot.key);
    if acc.unresolved_keys.insert(key.clone()) {
        acc.unresolved_parameters.push(UnresolvedParameter {
            path: key,
            slot: slot.key.clone(),
            label: slot.label.clone(),
            unit: slot.unit.clone(),
            required: slot.required,
        });
    }
}
pub(crate) fn expand_block(
    db: &Connection,
    version_id: &str,
    values: HashMap<String, BoundValue>,
    path: &str,
    depth: usize,
    active: &mut HashSet<String>,
    acc: &mut ComposeAccumulator,
) -> Result<(), CliError> {
    if depth > 12 {
        return Err(CliError::input("步骤块组合超过 12 层；请减少嵌套层级"));
    }
    if !active.insert(version_id.to_owned()) {
        return Err(CliError::input("步骤块组合出现循环引用"));
    }
    let (_, data) = block_version_data(db, version_id)?;
    if !acc.block_version_ids.iter().any(|id| id == version_id) {
        acc.block_version_ids.push(version_id.to_owned());
    }
    if data.module_contract.is_some()
        && !acc
            .module_contracts
            .iter()
            .any(|entry| entry.get("block_version_id").and_then(Value::as_str) == Some(version_id))
    {
        acc.module_contracts.push(json!({
            "block_version_id":version_id,
            "module_contract":data.module_contract,
            "catalog_review":data.catalog_review
        }));
    }
    let slots = validate_parameter_slots(&data.parameter_slots)?;
    for slot in &data.parameter_slots {
        let slot_path = format!("{path}/{}", slot.key);
        if let Some(resolved) = values.get(&slot.key) {
            acc.resolved_parameters.push(ResolvedParameter {
                path: Some(slot_path),
                slot: slot.key.clone(),
                label: slot.label.clone(),
                value: resolved.value.clone(),
                unit: resolved.unit.clone(),
                provenance: resolved.provenance.clone(),
            });
        } else {
            add_unresolved(acc, path, slot);
        }
    }
    for step in &data.steps {
        match step {
            ProtocolStep::Text { use_id, title, instruction, parameter_refs, visualization, time_scope, checks, operation_spec, .. } => {
                if acc.steps.len() >= 500 {
                    return Err(CliError::input("组合展开超过 500 个操作步骤；请拆分预览"));
                }
                let step_path = format!("{path}/{use_id}");
                let mut parameters = Vec::new();
                let mut unresolved_refs = Vec::new();
                for key in parameter_refs {
                    if let Some(value) = values.get(key) {
                        let slot = &slots[key];
                        parameters.push(ResolvedParameter {
                            path: None,
                            slot: key.clone(),
                            label: slot.label.clone(),
                            value: value.value.clone(),
                            unit: value.unit.clone(),
                            provenance: value.provenance.clone(),
                        });
                    } else {
                        unresolved_refs.push(key.clone());
                        if let Some(slot) = slots.get(key) {
                            add_unresolved(acc, path, slot);
                        }
                    }
                }
                let visualization = visualization
                    .as_ref()
                    .map(|value| {
                        serde_json::to_value(value)
                            .map(|value| value.as_str().unwrap_or_default().to_owned())
                            .map_err(|e| CliError::internal(e.to_string()))
                    })
                    .transpose()?;
                acc.steps.push(OperationStep { path: step_path, use_id: use_id.clone(), title: title.clone(), instruction: instruction.clone(), parameters, unresolved_parameter_refs: unresolved_refs, source_block_version_id: version_id.to_owned(), visualization, time_scope: *time_scope, operation_spec: operation_spec.clone(), checks: checks.clone(), });
            }
            ProtocolStep::BlockUse {
                use_id,
                block_version_id,
                bindings,
            } => {
                let (_, child) = block_version_data(db, block_version_id)?;
                let child_slots = validate_parameter_slots(&child.parameter_slots)?;
                let mut child_values = HashMap::new();
                for binding in bindings {
                    let slot = child_slots
                        .get(&binding.slot)
                        .ok_or_else(|| CliError::internal("已保存的子块绑定引用不存在槽位"))?;
                    if let Some(value) = &binding.value {
                        child_values.insert(binding.slot.clone(), BoundValue {
                            value: value.clone(),
                            unit: binding.unit.clone(),
                            provenance: vec![json!({"kind":"literal_binding","block_version_id":version_id,"use_id":use_id})],
                        });
                    } else if let Some(source_key) = &binding.source_slot {
                        if let Some(parent_value) = values.get(source_key) {
                            let mut provenance = parent_value.provenance.clone();
                            provenance.push(json!({"kind":"parent_slot_binding","block_version_id":version_id,"use_id":use_id,"parent_slot":source_key}));
                            child_values.insert(
                                binding.slot.clone(),
                                BoundValue {
                                    value: parent_value.value.clone(),
                                    unit: slot.unit.clone(),
                                    provenance,
                                },
                            );
                        }
                    }
                }
                expand_block(
                    db,
                    block_version_id,
                    child_values,
                    &format!("{path}/{use_id}"),
                    depth + 1,
                    active,
                    acc,
                )?;
            }
        }
    }
    active.remove(version_id);
    Ok(())
}
pub(crate) fn block_preview(
    db: &Connection,
    version_id: &str,
    bindings: &[RootParameterValue],
) -> Result<Value, CliError> {
    let (_, data) = block_version_data(db, version_id)?;
    let block = get(db, &data.block_id)?;
    let values = validate_root_bindings(&data.parameter_slots, bindings)?;
    let mut acc = ComposeAccumulator::default();
    expand_block(
        db,
        version_id,
        values,
        "root",
        1,
        &mut HashSet::new(),
        &mut acc,
    )?;
    let ready = !acc.unresolved_parameters.iter().any(|p| p.required);
    let preview = ProtocolPreview {
        block_version_id: version_id.to_owned(),
        block_id: data.block_id.clone(),
        title: block["data"]["title"]
            .as_str()
            .unwrap_or_default()
            .to_owned(),
        version: data.version,
        ready,
        steps: acc.steps,
        resolved_parameters: acc.resolved_parameters,
        unresolved_parameters: acc.unresolved_parameters,
        block_version_ids: acc.block_version_ids,
        module_contracts: acc.module_contracts,
    };
    serde_json::to_value(preview).map_err(|e| CliError::internal(e.to_string()))
}
/// `protocol.time_budget`: Core's serial time projection over the persisted
/// OperationSpec v2 sidecars of one block version's expanded steps. Steps
/// without `timing` add no duration; repeat/schedule/anchor rules follow
/// `docs/contracts/operation-spec-v2.md`. Read-only: no records change.
pub(crate) fn protocol_time_budget(db: &Connection, version_id: &str) -> Result<Value, CliError> {
    let preview = block_preview(db, version_id, &[])?;
    let steps: Vec<asterlab_core::protocol_preview::OperationStep> =
        serde_json::from_value(preview["steps"].clone())
            .map_err(|e| CliError::internal(e.to_string()))?;
    let mut specs = Vec::with_capacity(steps.len());
    let mut without_timing = 0usize;
    for step in &steps {
        match &step.operation_spec {
            Some(spec) => specs.push(spec.clone()),
            None => without_timing += 1,
        }
    }
    let budget = asterlab_core::protocol_preview::project_time_budget(&specs)
        .map_err(CliError::input)?;
    Ok(serde_json::json!({
        "block_version_id": version_id,
        "step_count": steps.len(),
        "steps_with_timing": specs.len(),
        "steps_without_timing": without_timing,
        "total_span_s": budget.total_span_s,
        "hands_on_s": budget.hands_on_s,
        "parallel_windows": budget.parallel_windows,
    }))
}
pub fn protocol_block_manifest(
    db: &Connection,
    root_version_id: Option<&str>,
    bindings: &Value,
) -> Result<Value, String> {
    let Some(version_id) = root_version_id else {
        return Ok(
            json!({"root_block_version_id":null,"block_version_ids":[],"root_bindings":[],"protocol_ready":true,"steps":[],"resolved_parameters":[],"unresolved_parameters":[],"module_contracts":[]}),
        );
    };
    let typed: Vec<RootParameterValue> = serde_json::from_value(bindings.clone())
        .map_err(|e| format!("Protocol root_bindings 格式无效：{e}"))?;
    let preview = block_preview(db, version_id, &typed).map_err(|e| e.message)?;
    Ok(json!({
        "root_block_version_id":version_id,
        "block_version_ids":preview["block_version_ids"],
        "module_contracts":preview["module_contracts"],
        "root_bindings":typed,
        "protocol_ready":preview["ready"],
        "steps":preview["steps"],
        "resolved_parameters":preview["resolved_parameters"],
        "unresolved_parameters":preview["unresolved_parameters"]
    }))
}
pub(crate) fn protocol_version_get(
    db: &Connection,
    protocol_version_id: &str,
) -> Result<Value, CliError> {
    let (kind, _, protocol_id, revision, raw) = kind_id(db, protocol_version_id)?
        .ok_or_else(|| CliError::missing(format!("Protocol 版本不存在：{protocol_version_id}")))?;
    if kind != "protocol_version" {
        return Err(CliError::missing(format!(
            "Protocol 版本不存在：{protocol_version_id}"
        )));
    }
    let protocol_id = protocol_id.ok_or_else(|| CliError::internal("Protocol 版本缺少所属 Protocol"))?;
    let data: Value = serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    let bindings: Vec<RootParameterValue> = serde_json::from_value(
        data.get("root_bindings").cloned().unwrap_or_else(|| json!([])),
    )
    .map_err(|e| CliError::internal(format!("Protocol root_bindings 无效：{e}")))?;
    let preview = match data["root_block_version_id"].as_str() {
        Some(root_id) => block_preview(db, root_id, &bindings)?,
        None => json!({
            "steps": [], "resolved_parameters": [], "unresolved_parameters": [],
            "block_version_ids": [], "ready": true
        }),
    };
    let parameter_slots = match data["root_block_version_id"].as_str() {
        Some(root_id) => block_version_data(db, root_id)?.1.parameter_slots,
        None => Vec::new(),
    };
    let view = asterlab_core::protocol_version::ProtocolVersionView {
        protocol_version_id: protocol_version_id.to_owned(),
        protocol_id,
        version: data["version"].as_i64().unwrap_or(revision),
        root_block_version_id: data["root_block_version_id"].as_str().map(str::to_owned),
        root_bindings: serde_json::to_value(bindings)
            .map_err(|e| CliError::internal(e.to_string()))?
            .as_array()
            .cloned()
            .unwrap_or_default(),
        parameter_slots,
        steps: serde_json::from_value(preview["steps"].clone())
            .map_err(|e| CliError::internal(e.to_string()))?,
        resolved_parameters: serde_json::from_value(preview["resolved_parameters"].clone())
            .map_err(|e| CliError::internal(e.to_string()))?,
        unresolved_parameters: serde_json::from_value(preview["unresolved_parameters"].clone())
            .map_err(|e| CliError::internal(e.to_string()))?,
        block_version_ids: serde_json::from_value(preview["block_version_ids"].clone())
            .map_err(|e| CliError::internal(e.to_string()))?,
        frozen: true,
    };
    serde_json::to_value(view).map_err(|e| CliError::internal(e.to_string()))
}

pub(crate) fn protocol_version_list(
    db: &Connection,
    protocol_id: &str,
) -> Result<Vec<Value>, CliError> {
    let (kind, _, _, _, _) = kind_id(db, protocol_id)?
        .ok_or_else(|| CliError::missing(format!("Protocol 不存在：{protocol_id}")))?;
    if kind != "protocol" {
        return Err(CliError::missing(format!("Protocol 不存在：{protocol_id}")));
    }
    let mut versions = list(db, Some("protocol_version"), None)?
        .into_iter()
        .filter(|record| record["parent_id"] == protocol_id)
        .map(|record| {
            record["id"]
                .as_str()
                .ok_or_else(|| CliError::internal("Protocol 版本记录缺少 id"))
                .and_then(|id| protocol_version_get(db, id))
        })
        .collect::<Result<Vec<_>, _>>()?;
    versions.sort_by_key(|version| version["version"].as_i64().unwrap_or_default());
    Ok(versions)
}

pub(crate) fn protocol_manifest_entry(
    db: &Connection,
    version_id: &str,
    expected_project: &str,
    assay_id: Value,
    assay_title: Value,
) -> Result<Value, CliError> {
    protocol_manifest_entry_with_bindings(
        db,
        version_id,
        expected_project,
        assay_id,
        assay_title,
        None,
    )
}
pub(crate) fn protocol_manifest_entry_with_bindings(
    db: &Connection,
    version_id: &str,
    expected_project: &str,
    assay_id: Value,
    assay_title: Value,
    root_bindings_override: Option<&[RootParameterValue]>,
) -> Result<Value, CliError> {
    let (kind, project, protocol_id, revision, version_raw) = kind_id(db, version_id)?
        .ok_or_else(|| CliError::missing(format!("Protocol 版本不存在：{version_id}")))?;
    if kind != "protocol_version" {
        return Err(CliError::input("引用必须指向 Protocol 版本"));
    }
    if project.as_deref() != Some(expected_project) {
        return Err(CliError::input("Protocol 版本必须属于同一项目"));
    }
    let protocol_id =
        protocol_id.ok_or_else(|| CliError::internal("Protocol 版本缺少所属 Protocol"))?;
    let (protocol_kind, _, _, _, protocol_raw) =
        kind_id(db, &protocol_id)?.ok_or_else(|| CliError::missing("Protocol 不存在"))?;
    if protocol_kind != "protocol" {
        return Err(CliError::internal("Protocol 版本的所属记录类型无效"));
    }
    let protocol_data: Value =
        serde_json::from_str(&protocol_raw).map_err(|e| CliError::internal(e.to_string()))?;
    let version_data: Value =
        serde_json::from_str(&version_raw).map_err(|e| CliError::internal(e.to_string()))?;
    let empty_bindings = json!([]);
    let override_bindings = root_bindings_override
        .map(serde_json::to_value)
        .transpose()
        .map_err(|e| CliError::internal(e.to_string()))?;
    let root_bindings = override_bindings
        .as_ref()
        .or_else(|| version_data.get("root_bindings"))
        .unwrap_or(&empty_bindings);
    let block_manifest = protocol_block_manifest(
        db,
        version_data["root_block_version_id"].as_str(),
        root_bindings,
    )
    .map_err(CliError::input)?;
    Ok(json!({
        "assay_id":assay_id,
        "assay_title":assay_title,
        "protocol_version_id":version_id,
        "protocol_id":protocol_id,
        "protocol_title":protocol_data["title"],
        "protocol_version":version_data["version"].as_u64().unwrap_or(revision as u64),
        "root_block_version_id":block_manifest["root_block_version_id"],
        "block_version_ids":block_manifest["block_version_ids"],
        "module_contracts":block_manifest["module_contracts"],
        "root_bindings":block_manifest["root_bindings"],
        "protocol_ready":block_manifest["protocol_ready"],
        "steps":block_manifest["steps"],
        "resolved_parameters":block_manifest["resolved_parameters"],
        "unresolved_parameters":block_manifest["unresolved_parameters"]
    }))
}
pub(crate) fn build_run_manifests(
    db: &Connection,
    design_data: &Value,
    project_id: &str,
    global_protocol_version_id: Option<&str>,
) -> Result<(Vec<Value>, Vec<Value>), CliError> {
    let design: DesignFields = serde_json::from_value(design_data.clone())
        .map_err(|e| CliError::internal(format!("当前设计数据无效：{e}")))?;
    let mut protocol_manifest = Vec::new();
    for assay in &design.assays {
        if let Some(version_id) = assay
            .protocol_version_id
            .as_deref()
            .filter(|id| !id.is_empty())
        {
            protocol_manifest.push(protocol_manifest_entry(
                db,
                version_id,
                project_id,
                json!(assay.id),
                json!(assay.title),
            )?);
        }
    }
    if let Some(version_id) = global_protocol_version_id.filter(|id| !id.is_empty()) {
        let already_manifested = protocol_manifest.iter().any(|entry| {
            entry["protocol_version_id"].as_str() == Some(version_id)
        });
        if !already_manifested {
            protocol_manifest.push(protocol_manifest_entry(
                db,
                version_id,
                project_id,
                Value::Null,
                json!("未分配到检测的全局 Protocol"),
            )?);
        }
    }

    let mut material_manifest = Vec::new();
    for output in &design.planned_outputs {
        let producer_protocol = protocol_manifest_entry_with_bindings(
            db,
            &output.protocol_version_id,
            project_id,
            Value::Null,
            json!(format!("材料制备：{}", output.title)),
            Some(&output.root_bindings),
        )?;
        let consumers: Vec<Value> = design
            .sample_allocations
            .iter()
            .filter(|allocation| {
                allocation.planned_output_id.as_deref() == Some(output.id.as_str())
            })
            .map(|allocation| {
                let assay = design
                    .assays
                    .iter()
                    .find(|assay| assay.id == allocation.assay_id);
                json!({
                    "assay_id":allocation.assay_id,
                    "assay_title":assay.map(|value| value.title.as_str()).unwrap_or("未知检测"),
                    "group_id":allocation.group_id,
                    "group_name":allocation.group_name
                })
            })
            .collect();
        material_manifest.push(json!({
            "planned_output_id":output.id,
            "title":output.title,
            "evidence_id":output.evidence_id,
            "producer_protocol":producer_protocol,
            "consumers":consumers
        }));
    }
    Ok((protocol_manifest, material_manifest))
}

pub(crate) fn step_materialization_issues(design: &Value, protocol_manifest: &[Value]) -> Vec<String> {
    let mygo = &design["mygo"];
    let tasks = mygo["stepTasks"].as_array().cloned().unwrap_or_default();
    let mut fallback_execution = serde_json::Map::new();
    let execution = match mygo.get("stepExecution") {
        Some(value) => match value.as_object() {
            Some(execution) => execution,
            None => return vec!["STEP_EXECUTION_INVALID: stepExecution must be an object".to_owned()],
        },
        None => {
            for task in &tasks {
                if let Some(step_id) = task["stepId"].as_str() {
                    fallback_execution.entry(step_id.to_owned()).or_insert(json!("executable"));
                }
            }
            if fallback_execution.is_empty() {
                return Vec::new();
            }
            &fallback_execution
        }
    };
    let mut issues = Vec::new();
    let mut has_executable = false;
    for (step_id, mode) in execution {
        let path = tasks
            .iter()
            .find(|task| task["stepId"].as_str() == Some(step_id.as_str()))
            .and_then(|task| task["path"].as_str())
            .filter(|path| !path.trim().is_empty())
            .unwrap_or(step_id);
        let blocks = mygo["stepBlocks"][step_id]
            .as_array()
            .map(|blocks| blocks.iter().filter_map(Value::as_str).collect::<Vec<_>>())
            .unwrap_or_default();
        let mode = mode.as_str().unwrap_or("");
        match mode {
            "organizational" => {
                if !blocks.is_empty()
                    || tasks.iter().any(|task| task["stepId"].as_str() == Some(step_id))
                {
                    issues.push(format!(
                        "STEP_EXECUTION_INVALID: organizational step {path} carries executable bindings"
                    ));
                }
            }
            "executable" => {
                has_executable = true;
                if blocks.len() != 1 {
                    issues.push(format!(
                        "STEP_NOT_MATERIALIZED: {path} must reference exactly one ProtocolBlock version"
                    ));
                    continue;
                }
                let assay_id = mygo["stepAssays"][step_id].as_str().or_else(|| {
                    tasks
                        .iter()
                        .find(|task| task["stepId"].as_str() == Some(step_id.as_str()))
                        .and_then(|task| task["assayId"].as_str())
                }).unwrap_or("");
                if assay_id.is_empty() {
                    issues.push(format!(
                        "STEP_NOT_MATERIALIZED: {path} has no assay ProtocolVersion scope"
                    ));
                    continue;
                }
                let Some(protocol) = protocol_manifest
                    .iter()
                    .find(|entry| entry["assay_id"].as_str() == Some(assay_id))
                else {
                    issues.push(format!(
                        "STEP_NOT_MATERIALIZED: {path} assay {assay_id} has no frozen ProtocolVersion"
                    ));
                    continue;
                };
                let block_ids = protocol["block_version_ids"]
                    .as_array()
                    .into_iter()
                    .flatten()
                    .filter_map(Value::as_str);
                if !block_ids.into_iter().any(|id| id == blocks[0]) {
                    issues.push(format!(
                        "STEP_NOT_MATERIALIZED: {path} ProtocolBlock {} is absent from assay {assay_id}'s frozen ProtocolVersion",
                        blocks[0]
                    ));
                }
            }
            _ => issues.push(format!(
                "STEP_EXECUTION_INVALID: step {step_id} has unknown execution mode {mode:?}"
            )),
        }
    }
    if !has_executable {
        issues.push("NO_EXECUTABLE_STEP: design has no executable Step".to_owned());
    }
    issues
}

pub(crate) fn asterdoc_step_materialization_issues(
    db: &Connection,
    design_id: &str,
    design: &Value,
) -> Result<Vec<String>, CliError> {
    let (kind, project, _, _, _) = kind_id(db, design_id)?
        .ok_or_else(|| CliError::missing("实验设计不存在"))?;
    if kind != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let project = project.ok_or_else(|| CliError::internal("实验设计缺少项目归属"))?;
    let mut design_fields = design.clone();
    if let Some(object) = design_fields.as_object_mut() {
        object.remove("mygo");
        object.remove("asterlab");
    }
    let (protocol_manifest, _) = build_run_manifests(db, &design_fields, &project, None)?;
    Ok(step_materialization_issues(design, &protocol_manifest))
}

pub(crate) fn labware_profile_snapshot(db: &Connection, design_data: &Value) -> Result<Vec<Value>, CliError> {
    let plan: ContainerPlan = serde_json::from_value(
        design_data
            .get("container_plan")
            .cloned()
            .unwrap_or_else(|| json!({})),
    )
    .map_err(|e| CliError::internal(format!("设计 container_plan 无效：{e}")))?;
    let registry = load_labware_registry(db)?;
    let mut seen = std::collections::HashSet::new();
    let mut profiles = Vec::new();
    for container in &plan.containers {
        let key = format!(
            "{}@{}",
            container.profile_ref.id, container.profile_ref.version
        );
        if !seen.insert(key) {
            continue;
        }
        let profile = registry.get(&container.profile_ref).ok_or_else(|| {
            CliError::input(format!(
                "容器 {} 引用了未注册的 LabwareProfile {}@{}",
                container.id, container.profile_ref.id, container.profile_ref.version
            ))
        })?;
        profiles
            .push(serde_json::to_value(profile).map_err(|e| CliError::internal(e.to_string()))?);
    }
    Ok(profiles)
}


pub(crate) fn execution_resource_snapshot(
    db: &Connection,
    project_id: &str,
    design_data: &Value,
    view_id: Option<&str>,
) -> Result<Vec<Value>, CliError> {
    let protocol_ids: HashSet<String> = design_data["assays"]
        .as_array()
        .into_iter()
        .flatten()
        .filter_map(|assay| assay["protocol_version_id"].as_str())
        .filter(|id| !id.is_empty())
        .map(str::to_owned)
        .collect();
    if protocol_ids.is_empty() {
        return Ok(Vec::new());
    }
    let mut resources = Vec::new();
    let mut seen = HashSet::new();
    let views = if let Some(id) = view_id {
        let view = get(db, id)?;
        if view["kind"] != "view" || view["project_id"].as_str() != Some(project_id) {
            return Err(CliError::input("view_id 必须是当前项目的 View"));
        }
        vec![view]
    } else {
        list(db, Some("view"), Some(project_id))?
    };
    for view in views {
        let Some(view_id) = view["id"].as_str() else { continue };
        let document = get_view(db, view_id)?["latest_version"]["data"]["root"].clone();
        let mut stack = vec![document];
        while let Some(node) = stack.pop() {
            if node.is_null() {
                continue;
            }
            let component = node.get("component").unwrap_or(&node);
            if let Some(reference) = component.get("model_ref") {
                let step = &reference["step"];
                let protocol_id = step["protocol_version_id"].as_str().unwrap_or_default();
                let step_path = step["step_path"].as_str().unwrap_or_default();
                if reference["kind"].as_str() == Some("operation_use")
                    && protocol_ids.contains(protocol_id)
                    && !step_path.is_empty()
                {
                    let scope = step["scope"].clone();
                    let key = serde_json::to_string(&json!({"path":step_path,"scope":&scope}))
                        .unwrap_or_else(|_| format!("{step_path}:{scope}"));
                    if seen.insert(key) {
                        let mut tables = serde_json::Map::new();
                        for child in component["slots"]["detail"].as_array().into_iter().flatten() {
                            let preset = child["props"]["preset"].as_str().unwrap_or_default();
                            if ["materials", "instruments", "checks"].contains(&preset) {
                                tables.insert(
                                    preset.to_owned(),
                                    json!({
                                        "title": child["props"]["title"],
                                        "columns": child["props"]["columns"],
                                        "rows": child["props"]["rows"]
                                    }),
                                );
                            }
                        }
                        resources.push(json!({
                            "step_path": step_path,
                            "scope": scope,
                            "tables": tables
                        }));
                    }
                }
            }
            if let Some(slots) = component["slots"].as_object() {
                for child in slots.values().filter_map(Value::as_array).flatten() {
                    stack.push(child.clone());
                }
            }
            if let Some(nodes) = component["workflow"]["nodes"].as_array() {
                for child in nodes {
                    stack.push(child.clone());
                }
            }
        }
    }
    Ok(resources)
}

pub(crate) fn create_run_in_transaction(
    tx: &Transaction<'_>,
    design_id: &str,
    title: &str,
    scheduled_date: Option<&str>,
    global_protocol_version_id: Option<&str>,
    view_id: Option<&str>,
    repeat_of_run_id: Option<&str>,
    repeat_relationship: &str,
    operation_id: Option<&str>,
) -> Result<Value, CliError> {
    create_run_with_source_in_transaction(
        tx,
        design_id,
        title,
        scheduled_date,
        global_protocol_version_id,
        view_id,
        repeat_of_run_id,
        repeat_relationship,
        None,
        operation_id,
    )
}

/// `create_run_in_transaction` plus the optional upstream sample-source Run.
/// Separate so the MyGo bridge and existing tests keep the 9-argument call.
pub(crate) fn create_run_with_source_in_transaction(
    tx: &Transaction<'_>,
    design_id: &str,
    title: &str,
    scheduled_date: Option<&str>,
    global_protocol_version_id: Option<&str>,
    view_id: Option<&str>,
    repeat_of_run_id: Option<&str>,
    repeat_relationship: &str,
    source_run_id: Option<&str>,
    operation_id: Option<&str>,
) -> Result<Value, CliError> {
    check_text(title, "运行名称", 160)?;
    if let Some(date) = scheduled_date.filter(|date| !date.trim().is_empty()) {
        validate_date(date, "计划开始日期")?;
    }
    if !["independent", "same_source", "paired", "unknown"].contains(&repeat_relationship) {
        return Err(CliError::input(
            "repeat_relationship 只能是 independent、same_source、paired 或 unknown",
        ));
    }
    let (kind, project, _, revision, design_raw) =
        kind_id(tx, design_id)?.ok_or_else(|| CliError::missing("实验设计不存在"))?;
    if kind != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let project = project.ok_or_else(|| CliError::internal("实验设计缺少项目归属"))?;
    let version: String = tx
        .query_row(
            "SELECT id FROM records WHERE kind='design_version' AND parent_id=?1 AND revision=?2",
            params![design_id, revision],
            |row| row.get(0),
        )
        .optional()
        .map_err(|e| CliError::internal(e.to_string()))?
        .ok_or_else(|| CliError::internal("当前设计版本快照缺失"))?;
    let mut design_data: Value =
        serde_json::from_str(&design_raw).map_err(|e| CliError::internal(e.to_string()))?;
    // Revalidate the design before freezing it. Keep the MyGo extension for
    // the source check, then strip it before strict DesignFields parsing.
    let creation_audit = run_creation_audit(&design_data, revision, &version)?;
    let design_checks = design_data
        .get("mygo")
        .and_then(|mygo| mygo.get("stepChecks"))
        .cloned()
        .unwrap_or_else(|| json!([]));
    let step_design = json!({
        "mygo": design_data.get("mygo").cloned().unwrap_or(Value::Null)
    });
    if let Some(object) = design_data.as_object_mut() {
        object.remove("mygo");
    }
    let (protocol_manifest, material_manifest) =
        build_run_manifests(tx, &design_data, &project, global_protocol_version_id)?;
    let materialization_issues = step_materialization_issues(&step_design, &protocol_manifest);
    if !materialization_issues.is_empty() {
        return Err(CliError::input(materialization_issues.join("; ")));
    }
    let labware_profiles = labware_profile_snapshot(tx, &design_data)?;
    let execution_resources = execution_resource_snapshot(tx, &project, &design_data, view_id)?;
    if let Some(previous_id) = repeat_of_run_id.filter(|id| !id.is_empty()) {
        let (previous_kind, previous_project, previous_design, _, _) = kind_id(tx, previous_id)?
            .ok_or_else(|| CliError::missing("被重复执行的 Run 不存在"))?;
        if previous_kind != "run"
            || previous_design.as_deref() != Some(design_id)
            || previous_project.as_deref() != Some(project.as_str())
        {
            return Err(CliError::input(
                "repeat_of_run_id 必须指向同项目、同一设计下的既有 Run",
            ));
        }
    }
    // A Run may name one upstream harvest Run it draws samples from. The
    // relation is project-scoped, so it can cross Designs; it never rewrites
    // `repeat_of_run_id`, which stays the same-Design re-execution relation.
    if let Some(source_id) = source_run_id.filter(|id| !id.is_empty()) {
        let (source_kind, source_project, _, _, _) = kind_id(tx, source_id)?
            .ok_or_else(|| CliError::missing("source_run_id 指向的上游 Run 不存在"))?;
        if source_kind != "run" {
            return Err(CliError::input("source_run_id 必须指向实验运行"));
        }
        if source_project.as_deref() != Some(project.as_str()) {
            return Err(CliError::input(
                "source_run_id 必须指向同一项目下的上游 Run；跨项目引用不成立",
            ));
        }
    }
    let run_index: i64 = tx
        .query_row(
            "SELECT COUNT(*) + 1 FROM records WHERE kind='run' AND parent_id=?1",
            [design_id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut data = json!({
        "title":title.trim(),
        "scheduled_date":scheduled_date.filter(|date| !date.trim().is_empty()),
        "protocol_version_id":global_protocol_version_id.filter(|id| !id.is_empty()),
        "protocol_manifest":protocol_manifest,
        "material_manifest":material_manifest,
        "container_plan":design_data["container_plan"].clone(),
        "labware_profiles":labware_profiles,
        "execution_resources":execution_resources,
        "view_id":view_id.filter(|id| !id.is_empty()),
        "design_version_id":version,
        "design_revision":revision,
        "run_index":run_index,
        "repeat_of_run_id":repeat_of_run_id.filter(|id| !id.is_empty()),
        "repeat_relationship":repeat_relationship,
        "source_run_id":source_run_id.filter(|id| !id.is_empty()),
        "creation_audit":creation_audit,
        "design_checks":design_checks,
        "status":"planned"
    });
    if let Some(op) = operation_id.filter(|s| !s.trim().is_empty()) {
        data["operation_id"] = json!(op);
        data["operation_payload"] = json!({"design_id": design_id, "title": title});
    }
    insert(tx, "run", Some(&project), Some(design_id), 1, &data)
}

// run_creation_audit recomputes the design rules Core can verify without the
// UI and returns the audit stored on the Run in the same transaction. Errors
// block creation; warnings are recorded but never block a novel design.
pub(crate) fn run_creation_audit(design_data: &Value, revision: i64, version_id: &str) -> Result<Value, CliError> {
    let mut errors: Vec<String> = Vec::new();
    let mut warnings: Vec<String> = Vec::new();
    if let Some(mygo) = design_data.get("mygo") {
        if let Some(step_tasks) = mygo.get("stepTasks").and_then(Value::as_array) {
            let mut claims: std::collections::HashMap<(String, String), usize> =
                std::collections::HashMap::new();
            for link in step_tasks {
                let assay = link.get("assayId").and_then(Value::as_str).unwrap_or("").trim();
                let timepoint = link.get("timepoint").and_then(Value::as_str).unwrap_or("").trim();
                if assay.is_empty() {
                    continue;
                }
                *claims.entry((assay.to_owned(), timepoint.to_owned())).or_default() += 1;
            }
            let mut pairs: Vec<_> = claims.into_iter().filter(|(_, count)| *count > 1).collect();
            pairs.sort();
            for ((assay, timepoint), count) in pairs {
                warnings.push(format!(
                    "检测 {assay} 的 {timepoint} 被 {count} 个步骤共享；任务按 path / Step ID 区分，不按时间点合并。"
                ));
            }
        }
        if let Some(assays) = mygo.get("assays").and_then(Value::as_object) {
            let mut assay_ids: Vec<&String> = assays.keys().collect();
            assay_ids.sort();
            for assay_id in assay_ids {
                let params = &assays[assay_id];
                let declared: std::collections::HashSet<String> = params
                    .get("timepoints")
                    .and_then(Value::as_array)
                    .into_iter()
                    .flatten()
                    .filter_map(Value::as_str)
                    .map(|value| value.trim().to_owned())
                    .collect();
                let mut seen = std::collections::HashSet::new();
                for well in params.get("wells").and_then(Value::as_array).into_iter().flatten() {
                    let well_id = well.get("wellId").and_then(Value::as_str).unwrap_or("").trim().to_uppercase();
                    let timepoint = well.get("timepoint").and_then(Value::as_str).unwrap_or("").trim();
                    let sample = well.get("sampleId").and_then(Value::as_str).unwrap_or("").trim();
                    let group = well.get("groupId").and_then(Value::as_str).unwrap_or("").trim();
                    let replicate = well.get("replicate").and_then(Value::as_i64).unwrap_or(0);
                    let mut chars = well_id.chars();
                    let row = chars.next().unwrap_or(' ');
                    let col: i64 = chars.as_str().parse().unwrap_or(0);
                    let valid_plate_well = ('A'..='P').contains(&row) && (1..=24).contains(&col);
                    let valid_linear_slot = well_id.parse::<i64>().is_ok_and(|idx| (1..=1536).contains(&idx));
                    if !valid_plate_well && !valid_linear_slot {
                        errors.push(format!("检测 {assay_id} 的孔位 {well_id} 无效，应为 A1–P24 或正整数槽位。"));
                        continue;
                    }
                    if group.is_empty() {
                        errors.push(format!("检测 {assay_id} 的孔位 {well_id} 缺少分组。"));
                    }
                    if replicate < 0 || (replicate > 0 && sample.is_empty()) {
                        errors.push(format!("检测 {assay_id} 的孔位 {well_id} 样本/重复编号无效。"));
                    }
                    if sample.is_empty() || replicate == 0 {
                        warnings.push(format!(
                            "检测 {assay_id} 的孔位 {well_id} 尚未指定完整的样本与技术重复映射。"
                        ));
                    }
                    let key = format!("{well_id}@{timepoint}");
                    if !seen.insert(key) {
                        errors.push(format!("检测 {assay_id} 的孔位 {well_id} 在 {timepoint} 被重复分配。"));
                    }
                    if !timepoint.is_empty() && !declared.contains(timepoint) {
                        warnings.push(format!(
                            "检测 {assay_id} 的孔位使用了未在检测时间点中声明的时间点 {timepoint}。"
                        ));
                    }
                }
            }
        }
    }
    if !errors.is_empty() {
        errors.sort();
        return Err(CliError::input(errors.join("；")));
    }
    warnings.sort();
    warnings.dedup();
    // The designer saves the warnings the researcher confirmed into this
    // design revision before creating the Run. Acknowledgement is therefore
    // bound to the same revision Core freezes; a stale or absent ack cannot
    // claim the current warnings were accepted.
    let ack = design_data.get("mygo").and_then(|mygo| mygo.get("preflightAck"));
    let ack_warnings: std::collections::HashSet<String> = ack
        .and_then(|ack| ack.get("warnings"))
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(Value::as_str)
        .map(str::to_owned)
        .collect();
    let acknowledged: Vec<String> = warnings
        .iter()
        .filter(|warning| ack_warnings.contains(*warning))
        .cloned()
        .collect();
    let decision = if warnings.is_empty() {
        "created_clean"
    } else if acknowledged.len() == warnings.len() {
        "acknowledged_warnings"
    } else {
        "created_with_unacknowledged_warnings"
    };
    let actor = ack
        .and_then(|ack| ack.get("actorId"))
        .and_then(Value::as_str)
        .filter(|value| !value.trim().is_empty())
        .map(str::to_owned)
        .or_else(|| {
            std::env::var("USERNAME")
                .or_else(|_| std::env::var("USER"))
                .ok()
                .filter(|value| !value.trim().is_empty())
        });
    Ok(json!({
        "rules_version": 1,
        "design_revision": revision,
        "design_version_id": version_id,
        "actor_id": actor,
        "recorded_at": stamp(),
        "decision": decision,
        "acknowledged_warnings": acknowledged,
        "warnings": warnings,
        "errors": []
    }))
}


