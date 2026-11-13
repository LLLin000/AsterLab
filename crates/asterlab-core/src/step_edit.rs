//! `step.edit`: block-level step editing inside the agent edit v1 flow.
//!
//! The four stages of `docs/contracts/agent-edit-v1.md` stay the same:
//! propose (Agent assembles ops from read models) -> validate (this module,
//! read-only) -> confirm (human) -> commit (new versions in one transaction).
//! What this command adds is *structural* editing of `ProtocolBlock` trees:
//! `insert`, `reorder`, `reuse` and `bind`, each bound by `expected_version`.
//!
//! Authority split:
//!
//! - an Agent never assembles a version number. Core re-reads the head, assigns
//!   `previous_version + 1` and rewrites every parent reference to the new
//!   child version id it just created; the caller only appends records;
//! - paths use the workflow/Protocol expansion addressing (`root`,
//!   `root/<use_id>`, `root/<use_id>/<use_id>`), so a deep sub-step edit is the
//!   same command as a top-level one;
//! - a path that leaves the root tree is `FORBIDDEN_FIELD`; a syntactically
//!   valid path that does not resolve is `STALE_REFERENCE`;
//! - immutable versions are never mutated: a changed block becomes a new
//!   version carrying `based_on_version_id`, an unchanged block keeps its exact
//!   version (reuse), and every frozen Run snapshot remains untouched.

use crate::agent_edit::{
    AgentEditError, AgentEditErrorCode, AgentEditIssue, IssueSeverity, ProposalOrigin,
    ValidationStatus, AGENT_EDIT_SCHEMA_VERSION,
};
use crate::operation_module::{ModuleOrigin, ParameterValueType};
use crate::protocol_block::{
    ModuleContractAuthority, ParameterSlot, ProtocolBlockVersionData, ProtocolStep, StepBinding,
};
use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, BTreeSet, HashSet};

pub const STEP_EDIT_SCHEMA_VERSION: u32 = AGENT_EDIT_SCHEMA_VERSION;

/// Upper bounds mirror the block writer (`protocol.block.create/revise`) so an
/// edit cannot produce a block the existing commands would refuse to store.
const MAX_OPS: usize = 100;
const MAX_STEPS: usize = 100;
const MAX_DEPTH: usize = 12;

fn default_schema_version() -> u32 {
    STEP_EDIT_SCHEMA_VERSION
}

/// Block-level operation kind. `op` is the wire discriminator.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum StepEditOpKind {
    Insert,
    Reorder,
    Reuse,
    Bind,
}

impl StepEditOpKind {
    fn label(self) -> &'static str {
        match self {
            StepEditOpKind::Insert => "插入",
            StepEditOpKind::Reorder => "重排",
            StepEditOpKind::Reuse => "复用",
            StepEditOpKind::Bind => "绑定",
        }
    }
}

/// One block-level operation. `path` addresses a container (`root` or a block
/// use step) or, for `bind`, the block use step itself.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "op", rename_all = "snake_case", deny_unknown_fields)]
pub enum StepEditOp {
    /// Insert a new readable text step at `position` (0-based) in the container.
    Insert {
        path: String,
        position: usize,
        step: ProtocolStep,
    },
    /// Replace the order of the container's steps with an exact permutation.
    Reorder { path: String, order: Vec<String> },
    /// Insert a block use of an existing immutable block version.
    Reuse {
        path: String,
        position: usize,
        use_id: String,
        block_version_id: String,
        #[serde(default)]
        bindings: Vec<StepBinding>,
    },
    /// Upsert parameter bindings of one existing block use step.
    Bind {
        path: String,
        bindings: Vec<StepBinding>,
    },
}

impl StepEditOp {
    pub fn kind(&self) -> StepEditOpKind {
        match self {
            StepEditOp::Insert { .. } => StepEditOpKind::Insert,
            StepEditOp::Reorder { .. } => StepEditOpKind::Reorder,
            StepEditOp::Reuse { .. } => StepEditOpKind::Reuse,
            StepEditOp::Bind { .. } => StepEditOpKind::Bind,
        }
    }

