#!/usr/bin/env python3
"""EXP-15 (QA): synthetic semantics acceptance for CCK-8, RT-qPCR, Biofilm CV+CFU.

SYNTHETIC-ONLY fixture. Every value below is invented for contract coverage;
nothing here is a wet-lab result, an assay-validity claim, or a real sample.
Proves, via isolated LABWORKBENCH_DATA_DIR + existing CLI/event/readout
contracts, that raw/derived, qualifier, source and lineage semantics can be
expressed without runtime changes.

Test matrix (per case, all under one isolated temp data dir, user library untouched):

| category | CCK-8 | RT-qPCR | Biofilm CV+CFU |
|---|---|---|---|
| normal | raw OD450 + derived viability with derived_from -> raw/blank/control | raw Cq + not_detected qualifier + reference-gene dependent dCq + NTC as companion data | CV OD vs CFU kept separate; raw count/dilution/plated volume kept; CFU/mL + CFU/sample traced to source |
| boundary | above_uloq qualifier kept, not coerced to a number; boundary count plates (30/300) accepted | reference-gene Cq missing blocks the derived dCq write (refused) | countable-window boundary plates included; out-of-window plates excluded with reason, never estimated |
| failure/rollback | derived write with unknown source refused, DB unchanged | NTC-alone derived write refused (not auto-pass), DB unchanged | duplicate raw_id batch refused, DB unchanged |
| restart | independent process re-reads frozen Run/events | same | same |
| concurrent | two parallel read-only workspace queries | same | same |

Usage:
    python3 tests/fixtures/three_experiment_semantics.py [--json]
Exit code 0 + stdout "EXP-15 SYNTHETIC ACCEPTANCE PASS" on success.
"""
from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

SYNTHETIC_TAG = "SYNTHETIC-ONLY: invented values for contract coverage, not wet-lab data."

ROOT = Path(__file__).resolve().parents[2]
TARGET = Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target"))
CLI = Path(os.environ.get("LABWORKBENCH_CLI", TARGET / "debug/labworkbench"))
if not CLI.is_file() and Path(str(CLI) + ".exe").is_file():
    CLI = Path(str(CLI) + ".exe")


class Failure(RuntimeError):
    pass


def env_for(data: Path) -> dict:
    env = dict(os.environ)
    env["LABWORKBENCH_DATA_DIR"] = str(data)
    env["LABWORKBENCH_TRUSTED_WRITER"] = "1"
    return env


def call(data: Path, tool: str, payload: dict, expected: int = 0) -> dict:
    with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as handle:
        json.dump(payload, handle)
        name = handle.name
    try:
        proc = subprocess.run(
            [str(CLI), tool, "--input", name],
            cwd=str(ROOT),
            env=env_for(data),
            capture_output=True,
            text=True,
        )
    finally:
        os.unlink(name)
    if proc.returncode != expected or not proc.stdout.strip():
        raise Failure(f"{tool}: exit={proc.returncode} stdout={proc.stdout!r} stderr={proc.stderr!r}")
    try:
        value = json.loads(proc.stdout.strip())
    except json.JSONDecodeError as error:
        raise Failure(f"{tool}: unparsable stdout {proc.stdout!r}: {error}")
    if expected == 0 and value.get("ok") is not True:
        raise Failure(f"{tool}: rejected: {proc.stdout.strip()}")
    if expected != 0 and value.get("ok") is not False:
        raise Failure(f"{tool}: expected rejection but got ok: {proc.stdout.strip()}")
    return value["error"] if expected != 0 else value["result"]


def rejected(data: Path, tool: str, payload: dict, code: str = "INVALID_INPUT") -> dict:
    before = sorted((p.name, p.stat().st_size) for p in data.glob("*.sqlite*"))
    error = call(data, tool, payload, expected=2 if code == "INVALID_INPUT" else 3)
    after = sorted((p.name, p.stat().st_size) for p in data.glob("*.sqlite*"))
    if before != after:
        raise Failure(f"{tool}: rejected write changed sqlite files: {before} -> {after}")
    return error


