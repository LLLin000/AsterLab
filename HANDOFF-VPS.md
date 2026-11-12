# Handoff — moving AsterLab to a VPS

Date: 2026-11-12. Baseline: `main` = `5e20027` (`a1a84d2` plus the VPS
handoff and reproducible fixture). Frozen tag `sdk-v1-rc3` = `b267d3f`; do not move it.

## 1. What the VPS can and cannot do

This is a **Windows-first desktop product**. Split the work deliberately.

| Capability | Linux VPS | Windows host |
|---|---|---|
| Rust Core CLI (`labworkbench`) | **yes** | yes |
| `crates/asterlab-core`, `crates/asterlab-app` tests | **yes** | yes |
| Module manifest / validators / parity fixture | **yes** | yes |
| Acceptance fixture via `scripts/rc3-fixture.py` | **yes** | yes |
| Go non-GUI packages | likely yes — verify `go build ./...` | yes |
| MyGo/Slint desktop app (`prototypes/mygo-run`) | **not without porting** | yes |
| `runs.dll` native module (`mygo native check`) | builds a host-native `libRuns.so` for local Go tests; not the shipped `.dll` | yes (MSVC) |
| `mygo-automation` UI automation, `scripts/gui-e2e-v11.ps1`, screenshot scripts | **no** (Win32 + PowerShell) | yes |

Windows-only by construction: `target/vsdev.cmd` (MSVC environment),
`.mygo/native/.../x86_64-pc-windows-msvc`, `AsterLab-Slint.exe`,
`LabWorkbench*.exe`, and every GUI acceptance script.

Recommendation: the VPS owns Core/SDK/CLI/tests/docs; keep one Windows machine
(or VM) for the GUI acceptance gate.

## 2. Bootstrap

Toolchain measured on the Windows host; match or exceed on the VPS.

- Go 1.27.3
- rustc 1.99.0 (stable), Cargo
- Python 3.14 (fixture scripts only)
- A C toolchain for `rusqlite` (`cc`, `gcc`, `pkg-config`, `libsqlite3-dev` on Linux)
- A CJK TrueType font for the PDF test (`fonts-droid-fallback` on Debian/Ubuntu)
- Git

```bash
git clone https://github.com/LLLin000/AsterLab.git
cd AsterLab
git checkout main          # a1a84d2
```

Build/test on Linux:

```bash
# Rust Core CLI + tests
cargo test --bin labworkbench
cargo test --manifest-path crates/asterlab-core/Cargo.toml
cargo test --manifest-path crates/asterlab-app/Cargo.toml
cargo build --bin labworkbench

# Go: build the host-native Runs library first, then test
cd prototypes/mygo-run
go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native build -target host ./modules/runs
go test ./...
```

The script `scripts/mygo-native-linux.sh` performs the native build and the
Go test run. The host build lands in `.mygo/native/linux-amd64/libRuns.so`
(gitignored); Windows keeps using the published `Runs.dll`. This is the same
Go contract with different build artifacts per platform.

If the GUI packages do not build on Linux, these are the ones that matter for
Core work and do build:

```
internal/asterdoc  internal/workflowtree  internal/runworkspace  internal/protocol
internal/notebook  internal/designer      internal/shell         internal/state
internal/tools     internal/assets        internal/analysis      internal/assay(s)
internal/resultgrid internal/runui        internal/blocks        internal/blockui
internal/workflow  internal/theme         internal/state
```

Environment variables:

| Variable | Purpose |
|---|---|
| `LABWORKBENCH_DATA_DIR` | SQLite data directory (one per fixture/run) |
| `CARGO_TARGET_DIR` | build output; **set it explicitly on the VPS** |
| `GOTMPDIR`, `TMP`, `TEMP` | Go/MSVC scratch; keep off the data volume |
| `ASTERLAB_APP_BIN` | only for `workflow.inspect_tree` (needs the Windows app) |
| `LABWORKBENCH_CLI` | path to `labworkbench` for `scripts/rc3-fixture.py` |

Disk: the repo plus build caches reached ~7 GB. The Windows host ran out of
space at 452 GB full; do not assume a small VPS volume is enough. Keep
`CARGO_TARGET_DIR` and `GOTMPDIR` on a volume with headroom.

