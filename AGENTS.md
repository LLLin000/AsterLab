# AsterLab Agent 指南

本文件约束所有在本仓库工作的 agent。产品语义和已实现契约仍以 `docs/README.md` 列出的权威文档为准；本文件只补充工作方式与规划边界。

## 1. Kandev 是项目的规划与执行应用

Kandev 承载 AsterLab 的全部**任务级规划**：路线图、里程碑、优先级、依赖、卡片范围、验收条件和执行状态。仓库里不再另建 TODO、roadmap 或 backlog 文档；需要新工作项时，先在 Kandev 建卡，再开始实现。

- 本地地址：`http://127.0.0.1:38429`
- 工作区：`asterlab`（`b67b6785-db21-46de-bd68-c0c437823798`）
- 仓库：`LLLin000/AsterLab`，默认分支 `main`
- API token 只放在环境变量或密钥存储中，禁止写入仓库、任务描述、日志或提交。
- 卡片 ID（`REL-01`、`AG-03`、`DES-02`、`RUN-04`、`QA-05`、`RC-01` 等）写在标题前缀和描述里，是当前引用标识；改标题时保留它。分支名、PR 标题、提交信息和任务评论都应带上卡片 ID。
- 任务必须绑定仓库后才能启动。只传 `workspace_id` 创建的任务不会自动继承工作区仓库，API 会返回 `runner_ineligible_reason: "no_repository"`。

### 权威边界

| 事实 | 权威来源 |
|---|---|
| 任务范围、优先级、依赖、里程碑、验收、执行状态 | Kandev 卡片 |
| 领域术语与对象边界 | `CONTEXT.md` |
| 已实现的 CLI/HTTP 字段、枚举、错误码 | `AGENT_API.md` |
| 架构与模块职责 | `docs/architecture.md` |
| 当前实施阶段与未完成项 | `docs/implementation-plan.md` |
| UI 规则 | `docs/product/ux-contract.md` |
| 模块体系与组合铁律 | `docs/asterdoc-contract.md` |

### 模块与架构硬规则（AsterML v2）

- **能用现有通用内容模块（`prose`、`group_table`、`param_table`、`figure`、`data_table`、`checklist`、`calculator`、`timer`、`attachment`、`subflow`、`plate`）与第二层科学原语组合实现的需求，严禁新增实验专用模块、实验专用字段或实验专用 UI 面板。**
- **图片 / SVG 模块（`figure@1`）仅负责补充表达，绝不承担结构化实验数据职责**（分组、参数、读数必须存于科学数据模块中，不得从图像文字解析）。
- **步骤内自由组合与同源渲染**：Design 允许在任意 `step@1.blocks[]` 中自由插入和排列通用模块，支持自包含冻结为 Run（无需预先手动创建外部 `ProtocolBlockVersion` UUID）；Run 和 Notebook 消费同一份冻结模块树与 `RecordProjection`。

Kandev 卡片与权威文档冲突时，不允许在实现中静默选择一方。先在卡片评论中记录冲突，改卡或改文档对齐后再动代码。

## 2. 任务卡写法

所有 Kandev 卡必须遵循 [`docs/engineering/kandev-card-template.md`](docs/engineering/kandev-card-template.md)。标题格式为 `<卡片 ID> (<Category>) <中文标题>`，分类只使用 `(Docs)`、`(Audit)`、`(Contract)`、`(Core)`、`(UI)`、`(Integration)`、`(QA)`、`(Build)`、`(Release)`。

- 一张卡只有一个分类；需要同时改 Core 和 UI 时拆卡，用 `blocked_by` 连接。
- `范围` 使用机器可读的 `<!-- kandev-scope -->` 块；`REL-04` 的范围检查只解析该块。
- `验收` 必须是命令 + 期望结果，或截图规格 + 规则 id；禁止 `TBD`、`可选`、`你确认`。
- 描述只保留结果、范围、实施、验收、证据、非目标和分类工作方法；逐步清单放任务 Plan。
- 进入 `Ready` 前逐条核对模板第 4 节的 Definition of Ready。

