# SDK Foundation v1: compatibility and gaps

- Date: 2026-11-10
- Baseline: working tree at commit `271eeff`.
- This file reports exercised checks only. It does not claim a user study or real-device validation.

## 1. Acceptance gates

| # | Gate | Status | Evidence |
| --- | --- | --- | --- |
| 1 | `plate@1` supports every plate format | Schema and read-only rendering implemented; interactive editing is partial | The duplicate AsterDoc `plate96@1` kind is removed; formats live under `plate@1`. The dedicated interactive editor is 96-well; other supported geometries render read-only. Old `kind: plate96` source needs explicit migration to `kind: plate` plus `format: 96`. |
| 2 | Plate group/well/identity validation and assignment | Implemented | Geometry, typed group references, unique well mappings, explicit optional sample IDs/technical replicates, and clear-well cleanup are checked by Go and Rust/Core. |
| 3 | Schema, unit and reference validation | Implemented for Foundation v1 | `validateSpecModule`, `validatePortValue` and typed-reference tests; invalid units, dimensions and missing reference targets are rejected in Go and Rust. |
| 4 | Go/Rust consistency | Implemented for the tested surface | `TestAsterDocGoRustParity` covers explicit plate sample/replicate identities, duplicate technical replicates, dilution, instrument profiles, and typed references. |
| 5 | Module actions require Core permission | Existing contract retained | `View.Can(PermDocumentEdit)` and Core command checks; no new write bypass was added. |
| 6 | Document and record are not dual-written | Retained | New modules are document/render-only. Dilution outputs are derived and explicitly not written as Run facts. |
| 7 | One registry for SDK modules | Implemented | `plate`, `dilution`, and `instrument_profile` read `spec/modules.json`; the registry shares generic callbacks by renderer key. |
| 8 | Notebook can show modules through the same viewer | Implemented through the shared markdown code-block hook | The same renderer path is used for document surfaces; there is no notebook-only module parser. |
| 9 | New module does not change Core transaction logic | Implemented for these modules | No SQLite/Run transaction changes were made for the new modules. |
| 10 | New fields/features are versioned, not retrofitted | Contract defined | Old `step@1` is unchanged; new module kinds have schema 1. A future breaking change requires a new schema/migration. |

## 2. Implemented in this pass

- `spec/modules.json` is the single manifest for 14 module kinds and declares their Design/Document/Run renderer keys.
- `Registry.RegisterRenderer` registers reusable callbacks once (`generic.editor`, `generic.run`); module-specific callbacks such as `plate.editor` remain specialized. Missing surface declarations show a gap, not a fallback.
- Foundation port types in the shared manifest: quantity, typed reference, enum, decimal/integer/boolean, list, table, geometry, artifact, duration, timepoint, relative time, with legacy aliases.
- Shared `spec/units.json` named UCUM subset and explicit conversion helper.
- Typed reference target validation in Go and Rust.
- `dilution@1`: quantity validation, deterministic `dilution.v1` derived volumes, Go renderer.
- `instrument_profile@1`: capability/constraint/evidence declaration with no hardware control, Go renderer.
- Machine-checkable Foundation schema and manifest contract.
- Run 文件关联通过独立 Core `asset_link` 记录保存 `root_id + relative_path`，并关联 Run/task/sample；工作区 session 单独保存本机 root 映射。AsterML 不嵌入文件字节或绝对路径；打开文件时校验元数据/hash，缺失文件可重新定位。
- Workflow steps/order/edges are semantic AsterDoc data; node coordinates are saved separately in `workspace-session.json` under `canvas_layouts[design_id]` and do not affect compilation.

## 3. Remaining gaps

### Contract and tooling

