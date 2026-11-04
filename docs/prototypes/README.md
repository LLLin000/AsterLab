# AsterLab 原生界面 Spike（P1）

状态：可运行、只读的 **Slint 技术 Spike**。不是正式迁移，也不替代现有 Web/CLI；
不作为面向实验人员的最终 UX 验收。下一轮任务流程规格见
[`p2-operator-workflow-spec.md`](./p2-operator-workflow-spec.md)。

设计口径见仓库根目录 [`DESIGN.md`](../../DESIGN.md)；P1 需求边界见
[`asterlab-native-ui-acceptance.md`](./asterlab-native-ui-acceptance.md)。

## 运行

```bash
cargo build --release --manifest-path apps/desktop/Cargo.toml

# FemtoVG（默认，GPU 2D）
apps/desktop/target/release/labworkbench-desktop.exe \
  --db docs/prototypes/fixtures/l929-demo.sqlite \
  --renderer femtovg

# 软件渲染（低内存基线）
apps/desktop/target/release/labworkbench-desktop.exe \
  --db docs/prototypes/fixtures/l929-demo.sqlite \
  --renderer software
```

- `fixtures/l929-demo.sqlite` 是合成 UI smoke 数据，不是生产实验库；可替换为任意只读副本。
- `--db` 必填。缺失、不可读或路径不存在时直接退出；不会创建或迁移数据库。
- 打开方式为 `SQLITE_OPEN_READ_ONLY`；界面选择、展开、孔位高亮都只存在于内存。
- 记录内部启动阶段：`LABWORKBENCH_TIMING=1`，例如：
  ```
  timing args: 0 ms
  timing model: 3 ms
  timing backend: 39 ms
  timing window: 373 ms
  timing state: 378 ms
  timing shown: 384 ms
  ```

## 本轮实现

- 左侧项目 / 实验设计 / 实验运行三级导航；顶部上下文栏包含 Run；中央实验结构画布；右侧上下文检查器。
- 显式 Run 选择；孔板列表只包含当前 Run 且与当前检测关联的板。切换检测不会继续显示无关板。
- 默认优先选择有孔板的检测（L929 下为 CCK-8）；无孔板时显示“该检测暂无培养板记录”。
- 画布节点：处理组、检测项目。只有当前检测在记录中明确列出的组别才绘制连线；
  `划痕成像` 没有组别记录时显示“组别未明确”，不画推断连线。
- 检查器随选择切换：检测参数、组别与计划孔位、来源/证据、执行；组别；培养板；孔位。
- 培养板：缩略 14px 节距 → 俯视 34px 节距，320ms 过渡；行 A–H、列 1–12 来自真实板尺寸；
  已映射孔为中性数据色，选中孔才有紫色描边/外环。
- 未记录/待确认/未记录映射保持低对比文本，不填默认值。

## 验证证据

截图（`docs/prototypes/assets/`）：

- `native-workspace.png`：默认状态，CCK-8 被选中，Run 与孔板上下文一致。
- `native-livedead-noplate.png`：切换到活死染色，显示“该检测暂无培养板记录”，不再残留 CCK-8 板。
- `native-plate-expanded.png`：96 孔俯视，行列坐标；A1 已映射。
- `native-well.png`：选中 A1，检查器显示 E1 / SYN-CCK8-A1 / 备注。
- `native-software.png`：软件渲染，中文与布局一致。
- `native-interaction.mp4`：交互录屏（选择检测 → 展开 → 滚动 → 选择 A1）。
- `web-baseline.png`：同一隔离数据库的 Web 基线。

交互通过 Windows UI Automation `InvokePattern` 实际执行，并断言：

- 默认检查器为 CCK-8，且存在 `CCK-8 演示板 A`；
- 选择 `检测项目 活死染色` 后出现 `活/死细胞荧光图像`、`该检测暂无培养板记录`，且不存在 CCK-8 板页签；
- 切回 `检测项目 CCK-8` 后孔板页签恢复；选择 Run 不丢失当前检测/孔板；
- 展开后出现 `收起`、轴标签 `A`…`H` / `1`…`12`；
- 选择 `孔位 A1，已记录映射` 后检查器出现 `孔位 A1`、`已记录映射`、`E1`、`SYN-CCK8-A1`。

## 性能实测

