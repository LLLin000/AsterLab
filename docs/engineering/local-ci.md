# Local CI: Windows Gate and Linux lane

Status: binding from 2026-11-14. GitHub-hosted runner minutes are exhausted, so
pull-request verification runs on the two real hosts instead of Actions.

## Who runs what

| Lane | Host | Command | Never runs |
|---|---|---|---|
| Windows Gate | Windows workstation | `./scripts/ci/local-gate.ps1 -Pr <n>` | any Linux test, `cargo test` of the Rust targets |
| Linux lane | VPS | `scripts/ci/local-gate.sh --label <name>` | Windows build, GUI, screenshots |

The Windows Gate lane does not run Linux tests even though it could: the Linux
lane owns them, and a second copy of the same suite on a different OS hides
which lane a failure belongs to. `windows-build.yml` and `linux-tests.yml`
therefore keep only `workflow_dispatch`. `scope-check` and `linux-tests` still
run on pull requests because their runner is the self-hosted VPS and costs no
minutes; they stay in the `main` ruleset, and the Windows lane never runs the
Linux suite itself.

## Windows Gate gate

```powershell
./scripts/ci/local-gate.ps1 -Pr 69
```

Steps, mirroring the retired workflows minus every Linux-only step:

1. `scripts/ci/test_scope_check.py` then `scripts/ci/scope_check.py` against the
   PR diff and the card's `<!-- kandev-scope -->` block.
2. REL-05 native generator hashes: selftest, `--check`, `--regen 2`.
3. `scripts/mygo-build.ps1` -> `prototypes/mygo-run/dist/{mygo-run.exe,Runs.dll}`.
4. `go test -tags=e2e -count=1 ./...` for `prototypes/mygo-run`.
5. The build must not rewrite committed bindings.
6. SHA256 of both artifacts into `target/local-gate/<label>/artifacts.txt`.

Exit code 0 means PASS; the report lands in
`target/local-gate/<label>/report.md` and is not committed (`/target` is
ignored). Paste the `LOCAL_GATE: PASS` line with the artifact hashes into the
Kandev card evidence section, not into git.

## Linux lane gate

```bash
scripts/ci/local-gate.sh --label RUN-05
```

L0 `cargo build` and L1 `cargo test` for `labworkbench`, `crates/asterlab-core`
and `crates/asterlab-app`; REL-05 hashes; native `Runs` module for
`linux-amd64`; `go test ./...`. Report: `target/local-gate/<label>/report.md`.

## Per-card checkouts

`ovnfs new --name <card> --rev origin/main` copies the source folder's refs, and that
folder is often on a feature branch with a stale `origin/main`. Every checkout must
start with

```bash
git fetch origin main && git merge origin/main
```

before branching; otherwise the card base is older than the CI scripts, the evidence
directories and the other cards it was written against, and `local-gate.ps1` will not
even exist. Merging (never rebasing, never force-pushing) keeps the other lanes'
commits intact.

A checkout also inherits the source folder's ignored `prototypes/mygo-run/dist/`, which can
be months old. Delete it and rebuild before taking any evidence:

```powershell
rm -rf prototypes/mygo-run/dist
./scripts/mygo-build.ps1
grep commit prototypes/mygo-run/dist/BUILD_INFO.txt   # must equal git rev-parse --short HEAD
```

A screenshot whose `BUILD_INFO.txt` names a different commit is not evidence: it shows a
retired frontend. This bit once — a lane screenshotted a November 9 build of a card base
that was itself 106 commits behind `main`.

## Merge rule

A card merges when its lane reports `LOCAL_GATE: PASS` on the exact head commit
being merged, and `scope-check` (self-hosted) is green when the runner is
online. Required status checks on `main` no longer list `linux-tests`: a job
that cannot run must not be able to block a merge.

Evidence stays on the card and in the PR body; the machine-readable report is a
local artifact, so no lane can rewrite history to look green.
