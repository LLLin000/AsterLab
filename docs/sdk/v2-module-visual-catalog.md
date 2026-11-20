# AsterLab v2 全量模块、Obsidian 单库工作台与物理动效实图图鉴（21 个 SDK 模块）

状态：已实现（`feat/v2-generic-modules` · 2026-11-20 更新至 Obsidian 单库单真源 + 4 档物理动效）

> [!NOTE]
> 本文档中的所有截图均在分支 `feat/v2-generic-modules`（[design-system.md](../../docs/product/design-system.md) / [ux-contract.md](../../docs/product/ux-contract.md) / [asterdoc-contract.md](../../docs/asterdoc-contract.md)）通过真实无头 MyGo UI 交互测试与 PyMuPDF A4 导出器实时生成。逐个对照**第一层 11 个通用内容模块**、**第二层 10 个声明与文档原语模块**、**独立「储藏室货架」工作台**、**Obsidian 单库平权分屏与 `UnifiedLive`（`编辑 | 阅读 | 源码`）单真源工作台**、**Apple Calendar 风格 `实验日程`与 `Ctrl+K` 命令面板**，以及**组件级真实 A4 PDF 导出**。全表面架构复盘与全量测试/构建账本见 [AsterLab v2 通用模块架构演进复盘与全表面实图验收档案](./v2-architecture-retrospective-and-evidence.md)。

---

## 第一部分：第一层通用内容模块（11 类步骤内可自由组合组件）

### 1. `prose@1` — 富文本 / 注意事项提示块

- **核心语义**：纯视觉/文本表达块（不承担结构化参数）；支持 `tone`: `plain` | `note` | `tip` | `warning`，支持段落与项目符号渲染。
- **渲染与闭环实现**：[widget_prose.go](../../prototypes/mygo-run/internal/asterdoc/widget_prose.go)

![prose@1 Design 编辑态与 Run 四种语气实图](./evidence/v2-visual-catalog/comp_01_prose.png)

- **验收状态**：
  - ✅ 所有模块卡片顶部统一支持 Design 内联铅笔图标点击重命名（`renderInlineEditableTitle`）并写回 `.asterml` `title`；`prose` 在 Design / Run / 实验记录 / PDF 四端同源渲染。

---

### 2. `group_table@1` — 通用实验分组表

- **核心语义**：集中定义实验组别、科学角色（`control` / `treatment` / `comparator` / `reference`）、处理条件、独立样本数 `independent_n` 与技术重复 `technical_repeats`。
- **渲染与闭环实现**：[widget_tables.go](../../prototypes/mygo-run/internal/asterdoc/widget_tables.go)

![group_table@1 Design 编辑态与只读呈现态实图（顺序 8 色调色盘 + 完整中文角色标签 + 无裁切列宽）](./evidence/v2-visual-catalog/comp_02_group_table.png)

- **已修复并实图确认**：
  - ✅ `角色` 列改为纯科研中文徽章（`对照组` / `处理组` / `阳性对照` / `参照组`）并加宽列宽；`n / 重复` 列无省略号裁切；
  - ✅ 组别圆点统一使用顺序 8 色 `CategoricalPalette`（`GroupColorIndexed`），各组颜色与关联 `plate@1` 孔板严格一致，并在 `group_mastermix` 中自动解析为人类可读中文组名。

---

### 3. `param_table@1` — 通用参数表（默认标题「实验条件」）

- **核心语义**：定义操作条件（键、标签、计划值、单位、`record_actual`、备注）；Run 执行时可补录实测值并供下游 `calculator@1` 引用（`@actual.<key>`）。
- **渲染与闭环实现**：[widget_tables.go](../../prototypes/mygo-run/internal/asterdoc/widget_tables.go) · [composer.go](../../prototypes/mygo-run/internal/asterdoc/composer.go)

![param_table@1 Design 默认标题「实验条件」+ Run 实测录入态实图](./evidence/v2-visual-catalog/comp_03_param_table.png)

- **已修复并实图确认**：
  - ✅ 默认回退标题与 Starter 模板统一收口为简洁自然的「**实验条件**」，支持卡片标题栏内联点击编辑自定义写回 `.asterml`。

