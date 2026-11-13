//! Unified structured diagnostics for Core validation paths and the CLI.
//!
//! `CONTEXT.md` fixes the vocabulary for a design check: every issue carries
//! `path`, `code`, `severity`, `message`, `affects`, `next_action` and
//! `source`. Before this module each validation path used its own shape
//! (`SpecIssue`, `ContractIssue`, `SemanticIssue`, `AgentEditIssue`,
//! `TimelineIssue`, or bare strings), so an Agent had to parse prose and infer
//! the source. [`Diagnostic`] is the single wire shape; the legacy types stay
//! in place and convert into it without losing their fields.
//!
//! This module performs no I/O and never changes a validation decision: it
//! only normalizes an existing finding.

use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::agent_edit::{AgentEditIssue, IssueSeverity};
use crate::execution_timeline::TimelineIssue;
use crate::operation_module::ContractIssue;
use crate::operation_spec::SpecIssue;
use crate::semantic_component::SemanticIssue;

pub const DIAGNOSTIC_SCHEMA_VERSION: u32 = 1;

/// Three levels from `CONTEXT.md` `ValidationIssue`. A blocking diagnostic
/// refuses the write; the other two are reported without silently defaulting.
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum DiagnosticSeverity {
    Blocking,
    NeedsConfirmation,
    Advisory,
}

impl DiagnosticSeverity {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Blocking => "blocking",
            Self::NeedsConfirmation => "needs_confirmation",
            Self::Advisory => "advisory",
        }
    }

    pub fn from_wire(value: &str) -> Self {
        match value {
            "needs_confirmation" => Self::NeedsConfirmation,
            "advisory" => Self::Advisory,
            _ => Self::Blocking,
        }
    }
}

impl From<IssueSeverity> for DiagnosticSeverity {
    fn from(value: IssueSeverity) -> Self {
        match value {
            IssueSeverity::Blocking => Self::Blocking,
            IssueSeverity::NeedsConfirmation => Self::NeedsConfirmation,
            IssueSeverity::Advisory => Self::Advisory,
        }
    }
}

/// Provenance of the finding, from `CONTEXT.md` "值、来源与事实状态".
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum DiagnosticSource {
    /// The researcher stated this value or decision.
    UserConfirmed,
    /// Extracted from an imported record or document.
    ImportedRecord,
    /// Produced by a referenced Protocol version.
    ReferencedProtocol,
    /// Produced by a deterministic Core check or calculation.
    Derived,
    /// Suggested by an Agent; never a fact.
    AgentSuggestion,
    /// Origin not recorded.
    Unknown,
}

impl DiagnosticSource {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::UserConfirmed => "user_confirmed",
            Self::ImportedRecord => "imported_record",
            Self::ReferencedProtocol => "referenced_protocol",
            Self::Derived => "derived",
            Self::AgentSuggestion => "agent_suggestion",
            Self::Unknown => "unknown",
        }
    }

    pub fn from_wire(value: &str) -> Self {
        match value {
            "user_confirmed" => Self::UserConfirmed,
            "imported_record" => Self::ImportedRecord,
            "referenced_protocol" => Self::ReferencedProtocol,
            "derived" => Self::Derived,
            "agent_suggestion" => Self::AgentSuggestion,
            _ => Self::Unknown,
        }
    }
}

/// One structured validation finding. Field set is fixed by `CONTEXT.md`;
/// `affects` may be empty, everything else is always present.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct Diagnostic {
    /// Field or object path the finding anchors to.
    pub path: String,
    /// Stable machine code; Agents branch on this, not on `message`.
    pub code: String,
    pub severity: DiagnosticSeverity,
    pub message: String,
    /// Downstream objects or decisions affected by this finding.
    #[serde(default)]
    pub affects: Vec<String>,
    /// Concrete next step for the Agent or the researcher.
    pub next_action: String,
    pub source: DiagnosticSource,
}

