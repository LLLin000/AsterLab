#![recursion_limit = "256"]

//! Shared AsterLab application core.

#[path = "../../../src/cli.rs"]
mod cli_impl;

use std::path::Path;
use serde_json::{json, Value};
use rusqlite::Connection;

pub struct AppCore {
    db: Connection,
}

impl AppCore {
    pub fn open(data_dir: &Path) -> Result<AppCore, String> {
        let db = cli_impl::open_db(data_dir).map_err(|e| e.message)?;
        Ok(AppCore { db })
    }

    pub fn load_run(&self, run_id: &str) -> Result<Value, String> {
        let mut v = cli_impl::run_workspace(&self.db, run_id).map_err(|e| e.message)?;
        v["revision"] = json!(self.revision(run_id)?);
        Ok(v)
    }

    fn revision(&self, run_id: &str) -> Result<i64, String> {
        self.db.query_row(
            "SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1",
            rusqlite::params![run_id],
            |row| row.get(0),
        ).map_err(|e| e.to_string())
    }

    fn find_operation_event(&self, run_id: &str, op_id: &str) -> Option<Value> {
        let mut stmt = self.db.prepare(
            "SELECT data_json FROM records WHERE kind='execution_event' AND parent_id=?1",
        ).ok()?;
        let rows = stmt.query_map([run_id], |row| row.get::<_, String>(0)).ok()?;
        for row in rows.flatten() {
            if let Ok(v) = serde_json::from_str::<Value>(&row) {
                if v.get("operation_id").and_then(Value::as_str) == Some(op_id) {
                    return Some(v);
                }
            }
        }
        None
    }

    pub fn save_readout(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input.get("run_id").and_then(Value::as_str).map(str::to_owned)
            .ok_or_else(|| "run_id is required".to_owned())?;
        let op_id = input.get("operation_id").and_then(Value::as_str).map(str::to_owned);
        if let Some(op) = &op_id {
            if let Some(existing) = self.find_operation_event(&run_id, op) {
                let existing_payload = existing.get("operation_payload").cloned().unwrap_or(Value::Null);
                if existing_payload == input {
                    return self.load_run(&run_id);
                }
                return Err("operation id conflict: same operation_id with different payload".to_owned());
            }
        }
        let tx = self.db.transaction().map_err(|e| e.to_string())?;
        let mut event = input.clone();
        if event.get("event_type").is_none() {
            event["event_type"] = json!("measurement");
        }
        if event.get("content").is_none() {
            event["content"] = json!("结构化记录");
        }
        if let Some(op) = &op_id {
            event["operation_id"] = json!(op);
        }
        event["operation_payload"] = input.clone();
        cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e| e.message)?;
        tx.commit().map_err(|e| e.to_string())?;
        self.load_run(&run_id)
    }

    pub fn complete_task(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input.get("run_id").and_then(Value::as_str).map(str::to_owned)
            .ok_or_else(|| "run_id is required".to_owned())?;
        let op_id = input.get("operation_id").and_then(Value::as_str).map(str::to_owned);
        if let Some(op) = &op_id {
            if let Some(existing) = self.find_operation_event(&run_id, op) {
                let existing_payload = existing.get("operation_payload").cloned().unwrap_or(Value::Null);
                if existing_payload == input {
                    return self.load_run(&run_id);
                }
                return Err("operation id conflict: same operation_id with different payload".to_owned());
            }
        }
        let tx = self.db.transaction().map_err(|e| e.to_string())?;
        let mut event = input.clone();
        if event.get("event_type").is_none() {
            event["event_type"] = json!("operation");
        }
        if event.get("content").is_none() {
            event["content"] = json!("完成任务");
        }
        if event.get("step_status").is_none() {
            event["step_status"] = json!("completed");
        }
        if let Some(op) = &op_id {
            event["operation_id"] = json!(op);
        }
        event["operation_payload"] = input.clone();
        cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e| e.message)?;
        tx.commit().map_err(|e| e.to_string())?;
        self.load_run(&run_id)
    }

    pub fn list_designs(&self, project_id: Option<&str>) -> Result<Value, String> {
        cli_impl::list_designs(&self.db, project_id).map_err(|e| e.message)
    }

    pub fn load_design(&self, design_id: &str) -> Result<Value, String> {
        cli_impl::load_design(&self.db, design_id).map_err(|e| e.message)
    }

    pub fn save_design(&mut self, input: Value) -> Result<Value, String> {
        // create path
        if let (Some(project_id), Some(design)) = (
            input.get("project_id").and_then(Value::as_str),
            input.get("design"),
        ) {
            return cli_impl::create_design_record(&self.db, project_id, design).map_err(|e| e.message);
        }
        // revise path
        let id = input.get("id").or_else(|| input.get("design_id")).and_then(Value::as_str)
            .ok_or_else(|| "design id is required".to_owned())?;
        let expected = input.get("expected_revision").and_then(Value::as_i64)
            .ok_or_else(|| "expected_revision is required".to_owned())?;
        let changes = input.get("changes").cloned().unwrap_or_else(|| input.get("design").cloned().unwrap_or(Value::Null));
        cli_impl::revise_design_record(&self.db, id, expected, &changes).map_err(|e| e.message)
    }

    pub fn create_run(&mut self, design_id: &str, title: Option<&str>) -> Result<Value, String> {
        cli_impl::create_run_for_design(&self.db, design_id, title).map_err(|e| e.message)
    }
}


#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    #[test]
    fn idempotent_readout_and_rollback() {
        let d = std::env::temp_dir().join(format!("asterlab-app-test-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&d).unwrap();
        let mut core = AppCore::open(&d).unwrap();
        let proj = cli_impl::create_project(&core.db, "p").unwrap();
        let pid = proj["id"].as_str().unwrap();
        let design = core.save_design(serde_json::json!({"project_id": pid, "design": {"title": "d"}})).unwrap();
        let did = design["design"]["id"].as_str().unwrap();
        let run = core.create_run(did, None).unwrap();
        let rid = run["id"].as_str().unwrap();
        let rev0 = core.load_run(rid).unwrap()["revision"].as_i64().unwrap();

        let input = serde_json::json!({"run_id": rid, "operation_id": "op1", "measurements": [{"endpoint":"e","value":1.0}]});
        let r1 = core.save_readout(input.clone()).unwrap();
        let rev1 = r1["revision"].as_i64().unwrap();
        assert!(rev1 > rev0);

        // same op+payload returns existing, no new event
        let r2 = core.save_readout(input.clone()).unwrap();
        assert_eq!(r2["revision"].as_i64().unwrap(), rev1);

        // different payload conflict
        let bad = serde_json::json!({"run_id": rid, "operation_id": "op1", "measurements": [{"endpoint":"e","value":2.0}]});
        assert!(core.save_readout(bad).is_err());

        // invalid readout rolls back: event count unchanged
        let before = core.load_run(rid).unwrap()["revision"].as_i64().unwrap();
        let invalid = serde_json::json!({"run_id": rid, "measurements": [{"endpoint":"e","value": f64::NAN}]});
        assert!(core.save_readout(invalid).is_err());
        let after = core.load_run(rid).unwrap()["revision"].as_i64().unwrap();
        assert_eq!(before, after);

        let _ = fs::remove_dir_all(&d);
    }
}
