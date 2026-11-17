# QA-HUVEC-01 证据包（HUVEC 成管全链路 + RT-qPCR）

-  checkout: `O:/qahuvec`（ovnfs），HEAD `6071ece`（= 本轮合并后的 origin/main；`a31e04c → 6071ece` 增量仅 CI yml + QA-10 README，无产品代码，见下）
-  数据：`D:/.ci-cache/qa-huvec-01/`（HUVEC run `c2431573…` rev 8 进行中；RT-qPCR run `1b0f363a…` **已完成 13/13**）；资产 `D:/.ci-cache/qa-huvec-01-assets/`（162 件）
-  应用：`D:/.ci-cache/qa-huvec-01/bin/mygo-run.exe`（SHA256 `c4eb876b…`，由 `a31e04c` 构建；合并增量不含产品代码故二进制仍具代表性）；CLI `D:/.ci-cache/cargo-target/debug/labworkbench.exe`
-  窗口：真实 `AsterLab` 窗口 + `mygo-automation` 驱动；截图名中 `1360x880 / 900x600` 为 DIP 尺寸（实际像素 2018×1308 / 1328×888，与 UI-RUN/UI-REC 等包惯例一致）

## 本轮完成（接前人 65% 继续）

1. **合并 origin/main**：`a31e04c → 6071ece`（fast-forward）。`git diff a31e04c..HEAD --stat` 仅 3 个文件（2 个 workflow + QA-10 README），无产品代码变更。
2. **RT-qPCR 全路径（CLI + 真实 GUI）**
   - CLI `readout.submit` 8 条 Nanodrop 读数（QC 任务 `root/qc/qc`，protocol_version `3cf407eb…`）：RNA-S01 342.5/1.98、S02 318.0/1.96、**S03 96.4/1.52（污染）**、S04 365.2/1.99。
   - CLI `calculator.sample_normalize_batch`（target 1000 ng / 20 µL）：`cli-calculator-normalize.json`（S01 2.92µL / S02 3.14µL / S03 10.37µL / S04 2.74µL + 稀释液补足，insufficient 0）。
   - GUI `QC 剔除` 排除 RNA-S03（原因随事件写入）→ `rtqpcr-qc-exclusion-1360x880-light.png`（8 条读数实时渲染 + “QC 剔除已记录”）。
   - GUI 逐任务 `标记完成`：提取 10/10 → 质控 1/1 → 逆转录 → qPCR，**Run rev 22 `completed`，4/4 步已完成** → `rtqpcr-run-complete-1360x880-light.png`。CLI 交叉验证见 `cli-run-workspace-rtqpcr.txt`（22 events：8 measurement + 1 剔除 + 13 完成）。
   - 结论：污染样本排除**不阻塞**任务完成与整单完成。
3. **截图**：HUVEC run-bench（`run-bench-…-light.png`）、设计步骤（`design-steps-…-light.png`）、 Notebook 图板 FOV1/FOV2（沿用）、协议归档预览 RT-qPCR（`protocol-archive-…-light.png`，13/13 completed）与 **HUVEC 已签核归档**（`protocol-archive-signed-…-light.png`：签核 Lin + hash `f5d984…`，任务完成 5/11）。
   深色：切 OS 深色 + relaunch（应用跟随系统主题，应用内无开关；切回浅色已恢复）→ `run-bench/design-steps/notebook-board-…-dark.png`，计时器重启恢复再次验证（1:35:41…继续走）。
   重命名：`run-timer-running-1360x880-light.png → run-timer-running-900x600-light.png`（1328×888，符合既有 `*900x600*` 惯例）。
4. **CLI 交叉读数**：`cli-run-workspace-huvec.txt`（11 tasks：skip/completed 状态 + 8 events）、`cli-run-workspace-rtqpcr.txt`、`cli-calculator-normalize.json`、`cli-run-list.txt`、`cli-pantry-list.txt`、`BUILD_INFO.txt`。
5. **前人已证（本轮复核信任，不重做）**：构建+BUILD_INFO、种子、162 关联、计时器+重启恢复、跳过、t0、设计联动回环、Notebook 图板+签核；非阻塞发现 2 项（同模式第 2 秒表静默 no-op；bench 采图条缺失——见下 F1/F2）。

## 发现（均未修；F3 已由 coordinator 在安静盒复测关闭）

- **F3（已关闭，非产品缺陷）—— 导出 Save 对话框“卡死”为并行窗口 churn 误判**：coordinator 在安静盒复测（同一数据目录、同版二进制）：按 `导出归档包` 后标题为“导出归档包”的原生保存对话框**正常弹出**（进程第 2 可见窗），automation 保持响应，Esc 关闭后应用完全恢复。所谓“卡死”（计时器冻结、CPU 归零）正是模态对话框打开时的**正常**表现；agent 侧“无可见对话框”是因为当时 coordinator 的 UX-07 Gate 截图窗在抢前台（“单 lane”判断有误，我的窗口与之重叠时段一致），对话框被遮挡且 automation 驱不动原生对话框。文件落盘点击链按项目惯例标**未验证**（automation 无法驱动原生对话框）；PDF 图片嵌入已由 UI-REC-PDF Gate 的落盘链证明覆盖（真 PNG＋TIFF → 2 个放置操作），此处不重复。
- **F1（沿用，非阻塞）**：同一任务第 2 个同模式秒表创建静默 no-op（以观察记录诚实计时 274s 单次曝光为准）。建议 follow-up 卡。
- **F2（沿用，非阻塞）**：bench 采图进度条缺失（`designPlateWells` 只读 Design 页 mygo 板实例，不读冻结 AsterDoc 板）。矩阵/历史对账为有效证明。建议 follow-up 卡。
- 观察（非缺陷）：终态归档预览跟随**当前加载 Run**，不跟随版本选择器（HUVEC 页切 v1 时曾短暂显示 RT-qPCR 归档——实为 loaded-run 上下文，见截图时间线）；深色截图侧栏图标呈白块（抓屏 artifact 可能性大，导航操作不受影响）。

## 命令（复现）

```bat
set CARGO_TARGET_DIR=D:/.ci-cache/cargo-target & set GOCACHE=D:/.ci-cache/go & set TEMP=D:/.ci-cache/tmp & set TMP=D:/.ci-cache/tmp
set LABWORKBENCH_DATA_DIR=D:/.ci-cache/qa-huvec-01
D:/.ci-cache/cargo-target/debug/labworkbench.exe readout.submit --input ro.json
D:/.ci-cache/cargo-target/debug/labworkbench.exe calculator.sample_normalize_batch --input norm.json
D:/.ci-cache/cargo-target/debug/labworkbench.exe run.workspace --input q.json
set MYGO_AUTOMATION=D:/.ci-cache/qa-huvec-01/gui.sock & mygo-automation open & mygo-automation snapshot
```

## 结论

全绿。三个发现均为非阻塞（F3 已证伪，F1/F2 建议 follow-up 卡）。本卡可合。
