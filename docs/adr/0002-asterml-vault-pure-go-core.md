# ADR 0002: `.asterml` Local Vault SSOT & Pure Go Unified Core (`astercore`)

- Status: accepted
- Date: 2026-11-20
- Supersedes:
  - Frontend Web-first clause in `docs/adr/0001-web-first-beta.md`
  - Authoritative SQLite `records` store & mandatory Rust FFI (`Runs.dll`) assumption in `docs/architecture/mygo-v1-contract.md` and `docs/release/beta-scope.md` §1.1

## Context

AsterLab's workspace has evolved to an Obsidian-grade single-vault local experiment workbench built on MyGo Native (`prototypes/mygo-run`, 100% Go) editing transparent `.asterml` files on disk. Maintaining a second authoritative backend in Rust + SQLite (`records` JSON blob table + `Runs.dll` `purego` FFI) caused severe architectural split-brain:

1. **Inverted source of truth**: SQLite owned experiments while `.asterml` was only a one-way shadow mirror, breaking free folder organization, external editors, Git/iCloud/OneDrive/Syncthing sync, and mobile standalone editing.
2. **Dual-language parity tax**: Maintaining duplicate AsterML parsers, module registries, unit tables, and validation rules across Go and Rust created drift and slowed iteration.
3. **Cross-platform packaging friction**: Shipping native dynamic/static Rust libraries (`Runs.dll` / `libRuns.so` / `.dylib` / Android `.so` / iOS `.a`) alongside a pure-Go MyGo binary added high maintenance overhead for a single-language GUI stack.

## Decision

1. **`.asterml` Local Vault is the Single Source of Truth (SSOT)**:
   - User experiments live as open, Markdown-compatible `.asterml` files inside user-managed `<Vault>/<ProjectFolder>/*.asterml` directories, alongside `./assets/` and `./templates/`.
   - **No file-format locking**: `.asterml` remains a clean, universally editable plain-text format (like `.md` in Obsidian) so external editors, Git, cloud sync, and AI Agents can always inspect and edit files cleanly. Any optional edit-lock or freeze guard lives strictly at the **application layer** as an opt-in UX feature, never as a mandatory lock baked into the `.asterml` format.
   - Syncable vault config (`.asterlab/config.json`) is separated from machine-local layout state (`.asterlab/local.json`). Pantry inventory migrates into human-readable files under `.asterlab/pantry/` so switching or syncing vaults keeps reagent records intact.

2. **Local index is downgraded to a disposable, rebuildable query cache**:
   - `.asterlab/cache/index.json` (zero-CGo pure Go incremental index) serves solely as a sync-excluded local acceleration cache for `MetadataCache`, full-text search, backlinks, active timers, and cross-experiment `LabBases` queries (and can upgrade transparently to FTS5 SQLite later if needed without changing `.asterml` data).
   - Deleting or corrupting the cache causes zero data loss; AsterLab rebuilds it automatically from `<Vault>/**/*.asterml`.

3. **Pure Go Unified Core (`astercore`) serves Desktop, Mobile, and CLI**:
   - All core logic—AsterML parsing/validation/UCUM units/formula evaluation, scientific diagnostics (`Diagnostic`, ARRIVE 2.0 rules, `MixPlan`, `TimeBudget`, `InstrumentProfile` matching), `VaultFS` (event bus + link rewriting + conflict-safe atomic writes + File Recovery snapshots), `FileManager`, `VaultIndex`, assay analyzers (CCK-8, RT-qPCR ΔΔCq, descriptive stats), and the Vault-native `cmd/asterlab` JSON CLI—converges into one Pure Go Core shared directly via Go package imports.
   - Desktop (Windows/macOS/Linux), Mobile (Android/iOS), and CLI compile cleanly without `Runs.dll` or CGo.

4. **Progressive retirement of Rust Core (`crates/asterlab-core` & `Runs.dll`)**:
   - Existing Rust Core crates and parity/CLI fixtures are retained intact as the golden reference during migration.
   - High-value scientific invariants, deterministic calculators, diagnostics, and CLI contract fixtures are ported into Go Core with equivalence tests. Once Go Core passes all equivalence gates, `Runs.dll` FFI and legacy SQLite-only UI routes will be formally archived.

## Consequences

- **100% frontend/backend/CLI consistency**: Single parser, single validator, single unit registry, zero FFI serialization overhead, and zero Go/Rust parity drift.
- **True offline & multi-platform parity**: Missing `Runs.dll` is no longer an error; desktop and mobile run the exact same Go Vault & Core engine.
- **External heavy compute stays pluggable**: If future workflows require heavy bioinformatics or 3D image processing, they integrate as optional out-of-process tools/sidecars rather than coupling everyday lab note editing to FFI.

## Boundaries

- All AsterML v2 composition laws remain binding: no assay-specific hardcoded modules or UI panels; `figure@1` remains visual-only and never stores primary structured readings.
- Scientific red lines remain strictly enforced in Go Core: `Run` count != independent `n`; technical replicates != independent experimental units; raw observations are never overwritten by derived results; unknown/not-applicable/zero stay distinct.
