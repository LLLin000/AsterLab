//! Read-only validate/preview for agent edit v1.
//!
//! [`validate`] resolves a [`Proposal`] against the authoritative head and
//! answers with a [`ValidationResult`]. It is the `validate` stage of
//! `docs/contracts/agent-edit-v1.md`: it takes `&self` store access only,
//! holds no lock, reserves no version and writes nothing. A rejection is a
//! result (`status = blocked`), never a transport error.
//!
//! Authority split:
//!
//! - the caller supplies the stored facts through [`EditTargetStore`]; this
//!   module never guesses a slot, unit, version or step path;
//! - `diffs[].current` is the value the store can prove. The record store
//!   keeps `{slot, value, unit}` per binding and no provenance, so a resolved
//!   `current` binding reports `source = unknown` and `confirmed = false`
//!   rather than inventing a source (provenance persistence is the commit
//!   card's open item, contract §11.3);
//! - `before` is only a concurrency claim: staleness is diagnosed on the
//!   facts the store backs (`value`, `unit`), never on provenance.

use crate::agent_edit::{
    AgentEditErrorCode, AgentEditIssue, FieldDiff, IssueSeverity, Proposal, ResolvedFieldDiff,
    ValidationResult, ValidationStatus, AGENT_EDIT_SCHEMA_VERSION,
};
use crate::operation_module::{ParameterBinding, ParameterValueType, ValueSource};
use crate::operation_spec::OperationSpec;
use crate::protocol_block::{ParameterSlot, ProtocolBlockVersionData, ProtocolStep, StepBinding};
use crate::semantic_changeset::{ChangeTargetKind, FieldOperation};
use crate::semantic_component::ProtocolScope;
use serde_json::{Map, Value};
use sha2::{Digest, Sha256};
use std::collections::{BTreeMap, HashSet};

/// One immutable Protocol version record as the store returns it.
#[derive(Debug, Clone, PartialEq)]
pub struct StoredProtocolVersion {
    pub version_id: String,
    pub protocol_id: String,
    pub revision: i64,
    pub root_block_version_id: Option<String>,
    pub root_bindings: Vec<StepBinding>,
}

/// One immutable ProtocolBlock version record as the store returns it.
#[derive(Debug, Clone, PartialEq)]
pub struct StoredBlockVersion {
    pub version_id: String,
    pub revision: i64,
    pub data: ProtocolBlockVersionData,
}

/// Read-only view of the records the validator needs. Implementations must not
/// write; validation uses these calls as its only access to the store.
pub trait EditTargetStore {
    /// Latest `protocol_version` record of a Protocol.
    fn protocol_head(&self, protocol_id: &str) -> Result<Option<StoredProtocolVersion>, String>;
    /// One `protocol_block_version` record by its own version id.
    fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String>;
}

/// Validate one proposal and return the read-only preview.
///
/// `Err` is reserved for a store read failure. Every semantic rejection is
/// reported inside the returned [`ValidationResult`].
pub fn validate(
    proposal: &Proposal,
    store: &dyn EditTargetStore,
) -> Result<ValidationResult, String> {
    if proposal.schema_version != AGENT_EDIT_SCHEMA_VERSION {
        let issues = vec![issue(
            AgentEditErrorCode::InvalidProposal,
            "schema_version",
            format!(
                "只接受 schema_version {}，收到 {}",
                AGENT_EDIT_SCHEMA_VERSION, proposal.schema_version
            ),
            "按 agent edit v1 组稿",
        )];
        return Ok(rejected_result(proposal, None, issues));
    }

    let structural = structural_issues(proposal);
    if !structural.is_empty() {
        return Ok(rejected_result(proposal, None, structural));
    }

    if matches!(
        proposal.target.kind,
        ChangeTargetKind::Design | ChangeTargetKind::Run
    ) {
        let issues = vec![issue_affecting(
            AgentEditErrorCode::ForbiddenField,
            "target.kind",
            "agent edit v1 不能编辑 Design 或 Run；事实与设计走各自正式命令",
            "改用 design.revise 或 Run 命令",
            vec!["全部 diff".to_owned()],
        )];
        return Ok(rejected_result(proposal, None, issues));
    }

    match proposal.target.kind {
        ChangeTargetKind::Protocol => validate_protocol(proposal, store),
        ChangeTargetKind::BlockVersion => validate_block_version(proposal, store),
        ChangeTargetKind::Design | ChangeTargetKind::Run => unreachable!("handled above"),
    }
}

fn validate_protocol(
    proposal: &Proposal,
    store: &dyn EditTargetStore,
) -> Result<ValidationResult, String> {
    let Some(location) = proposal.target.location.clone() else {
        let issues = vec![issue(
            AgentEditErrorCode::InvalidProposal,
            "target.location",
            "protocol target 必须给出 protocol_version_id、scope 与 step_path",
            "从 protocol.block.preview 读取位置后重新组稿",
        )];
        return Ok(rejected_result(proposal, None, issues));
    };
    let Some(head) = store.protocol_head(&proposal.target.object_id)? else {
        let issues = vec![issue(
            AgentEditErrorCode::StaleReference,
            "target.object_id",
            format!("Protocol {} 不存在", proposal.target.object_id),
            "重新读取 Protocol 列表后重新组稿",
        )];
        return Ok(rejected_result(proposal, None, issues));
    };

    let head_id = (head.version_id.clone(), head.revision);
    let mut issues = Vec::new();
    version_issues(proposal, &head.version_id, head.revision, &mut issues);
    if location.protocol_version_id != head.version_id {
        issues.push(issue(
            AgentEditErrorCode::StaleReference,
            "target.location.protocol_version_id",
            format!(
                "location 指向 {}，当前 head 是 {}",
                location.protocol_version_id, head.version_id
            ),
            "重新读取 protocol.block.preview 后重新组稿",
        ));
    }

    let expansion = match head.root_block_version_id.as_deref() {
        Some(root) => match expand(store, root, &head.root_bindings) {
            Ok(expansion) => Some(expansion),
            Err(message) => {
                issues.push(issue(
                    AgentEditErrorCode::StaleReference,
                    "target.location.protocol_version_id",
                    format!("Protocol 版本无法展开：{message}"),
                    "重新读取 protocol.block.preview 后重新组稿",
                ));
                None
            }
        },
        None => {
            issues.push(issue(
                AgentEditErrorCode::StaleReference,
                "target.location.step_path",
                "Protocol 版本只有正文，没有可展开的根步骤块",
                "改用带根步骤块的 Protocol 版本",
            ));
            None
        }
    };
    if let Some(expansion) = &expansion {
        issues.extend(expansion.operation_spec_issues.iter().cloned());
    }

    let facts = expansion.as_ref().and_then(|expansion| {
        let matches = expansion
            .steps
            .iter()
            .filter(|step| step.path == location.step_path)
            .collect::<Vec<_>>();
        match matches.as_slice() {
            [step] => Some(SlotFacts {
                slots: expansion.slots_of(&step.block_version_id),
                values: &step.values,
                owner: SlotOwner::ProtocolStep {
                    scope: &location.scope,
                    step_path: &step.path,
                },
            }),
            [] => {
                issues.push(issue(
                    AgentEditErrorCode::StaleReference,
                    "target.location.step_path",
                    format!("step_path {} 在展开结果中不存在", location.step_path),
                    "重新读取 protocol.block.preview 后重新组稿",
                ));
                None
            }
            _ => {
                issues.push(issue(
                    AgentEditErrorCode::StaleReference,
                    "target.location.step_path",
                    format!("step_path {} 在展开结果中不唯一", location.step_path),
                    "修正步骤块的 use_id 后重新组稿",
                ));
                None
            }
        }
    });

    let mut diffs = Vec::new();
    let mut invalidated_refs = Vec::new();
    match facts {
        Some(facts) => {
            for diff in &proposal.diffs {
                resolve_diff(diff, &facts, &mut diffs, &mut issues, &mut invalidated_refs);
            }
        }
        None => diffs.extend(proposal.diffs.iter().map(rejected_diff)),
    }
    no_effective_change(&diffs, &mut issues);
    Ok(finish(
        proposal,
        Some(head_id),
        diffs,
        issues,
        invalidated_refs,
    ))
}

