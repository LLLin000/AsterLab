# Step Detail 组件视觉审计

- 日期:2026-11-08
- 范围:Step Detail 的 Execution brief、表格、Markdown/说明、组件弹层
- 视口:1440×1000、390×844;深色和浅色主题
- 状态:浏览器走查;不是用户测试

## 结果

Step Detail 从单列全宽改为桌面两列:

- Execution brief:整行
- 材料与试剂 / 仪器与程序 / 检查与接受:两列;第三张自动换行
- Markdown / 说明:整行
- 手机:全部单列

## 组件调整

| 组件 | 调整 |
|---|---|
| Execution brief | 标题/说明层级、元数据分隔线、检查项间距 |
| 只读表格 | 取消 220px 最小高度;表头更紧凑;首列加粗;行 hover;单元格 8/10px padding |
| 可编辑表格 | 保留编辑状态、保存按钮和输入可访问性 |
| Markdown/说明 | 保持整行宽度;继续使用现有预览/编辑样式 |
| Proposal 小窗 | 增加 ×;点同一卡片关闭;点外部关闭;Esc 关闭 |
| + 组件面板 | 点外部关闭 |
| 移动端 | 两列变单列;无文档级横向溢出 |

## 截图证据

- `target/components-compact.png`:深色主题两列表格
- `target/detail-light.png`:浅色主题两列布局
- `target/detail-two-column-final.png`:两列布局
- `target/detail-current.png`:调整前对照

## 发现

- 只读表格原先 `min-height:220px`,两行数据也占满整块;已按内容收缩。
- 两列表格下,长文本仍可换行;当前演示材料/仪器没有溢出。
- 组件小窗不会自动关闭的问题已修;点击同一卡片、空白、× 或 Esc 都关闭。
- `+ 组件` 面板点外部关闭。

## 尚未覆盖

- 当前演示 View 没有 calculator、well plate、workflow 子流程卡片,未做这些组件的真实截图审计。
- 390px 只检查了表格和两列切换,未逐项检查 calculator/plate 的触控。
- 未做真实设备触控、对比度测量或用户走查。
