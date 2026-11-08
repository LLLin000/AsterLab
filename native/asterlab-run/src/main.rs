//! AsterLab Gate 2 Run slice. Reads/writes through the existing labworkbench CLI.
use std::collections::HashMap;
use std::io::Write;
use std::process::{Command, Stdio};

use rune::color::hex;
use rune::tok::{self, space};
use rune::tok::TextStyle;
use rune::{AppSpec, Cx, El, Icon as RuneIcon, ScrollSpec, TextEdit, col, el, row, text};
use rune_kit::{Button, ButtonVariant, Card, IconButton, Kit, PaletteSeed, SegmentedControl, Sidebar, SidebarRow, SidebarSection, TextInput};
use serde_json::{Value, json};

const TITLE: TextStyle = TextStyle::new(20.0, 600.0);
const SECTION: TextStyle = TextStyle::new(16.0, 600.0);
const BODY: TextStyle = TextStyle::new(14.5, 400.0);
const LABEL: TextStyle = TextStyle::new(12.5, 500.0);

fn cli() -> String {
    std::env::var("ASTERLAB_CLI").unwrap_or_else(|_| {
        std::env::current_exe().ok().and_then(|p| p.parent().map(|d| d.join("labworkbench.exe")))
            .filter(|p| p.exists()).map(|p| p.to_string_lossy().into_owned())
            .unwrap_or_else(|| "target/release/labworkbench.exe".into())
    })
}
fn data() -> String { std::env::var("LABWORKBENCH_DATA_DIR").unwrap_or_else(|_| "target/asterlab-ui-runtime3".into()) }
fn call(cmd: &str, input: Value) -> Result<Value, String> {
    let mut child = Command::new(cli()).arg(cmd).arg("--input").arg("-")
        .env("LABWORKBENCH_DATA_DIR", data())
        .stdin(Stdio::piped()).stdout(Stdio::piped()).stderr(Stdio::piped())
        .spawn().map_err(|e| e.to_string())?;
    child.stdin.take().unwrap().write_all(input.to_string().as_bytes()).map_err(|e| e.to_string())?;
    let out = child.wait_with_output().map_err(|e| e.to_string())?;
    let raw = String::from_utf8_lossy(&out.stdout);
    let v: Value = serde_json::from_str(raw.trim()).map_err(|_| format!("{cmd}: {raw}"))?;
    if v["ok"].as_bool() != Some(true) {
        let e = &v["error"];
        let msg = e["message"].as_str().or_else(|| e.as_str()).unwrap_or("命令失败");
        let code = e["code"].as_str().unwrap_or("");
        return Err(if code.is_empty() { msg.to_owned() } else { format!("{msg} ({code})") });
    }
    Ok(v["result"].clone())
}
fn s(v: &Value, key: &str) -> String { v[key].as_str().unwrap_or("").to_string() }
fn arr<'a>(v: &'a Value, key: &str) -> &'a [Value] { v[key].as_array().map(Vec::as_slice).unwrap_or(&[]) }

