# EXP-07 evidence — native Timer, Pantry, RecordProjection, normalization, derive, sign/archive

Card: EXP-07 (Core) MyGo native module bridge for the EXP-02/05/06 Core APIs.
Commit under test: the EXP-07 branch (see the PR); base `80c2477`.

## 1. Acceptance commands

### Go module test

```sh
cd prototypes/mygo-run && go test ./modules/runs/... -count=1
```

Result: exit 0 (`go-test-modules-runs.out`).

```text
ok  	asterlab-mygo-prototype/modules/runs	0.142s
```

The test file is `modules/runs/runs_test.go`. It drives the real native bridge:
`NormalizeRuntime` arithmetic and rejection of a non-finite input, the pantry
upsert → list → label export → delete → reload round trip, and the errors a
timer, a projection, sign/archive and Design derivation return for a missing
Run.

### Rust bridge test

```sh
cargo test --manifest-path prototypes/mygo-run/modules/runs/rust/Cargo.toml
```

Result: exit 0 (`cargo-test.out`).

```text
running 47 tests
...............................................
test result: ok. 47 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 2.27s
```

41 of those are the CLI tests the crate already compiled; 6 are the new
`tests::` cases. The new cases seed a real SQLite database with a project,
design, design version and frozen Run and then exercise: runtime
normalization, pantry lifecycle, timer start/pause with a caller-supplied
clock, the Record projection (notes, assets, `recording` state), sign and
archive (manifest members, notes locked, second call rejected), and Run→Design
derivation (measured timer drift, frozen Run revision unchanged).

### Cross-platform generator gate (REL-05)

```sh
python3 scripts/ci/native_generator_hash.py --check
python3 scripts/ci/native_generator_hash.py --regen 2
python3 scripts/ci/test_native_generator_hash.py
```

Result: exit 0 (`native-generator-hash.txt`).

```text
native-generator-hash: interface hash 105c1545834f5199 format 2
native-generator-hash: input sha256 6ebab51b13b7474de9ab255ae2e2b0c2c35c634a9ae0027cef923d99d6822484
native-generator-hash: output sha256 c45fa8d94b42097d6a818af0d003cdc4ae206212a411b28a59ea7f528dfb7d4a
native-generator-hash:   prototypes/mygo-run/modules/runs/runs_native.go sha256 df62778282b1cc1e804807158a56bff69dd5ffcaa305d5e7560b54599f37044a drift none
native-generator-hash:   prototypes/mygo-run/modules/runs/rust/src/mygo.rs sha256 679bbb77a11ff3a42d2f26439190a992bd547b27e69ce4adc014ffe1065f927c drift none
native-generator-hash: OK golden scripts/ci/native_generator_hashes.json
native-generator-hash: OK 2 regeneration(s) identical, generator v0.8.4-0.20261108220316-c4197b7fe562
selftest: 10/10 passed
```

### Whole Go tree (CI parity)

```sh
cd prototypes/mygo-run && go test ./...
```

Result: exit 0, 22 packages ok (`go-test-all.out`). This is the check that
catches interface implementations outside the module.

## 2. Scope note

Two paths are outside the originally listed `write:` entries and were added to
the card `kandev-scope` on 2026-11-15 with the card owner's approval; `forbid`
became empty, so the write allowlist alone denies everything else:

| Path | Why it is required |
|---|---|
| `scripts/ci/native_generator_hashes.json` | Any change to `modules/runs/runs.go` changes the REL-05 input/output hashes. Leaving the golden stale makes `linux-tests` fail at `native_generator_hash.py --check`; `--update-golden` is the only documented way to record a reviewed binding. |
| `prototypes/mygo-run/internal/state/fake.go` | `DesignService` gained `DeriveDesignFromRun`. The explicit `FakeDesignService` in `internal/state` no longer satisfies the interface, which breaks `internal/designer` tests and `go test ./...`. One stub method restores it; no page UI is touched, matching the card's non-goal wording. |

The REL-04 gate now passes with the updated card (`scope-check.out`, 13 changed
paths, 0 violations); it still rejects every path the card does not list.

## 3. Redaction

Outputs are the commands' stdout/stderr. Absolute repository paths were replaced
with `<repo>`, and Go test scratch directories with `<tmp>`; no product values
or hashes were changed.
