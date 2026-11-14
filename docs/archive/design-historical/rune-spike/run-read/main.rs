//! Read-only Rune adapter for one existing frozen RunWorkspace JSON snapshot.
//! It never opens or writes SQLite; it renders tasks, timepoints, status and measurements.
use rune::tok::{self, radius, space, type_};
use rune::{AppSpec, Cx, El, col, row, text};
use serde_json::Value;

struct Run {
    status: String,
    tasks: Vec<(String, String, String)>,
    measurements: Vec<String>,
}

fn load() -> Run {
    let v: Value = serde_json::from_str(include_str!("../run-workspace.json")).expect("RunWorkspace JSON");
    let step_title = |path: &str| {
        v["steps"].as_array().and_then(|steps| steps.iter().find_map(|s| {
            (s["step"]["path"].as_str() == Some(path)).then(|| s["step"]["title"].as_str().unwrap_or(path).to_owned())
        })).unwrap_or_else(|| path.to_owned())
    };
    let tasks = v["tasks"].as_array().into_iter().flatten().map(|t| {
        let path = t["step_path"].as_str().unwrap_or("?");
        (t["timepoint"].as_str().unwrap_or("一次").to_owned(), step_title(path), t["status"].as_str().unwrap_or("unknown").to_owned())
    }).collect();
    let mut measurements = Vec::new();
    for e in v["events"].as_array().into_iter().flatten() {
        for m in e["measurements"].as_array().into_iter().flatten() {
            measurements.push(format!("{} · {} {} {} · {}", e["timepoint"].as_str().unwrap_or("一次"), m["endpoint"].as_str().unwrap_or("?"), m["value"], m["unit"].as_str().unwrap_or(""), m["sample_id"].as_str().unwrap_or("")));
        }
    }
    Run { status: v["status"].as_str().unwrap_or("?").to_owned(), tasks, measurements }
}

fn view(r: &Run, cx: &mut Cx<Run>) -> El {
    let t = tok::theme(cx.system_dark());
    let task_rows = r.tasks.iter().map(|(tp, title, status)| {
        row().w_full().h(24.0).gap(space::S2).items_center()
            .child(text(tp).w(42.0).font(type_::LABEL).color(t.text_secondary))
            .child(text(title).w(180.0).font(type_::BODY).color(t.text))
            .child(text(status).font(type_::LABEL).color(t.text_secondary))
    });
    let measure_rows = r.measurements.iter().take(8).map(|m| text(m).w_full().font(type_::LABEL).color(t.text));
    col().w_full().h_full().px(space::S6).py(space::S6).gap(space::S3).fill(t.bg)
        .child(text("冻结 Run · 只读").font(type_::TITLE).color(t.text))
        .child(text(format!("状态：{} · 任务 {} · 测量 {}", r.status, r.tasks.len(), r.measurements.len())).font(type_::BODY).color(t.text_secondary))
        .child(
            col().w_full().gap(space::S1).px(space::S3).py(space::S3).radius(radius::LG).fill(t.card).border(1.0, t.card_border)
                .child(text("时间点 / 任务 / 状态").font(type_::TITLE).color(t.text))
                .children(task_rows),
        )
        .child(
            col().w_full().gap(space::S1).px(space::S3).py(space::S3).radius(radius::LG).fill(t.card).border(1.0, t.card_border)
                .child(text("测量值").font(type_::TITLE).color(t.text))
                .children(measure_rows),
        )
}

fn main() {
    rune::run(AppSpec::new("Frozen Run read-only", (920.0, 600.0), load, view));
}