impl Diagnostic {
    pub fn new(
        severity: DiagnosticSeverity,
        code: impl Into<String>,
        path: impl Into<String>,
        message: impl Into<String>,
    ) -> Self {
        let path = path.into();
        Self {
            next_action: default_next_action(&path),
            path,
            code: code.into(),
            severity,
            message: message.into(),
            affects: Vec::new(),
            source: DiagnosticSource::Derived,
        }
    }

    pub fn blocking(
        code: impl Into<String>,
        path: impl Into<String>,
        message: impl Into<String>,
    ) -> Self {
        Self::new(DiagnosticSeverity::Blocking, code, path, message)
    }

    pub fn needs_confirmation(
        code: impl Into<String>,
        path: impl Into<String>,
        message: impl Into<String>,
    ) -> Self {
        Self::new(
            DiagnosticSeverity::NeedsConfirmation,
            code,
            path,
            message,
        )
    }

    pub fn advisory(
        code: impl Into<String>,
        path: impl Into<String>,
        message: impl Into<String>,
    ) -> Self {
        Self::new(DiagnosticSeverity::Advisory, code, path, message)
    }

    pub fn with_affects(mut self, affects: Vec<String>) -> Self {
        self.affects = affects;
        self
    }

    pub fn with_next_action(mut self, next_action: impl Into<String>) -> Self {
        self.next_action = next_action.into();
        self
    }

    pub fn with_source(mut self, source: DiagnosticSource) -> Self {
        self.source = source;
        self
    }

    /// Build a diagnostic from a legacy prose-only error. The message is kept
    /// verbatim; a leading `CODE:` token is lifted into `code`, and the rest
    /// becomes `path` when it looks like an identifier.
    pub fn from_legacy_string(raw: impl Into<String>) -> Self {
        let raw = raw.into();
        let (code, path) = {
            let (code, path) = split_legacy_code(&raw);
            (code.to_owned(), path.to_owned())
        };
        let mut diagnostic = Self::new(DiagnosticSeverity::Blocking, code, path, raw);
        diagnostic.source = DiagnosticSource::Unknown;
        diagnostic
    }

    /// Normalize one entry of the legacy CLI `error.issues` array. Object
    /// entries keep their declared fields; bare strings use
    /// [`Diagnostic::from_legacy_string`].
    pub fn from_legacy_json(value: &Value) -> Self {
        match value {
            Value::String(raw) => Self::from_legacy_string(raw.clone()),
            Value::Object(map) => {
                let message = map
                    .get("message")
                    .and_then(Value::as_str)
                    .map(str::to_owned)
                    .unwrap_or_else(|| value.to_string());
                let code = map
                    .get("code")
                    .and_then(Value::as_str)
                    .unwrap_or("UNSPECIFIED");
                let path = map
                    .get("path")
                    .and_then(Value::as_str)
                    .unwrap_or_default();
                let severity = map
                    .get("severity")
                    .and_then(Value::as_str)
                    .map(DiagnosticSeverity::from_wire)
                    .unwrap_or(DiagnosticSeverity::Blocking);
                let source = map
                    .get("source")
                    .and_then(Value::as_str)
                    .map(DiagnosticSource::from_wire)
                    .unwrap_or(DiagnosticSource::Derived);
                let affects = map
                    .get("affects")
                    .and_then(Value::as_array)
                    .map(|items| {
                        items
                            .iter()
                            .filter_map(Value::as_str)
                            .map(str::to_owned)
                            .collect()
                    })
                    .unwrap_or_default();
                let next_action = map
                    .get("next_action")
                    .and_then(Value::as_str)
                    .map(str::to_owned)
                    .unwrap_or_else(|| default_next_action(path));
                Self {
                    path: path.to_owned(),
                    code: code.to_owned(),
                    severity,
                    message,
                    affects,
                    next_action,
                    source,
                }
            }
            other => Self::from_legacy_string(other.to_string()),
        }
    }
}

fn default_next_action(path: &str) -> String {
    if path.is_empty() {
        "correct the request and re-run validation".to_owned()
    } else {
        format!("correct {path} and re-run validation")
    }
}