---

### 4. `figure@1` — 图片 / SVG 示意图（便携 `./assets/` 相对路径 + Agent 物理比例尺绘制规范）

- **核心语义**：纯视觉/空间结构表达（由 Agent 或科研人员根据处理数与实物尺度绘制 90 mm 琼脂培养皿、牛津杯/纸片排布、离心分层、Transwell 腔室、WB/凝胶泳道等 `figure@1.svg`，或挂载本地参照照片），**绝不**作为结构化实验数据唯一数据源。
- **渲染与绘制规范**：[widget_figure.go](../../prototypes/mygo-run/internal/asterdoc/widget_figure.go) · [agent-svg-kit.md](./agent-svg-kit.md) · `docs/sdk/svg-parts/*.svg`

![figure@1 90 mm 琼脂培养皿 1 mm = 2.4 px 矢量 SVG 示意图（g transform 坐标系叠加 + 中文标签居中防裁切）与便携 ./assets/ 图片实图](./evidence/v2-visual-catalog/comp_04_figure_v2.png)

- **已升级并实图确认**：
  - ✅ **物理尺度与 `<g transform="translate(...)">` 全链修通**：像抑菌圈、涂布分区等连续平皿排布不再伪装成离散 `plate@1` 孔板，统一回归 `figure@1.svg`（标准 `viewBox="0 0 360 280"`、`1 mm = 2.4 px`、外圆 `r="108"`、6 mm 滤纸片 `r="7.2"`、极坐标 `(±44, ±44)` 对称布片）；[widget_figure.go](../../prototypes/mygo-run/internal/asterdoc/widget_figure.go) 与 [pdf_sheet.go](../../prototypes/mygo-run/internal/asterdoc/pdf_sheet.go) 新增 `<g transform="translate(tx, ty)">` 坐标栈累加与 `estimateSVGTextWidth` 中文角标宽度测算；
  - ✅ **可复用部件库落地**：新增 [agent-svg-kit.md](./agent-svg-kit.md) 与 6 个标准积木（`petri-dish-90mm.svg`、`oxford-cup-and-disc.svg`、`centrifuge-tube-layers.svg`、`transwell-insert.svg`、`gel-wb-lanes.svg`、`arrows-and-scalebar.svg`），同时打通 `View.BaseDir` 与 `PortableFigureAssetPath` 自动将本地图片拷入 `./assets/`。

---

### 5. `data_table@1` — 通用数据记录表（列表 + 2D 孔板矩阵 + 一键插入公式列）

- **核心语义**：支持自定义列（`text` / `number` / `categorical` / `formula`）、列单位与质控范围警戒（`min` / `max`）；绑定 `container_ref` 后支持 `layout: matrix` 二维孔板对齐视图与全板 TSV/CSV 矩阵粘贴。
- **渲染与闭环实现**：[widget_tables.go](../../prototypes/mygo-run/internal/asterdoc/widget_tables.go)

![data_table@1 Run 列表态清晰前缀与 2D 孔板矩阵（layout: matrix + 整板粘贴 + 超限标黄）实图](./evidence/v2-visual-catalog/comp_05_data_table.png)

- **已修复并实图确认**：
  - ✅ Design 态列类型下拉全面中文化为 `数值` / `文本` / `分类` / `自动公式`；当选择 `自动公式` 时展示 `点选列插入：` 可点击列键胶囊，点击即可把列 key 插入公式表达式框，无需手工记忆列名；
  - ✅ Run 列表态明确标注 `读数: 正常` 与 `样本: 正常`；关联 `container_ref` 时支持 `列表 | 孔板矩阵` 切换、`粘贴整板矩阵` 与超质控区间金色边框提示（如 `B2 = 2.640`）。

---

### 6. `checklist@1` — 检查清单（支持必检与规则豁免）

- **核心语义**：步骤内可任意插入的操作核对单；支持 `required` 必检项与 `allow_rule_skip` + `skip_rule`（Run 可一键按规则豁免并记入事件追踪）。
- **渲染与闭环实现**：[widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go)

