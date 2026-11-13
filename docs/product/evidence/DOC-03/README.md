# DOC-03 Evidence — Frozen Protocol and Run PDF Export

## Baseline and Scope
- Repository: `LLLin000/AsterLab`, base commit `4a75c40` (`SDK-04 (Core) 统一结构化诊断输出 (#29)`)
- Isolated checkout: `ovnfs` `doc03-v2` (`O:\doc03-v2`), isolated runtime dir `C:\Users\Lin\AppData\Local\Temp\doc03-verify\data`
- Scope files modified:
  - `prototypes/mygo-run/internal/asterdoc/pdf.go`
  - `prototypes/mygo-run/internal/asterdoc/pdf_test.go`
  - `prototypes/mygo-run/main.go`
  - `prototypes/mygo-run/main_test.go`
  - `docs/release/pdf-export.md`
  - `docs/product/evidence/DOC-03/`

## UX Contract Rule Citations
- `UX-P1` (Task first): Run primary recording controls remain first; `导出 PDF` stays secondary (`components.ButtonSecondary`) and visible without scrolling at both `1360×880` and `900×600`.
- `UX-P2` (Progressive disclosure): Run revision, freeze timestamp, planned/actual parameters, grouped tasks, and event readouts live in the exported PDF and collapsible frozen document viewer without cluttering the default Run/Protocol views.
- `UX-P3` (One component language): Export button, spinner toast (`正在导出 PDF…`), error toasts (`无法导出 PDF` / `PDF 导出失败`), and success toast (`PDF 已保存` with `打开 PDF`) use `components.*` and `c.Theme()` tokens.
- `UX-P5` (Explicit, recoverable state): `导出 PDF` disables while exporting or on frozen doc load error; export writes to a temporary file, syncs, and atomically renames on success while cleaning up on error or dialog cancel.
- `UX-LAYOUT` / `UX-DONE`: Verified on Windows at `1360×880` and `900×600` in both light and dark themes.

## Automated Checks
- `cd prototypes/mygo-run && go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native check ./modules/runs` → exit 0 (`Built Runs for windows/amd64`, `Native module ./modules/runs is fine`)
- `cd prototypes/mygo-run && CGO_ENABLED=0 go build -tags=e2e -trimpath -o dist/mygo-run.exe .` → exit 0
- `cd prototypes/mygo-run && go test ./... -count=1` → exit 0 (`20 packages ok, 4 no tests`), including:
  - `TestExportFrozenPDFUsesVerifiedFrozenContent`
  - `TestExportFrozenPDFWithRunRecordsPaginatesDeterministically` (including 8-iteration CJK byte-equality check)
  - `TestRenderRunRecordsIncludesGroupedAndUnassociatedData`

## Real Windows GUI Exports (3 Runs + Repeat Byte Determinism)
Exported directly through the built Windows GUI (`dist/mygo-run.exe`) by clicking `导出 PDF` via `mygo-automation` and confirming the native Win32 Save dialog (`toast=True`, `PDF 已保存`):

| Case | File | Pages | Size (bytes) | SHA-256 | Notes |
|---|---|---|---|---|---|
| Run 1 (Short CCK-8) | `doc03-run1-short.pdf` | 2 | 23362 | `938FD50C26C095B0B26E80188C8764489C6D10F999B8936EF56DE0B0C6777A31` | Short Chinese protocol + freeze timestamp + planned/actual param + measurement + check |
| Run 1 (Repeat export) | `doc03-run1-short-repeat.pdf` (temp) | 2 | 23362 | `938FD50C26C095B0B26E80188C8764489C6D10F999B8936EF56DE0B0C6777A31` | Exact byte-for-byte match (`SHA-256` identical) |
| Run 2 (Medium Grouped RNA) | `doc03-run2-grouped.pdf` | 3 | 34109 | `45DB6FA512AAB34619A88DF773EA1D2B98EC694B0368F701B2346C0917507964` | Grouped sub-steps expanded (`01 收集细胞` → `02 裂解离心` → `03 RNA 洗脱` → `qPCR 扩增`), material use, deviation, lineage, unassociated observation |
| Run 3 (Long Multi-Page CJK) | `doc03-run3-long.pdf` | 5 | 40708 | `44EB1BE115A0C6621C4101EAA16731B15E5A107D710CB5DE26DB4830F87290FE` | 25 Chinese bullet items + 60 OD450 well measurements across 5 pages, clean CJK font embedding (`C:/Windows/Fonts/simhei.ttf`), zero broken lines or missing glyphs |

### Field-by-Field UI Parity
Verified against Run workspace (`runworkspace.View`) and rendered PDF pages (`Windows.Data.Pdf.PdfDocument`):
- Frozen metadata: `Document Revision`, `Source Hash`, `Linked Run`, `Run Revision`, `冻结时间` (RFC 3339 from Core `CreationAudit.RecordedAt`, fallback `未记录（旧版 Run）`)
- Plan & modules: Markdown prose outside `asterlab` fences, `Experiment Plan`, static module previews
- Grouped/child tasks: expanded in workspace order without duplicating group container headers
- Planned & actual parameters, measurements (`SampleID · Field Value Unit`), event timestamps/types/contents, checks, materials, deviations, lineage edges, and unassociated records all match UI values.

## Real-Window Screenshots
- `run-1360x880-light.png` / `protocol-1360x880-light.png` / `run-exported-1360x880-light.png`
- `run-1360x880-dark.png` / `protocol-1360x880-dark.png`
- `run-900x600-light.png` / `protocol-900x600-light.png`
- `run-900x600-dark.png` / `protocol-900x600-dark.png`
