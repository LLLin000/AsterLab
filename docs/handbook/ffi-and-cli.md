# Pure-Go CLI、深度链接与历史 FFI 契约

依据 [ADR-0002](../adr/0002-asterml-vault-pure-go-core.md)，Agent 与外部自动化脚本直接通过 **Pure-Go Vault-Native CLI（`cmd/asterlab`）** 读写并校验本地 `.asterml` Vault，不再强制经过 `Runs.dll` 或 SQLite；`asterlab://` 深度链接支持直接定位实验文件、步骤与储藏室试剂。

## 1. Pure-Go Vault-Native CLI (`cmd/asterlab`)

源码位于 [cmd/asterlab/main.go](../../prototypes/mygo-run/cmd/asterlab/main.go)，通过 `--vault <dir>` 指定实验室目录（默认读取 `vault.DefaultVaultDir()` / `ASTERLAB_VAULT_DIR`）：

```powershell
# 列出可用工具清单
go run ./cmd/asterlab tools --json

# 按状态/标签筛选 Vault 中的实验列表
go run ./cmd/asterlab --vault C:\LabVault vault.list --status in_progress --tag cck8

# 全库全文搜索
go run ./cmd/asterlab --vault C:\LabVault vault.search "OD450"

# 读取实验 Frontmatter 与 step 模块树
go run ./cmd/asterlab --vault C:\LabVault asterdoc.read 2026/cck8.asterml

# 运行 Pure-Go AsterCore 分级科学诊断（ARRIVE 2.0 / 对照组 / 浓度越界）
go run ./cmd/asterlab --vault C:\LabVault asterdoc.assess 2026/cck8.asterml

# 运行确定性 CCK-8 / RT-qPCR ΔΔCq / 描述性统计分析
go run ./cmd/asterlab --vault C:\LabVault asterdoc.analyze 2026/cck8.asterml
```

- **单行 JSON stdout 契约**：所有子命令向标准输出打印单行结构化 JSON（包含 `ok` 及对应结果或 `code`/`error`）。
- **固定退出码语义**：
  - `0`：成功（对 `asterdoc.assess` 表示无 `blocking` 阻断问题）
  - `2`：参数或子命令用法错误
  - `3`：Vault 目录或目标 `.asterml` 文件不存在
  - `4`：科学诊断阻断（`asterdoc.assess` 检测到 `SeverityBlocking` 问题，如目标浓度高于母液）
  - `5`：`.asterml` 语法解析失败

## 2. `asterlab://` URI 深度链接路由

解析实现位于 [deeplink.go](../../prototypes/mygo-run/internal/state/deeplink.go) 的 `ParseAsterLabURI`，并在 [main.go](../../prototypes/mygo-run/main.go) 单实例与启动参数中统一调度：

| URI 路由格式 | 路由动作 |
|---|---|
| `asterlab://open?file=<relPath>&step=<stepID>` | 在活动窗格打开指定 `.asterml` 文件并自动滚动聚焦至 `<stepID>` |
| `asterlab://pantry?code=<labelCode>` | 切换至 `TabPantry` 储藏室货架并高亮定位对应试剂条码 |
| `asterlab://run/<runID>` · `asterlab://run/<runID>/well/<well>` | 打开对应历史 Run 并定位孔板指定孔位 |

## 3. 可选历史 Rust FFI 与 `labworkbench` CLI（Deprecated / Legacy）

在仍需兼容历史 SQLite 冻存记录或原有测试夹具的场景下，仓库继续保留：

- **`modules/runs`（`//mygo:native`）**：暴露 `RunService`、`DesignService`、`TimeBudgetService` 与 `MixPlanService`；依据 ADR-0002，桌面主程序启动若缺少 `Runs.dll` / `libRuns.so` 将静默运行在纯 Go Vault 模式，不再阻塞报错。
- **`labworkbench` CLI（`src/cli.rs` & [AGENT_API.md](../../AGENT_API.md)）**：历史 SQLite 存储上的 `--input <json>` / `--diagnostics json` 契约参考。