同一台机器、同一隔离数据库，2026-11-04。内存为进程工作集/私有内存；CPU 为单核百分比
（8 核机器上 4.7% ≈ 全机 0.6%）。

| 指标 | FemtoVG | 软件渲染 | Web 基线 |
|---|---:|---:|---:|
| 窗口显示（内部计时） | 384 ms | 63 ms | 未单独计时 |
| 进程数 | 1 | 1 | 1 个服务 + 9 个 Chromium 进程 |
| 工作集 | 84–97 MB | 29.4 MB | ≈ 137.7 MB（Chromium 树） |
| 私有内存 | 87–102 MB | 6.9 MB | 服务 2.7 MB（浏览器私有内存未合计） |
| 空闲 CPU（5–10 s） | 4.7% 单核 | 0% | 服务 0% |
| 最小化空闲 CPU | 0% | — | — |
| 线程 / 句柄 | 16 / ≈401 | — | 服务 19 线程 |

说明：

- UI Automation 首次发现窗口需要 3.5–5.9 s，这是自动化树构建/查找延迟，不是窗口绘制时间；
  内部 `timing shown` 才是用户可见时间。
- FemtoVG 的空闲 CPU 在启动后前 5 秒约 22%，10 秒后降到约 4.7% 单核；软件渲染为 0%。
  这仍不满足“接近 0%”的目标，是当前主要技术风险。
- 未对 60 FPS 做逐帧计数。展开/选择路径已实际走通，动画时长按设计为 320ms；
  帧率需要后续用渲染器计数或录制逐帧分析补充。
- Web 基线使用已启动的本地服务与同一 omp 管理 Chromium；浏览器进程树包含浏览器外壳进程，
  因此总量是上界参考，不是严格同口径的“单页成本”。

交互延迟（UIA 调用 → 目标状态可读，3 次）：

| 操作 | 观测值 |
|---|---:|
| 展开培养板（“展开俯视” → “收起”可读） | 3–23 ms |
| 收起培养板 | 10–39 ms |
| 选择孔位 → 检查器标题更新 | 17–36 ms |

状态提交是即时的，320ms 是纯视觉过渡；这满足“常规切换 P95 ≤ 100ms”的初始目标
（不含磁盘 I/O），但没有证明 60 FPS。

## 已知限制与风险

1. **FemtoVG 空闲开销**：私有内存约 101 MB、稳定空闲约 4.7% 单核，接近/超过 100 MB 与近 0% 目标。
   软件渲染显著更轻。若正式产品以资源预算优先，需要继续定位 FemtoVG 的重绘/同步来源。
2. **固定画布几何**：两列宽度、节点高度与 16px/12px 间距在 Rust 中写死；组别/检测很多时会依赖滚动，
   不会自动重排。P1 只验证 L929 规模。
3. **384 孔板**：当前 96 孔使用独立元素 + 动画；384 孔需要定制绘制或虚拟化，尚未实现。
4. **键盘路径**：孔位有 `FocusScope`，导航/检测目前主要通过指针或辅助功能 Invoke；完整 Tab 顺序待补。
5. **减少动态效果**：Slint 目前没有统一 API；接入平台设置前，设计上保留“直接显示终态”的位置。
6. **首屏滚动**：840px 高度下培养板下半部分在折叠线以下；需要滚动查看，这是有意保留的工作区滚动。
7. **范围外**：移动端、跨设备同步、编辑/写入、正式数据库迁移、旧 Web 退役。

## 许可

`slint = "=1.18.1"` 的 SPDX 许可为
`GPL-3.0-only OR LicenseRef-Slint-Royalty-free-2.0 OR LicenseRef-Slint-Software-3.0`。
本原型未做商业分发；正式分发前必须选择许可方案，并满足署名/商业授权条件。
关于窗口使用 Slint 自带的 `AboutSlint`，正式发布时的署名位置需按最终许可复核。

## 回归检查

```bash
cargo test --manifest-path apps/desktop/Cargo.toml
node tests/smoke.cjs
sha256sum data/labworkbench.sqlite
```

本轮结果：5 个 Rust 测试通过；Web/CLI smoke 通过（37 个 CLI 工具、移动端 390px 无横向溢出）；
默认数据库 SHA-256 保持 `6d9af98c05d6f7ee128a0ef13a2add26ceda9b66832a77fa5575ba800e4dfb3d`。
