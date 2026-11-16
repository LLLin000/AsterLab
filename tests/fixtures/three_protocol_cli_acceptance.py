#!/usr/bin/env python3
"""QA-09: isolated CLI acceptance for three real-world protocols.

Each case runs in its own throwaway data directory under the build tree and
talks to `labworkbench` only through the public JSON CLI, exactly like an
external agent. The user's main database is never opened: the script always
sets `LABWORKBENCH_DATA_DIR` to a fresh directory it created.

Cases
-----
huvec    official `docs/sdk/huvec-tube-formation-case/huvec-tube-formation.asterml`
         (3% / 5% / 7% GelMA, 12-well plate, 6h/12h/24h, 6 FOV per well):
         validate -> compile -> create_run, 7 frozen tasks, 9 wells x 3
         timepoints, 162 image links with well/timepoint/FOV coordinates.
rtqpcr   96-well cross plate + Nanodrop mass balancing + QC drop + retry +
         Ct CSV batch import.
invivo   in-vivo efficacy schedule anchored on the run date + blind view +
         `censored_dead` + 1-to-3 aliquot split + 1D Western blot lanes +
         non-scalar (categorical/table) readouts.

Every case also exercises the three orthogonal modes the QA card asks for:
`normal` (writes land and read back), `boundary` (invalid input is rejected)
and `rollback` (a rejected multi-row write leaves no partial record). A
separate-process restart check and a concurrent read-only query pair cover the
remaining QA matrix rows.

Usage:
    python3 tests/fixtures/three_protocol_cli_acceptance.py [--json]

Environment:
    LABWORKBENCH_CLI    labworkbench binary (default: <target>/debug/labworkbench)
    CARGO_TARGET_DIR    build directory that also hosts the scratch data dirs
"""
from __future__ import annotations

import argparse
import concurrent.futures
import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
EVIDENCE = ROOT / "docs/release/evidence/QA-09"
HUVEC_SOURCE = ROOT / "docs/sdk/huvec-tube-formation-case/huvec-tube-formation.asterml"
TARGET = Path(os.environ.get("CARGO_TARGET_DIR", ROOT / "target"))
CLI = Path(os.environ.get("LABWORKBENCH_CLI", TARGET / "debug/labworkbench"))
if not CLI.is_file() and Path(str(CLI) + ".exe").is_file():
    CLI = Path(str(CLI) + ".exe")


class Failure(RuntimeError):
    pass


# --------------------------------------------------------------------------
# CLI plumbing
# --------------------------------------------------------------------------
def env_for(data: Path) -> dict:
    return {
        **os.environ,
        "LABWORKBENCH_DATA_DIR": str(data),
        "LABWORKBENCH_TRUSTED_WRITER": "1",
    }


def call(data: Path, tool: str, payload: dict, expected: int = 0) -> dict:
    proc = subprocess.run(
        [str(CLI), tool, "--input", "-"],
        input=json.dumps(payload).encode(),
        capture_output=True,
        env=env_for(data),
    )
    out = proc.stdout.decode("utf-8", "replace").strip()
    try:
        value = json.loads(out)
    except json.JSONDecodeError:
        raise Failure(
            f"{tool}: non-JSON exit {proc.returncode} stdout={out[:400]} "
            f"stderr={proc.stderr.decode('utf-8', 'replace')[:400]}"
        ) from None
    if proc.returncode != expected:
        raise Failure(f"{tool}: expected exit {expected}, got {proc.returncode}: {value}")
    if value.get("ok") != (expected == 0):
        raise Failure(f"{tool}: unexpected response {value}")
    if expected:
        return value
    return value["result"]


def rejected(data: Path, tool: str, payload: dict, code: str = "INVALID_INPUT") -> dict:
    """Assert one write is refused with `code` and report the error payload."""
    value = call(data, tool, payload, expected=2)
    got = value["error"]["code"]
    if got != code:
        raise Failure(f"{tool}: expected error {code}, got {got}: {value['error']}")
    return value["error"]


def checksum(data: Path) -> str:
    dbs = sorted(
        p
        for p in data.rglob("*")
        if p.is_file() and p.suffix in {".db", ".sqlite", ".sqlite3"}
    )
    if len(dbs) != 1:
        raise Failure(f"expected exactly one SQLite database in {data.name}, found {[p.name for p in dbs]}")
    return hashlib.sha256(dbs[0].read_bytes()).hexdigest()


def record_count(data: Path, kind: str, project_id: str) -> int:
    return len(call(data, "record.list", {"kind": kind, "project_id": project_id})["records"])


def new_data(parent: Path, name: str) -> Path:
    data = parent / name
    data.mkdir()
    if data.resolve() == Path.home().resolve():
        raise Failure("refusing to run against the home directory")
    return data


def block(project: str, data: Path, title: str, steps: list, slots: list | None = None) -> str:
    return call(
        data,
        "protocol.block.create",
        {
            "project_id": project,
            "title": title,
            "summary": title,
            "parameter_slots": slots or [],
            "steps": steps,
        },
    )["version"]["id"]


