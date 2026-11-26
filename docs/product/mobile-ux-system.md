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

## 6. 多设备 SafeArea 契约与真全面屏（Edge-to-Edge）

- 遵循 MyGo 单次 `SafeArea` 原则：主窗口根部由 `padInsets` 处理系统安全区，仅 `ui.Overlay` 全屏浮层/抽屉读取 1 次 `c.SafeArea()`。详情与设备矩阵见 `mobile_overflow_audit_test.go`。
- **真全面屏流淌**：主滚动舞台不得用硬 `.Clip()` 截断 `extendUnderBottom` 对底部弧形 Home Indicator 区域的自然延伸，同时 `c.Root().Background(tok.Canvas)` 与宿主背景必须同步当前画布色，消除屏幕底部弯曲区露出的纯黑 `#000000` 边。

## 7. 移动端统一物理动效体系（Mobile Motion System）

基于 Apple WWDC *Designing Fluid Interfaces*、Material Design 3 Expressive（Spatial vs Effects 双轨）与现代触控交互工程（Emil Kowalski / Rauno Freiberg）：

1. **Spatial / Effects 双轨分离**：空间位移动画（抽屉、页面推入、Sheet、手风琴高度）统一使用弹簧物理；光学渐变（暗幕 `alpha`、颜色、透明淡入）统一使用纯无回弹 `EaseOut`（`NonMotion: true`）。
2. **手势 1:1 跟手与速度继承**：抽屉与右滑返回从手指接触第 1 毫米起直接驱动目标面板本身（正文固定不偏移回弹）；松手瞬间将甩动速度注入 `Draggable` 弹簧，开合手感完全对称且随时可反向打断。
3. **非阻塞式利落退场**：入场给足空间方向感（`260–340ms`），退场干脆利落（`140–220ms`）；触发关闭时遮罩立即切至 `PassThrough()`，绝不锁死后续点击。
4. **高频零阻力**：打字、切换光标与微缩按压不做拖泥带水的过渡；系统开启 `ReduceMotion` 时所有空间位自动退化为瞬时切换。

| Token | 类型 | MyGo 物理参数 | 使用范围 |
|---|---|---|---|
| `MotionTouchPress` | Micro Spatial | `90ms EaseOut` | 按钮/胶囊/列表行 `:active 0.97` 按压微缩 |
| `MotionEffect` | Pure Effects | `180ms EaseOut (NonMotion)` | 暗幕遮罩透明度、图标/激活色彩交叉渐变 |
| `MotionInline` | Compact Spatial | `Spring(260ms, bounce: 0)` | 批注手风琴折叠展开、导航胶囊 ↔ 键盘工具栏过渡 |
| `MotionDrawer` | Gesture Spatial | `Spring(300ms, bounce: 0)` | 左右全高抽屉滑入/滑出（开合双侧一致） |
| `MotionPagePush` | Navigation Spatial | `Spring(330ms, bounce: 0)` | 全屏二级页面 Push / 右滑 Pop 退回 |
| `MotionSheet` | Overlay Spatial | `Spring(340ms, bounce: 0.04)` | 底部功能 Sheet、拍照归宿 Sheet、AI 预览 Sheet、标签概览缩放 |

### 七类标准场景编排 & 底部双栏 Obsidian 规则
- **底部双栏 Obsidian 逻辑**：软键盘未弹出时底部**始终呈现导航胶囊**（含 `ModeEdit` 浏览态）；软键盘弹出进入实际输入态时平滑交叉过渡（`MotionInline`）为**键盘编辑工具栏**（图标一律中性色 `tok.Text`，仅激活状态亮 `tok.Accent`，隐藏水平滚动条避免残影）。
- **AI 转结构预览卡后续迭代纪律（Tap-to-Edit Parameter Chip）**：未来在 `✨` 预览卡补充参数微调时，严禁在原胶囊坐标下就地弹出局部浮窗（会被移动端软键盘直接遮挡），**必须点按参数胶囊后复用第 5 节的「键盘顶栏悬浮输入卡（严格锚定 `safeBottom + Spacing8`）」**，既保持预览卡默认极简零冗余控件，又彻底规避输入法遮挡。
