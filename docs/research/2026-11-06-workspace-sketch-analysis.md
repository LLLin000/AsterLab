# 草图实现研究：实验设计工作区

- 日期：2026-11-06
- 输入：手绘草图：左侧导航栏、顶部多行实验设计流程、下方表格/流程/稀释计算器/Markdown 卡片，以及“自由组合”。
- 结论：这不是一个页面，而是三层工作区：**导航壳 + 流程画布 + 组件卡片区**。现有 `ViewDocument`、`WorkflowGraph`、`ViewPatch` 能承载，不需要新造科学模型；需要补的是布局层和卡片交互。

## 1. 草图拆解

```text
┌────────┬──────────────────────────────────────────────┐
│ 导航栏  │ 实验设计流程                                  │
│        │  A  →  B  →  C  →  D                         │
│  ●     │  A' →  B' →  C' →  D'                        │
│  ●     │  A'' ┐                                       │
│  ●     │  A'''┘ → B'' → C'''                          │
│        ├──────────────────────┬───────────────────────┤
│        │  表格                 │  稀释计算器            │
│        │  [grid]              │  ○ × ○ = ○            │
│        ├──────────────────────┼───────────────────────┤
│        │  流程                 │  Markdown             │
│        │  A→B→C→D             │  ─────                │
│        │                      │  ─────           [+]  │
└────────┴──────────────────────┴───────────────────────┘
```

四个区域的性质不同：

| 区域 | 作用 | 数据来源 | 交互 |
|---|---|---|---|
| 导航栏 | 切换工作区 | App 状态 | 选择、折叠 |
| 顶部流程 | 看实验设计/步骤关系 | `ViewDocument.root.workflow` | 缩放、平移、选中、进入子流程 |
| 下方卡片 | 查看/编辑当前步骤的组件 | `slots.detail` | 添加、删除、移动、编辑 |
| 卡片内容 | 表格/计算/Markdown/子流程 | 各自组件的 `props` 或 `model_ref` | 组件内编辑 |

## 2. 导航栏

采用 Material 3 navigation rail 的约束：

- 3–7 个一级入口，固定左侧，不随内容滚动。
- 折叠时只显示图标；展开显示文字；当前项用 pill/胶囊高亮。
- 桌面优先；窄窗口切换为底部导航或抽屉，不能同时出现 rail 和 bar。

实现：现有 `aside` 可保留为 rail，增加折叠状态和 active 状态；不要把它做成左侧大面板。草图中的三个椭圆就是三个入口，不需要新增路由框架。

参考：<https://m3.material.io/components/navigation-rail/guidelines>

## 3. 顶部流程画布

### 3.1 数据

- 节点：`WorkflowGraph.nodes`，每个节点指向 `ComponentInstance`。
- 连线：`WorkflowGraph.edges`，`from`/`to` 是 `node_id`。
- 节点位置：`VisualSpec.x/y`（已有）；拖动只写 View。
- 行/泳道：草图的 A/A'/A'' 是**视觉泳道**，不是新的科学对象。第一版用 `VisualSpec` 增加 `lane` 或按 `y` 分组；不把 lane 写入 Protocol。
- 分支/合并：由真实 edges 表达；没有 edge 就不画箭头，不能按数组顺序假装执行顺序。

### 3.2 布局

草图的多行结构适合分层 DAG：

- 简单树：Dagre，按 rank 分层，快。
- 复杂分支/嵌套：ELK layered，支持 compound、ports、正交/样条路由。
- 当前自研自动布局只适合单行；应换成布局引擎，节点坐标仍存回 View。

推荐顺序：

1. 先用 Dagre/ELK 计算节点位置，前端渲染 SVG。
2. 手工拖动后保存 `VisualSpec.x/y`，布局引擎不再覆盖已锁定节点。
3. 选中节点显示高亮轮廓和连接点；双击有 `workflow` 的节点进入子流程。
4. 边用 SVG path；分支/合并用正交或轻微曲线，减少交叉。

参考：

- React Flow 布局指南：<https://reactflow.dev/learn/layouting/layouting>
- ELK layered：<https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html>
- elkjs：<https://github.com/kieler/elkjs>

### 3.3 视觉

- 节点默认只有科学图标 + 步骤名；选中才显示边框、连接点和工具按钮，避免画布变成“每个节点一个卡片”。
- 当前 Lucide 图标先保留；图标由 `VisualSpec.symbol` 决定，不按标题猜。
- 画布网格、背景和工具栏层级要低于节点，避免抢视觉。

