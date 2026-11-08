# Frozen Run read-only adapter

Minimal proof that Rune can consume an existing `RunWorkspace` snapshot without touching SQLite.

- Input: `run-workspace.json`, exported read-only from an existing frozen Run.
- Output: timepoint / task title / status, plus measurement values.
- It does not open the database and has no write path.

Build inside the pinned Rune clone with the `aster-spike` dependency setup:

```bash
mkdir -p examples/run-read/src
cp Cargo.toml examples/run-read/Cargo.toml
cp main.rs examples/run-read/src/main.rs
cp run-workspace.json examples/run-read/run-workspace.json
cargo run -p run-read -- --snap run-read.png --scale 2 --theme light
```