def text_step(use_id: str, title: str, instruction: str, params: list | None = None) -> dict:
    step = {"kind": "text", "use_id": use_id, "title": title, "instruction": instruction}
    if params:
        step["parameter_refs"] = params
    return step


def restart_reads(data: Path, run_id: str) -> dict:
    """Re-read the frozen Run from a second, independent CLI process."""
    events = call(data, "record.list", {"kind": "execution_event"})["records"]
    workspace = call(data, "run.workspace", {"id": run_id})
    return {"events": len(events), "tasks": len(workspace["tasks"]), "steps": len(workspace["steps"])}


def concurrent_reads(data: Path, run_id: str, project_id: str) -> int:
    def job() -> bool:
        workspace = call(data, "run.workspace", {"id": run_id})
        records = call(data, "record.list", {"kind": "execution_event", "project_id": project_id})
        return bool(workspace["run_id"] == run_id and records["records"] is not None)

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
        return sum(1 for ok in pool.map(lambda _: job(), range(2)) if ok)


# --------------------------------------------------------------------------
# Case 1: HUVEC tube formation (official AsterML sample)
# --------------------------------------------------------------------------
HUVEC_STEPS = [
    ("step-01-sterilize", "01 GelMA 热冷循环消毒与脱泡"),
    ("step-02-coat-cure", "02 12 孔板铺胶与 405 nm 光固化"),
    ("step-03-soak", "03 完全培养基 4°C 浸泡平衡"),
    ("step-04-seed", "04 复温与接种 HUVEC"),
    ("step-05a-img-6h", "05a 6h 光镜观察与采集"),
    ("step-05b-img-12h", "05b 12h 光镜观察与采集"),
    ("step-05c-img-24h", "05c 24h 光镜观察与采集"),
]
HUVEC_WELLS = ["A1", "A2", "A3", "B1", "B2", "B3", "C1", "C2", "C3"]
HUVEC_TIMEPOINTS = ["6h", "12h", "24h"]
FOV_PER_WELL = 6


def bind_huvec(source: str, block_ids: dict, protocol_version_id: str) -> str:
    """Replace the document's ProtocolBlock placeholders with real versions."""
    def substitute(match: re.Match) -> str:
        key = match.group(1)
        if key not in block_ids:
            raise Failure(f"unbound ProtocolBlock placeholder {key}")
        return block_ids[key]

    bound = re.sub(r"\{\{protocol_block_version:([^}]+)\}\}", substitute, source)
    return bound.replace("{{protocol_version:huvec-tube-formation}}", protocol_version_id)


