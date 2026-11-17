# AsterLab v0.1.0-beta release notes — DRAFT (RC-05, base `e2a4603`)

Status: draft for coordinator review. Every claim below is checked
against `docs/release/beta-scope.md` §4 (DoD) and §3 (non-goals);
anything without merged evidence at `e2a4603` is marked **[UNVERIFIED]**
or **[BLOCKED]** and must not ship as a promise. Non-goals are never
described as "coming soon".

## What this Beta is

- Windows 10/11 x64, single-user, offline local desktop app
  (`beta-scope.md` §1.1). No server, no cloud sync, no accounts.
- MyGo Native frontend `prototypes/mygo-run` (MyGo pin
  `v0.8.4-0.20261108220316-c4197b7fe562`) over the Rust Core authority
  (`crates/asterlab-core`, `crates/asterlab-app`, `src/cli.rs`); Go holds
  no business rules and never touches SQLite directly.
- Data: local SQLite (WAL), default
  `%LOCALAPPDATA%\AsterLab\data\labworkbench.sqlite`, overridable via
  `LABWORKBENCH_DATA_DIR`. Single machine, not shared across platforms.
- **Unsigned.** EXE, DLL, and setup exe are Authenticode `NotSigned`
  (verified `Get-AuthenticodeSignature` in RC-05); signing was not tested.
  This is the known state, not a defect.

## Verified capabilities (with evidence)

- Clean-machine install needs no Rust/Go/Node; the app opens a real
  window and creates the data dir/database (`DS-01`: RC-01 + RC-05
  drill at `e2a4603`, `integrity_check` = `ok`).
- Six surfaces reachable, one primary action each
  (`DS-02`, verified via automation snapshot at `e2a4603`; multi-size
  window screenshots: see mapping note).
- Design → save → Run → write readouts → restart → read back works
  through Core transactions; repeat submission IDs do not duplicate
  facts (`DS-05`: Go suite green in RC-04; GUI E2E step **[UNVERIFIED]**
  at `e2a4603` — not re-run in RC-05).
- Design/Protocol versioning: save creates a revision; frozen Runs keep
  their snapshot and reject stale revisions (`DS-03`/`DS-04`: Go
  revision/freeze tests PASS in RC-04; the GUI freeze badge screenshot
  **[UNVERIFIED]** at `e2a4603`).
- Write boundaries hold: Protocol has no edit controls, Notebook writes
  prose only, Tools sends unconfirmed drafts (`DS-06`: page tests PASS
  in RC-04 Go suite).
- SDK dual registration enforced on both sides; unregistered rules are
  rejected at startup (`DS-07`: Go + `cargo` suites; the
  `labworkbench sdk.validate` CLI probe **[UNVERIFIED]** at `e2a4603`).
- Legacy v0 database migrates automatically on first open: 4/4 rows
  preserved byte-exact, `user_version` 0 → 1, `integrity_check` ok
  (QA-05 + RC-02 + RC-05 drill at `e2a4603`).
- Install / overwrite-upgrade / uninstall / reinstall never creates,
  modifies, or deletes the user data dir (structural: the `.nsi`
  touches only INSTDIR, shortcuts, HKCU key); uninstall leaves zero
  residue and the DB is byte-identical afterwards (RC-02 + RC-05
  drill: hash `1ABEE49A…` across the uninstall/reinstall cycle).
- Backup/restore file semantics: closed-file copy passes
  `integrity_check`, restores the exact rows, and a simulated row loss
  recovers 4/4 rows with matching hashes (QA-05). **Limit:** only file
  semantics were proven; the in-app backup/restore buttons were not
  exercised end-to-end (**[UNVERIFIED]** UI path, `DS-09` partial).

## Known blockers and limits (ship as-is, do not soften)

- **Windows gate not fully green (`DS-08` [BLOCKED]):** one
  deterministic Windows-only cargo failure —
  `protocol_version_queries_return_expansion_and_stable_not_found`
  (open SQLite handle vs `remove_dir_all`, OS error 32); reproduces
  solo, never on Linux. Linux lane PASS, Windows build + full Go suite
  PASS (RC-04). Needs an owning fix card; no code was touched for it.
- **Platform-authority docs inconsistent (`DS-12` [UNVERIFIED]):** no
  superseding ADR for `0001-web-first-beta` was written in RC-05, and
  the `README.md` / `docs/architecture.md` frontend statements are
  untouched. Do not present this Beta as "the approved platform
  decision".
