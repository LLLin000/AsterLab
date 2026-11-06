# View Canvas 实装记录

- 日期：2026-11-06
- 基线：`7374c6e` 后的本地修改，分支 `feature/slint-native-spike`
- 状态：可运行的浏览器原型，不是最终视觉规范

## 已实现

- `web/view.html` 使用 SVG 画布渲染 `ViewDocument` 的 `WorkflowGraph`；节点来自真实组件，边来自真实 `edges`。
- 节点图标来自 Lucide 的 ISC 线性 SVG（grid-3x3、pipette、clock、microscope、flask-conical），由 `VisualSpec.symbol` 或组件 definition 选择；不再手绘或按标题关键词推断。来源与许可见 `2026-11-06-icon-library.md` 和 `web/icons/LICENSE.md`。
- 滚轮缩放、空白拖拽平移、适应屏幕、节点选择、双击进入嵌套 workflow（有 workflow 时）。
- 节点拖动写入 `VisualSpec.x/y`，通过 `set_visual` ViewPatch 保存；只改 View，不改 Protocol。
- 撤销/重做使用 `instance_id + before/after` 快照，保存串行化，并在服务器确认后更新历史。
- Step Detail 通过 `GET /api/views/{view_id}/steps/{instance_id}` 读取后端精确投影：后端加载真实 ProtocolVersion 的 `root_bindings`，展开 ProtocolPreview，再按 `step_path` 返回当前步骤的参数、单位、来源和未解决项；前端不再自行拼装。
- 孔板详情使用 `/api/views/{id}/data` 返回的 `rows`/`columns` 按 LabwareProfile 渲染，当前支持 6、96 和 PCR 96 孔；孔位可选择并直接用于计算。
- 浏览器验收截图见 `target/asterlab-bioicons.png`。

## 仍未实现

- 自由手绘、笔刷、复杂路径编辑、多人协作。
- 通用的 Table/Calculator DataBinding；目前只接受显式 typed input。
- Operation TargetSelection 到 `position_ids[]` 的自动推导；当前必须由调用方显式给出目标孔位；缺失保持 unresolved。
- 自动布局目前是前端默认网格；尚未做碰撞规避或吸附算法。