def case_huvec(parent: Path) -> dict:
    data = new_data(parent, "huvec")
    project = call(data, "project.create", {"name": "QA-09 HUVEC 成管隔离回归"})["record"]["id"]
    block_ids = {}
    for step_id, title in HUVEC_STEPS:
        block_ids[step_id] = block(
            project,
            data,
            title,
            [text_step("run", title, "按方案执行并记录实际值。")],
        )
    root = block(
        project,
        data,
        "HUVEC 成管协议根块",
        [
            {
                "kind": "block_use",
                "use_id": step_id,
                "block_version_id": block_ids[step_id],
                "bindings": [],
            }
            for step_id, _ in HUVEC_STEPS
        ],
    )
    protocol = call(
        data,
        "protocol.create",
        {"project_id": project, "title": "huvec-tube-formation", "root_block_version_id": root},
    )["version"]["id"]

    source = HUVEC_SOURCE.read_text(encoding="utf-8")
    validated = call(data, "asterdoc.validate", {"source": source})
    if not (validated["valid"] and validated["compiled"]):
        raise Failure(f"huvec: unbound sample failed validate: {validated}")
    step_blocks = validated["design"]["mygo"]["stepBlocks"]
    if len(step_blocks) != len(HUVEC_STEPS):
        raise Failure(f"huvec: sample materialises {len(step_blocks)} steps, expected {len(HUVEC_STEPS)}")

    bound = bind_huvec(source, block_ids, protocol)
    design = call(data, "design.create", {"project_id": project, "design": {"title": "HUVEC 成管"}})["design"]
    document = call(data, "asterdoc.save", {"design_id": design["id"], "source": bound, "expected_revision": 0})
    revision = document["revision"]

    # regression: the retired enum values must still be rejected by the compiler
    retired = bound.replace(
        "timepoint_relation: same_experimental_unit_repeated",
        "timepoint_relation: longitudinal_same_wells_anchored_at_seeding",
    ).replace("planned_run_relationship: same_source", "planned_run_relationship: single_longitudinal_plate")
    if call(data, "asterdoc.validate", {"source": retired})["valid"]:
        raise Failure("huvec: retired enum values are still accepted")
    if call(data, "asterdoc.validate", {"source": bound})["valid"] is not True:
        raise Failure("huvec: bound sample no longer validates")

    # rollback: a structurally invalid revision is refused and nothing is appended
    broken = bound.replace("id: step-03-soak", "id: step-02-coat-cure")
    rejected(data, "asterdoc.save", {"design_id": design["id"], "source": broken, "expected_revision": revision})
    if call(data, "asterdoc.load", {"design_id": design["id"]})["revision"] != revision:
        raise Failure("huvec: rejected AsterDoc save still appended a revision")

    compiled = call(data, "asterdoc.compile", {"design_id": design["id"], "revision": revision})
    assay = compiled["design"]["assays"][0]
    if assay["protocol_version_id"] != protocol:
        raise Failure(f"huvec: compile lost bound protocol: {assay['protocol_version_id']}")
    if assay["timepoint_relation"] != "same_experimental_unit_repeated":
        raise Failure(f"huvec: timepoint_relation not normalized: {assay['timepoint_relation']}")
    if compiled["design"]["planned_run_relationship"] != "same_source":
        raise Failure("huvec: planned_run_relationship not normalized")

    run = call(data, "asterdoc.create_run", {"design_id": design["id"], "revision": revision, "title": "HUVEC 6h/12h/24h run"})
    workspace = call(data, "run.workspace", {"id": run["id"]})
    if len(workspace["tasks"]) != len(HUVEC_STEPS):
        raise Failure(f"huvec: expected {len(HUVEC_STEPS)} frozen tasks, got {len(workspace['tasks'])}")
    planned_wells = workspace["frozen_design"]["mygo"]["assays"]["assay-tube-formation"]["wells"]
    expected_wells = len(HUVEC_WELLS) * len(HUVEC_TIMEPOINTS)
    if len(planned_wells) != expected_wells:
        raise Failure(f"huvec: expected {expected_wells} planned well/timepoint slots, got {len(planned_wells)}")
    if {well["timepoint"] for well in planned_wells} != set(HUVEC_TIMEPOINTS):
        raise Failure("huvec: planned wells do not cover 6h/12h/24h")

    # normal: 162 image links, each carrying well + timepoint + FOV coordinates
    images = data / "images"
    images.mkdir()
    step_path = workspace["tasks"][0]["step_path"]
    task_key = workspace["tasks"][0]["task_key"]
    for well in HUVEC_WELLS:
        for timepoint in HUVEC_TIMEPOINTS:
            for fov in range(1, FOV_PER_WELL + 1):
                name = f"{well}-{timepoint}-fov{fov}.png"
                (images / name).write_bytes(b"\x89PNG\r\n\x1a\n" + name.encode())
                call(
                    data,
                    "asset.link",
                    {
                        "root_id": "qa09-huvec-images",
                        "root_dir": str(images),
                        "relative_path": name,
                        "run_id": run["id"],
                        "task_key": task_key,
                        "well_id": well,
                        "timepoint": timepoint,
                        "fov_index": fov,
                    },
                )
    links = call(data, "asset.list", {"run_id": run["id"]})["records"]
    if len(links) != len(HUVEC_WELLS) * len(HUVEC_TIMEPOINTS) * FOV_PER_WELL:
        raise Failure(f"huvec: expected 162 image links, got {len(links)}")
    coverage = call(data, "run.workspace", {"id": run["id"]})["asset_coverage"]
    if len(coverage) != expected_wells:
        raise Failure(f"huvec: expected {expected_wells} asset coverage cells, got {len(coverage)}")
    if any(len(cell["asset_ids"]) != FOV_PER_WELL for cell in coverage):
        raise Failure("huvec: asset coverage does not hold 6 FOV per well/timepoint")

    call(
        data,
        "readout.submit",
        {
            "run_id": run["id"],
            "step_path": workspace["tasks"][4]["step_path"],
            "protocol_version_id": protocol,
            "endpoint": "per_well_fov_count",
            "value": FOV_PER_WELL,
            "unit": "images",
            "assay_id": "assay-tube-formation",
            "sample_id": "HUVEC-3%-1",
            "timepoint": "6h",
        },
    )

    # boundary: FOV index and orphan coordinates are refused before any write
    before = checksum(data)
    rejected(
        data,
        "asset.link",
        {
            "root_id": "qa09-huvec-images",
            "root_dir": str(images),
            "relative_path": "A1-6h-fov1.png",
            "run_id": run["id"],
            "task_key": task_key,
            "well_id": "A1",
            "timepoint": "6h",
            "fov_index": 0,
        },
    )
    rejected(
        data,
        "asset.link",
        {
            "root_id": "qa09-huvec-images",
            "root_dir": str(images),
            "relative_path": "A1-6h-fov1.png",
            "run_id": run["id"],
            "task_key": task_key,
            "timepoint": "6h",
        },
    )
    if checksum(data) != before:
        raise Failure("huvec: rejected asset links still wrote to the database")

    return {
        "case": "HUVEC tube formation",
        "project_id": project,
        "design_id": design["id"],
        "asterdoc_revision": revision,
        "protocol_version_id": protocol,
        "run_id": run["id"],
        "frozen_tasks": len(workspace["tasks"]),
        "planned_well_timepoints": len(planned_wells),
        "image_links": len(links),
        "asset_coverage_cells": len(coverage),
        "rollback": "PASS",
        "boundary": "PASS",
        "restart": restart_reads(data, run["id"]),
        "concurrent_queries": concurrent_reads(data, run["id"], project),
        "checksum": checksum(data),
        "data_dir": data,
    }


