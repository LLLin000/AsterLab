# AsterLab 实施计划

- 日期：2026-11-05
- 状态：唯一当前实施计划；研究文档中的候选方案不自动进入这里。
- 基线：`44318b4` 之后的语义/契约修订。
- 关联：[CONTEXT](../CONTEXT.md)、[架构](architecture.md)、[Core Contract v1](contracts/core-v1.md)。

## 0. 当前结论

Core 方向正确，但还不能直接拆 Slint。

先冻结：

- `DesignMap`、`ProtocolPreview`、`RunWorkspace` 查询契约；
- `OperationStep`、`StepOccurrenceKey` 身份；
- `ToolAction` / `ProtocolStep` / `InstrumentProfile` / `ExecutionEvent` 边界；
- 旧数据、旧 CLI/HTTP 兼容规则。

否则会把 `main.rs` 和 `AppWindow.slint` 的临时推导搬进新目录，形成“架构完成”的假象。

## 1. 已完成，不重复做

- `design.map` CLI + HTTP 只读投影。
- Web 只读实验地图与设计卡入口。
- Slint 设计模式实验地图首屏（组别/检测/待确认摘要）。
- `AsterLab-Slint.exe` + L929 fixture 启动脚本。
- CLI/HTTP/UI smoke 覆盖地图与 390px。
- 架构文档与领域术语初稿。

这些是原型资产，不等于 Core 已完成。

## 2. 阶段

### P0 · Core Contract Freeze（已完成）

**只做契约，不做 UI。**

交付：

- [x] `CONTEXT.md` 语义补充：Design/DesignVersion、Protocol/ProtocolVersion、StepOccurrence、ToolAction、InstrumentProfile、读模型、检查级别。
- [x] [Core Contract v1](contracts/core-v1.md)：`CurrentDesignMap`、`FrozenDesignView`、`ProtocolPreview`、`RunWorkspace`、`OperationStep`、`PlannedTask`、`StepOccurrence`、`TargetRef`。
- [x] 模块依赖方向与旧数据兼容规则。
- [x] 测试矩阵（C1–C12）。
- [x] `TargetRef` / `TargetSelection` 类型化；`Unresolved` / `NotApplicable` 分开。
- [x] `ExecutionEvent.occurrence_id` + 独立 `idempotency_key` 方向。
- [x] `CurrentDesignMap` 与 `FrozenDesignView` 分开，共享 `DesignMapContent`。
- [x] `OperationStep` / `PlannedTask` / `StepOccurrence` 分离；RunWorkspace 返回 steps/tasks/occurrences。

**禁止：**

- 不新增 Slint 页面、按钮或动画。
- 不实现 `StepMeta`。
- 不全面移动 `src/cli.rs`。
- 不修改正式 SQLite 数据。
- 不把 `ManifestItem {title, detail}` 当 Run 步骤模型。

**验收：** 接口、依赖方向、身份、兼容规则和测试用例明确；P0 契约已冻结，进入 P1-A。

### P1 · 抽共享 Core

P1-A 已完成：

- 新增 `crates/asterlab-core`，`design_map::build_current` 纯投影。
- CLI/HTTP 调用共享 Core，保持 `design.map` 扁平 JSON。
- Slint 地图摘要/待确认改用 Core 投影，不再自行判断 n/Protocol/时间关系。
- 核心测试 + CLI/HTTP 一致性 smoke 通过。

P1-B 已完成：

- `crates/asterlab-core/src/protocol_preview.rs` 定义 `ProtocolPreview`、`OperationStep`、`ResolvedParameter`、`UnresolvedParameter`。
- CLI 递归展开直接构造 typed `OperationStep`；`protocol.block.preview` 仍序列化为原扁平 JSON。
- 步骤数组顺序即执行顺序；`step_path` 保留来源路径。
- smoke 覆盖递归组合、参数、路径和未解决槽位。

P1-C 已完成（只读查询）：

- `crates/asterlab-core/src/run_workspace.rs` 定义 `RunWorkspace`、`RunScope`、`RunStep`、`PlannedTask`、`StepOccurrence`、`TargetSelection`。
- `run.workspace` CLI + `GET /api/runs/{id}/workspace`。
- 读取 Run 冻结的 `DesignVersion` 和 `protocol_manifest`，不读当前 Design。
- `steps/tasks` 分开；`tasks` 不在缺少目标时生成笛卡尔积；旧事件不伪造 occurrence。
- 测试覆盖 tasks、legacy events、CLI/HTTP 查询。

下一步 P2：通过正式 Core 命令构建 L929 可执行 Run Fixture（多步骤、两个检测/时间点、实际板图、一条实际事件），再接 Slint 执行台。

- Core 查询：`ExperimentQueries::design_map` / `protocol_preview` / `run_workspace`。
- CLI/HTTP 变成 Adapter；不复制校验。
- 桌面端停止自行解释科学关系；先消费共享读模型，不重写 UI。
- 保留旧 CLI/HTTP 输出；必要时用兼容投影。

**验收：** Slint、CLI、HTTP 对同一 design/run 得到同一语义；旧契约不退化；桌面端不再二次计算 n/覆盖/时间关系。

### P2 · 可执行 Run Fixture（已完成）

`tests/p2-fixture.cjs` 通过正式 CLI 命令构建 synthetic L929 Run，验证：

- 至少 3 个步骤；
- 参数绑定；
- 两个检测/时间点；
- 一张实际板图；
- 一条实际执行事件；
- 不涉及正式数据库。

**结果：** 6 个步骤、CCK-8 5 个任务、活死 5 个任务、1 个实际孔位、1 条 legacy 事件；修改当前设计后冻结 objective 不变；CCK-8/活死不串用。`node tests/p2-fixture.cjs` 通过。

