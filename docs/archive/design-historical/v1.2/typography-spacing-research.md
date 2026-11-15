# AsterLab 高级感基础：字、间距、排版与层级

- 日期：2026-11-10
- 目的：为 v1.3 / AsterLab Studio 视觉方向提供可执行规则，不再靠“多留白、换配色”猜高级感。
- 结论：高级感来自 **受控的字号层级 + 少而有节奏的间距 + 清楚的弱化关系 + 极轻的边界**，不是装饰、阴影或大面积空白。

## 一、先定四条硬规则

### 1. 每屏只保留 3 个文字层级

同时使用字号、字重、颜色和空白，而不是只靠字号。

- **Primary**：页面/对象标题，20–24px，600–650
- **Secondary**：区块标题、当前值，13–15px，600
- **Tertiary**：说明、单位、坐标、元数据，11–12px，400–500，降低对比

标签必须弱于值。数据界面优先让数值和名称先被看到，单位/说明后退。

依据：Refactoring UI 的“先弱化次要信息，再强调主要信息”，以及 Material 3 的 role-based type scale（display/headline/title/body/label）。

### 2. 行高按用途分，不要全站一个值

- 标题：`1.15–1.25`
- 正文/说明：`1.45–1.6`
- 表格/工具栏：`1.25–1.35`
- 中文正文比拉丁文更需要行高，桌面实验记录正文取 `1.5–1.6`
- 大字号不要用大行高；小字号不要用紧行高

依据：Butterick 的 line spacing 推荐 120%–145%；Material 3 对大标题建议约 1.2，小正文约 1.5。

### 3. 行长必须限制

- 中文正文：每行约 **25–35 个汉字**
- 英文/混合正文：**45–75 字符**（宽松上限 90）
- 说明文字最大宽度 **640–720px**，超过就分栏或缩短
- Inspector 文案不要铺满整列；标签短、值靠右或独立行

依据：Butterick 的 45–90 characters；Refactoring UI 的 45–75 characters。

### 4. 间距是节奏，不是填空

采用 **4px 基础、8px 主节奏**：

```
4   icon ↔ label / 紧凑内联
8   label ↔ value / 行内分组
12  控件内部 / 紧凑卡片
16  卡片 padding / 一组字段
24  页面边距 / 区块之间
32  页面主区块
48  页面级留白
```

- 组件内边距不要用 10/13/17 这类无规律数字
- 同一层级的区块间距一致；不要把“空白”当作高级感
- 小屏优先减少 24/32，不要压缩 8/12 到 4
- 表格行高 30–34px；工具条 36–40px；主按钮 32–36px

依据：Design Systems 的 8pt spatial system 与 4pt/8pt increment；Refactoring UI 的“一致 spacing system 减少决策疲劳”。

## 二、字体的高级感来自哪里

### 字体数量

- 最多 **两套**：系统 UI sans + 一个等宽数字/代码字体
- 中文 fallback 明确：`PingFang SC` / `Microsoft YaHei UI` / `Noto Sans SC`
- 不要为了“设计感”混用三套字体
- 数字、孔位、浓度、Cq、OD 值用 **tabular numerals**，避免表格跳动

依据：Material 3 明确建议表格/频繁变化数字使用 tabular figures；Adrian Manolache 的 UI typography 指南建议最多两套字体。

### 字号/字重

建议桌面 UI 初始阶梯：

| 角色 | 大小 | 行高 | 字重 |
| --- | --- | --- | --- |
| Page title | 20–22 | 1.25 | 650 |
| Section title | 14–15 | 1.35 | 600 |
| Body/value | 13 | 1.5 | 400–500 |
| Meta/label | 11–12 | 1.35 | 450–500 |
| Data/ID | 12 | 1.3 | Mono, 400 |

不要全屏用 11px。11px 只用于元数据；主要内容至少 13px；标题与正文之间要有明显差。

### 字距

- 中文默认 `letter-spacing: 0`
- 大号标题可轻微收紧（约 `-0.01em`），不要负到粘连
- 全大写/等宽 ID 可少量增加 tracking
- 不要用 `letter-spacing` 修复字体本身的问题

