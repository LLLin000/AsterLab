//! Stable step paths for AsterDoc step modules.
//!
//! Contract: `docs/contracts/asterdoc-step-path-v1.md`.
//!
//! A step module may omit `path`, and the design canvas writes `path: ""` for
//! every step it creates. The workflow tree refuses to guess such a step, so
//! the writer must either derive a path or refuse the edit with a reason the
//! caller can act on. This module owns that decision: the derived path is
//! `root/<step-id>`, the same `root/...` addressing `step.edit` already uses,
//! and the only refusals are the ones where derivation cannot produce a
//! unique, acyclic path.

use std::collections::HashMap;

/// Root segment shared with the Protocol/step.edit addressing.
pub const ROOT_SEGMENT: &str = "root";

/// A refused step-path edit. `code` is the stable reason (the CLI still
/// reports `INVALID_INPUT`; the code separates the branches in tests and
/// telemetry), `message` names the step, the missing field and the fix.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StepPathError {
    pub code: &'static str,
    pub message: String,
}

impl StepPathError {
    fn new(code: &'static str, message: String) -> Self {
        Self { code, message }
    }
}

/// Derive `root/<step-id>` for a step that has no path.
pub fn derive_step_path(step_id: &str) -> Result<String, StepPathError> {
    let id = step_id.trim();
    if id.is_empty() || id.contains('/') {
        return Err(StepPathError::new(
            "step_path_derivation_failed",
            format!(
                "step path cannot be derived: a step id must be non-empty and must not contain \"/\" \
                 (the derived path is root/<step-id>); set path explicitly on this step instead of id \"{}\"",
                step_id.trim()
            ),
        ));
    }
    Ok(format!("{ROOT_SEGMENT}/{id}"))
}

/// Resolve a step path, deriving `root/<step-id>` when the stored path is
/// empty. A stored path is returned trimmed and otherwise unchanged.
pub fn resolve_step_path(step_id: &str, path: &str) -> Result<String, StepPathError> {
    if path.trim().is_empty() {
        derive_step_path(step_id)
    } else {
        Ok(path.trim().to_owned())
    }
}

/// Final path segment, or `None` for an empty path.
pub fn path_leaf(path: &str) -> Option<&str> {
    path.rsplit('/').find(|segment| !segment.is_empty())
}

/// The path a step takes when nested under `parent_path`.
pub fn nest_step_path(
    child_id: &str,
    child_path: &str,
    parent_id: &str,
    parent_path: &str,
) -> Result<String, StepPathError> {
    let child_path = resolve_step_path(child_id, child_path)?;
    let parent_path = resolve_step_path(parent_id, parent_path)?;
    let leaf = path_leaf(&child_path).ok_or_else(|| {
        StepPathError::new(
            "step_path_derivation_failed",
            format!(
                "step {child_id} has path \"{child_path}\" with no final segment; give it a path like root/<step-id> before nesting it"
            ),
        )
    })?;
    if parent_path == child_path || parent_path.starts_with(&format!("{child_path}/")) {
        return Err(StepPathError::new(
            "step_path_cycle",
            format!(
                "nest_step would put step {child_id} under its own descendant step {parent_id} \
                 (path {parent_path} is inside {child_path}); choose a parent outside {child_path}"
            ),
        ));
    }
    let new_path = format!("{parent_path}/{leaf}");
    if new_path == child_path {
        return Err(StepPathError::new(
            "step_path_already_nested",
            format!(
                "step {child_id} is already at {child_path} inside step {parent_id}; \
                 choose a different parentId or promote it first"
            ),
        ));
    }
    Ok(new_path)
}

/// The path a step takes when promoted one level out of its parent.
pub fn promote_step_path(step_id: &str, path: &str) -> Result<String, StepPathError> {
    let path = path.trim();
    if path.is_empty() {
        return Err(StepPathError::new(
            "step_path_derivation_failed",
            format!(
                "step {step_id} has no path; promote_step needs a nested path like root/<parent>/<step> — \
                 set path on step {step_id} or nest it under a parent first"
            ),
        ));
    }
    let leaf = path_leaf(path).ok_or_else(|| {
        StepPathError::new(
            "step_path_derivation_failed",
            format!(
                "step {step_id} has path \"{path}\" with no final segment; set a path like root/<step-id> first"
            ),
        )
    })?;
    let parent_path = path.rsplit_once('/').map(|(parent, _)| parent).unwrap_or("");
    let root = parent_path.rsplit_once('/').map(|(parent, _)| parent).unwrap_or(parent_path);
    let new_path = if root.is_empty() {
        leaf.to_owned()
    } else {
        format!("{root}/{leaf}")
    };
    if new_path == path {
        return Err(StepPathError::new(
            "step_path_already_top_level",
            format!(
                "step {step_id} is already at the top level (path {path}); promote_step needs a nested step — \
                 nest it under a parent step first"
            ),
        ));
    }
    Ok(new_path)
}

