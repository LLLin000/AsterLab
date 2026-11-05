use asterlab_core::component_data::{
    calculate_reagent, project_well_plate, reagent_table, run_reagent_tool,
};
use asterlab_core::component_framework::{
    default_registry, ComponentBindingContext, ComponentDefinition, ComponentDefinitionRef,
    ComponentInstance, ComponentRegistry, ContextValidationMode, ModelPathResolution, ModelRef,
    ViewChangeSet, ViewDocument, WorkflowEdge, WorkflowGraph, WorkflowNode,
};
use asterlab_core::labware::{
    default_labware_registry, ContainerPlan, LabwareProfile, LabwareRegistry,
};
use asterlab_core::operation_module::{
    ModuleBinding, ModuleOrigin, OperationModule, ParameterBinding, ParameterValueType,
    ProtocolBlockVersionRef, ValueSource,
};
use asterlab_core::protocol_block::ReviewRecord;
use asterlab_core::protocol_block::{ParameterSlot, ProtocolBlockVersionData, ProtocolStep};
use asterlab_core::protocol_preview::{
    OperationStep, ProtocolPreview, ResolvedParameter, UnresolvedParameter,
};
use asterlab_core::semantic_catalog::{compile_operation_module, CompilePolicy};
use asterlab_core::semantic_changeset::ChangeSet;
use asterlab_core::semantic_component::ProtocolScope;
use asterlab_core::semantic_query::{assess_block_patch, inspect_block, search_modules};
use chrono::{DateTime, NaiveDate, Utc};
use rusqlite::{params, Connection, OptionalExtension, Transaction};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::{HashMap, HashSet},
    fs,
    io::{self, Read},
    path::Path,
};
use uuid::Uuid;

const INPUT_LIMIT: u64 = 2 * 1024 * 1024;