fn validate_block_version(
    proposal: &Proposal,
    store: &dyn EditTargetStore,
) -> Result<ValidationResult, String> {
    let Some(block) = store.block_version(&proposal.target.object_id)? else {
        let issues = vec![issue(
            AgentEditErrorCode::StaleReference,
            "target.object_id",
            format!("步骤块版本 {} 不存在", proposal.target.object_id),
            "重新读取步骤块版本后重新组稿",
        )];
        return Ok(rejected_result(proposal, None, issues));
    };

    let head_id = (block.version_id.clone(), block.revision);
    let mut issues = Vec::new();
    version_issues(proposal, &block.version_id, block.revision, &mut issues);
    issues.extend(block_operation_spec_issues(&block.data, "steps"));

    // A block version stores slot declarations only, never bound values, so a
    // `params.<slot>` path has no current value to resolve here.
    let values = BTreeMap::new();
    let facts = SlotFacts {
        slots: &block.data.parameter_slots,
        values: &values,
        owner: SlotOwner::BlockVersion {
            version_id: &block.version_id,
        },
    };

    let mut diffs = Vec::new();
    let mut invalidated_refs = Vec::new();
    for diff in &proposal.diffs {
        resolve_diff(diff, &facts, &mut diffs, &mut issues, &mut invalidated_refs);
    }
    no_effective_change(&diffs, &mut issues);
    Ok(finish(
        proposal,
        Some(head_id),
        diffs,
        issues,
        invalidated_refs,
    ))
}

fn version_issues(
    proposal: &Proposal,
    head_version_id: &str,
    head_revision: i64,
    issues: &mut Vec<AgentEditIssue>,
) {
    if proposal.expected_version != head_revision {
        issues.push(issue_affecting(
            AgentEditErrorCode::VersionConflict,
            "expected_version",
            format!(
                "expected_version {} 与当前 revision {} 不一致",
                proposal.expected_version, head_revision
            ),
            "重新读取后重新组稿，不合并、不覆盖",
            vec!["全部 diff".to_owned()],
        ));
    }
    if proposal.base_version_id != head_version_id {
        issues.push(issue_affecting(
            AgentEditErrorCode::VersionConflict,
            "base_version_id",
            format!(
                "base_version_id {} 不是当前 head {}",
                proposal.base_version_id, head_version_id
            ),
            "重新读取后重新组稿，不合并、不覆盖",
            vec!["全部 diff".to_owned()],
        ));
    }
}

struct SlotFacts<'a> {
    slots: &'a [ParameterSlot],
    values: &'a BTreeMap<String, SlotValue>,
    owner: SlotOwner<'a>,
}

enum SlotOwner<'a> {
    ProtocolStep {
        scope: &'a ProtocolScope,
        step_path: &'a str,
    },
    BlockVersion {
        version_id: &'a str,
    },
}

fn slot_ref(owner: &SlotOwner<'_>, slot: &str) -> String {
    match owner {
        SlotOwner::ProtocolStep { scope, step_path } => {
            format!("core:binding:{}:{}/{}", scope.key(), step_path, slot)
        }
        SlotOwner::BlockVersion { version_id } => format!("core:slot:{version_id}/{slot}"),
    }
}

