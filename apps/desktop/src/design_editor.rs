//! Native design-editor presenter and CLI command adapter.
//!
//! The UI never writes SQLite directly. The adapter invokes the same Core CLI
//! command surface used by agents until a shared in-process application service
//! is available.

use asterlab_core::design_commands::{
    validate_draft, DesignCommandError, DesignDraft, DesignSaved,
};
use serde_json::{json, Value};
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EditorState {
    NewDraft,
    EditingDraft,
    Dirty,
    Validating,
    Conflict,
    Saved,
    Failed,
}

#[derive(Debug, Clone)]
pub struct DesignEditorPresenter {
    pub state: EditorState,
    pub project_id: String,
    pub design_id: Option<String>,
    pub expected_revision: Option<i64>,
    pub draft: DesignDraft,
    pub saved: Option<DesignSaved>,
    pub error: Option<DesignCommandError>,
}

impl DesignEditorPresenter {
    pub fn new(project_id: impl Into<String>, draft: DesignDraft) -> Self {
        Self {
            state: EditorState::NewDraft,
            project_id: project_id.into(),
            design_id: None,
            expected_revision: None,
            draft,
            saved: None,
            error: None,
        }
    }

    pub fn edit(
        project_id: impl Into<String>,
        design_id: impl Into<String>,
        expected_revision: i64,
        draft: DesignDraft,
    ) -> Self {
        Self {
            state: EditorState::EditingDraft,
            project_id: project_id.into(),
            design_id: Some(design_id.into()),
            expected_revision: Some(expected_revision),
            draft,
            saved: None,
            error: None,
        }
    }

    pub fn mark_dirty(&mut self) {
        if matches!(
            self.state,
            EditorState::NewDraft | EditorState::EditingDraft | EditorState::Saved
        ) {
            self.state = EditorState::Dirty;
        }
    }

    pub fn mark_conflict(&mut self, error: DesignCommandError) {
        self.state = EditorState::Conflict;
        self.error = Some(error);
    }

    pub fn save(&mut self, writer: &dyn DesignWriter) -> Result<DesignSaved, DesignCommandError> {
        if let Err(error) = validate_draft(&self.draft) {
            self.state = EditorState::Failed;
            self.error = Some(error.clone());
            return Err(error);
        }
        self.state = EditorState::Validating;
        let result = match (&self.design_id, self.expected_revision) {
            (Some(design_id), Some(expected_revision)) => {
                writer.revise(design_id, expected_revision, &self.draft)
            }
            _ => writer.create(&self.project_id, &self.draft),
        };
        match result {
            Ok(saved) => {
                self.design_id = Some(saved.design_id.clone());
                self.expected_revision = Some(saved.revision);
                self.saved = Some(saved.clone());
                self.state = EditorState::Saved;
                self.error = None;
                Ok(saved)
            }
            Err(error) => {
                if error.code == "REVISION_CONFLICT" {
                    self.state = EditorState::Conflict;
                } else {
                    self.state = EditorState::Failed;
                }
                self.error = Some(error.clone());
                Err(error)
            }
        }
    }
}

pub trait DesignWriter {
    fn create(
        &self,
        project_id: &str,
        draft: &DesignDraft,
    ) -> Result<DesignSaved, DesignCommandError>;
    fn revise(
        &self,
        design_id: &str,
        expected_revision: i64,
        draft: &DesignDraft,
    ) -> Result<DesignSaved, DesignCommandError>;
}

#[derive(Debug, Clone)]
pub struct CliDesignWriter {
    pub executable: PathBuf,
}

impl CliDesignWriter {
    pub fn from_env() -> Result<Self, DesignCommandError> {
        let executable = std::env::var_os("LABWORKBENCH_CLI")
            .map(PathBuf::from)
            .or_else(|| {
                let candidate = PathBuf::from("target/release/labworkbench.exe");
                candidate.exists().then_some(candidate)
            })
            .ok_or_else(|| DesignCommandError {
                code: "CLI_NOT_FOUND".into(),
                path: None,
                message: "找不到 labworkbench CLI；请设置 LABWORKBENCH_CLI".into(),
            })?;
        Ok(Self { executable })
    }

