//! AsterLab Rune feasibility spike: Chinese text, editable fields, focusable save action, headless snap.
//! `cargo run -p aster-spike -- --snap aster-spike.png`

use rune::tok::{self, radius, space, type_};
use rune::{AppSpec, Cx, El, TextEdit, col, row, el, text, text_input};

struct Spike {
    well: TextEdit,
    group: TextEdit,
    od: TextEdit,
    rows: Vec<TextEdit>,
    saved: bool,
}

fn init() -> Spike {
    Spike {
        well: TextEdit::new("A1"),
        group: TextEdit::new("Control"),
        od: TextEdit::new("0.77"),
        rows: (0..100).map(|i| TextEdit::new(format!("A{}", i + 1))).collect(),
        saved: false,
    }
}

fn field(label: &'static str, edit: &TextEdit, key: &'static str, t: tok::Theme, access: fn(&mut Spike) -> &mut TextEdit) -> El {
    col().gap(space::S1).child(text(label).font(type_::LABEL).color(t.text_secondary)).child(
        el().w_full().h(32.0).px(space::S2).radius(radius::MD).fill(t.card)
            .border(1.0, t.card_border).center_y()
            .child(text_input(edit, type_::BODY).key(key).single_line().color(t.text).on_edit(access)),
    )
}

fn view(s: &Spike, cx: &mut Cx<Spike>) -> El {
    let t = tok::theme(cx.system_dark());
    col().w_full().h_full().px(space::S6).py(space::S6).gap(16.0).fill(t.bg)
        .child(text("实验运行 · Run 单屏 Spike").font(type_::TITLE).color(t.text))
        .child(text("L929 closure run · 当前任务 3/5 · D7 · 中文标签、输入和表格字段").font(type_::BODY).color(t.text_secondary))
        .child(text("这是一段用于验证中文长文本换行、标点、数字 12345 和单位 mg/mL 混排的说明。实验记录必须保持未知与已知的边界，不能根据孔位数量推断独立实验单位，也不能把计划参数写成实际测量值。").w_full().wrap().font(type_::BODY).color(t.text_secondary))
        .child(
            col().w_full().gap(space::S2).px(16.0).py(16.0).radius(radius::LG).fill(t.card)
                .border(1.0, t.card_border)
                .child(text("CCK-8 读板记录").font(type_::TITLE).color(t.text))
                .child(text("按当前 Assay 的结构化字段记录，保存后进入结果。").font(type_::LABEL).color(t.text_secondary))
                .child(
                    row().w_full().gap(space::S2)
                        .child(field("孔位", &s.well, "well", t, |s| &mut s.well))
                        .child(field("组别", &s.group, "group", t, |s| &mut s.group))
                        .child(field("OD450", &s.od, "od", t, |s| &mut s.od)),
                )
                .child(
                    row().gap(space::S2).items_center()
                        .child(
                            el().key("save").role("button").label("保存读数").h(30.0).px(space::S3)
                                .center().radius(radius::MD).fill(tok::accent(1, cx.system_dark()))
                                .on_press(|s: &mut Spike| s.saved = true)
                                .child(text("保存读数").font(type_::LABEL).color(t.knob)),
                        )
                        .child(text(if s.saved { "已保存 · 0.77 AU" } else { "未保存" })
                            .font(type_::LABEL).color(if s.saved { t.text } else { t.text_secondary })),
                ),
        )
        .child(
            col().key("table-card").w_full().gap(space::S1).px(16.0).py(16.0).radius(radius::LG).fill(t.card).border(1.0, t.card_border)
                .child(text("100 行可编辑记录").font(type_::TITLE).color(t.text))
                .child(
                    col().key("rows").w_full().h(220.0).scroll(rune::ScrollSpec { stick_bottom: false })
                        .children(s.rows.iter().enumerate().map(|(i, edit)| {
                            row().w_full().h(28.0).gap(space::S2).items_center()
                                .child(text(format!("{:03}", i + 1)).w(32.0).font(type_::LABEL).color(t.text_secondary))
                                .child(el().w(180.0).h(24.0).px(space::S1).radius(radius::SM).fill(t.bg).border(1.0, t.card_border).center_y()
                                    .child(text_input(edit, type_::LABEL).key(format!("cell-{i}")).single_line().color(t.text).on_edit(move |s: &mut Spike| &mut s.rows[i])))
                                .child(text("OD450 · AU").font(type_::LABEL).color(t.text_secondary))
                        })),
                ),
        )
        .child(text("材料与试剂 未绑定 · 仪器与程序 未绑定 · 检查与接受 0/2").font(type_::BODY).color(t.text_secondary))
}

fn main() {
    rune::run(AppSpec::<Spike> {
        ..AppSpec::new("AsterLab Rune Spike", (920.0, 600.0), init, view)
    });
}
