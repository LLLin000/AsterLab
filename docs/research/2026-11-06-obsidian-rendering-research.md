# Obsidian 渲染逻辑研究：Pele、Table、Canvas

- 日期：2026-11-06
- 目的：研究 Obsidian 的渲染分层，重点看 Pele（Mermaid → SVG）、Bases 表格和 Canvas，判断哪些模式适合 AsterLab 的统一工作区。
- 证据：Obsidian 核心闭源；本文依据官方文档、公开 typings、Pele 开源仓库和社区技术拆解。标注“实现”的只写可核对事实。

## 结论

Obsidian 值得学的不是某个具体 UI，而是它的**渲染分层**：

```text
源数据 / Markdown
   ↓ parse
结构化模型
   ↓ layout
可复用布局
   ↓ render
HTML / SVG
   ↓ mount
随容器宽度、字体、主题刷新
```

AsterLab 应借用这四层，而不是把组件渲染写成一个巨大的 `renderDetail()`：

1. `parse`：ComponentInstance / ViewDocument 已经是模型，不需要再解析。
2. `layout`：节点位置、卡片高度、表格列宽独立计算。
3. `render`：按 Definition 选择 renderer；同一个 renderer 支持 compact/detail。
4. `mount`：处理容器 resize、字体、主题、错误和重绘。

Pele 是这套分层最干净的公开样例；Bases 表格是虚拟化表格的样例；Canvas 是“内容与画布分离”的样例。

## 1. Pele：Mermaid 的 SVG 渲染器

Pele 是 Obsidian 官方的开源 Mermaid 渲染器，MIT，TypeScript，无运行时依赖。

公开入口：

```ts
import { parse, render, mount } from 'pele';

const model = parse(text);
const { svg } = render(model);
mount(element, text, { zoom: 'auto' });
```

内部流程：

```text
detect -> preprocess -> parse -> DiagramModel
       -> layout -> SVG path/text
       -> mount：读容器宽度/字体，必要时重新 render
```

值得 AsterLab 学习的点：

| 点 | Pele 做法 | AsterLab 对应 |
|---|---|---|
| 模型与渲染分离 | parse 产出 `DiagramModel`，render 只负责 SVG | ViewDocument / ComponentInstance 与 renderer 分离 |
| 类型注册 | `register(diagram)`；按需加载 | component definition → renderer registry |
| 主题 | CSS variables；颜色变化不重绘 | `--surface/--line/--accent` token |
| 容器适配 | `mount` 读宽度和字体，宽度变化时重绘 | ResizeObserver / 分栏变化后刷新卡片 |
| 错误 | 保留旧图并叠加错误；有错误码 | unresolved 状态不伪造结果 |
| 安全 | 转义文本、过滤链接/样式、限制输入/输出/边数 | 不接受可执行 SVG/脚本；限制导入内容 |
| 缩放 | zoom 独立于 layout | 画布缩放与节点布局分离 |

不应照搬：

- Mermaid 语法不适合作为 AsterLab 的规范模型；它只能作为导入/导出或图表示例。
- Pele 的自动布局面向 Mermaid 图形，不处理“多组样品进入同一节点”的实验语义。
- Pele 的 `diagram` 注册表是图形类型，不是科研组件类型。

来源：

- <https://pele.run/>
- <https://github.com/obsidianmd/pele>
- <https://github.com/obsidianmd/pele/blob/main/src/core.ts>
- <https://github.com/obsidianmd/pele/blob/main/src/mount.ts>

## 2. Obsidian 的 Markdown 阅读视图

公开 typings 暴露了 `ReadViewRenderer`：

- 按 section 渲染，不是每次替换整篇 HTML。
- 维护 `sections`、`header/footer`、`reusable section` 和异步 section。
- 有 `set(text)`、`parseSync/parseAsync`、`queueRender/rerender`、`onRendered`。
- 有独立 sizer 和滚动状态，避免整页 reflow。
- 编辑态用 CodeMirror 6；阅读态是 Markdown → HTML 渲染管线。

AsterLab 的 Markdown 卡片目前很小，不需要完整 section 池；但应保留：

- 编辑态 / 预览态分离；
- 只在文本变化时更新预览；
- 长文档才考虑分 section 和异步渲染；
- 渲染必须 sanitize，不能直接注入任意 HTML。

来源：

- <https://obsidian-typings.github.io/obsidian-typings/public/api/obsidian/internals/ReadViewRenderer/>
- <https://docs.obsidian.md/Plugins/Editor/Editor+extensions>
- <https://help.obsidian.md/obsidian/credits>

## 3. Bases Table：虚拟化表格

Obsidian 的 Bases table 不是普通 Markdown 表格，而是数据视图：

- 一行对应一个文件/条目，列对应属性。
- 支持行高、列宽、列排序/调整、单元格选择、键盘移动、复制粘贴。
- 支持分组和每列 summary（sum/avg/median/...）。
- 内部 `BasesTableView` 只渲染视口行，保留少量 unused rows 复用；滚动时 `renderViewport()` 回收行。
- 列宽保存到视图配置，不写回数据。