/// First duplicated non-empty step path, if any.
pub fn find_step_path_conflict(
    steps: &[(String, String)],
) -> Option<StepPathError> {
    let mut seen: HashMap<&str, &str> = HashMap::new();
    for (id, path) in steps {
        let path = path.trim();
        if path.is_empty() {
            continue;
        }
        if let Some(previous) = seen.get(path) {
            return Some(StepPathError::new(
                "step_path_conflict",
                format!(
                    "steps {previous} and {id} would both use path {path}; step paths must stay unique — \
                     give one of them a different path (for example path: {path}-2) and retry"
                ),
            ));
        }
        seen.insert(path, id);
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn derives_root_path_from_stable_step_id() {
        assert_eq!(derive_step_path("05-imaging-6h").unwrap(), "root/05-imaging-6h");
        assert_eq!(derive_step_path("  seed  ").unwrap(), "root/seed");
    }

    #[test]
    fn rejects_ids_that_cannot_be_a_path_segment() {
        for id in ["", "   ", "root/child"] {
            let error = derive_step_path(id).unwrap_err();
            assert_eq!(error.code, "step_path_derivation_failed");
            assert!(error.message.contains("path"), "{}", error.message);
            assert!(error.message.contains("step id"), "{}", error.message);
        }
    }

    #[test]
    fn resolve_keeps_a_stored_path_and_derives_an_empty_one() {
        assert_eq!(resolve_step_path("seed", "root/prep/seed").unwrap(), "root/prep/seed");
        assert_eq!(resolve_step_path("seed", "  huvec/04-seed ").unwrap(), "huvec/04-seed");
        assert_eq!(resolve_step_path("seed", "").unwrap(), "root/seed");
        assert_eq!(resolve_step_path("seed", "   ").unwrap(), "root/seed");
    }

    #[test]
    fn nests_pathless_child_under_pathless_parent() {
        assert_eq!(
            nest_step_path("04-seed", "", "03-soak", "").unwrap(),
            "root/03-soak/04-seed"
        );
    }

    #[test]
    fn nests_a_child_one_level_below_its_new_parent() {
        assert_eq!(
            nest_step_path("child", "root/child", "parent", "root/parent").unwrap(),
            "root/parent/child"
        );
        assert_eq!(
            nest_step_path("child", "huvec/child", "parent", "huvec/prep").unwrap(),
            "huvec/prep/child"
        );
    }

    #[test]
    fn rejects_nesting_a_step_into_its_own_descendant() {
        let error = nest_step_path("parent", "root/parent", "child", "root/parent/child")
            .unwrap_err();
        assert_eq!(error.code, "step_path_cycle");
        assert!(error.message.contains("root/parent/child"), "{}", error.message);
    }

    #[test]
    fn rejects_nesting_a_step_already_below_that_parent() {
        let error =
            nest_step_path("child", "root/parent/child", "parent", "root/parent").unwrap_err();
        assert_eq!(error.code, "step_path_already_nested");
        assert!(error.message.contains("root/parent/child"), "{}", error.message);
        assert!(error.message.contains("promote"), "{}", error.message);
    }

    #[test]
    fn promotes_one_level_up() {
        assert_eq!(
            promote_step_path("child", "root/parent/child").unwrap(),
            "root/child"
        );
        assert_eq!(
            promote_step_path("child", "huvec/prep/child").unwrap(),
            "huvec/child"
        );
    }

    #[test]
    fn rejects_promoting_a_top_level_step() {
        let error = promote_step_path("child", "root/child").unwrap_err();
        assert_eq!(error.code, "step_path_already_top_level");
        assert!(error.message.contains("root/child"), "{}", error.message);
        assert!(error.message.contains("nest"), "{}", error.message);
    }

    #[test]
    fn names_both_steps_when_two_paths_collide() {
        let steps = vec![
            ("parent".to_owned(), "root/parent".to_owned()),
            ("child".to_owned(), "root/parent".to_owned()),
            ("other".to_owned(), String::new()),
        ];
        let error = find_step_path_conflict(&steps).unwrap();
        assert_eq!(error.code, "step_path_conflict");
        assert!(error.message.contains("parent"), "{}", error.message);
        assert!(error.message.contains("child"), "{}", error.message);
        assert!(error.message.contains("root/parent"), "{}", error.message);
        assert!(error.message.contains("path"), "{}", error.message);
    }

    #[test]
    fn ignores_missing_paths_and_unique_paths() {
        let steps = vec![
            ("a".to_owned(), "root/a".to_owned()),
            ("b".to_owned(), String::new()),
            ("c".to_owned(), "root/c".to_owned()),
        ];
        assert!(find_step_path_conflict(&steps).is_none());
    }
}
