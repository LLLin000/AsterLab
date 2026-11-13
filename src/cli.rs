use asterlab_core::agent_edit::{
    AgentEditError, AgentEditErrorCode, ConfirmRequest, Proposal,
};
use asterlab_core::agent_edit_commit::{commit as commit_agent_edit_request, CommitError, CommitStore, CommittedVersion};
use asterlab_core::agent_edit_validate::{
    validate as validate_agent_edit_request, EditTargetStore, StoredBlockVersion,
    StoredProtocolVersion,
};
use asterlab_core::assets as asset_refs;
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
use asterlab_core::protocol_block::{ParameterSlot, ProtocolBlockVersionData, ProtocolStep, StepBinding};
use asterlab_core::protocol_preview::{
    OperationStep, ProtocolPreview, ResolvedParameter, UnresolvedParameter,
};
use asterlab_core::semantic_catalog::{compile_operation_module, CompilePolicy};
use asterlab_core::semantic_changeset::ChangeSet;
use asterlab_core::semantic_component::ProtocolScope;
use asterlab_core::semantic_query::{assess_block_patch, inspect_block, search_modules};
use asterlab_core::workflow_tree::{
    BlockDefinition, BlockParameter, BlockStep, DesignStep, Document, RunTask,
};
use chrono::{DateTime, NaiveDate, Utc};
use sha2::{Digest, Sha256};
use rusqlite::{params, Connection, OptionalExtension, Transaction, TransactionBehavior};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::{json, Value};
use std::{
    collections::{HashMap, HashSet},
    fs,
    io::{self, Read},
    path::{Path, PathBuf},
};
use uuid::Uuid;

const INPUT_LIMIT: u64 = 2 * 1024 * 1024;

const RECORDS_SCHEMA_VERSION: i64 = 1;