- **RC-3 fixture reproducibility (`DS-13` [UNVERIFIED]):**
  `scripts/rc3-fixture.py` was not re-run at `e2a4603` in RC-05.
- Concurrent restores / restore-during-writes: unsupported (QA-05
  NOT RUN). `operator` / `checked_by` are labels, not signatures.
- Non-goals (`beta-scope.md` §3, all excluded): plugin ABI, device
  control, multi-user/cloud/scheduled backup, e-signatures, code
  signing, a second (Web) frontend, Linux/macOS GUI, static renderers,
  full ISA/AnIML/OBI/RO-Crate I/O, shared data dirs, attachments,
  inventory/lot management, security audit.

## Assets (all `NotSigned`; `release/` is git-ignored, rebuildable)

- `AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe` —
  SHA256 `FDF1CB04…15905F` (16665902 bytes).
- `AsterLab-MyGo-0.1.0-beta-unsigned-win-x64.zip` —
  SHA256 `5B1008A7…7AD2CE` (20174283 bytes).
- dist: `mygo-run.exe` `2B3A4636…DC0DD59`,
  `Runs.dll` `F0F12C9A…5116212` (match installed copies).
- Rebuild: `scripts/mygo-build.ps1` →
  `makensis scripts/mygo-installer.nsi` →
  `scripts/mygo-package.ps1` (full hashes in `inventory.md`).

---

## Line-by-line mapping (draft claim → scope → verdict)

| Scope | Draft claim | Evidence at `e2a4603` | Verdict |
|---|---|---|---|
| §1.1 platform | Win x64 single-user offline desktop | RC-01 + RC-05 drill | verified |
| §1.2 deliverables | EXE/DLL/BUILD_INFO/setup/zip + SHA256 + unsigned | RC-05 `inventory.md` | verified (`DS-10`) |
| §3 non-goals (11) | excluded list above | `KNOWN_LIMITATIONS.md` + RC-03 docs | verified as excluded; full §3↔doc audit not redone → mapping partial |
| DS-01 install-and-launch | no-toolchain install, window, DB created | RC-01; RC-05 drill | verified |
| DS-02 six surfaces | all reachable, one action each | automation snapshot `e2a4603` | verified reachable; 900×600 + 1360×880 window shots **[UNVERIFIED]** here |
| DS-03 versioned writes | revision save / stale conflict | Go revision/rollback tests PASS (RC-04) | verified (Go); cargo side unaffected |
| DS-04 frozen runs | freeze badge, snapshot pinned | `asterdoc_freeze_test` PASS (RC-04) | verified (Go); GUI badge shot **[UNVERIFIED]** here |
| DS-05 restart round-trip | tx writes, idempotent repeats | Go suite PASS (RC-04); cck8/concurrency Windows runs skip on missing fixture (documented, Linux lane covers) | partial — GUI E2E step **[UNVERIFIED]** |
| DS-06 write boundaries | page-control tests | PASS in RC-04 Go suite | verified (Go) |
| DS-07 SDK dual registration | reject unknown, accept `rna_reagent_usage@1` | cargo + Go suites (RC-04) | verified (suites); CLI `sdk.validate` probe **[UNVERIFIED]** here |
| DS-08 full gate | Rust×3 + native check + Go + GUI E2E | 1 deterministic cargo failure | **[BLOCKED]** (RC-04) |
| DS-09 backup/restore on platform | file semantics proven; UI path not E2E'd | QA-05 (closed-file) | partial — UI path **[UNVERIFIED]** |
| DS-10 deliverables | complete + checksums + clean uninstall | RC-05 inventory + drill | verified |
| DS-11 docs match impl | no unimplemented claims | draft marks unverified items as such; RC-03 linkcheck | mapping partial (no fresh full audit) |
| DS-12 platform authority | new ADR + README/architecture fixes | none in RC-05 | **[UNVERIFIED]** — needs owning card |
| DS-13 rc3 fixture | `assess: warning []`, 3 tasks unchanged | not re-run in RC-05 | **[UNVERIFIED]** — needs VPS run |
| L-01…L-06 closure | design→run→reread; freeze; boundaries; module gate; delivery loop; recovery | L-06 recovery = DS-09 partial; rest per DS rows above | L-06 partial, rest as mapped |
