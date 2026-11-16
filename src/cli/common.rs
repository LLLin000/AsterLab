use super::*;

#[derive(Debug)]
pub(crate) struct CliError {
    pub(crate) code: &'static str,
    pub(crate) message: String,
    pub(crate) exit: i32,
    /// Agent edit v1 optional fields; empty for the older command set.
    pub(crate) path: Option<String>,
    pub(crate) current_version: Option<i64>,
    pub(crate) current_version_id: Option<String>,
    pub(crate) issues: Vec<Value>,
}
impl CliError {
    pub(crate) fn input(message: impl Into<String>) -> Self {
        Self {
            code: "INVALID_INPUT",
            message: message.into(),
            exit: 2,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
    pub(crate) fn missing(message: impl Into<String>) -> Self {
        Self {
            code: "NOT_FOUND",
            message: message.into(),
            exit: 3,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
    pub(crate) fn conflict(message: impl Into<String>) -> Self {
        Self {
            code: "REVISION_CONFLICT",
            message: message.into(),
            exit: 4,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
    pub(crate) fn internal(message: impl Into<String>) -> Self {
        Self {
            code: "INTERNAL",
            message: message.into(),
            exit: 5,
            path: None,
            current_version: None,
            current_version_id: None,
            issues: Vec::new(),
        }
    }
}

