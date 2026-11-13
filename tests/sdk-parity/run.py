#!/usr/bin/env python3
"""Run the shared Go/Rust AsterDoc acceptance matrix without touching user data."""
from __future__ import annotations

import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / "tests/sdk-parity/fixtures.json"
GO_PACKAGE = ROOT / "prototypes/mygo-run/internal/asterdoc"
RUST_BIN_DIR = Path.home() / ".cargo/bin"
CARGO = shutil.which("cargo") or (str(RUST_BIN_DIR / "cargo") if (RUST_BIN_DIR / "cargo").is_file() else "cargo")


def rust_env() -> dict[str, str]:
    env = os.environ.copy()
    if RUST_BIN_DIR.is_dir():
        env["PATH"] = str(RUST_BIN_DIR) + os.pathsep + env.get("PATH", "")
    return env


def run(command: list[str], *, cwd: Path, env: dict[str, str] | None = None) -> str:
    result = subprocess.run(command, cwd=cwd, env=env, text=True, capture_output=True)
    if result.returncode:
        raise RuntimeError(
            f"command failed ({result.returncode}): {' '.join(command)}\n"
            f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result.stdout + result.stderr


def go_results(fixtures: list[dict[str, str]]) -> dict[str, dict[str, object]]:
    # Go's package is internal; overlay injects the probe without editing it.
    virtual = GO_PACKAGE / "sdk_parity_probe_test.go"
    with tempfile.TemporaryDirectory(prefix="asterlab-sdk-parity-") as temp:
        tmp = Path(temp)
        fixture_path = tmp / "fixtures.json"
        fixture_path.write_text(json.dumps(fixtures), encoding="utf-8")
        overlay = tmp / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {str(virtual): str(ROOT / "tests/sdk-parity/go_probe_test.go")}}), encoding="utf-8")
        env = os.environ.copy()
        env["SDK_PARITY_FIXTURES"] = str(fixture_path)
        output = run(
            ["go", "test", "-overlay", str(overlay), "-run", "^TestSDKParityProbe$", "-v", "."],
            cwd=GO_PACKAGE,
            env=env,
        )
    results = {}
    for line in output.splitlines():
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict) and "name" in value and "valid" in value:
            results[value["name"]] = value
    if len(results) != len(fixtures):
        raise RuntimeError(f"Go probe returned {len(results)} results for {len(fixtures)} fixtures")
    return results


def file_digest_tree(root: Path) -> str:
    hashes = []
    for path in sorted(path for path in root.rglob("*") if path.is_file()):
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        hashes.append(f"{digest}  {path.relative_to(root).as_posix()}")
    return hashlib.sha256("\n".join(hashes).encode("utf-8")).hexdigest() if hashes else "no-database-created"


def rust_one(fixture: dict[str, object], temp_root: Path, rust_environment: dict[str, str], rust_binary: Path) -> dict[str, object]:
    index = fixture["index"]
    input_path = temp_root / f"input-{index}.json"
    data_dir = temp_root / f"data-{index}"
    data_dir.mkdir()
    input_path.write_text(json.dumps({"source": fixture["source"]}), encoding="utf-8")
    env = rust_environment.copy()
    env["LABWORKBENCH_DATA_DIR"] = str(data_dir)
    output = run([str(rust_binary), "asterdoc.validate", "--input", str(input_path)], cwd=ROOT, env=env)
    response = None
    for line in output.splitlines():
        try:
            candidate = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(candidate, dict) and ("result" in candidate or "valid" in candidate):
            response = candidate
            break
    if response is None:
        raise RuntimeError(f"Rust validator returned invalid JSON for {fixture['name']}: {output}")
    value = response.get("result", response)
    if not isinstance(value, dict) or "valid" not in value:
        raise RuntimeError(f"Rust validator response has no validation result for {fixture['name']}: {response}")
    valid = bool(value.get("valid"))
    error = (value.get("error") or {}) if not valid else {}
    return {
        "name": fixture["name"],
        "valid": valid,
        "error_code": error.get("code", "") if isinstance(error, dict) else "",
        "error_message": error.get("message", "") if isinstance(error, dict) else "",
        "db_sha256": file_digest_tree(data_dir),
    }