## 3.4 同一个语法，不是两套系统

用户已经明确：顶部流程图和下方组件区**不是两套设计**。这里采用现有 `ComponentInstance` 作为唯一块模型：

```text
ComponentInstance
├─ definition: { id, version }       // operation / workflow / table / calculator / note / plate
├─ model_ref                         // 可选：Protocol step / PlannedContainer
├─ props                             // 组件自己的数据
├─ slots                             // 详情子组件
├─ workflow                          // 可选：这个组件展开后的子流程
└─ visual                            // 可选：x/y/symbol/lane
```

- 顶部节点是 `WorkflowNode { node_id, component }`，下方卡片是某个组件 `slots.detail` 里的 `ComponentInstance`；两者类型相同、校验相同、ViewPatch 相同。
- **不是同一个实例**：一个 Operation 节点与它内部的 Table/Calculator 是父子实例。共用的是类型、Definition、Renderer 契约和 ViewPatch，不是 instance 身份。
- `WorkflowGraph.edges` 只是视觉关系，不自动表示样本 pooling/batching/合并；真实样本流转仍来自 Design/Protocol/Run 的既有引用。
- “节点视图”和“详情视图”只是同一个组件的两种 presentation：operation 的节点视图是图标+步骤名，详情视图是参数+子组件；table 的节点视图可以是缩略表，详情视图是可编辑表格。
- 不为节点和卡片各造一套 schema。差异放在 renderer 的 `view_mode`（compact/detail）和组件的 `props`，不放在科学对象上。
- 多组样品进入同一节点：用 `WorkflowGraph.edges` 的多个入边表达；如果只是视觉分组，用 `VisualSpec.lane`/`group` 或 `props.group_label`，不把 lane 写进 Protocol。样本身份仍来自 Design/Run 的既有引用，不从位置推断。
- 受限流程图：图必须是 DAG（现有校验）；节点类型受组件注册表限制；只允许声明过的连接；禁止自由连线到任意位置。拖动只改 `VisualSpec`，新增/删除/改顺序必须走正式命令。

这样顶部大图、底部小流程、表格、计算器、Markdown 都是同一个块模型的渲染结果，用户可以“把组件放到画布/详情区”，而不是在两个编辑器之间搬运数据。

## 3.5 成熟实现参考

| 项目 | 可借鉴 | 不照搬 |
|---|---|---|
| React Flow sub-flows/handles | parent/child 节点、显式端口、子图 | 它的编辑器数据不是科学事实；不要把拖拽结果直接写成 Protocol |
| ELK Layered | DAG 分层、compound group、端口约束、正交路由 | 不要把布局参数暴露成用户模型 |
| diagram-js / bpmn-js | “命令 + 规则 + 连接校验”的交互架构 | 不引入 BPMN 的完整符号和执行语义 |
| JointJS ports/routers | 端口和 Manhattan 路由分离 | 不为了路由换整套图框架 |
| GoJS groups/ports | 节点、链接、端口记录分离 | 不让 group 自动等于样本或执行语义 |
| Dagre | 简单分层布局、rank/spacing | 复杂嵌套和端口布局时改用 ELK |
| tldraw shapes/bindings | parented shape、持久 binding | 不采用自由白板式任意摆放 |
| Mermaid | 文本 fixture/导入导出 | 不作为唯一编辑模型 |

## 3.6 约束流程图规则

1. 图必须是 DAG；边只能连接声明过的端口。
2. 节点类型来自组件注册表，不允许自由文本节点。
3. 多组样品进入同一节点用多条真实入边/端口表达，不为每个样品复制流程节点。
4. 视觉泳道、分组和坐标只写 `VisualSpec`；不能改变科学顺序。
5. 拖动只改位置；连线手势必须通过端口/类型/方向校验。
6. 自动布局只负责位置和路由，不能决定哪些节点相连。
7. 没有 edge 就不画箭头，不能按数组顺序猜执行顺序。


## 3.7 领域工具给出的关键约束

