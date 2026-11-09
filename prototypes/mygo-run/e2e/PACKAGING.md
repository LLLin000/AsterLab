# Packaging (UNSIGNED)

- Output: `prototypes/mygo-run/dist/` contains `mygo-run.exe`, `Runs.dll`, `BUILD_INFO.txt`.
- No installer, no code signing, no certificate.
- Users must keep exe and DLL in same folder.
- Label all artifacts "UNSIGNED PROTOTYPE".

- Unsigned zip: `scripts/mygo-package.ps1` → `release/AsterLab-MyGo-0.1.0-beta-unsigned-win-x64.zip`.
- Checksums: `BUILD_INFO.txt` (SHA256 for Runs.dll and mygo-run.exe).
