# Kandev 任务卡模板

本文件固定 AsterLab 在 Kandev 中创建、细化、验收任务的写法。所有任务卡在进入 `Ready` 前必须满足本文的 `Definition of Ready`。

## 1. 分类标签

分类标签写在任务标题的圆括号里，使用英文，便于筛选和让 agent 一眼判断工作类型：

| 标签 | 适用工作 | 主要产出 | 默认泳道 |
|---|---|---|---|
| `(Docs)` | 文档、发布说明、权威性整理 | Markdown 变更、链接/命令检查 | VPS Dev |
| `(Audit)` | 只读盘点、差异审计、风险评估 | 审计报告、处置表 | VPS Dev |
| `(Contract)` | 接口、数据、错误码、版本契约 | 契约文档 + 必要类型定义 | VPS Dev |
| `(Core)` | Rust Core、CLI、业务规则、迁移 | 代码 + 测试 | VPS Dev |
| `(UI)` | MyGo/Slint GUI、交互、视觉 | 代码 + Windows 实机证据 | Windows Gate |
| `(Integration)` | 跨层闭环、端到端 gate、fixture | 走查记录、截图、原始输出 | Windows Gate |
| `(QA)` | 回归、数据完整性、安全、备份恢复 | 测试矩阵 + 证据 | VPS Dev / Windows Gate |
| `(Build)` | CI、跨平台构建、生成器、可重现性 | CI 配置、构建产物、hash | VPS Dev |
| `(Release)` | 安装器、升级、Release 资产、回滚 | 安装包、资产、演练记录 | Windows Gate |

规则：

- 一张卡只能有一个分类。需要同时改 Core 和 UI 时，拆成两张卡，用 `blocked_by` 连接。
- 分类不是泳道的替代品。泳道决定在哪里执行；分类决定 agent 用什么方法执行。
- 标题格式：`<卡片 ID> (<分类>) <中文标题>`，例如 `RUN-03a (Core) 统一记录内核：事实表与查询`。

## 2. 卡片模板

```markdown
# <ID> (<Category>) <标题>

## 结果
完成后，<谁> 能 <做什么>。可观察信号：<...>。

## 基线
- 从任务 worktree 的当前分支开始；不 fetch/rebase 其他任务分支。
- 仓库：LLLin000/AsterLab；默认分支：main。

## 范围
<!-- kandev-scope
write:
  - path/to/file-or-dir
read:
  - path/to/file-or-dir
forbid:
  - path/to/other
-->

只读卡写 `write: []`。范围只列路径，不在块内写解释。

## 实施
1. ...
2. ...

## 验收
- 命令：`...`
- 期望：exit code / stdout 子串 / 行数 / hash
- UI：1360×880 与 900×600，浅色/深色，规则 id `...`
- 证据：`docs/product/evidence/<ID>/` 或 `docs/release/evidence/<ID>/`

## 证据
- 原始输出：
- 截图/录屏：
- PR / commit：

## 非目标
- ...
- ...

## 依赖 / 风险
- blocked_by：以 Kandev Relations 为准，正文不重复完整列表。
- 风险 / 回滚：...

## 工作方法（<Category>）
<从第 3 节复制的分类 prompt>
```

评审、合并权限和 Review WIP 见 [`kandev-review-policy.md`](kandev-review-policy.md)。

硬规则：

- 不在正文重复 `lane`、`milestone`、`priority`、依赖列表；这些用 Kandev 的 label、priority 和 `blocked_by`。
- `范围` 必须机器可读。`(Build)` 的范围检查读取 `<!-- kandev-scope -->` 块，不解析自然语言。
- 验收必须是命令 + 期望结果，或明确的截图规格 + 规则 id；不写“测试全绿”“实机走查”“你确认”。
- `DoD` 只保留人工签核项；能自动验证的内容放 `验收`。
- 描述控制在 800–1500 字；逐步清单放任务 Plan，评审结论放评论。
- 远期卡可以是 `candidate`，但一旦进入 `Ready`，必须满足本文全部要求。

## 3. 分类工作方法 prompt

### `(Docs)`

```text
工作方法（Docs）：
- 只改范围列出的文档；先读 docs/README.md 的权威表，冲突先记录再改。
- 不做代码、契约或 schema 改动；需要时停止并开新卡。
- 验收用 grep、链接检查或渲染检查；贴原始输出。
- 证据放文档自身链接或 docs/release/evidence/<ID>/。
```

