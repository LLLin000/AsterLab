use super::*;

pub(crate) fn preview_protocol_block(
    db: &Connection,
    version_id: &str,
    bindings: &Value,
) -> Result<Value, CliError> {
    let typed: Vec<RootParameterValue> = serde_json::from_value(bindings.clone())
        .map_err(|e| CliError::input(format!("步骤块 bindings 格式无效：{e}")))?;
    block_preview(db, version_id, &typed)
}
pub(crate) fn create_protocol_block_records(
    tx: &Transaction<'_>,
    project_id: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let mut input = data.clone();
    input["project_id"] = json!(project_id);
    let i: ProtocolBlockCreate = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("步骤块数据结构无效：{e}")))?;
    ensure_project(tx, &i.project_id)?;
    check_text(&i.title, "步骤块名称", 160)?;
    if i.summary.chars().count() > 1000 {
        return Err(CliError::input("步骤块摘要不能超过 1000 个字符"));
    }
    validate_block_content(tx, &i.project_id, &i.parameter_slots, &i.steps)?;
    let block = insert(
        tx,
        "protocol_block",
        Some(&i.project_id),
        Some(&i.project_id),
        1,
        &json!({"title":i.title.trim(),"summary":i.summary}),
    )?;
    let version_data = ProtocolBlockVersionData {
        block_id: block["id"].as_str().unwrap_or_default().to_owned(),
        version: 1,
        summary: i.summary,
        parameter_slots: i.parameter_slots,
        steps: i.steps,
        module_contract: None,
        module_origin: None,
        module_contract_authority: None,
        catalog_review: None,
        based_on_version_id: None,
        step_edit: None,
    };
    let serialized =
        serde_json::to_value(version_data).map_err(|e| CliError::internal(e.to_string()))?;
    let version = insert(
        tx,
        "protocol_block_version",
        Some(&i.project_id),
        block["id"].as_str(),
        1,
        &serialized,
    )?;
    Ok(json!({"block":block,"version":version}))
}
pub(crate) fn revise_protocol_block_records(
    tx: &Transaction<'_>,
    block_id: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let mut input = data.clone();
    input["block_id"] = json!(block_id);
    let i: ProtocolBlockRevise = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("步骤块修订数据结构无效：{e}")))?;
    let (kind, project, _, _, _) =
        kind_id(tx, &i.block_id)?.ok_or_else(|| CliError::missing("步骤块不存在"))?;
    if kind != "protocol_block" {
        return Err(CliError::input("block_id 必须指向 protocol_block"));
    }
    let project_id = project.ok_or_else(|| CliError::internal("步骤块缺少项目归属"))?;
    let latest: i64 = tx.query_row("SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='protocol_block_version' AND parent_id=?1", [&i.block_id], |r| r.get(0)).map_err(|e| CliError::internal(e.to_string()))?;
    if latest != i.expected_version {
        return Err(CliError::conflict(format!(
            "步骤块已从 v{} 更新为 v{}；请先重新读取。",
            i.expected_version, latest
        )));
    }
    let latest_id: String = tx.query_row("SELECT id FROM records WHERE kind='protocol_block_version' AND parent_id=?1 AND revision=?2", params![i.block_id, latest], |r| r.get(0)).map_err(|e| CliError::internal(e.to_string()))?;
    let (_, current_data) = block_version_data(tx, &latest_id)?;
    let summary = i.summary.unwrap_or(current_data.summary);
    if summary.chars().count() > 1000 {
        return Err(CliError::input("步骤块摘要不能超过 1000 个字符"));
    }
    validate_block_content(tx, &project_id, &i.parameter_slots, &i.steps)?;
    let module_contract = current_data.module_contract.clone();
    let module_origin = module_contract.as_ref().map(|_| ModuleOrigin::Derived);
    let module_contract_authority = module_contract
        .as_ref()
        .map(|_| asterlab_core::protocol_block::ModuleContractAuthority::SourceOnly);
    let version_data = ProtocolBlockVersionData {
        block_id: i.block_id.clone(),
        version: latest + 1,
        summary,
        parameter_slots: i.parameter_slots,
        steps: i.steps,
        module_contract,
        module_origin,
        module_contract_authority,
        catalog_review: None,
        based_on_version_id: None,
        step_edit: None,
    };
    let serialized =
        serde_json::to_value(version_data).map_err(|e| CliError::internal(e.to_string()))?;
    let version = insert(
        tx,
        "protocol_block_version",
        Some(&project_id),
        Some(&i.block_id),
        latest + 1,
        &serialized,
    )?;
    Ok(json!({"version":version}))
}
pub(crate) fn apply_binding_changes(
    tx: &Connection,
    data: &ProtocolBlockVersionData,
    changeset: &ChangeSet,
) -> Result<(ProtocolBlockVersionData, Vec<Value>), CliError> {
    let mut next = data.clone();
    let mut diff = Vec::new();
    for change in &changeset.changes {
        let parts = change.path.split('.').collect::<Vec<_>>();
        if parts.len() != 4 || parts[0] != "steps" || parts[2] != "bindings" {
            return Err(CliError::input(format!(
                "仅支持 steps.<use_id>.bindings.<slot> 路径，收到 {}",
                change.path
            )));
        }
        let use_id = parts[1];
        let slot_key = parts[3];
        let matches = next
            .steps
            .iter()
            .enumerate()
            .filter(|(_, step)| matches!(step, ProtocolStep::BlockUse { use_id: id, .. } if id == use_id))
            .map(|(index, _)| index)
            .collect::<Vec<_>>();
        if matches.len() != 1 {
            return Err(CliError::input(format!(
                "use_id {} 必须唯一存在，实际匹配 {} 处",
                use_id,
                matches.len()
            )));
        }
        let index = matches[0];
        let ProtocolStep::BlockUse {
            block_version_id,
            bindings,
            ..
        } = &mut next.steps[index]
        else {
            unreachable!();
        };
        let (_, child) = block_version_data(tx, block_version_id)?;
        let child_slots = validate_parameter_slots(&child.parameter_slots)?;
        let child_slot = child_slots
            .get(slot_key)
            .ok_or_else(|| CliError::input(format!("子块不存在参数槽位：{slot_key}")))?;
        let binding = bindings
            .iter_mut()
            .find(|binding| binding.slot == slot_key)
            .ok_or_else(|| CliError::input(format!("BlockUse 不存在绑定：{slot_key}")))?;
        let old_value = binding.value.clone();
        match change.operation {
            asterlab_core::semantic_changeset::FieldOperation::Set => {
                let value = change
                    .value
                    .clone()
                    .ok_or_else(|| CliError::input("Set 必须提供 value"))?;
                validate_parameter_value(child_slot, &value, child_slot.unit.as_deref())?;
                if let Some(module) = &child.module_contract {
                    if let Some(parameter) = module.parameters.iter().find(|p| p.key == slot_key) {
                        let typed = ParameterBinding {
                            parameter_key: slot_key.to_string(),
                            value: value.clone(),
                            unit: child_slot.unit.clone(),
                            source: ValueSource::ProtocolFixed,
                            source_reference: Some("semantic.apply_binding_patch".into()),
                            confirmed: true,
                        };
                        typed.validate_against(parameter).map_err(|issues| {
                            CliError::input(
                                issues
                                    .into_iter()
                                    .map(|issue| format!("{}: {}", issue.code, issue.message))
                                    .collect::<Vec<_>>()
                                    .join("; "),
                            )
                        })?;
                    }
                }
                binding.value = Some(value.clone());
                binding.source_slot = None;
                binding.unit = child_slot.unit.clone();
                diff.push(json!({"path":change.path,"old_value":old_value,"new_value":value}));
            }
            asterlab_core::semantic_changeset::FieldOperation::Unset => {
                if child_slot.required {
                    return Err(CliError::input(format!(
                        "必填参数 {} 不能删除；先改为非必填或保留值",
                        slot_key
                    )));
                }
                binding.value = None;
                binding.source_slot = None;
                binding.unit = None;
                diff.push(json!({"path":change.path,"old_value":old_value,"new_value":null}));
            }
        }
    }
    Ok((next, diff))
}

