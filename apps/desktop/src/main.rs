//! 原生桌面验证原型入口（P1 spike）。
//!
//! 只读打开调用方显式传入的数据库并做内存内展示；不创建目录/结构、不写入。

mod data;

use data::ReadModel;
use slint::{ModelRc, SharedString, VecModel};
use std::cell::RefCell;
use std::collections::HashMap;
use std::path::PathBuf;
use std::rc::Rc;

slint::include_modules!();

#[derive(Default, Clone)]
struct Draft {
    comparison: Option<String>,
    experimental_unit: Option<String>,
    relations: HashMap<String, String>,
    group_n: HashMap<String, Option<u32>>,
    group_technical: HashMap<String, Option<u32>>,
}

impl Draft {
    fn touched(&self) -> bool {
        self.comparison.is_some()
            || self.experimental_unit.is_some()
            || !self.relations.is_empty()
            || !self.group_n.is_empty()
            || !self.group_technical.is_empty()
    }
}

#[derive(Default, Clone)]
struct Selection {
    project_id: String,
    design_id: String,
    run_id: String,
    assay_id: String,
    group_id: String,
    plate_id: String,
    well_id: String,
    mode: String,
    draft: Draft,
}

const GROUP_W: f32 = 232.0;
const GROUP_H: f32 = 64.0;
const GROUP_GAP: f32 = 10.0;
const ASSAY_X: f32 = 328.0;
const ASSAY_W: f32 = 296.0;
const ASSAY_H: f32 = 96.0;
const ASSAY_GAP: f32 = 12.0;
const LINK_X: f32 = 280.0;

fn print_help() {
    println!("labworkbench-desktop（原生 UI 验证原型，只读）");
    println!();
    println!("用法：");
    println!("  labworkbench-desktop --db <数据库路径> [--renderer femtovg|software]");
    println!();
    println!("说明：");
    println!("  --db       必填，显式传入要打开的 SQLite 数据库，不做任何默认或创建。");
    println!("  --renderer 渲染器二选一：femtovg（默认）或 software。");
    println!("  -h/--help  显示本帮助。");
}

fn parse_args() -> (PathBuf, String) {
    let mut db: Option<String> = None;
    let mut renderer = String::from("femtovg");
    let mut args = std::env::args().skip(1);
    while let Some(arg) = args.next() {
        if let Some(value) = arg.strip_prefix("--db=") {
            db = Some(value.to_string());
        } else if let Some(value) = arg.strip_prefix("--renderer=") {
            renderer = value.to_string();
        } else {
            match arg.as_str() {
                "--db" => match args.next() {
                    Some(value) => db = Some(value),
                    None => {
                        eprintln!("错误：--db 缺少路径参数。");
                        print_help();
                        std::process::exit(2);
                    }
                },
                "--renderer" => match args.next() {
                    Some(value) => renderer = value,
                    None => {
                        eprintln!("错误：--renderer 缺少取值（femtovg|software）。");
                        print_help();
                        std::process::exit(2);
                    }
                },
                "-h" | "--help" => {
                    print_help();
                    std::process::exit(0);
                }
                _ => {
                    eprintln!("错误：未知参数 {arg}。");
                    print_help();
                    std::process::exit(2);
                }
            }
        }
    }
    let db = match db {
        Some(path) if !path.trim().is_empty() => path,
        _ => {
            eprintln!("错误：必须显式传入 --db <数据库路径>，不会使用或创建默认数据库。");
            print_help();
            std::process::exit(2);
        }
    };
    if renderer != "femtovg" && renderer != "software" {
        eprintln!("错误：--renderer 仅支持 femtovg 或 software，当前为 {renderer}。");
        print_help();
        std::process::exit(2);
    }
    (PathBuf::from(db), renderer)
}

fn select_backend(renderer: &str) {
    let selector = slint::BackendSelector::new()
        .backend_name("winit".into())
        .renderer_name(renderer.into());
    if let Err(err) = selector.select() {
        eprintln!("图形后端初始化失败：{err:?}");
        std::process::exit(1);
    }
}

fn missing() -> SharedString {
    SharedString::from("未记录")
}

fn unconfirmed() -> SharedString {
    SharedString::from("待确认")
}

fn present(value: &Option<String>) -> bool {
    value.as_ref().is_some_and(|s| !s.trim().is_empty())
}

fn opt_text(value: &Option<String>) -> SharedString {
    match value {
        Some(s) if !s.trim().is_empty() => SharedString::from(s.as_str()),
        _ => missing(),
    }
}

fn join_nonempty(values: &[String], separator: &str) -> Option<String> {
    let parts: Vec<&str> = values
        .iter()
        .map(String::as_str)
        .filter(|value| !value.trim().is_empty())
        .collect();
    if parts.is_empty() {
        None
    } else {
        Some(parts.join(separator))
    }
}

fn relation_text(value: &Option<String>) -> SharedString {
    match value.as_deref() {
        Some("same_experimental_unit_repeated") => SharedString::from("同一实验单位重复测量"),
        Some("same_physical_sample_repeated") => SharedString::from("同一份物理样本重复测量"),
        Some("independent_sample_per_timepoint") => SharedString::from("各时间点独立取样"),
        Some(value) if !value.trim().is_empty() => {
            SharedString::from(format!("待确认（原值：{value}）"))
        }
        _ => unconfirmed(),
    }
}

fn run_status_text(value: &Option<String>) -> SharedString {
    match value.as_deref() {
        Some("planned") => SharedString::from("计划中"),
        Some("in_progress") => SharedString::from("进行中"),
        Some("completed") => SharedString::from("已完成"),
        Some("blocked") => SharedString::from("已阻塞"),
        Some("cancelled") => SharedString::from("已取消"),
        Some(value) if !value.trim().is_empty() => {
            SharedString::from(format!("待确认（原值：{value}）"))
        }
        _ => unconfirmed(),
    }
}

fn conflict_status_text(value: &Option<String>) -> SharedString {
    match value.as_deref() {
        Some("unresolved") => SharedString::from("待核对"),
        Some("resolved") => SharedString::from("已说明"),
        Some(value) if !value.trim().is_empty() => {
            SharedString::from(format!("待确认（原值：{value}）"))
        }
        _ => unconfirmed(),
    }
}

fn group_role_text(value: &str) -> SharedString {
    match value {
        "control" => SharedString::from("对照"),
        "treatment" => SharedString::from("处理"),
        "comparator" => SharedString::from("比较组"),
        "reference" => SharedString::from("参照组"),
        "other" => SharedString::from("其他"),
        "" => missing(),
        _ => SharedString::from(value),
    }
}

fn format_label(format: &str) -> String {
    let trimmed = format.trim();
    if trimmed.is_empty() {
        "规格未记录".to_owned()
    } else if trimmed.contains('孔') {
        trimmed.to_owned()
    } else {
        format!("{trimmed} 孔板")
    }
}

fn scope_text(value: &Option<String>) -> String {
    match value.as_deref() {
        Some("per_timepoint") => "每时间点独立位置".to_owned(),
        Some("whole_assay_unique_positions") => "全板唯一位置".to_owned(),
        Some(value) if !value.trim().is_empty() => format!("待确认（原值：{value}）"),
        _ => "范围待确认".to_owned(),
    }
}

fn model_rc<T: Clone + Send + 'static>(items: Vec<T>) -> ModelRc<T> {
    Rc::new(VecModel::from(items)).into()
}

fn well_mapped(well: &data::Well) -> bool {
    present(&well.group_label) || present(&well.sample_id) || present(&well.note)
}

fn mapped_well_count(plate: &data::Plate) -> i32 {
    plate.wells.iter().filter(|w| well_mapped(w)).count() as i32
}

/// Spreadsheet row letters; shared by coordinates and plate axis labels.
fn row_letters(row: u32) -> String {
    let mut index = u64::from(row) + 1;
    let mut label = [0u8; 8];
    let mut start = label.len();
    while index > 0 {
        start -= 1;
        label[start] = b'A' + ((index - 1) % 26) as u8;
        index = (index - 1) / 26;
    }
    std::str::from_utf8(&label[start..])
        .expect("ASCII row labels")
        .to_owned()
}

fn coordinate(row: u32, column: u32) -> String {
    format!("{}{}", row_letters(row), u64::from(column) + 1)
}

fn section(label: &str) -> InspectorItem {
    InspectorItem {
        kind: SharedString::from("section"),
        label: SharedString::from(label),
        value: SharedString::from(""),
        missing: false,
    }
}

fn field(label: &str, value: Option<String>) -> InspectorItem {
    let present = value.as_ref().is_some_and(|text| !text.trim().is_empty());
    InspectorItem {
        kind: SharedString::from("field"),
        label: SharedString::from(label),
        value: match value {
            Some(text) if present => SharedString::from(text),
            _ => missing(),
        },
        missing: !present,
    }
}

fn field_state(label: &str, value: &str, missing_value: bool) -> InspectorItem {
    InspectorItem {
        kind: SharedString::from("field"),
        label: SharedString::from(label),
        value: SharedString::from(value),
        missing: missing_value,
    }
}

fn assay_groups<'a>(design: &'a data::Design, assay: &data::Assay) -> Vec<&'a data::Group> {
    let ids: Vec<&str> = assay
        .group_ids
        .iter()
        .map(String::as_str)
        .filter(|id| !id.is_empty())
        .collect();
    if !ids.is_empty() {
        design
            .groups
            .iter()
            .filter(|group| ids.contains(&group.id.as_str()))
            .collect()
    } else if !assay.group_names.is_empty() {
        design
            .groups
            .iter()
            .filter(|group| assay.group_names.iter().any(|name| name == &group.name))
            .collect()
    } else {
        Vec::new()
    }
}

