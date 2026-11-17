# UI-REC-PDF-10 记录归档 PDF 导出（含实拍图板）

- 卡：`UI-REC-PDF-10`（Windows Gate，IN_PROGRESS）
- 基线：`origin/main`（`75541e2`），分支 `feat/ui-rec-pdf-10`
- 写范围：`internal/protocol/archive.go`（新增 `Archive.PDF`＋`ImageLoader`）、
  `protocol_page.go`（新增"导出 PDF"按钮＋`recordPDFLoader`＋`exportRecordBytes`）、
  `internal/protocol/archive_test.go`、`protocol_page_test.go`、本目录
- 读范围：`UI-REDESIGN.md §2.3`、`internal/notebook/record/`、`internal/assets/assets.go`、
  `internal/asterdoc/pdf.go`、`docs/product/evidence/UI-REC-10/README.md`

## 做了什么

终态归档新增**独立的 PDF 导出选项**，与 `.md` 导出并列，Markdown 导出字节不变：

- `Archive.PDF(w, load)`：同一 `RecordProjection` 的另一种渲染——来源头
  （Run/设计 rev/状态/导出时间/签核行）、计划-vs-实际参数对照、签核后展平正文
  （`@matrix(...)` 经投影求解，与 `Markdown()` 取同一 `SourceRefs[0]`／`All`
  切片规则）、派生参数，以及**嵌入式实拍图板**（代表矩阵每个已填充槽：
  `Well · Timepoint · FOV n — 文件名` Caption ＋ PNG 内嵌图）。
- 图片只认 Core 坐标：投影已按 `WellID/Timepoint/FOVIndex` 归槽，
  `PDF` 只遍历槽位；`load` 由页面层接 `assets.Roots.Resolve`（相对路径解析，
  永不从文件名推断）＋共享解码器（TIFF 经已注册的 `x/image/tiff` 解码）
  ＋`png.Encode` 转 gofpdf 可嵌入格式；无新依赖（gofpdf＋标准库）。
- 任一图片不可用／不可解码 → 导出直接失败，错误点名资产
  （`record PDF image "img-x" (A1 · 6h · FOV 2) unavailable: …`），
  页面以 Toast 原文展示；**永不静默丢图**。
- 确定性：PDF 创建／修改戳固定（`2000-01-01`），相同输入（含 `ExportedAt`）
  → 字节相同；CJK 字体沿用 `asterdoc/pdf.go` 的系统字体候选
  （Windows `simhei.ttf`），无字体机器回退 Helvetica（中文退化但图板与参数照常导出）。

## 测试（本卡范围）

```powershell
cd prototypes/mygo-run
CARGO_TARGET_DIR=D:/.ci-cache/cargo-target GOCACHE=D:/.ci-cache/go `
  go test ./internal/protocol/... -count=1
CARGO_TARGET_DIR=D:/.ci-cache/cargo-target GOCACHE=D:/.ci-cache/go `
  go test -run 'TestProtocolNever|TestProtocolArchive' . -count=1
```

- 通过：`internal/protocol` 全过（含新增 4 项：`TestArchivePDFDeterministic`
  字节稳定＋`%PDF-` 头、`TestArchivePDFEmbedsImages` 解析 `/Subtype /Image`
  对象数＝2、`TestArchivePDFMissingAssetNamesIt` 缺图点名、
  `TestArchivePDFNilLoaderWithImagesFails` 未接线点名）；
  页面 3 项全过（归档切换断言新增"导出 PDF"按钮）。
- 既有 `TestArchive*`、`TestNotebook*`、`TestProtocolNever*` 全过；
  `Markdown()` 未改（`git diff` 无 Markdown 行变更）。
- 本 checkout 失败、与本卡无关（原生绑定陈旧，
  `native: the implementation of Runs is older than its sources`，
  同 UI-REC-10 记录）：`internal/asterdoc` 的 `TestPlateMixPlanDerivesGelAndSeedingRows`、
  顶层 `TestProtocolBlockPreviewFixture`；需 coordinator 在 Windows Gate 做
  `mygo native build` 后重跑。

## 设计 caveats

- PDF 版式是"归档包"不是"排版复刻"：文本流式＋每图独立 Caption 块，
  一图一块、按矩阵顺序排列；不做多栏拼图（gofpdf 坐标拼图在缺槽时易错位，
  Caption＋单列最抗缺图）。
- 无字体机器中文退化是已知让步：判据是"图板与参数永不丢"优先于"异体机排版一致"；
  门禁机为 Windows（`simhei.ttf` 存在），实际不受影响。
- `x/image/tiff` 由主包已有空白导入提供解码能力，
  `protocol` 包自身只用标准库 `image/png` 做 `DecodeConfig` 量尺寸，
  `go.mod` 未动。

## 留给 coordinator 人工验证（Windows Gate，本卡不伪造 GUI 证据）

1. GUI 真窗：冻结分支切"终态归档"，点"导出 PDF"，含 TIFF 图的 Run
  （复用 UI-REC-10 门禁库 A1/6h/FOV1 PNG＋FOV2 TIFF）应得带两图的 PDF；
  拔掉 `asset_roots` 后导出，Toast 应点名缺失资产。
2. 全量套件＋原生模块检查（`mygo native build` 后）。
3. 真窗截图附本目录（本卡未生成 GUI 截图；`ui.Tester` 覆盖见上）。
