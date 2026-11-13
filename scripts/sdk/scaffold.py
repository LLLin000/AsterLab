#!/usr/bin/env python3
"""SDK-01: generate an AsterDoc module skeleton from the v1 contract.

The generator refuses the cases that would fail `scripts/sdk/check.py` later:
a duplicate `kind@schema`, a schema gap for an existing kind, an unknown
category or permission, or a rule name that is not registered in both the Go
and Rust validators.

Output is written under `prototypes/mygo-run/internal/asterdoc/_scaffold/`.
Directories starting with `_` are ignored by the Go tool, so a skeleton never
changes a build until a maintainer applies it. The files are:

    README.md          apply checklist and guardrails
    manifest.json      object to append to spec/modules.json
    module.go          ModuleSpec constructor and renderer
    registration.go    the BuiltinRegistry() entry
    module.rs          Rust mirror declaration and rule registration note
    module_test.go     minimal Go contract test (valid + rejected sample)

Exit codes: 0 = generated (or dry run), 1 = refused input, 2 = unusable repo.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import manifest_spec as spec  # noqa: E402  (path bootstrap must precede the import)

MODES = ("design", "protocol", "run", "notebook", "review")


class Refused(Exception):
    """Input the generator must not turn into files."""


def constructor_name(kind):
    head, *rest = kind.split("_")
    return head + "".join(part.capitalize() for part in rest) + "Spec"


def pascal(value):
    return "".join(part.capitalize() for part in value.split("_"))


def next_schema(manifest, kind):
    schemas = [module.get("schema") for module in manifest["modules"] if module.get("kind") == kind]
    return max(schemas) + 1 if schemas else 1


def validate_request(manifest, arguments, categories, go_rules, rust_rules, permissions):
    kind = arguments.kind
    if not spec.KIND_RE.match(kind):
        raise Refused(f"kind {kind!r} must match {spec.KIND_RE.pattern}")
    existing = [module for module in manifest["modules"] if module.get("kind") == kind]
    expected = next_schema(manifest, kind)
    if any(module.get("schema") == arguments.schema for module in existing):
        raise Refused(f"{kind}@{arguments.schema} already exists in {spec.MANIFEST}")
    if arguments.schema != expected:
        if existing:
            raise Refused(
                f"{kind} already has schema(s) "
                f"{sorted(module.get('schema') for module in existing)}; "
                f"the next schema is {expected}"
            )
        raise Refused(f"the first schema for a new kind must be 1, not {arguments.schema}")
    if arguments.category not in categories:
        raise Refused(
            f"category {arguments.category!r} is not registered; choose one of "
            f"{', '.join(sorted(categories))}"
        )
    if arguments.title is not None and not arguments.title.strip():
        raise Refused("--title cannot be empty")
    for permission in arguments.permission:
        if permission not in permissions:
            raise Refused(
                f"permission {permission!r} has no `Perm*` constant in {spec.GO_REGISTRY}"
            )
    for rule in arguments.rule:
        missing = [
            where
            for where, known in (("Go", go_rules), ("Rust", rust_rules))
            if rule not in known
        ]
        if missing:
            raise Refused(
                f"rule {rule!r} is not registered in {'/'.join(missing)}; register it in "
                f"knownModuleRules and ASTERDOC_RULES with a validator arm before scaffolding"
            )
    duplicates = {value for value in arguments.rule if arguments.rule.count(value) > 1}
    if duplicates:
        raise Refused(f"rules repeat: {', '.join(sorted(duplicates))}")


def manifest_fragment(manifest, arguments):
    kind = arguments.kind
    schema = arguments.schema
    return {
        "kind": kind,
        "schema": schema,
        "title": arguments.title or kind,
        "description": f"脚手架生成的 {kind} 模块；替换为它代表什么、不代表什么。",
        "sdk": manifest.get("sdkVersion", ""),
        "category": arguments.category,
        "capabilities": list(arguments.capability),
        "permissions": list(arguments.permission),
        "renderers": {
            "design": "generic.editor",
            "document": f"{kind}.viewer",
            "run": "generic.run",
        },
        "lifecycle": {"states": ["draft", "validated", "frozen"], "freeze": "immutable-revision"},
        "provenance": {"origin": "planned", "record_boundary": "run"},
        "modes": list(MODES),
        "inputs": [
            {"name": "kind", "type": "string", "required": True, "description": f"固定为 {kind}"},
            {"name": "id", "type": "string", "required": True, "description": "文档内稳定 ID"},
            {
                "name": "schema",
                "type": "int",
                "required": True,
                "description": f"模块版本，当前为 {schema}",
            },
            {"name": "title", "type": "string", "required": True, "description": "显示标题"},
        ],
        "outputs": [
            {
                "name": "module_view",
                "type": "object",
                "description": "只读投影；不写 Run 事实",
                "origin": "view",
            }
        ],
        "actions": [],
        "rules": list(arguments.rule),
    }


def go_module(arguments):
    kind = arguments.kind
    constructor = constructor_name(kind)
    return f"""package asterdoc

