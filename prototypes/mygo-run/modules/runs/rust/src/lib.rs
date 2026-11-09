mod mygo;

use mygo::*;
use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use asterlab_app::AppCore;

pub fn make_run_service() -> Result<Arc<dyn RunService>, Error> {
    let dir = std::env::var("LABWORKBENCH_DATA_DIR")
        .map(PathBuf::from)
        .map_err(|_| Error::Go("LABWORKBENCH_DATA_DIR is required".into()))?;
    let core = AppCore::open(&dir).map_err(Error::Go)?;
    Ok(Arc::new(CoreRunService { core: Mutex::new(Some(core)) }))
}

struct CoreRunService { core: Mutex<Option<AppCore>> }

impl RunService for CoreRunService {
    fn load_run(&self, _ctx: &Context, run_id: String) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("closed".into()))?;
        let v = core.load_run(&run_id).map_err(Error::Go)?;
        Ok(snapshot(v))
    }

    fn save_readout(&self, _ctx: &Context, input: ReadoutInput) -> Result<RunSnapshot, Error> {
        let mut guard = self.core.lock().map_err(|_| Error::Go("core lock poisoned".into()))?;
        let core = guard.as_mut().ok_or_else(|| Error::Go("closed".into()))?;
        let run_id = input.run_id.clone();
        let measurements: Vec<serde_json::Value> = input.measurements.iter().map(|m| serde_json::json!({
            "endpoint": m.endpoint, "value": m.value, "unit": m.unit, "sample_id": m.sample_id
        })).collect();
        let event = serde_json::json!({
            "run_id": run_id,
            "event_type": "measurement",
            "content": "结构化记录",
            "measurements": measurements
        });
        let v = core.save_readout(event).map_err(Error::Go)?;
        Ok(snapshot(v))
    }
    fn close(&self) -> Result<(), Error> {
        if let Ok(mut g) = self.core.lock() { g.take(); }
        Ok(())
    }
}

fn snapshot(v: serde_json::Value) -> RunSnapshot {
    let mut s = RunSnapshot {
        run_id: v["run_id"].as_str().unwrap_or("").into(),
        revision: v["revision"].as_u64().unwrap_or(0),
        title: v["frozen_design"]["content"]["title"].as_str().unwrap_or("").into(),
        status: v["status"].as_str().unwrap_or("").into(),
        tasks: vec![],
        measurements: vec![],
    };
    for t in v["tasks"].as_array().into_iter().flatten() {
        s.tasks.push(RunTask {
            key: t["task_key"].as_str().unwrap_or("").into(),
            title: t["step_path"].as_str().unwrap_or("").into(),
            timepoint: t["timepoint"].as_str().unwrap_or("准备").into(),
            status: t["status"].as_str().unwrap_or("").into(),
        });
    }
    for e in v["events"].as_array().into_iter().flatten() {
        for m in e["measurements"].as_array().into_iter().flatten() {
            s.measurements.push(Measurement {
                sample_id: m["sample_id"].as_str().unwrap_or("").into(),
                endpoint: m["endpoint"].as_str().unwrap_or("").into(),
                value: m["value"].as_f64().unwrap_or(0.0),
                unit: m["unit"].as_str().unwrap_or("").into(),
            });
        }
    }
    s
}
