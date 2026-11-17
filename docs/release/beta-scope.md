# AsterLab v0.1.0 Beta 交付范围

- 状态：v0.1.0 Beta 范围冻结候选；本文只定义交付范围，不定义字段、枚举或错误码。
- 基线：`main` @ `0fac770`（2026-11-17，CLOSE-07 合并后；原 `320d5a0`（2026-11-13）已过期，仅留作范围冻结起草基线）。
- 权威关系：领域术语以 [`CONTEXT.md`](../../CONTEXT.md) 为准，已实现字段以 [`AGENT_API.md`](../../AGENT_API.md) 为准，架构以 [`docs/architecture.md`](../architecture.md) 为准，UI 规则以 [`docs/product/ux-contract.md`](../product/ux-contract.md) 为准，文件所有权以 [`docs/product/ownership.md`](../product/ownership.md) 为准。本文与上述文档不一致时的处理见 §1.4。
- 引用方式：下游卡片写 `依据 DS-04`、`阻塞 L-02`；不要复制条目正文，也不要改写编号。

## 1. 交付平台

### 1.1 平台定义

| 项 | 定义 |
|---|---|
| 产品形态 | Windows x64 单机本地桌面应用；离线运行；单用户；无服务端部署、无云同步 |
| 前端 | MyGo Native 桌面应用 `prototypes/mygo-run`（Go + MyGo pin `v0.8.4-0.20261108220316-c4197b7fe562`），六个表面：项目工作区 / 实验设计 / 实验运行 / Notebook / 实验方案 / 资料与工具 |
| 后端 | Rust Core：`crates/asterlab-core`、`crates/asterlab-app`、`src/cli.rs`；校验、修订、幂等与事务提交的唯一权威 |
| 前后端边界 | MyGo 经 native module `prototypes/mygo-run/modules/runs` 调用同一批 Rust 函数；Go 不直连 SQLite，不复制业务规则，正常业务调用不使用 `os/exec`（`os/exec` 仅出现在 `internal/agentedit/client.go` 的开发态 CLI 调试路径）；Core 是校验与投影的唯一权威，Go 侧 `internal/workflowtree` 显示投影仍保留、与 Core 对齐直至 GUI 迁移，不在其上另建领域推导（[`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §7） |
| 数据 | 本地 SQLite（WAL）；默认 `%LOCALAPPDATA%\AsterLab\data\labworkbench.sqlite`，可由 `LABWORKBENCH_DATA_DIR` 覆盖；数据目录属于单机，不跨平台共享 |
| Agent / CLI 面 | `labworkbench tools --json`、`labworkbench schema <命令>`；`GET /api/...` 本地 HTTP 契约继续服务自动化与历史 Web 适配层，不是 Beta 交付面，也不是对外服务 |
| 验收泳道 | Windows Gate（`lane:win`）负责构建、安装、真实窗口与 GUI E2E；Linux VPS（`lane:vps`）只负责 Core / CLI / 非 GUI 包。Linux 上的 Go 测试使用 host-native `libRuns.so`，不能代替 Windows GUI 结论（[`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §1、[`cross-platform-builds.md`](../engineering/cross-platform-builds.md) §1） |

平台结论：**v0.1.0 Beta 的正式交付平台是 Windows x64 + MyGo Native 桌面应用 + 本地 Rust Core。** 本平台不含 Linux/macOS GUI 交付物，也不含浏览器 Web 前端作为正式前端（见 §1.4）。

### 1.2 交付物

| 产物 | 生成方式 | 说明 |
|---|---|---|
| `prototypes/mygo-run/dist/mygo-run.exe` | `scripts/mygo-build.ps1` | 桌面应用；不含 Rust/Node/Go 工具链依赖 |
| `prototypes/mygo-run/dist/Runs.dll` | `cd prototypes/mygo-run && mygo native check .\modules\runs` | 与 EXE 匹配的 Rust native module；不跨平台复用 `libRuns.so` |
| `BUILD_INFO.txt` | `scripts/mygo-build.ps1` | 构建来源信息 |
| 安装包 | `scripts/mygo-package.ps1` + `scripts/mygo-installer.nsi` | NSIS 安装包；当前未签名 |
| 验收记录 | `scripts/gui-e2e-v11.ps1`、`scripts/rc3-fixture.py`、`docs/release/MYGO_0.1.0_BETA_RC_VERIFICATION.md` | 原始输出与 SHA256 必须落盘 |

未签名（Authenticode `NotSigned`）是当前已知状态，不是缺陷；不得在任何交付说明中暗示已签名。

### 1.3 平台边界

- 只承诺 Windows 10/11 x64 行为。其他平台没有交付物，也没有验收结论。
- `Runs.dll` 与 `libRuns.so` 是同一 Go/Rust 契约的不同产物；禁止互相覆盖。
- 目标机器不需要 Rust、Go、Node；开发态需要，但打包产物不得包含工具链假设。
- VPS 上通过的 Core/CLI 测试只是 Windows Gate 的前置条件，不构成 GUI 证据。

### 1.4 文档冲突与处置

本仓库存在未消解的平台表述冲突。按 `AGENTS.md`，此处记录冲突，而不是在实现中静默选边：

| 位置 | 现表述 | 冲突 | 处置 |
|---|---|---|---|
| `docs/adr/0001-web-first-beta.md`（2026-11-07） | Web UI 是产品壳；Slint 不作第二正式前端 | 未被任何新 ADR 取代，但晚于它的 `docs/architecture/mygo-v1-contract.md`（2026-11-09，冻结）、`docs/product/ux-contract.md`（2026-11-12，绑定）、`docs/product/ownership.md`（2026-11-12）都按 MyGo 页面组织 | 本卡不改 ADR；**DS-12** 要求在发布前由独立卡新增取代该前端条款的 ADR；按 `doc-authority.md` §3，RC-05 已确认 DS-12 仍 `[UNVERIFIED]`，不得引用为“平台决策已获批准” |
| `README.md` 第 3 行 | “Web UI 是本轮正式交付前端，Slint 仅保留为研究原型” | **已解决（2026-11-13）**：根 `README.md` 第 8–9 行已改为 MyGo Native 交付方向，与 §1.1 一致；`README.md`“启动/当前工作流”节保留为历史 Web 包记录并已标注迁移说明 | 读当前方向以 §1.1 为准 |
| `docs/architecture.md` 第 38 行（现行文本已更新） | 旧记录称“Beta 交付面是 Web + Rust 本地服务” | **已解决**：`architecture.md` 现行 §1/§3 已写明 Beta 交付面是 MyGo Native + Rust 本地 Core，Slint §3/§6 标为历史 | 读现行 `architecture.md` §1 与 §3 章节头 |
| `docs/design/MYGO_EVALUATION.md`（2026-11-09） | MyGo 是视觉基准，不是生产替代 | 结论早于 MyGo v1 契约冻结与 RC 安装验证 | 按历史评估读取，不作当前结论 |
| `docs/release/MYGO_0.1.0_BETA_RC_VERIFICATION.md` | 与 Web 包同用 `0.1.0-beta` 版本名 | 两个 `0.1.0-beta` 产物并存，易误认 | 发布说明必须指明产物文件名与平台；Web 包不在本平台范围内 |

冲突未消解前，本文只作为 Beta 范围定义，**不得被引用为“平台决策已获批准”**。

## 2. 必须闭环

本节是发布要求，不是现状说明。某条是否已达成的唯一依据是 §4 对应行的证据；没有证据即未达成，未即发布阻塞。

| ID | 必须闭环 | 必须成立 | 依据 |
|---|---|---|---|
| L-01 | 设计 → Protocol → Run → 记录 → 重启回读 | 一条真实设计经同一 Core 命令保存，创建 Run 后写入读数，关闭并重开应用后仍能读回同一值 | [`ux-contract.md`](../product/ux-contract.md) §1；[`mygo-v1-contract.md`](../architecture/mygo-v1-contract.md) RunService/DesignService |
| L-02 | 版本与冻结 | Run 固定 DesignVersion、ProtocolVersion 与步骤块版本；已冻结 Run 不跟随后续设计修改；过期修订被拒绝并提示冲突，不静默覆盖 | [`CONTEXT.md`](../../CONTEXT.md) 读模型与执行身份；[`mygo-v1-contract.md`](../architecture/mygo-v1-contract.md) Errors |
| L-03 | 写入边界 | 事实只从 Run 经 Core 命令写入；Notebook 只写散文且只能引用已有事实；Protocol 只读；Tools 只送草稿，确认后才写 | [`ux-contract.md`](../product/ux-contract.md) §1、§1.1–1.3；[`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §6.3 |
| L-04 | SDK 模块闭环 | 新模块必须同时注册 `ASTERDOC_RULES`（`src/cli/asterdoc.rs`）与 `knownModuleRules`（`prototypes/mygo-run/internal/asterdoc/spec.go`）；模块在 Design / Protocol / Run / Notebook / CLI 可用；显示类模块不得要求 Core store/transaction 变更 | [`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §6.4–6.5；[`review-2026-11-12.md`](../product/review-2026-11-12.md) §3 |
| L-05 | Windows 交付闭环 | 构建 → native check → `go test ./...` → GUI E2E → 安装 → 启动 → 重启 → 卸载，安装后 EXE/DLL 哈希与构建产物一致，用户数据库在卸载后仍可读 | [`cross-platform-builds.md`](../engineering/cross-platform-builds.md) §6、§8；[`MYGO_0.1.0_BETA_RC_VERIFICATION.md`](MYGO_0.1.0_BETA_RC_VERIFICATION.md) |
| L-06 | 数据可恢复 | 交付平台上存在可执行验证的备份/恢复路径，恢复后记录、版本与执行事件可读且未被改写 | `BACKUP_RESTORE.md`（交付平台 MyGo 路径）+ `tests/backup-restore.cjs`（历史 Web 包路径）；语义以 QA-05 封闭文件复制验收为准，应用内按钮端到端见 `KNOWN_LIMITATIONS.md` |

## 3. 非目标

v0.1.0 Beta 明确不做以下内容；任何一项都不构成发布阻塞，也不得在发布说明中被描述为“即将支持”：

1. 插件 ABI、模板市场、真实设备/仪器控制。SDK 是编译进二进制的模块加稳定契约（[`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §5、[`review-2026-11-12.md`](../product/review-2026-11-12.md) §3 第 4 条）。
2. 多用户、协作、账号权限、云同步、自动定时备份、在线验收（[`docs/KNOWN_LIMITATIONS.md`](../KNOWN_LIMITATIONS.md)）。
3. 电子签名或审计级签名链；`operator` / `checked_by` 只是记录标签（[`docs/KNOWN_LIMITATIONS.md`](../KNOWN_LIMITATIONS.md)）。
4. 代码签名与签名流水线；当前产物未签名，签名状态如实披露（[`MYGO_0.1.0_BETA_RC_VERIFICATION.md`](MYGO_0.1.0_BETA_RC_VERIFICATION.md)）。
5. 第二种正式前端：不同时维护 Web/MyGo 两套发布前端（[`docs/architecture.md`](../architecture.md) §1）。Web 包只作为历史交付物保留。
6. Linux / macOS GUI 交付物；VPS 只验证 Core、CLI 与非 GUI Go 包（[`cross-platform-builds.md`](../engineering/cross-platform-builds.md) §1）。
7. `renderers.static` 静态渲染器与 `object-list` 可编辑行（[`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §5、[`review-2026-11-12.md`](../product/review-2026-11-12.md) §4 Wave 3）。
8. 完整 ISA / AnIML / OBI / RO-Crate 导入导出、通用工作流或脚本引擎、自动生成完整实验方案（[`docs/architecture.md`](../architecture.md) §8）。
9. 跨平台共享数据目录；SQLite 数据属于单机单平台（[`cross-platform-builds.md`](../engineering/cross-platform-builds.md) §9）。
10. 全仓 `cargo fmt`；会埋掉 review diff（[`HANDOFF-VPS.md`](../../HANDOFF-VPS.md) §5）。
11. 文件附件、样本库存/试剂批次、可视化自定义字段类型系统与安全审计：Beta 不交付附件存储、库存/批次管理或可视化自定义字段类型系统，也不出具安全审计或渗透测试结论（根 [`README.md`](../../README.md) 第 55–58 行「当前限制」、[`docs/KNOWN_LIMITATIONS.md`](../KNOWN_LIMITATIONS.md)）。

## 4. 发布 DoD

> RC-05 已知结论（基线 `e2a4603`，`evidence/RC-05/notes-vs-scope.md`）：DS-08 `[BLOCKED]`（Windows 单测 `protocol_version_queries_return_expansion_and_stable_not_found` 确定性失败，OS error 32）；DS-12/DS-13 `[UNVERIFIED]`；DS-05 GUI E2E 步、DS-07 CLI `sdk.validate` 探针、`DS-04` GUI 冻结徽标截图 `[UNVERIFIED]`；DS-09 仅封闭文件语义已证、应用内按钮路径 `[UNVERIFIED]`。以下 DoD 仍是发布要求：未验证项不得冒充完成。


| ID | 要求 | 证据（命令 / 工件） | 泳道 |
|---|---|---|---|
| DS-01 | 干净 Windows 机器安装后可直接启动，无需 Rust/Go/Node；窗口出现并创建数据目录与数据库 | 安装 `AsterLab-MyGo-*-setup.exe`；观察窗口标题与 `%LOCALAPPDATA%\AsterLab\data\labworkbench.sqlite`；`PRAGMA integrity_check` = `ok` | Windows |
| DS-02 | 六个表面全部可达，且每面只有一个主操作，符合 IA 定义 | [`ux-contract.md`](../product/ux-contract.md) §1 表面表；900×600 与 1360×880 真实窗口截图 | Windows |
| DS-03 | 设计与 Protocol 版本化写入闭环：保存产生新修订；过期修订返回冲突且保留草稿 | `go test ./...` 中 revision/rollback 用例；`stale_revision_rollback_test.go` | VPS + Windows |
| DS-04 | Run 创建必须经人工确认，创建后显示冻结修订；冻结快照不随后续设计修改 | GUI E2E 的建 Run 步骤；`asterdoc_freeze_test.go`；截图含 `已冻结 · Run <id> · rev <n>` | Windows |
| DS-05 | 读数与步骤完成写入走 Core 事务，重启后回读一致；重复操作 ID 不产生第二条事实 | `scripts/gui-e2e-v11.ps1`（粘贴 → 保存 → 重启 → 回读）；`cck8_loop_test.go`、`concurrency_cross_process_test.go` | Windows |
| DS-06 | L-03 写入边界可被测试证明：Protocol 无编辑控件，Notebook 无记录控件，Tools 送出的是未确认草稿 | 页面测试 `notebook_page_test.go`、`protocol_page_test.go`；截图审查 | Windows |
| DS-07 | SDK 模块双注册且可跨表面使用；未注册规则在启动时被两侧拒绝 | `cargo test --bin labworkbench`；`go test ./...`；`labworkbench sdk.validate` 接受 `rna_reagent_usage@1`、拒绝 `target_concentration > stock_concentration` | VPS + Windows |
| DS-08 | Windows 门禁全绿：Rust 三组测试、native check、`go test ./...`、GUI E2E 输出 `GUI E2E PASS` | `cargo test --bin labworkbench`；`cargo test --manifest-path crates/asterlab-core/Cargo.toml`；`cargo test --manifest-path crates/asterlab-app/Cargo.toml`；`cd prototypes/mygo-run && mygo native check .\modules\runs`；`powershell -File scripts/gui-e2e-v11.ps1` | VPS + Windows |
| DS-09 | L-06 数据可恢复：备份/恢复路径在交付平台可执行，恢复后数据一致 | 备份 → 恢复 → 重启回读的原始输出，恢复后 `PRAGMA integrity_check` = `ok` | Windows |
| DS-10 | 交付物完整且可核对：EXE、DLL、安装包、`BUILD_INFO.txt`；公布 SHA256 与未签名状态；卸载不留残留 | `MYGO_0.1.0_BETA_RC_VERIFICATION.md` 格式的安装/启动/重启/卸载记录 | Windows |
| DS-11 | 文档与实现一致：[`docs/KNOWN_LIMITATIONS.md`](../KNOWN_LIMITATIONS.md) 覆盖 §3 的非目标；发布说明不宣称未实现能力 | 逐条比对 §3 与 [`KNOWN_LIMITATIONS.md`](../KNOWN_LIMITATIONS.md)；评审签字 | 人工 |
| DS-12 | 平台权威一致：新增 ADR 取代 `0001-web-first-beta.md` 的前端条款，并与 `README.md`、`docs/architecture.md` 表述一致 | 新 ADR 文件 + 两处文件修订 diff | 人工 |
| DS-13 | RC-3 验收夹具仍可复现：`assess: warning []` 且 3 个 protocol task 未变 | `python3 scripts/rc3-fixture.py`（`LABWORKBENCH_CLI` 指向构建产物，数据目录为新建临时目录） | VPS |

所有 DoD 的原始输出（命令、时间、机器）必须贴到发布记录或对应 PR；只写“通过”不算证据。