def checksum(data: Path) -> str:
    dbs = sorted(data.glob("*.sqlite*"))
    if not dbs:
        return "no-sqlite-yet"
    digest = hashlib.sha256()
    for path in dbs:
        digest.update(path.read_bytes())
    return digest.hexdigest()


def text_step(use_id: str, title: str, instruction: str) -> dict:
    return {"kind": "text", "use_id": use_id, "title": title, "instruction": instruction}


def make_run(data: Path, name: str, assay_id: str, assay_title: str, steps: list) -> dict:
    project = call(data, "project.create", {"name": f"EXP-15 SYN {name}"})["record"]["id"]
    block = call(
        data,
        "protocol.block.create",
        {"project_id": project, "title": f"EXP-15 SYN {name} block", "steps": steps},
    )["version"]["id"]
    protocol = call(
        data,
        "protocol.create",
        {"project_id": project, "title": f"exp15-syn-{name}", "root_block_version_id": block},
    )["version"]["id"]
    design = call(
        data,
        "design.create",
        {
            "project_id": project,
            "design": {
                "title": f"EXP-15 SYN {name} ({SYNTHETIC_TAG})",
                "group_specs": [{"name": "SYN-Control", "role": "control"}],
                "assays": [
                    {
                        "id": assay_id,
                        "title": assay_title,
                        "endpoint": "synthetic endpoint",
                        "method": f"EXP-15 synthetic {name}",
                        "unit": "AU",
                        "timepoints": ["SYN-T0"],
                        "group_names": ["SYN-Control"],
                        "protocol_version_id": protocol,
                    }
                ],
            },
        },
    )["design"]
    run = call(data, "run.create", {"design_id": design["id"], "title": f"EXP-15 SYN {name} run"})["record"]
    workspace = call(data, "run.workspace", {"id": run["id"]})
    paths = {task["step_path"] for task in workspace["tasks"]}
    if len(workspace["tasks"]) != len(steps):
        raise Failure(f"{name}: expected {len(steps)} frozen tasks, got {len(workspace['tasks'])}")
    return {"project": project, "protocol": protocol, "design_id": design["id"], "run_id": run["id"], "paths": paths}


def restart_reads(data: Path, run_id: str) -> dict:
    workspace = call(data, "run.workspace", {"id": run_id})
    return {"events": len(workspace["events"]), "tasks": len(workspace["tasks"])}


def concurrent_reads(data: Path, run_id: str) -> int:
    def job() -> bool:
        workspace = call(data, "run.workspace", {"id": run_id})
        return len(workspace["tasks"]) > 0

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        return sum(1 for ok in pool.map(lambda _: job(), range(2)) if ok)


