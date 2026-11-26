# AsterLab 移动端原生交互设计体系（Mobile UX System）

> 本文档定义 AsterLab iOS / Android 原生交互、手势物理、遮罩层级与输入法避让规范。**移动端不是桌面端弹窗与侧边栏的等比缩小版**；所有移动端改动必须先符合本规范与 [`design-system.md`](design-system.md)。

## 1. 滚动与触屏物理（Scroll & Touch Physics）

- **连续甩动与即时衔接（Zero Zombie Fling Tail）**：
  - `ui/touch.go` 使用 `flingStop = 45 dp/s` 截断极慢尾帧（消除约 1.2 秒“看似停住实则仍在惯性滑行”的死区）。
  - 惯性滚动中被手指按下打断时，仅抑制当次**误触点击（`swallowTap`）**；若手指随即继续滑动（超过 `touchSlop = 8 dp`），必须立即释放并响应新一轮平滑滚动，绝不吞掉整段跟手滑动。
- **全宽自由竖向滚动**：主画布禁止放置沿屏幕边缘阻断触控的竖向 `.Dragged()` 条带；页面内任意位置上下滑动都直接驱动 `ui.Scroll`。
- **滚动过程零管线重建**：浮动底栏胶囊（`mb-dock-capsule`）随滚动上/下平移与淡出时保持挂载（仅切换 `PassThrough()` / 位移 / 透明度），严禁滚动途中销毁与重建 `glass.Glass` 离屏材质管线。

## 2. 全画布对称闭环手势（Symmetric Full-Canvas Gestures）

- **全屏可触发、按方向分流**（对齐 Obsidian iOS / Bear / Craft 手势范式）：
  - `|dy| >= |dx|`：永远由内部最里层 `ui.Scroll` 处理竖向滚动。
  - `|dx| > |dy|`：优先由内层横向容器（日历 7 日条 `mobile.PageView`、储藏室温度标签 `ui.ScrollHorizontal`、宽表格/孔板横滑）消费；在主画布空白与正文卡片区域，**右滑跟手拉出左侧「实验库」抽屉 ↔ 抽屉/遮罩左滑关闭**，**左滑跟手拉出右侧「大纲与速算」抽屉 ↔ 抽屉/遮罩右滑关闭**。
- **全屏页右滑返回**：所有全屏移动端页面（两级设置页、全局搜索、命令面板、实验库切换、意见反馈、欢迎页、新建目录）支持从内容区向右拖拽返回上一级或关闭（两级设置页先由二级退回一级，再由一级关闭）。
- **可发现性兜底（Apple HIG）**：手势只做加速，绝不替代可见按钮；左上角实验库圆钮、右上角功能胶囊与底部导航栏始终可见可点。
- **原生下拉快捷动作（PullToRefresh）**：主滚动容器（日历、储藏室、实验文档）在顶部继续下拉触发 `ui.Element.PullToRefresh`，具备原生 iOS 橡皮筋 / Android 拉伸反馈，不再使用顶部局部 48dp 盲区热区。

## 3. 遮罩与层级纪律（Scrim & Elevation）

- **纯黑半透明遮罩铁律**：所有抽屉、弹窗、`SidePeek`、Sheet 的背景遮罩统一读取 `tok.Scrim`（浅色 `rgba(0,0,0,0.28)` / 深色 `rgba(0,0,0,0.56)`）或纯黑 `ui.RGBA(0, 0, 0, alpha)`。
- **严禁使用 `tok.Text.Alpha(...)` 充当遮罩**：在 Dark 模式下 `tok.Text` 为浅灰白色（`#f5f5f7`），将其用作遮罩会导致画面蒙上一层泛白雾霾，破坏层次感。

## 4. 阅读优先与键盘防误弹（Reading-First & Keyboard Hygiene）

- **移动端默认以实验/阅读防误触态（`ModeLab`）打开文档**：在未显式设置偏好时，手机/移动端打开 `.asterml` 实验文档默认进入 `ModeLab`，确保单手滑动浏览、勾选 Checklist、操作计时器与展开批注时零误触键盘；点击顶部编辑图标进入编辑态（若用户在设置中显式选择默认打开方式则遵从配置）。
- **滑动即收起键盘（`UIScrollViewKeyboardDismissModeOnDrag`）**：在竖向滚动主列表或实验文档时，若当前存在焦点且非本次滚动目标本身的多行输入框，MyGo 触屏层自动清除该输入焦点并隐藏软键盘。
- **禁止滚动过程重复重弹键盘**：iOS 宿主 `SurfaceWindow.swift` 仅在初次进入编辑态（`!wasEditing`）、切换输入控件（`another`）或用户显式再次点击（`c.show != shows`）时调用 `becomeFirstResponder()`；滚动引起的 `caret` 坐标变化绝对不得把已收起的软键盘重新拉起。

## 5. 移动端批注交互（方案 A：丝滑行内 Tufte 折纸展开 + 键盘顶栏极简输入）

- **查看态（行内 Tufte 折纸便签）**：
  - 点击正文里的 `[^N]` 上标后，直接在所在文本段正下方通过 `AnimateTo(..., theme.MotionFluid)` + 裁剪过渡平滑向下展开（以及再次点按/点「收起」丝滑向上收起）一条极简左 2px 紫线便签；零弹窗、零大块边框、不抢焦点、不遮挡正文，支持多条对照阅读。
- **编辑与新建态（键盘顶部悬浮极简输入卡）**：
  - 点击便签右侧轻文字「编辑」或工具栏 `[^]` 新建批注时，在 `ui.Overlay` 内呈现悬浮输入卡，严格贴靠于 `safeBottom + tok.Spacing8`（软键盘正上方），顶部仅保留纯文字 `取消 | 批注 N | 删除图标 / 保存`，无嵌套边框，确保输入法与批注正文零重叠。

## 6. 多设备 SafeArea 契约

- 遵循 MyGo 单次 `SafeArea` 原则：主窗口根部已由 `padInsets` 扣除系统安全区，仅 `ui.Overlay` 全屏浮层/抽屉读取 1 次 `c.SafeArea()`。详情与设备矩阵见 `mobile_overflow_audit_test.go`。
