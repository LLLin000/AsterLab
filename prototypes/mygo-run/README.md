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

It uses MyGo Native UI only: no HTML, JS, WebView, or Rust bridge. It shows:

- custom 40 DIP title bar (no Windows native title)
- project/design sidebar
- 2/3 record workspace + 1/3 readiness
- native checkbox, text inputs, buttons
- schedule D1/D3/D7
- SVG/path flow preview
- 96-well plate drawn as a 12-column grid

The data is synthetic. MyGo's Rust bridge was not tested.