pub(crate) struct DbComponentContext<'a> {
    pub(crate) db: &'a Connection,
    pub(crate) project_id: &'a str,
    pub(crate) design_version_id: Option<&'a str>,
}

impl<'a> DbComponentContext<'a> {
    pub(crate) fn new(db: &'a Connection, project_id: &'a str) -> Self {
        Self {
            db,
            project_id,
            design_version_id: None,
        }
    }

    pub(crate) fn with_design(
        db: &'a Connection,
        project_id: &'a str,
        design_version_id: Option<&'a str>,
    ) -> Self {
        Self {
            db,
            project_id,
            design_version_id,
        }
    }

    pub(crate) fn design_versions(&self) -> Vec<Value> {
        if let Some(id) = self.design_version_id {
            return self
                .owned_record(id, "design_version")
                .into_iter()
                .collect();
        }
        list(self.db, Some("design_version"), Some(self.project_id))
            .ok()
            .into_iter()
            .flatten()
            .collect()
    }

    pub(crate) fn scope_exists(&self, protocol_version_id: &str, scope: &ProtocolScope) -> bool {
        match scope {
            ProtocolScope::Unscoped => true,
            ProtocolScope::Assay { assay_id } => {
                self.design_versions().into_iter().any(|version| {
                    version["data"]["snapshot"]["assays"]
                        .as_array()
                        .is_some_and(|assays| {
                            assays.iter().any(|assay| {
                                assay["id"].as_str() == Some(assay_id.as_str())
                                    && assay["protocol_version_id"].as_str()
                                        == Some(protocol_version_id)
                            })
                        })
                })
            }
            ProtocolScope::PlannedOutput { planned_output_id } => {
                self.design_versions().into_iter().any(|version| {
                    version["data"]["snapshot"]["planned_outputs"]
                        .as_array()
                        .is_some_and(|outputs| {
                            outputs.iter().any(|output| {
                                output["id"].as_str() == Some(planned_output_id.as_str())
                                    && output["protocol_version_id"].as_str()
                                        == Some(protocol_version_id)
                            })
                        })
                })
            }
        }
    }

    pub(crate) fn owned_record(&self, id: &str, kind: &str) -> Option<Value> {
        let record = get(self.db, id).ok()?;
        if record["kind"] != kind || record["project_id"].as_str() != Some(self.project_id) {
            return None;
        }
        Some(record)
    }

    pub(crate) fn resolve_path(&self, value: &Value, path: &str) -> ModelPathResolution {
        let mut current = value.clone();
        for segment in path.split('.').filter(|segment| !segment.is_empty()) {
            let Some(next) = current.get(segment) else {
                return ModelPathResolution::Missing;
            };
            current = next.clone();
        }
        if current.is_null() {
            return ModelPathResolution::Unknown;
        }
        let value_type = match current {
            Value::String(_) => asterlab_core::component_framework::ComponentValueType::Text,
            Value::Number(_) => asterlab_core::component_framework::ComponentValueType::Number,
            Value::Bool(_) => asterlab_core::component_framework::ComponentValueType::Boolean,
            Value::Array(_) | Value::Object(_) => {
                asterlab_core::component_framework::ComponentValueType::Data
            }
            Value::Null => return ModelPathResolution::Unknown,
        };
        ModelPathResolution::Resolved(value_type)
    }
}

impl ComponentBindingContext for DbComponentContext<'_> {
    fn resolve_model_path(&self, model: &ModelRef, path: &str) -> ModelPathResolution {
        if let ModelRef::PlannedContainer {
            design_version_id,
            container_id,
        } = model
        {
            let Some(record) = self.owned_record(design_version_id, "design_version") else {
                return ModelPathResolution::Missing;
            };
            let Some(container) = record["data"]["snapshot"]["container_plan"]["containers"]
                .as_array()
                .and_then(|containers| {
                    containers
                        .iter()
                        .find(|container| container["id"].as_str() == Some(container_id.as_str()))
                })
            else {
                return ModelPathResolution::Missing;
            };
            return self.resolve_path(container, path);
        }
        let (id, kind) = match model {
            ModelRef::DesignVersion { id } => (id, "design_version"),
            ModelRef::ProtocolVersion { id } => (id, "protocol_version"),
            ModelRef::BlockVersion { id } => (id, "protocol_block_version"),
            ModelRef::Run { id } => (id, "run"),
            ModelRef::OperationUse { .. } => return ModelPathResolution::Unknown,
            ModelRef::PlannedContainer { .. } => return ModelPathResolution::Unknown,
        };
        let Some(record) = self.owned_record(id, kind) else {
            return ModelPathResolution::Missing;
        };
        let data = if kind == "design_version" {
            &record["data"]["snapshot"]
        } else {
            &record["data"]
        };
        self.resolve_path(data, path)
    }

    fn operation_use_exists(
        &self,
        block_version_id: &str,
        step: &asterlab_core::semantic_component::QualifiedStepRef,
    ) -> bool {
        if !self.scope_exists(&step.protocol_version_id, &step.scope) {
            return false;
        }
        let Some(block) = self.owned_record(block_version_id, "protocol_block_version") else {
            return false;
        };
        let Some(protocol) = self.owned_record(&step.protocol_version_id, "protocol_version")
        else {
            return false;
        };
        let Some(root_block_version_id) = protocol["data"]["root_block_version_id"].as_str() else {
            return false;
        };
        let bindings = serde_json::from_value::<Vec<RootParameterValue>>(
            protocol["data"]["root_bindings"].clone(),
        )
        .unwrap_or_default();
        let Ok(preview) = block_preview(self.db, root_block_version_id, &bindings) else {
            return false;
        };
        let _ = block;
        preview["steps"].as_array().is_some_and(|steps| {
            steps.iter().any(|candidate| {
                candidate["path"].as_str() == Some(step.step_path.as_str())
                    && candidate["source_block_version_id"].as_str() == Some(block_version_id)
            })
        })
    }
}

