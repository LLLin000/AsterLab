# UX-01 Surface Audit

## Build and seeded workspace

- Audited checkout: `O:\ux01`, branch `agent/UX-01-surface-audit`, HEAD `720efd6` (`origin/main` after the RUN-07 merge).
- Rebuilt GUI identity: `prototypes/mygo-run/dist/BUILD_INFO.txt` reports `commit 720efd6`; `mygo-run.exe` SHA-256 `5F03B37690BB8C6D2180E3BB6BCD3BE21371B1F9FCA08EEC60F5EB867B61713F`; `Runs.dll` SHA-256 `EB7DC70EE0A2BE15FE7F87339E05D0F49A46519882BDC98B2B3428D0A8A1CBF3`.
- Seed: `qa-harness.py`'s `seed_qa01()` CLI seed, in isolated data `C:/Users/Lin/AppData/Local/Temp/ux01-audit/qa-qa01/data`. One project (`QA-01 CCK-8 回归`), one design with `酶标仪读数` workflow, frozen Run `0eae89b2-9811-45b4-b85f-0870d0e5a218`, Protocol `QA-01 方案 v1`, two saved readouts (`A1=0.62`, `B1=0.71` OD450 AU), and a saved Notebook observation citing those readouts. CLI `run.workspace` readback confirmed a measurement event with both values and an observation event with the note.
- Screenshots are real Windows windows driven by `mygo-automation`; each PNG is exactly the content size in its filename. Capture dimensions and themes were verified during capture against the rebuilt `720efd6` binary.

## Screenshot coverage

The 12-image budget is allocated to the three workflow surfaces with visible continuation/creation/recording actions: 项目工作区, 实验设计, 实验运行. **Each of those three** has 1360×880 Light + Dark and 900×600 Light + Dark. Notebook, 实验方案, and 资料与工具 were inspected in the live GUI at 900×600 but have no screenshots in this 12-image set.

| Surface | 1360×880 Light | 1360×880 Dark | 900×600 Light | 900×600 Dark |
|---|---|---|---|---|
| 项目工作区 | [PNG](ux01-project-1360x880-light.png) | [PNG](ux01-project-1360x880-dark.png) | [PNG](ux01-project-900x600-light.png) | [PNG](ux01-project-900x600-dark.png) |
| 实验设计 | [PNG](ux01-design-1360x880-light.png) | [PNG](ux01-design-1360x880-dark.png) | [PNG](ux01-design-900x600-light.png) | [PNG](ux01-design-900x600-dark.png) |
| 实验运行 | [PNG](ux01-run-1360x880-light.png) | [PNG](ux01-run-1360x880-dark.png) | [PNG](ux01-run-900x600-light.png) | [PNG](ux01-run-900x600-dark.png) |

## Rule audit

