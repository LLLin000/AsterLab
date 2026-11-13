//! Commit stage of agent edit v1.
//!
//! [`commit`] re-validates one human-confirmed [`ConfirmRequest`] inside the
//! caller's transaction and appends exactly one new immutable version. It is
//! the `commit` stage of `docs/contracts/agent-edit-v1.md`:
//!
//! - a stale `expected_version` / `base_version_id` is `VERSION_CONFLICT`; no
//!   merge, no automatic rebase, no record written;
//! - the digest must be the digest of the exact proposal being confirmed;
//! - an `idempotency_key` that already committed the same digest replays the
//!   original result and writes no second version; the same key with another
//!   digest is `IDEMPOTENCY_MISMATCH`;
//! - the stored provenance of the new version carries the proposal id, the
//!   digest, the confirming human, the commit time and the new revision.
//!
//! Authority split: the caller supplies transactional record access through
//! [`CommitStore`]; this module decides admissibility, applies the diffs to the
//! version payload and never mutates the previous version. A version that a
//! Run already froze is therefore untouched by construction.

use crate::agent_edit::{
    AgentEditError, AgentEditErrorCode, CommitResult, ConfirmRequest, IssueSeverity,
    ResolvedFieldDiff, AGENT_EDIT_SCHEMA_VERSION,
};
use crate::agent_edit_validate::{normalize_unit, proposal_digest, validate, EditTargetStore};
use crate::semantic_changeset::{ChangeTargetKind, FieldOperation};
use serde_json::{json, Value};
use std::collections::BTreeSet;

/// One already committed version, as the query side of [`CommitStore`]
/// returns it for idempotent replay.
#[derive(Debug, Clone, PartialEq)]
pub struct CommittedVersion {
    pub version_id: String,
    pub revision: i64,
    /// The `agent_edit` provenance object stored on the version record.
    pub provenance: Value,
}

/// Transactional store used by [`commit`]. Implementations must keep the
/// caller's transaction open for the whole call: the head re-read, the
/// idempotency lookup and the append have to share one write transaction.
pub trait CommitStore {
    /// Read-only view used to re-run validation against the authoritative head.
    fn edit_store(&self) -> &dyn EditTargetStore;

    /// Full stored payload of one `protocol_version` record.
    fn protocol_version_data(&self, version_id: &str) -> Result<Option<Value>, String>;

    /// Previous commit for this idempotency key, if any.
    fn find_commit(&self, idempotency_key: &str) -> Result<Option<CommittedVersion>, String>;

    /// Core clock, RFC 3339. Supplied by the adapter so this crate needs no
    /// clock dependency.
    fn timestamp(&self) -> String;