# --------------------------------------------------------------------------
# Case 2: RT-qPCR (96-well cross plate, Nanodrop balancing, QC drop, Ct CSV)
# --------------------------------------------------------------------------
def case_rtqpcr(parent: Path) -> dict:
    data = new_data(parent, "rtqpcr")
    project = call(data, "project.create", {"name": "QA-09 RT-qPCR 隔离回归"})["record"]["id"]
    qc_block = block(
        project,
        data,
        "RNA 质检与逆转录",
        [
            text_step("qc", "RNA 质检", "记录 Nanodrop 浓度与 A260/A280。"),
            text_step("rt", "逆转录", "按质量配平后逆转录。"),
            text_step("qpcr", "qPCR 扩增", "上机并导出 Ct CSV。"),
        ],
    )
    protocol = call(
        data,
        "protocol.create",
        {"project_id": project, "title": "rtqpcr-Ct", "root_block_version_id": qc_block},
    )["version"]["id"]
    samples = [("S1", 210.0), ("S2", 145.0), ("S3", 3.5)]
    design = call(
        data,
        "design.create",
        {
            "project_id": project,
            "design": {
                "title": "RT-qPCR 交叉板",
                "experimental_unit": "独立 RNA 样本",
                "group_specs": [
                    {"name": "S1", "role": "treatment", "independent_n": 1},
                    {"name": "S2", "role": "treatment", "independent_n": 1},
                    {"name": "S3", "role": "treatment", "independent_n": 1},
                    {"name": "NTC", "role": "control", "independent_n": 1},
                ],
                "timepoints": ["T0"],
                "assays": [
                    {
                        "id": "rtqpcr",
                        "title": "RT-qPCR",
                        "endpoint": "Ct",
                        "method": "SYBR green, 96-well cross plate",
                        "unit": "cycles",
                        "timepoints": ["T0"],
                        "group_names": ["S1", "S2", "S3", "NTC"],
                        "protocol_version_id": protocol,
                        "timepoint_relation": "same_physical_sample_repeated",
                    }
                ],
            },
        },
    )["design"]
    run = call(data, "run.create", {"design_id": design["id"], "title": "RT-qPCR run"})["record"]
    workspace = call(data, "run.workspace", {"id": run["id"]})
    step_path = workspace["tasks"][0]["step_path"]

    # Nanodrop mass balancing: S3 cannot supply the target mass at the allowed volume
    balance = call(
        data,
        "calculator.sample_normalize_batch",
        {
            "target_mass_ng": 100.0,
            "reaction_volume_ul": 20.0,
            "samples": [
                {"name": name, "concentration": conc, "unit": "ng/µL"} for name, conc in samples
            ],
        },
    )
    if balance["insufficient_count"] != 1:
        raise Failure(f"rtqpcr: expected exactly one insufficient sample, got {balance['insufficient_count']}")
    if not balance["rows"][2]["insufficient"]:
        raise Failure("rtqpcr: S3 (3.5 ng/µL) should be flagged insufficient")

    plate = call(
        data,
        "plate.create",
        {
            "run_id": run["id"],
            "assay_id": "rtqpcr",
            "timepoint": "T0",
            "name": "qPCR cross plate",
            "format": "96",
            "rows": 8,
            "columns": 12,
        },
    )["record"]
    s3_qc = None
    for row, (name, _) in enumerate(samples + [("NTC", 0.0)]):
        call(
            data,
            "well.set",
            {
                "plate_id": plate["id"],
                "well_id": f"{chr(ord('A') + row)}1",
                "group_label": name,
                "sample_id": name,
            },
        )
        event = call(
            data,
            "execution_event.create",
            {
                "run_id": run["id"],
                "event_type": "observation",
                "content": f"{name} Nanodrop 浓度 {(dict(samples).get(name) or 0)} ng/µL",
                "protocol_step_path": step_path,
                "protocol_version_id": protocol,
                "assay_id": "rtqpcr",
                "timepoint": "T0",
                "sample_ids": [name],
                "attempt": 1,
                "step_status": "qc_dropped" if name == "S3" else "completed",
            },
        )
        if name == "S3":
            s3_qc = event["record"]["id"]

    # retry: S2 attempt 1 is blocked, attempt 2 completes with lineage back to it
    blocked = call(
        data,
        "execution_event.create",
        {
            "run_id": run["id"],
            "event_type": "deviation",
            "content": "S2 逆转录中断",
            "deviation": "heat block 断电 3 min",
            "protocol_step_path": step_path,
            "protocol_version_id": protocol,
            "assay_id": "rtqpcr",
            "timepoint": "T0",
            "sample_ids": ["S2"],
            "attempt": 1,
            "step_status": "blocked",
        },
    )["record"]["id"]
    retried = call(
        data,
        "execution_event.create",
        {
            "run_id": run["id"],
            "event_type": "operation",
            "content": "S2 重做逆转录",
            "protocol_step_path": step_path,
            "protocol_version_id": protocol,
            "assay_id": "rtqpcr",
            "timepoint": "T0",
            "sample_ids": ["S2"],
            "attempt": 2,
            "step_status": "completed",
            "lineage": [
                {
                    "from_kind": "execution_event",
                    "from_id": blocked,
                    "to_kind": "step",
                    "to_id": step_path,
                    "relation": "retry",
                }
            ],
        },
    )["record"]
    progress = call(data, "workflow.progress", {"run_id": run["id"], "step_path": step_path})
    step = progress["steps"][0]
    s2 = next(row for row in step["samples"] if row["sample_id"] == "S2")
    if s2["attempt"] != 2 or s2["attempt_count"] != 2:
        raise Failure(f"rtqpcr: retry attempts not projected: attempt={s2['attempt']} count={s2['attempt_count']}")
    if s2["status"] != "completed":
        raise Failure(f"rtqpcr: S2 retry did not reach completed: {s2['status']}")
    s3 = next(row for row in step["samples"] if row["sample_id"] == "S3")
    if s3["status"] != "qc_dropped":
        raise Failure(f"rtqpcr: S3 not excluded as QC dropped: {s3['status']}")
    if step["qc_dropped_samples"] != 1:
        raise Failure(f"rtqpcr: qc_dropped_samples={step['qc_dropped_samples']}")

    # normal: Ct values imported from the instrument CSV export
    csv = "row,column,field,value,unit\n"
    for row, (name, _) in enumerate(samples):
        csv += f"{row},1,cq,{20.5 + row * 1.5},cycles\n"
    csv += "3,1,cq,Undetermined,cycles\n"
    batch = call(
        data,
        "readout.batch",
        {
            "run_id": run["id"],
            "step_path": step_path,
            "protocol_version_id": protocol,
            "plate_id": plate["id"],
            "schema_id": "rtqpcr.plate96.cq.v1",
            "assay_id": "rtqpcr",
            "timepoint": "T0",
            "batch_id": "ct-csv-1",
            "csv": csv,
        },
    )
    if batch["accepted"] != 4:
        raise Failure(f"rtqpcr: expected 4 accepted Ct rows, got {batch['accepted']}")
    cells = batch["record"]["data"]["result_cells"]
    if cells[3]["qualifier"] != "not_detected":
        raise Failure(f"rtqpcr: Undetermined Ct not normalized: {cells[3]}")

    # boundary + rollback: a mixed batch is refused and writes nothing
    before = checksum(data)
    events_before = record_count(data, "execution_event", project)
    rejected(
        data,
        "readout.batch",
        {
            "run_id": run["id"],
            "step_path": step_path,
            "protocol_version_id": protocol,
            "plate_id": plate["id"],
            "schema_id": "rtqpcr.plate96.cq.v1",
            "batch_id": "ct-csv-conflict",
            "cells": [{"row": 0, "column": 2, "field": "cq", "value": 21.0, "unit": "cycles"}],
            "csv": "row,column,field,value\n1,2,cq,22.0\n",
        },
    )
    rejected(
        data,
        "readout.batch",
        {
            "run_id": run["id"],
            "step_path": step_path,
            "protocol_version_id": protocol,
            "plate_id": plate["id"],
            "schema_id": "rtqpcr.plate96.cq.v1",
            "batch_id": "ct-csv-bad-unit",
            "csv": "row,column,field,value\n0,2,cq,21.0\n",
        },
    )
    if record_count(data, "execution_event", project) != events_before:
        raise Failure("rtqpcr: rejected Ct batches appended an event")
    if checksum(data) != before:
        raise Failure("rtqpcr: rejected Ct batches changed the database")

    return {
        "case": "RT-qPCR cross plate",
        "project_id": project,
        "design_id": design["id"],
        "protocol_version_id": protocol,
        "run_id": run["id"],
        "plate_id": plate["id"],
        "normalization_insufficient": balance["insufficient_count"],
        "qc_dropped_event": s3_qc,
        "retry_event": retried["id"],
        "ct_rows": batch["accepted"],
        "rollback": "PASS",
        "boundary": "PASS",
        "restart": restart_reads(data, run["id"]),
        "concurrent_queries": concurrent_reads(data, run["id"], project),
        "checksum": checksum(data),
        "data_dir": data,
    }