- The CLI now exposes module discovery/description (`sdk.modules`, `sdk.describe`) and AsterDoc workflow tools. There is no standalone manifest-validation or `sdk.validate` command; `asterdoc.validate` validates candidate document source and attempts compilation.
- Structured diagnostics are defined in the schema but the CLI still returns `code` + `message`; `needs_confirmation` / `advisory` arrays are not serialized yet.
- Manifest contract checking exists in the loader/tests, but there is no standalone `sdk validate` CLI.
- There is no module scaffold generator or test harness command.
- Go and Rust validators are maintained in parallel; there is no code generation from the manifest. The parity test catches sampled drift, not all drift.
- Unknown YAML fields are preserved but not reported. A strict mode is not implemented.
- The Foundation JSON Schema is documentation/contract tooling, not the runtime validator. Runtime validation remains the Go/Rust validators.

### Scientific/domain

- The unit registry is a named subset, not all of UCUM. No compound-unit parser, offset temperature conversion, or molar-mass conversion.
- `step@1` still cannot express HUVEC material amounts, UV duration, temperature and concentrations as structured fields; a future version/composable module is required.
- `instrument_profile` has no instance/calibration/reservation model and no Device Adapter. It cannot control hardware.
- SDK modules now bind through `step@2.uses` and are filtered per Run task. `step@1` remains unchanged; role values are not yet a closed enum.
- Notebook has a read-only timeline mode. Real dark-theme 900×600 evidence at 150% scale is captured; 100%/200% and additional dark pages remain open.
- No ISA/AnIML/PROV serialization or import.
- Template shape and Go instantiation (`InstantiateTemplate` / `TemplateManifest`) exist; there is no template market, remote download, or template CLI.
- There is no dedicated static/PDF renderer for the new modules; the existing generic static preview path is read-only.
- The shared renderer registry supports explicit Design, Document, and read-only Run callbacks. The generic editor is key-selected, not capability-gated; composite fields remain read-only there.

### Validation coverage

- External cross-document references are not resolved.
- The free `reference.ref` field can point at a record-store object but is not type-checked against record kinds.
- Duplicate YAML key behavior is not yet pinned as a shared diagnostic test.
- Reference target validation checks existence and kind, not semantic role (for example, an assay group ref does not prove the group is a valid control).
- Calculator failure paths are tested in Go; Rust validates inputs but does not expose a calculator result.

### UI/editor follow-up

The prior editor items are now covered by headless and real-window checks:

- local AsterDoc draft undo/redo for inspector fields, tasks, checks and workflow edits, plus Designer draft history;
- real GUI CCK-8: opened the existing document, selected a step, edited a title, undid/redid it, saved rev 3, bound a same-project Protocol through Core, created a Run from rev 5, and observed the Run workspace;
- real window at 900×600: minimum window lowered to 900×600; snapshot showed no top-level element outside the 900×600 client area, and the inspector scrolls internally.
- `TestCreateRunFromAsterDocFreezesDocumentAndDesign` 从 AsterDoc `plate@1` 冻结 Run，生成同一板映射的 CCK-8 网格、保存 96-cell readout batch，经新 `RunService` reload 后验证样本/技术重复身份；新 AsterDoc revision 不改写旧 Run。
- `TestCCK8VerticalSliceLoop` 继续覆盖读数、任务检查/完成、Review、Notebook provenance、服务重载与 CLI cross-read。

Evidence images (generated from the real window, not headless):

- `prototypes/mygo-run/target/e2e-cck8-900x600.png`
- `prototypes/mygo-run/target/e2e-cck8-run-1080x640.png`

Remaining: high-DPI/scale-200 and dark-mode real-window capture are not yet part of this evidence set.

## 4. Compatibility policy

- Do not edit a released module's schema semantics in place.
- Add a new `schema` for a breaking change; add a migration or keep the old renderer.
- The user-requested cleanup removes `plate96@1`; old AsterDoc source must be migrated explicitly to `plate@1` with `format: 96`. Immutable historical revisions are not rewritten, and no compatibility alias is retained.
- Treat a renderer capability as a claim only when the renderer exists.
- Do not add a record-writing path to a document module.

## 5. Agent 工作流（固定命令名）

GUI 与 Agent 是同一 Core 的两个独立客户端；不依赖 UI 自动化。`tools --json` 暴露 `sdk.modules`、`sdk.describe`、`asterdoc.load`、`asterdoc.inspect`、`asterdoc.validate`、`asterdoc.assess`、`asterdoc.command`、`asterdoc.compile`、`asterdoc.create_run` 等入口及其 Schema。