    pub fn path(&self) -> &str {
        match self {
            StepEditOp::Insert { path, .. }
            | StepEditOp::Reorder { path, .. }
            | StepEditOp::Reuse { path, .. }
            | StepEditOp::Bind { path, .. } => path,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepEditConfirmation {
    /// Must equal [`StepEditResult::edit_digest`].
    pub edit_digest: String,
    pub confirmed_by: String,
    pub idempotency_key: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
}

/// One `step.edit` request. Without `confirmation` it is the validate stage;
/// with it, the commit stage of the same proposal.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepEditRequest {
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    pub edit_id: String,
    /// Exact immutable root block version the ops were assembled against.
    pub block_version_id: String,
    /// `revision` (== `version`) of the root block's head record.
    pub expected_version: i64,
    pub origin: ProposalOrigin,
    pub ops: Vec<StepEditOp>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub confirmation: Option<StepEditConfirmation>,
}

/// One immutable block version as the store returns it.
#[derive(Debug, Clone, PartialEq)]
pub struct StoredEditBlock {
    pub version_id: String,
    pub block_id: String,
    pub revision: i64,
    pub project_id: Option<String>,
    pub data: ProtocolBlockVersionData,
}

/// Current head of a `protocol_block` record.
#[derive(Debug, Clone, PartialEq)]
pub struct StoredBlockHead {
    pub block_id: String,
    pub version_id: String,
    pub revision: i64,
}

/// Read-only store access. Validation uses these calls as its only access.
pub trait StepEditStore {
    fn step_block_version(&self, version_id: &str) -> Result<Option<StoredEditBlock>, String>;
    fn step_block_head(&self, block_id: &str) -> Result<Option<StoredBlockHead>, String>;
}

/// One version appended by a previous `step.edit` commit, used for replay.
#[derive(Debug, Clone, PartialEq)]
pub struct CommittedStepEdit {
    pub version_id: String,
    pub revision: i64,
    /// The `step_edit` provenance object stored on the version record.
    pub provenance: Value,
}

/// Transactional store used by [`commit`].
pub trait StepEditCommitStore {
    fn step_store(&self) -> &dyn StepEditStore;
    fn find_step_commit(&self, idempotency_key: &str) -> Result<Vec<CommittedStepEdit>, String>;
    /// Core clock, RFC 3339. Supplied by the adapter.
    fn timestamp(&self) -> String;
    fn append_block_version(
        &mut self,
        block_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String>;
}

/// Provenance stored as `data.step_edit` on every version this command
/// creates. Typed (not an extra JSON key) because block version payloads are
/// parsed with `deny_unknown_fields`.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepEditProvenance {
    pub edit_id: String,
    pub edit_digest: String,
    pub idempotency_key: String,
    pub confirmed_by: String,
    pub committed_at: String,
    pub block_id: String,
    pub based_on_version_id: String,
    pub invalidated_refs: Vec<String>,
}

/// One op after resolution against the authoritative tree.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ResolvedStepOp {
    pub op: StepEditOpKind,
    pub path: String,
    pub changed: bool,
    pub description: String,
}

/// One block version this edit would create.
///
/// `new_version` is `previous_version + 1`; the version *record id* is assigned
/// by the store at commit time, so the preview never invents one.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct PlannedBlockView {
    pub block_id: String,
    pub based_on_version_id: String,
    pub previous_version: i64,
    pub new_version: i64,
    /// Resulting step order (use ids), so the confirming human can read the
    /// structure without a second command.
    pub order: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepEditResult {
    pub schema_version: u32,
    pub edit_id: String,
    pub edit_digest: String,
    pub block_version_id: String,
    pub block_id: String,
    pub current_revision: i64,
    pub status: ValidationStatus,
    pub requires_confirmation: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub confirmation_prompt: Option<String>,
    pub ops: Vec<ResolvedStepOp>,
    /// Child blocks before their parents: a parent's new version references the
    /// child version created in the same commit.
    pub blocks: Vec<PlannedBlockView>,
    pub issues: Vec<AgentEditIssue>,
    pub invalidated_refs: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CreatedBlockVersion {
    pub block_id: String,
    pub based_on_version_id: String,
    pub version_id: String,
    pub version: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepEditCommitResult {
    pub schema_version: u32,
    pub edit_id: String,
    pub edit_digest: String,
    pub block_version_id: String,
    pub block_id: String,
    pub previous_version_id: String,
    pub new_root_block_version_id: String,
    pub new_revision: i64,
    pub created_versions: Vec<CreatedBlockVersion>,
    pub invalidated_refs: Vec<String>,
    pub confirmed_by: String,
    pub committed_at: String,
    pub idempotency_key: String,
    pub replayed: bool,
}

/// Commit failure. `Store` is infrastructure; `Rejected` wrote nothing.
#[derive(Debug, Clone, PartialEq)]
pub enum StepEditCommitError {
    Rejected(AgentEditError),
    Store(String),
}

/// Canonical `sha256:<hex>` over the request without its confirmation block:
/// the digest binds "the ops the human confirmed", not the confirmation itself.
pub fn edit_digest(request: &StepEditRequest) -> String {
    #[derive(Serialize)]
    struct DigestSource<'a> {
        schema_version: u32,
        edit_id: &'a str,
        block_version_id: &'a str,
        expected_version: i64,
        origin: &'a ProposalOrigin,
        ops: &'a [StepEditOp],
    }
    let source = DigestSource {
        schema_version: request.schema_version,
        edit_id: &request.edit_id,
        block_version_id: &request.block_version_id,
        expected_version: request.expected_version,
        origin: &request.origin,
        ops: &request.ops,
    };
    let value = serde_json::to_value(source).unwrap_or(Value::Null);
    let bytes = serde_json::to_vec(&canonicalize(value)).unwrap_or_default();
    let hash = Sha256::digest(bytes);
    format!(
        "sha256:{}",
        hash.iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
}

fn canonicalize(value: Value) -> Value {
    match value {
        Value::Object(object) => {
            let sorted = object
                .into_iter()
                .map(|(key, value)| (key, canonicalize(value)))
                .collect::<BTreeMap<_, _>>();
            let mut out = Map::new();
            for (key, value) in sorted {
                out.insert(key, value);
            }
            Value::Object(out)
        }
        Value::Array(items) => Value::Array(items.into_iter().map(canonicalize).collect()),
        other => other,
    }
}

fn issue(
    code: AgentEditErrorCode,
    path: impl Into<String>,
    message: impl Into<String>,
    next_action: impl Into<String>,
) -> AgentEditIssue {
    AgentEditIssue {
        code,
        severity: IssueSeverity::Blocking,
        path: path.into(),
        message: message.into(),
        affects: Vec::new(),
        next_action: next_action.into(),
    }
}

fn issue_affecting(
    code: AgentEditErrorCode,
    path: impl Into<String>,
    message: impl Into<String>,
    next_action: impl Into<String>,
    affects: Vec<String>,
) -> AgentEditIssue {
    AgentEditIssue {
        code,
        severity: IssueSeverity::Blocking,
        path: path.into(),
        message: message.into(),
        affects,
        next_action: next_action.into(),
    }
}

// ---------------------------------------------------------------------------
// Tree
// ---------------------------------------------------------------------------

struct ArenaBlock {
    version_id: String,
    block_id: String,
    revision: i64,
    project_id: Option<String>,
    data: ProtocolBlockVersionData,
    original_steps: Vec<ProtocolStep>,
    steps: Vec<ProtocolStep>,
}

#[derive(Default)]
struct Arena {
    blocks: Vec<ArenaBlock>,
    by_version: BTreeMap<String, usize>,
}

enum LoadError {
    Issue(Box<AgentEditIssue>),
    Store(String),
}

fn load_error(
    code: AgentEditErrorCode,
    path: impl Into<String>,
    message: impl Into<String>,
    next: impl Into<String>,
) -> LoadError {
    LoadError::Issue(Box::new(issue(code, path, message, next)))
}

/// Load one block version and, recursively, every block use it references.
fn load_block(
    store: &dyn StepEditStore,
    arena: &mut Arena,
    version_id: &str,
    path: &str,
    active: &mut Vec<String>,
) -> Result<usize, LoadError> {
    // Active is checked before the arena cache: a version already on the
    // current path is a cycle even though it has been loaded.
    if active.iter().any(|id| id == version_id) {
        return Err(load_error(
            AgentEditErrorCode::InvalidProposal,
            path,
            format!("步骤块组合出现循环引用：{version_id}"),
            "先解除循环引用，再提交编辑",
        ));
    }
    if let Some(index) = arena.by_version.get(version_id) {
        return Ok(*index);
    }
    if active.len() >= MAX_DEPTH {
        return Err(load_error(
            AgentEditErrorCode::InvalidProposal,
            path,
            format!("步骤块组合超过 {MAX_DEPTH} 层；无法定位这个路径"),
            "减少嵌套层级后重新组稿",
        ));
    }
    let stored = store
        .step_block_version(version_id)
        .map_err(LoadError::Store)?
        .ok_or_else(|| {
            load_error(
                AgentEditErrorCode::StaleReference,
                path,
                format!("步骤块版本 {version_id} 不存在"),
                "重新读取 protocol.block.get 后重新组稿",
            )
        })?;
    let index = arena.blocks.len();
    arena
        .by_version
        .insert(stored.version_id.clone(), index);
    arena.blocks.push(ArenaBlock {
        version_id: stored.version_id.clone(),
        block_id: stored.block_id.clone(),
        revision: stored.revision,
        project_id: stored.project_id.clone(),
        data: stored.data.clone(),
        original_steps: stored.data.steps.clone(),
        steps: stored.data.steps.clone(),
    });
    active.push(stored.version_id.clone());
    for step in arena.blocks[index].steps.clone() {
        if let ProtocolStep::BlockUse {
            use_id,
            block_version_id,
            ..
        } = &step
        {
            let child_path = format!("{path}/{use_id}");
            let parent_project = arena.blocks[index].project_id.clone();
            let child = load_block(store, arena, block_version_id, &child_path, active)?;
            let child_project = arena.blocks[child].project_id.clone();
            if parent_project.is_some()
                && child_project.is_some()
                && parent_project != child_project
            {
                return Err(load_error(
                    AgentEditErrorCode::InvalidBinding,
                    &child_path,
                    format!("步骤 {use_id} 引用的块版本属于另一个项目"),
                    "只复用同一项目的步骤块版本",
                ));
            }
        }
    }
    active.pop();
    Ok(index)
}

fn collect_subtree_ids(arena: &Arena, index: usize, out: &mut BTreeSet<String>) {
    if !out.insert(arena.blocks[index].block_id.clone()) {
        return;
    }
    for step in &arena.blocks[index].steps {
        if let ProtocolStep::BlockUse {
            block_version_id, ..
        } = step
        {
            if let Some(child) = arena.by_version.get(block_version_id) {
                collect_subtree_ids(arena, *child, out);
            }
        }
    }
}

struct PathTarget {
    container: usize,
    step: Option<(usize, usize)>,
    chain: Vec<String>,
}

/// Split a step path into segments, rejecting anything that leaves the root
/// tree or is not a step path at all.
fn path_segments(path: &str) -> Result<Vec<&str>, AgentEditIssue> {
    let trimmed = path.trim();
    if trimmed.is_empty() {
        return Err(issue(
            AgentEditErrorCode::ForbiddenField,
            "path",
            "path 不能为空；根容器写作 root",
            "用 root 或 root/<use_id> 寻址",
        ));
    }
    let segments = trimmed.split('/').collect::<Vec<_>>();
    if segments.len() > MAX_DEPTH + 1 {
        return Err(issue_affecting(
            AgentEditErrorCode::ForbiddenField,
            "path",
            format!("path {path} 超过 {MAX_DEPTH} 层嵌套"),
            "减少嵌套层级后重新组稿",
            vec!["全部 op".to_owned()],
        ));
    }
    if segments[0] != "root" {
        return Err(issue_affecting(
            AgentEditErrorCode::ForbiddenField,
            "path",
            format!("path {path} 必须以 root 开头"),
            "用 workflow.inspect_tree / protocol.block.preview 的 path 重新组稿",
            vec!["全部 op".to_owned()],
        ));
    }
    for segment in segments.iter().skip(1) {
        if segment.is_empty() || *segment == "." || *segment == ".." {
            return Err(issue_affecting(
                AgentEditErrorCode::ForbiddenField,
                "path",
                format!("path {path} 含越界或空段"),
                "用读模型给出的 use_id 重新组稿",
                vec!["全部 op".to_owned()],
            ));
        }
    }
    Ok(segments)
}

fn find_step(steps: &[ProtocolStep], use_id: &str) -> Option<usize> {
    steps.iter().position(|step| match step {
        ProtocolStep::Text { use_id: id, .. } | ProtocolStep::BlockUse { use_id: id, .. } => {
            id == use_id
        }
    })
}

fn resolve_path(arena: &Arena, root: usize, path: &str) -> Result<PathTarget, AgentEditIssue> {
    let segments = path_segments(path)?;
    let mut container = root;
    let mut step = None;
    let mut chain = vec![arena.blocks[root].block_id.clone()];
    for (offset, segment) in segments.iter().enumerate().skip(1) {
        let index = find_step(&arena.blocks[container].steps, segment).ok_or_else(|| {
            issue(
                AgentEditErrorCode::StaleReference,
                "path",
                format!(
                    "path {path} 在 {} 中找不到步骤 {segment}",
                    arena.blocks[container].version_id
                ),
                "重新读取 protocol.block.preview 后重新组稿",
            )
        })?;
        let last = offset + 1 == segments.len();
        step = Some((container, index));
        match &arena.blocks[container].steps[index] {
            ProtocolStep::BlockUse {
                block_version_id, ..
            } => {
                let child = *arena.by_version.get(block_version_id).ok_or_else(|| {
                    issue(
                        AgentEditErrorCode::StaleReference,
                        "path",
                        format!("子块版本 {block_version_id} 未加载；组合链已损坏"),
                        "重新读取 protocol.block.get",
                    )
                })?;
                if !last {
                    container = child;
                    chain.push(arena.blocks[child].block_id.clone());
                } else {
                    container = child;
                }
            }
            ProtocolStep::Text { .. } => {
                if !last {
                    return Err(issue(
                        AgentEditErrorCode::InvalidProposal,
                        "path",
                        format!("path {path} 的中段 {segment} 是文字步骤，不能再展开"),
                        "只对块引用步骤继续向下寻址",
                    ));
                }
            }
        }
    }
    Ok(PathTarget {
        container,
        step,
        chain,
    })
}

// ---------------------------------------------------------------------------
// Validation primitives
// ---------------------------------------------------------------------------

fn normalized_len(value: &str) -> usize {
    value.trim().chars().count()
}

fn slot_map(slots: &[ParameterSlot]) -> BTreeMap<&str, &ParameterSlot> {
    slots.iter().map(|slot| (slot.key.as_str(), slot)).collect()
}

fn type_name(value_type: ParameterValueType) -> &'static str {
    match value_type {
        ParameterValueType::Text => "text",
        ParameterValueType::Number => "number",
        ParameterValueType::Integer => "integer",
        ParameterValueType::Boolean => "boolean",
    }
}

fn value_matches(value_type: ParameterValueType, value: &Value) -> bool {
    match value_type {
        ParameterValueType::Text => value.as_str().is_some(),
        ParameterValueType::Number => value.as_f64().is_some_and(f64::is_finite),
        ParameterValueType::Integer => value.as_i64().is_some() || value.as_u64().is_some(),
        ParameterValueType::Boolean => value.as_bool().is_some(),
    }
}

fn validate_use_id(use_id: &str, container: &[ProtocolStep], path: &str) -> Vec<AgentEditIssue> {
    let mut issues = Vec::new();
    if use_id.trim().is_empty() || use_id.chars().count() > 80 {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "use_id",
            "use_id 必须为 1–80 个字符",
            "换一个稳定 use_id",
        ));
    } else if !use_id
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || c == '_' || c == '-')
    {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "use_id",
            format!("use_id 只能包含字母、数字、下划线或连字符：{use_id}"),
            "换一个稳定 use_id",
        ));
    }
    if find_step(container, use_id).is_some() {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "use_id",
            format!("{path} 中已存在 use_id {use_id}"),
            "同一块内 use_id 必须唯一",
        ));
    }
    issues
}

