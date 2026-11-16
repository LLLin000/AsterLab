# LB-18 证据：V2 设计系统一致性规范

基线：`ad8ed02`（`git rev-parse HEAD` = `ad8ed02ba67f430c6ee0177fb0a4aef89c02ec79`，2026-11-16）。
计数命令在 `prototypes/mygo-run` 内执行（另有说明除外）；输出为实际结果。
旧基线 `b2acf22` 的差异见 §5。

## 1. 反模式基线（`prototypes/mygo-run`，排除 `_test.go`）

```text
$ grep -rn "FontSize([0-9]" --include=*.go . | grep -v _test.go | wc -l
175

$ grep -rn "Padding([0-9]" --include=*.go . | grep -v _test.go | wc -l
97

$ grep -rn "Radius([0-9]" --include=*.go . | grep -v _test.go | wc -l
58

$ grep -rn "ui.Hex(" --include=*.go internal/asterdoc internal/uikit | grep -v _test.go | wc -l
3

$ grep -c 'ui\.Hex(' internal/theme/theme.go
11
```

按包分布（`for d in …; do printf …; done` 循环实测输出；根目录为顶层 `*.go`）：

```text
internal/runui FS=80 Pad=23 Rad=19
internal/protocol FS=25 Pad=3 Rad=0
internal/runworkspace FS=22 Pad=31 Rad=30
internal/tools FS=7 Pad=0 Rad=0
internal/resultgrid FS=1 Pad=7 Rad=5
internal/asterdoc FS=0 Pad=10 Rad=3
internal/designer FS=0 Pad=4 Rad=0
root FS=40 Pad=16 Rad=1
```

根目录细分（`for f in main.go design_page.go design_canvas.go notebook_page.go`）：`main.go` 34/6/1、`design_page.go` 4/2/0、`design_canvas.go` 2/5/0（FS/Pad/Rad）。

细则（真实行号，均复核过本基线）：

```text
$ grep -n "Padding(6)" internal/resultgrid/view.go
54:    ui.Text(c, label).Width(width).Padding(6).Background(c.Theme().Surface).Bold()
60:    ui.Text(c, label).Width(38).Padding(6).Bold()
81:    e := ui.TextInput(c, g.input(rr, cc)).Width(width).Padding(6).Placeholder("—").OnSubmit(commit).OnPaste(func(text string) bool {

$ grep -rln '\.Padding(6)' --include=*.go . | grep -v _test.go
./internal/resultgrid/view.go

$ grep -n 'ui\.Hex(' internal/asterdoc/view.go
735:    ui.Hex("#dcd6c8"), ui.Hex("#cfc4f0"), ui.Hex("#c9d6f0"), ui.Hex("#c9e3d2"),
736:    ui.Hex("#f0c8ce"), ui.Hex("#efdab4"),
739:    return ui.Hex("#f4f1ea")

$ grep -rn 'ui\.Hex(' --include=*.go internal/uikit | wc -l
0
```

- 单参数 `Padding(6)`：全项目仅 `internal/resultgrid/view.go:54,60,81` 三处。
- `ui.Hex` 非定义处仅 `internal/asterdoc/view.go:735,736,739`（`groupColor` 本地调色板，7 处字面值）；`uikit` 已收进 `plate.GroupColor`（`internal/uikit/plate/plate.go:86`），零字面值。
- 不在 6/10/12 档的 `Radius` 共 30 处：

```text
$ grep -rn 'Radius([0-9]' --include=*.go . | grep -v _test.go | grep -vE 'Radius\((6|10|12)\)' | wc -l
30
```

`runui.go:630,725,730,755,760,1095`（8）、`:1139`（5）；`runworkspace/recording.go:138,633,720`、`view.go:536,561,628,692,731,1013,1029,1096,1278,1296,1453,1481`；`resultgrid/plate.go:124,474`（5）、`:162`（4）、`:417`（8）；`asterdoc/view.go:198,371`（4.5）、`:567`（3）；`main.go:3140`（4）。

- 已合规范式（`internal/designer/step_list.go` 的 `designBudget`/`listToolbar`/`sectionSwitch`）：

```text
$ grep -n 'FontSize([0-9]\|Padding([0-9]\|Radius([0-9]\|ui\.Hex(' internal/designer/step_list.go
482:  ui.Row(c).FillWidth().Padding(0, t.Space(4), t.Space(1.5), t.Space(4)).Children(func() {
```

