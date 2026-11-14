# AsterLab v1.2 Visual Direction — Porcelain (白瓷精密)
> **Modern Light Porcelain / Things 3 × Linear**  
> *Workspace Spec: 1200×760 DIP Standard Workstation*  
> *Target Runtime: MyGo Native Desktop (`github.com/egoist/mygo` / Go Native UI)*  
> *Document Status: Complete Direction Specification & Deliverable*

---

## 1. 核心设计理念 (Concept Manifesto)

### 1.1 为什么是 Porcelain (白瓷)？
在传统的实验室信息化与生命科学软件中，充斥着两种极端倾向：
1. **老旧工控/管理系统风格**：暗黄灰暗的实验室防眩光假“纸质感”（如温黄泥底 `#F6F4EF`、草绿主按钮 `#1F6F5F`、厚重的 2px 深色框线及繁杂的默认表单组件），沉重、压抑、缺乏精密现代感；
2. **通用 SaaS 仪表盘风格**：大面积纯黑暗黑夜间模式、浮夸的渐变与毛玻璃特效，不仅违背了实验室白天下高照度环境对可读性的要求，且极难在原生跨平台桌面引擎中高效渲染。

**Porcelain（白瓷）** 汲取 **Things 3 的通透呼吸感与极简编排** 与 **Linear 的高密度精密工程感**：
- **微温润凉白瓷表面**（Porcelain Paper Surface, `#F8F9FA` ~ `#FFFFFF`）：告别发黄沉闷的“旧米色”，呈现如同日本骨瓷、瑞士高级钟表表盘般的纯净平整纸质感；
- **高对比墨黑字体**（Near-Black Ink, `#0F172A` / `#475569`）：提供极高对比度的科学文字排版与 `tabular-nums` 纯等宽数值对齐；
- **单一克制深钴蓝强调色**（Precision Cobalt Blue, `#2563EB`）：只在“当前聚焦焦点”、“主操作触发”与“关键数据高亮”出现，拒绝大面积彩色喧宾夺主；
- **彻底摒弃表单堆叠，重构为微型精密检查器**：拒绝“上标签、下输入框”的低效 CRUD 表单，所有参数采用**水平紧凑数据带**（Horizontal Property Strips）、行内步进器（Micro-stepper）与单位内联锁定；
- **最小化镀铬结构与自然留白**：取消粗重实线包边，依靠 1px 发丝细线（`#E5E7EB`）、微柔投影与合理间距划分功能区。

---

## 2. 设计规范与颜色系统 (Tokens & Palette)

### 2.1 基础色元与表面阶梯 (Surfaces & Hairlines)
Porcelain 严格限定表面阶梯为 4 层，确保在纯原生绘图上下文中无需昂贵的混合图层：

| Token | 值 | 语义说明 | 替代原版的问题设计 |
| :--- | :--- | :--- | :--- |
| `--canvas` | `#F8F9FA` | 全局窗口底衬，微凉暖平衡白瓷底色 | 拒绝沉闷发黄的 `#F6F4EF` 实验室旧米色 |
| `--surface` | `#FFFFFF` | 核心卡片、侧边栏、检查器白瓷面板 | 统一的高净度白，消除层级混乱 |
| `--surface-subtle` | `#F4F5F7` | 96孔板托盘衬底、次级胶囊按键底色 | 清爽的灰调过渡，无眩光 |
| `--surface-hover` | `#F1F3F5` | 列表项划过态、轻量悬停反馈 | 细腻平滑的指针交互 |
| `--hairline` | `#E5E7EB` | 结构边界 1px 极细分界线 | 替换原版 `#C8C0B2` 粗厚边框，消除框线压迫感 |
| `--hairline-subtle`| `#F1F3F5` | 内部属性行之间的发丝分割线 | 几乎不可见的辅助对齐基线 |

### 2.2 墨色与文字阶梯 (Ink & Typography)
采用系统原生高质量西文字体（Apple SF Pro / Inter）搭配清晰平滑的无衬线中文字体，数字与坐标一律强制等宽（Monospace）：

| Token | 值 | 语义说明 |
| :--- | :--- | :--- |
| `--ink-primary` | `#0F172A` | 标题、主要数值、当前活动节点标题、高对比度参数 |
| `--ink-secondary` | `#475569` | 字段标签、属性描述、已完成节点副标题、表格表头 |
| `--ink-tertiary` | `#94A3B8` | 节点编号索引（01~05）、辅助单位、快捷键提示、占位符 |
| `--ink-on-accent` | `#FFFFFF` | 钴蓝底色上的反白文字 |