    /// Append one `protocol_version` with the given revision.
    fn append_protocol_version(
        &mut self,
        protocol_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String>;

    /// Append one `protocol_block_version` with the given revision.
    fn append_block_version(
        &mut self,
        block_id: &str,
        revision: i64,
        data: Value,
    ) -> Result<(String, i64), String>;
}

/// Commit failure. `Store` is an infrastructure error (I/O, corrupt row);
/// `Rejected` is a semantic refusal that wrote nothing.
#[derive(Debug, Clone, PartialEq)]
pub enum CommitError {
    Rejected(AgentEditError),
    Store(String),
}

/// Re-validate and commit one confirmed proposal in the caller's transaction.
pub fn commit(
    request: &ConfirmRequest,
    store: &mut dyn CommitStore,
) -> Result<CommitResult, CommitError> {
    if request.schema_version != AGENT_EDIT_SCHEMA_VERSION {
        return Err(rejection(
            AgentEditErrorCode::InvalidProposal,
            "schema_version",
            format!(
                "只接受 schema_version {}，收到 {}",
                AGENT_EDIT_SCHEMA_VERSION, request.schema_version
            ),
            None,
            None,
        ));
    }
    if request.idempotency_key.trim().is_empty() {
        return Err(rejection(
            AgentEditErrorCode::InvalidProposal,
            "idempotency_key",
            "commit 必须提供 idempotency_key",
            None,
            None,
        ));
    }

    let digest = proposal_digest(&request.proposal);
    if digest != request.proposal_digest {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationMismatch,
            "proposal_digest",
            format!(
                "确认的 digest 与 proposal 不符：确认 {}，实际 {}",
                request.proposal_digest, digest
            ),
            None,
            None,
        ));
    }

    if let Some(existing) = store
        .find_commit(&request.idempotency_key)
        .map_err(CommitError::Store)?
    {
        if existing.provenance["proposal_digest"].as_str() != Some(digest.as_str()) {
            return Err(rejection(
                AgentEditErrorCode::IdempotencyMismatch,
                "idempotency_key",
                format!(
                    "idempotency_key {} 已用于另一个 proposal digest",
                    request.idempotency_key
                ),
                None,
                None,
            ));
        }
        return replay_result(request, existing);
    }

    if request.confirmed_by.trim().is_empty() {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationRequired,
            "confirmed_by",
            "commit 必须记录确认人；没有确认人的写入不是人工确认",
            None,
            None,
        ));
    }

    let validation = validate(&request.proposal, store.edit_store()).map_err(CommitError::Store)?;
    if let Some(blocking) = validation
        .issues
        .iter()
        .find(|issue| issue.severity == IssueSeverity::Blocking)
    {
        let mut error = AgentEditError {
            code: blocking.code,
            message: blocking.message.clone(),
            path: Some(blocking.path.clone()),
            current_version: Some(validation.current_revision),
            current_version_id: Some(validation.current_version_id.clone()).filter(|id| !id.is_empty()),
            issues: validation.issues.clone(),
        };
        if matches!(error.code, AgentEditErrorCode::VersionConflict) {
            error.message = format!(
                "{}（当前 revision {}，当前版本 {}）",
                error.message, validation.current_revision, validation.current_version_id
            );
        }
        return Err(CommitError::Rejected(error));
    }

    let required_paths = validation
        .issues
        .iter()
        .filter(|issue| issue.severity == IssueSeverity::NeedsConfirmation)
        .map(|issue| issue.path.clone())
        .collect::<BTreeSet<_>>();
    let acknowledged = request
        .acknowledged
        .iter()
        .map(|path| path.trim().to_owned())
        .filter(|path| !path.is_empty())
        .collect::<BTreeSet<_>>();
    if let Some(extra) = acknowledged.difference(&required_paths).next() {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationMismatch,
            extra.clone(),
            format!("acknowledged 包含 validate 未报告为 needs_confirmation 的 path：{extra}"),
            Some(validation.current_revision),
            Some(validation.current_version_id.clone()),
        ));
    }
    if let Some(missing) = required_paths.difference(&acknowledged).next() {
        return Err(rejection(
            AgentEditErrorCode::ConfirmationRequired,
            missing.clone(),
            format!("缺少对 needs_confirmation 问题的确认：{missing}"),
            Some(validation.current_revision),
            Some(validation.current_version_id.clone()),
        ));
    }

    let applied = applied_diffs(&validation.diffs);
    let committed_at = store.timestamp();
    let previous_version_id = validation.current_version_id.clone();
    let new_revision = validation.current_revision + 1;
    let provenance = json!({
        "schema_version": AGENT_EDIT_SCHEMA_VERSION,
        "proposal_id": request.proposal.proposal_id,
        "proposal_digest": digest,
        "confirmed_by": request.confirmed_by.trim(),
        "committed_at": committed_at,
        "idempotency_key": request.idempotency_key,
        "note": request.note,
        "origin": request.proposal.origin,
        "previous_version_id": previous_version_id,
        "new_revision": new_revision,
        "invalidated_refs": validation.invalidated_refs,
        "applied": applied,
    });

    let (new_version_id, stored_revision) = match request.proposal.target.kind {
        ChangeTargetKind::Protocol => {
            let current = store
                .protocol_version_data(&validation.current_version_id)
                .map_err(CommitError::Store)?
                .ok_or_else(|| {
                    CommitError::Store(format!(
                        "protocol_version {} 在事务内消失",
                        validation.current_version_id
                    ))
                })?;
            let next = apply_protocol_diffs(&request.proposal, &current, &applied, store)?;
            let mut next = next;
            next["version"] = json!(new_revision);
            next["agent_edit"] = provenance;
            store
                .append_protocol_version(&request.proposal.target.object_id, new_revision, next)
                .map_err(CommitError::Store)?
        }
        ChangeTargetKind::BlockVersion => {
            return Err(rejection(
                AgentEditErrorCode::ForbiddenField,
                "target.kind",
                "v1 的步骤块版本只保存参数槽声明，没有槽位默认值可写；改使用位置绑定请走 semantic.apply_binding_patch",
                Some(validation.current_revision),
                Some(validation.current_version_id.clone()),
            ));
        }
        ChangeTargetKind::Design | ChangeTargetKind::Run => {
            return Err(CommitError::Store(
                "validate 未拒绝 design/run 目标；commit 拒绝写入".to_owned(),
            ));
        }
    };
    if stored_revision != new_revision {
        return Err(CommitError::Store(format!(
            "append 返回 revision {stored_revision}，期望 {new_revision}"
        )));
    }

    Ok(CommitResult {
        schema_version: AGENT_EDIT_SCHEMA_VERSION,
        proposal_id: request.proposal.proposal_id.clone(),
        proposal_digest: digest,
        target: request.proposal.target.clone(),
        previous_version_id,
        new_version_id,
        new_revision,
        applied,
        invalidated_refs: validation.invalidated_refs,
        confirmed_by: request.confirmed_by.trim().to_owned(),
        committed_at,
        idempotency_key: request.idempotency_key.clone(),
        replayed: false,
    })
}