推荐顺序：
1. `tools --json` 发现命令、读写属性和 JSON Schema。
2. `sdk.modules` 列出已注册模块；`sdk.describe(kind, schema)` 读取字段、动作、权限和 renderer key。
3. `asterdoc.load` 读取当前完整 AsterML 源、revision 与 hash；`asterdoc.inspect` 返回结构化模块实例和对应动作/renderer 契约。两者 revision 不一致时重新读取。
4. 基于 inspect 结果准备语义命令，并在本地形成候选源。CLI 的 `asterdoc.command` 输入 Schema 用 `oneOf` 明确描述 add/remove/update、step、连线和孔位动作。
5. 将候选源交给 `asterdoc.validate`；要求 `valid=true` 且 `compiled=true`。提交前同时确认科学假设与待确认项，不让 Agent 替研究者推断。
6. 通过 `asterdoc.command` 提交单条命令，带当前 `expected_revision` 和唯一 `operation_id`。Core 会再次校验并保存不可变新版本。
7. 重新调用 `asterdoc.load` / `asterdoc.inspect` 确认 revision 和结果；用 `asterdoc.assess` 检查该已保存 revision 的 readiness、errors 与 warnings。遇到 revision conflict，重新读取、重建候选命令，禁止盲目重试。
8. 用 `asterdoc.compile` 按精确 revision 检查 Design 投影；该命令只读。
9. 用 `asterdoc.create_run` 按精确 revision 冻结 Run；研究者确认 `asterdoc.assess` 返回的 warnings 后，将确认的原文传入 `acknowledged_warnings`。
10. 执行时用 `run.workspace` 查询冻结计划，用 `execution_event.create` 记录结构化事件。CCK-8 孔板/表格读数共用 Run Core 的 readout-batch 写入；当前没有对应的 CLI 批量读数工具。

一条 `asterdoc.command` 是一个原子版本提交；跨多条命令的 `asterdoc.batch` 尚未实现。复杂设计应按依赖顺序逐条提交，每条提交后使用返回的新 revision。

`asterdoc.validate` 校验候选 source；`asterdoc.assess` 只评估已保存的精确 revision，并返回 Run 冻结所用 Core audit 的同一组 warnings（另加 readiness errors）。两者不可互相替代；研究者确认的 warning 原文可交给 `asterdoc.create_run.acknowledged_warnings`。

## 6. 运行限制（本 pass 不声明）

- 无 `asterdoc.batch` / 原子多命令批处理。
- 仅 CCK-8 结果网格 / Run Core 路径具备读数闭环；仪器控制未实现。
- 模块级 Run 视图为只读摘要；记录写入仅通过 Core 命令。

## 7. Verification commands
```sh
# Go Foundation and module tests
cd prototypes/mygo-run
go test ./internal/asterdoc

# Go/Rust parity and document store tests
go test . -run 'TestAsterDocGoRustParity|TestAsterDocStoreIsImmutable'

# Full Rust Core tests
cd ../..
cargo test --quiet

# Rebuild the native module after Rust contract changes
cd prototypes/mygo-run
go run github.com/egoist/mygo/cmd/mygo native check modules/runs
```
## Static / PDF (Phase 2)

Added: read-only static module preview (`internal/asterdoc.StaticPreview` / `StaticDocumentPreview`) renders any registered module deterministically from parsed data + revision/hash, with no write path or DB handle. Frozen revision loading is verified by `LoadFrozenAsterDocAt` (source hash checked before render). Minimal stdlib PDF export (`internal/asterdoc.ExportPDF`) is deterministic for identical input.

Limits: PDF layout is basic single-page text; no charts/images; multi-page pagination not implemented; viewer still requires Go UI host for interactive rendering.
## Phase 4 scientific modules

Added: material, sample, container, measurement_profile (sdk 1.0.0). All are
document/render-only with record_boundary=run. Remaining gaps: hardware control,
dynamic loading, step@1 semantics unchanged.
