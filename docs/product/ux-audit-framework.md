# AsterLab 五层 UI / UX 与微动效审计框架

> 状态：权威工程与视觉门禁规范（适用于桌面端 Vault、Live Preview 科学组件、Tools 与移动端适配）。

为了避免依赖人工逐点救火或上线后才发现「按钮点了没存上」「文字裁切」「SVG 压线」「说明文字泛滥」，所有涉及界面、组件与交互的改动必须按本五层框架逐层过检。

---

## 五层审计总览

| 层级 | 核心目标 | 验证手段 | 一票否决红线 |
|---|---|---|---|
| **L1 交互闭环层** | 任何可见控件必须「点击 → 写回源码/状态 → 校验通过 → 重绘生效」 | `ui.Tester` 真实点击测试（`tt.Click` / 键入 / `OnChange` 回环断言） | 存在无响应控件、`UpdateModule` 报错被静默吞掉、弹窗按 `Esc` 无法关闭 |
| **L2 几何与视觉矩阵层** | 四视口 × 双主题 × 三密度下排版稳健、像素清晰 | 自动化渲染截图 + 逐张人工/Agent 审图（`1440×900`、`900×640`、`520×900`、`390×844` × 浅/深色 × 空态/常规/极限长文） | 任何中文字符截断、SVG `<text>` 压线或溢出、并排卡片底部不齐、深色模式底透、横向无故裁切 |
| **L3 信息密度与文案层** | 对标 Obsidian 的安静纸面感：让用户专注实验内容而非说明书 | 扫描标题、副标题、设置项描述、空态提示与脚本文案 | 分类页顶部出现长引言、暴露技术/框架术语（`MyGo`/`astercore`/`SHA256`/竞品名）、自解释开关带冗长解释、同一指标头尾重复 |
| **L4 微动效与手感层** | 响应确定、节奏克制、状态连续，严格尊重 `ReduceMotion` | 校验 `internal/theme/motion.go` 接入点与状态过渡行为 | 悬停显现图标引发布局跳动、开关/分段切换无视觉过渡、实时重算数值无变化反馈 |
| **L5 真实环境边界层** | 真实科研工作流、输入法与 Windows 权限下零阻断 | IME/粘贴/撤销测试 + `check_ui_hygiene.py` + `sdk/check.py` + 实机环境核对 | 中文输入法吞字、撤销失败、出现硬编码颜色/微小 `Rem` 字号、权限不足时静默失败 |

---

## 各层可执行细则

### L1 交互闭环层（Click-Through & Schema Round-Trip）
1. **真写回校验**：所有修改 `.asterml` 源码的组件（`plate`、`group_table`、`param_table`、`data_table`、`calculator`、`serial_dilution`、`timer`、`checklist`、`figure`、`attachment`）修改动作后，必须能通过 `asterdoc.Parse` 校验；若需记录 UI 呈现元信息且不属科研契约字段，统一使用 `# @<key>: <value>` YAML 注释指令（并在 `Document.UpdateModule` 中保留），禁止写入未在 `spec/modules.json` 声明的 YAML 键导致校验静默失败。
2. **每个新增/修改控件至少 1 条点击回归**：测试中用真实鼠标点击/输入驱动状态变更，并断言输出源码与下一帧 UI 文本同步变化。

### L2 几何与视觉矩阵层（Viewport × Theme × Density）
1. **强制「先看基线、改后看图、迭代清零」**：UI 优化 PR 严禁只改代码不看 PNG 截图；每次迭代后用 `read_file` 逐张核查截图细节。
2. **并排矩形铁律**：两栏自动并排仅限 `availW >= 640px`、白名单同构/互补组件，且同时满足宽裕度与预估高度比 `<= 1.40`，外层使用 `ui.Stretch` + 卡片 `.FillHeight()`，不并排时立刻优雅回退单栏。
3. **SVG 文字光学对齐**：绘制 SVG 内嵌 `<text>` 时，必须按字母基线补偿 Y 偏移（默认 `py - fsize*0.84`，`dominant-baseline="middle"` 时 `0.52`），避免文字下坠压线；保留合理缩放字号，尊重显式颜色。
4. **长文本兼容**：图注、参数值、说明文本不得用单行输入框强制截断；非聚焦阅读时应完整展示或支持自然换行。