pub(crate) fn load_component_registry(db: &Connection) -> Result<ComponentRegistry, CliError> {
    let mut registry = default_registry();
    for record in list(db, Some("component_definition"), None)? {
        let definition: ComponentDefinition = serde_json::from_value(record["data"].clone())
            .map_err(|e| CliError::internal(format!("ComponentDefinition 数据无效：{e}")))?;
        registry
            .register(definition)
            .map_err(|error| CliError::input(error))?;
    }
    Ok(registry)
}

pub(crate) fn create_labware_profile(
    tx: &Transaction<'_>,
    data: &Value,
) -> Result<Value, CliError> {
    let i: LabwareDefine = serde_json::from_value(data.clone())
        .map_err(|e| CliError::input(format!("LabwareProfile 数据结构无效：{e}")))?;
    let mut registry = load_labware_registry(tx)?;
    registry
        .register(i.profile.clone())
        .map_err(CliError::input)?;
    let record = insert(
        tx,
        "labware_profile",
        None,
        None,
        1,
        &serde_json::to_value(i.profile).map_err(|e| CliError::internal(e.to_string()))?,
    )?;
    Ok(json!({"record":record}))
}

pub(crate) fn create_component_definition(
    tx: &Transaction<'_>,
    data: &Value,
) -> Result<Value, CliError> {
    let i: ComponentDefine = serde_json::from_value(data.clone())
        .map_err(|e| CliError::input(format!("ComponentDefinition 数据结构无效：{e}")))?;
    let mut registry = default_registry();
    registry
        .register(i.definition.clone())
        .map_err(CliError::input)?;
    let record = insert(
        tx,
        "component_definition",
        None,
        None,
        1,
        &serde_json::to_value(i.definition).map_err(|e| CliError::internal(e.to_string()))?,
    )?;
    Ok(json!({"record":record}))
}

pub(crate) fn get_view(db: &Connection, id: &str) -> Result<Value, CliError> {
    let record = get(db, id)?;
    if record["kind"] != "view" {
        return Err(CliError::input("id 必须指向 view"));
    }
    let versions: Vec<Value> = list(db, Some("view_document"), record["project_id"].as_str())?
        .into_iter()
        .filter(|version| version["parent_id"] == id)
        .collect();
    let latest = versions
        .iter()
        .max_by_key(|version| version["revision"].as_i64().unwrap_or(0))
        .cloned()
        .ok_or_else(|| CliError::internal("View 缺少文档版本"))?;
    Ok(json!({"view":record,"latest_version":latest,"versions":versions}))
}

pub(crate) fn revise_view_tx(
    tx: &Transaction<'_>,
    view_id: &str,
    changeset: &Value,
) -> Result<Value, CliError> {
    let payload = json!({"view_id":view_id,"changeset":changeset});
    revise_view_records(tx, view_id, &payload)
}