/// `"STEP_NOT_MATERIALIZED: cck8/seed"` becomes `("STEP_NOT_MATERIALIZED",
/// "cck8/seed")`. Anything without an upper-case code prefix stays code-less
/// and keeps an empty path.
fn split_legacy_code(raw: &str) -> (&str, &str) {
    let Some((prefix, rest)) = raw.split_once(':') else {
        return ("UNSPECIFIED", "");
    };
    let is_code = !prefix.is_empty()
        && prefix.len() <= 64
        && prefix
            .chars()
            .all(|ch| ch.is_ascii_uppercase() || ch.is_ascii_digit() || ch == '_');
    if !is_code {
        return ("UNSPECIFIED", "");
    }
    (prefix, rest.trim())
}

impl From<&SpecIssue> for Diagnostic {
    fn from(issue: &SpecIssue) -> Self {
        Self::blocking(issue.code.as_str(), issue.path.clone(), issue.message.clone())
    }
}

impl From<SpecIssue> for Diagnostic {
    fn from(issue: SpecIssue) -> Self {
        Self::from(&issue)
    }
}

impl From<&ContractIssue> for Diagnostic {
    fn from(issue: &ContractIssue) -> Self {
        Self::blocking(issue.code.clone(), issue.path.clone(), issue.message.clone())
    }
}

impl From<ContractIssue> for Diagnostic {
    fn from(issue: ContractIssue) -> Self {
        Self::from(&issue)
    }
}

impl From<&SemanticIssue> for Diagnostic {
    fn from(issue: &SemanticIssue) -> Self {
        Self::blocking(issue.code.clone(), issue.path.clone(), issue.message.clone())
    }
}

impl From<SemanticIssue> for Diagnostic {
    fn from(issue: SemanticIssue) -> Self {
        Self::from(&issue)
    }
}

impl From<&AgentEditIssue> for Diagnostic {
    fn from(issue: &AgentEditIssue) -> Self {
        Self {
            path: issue.path.clone(),
            code: issue.code.as_str().to_owned(),
            severity: issue.severity.into(),
            message: issue.message.clone(),
            affects: issue.affects.clone(),
            next_action: issue.next_action.clone(),
            source: DiagnosticSource::Derived,
        }
    }
}

impl From<AgentEditIssue> for Diagnostic {
    fn from(issue: AgentEditIssue) -> Self {
        Self::from(&issue)
    }
}

impl From<&TimelineIssue> for Diagnostic {
    fn from(issue: &TimelineIssue) -> Self {
        Self::blocking(
            issue.code.clone(),
            issue.step_path.clone().unwrap_or_default(),
            issue.message.clone(),
        )
    }
}

