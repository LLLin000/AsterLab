# REL-03 分支差异审计：`feature/v2-run-workspace` vs `main`

- 卡片：REL-03 (Audit) Run Workspace 分支差异审计
- 状态：只读审计快照。本文档是唯一产出；不改代码、契约、分支或 tag。
- 审计日期：2026-11-13，worktree 分支 `feature/rel-03-audit-run-wor-7g8` @ `423f881`
- 钉住引用（复核时以此为准，不随 main 前进漂移）：
  - `origin/main` = `4a3780b`（审计基准，默认分支最新）
  - `origin/feature/v2-run-workspace` = `5e20027`（远端分支未重新推送）
  - 本地 `main` = `423f881`（审计开始时落后 origin/main 1 个纯文档提交）
- 前序产物：本地分支 `agent/REL-03-branch-audit` @ `cdc06e6` 保存了本卡上一轮的 647 行草稿（未推送、未合入）。本轮逐条复核后重写钉住引用；附录 A 的 28 个文件清单与 `git show --name-only` 28/28 一致。

## 结论

1. `origin/feature/v2-run-workspace` 是 `main` 的**祖先**，不是待合并分支。`git merge-base` 返回分支 tip 本身 `5e20027`；`main..feature` = 0 个 commit，`feature..main` = 11 个 commit（本地 `main`），相对 `origin/main` 为 12 个。
2. 分支无独有 commit、无独有文件：`git diff --stat main...origin/feature/v2-run-workspace` 为空，`--diff-filter=DR`（只存在于分支的删除/重命名）为空。
3. 处置：整条分支 **drop**，不 merge、不 rebase。分支携带的 28 个 commit 全部为 **drop**（`main` 已逐字包含），没有任何 commit 需要 redo。
4. 风险：对该分支再 merge 是 no-op；若把分支内容「重做」成新提交，会覆盖 `main` 上 11 个后续提交，其中 `436b826`、`102390d` 已把工作流树投影迁入 Rust Core（`crates/asterlab-core/src/workflow_tree.rs`），重做会回退该迁移。
5. 本卡未改代码、未切换/删除分支、未移动 tag；唯一仓库写入是本文档。`git fetch origin --tags` 补齐了本 worktree 原先缺失的 4 个 tag ref，既有 tag 未移动。

## 基线

| 引用 | SHA | 说明 |
|---|---|---|
| `main`（本地，审计开始时） | `423f881` | 落后 origin/main 1 个仅文档提交；仍包含分支 tip |
| `origin/main` | `4a3780b` | 默认分支最新；本审计与 PR 的基准 |
| `origin/feature/v2-run-workspace` | `5e20027` | 被审计分支 tip |
| merge-base | `5e20027` | 等于分支 tip，故分支是 main 的祖先 |

## 1. Commits

### 1.1 授权命令的原始输出

审计开始时（本地 `HEAD` = `feature/rel-03-audit-run-wor-7g8` @ `423f881`）：

```text
$ git fetch origin
(无输出；origin/main 仍为 423f881)

$ git fetch origin --tags
From https://github.com/LLLin000/AsterLab
   423f881..4a3780b  main        -> origin/main
 * [new tag]         sdk-v1-rc1  -> sdk-v1-rc1
 * [new tag]         sdk-v1-rc2  -> sdk-v1-rc2
 * [new tag]         sdk-v1-rc3  -> sdk-v1-rc3
 * [new tag]         v0.1.0-rc.1 -> v0.1.0-rc.1

$ git log --oneline --reverse main..origin/feature/v2-run-workspace
(无输出)

$ git diff --stat main...origin/feature/v2-run-workspace
(无输出)

$ git status --short
(无输出)

$ git diff --stat
(无输出)
```

`main..feature` 与 `main...feature` 同时为空是同一事实的两面：三点差异只比较 merge-base 之后的分支一侧，而分支一侧没有任何 commit。

计数与祖先关系：

```text
$ git rev-list --count main..origin/feature/v2-run-workspace
0
$ git rev-list --count origin/feature/v2-run-workspace..main
11
$ git rev-list --count origin/feature/v2-run-workspace..origin/main
12
$ git merge-base main origin/feature/v2-run-workspace
5e20027ce6d0cf53e104beb740830ccb914da9e8
$ git merge-base --is-ancestor origin/feature/v2-run-workspace origin/main; echo $?
0
$ git merge-base --is-ancestor origin/feature/v2-run-workspace main; echo $?
0
```

`origin/main` 在分支 tip 之后新增的 12 个提交：

```text
$ git log --oneline origin/feature/v2-run-workspace..origin/main
4a3780b docs: require Worktree executor for parallel Kandev agents
7c79ed5 docs: reflect the two-agent Kandev feeder and parallel scope rule
2d6ab10 docs: fix the Kandev card template and roadmap reconciliation
320d5a0 docs: add agent guide for the Kandev planning workflow
35a87d5 test(g1): add the RNA hierarchy fixture and Windows gate evidence
102390d feat(workflow): route GUI hierarchy through Core
436b826 feat(core): project workflow hierarchy without the desktop app
2052731 docs: define the Linux and Windows build workflow
df0dcd3 fix(vps): guard the RC-3 fixture data directory
8eb197c docs(vps): build the host-native Runs library before Go tests
3216303 fix(asterdoc): find a CJK font on Linux
```

### 1.2 分支自身血缘：28 个 commit