/// Diffs Core actually writes, with `confirmed` set by the human confirmation.
fn applied_diffs(diffs: &[ResolvedFieldDiff]) -> Vec<ResolvedFieldDiff> {
    diffs
        .iter()
        .filter(|diff| diff.changed)
        .map(|diff| {
            let mut applied = diff.clone();
            if let Some(Value::Object(object)) = applied.requested.as_mut() {
                object.insert("confirmed".to_owned(), Value::Bool(true));
                if let Some(Value::String(unit)) = object.get("unit") {
                    let normalized = normalize_unit(unit).to_owned();
                    if &normalized != unit {
                        object.insert("unit".to_owned(), Value::String(normalized));
                    }
                }
            }
            applied
        })
        .collect()
}

/// Apply `params.<slot>` diffs to the Protocol version's own root bindings.
///
/// Only a slot bound by the Protocol version itself is writable here: a text
/// step directly inside the root block reads `root_bindings`. A deeper step
/// reads a `BlockUse` binding that lives in an immutable parent block version,
/// so it has no Protocol-level storage to append.
fn apply_protocol_diffs(
    request: &crate::agent_edit::Proposal,
    current: &Value,
    applied: &[ResolvedFieldDiff],
    store: &dyn CommitStore,
) -> Result<Value, CommitError> {
    let location = request.target.location.as_ref().ok_or_else(|| {
        CommitError::Store("validated protocol target without location".to_owned())
    })?;
    let root_block_version_id = current["root_block_version_id"]
        .as_str()
        .ok_or_else(|| CommitError::Store("Protocol 版本没有根步骤块".to_owned()))?;
    let root = store
        .edit_store()
        .block_version(root_block_version_id)
        .map_err(CommitError::Store)?
        .ok_or_else(|| {
            CommitError::Store(format!("根步骤块版本 {root_block_version_id} 不存在"))
        })?;

    let segments = location.step_path.split('/').collect::<Vec<_>>();
    if segments.len() != 2 || segments[0] != "root" {
        return Err(rejection(
            AgentEditErrorCode::ForbiddenField,
            "target.location.step_path",
            format!(
                "Protocol 版本只能提交根块自身的参数绑定，收到 {}；子步骤绑定属于不可变块版本的 BlockUse",
                location.step_path
            ),
            Some(request.expected_version),
            Some(request.base_version_id.clone()),
        ));
    }
    let use_id = segments[1];
    let matches = root
        .data
        .steps
        .iter()
        .filter(|step| match step {
            crate::protocol_block::ProtocolStep::Text { use_id: id, .. } => id.as_str() == use_id,
            crate::protocol_block::ProtocolStep::BlockUse { use_id: id, .. } => {
                id.as_str() == use_id
            }
        })
        .count();
    if matches != 1 {
        return Err(rejection(
            AgentEditErrorCode::StaleReference,
            "target.location.step_path",
            format!("step_path {} 在根步骤块中不唯一或不存在", location.step_path),
            Some(request.expected_version),
            Some(request.base_version_id.clone()),
        ));
    }
    let is_text = root.data.steps.iter().any(|step| {
        matches!(step, crate::protocol_block::ProtocolStep::Text { use_id: id, .. } if id.as_str() == use_id)
    });
    if !is_text {
        return Err(rejection(
            AgentEditErrorCode::ForbiddenField,
            "target.location.step_path",
            format!(
                "step_path {} 指向子块使用，其参数绑定由父块版本保存；Protocol 版本只能改根块自身步骤的参数",
                location.step_path
            ),
            Some(request.expected_version),
            Some(request.base_version_id.clone()),
        ));
    }

    let mut bindings = current["root_bindings"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    for diff in applied {
        let slot = diff.path.strip_prefix("params.").ok_or_else(|| {
            CommitError::Store(format!("已解析 path 越界：{}", diff.path))
        })?;
        if !root.data.parameter_slots.iter().any(|declared| declared.key == slot) {
            return Err(rejection(
                AgentEditErrorCode::InvalidBinding,
                diff.path.clone(),
                format!("槽位 {slot} 不属于根步骤块"),
                Some(request.expected_version),
                Some(request.base_version_id.clone()),
            ));
        }
        match diff.operation {
            FieldOperation::Set => {
                let requested = diff.requested.as_ref().ok_or_else(|| {
                    CommitError::Store(format!("set {} 缺少 requested 值", diff.path))
                })?;
                let mut binding = json!({
                    "slot": slot,
                    "value": requested.get("value").cloned().unwrap_or(Value::Null),
                });
                if let Some(unit) = requested.get("unit").and_then(Value::as_str) {
                    binding["unit"] = json!(unit);
                }
                match bindings
                    .iter_mut()
                    .find(|existing| existing["slot"].as_str() == Some(slot))
                {
                    Some(existing) => *existing = binding,
                    None => bindings.push(binding),
                }
            }
            FieldOperation::Unset => {
                bindings.retain(|existing| existing["slot"].as_str() != Some(slot));
            }
        }
    }

    let mut next = current.clone();
    next["root_bindings"] = Value::Array(bindings);
    Ok(next)
}

fn replay_result(
    request: &ConfirmRequest,
    existing: CommittedVersion,
) -> Result<CommitResult, CommitError> {
    let provenance = &existing.provenance;
    let version_id = existing.version_id.clone();
    let revision = existing.revision;
    let string = |key: &str| -> Result<String, CommitError> {
        provenance
            .get(key)
            .and_then(Value::as_str)
            .map(str::to_owned)
            .ok_or_else(|| {
                CommitError::Store(format!(
                    "idempotency record {version_id} 缺少 agent_edit.{key}"
                ))
            })
    };
    let applied: Vec<ResolvedFieldDiff> = serde_json::from_value(
        provenance
            .get("applied")
            .cloned()
            .unwrap_or_else(|| json!([])),
    )
    .map_err(|error| {
        CommitError::Store(format!(
            "idempotency record {} 的 applied 无效：{error}",
            version_id
        ))
    })?;
    let invalidated_refs: Vec<String> = serde_json::from_value(
        provenance
            .get("invalidated_refs")
            .cloned()
            .unwrap_or_else(|| json!([])),
    )
    .map_err(|error| {
        CommitError::Store(format!(
            "idempotency record {} 的 invalidated_refs 无效：{error}",
            version_id
        ))
    })?;
    Ok(CommitResult {
        schema_version: AGENT_EDIT_SCHEMA_VERSION,
        proposal_id: string("proposal_id")?,
        proposal_digest: string("proposal_digest")?,
        target: request.proposal.target.clone(),
        previous_version_id: string("previous_version_id")?,
        new_version_id: version_id.clone(),
        new_revision: revision,
        applied,
        invalidated_refs,
        confirmed_by: string("confirmed_by")?,
        committed_at: string("committed_at")?,
        idempotency_key: request.idempotency_key.clone(),
        replayed: true,
    })
}

fn rejection(
    code: AgentEditErrorCode,
    path: impl Into<String>,
    message: impl Into<String>,
    current_version: Option<i64>,
    current_version_id: Option<String>,
) -> CommitError {
    CommitError::Rejected(AgentEditError {
        code,
        message: message.into(),
        path: Some(path.into()),
        current_version,
        current_version_id,
        issues: Vec::new(),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::agent_edit::Proposal;
    use crate::agent_edit_validate::{StoredBlockVersion, StoredProtocolVersion};
    use crate::operation_module::ParameterValueType;
    use crate::protocol_block::{
        ParameterSlot, ProtocolBlockVersionData, ProtocolStep, StepBinding,
    };
    use serde_json::json;

    #[derive(Default)]
    struct MemoryStore {
        protocols: Vec<MemoryProtocol>,
        blocks: Vec<MemoryBlock>,
    }

    struct MemoryProtocol {
        id: String,
        protocol_id: String,
        revision: i64,
        data: Value,
    }

    struct MemoryBlock {
        id: String,
        revision: i64,
        data: ProtocolBlockVersionData,
    }

    impl MemoryStore {
        fn protocol(&self, version_id: &str) -> Option<&MemoryProtocol> {
            self.protocols.iter().find(|record| record.id == version_id)
        }
    }

    impl EditTargetStore for MemoryStore {
        fn protocol_head(
            &self,
            protocol_id: &str,
        ) -> Result<Option<StoredProtocolVersion>, String> {
            let Some(head) = self
                .protocols
                .iter()
                .filter(|record| record.protocol_id == protocol_id)
                .max_by_key(|record| record.revision)
            else {
                return Ok(None);
            };
            Ok(Some(StoredProtocolVersion {
                version_id: head.id.clone(),
                protocol_id: head.protocol_id.clone(),
                revision: head.revision,
                root_block_version_id: head.data["root_block_version_id"]
                    .as_str()
                    .map(str::to_owned),
                root_bindings: serde_json::from_value(
                    head.data
                        .get("root_bindings")
                        .cloned()
                        .unwrap_or_else(|| json!([])),
                )
                .map_err(|error| error.to_string())?,
            }))
        }

        fn block_version(&self, version_id: &str) -> Result<Option<StoredBlockVersion>, String> {
            Ok(self
                .blocks
                .iter()
                .find(|record| record.id == version_id)
                .map(|record| StoredBlockVersion {
                    version_id: record.id.clone(),
                    revision: record.revision,
                    data: record.data.clone(),
                }))
        }
    }

    impl CommitStore for MemoryStore {
        fn edit_store(&self) -> &dyn EditTargetStore {
            self
        }

        fn protocol_version_data(&self, version_id: &str) -> Result<Option<Value>, String> {
            Ok(self.protocol(version_id).map(|record| record.data.clone()))
        }

        fn find_commit(&self, idempotency_key: &str) -> Result<Option<CommittedVersion>, String> {
            Ok(self
                .protocols
                .iter()
                .find(|record| {
                    record.data["agent_edit"]["idempotency_key"].as_str()
                        == Some(idempotency_key)
                })
                .map(|record| CommittedVersion {
                    version_id: record.id.clone(),
                    revision: record.revision,
                    provenance: record.data["agent_edit"].clone(),
                }))
        }

        fn timestamp(&self) -> String {
            "2026-11-13T09:24:11Z".to_owned()
        }

        fn append_protocol_version(
            &mut self,
            protocol_id: &str,
            revision: i64,
            data: Value,
        ) -> Result<(String, i64), String> {
            let id = format!("{protocol_id}-v{revision}");
            self.protocols.push(MemoryProtocol {
                id: id.clone(),
                protocol_id: protocol_id.to_owned(),
                revision,
                data,
            });
            Ok((id, revision))
        }

        fn append_block_version(
            &mut self,
            block_id: &str,
            revision: i64,
            data: Value,
        ) -> Result<(String, i64), String> {
            let id = format!("{block_id}-v{revision}");
            let data = serde_json::from_value(data).map_err(|error| error.to_string())?;
            self.blocks.push(MemoryBlock {
                id: id.clone(),
                revision,
                data,
            });
            Ok((id, revision))
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
            checks: Vec::new(),
        }
    }

    fn root_store() -> MemoryStore {
        let root = ProtocolBlockVersionData {
            block_id: "block-rna-root".to_owned(),
            version: 1,
            summary: "root".to_owned(),
            parameter_slots: vec![slot(
                "volume_per_sample",
                ParameterValueType::Number,
                Some("uL"),
            )],
            steps: vec![text_step("lyse")],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
        };
        MemoryStore {
            protocols: vec![MemoryProtocol {
                id: "protocol-rna-v1".to_owned(),
                protocol_id: "protocol-rna".to_owned(),
                revision: 1,
                data: json!({
                    "protocol_id": "protocol-rna",
                    "version": 1,
                    "body": "",
                    "root_block_version_id": "block-rna-root-v1",
                    "root_bindings": []
                }),
            }],
            blocks: vec![MemoryBlock {
                id: "block-rna-root-v1".to_owned(),
                revision: 1,
                data: root,
            }],
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

    fn proposal() -> Proposal {
        serde_json::from_value(json!({
            "proposal_id": "ae-rna-lyse-001",
            "target": {
                "kind": "protocol",
                "object_id": "protocol-rna",
                "location": {
                    "protocol_version_id": "protocol-rna-v1",
                    "scope": {"kind": "unscoped"},
                    "step_path": "root/lyse"
                }
            },
            "base_version_id": "protocol-rna-v1",
            "expected_version": 1,
            "origin": {"source": "agent", "rationale": "kit instruction update"},
            "diffs": [{
                "path": "params.volume_per_sample",
                "operation": "set",
                "after": binding("volume_per_sample", json!(250.0), Some("uL"))
            }]
        }))
        .unwrap()
    }

    fn confirm(proposal: Proposal, key: &str) -> ConfirmRequest {
        let digest = proposal_digest(&proposal);
        ConfirmRequest {
            schema_version: 1,
            proposal,
            proposal_digest: digest,
            idempotency_key: key.to_owned(),
            confirmed_by: "researcher:zhang".to_owned(),
            acknowledged: Vec::new(),
            note: Some("按试剂盒说明书更新体积。".to_owned()),
        }
    }

    fn rejected_code(error: CommitError) -> (AgentEditErrorCode, String) {
        match error {
            CommitError::Rejected(error) => (
                error.code,
                error.path.unwrap_or_default(),
            ),
            CommitError::Store(message) => panic!("unexpected store error: {message}"),
        }
    }

    #[test]
    fn commit_appends_one_version_and_records_the_confirmation() {
        let mut store = root_store();
        let request = confirm(proposal(), "ae-rna-lyse-001-confirm-1");
        let result = commit(&request, &mut store).unwrap();

        assert_eq!(result.new_revision, 2);
        assert_eq!(result.new_version_id, "protocol-rna-v2");
        assert_eq!(result.previous_version_id, "protocol-rna-v1");
        assert!(!result.replayed);
        assert_eq!(result.confirmed_by, "researcher:zhang");
        assert_eq!(result.applied.len(), 1);
        assert_eq!(result.applied[0].requested.as_ref().unwrap()["confirmed"], true);
        assert_eq!(
            result.invalidated_refs,
            vec!["core:binding:unscoped:root/lyse/volume_per_sample".to_owned()]
        );

        assert_eq!(store.protocols.len(), 2);
        let next = &store.protocol("protocol-rna-v2").unwrap().data;
        assert_eq!(
            next["root_bindings"],
            json!([{"slot": "volume_per_sample", "value": 250.0, "unit": "uL"}])
        );
        assert_eq!(next["agent_edit"]["proposal_id"], "ae-rna-lyse-001");
        assert_eq!(next["agent_edit"]["proposal_digest"], result.proposal_digest);
        assert_eq!(next["agent_edit"]["confirmed_by"], "researcher:zhang");
        assert_eq!(next["agent_edit"]["idempotency_key"], "ae-rna-lyse-001-confirm-1");
        assert_eq!(next["agent_edit"]["new_revision"], 2);
        assert_eq!(next["agent_edit"]["committed_at"], "2026-11-13T09:24:11Z");

        // The previous version and its bindings are untouched.
        let previous = &store.protocol("protocol-rna-v1").unwrap().data;
        assert_eq!(previous["root_bindings"], json!([]));
        assert!(previous.get("agent_edit").is_none());
    }

    #[test]
    fn unit_alias_is_stored_as_ul() {
        let mut store = root_store();
        let mut proposal = proposal();
        proposal.diffs[0].after = Some(binding("volume_per_sample", json!(250.0), Some("µL")));
        let request = confirm(proposal, "key-unit-alias");
        let result = commit(&request, &mut store).unwrap();

        assert_eq!(result.applied[0].requested.as_ref().unwrap()["unit"], "uL");
        let next = &store.protocol("protocol-rna-v2").unwrap().data;
        assert_eq!(next["root_bindings"][0]["unit"], "uL");
        assert_eq!(next["root_bindings"][0]["value"], 250.0);
    }

    #[test]
    fn stale_proposal_is_a_version_conflict() {
        let mut store = root_store();
        let mut request = confirm(proposal(), "key-stale");
        request.proposal.expected_version = 0;
        request.proposal.base_version_id = "protocol-rna-v0".to_owned();
        // The digest covers the mutated proposal.
        request.proposal_digest = proposal_digest(&request.proposal);

        let (code, path) = rejected_code(commit(&request, &mut store).unwrap_err());
        assert_eq!(code, AgentEditErrorCode::VersionConflict);
        assert_eq!(path, "expected_version");
        assert_eq!(store.protocols.len(), 1);
    }

    #[test]
    fn digest_mismatch_is_rejected_before_writing() {
        let mut store = root_store();
        let mut request = confirm(proposal(), "key-digest");
        request.proposal_digest = format!("sha256:{}", "0".repeat(64));
        let (code, path) = rejected_code(commit(&request, &mut store).unwrap_err());
        assert_eq!(code, AgentEditErrorCode::ConfirmationMismatch);
        assert_eq!(path, "proposal_digest");
        assert_eq!(store.protocols.len(), 1);
    }

    #[test]
    fn replay_returns_the_original_result_without_a_second_version() {
        let mut store = root_store();
        let request = confirm(proposal(), "key-replay");
        let first = commit(&request, &mut store).unwrap();
        let second = commit(&request, &mut store).unwrap();

        assert!(!first.replayed);
        assert!(second.replayed);
        assert_eq!(second.new_version_id, first.new_version_id);
        assert_eq!(second.new_revision, first.new_revision);
        assert_eq!(second.applied, first.applied);
        assert_eq!(second.committed_at, first.committed_at);
        assert_eq!(second.previous_version_id, first.previous_version_id);
        assert_eq!(second.proposal_digest, first.proposal_digest);
        assert_eq!(second.invalidated_refs, first.invalidated_refs);
        assert_eq!(second.idempotency_key, first.idempotency_key);
        assert_eq!(second.target, first.target);
        assert_eq!(store.protocols.len(), 2, "replay must not append");
    }

    #[test]
    fn same_key_with_another_digest_is_an_idempotency_mismatch() {
        let mut store = root_store();
        let first = confirm(proposal(), "key-reuse");
        commit(&first, &mut store).unwrap();

        let mut next_proposal = proposal();
        next_proposal.diffs[0].after = Some(binding("volume_per_sample", json!(300.0), Some("uL")));
        let mut second = confirm(next_proposal, "key-reuse");
        second.proposal_digest = proposal_digest(&second.proposal);
        let (code, path) = rejected_code(commit(&second, &mut store).unwrap_err());
        assert_eq!(code, AgentEditErrorCode::IdempotencyMismatch);
        assert_eq!(path, "idempotency_key");
        assert_eq!(store.protocols.len(), 2);
    }

    #[test]
    fn a_commit_without_a_confirmer_is_rejected() {
        let mut store = root_store();
        let mut request = confirm(proposal(), "key-no-confirmer");
        request.confirmed_by = "  ".to_owned();
        let (code, path) = rejected_code(commit(&request, &mut store).unwrap_err());
        assert_eq!(code, AgentEditErrorCode::ConfirmationRequired);
        assert_eq!(path, "confirmed_by");
        assert_eq!(store.protocols.len(), 1);
    }

    #[test]
    fn acknowledged_paths_must_match_reported_issues() {
        let mut store = root_store();
        let mut request = confirm(proposal(), "key-acked");
        request.acknowledged = vec!["params.volume_per_sample".to_owned()];
        let (code, _) = rejected_code(commit(&request, &mut store).unwrap_err());
        assert_eq!(code, AgentEditErrorCode::ConfirmationMismatch);
        assert_eq!(store.protocols.len(), 1);
    }

    #[test]
    fn nested_binding_owner_is_not_writable_by_a_protocol_version() {
        let child = ProtocolBlockVersionData {
            block_id: "block-rna-lyse".to_owned(),
            version: 1,
            summary: "lysis".to_owned(),
            parameter_slots: vec![slot(
                "volume_per_sample",
                ParameterValueType::Number,
                Some("uL"),
            )],
            steps: vec![text_step("lyse")],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
        };
        let root = ProtocolBlockVersionData {
            block_id: "block-rna-root".to_owned(),
            version: 1,
            summary: "root".to_owned(),
            parameter_slots: Vec::new(),
            steps: vec![ProtocolStep::BlockUse {
                use_id: "lyse".to_owned(),
                block_version_id: "block-rna-lyse-v1".to_owned(),
                bindings: vec![StepBinding {
                    slot: "volume_per_sample".to_owned(),
                    value: Some(json!(200.0)),
                    source_slot: None,
                    unit: Some("uL".to_owned()),
                }],
            }],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
        };
        let mut store = MemoryStore {
            protocols: vec![MemoryProtocol {
                id: "protocol-rna-v1".to_owned(),
                protocol_id: "protocol-rna".to_owned(),
                revision: 1,
                data: json!({
                    "protocol_id": "protocol-rna",
                    "version": 1,
                    "body": "",
                    "root_block_version_id": "block-rna-root-v1",
                    "root_bindings": []
                }),
            }],
            blocks: vec![
                MemoryBlock {
                    id: "block-rna-root-v1".to_owned(),
                    revision: 1,
                    data: root,
                },
                MemoryBlock {
                    id: "block-rna-lyse-v1".to_owned(),
                    revision: 1,
                    data: child,
                },
            ],
        };
        let mut proposal = proposal();
        proposal.target.location.as_mut().unwrap().step_path = "root/lyse/lyse".to_owned();
        let request = confirm(proposal, "key-nested");
        let (code, path) = rejected_code(commit(&request, &mut store).unwrap_err());
        assert_eq!(code, AgentEditErrorCode::ForbiddenField);
        assert_eq!(path, "target.location.step_path");
        assert_eq!(store.protocols.len(), 1);
    }
}
