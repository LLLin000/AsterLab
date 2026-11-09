#![recursion_limit = "256"]
mod mygo;
use mygo::*;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

#[path = "../../../../../../src/cli.rs"]
mod cli_impl;

use asterlab_app::AppCore;

pub fn make_run_service() -> Result<Arc<dyn RunService>, Error> {
    let dir = cli_impl::data_dir().map_err(map_cli)?;
    let core = AppCore::open(&dir).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
    Ok(Arc::new(CoreRunService { core: Mutex::new(Some(core)) }))
}

pub fn make_design_service() -> Result<Arc<dyn DesignService>, Error> {
    let dir = cli_impl::data_dir().map_err(map_cli)?;
    let db = cli_impl::open_db(&dir).map_err(|e| map_cli(e))?;
    Ok(Arc::new(CoreDesignService { db: Mutex::new(Some(db)) }))
}

struct CoreRunService { core: Mutex<Option<AppCore>> }

impl RunService for CoreRunService {
    fn load_run(&self, _ctx: &Context, run_id: String) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let v = core.load_run(&run_id).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_readouts_batch(&self, _ctx: &Context, input: BatchReadoutInput) -> Result<BatchReadoutResult, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let cells: Vec<serde_json::Value> = input.cells.iter().map(|c| serde_json::json!({
            "row":c.row,"column":c.column,"sample_id":c.sample_id,"group":c.group,
            "replicate":c.replicate,"field":c.field,"value":c.value,"status":c.status
        })).collect();
        let request = serde_json::json!({
            "run_id":input.run_id,"operation_id":input.operation_id,"task_key":input.task_key,
            "assay_id":input.assay_id,"timepoint":input.timepoint,
            "protocol_step_path":input.protocol_step_path,"protocol_version_id":input.protocol_version_id,
            "schema_id":input.schema_id,"schema_version":input.schema_version,
            "plate_id":input.plate_id,"expected_revision":input.expected_revision,"cells":cells
        });
        let result = core.save_readouts_batch(request).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(BatchReadoutResult {
            run_id: result["run_id"].as_str().unwrap_or_default().to_owned(),
            revision: result["revision"].as_u64().unwrap_or_default(),
            batch_id: result["batch_id"].as_str().unwrap_or_default().to_owned(),
            event_id: result["event_id"].as_str().unwrap_or_default().to_owned(),
            accepted: result["accepted"].as_i64().unwrap_or_default(),
            rejected: vec![],
        })
    }

    fn save_readout(&self, _ctx: &Context, input: ReadoutInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let run_id = input.run_id.clone();
        let measurements: Vec<serde_json::Value> = input.measurements.iter().map(|m| serde_json::json!({
            "endpoint": m.endpoint, "value": m.value, "unit": m.unit, "sample_id": m.sample_id
        })).collect();
        let mut event = serde_json::json!({
            "run_id": run_id,
            "event_type": "measurement",
            "content": "结构化记录",
            "measurements": measurements
        });
        if !input.task_key.is_empty() { event["task_key"] = serde_json::json!(input.task_key); }
        if !input.timepoint.is_empty() { event["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.assay_id.is_empty() { event["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.protocol_step_path.is_empty() { event["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { event["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if !input.operation_id.is_empty() { event["operation_id"] = serde_json::json!(input.operation_id); }
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn complete_task(&self, _ctx: &Context, input: CompleteTaskInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "operation",
            "content": "完成任务",
            "task_key": input.task_key,
            "step_status": "completed",
            "operation_id": input.operation_id,
            "assay_id": input.assay_id,
            "timepoint": input.timepoint,
            "protocol_step_path": input.protocol_step_path,
            "protocol_version_id": input.protocol_version_id
        });
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn close(&self) -> Result<(), Error> {
        if let Ok(mut g) = self.core.lock() { g.take(); }
        Ok(())
    }
}

struct CoreDesignService { db: Mutex<Option<rusqlite::Connection>> }

impl DesignService for CoreDesignService {
    fn list_designs(&self, _ctx: &Context) -> Result<Vec<DesignSummary>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut stmt = db.prepare("SELECT id,revision,data_json FROM records WHERE kind='design' ORDER BY created_at DESC").map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |r| Ok((r.get::<_,String>(0)?, r.get::<_,i64>(1)?, r.get::<_,String>(2)?))).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows {
            let (id, rev, raw) = row.map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
            let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            out.push(DesignSummary { design_id: id, title: data["title"].as_str().unwrap_or("").into(), revision: rev as u64 });
        }
        Ok(out)
    }

    fn load_design(&self, _ctx: &Context, design_id: String) -> Result<DesignSnapshot, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let row = db.query_row("SELECT kind,revision,data_json FROM records WHERE id=?1", rusqlite::params![design_id], |r| Ok((r.get::<_,String>(0)?, r.get::<_,i64>(1)?, r.get::<_,String>(2)?)));
        let (kind, revision, raw) = match row { Ok(x) => x, Err(_) => return Err(Error::Go("NOT_FOUND: design not found".into())) };
        if kind != "design" { return Err(Error::Go("INVALID: not a design".into())); }
        let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
        let title = data["title"].as_str().unwrap_or("").to_string();
        Ok(DesignSnapshot { design_id: design_id.clone(), revision: revision as u64, title, data: data.to_string() })
    }

    fn save_design(&self, _ctx: &Context, input: DesignSaveInput) -> Result<DesignSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let row = db.query_row("SELECT kind,revision,data_json,project_id FROM records WHERE id=?1", rusqlite::params![input.design_id], |r| Ok((r.get::<_,String>(0)?, r.get::<_,i64>(1)?, r.get::<_,String>(2)?, r.get::<_,Option<String>>(3)?)));
        let (kind, revision, raw, project) = match row { Ok(x) => x, Err(_) => return Err(Error::Go("NOT_FOUND: design not found".into())) };
        if kind != "design" { return Err(Error::Go("INVALID: not a design".into())); }
        if revision != input.expected_revision as i64 { return Err(Error::Go(format!("CONFLICT: design revision {} != {}", revision, input.expected_revision))); }
        let mut data = serde_json::from_str::<serde_json::Value>(&raw).unwrap_or(serde_json::json!({}));
        let changes: serde_json::Value = serde_json::from_str(&input.changes).unwrap_or(serde_json::json!({}));
        if let Some(obj) = changes.as_object() {
            for (k, v) in obj {
                if !v.is_null() {
                    data[k.clone()] = v.clone();
                }
            }
        }
        let next = revision + 1;
        let newdata = serde_json::to_string(&data).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let at = "";
        db.execute("UPDATE records SET revision=?2,data_json=?3,updated_at=?4 WHERE id=?1",
            rusqlite::params![input.design_id, next, newdata, at]).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let version_id = uuid::Uuid::new_v4().to_string();
        let snapshot = serde_json::json!({"design_id": input.design_id, "design_revision": next, "snapshot": data});
        let raw = serde_json::to_string(&snapshot).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        db.execute("INSERT INTO records(id,kind,project_id,parent_id,revision,data_json,created_at,updated_at) VALUES(?1,?2,?3,?4,?5,?6,?7,?7)",
            rusqlite::params![version_id, "design_version", project, input.design_id, next, raw, at]).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let title = data["title"].as_str().unwrap_or("").to_string();
        Ok(DesignSnapshot { design_id: input.design_id, revision: next as u64, title, data: data.to_string() })
    }

    fn create_run(&self, _ctx: &Context, design_id: String, operation_id: String) -> Result<RunSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let op = if operation_id.trim().is_empty() { None } else { Some(operation_id.as_str()) };
        let record = cli_impl::create_run_for_design(db, &design_id, None, op).map_err(map_cli)?;
        let run_id = record["id"].as_str().unwrap_or("").to_string();
        let v = cli_impl::run_workspace(db, &run_id).map_err(map_cli)?;
        let mut s = snapshot(v);
        s.run_id = run_id;
        Ok(s)
    }

    fn close(&self) -> Result<(), Error> {
        if let Ok(mut g) = self.db.lock() { g.take(); }
        Ok(())
    }
}

