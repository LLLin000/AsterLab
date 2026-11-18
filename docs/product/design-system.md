# AsterLab 设计系统

本文是 AsterLab UI 的唯一视觉与语义规范。任何 UI 改动开工前必读，验收时必须能引用本文条目。

权威关系：通用颜色/间距/圆角/字号由 `Tokens` 定义；具备科学语义的颜色由本文登记的具名调色板提供。本文规定使用方式与允许的例外；实现与本文冲突时，以对应的 `Tokens` 或具名调色板函数为准，并修本文。

## 1. 单一参数源

所有通用视觉参数来自 `theme.GetTokens(c.Theme())`；唯一例外是本文件登记的具名科学语义颜色调色板。禁止在调用点写字面值。

| 类别 | 唯一来源 | 禁止写法 | 正确写法 |
|---|---|---|---|
| 颜色 | `Tokens`（`Canvas/Surface/Elevated/Border/Text/TextMuted/Accent/Success/Warning/Danger` 及 `*Bg/*Text`）；具名科学调色板（见 §1 例外） | 在渲染/组件调用点写 `ui.Hex(...)`、`ui.RGBA(...)` | `tokens.Surface`、`tokens.DangerText`；调色板色复用具名函数 |
| 字号 | `TextPage/TextSection/TextBody/TextMeta/TextData`，或 `t.Rem(x)`（随主题缩放） | `FontSize(12)`、`FontSize(11)` | `FontSize(float32(tokens.TextMeta))`、`.FontSize(t.Rem(.85))` |
| 间距 | `Spacing4/8/12/16/24/32/48`，或 `t.Space(n)`（n 是单位倍数；`Theme.Spacing=4`，`t.Space(1/2/3/4/6/8/12)` 对应 `4/8/12/16/24/32/48`） | `Padding(3, 8)`、`Padding(6, 12)`、`Gap(5)` | `Padding(t.Space(2), t.Space(4))`、`Gap(t.Space(1))` |
| 圆角 | `ControlRadius`（控件）/`PanelRadius`（面板）/`DialogRadius`（对话框） | `Radius(3)`、`Radius(5)`、`Radius(7)` | `.Radius(float32(tokens.ControlRadius))` |
| 控件高度 | `ButtonHeight/InputHeight/RowHeight` | `Height(28)`、`MinHeight(33)` | `.Height(float32(tokens.ButtonHeight))` |

颜色字面值只允许出现在 `internal/theme/theme.go`（定义）与确有科学语义的具名调色板函数（如孔板分组色）中；已有同语义调色板时必须复用，不得散落在渲染代码里。

## 2. 间距体系

- 基础单位 4；只用 4 / 8 / 12 / 16 / 24 / 32 / 48。
- 语义嵌套：容器内边距 ≥ 子元素间距；同一层嵌套的上下左右取同一档。
- 相邻同级元素均分剩余空间，不要在个别兄弟上加补偿 padding。
- 行内文字与控件间隔用 4/8；区块之间用 16/24；页面级分隔用 32/48。
- 900×600 下先压缩 32/48 到 24/32，不要压缩到 4 以下。

## 3. 圆角

三档，不做第四种：控件 `ControlRadius`(6)、面板 `PanelRadius`(10)、对话框 `DialogRadius`(12)。同类容器全项目一致。嵌套圆角保持内小外大，不要出现内外同值或内大外小。连续圆角在性能允许时优先；性能不允许时在本文记录例外，不在代码里各写各的。

## 4. 组件语义

先选组件，再套样式；不要给错误的组件换皮。来源：`github.com/oneo-me/onevra/source/components`（本地 `~/go/pkg/mod/github.com/oneo-me/onevra/source@*/components/`）、mygo `ui.*` 原语、仓库 `internal/uikit` 与各 `internal/<domain>` 包。

