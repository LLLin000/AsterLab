//! Agent edit v1: `propose -> validate -> confirm -> commit` contract.
//!
//! Types only. This module reads no store, writes no record and is not wired
//! into the CLI or the GUI. The normative contract, the error-code table and
//! the RNA parameter-binding dry-run live in `docs/contracts/agent-edit-v1.md`.
//!
//! Split of authority:
//!
//! - an Agent builds [`Proposal`] from read models and never declares its own
//!   invalidation set, confirmation requirement, digest or `confirmed` flag;
//! - Core answers with [`ValidationResult`], which carries the authoritative
//!   `current` values, the Core-computed digest and the confirmation decision;
//! - a human confirmation is a [`ConfirmRequest`], which re-sends the exact
//!   proposal it approves;
//! - [`CommitResult`] is produced by a single version-bumping transaction or
//!   the commit fails with [`AgentEditError`] and writes nothing.

use crate::semantic_changeset::{ChangeTarget, FieldOperation};
use serde::{Deserialize, Serialize};
use serde_json::Value;

pub const AGENT_EDIT_SCHEMA_VERSION: u32 = 1;

fn default_schema_version() -> u32 {
    AGENT_EDIT_SCHEMA_VERSION
}

/// Closed error code vocabulary. Wire strings are stable and are what
/// downstream cards and the CLI surface; do not rename or reuse them.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum AgentEditErrorCode {
    /// The proposal is malformed: empty diff list, duplicate or parent/child
    /// conflicting paths, `set` without `after`, `unset` with `after`, or no
    /// effective change.
    InvalidProposal,
    /// The binding is not admissible for this target: unknown parameter slot,
    /// wrong binding owner, or a binding whose `parameter_key` does not match
    /// the path slot.
    InvalidBinding,
    /// The value carries a unit that is not exactly the slot unit. No implicit
    /// conversion is performed.
    UnitMismatch,
    /// A referenced object, version, scope or step path no longer resolves.
    StaleReference,
    /// The edit touches a field outside the v1 authority of this command
    /// (identity, module contract, frozen snapshot, run facts) or an
    /// unsupported target kind.
    ForbiddenField,
    /// `expected_version` or `base_version_id` does not match the
    /// authoritative head at validate or commit time.
    VersionConflict,
    /// Commit was attempted without the confirmation the validated proposal
    /// requires, or with an incomplete acknowledgement set.
    ConfirmationRequired,
    /// The confirmation does not apply to this proposal: digest mismatch or a
    /// confirmation produced against a different base version.
    ConfirmationMismatch,
    /// The `idempotency_key` was already used for a different proposal digest.
    IdempotencyMismatch,
}

/// Three levels from `CONTEXT.md` `ValidationIssue`. v1 validation only
/// produces `Blocking`; the other two are the shared vocabulary for the later
/// contextual checks.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum IssueSeverity {
    Blocking,
    NeedsConfirmation,
    Advisory,
}

/// Projection of [`ValidationResult`]. `Blocked` wins over
/// `NeedsConfirmation`; `Valid` means no blocking issue and no confirmation
/// requirement.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum ValidationStatus {
    Valid,
    NeedsConfirmation,
    Blocked,
}

/// Who authored the proposal. Provenance only; it never grants authority.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum EditOrigin {
    User,
    Agent,
    Tool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ProposalOrigin {
    pub source: EditOrigin,
    /// Free-text reason shown to the confirming human. Not evidence.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rationale: Option<String>,
}

/// One field-local change. `before` is the value the Agent believes it read;
/// it is a claim, not authority. `after` is the requested value.
///
/// For `params.<slot_key>` paths both sides are `ParameterBinding`-shaped JSON
/// (`parameter_key`, `value`, optional `unit`, `source`, optional
/// `source_reference`, `confirmed`). An Agent must always send
/// `confirmed: false`; Core sets the flag from [`ConfirmRequest`].
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct FieldDiff {
    /// Path relative to the target object; see the contract's path allowlist.
    pub path: String,
    pub operation: FieldOperation,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub before: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub after: Option<Value>,
}

/// The reviewable request. One proposal addresses one target and produces at
/// most one new version; v1 has no cross-object atomic commit.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct Proposal {
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    pub proposal_id: String,
    pub target: ChangeTarget,
    /// Exact immutable version record the diffs were computed against.
    pub base_version_id: String,
    /// `revision` of the target record at the time the diffs were computed.
    pub expected_version: i64,
    pub origin: ProposalOrigin,
    pub diffs: Vec<FieldDiff>,
}

