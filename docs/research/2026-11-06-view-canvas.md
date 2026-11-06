# View Canvas 实装记录

- 日期：2026-11-06
- 基线：`7374c6e` 后的本地修改，分支 `feature/slint-native-spike`
- 状态：可运行的浏览器原型，不是最终视觉规范

## 已实现

- `web/view.html` 使用 SVG 画布渲染 `ViewDocument` 的 `WorkflowGraph`；节点来自真实组件，边来自真实 `edges`。
- 节点图标按 `props.label` / definition 显示，支持铺板、加药、孵育、读板等符号。
- 滚轮缩放、空白拖拽平移、适应屏幕、节点选择、双击进入嵌套 workflow（有 workflow 时）。
- 节点拖动写入 `VisualSpec.x/y`，通过 `set_visual` ViewPatch 保存；只改 View，不改 Protocol。
- 撤销/重做使用本地布局历史并持久化；浏览器验收截图见 `target/canvas-four-steps.png`。

## 仍未实现

- 自由手绘、笔刷、复杂路径编辑、多人协作。
- 通用的 Table/Calculator DataBinding；目前只接受显式 typed input。
- Operation TargetSelection 到 `position_ids[]` 的自动推导；当前必须由调用方显式给出目标孔位；缺失保持 unresolved。
- 自动布局目前是前端默认网格；尚未做碰撞规避或吸附算法。
