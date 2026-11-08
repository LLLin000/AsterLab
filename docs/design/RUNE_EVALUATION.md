# Rune Technical Feasibility Report

- 日期：2026-11-08
- 结论：**技术可行性尚未验证完毕；既不是失败，也还不能批准迁移。**
- 固定 revision：`a7f276c3630f85a06a0230295da4d2bd172d8d3b`（`anomalyco/rune` `dev`）
- 许可：MIT（仓库声明）
- 状态：Rune 官方仍是 pre-release；pin revision，不追浮动分支。
- 重要区分：**未测 ≠ 失败**。本报告把已有证据、未测项和失败项分开。

## Current verdict

| 能力 | 证据 | 判定 |
|---|---|---|
| Windows 编译/运行 | `cargo test -p hello` 通过 | 通过 |
| GPU 原生渲染 | headless 渲染 + 150% scale | 通过 |
| 中文短标签/长段落 | 显式 `.wrap()` 后可换行 | 通过（有条件） |
| CJK 分段错误 | 通过 spike crate 直接启用 Parley `complex-scripts`，`cargo tree` 确认；零 ICU 报错 | 可复现的 feature unification |
| Headless PNG / 元素树 | `--snap`、`--tree` | 通过 |
| 脚本点击/输入 | 100 行表格中编辑单元格；Tab 到下一单元 | 通过 |
| 真实冻结 Run | workspace JSON → Rune 只读 | 通过；11 任务 / 10 测量 |
| 100 行可编辑表格 | 100 个 TextEdit，滚动容器；点击/中文输入 | 初测通过；未做粘贴/虚拟化 |
| 中文 IME | 未安装中文输入法，未测 | **未证明**；不是失败 |
| Release 构建 | 12m56s（干净 target）；25.8 MB；正常窗口打开/重启 | 通过 |
| 真实数据库接入 | 未做 | **未开始** |

## What ran

| 项目 | 命令 / 环境 | 结果 |
|---|---|---|
| Rune 构建 | `cargo test -p hello` in pinned clone | 通过；`hello` 1/1 |
| Spike 编译 | `cargo check -p aster-spike` | 通过 |
| Headless PNG | `--snap ... --scale 2 --theme light` | 通过 |
| Accessibility tree | `--tree ...json` | 通过；中文 label、text-input、button、100 个 cell |
| 150% scale | `--scale 1.5 --theme dark` | 清晰；长中文可换行 |
| Scripted input | 点击第 4 行单元格并键入 `B2` | 单元格从 `A4` 变为 `A4B2` |
| 100 行表格 | 100 个 TextEdit，scroll viewport | 528 layout nodes；脚本 126 帧，first 83ms，p50 2.3ms；可编辑 |
| 中文 IME | — | **未测**：系统没有安装中文输入法 |
| 真实冻结 Run | 1c0c8989… workspace JSON | 11 任务、10 测量、时间点/状态正确显示 |
| 普通窗口 | release exe 无参数启动两次 | 窗口标题正常、响应、关闭/重启 |
| Release 构建 | 干净 target（无手工改 Rune） | **通过**：12m56s；25,764,352 bytes |

Spike 源码：[rune-spike/](rune-spike/)（临时 clone：`target/rune-spike/examples/aster-spike/`）。

证据：

- `docs/design/rune-spike-cjk-fixed.png`
- `docs/design/rune-spike-input.png`
- `docs/design/rune-spike-150.png`
- `docs/design/rune-spike-fixed-tree.json`
- `docs/design/rune-spike-input-tree.json`

## CJK finding and fix

原始现象：

```text
ICU4X data error: No segmentation model for complex script: Chinese/Japanese
```

Rune 的 `parley` 依赖默认关闭：

```toml
parley = { version = "0.12", default-features = false, features = ["std"] }
```

Parley 的 `complex-scripts` feature 才会启用 CJK/Thai/Khmer/Lao/Myanmar 的字典分段。Spike **不修改 Rune clone**，而是在自己的 crate 直接依赖同一 Parley 版本：

```toml
parley = { version = "0.12", default-features = false, features = ["std", "complex-scripts"] }
```

`cargo tree -e features -p aster-spike` 已确认最终图包含：

