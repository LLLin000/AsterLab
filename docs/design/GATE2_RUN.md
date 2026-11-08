# Gate 2 — Run Vertical Slice

- 状态：**实现完成，待主人验收保存/重启闭环**。
- 日期：2026-11-08
- 代码：`native/asterlab-run/`
- 正式视觉：`rune-gate2-titlebar.png`
- 自定义 titlebar：无 Windows 原生标题栏，40pt 拖动带 + Rune 绘制 caption 按钮
- 窄窗：`rune-gate2-900.png`
- 150% / 200%：`rune-gate2-150.png`, `rune-gate2-200.png`
- 深色：`rune-gate2-dark.png`

## 已交付

- Rune + **rune-kit** 组件（`Button`, `TextInput`, `Card`, `Sidebar`, `SidebarRow`, `SegmentedControl`, `IconButton`）；不再手绘按钮/卡片。
- 左侧图标 rail + 项目/设计侧栏：
  - 项目是文件夹；
  - 设计是文件行；
  - 行上显示执行次数；
  - 点击设计切换到它的 Run。
- 主区：执行安排 / 结果 segmented control、时间点 strip、当前任务 Card、检查 SegmentedControl、测量输入、保存/完成、就绪 Card。
- Run 读写走现有 `labworkbench.exe` CLI，不直接打开 SQLite。
- Rust 应用层新增 blocking-check 完成校验与完成事件幂等守卫（`src/cli.rs` + `tests/execution-acceptance.cjs`）。
- 字体：Rune `TextStyle` 20/16/14.5/12.5；families `Inter, Segoe UI, Microsoft YaHei`。
- Release：`target/rune-spike/target/release/asterlab-run.exe`，26,232,320 bytes；无参数窗口启动正常。

## 验收状态

| 项 | 状态 |
|---|---|
| 真实 Run 读取 | 通过 |
| 项目/设计侧栏 | 通过 |
| 鼠标点击 tab/设计行 | 通过（rune-kit 控件） |
| 中文显示/换行 | 通过 |
| 100%/150%/200% | 截图通过 |
| 窄窗 900 | 通过，卡片单列 |
| 保存读数 | **通过**：debug 填充 `Control-1/0.82` 后保存，CLI 回读成功；正常 UI 需填写样本 |
| 关闭重启回看 | 数据已写入；需主人用真实键盘完整走一次 |
| 安装包/签名 | 未做 |

## 已知限制

- `SidebarRow` 对长设计名使用截断，完整名需要 hover/检查器（当前窗口无 tooltip）。
- 右侧“就绪信息”在数据缺失时显示“未绑定”，不伪造内容。
- 保存失败的根因之一是样本编号留空；现在会明确提示 `测量样本编号不能为空`。
- 当前自动化环境的 `computer.type` 对自绘 TextInput 不能稳定注入文本；鼠标点击、rune-kit 控件和保存命令已验证。
- 不包含其他页面迁移。
