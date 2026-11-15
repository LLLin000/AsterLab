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
| `timing` | `{ "kind": "hands_on" | "wait" | "timed_endpoint", "duration": Quantity, "minimum_duration"?: Quantity }` | Positive finite duration in `s`, `min`, `h`, or `d`. `hands_on` contributes to net hands-on time; other kinds contribute to elapsed span and a parallel window. `minimum_duration` is an optional lower bound for one occurrence, in a time unit and greater than zero; the projection charges `max(duration, minimum_duration)`, so it expresses `4°C ≥ 2h` without changing `duration`. |
| `repeat` | `{ "count": positive integer }` | Repeats the step consecutively. Missing means one occurrence. |
| `skip_condition` | `{ "check_id": non-empty string, "outcome": "passed" | "failed" }` | Declares the check outcome that conditionally skips the step. It is descriptive; this type does not evaluate checks. |
| `time_anchor_id` | non-empty string | Stable reference to a protocol/design time origin, such as `t0`. Required when `schedule` is present. |
| `schedule` | `{ "interval": Quantity, "occurrences": positive integer, "offset"?: Quantity }` | Periodic occurrences separated by a positive interval in a time unit. Uses `time_anchor_id` as its origin. `offset` is an optional delay from the anchor to the first occurrence, in a time unit and zero or greater; later occurrences still advance by `interval`, so `offset: 6h` with `interval: 6h` and `occurrences: 3` yields the anchor-relative starts `t0 + 6h`, `t0 + 12h`, `t0 + 18h`. A negative offset is `value_out_of_range`; an offset without `time_anchor_id` is `missing_parameter`. |

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

Each `wait` or `timed_endpoint` occurrence produces its own parallel window. Durations are converted to seconds for this projection only; the authored quantity and unit are unchanged. One occurrence costs `max(duration, minimum_duration)` when `minimum_duration` is present, so a declared `2 h` incubation with `minimum_duration: 2 h` charges `2 h` and a lower bound above `duration` wins. The projection sums known step durations in the supplied order. Repeat count multiplies each occurrence duration; a schedule `offset` delays only the first occurrence and therefore adds that delay to the span once before the occurrence windows. Periodic interval gaps add `(occurrences - 1) × interval` to the span and separate the corresponding windows. `hands_on_s` sums only `hands_on` durations. `wait` and `timed_endpoint` create parallel windows and do not count as hands-on. A missing timing contributes no duration, so an `offset` without `timing` adds nothing. Conditional skips are assumed not to occur, so estimates do not depend on runtime check outcomes. This is a deterministic estimate, not a runtime calendar or a claim that steps actually ran in parallel.

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
    "duration": { "value": 18, "unit": "min" },
    "minimum_duration": { "value": 20, "unit": "min" }
  },
  "repeat": { "count": 3 },
  "skip_condition": { "check_id": "no_bubbles", "outcome": "passed" },
  "time_anchor_id": "t0",
  "schedule": {
    "interval": { "value": 6, "unit": "h" },
    "occurrences": 3,
    "offset": { "value": 0, "unit": "min" }
  }
}
```

The time model records estimates and declared schedule structure only. It does not persist timers, execution facts, endpoint values, or frozen Run changes.

## Example: cold chain with anchor offsets

```json
{
  "schema_version": 2,
  "operation": {
    "kind": "incubate",
    "temperature": { "value": 4, "unit": "C" },
    "duration": { "value": 2, "unit": "h" }
  },
  "timing": {
    "kind": "wait",
    "duration": { "value": 2, "unit": "h" },
    "minimum_duration": { "value": 2, "unit": "h" }
  },
  "time_anchor_id": "t0",
  "schedule": {
    "interval": { "value": 6, "unit": "h" },
    "occurrences": 3,
    "offset": { "value": 6, "unit": "h" }
  }
}
```

The first occurrence starts at `t0 + 6h`, the second at `t0 + 12h`, and the third at `t0 + 18h`; the declared `interval` supplies the spacing, so a uniform `interval` cannot produce a non-uniform `t0 + 6h/12h/24h` series in one rule. Each occurrence costs `max(2h, 2h) = 2h`, giving three `2h` parallel windows.
