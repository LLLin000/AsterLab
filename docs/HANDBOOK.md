# AsterLab Complete Engineering & Product Handbook

> [!NOTE]
> 本文档参考 OpenCode / Oh My Pi（OMP）工程手册范式编排，作为 AsterLab 全仓唯一的**系统全景导航与设计诊断基准**。具体域边界、CLI 字段与视觉硬约束仍以各权威专章为准（见 [docs/README.md](./README.md)）；**后续所有问题排查、新特性设计与 UI/UX 调优，均以本手册的边界、机制、校验门禁与已知缺口清单作为检索入口**。

---

## 目录

1. [产品定位与领域模型总览](#1-产品定位与领域模型总览)
2. [分层架构与代码包全景（Pure-Go Vault × Native Core）](#2-分层架构与代码包全景pure-go-vault--native-core)
3. [核心接口、桥接契约与本地存储协议](#3-核心接口桥接契约与本地存储协议)
4. [AsterML v2 SDK 与 21 模块契约体系](#4-asterml-v2-sdk-与-21-模块契约体系)
5. [UI 视觉设计系统、多主题引擎与 Liquid Glass 物理规范](#5-ui-视觉设计系统多主题引擎与-liquid-glass-物理规范)
6. [桌面端与 Obsidian iOS 26 移动端 UX 交互规范](#6-桌面端与-obsidian-ios-26-移动端-ux-交互规范)
7. [移动端运行边界与双端同步设计决策](#7-移动端运行边界与双端同步设计决策)
8. [基于文档发现问题与持续优化的工作方法（Verification & Hotspot Audit）](#8-基于文档发现问题与持续优化的工作方法verification--hotspot-audit)

---

## 1. 产品定位与领域模型总览

AsterLab 是面向生物/湿实验场景的**以实验流程与单个可追溯文件为中心**的设计、执行与记录工作台，兼顾科研数据严格性与 Obsidian 级无摩擦书写体验。

### 1.1 双层工作流哲学

- **本地 `.asterml` 单库单真源工作区（UnifiedLive）**：
  - 将本地目录作为一个透明 Vault（`<Vault>/`），所有实验都是带 YAML frontmatter 与 ```asterlab 模块围栏的标准 Markdown 文本，配合 `./assets/`（图片/原始数据）与 `templates/`（无日期空白方案）。
  - 同一个 `.asterml` 文件在「编辑 / 阅读 / 源码」三态中直接承载方案设计、内联检查勾选项、倒计时、参数实测偏离、孔板多层标注与读数表——**桌面端负责复杂编排与多窗格对照，手机端负责实验台单手快速查检与原位记录**。
- **严谨审计层三对象（Core Frozen Model，[CONTEXT.md](../CONTEXT.md)）**：
  - 当实验进入受控冻存与强审计链路时，领域模型严格划分为三层不可混淆的对象：`Design`（`DesignVersion` 回答“比较什么与为何做”） → `Protocol`（`ProtocolVersion` 与无环共享 `ProtocolBlockVersion` 回答“具体怎么做”） → `Run`（固定引用某版 Design/Protocol 的不可变一次执行，拥有 `PlannedTask`、`StepOccurrence` 与追加式 `ExecutionEvent`）。
  - **关键科学红线**：Run 数量不等于独立样本量 `n`；技术重复不由容器孔数反推；只读投影（`CurrentDesignMap`、`ProtocolPreview`、`RunWorkspace`）不得假造缺失科学参数。

---

## 2. 分层架构与代码包全景（Pure-Go Vault × Native Core）

AsterLab 现行交付前端为 **MyGo Native（`prototypes/mygo-run`，基于 `mygo@v0.8.4` + `onevra@v0.2026.18`）**，后端同时存在**纯 Go `.asterml` 单库文件引擎**与**本地 Rust Core FFI 审计核心**。

```mermaid
flowchart TB
  subgraph UI["UI & Workbench Shell (MyGo Native + Onevra)"]
    Shell["internal/shell<br/>(Split Panes / Ribbon / Tree / Calendar / Mobile iOS26 / Settings / Ctrl+K)"]
    Theme["internal/theme<br/>(Tokens / 3 Presets / Liquid Glass / 4-Tier Motion)"]
    DocUI["internal/asterdoc<br/>(UnifiedLive Renderers / Plate Layers / PDF Export)"]
    Tools["internal/tools & uikit<br/>(Pantry Shelf / Cryo Boxes / BioRender Stickers / Plate 2D)"]
  end

  subgraph Engine["Pure-Go Vault & Document Engine (零网络/零外部库可独立运行)"]
    Vault["internal/vault<br/>(Vault FS Scan / LiveDocument 100-Undo / WriteFileAtomic / Config)"]
    Spec["internal/asterdoc/spec<br/>(modules.json 21 kinds / units.json UCUM / Go Rules)"]
  end

  subgraph Core["Rust Local Core & CLI (可选强审计 / SQLite 冻结链路)"]
    Bridge["modules/runs<br/>(mygo:native FFI: RunService / DesignService / TimeBudget / MixPlan)"]
    Rust["crates/asterlab-core & src/cli.rs<br/>(SQLite WAL / Frozen Run / labworkbench CLI)"]
  end

  Shell --> Theme
  Shell --> Vault
  DocUI --> Spec
  Shell --> DocUI
  Tools --> Theme
  Vault --> DocUI
  Bridge --> Rust
```

### 2.1 `prototypes/mygo-run` 核心包索引

| 包路径 | 核心职责 | 关键入口文件 |
|---|---|---|
| 根目录 (`main`) | 应用启动、窗口托管、Vault ↔ Shell 绑定、设计/运行宿主页面 | [main.go](../prototypes/mygo-run/main.go)、[vault_host.go](../prototypes/mygo-run/vault_host.go)、[experiment_page.go](../prototypes/mygo-run/experiment_page.go) |
| `internal/shell` | Obsidian 级左 Ribbon + 文件树、平权 `PaneNode` 分屏树、日历Home、`Ctrl+K` 命令面板、`Ctrl+,` 设置中心、**iOS 26 移动端壳** | [shell.go](../prototypes/mygo-run/internal/shell/shell.go)、[tabs.go](../prototypes/mygo-run/internal/shell/tabs.go)、[file_tree.go](../prototypes/mygo-run/internal/shell/file_tree.go)、[calendar.go](../prototypes/mygo-run/internal/shell/calendar.go)、[mobile.go](../prototypes/mygo-run/internal/shell/mobile.go)、[settings.go](../prototypes/mygo-run/internal/shell/settings.go)、[right_drawer.go](../prototypes/mygo-run/internal/shell/right_drawer.go)、[command_palette.go](../prototypes/mygo-run/internal/shell/command_palette.go) |
| `internal/theme` | 唯一通用设计 Token 源（`Obsidian` / `Zinc` / `Warm` 三色盘）、`LiquidGlass` 材质辅助器、科学分类调色板、4 档物理动效 | [theme.go](../prototypes/mygo-run/internal/theme/theme.go)、[motion.go](../prototypes/mygo-run/internal/theme/motion.go) |
| `internal/vault` | 本地单库根目录初始化、目录扫描、模板孵化、回收站与内存级多窗格共享 `LiveDocument`（100 步 Undo/Redo + 外部修改重载） | [vault.go](../prototypes/mygo-run/internal/vault/vault.go)、[live_doc.go](../prototypes/mygo-run/internal/vault/live_doc.go) |
| `internal/asterdoc` | AsterML 解析/编译/校验、所有 21 模块双态渲染、无框 `BorderlessTextInput`、公式求值、孔板图层写回与 PyMuPDF/A4 PDF 导出 | [asterdoc.go](../prototypes/mygo-run/internal/asterdoc/asterdoc.go)、[view.go](../prototypes/mygo-run/internal/asterdoc/view.go)、[widget_tables.go](../prototypes/mygo-run/internal/asterdoc/widget_tables.go)、[widget_ops.go](../prototypes/mygo-run/internal/asterdoc/widget_ops.go)、[widget_figure.go](../prototypes/mygo-run/internal/asterdoc/widget_figure.go)、[plate_ops.go](../prototypes/mygo-run/internal/asterdoc/plate_ops.go)、[inputs.go](../prototypes/mygo-run/internal/asterdoc/inputs.go)、[pdf_sheet.go](../prototypes/mygo-run/internal/asterdoc/pdf_sheet.go) |
| `internal/tools` | 独立「储藏室货架」工作台、低温盒/离心管架孔位分布、独立速算/孔板/模板引擎 | [pantry_shelf.go](../prototypes/mygo-run/internal/tools/pantry_shelf.go)、[pantry_boxes.go](../prototypes/mygo-run/internal/tools/pantry_boxes.go)、[bench_calc.go](../prototypes/mygo-run/internal/tools/bench_calc.go) |
| `internal/uikit` | 跨表面复用孔板核心（`plate.View`）与 NIH BioArt + EDT 白边贴纸生成器（`labsticker`） | `internal/uikit/plate/`、`internal/uikit/labsticker/` |
| `modules/runs` | Go ⇄ Rust `mygo:native` 自动生成胶水接口与原生动态库绑定 | [runs.go](../prototypes/mygo-run/modules/runs/runs.go)、[runs_native.go](../prototypes/mygo-run/modules/runs/runs_native.go) |

---

## 3. 核心接口、桥接契约与本地存储协议

### 3.1 Pure-Go Vault 文件与文档契约（`internal/vault`）

Vault 目录遵循完全透明的文件系统契约，无专有二进制数据库锁定：

```text
<VaultRoot>/
├── .asterlab/
│   ├── config.json         # vault.Config：主题模式/预设/LiquidGlass、快捷键、移动端手势、默认板型与余量
│   ├── workspace.json      # shell.WorkspacePersistState：分屏布局、打开的页签与焦点面板
│   ├── trash_meta.json     # 软删除元数据
│   └── trash/              # 回收站文件
├── assets/                 # ./assets/<file> 便携相对路径图片与附件（Ctrl+V / 拖拽自动导入）
├── templates/              # 无日期模板（ClearExecutionDataForTemplate 生成）
└── <ProjectFolder>/*.asterml
```

核心接口职责：
- **`vault.Vault`**：`ScanTree()` 扫描真实目录；`CreateExperiment` / `CreateFolder` / `Rename` / `Move` / `Trash` / `DuplicateFile` / `SaveAsTemplate` 操作磁盘并维护路径；`ImportAssetBytes` 支持截屏即时存盘。
- **`vault.DocStore` & `LiveDocument`**：按规范化 `RelPath` 单例缓存所有已打开文件；`ApplyEdit` 与 `ApplySourceDraft` 推入最大 **100 步 (`maxUndoStack`)** 撤销栈并通过 `asterdoc.WriteFileAtomic`（临时文件 `Sync` + `os.Rename`）落盘；`ReloadIfExternalChanged()` 检测外部（Agent / 其他编辑器 / 云同步）引发的 `DiskMtime` 变化并热刷新。

### 3.2 Native FFI Bridge（`modules/runs/runs.go`）

MyGo 经 `//mygo:native` 生成 4 个只读/事务服务接口连接 Rust `crates/asterlab-core`：
1. **`RunService`**：`LoadRun`、`ListRuns`、`SaveReadout` / `SaveReadoutsBatch`、`SaveTaskChecks`、`SaveAssetLink` / `UpdateAssetLinkCoordinates` / `UnlinkAssetLink`、`CompleteTask`、`CreateExecutionEvent`、`UpdateTimer`、`ExecuteCommand`（INT-01 通用写入口，要求 `IdempotencyKey`）、`QueryProjection`（只读查询投影）。
2. **`DesignService`**：`ListDesigns`、`LoadDesign`、`SaveDesign`、`CreateAsterDocDesign`、`SaveAsterDoc`、`ApplyAsterDocCommand`、`CompileAsterDoc`、`AssessAsterDoc`、`CreateRunFromAsterDoc`（将某修订的 AsterDoc 原子冻结为 Run）、`DeriveDesignFromRun`。
3. **`TimeBudgetService`**：无状态只读接口 `ProjectTimeBudget`（计算并行/串行步骤预算）。
4. **`MixPlanService`**：无状态只读接口 `PlateMixPlan`（按组孔数计算 GelMA 与细胞接种配平量）。

### 3.3 Agent CLI 契约（[AGENT_API.md](../AGENT_API.md)）

- 调用形如 `labworkbench.exe <cmd> --input <json|-> [--diagnostics json]`，输入闭环 JSON（<= 2 MiB），未知字段一律拒绝。
- 输出单行 JSON：`{"ok": true, "tool": "...", "result": ...}` 或 `{"ok": false, "tool": "...", "error": {"code": "...", "message": "..."}}`，退出码语义固定：`0` 成功、`2` 输入错误、`3` 不存在、`4` `REVISION_CONFLICT`、`5` 内部错误。
- 带 `--diagnostics json` 时追加固定结构诊断数组 `Diagnostic{path, code, severity, message, affects, next_action, source}`。

---

## 4. AsterML v2 SDK 与 21 模块契约体系

权威规范见 [docs/asterdoc-contract.md](./asterdoc-contract.md) 与 [prototypes/mygo-run/internal/asterdoc/spec/modules.json](../prototypes/mygo-run/internal/asterdoc/spec/modules.json)。

### 4.1 三条架构铁律（不可违背）

1. **禁止实验专用模块 / 专用字段 / 专用面板**：能由现有通用内容模块与第二层原语组合实现的需求，严禁新增例如 `cck8`、`qpcr`、`wb` 等实验专用 `kind`。
2. **`figure@1` 仅负责视觉补充表达**：琼脂平皿分区、流式门控、WB 泳道示意图存放在 `figure@1`（支持内嵌矢量 `<svg>` 或 `./assets/`）；所有分组、剂量、读数真值必须保存在结构化模块与 Run 事件中，绝不解析图上文字反推数据。
3. **步骤内自由组合与单真源**：`step@1.blocks[]` 支持任意顺序组合内容模块或 `block_refs` 引用；支持自包含编译冻结（无需预先登记外部 `ProtocolBlockVersion` UUID）。

### 4.2 全量 21 个注册模块清单（`sdkVersion: "2.0.0"`）

| 层级 | `kind@schema` | 分类 (`category`) | 核心能力与单真源内联执行语义 |
|---|---|---|---|
| **容器** | `plate@1` | `labware` | 标准 `4/6/12/24/48/96/384` 孔板、`rack_1d` 管架/泳道、`dish`、`cohort`；支持顶部图层切换（`[分组]` / 自定义 `layers` / `[+]`）、选中孔 `单值 \| 平铺列表 \| 等比序列` 填充、`excluded_wells` 与右键导出 CSV/PNG |
| **第一层** | `prose@1` | `document` | 纯展示富文本与提示卡（`tone: plain \| note \| tip \| warning`），`UnifiedLive` 下无外框 |
| **第一层** | `group_table@1` | `workflow` | 集中声明实验分组、对照角色（`control/treatment/comparator/reference`）、条件、`independent_n` 与重复数；移动端窄屏自动切换为无边框紧凑行卡片 |
| **第一层** | `param_table@1` | `measurement` | 默认标题「实验条件」；支持 `value/unit` 与内联实测偏离 `actual` 录入（`记偏离`），供下游 `@actual.<key>` 引用 |
| **第一层** | `figure@1` | `document` | 矢量 SVG（支持 `<g transform>` 坐标栈与中文宽度测算）或 `./assets/` 照片；支持剪贴板 `Ctrl+V` 贴图与灯箱缩放 |
| **第一层** | `data_table@1` | `measurement` | 自定义列（`text/number/categorical/formula`）、`min/max` 质控标黄、`source_ref`（`@groups` / `@plate.<id>`）与 2D 矩阵整板 TSV/CSV 粘贴 |
| **第一层** | `checklist@1` | `workflow` | 可在「编辑」与「阅读」双态点击勾选 `done` 并支持必检及 `skip_rule` 规则豁免 |
| **第一层** | `calculator@1` | `measurement` | 统一支持 `dilution`、`serial_dilution`、`group_mastermix`、`sample_normalize`、`formula` 与 `@plate.wells` / `@param.<key>` 联动求解 |
| **第一层** | `timer@1` | `workflow` | 支持 `countdown`、`stopwatch`、`cycle`、`window` 四态计时，`▶/⏸/↺` 实时更新内联 `status/elapsed_sec` |
| **第一层** | `attachment@1` | `document` | 挂载 `./assets/` 步骤证据、`design_ref` 及按孔位跟踪采图进度的 `container_grid` |
| **第一层** | `subflow@1` | `workflow` | 步骤内折叠/展开子操作清单，支持内联增删与逐条完成勾选 |
| **第二层** | `experiment@1` | `workflow` | 顶层实验声明（研究目的 `objective`、独立实验单位 `experimental_unit`、状态与结论） |
| **第二层** | `group@1` / `assay@1` / `workflow@1` / `step@1` | `workflow`/`measurement` | 声明单组、检测指标、流程依赖图与容器步骤树 |
| **第二层** | `instrument_profile@1` / `material@1` / `sample@1` / `reference@1` / `readout@1` | `resource`/`document`/`measurement` | 仪器能力档案、试剂材料（可绑储藏室 `pantry_code` 坐标跳转）、样本、文献引用与 `readout.viewer` 结构卡片 |

- **单位系统（[units.json](../prototypes/mygo-run/internal/asterdoc/spec/units.json)）**：严格校验 UCUM 子集（如 `Cel`、`uM`、`ng/uL`、`cells/mL`、`mW/cm2`、`xg`），禁止隐式未知单位换算。
- **双语言 Parity Gate**：`TestAsterDocGoRustParity` 与 `python scripts/sdk/check.py` 强制校验 Go（`internal/asterdoc`）与 Rust（`src/cli/asterdoc.rs`）在 21 个模块、20 条校验规则与 21 种端口类型上的完全一致。

### 4.3 第一层内容模块、容器原语与 A4 PDF 导出实图图鉴

> 以下图片均由 `prototypes/mygo-run` 真实渲染生成，完整文字/代码对照详见 [v2-module-visual-catalog.md](./sdk/v2-module-visual-catalog.md)。

#### 4.3.1 第一层通用内容模块（Layer 1 Content Blocks）

| `prose@1` 富文本与提示卡 | `group_table@1` 实验分组 |
|---|---|
| ![prose@1 富文本与提示卡](./sdk/evidence/v2-visual-catalog/comp_01_prose.png) | ![group_table@1 实验分组](./sdk/evidence/v2-visual-catalog/comp_02_group_table.png) |

| `param_table@1` 实验条件与偏离录入 | `figure@1` SVG 矢量与实拍图 |
|---|---|
| ![param_table@1 实验条件与偏离录入](./sdk/evidence/v2-visual-catalog/comp_03_param_table.png) | ![figure@1 SVG 矢量与实拍图](./sdk/evidence/v2-visual-catalog/comp_04_figure_v2.png) |

| `data_table@1` 行记录与二维孔板矩阵 | `checklist@1` 双态检查与规则豁免 |
|---|---|
| ![data_table@1 行记录与二维孔板矩阵](./sdk/evidence/v2-visual-catalog/comp_05_data_table.png) | ![checklist@1 双态检查与规则豁免](./sdk/evidence/v2-visual-catalog/comp_06_checklist.png) |

| `calculator@1` 稀释与梯度稀释 | `calculator@1` 配平 / 归一化 / 公式 |
|---|---|
| ![calculator@1 稀释与梯度稀释](./sdk/evidence/v2-visual-catalog/comp_07_calculator_dilution_serial.png) | ![calculator@1 配平/归一化/公式](./sdk/evidence/v2-visual-catalog/comp_08_calculator_mastermix_norm_formula.png) |

| `timer@1` 四态计时器 | `attachment@1` 附件与按孔位采图网 | `subflow@1` 折叠子流程 |
|---|---|---|
| ![timer@1 四态计时器](./sdk/evidence/v2-visual-catalog/comp_09_timer_all_modes.png) | ![attachment@1 附件与按孔位采图网](./sdk/evidence/v2-visual-catalog/comp_10_attachment.png) | ![subflow@1 折叠子流程](./sdk/evidence/v2-visual-catalog/comp_11_subflow.png) |

#### 4.3.2 容器孔板 (`plate@1`) 与第二层科学原语卡片

| 6 / 12 / 24 孔板、管架与泳道几何体 | 96 孔板内联态（含多层图与序列条） |
|---|---|
| ![plate@1 多孔板与管架几何体](./sdk/evidence/v2-visual-catalog/comp_12_plate_geometries_v3.png) | ![plate@1 96 孔板与序列填充条](./sdk/evidence/v2-visual-catalog/plate_layers_serial_bar_light.png) |

| 96 孔板单卡视图 | 孔板灯箱展开浮条与图层切换 | 第二层科学原语与 `readout.viewer` |
|---|---|---|
| ![plate@1 96 孔板单卡视图](./sdk/evidence/v2-visual-catalog/comp_13_plate_96_v3.png) | ![plate@1 灯箱展开浮条](./sdk/evidence/v2-visual-catalog/comp_13b_plate_overlay_bar_v3.png) | ![第二层 10 类科学原语总览](./sdk/evidence/v2-visual-catalog/comp_14_top_level_doc_cards.png) |

#### 4.3.3 A4 纸质执行单与实验记录 PDF 导出实图

| RT-qPCR 384 实验台执行单 | 抑菌圈测定方案执行单 |
|---|---|
| ![RT-qPCR A4 PDF 导出](./sdk/evidence/v2-visual-catalog/pdf_export_rt_qpcr_real_v2.png) | ![抑菌圈方案 A4 PDF 导出](./sdk/evidence/v2-visual-catalog/pdf_export_inhibition_zone_real_v2.png) |

| HUVEC 血管生成实测方案单 | 抑菌圈 Run 实测记录归档 |
|---|---|
| ![HUVEC A4 PDF 导出](./sdk/evidence/v2-visual-catalog/pdf_export_huvec_real_v2.png) | ![抑菌圈 Run 记录 PDF 导出](./sdk/evidence/v2-visual-catalog/pdf_export_iz_run_record_real_v2.png) |

---

## 5. UI 视觉设计系统、多主题引擎与 Liquid Glass 物理规范

所有视觉代码必须遵守 [docs/product/design-system.md](./product/design-system.md)（`UX-P3` / `UX-DESIGN`），参数唯一来源于 [internal/theme/theme.go](../prototypes/mygo-run/internal/theme/theme.go) 与 [motion.go](../prototypes/mygo-run/internal/theme/motion.go)。

### 5.1 单一 Token 源与三套色盘预设（2026-11-20 升级）

告别早期单调偏黄的 `#f7f6f3` 底色，当前主题系统由 `TokensFor(preset, dark, liquidGlass)` 统一驱动，用户可在 **设置 → 外观** 与 **`Ctrl+K` 命令面板** 中即时切换：

| 预设 ID | 浅色画布 / 卡片 / 选中胶囊 / 主色 | 深色画布 / 浮层 / 主色 | 适用定位 |
|---|---|---|---|
| **`obsidian`（默认推荐）** | `Canvas #ffffff` · `Elevated #f5f6f8`（无框卡片） · `SelectedRow #ededf0` · `Accent #6c5ce7`（软标签 `#eeebff`） | `Canvas #111215` · `Surface #16181d` · `Elevated #1e2027` · `Accent #a596ff`（软标签 `#2a2645`） | 与 Obsidian 原生一致的极致纯净白纸与深空暗色，零色偏 |
| **`zinc`（冷银极简）** | `Canvas #f4f5f7` · `Surface #ffffff` · `Elevated #eceef2` · `Accent #635bff` | `Canvas #0f1115` · `Surface #16181e` · `Accent #a397ff` | 中性冷银底衬纯白卡片，边界分明 |
| **`warm`（经典暖纸）** | `Canvas #f7f6f3` · `Surface #ffffff` · `Accent #5b45c9` | `Canvas #171614` · `Surface #1e1d1a` · `Accent #a996ff` | 保留早期实验室暖白/暖炭色盘备份 |

- **科学语义色例外**：孔板分组（8 色 `theme.CategoricalPalette`）与孔位对照角色色（`theme.WellRoleColor`）只能调用这两个具名函数，严禁在业务视图手写 `ui.Hex(...)`。
- **空间与圆角体系**：仅用 `4 / 8 / 12 / 16 / 24 / 32 / 48`（或 `t.Space(n)`），三档圆角 `ControlRadius(6)` / `PanelRadius(12)` / `DialogRadius(14)`（浮动全胶囊与圆形按钮使用 `height/2`）。

### 5.2 Light vs Dark Liquid Glass 渲染铁律（`theme.go`）

经过 `mygo/plugins/glass` 着色器曲线验证：
1. **浅色悬浮胶囊 (`ApplyCapsuleGlass`)**：采用 `glass.Glass{Style: glass.Regular, Tint: tok.Surface.Alpha(0.76), Interactive: interactive}` 配合 `tok.Surface.Alpha(0.95)` 亮边环与 `Shadow(0, 8, 28)` 投影——在让底层色彩柔雾透出的同时确保前景黑字/黑图标高对比可读。
2. **暗黑模式禁用 `glass.Regular` 无遮罩透射与 Glass-on-Glass 叠加**：
   - `glass.Regular` 在 Dark 模式下自带 `paneDarkLow = 0.15`、`paneDarkHigh = 0.51` 与 `saturation = 2.2`，会将暗背景抬灰、亮孔位涂抹成杂斑；
   - 因此 Dark 模式胶囊必须采用 **`glass.Clear` + 84% 深烟熏曜石 `rgba(24,26,33,0.84)` + 亮银内高光边框 `rgba(255,255,255,0.14)`**；
   - **禁止多层玻璃叠加**：桌面端左右固定侧栏与内容区采用纯实底（`tok.RailBackground` / `tok.Canvas`），Liquid Glass 仅用于顶栏和移动端悬浮胶囊、抽屉、命令面板。
3. **Anti-XP-Box（去重框哲学）**：正文与属性编辑禁止白底圆角输入框套娃，统一使用 `asterdoc.BorderlessTextInput`（`ui.TextInputBase` 静态透明，仅 Hover/Focus 柔和底色）与 `StyleSelectQuiet`。

#### 5.2.1 多主题色盘与深浅色 Liquid Glass 实拍对比

| Obsidian Light（纯白默认 + 浅色 Liquid Glass 顶栏） | Obsidian Dark（深空曜石 + `glass.Clear` 烟熏胶囊，零晕染） |
|---|---|
| ![Obsidian Light 桌面端](./sdk/evidence/v2-visual-catalog/liquid_glass_desktop_obsidian_light.png) | ![Obsidian Dark 桌面端日历](./sdk/evidence/v2-visual-catalog/liquid_glass_desktop_obsidian_dark.png) |

| Obsidian Dark 实验页（强对比孔板与文档正交视区） | Cool Zinc Light（冷银底 `#f4f5f7` 衬纯白卡片） | Cool Zinc Light 移动端 |
|---|---|---|
| ![Obsidian Dark 实验页](./sdk/evidence/v2-visual-catalog/liquid_glass_desktop_exp_dark.png) | ![Cool Zinc Light 桌面端](./sdk/evidence/v2-visual-catalog/liquid_glass_desktop_zinc_light.png) | ![Cool Zinc Light 移动端](./sdk/evidence/v2-visual-catalog/liquid_glass_mobile_zinc_light.png) |

### 5.3 四档 Oklab 物理动效预算（`internal/theme/motion.go`）

| Token | 曲线参数 | 用途 |
|---|---|---|
| `MotionMicro` | `100ms EaseOut` + `Oklab` | 按钮/孔位按压微缩 `0.97`（`ApplyTactilePress`）、悬浮手柄渐显 |
| `MotionSnappy` | 临界阻尼 `160ms, Bounce: 0` | 磁吸胶囊滑块（`SharedPillTransition`）、孔板 Oklab 染色、选孔底条托出（`FloatingBarTransition`） |
| `MotionFluid` | 柔和弹簧 `220ms, Bounce: 0.04` | 手风琴折叠（`AccordionTransition`）、四向分屏液态预览（`LiquidSplitTransition`）、`NumericRoll` 数字滚轮与移动端底栏收放 |
| `MotionHero` | 空间弹簧 `260ms, Bounce: 0.03` | `HeroFlyTransition` 图表/孔板灯箱一镜到底放大与日历网格滑动 |
| `ApplyNudge` | 欠阻尼脉冲 `±5px` | 空提交或非法参数轻晃提醒（替代刺眼弹窗） |

---

## 6. 桌面端与 Obsidian iOS 26 移动端 UX 交互规范

### 6.1 桌面端 3 主表面 + 平权分屏（[ux-contract.md](./product/ux-contract.md)）

- **左栏（40px Ribbon + Vault 文件树）**：按显式 `depth` 绘制缩进引导线，支持 `F2` 内联重命名、拖拽文件到日历改期或拖到编辑区四向边缘（25% 热区）分屏，以及上下文菜单优先原则（`ContextMenu`）。
- **主区（`PaneNode` 平权分屏树）**：任意叶窗格平权托管 `实验日程`（月/周正交视图）、`储藏室货架`（BioRender 容器/冻存盒/试剂详单）或 `.asterml` 文档（`编辑 | 阅读 | 源码`），多窗格开同一文件共享同一 `*vault.LiveDocument`。
- **右伴侣抽屉（默认折叠）**：仅收拢「步骤大纲」与「速算与计时」。
- **全局检索与配置**：`Ctrl+K/O` 命令面板（实验 / 储藏室试剂 / 快捷命令 & 主题实时切换）；`Ctrl+,` 可搜索 6 类设置中心。

#### 6.1.1 工作台分屏与 UnifiedLive 三态视图

| 桌面分屏 + 右伴侣抽屉（Light） | 桌面分屏 + 右伴侣抽屉（Dark） |
|---|---|
| ![桌面分屏 Light](./sdk/evidence/v2-visual-catalog/workbench_obsidian_split_drawer_light.png) | ![桌面分屏 Dark](./sdk/evidence/v2-visual-catalog/workbench_obsidian_split_drawer_dark.png) |

| UnifiedLive「编辑」态 | UnifiedLive「阅读」态 | UnifiedLive「源码」态 |
|---|---|---|
| ![UnifiedLive 编辑态](./sdk/evidence/v2-visual-catalog/unified_live_edit_light.png) | ![UnifiedLive 阅读态](./sdk/evidence/v2-visual-catalog/unified_live_read_light.png) | ![UnifiedLive 源码态](./sdk/evidence/v2-visual-catalog/unified_live_source_light.png) |

| 单真源方案灯箱放大 (`figure@1`) | 实验记录 Notebook ELN 双栏对照 |
|---|---|
| ![方案灯箱放大](./sdk/evidence/v2-visual-catalog/design_iz_figure_zoom_light.png) | ![Notebook ELN 双栏对照](./sdk/evidence/v2-visual-catalog/notebook_eln_split_light.png) |

#### 6.1.2 实验日程 Home、弹窗、`Ctrl+K` 命令面板与设置中心

| 实验日程月视图（Light） | 实验日程月视图（Dark） | 实验日程周时间网格（Light） |
|---|---|---|
| ![日程月视图 Light](./sdk/evidence/v2-visual-catalog/home_calendar_month_light.png) | ![日程月视图 Dark](./sdk/evidence/v2-visual-catalog/home_calendar_month_dark.png) | ![日程周视图 Light](./sdk/evidence/v2-visual-catalog/home_calendar_week_light.png) |

| 新建实验（从模板生成） | `Ctrl+K` 全局命令面板 | 设置中心 — 外观与主题开关 |
|---|---|---|
| ![新建实验对话框](./sdk/evidence/v2-visual-catalog/vault_new_exp_dialog_light.png) | ![Ctrl+K 命令面板](./sdk/evidence/v2-visual-catalog/workbench_command_palette_light.png) | ![设置中心外观](./sdk/evidence/v2-visual-catalog/vault_settings_appearance_light.png) |

#### 6.1.3 储藏室货架、NIH BioArt 实验贴纸、冻存盒与工作台独立工具

| 储藏室货架主页（Standalone） | BioRender 级实验贴纸全表 | 货架暗黑模式视角 |
|---|---|---|
| ![储藏室货架](./sdk/evidence/v2-visual-catalog/tools_pantry_standalone_light.png) | ![BioRender 实验贴纸表](./sdk/evidence/v2-visual-catalog/comp_16_labsticker_sheet_v3.png) | ![储藏室暗黑模式](./sdk/evidence/v2-visual-catalog/comp_16c_labsticker_shelf_dark_v3.png) |

| 9×9 冻存盒网格（Light） | 9×9 冻存盒网格（Dark） | 96 孔 PCR 管盒（Light） |
|---|---|---|
| ![9x9 冻存盒 Light](./sdk/evidence/v2-visual-catalog/tools_pantry_box_cryo_9x9_light.png) | ![9x9 冻存盒 Dark](./sdk/evidence/v2-visual-catalog/tools_pantry_box_cryo_9x9_dark.png) | ![96 孔 PCR 管盒](./sdk/evidence/v2-visual-catalog/tools_pantry_box_pcr_96_light.png) |

| 15/50mL 离心管架 | 试剂详单与消耗追溯 | 独立孔板 Studio | 独立实验速算台 |
|---|---|---|---|
| ![离心管架](./sdk/evidence/v2-visual-catalog/tools_pantry_box_falcon_rack_light.png) | ![试剂详单](./sdk/evidence/v2-visual-catalog/comp_16d_pantry_reagent_detail_v3.png) | ![孔板 Studio](./sdk/evidence/v2-visual-catalog/tools_plate_studio_v3.png) | ![独立速算台](./sdk/evidence/v2-visual-catalog/tools_bench_calc_v1.png) |

#### 6.1.4 五类原生上下文右键菜单 (`ContextMenu`)

| 文件树右键 | 窗格标签页右键 | 孔板选区右键 | 步骤卡片右键 | 日程事件右键 |
|---|---|---|---|---|
| ![文件树右键](./sdk/evidence/v2-visual-catalog/context_menu_file_tree_light.png) | ![页签右键](./sdk/evidence/v2-visual-catalog/context_menu_pane_tab_light.png) | ![孔板右键](./sdk/evidence/v2-visual-catalog/context_menu_plate_light.png) | ![步骤卡片右键](./sdk/evidence/v2-visual-catalog/context_menu_step_block_light.png) | ![日程事件右键](./sdk/evidence/v2-visual-catalog/context_menu_calendar_light.png) |

### 6.2 Obsidian iOS 26 移动端自适应壳（`< 680dp` / Android / iOS，[mobile.go](../prototypes/mygo-run/internal/shell/mobile.go)）

- **顶栏释放视觉中心**：顶部用 `ScrollEdgeSoft` 柔和羽化滚动内容，左上固定 `40×40` 圆形独立玻璃抽屉钮，右上仅留双图标合并玻璃药丸（文档页为 `[编辑/阅读切态 | ⋯]`, 日程页为 `[速算 | 搜索]`），文档标题优雅落在正文首行。
- **无框软灰卡片体系**：文档属性收拢入无 1px 外框的 `Tok.Elevated`（`#f5f6f8` / `#1e2027`）`R14`「实验属性」卡片；`group_table` 窄屏自动转成无截断行卡片；日历首日紧凑显示 `11/1` / `12/1`。
- **居中纯图标底浮岛（拒绝堆文字）**：
  - **导航态**：居中 `48dp` 高 `Radius(24)` 玻璃胶囊仅放 6 枚纯细线图标 **`<  >  🔍  +  [N]  ≡`**；向下滚动页面超过 36dp 自动滑隐，轻滑上拉或点底部提示条弹回。
  - **编辑态**：自动切换为同尺度居中纯图标编辑胶囊（`↶ ↷ | +步骤 ☑ ⊞ ⬡ ⏱ | ⊞` 切回导航）。
- **手势抽屉与三层底部控制岛**：屏幕左右 18dp 边缘热区支持 `Dragged()` + `DragVelocity()` 1:1 跟手滑出文件库或步骤大纲；抽屉底部复刻 Obsidian 3 层控制岛（5 枚纯图标动作 → `[📁 实验列表 ↕]` 模式切换胶囊 → `AsterLab / 实验统计` + 右下独立 `40dp` 圆形玻璃 `⚙` 设置按钮）。
- **标签页概览与下拉快捷动作**：点击 `[N]` 弹起双列卡片网格概览并支持单卡关闭 `×`；页面顶部 `Y <= 2` 继续阻尼下拉触发可自定义的 Quick Action（默认唤出命令面板）。

#### 6.2.1 移动端全流程实图（阅读 / 编辑 / 抽屉 / 日程 / 多页签概览）

| 移动端阅读态（Obsidian Light） | 移动端阅读态（Obsidian Dark） | 移动端编辑态底条（Light） | 移动端编辑态底条（Dark） |
|---|---|---|---|
| ![移动端阅读 Light](./sdk/evidence/v2-visual-catalog/mobile_experiment_read_obsidian_light.png) | ![移动端阅读 Dark](./sdk/evidence/v2-visual-catalog/mobile_experiment_read_obsidian_dark.png) | ![移动端编辑 Light](./sdk/evidence/v2-visual-catalog/mobile_experiment_edit_light.png) | ![移动端编辑 Dark](./sdk/evidence/v2-visual-catalog/mobile_experiment_edit_obsidian_dark.png) |

| 侧滑文件树抽屉 + 3 层底岛 | 移动端实验日程月历与当日卡片 | `[N]` 双列标签页卡片概览 |
|---|---|---|
| ![移动端抽屉](./sdk/evidence/v2-visual-catalog/mobile_drawer_tree_light.png) | ![移动端日程](./sdk/evidence/v2-visual-catalog/mobile_workbench_calendar_light.png) | ![移动端页签概览](./sdk/evidence/v2-visual-catalog/mobile_tab_switcher_light.png) |

---

## 7. 移动端运行边界与双端同步设计决策

> 来源：2026-11-20 架构评估（已同步收录于 [docs/architecture.md §10](./architecture.md#10-移动端运行边界与双端同步决策记录2026-11-20-架构讨论)）。

1. **后端边界（纯 Go Vault 模式无需外部服务端）**：
   - `.asterml` 解析校验、`Vault` 目录扫描、`LiveDocument` 100 步撤销与原子写盘、日历与储藏室目前 **100% 运行在纯 Go 层**；
   - 唯一强绑定在桌面端启动流程的是 `main.go:4584` 对 Rust FFI `runs.NewRunService()` / `runs.NewDesignService()`（`runs.dll`）的初始化调用；
   - 面向 Android/iOS 真机运行有两种路径：**（A）纯 Go Vault 降级模式**（缺原生库时静默降级 `syncErr`，直跑本地沙盒 Vault，零后端改造成本）；**（B）完整 Core 审计模式**（交叉编译 `crates/asterlab-core` 为 `libruns.so` / `.a`）。
2. **双端同步推荐路线（文件级透明同步 + 结构化防冲突）**：
   - **起步零服务期**：利用单库 `.asterml` + `./assets/` 天然兼容 **iCloud Drive / Syncthing / WebDAV / OneDrive**；客户端补齐三点即可：(a) 前后台切换触发 `LiveDocument.ReloadIfExternalChanged()` 热重载；(b) 冲突发生时按 `.asterml` 的 `step.id` 与块 `id` 做三路合并，仅在同块同字段撞车时保存 `*.conflict-<device>.asterml` 提示对比，严禁静默覆盖；(c) `assets/` 大图采用按需/缩略图分层同步。
   - **进阶可控自托管**：保留本地 `.asterml` 离线读写引擎不变，在外围挂载极薄的按 `sha256` 分块内容寻址 Blob 同步服务（S3/OSS/WebDAV + 元清单），共用同一套按块合并引擎。

---

## 8. 基于文档发现问题与持续优化的工作方法（Verification & Hotspot Audit）

本手册不只记录“做完了什么”，还提供一套可执行的**巡检与优化工作流**：任何新开发或重构前，按下列步骤从文档和工具门禁中定位真实差距。

### 8.1 必跑自动化校验门禁

```powershell
# 1. SDK 双端契约校验（21 模块 / 21 规则 / Go-Rust Parity / units）
python scripts/sdk/check.py

# 2. 全量 Go 单元测试 + 构建 + 视觉目录截图刷新
$env:ASTERLAB_VISUAL_DIR="D:\L\AI\Cindy\CellPlanner-ProductStudy\cellplanner-replica\docs\sdk\evidence\v2-visual-catalog"
cd prototypes/mygo-run
go test -p=2 -count=1 ./internal/...
go test -p=2 -count=1 .
go build
```

```powershell
# 3. Design System P3 合规扫描（检查调用点硬编码字面值基线）
rg -n "ui\.Hex\(" prototypes/mygo-run --include=*.go | rg -v "internal/theme/"
rg -n "FontSize\([0-9]" prototypes/mygo-run --include=*.go
```

### 8.2 当前已观测的已知差距与下一阶段优化热点（Actionable Backlog Seeds）

*(按 `AGENTS.md` 规范，具体立项先在 Kandev 建卡，此处只做事实级代码现状索引：)*

| 类别 | 观测位置 / 证据 | 现状与可优化方向 |
|---|---|---|
| **移动端真机启动降级** | [main.go#L4584-L4613](../prototypes/mygo-run/main.go#L4584-L4613) | 缺 `Runs.dll` / `libRuns.so` 时会给 `a.syncErr` 赋值并显示错误条；后续应区分「纯 Go Vault 独立运行态」与「Core 服务连接异常态」，避免移动端无原生库时误报红条。 |
| **双端同步冲突与外改感知** | [live_doc.go#L195-L220](../prototypes/mygo-run/internal/vault/live_doc.go#L195-L220) | 已有基础 `ReloadIfExternalChanged()`，但尚缺周期/窗口聚焦事件自动轮询触发、基于 `step.id`/`block.id` 的结构化三路合并及 `*.conflict-*.asterml` 冲突副本 UI 提示。 |
| **Dark 模态弹窗背景透射** | [vault_settings_hotkeys_dark.png](./sdk/evidence/v2-visual-catalog/vault_settings_hotkeys_dark.png) · `onevra/components/dialog.go` | `components.Dialog` 使用上游默认 `.18` 背景暗角 + `.96` `glass.Regular`，在暗色主题下仍会让底层亮文字轻微透出重影；可在 `settings.go` 对话框层外垫深色 scrim 或向上游反馈 dark dialog 策略。 |
| **历史 UI 包 token 债务清理** | [docs/product/design-system.md §6](./product/design-system.md) | `UnifiedLive` 新路径与 `shell`/`theme` 已收口 token，但遗留设计/运行包（`internal/runui`、`internal/runworkspace`、`internal/resultgrid` 等）仍登记有历史字面值 `FontSize`/`Padding`/`Radius` 基线待分卡清理。 |
| **系统软键盘避让与真机手势** | [mobile.go](../prototypes/mygo-run/internal/shell/mobile.go) · `mygo@v0.8.4/ui` | 当前通过 `ModeEdit` 切换底栏胶囊模拟“键盘上方编辑工具条”；待 `mygo/ui` 对 Android/iOS 暴露虚拟键盘 Inset 与原生 `OnLongPress` 后，可将底部胶囊精准抬升至软键盘上沿，并为文件树/孔位补齐真长按触控菜单。 |
| **移动端周视图与多选操作** | [calendar.go](../prototypes/mygo-run/internal/shell/calendar.go) · [file_tree.go](../prototypes/mygo-run/internal/shell/file_tree.go) | 月历网格已适配移动端点状显示与 `11/1` 紧凑徽标，但周视图 `renderAppleWeekTimeGrid` 在 390px 窄屏下的横向排布压缩与左树长按批量移入项目尚有精简空间。 |

#### 8.2.1 Dark 模态弹窗透射问题实图证据

![Dark 快捷键设置弹窗底色透射证据](./sdk/evidence/v2-visual-catalog/vault_settings_hotkeys_dark.png)