无颜色/字号/圆角字面值（仅一处 `Padding(0, …)` 归零写法）；该文件引用 `components` 31 处。

## 2. Token 与组件来源抽查（本基线实测）

- `internal/theme/theme.go:22-25` 声明 `Spacing4…48`、`TextPage/TextSection/TextBody/TextMeta/TextData`、`ButtonHeight/InputHeight/RowHeight`、`ControlRadius/PanelRadius/DialogRadius`；`:36-40` 取值为 4/8/12/16/24/32/48、22/15/13/12/12、30/30/32、6/10/12；`:126` `t.Spacing = 4`。
- `t.Space`/`t.Rem` 为 `github.com/egoist/mygo` `ui.Theme` 方法（`ui/theme.go:86,152`）。
- `components.Button/IconButton/TabsSegmented/Badge/Table/TableVirtualized/Dialog/Sidebar/Progress/Skeleton/Tabs` 在本地 `onevra/source@v0.2026.18/components/` 中均有定义（`button.go`、`icon_button.go`、`tabs.go`、`badge.go`、`table.go`、`table_virtualized.go`、`dialog.go`、`sidebar.go`、`progress.go`、`skeleton.go`）；`components.Tabs(c, &index, components.TabsSegmented, …)` 与 `tabs.go:37` 签名一致；`mygo-run` 的 `design_page.go`/`design_canvas.go` 已导入 `components` 与 `lucide`。

## 3. 文档挂载（仓库根目录 grep 原始输出）

```text
$ grep -rn 'design-system.md' AGENTS.md docs/product/ux-contract.md docs/product/ownership.md
AGENTS.md:61:- 动任何界面之前先读 [`docs/product/design-system.md`](docs/product/design-system.md)：它规定了唯一 token 源、4 倍数间距、三档圆角、需求→组件对照表（Do/Don't）与文案规则。
AGENTS.md:63:- 通用颜色/圆角/控件高度取自 `internal/theme` 的 `Tokens`；字号用文本 token 或 `t.Rem`，间距用间距 token 或 `t.Space`；科学语义颜色只能复用 `docs/product/design-system.md` §1 登记的具名调色板；调用点不写字面值。
AGENTS.md:64:- 需要新组件时，先确认仓库与 onevra 都没有等价实现，并在 `docs/product/design-system.md` 的组件表登记原因。
docs/product/ux-contract.md:173:`docs/product/design-system.md` is the enforceable visual contract behind P3: the
docs/product/ux-contract.md:183:  in `docs/product/design-system.md` §1; call sites must not contain literals.
```

## 4. 规范条目计数

```text
$ grep -c '^- Do:' docs/product/design-system.md
12
$ grep -c "^- Don't:" docs/product/design-system.md
12
```

## 5. 结论与相对旧基线的修正

规范落在三处：

- `docs/product/design-system.md`：单一 token 源、4 倍数间距、三档圆角、需求→组件对照表、文案规则、反模式基线与机器检查命令、12 组 Do/Don't 范例。
- `docs/product/ux-contract.md` §8：与 P3 绑定，规则 id `UX-DESIGN`。
- `AGENTS.md` §3「UI 工作方法（强制）」：开工前必读，组件按语义选，PR 带 `## 设计一致性`；`Windows Work` WIP=4（LB-18 跟进由 2 改为 4，GUI/native build/截图/E2E/最终集成串行规则不变）。

相对 `b2acf22` 草稿的修正（本基线复核发现，旧引用已失效）：

- `uikit/plate/plate.go` 的 16 处 `ui.Hex` 已重构为具名 `plate.GroupColor`（`:86`，内部用 token）；D1 改指 `asterdoc/view.go:734-739`。
- `asterdoc/view.go:493` 不再是 `Radius(3)`，现为 `:567`（另 `:198,371` 为 4.5）。
- `runui.go:629` 手搓按钮现为 `:630`；`runui` 新增 `Radius(8)` 两处（`:760,1095`）。
- `designBudget` 为 `step_list.go` 实际标识符（草稿误写 `budgetBar`）。
- 「非 4 倍数 `Padding`」行收窄为可验证口径：单参数 `Padding(6)` 共 3 处。

反模式清理不在本卡，按第 1 节清单另开卡执行。
