#!/usr/bin/env python3
"""Generate repeatable valid/malformed documents from the registered module schemas."""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
MANIFEST = ROOT / "prototypes/mygo-run/internal/asterdoc/spec/modules.json"
OUTPUT = ROOT / "tests/sdk-parity/fixtures.json"


def default(port: dict[str, object]) -> object:
    kind = port["type"]
    if kind == "enum":
        return port["values"][0]
    if kind == "string":
        return "value"
    if kind == "int":
        return 1
    if kind == "decimal":
        return 0.1
    if kind == "bool":
        return True
    if kind == "quantity":
        dimension = port.get("dimension")
        unit = {"volume": "mL", "mass_concentration": "mg/mL"}.get(dimension, "mL")
        return {"value": 1, "unit": unit}
    if kind == "reference":
        return "group-1" if port.get("target") == "group" else "assay-1"
    if kind == "well-map":
        return {"group-1": ["A1"]}
    if kind == "geometry":
        return {}
    if kind == "object-list":
        return []
    if kind == "string[]":
        return ["value"]
    if kind == "list":
        item_type = port.get("item_type")
        if item_type == "reference":
            return ["step-1"]
        if item_type == "object":
            return [{"name": "capability"}]
        return []
    if kind == "object":
        return {}
    raise ValueError(f"unhandled manifest input type {kind!r} for {port['name']}")


def module_data(module: dict[str, object], identifier: str) -> dict[str, object]:
    data = {"kind": module["kind"], "id": identifier, "schema": module["schema"]}
    for port in module.get("inputs", []):
        name = port["name"]
        if name in {"kind", "id", "schema"}:
            continue
        if port.get("required"):
            data[name] = default(port)
    kind = module["kind"]
    if kind == "plate":
        data.update(format=96, groups={"group-1": ["A1"]})
    elif kind == "prose":
        data["body"] = "Sample instruction"
    elif kind == "group_table":
        data["rows"] = [{"id": "g-ctrl", "name": "Control", "role": "control"}]
    elif kind == "param_table":
        data["rows"] = [{"key": "od600", "label": "OD600", "value": 0.5, "unit": "AU"}]
    elif kind == "figure":
        data["svg"] = "<svg viewBox=\"0 0 100 100\"><circle cx=\"50\" cy=\"50\" r=\"40\"/></svg>"
    elif kind == "data_table":
        data["columns"] = [{"key": "value", "label": "Value", "value_kind": "number", "unit": "mm"}]
    elif kind == "checklist":
        data["items"] = [{"id": "chk-1", "label": "Verify sterile", "required": True}]
    elif kind == "calculator":
        data.update(
            mode="dilution",
            stock={"value": 10, "unit": "mg/mL"},
            target={"value": 2, "unit": "mg/mL"},
            final_volume={"value": 1, "unit": "mL"},
        )
    elif kind == "timer":
        data.update(mode="countdown", duration={"value": 10, "unit": "min"})
    elif kind == "attachment":
        data.update(scope="step_evidence", media_hint="Dish photo")
    elif kind == "subflow":
        data.update(title="Subflow", items=[{"id": "sf-1", "title": "Prepare tube"}])
    elif kind == "instrument_profile":
        data.update(capabilities=[{"name": "speed"}])
    elif kind == "step":
        data["execution"] = "executable"
        data["assay"] = "assay-1"
    return data


def yaml_scalar(value: object) -> str:
    if isinstance(value, str):
        return json.dumps(value, ensure_ascii=False)
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))


def render(data: dict[str, object]) -> str:
    body = "\n".join(f"{key}: {yaml_scalar(value)}" for key, value in data.items())
    return f"```asterlab\n{body}\n```\n"


def dependencies(kind: str, schema: int) -> list[dict[str, object]]:
    output = []
    if kind != "experiment":
        exp = next(item for item in manifest["modules"] if item["kind"] == "experiment")
        output.append(module_data(exp, "experiment-1"))
    if kind in {"plate", "sample"}:
        group = next(item for item in manifest["modules"] if item["kind"] == "group")
        output.append(module_data(group, "group-1"))
    if kind in {"step", "workflow"}:
        assay = next(item for item in manifest["modules"] if item["kind"] == "assay")
        output.append(module_data(assay, "assay-1"))
    if kind == "workflow":
        step = next(item for item in manifest["modules"] if item["kind"] == "step" and item["schema"] == 1)
        workflow_step = module_data(step, "step-1")
        workflow_step.pop("assay", None)
        workflow_step["execution"] = "organizational"
        output.append(workflow_step)
    return output


def source_for(data: dict[str, object], extra: list[dict[str, object]]) -> str:
    return "".join(render(item) for item in [*extra, data])


def cases_for(module: dict[str, object]) -> list[dict[str, object]]:
    kind, schema = module["kind"], module["schema"]
    base_id = f"{kind.replace('_', '-')}-{schema}"
    extras = dependencies(kind, schema)
    base = module_data(module, base_id)
    cases = [{"name": f"valid-{kind}-schema{schema}", "kind": kind, "schema": schema, "expected": True, "expected_error": None, "historical": schema < max(item["schema"] for item in manifest["modules"] if item["kind"] == kind), "source": source_for(base, extras)}]

    business_required = [p for p in module.get("inputs", []) if p.get("required") and p["name"] not in {"kind", "id", "schema"}]
    if business_required:
        required_name = business_required[0]["name"]
        missing = dict(base)
        missing.pop(required_name, None)
        cases.append({"name": f"invalid-{kind}-schema{schema}-missing-{required_name}", "kind": kind, "schema": schema, "expected": False, "expected_error": "missing_field", "source": source_for(missing, extras)})

    string_field = next((p for p in module.get("inputs", []) if p["type"] == "string" and p["name"] not in {"kind", "id"}), None)
    wrong_name = string_field["name"] if string_field is not None else (business_required[0]["name"] if business_required else "title")
    wrong_type = dict(base)
    wrong_type[wrong_name] = {"unexpected": "object"}
    cases.append({"name": f"invalid-{kind}-schema{schema}-wrong-type-{wrong_name}", "kind": kind, "schema": schema, "expected": False, "expected_error": "invalid_type", "source": source_for(wrong_type, extras)})
    special = dict(base)
    if kind == "plate":
        special["groups"] = {"group-1": ["Z99"]}
        special_name, error_kind = "out-of-range-well", "rule_rejection"
    elif kind == "calculator":
        special["target"] = {"value": 1, "unit": "mL"}
        special_name, error_kind = "invalid-unit", "unit_or_quantity"
    elif kind == "sample":
        special["group_ref"] = "missing-group"
        special_name, error_kind = "missing-reference", "invalid_reference"
    elif kind == "assay":
        special["group_refs"] = ["missing-group"]
        special_name, error_kind = "missing-reference", "invalid_reference"
    elif kind == "step":
        special["assay"] = "missing-assay"
        special_name, error_kind = "missing-reference", "invalid_reference"
    else:
        special_name = ""
        error_kind = ""
    if special_name:
        cases.append({"name": f"invalid-{kind}-schema{schema}-{special_name}", "kind": kind, "schema": schema, "expected": False, "expected_error": error_kind, "source": source_for(special, extras)})
    return cases


manifest = json.loads(MANIFEST.read_text(encoding="utf-8"))
fixtures = [case for module in manifest["modules"] for case in cases_for(module)]
OUTPUT.write_text(json.dumps(fixtures, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print(f"generated {len(fixtures)} fixtures for {len(manifest['modules'])} kind/schema entries")
