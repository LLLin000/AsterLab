//! Formal ProtocolBlockVersion data shape.
//!
//! This is the persisted/local block shape used by the CLI. The optional
//! `module_contract` sidecar is additive: old blocks without it remain readable.

use crate::operation_module::{
    CatalogModuleRef, ModuleOrigin, OperationModule, ParameterValueType, ReviewStatus,
};
use crate::protocol_preview::{StepCheck, StepTimeScope};
use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ParameterSlot {
    pub key: String,
    pub label: String,
    pub value_type: ParameterValueType,
    #[serde(default)]
    pub unit: Option<String>,
    #[serde(default = "required_parameter")]
    pub required: bool,
}

fn required_parameter() -> bool {
    true
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct StepBinding {
    pub slot: String,
    #[serde(default)]
    pub value: Option<Value>,
    #[serde(default)]
    pub source_slot: Option<String>,
    #[serde(default)]
    pub unit: Option<String>,
}

#[derive(Debug, Clone, Copy, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum StepVisualization {
    Transfer,
    Incubation,
    PlateLayout,
    SampleSplit,
    Measurement,
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case", deny_unknown_fields)]
pub enum ProtocolStep {
    Text {
        use_id: String,
        title: String,
        instruction: String,
        #[serde(default)]
        parameter_refs: Vec<String>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        visualization: Option<StepVisualization>,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        time_scope: Option<StepTimeScope>,
        #[serde(default, skip_serializing_if = "Vec::is_empty")]
        checks: Vec<StepCheck>,
    },
    BlockUse {
        use_id: String,
        block_version_id: String,
        #[serde(default)]
        bindings: Vec<StepBinding>,
    },
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ReviewRecord {
    pub review_id: String,
    pub subject: CatalogModuleRef,
    pub status: ReviewStatus,
    pub reviewer: String,
    pub reviewed_at: String,
    pub evidence_id: String,
    pub evidence_locator: String,
    pub content_digest: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReviewIssue {
    pub code: String,
    pub message: String,
}

impl ReviewRecord {
    pub fn validate_for(&self, module: &OperationModule) -> Vec<ReviewIssue> {
        let mut issues = Vec::new();
        if self.review_id.trim().is_empty() {
            issues.push(review_issue(
                "empty_review_id",
                "review id must not be empty",
            ));
        }
        if self.reviewer.trim().is_empty() {
            issues.push(review_issue("reviewer", "reviewer must not be empty"));
        }
        if self.reviewed_at.trim().is_empty() {
            issues.push(review_issue("reviewed_at", "reviewed_at must not be empty"));
        }
        if self.evidence_id.trim().is_empty() {
            issues.push(review_issue(
                "review_evidence",
                "evidence id must not be empty",
            ));
        }
        if self.evidence_locator.trim().is_empty() {
            issues.push(review_issue(
                "review_evidence_locator",
                "evidence locator must not be empty",
            ));
        }
        if !self.subject.matches_exact(&module.identity.catalog) {
            issues.push(review_issue(
                "review_subject_mismatch",
                "review subject does not match the exact catalog version",
            ));
        }
        if self.status.rank() < ReviewStatus::SourceChecked.rank() {
            issues.push(review_issue(
                "review_status",
                "review status is below source-checked",
            ));
        }
        if self.content_digest != module_content_digest(module) {
            issues.push(review_issue(
                "review_content_digest",
                "review content digest does not match the module",
            ));
        }
        issues
    }
}

fn review_issue(code: impl Into<String>, message: impl Into<String>) -> ReviewIssue {
    ReviewIssue {
        code: code.into(),
        message: message.into(),
    }
}

/// Canonical SHA-256 digest over the module's JSON content. Object keys are
/// sorted so the digest is stable across map insertion order.
pub fn module_content_digest(module: &OperationModule) -> String {
    let value = serde_json::to_value(module).unwrap_or(Value::Null);
    let canonical = canonicalize(value);
    let bytes = serde_json::to_vec(&canonical).unwrap_or_default();
    let hash = Sha256::digest(bytes);
    hash.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn canonicalize(value: Value) -> Value {
    match value {
        Value::Object(object) => {
            let sorted = object
                .into_iter()
                .filter(|(_, value)| !value.is_null())
                .map(|(key, value)| (key, canonicalize(value)))
                .collect::<BTreeMap<_, _>>();
            Value::Object(sorted.into_iter().collect::<Map<_, _>>())
        }
        Value::Array(items) => Value::Array(items.into_iter().map(canonicalize).collect()),
        other => other,
    }
}

#[derive(Debug, Clone, Copy, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ModuleContractAuthority {
    /// The sidecar is the current, reviewed definition of this block.
    Current,
    /// The sidecar is only provenance for a locally derived block.
    SourceOnly,
}

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ProtocolBlockVersionData {
    pub block_id: String,
    pub version: i64,
    pub summary: String,
    pub parameter_slots: Vec<ParameterSlot>,
    pub steps: Vec<ProtocolStep>,
    /// Additive sidecar for compiled catalog modules. Old blocks omit it.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub module_contract: Option<OperationModule>,
    /// Explicit local relation to the catalog module.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub module_origin: Option<ModuleOrigin>,
    /// Distinguishes a current reviewed contract from a source-only reference.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub module_contract_authority: Option<ModuleContractAuthority>,
    /// Additive review proof for imported catalog modules.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub catalog_review: Option<ReviewRecord>,
}
