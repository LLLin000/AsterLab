# OperationSpec Contract v2

- Status: additive Rust type and deterministic preview model; not an executor, timer, or Run persistence format.
- Types: [`operation_spec.rs`](../../crates/asterlab-core/src/operation_spec.rs), [`protocol_preview.rs`](../../crates/asterlab-core/src/protocol_preview.rs).
- v1 contract: [`operation-spec-v1.md`](operation-spec-v1.md).

## Compatibility and authority

`ProtocolBlockVersion` remains authoritative for protocol content and execution. `OperationSpec` is a typed sidecar; it does not create another execution path or rewrite block bindings.

The latest `schema_version` written is `2`. Readers accept schema versions 1 and 2. A v1 document with no extension fields deserializes with `timing`, `repeat`, `skip_condition`, `time_anchor_id`, and `schedule` set to `None`; absent options are omitted on serialization. v1 frozen Runs are not migrated or reinterpreted. Unknown fields and unsupported schema versions remain errors.

## Optional v2 fields

All fields below are optional and default to `None`.

| Field | Shape | Meaning and validation |
|---|---|---|
| `timing` | `{ "kind": "hands_on" | "wait" | "timed_endpoint", "duration": Quantity }` | Positive finite duration in `s`, `min`, `h`, or `d`. `hands_on` contributes to net hands-on time; other kinds contribute to elapsed span and a parallel window. |
| `repeat` | `{ "count": positive integer }` | Repeats the step consecutively. Missing means one occurrence. |
| `skip_condition` | `{ "check_id": non-empty string, "outcome": "passed" | "failed" }` | Declares the check outcome that conditionally skips the step. It is descriptive; this type does not evaluate checks. |
| `time_anchor_id` | non-empty string | Stable reference to a protocol/design time origin, such as `t0`. Required when `schedule` is present. |
| `schedule` | `{ "interval": Quantity, "occurrences": positive integer }` | Periodic occurrences separated by a positive interval in a time unit. Uses `time_anchor_id` as its origin. |

The existing closed semantic errors remain `unit_mismatch`, `missing_parameter`, and `value_out_of_range`. Validation reports deterministic paths under these fields. It does not infer missing durations, evaluate a condition, or convert scientific quantities.

## Workflow time projection

`protocol_preview::project_time_budget` accepts OperationSpecs in protocol order and returns:

```json
{
  "total_span_s": 180090.0,
  "hands_on_s": 60.0,
  "parallel_windows": [
    { "step_index": 1, "start_s": 60.0, "end_s": 7260.0,
      "duration_s": 7200.0, "time_anchor_id": "cold_start" },
    { "step_index": 2, "start_s": 7260.0, "end_s": 7270.0,
      "duration_s": 10.0, "time_anchor_id": "t0" },
    { "step_index": 2, "start_s": 93670.0, "end_s": 93680.0,
      "duration_s": 10.0, "time_anchor_id": "t0" },
    { "step_index": 2, "start_s": 180080.0, "end_s": 180090.0,
      "duration_s": 10.0, "time_anchor_id": "t0" }
  ]
}
```

Each `wait` or `timed_endpoint` occurrence produces its own parallel window. Durations are converted to seconds for this projection only; the authored quantity and unit are unchanged. The projection sums known step durations in the supplied order. Repeat count multiplies each occurrence duration; periodic interval gaps add `(occurrences - 1) × interval` to the span and separate the corresponding windows. `hands_on_s` sums only `hands_on` durations. `wait` and `timed_endpoint` create parallel windows and do not count as hands-on. A missing timing contributes no duration. Conditional skips are assumed not to occur, so estimates do not depend on runtime check outcomes. This is a deterministic estimate, not a runtime calendar or a claim that steps actually ran in parallel.

## Unit registry additions

The AsterDoc registry preserves all 36 existing tokens and adds:

| Token | Dimension | Factor | Boundary |
|---|---|---:|---|
| `%` | `fraction` | `0.01` | Percentage points map to a unit fraction; the concentration basis remains external. |
| `cells/mL` | `cell_concentration` | `1` | Count concentration; no conversion to another cell-count unit is implied. |
| `cells/well` | `cells_per_well` | `1` | Per-well cell count; distinct from concentration. |
| `mW/cm2` | `irradiance` | `10` | Factor is relative to W/m². |
| `nm` | `length` | `1e-9` | Already present in the v1 registry; retained unchanged. |

The Go validator embeds `spec/units.json`; it reads the same symbols, dimensions, and factors. Unit conversion remains explicit and is allowed only between units with the same registered dimension.

## Example

```json
{
  "schema_version": 2,
  "operation": {
    "kind": "measure",
    "endpoint_id": "tube_length",
    "unit": "count"
  },
  "timing": {
    "kind": "timed_endpoint",
    "duration": { "value": 18, "unit": "min" }
  },
  "repeat": { "count": 3 },
  "skip_condition": { "check_id": "no_bubbles", "outcome": "passed" },
  "time_anchor_id": "t0",
  "schedule": {
    "interval": { "value": 6, "unit": "h" },
    "occurrences": 3
  }
}
```

The time model records estimates and declared schedule structure only. It does not persist timers, execution facts, endpoint values, or frozen Run changes.
