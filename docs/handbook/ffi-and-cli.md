# Pure-Go CLI 与深度链接契约

Agent 与外部自动化脚本直接通过 **Pure-Go Vault-Native CLI（`cmd/asterlab`）** 读写并校验本地 `.asterml` Vault，产品主运行时已彻底弃用 `Runs.dll` / SQLite 与 Rust CLI；`asterlab://` 深度链接支持直接定位实验文件、步骤与储藏室试剂。

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

## 3. 已弃用的历史 Rust 路径说明（Archived）

仓库历史阶段曾存在 `modules/runs`（`Runs.dll`）与 Rust `labworkbench`（`src/cli.rs`）SQLite 存储方案，现已全面弃用并由上方的 Pure-Go `vault` + `astercore` + `cmd/asterlab` 完全替代。新功能与自动化工具一律基于 `.asterml` 与 `cmd/asterlab` 开发，不引入任何 Rust 或 SQLite 依赖。
