# SDK-02-FIX readout@1 validation evidence

## Go package tests

Command: `cd prototypes/mygo-run && go test ./internal/asterdoc`

Raw output:

```text
ok   	asterlab-mygo-prototype/internal/asterdoc	0.404s
```

Exit code: 0.

## Go/Rust parity command

Command: `python3 tests/sdk-parity/run.py`

Output (local worktree prefix redacted):

```text
python3: can't open file '<worktree>/tests/sdk-parity/run.py': [Errno 2] No such file or directory
```

Exit code: 2. The parity runner path is absent from this task worktree, so the acceptance criterion that `readout@1` no longer appears in its difference report could not be exercised here.

## Error behavior

Go `Parse` exposes validation failures as descriptive `error` strings. Rust `ReadoutSchema` validation emits typed `ReadoutIssueCode` values. This Go parser has no stable error-code field to map to the Rust codes; this change preserves rejection semantics and identifies offending fields in the Go error text. The parity runner could not be run in this worktree to confirm its difference report.