/// A diff after Core resolution: `current` is authoritative (never the
/// Agent's `before`), `requested` is what would be written.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ResolvedFieldDiff {
    pub path: String,
    pub operation: FieldOperation,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub current: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub requested: Option<Value>,
    /// True only when both sides resolved and differ. Unresolved paths report
    /// false and carry a blocking issue instead.
    pub changed: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct AgentEditIssue {
    pub code: AgentEditErrorCode,
    pub severity: IssueSeverity,
    pub path: String,
    pub message: String,
    /// Objects or downstream decisions affected by this issue.
    #[serde(default)]
    pub affects: Vec<String>,
    /// Concrete next step for the Agent or the researcher.
    pub next_action: String,
}

/// Read-only validation answer. Semantic rejections are a result, not a
/// transport error: `status` is `blocked` and `issues` is non-empty.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ValidationResult {
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    pub proposal_id: String,
    /// Core-computed `sha256:<lowercase hex>` over the canonical proposal. A
    /// confirmation is valid only for this exact digest.
    pub proposal_digest: String,
    pub target: ChangeTarget,
    /// Authoritative head used for this validation.
    pub current_version_id: String,
    pub current_revision: i64,
    pub status: ValidationStatus,
    pub diffs: Vec<ResolvedFieldDiff>,
    pub issues: Vec<AgentEditIssue>,
    /// References invalidated by this edit; Core-owned, never Agent-declared.
    pub invalidated_refs: Vec<String>,
    /// Core confirmation policy. An Agent cannot set or waive it.
    pub requires_confirmation: bool,
    /// What the confirming human must be shown. Core-generated text.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub confirmation_prompt: Option<String>,
}

/// The human confirmation, which re-sends the exact proposal it approves so
/// that commit needs no server-side session.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ConfirmRequest {
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    pub proposal: Proposal,
    /// Must equal the digest Core returned in [`ValidationResult`].
    pub proposal_digest: String,
    /// Scopes idempotent replay of one commit.
    pub idempotency_key: String,
    /// Record label only; a record label is not an electronic signature.
    pub confirmed_by: String,
    /// Every `needs_confirmation` issue path the human accepted. v1 validation
    /// produces none, so this is normally empty.
    #[serde(default)]
    pub acknowledged: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub note: Option<String>,
}

/// Result of a commit that bumped exactly one version.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CommitResult {
    #[serde(default = "default_schema_version")]
    pub schema_version: u32,
    pub proposal_id: String,
    pub proposal_digest: String,
    pub target: ChangeTarget,
    pub previous_version_id: String,
    pub new_version_id: String,
    pub new_revision: i64,
    /// Values Core actually wrote; `requested` here is post-confirmation and
    /// may carry `confirmed: true` where the proposal carried `false`.
    pub applied: Vec<ResolvedFieldDiff>,
    pub invalidated_refs: Vec<String>,
    pub confirmed_by: String,
    /// RFC 3339 from the Core clock, not from the client.
    pub committed_at: String,
    pub idempotency_key: String,
    /// True when this result was replayed for a repeated idempotency key; no
    /// second version was written.
    pub replayed: bool,
}

