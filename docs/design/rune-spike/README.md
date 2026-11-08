# Rune feasibility spike source

This is the exact source used for [RUNE_EVALUATION.md](../RUNE_EVALUATION.md), pinned to Rune `a7f276c3630f85a06a0230295da4d2bd172d8d3b`.

It is a standalone example, not part of the AsterLab crate.

```bash
git clone --filter=blob:none --depth 1 --branch dev https://github.com/anomalyco/rune /tmp/rune-spike
cd /tmp/rune-spike
git checkout a7f276c3630f85a06a0230295da4d2bd172d8d3b
mkdir -p examples/aster-spike/src
cp <repo>/docs/design/rune-spike/Cargo.toml examples/aster-spike/Cargo.toml
cp <repo>/docs/design/rune-spike/main.rs examples/aster-spike/src/main.rs
# add "examples/aster-spike" to the workspace members
cargo run -p aster-spike -- --snap out.png --scale 2 --theme light --tree tree.json
```

The long Chinese paragraph intentionally exercises CJK line breaking. At this revision it emits `ICU4X data error: No segmentation model for complex script: Chinese/Japanese` and overflows.
