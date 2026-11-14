#!/usr/bin/env python3
"""Run isolated CCK-8, RT-qPCR, and non-plate CLI fixtures for QA-07."""
from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "docs/release/evidence/QA-07"
TARGET = Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target"))
CLI = Path(os.environ.get("LABWORKBENCH_CLI", TARGET / "debug/labworkbench"))
if not CLI.is_file() and Path(str(CLI) + ".exe").is_file():
    CLI = Path(str(CLI) + ".exe")

CASES = [
    {"name": "CCK-8", "assay": "cck8", "endpoint": "OD450", "unit": "AU", "sample": "CCK8-S1", "value": 0.735, "target": {"kind": "well", "plate_id": "plate-cck8", "coordinate": "A1"}},
    {"name": "RNA-RT-qPCR", "assay": "rtqpcr", "endpoint": "cq", "unit": "cycles", "sample": "RNA-S1", "value": 22.4, "target": {"kind": "unit", "unit_id": "RNA-S1"}},
    {"name": "non-plate", "assay": "nonplate", "endpoint": "concentration", "unit": "ng/uL", "sample": "NP-S1", "value": 18.2, "target": {"kind": "unit", "unit_id": "NP-S1"}},
]


def call(cli: Path, data: Path, tool: str, payload: dict, expected: int = 0) -> dict:
    env = {**os.environ, "LABWORKBENCH_DATA_DIR": str(data), "LABWORKBENCH_TRUSTED_WRITER": "1"}
    proc = subprocess.run([str(cli), tool, "--input", "-"], input=json.dumps(payload).encode(), capture_output=True, env=env)
    if proc.returncode != expected:
        raise RuntimeError(f"{tool}: expected exit {expected}, got {proc.returncode}; stdout={proc.stdout.decode(errors='replace')} stderr={proc.stderr.decode(errors='replace')}")
    value = json.loads(proc.stdout)
    if value.get("ok") != (expected == 0):
        raise RuntimeError(f"{tool}: unexpected response {value}")
    if expected:
        return value
    return value["result"]


def checksum(data: Path) -> str:
    dbs = sorted(p for p in data.rglob("*") if p.is_file() and p.suffix in {".db", ".sqlite", ".sqlite3"})
    if len(dbs) != 1:
        raise RuntimeError(f"expected exactly one SQLite database in {data}, found {[p.name for p in dbs]}")
    return hashlib.sha256(dbs[0].read_bytes()).hexdigest()


