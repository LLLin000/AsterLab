#!/usr/bin/env bash
# scripts/ci/local-gate.sh
#
# Linux lane CI, run on the VPS instead of GitHub Actions.
#
# Why: hosted-runner minutes are exhausted. The Linux lane owns every Linux
# test, so the Windows Gate lane never runs them (docs/engineering/local-ci.md).
#
# What it runs (same contract as .github/workflows/linux-tests.yml):
#   L0 cargo build  - bin labworkbench + crates/asterlab-core + crates/asterlab-app
#   L1 cargo test   - the same three targets
#   REL-05 native generator hashes (two Linux generations)
#   native Runs module for linux-amd64 + Go tests
#
# Usage: scripts/ci/local-gate.sh [--label <name>] [--skip-go]
# Exit 0 = PASS, 1 = FAIL. Report: target/local-gate/<label>/report.md

set -uo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

label="local-$(git rev-parse --short HEAD)"
skip_go=0
while [[ $# -gt 0 ]]; do
    case "$1" in
        --label) label="$2"; shift 2 ;;
        --skip-go) skip_go=1; shift ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

report_dir="target/local-gate/$label"
rm -rf "$report_dir"
mkdir -p "$report_dir"
report="$report_dir/report.md"
log_file="$report_dir/report.log"
: >"$log_file"

log() { printf '%s\n' "$*" | tee -a "$log_file"; }
failures=0

step() {
    local name="$1"; shift
    log ""
    log "## $name"
    if "$@" >>"$log_file" 2>&1; then
        log "RESULT: PASS"
    else
        log "RESULT: FAIL - $name (exit $?)"
        failures=$((failures + 1))
    fi
}

cargo_l0() {
    cargo build --locked --bin labworkbench &&
        cargo build --locked --manifest-path crates/asterlab-core/Cargo.toml &&
        cargo build --locked --manifest-path crates/asterlab-app/Cargo.toml
}

cargo_l1() {
    cargo test --locked --bin labworkbench &&
        cargo test --locked --manifest-path crates/asterlab-core/Cargo.toml &&
        cargo test --locked --manifest-path crates/asterlab-app/Cargo.toml
}

native_hashes() {
    python3 scripts/ci/test_native_generator_hash.py &&
        python3 scripts/ci/native_generator_hash.py --check &&
        python3 scripts/ci/native_generator_hash.py --regen 2
}

go_gate() {
    (
        cd prototypes/mygo-run &&
            go run github.com/egoist/mygo/cmd/mygo@v0.8.4-0.20261108220316-c4197b7fe562 native build -target host ./modules/runs &&
            sha256sum .mygo/native/linux-amd64/libRuns.so &&
            go test ./...
    )
}

log "# Linux lane local CI - $label"
log ""
log "- repo: $repo_root"
log "- commit: $(git rev-parse HEAD)"
log "- host: $(uname -srm)"
log "- rustc: $(rustc --version 2>/dev/null || echo missing)"
log "- go: $(go version 2>/dev/null || echo missing)"
log "- lane: Linux only; Windows Gate never runs these (docs/engineering/local-ci.md)"

step "L0 build" cargo_l0
step "L1 tests" cargo_l1
step "native-generator-hash" native_hashes
if [[ $skip_go -eq 0 ]]; then
    step "native-runs-and-go-tests" go_gate
fi

verdict=PASS
if [[ $failures -ne 0 ]]; then
    verdict=FAIL
fi

log ""
log "## Summary"
if [[ $verdict == PASS ]]; then
    log "All Linux lane checks passed."
else
    log "$failures step(s) failed."
fi
log ""
log "LOCAL_GATE: $verdict"

{
    printf '# Linux lane local CI report\n\n'
    printf -- '- label: %s\n' "$label"
    printf -- '- commit: %s\n' "$(git rev-parse HEAD)"
    printf -- '- generated: %s\n\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    cat "$log_file"
} >"$report"
echo "report: $report"

[[ "$verdict" == PASS ]]