    fn run(&self, tool: &str, input: &Value) -> Result<Value, DesignCommandError> {
        let stamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_nanos())
            .unwrap_or_default();
        let path =
            std::env::temp_dir().join(format!("asterlab-{}-{stamp}.json", std::process::id()));
        fs::write(
            &path,
            serde_json::to_vec(input).map_err(|error| DesignCommandError {
                code: "INTERNAL".into(),
                path: None,
                message: error.to_string(),
            })?,
        )
        .map_err(|error| DesignCommandError {
            code: "INTERNAL".into(),
            path: None,
            message: error.to_string(),
        })?;
        let output = Command::new(&self.executable)
            .arg(tool)
            .arg("--input")
            .arg(&path)
            .output();
        let _ = fs::remove_file(&path);
        let output = output.map_err(|error| DesignCommandError {
            code: "CLI_FAILED".into(),
            path: None,
            message: error.to_string(),
        })?;
        let envelope: Value =
            serde_json::from_slice(&output.stdout).map_err(|error| DesignCommandError {
                code: "CLI_FAILED".into(),
                path: None,
                message: format!("CLI 输出不是 JSON：{error}"),
            })?;
        if envelope["ok"].as_bool() != Some(true) {
            return Err(DesignCommandError {
                code: envelope["error"]["code"]
                    .as_str()
                    .unwrap_or("CLI_FAILED")
                    .to_owned(),
                path: None,
                message: envelope["error"]["message"]
                    .as_str()
                    .unwrap_or("CLI 命令失败")
                    .to_owned(),
            });
        }
        Ok(envelope["result"].clone())
    }
}

impl DesignWriter for CliDesignWriter {
    fn create(
        &self,
        project_id: &str,
        draft: &DesignDraft,
    ) -> Result<DesignSaved, DesignCommandError> {
        let result = self.run(
            "design.create",
            &json!({"project_id": project_id, "design": draft.to_design_json()}),
        )?;
        Ok(DesignSaved {
            design_id: result["design"]["id"]
                .as_str()
                .unwrap_or_default()
                .to_owned(),
            revision: result["design"]["revision"].as_i64().unwrap_or(1),
            design_version_id: result["version"]["id"]
                .as_str()
                .unwrap_or_default()
                .to_owned(),
        })
    }

    fn revise(
        &self,
        design_id: &str,
        expected_revision: i64,
        draft: &DesignDraft,
    ) -> Result<DesignSaved, DesignCommandError> {
        let result = self.run(
            "design.revise",
            &json!({
                "id": design_id,
                "expected_revision": expected_revision,
                "changes": draft.to_design_json()
            }),
        )?;
        Ok(DesignSaved {
            design_id: design_id.to_owned(),
            revision: result["record"]["revision"]
                .as_i64()
                .unwrap_or(expected_revision + 1),
            design_version_id: result["version"]["id"]
                .as_str()
                .unwrap_or_default()
                .to_owned(),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::cell::RefCell;

    struct FakeWriter {
        calls: RefCell<Vec<String>>,
    }

    impl DesignWriter for FakeWriter {
        fn create(
            &self,
            _project_id: &str,
            draft: &DesignDraft,
        ) -> Result<DesignSaved, DesignCommandError> {
            self.calls.borrow_mut().push("create".into());
            let _ = draft;
            Ok(DesignSaved {
                design_id: "d1".into(),
                revision: 1,
                design_version_id: "v1".into(),
            })
        }
        fn revise(
            &self,
            _design_id: &str,
            expected: i64,
            _draft: &DesignDraft,
        ) -> Result<DesignSaved, DesignCommandError> {
            self.calls.borrow_mut().push(format!("revise:{expected}"));
            Ok(DesignSaved {
                design_id: "d1".into(),
                revision: expected + 1,
                design_version_id: "v2".into(),
            })
        }
    }

    fn draft() -> DesignDraft {
        DesignDraft {
            title: "D".into(),
            objective: "O".into(),
            experimental_unit: None,
            groups: vec![],
            assays: vec![],
        }
    }

    #[test]
    fn save_creates_then_revises() {
        let writer = FakeWriter {
            calls: RefCell::new(vec![]),
        };
        let mut presenter = DesignEditorPresenter::new("p1", draft());
        presenter.mark_dirty();
        let first = presenter.save(&writer).unwrap();
        assert_eq!(presenter.state, EditorState::Saved);
        assert_eq!(first.revision, 1);
        presenter.mark_dirty();
        let second = presenter.save(&writer).unwrap();
        assert_eq!(second.revision, 2);
        assert_eq!(writer.calls.borrow().as_slice(), ["create", "revise:1"]);
    }

    #[test]
    fn invalid_draft_does_not_call_writer() {
        let writer = FakeWriter {
            calls: RefCell::new(vec![]),
        };
        let mut presenter = DesignEditorPresenter::new(
            "p1",
            DesignDraft {
                title: "".into(),
                objective: "".into(),
                experimental_unit: None,
                groups: vec![],
                assays: vec![],
            },
        );
        assert!(presenter.save(&writer).is_err());
        assert!(writer.calls.borrow().is_empty());
        assert_eq!(presenter.state, EditorState::Failed);
    }
}