fn assay_title(model: &ReadModel, assay_id: &Option<String>) -> Option<String> {
    let id = assay_id.as_ref()?;
    model.designs.iter().find_map(|design| {
        design
            .assays
            .iter()
            .find(|assay| &assay.id == id)
            .map(|assay| assay.title.clone())
    })
}

fn clear_inspector(app: &AppWindow) {
    app.set_inspector_kind(SharedString::from(""));
    app.set_inspector_title(SharedString::from("未选择对象"));
    app.set_inspector_subtitle(SharedString::from(""));
    app.set_inspector_fields(model_rc(Vec::<InspectorItem>::new()));
}

fn clear_assay(app: &AppWindow, sel: &mut Selection) {
    sel.assay_id.clear();
    app.set_selected_assay_id(SharedString::from(""));
}

fn clear_plate(app: &AppWindow, sel: &mut Selection) {
    sel.plate_id.clear();
    sel.well_id.clear();
    app.set_selected_plate_id(SharedString::from(""));
    app.set_selected_well_id(SharedString::from(""));
    app.set_has_plate(false);
    app.set_plate_name(SharedString::from(""));
    app.set_plate_meta(SharedString::from(""));
    app.set_plate_rows(0);
    app.set_plate_columns(0);
    app.set_plate_mapped_count(0);
    app.set_wells(model_rc(Vec::<WellCell>::new()));
    app.set_row_labels(model_rc(Vec::<AxisLabel>::new()));
    app.set_column_labels(model_rc(Vec::<AxisLabel>::new()));
}

fn clear_design(app: &AppWindow, sel: &mut Selection) {
    sel.design_id.clear();
    sel.run_id.clear();
    sel.assay_id.clear();
    sel.group_id.clear();
    sel.plate_id.clear();
    sel.well_id.clear();
    sel.mode = "design".to_owned();
    sel.draft = Draft::default();
    app.set_mode(SharedString::from("design"));
    app.set_selected_design_id(SharedString::from(""));
    app.set_selected_run_id(SharedString::from(""));
    app.set_selected_assay_id(SharedString::from(""));
    app.set_selected_group_id(SharedString::from(""));
    app.set_runs(model_rc(Vec::<RunEntry>::new()));
    app.set_design_title(SharedString::from(""));
    app.set_design_objective(missing());
    app.set_experimental_unit(unconfirmed());
    app.set_run_title(SharedString::from(""));
    app.set_run_status(unconfirmed());
    app.set_group_count(0);
    app.set_assay_count(0);
    app.set_map_n_text(SharedString::from("独立 n 待确认"));
    app.set_map_protocol_text(SharedString::from("Protocol 未关联"));
    app.set_map_time_text(SharedString::from("时间点待确认"));
    app.set_map_sample_text(SharedString::from("样本关系待确认"));
    app.set_map_attention(model_rc(Vec::<CheckDetail>::new()));
    app.set_group_nodes(model_rc(Vec::<GroupNodeData>::new()));
    app.set_assay_nodes(model_rc(Vec::<AssayNodeData>::new()));
    app.set_link_segments(model_rc(Vec::<LinkSegment>::new()));
    app.set_canvas_width(0.0);
    app.set_canvas_height(0.0);
    app.set_plates(model_rc(Vec::<PlateEntry>::new()));
    clear_plate(app, sel);
    clear_inspector(app);
}

/// Design mode reads the current Design record. Prepare/Execute read the
/// snapshot frozen by the selected Run; if it is missing, no design is shown.
fn active_design<'a>(model: &'a ReadModel, sel: &Selection) -> Option<&'a data::Design> {
    if sel.mode == "prepare" || sel.mode == "execute" {
        let run = model.runs.iter().find(|run| run.id == sel.run_id)?;
        let version_id = run.design_version_id.as_ref()?;
        return model
            .design_versions
            .iter()
            .find(|version| &version.id == version_id)
            .map(|version| &version.snapshot);
    }
    model
        .designs
        .iter()
        .find(|design| design.id == sel.design_id)
}

/// 画布坐标与连线由 Rust 计算：节点位置确定，Slint 不重复推理布局。
fn refresh_canvas(app: &AppWindow, model: &ReadModel, sel: &Selection) {
    let design = active_design(model, sel);
    let Some(design) = design else {
        app.set_group_nodes(model_rc(Vec::<GroupNodeData>::new()));
        app.set_assay_nodes(model_rc(Vec::<AssayNodeData>::new()));
        app.set_link_segments(model_rc(Vec::<LinkSegment>::new()));
        app.set_group_count(0);
        app.set_assay_count(0);
        app.set_canvas_width(0.0);
        app.set_canvas_height(0.0);
        return;
    };

    let selected_assay = design.assays.iter().find(|assay| assay.id == sel.assay_id);
    let linked_ids: Vec<&str> = selected_assay
        .map(|assay| {
            assay_groups(design, assay)
                .iter()
                .map(|group| group.id.as_str())
                .collect()
        })
        .unwrap_or_default();

    let groups = design
        .groups
        .iter()
        .enumerate()
        .map(|(index, group)| GroupNodeData {
            id: SharedString::from(group.id.as_str()),
            node_name: SharedString::from(group.name.as_str()),
            node_role: group_role_text(&group.role),
            linked: linked_ids.contains(&group.id.as_str()),
            selected: group.id == sel.group_id,
            x: 0.0,
            y: index as f32 * (GROUP_H + GROUP_GAP),
            w: GROUP_W,
            h: GROUP_H,
        })
        .collect::<Vec<_>>();

    let assays = design
        .assays
        .iter()
        .enumerate()
        .map(|(index, assay)| {
            let groups = assay_groups(design, assay);
            let timepoints = join_nonempty(&assay.timepoints, " · ");
            AssayNodeData {
                id: SharedString::from(assay.id.as_str()),
                node_title: SharedString::from(assay.title.as_str()),
                node_method: opt_text(&assay.method),
                node_timepoints: match &timepoints {
                    Some(text) => SharedString::from(text.as_str()),
                    None => SharedString::from("时间点未记录"),
                },
                has_timepoints: timepoints.is_some(),
                node_group_summary: if groups.is_empty() {
                    SharedString::from("组别未明确")
                } else {
                    SharedString::from(format!("{} 组", groups.len()))
                },
                has_groups: !groups.is_empty(),
                selected: assay.id == sel.assay_id,
                x: ASSAY_X,
                y: index as f32 * (ASSAY_H + ASSAY_GAP),
                w: ASSAY_W,
                h: ASSAY_H,
            }
        })
        .collect::<Vec<_>>();

    let mut segments = Vec::new();
    if let Some(assay) = selected_assay {
        if let Some(assay_index) = design.assays.iter().position(|item| item.id == assay.id) {
            let assay_center = assay_index as f32 * (ASSAY_H + ASSAY_GAP) + ASSAY_H / 2.0;
            for (index, group) in design.groups.iter().enumerate() {
                if !linked_ids.contains(&group.id.as_str()) {
                    continue;
                }
                let group_center = index as f32 * (GROUP_H + GROUP_GAP) + GROUP_H / 2.0;
                segments.push(LinkSegment {
                    x: GROUP_W,
                    y: group_center - 0.5,
                    w: LINK_X - GROUP_W,
                    h: 1.0,
                });
                let top = group_center.min(assay_center);
                let bottom = group_center.max(assay_center);
                segments.push(LinkSegment {
                    x: LINK_X - 0.5,
                    y: top,
                    w: 1.0,
                    h: bottom - top,
                });
                segments.push(LinkSegment {
                    x: LINK_X,
                    y: assay_center - 0.5,
                    w: ASSAY_X - LINK_X,
                    h: 1.0,
                });
            }
        }
    }

    let groups_height = design.groups.len() as f32 * (GROUP_H + GROUP_GAP) - GROUP_GAP;
    let assays_height = design.assays.len() as f32 * (ASSAY_H + ASSAY_GAP) - ASSAY_GAP;
    let canvas_height = groups_height.max(assays_height).max(0.0);

    app.set_group_nodes(model_rc(groups));
    app.set_assay_nodes(model_rc(assays));
    app.set_link_segments(model_rc(segments));
    app.set_group_count(design.groups.len() as i32);
    app.set_assay_count(design.assays.len() as i32);
    app.set_canvas_width(ASSAY_X + ASSAY_W);
    app.set_canvas_height(canvas_height);
    app.set_assay_column_x(ASSAY_X);
}