![checklist@1 Design 编排态与 Run 勾选/规则豁免态实图](./evidence/v2-visual-catalog/comp_06_checklist.png)

- **验收状态**：✅ Design 增删项与 Run 勾选/按规则豁免全链路通过。

---

### 7. `calculator@1` — 通用科学计算器（5 合 1 能力模式 + Design 写回）

- **核心语义**：用单一计算器契约收口 `dilution`、`serial_dilution`、`group_mastermix`（兼容 `container_ref` 与 `plate_ref`）、`sample_normalize`（`target_mass` / `volume_per_unit`）与 `formula`（自由公式 + 结果 `unit`），支持 `@actual.*` 自动代入上游实测值。
- **渲染与闭环实现**：[widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go) · [calc_eval.go](../../prototypes/mygo-run/internal/asterdoc/calc_eval.go)

![calculator@1 稀释与梯度稀释模式实图](./evidence/v2-visual-catalog/comp_07_calculator_dilution_serial.png)

![calculator@1 分组混配(container_ref)、样本归一化与自由公式(18.8 mm)实图](./evidence/v2-visual-catalog/comp_08_calculator_mastermix_norm_formula.png)

- **已修复并实图确认**：
  - ✅ Design 态支持顶部 `计算模式` 下拉切换、`sample_normalize` / `formula` 参数输入框与 `保存计算设定` 写回 `.asterml`；
  - ✅ `calc_eval.go` 与 `pdf_sheet.go` 接入 `PlateGroupLabelMap(doc)`，在 `group_mastermix` 结果行与导出 PDF 配方表中优先显示人类可读中文组名（如 `对照组 体系 (9 孔/份)`），不再泄露 `g-ctrl` 等内部 ID。

---

### 8. `timer@1` — 通用实验计时器（4 种模型 · 中文运行状态 · Design 可编辑）

- **核心语义**：支持 `countdown`、`stopwatch`、`cycle`（多轮循环阶段）、`window`（相对 `t0` 时间窗提醒），Run 直接驱动 Core 持久化计时状态机。
- **渲染与闭环实现**：[widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go) · [asterdoc.go](../../prototypes/mygo-run/internal/asterdoc/asterdoc.go)

![timer@1 四种计时模型（中文化「计时中」+ window 06:00:00 + Design 内联设定）实图](./evidence/v2-visual-catalog/comp_09_timer_all_modes.png)

- **已修复并实图确认**：
  - ✅ Run 运行状态中文化（`计时中` / `已暂停` / `已完成` / `待启动`）；
  - ✅ `window` 模式同时回退读取 `offset` 与 `duration`，主读数准确显示 `06:00:00`；Design 态支持模式、数值/单位、轮数内联编辑并保存写回。

---

### 9. `attachment@1` — 文件附件与容器多视野采图槽

- **核心语义**：统一步骤证据（`step_evidence`）、设计参考文档（`design_ref`）与按容器坐标组织的采图槽（`container_grid`）。
- **渲染与闭环实现**：[widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go)

![attachment@1 步骤照片挂载与 container_grid 孔位进图芯片（A1 2/6、A2 0/6）实图](./evidence/v2-visual-catalog/comp_10_attachment.png)

- **已修复并实图确认**：
  - ✅ 通过 `resolveFigurePath(v.BaseDir, ...)` 支持 `./assets/...` 相对路径预览缩略图；
  - ✅ `container_grid` 在卡片内展示紧凑型孔位采图进度胶囊（`A1 2/6`、`A2 0/6`、`A3 0/6`），Run 态点击指定孔位胶囊可直接关联该孔图像。

---

### 10. `subflow@1` — 内联 / 共享子流程

- **核心语义**：支持在一个步骤内折叠或展开一组有序子操作明细；Design 支持内联增删条目并写回 `.asterml`，Run 可逐项勾选完成。
- **渲染与闭环实现**：[widget_ops.go](../../prototypes/mygo-run/internal/asterdoc/widget_ops.go)

![subflow@1 Design 内联增删子步态与 Run 勾选执行态实图](./evidence/v2-visual-catalog/comp_11_subflow.png)

