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
    fn list_runs(&self, _ctx: &Context) -> Result<Vec<RunSummary>, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let db = core.raw_db();
        let mut stmt = db.prepare("SELECT id,project_id,data_json,updated_at FROM records WHERE kind='run' ORDER BY updated_at DESC, rowid DESC LIMIT 50")
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |row| Ok((row.get::<_,String>(0)?, row.get::<_,Option<String>>(1)?, row.get::<_,String>(2)?, row.get::<_,String>(3)?)))
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows.flatten() {
            let (id, _project, raw, updated) = row;
            let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            out.push(RunSummary {
                run_id: id,
                title: data["title"].as_str().unwrap_or("").into(),
                status: data["status"].as_str().unwrap_or("").into(),
                design_id: data["design_id"].as_str().unwrap_or("").into(),
                updated_at: updated,
            });
        }
        Ok(out)
    }

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
        let mut request = serde_json::json!({
            "run_id":input.run_id,"operation_id":input.operation_id,"task_key":input.task_key,
            "schema_id":input.schema_id,"schema_version":input.schema_version,
            "expected_revision":input.expected_revision,"cells":cells
        });
        if !input.assay_id.is_empty() { request["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.timepoint.is_empty() { request["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.protocol_step_path.is_empty() { request["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { request["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if !input.plate_id.is_empty() { request["plate_id"] = serde_json::json!(input.plate_id); }
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

    fn save_notebook(&self, _ctx: &Context, input: NotebookInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "observation",
            "stage": "notebook",
            "content": input.markdown,
            "operation_id": input.operation_id
        });
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_task_checks(&self, _ctx: &Context, input: TaskCheckInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let checks: Vec<serde_json::Value> = input.checks.iter().map(|check| serde_json::json!({
            "check_id": check.check_id, "status": check.status
        })).collect();
        let mut event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "operation",
            "content": "核对执行检查项",
            "task_key": input.task_key,
            "operation_id": input.operation_id,
            "check_ack": true,
            "checks": checks
        });
        if !input.assay_id.is_empty() { event["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.timepoint.is_empty() { event["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.protocol_step_path.is_empty() { event["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { event["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if input.checks.is_empty() { event["checks"] = serde_json::json!([]); }
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn complete_task(&self, _ctx: &Context, input: CompleteTaskInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let checks: Vec<serde_json::Value> = input.checks.iter().map(|check| serde_json::json!({
            "check_id": check.check_id, "status": check.status
        })).collect();
        let mut event = serde_json::json!({
            "run_id": input.run_id,
            "event_type": "operation",
            "content": "完成任务",
            "task_key": input.task_key,
            "step_status": "completed",
            "operation_id": input.operation_id,
            "check_ack": true,
            "checks": checks
        });
        if !input.assay_id.is_empty() { event["assay_id"] = serde_json::json!(input.assay_id); }
        if !input.timepoint.is_empty() { event["timepoint"] = serde_json::json!(input.timepoint); }
        if !input.protocol_step_path.is_empty() { event["protocol_step_path"] = serde_json::json!(input.protocol_step_path); }
        if !input.protocol_version_id.is_empty() { event["protocol_version_id"] = serde_json::json!(input.protocol_version_id); }
        if input.checks.is_empty() { event["checks"] = serde_json::json!([]); }
        let v = core.save_readout(event).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(snapshot(v))
    }

    fn save_asset_link(&self, _ctx: &Context, input: AssetLinkInput) -> Result<Asset, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let asset_json = serde_json::json!({
            "asset": {
                "id": input.asset.id,
                "rootId": input.asset.root_id,
                "relativePath": input.asset.relative_path,
                "mediaType": input.asset.media_type,
                "size": input.asset.size,
                "modTime": input.asset.mod_time,
                "sha256": input.asset.sha256,
                "availability": input.asset.availability,
                "link": {
                    "runId": input.asset.link.run_id,
                    "taskKey": input.asset.link.task_key,
                    "sampleId": if input.asset.link.sample_id.is_empty() { serde_json::Value::Null } else { serde_json::json!(input.asset.link.sample_id) }
                },
                "linkedAt": input.asset.linked_at,
                "actorId": input.asset.actor_id
            }
        });
        let result = core.save_asset_link(asset_json).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        Ok(asset_from_value(&result))
    }

    fn list_asset_links(&self, _ctx: &Context, run_id: String) -> Result<Vec<Asset>, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("INTERNAL: core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let v = core.list_asset_links(&run_id).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let arr = v.as_array().cloned().unwrap_or_default();
        Ok(arr.iter().map(asset_from_value).collect())
    }

    fn close(&self) -> Result<(), Error> {
        if let Ok(mut g) = self.core.lock() { g.take(); }
        Ok(())
    }
}

fn asset_from_value(v: &serde_json::Value) -> Asset {
    Asset {
        id: v.get("id").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        root_id: v.get("rootId").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        relative_path: v.get("relativePath").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        media_type: v.get("mediaType").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        size: v.get("size").and_then(|x| x.as_i64()).unwrap_or(0),
        mod_time: v.get("modTime").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        sha256: v.get("sha256").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        availability: v.get("availability").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        link: AssetLink {
            run_id: v.get("link").and_then(|x| x.get("runId")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            task_key: v.get("link").and_then(|x| x.get("taskKey")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
            sample_id: v.get("link").and_then(|x| x.get("sampleId")).and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        },
        linked_at: v.get("linkedAt").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
        actor_id: v.get("actorId").and_then(|x| x.as_str()).unwrap_or("").to_owned(),
    }
}

fn parameter_text(value: &serde_json::Value) -> String {
    match value {
        serde_json::Value::String(text) => text.clone(),
        serde_json::Value::Number(number) => number.to_string(),
        serde_json::Value::Bool(flag) => flag.to_string(),
        serde_json::Value::Null => String::new(),
        other => other.to_string(),
    }
}

struct CoreDesignService { db: Mutex<Option<rusqlite::Connection>> }

impl DesignService for CoreDesignService {
    fn list_projects(&self, _ctx: &Context) -> Result<Vec<ProjectSummary>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut stmt = db.prepare("SELECT id,data_json FROM records WHERE kind='project' ORDER BY created_at ASC")
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |row| Ok((row.get::<_,String>(0)?, row.get::<_,String>(1)?)))
            .map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows.flatten() {
            let (id, raw) = row;
            let data: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            out.push(ProjectSummary {
                project_id: id,
                name: data["name"].as_str().unwrap_or("未命名项目").into(),
            });
        }
        Ok(out)
    }

    fn list_protocols(&self, _ctx: &Context) -> Result<Vec<ProtocolSummary>, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let mut stmt = db.prepare(
            "SELECT v.id, v.parent_id, v.data_json, p.data_json FROM records v LEFT JOIN records p ON p.id=v.parent_id WHERE v.kind='protocol_version' ORDER BY v.created_at DESC"
        ).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let rows = stmt.query_map([], |row| Ok((
            row.get::<_,String>(0)?, row.get::<_,Option<String>>(1)?,
            row.get::<_,String>(2)?, row.get::<_,Option<String>>(3)?,
        ))).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?;
        let mut out = Vec::new();
        for row in rows.flatten() {
            let (id, protocol_id, raw, protocol_raw) = row;
            let version: serde_json::Value = serde_json::from_str(&raw).unwrap_or(serde_json::json!({}));
            let protocol: serde_json::Value = serde_json::from_str(protocol_raw.as_deref().unwrap_or("{}")).unwrap_or(serde_json::json!({}));
            out.push(ProtocolSummary {
                protocol_version_id: id,
                protocol_id: protocol_id.unwrap_or_default(),
                title: protocol["title"].as_str().or_else(|| version["title"].as_str()).unwrap_or("Protocol").into(),
                version: version["version"].as_i64().unwrap_or(1) as i64,
            });
        }
        Ok(out)
    }

    fn preview_protocol_block(&self, _ctx: &Context, block_version_id: String) -> Result<ProtocolBlockPreview, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let manifest = cli_impl::protocol_block_manifest(db, Some(&block_version_id), &serde_json::json!([]))
            .map_err(|e| Error::Go(format!("INVALID: {e}")))?;
        let mut steps = Vec::new();
        for step in manifest["steps"].as_array().into_iter().flatten() {
            let path = step["path"].as_str().unwrap_or("").to_owned();
            if path.is_empty() {
                continue;
            }
            let mut parameters = Vec::new();
            for parameter in step["parameters"].as_array().into_iter().flatten() {
                parameters.push(ProtocolBlockParameter {
                    key: parameter["slot"].as_str().unwrap_or("").to_owned(),
                    label: parameter["label"].as_str().unwrap_or("").to_owned(),
                    value: parameter_text(&parameter["value"]),
                    unit: parameter["unit"].as_str().unwrap_or("").to_owned(),
                });
            }
            let unresolved_parameter_refs = step["unresolved_parameter_refs"]
                .as_array()
                .into_iter()
                .flatten()
                .filter_map(|value| value.as_str().map(str::to_owned))
                .collect();
            steps.push(ProtocolBlockStep {
                use_id: step["use_id"].as_str().unwrap_or("").to_owned(),
                path,
                title: step["title"].as_str().unwrap_or("").to_owned(),
                instruction: step["instruction"].as_str().unwrap_or("").to_owned(),
                source_block_version_id: step["source_block_version_id"].as_str().unwrap_or("").to_owned(),
                parameters,
                unresolved_parameter_refs,
            });
        }
        Ok(ProtocolBlockPreview {
            block_version_id,
            ready: manifest["protocol_ready"].as_bool().unwrap_or(false),
            steps,
        })
    }

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

    fn create_aster_doc_design(&self, _ctx: &Context, input: AsterDocCreateInput) -> Result<AsterDocSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let request = serde_json::json!({
            "project_id": input.project_id,
            "title": input.title,
            "source": input.source,
            "operation_id": input.operation_id,
            "actor_id": input.actor_id
        });
        let value = cli_impl::create_asterdoc_design(db, &request).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn apply_aster_doc_command(&self, _ctx: &Context, input: AsterDocCommandInput) -> Result<AsterDocSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let command: serde_json::Value = serde_json::from_str(&input.command)
            .map_err(|e| Error::Go(format!("INVALID: command is not JSON: {e}")))?;
        let request = serde_json::json!({
            "design_id": input.design_id,
            "expected_revision": input.expected_revision,
            "operation_id": input.operation_id,
            "actor_id": input.actor_id,
            "command": command
        });
        let value = cli_impl::apply_asterdoc_command(db, &request).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn save_aster_doc(&self, _ctx: &Context, input: AsterDocSaveInput) -> Result<AsterDocSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let request = serde_json::json!({
            "design_id": input.design_id,
            "source": input.source,
            "expected_revision": input.expected_revision,
            "operation_id": input.operation_id,
            "actor_id": input.actor_id
        });
        let value = cli_impl::save_asterdoc_version(db, &request).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn load_aster_doc(&self, _ctx: &Context, design_id: String) -> Result<AsterDocSnapshot, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::load_asterdoc(db, &design_id).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn assess_aster_doc(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<AsterDocAssessment, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::assess_asterdoc(db, &serde_json::json!({
            "design_id": design_id, "revision": revision
        })).map_err(map_cli)?;
        Ok(AsterDocAssessment {
            design_id: value["design_id"].as_str().unwrap_or("").into(),
            revision: value["revision"].as_u64().unwrap_or_default(),
            source_hash: value["source_hash"].as_str().unwrap_or("").into(),
            state: value["state"].as_str().unwrap_or("").into(),
            errors: value["errors"].as_array().into_iter().flatten().filter_map(|v| v.as_str().map(str::to_owned)).collect(),
            warnings: value["warnings"].as_array().into_iter().flatten().filter_map(|v| v.as_str().map(str::to_owned)).collect(),
        })
    }

    fn create_run_from_aster_doc(&self, _ctx: &Context, input: AsterDocRunInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let title = if input.title.is_empty() { None } else { Some(input.title.as_str()) };
        let op = if input.operation_id.is_empty() { None } else { Some(input.operation_id.as_str()) };
        let v = cli_impl::create_run_from_asterdoc(db, &input.design_id, input.document_revision, op, title, &input.acknowledged_warnings)
            .map_err(map_cli)?;
        let run_id = v["id"].as_str().unwrap_or_default();
        let workspace = cli_impl::run_workspace(db, run_id).map_err(map_cli)?;
        let mut value = workspace;
        value["revision"] = v["revision"].clone();
        Ok(snapshot(value))
    }

    fn compile_aster_doc(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<AsterDocProjection, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let doc = cli_impl::load_asterdoc_at(db, &design_id, revision).map_err(map_cli)?;
        let source = doc["source"].as_str().unwrap_or("");
        let design = cli_impl::compile_asterdoc_to_design(source).map_err(map_cli)?;
        Ok(AsterDocProjection {
            design: serde_json::to_string(&design).map_err(|e| Error::Go(format!("INTERNAL: {e}")))?,
            document: asterdoc_snapshot(doc),
        })
    }

    fn load_aster_doc_at(&self, _ctx: &Context, design_id: String, revision: u64) -> Result<AsterDocSnapshot, Error> {
        let guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_ref().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let value = cli_impl::load_asterdoc_at(db, &design_id, revision).map_err(map_cli)?;
        Ok(asterdoc_snapshot(value))
    }

    fn save_design(&self, _ctx: &Context, input: DesignSaveInput) -> Result<DesignSnapshot, Error> {
        let mut guard = self.db.lock().map_err(|_| Error::Go("INTERNAL: db lock poisoned".into()))?;
        let db = guard.as_mut().ok_or_else(|| Error::Go("INTERNAL: closed".into()))?;
        let changes: serde_json::Value = serde_json::from_str(&input.changes)
            .unwrap_or(serde_json::Value::Null);
        let result = cli_impl::revise_design_record(
            db,
            &input.design_id,
            input.expected_revision as i64,
            &changes,
        ).map_err(map_cli)?;
        let data = result["design"]["data"].clone();
        Ok(DesignSnapshot {
            design_id: input.design_id,
            revision: result["design"]["revision"].as_u64().unwrap_or_default(),
            title: data["title"].as_str().unwrap_or("").to_string(),
            data: serde_json::to_string(&data).unwrap_or_default(),
        })
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

fn asterdoc_snapshot(v: serde_json::Value) -> AsterDocSnapshot {
    AsterDocSnapshot {
        design_id: v["design_id"].as_str().unwrap_or("").into(),
        revision: v["revision"].as_u64().unwrap_or_default(),
        design_revision: v["design_revision"].as_u64().unwrap_or_default(),
        source: v["source"].as_str().unwrap_or("").into(),
        source_hash: v["source_hash"].as_str().unwrap_or("").into(),
        schema_version: v["schema_version"].as_i64().unwrap_or_default(),
        compiler_version: v["compiler_version"].as_str().unwrap_or("").into(),
        created_at: v["created_at"].as_str().unwrap_or("").into(),
        actor_id: v["actor_id"].as_str().unwrap_or("").into(),
    }
}

fn snapshot(v: serde_json::Value) -> RunSnapshot {
    let mut design_content = v["frozen_design"]["content"].clone();
    if let Some(mygo) = v["frozen_design"].get("mygo") {
        if !mygo.is_null() { design_content["mygo"] = mygo.clone(); }
    }
    if let Some(asterlab) = v["frozen_design"].get("asterlab") {
        if !asterlab.is_null() { design_content["asterlab"] = asterlab.clone(); }
    }
    let design_data = serde_json::to_string(&design_content).unwrap_or_default();
    let design_id = v["design_id"].as_str().unwrap_or("").into();
    let design_revision = v["design_revision"].as_u64().or_else(|| v["revision"].as_u64()).unwrap_or(0);
    let mut s = RunSnapshot {
        run_id: v["run_id"].as_str().unwrap_or("").into(),
        design_id,
        design_revision,
        design_data,
        revision: v["revision"].as_u64().unwrap_or(0),
        title: v["frozen_design"]["content"]["title"].as_str().unwrap_or("").into(),
        status: v["status"].as_str().unwrap_or("").into(),
        tasks: vec![],
        measurements: vec![],
        events: vec![],
        aster_doc: v["asterdoc"].as_object().map(|doc| AsterDocRef {
            design_id: doc["design_id"].as_str().unwrap_or("").into(),
            revision: doc["revision"].as_u64().unwrap_or_default(),
            source_hash: doc["source_hash"].as_str().unwrap_or("").into(),
            schema_version: doc["schema_version"].as_i64().unwrap_or_default() as i64,
            compiler_version: doc["compiler_version"].as_str().unwrap_or("").into(),
        }),
        creation_audit: v["creation_audit"].as_object().map(|audit| CreationAudit {
            rules_version: audit["rules_version"].as_i64().unwrap_or_default() as i64,
            design_revision: audit["design_revision"].as_u64().unwrap_or_default(),
            design_version_id: audit["design_version_id"].as_str().unwrap_or("").into(),
            actor_id: audit["actor_id"].as_str().unwrap_or("").into(),
            recorded_at: audit["recorded_at"].as_str().unwrap_or("").into(),
            decision: audit["decision"].as_str().unwrap_or("").into(),
            acknowledged_warnings: audit["acknowledged_warnings"].as_array().into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect(),
            warnings: audit["warnings"].as_array().into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect(),
            errors: audit["errors"].as_array().into_iter().flatten()
                .filter_map(|value| value.as_str().map(str::to_owned)).collect(),
        }),
    };
    // A task names the design step it executes through the design's explicit
    // stepTasks entries, matched on assay and timepoint; several matches stay
    // unresolved. Only designs written before stepTasks existed fall back to
    // the old unique-assay rule, and a title never decides either way.
    let step_tasks = v["frozen_design"]["mygo"]["stepTasks"]
        .as_array()
        .cloned()
        .unwrap_or_default();
    // A present table, even empty, means the design authored its bindings:
    // tasks without an entry stay unresolved instead of falling back.
    let explicit_links = v["frozen_design"]["mygo"]
        .as_object()
        .is_some_and(|mygo| mygo.contains_key("stepTasks"));
    // How many protocol paths each (assay, timepoint) pair appears under: an
    // entry without a path may only claim a pair that occurs once.
    let mut paths_per_pair: std::collections::HashMap<(String, String), std::collections::HashSet<String>> =
        std::collections::HashMap::new();
    for task in v["tasks"].as_array().into_iter().flatten() {
        let assay = task["scope"]
            .get("assay_id")
            .or_else(|| task["scope"].get("Assay").and_then(|a| a.get("assay_id")))
            .and_then(|x| x.as_str())
            .unwrap_or("");
        let timepoint = task["timepoint"].as_str().unwrap_or("");
        if assay.is_empty() {
            continue;
        }
        paths_per_pair
            .entry((assay.to_string(), timepoint.to_string()))
            .or_default()
            .insert(task["step_path"].as_str().unwrap_or("").to_string());
    }
    // Checks belong to the frozen protocol step. Match by its path; a path
    // names one step, and a task stores exactly that path.
    let mut checks_by_path: std::collections::HashMap<String, Vec<TaskCheck>> =
        std::collections::HashMap::new();
    for step in v["steps"].as_array().into_iter().flatten() {
        let path = step["step"]["path"].as_str().unwrap_or("").to_owned();
        let checks = step["step"]["checks"].as_array().into_iter().flatten().map(|check| TaskCheck {
            check_id: check["check_id"].as_str().unwrap_or("").into(),
            label: check["label"].as_str().unwrap_or("").into(),
            instruction: check["instruction"].as_str().unwrap_or("").into(),
            blocking: check["blocking"].as_bool().unwrap_or(false),
            status: String::new(),
        }).collect();
        checks_by_path.insert(path, checks);
    }
    let mut steps_by_path: std::collections::HashMap<String, (String, String, String, String)> =
        std::collections::HashMap::new();
    for step in v["steps"].as_array().into_iter().flatten() {
        let path = step["step"]["path"].as_str().unwrap_or("").to_owned();
        if path.is_empty() { continue; }
        let title = step["step"]["title"].as_str().unwrap_or("").to_owned();
        let instruction = step["step"]["instruction"].as_str().unwrap_or("").to_owned();
        let parameters_json = serde_json::to_string(&step["step"]["parameters"]).unwrap_or_else(|_| "[]".into());
        let source_block_version_id = step["step"]["source_block_version_id"].as_str().unwrap_or("").to_owned();
        steps_by_path.insert(path, (title, instruction, parameters_json, source_block_version_id));
    }
    let mut steps_by_assay: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    if let Some(step_assays) = v["frozen_design"]["mygo"]["stepAssays"].as_object() {
        for (node, assay) in step_assays {
            if let Some(assay) = assay.as_str().filter(|a| !a.is_empty()) {
                steps_by_assay.entry(assay.to_string()).or_default().push(node.clone());
            }
        }
    }
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
        let timepoint = t["timepoint"].as_str().unwrap_or("").to_string();
        let design_step_id = if explicit_links {
            let step_path = t["step_path"].as_str().unwrap_or("");
            let pair: Vec<&serde_json::Value> = step_tasks
                .iter()
                .filter(|link| {
                    link["assayId"].as_str() == Some(assay_id.as_str())
                        && link["timepoint"].as_str() == Some(timepoint.as_str())
                })
                .collect();
            // A path-qualified entry is exact. Otherwise the pair must occur
            // under exactly one protocol path and name exactly one step.
            let by_path: Vec<&serde_json::Value> = pair
                .iter()
                .copied()
                .filter(|link| link["path"].as_str() == Some(step_path))
                .collect();
            let chosen: Option<String> = if by_path.len() == 1 {
                by_path[0]["stepId"].as_str().map(str::to_owned)
            } else if by_path.is_empty() {
                let pair_paths = paths_per_pair
                    .get(&(assay_id.clone(), timepoint.clone()))
                    .map(|paths| paths.len())
                    .unwrap_or(0);
                let unqualified: Vec<&serde_json::Value> = pair
                    .iter()
                    .copied()
                    .filter(|link| {
                        link.get("path")
                            .and_then(|p| p.as_str())
                            .map(str::is_empty)
                            .unwrap_or(true)
                    })
                    .collect();
                if pair_paths == 1 && unqualified.len() == 1 {
                    unqualified[0]["stepId"].as_str().map(str::to_owned)
                } else {
                    None
                }
            } else {
                None
            };
            chosen.unwrap_or_default()
        } else {
            steps_by_assay
                .get(&assay_id)
                .filter(|nodes| nodes.len() == 1)
                .map(|nodes| nodes[0].clone())
                .unwrap_or_default()
        };
        let path: String = t["step_path"].as_str().unwrap_or("").into();
        let (title, instruction, parameters_json, source_block_version_id) = steps_by_path
            .get(&path)
            .cloned()
            .unwrap_or_else(|| (path.clone(), String::new(), "[]".into(), String::new()));
        let mut checks = checks_by_path.get(&path).cloned().unwrap_or_default();
        for entry in v["design_checks"].as_array().into_iter().flatten() {
            if entry["path"].as_str() != Some(path.as_str()) {
                continue;
            }
            for check in entry["checks"].as_array().into_iter().flatten() {
                let check_id = check["check_id"].as_str().unwrap_or("");
                if check_id.is_empty()
                    || checks.iter().any(|existing| existing.check_id == check_id)
                {
                    continue;
                }
                checks.push(TaskCheck {
                    check_id: check_id.into(),
                    label: check["label"].as_str().unwrap_or("").into(),
                    instruction: "".into(),
                    blocking: check["blocking"].as_bool().unwrap_or(false),
                    status: String::new(),
                });
            }
        }
        s.tasks.push(RunTask {
            key: t["task_key"].as_str().unwrap_or("").into(),
            title,
            path,
            timepoint,
            status: t["status"].as_str().unwrap_or("").into(),
            assay_id,
            protocol_version_id,
            design_step_id,
            instruction,
            parameters_json,
            source_block_version_id,
            checks,
        });
    }
    // Apply only formal CheckAck events. Free-form execution_event checks are
    // observations, not authoritative acknowledgements.
    for task in &mut s.tasks {
        for check in &mut task.checks {
            let mut best: Option<(String, String)> = None;
            for e in v["events"].as_array().into_iter().flatten() {
                if !e["check_ack"].as_bool().unwrap_or(false) {
                    continue;
                }
                let matches = e["task_key"].as_str() == Some(task.key.as_str())
                    || e["protocol_step_path"].as_str() == Some(task.path.as_str());
                if !matches {
                    continue;
                }
                let at = e["recorded_at"].as_str()
                    .or_else(|| e["occurred_at"].as_str())
                    .unwrap_or("")
                    .to_owned();
                for ack in e["checks"].as_array().into_iter().flatten() {
                    if ack["check_id"].as_str() != Some(check.check_id.as_str()) {
                        continue;
                    }
                    if best.as_ref().map_or(true, |(prev, _)| at.as_str() >= prev.as_str()) {
                        best = Some((at.clone(), ack["status"].as_str().unwrap_or("").to_owned()));
                    }
                }
            }
            if let Some((_, status)) = best {
                check.status = status;
            }
        }
    }
    for e in v["events"].as_array().into_iter().flatten() {
        s.events.push(RunEvent {
            id: e["event_id"].as_str().or_else(||e["id"].as_str()).unwrap_or("").into(),
            event_type: e["event_type"].as_str().unwrap_or("").into(),
            content: e["content"].as_str().unwrap_or("").into(),
            occurred_at: e["occurred_at"].as_str().unwrap_or("").into(),
            recorded_at: e["recorded_at"].as_str().unwrap_or("").into(),
            task_key: e["task_key"].as_str().unwrap_or("").into(),
            step_status: e["step_status"].as_str().unwrap_or("").into(),
            timepoint: e["timepoint"].as_str().unwrap_or("").into(),
            stage: e["stage"].as_str().unwrap_or("").into(),
            protocol_step_path: e["protocol_step_path"].as_str().unwrap_or("").into(),
            assay_id: e["assay_id"].as_str().unwrap_or("").into(),
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
            checks: e["checks"].as_array().into_iter().flatten().map(|check| CheckAck {
                check_id: check["check_id"].as_str().unwrap_or("").into(),
                status: check["status"].as_str().unwrap_or("").into(),
            }).collect(),
            data_json: serde_json::to_string(e).unwrap_or_default(),
        });
        for m in e["measurements"].as_array().into_iter().flatten() {
            s.measurements.push(Measurement {
                sample_id: m["sample_id"].as_str().unwrap_or("").into(),
                endpoint: m["endpoint"].as_str().unwrap_or("").into(),
                value: m["value"].as_f64().unwrap_or(0.0),
                unit: m["unit"].as_str().unwrap_or("").into(),
                task_key: e["task_key"].as_str().unwrap_or("").into(),
                protocol_step_path: e["protocol_step_path"].as_str().unwrap_or("").into(),
            });
        }
    }
    if s.title.is_empty() {
        s.title = v["title"].as_str().unwrap_or("").into();
    }
    s
}