## 3. 当前工作流

| 流程 | 泳道 | 步骤 | 执行者 |
|---|---|---|---|
| VPS Dev | `lane:vps` | Inbox → Ready → Agent → Review → Verify → Merged | 本机 omp agent |
| Windows Gate | `lane:win` | Todo → Windows Work → Windows Verify → Done | Windows 人工/受控环境 |

- **VPS Dev** 承担 Core、CLI、Rust/Go 非 GUI 包、契约、文档和 Linux 可跑的测试。
- **Windows Gate** 承担 MyGo/Slint GUI、`runs.dll`、Windows 安装包和 PowerShell E2E。当前该流程没有 agent profile，也没有注册 Windows executor，是人工泳道；不要把它当作自动队列，也不要在 VPS 上假装执行。
- **检查按平台分工**：VPS 泳道的必需检查是 `linux-tests` 与 `scope-check`（跑在本机自托管 runner）；`windows-build` 只在 Windows 机器上验证（Windows Gate 泳道），不作为 VPS PR 的必需检查。
- **泳道之间不建 `blocked_by` 边**：依赖是产品的执行顺序，不是泳道的运行约束；跨泳道顺序写进下游卡片正文的 `排序:` 一行，由人工泳道自己遵守。跨泳道边没有自动化去执行它，却会被实现 agent 读到并误判成自己被阻塞（2026-11-14 `DES-01 → DES-00` 的边让 Core 卡空转多轮）。
- 卡片的 `lane` 标签和所在流程不一致时，先对齐再执行。

### Windows Gate 执行方法

