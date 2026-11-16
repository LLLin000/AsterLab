use super::*;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub(crate) enum GroupRole {
    Control,
    Treatment,
    Comparator,
    Reference,
    Other,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct GroupSpec {
    #[serde(default)]
    pub(crate) id: String,
    pub(crate) name: String,
    #[serde(default)]
    pub(crate) role: Option<GroupRole>,
    #[serde(default)]
    pub(crate) condition: Option<String>,
    #[serde(default)]
    pub(crate) independent_n: Option<u32>,
    #[serde(default)]
    pub(crate) technical_repeats: Option<u32>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub(crate) enum LegacySampleRelation {
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
pub(crate) fn is_unknown_legacy_relation(relation: &LegacySampleRelation) -> bool {
    matches!(relation, LegacySampleRelation::Unknown)
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub(crate) enum TimepointRelation {
    SameExperimentalUnitRepeated,
    SamePhysicalSampleRepeated,
    IndependentSamplePerTimepoint,
    Unknown,
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub(crate) enum PhysicalSlotsScope {
    PerTimepoint,
    WholeAssayUniquePositions,
    Unknown,
}
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub(crate) enum SourceRelation {
    SplitFromAssay,
    SequentialAfterAssay,
    NotApplicable,
    Unknown,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub(crate) enum EvidenceState {
    SourceRecord,
    Proposal,
    UserConfirmed,
    Calculated,
    AgentSuggestion,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub(crate) enum ConflictStatus {
    Unresolved,
    Resolved,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub(crate) enum RunRelationship {
    Independent,
    SameSource,
    Paired,
    Unknown,
}
impl RunRelationship {
    pub(crate) fn as_str(&self) -> &'static str {
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
pub(crate) struct AssayPlan {
    pub(crate) id: String,
    pub(crate) title: String,
    #[serde(default)]
    pub(crate) endpoint: String,
    #[serde(default)]
    pub(crate) method: String,
    #[serde(default)]
    pub(crate) unit: String,
    #[serde(default)]
    pub(crate) timepoints: Vec<String>,
    #[serde(default)]
    pub(crate) timepoint_timing: Vec<TimepointTiming>,
    #[serde(default)]
    pub(crate) group_names: Vec<String>,
    #[serde(default)]
    pub(crate) group_ids: Vec<String>,
    #[serde(default)]
    pub(crate) time_anchor_event: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub(crate) sample_use_policy: Option<LegacySampleRelation>,
    #[serde(default)]
    pub(crate) timepoint_relation: Option<TimepointRelation>,
    #[serde(default)]
    pub(crate) container_type: Option<String>,
    #[serde(default)]
    pub(crate) container_format: Option<String>,
    #[serde(default)]
    pub(crate) protocol_version_id: Option<String>,
    #[serde(default)]
    pub(crate) controls: Vec<String>,
    #[serde(default)]
    pub(crate) control_group_ids: Vec<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct TimepointTiming {
    pub(crate) timepoint: String,
    #[serde(default)]
    pub(crate) offset_value: Option<f64>,
    pub(crate) unit: String,
    #[serde(default)]
    pub(crate) window_before: Option<f64>,
    #[serde(default)]
    pub(crate) window_after: Option<f64>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SampleAllocation {
    pub(crate) assay_id: String,
    pub(crate) group_name: String,
    #[serde(default)]
    pub(crate) group_id: Option<String>,
    // Legacy input only. normalize_design migrates this before omitting it from output.
    #[serde(default, skip_serializing_if = "is_unknown_legacy_relation")]
    pub(crate) relation: LegacySampleRelation,
    #[serde(default)]
    pub(crate) timepoint_relation: Option<TimepointRelation>,
    #[serde(default)]
    pub(crate) source_relation: Option<SourceRelation>,
    #[serde(default)]
    pub(crate) source_assay_id: Option<String>,
    #[serde(default)]
    pub(crate) physical_slots_per_group: Option<u32>,
    #[serde(default)]
    pub(crate) physical_slots_scope: Option<PhysicalSlotsScope>,
    #[serde(default)]
    pub(crate) evidence_id: Option<String>,
    #[serde(default)]
    pub(crate) planned_output_id: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct PlannedOutput {
    pub(crate) id: String,
    pub(crate) title: String,
    pub(crate) protocol_version_id: String,
    #[serde(default)]
    pub(crate) root_bindings: Vec<RootParameterValue>,
    #[serde(default)]
    pub(crate) evidence_id: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SourceEvidence {
    pub(crate) id: String,
    pub(crate) source_document: String,
    #[serde(default)]
    pub(crate) source_date: Option<String>,
    #[serde(default)]
    pub(crate) source_locator: Option<String>,
    pub(crate) claim: String,
    pub(crate) state: EvidenceState,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EvidenceConflict {
    pub(crate) id: String,
    pub(crate) topic: String,
    pub(crate) evidence_ids: Vec<String>,
    pub(crate) status: ConflictStatus,
    #[serde(default)]
    pub(crate) affected_assay_ids: Vec<String>,
    #[serde(default)]
    pub(crate) affects_container_estimate: bool,
    #[serde(default)]
    pub(crate) resolution: Option<String>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct DesignFields {
    pub(crate) title: String,
    #[serde(default)]
    pub(crate) objective: String,
    #[serde(default)]
    pub(crate) research_question: Option<String>,
    #[serde(default)]
    pub(crate) factors: Vec<asterlab_core::design_map::DesignFactor>,
    #[serde(default)]
    pub(crate) experimental_unit: String,
    #[serde(default)]
    pub(crate) group_specs: Vec<GroupSpec>,
    #[serde(default)]
    pub(crate) timepoints: Vec<String>,
    #[serde(default)]
    pub(crate) primary_endpoint: String,
    #[serde(default)]
    pub(crate) endpoint_method: String,
    #[serde(default)]
    pub(crate) endpoint_unit: String,
    #[serde(default)]
    pub(crate) sample_size_rationale: String,
    #[serde(default)]
    pub(crate) randomization: String,
    #[serde(default)]
    pub(crate) blinding: String,
    #[serde(default)]
    pub(crate) inclusion_criteria: String,
    #[serde(default)]
    pub(crate) exclusion_criteria: String,
    #[serde(default)]
    pub(crate) analysis_plan: String,
    #[serde(default)]
    pub(crate) controls: String,
    #[serde(default)]
    pub(crate) assays: Vec<AssayPlan>,
    #[serde(default)]
    pub(crate) sample_allocations: Vec<SampleAllocation>,
    #[serde(default)]
    pub(crate) planned_outputs: Vec<PlannedOutput>,
    #[serde(default)]
    pub(crate) container_plan: ContainerPlan,
    #[serde(default)]
    pub(crate) source_evidence: Vec<SourceEvidence>,
    #[serde(default)]
    pub(crate) evidence_conflicts: Vec<EvidenceConflict>,
    #[serde(default)]
    pub(crate) planned_run_count: Option<u32>,
    #[serde(default)]
    pub(crate) planned_run_relationship: RunRelationship,
    // Kept for compatibility with the browser form and older imported records.
    #[serde(default)]
    pub(crate) groups: String,
    #[serde(default)]
    pub(crate) hypothesis: String,
    #[serde(default)]
    pub(crate) replicate_plan: String,
    #[serde(default)]
    pub(crate) independent_variables: String,
    // Frozen SDK module payload. Authored through AsterDoc semantic commands;
    // legacy Design writes do not normally carry it.
    #[serde(default)]
    pub(crate) asterlab: Option<Value>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct DesignCreate {
    pub(crate) project_id: String,
    pub(crate) design: DesignFields,
}
#[derive(Debug, Default, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct DesignChanges {
    pub(crate) title: Option<String>,
    pub(crate) objective: Option<String>,
    pub(crate) research_question: Option<Option<String>>,
    pub(crate) factors: Option<Vec<asterlab_core::design_map::DesignFactor>>,
    pub(crate) experimental_unit: Option<String>,
    pub(crate) group_specs: Option<Vec<GroupSpec>>,
    pub(crate) timepoints: Option<Vec<String>>,
    pub(crate) primary_endpoint: Option<String>,
    pub(crate) endpoint_method: Option<String>,
    pub(crate) endpoint_unit: Option<String>,
    pub(crate) sample_size_rationale: Option<String>,
    pub(crate) randomization: Option<String>,
    pub(crate) blinding: Option<String>,
    pub(crate) inclusion_criteria: Option<String>,
    pub(crate) exclusion_criteria: Option<String>,
    pub(crate) analysis_plan: Option<String>,
    pub(crate) controls: Option<String>,
    pub(crate) assays: Option<Vec<AssayPlan>>,
    pub(crate) sample_allocations: Option<Vec<SampleAllocation>>,
    pub(crate) planned_outputs: Option<Vec<PlannedOutput>>,
    pub(crate) container_plan: Option<ContainerPlan>,
    pub(crate) source_evidence: Option<Vec<SourceEvidence>>,
    pub(crate) evidence_conflicts: Option<Vec<EvidenceConflict>>,
    pub(crate) planned_run_count: Option<u32>,
    pub(crate) planned_run_relationship: Option<RunRelationship>,
    pub(crate) groups: Option<String>,
    pub(crate) hypothesis: Option<String>,
    pub(crate) replicate_plan: Option<String>,
    pub(crate) independent_variables: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct DesignRevise {
    pub(crate) id: String,
    pub(crate) expected_revision: i64,
    pub(crate) changes: DesignChanges,
}
#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProjectFilter {
    pub(crate) project_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct EmptyInput {}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct IdInput {
    pub(crate) id: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RunWorkspaceInput {
    pub(crate) id: String,
    /// `blind` masks frozen group identity; `unblinded` (default) renders it.
    #[serde(default)]
    pub(crate) view: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct InspectTreeInput {
    pub(crate) design_id: String,
    #[serde(default)]
    pub(crate) revision: Option<u64>,
    #[serde(default)]
    pub(crate) run_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct WorkflowProgressInput {
    pub(crate) run_id: String,
    #[serde(default)]
    pub(crate) step_path: Option<String>,
    #[serde(default)]
    pub(crate) timepoint: Option<String>,
    #[serde(default)]
    pub(crate) sample_id: Option<String>,
    #[serde(default)]
    pub(crate) sample_ids: Vec<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProjectCreate {
    pub(crate) name: String,
    #[serde(default)]
    pub(crate) description: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProjectRevise {
    pub(crate) id: String,
    pub(crate) expected_revision: i64,
    #[serde(default)]
    pub(crate) name: Option<String>,
    #[serde(default)]
    pub(crate) description: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RunCreate {
    pub(crate) design_id: String,
    pub(crate) title: String,
    #[serde(default)]
    pub(crate) scheduled_date: Option<String>,
    #[serde(default)]
    pub(crate) protocol_version_id: Option<String>,
    #[serde(default)]
    pub(crate) view_id: Option<String>,
    #[serde(default)]
    pub(crate) repeat_of_run_id: Option<String>,
    #[serde(default)]
    pub(crate) repeat_relationship: RunRelationship,
    #[serde(default)]
    pub(crate) source_run_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RunRevise {
    pub(crate) id: String,
    pub(crate) expected_revision: i64,
    #[serde(default)]
    pub(crate) title: Option<String>,
    #[serde(default)]
    pub(crate) scheduled_date: Option<Option<String>>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RunPatch {
    #[serde(default)]
    pub(crate) title: Option<String>,
    #[serde(default)]
    pub(crate) scheduled_date: Option<Option<String>>,
    #[serde(default)]
    pub(crate) status: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct TaskCreate {
    pub(crate) project_id: String,
    #[serde(default)]
    pub(crate) run_id: Option<String>,
    pub(crate) title: String,
    pub(crate) scheduled_date: String,
    #[serde(default = "planned")]
    pub(crate) status: String,
    #[serde(default)]
    pub(crate) notes: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct StatusUpdate {
    pub(crate) id: String,
    pub(crate) expected_revision: i64,
    pub(crate) status: String,
}
pub(crate) fn planned() -> String {
    "planned".into()
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolCreate {
    #[serde(default)]
    pub(crate) project_id: Option<String>,
    pub(crate) title: String,
    #[serde(default)]
    pub(crate) body: Option<String>,
    #[serde(default)]
    pub(crate) root_block_version_id: Option<String>,
    #[serde(default)]
    pub(crate) root_bindings: Vec<RootParameterValue>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolRevise {
    pub(crate) protocol_id: String,
    pub(crate) expected_version: i64,
    #[serde(default)]
    pub(crate) body: Option<String>,
    #[serde(default)]
    pub(crate) root_block_version_id: Option<Value>,
    #[serde(default)]
    pub(crate) root_bindings: Option<Vec<RootParameterValue>>,
}
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolBlockCreate {
    pub(crate) project_id: String,
    pub(crate) title: String,
    #[serde(default)]
    pub(crate) summary: String,
    #[serde(default)]
    pub(crate) parameter_slots: Vec<ParameterSlot>,
    pub(crate) steps: Vec<ProtocolStep>,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct CatalogOperationImport {
    pub(crate) project_id: String,
    pub(crate) module: OperationModule,
    pub(crate) review_id: String,
    #[serde(default)]
    pub(crate) summary: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SemanticInspectInput {
    pub(crate) block_version_id: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SemanticSearchInput {
    #[serde(default)]
    pub(crate) query: String,
    #[serde(default)]
    pub(crate) project_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct SemanticPatchInput {
    pub(crate) changeset: ChangeSet,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewCreate {
    pub(crate) project_id: String,
    pub(crate) title: String,
    pub(crate) document: ViewDocument,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct LabwareDefine {
    pub(crate) profile: LabwareProfile,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ComponentDefine {
    pub(crate) definition: ComponentDefinition,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewProject {
    pub(crate) project_id: String,
    pub(crate) protocol_version_id: String,
    pub(crate) scope: ProtocolScope,
    #[serde(default)]
    pub(crate) document_id: Option<String>,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewDataPreview {
    pub(crate) view_id: String,
    pub(crate) instance_id: String,
    #[serde(default)]
    pub(crate) position_ids: Vec<String>,
    #[serde(default)]
    pub(crate) calculator_instance_id: Option<String>,
    #[serde(default)]
    pub(crate) volume_per_well: Option<f64>,
    #[serde(default)]
    pub(crate) volume_unit: Option<String>,
    #[serde(default)]
    pub(crate) extra_ratio: Option<f64>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewGet {
    pub(crate) id: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewRevise {
    pub(crate) view_id: String,
    pub(crate) changeset: ViewChangeSet,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewProposalCreate {
    pub(crate) view_id: String,
    pub(crate) changeset: ViewChangeSet,
    #[serde(default)]
    pub(crate) source: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewProposalAction {
    pub(crate) view_id: String,
    pub(crate) proposal_id: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ViewProposalList {
    pub(crate) view_id: String,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct CatalogReviewCreate {
    pub(crate) project_id: String,
    pub(crate) review: ReviewRecord,
}
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolBlockRevise {
    pub(crate) block_id: String,
    pub(crate) expected_version: i64,
    #[serde(default)]
    pub(crate) summary: Option<String>,
    #[serde(default)]
    pub(crate) parameter_slots: Vec<ParameterSlot>,
    pub(crate) steps: Vec<ProtocolStep>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolBlockList {
    #[serde(default)]
    pub(crate) project_id: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolBlockGet {
    pub(crate) id: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolVersionGet {
    pub(crate) protocol_version_id: String,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolVersionList {
    pub(crate) protocol_id: String,
}
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct RootParameterValue {
    pub(crate) slot: String,
    pub(crate) value: Value,
    #[serde(default)]
    pub(crate) unit: Option<String>,
}
#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct ProtocolBlockPreview {
    pub(crate) block_version_id: String,
    #[serde(default)]
    pub(crate) bindings: Vec<RootParameterValue>,
}
#[derive(Debug, Clone)]
pub(crate) struct BoundValue {
    pub(crate) value: Value,
    pub(crate) unit: Option<String>,
    pub(crate) provenance: Vec<Value>,
}