fn set_inspector_assay(app: &AppWindow, model: &ReadModel, sel: &Selection) {
    let Some(design) = active_design(model, sel) else {
        clear_inspector(app);
        return;
    };
    let Some(assay) = design.assays.iter().find(|assay| assay.id == sel.assay_id) else {
        clear_inspector(app);
        return;
    };
    let groups = assay_groups(design, assay);
    app.set_inspector_kind(SharedString::from("检测项目"));
    app.set_inspector_title(SharedString::from(assay.title.as_str()));
    app.set_inspector_subtitle(match join_nonempty(&assay.timepoints, " · ") {
        Some(text) => SharedString::from(format!("时间点 · {text}")),
        None => SharedString::from("时间点未记录"),
    });

    let mut items = vec![section("检测参数")];
    items.push(field("终点", assay.endpoint.clone()));
    items.push(field("方法", assay.method.clone()));
    items.push(field("单位", assay.unit.clone()));
    items.push(field("容器", assay.container_format.clone()));
    items.push(field("时间基准", assay.time_anchor_event.clone()));
    items.push(field("时间点", join_nonempty(&assay.timepoints, " · ")));
    items.push(field_state(
        "时间点关系",
        &relation_text(&assay.timepoint_relation),
        !present(&assay.timepoint_relation),
    ));

    items.push(section("组别与计划孔位"));
    if !assay.allocations.is_empty() {
        for allocation in &assay.allocations {
            let label = if allocation.group_name.trim().is_empty() {
                "未命名组别"
            } else {
                allocation.group_name.as_str()
            };
            match allocation.slots_per_group {
                Some(slots) => items.push(field_state(
                    label,
                    &format!("每组 {slots} 孔 · {}", scope_text(&allocation.slots_scope)),
                    false,
                )),
                None => items.push(field_state(label, "计划孔位未记录", true)),
            }
        }
    } else if !groups.is_empty() {
        for group in &groups {
            items.push(field_state(&group.name, "计划孔位未记录", true));
        }
    } else {
        items.push(field("组别关联", None));
    }

    items.push(section("来源与说明"));
    let source = assay
        .evidence_id
        .as_ref()
        .and_then(|id| design.sources.iter().find(|source| &source.id == id));
    items.push(field(
        "来源文档",
        source.and_then(|source| source.source_document.clone()),
    ));
    items.push(field(
        "证据定位",
        source.and_then(|source| source.source_locator.clone()),
    ));
    items.push(field(
        "证据说明",
        source.and_then(|source| source.claim.clone()),
    ));
    items.push(field("控制说明", join_nonempty(&assay.controls, "；")));
    for conflict in &design.conflicts {
        let status = conflict_status_text(&conflict.status);
        let value = if conflict.evidence_ids.is_empty() {
            status.to_string()
        } else {
            format!("{status}（{}）", conflict.evidence_ids.join("、"))
        };
        items.push(field_state(
            &format!("分歧 · {}", conflict.topic),
            &value,
            !present(&conflict.status),
        ));
    }

    items.push(section("执行"));
    let run = model
        .runs
        .iter()
        .find(|run| run.id == sel.run_id)
        .or_else(|| {
            model
                .runs
                .iter()
                .find(|run| run.project_id == design.project_id && run.design_id == design.id)
        });
    match run {
        Some(run) => {
            items.push(field("运行", Some(run.title.clone())));
            items.push(field_state(
                "状态",
                &run_status_text(&run.status),
                !present(&run.status),
            ));
        }
        None => {
            items.push(field("运行", None));
            items.push(field("状态", None));
        }
    }

    app.set_inspector_fields(model_rc(items));
}

fn set_inspector_group(app: &AppWindow, model: &ReadModel, sel: &Selection) {
    let Some(design) = active_design(model, sel) else {
        clear_inspector(app);
        return;
    };
    let Some(group) = design.groups.iter().find(|group| group.id == sel.group_id) else {
        clear_inspector(app);
        return;
    };
    app.set_inspector_kind(SharedString::from("处理组"));
    app.set_inspector_title(SharedString::from(group.name.as_str()));
    app.set_inspector_subtitle(SharedString::from(format!(
        "{} · 组别条件与检测关系",
        group_role_text(&group.role)
    )));

    let mut items = vec![section("组别")];
    items.push(field_state("名称", &group.name, false));
    items.push(field_state(
        "角色",
        &group_role_text(&group.role),
        group.role.trim().is_empty(),
    ));
    items.push(field("条件", group.condition.clone()));

    items.push(section("关联检测"));
    let mut linked = 0;
    for assay in &design.assays {
        if !assay_groups(design, assay)
            .iter()
            .any(|item| item.id == group.id)
        {
            continue;
        }
        linked += 1;
        let allocation = assay
            .allocations
            .iter()
            .find(|allocation| allocation.group_id == group.id);
        let value = match allocation.and_then(|allocation| allocation.slots_per_group) {
            Some(slots) => format!("每组 {slots} 孔"),
            None => "计划孔位未记录".to_owned(),
        };
        items.push(field_state(&assay.title, &value, allocation.is_none()));
    }
    if linked == 0 {
        items.push(field("关联检测", None));
    }

    app.set_inspector_fields(model_rc(items));
}

fn set_inspector_plate(app: &AppWindow, model: &ReadModel, sel: &Selection) {
    let Some(plate) = model.plates.iter().find(|plate| plate.id == sel.plate_id) else {
        clear_inspector(app);
        return;
    };
    app.set_inspector_kind(SharedString::from("培养板"));
    app.set_inspector_title(SharedString::from(plate.name.as_str()));
    app.set_inspector_subtitle(SharedString::from(format!(
        "{} 孔板 · {} 行 × {} 列",
        plate.format, plate.rows, plate.columns
    )));

    let items = vec![
        section("培养板"),
        field("规格", Some(format_label(&plate.format))),
        field("时间点", plate.timepoint.clone()),
        field("关联检测", assay_title(model, &plate.assay_id)),
        field_state(
            "已映射孔数",
            &format!(
                "{} / {}",
                mapped_well_count(plate),
                plate.rows * plate.columns
            ),
            false,
        ),
        section("查看孔位"),
        field_state("提示", "展开培养板并选择孔位，查看该孔的记录。", false),
    ];
    app.set_inspector_fields(model_rc(items));
}

fn set_inspector_well(app: &AppWindow, model: &ReadModel, sel: &Selection) {
    let Some(plate) = model.plates.iter().find(|plate| plate.id == sel.plate_id) else {
        clear_inspector(app);
        return;
    };
    if sel.well_id.is_empty() {
        set_inspector_plate(app, model, sel);
        return;
    }
    let stored = plate
        .wells
        .iter()
        .find(|well| well.coordinate.eq_ignore_ascii_case(&sel.well_id));
    app.set_inspector_kind(SharedString::from("孔位"));
    app.set_inspector_title(SharedString::from(format!("孔位 {}", sel.well_id)));
    app.set_inspector_subtitle(SharedString::from(plate.name.as_str()));

    let mapped = stored.is_some_and(well_mapped);
    let items = vec![
        section("孔位记录"),
        field_state("坐标", &sel.well_id, false),
        field_state(
            "记录状态",
            if mapped {
                "已记录映射"
            } else {
                "未记录映射"
            },
            !mapped,
        ),
        field("分组标记", stored.and_then(|well| well.group_label.clone())),
        field("样本编号", stored.and_then(|well| well.sample_id.clone())),
        field("备注", stored.and_then(|well| well.note.clone())),
        section("所在培养板"),
        field("名称", Some(plate.name.clone())),
        field("规格", Some(format_label(&plate.format))),
        field("时间点", plate.timepoint.clone()),
        field("关联检测", assay_title(model, &plate.assay_id)),
        field_state(
            "已映射孔数",
            &format!(
                "{} / {}",
                mapped_well_count(plate),
                plate.rows * plate.columns
            ),
            false,
        ),
    ];
    app.set_inspector_fields(model_rc(items));
}

fn evidence_state_text(value: &Option<String>) -> String {
    match value.as_deref() {
        Some("source_record") => "原始记录".to_owned(),
        Some("proposal") => "建议".to_owned(),
        Some("user_confirmed") => "用户确认".to_owned(),
        Some("calculated") => "程序推导".to_owned(),
        Some("agent_suggestion") => "Agent 建议".to_owned(),
        Some(value) if !value.trim().is_empty() => format!("待确认（原值：{value}）"),
        _ => "来源状态未记录".to_owned(),
    }
}

fn source_relation_text(value: &Option<String>) -> String {
    match value.as_deref() {
        Some("not_applicable") => "无跨检测来源".to_owned(),
        Some("split_from_assay") => "从其他检测分样".to_owned(),
        Some("sequential_after_assay") => "在其他检测之后顺序使用".to_owned(),
        Some(value) if !value.trim().is_empty() => format!("待确认（原值：{value}）"),
        _ => "来源待确认".to_owned(),
    }
}

fn assay_title_in(design: &data::Design, id: &str) -> String {
    design
        .assays
        .iter()
        .find(|assay| assay.id == id)
        .map(|assay| assay.title.clone())
        .unwrap_or_else(|| id.to_owned())
}

fn comparison_text(value: &str) -> String {
    match value {
        "treatment_vs_control" => "处理组 vs 对照".to_owned(),
        "main_effect" => "因素主效应".to_owned(),
        "interaction" => "交互作用".to_owned(),
        "unknown" => "待确认".to_owned(),
        _ => value.to_owned(),
    }
}

fn unit_text(value: &str) -> String {
    match value {
        "independent_culture" => "独立培养单位".to_owned(),
        "technical_well" => "技术孔".to_owned(),
        "animal" => "动物 / 个体".to_owned(),
        "unknown" => "待确认".to_owned(),
        _ => value.to_owned(),
    }
}

fn unit_missing(value: &Option<String>) -> bool {
    value.as_ref().is_none_or(|text| {
        let text = text.trim();
        text.is_empty() || text == "未记录" || text.starts_with("待确认")
    })
}

fn design_projection(design: &data::Design) -> Option<asterlab_core::design_map::CurrentDesignMap> {
    asterlab_core::design_map::build_current(&design.id, design.revision, &design.raw_data).ok()
}