fn map_cli(e: cli_impl::CliError) -> Error {
    match e.code {
        "INVALID_INPUT" => Error::Go(format!("INVALID: {}", e.message)),
        "NOT_FOUND" => Error::Go(format!("NOT_FOUND: {}", e.message)),
        "REVISION_CONFLICT" => Error::Go(format!("CONFLICT: {}", e.message)),
        _ => Error::Go(format!("INTERNAL: {}", e.message)),
    }
}

fn snapshot(v: serde_json::Value) -> RunSnapshot {
    let design_id = v["design_id"].as_str().unwrap_or("").into();
    let design_revision = v["design_revision"].as_u64().or_else(|| v["revision"].as_u64()).unwrap_or(0);
    let mut s = RunSnapshot {
        run_id: v["run_id"].as_str().unwrap_or("").into(),
        design_id,
        design_revision,
        revision: v["revision"].as_u64().unwrap_or(0),
        title: v["frozen_design"]["content"]["title"].as_str().unwrap_or("").into(),
        status: v["status"].as_str().unwrap_or("").into(),
        tasks: vec![],
        measurements: vec![],
        events: vec![],
    };
    for t in v["tasks"].as_array().into_iter().flatten() {
        let assay_id = t
            .get("scope")
            .and_then(|sc| {
                sc.get("assay_id")
                    .or_else(|| sc.get("Assay").and_then(|a| a.get("assay_id")))
            })
            .and_then(|x| x.as_str())
            .unwrap_or("")
            .to_string();
        let protocol_version_id = v["scopes"]
            .as_array()
            .into_iter()
            .flatten()
            .find(|sc| sc["kind"].as_str() == Some("assay") && sc["id"].as_str() == Some(assay_id.as_str()))
            .and_then(|sc| sc["protocol_version_id"].as_str())
            .unwrap_or("")
            .to_string();
        s.tasks.push(RunTask {
            key: t["task_key"].as_str().unwrap_or("").into(),
            title: t["step_path"].as_str().unwrap_or("").into(),
            timepoint: t["timepoint"].as_str().unwrap_or("").into(),
            status: t["status"].as_str().unwrap_or("").into(),
            assay_id,
            protocol_version_id,
        });
    }
    for e in v["events"].as_array().into_iter().flatten() {
        s.events.push(RunEvent {
            id: e["event_id"].as_str().or_else(||e["id"].as_str()).unwrap_or("").into(),
            event_type: e["event_type"].as_str().unwrap_or("").into(),
            content: e["content"].as_str().unwrap_or("").into(),
            task_key: e["task_key"].as_str().unwrap_or("").into(),
            step_status: e["step_status"].as_str().unwrap_or("").into(),
            timepoint: e["timepoint"].as_str().unwrap_or("").into(),
            batch_id: e["batch_id"].as_str().unwrap_or("").into(),
            schema_id: e["schema_id"].as_str().unwrap_or("").into(),
            schema_version: e["schema_version"].as_u64().unwrap_or_default() as i64,
            result_cells: e["result_cells"].as_array().into_iter().flatten().map(|cell| ResultCell {
                row: cell["row"].as_u64().unwrap_or_default() as i64,
                column: cell["column"].as_u64().unwrap_or_default() as i64,
                sample_id: cell["sample_id"].as_str().unwrap_or("").into(),
                group: cell["group"].as_str().unwrap_or("").into(),
                replicate: cell["replicate"].as_u64().unwrap_or_default() as i64,
                field: cell["field"].as_str().unwrap_or("").into(),
                value: cell["value"].as_f64(),
                status: cell["status"].as_str().unwrap_or("").into(),
            }).collect(),
        });
        for m in e["measurements"].as_array().into_iter().flatten() {
            s.measurements.push(Measurement {
                sample_id: m["sample_id"].as_str().unwrap_or("").into(),
                endpoint: m["endpoint"].as_str().unwrap_or("").into(),
                value: m["value"].as_f64().unwrap_or(0.0),
                unit: m["unit"].as_str().unwrap_or("").into(),
            });
        }
    }
    if s.title.is_empty() {
        s.title = v["title"].as_str().unwrap_or("").into();
    }
    s
}