- **已修复并实图确认**：✅ Design 态卡片支持每行删除按钮及底部 `新增子步骤描述 + 备注 + 添加子步` 表单直接写回 `.asterml`。

---

### 11. `plate@1` — 全仓唯一统一的孔板与管条排布标注系统

- **核心语义**：以同一套 `plateui.View` + `ApplyUnifiedPlateAnnotation` + `AppendPlateAnnotation` 组件承接 **7 类标准离散实验容器**：`6 孔板` / `12 孔板` / `24 孔板` / `48 孔板` / `96 孔板` / `384 孔板` 及 `8 联管 / 泳道`（`rack_1d`）；连续琼脂平皿已彻底抽离至 `figure@1.svg`。
- **渲染与交互实现**：[uikit/plate/plate.go](../../prototypes/mygo-run/internal/uikit/plate/plate.go) · [uikit/plate/export.go](../../prototypes/mygo-run/internal/uikit/plate/export.go) · [asterdoc/plate_ops.go](../../prototypes/mygo-run/internal/asterdoc/plate_ops.go) · [asterdoc/view.go](../../prototypes/mygo-run/internal/asterdoc/view.go)

![plate@1 自适应孔径实图：6 孔板 / 12 孔板 / 24 孔板 / 8 联管竖向胶囊（纯孔板与管条家族、无英文维度术语）](./evidence/v2-visual-catalog/comp_12_plate_geometries_v3.png)

![plate@1 顶部图层胶囊（[分组] / [浓度梯度] / [+]）+ 选孔底条「单值 | 平铺列表 | 等比序列」与物理动效实图](./evidence/v2-visual-catalog/plate_layers_serial_bar_light.png)

![plate@1 96 孔 qPCR 内联卡：中文分组图例 + 精简单框悬浮条实图](./evidence/v2-visual-catalog/comp_13_plate_96_v3.png)

![plate@1 放大编辑浮层：整行快选与精简底部悬浮条实图](./evidence/v2-visual-catalog/comp_13b_plate_overlay_bar_v3.png)

- **核心升级与闭环落地**：
  1. **聚焦 7 类真实孔板/管条与自然科研文案**：`6/12/24/48/96/384` 及 `8 联管/泳道`，副标题统一为 `N 孔板 · 已排 K 孔` / `N 联管 / 泳道 · 已排 K 位`；
  2. **UnifiedLive 图层胶囊与三态填充条**：支持顶部 `[分组]` 与自定义图层胶囊丝滑 Oklab 变色切换及 `[+]` 新建图层；选中孔后底部托出 `单值 | 平铺列表 | 等比序列`（含 `÷2/÷5/÷10` 与按行/按列）填充条，并配备完整孔板右键菜单（清空、复制/粘贴标注、反选、边缘孔全选、标记异常孔 `excluded_wells`、导出 CSV/PNG）。

---

## 第二部分：第二层科学原语与文档声明模块（10 类顶层结构模块）

包含 `experiment@1`、`group@1`、`assay@1`、`workflow@1`、`instrument_profile@1`、`material@1`、`sample@1`、`reference@1`、`step@1`、`readout@1`。

![10 类文档顶层与第二层科研原语模块阅读态渲染实图（最末为 readout.viewer 专属视图卡）](./evidence/v2-visual-catalog/comp_14_top_level_doc_cards.png)

- **已确认**：✅ 补齐 `readout@1` 的专属阅读态视图 `readout.viewer`（上图最末卡直接呈现终点 `OD450` + `[孔板矩阵]` `[AU]` `[96 孔]` 胶囊），全部 10 类第二层原语均具备专属结构化可读卡片；同时在 Design 界面彻底移除了旧的第 4 个全局通用表格编辑器页签。

---

## 第三部分：独立「储藏室货架」工作台（已去冗余收口 · BioRender 容器/盒子贴纸 + 盒内孔位）

随 2026-11-20 Obsidian 单库重构，原“资料与工具 4 合 1 杂烩页签”已彻底收口：**模板/标准方案并入 `<Vault>/templates/` 文件树与新建实验选择框，孔板/速算全面融入实验文档与右抽屉**；侧栏保留纯粹的 **「储藏室货架」**（`tools.ViewPantry`）：

