# UI 参考合成：n8n / Benchling / Linear / Obsidian

- 日期：2026-11-06
- 目的：把四个成熟产品的交互原则落到 AsterLab 的具体尺寸、组件和状态，不做表面换皮。
- 证据：官方文档、源码和公开说明；数值为 AsterLab 适配值，不声称是原产品测量值。

## 采用的原则

| 参考 | 采用的原则 | AsterLab 落地 |
|---|---|---|
| n8n | 画布安静、节点紧凑、操作上下文出现 | 16 px 网格、96×96 节点、2 px 边、悬浮缩放工具、选中才显示连接点 |
| Benchling | 孔板是结构化工作区，表格有明确行高和提交状态 | 30 px 孔位、行列标签、选择/清空、组别图例、表格草稿与保存状态 |
| Linear | 导航退后、标题/正文/元数据分层、少而准的强调色 | 56 px rail、22 px 标题、15 px 卡片标题、12 px 元数据、单一蓝色强调 |
| Obsidian | 画布与详情分区、选中局部操作、2×2 卡片工作区 | 顶部流程 + 下方固定详情；卡片不进无限画布；选择与 Focus 分离 |

## 具体规格

### 画布

- 背景：低对比点阵；AsterLab 使用 16 px 节奏。
- 节点：96×96 圆角节点，图标 48×48，标题在下方；标题和类型不塞进节点内部。
- 边：2 px 中性色，端口有 8 px 不可见命中区；只有 direction 需要时显示箭头。
- 控制：缩放/适应/选择/手形集中在一个浮动工具条；不在每个节点常驻按钮。
- 选中：2 px 蓝色轮廓 + 浅色底；焦点和选中是不同状态。
- 画布顺序只是 View；拖动不改 Protocol。

### Step Detail

- 区域：顶部流程约占 34vh；详情占剩余高度，可调整。
- 卡片：内容感知的 2–3 列网格；图例、组件库、Inbox 以浮层或抽屉出现，不挤压主网格。
- 卡片头：15/20 标题，12/16 元数据；操作按钮 28–32 px。
- 卡片内容独立滚动；卡片本身填满网格单元。
- 未保存：行/卡状态文字 + 轻量标记；不能只靠颜色。
- 空态：说明下一步操作，不显示假数据。

### 孔板

- 96 孔：30×30 px，4 px gap；行/列标签始终可见。
- 选择：本地选中，不自动全选；选择详情条显示孔位、role、target。
- 组别/条件：独立图例；不把多个维度压成一种颜色。
- 未解析板型：显示 Profile 与原因，不回退 8×12。

### 表格

- 表头 32 px，行 34 px；数字右对齐、单位在表头或字段可见。
- 单元格编辑以草稿保存；保存状态与错误就地显示。
- 小表用语义化 table；大表才虚拟化。
- 不把任意单元格自动送进 Calculator。

### 计算器

- 输入 36 px；值 + 单位 + 来源；未解析时明确显示“待确认”。
- 结果区独立，显示 ToolAction、孔数、单位和 state。
- 计算器必须显式绑定孔板；不允许 firstPlate 猜测。

### 导航与排版

- Rail 56 px；工作区标题 22/28；卡片标题 15/20；正文 13/18；元数据 12/16。
- 4/8/12/16/24 px 间距；卡片圆角 10，输入 6。
- 主背景 `#f7f8fa`，surface `#fff`，line `#e4e7ec`，ink `#202938`，muted `#667085`，accent `#2563eb`。
- 只有主操作/选中/焦点使用 accent；状态色只表达状态。

## 已落地的组件模块

```text
web/components/wellplate.js/css
web/components/table.js/css
web/components/calculator.js/css
web/components/markdown.js/css
web/components/flow.js/css
web/components/proposal.js/css
web/components/tokens.css
```

所有模块通过 `window.AsterUI.register(id@version, {renderDetail, mount, renderCompact})` 注册；宿主负责 grid、mount 生命周期、Plate/Calculator 上下文和 ViewChangeSet。

## 不采用

- 不复制 n8n 的集成 logo 节点。
- 不把 Benchling 的文件/结果表模型搬进 AsterLab。
- 不用 Linear 的整个布局，只借层级、间距和安静感。
- 不采用 Obsidian 的自由无限画布和自由卡片缩放。