fn validate_text_step(
    step: &ProtocolStep,
    container: &[ProtocolStep],
    container_slots: &[ParameterSlot],
    path: &str,
) -> Vec<AgentEditIssue> {
    let ProtocolStep::Text {
        use_id,
        title,
        instruction,
        parameter_refs,
        checks,
        ..
    } = step
    else {
        return vec![issue(
            AgentEditErrorCode::InvalidProposal,
            "step.kind",
            "insert 只接受文字步骤；复用已有步骤块请用 op=reuse",
            "把 step.kind 改为 text，或用 reuse 引用确切块版本",
        )];
    };
    let mut issues = validate_use_id(use_id, container, path);
    if normalized_len(title) == 0 || title.chars().count() > 200 {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "step.title",
            "步骤标题必须为 1–200 个非空字符",
            "给步骤一个可读标题",
        ));
    }
    if normalized_len(instruction) == 0 || instruction.chars().count() > 4000 {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "step.instruction",
            "步骤说明必须为 1–4000 个非空字符",
            "写清这一步具体怎么做",
        ));
    }
    let slots = slot_map(container_slots);
    let mut seen = HashSet::new();
    for key in parameter_refs {
        if !slots.contains_key(key.as_str()) {
            issues.push(issue(
                AgentEditErrorCode::InvalidBinding,
                "step.parameter_refs",
                format!("步骤 {use_id} 引用了未定义的参数槽位：{key}"),
                "只用本块 parameter_slots 中已声明的槽位",
            ));
        }
        if !seen.insert(key.as_str()) {
            issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                "step.parameter_refs",
                format!("步骤 {use_id} 重复引用参数：{key}"),
                "去掉重复引用",
            ));
        }
    }
    if checks.len() > 50 {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "step.checks",
            "单个步骤最多定义 50 个检查项",
            "拆分成多个步骤",
        ));
    }
    let mut check_ids = HashSet::new();
    for check in checks {
        if normalized_len(&check.check_id) == 0
            || check.check_id.chars().count() > 80
            || normalized_len(&check.label) == 0
            || check.label.chars().count() > 200
        {
            issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                "step.checks",
                "检查项 ID 与名称必须非空且不超过 80/200 个字符",
                "修正检查项定义",
            ));
        }
        if !check_ids.insert(check.check_id.as_str()) {
            issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                "step.checks",
                format!("步骤 {use_id} 的检查项 ID 重复：{}", check.check_id),
                "检查项 ID 必须唯一",
            ));
        }
    }
    issues
}

/// Checks one binding set against the child slots and the parent slots, using
/// the same rules as `protocol.block.create/revise`. No unit is converted.
fn validate_bindings(
    path: &str,
    parent_slots: &[ParameterSlot],
    child_slots: &[ParameterSlot],
    bindings: &[StepBinding],
) -> Vec<AgentEditIssue> {
    let mut issues = Vec::new();
    let parents = slot_map(parent_slots);
    let children = slot_map(child_slots);
    let mut mapped = HashSet::new();
    for binding in bindings {
        let Some(child) = children.get(binding.slot.as_str()) else {
            issues.push(issue(
                AgentEditErrorCode::InvalidBinding,
                path,
                format!("子块不存在参数槽位：{}", binding.slot),
                "按子块 parameter_slots 重新绑定",
            ));
            continue;
        };
        if !mapped.insert(binding.slot.as_str()) {
            issues.push(issue(
                AgentEditErrorCode::InvalidBinding,
                path,
                format!("子块参数 {} 被重复绑定", binding.slot),
                "每个槽位只绑定一次",
            ));
            continue;
        }
        match (&binding.value, &binding.source_slot) {
            (Some(value), None) => {
                if !value_matches(child.value_type, value) {
                    issues.push(issue(
                        AgentEditErrorCode::InvalidBinding,
                        path,
                        format!(
                            "参数 {} 的值类型应为 {}",
                            binding.slot,
                            type_name(child.value_type)
                        ),
                        "按槽位 value_type 重新给值",
                    ));
                }
                if child.unit.as_deref() != binding.unit.as_deref() {
                    issues.push(issue(
                        AgentEditErrorCode::UnitMismatch,
                        path,
                        format!(
                            "参数 {} 的单位必须与槽位声明完全一致（{}，收到 {}）；不做隐式换算",
                            binding.slot,
                            child.unit.as_deref().unwrap_or("无单位"),
                            binding.unit.as_deref().unwrap_or("无单位")
                        ),
                        "用槽位单位重新提交；改单位定义必须另提步骤块新版本",
                    ));
                }
            }
            (None, Some(source_key)) => {
                if binding.unit.is_some() {
                    issues.push(issue(
                        AgentEditErrorCode::InvalidBinding,
                        path,
                        "source_slot 绑定不能另填 unit；单位来自父参数槽位",
                        "去掉 unit",
                    ));
                }
                match parents.get(source_key.as_str()) {
                    None => issues.push(issue(
                        AgentEditErrorCode::InvalidBinding,
                        path,
                        format!("父步骤块不存在参数槽位：{source_key}"),
                        "映射到本块已声明的槽位",
                    )),
                    Some(parent) => {
                        if parent.value_type != child.value_type || parent.unit != child.unit {
                            issues.push(issue_affecting(
                                AgentEditErrorCode::InvalidBinding,
                                path,
                                format!(
                                    "父槽位 {source_key} 与子槽位 {} 的类型或单位不一致；不会自动换算",
                                    binding.slot
                                ),
                                "改用类型与单位一致的父槽位，或直接给值",
                                vec![format!("params.{source_key}")],
                            ));
                        }
                    }
                }
            }
            _ => issues.push(issue(
                AgentEditErrorCode::InvalidBinding,
                path,
                format!(
                    "参数 {} 必须且只能设置 value 或 source_slot 之一",
                    binding.slot
                ),
                "给出 value 或 source_slot，不要都填或都不填",
            )),
        }
    }
    if let Some(missing) = child_slots
        .iter()
        .find(|slot| slot.required && !mapped.contains(slot.key.as_str()))
    {
        issues.push(issue(
            AgentEditErrorCode::InvalidBinding,
            path,
            format!("子块必填参数 {} 尚未绑定", missing.key),
            "映射到父槽位或明确给值",
        ));
    }
    issues
}

fn block_use_bindings(step: &ProtocolStep) -> Option<(&str, &str, Vec<StepBinding>)> {
    match step {
        ProtocolStep::BlockUse {
            use_id,
            block_version_id,
            bindings,
        } => Some((use_id, block_version_id, bindings.clone())),
        ProtocolStep::Text { .. } => None,
    }
}

fn step_use_id(step: &ProtocolStep) -> &str {
    match step {
        ProtocolStep::Text { use_id, .. } | ProtocolStep::BlockUse { use_id, .. } => use_id,
    }
}

// ---------------------------------------------------------------------------
// Op application
// ---------------------------------------------------------------------------

fn apply_ops(
    store: &dyn StepEditStore,
    arena: &mut Arena,
    root: usize,
    ops: &[StepEditOp],
) -> Result<(Vec<ResolvedStepOp>, Vec<AgentEditIssue>), LoadError> {
    let mut resolved = Vec::new();
    let mut issues = Vec::new();
    for op in ops {
        match apply_one(store, arena, root, op) {
            Ok(entry) => resolved.push(entry),
            Err(LoadError::Issue(found)) => issues.push(*found),
            Err(store_error) => return Err(store_error),
        }
    }
    Ok((resolved, issues))
}