![独立储藏室货架完整工作台实图（无 4 工具旧侧栏杂项，纯正 BioRender 温区货架与试剂收纳盒）](./evidence/v2-visual-catalog/tools_pantry_standalone_light.png)

![12 款 BioRender 风实验室容器白边贴纸图鉴（基于 NIH BioArt 矢量骨架 + 清透渐变调色 + 2D EDT 刀模白边合成）](./evidence/v2-visual-catalog/comp_16_labsticker_sheet_v3.png)

![5 款 BioRender 风储藏室二级收纳盒与离心管架白边贴纸图鉴（9x9/10x10 冻存盒 · 50孔 EP 管盒 · 15/50 mL 离心管架 · 96孔 PCR 盒）](./evidence/v2-visual-catalog/comp_16e_labsticker_boxes_v1.png)

![储藏室货架贴纸视图浅色与深色模式实图](./evidence/v2-visual-catalog/comp_16b_labsticker_shelf_light_v3.png)

![储藏室货架贴纸视图深色模式实图（纯白刀模背衬与顶光内凹柜格层次分明）](./evidence/v2-visual-catalog/comp_16c_labsticker_shelf_dark_v3.png)

![资料与工具 -> 储藏室货架完整工作台实图（双列等高对齐温区货架 + 5 款收纳盒/离心管架贴纸 + 独立试剂瓶 + 搜索与临期筛选）](./evidence/v2-visual-catalog/tools_pantry_workbench_v4.png)

![点击冻存盒贴纸下钻进入 9x9 孔位空间分布图（Light：A1..I9 孔位矩阵 + 试剂配色圆帽 + 临期警戒 + 右侧孔位详情与盒内清单）](./evidence/v2-visual-catalog/tools_pantry_box_cryo_9x9_light.png)

![冻存盒 9x9 孔位空间分布图（Dark 主题）](./evidence/v2-visual-catalog/tools_pantry_box_cryo_9x9_dark.png)

![96 孔 0.2 mL PCR 盒孔位分布图（B1–B8 八联排整行占位与单孔引物直视）](./evidence/v2-visual-catalog/tools_pantry_box_pcr_96_light.png)

![4°C 15 mL 离心管架（5x5 圆孔视图：直观呈现各位置管名、效期与快速放入）](./evidence/v2-visual-catalog/tools_pantry_box_falcon_rack_light.png)

![点击任意试剂贴纸呼出「试剂档案与说明书」弹窗实图（实况状态徽标 + 更换贴纸 + 所属收纳盒/孔位绑定 + 产品官网 + 本地 PDF 说明书上传查阅）](./evidence/v2-visual-catalog/comp_16d_pantry_reagent_detail_v3.png)

> **原「资料与工具」其余三项的能力去向（2026-11-20 架构去冗余收口）**：
> - 原 `标准方案库` → 统一整合为实验库真实文件夹 `<Vault>/templates/*.asterml`，在左树与「新建实验」弹窗模板选择器直接预览并派生；
> - 原 `孔板速排` → 能力完整收敛入文档内联 `plate@1`（顶部图层切换 + 底部 `单值 / 平铺列表 / 等比序列` + 导出 CSV/PNG + 右键菜单），不再重复维护旁路克隆页；
> - 原 `实验台速算` → 收敛入右侧滑出伴侣抽屉 `速算与计时`（配液速算 + 多计时器）及文内 `calculator@1` 的 `@plate.wells` 联动计算。
> *(下附工具引擎 `sop_library` / `plate_studio` / `bench_calc` 的独立底层能力参考图：)*

<details>
<summary>展开历史独立工具组件参考图（已并入 Vault / 内联孔板 / 右抽屉）</summary>

![标准方案库参考实图](./evidence/v2-visual-catalog/tools_sop_library_v1.png)
![孔板速排工作台参考实图](./evidence/v2-visual-catalog/tools_plate_studio_v3.png)
![孔板速排悬浮放大框选参考实图](./evidence/v2-visual-catalog/tools_plate_studio_overlay_v3.png)
![导出 2x 高清 12 孔培养板图](./evidence/v2-visual-catalog/tools_plate_exported_png_v2.png)
![实验台速算参考实图](./evidence/v2-visual-catalog/tools_bench_calc_v1.png)

