# RDO-01 证据：通用定量记录模块

结论：三项验收命令 exit 0；L1（`labworkbench` + `asterlab-core` + `asterlab-app`）全绿；`internal/asterdoc` 与其余 Go 包全绿。

## 文件

| 文件 | 证明 |
|---|---|
| `acceptance.txt` | 三条验收命令的原始输出与 exit code：`readout_schema` 12 通过、`readout_submit` 11 通过、`modules[]` 长度 17（≥16） |
| `l1.txt` | 工作方法 Core 要求的 L1 套件：`cargo test --bin labworkbench` 17 通过、`asterlab-core` 298+3+6 通过、`asterlab-app` 23 通过 |
| `go.txt` | `go test ./internal/asterdoc/` 通过；除根包外全部包通过；根包失败行全部是 `the implementation of Runs is not built`（本 worktree 未构建 native 动态库，CI 在 Go tests 前跑 `mygo native build -target host ./modules/runs`） |
| `spec.txt` | Rust CLI 从同一 `modules.json` 校验 `readout` 文档：合法文档 `valid: true`，`value_kind: spectrum` 被拒并给出五个枚举值；`modules[]` 长度 17 与 `readout@1` 条目原文 |
| `readout-doc.asterlab` / `readout-doc-invalid-kind.asterlab` | `spec.txt` 使用的输入 fixture |

## 实现要点

- `modules.json` 新增 `readout@1`：category `measurement`，权限 `run.readout`，`value_kind` 枚举覆盖 `numeric`/`table`/`plate`/`categorical`/`file`，并声明 `missing_reasons`、`null_allowed`、`source_kinds`、`table_columns`、`plate_format`；`rules: []`，因此不涉及 `ASTERDOC_RULES`/`knownModuleRules` 两端注册。
- `crates/asterlab-core/src/readout_records.rs`：`ReadoutRegistry` 注册并校验 schema（同 id 只能重复注册相同定义），`plan_readout_submission` 让孔板/单值/表格共用同一次 `validate_schema` → `validate_readout` → `check_derived_sources` 校验和同一个 `execution_event.create` 载荷；`query_readout_records` 读回存储的读数。
- 存储：`result_cells` 里 `kind: "cell"` 是数值事实单元格（`query_readings` 消费），`kind: "record"` 是完整 v1 `Readout`（表格、分类、文件没有无损 `readouts[]` 形式，见契约 §2.2）。`query_readings` 跳过 `record` 条目，既不丢数据也不污染数值事实表。
- 孔板读数按孔写 `readouts[]`（契约 §2.2 `per_cell`），单值读数写单条 `readouts[]`；表格读数 `readouts[]` 保持空。

## 已知缺口与限制（如实陈述）

- `readout@1` 只在 manifest 与 Rust Core 侧落地；Go `BuiltinRegistry()` 没有对应 `ModuleSpec`，MyGo 侧没有解析器/编辑器/Run 视图。因此 Go 的 16 条注册计数断言不变，MyGo 显示与编辑属 Windows Gate。
- 表格读数不进入数值事实表：`ReadoutValue::Table` 没有逐 cell 的样本/孔位身份，`reading_facts` 的 key 无法归属；按记录保存并经 `query_readout_records` 读回。
- 混合 schema 的提交不写事件级 `schema_id`/`schema_version`（单一 schema 时才写，避免把批次标成某一个 schema）。
- `table_columns` 里的分类列只校验列声明；表格单元格值是数值网格，无逐格缺失原因（契约 §8 未决项 2）。

## 卡片依赖异常

Kandev `blocked_by` 当前指向 `QA-07`（三组 fixture 回归），该卡语义上排在本卡之后，且其描述依赖本卡产出；上游真实前置 `SCI-03`（`ReadoutSchema` 契约，PR #26）已合并，本分支基于 `dd00f20`。未绕过 `SCI-03`。