fn apply_one(
    store: &dyn StepEditStore,
    arena: &mut Arena,
    root: usize,
    op: &StepEditOp,
) -> Result<ResolvedStepOp, LoadError> {
    match op {
        StepEditOp::Insert {
            path,
            position,
            step,
        } => {
            let target = resolve_path(arena, root, path).map_err(issue_error)?;
            let container_slots = arena.blocks[target.container].data.parameter_slots.clone();
            let container_steps = arena.blocks[target.container].steps.clone();
            let mut issues = validate_text_step(step, &container_steps, &container_slots, path);
            if *position > container_steps.len() {
                issues.push(issue(
                    AgentEditErrorCode::InvalidProposal,
                    "position",
                    format!(
                        "position {position} 超出 {} 的步骤数 {}",
                        path,
                        container_steps.len()
                    ),
                    format!("position 取 0..={}", container_steps.len()),
                ));
            }
            if container_steps.len() >= MAX_STEPS {
                issues.push(issue(
                    AgentEditErrorCode::InvalidProposal,
                    "position",
                    format!("{path} 已达到 {MAX_STEPS} 个步骤上限"),
                    "拆分成多个步骤块",
                ));
            }
            if let Some(first) = issues.into_iter().next() {
                return Err(LoadError::Issue(Box::new(first)));
            }
            arena.blocks[target.container]
                .steps
                .insert(*position, step.clone());
            Ok(ResolvedStepOp {
                op: StepEditOpKind::Insert,
                path: path.clone(),
                changed: true,
                description: format!(
                    "在 {path} 的位置 {} 插入步骤 {}",
                    position,
                    step_use_id(step)
                ),
            })
        }
        StepEditOp::Reorder { path, order } => {
            let target = resolve_path(arena, root, path).map_err(issue_error)?;
            let current = arena.blocks[target.container].steps.clone();
            let current_ids = current.iter().map(step_use_id).collect::<Vec<_>>();
            let mut sorted_order = order.clone();
            let mut sorted_current = current_ids
                .iter()
                .map(|id| (*id).to_owned())
                .collect::<Vec<_>>();
            sorted_order.sort();
            sorted_current.sort();
            if sorted_order != sorted_current {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::InvalidProposal,
                    "order",
                    format!(
                        "order 必须是 {path} 当前 use_id 的完整排列：{:?}",
                        current_ids
                    ),
                    "按 protocol.block.preview 的实际步骤集合重写 order",
                ))));
            }
            let mut steps = Vec::with_capacity(current.len());
            for use_id in order {
                let index = find_step(&current, use_id).expect("permutation was checked");
                steps.push(current[index].clone());
            }
            arena.blocks[target.container].steps = steps;
            Ok(ResolvedStepOp {
                op: StepEditOpKind::Reorder,
                path: path.clone(),
                changed: true,
                description: format!("把 {path} 重排为 {:?}", order),
            })
        }
        StepEditOp::Reuse {
            path,
            position,
            use_id,
            block_version_id,
            bindings,
        } => {
            let target = resolve_path(arena, root, path).map_err(issue_error)?;
            let container_steps = arena.blocks[target.container].steps.clone();
            let container_slots = arena.blocks[target.container].data.parameter_slots.clone();
            if *position > container_steps.len() {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::InvalidProposal,
                    "position",
                    format!("position {position} 超出 {path} 的步骤数 {}", container_steps.len()),
                    format!("position 取 0..={}", container_steps.len()),
                ))));
            }
            if container_steps.len() >= MAX_STEPS {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::InvalidProposal,
                    "position",
                    format!("{path} 已达到 {MAX_STEPS} 个步骤上限"),
                    "拆分成多个步骤块",
                ))));
            }
            if let Some(first) = validate_use_id(use_id, &container_steps, path)
                .into_iter()
                .next()
            {
                return Err(LoadError::Issue(Box::new(first)));
            }
            let mut active = Vec::new();
            let child = load_block(
                store,
                arena,
                block_version_id,
                &format!("{path}/{use_id}"),
                &mut active,
            )?;
            let mut subtree = BTreeSet::new();
            collect_subtree_ids(arena, child, &mut subtree);
            if target
                .chain
                .iter()
                .any(|block_id| subtree.contains(block_id))
            {
                return Err(LoadError::Issue(Box::new(issue_affecting(
                    AgentEditErrorCode::InvalidProposal,
                    "block_version_id",
                    format!("复用 {block_version_id} 会形成循环引用"),
                    "复用一个不包含当前祖先块的版本",
                    vec![path.clone()],
                ))));
            }
            let child_slots = arena.blocks[child].data.parameter_slots.clone();
            if child_slots.len() + container_slots.len() > 200 {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::InvalidProposal,
                    path,
                    "组合后的参数槽位过多",
                    "减少嵌套或拆分步骤块",
                ))));
            }
            if let Some(first) =
                validate_bindings(path, &container_slots, &child_slots, bindings)
                    .into_iter()
                    .next()
            {
                return Err(LoadError::Issue(Box::new(first)));
            }
            arena.blocks[target.container].steps.insert(
                *position,
                ProtocolStep::BlockUse {
                    use_id: use_id.clone(),
                    block_version_id: block_version_id.clone(),
                    bindings: bindings.clone(),
                },
            );
            Ok(ResolvedStepOp {
                op: StepEditOpKind::Reuse,
                path: path.clone(),
                changed: true,
                description: format!(
                    "在 {path} 的位置 {} 复用块版本 {block_version_id} 作为 {use_id}",
                    position
                ),
            })
        }
        StepEditOp::Bind { path, bindings } => {
            let target = resolve_path(arena, root, path).map_err(issue_error)?;
            let Some((parent, index)) = target.step else {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::InvalidBinding,
                    "path",
                    format!("bind 需要指向一个块引用步骤，收到容器 {path}"),
                    "把 path 指到 root/.../<use_id> 形式的块引用步骤",
                ))));
            };
            let current = arena.blocks[parent].steps[index].clone();
            let Some((use_id, block_version_id, existing)) = block_use_bindings(&current) else {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::InvalidBinding,
                    "path",
                    format!("{path} 是文字步骤，没有可绑定的参数槽位"),
                    "只对块引用步骤绑定参数",
                ))));
            };
            let Some(child) = arena.by_version.get(block_version_id).copied() else {
                return Err(LoadError::Issue(Box::new(issue(
                    AgentEditErrorCode::StaleReference,
                    "path",
                    format!("子块版本 {block_version_id} 未加载；组合链已损坏"),
                    "重新读取 protocol.block.get",
                ))));
            };
            let child_slots = arena.blocks[child].data.parameter_slots.clone();
            let parent_slots = arena.blocks[parent].data.parameter_slots.clone();
            let mut merged = existing;
            for binding in bindings {
                match merged.iter_mut().find(|entry| entry.slot == binding.slot) {
                    Some(entry) => *entry = binding.clone(),
                    None => merged.push(binding.clone()),
                }
            }
            if let Some(first) =
                validate_bindings(path, &parent_slots, &child_slots, &merged)
                    .into_iter()
                    .next()
            {
                return Err(LoadError::Issue(Box::new(first)));
            }
            let changed = merged != block_use_bindings(&current).map(|(_, _, b)| b).unwrap_or_default();
            arena.blocks[parent].steps[index] = ProtocolStep::BlockUse {
                use_id: use_id.to_owned(),
                block_version_id: block_version_id.to_owned(),
                bindings: merged,
            };
            Ok(ResolvedStepOp {
                op: StepEditOpKind::Bind,
                path: path.clone(),
                changed,
                description: format!("更新 {path} 的参数绑定"),
            })
        }
    }
}

fn issue_error(found: AgentEditIssue) -> LoadError {
    LoadError::Issue(Box::new(found))
}

// ---------------------------------------------------------------------------
// Planning
// ---------------------------------------------------------------------------

struct PlannedBlock {
    block_id: String,
    based_on_version_id: String,
    previous_version: i64,
    new_version: i64,
    /// Steps before parent references are rewritten to the new child versions;
    /// the ids only exist after the child is appended in the same commit.
    steps: Vec<ProtocolStep>,
    data: ProtocolBlockVersionData,
}

impl PlannedBlock {
    fn view(&self) -> PlannedBlockView {
        PlannedBlockView {
            block_id: self.block_id.clone(),
            based_on_version_id: self.based_on_version_id.clone(),
            previous_version: self.previous_version,
            new_version: self.new_version,
            order: self.steps.iter().map(step_use_id).map(str::to_owned).collect(),
        }
    }
}

fn post_order(arena: &Arena, index: usize, visited: &mut HashSet<usize>, order: &mut Vec<usize>) {
    if !visited.insert(index) {
        return;
    }
    for step in &arena.blocks[index].steps {
        if let ProtocolStep::BlockUse {
            block_version_id, ..
        } = step
        {
            if let Some(child) = arena.by_version.get(block_version_id) {
                post_order(arena, *child, visited, order);
            }
        }
    }
    order.push(index);
}

fn plan_blocks(
    arena: &Arena,
    root: usize,
    store: &dyn StepEditStore,
) -> Result<Vec<PlannedBlock>, LoadError> {
    let mut order = Vec::new();
    post_order(arena, root, &mut HashSet::new(), &mut order);
    let mut planned = Vec::new();
    let mut by_block: BTreeSet<String> = BTreeSet::new();
    // Base version ids that this edit replaces. A parent whose steps reference
    // one of them must move too, so its new version points at the new child.
    let mut replaced: BTreeSet<String> = BTreeSet::new();
    for index in order {
        let block = &arena.blocks[index];
        let child_moved = block.steps.iter().any(|step| match step {
            ProtocolStep::BlockUse {
                block_version_id, ..
            } => replaced.contains(block_version_id),
            ProtocolStep::Text { .. } => false,
        });
        if block.steps == block.original_steps && !child_moved {
            continue;
        }
        if !by_block.insert(block.block_id.clone()) {
            return Err(LoadError::Issue(Box::new(issue_affecting(
                AgentEditErrorCode::InvalidProposal,
                format!("blocks.{}", block.block_id),
                format!(
                    "步骤块 {} 在一次编辑中出现多个被改动的版本；无法同时生成两个新版本",
                    block.block_id
                ),
                "把这两个位置的编辑拆成两次提交",
                vec![block.block_id.clone()],
            ))));
        }
        let head = store
            .step_block_head(&block.block_id)
            .map_err(LoadError::Store)?
            .ok_or_else(|| {
                load_error(
                    AgentEditErrorCode::StaleReference,
                    format!("blocks.{}", block.block_id),
                    format!("步骤块 {} 不存在", block.block_id),
                    "重新读取 protocol.block.list",
                )
            })?;
        if head.version_id != block.version_id {
            return Err(LoadError::Issue(Box::new(issue_affecting(
                AgentEditErrorCode::VersionConflict,
                format!("blocks.{}", block.block_id),
                format!(
                    "步骤块 {} 的 head 是 {}（v{}），本次编辑基于 {}（v{}）",
                    block.block_id, head.version_id, head.revision, block.version_id, block.revision
                ),
                "重新读取该步骤块后重新组稿；不合并、不覆盖",
                vec![block.block_id.clone()],
            ))));
        }
        let mut data = block.data.clone();
        data.version = head.revision + 1;
        data.steps = block.steps.clone();
        data.based_on_version_id = Some(block.version_id.clone());
        if data.module_contract.is_some() {
            data.module_origin = Some(ModuleOrigin::Derived);
            data.module_contract_authority = Some(ModuleContractAuthority::SourceOnly);
        }
        data.step_edit = None;
        replaced.insert(block.version_id.clone());
        planned.push(PlannedBlock {
            block_id: block.block_id.clone(),
            based_on_version_id: block.version_id.clone(),
            previous_version: head.revision,
            new_version: head.revision + 1,
            steps: block.steps.clone(),
            data,
        });
    }
    Ok(planned)
}

