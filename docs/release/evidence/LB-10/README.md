# LB-10 evidence — Runs bridge: asset unlink and coordinate update

Card: `LB-10 (Contract) Runs 桥接的资产解绑与坐标更新`
Base commit: `a339234` (`origin/main` after BE-05 #131 and #132)

## Environment

- `cargo 1.100.0`, `rustc 1.100.0`, `go 1.27.3 linux/amd64`, `python 3.12.3`
- Shared build cache used as-is (`CARGO_TARGET_DIR=/srv/build/asterlab-target`); no target-dir override.
- The branch was rebased onto `a339234` after BE-05 (#131) regenerated the same bindings and moved the
  golden; `runs.go` / `rust/src/lib.rs` were merged by hand, then the two generated files were re-emitted by
  the pinned generator and the golden rewritten. Nothing in the generated files was edited by hand.
- The host-native Runs library was built once with the pinned generator
  (`go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native check ./modules/runs`);
  the Go module tests load that library, so `go test ./modules/...` needs it first, as CI does.
- `go test ./modules/... ./internal/...` re-renders the golden PNG fixtures under `internal/**` as a test side
  effect. Those paths are outside this card's `scope.write`, so the re-rendered files were restored after the
  run and are not part of the commit.
- Repository paths in the captured output are rewritten to `<repo>`; no task-worktree absolute path appears in any artifact.

## Acceptance commands

| Command | Exit | Result |
|---|---:|---|
| `cargo test --bin labworkbench` | 0 | 51 passed, 0 failed |
| `cargo test --manifest-path crates/asterlab-core/Cargo.toml` | 0 | 435 + 3 + 6 + 2 passed, 0 failed |
| `cd prototypes/mygo-run && go build ./...` | 0 | no output |
| `cd prototypes/mygo-run && go test ./modules/... ./internal/... -count=1` | 0 | every package `ok` |

Raw files:

- `cargo-test-bin-labworkbench.txt`
- `cargo-test-asterlab-core.txt`
- `go-build.txt` (empty: a clean build prints nothing)
- `go-test-modules-internal.txt`

Supporting runs, not part of the card's acceptance list:

- `cargo-test-asterlab-app.txt` — CI also runs this suite; exit 0, 59 passed (`AppCore::raw_db_mut`).
- `rust-runs-module-tests.txt` — `cd prototypes/mygo-run/modules/runs/rust && cargo test`, exit 0, 72 passed,
  including the four new `update_asset_link_coordinates_*` / `unlink_asset_link_*` tests.
- `generator-check.txt` — `python3 scripts/ci/native_generator_hash.py --check`, exit 0
  (`interface hash 5ea2cab79d3ea6db`, `output sha256 9c44cfe1…`).
- `generator-regen.txt` — `--regen 2`, both regenerations `9c44cfe1…`, identical to the committed files.
- `go-bridge-verification.txt` — throwaway Go test against the generated bridge (see below).
- `go-bridge-events.txt` — `asset.events` / `asset.list` transcript for the same fixture.

## The Go-side check

The bridge methods are generated; the Go-level behaviour that needs proof is the
encode/decode round trip and the error prefixes. A throwaway `_test.go` in
`prototypes/mygo-run/modules/runs` exercised exactly that against a real frozen Run
(`scripts/rc3-fixture.py`, run `d083269b-…`), then was deleted:

1. `SaveAssetLink` → the link starts without coordinates (`wellId` empty, `FOVIndex 0`).
2. `UpdateAssetLinkCoordinates(runID, assetID, "A1", "t6h", 1)` → the returned link carries
   `A1 / t6h / 1`; `id`, `sha256` and `relativePath` are unchanged.
3. `UpdateAssetLinkCoordinates(…, "A", "t6h", 1)` → `INVALID: …`;
   `(…, "asset-not-linked", …)` → `NOT_FOUND: …`; `ListAssetLinks` still reports `A1 / t6h / 1`.
4. `UpdateAssetLinkCoordinates(…, "B2", "", 0)` → the zero values clear the timepoint and FOV.
5. `UnlinkAssetLink` → returns the stored link, `ListAssetLinks` is empty, a second call is `NOT_FOUND`.

Observed event trail for the same fixture (`go-bridge-events.txt`):

```
linked None previous= None
updated A1_t6h_fov01 previous= None
updated B2 previous= A1_t6h_fov01
unlinked B2 previous= None
```

`asset.list` afterwards returns `{"records":[]}`, i.e. the row is gone and only the event history remains.

The throwaway file was deleted before the commit and is not in the diff:
`prototypes/mygo-run/modules/runs/runs_test.go` already exists on `main`, but the card's `scope.write`
lists only `runs.go`, `runs_native.go` and `rust/src/**` under that module, so a committed Go test file
would fail `scope-check` and is outside this card's range. The same behaviour is covered permanently by the
Rust module tests (`rust-runs-module-tests.txt`), which drive the bridge impls against a real Core database.
Adding committed Go tests needs the card owner to add `prototypes/mygo-run/modules/runs/*_test.go` to
`scope.write`; the implementation must not widen the scope itself.

## What changed

- `prototypes/mygo-run/modules/runs/runs.go`: `RunService.UnlinkAssetLink` and
  `RunService.UpdateAssetLinkCoordinates`, with the identity and error contract in their doc comments.
- `prototypes/mygo-run/modules/runs/rust/src/lib.rs`: `unlink_asset_link_impl` /
  `update_asset_link_coordinates_impl` resolve the `asset_link` record id inside the Run, then call the
  existing `cli_impl::unlink_asset_link` / `cli_impl::update_asset_link`, so Core keeps the single copy of
  the coordinate rules and the transaction. Plus four unit tests.
- `src/cli.rs`: `update_asset_link` and `AssetLinkUpdateInput` are `pub(crate)` for the bridge; no behaviour change.
- `crates/asterlab-app/src/lib.rs`: `AppCore::raw_db_mut`, the mutable handle those writers need (same as the CLI adapter).
- `runs_native.go` / `rust/src/mygo.rs`: regenerated by the pinned generator; not hand-edited.
- `scripts/ci/native_generator_hashes.json`: golden rewritten in this commit (`--update-golden`), pin unchanged.
- `AGENT_API.md` §资产坐标 and `docs/contracts/asset-coordinates-v1.md` §6.1/§0/§8: the Go surface, the
  Go zero-value mapping, the error codes and the ambiguous-multi-coordinate rule.

Not in this card: no UI (LB-12 wires it), no Core semantics change, frozen Run untouched.
