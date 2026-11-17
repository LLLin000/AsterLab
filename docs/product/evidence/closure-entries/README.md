# CLOSE-07 证据 — 统一功能入口状态与安装 DeepLink 出口

基线：`origin/main` @ `6010ab9`（CLOSE-06 合并后），fresh ovnfs checkout `close07`。
本卡只做功能与入口收口，不做配色/字号/美学重设计。

## 1. 六表面入口矩阵（UX-IA-1）

已复核仍存在的问题（`6010ab9` 上均存在，已修）：

- 两个设盲开关并存：`runworkspace/view.go runHeader`（顶栏）与
  `runworkspace/bench.go renderBlindToggle`（执行台面）同调
  `s.toggleBlind()`。已删除 bench 开关及其 `分组已隐藏` 行；
  顶栏开关是唯一入口，仍消费 Core `run.workspace view=blind|unblinded`
 （`blind_test.go` 覆盖 Core 确认/拒绝路径）。
- 样本命令常驻：`样本排除`、`分类与表格读数` 对每个任务都渲染。
  现按任务能力呈现：`taskCarriesSamples`（measurement plan / 已记录读数 /
  采图槽位）为真才渲染；纯操作步骤只保留通用`偏差与跳过`。
  计时区保持通用（任意步骤可新建计时器是既定功能，空态文案如实说明）。
- 竞争主操作：bench 各提交（推进一轮/按规则跳过/记偏差/记观察/
  记录排除/提交读数）曾全是 Primary。现全部降为 Secondary；
  唯一 Primary 按任务状态归属：读数任务`保存读数`、
  无读数任务`标记完成`；`recording.go` 在 `canMarkComplete` 时把保存
  降为 Secondary，即使同屏也永不竞争。
- 写边界复核（无改动，有证据）：Tools 只产出草稿（prod 未接线任何写
  回调，`handoff` 止于 Draft；pantry 是 Tools 自有物资台账）；
  Protocol 只读（DES-05）；Notebook 散文 + 派生（CLOSE-04 已批）；
  Run 经 Core 命令写事实。`frozenIdentity` 默认折叠（UX-P2，
  `TestProcedureShowsFrozenProtocolReadOnly`）。

截图（tester 真实渲染，精确尺寸）：

- `entries-readout-1360x880-light/dark.png`、`entries-readout-900x600-light/dark.png`：
  读数任务 — 执行台面/计时/偏差与跳过/样本排除/分类与表格读数/保存读数
  齐备，`标记完成` 与各表单提交均不出现，设盲开关仅顶栏一处。
- `entries-procedure-1360x880-light.png`、`entries-procedure-900x600-dark.png`：
  纯操作任务 — 样本区隐藏，唯一主操作为`标记完成`。

## 2. 全局 Timer 与诊断折叠（UX-LAYOUT，UX-P2）

- `shell.go timerStrip`：按紧急度排序（剩余最少的倒计时在前），默认只
  显示 2 个当前/即将到期计时器，其余收进`＋n 更多`，展开后每项仍经
  `OnTimerSelect` 定位到 Run，可`收起`。只用 Ghost 按钮与主题 token，
  不新增颜色/字号。`TestTimerStripCollapsesOverflow` 覆盖折叠/展开/
  跳转/收回；原有两计时器用例不受影响。
- 层级诊断默认折叠：`flowRail` 的 `TreeIssues`（如 `…have no design step`）
  收进`层级诊断（n）` Collapsible，一击展开。`ShowTreeIssues` 默认关。

## 3. 安装器 URI 与统一导出（UX-IA-3，UX-P5）

- `scripts/mygo-installer.nsi`：注册用户级（HKCU）`Software\Classes\asterlab`，
  `URL Protocol`，`open/command = "<INSTDIR>\mygo-run.exe" "%1"`。
  不拼 shell、不提权；卸载段 `DeleteRegKey HKCU ...\asterlab`。
  `makensis` 编译通过（含注册段）。
- 安装/卸载往返（stub exe，同 RC-01 流程）：安装后注册表四键齐全、
  快捷方式与卸载项齐全；`uninstall.exe /S` 后安装目录/URI 键/卸载项/
  快捷方式全部消失，宿主无残留（`%LOCALAPPDATA%\AsterLab` 未动，
  本次验证只用隔离目录）。
- 严格解析（`internal/state`，无 shell 执行）：路由仅
  `asterlab://run/<id>`（打开 Run）与
  `asterlab://run/<id>/well/<孔位>`（附带选孔），其余一律可见错误；
  `deeplink_test.go` 覆盖大小写/空格归一、非法孔位、尾随段、空路径。
  运行中应用跳转：`main.go` 起服即取单实例锁，第二个实例（外部点击）
  转发参数后退出；首实例经 `RunOnMain` 解析同一 `openRunWellLink`
  并聚焦窗口。两条路径共用同一 resolve 函数。
- 统一导出（UX-IA-1）：冻结/预览文档出口改名`导出方案 PDF`
 （`main.go documentExportButton`）；终态归档出口为`导出归档包` /
  `导出归档 PDF` /（已签核）`导出校验包`（`protocol_page.go`）；
  Notebook 重复的归档包/PDF 行已删除（文案本就写“归档包在实验方案页
  导出”）。同一含义只有一个入口，同一名称只有一个含义。

## 4. 未在本卡验证（诚实缺口，转 Gate）

- 已装 exe 的真实外部点击拉起与双实例跳转：解析、单实例转发、聚焦
  代码就绪并经单测覆盖，但未用已装 exe 做两次真实窗口验证
  （构建 exe 已就绪：`dist/mygo-run.exe` SHA256
  `545F714C2AAD9F121BD603833D73E53FBE05C728545A07A81A0F070F80`，
  `Runs.dll` SHA256 `2EA905CB0155D5164BAD1CEF33174B8241BADD31B84240`）。
  RC-01 的安装/首启/卸载链路仍是唯一可用的实机发射证明。
- 原生文件 picker 点击链：沿 CLOSE-06 惯例未验证。
- `design_canvas.go` / `design_page.go` 的 Design 表面 Primary
  （`删除步骤`/`进入子流程`/`重试`）超出本卡写范围，未动。
- Tools→Run 的“发送到当前 Run 任务”第二 handoff：Tools 当前只送草稿
  （符合“只送草稿”），该 handoff 仍缺，属后续功能卡范畴。

## 测试

`cd prototypes/mygo-run`（`GOCACHE=D:/.ci-cache/go`）：

- `go test ./internal/shell/... ./internal/state/... ./internal/runworkspace/... ./internal/tools/... -count=1` → 全过
- `go test . -count=1`（根包，main/protocol/notebook 改动）→ 通过
- `CLOSE07_EVIDENCE=1 go test ./internal/runworkspace -run TestClose07EntriesEvidence` → 6 张截图
- 证据 PNG 由 `ui.Tester` 在 1360x880 / 900x600、浅/深色下真实渲染。

## Windows Gate 独立验证（`6923fd4`，coordinator 执行）

- 二进制：门禁构建（`BUILD_INFO` commit `6923fd4` 与 head 一致；`LOCAL_GATE: PASS`）。
- 双实例跳转（`close07-deeplink-jump-1360x880-light.png`）：实例 A 开工作区，实例 B 带 `asterlab://run/<id>/well/A1` 启动——B 瞬退（rc=0），A 跳到 Run 页并选中 A1。原生 URI 注册表项由安装器写入（RC-01 覆盖 uninstall 清理）。
- 单盲入口：Run 页恰 1 个设盲开关；导出命名统一（`导出方案 PDF`，无裸 `导出 PDF`）。
