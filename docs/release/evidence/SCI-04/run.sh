#!/usr/bin/env bash
# SCI-04 evidence: sample-level execution identity and workflow.progress.
#
# Builds on the reviewed RNA extraction fixture (frozen Run) without modifying
# it: the database is copied to a temp dir, events are appended through the
# public CLI, and the projection is read back.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
out="$root/docs/release/evidence/SCI-04"
data="$(mktemp -d)"
trap 'rm -rf "$data"' EXIT

cp "$root/prototypes/mygo-run/testdata/rna-case/labworkbench.sqlite" "$data/labworkbench.sqlite"
export LABWORKBENCH_DATA_DIR="$data"
cd "$root"

run() {
  cargo run --quiet --bin labworkbench -- "$@" 2>/dev/null
}

RUN="ad9b452e-f8ea-404b-9a2a-7124f9403b02"
PV="2066e8ba-17c9-48be-976a-64e4ddde64a6"
STEP="root/rna-extraction/02-lyse"

echo "== schema workflow.progress ==" > "$out/schema-workflow-progress.out"
run schema workflow.progress >> "$out/schema-workflow-progress.out"

: > "$out/event-identity.out"
# attempt 1: S1 completes, S2 is blocked.
s1="$(run execution_event.create --input - <<EOF
{"run_id":"$RUN","event_type":"operation","content":"S1 lysis complete","step_status":"completed","protocol_step_path":"$STEP","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S1"],"attempt":1,"occurred_at":"2026-11-13T09:00:00Z","checks":[{"check_id":"c02-lysis","status":"passed"}]}
EOF
)"
s2="$(run execution_event.create --input - <<EOF
{"run_id":"$RUN","event_type":"deviation","content":"S2 centrifuge interrupted","deviation":"centrifuge interrupted for 5 min","step_status":"blocked","protocol_step_path":"$STEP","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S2"],"attempt":1,"occurred_at":"2026-11-13T09:10:00Z"}
EOF
)"
s1_id="$(printf '%s' "$s1" | python3 -c 'import json,sys;print(json.load(sys.stdin)["result"]["record"]["id"])')"
s2_id="$(printf '%s' "$s2" | python3 -c 'import json,sys;print(json.load(sys.stdin)["result"]["record"]["id"])')"
# attempt 2: S2 is retried and completes.
s2_retry="$(run execution_event.create --input - <<EOF
{"run_id":"$RUN","event_type":"operation","content":"S2 lysis complete after retry","step_status":"completed","protocol_step_path":"$STEP","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S2"],"attempt":2,"occurred_at":"2026-11-13T10:00:00Z","checks":[{"check_id":"c02-lysis","status":"passed"}],"lineage":[{"from_kind":"execution_event","from_id":"$s2_id","to_kind":"step","to_id":"$STEP","relation":"retry"}]}
EOF
)"
s2_retry_id="$(printf '%s' "$s2_retry" | python3 -c 'import json,sys;print(json.load(sys.stdin)["result"]["record"]["id"])')"
# Same identity again: must return the existing record, not a new fact.
s1_duplicate="$(run execution_event.create --input - <<EOF
{"run_id":"$RUN","event_type":"operation","content":"S1 lysis duplicate","step_status":"completed","protocol_step_path":"$STEP","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S1"],"attempt":1,"checks":[{"check_id":"c02-lysis","status":"passed"}]}
EOF
)"
s1_duplicate_id="$(printf '%s' "$s1_duplicate" | python3 -c 'import json,sys;print(json.load(sys.stdin)["result"]["record"]["id"])')"

{
  echo "attempt 1 S1 event:      $s1_id"
  echo "attempt 1 S2 event:      $s2_id (deviation, blocked)"
  echo "attempt 2 S2 retry:      $s2_retry_id (lineage relation=retry)"
  echo "duplicate S1 attempt 1:  $s1_duplicate_id"
  echo "duplicate === original:  $([ "$s1_duplicate_id" = "$s1_id" ] && echo yes || echo NO)"
  echo
  echo "== attempt gap is rejected =="
} >> "$out/event-identity.out"
set +e
run execution_event.create --input - <<EOF > "$out/attempt-gap.out"
{"run_id":"$RUN","event_type":"operation","content":"S1 skipped attempt","step_status":"completed","protocol_step_path":"$STEP","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S1"],"attempt":4,"checks":[{"check_id":"c02-lysis","status":"passed"}]}
EOF
gap_exit=$?
set -e
echo "exit=$gap_exit" >> "$out/attempt-gap.out"

run workflow.progress --input - <<EOF > "$out/workflow-progress-retry.out"
{"run_id":"$RUN","step_path":"$STEP"}
EOF

# Partial matrix: one sample starts another step, the second sample never appears.
run execution_event.create --input - <<EOF > /dev/null
{"run_id":"$RUN","event_type":"operation","content":"S1 add ethanol in progress","step_status":"in_progress","protocol_step_path":"root/rna-extraction/03-ethanol","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S1"],"attempt":1,"occurred_at":"2026-11-13T11:00:00Z"}
EOF
# Single completion: S1 completes a step alone; S2 stays not_started.
run execution_event.create --input - <<EOF > /dev/null
{"run_id":"$RUN","event_type":"operation","content":"S1 bind complete","step_status":"completed","protocol_step_path":"root/rna-extraction/04-bind","protocol_version_id":"$PV","assay_id":"assay-rtqpcr","sample_ids":["S1"],"attempt":1,"occurred_at":"2026-11-13T11:30:00Z","checks":[{"check_id":"c04-bind","status":"passed"}]}
EOF

{
  echo "== step 03-ethanol: S1 in_progress, S2 not_started =="
  run workflow.progress --input - <<EOF
{"run_id":"$RUN","step_path":"root/rna-extraction/03-ethanol"}
EOF
  echo
  echo "== step 04-bind: one completed sample does not complete the step =="
  run workflow.progress --input - <<EOF
{"run_id":"$RUN","step_path":"root/rna-extraction/04-bind"}
EOF
} > "$out/workflow-progress-partial.out"

echo "evidence written to $out"
