#![recursion_limit = "256"]

//! Shared AsterLab application core.

#[path = "../../../src/cli.rs"]
mod cli_impl;

mod agent_edit_preview;

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

    pub fn raw_db(&self) -> &Connection {
        &self.db
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

    fn find_operation_event_tx(tx: &rusqlite::Transaction<'_>, run_id: &str, op_id: &str) -> Option<Value> {
        let mut stmt = tx.prepare(
            "SELECT id,data_json FROM records WHERE kind='execution_event' AND parent_id=?1",
        ).ok()?;
        let rows = stmt.query_map([run_id], |row| Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?))).ok()?;
        for row in rows.flatten() {
            if let Ok(mut v) = serde_json::from_str::<Value>(&row.1) {
                if v.get("operation_id").and_then(Value::as_str) == Some(op_id) {
                    v["id"] = json!(row.0);
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
        let tx = self.db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).map_err(|e| e.to_string())?;
        if let Some(op) = &op_id {
            if let Some(existing) = Self::find_operation_event_tx(&tx, &run_id, op) {
                let existing_payload = existing.get("operation_payload").cloned().unwrap_or(Value::Null);
                if existing_payload == input {
                    tx.commit().map_err(|e| e.to_string())?;
                    return self.load_run(&run_id);
                }
                return Err("operation id conflict: same operation_id with different payload".to_owned());
            }
        }
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

    pub fn save_readouts_batch(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input["run_id"].as_str().ok_or("run_id is required")?.to_owned();
        let operation_id = input["operation_id"].as_str().filter(|s| !s.is_empty()).ok_or("operation_id is required")?.to_owned();
        let cells = input["cells"].as_array().ok_or("cells is required")?;
        let schema = input["schema_id"].as_str().filter(|s| !s.is_empty()).ok_or("schema_id is required")?;
        if !matches!(schema, "cck8.plate96.od450.v1" | "qpcr.sample_gene.cq.v1") || input["schema_version"].as_u64() != Some(1) {
            return Err("unsupported result schema".into());
        }
        if cells.is_empty() || cells.len() > 100 { return Err("batch must contain 1..=100 cells".into()); }
        let tx = self.db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).map_err(|e| e.to_string())?;
        eprintln!("run.save_batch layer=core transaction=start run_id={} operation_id={} schema_id={} cell_count={}", run_id, operation_id, schema, cells.len());
        if let Some(existing) = Self::find_operation_event_tx(&tx, &run_id, &operation_id) {
            if existing.get("operation_payload") == Some(&input) {
                let event_id = existing["id"].as_str().unwrap_or_default().to_owned();
                let revision = tx.query_row("SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1", [&run_id], |r| r.get::<_, i64>(0)).map_err(|e|e.to_string())? as u64;
                tx.commit().map_err(|e|e.to_string())?;
                return Ok(json!({"run_id":run_id,"revision":revision,"batch_id":existing["batch_id"],"event_id":event_id,"accepted":cells.len(),"rejected":[]}));
            }
            return Err("operation id conflict: same operation_id with different payload".into());
        }
        let revision = tx.query_row("SELECT COUNT(*) FROM records WHERE kind='execution_event' AND parent_id=?1", [&run_id], |r| r.get::<_, i64>(0)).map_err(|e|e.to_string())? as u64;
        if let Some(expected) = input["expected_revision"].as_u64() {
            if expected != revision { return Err(format!("stale revision: expected {expected}, current {revision}")); }
        }
        let mut seen = std::collections::HashSet::new();
        let mut measurements = Vec::with_capacity(cells.len());
        let mut readouts = Vec::with_capacity(cells.len());
        for (index, cell) in cells.iter().enumerate() {
            let row = cell["row"].as_u64().ok_or_else(||format!("cell {index}: invalid row"))?;
            let column = cell["column"].as_u64().ok_or_else(||format!("cell {index}: invalid column"))?;
            if (schema == "cck8.plate96.od450.v1" && (row > 7 || column > 11)) || !seen.insert((row,column)) {
                return Err(format!("cell {index}: out of range or duplicate position"));
            }
            let field = cell["field"].as_str().filter(|s| !s.is_empty()).ok_or_else(||format!("cell {index}: field is required"))?;
            let status = cell["status"].as_str().unwrap_or("");
            let value = cell["value"].as_f64();
            if !matches!(status, "raw" | "missing") || (status == "raw" && value.is_none()) || (status == "missing" && value.is_some()) {
                return Err(format!("cell {index}: invalid status/value"));
            }
            if status == "raw" {
                let sample_id = cell["sample_id"].as_str().filter(|id|!id.trim().is_empty()).map(Value::from).unwrap_or(Value::Null);
                measurements.push(json!({"endpoint":field,"value":value.unwrap(),"unit":"","sample_id":sample_id,"row":row,"column":column,"group":cell["group"],"replicate":cell["replicate"],"status":status,"field":field}));
                let readout_id = format!("readout-{operation_id}-{row}-{column}-{field}");
                readouts.push(json!({"readout_id":readout_id,"endpoint":field,"value":value.unwrap(),"unit":"","sample_id":sample_id,"result_kind":"raw"}));
            }
        }
        let batch_id = format!("batch-{operation_id}");
        let event = json!({"run_id":run_id,"event_type":"measurement","content":"Structured readout batch","task_key":input["task_key"],"assay_id":input["assay_id"],"timepoint":input["timepoint"],"protocol_step_path":input["protocol_step_path"],"protocol_version_id":input["protocol_version_id"],"measurements":measurements,"result_cells":cells,"readouts":readouts,"operation_id":operation_id,"operation_payload":input,"batch_id":batch_id,"schema_id":schema,"schema_version":1});
        cli_impl::create_execution_event_tx(&tx, &run_id, &event).map_err(|e|e.message)?;
        let event_id: String = tx.query_row("SELECT id FROM records WHERE kind='execution_event' AND parent_id=?1 ORDER BY rowid DESC LIMIT 1", [&run_id], |r|r.get(0)).map_err(|e|e.to_string())?;
        let revision = revision + 1;
        tx.commit().map_err(|e|e.to_string())?;
        eprintln!("run.save_batch layer=core transaction=commit run_id={} operation_id={} batch_id={} event_id={} cell_count={}", run_id, operation_id, batch_id, event_id, cells.len());
        Ok(json!({"run_id":run_id,"revision":revision,"batch_id":batch_id,"event_id":event_id,"accepted":cells.len(),"rejected":[]}))
    }

    pub fn complete_task(&mut self, input: Value) -> Result<Value, String> {
        let run_id = input.get("run_id").and_then(Value::as_str).map(str::to_owned)
            .ok_or_else(|| "run_id is required".to_owned())?;
        let op_id = input.get("operation_id").and_then(Value::as_str).map(str::to_owned);
        let tx = self.db.transaction_with_behavior(rusqlite::TransactionBehavior::Immediate).map_err(|e| e.to_string())?;
        if let Some(op) = &op_id {
            if let Some(existing) = Self::find_operation_event_tx(&tx, &run_id, op) {
                let existing_payload = existing.get("operation_payload").cloned().unwrap_or(Value::Null);
                if existing_payload == input {
                    tx.commit().map_err(|e| e.to_string())?;
                    return self.load_run(&run_id);
                }
                return Err("operation id conflict: same operation_id with different payload".to_owned());
            }
        }
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

    pub fn apply_asterdoc_command(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::apply_asterdoc_command(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn create_asterdoc_design(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::create_asterdoc_design(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn save_asterdoc(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::save_asterdoc_version(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn load_asterdoc(&self, design_id: &str) -> Result<Value, String> {
        cli_impl::load_asterdoc(&self.db, design_id).map_err(|e| e.message)
    }

    pub fn load_asterdoc_at(&self, design_id: &str, revision: u64) -> Result<Value, String> {
        cli_impl::load_asterdoc_at(&self.db, design_id, revision).map_err(|e| e.message)
    }

    pub fn assess_asterdoc(&self, input: Value) -> Result<Value, String> {
        cli_impl::assess_asterdoc(&self.db, &input).map_err(|e| e.message)
    }

    pub fn create_run_from_asterdoc(&mut self, design_id: &str, revision: u64, operation_id: Option<&str>, title: Option<&str>, acknowledged: &[String]) -> Result<Value, String> {
        cli_impl::create_run_from_asterdoc(&mut self.db, design_id, revision, operation_id, title, acknowledged)
            .map_err(|e| e.message)
    }

    pub fn compile_asterdoc(&self, design_id: &str, revision: u64) -> Result<Value, String> {
        let doc = cli_impl::load_asterdoc_at(&self.db, design_id, revision).map_err(|e| e.message)?;
        let source = doc["source"].as_str().unwrap_or("");
        let design = cli_impl::compile_asterdoc_to_design(source).map_err(|e| e.message)?;
        Ok(json!({"design": design, "document": doc}))
    }

    pub fn create_run(&mut self, design_id: &str, title: Option<&str>, operation_id: Option<&str>) -> Result<Value, String> {
        cli_impl::create_run_for_design(&mut self.db, design_id, title, operation_id).map_err(|e| e.message)
    }

    pub fn save_asset_link(&mut self, input: Value) -> Result<Value, String> {
        cli_impl::save_asset_link(&mut self.db, &input).map_err(|e| e.message)
    }

    pub fn list_asset_links(&self, run_id: &str) -> Result<Value, String> {
        cli_impl::list_asset_links(&self.db, run_id).map_err(|e| e.message)
    }

    /// Read-only `agent_edit.validate`: resolve a proposal against the current
    /// head and return the preview. Writes nothing; see
    /// `docs/contracts/agent-edit-v1.md`.
    pub fn agent_edit_validate(&self, proposal: Value) -> Result<Value, String> {
        agent_edit_preview::validate_proposal_value(&self.db, proposal)
    }

    /// Digest over every record row; evidence helper for read-only commands.
    pub fn records_digest(&self) -> Result<String, String> {
        agent_edit_preview::records_digest(&self.db)
    }
}


#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    fn fixture() -> (std::path::PathBuf, String) {
        let d = std::env::temp_dir().join(format!("asterlab-batch-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&d).unwrap();
        let mut core = AppCore::open(&d).unwrap();
        let project = cli_impl::create_project(&core.db, "p").unwrap();
        let design = core.save_design(json!({"project_id":project["id"],"design":{"title":"d"}})).unwrap();
        let run = core.create_run(design["design"]["id"].as_str().unwrap(), None, None).unwrap();
        (d, run["id"].as_str().unwrap().to_owned())
    }

    fn batch(run_id: &str, op: &str, expected_revision: u64) -> Value {
        let cells: Vec<Value> = (0..8).flat_map(|r| (0..12).map(move |c| json!({
            "row":r,"column":c,"sample_id":"","group":"control","replicate":1,
            "field":"od450","value":(r*12+c) as f64,"status":"raw"
        }))).collect();
        json!({"run_id":run_id,"operation_id":op,"schema_id":"cck8.plate96.od450.v1",
            "schema_version":1,"expected_revision":expected_revision,"cells":cells})
    }
    #[test]
    fn batch_is_atomic_idempotent_and_preserves_missing_cells() {
        let (dir, run_id) = fixture();
        let mut core = AppCore::open(&dir).unwrap();
        let input = batch(&run_id, "batch-op", 0);
        let result = core.save_readouts_batch(input.clone()).unwrap();
        assert_eq!(result["accepted"], 96);
        let revision = result["revision"].as_u64().unwrap();
        let event_id = result["event_id"].as_str().unwrap().to_owned();
        assert_eq!(core.save_readouts_batch(input.clone()).unwrap()["event_id"], event_id);
        let first = core.load_run(&run_id).unwrap();
        assert_eq!(first["revision"], revision);
        let first_event = first["events"].as_array().unwrap().iter().find(|e| e["event_id"] == event_id).unwrap();
        assert_eq!(first_event["readouts"].as_array().unwrap().len(), 96);
        assert!(first_event["readouts"][0]["readout_id"].as_str().is_some_and(|id| !id.is_empty()));
        let mut changed = input.clone();
        changed["cells"][0]["value"] = json!(999.0);
        assert!(core.save_readouts_batch(changed).unwrap_err().contains("conflict"));
        let mut invalid = batch(&run_id, "bad-op", revision);
        invalid["cells"][95]["value"] = Value::Null;
        assert!(core.save_readouts_batch(invalid).is_err());
        assert_eq!(core.load_run(&run_id).unwrap()["revision"], revision);
        let mut missing = batch(&run_id, "missing-op", revision);
        missing["cells"][0]["value"] = Value::Null;
        missing["cells"][0]["status"] = json!("missing");
        let missing_result = core.save_readouts_batch(missing).unwrap();
        let snapshot = core.load_run(&run_id).unwrap();
        let event = snapshot["events"].as_array().unwrap().iter().find(|e|e["event_id"] == missing_result["event_id"]).unwrap();
        assert_eq!(event["result_cells"][0]["status"], "missing");
        let _ = fs::remove_dir_all(dir);
    }

    #[test]
    fn concurrent_batch_writers_use_authoritative_revision() {
        let (dir, run_id) = fixture();
        let mut threads = Vec::new();
        for n in 0..2 {
            let dir = dir.clone();
            let run_id = run_id.clone();
            threads.push(std::thread::spawn(move || {
                let mut core = AppCore::open(&dir).unwrap();
                core.save_readouts_batch(batch(&run_id, &format!("parallel-{n}"), 0))
            }));
        }
        let results: Vec<_> = threads.into_iter().map(|t|t.join().unwrap()).collect();
        assert_eq!(results.iter().filter(|r|r.is_ok()).count(), 1);
        assert_eq!(results.iter().filter(|r|r.as_ref().is_err_and(|e|e.contains("stale revision"))).count(), 1);
        let core = AppCore::open(&dir).unwrap();
        assert_eq!(core.load_run(&run_id).unwrap()["revision"], 1);
        let _ = fs::remove_dir_all(dir);
    }
    #[test]
    fn idempotent_readout_and_rollback() {
        let d = std::env::temp_dir().join(format!("asterlab-app-test-{}", uuid::Uuid::new_v4()));
        fs::create_dir_all(&d).unwrap();
        let mut core = AppCore::open(&d).unwrap();
        let proj = cli_impl::create_project(&core.db, "p").unwrap();
        let pid = proj["id"].as_str().unwrap();
        let design = core.save_design(serde_json::json!({"project_id": pid, "design": {"title": "d"}})).unwrap();
        let did = design["design"]["id"].as_str().unwrap();
        let run = core.create_run(did, None, None).unwrap();
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
    #[test]
    fn readout_persists_across_reopen() {
        let d = std::env::temp_dir().join(format!("asterlab-reopen-{}", uuid::Uuid::new_v4()));
        std::fs::create_dir_all(&d).unwrap();
        std::env::set_var("LABWORKBENCH_DATA_DIR", &d);
        let resolved = cli_impl::data_dir().unwrap();
        assert_eq!(resolved, d);
        let rid = {
            let mut core = AppCore::open(&resolved).unwrap();
            assert_eq!(core.db.query_row("PRAGMA journal_mode", [], |row| row.get::<_, String>(0)).unwrap(), "wal");
            let proj = cli_impl::create_project(&core.db, "p").unwrap();
            let pid = proj["id"].as_str().unwrap();
            let design = core.save_design(serde_json::json!({"project_id": pid, "design": {"title": "d", "mygo": {"workflow": {"id":"wf"}, "instances": {}}}})).unwrap();
            let did = design["design"]["id"].as_str().unwrap();
            assert_eq!(core.load_design(did).unwrap()["design"]["data"]["mygo"]["workflow"]["id"], "wf");
            let run = core.create_run(did, None, None).unwrap();
            let rid = run["id"].as_str().unwrap().to_owned();
            let input = serde_json::json!({"run_id": rid, "operation_id": "op1", "measurements": [{"endpoint":"e","value":1.0}]});
            core.save_readout(input).unwrap();
            rid
        };
        let core2 = AppCore::open(&cli_impl::data_dir().unwrap()).unwrap();
        let v = core2.load_run(&rid).unwrap();
        assert!(v["revision"].as_i64().unwrap() > 0);
        let _ = std::fs::remove_dir_all(&d);
        std::env::remove_var("LABWORKBENCH_DATA_DIR");
    }
}