# --------------------------------------------------------------------------
# Case 3: in-vivo efficacy with blinded assessment and downstream WB/BCA-ELISA
# --------------------------------------------------------------------------
def case_invivo(parent: Path) -> dict:
    data = new_data(parent, "invivo")
    project = call(data, "project.create", {"name": "QA-09 体内药效隔离回归"})["record"]["id"]
    dose_block = block(
        project,
        data,
        "给药与称重",
        [
            {
                "kind": "text",
                "use_id": "dose",
                "title": "按体重给药",
                "instruction": "按实测体重换算给药体积；D0/D3/D6/D10/D14 各执行一次。",
                "time_scope": "per_timepoint",
            }
        ],
    )
    sampling_block = block(
        project,
        data,
        "取材与分样",
        [text_step("sample", "安乐死与取材", "按盲法编号取材，1 分为 3 份。")],
    )
    wb_block = block(
        project,
        data,
        "WB 与 BCA",
        [text_step("wb", "蛋白定量与 1D 泳道", "BCA 定量后上样，记录泳道分级。")],
    )
    sample_protocol = block(
        project,
        data,
        "取材分样协议根",
        [
            {"kind": "block_use", "use_id": "sample", "block_version_id": sampling_block, "bindings": []},
        ],
    )
    dose_protocol = call(
        data,
        "protocol.create",
        {"project_id": project, "title": "invivo-efficacy-dosing", "root_block_version_id": dose_block},
    )["version"]["id"]
    sample_protocol_version = call(
        data,
        "protocol.create",
        {"project_id": project, "title": "invivo-sampling", "root_block_version_id": sample_protocol},
    )["version"]["id"]
    wb_protocol = call(
        data,
        "protocol.create",
        {"project_id": project, "title": "invivo-wb", "root_block_version_id": wb_block},
    )["version"]["id"]
    timing = [
        {"timepoint": "D0", "offset_value": 0.0, "unit": "day"},
        {"timepoint": "D3", "offset_value": 3.0, "unit": "day", "window_before": 0.5, "window_after": 0.5},
        {"timepoint": "D6", "offset_value": 6.0, "unit": "day", "window_before": 0.5, "window_after": 0.5},
        {"timepoint": "D10", "offset_value": 10.0, "unit": "day", "window_before": 0.5, "window_after": 0.5},
        {"timepoint": "D14", "offset_value": 14.0, "unit": "day", "window_before": 0.5, "window_after": 0.5},
    ]
    groups = [
        {"name": "Vehicle", "role": "control", "independent_n": 6},
        {"name": "Low", "role": "treatment", "independent_n": 6},
        {"name": "High", "role": "treatment", "independent_n": 6},
    ]
    design = call(
        data,
        "design.create",
        {
            "project_id": project,
            "design": {
                "title": "A549 移植瘤体内药效",
                "experimental_unit": "一只动物",
                "blinding": "给药与测量由盲法编号执行，分析前不解盲",
                "group_specs": groups,
                "timepoints": ["D0", "D3", "D6", "D10", "D14"],
                "assays": [
                    {
                        "id": "efficacy",
                        "title": "肿瘤体积",
                        "endpoint": "肿瘤体积",
                        "method": "游标卡尺",
                        "unit": "mm3",
                        "timepoints": ["D0", "D3", "D6", "D10", "D14"],
                        "timepoint_timing": timing,
                        "time_anchor_event": "treatment_start",
                        "group_names": [g["name"] for g in groups],
                        "protocol_version_id": dose_protocol,
                    },
                    {
                        "id": "wb",
                        "title": "下游 WB",
                        "endpoint": "条带分级",
                        "method": "1D 泳道",
                        "unit": "categorical",
                        "timepoints": ["D14"],
                        "group_names": [g["name"] for g in groups],
                        "protocol_version_id": sample_protocol_version,
                    },
                    {
                        "id": "bca_elisa",
                        "title": "BCA/ELISA",
                        "endpoint": "蛋白浓度",
                        "method": "BCA 定量 + ELISA",
                        "unit": "ug/mL",
                        "timepoints": ["D14"],
                        "group_names": [g["name"] for g in groups],
                        "protocol_version_id": wb_protocol,
                    },
                ],
            },
        },
    )["design"]
    run = call(
        data,
        "run.create",
        {"design_id": design["id"], "title": "A549 efficacy run", "scheduled_date": "2026-11-20"},
    )["record"]
    workspace = call(data, "run.workspace", {"id": run["id"]})
    efficacy_steps = [task for task in workspace["tasks"] if task["scope"]["assay_id"] == "efficacy"]
    if len(efficacy_steps) != 5:
        raise Failure(f"invivo: expected 5 efficacy tasks, got {len(efficacy_steps)}")
    scheduled = {task["timepoint"]: task["schedule"] for task in efficacy_steps}
    if scheduled["D0"] is None or scheduled["D0"]["anchor_date"] != "2026-11-20":
        raise Failure(f"invivo: D0 has no anchored schedule: {scheduled['D0']}")
    if scheduled["D3"]["window_start"] != "2026-11-22T12:00:00Z":
        raise Failure(f"invivo: D3 window wrong: {scheduled['D3']}")
    if scheduled["D14"]["window_end"] != "2026-12-04T12:00:00Z":
        raise Failure(f"invivo: D14 window wrong: {scheduled['D14']}")

    dose_path = next(task["step_path"] for task in workspace["tasks"] if task["scope"]["assay_id"] == "efficacy")
    wb_path = next(task["step_path"] for task in workspace["tasks"] if task["scope"]["assay_id"] == "wb")  # root/sample/sample
    bca_path = next(task["step_path"] for task in workspace["tasks"] if task["scope"]["assay_id"] == "bca_elisa")

    # blinding: the blind projection hides group identity but keeps the facts
    blind = call(data, "run.workspace", {"id": run["id"], "view": "blind"})
    if any(group["name"] for group in blind["frozen_design"]["content"]["groups"]):
        raise Failure("invivo: blind view leaked group names")
    rejected(data, "run.workspace", {"id": run["id"], "view": "masked"})

    # periodic dosing: one blind dosing event per scheduled timepoint (D0/D3/D6/D10)
    animals = [f"m-{index:02d}" for index in range(1, 7)]
    for index, timepoint in enumerate(["D0", "D3", "D6", "D10"]):
        call(
            data,
            "execution_event.create",
            {
                "run_id": run["id"],
                "event_type": "operation",
                "content": f"{timepoint} 盲法给药",
                "protocol_step_path": dose_path,
                "protocol_version_id": dose_protocol,
                "assay_id": "efficacy",
                "timepoint": timepoint,
                "sample_ids": animals,
                "blind_masked": True,
                "step_status": "completed",
                "attempt": 1,
            },
        )

    # censored_dead: one animal is lost mid-study and must not count as complete
    call(
        data,
        "execution_event.create",
        {
            "run_id": run["id"],
            "event_type": "deviation",
            "content": "m-04 死亡，记录删失",
            "deviation": "麻醉后死亡",
            "protocol_step_path": dose_path,
                "protocol_version_id": dose_protocol,
            "assay_id": "efficacy",
            "timepoint": "D10",
            "sample_ids": ["m-04"],
            "step_status": "censored_dead",
            "attempt": 2,
        },
    )

    # 1-to-3 split: one necropsy sample feeds WB + BCA + ELISA
    split = call(
        data,
        "execution_event.create",
        {
            "run_id": run["id"],
            "event_type": "material_use",
            "content": "m-01 取材后 1 分为 3",
            "protocol_step_path": wb_path,
                "protocol_version_id": sample_protocol_version,
            "assay_id": "wb",
            "timepoint": "D14",
            "sample_ids": ["m-01-a", "m-01-b", "m-01-c"],
            "parent_sample_id": "m-01",
            "aliquot_from": "m-01 tumor lysate",
            "materials": [
                {"material": "m-01 tumor lysate"},
                {"material": "m-01-a aliquot", "planned_output_id": None},
                {"material": "m-01-b aliquot"},
                {"material": "m-01-c aliquot"},
            ],
        },
    )["record"]
    if split["data"]["parent_sample_id"] != "m-01":
        raise Failure(f"invivo: aliquot lineage lost: {split['data']}")
    if len(split["data"]["sample_ids"]) != 3:
        raise Failure("invivo: expected a 1-to-3 aliquot split")

    # boundary: a retry that skips an attempt number is refused
    rejected(
        data,
        "execution_event.create",
        {
            "run_id": run["id"],
            "event_type": "operation",
            "content": "m-02 跳号重试",
            "protocol_step_path": dose_path,
                "protocol_version_id": dose_protocol,
            "assay_id": "efficacy",
            "timepoint": "D10",
            "sample_ids": ["m-02"],
            "step_status": "completed",
            "attempt": 4,
        },
    )

    # 1D lanes: WB membrane strips, lanes named explicitly
    lanes = call(
        data,
        "plate.create",
        {
            "run_id": run["id"],
            "assay_id": "wb",
            "timepoint": "D14",
            "name": "WB 1D 泳道",
            "format": "custom",
            "rows": 1,
            "columns": 12,
        },
    )["record"]
    for lane in range(1, 4):
        call(
            data,
            "well.set",
            {
                "plate_id": lanes["id"],
                "well_id": f"Lane{lane}",
                "group_label": f"lane-{lane}",
                "sample_id": f"m-0{lane}-a",
                "target": "GAPDH",
                "dilution_factor": 10,
            },
        )
    rejected(
        data,
        "well.set",
        {"plate_id": lanes["id"], "well_id": "Tube13"},
    )

    # non-scalar readouts: categorical band grade + a lane/grad table
    typed = call(
        data,
        "readout.submit",
        {
            "run_id": run["id"],
            "step_path": wb_path,
                "protocol_version_id": sample_protocol_version,
            "assay_id": "wb",
            "timepoint": "D14",
            "readout_schemas": [
                {
                    "schema_id": "wb_grade",
                    "schema_version": 1,
                    "endpoint": "wb_grade",
                    "value_kind": "categorical",
                    "unit": {"canonical": None, "required": False},
                    "missing": {"allowed": [], "null_allowed": False},
                    "source": {"raw": True, "derived": False},
                    "link": {"timepoint_required": False, "sample_required": False},
                    "categorical": {"options": ["positive", "negative"], "allow_other": False},
                },
                {
                    "schema_id": "wb_table",
                    "schema_version": 1,
                    "endpoint": "wb_table",
                    "value_kind": "table",
                    "unit": {"canonical": None, "required": False},
                    "missing": {"allowed": [], "null_allowed": True},
                    "source": {"raw": True, "derived": False},
                    "link": {"timepoint_required": False, "sample_required": False},
                    "table": {
                        "columns": [
                            {"key": "lane", "label": "Lane", "value_kind": "numeric"},
                            {"key": "grade", "label": "Grade", "value_kind": "categorical"},
                        ]
                    },
                },
            ],
            "readouts": [
                {
                    "schema_id": "wb_grade",
                    "schema_version": 1,
                    "value": {"kind": "categorical", "category": "positive"},
                    "source": {"kind": "raw", "derived_from": []},
                },
                {
                    "schema_id": "wb_table",
                    "schema_version": 1,
                    "value": {"kind": "table", "columns": ["lane", "grade"], "rows": [[1, "+"], [2, "+++"]]},
                    "source": {"kind": "raw", "derived_from": []},
                },
            ],
        },
    )
    if typed["accepted"] != 2:
        raise Failure(f"invivo: expected 2 non-scalar readouts, got {typed['accepted']}")

    # scalar BCA/ELISA readout on the shared protocol step
    call(
        data,
        "readout.submit",
        {
            "run_id": run["id"],
            "step_path": bca_path,
            "protocol_version_id": wb_protocol,
            "endpoint": "protein_concentration",
            "value": 1.84,
            "unit": "ug/mL",
            "sample_id": "m-01-b",
            "assay_id": "bca_elisa",
            "timepoint": "D14",
        },
    )

    # rollback: an event citing an unknown check writes nothing
    before = checksum(data)
    events_before = record_count(data, "execution_event", project)
    rejected(
        data,
        "execution_event.create",
        {
            "run_id": run["id"],
            "event_type": "operation",
            "content": "m-02 未知检查项",
            "protocol_step_path": dose_path,
            "protocol_version_id": dose_protocol,
            "assay_id": "efficacy",
            "timepoint": "D10",
            "sample_ids": ["m-02"],
            "checks": [{"check_id": "chk-not-in-protocol", "status": "passed"}],
        },
    )
    if record_count(data, "execution_event", project) != events_before:
        raise Failure("invivo: rejected event was still appended")
    if checksum(data) != before:
        raise Failure("invivo: rejected event changed the database")

    by_timepoint = {
        step["timepoint"]: step
        for step in call(data, "workflow.progress", {"run_id": run["id"]})["steps"]
        if step["step_path"] == dose_path
    }
    if by_timepoint["D10"]["censored_dead_samples"] != 1:
        raise Failure(f"invivo: D10 censored_dead not projected: {by_timepoint['D10']}")
    m04 = next(row for row in by_timepoint["D10"]["samples"] if row["sample_id"] == "m-04")
    if m04["status"] != "censored_dead":
        raise Failure(f"invivo: censored_dead animal not excluded at D10: {m04['status']}")

    return {
        "case": "in-vivo efficacy + WB/BCA-ELISA",
        "project_id": project,
        "protocol_versions": {"efficacy": dose_protocol, "wb": sample_protocol_version, "bca_elisa": wb_protocol},
        "run_id": run["id"],
        "scheduled_tasks": len(efficacy_steps),
        "dosing_events": 4,
        "censored_dead_event": "m-04",
        "aliquot_event": split["id"],
        "wb_lanes": 3,
        "non_scalar_readouts": typed["accepted"],
        "rollback": "PASS",
        "boundary": "PASS",
        "restart": restart_reads(data, run["id"]),
        "concurrent_queries": concurrent_reads(data, run["id"], project),
        "checksum": checksum(data),
        "data_dir": data,
    }