AsterLab 应借：

- 表格 = 数据 + 列定义 + 视图设置（列宽/排序/行高），不把展示设置写进科学数据。
- 行数大时虚拟化；小表用语义化 HTML 即可。
- 表头操作、列宽、选择、键盘导航要明确。
- 分组/summary 属于视图，不是新的科学事实。

不应照搬：

- Obsidian 的“一行 = 一个文件”模型不适合实验表格。
- Bases 表格偏只读分析；AsterLab 需要可编辑单元格和单位/校验状态。
- 不因为表格能计算就自动把它当 Calculator 输入；仍需显式绑定。

来源：

- <https://obsidian.md/help/bases/views/table>
- <https://docs.obsidian.md/plugins/guides/bases-view>
- <https://obsidian-typings.github.io/obsidian-typings/catalyst/api/obsidian/internals/internal-plugins/bases/BasesTableView/BasesTableView/>

## 4. Canvas：内容与画布分离

Obsidian Canvas 使用 JSON Canvas：

- 顶层 `nodes` + `edges`。
- node：`id/type/x/y/width/height/color`；type 为 `text`、`file`、`link`、`group`。
- edge：`fromNode/toNode`、`fromSide/toSide`、`fromEnd/toEnd`、`label`、`color`。
- group 是视觉容器，不是新的内容类型。
- 文件/链接/文本内容与画布位置分开。

AsterLab 应借：

- 节点位置、尺寸、颜色、泳道是 View 层；科学身份来自 ComponentInstance。
- edge 的 `fromSide/toSide` 是视觉锚点，不代表样本语义。
- group 只能是视觉分组，不能自动生成样本合并。
- 画布与详情共享同一内容模型，但实例可以不同。

不应照搬：

- JSON Canvas 的 text/file/link 类型不足以承载实验组件。
- 自由连线/自由画布会削弱科学约束；AsterLab 必须保留 DAG/端口/类型校验。

来源：

- <https://jsoncanvas.org/spec/1.0/>
- <https://obsidian.md/help/plugins/canvas>

## 5. 对 AsterLab 的实现建议

### 5.1 渲染器接口

```ts
interface ComponentRenderer {
  parse?(raw: unknown): ComponentModel;
  layout(model: ComponentModel, box: Box): LaidOut;
  render(laidOut: LaidOut, mode: 'compact' | 'detail'): string | SVGElement;
  mount?(el: HTMLElement, model: ComponentModel): { refresh(): void; destroy(): void };
}
```

- `Operation` 的 compact 是图标+名称；detail 是参数+子组件。
- `Table` 的 compact 是摘要；detail 是可编辑表格。
- `Calculator` 的 compact 是公式摘要；detail 是输入+结果。
- `Markdown` 的 compact 是首行摘要；detail 是编辑/预览。
- `Workflow` 的 compact 是步骤数/分支数；detail 是子画布。

### 5.2 渲染分层

```text
ViewDocument
  ↓ 选择 component / surface
ComponentInstance
  ↓ definition-specific adapter
ComponentModel
  ↓ layout
LaidOut
  ↓ render
HTML / SVG
  ↓ mount / refresh
```

不要把布局、DOM 事件和科学校验混在一个函数里。

### 5.3 具体取舍

| 需求 | 采用 | 不采用 |
|---|---|---|
| 流程图 | SVG + 自己布局；可借 Pele 的 mount/refresh/主题 | 不把 Mermaid 当规范模型 |
| 卡片 | Masonry + 组件 renderer | 不为卡片和节点各建 schema |
| 表格 | 小表语义化 HTML；大表虚拟化 | 不照搬 Bases 的文件行模型 |
| Markdown | 编辑/预览分离 + sanitize | 不直接注入 HTML |
| 画布 | JSON Canvas 的 node/edge/visual 分离 | 不采用自由白板语义 |
| 错误 | unresolved + 保留旧内容 | 不伪造默认值 |

## 6. 最小实现顺序

1. 建立 `renderCompact(component)` / `renderDetail(component)` 的 Definition 分派。
2. 把 Table、Calculator、Markdown、Workflow 各自拆成 renderer。
3. 加 `mount` 生命周期：分栏/窗口变化时只刷新受影响的卡片。
4. 表格在行数超过阈值时虚拟化；小表保持可编辑语义。
5. Markdown 预览按文本变化更新；长文档再分 section。
6. 画布沿用当前 SVG + `VisualSpec`，只补 Pele 式的容器适配和错误显示。

## 7. 边界

- Obsidian 核心不是开源的；不要假设其内部 API 可复用。
- Pele 是 MIT 的可复用库，但它只负责 Mermaid → SVG；不是 AsterLab 的组件框架。
- Bases table 的行为来自官方帮助和公开 typings，具体内部实现可能随版本变化。
- 这些结论用于架构参考，不代表 AsterLab 已经完成对应实现。
