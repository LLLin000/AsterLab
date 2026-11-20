# 桌面端工作台、平权分屏与储藏室

桌面端以「**三个主要表面 + 平权四向分屏**」组织全部科研界面，去除多余顶栏工具堆砌与二级模式弹窗。

## 三大工作区表面

1. **左栏（40px Ribbon + Vault 文件树）**
   - 文件树依据显式 `depth` 绘制纵向缩进引导线，支持 `F2` 内联重命名、文件右键菜单，以及直接将 `.asterml` 文件拖入日程改期或拖入编辑区边缘拆屏。
2. **主区（`PaneNode` 平权分屏树）**
   - 任意叶窗格均可平权托管 `实验日程`、`储藏室货架` 或 `.asterml` 文档（`编辑 | 阅读 | 源码`），拖拽页签至窗格四周 25% 热区即可通过 `LiquidSplitTransition` 动态四向分屏。
   - 多个窗格同时打开同一 `.asterml` 文件时共享同一内存 `*vault.LiveDocument`，编辑与撤销完全同步。
3. **右伴侣抽屉（默认折叠）**
   - 仅收拢与当前实验直接相关的「步骤大纲」与「速算与计时」两项辅助工具，不承载全域数据或设置。

## 分屏与 UnifiedLive 三态视图

| 桌面分屏 + 右伴侣抽屉（Light） | 桌面分屏 + 右伴侣抽屉（Dark） |
|---|---|
| ![桌面分屏 Light](../sdk/evidence/v2-visual-catalog/workbench_obsidian_split_drawer_light.png) | ![桌面分屏 Dark](../sdk/evidence/v2-visual-catalog/workbench_obsidian_split_drawer_dark.png) |

| UnifiedLive「编辑」态 | UnifiedLive「阅读」态 | UnifiedLive「源码」态 |
|---|---|---|
| ![UnifiedLive 编辑态](../sdk/evidence/v2-visual-catalog/unified_live_edit_light.png) | ![UnifiedLive 阅读态](../sdk/evidence/v2-visual-catalog/unified_live_read_light.png) | ![UnifiedLive 源码态](../sdk/evidence/v2-visual-catalog/unified_live_source_light.png) |

| 方案灯箱放大 (`figure@1`) | Notebook ELN 双栏对照 |
|---|---|
| ![方案灯箱放大](../sdk/evidence/v2-visual-catalog/design_iz_figure_zoom_light.png) | ![Notebook ELN 双栏对照](../sdk/evidence/v2-visual-catalog/notebook_eln_split_light.png) |

## 实验日程、`Ctrl+K` 命令面板与设置中心

- **实验日程（Home）**：默认提供月视图与周时间网格，支持直接从左栏拖拽实验改期、点击空白格快速建实验，以及右键变更状态。
- **全局命令面板 (`Ctrl+K` / `Ctrl+O`)**：统一检索实验文件、储藏室试剂、常用操作命令及实时主题/毛玻璃开关。
- **设置中心 (`Ctrl+,`)**：内置关键字筛选，分为 6 类清晰配置项。

| 实验日程月视图（Light） | 实验日程月视图（Dark） | 实验日程周网格（Light） |
|---|---|---|
| ![日程月视图 Light](../sdk/evidence/v2-visual-catalog/home_calendar_month_light.png) | ![日程月视图 Dark](../sdk/evidence/v2-visual-catalog/home_calendar_month_dark.png) | ![日程周视图 Light](../sdk/evidence/v2-visual-catalog/home_calendar_week_light.png) |

| 从模板新建实验 | `Ctrl+K` 全局命令面板 | 设置中心 — 外观开关 |
|---|---|---|
| ![新建实验对话框](../sdk/evidence/v2-visual-catalog/vault_new_exp_dialog_light.png) | ![Ctrl+K 命令面板](../sdk/evidence/v2-visual-catalog/workbench_command_palette_light.png) | ![设置中心外观](../sdk/evidence/v2-visual-catalog/vault_settings_appearance_light.png) |

## 储藏室货架、容器贴纸与独立工具

储藏室通过 `internal/uikit/labsticker` 生成基于 NIH BioArt 矢量化与 EDT 物理白边的试剂耗材图标，并与实验文档内 `material@1` 的 `pantry_code` 双向定位；配套提供 9×9 冻存盒、96 孔 PCR 盒、离心管架孔位盘点及独立孔板 Studio / 速算台。

| 储藏室货架主页（Light） | 储藏室货架（Dark） | 实验器材矢量贴纸全表 |
|---|---|---|
| ![储藏室货架](../sdk/evidence/v2-visual-catalog/tools_pantry_standalone_light.png) | ![储藏室暗黑模式](../sdk/evidence/v2-visual-catalog/comp_16c_labsticker_shelf_dark_v3.png) | ![实验器材贴纸表](../sdk/evidence/v2-visual-catalog/comp_16_labsticker_sheet_v3.png) |

| 9×9 冻存盒（Light） | 9×9 冻存盒（Dark） | 96 孔 PCR 管盒 | 15/50mL 离心管架 |
|---|---|---|---|
| ![9x9 冻存盒 Light](../sdk/evidence/v2-visual-catalog/tools_pantry_box_cryo_9x9_light.png) | ![9x9 冻存盒 Dark](../sdk/evidence/v2-visual-catalog/tools_pantry_box_cryo_9x9_dark.png) | ![96 孔 PCR 管盒](../sdk/evidence/v2-visual-catalog/tools_pantry_box_pcr_96_light.png) | ![离心管架](../sdk/evidence/v2-visual-catalog/tools_pantry_box_falcon_rack_light.png) |

| 试剂详单与消耗追溯 | 独立孔板 Studio | 独立实验速算台 |
|---|---|---|
| ![试剂详单](../sdk/evidence/v2-visual-catalog/comp_16d_pantry_reagent_detail_v3.png) | ![孔板 Studio](../sdk/evidence/v2-visual-catalog/tools_plate_studio_v3.png) | ![独立速算台](../sdk/evidence/v2-visual-catalog/tools_bench_calc_v1.png) |

## 上下文右键菜单 (`ContextMenu`)

低频管理操作与批处理全部下沉至上下文右键菜单，不在卡片表面常态常驻一排文字按钮：

| 文件树右键 | 窗格标签页右键 | 孔板选区右键 | 步骤卡片右键 | 日程事件右键 |
|---|---|---|---|---|
| ![文件树右键](../sdk/evidence/v2-visual-catalog/context_menu_file_tree_light.png) | ![页签右键](../sdk/evidence/v2-visual-catalog/context_menu_pane_tab_light.png) | ![孔板右键](../sdk/evidence/v2-visual-catalog/context_menu_plate_light.png) | ![步骤卡片右键](../sdk/evidence/v2-visual-catalog/context_menu_step_block_light.png) | ![日程事件右键](../sdk/evidence/v2-visual-catalog/context_menu_calendar_light.png) |