fn map_summary(
    map: &asterlab_core::design_map::CurrentDesignMap,
) -> (String, String, String, String) {
    let content = &map.content;
    let n_missing = content.experimental_unit.is_none()
        || content
            .groups
            .iter()
            .any(|group| group.independent_n.is_none());
    let protocol_missing = content
        .assays
        .iter()
        .any(|assay| assay.protocol_version_id.is_none());
    let time_missing = content
        .assays
        .iter()
        .any(|assay| assay.timepoints.is_empty());
    let sample_missing = content.assays.iter().any(|assay| {
        assay.timepoint_relation.is_none()
            || matches!(assay.timepoint_relation.as_deref(), Some("unknown"))
    });
    (
        if n_missing {
            "独立 n 待确认"
        } else {
            "独立 n 已记录"
        }
        .to_owned(),
        if protocol_missing {
            "Protocol 未关联"
        } else {
            "Protocol 已关联"
        }
        .to_owned(),
        if time_missing {
            "时间点待确认"
        } else {
            "时间点已记录"
        }
        .to_owned(),
        if sample_missing {
            "样本关系待确认"
        } else {
            "样本关系已记录"
        }
        .to_owned(),
    )
}

fn map_attention(map: &asterlab_core::design_map::CurrentDesignMap) -> Vec<CheckDetail> {
    map.content
        .unresolved
        .iter()
        .take(6)
        .map(|issue| CheckDetail {
            level: SharedString::from("warn"),
            title: SharedString::from(issue.question.as_str()),
            detail: SharedString::from(format!(
                "影响：{}",
                if issue.affects.is_empty() {
                    "尚未说明".to_owned()
                } else {
                    issue.affects.join("、")
                }
            )),
        })
        .collect()
}

fn set_question(app: &AppWindow, design: &data::Design, sel: &Selection) {
    let clear = |app: &AppWindow| {
        app.set_question_kind(SharedString::from(""));
        app.set_question_title(SharedString::from(""));
        app.set_question_detail(SharedString::from(""));
        app.set_question_options(model_rc(Vec::<QuestionOption>::new()));
    };

    if sel.draft.comparison.is_none() {
        app.set_question_kind(SharedString::from("comparison"));
        app.set_question_title(SharedString::from("主要比较是什么？"));
        app.set_question_detail(SharedString::from(
            "组别存在不等于比较目标明确。先选当前最想回答的比较，界面只更新本地草案。",
        ));
        app.set_question_options(model_rc(vec![
            QuestionOption {
                label: SharedString::from("处理 vs 对照"),
                value: SharedString::from("treatment_vs_control"),
            },
            QuestionOption {
                label: SharedString::from("因素主效应"),
                value: SharedString::from("main_effect"),
            },
            QuestionOption {
                label: SharedString::from("交互作用"),
                value: SharedString::from("interaction"),
            },
            QuestionOption {
                label: SharedString::from("暂不确定"),
                value: SharedString::from("unknown"),
            },
        ]));
        return;
    }

    if unit_missing(&design.experimental_unit) && sel.draft.experimental_unit.is_none() {
        app.set_question_kind(SharedString::from("unit"));
        app.set_question_title(SharedString::from("每组 n 对应什么单位？"));
        app.set_question_detail(SharedString::from(
            "独立实验单位不能按孔数或技术重复代替；先确认这句话里的“一个样本”指什么。",
        ));
        app.set_question_options(model_rc(vec![
            QuestionOption {
                label: SharedString::from("独立培养"),
                value: SharedString::from("independent_culture"),
            },
            QuestionOption {
                label: SharedString::from("技术孔"),
                value: SharedString::from("technical_well"),
            },
            QuestionOption {
                label: SharedString::from("动物 / 个体"),
                value: SharedString::from("animal"),
            },
            QuestionOption {
                label: SharedString::from("暂不确定"),
                value: SharedString::from("unknown"),
            },
        ]));
        return;
    }

    if let Some(assay) = design.assays.iter().find(|assay| {
        let relation = assay.timepoint_relation.as_deref();
        (relation.is_none() || relation == Some("unknown"))
            && !sel.draft.relations.contains_key(&assay.id)
    }) {
        let prefix = format!("{}|", assay.id);
        app.set_question_kind(SharedString::from("relation"));
        app.set_question_title(SharedString::from(format!(
            "{} 的各时间点如何取样？",
            assay.title
        )));
        app.set_question_detail(SharedString::from(
            "这决定样本需求、板数和重复关系；无法判断时保留“待确认”。",
        ));
        app.set_question_options(model_rc(vec![
            QuestionOption {
                label: SharedString::from("同一单位"),
                value: SharedString::from(format!("{prefix}same_experimental_unit_repeated")),
            },
            QuestionOption {
                label: SharedString::from("同一样本"),
                value: SharedString::from(format!("{prefix}same_physical_sample_repeated")),
            },
            QuestionOption {
                label: SharedString::from("独立取样"),
                value: SharedString::from(format!("{prefix}independent_sample_per_timepoint")),
            },
            QuestionOption {
                label: SharedString::from("暂不确定"),
                value: SharedString::from(format!("{prefix}unknown")),
            },
        ]));
        return;
    }

    clear(app);
}

fn answer_question(
    app: &AppWindow,
    model: &ReadModel,
    sel: &mut Selection,
    kind: &str,
    value: &str,
) {
    match kind {
        "comparison" => sel.draft.comparison = Some(value.to_owned()),
        "unit" => sel.draft.experimental_unit = Some(value.to_owned()),
        "relation" => {
            if let Some((assay_id, relation)) = value.split_once('|') {
                sel.draft
                    .relations
                    .insert(assay_id.to_owned(), relation.to_owned());
            }
        }
        _ => return,
    }
    set_workspace_details(app, model, sel);
}

fn update_group_metric(
    app: &AppWindow,
    model: &ReadModel,
    sel: &mut Selection,
    group_id: &str,
    metric: &str,
    value: &str,
) {
    let parsed = value.trim().parse::<u32>().ok();
    match metric {
        "n" => {
            sel.draft.group_n.insert(group_id.to_owned(), parsed);
        }
        "technical" => {
            sel.draft
                .group_technical
                .insert(group_id.to_owned(), parsed);
        }
        _ => return,
    }
    set_workspace_details(app, model, sel);
}

