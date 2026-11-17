# RC-05 inventory — Beta 0.1.0 release assets (commit `e2a4603`)

Release SHA: `e2a460322416445bd78ce55a4a6b2af9544196d2`
(`docs(rc-04): full Linux + Windows gates evidence on d7d25e8 (#180)`, 2026-11-17;
`origin/main` at dispatch; checkout `rc05` pinned to this SHA for the whole run.)
Host: Windows x64. Cache prefixes used for every build
(`C:` is nearly full):
`CARGO_TARGET_DIR=D:/.ci-cache/cargo-target`,
`GOCACHE=D:/.ci-cache/go`, `TEMP=TMP=D:/.ci-cache/tmp`.

## 1. dist (built in-checkout, `scripts/mygo-build.ps1`, no source changes)

`prototypes/mygo-run/dist/` (git-ignored, not committed):

```text
AsterLab MyGo Prototype (UNSIGNED)
commit e2a4603
mygo pin v0.8.4-0.20261108220316-c4197b7fe562
Runs.dll SHA256 F0F12C9A45C6F6DBA3DAB69088C0843F6BD90D39955EE686377956DE55116212
mygo-run.exe SHA256 2B3A4636B19D17898E73A4D1F4853A147BDC09437505FAAB1D0B1B9E4DC0DD59
```

Independent re-hash (this run): `BUILD_INFO.txt`
`E11A37327406A0C292E9212CDC338EC3FA7E00EE03C8521C827A6AB638823A3D`,
`Runs.dll` `F0F12C9A…55116212`, `mygo-run.exe` `2B3A4636…1B9E4DC0DD59` — match.
`BUILD_INFO.txt` commit `e2a4603` == `git rev-parse --short HEAD` (match).

## 2. Installer (`scripts/mygo-installer.nsi`, unmodified, makensis 3.13)

- `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64-setup.exe`
  SHA256 `FDF1CB04ED87B04B56CD886C5EB23F2B7D200224A383E0D2F9268909AE15905F`
  (16665902 bytes). Only benign 6001 MUI warnings (same class as RC-01).
- Authenticode: `NotSigned` on EXE, DLL, and setup exe
  (`Get-AuthenticodeSignature`; signing NOT tested — no certificate).
- Git-ignored (`/release/` in `.gitignore`); not committed. Rebuild:
  `scripts/mygo-build.ps1` then `makensis scripts/mygo-installer.nsi`.

## 3. Zip (`scripts/mygo-package.ps1`, unmodified)

- `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64.zip`
  SHA256 `5B1008A712EF4FD4DDBF103FE53DEA5C33F24A4EA600CA8F6C35DDAD7AD2CE`
  (20174283 bytes). Contains `mygo-run.exe`, `Runs.dll`, `BUILD_INFO.txt`,
  `UNSIGNED.txt`. Git-ignored; rebuild via `scripts/mygo-package.ps1`
  after the dist build. (The pre-existing
  `release/AsterLab-0.1.0-beta-win-x64.zip` is the older Web-package zip,
  untouched by this card.)

## 4. What is NOT in `release/` / not committed

No tags were pushed and no GitHub release was created (per card rules;
tag push + release creation belong to the coordinator after review).