/// Transport-level rejection for `confirm`/`commit`. Blocking validation
/// findings are [`ValidationResult::issues`], not this type.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct AgentEditError {
    pub code: AgentEditErrorCode,
    pub message: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub current_version: Option<i64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub current_version_id: Option<String>,
    #[serde(default)]
    pub issues: Vec<AgentEditIssue>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::operation_module::ParameterBinding;

    /// The contract document is the single source of the dry-run payloads:
    /// editing an example there breaks this test instead of drifting from the
    /// types silently.
    const CONTRACT_DOC: &str = include_str!("../../../docs/contracts/agent-edit-v1.md");

    fn json_blocks() -> Vec<String> {
        let mut blocks = Vec::new();
        let mut lines = CONTRACT_DOC.lines();
        while let Some(line) = lines.next() {
            if line.trim() != "```json" {
                continue;
            }
            let mut block = String::new();
            for inner in lines.by_ref() {
                if inner.trim() == "```" {
                    break;
                }
                block.push_str(inner);
                block.push('\n');
            }
            blocks.push(block);
        }
        blocks
    }

    /// `(binding shape, propose, validate, commit)` in document order.
    fn dry_run() -> (String, String, String, String) {
        let blocks = json_blocks();
        assert_eq!(
            blocks.len(),
            4,
            "contract doc must keep exactly four ```json blocks"
        );
        (
            blocks[0].clone(),
            blocks[1].clone(),
            blocks[2].clone(),
            blocks[3].clone(),
        )
    }

    const REJECTED: [(&str, &str); 5] = [
        (
            "unit_mismatch",
            r#"{"proposal_id":"ae-rna-unit","target":{"kind":"protocol","object_id":"protocol-rna-lyse","location":{"protocol_version_id":"protocol-rna-lyse-v3","scope":{"kind":"assay","assay_id":"assay-rna-extraction"},"step_path":"root/lyse/lyse"}},"base_version_id":"protocol-rna-lyse-v3","expected_version":3,"origin":{"source":"agent"},"diffs":[{"path":"params.volume_per_sample","operation":"set","after":{"parameter_key":"volume_per_sample","value":0.25,"unit":"mL","source":"protocol_fixed","confirmed":false}}]}"#,
        ),
        (
            "forbidden_field",
            r#"{"proposal_id":"ae-rna-contract","target":{"kind":"block_version","object_id":"block-rna-lyse-v3"},"base_version_id":"block-rna-lyse-v3","expected_version":3,"origin":{"source":"agent"},"diffs":[{"path":"module_contract.title","operation":"set","after":"RNA lysis"}]}"#,
        ),
        (
            "invalid_binding",
            r#"{"proposal_id":"ae-rna-slot","target":{"kind":"protocol","object_id":"protocol-rna-lyse","location":{"protocol_version_id":"protocol-rna-lyse-v3","scope":{"kind":"assay","assay_id":"assay-rna-extraction"},"step_path":"root/lyse/lyse"}},"base_version_id":"protocol-rna-lyse-v3","expected_version":3,"origin":{"source":"agent"},"diffs":[{"path":"params.volume_per_well","operation":"set","after":{"parameter_key":"volume_per_well","value":100.0,"unit":"uL","source":"protocol_fixed","confirmed":false}}]}"#,
        ),
        (
            "stale_reference",
            r#"{"proposal_id":"ae-rna-step","target":{"kind":"protocol","object_id":"protocol-rna-lyse","location":{"protocol_version_id":"protocol-rna-lyse-v3","scope":{"kind":"assay","assay_id":"assay-rna-extraction"},"step_path":"root/lyse/incubate"}},"base_version_id":"protocol-rna-lyse-v3","expected_version":3,"origin":{"source":"agent"},"diffs":[{"path":"params.volume_per_sample","operation":"set","after":{"parameter_key":"volume_per_sample","value":250.0,"unit":"uL","source":"protocol_fixed","confirmed":false}}]}"#,
        ),
        (
            "version_conflict",
            r#"{"proposal_id":"ae-rna-version","target":{"kind":"protocol","object_id":"protocol-rna-lyse","location":{"protocol_version_id":"protocol-rna-lyse-v3","scope":{"kind":"assay","assay_id":"assay-rna-extraction"},"step_path":"root/lyse/lyse"}},"base_version_id":"protocol-rna-lyse-v2","expected_version":2,"origin":{"source":"agent"},"diffs":[{"path":"params.volume_per_sample","operation":"set","after":{"parameter_key":"volume_per_sample","value":250.0,"unit":"uL","source":"protocol_fixed","confirmed":false}}]}"#,
        ),
    ];

    #[test]
    fn parameter_binding_shape_matches_existing_type() {
        let (binding, _, _, _) = dry_run();
        let binding: ParameterBinding = serde_json::from_str(&binding).unwrap();
        assert_eq!(binding.parameter_key, "volume_per_sample");
        assert_eq!(binding.unit.as_deref(), Some("uL"));
        assert!(!binding.confirmed);
        let encoded = serde_json::to_value(&binding).unwrap();
        assert_eq!(encoded["source"], "protocol_fixed");
    }

    #[test]
    fn rna_dry_run_proposal_deserializes() {
        let (_, propose, _, _) = dry_run();
        let proposal: Proposal = serde_json::from_str(&propose).unwrap();
        assert_eq!(proposal.schema_version, AGENT_EDIT_SCHEMA_VERSION);
        assert_eq!(proposal.proposal_id, "ae-rna-lyse-001");
        assert_eq!(proposal.base_version_id, "protocol-rna-lyse-v3");
        assert_eq!(proposal.expected_version, 3);
        assert_eq!(proposal.origin.source, EditOrigin::Agent);
        assert_eq!(proposal.diffs.len(), 4);
        assert_eq!(proposal.diffs[0].operation, FieldOperation::Set);
        assert_eq!(proposal.diffs[0].after.as_ref().unwrap()["unit"], "uL");
        assert_eq!(proposal.diffs[3].operation, FieldOperation::Unset);
        assert!(proposal.diffs[3].after.is_none());

        // An Agent may never carry its own confirmation flag.
        for diff in &proposal.diffs {
            if let Some(after) = &diff.after {
                assert_eq!(after["confirmed"], false);
            }
        }
    }

    #[test]
    fn validation_result_round_trips() {
        let (_, _, validate, _) = dry_run();
        let result: ValidationResult = serde_json::from_str(&validate).unwrap();
        assert_eq!(result.status, ValidationStatus::NeedsConfirmation);
        assert!(result.issues.is_empty());
        assert!(result.requires_confirmation);
        assert_eq!(result.current_revision, 3);
        assert_eq!(result.diffs.len(), 4);
        assert!(result.diffs.iter().all(|diff| diff.changed));
        assert!(result.diffs[1].current.is_none());
        assert_eq!(result.invalidated_refs.len(), 4);

        let encoded = serde_json::to_value(&result).unwrap();
        let expected: Value = serde_json::from_str(&validate).unwrap();
        assert_eq!(encoded, expected);
    }

    #[test]
    fn confirm_request_embeds_the_validated_proposal() {
        let (_, propose, validate, _) = dry_run();
        let digest = serde_json::from_str::<Value>(&validate).unwrap()["proposal_digest"].clone();
        let request = serde_json::json!({
            "schema_version": 1,
            "proposal": serde_json::from_str::<Value>(&propose).unwrap(),
            "proposal_digest": digest,
            "idempotency_key": "ae-rna-lyse-001-confirm-1",
            "confirmed_by": "researcher:zhang"
        });

        let parsed: ConfirmRequest = serde_json::from_value(request).unwrap();
        assert_eq!(parsed.proposal.proposal_id, "ae-rna-lyse-001");
        assert_eq!(parsed.proposal.diffs.len(), 4);
        assert_eq!(parsed.confirmed_by, "researcher:zhang");
        assert!(parsed.acknowledged.is_empty());
        assert!(parsed.note.is_none());
    }

    #[test]
    fn commit_result_carries_server_confirmed_values() {
        let (_, _, _, commit) = dry_run();
        let result: CommitResult = serde_json::from_str(&commit).unwrap();
        assert_eq!(result.new_version_id, "protocol-rna-lyse-v4");
        assert_eq!(result.previous_version_id, "protocol-rna-lyse-v3");
        assert_eq!(result.new_revision, 4);
        assert!(!result.replayed);
        let applied = &result.applied[0];
        assert_eq!(applied.requested.as_ref().unwrap()["confirmed"], true);
        assert_eq!(applied.current.as_ref().unwrap()["confirmed"], true);
    }

    #[test]
    fn error_codes_are_stable_wire_strings() {
        let expected = [
            (AgentEditErrorCode::InvalidProposal, "INVALID_PROPOSAL"),
            (AgentEditErrorCode::InvalidBinding, "INVALID_BINDING"),
            (AgentEditErrorCode::UnitMismatch, "UNIT_MISMATCH"),
            (AgentEditErrorCode::StaleReference, "STALE_REFERENCE"),
            (AgentEditErrorCode::ForbiddenField, "FORBIDDEN_FIELD"),
            (AgentEditErrorCode::VersionConflict, "VERSION_CONFLICT"),
            (
                AgentEditErrorCode::ConfirmationRequired,
                "CONFIRMATION_REQUIRED",
            ),
            (
                AgentEditErrorCode::ConfirmationMismatch,
                "CONFIRMATION_MISMATCH",
            ),
            (
                AgentEditErrorCode::IdempotencyMismatch,
                "IDEMPOTENCY_MISMATCH",
            ),
        ];
        for (code, wire) in expected {
            assert_eq!(serde_json::to_value(code).unwrap(), wire);
        }
    }

    #[test]
    fn rejected_examples_are_wire_valid_proposals() {
        for (label, payload) in REJECTED {
            let proposal: Proposal = serde_json::from_str(payload)
                .unwrap_or_else(|error| panic!("{label} must parse: {error}"));
            assert!(!proposal.diffs.is_empty(), "{label} must carry a diff");
        }
    }

    #[test]
    fn unknown_fields_are_rejected() {
        let (_, propose, _, _) = dry_run();
        let mut proposal: Value = serde_json::from_str(&propose).unwrap();
        proposal
            .as_object_mut()
            .unwrap()
            .insert("requires_confirmation".into(), Value::Bool(false));
        assert!(serde_json::from_value::<Proposal>(proposal).is_err());
    }
}