| 需求 | 唯一组件 | Do | Don't |
|---|---|---|---|
| 主/次操作 | `components.Button`（Primary/Secondary/Ghost 三档） | 一个表面只有一个 Primary | 用 `ui.ButtonBase` 手搓按钮外观 |
| 图标操作 | `components.IconButton` + `lucide.*` | 提供 tooltip 文案 | 用文字按钮做图标动作 |
| 分段切换 | `components.TabsSegmented` | 视图切换（流程/孔板/配液单） | 自己写一排可点 Box |
| 状态标签 | `components.Badge` | 用 §3 状态词 | 自造彩色圆点标签 |
| 表格 | `components.Table` / `TableVirtualized` | 记录、参数表 | 用 Column+Row 拼表头 |
| 键值对 | 标签 `TextMeta` + 值 `TextData/TextBody`，值加 `tnum` | 数值对齐 | 标语式大字号 |
| 空态 | 一行 `TextMeta` + 一个下一步动作 | 说清下一步 | 插画 + 营销文案 |
| 错误/警告 | `Badge` + 行内横幅（`*Bg`/`*Text` token） | 原因 + 重试 | 只弹 Toast 就消失 |
| 对话框 | `components.Dialog`（`DialogRadius`） | 破坏性动作二次确认 | 自建浮层 |
| 侧栏/导航 | `components.Sidebar` / `components.Tabs` | 沿用既有 shell | 每页新写一套导航 |
| 进度/预算 | `components.Progress` 或既有 `budgetBar` | 数值来自单一来源并标注来源 | 同一指标两套组件 |
| 骨架屏 | `components.Skeleton` | 首屏加载 | 长时间显示「加载中…」而无结构 |

规则：同类控件全项目只有一种用法。需要变体时，在既有组件上加参数，不新写第二个。

## 5. 文案

- 先定页面定位，再写字。一个页面只有一个标题。
- 只保留核心标题、关键键值对与必要的动作标签；说明性文字能删就删。
- 书面语，禁止营销腔、感叹号、emoji、口语（「搞定」「一键起飞」）。
- 低频说明折叠进 tooltip / alert / 悬停详情 / 可展开行，不常驻。
- 状态词只能取 `ux-contract.md` §3 的词表，不自造同义词。
- 单位与数值：数值右对齐 + `tnum`；单位与数值之间留一个空格。

## 6. 反模式基线（2026-11-16，`ad8ed02`）

以下为当前代码里真实存在的违规计数（`prototypes/mygo-run`，排除 `_test.go`）。清理按这张清单另开卡，不要在功能卡里顺手重构。

| 反模式 | 总数 | 集中位置 |
|---|---|---|
| `ui.Hex(...)` 颜色字面值 | 3 行（定义处 `theme.go` 另有 11 行，允许） | `internal/asterdoc/view.go:735,736,739`（`groupColor` 本地调色板）；`uikit` 已收进 `plate.GroupColor`，无字面值 |
| `FontSize(数字)` | 175 | `internal/runui/runui.go` 80、根目录 40（`main.go` 34、`design_page.go` 4、`design_canvas.go` 2）、`internal/protocol` 25、`internal/runworkspace` 22、`internal/tools` 7、`internal/resultgrid` 1 |
| `Padding(数字)` | 97 | `internal/runworkspace` 31、`internal/runui` 23、根目录 16（`main.go` 6、`design_canvas.go` 5、`design_page.go` 2、`notebook_page.go` 3）、`internal/asterdoc` 10、`internal/resultgrid` 7、`internal/designer` 4、`internal/protocol` 3，另 `internal/shell`/`blockui`/`blocks` 各 1 |
| `Radius(数字)` | 58 | `internal/runworkspace` 30、`internal/runui` 19、`internal/resultgrid` 5、`internal/asterdoc` 3、`main.go` 1 |
| 单参数 `Padding(6)`（6 不是 4 的倍数） | 3 | `internal/resultgrid/view.go:54,60,81`（全项目仅有的三处单参数 `Padding(6)`） |
| 不在 6/10/12 档的 `Radius` | 30 | `runui` 7（`runui.go:630,725,730,755,760,1095` 为 8；`:1139` 为 5）、`runworkspace` 15（`recording.go:138,633,720`；`view.go:536,561,628,692,731,1013,1029,1096,1278,1296,1453,1481`）、`resultgrid` 4（`plate.go:124,474` 为 5；`:162` 为 4；`:417` 为 8）、`asterdoc` 3（`view.go:198,371` 为 4.5；`:567` 为 3）、`main.go:3140`（4） |