fn set_workspace_details(app: &AppWindow, model: &ReadModel, sel: &Selection) {
    let active = active_design(model, sel);
    let current = model
        .designs
        .iter()
        .find(|design| design.id == sel.design_id);
    let run = model.runs.iter().find(|run| run.id == sel.run_id);

    let design_revision_text = if sel.mode == "prepare" || sel.mode == "execute" {
        match run
            .and_then(|run| run.design_version_id.as_ref())
            .and_then(|id| {
                model
                    .design_versions
                    .iter()
                    .find(|version| &version.id == id)
            }) {
            Some(version) => format!("冻结 r{}", version.revision),
            None => "冻结版本未记录".to_owned(),
        }
    } else {
        match current {
            Some(design) => format!("设计 r{}", design.revision),
            None => "设计版本未记录".to_owned(),
        }
    };

    let frozen_note = match run {
        None => "未选择执行批次".to_owned(),
        Some(run) => match run.design_version_id.as_ref() {
            None => "该批次未引用设计版本".to_owned(),
            Some(version_id) => match model
                .design_versions
                .iter()
                .find(|version| &version.id == version_id)
            {
                None => "冻结设计快照不可读取".to_owned(),
                Some(version) => match current {
                    Some(current) if current.revision != version.revision => format!(
                        "当前设计 r{}；本批次冻结 r{}",
                        current.revision, version.revision
                    ),
                    _ => format!("冻结版本 r{} 与当前设计一致", version.revision),
                },
            },
        },
    };

    let run_meta = match run {
        Some(run) => format!(
            "{} · {} · {} · 第 {} 轮",
            run.title,
            run_status_text(&run.status),
            run.scheduled_date.as_deref().unwrap_or("计划日期未记录"),
            run.run_index
                .map_or_else(|| "—".to_owned(), |n| n.to_string())
        ),
        None => "未选择执行批次".to_owned(),
    };

    let Some(design) = active else {
        app.set_group_details(model_rc(Vec::<GroupDetail>::new()));
        app.set_assay_details(model_rc(Vec::<AssayDetail>::new()));
        app.set_allocation_details(model_rc(Vec::<AllocationDetail>::new()));
        app.set_evidence_details(model_rc(Vec::<EvidenceDetail>::new()));
        app.set_conflict_details(model_rc(Vec::<ConflictDetail>::new()));
        app.set_map_n_text(SharedString::from("独立 n 待确认"));
        app.set_map_protocol_text(SharedString::from("Protocol 未关联"));
        app.set_map_time_text(SharedString::from("时间点待确认"));
        app.set_map_sample_text(SharedString::from("样本关系待确认"));
        app.set_map_attention(model_rc(Vec::<CheckDetail>::new()));
        app.set_check_details(model_rc(vec![CheckDetail {
            level: SharedString::from("warn"),
            title: SharedString::from("当前模式没有可读取的设计"),
            detail: SharedString::from(frozen_note.as_str()),
        }]));
        app.set_planned_output_details(model_rc(Vec::<PlannedOutputDetail>::new()));
        app.set_protocol_manifest(model_rc(Vec::<ManifestDetail>::new()));
        app.set_material_manifest(model_rc(Vec::<ManifestDetail>::new()));
        app.set_execution_events(model_rc(Vec::<ManifestDetail>::new()));
        app.set_design_hypothesis(SharedString::from("未记录"));
        app.set_design_variables(SharedString::from("未记录"));
        app.set_design_primary_endpoint(SharedString::from("未记录"));
        app.set_design_run_plan(SharedString::from("未记录"));
        app.set_design_revision_text(SharedString::from(design_revision_text));
        app.set_frozen_note(SharedString::from(frozen_note));
        app.set_run_meta(SharedString::from(run_meta));
        app.set_design_comparison(SharedString::from("未记录"));
        app.set_draft_note(SharedString::from(""));
        app.set_question_kind(SharedString::from(""));
        app.set_question_title(SharedString::from(""));
        app.set_question_detail(SharedString::from(""));
        app.set_question_options(model_rc(Vec::<QuestionOption>::new()));
        return;
    };

    let comparison = sel
        .draft
        .comparison
        .as_deref()
        .map(comparison_text)
        .unwrap_or_else(|| "未记录".to_owned());
    let experimental_unit = sel
        .draft
        .experimental_unit
        .as_deref()
        .map(unit_text)
        .unwrap_or_else(|| opt_text(&design.experimental_unit).to_string());
    app.set_design_comparison(SharedString::from(comparison));
    app.set_experimental_unit(SharedString::from(experimental_unit));
    app.set_draft_note(SharedString::from(if sel.draft.touched() {
        "本地草案 · 未写入数据库"
    } else {
        ""
    }));

    let group_n_text = |group: &data::Group| -> String {
        match sel.draft.group_n.get(&group.id) {
            Some(Some(n)) => n.to_string(),
            Some(None) => "待确认".to_owned(),
            None => group
                .independent_n
                .map_or_else(|| "待确认".to_owned(), |n| n.to_string()),
        }
    };
    let group_technical_text = |group: &data::Group| -> String {
        match sel.draft.group_technical.get(&group.id) {
            Some(Some(n)) => n.to_string(),
            Some(None) => "待确认".to_owned(),
            None => group
                .technical_repeats
                .map_or_else(|| "待确认".to_owned(), |n| n.to_string()),
        }
    };

    let groups = design
        .groups
        .iter()
        .map(|group| GroupDetail {
            id: SharedString::from(group.id.as_str()),
            name: SharedString::from(group.name.as_str()),
            role: group_role_text(&group.role),
            condition: group
                .condition
                .clone()
                .filter(|value| !value.trim().is_empty())
                .unwrap_or_else(|| "条件未记录".to_owned())
                .into(),
            n_text: SharedString::from(group_n_text(group)),
            technical_text: SharedString::from(group_technical_text(group)),
        })
        .collect::<Vec<_>>();

    let assays = design
        .assays
        .iter()
        .map(|assay| {
            let groups = assay_groups(design, assay);
            let timing = if assay.timepoint_timing.is_empty() {
                "未记录".to_owned()
            } else {
                assay
                    .timepoint_timing
                    .iter()
                    .map(|timing| {
                        let offset = match timing.offset_value {
                            Some(value) => {
                                format!("{value} {}", timing.unit.as_deref().unwrap_or(""))
                            }
                            None => "偏移未记录".to_owned(),
                        };
                        let window = match (timing.window_before, timing.window_after) {
                            (Some(before), Some(after)) => format!("（−{before} / +{after}）"),
                            _ => String::new(),
                        };
                        format!("{}：{offset}{window}", timing.timepoint)
                    })
                    .collect::<Vec<_>>()
                    .join("；")
            };
            let slots = if assay.allocations.is_empty() {
                "未记录".to_owned()
            } else {
                assay
                    .allocations
                    .iter()
                    .map(|allocation| {
                        let name = if allocation.group_name.trim().is_empty() {
                            "未命名组别"
                        } else {
                            allocation.group_name.as_str()
                        };
                        match allocation.slots_per_group {
                            Some(slots) => format!(
                                "{name}：每组 {slots} 孔（{}）",
                                scope_text(&allocation.slots_scope)
                            ),
                            None => format!("{name}：孔位未记录"),
                        }
                    })
                    .collect::<Vec<_>>()
                    .join("；")
            };
            let relation = sel
                .draft
                .relations
                .get(&assay.id)
                .cloned()
                .or_else(|| assay.timepoint_relation.clone());
            AssayDetail {
                id: SharedString::from(assay.id.as_str()),
                title: SharedString::from(assay.title.as_str()),
                endpoint: opt_text(&assay.endpoint),
                method: opt_text(&assay.method),
                timepoints: match join_nonempty(&assay.timepoints, " · ") {
                    Some(value) => SharedString::from(value),
                    None => SharedString::from("未记录"),
                },
                timing: SharedString::from(timing),
                relation: relation_text(&relation),
                container: opt_text(&assay.container_format),
                protocol: if assay.protocol_version_id.is_some() {
                    SharedString::from("已引用 Protocol")
                } else {
                    SharedString::from("未引用 Protocol")
                },
                coverage: if groups.is_empty() {
                    SharedString::from("组别未列出")
                } else {
                    SharedString::from(format!("{} 组覆盖", groups.len()))
                },
                slots: SharedString::from(slots),
                controls: match join_nonempty(&assay.controls, "；") {
                    Some(value) => SharedString::from(value),
                    None => SharedString::from("未记录"),
                },
            }
        })
        .collect::<Vec<_>>();

    let allocations = design
        .assays
        .iter()
        .flat_map(|assay| {
            assay.allocations.iter().map(move |allocation| {
                let output = allocation
                    .planned_output_id
                    .as_ref()
                    .and_then(|id| {
                        design
                            .planned_outputs
                            .iter()
                            .find(|output| &output.id == id)
                    })
                    .map(|output| output.title.clone())
                    .unwrap_or_else(|| "无计划产物".to_owned());
                let source = match (&allocation.source_relation, &allocation.source_assay_id) {
                    (Some(_), Some(id)) => format!(
                        "{} · {}",
                        source_relation_text(&allocation.source_relation),
                        assay_title_in(design, id)
                    ),
                    _ => source_relation_text(&allocation.source_relation),
                };
                AllocationDetail {
                    assay: SharedString::from(assay.title.as_str()),
                    group: SharedString::from(if allocation.group_name.trim().is_empty() {
                        "未命名组别"
                    } else {
                        allocation.group_name.as_str()
                    }),
                    slots: SharedString::from(match allocation.slots_per_group {
                        Some(slots) => format!("{slots} 孔"),
                        None => "未记录".to_owned(),
                    }),
                    scope: SharedString::from(scope_text(&allocation.slots_scope)),
                    source: SharedString::from(source),
                    output: SharedString::from(output),
                }
            })
        })
        .collect::<Vec<_>>();

    let evidence = design
        .sources
        .iter()
        .map(|source| EvidenceDetail {
            document: opt_text(&source.source_document),
            locator: opt_text(&source.source_locator),
            claim: opt_text(&source.claim),
            state: SharedString::from(evidence_state_text(&source.state)),
        })
        .collect::<Vec<_>>();

    let conflicts = design
        .conflicts
        .iter()
        .map(|conflict| ConflictDetail {
            topic: SharedString::from(conflict.topic.as_str()),
            status: conflict_status_text(&conflict.status),
            evidence: SharedString::from(if conflict.evidence_ids.is_empty() {
                "证据未记录".to_owned()
            } else {
                conflict.evidence_ids.join("、")
            }),
            affected: SharedString::from(if conflict.affected_assay_ids.is_empty() {
                String::new()
            } else {
                conflict
                    .affected_assay_ids
                    .iter()
                    .map(|id| assay_title_in(design, id))
                    .collect::<Vec<_>>()
                    .join("、")
            }),
        })
        .collect::<Vec<_>>();

    let mut checks = Vec::new();
    if sel.draft.comparison.is_none() {
        checks.push(CheckDetail {
            level: SharedString::from("warn"),
            title: SharedString::from("主要比较未记录"),
            detail: SharedString::from("组别存在不等于比较目标明确；先定义主要比较。"),
        });
    } else if sel.draft.comparison.as_deref() == Some("unknown") {
        checks.push(CheckDetail {
            level: SharedString::from("info"),
            title: SharedString::from("主要比较待确认"),
            detail: SharedString::from("本地草案已标记为暂不确定。"),
        });
    }
    if unit_missing(&design.experimental_unit) && sel.draft.experimental_unit.is_none() {
        checks.push(CheckDetail {
            level: SharedString::from("warn"),
            title: SharedString::from("实验单位待确认"),
            detail: SharedString::from("独立 n 的单位未确认；不能用孔数或技术重复代替。"),
        });
    }
    let groups_without_n: Vec<&str> = design
        .groups
        .iter()
        .filter(|group| {
            group.independent_n.is_none()
                && !matches!(sel.draft.group_n.get(&group.id), Some(Some(_)))
        })
        .map(|group| group.name.as_str())
        .collect();
    if !groups_without_n.is_empty() {
        checks.push(CheckDetail {
            level: SharedString::from("warn"),
            title: SharedString::from("独立 n 待确认"),
            detail: SharedString::from(format!(
                "{} 的 independent_n 为空；不能用孔数或技术重复代替 n。",
                groups_without_n.join("、")
            )),
        });
    }
    let groups_without_technical: Vec<&str> = design
        .groups
        .iter()
        .filter(|group| {
            group.technical_repeats.is_none()
                && !matches!(sel.draft.group_technical.get(&group.id), Some(Some(_)))
        })
        .map(|group| group.name.as_str())
        .collect();
    if !groups_without_technical.is_empty() {
        checks.push(CheckDetail {
            level: SharedString::from("info"),
            title: SharedString::from("技术重复待确认"),
            detail: SharedString::from(groups_without_technical.join("、")),
        });
    }
    for assay in &design.assays {
        if assay.protocol_version_id.is_none() {
            checks.push(CheckDetail {
                level: SharedString::from("warn"),
                title: SharedString::from("检测未引用 Protocol"),
                detail: SharedString::from(format!(
                    "{} 的 protocol_version_id 为空，不能生成执行步骤。",
                    assay.title
                )),
            });
        }
        if assay.group_ids.is_empty() && assay.group_names.is_empty() {
            checks.push(CheckDetail {
                level: SharedString::from("warn"),
                title: SharedString::from("检测未列出适用组别"),
                detail: SharedString::from(format!(
                    "{} 没有 group_ids/group_names 记录；不推断覆盖关系。",
                    assay.title
                )),
            });
        }
        if !present(&assay.timepoint_relation) && !sel.draft.relations.contains_key(&assay.id) {
            checks.push(CheckDetail {
                level: SharedString::from("warn"),
                title: SharedString::from("时间点关系待确认"),
                detail: SharedString::from(format!("{} 未记录时间点关系。", assay.title)),
            });
        }
    }
    for conflict in &design.conflicts {
        if !matches!(conflict.status.as_deref(), Some("resolved")) {
            checks.push(CheckDetail {
                level: SharedString::from("warn"),
                title: SharedString::from("存在未解决证据冲突"),
                detail: SharedString::from(conflict.topic.as_str()),
            });
        }
    }
    if design.planned_outputs.is_empty() {
        checks.push(CheckDetail {
            level: SharedString::from("info"),
            title: SharedString::from("没有计划产物"),
            detail: SharedString::from("planned_outputs 为空；不能推断检测共享同一份材料。"),
        });
    }
    if let Some(run) = run {
        if run.design_version_id.is_none() {
            checks.push(CheckDetail {
                level: SharedString::from("warn"),
                title: SharedString::from("执行批次未引用设计版本"),
                detail: SharedString::from("不能按冻结设计展示准备或执行。"),
            });
        }
        if run.protocol_manifest.is_empty() {
            checks.push(CheckDetail {
                level: SharedString::from("warn"),
                title: SharedString::from("执行批次没有 Protocol 清单"),
                detail: SharedString::from("protocol_manifest 为空；不能生成步骤顺序或完成状态。"),
            });
        }
        if run.material_manifest.is_empty() {
            checks.push(CheckDetail {
                level: SharedString::from("info"),
                title: SharedString::from("执行批次没有材料清单"),
                detail: SharedString::from("material_manifest 为空；准备模式不能确认实际材料。"),
            });
        }
    } else {
        checks.push(CheckDetail {
            level: SharedString::from("warn"),
            title: SharedString::from("未选择执行批次"),
            detail: SharedString::from("准备与执行模式需要先选择 Run。"),
        });
    }

    let planned_outputs = design
        .planned_outputs
        .iter()
        .map(|output| PlannedOutputDetail {
            title: SharedString::from(output.title.as_str()),
            protocol: if output.protocol_version_id.is_some() {
                SharedString::from("Protocol 已引用")
            } else {
                SharedString::from("Protocol 未引用")
            },
            evidence: opt_text(&output.evidence_id),
        })
        .collect::<Vec<_>>();

    let protocol_manifest = run
        .map(|run| {
            run.protocol_manifest
                .iter()
                .map(|item| ManifestDetail {
                    title: SharedString::from(item.title.as_str()),
                    detail: SharedString::from(item.detail.as_str()),
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    let material_manifest = run
        .map(|run| {
            run.material_manifest
                .iter()
                .map(|item| ManifestDetail {
                    title: SharedString::from(item.title.as_str()),
                    detail: SharedString::from(item.detail.as_str()),
                })
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    let events = model
        .events
        .iter()
        .filter(|event| event.run_id == sel.run_id)
        .map(|event| {
            let title = format!(
                "{} · {} · {}",
                event
                    .event_type
                    .as_deref()
                    .map(event_type_text)
                    .unwrap_or_else(|| "事件类型未记录".to_owned()),
                event.stage.as_deref().unwrap_or("阶段未记录"),
                event.timepoint.as_deref().unwrap_or("时间点未记录")
            );
            let detail = format!(
                "{}{}{}",
                event.content.as_deref().unwrap_or("内容未记录"),
                event
                    .occurred_at
                    .as_deref()
                    .map(|value| format!(" · {value}"))
                    .unwrap_or_default(),
                event
                    .step_status
                    .as_deref()
                    .map(|value| format!(" · {value}"))
                    .unwrap_or_default()
            );
            ManifestDetail {
                title: SharedString::from(title),
                detail: SharedString::from(detail),
            }
        })
        .collect::<Vec<_>>();

    app.set_group_details(model_rc(groups));
    app.set_assay_details(model_rc(assays));
    app.set_allocation_details(model_rc(allocations));
    app.set_evidence_details(model_rc(evidence));
    app.set_conflict_details(model_rc(conflicts));
    if let Some(map) = design_projection(design) {
        let (map_n, map_protocol, map_time, map_sample) = map_summary(&map);
        app.set_map_n_text(SharedString::from(map_n));
        app.set_map_protocol_text(SharedString::from(map_protocol));
        app.set_map_time_text(SharedString::from(map_time));
        app.set_map_sample_text(SharedString::from(map_sample));
        app.set_map_attention(model_rc(map_attention(&map)));
    } else {
        app.set_map_n_text(SharedString::from("设计投影不可用"));
        app.set_map_protocol_text(SharedString::from("设计投影不可用"));
        app.set_map_time_text(SharedString::from("设计投影不可用"));
        app.set_map_sample_text(SharedString::from("设计投影不可用"));
        app.set_map_attention(model_rc(Vec::<CheckDetail>::new()));
    }
    app.set_check_details(model_rc(checks));
    app.set_planned_output_details(model_rc(planned_outputs));
    app.set_protocol_manifest(model_rc(protocol_manifest));
    app.set_material_manifest(model_rc(material_manifest));
    app.set_execution_events(model_rc(events));
    app.set_design_hypothesis(opt_text(&design.hypothesis));
    app.set_design_variables(opt_text(&design.independent_variables));
    app.set_design_primary_endpoint(opt_text(&design.primary_endpoint));
    app.set_design_run_plan(SharedString::from(match design.planned_run_count {
        Some(count) => format!(
            "{count} 轮 · {}",
            design
                .planned_run_relationship
                .as_deref()
                .map(run_relation_text)
                .unwrap_or_else(|| "关系待确认".to_owned())
        ),
        None => "未记录".to_owned(),
    }));
    app.set_design_revision_text(SharedString::from(design_revision_text));
    app.set_frozen_note(SharedString::from(frozen_note));
    app.set_run_meta(SharedString::from(run_meta));
    set_question(app, design, sel);
}

fn event_type_text(value: &str) -> String {
    match value {
        "operation" => "操作".to_owned(),
        "material_use" => "材料使用".to_owned(),
        "measurement" => "测量".to_owned(),
        "deviation" => "偏差".to_owned(),
        "observation" => "观察".to_owned(),
        _ => value.to_owned(),
    }
}

fn run_relation_text(value: &str) -> String {
    match value {
        "independent" => "独立来源".to_owned(),
        "same_source" => "同一来源".to_owned(),
        "paired" => "配对/区组".to_owned(),
        "unknown" => "关系待确认".to_owned(),
        _ => value.to_owned(),
    }
}

fn design_runs<'a>(model: &'a ReadModel, design_id: &str) -> Vec<&'a data::Run> {
    model
        .runs
        .iter()
        .filter(|run| run.design_id == design_id)
        .collect()
}

/// A plate is in context only when it belongs to the selected run and, if an
/// assay is selected, to that assay. This prevents showing a CCK-8 plate while
/// the inspector says live/dead.
fn plate_matches_context(plate: &data::Plate, sel: &Selection) -> bool {
    !sel.run_id.is_empty()
        && plate.run_id == sel.run_id
        && (sel.assay_id.is_empty() || plate.assay_id.as_deref() == Some(sel.assay_id.as_str()))
}

fn refresh_plates<'a>(
    app: &AppWindow,
    model: &'a ReadModel,
    sel: &Selection,
) -> Vec<&'a data::Plate> {
    let plates: Vec<&data::Plate> = model
        .plates
        .iter()
        .filter(|plate| plate_matches_context(plate, sel))
        .collect();
    app.set_plates(model_rc(
        plates
            .iter()
            .map(|plate| PlateEntry {
                id: SharedString::from(plate.id.as_str()),
                name: SharedString::from(plate.name.as_str()),
                format: SharedString::from(plate.format.as_str()),
                timepoint: opt_text(&plate.timepoint),
                mapped_count: mapped_well_count(plate),
            })
            .collect(),
    ));
    plates
}

fn first_plate_for_context<'a>(plates: &[&'a data::Plate]) -> Option<&'a data::Plate> {
    plates
        .iter()
        .copied()
        .find(|plate| plate.wells.iter().any(well_mapped))
        .or_else(|| plates.first().copied())
}

