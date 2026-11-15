# EXP-02 evidence

Card: EXP-02 (Core) 板联动算量与运行期动态配平计算器

## Acceptance commands

Raw output files:

| file | command | exit |
|---|---|---|
| `cargo-test-labworkbench.txt` | `cargo test --bin labworkbench -q` | 0 |
| `cargo-test-core.txt` | `cargo test --manifest-path crates/asterlab-core/Cargo.toml -q` | 0 |
| `go-test.txt` | `cd prototypes/mygo-run && go test ./... -count=1` | 0 |

The Go suite needs the native Runs module built first, exactly as
`.github/workflows/linux-tests.yml` does:

```sh
cd prototypes/mygo-run
go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native build -target host ./modules/runs
go test ./... -count=1
```

## HUVEC 3%/5%/7% x 3 replicates, 12-well plate

Inputs: 200 µL/well, 10% excess, LAP stock 0.5% w/v -> final 0.05% w/v,
seeding density 20 000 cells/well, factor stock 10 µg/mL -> final 50 ng/mL.

CLI inputs and raw outputs:

```json
// calculator.gel_mastermix
{"volume_per_well_ul":200,"excess_ratio":0.1,"lap_stock_percent_wv":0.5,"lap_final_percent_wv":0.05,"groups":[{"name":"3%","gelma_percent_wv":3,"wells":3},{"name":"5%","gelma_percent_wv":5,"wells":3},{"name":"7%","gelma_percent_wv":7,"wells":3}]}

// calculator.cell_seeding
{"density_cells_per_well":20000,"volume_per_well_ul":200,"excess_ratio":0.1,"factor_stock_mg_per_ml":0.01,"factor_final_mg_per_ml":0.00005,"groups":[{"name":"3%","wells":3},{"name":"5%","wells":3},{"name":"7%","wells":3}]}

// calculator.runtime_normalize
{"planned_wells":9,"actual_wells":7,"volume_per_well_ul":200,"excess_ratio":0.1,"dead_volume_ul":50,"prepared_volume_ul":2000}
```

| tool | output |
|---|---|
| `calculator.gel_mastermix` | `calculator-gel-mastermix.json` |
| `calculator.cell_seeding` | `calculator-cell-seeding.json` |
| `calculator.runtime_normalize` | `calculator-runtime-normalize.json` |

The CLI rounds display values to 1e-6 (1 nL) so 2030 µL reads as `2030.0`
instead of `2030.0000000000002`.

Expected GelMA rows (each group has 3 wells, 660 µL working volume):

| group | GelMA mass | LAP stock | solvent |
|---|---|---|---|
| 3% | 19.8 mg | 66 µL | 594 µL |
| 5% | 33.0 mg | 66 µL | 594 µL |
| 7% | 46.2 mg | 66 µL | 594 µL |
| total | 99.0 mg | 198 µL | 1782 µL |

Cell seeding: 198 000 cells total, 1980 µL suspension, 9.9 µL factor stock.
Runtime normalize: planned 2030 µL, required 1590 µL, 0 additional, 410 µL unused.

The same numbers are pinned by:

- Rust: `crates/asterlab-core/src/calculators.rs` (`huvec_gel_mastermix_scales_with_wells_and_excess`,
  `cell_seeding_counts_cells_and_factor_volume`, `runtime_normalize_recomputes_from_actual_wells_and_dead_volume`)
  and `src/cli.rs` (`plate_mix_calculators_match_the_huvec_case`).
- Go: `prototypes/mygo-run/internal/asterdoc/parity_fixture_test.go`
  (`TestPlateMixPlanDerivesGelAndSeedingRows`).

## AsterDoc plate linkage

`plate@1` keeps its original required fields; the optional `mix_*` inputs and
the two derived outputs (`gel.mastermix.v1`, `cell.seeding.v1`) are validated by
both `internal/asterdoc` (`plate-mix-fields`) and Rust Core
(`asterdoc_plate_mix_fields`). The Go/Rust accept-reject samples live in
`prototypes/mygo-run/asterdoc_store_test.go`; the Linux run of that store test
skips when the Windows fixture SQLite is absent, so Rust acceptance of the same
source is also pinned by `asterdoc_plate_mix_fields_match_go_calculator_cases`
in `src/cli.rs`.

No GUI layout, `shell`, `runworkspace`, frozen Run, or `dilution@1` behaviour was
changed.