#[derive(Debug)]
pub(crate) struct CliError {
    pub(crate) code: &'static str,
    pub(crate) message: String,
    pub(crate) exit: i32,
}
impl CliError {
    fn input(message: impl Into<String>) -> Self {
        Self {
            code: "INVALID_INPUT",
            message: message.into(),
            exit: 2,
        }
    }
    fn missing(message: impl Into<String>) -> Self {
        Self {
            code: "NOT_FOUND",
            message: message.into(),
            exit: 3,
        }
    }
    fn conflict(message: impl Into<String>) -> Self {
        Self {
            code: "REVISION_CONFLICT",
            message: message.into(),
            exit: 4,
        }
    }
    fn internal(message: impl Into<String>) -> Self {
        Self {
            code: "INTERNAL",
            message: message.into(),
            exit: 5,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum GroupRole {
    Control,
    Treatment,
    Comparator,
    Reference,
    Other,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct GroupSpec {
    #[serde(default)]
    id: String,
    name: String,
    #[serde(default)]
    role: Option<GroupRole>,
    #[serde(default)]
    condition: Option<String>,
    #[serde(default)]
    independent_n: Option<u32>,
    #[serde(default)]
    technical_repeats: Option<u32>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum LegacySampleRelation {
    NewPerTimepoint,
    SameSampleRepeated,
    SplitFromAssay,
    SequentialAfterAssay,
    Unknown,
}
impl Default for LegacySampleRelation {
    fn default() -> Self {
        Self::Unknown
    }
}
fn is_unknown_legacy_relation(relation: &LegacySampleRelation) -> bool {
    matches!(relation, LegacySampleRelation::Unknown)
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
enum TimepointRelation {
    SameExperimentalUnitRepeated,
    SamePhysicalSampleRepeated,
    IndependentSamplePerTimepoint,
    Unknown,
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
enum PhysicalSlotsScope {
    PerTimepoint,
    WholeAssayUniquePositions,
    Unknown,
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
enum SourceRelation {
    SplitFromAssay,
    SequentialAfterAssay,
    NotApplicable,
    Unknown,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum EvidenceState {
    SourceRecord,
    Proposal,
    UserConfirmed,
    Calculated,
    AgentSuggestion,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum ConflictStatus {
    Unresolved,
    Resolved,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum RunRelationship {
    Independent,
    SameSource,
    Paired,
    Unknown,
}
impl RunRelationship {
    fn as_str(&self) -> &'static str {
        match self {
            Self::Independent => "independent",
            Self::SameSource => "same_source",
            Self::Paired => "paired",
            Self::Unknown => "unknown",
        }
    }
}
impl Default for RunRelationship {
    fn default() -> Self {
        Self::Unknown
    }
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct AssayPlan {
    id: String,
    title: String,
    #[serde(default)]
    endpoint: String,
    #[serde(default)]
    method: String,
    #[serde(default)]
    unit: String,
    #[serde(default)]
    timepoints: Vec<String>,
    #[serde(default)]
    timepoint_timing: Vec<TimepointTiming>,
    #[serde(default)]
    group_names: Vec<String>,
    #[serde(default)]
    group_ids: Vec<String>,
    #[serde(default)]
    time_anchor_event: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    sample_use_policy: Option<LegacySampleRelation>,
    #[serde(default)]
    timepoint_relation: Option<TimepointRelation>,
    #[serde(default)]
    container_type: Option<String>,
    #[serde(default)]
    container_format: Option<String>,
    #[serde(default)]
    protocol_version_id: Option<String>,
    #[serde(default)]
    controls: Vec<String>,
    #[serde(default)]
    control_group_ids: Vec<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct TimepointTiming {
    timepoint: String,
    #[serde(default)]
    offset_value: Option<f64>,
    unit: String,
    #[serde(default)]
    window_before: Option<f64>,
    #[serde(default)]
    window_after: Option<f64>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct SampleAllocation {
    assay_id: String,
    group_name: String,
    #[serde(default)]
    group_id: Option<String>,
    // Legacy input only. normalize_design migrates this before omitting it from output.
    #[serde(default, skip_serializing_if = "is_unknown_legacy_relation")]
    relation: LegacySampleRelation,
    #[serde(default)]
    timepoint_relation: Option<TimepointRelation>,
    #[serde(default)]
    source_relation: Option<SourceRelation>,
    #[serde(default)]
    source_assay_id: Option<String>,
    #[serde(default)]
    physical_slots_per_group: Option<u32>,
    #[serde(default)]
    physical_slots_scope: Option<PhysicalSlotsScope>,
    #[serde(default)]
    evidence_id: Option<String>,
    #[serde(default)]
    planned_output_id: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct PlannedOutput {
    id: String,
    title: String,
    protocol_version_id: String,
    #[serde(default)]
    root_bindings: Vec<RootParameterValue>,
    #[serde(default)]
    evidence_id: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct SourceEvidence {
    id: String,
    source_document: String,
    #[serde(default)]
    source_date: Option<String>,
    #[serde(default)]
    source_locator: Option<String>,
    claim: String,
    state: EvidenceState,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EvidenceConflict {
    id: String,
    topic: String,
    evidence_ids: Vec<String>,
    status: ConflictStatus,
    #[serde(default)]
    affected_assay_ids: Vec<String>,
    #[serde(default)]
    affects_container_estimate: bool,
    #[serde(default)]
    resolution: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct DesignFields {
    title: String,
    #[serde(default)]
    objective: String,
    #[serde(default)]
    experimental_unit: String,
    #[serde(default)]
    group_specs: Vec<GroupSpec>,
    #[serde(default)]
    timepoints: Vec<String>,
    #[serde(default)]
    primary_endpoint: String,
    #[serde(default)]
    endpoint_method: String,
    #[serde(default)]
    endpoint_unit: String,
    #[serde(default)]
    sample_size_rationale: String,
    #[serde(default)]
    randomization: String,
    #[serde(default)]
    blinding: String,
    #[serde(default)]
    inclusion_criteria: String,
    #[serde(default)]
    exclusion_criteria: String,
    #[serde(default)]
    analysis_plan: String,
    #[serde(default)]
    controls: String,
    #[serde(default)]
    assays: Vec<AssayPlan>,
    #[serde(default)]
    sample_allocations: Vec<SampleAllocation>,
    #[serde(default)]
    planned_outputs: Vec<PlannedOutput>,
    #[serde(default)]
    container_plan: ContainerPlan,
    #[serde(default)]
    source_evidence: Vec<SourceEvidence>,
    #[serde(default)]
    evidence_conflicts: Vec<EvidenceConflict>,
    #[serde(default)]
    planned_run_count: Option<u32>,
    #[serde(default)]
    planned_run_relationship: RunRelationship,
    // Kept for compatibility with the browser form and older imported records.
    #[serde(default)]
    groups: String,
    #[serde(default)]
    hypothesis: String,
    #[serde(default)]
    replicate_plan: String,
    #[serde(default)]
    independent_variables: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct DesignCreate {
    project_id: String,
    design: DesignFields,
}
#[derive(Debug, Default, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct DesignChanges {
    title: Option<String>,
    objective: Option<String>,
    experimental_unit: Option<String>,
    group_specs: Option<Vec<GroupSpec>>,
    timepoints: Option<Vec<String>>,
    primary_endpoint: Option<String>,
    endpoint_method: Option<String>,
    endpoint_unit: Option<String>,
    sample_size_rationale: Option<String>,
    randomization: Option<String>,
    blinding: Option<String>,
    inclusion_criteria: Option<String>,
    exclusion_criteria: Option<String>,
    analysis_plan: Option<String>,
    controls: Option<String>,
    assays: Option<Vec<AssayPlan>>,
    sample_allocations: Option<Vec<SampleAllocation>>,
    planned_outputs: Option<Vec<PlannedOutput>>,
    container_plan: Option<ContainerPlan>,
    source_evidence: Option<Vec<SourceEvidence>>,
    evidence_conflicts: Option<Vec<EvidenceConflict>>,
    planned_run_count: Option<u32>,
    planned_run_relationship: Option<RunRelationship>,
    groups: Option<String>,
    hypothesis: Option<String>,
    replicate_plan: Option<String>,
    independent_variables: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct DesignRevise {
    id: String,
    expected_revision: i64,
    changes: DesignChanges,
}
#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProjectFilter {
    project_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct EmptyInput {}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct IdInput {
    id: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProjectCreate {
    name: String,
    #[serde(default)]
    description: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProjectRevise {
    id: String,
    expected_revision: i64,
    #[serde(default)]
    name: Option<String>,
    #[serde(default)]
    description: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RunCreate {
    design_id: String,
    title: String,
    #[serde(default)]
    scheduled_date: Option<String>,
    #[serde(default)]
    protocol_version_id: Option<String>,
    #[serde(default)]
    repeat_of_run_id: Option<String>,
    #[serde(default)]
    repeat_relationship: RunRelationship,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RunRevise {
    id: String,
    expected_revision: i64,
    #[serde(default)]
    title: Option<String>,
    #[serde(default)]
    scheduled_date: Option<Option<String>>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RunPatch {
    #[serde(default)]
    title: Option<String>,
    #[serde(default)]
    scheduled_date: Option<Option<String>>,
    #[serde(default)]
    status: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct TaskCreate {
    project_id: String,
    #[serde(default)]
    run_id: Option<String>,
    title: String,
    scheduled_date: String,
    #[serde(default = "planned")]
    status: String,
    #[serde(default)]
    notes: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct StatusUpdate {
    id: String,
    expected_revision: i64,
    status: String,
}
fn planned() -> String {
    "planned".into()
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProtocolCreate {
    #[serde(default)]
    project_id: Option<String>,
    title: String,
    #[serde(default)]
    body: Option<String>,
    #[serde(default)]
    root_block_version_id: Option<String>,
    #[serde(default)]
    root_bindings: Vec<RootParameterValue>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProtocolRevise {
    protocol_id: String,
    expected_version: i64,
    #[serde(default)]
    body: Option<String>,
    #[serde(default)]
    root_block_version_id: Option<Value>,
    #[serde(default)]
    root_bindings: Option<Vec<RootParameterValue>>,
}
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ProtocolBlockCreate {
    project_id: String,
    title: String,
    #[serde(default)]
    summary: String,
    #[serde(default)]
    parameter_slots: Vec<ParameterSlot>,
    steps: Vec<ProtocolStep>,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct CatalogOperationImport {
    project_id: String,
    module: OperationModule,
    review_id: String,
    #[serde(default)]
    summary: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SemanticInspectInput {
    block_version_id: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SemanticSearchInput {
    #[serde(default)]
    query: String,
    #[serde(default)]
    project_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SemanticPatchInput {
    changeset: ChangeSet,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ViewCreate {
    project_id: String,
    title: String,
    document: ViewDocument,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct LabwareDefine {
    profile: LabwareProfile,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ComponentDefine {
    definition: ComponentDefinition,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ViewProject {
    project_id: String,
    protocol_version_id: String,
    scope: ProtocolScope,
    #[serde(default)]
    document_id: Option<String>,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ViewDataPreview {
    view_id: String,
    instance_id: String,
    #[serde(default)]
    calculator_instance_id: Option<String>,
    #[serde(default)]
    volume_per_well: Option<f64>,
    #[serde(default)]
    extra_ratio: Option<f64>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ViewGet {
    id: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ViewRevise {
    view_id: String,
    changeset: ViewChangeSet,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct CatalogReviewCreate {
    project_id: String,
    review: ReviewRecord,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ProtocolBlockRevise {
    block_id: String,
    expected_version: i64,
    #[serde(default)]
    summary: Option<String>,
    #[serde(default)]
    parameter_slots: Vec<ParameterSlot>,
    steps: Vec<ProtocolStep>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProtocolBlockList {
    #[serde(default)]
    project_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProtocolBlockGet {
    id: String,
}
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct RootParameterValue {
    slot: String,
    value: Value,
    #[serde(default)]
    unit: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProtocolBlockPreview {
    block_version_id: String,
    #[serde(default)]
    bindings: Vec<RootParameterValue>,
}
#[derive(Debug, Clone)]
struct BoundValue {
    value: Value,
    unit: Option<String>,
    provenance: Vec<Value>,
}
#[derive(Default)]
struct ComposeAccumulator {
    steps: Vec<OperationStep>,
    resolved_parameters: Vec<ResolvedParameter>,
    unresolved_parameters: Vec<UnresolvedParameter>,
    block_version_ids: Vec<String>,
    module_contracts: Vec<Value>,
    unresolved_keys: HashSet<String>,
}
fn block_version_data(
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
fn validate_parameter_value(
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
fn validate_parameter_slots(
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
fn validate_block_content(
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
    for step in steps {
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
                ..
            } => {
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
fn validate_root_bindings(
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
fn add_unresolved(acc: &mut ComposeAccumulator, path: &str, slot: &ParameterSlot) {
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
fn expand_block(
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
            ProtocolStep::Text {
                use_id,
                title,
                instruction,
                parameter_refs,
                visualization,
                time_scope,
            } => {
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
                acc.steps.push(OperationStep {
                    path: step_path,
                    use_id: use_id.clone(),
                    title: title.clone(),
                    instruction: instruction.clone(),
                    parameters,
                    unresolved_parameter_refs: unresolved_refs,
                    source_block_version_id: version_id.to_owned(),
                    visualization,
                    time_scope: *time_scope,
                });
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
fn block_preview(
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
fn protocol_manifest_entry_with_bindings(
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
        protocol_manifest.push(protocol_manifest_entry(
            db,
            version_id,
            project_id,
            Value::Null,
            json!("未分配到检测的全局 Protocol"),
        )?);
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

fn labware_profile_snapshot(db: &Connection, design_data: &Value) -> Result<Vec<Value>, CliError> {
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

pub(crate) fn create_run_in_transaction(
    tx: &Transaction<'_>,
    design_id: &str,
    title: &str,
    scheduled_date: Option<&str>,
    global_protocol_version_id: Option<&str>,
    repeat_of_run_id: Option<&str>,
    repeat_relationship: &str,
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
    let design_data: Value =
        serde_json::from_str(&design_raw).map_err(|e| CliError::internal(e.to_string()))?;
    let (protocol_manifest, material_manifest) =
        build_run_manifests(tx, &design_data, &project, global_protocol_version_id)?;
    let labware_profiles = labware_profile_snapshot(tx, &design_data)?;
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
    let run_index: i64 = tx
        .query_row(
            "SELECT COUNT(*) + 1 FROM records WHERE kind='run' AND parent_id=?1",
            [design_id],
            |row| row.get(0),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    let data = json!({
        "title":title.trim(),
        "scheduled_date":scheduled_date.filter(|date| !date.trim().is_empty()),
        "protocol_version_id":global_protocol_version_id.filter(|id| !id.is_empty()),
        "protocol_manifest":protocol_manifest,
        "material_manifest":material_manifest,
        "container_plan":design_data["container_plan"].clone(),
        "labware_profiles":labware_profiles,
        "design_version_id":version,
        "design_revision":revision,
        "run_index":run_index,
        "repeat_of_run_id":repeat_of_run_id.filter(|id| !id.is_empty()),
        "repeat_relationship":repeat_relationship,
        "status":"planned"
    });
    insert(tx, "run", Some(&project), Some(design_id), 1, &data)
}

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
    ]
    .contains(&event.event_type.as_str())
    {
        return Err(CliError::input(
            "event_type 必须是 operation、material_use、measurement、deviation 或 observation",
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
    if event
        .step_status
        .as_deref()
        .is_some_and(|status| !["in_progress", "completed", "blocked", "skipped"].contains(&status))
    {
        return Err(CliError::input(
            "step_status 必须是 in_progress、completed、blocked 或 skipped",
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
                    && manifest["steps"].as_array().is_some_and(|steps| {
                        steps
                            .iter()
                            .any(|step| step["path"].as_str() == Some(step_path.as_str()))
                    })
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
    match event.event_type.as_str() {
        "material_use" if event.materials.is_empty() => {
            return Err(CliError::input("材料使用事件至少需要一项材料"));
        }
        "measurement" if event.measurements.is_empty() => {
            return Err(CliError::input("测量事件至少需要一项数值读数"));
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
    let data = json!({
        "event_type":event.event_type,
        "content":event.content,
        "occurred_at":event.occurred_at,
        "stage":event.stage,
        "timepoint":event.timepoint,
        "step_status":event.step_status,
        "protocol_step_path":event.protocol_step_path,
        "protocol_version_id":event.protocol_version_id,
        "assay_id":event.assay_id,
        "planned_output_id":event.planned_output_id,
        "sample_ids":event.sample_ids,
        "materials":event.materials.iter().map(|m|json!({"material":m.material,"planned_output_id":m.planned_output_id,"amount":m.amount,"unit":m.unit})).collect::<Vec<_>>(),
        "measurements":event.measurements.iter().map(|m|json!({"endpoint":m.endpoint,"value":m.value,"unit":m.unit,"sample_id":m.sample_id})).collect::<Vec<_>>(),
        "deviation":event.deviation
    });
    insert(
        tx,
        "execution_event",
        project.as_deref(),
        Some(run_id),
        1,
        &data,
    )
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

fn assay_requires_timepoint(snapshot: &Value, assay: &Value) -> bool {
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
fn apply_binding_changes(
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

struct DbComponentContext<'a> {
    db: &'a Connection,
    project_id: &'a str,
}

impl<'a> DbComponentContext<'a> {
    fn new(db: &'a Connection, project_id: &'a str) -> Self {
        Self { db, project_id }
    }

    fn scope_exists(&self, protocol_version_id: &str, scope: &ProtocolScope) -> bool {
        match scope {
            ProtocolScope::Unscoped => true,
            ProtocolScope::Assay { assay_id } => {
                list(self.db, Some("design_version"), Some(self.project_id))
                    .ok()
                    .into_iter()
                    .flatten()
                    .any(|version| {
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
                list(self.db, Some("design_version"), Some(self.project_id))
                    .ok()
                    .into_iter()
                    .flatten()
                    .any(|version| {
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

    fn owned_record(&self, id: &str, kind: &str) -> Option<Value> {
        let record = get(self.db, id).ok()?;
        if record["kind"] != kind || record["project_id"].as_str() != Some(self.project_id) {
            return None;
        }
        Some(record)
    }

    fn resolve_path(&self, value: &Value, path: &str) -> ModelPathResolution {
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

fn load_component_registry(db: &Connection) -> Result<ComponentRegistry, CliError> {
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
            props: json!({}),
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

fn find_component<'a>(value: &'a Value, instance_id: &str) -> Option<&'a Value> {
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

fn find_planned_container(value: &Value, instance_id: &str) -> Option<(String, String)> {
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
    let plate = project_well_plate(&plan, &registry, &container_id)
        .map_err(|issues| CliError::input(issues.join("；")))?;
    let mut volume_per_well = i.volume_per_well;
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
        let binding = document["bindings"]
            .as_array()
            .and_then(|bindings| {
                bindings.iter().find(|binding| {
                    binding["target"]["instance_id"].as_str() == Some(calculator_id.as_str())
                        && binding["target"]["port_id"].as_str() == Some("values")
                })
            })
            .ok_or_else(|| CliError::input("calculator 缺少 values 的 DataBinding"))?;
        let source_id = binding["source"]["instance_id"]
            .as_str()
            .ok_or_else(|| CliError::input("calculator binding source 必须是 ComponentPort"))?;
        let source = find_component(&document["root"], source_id)
            .ok_or_else(|| CliError::input("calculator binding source 组件不存在"))?;
        volume_per_well = source["props"]["rows"]
            .as_array()
            .and_then(|rows| rows.first())
            .and_then(|row| row.as_array())
            .and_then(|row| row.first())
            .and_then(Value::as_f64);
        if volume_per_well.is_none() {
            return Err(CliError::input("calculator binding 的表格单元格不是数值"));
        }
    }
    let mut calculation = calculate_reagent(&plate, volume_per_well, i.extra_ratio);
    if let (Some(volume), Some(ratio)) = (volume_per_well, i.extra_ratio) {
        let invocation = run_reagent_tool(&plate, volume, ratio).map_err(|issues| {
            CliError::input(
                issues
                    .into_iter()
                    .map(|issue| format!("{}: {}", issue.code, issue.message))
                    .collect::<Vec<_>>()
                    .join("；"),
            )
        })?;
        calculation.total_volume = invocation.outputs["total_volume"].as_f64();
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
    i.document
        .validate_with_context_mode(
            &registry,
            &DbComponentContext::new(tx, &i.project_id),
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
    result
        .document
        .validate_with_context_mode(
            &registry,
            &DbComponentContext::new(tx, &project_id),
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

fn record_document(db: &Connection, id: &str) -> Result<(Value, ViewDocument), CliError> {
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

fn semantic_error_with_conflict(
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

fn semantic_error(issues: Vec<asterlab_core::semantic_component::SemanticIssue>) -> CliError {
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
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct PlateCreate {
    run_id: String,
    name: String,
    format: String,
    rows: u32,
    columns: u32,
    #[serde(default)]
    assay_id: Option<String>,
    #[serde(default)]
    timepoint: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct WellSet {
    plate_id: String,
    well_id: String,
    #[serde(default)]
    expected_revision: Option<i64>,
    #[serde(default)]
    group_label: String,
    #[serde(default)]
    sample_id: String,
    #[serde(default)]
    note: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct NoteCreate {
    run_id: String,
    content: String,
    #[serde(default)]
    event_time: Option<String>,
    #[serde(default)]
    observation: String,
    #[serde(default)]
    result: String,
    #[serde(default)]
    deviation: String,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventMaterialUse {
    material: String,
    #[serde(default)]
    planned_output_id: Option<String>,
    #[serde(default)]
    amount: Option<f64>,
    #[serde(default)]
    unit: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventMeasurement {
    endpoint: String,
    value: f64,
    #[serde(default)]
    unit: Option<String>,
    #[serde(default)]
    sample_id: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct ExecutionEventCreate {
    run_id: String,
    event_type: String,
    content: String,
    #[serde(default)]
    occurred_at: Option<String>,
    #[serde(default)]
    stage: Option<String>,
    #[serde(default)]
    timepoint: Option<String>,
    #[serde(default)]
    step_status: Option<String>,
    #[serde(default)]
    protocol_step_path: Option<String>,
    #[serde(default)]
    protocol_version_id: Option<String>,
    #[serde(default)]
    assay_id: Option<String>,
    #[serde(default)]
    planned_output_id: Option<String>,
    #[serde(default)]
    sample_ids: Vec<String>,
    #[serde(default)]
    materials: Vec<EventMaterialUse>,
    #[serde(default)]
    measurements: Vec<EventMeasurement>,
    #[serde(default)]
    deviation: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct FormulaCreate {
    project_id: String,
    title: String,
    expression: String,
    #[serde(default)]
    variables: String,
    #[serde(default)]
    notes: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct FormulaRevise {
    id: String,
    expected_revision: i64,
    #[serde(default)]
    title: Option<String>,
    #[serde(default)]
    expression: Option<String>,
    #[serde(default)]
    variables: Option<String>,
    #[serde(default)]
    notes: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct TableCreate {
    project_id: String,
    name: String,
    columns: Vec<String>,
    rows: Vec<Vec<String>>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct TableRevise {
    id: String,
    expected_revision: i64,
    name: String,
    columns: Vec<String>,
    rows: Vec<Vec<String>>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct DilutionCalc {
    stock_concentration: f64,
    stock_unit: String,
    target_concentration: f64,
    target_unit: String,
    final_volume: f64,
    volume_unit: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SolutionCalc {
    concentration_mol_l: f64,
    final_volume: f64,
    volume_unit: String,
    molecular_weight_g_mol: f64,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct SerialDilutionCalc {
    initial_concentration: f64,
    concentration_unit: String,
    transfer_volume: f64,
    diluent_volume: f64,
    steps: u32,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ReactionComponent {
    name: String,
    volume_ul: f64,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ReactionVolumeCalc {
    target_volume_ul: f64,
    components: Vec<ReactionComponent>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct StudyImport {
    project: ProjectCreate,
    #[serde(default)]
    designs: Vec<DesignFields>,
    #[serde(default)]
    protocols: Vec<ProtocolBody>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct ProtocolBody {
    title: String,
    body: String,
}

fn stamp() -> String {
    Utc::now().to_rfc3339()
}
fn make_id() -> String {
    Uuid::new_v4().to_string()
}
fn check_text(value: &str, label: &str, max: usize) -> Result<(), CliError> {
    let s = value.trim();
    if s.is_empty() {
        return Err(CliError::input(format!("{label}不能为空")));
    }
    if s.chars().count() > max {
        return Err(CliError::input(format!("{label}不能超过 {max} 个字符")));
    }
    Ok(())
}
fn validate_groups(groups: &[GroupSpec]) -> Result<(), CliError> {
    let mut names = std::collections::HashSet::new();
    let mut ids = std::collections::HashSet::new();
    for g in groups {
        check_text(&g.id, "组别 ID", 80)?;
        check_text(&g.name, "组别名称", 120)?;
        if !ids.insert(g.id.as_str()) {
            return Err(CliError::input(format!("组别 ID 重复：{}", g.id)));
        }
        if !names.insert(g.name.trim().to_lowercase()) {
            return Err(CliError::input(format!("组别名称重复：{}", g.name)));
        }
        if g.independent_n == Some(0) || g.technical_repeats == Some(0) {
            return Err(CliError::input("n 和技术重复数只能是正整数或留空"));
        }
        if g.condition
            .as_ref()
            .is_some_and(|v| v.chars().count() > 500)
        {
            return Err(CliError::input("处理条件不能超过 500 个字符"));
        }
    }
    Ok(())
}

fn legacy_timepoint_relation(relation: &LegacySampleRelation) -> Option<TimepointRelation> {
    match relation {
        LegacySampleRelation::NewPerTimepoint => {
            Some(TimepointRelation::IndependentSamplePerTimepoint)
        }
        // The old label did not distinguish an experimental unit from a physical sample.
        // Preserve that uncertainty instead of silently choosing one interpretation.
        LegacySampleRelation::SameSampleRepeated => Some(TimepointRelation::Unknown),
        LegacySampleRelation::Unknown
        | LegacySampleRelation::SplitFromAssay
        | LegacySampleRelation::SequentialAfterAssay => None,
    }
}

fn legacy_source_relation(relation: &LegacySampleRelation) -> SourceRelation {
    match relation {
        LegacySampleRelation::SplitFromAssay => SourceRelation::SplitFromAssay,
        LegacySampleRelation::SequentialAfterAssay => SourceRelation::SequentialAfterAssay,
        _ => SourceRelation::Unknown,
    }
}

fn normalize_design(d: &mut DesignFields, previous: Option<&DesignFields>) {
    if d.group_specs.is_empty() && !d.groups.trim().is_empty() {
        d.group_specs = d
            .groups
            .lines()
            .filter(|name| !name.trim().is_empty())
            .map(|name| GroupSpec {
                id: String::new(),
                name: name.trim().to_owned(),
                role: None,
                condition: None,
                independent_n: None,
                technical_repeats: None,
            })
            .collect();
    }

    let mut used_ids = std::collections::HashSet::new();
    for group in &mut d.group_specs {
        if group.id.trim().is_empty() {
            let matches: Vec<&GroupSpec> = previous
                .into_iter()
                .flat_map(|old| old.group_specs.iter())
                .filter(|old| old.name == group.name && !old.id.trim().is_empty())
                .collect();
            if matches.len() == 1 && !used_ids.contains(matches[0].id.as_str()) {
                group.id = matches[0].id.clone();
            } else {
                group.id = make_id();
            }
        }
        used_ids.insert(group.id.clone());
    }

    let groups_by_id: std::collections::HashMap<String, String> = d
        .group_specs
        .iter()
        .map(|group| (group.id.clone(), group.name.clone()))
        .collect();
    let groups_by_name: std::collections::HashMap<String, String> = d
        .group_specs
        .iter()
        .map(|group| (group.name.clone(), group.id.clone()))
        .collect();
    for assay in &mut d.assays {
        if assay.timepoint_relation.is_none() {
            assay.timepoint_relation = Some(
                assay
                    .sample_use_policy
                    .as_ref()
                    .and_then(legacy_timepoint_relation)
                    .unwrap_or(TimepointRelation::Unknown),
            );
        }
        assay.sample_use_policy = None;
        if assay.group_ids.is_empty() {
            if let Some(ids) = assay
                .group_names
                .iter()
                .map(|name| groups_by_name.get(name).cloned())
                .collect::<Option<Vec<_>>>()
            {
                assay.group_ids = ids;
            }
        }
        if !assay.group_ids.is_empty() {
            assay.group_names = assay
                .group_ids
                .iter()
                .filter_map(|id| groups_by_id.get(id).cloned())
                .collect();
        }
    }
    let assay_timepoint_relations: std::collections::HashMap<String, TimepointRelation> = d
        .assays
        .iter()
        .filter_map(|assay| {
            assay
                .timepoint_relation
                .as_ref()
                .map(|relation| (assay.id.clone(), relation.clone()))
        })
        .collect();
    for allocation in &mut d.sample_allocations {
        if allocation.timepoint_relation.is_none() {
            allocation.timepoint_relation = Some(
                legacy_timepoint_relation(&allocation.relation)
                    .or_else(|| assay_timepoint_relations.get(&allocation.assay_id).cloned())
                    .unwrap_or(TimepointRelation::Unknown),
            );
        }
        if allocation.source_relation.is_none() {
            allocation.source_relation = Some(legacy_source_relation(&allocation.relation));
        }
        if allocation.group_id.is_none() {
            allocation.group_id = groups_by_name.get(&allocation.group_name).cloned();
        }
        if allocation.physical_slots_per_group.is_some()
            && allocation.physical_slots_scope.is_none()
        {
            allocation.physical_slots_scope = Some(PhysicalSlotsScope::Unknown);
        }
        if let Some(name) = allocation
            .group_id
            .as_ref()
            .and_then(|id| groups_by_id.get(id))
        {
            allocation.group_name = name.clone();
        }
        allocation.relation = LegacySampleRelation::Unknown;
    }
}

fn load_labware_registry(db: &Connection) -> Result<LabwareRegistry, CliError> {
    let mut registry = default_labware_registry();
    for record in list(db, Some("labware_profile"), None)? {
        let profile: LabwareProfile = serde_json::from_value(record["data"].clone())
            .map_err(|e| CliError::internal(format!("LabwareProfile 数据无效：{e}")))?;
        registry
            .register(profile)
            .map_err(|error| CliError::input(error))?;
    }
    Ok(registry)
}

pub(crate) fn normalize_validate_design_data(
    data: Value,
    previous: Option<&Value>,
) -> Result<Value, CliError> {
    normalize_validate_design_data_with_registry(data, previous, &default_labware_registry())
}

fn normalize_validate_design_data_with_registry(
    data: Value,
    previous: Option<&Value>,
    registry: &LabwareRegistry,
) -> Result<Value, CliError> {
    let mut design: DesignFields = serde_json::from_value(data)
        .map_err(|e| CliError::input(format!("设计数据结构无效：{e}")))?;
    let previous_design = previous
        .map(|value| serde_json::from_value::<DesignFields>(value.clone()))
        .transpose()
        .map_err(|e| CliError::internal(format!("现有设计数据无效：{e}")))?;
    normalize_design(&mut design, previous_design.as_ref());
    validate_design_with_registry(&design, registry)?;
    serde_json::to_value(design).map_err(|e| CliError::internal(e.to_string()))
}

pub(crate) fn run_workspace(db: &Connection, run_id: &str) -> Result<Value, CliError> {
    let run = get(db, run_id)?;
    if run["kind"] != "run" {
        return Err(CliError::input("指定记录不是实验运行"));
    }
    let design_id = run["parent_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("执行批次缺少所属设计"))?;
    let version_id = run["data"]["design_version_id"]
        .as_str()
        .filter(|value| !value.is_empty())
        .ok_or_else(|| CliError::input("执行批次没有冻结设计版本；不能建立 RunWorkspace"))?;
    let version = get(db, version_id)?;
    if version["kind"] != "design_version" {
        return Err(CliError::internal("冻结设计版本记录类型无效"));
    }
    if version["parent_id"].as_str() != Some(design_id) {
        return Err(CliError::input("冻结设计版本不属于本执行批次的设计"));
    }
    if version["project_id"].as_str() != run["project_id"].as_str() {
        return Err(CliError::input("冻结设计版本与执行批次不属于同一项目"));
    }
    let revision = version["data"]["design_revision"]
        .as_i64()
        .ok_or_else(|| CliError::internal("冻结设计版本号无效"))?;
    let snapshot = &version["data"]["snapshot"];
    let project = run["project_id"].as_str();

    let mut events = Vec::new();
    for record in list(db, Some("execution_event"), project)? {
        if record["parent_id"].as_str() != Some(run_id) {
            continue;
        }
        let mut data = record["data"].clone();
        data["id"] = record["id"].clone();
        events.push(data);
    }
    let mut plates = Vec::new();
    for record in list(db, Some("plate"), project)? {
        if record["parent_id"].as_str() != Some(run_id) {
            continue;
        }
        let mut data = record["data"].clone();
        data["id"] = record["id"].clone();
        plates.push(data);
    }
    let plate_ids: HashSet<String> = plates
        .iter()
        .filter_map(|plate| plate["id"].as_str().map(str::to_owned))
        .collect();
    let mut wells_by_plate: HashMap<String, Vec<Value>> = HashMap::new();
    for record in list(db, Some("well"), project)? {
        let Some(plate_id) = record["parent_id"].as_str() else {
            continue;
        };
        if !plate_ids.contains(plate_id) {
            continue;
        }
        let mut data = record["data"].clone();
        if data["well_id"].as_str().is_none() {
            data["well_id"] = record["id"].clone();
        }
        wells_by_plate
            .entry(plate_id.to_owned())
            .or_default()
            .push(data);
    }
    for plate in &mut plates {
        let plate_id = plate["id"].as_str().unwrap_or_default().to_owned();
        plate["wells"] = json!(wells_by_plate.remove(&plate_id).unwrap_or_default());
    }

    let workspace = asterlab_core::run_workspace::build(
        run_id,
        design_id,
        version_id,
        revision,
        snapshot,
        &run["data"],
        &plates,
        &events,
    )
    .map_err(|error| CliError::internal(error.to_string()))?;
    serde_json::to_value(workspace).map_err(|error| CliError::internal(error.to_string()))
}

pub(crate) fn design_map(db: &Connection, design_id: &str) -> Result<Value, CliError> {
    let record = get(db, design_id)?;
    if record["kind"] != "design" {
        return Err(CliError::input("指定记录不是实验设计"));
    }
    let revision = record["revision"]
        .as_i64()
        .ok_or_else(|| CliError::internal("设计版本号无效"))?;
    let map = asterlab_core::design_map::build_current(design_id, revision, &record["data"])
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(map.to_flat_json())
}

fn validate_design(d: &DesignFields) -> Result<(), CliError> {
    validate_design_with_registry(d, &default_labware_registry())
}

fn validate_design_with_registry(
    d: &DesignFields,
    registry: &LabwareRegistry,
) -> Result<(), CliError> {
    check_text(&d.title, "设计名称", 160)?;
    for (value, label, max) in [
        (&d.objective, "研究问题", 5000),
        (&d.experimental_unit, "实验单位", 500),
        (&d.primary_endpoint, "主要检测终点", 1000),
        (&d.endpoint_method, "检测方法", 1000),
        (&d.endpoint_unit, "终点单位", 100),
        (&d.sample_size_rationale, "样本量依据", 5000),
        (&d.randomization, "随机化方法", 2000),
        (&d.blinding, "盲法", 2000),
        (&d.inclusion_criteria, "纳入标准", 5000),
        (&d.exclusion_criteria, "排除标准", 5000),
        (&d.analysis_plan, "分析计划", 5000),
        (&d.controls, "对照设置", 2000),
    ] {
        if value.chars().count() > max {
            return Err(CliError::input(format!("{label}不能超过 {max} 个字符")));
        }
    }
    validate_groups(&d.group_specs)?;
    d.container_plan
        .validate(registry)
        .map_err(|issues| CliError::input(format!("容器计划无效：{}", issues.join("；"))))?;
    if d.group_specs.len() > 100 {
        return Err(CliError::input("组别数量最多 100 个"));
    }
    if d.timepoints.len() > 100 {
        return Err(CliError::input("时间点数量最多 100 个"));
    }
    for t in &d.timepoints {
        check_text(t, "时间点", 100)?;
    }
    if d.planned_run_count.is_some_and(|n| n == 0 || n > 1000) {
        return Err(CliError::input("计划执行轮次必须为 1–1000；未知时留空"));
    }
    if d.assays.len() > 100 || d.sample_allocations.len() > 10000 {
        return Err(CliError::input(
            "检测最多 100 项；样本分配关系最多 10000 条",
        ));
    }
    let mut assay_ids = std::collections::HashSet::new();
    let group_ids: std::collections::HashSet<&str> =
        d.group_specs.iter().map(|g| g.id.as_str()).collect();
    let group_names: std::collections::HashSet<&str> =
        d.group_specs.iter().map(|g| g.name.as_str()).collect();
    for assay in &d.assays {
        check_text(&assay.id, "检测 ID", 80)?;
        check_text(&assay.title, "检测名称", 160)?;
        if !assay_ids.insert(assay.id.as_str()) {
            return Err(CliError::input(format!("检测 ID 重复：{}", assay.id)));
        }
        for t in &assay.timepoints {
            check_text(t, "检测时间点", 100)?;
        }
        if assay.timepoint_timing.len() > 100 {
            return Err(CliError::input("单项检测的相对时间最多 100 项"));
        }
        if !assay.timepoint_timing.is_empty() {
            if assay.timepoint_timing.len() != assay.timepoints.len()
                || assay
                    .timepoint_timing
                    .iter()
                    .zip(&assay.timepoints)
                    .any(|(timing, label)| timing.timepoint != *label)
            {
                return Err(CliError::input(
                    "相对时间必须与测量时间点逐项对应且顺序一致",
                ));
            }
            if assay
                .time_anchor_event
                .as_deref()
                .is_none_or(|anchor| anchor.trim().is_empty())
            {
                return Err(CliError::input(
                    "填写相对时间前必须明确时间起算事件；未知时先不填写相对时间",
                ));
            }
            for timing in &assay.timepoint_timing {
                if !["min", "h", "day"].contains(&timing.unit.as_str()) {
                    return Err(CliError::input("相对时间单位只能是 min、h 或 day"));
                }
                if timing.offset_value.is_some_and(|v| !v.is_finite())
                    || timing
                        .window_before
                        .is_some_and(|v| !v.is_finite() || v < 0.0)
                    || timing
                        .window_after
                        .is_some_and(|v| !v.is_finite() || v < 0.0)
                {
                    return Err(CliError::input(
                        "相对时间偏移必须为有限数值；允许时间窗不能小于 0",
                    ));
                }
                if timing.offset_value.is_none()
                    && (timing.window_before.is_some() || timing.window_after.is_some())
                {
                    return Err(CliError::input("填写允许时间窗时必须同时填写相对时间偏移"));
                }
            }
        }
        let mut selected_group_ids = std::collections::HashSet::new();
        for group_id in &assay.group_ids {
            check_text(group_id, "检测适用组别 ID", 80)?;
            if !group_ids.contains(group_id.as_str())
                || !selected_group_ids.insert(group_id.as_str())
            {
                return Err(CliError::input("检测适用组别必须引用已定义的唯一组别 ID"));
            }
        }
        for group_name in &assay.group_names {
            check_text(group_name, "检测适用组别", 120)?;
            if !group_names.contains(group_name.as_str()) {
                return Err(CliError::input("检测适用组别必须引用已定义的组别"));
            }
        }
        let mut control_ids = std::collections::HashSet::new();
        for group_id in &assay.control_group_ids {
            check_text(group_id, "对照组 ID", 80)?;
            if !group_ids.contains(group_id.as_str()) || !control_ids.insert(group_id.as_str()) {
                return Err(CliError::input("检测对照必须引用已定义的唯一组别 ID"));
            }
        }
        if assay.timepoints.len() > 100 || assay.controls.len() > 100 {
            return Err(CliError::input("单项检测的时间点或对照最多 100 项"));
        }
    }
    let mut evidence_ids = std::collections::HashSet::new();
    for evidence in &d.source_evidence {
        check_text(&evidence.id, "来源证据 ID", 80)?;
        check_text(&evidence.source_document, "来源文档", 500)?;
        check_text(&evidence.claim, "来源记录内容", 2000)?;
        if let Some(locator) = &evidence.source_locator {
            check_text(locator, "来源定位", 500)?;
        }
        if !evidence_ids.insert(evidence.id.as_str()) {
            return Err(CliError::input(format!(
                "来源证据 ID 重复：{}",
                evidence.id
            )));
        }
    }
    for allocation in &d.sample_allocations {
        if !assay_ids.contains(allocation.assay_id.as_str()) {
            return Err(CliError::input(format!(
                "样本分配引用了不存在的检测：{}",
                allocation.assay_id
            )));
        }
        if !group_names.contains(allocation.group_name.as_str()) {
            return Err(CliError::input(format!(
                "样本分配引用了不存在的组别：{}",
                allocation.group_name
            )));
        }
        if allocation
            .group_id
            .as_deref()
            .is_none_or(|id| !group_ids.contains(id))
        {
            return Err(CliError::input("样本分配必须引用已定义的组别 ID"));
        }
        if allocation
            .physical_slots_per_group
            .is_some_and(|n| n == 0 || n > 100000)
        {
            return Err(CliError::input("每组容器位数量必须为 1–100000；未知时留空"));
        }
        let needs_source = matches!(
            allocation.source_relation.as_ref(),
            Some(SourceRelation::SplitFromAssay | SourceRelation::SequentialAfterAssay)
        );
        if needs_source && allocation.source_assay_id.is_none() {
            return Err(CliError::input("分样或顺序复用关系必须指定来源检测"));
        }
        if !needs_source && allocation.source_assay_id.is_some() {
            return Err(CliError::input("只有分样或顺序复用关系可以填写来源检测"));
        }
        if let Some(source) = &allocation.source_assay_id {
            if !assay_ids.contains(source.as_str()) || source == &allocation.assay_id {
                return Err(CliError::input("样本分配的来源检测必须是另一项已定义检测"));
            }
        }
        if allocation
            .evidence_id
            .as_ref()
            .is_some_and(|id| !evidence_ids.contains(id.as_str()))
        {
            return Err(CliError::input("样本分配引用的来源证据不存在"));
        }
        if allocation
            .planned_output_id
            .as_ref()
            .is_some_and(|id| !d.planned_outputs.iter().any(|output| &output.id == id))
        {
            return Err(CliError::input("样本分配引用了不存在的计划产物"));
        }
    }
    if d.planned_outputs.len() > 1000 {
        return Err(CliError::input("计划产物最多 1000 项"));
    }
    let mut output_ids = std::collections::HashSet::new();
    for output in &d.planned_outputs {
        check_text(&output.id, "计划产物 ID", 80)?;
        check_text(&output.title, "计划产物名称", 160)?;
        check_text(&output.protocol_version_id, "产物制备 Protocol 版本 ID", 80)?;
        if !output_ids.insert(output.id.as_str()) {
            return Err(CliError::input(format!("计划产物 ID 重复：{}", output.id)));
        }
        let mut slots = std::collections::HashSet::new();
        for binding in &output.root_bindings {
            check_text(&binding.slot, "产物参数槽位", 100)?;
            if !slots.insert(binding.slot.as_str()) {
                return Err(CliError::input(format!(
                    "计划产物参数重复：{}",
                    binding.slot
                )));
            }
        }
        if output
            .evidence_id
            .as_ref()
            .is_some_and(|id| !evidence_ids.contains(id.as_str()))
        {
            return Err(CliError::input("计划产物引用的来源证据不存在"));
        }
    }
    for conflict in &d.evidence_conflicts {
        check_text(&conflict.id, "冲突 ID", 80)?;
        check_text(&conflict.topic, "冲突主题", 500)?;
        let mut affected_assay_ids = std::collections::HashSet::new();
        for assay_id in &conflict.affected_assay_ids {
            check_text(assay_id, "冲突关联检测 ID", 80)?;
            if !assay_ids.contains(assay_id.as_str()) || !affected_assay_ids.insert(assay_id) {
                return Err(CliError::input(
                    "冲突关联检测必须是已定义的检测，且不能重复",
                ));
            }
        }
        if conflict.affects_container_estimate && conflict.affected_assay_ids.is_empty() {
            return Err(CliError::input("影响容器位估算的冲突必须关联至少一项检测"));
        }
        if conflict.evidence_ids.len() < 2
            || conflict
                .evidence_ids
                .iter()
                .any(|id| !evidence_ids.contains(id.as_str()))
        {
            return Err(CliError::input(
                "每项证据冲突必须引用至少两条已保存的来源证据",
            ));
        }
        if matches!(conflict.status, ConflictStatus::Resolved)
            && conflict
                .resolution
                .as_deref()
                .unwrap_or("")
                .trim()
                .is_empty()
        {
            return Err(CliError::input(
                "已确认的冲突解释必须写明依据；否则应保留为未解决",
            ));
        }
    }
    Ok(())
}
fn validate_date(value: &str, label: &str) -> Result<(), CliError> {
    if NaiveDate::parse_from_str(value, "%Y-%m-%d").is_err() {
        return Err(CliError::input(format!("{label}必须使用 YYYY-MM-DD 格式")));
    }
    Ok(())
}
fn open_db(path: &Path) -> Result<Connection, CliError> {
    fs::create_dir_all(path).map_err(|e| CliError::internal(e.to_string()))?;
    let db = Connection::open(path.join("labworkbench.sqlite"))
        .map_err(|e| CliError::internal(e.to_string()))?;
    db.pragma_update(None, "foreign_keys", "ON")
        .map_err(|e| CliError::internal(e.to_string()))?;
    db.execute_batch("CREATE TABLE IF NOT EXISTS records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT REFERENCES records(id),parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,revision INTEGER NOT NULL DEFAULT 1,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL); CREATE INDEX IF NOT EXISTS records_kind_parent ON records(kind,parent_id); CREATE INDEX IF NOT EXISTS records_project ON records(project_id);")
      .map_err(|e| CliError::internal(e.to_string()))?;
    Ok(db)
}
fn kind_id(
    db: &Connection,
    id: &str,
) -> Result<Option<(String, Option<String>, Option<String>, i64, String)>, CliError> {
    db.query_row(
        "SELECT kind,project_id,parent_id,revision,data_json FROM records WHERE id=?1",
        [id],
        |r| Ok((r.get(0)?, r.get(1)?, r.get(2)?, r.get(3)?, r.get(4)?)),
    )
    .optional()
    .map_err(|e| CliError::internal(e.to_string()))
}
fn has_kind(db: &Connection, id: &str, kind: &str) -> Result<bool, CliError> {
    Ok(kind_id(db, id)?.is_some_and(|r| r.0 == kind))
}
fn insert(
    tx: &Transaction<'_>,
    kind: &str,
    project: Option<&str>,
    parent: Option<&str>,
    revision: i64,
    data: &Value,
) -> Result<Value, CliError> {
    let id = make_id();
    let at = stamp();
    let raw = serde_json::to_string(data).map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute("INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",params![id,kind,project,parent,revision,raw,at]).map_err(|e|CliError::internal(e.to_string()))?;
    Ok(
        json!({"id":id,"kind":kind,"project_id":project,"parent_id":parent,"revision":revision,"data":data,"created_at":at,"updated_at":at}),
    )
}
fn get(db: &Connection, id: &str) -> Result<Value, CliError> {
    let (kind, project, parent, revision, raw) =
        kind_id(db, id)?.ok_or_else(|| CliError::missing(format!("记录不存在：{id}")))?;
    let data: Value = serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
    let (created, updated): (String, String) = db
        .query_row(
            "SELECT created_at,updated_at FROM records WHERE id=?1",
            [id],
            |r| Ok((r.get(0)?, r.get(1)?)),
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok(
        json!({"id":id,"kind":kind,"project_id":project,"parent_id":parent,"revision":revision,"data":data,"created_at":created,"updated_at":updated}),
    )
}
fn list(
    db: &Connection,
    kind: Option<&str>,
    project: Option<&str>,
) -> Result<Vec<Value>, CliError> {
    let mut stmt=db.prepare("SELECT id,kind,project_id,parent_id,revision,data_json,created_at,updated_at FROM records ORDER BY created_at DESC,rowid DESC").map_err(|e|CliError::internal(e.to_string()))?;
    let mapped=stmt.query_map([],|r|{let raw:String=r.get(5)?;let data:Value=serde_json::from_str(&raw).unwrap_or(Value::Null);Ok(json!({"id":r.get::<_,String>(0)?,"kind":r.get::<_,String>(1)?,"project_id":r.get::<_,Option<String>>(2)?,"parent_id":r.get::<_,Option<String>>(3)?,"revision":r.get::<_,i64>(4)?,"data":data,"created_at":r.get::<_,String>(6)?,"updated_at":r.get::<_,String>(7)?}))}).map_err(|e|CliError::internal(e.to_string()))?;
    let mut out = Vec::new();
    for row in mapped {
        let r = row.map_err(|e| CliError::internal(e.to_string()))?;
        if r["kind"] == "agent_draft" {
            continue;
        }
        if kind.is_some_and(|k| r["kind"] != k) {
            continue;
        }
        if project.is_some_and(|p| r["project_id"] != p) {
            continue;
        }
        out.push(r)
    }
    Ok(out)
}
fn parse_input<T: DeserializeOwned>(path: &str) -> Result<T, CliError> {
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
fn parse_args(args: &[String]) -> Result<&str, CliError> {
    if args.len() != 2 || args[0] != "--input" {
        return Err(CliError::input("命令格式：labworkbench <tool> --input <json-file|->；用 `labworkbench tools --json` 查看工具，用 `labworkbench schema <tool>` 查看字段定义"));
    }
    Ok(&args[1])
}
fn output(tool: &str, result: Value) {
    println!("{}", json!({"ok":true,"tool":tool,"result":result}));
}
fn ensure_project(db: &Connection, id: &str) -> Result<(), CliError> {
    if !has_kind(db, id, "project")? {
        return Err(CliError::missing(format!("项目不存在：{id}")));
    }
    Ok(())
}
fn update_data(
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
fn set_status(
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
fn concentration_factor(unit: &str) -> Result<f64, CliError> {
    match unit {
        "M" => Ok(1.0),
        "mM" => Ok(1e-3),
        "µM" => Ok(1e-6),
        "nM" => Ok(1e-9),
        _ => Err(CliError::input("浓度单位只能是 M、mM、µM、nM")),
    }
}
fn volume_factor(unit: &str) -> Result<f64, CliError> {
    match unit {
        "L" => Ok(1.0),
        "mL" => Ok(1e-3),
        "µL" => Ok(1e-6),
        _ => Err(CliError::input("体积单位只能是 L、mL、µL")),
    }
}
fn create_design_tx(
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
fn merge_design(base: &mut Value, c: &DesignChanges) {
    macro_rules! put {
        ($field:ident) => {
            if let Some(v) = &c.$field {
                base[stringify!($field)] = json!(v)
            }
        };
    }
    put!(title);
    put!(objective);
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

pub fn run(args: &[String], data_dir: &Path) -> i32 {
    if args.is_empty() || args[0] == "--help" || args[0] == "-h" {
        print_help();
        return 0;
    }
    if args[0] == "tools" {
        if args.len() == 2 && args[1] == "--json" {
            println!("{}", json!({"ok":true,"tools":tool_specs()}));
            return 0;
        }
        print_help();
        return 0;
    }
    if args[0] == "schema" {
        if args.len() != 2 {
            return fail(
                "schema",
                CliError::input("格式：labworkbench schema <tool>"),
            );
        }
        return match schema(&args[1]) {
            Some(s) => {
                println!(
                    "{}",
                    json!({"ok":true,"tool":args[1],"input_schema":s,"output_schema":output_schema(&args[1])})
                );
                0
            }
            None => fail(
                &args[1],
                CliError::missing("未知工具；请运行 `labworkbench tools --json`"),
            ),
        };
    }
    let tool = args[0].as_str();
    let path = match parse_args(&args[1..]) {
        Ok(v) => v,
        Err(e) => return fail(tool, e),
    };
    let result = dispatch(tool, path, data_dir);
    match result {
        Ok(v) => {
            output(tool, v);
            0
        }
        Err(e) => fail(tool, e),
    }
}
fn fail(tool: &str, e: CliError) -> i32 {
    println!(
        "{}",
        json!({"ok":false,"tool":tool,"error":{"code":e.code,"message":e.message}})
    );
    e.exit
}
fn print_help() {
    println!("Lab Workbench typed CLI\n\n  labworkbench tools --json\n  labworkbench schema <tool>\n  labworkbench <tool> --input <json-file|->\n\nAll tool inputs use a strict per-command schema; unknown fields are rejected. All output is one JSON envelope on stdout. Data directory follows LABWORKBENCH_DATA_DIR (default: ./data).");
}

fn require_trusted_writer(tool: &str) -> Result<(), CliError> {
    if std::env::var("LABWORKBENCH_TRUSTED_WRITER").ok().as_deref() == Some("1") {
        Ok(())
    } else {
        Err(CliError::input(format!(
            "{tool} 只允许受信任的本地写入端调用；请设置 LABWORKBENCH_TRUSTED_WRITER=1"
        )))
    }
}

fn dispatch(tool: &str, path: &str, data_dir: &Path) -> Result<Value, CliError> {
    if tool == "import.study.preview" {
        let i: StudyImport = parse_input(path)?;
        validate_import(&i)?;
        let mut designs = i.designs.clone();
        for design in &mut designs {
            normalize_design(design, None);
        }
        let warnings = designs
            .iter()
            .enumerate()
            .flat_map(|(ix, d)| {
                let mut w = Vec::new();
                if d.experimental_unit.trim().is_empty() {
                    w.push(format!("designs[{ix}].experimental_unit 未填写"))
                }
                if d.group_specs.is_empty() {
                    w.push(format!("designs[{ix}].group_specs 为空"))
                }
                if d.primary_endpoint.trim().is_empty() {
                    w.push(format!("designs[{ix}].primary_endpoint 未填写"))
                }
                w
            })
            .collect::<Vec<_>>();
        return Ok(
            json!({"valid":true,"writes":false,"project_count":1,"design_count":i.designs.len(),"protocol_count":i.protocols.len(),"warnings":warnings}),
        );
    }
    if tool == "calculator.dilution" {
        let i: DilutionCalc = parse_input(path)?;
        let c1 = concentration_factor(&i.stock_unit)?;
        let c2 = concentration_factor(&i.target_unit)?;
        let vf = volume_factor(&i.volume_unit)?;
        if !(i.stock_concentration.is_finite()
            && i.stock_concentration > 0.0
            && i.target_concentration.is_finite()
            && i.target_concentration >= 0.0
            && i.final_volume.is_finite()
            && i.final_volume > 0.0)
        {
            return Err(CliError::input(
                "储液浓度和终体积必须为正数，目标浓度不得小于 0",
            ));
        }
        if i.target_concentration * c2 > i.stock_concentration * c1 {
            return Err(CliError::input("目标浓度高于储液浓度，不能通过稀释配制"));
        }
        let v1_l =
            (i.target_concentration * c2) * (i.final_volume * vf) / (i.stock_concentration * c1);
        return Ok(
            json!({"stock_volume":v1_l/vf,"volume_unit":i.volume_unit,"diluent_volume":i.final_volume-v1_l/vf,"formula":"C1 × V1 = C2 × V2","assumption":"浓度与体积单位已按输入换算；未计死体积、回收率或纯度修正"}),
        );
    }
    if tool == "calculator.solution" {
        let i: SolutionCalc = parse_input(path)?;
        let vf = volume_factor(&i.volume_unit)?;
        if !(i.concentration_mol_l.is_finite()
            && i.concentration_mol_l > 0.0
            && i.final_volume.is_finite()
            && i.final_volume > 0.0
            && i.molecular_weight_g_mol.is_finite()
            && i.molecular_weight_g_mol > 0.0)
        {
            return Err(CliError::input("浓度、体积和分子量都必须是正数"));
        }
        let grams = i.concentration_mol_l * (i.final_volume * vf) * i.molecular_weight_g_mol;
        return Ok(
            json!({"mass_g":grams,"mass_mg":grams*1000.0,"formula":"m = C × V × MW","assumption":"体积按所选单位换算为 L；未修正纯度、水合物或盐型"}),
        );
    }
    if tool == "calculator.serial_dilution" {
        let i: SerialDilutionCalc = parse_input(path)?;
        concentration_factor(&i.concentration_unit)?;
        if !(i.initial_concentration.is_finite()
            && i.initial_concentration > 0.0
            && i.transfer_volume.is_finite()
            && i.transfer_volume > 0.0
            && i.diluent_volume.is_finite()
            && i.diluent_volume >= 0.0
            && i.steps > 0
            && i.steps <= 30)
        {
            return Err(CliError::input(
                "初始浓度和转移体积需大于 0；稀释液体积不得小于 0；级数为 1–30",
            ));
        }
        let ratio = i.transfer_volume / (i.transfer_volume + i.diluent_volume);
        let levels=(1..=i.steps).map(|step|json!({"step":step,"concentration":i.initial_concentration*ratio.powi(step as i32),"unit":i.concentration_unit})).collect::<Vec<_>>();
        return Ok(
            json!({"per_step_dilution_factor":1.0/ratio,"levels":levels,"formula":"C_next = C_previous × transfer_volume / (transfer_volume + diluent_volume)","assumption":"每步充分混匀；转移体积与稀释液体积单位相同；未计死体积或回收率"}),
        );
    }
    if tool == "calculator.reaction_volume" {
        let i: ReactionVolumeCalc = parse_input(path)?;
        if !(i.target_volume_ul.is_finite() && i.target_volume_ul > 0.0) {
            return Err(CliError::input("目标总体积必须大于 0 µL"));
        }
        if i.components.len() > 100 {
            return Err(CliError::input("组分最多 100 项"));
        }
        for c in &i.components {
            check_text(&c.name, "组分名称", 120)?;
            if !c.volume_ul.is_finite() || c.volume_ul < 0.0 {
                return Err(CliError::input("组分体积必须是大于等于 0 的有限数值"));
            }
        }
        let used: f64 = i.components.iter().map(|c| c.volume_ul).sum();
        if used > i.target_volume_ul {
            return Err(CliError::input("组分总体积超过目标总体积"));
        }
        return Ok(
            json!({"allocated_volume_ul":used,"remaining_volume_ul":i.target_volume_ul-used,"unit":"µL","assumption":"按输入组分体积求和；未判断配方科学适用性"}),
        );
    }
    let db = open_db(data_dir)?;
    match tool {
        "project.list" => {
            let _: EmptyInput = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("project"),None)?}))
        }
        "project.create" => {
            let i: ProjectCreate = parse_input(path)?;
            check_text(&i.name, "项目名称", 120)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "project",
                None,
                None,
                1,
                &json!({"name":i.name.trim(),"description":i.description.trim()}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "project.revise" => {
            let i: ProjectRevise = parse_input(path)?;
            if i.name.is_none() && i.description.is_none() {
                return Err(CliError::input("至少提供 name 或 description 中的一项"));
            }
            let (_, _, _, revision, raw) =
                kind_id(&db, &i.id)?.ok_or_else(|| CliError::missing("项目不存在"))?;
            if revision != i.expected_revision {
                return Err(CliError::conflict(format!(
                    "项目版本已从 v{} 更新为 v{revision}；请先重新读取。",
                    i.expected_revision
                )));
            }
            let mut data: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            if let Some(name) = i.name {
                check_text(&name, "项目名称", 120)?;
                data["name"] = json!(name.trim())
            }
            if let Some(description) = i.description {
                data["description"] = json!(description.trim())
            }
            Ok(json!({"record":update_data(&db,&i.id,i.expected_revision,"project",&data)?}))
        }
        "design.list" => {
            let i: ProjectFilter = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("design"),i.project_id.as_deref())?}))
        }
        "design.get" => {
            let i: IdInput = parse_input(path)?;
            let r = get(&db, &i.id)?;
            if r["kind"] != "design" {
                return Err(CliError::input("指定记录不是实验设计"));
            }
            let revision = r["revision"]
                .as_i64()
                .ok_or_else(|| CliError::internal("设计版本号无效"))?;
            let version_id: String = db
                .query_row(
                    "SELECT id FROM records WHERE kind='design_version' AND parent_id=?1 AND revision=?2",
                    params![i.id, revision],
                    |row| row.get(0),
                )
                .optional()
                .map_err(|e| CliError::internal(e.to_string()))?
                .ok_or_else(|| CliError::internal("当前设计版本快照缺失"))?;
            Ok(json!({"design":r,"version":get(&db,&version_id)?}))
        }
        "design.map" => {
            let i: IdInput = parse_input(path)?;
            Ok(design_map(&db, &i.id)?)
        }
        "design.create" => {
            let i: DesignCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let registry = load_labware_registry(&db)?;
            let r = create_design_tx(&tx, &i.project_id, &i.design, &registry)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(r)
        }
        "design.revise" => {
            let i: DesignRevise = parse_input(path)?;
            let changes =
                serde_json::to_value(&i.changes).map_err(|e| CliError::internal(e.to_string()))?;
            if !changes
                .as_object()
                .is_some_and(|o| o.values().any(|v| !v.is_null()))
            {
                return Err(CliError::input("changes 至少包含一个非 null 字段"));
            }
            let (kind, project, parent, revision, raw) = kind_id(&db, &i.id)?
                .ok_or_else(|| CliError::missing(format!("设计不存在：{}", i.id)))?;
            if kind != "design" {
                return Err(CliError::input("指定记录不是实验设计"));
            }
            if revision != i.expected_revision {
                return Err(CliError::conflict(format!(
                    "设计版本已从 v{} 更新为 v{}；先重新读取后再提交。",
                    i.expected_revision, revision
                )));
            }
            let previous: DesignFields = serde_json::from_str(&raw)
                .map_err(|e| CliError::internal(format!("现有设计无法读取：{e}")))?;
            let mut value: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            merge_design(&mut value, &i.changes);
            let mut d: DesignFields = serde_json::from_value(value)
                .map_err(|e| CliError::input(format!("修改后设计无效：{e}")))?;
            normalize_design(&mut d, Some(&previous));
            validate_design_with_registry(&d, &load_labware_registry(&db)?)?;
            let next = revision + 1;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let newdata =
                serde_json::to_string(&d).map_err(|e| CliError::internal(e.to_string()))?;
            tx.execute(
                "UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1",
                params![i.id, next, newdata, stamp()],
            )
            .map_err(|e| CliError::internal(e.to_string()))?;
            let snapshot = insert(
                &tx,
                "design_version",
                project.as_deref(),
                Some(&i.id),
                next,
                &json!({"design_id":i.id,"design_revision":next,"snapshot":d}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            let _ = parent;
            Ok(json!({"record":get(&db,&i.id)? ,"version":snapshot}))
        }
        "run.list" => {
            let i: ProjectFilter = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("run"),i.project_id.as_deref())?}))
        }
        "run.set_status" => {
            let i: StatusUpdate = parse_input(path)?;
            if i.status == "in_progress" {
                ensure_run_protocol_ready(&db, &i.id)?;
            }
            Ok(
                json!({"record":set_status(&db,i,"run",&["planned","in_progress","completed","blocked","cancelled"])?}),
            )
        }
        "run.revise" => {
            let i: RunRevise = parse_input(path)?;
            let mut changes = serde_json::Map::new();
            if let Some(title) = i.title {
                changes.insert("title".into(), json!(title));
            }
            if let Some(scheduled_date) = i.scheduled_date {
                changes.insert("scheduled_date".into(), json!(scheduled_date));
            }
            Ok(json!({"record":patch_run(&db,&i.id,i.expected_revision,&Value::Object(changes))?}))
        }
        "run.workspace" => {
            let i: IdInput = parse_input(path)?;
            Ok(run_workspace(&db, &i.id)?)
        }
        "run.create" => {
            let i: RunCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let record = create_run_in_transaction(
                &tx,
                &i.design_id,
                &i.title,
                i.scheduled_date.as_deref(),
                i.protocol_version_id.as_deref(),
                i.repeat_of_run_id.as_deref(),
                i.repeat_relationship.as_str(),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":record}))
        }
        "task.create" => {
            let i: TaskCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            check_text(&i.title, "任务名称", 160)?;
            check_text(&i.scheduled_date, "计划日期", 32)?;
            validate_date(&i.scheduled_date, "计划日期")?;
            if !["planned", "in_progress", "done", "blocked"].contains(&i.status.as_str()) {
                return Err(CliError::input(
                    "status 必须是 planned、in_progress、done、blocked 之一",
                ));
            }
            if let Some(id) = &i.run_id {
                let (run_kind, run_project, _, _, _) =
                    kind_id(&db, id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
                if run_kind != "run" {
                    return Err(CliError::input("run_id 必须指向实验运行"));
                }
                if run_project.as_deref() != Some(i.project_id.as_str()) {
                    return Err(CliError::input("run_id 与 project_id 不属于同一项目"));
                }
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "task",
                Some(&i.project_id),
                i.run_id.as_deref(),
                1,
                &json!({"title":i.title.trim(),"scheduled_date":i.scheduled_date,"status":i.status,"notes":i.notes}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "task.set_status" => {
            let i: StatusUpdate = parse_input(path)?;
            Ok(
                json!({"record":set_status(&db,i,"task",&["planned","in_progress","done","blocked"])?}),
            )
        }
        "protocol.list" => {
            let i: ProjectFilter = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("protocol"),i.project_id.as_deref())?}))
        }
        "protocol.block.list" => {
            let i: ProtocolBlockList = parse_input(path)?;
            Ok(json!({"records":list(&db,Some("protocol_block"),i.project_id.as_deref())?}))
        }
        "protocol.block.get" => {
            let i: ProtocolBlockGet = parse_input(path)?;
            let block = get(&db, &i.id)?;
            if block["kind"] != "protocol_block" {
                return Err(CliError::input("id 必须指向 protocol_block"));
            }
            let versions: Vec<Value> = list(
                &db,
                Some("protocol_block_version"),
                block["project_id"].as_str(),
            )?
            .into_iter()
            .filter(|v| v["parent_id"] == i.id)
            .collect();
            let latest = versions
                .iter()
                .max_by_key(|v| v["revision"].as_i64().unwrap_or(0))
                .cloned()
                .ok_or_else(|| CliError::internal("步骤块缺少 v1 版本"))?;
            Ok(json!({"block":block,"latest_version":latest,"versions":versions}))
        }
        "protocol.block.create" => {
            let i: ProtocolBlockCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_protocol_block_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "semantic.apply_binding_patch" => {
            require_trusted_writer(tool)?;
            let i: SemanticPatchInput = parse_input(path)?;
            if i.changeset.target.kind
                != asterlab_core::semantic_changeset::ChangeTargetKind::BlockVersion
            {
                return Err(CliError::input(
                    "semantic.apply_binding_patch 只接受 block_version 目标",
                ));
            }
            let target_id = i.changeset.target.object_id.clone();
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let (record, data) = block_version_data(&tx, &target_id)?;
            if record["revision"].as_i64() != Some(data.version) {
                return Err(CliError::conflict("BlockVersion 版本记录不一致"));
            }
            let block_id = data.block_id.clone();
            let latest: i64 = tx.query_row(
                "SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='protocol_block_version' AND parent_id=?1",
                [&block_id],
                |row| row.get(0),
            ).map_err(|e| CliError::internal(e.to_string()))?;
            if latest != record["revision"].as_i64().unwrap_or(0) {
                return Err(CliError::conflict(format!(
                    "目标不是最新 BlockVersion（最新 v{}）；请重新读取后再提交",
                    latest
                )));
            }
            if data.version != i.changeset.expected_version {
                return Err(CliError::conflict(format!(
                    "expected_version={} 与当前 BlockVersion v{} 不一致",
                    i.changeset.expected_version, data.version
                )));
            }
            let project = record["project_id"].as_str().unwrap_or_default();
            let (_, _, parent_project, _, _) =
                kind_id(&tx, &block_id)?.ok_or_else(|| CliError::missing("Block 不存在"))?;
            if parent_project.as_deref() != Some(project) {
                return Err(CliError::input("BlockVersion 与 Block 不属于同一项目"));
            }
            let (mut next, diff) = apply_binding_changes(&tx, &data, &i.changeset)?;
            validate_block_content(&tx, project, &next.parameter_slots, &next.steps)?;
            next.version = latest + 1;
            next.module_origin = next.module_contract.as_ref().map(|_| ModuleOrigin::Derived);
            next.module_contract_authority = next
                .module_contract
                .as_ref()
                .map(|_| asterlab_core::protocol_block::ModuleContractAuthority::SourceOnly);
            next.catalog_review = None;
            let serialized =
                serde_json::to_value(&next).map_err(|e| CliError::internal(e.to_string()))?;
            let version = insert(
                &tx,
                "protocol_block_version",
                Some(project),
                Some(&block_id),
                next.version,
                &serialized,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"version":version,"changes":diff,"requires_protocol_rebind":true}))
        }
        "labware.define" => {
            require_trusted_writer(tool)?;
            let i: LabwareDefine = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_labware_profile(&tx, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "component.list" => {
            let _: EmptyInput = parse_input(path)?;
            let registry = load_component_registry(&db)?;
            let definitions = registry
                .definitions()
                .map(|definition| {
                    json!({
                        "id": definition.id,
                        "version": definition.version,
                        "family": definition.family,
                        "title": definition.title,
                        "implemented": registry.is_implemented(&definition.id, definition.version)
                    })
                })
                .collect::<Vec<_>>();
            Ok(json!({"definitions":definitions}))
        }
        "component.define" => {
            require_trusted_writer(tool)?;
            let i: ComponentDefine = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_component_definition(&tx, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.data_preview" => {
            let i: ViewDataPreview = parse_input(path)?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            view_data_preview(&db, &payload)
        }
        "view.project" => {
            let i: ViewProject = parse_input(path)?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            project_view_records(&db, &payload)
        }
        "view.create" => {
            let i: ViewCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_view_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.get" => {
            let i: ViewGet = parse_input(path)?;
            get_view(&db, &i.id)
        }
        "view.revise" => {
            let i: ViewRevise = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let view_id = i.view_id.clone();
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = revise_view_records(&tx, &view_id, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "semantic.inspect" => {
            let i: SemanticInspectInput = parse_input(path)?;
            let (_, data) = block_version_data(&db, &i.block_version_id)?;
            serde_json::to_value(inspect_block(&i.block_version_id, &data))
                .map_err(|e| CliError::internal(e.to_string()))
        }
        "semantic.search" => {
            let i: SemanticSearchInput = parse_input(path)?;
            let mut modules = Vec::new();
            for version in list(&db, Some("protocol_block_version"), i.project_id.as_deref())? {
                if version["data"]["module_origin"].as_str() != Some("imported")
                    || version["data"]["module_contract_authority"].as_str() != Some("current")
                {
                    continue;
                }
                if let Some(module) = version["data"]
                    .get("module_contract")
                    .cloned()
                    .and_then(|value| serde_json::from_value(value).ok())
                {
                    let module: OperationModule = module;
                    modules.push(module);
                }
            }
            Ok(json!({"modules": search_modules(&modules, &i.query)}))
        }
        "semantic.validate" => {
            let i: SemanticPatchInput = parse_input(path)?;
            match i.changeset.validate() {
                Ok(()) => Ok(json!({"valid": true, "issues": []})),
                Err(issues) => Ok(json!({"valid": false, "issues": issues})),
            }
        }
        "semantic.propose_patch" => {
            let i: SemanticPatchInput = parse_input(path)?;
            i.changeset.validate().map_err(semantic_error)?;
            Ok(json!({"summary": i.changeset.summary(), "requires_confirmation": true}))
        }
        "semantic.assess_patch" => {
            let i: SemanticPatchInput = parse_input(path)?;
            if i.changeset.target.kind
                != asterlab_core::semantic_changeset::ChangeTargetKind::BlockVersion
            {
                return Err(CliError::input(
                    "semantic.assess_patch 只接受 block_version 目标",
                ));
            }
            let target_id = &i.changeset.target.object_id;
            let (record, data) = block_version_data(&db, target_id)?;
            let (next, _) = apply_binding_changes(&db, &data, &i.changeset)?;
            let project = record["project_id"].as_str().unwrap_or_default();
            validate_block_content(&db, project, &next.parameter_slots, &next.steps)?;
            let mut assessment =
                assess_block_patch(target_id, &data, &i.changeset).map_err(semantic_error)?;
            let mut refs = assessment.referencing_refs.clone();
            for version in list(&db, Some("protocol_version"), None)? {
                if version["data"]["root_block_version_id"].as_str() == Some(target_id.as_str()) {
                    refs.push(format!(
                        "protocol_version:{}",
                        version["id"].as_str().unwrap_or_default()
                    ));
                }
            }
            for run in list(&db, Some("run"), None)? {
                if run["data"]["protocol_manifest"]
                    .as_array()
                    .is_some_and(|entries| {
                        entries.iter().any(|entry| {
                            entry["block_version_ids"]
                                .as_array()
                                .is_some_and(|ids| ids.iter().any(|id| id == target_id))
                        })
                    })
                {
                    refs.push(format!("run:{}", run["id"].as_str().unwrap_or_default()));
                }
            }
            assessment.referencing_refs = refs.clone();
            assessment.would_require_rebind = refs;
            assessment.invalidated_refs = vec![];
            serde_json::to_value(assessment).map_err(|e| CliError::internal(e.to_string()))
        }
        "catalog.review.record" => {
            require_trusted_writer(tool)?;
            let i: CatalogReviewCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_catalog_review_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "catalog.operation.import" => {
            require_trusted_writer(tool)?;
            let i: CatalogOperationImport = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = import_catalog_operation_records(
                &tx,
                payload["project_id"].as_str().unwrap_or_default(),
                &payload,
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "protocol.block.revise" => {
            let i: ProtocolBlockRevise = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let block_id = i.block_id.clone();
            let payload = serde_json::to_value(i).map_err(|e| CliError::internal(e.to_string()))?;
            let result = revise_protocol_block_records(&tx, &block_id, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "protocol.block.preview" => {
            let i: ProtocolBlockPreview = parse_input(path)?;
            Ok(block_preview(&db, &i.block_version_id, &i.bindings)?)
        }
        "protocol.create" => {
            let i: ProtocolCreate = parse_input(path)?;
            check_text(&i.title, "Protocol 名称", 160)?;
            if let Some(p) = &i.project_id {
                ensure_project(&db, p)?
            }
            if i.body.is_none() && i.root_block_version_id.is_none() {
                return Err(CliError::input(
                    "Protocol 必须提供正文或 root_block_version_id",
                ));
            }
            if let Some(block_version_id) = &i.root_block_version_id {
                let (block_record, block_data) = block_version_data(&db, block_version_id)?;
                if i.project_id.as_deref() != block_record["project_id"].as_str() {
                    return Err(CliError::input("Protocol 与根步骤块必须属于同一项目"));
                }
                validate_root_bindings(&block_data.parameter_slots, &i.root_bindings)?;
            } else if !i.root_bindings.is_empty() {
                return Err(CliError::input(
                    "未指定 root_block_version_id 时不能提供 root_bindings",
                ));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let p = insert(
                &tx,
                "protocol",
                i.project_id.as_deref(),
                i.project_id.as_deref(),
                1,
                &json!({"title":i.title.trim()}),
            )?;
            let v = insert(
                &tx,
                "protocol_version",
                i.project_id.as_deref(),
                p["id"].as_str(),
                1,
                &json!({"protocol_id":p["id"],"version":1,"body":i.body.unwrap_or_default(),"root_block_version_id":i.root_block_version_id,"root_bindings":i.root_bindings}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"protocol":p,"version":v}))
        }
        "protocol.revise" => {
            let i: ProtocolRevise = parse_input(path)?;
            if !has_kind(&db, &i.protocol_id, "protocol")? {
                return Err(CliError::missing("Protocol 不存在"));
            }
            let latest:i64=db.query_row("SELECT COALESCE(MAX(revision),0) FROM records WHERE kind='protocol_version' AND parent_id=?1",[&i.protocol_id],|r|r.get(0)).map_err(|e|CliError::internal(e.to_string()))?;
            if latest != i.expected_version {
                return Err(CliError::conflict(format!(
                    "Protocol 已從 v{} 更新為 v{}；請先重新讀取。",
                    i.expected_version, latest
                )));
            }
            let (_, project, _, _, _) = kind_id(&db, &i.protocol_id)?
                .ok_or_else(|| CliError::missing("Protocol 不存在"))?;
            if i.body.is_none() && i.root_block_version_id.is_none() && i.root_bindings.is_none() {
                return Err(CliError::input(
                    "至少提供 body、root_block_version_id 或 root_bindings 之一",
                ));
            }
            let previous_raw: String = db.query_row("SELECT data_json FROM records WHERE kind='protocol_version' AND parent_id=?1 AND revision=?2", params![i.protocol_id, latest], |r| r.get(0)).map_err(|e| CliError::internal(e.to_string()))?;
            let previous: Value = serde_json::from_str(&previous_raw)
                .map_err(|e| CliError::internal(e.to_string()))?;
            let body = i
                .body
                .unwrap_or_else(|| previous["body"].as_str().unwrap_or_default().to_owned());
            let root_id = match i.root_block_version_id {
                None => previous
                    .get("root_block_version_id")
                    .and_then(Value::as_str)
                    .map(str::to_owned),
                Some(Value::Null) => None,
                Some(Value::String(id)) => Some(id),
                Some(_) => {
                    return Err(CliError::input("root_block_version_id 必须是字符串或 null"))
                }
            };
            let previous_bindings = previous
                .get("root_bindings")
                .and_then(Value::as_array)
                .cloned()
                .unwrap_or_default();
            let root_bindings =
                match (&root_id, i.root_bindings) {
                    (Some(_), Some(values)) => serde_json::to_value(values)
                        .map_err(|e| CliError::internal(e.to_string()))?,
                    (Some(_), None) => json!(previous_bindings),
                    (None, Some(values)) => serde_json::to_value(values)
                        .map_err(|e| CliError::internal(e.to_string()))?,
                    (None, None) => json!([]),
                };
            if let Some(block_version_id) = &root_id {
                let (block_record, block_data) = block_version_data(&db, block_version_id)?;
                if project.as_deref() != block_record["project_id"].as_str() {
                    return Err(CliError::input("Protocol 与根步骤块必须属于同一项目"));
                }
                let typed_bindings: Vec<RootParameterValue> =
                    serde_json::from_value(root_bindings.clone())
                        .map_err(|e| CliError::input(format!("root_bindings 格式无效：{e}")))?;
                validate_root_bindings(&block_data.parameter_slots, &typed_bindings)?;
            } else if root_bindings.as_array().is_some_and(|v| !v.is_empty()) {
                return Err(CliError::input(
                    "清除 root_block_version_id 时必须清空 root_bindings",
                ));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let v = insert(
                &tx,
                "protocol_version",
                project.as_deref(),
                Some(&i.protocol_id),
                latest + 1,
                &json!({"protocol_id":i.protocol_id,"version":latest+1,"body":body,"root_block_version_id":root_id,"root_bindings":root_bindings}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"version":v}))
        }
        "plate.create" => {
            let i: PlateCreate = parse_input(path)?;
            let (kind, project, _, _, _) =
                kind_id(&db, &i.run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
            if kind != "run" {
                return Err(CliError::input("run_id 必须指向实验运行"));
            }
            if let Some(assay_id) = i.assay_id.as_deref().filter(|id| !id.is_empty()) {
                validate_run_assay_reference(&db, &i.run_id, assay_id, i.timepoint.as_deref())?;
            } else if i
                .timepoint
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty())
            {
                return Err(CliError::input(
                    "填写板图 timepoint 时必须同时关联 assay_id",
                ));
            }
            check_text(&i.name, "板名", 100)?;
            if !(1..=32).contains(&i.rows) || !(1..=48).contains(&i.columns) {
                return Err(CliError::input("板型行列范围无效：行 1–32、列 1–48"));
            }
            let standard = match i.format.as_str() {
                "6" => Some((2, 3)),
                "12" => Some((3, 4)),
                "24" => Some((4, 6)),
                "48" => Some((6, 8)),
                "96" => Some((8, 12)),
                "384" => Some((16, 24)),
                "custom" => None,
                _ => {
                    return Err(CliError::input(
                        "format 必须是 6、12、24、48、96、384 或 custom",
                    ))
                }
            };
            if standard.is_some_and(|(rows, columns)| (i.rows, i.columns) != (rows, columns)) {
                return Err(CliError::input(
                    "标准板型的 rows 和 columns 必须与 format 对应；自定义布局请使用 format=custom",
                ));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "plate",
                project.as_deref(),
                Some(&i.run_id),
                1,
                &json!({"name":i.name.trim(),"format":i.format,"rows":i.rows,"columns":i.columns,"assay_id":i.assay_id.filter(|id| !id.is_empty()),"timepoint":i.timepoint.filter(|point| !point.trim().is_empty())}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "well.set" => {
            let i: WellSet = parse_input(path)?;
            let (_, project, _, _, raw) = kind_id(&db, &i.plate_id)?
                .filter(|r| r.0 == "plate")
                .ok_or_else(|| CliError::missing("板布局不存在"))?;
            let plate: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            let (letters, digits) = split_well(&i.well_id)?;
            let row = letters.chars().fold(0usize, |n, c| {
                n * 26 + (c.to_ascii_uppercase() as usize - 'A' as usize + 1)
            });
            let col = digits
                .parse::<usize>()
                .map_err(|_| CliError::input("孔位列号无效"))?;
            if row == 0
                || row > plate["rows"].as_u64().unwrap_or(0) as usize
                || col == 0
                || col > plate["columns"].as_u64().unwrap_or(0) as usize
            {
                return Err(CliError::input("孔位超出板型范围"));
            }
            let old:Option<(String,i64)>=db.query_row("SELECT id,revision FROM records WHERE kind='well' AND parent_id=?1 AND upper(json_extract(data_json,'$.well_id'))=?2",params![i.plate_id,i.well_id.to_ascii_uppercase()],|r|Ok((r.get(0)?,r.get(1)?))).optional().map_err(|e|CliError::internal(e.to_string()))?;
            let data = json!({"well_id":i.well_id.to_ascii_uppercase(),"group_label":i.group_label,"sample_id":i.sample_id,"note":i.note});
            let r = if let Some(id) = old {
                let expected = i.expected_revision.ok_or_else(|| {
                    CliError::conflict("孔位已存在；修订时必须提供 expected_revision")
                })?;
                if expected != id.1 {
                    return Err(CliError::conflict(format!(
                        "孔位版本已从 v{expected} 更新为 v{}；请先重新读取。",
                        id.1
                    )));
                }
                update_data(&db, &id.0, expected, "well", &data)?
            } else {
                if i.expected_revision.is_some() {
                    return Err(CliError::conflict(
                        "孔位尚不存在；创建时不应提供 expected_revision",
                    ));
                }
                let tx = db
                    .unchecked_transaction()
                    .map_err(|e| CliError::internal(e.to_string()))?;
                let created = insert(&tx, "well", project.as_deref(), Some(&i.plate_id), 1, &data)?;
                tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
                created
            };
            Ok(json!({"record":r}))
        }
        "note.create" => {
            let i: NoteCreate = parse_input(path)?;
            let (kind, project, _, _, _) =
                kind_id(&db, &i.run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
            if kind != "run" {
                return Err(CliError::input("run_id 必须指向实验运行"));
            }
            check_text(&i.content, "记录内容", 10000)?;
            if let Some(time) = &i.event_time {
                if DateTime::parse_from_rfc3339(time).is_err() {
                    return Err(CliError::input(
                        "event_time 必须是带时区的 RFC 3339 时间，例如 2026-11-02T10:30:00+08:00",
                    ));
                }
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "note",
                project.as_deref(),
                Some(&i.run_id),
                1,
                &json!({"content":i.content,"event_time":i.event_time,"observation":i.observation,"result":i.result,"deviation":i.deviation}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "execution_event.create" => {
            let i: ExecutionEventCreate = parse_input(path)?;
            let payload =
                serde_json::to_value(&i).map_err(|e| CliError::internal(e.to_string()))?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = create_execution_event_tx(&tx, &i.run_id, &payload)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "formula.create" => {
            let i: FormulaCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            check_text(&i.title, "公式名称", 160)?;
            check_text(&i.expression, "公式表达式", 2000)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "formula",
                Some(&i.project_id),
                Some(&i.project_id),
                1,
                &json!({"title":i.title,"expression":i.expression,"variables":i.variables,"notes":i.notes}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "formula.revise" => {
            let i: FormulaRevise = parse_input(path)?;
            if i.title.is_none()
                && i.expression.is_none()
                && i.variables.is_none()
                && i.notes.is_none()
            {
                return Err(CliError::input("至少提供一个要修订的公式字段"));
            }
            let (_, _, _, revision, raw) =
                kind_id(&db, &i.id)?.ok_or_else(|| CliError::missing("公式不存在"))?;
            if revision != i.expected_revision {
                return Err(CliError::conflict(format!(
                    "公式版本已从 v{} 更新为 v{revision}；请先重新读取。",
                    i.expected_revision
                )));
            }
            let mut data: Value =
                serde_json::from_str(&raw).map_err(|e| CliError::internal(e.to_string()))?;
            if let Some(v) = i.title {
                check_text(&v, "公式名称", 160)?;
                data["title"] = json!(v.trim())
            }
            if let Some(v) = i.expression {
                check_text(&v, "公式表达式", 2000)?;
                data["expression"] = json!(v.trim())
            }
            if let Some(v) = i.variables {
                data["variables"] = json!(v)
            }
            if let Some(v) = i.notes {
                data["notes"] = json!(v)
            }
            Ok(json!({"record":update_data(&db,&i.id,i.expected_revision,"formula",&data)?}))
        }
        "table.create" => {
            let i: TableCreate = parse_input(path)?;
            ensure_project(&db, &i.project_id)?;
            check_text(&i.name, "资料表名称", 120)?;
            if i.columns.is_empty() || i.columns.len() > 40 || i.rows.len() > 5000 {
                return Err(CliError::input("资料表列数应为 1–40，行数最多 5000"));
            }
            if i.rows.iter().any(|r| r.len() != i.columns.len()) {
                return Err(CliError::input("每行单元格数必须与 columns 数量一致"));
            }
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let r = insert(
                &tx,
                "library_table",
                Some(&i.project_id),
                Some(&i.project_id),
                1,
                &json!({"name":i.name,"columns":i.columns,"rows":i.rows}),
            )?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"record":r}))
        }
        "table.revise" => {
            let i: TableRevise = parse_input(path)?;
            check_text(&i.name, "资料表名称", 120)?;
            if i.columns.is_empty() || i.columns.len() > 40 || i.rows.len() > 5000 {
                return Err(CliError::input("资料表列数应为 1–40，行数最多 5000"));
            }
            if i.rows.iter().any(|r| r.len() != i.columns.len()) {
                return Err(CliError::input("每行单元格数必须与 columns 数量一致"));
            }
            let data = json!({"name":i.name,"columns":i.columns,"rows":i.rows});
            Ok(json!({"record":update_data(&db,&i.id,i.expected_revision,"library_table",&data)?}))
        }
        "record.list" => {
            let i: RecordList = parse_input(path)?;
            if let Some(k) = &i.kind {
                if ![
                    "project",
                    "design",
                    "design_version",
                    "run",
                    "task",
                    "protocol",
                    "protocol_version",
                    "protocol_block",
                    "protocol_block_version",
                    "plate",
                    "well",
                    "note",
                    "execution_event",
                    "formula",
                    "library_table",
                ]
                .contains(&k.as_str())
                {
                    return Err(CliError::input(
                        "不支持的 kind；请通过 schema record.list 查看允许值",
                    ));
                }
            }
            Ok(json!({"records":list(&db,i.kind.as_deref(),i.project_id.as_deref())?}))
        }
        "record.get" => {
            let i: IdInput = parse_input(path)?;
            let r = get(&db, &i.id)?;
            if r["kind"] == "agent_draft" {
                return Err(CliError::missing("该旧版草稿类型已停用"));
            }
            Ok(json!({"record":r}))
        }
        "import.study.apply" => {
            let i: StudyImport = parse_input(path)?;
            validate_import(&i)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let p = insert(
                &tx,
                "project",
                None,
                None,
                1,
                &json!({"name":i.project.name.trim(),"description":i.project.description.trim()}),
            )?;
            let mut designs = Vec::new();
            for d in &i.designs {
                designs.push(create_design_tx(
                    &tx,
                    p["id"].as_str().unwrap_or_default(),
                    d,
                    &default_labware_registry(),
                )?)
            }
            let mut protocols = Vec::new();
            for proto in &i.protocols {
                let pr = insert(
                    &tx,
                    "protocol",
                    Some(p["id"].as_str().unwrap_or_default()),
                    p["id"].as_str(),
                    1,
                    &json!({"title":proto.title.trim()}),
                )?;
                let ver = insert(
                    &tx,
                    "protocol_version",
                    Some(p["id"].as_str().unwrap_or_default()),
                    pr["id"].as_str(),
                    1,
                    &json!({"protocol_id":pr["id"],"version":1,"body":proto.body}),
                )?;
                protocols.push(json!({"protocol":pr,"version":ver}))
            }
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(json!({"writes":true,"project":p,"designs":designs,"protocols":protocols}))
        }
        _ => Err(CliError::missing(
            "未知工具；请运行 `labworkbench tools --json`",
        )),
    }
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RecordList {
    #[serde(default)]
    kind: Option<String>,
    #[serde(default)]
    project_id: Option<String>,
}
fn validate_import(i: &StudyImport) -> Result<(), CliError> {
    check_text(&i.project.name, "项目名称", 120)?;
    for d in &i.designs {
        let mut normalized = d.clone();
        normalize_design(&mut normalized, None);
        validate_design(&normalized)?
    }
    for p in &i.protocols {
        check_text(&p.title, "Protocol 名称", 160)?
    }
    if i.designs.len() > 500 || i.protocols.len() > 500 {
        return Err(CliError::input("单次导入的设计或 Protocol 最多 500 项"));
    }
    Ok(())
}
fn split_well(value: &str) -> Result<(String, String), CliError> {
    let s = value.trim().to_ascii_uppercase();
    let letters: String = s.chars().take_while(|c| c.is_ascii_alphabetic()).collect();
    let digits: String = s.chars().skip_while(|c| c.is_ascii_alphabetic()).collect();
    if letters.is_empty()
        || digits.is_empty()
        || letters.len() > 2
        || !digits.chars().all(|c| c.is_ascii_digit())
    {
        return Err(CliError::input("孔位格式应为字母行号加数字列号，例如 B4"));
    }
    Ok((letters, digits))
}

fn design_payload_schema() -> Value {
    design_schema()
}
fn record_data_schema(kind: &str) -> Value {
    match kind {
        "project" => {
            json!({"type":"object","additionalProperties":false,"required":["name","description"],"properties":{"name":{"type":"string"},"description":{"type":"string"}}})
        }
        "design" => design_payload_schema(),
        "design_version" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","design_revision","snapshot"],"properties":{"design_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"snapshot":design_payload_schema()}})
        }
        "run" => {
            json!({"type":"object","additionalProperties":false,"required":["title","scheduled_date","protocol_version_id","design_version_id","design_revision","status"],"properties":{"title":{"type":"string"},"scheduled_date":{"type":["string","null"],"format":"date"},"protocol_version_id":{"type":["string","null"]},"design_version_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","completed","blocked","cancelled"]},"run_index":{"type":"integer","minimum":1},"repeat_of_run_id":{"type":["string","null"]},"repeat_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]},"container_plan":{"type":"object","additionalProperties":true},"labware_profiles":{"type":"array","items":{"type":"object","additionalProperties":true}},"material_manifest":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["planned_output_id","title","evidence_id","producer_protocol","consumers"],"properties":{"planned_output_id":{"type":"string"},"title":{"type":"string"},"evidence_id":{"type":["string","null"]},"producer_protocol":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","protocol_version_id","protocol_id","protocol_title","protocol_version","root_block_version_id","block_version_ids","root_bindings","protocol_ready","steps","resolved_parameters","unresolved_parameters"],"properties":{"assay_id":{"type":["string","null"]},"assay_title":{"type":"string"},"protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},"protocol_title":{"type":"string"},"protocol_version":{"type":"integer","minimum":1},"root_block_version_id":{"type":["string","null"]},"block_version_ids":{"type":"array","items":{"type":"string"}},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","unit"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"protocol_ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object"}},"resolved_parameters":{"type":"array","items":{"type":"object"}},"unresolved_parameters":{"type":"array","items":{"type":"object"}}}},"consumers":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","group_id","group_name"],"properties":{"assay_id":{"type":"string"},"assay_title":{"type":"string"},"group_id":{"type":["string","null"]},"group_name":{"type":"string"}}}}}}},"protocol_manifest":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","protocol_version_id","protocol_id","protocol_title","protocol_version"],"properties":{"assay_id":{"type":["string","null"]},"assay_title":{"type":"string"},"protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},"protocol_title":{"type":"string"},"protocol_version":{"type":"integer","minimum":1},"root_block_version_id":{"type":["string","null"]},"block_version_ids":{"type":"array","items":{"type":"string"}},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","unit"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"protocol_ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object"}},"resolved_parameters":{"type":"array","items":{"type":"object"}},"unresolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","unit","required"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}}}}}}})
        }
        "task" => {
            json!({"type":"object","additionalProperties":false,"required":["title","scheduled_date","status","notes"],"properties":{"title":{"type":"string"},"scheduled_date":{"type":"string","format":"date"},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]},"notes":{"type":"string"}}})
        }
        "protocol" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"properties":{"title":{"type":"string"}}})
        }
        "protocol_version" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol_id","version","body"],"properties":{"protocol_id":{"type":"string"},"version":{"type":"integer","minimum":1},"body":{"type":"string"},"root_block_version_id":{"type":["string","null"]},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}}}})
        }
        "protocol_block" => {
            json!({"type":"object","additionalProperties":false,"required":["title","summary"],"properties":{"title":{"type":"string"},"summary":{"type":"string"}}})
        }
        "catalog_review" => {
            json!({"type":"object","additionalProperties":false,"required":["review_id","subject","status","reviewer","reviewed_at","evidence_id","evidence_locator","content_digest"],"properties":{"review_id":{"type":"string"},"subject":{"type":"object","additionalProperties":false,"required":["module_id","version"],"properties":{"module_id":{"type":"string"},"version":{"type":"integer","minimum":1}}},"status":{"type":"string","enum":["research_draft","source_checked","expert_reviewed","lab_validated"]},"reviewer":{"type":"string"},"reviewed_at":{"type":"string"},"evidence_id":{"type":"string"},"evidence_locator":{"type":"string"},"content_digest":{"type":"string"}}})
        }
        "labware_profile" => {
            json!({"type":"object","additionalProperties":true})
        }
        "component_definition" => {
            json!({"type":"object","additionalProperties":true})
        }
        "view" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"properties":{"title":{"type":"string"}}})
        }
        "view_document" => {
            json!({"type":"object","additionalProperties":true})
        }
        "protocol_block_version" => {
            json!({"type":"object","additionalProperties":false,"required":["block_id","version","summary","parameter_slots","steps"],"properties":{"block_id":{"type":"string"},"version":{"type":"integer","minimum":1},"summary":{"type":"string"},"module_contract":{"type":["object","null"]},"module_origin":{"type":["string","null"],"enum":["imported","derived","local_custom",null]},"module_contract_authority":{"type":["string","null"],"enum":["current","source_only",null]},"catalog_review":{"type":["object","null"]},"parameter_slots":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["key","label","value_type","unit","required"],"properties":{"key":{"type":"string"},"label":{"type":"string"},"value_type":{"type":"string","enum":["text","number","integer","boolean"]},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}},"steps":{"type":"array","items":{"oneOf":[{"type":"object","additionalProperties":false,"required":["kind","use_id","title","instruction","parameter_refs"],"properties":{"kind":{"const":"text"},"use_id":{"type":"string"},"title":{"type":"string"},"instruction":{"type":"string"},"parameter_refs":{"type":"array","items":{"type":"string"}},"visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]}}},{"type":"object","additionalProperties":false,"required":["kind","use_id","block_version_id","bindings"],"properties":{"kind":{"const":"block_use"},"use_id":{"type":"string"},"block_version_id":{"type":"string"},"bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","source_slot","unit"],"properties":{"slot":{"type":"string"},"value":{},"source_slot":{"type":["string","null"]},"unit":{"type":["string","null"]}}}}}}]}}}})
        }
        "plate" => {
            json!({"type":"object","additionalProperties":false,"required":["name","format","rows","columns"],"properties":{"name":{"type":"string"},"format":{"type":"string"},"rows":{"type":"integer","minimum":1,"maximum":32},"columns":{"type":"integer","minimum":1,"maximum":48},"assay_id":{"type":["string","null"]},"timepoint":{"type":["string","null"]}}})
        }
        "well" => {
            json!({"type":"object","additionalProperties":false,"required":["well_id","group_label","sample_id","note"],"properties":{"well_id":{"type":"string"},"group_label":{"type":"string"},"sample_id":{"type":"string"},"note":{"type":"string"}}})
        }
        "note" => {
            json!({"type":"object","additionalProperties":false,"required":["content","event_time","observation","result","deviation"],"properties":{"content":{"type":"string"},"event_time":{"type":["string","null"],"format":"date-time"},"event_time_local":{"type":["string","null"]},"event_timezone":{"type":["string","null"]},"observation":{"type":"string"},"result":{"type":"string"},"deviation":{"type":"string"}}})
        }
        "execution_event" => {
            json!({"type":"object","additionalProperties":false,"required":["event_type","content","occurred_at","stage","timepoint","step_status","sample_ids","materials","measurements","deviation"],"properties":{"event_type":{"type":"string","enum":["operation","material_use","measurement","deviation","observation"]},"content":{"type":"string"},"occurred_at":{"type":["string","null"],"format":"date-time"},"stage":{"type":["string","null"]},"timepoint":{"type":["string","null"],"maxLength":100},"step_status":{"type":["string","null"],"enum":["in_progress","completed","blocked","skipped",null]},"protocol_step_path":{"type":["string","null"]},"protocol_version_id":{"type":["string","null"]},"assay_id":{"type":["string","null"]},"planned_output_id":{"type":["string","null"]},"sample_ids":{"type":"array","items":{"type":"string"}},"materials":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["material","planned_output_id","amount","unit"],"properties":{"material":{"type":"string"},"planned_output_id":{"type":["string","null"]},"amount":{"type":["number","null"]},"unit":{"type":["string","null"]}}}},"measurements":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["endpoint","value","unit","sample_id"],"properties":{"endpoint":{"type":"string"},"value":{"type":"number"},"unit":{"type":["string","null"]},"sample_id":{"type":["string","null"]}}}},"deviation":{"type":["string","null"]}}})
        }
        "formula" => {
            json!({"type":"object","additionalProperties":false,"required":["title","expression","variables","notes"],"properties":{"title":{"type":"string"},"expression":{"type":"string"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "library_table" => {
            json!({"type":"object","additionalProperties":false,"required":["name","columns","rows"],"properties":{"name":{"type":"string"},"columns":{"type":"array","items":{"type":"string"}},"rows":{"type":"array","items":{"type":"array","items":{"type":"string"}}}}})
        }
        _ => json!({"type":"object","additionalProperties":false}),
    }
}
fn record_schema(kind: &str) -> Value {
    let mut data = record_data_schema(kind);
    data["additionalProperties"] = json!(false);
    json!({"type":"object","additionalProperties":false,"required":["id","kind","project_id","parent_id","revision","data","created_at","updated_at"],"properties":{"id":{"type":"string"},"kind":{"const":kind},"project_id":{"type":["string","null"]},"parent_id":{"type":["string","null"]},"revision":{"type":"integer","minimum":1},"data":data,"created_at":{"type":"string","format":"date-time"},"updated_at":{"type":"string","format":"date-time"}}})
}
fn any_record_schema() -> Value {
    let kinds = [
        "project",
        "design",
        "design_version",
        "run",
        "task",
        "protocol",
        "protocol_version",
        "protocol_block",
        "protocol_block_version",
        "labware_profile",
        "component_definition",
        "view",
        "view_document",
        "catalog_review",
        "plate",
        "well",
        "note",
        "execution_event",
        "formula",
        "library_table",
    ];
    json!({"oneOf":kinds.iter().map(|k|record_schema(k)).collect::<Vec<_>>()})
}
fn result_schema(tool: &str) -> Value {
    let arr = |kind: &str| json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":record_schema(kind)}}});
    let one = |kind: &str| json!({"type":"object","additionalProperties":false,"required":["record"],"properties":{"record":record_schema(kind)}});
    let record_and_version = |record_kind: &str| json!({"type":"object","additionalProperties":false,"required":["record","version"],"properties":{"record":record_schema(record_kind),"version":record_schema("design_version")}});
    match tool {
        "project.list" => arr("project"),
        "project.create" => one("project"),
        "project.revise" => one("project"),
        "design.list" => arr("design"),
        "design.get" | "design.create" => {
            json!({"type":"object","additionalProperties":false,"required":["design","version"],"properties":{"design":record_schema("design"),"version":record_schema("design_version")}})
        }
        "design.map" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision","title","objective","experimental_unit","groups","assays","sample_links","planned_outputs","conflicts","unresolved"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"title":{"type":"string"},"objective":{"type":"string"},"experimental_unit":{"type":["string","null"]},"groups":{"type":"array","items":{"type":"object","additionalProperties":true}},"assays":{"type":"array","items":{"type":"object","additionalProperties":true}},"sample_links":{"type":"array","items":{"type":"object","additionalProperties":true}},"planned_outputs":{"type":"array","items":{"type":"object","additionalProperties":true}},"conflicts":{"type":"array","items":{"type":"object","additionalProperties":true}},"unresolved":{"type":"array","items":{"type":"object","additionalProperties":true}}}})
        }
        "design.revise" => record_and_version("design"),
        "run.list" => arr("run"),
        "run.create" => one("run"),
        "run.set_status" => one("run"),
        "run.revise" => one("run"),
        "run.workspace" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","design_id","design_version_id","design_revision","frozen_design","protocol_version_id","status","scopes","steps","tasks","occurrences","materials","plates","events"],"properties":{"run_id":{"type":"string"},"design_id":{"type":"string"},"design_version_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"frozen_design":{"type":"object","additionalProperties":true},"protocol_version_id":{"type":["string","null"]},"status":{"type":"string"},"scopes":{"type":"array","items":{"type":"object","additionalProperties":true}},"steps":{"type":"array","items":{"type":"object","additionalProperties":true}},"tasks":{"type":"array","items":{"type":"object","additionalProperties":true}},"occurrences":{"type":"array","items":{"type":"object","additionalProperties":true}},"materials":{"type":"array","items":{"type":"object","additionalProperties":true}},"plates":{"type":"array","items":{"type":"object","additionalProperties":true}},"events":{"type":"array","items":{"type":"object","additionalProperties":true}}}})
        }
        "task.create" => one("task"),
        "task.set_status" => one("task"),
        "protocol.list" => arr("protocol"),
        "protocol.block.list" => arr("protocol_block"),
        "protocol.block.get" => {
            json!({"type":"object","additionalProperties":false,"required":["block","latest_version","versions"],"properties":{"block":record_schema("protocol_block"),"latest_version":record_schema("protocol_block_version"),"versions":{"type":"array","items":record_schema("protocol_block_version")}}})
        }
        "catalog.review.record" => one("catalog_review"),
        "protocol.block.create" | "catalog.operation.import" => {
            json!({"type":"object","additionalProperties":false,"required":["block","version"],"properties":{"block":record_schema("protocol_block"),"version":record_schema("protocol_block_version")}})
        }
        "protocol.block.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["version"],"properties":{"version":record_schema("protocol_block_version")}})
        }
        "protocol.block.preview" => {
            json!({"type":"object","additionalProperties":false,"required":["block_version_id","block_id","title","version","ready","steps","resolved_parameters","unresolved_parameters","block_version_ids"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"block_version_id":{"type":"string"},"block_id":{"type":"string"},"title":{"type":"string"},"version":{"type":"integer"},"ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","use_id","title","instruction","parameters","unresolved_parameter_refs","source_block_version_id"],"properties":{"path":{"type":"string"},"use_id":{"type":"string"},"title":{"type":"string"},"instruction":{"type":"string"},"parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","label","value","unit","provenance"],"properties":{"slot":{"type":"string"},"label":{"type":"string"},"value":{},"unit":{"type":["string","null"]},"provenance":{"type":"array","items":{"type":"object"}}}}},"unresolved_parameter_refs":{"type":"array","items":{"type":"string"}},"source_block_version_id":{"type":"string"},"visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]}}}},"resolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","value","unit","provenance"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"value":{},"unit":{"type":["string","null"]},"provenance":{"type":"array","items":{"type":"object"}}}}},"unresolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","unit","required"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}},"block_version_ids":{"type":"array","items":{"type":"string"}}}})
        }
        "protocol.create" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol","version"],"properties":{"protocol":record_schema("protocol"),"version":record_schema("protocol_version")}})
        }
        "protocol.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["version"],"properties":{"version":record_schema("protocol_version")}})
        }
        "plate.create" => one("plate"),
        "well.set" => one("well"),
        "note.create" => one("note"),
        "execution_event.create" => one("execution_event"),
        "formula.create" => one("formula"),
        "formula.revise" => one("formula"),
        "table.create" => one("library_table"),
        "table.revise" => one("library_table"),
        "record.list" => {
            json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":any_record_schema()}}})
        }
        "record.get" => {
            json!({"type":"object","additionalProperties":false,"required":["record"],"properties":{"record":any_record_schema()}})
        }
        "import.study.preview" => {
            json!({"type":"object","additionalProperties":false,"required":["valid","writes","project_count","design_count","protocol_count","warnings"],"properties":{"valid":{"const":true},"writes":{"const":false},"project_count":{"const":1},"design_count":{"type":"integer","minimum":0},"protocol_count":{"type":"integer","minimum":0},"warnings":{"type":"array","items":{"type":"string"}}}})
        }
        "import.study.apply" => {
            json!({"type":"object","additionalProperties":false,"required":["writes","project","designs","protocols"],"properties":{"writes":{"const":true},"project":record_schema("project"),"designs":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["design","version"],"properties":{"design":record_schema("design"),"version":record_schema("design_version")}}},"protocols":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["protocol","version"],"properties":{"protocol":record_schema("protocol"),"version":record_schema("protocol_version")}}}}})
        }
        "calculator.dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["stock_volume","volume_unit","diluent_volume","formula","assumption"],"properties":{"stock_volume":{"type":"number","minimum":0},"volume_unit":{"type":"string","enum":["L","mL","µL"]},"diluent_volume":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.solution" => {
            json!({"type":"object","additionalProperties":false,"required":["mass_g","mass_mg","formula","assumption"],"properties":{"mass_g":{"type":"number","minimum":0},"mass_mg":{"type":"number","minimum":0},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.serial_dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["per_step_dilution_factor","levels","formula","assumption"],"properties":{"per_step_dilution_factor":{"type":"number","exclusiveMinimum":0},"levels":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["step","concentration","unit"],"properties":{"step":{"type":"integer","minimum":1},"concentration":{"type":"number","minimum":0},"unit":{"type":"string","enum":["M","mM","µM","nM"]}}}},"formula":{"type":"string"},"assumption":{"type":"string"}}})
        }
        "calculator.reaction_volume" => {
            json!({"type":"object","additionalProperties":false,"required":["allocated_volume_ul","remaining_volume_ul","unit","assumption"],"properties":{"allocated_volume_ul":{"type":"number","minimum":0},"remaining_volume_ul":{"type":"number","minimum":0},"unit":{"const":"µL"},"assumption":{"type":"string"}}})
        }
        _ => json!({"type":"object"}),
    }
}
fn output_schema(tool: &str) -> Value {
    json!({"oneOf":[{"type":"object","additionalProperties":false,"required":["ok","tool","result"],"properties":{"ok":{"const":true},"tool":{"const":tool},"result":result_schema(tool)}},{"type":"object","additionalProperties":false,"required":["ok","tool","error"],"properties":{"ok":{"const":false},"tool":{"const":tool},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string","enum":["INVALID_INPUT","NOT_FOUND","REVISION_CONFLICT","INTERNAL"]},"message":{"type":"string"}}}}}]})
}
fn input_schema(tool: &str) -> Option<Value> {
    let s = match tool {
        "project.list" => json!({"type":"object","additionalProperties":false,"properties":{}}),
        "project.create" => {
            json!({"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"description":{"type":"string"}}})
        }
        "project.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision"],"anyOf":[{"required":["name"]},{"required":["description"]}],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"description":{"type":"string"}}})
        }
        "design.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":"string"}}})
        }
        "design.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "design.map" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "design.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","design"],"properties":{"project_id":{"type":"string"},"design":design_schema()}})
        }
        "design.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","changes"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"changes":{"type":"object","additionalProperties":false,"minProperties":1,"properties":design_schema()["properties"]}}})
        }
        "run.list" | "protocol.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":"string"}}})
        }
        "run.workspace" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "protocol.block.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"project_id":{"type":"string"}}})
        }
        "protocol.block.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "protocol.block.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","steps"],"properties":{"project_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"summary":{"type":"string","maxLength":1000},"parameter_slots":{"type":"array","maxItems":100,"items":parameter_slot_input_schema()},"steps":{"type":"array","minItems":1,"maxItems":100,"items":protocol_step_input_schema()}}})
        }
        "component.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{}})
        }
        "labware.define" => {
            json!({"type":"object","additionalProperties":false,"required":["profile"],"properties":{"profile":{"type":"object"}}})
        }
        "component.define" => {
            json!({"type":"object","additionalProperties":false,"required":["definition"],"properties":{"definition":{"type":"object"}}})
        }
        "view.project" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","protocol_version_id","scope"],"properties":{"project_id":{"type":"string"},"protocol_version_id":{"type":"string"},"scope":{"type":"object"},"document_id":{"type":"string"}}})
        }
        "view.data_preview" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","instance_id"],"properties":{"view_id":{"type":"string"},"instance_id":{"type":"string"},"calculator_instance_id":{"type":"string"},"volume_per_well":{"type":["number","null"]},"extra_ratio":{"type":["number","null"]}}})
        }
        "view.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","document"],"properties":{"project_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160},"document":{"type":"object"}}})
        }
        "view.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "view.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","changeset"],"properties":{"view_id":{"type":"string"},"changeset":{"type":"object"}}})
        }
        "semantic.inspect" => {
            json!({"type":"object","additionalProperties":false,"required":["block_version_id"],"properties":{"block_version_id":{"type":"string"}}})
        }
        "semantic.search" => {
            json!({"type":"object","additionalProperties":false,"properties":{"query":{"type":"string"},"project_id":{"type":"string"}}})
        }
        "semantic.validate"
        | "semantic.propose_patch"
        | "semantic.assess_patch"
        | "semantic.apply_binding_patch" => {
            json!({"type":"object","additionalProperties":false,"required":["changeset"],"properties":{"changeset":{"type":"object"}}})
        }
        "catalog.review.record" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","review"],"properties":{"project_id":{"type":"string"},"review":{"type":"object"}}})
        }
        "catalog.operation.import" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","module","review_id"],"properties":{"project_id":{"type":"string"},"summary":{"type":"string","maxLength":1000},"module":{"type":"object"},"review_id":{"type":"string"}}})
        }
        "protocol.block.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["block_id","expected_version","steps"],"properties":{"block_id":{"type":"string"},"expected_version":{"type":"integer","minimum":1},"summary":{"type":"string","maxLength":1000},"parameter_slots":{"type":"array","maxItems":100,"items":parameter_slot_input_schema()},"steps":{"type":"array","minItems":1,"maxItems":100,"items":protocol_step_input_schema()}}})
        }
        "protocol.block.preview" => {
            json!({"type":"object","additionalProperties":false,"required":["block_version_id"],"properties":{"block_version_id":{"type":"string"},"bindings":root_parameter_values_input_schema()}})
        }
        "run.set_status" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","status"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","completed","blocked","cancelled"]}}})
        }
        "run.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision"],"anyOf":[{"required":["title"]},{"required":["scheduled_date"]}],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"scheduled_date":{"type":["string","null"],"format":"date"}}})
        }
        "run.create" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","title"],"properties":{"design_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"scheduled_date":{"type":["string","null"],"format":"date"},"protocol_version_id":{"type":["string","null"]},"repeat_of_run_id":{"type":["string","null"]},"repeat_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]}}})
        }
        "task.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","scheduled_date"],"properties":{"project_id":{"type":"string"},"run_id":{"type":["string","null"]},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"scheduled_date":{"type":"string","minLength":1,"format":"date"},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]},"notes":{"type":"string"}}})
        }
        "task.set_status" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","status"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]}}})
        }
        "protocol.create" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"anyOf":[{"required":["body"]},{"required":["root_block_version_id"]}],"properties":{"project_id":{"type":["string","null"]},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"body":{"type":"string"},"root_block_version_id":{"type":"string"},"root_bindings":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":"string","maxLength":40}}}}}})
        }
        "protocol.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol_id","expected_version"],"anyOf":[{"required":["body"]},{"required":["root_block_version_id"]},{"required":["root_bindings"]}],"properties":{"protocol_id":{"type":"string"},"expected_version":{"type":"integer","minimum":1},"body":{"type":"string"},"root_block_version_id":{"type":["string","null"]},"root_bindings":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":"string","maxLength":40}}}}}})
        }
        "plate.create" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","name","format","rows","columns"],"properties":{"run_id":{"type":"string"},"assay_id":{"type":"string","description":"可选；必须是 Run 冻结设计中的检测 ID"},"timepoint":{"type":"string","description":"可选；标注该实际板图所用的检测时间点，需与冻结设计一致"},"name":{"type":"string","minLength":1,"maxLength":100,"pattern":"\\S"},"format":{"type":"string","enum":["6","12","24","48","96","384","custom"]},"rows":{"type":"integer","minimum":1,"maximum":32},"columns":{"type":"integer","minimum":1,"maximum":48}}})
        }
        "well.set" => {
            json!({"type":"object","additionalProperties":false,"required":["plate_id","well_id"],"properties":{"plate_id":{"type":"string"},"well_id":{"type":"string","pattern":"^[A-Za-z]{1,2}[0-9]{1,2}$"},"expected_revision":{"type":"integer","minimum":1,"description":"仅覆盖已存在孔位时必需；创建新孔位时省略"},"group_label":{"type":"string"},"sample_id":{"type":"string"},"note":{"type":"string"}}})
        }
        "note.create" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","content"],"properties":{"run_id":{"type":"string"},"content":{"type":"string","minLength":1,"maxLength":10000,"pattern":"\\S"},"event_time":{"type":["string","null"],"format":"date-time"},"observation":{"type":"string"},"result":{"type":"string"},"deviation":{"type":"string"}}})
        }
        "execution_event.create" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","event_type","content"],"properties":{"run_id":{"type":"string"},"event_type":{"type":"string","enum":["operation","material_use","measurement","deviation","observation"]},"content":{"type":"string","minLength":1,"maxLength":5000,"pattern":"\\S"},"occurred_at":{"type":["string","null"],"format":"date-time"},"stage":{"type":["string","null"],"maxLength":160},"timepoint":{"type":["string","null"],"maxLength":100,"description":"检测步骤的计划时间点；必须属于Run冻结设计中的该检测"},"step_status":{"type":["string","null"],"enum":["in_progress","completed","blocked","skipped",null],"description":"显式步骤状态；普通事件不代表步骤完成"},"protocol_step_path":{"type":["string","null"],"maxLength":500},"protocol_version_id":{"type":["string","null"],"description":"关联步骤所引用的冻结 Protocol 版本"},"assay_id":{"type":["string","null"],"description":"关联步骤所属的冻结检测；共享制备步骤留空"},"planned_output_id":{"type":["string","null"],"description":"共享制备步骤所属计划产物；检测步骤留空"},"sample_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"}},"materials":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["material"],"properties":{"material":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"planned_output_id":{"type":["string","null"]},"amount":{"type":["number","null"],"minimum":0},"unit":{"type":["string","null"],"maxLength":40}}}},"measurements":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["endpoint","value"],"properties":{"endpoint":{"type":"string","minLength":1,"maxLength":300,"pattern":"\\S"},"value":{"type":"number"},"unit":{"type":["string","null"],"maxLength":40},"sample_id":{"type":["string","null"],"maxLength":120}}}},"deviation":{"type":["string","null"],"maxLength":5000}}})
        }
        "formula.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","title","expression"],"properties":{"project_id":{"type":"string"},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"expression":{"type":"string","minLength":1,"maxLength":2000,"pattern":"\\S"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "formula.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision"],"anyOf":[{"required":["title"]},{"required":["expression"]},{"required":["variables"]},{"required":["notes"]}],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"expression":{"type":"string","minLength":1,"maxLength":2000,"pattern":"\\S"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "table.create" => {
            json!({"type":"object","additionalProperties":false,"required":["project_id","name","columns","rows"],"properties":{"project_id":{"type":"string"},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"columns":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":40},"rows":{"type":"array","maxItems":5000,"items":{"type":"array","items":{"type":"string"}}}}})
        }
        "table.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["id","expected_revision","name","columns","rows"],"properties":{"id":{"type":"string"},"expected_revision":{"type":"integer","minimum":1},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"columns":{"type":"array","items":{"type":"string"},"minItems":1,"maxItems":40},"rows":{"type":"array","maxItems":5000,"items":{"type":"array","items":{"type":"string"}}}}})
        }
        "record.list" => {
            json!({"type":"object","additionalProperties":false,"properties":{"kind":{"type":"string","enum":["project","design","design_version","run","task","protocol","protocol_version","protocol_block","protocol_block_version","plate","well","note","execution_event","formula","library_table"]},"project_id":{"type":"string"}}})
        }
        "record.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "import.study.preview" | "import.study.apply" => {
            json!({"type":"object","additionalProperties":false,"required":["project"],"properties":{"project":{"type":"object","additionalProperties":false,"required":["name"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"description":{"type":"string"}}},"designs":{"type":"array","maxItems":500,"items":design_schema()},"protocols":{"type":"array","maxItems":500,"items":{"type":"object","additionalProperties":false,"required":["title","body"],"properties":{"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"body":{"type":"string"}}}}}})
        }
        "calculator.dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["stock_concentration","stock_unit","target_concentration","target_unit","final_volume","volume_unit"],"properties":{"stock_concentration":{"type":"number","exclusiveMinimum":0},"stock_unit":{"type":"string","enum":["M","mM","µM","nM"]},"target_concentration":{"type":"number","minimum":0},"target_unit":{"type":"string","enum":["M","mM","µM","nM"]},"final_volume":{"type":"number","exclusiveMinimum":0},"volume_unit":{"type":"string","enum":["L","mL","µL"]}}})
        }
        "calculator.solution" => {
            json!({"type":"object","additionalProperties":false,"required":["concentration_mol_l","final_volume","volume_unit","molecular_weight_g_mol"],"properties":{"concentration_mol_l":{"type":"number","exclusiveMinimum":0},"final_volume":{"type":"number","exclusiveMinimum":0},"volume_unit":{"type":"string","enum":["L","mL","µL"]},"molecular_weight_g_mol":{"type":"number","exclusiveMinimum":0}}})
        }
        "calculator.serial_dilution" => {
            json!({"type":"object","additionalProperties":false,"required":["initial_concentration","concentration_unit","transfer_volume","diluent_volume","steps"],"properties":{"initial_concentration":{"type":"number","exclusiveMinimum":0},"concentration_unit":{"type":"string","enum":["M","mM","µM","nM"]},"transfer_volume":{"type":"number","exclusiveMinimum":0},"diluent_volume":{"type":"number","minimum":0},"steps":{"type":"integer","minimum":1,"maximum":30}}})
        }
        "calculator.reaction_volume" => {
            json!({"type":"object","additionalProperties":false,"required":["target_volume_ul","components"],"properties":{"target_volume_ul":{"type":"number","exclusiveMinimum":0},"components":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["name","volume_ul"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"volume_ul":{"type":"number","minimum":0}}}}}})
        }
        _ => return None,
    };
    Some(s)
}
fn design_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,"required":["title"],
        "properties":{
        "title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S","description":"简短识别名称。"},
            "objective":{"type":"string","maxLength":5000,"description":"本实验实际要回答的研究问题；未知时留空。"},
            "experimental_unit":{"type":"string","maxLength":500,"description":"独立观测单位的明确定义；必须由研究者确定。"},
            "group_specs":{"type":"array","maxItems":100,"description":"已定义实验组。id 是稳定引用；independent_n 是每组独立实验单位数，不能用技术重复代替。","items":{"type":"object","additionalProperties":false,"required":["name"],"properties":{
                "id":{"type":"string","maxLength":80},"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"role":{"type":["string","null"],"enum":["control","treatment","comparator","reference","other",null]},
                "condition":{"type":["string","null"],"maxLength":500},"independent_n":{"type":["integer","null"],"minimum":1},"technical_repeats":{"type":["integer","null"],"minimum":1}
            }}},
            "timepoints":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":100,"pattern":"\\S"}},
            "primary_endpoint":{"type":"string","maxLength":1000},"endpoint_method":{"type":"string","maxLength":1000},"endpoint_unit":{"type":"string","maxLength":100},
            "sample_size_rationale":{"type":"string","maxLength":5000},"randomization":{"type":"string","maxLength":2000},"blinding":{"type":"string","maxLength":2000},
            "inclusion_criteria":{"type":"string","maxLength":5000},"exclusion_criteria":{"type":"string","maxLength":5000},"analysis_plan":{"type":"string","maxLength":5000},"controls":{"type":"string","maxLength":2000},
            "assays":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["id","title"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"endpoint":{"type":"string","maxLength":1000},"method":{"type":"string","maxLength":1000},"unit":{"type":"string","maxLength":100},"timepoints":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":100}},"timepoint_timing":{"type":"array","maxItems":100,"items":timepoint_timing_input_schema()},"group_names":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":120}},"group_ids":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":80}},"time_anchor_event":{"type":["string","null"],"maxLength":120},"sample_use_policy":{"description":"Legacy input only; normalized output uses timepoint_relation.","type":["string","null"],"enum":["new_per_timepoint","same_sample_repeated","split_from_assay","sequential_after_assay","unknown",null]},"timepoint_relation":{"type":["string","null"],"enum":["same_experimental_unit_repeated","same_physical_sample_repeated","independent_sample_per_timepoint","unknown",null]},"container_type":{"type":["string","null"],"maxLength":80},"container_format":{"type":["string","null"],"maxLength":80},"protocol_version_id":{"type":["string","null"]},"controls":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":300}},"control_group_ids":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":80}}}}},
            "sample_allocations":{"type":"array","maxItems":10000,"items":{"type":"object","additionalProperties":false,"required":["assay_id","group_name"],"properties":{"assay_id":{"type":"string"},"group_name":{"type":"string"},"group_id":{"type":["string","null"]},"relation":{"description":"Legacy input only; normalized output uses timepoint_relation and source_relation.","type":"string","enum":["new_per_timepoint","same_sample_repeated","split_from_assay","sequential_after_assay","unknown"]},"source_relation":{"type":["string","null"],"enum":["split_from_assay","sequential_after_assay","not_applicable","unknown",null]},"timepoint_relation":{"type":["string","null"],"enum":["same_experimental_unit_repeated","same_physical_sample_repeated","independent_sample_per_timepoint","unknown",null]},"source_assay_id":{"type":["string","null"]},"physical_slots_per_group":{"type":["integer","null"],"minimum":1},"physical_slots_scope":{"type":["string","null"],"enum":["per_timepoint","whole_assay_unique_positions","unknown",null]},"evidence_id":{"type":["string","null"]},"planned_output_id":{"type":["string","null"]}}}},
            "container_plan":{"type":"object","additionalProperties":true},
            "planned_outputs":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"required":["id","title","protocol_version_id"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"title":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"protocol_version_id":{"type":"string","minLength":1,"maxLength":80},"root_bindings":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string","minLength":1,"maxLength":100},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":["string","null"],"maxLength":40}}}},"evidence_id":{"type":["string","null"]}}}},
            "source_evidence":{"type":"array","maxItems":2000,"items":{"type":"object","additionalProperties":false,"required":["id","source_document","claim","state"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"source_document":{"type":"string","minLength":1,"maxLength":500},"source_date":{"type":["string","null"],"maxLength":40},"source_locator":{"type":["string","null"],"maxLength":500},"claim":{"type":"string","minLength":1,"maxLength":2000},"state":{"type":"string","enum":["source_record","proposal","user_confirmed","calculated","agent_suggestion"]}}}},
            "evidence_conflicts":{"type":"array","maxItems":1000,"items":{"type":"object","additionalProperties":false,"required":["id","topic","evidence_ids","status"],"properties":{"id":{"type":"string","minLength":1,"maxLength":80},"topic":{"type":"string","minLength":1,"maxLength":500},"evidence_ids":{"type":"array","minItems":2,"items":{"type":"string"}},"status":{"type":"string","enum":["unresolved","resolved"]},"affected_assay_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":80}},"affects_container_estimate":{"type":"boolean"},"resolution":{"type":["string","null"],"maxLength":2000}}}},
            "planned_run_count":{"type":["integer","null"],"minimum":1,"maximum":1000},"planned_run_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]},
            "groups":{"type":"string"},"hypothesis":{"type":"string"},"replicate_plan":{"type":"string"},"independent_variables":{"type":"string"}
        }
    })
}
fn schema(tool: &str) -> Option<Value> {
    input_schema(tool)
}
fn parameter_slot_input_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["key","label","value_type"],
        "properties":{
            "key":{"type":"string","pattern":"^[A-Za-z][A-Za-z0-9_]*$","maxLength":80},
            "label":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},
            "value_type":{"type":"string","enum":["text","number","integer","boolean"]},
            "unit":{"type":"string","maxLength":40},
            "required":{"type":"boolean"}
        }
    })
}
fn step_binding_input_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["slot"],"anyOf":[{"required":["value"]},{"required":["source_slot"]}],
        "properties":{
            "slot":{"type":"string"},
            "value":{"type":["string","number","integer","boolean"]},
            "source_slot":{"type":"string"},
            "unit":{"type":"string","maxLength":40}
        }
    })
}
fn protocol_step_input_schema() -> Value {
    json!({"oneOf":[
        {"type":"object","additionalProperties":false,"required":["kind","use_id","title","instruction"],"properties":{
            "kind":{"const":"text"},"use_id":{"type":"string","minLength":1,"maxLength":80},
            "title":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S"},
            "instruction":{"type":"string","minLength":1,"maxLength":4000,"pattern":"\\S"},
            "parameter_refs":{"type":"array","maxItems":100,"items":{"type":"string"}},
            "visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]}
        }},
        {"type":"object","additionalProperties":false,"required":["kind","use_id","block_version_id"],"properties":{
            "kind":{"const":"block_use"},"use_id":{"type":"string","minLength":1,"maxLength":80},
            "block_version_id":{"type":"string"},"bindings":{"type":"array","maxItems":100,"items":step_binding_input_schema()}
        }}
    ]})
}
fn root_parameter_values_input_schema() -> Value {
    json!({"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{
        "slot":{"type":"string"},"value":{"type":["string","number","integer","boolean"]},"unit":{"type":"string","maxLength":40}
    }}})
}
fn timepoint_timing_input_schema() -> Value {
    json!({
        "type":"object",
        "additionalProperties":false,
        "required":["timepoint","unit"],
        "properties":{
            "timepoint":{"type":"string","minLength":1,"maxLength":100,"pattern":"\\S"},
            "offset_value":{"type":["number","null"]},
            "unit":{"type":"string","enum":["min","h","day"]},
            "window_before":{"type":["number","null"],"minimum":0},
            "window_after":{"type":["number","null"],"minimum":0}
        }
    })
}
fn tool_specs() -> Vec<Value> {
    let names = [
        "project.list",
        "project.create",
        "project.revise",
        "design.list",
        "design.get",
        "design.map",
        "design.create",
        "design.revise",
        "run.list",
        "run.create",
        "run.set_status",
        "run.revise",
        "run.workspace",
        "task.create",
        "task.set_status",
        "protocol.list",
        "protocol.block.list",
        "protocol.block.get",
        "protocol.block.create",
        "labware.define",
        "component.list",
        "component.define",
        "view.project",
        "view.data_preview",
        "view.create",
        "view.get",
        "view.revise",
        "semantic.inspect",
        "semantic.search",
        "semantic.apply_binding_patch",
        "semantic.validate",
        "semantic.propose_patch",
        "semantic.assess_patch",
        "catalog.review.record",
        "catalog.operation.import",
        "protocol.block.revise",
        "protocol.block.preview",
        "protocol.create",
        "protocol.revise",
        "plate.create",
        "well.set",
        "note.create",
        "execution_event.create",
        "formula.create",
        "formula.revise",
        "table.create",
        "table.revise",
        "record.list",
        "record.get",
        "import.study.preview",
        "import.study.apply",
        "calculator.dilution",
        "calculator.solution",
        "calculator.serial_dilution",
        "calculator.reaction_volume",
    ];
    names.iter().filter_map(|n|Some(json!({"name":n,"description":tool_description(n),"side_effects":tool_effects(n),"writes_data":tool_writes(n),"command":format!("labworkbench {n} --input <json-file|->"),"input_schema":input_schema(n)?,"output_schema":output_schema(n)}))).collect()
}
fn tool_writes(tool: &str) -> bool {
    !matches!(
        tool,
        "project.list"
            | "design.list"
            | "design.get"
            | "run.list"
            | "protocol.list"
            | "protocol.block.list"
            | "protocol.block.get"
            | "protocol.block.preview"
            | "component.list"
            | "semantic.inspect"
            | "semantic.search"
            | "semantic.validate"
            | "semantic.propose_patch"
            | "semantic.assess_patch"
            | "record.list"
            | "record.get"
            | "import.study.preview"
            | "calculator.dilution"
            | "calculator.solution"
            | "calculator.serial_dilution"
            | "calculator.reaction_volume"
    )
}
fn tool_description(tool: &str) -> &'static str {
    match tool {
        "project.list" => "列出项目，返回稳定 id 和版本字段。",
        "project.create" => "创建一个新项目。",
        "project.revise" => "按 expected_revision 修订项目名称或说明。",
        "design.list" => "按项目查询实验设计。",
        "design.get" => "读取实验设计及其当前版本。",
        "design.map" => "只读生成实验地图：组别、检测、时间点、样本关系、Protocol 关联和待确认项；不写数据库。",
        "design.create" => "创建实验设计并自动保存 v1 快照。",
        "design.revise" => {
            "只修改 changes 中提供的字段，保留其他值并新增版本；必须提供 expected_revision。"
        }
        "run.list" => "按项目查询实验运行。",
        "run.create" => "从设计当前版本创建运行并固定设计/Protocol 版本引用。",
        "run.set_status" => "更新运行状态；必须提供读取时的 expected_revision。Protocol 必填参数未确认时拒绝开始运行。",
        "run.revise" => "按 expected_revision 修订运行名称或计划日期；设计版本、Protocol 清单、材料清单和重复关系不可改写。",
        "task.create" => "按项目、日期和可选运行创建日历任务。",
        "task.set_status" => "更新任务状态；必须提供读取时的 expected_revision。",
        "protocol.list" => "按项目查询 Protocol。",
        "protocol.block.list" => "按项目列出可复用步骤块。",
        "protocol.block.get" => "读取步骤块及其全部不可变版本。",
        "protocol.block.create" => "创建可作为完整 Protocol 或子步骤复用的版本化步骤块；只记录明确提供的操作和参数槽位。",
        "component.list" => "列出组件定义及是否已实现;未实现定义只能作为 catalog-only 查看。",
        "labware.define" => "注册一个不可变 LabwareProfile 版本;Design 容器计划从持久化注册表解析。",
        "component.define" => "注册一个不可变的 ComponentDefinition 版本;View 创建/修订从持久化注册表加载。",
        "view.project" => "从真实 ProtocolPreview 生成只读 ViewDocument 初始投影;不写入。",
        "view.data_preview" => "从 ViewDocument 的 PlannedContainer 读取孔位，执行通用试剂总量计算并生成表格；不写入。",
        "view.create" => "在受控事务中保存新的统一组件 ViewDocument。",
        "view.get" => "读取 ViewDocument 元数据及全部不可变版本。",
        "view.revise" => "按 expected_revision 应用 ViewChangeSet，生成新的 ViewDocument 版本。",
        "semantic.inspect" => "读取一个步骤块版本的步骤、绑定与 module_contract；不写入。",
        "semantic.search" => "按标题/目的/适用范围搜索当前项目已导入且来源为 current 的 Catalog 模块；不写入。",
        "semantic.validate" => "校验 ChangeSet 结构、路径和受保护字段；不写入。",
        "semantic.propose_patch" => "返回精确目标与 expected version 的变更摘要；不写入。",
        "semantic.assess_patch" => "在只读上下文中模拟一个绑定变更，返回旧值、新值和受影响引用；不写入。",
        "semantic.apply_binding_patch" => "受信任地提交一个 BlockUse 绑定变更，生成新的不可变 BlockVersion；不改 Protocol/Run。",
        "catalog.review.record" => "保存一条审核记录；正式导入必须通过 review_id 引用它。",
        "catalog.operation.import" => "在受控事务中导入审核过的 OperationModule，分配本地 Block/Version ID，并保存 module_contract 与 ReviewRecord。",
        "protocol.block.revise" => "按 expected_version 创建步骤块新版本；旧版本和引用不变。",
        "protocol.block.preview" => "只读展开组合步骤，返回操作顺序、参数来源和未填写槽位；不写入。",
        "protocol.create" => "创建 Protocol；可引用一个确切根步骤块版本及本版参数绑定，也兼容自由文本正文。",
        "protocol.revise" => "追加 Protocol 新版本；参数值可改变而不复制步骤块；必须提供 expected_version。",
        "plate.create" => "为实验运行创建指定行列数的空布局。",
        "well.set" => "创建或覆盖一个板孔映射。",
        "note.create" => "向运行追加一条实际记录；事件时间必须显式提供或留空。",
        "execution_event.create" => "向运行追加结构化实际事件；事件时间不确定时留空，材料/测量必须填写明确单位或样本标识。",
        "formula.create" => "保存公式表达式及变量单位说明，不执行公式代码。",
        "formula.revise" => "按 expected_revision 修订公式字段。",
        "table.create" => "创建列名固定、每行长度与列数一致的资料表。",
        "table.revise" => "按 expected_revision 替换资料表名称、列和全部行。",
        "record.list" => "按允许的 kind 和项目筛选记录，不返回停用的旧草稿。",
        "record.get" => "按 id 读取一条允许的记录，不返回停用的旧草稿。",
        "import.study.preview" => "校验项目/设计/Protocol 导入结构并报告待补项，不写数据库。",
        "import.study.apply" => "在单一事务中导入项目、设计快照和 Protocol v1。",
        "calculator.dilution" => "按 C1V1=C2V2 换算储液体积并校验单位。",
        "calculator.solution" => "按 m=C×V×MW 计算理论质量。",
        "calculator.serial_dilution" => "按相同体积单位计算逐级稀释浓度。",
        "calculator.reaction_volume" => "求反应已分配体积与剩余体积，不判断配方科学适用性。",
        _ => "未定义工具。",
    }
}
fn tool_effects(tool: &str) -> &'static str {
    match tool {
        "project.list"
        | "design.list"
        | "design.get"
        | "design.map"
        | "run.list"
        | "run.workspace"
        | "protocol.list"
        | "protocol.block.list"
        | "protocol.block.get"
        | "protocol.block.preview"
        | "record.list"
        | "record.get" => "只读；不修改数据库。",
        "import.study.preview" => "只校验导入内容并返回警告；不写入数据库。",
        "calculator.dilution"
        | "calculator.solution"
        | "calculator.serial_dilution"
        | "calculator.reaction_volume" => "只返回算术结果；不保存输入或结果。",
        "project.create" => "创建一个项目记录。",
        "project.revise" => "按 expected_revision 更新一个项目记录并递增 revision。",
        "design.create" => "创建实验设计并创建不可变的 v1 设计快照。",
        "design.revise" => "更新当前实验设计并创建新的不可变版本快照；不改写旧快照。",
        "run.create" => "创建一次运行并固定引用当时的设计版本和可选 Protocol 版本。",
        "run.set_status" | "task.set_status" => {
            "按 expected_revision 更新一个状态并递增 revision。"
        }
        "run.revise" => "按 expected_revision 只修订运行名称或计划日期；不可修改已冻结的科学设计和 Protocol/材料清单。",
        "task.create" => "创建一条日历任务。",
        "protocol.create" => "创建 Protocol 记录并创建不可变的 v1 正文版本。",
        "protocol.revise" => "为 Protocol 追加一个不可变的新版本；不覆盖旧正文。",
        "protocol.block.create" => "创建步骤块元数据和不可变 v1；步骤块只引用确切的其他版本。",
        "view.create" | "view.revise" => "保存新的 ViewDocument 版本；不改 Protocol/Run。",
        "view.get" => "只读；不修改数据库。",
        "semantic.inspect"
        | "semantic.search"
        | "semantic.validate"
        | "semantic.propose_patch"
        | "semantic.assess_patch" => "只读；不修改数据库或冻结 Run。",
        "catalog.review.record" => "保存一条审核证明；不创建 Block 或 Protocol。",
        "catalog.operation.import" => "创建一个本地导入的步骤块及其不可变版本；保留 Catalog 精确版本、module_contract 和审核记录。",
        "protocol.block.revise" => "为步骤块追加不可变版本；不会改写已保存的组合或运行。",
        "plate.create" => "为指定运行创建一个空铺板布局。",
        "well.set" => "新建孔位映射，或按 expected_revision 覆盖一个已有孔位。",
        "note.create" => "为运行追加一条实验记录；不修改既有记录。",
        "execution_event.create" => "为运行追加一条不可覆盖的结构化操作、材料使用、测量、偏差或观察事件。",
        "formula.create" => "创建一条公式记录。",
        "formula.revise" => "按 expected_revision 更新一条公式记录并递增 revision。",
        "table.create" => "创建一张资料表。",
        "table.revise" => "按 expected_revision 替换资料表的名称、列和全部数据行。",
        "import.study.apply" => {
            "在一个数据库事务中创建项目、设计及其 v1 快照、Protocol 及其 v1 版本；失败时整体回滚。"
        }
        _ => "未定义副作用。",
    }
}