`origin/feature/v2-run-workspace` 没有 merge commit（`git log --merges origin/main` 为空），是线性历史。tip `5e20027` 的父链上，`feature/v2-document-pdf` tip `d6bd918` 之后的 28 个 commit 是该分支携带的全部工作：

```text
$ git rev-list --count d6bd918..origin/feature/v2-run-workspace
28
```

依据 A（下表全部适用）：该 commit 是分支 tip 的祖先，tip 是 `main` 的祖先（`--is-ancestor` 退出码 0），因此每个 commit 的内容都已逐字包含在 `main` 中；不需要逐个判断补丁是否被改写。

下表 `files` / `+` / `−` 取自 `git show --numstat`（重命名与同一文件多次修改会重复计数）；`落点` 列出 `main` 上修改同一文件或接续同一工作流的提交，仅作定位，不表示分支内容缺失。

| # | commit | 日期 | 主题 | files | + | − | 处置 | 理由 | 目标卡 / 落点 |
|---|---|---|---|---|---|---|---|---|---|
| 1 | `eb627cf` | 2026-11-11 | feat(run): Run Workspace execution surface + unified recording module + local asset SDK | 9 | 2227 | 0 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 在其上继续改 `runworkspace/model.go`、`runs` 库 |
| 2 | `837e36f` | 2026-11-11 | feat(run): render runworkspace as the Run page; document the SDK module catalog | 3 | 269 | 0 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 改 `main.go` |
| 3 | `e315a81` | 2026-11-11 | feat(sdk): make module renderers an executed contract | 11 | 813 | 58 | drop | 祖先，已含于 main | 无；`main` 无后续提交改这些渲染器文件 |
| 4 | `a20fc92` | 2026-11-11 | fix(sdk): enforce renderer mode in frozen module view | 3 | 67 | 37 | drop | 祖先，已含于 main | 无 |
| 5 | `d649ed1` | 2026-11-11 | feat(run): freeze Run UI v1 with plate-first recording | 52 | 4198 | 1095 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 改 `runworkspace/model.go`、`main.go`、`src/cli.rs` |
| 6 | `12e68fe` | 2026-11-11 | test(sdk): add RNA extraction and RT-qPCR containment adaptivity case | 17 | 873 | 0 | drop | 祖先，已含于 main | 无新卡；`main 35a87d5` 补 RNA fixture 与 G1 证据 |
| 7 | `1aaec28` | 2026-11-11 | fix(run): restore measurement, check, event, and session correctness | 24 | 779 | 124 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 改 `runworkspace/model.go`、`src/cli.rs` |
| 8 | `3f0e358` | 2026-11-11 | feat(sdk): add read-only workflow hierarchy projection and RFC | 5 | 832 | 0 | drop | 祖先，已含于 main | 无新卡；投影已由 `main 436b826`/`102390d` 迁入 Rust Core |
| 9 | `0a0e9d9` | 2026-11-11 | feat(run): harden hierarchy projection and add Run tree navigation | 17 | 745 | 179 | drop | 祖先，已含于 main | 同上 |
| 10 | `6608ab1` | 2026-11-11 | refactor(run): use the existing collapsible tree for Run navigation | 14 | 152 | 91 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 改 `runworkspace/view.go`、`main.go` |
| 11 | `d4224f8` | 2026-11-12 | fix(run): separate display order from dependencies and harden session identity | 6 | 138 | 35 | drop | 祖先，已含于 main | 无新卡；`main 436b826`/`102390d` 改 `workflowtree/tree.go` |
| 12 | `dfd3ada` | 2026-11-12 | feat(design): add read-only ProtocolBlock hierarchy projection | 13 | 976 | 79 | drop | 祖先，已含于 main | 同上 |
| 13 | `abb8c75` | 2026-11-12 | feat(design): add one-level read-only Canvas drill-down | 16 | 412 | 19 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 改 `design_canvas.go` 同层文件 |
| 14 | `f67c382` | 2026-11-12 | feat(design): make the Canvas subflow readable | 24 | 658 | 120 | drop | 祖先，已含于 main | 同上 |
| 15 | `c3382d4` | 2026-11-12 | feat(agent): expose read-only workflow.inspect_tree | 12 | 413 | 39 | drop | 祖先，已含于 main | 无新卡；`main 436b826`/`102390d` 改 `inspect_tree.go`、`src/cli.rs` |
| 16 | `991129c` | 2026-11-12 | docs: record final Track E commit hashes | 1 | 1 | 1 | drop | 祖先，已含于 main | 无 |
| 17 | `ca06c55` | 2026-11-12 | fix(workflow): make inspect_tree run-scoped and identity-safe | 7 | 369 | 66 | drop | 祖先，已含于 main | 无新卡；`main 436b826`/`102390d` 改 `inspect_tree.go`、`tree.go` |
| 18 | `3823c8d` | 2026-11-12 | fix(workflow): join frozen tasks to design steps by block identity | 10 | 299 | 33 | drop | 祖先，已含于 main | 无新卡；`main 436b826`/`102390d` 改 `tree.go`、`src/cli.rs` |
| 19 | `b267d3f` | 2026-11-12 | feat(sdk): require an exact ProtocolBlock source for executable steps | 20 | 848 | 98 | drop | 祖先，已含于 main | 即冻结 tag `sdk-v1-rc3` 指向的提交（HANDOFF-VPS §4）；tag 不随分支处置变化 |
| 20 | `0cdb55d` | 2026-11-12 | docs: record the product and SDK review baseline | 6 | 651 | 0 | drop | 祖先，已含于 main | 无 |
| 21 | `e90cdbc` | 2026-11-12 | docs(product): environment gate, file ownership, three product boundaries | 4 | 234 | 19 | drop | 祖先，已含于 main | 无 |
| 22 | `2ec8831` | 2026-11-12 | docs(product): finish the D: environment gate | 1 | 50 | 11 | drop | 祖先，已含于 main | 无 |
| 23 | `0ab2300` | 2026-11-12 | feat(product): Protocol becomes surface 6 and page bodies leave main.go | 12 | 699 | 592 | drop | 祖先，已含于 main | 无新卡；`main 102390d` 改 `main.go` |
| 24 | `a7661b0` | 2026-11-12 | feat(product): page-level UX pass for Run, Design, Notebook and Protocol | 21 | 251 | 128 | drop | 祖先，已含于 main | 同上 |
| 25 | `0b27fbd` | 2026-11-12 | feat(sdk): sdk.validate, rule registry, shared parity fixture, list editing | 6 | 367 | 7 | drop | 祖先，已含于 main | 无新卡；`main 436b826`/`102390d` 改 `src/cli.rs` |
| 26 | `ad19bbe` | 2026-11-12 | feat(sdk): ship rna_reagent_usage@1 as the extensibility proof | 12 | 336 | 6 | drop | 祖先，已含于 main | 无 |
| 27 | `a1a84d2` | 2026-11-12 | docs(product): record the final integration gate | 6 | 13 | 0 | drop | 祖先，已含于 main | 无 |
| 28 | `5e20027` | 2026-11-12 | docs: add the VPS handoff and make the acceptance fixture reproducible | 2 | 358 | 0 | drop | 分支 tip，本身就在 main 上 | 无；`main df0dcd3`/`2052731` 在其后修 `rc3-fixture.py`、`HANDOFF-VPS.md` |