</details>

---

## 第四部分：Obsidian 级单库全等分屏、`UnifiedLive` 单真源画布与物理动效实图

1. **Obsidian 级 40px Ribbon + Vault 文件树 + 全等左右分屏 + 极简右抽屉（Light / Dark）**：左侧提供真实磁盘文件树（`.asterlab/`、`assets/`、`templates/`、`<项目文件夹>/*.asterml`）；中心支持左 pane `编辑` + 右 pane `阅读` 平权分屏（拖拽边缘 25% 区域触发液态吸附预演）与共享 100 步 `Undo`/`Redo`；右侧伴侣抽屉默认折叠（只留 `步骤大纲` 与 `速算与计时`）：

![Obsidian 单库透明文件树 + 左编辑右阅读全等分屏 + 右侧步骤大纲实图（Light）](./evidence/v2-visual-catalog/workbench_obsidian_split_drawer_light.png)

![Obsidian 单库透明文件树 + 左编辑右阅读全等分屏深色主题实图（Dark）](./evidence/v2-visual-catalog/workbench_obsidian_split_drawer_dark.png)

2. **`UnifiedLive` 单真源画布三态：`编辑 | 阅读 | 源码` + 磁吸单行页眉 + 内联实操组件**：无框 `# 标题` + 单行属性胶囊（日期/项目/状态），无多余卡片外框；**`阅读` 态下的清单勾选与实时计时器依然可直接点击并内联写回 `.asterml`**：

![UnifiedLive 编辑态画布实图（单行面包屑 + 无框标题与属性胶囊 + 步骤块）](./evidence/v2-visual-catalog/unified_live_edit_light.png)

![UnifiedLive 阅读态画布实图（纯净纸面排版 + 可交互勾选与启动计时器 + @plate.wells 自动联动计算）](./evidence/v2-visual-catalog/unified_live_read_light.png)

![UnifiedLive 源码态实时编辑器实图（AsterML Markdown/YAML 源码双向即时同步）](./evidence/v2-visual-catalog/unified_live_source_light.png)

3. **Apple Calendar 风格 `实验日程`（月 / 周）+ 新建实验对话窗 + `Ctrl+K` 悬浮命令面板**：日历与项目目录树完全正交，新建实验支持同步选择 **实验名称 + 保存位置（项目文件夹） + 日期 + 选用模板**（空标题提交触发温柔海绵微晃提醒）：

![新建实验弹窗实图（支持项目文件夹 + 日期 + 模板选择 + 空提交微晃校验）](./evidence/v2-visual-catalog/vault_new_exp_dialog_light.png)

![Apple Calendar 风格实验日程月视图与原地毛玻璃日程 Popover 实图（Light）](./evidence/v2-visual-catalog/home_calendar_month_light.png)

![Apple Calendar 风格实验日程月视图深色主题实图（Dark）](./evidence/v2-visual-catalog/home_calendar_month_dark.png)

![Apple Calendar 风格周视图实图（全天条 + 08:00–20:00 时间轴块 + 今日红色当前时间线）](./evidence/v2-visual-catalog/home_calendar_week_light.png)

![Ctrl+K 悬浮命令面板实图（按 .asterml 实验、储藏室试剂与快捷命令分组直达）](./evidence/v2-visual-catalog/workbench_command_palette_light.png)

4. **全工作台四档物理动效与微交互闭环（[motion.go](../../prototypes/mygo-run/internal/theme/motion.go)）**：
   - `MotionMicro`（100ms 触感微缩与悬浮手柄渐显）、`MotionSnappy`（160ms 零回弹磁吸胶囊 / Oklab 孔板染色 / 选孔底条托出）、`MotionFluid`（220ms 手风琴折叠 / 四向 25% 分屏液态预览 / `NumericRoll` 数字滚轮）、`MotionHero`（260ms 图表与大孔板一镜到底缩放及日历网格切换）以及 `ApplyNudge`（空提交/非法梯度参数 `±5px` 海绵微晃提醒）；全部动效严格采用 Oklab 色彩插值与 `Preferences.ReduceMotion` 无障碍降级。

