# AsterLab 内测反馈工作流与新会话接手指南（Beta Feedback）

> 状态：已实现 · 适用于所有新开启的 Agent 会话与维护者

本文件说明 AsterLab 内置可开关的**可视化截图标注反馈模式**（随所有标准发行包一同编译，默认关闭，可通过底部状态栏「反馈」、`设置 → 关于 → 诊断与反馈`、`Ctrl+K` 命令面板或 `Ctrl+Shift+D` 随时开启）如何采集、中继与落盘反馈，以及**新开启的 AI 会话在收到类似 `FB-20261124-xxxxxx` 的反馈编号时如何一键拉取、排查与重打发行包**。

---

## 1. 系统架构与仓库边界

| 环节 | 说明 | 权威位置 / 参数 |
|---|---|---|
| **内置反馈模式（默认关闭）** | 无须单独安装独立内测包；标准发行构建（`-tags=mygo_noinspector` 仅剥离 MyGo 底层 F12 检查器）已内置可视化红框拾取、弹窗批注、队列管理与直传能力，默认 `FeedbackMode=false`（零悬浮打扰、零启动磁盘 I/O） | `prototypes/mygo-run/dev_inspector_enabled.go`、`internal/shell/{feedback_welcome,settings}.go`、`internal/devfeedback/` |
| **VPS 中继服务** | 写只读授权中继，接收客户端提交的 JSON/截图并推送至私有反馈仓库 | `https://dev.asterlab.party/api/feedback`（`ssh vps` 上 `asterlab-feedback-relay.service`，监听 `127.0.0.1:38435`，Caddy 反代） |
| **密钥与凭据管理** | 仅存在于 VPS 与 GitHub Secrets，**严禁写入代码仓库、提交记录、日志或终端回显** | VPS `/etc/asterlab-feedback-relay.env`（`FEEDBACK_UPLOAD_KEY`）；GitHub Secrets `BETA_FEEDBACK_KEY` / `BETA_FEEDBACK_ENDPOINT` |
| **私有反馈存储库** | 存储所有已上传的用户内测截图、环境元数据与问题清单 | 私有仓库 `LLLin000/asterlab-feedback`，目录格式 `submissions/FB-<YYYYMMDD-HHMMSS>-<tester>-<version>/` |
| **单次提交包结构** | 每次上传在独立目录中包含完整上下文，无需向用户反复追问 | `feedback.md`、`manifest.json`、`submission.json`、`logs/recent.log`、`shots/ui-*-crop.png`、`shots/ui-*-full.png` |

### 客户端本地临时缓存与自动清理策略

为避免反馈文件长期堆积污染系统或实验库，内测客户端按以下优先级解析临时会话目录（`resolveDevFeedbackDir` in `prototypes/mygo-run/dev_inspector_enabled.go`）：

1. `<ActiveVault>/.asterlab/cache/beta-feedback`（**首选**，跟随当前打开的实验库，用户完全可控，且避开 Windows 企业终端在 `C:\Users\*` 下的权限/DLP 拦截）
2. 可执行文件同级 `feedback/`（排除 macOS `.app/Contents/MacOS` 包内部）
3. `os.UserCacheDir()/AsterLab/beta-feedback`
4. `os.TempDir()/AsterLab-BetaFeedback`

- 切换 Vault 时自动调用 `resetDevFeedbackDirCache()` 重新绑定目录并水合草稿。
- 用户**上传到服务器成功**或通过**系统文件框手动导出（`mygo.Dialog.Save`）**后，自动清空本地临时 `live-session/`，不留垃圾文件。

---

## 2. 新 Session 标准拉取与排查 SOP

当用户给出任何 `FB-YYYYMMDD-HHMMSS` 编号并说「拉取 `FB-...` 修复」或「看看 `FB-...`」时，新会话按以下顺序执行：

### Step 1：在仓库外拉取最新反馈（严禁将截图提交进 `AsterLab` 主仓库）

将 `LLLin000/asterlab-feedback` 克隆或更新到当前对话的 scratch/临时目录（不要放进项目根目录）：

```powershell
# 若首次拉取（把 <scratch> 换为会话临时目录）
gh repo clone LLLin000/asterlab-feedback "<scratch>\asterlab-feedback"

# 若已存在，直接更新
git -C "<scratch>\asterlab-feedback" pull --ff-only
```

### Step 2：通读文案 + 必看截图

1. 用 `read_file` 读取 `submissions/FB-xxxx*/feedback.md` 全文。
2. 逐张用 `read_file` 查看 `submissions/FB-xxxx*/shots/ui-*-crop.png`（必要时查看 `ui-*-full.png` 核对整体排版与上下文坐标），不要仅凭文字猜测 UI 表现。

### Step 3：深挖代码根因与区分讨论点

1. **区分两类问题**：
   - 明确的功能 Bug / 性能卡顿 / 布局溢出：追查真实代码根因（避免表面打补丁），形成修复方案。
   - 产品取舍或交互重构建议（如某个标记语法或组件是否冗余）：在实现前先与用户对齐共识，再动手改代码。
2. **在独立特性分支实现并跑全套本地门禁**：
   - `python scripts/sdk/check.py`
   - `python scripts/ci/check_ui_hygiene.py`（必须 0 违规）
   - `cd prototypes/mygo-run && go build -tags=mygo_noinspector . && go build -tags="e2e,devdebug" .`
   - `go test -count=1 ./internal/... ./cmd/... ./e2e` 及相关根包视觉/交互测试
3. 开 PR 时按要求附带 `## 设计一致性` / `UX-DESIGN` 小节，等待 `gh pr checks <PR> --watch` 通过后合入 `main`。

---