合计：28 条 drop，0 条 merge，0 条 redo。逐条 `+` 合计 18,028、`−` 合计 2,837，高于 §2.3 的净差异（16,696 / 1,505），因为中间提交反复改写同一行；合并口径看净差异。

### 1.3 每 commit 的变更文件

见[附录 A](#附录-a每个-commit-的变更文件)。

## 2. Files

### 2.1 只在分支上的文件差异

```text
$ git diff --stat main...origin/feature/v2-run-workspace
(无输出)

$ git diff --name-only --diff-filter=DR origin/main...origin/feature/v2-run-workspace
(无输出)
```

结论：0 个文件只在分支上存在，0 个删除/重命名只存在于分支。分支不持有任何未落地的文件内容。

### 2.2 `main` 独有差异（`feature..origin/main`）

```text
$ git diff --shortstat origin/feature/v2-run-workspace..origin/main
 39 files changed, 3348 insertions(+), 110 deletions(-)
```

这不是「分支缺失的内容」，而是 `5e20027` 之后 `main` 的演进（12 个提交）；其中 8 个是 G1 证据 PNG（二进制），0 个删除/重命名。

```text
$ git diff --name-status origin/feature/v2-run-workspace..origin/main
A	AGENTS.md
M	HANDOFF-VPS.md
M	README.md
M	crates/asterlab-core/src/lib.rs
A	crates/asterlab-core/src/workflow_tree.rs
A	docs/engineering/cross-platform-builds.md
A	docs/engineering/kandev-card-template.md
A	docs/engineering/kandev-roadmap-reconciliation.md
A	docs/product/evidence/g1-design-drill-1360-light.png
A	docs/product/evidence/g1-design-drill-900-light.png
A	docs/product/evidence/g1-design-root-1360-dark.png
A	docs/product/evidence/g1-design-root-1360-light.png
A	docs/product/evidence/g1-run-1360-dark.png
A	docs/product/evidence/g1-run-1360-light.png
A	docs/product/evidence/g1-run-900-dark.png
A	docs/product/evidence/g1-run-900-light.png
M	docs/sdk/design-hierarchy-track-e.md
M	docs/sdk/workflow-hierarchy-rfc.md
M	docs/sdk/workflow-tree-contract-v1.md
M	prototypes/mygo-run/NATIVE_MODULE.md
M	prototypes/mygo-run/design_hierarchy.go
M	prototypes/mygo-run/design_hierarchy_test.go
M	prototypes/mygo-run/inspect_tree.go
M	prototypes/mygo-run/inspect_tree_test.go
M	prototypes/mygo-run/internal/asterdoc/pdf.go
M	prototypes/mygo-run/internal/runworkspace/model.go
M	prototypes/mygo-run/internal/state/fake.go
M	prototypes/mygo-run/internal/workflowtree/tree.go
M	prototypes/mygo-run/main.go
M	prototypes/mygo-run/modules/runs/runs.go
M	prototypes/mygo-run/modules/runs/runs_native.go
M	prototypes/mygo-run/modules/runs/rust/src/lib.rs
M	prototypes/mygo-run/modules/runs/rust/src/mygo.rs
M	prototypes/mygo-run/pathbound_tree_test.go
A	scripts/kandev-ready-feeder.py
A	scripts/mygo-native-linux.sh
M	scripts/rc3-fixture.py
A	scripts/rna-fixture.py
M	src/cli.rs
```

其中 `436b826`、`102390d` 修改了 12 个分支引入的文件（`runworkspace/model.go`、`workflowtree/tree.go`、`inspect_tree.go`、`design_hierarchy.go`、`state/fake.go`、`main.go`、`modules/runs/*`、`src/cli.rs` 等）。这是 drop 而非 redo 的直接理由：分支内容是这些提交的**输入**，已被取代。

### 2.3 分支引入的文件面（`d6bd918..5e20027`）

```text
$ git diff --shortstat d6bd918..origin/feature/v2-run-workspace
 182 files changed, 16696 insertions(+), 1505 deletions(-)
$ git diff --name-status d6bd918..origin/feature/v2-run-workspace | cut -f1 | sort | uniq -c
    128 A
     54 M
```

按目录（182 个文件，含 68 个二进制/图片/sqlite）：

| 目录 | 文件数 |
|---|---|
| `prototypes/mygo-run/**` | 78 |
| `docs/sdk/**` | 70 |
| `docs/product/**` | 22 |
| `src/cli.rs` | 1 |
| `crates/asterlab-core/**` | 1 |
| `crates/asterlab-app/**` | 1 |
| `scripts/**` | 4 |
| `tests/asterdoc_parity_samples.json` | 1 |
| `AGENT_API.md`、`HANDOFF-VPS.md`、`AsterLab.cmd`、`docs/asterdoc-contract.md` | 4 |

改动量最大的文件：`src/cli.rs` (+1342/−44)、`internal/runworkspace/view.go` (+1046)、`internal/runworkspace/model.go` (+698)、`internal/workflowtree/tree.go` (+633)、`main.go` (+630/−687)、`internal/resultgrid/plate.go` (+539)。

裁定：这 182 个文件的最终形态已全部在 `main` 中；没有需要抢救或补交的内容。

## 3. Uncommitted

| 时刻 | `git status --short` | `git diff --stat` | 说明 |
|---|---|---|---|
| T0 审计开始（`HEAD` = `feature/rel-03-audit-run-wor-7g8` @ `423f881`） | 空 | 空 | `git stash list` 空；无 staged/unstaged/untracked 项 |
| T1 fetch 后 | 空 | 空 | `git fetch origin` 无输出；`git fetch origin --tags` 把 `origin/main` 推进到 `4a3780b` 并补入 4 个 tag ref |
| T2 写入本文档 | `?? docs/release/branch-audit.md` | 空 | 本卡唯一产出；未 stage 其他文件 |
| T3 提交后 | 空 | 空 | 与 T0 一致 |

补充事实：

- 审计期间另一卡向 `origin/main` 推了 1 个纯文档提交 `4a3780b`（`docs: require Worktree executor for parallel Kandev agents`）。本地 `main` 未同步，因此 `feature..main` = 11、`feature..origin/main` = 12；drop 结论不由该差值决定。
- 本 worktree 原先没有 tag ref（`git tag -l` 为空）。`git fetch origin --tags` 补入 `sdk-v1-rc1`、`sdk-v1-rc2`、`sdk-v1-rc3`、`v0.1.0-rc.1`；`sdk-v1-rc3` 仍解析为 annotated tag `e602838` → commit `b267d3f`，未移动、未删除。
- 本卡提交只含 `docs/release/branch-audit.md`，落在任务 worktree 分支 `feature/rel-03-audit-run-wor-7g8`；未移动 `HEAD` 到别的分支，未新开分支。
- 本地另有分支 `agent/REL-01-beta-scope`（`71a4b5e`，已推送）与 `agent/REL-03-branch-audit`（`cdc06e6`，仅本地、上一轮草稿）；本卡未修改、未删除它们。
- 本地 `main` 落后 `origin/main` 1 个纯文档提交（`4a3780b`）。这是只读发现；本卡不合入、不 rebase，是否需要同步由人工或其他卡决定。

## 4. Disposition

| 对象 | 现状 | 处置 | 理由 | 目标卡 |
|---|---|---|---|---|
| 分支 `feature/v2-run-workspace` (`5e20027`) | `main` 的祖先；0 个独有 commit、0 个独有文件 | **drop**（不 merge、不 rebase） | 无内容可合并；保留只会诱导误 merge 或误删后的误重做 | 无；删除远端分支需人工确认（本卡非目标） |
| 28 个 commit（`eb627cf`..`5e20027`） | 全部是 `main` 的祖先，内容逐字包含 | **drop**（逐条见 §1.2） | 依据 A：祖先关系即内容包含；`436b826`/`102390d` 已在其上改造 | 无 |
| `feature..main` 的 39 个文件 | main 侧演进（Core 迁移、G1 证据、VPS 文档、Kandev 工具、CI 脚本） | **保留 main** | 这些提交是分支内容的后继，回退即回归 | 对应各卡（G1、VPS、REL 系列） |
| 冻结 tag `sdk-v1-rc3` → `b267d3f` | 指向分支血缘中的提交；annotated tag `e602838` | **不动** | frozen tag 不可变；分支 drop 不影响 tag 可达性；本地 ref 是 fetch 补入的，非本卡创建 | 无 |
| 本地 `main` 落后 `origin/main` 1 个提交（`4a3780b`） | 只读发现 | **本卡不处理** | 本卡非目标含「不合入、不 rebase」 | 需要时另开 Docs 卡同步 |
| `docs/release/branch-audit.md`（本文档，T2 未提交） | 本卡唯一产出 | **提交到任务 worktree 分支并开 PR** | 卡片要求证据进仓库 | REL-03 |

不做的事（本卡已遵守）：未 merge / rebase / cherry-pick；未删除分支或 tag；未编辑源码、契约、`spec/modules.json`；未把分支内容重建为新提交。

## 复核命令

```bash
git fetch origin
git rev-parse --short main origin/main origin/feature/v2-run-workspace
git merge-base origin/main origin/feature/v2-run-workspace
git rev-list --left-right --count origin/feature/v2-run-workspace...origin/main
git diff --stat origin/main...origin/feature/v2-run-workspace
git diff --name-only --diff-filter=DR origin/main...origin/feature/v2-run-workspace
git rev-list --count d6bd918..origin/feature/v2-run-workspace
git log --oneline --reverse d6bd918..origin/feature/v2-run-workspace
git diff --shortstat origin/feature/v2-run-workspace..origin/main
```

`origin/main` 前进后，把上表的 `4a3780b` 换成当前 `origin/main`：只要 `git merge-base --is-ancestor origin/feature/v2-run-workspace origin/main` 仍返回 0，drop 结论不变。若该命令返回非 0，说明分支被重新推送了新提交，本审计失效，需要重跑。

## 附录 A：每个 commit 的变更文件

```text
### eb627cf feat(run): Run Workspace execution surface + unified recording module + local asset SDK
  prototypes/mygo-run/internal/assets/assets.go
  prototypes/mygo-run/internal/assets/assets_test.go
  prototypes/mygo-run/internal/runworkspace/fixture_test.go
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/runworkspace/model_test.go
  prototypes/mygo-run/internal/runworkspace/recording.go
  prototypes/mygo-run/internal/runworkspace/recording_test.go
  prototypes/mygo-run/internal/runworkspace/view.go
  prototypes/mygo-run/internal/runworkspace/view_test.go
### 837e36f feat(run): render runworkspace as the Run page; document the SDK module catalog
  docs/sdk/README.md
  docs/sdk/module-catalog-v1.md
  prototypes/mygo-run/main.go
### e315a81 feat(sdk): make module renderers an executed contract
  docs/sdk/module-catalog-v1.md
  docs/sdk/module-contract-v1.md
  prototypes/mygo-run/internal/asterdoc/evidence-asterdoc.png
  prototypes/mygo-run/internal/asterdoc/evidence-plate-zoom.png
  prototypes/mygo-run/internal/asterdoc/modules.go
  prototypes/mygo-run/internal/asterdoc/registry.go
  prototypes/mygo-run/internal/asterdoc/renderer_test.go
  prototypes/mygo-run/internal/asterdoc/spec/modules.json
  prototypes/mygo-run/internal/asterdoc/template_test.go
  prototypes/mygo-run/internal/asterdoc/testdata/template-complex.asterml
  prototypes/mygo-run/internal/asterdoc/view.go
### a20fc92 fix(sdk): enforce renderer mode in frozen module view
  docs/sdk/module-contract-v1.md
  prototypes/mygo-run/internal/asterdoc/renderer_test.go
  prototypes/mygo-run/internal/asterdoc/view.go
### d649ed1 feat(run): freeze Run UI v1 with plate-first recording
  AsterLab.cmd
  crates/asterlab-app/src/lib.rs
  docs/asterdoc-contract.md
  docs/sdk/compatibility-and-gaps.md
  docs/sdk/evidence/runui-primary.png
  docs/sdk/module-catalog-v1.md
  docs/sdk/module-contract-v1.md
  docs/sdk/module-development-guide.md
  docs/sdk/run-ui-v1-baseline.md
  docs/sdk/signing-and-release.md
  prototypes/mygo-run/asset_link_persistence_test.go
  prototypes/mygo-run/asterdoc_freeze_test.go
  prototypes/mygo-run/asterdoc_store_test.go
  prototypes/mygo-run/canvas_layout_persist_test.go
  prototypes/mygo-run/cck8_loop_test.go
  prototypes/mygo-run/design_canvas.go
  prototypes/mygo-run/gui_seed_test.go
  prototypes/mygo-run/internal/assays/schema.go
  prototypes/mygo-run/internal/assays/schema_test.go
  prototypes/mygo-run/internal/assays/well_test.go
  prototypes/mygo-run/internal/assets/assets.go
  prototypes/mygo-run/internal/assets/assets_test.go
  prototypes/mygo-run/internal/asterdoc/asterdoc.go
  prototypes/mygo-run/internal/asterdoc/asterdoc_test.go
  prototypes/mygo-run/internal/asterdoc/generic_editor_test.go
  prototypes/mygo-run/internal/asterdoc/modules.go
  prototypes/mygo-run/internal/asterdoc/pdf_test.go
  prototypes/mygo-run/internal/asterdoc/plate_sample_test.go
  prototypes/mygo-run/internal/asterdoc/registry.go
  prototypes/mygo-run/internal/asterdoc/renderer_test.go
  prototypes/mygo-run/internal/asterdoc/spec/foundation-v1.schema.json
  prototypes/mygo-run/internal/asterdoc/spec/modules.json
  prototypes/mygo-run/internal/asterdoc/testdata/cck8-experiment.asterml
  prototypes/mygo-run/internal/asterdoc/view.go
  prototypes/mygo-run/internal/resultgrid/API.md
  prototypes/mygo-run/internal/resultgrid/grid.go
  prototypes/mygo-run/internal/resultgrid/plate.go
  prototypes/mygo-run/internal/resultgrid/plate_edit.go
  prototypes/mygo-run/internal/resultgrid/plate_edit_test.go
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/runworkspace/recording.go
  prototypes/mygo-run/internal/runworkspace/view.go
  prototypes/mygo-run/internal/uikit/plate/plate.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/modules/runs/runs.go
  prototypes/mygo-run/modules/runs/runs_native.go
  prototypes/mygo-run/modules/runs/rust/src/lib.rs
  prototypes/mygo-run/modules/runs/rust/src/mygo.rs
  prototypes/mygo-run/notebook_page_test.go
  scripts/build-mygo.ps1
  scripts/sign-local.ps1
  src/cli.rs
### 12e68fe test(sdk): add RNA extraction and RT-qPCR containment adaptivity case
  docs/sdk/rna-extraction-case/REPORT.md
  docs/sdk/rna-extraction-case/evidence/design-flow-v2.png
  docs/sdk/rna-extraction-case/evidence/design-step01.png
  docs/sdk/rna-extraction-case/evidence/notebook-timeline2.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-design-step01.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-design.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-run.png
  docs/sdk/rna-extraction-case/evidence/run-01.png
  docs/sdk/rna-extraction-case/evidence/run-05-wash1.png
  docs/sdk/rna-extraction-case/evidence/run-06-wash2a.png
  docs/sdk/rna-extraction-case/evidence/run-07-wash2b.png
  docs/sdk/rna-extraction-case/evidence/run-09-elute.png
  docs/sdk/rna-extraction-case/evidence/run-10-qc.png
  docs/sdk/rna-extraction-case/evidence/run-history-empty.png
  docs/sdk/rna-extraction-case/evidence/run-restart.png
  docs/sdk/rna-extraction-case/purelink-rna.asterml
  docs/sdk/rna-extraction-case/rtqpcr-parent.asterml
### 1aaec28 fix(run): restore measurement, check, event, and session correctness
  crates/asterlab-core/src/run_workspace.rs
  docs/sdk/rna-extraction-case/REPORT.md
  docs/sdk/rna-extraction-case/evidence/notebook-restart.png
  docs/sdk/rna-extraction-case/evidence/notebook-run-context.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-run-fixed.png
  docs/sdk/rna-extraction-case/evidence/run-01-fixed.png
  docs/sdk/rna-extraction-case/evidence/run-05-wash1-fixed.png
  docs/sdk/rna-extraction-case/evidence/run-06-history-deviation.png
  docs/sdk/rna-extraction-case/evidence/run-06-wash2a-fixed.png
  docs/sdk/rna-extraction-case/evidence/run-09-elute-fixed.png
  docs/sdk/rna-extraction-case/evidence/run-10-checked.png
  docs/sdk/rna-extraction-case/evidence/run-10-qc-fixed.png
  docs/sdk/rna-extraction-case/evidence/run-10-restart-checked.png
  docs/sdk/run-correctness-track-a.md
  prototypes/mygo-run/internal/runworkspace/fixture_test.go
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/runworkspace/model_test.go
  prototypes/mygo-run/internal/runworkspace/view.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/modules/runs/runs.go
  prototypes/mygo-run/modules/runs/runs_native.go
  prototypes/mygo-run/modules/runs/rust/src/lib.rs
  prototypes/mygo-run/modules/runs/rust/src/mygo.rs
  src/cli.rs
### 3f0e358 feat(sdk): add read-only workflow hierarchy projection and RFC
  docs/sdk/rna-extraction-case/rtqpcr-parent-path-bound.asterml
  docs/sdk/workflow-hierarchy-rfc.md
  docs/sdk/workflow-hierarchy-ui-proposal.md
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/internal/workflowtree/tree_test.go
### 0a0e9d9 feat(run): harden hierarchy projection and add Run tree navigation
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-restart.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-wash2a.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree.png
  docs/sdk/rna-extraction-case/evidence/run-09-elute-regression.png
  docs/sdk/workflow-hierarchy-rfc.md
  docs/sdk/workflow-hierarchy-track-d.md
  docs/sdk/workflow-hierarchy-ui-proposal.md
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/runworkspace/view.go
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/internal/workflowtree/tree_test.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/modules/runs/runs.go
  prototypes/mygo-run/modules/runs/runs_native.go
  prototypes/mygo-run/modules/runs/rust/src/lib.rs
  prototypes/mygo-run/modules/runs/rust/src/mygo.rs
  prototypes/mygo-run/pathbound_tree_test.go
### 6608ab1 refactor(run): use the existing collapsible tree for Run navigation
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-collapsed-restart.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-collapsed.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-restart.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-standard-click-wash2a.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-standard-restart.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-standard.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-wash2a.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree.png
  docs/sdk/workflow-hierarchy-rfc.md
  docs/sdk/workflow-hierarchy-track-d.md
  docs/sdk/workflow-hierarchy-ui-proposal.md
  prototypes/mygo-run/internal/runworkspace/view.go
  prototypes/mygo-run/internal/runworkspace/view_test.go
  prototypes/mygo-run/main.go
### d4224f8 fix(run): separate display order from dependencies and harden session identity
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/internal/workflowtree/tree_test.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/pathbound_tree_test.go
  prototypes/mygo-run/session_test.go
  prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite
### dfd3ada feat(design): add read-only ProtocolBlock hierarchy projection
  prototypes/mygo-run/block_preview_test.go
  prototypes/mygo-run/design_hierarchy.go
  prototypes/mygo-run/design_hierarchy_test.go
  prototypes/mygo-run/internal/state/fake.go
  prototypes/mygo-run/internal/workflowtree/design.go
  prototypes/mygo-run/internal/workflowtree/design_test.go
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/modules/runs/runs.go
  prototypes/mygo-run/modules/runs/runs_native.go
  prototypes/mygo-run/modules/runs/rust/src/lib.rs
  prototypes/mygo-run/modules/runs/rust/src/mygo.rs
  prototypes/mygo-run/pathbound_tree_test.go
### abb8c75 feat(design): add one-level read-only Canvas drill-down
  docs/sdk/design-hierarchy-track-e.md
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-preview.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-return.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-root.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow-restart.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow.png
  docs/sdk/rna-extraction-case/evidence/rtqpcr-tree-standard-regression.png
  docs/sdk/workflow-hierarchy-rfc.md
  docs/sdk/workflow-hierarchy-track-d.md
  docs/sdk/workflow-hierarchy-ui-proposal.md
  prototypes/mygo-run/design_canvas.go
  prototypes/mygo-run/design_hierarchy_test.go
  prototypes/mygo-run/internal/uikit/workflownode/node.go
  prototypes/mygo-run/internal/workflow/model.go
  prototypes/mygo-run/internal/workflow/view.go
  prototypes/mygo-run/main.go
### f67c382 feat(design): make the Canvas subflow readable
  docs/sdk/design-hierarchy-track-e.md
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-preview-1360x880.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-preview-900x600.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-preview.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow-1360x880.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow-900x600.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow-detail-1360x880.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow-detail.png
  docs/sdk/rna-extraction-case/evidence/design-hierarchy-subflow.png
  prototypes/mygo-run/block_preview_test.go
  prototypes/mygo-run/design_canvas.go
  prototypes/mygo-run/design_hierarchy.go
  prototypes/mygo-run/design_hierarchy_test.go
  prototypes/mygo-run/internal/uikit/workflownode/node.go
  prototypes/mygo-run/internal/workflow/model.go
  prototypes/mygo-run/internal/workflow/view.go
  prototypes/mygo-run/internal/workflowtree/design.go
  prototypes/mygo-run/internal/workflowtree/design_test.go
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/modules/runs/runs.go
  prototypes/mygo-run/modules/runs/runs_native.go
  prototypes/mygo-run/modules/runs/rust/src/lib.rs
  prototypes/mygo-run/modules/runs/rust/src/mygo.rs
### c3382d4 feat(agent): expose read-only workflow.inspect_tree
  AGENT_API.md
  docs/sdk/design-hierarchy-track-e.md
  docs/sdk/rna-extraction-case/evidence/inspect-tree-design.json
  docs/sdk/rna-extraction-case/evidence/inspect-tree-run.json
  docs/sdk/workflow-hierarchy-rfc.md
  docs/sdk/workflow-tree-contract-v1.md
  prototypes/mygo-run/inspect_tree.go
  prototypes/mygo-run/inspect_tree_test.go
  prototypes/mygo-run/internal/workflowtree/design.go
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/main.go
  src/cli.rs
### 991129c docs: record final Track E commit hashes
  docs/sdk/design-hierarchy-track-e.md
### ca06c55 fix(workflow): make inspect_tree run-scoped and identity-safe
  docs/sdk/design-hierarchy-track-e.md
  docs/sdk/rna-extraction-case/evidence/inspect-tree-design.json
  docs/sdk/rna-extraction-case/evidence/inspect-tree-run.json
  docs/sdk/workflow-tree-contract-v1.md
  prototypes/mygo-run/inspect_tree.go
  prototypes/mygo-run/inspect_tree_test.go
  prototypes/mygo-run/internal/workflowtree/tree.go
### 3823c8d fix(workflow): join frozen tasks to design steps by block identity
  docs/sdk/README.md
  docs/sdk/evidence/cck8-hierarchy-900x600-dark-v1rc2.png
  docs/sdk/evidence/cck8-hierarchy-900x600-light-v1rc2.png
  docs/sdk/track-a-cck8-evidence.md
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/internal/workflowtree/tree_test.go
  prototypes/mygo-run/modules/runs/rust/src/lib.rs
  scripts/gui-e2e-v11.ps1
  src/cli.rs
### b267d3f feat(sdk): require an exact ProtocolBlock source for executable steps
  docs/sdk/README.md
  docs/sdk/compatibility-and-gaps.md
  docs/sdk/foundation-v1.md
  docs/sdk/module-catalog-v1.md
  docs/sdk/step-binding-v1.md
  docs/sdk/track-a-cck8-evidence.md
  docs/sdk/workflow-hierarchy-rfc.md
  prototypes/mygo-run/internal/asterdoc/asterdoc_test.go
  prototypes/mygo-run/internal/asterdoc/modules.go
  prototypes/mygo-run/internal/asterdoc/registry.go
  prototypes/mygo-run/internal/asterdoc/renderer_test.go
  prototypes/mygo-run/internal/asterdoc/spec.go
  prototypes/mygo-run/internal/asterdoc/spec/modules.json
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/runworkspace/model_test.go
  prototypes/mygo-run/internal/workflowtree/design.go
  prototypes/mygo-run/internal/workflowtree/design_test.go
  prototypes/mygo-run/internal/workflowtree/tree.go
  prototypes/mygo-run/internal/workflowtree/tree_test.go
  src/cli.rs
### 0cdb55d docs: record the product and SDK review baseline
  docs/product/audit-run-notebook-tools.md
  docs/product/audit-shell-design.md
  docs/product/review-2026-11-12.md
  docs/product/ux-contract.md
  docs/sdk/extensibility-probes.md
  docs/sdk/sdk-readiness-audit.md
### e90cdbc docs(product): environment gate, file ownership, three product boundaries
  docs/product/environment-gate.md
  docs/product/ownership.md
  docs/product/review-2026-11-12.md
  docs/product/ux-contract.md
### 2ec8831 docs(product): finish the D: environment gate
  docs/product/environment-gate.md
### 0ab2300 feat(product): Protocol becomes surface 6 and page bodies leave main.go
  docs/product/evidence/wave1-project-900.png
  docs/product/evidence/wave1-protocol-frozen-1360.png
  docs/product/evidence/wave1-protocol-frozen-900.png
  docs/product/evidence/wave1-protocol-preview.png
  docs/product/review-2026-11-12.md
  prototypes/mygo-run/design_page.go
  prototypes/mygo-run/internal/shell/shell.go
  prototypes/mygo-run/internal/shell/shell_test.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/notebook_page.go
  prototypes/mygo-run/protocol_page.go
  prototypes/mygo-run/run_page.go
### a7661b0 feat(product): page-level UX pass for Run, Design, Notebook and Protocol
  docs/product/evidence/wave2-design-900.png
  docs/product/evidence/wave2-design-docked-1360.png
  docs/product/evidence/wave2-notebook-1360.png
  docs/product/evidence/wave2-notebook-900.png
  docs/product/evidence/wave2-protocol-900.png
  docs/product/evidence/wave2-run-1360.png
  docs/product/evidence/wave2-run-900.png
  docs/product/review-2026-11-12.md
  prototypes/mygo-run/design_canvas.go
  prototypes/mygo-run/internal/designer/designer-mapping-roundtrip.png
  prototypes/mygo-run/internal/designer/designer.go
  prototypes/mygo-run/internal/designer/designer.png
  prototypes/mygo-run/internal/designer/designer_test.go
  prototypes/mygo-run/internal/protocol/protocol.png
  prototypes/mygo-run/internal/protocol/view.go
  prototypes/mygo-run/internal/runworkspace/model.go
  prototypes/mygo-run/internal/runworkspace/view.go
  prototypes/mygo-run/internal/runworkspace/view_test.go
  prototypes/mygo-run/main.go
  prototypes/mygo-run/notebook_page.go
  prototypes/mygo-run/notebook_page_test.go
### 0b27fbd feat(sdk): sdk.validate, rule registry, shared parity fixture, list editing
  docs/product/review-2026-11-12.md
  prototypes/mygo-run/internal/asterdoc/list_editor_test.go
  prototypes/mygo-run/internal/asterdoc/parity_fixture_test.go
  prototypes/mygo-run/internal/asterdoc/view.go
  src/cli.rs
  tests/asterdoc_parity_samples.json
### ad19bbe feat(sdk): ship rna_reagent_usage@1 as the extensibility proof
  docs/product/review-2026-11-12.md
  docs/sdk/module-catalog-v1.md
  prototypes/mygo-run/asterdoc_store_test.go
  prototypes/mygo-run/internal/asterdoc/asterdoc_test.go
  prototypes/mygo-run/internal/asterdoc/modules.go
  prototypes/mygo-run/internal/asterdoc/parity_fixture_test.go
  prototypes/mygo-run/internal/asterdoc/registry.go
  prototypes/mygo-run/internal/asterdoc/renderer_test.go
  prototypes/mygo-run/internal/asterdoc/spec.go
  prototypes/mygo-run/internal/asterdoc/spec/modules.json
  src/cli.rs
  tests/asterdoc_parity_samples.json
### a1a84d2 docs(product): record the final integration gate
  docs/product/evidence/wave2-dark-design-900.png
  docs/product/evidence/wave2-dark-notebook-1360.png
  docs/product/evidence/wave2-dark-protocol-900.png
  docs/product/evidence/wave2-dark-run-1360.png
  docs/product/evidence/wave2-dark-run-900.png
  docs/product/review-2026-11-12.md
### 5e20027 docs: add the VPS handoff and make the acceptance fixture reproducible
  HANDOFF-VPS.md
  scripts/rc3-fixture.py
```

## 审计边界

- 只读命令，加 `git fetch origin`（卡片实施第 1 条要求）与 `git fetch origin --tags`（补齐本地缺失的 tag ref，未移动既有 tag）。
- 未执行 `merge`、`rebase`、`cherry-pick`、`checkout`、`reset`、`branch -d`、`tag -d` 等写操作；未新建或删除分支。
- 未编辑源码、契约、`spec/modules.json`；`git status --short` 在审计开始时与提交后为同一状态（空）。
- `sdk-v1-rc3` 仍指向 `b267d3f`（annotated tag `e602838`），未移动。