struct Row { sample: TextEdit, value: TextEdit }
#[derive(Clone, Copy, PartialEq, Eq)]
enum Page { Design, Protocol, Run, Tools }
struct App {
    run_id: String, workspace: Value, selected: usize, checks: HashMap<String, String>,
    rows: Vec<Row>, results: bool, message: String,
    page: Page, sidebar_open: bool,
    projects: Vec<Value>, designs: Vec<Value>, runs: Vec<Value>, protocols: Vec<Value>, records: Vec<Value>,
}
fn init() -> App {
    let env_id = std::env::var("ASTERLAB_RUN_ID").ok();
    let result = if let Some(id) = env_id.clone() {
        call("run.workspace", json!({"id": id}))
    } else {
        match call("run.list", json!({})) {
            Ok(l) => match arr(&l, "records").first().map(|r| s(r, "id")) {
                Some(id) => call("run.workspace", json!({"id": id})),
                None => Err("run.list 没有记录".into()),
            },
            Err(e) => Err(e),
        }
    };
    let workspace = match result {
        Ok(w) => w,
        Err(e) => return App { run_id: String::new(), workspace: Value::Null, selected: 0, checks: HashMap::new(), rows: Vec::new(), results: false, message: format!("无法读取 Run（id={env_id:?}）：{e}"), page: Page::Run, sidebar_open: true, projects: Vec::new(), designs: Vec::new(), runs: Vec::new(), protocols: Vec::new(), records: Vec::new() },
    };
    let projects = call("project.list", json!({})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default();
    let designs = call("design.list", json!({})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default();
    let runs = call("run.list", json!({})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default();
    let run_id = s(&workspace, "run_id");
    let protocols = call("protocol.list", json!({})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default();
    let mut records = call("record.list", json!({"kind":"formula"})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default();
    records.extend(call("record.list", json!({"kind":"library_table"})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default());
    records.extend(call("record.list", json!({"kind":"note"})).ok().and_then(|x| x["records"].as_array().cloned()).unwrap_or_default());
    let mut a = App { run_id, workspace, selected: 0, checks: HashMap::new(), rows: Vec::new(), results: false, message: String::new(), page: Page::Run, sidebar_open: true, projects, designs, runs, protocols, records };
    a.selected = a.first_open();
    a.reset_rows();
    if let Ok(pg) = std::env::var("ASTERLAB_PAGE") {
        a.page = match pg.as_str() { "design" => Page::Design, "protocol" => Page::Protocol, "tools" => Page::Tools, _ => Page::Run };
    }
    if std::env::var("ASTERLAB_DEBUG_FILL").ok().as_deref() == Some("1") {
        if let Some(row) = a.rows.first_mut() { row.sample = TextEdit::new("Control-1"); row.value = TextEdit::new("0.82"); }
    }
    a
}
impl App {
    fn tasks(&self) -> &[Value] { arr(&self.workspace, "tasks") }
    fn first_open(&self) -> usize { self.tasks().iter().position(|t| s(t, "status") != "completed").unwrap_or(0) }
    fn task(&self) -> Option<&Value> { self.tasks().get(self.selected) }
    fn step(&self) -> Option<&Value> {
        let path = s(self.task()?, "step_path");
        arr(&self.workspace, "steps").iter().find(|x| s(&x["step"], "path") == path).map(|x| &x["step"])
    }
    fn assay(&self) -> Option<&Value> {
        let id = s(&self.task()?["scope"], "assay_id");
        arr(&self.workspace["frozen_design"]["content"], "assays").iter().find(|a| s(a, "id") == id)
    }
    fn endpoint(&self) -> String { self.assay().map(|a| s(a, "endpoint")).unwrap_or_default() }
    fn unit(&self) -> String { self.assay().map(|a| s(a, "unit")).unwrap_or_default() }
    fn protocol(&self) -> String {
        let id = self.task().map(|t| s(&t["scope"], "assay_id")).unwrap_or_default();
        arr(&self.workspace, "scopes").iter().find(|x| s(x, "id") == id).map(|x| s(x, "protocol_version_id")).unwrap_or_default()
    }
    fn reset_rows(&mut self) {
        let sample = self.task().and_then(|t| t["target"].as_array()).and_then(|a| a.first()).map(|t| s(t, "sample_id")).filter(|x| !x.is_empty()).unwrap_or_default();
        self.rows = vec![Row { sample: TextEdit::new(sample), value: TextEdit::new("") }];
    }
    fn events(&self) -> Vec<&Value> {
        let key = self.task().map(|t| t["task_key"].to_string()).unwrap_or_default();
        arr(&self.workspace, "events").iter().filter(|e| e["task_key"].to_string() == key).collect()
    }
    fn measurements(&self) -> Vec<(String, String, String)> {
        let mut out = Vec::new();
        for e in self.events() { for m in arr(e, "measurements") { out.push((s(m, "sample_id"), m["value"].to_string(), s(m, "unit"))); } }
        out
    }
    fn save_measure(&mut self) {
        let Some(task) = self.task() else { self.message = "没有任务".into(); return; };
        let Some(step) = self.step() else { self.message = "没有冻结步骤".into(); return; };
        let mut ms = Vec::new();
        for r in &self.rows {
            let Ok(v) = r.value.text.trim().parse::<f64>() else { continue };
            if !v.is_finite() { continue; }
            ms.push(json!({"endpoint": self.endpoint(), "value": v, "unit": self.unit(), "sample_id": r.sample.text.trim()}));
        }
        if ms.is_empty() { self.message = "至少填写一条有效数值".into(); return; }
        let payload = json!({"run_id":self.run_id,"event_type":"measurement","content":format!("结构化记录 · {}", self.assay().map(|a| s(a,"title")).unwrap_or_else(|| "检测".into())),"protocol_step_path":step["path"],"protocol_version_id":self.protocol(),"assay_id":task["scope"]["assay_id"],"timepoint":task["timepoint"],"task_key":task["task_key"],"measurements":ms});
        match call("execution_event.create", payload) { Ok(_) => { self.message = "读数已保存".into(); self.reload(); }, Err(e) => self.message = format!("保存失败：{e}") }
    }
    fn complete(&mut self) {
        let Some(task) = self.task() else { return; };
        let Some(step) = self.step() else { return; };
        let mut checks = Vec::new();
        let mut blocked = false;
        for c in arr(step, "checks") {
            let id = s(c, "check_id");
            let status = self.checks.get(&id).cloned().unwrap_or_default();
            if c["blocking"].as_bool() == Some(true) && status != "passed" { blocked = true; }
            if !status.is_empty() && status != "未记录" { checks.push(json!({"check_id":id,"status":status})); }
        }
        if blocked { self.message = "必需检查未通过，不能完成本步".into(); return; }
        let payload = json!({"run_id":self.run_id,"event_type":"operation","content":"确认完成","step_status":"completed","protocol_step_path":step["path"],"protocol_version_id":self.protocol(),"assay_id":task["scope"]["assay_id"],"timepoint":task["timepoint"],"task_key":task["task_key"],"checks":checks});
        match call("execution_event.create", payload) { Ok(_) => { self.message = "本步已完成".into(); self.reload(); self.selected = self.first_open(); self.checks.clear(); self.reset_rows(); }, Err(e) => self.message = format!("完成失败：{e}") }
    }
    fn reload(&mut self) { if let Ok(w) = call("run.workspace", json!({"id": self.run_id})) { self.workspace = w; } }
    fn select_design(&mut self, design_id: &str) {
        if let Some(run) = self.runs.iter().find(|r| r["parent_id"].as_str() == Some(design_id)).cloned() {
            if let Ok(w) = call("run.workspace", json!({"id": s(&run, "id")})) {
                self.workspace = w;
                self.run_id = s(&run, "id");
                self.selected = self.first_open();
                self.checks.clear(); self.reset_rows(); self.message.clear();
                return;
            }
        }
        self.message = "这个设计还没有运行".into();
    }
}
fn theme(dark: bool) -> tok::Theme {
    let mut t = tok::theme(dark);
    if dark {
        t.bg = hex("#100f0d").unwrap(); t.card = hex("#201d18").unwrap(); t.card_border = hex("#3a342b").unwrap();
        t.text = hex("#f5f1e9").unwrap(); t.text_secondary = hex("#b8afa2").unwrap();
        t.separator = hex("#332d25").unwrap(); t.hover = hex("#2d2720").unwrap();
    } else {
        t.bg = hex("#f7f6f3").unwrap(); t.card = hex("#ffffff").unwrap(); t.card_border = hex("#e7e3db").unwrap();
        t.text = hex("#1c1b18").unwrap(); t.text_secondary = hex("#6b655b").unwrap();
        t.separator = hex("#e7e3db").unwrap(); t.hover = hex("#efece6").unwrap();
    }
    t
}
fn accent() -> rune::Color { hex("#5b45c9").unwrap() }

fn btn(key: impl Into<String>, label: String, primary: bool, k: &Kit, action: impl Fn(&mut App) + 'static) -> El {
    Button::new(label).variant(if primary { ButtonVariant::Submit } else { ButtonVariant::Neutral })
        .on_press(move |x: &mut App| action(x)).build(k).key(key)
}
fn input(edit: &TextEdit, _key: &str, k: &Kit, access: impl for<'a> Fn(&'a mut App) -> &'a mut TextEdit + 'static) -> El {
    TextInput::new(edit).on_edit(access).build(k)
}
fn status_cn(s: &str) -> &'static str {
    match s { "completed" => "已完成", "not_started" => "未开始", "blocked" => "阻断", "in_progress" => "进行中", _ => "待确认" }
}

static I_DESIGN: RuneIcon = RuneIcon { name: "design", view: 24.0, body: r#"<path d="M4 4h16v16H4z" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M8 9h8M8 13h8" stroke="currentColor" stroke-width="1.5"/>"# };
static I_PROTOCOL: RuneIcon = RuneIcon { name: "protocol", view: 24.0, body: r#"<path d="M6 4h12v16H6z" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M9 8h6M9 12h6M9 16h4" stroke="currentColor" stroke-width="1.5"/>"# };
static I_RUN: RuneIcon = RuneIcon { name: "run", view: 24.0, body: r#"<path d="M8 5l11 7-11 7z" fill="currentColor"/>"# };
static I_SIDEBAR: RuneIcon = RuneIcon { name: "sidebar", view: 24.0, body: r#"<path d="M4 5h16v14H4z" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M9 5v14" stroke="currentColor" stroke-width="1.5"/>"# };
static I_TOOLS: RuneIcon = RuneIcon { name: "tools", view: 24.0, body: r#"<path d="M4 20l8-8M14 6l4 4-6 6-4-4 6-6z" fill="none" stroke="currentColor" stroke-width="1.5"/>"# };

fn view(a: &App, cx: &mut Cx<App>) -> El {
    let t = theme(cx.system_dark());
    let mut k = Kit::from_palette(PaletteSeed { bg: t.bg, fg: t.text, accent: accent() });
    k.c.bg_icon_button_contrast = accent();
    k.c.bg_accent = accent();
    k.c.text_accent = accent();
    k.c.submit_fg = hex("#ffffff").unwrap();
    k.c.submit_icon = hex("#ffffff").unwrap();
    k.c.submit_hover = hex("#4b37ae").unwrap();
    k.c.contrast_hover = hex("#4b37ae").unwrap();
    let title = a.workspace["frozen_design"]["content"]["title"].as_str().unwrap_or("实验运行");
    let status = status_cn(a.workspace["status"].as_str().unwrap_or(""));
    let selected_tp = a.task().and_then(|x| x["timepoint"].as_str()).unwrap_or("").to_owned();
    let mut tps: Vec<(String, usize, usize)> = Vec::new();
    for task in a.tasks() {
        let tp = task["timepoint"].as_str().unwrap_or("").to_owned();
        match tps.iter_mut().find(|(name, _, _)| *name == tp) {
            Some((_, done, total)) => { *total += 1; if s(task, "status") == "completed" { *done += 1; } }
            None => tps.push((tp, if s(task, "status") == "completed" { 1 } else { 0 }, 1)),
        }
    }
    let step_title = a.step().map(|x| s(x, "title")).unwrap_or_else(|| "没有任务".into());
    let instruction = a.step().map(|x| s(x, "instruction")).unwrap_or_default();

    let page = a.page;
    // Rail
    let rail = col().w(56.0).h_full().px(6.0).py(8.0).gap(6.0).fill(t.card).border(1.0, t.card_border)
        .child(text("AL").font(TITLE).color(accent()))
        .child(IconButton::new(&I_DESIGN).label("实验设计").variant(if page==Page::Design { rune_kit::IconButtonVariant::Submit } else { rune_kit::IconButtonVariant::Neutral }).build(&k).on_press(|x: &mut App| x.page = Page::Design))
        .child(IconButton::new(&I_PROTOCOL).label("Protocol").variant(if page==Page::Protocol { rune_kit::IconButtonVariant::Submit } else { rune_kit::IconButtonVariant::Neutral }).build(&k).on_press(|x: &mut App| x.page = Page::Protocol))
        .child(IconButton::new(&I_RUN).label("实验运行").variant(if page==Page::Run { rune_kit::IconButtonVariant::Submit } else { rune_kit::IconButtonVariant::Neutral }).build(&k).on_press(|x: &mut App| x.page = Page::Run))
        .child(IconButton::new(&I_TOOLS).label("资料与工具").variant(if page==Page::Tools { rune_kit::IconButtonVariant::Submit } else { rune_kit::IconButtonVariant::Neutral }).build(&k).on_press(|x: &mut App| x.page = Page::Tools));

    // Side panel
    let current_design = s(&a.workspace, "design_id");
    let mut side = col().w(260.0).h_full().px(12.0).py(12.0).gap(8.0).fill(t.card).border(1.0, t.card_border)
        .child(text("项目 / 实验设计").font(SECTION).color(t.text));
    let mut sidebar = Sidebar::new().label("设计");
    for project in &a.projects {
        let pid = s(project, "id");
        let pname = project["data"]["name"].as_str().unwrap_or("未命名项目").to_owned();
        let rows: Vec<El> = a.designs.iter().filter(|d| d["project_id"].as_str() == Some(pid.as_str())).map(|design| {
            let did = s(design, "id");
            let dname = design["data"]["title"].as_str().unwrap_or("未命名设计").to_owned();
            let selected = did == current_design;
            let runs = a.runs.iter().filter(|r| r["parent_id"].as_str() == Some(did.as_str())).count();
            let did2 = did.clone();
            SidebarRow::new(format!(" {dname}")).detail(if runs == 0 { "尚无运行".into() } else { format!("{runs} 次执行") })
                .selected(selected)
                .on_select(move |x: &mut App, _| x.select_design(&did2))
                .build(&k).key(format!("design-{did}"))
        }).collect();
        let section = SidebarSection::new(pname).collapsed(false).rows(rows);
        sidebar = sidebar.section(section);
    }
    let sidebar_el = sidebar.build(&k);
    side = side.child(sidebar_el.scroll(ScrollSpec { stick_bottom: false }));

    let mut content = col().w_full().grow(1.0).min_w(0.0).gap(12.0).px(16.0).py(16.0);
    if page == Page::Run {
        content = content.child(row().w_full().items_center().gap(space::S3)
            .child(text(title.to_owned()).font(TITLE).color(t.text).wrap())
            .child(el().grow(1.0))
            .child(text(status.to_owned()).font(LABEL).color(t.text_secondary)));
        content = content.child(row().gap(space::S3)
            .child(SegmentedControl::new(&["执行安排", "结果"], Some(if a.results { 1 } else { 0 }))
                .label("视图").on_pick(|x: &mut App, i, _| x.results = i == 1).build(&k)));

        let mut strip = row().w_full().gap(space::S2).wrap();
        for (tp, done, total) in &tps {
            if tp.is_empty() { continue; }
            let active = *tp == selected_tp;
            let tp2 = tp.clone();
            strip = strip.child(btn(format!("tp-{tp}"), format!("{tp} {done}/{total}"), active, &k,
                move |x: &mut App| { if let Some(i) = x.tasks().iter().position(|task| task["timepoint"].as_str().unwrap_or("") == tp2) { x.selected = i; x.checks.clear(); x.reset_rows(); } }));
        }
        content = content.child(strip);

        if a.results {
            let mut r = Card::new().title("结果与测量", Some(&I_RUN));
            let ms = a.measurements();
            if ms.is_empty() { r = r.child(text("暂无测量记录").font(BODY).color(t.text_secondary)); }
            for (sample, value, unit) in ms { r = r.child(row().w_full().gap(space::S2).child(text(sample).w(220.0).font(BODY).color(t.text)).child(text(format!("{value} {unit}")).font(BODY).color(t.text_secondary))); }
            content = content.child(r.build(&k).fill(t.card).border(1.0, t.card_border).radius(12.0));
        } else {
            let mut left = Card::new().title(format!("当前任务 · {step_title}"), Some(&I_RUN)).description(instruction);
            let checks = a.step().map(|s| arr(s, "checks")).unwrap_or(&[]);
            if !checks.is_empty() {
                left = left.child(text("检查").font(LABEL).color(t.text_secondary));
                for c in checks {
                    let id = s(c, "check_id");
                    let cur = a.checks.get(&id).map(String::as_str).unwrap_or("");
                    let idx = match cur { "passed" => 1, "failed" => 2, "skipped" => 3, _ => 0 };
                    let label = format!("{} {}", s(c, "label"), if c["blocking"].as_bool() == Some(true) { "· 必需" } else { "" });
                    let id2 = id.clone();
                    left = left.child(row().w_full().gap(space::S3).items_center()
                        .child(text(label).w(220.0).font(BODY).color(t.text))
                        .child(SegmentedControl::new(&["未记录", "通过", "未通过", "不适用"], Some(idx))
                            .on_pick(move |x: &mut App, i, _| {
                                match i { 0 => { x.checks.remove(&id2); }, 1 => { x.checks.insert(id2.clone(), "passed".into()); }, 2 => { x.checks.insert(id2.clone(), "failed".into()); }, _ => { x.checks.insert(id2.clone(), "skipped".into()); } }
                            }).build(&k).w(300.0)));
                }
            }
            left = left.child(text(format!("记录 {}（{}）", a.endpoint(), a.unit())).font(SECTION).color(t.text));
            left = left.child(row().w_full().gap(space::S2)
                .child(text("样本 / 孔位").w(170.0).font(LABEL).color(t.text_secondary))
                .child(text("数值").w(110.0).font(LABEL).color(t.text_secondary))
                .child(text("单位").font(LABEL).color(t.text_secondary)));
            for (i, r) in a.rows.iter().enumerate() {
                left = left.child(row().w_full().gap(space::S2).items_center()
                    .child(input(&r.sample, "sample", &k, move |x: &mut App| &mut x.rows[i].sample).w(170.0))
                    .child(input(&r.value, "value", &k, move |x: &mut App| &mut x.rows[i].value).w(110.0))
                    .child(text(a.unit()).font(LABEL).color(t.text_secondary)));
            }
            left = left.child(row().gap(space::S2)
                .child(btn("add", "+ 添加行".into(), false, &k, |x: &mut App| x.rows.push(Row { sample: TextEdit::new(""), value: TextEdit::new("") })))
                .child(btn("save", "保存读数".into(), true, &k, |x: &mut App| x.save_measure()))
                .child(btn("complete", "完成本步".into(), false, &k, |x: &mut App| x.complete())));
            if !a.message.is_empty() { left = left.child(text(a.message.clone()).font(LABEL).color(t.text_secondary).wrap()); }
            let materials = arr(&a.workspace, "materials").len();
            let plates = arr(&a.workspace, "plates").len();
            let instruments = arr(&a.workspace, "instruments").len();
            let readiness = |label: &str, value: String| row().w_full().gap(space::S2).items_center()
                .child(text(label.to_owned()).w(88.0).font(LABEL).color(t.text_secondary))
                .child(text(value).font(BODY).color(t.text).wrap());
            let mut right = Card::new().title("就绪信息", Some(&I_TOOLS));
            right = right.child(readiness("材料 / 试剂", if materials == 0 { "未绑定".into() } else { format!("{materials} 项") }))
                .child(readiness("仪器 / 程序", if instruments == 0 { "未绑定".into() } else { format!("{instruments} 项") }))
                .child(readiness("孔板", if plates == 0 { "未绑定".into() } else { format!("{plates} 块") }));
            content = content.child(row().w_full().gap(16.0).items_start().wrap()
                .child(left.build(&k).fill(t.card).border(1.0, t.card_border).radius(12.0).w(560.0).min_w(0.0))
                .child(right.build(&k).fill(t.card).border(1.0, t.card_border).radius(12.0).w(240.0).min_w(240.0)));
        }
    } else if page == Page::Design {
        let mut c = Card::new().title("设计", Some(&I_DESIGN));
        if a.designs.is_empty() { c = c.child(text("暂无设计").font(BODY).color(t.text_secondary)); }
        for d in &a.designs {
            let name = d["data"]["title"].as_str().unwrap_or("未命名设计");
            let n = d["data"]["assays"].as_array().map(|x| x.len()).unwrap_or(0);
            c = c.child(text(format!("{name} · {n} 个检测")).font(BODY).color(t.text).wrap());
        }
        content = content.child(c.build(&k).fill(t.card).border(1.0, t.card_border).radius(12.0));
    } else if page == Page::Protocol {
        let mut c = Card::new().title("Protocol", Some(&I_PROTOCOL));
        if a.protocols.is_empty() { c = c.child(text("暂无 Protocol").font(BODY).color(t.text_secondary)); }
        for p in &a.protocols {
            let name = p["data"]["title"].as_str().unwrap_or("未命名 Protocol");
            let rev = p["revision"].as_i64().unwrap_or(1);
            c = c.child(text(format!("{name} · v{rev}")).font(BODY).color(t.text).wrap());
        }
        content = content.child(c.build(&k).fill(t.card).border(1.0, t.card_border).radius(12.0));
    } else {
        let mut c = Card::new().title("记录", Some(&I_TOOLS));
        if a.records.is_empty() { c = c.child(text("暂无记录").font(BODY).color(t.text_secondary)); }
        for r in a.records.iter().take(20) {
            let et = r["data"]["event_type"].as_str().unwrap_or("记录");
            let content_text = r["data"]["content"].as_str().unwrap_or("");
            c = c.child(text(format!("{et} · {content_text}")).font(BODY).color(t.text).wrap());
        }
        content = content.child(c.build(&k).fill(t.card).border(1.0, t.card_border).radius(12.0));
    }
    let main_area = col().grow(1.0).min_w(0.0).min_h(0.0).scroll(ScrollSpec { stick_bottom: false }).child(content);
    let insets = cx.chrome_insets();
    let top = row().w_full().h(40.0).px(14.0).pr(14.0 + insets.trailing)
        .items_center().gap(space::S2).fill(t.card).border(1.0, t.card_border)
        .window_drag()
        .child(text("AsterLab").font(SECTION).color(t.text))
        .child(IconButton::new(&I_SIDEBAR).label("侧栏").variant(rune_kit::IconButtonVariant::Ghost).build(&k).on_press(|x: &mut App| x.sidebar_open = !x.sidebar_open))
        .child(el().grow(1.0))
        .child(text(match page { Page::Design=>"设计", Page::Protocol=>"Protocol", Page::Run=>"实验运行", Page::Tools=>"资料与工具" }).font(LABEL).color(t.text_secondary));
    let body = row().w_full().grow(1.0).min_h(0.0)
        .child(rail);
    let mut body = body;
    if a.sidebar_open { body = body.child(side); }
    let body = body.child(main_area);
    col().w_full().h_full().fill(t.bg).child(top).child(body)
}

fn main() {
    let mut spec = AppSpec::new("AsterLab Run · Gate 2", (1200.0, 760.0), init, view);
    spec.families = ("Inter, Segoe UI, Microsoft YaHei", "Cascadia Mono, Consolas");
    spec.min_size = Some((900.0, 600.0));
    rune::App::new(spec).titlebar(rune::Titlebar::new(40.0)).run();
}