| 来源 | 事实 | 对 AsterLab 的含义 |
|---|---|---|
| Benchling Workflows | 任务 schema、task group、字段映射和 router 是显式结构 | 流程节点和分支规则要显式；不能靠画布位置推断 |
| Labguru protocols / Plate Designer | protocol 模板复制到 experiment；孔位关联样本/试剂 | 模板版和本次实例要区分；孔板是有结构的数据，不是装饰 |
| Opentrons Protocol Designer | 先选 labware/liquid，再添加 typed transfer steps | 节点面只显示关键摘要，详细参数进详情 |
| Autoprotocol / LabOP | JSON 协议、容器/孔位引用、命名 sample collection | 多样品应作为显式 collection/batch 输入，而不是复制节点 |
| n8n / Node-RED | nodes 与 connections 分离；subflow 是普通节点的组合 | 可以借结构，不要把执行语义搬进 View |

### “多样品进入同一节点”的推荐表达

```text
Group A ─┐
Group B ─┼─▶ [ shared operation ] ─▶ ...
Group C ─┘
```

- 一个 process node 接收多个入边，边上携带已有的组/样品引用或 collection。
- 节点上显示 `3 groups · 24 samples` 这类摘要；展开详情才看到成员。
- 明确选择语义：`each` / `pool` / `explicit subset`；如果领域没有确定，保持 unresolved。
- 不把 24 个样品复制成 24 个流程节点，也不把 lane 名称当样本 ID。
- 样本身份/计数必须来自现有 Design/Run/ContainerPlan，不因为视觉分组自动生成。

## 3.8 统一模型的建议增量

现有 `ComponentInstance` 已经足够承载节点和卡片。建议只做小增量：

```text
WorkflowEdge {
  id,
  from: { instance_id, port_id },
  to:   { instance_id, port_id },
  // 可选 view-only：route points / style
}
```

- 如果暂时不做端口级连接，至少保留 `from`/`to` 的 node_id，并把“边只属于哪个输入端口”作为后续校验。
- 添加 `VisualSpec.lane` 或 `props.group_label` 作为视觉分组；不写 Protocol。
- `ComponentInstance.workflow` 表示“这个块内部还有流程”；`slots.detail` 表示“这个块的详情组件”。两者都是同一个块的子关系，不是第二套系统。
- 不要新增 `SampleCollection` 科学实体；先使用现有 `ModelRef`/`DataBinding`/Design 数据，若确有缺口再补最小引用。

## 3.9 UI 尺寸与交互规则

| 区域 | 规则 |
|---|---|
| 左侧导航 | 折叠 56–64 px，展开 200–240 px；3–7 个入口；窄屏换底部/抽屉 |
| 顶部流程 | 桌面约占 55% 可用高度，最小 280–320 px；可拖动分隔条 |
| 下方卡片 | 最小卡片宽约 280 px；12–16 px 间距；Masonry 自动拼图 |
| 卡片 | 标题（名称/类型/状态/溢出）+ 内容 + 可选来源；一致的内边距/焦点/选中态 |
| 交互 | 拖动节点只改视觉；重排卡片用显式 arrange/键盘替代；删除需确认 |
| 状态 | 默认/悬停/键盘焦点/选中/编辑/禁用/加载/unresolved 全部定义 |

来源：Material 3 navigation rail、Fluent 2 layout、Carbon data table、Masonry、Muuri、AG Grid keyboard navigation、WCAG 2.2 target size。

## 3.10 分阶段实现

1. **统一块渲染**：同一个 `ComponentInstance` 渲染 compact node / detail card；不新增 schema。
2. **受限图**：边/端口校验、DAG、泳道视觉、多入边摘要；先手动布局，后接 ELK/Dagre。
3. **卡片内容**：表格编辑、稀释计算器、Markdown 编辑/预览；每块有 unresolved 状态。
4. **自由组合**：组件库、添加/删除/移动、Masonry 重排；拖动重排再评估 Muuri/Gridstack。
5. **持久化**：布局/顺序写 ViewChangeSet；科学变更仍走 Protocol/Design 命令。

## 3.11 不采用

- 不采用自由白板式 mind map；不允许任意连线。
- 不为节点和卡片各建一套数据模型。
- 不让拖动改变 Protocol 顺序。
- 不把 lane、坐标、自动布局结果写入科学对象。
- 不因为 Markdown/Table 有数字就自动成为计算参数。

## 4. 下方组件卡片区

### 4.1 布局

草图的四块卡片适合 **Masonry + 拖拽**，不是固定竖排：