import "github.com/egoist/mygo/ui"

// {constructor} implements {kind}@{arguments.schema}. Copy this file into
// prototypes/mygo-run/internal/asterdoc/ and register it in registry.go.
func {constructor}() ModuleSpec {{
	return moduleDocumentSpec(
		builtinDescriptor("{kind}"),
		specValidator("{kind}"),
		func(v *View, c *ui.Context, data map[string]any) {{
			v.GenericModuleSummary(c, data)
		}},
	)
}}
"""


def go_registration(arguments):
    constructor = constructor_name(arguments.kind)
    return f"""package asterdoc

// Register {arguments.kind}@{arguments.schema} in BuiltinRegistry()
// (registry.go) by appending the constructor to the specs slice:
//
//	specs := []ModuleSpec{{
//		...,
//		{constructor}(),
//	}}
//
// Register rejects a duplicate kind@schema, a nil Validate func, and a declared
// renderer key without an implementation.
"""


def go_test(arguments):
    kind = arguments.kind
    schema = arguments.schema
    return f"""package asterdoc

import "testing"

func Test{pascal(kind)}SpecContract(t *testing.T) {{
	spec, ok := BuiltinRegistry().Lookup("{kind}", {schema})
	if !ok {{
		t.Fatalf("{kind}@{schema} is not registered")
	}}
	if spec.Category != "{arguments.category}" {{
		t.Fatalf("category = %q, want {arguments.category}", spec.Category)
	}}
	valid := map[string]any{{
		"kind":   "{kind}",
		"id":     "{kind_slug(kind)}-01",
		"schema": {schema},
		"title":  "{arguments.title or kind}",
	}}
	if err := spec.Validate(valid); err != nil {{
		t.Fatalf("valid module rejected: %v", err)
	}}
	missing := map[string]any{{"kind": "{kind}", "id": "{kind_slug(kind)}-01", "schema": {schema}}}
	if err := spec.Validate(missing); err == nil {{
		t.Fatal("module without title accepted")
	}}
}}
"""


def kind_slug(kind):
    return kind.replace("_", "-")


def rust_mirror(arguments):
    kind = arguments.kind
    schema = arguments.schema
    if arguments.rule:
        rules = ", ".join(repr(rule) for rule in arguments.rule)
        note = (
            "// Rules declared by this module: "
            + rules
            + "\n// Each name must already have an ASTERDOC_RULES entry and a\n"
            "// validate_asterdoc_module match arm in src/cli.rs.\n"
        )
    else:
        note = (
            "// This module declares no custom rule, so src/cli.rs needs no edit:\n"
            "// include_str! embeds spec/modules.json and the descriptor is shared.\n"
        )
    return f"""// Rust mirror for {kind}@{schema}, generated by scripts/sdk/scaffold.py.
//
// src/cli.rs embeds prototypes/mygo-run/internal/asterdoc/spec/modules.json via
// include_str!, so the descriptor, ports and permissions have no second
// declaration to keep in sync. Only rule names are exhaustive in Rust.
{note}pub(crate) const SCAFFOLDED_MODULE: (&str, i64) = ("{kind}", {schema});
"""


def readme(arguments):
    kind = arguments.kind
    constructor = constructor_name(kind)
    if arguments.rule:
        rules = ", ".join(f"`{rule}`" for rule in arguments.rule)
        rust_step = (
            f"4. Rust: {rules} must already be in `ASTERDOC_RULES` and in the\n"
            "   `validate_asterdoc_module` match; this skeleton does not add arms."
        )
    else:
        rust_step = (
            "4. Rust: `src/cli.rs` embeds `spec/modules.json`; this module declares\n"
            "   no rule, so no declaration needs copying. `module.rs` records that."
        )
    return f"""# {kind}@{arguments.schema} module skeleton

Generated by `python3 scripts/sdk/scaffold.py --kind {kind} --schema {arguments.schema}`.
`_scaffold/` is ignored by the Go tool: nothing here compiles until it is applied.