fn resolve_diff(
    diff: &FieldDiff,
    facts: &SlotFacts<'_>,
    diffs: &mut Vec<ResolvedFieldDiff>,
    issues: &mut Vec<AgentEditIssue>,
    invalidated_refs: &mut Vec<String>,
) {
    let Some(segments) = split_path(&diff.path) else {
        diffs.push(rejected_diff(diff));
        return;
    };
    if segments.len() != 2 || segments[0] != "params" {
        issues.push(issue(
            AgentEditErrorCode::ForbiddenField,
            &diff.path,
            format!(
                "agent edit v1 只允许 params.<slot_key> 路径，收到 {}",
                diff.path
            ),
            "改用对应正式命令：design.revise、Protocol 新版本命令或 Run 命令",
        ));
        diffs.push(rejected_diff(diff));
        return;
    }
    let slot_key = segments[1];
    let Some(slot) = facts.slots.iter().find(|slot| slot.key == slot_key) else {
        issues.push(issue(
            AgentEditErrorCode::InvalidBinding,
            &diff.path,
            format!("步骤块未声明参数槽位 {slot_key}"),
            "按 protocol.block.preview 返回的 OperationStep.parameters 选槽位",
        ));
        diffs.push(rejected_diff(diff));
        return;
    };
    invalidated_refs.push(slot_ref(&facts.owner, slot_key));

    let mut local = Vec::new();
    let requested = match diff.operation {
        FieldOperation::Set => {
            let parsed = diff
                .after
                .as_ref()
                .map(|value| serde_json::from_value::<ParameterBinding>(value.clone()));
            match parsed {
                Some(Ok(binding)) => Some(binding),
                Some(Err(error)) => {
                    local.push(issue(
                        AgentEditErrorCode::InvalidBinding,
                        &diff.path,
                        format!("绑定形状无效：{error}"),
                        "按 ParameterBinding 字段重写绑定：parameter_key、value、unit、source、source_reference、confirmed",
                    ));
                    None
                }
                None => None,
            }
        }
        FieldOperation::Unset => None,
    };

    if let Some(binding) = &requested {
        if binding.parameter_key != slot_key {
            local.push(issue(
                AgentEditErrorCode::InvalidBinding,
                format!("{}.parameter_key", diff.path),
                format!(
                    "parameter_key {} 与 path 槽位 {slot_key} 不一致",
                    binding.parameter_key
                ),
                "让 parameter_key 与 path 的槽位一致",
            ));
        }
        if !value_matches(slot.value_type, &binding.value) {
            local.push(issue(
                AgentEditErrorCode::InvalidBinding,
                format!("{}.value", diff.path),
                format!("值不符合槽位类型 {}", value_type_name(slot.value_type)),
                "按槽位 value_type 重新填值",
            ));
        }
        if binding.source == ValueSource::Unknown && !binding.value.is_null() {
            local.push(issue(
                AgentEditErrorCode::InvalidBinding,
                format!("{}.value", diff.path),
                "source 为 unknown 的绑定不能携带值",
                "提供真实来源，或把值留空",
            ));
        }
        if binding.confirmed {
            local.push(issue(
                AgentEditErrorCode::ForbiddenField,
                format!("{}.confirmed", diff.path),
                "Agent 不能声明 confirmed: true；确认只由 Core 按人工确认置位",
                "提交 confirmed: false，由研究者在 confirm 阶段确认",
            ));
        }
        if !units_match(slot.unit.as_deref(), binding.unit.as_deref()) {
            local.push(issue(
                AgentEditErrorCode::UnitMismatch,
                format!("{}.unit", diff.path),
                format!(
                    "单位必须与槽位声明精确一致（槽位 {}，绑定 {}）；不做隐式换算",
                    slot.unit.as_deref().unwrap_or("无单位"),
                    binding.unit.as_deref().unwrap_or("无单位")
                ),
                "用槽位单位重新提交；要改单位定义须另提 block_version 提案生成新版本",
            ));
        }
    }

    let current = facts
        .values
        .get(slot_key)
        .map(|resolved| current_binding(slot, resolved));

    if let (Some(before), Some(current)) = (&diff.before, &current) {
        if let Ok(before) = serde_json::from_value::<ParameterBinding>(before.clone()) {
            if before.value != current.value
                || !units_match(before.unit.as_deref(), current.unit.as_deref())
            {
                local.push(issue(
                    AgentEditErrorCode::StaleReference,
                    &diff.path,
                    "before 与权威当前值不一致，读取已过期",
                    "重新读取 protocol.block.preview 后重新组稿",
                ));
            }
        }
    }

    if diff.operation == FieldOperation::Unset && current.is_none() {
        local.push(issue(
            AgentEditErrorCode::InvalidProposal,
            &diff.path,
            format!("unset 目标 {slot_key} 本来未绑定"),
            "移除该 diff；unset 只能删除既有绑定",
        ));
    }

    let changed = local.is_empty()
        && match diff.operation {
            FieldOperation::Set => requested
                .as_ref()
                .is_some_and(|requested| current.as_ref().is_none_or(|current| !same_facts(current, requested))),
            FieldOperation::Unset => current.is_some(),
        };

    issues.extend(local);
    diffs.push(ResolvedFieldDiff {
        path: diff.path.clone(),
        operation: diff.operation,
        current: current.as_ref().and_then(|binding| binding_value(binding)),
        requested: requested.as_ref().and_then(|binding| binding_value(binding)),
        changed,
    });
}

fn same_facts(current: &ParameterBinding, requested: &ParameterBinding) -> bool {
    current.value == requested.value
        && units_match(current.unit.as_deref(), requested.unit.as_deref())
}

fn current_binding(slot: &ParameterSlot, resolved: &SlotValue) -> ParameterBinding {
    ParameterBinding {
        parameter_key: slot.key.clone(),
        value: resolved.value.clone(),
        unit: resolved.unit.clone().or_else(|| slot.unit.clone()),
        // The record store keeps no binding provenance yet; `unknown` says
        // exactly that instead of inventing a source.
        source: ValueSource::Unknown,
        source_reference: None,
        confirmed: false,
    }
}

fn binding_value(binding: &ParameterBinding) -> Option<Value> {
    serde_json::to_value(binding).ok()
}

fn value_matches(value_type: ParameterValueType, value: &Value) -> bool {
    match value_type {
        ParameterValueType::Text => value.is_string(),
        ParameterValueType::Number => value.as_f64().is_some_and(f64::is_finite),
        ParameterValueType::Integer => value.as_i64().is_some() || value.as_u64().is_some(),
        ParameterValueType::Boolean => value.is_boolean(),
    }
}

fn value_type_name(value_type: ParameterValueType) -> &'static str {
    match value_type {
        ParameterValueType::Text => "text",
        ParameterValueType::Number => "number",
        ParameterValueType::Integer => "integer",
        ParameterValueType::Boolean => "boolean",
    }
}

/// The only unit alias v1 accepts: micro-litre spellings of `uL`. Everything
/// else must match exactly; there is no implicit conversion. Callers that
/// persist a value must store the normalized spelling.
pub(crate) fn normalize_unit(unit: &str) -> &str {
    match unit {
        "µL" | "μL" | "ul" => "uL",
        other => other,
    }
}

fn units_match(declared: Option<&str>, requested: Option<&str>) -> bool {
    match (declared, requested) {
        (None, None) => true,
        (Some(declared), Some(requested)) => normalize_unit(declared) == normalize_unit(requested),
        _ => false,
    }
}

#[derive(Debug, Clone, PartialEq)]
struct SlotValue {
    value: Value,
    unit: Option<String>,
}

struct StepFrame {
    path: String,
    block_version_id: String,
    values: BTreeMap<String, SlotValue>,
}

#[derive(Default)]
struct Expansion {
    steps: Vec<StepFrame>,
    slots_by_block: BTreeMap<String, Vec<ParameterSlot>>,
    operation_spec_issues: Vec<AgentEditIssue>,
}

impl Expansion {
    fn slots_of(&self, block_version_id: &str) -> &[ParameterSlot] {
        self.slots_by_block
            .get(block_version_id)
            .map(Vec::as_slice)
            .unwrap_or(&[])
    }
}
fn block_operation_spec_issues(
    data: &ProtocolBlockVersionData,
    path_prefix: &str,
) -> Vec<AgentEditIssue> {
    data.steps
        .iter()
        .enumerate()
        .filter_map(|(index, step)| match step {
            ProtocolStep::Text {
                operation_spec: Some(spec),
                ..
            } => Some((index, spec)),
            ProtocolStep::Text {
                operation_spec: None,
                ..
            }
            | ProtocolStep::BlockUse { .. } => None,
        })
        .flat_map(|(index, spec)| {
            validate_operation_spec(
                spec,
                &format!("{path_prefix}[{index}].operation_spec"),
            )
        })
        .collect()
}

fn validate_operation_spec(spec: &OperationSpec, path: &str) -> Vec<AgentEditIssue> {
    if !spec.is_supported_schema_version() {
        return vec![issue(
            AgentEditErrorCode::InvalidProposal,
            format!("{path}.schema_version"),
            format!(
                "unsupported OperationSpec schema_version {}",
                spec.schema_version
            ),
            "升级或移除此 OperationSpec 后重新校验",
        )];
    }
    spec.validate()
        .err()
        .unwrap_or_default()
        .into_iter()
        .map(|spec_issue| {
            issue(
                AgentEditErrorCode::InvalidProposal,
                format!("{path}.{}", spec_issue.path),
                format!(
                    "{}: {}",
                    spec_issue.code.as_str(),
                    spec_issue.message
                ),
                "修正 OperationSpec 字段后重新校验",
            )
        })
        .collect()
}

