//! S1-C2 semantic ChangeSet contract.
//!
//! This is a reviewable change proposal, not a transaction. It identifies the
//! authoritative object and version, the affected step location, and local field
//! changes. Applying it is intentionally out of scope.

use crate::semantic_component::{ProtocolScope, SemanticIssue};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::HashSet;

pub const SEMANTIC_CHANGESET_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ChangeTargetKind {
    Design,
    Protocol,
    Run,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ChangeLocation {
    pub protocol_version_id: String,
    pub scope: ProtocolScope,
    pub step_path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ChangeTarget {
    pub kind: ChangeTargetKind,
    pub object_id: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub location: Option<ChangeLocation>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum FieldOperation {
    Set,
    Unset,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct FieldChange {
    pub path: String,
    pub operation: FieldOperation,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ChangeSet {
    pub changeset_id: String,
    pub target: ChangeTarget,
    pub expected_version: i64,
    pub changes: Vec<FieldChange>,
    #[serde(default)]
    pub invalidated_refs: Vec<String>,
    #[serde(default)]
    pub requires_confirmation: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ChangePreview {
    pub changeset_id: String,
    pub target: ChangeTarget,
    pub expected_version: i64,
    pub field_paths: Vec<String>,
    pub invalidated_refs: Vec<String>,
    pub requires_confirmation: bool,
}

impl ChangeSet {
    pub fn validate(&self) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = Vec::new();
        if self.changeset_id.trim().is_empty() {
            issues.push(issue(
                "empty_changeset_id",
                "changeset_id",
                "changeset id must not be empty",
            ));
        }
        if self.target.object_id.trim().is_empty() {
            issues.push(issue(
                "empty_target_id",
                "target.object_id",
                "target object id must not be empty",
            ));
        }
        if self.expected_version < 0 {
            issues.push(issue(
                "invalid_expected_version",
                "expected_version",
                "expected version must not be negative",
            ));
        }
        if let Some(location) = &self.target.location {
            if location.protocol_version_id.trim().is_empty() {
                issues.push(issue(
                    "empty_protocol_id",
                    "target.location.protocol_version_id",
                    "location Protocol version id must not be empty",
                ));
            }
            if location.step_path.trim().is_empty() {
                issues.push(issue(
                    "empty_step_path",
                    "target.location.step_path",
                    "location step path must not be empty",
                ));
            }
        }
        if self.changes.is_empty() {
            issues.push(issue(
                "empty_changes",
                "changes",
                "changeset must contain at least one field change",
            ));
        }

        let mut paths = HashSet::new();
        for change in &self.changes {
            validate_path(&change.path, &mut issues);
            if !paths.insert(change.path.clone()) {
                issues.push(issue(
                    "duplicate_path",
                    &change.path,
                    "a field path may only be changed once per changeset",
                ));
            }
            match change.operation {
                FieldOperation::Set if change.value.is_none() => {
                    issues.push(issue("missing_value", &change.path, "Set requires a value"))
                }
                FieldOperation::Unset if change.value.is_some() => issues.push(issue(
                    "unexpected_value",
                    &change.path,
                    "Unset must not carry a value",
                )),
                _ => {}
            }
        }

        for reference in &self.invalidated_refs {
            if reference.trim().is_empty() {
                issues.push(issue(
                    "empty_invalidated_ref",
                    "invalidated_refs",
                    "invalidated references must not be empty",
                ));
            }
        }

        finish(issues)
    }

    /// Check the proposal against the current authoritative version. No object
    /// is loaded or written here; the caller supplies the version it read.
    pub fn validate_against(&self, current_version: i64) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = self.validate().err().unwrap_or_default();
        if self.expected_version != current_version {
            issues.push(issue(
                "version_conflict",
                "expected_version",
                "expected version does not match the current authoritative version",
            ));
        }
        finish(issues)
    }

    pub fn preview(&self) -> ChangePreview {
        ChangePreview {
            changeset_id: self.changeset_id.clone(),
            target: self.target.clone(),
            expected_version: self.expected_version,
            field_paths: self
                .changes
                .iter()
                .map(|change| change.path.clone())
                .collect(),
            invalidated_refs: self.invalidated_refs.clone(),
            requires_confirmation: self.requires_confirmation,
        }
    }
}

fn validate_path(path: &str, issues: &mut Vec<SemanticIssue>) {
    if path.trim().is_empty() {
        issues.push(issue("empty_path", path, "field path must not be empty"));
        return;
    }
    if path.starts_with('.') || path.ends_with('.') || path.contains("..") {
        issues.push(issue(
            "invalid_path",
            path,
            "field path must not contain empty segments",
        ));
    }
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

fn finish(issues: Vec<SemanticIssue>) -> Result<(), Vec<SemanticIssue>> {
    if issues.is_empty() {
        Ok(())
    } else {
        Err(issues)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn target() -> ChangeTarget {
        ChangeTarget {
            kind: ChangeTargetKind::Protocol,
            object_id: "protocol-v1".into(),
            location: Some(ChangeLocation {
                protocol_version_id: "protocol-v1".into(),
                scope: ProtocolScope::Assay {
                    assay_id: "cck8".into(),
                },
                step_path: "cck8/transfer".into(),
            }),
        }
    }

    fn changeset(changes: Vec<FieldChange>) -> ChangeSet {
        ChangeSet {
            changeset_id: "cs-1".into(),
            target: target(),
            expected_version: 3,
            changes,
            invalidated_refs: vec!["binding:cck8:volume".into()],
            requires_confirmation: true,
        }
    }

    #[test]
    fn valid_set_and_unset_preview_exact_target() {
        let set = changeset(vec![
            FieldChange {
                path: "params.volume".into(),
                operation: FieldOperation::Set,
                value: Some(json!(100.0)),
            },
            FieldChange {
                path: "params.note".into(),
                operation: FieldOperation::Unset,
                value: None,
            },
        ]);
        assert!(set.validate().is_ok());
        let preview = set.preview();
        assert_eq!(preview.field_paths, vec!["params.volume", "params.note"]);
        assert_eq!(preview.target.object_id, "protocol-v1");
        assert_eq!(preview.expected_version, 3);
        assert!(preview.requires_confirmation);
    }

    #[test]
    fn duplicate_paths_are_rejected() {
        let duplicate = changeset(vec![
            FieldChange {
                path: "params.volume".into(),
                operation: FieldOperation::Set,
                value: Some(json!(100.0)),
            },
            FieldChange {
                path: "params.volume".into(),
                operation: FieldOperation::Unset,
                value: None,
            },
        ]);
        let errors = duplicate.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "duplicate_path"));
    }

    #[test]
    fn set_requires_value_and_unset_rejects_value() {
        let missing = changeset(vec![FieldChange {
            path: "params.volume".into(),
            operation: FieldOperation::Set,
            value: None,
        }]);
        assert!(missing
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "missing_value"));

        let unexpected = changeset(vec![FieldChange {
            path: "params.volume".into(),
            operation: FieldOperation::Unset,
            value: Some(json!(1)),
        }]);
        assert!(unexpected
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "unexpected_value"));
    }

    #[test]
    fn invalid_paths_and_empty_changes_are_rejected() {
        let invalid = changeset(vec![FieldChange {
            path: "params..volume".into(),
            operation: FieldOperation::Set,
            value: Some(json!(1)),
        }]);
        assert!(invalid
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "invalid_path"));

        let empty = changeset(vec![]);
        assert!(empty
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "empty_changes"));
    }

    #[test]
    fn version_conflict_is_explicit() {
        let set = changeset(vec![FieldChange {
            path: "params.volume".into(),
            operation: FieldOperation::Set,
            value: Some(json!(100.0)),
        }]);
        assert!(set.validate_against(3).is_ok());
        let errors = set.validate_against(4).unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "version_conflict"));
    }

    #[test]
    fn target_location_is_required_for_semantic_precision_when_supplied() {
        let mut set = changeset(vec![FieldChange {
            path: "params.volume".into(),
            operation: FieldOperation::Set,
            value: Some(json!(100.0)),
        }]);
        set.target.location.as_mut().unwrap().step_path.clear();
        let errors = set.validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "empty_step_path"));
    }
}
