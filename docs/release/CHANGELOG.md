# Changelog — AsterLab Beta 0.1.0

Only merged, evidenced capabilities are listed. Each entry names the merged
commit/PR and its evidence; anything without merged evidence is out of scope.
`[RELEASE-CUT]` values are filled when the release is cut.

- Version: `0.1.0-beta-unsigned` (`[RELEASE-CUT: confirm DisplayVersion]`).
- Source: `[RELEASE-CUT: commit hash]`; MyGo pin
  `v0.8.4-0.20261108220316-c4197b7fe562` (per `BUILD_INFO.txt`, RC-01 §1).
- Installer: `[RELEASE-CUT: setup exe file name + SHA-256]`.
  Reference build: `AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe`
  (SHA-256 `66059cfe…9e0b7899`), built from `origin/main` @ `45d7ee8` with
  `scripts/mygo-installer.nsi` unmodified — `evidence/RC-01/README.md` §1–2.

## V2 panels (Design / Run / Record)

- **Design panel**: all plate formats with budget bars — UI-DES-10,
  merged `ad8ed02` (PR #152).
- **Run panel**: run bench with timer and 12-well × 6-field imaging —
  `feat(ui-run-10)`, merged `579febf` (PR #164).
- **Record panel**: same-source dual rendering (auto board + terminal
  archive) — UI-REC-10, merged `898d002` (PR #165).
- **Experiment interactions**: blind switch, 1D lanes, exclusion, typed
  readout — UI-EXP-10, merged `a31e04c` (PR #173); global timer strip and
  run-well deep links — `feat(ui-int-10)`, merged `9e8a87a` (PR #162).
- Smoke evidence that the panels ship in the installed app: RC-01 first
  launch shows the `AsterLab` main window with workspace nav
  (`evidence/RC-01/first-launch.png`).
- V2 design-system consistency baseline: LB-18, merged `eef6d88` (PR #155);
  contract coordinates behind the panels: LB-00/LB-08/LB-10
  (`evidence/LB-00`, `evidence/LB-08`, `evidence/LB-10`).

## Bridge v2 (MyGo ↔ Rust Core)

- Generic command/query channel + BE-08/09 exposure — INT-01, merged
  `6b1f3be` (PR #160); generic channel routing to Core authority — BE-10,
  merged `a37084f` (PR #163).
- Go does not touch SQLite or duplicate business rules; normal calls avoid
  `os/exec` (`beta-scope.md` §1.1). Native-bridge acceptance for the
  EXP-02/05/06 Core APIs: `evidence/EXP-07/` (cargo + Go suites green).

## PDF archive export

- Terminal record archive PDF export with embedded image board —
  `feat(ui-rec-pdf)`, merged `2ba3369` (PR #172).
- Semantics (frozen-source hash check, revision/source-hash/Run-ID header,
  wrapped CJK pagination, temp-file + rename write): `pdf-export.md`.
- Record text/vocabulary behind the export: RDO-01
  (`evidence/RDO-01/acceptance.txt`, L1 green).

## Installer (Windows x64, user-level, unsigned)

- Fresh user-level install + first launch + zero-residue uninstall verified —
  `evidence/RC-01/README.md` (base `origin/main` @ `45d7ee8`).
- Unsigned Authenticode status disclosed; signing not tested
  (`MYGO_0.1.0_BETA_RC_VERIFICATION.md`).
- Windows CI/build caching behind reproducible builds: `75541e2` (PR #171);
  generator-hash reproducibility: `evidence/REL-05/`.

## Data safety

- Legacy v0 → v1 SQLite migration with exact row preservation + failed
  migration leaves the DB untouched: `evidence/QA-05/` (see
  `BACKUP_RESTORE.md`).
- Three-protocol CLI regression fixtures (HUVEC / RT-qPCR / in-vivo):
  QA-09, merged `b33e330` (PR #153), `evidence/QA-09/results.json`.
- File-reference security boundary (`asset.link` relative-path only,
  `INVALID_INPUT` on escape): QA-06, `evidence/QA-06/`.

## Non-goals (not in this Beta)

Per `beta-scope.md` §3: plugin ABI, multi-user/collaboration/cloud sync,
electronic signatures, code signing, a second (Web) frontend, Linux/macOS GUI.