fn expand(
    store: &dyn EditTargetStore,
    root_version_id: &str,
    root_bindings: &[StepBinding],
) -> Result<Expansion, String> {
    let mut expansion = Expansion::default();
    let mut active = HashSet::new();
    let mut values = BTreeMap::new();
    for binding in root_bindings {
        if let Some(value) = &binding.value {
            values.insert(
                binding.slot.clone(),
                SlotValue {
                    value: value.clone(),
                    unit: binding.unit.clone(),
                },
            );
        }
    }
    walk(
        store,
        root_version_id,
        "root",
        values,
        1,
        &mut active,
        &mut expansion,
    )?;
    Ok(expansion)
}

fn walk(
    store: &dyn EditTargetStore,
    version_id: &str,
    path: &str,
    values: BTreeMap<String, SlotValue>,
    depth: usize,
    active: &mut HashSet<String>,
    expansion: &mut Expansion,
) -> Result<(), String> {
    if depth > 12 {
        return Err(format!("步骤块组合超过 12 层：{path}"));
    }
    if !active.insert(version_id.to_owned()) {
        return Err(format!("步骤块组合出现循环引用：{version_id}"));
    }
    let result = walk_inner(store, version_id, path, values, depth, active, expansion);
    active.remove(version_id);
    result
}

fn walk_inner(
    store: &dyn EditTargetStore,
    version_id: &str,
    path: &str,
    values: BTreeMap<String, SlotValue>,
    depth: usize,
    active: &mut HashSet<String>,
    expansion: &mut Expansion,
) -> Result<(), String> {
    let Some(block) = store.block_version(version_id)? else {
        return Err(format!("步骤块版本不存在：{version_id}"));
    };
    expansion
        .slots_by_block
        .entry(version_id.to_owned())
        .or_insert_with(|| block.data.parameter_slots.clone());
    for step in &block.data.steps {
        match step {
            ProtocolStep::Text {
                use_id,
                operation_spec,
                ..
            } => {
                let step_path = format!("{path}/{use_id}");
                if let Some(spec) = operation_spec {
                    expansion.operation_spec_issues.extend(validate_operation_spec(
                        spec,
                        &format!("{step_path}.operation_spec"),
                    ));
                }
                expansion.steps.push(StepFrame {
                    path: step_path,
                    block_version_id: version_id.to_owned(),
                    values: values.clone(),
                });
            }
            ProtocolStep::BlockUse {
                use_id,
                block_version_id,
                bindings,
            } => {
                let mut child = BTreeMap::new();
                for binding in bindings {
                    if let Some(value) = &binding.value {
                        child.insert(
                            binding.slot.clone(),
                            SlotValue {
                                value: value.clone(),
                                unit: binding.unit.clone(),
                            },
                        );
                    } else if let Some(source) = &binding.source_slot {
                        if let Some(parent) = values.get(source) {
                            child.insert(binding.slot.clone(), parent.clone());
                        }
                    }
                }
                walk(
                    store,
                    block_version_id,
                    &format!("{path}/{use_id}"),
                    child,
                    depth + 1,
                    active,
                    expansion,
                )?;
            }
        }
    }
    Ok(())
}

fn structural_issues(proposal: &Proposal) -> Vec<AgentEditIssue> {
    let mut issues = Vec::new();
    if proposal.diffs.is_empty() {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "diffs",
            "proposal 至少需要一个字段级 diff",
            "按 params.<slot_key> 组稿",
        ));
        return issues;
    }
    let mut seen = HashSet::new();
    let mut parsed: Vec<(&str, Vec<&str>)> = Vec::new();
    for diff in &proposal.diffs {
        let Some(segments) = split_path(&diff.path) else {
            issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                &diff.path,
                "path 语法无效：点分、无空段、无前导/尾随点、无通配、无数组下标",
                "修正 path",
            ));
            continue;
        };
        if !seen.insert(diff.path.clone()) {
            issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                &diff.path,
                "同一 proposal 内同一 path 只能出现一次",
                "合并为一个 diff",
            ));
        }
        match diff.operation {
            FieldOperation::Set if diff.after.is_none() => issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                &diff.path,
                "set 必须提供 after",
                "补上 after",
            )),
            FieldOperation::Unset if diff.after.is_some() => issues.push(issue(
                AgentEditErrorCode::InvalidProposal,
                &diff.path,
                "unset 不得携带 after",
                "移除 after",
            )),
            _ => {}
        }
        parsed.push((diff.path.as_str(), segments));
    }
    for (index, (left_path, left)) in parsed.iter().enumerate() {
        for (right_path, right) in parsed.iter().skip(index + 1) {
            if parent_child(left, right) {
                issues.push(issue(
                    AgentEditErrorCode::InvalidProposal,
                    format!("{left_path} / {right_path}"),
                    "父子 path 冲突：不能同时修改一个节点和它的子字段",
                    "只保留具体 path",
                ));
            }
        }
    }
    issues
}

fn split_path(path: &str) -> Option<Vec<&str>> {
    if path.is_empty() {
        return None;
    }
    let mut segments = Vec::new();
    for segment in path.split('.') {
        if segment.is_empty()
            || segment.trim() != segment
            || segment.contains('*')
            || segment.contains('[')
            || segment.contains(']')
        {
            return None;
        }
        segments.push(segment);
    }
    Some(segments)
}

fn parent_child(left: &[&str], right: &[&str]) -> bool {
    let shorter = left.len().min(right.len());
    left.len() != right.len() && left[..shorter] == right[..shorter]
}

fn no_effective_change(diffs: &[ResolvedFieldDiff], issues: &mut Vec<AgentEditIssue>) {
    if issues.is_empty() && !diffs.iter().any(|diff| diff.changed) {
        issues.push(issue(
            AgentEditErrorCode::InvalidProposal,
            "diffs",
            "全部 diff 解析后没有有效变更",
            "修改至少一个参数值，或放弃该 proposal",
        ));
    }
}

fn rejected_diff(diff: &FieldDiff) -> ResolvedFieldDiff {
    ResolvedFieldDiff {
        path: diff.path.clone(),
        operation: diff.operation,
        current: None,
        requested: diff.after.clone(),
        changed: false,
    }
}

fn rejected_result(
    proposal: &Proposal,
    head: Option<(String, i64)>,
    issues: Vec<AgentEditIssue>,
) -> ValidationResult {
    let diffs = proposal.diffs.iter().map(rejected_diff).collect();
    finish(proposal, head, diffs, issues, Vec::new())
}