impl From<TimelineIssue> for Diagnostic {
    fn from(issue: TimelineIssue) -> Self {
        Self::from(&issue)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::agent_edit::AgentEditErrorCode;
    use crate::operation_spec::SpecErrorCode;

    #[test]
    fn diagnostic_serializes_every_contract_field() {
        let diagnostic = Diagnostic::blocking("assignment_position_invalid", "assignments[0]", "bad")
            .with_affects(vec!["layout".to_owned()])
            .with_source(DiagnosticSource::ImportedRecord);
        let value = serde_json::to_value(&diagnostic).unwrap();
        for field in [
            "path",
            "code",
            "severity",
            "message",
            "affects",
            "next_action",
            "source",
        ] {
            assert!(value.get(field).is_some(), "missing {field}");
        }
        assert_eq!(value["severity"], "blocking");
        assert_eq!(value["source"], "imported_record");
        let decoded: Diagnostic = serde_json::from_value(value).unwrap();
        assert_eq!(decoded, diagnostic);
    }

    #[test]
    fn diagnostic_severity_wire_values_match_context() {
        assert_eq!(DiagnosticSeverity::Blocking.as_str(), "blocking");
        assert_eq!(
            DiagnosticSeverity::NeedsConfirmation.as_str(),
            "needs_confirmation"
        );
        assert_eq!(DiagnosticSeverity::Advisory.as_str(), "advisory");
        assert_eq!(
            DiagnosticSeverity::from_wire("advisory"),
            DiagnosticSeverity::Advisory
        );
    }

    #[test]
    fn diagnostic_source_wire_values_match_context() {
        let expected = [
            "user_confirmed",
            "imported_record",
            "referenced_protocol",
            "derived",
            "agent_suggestion",
            "unknown",
        ];
        let actual: Vec<&str> = [
            DiagnosticSource::UserConfirmed,
            DiagnosticSource::ImportedRecord,
            DiagnosticSource::ReferencedProtocol,
            DiagnosticSource::Derived,
            DiagnosticSource::AgentSuggestion,
            DiagnosticSource::Unknown,
        ]
        .iter()
        .map(|source| source.as_str())
        .collect();
        assert_eq!(actual, expected);
        for wire in expected {
            assert_eq!(DiagnosticSource::from_wire(wire).as_str(), wire);
        }
    }

    #[test]
    fn diagnostic_from_legacy_string_extracts_code_and_path() {
        let diagnostic = Diagnostic::from_legacy_string("STEP_NOT_MATERIALIZED: cck8/seed");
        assert_eq!(diagnostic.code, "STEP_NOT_MATERIALIZED");
        assert_eq!(diagnostic.path, "cck8/seed");
        assert_eq!(diagnostic.message, "STEP_NOT_MATERIALIZED: cck8/seed");
        assert_eq!(diagnostic.source, DiagnosticSource::Unknown);
        assert_eq!(diagnostic.severity, DiagnosticSeverity::Blocking);
    }

    #[test]
    fn diagnostic_from_legacy_string_without_code_keeps_message() {
        let diagnostic = Diagnostic::from_legacy_string("container id must not be empty");
        assert_eq!(diagnostic.code, "UNSPECIFIED");
        assert_eq!(diagnostic.path, "");
        assert_eq!(diagnostic.message, "container id must not be empty");
    }

    #[test]
    fn diagnostic_from_legacy_json_keeps_agent_edit_fields() {
        let issue = AgentEditIssue {
            code: AgentEditErrorCode::StaleReference,
            severity: IssueSeverity::NeedsConfirmation,
            path: "params.volume".to_owned(),
            message: "step no longer exists".to_owned(),
            affects: vec!["run".to_owned()],
            next_action: "re-read the protocol".to_owned(),
        };
        let diagnostic = Diagnostic::from_legacy_json(&serde_json::to_value(&issue).unwrap());
        assert_eq!(diagnostic.code, "STALE_REFERENCE");
        assert_eq!(diagnostic.severity, DiagnosticSeverity::NeedsConfirmation);
        assert_eq!(diagnostic.path, "params.volume");
        assert_eq!(diagnostic.affects, vec!["run".to_owned()]);
        assert_eq!(diagnostic.next_action, "re-read the protocol");
    }

    #[test]
    fn diagnostic_from_spec_issue_marks_derived_source() {
        let issue = SpecIssue {
            code: SpecErrorCode::ValueOutOfRange,
            path: "operation.temperature".to_owned(),
            message: "value must be greater than zero".to_owned(),
        };
        let diagnostic = Diagnostic::from(&issue);
        assert_eq!(diagnostic.code, "value_out_of_range");
        assert_eq!(diagnostic.path, "operation.temperature");
        assert_eq!(diagnostic.source, DiagnosticSource::Derived);
        assert_eq!(diagnostic.severity, DiagnosticSeverity::Blocking);
    }

    #[test]
    fn diagnostic_from_contract_issue_preserves_message() {
        let issue = ContractIssue {
            code: "schema_version".to_owned(),
            path: "schema_version".to_owned(),
            message: "unsupported operation-module schema version".to_owned(),
        };
        let diagnostic = Diagnostic::from(issue);
        assert_eq!(diagnostic.message, "unsupported operation-module schema version");
        assert_eq!(diagnostic.next_action, "correct schema_version and re-run validation");
    }
}