已合规的范式：`internal/uikit/plate/plate.go:277` 以 `t.ControlRadius` 设控件圆角；`internal/runworkspace/view.go` 以 `tokens.PanelRadius` 设面板圆角。间距用 `t.Space(n)`，字号缩放用 `t.Rem(x)`。
`internal/designer/step_list.go:528,558` 当前用 `Radius(t.Space(...))`；这属于现存的不合规圆角用法，不应复制为范例。代码清理不在本次文档卡范围内。

## 7. 验收方式

UI 卡的 PR 必须带 `## 设计一致性` 小节，写明：

1. 用到的 token（列出实际引用，例如 `tokens.Spacing16`、`tokens.TextMeta`）；
2. 复用的既有组件（列名 + 文件）；
3. 新增组件（若有）：名字 + 为何仓库与 onevra 都没有等价实现；
4. 若引入本文未列的新做法，先改本文并说明理由。

可机器检查的部分（后续可加进本地门禁）：

```text
grep -rn "ui\.Hex(" prototypes/mygo-run --include=*.go | grep -v internal/theme/    # 应为 0（或仅具名调色板函数内）
grep -rn "FontSize([0-9]" prototypes/mygo-run --include=*.go                      # 应为 0
grep -rnE "Padding\(([0-9]+, )?[0-9]+" prototypes/mygo-run --include=*.go         # 应为 0
grep -rn "Radius([0-9]" prototypes/mygo-run --include=*.go                        # 应为 0
```

## 8. Do / Don't 范例

每条取自本仓库真实代码或真实违规点，可直接对照。

**D1 颜色**
- Do: `components.Text(c, title).TextColor(tokens.Text)`；分组色复用 `plate.GroupColor`（`internal/uikit/plate/plate.go:86`，内部已用 token，无字面值）。
- Don't: 在视图里自建调色板（`internal/asterdoc/view.go:734-739` `groupColor`，7 个 `ui.Hex` 字面值）；应复用 `plate.GroupColor` 或收进 `theme`。

**D2 字号**
- Do: `.FontSize(t.Rem(.85))` 或 `.FontSize(float32(tokens.TextMeta))`。
- Don't: `.FontSize(12)`（`internal/runui/runui.go` 有 80 处；主题字号变化时它们不跟随）。

**D3 间距**
- Do: `.Padding(t.Space(2), t.Space(4))`。
- Don't: `.Padding(6, 12)`（`design_page.go:37`、`design_canvas.go:111`）——6 不是 4 的倍数，且绕过了 `t.Space` 的缩放。

**D4 圆角**
- Do: `.Radius(float32(tokens.ControlRadius))`。
- Don't: `.Radius(8)`（`internal/runui/runui.go:630,725,730,755,760,1095`；另 `:1139` 为 `Radius(5)`）——不在 6/10/12 三档内，同类容器随后各写各的。

**D5 按钮**
- Do: `components.Button(c, "创建 Run", components.ButtonPrimary)`，一屏只有一个 Primary。
- Don't: `ui.ButtonBase(c).Radius(8).Padding(6, 12)` 手搓按钮（`internal/runui/runui.go:630`）。

**D6 分段控件**
- Do: `components.Tabs(c, &index, components.TabsSegmented, ...)`。
- Don't: 自己写一排可点 `ui.Box` 冒充分段控件。

**D7 状态标签**
- Do: `components.Badge` + `ux-contract.md` §3 的词（`草稿`/`未保存`/`校验中`/`未解析`）。
- Don't: 自造色点 + 新词（如把 `校验中` 写成「检查中」）。