fn finish(
    proposal: &Proposal,
    head: Option<(String, i64)>,
    diffs: Vec<ResolvedFieldDiff>,
    issues: Vec<AgentEditIssue>,
    invalidated_refs: Vec<String>,
) -> ValidationResult {
    // v1 policy: every commit needs human confirmation; an Agent cannot waive
    // it and the proposal has no field for it.
    let requires_confirmation = true;
    let blocked = issues
        .iter()
        .any(|issue| issue.severity == IssueSeverity::Blocking);
    let status = if blocked {
        ValidationStatus::Blocked
    } else if requires_confirmation {
        ValidationStatus::NeedsConfirmation
    } else {
        ValidationStatus::Valid
    };
    ValidationResult {
        schema_version: AGENT_EDIT_SCHEMA_VERSION,
        proposal_id: proposal.proposal_id.clone(),
        proposal_digest: proposal_digest(proposal),
        target: proposal.target.clone(),
        current_version_id: head.as_ref().map(|(id, _)| id.clone()).unwrap_or_default(),
        current_revision: head.map(|(_, revision)| revision).unwrap_or(0),
        status,
        confirmation_prompt: (status != ValidationStatus::Blocked)
            .then(|| confirmation_prompt(proposal, &diffs)),
        diffs,
        issues,
        invalidated_refs,
        requires_confirmation,
    }
}

fn confirmation_prompt(proposal: &Proposal, diffs: &[ResolvedFieldDiff]) -> String {
    let where_ = proposal
        .target
        .location
        .as_ref()
        .map(|location| location.step_path.clone())
        .unwrap_or_else(|| proposal.target.object_id.clone());
    let changes = diffs
        .iter()
        .filter(|diff| diff.changed)
        .map(|diff| {
            let slot = diff.path.trim_start_matches("params.");
            format!(
                "{slot} {} → {}",
                binding_summary(diff.current.as_ref()),
                binding_summary(diff.requested.as_ref())
            )
        })
        .collect::<Vec<_>>();
    format!(
        "确认修改 {} 的 {}。确认后将为 {} 生成新版本。",
        where_,
        changes.join("；"),
        proposal.target.object_id
    )
}

fn binding_summary(value: Option<&Value>) -> String {
    let Some(value) = value else {
        return "未绑定".to_owned();
    };
    let rendered = match value.get("value") {
        Some(Value::String(text)) => text.clone(),
        Some(Value::Null) | None => return "未绑定".to_owned(),
        Some(other) => other.to_string(),
    };
    match value.get("unit").and_then(Value::as_str) {
        Some(unit) => format!("{rendered} {unit}"),
        None => rendered,
    }
}