---

## 第五部分：组件级真实 A4 PDF 渲染导出（设计方案 + 完整实验实测记录 v2 实图校验）

[pdf_sheet.go](../../prototypes/mygo-run/internal/asterdoc/pdf_sheet.go) 基于 `RenderDocumentSheet` 同一份结构化文档模型实现分页排版、矢量表格跨页重复表头、高精向量孔板与修正 `<g transform="translate">` + 中文字符宽测算后的纯矢量 SVG 图形渲染，并通过 `PlateGroupLabelMap(doc)` 解决组别 ID 还原；`ExportDocumentPDF` 同时驱动设计与记录双页统一的 **`导出 PDF`**（输出完整方案、实测参数、计算器实测代入、✓ 检查勾选、已填测量表、Markdown 笔记与内嵌 `asterlab` 模块）：

1. **RT-qPCR 多靶点实验真实导出 A4 `.pdf`（2 页，已显示人类可读中文分组配液行名 + 96 孔三复孔彩板）**：

![RT-qPCR 多靶点实验真实导出 A4 PDF v2 实图（对照组/VEGF 诱导组/抑制剂干预组体系用量清晰列出 + 96 孔三复孔矢量图）](./evidence/v2-visual-catalog/pdf_export_rt_qpcr_real_v2.png)

2. **四象限抑菌圈实验设计 A4 `.pdf`（2 页，抑菌圈平皿回归高清晰 90 mm 琼脂平皿 `1 mm = 2.4 px` 纯矢量 SVG + 双径测量记录表）**：

![四象限抑菌圈实验真实导出 A4 PDF v2 实图（第 2 页纯净 90 mm 琼脂皿四象限滤纸片 SVG 示意图与中文角标完整居中不裁切）](./evidence/v2-visual-catalog/pdf_export_inhibition_zone_real_v2.png)

3. **HUVEC 成管实验真实导出 A4 `.pdf`（2 页，12 孔三浓度 GelMA 排板图 + VEGF 稀释/配平双计算器表格 + 成管定量汇总表）**：

![HUVEC 成管实验真实导出 A4 PDF v2 实图（12 孔三浓度水凝胶排板矢量图 + VEGF 稀释与 6900 uL 配平表及成管定量表）](./evidence/v2-visual-catalog/pdf_export_huvec_real_v2.png)

4. **四象限抑菌圈「完整实验记录」真实导出 A4 `.pdf`（3 页：`pdf_export_iz_run_record_real_v2.png`——第一/二节水合实测参数、✓ 绿色勾选与计算后数据表，第三节渲染笔记正文及内嵌 `param_table` 实测表）**：

![四象限抑菌圈完整实验记录真实导出 A4 PDF v2 实图（实测参数、✓ 勾选框、已填数据表与第三节实验笔记及内嵌模块同源输出）](./evidence/v2-visual-catalog/pdf_export_iz_run_record_real_v2.png)

---

## 第六部分：Obsidian 设置中心、全工作台右键上下文菜单与 MyGo 移动端自适应实图归档

1. **Obsidian 风格可搜索设置中心（[settings.go](../../prototypes/mygo-run/internal/shell/settings.go) · `Ctrl+,`）**：支持实时搜索、明暗/跟随系统主题即时切换、字号缩放、减弱动效开关、`.asterml` 默认打开模式、默认板型/余量、全量快捷键总览/恢复默认，以及移动端紧凑预览开关，设置持久化写入 `<Vault>/.asterlab/config.json`：

   ![Obsidian 风格设置中心（外观与主题 · 浅色主题即时配置）](./evidence/v2-visual-catalog/vault_settings_appearance_light.png)

   ![Obsidian 风格设置中心（快捷键总览与恢复默认 · 深色主题）](./evidence/v2-visual-catalog/vault_settings_hotkeys_dark.png)

