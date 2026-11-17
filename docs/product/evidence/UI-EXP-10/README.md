# UI-EXP-10 证据：盲法视图开关 · 1D 容器 · 样本排除与分类/表格读数

卡片：Kandev `8011cd3-9057-4002-ac90-60508408856d` · Windows Gate lane ·
基线 `origin/main 75541e2`（含 BE-10 #163 与 CORE-01 #170）。

GUI 消费 BE-10 Bridge v2 通道：`run.workspace(view=blind|unblinded)` 读经
`runs.QueryProjection`，样本排除经 `ExecuteCommand{kind: execution_event.create}`，
分类/表格读数经 `ExecuteCommand{kind: readout.submit}`。Go 不做盲法脱敏逻辑，
只消费 Core 投影（纯展示遮罩）。

## 变更清单（全部在卡片 scope 内）

| 文件 | 变更 |
|---|---|
| `internal/runworkspace/channel.go`（新） | Bridge v2 共享缝：kind 常量、每动作一键、`channelError`（UX-P5/STATE） |
| `internal/runworkspace/blind.go`（新） | `toggleBlind`：进入盲法经 Core 确认，离开本地恢复；`blindQueryParams` 精确 CLI JSON |
| `internal/runworkspace/view.go` | `Callbacks` += `QueryProjection`/`ExecuteCommand`；`State` += `BlindErr`；Run 顶栏设盲开关 + 遮罩提示 + 失败横幅（原因+重试） |
| `internal/runworkspace/bench.go` | 盲法开关走同一 `toggleBlind`；bench 接入排除与分类/表格录入区 |
| `internal/runworkspace/recording.go` | `RecordingView.Blind`；每帧同步 `Grid.Blind`；表格行同样遮罩分组 |
| `internal/runworkspace/exclusion.go`（新） | `qc_dropped`/`censored_dead`/`rework` 三态 + 样本 ID 解析 + `execution_event.create` 打包 + 录入区 |
| `internal/runworkspace/readout.go`（新） | categorical/table `readout.submit` 打包（含可选 `readout_schemas`）+ 录入区（选项 chips、表格 CSV） |
| `internal/uikit/plate/rack.go`（新） | 1D 管架/动物编号条：Lane<N> 坐标、共享 2D 状态色、盲法遮罩 |
| `internal/resultgrid/plate.go` | `plateLabel` 1D → `N 管架`；1D 网格走管架条 + 同一编辑器 |
| 各 `*_test.go` | 12 个新测试（见下） |

`sibling` UiRec10 路径（`asterdoc/protocol/protocol_page`）未动；
`modules/runs`、`crates`、`src` 未动；无新依赖。

## 验证（O:/uiexp10 精确命令与结果）

```sh
cd prototypes/mygo-run
export CARGO_TARGET_DIR=D:/.ci-cache/cargo-target GOCACHE=D:/.ci-cache/go TEMP=D:/.ci-cache/tmp TMP=D:/.ci-cache/tmp
mygo native check ./modules/runs
# ○ Building Runs for windows/amd64 … ✓ Built Runs for windows/amd64 2m 18s
# • Native module ./modules/runs is fine

go vet ./internal/runworkspace/... ./internal/resultgrid/... ./internal/uikit/...
# （无输出，exit 0）

gofmt -l <12 个新建/触及文件>
# （无输出；仓库另有既有非 gofmt 文件如 model.go/grid.go，本卡未动）

go test ./internal/runworkspace/... ./internal/resultgrid/... ./internal/uikit/...
# ok  asterlab-mygo-prototype/internal/runworkspace  0.783s
# ok  asterlab-mygo-prototype/internal/resultgrid    2.524s
# ok  asterlab-mygo-prototype/internal/uikit/plate   2.420s
```

首轮曾有 1 个失败（`TestRecordingBlindMasksTableGroups`：样本 `Control-1`
与分组 `Control` 子串碰撞），已将用例分组改名 `给药组`/样本 `M3-1` 后全绿。
新测试覆盖：params 精确 JSON、开关→`run.workspace` kind/params 断言、Core
`INVALID_INPUT` 留罩+原因+重试、无回调本地遮罩、表格行遮罩、
排除 payload 全字段 pin（含 `blind_masked`、无 revision）、分类/table
payload pin（含 `columns/rows [[1,"+"]]`)、表格解析错误、rack 渲染/盲法/点选、
1D `plateLabel` 与条带渲染。

## 设计一致性（UX-DESIGN）

- **UX-P1**：排除与读数录入区各只有一个主写动作（记录排除 / 提交读数）；
  盲法开关在顶栏常驻可见。
- **UX-P2**：默认视图无裸 id——开关文案为 `○/● 设盲视图` 而非 `view=blind`；
  分组名以 `已设盲 · 分组已隐藏` 替代，Lane 坐标与样本编号保持可见；
  技术属性仍收在既有披露区。
- **UX-P3**：全部复用 theme tokens（`Surface/Border/WarningBg/DangerBg/
  Accent(.10)`）与 `components.Button`；1D 条带复用 `plate.GroupColor`，
  无新视觉语言；字号沿用 `TextMeta(12)/Body(13)`。
- **UX-P4**：1D 容器渲染为序列条带而非孔板矩阵；分类/表格读数是独立录入区，
  不复用数值网格。
- **UX-STATE / UX-P5**：`BlindErr`、排除/读数 `Err/Done` 均命名原因并提供重试；
  草稿（form）在失败时保留；`已设盲`为瞬态成功态。
- **UX-IA**：写操作只走 Run（经 Core 命令），Protocol/历史保持只读；Notebook 未动。
- **设计系统**：色板与字阶见 `docs/design/v1.2/design-system.md` §1–2；
  数值沿用 `tnum`；中英文案空格遵循 §7。

## 诚实 caveats（非 Gate 事项不伪造）

1. **无真窗截图**：本卡只做 `ui.Tester` headless 断言；1360×880 / 900×600
   真机截图是 coordinator 的串行 Windows Gate 事项。
2. **宿主接线待完成**（scope 外）：`shell`/宿主须把 `runs.QueryProjection` /
   `runs.ExecuteCommand` 传给 `Callbacks`，并把 `s.bench().Blind` 传给其构造的
   `RecordingView.Blind`；未接线时开关退化为本地遮罩（已测试）。
3. **排除未含血统字段**：`parent_sample_id`/`aliquot_from` 暂不提供录入，
   payload 保持省略（与旧版本一致），后续卡可加。
4. **盲法脱敏以 Core 为准**：UI 只做展示遮罩；`blind_masked: true` 随排除事件
   上报发生在遮罩下的写入。