**D8 错误呈现**
- Do: 行内横幅（`tokens.DangerBg`/`DangerText`）+ 原因 + `重试`。
- Don't: 只 `c.AddToast(...)` 让原因一闪而过。

**D9 表格与数值**
- Do: `components.Table`；数值列 `.FontFeatures("tnum")` 右对齐。
- Don't: 用 `Column`+`Row` 手拼表头，或数值左对齐导致列抖动。

**D10 空态**
- Do: 一行 `TextMeta` 说明 + 一个明确的下一步动作。
- Don't: 插画 + 营销文案，或只有「暂无数据」而不给出口。

**D11 文案**
- Do: 标题「实验设计」、值「9/12 孔」、动作「创建 Run」。
- Don't: 「一键搞定你的实验！」这类营销腔，或常驻的三段说明文字（应折叠进 tooltip）。

**D12 新增组件**
- Do: 先在 onevra `components/` 与仓库既有包里找等价实现，复用或加参数。
- Don't: 每个页面新写一套视觉词汇（P3 直接失败）。

---

## 五、AsterML v2 通用模块组件映射表（2026-11-18）

`prototypes/mygo-run` 的 AsterML v2 工作台（Design 三标签页 + 同源 `ModeRun` 执行）严格遵循上述规则，所有通用内容模块优先复用社区库组件与统一 `Tokens`：

| 场景 / 通用模块 | 复用组件与包 | 关键规范 |
|---|---|---|
| 步骤工作台分栏 / 源码双栏预览 | `onevra/components.Split` · `SplitPercent` | 左侧步骤大纲固定像素可调，右侧源码与实时预览按百分比分割 |
| 插入 10 类通用模块 / 添加全局模块 | `onevra/components.MenuButton` + `Tooltip` | 图标与语义菜单分离；快捷操作只置 tooltip 不写三段常驻说明 |
| 参数表 / 分组表 / 数据表 / 计算器结果 | `onevra/components.Table` + `TableRow` | 数值列启用 `.FontFeatures("tnum")`，质控边界用 `Badge` 语义色调 |
| AsterML 源码编辑 | `mygo/ui/editor`（Markdown 高亮、行号、DiffBase） | 与右侧 `asterdoc.ModeProtocol` 实时预览同屏联动 |
| 示意图 / 培养皿分区 SVG | `mygo/ui.ParseSVG` + `ui.Image` + `<text>` 叠加 | 纯视觉辅助表达；分组与量化数据严格归属 `group_table` / `data_table` |
| 孔板 / 培养皿 / 1D 管架网格 | `internal/uikit/plate` | 复用 `theme.Palette` 与语义分组色，`ui.Hex` 只保留在 `theme.go` |
| 步骤内模块拖拽排序 | `ui.Box`（`.Drag` / `.DragOver` / `.Drop`）+ `lucide.GripVertical` | 手柄固定于模块标题栏左侧；落位复用 `MoveStepBlockToIndex` 与主题高亮边框 |
| 图像与实拍附件查看 | `components.Dialog` Lightbox + `ui.ScrollBoth` + `x/image/tiff` | 异步后台解码防阻塞主线程；提供 `75%/100%/150%/200%` 缩放与外部打开，支持 Design 拖入文件 |
| 通用数据表粘贴与键盘导航 | `components.Table` + `ui.TextField`（`FocusBind` / `OnPaste` / `OnTextKey`） | 支持 Excel TSV/CSV 粘贴自动扩行与方向键/`Enter`/`Tab` 移焦；提交成功显示 `已保存至 Core` `Badge` |
| 计算器实测联动 | `EvalCalculatorWithActuals` + `components.Badge`（`Accent`） | 在 Run `ModeRun` 自动代入 `actual_parameters` 与上游测量值并标记 `已代入实测`；低浓度自动标注 `insufficient` |
