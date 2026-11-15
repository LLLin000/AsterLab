# LB-05 — Real-window evidence: 12-well plate and dilution totals

## Seed and build

- Isolated GUI data: `%TEMP%\lb05-clean\data`; design `9cf2057b-6285-4973-a9f5-cfd32f447332`.
- Created a project and design with the AsterLab CLI, then seeded the MyGo design extension in the isolated fixture: one 12-well plate block, three groups (3%, 5%, 7%) with four assigned wells each, and a dilution calculator with a saved per-well result of 20.0 µL stock + 180.0 µL diluent. Each group total is 80.0 µL stock + 720.0 µL diluent.
- Captured from a launched `prototypes/mygo-run/dist/mygo-run.exe`, with `ASTERLAB_THEME=light` and the real window resized by `user32 MoveWindow` to 1360×880 or 900×600. Images contain the client area (1344×871 or 884×591 pixels).
- Build commit: `51502bb` (`BUILD_INFO.txt` matched `git rev-parse --short HEAD` at capture).

```text
Runs.dll SHA256 0B82665D1AE1C8774568A1CACAD37F0F2FD34D366200F5173FF5526D1016D13F
mygo-run.exe SHA256 C8FF3590317A37A37EEB80041929663D9C809C748566A3D0C6B3307DE29F3CD5
```

UX rules: `UX-P4` (content decides layout), `UX-LAYOUT`; Design requirement `UI-REDESIGN §2.1` (12 孔板 ↔ 配液单).

## Captures

| Real-window image | Window size | Theme | What it shows | PNG SHA256 |
|---|---:|---|---|---|
| `lb05-real-plate-1360x880-light.png` | 1360×880 | light | 3×4 plate geometry, rows A–C and columns 1–4; 12 wells across three groups | `6be92888e3838c2ee6d1192e0b5ae4c5953312b332db5035c305a5bebc6ddcaf` |
| `lb05-real-plate-900x600-light.png` | 900×600 | light | Same 3×4 geometry and group assignment at narrow size | `622add332e33182af5d0ebd48d623175205046a01ec0daa83ad536c81e5f783f` |
| `lb05-real-dilution-1360x880-light.png` | 1360×880 | light | Per-well 20.0/180.0 µL and 4-well group totals 80.0/720.0 µL | `8b8b9784e1d7de39f673400944b3684f462ceea3c6af8884a0d862e7f46e854f` |
| `lb05-real-dilution-900x600-light.png` | 900×600 | light | Per-well 20.0/180.0 µL and group totals at narrow size | `3bc20246f32f06f0bce57742370a00da2966cf8d64cc9c9b35e90ca9bf41e263` |

Each screenshot has an adjacent accessibility snapshot (`*.txt`). Dilution captures are scrolled to show the per-well calculations and all three group totals together.