CASES = [("huvec", case_huvec), ("rtqpcr", case_rtqpcr), ("invivo", case_invivo)]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true", help="emit machine-readable results")
    args = parser.parse_args()
    if not CLI.is_file():
        print(f"labworkbench not found at {CLI}; build it or set LABWORKBENCH_CLI", file=sys.stderr)
        return 1
    TARGET.mkdir(parents=True, exist_ok=True)
    parent = Path(tempfile.mkdtemp(prefix="qa09-three-protocol-", dir=TARGET))
    results = []
    try:
        for name, case in CASES:
            print(f"== {name} ==", flush=True)
            result = case(parent)
            summary = {key: value for key, value in result.items() if key not in {"data_dir", "project_id", "design_id", "protocol_version_id"}}
            print(json.dumps(summary, ensure_ascii=False), flush=True)
            results.append({key: value for key, value in result.items() if key != "data_dir"})
    except Failure as error:
        print(f"FAIL: {error}", file=sys.stderr)
        records = [*results_head(results), {"failure": str(error)}]
        write_evidence(records, failed=True)
        return 1
    write_evidence(results)
    if args.json:
        print(json.dumps(results, ensure_ascii=False, indent=2))
    print("QA-09 three-protocol CLI acceptance: PASS")
    return 0


def results_head(results: list) -> list:
    return [dict(item) for item in results]


def write_evidence(results: list, failed: bool = False) -> None:
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    (EVIDENCE / "results.json").write_text(
        json.dumps({"failed": failed, "cases": results}, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Failure as error:
        print(f"FAIL: {error}", file=sys.stderr)
        sys.exit(1)