fn select_mode(app: &AppWindow, model: &ReadModel, sel: &mut Selection, mode: &str) {
    if !matches!(mode, "design" | "prepare" | "execute") {
        return;
    }
    sel.mode = mode.to_owned();
    app.set_mode(SharedString::from(mode));

    if let Some(design) = active_design(model, sel) {
        if !design.assays.iter().any(|assay| assay.id == sel.assay_id) {
            clear_assay(app, sel);
        }
    } else {
        clear_assay(app, sel);
    }

    refresh_canvas(app, model, sel);
    let plates = refresh_plates(app, model, sel);
    match first_plate_for_context(&plates).map(|plate| plate.id.clone()) {
        Some(id) => select_plate(app, model, sel, &id),
        None => clear_plate(app, sel),
    }
    set_workspace_details(app, model, sel);

    if !sel.assay_id.is_empty() {
        set_inspector_assay(app, model, sel);
    } else if sel.plate_id.is_empty() {
        clear_inspector(app);
    }
}

fn select_run(app: &AppWindow, model: &ReadModel, sel: &mut Selection, run_id: &str) {
    let Some(run) = model.runs.iter().find(|run| run.id == run_id) else {
        return;
    };
    if sel.design_id.is_empty() || run.design_id != sel.design_id {
        return;
    }
    sel.run_id = run.id.clone();
    app.set_selected_run_id(SharedString::from(run.id.as_str()));
    app.set_run_title(SharedString::from(run.title.as_str()));
    app.set_run_status(run_status_text(&run.status));

    refresh_canvas(app, model, sel);
    let plates = refresh_plates(app, model, sel);
    match first_plate_for_context(&plates).map(|plate| plate.id.clone()) {
        Some(id) => select_plate(app, model, sel, &id),
        None => clear_plate(app, sel),
    }
    set_workspace_details(app, model, sel);

    if !sel.assay_id.is_empty() {
        set_inspector_assay(app, model, sel);
    } else if sel.plate_id.is_empty() {
        clear_inspector(app);
    }
}

