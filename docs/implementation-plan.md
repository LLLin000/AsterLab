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

### P0 · Core Contract Freeze（当前）

**只做契约，不做 UI。**

交付：

- [x] `CONTEXT.md` 语义补充：Design/DesignVersion、Protocol/ProtocolVersion、StepOccurrence、ToolAction、InstrumentProfile、读模型、检查级别。
- [x] [Core Contract v1](contracts/core-v1.md)：`DesignMap`、`ProtocolPreview`、`RunWorkspace`、`OperationStep`、`StepOccurrenceKey`。
- [x] 模块依赖方向与旧数据兼容规则。
- [x] 测试矩阵（C1–C10）。
- [ ] 审查并冻结 `StepOccurrenceKey.target` / `occurrence_id` 的具体落库方式。
- [ ] 审查 `RunWorkspace.frozen_design` 是否复用 `DesignMap` 类型。

**禁止：**

- 不新增 Slint 页面、按钮或动画。
- 不实现 `StepMeta`。
- 不全面移动 `src/cli.rs`。
- 不修改正式 SQLite 数据。
- 不把 `ManifestItem {title, detail}` 当 Run 步骤模型。

**验收：** 接口、依赖方向、身份、兼容规则和测试用例明确；审查后才能 P1。

### P1 · 抽共享 Core

优先迁移已有测试基础的 `design.map`，再建立 `run.workspace`。

- Core 查询：`ExperimentQueries::design_map` / `protocol_preview` / `run_workspace`。
- CLI/HTTP 变成 Adapter；不复制校验。
- 桌面端停止自行解释科学关系；先消费共享读模型，不重写 UI。
- 保留旧 CLI/HTTP 输出；必要时用兼容投影。

**验收：** Slint、CLI、HTTP 对同一 design/run 得到同一语义；旧契约不退化；桌面端不再二次计算 n/覆盖/时间关系。

### P2 · 可执行 Run Fixture

当前 L929 fixture 的 `protocol_manifest` 为空，执行模式只能显示空清单。必须通过正式 Core 命令构建：

- 至少 3 个步骤；
- 参数绑定；
- 两个检测/时间点；
- 一张实际板图；
- 一条实际执行事件；
- 不涉及正式数据库。

**验收：** 不启动 Slint，只用 Core 查询就能回答 D1/D3 的任务、参数、目标和记录，且不能串到另一检测。

### P3 · 拆 Slint

此阶段才拆 `AppWindow.slint`：

- `AppShell`
- `ExperimentMapView`
- `DesignDetailView`
- `ProtocolStepsView`
- `RunWorkspaceView`
- `components/`

执行台实现 `StepList → StepDetail → Target/Plate → Record`；不再只是 Manifest 字符串列表。

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

- 决定 `StepOccurrenceKey` 的 target/occurrence 落库方式；
- 决定 `RunWorkspace.frozen_design` 的类型复用；
- 给 `ToolAction` / `InstrumentProfile` 定版本与所有者；
- 完成 P1 迁移清单和对应测试命名。

P0 审查通过后，才开始 P1 的 `design.map` 共享查询迁移。
