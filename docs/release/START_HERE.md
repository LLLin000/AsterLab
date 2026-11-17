# Start AsterLab (Beta 0.1.0)

This page covers the Beta desktop app installed by the NSIS installer
(`evidence/RC-01/README.md`). The historical `START_HERE.bat` + browser flow
belongs to the Web package, which is not the Beta delivery surface
(`beta-scope.md` §1.4).

## Install

1. Run `[RELEASE-CUT: installer file name]`.
2. Accept the unsigned-app SmartScreen prompt (expected; see `README-BETA.md`).
3. Confirm **AsterLab MyGo** appears in the Start Menu.

Verified layout after install (`evidence/RC-01/README.md` §4):

- `%LOCALAPPDATA%\Programs\AsterLab MyGo\` holds `mygo-run.exe`, `Runs.dll`,
  `BUILD_INFO.txt`, `uninstall.exe`.
- HKCU `Software\Microsoft\Windows\CurrentVersion\Uninstall\AsterLabMyGo`
  carries `DisplayVersion 0.1.0-beta-unsigned`.
- Shortcuts: `%SMPROGRAMS%\AsterLab MyGo\AsterLab MyGo.lnk`,
  `%DESKTOP%\AsterLab MyGo.lnk`.

## Launch

1. Open **AsterLab MyGo** from the Start Menu or Desktop.
2. The `AsterLab` main window appears with workspace navigation and an empty
   design/run count on first run (`evidence/RC-01/first-launch.png`).
3. To keep data beside the install (or to test safely), set
   `LABWORKBENCH_DATA_DIR` to a folder before launching; RC-01 verified first
   launch this way without touching the live `%LOCALAPPDATA%\AsterLab`.

## Stop

Close the `AsterLab` window. (Sending `WM_CLOSE` exits the process within
2 seconds; `MYGO_0.1.0_BETA_RC_VERIFICATION.md` § Installation.)

## Uninstall

Run the registered uninstaller (`uninstall.exe /S`). Program folder, HKCU key,
and shortcuts are removed; the user database is preserved
(`evidence/RC-01/README.md` §6).

Stuck? See `TROUBLESHOOTING.md`. Back up first? See `BACKUP_RESTORE.md`.