fn rewrite_refs(
    steps: &[ProtocolStep],
    mapping: &BTreeMap<String, String>,
) -> Vec<ProtocolStep> {
    steps
        .iter()
        .map(|step| match step {
            ProtocolStep::BlockUse {
                use_id,
                block_version_id,
                bindings,
            } => ProtocolStep::BlockUse {
                use_id: use_id.clone(),
                block_version_id: mapping
                    .get(block_version_id)
                    .cloned()
                    .unwrap_or_else(|| block_version_id.clone()),
                bindings: bindings.clone(),
            },
            other => other.clone(),
        })
        .collect()
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

struct Resolution {
    result: StepEditResult,
    plan: Vec<PlannedBlock>,
}

/// Read-only validate stage. `Err` is a store failure only; every semantic
/// rejection is `status = blocked` with `issues`.
pub fn validate(request: &StepEditRequest, store: &dyn StepEditStore) -> Result<StepEditResult, String> {
    Ok(resolve(request, store)?.result)
}

fn resolve(request: &StepEditRequest, store: &dyn StepEditStore) -> Result<Resolution, String> {
    let digest = edit_digest(request);
    let mut issues = Vec::new();
    if request.schema_version != STEP_EDIT_SCHEMA_VERSION {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "schema_version",
            format!(
                "只接受 schema_version {STEP_EDIT_SCHEMA_VERSION}，收到 {}",
                request.schema_version
            ),
            "按 step edit v1 组稿",
        ));
    }
    if request.edit_id.trim().is_empty() {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "edit_id",
            "edit_id 不能为空",
            "给本次编辑一个稳定标识",
        ));
    }
    if request.ops.is_empty() {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "ops",
            "ops 不能为空",
            "至少给出一个 insert/reorder/reuse/bind 操作",
        ));
    }
    if request.ops.len() > MAX_OPS {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "ops",
            format!("单次编辑最多 {MAX_OPS} 个操作"),
            "拆成多次提交",
        ));
    }
    if request.expected_version < 1 {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "expected_version",
            "expected_version 必须是读取时的块版本号（>= 1）",
            "从 protocol.block.get 读取当前版本号",
        ));
    }
    if !issues.is_empty() {
        return Ok(blocked_resolution(request, digest, "", 0, issues));
    }

    let Some(root_block) = store.step_block_version(&request.block_version_id)? else {
        return Ok(blocked_resolution(
            request,
            digest,
            "",
            0,
            vec![issue(
                AgentEditErrorCode::StaleReference,
                "block_version_id",
                format!("步骤块版本 {} 不存在", request.block_version_id),
                "重新读取 protocol.block.get 后重新组稿",
            )],
        ));
    };
    let block_id = root_block.block_id.clone();
    let Some(head) = store.step_block_head(&block_id)? else {
        return Ok(blocked_resolution(
            request,
            digest,
            &block_id,
            0,
            vec![issue(
                AgentEditErrorCode::StaleReference,
                "block_version_id",
                format!("步骤块 {block_id} 没有可用版本"),
                "重新读取 protocol.block.list",
            )],
        ));
    };
    let mut issues = Vec::new();
    if head.version_id != request.block_version_id {
        issues.push(issue(
            AgentEditErrorCode::StaleReference,
            "block_version_id",
            format!(
                "block_version_id 指向 {}，当前 head 是 {}（v{}）",
                request.block_version_id, head.version_id, head.revision
            ),
            "从当前 head 重新组稿",
        ));
    }
    if head.revision != request.expected_version {
        issues.push(issue_affecting(
            AgentEditErrorCode::VersionConflict,
            "expected_version",
            format!(
                "expected_version {} 与当前版本 {} 不一致",
                request.expected_version, head.revision
            ),
            "重新读取后重新组稿；不合并、不覆盖",
            vec!["全部 op".to_owned()],
        ));
    }
    if !issues.is_empty() {
        return Ok(blocked_resolution(request, digest, &block_id, head.revision, issues));
    }

    let mut arena = Arena::default();
    let mut active = Vec::new();
    let root = match load_block(store, &mut arena, &request.block_version_id, "root", &mut active) {
        Ok(index) => index,
        Err(LoadError::Issue(found)) => {
            return Ok(blocked_resolution(
                request,
                digest,
                &block_id,
                head.revision,
                vec![*found],
            ))
        }
        Err(LoadError::Store(message)) => return Err(message),
    };

    let (resolved_ops, mut op_issues) = match apply_ops(store, &mut arena, root, &request.ops) {
        Ok(result) => result,
        Err(LoadError::Issue(found)) => {
            return Ok(blocked_resolution(
                request,
                digest,
                &block_id,
                head.revision,
                vec![*found],
            ))
        }
        Err(LoadError::Store(message)) => return Err(message),
    };

    if op_issues.is_empty() {
        match plan_blocks(&arena, root, store) {
            Ok(plan) if plan.is_empty() => op_issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                "ops",
                "这次编辑没有产生任何变更；不生成空版本",
                "确认 op 是否与当前结构一致",
            )),
            Ok(plan) => {
                let views = plan.iter().map(PlannedBlock::view).collect::<Vec<_>>();
                let invalidated = invalidated_refs(&plan);
                let prompt = confirmation_prompt(&resolved_ops, &views);
                return Ok(Resolution {
                    result: StepEditResult {
                        schema_version: STEP_EDIT_SCHEMA_VERSION,
                        edit_id: request.edit_id.clone(),
                        edit_digest: digest,
                        block_version_id: request.block_version_id.clone(),
                        block_id: block_id.clone(),
                        current_revision: head.revision,
                        status: ValidationStatus::NeedsConfirmation,
                        requires_confirmation: true,
                        confirmation_prompt: Some(prompt),
                        ops: resolved_ops,
                        blocks: views,
                        issues: Vec::new(),
                        invalidated_refs: invalidated,
                    },
                    plan,
                });
            }
            Err(LoadError::Issue(found)) => op_issues.push(*found),
            Err(LoadError::Store(message)) => return Err(message),
        }
    }

    Ok(Resolution {
        result: blocked(request, digest, &block_id, head.revision, op_issues)
            .with_ops_and_op_issues(resolved_ops),
        plan: Vec::new(),
    })
}

impl StepEditResult {
    /// Keep the resolved ops in a blocked answer: the Agent can see which ops
    /// resolved before the first failure, but no plan is shown.
    fn with_ops_and_op_issues(mut self, ops: Vec<ResolvedStepOp>) -> Self {
        if !ops.is_empty() {
            self.ops = ops;
        }
        self
    }
}

fn blocked_resolution(
    request: &StepEditRequest,
    digest: String,
    block_id: &str,
    current_revision: i64,
    issues: Vec<AgentEditIssue>,
) -> Resolution {
    Resolution {
        result: blocked(request, digest, block_id, current_revision, issues),
        plan: Vec::new(),
    }
}

fn blocked(
    request: &StepEditRequest,
    digest: String,
    block_id: &str,
    current_revision: i64,
    issues: Vec<AgentEditIssue>,
) -> StepEditResult {
    StepEditResult {
        schema_version: STEP_EDIT_SCHEMA_VERSION,
        edit_id: request.edit_id.clone(),
        edit_digest: digest,
        block_version_id: request.block_version_id.clone(),
        block_id: block_id.to_owned(),
        current_revision,
        status: ValidationStatus::Blocked,
        requires_confirmation: true,
        confirmation_prompt: None,
        ops: Vec::new(),
        blocks: Vec::new(),
        issues,
        invalidated_refs: Vec::new(),
    }
}

fn invalidated_refs(plan: &[PlannedBlock]) -> Vec<String> {
    let mut refs = plan
        .iter()
        .map(|block| format!("core:block:{}", block.based_on_version_id))
        .collect::<Vec<_>>();
    refs.sort();
    refs.dedup();
    refs
}

fn confirmation_prompt(ops: &[ResolvedStepOp], blocks: &[PlannedBlockView]) -> String {
    let mut counts = BTreeMap::new();
    for op in ops {
        *counts.entry(op.op.label()).or_insert(0usize) += 1;
    }
    let summary = counts
        .into_iter()
        .map(|(label, count)| format!("{label} {count}"))
        .collect::<Vec<_>>()
        .join("、");
    let targets = blocks
        .iter()
        .map(|block| format!("{} v{}→v{}", block.block_id, block.previous_version, block.new_version))
        .collect::<Vec<_>>()
        .join("、");
    format!(
        "将应用 {} 个 block 级操作（{summary}），生成 {} 个新步骤块版本：{targets}。冻结版本与已冻结 Run 不改写。",
        ops.len(),
        blocks.len()
    )
}