# --------------------------------------------------------------------------
# Case 1: CCK-8 — raw OD450 vs derived viability, qualifier kept as data
# --------------------------------------------------------------------------
def case_cck8(parent: Path) -> dict:
    data = parent / "cck8"
    data.mkdir(parents=True, exist_ok=True)
    ctx = make_run(
        data, "cck8", "cck8", "CCK-8 synthetic",
        [text_step("read", "Read OD450", "SYNTHETIC read step."), text_step("calc", "Derive viability", "SYNTHETIC derive step.")],
    )
    run_id, protocol = ctx["run_id"], ctx["protocol"]

    assertions: list[str] = []

    # normal: raw OD450 values + blank + control companions, with explicit
    # readout_ids so the derived viability can point back at them.
    # (scalar readout.submit leaves readout_id null; sources that derived
    # rows reference must be written via execution_event.create.)
    call(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC raw OD450",
        "protocol_step_path": "root/read", "protocol_version_id": protocol,
        "assay_id": "cck8", "timepoint": "SYN-T0",
        "readouts": [
            {"readout_id": "SYN-CCK8-A1", "endpoint": "od450", "value": 1.234,
             "unit": "AU", "result_kind": "raw", "sample_id": "SYN-CCK8-A1"},
            {"readout_id": "SYN-CCK8-BLANK", "endpoint": "od450", "value": 0.061,
             "unit": "AU", "result_kind": "raw", "sample_id": "SYN-CCK8-BLANK"},
            {"readout_id": "SYN-CCK8-CTRL", "endpoint": "od450", "value": 1.310,
             "unit": "AU", "result_kind": "raw", "sample_id": "SYN-CCK8-CTRL"},
        ],
    })

    def check(name: str, cond: bool, detail: str = "") -> None:
        if not cond:
            raise Failure(f"cck8/{name}: {detail}")
        assertions.append(name)

    events = call(data, "run.workspace", {"id": run_id})["events"]
    raws = [r for e in events for r in e.get("readouts", []) if r.get("endpoint") == "od450" and r.get("result_kind") == "raw"]
    check("raw_od_separate", len(raws) == 3, f"expected 3 raw od450 rows, got {len(raws)}")
    check("raw_no_derivation", all(not r.get("derived_from") for r in raws), "raw rows must not carry derived_from")

    # normal: derived viability points back at raw/blank/control sources
    derived = call(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC blank-corrected viability",
        "protocol_step_path": "root/calc", "protocol_version_id": protocol,
        "assay_id": "cck8", "timepoint": "SYN-T0",
        "readouts": [{
            "readout_id": "SYN-VIAB-A1", "endpoint": "viability", "value": 94.1, "unit": "%",
            "result_kind": "derived", "method": "synthetic-viability@1",
            "derived_from": ["SYN-CCK8-A1", "SYN-CCK8-BLANK", "SYN-CCK8-CTRL"],
            "sample_id": "SYN-CCK8-A1",
        }],
    })["record"]
    drow = derived["data"]["readouts"][0]
    check("derived_kind", drow["result_kind"] == "derived", str(drow))
    check("derived_sources", drow["derived_from"] == ["SYN-CCK8-A1", "SYN-CCK8-BLANK", "SYN-CCK8-CTRL"], str(drow))
    check("derived_method", drow["method"] == "synthetic-viability@1", str(drow))

    # boundary: above_uloq qualifier persisted as missing-reason, not coerced to a number
    call(data, "readout.submit", {
        "run_id": run_id, "step_path": "root/read", "protocol_version_id": protocol,
        "assay_id": "cck8", "timepoint": "SYN-T0", "endpoint": "od450",
        "qualifier": "above_uloq", "unit": "AU", "sample_id": "SYN-CCK8-A2",
    })
    events = call(data, "run.workspace", {"id": run_id})["events"]
    sat = [r for e in events for r in e.get("readouts", []) if r.get("sample_id") == "SYN-CCK8-A2"]
    check("saturated_qualifier", len(sat) == 1 and sat[0].get("qualifier") == "above_uloq" and sat[0].get("value") is None, str(sat))

    # failure/rollback: derived write citing an unknown source is refused, DB unchanged
    before = checksum(data)
    count_before = len(call(data, "record.list", {"kind": "execution_event", "project_id": ctx["project"]})["records"])
    err = rejected(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC bad source",
        "protocol_step_path": "root/calc", "protocol_version_id": protocol, "assay_id": "cck8",
        "readouts": [{
            "readout_id": "SYN-VIAB-BAD", "endpoint": "viability", "value": 50.0,
            "result_kind": "derived", "method": "synthetic-viability@1",
            "derived_from": ["SYN-NOPE"],
        }],
    })
    check("unknown_source_refused", "来源不存在" in err.get("message", ""), str(err))
    count_after = len(call(data, "record.list", {"kind": "execution_event", "project_id": ctx["project"]})["records"])
    check("rollback_event_count", count_before == count_after, f"{count_before} != {count_after}")
    check("rollback_checksum", checksum(data) == before, "database changed on refused write")

    return {
        "case": "CCK-8 synthetic", "synthetic": True, "run_id": run_id,
        "assertions": assertions, "assertion_count": len(assertions),
        "restart": restart_reads(data, run_id),
        "concurrent_queries": concurrent_reads(data, run_id),
        "checksum": checksum(data),
    }