#[derive(Debug)]
pub(crate) struct CliError {
    pub(crate) code: &'static str,
    pub(crate) message: String,
    pub(crate) exit: i32,
    /// Agent edit v1 optional fields; empty for the older command set.
    pub(crate) path: Option<String>,
    pub(crate) current_version: Option<i64>,
    pub(crate) current_version_id: Option<String>,
    pub(crate) issues: Vec<Value>,
}
impl CliError {
    fn input(message: impl Into<String>) -> Self {
        Self {
            code: "INVALID_INPUT",
            message: message.into(),
            exit: 2,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
    fn missing(message: impl Into<String>) -> Self {
        Self {
            code: "NOT_FOUND",
            message: message.into(),
            exit: 3,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
    fn conflict(message: impl Into<String>) -> Self {
        Self {
            code: "REVISION_CONFLICT",
            message: message.into(),
            exit: 4,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
    fn internal(message: impl Into<String>) -> Self {
        Self {
            code: "INTERNAL",
            message: message.into(),
            exit: 5,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
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
    research_question: Option<String>,
    #[serde(default)]
    factors: Vec<asterlab_core::design_map::DesignFactor>,
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
    // Frozen SDK module payload. Authored through AsterDoc semantic commands;
    // legacy Design writes do not normally carry it.
    #[serde(default)]
    asterlab: Option<Value>,
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
    research_question: Option<Option<String>>,
    factors: Option<Vec<asterlab_core::design_map::DesignFactor>>,
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
struct InspectTreeInput {
    design_id: String,
    #[serde(default)]
    revision: Option<u64>,
    #[serde(default)]
    run_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct WorkflowProgressInput {
    run_id: String,
    #[serde(default)]
    step_path: Option<String>,
    #[serde(default)]
    sample_id: Option<String>,
    #[serde(default)]
    sample_ids: Vec<String>,
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
    view_id: Option<String>,
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
    position_ids: Vec<String>,
    #[serde(default)]
    calculator_instance_id: Option<String>,
    #[serde(default)]
    volume_per_well: Option<f64>,
    #[serde(default)]
    volume_unit: Option<String>,
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
struct ViewProposalCreate {
    view_id: String,
    changeset: ViewChangeSet,
    #[serde(default)]
    source: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ViewProposalAction {
    view_id: String,
    proposal_id: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct ViewProposalList {
    view_id: String,
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
                checks,
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
                checks,
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
                    checks: checks.clone(),
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

fn step_materialization_issues(design: &Value, protocol_manifest: &[Value]) -> Vec<String> {
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

fn asterdoc_step_materialization_issues(
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


fn execution_resource_snapshot(
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
fn run_creation_audit(design_data: &Value, revision: i64, version_id: &str) -> Result<Value, CliError> {
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
                    if !('A'..='H').contains(&row) || !(1..=12).contains(&col) {
                        errors.push(format!("检测 {assay_id} 的孔位 {well_id} 无效，应为 A1–H12。"));
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


// --- AsterDoc document store -------------------------------------------------

const ASTERDOC_SCHEMA_VERSION: u64 = 1;
const ASTERDOC_COMPILER_VERSION: &str = "asterdoc-core-0.1.0";

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
    let next = apply_asterdoc_patch(&source, &command)?;
    save_asterdoc_version(db, &json!({
        "design_id": design_id,
        "source": next,
        "expected_revision": expected,
        "operation_id": input.get("operation_id"),
        "actor_id": input.get("actor_id")
    }))
}

struct AsterBlock {
    body_start: usize,
    body_end: usize,
    fence_start: usize,
    fence_end: usize,
    value: serde_yaml::Value,
}

fn asterdoc_blocks(source: &str) -> Result<Vec<AsterBlock>, CliError> {
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

fn yaml_id(value: &serde_yaml::Value) -> String {
    value
        .get("id")
        .and_then(|id| id.as_str())
        .unwrap_or("")
        .to_owned()
}

fn yaml_kind(value: &serde_yaml::Value) -> String {
    value
        .get("kind")
        .and_then(|kind| kind.as_str())
        .unwrap_or("")
        .to_owned()
}

fn append_asterdoc_module(source: &str, module: &serde_yaml::Value) -> Result<String, CliError> {
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

fn apply_replacements(mut source: String, mut replacements: Vec<(usize, usize, String)>) -> String {
    replacements.sort_by(|a, b| b.0.cmp(&a.0));
    for (start, end, text) in replacements {
        if start <= end && end <= source.len() {
            source.replace_range(start..end, &text);
        }
    }
    source
}

fn apply_asterdoc_patch(source: &str, command: &Value) -> Result<String, CliError> {
    let kind = command.get("type").and_then(Value::as_str)
        .ok_or_else(|| CliError::input("command.type is required"))?;
    let module_id = command.get("moduleId").and_then(Value::as_str).unwrap_or("");
    if kind == "add_step" {
        let module = command.get("module").cloned().unwrap_or(Value::Null);
        if module.is_null() {
            return Err(CliError::input("add_step needs module"));
        }
        let module_yaml: serde_yaml::Value = serde_json::from_value(module)
            .map_err(|e| CliError::input(format!("module is invalid: {e}")))?;
        let step_id = yaml_id(&module_yaml);
        if step_id.is_empty() {
            return Err(CliError::input("step needs an id"));
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
    if kind == "remove_step" {
        let blocks = asterdoc_blocks(source)?;
        let index = blocks.iter().position(|block| yaml_id(&block.value) == module_id)
            .ok_or_else(|| CliError::missing(format!("step {module_id} does not exist")))?;
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

fn validate_asterdoc_references(modules: &[serde_yaml::Value]) -> Result<(), CliError> {
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

fn parse_asterdoc_header(
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

const ASTERDOC_MODULE_SPEC: &str = include_str!(
    "../prototypes/mygo-run/internal/asterdoc/spec/modules.json"
);

fn asterdoc_spec() -> &'static Value {
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
const ASTERDOC_RULES: &[&str] = &[
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
];

fn validate_asterdoc_spec(spec: &Value) -> Result<(), String> {
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

const ASTERDOC_UNITS: &str = include_str!(
    "../prototypes/mygo-run/internal/asterdoc/spec/units.json"
);

fn asterdoc_units() -> &'static Value {
    use std::sync::OnceLock;
    static UNITS: OnceLock<Value> = OnceLock::new();
    UNITS.get_or_init(|| {
        serde_json::from_str(ASTERDOC_UNITS)
            .expect("embedded AsterDoc unit registry must be valid JSON")
    })
}

fn asterdoc_unit(symbol: &str) -> Option<(String, f64)> {
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

fn asterdoc_map_get<'a>(value: &'a serde_yaml::Value, key: &str) -> Option<&'a serde_yaml::Value> {
    value
        .as_mapping()?
        .get(serde_yaml::Value::String(key.to_owned()))
}

fn asterdoc_quantity(value: &serde_yaml::Value, dimension: &str) -> Result<(), CliError> {
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

fn asterdoc_quantity_parts(value: &serde_yaml::Value) -> Result<(f64, String, String, f64), CliError> {
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

fn asterdoc_artifact(value: &serde_yaml::Value) -> Result<(), CliError> {
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

fn validate_asterdoc_port(kind: &str, input: &Value, value: &serde_yaml::Value) -> Result<(), CliError> {
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

fn validate_asterdoc_list_item(kind: &str, input: &Value, item: &serde_yaml::Value) -> Result<(), CliError> {
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

fn asterdoc_module_spec(kind: &str, schema: i64) -> Option<&'static Value> {
    asterdoc_spec()["modules"].as_array()?.iter().find(|module| {
        module["kind"].as_str() == Some(kind) && module["schema"].as_i64() == Some(schema)
    })
}

fn asterdoc_supported() -> String {
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

fn plate_geometry(format: i64) -> Option<(i64, i64)> {
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

fn role_allowed_for_module(role: &str, spec: &Value) -> bool {
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

fn validate_asterdoc_module(body: &str, line_no: usize) -> Result<(), CliError> {
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

fn parse_asterdoc_modules(source: &str) -> Result<Vec<serde_yaml::Value>, CliError> {
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

fn ytext(module: &serde_yaml::Value, key: &str) -> String {
    module
        .get(key)
        .and_then(|value| value.as_str())
        .map(str::trim)
        .unwrap_or("")
        .to_owned()
}

fn ystrings(module: &serde_yaml::Value, key: &str) -> Vec<String> {
    module
        .get(key)
        .and_then(|value| value.as_sequence())
        .into_iter()
        .flatten()
        .filter_map(|value| value.as_str().map(str::to_owned))
        .collect()
}

fn ywells(module: &serde_yaml::Value, group: &str) -> Vec<String> {
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

fn compile_modules_to_design(modules: &[serde_yaml::Value]) -> Result<Value, CliError> {
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

fn parse_well_coordinate(well: &str) -> Option<usize> {
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
        "protocol_step_path":event.protocol_step_path,
        "protocol_version_id":event.protocol_version_id,
        "assay_id":event.assay_id,
        "planned_output_id":event.planned_output_id,
        "task_key":event.task_key,
        "sample_ids":event.sample_ids,
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
fn merge_design_checks(run_data: &Value, frozen: &mut Value, step_path: &str) {
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

fn frozen_step_for_event(run_data: &Value, event: &ExecutionEventCreate) -> Option<Value> {
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
fn refresh_run_status_in_transaction(tx: &Transaction<'_>, run_id: &str) -> Result<(), CliError> {
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
    design_version_id: Option<&'a str>,
}

impl<'a> DbComponentContext<'a> {
    fn new(db: &'a Connection, project_id: &'a str) -> Self {
        Self {
            db,
            project_id,
            design_version_id: None,
        }
    }

    fn with_design(
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

    fn design_versions(&self) -> Vec<Value> {
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

    fn scope_exists(&self, protocol_version_id: &str, scope: &ProtocolScope) -> bool {
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

fn latest_view_proposal(
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

fn find_slot_position(value: &Value, instance_id: &str) -> Option<(String, String, usize)> {
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

fn operation_changes(
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

fn view_design_version(document: &ViewDocument) -> Result<Option<String>, CliError> {
    let mut found = std::collections::BTreeSet::new();
    fn walk(component: &ComponentInstance, found: &mut std::collections::BTreeSet<String>) {
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
    #[serde(default)]
    row: Option<usize>,
    #[serde(default)]
    column: Option<usize>,
    #[serde(default)]
    group: Option<String>,
    #[serde(default)]
    replicate: Option<usize>,
    #[serde(default)]
    status: Option<String>,
    #[serde(default)]
    field: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventReadout {
    #[serde(default)]
    readout_id: Option<String>,
    endpoint: String,
    #[serde(default)]
    value: Option<f64>,
    #[serde(default)]
    qualifier: Option<String>,
    #[serde(default)]
    unit: Option<String>,
    #[serde(default)]
    sample_id: Option<String>,
    #[serde(default = "raw_result_kind")]
    result_kind: String,
    #[serde(default)]
    method: Option<String>,
    #[serde(default)]
    derived_from: Vec<String>,
}
fn raw_result_kind() -> String { "raw".into() }
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventCheck {
    check_id: String,
    status: String,
    #[serde(default)]
    note: Option<String>,
    #[serde(default)]
    checked_by: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventControl {
    role: String,
    #[serde(default)]
    subtype: Option<String>,
    status: String,
    #[serde(default)]
    control_id: Option<String>,
    #[serde(default)]
    sample_id: Option<String>,
    #[serde(default)]
    experimental_unit: Option<String>,
    #[serde(default)]
    expected: Option<String>,
    #[serde(default)]
    observed: Option<String>,
    #[serde(default)]
    note: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventActualParameter {
    key: String,
    value: Value,
    #[serde(default)]
    unit: Option<String>,
    #[serde(default)]
    source: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct EventLineage {
    from_kind: String,
    from_id: String,
    to_kind: String,
    to_id: String,
    relation: String,
    #[serde(default)]
    note: Option<String>,
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
    task_key: Option<String>,
    #[serde(default)]
    sample_ids: Vec<String>,
    /// Sample-level attempt for one (step, sample) identity. Omitted for
    /// step-level or legacy events; the read model then derives the attempt.
    #[serde(default)]
    attempt: Option<u32>,
    #[serde(default)]
    materials: Vec<EventMaterialUse>,
    #[serde(default)]
    measurements: Vec<EventMeasurement>,
    #[serde(default)]
    batch_id: Option<String>,
    #[serde(default)]
    schema_id: Option<String>,
    #[serde(default)]
    schema_version: Option<u32>,
    #[serde(default)]
    result_cells: Vec<Value>,
    #[serde(default)]
    readouts: Vec<EventReadout>,
    #[serde(default)]
    checks: Vec<EventCheck>,
    #[serde(default)]
    controls: Vec<EventControl>,
    #[serde(default)]
    lineage: Vec<EventLineage>,
    #[serde(default)]
    actual_parameters: Vec<EventActualParameter>,
    #[serde(default)]
    operator: Option<String>,
    #[serde(default)]
    deviation: Option<String>,
    #[serde(default)]
    operation_id: Option<String>,
    #[serde(default)]
    operation_payload: Option<Value>,
    // Internal marker for the native SaveTaskChecks/CompleteTask path. The
    // public execution_event.create schema deliberately does not expose it.
    #[serde(default)]
    check_ack: bool,
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
    let mut value =
        serde_json::to_value(workspace).map_err(|error| CliError::internal(error.to_string()))?;
    value["creation_audit"] = run["data"]
        .get("creation_audit")
        .cloned()
        .unwrap_or(Value::Null);
    value["asterdoc"] = run["data"]
        .get("asterdoc")
        .cloned()
        .unwrap_or(Value::Null);
    value["design_checks"] = run["data"]
        .get("design_checks")
        .cloned()
        .unwrap_or_else(|| json!([]));
    Ok(value)
}

fn inspect_workflow_tree(input: &InspectTreeInput, data_dir: &Path) -> Result<Value, CliError> {
    let db = open_db(data_dir)?;
    workflow_tree_for_db(
        &db,
        Some(&input.design_id),
        input.revision,
        input.run_id.as_deref(),
    )
}

/// workflow_tree_for_db builds the read-only hierarchy projection for a
/// Design or a frozen Run. Native services call this directly so the GUI and
/// CLI share one Core-driven implementation.
pub(crate) fn workflow_tree_for_db(
    db: &Connection,
    design_id: Option<&str>,
    requested_revision: Option<u64>,
    run_id: Option<&str>,
) -> Result<Value, CliError> {
    let (design_id, revision, source, tasks, design) = if let Some(run_id) = run_id {
        let workspace = run_workspace(db, run_id)?;
        let frozen_design_id = workspace["design_id"]
            .as_str()
            .ok_or_else(|| CliError::internal("Run 缺少设计 ID"))?
            .to_owned();
        if let Some(requested) = design_id {
            if frozen_design_id != requested {
                return Err(CliError::input("design_id 必须等于 Run 的所属设计"));
            }
        }
        let revision = workspace["asterdoc"]["revision"]
            .as_u64()
            .filter(|revision| *revision > 0)
            .or_else(|| workspace["design_revision"].as_u64())
            .ok_or_else(|| CliError::internal("Run 缺少冻结 AsterDoc 版本号"))?;
        if let Some(requested) = requested_revision {
            if requested != revision {
                return Err(CliError::conflict(format!(
                    "Run 冻结的 AsterDoc revision 为 {revision}，请求为 {requested}"
                )));
            }
        }
        let document = load_asterdoc_at(db, &frozen_design_id, revision)?;
        let frozen_hash = workspace["asterdoc"]["source_hash"].as_str().unwrap_or("");
        let stored_hash = document["source_hash"].as_str().unwrap_or("");
        if !frozen_hash.is_empty() && !stored_hash.is_empty() && frozen_hash != stored_hash {
            return Err(CliError::conflict(format!(
                "Run 冻结的 AsterDoc source_hash 与 revision v{revision} 不一致"
            )));
        }
        let source = document["source"].as_str().unwrap_or("").to_owned();
        let tasks = workflow_tasks_from_workspace(&workspace);
        (frozen_design_id, revision, source, tasks, false)
    } else {
        let design_id = design_id.ok_or_else(|| CliError::input("design_id 必填"))?;
        let document = if let Some(revision) = requested_revision {
            load_asterdoc_at(db, design_id, revision)?
        } else {
            load_asterdoc(db, design_id)?
        };
        let revision = document["revision"].as_u64().unwrap_or(0);
        let source = document["source"].as_str().unwrap_or("").to_owned();
        (design_id.to_owned(), revision, source, Vec::new(), true)
    };

    let document = parse_workflow_document(&source)?;
    let blocks = load_workflow_blocks(db, &document)?;
    let mut value = if design {
        asterlab_core::workflow_tree::build_design(&document, &blocks)
    } else {
        asterlab_core::workflow_tree::build_run(&document, &blocks, &tasks)
    };
    value["designId"] = json!(design_id);
    value["revision"] = json!(revision);
    if let Some(run_id) = run_id {
        value["runId"] = json!(run_id);
    }
    Ok(value)
}

fn parse_workflow_document(source: &str) -> Result<Document, CliError> {
    let mut document = Document::default();
    let mut workflow_seen = false;
    for block in asterdoc_blocks(source)? {
        let value = &block.value;
        let kind = value.get("kind").and_then(|value| value.as_str()).unwrap_or("");
        match kind {
            "reference" => {
                let view = value.get("view").and_then(|value| value.as_str()).unwrap_or("");
                if view == "protocol_block" {
                    let id = value.get("id").and_then(|value| value.as_str()).unwrap_or("");
                    let reference = value.get("ref").and_then(|value| value.as_str()).unwrap_or("");
                    if !id.is_empty() && !reference.is_empty() {
                        document
                            .reference_blocks
                            .insert(id.to_owned(), reference.to_owned());
                    }
                }
            }
            "step" => {
                let id = value.get("id").and_then(|value| value.as_str()).unwrap_or("");
                if id.is_empty() {
                    continue;
                }
                let schema = value.get("schema").and_then(|value| value.as_i64()).unwrap_or(0);
                let execution = value
                    .get("execution")
                    .and_then(|value| value.as_str())
                    .unwrap_or("");
                let organizational = schema == 3 && execution == "organizational";
                let mut path = value
                    .get("path")
                    .and_then(|value| value.as_str())
                    .unwrap_or("")
                    .to_owned();
                if organizational && path.is_empty() {
                    path = id.to_owned();
                }
                let mut refs = Vec::new();
                if let Some(uses) = value.get("uses").and_then(|value| value.as_sequence()) {
                    for item in uses {
                        if let Some(reference) =
                            item.get("ref").and_then(|value| value.as_str())
                        {
                            if !reference.is_empty() {
                                refs.push(reference.to_owned());
                            }
                        }
                    }
                }
                document.steps.push(DesignStep {
                    id: id.to_owned(),
                    title: value
                        .get("title")
                        .and_then(|value| value.as_str())
                        .unwrap_or("")
                        .to_owned(),
                    path,
                    order: value.get("order").and_then(|value| value.as_i64()).unwrap_or(0),
                    organizational,
                    refs,
                });
            }
            "workflow" if !workflow_seen => {
                workflow_seen = true;
                if let Some(order) = value
                    .get("business_order")
                    .and_then(|value| value.as_sequence())
                {
                    document.order = order
                        .iter()
                        .filter_map(|value| value.as_str().map(str::to_owned))
                        .collect();
                }
                if let Some(edges) = value.get("edges").and_then(|value| value.as_sequence()) {
                    for edge in edges {
                        let from = edge.get("from").and_then(|value| value.as_str()).unwrap_or("");
                        let to = edge.get("to").and_then(|value| value.as_str()).unwrap_or("");
                        if !from.is_empty() && !to.is_empty() {
                            document.edges.push((from.to_owned(), to.to_owned()));
                        }
                    }
                }
            }
            _ => {}
        }
    }
    Ok(document)
}

fn load_workflow_blocks(
    db: &Connection,
    document: &Document,
) -> Result<HashMap<String, Result<BlockDefinition, String>>, CliError> {
    let mut versions = HashSet::new();
    for step in &document.steps {
        for reference in &step.refs {
            if let Some(version) = document.reference_blocks.get(reference) {
                versions.insert(version.clone());
            }
        }
    }
    let mut blocks = HashMap::new();
    for version in versions {
        let parsed = block_preview(db, &version, &[])
            .map_err(|error| error.message)
            .and_then(workflow_block_definition);
        blocks.insert(version, parsed);
    }
    Ok(blocks)
}

fn workflow_block_definition(value: Value) -> Result<BlockDefinition, String> {
    let steps = value
        .get("steps")
        .and_then(Value::as_array)
        .ok_or_else(|| "ProtocolPreview 缺少 steps".to_owned())?;
    let mut definition = BlockDefinition::default();
    for step in steps {
        let parameters = step
            .get("parameters")
            .and_then(Value::as_array)
            .map(|parameters| {
                parameters
                    .iter()
                    .map(|parameter| BlockParameter {
                        key: parameter
                            .get("slot")
                            .and_then(Value::as_str)
                            .unwrap_or("")
                            .to_owned(),
                        label: parameter
                            .get("label")
                            .and_then(Value::as_str)
                            .unwrap_or("")
                            .to_owned(),
                        value: parameter.get("value").cloned().unwrap_or(Value::Null),
                        unit: parameter
                            .get("unit")
                            .and_then(Value::as_str)
                            .map(str::to_owned),
                    })
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();
        let unresolved_parameters = step
            .get("unresolved_parameter_refs")
            .and_then(Value::as_array)
            .map(|values| {
                values
                    .iter()
                    .filter_map(|value| value.as_str().map(str::to_owned))
                    .collect()
            })
            .unwrap_or_default();
        definition.steps.push(BlockStep {
            use_id: step
                .get("use_id")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            path: step
                .get("path")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            title: step
                .get("title")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            instruction: step
                .get("instruction")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            source_block_version_id: step
                .get("source_block_version_id")
                .and_then(Value::as_str)
                .unwrap_or("")
                .to_owned(),
            parameters,
            unresolved_parameters,
        });
    }
    Ok(definition)
}

fn workflow_tasks_from_workspace(workspace: &Value) -> Vec<RunTask> {
    let mut step_details: HashMap<(String, String), (String, String)> = HashMap::new();
    for step in workspace["steps"].as_array().into_iter().flatten() {
        let inner = &step["step"];
        let path = inner["path"].as_str().unwrap_or("").to_owned();
        let assay_id = step["scope"]["assay_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        let title = inner["title"].as_str().unwrap_or("").to_owned();
        let source = inner["source_block_version_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        step_details.insert((path, assay_id), (title, source));
    }
    let mut protocol_versions: HashMap<String, String> = HashMap::new();
    for scope in workspace["scopes"].as_array().into_iter().flatten() {
        let assay_id = scope["id"].as_str().unwrap_or("").to_owned();
        let version = scope["protocol_version_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        if !assay_id.is_empty() {
            protocol_versions.insert(assay_id, version);
        }
    }
    let global_protocol = workspace["protocol_version_id"]
        .as_str()
        .unwrap_or("")
        .to_owned();
    let mygo = &workspace["frozen_design"]["mygo"];
    let step_tasks = mygo["stepTasks"].as_array().cloned().unwrap_or_default();
    let explicit_links = mygo
        .as_object()
        .is_some_and(|mygo| mygo.contains_key("stepTasks"));
    let mut step_blocks: HashMap<String, Vec<String>> = HashMap::new();
    for (step, blocks) in mygo["stepBlocks"]
        .as_object()
        .into_iter()
        .flat_map(|blocks| blocks.iter())
    {
        let versions = blocks
            .as_array()
            .into_iter()
            .flatten()
            .filter_map(|value| value.as_str().map(str::to_owned))
            .collect::<Vec<_>>();
        if !versions.is_empty() {
            step_blocks.insert(step.clone(), versions);
        }
    }
    let mut steps_by_assay: HashMap<String, Vec<String>> = HashMap::new();
    for (step, assay) in mygo["stepAssays"]
        .as_object()
        .into_iter()
        .flat_map(|assays| assays.iter())
    {
        if let Some(assay) = assay.as_str().filter(|assay| !assay.is_empty()) {
            steps_by_assay
                .entry(assay.to_owned())
                .or_default()
                .push(step.clone());
        }
    }
    let mut paths_per_pair: HashMap<(String, String), HashSet<String>> = HashMap::new();
    for task in workspace["tasks"].as_array().into_iter().flatten() {
        let assay = task["scope"]["assay_id"].as_str().unwrap_or("");
        if assay.is_empty() {
            continue;
        }
        paths_per_pair
            .entry((
                assay.to_owned(),
                task["timepoint"].as_str().unwrap_or("").to_owned(),
            ))
            .or_default()
            .insert(task["step_path"].as_str().unwrap_or("").to_owned());
    }

    let mut tasks = Vec::new();
    for task in workspace["tasks"].as_array().into_iter().flatten() {
        let path = task["step_path"].as_str().unwrap_or("").to_owned();
        let assay_id = task["scope"]["assay_id"]
            .as_str()
            .unwrap_or("")
            .to_owned();
        let timepoint = task["timepoint"].as_str().unwrap_or("").to_owned();
        let (title, source) = step_details
            .get(&(path.clone(), assay_id.clone()))
            .cloned()
            .unwrap_or_default();
        let design_step_id = if explicit_links {
            let pair = step_tasks
                .iter()
                .filter(|link| {
                    link["assayId"].as_str() == Some(assay_id.as_str())
                        && link["timepoint"].as_str() == Some(timepoint.as_str())
                })
                .collect::<Vec<_>>();
            let by_path = pair
                .iter()
                .copied()
                .filter(|link| link["path"].as_str() == Some(path.as_str()))
                .collect::<Vec<_>>();
            let mut chosen = if by_path.len() == 1 {
                by_path[0]["stepId"].as_str().map(str::to_owned)
            } else if by_path.is_empty() {
                let pair_paths = paths_per_pair
                    .get(&(assay_id.clone(), timepoint.clone()))
                    .map(HashSet::len)
                    .unwrap_or(0);
                let unqualified = pair
                    .iter()
                    .copied()
                    .filter(|link| {
                        link.get("path")
                            .and_then(Value::as_str)
                            .map(str::is_empty)
                            .unwrap_or(true)
                    })
                    .collect::<Vec<_>>();
                if pair_paths == 1 && unqualified.len() == 1 {
                    unqualified[0]["stepId"].as_str().map(str::to_owned)
                } else {
                    None
                }
            } else {
                None
            };
            if chosen.is_none() && !source.is_empty() {
                let mut candidates = step_blocks
                    .iter()
                    .filter(|(_, blocks)| blocks.iter().any(|block| block == &source))
                    .map(|(step, _)| step.clone())
                    .collect::<Vec<_>>();
                candidates.sort_unstable();
                if candidates.len() == 1 {
                    chosen = candidates.pop();
                }
            }
            chosen.unwrap_or_default()
        } else {
            steps_by_assay
                .get(&assay_id)
                .filter(|steps| steps.len() == 1)
                .map(|steps| steps[0].clone())
                .unwrap_or_default()
        };
        tasks.push(RunTask {
            key: task["task_key"].as_str().unwrap_or("").to_owned(),
            path,
            title: task["title"]
                .as_str()
                .map(str::to_owned)
                .unwrap_or_else(|| title.clone()),
            status: task["status"].as_str().unwrap_or("").to_owned(),
            protocol_version_id: protocol_versions
                .get(&assay_id)
                .cloned()
                .unwrap_or_else(|| global_protocol.clone()),
            source_block_version_id: source,
            assay_id,
            timepoint,
            design_step_id,
        });
    }
    tasks
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
    if let Some(question) = &d.research_question {
        if question.chars().count() > 5000 {
            return Err(CliError::input("研究问题不能超过 5000 个字符"));
        }
    }
    if d.factors.len() > 50 {
        return Err(CliError::input("因素数量最多 50 个"));
    }
    let mut factor_names = std::collections::HashSet::new();
    for factor in &d.factors {
        check_text(&factor.name, "因素名称", 120)?;
        if !factor_names.insert(factor.name.trim().to_lowercase()) {
            return Err(CliError::input(format!("因素名称重复：{}", factor.name)));
        }
        if factor.levels.is_empty() {
            return Err(CliError::input("因素必须至少有一个水平"));
        }
        if factor.levels.len() > 50 {
            return Err(CliError::input("每个因素的水平数量最多 50 个"));
        }
        for level in &factor.levels {
            check_text(level, "因素水平", 500)?;
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
pub fn data_dir() -> Result<PathBuf, CliError> {
    let path = if let Ok(v) = std::env::var("LABWORKBENCH_DATA_DIR") {
        if v.trim().is_empty() {
            return Err(CliError::internal("LABWORKBENCH_DATA_DIR is empty"));
        }
        PathBuf::from(v.trim())
    } else if let Ok(base) = std::env::var("LOCALAPPDATA") {
        PathBuf::from(base).join("AsterLab").join("data")
    } else if cfg!(not(windows)) {
        PathBuf::from(std::env::var("HOME").map_err(|e| CliError::internal(e.to_string()))?)
            .join(".local/share/AsterLab/data")
    } else {
        return Err(CliError::internal("LOCALAPPDATA is unavailable"));
    };
    if !path.is_absolute() {
        return Err(CliError::internal("data directory must be an absolute path"));
    }
    ensure_data_dirs(&path).map_err(|e| CliError::internal(e.to_string()))?;
    log_data_dir_once(&path);
    Ok(path)
}

fn ensure_data_dirs(data_dir: &Path) -> std::io::Result<()> {
    fs::create_dir_all(data_dir)?;
    if let Some(parent) = data_dir.parent() {
        fs::create_dir_all(parent.join("logs"))?;
        fs::create_dir_all(parent.join("cache"))?;
        fs::create_dir_all(parent.join("backups"))?;
    }
    Ok(())
}

fn log_data_dir_once(data_dir: &Path) {
    use std::sync::atomic::{AtomicBool, Ordering};
    static LOGGED: AtomicBool = AtomicBool::new(false);
    if !LOGGED.swap(true, Ordering::Relaxed) {
        eprintln!("asterlab data dir: {}", data_dir.display());
    }
}

pub fn db_path(data_dir: &Path) -> PathBuf {
    data_dir.join("labworkbench.sqlite")
}


fn migrate_records_schema(db: &mut Connection) -> Result<(), CliError> {
    let tx = db
        .transaction_with_behavior(TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let user_version: i64 = tx
        .query_row("PRAGMA user_version", [], |row| row.get(0))
        .map_err(|error| CliError::internal(error.to_string()))?;
    if user_version > RECORDS_SCHEMA_VERSION {
        return Err(CliError::internal(format!(
            "database schema version {user_version} is newer than supported version {RECORDS_SCHEMA_VERSION}"
        )));
    }
    if user_version < 0 {
        return Err(CliError::internal(format!(
            "unsupported database schema version {user_version}"
        )));
    }
    if user_version == RECORDS_SCHEMA_VERSION {
        tx.commit()
            .map_err(|error| CliError::internal(error.to_string()))?;
        return Ok(());
    }

    let records_exists: bool = tx
        .query_row(
            "SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='records')",
            [],
            |row| row.get(0),
        )
        .map_err(|error| CliError::internal(error.to_string()))?;
    if !records_exists {
        tx.execute_batch(
            "CREATE TABLE records(\
                id TEXT PRIMARY KEY,\
                kind TEXT NOT NULL,\
                project_id TEXT REFERENCES records(id),\
                parent_id TEXT REFERENCES records(id) ON DELETE RESTRICT,\
                revision INTEGER NOT NULL DEFAULT 1,\
                data_json TEXT NOT NULL,\
                created_at TEXT NOT NULL,\
                updated_at TEXT NOT NULL\
            );",
        )
        .map_err(|error| CliError::internal(error.to_string()))?;
    } else {
        let columns = {
            let mut statement = tx
                .prepare("PRAGMA table_info(records)")
                .map_err(|error| CliError::internal(error.to_string()))?;
            let columns = statement
                .query_map([], |row| row.get::<_, String>(1))
                .map_err(|error| CliError::internal(error.to_string()))?
                .collect::<rusqlite::Result<Vec<_>>>()
                .map_err(|error| CliError::internal(error.to_string()))?;
            columns
        };
        for required in [
            "id",
            "kind",
            "parent_id",
            "revision",
            "data_json",
            "created_at",
            "updated_at",
        ] {
            if !columns.iter().any(|column| column == required) {
                return Err(CliError::internal(format!(
                    "unsupported legacy records schema: missing {required} column"
                )));
            }
        }
        if !columns.iter().any(|column| column == "project_id") {
            tx.execute_batch(
                r#"ALTER TABLE records ADD COLUMN project_id TEXT REFERENCES records(id);
                 WITH RECURSIVE project_ancestors(record_id, ancestor_id, kind, parent_id) AS (
                     SELECT id, id, kind, parent_id FROM records
                     UNION
                     SELECT ancestors.record_id, parent.id, parent.kind, parent.parent_id
                     FROM project_ancestors AS ancestors
                     JOIN records AS parent ON parent.id=ancestors.parent_id
                     WHERE ancestors.kind <> 'project'
                 )
                 UPDATE records
                 SET project_id=(
                     SELECT ancestor_id FROM project_ancestors
                     WHERE record_id=records.id AND kind='project' LIMIT 1
                 )
                 WHERE kind <> 'project';"#,
            )
            .map_err(|error| CliError::internal(error.to_string()))?;
        }
    }

    tx.execute_batch(
        "CREATE INDEX IF NOT EXISTS records_kind_parent ON records(kind,parent_id);\
         CREATE INDEX IF NOT EXISTS records_project ON records(project_id);",
    )
    .map_err(|error| CliError::internal(error.to_string()))?;
    tx.pragma_update(None, "user_version", RECORDS_SCHEMA_VERSION)
        .map_err(|error| CliError::internal(error.to_string()))?;
    tx.commit()
        .map_err(|error| CliError::internal(error.to_string()))?;
    Ok(())
}

pub(crate) fn open_db(path: &Path) -> Result<Connection, CliError> {
    let data_dir = if path.extension().map(|e| e == "sqlite").unwrap_or(false) {
        path.parent().unwrap_or(path).to_path_buf()
    } else {
        path.to_path_buf()
    };
    ensure_data_dirs(&data_dir).map_err(|e| CliError::internal(e.to_string()))?;
    let db_file = data_dir.join("labworkbench.sqlite");
    let mut db = Connection::open(&db_file).map_err(|e| CliError::internal(e.to_string()))?;
    db.busy_timeout(std::time::Duration::from_secs(10))
        .map_err(|e| CliError::internal(e.to_string()))?;
    db.pragma_update(None, "foreign_keys", "ON")
        .map_err(|e| CliError::internal(e.to_string()))?;
    migrate_records_schema(&mut db)?;

    // WAL is a persistent database property. A concurrent opener may hold the
    // exclusive lock needed to change it; keep the existing journal mode rather
    // than failing the whole connection. Writes remain transactional.
    if let Err(error) = db.pragma_update(None, "journal_mode", "WAL") {
        eprintln!(
            "asterlab: WAL not enabled for {} ({error}); continuing with existing journal mode",
            db_file.display()
        );
    }
    log_data_dir_once(&data_dir);
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

fn take_mygo_extension(value: &mut Value) -> Option<Value> {
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
    let mut error = json!({"code":e.code,"message":e.message});
    if let Some(path) = e.path {
        error["path"] = json!(path);
    }
    if let Some(current_version) = e.current_version {
        error["current_version"] = json!(current_version);
    }
    if let Some(current_version_id) = e.current_version_id {
        error["current_version_id"] = json!(current_version_id);
    }
    if !e.issues.is_empty() {
        error["issues"] = Value::Array(e.issues);
    }
    println!("{}", json!({"ok":false,"tool":tool,"error":error}));
    e.exit
}
fn print_help() {
    println!("AsterLab typed CLI\n\n  labworkbench tools --json\n  labworkbench schema <tool>\n  labworkbench <tool> --input <json-file|->\n\nAll tool inputs use a strict per-command schema; unknown fields are rejected. All output is one JSON envelope on stdout. Data directory follows LABWORKBENCH_DATA_DIR (default: ./data).");
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

/// Read-only record view shared by the validation and commit adapters.
struct CliEditStore<'a> {
    conn: &'a Connection,
}

fn edit_protocol_head(
    conn: &Connection,
    protocol_id: &str,
) -> Result<Option<StoredProtocolVersion>, String> {
    let row = conn
        .query_row(
            "SELECT id,revision,data_json FROM records \
             WHERE kind='protocol_version' AND parent_id=?1 \
             ORDER BY revision DESC LIMIT 1",
            [protocol_id],
            |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, i64>(1)?,
                    row.get::<_, String>(2)?,
                ))
            },
        )
        .optional()
        .map_err(|error| error.to_string())?;
    let Some((version_id, revision, raw)) = row else {
        return Ok(None);
    };
    let data: Value = serde_json::from_str(&raw)
        .map_err(|error| format!("protocol_version {version_id} data_json 无效：{error}"))?;
    let root_block_version_id = data
        .get("root_block_version_id")
        .and_then(Value::as_str)
        .map(str::to_owned);
    let root_bindings = match data.get("root_bindings") {
        None | Some(Value::Null) => Vec::new(),
        Some(value) => serde_json::from_value::<Vec<StepBinding>>(value.clone())
            .map_err(|error| format!("protocol_version {version_id} root_bindings 无效：{error}"))?,
    };
    Ok(Some(StoredProtocolVersion {
        version_id,
        protocol_id: protocol_id.to_owned(),
        revision,
        root_block_version_id,
        root_bindings,
    }))
}

fn edit_block_version(
    conn: &Connection,
    version_id: &str,
) -> Result<Option<StoredBlockVersion>, String> {
    let row = conn
        .query_row(
            "SELECT revision,data_json FROM records \
             WHERE kind='protocol_block_version' AND id=?1",
            [version_id],
            |row| Ok((row.get::<_, i64>(0)?, row.get::<_, String>(1)?)),
        )
        .optional()
        .map_err(|error| error.to_string())?;
    let Some((revision, raw)) = row else {
        return Ok(None);
    };
    let data: ProtocolBlockVersionData = serde_json::from_str(&raw).map_err(|error| {
        format!("protocol_block_version {version_id} data_json 无效：{error}")
    })?;
    Ok(Some(StoredBlockVersion {
        version_id: version_id.to_owned(),
        revision,
        data,
    }))
}

impl EditTargetStore for CliEditStore<'_> {
    fn protocol_head(
        &self,
        protocol_id: &str,
    ) -> Result<Option<StoredProtocolVersion>, String> {
        edit_protocol_head(self.conn, protocol_id)
    }

    fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String> {
        edit_block_version(self.conn, version_id)
    }
}

/// Transactional commit view: every read and the append go through one
/// `BEGIN IMMEDIATE` transaction so a concurrent proposal cannot slip between
/// the head re-read and the insert.
struct CliCommitStore<'a, 'conn> {
    tx: &'a Transaction<'conn>,
}

impl EditTargetStore for CliCommitStore<'_, '_> {
    fn protocol_head(
        &self,
        protocol_id: &str,
    ) -> Result<Option<StoredProtocolVersion>, String> {
        edit_protocol_head(self.tx, protocol_id)
    }

    fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String> {
        edit_block_version(self.tx, version_id)
    }
}

impl CommitStore for CliCommitStore<'_, '_> {
    fn edit_store(&self) -> &dyn EditTargetStore {
        self
    }

    fn protocol_version_data(&self, version_id: &str) -> Result<Option<Value>, String> {
        let row = self
            .tx
            .query_row(
                "SELECT data_json FROM records WHERE kind='protocol_version' AND id=?1",
                [version_id],
                |row| row.get::<_, String>(0),
            )
            .optional()
            .map_err(|error| error.to_string())?;
        row.map(|raw| serde_json::from_str(&raw).map_err(|error| error.to_string()))
            .transpose()
    }

    fn find_commit(&self, idempotency_key: &str) -> Result<Option<CommittedVersion>, String> {
        let row = self
            .tx
            .query_row(
                "SELECT id,revision,data_json FROM records \
                 WHERE kind IN ('protocol_version','protocol_block_version') \
                   AND json_extract(data_json,'$.agent_edit.idempotency_key')=?1 \
                 ORDER BY created_at DESC, rowid DESC LIMIT 1",
                [idempotency_key],
                |row| {
                    Ok((
                        row.get::<_, String>(0)?,
                        row.get::<_, i64>(1)?,
                        row.get::<_, String>(2)?,
                    ))
                },
            )
            .optional()
            .map_err(|error| error.to_string())?;
        let Some((version_id, revision, raw)) = row else {
            return Ok(None);
        };
        let data: Value = serde_json::from_str(&raw).map_err(|error| error.to_string())?;
        Ok(Some(CommittedVersion {
            version_id,
            revision,
            provenance: data["agent_edit"].clone(),
        }))
    }

    fn timestamp(&self) -> String {
        stamp()
    }

    fn append_protocol_version(
        &mut self,
        protocol_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String> {
        let (kind, project, _, _, _) = kind_id(self.tx, protocol_id)
            .map_err(|error| error.message)?
            .ok_or_else(|| format!("Protocol 不存在：{protocol_id}"))?;
        if kind != "protocol" {
            return Err(format!("{protocol_id} 不是 protocol 记录"));
        }
        let record = insert(
            self.tx,
            "protocol_version",
            project.as_deref(),
            Some(protocol_id),
            revision,
            &data,
        )
        .map_err(|error| error.message)?;
        Ok((
            record["id"].as_str().unwrap_or_default().to_owned(),
            record["revision"].as_i64().unwrap_or(revision),
        ))
    }

    fn append_block_version(
        &mut self,
        block_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String> {
        let (kind, project, _, _, _) = kind_id(self.tx, block_id)
            .map_err(|error| error.message)?
            .ok_or_else(|| format!("步骤块不存在：{block_id}"))?;
        if kind != "protocol_block" {
            return Err(format!("{block_id} 不是 protocol_block 记录"));
        }
        let record = insert(
            self.tx,
            "protocol_block_version",
            project.as_deref(),
            Some(block_id),
            revision,
            &data,
        )
        .map_err(|error| error.message)?;
        Ok((
            record["id"].as_str().unwrap_or_default().to_owned(),
            record["revision"].as_i64().unwrap_or(revision),
        ))
    }
}

/// Read-only `agent_edit.validate`: resolve one proposal against the current
/// head. Semantic rejections stay in the result (`status = blocked`).
pub(crate) fn validate_agent_edit(db: &Connection, input: Value) -> Result<Value, CliError> {
    let proposal: Proposal = serde_json::from_value(input).map_err(|error| {
        CliError::input(format!(
            "proposal 不是合法的 agent edit v1 载荷：{error}"
        ))
    })?;
    let result = validate_agent_edit_request(&proposal, &CliEditStore { conn: db })
        .map_err(CliError::internal)?;
    serde_json::to_value(result).map_err(|error| CliError::internal(error.to_string()))
}

/// `agent_edit.commit`: append one confirmed version in a single write
/// transaction. A stale or concurrent proposal returns `VERSION_CONFLICT` and
/// writes nothing.
pub(crate) fn commit_agent_edit(db: &mut Connection, input: Value) -> Result<Value, CliError> {
    let request: ConfirmRequest = serde_json::from_value(input).map_err(|error| {
        CliError::input(format!(
            "commit 载荷不符合 agent edit v1 ConfirmRequest：{error}"
        ))
    })?;
    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|error| CliError::internal(error.to_string()))?;
    let outcome = {
        let mut store = CliCommitStore { tx: &tx };
        commit_agent_edit_request(&request, &mut store)
    };
    match outcome {
        Ok(result) => {
            tx.commit()
                .map_err(|error| CliError::internal(error.to_string()))?;
            serde_json::to_value(result).map_err(|error| CliError::internal(error.to_string()))
        }
        Err(CommitError::Rejected(error)) => {
            tx.rollback().ok();
            Err(agent_edit_cli_error(error))
        }
        Err(CommitError::Store(message)) => {
            tx.rollback().ok();
            Err(CliError::internal(message))
        }
    }
}

fn agent_edit_cli_error(error: AgentEditError) -> CliError {
    let code = match error.code {
        AgentEditErrorCode::InvalidProposal => "INVALID_PROPOSAL",
        AgentEditErrorCode::InvalidBinding => "INVALID_BINDING",
        AgentEditErrorCode::UnitMismatch => "UNIT_MISMATCH",
        AgentEditErrorCode::StaleReference => "STALE_REFERENCE",
        AgentEditErrorCode::ForbiddenField => "FORBIDDEN_FIELD",
        AgentEditErrorCode::VersionConflict => "VERSION_CONFLICT",
        AgentEditErrorCode::ConfirmationRequired => "CONFIRMATION_REQUIRED",
        AgentEditErrorCode::ConfirmationMismatch => "CONFIRMATION_MISMATCH",
        AgentEditErrorCode::IdempotencyMismatch => "IDEMPOTENCY_MISMATCH",
    };
    let exit = match error.code {
        AgentEditErrorCode::VersionConflict => 4,
        AgentEditErrorCode::StaleReference => 3,
        _ => 2,
    };
    let issues = error
        .issues
        .into_iter()
        .filter_map(|issue| serde_json::to_value(issue).ok())
        .collect();
    CliError {
        code,
        message: error.message,
        exit,
        path: error.path,
        current_version: error.current_version,
        current_version_id: error.current_version_id,
        issues,
    }
}

fn dispatch(tool: &str, path: &str, data_dir: &Path) -> Result<Value, CliError> {
    if tool == "workflow.inspect_tree" {
        let input: InspectTreeInput = parse_input(path)?;
        return inspect_workflow_tree(&input, data_dir);
    }
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
    let mut db = open_db(data_dir)?;
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
        "asterdoc.load" => {
            let input: Value = parse_input(path)?;
            let design_id = input.get("design_id").or_else(|| input.get("designId"))
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            load_asterdoc(&db, design_id)
        }
        "asterdoc.save" => {
            let input: Value = parse_input(path)?;
            save_asterdoc_version(&mut db, &input)
        }
        "asterdoc.command" => {
            let input: Value = parse_input(path)?;
            apply_asterdoc_command(&mut db, &input)
        }
        "asterdoc.compile" => {
            let input: Value = parse_input(path)?;
            let design_id = input.get("design_id").and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            let revision = input.get("revision").and_then(Value::as_u64)
                .ok_or_else(|| CliError::input("revision is required"))?;
            let doc = load_asterdoc_at(&db, design_id, revision)?;
            let design = compile_asterdoc_to_design(doc["source"].as_str().unwrap_or(""))?;
            Ok(json!({"design": design, "document": doc}))
        }
        "asterdoc.create_run" => {
            let input: Value = parse_input(path)?;
            let design_id = input.get("design_id").and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            let revision = input.get("revision").and_then(Value::as_u64)
                .ok_or_else(|| CliError::input("revision is required"))?;
            let op = input.get("operation_id").and_then(Value::as_str);
            let title = input.get("title").and_then(Value::as_str);
            let acknowledged: Vec<String> = input.get("acknowledged_warnings")
                .and_then(Value::as_array).into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect();
            create_run_from_asterdoc(&mut db, design_id, revision, op, title, &acknowledged)
        }
        "sdk.modules" => {
            let _: EmptyInput = parse_input(path)?;
            let modules = asterdoc_spec()["modules"].as_array().cloned().unwrap_or_default();
            let summaries: Vec<Value> = modules
                .iter()
                .map(|module| {
                    json!({
                        "kind": module.get("kind").cloned().unwrap_or(Value::Null),
                        "schema": module.get("schema").cloned().unwrap_or(Value::Null),
                        "title": module.get("title").cloned().unwrap_or(Value::Null),
                        "category": module.get("category").cloned().unwrap_or(Value::Null),
                        "capabilities": module.get("capabilities").cloned().unwrap_or(Value::Null),
                        "renderers": module.get("renderers").cloned().unwrap_or(Value::Null),
                        "actions": module.get("actions").cloned().unwrap_or(Value::Null)
                    })
                })
                .collect();
            Ok(json!({"modules": summaries}))
        }
        "sdk.describe" => {
            let input: Value = parse_input(path)?;
            let kind = input
                .get("kind")
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("kind is required"))?;
            let schema = input
                .get("schema")
                .and_then(Value::as_i64)
                .ok_or_else(|| CliError::input("schema is required"))?;
            let module = asterdoc_module_spec(kind, schema)
                .ok_or_else(|| CliError::missing(format!("unknown module {kind}@{schema}")))?;
            Ok(json!({"module": module}))
        }
        "sdk.validate" => {
            let input: Value = parse_input(path)?;
            let source = if let Some(source) = input.get("source").and_then(Value::as_str) {
                source.to_owned()
            } else {
                let design_id = input
                    .get("design_id")
                    .and_then(Value::as_str)
                    .ok_or_else(|| CliError::input("source or design_id is required"))?;
                let revision = input.get("revision").and_then(Value::as_u64);
                let doc = if let Some(revision) = revision {
                    load_asterdoc_at(&db, design_id, revision)?
                } else {
                    load_asterdoc(&db, design_id)?
                };
                doc.get("source").and_then(Value::as_str).unwrap_or("").to_owned()
            };
            let mut errors: Vec<String> = Vec::new();
            let mut warnings: Vec<String> = Vec::new();
            let valid = validate_asterdoc_source(&source).is_ok();
            if let Err(error) = validate_asterdoc_source(&source) {
                errors.push(error.message);
            }
            let mut compiled = false;
            let mut design_value = Value::Null;
            if valid {
                match compile_asterdoc_to_design(&source) {
                    Ok(design) => {
                        compiled = true;
                        design_value = design;
                    }
                    Err(error) => {
                        compiled = false;
                        errors.push(error.message);
                    }
                }
            }
            let modules: Vec<Value> = if valid {
                parse_asterdoc_modules(&source)
                    .unwrap_or_default()
                    .iter()
                    .map(|m| { let mj = serde_json::to_value(m).unwrap_or(Value::Null); json!({"kind": mj.get("kind").cloned().unwrap_or(Value::Null), "schema": mj.get("schema").cloned().unwrap_or(Value::Null)}) })
                    .collect()
            } else {
                Vec::new()
            };
            let mut out = json!({
                "valid": valid,
                "compiled": compiled,
                "errors": errors,
                "warnings": warnings,
                "modules": modules
            });
            if compiled {
                out["design"] = design_value;
            }
            Ok(out)
        }
        "asterdoc.inspect" => {
            let input: Value = parse_input(path)?;
            let design_id = input
                .get("design_id")
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("design_id is required"))?;
            let revision = input.get("revision").and_then(Value::as_u64);
            let doc = if let Some(revision) = revision {
                load_asterdoc_at(&db, design_id, revision)?
            } else {
                load_asterdoc(&db, design_id)?
            };
            let module_id = input.get("module_id").and_then(Value::as_str);
            let source = doc.get("source").and_then(Value::as_str).unwrap_or("");
            let modules = parse_asterdoc_modules(source)?;
            let mut inspected = Vec::new();
            for module in &modules {
                let kind = module.get("kind").and_then(|v| v.as_str()).unwrap_or("").to_owned();
                let id = module.get("id").and_then(|v| v.as_str()).unwrap_or("").to_owned();
                if let Some(filter) = module_id {
                    if id != filter {
                        continue;
                    }
                }
                let schema = module.get("schema").and_then(|v| v.as_i64()).unwrap_or(0);
                let spec = asterdoc_module_spec(&kind, schema);
                let manifest = spec.map(|spec_value| {
                    json!({
                        "renderers": spec_value.get("renderers").cloned().unwrap_or(Value::Null),
                        "actions": spec_value.get("actions").cloned().unwrap_or(Value::Null)
                    })
                });
                let data = serde_json::to_value(module).unwrap_or(Value::Null);
                inspected.push(json!({
                    "kind": kind,
                    "id": id,
                    "schema": schema,
                    "data": data,
                    "manifest": manifest.unwrap_or(Value::Null)
                }));
            }
            if let Some(filter) = module_id {
                if inspected.is_empty() {
                    return Err(CliError::missing(format!("module {filter} not found")));
                }
            }
            Ok(json!({
                "design_id": design_id,
                "revision": doc.get("revision").cloned().unwrap_or(Value::Null),
                "source_hash": doc.get("source_hash").cloned().unwrap_or(Value::Null),
                "modules": inspected
            }))
        }
        "asterdoc.assess" => {
            let input: Value = parse_input(path)?;
            assess_asterdoc(&db, &input)
        }

        "asterdoc.validate" => {
            let input: Value = parse_input(path)?;
            let source = input
                .get("source")
                .and_then(Value::as_str)
                .ok_or_else(|| CliError::input("source is required"))?;
            if let Err(error) = validate_asterdoc_source(source) {
                return Ok(json!({
                    "valid": false,
                    "compiled": false,
                    "error": {"code": error.code, "message": error.message}
                }));
            }
            match compile_asterdoc_to_design(source) {
                Ok(design) => Ok(json!({
                    "valid": true,
                    "compiled": true,
                    "design": design
                })),
                Err(error) => Ok(json!({
                    "valid": true,
                    "compiled": false,
                    "error": {"code": error.code, "message": error.message}
                })),
            }
        }
        "run.workspace" => {
            let i: IdInput = parse_input(path)?;
            Ok(run_workspace(&db, &i.id)?)
        }
        "workflow.progress" => {
            let i: WorkflowProgressInput = parse_input(path)?;
            if i.run_id.trim().is_empty() {
                return Err(CliError::input("run_id 不能为空"));
            }
            let workspace: asterlab_core::run_workspace::RunWorkspace =
                serde_json::from_value(run_workspace(&db, &i.run_id)?)
                    .map_err(|e| CliError::internal(e.to_string()))?;
            let query = asterlab_core::sample_progress::SampleProgressQuery {
                step_path: i.step_path,
                sample_id: i.sample_id,
                sample_ids: i.sample_ids,
            };
            serde_json::to_value(asterlab_core::sample_progress::build_from_workspace(
                &workspace, &query,
            ))
            .map_err(|e| CliError::internal(e.to_string()))
        }
        "run.create" => {
            require_trusted_writer(tool)?;
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
                i.view_id.as_deref(),
                i.repeat_of_run_id.as_deref(),
                i.repeat_relationship.as_str(),
                None,
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
        "view.proposal.create" => {
            let i: ViewProposalCreate = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let changeset = serde_json::to_value(&i.changeset)
                .map_err(|e| CliError::internal(e.to_string()))?;
            let result = create_view_proposal_tx(&tx, &i.view_id, &changeset, &i.source)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.proposal.list" => {
            let i: ViewProposalList = parse_input(path)?;
            list_view_proposals(&db, &i.view_id)
        }
        "view.proposal.accept" => {
            let i: ViewProposalAction = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let result = accept_view_proposal_tx(&tx, &i.view_id, &i.proposal_id)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
        }
        "view.proposal.reject" => {
            let i: ViewProposalAction = parse_input(path)?;
            let tx = db
                .unchecked_transaction()
                .map_err(|e| CliError::internal(e.to_string()))?;
            let result = reject_view_proposal_tx(&tx, &i.view_id, &i.proposal_id)?;
            tx.commit().map_err(|e| CliError::internal(e.to_string()))?;
            Ok(result)
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
        "agent_edit.validate" => {
            let i: Value = parse_input(path)?;
            validate_agent_edit(&db, i)
        }
        "agent_edit.commit" => {
            let i: Value = parse_input(path)?;
            commit_agent_edit(&mut db, i)
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
                    "asset_link",
                    "asset_link_event",
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
        "asset.link" => {
            let i: AssetLinkCreate = parse_input(path)?;
            let (record, event) = link_local_asset(&mut db, &i)?;
            Ok(json!({"record":record,"event":event}))
        }
        "asset.unlink" => {
            let i: AssetUnlinkInput = parse_input(path)?;
            let (removed, event) =
                unlink_asset_link(&mut db, &i.id, i.actor_id.as_deref().unwrap_or(""))?;
            Ok(json!({"removed":removed,"event":event}))
        }
        "asset.list" => {
            let i: AssetListInput = parse_input(path)?;
            Ok(json!({"records":list_asset_links(&db,&i.run_id)?}))
        }
        "asset.events" => {
            let i: AssetEventsInput = parse_input(path)?;
            Ok(json!({"records":list_asset_events(&db,&i.run_id,i.action.as_deref())?}))
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
fn asset_link_payload_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["id","rootId","relativePath","mediaType","size","modTime","sha256","availability","link","linkedAt","actorId"],
        "properties":{
            "id":{"type":"string"},
            "rootId":{"type":"string"},
            "relativePath":{"type":"string"},
            "mediaType":{"type":"string","enum":["image","table","document","binary","unknown"]},
            "size":{"type":"integer","minimum":0},
            "modTime":{"type":"string"},
            "sha256":{"type":"string","maxLength":64},
            "availability":{"type":"string","enum":["available","unavailable"]},
            "link":{"type":"object","additionalProperties":false,"required":["runId","taskKey","sampleId"],"properties":{"runId":{"type":"string"},"taskKey":{"type":"string"},"sampleId":{"type":["string","null"]}}},
            "linkedAt":{"type":"string"},
            "actorId":{"type":"string"}
        }
    })
}
fn asset_link_event_payload_schema() -> Value {
    json!({
        "type":"object","additionalProperties":false,
        "required":["id","action","assetId","rootId","relativePath","mediaType","size","modTime","sha256","availability","link","at","actorId"],
        "properties":{
            "id":{"type":"string"},
            "action":{"type":"string","enum":["linked","unlinked"]},
            "assetId":{"type":"string"},
            "rootId":{"type":"string"},
            "relativePath":{"type":"string"},
            "mediaType":{"type":"string","enum":["image","table","document","binary","unknown"]},
            "size":{"type":"integer","minimum":0},
            "modTime":{"type":"string"},
            "sha256":{"type":"string","maxLength":64},
            "availability":{"type":"string","enum":["available","unavailable"]},
            "link":{"type":"object","additionalProperties":false,"required":["runId","taskKey","sampleId"],"properties":{"runId":{"type":"string"},"taskKey":{"type":"string"},"sampleId":{"type":["string","null"]}}},
            "at":{"type":"string"},
            "actorId":{"type":"string"}
        }
    })
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
            json!({"type":"object","additionalProperties":false,"required":["title","scheduled_date","protocol_version_id","design_version_id","design_revision","status"],"properties":{"title":{"type":"string"},"scheduled_date":{"type":["string","null"],"format":"date"},"protocol_version_id":{"type":["string","null"]},"design_version_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"status":{"type":"string","enum":["planned","in_progress","completed","blocked","cancelled"]},"run_index":{"type":"integer","minimum":1},"repeat_of_run_id":{"type":["string","null"]},"repeat_relationship":{"type":"string","enum":["independent","same_source","paired","unknown"]},"container_plan":{"type":"object","additionalProperties":true},"labware_profiles":{"type":"array","items":{"type":"object","additionalProperties":true}},"material_manifest":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["planned_output_id","title","evidence_id","producer_protocol","consumers"],"properties":{"planned_output_id":{"type":"string"},"title":{"type":"string"},"evidence_id":{"type":["string","null"]},"producer_protocol":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","protocol_version_id","protocol_id","protocol_title","protocol_version","root_block_version_id","block_version_ids","root_bindings","protocol_ready","steps","resolved_parameters","unresolved_parameters"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"assay_id":{"type":["string","null"]},"assay_title":{"type":"string"},"protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},"protocol_title":{"type":"string"},"protocol_version":{"type":"integer","minimum":1},"root_block_version_id":{"type":["string","null"]},"block_version_ids":{"type":"array","items":{"type":"string"}},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","unit"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"protocol_ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object"}},"resolved_parameters":{"type":"array","items":{"type":"object"}},"unresolved_parameters":{"type":"array","items":{"type":"object"}}}},"consumers":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","group_id","group_name"],"properties":{"assay_id":{"type":"string"},"assay_title":{"type":"string"},"group_id":{"type":["string","null"]},"group_name":{"type":"string"}}}}}}},"protocol_manifest":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["assay_id","assay_title","protocol_version_id","protocol_id","protocol_title","protocol_version"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"assay_id":{"type":["string","null"]},"assay_title":{"type":"string"},"protocol_version_id":{"type":"string"},"protocol_id":{"type":"string"},"protocol_title":{"type":"string"},"protocol_version":{"type":"integer","minimum":1},"root_block_version_id":{"type":["string","null"]},"block_version_ids":{"type":"array","items":{"type":"string"}},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","unit"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"protocol_ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object"}},"resolved_parameters":{"type":"array","items":{"type":"object"}},"unresolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","unit","required"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}}}}}}})
        }
        "task" => {
            json!({"type":"object","additionalProperties":false,"required":["title","scheduled_date","status","notes"],"properties":{"title":{"type":"string"},"scheduled_date":{"type":"string","format":"date"},"status":{"type":"string","enum":["planned","in_progress","done","blocked"]},"notes":{"type":"string"}}})
        }
        "protocol" => {
            json!({"type":"object","additionalProperties":false,"required":["title"],"properties":{"title":{"type":"string"}}})
        }
        "protocol_version" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol_id","version","body"],"properties":{"protocol_id":{"type":"string"},"version":{"type":"integer","minimum":1},"body":{"type":"string"},"root_block_version_id":{"type":["string","null"]},"root_bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value"],"properties":{"slot":{"type":"string"},"value":{},"unit":{"type":["string","null"]}}}},"agent_edit":{"type":["object","null"],"additionalProperties":true}}})
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
            json!({"type":"object","additionalProperties":false,"required":["block_id","version","summary","parameter_slots","steps"],"properties":{"block_id":{"type":"string"},"version":{"type":"integer","minimum":1},"summary":{"type":"string"},"module_contract":{"type":["object","null"]},"module_origin":{"type":["string","null"],"enum":["imported","derived","local_custom",null]},"module_contract_authority":{"type":["string","null"],"enum":["current","source_only",null]},"catalog_review":{"type":["object","null"]},"parameter_slots":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["key","label","value_type","unit","required"],"properties":{"key":{"type":"string"},"label":{"type":"string"},"value_type":{"type":"string","enum":["text","number","integer","boolean"]},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}},"steps":{"type":"array","items":{"oneOf":[{"type":"object","additionalProperties":false,"required":["kind","use_id","title","instruction","parameter_refs"],"properties":{"kind":{"const":"text"},"use_id":{"type":"string"},"title":{"type":"string"},"instruction":{"type":"string"},"parameter_refs":{"type":"array","items":{"type":"string"}},"visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]},"checks":{"type":"array","items":{"type":"object","additionalProperties":true}}}},{"type":"object","additionalProperties":false,"required":["kind","use_id","block_version_id","bindings"],"properties":{"kind":{"const":"block_use"},"use_id":{"type":"string"},"block_version_id":{"type":"string"},"bindings":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","value","source_slot","unit"],"properties":{"slot":{"type":"string"},"value":{},"source_slot":{"type":["string","null"]},"unit":{"type":["string","null"]}}}}}}]}}}})
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
            json!({"type":"object","additionalProperties":false,"required":["event_type","content","occurred_at","stage","timepoint","step_status","sample_ids","materials","measurements","deviation"],"properties":{"event_type":{"type":"string","enum":["operation","material_use","measurement","deviation","observation"]},"content":{"type":"string"},"occurred_at":{"type":["string","null"],"format":"date-time"},"stage":{"type":["string","null"]},"timepoint":{"type":["string","null"],"maxLength":100},"step_status":{"type":["string","null"],"enum":["in_progress","completed","blocked","skipped",null]},"protocol_step_path":{"type":["string","null"]},"protocol_version_id":{"type":["string","null"]},"assay_id":{"type":["string","null"]},"planned_output_id":{"type":["string","null"]},"sample_ids":{"type":"array","items":{"type":"string"}},"materials":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["material","planned_output_id","amount","unit"],"properties":{"material":{"type":"string"},"planned_output_id":{"type":["string","null"]},"amount":{"type":["number","null"]},"unit":{"type":["string","null"]}}}},"measurements":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["endpoint","value","unit","sample_id"],"properties":{"endpoint":{"type":"string"},"value":{"type":"number"},"unit":{"type":["string","null"]},"sample_id":{"type":["string","null"]}}}},"readouts":{"type":"array","items":{"type":"object","additionalProperties":true}},"checks":{"type":"array","items":{"type":"object","additionalProperties":true}},"controls":{"type":"array","items":{"type":"object","additionalProperties":true}},"lineage":{"type":"array","items":{"type":"object","additionalProperties":true}},"actual_parameters":{"type":"array","items":{"type":"object","additionalProperties":true}},"operator":{"type":["string","null"]},"deviation":{"type":["string","null"]}}})
        }
        "formula" => {
            json!({"type":"object","additionalProperties":false,"required":["title","expression","variables","notes"],"properties":{"title":{"type":"string"},"expression":{"type":"string"},"variables":{"type":"string"},"notes":{"type":"string"}}})
        }
        "library_table" => {
            json!({"type":"object","additionalProperties":false,"required":["name","columns","rows"],"properties":{"name":{"type":"string"},"columns":{"type":"array","items":{"type":"string"}},"rows":{"type":"array","items":{"type":"array","items":{"type":"string"}}}}})
        }
        "asset_link" => asset_link_payload_schema(),
        "asset_link_event" => asset_link_event_payload_schema(),
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
        "asset_link",
        "asset_link_event",
    ];
    json!({"oneOf":kinds.iter().map(|k|record_schema(k)).collect::<Vec<_>>()})
}
fn workflow_progress_output_schema() -> Value {
    let status = json!({"type":"string","enum":["not_started","in_progress","completed","failed","skipped"]});
    let attempt = json!({
        "type":"object","additionalProperties":false,
        "required":["attempt","status","event_ids","last_event_at","deviation"],
        "properties":{
            "attempt":{"type":"integer","minimum":1},
            "status":status.clone(),
            "event_ids":{"type":"array","items":{"type":"string"}},
            "last_event_at":{"type":["string","null"]},
            "deviation":{"type":"boolean"}
        }
    });
    let sample = json!({
        "type":"object","additionalProperties":false,
        "required":["sample_id","status","attempt","attempt_count","last_event_at","attempts"],
        "properties":{
            "sample_id":{"type":"string"},
            "status":status.clone(),
            "attempt":{"type":["integer","null"],"minimum":1},
            "attempt_count":{"type":"integer","minimum":0},
            "last_event_at":{"type":["string","null"]},
            "attempts":{"type":"array","items":attempt}
        }
    });
    json!({
        "type":"object","additionalProperties":false,
        "required":["schema_version","run_id","sample_ids","steps","issues"],
        "properties":{
            "schema_version":{"type":"integer","minimum":1},
            "run_id":{"type":"string"},
            "sample_ids":{"type":"array","items":{"type":"string"}},
            "steps":{"type":"array","items":{
                "type":"object","additionalProperties":false,
                "required":["step_path","status","complete","total_samples","completed_samples","failed_samples","skipped_samples","in_progress_samples","not_started_samples","last_event_at","samples"],
                "properties":{
                    "step_path":{"type":"string"},
                    "status":status,
                    "complete":{"type":"boolean"},
                    "total_samples":{"type":"integer","minimum":0},
                    "completed_samples":{"type":"integer","minimum":0},
                    "failed_samples":{"type":"integer","minimum":0},
                    "skipped_samples":{"type":"integer","minimum":0},
                    "in_progress_samples":{"type":"integer","minimum":0},
                    "not_started_samples":{"type":"integer","minimum":0},
                    "last_event_at":{"type":["string","null"]},
                    "samples":{"type":"array","items":sample}
                }
            }},
            "issues":{"type":"array","items":{
                "type":"object","additionalProperties":false,
                "required":["code","message","step_path","sample_id","event_id"],
                "properties":{
                    "code":{"type":"string"},
                    "message":{"type":"string"},
                    "step_path":{"type":["string","null"]},
                    "sample_id":{"type":["string","null"]},
                    "event_id":{"type":["string","null"]}
                }
            }}
        }
    })
}
fn result_schema(tool: &str) -> Value {
    let arr = |kind: &str| json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":record_schema(kind)}}});
    let asterdoc_version = || json!({"type":"object","additionalProperties":true,"required":["design_id","revision","source","source_hash","schema_version","compiler_version"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":0},"source":{"type":"string"},"source_hash":{"type":"string"},"schema_version":{"type":"integer","minimum":1},"compiler_version":{"type":"string"},"design_revision":{"type":"integer","minimum":0},"operation_id":{"type":["string","null"]},"actor_id":{"type":["string","null"]},"created_at":{"type":["string","null"]}}});
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
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision","title","objective","research_question","factors","experimental_unit","groups","assays","sample_links","planned_outputs","conflicts","unresolved"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"title":{"type":"string"},"objective":{"type":"string"},"research_question":{"type":["string","null"]},"factors":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["name","levels"],"properties":{"name":{"type":"string"},"levels":{"type":"array","items":{"type":"string"}}}}},"experimental_unit":{"type":["string","null"]},"groups":{"type":"array","items":{"type":"object","additionalProperties":true}},"assays":{"type":"array","items":{"type":"object","additionalProperties":true}},"sample_links":{"type":"array","items":{"type":"object","additionalProperties":true}},"planned_outputs":{"type":"array","items":{"type":"object","additionalProperties":true}},"conflicts":{"type":"array","items":{"type":"object","additionalProperties":true}},"unresolved":{"type":"array","items":{"type":"object","additionalProperties":true}}}})
        }
        "design.revise" => record_and_version("design"),
        "run.list" => arr("run"),
        "run.create" => one("run"),
        "run.set_status" => one("run"),
        "run.revise" => one("run"),
        "run.workspace" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id","design_id","design_version_id","design_revision","frozen_design","protocol_version_id","status","scopes","steps","tasks","occurrences","materials","plates","events"],"properties":{"run_id":{"type":"string"},"design_id":{"type":"string"},"design_version_id":{"type":"string"},"design_revision":{"type":"integer","minimum":1},"frozen_design":{"type":"object","additionalProperties":true},"protocol_version_id":{"type":["string","null"]},"status":{"type":"string"},"scopes":{"type":"array","items":{"type":"object","additionalProperties":true}},"steps":{"type":"array","items":{"type":"object","additionalProperties":true}},"tasks":{"type":"array","items":{"type":"object","additionalProperties":true}},"occurrences":{"type":"array","items":{"type":"object","additionalProperties":true}},"materials":{"type":"array","items":{"type":"object","additionalProperties":true}},"plates":{"type":"array","items":{"type":"object","additionalProperties":true}},"events":{"type":"array","items":{"type":"object","additionalProperties":true}}}})
        }
        "workflow.inspect_tree" => json!({"type":"object","additionalProperties":false,"required":["designId","revision","design","roots","order","precedes","issues"],"properties":{"designId":{"type":"string"},"revision":{"type":"integer","minimum":0},"runId":{"type":"string"},"design":{"type":"boolean"},"roots":{"type":"array","items":{"type":"object","additionalProperties":true}},"order":{"type":"array","items":{"type":"string"}},"precedes":{"type":"object","additionalProperties":{"type":"array","items":{"type":"string"}}},"issues":{"type":"array","items":{"type":"string"}}}}),
        "workflow.progress" => workflow_progress_output_schema(),
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
            json!({"type":"object","additionalProperties":false,"required":["block_version_id","block_id","title","version","ready","steps","resolved_parameters","unresolved_parameters","block_version_ids"],"properties":{"module_contracts":{"type":"array","items":{"type":"object","additionalProperties":true}},"block_version_id":{"type":"string"},"block_id":{"type":"string"},"title":{"type":"string"},"version":{"type":"integer"},"ready":{"type":"boolean"},"steps":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","use_id","title","instruction","parameters","unresolved_parameter_refs","source_block_version_id"],"properties":{"path":{"type":"string"},"use_id":{"type":"string"},"title":{"type":"string"},"instruction":{"type":"string"},"parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["slot","label","value","unit","provenance"],"properties":{"slot":{"type":"string"},"label":{"type":"string"},"value":{},"unit":{"type":["string","null"]},"provenance":{"type":"array","items":{"type":"object"}}}}},"unresolved_parameter_refs":{"type":"array","items":{"type":"string"}},"source_block_version_id":{"type":"string"},"visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]},"checks":{"type":"array","items":{"type":"object","additionalProperties":true}}}}},"resolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","value","unit","provenance"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"value":{},"unit":{"type":["string","null"]},"provenance":{"type":"array","items":{"type":"object"}}}}},"unresolved_parameters":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["path","slot","label","unit","required"],"properties":{"path":{"type":"string"},"slot":{"type":"string"},"label":{"type":"string"},"unit":{"type":["string","null"]},"required":{"type":"boolean"}}}},"block_version_ids":{"type":"array","items":{"type":"string"}}}})
        }
        "protocol.create" => {
            json!({"type":"object","additionalProperties":false,"required":["protocol","version"],"properties":{"protocol":record_schema("protocol"),"version":record_schema("protocol_version")}})
        }
        "protocol.revise" => {
            json!({"type":"object","additionalProperties":false,"required":["version"],"properties":{"version":record_schema("protocol_version")}})
        }
        "agent_edit.validate" => {
            json!({"type":"object","additionalProperties":false,"required":["schema_version","proposal_id","proposal_digest","target","current_version_id","current_revision","status","diffs","issues","invalidated_refs","requires_confirmation"],"properties":{"schema_version":{"type":"integer"},"proposal_id":{"type":"string"},"proposal_digest":{"type":"string"},"target":{"type":"object","additionalProperties":true},"current_version_id":{"type":"string"},"current_revision":{"type":"integer"},"status":{"type":"string","enum":["valid","needs_confirmation","blocked"]},"diffs":{"type":"array","items":{"type":"object","additionalProperties":true}},"issues":{"type":"array","items":{"type":"object","additionalProperties":true}},"invalidated_refs":{"type":"array","items":{"type":"string"}},"requires_confirmation":{"type":"boolean"},"confirmation_prompt":{"type":["string","null"]}}})
        }
        "agent_edit.commit" => {
            json!({"type":"object","additionalProperties":false,"required":["schema_version","proposal_id","proposal_digest","target","previous_version_id","new_version_id","new_revision","applied","invalidated_refs","confirmed_by","committed_at","idempotency_key","replayed"],"properties":{"schema_version":{"type":"integer"},"proposal_id":{"type":"string"},"proposal_digest":{"type":"string"},"target":{"type":"object","additionalProperties":true},"previous_version_id":{"type":"string"},"new_version_id":{"type":"string"},"new_revision":{"type":"integer"},"applied":{"type":"array","items":{"type":"object","additionalProperties":true}},"invalidated_refs":{"type":"array","items":{"type":"string"}},"confirmed_by":{"type":"string"},"committed_at":{"type":"string"},"idempotency_key":{"type":"string"},"replayed":{"type":"boolean"}}})
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
        "asset.link" => {
            json!({"type":"object","additionalProperties":false,"required":["record","event"],"properties":{"record":record_schema("asset_link"),"event":record_schema("asset_link_event")}})
        }
        "asset.unlink" => {
            json!({"type":"object","additionalProperties":false,"required":["removed","event"],"properties":{"removed":record_schema("asset_link"),"event":record_schema("asset_link_event")}})
        }
        "asset.list" => {
            json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":asset_link_payload_schema()}}})
        }
        "asset.events" => {
            json!({"type":"object","additionalProperties":false,"required":["records"],"properties":{"records":{"type":"array","items":record_schema("asset_link_event")}}})
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
        "sdk.modules" => json!({"type":"object","additionalProperties":false,"required":["modules"],"properties":{"modules":{"type":"array","items":{"type":"object","additionalProperties":true,"required":["kind","schema"],"properties":{"kind":{"type":"string"},"schema":{"type":"integer","minimum":1},"title":{"type":["string","null"]},"category":{"type":["string","null"]},"capabilities":{"type":["array","null"]},"renderers":{"type":["object","null"]},"actions":{"type":["array","null"]}}}}}}),
        "sdk.validate" => json!({"type":"object","additionalProperties":false,"required":["valid","compiled","errors","warnings","modules"],"properties":{"valid":{"type":"boolean"},"compiled":{"type":"boolean"},"errors":{"type":"array","items":{"type":"string"}},"warnings":{"type":"array","items":{"type":"string"}},"modules":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","schema"],"properties":{"kind":{},"schema":{}}}},"design":{"type":"object","additionalProperties":true}}}),
        "asterdoc.inspect" => json!({"type":"object","additionalProperties":false,"required":["design_id","revision","source_hash","modules"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":0},"source_hash":{"type":"string"},"modules":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["kind","id","schema","data","manifest"],"properties":{"kind":{"type":"string"},"id":{"type":"string"},"schema":{"type":"integer","minimum":0},"data":{"type":"object","additionalProperties":true},"manifest":{"type":["object","null"],"additionalProperties":true}}}}}}),
        "asterdoc.assess" => json!({"type":"object","additionalProperties":false,"required":["design_id","revision","source_hash","state","errors","warnings"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":0},"source_hash":{"type":"string"},"state":{"type":"string","enum":["ready","warning","incomplete"]},"errors":{"type":"array","items":{"type":"string"}},"warnings":{"type":"array","items":{"type":"string"}}}}),

        "asterdoc.validate" => json!({"type":"object","additionalProperties":false,"required":["valid","compiled"],"properties":{"valid":{"type":"boolean"},"compiled":{"type":"boolean"},"design":{"type":"object","additionalProperties":true},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string"},"message":{"type":"string"}}}}}),
        "asterdoc.load" | "asterdoc.save" | "asterdoc.command" => asterdoc_version(),
        "asterdoc.compile" => json!({"type":"object","additionalProperties":false,"required":["design","document"],"properties":{"design":{"type":"object","additionalProperties":true},"document":asterdoc_version()}}),
        "asterdoc.create_run" => record_schema("run"),
        _ => json!({"type":"object"}),
    }
}
fn output_schema(tool: &str) -> Value {
    let success = json!({"type":"object","additionalProperties":false,"required":["ok","tool","result"],"properties":{"ok":{"const":true},"tool":{"const":tool},"result":result_schema(tool)}});
    let error = if matches!(tool, "agent_edit.validate" | "agent_edit.commit") {
        json!({"type":"object","additionalProperties":false,"required":["ok","tool","error"],"properties":{"ok":{"const":false},"tool":{"const":tool},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string","enum":["INVALID_PROPOSAL","INVALID_BINDING","UNIT_MISMATCH","STALE_REFERENCE","FORBIDDEN_FIELD","VERSION_CONFLICT","CONFIRMATION_REQUIRED","CONFIRMATION_MISMATCH","IDEMPOTENCY_MISMATCH","INVALID_INPUT","NOT_FOUND","REVISION_CONFLICT","INTERNAL"]},"message":{"type":"string"},"path":{"type":["string","null"]},"current_version":{"type":["integer","null"]},"current_version_id":{"type":["string","null"]},"issues":{"type":"array","items":{"type":"object","additionalProperties":true}}}}}})
    } else {
        json!({"type":"object","additionalProperties":false,"required":["ok","tool","error"],"properties":{"ok":{"const":false},"tool":{"const":tool},"error":{"type":"object","additionalProperties":false,"required":["code","message"],"properties":{"code":{"type":"string","enum":["INVALID_INPUT","NOT_FOUND","REVISION_CONFLICT","INTERNAL"]},"message":{"type":"string"}}}}})
    };
    json!({"oneOf":[success,error]})
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
            json!({"type":"object","additionalProperties":false,"required":["view_id","instance_id"],"properties":{"view_id":{"type":"string"},"instance_id":{"type":"string"},"position_ids":{"type":"array","items":{"type":"string"}},"calculator_instance_id":{"type":"string"},"volume_per_well":{"type":["number","null"]},"volume_unit":{"type":"string"},"volume_per_well":{"type":["number","null"]},"extra_ratio":{"type":["number","null"]}}})
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
        "view.proposal.create" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","changeset"],"properties":{"view_id":{"type":"string"},"changeset":{"type":"object"},"source":{"type":"string","maxLength":120}}})
        }
        "view.proposal.list" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id"],"properties":{"view_id":{"type":"string"}}})
        }
        "view.proposal.accept" | "view.proposal.reject" => {
            json!({"type":"object","additionalProperties":false,"required":["view_id","proposal_id"],"properties":{"view_id":{"type":"string"},"proposal_id":{"type":"string"}}})
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
        "workflow.inspect_tree" => json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"run_id":{"type":"string"}}}),
        "workflow.progress" => json!({"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1},"step_path":{"type":"string","minLength":1,"maxLength":500,"description":"只看一个冻结步骤"},"sample_id":{"type":"string","minLength":1,"maxLength":120,"description":"只看一个样本"},"sample_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":120},"description":"必须出现在矩阵里、即使尚无事件的样本"}}}),
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
        "agent_edit.validate" => {
            json!({"type":"object","additionalProperties":false,"required":["proposal_id","target","base_version_id","expected_version","origin","diffs"],"properties":{"schema_version":{"type":"integer"},"proposal_id":{"type":"string"},"target":{"type":"object"},"base_version_id":{"type":"string"},"expected_version":{"type":"integer"},"origin":{"type":"object"},"diffs":{"type":"array","items":{"type":"object"}}}})
        }
        "agent_edit.commit" => {
            json!({"type":"object","additionalProperties":false,"required":["proposal","proposal_digest","idempotency_key","confirmed_by"],"properties":{"schema_version":{"type":"integer"},"proposal":{"type":"object"},"proposal_digest":{"type":"string"},"idempotency_key":{"type":"string"},"confirmed_by":{"type":"string"},"acknowledged":{"type":"array","items":{"type":"string"}},"note":{"type":["string","null"]}}})
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
            json!({"type":"object","additionalProperties":false,"required":["run_id","event_type","content"],"properties":{"run_id":{"type":"string"},"event_type":{"type":"string","enum":["operation","material_use","measurement","deviation","observation"]},"content":{"type":"string","minLength":1,"maxLength":5000,"pattern":"\\S"},"occurred_at":{"type":["string","null"],"format":"date-time"},"stage":{"type":["string","null"],"maxLength":160},"timepoint":{"type":["string","null"],"maxLength":100,"description":"检测步骤的计划时间点；必须属于Run冻结设计中的该检测"},"step_status":{"type":["string","null"],"enum":["in_progress","completed","blocked","skipped",null],"description":"显式步骤状态；普通事件不代表步骤完成"},"protocol_step_path":{"type":["string","null"],"maxLength":500},"protocol_version_id":{"type":["string","null"],"description":"关联步骤所引用的冻结 Protocol 版本"},"assay_id":{"type":["string","null"],"description":"关联步骤所属的冻结检测；共享制备步骤留空"},"planned_output_id":{"type":["string","null"],"description":"共享制备步骤所属计划产物；检测步骤留空"},"sample_ids":{"type":"array","maxItems":100,"items":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"}},"attempt":{"type":["integer","null"],"minimum":1,"maximum":1000,"description":"样本级 attempt；必须关联冻结步骤与样本，且只能是当前 attempt 或下一个 attempt"},"materials":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["material"],"properties":{"material":{"type":"string","minLength":1,"maxLength":160,"pattern":"\\S"},"planned_output_id":{"type":["string","null"]},"amount":{"type":["number","null"],"minimum":0},"unit":{"type":["string","null"],"maxLength":40}}}},"measurements":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["endpoint","value"],"properties":{"endpoint":{"type":"string","minLength":1,"maxLength":300,"pattern":"\\S"},"value":{"type":"number"},"unit":{"type":["string","null"],"maxLength":40},"sample_id":{"type":["string","null"],"maxLength":120}}}},"readouts":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["endpoint"],"properties":{"readout_id":{"type":["string","null"],"maxLength":160},"endpoint":{"type":"string","minLength":1,"maxLength":300},"value":{"type":["number","null"]},"qualifier":{"type":["string","null"],"enum":["not_detected","below_lod","above_uloq","invalid","other",null]},"unit":{"type":["string","null"],"maxLength":40},"sample_id":{"type":["string","null"],"maxLength":120},"result_kind":{"type":"string","enum":["raw","derived"]},"method":{"type":["string","null"],"maxLength":300},"derived_from":{"type":"array","maxItems":100,"items":{"type":"string","maxLength":160}}}}},"checks":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["check_id","status"],"properties":{"check_id":{"type":"string","minLength":1,"maxLength":80},"status":{"type":"string","enum":["passed","failed","blocked","skipped"]},"note":{"type":["string","null"],"maxLength":2000},"checked_by":{"type":["string","null"],"maxLength":160}}}},"controls":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["role","status"],"properties":{"role":{"type":"string","enum":["blank","negative","positive","standard","vehicle","untreated","unstained","other"]},"subtype":{"type":["string","null"],"maxLength":80},"status":{"type":"string","enum":["passed","failed","blocked","skipped","not_run"]},"control_id":{"type":["string","null"],"maxLength":500},"sample_id":{"type":["string","null"],"maxLength":500},"experimental_unit":{"type":["string","null"],"maxLength":500},"expected":{"type":["string","null"],"maxLength":500},"observed":{"type":["string","null"],"maxLength":500},"note":{"type":["string","null"],"maxLength":500}}}},"lineage":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["from_kind","from_id","to_kind","to_id","relation"],"properties":{"from_kind":{"type":"string","minLength":1,"maxLength":120},"from_id":{"type":"string","minLength":1,"maxLength":120},"to_kind":{"type":"string","minLength":1,"maxLength":120},"to_id":{"type":"string","minLength":1,"maxLength":120},"relation":{"type":"string","minLength":1,"maxLength":120},"note":{"type":["string","null"],"maxLength":500}}}},"actual_parameters":{"type":"array","maxItems":100,"items":{"type":"object","additionalProperties":false,"required":["key","value"],"properties":{"key":{"type":"string","minLength":1,"maxLength":120},"value":{},"unit":{"type":["string","null"],"maxLength":40},"source":{"type":["string","null"],"maxLength":300}}}},"operator":{"type":["string","null"],"maxLength":160},"deviation":{"type":["string","null"],"maxLength":5000}}})
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
            json!({"type":"object","additionalProperties":false,"properties":{"kind":{"type":"string","enum":["project","design","design_version","run","task","protocol","protocol_version","protocol_block","protocol_block_version","plate","well","note","execution_event","formula","library_table","asset_link","asset_link_event"]},"project_id":{"type":"string"}}})
        }
        "record.get" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string"}}})
        }
        "asset.link" => {
            json!({"type":"object","additionalProperties":false,"required":["root_id","root_dir","run_id","task_key"],"anyOf":[{"required":["relative_path"]},{"required":["path"]}],"properties":{"root_id":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S","description":"逻辑根 ID；只保存它和相对路径，不保存绝对路径"},"root_dir":{"type":"string","minLength":1,"description":"本机根目录；仅用于本次读取元数据与计算 sha256，不写入记录"},"relative_path":{"type":"string","minLength":1,"description":"相对 root_dir 的文件路径；不得包含 .. 或绝对路径"},"path":{"type":"string","minLength":1,"description":"本机文件绝对路径；与 root_dir 一起用于推导 relative_path"},"run_id":{"type":"string","minLength":1},"task_key":{"type":"string","minLength":1,"maxLength":1000},"sample_id":{"type":["string","null"],"maxLength":200},"actor_id":{"type":["string","null"],"maxLength":200}}})
        }
        "asset.unlink" => {
            json!({"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1,"description":"asset_link 记录 ID"},"actor_id":{"type":["string","null"],"maxLength":200}}})
        }
        "asset.list" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1}}})
        }
        "asset.events" => {
            json!({"type":"object","additionalProperties":false,"required":["run_id"],"properties":{"run_id":{"type":"string","minLength":1},"action":{"type":"string","enum":["linked","unlinked"]}}})
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
        "sdk.modules" => {
            json!({"type":"object","additionalProperties":false,"properties":{}})
        }
        "sdk.describe" => {
            json!({"type":"object","additionalProperties":false,"required":["kind","schema"],"properties":{"kind":{"type":"string"},"schema":{"type":"integer","minimum":1}}})
        }
        "sdk.validate" => {
            json!({"type":"object","additionalProperties":false,"properties":{"source":{"type":"string","minLength":1},"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":0}}})
        }
        "asterdoc.inspect" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string"},"revision":{"type":"integer","minimum":1},"module_id":{"type":"string"}}})
        }
        "asterdoc.assess" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1}}})
        }

        "asterdoc.validate" => {
            json!({"type":"object","additionalProperties":false,"required":["source"],"properties":{"source":{"type":"string","minLength":1}}})
        }
        "asterdoc.load" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id"],"properties":{"design_id":{"type":"string","minLength":1}}})
        }
        "asterdoc.save" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","source"],"properties":{"design_id":{"type":"string","minLength":1},"source":{"type":"string","maxLength":400000},"expected_revision":{"type":"integer","minimum":0},"operation_id":{"type":"string","minLength":1},"actor_id":{"type":"string","minLength":1}}})
        }
        "asterdoc.command" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","expected_revision","command"],"properties":{
                "design_id":{"type":"string","minLength":1},
                "expected_revision":{"type":"integer","minimum":0},
                "operation_id":{"type":"string","minLength":1},
                "actor_id":{"type":"string","minLength":1},
                "command":{"oneOf":[
                    {"type":"object","additionalProperties":false,"required":["type","module"],"properties":{"type":{"const":"add_module"},"module":{"type":"object","required":["kind","id","schema"],"properties":{"kind":{"type":"string"},"id":{"type":"string","minLength":1},"schema":{"type":"integer","minimum":1}}}}},
                    {"type":"object","additionalProperties":false,"required":["type","module"],"properties":{"type":{"const":"add_step"},"workflowId":{"type":"string"},"module":{"type":"object","required":["kind","id","schema"],"properties":{"kind":{"const":"step"},"id":{"type":"string","minLength":1},"schema":{"type":"integer","minimum":1}}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId"],"properties":{"type":{"enum":["remove_module","remove_step"]},"moduleId":{"type":"string","minLength":1}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","properties"],"properties":{"type":{"const":"update_module"},"moduleId":{"type":"string","minLength":1},"properties":{"type":"object"}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","group","wells"],"properties":{"type":{"const":"assign_wells"},"moduleId":{"type":"string","minLength":1},"group":{"type":"string","minLength":1},"wells":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","wells"],"properties":{"type":{"const":"clear_wells"},"moduleId":{"type":"string","minLength":1},"wells":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}}}},
                    {"type":"object","additionalProperties":false,"required":["type","moduleId","from","to"],"properties":{"type":{"enum":["connect_steps","disconnect_steps"]},"moduleId":{"type":"string","minLength":1},"from":{"type":"string","minLength":1},"to":{"type":"string","minLength":1}}}
                ]}
            }})
        }
        "asterdoc.compile" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision"],"properties":{"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1}}})
        }
        "asterdoc.create_run" => {
            json!({"type":"object","additionalProperties":false,"required":["design_id","revision"],"properties":{"design_id":{"type":"string","minLength":1},"revision":{"type":"integer","minimum":1},"operation_id":{"type":"string","minLength":1},"title":{"type":"string","minLength":1},"acknowledged_warnings":{"type":"array","items":{"type":"string"}}}})
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
            "research_question":{"type":["string","null"],"maxLength":5000,"description":"研究者明确提出的研究问题；未知时留空。"},
            "factors":{"type":"array","maxItems":50,"items":{"type":"object","additionalProperties":false,"required":["name","levels"],"properties":{"name":{"type":"string","minLength":1,"maxLength":120,"pattern":"\\S"},"levels":{"type":"array","minItems":1,"maxItems":50,"items":{"type":"string","minLength":1,"maxLength":500,"pattern":"\\S"}}}}},
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
            "visualization":{"type":"string","enum":["transfer","incubation","plate_layout","sample_split","measurement"]},"time_scope":{"type":"string","enum":["once","per_timepoint","per_assay","unresolved"]},
            "checks":{"type":"array","maxItems":50,"items":{"type":"object","additionalProperties":false,"required":["check_id","label"],"properties":{"check_id":{"type":"string","minLength":1,"maxLength":80,"pattern":"\\S"},"label":{"type":"string","minLength":1,"maxLength":200,"pattern":"\\S"},"instruction":{"type":"string","maxLength":2000},"blocking":{"type":"boolean"}}}}
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
        "workflow.inspect_tree",
        "workflow.progress",
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
        "agent_edit.validate",
        "agent_edit.commit",
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
        "asset.link",
        "asset.unlink",
        "asset.list",
        "asset.events",
        "import.study.preview",
        "import.study.apply",
        "calculator.dilution",
        "calculator.solution",
        "calculator.serial_dilution",
        "calculator.reaction_volume",
        "sdk.modules",
        "sdk.validate",
        "sdk.describe",
        "asterdoc.inspect",
        "asterdoc.assess",
        "asterdoc.validate",
        "asterdoc.load",
        "asterdoc.save",
        "asterdoc.command",
        "asterdoc.compile",
        "asterdoc.create_run",
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
            | "workflow.inspect_tree"
            | "workflow.progress"
            | "protocol.list"
            | "protocol.block.list"
            | "protocol.block.get"
            | "protocol.block.preview"
            | "agent_edit.validate"
            | "component.list"
            | "semantic.inspect"
            | "semantic.search"
            | "semantic.validate"
            | "semantic.propose_patch"
            | "semantic.assess_patch"
            | "record.list"
            | "record.get"
            | "asset.list"
            | "asset.events"
            | "import.study.preview"
            | "calculator.dilution"
            | "calculator.solution"
            | "calculator.serial_dilution"
            | "calculator.reaction_volume"
            | "sdk.modules"
            | "sdk.describe"
            | "sdk.validate"
            | "asterdoc.inspect"
            | "asterdoc.assess"
            | "asterdoc.validate"
            | "asterdoc.compile"
            | "asterdoc.load"
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
        "workflow.inspect_tree" => "只读投影 Design（可选 Run）的 Use/Definition/Occurrence 层级树；由 Rust Core 直接计算，不依赖桌面应用。",
        "workflow.progress" => "只读投影一个 Run 每个冻结步骤的样本矩阵：每个样本的当前状态、attempt 历史与最后事件时间；步骤完成只在全部样本完成时成立。",
        "run.create" => "从设计当前版本创建运行并固定设计/Protocol 版本引用；仅受信任的本地写入端可调用，Agent 不得自行创建 Run。",
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
        "agent_edit.validate" => "只读校验 agent 提案：解析绑定、版本和单位，返回 Core 计算的 digest、无效引用与确认提示；不写数据库。",
        "agent_edit.commit" => "提交人工确认过的 agent 提案：单事务重新校验版本并追加一个新版本；过期或并发提案返回 VERSION_CONFLICT，不覆盖旧数据。",
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
        "asset.link" => "关联本地文件到 Run 任务：只保存逻辑根、相对路径与实测 size/mtime/类型/sha256，不复制文件；同时追加 linked 事件。",
        "asset.unlink" => "解除一条 asset_link 并追加 unlinked 事件；只删除引用，不删除原文件。",
        "asset.list" => "按 Run 查询当前有效的本地文件引用；只读。",
        "asset.events" => "按 Run 查询 linked/unlinked 事件历史；只读。",
        "import.study.preview" => "校验项目/设计/Protocol 导入结构并报告待补项，不写数据库。",
        "import.study.apply" => "在单一事务中导入项目、设计快照和 Protocol v1。",
        "calculator.dilution" => "按 C1V1=C2V2 换算储液体积并校验单位。",
        "calculator.solution" => "按 m=C×V×MW 计算理论质量。",
        "calculator.serial_dilution" => "按相同体积单位计算逐级稀释浓度。",
        "calculator.reaction_volume" => "求反应已分配体积与剩余体积，不判断配方科学适用性。",
        "sdk.modules" => "从嵌入的 modules.json 返回 AsterDoc 模块摘要（kind/schema/title/category/capabilities/renderers/actions）；只读。",
        "sdk.describe" => "按 kind+schema 返回完整的 AsterDoc 模块 manifest；只读。",
        "sdk.validate" => "校验 AsterDoc 源码或已保存设计并尝试编译；不写入。",
        "asterdoc.inspect" => "读取设计的精确/当前 AsterDoc 修订，解析模块并返回 manifest 渲染器/动作契约；只读。",
        "asterdoc.validate" => "校验候选 AsterDoc 源码并尝试编译；不写入。",
        "asterdoc.assess" => "对当前或指定 AsterDoc revision 执行只读 readiness 检查，返回 errors 与 warnings。",
        "asterdoc.load" => "读取设计的最新 AsterDoc 修订；只读。",
        "asterdoc.save" => "保存新的不可变 AsterDoc 修订；expected_revision 用于并发保护，operation_id 可用于幂等重试。",
        "asterdoc.command" => "按 expected_revision 将单条语义命令应用到 AsterDoc，并保存新修订；可提供 operation_id 做幂等重试。",
        "asterdoc.compile" => "将精确 AsterDoc revision 编译为设计；只读。",
        "asterdoc.create_run" => "从指定 AsterDoc revision 创建并冻结运行。",
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
        | "workflow.inspect_tree"
        | "workflow.progress"
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
        "agent_edit.validate" => "只读；不修改数据库。",
        "agent_edit.commit" => "在一个事务中追加一个不可变版本，并记录提案 ID、确认人和版本号；不修改旧版本或冻结 Run。",
        "protocol.block.create" => "创建步骤块元数据和不可变 v1；步骤块只引用确切的其他版本。",
        "view.create" | "view.revise" => "保存新的 ViewDocument 版本；不改 Protocol/Run。",
        "view.get" => "只读；不修改数据库。",
        "view.proposal.create" => "为外部 Agent 保存一条待审阅的 View 变更提案；不写 ViewDocument。",
        "view.proposal.list" => "读取当前 View 的提案收件箱；只读。",
        "view.proposal.accept" => "接受待审阅提案并提交其冻结的 ViewChangeSet；可能产生 revision conflict。",
        "view.proposal.reject" => "拒绝待审阅提案；不改 ViewDocument。",
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
        "asset.link" => "读取本地文件元数据并流式计算 sha256；创建或刷新一条 asset_link，并追加 linked 事件；不复制文件。",
        "asset.unlink" => "删除一条 asset_link 引用并追加 unlinked 事件；不删除原文件。",
        "asset.list" | "asset.events" => "只读；不修改数据库。",
        "formula.create" => "创建一条公式记录。",
        "formula.revise" => "按 expected_revision 更新一条公式记录并递增 revision。",
        "table.create" => "创建一张资料表。",
        "table.revise" => "按 expected_revision 替换资料表的名称、列和全部数据行。",
        "import.study.apply" => {
            "在一个数据库事务中创建项目、设计及其 v1 快照、Protocol 及其 v1 版本；失败时整体回滚。"
        }
        "sdk.modules" | "sdk.describe" | "sdk.validate" | "asterdoc.inspect" | "asterdoc.assess" | "asterdoc.validate" | "asterdoc.compile" | "asterdoc.load" => "只读；不修改数据库。",
        "asterdoc.save" => "保存新的不可变 AsterDoc 修订。",
        "asterdoc.command" => "应用语义命令并保存新的不可变 AsterDoc 修订。",
        "asterdoc.create_run" => "创建运行并固定 AsterDoc 修订引用。",
        _ => "未定义副作用。",
    }
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct AssetLinkCreate {
    root_id: String,
    root_dir: String,
    #[serde(default)]
    relative_path: Option<String>,
    #[serde(default)]
    path: Option<String>,
    run_id: String,
    task_key: String,
    #[serde(default)]
    sample_id: Option<String>,
    #[serde(default)]
    actor_id: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct AssetUnlinkInput {
    id: String,
    #[serde(default)]
    actor_id: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct AssetListInput {
    run_id: String,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct AssetEventsInput {
    run_id: String,
    #[serde(default)]
    action: Option<String>,
}

fn check_asset_root_id(root_id: &str) -> Result<(), CliError> {
    if root_id.is_empty() {
        return Err(CliError::input("root_id is required"));
    }
    if root_id.len() > 200 || root_id.chars().any(char::is_control) {
        return Err(CliError::input("root_id is invalid"));
    }
    Ok(())
}

fn check_asset_sample_id(sample_id: &str) -> Result<(), CliError> {
    if sample_id.len() > 200 || sample_id.chars().any(char::is_control) {
        return Err(CliError::input("sample_id is invalid"));
    }
    Ok(())
}

/// Validates the Run/task association and returns the Run's project id.
fn asset_link_scope(db: &Connection, run_id: &str, task_key: &str) -> Result<String, CliError> {
    let (kind, project, _, _, _) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id must point to a run"));
    }
    if task_key.is_empty() || task_key.len() > 1000 || task_key.chars().any(char::is_control) {
        return Err(CliError::input("task_key is invalid"));
    }
    let workspace = run_workspace(db, run_id)?;
    let task_exists = workspace["tasks"]
        .as_array()
        .is_some_and(|tasks| tasks.iter().any(|task| task["task_key"].as_str() == Some(task_key)));
    if !task_exists {
        return Err(CliError::input("task_key does not belong to this Run"));
    }
    project.ok_or_else(|| CliError::internal("实验运行缺少所属项目"))
}

fn asset_link_record_id(
    asset: &asset_refs::AssetRef,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
) -> String {
    let mut hasher = Sha256::new();
    for part in [
        asset.root_id.as_str(),
        asset.relative_path.as_str(),
        run_id,
        task_key,
        sample_id,
    ] {
        hasher.update(part.as_bytes());
        hasher.update(b"\0");
    }
    let digest = hasher.finalize();
    let mut record_id = String::from("asset-link-");
    for byte in digest.iter().take(16) {
        record_id.push_str(&format!("{byte:02x}"));
    }
    record_id
}

fn asset_link_data(
    asset: &asset_refs::AssetRef,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    linked_at: &str,
    actor_id: &str,
) -> Value {
    json!({
        "id": asset.id,
        "rootId": asset.root_id,
        "relativePath": asset.relative_path,
        "mediaType": asset.media_type,
        "size": asset.size,
        "modTime": asset.mod_time,
        "sha256": asset.sha256,
        "availability": asset.availability,
        "link": {
            "runId": run_id,
            "taskKey": task_key,
            "sampleId": if sample_id.is_empty() { Value::Null } else { Value::String(sample_id.to_owned()) }
        },
        "linkedAt": linked_at,
        "actorId": actor_id
    })
}

fn asset_link_event_data(
    event_id: &str,
    action: &str,
    asset: &asset_refs::AssetRef,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    at: &str,
    actor_id: &str,
) -> Value {
    json!({
        "id": event_id,
        "action": action,
        "assetId": asset.id,
        "rootId": asset.root_id,
        "relativePath": asset.relative_path,
        "mediaType": asset.media_type,
        "size": asset.size,
        "modTime": asset.mod_time,
        "sha256": asset.sha256,
        "availability": asset.availability,
        "link": {
            "runId": run_id,
            "taskKey": task_key,
            "sampleId": if sample_id.is_empty() { Value::Null } else { Value::String(sample_id.to_owned()) }
        },
        "at": at,
        "actorId": actor_id
    })
}

/// Writes the durable link and its append-only event in one transaction.
fn store_asset_link(
    db: &mut Connection,
    asset: &asset_refs::AssetRef,
    project: &str,
    run_id: &str,
    task_key: &str,
    sample_id: &str,
    linked_at: &str,
    actor_id: &str,
) -> Result<(Value, Value), CliError> {
    let record_id = asset_link_record_id(asset, run_id, task_key, sample_id);
    let data = asset_link_data(asset, run_id, task_key, sample_id, linked_at, actor_id);
    let event_id = format!("asset-event-{}", Uuid::new_v4());
    let event = asset_link_event_data(
        &event_id, "linked", asset, run_id, task_key, sample_id, linked_at, actor_id,
    );
    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    let exists: bool = tx
        .query_row("SELECT 1 FROM records WHERE id=?1", [&record_id], |_| Ok(()))
        .optional()
        .map_err(|e| CliError::internal(e.to_string()))?
        .is_some();
    if exists {
        tx.execute(
            "UPDATE records SET data_json=?1, updated_at=?2 WHERE id=?3",
            params![
                serde_json::to_string(&data).map_err(|e| CliError::internal(e.to_string()))?,
                stamp(),
                record_id
            ],
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    } else {
        tx.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
            params![
                record_id,
                "asset_link",
                project,
                run_id,
                1i64,
                serde_json::to_string(&data).map_err(|e| CliError::internal(e.to_string()))?,
                stamp()
            ],
        )
        .map_err(|e| CliError::internal(e.to_string()))?;
    }
    tx.execute(
        "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
        params![
            event_id,
            "asset_link_event",
            project,
            run_id,
            1i64,
            serde_json::to_string(&event).map_err(|e| CliError::internal(e.to_string()))?,
            stamp()
        ],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok((get(db, &record_id)?, get(db, &event_id)?))
}

/// `asset.link`: computes the file's observed metadata and SHA-256, then stores
/// only the logical root, relative path and metadata. The file itself is never
/// copied; `root_dir` is used for this call only and is not persisted.
fn link_local_asset(
    db: &mut Connection,
    request: &AssetLinkCreate,
) -> Result<(Value, Value), CliError> {
    let root_id = request.root_id.trim();
    check_asset_root_id(root_id)?;
    let run_id = request.run_id.trim();
    let task_key = request.task_key.trim();
    let sample_id = request.sample_id.as_deref().unwrap_or("").trim();
    check_asset_sample_id(sample_id)?;
    let actor_id = request.actor_id.as_deref().unwrap_or("").trim();
    if actor_id.len() > 200 || actor_id.chars().any(char::is_control) {
        return Err(CliError::input("actor_id is invalid"));
    }
    let root_dir = request.root_dir.as_str();
    if root_dir.is_empty() {
        return Err(CliError::input("root_dir is required"));
    }
    // Validate the Run/task before reading a potentially large file.
    let project = asset_link_scope(db, run_id, task_key)?;
    let asset = asset_refs::link(
        root_id,
        Path::new(root_dir),
        request.path.as_deref(),
        request.relative_path.as_deref(),
    )
    .map_err(|error| CliError::input(error.to_string()))?;
    store_asset_link(
        db, &asset, &project, run_id, task_key, sample_id,
        &stamp(), actor_id,
    )
}

/// Legacy `save_asset_link` input used by the Run UI module: the caller has
/// already observed the metadata. The stored link and a `linked` event are
/// written together.
pub(crate) fn save_asset_link(db: &mut Connection, input: &Value) -> Result<Value, CliError> {
    let asset = input
        .get("asset")
        .ok_or_else(|| CliError::input("asset is required"))?;
    let root_id = asset
        .get("rootId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    check_asset_root_id(&root_id)?;
    let relative_path = asset_refs::normalize_relative_path(
        asset
            .get("relativePath")
            .and_then(Value::as_str)
            .unwrap_or(""),
    )
    .map_err(|error| CliError::input(error.to_string()))?;

    let link = asset
        .get("link")
        .ok_or_else(|| CliError::input("asset link is required"))?;
    let run_id = link
        .get("runId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    if run_id.is_empty() {
        return Err(CliError::input("runId is required"));
    }
    let task_key = link
        .get("taskKey")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    let sample_id = link
        .get("sampleId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .trim()
        .to_owned();
    check_asset_sample_id(&sample_id)?;
    let project = asset_link_scope(db, &run_id, &task_key)?;

    let media_type: asset_refs::MediaType = serde_json::from_value(json!(
        asset
            .get("mediaType")
            .and_then(Value::as_str)
            .unwrap_or("unknown")
    ))
    .map_err(|_| CliError::input("mediaType is invalid"))?;
    let size = asset.get("size").and_then(Value::as_i64).unwrap_or(0);
    if size < 0 {
        return Err(CliError::input("size must not be negative"));
    }
    let mod_time = asset
        .get("modTime")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    let sha256 = asset
        .get("sha256")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    if !sha256.is_empty()
        && (sha256.len() != 64 || !sha256.chars().all(|c| c.is_ascii_hexdigit()))
    {
        return Err(CliError::input("sha256 must be 64 hexadecimal characters"));
    }
    let availability: asset_refs::Availability = serde_json::from_value(json!(
        asset
            .get("availability")
            .and_then(Value::as_str)
            .unwrap_or("available")
    ))
    .map_err(|_| CliError::input("availability is invalid"))?;
    let linked_at = asset
        .get("linkedAt")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();
    let linked_at = if linked_at.is_empty() { stamp() } else { linked_at };
    let actor_id = asset
        .get("actorId")
        .and_then(Value::as_str)
        .unwrap_or("")
        .to_owned();

    let reference = asset_refs::AssetRef {
        id: asset_refs::stable_id(&root_id, &relative_path),
        root_id,
        relative_path,
        media_type,
        size: size as u64,
        mod_time,
        sha256,
        availability,
    };
    let (record, _) = store_asset_link(
        db, &reference, &project, &run_id, &task_key, &sample_id, &linked_at, &actor_id,
    )?;
    Ok(record["data"].clone())
}

pub(crate) fn list_asset_links(db: &Connection, run_id: &str) -> Result<Value, CliError> {
    let (kind, _, _, _, _) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id must point to a run"));
    }
    let mut stmt = db
        .prepare("SELECT data_json FROM records WHERE kind='asset_link' AND parent_id=?1 ORDER BY created_at ASC, rowid ASC")
        .map_err(|e| CliError::internal(e.to_string()))?;
    let rows = stmt
        .query_map([run_id], |row| row.get::<_, String>(0))
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut out = Vec::new();
    for row in rows.flatten() {
        if let Ok(v) = serde_json::from_str::<Value>(&row) {
            out.push(v);
        }
    }
    Ok(Value::Array(out))
}

/// Removes only the reference; the referenced file is never touched.
pub(crate) fn unlink_asset_link(
    db: &mut Connection,
    id: &str,
    actor_id: &str,
) -> Result<(Value, Value), CliError> {
    if actor_id.len() > 200 || actor_id.chars().any(char::is_control) {
        return Err(CliError::input("actor_id is invalid"));
    }
    let record = get(db, id)?;
    if record["kind"] != "asset_link" {
        return Err(CliError::input("id must point to an asset_link"));
    }
    let data = record["data"].clone();
    let run_id = record["parent_id"]
        .as_str()
        .ok_or_else(|| CliError::internal("asset_link 缺少所属 Run"))?;
    if !has_kind(db, run_id, "run")? {
        return Err(CliError::internal("asset_link 所属 Run 不存在"));
    }
    let project = record["project_id"].as_str().unwrap_or("");
    let at = stamp();
    let actor_id = if actor_id.is_empty() {
        data["actorId"].as_str().unwrap_or("")
    } else {
        actor_id
    };
    let event_id = format!("asset-event-{}", Uuid::new_v4());
    let event = json!({
        "id": event_id,
        "action": "unlinked",
        "assetId": data["id"],
        "rootId": data["rootId"],
        "relativePath": data["relativePath"],
        "mediaType": data["mediaType"],
        "size": data["size"],
        "modTime": data["modTime"],
        "sha256": data["sha256"],
        "availability": data["availability"],
        "link": data["link"],
        "at": at,
        "actorId": actor_id
    });
    let tx = db
        .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
        .map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute("DELETE FROM records WHERE id=?1", [id])
        .map_err(|e| CliError::internal(e.to_string()))?;
    tx.execute(
        "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
        params![
            event_id,
            "asset_link_event",
            project,
            run_id,
            1i64,
            serde_json::to_string(&event).map_err(|e| CliError::internal(e.to_string()))?,
            at
        ],
    )
    .map_err(|e| CliError::internal(e.to_string()))?;
    tx.commit()
        .map_err(|e| CliError::internal(e.to_string()))?;
    Ok((record, get(db, &event_id)?))
}

pub(crate) fn list_asset_events(
    db: &Connection,
    run_id: &str,
    action: Option<&str>,
) -> Result<Value, CliError> {
    if let Some(action) = action {
        if !matches!(action, "linked" | "unlinked") {
            return Err(CliError::input("action must be linked or unlinked"));
        }
    }
    let (kind, _, _, _, _) =
        kind_id(db, run_id)?.ok_or_else(|| CliError::missing("实验运行不存在"))?;
    if kind != "run" {
        return Err(CliError::input("run_id must point to a run"));
    }
    let mut stmt = db
        .prepare("SELECT id,kind,project_id,parent_id,revision,data_json,created_at,updated_at FROM records WHERE kind='asset_link_event' AND parent_id=?1 ORDER BY created_at ASC, rowid ASC")
        .map_err(|e| CliError::internal(e.to_string()))?;
    let rows = stmt
        .query_map([run_id], |row| {
            let raw: String = row.get(5)?;
            let data: Value = serde_json::from_str(&raw).unwrap_or(Value::Null);
            Ok(json!({
                "id": row.get::<_, String>(0)?,
                "kind": row.get::<_, String>(1)?,
                "project_id": row.get::<_, Option<String>>(2)?,
                "parent_id": row.get::<_, Option<String>>(3)?,
                "revision": row.get::<_, i64>(4)?,
                "data": data,
                "created_at": row.get::<_, String>(6)?,
                "updated_at": row.get::<_, String>(7)?
            }))
        })
        .map_err(|e| CliError::internal(e.to_string()))?;
    let mut out = Vec::new();
    for row in rows {
        let record = row.map_err(|e| CliError::internal(e.to_string()))?;
        if action.is_some_and(|action| record["data"]["action"] != action) {
            continue;
        }
        out.push(record);
    }
    Ok(Value::Array(out))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn agent_tools_are_discoverable() {
        let specs = tool_specs();
        let names: Vec<&str> = specs.iter().filter_map(|v| v.get("name").and_then(Value::as_str)).collect();
        for tool in ["sdk.modules", "sdk.describe", "sdk.validate", "asterdoc.inspect", "asterdoc.assess", "asterdoc.validate", "asterdoc.load", "asterdoc.save", "asterdoc.command", "asterdoc.compile", "asterdoc.create_run"] {
            assert!(names.contains(&tool), "missing {tool}");
            assert!(input_schema(tool).is_some(), "missing input schema for {tool}");
        }
        let command = input_schema("asterdoc.command").unwrap();
        assert!(command["required"].as_array().unwrap().contains(&json!("expected_revision")));
        assert!(command["properties"].get("operation_id").is_some());
        assert!(command["properties"].get("revision").is_none());
        let variants = command["properties"]["command"]["oneOf"].as_array().unwrap();
        let declared_types: Vec<&str> = variants.iter().flat_map(|variant| {
            let kind = &variant["properties"]["type"];
            kind["const"].as_str().into_iter().chain(
                kind["enum"].as_array().into_iter().flatten().filter_map(Value::as_str)
            )
        }).collect();
        for action in ["add_module", "add_step", "update_module", "assign_wells", "clear_wells", "connect_steps", "disconnect_steps", "remove_module", "remove_step"] {
            assert!(declared_types.contains(&action), "command schema omits {action}");
        }
        assert_eq!(result_schema("sdk.modules")["required"][0], "modules");
        assert_eq!(result_schema("asterdoc.assess")["required"][3], "state");
        let plate = asterdoc_module_spec("plate", 1).unwrap();
        assert!(asterdoc_module_spec("plate96", 1).is_none());
        for field in ["samples", "replicates"] {
            assert!(plate["inputs"].as_array().unwrap().iter().any(|port| port["name"] == field));
        }
    }

    #[test]
    fn compiler_records_the_block_version_each_step_reuses() {
        let source = "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8\n```\n\
```asterlab\nkind: group\nid: g-1\nschema: 1\nname: Control\nrole: control\n```\n\
```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\nendpoint: OD450\nunit: AU\ngroup_refs: [g-1]\n```\n\
```asterlab\nkind: reference\nid: ref-block\nschema: 1\nref: block-v1\nview: protocol_block\n```\n\
```asterlab\nkind: reference\nid: ref-seed-block\nschema: 1\nref: block-seed\nview: protocol_block\n```\n\
```asterlab\nkind: sample\nid: s-1\nschema: 1\nsample_id: S1\ngroup_ref: g-1\namount: {value: 1, unit: uL}\n```\n\
```asterlab\nkind: step\nid: step-read\nschema: 2\ntitle: Read\nassay: assay-cck8\npath: cck8/readout\norder: 1\nuses:\n  - {ref: ref-block, role: other}\n  - {ref: s-1, role: sample}\n```\n\
```asterlab\nkind: step\nid: step-seed\nschema: 2\ntitle: Seed\nassay: assay-cck8\npath: cck8/seed\norder: 2\nuses:\n  - {ref: s-1, role: sample}\n```\n\
```asterlab\nkind: step\nid: step-organize\nschema: 3\ntitle: Incubate\nexecution: organizational\norder: 3\n```\n\
```asterlab\nkind: workflow\nid: wf\nschema: 1\nsteps: [step-read, step-seed, step-organize]\n```\n";
        let design = compile_asterdoc_to_design(source).unwrap();
        let blocks = &design["mygo"]["stepBlocks"];
        assert_eq!(blocks["step-read"], json!(["block-v1"]));
        assert!(blocks.get("step-seed").is_none(), "a sample reference is not a block reference: {blocks}");
        assert_eq!(design["mygo"]["stepExecution"]["step-organize"], "organizational");
        let task_ids: Vec<_> = design["mygo"]["stepTasks"]
            .as_array().unwrap().iter()
            .filter_map(|task| task["stepId"].as_str())
            .collect();
        assert_eq!(task_ids, ["step-read", "step-seed"]);
        let manifest = json!([{"assay_id":"assay-cck8","block_version_ids":["block-v1"]}]);
        let issues = step_materialization_issues(&design, manifest.as_array().unwrap());
        assert!(issues.iter().any(|issue| issue.contains("STEP_NOT_MATERIALIZED: cck8/seed")));
        assert!(!issues.iter().any(|issue| issue.contains("step-organize")));
        let wrong_scope = json!([
            {"assay_id":"assay-cck8","block_version_ids":["block-v1"]},
            {"assay_id":"assay-other","block_version_ids":["block-seed"]}
        ]);
        let wrong_scope_issues = step_materialization_issues(&design, wrong_scope.as_array().unwrap());
        assert!(wrong_scope_issues.iter().any(|issue| issue.contains("STEP_NOT_MATERIALIZED: cck8/seed")));
        let legacy = json!({"mygo":{"stepTasks":[{"stepId":"legacy-seed","path":"cck8/seed","assayId":"assay-cck8"}]}});
        let legacy_issues = step_materialization_issues(&legacy, &[]);
        assert!(legacy_issues.iter().any(|issue| issue.contains("STEP_NOT_MATERIALIZED: cck8/seed")));

        let bound_seed = source.replace(
            "uses:\n  - {ref: s-1, role: sample}\n```\n```asterlab\nkind: step\nid: step-organize",
            "uses:\n  - {ref: ref-seed-block, role: other}\n  - {ref: s-1, role: sample}\n```\n```asterlab\nkind: step\nid: step-organize",
        );
        let bound_design = compile_asterdoc_to_design(&bound_seed).unwrap();
        assert_eq!(bound_design["mygo"]["stepBlocks"]["step-seed"], json!(["block-seed"]));
        let complete_manifest = json!([{"assay_id":"assay-cck8","block_version_ids":["block-v1","block-seed"]}]);
        assert!(step_materialization_issues(&bound_design, complete_manifest.as_array().unwrap()).is_empty());
        let invalid_org = source.replace(
            "execution: organizational\n",
            "execution: organizational\npath: cck8/incubation\n",
        );
        assert!(validate_asterdoc_source(&invalid_org).is_err());
    }

    #[test]
    fn compiler_keeps_sample_identity_explicit_across_timepoints() {
        let source = "```asterlab\nkind: experiment\nid: exp-1\nschema: 1\ntitle: CCK-8\n```\n\
```asterlab\nkind: group\nid: g-control\nschema: 1\nname: Control\nrole: control\n```\n\
```asterlab\nkind: assay\nid: assay-cck8\nschema: 1\ntitle: CCK-8\nendpoint: OD450\nunit: AU\ngroup_refs: [g-control]\ntimepoints: [D1, D3]\n```\n\
```asterlab\nkind: plate\nid: plate-1\nschema: 1\nformat: 96\nassay: assay-cck8\ngroups:\n  g-control: [A1, A2, A3]\nsamples:\n  A1: sample-1\n  A2: sample-1\n  A3: sample-1\nreplicates:\n  A1: 1\n  A2: 2\n  A3: 3\n```\n";
        let design = compile_asterdoc_to_design(source).unwrap();
        let wells = design["mygo"]["assays"]["assay-cck8"]["wells"].as_array().unwrap();
        assert_eq!(wells.len(), 6, "unscoped plate layout should project to every declared assay timepoint");
        for timepoint in ["D1", "D3"] {
            let at_timepoint: Vec<_> = wells.iter().filter(|well| well["timepoint"] == timepoint).collect();
            assert_eq!(at_timepoint.len(), 3);
            for (index, well) in at_timepoint.iter().enumerate() {
                assert_eq!(well["sampleId"], "sample-1");
                assert_eq!(well["replicate"], index as i64 + 1);
            }
        }

        let source_without_identity = source.replace(
            "samples:\n  A1: sample-1\n  A2: sample-1\n  A3: sample-1\nreplicates:\n  A1: 1\n  A2: 2\n  A3: 3\n",
            "",
        );
        let unmapped = compile_asterdoc_to_design(&source_without_identity).unwrap();
        let first = &unmapped["mygo"]["assays"]["assay-cck8"]["wells"][0];
        assert_eq!(first["sampleId"], "");
        assert_eq!(first["replicate"], 0);
    }

    #[test]
    fn assess_includes_run_creation_audit_warnings() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('design-1','design','project-1',NULL,1,'{}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-control
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-control]
timepoints: [D1]
```
```asterlab
kind: plate
id: plate-1
schema: 1
format: 96
assay: assay-cck8
groups:
  g-control: [A1]
```
"#;
        save_asterdoc_version(&mut db, &json!({
            "design_id": "design-1",
            "source": source,
            "expected_revision": 0
        })).unwrap();

        let assessed = assess_asterdoc(&db, &json!({"design_id": "design-1"})).unwrap();
        let compiled = compile_asterdoc_to_design(source).unwrap();
        let freeze_audit = run_creation_audit(&compiled, 0, "").unwrap();
        assert_eq!(assessed["warnings"], freeze_audit["warnings"]);
        assert!(assessed["warnings"].as_array().unwrap().iter().any(|warning|
            warning.as_str().unwrap_or("").contains("尚未指定完整的样本与技术重复映射")
        ));
    }


    #[test]
    fn assess_and_run_creation_reject_unmaterialized_steps() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('design-1','design','project-1',NULL,1,'{}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-1
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-1]
```
```asterlab
kind: step
id: step-seed
schema: 3
title: Seed
execution: executable
assay: assay-cck8
path: cck8/seed
```
"#;
        save_asterdoc_version(&mut db, &json!({
            "design_id": "design-1",
            "source": source,
            "expected_revision": 0
        })).unwrap();

        let assessed = assess_asterdoc(&db, &json!({"design_id": "design-1"})).unwrap();
        assert!(assessed["errors"].as_array().unwrap().iter().any(|error|
            error.as_str().unwrap_or("").contains("STEP_NOT_MATERIALIZED: cck8/seed")
        ), "assessment errors = {}", assessed["errors"]);

        let error = create_run_from_asterdoc(&mut db, "design-1", 1, None, None, &[])
            .unwrap_err();
        assert!(
            error.message.contains("STEP_NOT_MATERIALIZED: cck8/seed"),
            "{}",
            error.message
        );
        let revision: i64 = db.query_row(
            "SELECT revision FROM records WHERE id='design-1'",
            [],
            |row| row.get(0),
        ).unwrap();
        let run_count: i64 = db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='run'",
            [],
            |row| row.get(0),
        ).unwrap();
        let design_version_count: i64 = db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='design_version'",
            [],
            |row| row.get(0),
        ).unwrap();
        assert_eq!((revision, run_count, design_version_count), (1, 0, 0));
    }

    #[test]
    fn step_materialization_preserves_frozen_run_protocol_block_revision() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch(
            "CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
             INSERT INTO records VALUES('project-1','project',NULL,NULL,1,'{}','now','now');
             INSERT INTO records VALUES('protocol-1','protocol','project-1',NULL,1,'{\"title\":\"CCK-8\"}','now','now');
             INSERT INTO records VALUES('protocol-v1','protocol_version','project-1','protocol-1',1,'{\"protocol_id\":\"protocol-1\",\"version\":1,\"body\":\"\",\"root_block_version_id\":\"block-v1\",\"root_bindings\":[]}','now','now');
             INSERT INTO records VALUES('block-1','protocol_block','project-1',NULL,1,'{\"title\":\"CCK-8 block\",\"summary\":\"\"}','now','now');",
        ).unwrap();
        let source = r#"```asterlab
kind: experiment
id: exp-1
schema: 1
title: CCK-8
```
```asterlab
kind: group
id: g-1
schema: 1
name: Control
role: control
```
```asterlab
kind: assay
id: assay-cck8
schema: 1
title: CCK-8
endpoint: OD450
unit: AU
group_refs: [g-1]
protocol_version_id: protocol-v1
```
"#;
        let mut design = compile_asterdoc_to_design(source).unwrap();
        let design_object = design.as_object_mut().unwrap();
        design_object.remove("mygo");
        design_object.remove("asterlab");
        let design_json = serde_json::to_string(&design).unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
            rusqlite::params![design_json],
        ).unwrap();
        db.execute(
            "INSERT INTO records VALUES('design-v1','design_version','project-1','design-1',1,'{}','now','now')",
            [],
        ).unwrap();
        let block_v1 = json!({
            "block_id":"block-1", "version":1, "summary":"",
            "parameter_slots":[],
            "steps":[{"kind":"text","use_id":"seed","title":"Seed","instruction":"Seed cells"}]
        });
        db.execute(
            "INSERT INTO records VALUES('block-v1','protocol_block_version','project-1','block-1',1,?1,'now','now')",
            rusqlite::params![serde_json::to_string(&block_v1).unwrap()],
        ).unwrap();

        let tx = db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).unwrap();
        let run = create_run_in_transaction(
            &tx, "design-1", "Run 1", None, None, None, None, "independent", None,
        ).unwrap();
        let run_id = run["id"].as_str().unwrap().to_owned();
        tx.commit().unwrap();
        let before = get(&db, &run_id).unwrap()["data"]["protocol_manifest"].clone();
        assert_eq!(before[0]["protocol_version_id"], "protocol-v1");
        assert_eq!(before[0]["block_version_ids"], json!(["block-v1"]));

        let block_v2 = json!({
            "block_id":"block-1", "version":2, "summary":"Revised",
            "parameter_slots":[],
            "steps":[{"kind":"text","use_id":"seed","title":"Revised seed","instruction":"Updated"}]
        });
        db.execute(
            "INSERT INTO records VALUES('block-v2','protocol_block_version','project-1','block-1',2,?1,'later','later')",
            rusqlite::params![serde_json::to_string(&block_v2).unwrap()],
        ).unwrap();
        let after = get(&db, &run_id).unwrap()["data"]["protocol_manifest"].clone();
        assert_eq!(after, before);
    }

    #[test]
    fn clear_wells_removes_sample_and_replicate_mappings() {
        let source = "```asterlab\nkind: plate\nid: p1\nschema: 1\nformat: 96\ngroups:\n  g1: [A1, A2]\nsamples:\n  A1: S1\n  A2: S2\nreplicates:\n  A1: 1\n  A2: 1\n```\n";
        let next = apply_asterdoc_patch(source, &json!({
            "type": "clear_wells", "moduleId": "p1", "wells": ["A1"]
        })).unwrap();
        let mut blocks = asterdoc_blocks(&next).unwrap();
        let block = blocks.remove(0);
        let groups = block.value["groups"]["g1"].as_sequence().unwrap();
        assert_eq!(groups.len(), 1);
        assert_eq!(groups[0].as_str(), Some("A2"));
        assert!(block.value["samples"].get("A1").is_none());
        assert!(block.value["replicates"].get("A1").is_none());


    }
    #[test]
    fn manifest_rule_must_be_registered_in_rust() {
        let spec = json!({
            "foundationVersion": 1,
            "sdkVersion": "1.0.0",
            "modules": [{
                "kind": "probe", "schema": 1, "sdk": "1.0.0", "category": "workflow",
                "inputs": [], "outputs": [], "actions": [], "permissions": [],
                "rules": ["not-a-real-rule"]
            }]
        });
        let error = validate_asterdoc_spec(&spec).unwrap_err();
        assert!(error.contains("unregistered rule not-a-real-rule"), "{error}");
    }

    #[test]
    fn read_only_tools_do_not_write() {
        for tool in ["sdk.modules", "sdk.describe", "sdk.validate", "asterdoc.inspect", "asterdoc.assess", "asterdoc.validate", "asterdoc.compile", "asterdoc.load"] {
            assert!(!tool_writes(tool), "{tool} should be read-only");
        }
        for tool in ["asterdoc.save", "asterdoc.command", "asterdoc.create_run"] {
            assert!(tool_writes(tool), "{tool} should write");
        }
    }
    #[test]
    fn workflow_inspect_tree_matches_shared_go_evidence() {
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .find(|dir| {
                dir.join("prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite")
                    .is_file()
            })
            .expect("repository root with RNA evidence database");
        let data_dir = std::env::temp_dir().join(format!("asterlab-workflow-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).expect("temp data dir");
        fs::copy(
            root.join("prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite"),
            data_dir.join("labworkbench.sqlite"),
        )
        .expect("copy RNA evidence database");
        let design_id = "d5525203-56bb-4b52-82c4-0e05f0ed2ff8";
        let run_id = "ad9b452e-f8ea-404b-9a2a-7124f9403b02";

        let design = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: Some(1),
                run_id: None,
            },
            &data_dir,
        )
        .expect("design projection");
        let design_evidence: Value = serde_json::from_str(
            &fs::read_to_string(
                root.join("docs/sdk/rna-extraction-case/evidence/inspect-tree-design.json"),
            )
            .expect("design evidence"),
        )
        .expect("design evidence json");
        assert_eq!(design, design_evidence["result"]);

        let run = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect("run projection");
        let run_evidence: Value = serde_json::from_str(
            &fs::read_to_string(
                root.join("docs/sdk/rna-extraction-case/evidence/inspect-tree-run.json"),
            )
            .expect("run evidence"),
        )
        .expect("run evidence json");
        assert_eq!(run, run_evidence["result"]);

        let wrong_design = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: "00000000-0000-0000-0000-000000000000".into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("Run must reject a different design");
        assert_eq!(wrong_design.code, "INVALID_INPUT");

        let stale_revision = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: Some(2),
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("Run must reject a non-frozen revision");
        assert_eq!(stale_revision.code, "REVISION_CONFLICT");

        let db = open_db(&data_dir).expect("open evidence database");
        let run_record = get(&db, run_id).expect("run record");
        let mut data = run_record["data"].clone();
        data["asterdoc"]["source_hash"] = json!("corrupt-source-hash");
        db.execute(
            "UPDATE records SET data_json=?2 WHERE id=?1",
            params![run_id, serde_json::to_string(&data).unwrap()],
        )
        .expect("corrupt frozen source hash");
        drop(db);
        let corrupt_hash = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("Run must reject a frozen source hash mismatch");
        assert_eq!(corrupt_hash.code, "REVISION_CONFLICT");
        assert!(
            corrupt_hash.message.contains("source_hash"),
            "{}",
            corrupt_hash.message
        );

        let db = open_db(&data_dir).expect("reopen evidence database");
        let mut old_data = run_record["data"].clone();
        old_data["design_revision"] = run_record["data"]["asterdoc"]["revision"].clone();
        if let Some(object) = old_data.as_object_mut() {
            object.remove("asterdoc");
        }
        db.execute(
            "UPDATE records SET data_json=?2 WHERE id=?1",
            params![run_id, serde_json::to_string(&old_data).unwrap()],
        )
        .expect("remove frozen document metadata");
        drop(db);
        let old_run = inspect_workflow_tree(
            &InspectTreeInput {
                design_id: design_id.into(),
                revision: None,
                run_id: Some(run_id.into()),
            },
            &data_dir,
        )
        .expect_err("legacy Run without a frozen document must fail explicitly");
        assert_eq!(old_run.code, "NOT_FOUND");
        assert!(old_run.message.contains("AsterDoc revision"));
        fs::remove_dir_all(&data_dir).expect("remove temp data dir");
    }

    #[test]
    fn asterdoc_parity_samples() {
        // This file is compiled into more than one crate (the CLI and the app),
        // so the fixture is found by walking up from the compiling manifest
        // instead of assuming the repository root is the working directory.
        let fixture = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .map(|dir| dir.join("tests/asterdoc_parity_samples.json"))
            .find(|path| path.is_file())
            .expect("shared parity fixture");
        let raw = std::fs::read_to_string(&fixture)
            .unwrap_or_else(|e| panic!("read {}: {e}", fixture.display()));
        let samples: serde_json::Value = serde_json::from_str(&raw).expect("json");
        for sample in samples.as_array().expect("array") {
            let source = sample["source"].as_str().unwrap_or("");
            let expected = sample["valid"].as_bool().unwrap_or(false);
            let actual = validate_asterdoc_source(source).is_ok();
            assert_eq!(actual, expected, "sample {} mismatch", sample["name"]);
        }
    }

    fn seed_asset_run(db: &Connection) -> String {
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('project-1','project',NULL,NULL,1,?1,'now','now')",
            [json!({"name":"p","description":""}).to_string()],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('design-1','design','project-1',NULL,1,?1,'now','now')",
            [json!({"title":"d"}).to_string()],
        )
        .unwrap();
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('version-1','design_version','project-1','design-1',1,?1,'now','now')",
            [json!({"design_id":"design-1","design_revision":1,"snapshot":{"title":"d"}}).to_string()],
        )
        .unwrap();
        let step = json!({
            "path":"s/1","use_id":"u1","title":"T","instruction":"I",
            "parameters":[],"unresolved_parameter_refs":[],"source_block_version_id":"b1"
        });
        db.execute(
            "INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES('run-1','run','project-1','design-1',1,?1,'now','now')",
            [json!({
                "title":"r","design_version_id":"version-1","design_revision":1,
                "status":"in_progress",
                "protocol_manifest":[{"assay_id":"assay-1","assay_title":"A","steps":[step]}]
            })
            .to_string()],
        )
        .unwrap();
        run_workspace(db, "run-1").unwrap()["tasks"][0]["task_key"]
            .as_str()
            .expect("task key")
            .to_owned()
    }

    fn dir_size(path: &Path) -> u64 {
        fs::read_dir(path)
            .map(|entries| {
                entries
                    .flatten()
                    .map(|entry| {
                        let path = entry.path();
                        if path.is_dir() {
                            dir_size(&path)
                        } else {
                            fs::metadata(&path).map(|meta| meta.len()).unwrap_or(0)
                        }
                    })
                    .sum()
            })
            .unwrap_or(0)
    }

    fn contains_file_named(path: &Path, name: &str) -> bool {
        fs::read_dir(path)
            .map(|entries| {
                entries.flatten().any(|entry| {
                    let path = entry.path();
                    path.file_name().and_then(|value| value.to_str()) == Some(name)
                        || (path.is_dir() && contains_file_named(&path, name))
                })
            })
            .unwrap_or(false)
    }

    #[test]
    fn asset_link_hashes_a_large_file_without_copying_it() {
        use std::io::Write;

        let data_dir = std::env::temp_dir().join(format!("asterlab-asset-db-{}", Uuid::new_v4()));
        let root_dir = std::env::temp_dir().join(format!("asterlab-asset-root-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        fs::create_dir_all(root_dir.join("d1")).unwrap();
        let mut db = open_db(&data_dir).unwrap();
        let task_key = seed_asset_run(&db);

        let source = root_dir.join("d1").join("large.tif");
        let chunk = vec![0xABu8; 1024 * 1024];
        {
            let mut file = fs::File::create(&source).unwrap();
            for _ in 0..8 {
                file.write_all(&chunk).unwrap();
            }
        }
        let mut hasher = Sha256::new();
        for _ in 0..8 {
            hasher.update(&chunk);
        }
        let expected_hash: String = hasher
            .finalize()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();

        let before = dir_size(&data_dir);
        let request = AssetLinkCreate {
            root_id: "project-data".into(),
            root_dir: root_dir.display().to_string(),
            relative_path: Some("d1/large.tif".into()),
            path: None,
            run_id: "run-1".into(),
            task_key: task_key.clone(),
            sample_id: Some("sample-1".into()),
            actor_id: Some("tester".into()),
        };
        let (record, event) = link_local_asset(&mut db, &request).unwrap();
        let after = dir_size(&data_dir);
        assert!(
            after.saturating_sub(before) < 1024 * 1024,
            "linking copied the file into the data dir: {before} -> {after}"
        );
        assert!(
            !contains_file_named(&data_dir, "large.tif"),
            "the source file must not be copied into the data directory"
        );
        assert_eq!(fs::metadata(&source).unwrap().len(), 8 * 1024 * 1024);

        let data = &record["data"];
        assert_eq!(data["size"], 8 * 1024 * 1024);
        assert_eq!(data["sha256"], expected_hash);
        assert_eq!(data["mediaType"], "image");
        assert_eq!(data["relativePath"], "d1/large.tif");
        assert_eq!(data["rootId"], "project-data");
        assert_eq!(data["link"]["sampleId"], "sample-1");
        assert!(
            !data.to_string().contains(&root_dir.display().to_string()),
            "absolute root path must not be persisted: {data}"
        );
        assert_eq!(event["data"]["action"], "linked");
        assert_eq!(event["data"]["sha256"], expected_hash);

        let links = list_asset_links(&db, "run-1").unwrap();
        assert_eq!(links.as_array().unwrap().len(), 1);
        assert_eq!(links[0]["sha256"], expected_hash);
        let events = list_asset_events(&db, "run-1", None).unwrap();
        assert_eq!(events.as_array().unwrap().len(), 1);
        assert_eq!(events[0]["data"]["action"], "linked");
        assert_eq!(events[0]["data"]["assetId"], data["id"]);

        // Relinking the same file refreshes the same reference and appends
        // another event instead of creating a second asset or copying bytes.
        let (again, _) = link_local_asset(&mut db, &request).unwrap();
        assert_eq!(again["id"], record["id"]);
        assert_eq!(again["data"]["id"], data["id"]);
        assert_eq!(
            list_asset_events(&db, "run-1", Some("linked"))
                .unwrap()
                .as_array()
                .unwrap()
                .len(),
            2
        );

        let (removed, unlink_event) =
            unlink_asset_link(&mut db, record["id"].as_str().unwrap(), "tester").unwrap();
        assert_eq!(removed["data"]["sha256"], expected_hash);
        assert_eq!(unlink_event["data"]["action"], "unlinked");
        assert_eq!(list_asset_links(&db, "run-1").unwrap(), json!([]));
        let events = list_asset_events(&db, "run-1", None).unwrap();
        let actions: Vec<&str> = events
            .as_array()
            .unwrap()
            .iter()
            .map(|event| event["data"]["action"].as_str().unwrap())
            .collect();
        assert_eq!(actions, ["linked", "linked", "unlinked"]);
        assert_eq!(fs::metadata(&source).unwrap().len(), 8 * 1024 * 1024);

        drop(db);
        fs::remove_dir_all(&data_dir).unwrap();
        fs::remove_dir_all(&root_dir).unwrap();
    }

    #[test]
    fn asset_link_rejects_paths_outside_the_root_without_writing() {
        let mut db = Connection::open_in_memory().unwrap();
        db.execute_batch("CREATE TABLE records(id TEXT PRIMARY KEY,kind TEXT NOT NULL,project_id TEXT,parent_id TEXT,revision INTEGER NOT NULL,data_json TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);").unwrap();
        let task_key = seed_asset_run(&db);
        let root_dir = std::env::temp_dir().join(format!("asterlab-asset-root-{}", Uuid::new_v4()));
        fs::create_dir_all(&root_dir).unwrap();
        let outside = root_dir
            .parent()
            .unwrap()
            .join(format!("asterlab-asset-outside-{}.txt", Uuid::new_v4()));
        fs::write(&outside, b"secret").unwrap();

        let request = |relative_path: Option<&str>, path: Option<&str>, task: &str| AssetLinkCreate {
            root_id: "project-data".into(),
            root_dir: root_dir.display().to_string(),
            relative_path: relative_path.map(str::to_owned),
            path: path.map(str::to_owned),
            run_id: "run-1".into(),
            task_key: task.to_owned(),
            sample_id: None,
            actor_id: None,
        };
        let error = link_local_asset(
            &mut db,
            &request(None, Some(outside.to_str().unwrap()), &task_key),
        )
        .unwrap_err();
        assert!(error.message.contains("outside its root"), "{error:?}");
        let error = link_local_asset(&mut db, &request(Some("../secret.txt"), None, &task_key))
            .unwrap_err();
        assert!(error.message.contains("escape"), "{error:?}");
        let error = link_local_asset(
            &mut db,
            &request(Some("missing.txt"), None, "not-a-task"),
        )
        .unwrap_err();
        assert!(error.message.contains("does not belong"), "{error:?}");
        assert_eq!(list_asset_links(&db, "run-1").unwrap(), json!([]));
        assert_eq!(list_asset_events(&db, "run-1", None).unwrap(), json!([]));

        fs::remove_dir_all(&root_dir).unwrap();
        fs::remove_file(&outside).unwrap();
    }

    struct AgentEditFixture {
        data_dir: std::path::PathBuf,
        protocol_id: String,
        protocol_version_id: String,
        run_id: String,
    }

    fn seed_agent_edit_fixture(db: &mut Connection) -> AgentEditFixture {
        let data_dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let project = create_project(db, "agent edit").unwrap();
        let project_id = project["id"].as_str().unwrap().to_owned();
        let block = {
            let tx = db.unchecked_transaction().unwrap();
            let result = create_protocol_block_records(
                &tx,
                &project_id,
                &json!({
                    "title": "RNA lysis",
                    "summary": "lysis",
                    "parameter_slots": [{
                        "key": "volume_per_sample",
                        "label": "Volume per sample",
                        "value_type": "number",
                        "unit": "uL",
                        "required": true
                    }],
                    "steps": [{
                        "kind": "text",
                        "use_id": "lyse",
                        "title": "Lyse",
                        "instruction": "Add buffer"
                    }]
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            result
        };
        let block_version_id = block["version"]["id"].as_str().unwrap().to_owned();
        let (protocol_id, protocol_version_id) = {
            let tx = db.unchecked_transaction().unwrap();
            let protocol = insert(
                &tx,
                "protocol",
                Some(&project_id),
                Some(&project_id),
                1,
                &json!({"title": "RNA protocol"}),
            )
            .unwrap();
            let protocol_id = protocol["id"].as_str().unwrap().to_owned();
            let version = insert(
                &tx,
                "protocol_version",
                Some(&project_id),
                Some(&protocol_id),
                1,
                &json!({
                    "protocol_id": protocol_id,
                    "version": 1,
                    "body": "",
                    "root_block_version_id": block_version_id,
                    "root_bindings": []
                }),
            )
            .unwrap();
            tx.commit().unwrap();
            (
                protocol_id,
                version["id"].as_str().unwrap().to_owned(),
            )
        };
        let design = create_design_record(db, &project_id, &json!({"title": "d"})).unwrap();
        let design_id = design["design"]["id"].as_str().unwrap().to_owned();
        let run = {
            let tx = db
                .transaction_with_behavior(rusqlite::TransactionBehavior::Immediate)
                .unwrap();
            let run = create_run_in_transaction(
                &tx,
                &design_id,
                "run",
                None,
                Some(&protocol_version_id),
                None,
                None,
                "independent",
                None,
            )
            .unwrap();
            tx.commit().unwrap();
            run
        };
        AgentEditFixture {
            data_dir,
            protocol_id,
            protocol_version_id,
            run_id: run["id"].as_str().unwrap().to_owned(),
        }
    }

    fn agent_edit_proposal(fixture: &AgentEditFixture) -> Value {
        json!({
            "schema_version": 1,
            "proposal_id": "ae-rna-lyse-001",
            "target": {
                "kind": "protocol",
                "object_id": fixture.protocol_id,
                "location": {
                    "protocol_version_id": fixture.protocol_version_id,
                    "scope": {"kind": "unscoped"},
                    "step_path": "root/lyse"
                }
            },
            "base_version_id": fixture.protocol_version_id,
            "expected_version": 1,
            "origin": {"source": "agent", "rationale": "kit instruction update"},
            "diffs": [{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": {
                    "parameter_key": "volume_per_sample",
                    "value": 250.0,
                    "unit": "uL",
                    "source": "protocol_fixed",
                    "source_reference": "rna-kit-ifu@2026-03#step-2",
                    "confirmed": false
                }
            }]
        })
    }

    fn agent_edit_confirm_request(
        proposal: Value,
        digest: &str,
        idempotency_key: &str,
    ) -> Value {
        json!({
            "schema_version": 1,
            "proposal": proposal,
            "proposal_digest": digest,
            "idempotency_key": idempotency_key,
            "confirmed_by": "researcher:zhang",
            "acknowledged": [],
            "note": "按试剂盒说明书更新体积。"
        })
    }

    #[test]
    fn agent_edit_commit_appends_a_version_and_keeps_the_frozen_run() {
        let data_dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let mut db = open_db(&data_dir).unwrap();
        let fixture = seed_agent_edit_fixture(&mut db);
        let proposal = agent_edit_proposal(&fixture);
        let validation = validate_agent_edit(&db, proposal.clone()).unwrap();
        assert_eq!(validation["status"], "needs_confirmation");
        let digest = validation["proposal_digest"].as_str().unwrap().to_owned();

        let run_before = get(&db, &fixture.run_id).unwrap();
        let previous_version_before = get(&db, &fixture.protocol_version_id).unwrap();

        let request =
            agent_edit_confirm_request(proposal, &digest, "ae-rna-lyse-001-confirm-1");
        let result = commit_agent_edit(&mut db, request).unwrap();
        assert_eq!(result["new_revision"], 2);
        assert_eq!(result["previous_version_id"], fixture.protocol_version_id);
        assert_eq!(result["replayed"], false);
        assert_eq!(result["confirmed_by"], "researcher:zhang");
        assert_eq!(result["applied"][0]["requested"]["confirmed"], true);
        assert_eq!(
            result["invalidated_refs"],
            json!(["core:binding:unscoped:root/lyse/volume_per_sample"])
        );

        // The frozen Run still points at the previous version and did not move.
        let run_after = get(&db, &fixture.run_id).unwrap();
        assert_eq!(run_after, run_before);
        assert_eq!(
            run_after["data"]["protocol_manifest"][0]["protocol_version_id"],
            fixture.protocol_version_id
        );
        // The previous version is immutable, including its bindings.
        let previous_version_after = get(&db, &fixture.protocol_version_id).unwrap();
        assert_eq!(previous_version_after, previous_version_before);

        let new_version_id: String = db
            .query_row(
                "SELECT id FROM records WHERE kind='protocol_version' AND parent_id=?1 AND revision=2",
                [&fixture.protocol_id],
                |row| row.get(0),
            )
            .unwrap();
        let new_version = get(&db, &new_version_id).unwrap();
        assert_eq!(
            new_version["data"]["root_bindings"],
            json!([{"slot": "volume_per_sample", "value": 250.0, "unit": "uL"}])
        );
        assert_eq!(
            new_version["data"]["agent_edit"]["proposal_id"],
            "ae-rna-lyse-001"
        );
        assert_eq!(
            new_version["data"]["agent_edit"]["proposal_digest"],
            digest
        );
        assert_eq!(
            new_version["data"]["agent_edit"]["confirmed_by"],
            "researcher:zhang"
        );
        assert_eq!(new_version["data"]["agent_edit"]["new_revision"], 2);

        // A stale proposal on the same base is a version conflict, not a
        // silent overwrite.
        let mut stale = agent_edit_confirm_request(
            agent_edit_proposal(&fixture),
            &digest,
            "ae-rna-lyse-001-confirm-2",
        );
        stale["proposal_digest"] = json!(validation["proposal_digest"]);
        let error = commit_agent_edit(&mut db, stale).unwrap_err();
        assert_eq!(error.code, "VERSION_CONFLICT");
        assert_eq!(error.path.as_deref(), Some("expected_version"));
        assert_eq!(error.current_version, Some(2));
        assert_eq!(error.current_version_id.as_deref(), Some(new_version_id.as_str()));
        let count: i64 = db
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='protocol_version' AND parent_id=?1",
                [&fixture.protocol_id],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 2);

        fs::remove_dir_all(&data_dir).ok();
        fs::remove_dir_all(&fixture.data_dir).ok();
    }

    #[test]
    fn concurrent_agent_edit_commits_leave_exactly_one_winner() {
        let data_dir = std::env::temp_dir().join(format!("asterlab-agent-edit-{}", Uuid::new_v4()));
        fs::create_dir_all(&data_dir).unwrap();
        let mut seed = open_db(&data_dir).unwrap();
        let fixture = seed_agent_edit_fixture(&mut seed);
        let proposal = agent_edit_proposal(&fixture);
        let validation = validate_agent_edit(&seed, proposal.clone()).unwrap();
        let digest = validation["proposal_digest"].as_str().unwrap().to_owned();
        drop(seed);

        let barrier = std::sync::Arc::new(std::sync::Barrier::new(2));
        let mut handles = Vec::new();
        for key in ["ae-concurrent-a", "ae-concurrent-b"] {
            let data_dir = data_dir.clone();
            let proposal = proposal.clone();
            let digest = digest.clone();
            let barrier = barrier.clone();
            handles.push(std::thread::spawn(move || {
                let mut db = open_db(&data_dir).unwrap();
                barrier.wait();
                commit_agent_edit(
                    &mut db,
                    agent_edit_confirm_request(proposal, &digest, key),
                )
            }));
        }
        let results = handles
            .into_iter()
            .map(|handle| handle.join().unwrap())
            .collect::<Vec<_>>();
        let successes = results.iter().filter(|result| result.is_ok()).count();
        let conflicts = results
            .iter()
            .filter(|result| {
                result
                    .as_ref()
                    .is_err_and(|error| error.code == "VERSION_CONFLICT")
            })
            .count();
        assert_eq!(successes, 1, "{results:?}");
        assert_eq!(conflicts, 1, "{results:?}");

        let db = open_db(&data_dir).unwrap();
        let count: i64 = db
            .query_row(
                "SELECT COUNT(*) FROM records WHERE kind='protocol_version' AND parent_id=?1",
                [&fixture.protocol_id],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 2, "exactly one new version");

        fs::remove_dir_all(&data_dir).ok();
        fs::remove_dir_all(&fixture.data_dir).ok();
    }
    fn migration_evidence_path(file: &str) -> PathBuf {
        let relative = Path::new("docs/release/evidence/QA-05").join(file);
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .map(|directory| directory.join(&relative))
            .find(|path| path.is_file())
            .unwrap_or_else(|| {
                panic!(
                    "migration evidence fixture not found: {}",
                    relative.display()
                )
            })
    }

    fn legacy_v0_fixture_path() -> PathBuf {
        migration_evidence_path("legacy-v0.sqlite")
    }

    fn migration_test_dir() -> PathBuf {
        std::env::temp_dir().join(format!("asterlab-schema-migration-{}", Uuid::new_v4()))
    }

    type LegacyRecordRow = (String, String, Option<String>, i64, String, String, String);

    fn legacy_rows(db: &Connection) -> Vec<LegacyRecordRow> {
        let mut statement = db
            .prepare(
                "SELECT id,kind,parent_id,revision,data_json,created_at,updated_at \
                 FROM records ORDER BY id",
            )
            .unwrap();
        statement
            .query_map([], |row| {
                Ok((
                    row.get(0)?,
                    row.get(1)?,
                    row.get(2)?,
                    row.get(3)?,
                    row.get(4)?,
                    row.get(5)?,
                    row.get(6)?,
                ))
            })
            .unwrap()
            .collect::<rusqlite::Result<Vec<_>>>()
            .unwrap()
    }

    fn database_schema(db: &Connection) -> Vec<(String, String, Option<String>)> {
        let mut statement = db
            .prepare(
                "SELECT type,name,sql FROM sqlite_master \
                 WHERE name NOT LIKE 'sqlite_%' ORDER BY type,name",
            )
            .unwrap();
        statement
            .query_map([], |row| Ok((row.get(0)?, row.get(1)?, row.get(2)?)))
            .unwrap()
            .collect::<rusqlite::Result<Vec<_>>>()
            .unwrap()
    }

    fn sqlite_user_version(db: &Connection) -> i64 {
        db.query_row("PRAGMA user_version", [], |row| row.get(0))
            .unwrap()
    }

    fn sha256_file(path: &Path) -> String {
        format!("{:x}", Sha256::digest(fs::read(path).unwrap()))
    }

    #[test]
    fn schema_migration_legacy_v0() {
        let fixture = legacy_v0_fixture_path();
        let fixture_hash = sha256_file(&fixture);
        let source = Connection::open(&fixture).unwrap();
        let original_rows = legacy_rows(&source);
        assert_eq!(sqlite_user_version(&source), 0);
        drop(source);

        let data_dir = migration_test_dir();
        fs::create_dir_all(&data_dir).unwrap();
        fs::copy(&fixture, data_dir.join("labworkbench.sqlite")).unwrap();

        let db = open_db(&data_dir).expect("legacy v0 database should migrate");
        assert_eq!(
            db.query_row("PRAGMA integrity_check", [], |row| row.get::<_, String>(0))
                .unwrap(),
            "ok"
        );
        assert_eq!(sqlite_user_version(&db), 1);
        assert_eq!(legacy_rows(&db), original_rows);
        assert_eq!(
            db.query_row(
                "SELECT project_id FROM records WHERE id='design-legacy'",
                [],
                |row| row.get::<_, Option<String>>(0),
            )
            .unwrap()
            .as_deref(),
            Some("project-legacy")
        );
        assert_eq!(
            db.query_row(
                "SELECT COUNT(*) FROM records \
                 WHERE kind <> 'project' AND project_id IS NOT 'project-legacy'",
                [],
                |row| row.get::<_, i64>(0),
            )
            .unwrap(),
            0
        );
        drop(db);

        let restarted = open_db(&data_dir).expect("migrated database should reopen");
        assert_eq!(sqlite_user_version(&restarted), 1);
        assert_eq!(
            restarted
                .query_row("PRAGMA integrity_check", [], |row| row.get::<_, String>(0))
                .unwrap(),
            "ok"
        );
        assert_eq!(legacy_rows(&restarted), original_rows);
        drop(restarted);

        assert_eq!(sha256_file(&fixture), fixture_hash, "source fixture changed");
        fs::remove_dir_all(data_dir).unwrap();
    }

    #[test]
    fn failed_schema_migration_preserves_database() {
        let fixture = legacy_v0_fixture_path();
        let data_dir = migration_test_dir();
        fs::create_dir_all(&data_dir).unwrap();
        let db_file = data_dir.join("labworkbench.sqlite");
        fs::copy(&fixture, &db_file).unwrap();
        {
            let db = Connection::open(&db_file).unwrap();
            let failure_sql =
                fs::read_to_string(migration_evidence_path("failure-v0.sql")).unwrap();
            db.execute_batch(&failure_sql).unwrap();
        }

        let before_hash = sha256_file(&db_file);
        let before = Connection::open(&db_file).unwrap();
        let before_schema = database_schema(&before);
        let before_rows = legacy_rows(&before);
        let before_count: i64 = before
            .query_row("SELECT COUNT(*) FROM records", [], |row| row.get(0))
            .unwrap();
        let before_version = sqlite_user_version(&before);
        drop(before);

        let error = open_db(&data_dir)
            .err()
            .expect("injected migration failure should abort opening the database");
        assert!(error.message.contains("injected legacy schema migration failure"));

        let after = Connection::open(&db_file).unwrap();
        assert_eq!(database_schema(&after), before_schema);
        assert_eq!(legacy_rows(&after), before_rows);
        assert_eq!(
            after
                .query_row("SELECT COUNT(*) FROM records", [], |row| row.get::<_, i64>(0))
                .unwrap(),
            before_count
        );
        assert_eq!(sqlite_user_version(&after), before_version);
        drop(after);
        assert_eq!(sha256_file(&db_file), before_hash);
        fs::remove_dir_all(data_dir).unwrap();
    }
}