2. **右键上下文菜单全表面实时归档（隐藏高频能力不堆砌按钮，全由右键呼出）**：

   ![左树项目/实验文件右键菜单实图（打开 / 新标签页 / 左右下分屏 / 创建副本 / 存为模板 / 重命名 F2 / 删除 Del）](./evidence/v2-visual-catalog/context_menu_file_tree_light.png)

   ![顶部窗格标签栏右键菜单实图（关闭 Ctrl+W / 关闭其他与右侧 / 分屏 / 固定 / 文件树定位）](./evidence/v2-visual-catalog/context_menu_pane_tab_light.png)

   ![96 孔板右键上下文菜单实图（清空选区 / 复制粘贴标注 / 反选 / 选中最外圈边缘孔 / 标记异常孔 / 导出 CSV）](./evidence/v2-visual-catalog/context_menu_plate_light.png)

   ![步骤内模块块级右键菜单实图（剪切 Ctrl+X / 复制 Ctrl+C / 粘贴 Ctrl+V / 创建副本 Ctrl+D / 上下移动 / 删除）](./evidence/v2-visual-catalog/context_menu_step_block_light.png)

   ![实验日程空白日期格右键菜单实图（在此日新建实验）](./evidence/v2-visual-catalog/context_menu_calendar_light.png)

3. **MyGo Obsidian iOS 26 移动端自适应与多主题 Liquid Glass（[mobile.go](../../prototypes/mygo-run/internal/shell/mobile.go) · [theme.go](../../prototypes/mygo-run/internal/theme/theme.go)）**：
   - 主色盘升级为默认纯净白纸 `obsidian`（`#ffffff` + 无边框 `#f5f6f8` 属性/分组卡片 / 暗色 `#111215`），同步支持极简冷银 `zinc`（`#f4f5f7`）与经典暖纸 `warm`（`#f7f6f3`）切换及 `LiquidGlass` 实时开关；
   - 移动端采用左上独立 `40dp` 圆形玻璃钮 + 右上双图标药丸、底部居中纯图标玻璃浮岛（`< > 🔍 + [N] ≡`，编辑态切为纯图标编辑条）、抽屉三层底部控制岛及标签页卡片概览；Dark 模式胶囊采用深烟熏 `glass.Clear` + 亮银高光边避免穿刺发灰，桌面侧栏去除多层玻璃叠加保持高对比：

   ![Obsidian 移动端实验阅读态（390x844 · 独立圆钮 + 无框属性/分组卡片 + 底部居中纯图标 Liquid Glass 穿透孔板）](./evidence/v2-visual-catalog/mobile_experiment_read_obsidian_light.png)

   ![Obsidian 移动端左侧文件树抽屉（#ededf0 圆角选中条 + 底部五图标工具排 + 模式切换胶囊 + 左下库统计与右下独立圆形设置钮）](./evidence/v2-visual-catalog/mobile_drawer_tree_light.png)

   ![Obsidian 移动端实验编辑态（单栏文档 + 底部紧凑纯图标编辑工具条）](./evidence/v2-visual-catalog/mobile_experiment_edit_obsidian_dark.png)

   ![Obsidian 移动端深色阅读态（深烟熏 Clear 液态玻璃胶囊 + 亮银边缘高光透出蓝绿 96 孔板）](./evidence/v2-visual-catalog/mobile_experiment_read_obsidian_dark.png)

   ![Obsidian 移动端实验日程与标签页概览（390x844 · 点状日历 + 底部纯图标胶囊 / 双列标签卡片概览）](./evidence/v2-visual-catalog/mobile_workbench_calendar_light.png)

   ![Obsidian 移动端标签页卡片概览浮层](./evidence/v2-visual-catalog/mobile_tab_switcher_light.png)

   ![桌面端 Obsidian 纯净浅色 / 深空曜石暗色 / Cool Zinc 冷银主题对照（1280x800）](./evidence/v2-visual-catalog/liquid_glass_desktop_obsidian_light.png)

   ![桌面端 Obsidian 深色实验页（去侧栏 Glass-on-Glass 叠加后文字锐利高对比）](./evidence/v2-visual-catalog/liquid_glass_desktop_exp_dark.png)

   ![移动端与桌面端 Cool Zinc 冷银无彩色预设实图](./evidence/v2-visual-catalog/liquid_glass_mobile_zinc_light.png)