P1-C 加固同时完成：

- 步骤反序列化失败返回错误，不再静默丢弃。
- `time_scope` 显式控制任务展开；缺少时不复制到全部 Assay 时间点。
- 冻结 DesignVersion 校验归属项目/设计，错误引用被拒绝。
- `RunScope` 保留 `protocol_ready`、已解析/未解析参数。
- `RunPlateView` 包含显式保存的实际孔位，不做计划→实际推断。

### P3 · Slint 消费 RunWorkspace（进行中）

P3-A 已完成：

- Slint 执行模式读取共享 `RunWorkspace`，显示冻结作用域、步骤、参数、任务、事件和实际孔位。
- 步骤列表可点击；当前步骤显示说明、参数、任务和事件。
- 执行模式的设计标题/研究目的改为冻结 DesignVersion，不显示当前 Design 覆盖。
- 同一 `root/seed` 步骤在 CCK-8 与活死染色中按 scope 隔离，任务/事件不串用。
- 截图：`docs/prototypes/assets/slint-run-workspace.png`。

P3-B（进行中）才拆 `AppWindow.slint`：

- [x] `ExperimentMapView.slint`
- [x] `RunStepsView.slint`
- [ ] `AppShell`
- [ ] `DesignDetailView`
- [ ] `ProtocolStepsView`
- [ ] `components/`

执行台实现 `StepList → StepDetail → Target/Plate → Record`；不再只是 Manifest 字符串列表。

### P3-C · Native Design Editing（命令切片已完成）

- 新增 `crates/asterlab-core/src/design_commands.rs`：
  - `DesignDraft` / `DesignGroupDraft` / `DesignAssayDraft`
  - `CreateDesign` / `ReviseDesign` / `DesignSaved`
  - `DesignCommands` trait
  - `validate_draft`：结构、稳定 ID、组别引用、时间点重复；缺 n/来源/剂量仍是合法草案
- 4 个 core tests 覆盖空草案、重复组 ID、未知组引用、重复时间点。
- 已新增 `apps/desktop/src/design_editor.rs`：
  - `EditorState`（NewDraft/EditingDraft/Dirty/Validating/Conflict/Saved/Failed）
  - `DesignEditorPresenter`
  - `DesignWriter` + `CliDesignWriter`（通过正式 CLI 命令写入，不直接写 SQL）
  - presenter 测试：create → revise、无效草案不调用 writer
- 尚未接 Slint 四步编辑器或保存后的 ReadModel 重载。

下一步：实现 Slint DesignEditor（研究问题/组别/检测/时间点），接 `CliDesignWriter`，保存后重建 ReadModel。

**验收：** 研究者从地图进入冻结 Run，找到当前步骤、参数、目标和记录入口；模式主次不同。

### P4 · Protocol 编辑

在步骤展示链路稳定后，再引入：

- 可选 `StepMeta`（唯一 Schema）；
- 多步骤编辑器；
- Protocol 组合器；
- 上下文仪器模块（不建全局工具中心）。

**验收：** 创建/预览/修订/冻结多步骤 Protocol；旧版本、旧 Run、参数绑定不变。

### P5 · 写入闭环与性能

- Slint → Application Commands → Core → SQLite → reload。
- 记录实际事件、偏差、更正；不覆盖历史。
- 动画只响应 UI 状态，不产生执行事实。
- 性能预算：空闲 CPU、内存、帧率。

**验收：** 端到端完成一次真实结构交互；记录关联正确；资源预算达标。

## 3. 当前代码对应关系

| 现有文件 | P1 动作 |
|---|---|
| `src/cli.rs` | 保留命令包装；逐步把 DesignMap/Preview/Manifest 投影迁到 Core |
| `src/main.rs` | HTTP 调用 Application Service；不复制校验 |
| `apps/desktop/src/data.rs` | 降为轻量 SQLite 读适配器；不做领域解释 |
| `apps/desktop/src/main.rs` | 先消费共享读模型；P3 前不重排 UI |
| `apps/desktop/ui/AppWindow.slint` | P0/P1 不重写；P3 才拆 |
| `tests/` | 增加跨 Adapter、冻结、实例、兼容测试 |

## 4. 不可跳过的测试

见 [Core Contract v1 §6](contracts/core-v1.md#6-测试矩阵p0p1-必须)：

- DesignMap 一致性
- 冻结版本
- Protocol 递归
- 未知语义
- 时间点/检测隔离
- 操作实例
- 旧数据兼容
- 版本冲突
- 并行访问

## 5. 约束

1. 先接口，后 UI。
2. 每轮只迁移一个领域能力。
3. UI 不直接读数据库；`main.rs` 不重新实现领域投影。
4. 字段扩展先确定归属；Design/Protocol/Run/ToolAction 不混合。
5. 必须有可执行冻结 Fixture；空 Run 不能作为执行台验收。
6. 旧 CLI/Web 不退化；不改正式实验库。
7. 技术测试通过不等于产品交互通过；阅读器不能验收为操作台。
8. Agent 提出局部修改，Core 实施/验证，Slint 让用户确认；Agent 不绕过 Core 写库。

## 6. 下一步

只做 P0 剩余审查：

P0 契约已冻结。下一步只做 P1-A：

- 在现有根 crate 增加共享 `DesignMapContent` / `CurrentDesignMap` 投影；
- CLI/HTTP 改为调用共享实现，保持现有扁平 JSON 兼容；
- 增加 C1/C2/C11 测试；
- 不拆 Slint、不实现 StepMeta、不迁移 RunWorkspace。

P1-A/P1-B 已通过；下一步只做 P1-C（RunWorkspace）。
