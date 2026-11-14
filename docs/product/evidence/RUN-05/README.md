# RUN-05 Evidence — Multi-Assay Recording Layouts Over Shared ResultGrid Kernel

## Baseline & Scope
- Repository: `LLLin000/AsterLab`, isolated `ovnfs` checkout `O:\run05`.
- Scope files modified:
  - `prototypes/mygo-run/internal/assays/layouts.go`
  - `prototypes/mygo-run/internal/assays/schema.go`
  - `prototypes/mygo-run/internal/assays/schema_test.go`
  - `prototypes/mygo-run/internal/runworkspace/recording.go`
  - `prototypes/mygo-run/internal/runworkspace/recording_test.go`
- Zero changes to `internal/resultgrid/` and zero Core changes (`crates/`, `src/` untouched).

## Architecture
- `internal/assays/layouts.go` introduces `LayoutConfig` + `RegisterLayout` / `LookupLayout` / `DefaultViewForSchema` and a declarative `matrixSchema` builder:
  1. `cck8` (`Layout: "plate96"`, `DefaultView: "plate"`, `SchemaID: "cck8.plate96.od450.v1"`)
  2. `qpcr` (`Layout: "sample_gene"`, `DefaultView: "table"`, `SchemaID: "qpcr.sample_gene.cq.v1"`)
  3. `western_blot` (`Layout: "sample_target"`, `DefaultView: "table"`, `SchemaID: "qpcr.sample_gene.cq.v1"`, `Field: intensity / AU`) — registered purely by config with no custom builder or `resultgrid` change.
- `internal/runworkspace/recording.go` routes `SelectKind` and `availableViews` through `assays.DefaultViewForSchema(grid.Schema)` and renders non-plate matrix layouts cleanly in `renderTable`.

## Verification
1. `git diff crates/ src/ prototypes/mygo-run/internal/resultgrid`: empty (0 lines).
2. `go test ./... -count=1`: 21 packages passed (including `TestBuildSchemaThirdAssayViaLayoutConfigOnly` and `TestSelectKindRoutesByAssayLayoutConfig`).
3. Real Windows GUI verification (`dist/mygo-run.exe` + `Runs.dll`) across all 3 assay types (1360×880 Light & 900×600 Dark):
   - CCK-8 opens on `孔板` (`A1 = 0.84 AU` saved to Core).
   - RT-qPCR opens on `表格` (`S1 · GAPDH_R1 = 19.4 Cq` saved to Core).
   - Western Blot (config-only 3rd type) opens on `表格` (`g-ctrl · Target_R1 = 14200 intensity` saved to Core via `run.save_batch`).

## Screenshots
- `run05-cck8-1360x880-light.png`
- `run05-cck8-900x600-dark.png`
- `run05-qpcr-1360x880-light.png`
- `run05-qpcr-900x600-dark.png`
- `run05-wb-1360x880-light.png`
- `run05-wb-900x600-dark.png`
