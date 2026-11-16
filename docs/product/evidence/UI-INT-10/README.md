# UI-INT-10 Windows Gate

## Build and environment

- Windows 11 build 26200; built with `scripts/mygo-build.ps1`.
- Build commit: `1efc593`; MyGo pin: `v0.8.4-0.20261108220316-c4197b7fe562`.
- `Runs.dll` SHA-256: `EEA6B335899D8B6C6F0202152DA9D9BDD70D902BE3C2642EAB855F969DAC9991`.
- `mygo-run.exe` SHA-256: `5E75D30F61879E695F7E96AC2B8A87E2D5D0C901ED690D45D95D3EB43784E631`.
- `go test ./... -count=1`: passed after the native module refresh.

## Scenarios

- Started the app with the uppercase URI argument `ASTERLAB://run/9158fa08-426f-4c0d-a20b-0dbaa607103c/well/A1`; the Run opened with well A1 selected.
- Seeded a disposable database with two Runs and started one countdown and one stopwatch through `modules/runs.RunService`. Both appeared in the global strip; pausing both removed the strip.
- Pressing the second Run's timer button navigated to that Run; pressing the first returned to the original Run.
- Captured inactive and active states in light and dark appearance at exact 900×600 and 1360×880 screenshot dimensions. Windows appearance registry values were temporarily set to dark for capture and restored to their original light values (`1/1`).
- The installer does not register the `asterlab:` URI scheme; external click-to-launch was not verified.

## Screenshots

| Appearance | Timer strip | Window | File |
|---|---|---:|---|
| Light | Inactive | 900×600 | [light-900-inactive.png](light-900-inactive.png) |
| Light | Active | 900×600 | [light-900-active.png](light-900-active.png) |
| Light | Inactive | 1360×880 | [light-1360-inactive.png](light-1360-inactive.png) |
| Light | Active | 1360×880 | [light-1360-active.png](light-1360-active.png) |
| Dark | Inactive | 900×600 | [dark-900-inactive.png](dark-900-inactive.png) |
| Dark | Active | 900×600 | [dark-900-active.png](dark-900-active.png) |
| Dark | Inactive | 1360×880 | [dark-1360-inactive.png](dark-1360-inactive.png) |
| Dark | Active | 1360×880 | [dark-1360-active.png](dark-1360-active.png) |