fn issue(
    code: AgentEditErrorCode,
    path: impl Into<String>,
    message: impl Into<String>,
    next_action: impl Into<String>,
) -> AgentEditIssue {
    issue_affecting(code, path, message, next_action, Vec::new())
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

/// Core-computed `sha256:<lowercase hex>` over the canonical proposal JSON:
/// object keys sorted, array order preserved, no extra whitespace, UTF-8.
pub fn proposal_digest(proposal: &Proposal) -> String {
    let value = serde_json::to_value(proposal).unwrap_or(Value::Null);
    let canonical = canonicalize(value);
    let bytes = serde_json::to_vec(&canonical).unwrap_or_default();
    let hash = Sha256::digest(bytes);
    format!(
        "sha256:{}",
        hash.iter().map(|byte| format!("{byte:02x}")).collect::<String>()
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

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;
    use crate::operation_spec::OperationSpec;

    const STEP: &str = "root/lyse/lyse";
    const SCOPE_ASSAY: &str = "assay-rna-extraction";

    #[derive(Default)]
    struct MemoryStore {
        protocols: Vec<StoredProtocolVersion>,
        blocks: Vec<StoredBlockVersion>,
    }

    impl EditTargetStore for MemoryStore {
        fn protocol_head(
            &self,
            protocol_id: &str,
        ) -> Result<Option<StoredProtocolVersion>, String> {
            Ok(self
                .protocols
                .iter()
                .filter(|version| version.protocol_id == protocol_id)
                .max_by_key(|version| version.revision)
                .cloned())
        }

        fn block_version(
            &self,
            version_id: &str,
        ) -> Result<Option<StoredBlockVersion>, String> {
            Ok(self
                .blocks
                .iter()
                .find(|block| block.version_id == version_id)
                .cloned())
        }
    }

    fn slot(key: &str, value_type: ParameterValueType, unit: Option<&str>) -> ParameterSlot {
        ParameterSlot {
            key: key.to_owned(),
            label: key.to_owned(),
            value_type,
            unit: unit.map(str::to_owned),
            required: true,
        }
    }

    fn text_step(use_id: &str) -> ProtocolStep {
        ProtocolStep::Text {
            use_id: use_id.to_owned(),
            title: use_id.to_owned(),
            instruction: "do it".to_owned(),
            parameter_refs: Vec::new(),
            visualization: None,
            time_scope: None,
            operation_spec: None,
            checks: Vec::new(),
        }
    }

    fn store() -> MemoryStore {
        let lyse = ProtocolBlockVersionData {
            block_id: "block-rna-lyse".to_owned(),
            version: 1,
            summary: "lysis".to_owned(),
            parameter_slots: vec![
                slot("volume_per_sample", ParameterValueType::Number, Some("uL")),
                slot("sample_count", ParameterValueType::Integer, None),
                slot(
                    "target_concentration",
                    ParameterValueType::Number,
                    Some("ng/uL"),
                ),
                slot("extra_ratio", ParameterValueType::Number, None),
                slot("incubate_minutes", ParameterValueType::Integer, None),
            ],
            steps: vec![text_step("lyse"), text_step("incubate")],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
            based_on_version_id: None,
            step_edit: None,
        };
        let root = ProtocolBlockVersionData {
            block_id: "block-rna-root".to_owned(),
            version: 1,
            summary: "root".to_owned(),
            parameter_slots: Vec::new(),
            steps: vec![ProtocolStep::BlockUse {
                use_id: "lyse".to_owned(),
                block_version_id: "block-rna-lyse-v1".to_owned(),
                bindings: vec![
                    StepBinding {
                        slot: "volume_per_sample".to_owned(),
                        value: Some(json!(200.0)),
                        source_slot: None,
                        unit: Some("uL".to_owned()),
                    },
                    StepBinding {
                        slot: "target_concentration".to_owned(),
                        value: Some(json!(50.0)),
                        source_slot: None,
                        unit: Some("ng/uL".to_owned()),
                    },
                    StepBinding {
                        slot: "extra_ratio".to_owned(),
                        value: Some(json!(0.1)),
                        source_slot: None,
                        unit: None,
                    },
                ],
            }],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
            based_on_version_id: None,
            step_edit: None,
        };
        MemoryStore {
            protocols: vec![StoredProtocolVersion {
                version_id: "protocol-rna-lyse-v3".to_owned(),
                protocol_id: "protocol-rna-lyse".to_owned(),
                revision: 3,
                root_block_version_id: Some("block-rna-root-v1".to_owned()),
                root_bindings: Vec::new(),
            }],
            blocks: vec![
                StoredBlockVersion {
                    version_id: "block-rna-root-v1".to_owned(),
                    revision: 1,
                    data: root,
                },
                StoredBlockVersion {
                    version_id: "block-rna-lyse-v1".to_owned(),
                    revision: 1,
                    data: lyse,
                },
            ],
        }
    }

    fn binding(slot: &str, value: Value, unit: Option<&str>) -> Value {
        let mut encoded = json!({
            "parameter_key": slot,
            "value": value,
            "source": "protocol_fixed",
            "source_reference": "rna-kit-ifu@2026-03#step-2",
            "confirmed": false
        });
        if let Some(unit) = unit {
            encoded["unit"] = json!(unit);
        }
        encoded
    }

    fn proposal(diffs: Value) -> Proposal {
        serde_json::from_value(json!({
            "proposal_id": "ae-rna-lyse-001",
            "target": {
                "kind": "protocol",
                "object_id": "protocol-rna-lyse",
                "location": {
                    "protocol_version_id": "protocol-rna-lyse-v3",
                    "scope": {"kind": "assay", "assay_id": SCOPE_ASSAY},
                    "step_path": STEP
                }
            },
            "base_version_id": "protocol-rna-lyse-v3",
            "expected_version": 3,
            "origin": {"source": "agent", "rationale": "kit instruction update"},
            "diffs": diffs
        }))
        .unwrap()
    }

    fn validate_proposal(proposal: Proposal) -> ValidationResult {
        validate(&proposal, &store()).unwrap()
    }

    /// Root slot `transfer_volume` reaches the child slot `volume_per_sample`
    /// through a `source_slot` binding, so the resolved `current` must follow
    /// the chain instead of reporting "unbound".
    fn pass_through_store() -> MemoryStore {
        let lyse = ProtocolBlockVersionData {
            block_id: "block-pass-lyse".to_owned(),
            version: 1,
            summary: "lysis".to_owned(),
            parameter_slots: vec![slot("volume_per_sample", ParameterValueType::Number, Some("uL"))],
            steps: vec![text_step("lyse")],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
            based_on_version_id: None,
            step_edit: None,
        };
        let root = ProtocolBlockVersionData {
            block_id: "block-pass-root".to_owned(),
            version: 1,
            summary: "root".to_owned(),
            parameter_slots: vec![slot("transfer_volume", ParameterValueType::Number, Some("uL"))],
            steps: vec![ProtocolStep::BlockUse {
                use_id: "lyse".to_owned(),
                block_version_id: "block-pass-lyse-v1".to_owned(),
                bindings: vec![StepBinding {
                    slot: "volume_per_sample".to_owned(),
                    value: None,
                    source_slot: Some("transfer_volume".to_owned()),
                    unit: None,
                }],
            }],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
            based_on_version_id: None,
            step_edit: None,
        };
        MemoryStore {
            protocols: vec![StoredProtocolVersion {
                version_id: "protocol-pass-v1".to_owned(),
                protocol_id: "protocol-pass".to_owned(),
                revision: 1,
                root_block_version_id: Some("block-pass-root-v1".to_owned()),
                root_bindings: vec![StepBinding {
                    slot: "transfer_volume".to_owned(),
                    value: Some(json!(300.0)),
                    source_slot: None,
                    unit: Some("uL".to_owned()),
                }],
            }],
            blocks: vec![
                StoredBlockVersion {
                    version_id: "block-pass-root-v1".to_owned(),
                    revision: 1,
                    data: root,
                },
                StoredBlockVersion {
                    version_id: "block-pass-lyse-v1".to_owned(),
                    revision: 1,
                    data: lyse,
                },
            ],
        }
    }
    fn invalid_transfer_spec() -> OperationSpec {
        serde_json::from_value(json!({
            "schema_version": 1,
            "operation": {
                "kind": "transfer",
                "volume": { "value": 10.0, "unit": "min" },
                "source": { "container_id": "tube-1" },
                "destination": { "container_id": "plate-1" }
            }
        }))
        .unwrap()
    }

    #[test]
    fn protocol_revision_rejects_invalid_spec_on_exact_child_block_version() {
        let mut store = store();
        let ProtocolStep::Text { operation_spec, .. } = &mut store.blocks[1].data.steps[0] else {
            panic!("expected child text step");
        };
        *operation_spec = Some(invalid_transfer_spec());

        let result = validate(
            &proposal(json!([{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(250.0), Some("uL"))
            }])),
            &store,
        )
        .unwrap();
        assert_eq!(result.status, ValidationStatus::Blocked);
        assert_eq!(
            result.issues[0].path,
            "root/lyse/lyse.operation_spec.operation.volume"
        );
        assert!(result.issues[0].message.contains("unit_mismatch"));
    }

    #[test]
    fn block_revision_rejects_invalid_spec_with_step_position() {
        let mut store = store();
        let ProtocolStep::Text { operation_spec, .. } = &mut store.blocks[1].data.steps[0] else {
            panic!("expected child text step");
        };
        *operation_spec = Some(invalid_transfer_spec());
        let proposal: Proposal = serde_json::from_value(json!({
            "proposal_id": "invalid-spec-block",
            "target": { "kind": "block_version", "object_id": "block-rna-lyse-v1" },
            "base_version_id": "block-rna-lyse-v1",
            "expected_version": 1,
            "origin": { "source": "agent" },
            "diffs": [{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(250.0), Some("uL"))
            }]
        }))
        .unwrap();

        let result = validate(&proposal, &store).unwrap();
        assert_eq!(result.status, ValidationStatus::Blocked);
        assert_eq!(
            result.issues[0].path,
            "steps[0].operation_spec.operation.volume"
        );
        assert!(result.issues[0].message.contains("unit_mismatch"));
    }
    fn valid_transfer_spec() -> OperationSpec {
        serde_json::from_value(json!({
            "schema_version": 1,
            "operation": {
                "kind": "transfer",
                "volume": { "value": 10.0, "unit": "uL" },
                "source": { "container_id": "tube-1" },
                "destination": { "container_id": "plate-1" }
            }
        }))
        .unwrap()
    }

    #[test]
    fn valid_spec_on_referenced_child_is_accepted_and_legacy_step_stays_absent() {
        let mut store = store();
        let ProtocolStep::Text { operation_spec, .. } = &mut store.blocks[1].data.steps[0] else {
            panic!("expected child text step");
        };
        *operation_spec = Some(valid_transfer_spec());

        let result = validate(
            &proposal(json!([{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(250.0), Some("uL"))
            }])),
            &store,
        )
        .unwrap();
        assert!(result.issues.is_empty(), "{:?}", result.issues);

        let legacy = text_step("legacy");
        let encoded = serde_json::to_value(&legacy).unwrap();
        assert!(encoded.get("operation_spec").is_none());
        assert_eq!(
            serde_json::from_value::<ProtocolStep>(encoded).unwrap(),
            legacy
        );
    }
    #[test]
    fn protocol_block_optional_operation_spec_roundtrips() {
        let legacy = text_step("legacy");
        let legacy_json = serde_json::to_value(&legacy).unwrap();
        assert!(legacy_json.get("operation_spec").is_none());
        assert_eq!(
            serde_json::from_value::<ProtocolStep>(legacy_json).unwrap(),
            legacy
        );

        let mut typed = text_step("typed");
        let ProtocolStep::Text { operation_spec, .. } = &mut typed else {
            unreachable!();
        };
        *operation_spec = Some(valid_transfer_spec());
        let encoded = serde_json::to_value(&typed).unwrap();
        assert!(encoded.get("operation_spec").is_some());
        assert_eq!(
            serde_json::from_value::<ProtocolStep>(encoded).unwrap(),
            typed
        );
    }

    #[test]
    fn parent_slot_binding_resolves_the_current_value() {
        let proposal: Proposal = serde_json::from_value(json!({
            "proposal_id": "ae-pass-through",
            "target": {
                "kind": "protocol",
                "object_id": "protocol-pass",
                "location": {
                    "protocol_version_id": "protocol-pass-v1",
                    "scope": {"kind": "unscoped"},
                    "step_path": STEP
                }
            },
            "base_version_id": "protocol-pass-v1",
            "expected_version": 1,
            "origin": {"source": "agent"},
            "diffs": [{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(400.0), Some("uL"))
            }]
        }))
        .unwrap();
        let result = validate(&proposal, &pass_through_store()).unwrap();
        assert!(result.issues.is_empty(), "{:?}", result.issues);
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);
        let current = result.diffs[0].current.as_ref().unwrap();
        assert_eq!(current["value"], 300.0);
        assert_eq!(current["unit"], "uL");
        assert_eq!(
            result.invalidated_refs,
            vec![format!("core:binding:unscoped:{STEP}/volume_per_sample")]
        );
    }

    fn codes(result: &ValidationResult) -> Vec<(String, String)> {
        result
            .issues
            .iter()
            .map(|issue| {
                (
                    serde_json::to_value(issue.code)
                        .unwrap()
                        .as_str()
                        .unwrap_or_default()
                        .to_owned(),
                    issue.path.clone(),
                )
            })
            .collect()
    }

    #[test]
    fn illegal_binding_is_blocked() {
        // Unknown slot for this step.
        let result = validate_proposal(proposal(json!([{
            "path": "params.volume_per_well",
            "operation": "set",
            "after": binding("volume_per_well", json!(100.0), Some("uL"))
        }])));
        assert_eq!(result.status, ValidationStatus::Blocked);
        assert_eq!(
            codes(&result),
            vec![("INVALID_BINDING".to_owned(), "params.volume_per_well".to_owned())]
        );
        assert_eq!(result.diffs.len(), 1);
        assert!(!result.diffs[0].changed);
        assert!(result.confirmation_prompt.is_none());

        // Slot exists but the binding claims another key.
        let result = validate_proposal(proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_well", json!(250.0), Some("uL"))
        }])));
        assert_eq!(
            codes(&result),
            vec![(
                "INVALID_BINDING".to_owned(),
                "params.volume_per_sample.parameter_key".to_owned()
            )]
        );

        // Integer slot cannot take a fractional value.
        let result = validate_proposal(proposal(json!([{
            "path": "params.sample_count",
            "operation": "set",
            "after": binding("sample_count", json!(1.5), None)
        }])));
        assert_eq!(
            codes(&result),
            vec![("INVALID_BINDING".to_owned(), "params.sample_count.value".to_owned())]
        );
    }

    #[test]
    fn wrong_unit_is_blocked_and_alias_is_accepted() {
        let result = validate_proposal(proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(0.25), Some("mL"))
        }])));
        assert_eq!(result.status, ValidationStatus::Blocked);
        assert_eq!(
            codes(&result),
            vec![("UNIT_MISMATCH".to_owned(), "params.volume_per_sample.unit".to_owned())]
        );

        // `µL` is the only accepted alias and normalises to `uL`.
        let result = validate_proposal(proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("µL"))
        }])));
        assert!(result.issues.is_empty(), "{:?}", result.issues);
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);

        // A slot without a unit rejects a unit-carrying value.
        let result = validate_proposal(proposal(json!([{
            "path": "params.sample_count",
            "operation": "set",
            "after": binding("sample_count", json!(8), Some("uL"))
        }])));
        assert_eq!(
            codes(&result),
            vec![("UNIT_MISMATCH".to_owned(), "params.sample_count.unit".to_owned())]
        );
    }

    #[test]
    fn stale_reference_is_blocked() {
        let mut unknown_step = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        let location = unknown_step.target.location.as_mut().unwrap();
        location.step_path = "root/lyse/wash".to_owned();
        let result = validate(&unknown_step, &store()).unwrap();
        assert_eq!(result.status, ValidationStatus::Blocked);
        assert_eq!(
            codes(&result),
            vec![("STALE_REFERENCE".to_owned(), "target.location.step_path".to_owned())]
        );

        let mut stale_version = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        stale_version.target.location.as_mut().unwrap().protocol_version_id =
            "protocol-rna-lyse-v2".to_owned();
        let result = validate(&stale_version, &store()).unwrap();
        assert_eq!(
            codes(&result),
            vec![(
                "STALE_REFERENCE".to_owned(),
                "target.location.protocol_version_id".to_owned()
            )]
        );
    }

    #[test]
    fn stale_before_value_is_reported_on_backed_facts_only() {
        let mut diff = json!({
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        });
        diff["before"] = binding("volume_per_sample", json!(180.0), Some("uL"));
        let result = validate_proposal(proposal(json!([diff])));
        assert_eq!(
            codes(&result),
            vec![("STALE_REFERENCE".to_owned(), "params.volume_per_sample".to_owned())]
        );

        // Provenance the store does not keep must not be reported as staleness.
        let mut diff = json!({
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        });
        let mut before = binding("volume_per_sample", json!(200.0), Some("uL"));
        before["source"] = json!("lab_sop");
        before["confirmed"] = json!(true);
        diff["before"] = before;
        let result = validate_proposal(proposal(json!([diff])));
        assert!(result.issues.is_empty(), "{:?}", result.issues);
    }

    #[test]
    fn forbidden_fields_and_targets_are_blocked() {
        let result = validate_proposal(proposal(json!([{
            "path": "module_contract.title",
            "operation": "set",
            "after": "RNA lysis"
        }])));
        assert_eq!(
            codes(&result),
            vec![("FORBIDDEN_FIELD".to_owned(), "module_contract.title".to_owned())]
        );

        let result = validate_proposal(proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": {
                "parameter_key": "volume_per_sample",
                "value": 250.0,
                "unit": "uL",
                "source": "protocol_fixed",
                "source_reference": "rna-kit-ifu@2026-03#step-2",
                "confirmed": true
            }
        }])));
        assert_eq!(
            codes(&result),
            vec![(
                "FORBIDDEN_FIELD".to_owned(),
                "params.volume_per_sample.confirmed".to_owned()
            )]
        );

        let mut run_target = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        run_target.target = serde_json::from_value(json!({
            "kind": "run",
            "object_id": "run-1"
        }))
        .unwrap();
        let result = validate(&run_target, &store()).unwrap();
        assert_eq!(codes(&result), vec![("FORBIDDEN_FIELD".to_owned(), "target.kind".to_owned())]);
    }

    #[test]
    fn version_conflict_reports_both_version_fields() {
        let mut stale = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        stale.expected_version = 2;
        stale.base_version_id = "protocol-rna-lyse-v2".to_owned();
        let result = validate(&stale, &store()).unwrap();
        assert_eq!(
            codes(&result),
            vec![
                ("VERSION_CONFLICT".to_owned(), "expected_version".to_owned()),
                ("VERSION_CONFLICT".to_owned(), "base_version_id".to_owned())
            ]
        );
        assert_eq!(result.current_version_id, "protocol-rna-lyse-v3");
        assert_eq!(result.current_revision, 3);
        assert_eq!(result.status, ValidationStatus::Blocked);
    }

    #[test]
    fn unset_of_an_unbound_slot_is_blocked() {
        let result = validate_proposal(proposal(json!([{
            "path": "params.sample_count",
            "operation": "unset"
        }])));
        assert_eq!(
            codes(&result),
            vec![("INVALID_PROPOSAL".to_owned(), "params.sample_count".to_owned())]
        );
    }

    #[test]
    fn no_effective_change_is_blocked() {
        let result = validate_proposal(proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(200.0), Some("uL"))
        }])));
        assert_eq!(codes(&result), vec![("INVALID_PROPOSAL".to_owned(), "diffs".to_owned())]);
    }

    #[test]
    fn valid_proposal_previews_resolved_diffs_without_writing() {
        let result = validate_proposal(proposal(json!([
            {
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(250.0), Some("uL"))
            },
            {
                "path": "params.sample_count",
                "operation": "set",
                "after": binding("sample_count", json!(8), None)
            },
            {
                "path": "params.extra_ratio",
                "operation": "unset"
            }
        ])));
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);
        assert!(result.issues.is_empty());
        assert!(result.requires_confirmation);
        assert_eq!(result.current_version_id, "protocol-rna-lyse-v3");
        assert_eq!(result.current_revision, 3);
        assert_eq!(result.diffs.len(), 3);
        assert!(result.diffs.iter().all(|diff| diff.changed));

        let current = result.diffs[0].current.as_ref().unwrap();
        assert_eq!(current["parameter_key"], "volume_per_sample");
        assert_eq!(current["value"], 200.0);
        assert_eq!(current["unit"], "uL");
        assert_eq!(current["source"], "unknown");
        assert_eq!(current["confirmed"], false);

        assert!(result.diffs[1].current.is_none());
        assert_eq!(result.diffs[2].requested, None);
        assert_eq!(
            result.invalidated_refs,
            vec![
                format!("core:binding:assay:{SCOPE_ASSAY}:{STEP}/volume_per_sample"),
                format!("core:binding:assay:{SCOPE_ASSAY}:{STEP}/sample_count"),
                format!("core:binding:assay:{SCOPE_ASSAY}:{STEP}/extra_ratio"),
            ]
        );

        let prompt = result.confirmation_prompt.as_deref().unwrap();
        assert!(
            prompt.contains("volume_per_sample 200.0 uL → 250.0 uL"),
            "{prompt}"
        );
        assert!(prompt.starts_with("确认修改 root/lyse/lyse"), "{prompt}");

        assert!(result.proposal_digest.starts_with("sha256:"));
        assert_eq!(result.proposal_digest.len(), "sha256:".len() + 64);
    }

    #[test]
    fn digest_is_stable_and_sensitive() {
        let first = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        let same = serde_json::from_value(serde_json::to_value(&first).unwrap()).unwrap();
        assert_eq!(proposal_digest(&first), proposal_digest(&same));

        let mut changed = first.clone();
        changed.diffs[0].after = Some(binding("volume_per_sample", json!(251.0), Some("uL")));
        assert_ne!(proposal_digest(&first), proposal_digest(&changed));
    }

    #[test]
    fn block_version_target_uses_its_own_slots_and_refs() {
        let mut block_target = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        block_target.target = serde_json::from_value(json!({
            "kind": "block_version",
            "object_id": "block-rna-lyse-v1"
        }))
        .unwrap();
        block_target.base_version_id = "block-rna-lyse-v1".to_owned();
        block_target.expected_version = 1;
        let result = validate(&block_target, &store()).unwrap();
        assert!(result.issues.is_empty(), "{:?}", result.issues);
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);
        assert_eq!(
            result.invalidated_refs,
            vec!["core:slot:block-rna-lyse-v1/volume_per_sample".to_owned()]
        );
    }

    #[test]
    fn structural_problems_are_rejected_before_resolution() {
        let result = validate_proposal(proposal(json!([
            {"path": "params.a", "operation": "set", "after": json!(1)},
            {"path": "params.a.b", "operation": "set", "after": json!(2)},
            {"path": "params.*", "operation": "set", "after": json!(3)},
            {"path": "params.a", "operation": "set", "after": json!(4)}
        ])));
        let paths = result
            .issues
            .iter()
            .map(|issue| issue.path.clone())
            .collect::<Vec<_>>();
        assert!(paths.contains(&"params.*".to_owned()), "{paths:?}");
        assert!(paths.contains(&"params.a".to_owned()), "{paths:?}");
        assert!(
            paths.contains(&"params.a / params.a.b".to_owned()),
            "{paths:?}"
        );
        assert_eq!(result.current_version_id, "");

        let empty = serde_json::from_value(json!({
            "proposal_id": "ae-empty",
            "target": {"kind": "protocol", "object_id": "protocol-rna-lyse"},
            "base_version_id": "protocol-rna-lyse-v3",
            "expected_version": 3,
            "origin": {"source": "agent"},
            "diffs": []
        }))
        .unwrap();
        let result = validate(&empty, &store()).unwrap();
        assert_eq!(result.issues[0].code, AgentEditErrorCode::InvalidProposal);
        assert_eq!(result.issues[0].path, "diffs");
    }

    #[test]
    fn protocol_target_requires_a_location() {
        let mut no_location = proposal(json!([{
            "path": "params.volume_per_sample",
            "operation": "set",
            "after": binding("volume_per_sample", json!(250.0), Some("uL"))
        }]));
        no_location.target.location = None;
        let result = validate(&no_location, &store()).unwrap();
        assert_eq!(
            codes(&result),
            vec![("INVALID_PROPOSAL".to_owned(), "target.location".to_owned())]
        );
    }
}