fn select_assay(app: &AppWindow, model: &ReadModel, sel: &mut Selection, assay_id: &str) {
    let exists = active_design(model, sel)
        .is_some_and(|design| design.assays.iter().any(|assay| assay.id == assay_id));
    if !exists {
        clear_assay(app, sel);
        refresh_canvas(app, model, sel);
        return;
    }
    sel.assay_id = assay_id.to_owned();
    app.set_selected_assay_id(SharedString::from(assay_id));
    refresh_canvas(app, model, sel);

    let plates = refresh_plates(app, model, sel);
    match first_plate_for_context(&plates).map(|plate| plate.id.clone()) {
        Some(id) => select_plate(app, model, sel, &id),
        None => clear_plate(app, sel),
    }

    set_inspector_assay(app, model, sel);
}

fn select_group(app: &AppWindow, model: &ReadModel, sel: &mut Selection, group_id: &str) {
    let exists = active_design(model, sel)
        .is_some_and(|design| design.groups.iter().any(|group| group.id == group_id));
    if !exists {
        return;
    }
    sel.group_id = group_id.to_owned();
    app.set_selected_group_id(SharedString::from(group_id));
    refresh_canvas(app, model, sel);
    set_inspector_group(app, model, sel);
}

fn select_well(app: &AppWindow, model: &ReadModel, sel: &mut Selection, well_id: &str) {
    if !model
        .plates
        .iter()
        .any(|plate| plate.id == sel.plate_id && plate_matches_context(plate, sel))
    {
        return;
    }
    sel.well_id = well_id.to_owned();
    app.set_selected_well_id(SharedString::from(well_id));
    set_inspector_well(app, model, sel);
}

fn select_plate(app: &AppWindow, model: &ReadModel, sel: &mut Selection, plate_id: &str) {
    let plate = model
        .plates
        .iter()
        .find(|plate| plate.id == plate_id && plate_matches_context(plate, sel));
    let Some(plate) = plate else {
        return;
    };
    sel.plate_id = plate.id.clone();
    sel.well_id.clear();
    app.set_selected_plate_id(SharedString::from(plate.id.as_str()));
    app.set_selected_well_id(SharedString::from(""));
    app.set_has_plate(true);
    app.set_plate_name(SharedString::from(plate.name.as_str()));
    app.set_plate_meta(SharedString::from(format!(
        "{} · 时间点：{} · {}",
        format_label(&plate.format),
        opt_text(&plate.timepoint),
        match assay_title(model, &plate.assay_id) {
            Some(title) if !title.trim().is_empty() => title,
            _ => "关联检测未记录".to_owned(),
        }
    )));
    app.set_plate_rows(plate.rows as i32);
    app.set_plate_columns(plate.columns as i32);

    let mut cells = Vec::with_capacity(plate.rows as usize * plate.columns as usize);
    for row in 0..plate.rows {
        for column in 0..plate.columns {
            let id = coordinate(row, column);
            let stored = plate
                .wells
                .iter()
                .find(|well| well.coordinate.eq_ignore_ascii_case(&id));
            let (group_label, sample_id, note, mapped) = match stored {
                Some(well) => (
                    well.group_label.clone().unwrap_or_default(),
                    well.sample_id.clone().unwrap_or_default(),
                    well.note.clone().unwrap_or_default(),
                    well_mapped(well),
                ),
                None => (String::new(), String::new(), String::new(), false),
            };
            cells.push(WellCell {
                well_id: SharedString::from(id.as_str()),
                row: row as i32,
                column: column as i32,
                group_label: SharedString::from(group_label.as_str()),
                sample_id: SharedString::from(sample_id.as_str()),
                note: SharedString::from(note.as_str()),
                mapped,
            });
        }
    }
    app.set_plate_mapped_count(mapped_well_count(plate));
    app.set_wells(model_rc(cells));

    let rows = (0..plate.rows)
        .map(|index| AxisLabel {
            index: index as i32,
            text: SharedString::from(row_letters(index)),
        })
        .collect::<Vec<_>>();
    let columns = (0..plate.columns)
        .map(|index| AxisLabel {
            index: index as i32,
            text: SharedString::from((index + 1).to_string()),
        })
        .collect::<Vec<_>>();
    app.set_row_labels(model_rc(rows));
    app.set_column_labels(model_rc(columns));

    set_inspector_plate(app, model, sel);
}

fn select_design(app: &AppWindow, model: &ReadModel, sel: &mut Selection, design_id: &str) {
    let design = model.designs.iter().find(|design| design.id == design_id);
    let Some(design) = design else {
        clear_design(app, sel);
        return;
    };
    sel.design_id = design.id.clone();
    sel.run_id.clear();
    sel.assay_id.clear();
    sel.group_id.clear();
    sel.plate_id.clear();
    sel.well_id.clear();
    sel.mode = "design".to_owned();
    sel.draft = Draft::default();
    app.set_mode(SharedString::from("design"));
    app.set_selected_design_id(SharedString::from(design.id.as_str()));
    app.set_selected_run_id(SharedString::from(""));
    app.set_selected_assay_id(SharedString::from(""));
    app.set_selected_group_id(SharedString::from(""));
    app.set_design_title(SharedString::from(design.title.as_str()));
    app.set_design_objective(opt_text(&design.objective));
    app.set_experimental_unit(opt_text(&design.experimental_unit));

    let runs = design_runs(model, &design.id);
    app.set_runs(model_rc(
        runs.iter()
            .map(|run| RunEntry {
                id: SharedString::from(run.id.as_str()),
                title: SharedString::from(run.title.as_str()),
                detail: SharedString::from(format!(
                    "{} · {} 块板",
                    run_status_text(&run.status),
                    model
                        .plates
                        .iter()
                        .filter(|plate| plate.run_id == run.id)
                        .count()
                )),
            })
            .collect(),
    ));

    match runs.first().map(|run| run.id.clone()) {
        Some(id) => select_run(app, model, sel, &id),
        None => {
            app.set_run_title(SharedString::from(""));
            app.set_run_status(unconfirmed());
            refresh_canvas(app, model, sel);
        }
    }

    // Prefer an assay that has a plate in this run; otherwise the first assay.
    let default_assay = design
        .assays
        .iter()
        .find(|assay| {
            model.plates.iter().any(|plate| {
                plate.run_id == sel.run_id && plate.assay_id.as_deref() == Some(assay.id.as_str())
            })
        })
        .or_else(|| design.assays.first())
        .map(|assay| assay.id.clone());

    match default_assay {
        Some(id) => select_assay(app, model, sel, &id),
        None => {
            clear_assay(app, sel);
            refresh_canvas(app, model, sel);
            let plates = refresh_plates(app, model, sel);
            match first_plate_for_context(&plates).map(|plate| plate.id.clone()) {
                Some(id) => select_plate(app, model, sel, &id),
                None => {
                    clear_plate(app, sel);
                    clear_inspector(app);
                }
            }
        }
    }
    set_workspace_details(app, model, sel);
}