## 3. Verification ladder (run in this order)

1. `cargo test --bin labworkbench` — expect 10 passed.
2. `cargo test --manifest-path crates/asterlab-core/Cargo.toml` — 160 + 3 + 6.
3. `cargo test --manifest-path crates/asterlab-app/Cargo.toml` — 14.
4. `cd prototypes/mygo-run && go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native build -target host ./modules/runs && go test ./...` — all packages, using the host-native `libRuns.so`.
5. Build the CLI, then rebuild the acceptance fixture:
   ```bash
   LABWORKBENCH_CLI=$PWD/../../target/debug/labworkbench \
   ASTERLAB_DATA_DIR=/tmp/rc3-data python scripts/rc3-fixture.py
   ```
   Expect `assess: warning []` and **3** protocol tasks
   (`root/seed/seed`, `root/readout/incubate`, `root/readout/read`).
6. CLI acceptance on the produced ids (`/tmp/rc3-ids.json`):
   `run.workspace` shows the tasks; `asterdoc.assess` has no errors.
7. SDK probe: `sdk.validate` accepts a document using `rna_reagent_usage@1` and
   rejects `target_concentration > stock_concentration`.
8. Windows only: `go build -tags e2e`, native module check, then
   `scripts/gui-e2e-v11.ps1` (expect `GUI E2E PASS`).

## 4. What is done

| Commit | Content |
|---|---|
| `b267d3f` (tag `sdk-v1-rc3`) | `step@3.execution`, exact ProtocolBlock source, `STEP_NOT_MATERIALIZED` gate |
| `0ab2300` | Protocol is surface 6; frozen vs preview; design cap removed; page bodies extracted |
| `a7661b0` | Run recording-first + `历史` tab; Design docked inspector; Notebook prose-only + citations |
| `0b27fbd` | `sdk.validate`; rule registry; shared Go/Rust parity fixture; editable `string[]` |
| `ad19bbe` | `rna_reagent_usage@1` shipped end-to-end with no Core change |
| `a1a84d2` | final gate evidence |

Gate results at `a1a84d2`: Go `./...` pass; Rust 169/14/10; native module check
pass; real CCK-8 GUI E2E pass; six surfaces captured at 1360×880 and 900×600 in
both themes (`docs/product/evidence/`).

## 5. Not done — do not claim it

- `renderers.static` is not implemented; no module declares a static renderer.
- `object-list` fields are still read-only in the generic editor (`string[]` is
  editable).
- Below 1100px the Design inspector opens as an inline column, not a bottom
  sheet.
- The RNA multi-step Run acceptance (the `docs/sdk/rna-extraction-case` flow) has
  not been re-run after the six-surface change.
- No plugin ABI, no template market, no device control. The SDK is compiled-in
  modules with a stable contract, and the docs say so.
- `cargo fmt` is not clean repo-wide; do not run it wholesale, it buries review
  diffs.

## 6. Rules that must survive the move

1. `docs/product/ux-contract.md` is binding. A UI change cites a rule id or it is
   not reviewable.
2. `docs/product/ownership.md`: one writer per file; navigation and routing
   belong to Integration. Page bodies live in `*_page.go`, not `main.go`.
3. Facts are written only from Run through Core commands. Notebook writes prose
   and may cite facts. Tools sends drafts. Protocol is read-only.
4. A display-only module must never require a Core store/transaction change. The
   RNA module is the reference: manifest, Go spec, Rust validator, docs — no Core.
5. Adding a rule to `spec/modules.json` requires registering it in **both**
   `ASTERDOC_RULES` (Rust) and `knownModuleRules` (Go); both reject unregistered
   rules at startup.
6. Frozen tags and frozen Runs are immutable.

## 7. Next three steps

1. Re-run the Windows GUI gate on the merged `main` (the last run was on the
   feature branch tip; `a1a84d2` only adds docs and evidence, but re-run anyway).
2. Pick the next module from the audit's three classes and ship it the same way
   as the RNA one, recording the change-impact table.
3. Close one of the two SDK gaps (`renderers.static` or `object-list` editing)
   before adding more module kinds.