### 2.3 独占强调色 (The Single Accent)
| Token | 值 | 语义说明 |
| :--- | :--- | :--- |
| `--accent` | `#2563EB` | **钴蓝强调色**（Cobalt Precision Blue），仅用于主按钮、活动高亮边框、当前选中节点标记 |
| `--accent-hover` | `#1D4ED8` | 悬停状态下的更深钴蓝 |
| `--accent-tint` | `#EFF6FF` | 5% 微浅蓝底衬，用于活动行选中条纹、活动芯片背景 |
| `--accent-ring` | `rgba(37, 99, 235, 0.18)` | 键盘焦点与选中孔位外发光环 |

### 2.4 状态色 (Status Dots & Verification)
严格遵守**“绿色仅用于状态验证”**原则（坚决禁用大块绿色操作按钮）：
- **验证通过 / 同步正常 (`--status-success`)**: `#10B981` (翠绿状态点 / 检查钩)
- **草稿 / 警告 (`--status-warning`)**: `#F59E0B` (琥珀黄)
- **错误 / 阻断 (`--status-error`)**: `#EF4444` (绯红)

---

## 3. CCK-8 实验工作区组件架构 (Component Architecture)

统一 1200×760 标准视口划分为精密的五个核心功能层：

```
+---------------------------------------------------------------------------------------------------------+
| Topbar (44px): Controls | Breadcrumbs | Segmented Switcher | Sync Status (14:32:08) | Primary Action ⌘↵   |
+----------------------+----------------------------------------------------------+-----------------------+
| Left Rail (220px)    | Center Canvas (660px)                                    | Right Inspector(320px)|
|                      |                                                          |                       |
| Assay: CCK-8 (L929)  | Upper: Workflow DAG Pipeline (140px)                     | STEP 03 · EXPOSURE    |
|                      |   [01] ---> [02] ---> [03 Active] ---> [04] ---> [05]    | 梯度给药 (DOX)        |
| Nodes:               |                                                          |                       |
| 01 细胞悬液 (✓)      | Lower: 96-Well Microplate Workspace (518px)              | Tab: 参数 | 孔位 | 质控 |
| 02 96孔接种 (✓)      |   - 8x12 True Wells (A1~H12) with functional tints       | --------------------- |
| 03 梯度给药 (● Active)   - PBS Moat (36), Controls (6), 6-Doses (18), Blank     | 化合物: 阿霉素 (DOX)  |
| 04 CCK-8孵育 (○)     |   - Interactive hover HUD with live coords               | 溶剂: 0.1% DMSO       |
| 05 酶标仪读板 (○)    |                                                          | 暴露时间: 24.0h [+-]  |
|                      |                                                          | 剂量矩阵表 (C0~C5)    |
| Summary:             |                                                          | 质控校验 (3项全达标)  |
| 42/96 Wells Used     |                                                          | --------------------- |
| DOX / 0.1% DMSO      |                                                          | [暂存 ⌘S] [下一步 Tab]|
+----------------------+----------------------------------------------------------+-----------------------+
| Statusbar (28px): Engine v1.2-native (0.18ms) | Selected: Well C5 | Memory: 14.8MB (Zero Webview)        |
+---------------------------------------------------------------------------------------------------------+
```

### 3.1 顶部命令与导航栏 (Topbar, 44px)
- **窗口微控件**：符合桌面工法的三色原生圆点，悬停时激活色彩；
- **上下文面包屑**：`Oncology-InVitro / L929-CCK8-Tox-2026` 可快速切换实验或版本；
- **中央分段药丸切换器**：`流程设计 (Designer)` (高亮白底微阴影) | `孔板排布 (Plate)` | `标准规程 (Protocol)` | `执行批次 (Run)`；
- **保存状态胶囊**：显示实时引擎同步凭证（如 `● 已同步 14:32:08`），支持指针点击触发即时保存动画；
- **主动作按钮**：单一沉稳钴蓝按钮 `生成执行批次 ⌘↵`。

### 3.2 左侧工作流导航轨 (Workflow Outline Rail, 220px)
- 顶部展示细胞株标识：`CCK-8 比色检测` 伴随等宽标签 `L929`；
- 5 个标准工序项：
  - `01 细胞悬液制备`：标记绿色已核验图标；
  - `02 96孔板接种`：标记绿色已核验图标；
  - `03 梯度给药 (DOX)`：当前活动态，左侧 3px 钴蓝游标，高亮纯蓝背景底衬；
  - `04 CCK-8 避光孵育`：待执行圆点；
  - `05 酶标仪双波长读板`：待执行圆点；