pub(crate) fn view_step_detail(
    db: &Connection,
    view_id: &str,
    instance_id: &str,
) -> Result<Value, CliError> {
    let (kind, project, _, _, _) =
        kind_id(db, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    if kind != "view" {
        return Err(CliError::input("view_id 必须指向 view"));
    }
    let project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let latest: i64 = db
        .query_row(
            "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='view_document' AND parent_id=?1",
            [view_id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let latest_id: String = db
        .query_row(
            "SELECT id FROM records WHERE kind='view_document' AND parent_id=?1 AND revision=?2",
            params![view_id, latest],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let (_, document) = record_document(db, &latest_id)?;
    let root = serde_json::to_value(&document.root)
        .map_err(|e| CliError::internal(format!("ViewDocument root 无效：{e}")))?;
    let component = find_component(&root, instance_id)
        .ok_or_else(|| CliError::missing("View 中不存在该组件"))?;
    let reference = component
        .get("model_ref")
        .ok_or_else(|| CliError::input("该组件没有科学引用"))?;
    if reference.get("kind").and_then(Value::as_str) != Some("operation_use") {
        return Err(CliError::input("step detail 仅支持 operation_use 组件"));
    }
    let block_version_id = reference
        .get("block_version_id")
        .and_then(Value::as_str)
        .ok_or_else(|| CliError::input("operation_use 缺少 block_version_id"))?;
    let step_ref = reference
        .get("step")
        .ok_or_else(|| CliError::input("operation_use 缺少 step"))?;
    let protocol_version_id = step_ref
        .get("protocol_version_id")
        .and_then(Value::as_str)
        .ok_or_else(|| CliError::input("operation_use 缺少 protocol_version_id"))?;
    let step_path = step_ref
        .get("step_path")
        .and_then(Value::as_str)
        .ok_or_else(|| CliError::input("operation_use 缺少 step_path"))?;
    let protocol = get(db, protocol_version_id)?;
    if protocol["kind"] != "protocol_version"
        || protocol["project_id"].as_str() != Some(project_id.as_str())
    {
        return Err(CliError::input("ProtocolVersion 必须属于当前项目"));
    }
    let root_block = protocol["data"]["root_block_version_id"]
        .as_str()
        .ok_or_else(|| CliError::input("ProtocolVersion 没有根 BlockVersion"))?;
    let bindings: Vec<RootParameterValue> =
        serde_json::from_value(protocol["data"]["root_bindings"].clone()).unwrap_or_default();
    let preview = block_preview(db, root_block, &bindings)?;
    let step = preview["steps"]
        .as_array()
        .and_then(|steps| {
            steps
                .iter()
                .find(|step| step["path"].as_str() == Some(step_path))
        })
        .cloned()
        .ok_or_else(|| CliError::missing(format!("ProtocolPreview 中没有步骤 {step_path}")))?;
    if step["source_block_version_id"].as_str() != Some(block_version_id) {
        return Err(CliError::input(format!(
            "步骤 {step_path} 的 BlockVersion 与引用中的 {block_version_id} 不一致"
        )));
    }
    let scope: ProtocolScope = serde_json::from_value(
        step_ref
            .get("scope")
            .cloned()
            .unwrap_or_else(|| json!({"kind":"unscoped"})),
    )
    .map_err(|e| CliError::input(format!("operation_use scope 无效：{e}")))?;
    let design_version_id = view_design_version(&document)?;
    let context = DbComponentContext::with_design(db, &project_id, design_version_id.as_deref());
    if !context.scope_exists(protocol_version_id, &scope) {
        return Err(CliError::input(format!(
            "步骤 {step_path} 的科学 Scope 不属于当前 View 的 DesignVersion"
        )));
    }
    Ok(json!({
        "view_id": view_id,
        "instance_id": instance_id,
        "protocol_version_id": protocol_version_id,
        "block_version_id": block_version_id,
        "scope": step_ref.get("scope").cloned().unwrap_or(Value::Null),
        "step_path": step_path,
        "step": step,
        "protocol_ready": preview["ready"],
        "resolved_parameters": preview["resolved_parameters"],
        "unresolved_parameters": preview["unresolved_parameters"]
    }))
}

pub(crate) fn create_view_proposal_tx(
    tx: &Transaction<'_>,
    view_id: &str,
    changeset: &Value,
    source: &str,
) -> Result<Value, CliError> {
    let (kind, project, _, _, _) =
        kind_id(tx, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    if kind != "view" {
        return Err(CliError::input("view_id 必须指向 view"));
    }
    let project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let typed: ViewChangeSet = serde_json::from_value(changeset.clone())
        .map_err(|e| CliError::input(format!("提案 changeset 无效：{e}")))?;
    let preview = view_proposal(tx, view_id, &typed)?;
    let proposal_id = make_id();
    let data = json!({
        "proposal_id": proposal_id,
        "view_id": view_id,
        "document_id": preview["document_id"],
        "expected_revision": preview["expected_revision"],
        "changeset": changeset,
        "preview": preview,
        "source": source.trim(),
        "status": "pending",
        "accepted_revision": Value::Null
    });
    insert(
        tx,
        "view_proposal",
        Some(&project_id),
        Some(view_id),
        1,
        &data,
    )
}

pub(crate) fn list_view_proposals(db: &Connection, view_id: &str) -> Result<Value, CliError> {
    let (kind, project, _, _, _) =
        kind_id(db, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    if kind != "view" {
        return Err(CliError::input("view_id 必须指向 view"));
    }
    let project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let records = list(db, Some("view_proposal"), Some(&project_id))?;
    let mut latest: std::collections::BTreeMap<String, Value> = std::collections::BTreeMap::new();
    for record in records.into_iter().filter(|r| r["parent_id"] == view_id) {
        let proposal_id = record["data"]["proposal_id"]
            .as_str()
            .unwrap_or_default()
            .to_string();
        let replace = latest
            .get(&proposal_id)
            .map(|old| record["revision"].as_i64() > old["revision"].as_i64())
            .unwrap_or(true);
        if replace {
            latest.insert(proposal_id, record);
        }
    }
    Ok(json!({"records": latest.into_values().collect::<Vec<_>>()}))
}

pub(crate) fn latest_view_proposal(
    tx: &Transaction<'_>,
    view_id: &str,
    proposal_id: &str,
) -> Result<Value, CliError> {
    let (_, project, _, _, _) =
        kind_id(tx, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    let project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let records = list(tx, Some("view_proposal"), Some(&project_id))?;
    records
        .into_iter()
        .filter(|record| {
            record["parent_id"] == view_id
                && record["data"]["proposal_id"].as_str() == Some(proposal_id)
        })
        .max_by_key(|record| record["revision"].as_i64().unwrap_or(0))
        .ok_or_else(|| CliError::missing("提案不存在"))
}

pub(crate) fn accept_view_proposal_tx(
    tx: &Transaction<'_>,
    view_id: &str,
    proposal_id: &str,
) -> Result<Value, CliError> {
    let proposal = latest_view_proposal(tx, view_id, proposal_id)?;
    if proposal["data"]["status"].as_str() != Some("pending") {
        return Err(CliError::conflict("提案不是待审阅状态"));
    }
    let changeset = proposal["data"]["changeset"].clone();
    let version = revise_view_records(tx, view_id, &json!({"changeset": changeset}))?;
    let mut data = proposal["data"].clone();
    data["status"] = json!("accepted");
    data["accepted_revision"] = version["version"]["revision"].clone();
    let existing_revision = proposal["revision"].as_i64().unwrap_or(1);
    let status_record = insert(
        tx,
        "view_proposal",
        proposal["project_id"].as_str(),
        Some(view_id),
        existing_revision + 1,
        &data,
    )?;
    Ok(json!({"proposal": status_record, "version": version["version"]}))
}

pub(crate) fn reject_view_proposal_tx(
    tx: &Transaction<'_>,
    view_id: &str,
    proposal_id: &str,
) -> Result<Value, CliError> {
    let proposal = latest_view_proposal(tx, view_id, proposal_id)?;
    if proposal["data"]["status"].as_str() != Some("pending") {
        return Err(CliError::conflict("提案不是待审阅状态"));
    }
    let mut data = proposal["data"].clone();
    data["status"] = json!("rejected");
    let revision = proposal["revision"].as_i64().unwrap_or(1) + 1;
    let record = insert(
        tx,
        "view_proposal",
        proposal["project_id"].as_str(),
        Some(view_id),
        revision,
        &data,
    )?;
    Ok(json!({"proposal": record}))
}

pub(crate) fn view_selection_context(
    db: &Connection,
    view_id: &str,
    instance_id: &str,
) -> Result<Value, CliError> {
    let (kind, project, _, _, _) =
        kind_id(db, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    if kind != "view" {
        return Err(CliError::input("view_id 必须指向 view"));
    }
    let _project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let latest: i64 = db
        .query_row(
            "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='view_document' AND parent_id=?1",
            [view_id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let latest_id: String = db
        .query_row(
            "SELECT id FROM records WHERE kind='view_document' AND parent_id=?1 AND revision=?2",
            params![view_id, latest],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let (_, document) = record_document(db, &latest_id)?;
    let root = serde_json::to_value(&document.root)
        .map_err(|e| CliError::internal(format!("ViewDocument root 无效：{e}")))?;
    let component = find_component(&root, instance_id)
        .ok_or_else(|| CliError::missing("View 中不存在该组件"))?;
    let renderer_key = format!(
        "{}@{}",
        component["definition"]["id"].as_str().unwrap_or("unknown"),
        component["definition"]["version"].as_u64().unwrap_or(0)
    );
    let (step_status, step_detail, step_error) =
        if component["model_ref"]["kind"].as_str() == Some("operation_use") {
            match view_step_detail(db, view_id, instance_id) {
                Ok(detail) => ("resolved", Some(detail), Value::Null),
                Err(error) => (
                    "error",
                    None,
                    json!({"code": error.code, "message": error.message}),
                ),
            }
        } else {
            ("not_applicable", None, Value::Null)
        };
    let protocol_ready = step_detail
        .as_ref()
        .and_then(|detail| detail.get("protocol_ready"))
        .cloned()
        .unwrap_or(Value::Null);
    let unresolved_parameters = step_detail
        .as_ref()
        .and_then(|detail| detail.get("unresolved_parameters"))
        .cloned()
        .unwrap_or(Value::Null);
    Ok(json!({
        "view_id": view_id,
        "document_id": document.document_id,
        "revision": document.revision,
        "instance_id": instance_id,
        "renderer_key": renderer_key,
        "component": component,
        "protocol_ready": protocol_ready,
        "unresolved_parameters": unresolved_parameters,
        "step_detail": {
            "status": step_status,
            "detail": step_detail,
            "error": step_error
        }
    }))
}

pub(crate) fn find_slot_position(value: &Value, instance_id: &str) -> Option<(String, String, usize)> {
    if let Some(slots) = value.get("slots").and_then(Value::as_object) {
        for (slot, children) in slots {
            if let Some(children) = children.as_array() {
                for (index, child) in children.iter().enumerate() {
                    if child["instance_id"].as_str() == Some(instance_id) {
                        return Some((
                            value["instance_id"]
                                .as_str()
                                .unwrap_or_default()
                                .to_string(),
                            slot.clone(),
                            index,
                        ));
                    }
                    if let Some(found) = find_slot_position(child, instance_id) {
                        return Some(found);
                    }
                }
            }
        }
    }
    if let Some(nodes) = value
        .get("workflow")
        .and_then(|workflow| workflow.get("nodes"))
        .and_then(Value::as_array)
    {
        for node in nodes {
            if let Some(component) = node.get("component") {
                if let Some(found) = find_slot_position(component, instance_id) {
                    return Some(found);
                }
            }
        }
    }
    None
}

pub(crate) fn operation_changes(
    changeset: &ViewChangeSet,
    before: &Value,
    after: &Value,
    before_bindings: &[Value],
    after_bindings: &[Value],
) -> Vec<Value> {
    let operations = serde_json::to_value(&changeset.operations).unwrap_or_else(|_| json!([]));
    operations
        .as_array()
        .map(|operations| {
            operations
                .iter()
                .map(|operation| {
                    let kind = operation["kind"].as_str().unwrap_or("unknown");
                    let instance_id = operation["instance_id"].as_str().unwrap_or("");
                    match kind {
                        "update_props" => json!({
                            "kind": kind,
                            "instance_id": instance_id,
                            "property": operation["property"],
                            "before": find_component(before, instance_id)
                                .and_then(|c| c["props"].get(operation["property"].as_str()?))
                                .cloned()
                                .unwrap_or(Value::Null),
                            "after": find_component(after, instance_id)
                                .and_then(|c| c["props"].get(operation["property"].as_str()?))
                                .cloned()
                                .unwrap_or(Value::Null)
                        }),
                        "add_slot_child" => json!({
                            "kind": kind,
                            "instance_id": operation["child"]["instance_id"],
                            "parent_instance_id": operation["parent_instance_id"],
                            "slot_id": operation["slot_id"],
                            "position": find_slot_position(after, operation["child"]["instance_id"].as_str().unwrap_or("")),
                            "before": Value::Null,
                            "after": find_component(after, operation["child"]["instance_id"].as_str().unwrap_or(""))
                        }),
                        "remove_instance" => json!({
                            "kind": kind,
                            "instance_id": instance_id,
                            "before": find_component(before, instance_id),
                            "after": Value::Null
                        }),
                        "move_slot_child" => json!({
                            "kind": kind,
                            "instance_id": instance_id,
                            "before": find_slot_position(before, instance_id),
                            "after": find_slot_position(after, instance_id)
                        }),
                        "set_visual" => json!({
                            "kind": kind,
                            "instance_id": instance_id,
                            "before": find_component(before, instance_id).map(|c| c["visual"].clone()),
                            "after": find_component(after, instance_id).map(|c| c["visual"].clone())
                        }),
                        "connect" | "disconnect" => json!({
                            "kind": kind,
                            "workflow_instance_id": operation["workflow_instance_id"],
                            "edge": operation["edge"]
                        }),
                        "add_binding" | "replace_binding" => {
                            let binding_id = operation["binding"]["binding_id"].as_str().unwrap_or("");
                            json!({
                                "kind": kind,
                                "binding_id": binding_id,
                                "before": before_bindings.iter().find(|b| b["binding_id"].as_str() == Some(binding_id)),
                                "after": after_bindings.iter().find(|b| b["binding_id"].as_str() == Some(binding_id))
                            })
                        }
                        "remove_binding" => json!({
                            "kind": kind,
                            "binding_id": operation["binding_id"],
                            "before": before_bindings.iter().find(|b| b["binding_id"] == operation["binding_id"]),
                            "after": Value::Null
                        }),
                        _ => json!({"kind": kind, "operation": operation}),
                    }
                })
                .collect()
        })
        .unwrap_or_default()
}

pub(crate) fn view_proposal(
    db: &Connection,
    view_id: &str,
    changeset: &ViewChangeSet,
) -> Result<Value, CliError> {
    let (kind, project, _, _, _) =
        kind_id(db, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    if kind != "view" {
        return Err(CliError::input("view_id 必须指向 view"));
    }
    let project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let latest: i64 = db
        .query_row(
            "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='view_document' AND parent_id=?1",
            [view_id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let latest_id: String = db
        .query_row(
            "SELECT id FROM records WHERE kind='view_document' AND parent_id=?1 AND revision=?2",
            params![view_id, latest],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let (_, document) = record_document(db, &latest_id)?;
    let registry = load_component_registry(db)?;
    let result = document
        .apply_changeset(changeset, &registry)
        .map_err(semantic_error_with_conflict)?;
    let design_version_id = view_design_version(&result.document)?;
    result
        .document
        .validate_with_context_mode(
            &registry,
            &DbComponentContext::with_design(db, &project_id, design_version_id.as_deref()),
            ContextValidationMode::AllowUnresolved,
        )
        .map_err(semantic_error)?;
    let before = serde_json::to_value(&document.root)
        .map_err(|e| CliError::internal(format!("ViewDocument root 无效：{e}")))?;
    let after = serde_json::to_value(&result.document.root)
        .map_err(|e| CliError::internal(format!("ViewDocument root 无效：{e}")))?;
    let before_bindings = document
        .bindings
        .iter()
        .filter_map(|binding| serde_json::to_value(binding).ok())
        .collect::<Vec<_>>();
    let after_bindings = result
        .document
        .bindings
        .iter()
        .filter_map(|binding| serde_json::to_value(binding).ok())
        .collect::<Vec<_>>();
    let operation_changes = operation_changes(
        changeset,
        &before,
        &after,
        &before_bindings,
        &after_bindings,
    );
    let no_op = !operation_changes.is_empty()
        && operation_changes.iter().all(|operation| {
            let before = operation.get("before");
            let after = operation.get("after");
            before.is_some() && after.is_some() && before == after
        });
    let changes = result
        .changed_instances
        .iter()
        .map(|instance| {
            json!({
                "instance_id": instance,
                "before": find_component(&before, instance).cloned().unwrap_or(Value::Null),
                "after": find_component(&after, instance).cloned().unwrap_or(Value::Null)
            })
        })
        .collect::<Vec<_>>();
    Ok(json!({
        "valid": true,
        "view_id": view_id,
        "document_id": document.document_id,
        "expected_revision": changeset.expected_revision,
        "new_revision": result.document.revision,
        "changed_instances": result.changed_instances,
        "affected_bindings": result.affected_bindings,
        "requires_confirmation": result.requires_confirmation,
        "diff_scope": "net",
        "no_op": no_op,
        "operation_changes": operation_changes,
        "changes": changes
    }))
}

pub(crate) fn project_view_records(db: &Connection, data: &Value) -> Result<Value, CliError> {
    let i: ViewProject = serde_json::from_value(data.clone())
        .map_err(|e| CliError::input(format!("View 投影参数无效：{e}")))?;
    let protocol = get(db, &i.protocol_version_id)?;
    if protocol["kind"] != "protocol_version"
        || protocol["project_id"].as_str() != Some(i.project_id.as_str())
    {
        return Err(CliError::input("ProtocolVersion 必须属于当前项目"));
    }
    let root = protocol["data"]["root_block_version_id"]
        .as_str()
        .ok_or_else(|| CliError::input("Protocol 没有根 BlockVersion"))?;
    let bindings: Vec<RootParameterValue> =
        serde_json::from_value(protocol["data"]["root_bindings"].clone()).unwrap_or_default();
    let preview = block_preview(db, root, &bindings)?;
    let steps = preview["steps"].as_array().cloned().unwrap_or_default();
    let mut nodes = Vec::new();
    let mut edges = Vec::new();
    let mut previous: Option<String> = None;
    for (index, step) in steps.iter().enumerate() {
        let node_id = format!("step-{}", index + 1);
        let Some(path) = step["path"].as_str() else {
            continue;
        };
        let Some(block_version_id) = step["source_block_version_id"].as_str() else {
            continue;
        };
        let component = ComponentInstance {
            instance_id: node_id.clone(),
            definition: ComponentDefinitionRef::new("operation", 1),
            model_ref: Some(ModelRef::OperationUse {
                block_version_id: block_version_id.into(),
                step: asterlab_core::semantic_component::QualifiedStepRef {
                    protocol_version_id: i.protocol_version_id.clone(),
                    scope: i.scope.clone(),
                    step_path: path.into(),
                },
            }),
            props: json!({"label": step["title"].as_str().unwrap_or("Step")}),
            slots: Default::default(),
            workflow: None,
            visual: None,
        };
        nodes.push(WorkflowNode {
            node_id: node_id.clone(),
            component: Box::new(component),
        });
        if let Some(previous) = previous.replace(node_id.clone()) {
            edges.push(WorkflowEdge {
                from: previous,
                to: node_id,
            });
        }
    }
    let document = ViewDocument {
        schema_version: asterlab_core::component_framework::COMPONENT_FRAMEWORK_SCHEMA_VERSION,
        document_id: i
            .document_id
            .unwrap_or_else(|| format!("view-{}", i.protocol_version_id)),
        revision: 1,
        root: ComponentInstance {
            instance_id: "experiment_main".into(),
            definition: ComponentDefinitionRef::new("workflow", 1),
            model_ref: Some(ModelRef::ProtocolVersion {
                id: i.protocol_version_id.clone(),
            }),
            props: json!({}),
            slots: Default::default(),
            workflow: Some(WorkflowGraph { nodes, edges }),
            visual: None,
        },
        bindings: vec![],
    };
    document
        .validate_with_context_mode(
            &load_component_registry(db)?,
            &DbComponentContext::new(db, &i.project_id),
            ContextValidationMode::AllowUnresolved,
        )
        .map_err(semantic_error)?;
    Ok(json!({"document":document}))
}


pub(crate) fn ensure_design_workflow(
    db: &mut Connection,
    design_id: &str,
) -> Result<Value, CliError> {
    let design = get(db, design_id)?;
    if design["kind"] != "design" {
        return Err(CliError::input("design_id 必须指向实验设计"));
    }
    let project = design["project_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("实验设计缺少项目归属"))?
        .to_owned();
    for view in list(db, Some("view"), Some(&project))? {
        let Some(view_id) = view["id"].as_str() else { continue };
        let Ok(document) = get_view(db, view_id) else { continue };
        if document["latest_version"]["data"]["document_id"].as_str() == Some(&format!("design-{design_id}-workflow")) {
            return Ok(json!({"view": view, "latest_version": document["latest_version"]}));
        }
    }
    let assays = design["data"]["assays"].as_array().cloned().unwrap_or_default();
    let mut nodes = Vec::new();
    let mut edges = Vec::new();
    let mut schema_version = 1;
    for (assay_index, assay) in assays.iter().enumerate() {
        let Some(protocol_version_id) = assay["protocol_version_id"].as_str().filter(|id| !id.is_empty()) else {
            continue;
        };
        let Some(assay_id) = assay["id"].as_str() else { continue };
        let projected = project_view_records(
            db,
            &json!({
                "project_id": project,
                "protocol_version_id": protocol_version_id,
                "scope": {"kind": "assay", "assay_id": assay_id}
            }),
        )?;
        let document = &projected["document"];
        schema_version = document["schema_version"].as_i64().unwrap_or(1);
        let node_items = document["root"]["workflow"]["nodes"].as_array().cloned().unwrap_or_default();
        let edge_items = document["root"]["workflow"]["edges"].as_array().cloned().unwrap_or_default();
        let prefix = format!("a{}-", assay_index + 1);
        let id_map: std::collections::HashMap<String, String> = node_items.iter().filter_map(|node| {
            let old = node["node_id"].as_str()?.to_owned();
            Some((old.clone(), format!("{prefix}{old}")))
        }).collect();
        for node in node_items {
            let Some(old) = node["node_id"].as_str().map(str::to_owned) else { continue };
            let Some(new_id) = id_map.get(&old) else { continue };
            let mut next = node;
            next["node_id"] = json!(new_id);
            if let Some(component) = next.get_mut("component") {
                component["instance_id"] = json!(new_id);
            }
            nodes.push(next);
        }
        for edge in edge_items {
            let Some(from) = edge["from"].as_str() else { continue };
            let Some(to) = edge["to"].as_str() else { continue };
            let (Some(from), Some(to)) = (id_map.get(from), id_map.get(to)) else { continue };
            edges.push(json!({"from": from, "to": to}));
        }
    }
    if nodes.is_empty() {
        return Err(CliError::input("当前设计没有可投影的 Protocol 步骤"));
    }
    let title = format!("{} Workflow", design["data"]["title"].as_str().unwrap_or("Experiment"));
    let document = json!({
        "schema_version": schema_version,
        "document_id": format!("design-{design_id}-workflow"),
        "revision": 1,
        "root": {
            "instance_id": "experiment_main",
            "definition": {"id": "workflow", "version": 1},
            "model_ref": null,
            "props": {},
            "slots": {},
            "workflow": {"nodes": nodes, "edges": edges},
            "visual": null
        },
        "bindings": []
    });
    let tx = db.unchecked_transaction().map_err(|error| CliError::internal(error.to_string()))?;
    let result = create_view_records(
        &tx,
        &project,
        &json!({"project_id": project, "title": title, "document": document}),
    )?;
    tx.commit().map_err(|error| CliError::internal(error.to_string()))?;
    Ok(result)
}

pub(crate) fn find_component<'a>(value: &'a Value, instance_id: &str) -> Option<&'a Value> {
    if value.get("instance_id").and_then(Value::as_str) == Some(instance_id) {
        return Some(value);
    }
    if let Some(slots) = value.get("slots").and_then(Value::as_object) {
        for children in slots.values().filter_map(Value::as_array) {
            for child in children {
                if let Some(found) = find_component(child, instance_id) {
                    return Some(found);
                }
            }
        }
    }
    if let Some(nodes) = value
        .get("workflow")
        .and_then(|workflow| workflow.get("nodes"))
        .and_then(Value::as_array)
    {
        for node in nodes {
            if let Some(component) = node.get("component") {
                if let Some(found) = find_component(component, instance_id) {
                    return Some(found);
                }
            }
        }
    }
    None
}

pub(crate) fn find_planned_container(value: &Value, instance_id: &str) -> Option<(String, String)> {
    let component = find_component(value, instance_id)?;
    let model = component.get("model_ref")?;
    if model.get("kind").and_then(Value::as_str) == Some("planned_container") {
        return Some((
            model.get("design_version_id")?.as_str()?.to_owned(),
            model.get("container_id")?.as_str()?.to_owned(),
        ));
    }
    None
}

pub(crate) fn view_data_preview(db: &Connection, data: &Value) -> Result<Value, CliError> {
    let i: ViewDataPreview = serde_json::from_value(data.clone())
        .map_err(|e| CliError::input(format!("View 数据预览参数无效：{e}")))?;
    let view = get_view(db, &i.view_id)?;
    let document = &view["latest_version"]["data"];
    let Some((design_version_id, container_id)) =
        find_planned_container(&document["root"], &i.instance_id)
    else {
        return Err(CliError::input(
            "ViewDocument 中没有引用 PlannedContainer 的 WellPlate 组件",
        ));
    };
    let design = get(db, &design_version_id)?;
    if design["kind"] != "design_version" {
        return Err(CliError::input("PlannedContainer 引用的不是 DesignVersion"));
    }
    let plan_value = &design["data"]["snapshot"]["container_plan"];
    let plan: ContainerPlan = serde_json::from_value(if plan_value.is_null() {
        json!({})
    } else {
        plan_value.clone()
    })
    .map_err(|e| CliError::internal(format!("DesignVersion container_plan 无效：{e}")))?;
    let registry = load_labware_registry(db)?;
    let mut plate = project_well_plate(&plan, &registry, &container_id)
        .map_err(|issues| CliError::input(issues.join("；")))?;
    let has_target_positions = !i.position_ids.is_empty();
    if has_target_positions {
        let known = plate
            .positions
            .iter()
            .map(|position| position.position_id.clone())
            .collect::<HashSet<_>>();
        let missing = i
            .position_ids
            .iter()
            .filter(|position| !known.contains(*position))
            .cloned()
            .collect::<Vec<_>>();
        if !missing.is_empty() {
            return Err(CliError::input(format!(
                "position_ids 不属于该计划容器：{}",
                missing.join("、")
            )));
        }
        plate
            .positions
            .retain(|position| i.position_ids.iter().any(|id| id == &position.position_id));
    }
    if let Some(calculator_id) = &i.calculator_instance_id {
        let calculator = find_component(&document["root"], calculator_id)
            .ok_or_else(|| CliError::input("calculator_instance_id 不存在"))?;
        if calculator["definition"]["id"].as_str() != Some("tool.calculator") {
            return Err(CliError::input(
                "calculator_instance_id 必须指向 tool.calculator",
            ));
        }
        if calculator["props"]["tool_ref"].as_str() != Some("tool.reagent_total@1") {
            return Err(CliError::input(
                "calculator tool_ref 必须是 tool.reagent_total@1",
            ));
        }
    }
    let volume_per_well = i.volume_per_well;
    let volume_unit = i
        .volume_unit
        .as_deref()
        .and_then(asterlab_core::component_data::normalize_volume_unit);
    if i.volume_unit.is_some() && volume_unit.is_none() {
        return Err(CliError::input("volume_unit 不是允许的体积单位"));
    }
    let mut calculation = calculate_reagent(&plate, volume_per_well, volume_unit, i.extra_ratio);
    if !has_target_positions {
        calculation.state = "unresolved".into();
        calculation.total_volume = None;
        calculation.unresolved.push("position_ids".into());
    }
    if let (true, Some(volume), Some(unit), Some(ratio)) = (
        has_target_positions,
        volume_per_well,
        volume_unit,
        i.extra_ratio,
    ) {
        let invocation = run_reagent_tool(&plate, volume, unit, ratio).map_err(|issues| {
            CliError::input(
                issues
                    .into_iter()
                    .map(|issue| format!("{}: {}", issue.code, issue.message))
                    .collect::<Vec<_>>()
                    .join("；"),
            )
        })?;
        calculation.total_volume = invocation.outputs["total_volume"].as_f64();
        calculation.unit = invocation.outputs["unit"].as_str().map(str::to_owned);
        calculation.tool_action = Some(format!(
            "{}@{}",
            invocation.action_id, invocation.action_version
        ));
    }
    let table = reagent_table(&calculation);
    Ok(json!({
        "plate": plate,
        "calculation": calculation,
        "table": table
    }))
}

pub(crate) fn view_design_version(document: &ViewDocument) -> Result<Option<String>, CliError> {
    let mut found = std::collections::BTreeSet::new();
    pub(crate) fn walk(component: &ComponentInstance, found: &mut std::collections::BTreeSet<String>) {
        if let Some(ModelRef::PlannedContainer {
            design_version_id, ..
        }) = &component.model_ref
        {
            found.insert(design_version_id.clone());
        }
        for child in component.slots.values().flatten() {
            walk(child, found);
        }
        if let Some(workflow) = &component.workflow {
            for node in &workflow.nodes {
                walk(&node.component, found);
            }
        }
    }
    walk(&document.root, &mut found);
    match found.len() {
        0 => Ok(None),
        1 => Ok(found.into_iter().next()),
        _ => Err(CliError::input(
            "同一个 ViewDocument 的 PlannedContainer 必须来自同一个 DesignVersion",
        )),
    }
}

pub(crate) fn create_view_records(
    tx: &Transaction<'_>,
    project_id: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let mut input = data.clone();
    input["project_id"] = json!(project_id);
    let i: ViewCreate = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("View 数据结构无效：{e}")))?;
    ensure_project(tx, &i.project_id)?;
    check_text(&i.title, "视图名称", 160)?;
    if i.document.revision != 1 {
        return Err(CliError::input("新建 ViewDocument revision 必须为 1"));
    }
    let registry = load_component_registry(tx)?;
    let design_version_id = view_design_version(&i.document)?;
    i.document
        .validate_with_context_mode(
            &registry,
            &DbComponentContext::with_design(tx, &i.project_id, design_version_id.as_deref()),
            ContextValidationMode::AllowUnresolved,
        )
        .map_err(semantic_error)?;
    let view = insert(
        tx,
        "view",
        Some(&i.project_id),
        Some(&i.project_id),
        1,
        &json!({"title":i.title.trim()}),
    )?;
    let view_id = view["id"].as_str().unwrap_or_default();
    let version = insert(
        tx,
        "view_document",
        Some(&i.project_id),
        Some(view_id),
        1,
        &serde_json::to_value(&i.document).map_err(|e| CliError::internal(e.to_string()))?,
    )?;
    Ok(json!({"view":view,"version":version}))
}

pub(crate) fn revise_view_records(
    tx: &Transaction<'_>,
    view_id: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let mut input = data.clone();
    input["view_id"] = json!(view_id);
    let i: ViewRevise = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("View 修订数据结构无效：{e}")))?;
    let (kind, project, _, _, _) =
        kind_id(tx, view_id)?.ok_or_else(|| CliError::missing("View 不存在"))?;
    if kind != "view" {
        return Err(CliError::input("view_id 必须指向 view"));
    }
    let project_id = project.ok_or_else(|| CliError::internal("View 缺少项目归属"))?;
    let latest: i64 = tx
        .query_row(
            "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='view_document' AND parent_id=?1",
            [view_id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let latest_id: String = tx
        .query_row(
            "SELECT id FROM records WHERE kind='view_document' AND parent_id=?1 AND revision=?2",
            params![view_id, latest],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let (_, document) = record_document(tx, &latest_id)?;
    let registry = load_component_registry(tx)?;
    let result = document
        .apply_changeset(&i.changeset, &registry)
        .map_err(semantic_error_with_conflict)?;
    let design_version_id = view_design_version(&result.document)?;
    result
        .document
        .validate_with_context_mode(
            &registry,
            &DbComponentContext::with_design(tx, &project_id, design_version_id.as_deref()),
            ContextValidationMode::AllowUnresolved,
        )
        .map_err(semantic_error)?;
    let version = insert(
        tx,
        "view_document",
        Some(&project_id),
        Some(view_id),
        latest + 1,
        &serde_json::to_value(&result.document).map_err(|e| CliError::internal(e.to_string()))?,
    )?;
    Ok(
        json!({"version":version,"changes":result.changed_instances,"affected_bindings":result.affected_bindings,"requires_confirmation":result.requires_confirmation}),
    )
}

pub(crate) fn record_document(db: &Connection, id: &str) -> Result<(Value, ViewDocument), CliError> {
    let record = get(db, id)?;
    if record["kind"] != "view_document" {
        return Err(CliError::input("id 必须指向 view_document"));
    }
    let document: ViewDocument = serde_json::from_value(record["data"].clone())
        .map_err(|e| CliError::internal(format!("ViewDocument 数据无效：{e}")))?;
    Ok((record, document))
}

pub(crate) fn create_catalog_review_records(
    tx: &Transaction<'_>,
    project_id: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let mut input = data.clone();
    input["project_id"] = json!(project_id);
    let i: CatalogReviewCreate = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("审核记录数据结构无效：{e}")))?;
    ensure_project(tx, &i.project_id)?;
    if i.review.review_id.trim().is_empty()
        || i.review.reviewer.trim().is_empty()
        || i.review.evidence_id.trim().is_empty()
        || i.review.evidence_locator.trim().is_empty()
        || i.review.content_digest.trim().is_empty()
    {
        return Err(CliError::input("审核记录缺少必填来源字段"));
    }
    let record = insert(
        tx,
        "catalog_review",
        Some(&i.project_id),
        Some(&i.project_id),
        1,
        &serde_json::to_value(&i.review).map_err(|e| CliError::internal(e.to_string()))?,
    )?;
    Ok(json!({"record":record}))
}

pub(crate) fn import_catalog_operation_records(
    tx: &Transaction<'_>,
    project_id: &str,
    data: &Value,
) -> Result<Value, CliError> {
    let mut input = data.clone();
    input["project_id"] = json!(project_id);
    let i: CatalogOperationImport = serde_json::from_value(input)
        .map_err(|e| CliError::input(format!("Catalog 导入数据结构无效：{e}")))?;
    ensure_project(tx, &i.project_id)?;
    let review_record = get(tx, &i.review_id).map_err(|_| CliError::input("审核记录不存在"))?;
    if review_record["kind"] != "catalog_review" {
        return Err(CliError::input("review_id 必须指向 catalog_review"));
    }
    if review_record["project_id"].as_str() != Some(i.project_id.as_str()) {
        return Err(CliError::input("审核记录必须属于同一项目"));
    }
    let review: ReviewRecord = serde_json::from_value(review_record["data"].clone())
        .map_err(|e| CliError::input(format!("审核记录数据无效：{e}")))?;
    let summary = i
        .summary
        .unwrap_or_else(|| i.module.semantics.purpose.clone());
    if summary.chars().count() > 1000 {
        return Err(CliError::input("步骤块摘要不能超过 1000 个字符"));
    }
    let block = insert(
        tx,
        "protocol_block",
        Some(&i.project_id),
        Some(&i.project_id),
        1,
        &json!({"title":i.module.identity.title.trim(),"summary":summary}),
    )?;
    let block_id = block["id"].as_str().unwrap_or_default().to_owned();
    let binding = ModuleBinding {
        catalog: Some(i.module.identity.catalog.clone()),
        block_version: ProtocolBlockVersionRef {
            block_id: block_id.clone(),
            // The DB assigns the real version record id on insert below. The
            // compiler only needs a stable local placeholder; it is never
            // persisted as a BlockUse reference.
            version_id: format!("pending-{block_id}"),
        },
        origin: ModuleOrigin::Imported,
        based_on: None,
    };
    let compiled =
        compile_operation_module(&i.module, binding, CompilePolicy::default(), Some(&review))
            .map_err(semantic_error)?;
    let mut version_data = compiled.to_block_version_data(1).map_err(semantic_error)?;
    version_data.catalog_review = Some(review);
    validate_block_content(
        tx,
        &i.project_id,
        &version_data.parameter_slots,
        &version_data.steps,
    )?;
    let serialized =
        serde_json::to_value(version_data).map_err(|e| CliError::internal(e.to_string()))?;
    let version = insert(
        tx,
        "protocol_block_version",
        Some(&i.project_id),
        Some(&block_id),
        1,
        &serialized,
    )?;
    Ok(json!({"block":block,"version":version}))
}

pub(crate) fn semantic_error_with_conflict(
    issues: Vec<asterlab_core::semantic_component::SemanticIssue>,
) -> CliError {
    if issues.iter().any(|issue| issue.code == "revision_conflict") {
        return CliError::conflict(
            issues
                .into_iter()
                .map(|issue| format!("{}: {}", issue.code, issue.message))
                .collect::<Vec<_>>()
                .join("; "),
        );
    }
    semantic_error(issues)
}

pub(crate) fn semantic_error(issues: Vec<asterlab_core::semantic_component::SemanticIssue>) -> CliError {
    let message = issues
        .into_iter()
        .map(|issue| format!("{}: {}", issue.code, issue.message))
        .collect::<Vec<_>>()
        .join("; ");
    CliError::input(message)
}

pub(crate) fn ensure_run_protocol_ready(db: &Connection, run_id: &str) -> Result<(), CliError> {
    let (kind, _, _, _, raw) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("id 必须指向实验运行"));
    }
    let data: Value = serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    let unresolved: Vec<String> = data
        .get("protocol_manifest")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .chain(
            data.get("material_manifest")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .filter_map(|entry| entry.get("producer_protocol")),
        )
        .filter(|entry| entry.get("protocol_ready").and_then(Value::as_bool) == Some(false))
        .flat_map(|entry| {
            entry
                .get("unresolved_parameters")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
        })
        .filter(|parameter| parameter.get("required").and_then(Value::as_bool) == Some(true))
        .filter_map(|parameter| {
            parameter
                .get("path")
                .and_then(Value::as_str)
                .map(str::to_owned)
        })
        .collect();
    if !unresolved.is_empty() {
        return Err(CliError::input(format!("运行尚不能开始：Protocol 有必填参数待确认（{}）。请保存新的 Protocol 版本并重新创建运行。", unresolved.join("、"))));
    }
    Ok(())
}
