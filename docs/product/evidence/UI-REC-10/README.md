# UI-REC-10 记录板块同源双渲染：自动图板与终态归档

- 卡：`UI-REC-10`（Windows Gate，IN_PROGRESS）
- 基线：`origin/main`（含 #162），分支 `feat/ui-rec-10-record-dual-render`
- 写范围：`prototypes/mygo-run/notebook_page.go`、`protocol_page.go`、
  `internal/notebook/record/`（新建）、`internal/protocol/archive.go`（新建）、本目录

## 做了什么

同源读模型（新建，不可变事实之外无第二推导）：

- `internal/notebook/record/`（leaf 包，只依赖 `modules/runs`＋标准库，
  供 Notebook 图板、Protocol 归档、Run 历史三方导入，不形成
  `notebook→asterdoc→protocol` 环）：
  - `record.Project(snap, assets)`：唯一的计划-vs-实际摘要与
    Well × Timepoint × FOV 矩阵推导。图片归槽只认 Core 坐标
    （`WellID/Timepoint/FOVIndex`，与 Run 画廊 `assetMatrix` 同规则），
    缺坐标进未定位桶，永不猜；FOV 1–6 槽，空槽即缺。
  - `@matrix(6h,12h,24h)`／`@matrix(*)` 解析、按投影求解、导出时展平。
  - `record.Sign/Verify/StripSignature`：签核附在正文本身的 HTML 注释尾，
    走既有 `SaveNotebook` 持久化，无新 Core 命令；锁＝签名可验证。
  - `record.DeriveDesign`：实测参数→新设计草案（含来源），`JSON()` 交接。
  - 状态词：`未保存 / 已保存 / 已签核归档`（`RecordState`）。
- `internal/protocol/archive.go`：`BuildArchive`＋`Markdown()` 终态归档包
  （来源头、计划vs实际、代表矩阵、展平正文、派生参数、签核行）。
- `notebook_page.go`：第 5 个模式**图板**（摘要＋FOV 1–6 矩阵＋引用插入＋
  派生＋签核/解锁＋导出）；签核后强制阅读态并锁编辑态（工具栏、快捷键、
  `markNotebookDirty` 三处守卫），已签核打开即阅读态；正文仍只从
  Notebook 写（引用插入、签核均为正文写操作，无测量录入控件）。
- `protocol_page.go`：冻结分支新增**冻结文档／终态归档**切换；归档读同一
  投影＋签核态＋导出 `.md`。无 AsterDoc 分支与预览分支行为不变。

## 设计一致性（`## 设计一致性`）

- UX-IA-4 / §1.2：记录事实只读，研判正文只在 Notebook 写；归档与图板是
  同一投影的两种渲染，无第二历史存储，无事实写控件。
- UX-P1：保存仍在首屏工具栏；图板是独立模式，不挤占正文模式。
- UX-P4：记录主体是表格/矩阵行，不是卡片堆。
- UX-STATE / §3：全表面统一 `未保存 / 已保存 / 已签核归档`；失败有因＋重试
  （沿用既有 `noteErr`＋重试行）。
- UX-P2：默认视图无裸 id（矩阵只显示孔位/时间点/文件名）；坐标缺失显式
  列为"未定位图"。
- LB-03 不回退：`notebook.Build` 时间线原位不动；投影只接管摘要/矩阵推导。
- LB-15 不碰：无测量录入控件新增；错误/恢复沿用既有草稿与 Core 拒绝路径。

## 给 UI-RUN-10 的契约（同源另一半）

Run 历史审计渲染请从 `internal/runworkspace` 改读
`record.Project(snap, assets)` 的 `Summary`（计划-vs-实际行）与
`Matrix`（审计对账），替代 `BuildViewModel` 内各自的摘要推导；
条目级时间线仍归 `notebook.Build`。`assets` 取 `a.runAssets`
（`[]runs.Asset`，坐标在 `Link.WellID/Timepoint/FOVIndex`）。

## 测试（本卡范围）

```powershell
cd prototypes/mygo-run
go test ./internal/notebook/... ./internal/protocol/... -count=1
go test -run 'TestNotebook|TestProtocol' . -count=1
```

- 通过：`internal/notebook`、`internal/notebook/record`（投影/矩阵/引用/签核/
  派生/计数口径/无测量孔位成格）、`internal/protocol`（归档展平/签核保持/
  预览戳稳定/计数口径）、页面新增 5 项
  （图板渲染＋FOV 切换、签核锁定/解锁、密封模式锁定、归档切换、签核态）。
- 既有 `TestNotebook*`、`TestProtocolNever*` 全过。
- `TestProtocolBlockPreviewFixture` 在本 checkout 失败，原因系原生绑定
  陈旧（`native: the implementation of Runs is older than its sources`），
  与本卡无关，需 coordinator 在 Windows Gate 做 `mygo native build` 后重跑；
  本卡未改 `modules/runs`、`crates/**`、`src/**`。

## 已确认的范围缺口（不在本卡写范围内，需 follow-up 卡）

- PDF 归档包：设计文档要求导出 PDF 自动展平为实拍图板＋参数对照，但既有
  `asterdoc.ExportPDFWithRun`／`ExportFrozenPDFWithRunRecords` 只渲染文本行
  （`renderPDFLine` 无图片嵌入），不能满足实拍图要求。本卡不扩展写范围
  去实现图片版 PDF，归档导出保持 `.md`；PDF 由 coordinator 另起 follow-up。

## 留给 coordinator 人工验证（Windows Gate，不在本卡执行）

1. GUI 真窗 1360×880 与 900×600：图板模式（摘要、FOV 1–6 切换、缺槽显示）、
   签核→锁定→解锁、归档切换＋导出 `.md`、派生 JSON 导出。
2. 全量套件＋原生模块检查（`mygo native build` 后）。
3. 截图附本目录（本卡未在 Linux 生成 GUI 截图；`ui.Tester` 覆盖见上）。
