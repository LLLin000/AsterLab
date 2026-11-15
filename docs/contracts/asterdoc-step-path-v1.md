# AsterDoc Step Path v1（空 step path 的派生与拒绝）

- 日期：2026-11-15
- 状态：v1 契约；只补全路径，不改写既有路径与冻结 Run。
- 代码：`crates/asterlab-core/src/asterdoc_path.rs`（唯一边界实现）、`src/cli.rs`（`asterdoc.command` 的 `add_step` / `nest_step` / `promote_step`）。
- 关联：[AGENT_API](../../AGENT_API.md) §`asterdoc.command`、[workflow-tree-contract-v1](../sdk/workflow-tree-contract-v1.md)、[CONTEXT](../../CONTEXT.md) `OperationStep`。

## 0. 目标与边界

LB-07 实测：HUVEC 设计的 7 个步骤 `path` 全为空（设计画布 `canvasAddStep` 写入 `path: ""`），而 `nest_step` / `promote_step` 对空路径直接报 `step path must have a final segment`，缩进/提升在真实设计上不可用。

本契约固定三件事：

1. 空 `path` **不拒绝写入**，而是在 Core 写新 revision 时按稳定规则派生；
2. 派生不能产生唯一无环路径时，**明确拒绝**并给出可操作原因（哪个步骤、缺哪个字段、怎么补）；
3. 既有 `path` 与已冻结 Run 的路径**不改写**。

范围边界：

- 只影响新生成的 AsterDoc revision。`asterdoc_version` 历史记录与 Run 冻结的 `asterdoc.source_hash` 不变；`run.workspace` / `workflow.inspect_tree` 对旧 revision 的读法不变。
- 派生只发生一次，写入后就是普通路径；再次 nest/promote 按普通路径处理。
- 不做 UI、不改 `prototypes/mygo-run/**/*.go`。设计画布继续发送 `path: ""`，由 Core 补全。

## 1. 派生规则

```text
step.path 为空（缺失、""、纯空白）-> root/<step-id>
```

- `root` 前缀与 `step.edit` 的 Protocol 寻址一致（`root`、`root/<use_id>`）。
- `<step-id>` 是步骤模块的稳定 `id`，不做 slugify、不做时间戳派生；同一 id 永远得到同一路径。
- 派生时机：
  - `add_step`：模块未带 `path` 时写入派生路径，避免再从画布产生不可达步骤；
  - `nest_step`：目标步骤缺 `path` 时先派生 `root/<id>` 再计算新路径；父步骤缺 `path` 时同一次 revision 内一并补全 `root/<parent-id>`，否则子路径会指向没有任何步骤使用的父路径；
  - `promote_step`：目标步骤缺 `path` 时只补全 `root/<id>`（已是最外层，没有可提升的父段）。
- 已带 `path` 的步骤原样保留（只做首尾空白 trim）；派生不会覆盖它。

## 2. 路径计算

```text
nest_step(child, parent)     -> <parent_path>/<child_leaf>
promote_step(child)          -> <parent(parent_path)>/<child_leaf>
child_leaf                   = 路径最后一个非空段
```

- `root/parent/child` 提升一级得到 `root/child`；`huvec/prep/child` 得到 `huvec/child`。
- `root/child`（深度 2）已经是该命名空间的最外层，`promote_step` 拒绝而不是生成 `root/child` 自身。
- 目标步骤已有后代时，后代路径按旧前缀整体改写：`root/parent/child` 提升为 `root/child` 时，`root/parent/child/deep` 变为 `root/child/deep`。
- 目标步骤原本没有 `path` 时不存在可识别的后代（后代靠路径前缀定位），只改写目标步骤本身。

## 3. 拒绝条件（`INVALID_INPUT`，不写入 revision）

| code | 触发 | 消息要求 |
|---|---|---|
| `step_path_derivation_failed` | id 为空、含 `/`，或既有路径没有最终段 | 指出步骤 id、规则 `root/<step-id>`、以及“为该步骤显式设置 path” |
| `step_path_already_nested` | 目标已经在该父步骤下（新路径等于旧路径） | 指出当前路径，建议换 `parentId` 或先 `promote_step` |
| `step_path_already_top_level` | `promote_step` 的路径已在最外层 | 指出当前路径，建议先 `nest_step` |
| `step_path_cycle` | 父路径等于目标路径或位于目标子树内 | 指出父子步骤 id 与冲突路径 |
| `step_path_conflict` | 改写后两个步骤路径相同 | 指出两个步骤 id、冲突路径，并给出改名示例 `path: <path>-2` |

- 所有拒绝都在保存前完成整篇校验，**不产生部分 revision**。
- 错误通过 `error.message` 传递（`asterdoc.command` 的失败 schema 只有 `code`/`message`）；`code` 由 Core 保留，用于测试与后续诊断扩展，不新增 CLI 错误码。
- `--diagnostics json` 下这些失败仍产生一条 `blocking` 诊断，`source = derived`。

## 4. 兼容与冻结

- 冻结语义不变：Run 固定引用创建时的 `asterdoc.revision` 与 `source_hash`；补全只写新的 revision，旧 revision 字节不变。
- 空 `path` 的回读语义不变：`workflow.inspect_tree` 对仍为空的步骤继续报 `step <id> has no path`，不猜父级。
- 无路径的历史设计不会被迁移；下一次 `add_step` / `nest_step` / `promote_step` 只补写被操作的步骤。