`PASS`/`FAIL` uses the enforceable tests in `docs/product/ux-contract.md` §§2–4. Every FAIL below is an exact defect finding for follow-up card UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`); this audit made no source-code changes.

| Surface | Rule id | Result | Evidence (screenshot + observation) |
|---|---|---|---|
| 项目工作区 | UX-P1 | PASS | [900×600 Light](ux01-project-900x600-light.png), [900×600 Dark](ux01-project-900x600-dark.png): the existing-design card and recent Run card are both visible above the fold, so the prescribed continue/open-design action is reachable. |
| 项目工作区 | UX-P2 | PASS | [1360×880 Light](ux01-project-1360x880-light.png): default cards show project/design/Run titles and revisions, with no raw UUID or step/block id. |
| 项目工作区 | UX-P3 | PASS | [1360×880 Dark](ux01-project-1360x880-dark.png): surfaces and text follow the theme; `internal/shell/shell.go` uses theme colors, `Space`, and `Rem` rather than literal colors/font sizes. |
| 项目工作区 | UX-P4 | PASS | [1360×880 Light](ux01-project-1360x880-light.png): active work appears as a two-column item grid followed by a recent-run list, not a repeated stack of generic cards. |
| 项目工作区 | UX-P5 | FAIL | [900×600 Light](ux01-project-900x600-light.png): the Run card says “执行中” (and “Run · 执行中”) instead of the contract state “进行中”; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 项目工作区 | UX-LAYOUT | FAIL | [1360×880 Light](ux01-project-1360x880-light.png), [900×600 Light](ux01-project-900x600-light.png): the grid has no explicit “show all” affordance required by §4, despite having no hard item cap; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验设计 | UX-P1 | PASS | [900×600 Light](ux01-design-900x600-light.png), [900×600 Dark](ux01-design-900x600-dark.png): “创建 Run” is visible in the initial viewport. |
| 实验设计 | UX-P2 | PASS | [1360×880 Light](ux01-design-1360x880-light.png): the workflow canvas shows the human-readable step title, with no raw UUID/step/block id in the default view. |
| 实验设计 | UX-P3 | FAIL | [1360×880 Light](ux01-design-1360x880-light.png): `design_page.go` and `design_canvas.go` contain hard-coded font sizes (for example `FontSize(16)`, `FontSize(11)`, `FontSize(15)`) instead of theme `Rem` values; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验设计 | UX-P4 | PASS | [1360×880 Light](ux01-design-1360x880-light.png): the main region is a dotted workflow canvas with a positioned step node, matching graph content. |
| 实验设计 | UX-P5 | FAIL | [900×600 Dark](ux01-design-900x600-dark.png): the warning state is labeled “可创建（有警告）” rather than the contract vocabulary `待确认`; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验设计 | UX-LAYOUT | FAIL | [1360×880 Light](ux01-design-1360x880-light.png), [900×600 Light](ux01-design-900x600-light.png): at 1360 there is no persistent ≥320px right inspector, and below 1100 the selected-step detail is a clipped floating card rather than a bottom sheet; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验运行 | UX-P1 | PASS | [900×600 Light](ux01-run-900x600-light.png), [900×600 Dark](ux01-run-900x600-dark.png): the recording panel and “标记完成” control appear in the initial viewport. |
| 实验运行 | UX-P2 | FAIL | [1360×880 Light](ux01-run-1360x880-light.png): default step-resource/frozen metadata exposes raw identifiers including `ref-assay` and `assay-cck8`; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验运行 | UX-P3 | FAIL | [1360×880 Dark](ux01-run-1360x880-dark.png): `internal/runui/runui.go` uses hard-coded font sizes (including `FontSize(18)`, `FontSize(11)`, and `FontSize(14)`) outside theme tokens; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验运行 | UX-P4 | PASS | [1360×880 Light](ux01-run-1360x880-light.png): a procedure panel sits beside a 96-well matrix and recorded-value grid; the content shape follows the procedure plus plate data. |
| 实验运行 | UX-P5 | FAIL | [900×600 Light](ux01-run-900x600-light.png): the frozen Run header exposes `in_progress` instead of the contract state “进行中”; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验运行 | UX-LAYOUT | PASS | [1360×880 Light](ux01-run-1360x880-light.png), [900×600 Light](ux01-run-900x600-light.png): the wide view is procedure/recording two-column; the narrow view starts with the recording panel. |
| Notebook | UX-P1 | PASS | No screenshot in the 12-image allocation; live 900×600 GUI snapshot showed the “保存笔记” toolbar button in the initial viewport with the saved prose entry loaded. |
| Notebook | UX-P2 | PASS | No screenshot in the 12-image allocation; live 900×600 GUI snapshot showed the Run title and note text, with no raw UUID/step/block id. |
| Notebook | UX-P3 | FAIL | No screenshot in the 12-image allocation; live GUI plus `notebook_page.go` show hard-coded font sizes (`FontSize(20)`, `FontSize(13)`, `FontSize(11)`) outside theme tokens; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| Notebook | UX-P4 | PASS | No screenshot in the 12-image allocation; live view renders prose in the paper/editor region and code caps the document at 820px (`notebook_page.go`). |
| Notebook | UX-P5 | FAIL | No screenshot in the 12-image allocation; after saving, the live 900×600 snapshot still showed the Save control but no `已保存` state word, and unsaved state is only a dot; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| Notebook | UX-LAYOUT | PASS | No screenshot in the 12-image allocation; live 900×600 snapshot retained “保存笔记” in the wrapping toolbar; `notebook_page.go` caps paper width at 820px. |
| 实验方案 | UX-P1 | PASS | No screenshot in the 12-image allocation; live 900×600 GUI snapshot showed the Protocol version selector, frozen document heading, and “导出 PDF” control without scrolling. |
| 实验方案 | UX-P2 | FAIL | No screenshot in the 12-image allocation; the default frozen-state line displays the eight-character Run UUID prefix `0eae89b2`; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 实验方案 | UX-P3 | PASS | No screenshot in the 12-image allocation; the active document route uses `t.Rem`, theme colors, and shared Protocol components (`protocol_page.go`, `protocol.go`), with no literal font size/color on that route. |
| 实验方案 | UX-P4 | PASS | No screenshot in the 12-image allocation; live document view is a frozen document with centered max-width 820px (`protocol_page.go`), matching the document-mode contract. |
| 实验方案 | UX-P5 | PASS | No screenshot in the 12-image allocation; live state line explicitly says “已冻结” and the version is read-only, matching the contract vocabulary. |
| 实验方案 | UX-LAYOUT | PASS | No screenshot in the 12-image allocation; live document view uses a centered 820px maximum width, the required document-mode layout. |
| 资料与工具 | UX-P1 | FAIL | No screenshot in the 12-image allocation; live 900×600 GUI snapshot marked “Use plate in Designer / create draft” offscreen, so the prepared-draft primary handoff is below the fold; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 资料与工具 | UX-P2 | PASS | No screenshot in the 12-image allocation; live 900×600 snapshot showed tool labels and a plate matrix, not raw UUID/step/block ids. |
| 资料与工具 | UX-P3 | FAIL | No screenshot in the 12-image allocation; `internal/tools/tools.go` uses literal font sizes (for example `FontSize(20)`, `FontSize(11)`, `FontSize(12)`) and pixel padding/width outside theme tokens; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 资料与工具 | UX-P4 | PASS | No screenshot in the 12-image allocation; live plate-tool view is an 8×12 well matrix, matching its content shape. |
| 资料与工具 | UX-P5 | FAIL | No screenshot in the 12-image allocation; tool errors/status use English strings such as “Import accepted” and expose no retry control, rather than §3 state vocabulary plus cause/retry; file UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`). |
| 资料与工具 | UX-LAYOUT | PASS | No screenshot in the 12-image allocation; live 900×600 snapshot kept the 190px tool list beside its content, as required by the Tools breakpoint rule. |

## Findings boundary

No production code was changed. All findings are tracked for follow-up by UX-02 (`9624f206-7f45-41ac-a5f4-55055e72c5ef`).