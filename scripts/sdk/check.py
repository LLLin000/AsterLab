#!/usr/bin/env python3
"""SDK-01: check the AsterDoc module manifest against its Go/Rust mirrors.

Checks
------
* `manifest`      structural contract of `spec/modules.json`
                  (identity, ports, references, derived outputs, rules, lifecycle).
* `registration`  every `kind@schema` registered exactly once by
                  `BuiltinRegistry()`, and the registered set equals the manifest.
* `schema`        each kind's schemas are 1..N with no gap and no duplicate.
* `permissions`   `actions[].permission` is declared in the module and backed by
                  a Go `Perm*` constant.
* `catalog`       `docs/sdk/module-catalog-v1.md` lists exactly the manifest keys
                  and its declared counts match the manifest.
* `parity`        the Go and Rust rule names, Foundation port types, categories
                  and rule dispatch arms are the same sets.
* `scaffold`      generated skeleton directories under
                  `internal/asterdoc/_scaffold/` agree with their fragments.

Exit codes: 0 = all checks pass, 1 = contract violation, 2 = unusable input.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import manifest_spec as spec  # noqa: E402  (path bootstrap must precede the import)

SCAFFOLD_FILES = ("README.md", "manifest.json", "module.go", "registration.go", "module.rs", "module_test.go")
SCAFFOLD_CONST_RE = re.compile(
    r'SCAFFOLDED_MODULE: \(&str, i64\) = \("([^"]+)", ([0-9]+)\);'
)


def _append(report, check_id, problems, detail):
    report["checks"].append(
        {
            "id": check_id,
            "ok": not problems,
            "detail": detail if not problems else f"{len(problems)} problem(s)",
            "problems": problems,
        }
    )
    if problems:
        report["ok"] = False


def _check_manifest(report, manifest, port_types, categories, rules):
    problems = []
    if manifest.get("schemaVersion") != 1:
        problems.append(f"schemaVersion must be 1, found {manifest.get('schemaVersion')!r}")
    if manifest.get("foundationVersion") != 1:
        problems.append(
            f"foundationVersion must be 1, found {manifest.get('foundationVersion')!r}"
        )
    sdk_version = manifest.get("sdkVersion")
    if not isinstance(sdk_version, str) or not sdk_version.strip():
        problems.append("sdkVersion must be a non-empty string")
        sdk_version = ""
    seen = {}
    for module in manifest["modules"]:
        key = spec.module_key(module)
        problems.extend(spec.validate_module(module, sdk_version, port_types, categories, rules))
        if key in seen:
            problems.append(f"{key}: declared twice")
        seen[key] = module
    _append(
        report,
        "manifest",
        problems,
        f"{len(seen)} modules declared",
    )


def _check_registration(report, manifest):
    order = spec.go_registration_order(report["root"])
    duplicates = sorted({key for key in order if order.count(key) > 1})
    problems = [f"{key}: registered twice by BuiltinRegistry()" for key in duplicates]
    manifest_keys = {spec.module_key(module) for module in manifest["modules"]}
    registered = set(order)
    for key in sorted(manifest_keys - registered):
        problems.append(f"{key}: declared in the manifest but not registered in Go")
    for key in sorted(registered - manifest_keys):
        problems.append(f"{key}: registered in Go but missing from the manifest")
    _append(
        report,
        "registration",
        problems,
        f"{len(registered)} Go registrations match the manifest",
    )


def _check_schema(report, manifest):
    by_kind = {}
    for module in manifest["modules"]:
        by_kind.setdefault(module.get("kind"), []).append(module.get("schema"))
    problems = []
    for kind, schemas in sorted(by_kind.items()):
        expected = list(range(1, len(schemas) + 1))
        if sorted(schemas) != expected:
            problems.append(
                f"{kind}: schemas {sorted(schemas)} are not the incrementing sequence {expected}"
            )
    _append(
        report,
        "schema",
        problems,
        "every kind numbers its schemas 1..N without gaps",
    )


def _check_permissions(report, manifest, permissions):
    problems = []
    for module in manifest["modules"]:
        problems.extend(spec.permission_problems(module, permissions))
    _append(
        report,
        "permissions",
        problems,
        f"declared permissions are backed by {len(permissions)} `Perm*` constants",
    )


def _check_catalog(report, manifest):
    rows, module_claims, kind_claims = spec.catalog_facts(report["root"])
    manifest_keys = {spec.module_key(module) for module in manifest["modules"]}
    kinds = {module.get("kind") for module in manifest["modules"]}
    problems = []
    if len(set(rows)) != len(rows):
        problems.append(f"{spec.CATALOG} repeats a table row")
    for key in sorted(manifest_keys - set(rows)):
        problems.append(f"{key}: missing from the catalog table")
    for key in sorted(set(rows) - manifest_keys):
        problems.append(f"{key}: listed in the catalog but absent from the manifest")
    if not module_claims:
        problems.append(f"{spec.CATALOG} does not declare a `kind@schema` count")
    for claim in sorted(module_claims):
        if claim != len(manifest["modules"]):
            problems.append(
                f"{spec.CATALOG} claims {claim} `kind@schema`, manifest has {len(manifest['modules'])}"
            )
    if not kind_claims:
        problems.append(f"{spec.CATALOG} does not declare a `kind` count")
    for claim in sorted(kind_claims):
        if claim != len(kinds):
            problems.append(
                f"{spec.CATALOG} claims {claim} `kind`, manifest has {len(kinds)}"
            )
    _append(
        report,
        "catalog",
        problems,
        f"catalog rows and counts match {len(manifest_keys)} manifest keys / {len(kinds)} kinds",
    )


def _check_parity(report):
    go_rules = spec.go_rules(report["root"])
    rust_rules = spec.rust_rules(report["root"])
    go_types = spec.go_port_types(report["root"])
    rust_types = spec.rust_port_types(report["root"])
    go_categories = spec.go_categories(report["root"])
    rust_categories = spec.rust_categories(report["root"])
    dispatch = spec.rust_dispatch_rules(report["root"])
    problems = []
    for name, go_set, rust_set in (
        ("rule name", go_rules, rust_rules),
        ("Foundation port type", go_types, rust_types),
        ("module category", go_categories, rust_categories),
        ("rule dispatch arm", rust_rules, dispatch),
    ):
        for value in sorted(go_set - rust_set):
            problems.append(f"{name} {value!r}: present in Go, missing in Rust")
        for value in sorted(rust_set - go_set):
            problems.append(f"{name} {value!r}: present in Rust, missing in Go")
    _append(
        report,
        "parity",
        problems,
        f"Go/Rust agree on {len(go_rules)} rules, {len(go_types)} port types, "
        f"{len(go_categories)} categories and {len(dispatch)} dispatch arms",
    )


def _check_scaffold(report, manifest, port_types, categories, go_rules, rust_rules, permissions):
    root = Path(report["root"])
    scaffold_root = root / spec.SCAFFOLD_ROOT
    problems = []
    directories = []
    if scaffold_root.is_dir():
        directories = sorted(path for path in scaffold_root.iterdir() if path.is_dir())
    manifest_keys = {spec.module_key(module) for module in manifest["modules"]}
    for directory in directories:
        label = f"{spec.SCAFFOLD_ROOT}/{directory.name}"
        match = spec.SCAFFOLD_DIR_RE.match(directory.name)
        if match is None:
            problems.append(f"{label}: directory name must be <kind>@<schema>")
            continue
        kind, schema = match.group("kind"), int(match.group("schema"))
        expected = f"{kind}@{schema}"
        for name in SCAFFOLD_FILES:
            if not (directory / name).is_file():
                problems.append(f"{label}: missing {name}")
        fragment_path = directory / "manifest.json"
        if not fragment_path.is_file():
            continue
        try:
            fragment = json.loads(fragment_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            problems.append(f"{label}: manifest.json is unusable: {error}")
            continue
        if spec.module_key(fragment) != expected:
            problems.append(
                f"{label}: manifest.json declares {spec.module_key(fragment)}, directory says {expected}"
            )
            continue
        problems.extend(
            spec.validate_module(
                fragment, manifest.get("sdkVersion", ""), port_types, categories, go_rules
            )
        )
        problems.extend(spec.permission_problems(fragment, permissions))
        for rule in fragment.get("rules", []) if isinstance(fragment.get("rules"), list) else []:
            if rule not in rust_rules:
                problems.append(f"{label}: rule {rule!r} is not registered in Rust")
        rust_path = directory / "module.rs"
        if not rust_path.is_file():
            continue
        declaration = SCAFFOLD_CONST_RE.search(rust_path.read_text(encoding="utf-8"))
        if declaration is None:
            problems.append(f"{label}: module.rs has no SCAFFOLDED_MODULE declaration")
        elif (declaration.group(1), int(declaration.group(2))) != (kind, schema):
            problems.append(
                f"{label}: module.rs declares {declaration.group(1)}@{declaration.group(2)}"
            )
        if expected in manifest_keys:
            problems.append(
                f"{label}: skeleton is stale; {expected} is already in the manifest"
            )
    _append(
        report,
        "scaffold",
        problems,
        f"{len(directories)} scaffold directories consistent",
    )


def check(root):
    root = Path(root).resolve()
    manifest = spec.load_manifest(root)
    port_types = spec.go_port_types(root)
    categories = spec.go_categories(root)
    go_rules = spec.go_rules(root)
    rust_rules = spec.rust_rules(root)
    permissions = spec.go_permissions(root)
    report = {
        "ok": True,
        "root": str(root),
        "manifest": spec.MANIFEST,
        "sdkVersion": manifest.get("sdkVersion"),
        "modules": len(manifest["modules"]),
        "kinds": len({module.get("kind") for module in manifest["modules"]}),
        "checks": [],
    }
    _check_manifest(report, manifest, port_types, categories, go_rules)
    _check_registration(report, manifest)
    _check_schema(report, manifest)
    _check_permissions(report, manifest, permissions)
    _check_catalog(report, manifest)
    _check_parity(report)
    _check_scaffold(
        report, manifest, port_types, categories, go_rules, rust_rules, permissions
    )
    return report


def render_text(report):
    lines = [
        f"sdk check: {report['manifest']} — {report['modules']} modules, "
        f"{report['kinds']} kinds, sdk {report['sdkVersion']}"
    ]
    for result in report["checks"]:
        status = "ok  " if result["ok"] else "FAIL"
        lines.append(f"  {status} {result['id']:<13} {result['detail']}")
        for problem in result["problems"]:
            lines.append(f"         - {problem}")
    failed = sum(1 for result in report["checks"] if not result["ok"])
    lines.append("sdk check: ok" if failed == 0 else f"sdk check: {failed} check(s) failed")
    return "\n".join(lines)


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Check the AsterDoc module manifest against its Go and Rust mirrors."
    )
    parser.add_argument("--json", action="store_true", help="print the report as JSON")
    parser.add_argument(
        "--root",
        default=str(spec.REPO_ROOT),
        help="repository root (default: the checkout containing this script)",
    )
    arguments = parser.parse_args(argv)
    try:
        report = check(arguments.root)
    except spec.SpecError as error:
        print(f"sdk check: {error}", file=sys.stderr)
        return 2
    if arguments.json:
        print(json.dumps(report, indent=2, ensure_ascii=False))
    else:
        print(render_text(report))
    return 0 if report["ok"] else 1


if __name__ == "__main__":
    sys.exit(main())
