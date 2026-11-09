# AsterLab MyGo prototype

Isolated visual prototype; **not production code**.

Run:

```sh
go run .
```

Headless snapshot (no window):

```sh
go test -run TestSnap
```

The snapshot is written to `docs/design/mygo-prototype.png`.

It uses MyGo Native UI and the generated Rust Native Module. The module calls the shared `asterlab-app` library, which includes the existing CLI command implementation over SQLite. It shows:

- custom 40 DIP title bar (no Windows native title)
- project/design sidebar
- 2/3 record workspace + 1/3 readiness
- native checkbox, text inputs, buttons
- schedule D1/D3/D7
- SVG/path flow preview
- 96-well plate drawn as a 12-column grid

The schedule/flow/plate are synthetic; `LoadRun`/`SaveReadout` use the real SQLite Run. See `NATIVE_MODULE.md`.