// ---------------------------------------------------------------------------
// Commit
// ---------------------------------------------------------------------------

fn rejection(
    code: AgentEditErrorCode,
    path: &str,
    message: String,
    current_version: Option<i64>,
    current_version_id: Option<String>,
) -> StepEditCommitError {
    StepEditCommitError::Rejected(AgentEditError {
        code,
        message,
        path: Some(path.to_owned()),
        current_version,
        current_version_id,
        issues: Vec::new(),
    })
}

/// Commit stage. Re-reads the head inside the caller's transaction, appends
/// one new version per changed block (children first) and never mutates a
/// version that already exists.
pub fn commit(
    request: &StepEditRequest,
    store: &mut dyn StepEditCommitStore,
) -> Result<StepEditCommitResult, StepEditCommitError> {
    let Some(confirmation) = request.confirmation.clone() else {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationRequired,
            "confirmation",
            "commit 必须携带研究者确认；没有确认的写入不是人工确认".to_owned(),
            None,
            None,
        ));
    };
    if confirmation.confirmed_by.trim().is_empty() {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationRequired,
            "confirmation.confirmed_by",
            "confirmation.confirmed_by 不能为空".to_owned(),
            None,
            None,
        ));
    }
    if confirmation.idempotency_key.trim().is_empty() {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationRequired,
            "confirmation.idempotency_key",
            "commit 必须提供 idempotency_key".to_owned(),
            None,
            None,
        ));
    }
    let digest = edit_digest(request);
    if confirmation.edit_digest != digest {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationMismatch,
            "confirmation.edit_digest",
            format!(
                "确认的 edit_digest 与本次 ops 不符：确认 {}，实际 {}",
                confirmation.edit_digest, digest
            ),
            None,
            None,
        ));
    }

    let existing = store
        .find_step_commit(&confirmation.idempotency_key)
        .map_err(StepEditCommitError::Store)?;
    if !existing.is_empty() {
        return replay(request, &confirmation, &digest, &existing);
    }

    let resolution = resolve(request, store.step_store()).map_err(StepEditCommitError::Store)?;
    if let Some(first) = resolution
        .result
        .issues
        .iter()
        .find(|issue| issue.severity == IssueSeverity::Blocking)
    {
        return Err(StepEditCommitError::Rejected(AgentEditError {
            code: first.code,
            message: first.message.clone(),
            path: Some(first.path.clone()),
            current_version: Some(resolution.result.current_revision),
            current_version_id: Some(resolution.result.block_version_id.clone()),
            issues: resolution.result.issues.clone(),
        }));
    }
    if resolution.plan.is_empty() {
        return Err(rejection(
            AgentEditErrorCode::InvalidProposal,
            "ops",
            "这次编辑没有产生任何变更；不生成空版本".to_owned(),
            Some(resolution.result.current_revision),
            Some(request.block_version_id.clone()),
        ));
    }

    let committed_at = store.timestamp();
    let invalidated = invalidated_refs(&resolution.plan);
    let mut mapping: BTreeMap<String, String> = BTreeMap::new();
    let mut created = Vec::with_capacity(resolution.plan.len());
    for planned in &resolution.plan {
        let steps = rewrite_refs(&planned.steps, &mapping);
        let mut data = planned.data.clone();
        data.steps = steps;
        data.step_edit = Some(StepEditProvenance {
            edit_id: request.edit_id.clone(),
            edit_digest: digest.clone(),
            idempotency_key: confirmation.idempotency_key.clone(),
            confirmed_by: confirmation.confirmed_by.clone(),
            committed_at: committed_at.clone(),
            block_id: planned.block_id.clone(),
            based_on_version_id: planned.based_on_version_id.clone(),
            invalidated_refs: invalidated.clone(),
        });
        let payload = serde_json::to_value(&data).map_err(|error| {
            StepEditCommitError::Store(format!("新步骤块版本序列化失败：{error}"))
        })?;
        let (version_id, revision) = store
            .append_block_version(&planned.block_id, planned.new_version, payload)
            .map_err(StepEditCommitError::Store)?;
        if version_id.is_empty() || revision != planned.new_version {
            return Err(StepEditCommitError::Store(format!(
                "步骤块 {} 的追加结果与计划不符（期望 v{}，得到 {} v{}）",
                planned.block_id, planned.new_version, version_id, revision
            )));
        }
        mapping.insert(planned.based_on_version_id.clone(), version_id.clone());
        created.push(CreatedBlockVersion {
            block_id: planned.block_id.clone(),
            based_on_version_id: planned.based_on_version_id.clone(),
            version_id,
            version: revision,
        });
    }
    let root = created
        .iter()
        .find(|entry| entry.based_on_version_id == request.block_version_id)
        .ok_or_else(|| {
            StepEditCommitError::Store("计划缺少根步骤块的新版本".to_owned())
        })?
        .clone();

    Ok(StepEditCommitResult {
        schema_version: STEP_EDIT_SCHEMA_VERSION,
        edit_id: request.edit_id.clone(),
        edit_digest: digest,
        block_version_id: request.block_version_id.clone(),
        block_id: resolution.result.block_id.clone(),
        previous_version_id: request.block_version_id.clone(),
        new_root_block_version_id: root.version_id,
        new_revision: root.version,
        created_versions: created,
        invalidated_refs: invalidated,
        confirmed_by: confirmation.confirmed_by.clone(),
        committed_at,
        idempotency_key: confirmation.idempotency_key.clone(),
        replayed: false,
    })
}

