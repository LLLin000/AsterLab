#!/usr/bin/env python3
"""Fixture tests for scripts/sdk/check.py and scripts/sdk/scaffold.py.

Run directly:

    python3 scripts/sdk/test_sdk_tools.py

Each case builds a minimal but real-shaped repository root in a temporary
directory: the manifest, the Go registries, the Rust mirror and the catalog.
The tools read source text, so the fixtures keep the same declarations the real
files use instead of mocking the parser.
"""

from __future__ import annotations

import io
import json
import pathlib
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import check  # noqa: E402  (path bootstrap must precede the imports)
import manifest_spec as spec  # noqa: E402
import scaffold  # noqa: E402

MANIFEST = {
    "schemaVersion": 1,
    "foundationVersion": 1,
    "sdkVersion": "1.0.0",
    "modules": [
        {
            "kind": "alpha",
            "schema": 1,
            "title": "Alpha",
            "sdk": "1.0.0",
            "category": "measurement",
            "permissions": ["document.edit"],
            "renderers": {"document": "alpha.viewer"},
            "inputs": [
                {"name": "kind", "type": "string", "required": True},
                {"name": "title", "type": "string"},
            ],
            "outputs": [],
            "actions": [
                {
                    "id": "edit",
                    "title": "Edit",
                    "permission": "document.edit",
                    "writes": ["title"],
                }
            ],
            "rules": ["alpha-fields"],
        }
    ],
}

GO_SPEC = """package asterdoc

var foundationPortTypes = map[string]bool{
\t"string": true, "int": true, "object": true,
}

var foundationCategories = map[string]bool{
\t"measurement": true,
}

var knownModuleRules = map[string]bool{
\t"alpha-fields": true,
}
"""

GO_REGISTRY = """package asterdoc

const (
\tPermDocumentEdit = "document.edit"
\tPermRunReadout   = "run.readout"
)

func BuiltinRegistry() *Registry {
\tregistry := NewRegistry()
\tspecs := []ModuleSpec{
\t\talphaSpec(),
\t}
\tfor _, module := range specs {
\t\tif err := registry.Register(module); err != nil {
\t\t\treturn registry
\t\t}
\t}
\treturn registry
}
"""

GO_MODULES = """package asterdoc

func alphaSpec() ModuleSpec {
\treturn moduleDocumentSpec(builtinDescriptor("alpha"), specValidator("alpha"), nil)
}
"""

RUST_CLI = """const ASTERDOC_RULES: &[&str] = &[
    "alpha-fields",
];

fn validate_asterdoc_spec(spec: &Value) -> Result<(), String> {
    let categories = ["measurement"];
    let types = ["string", "int", "object"];
    Ok(())
}

fn validate_asterdoc_module(body: &str) -> Result<(), CliError> {
    for rule in spec["rules"].as_array().into_iter().flatten() {
        match rule.as_str().unwrap_or("") {
            "alpha-fields" => {
                let _ = text("title");
            }
            other => {
                return Err(CliError::internal(format!("unknown rule {other}")))
            }
        }
    }
    Ok(())
}
"""

CATALOG = """# Catalog

> 权威来源：`spec/modules.json`（1 个 `kind@schema`，对应 1 个 `kind`）。

## 1. 已注册模块总表

| kind@schema | category |
|---|---|
| alpha@1 | measurement |

## 2. Ports
"""

FILES = {
    spec.MANIFEST: json.dumps(MANIFEST, indent=2) + "\n",
    spec.GO_SPEC: GO_SPEC,
    spec.GO_REGISTRY: GO_REGISTRY,
    spec.GO_MODULES: GO_MODULES,
    spec.RUST_CLI: RUST_CLI,
    spec.CATALOG: CATALOG,
}


class Fixture:
    def __init__(self):
        self._temporary = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self._temporary.name)
        for relative, content in FILES.items():
            self.write(relative, content)

    def write(self, relative, content):
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8", newline="\n")

    def manifest(self):
        return json.loads((self.root / spec.MANIFEST).read_text(encoding="utf-8"))

    def write_manifest(self, manifest):
        self.write(spec.MANIFEST, json.dumps(manifest, indent=2) + "\n")

    def cleanup(self):
        self._temporary.cleanup()


def run_scaffold(*arguments):
    stdout, stderr = io.StringIO(), io.StringIO()
    with redirect_stdout(stdout), redirect_stderr(stderr):
        code = scaffold.main(["--root", str(FIXTURE.root), *arguments])
    return code, stdout.getvalue(), stderr.getvalue()


FIXTURE = None


def setUpModule():
    global FIXTURE
    FIXTURE = Fixture()


def tearDownModule():
    FIXTURE.cleanup()


