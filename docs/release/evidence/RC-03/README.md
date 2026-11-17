# RC-03 evidence — Beta release docs

Card: RC-03 (docs/release: README-BETA, START_HERE, BACKUP_RESTORE, CHANGELOG,
TROUBLESHOOTING + this evidence dir). No app code touched; `crates/`,
`src/`, and `prototypes/` are out of scope and were not modified.
Base: `origin/main` @ `[RELEASE-CUT: commit hash]` (fresh ovnfs checkout
`rc03`, `git reset --hard && git clean -fdx`, merged `origin/main`).
Reference build values below are from RC-01 (`evidence/RC-01/README.md`,
base `45d7ee8`); `[RELEASE-CUT]` placeholders in the docs are filled when the
release is cut.

## Link check

Script: [`linkcheck.py`](linkcheck.py) — every relative link in the six Beta
docs must resolve on disk. Run from the repo root:

```text
python docs/release/evidence/RC-03/linkcheck.py
```

Record (2026-11-17, checkout `rc03` @ `4fc6c54` + RC-03 work):

```text
checked 68 relative links in 6 files
all relative links resolve: OK
```

## Per-claim evidence mapping

| Doc claim | Evidence source |
|---|---|
| Install layout (`mygo-run.exe`, `Runs.dll`, `BUILD_INFO.txt`, `uninstall.exe` under `%LOCALAPPDATA%\Programs\AsterLab MyGo`; HKCU key; shortcuts; `/S` silent install) | `evidence/RC-01/README.md` §§1–4; BUILD_INFO fields (commit `45d7ee8`, mygo pin `v0.8.4-0.20261108220316-c4197b7fe562`, SHA-256 `F11574…`, `350AAD…`), installer SHA-256 `66059cfe…9e0b7899` |
| First launch reaches usable main window; `first-launch.png`; isolated `LABWORKBENCH_DATA_DIR`; zero-residue uninstall preserving user DB | `evidence/RC-01/README.md` §§5–6 |
| Unsigned status is known state, not a defect; signing NOT TESTED | `../../beta-scope.md` §1.2; `../../MYGO_0.1.0_BETA_RC_VERIFICATION.md` (Authenticode `NotSigned`) |
| Graceful close (WM_CLOSE ≤ 2 s); WAL mode + `integrity_check ok`; probe row survives restart/uninstall | `../../MYGO_0.1.0_BETA_RC_VERIFICATION.md` §§ Installation, Close |
| Backup closed-file copy semantics (4 rows, `ok`, identical hashes); restore of deleted row; auto v0→v1 migration; failed migration leaves DB untouched; concurrent restore NOT RUN/unsupported | `evidence/QA-05/README.md` + `legacy-migration-output.txt`, `failed-migration-output.txt` |
| V2 panels shipped (Design plate formats UI-DES-10; Run bench ui-run-10; Record dual-render UI-REC-10; EXP interactions UI-EXP-10; timer strip ui-int-10) | Merged commits `ad8ed02` (#152), `579febf` (#164), `898d002` (#165), `a31e04c` (#173), `9e8a87a` (#162); in-app smoke via `evidence/RC-01/first-launch.png` |
| Bridge v2 channel + routing; Go never touches SQLite | Merged `6b1f3be` (#160), `a37084f` (#163); `../../beta-scope.md` §1.1; `evidence/EXP-07/` |
| Record archive PDF export + frozen-hash rejection semantics | Merged `2ba3369` (#172); `../../pdf-export.md`; `evidence/RDO-01/acceptance.txt` |
| File-reference boundary (`INVALID_INPUT` on escape) | `evidence/QA-06/README.md` |
| Three-protocol CLI regression (HUVEC/RT-qPCR/in-vivo) | QA-09, merged `b33e330` (#153); `evidence/QA-09/results.json` |
| Non-goals excluded from the Beta | `../../beta-scope.md` §3 |

## Scope discipline

- Nothing in these docs describes an unmerged or unevidenced capability; the
  CHANGELOG lists only the merged items above.
- Every TROUBLESHOOTING recovery cites its evidence source; the closing
  "Not verified" section names what was deliberately left out.
- `../../START_HERE.bat` (historical Web-package launcher) is documented as such in
  `../../START_HERE.md`, not presented as the Beta path.
