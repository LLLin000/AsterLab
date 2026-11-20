# Native FFI 桥接与 Agent CLI 契约

当工作流程从日常本地 Vault 笔记升级到不可变受控冻存，或由外部自动化 Agent 协同执行时，系统通过 Go⇄Rust FFI 与 `labworkbench` JSON CLI 操作 Core SQLite 存储。

## Native FFI Bridge（`modules/runs`）

`prototypes/mygo-run/modules/runs/runs.go` 通过 `//mygo:native` 自动生成胶水层并绑定至 Rust `crates/asterlab-core`，暴露四组原生服务接口：

| 服务接口 | 核心方法与职责 |
|---|---|
| **`RunService`** | `LoadRun`、`ListRuns`、`SaveReadout` / `SaveReadoutsBatch`、`SaveTaskChecks`、`SaveAssetLink` / `UpdateAssetLinkCoordinates` / `UnlinkAssetLink`、`CompleteTask`、`CreateExecutionEvent`、`UpdateTimer`、通用幂等写入口 `ExecuteCommand`（强校验 `IdempotencyKey`）、只读投影查询 `QueryProjection` |
| **`DesignService`** | `ListDesigns`、`LoadDesign`、`SaveDesign`、`CreateAsterDocDesign`、`SaveAsterDoc`、`ApplyAsterDocCommand`、`CompileAsterDoc`、`AssessAsterDoc`、`CreateRunFromAsterDoc`（将某修订的 AsterDoc 原子冻结为不可变 Run）、`DeriveDesignFromRun` |
| **`TimeBudgetService`** | 无状态只读接口 `ProjectTimeBudget`，计算流程并行与串行时间窗口预算 |
| **`MixPlanService`** | 无状态只读接口 `PlateMixPlan`，按分组与复孔数量计算试剂配平总量与富余量 |

## Agent CLI 调用契约（`labworkbench`）

详细命令全表见 [AGENT_API.md](../../AGENT_API.md)。CLI 与桌面端共用同一本地 SQLite 存储，遵循严格的封闭 JSON 契约：

```powershell
.\target\release\labworkbench.exe tools --json
.\target\release\labworkbench.exe schema design.create
.\target\release\labworkbench.exe design.create --input .\design.json --diagnostics json
```

- **封闭输入**：所有业务命令均通过 `--input <json-file|->` 传入最大 2 MiB 的 UTF-8 JSON；未知字段、超范围数值或类型不匹配一律拒绝。
- **单行结构化响应**：`stdout` 每次固定输出一行 JSON（`{"ok":true,"tool":...,"result":...}` 或 `{"ok":false,"tool":...,"error":{"code":...,"message":...}}`）。
- **固定退出码语义**：
  - `0`：成功
  - `2`：输入或用法错误（`INVALID_INPUT`）
  - `3`：目标对象不存在
  - `4`：`REVISION_CONFLICT`（乐观锁版本冲突，需重新读取后再修订）
  - `5`：内部错误
- **机器诊断视图 (`--diagnostics json`)**：追加标准 `Diagnostic` 数组（`path`、`code`、`severity`: `blocking | needs_confirmation | advisory`、`message`、`affects`、`next_action`、`source`），Agent 必须基于稳定 `code` 分支决策，禁止正则解析自然语言 `message`。