### `(Audit)`

```text
工作方法（Audit）：
- 只读；不改代码、不切分支、不 rebase。
- 每条结论附 commit / file / 命令；审计前后 git status 一致。
- 输出表格：对象、现状、处置（merge/redo/drop）、理由、目标卡。
- 证据放 docs/release/<ID>/ 或任务评论。
```

### `(Contract)`

```text
工作方法（Contract）：
- 只写契约文档 + 必要类型定义；不实现行为、不改 GUI。
- 明确输入、输出、错误码、版本、并发、回滚；给 dry-run 示例。
- 验收：文档评审 + 类型可编译；错误码能被后续卡直接引用。
```

### `(Core)`

```text
工作方法（Core）：
- 先写失败测试，再实现；只改范围列出的 Rust/CLI 文件。
- 运行 L1：cargo test --bin labworkbench + crates/asterlab-core + crates/asterlab-app；贴原始输出。
- 不做 GUI、Windows、安装器改动；需要时停止并转 Windows Gate。
- 保持冻结 Run/Protocol 语义；不改 docs/contracts/* 除非卡里明写。
```

### `(UI)`

```text
工作方法（UI）：
- 先读 docs/product/ux-contract.md 与 docs/product/ownership.md；每条 UI 改动引用规则 id。
- 只在 Windows 实机验收；1360×880 与 900×600，浅色/深色截图。
- 不改 Core 契约；需要 Core 变更时停止并开 Core 卡。
- 证据放 docs/product/evidence/<ID>/。
- Windows 卡按卡片基线使用独立 ovnfs checkout；遵循 Windows Gate 的并行上限，GUI/build/E2E 验收一律串行。
```

### `(Integration)`

```text
工作方法（Integration）：
- 不新增功能；用固定 fixture 走完整路径。
- 每步记录命令、截图、原始输出；失败即阻断，不跳过。
- 不修改冻结设计/Protocol/Run；发现缺陷开修复卡。
- 证据放 docs/product/evidence/<ID>/ 或 docs/release/evidence/<ID>/。
```

### `(QA)`

```text
工作方法（QA）：
- 先列测试矩阵：正常、边界、失败、重启、并发。
- 使用隔离数据目录；不碰用户主库；每个用例可重复。
- 贴原始命令输出与数据库校验和；记录复现步骤。
- 发现缺陷开修复卡，不在本卡静默修代码。
```

### `(Build)`

```text
工作方法（Build）：
- 只改 CI、构建脚本或生成器；不改业务逻辑。
- 在 Linux 与 Windows runner 上验证；记录 commit SHA、工具版本、产物 hash。
- 失败路径必须可见；不跳过检查。
- 证据放 docs/release/evidence/<ID>/。
```

### `(Release)`

```text
工作方法（Release）：
- 在干净 Windows/CI 环境验证；记录版本、commit SHA、资产 hash。
- 不覆盖用户数据；升级、卸载、回滚必须演练。
- 发布说明只写已验证能力；不宣称未实现功能。
- 证据放 docs/release/evidence/<ID>/。
```

## 4. Definition of Ready

一张卡进入 `Ready` 前必须同时满足：

- [ ] 标题含稳定卡片 ID 和英文分类标签。
- [ ] `结果` 是可观察的用户/系统结果，不是活动描述。
- [ ] `范围` 是机器可读的 `kandev-scope` 块，包含 write/read/forbid。
- [ ] `验收` 有命令 + 期望结果，或截图规格 + 规则 id。
- [ ] `证据` 有固定路径。
- [ ] `非目标` 至少 3 条。
- [ ] 依赖已写入 Kandev `blocked_by`，正文不重复完整列表。
- [ ] 分类与泳道一致；跨 Core/UI 的工作已拆卡。
- [ ] 没有 `TBD`、`可选`、`你确认` 这类不可执行表述。
- [ ] Parallel cards have disjoint machine-readable `scope.write` sets; overlapping or unclear ownership is serial, and real ordering is recorded with `blocked_by`.
- [ ] Each parallel task has an isolated checkout: Windows Gate uses one ovnfs checkout per card; VPS Dev uses the Worktree executor. Never share a checkout.
- [ ] Windows Gate defaults to serial and allows at most two independent work cards; Windows GUI/build/E2E/final integration always runs serially.
