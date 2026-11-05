//! S1-C2 semantic ChangeSet contract.
//!
//! This is a reviewable change proposal, not a transaction. Core decides
//! invalidation and confirmation through [`ChangeContext`]; an Agent only
//! proposes exact targets and field-local changes.

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
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ChangeSummary {
    pub changeset_id: String,
    pub target: ChangeTarget,
    pub expected_version: i64,
    pub field_paths: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ChangePreview {
    pub summary: ChangeSummary,
    pub invalidated_refs: Vec<String>,
    pub requires_confirmation: bool,
}

/// Core-owned assessment inputs. These are deliberately not fields on
/// `ChangeSet`; an Agent cannot declare its own invalidation set or bypass
/// confirmation policy.
pub trait ChangeContext {
    fn invalidated_refs(&self, changeset: &ChangeSet) -> Vec<String>;
    fn requires_confirmation(&self, changeset: &ChangeSet) -> bool;
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
        validate_target(&self.target, &self.changes, &mut issues);
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
            if protected_path(&change.path) {
                issues.push(issue(
                    "protected_field",
                    &change.path,
                    "ordinary changesets cannot edit module contracts or frozen snapshots",
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

        let paths = self
            .changes
            .iter()
            .map(|change| change.path.as_str())
            .collect::<Vec<_>>();
        for (index, left) in paths.iter().enumerate() {
            for right in paths.iter().skip(index + 1) {
                if parent_child_conflict(left, right) {
                    issues.push(issue(
                        "path_conflict",
                        format!("{left} / {right}"),
                        "parent and child field paths conflict in one changeset",
                    ));
                }
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

    /// Static summary only. It contains no claimed invalidation or confirmation.
    pub fn summary(&self) -> ChangeSummary {
        ChangeSummary {
            changeset_id: self.changeset_id.clone(),
            target: self.target.clone(),
            expected_version: self.expected_version,
            field_paths: self
                .changes
                .iter()
                .map(|change| change.path.clone())
                .collect(),
        }
    }

    /// Build a preview only after structural validation and Core assessment.
    pub fn assess(&self, context: &dyn ChangeContext) -> Result<ChangePreview, Vec<SemanticIssue>> {
        self.validate()?;
        Ok(ChangePreview {
            summary: self.summary(),
            invalidated_refs: context.invalidated_refs(self),
            requires_confirmation: context.requires_confirmation(self),
        })
    }
}

fn validate_target(
    target: &ChangeTarget,
    changes: &[FieldChange],
    issues: &mut Vec<SemanticIssue>,
) {
    match target.kind {
        ChangeTargetKind::Design => {
            if target.location.is_some() {
                issues.push(issue(
                    "location_not_applicable",
                    "target.location",
                    "a Design target cannot carry a Protocol step location",
                ));
            }
        }
        ChangeTargetKind::Protocol => {
            if target.location.is_none()
                && changes.iter().any(|change| step_scoped_path(&change.path))
            {
                issues.push(issue(
                    "target_location_required",
                    "target.location",
                    "step-scoped Protocol edits require Protocol version, scope and step path",
                ));
            }
        }
        ChangeTargetKind::Run => {
            issues.push(issue(
                "protected_run_patch",
                "target.kind",
                "ordinary changesets cannot modify Run plans, frozen snapshots or execution facts",
            ));
            if target.location.is_some() {
                issues.push(issue(
                    "location_not_applicable",
                    "target.location",
                    "Run changes use Run commands, not a Protocol step location",
                ));
            }
        }
    }
    if let Some(location) = &target.location {
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

fn step_scoped_path(path: &str) -> bool {
    ["steps", "blocks", "params", "bindings"]
        .iter()
        .any(|prefix| path == *prefix || path.starts_with(&format!("{prefix}.")))
}

fn protected_path(path: &str) -> bool {
    ["module_contract", "catalog", "frozen", "identity.catalog"]
        .iter()
        .any(|prefix| path == *prefix || path.starts_with(&format!("{prefix}.")))
}

fn parent_child_conflict(left: &str, right: &str) -> bool {
    left.starts_with(&format!("{right}.")) || right.starts_with(&format!("{left}."))
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
        }
    }

    struct TestContext;

    impl ChangeContext for TestContext {
        fn invalidated_refs(&self, _changeset: &ChangeSet) -> Vec<String> {
            vec!["core:binding:cck8:volume".into()]
        }

        fn requires_confirmation(&self, _changeset: &ChangeSet) -> bool {
            true
        }
    }

    #[test]
    fn valid_set_and_unset_summary_is_static() {
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
        let summary = set.summary();
        assert_eq!(summary.field_paths, vec!["params.volume", "params.note"]);
        assert_eq!(summary.target.object_id, "protocol-v1");
        assert_eq!(summary.expected_version, 3);
    }

    #[test]
    fn core_context_supplies_invalidation_and_confirmation() {
        let set = changeset(vec![FieldChange {
            path: "params.volume".into(),
            operation: FieldOperation::Set,
            value: Some(json!(100.0)),
        }]);
        let preview = set.assess(&TestContext).unwrap();
        assert_eq!(preview.invalidated_refs, vec!["core:binding:cck8:volume"]);
        assert!(preview.requires_confirmation);
    }

    #[test]
    fn duplicate_and_parent_child_paths_are_rejected() {
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
        assert!(duplicate
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "duplicate_path"));

        let conflicting = changeset(vec![
            FieldChange {
                path: "params".into(),
                operation: FieldOperation::Set,
                value: Some(json!({})),
            },
            FieldChange {
                path: "params.volume".into(),
                operation: FieldOperation::Set,
                value: Some(json!(100.0)),
            },
        ]);
        assert!(conflicting
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "path_conflict"));
    }

    #[test]
    fn protected_and_frozen_fields_are_rejected() {
        for path in ["module_contract", "catalog.module_id", "frozen.manifest"] {
            let set = changeset(vec![FieldChange {
                path: path.into(),
                operation: FieldOperation::Set,
                value: Some(json!(1)),
            }]);
            assert!(set
                .validate()
                .unwrap_err()
                .iter()
                .any(|issue| issue.code == "protected_field"));
        }
    }

    #[test]
    fn protocol_step_edit_requires_location() {
        let mut set = changeset(vec![FieldChange {
            path: "steps.transfer.params.volume".into(),
            operation: FieldOperation::Set,
            value: Some(json!(100.0)),
        }]);
        set.target.location = None;
        assert!(set
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "target_location_required"));
    }

    #[test]
    fn run_and_design_targets_are_guarded() {
        let mut run = changeset(vec![FieldChange {
            path: "events.od".into(),
            operation: FieldOperation::Set,
            value: Some(json!(1)),
        }]);
        run.target.kind = ChangeTargetKind::Run;
        assert!(run
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "protected_run_patch"));

        let mut design = changeset(vec![FieldChange {
            path: "objective".into(),
            operation: FieldOperation::Set,
            value: Some(json!("new")),
        }]);
        design.target.kind = ChangeTargetKind::Design;
        design.target.location = Some(target().location.unwrap());
        assert!(design
            .validate()
            .unwrap_err()
            .iter()
            .any(|issue| issue.code == "location_not_applicable"));
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
}
