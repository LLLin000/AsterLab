# QA-02 RNA Run Evidence

## Baseline and fixture

- Repository baseline: `origin/main` at `01e207a`; isolated checkout `O:\qa02`; branch `agent/QA-02-rna-regression`.
- Fixture: `scripts/rna-fixture.py`; isolated data: `C:\Users\Lin\AppData\Local\Temp\qa02\data`; CLI/app used: the existing `O:\ag05` build. Fixture output: [`fixture-output.txt`](fixture-output.txt).
- Seeded Run `4dc1bd1e-68af-46b9-becf-de4b614275ed`; 13 tasks: ten RNA extraction steps, RNA QC, reverse transcription, qPCR. Fixture assessment returned `warning []`; its run summary reported 13 tasks.
- `docs/product/evidence/AG-05/` exists and was used as the primary reference. `docs/product/evidence/DES-05/` is absent; no DES-05 content is inferred.

## GUI walk and progress

The real Windows app was opened with the isolated data dir and Run ID. Each of the 13 tree leaves was selected at **1360×880**; per-step screenshots show the selected/current-step highlight. [`step-log.txt`](step-log.txt) records the entire accessibility snapshot for every selection, including the instruction/“要记录什么” area, current state, and displayed counts. [`restart-snapshot.txt`](restart-snapshot.txt) and [`qa02-restart-900x600-dark.png`](qa02-restart-900x600-dark.png) document reopening at **900×600**.

| # | Step path | What the fixture says the step needs | GUI task state / displayed counts | Screenshot |
|---:|---|---|---|---|
| 1 | `root/rna/step-01` | 收集约 5×10^6 细胞，弃上清。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-01-1360x880-light.png) |
| 2 | `root/rna/step-02` | 加入裂解缓冲液，匀浆后离心。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-02-1360x880-light.png) |
| 3 | `root/rna/step-03` | 加入等体积 70% 乙醇，混匀。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-03-1360x880-light.png) |
| 4 | `root/rna/step-04` | 将混合液过柱，离心弃流出液。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-04-1360x880-light.png) |
| 5 | `root/rna/step-05` | 加入 Wash Buffer I，离心弃流出液。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-05-1360x880-light.png) |
| 6 | `root/rna/step-06` | 加入 Wash Buffer II，离心弃流出液。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-06-1360x880-light.png) |
| 7 | `root/rna/step-07` | 再次加入 Wash Buffer II，离心弃流出液。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-07-1360x880-light.png) |
| 8 | `root/rna/step-08` | 空柱离心 2 min，去除残留乙醇。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-08-1360x880-light.png) |
| 9 | `root/rna/step-09` | 加入无 RNase 水，离心收集 RNA。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-09-1360x880-light.png) |
| 10 | `root/rna/step-10` | 测定浓度与 A260/A280，-80 °C 保存。 | 未开始; extraction 0/10; Run 0/4 | [PNG](qa02-step-10-1360x880-light.png) |
| 11 | `root/qc/qc` | 记录 RNA 浓度与 A260/A280。 | 未开始; QC 0/1; Run 0/4 | [PNG](qa02-step-11-1360x880-light.png) |
| 12 | `root/rt/rt` | RNA 逆转录为 cDNA。 | 未开始; RT 0/1; Run 0/4 | [PNG](qa02-step-12-1360x880-light.png) |
| 13 | `root/qpcr/qpcr` | 配置反应并运行 qPCR，记录 Ct。 | 未开始; qPCR 0/1; Run 0/4 | [PNG](qa02-step-13-1360x880-light.png) |

**Progress limitation:** this is a navigation walk, not a successful completion pass. The fixture supplies no step checks, no parameter slots, and no recording plan; the UI shows “此步骤没有计划参数” and no completion control. Consequently task statuses stay `not_started` / `未开始` and counts remain zero. The Run header reads `planned`; no `进行中`/`已完成` transition was demonstrated. This evidence must not be read as a pass of task completion.