fn select_project(app: &AppWindow, model: &ReadModel, sel: &mut Selection, project_id: &str) {
    let project = model
        .projects
        .iter()
        .find(|project| project.id == project_id);
    let Some(project) = project else {
        return;
    };
    sel.project_id = project.id.clone();
    sel.design_id.clear();
    sel.run_id.clear();
    sel.assay_id.clear();
    sel.group_id.clear();
    sel.plate_id.clear();
    sel.well_id.clear();
    sel.mode = "design".to_owned();
    sel.draft = Draft::default();
    app.set_mode(SharedString::from("design"));
    app.set_selected_project_id(SharedString::from(project.id.as_str()));
    app.set_selected_design_id(SharedString::from(""));
    app.set_selected_run_id(SharedString::from(""));
    app.set_selected_assay_id(SharedString::from(""));
    app.set_selected_group_id(SharedString::from(""));
    app.set_runs(model_rc(Vec::<RunEntry>::new()));
    app.set_project_name(SharedString::from(project.name.as_str()));
    app.set_designs(model_rc(
        model
            .designs
            .iter()
            .filter(|design| design.project_id == project.id)
            .map(|design| DesignEntry {
                id: SharedString::from(design.id.as_str()),
                title: SharedString::from(design.title.as_str()),
                detail: SharedString::from(format!(
                    "{} 项检测 · {} 次运行",
                    design.assays.len(),
                    model
                        .runs
                        .iter()
                        .filter(|run| run.design_id == design.id)
                        .count()
                )),
            })
            .collect(),
    ));
    let first_design = model
        .designs
        .iter()
        .find(|design| design.project_id == project.id)
        .map(|design| design.id.clone());
    match first_design {
        Some(id) => select_design(app, model, sel, &id),
        None => clear_design(app, sel),
    }
}

fn main() {
    let started = std::time::Instant::now();
    let timing = std::env::var_os("LABWORKBENCH_TIMING").is_some();
    let mark = |label: &str| {
        if timing {
            eprintln!("timing {label}: {} ms", started.elapsed().as_millis());
        }
    };
    let (db_path, renderer) = parse_args();
    mark("args");
    if std::fs::File::open(&db_path).is_err() {
        eprintln!(
            "数据库不可读：{}（请确认路径存在且有读取权限，不会创建新文件）。",
            db_path.display()
        );
        std::process::exit(1);
    }
    let model = match data::load_read_model(&db_path) {
        Ok(model) => model,
        Err(err) => {
            eprintln!("数据库读取失败：{err}");
            std::process::exit(1);
        }
    };
    mark("model");
    select_backend(&renderer);
    mark("backend");

    let app = match AppWindow::new() {
        Ok(app) => app,
        Err(err) => {
            eprintln!("主窗口创建失败：{err:?}");
            std::process::exit(1);
        }
    };
    mark("window");
    app.set_data_counts(SharedString::from(format!(
        "{} 个项目 · {} 个设计 · {} 块孔板",
        model.projects.len(),
        model.designs.len(),
        model.plates.len()
    )));

    let model = Rc::new(model);
    let sel = Rc::new(RefCell::new(Selection::default()));

    app.set_projects(model_rc(
        model
            .projects
            .iter()
            .map(|project| ProjectEntry {
                id: SharedString::from(project.id.as_str()),
                name: SharedString::from(project.name.as_str()),
                detail: SharedString::from(format!(
                    "{} 个设计",
                    model
                        .designs
                        .iter()
                        .filter(|design| design.project_id == project.id)
                        .count()
                )),
            })
            .collect(),
    ));
    {
        let first = model.projects.first().map(|project| project.id.clone());
        match first {
            Some(id) => select_project(&app, &model, &mut sel.borrow_mut(), &id),
            None => {
                app.set_selected_project_id(SharedString::from(""));
                app.set_project_name(SharedString::from("暂无项目"));
                app.set_designs(model_rc(Vec::<DesignEntry>::new()));
                clear_design(&app, &mut sel.borrow_mut());
            }
        }
    }

    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_project_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_project(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_design_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_design(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_run_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_run(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_mode_selected(move |mode| {
            if let Some(app) = weak.upgrade() {
                select_mode(&app, &model, &mut sel.borrow_mut(), mode.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_question_answered(move |kind, value| {
            if let Some(app) = weak.upgrade() {
                answer_question(
                    &app,
                    &model,
                    &mut sel.borrow_mut(),
                    kind.as_str(),
                    value.as_str(),
                );
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_group_metric_changed(move |id, metric, value| {
            if let Some(app) = weak.upgrade() {
                update_group_metric(
                    &app,
                    &model,
                    &mut sel.borrow_mut(),
                    id.as_str(),
                    metric.as_str(),
                    value.as_str(),
                );
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_assay_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_assay(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_group_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_group(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_plate_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_plate(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let weak = app.as_weak();
        let model = Rc::clone(&model);
        let sel = Rc::clone(&sel);
        app.on_well_selected(move |id| {
            if let Some(app) = weak.upgrade() {
                select_well(&app, &model, &mut sel.borrow_mut(), id.as_str());
            }
        });
    }
    {
        let slot: Rc<RefCell<Option<AboutWindow>>> = Rc::new(RefCell::new(None));
        app.on_about_requested(move || match AboutWindow::new() {
            Ok(about) => {
                if about.show().is_err() {
                    eprintln!("关于窗口打开失败。");
                    return;
                }
                *slot.borrow_mut() = Some(about);
            }
            Err(err) => eprintln!("关于窗口创建失败：{err:?}"),
        });
    }

    mark("state");
    if let Err(err) = app.show() {
        eprintln!("窗口显示失败：{err:?}");
        std::process::exit(1);
    }
    mark("shown");
    if let Err(err) = app.run() {
        eprintln!("界面运行失败：{err:?}");
        std::process::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::{assay_groups, coordinate, data, row_letters};

    fn sample_design() -> data::Design {
        data::Design {
            id: "d1".into(),
            project_id: "p1".into(),
            revision: 1,
            title: "设计".into(),
            objective: None,
            hypothesis: None,
            experimental_unit: None,
            independent_variables: None,
            primary_endpoint: None,
            endpoint_method: None,
            endpoint_unit: None,
            planned_run_count: None,
            planned_run_relationship: None,
            analysis_plan: None,
            randomization: None,
            blinding: None,
            inclusion_criteria: None,
            exclusion_criteria: None,
            sample_size_rationale: None,
            groups: vec![
                data::Group {
                    id: "g1".into(),
                    name: "对照".into(),
                    role: "control".into(),
                    condition: None,
                    independent_n: None,
                    technical_repeats: None,
                },
                data::Group {
                    id: "g2".into(),
                    name: "处理".into(),
                    role: "treatment".into(),
                    condition: None,
                    independent_n: None,
                    technical_repeats: None,
                },
            ],
            assays: Vec::new(),
            planned_outputs: Vec::new(),
            sources: Vec::new(),
            conflicts: Vec::new(),
            raw_data: serde_json::json!({"title":"设计"}),
        }
    }

    fn sample_assay(group_ids: Vec<&str>, group_names: Vec<&str>) -> data::Assay {
        data::Assay {
            id: "a1".into(),
            title: "检测".into(),
            endpoint: None,
            method: None,
            unit: None,
            timepoints: Vec::new(),
            timepoint_timing: Vec::new(),
            group_ids: group_ids.into_iter().map(str::to_owned).collect(),
            group_names: group_names.into_iter().map(str::to_owned).collect(),
            control_group_ids: Vec::new(),
            timepoint_relation: None,
            time_anchor_event: None,
            container_format: None,
            controls: Vec::new(),
            allocations: Vec::new(),
            protocol_version_id: None,
            evidence_id: None,
        }
    }

    #[test]
    fn plate_coordinates_use_spreadsheet_rows() {
        assert_eq!(coordinate(0, 0), "A1");
        assert_eq!(coordinate(7, 11), "H12");
        assert_eq!(coordinate(25, 0), "Z1");
        assert_eq!(coordinate(26, 0), "AA1");
        assert_eq!(coordinate(31, 11), "AF12");
        assert_eq!(row_letters(0), "A");
        assert_eq!(row_letters(26), "AA");
    }

    #[test]
    fn assay_links_only_explicitly_listed_groups() {
        let design = sample_design();
        let by_id = sample_assay(vec!["g2"], vec![]);
        let linked = assay_groups(&design, &by_id);
        assert_eq!(linked.len(), 1);
        assert_eq!(linked[0].id, "g2");

        // Explicit ids win; names that are not listed do not become links.
        let by_id_only = sample_assay(vec!["g1"], vec!["处理"]);
        let linked = assay_groups(&design, &by_id_only);
        assert_eq!(linked.len(), 1);
        assert_eq!(linked[0].id, "g1");

        let by_name = sample_assay(vec![], vec!["处理"]);
        let linked = assay_groups(&design, &by_name);
        assert_eq!(linked.len(), 1);
        assert_eq!(linked[0].id, "g2");

        let unlinked = sample_assay(vec![], vec![]);
        assert!(assay_groups(&design, &unlinked).is_empty());
    }

    #[test]
    fn plates_only_match_selected_run_and_assay() {
        let plate = data::Plate {
            id: "p1".into(),
            run_id: "r1".into(),
            name: "板".into(),
            format: "96".into(),
            rows: 8,
            columns: 12,
            assay_id: Some("a1".into()),
            timepoint: None,
            wells: Vec::new(),
        };
        let selected = super::Selection {
            run_id: "r1".into(),
            assay_id: "a1".into(),
            ..Default::default()
        };
        assert!(super::plate_matches_context(&plate, &selected));

        let other_assay = super::Selection {
            assay_id: "a2".into(),
            ..selected.clone()
        };
        assert!(!super::plate_matches_context(&plate, &other_assay));

        let other_run = super::Selection {
            run_id: "r2".into(),
            ..selected.clone()
        };
        assert!(!super::plate_matches_context(&plate, &other_run));

        let no_assay = super::Selection {
            assay_id: String::new(),
            ..selected
        };
        assert!(super::plate_matches_context(&plate, &no_assay));

        let no_run = super::Selection::default();
        assert!(!super::plate_matches_context(&plate, &no_run));
    }
}