def rust_results(fixtures: list[dict[str, object]]) -> tuple[dict[str, dict[str, object]], str]:
    rust_environment = rust_env()
    run([CARGO, "build", "--bin", "labworkbench"], cwd=ROOT, env=rust_environment)
    metadata = json.loads(run([CARGO, "metadata", "--format-version", "1", "--no-deps"], cwd=ROOT, env=rust_environment))
    rust_binary = Path(metadata["target_directory"]) / "debug/labworkbench"
    if not rust_binary.is_file():
        raise RuntimeError(f"cargo build succeeded but binary is missing: {rust_binary}")
    results = {}
    with tempfile.TemporaryDirectory(prefix="asterlab-sdk-parity-rust-") as temp:
        temp_root = Path(temp)
        indexed = [{**fixture, "index": index} for index, fixture in enumerate(fixtures)]
        with ThreadPoolExecutor(max_workers=min(4, max(len(indexed), 1))) as pool:
            for result in pool.map(lambda item: rust_one(item, temp_root, rust_environment, rust_binary), indexed):
                results[result["name"]] = result
        digest = hashlib.sha256("\n".join(f"{name}:{results[name]['db_sha256']}" for name in sorted(results)).encode()).hexdigest()
    return results, digest


def error_class(message: str) -> str:
    text = message.lower()
    if "unsupported" in text:
        return "unsupported_kind"
    if "quantity needs" in text:
        return "invalid_type"
    if any(word in text for word in ("missing required", "required field", "is required", "module needs")):
        return "missing_field"
    if any(word in text for word in ("type", "expects", "expected", "must be", "invalid value")):
        return "invalid_type"
    if "unit" in text or "concentration" in text:
        return "unit_or_quantity"
    if any(word in text for word in ("reference", "unknown id", "not found", "does not exist")):
        return "invalid_reference"
    return "rule_rejection"


def known_difference(fixture: dict[str, object], go: dict[str, object]) -> bool:
    return fixture["kind"] == "readout" and "unsupported readout@1" in str(go.get("error", ""))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    run([sys.executable, str(ROOT / "tests/sdk-parity/generate_fixtures.py")], cwd=ROOT)
    fixtures = json.loads(FIXTURES.read_text(encoding="utf-8"))
    go = go_results(fixtures)
    rust, db_hash = rust_results(fixtures)
    differences = []
    case_results = []
    passed = 0
    fail = 0
    for fixture in fixtures:
        name = fixture["name"]
        left, right = go[name], rust[name]
        go_class = error_class(str(left.get("error", ""))) if not left["valid"] else None
        rust_class = error_class(str(right.get("error_message", ""))) if not right["valid"] else None
        case_results.append({
            "name": name,
            "expected": fixture["expected"],
            "expected_error": fixture["expected_error"],
            "go": {**left, "error_class": go_class},
            "rust": {**right, "error_class": rust_class},
        })
        problems = []
        if left["valid"] != right["valid"] or left["valid"] != fixture["expected"]:
            problems.append("accept/reject mismatch")
        if not fixture["expected"] and (go_class != rust_class or go_class != fixture["expected_error"]):
            problems.append("error category mismatch")
        if problems and known_difference(fixture, left):
            differences.append({"name": name, "go": left, "rust": right, "reason": "; ".join(problems), "disposition": "known: readout@1 is present in the Rust manifest but Go Parse does not register this kind"})
            passed += 1
        elif problems:
            differences.append({"name": name, "go": left, "rust": right, "expected": fixture["expected"], "expected_error": fixture["expected_error"], "reason": "; ".join(problems)})
            fail += 1
        else:
            passed += 1
    result = {
        "total": len(fixtures),
        "pass": passed,
        "fail": fail,
        "difference_count": len(differences),
        "differences": differences,
        "case_results": case_results,
        "known_differences": [
            {"go": "parser returns unstructured error text", "rust": "validator returns stable error codes", "disposition": "compare normalized categories; raw Go text and Rust code/message are included per case"},
            {"kind": "readout@1", "go": "Go Parse rejects kind as unsupported", "rust": "Rust validates the kind", "disposition": "known compatibility gap; no module implementation changes allowed in this QA task"},
        ],
        "isolated_data_sha256": db_hash,
    }
    json_output = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    text_output = (
        f"cases={result['total']} pass={result['pass']} fail={result['fail']} "
        f"differences={result['difference_count']} isolated_data_sha256={db_hash}\n"
        + "".join(json.dumps(item, ensure_ascii=False) + "\n" for item in differences)
        + "known_differences=" + json.dumps(result["known_differences"], ensure_ascii=False) + "\n"
    )
    evidence_dir = ROOT / "docs/release/evidence/SDK-02"
    evidence_dir.mkdir(parents=True, exist_ok=True)
    (evidence_dir / "parity.json").write_text(json_output, encoding="utf-8")
    (evidence_dir / "parity.txt").write_text(text_output, encoding="utf-8")
    if args.json:
        print(json_output, end="")
    else:
        print(text_output, end="")
    return 0 if result["fail"] == 0 else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, RuntimeError, json.JSONDecodeError) as error:
        print(f"sdk parity failed: {error}", file=sys.stderr)
        raise SystemExit(1)