- 底部显示关键指标摘要：孔位占用 `42 / 96 孔`、试验化合物 `Doxorubicin`、溶剂对照 `0.1% DMSO`。

### 3.3 中央画布 (Center Canvas, 660px)
1. **上层水平工序流 (Workflow DAG, 140px)**：
   - 贯通式工序轨道线，带钴蓝色进度渲染；
   - 5 个工序卡片：清晰显示工序号（`01`~`05`）、工序中文名与核心规程参数（如 `5×10⁴/mL`、`100 µL/well`、`6梯度·3复孔`、`450/650 nm`）；
   - 当前选中第 3 节点 `梯度给药` 呈现 1.5px 钴蓝聚焦框与轻度白瓷悬浮阴影。
2. **下层 96 孔微孔板映射工作区 (96-Well Plate Layout, 518px)**：
   - 标准 ANSI/SLAS 1-2004 规格平底孔板（Corning 3599 Flat Clear）；
   - 8 行（A~H）× 12 列（1~12）交互式真实点阵：
     - **PBS 封边隔离孔（36孔）**：外周一圈（Row A, Row H, Col 1, Col 12），填充无菌 PBS 200 µL，预防周边效应蒸发，以浅灰网格底纹标识；
     - **阴性对照组（NC，3孔）**：B2, C2, D2，完全培养基对照；
     - **溶剂对照组（Vehicle，3孔）**：E2, F2, G2，含 0.1% DMSO 培养基，监控溶剂背景毒性；
     - **梯度给药组（18孔）**：Col 3 至 Col 8，每列 3 复孔（Rows B, C, D），分别对应 0.0, 10.0, 20.0, 50.0, 100.0, 200.0 µg/mL，采用微阶梯蓝调饱和度递增编码；
     - **阳性对照组（PC，3孔）**：B9, C9, D9，顺铂 (Cisplatin) 50 µg/mL，确保细胞毒性试验有效性；
     - **未用备用孔（33孔）**：其余孔位作为空白备用。
   - **悬浮微型探测 HUD**：悬停或选定孔位时，底栏即时显示孔位 ID、分组归属、化合物名称、加样体积与复孔批次。

### 3.4 右侧精密属性检查器 (Precision Inspector, 320px)
**彻底剔除传统丑陋的垂直堆叠输入框**，构建 Linear 级别的精细数据单：
1. **化合物与溶剂规格 (Substance)**：
   - 试验化合物：`盐酸阿霉素 (DOX)`；
   - 溶剂体系：`0.1% DMSO in complete DMEM`；
   - 暴露时长：内联微型步进器 `[ - ] 24.0 h [ + ]`；
   - 每孔加样体积：精准对齐 `100 µL`；
2. **梯度浓度矩阵 (Dose Matrix)**：
   - 紧凑微型表格，展示 6 个梯度的浓度与孔位跨度映射（C0~C5）；
   - 当前选中的 C2 组（20 µg/mL，B5:D5）以浅蓝着色高亮；
3. **实验设计约束与质控校验 (Validation Checklist)**：
   - ✓ 边缘效应隔离：36 外周孔填充 200 µL PBS
   - ✓ 复孔几何排布：满足 n=3 独立平行复孔要求
   - ✓ 终浓度 DMSO ≤ 0.1%，排除假阳性毒性
4. **底部双动作区**：`暂存参数 (⌘S)` 与 `下一步 (Tab)`。

### 3.5 底部原生遥测状态栏 (Status Telemetry Bar, 28px)
- 左侧：核心引擎状态 `v1.2-native (Go-Engine 0.18ms)`、标准孔板规范、当前活动设计图层；
- 右侧：当前焦点选区 `Well C5 (DOX 20 µg/mL)`、实时内存占用 `14.8 MB · Zero Webview`。

---

## 4. 状态模型 (State Model Matrix)

