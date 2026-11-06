# Tabler UI 改版记录

- 日期：2026-11-06
- 分支：`feature/slint-native-spike`
- 设计系统：Tabler UI 1.7.3，本地 vendored，MIT
- 范围：仅 UI shell、布局与组件视觉；未修改 Scientific Core、Protocol/Design/Run

## 已落地

| 项目 | 实现 |
|---|---|
| Tabler CSS/JS | `web/vendor/tabler/tabler.min.css`、`tabler.min.js` |
| Sidebar | 232 px；项目选择、实验设计/Protocol/运行/资料；1100 px 以下折叠为 64 px |
| Header | 56 px；实验名、revision、状态、开发菜单（View ID 移入） |
| Canvas | 96 px 节点、16 px 点阵、2 px 边、视图工具；保留 Dagre/拖动/连接 |
| Detail | 12 列内容优先布局：Table/Note span 8，WellPlate/Calculator span 4 |
| WellPlate | 30 px 孔位、行列标签、全选/清空、选择详情、组别图例 |
| Table | 表头、行编辑、草稿/保存状态 |
| Calculator | 显式孔板选择、单位、未解析状态、独立结果区 |
| Markdown | 编辑/预览、draft、保存 |
| Proposal | 外部 Agent Inbox API；无内嵌 LLM 聊天 |
| Theme | 浅色/深色切换 |

## 验收截图

```text
target/tabler-1440-final.png
target/tabler-1920-final.png
target/tabler-dark-final.png
```

## 视觉差距清单

1. 卡片区在 7 个真实组件时仍需要内部滚动；首屏优先展示 Table/Note/WellPlate/Calculator。
2. Proposal 模块文件已实现，但主页面仍使用轻量 Inbox 面板；下一步统一为 Tabler offcanvas。
3. Table 模块的列宽和横向滚动还需在 1280 px 下做一次逐列检查。
4. 深色模式由 Tabler `data-bs-theme` 驱动；模块 token 已有深色变量，但尚未做全组件截图审查。
5. Workflow 节点在 1920 px 下密度偏低，后续可增加 fit-to-content 或更大的默认缩放。

## 组件模块合同

```text
web/components/CONTRACT.md
```

每个组件通过 `window.AsterUI.register(id@version, { renderDetail, mount })` 注册；
宿主持有 ViewDocument、ViewChangeSet、Selection/Proposal API。外部 Agent 使用 CLI/HTTP
（`view.proposal.*`）投递提案，不通过页面内聊天直接改数据。

## 验证

```text
node tests/w5-layout.test.cjs
node scripts/w6-proposal-tests.cjs
cargo test -p asterlab-core
cargo test
cargo clippy -p asterlab-core --all-targets
```