- Windows Gate 是人工泳道，不启动 Linux/VPS agent；开工前检查 Kandev 卡片的 `blocked_by`、范围和基线，依赖未完成时不开始。
- 每张卡只用独立 `ovnfs` checkout，严格遵循卡片基线；不改共享脏 checkout，不复用其他卡分支或工作区。
- 默认串行；Kandev `Windows Work` WIP=4 是上限。只有 `scope.write` 不重叠且数据隔离时才并行；GUI/native build/截图/E2E/最终集成始终串行。
- 完成后附 PR/分支、验收原始输出与证据；满足卡片验收后才移至 `Done`。详细操作见 [`cross-platform-builds.md` §10](docs/engineering/cross-platform-builds.md#10-windows-gate-task-execution)。

### UI 工作方法（强制）

- 动任何界面之前先读 [`docs/product/design-system.md`](docs/product/design-system.md)：它规定了唯一 token 源、4 倍数间距、三档圆角、需求→组件对照表（Do/Don't）与文案规则。
- 组件按语义选，不自造：先查 `github.com/oneo-me/onevra/source/components`（本地副本 `~/go/pkg/mod/github.com/oneo-me/onevra/source@*/components/`）与仓库既有包；同类控件全项目只有一种用法。
- 通用颜色/圆角/控件高度取自 `internal/theme` 的 `Tokens`；字号用文本 token 或 `t.Rem`，间距用间距 token 或 `t.Space`；科学语义颜色只能复用 `docs/product/design-system.md` §1 登记的具名调色板；调用点不写字面值。
- 需要新组件时，先确认仓库与 onevra 都没有等价实现，并在 `docs/product/design-system.md` 的组件表登记原因。
- 视觉相关 PR 必须带 `## 设计一致性` 小节（用了哪些 token、复用了哪些组件、新增了什么及原因），规则 id 用 `UX-DESIGN`。

### 状态语义

- `Inbox` / `Todo`：待人工准入，不会被 agent 自动领取。
- `Ready`：依赖感知候选队列。Home 控制器（`engctl auto --dispatch`，30 秒一轮）只把**无阻塞**的 VPS 卡从 `Inbox` 提升到 `Ready`，并在 `Agent` 有空位时移入。不要手动把 blocked 卡放进 `Ready`：Kandev 原生 `pull_from` 不会跳过 blocked 卡，会直接启动 agent。旧 `scripts/kandev-ready-feeder.py` 已停用（2026-11-15 起），不要再按 feeder 描述理解调度。
- `Agent`：进入即自动启动 agent；WIP=4，最多四个 agent 并行。并行任务必须使用 Worktree executor；共享 checkout 只能串行。一轮正常结束会自动进入 `Review`。
  - 实现 agent **不得**调用 Kandev 的 `move_task`/`update_task_state` 改本卡步骤（包括自己挪到 `Review`/`Merged`/`Done`）。流转由引擎的 turn 结束、Home 控制器或人工决定；违反会让卡在 PR 合并前被置为完成。
- `Review`：独立 review session（进入即新开，`profile_session_start_policy: new`）按分类决定审查方法：只审核、不改代码、不推送、不合并；通过时输出 `VERDICT: APPROVED` 并用 `engctl review outcome record` 记录 ReviewOutcome，合并由 Home Merge Gate 执行。WIP=4；满 4 个时控制器停止推进新卡。详见 [`docs/engineering/kandev-review-policy.md`](docs/engineering/kandev-review-policy.md)。
- `Verify`：在目标机器上按卡片验收层级重跑并贴原始输出；`on_pr_merged` 会把任务推进到 `Merged`。
- `Merged`：PR 合并后的终态，进入即完成任务。依赖卡只在这里之后解除阻塞。
- 注意 `state` 与步骤是两套东西：`state` 枚举只有 `CREATED`/`IN_PROGRESS`/`REVIEW`/`COMPLETED`，没有 `MERGED`；`Merged` 只是步骤名。依赖只认 `COMPLETED`，步骤到了 Merged 但 `state` 没到 `COMPLETED` 的卡仍会阻塞下游（2026-11-17 EXP-13/EXP-15 教训）。手工修状态只允许打合法枚举值。

### 通知与升级（Telegram @kandev00bot）

- 通知由 Home 控制器发出（`engctl auto --notify` 经 `TelegramTransport`），旧 `kandev-telegram-callback` 服务已下线，三按钮合并回调已失效；合并走 Home Merge Gate。
- 决策类实时推：`REVIEW: APPROVED`、返工熔断（`human-hold`）、卡住升级（`stuck-escalated`）、CHECK_FAILED 等。
- 卡住自修：无信号轮次经 Ready 重试，上限 2 次后移 Hold 并通知人工；RUNNING 超 600 秒无更新发一次 `STALLED` 通知，不停转。
- 每天 08:00 日报：24h 合并、Review 结论、熔断/自修、失败 session、看板快照。
- 配置在 `/etc/kandev-ready-feeder.env`（`TELEGRAM_BOT_TOKEN`/`TELEGRAM_CHAT_ID`，不进仓库）。

## 4. 开工前必须做

1. 读取对应 Kandev 卡片，确认 `范围`、`验收`、`依赖`、`lane` 和 `milestone`。
2. 读取 `CONTEXT.md`、`docs/architecture.md`、`docs/product/ux-contract.md`、`docs/product/ownership.md` 和 `HANDOFF-VPS.md` 第 6 节。
3. 检查 `blocked_by`：只用工具**实际返回**的字段判断。`list_tasks_kandev` 只返回任务概要（state、prs 等），不含关系字段；看不到 `blocked_by` 就按“无前置依赖”处理，不要从别的卡的 `depends_on`、卡片的 `非目标` 或“留给 X 卡”这类措辞反推自己被阻塞；卡片正文写了“无前置依赖”时直接开工。依赖未完成时不要绕过，也不要提前改该卡范围之外的文件。
4. 只修改卡片 `范围` 列出的文件。范围不足时先在卡片评论说明并退回，不得擅自扩围。
5. 不复用其他卡的分支或工作区；一卡一分支，避免并发写同一文件。
6. 如果收到的步骤提示词里还有未被替换的 `{{...}}` 占位符，先停下并在任务评论中指出；不要按字面量创建分支、文件或路径。

## 5. 完成与交接

- 运行卡片要求的验收命令，把**原始输出**贴到任务评论或 PR 描述，不要只说“通过了”。
- 提交到 `agent/<卡片ID>-<slug>` 分支，不直接推 `main`；推送后开 PR，并在任务里附 PR 链接。
- 需要 GUI、Windows 安装包或 Win32 验证的工作，停止当前泳道，在任务里说明并转 `Windows Gate`。
- 不移动 frozen tag（`sdk-v1-rc3`），不修改 frozen Run，不跑全仓 `cargo fmt`。
- 任务完成后只移到 `Review`；`Verify` 和 `Done` 由人工或明确指定的验证者推进。

## 6. 快速入口

```bash
# 服务状态
systemctl status kandev --no-pager

# 任务 API（token 放在 KANDEV_API_TOKEN）
curl -s -H "Authorization: Bearer $KANDEV_API_TOKEN" \
  http://127.0.0.1:38429/api/v1/workspaces/b67b6785-db21-46de-bd68-c0c437823798/tasks \
  | python3 -m json.tool
```

## 7. Engineering Home（调度与合并门）

`LLLin000/home` 是跨项目规范、项目注册与 VPS 调度/合并入口。当前已接入：

```bash
set -a
. /etc/kandev-ready-feeder.env
set +a
scripts/kandev-home-status.sh   # 只读；等价于 engctl status --project asterlab
```

- Kandev 使用方法（地址、API、状态机、排查）见 Home 的 [`docs/kandev-guide.md`](https://github.com/LLLin000/home/blob/main/docs/kandev-guide.md)；Web UI 在 `https://dev.asterlab.party`。
- 实际调度者是 Home 控制器常驻进程（`engctl auto --dispatch --notify` + `engctl merge-worker`，30 秒一轮），旧 feeder 服务已停用。
- `gh pr merge` 在本机被拦截，所有合并走 `engctl merge drive`；零 PR 的 Audit 卡走 `engctl completion audit`。
- 详细安装和迁移边界见 [`docs/engineering/engineering-home-integration.md`](docs/engineering/engineering-home-integration.md)。
- Home 不是任务权威：任务范围、依赖和状态仍以 Kandev 为准；项目技术语义仍以本仓库权威文档为准。
- 不要直接写 Kandev SQLite 或改变合并权限；状态修复用 Kandev API 并随后对账。

## 8. 内测反馈与拉取规范（Beta Feedback）

当用户给出 `FB-YYYYMMDD-HHMMSS` 反馈编号并要求「拉取修复」或「看看问题」时，**所有新会话按 [`docs/engineering/beta-feedback-workflow.md`](docs/engineering/beta-feedback-workflow.md) 执行**：

- **反馈源**：私有仓库 `LLLin000/asterlab-feedback` 的 `submissions/FB-*/`（包含 `feedback.md`、`manifest.json`、`logs/recent.log` 与 `shots/*.png`）；客户端通过 VPS 中继 `https://dev.asterlab.party/api/feedback` 上传（仅 `-tags=devdebug` 内测包启用，正式发行包严格 `-tags=mygo_noinspector`）。
- **拉取边界**：统一 clone/pull `LLLin000/asterlab-feedback` 到会话 `scratch` 临时目录读取 `feedback.md` 并逐张查看截图，**禁止把原始反馈截图或真实 Key/Token 提交进主仓库**。
- **本地缓存优先级**：内测端优先写 `<ActiveVault>/.asterlab/cache/beta-feedback`（上传或另存导出后自动清空）；若本机 exe 从带 `Low Mandatory Level` 的目录编译运行且遇到保存 `Access is denied`，先用 `icacls <exe> /setintegritylevel Medium` 排查令牌完整性级别。