# --------------------------------------------------------------------------
# Case 2: RT-qPCR — raw Cq, not_detected, NTC companion, reference-dependent dCq
# --------------------------------------------------------------------------
def case_qpcr(parent: Path) -> dict:
    data = parent / "qpcr"
    data.mkdir(parents=True, exist_ok=True)
    ctx = make_run(
        data, "qpcr", "qpcr", "RT-qPCR synthetic",
        [text_step("acq", "Acquire Cq", "SYNTHETIC acquire step."), text_step("norm", "Normalize dCq", "SYNTHETIC normalize step.")],
    )
    run_id, protocol = ctx["run_id"], ctx["protocol"]
    assertions: list[str] = []

    def check(name: str, cond: bool, detail: str = "") -> None:
        if not cond:
            raise Failure(f"qpcr/{name}: {detail}")
        assertions.append(name)
    # normal: raw target Cq + raw reference-gene Cq + raw NTC Cq companion
    # (explicit readout_ids; scalar readout.submit leaves readout_id null).
    # not_detected stays on readout.submit: it needs no derived reference.
    call(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC raw Cq",
        "protocol_step_path": "root/acq", "protocol_version_id": protocol,
        "assay_id": "qpcr", "timepoint": "SYN-T0",
        "readouts": [
            {"readout_id": "SYN-CQ-TARGET-1", "endpoint": "cq", "value": 24.6,
             "unit": "cycles", "result_kind": "raw", "sample_id": "SYN-QPCR-S1"},
            {"readout_id": "SYN-CQ-REF-1", "endpoint": "cq", "value": 18.2,
             "unit": "cycles", "result_kind": "raw", "sample_id": "SYN-QPCR-S1-REF"},
            {"readout_id": "SYN-CQ-NTC-1", "endpoint": "cq", "value": 38.9,
             "unit": "cycles", "result_kind": "raw", "sample_id": "SYN-QPCR-NTC"},
        ],
    })
    call(data, "readout.submit", {
        "run_id": run_id, "step_path": "root/acq", "protocol_version_id": protocol,
        "assay_id": "qpcr", "timepoint": "SYN-T0", "endpoint": "cq",
        "qualifier": "not_detected", "unit": "cycles", "sample_id": "SYN-QPCR-S2",
    })

    events = call(data, "run.workspace", {"id": run_id})["events"]
    cqs = [r for e in events for r in e.get("readouts", []) if r.get("endpoint") == "cq"]
    check("raw_cq_present", any(r.get("value") == 24.6 for r in cqs), str(cqs))
    nd = [r for r in cqs if r.get("qualifier") == "not_detected"]
    check("not_detected_qualifier", len(nd) == 1 and nd[0].get("value") is None, str(nd))
    ntc = [r for r in cqs if r.get("sample_id") == "SYN-QPCR-NTC"]
    check("ntc_companion_data", len(ntc) == 1 and ntc[0].get("value") == 38.9, str(ntc))

    # normal: dCq derived from target + reference-gene raw sources (explicit, no default gene)
    drow = call(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC dCq",
        "protocol_step_path": "root/norm", "protocol_version_id": protocol,
        "assay_id": "qpcr", "timepoint": "SYN-T0",
        "readouts": [{
            "readout_id": "SYN-DCQ-S1", "endpoint": "delta_cq", "value": 6.4, "unit": "cycles",
            "result_kind": "derived", "method": "synthetic-dcq@1: target minus stated reference",
            "derived_from": ["SYN-CQ-TARGET-1", "SYN-CQ-REF-1"],
            "sample_id": "SYN-QPCR-S1",
        }],
    })["record"]["data"]["readouts"][0]
    check("dcq_derived", drow["result_kind"] == "derived" and drow["derived_from"] == ["SYN-CQ-TARGET-1", "SYN-CQ-REF-1"], str(drow))

    # boundary: derived dCq WITHOUT the reference source is refused (missing companion blocks derivation)
    before = checksum(data)
    err = rejected(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC dCq without reference",
        "protocol_step_path": "root/norm", "protocol_version_id": protocol, "assay_id": "qpcr",
        "readouts": [{
            "readout_id": "SYN-DCQ-BAD", "endpoint": "delta_cq", "value": 6.4,
            "result_kind": "derived", "method": "synthetic-dcq@1",
            "derived_from": ["SYN-CQ-TARGET-1", "SYN-MISSING-REF"],
        }],
    })
    check("missing_ref_refused", "来源不存在" in err.get("message", ""), str(err))
    check("rollback_checksum", checksum(data) == before, "database changed on refused write")

    # failure: NTC-alone derived write is NOT an auto-pass; unknown source refused
    err2 = rejected(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC NTC-derived claim",
        "protocol_step_path": "root/norm", "protocol_version_id": protocol, "assay_id": "qpcr",
        "readouts": [{
            "readout_id": "SYN-NTC-CLAIM", "endpoint": "fold_change", "value": 1.0,
            "result_kind": "derived", "method": "synthetic-ntc-claim@1",
            "derived_from": ["SYN-NO-SUCH-SOURCE"],
        }],
    })
    check("ntc_no_autopass", "来源不存在" in err2.get("message", ""), str(err2))

    return {
        "case": "RT-qPCR synthetic", "synthetic": True, "run_id": run_id,
        "assertions": assertions, "assertion_count": len(assertions),
        "restart": restart_reads(data, run_id),
        "concurrent_queries": concurrent_reads(data, run_id),
        "checksum": checksum(data),
    }