def run_case(case: dict, parent: Path) -> dict:
    data = parent / case["assay"]
    data.mkdir()
    project = call(CLI, data, "project.create", {"name": f"QA-07 {case['name']} isolated fixture"})["record"]
    block = call(CLI, data, "protocol.block.create", {
        "project_id": project["id"], "title": f"{case['name']} readout steps",
        "steps": [{"kind": "text", "use_id": "read", "title": "Record readout", "instruction": "Record the synthetic fixture observation."}],
    })["version"]["id"]
    protocol = call(CLI, data, "protocol.create", {"project_id": project["id"], "title": f"{case['name']} protocol", "root_block_version_id": block})["version"]["id"]
    design = call(CLI, data, "design.create", {"project_id": project["id"], "design": {
        "title": f"{case['name']} design", "group_specs": [{"name": "Synthetic", "condition": "fixture only"}],
        "assays": [{"id": case["assay"], "title": case["name"], "endpoint": case["endpoint"], "method": "synthetic fixture", "unit": case["unit"], "timepoints": ["T0"], "group_names": ["Synthetic"], "protocol_version_id": protocol}],
    }})["design"]
    run = call(CLI, data, "run.create", {"design_id": design["id"], "title": f"{case['name']} fixture run"})["record"]
    manifest = run["data"]["protocol_manifest"][0]
    if case["assay"] == "cck8":
        plate = call(CLI, data, "plate.create", {"run_id": run["id"], "assay_id": case["assay"], "timepoint": "T0", "name": "Synthetic CCK-8 plate", "format": "96", "rows": 8, "columns": 12})["record"]
        call(CLI, data, "well.set", {"plate_id": plate["id"], "well_id": "A1", "group_label": "Synthetic", "sample_id": case["sample"], "note": "QA-07 synthetic mapping"})
    event = call(CLI, data, "execution_event.create", {
        "run_id": run["id"], "event_type": "measurement", "content": "Synthetic QA-07 observation",
        "protocol_step_path": manifest["steps"][0]["path"], "protocol_version_id": protocol,
        "assay_id": case["assay"], "timepoint": "T0",
        "readouts": [{"readout_id": f"{case['assay']}-readout-1", "endpoint": case["endpoint"], "value": case["value"], "unit": case["unit"], "sample_id": case["sample"], "result_kind": "raw"}],
    })["record"]
    rejected = call(CLI, data, "execution_event.create", {
        "run_id": run["id"], "event_type": "measurement", "content": "invalid readout boundary",
        "protocol_step_path": manifest["steps"][0]["path"], "protocol_version_id": protocol,
        "assay_id": case["assay"], "readouts": [{"endpoint": case["endpoint"], "value": case["value"], "qualifier": "invalid"}],
    }, expected=2)
    if rejected["error"]["code"] != "INVALID_INPUT":
        raise RuntimeError(f"{case['name']}: wrong boundary failure {rejected}")
    before = checksum(data)
    def verify_read() -> tuple[dict, dict]:
        records = call(CLI, data, "record.list", {"kind": "execution_event", "project_id": project["id"]})["records"]
        fetched = call(CLI, data, "record.get", {"id": event["id"]})["record"]
        workspace = call(CLI, data, "run.workspace", {"id": run["id"]})
        assert any(item["id"] == event["id"] for item in records)
        assert fetched["data"]["readouts"][0]["value"] == case["value"]
        assert workspace["run_id"] == run["id"]
        assert workspace["events"] and workspace["steps"]
        return fetched, workspace
    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        reads = list(pool.map(lambda _: verify_read(), range(2)))
    after = checksum(data)
    if before != after:
        raise RuntimeError(f"{case['name']}: read-only restart/query modified database checksum")
    if len(reads) != 2:
        raise RuntimeError(f"{case['name']}: concurrent query count mismatch")
    return {"name": case["name"], "project_id": project["id"], "design_id": design["id"], "run_id": run["id"], "event_id": event["id"], "readout_id": event["data"]["readouts"][0]["readout_id"], "value": case["value"], "checksum": after, "recovery": "PASS", "boundary": "PASS", "concurrent_queries": 2}


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--all", action="store_true")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    if not args.all:
        parser.error("--all is required")
    if not CLI.is_file():
        raise RuntimeError(f"labworkbench CLI not found: {CLI}; build it first")
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="qa07-", dir=TARGET) as temp:
        results = [run_case(case, Path(temp)) for case in CASES]
    report = {"fixture": "QA-07", "results": results}
    if args.json:
        rendered = json.dumps(report, ensure_ascii=False, sort_keys=True, indent=2)
        print(rendered)
        (EVIDENCE / "results.json").write_text(rendered + "\n", encoding="utf-8")
    else:
        lines = [f"PASS {result['name']} checksum={result['checksum']} recovery=PASS boundary=PASS concurrent_queries=2" for result in results]
        lines.append("QA-07 PASS (3 fixture groups)")
        rendered = "\n".join(lines)
        print(rendered)
        (EVIDENCE / "run-output.txt").write_text("$ python3 tests/fixtures/run_qa07.py --all\n" + rendered + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (AssertionError, OSError, RuntimeError, subprocess.SubprocessError, json.JSONDecodeError) as exc:
        print(f"FAIL QA-07: {exc}", file=sys.stderr)
        sys.exit(1)