## Parameter binding result

The fixture’s `block()` payload hardcodes `parameter_slots: []`; every `block_use` uses `bindings: []`. The seeded 13 step snapshots and `run.workspace` therefore have empty parameters and no resolved parameters. Tried `protocol.block.preview` against the fixture RNA block version with the AG-05 example keys `lysis_vol=200 uL` and `lysis_time=10 min`; raw CLI response is in [`parameter-binding-error.txt`](parameter-binding-error.txt):

```json
{"error":{"code":"INVALID_INPUT","message":"步骤块不存在根参数槽位：lysis_vol"},"ok":false,"tool":"protocol.block.preview"}
```

Used instead: no substitute values or synthetic binding were introduced. The UI walk records the fixture’s unbound procedural steps only. Therefore parameter binding is **not passed**; the existing fixture must expose slots before a bound Run can be tested.

## Frozen Run readback

`run.workspace` was captured before the GUI restart in [`workspace-before.json`](workspace-before.json), then again after reopening the frozen Run in [`workspace-after.json`](workspace-after.json). [`field-comparison.txt`](field-comparison.txt) quotes the protocol, design/AsterDoc versions, all 13 step paths, each step’s source block version and parameter fields, all task timepoints/statuses, and the frozen design timepoints. Every top-level `run.workspace.result` field compares equal (`True`).

| Frozen field | Before | After | Result |
|---|---|---|---|
| Run ID | `4dc1bd1e-68af-46b9-becf-de4b614275ed` | same | `==` |
| AsterDoc revision / source hash | `1` / `72c29a5e26f04bcf2416e7812e0752f6509cda03e0660562ce5564b089cffd7e` | same | `==` |
| Design version / revision | `7e0eb6d9-e48b-4a0f-8aa3-15f040a0cb54` / `2` | same | `==` |
| Protocol version (`scope.protocol_version_id`, version) | `551b8102-bf41-4537-a1bd-9346dc5a0074`, `1` | same | `==` |
| Protocol version (top-level `protocol_version_id`) | `null` | `null` | `==` |
| Step paths | 13 paths, `root/rna/step-01` … `root/qpcr/qpcr` | same ordered list | `==` |
| Parameters | all 13 `steps[].step.parameters=[]`; resolved/unresolved scope parameters `[]` | same | `==` |
| Timepoint | all 13 task timepoints `null`; frozen assay timepoints `[]` | same | `==` |
| Task statuses | all 13 `not_started` | same | `==` |
| Workspace result fields | all 17 top-level fields equal | all 17 top-level fields equal | `==` |

The full value-by-value comparison is in `field-comparison.txt`; the raw CLI outputs remain available above.

## UX contract rules referenced

- `UX-IA-3`: Run is the surface for executing steps and recording facts.
- `UX-STATE`: task status vocabulary is `未开始` / `进行中` / `已完成` / `阻塞`; observed task state was `未开始`.
- `UX-P5`: state should be explicit and recoverable; observed Run-level `planned` does not show a progress transition.
- `UX-P2`: step metadata/provenance should be progressively disclosed; selected-step view visibly includes the step ID.
- `UX-LAYOUT`: evidence at both contract dimensions, 1360×880 and 900×600.
- `UX-DONE`: screenshots are attached; full behavior is not done because bindings and task completion did not pass.

## Evidence files

- Commands: [`commands.txt`](commands.txt)
- Raw fixture and parameter binding CLI output: [`fixture-output.txt`](fixture-output.txt), [`parameter-binding-error.txt`](parameter-binding-error.txt)
- Raw `run.workspace`: [`workspace-before.json`](workspace-before.json), [`workspace-after.json`](workspace-after.json)
- GUI step logs and restart snapshot: [`step-log.txt`](step-log.txt), [`restart-snapshot.txt`](restart-snapshot.txt)
- Screenshots: 13 `qa02-step-*-1360x880-light.png` files and `qa02-restart-900x600-dark.png`.
