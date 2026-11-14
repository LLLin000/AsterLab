# DES-05 Protocol read-only evidence

## Baseline and scope
- Checkout: `O:\des05`; branch: `agent/DES-05-protocol-nav`.
- Base commit: `f4744e12aa401b649956fcd106c499f3b763e015` (`origin/main` after merge).
- UX rules applied: UX-IA-5 (Protocol is read-only), UX-P2 (no raw identifiers in the default view), UX-P3 (existing themed components), UX-P4 (document-shaped content), UX-P5 / UX-STATE (`预览`, `已冻结`, `未解析`), UX-LAYOUT (820px document maximum).

## What ran
Commands were run against an isolated data directory, `C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe`:

```sh
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe tools --json
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe project.create --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe protocol.block.create --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe protocol.block.create --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe protocol.create --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe protocol.create --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe protocol.list --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe protocol.block.get --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe design.create --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe asterdoc.save --input -
LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe O:/ag05/target/debug/labworkbench.exe asterdoc.create_run --input -
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/mygo-build.ps1
go test ./internal/protocol ./... -count=1
./scripts/ci/local-gate.ps1 -Pr <PR>
```
For CLI commands ending in `--input -`, JSON was passed on stdin as the corresponding CLI payload. The GUI was launched with `LABWORKBENCH_DATA_DIR=C:/Users/Lin/AppData/Local/Temp/des05-protocol-probe`, `ASTERLAB_DESIGN_ID=186e9b6d-ff34-4c0a-9184-0e392ff7ca5a` or `ASTERLAB_RUN_ID=0cdc3904-bd3d-4e56-a005-f7611a68744e`, `ASTERLAB_THEME=light|dark`, and `MYGO_AUTOMATION=<fresh socket>`; real windows were sized with `MoveWindow`, and screenshots were captured with `mygo-automation screenshot`.

The CLI seed created two Protocol summaries in one project, with distinct root block versions; the preview design references the first summary. `protocol.list` returned Protocol containers, `protocol.block.get` returned the block version, and `protocol.create` returned its newly created ProtocolVersion record. The app's existing `ListProtocols` returned the version summaries used by the selector. The UI switched to the other listed summary and displayed `未解析` rather than substituting the first version's body. The resolvable preview and Run views are read-only; no raw version/block/step identifiers or edit controls appear in the Protocol content.

The first full Go test invocation, before rebuilding the native module after the upstream merge, failed in existing native fixtures with “implementation of Runs is older than its sources.” After `scripts/mygo-build.ps1` refreshed Runs, `go test ./internal/protocol ./... -count=1` passed (21 packages ok, 4 no tests).

## Screenshots
All images are real Windows application windows captured through `mygo-automation`; each PNG is exactly the requested content size.
- `des05-preview-1360x880-light.png` — resolvable preview, before switching.
- `des05-switch-after-1360x880-light.png` — switched to the other listed version; `预览` + `未解析`.
- `des05-preview-900x600-light.png` — preview at narrow breakpoint.
- `des05-preview-1360x880-dark.png` — preview, dark theme.
- `des05-preview-900x600-dark.png` — preview, dark theme, narrow breakpoint.
- `des05-frozen-1360x880-dark.png` — frozen Run with Run id and frozen revision.
- `des05-frozen-900x600-light.png` — frozen Run at narrow breakpoint.
- `des05-frozen-switch-after-900x600-dark.png` — selected alternate version on a frozen Run; `已冻结` + `未解析`.

## 发现但未修的缺陷 / Core 缺口
DES-06 is tracking the missing Core query: **ProtocolVersionID → root block version ID + expanded content/parameters**. Until that query exists, this UI lists every ProtocolVersion summary but reports `未解析` for content that cannot be resolved through the existing frozen AsterDoc or exact design-revision APIs. It never displays another version's content as a fallback.