class CheckTest(unittest.TestCase):
    def setUp(self):
        # Every test starts from the pristine fixture files.
        for relative, content in FILES.items():
            FIXTURE.write(relative, content)

    def failing(self, report):
        return {result["id"] for result in report["checks"] if not result["ok"]}

    def test_valid_fixture_passes(self):
        report = check.check(FIXTURE.root)
        self.assertTrue(report["ok"], report["checks"])
        self.assertEqual(report["modules"], len(FIXTURE.manifest()["modules"]))

    def test_duplicate_manifest_key_fails(self):
        manifest = FIXTURE.manifest()
        manifest["modules"].append(json.loads(json.dumps(manifest["modules"][0])))
        FIXTURE.write_manifest(manifest)
        self.assertIn("manifest", self.failing(check.check(FIXTURE.root)))

    def test_schema_gap_fails(self):
        manifest = FIXTURE.manifest()
        gap = json.loads(json.dumps(manifest["modules"][0]))
        gap["schema"] = 3
        manifest["modules"].append(gap)
        FIXTURE.write_manifest(manifest)
        self.assertIn("schema", self.failing(check.check(FIXTURE.root)))

    def test_undeclared_action_permission_fails(self):
        manifest = FIXTURE.manifest()
        manifest["modules"][0]["actions"][0]["permission"] = "run.readout"
        FIXTURE.write_manifest(manifest)
        self.assertIn("permissions", self.failing(check.check(FIXTURE.root)))

    def test_unregistered_rule_fails(self):
        manifest = FIXTURE.manifest()
        manifest["modules"][0]["rules"] = ["beta-fields"]
        FIXTURE.write_manifest(manifest)
        self.assertIn("manifest", self.failing(check.check(FIXTURE.root)))

    def test_catalog_count_mismatch_fails(self):
        FIXTURE.write(spec.CATALOG, CATALOG.replace("1 个 `kind@schema`", "2 个 `kind@schema`"))
        self.assertIn("catalog", self.failing(check.check(FIXTURE.root)))

    def test_rust_mirror_drift_fails(self):
        FIXTURE.write(spec.RUST_CLI, RUST_CLI.replace('    "alpha-fields",', '    "alpha-fields",\n    "extra-rule",'))
        self.assertIn("parity", self.failing(check.check(FIXTURE.root)))

    def test_duplicate_registration_fails(self):
        FIXTURE.write(
            spec.GO_REGISTRY,
            GO_REGISTRY.replace("\t\talphaSpec(),\n", "\t\talphaSpec(),\n\t\talphaSpec(),\n"),
        )
        self.assertIn("registration", self.failing(check.check(FIXTURE.root)))

    def test_json_report_carries_module_count(self):
        report = check.check(FIXTURE.root)
        encoded = json.loads(json.dumps(report))
        self.assertEqual(encoded["modules"], len(FIXTURE.manifest()["modules"]))


class ScaffoldTest(unittest.TestCase):
    def setUp(self):
        for relative, content in FILES.items():
            FIXTURE.write(relative, content)
        for directory in (FIXTURE.root / spec.SCAFFOLD_ROOT).glob("*"):
            if directory.is_dir():
                for child in directory.rglob("*"):
                    if child.is_file():
                        child.unlink()

    def test_dry_run_lists_files_and_writes_nothing(self):
        code, stdout, _ = run_scaffold("--kind", "beta", "--schema", "1", "--dry-run")
        self.assertEqual(code, 0)
        self.assertIn("would create", stdout)
        self.assertIn("manifest.json", stdout)
        self.assertFalse((FIXTURE.root / spec.SCAFFOLD_ROOT / "beta@1").exists())

    def test_refuses_schema_gap(self):
        code, _, stderr = run_scaffold("--kind", "alpha", "--schema", "3")
        self.assertEqual(code, 1)
        self.assertIn("next schema is 2", stderr)

    def test_refuses_duplicate(self):
        code, _, stderr = run_scaffold("--kind", "alpha", "--schema", "1")
        self.assertEqual(code, 1)
        self.assertIn("already exists", stderr)

    def test_refuses_unregistered_rule(self):
        code, _, stderr = run_scaffold("--kind", "beta", "--schema", "1", "--rule", "beta-fields")
        self.assertEqual(code, 1)
        self.assertIn("not registered", stderr)

    def test_refuses_unknown_category(self):
        code, _, stderr = run_scaffold("--kind", "beta", "--schema", "1", "--category", "device")
        self.assertEqual(code, 1)
        self.assertIn("not registered", stderr)

    def test_generated_skeleton_satisfies_check(self):
        code, _, _ = run_scaffold("--kind", "beta", "--schema", "1")
        self.assertEqual(code, 0)
        report = check.check(FIXTURE.root)
        scaffold_result = next(item for item in report["checks"] if item["id"] == "scaffold")
        self.assertTrue(scaffold_result["ok"], scaffold_result["problems"])
        self.assertEqual(scaffold_result["detail"], "1 scaffold directories consistent")
        fragment = json.loads(
            (FIXTURE.root / spec.SCAFFOLD_ROOT / "beta@1" / "manifest.json").read_text()
        )
        self.assertEqual(spec.module_key(fragment), "beta@1")
        self.assertEqual(fragment["sdk"], "1.0.0")

    def test_force_overwrites(self):
        self.assertEqual(run_scaffold("--kind", "beta", "--schema", "1")[0], 0)
        code, _, stderr = run_scaffold("--kind", "beta", "--schema", "1")
        self.assertEqual(code, 1)
        self.assertIn("--force", stderr)
        self.assertEqual(run_scaffold("--kind", "beta", "--schema", "1", "--force")[0], 0)


if __name__ == "__main__":
    unittest.main()