```text
parley feature "complex-scripts"
parley feature "std"
```

结果：

- ICU4X 报错消失；
- 长中文段落正常换行；
- 100 行表格、中文标签、`mg/mL`、数字混排正常；
- 仍需每个长文本元素显式 `.wrap()`；Rune 默认不自动换行。

**判断：不是框架不可用，是依赖 feature 配置问题；通过 Cargo feature unification 可复现，无需手工改 clone。**  
仍需在 AsterLab 的 `asterlab-ui` crate 中固化这项依赖，避免只存在于 spike。

## 100-row editable table

- 100 个 `TextEdit` 行放在一个 220pt 的 scroll viewport。
- 元素树有 `cell-0` 到 `cell-99`。
- 脚本点击 `cell-3` 并输入 `B2`，值从 `A4` 变为 `A4B2`，证明字段获得焦点、接收键盘输入、回写状态。
- 126 帧脚本运行：first frame 83ms（含初始化），p50 2.3ms、p95 5.9ms；528 layout nodes。
- 这是**非虚拟化**的 100 行；只证明 100 行可用，不证明 1,000+ 行。
- 未验证：滚动到第 100 行后的焦点、粘贴、多选、撤销。

## Chinese IME

**未测。** 当前 Windows 没有安装中文输入法，Headless 也不能替代候选词/组合输入。

这不等于失败；它是下一轮必须由真实窗口完成的人工/自动化测试：

1. 打开真实 Rune 窗口。
2. 聚焦 `孔位` 或表格单元。
3. 切换微软拼音。
4. 输入 `ni hao`、选择候选词、退格、移动光标、失焦。
5. 截图/录屏并记录候选窗位置和焦点。

## Release build

命令：

```bash
CARGO_TARGET_DIR=C:/rune-release-target cargo build --release -p aster-spike
```

首次 600 秒超时没有产物，但这只是超时，不是编译错误。干净 target（Rune clone 未手工改，只通过 spike crate 的 Parley 直接依赖）完成：

```text
Finished `release` profile [optimized] target(s) in 12m 56s
aster-spike.exe: 25,764,352 bytes (~24.6 MiB)
SHA-256: 37046743B8B90B1980F9ED91E52F8404BE7B30F77472DBD1546C5F5D9E0D7F97
```

Headless snap 启动（进程 + GPU + 渲染，不是普通窗口启动）：

- cold: 3.48 s（第一次构建后）
- warm: 1.42 s
- clean release 直接运行：无 ICU 分段错误

普通窗口：release EXE 无参数启动两次，窗口标题 `AsterLab Rune Spike`、响应正常，关闭后可重启。安装包、签名仍未验证。

## Gate status

| Gate 1 项目 | 状态 |
|---|---|
| Windows 中文 IME | 未测 |
| CJK 字体/换行 | 通过，需本地 feature patch + `.wrap()` |
| 可编辑表格 | 100 行初测通过 |
| 100+ 行滚动 | 部分通过（100 行） |
| 高 DPI | 通过 1.5x |
| 键盘焦点 | 部分通过（单元格点击、Tab 到下一单元、输入） |
| Headless | 通过 |
| 打包 | 通过（release executable；安装包/签名后续） |
| 真实数据 | 未测 |

**Gate 1 尚未完成，不能批准 Run 切片。**  
但已不再是“框架做不了”；更准确的结论是：

> Rune 的基础 UI 能力已得到初步验证；CJK 分段问题有明确修复路径；剩余关键未知项是真实 Windows IME、完整键盘路径、安装包/签名和真实数据接入。

## Decision

- 不立即放弃 Rune。
- 不启动 AsterLab Run 迁移。
- Web Beta 继续作为可用默认和回退。
- 完成本轮剩余验证后再决定是否进入 Run 单屏。

## Next bounded checks

1. 真实 Windows IME（人工/自动）测试。
2. 100 行表格的滚动、粘贴、焦点、错误态。
3. Release 构建耗时、产物大小、启动时间。
4. Rune 读取冻结 Run fixture；不改数据库。
5. 若以上通过，再启动 Run 单屏；否则评估 Slint/WebView2。

No UI migration is approved by this report.