## 3. 本地重打内测版（含回传密钥与 Windows 完整性陷阱）

修复合并至 `main` 后，如需立刻给本机或内测朋友出新包：

### 3.1 获取密钥并构建（严禁打印 `$key`）

```powershell
$key = (ssh vps "sudo grep '^FEEDBACK_UPLOAD_KEY=' /etc/asterlab-feedback-relay.env | cut -d= -f2-").Trim()
if (-not $key) { throw "Missing FEEDBACK_UPLOAD_KEY" }
$commit = (git rev-parse --short HEAD).Trim()

$env:BETA_FEEDBACK_ENDPOINT = "https://dev.asterlab.party/api/feedback"
$env:BETA_FEEDBACK_KEY = $key

# 1. 本机测试直跑单文件（版本号按轮次递增，如 r8）
$ldflags = "-s -w -H=windowsgui " +
  "-X 'asterlab-mygo-prototype/internal/updater.Version=0.1.10-devdebug-r8' " +
  "-X 'asterlab-mygo-prototype/internal/updater.Commit=$commit' " +
  "-X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadEndpoint=https://dev.asterlab.party/api/feedback' " +
  "-X 'asterlab-mygo-prototype/internal/devfeedback.DefaultUploadKey=$key'"
Push-Location prototypes/mygo-run
go build -tags="devdebug" -ldflags $ldflags -o dist-dev/asterlab-dev-r8.exe .
Pop-Location

# 2. 外发朋友的便携包（Windows / macOS）
.\scripts\mygo-package-devbeta.ps1 -Version "0.1.10-dev"
# macOS 环境下对应运行：./scripts/mygo-package-devbeta-macos.sh
```

> 打包前若存在旧版遗留的 `prototypes/mygo-run/dist-dev/feedback/`，应先清理，确保新版本启动干净。

### 3.2 Windows `Low Mandatory Level` 保存失败排查提醒

如果本机工作区曾运行过 Codex Windows Sandbox（或在父目录留下了可继承的 `Mandatory Label\Low Mandatory Level` ACE），新编译出的 `.exe` 会继承 `Low` 完整性级别（`RID=0x1000`），触发 Windows 内核 `NO_WRITE_UP` 规则——导致应用写入 `Medium` 级别的 Vault 目录（如 `<Vault>/.asterlab/pantry.jsonl`、`.asterml` 文档或 `.asterlab/cache/beta-feedback`）全部报 `Access is denied`。

- 验证命令：`icacls <exe路径>` 查看是否存在 `Mandatory Label\Low Mandatory Level`。
- 立即修复：`icacls <exe路径> /setintegritylevel Medium`（`.zip` 便携包不带 NTFS 标签，用户解压到普通目录不受此影响）。

---

## 4. 历史反馈轮次索引

| 反馈批次 | 版本 | 状态 | 关联 PR / 备注 |
|---|---|---|---|
| `FB-20261124-003507` | `0.1.10-devdebug-r5` | 已全部修复合入 | PR #205（`34d5b58`），15 项（SVG 双标签、周历虚拟化、Alt 缓存、Pantry 回滚等） |
| `FB-20261124-012505` | `0.1.10-devdebug-r6` | 已全部修复合入 | PR #206（`5418267`），6 项（`UI-16`~`UI-21`：储藏室顶栏、公式极简、可视化 `/`、计时器齐宽、周历时段、Alt 记忆化 + 联用配方）及库内缓存改造 |
| `FB-20261124-110016` | `0.1.10-devdebug-r7` | 已全部修复合入 | PR #207，6 项（`UI-01`~`UI-06`：Callout 渲染、`#t0` 降噪、96 孔板排枪稀释、极简计时器、读数汇总确定性修复、IME/Live Preview 零截断） |
| `FB-20261124-113233` | `0.1.10-devdebug-r8` | 修复并纳入本次审计 | 2 项：`UI-01` 系列稀释 96 孔板 `# @vessel` 持久化、去冗余切换按钮与 `T6` 防裁切；`UI-02` SVG `<text>` 基线/缩放修复与长图注换行 |
| `FB-20261124-132219` | `0.1.10-devdebug-r10` | 已修复并升级界面质感 | 3 项 + 性能/质效升级：`UI-01` 右键菜单改用 `1×1` 光标锚点使组件内左键可即点关闭；`UI-02` 96 孔板排枪稀释渲染完整 8×12 `plateui.View` 孔板+移液流向；`UI-03` 多培养皿拓扑（`dish` 且 `rows×cols > 4`）渲染独立 90 mm 圆形平板矩阵；并消除滚动帧全量重解析与工具栏遮挡 |
| `FB-20261124-222743` | `0.1.10-devdebug-r14` | 已全部修复合入 | PR #217（`ea949cc`），5 项（多输出计算器换行卡片化、附件图库分孔直传、速算+快捷计时垂直分层、周视图 5 分钟精度与磁吸统一、新建实验标准路径）+ 启动异步预热 |
| `FB-20261125-003847` | `0.1.11-unsigned` | 已全部修复合入 | PR #217（`ea949cc`），空白孔板二维选区等比梯度稀释写入 |
| `FB-20261125-102534` / `FB-20261125-103716` / `FB-20261125-103901` | `0.1.11-unsigned` | 已全部修复合入 | PR #218（`eeb4809`），覆盖 Issue `#216` 全部 3 项（子步骤空起手、新建试剂继承温区与顶栏防换行、模板/目录长列表滚动与搜索、弹窗下周视图紫线屏蔽、设置页默认视图选项防溢出）并随 Windows 发行包打包 24MB 离线 ONNX 模型；PR #219（`e7f0123`）落地隐形实验管家体验升级 |