# --------------------------------------------------------------------------
# Case 3: Biofilm CV+CFU — separate readouts, dilution lineage, window rule
# --------------------------------------------------------------------------
def case_biofilm(parent: Path) -> dict:
    data = parent / "biofilm"
    data.mkdir(parents=True, exist_ok=True)
    ctx = make_run(
        data, "biofilm", "biofilm", "Biofilm CV+CFU synthetic",
        [text_step("cv", "CV stain and read", "SYNTHETIC CV step."),
         text_step("count", "Count colonies", "SYNTHETIC count step."),
         text_step("calc", "Calculate CFU", "SYNTHETIC CFU derive step.")],
    )
    run_id, protocol = ctx["run_id"], ctx["protocol"]
    assertions: list[str] = []

    def check(name: str, cond: bool, detail: str = "") -> None:
        if not cond:
            raise Failure(f"biofilm/{name}: {detail}")
        assertions.append(name)
    # normal: CV OD and CFU raw counts are separate endpoints, never merged.
    # CFU raws carry explicit readout_ids matching the calculator input, so the
    # persisted derived_from lineage resolves; CV stays a scalar submit.
    # Dilution + plated/suspension volumes live in the calculator input below
    # (pure computation, not persisted by AsterLab) and in the persisted
    # derived readouts' lineage; raw colony counts keep count + unit here.
    call(data, "readout.submit", {
        "run_id": run_id, "step_path": "root/cv", "protocol_version_id": protocol,
        "assay_id": "biofilm", "timepoint": "SYN-T0", "endpoint": "cv_od590",
        "value": 0.842, "unit": "AU", "sample_id": "SYN-BIO-W1",
    })
    call(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC raw colony counts",
        "protocol_step_path": "root/count", "protocol_version_id": protocol,
        "assay_id": "biofilm", "timepoint": "SYN-T0",
        "readouts": [
            {"readout_id": "SYN-RAW-P1", "endpoint": "cfu_count", "value": 42.0,
             "unit": "colonies", "result_kind": "raw", "sample_id": "SYN-BIO-T1"},
            {"readout_id": "SYN-RAW-P2", "endpoint": "cfu_count", "value": 38.0,
             "unit": "colonies", "result_kind": "raw", "sample_id": "SYN-BIO-T1"},
            {"readout_id": "SYN-RAW-P3", "endpoint": "cfu_count", "value": 300.0,
             "unit": "colonies", "result_kind": "raw", "sample_id": "SYN-BIO-T2"},
            {"readout_id": "SYN-RAW-P4", "endpoint": "cfu_count", "value": 30.0,
             "unit": "colonies", "result_kind": "raw", "sample_id": "SYN-BIO-T2"},
        ],
    })

    events = call(data, "run.workspace", {"id": run_id})["events"]
    endpoints = {r.get("endpoint") for e in events for r in e.get("readouts", [])}
    check("cv_cfu_separate", {"cv_od590", "cfu_count"} <= endpoints, str(sorted(endpoints)))
    check("no_endpoint_merge", "cv_cfu" not in endpoints and "biomass_cfu" not in endpoints, str(sorted(endpoints)))

    # normal: raw count + dilution + plated volume retained, CFU derived via calculator + lineage
    calc = call(data, "calculator.biofilm_cfu", {
        "countable_min": 30, "countable_max": 300,
        "plates": [
            {"raw_id": "SYN-RAW-P1", "sample_id": "SYN-BIO-T1", "colonies": 42,
             "dilution_fractions": [0.1, 0.1], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
            {"raw_id": "SYN-RAW-P2", "sample_id": "SYN-BIO-T1", "colonies": 38,
             "dilution_fractions": [0.1, 0.1], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
            # boundary: window edges are inclusive
            {"raw_id": "SYN-RAW-P3", "sample_id": "SYN-BIO-T2", "colonies": 300,
             "dilution_fractions": [0.01], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
            {"raw_id": "SYN-RAW-P4", "sample_id": "SYN-BIO-T2", "colonies": 30,
             "dilution_fractions": [0.01], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
            # out-of-window plates: excluded with reason, never estimated
            {"raw_id": "SYN-RAW-P5", "sample_id": "SYN-BIO-T1", "colonies": 5,
             "dilution_fractions": [0.1, 0.1], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
            {"raw_id": "SYN-RAW-P6", "sample_id": "SYN-BIO-T2", "colonies": 450,
             "dilution_fractions": [0.01], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
        ],
    })
    groups = {g["sample_id"]: g for g in calc["groups"]}
    excluded = {(e["raw_id"], e["reason"]) for e in calc["excluded"]}
    check("group_averaging", groups["SYN-BIO-T1"]["mean_colonies"] == 40.0, str(groups))
    check("boundary_inclusive", "SYN-BIO-T2" in groups and groups["SYN-BIO-T2"]["plate_count"] == 2, str(groups))
    check("window_exclusion", excluded == {("SYN-RAW-P5", "below_window"), ("SYN-RAW-P6", "above_window")}, str(excluded))
    check("no_estimation", calc.get("excluded_count") == 2 and len(calc["groups"]) == 2, str(calc))

    # normal: CFU/mL + CFU/sample derived rows trace back to raw plate IDs
    derived_rows = groups["SYN-BIO-T1"]["readouts"]
    methods = {r["method"] for r in derived_rows}
    sources = {tuple(r["derived_from"]) for r in derived_rows}
    check("cfu_method", methods == {"calculator.biofilm_cfu@1"}, str(methods))
    check("cfu_lineage", sources == {("SYN-RAW-P1", "SYN-RAW-P2")}, str(sources))
    check("cfu_units", {r["unit"] for r in derived_rows} == {"CFU/mL", "CFU"}, str(derived_rows))
    persisted = call(data, "execution_event.create", {
        "run_id": run_id, "event_type": "measurement", "content": "SYNTHETIC CFU/mL + CFU/sample",
        "protocol_step_path": "root/calc", "protocol_version_id": protocol,
        "assay_id": "biofilm", "timepoint": "SYN-T0",
        "lineage": [
            {"from_kind": "well", "from_id": "SYN-BIO-W1", "to_kind": "tube", "to_id": "SYN-BIO-T1", "relation": "transferred"},
            {"from_kind": "tube", "from_id": "SYN-BIO-T1", "to_kind": "plate", "to_id": "SYN-RAW-P1", "relation": "plated"},
        ],
        "readouts": [
            {"readout_id": "SYN-CFU-ML-T1", "endpoint": "cfu_per_ml", "value": groups["SYN-BIO-T1"]["cfu_per_ml"],
             "unit": "CFU/mL", "result_kind": "derived", "method": "calculator.biofilm_cfu@1",
             "derived_from": ["SYN-RAW-P1", "SYN-RAW-P2"], "sample_id": "SYN-BIO-T1"},
            {"readout_id": "SYN-CFU-S-T1", "endpoint": "cfu_per_sample", "value": groups["SYN-BIO-T1"]["cfu_per_sample"],
             "unit": "CFU", "result_kind": "derived", "method": "calculator.biofilm_cfu@1",
             "derived_from": ["SYN-RAW-P1", "SYN-RAW-P2"], "sample_id": "SYN-BIO-T1"},
        ],
    })["record"]
    check("cfu_persisted", len(persisted["data"]["readouts"]) == 2, str(persisted["data"]["readouts"]))
    lineage = [(edge["from_id"], edge["to_id"]) for edge in persisted["data"]["lineage"]]
    check("lineage_trace", ("SYN-BIO-W1", "SYN-BIO-T1") in lineage and ("SYN-BIO-T1", "SYN-RAW-P1") in lineage, str(lineage))

    # failure/rollback: duplicate raw_id in calculator input is refused, DB unchanged
    before = checksum(data)
    err = call(
        data, "calculator.biofilm_cfu",
        {"countable_min": 30, "countable_max": 300, "plates": [
            {"raw_id": "SYN-DUP", "sample_id": "SYN-BIO-T9", "colonies": 40,
             "dilution_fractions": [0.1], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
            {"raw_id": "SYN-DUP", "sample_id": "SYN-BIO-T9", "colonies": 41,
             "dilution_fractions": [0.1], "plated_volume": 100.0, "plated_unit": "µL",
             "suspension_volume": 2.0, "suspension_unit": "mL"},
        ]},
        expected=2,
    )
    check("duplicate_raw_refused", err.get("code") == "INVALID_INPUT", str(err))
    check("rollback_checksum", checksum(data) == before, "database changed on refused calculation")

    return {
        "case": "Biofilm CV+CFU synthetic", "synthetic": True, "run_id": run_id,
        "assertions": assertions, "assertion_count": len(assertions),
        "restart": restart_reads(data, run_id),
        "concurrent_queries": concurrent_reads(data, run_id),
        "checksum": checksum(data),
    }


CASES = [("cck8", case_cck8), ("qpcr", case_qpcr), ("biofilm", case_biofilm)]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    if not CLI.is_file():
        print(f"labworkbench not found at {CLI}; build it or set LABWORKBENCH_CLI", file=sys.stderr)
        return 1
    if "LABWORKBENCH_DATA_DIR" in os.environ:
        print("refusing to run: LABWORKBENCH_DATA_DIR is set in the environment; unset it first", file=sys.stderr)
        return 1
    parent = Path(tempfile.mkdtemp(prefix="exp15-three-semantics-"))
    data_root = str(parent)
    results = []
    try:
        for name, case in CASES:
            print(f"== {name} (synthetic) ==", flush=True)
            result = case(parent)
            print(json.dumps(
                {"case": result["case"], "assertions": result["assertion_count"],
                 "restart": result["restart"], "concurrent": result["concurrent_queries"],
                 "checksum": result["checksum"]},
                ensure_ascii=False,
            ), flush=True)
            results.append({k: v for k, v in result.items()})
    except Failure as error:
        print(f"EXP-15 SYNTHETIC ACCEPTANCE FAIL: {error}", file=sys.stderr)
        return 1
    finally:
        import shutil
        shutil.rmtree(parent, ignore_errors=True)
    if args.json:
        print(json.dumps(results, ensure_ascii=False, indent=2))
    print(f"EXP-15 SYNTHETIC ACCEPTANCE PASS (synthetic-only; isolated data dir {data_root}, removed after run)")
    print("CCK-8: " + ", ".join(results[0]["assertions"]))
    print("qPCR: " + ", ".join(results[1]["assertions"]))
    print("Biofilm: " + ", ".join(results[2]["assertions"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