fn replay(
    request: &StepEditRequest,
    confirmation: &StepEditConfirmation,
    digest: &str,
    existing: &[CommittedStepEdit],
) -> Result<StepEditCommitResult, StepEditCommitError> {
    let mut created = Vec::with_capacity(existing.len());
    let mut invalidated: Vec<String> = Vec::new();
    let mut committed_at = String::new();
    let mut confirmed_by = String::new();
    let mut root: Option<CommittedStepEdit> = None;
    for entry in existing {
        let provenance: StepEditProvenance =
            serde_json::from_value(entry.provenance.clone()).map_err(|error| {
                StepEditCommitError::Store(format!(
                    "步骤块版本 {} 的 step_edit 记录无效：{error}",
                    entry.version_id
                ))
            })?;
        if provenance.edit_digest != digest {
            return Err(rejection(
                AgentEditErrorCode::IdempotencyMismatch,
                "confirmation.idempotency_key",
                format!(
                    "idempotency_key {} 已用于另一个 edit_digest",
                    confirmation.idempotency_key
                ),
                None,
                None,
            ));
        }
        if committed_at.is_empty() {
            committed_at = provenance.committed_at.clone();
            confirmed_by = provenance.confirmed_by.clone();
            invalidated = provenance.invalidated_refs.clone();
        }
        if provenance.based_on_version_id == request.block_version_id {
            root = Some(entry.clone());
        }
        created.push(CreatedBlockVersion {
            block_id: provenance.block_id,
            based_on_version_id: provenance.based_on_version_id,
            version_id: entry.version_id.clone(),
            version: entry.revision,
        });
    }
    let root = root.ok_or_else(|| {
        StepEditCommitError::Store(
            "幂等重放找不到本次编辑的根步骤块新版本".to_owned(),
        )
    })?;
    created.sort_by_key(|entry| entry.version);
    Ok(StepEditCommitResult {
        schema_version: STEP_EDIT_SCHEMA_VERSION,
        edit_id: request.edit_id.clone(),
        edit_digest: digest.to_owned(),
        block_version_id: request.block_version_id.clone(),
        block_id: created
            .iter()
            .find(|entry| entry.based_on_version_id == request.block_version_id)
            .map(|entry| entry.block_id.clone())
            .unwrap_or_default(),
        previous_version_id: request.block_version_id.clone(),
        new_root_block_version_id: root.version_id,
        new_revision: root.revision,
        created_versions: created,
        invalidated_refs: invalidated,
        confirmed_by,
        committed_at,
        idempotency_key: confirmation.idempotency_key.clone(),
        replayed: true,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::agent_edit::EditOrigin;

    fn number_slot(key: &str) -> ParameterSlot {
        ParameterSlot {
            key: key.to_owned(),
            label: key.to_owned(),
            value_type: ParameterValueType::Number,
            unit: Some("uL".to_owned()),
            required: true,
        }
    }

    fn text(use_id: &str) -> ProtocolStep {
        ProtocolStep::Text {
            use_id: use_id.to_owned(),
            title: use_id.to_owned(),
            instruction: format!("do {use_id}"),
            parameter_refs: Vec::new(),
            visualization: None,
            time_scope: None,
            checks: Vec::new(),
        }
    }

    fn block_use(use_id: &str, version: &str, bindings: Vec<StepBinding>) -> ProtocolStep {
        ProtocolStep::BlockUse {
            use_id: use_id.to_owned(),
            block_version_id: version.to_owned(),
            bindings,
        }
    }

    fn version_data(
        block_id: &str,
        version: i64,
        slots: Vec<ParameterSlot>,
        steps: Vec<ProtocolStep>,
    ) -> ProtocolBlockVersionData {
        ProtocolBlockVersionData {
            block_id: block_id.to_owned(),
            version,
            summary: format!("{block_id} summary"),
            parameter_slots: slots,
            steps,
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
            based_on_version_id: None,
            step_edit: None,
        }
    }

    fn stored(version_id: &str, data: ProtocolBlockVersionData) -> StoredEditBlock {
        StoredEditBlock {
            version_id: version_id.to_owned(),
            block_id: data.block_id.clone(),
            revision: data.version,
            project_id: Some("proj".to_owned()),
            data,
        }
    }

    #[derive(Default)]
    struct MemoryStore {
        blocks: Vec<StoredEditBlock>,
        appended: Vec<(String, i64, Value)>,
    }

    impl MemoryStore {
        fn fixture() -> Self {
            let wash = stored(
                "wash-v1",
                version_data("b-wash", 1, Vec::new(), vec![text("rinse")]),
            );
            let lyse = stored(
                "lyse-v1",
                version_data(
                    "b-lyse",
                    1,
                    vec![number_slot("volume_per_sample")],
                    vec![text("iris")],
                ),
            );
            let root = stored(
                "root-v1",
                version_data(
                    "b-root",
                    1,
                    vec![number_slot("volume_per_sample")],
                    vec![
                        text("prep"),
                        block_use(
                            "lyse",
                            "lyse-v1",
                            vec![StepBinding {
                                slot: "volume_per_sample".to_owned(),
                                value: Some(serde_json::json!(200.0)),
                                source_slot: None,
                                unit: Some("uL".to_owned()),
                            }],
                        ),
                    ],
                ),
            );
            Self {
                blocks: vec![wash, lyse, root],
                appended: Vec::new(),
            }
        }

        fn version(&self, version_id: &str) -> StoredEditBlock {
            self.blocks
                .iter()
                .find(|block| block.version_id == version_id)
                .cloned()
                .unwrap_or_else(|| panic!("missing {version_id}"))
        }

        fn head_version(&self, block_id: &str) -> String {
            self.blocks
                .iter()
                .filter(|block| block.block_id == block_id)
                .max_by_key(|block| block.revision)
                .map(|block| block.version_id.clone())
                .unwrap_or_default()
        }
    }

    impl StepEditStore for MemoryStore {
        fn step_block_version(&self, version_id: &str) -> Result<Option<StoredEditBlock>, String> {
            Ok(self
                .blocks
                .iter()
                .find(|block| block.version_id == version_id)
                .cloned())
        }

        fn step_block_head(&self, block_id: &str) -> Result<Option<StoredBlockHead>, String> {
            Ok(self
                .blocks
                .iter()
                .filter(|block| block.block_id == block_id)
                .max_by_key(|block| block.revision)
                .map(|block| StoredBlockHead {
                    block_id: block.block_id.clone(),
                    version_id: block.version_id.clone(),
                    revision: block.revision,
                }))
        }
    }

    impl StepEditCommitStore for MemoryStore {
        fn step_store(&self) -> &dyn StepEditStore {
            self
        }

        fn find_step_commit(
            &self,
            idempotency_key: &str,
        ) -> Result<Vec<CommittedStepEdit>, String> {
            Ok(self
                .blocks
                .iter()
                .filter(|block| {
                    block.data.step_edit.as_ref().map(|edit| edit.idempotency_key.as_str())
                        == Some(idempotency_key)
                })
                .map(|block| CommittedStepEdit {
                    version_id: block.version_id.clone(),
                    revision: block.revision,
                    provenance: serde_json::to_value(&block.data.step_edit).unwrap(),
                })
                .collect())
        }

        fn timestamp(&self) -> String {
            "2026-11-13T00:00:00Z".to_owned()
        }

        fn append_block_version(
            &mut self,
            block_id: &str,
            revision: i64,
            data: Value,
        ) -> Result<(String, i64), String> {
            let parsed: ProtocolBlockVersionData = serde_json::from_value(data.clone())
                .map_err(|error| error.to_string())?;
            let version_id = format!("new-{block_id}-v{revision}");
            self.appended.push((block_id.to_owned(), revision, data));
            self.blocks.push(StoredEditBlock {
                version_id: version_id.clone(),
                block_id: block_id.to_owned(),
                revision,
                project_id: Some("proj".to_owned()),
                data: parsed,
            });
            Ok((version_id, revision))
        }
    }

    fn request(ops: Vec<StepEditOp>) -> StepEditRequest {
        StepEditRequest {
            schema_version: STEP_EDIT_SCHEMA_VERSION,
            edit_id: "se-1".to_owned(),
            block_version_id: "root-v1".to_owned(),
            expected_version: 1,
            origin: ProposalOrigin {
                source: EditOrigin::Agent,
                rationale: Some("structure change".to_owned()),
            },
            ops,
            confirmation: None,
        }
    }

    fn confirm(request: &StepEditRequest, digest: &str, key: &str) -> StepEditRequest {
        let mut confirmed = request.clone();
        confirmed.confirmation = Some(StepEditConfirmation {
            edit_digest: digest.to_owned(),
            confirmed_by: "researcher:zhang".to_owned(),
            idempotency_key: key.to_owned(),
            note: None,
        });
        confirmed
    }

    #[test]
    fn insert_adds_a_text_step_at_the_requested_position() {
        let store = MemoryStore::fixture();
        let request = request(vec![StepEditOp::Insert {
            path: "root".to_owned(),
            position: 1,
            step: text("incubate"),
        }]);
        let result = validate(&request, &store).expect("validate");
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);
        assert!(result.requires_confirmation);
        assert!(result.edit_digest.starts_with("sha256:"));
        assert!(result.issues.is_empty(), "{:?}", result.issues);
        assert_eq!(result.blocks.len(), 1);
        assert_eq!(result.blocks[0].block_id, "b-root");
        assert_eq!(result.blocks[0].based_on_version_id, "root-v1");
        assert_eq!(result.blocks[0].previous_version, 1);
        assert_eq!(result.blocks[0].new_version, 2);
        assert_eq!(result.blocks[0].order, ["prep", "incubate", "lyse"]);
        assert_eq!(result.invalidated_refs, ["core:block:root-v1"]);
        assert!(result.confirmation_prompt.is_some());
        // Only the edited block gets a version; its child keeps its version.
        assert_eq!(store.blocks.len(), 3);
        assert_eq!(store.head_version("b-root"), "root-v1");
    }

    #[test]
    fn deep_reuse_appends_the_child_before_its_parent() {
        let mut store = MemoryStore::fixture();
        let request = request(vec![StepEditOp::Reuse {
            path: "root/lyse".to_owned(),
            position: 1,
            use_id: "wash".to_owned(),
            block_version_id: "wash-v1".to_owned(),
            bindings: Vec::new(),
        }]);
        let preview = validate(&request, &store).expect("validate");
        assert_eq!(preview.status, ValidationStatus::NeedsConfirmation);
        let planned: Vec<&str> = preview
            .blocks
            .iter()
            .map(|block| block.block_id.as_str())
            .collect();
        assert_eq!(planned, ["b-lyse", "b-root"], "child version must come first");
        assert_eq!(preview.blocks[0].order, ["iris", "wash"]);
        assert_eq!(
            preview.invalidated_refs,
            ["core:block:lyse-v1", "core:block:root-v1"]
        );

        let digest = preview.edit_digest.clone();
        let committed = commit(&confirm(&request, &digest, "se-1-confirm"), &mut store)
            .expect("commit");
        assert_eq!(committed.created_versions.len(), 2);
        assert_eq!(committed.created_versions[0].block_id, "b-lyse");
        assert_eq!(committed.created_versions[0].based_on_version_id, "lyse-v1");
        assert_eq!(committed.created_versions[1].block_id, "b-root");
        assert_eq!(committed.new_revision, 2);
        assert_eq!(
            committed.new_root_block_version_id,
            committed.created_versions[1].version_id
        );
        assert_eq!(committed.confirmed_by, "researcher:zhang");
        assert!(!committed.replayed);

        // The parent's block use points at the new child version; the base
        // versions are untouched.
        let new_root = store.version(&committed.new_root_block_version_id);
        assert_eq!(new_root.data.based_on_version_id.as_deref(), Some("root-v1"));
        let ProtocolStep::BlockUse {
            block_version_id, ..
        } = &new_root.data.steps[1]
        else {
            panic!("expected the block use step at index 1");
        };
        assert_eq!(
            block_version_id,
            &committed.created_versions[0].version_id,
            "parent must reference the child version created in the same commit"
        );
        let base_root = store.version("root-v1");
        assert_eq!(base_root.data.steps, MemoryStore::fixture().version("root-v1").data.steps);
        assert!(base_root.data.step_edit.is_none());
    }

    #[test]
    fn reorder_requires_an_exact_permutation() {
        let store = MemoryStore::fixture();
        let bad = request(vec![StepEditOp::Reorder {
            path: "root".to_owned(),
            order: vec!["lyse".to_owned()],
        }]);
        let blocked = validate(&bad, &store).expect("validate");
        assert_eq!(blocked.status, ValidationStatus::Blocked);
        assert!(blocked.blocks.is_empty());
        let codes: Vec<String> = blocked
            .issues
            .iter()
            .map(|issue| serde_json::to_value(issue.code).unwrap().as_str().unwrap().to_owned())
            .collect();
        assert_eq!(codes, ["INVALID_PROPOSAL"]);
        assert_eq!(blocked.issues[0].path, "order");

        let good = request(vec![StepEditOp::Reorder {
            path: "root".to_owned(),
            order: vec!["lyse".to_owned(), "prep".to_owned()],
        }]);
        let result = validate(&good, &store).expect("validate");
        assert_eq!(result.blocks[0].order, ["lyse", "prep"]);
    }

    #[test]
    fn bind_rejects_a_wrong_unit_then_accepts_the_slot_unit() {
        let store = MemoryStore::fixture();
        let wrong = request(vec![StepEditOp::Bind {
            path: "root/lyse".to_owned(),
            bindings: vec![StepBinding {
                slot: "volume_per_sample".to_owned(),
                value: Some(serde_json::json!(0.2)),
                source_slot: None,
                unit: Some("mL".to_owned()),
            }],
        }]);
        let blocked = validate(&wrong, &store).expect("validate");
        assert_eq!(blocked.status, ValidationStatus::Blocked);
        assert_eq!(
            serde_json::to_value(blocked.issues[0].code).unwrap(),
            "UNIT_MISMATCH"
        );
        assert_eq!(blocked.issues[0].path, "root/lyse");

        let right = request(vec![StepEditOp::Bind {
            path: "root/lyse".to_owned(),
            bindings: vec![StepBinding {
                slot: "volume_per_sample".to_owned(),
                value: Some(serde_json::json!(0.2)),
                source_slot: None,
                unit: Some("uL".to_owned()),
            }],
        }]);
        let result = validate(&right, &store).expect("validate");
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);
        // A binding lives on the parent's block use, so only the root moves.
        assert_eq!(result.blocks.len(), 1);
        assert_eq!(result.blocks[0].block_id, "b-root");
    }

    #[test]
    fn paths_outside_the_root_tree_are_forbidden() {
        let store = MemoryStore::fixture();
        for path in ["", "other/lyse", "root/../lyse", "root//lyse"] {
            let request = request(vec![StepEditOp::Bind {
                path: path.to_owned(),
                bindings: Vec::new(),
            }]);
            let result = validate(&request, &store).expect("validate");
            assert_eq!(result.status, ValidationStatus::Blocked, "{path}");
            assert_eq!(
                serde_json::to_value(result.issues[0].code).unwrap(),
                "FORBIDDEN_FIELD",
                "{path}"
            );
        }
        let missing = request(vec![StepEditOp::Bind {
            path: "root/missing".to_owned(),
            bindings: Vec::new(),
        }]);
        let result = validate(&missing, &store).expect("validate");
        assert_eq!(result.status, ValidationStatus::Blocked);
        assert_eq!(
            serde_json::to_value(result.issues[0].code).unwrap(),
            "STALE_REFERENCE"
        );
        assert!(store.appended.is_empty());
    }

    #[test]
    fn stale_expected_version_and_stale_base_are_rejected() {
        let mut store = MemoryStore::fixture();
        let mut stale = request(vec![StepEditOp::Insert {
            path: "root".to_owned(),
            position: 0,
            step: text("first"),
        }]);
        stale.expected_version = 2;
        let blocked = validate(&stale, &store).expect("validate");
        assert_eq!(blocked.status, ValidationStatus::Blocked);
        assert_eq!(
            serde_json::to_value(blocked.issues[0].code).unwrap(),
            "VERSION_CONFLICT"
        );
        assert_eq!(blocked.issues[0].path, "expected_version");

        let request = request(vec![StepEditOp::Insert {
            path: "root".to_owned(),
            position: 0,
            step: text("first"),
        }]);
        let preview = validate(&request, &store).expect("validate");
        let digest = preview.edit_digest.clone();
        commit(&confirm(&request, &digest, "se-1-confirm"), &mut store).expect("commit");

        // The same base version is no longer the head.
        let again = validate(&request, &store).expect("validate");
        assert_eq!(again.status, ValidationStatus::Blocked);
        assert_eq!(
            serde_json::to_value(again.issues[0].code).unwrap(),
            "STALE_REFERENCE"
        );
        assert_eq!(again.issues[0].path, "block_version_id");
    }

    #[test]
    fn commit_requires_a_matching_confirmation_digest() {
        let mut store = MemoryStore::fixture();
        let request = request(vec![StepEditOp::Insert {
            path: "root".to_owned(),
            position: 0,
            step: text("first"),
        }]);
        let preview = validate(&request, &store).expect("validate");
        let missing = commit(&request, &mut store).expect_err("confirmation is required");
        assert!(matches!(
            missing,
            StepEditCommitError::Rejected(AgentEditError {
                code: AgentEditErrorCode::ConfirmationRequired,
                ..
            })
        ));
        let mismatch = confirm(&request, "sha256:deadbeef", "se-1-confirm");
        let rejected = commit(&mismatch, &mut store).expect_err("digest mismatch");
        assert!(matches!(
            rejected,
            StepEditCommitError::Rejected(AgentEditError {
                code: AgentEditErrorCode::ConfirmationMismatch,
                ..
            })
        ));
        assert!(store.appended.is_empty());
        // The digest covers the ops, not the confirmation block.
        assert_eq!(edit_digest(&request), edit_digest(&confirm(&request, &preview.edit_digest, "k")));
    }

    #[test]
    fn commit_replays_an_idempotency_key_without_a_second_version() {
        let mut store = MemoryStore::fixture();
        let request = request(vec![StepEditOp::Insert {
            path: "root".to_owned(),
            position: 0,
            step: text("first"),
        }]);
        let preview = validate(&request, &store).expect("validate");
        let confirmed = confirm(&request, &preview.edit_digest, "se-1-confirm");
        let first = commit(&confirmed, &mut store).expect("commit");
        assert_eq!(store.appended.len(), 1);
        let replay = commit(&confirmed, &mut store).expect("replay");
        assert!(replay.replayed);
        assert_eq!(replay.new_root_block_version_id, first.new_root_block_version_id);
        assert_eq!(replay.new_revision, first.new_revision);
        assert_eq!(replay.created_versions, first.created_versions);
        assert_eq!(store.appended.len(), 1);

        let mut other = confirmed.clone();
        other.ops.push(StepEditOp::Insert {
            path: "root".to_owned(),
            position: 0,
            step: text("second"),
        });
        let mismatched = commit(&other, &mut store).expect_err("digest mismatch first");
        assert!(matches!(
            mismatched,
            StepEditCommitError::Rejected(AgentEditError {
                code: AgentEditErrorCode::ConfirmationMismatch,
                ..
            })
        ));
    }

    #[test]
    fn a_no_op_edit_is_rejected_and_reuse_cannot_form_a_cycle() {
        let store = MemoryStore::fixture();
        let no_op = request(vec![StepEditOp::Reorder {
            path: "root".to_owned(),
            order: vec!["prep".to_owned(), "lyse".to_owned()],
        }]);
        let blocked = validate(&no_op, &store).expect("validate");
        assert_eq!(blocked.status, ValidationStatus::Blocked);
        assert_eq!(blocked.issues[0].path, "ops");

        let cycle = request(vec![StepEditOp::Reuse {
            path: "root/lyse".to_owned(),
            position: 0,
            use_id: "loop".to_owned(),
            block_version_id: "root-v1".to_owned(),
            bindings: Vec::new(),
        }]);
        let blocked = validate(&cycle, &store).expect("validate");
        assert_eq!(blocked.status, ValidationStatus::Blocked);
        assert_eq!(
            serde_json::to_value(blocked.issues[0].code).unwrap(),
            "INVALID_PROPOSAL"
        );
        assert!(blocked.issues[0].message.contains("循环"));
    }

    #[test]
    fn a_changed_child_that_is_no_longer_head_is_a_version_conflict() {
        let mut store = MemoryStore::fixture();
        // A concurrent writer already moved the child block forward.
        let mut moved = store.version("lyse-v1");
        moved.version_id = "lyse-v2".to_owned();
        moved.revision = 2;
        moved.data.version = 2;
        store.blocks.push(moved);
        let request = request(vec![StepEditOp::Insert {
            path: "root/lyse".to_owned(),
            position: 1,
            step: text("wait"),
        }]);
        let blocked = validate(&request, &store).expect("validate");
        assert_eq!(blocked.status, ValidationStatus::Blocked);
        assert_eq!(
            serde_json::to_value(blocked.issues[0].code).unwrap(),
            "VERSION_CONFLICT"
        );
        assert_eq!(blocked.issues[0].path, "blocks.b-lyse");
        assert!(store.appended.is_empty());
    }

    #[test]
    fn the_wire_shape_is_closed_and_uses_the_four_op_kinds() {
        let payload = serde_json::json!({
            "edit_id": "se-1",
            "block_version_id": "root-v1",
            "expected_version": 1,
            "origin": {"source": "agent"},
            "ops": [
                {"op": "insert", "path": "root", "position": 0,
                 "step": {"kind": "text", "use_id": "a", "title": "A", "instruction": "do A"}},
                {"op": "reorder", "path": "root", "order": ["a"]},
                {"op": "reuse", "path": "root", "position": 0, "use_id": "b",
                 "block_version_id": "wash-v1"},
                {"op": "bind", "path": "root/b", "bindings": []}
            ]
        });
        let request: StepEditRequest = serde_json::from_value(payload).expect("wire shape");
        assert_eq!(request.schema_version, STEP_EDIT_SCHEMA_VERSION);
        assert_eq!(
            request.ops.iter().map(|op| op.kind()).collect::<Vec<_>>(),
            [
                StepEditOpKind::Insert,
                StepEditOpKind::Reorder,
                StepEditOpKind::Reuse,
                StepEditOpKind::Bind
            ]
        );
        let mut extra = serde_json::to_value(&request).unwrap();
        extra["requires_confirmation"] = serde_json::json!(false);
        assert!(serde_json::from_value::<StepEditRequest>(extra).is_err());
        let unknown_op = serde_json::json!({"op": "delete", "path": "root"});
        assert!(serde_json::from_value::<StepEditOp>(unknown_op).is_err());
    }
}
