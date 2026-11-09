# AsterLab 组件工艺与交互细节规范

- 日期：2026-11-10
- 适用范围：AsterLab Studio / MyGo Native UI Kit
- 原则：**安静、精密、可逆、可键盘操作、反馈与真实状态一致**

## 组件通用状态

每个可交互组件必须定义：

`default / hover / focus-visible / pressed / selected / disabled / loading / error / read-only`

- hover 只在 `hover` 能力设备生效；不要作为唯一反馈
- focus-visible 使用 2px 外轮廓 + 2px offset，对比度至少 3:1
- pressed 只做轻微位移/明度变化，不改变布局尺寸
- disabled 降低对比但不隐藏原因；不可用动作应说明 why
- 状态不能只靠颜色：同时用图标、文字、位置或字重
- 键盘焦点不能被浮层/粘性栏完全遮挡

依据：WCAG 2.2 SC 2.4.7 / 2.4.11 / 2.4.13；NN/g Button States；Material 3 states。

## 反馈时序

| 时长 | 设计 |
| --- | --- |
| < 100ms | 直接操作，立即显示结果 |
| 100–1000ms | 保持流程；按钮显示 pressed/working |
| > 1s | 显示明确进行中状态；不要静默 |
| > 10s | 进度、可中断、可离开再回来 |

- 本地 UI 反馈不等待 Core；数据状态必须等 Core 确认
- 异步响应回来时，只能更新比它新的本地 intent；过期响应丢弃
- 保存按钮点击后立刻进入 `Saving`，成功后显示 `Saved rev X`；失败保留草稿并显示可操作错误

依据：NN/g Response Time Limits；Primer Saving；Cloudscape inline edit。

## 按钮

- 高度：28（紧凑）/32（默认）/36（主操作）
- 水平 padding：10/12/16；图标按钮 28/32，命中区域至少 32
- 圆角：6；主按钮只用于每屏一个主操作
- Primary：accent 填充；Secondary：1px border + surface；Ghost：无边框、muted
- 状态：hover 背景提升 4–6%；pressed 再深 3–5%；disabled 40–50% 对比
- 按钮文案用动词：保存、创建运行、应用粘贴；不要“确定/提交”这种泛词
- 按钮宽度不因 loading 文案改变；loading 保留原宽度，只换状态
- 危险操作不用主色；二次确认或 undo

## 输入框 / 字段

- 高度 32；水平 padding 10–12；圆角 6
- label 在字段上方 11px muted，不要只用 placeholder 代替 label
- 单位靠近值显示；数字 tabular，右对齐（表格中）
- focus：1px accent 边框 + 2px 外轮廓；错误：边框 danger + 11px 错误文本
- 错误文本在字段下方，不弹 toast；保留用户已输入内容
- 空白 ≠ 0；缺失/未测/无效是不同状态
- Enter 提交单行字段，Escape 恢复原值；Tab 顺序符合视觉顺序
- 多字段表单只有一个 Save；保存失败不关闭表单

## Select / Combobox

- 与输入框同高；chevron 16px 随状态旋转
- 选项行高 28–32；选中项用勾选 + 轻背景，不要整块 accent
- 键盘：↑/↓ 移动，Enter 选中，Esc 关闭，输入过滤
- 打开时焦点留在触发器，列表可滚动；关闭后焦点回到触发器

## Tabs / Segmented

- 高度 28–32；容器 1px 边框或底部一条线，不堆叠 pill
- 选中用 surface 提升 + 文字加深；不要高饱和填充
- 标签 2–5 个；超过则用 select 或可滚动 tabs
- 键盘 Left/Right 切换，Home/End 首尾

## 表格 / ResultGrid

- 表头 11px muted，行高 30–32px；数字右对齐并 tabular
- 行分隔用极轻 1px 线，不用全网格；避免斑马纹 + 边框 + hover 三重叠加
- hover 只高亮行背景；选中用左侧 2px accent 或轻背景
- 表头 sticky；滚动时列宽稳定
- 数字单元格：不因位数变化跳动；错误/缺失用非颜色标记
- 大表：批量粘贴先预览，不直接覆盖；显示将覆盖数量和无效行
- 编辑：单格修改进入 draft；显式保存；未保存数量可见
- 保存状态：Draft → Submitting → Saved rev X / Conflict / Error；绝不提前显示已同步

## 96 孔板

- 孔直径 18–20px；间距 4px；坐标 A–H/1–12 用 10px mono
- 默认低饱和 surface；分组用低饱和 tint；选中用 2px ring，不填满高饱和色
- 悬停显示孔位、样本、组别、重复、体积；不要只靠 tooltip 承载关键信息
- 矩形拖选、Shift 扩展、方向键移动、Space 切换选中
- 空白孔与未分配孔不同视觉；无效值单独标记

## Inspector / 属性面板

- 标签 11px muted，值 13px；宽屏两列，窄屏单列
- 每行 28–32px；字段间距 12–16；组间距 24
- 分组标题用 12–13px，不要每个字段一个 card
- 选中对象时 Inspector 内容切换；未选中时显示清楚空状态
- 编辑后显示 dirty；保存/取消与全局状态一致

## 浮层 / 命令面板 / 菜单

- 只在浮层使用阴影；面板本体依靠 surface 分层
- 宽度 320–520；最大高度 60vh；内部滚动
- 打开后焦点进入首个可操作项；Esc 关闭；点击外部关闭
- 命令面板显示快捷键、分组、空结果；不要遮挡当前焦点
- 菜单项 28–32 高；危险项分隔并右对齐

## Tooltip

- 仅解释图标/截断文本；不承载唯一关键信息
- 悬停 300–500ms 后出现；可 Esc 关闭；可移动到 tooltip 不失活
- 11–12px，短句，不换行过多

## Toast / 状态条

- 成功：3–5s 自动消失；错误：持续到用户处理
- 位置固定但不遮挡主操作；同屏最多一个 toast
- 数据保存优先使用**内联状态**（字段/页头），不要只靠 toast
- 错误文案说明发生了什么、保留了什么、下一步怎么做

## 拖拽 / 画布

- 拖动开始给 1px accent 轮廓；拖动中显示落点/连接目标
- 拖拽不改变文本布局；松开前可 Esc 取消
- 连接端口有明确 hit area（≥8px）；非法连接显示禁止态
- Undo 覆盖移动/连接/删除；一次拖动只产生一个 undo 记录
- 画布 pan/zoom 不抢占输入框滚动；空格+拖动可平移

## 动画

- 120–180ms ease-out；数据表格/保存状态不要弹跳
- 只动画 opacity/位置/强调；不改字号和布局
- 减少动态偏好下关闭非必要动画
- 加载骨架保持最终尺寸，避免布局跳动

## 文案

- 用用户语言，不显示 Core 字段名、表名、异常堆栈
- 句子短、动词开头；单位明确
- “已保存 rev 5 · 14:32”优于“同步成功”
- 不把示例数据当默认规则；示例标注 `[示例]`
- 空状态给下一步，不只写“暂无数据”

## 参考

- W3C WCAG 2.2 Focus Visible / Focus Not Obscured / Focus Appearance
- NN/g, *Button States: Communicate Interaction*
- NN/g, *Response Time Limits: 0.1s, 1s, 10s*
- Material Design 3, *States* / *Buttons*
- Apple HIG, *Buttons* / *Text fields* / *Focus and selection*
- Cloudscape, *Inline edit* / *User feedback*
- Primer, *Saving*
- Refactoring UI, layout/spacing/hierarchy notes