## Apply

1. Append `manifest.json`'s object to `modules[]` in
   `prototypes/mygo-run/internal/asterdoc/spec/modules.json`.
2. Copy `module.go` and `module_test.go` into
   `prototypes/mygo-run/internal/asterdoc/`.
3. Add `registration.go`'s constructor to `BuiltinRegistry()` in `registry.go`:
   `{constructor}(),`
{rust_step}
5. Run `python3 scripts/sdk/check.py` and
   `cd prototypes/mygo-run && go test ./internal/asterdoc`.
6. Add one valid and one invalid sample to `TestAsterDocGoRustParity`.
7. Update the counts in `docs/sdk/module-catalog-v1.md` and the module's row.
8. Delete this `_scaffold/{kind}@{arguments.schema}/` directory.

## Guardrails

- Display modules never open a Core store or transaction; readouts stay Run facts.
- A declared renderer needs an implementation or the registry refuses it.
- Missing values stay missing; do not fill a scientific value from a title.
- No second parser and no new fence language: `kind` + `id` + `schema` still select the module.
"""


def build_files(manifest, arguments):
    fragment = manifest_fragment(manifest, arguments)
    return {
        "README.md": readme(arguments),
        "manifest.json": json.dumps(fragment, indent=2, ensure_ascii=False) + "\n",
        "module.go": go_module(arguments),
        "registration.go": go_registration(arguments),
        "module.rs": rust_mirror(arguments),
        "module_test.go": go_test(arguments),
    }


def display_path(path, root):
    try:
        return str(Path(path).relative_to(root))
    except ValueError:
        return str(path)


def generate(arguments):
    root = Path(arguments.root).resolve()
    manifest = spec.load_manifest(root)
    categories = spec.go_categories(root)
    go_rules = spec.go_rules(root)
    rust_rules = spec.rust_rules(root)
    permissions = spec.go_permissions(root)
    validate_request(manifest, arguments, categories, go_rules, rust_rules, permissions)
    files = build_files(manifest, arguments)
    target = Path(arguments.out)
    if not target.is_absolute():
        target = root / target
    target = target / f"{arguments.kind}@{arguments.schema}"
    rel_target = display_path(target, root)
    targets = {name: target / name for name in files}
    existing = sorted(name for name, path in targets.items() if path.exists())
    if existing and not arguments.force:
        raise Refused(
            f"{rel_target} already has {', '.join(existing)}; pass --force to overwrite"
        )
    action = "would create" if arguments.dry_run else "created"
    print(
        f"scaffold {arguments.kind}@{arguments.schema}: {len(files)} files under {rel_target}"
    )
    for name, path in sorted(targets.items()):
        print(f"{action} {display_path(path, root)}")
        if arguments.dry_run:
            continue
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(files[name], encoding="utf-8", newline="\n")
    if arguments.dry_run:
        print("dry run: no file written")
    return 0


def main(argv=None):
    parser = argparse.ArgumentParser(
        description="Generate an AsterDoc module skeleton that check.py accepts."
    )
    parser.add_argument("--kind", required=True, help="stable lowercase module kind")
    parser.add_argument("--schema", required=True, type=int, help="module schema version")
    parser.add_argument("--dry-run", action="store_true", help="print the file list only")
    parser.add_argument("--force", action="store_true", help="overwrite an existing skeleton")
    parser.add_argument("--title", help="manifest title (default: the kind)")
    parser.add_argument("--category", default="measurement", help="manifest category")
    parser.add_argument(
        "--capability",
        action="append",
        default=[],
        metavar="NAME",
        help="declared capability (repeatable)",
    )
    parser.add_argument(
        "--permission",
        action="append",
        default=[],
        metavar="PERMISSION",
        help="declared permission; must have a Go Perm* constant (repeatable)",
    )
    parser.add_argument(
        "--rule",
        action="append",
        default=[],
        metavar="RULE",
        help="rule name already registered in Go and Rust (repeatable)",
    )
    parser.add_argument(
        "--out",
        default=spec.SCAFFOLD_ROOT,
        help="output root (default: the ignored internal/asterdoc/_scaffold)",
    )
    parser.add_argument(
        "--root",
        default=str(spec.REPO_ROOT),
        help="repository root (default: the checkout containing this script)",
    )
    arguments = parser.parse_args(argv)
    try:
        return generate(arguments)
    except Refused as error:
        print(f"scaffold: refused: {error}", file=sys.stderr)
        return 1
    except spec.SpecError as error:
        print(f"scaffold: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