### L3 信息密度与文案层（Obsidian Minimalism Budget）
1. **0 分类引言**：设置中心与面板分类页不设解释性介绍段落。
2. **0 技术黑话**：面向用户的可见文案、设置项与工具提示中，禁止出现内部引擎/框架/实现代号（`MyGo`、`Onevra`、`astercore`、`Pure-Go`、`glass.Group`、外部竞品名称等）。
3. **文案预算**：
   - 自解释标题/开关（如 `液态玻璃`、`减弱动效`、`隐藏 assets 文件夹`）：**0 描述**。
   - 非直观设置或操作提示：**不超过 14 个汉字**，直接说结果不说原理。
4. **静默于休止，现身于需要**：次要编辑按钮（标题 `✎`、行删除 `×`、图注放大镜、高级参数 Popover、图层条）默认隐藏或收敛，只在悬停/聚焦/已有数据时呈现。

### L4 微动效与手感层（Tactile Motion Standards）
所有动画必须复用 [`internal/theme/motion.go`](../../prototypes/mygo-run/internal/theme/motion.go) 令牌并响应 `ReduceMotion`：
1. **Hover Reveal（120–150ms，`MotionMicroFade`）**：用 `Opacity` 控制次要图标显现，绝不在悬停时增删占位宽度造成抖动。
2. **Tactile Press（`ApplyTactilePress`）**：所有胶囊切换、工具按钮、播放键、参数弹窗触发器附带轻量按压回弹反馈。
3. **Color & State Transition（`AnimateColor` / `MotionMicroFade`）**：分段按钮选中底色、检查项完成态文字柔化、滑块开关保持连续渐变。
4. **Numeric Continuity（`NumericRoll` + `tnum`）**：计算结果、计时读数与孔位统计启用 `FontFeatures("tnum")` 与数字滚动/无抖动更新。
5. **Surface Entrance（`DialogSurfaceTransition` / `FloatingBarTransition`）**：弹窗与浮动操作栏轻量缩放渐入，不生硬闪现。
6. **Surface Ladder & Hairline Contrast（表面阶梯与极细边线）**：顶层科学模块卡片使用 `BorderSubtle` 静息描边并在悬停时切换至 `Border`，配合 `ShadowAmbient` 建立柔和景深；多窗格分屏顶部以 `2px Accent` 条与 `Canvas`/`Surface` 底色区分活跃焦点与静默非活跃窗格。
7. **Frequency-Tiered Motion Law（按操作频率分级的动效时长）**：高频切换（分屏标签、右键菜单项悬停、`[ 编辑 | 实验 ]` 切换、卡片边框）使用 `MotionInstant`（`60ms`）；中频反馈使用 `MotionSnappy`；显隐微渐变使用 `MotionMicroFade`。

---

## 滚动帧预算（Scroll Frame Budget）
- **单帧零重复全量解析**：滚动与光标重绘路径必须走 `internal/asterdoc` 的 `RenderFrameCache`（`CachedSplitDocumentSegments`、`CachedParseStepTree`、`CachedParseDocumentOutline`、`CachedResolveRelations`、`CachedSVGMetrics`、`CachedLatexAST`）与 `FastSourceHash` 零分配摘要，禁止每帧重算 AST 或拷贝整篇文档字符串作为缓存键。

### L5 真实环境边界层（IME / DPI / Windows Permissions）
1. **门禁脚本**：`python scripts/sdk/check.py` 与 `python scripts/ci/check_ui_hygiene.py` 必须全绿通过。
2. **平台边界**：Windows 下注意检验高分屏缩放与 NTFS `Low Mandatory Level` 对 Vault 写入的影响（详见 [`beta-feedback-workflow.md`](../engineering/beta-feedback-workflow.md#32-windows-low-mandatory-level-保存失败排查提醒)）。