- 纯自适应拼图：Masonry（`7c05d4f` 已接入；更早版本使用 CSS Grid，当前代码以 `web/vendor/masonry.pkgd.min.js` 为准）。
- 需要拖动/排序/过滤：Muuri（MIT），支持 masonry-like 布局、拖拽和动画。
- 需要 12 栏、可缩放、布局 JSON：Gridstack.js。

当前需求是“自由组合 + 拼图”，建议分两步：

1. 保留 Masonry，先把卡片内容做对（表格、计算器、Markdown）。
2. 再评估 Muuri/Gridstack；不要同时引入两个布局库。

卡片顺序仍以 `slots.detail` 为真实顺序，拖动/移动用 `ViewChangeSet` 保存；网格坐标是 View 层，不是科学顺序。

### 4.2 表格卡片

- 使用 `content.table` / `layout.table` 现有组件，不新建表格模型。
- 支持列定义、行编辑、单位列、空状态；输入只改组件 `props`。
- 不把任意单元格自动解释为 Calculator 输入；计算必须显式绑定。
- 大表需要固定表头、键盘导航、列宽调整；先做最小可编辑表格。

### 4.3 稀释计算器

草图是 `C1V1 = C2V2` 的公式卡，不是通用表达式引擎。

- 复用 `tool.calculator` / `lab.solution_calculator`；不新增科学规则系统。
- 卡片结构：
  - 输入：母液浓度、终浓度、终体积；每个输入带单位、来源、解析状态。
  - 输出：所需母液体积、稀释剂体积；显示公式和 ToolAction 版本。
  - 未输入时显示 unresolved，不填默认值。
- 计算输入应来自显式字段或已绑定参数；不从表格第一格偷值。

### 4.4 Markdown 卡片

- 现有 `content.note` 可承担第一版；`content.markdown` 若尚未实现，不应为了 Markdown 新建组件类型。
- 编辑/预览分栏或切换；保存走 `update_props`。
- Markdown 输出必须 sanitize；不允许脚本/外链执行。
- 预览只读，编辑态可切回；不把 Markdown 文本当科学参数。

### 4.5 添加与自由组合

- “+”打开组件库，先只暴露已实现 Definition。
- 添加：`add_slot_child`；删除：`remove_instance`；移动：`move_slot_child`；均走 `ViewChangeSet` 和 CAS revision。
- 添加/移动绝不能修改 Protocol 执行顺序；改变步骤必须走正式科学命令。
- 空卡、未知类型和缺少输入都显示明确的 unresolved 状态，不伪造内容。

## 5. 视觉与可用性规则

- 8pt 间距体系；卡片圆角 12–16；边框低对比；标题 14/16，正文 12/13。
- 顶部画布和下方卡片区用可拖动分隔条，默认约 55/45。
- 画布工具栏浮动在顶部中间；不要在每个节点上常驻一排按钮。
- 卡片标题行：名称 + 类型/来源 + 溢出菜单；编辑控件在卡片体内。
- 所有关键状态使用文字 + 颜色，不仅靠颜色：resolved、unresolved、来源、版本。
- 键盘：Tab 焦点、Enter 选择、Esc 返回、Delete 删除组件前确认。
- 动效只用于添加/重排/进入子流程；遵守 `prefers-reduced-motion`。

参考：

- Material 3 navigation rail：<https://m3.material.io/components/navigation-rail/guidelines>
- Material 3 breakpoints：<https://m3.material.io/foundations/layout/breakpoints>
- React Flow subflows：<https://next.reactflow.dev/examples/grouping/sub-flows>
- Muuri / Gridstack 对比：<https://www.sencha.com/blog/must-try-javascript-grid-layouts-for-modern-web-design/>

## 6. 建议的实现顺序

1. **壳层**：导航栏 + 顶部画布/底部卡片分栏 + 卡片标题/操作。
2. **画布**：真实 edges、泳道布局、选中/进入子流程；不新增科学对象。
3. **卡片**：表格编辑、稀释计算器、Markdown 编辑预览。
4. **自由组合**：组件库、添加/删除/移动、Masonry 重排；再评估拖拽库。
5. **持久化**：所有布局/顺序写 ViewChangeSet，刷新恢复；旧 View 自动布局兼容。

## 7. 不做

- 不新增 OperationModule、LabwareProfile 或第二套科学模型。
- 不把画布拖动当成 Protocol 重排。
- 不让 Markdown/Table 自动成为科学参数。
- 不在这一轮引入完整 Figma/Excalidraw 自由绘制。
