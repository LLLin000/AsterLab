mod mygo;

use mygo::*;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

pub fn make_run_service() -> Result<Arc<dyn RunService>, Error> {
    Ok(Arc::new(FileRunService { path: store_path(), lock: Mutex::new(()) }))
}

fn store_path() -> PathBuf {
    if let Ok(p) = std::env::var("ASTERLAB_MYGO_RUNS") { return PathBuf::from(p); }
    std::env::temp_dir().join("asterlab-mygo-runs.json")
}

struct FileRunService { path: PathBuf, lock: Mutex<()> }

impl RunService for FileRunService {
    fn load_run(&self, _ctx: &Context, _run_id: String) -> Result<RunSnapshot, Error> {
        let _g = self.lock.lock().map_err(|_| Error::Go("lock poisoned".into()))?;
        read_snapshot(&self.path)
    }

    fn save_readout(&self, _ctx: &Context, input: ReadoutInput) -> Result<RunSnapshot, Error> {
        let _g = self.lock.lock().map_err(|_| Error::Go("lock poisoned".into()))?;
        let mut snap = read_snapshot(&self.path)?;
        if input.measurements.is_empty() { return Err(Error::Go("至少填写一条有效数值".into())); }
        for m in &input.measurements {
            if m.sample_id.trim().is_empty() { return Err(Error::Go("样本 / 孔位不能为空".into())); }
            if !m.value.is_finite() { return Err(Error::Go("测量值必须是有限数值".into())); }
        }
        snap.measurements.extend(input.measurements);
        // Write the whole snapshot to a sibling file and rename it, so a
        // failure leaves the previous snapshot intact.
        let tmp = self.path.with_extension("json.tmp");
        let text = serde_json::to_string_pretty(&json_snapshot(&snap)).map_err(|e| Error::Go(e.to_string()))?;
        std::fs::write(&tmp, text).map_err(|e| Error::Go(e.to_string()))?;
        std::fs::rename(&tmp, &self.path).map_err(|e| Error::Go(e.to_string()))?;
        Ok(snap)
    }
}

fn json_snapshot(s: &RunSnapshot) -> serde_json::Value {
    serde_json::json!({
        "runId": s.run_id, "title": s.title, "status": s.status,
        "tasks": s.tasks.iter().map(|t| serde_json::json!({"key":t.key,"title":t.title,"timepoint":t.timepoint,"status":t.status})).collect::<Vec<_>>(),
        "measurements": s.measurements.iter().map(|m| serde_json::json!({"sampleId":m.sample_id,"endpoint":m.endpoint,"value":m.value,"unit":m.unit})).collect::<Vec<_>>()
    })
}
fn read_snapshot(path: &PathBuf) -> Result<RunSnapshot, Error> {
    let text = std::fs::read_to_string(path).map_err(|e| Error::Go(e.to_string()))?;
    let v: serde_json::Value = serde_json::from_str(&text).map_err(|e| Error::Go(e.to_string()))?;
    let mut s = RunSnapshot { run_id: v["runId"].as_str().unwrap_or("").into(), title: v["title"].as_str().unwrap_or("").into(), status: v["status"].as_str().unwrap_or("").into(), tasks: vec![], measurements: vec![] };
    for t in v["tasks"].as_array().into_iter().flatten() { s.tasks.push(RunTask { key:t["key"].as_str().unwrap_or("").into(), title:t["title"].as_str().unwrap_or("").into(), timepoint:t["timepoint"].as_str().unwrap_or("").into(), status:t["status"].as_str().unwrap_or("").into() }); }
    for m in v["measurements"].as_array().into_iter().flatten() { s.measurements.push(Measurement { sample_id:m["sampleId"].as_str().unwrap_or("").into(), endpoint:m["endpoint"].as_str().unwrap_or("").into(), value:m["value"].as_f64().unwrap_or(0.0), unit:m["unit"].as_str().unwrap_or("").into() }); }
    Ok(s)
}
