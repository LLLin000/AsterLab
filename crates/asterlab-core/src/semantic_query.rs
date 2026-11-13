//! S3-A read-only semantic inspection and patch assessment.
//!
//! This module consumes existing `ProtocolBlockVersionData` and `ChangeSet`
//! values. It does not read SQLite, write, or apply a patch.

use crate::protocol_block::{ProtocolBlockVersionData, ProtocolStep, StepBinding};
use crate::semantic_changeset::{ChangeSet, ChangeSummary, FieldOperation};
use crate::semantic_component::SemanticIssue;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepInspection {
    pub use_id: String,
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub block_version_id: Option<String>,
    #[serde(default)]
    pub bindings: Vec<StepBinding>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instruction: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct BlockInspection {
    pub block_version_id: String,
    pub block_id: String,
    pub version: i64,
    pub summary: String,
    pub parameter_slots: Vec<crate::protocol_block::ParameterSlot>,
    pub steps: Vec<StepInspection>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub module_contract: Option<crate::operation_module::OperationModule>,
}

pub fn inspect_block(block_version_id: &str, data: &ProtocolBlockVersionData) -> BlockInspection {
    let steps = data
        .steps
        .iter()
        .map(|step| match step {
            ProtocolStep::Text {
                use_id,
                instruction,
                ..
            } => StepInspection {
                use_id: use_id.clone(),
                kind: "text".into(),
                block_version_id: None,
                bindings: vec![],
                instruction: Some(instruction.clone()),
            },
            ProtocolStep::BlockUse {
                use_id,
                block_version_id,
                bindings,
            } => StepInspection {
                use_id: use_id.clone(),
                kind: "block_use".into(),
                block_version_id: Some(block_version_id.clone()),
                bindings: bindings.clone(),
                instruction: None,
            },
        })
        .collect();
    BlockInspection {
        block_version_id: block_version_id.to_owned(),
        block_id: data.block_id.clone(),
        version: data.version,
        summary: data.summary.clone(),
        parameter_slots: data.parameter_slots.clone(),
        steps,
        module_contract: data.module_contract.clone(),
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct FieldValueChange {
    pub path: String,
    pub old_value: Option<Value>,
    pub new_value: Option<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct PatchAssessment {
    pub summary: ChangeSummary,
    pub current_version: i64,
    pub changes: Vec<FieldValueChange>,
    pub ready: bool,
    pub referencing_refs: Vec<String>,
    pub would_require_rebind: Vec<String>,
    pub invalidated_refs: Vec<String>,
    pub requires_confirmation: bool,
}

/// Assess a patch against one block version. It only recognizes
/// `steps.<use_id>.bindings.<slot>` paths; unsupported paths are rejected rather
/// than guessed. Invalidation refs are the block version itself; callers with a
/// database can add Protocol/Run references.
pub fn assess_block_patch(
    block_version_id: &str,
    data: &ProtocolBlockVersionData,
    changeset: &ChangeSet,
) -> Result<PatchAssessment, Vec<SemanticIssue>> {
    let mut issues = changeset.validate().err().unwrap_or_default();
    if changeset.expected_version != data.version {
        issues.push(issue(
            "version_conflict",
            "expected_version",
            "expected version does not match the block version",
        ));
    }

    let mut changes = Vec::new();
    for change in &changeset.changes {
        match binding_at_path(data, &change.path) {
            Some((use_id, slot, old)) => {
                let new_value = match change.operation {
                    FieldOperation::Set => change.value.clone(),
                    FieldOperation::Unset => None,
                };
                let _ = (use_id, slot);
                changes.push(FieldValueChange {
                    path: change.path.clone(),
                    old_value: old,
                    new_value,
                });
            }
            None => issues.push(issue(
                "unsupported_path",
                &change.path,
                "only steps.<use_id>.bindings.<slot> paths are assessed",
            )),
        }
    }

    if !issues.is_empty() {
        return Err(issues);
    }
    Ok(PatchAssessment {
        summary: changeset.summary(),
        current_version: data.version,
        changes,
        ready: true,
        referencing_refs: vec![format!("block_version:{block_version_id}")],
        would_require_rebind: vec![],
        invalidated_refs: vec![],
        requires_confirmation: true,
    })
}

fn binding_at_path(
    data: &ProtocolBlockVersionData,
    path: &str,
) -> Option<(String, String, Option<Value>)> {
    let parts = path.split('.').collect::<Vec<_>>();
    if parts.len() != 4 || parts[0] != "steps" || parts[2] != "bindings" {
        return None;
    }
    let use_id = parts[1];
    let slot = parts[3];
    let step = data.steps.iter().find_map(|step| match step {
        ProtocolStep::BlockUse {
            use_id: id,
            bindings,
            ..
        } if id == use_id => Some(bindings),
        _ => None,
    })?;
    let binding = step.iter().find(|binding| binding.slot == slot)?;
    Some((use_id.into(), slot.into(), binding.value.clone()))
}

fn issue(
    code: impl Into<String>,
    path: impl Into<String>,
    message: impl Into<String>,
) -> SemanticIssue {
    SemanticIssue {
        code: code.into(),
        path: path.into(),
        message: message.into(),
    }
}

pub fn search_modules(
    modules: &[crate::operation_module::OperationModule],
    query: &str,
) -> Vec<Value> {
    let query = query.trim().to_lowercase();
    modules
        .iter()
        .filter(|module| {
            query.is_empty()
                || module.identity.title.to_lowercase().contains(&query)
                || module.semantics.purpose.to_lowercase().contains(&query)
                || module
                    .identity
                    .applicability
                    .assay_families
                    .iter()
                    .any(|family| family.to_lowercase().contains(&query))
        })
        .map(|module| {
            json!({
                "catalog": module.identity.catalog,
                "title": module.identity.title,
                "purpose": module.semantics.purpose,
                "action_type": module.identity.action_type,
                "review_status": module.knowledge.review_status
            })
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::protocol_block::ProtocolBlockVersionData;

    fn data() -> ProtocolBlockVersionData {
        ProtocolBlockVersionData {
            block_id: "block-1".into(),
            version: 2,
            summary: "method".into(),
            parameter_slots: vec![],
            steps: vec![ProtocolStep::BlockUse {
                use_id: "treat".into(),
                block_version_id: "child-v1".into(),
                bindings: vec![StepBinding {
                    slot: "volume".into(),
                    value: Some(json!(50.0)),
                    source_slot: None,
                    unit: Some("uL".into()),
                }],
            }],
            module_contract: None,
            module_origin: None,
            module_contract_authority: None,
            catalog_review: None,
            based_on_version_id: None,
            step_edit: None,
        }
    }

    fn changeset() -> ChangeSet {
        serde_json::from_value(json!({
            "changeset_id": "cs-1",
            "target": {"kind": "block_version", "object_id": "block-1"},
            "expected_version": 2,
            "changes": [{
                "path": "steps.treat.bindings.volume",
                "operation": "set",
                "value": 100.0
            }]
        }))
        .unwrap()
    }

    #[test]
    fn inspect_and_assess_binding_change() {
        let data = data();
        let inspection = inspect_block("block-v1", &data);
        assert_eq!(inspection.steps[0].use_id, "treat");
        let assessment = assess_block_patch("block-v1", &data, &changeset()).unwrap();
        assert_eq!(assessment.changes[0].old_value, Some(json!(50.0)));
        assert_eq!(assessment.changes[0].new_value, Some(json!(100.0)));
        assert!(assessment.requires_confirmation);
    }

    #[test]
    fn unsupported_path_is_rejected() {
        let mut set = changeset();
        set.changes[0].path = "module_contract".into();
        let errors = assess_block_patch("block-v1", &data(), &set).unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "protected_field"));
    }
}
