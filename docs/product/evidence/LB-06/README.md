# LB-06 — Run asset coordinates

## 已验证

Real-window captures are from the Windows app at 1360×880, light theme. The app build stamp was `3876cec`, matching HEAD at capture time; `mygo-run.exe` SHA-256: `774CAE8BAD95257F58CBF340D842E4C4C5A4D7FAF37883FB94AB15E59B226720`; `Runs.dll` SHA-256: `46FE0A555166C16572084D06333869AAADA6576352077168047859F0AF3C7E4F`.

- `coordinate-form-1360x880-light.png`
- `asset-matrix-gaps-1360x880-light.png`
- `gap-prefill-1360x880-light.png`
- `unlabelled-readonly-1360x880-light.png`

## 未验证

The batch-file-picker path is **NOT VERIFIED**: the app opens an app-owned native dialog that this automation harness cannot drive. Batch coordinate inheritance is covered by unit tests only.

## Core gap

An existing unlabelled asset is shown read-only as `未标注 · 需要 Core 提供坐标更新/解除关联接口`, with no action. `runs.RunService` exposes only `SaveAssetLink` and `ListAssetLinks`; there is no Unlink/coordinate-update method. Coordinate updating is tracked by LB-08.