依据：AnatomyUI 的 tracking/leading 指南与“随字号增大而收紧”的规则。

## 三、层级与“高级”的关系

### 先弱化，再强调

- 标签颜色比正文低一档
- 单位/说明用 muted，不要和值同色同重
- 次要动作使用 ghost/quiet 按钮，不要每个动作都填充色
- 每个视图只保留一个主操作（Primary）
- 选中状态用细边框/低饱和底色，不要用大面积高饱和填充

依据：Refactoring UI 的 “emphasize by de-emphasizing”；Material 3 的 baseline/emphasized 权重分层。

### 减少边框，增加留白与对齐

- 容器优先用背景层次和 1px 极轻分隔线
- 不要每张卡都描边 + 阴影 + 圆角
- 阴影只给浮层/命令面板/对话；正文卡片用平面
- 对齐基线优先于“看起来居中”
- 同一行混用字号时，数值与标签按基线对齐

依据：Refactoring UI 建议用深度/阴影分层而不是滥用边框；Linear/Things 的低噪声界面同理。

### 高级 ≠ 空

高级感通常是：
- 信息密度足够
- 视觉噪音极少
- 对齐精确
- 状态反馈克制但清楚

大块无意义留白会让页面显得未完成，而不是高级。

## 四、AsterLab 落地规则

### 主界面

- 页面标题 20px / 600；正文 13px / 1.5；元数据 11px，muted
- 页面 padding 24px；窄屏 12px
- 工具条 40px；导航 48–56px；表格行 32px
- 主按钮 32–36px；图标按钮 28–32px
- 面板间距 16px；区块间距 24px；不要所有区域都加 Card

### Designer

- 节点标题 13px/600；参数摘要 11px muted；端口 6–8px
- 选中节点：1px accent + 柔和光晕，不用整块 accent 填充
- 画布工具栏是浮层，不占用顶部一整排按钮
- Inspector 标签 11px muted，值 13px；数值/单位用 tabular
- 96 孔坐标 10–11px mono；孔圆 18–20px

### Protocol

- 正文最大宽度 680px
- 步骤标题 15px/600；操作正文 13px/1.6
- 参数值用 13px tabular；单位 11px muted
- 章节间距 24–32px；步骤间距 16px
- 不要显示内部 Core 字段说明

### Run

- 表格行 30–32px；数字 tabular
- 顶部状态 11–12px；保存状态用低饱和 badge
- 50/50 分栏只在 ≥1100px；900px 切换单列 tab
- 主操作固定、禁用态明确；错误用文字 + 语义色，不只靠颜色

### Tools

- 计算器输入 13px tabular；结果 16–18px/600
- 96 孔板 18–20px 圆，不只依赖 96 个按钮
- 工具详情用列表/工作面，不做等权大卡片网格

## 五、视觉验收：四层

1. **结构**：区域位置、尺寸、间距、滚动、900×600 切换
2. **视觉**：字号层级、行高、字重、色彩、图标、留白、组件一致性
3. **交互**：真实点击、拖动、键盘、focus、loading/error/saved
4. **业务**：真实 Core、SQLite、保存、重开、数据来源

不要用“与设计稿像素差 <0.5%”作为跨渲染引擎标准。

## 参考

- Butterick, *Practical Typography — Line spacing*: https://practicaltypography.com/line-spacing.html
- Butterick, *Line length*: https://practicaltypography.com/line-length.html
- Material Design 3, *Typography*: https://m3.material.io/styles/typography/overview
- Material Design 3, *Type scale tokens*: https://m3.material.io/styles/typography/type-scale-tokens
- Design Systems, *Spacing, grids, and layouts*: https://www.designsystems.com/space-grids-and-layouts/
- Refactoring UI (Wathan/Schoger): https://refactoringui.com/
- Refactoring UI notes: https://www.joelsleppy.com/blog/notes-on-refactoring-ui/
- AnatomyUI, *Tracking and leading*: https://www.anatomyui.com/foundations/typography/tracking-leading

资料核对日期：2026-11-10。上述规则是 AsterLab 的候选落地规范，不是自动等同于用户测试结论。