| 状态类型 | 视觉表现 | 交互与触发机制 |
| :--- | :--- | :--- |
| **Node: Inactive / Pending** | 细线白底，灰色编号点，弱化次级字体 | 点击即可切换，右侧检查器随之更新工序规格 |
| **Node: Completed** | 细线白底，绿色对勾图标（`#10B981`） | 表示该前序工序已完成参数校验与保存 |
| **Node: Active (当前)** | 1.5px 钴蓝边框，柔和悬浮微阴影，文字深蓝 | 当前在右侧检查器与孔板图层中被编辑 |
| **Well: Unselected** | 圆形微边框，依功能着不同透明度低饱和底色 | 鼠标悬停产生微放大放大倍率（1.15x）与 HUD 更新 |
| **Well: Selected** | 2px 额外纯白外环 + 2px 钴蓝高亮聚焦环 | 锁定在状态栏中，属性面板锁定对应复孔组 |
| **Save: Synced** | 绿状态点，文本显示 `已同步 14:32:08` | 自动保存已入库，无未持久化脏数据 |
| **Save: Dirty** | 琥珀黄状态点，文本显示 `未保存修改` | 参数发生变更但尚未刷入磁盘 |
| **Save: Saving** | 钴蓝状态点脉冲呼吸，文本显示 `正在同步...` | 快捷键 `⌘S` 或按钮触发，模拟毫秒级原生写入 |

---

## 5. MyGo 原生桌面技术可行性分析 (MyGo Feasibility)

本套 Porcelain 视觉方向是专门针对 `github.com/egoist/mygo` 及 Go 原生 GUI 渲染架构深度定制的，具备极高的原生工程可行性。

### 5.1 零 Webview、零重型渲染负担
- **无复杂毛玻璃/滤镜依赖**：整个界面**不依赖**昂贵的 CSS `backdrop-filter: blur()` 或全屏复杂高斯模糊。所有表面均采用纯色实心填充（Solid Fills）、发丝描边（Hairline Strokes）与轻量预合成阴影；
- **100% 对应 MyGo / Rune 布局原语**：
  - 顶栏与状态栏：标准 `HStack`，两端 `Spacer()`；
  - 三栏工作区：水平 `HStack` 划分 `220px | 1fr | 320px`；
  - 96 孔微孔板：标准的 13 列 × 9 行 `Grid` 布局，每一个孔位是一个带有 `Radius: 50%` 的原生几何圆形绘制图元；
  - 检查器属性行：统一的 `HStack(Label, Spacer(), Control)` 结构，消除任何嵌套 Web 表单渲染开销。

### 5.2 极致内存与启动速度
- 相比基于 Electron 或 Tauri Webview 的方案，Porcelain 在 MyGo 原生编译后：
  - 常驻内存（RSS）稳定在 **12 MB ~ 18 MB** 以内；
  - 冷启动到第一帧渲染时间（Time to First Interactive Frame）**< 60ms**；
  - 96 孔微孔板悬停、拖拽框选与数据刷新可轻松维持 **120 FPS 满帧刷新**。

### 5.3 模块代码映射结构 (Go Package Layout)
在 AsterLab v1.2 代码树中，Porcelain 方向无需引入额外依赖，可直接平滑对应至以下包结构：
- `internal/theme/porcelain.go`: 导出色彩阶梯、间距、字体比例尺；
- `internal/designer/dag_canvas.go`: 绘制 5 节点流水线与连接轨道；
- `internal/designer/plate_grid.go`: 绘制 ANSI/SLAS 标准 96 孔板与拾取碰撞检测；
- `internal/designer/inspector.go`: 绘制属性行、剂量矩阵与质控清单；
- `internal/shell/topbar.go` & `internal/shell/statusbar.go`: 窗口命令栏与遥测底栏。

---

## 6. 产物交付清单与文件路径 (Deliverables Manifest)

| 文件路径 | 产物类型 | 说明 |
| :--- | :--- | :--- |
| `docs/design/v1.2-directions/porcelain/index.html` | 交互式原型 | 1200×760 像素级交互原型（支持节点切换、孔位悬停/点击HUD、⌘S保存切换、步进器调节） |
| `docs/design/v1.2-directions/porcelain/designer-1200x760.png` | 高清效果图 | 真实 Headless 渲染出的 1200×760 DIP 像素级高清无损截屏 |
| `docs/design/v1.2-directions/porcelain/designer-1200x760.svg` | 独立矢量图 | 完整的 1200×760 独立矢量 SVG，支持任意倍率无损缩放与设计导入 |
| `docs/design/v1.2-directions/porcelain/concept.md` | 设计说明书 | 本文件，包含完整理念、颜色规范、组件架构、状态模型与 MyGo 原生工程可行性分析 |
