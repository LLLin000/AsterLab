# WG-02 hierarchy diagnostic

Baseline: `ec5c933e43b559888f53f34f065e410fa229904f` (`origin/main`). The before image is from an `origin/main` build; the after image is from this branch, both on the same isolated Run/database at 1360×880 light.

The regression was in `prototypes/mygo-run/main.go:505-517` (baseline `:503-525`): `buildRunWorkspaceVM` called `RunService.WorkflowTree` for every selected Run, even when `snap.AsterDoc` was nil. Core routes that Run-ID lookup through its frozen hierarchy projection, which resolves the document revision; a design-created Run without an AsterDoc has no valid frozen revision to project, so the call returned `NOT_FOUND`. `internal/runworkspace.LoadProjection` now skips the loader for that state and preserves failures for pinned revisions with the original cause and a retry/check-the-frozen-version action, per UX-STATE and UX-P5.

## Evidence

- `wg02-before-1360x880-light.png` — origin/main, shows `层级诊断：NOT_FOUND: AsterDoc revision v1 does not exist` and the available recording panel.
- `wg02-after-1360x880-light.png` — fixed build, no hierarchy diagnostic; A1 value `0.82` was entered and saved (`已保存`, `已记录 1`).

The checkout did not contain the referenced `docs/product/evidence/QA-01/README.md` or `commands.txt`; this isolated fixture follows the requested CLI creation sequence and contains no AsterDoc revision. Fixture commands and inputs:

```text
$ labworkbench project.create --input -
{"name":"QA-01 CCK-8 regression"}

$ labworkbench protocol.block.create --input -
{"project_id":"33be279b-3892-4b64-a591-16f5f6d5c787","title":"CCK-8 quality check and readout","summary":"Synthetic regression fixture","parameter_slots":[],"steps":[{"kind":"text","use_id":"read","title":"Plate reader readout","instruction":"Read OD450.","checks":[]}]}

$ labworkbench protocol.create --input -
{"project_id":"33be279b-3892-4b64-a591-16f5f6d5c787","title":"QA-01 protocol","root_block_version_id":"7bd53959-40e0-47fc-8694-c27412a30b91"}

$ labworkbench design.create --input -
{"project_id":"33be279b-3892-4b64-a591-16f5f6d5c787","design":{"title":"QA-01 CCK-8 design","objective":"Synthetic GUI regression only","experimental_unit":"well","group_specs":[{"name":"Control","role":"control","condition":"vehicle","independent_n":2},{"name":"Treatment","role":"treatment","condition":"synthetic treatment","independent_n":2}],"assays":[{"id":"cck8","title":"CCK-8","endpoint":"OD450","method":"plate","unit":"AU","timepoints":["24 h"],"group_names":["Control","Treatment"],"protocol_version_id":"113d647d-c467-40ac-9e3e-82c8ddc75e60"}]}}

$ labworkbench run.create --input -
{"design_id":"e80d0542-52ce-4a74-90b9-6495e35c1a7e","title":"QA-01-CKK8"}

$ go test ./internal/runworkspace ./... -count=1
$ mygo native build -target host
$ powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/mygo-build.ps1
```

The CLI calls used `LABWORKBENCH_DATA_DIR=C:\Users\Lin\AppData\Local\Temp\wg02-gui-data` and `LABWORKBENCH_TRUSTED_WRITER=1`. The UI used a fresh isolated data directory and the real-window automation socket; `0.82` was saved through the Run recording panel. Both regression tests pass with the fix; on the baseline the flow still unconditionally called `WorkflowTree` (also reproduced in the before screenshot). The baseline source predates `LoadProjection`, so the new package tests cannot compile against it as-is.

Remaining risk: a pinned AsterDoc revision that Core cannot load remains a visible diagnostic; repair/retry behavior is intentionally not hidden or otherwise changed.
