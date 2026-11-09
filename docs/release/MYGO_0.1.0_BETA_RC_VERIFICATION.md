# AsterLab MyGo 0.1.0-beta RC verification

Date: 2026-11-09 (Windows x64, authenticated user `Lin`)

## Build artifacts

- Source commit: `47a9aea74fa8f34a77a0b044a62022b53f648fa2`
- MyGo pin: `v0.8.4-0.20261108220316-c4197b7fe562`
- EXE: `prototypes/mygo-run/dist/mygo-run.exe` — SHA256 `1DFF7E1FDD5C25A4360DA2CAF8DA1582107C5F3D7939778C4D452061E721AC5F`
- Runs.dll: `prototypes/mygo-run/dist/Runs.dll` — SHA256 `A111647601CA572982E009FF023E669C54CD2BB06348FA42D6C3ECA9413FC039`
- NSIS installer: `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe` — SHA256 `6AB01924776E6F80973D04ACF8FE7939FA5C00D4E80D5323F7A2D0A86CC2997D`
- Authenticode status: all three artifacts `NotSigned`; signing **NOT TESTED** (no signing certificate used). Artifacts are explicitly unsigned.

## Installation, launch, persistence, uninstall

- Installer: **PASS**; elevated installer exited `0`.
- Installed at `C:\Program Files\AsterLab MyGo`; installed EXE/DLL hashes matched build hashes.
- Authenticated-user canonical data directory: `C:\Users\Lin\AppData\Local\AsterLab\data`; database: `C:\Users\Lin\AppData\Local\AsterLab\data\labworkbench.sqlite`.
- The canonical database already existed before this test (20,480 bytes; last modified 2026-11-09 11:58:57). This was not a newly provisioned Windows profile; verification used the existing authenticated profile without deleting its data.
- Launch: **PASS**. Installed executable displayed a real window, title `AsterLab MyGo Prototype`, HWND `14226002` on first observed launch; restart window title matched, HWND `26221236`. The initial batch launch exited before a window was captured; direct invocation of the installed EXE then produced the verified window.
- SQLite: `PRAGMA journal_mode` returned `wal`; `PRAGMA integrity_check` returned `ok`. No WAL fallback message appeared in captured stdout/stderr (`%TEMP%\mygo-out.log`, `%TEMP%\mygo-err.log`, and restart equivalents); app logs reported the canonical data path. Thus no fallback was observed; the successful WAL mode and committed write independently establish writability.
- Durable write test: committed row `rc_release_probe('release-2026-11-09','2026-11-09T12:30:00Z')`; it was present after graceful window close and full restart. Post-uninstall query still returned the row and journal mode `wal`.
- Startup: first measured restart window appeared in `1.12 s` from launch; idle working set after 10 seconds was `76.9 MiB`.
- Close: sending `WM_CLOSE` to the real window caused the process to exit within 2 seconds (**PASS**, graceful close observed).
- Uninstall: **PASS**; elevated silent uninstaller exited `0`. `C:\Program Files\AsterLab MyGo` no longer existed (no EXE/DLL residue). User database remained at the absolute path above and the committed probe remained queryable.
- Database SHA256 after uninstall: `68F31CF97111DBE75C02068DA2EE848A5B04B7B48018CC72CC7380FBE10F9491`.
