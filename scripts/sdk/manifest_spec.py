#!/usr/bin/env python3
"""Load the AsterDoc module manifest and the Go/Rust mirrors it must match.

`scripts/sdk/check.py` and `scripts/sdk/scaffold.py` share this module. It uses
only the Python standard library so the SDK tools run on a bare interpreter on
Linux and Windows runners.

The manifest is authoritative; the Go and Rust validators mirror two field sets
from it (rule names, Foundation port types, categories). Those mirrors are
parsed from source text, not executed, so a drift fails here instead of at
startup on a user's document.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]

MANIFEST = "prototypes/mygo-run/internal/asterdoc/spec/modules.json"
GO_SPEC = "prototypes/mygo-run/internal/asterdoc/spec.go"
GO_REGISTRY = "prototypes/mygo-run/internal/asterdoc/registry.go"
GO_MODULES = "prototypes/mygo-run/internal/asterdoc/modules.go"
RUST_CLI = "src/cli.rs"
CATALOG = "docs/sdk/module-catalog-v1.md"
SCAFFOLD_ROOT = "prototypes/mygo-run/internal/asterdoc/_scaffold"

KIND_RE = re.compile(r"^[a-z][a-z0-9_]*$")
SCAFFOLD_DIR_RE = re.compile(r"^(?P<kind>[a-z][a-z0-9_]*)@(?P<schema>[0-9]+)$")


class SpecError(Exception):
    """Input the tools cannot evaluate; callers exit 2 instead of guessing."""


def read_text(root, relative):
    path = Path(root) / relative
    try:
        return path.read_text(encoding="utf-8")
    except OSError as error:
        raise SpecError(f"cannot read {relative}: {error}") from error


def load_manifest(root):
    """Return the parsed manifest; structural problems raise SpecError."""
    text = read_text(root, MANIFEST)
    try:
        manifest = json.loads(text)
    except json.JSONDecodeError as error:
        raise SpecError(f"{MANIFEST} is not valid JSON: {error}") from error
    if not isinstance(manifest, dict):
        raise SpecError(f"{MANIFEST} must hold a JSON object")
    modules = manifest.get("modules")
    if not isinstance(modules, list):
        raise SpecError(f"{MANIFEST} needs a modules array")
    for module in modules:
        if not isinstance(module, dict):
            raise SpecError(f"{MANIFEST} modules[] entries must be objects")
    return manifest


def module_key(module):
    return f"{module.get('kind')}@{module.get('schema')}"


def go_string_set(root, relative, variable):
    """Parse `var <variable> = map[string]bool{...}` into a set of strings."""
    text = read_text(root, relative)
    match = re.search(
        rf"var {re.escape(variable)} = map\[string\]bool\{{(.*?)\n\}}", text, re.S
    )
    if match is None:
        raise SpecError(f"cannot find `var {variable} = map[string]bool{{` in {relative}")
    values = re.findall(r'"([^"]+)"', match.group(1))
    if not values:
        raise SpecError(f"`{variable}` in {relative} is empty")
    return set(values)


def go_port_types(root):
    return go_string_set(root, GO_SPEC, "foundationPortTypes")


def go_categories(root):
    return go_string_set(root, GO_SPEC, "foundationCategories")


def go_rules(root):
    return go_string_set(root, GO_SPEC, "knownModuleRules")


def go_permissions(root):
    """Parse the `Perm*` constants from registry.go."""
    text = read_text(root, GO_REGISTRY)
    values = re.findall(r'^\s*Perm\w+\s*=\s*"([^"]+)"', text, re.M)
    if not values:
        raise SpecError(f"cannot find `Perm*` constants in {GO_REGISTRY}")
    return set(values)


FUNC_RE = re.compile(r"^func (\w+)\(([^)]*)\) ModuleSpec \{", re.M)


def _go_constructors(root):
    """Map constructor name -> (parameter names, body)."""
    text = read_text(root, GO_MODULES)
    matches = list(FUNC_RE.finditer(text))
    if not matches:
        raise SpecError(f"cannot find any `func ... (...) ModuleSpec {{` in {GO_MODULES}")
    out = {}
    for index, match in enumerate(matches):
        end = matches[index + 1].start() if index + 1 < len(matches) else len(text)
        params = [
            part.strip().split()[0]
            for part in match.group(2).split(",")
            if part.strip()
        ]
        out[match.group(1)] = (params, text[match.end():end])
    return out


def _resolve_keys(name, arguments, constructors, stack):
    if name in stack:
        raise SpecError(f"constructor recursion through {name}()")
    if name not in constructors:
        raise SpecError(f"registered constructor {name}() is not defined in {GO_MODULES}")
    params, body = constructors[name]
    for param, argument in zip(params, arguments):
        body = re.sub(rf"\b{re.escape(param)}\b", argument, body)
    if re.search(r'descriptorFor\("[^"]+",\s*[A-Za-z_]', body):
        raise SpecError(
            f"cannot resolve the schema of a module declared in {name}(): "
            "pass an integer literal or a constructor with a literal argument"
        )
    keys = set()
    for kind in re.findall(r'builtinDescriptor\("([^"]+)"\)', body):
        keys.add(f"{kind}@1")
    for kind, schema in re.findall(r'descriptorFor\("([^"]+)",\s*(\d+)\)', body):
        keys.add(f"{kind}@{int(schema)}")
    if keys:
        return keys
    for callee, raw_arguments in re.findall(r"\b(\w+)\(([^()]*)\)", body):
        if callee not in constructors:
            continue
        arguments = [part.strip() for part in raw_arguments.split(",") if part.strip()]
        keys |= _resolve_keys(callee, arguments, constructors, stack | {name})
    if not keys:
        raise SpecError(f"constructor {name}() declares no descriptor")
    return keys


def go_registration_order(root):
    """Return the `kind@schema` keys registered by BuiltinRegistry(), in order."""
    constructors = _go_constructors(root)
    registry = read_text(root, GO_REGISTRY)
    match = re.search(r"specs := \[\]ModuleSpec\{(.*?)\}", registry, re.S)
    if match is None:
        raise SpecError(f"cannot find `specs := []ModuleSpec{{` in {GO_REGISTRY}")
    calls = [entry.strip() for entry in match.group(1).split(",") if entry.strip()]
    if not calls:
        raise SpecError(f"BuiltinRegistry() registers no module specs in {GO_REGISTRY}")
    order = []
    for call in calls:
        call_match = re.fullmatch(r"(\w+)\(([^()]*)\)", call)
        if call_match is None:
            raise SpecError(f"unrecognised spec entry {call!r} in {GO_REGISTRY}")
        arguments = [
            part.strip() for part in call_match.group(2).split(",") if part.strip()
        ]
        resolved = _resolve_keys(call_match.group(1), arguments, constructors, set())
        order.extend(sorted(resolved))
    return order


def _string_block(root, relative, start_pattern, stop_pattern):
    text = read_text(root, relative)
    start = re.search(start_pattern, text)
    if start is None:
        raise SpecError(f"cannot find {start_pattern!r} in {relative}")
    stop = re.search(stop_pattern, text[start.end():])
    if stop is None:
        raise SpecError(f"cannot find the end of the block starting at {start_pattern!r} in {relative}")
    return text[start.end():start.end() + stop.start()]


def rust_rules(root):
    block = _string_block(
        root, RUST_CLI, r"const ASTERDOC_RULES: &\[&str\] = &\[", r"\];"
    )
    values = re.findall(r'"([^"]+)"', block)
    if not values:
        raise SpecError(f"ASTERDOC_RULES in {RUST_CLI} is empty")
    return set(values)


def rust_port_types(root):
    block = _string_block(root, RUST_CLI, r"let types = \[", r"\];")
    return set(re.findall(r'"([^"]+)"', block))


def rust_categories(root):
    block = _string_block(root, RUST_CLI, r"let categories = \[", r"\];")
    return set(re.findall(r'"([^"]+)"', block))


def rust_dispatch_rules(root):
    """Rule names with an arm in `validate_asterdoc_module`'s match block."""
    text = read_text(root, RUST_CLI)
    start = re.search(r'match rule\.as_str\(\)\.unwrap_or\(""\) \{', text)
    if start is None:
        raise SpecError(f"cannot find the rule match block in {RUST_CLI}")
    stop = re.search(r"\n([ \t]*)other => \{", text[start.end():])
    if stop is None:
        raise SpecError(f"cannot find the end of the rule match block in {RUST_CLI}")
    indent = stop.group(1)
    block = text[start.end():start.end() + stop.start()]
    rules = set()
    for line in block.splitlines():
        if not line.startswith(indent + '"'):
            continue
        arm = re.match(r'^\s*((?:"[^"]+"\s*\|?\s*)+)=>', line)
        if arm is not None:
            rules.update(re.findall(r'"([^"]+)"', arm.group(1)))
    if not rules:
        raise SpecError(f"cannot find rule match arms in {RUST_CLI}")
    return rules


CATALOG_ROW_RE = re.compile(r"^\| (?P<kind>[a-z][a-z0-9_]*)@(?P<schema>[0-9]+) \|", re.M)
CATALOG_MODULE_CLAIM_RE = re.compile(r"([0-9]+) 个 `kind@schema`")
CATALOG_KIND_CLAIM_RE = re.compile(r"(?<!其余 )([0-9]+) 个 `kind`")


def catalog_facts(root):
    """Return (row keys, claimed kind@schema counts, claimed kind counts)."""
    text = read_text(root, CATALOG)
    section = re.search(r"^## 1\. .*?$(.*?)(?=^## |\Z)", text, re.M | re.S)
    if section is None:
        raise SpecError(f"cannot find the registry table in {CATALOG}")
    rows = [
        f"{match.group('kind')}@{int(match.group('schema'))}"
        for match in CATALOG_ROW_RE.finditer(section.group(1))
    ]
    module_claims = {int(value) for value in CATALOG_MODULE_CLAIM_RE.findall(text)}
    kind_claims = {int(value) for value in CATALOG_KIND_CLAIM_RE.findall(text)}
    return rows, module_claims, kind_claims


def permission_problems(module, permissions):
    """Return problems with `permissions[]` and `actions[].permission`."""
    problems = []
    kind = module.get("kind")
    schema = module.get("schema")
    label = f"{kind}@{schema}"
    module_permissions = module.get("permissions", [])
    if not isinstance(module_permissions, list) or any(
        not isinstance(item, str) for item in module_permissions
    ):
        return [f"{label}: permissions must be a list of strings"]
    if len(set(module_permissions)) != len(module_permissions):
        problems.append(f"{label}: permissions repeat an entry")
    for permission in module_permissions:
        if permission not in permissions:
            problems.append(f"{label}: permission {permission!r} has no `Perm*` constant")

    actions = module.get("actions", [])
    if not isinstance(actions, list):
        return problems + [f"{label}: actions must be a list"]
    for action in actions:
        if not isinstance(action, dict):
            problems.append(f"{label}: actions[] entries must be objects")
            continue
        permission = action.get("permission")
        if not permission:
            problems.append(f"{label}: action {action.get('id')!r} has no permission")
        elif permission not in module_permissions:
            problems.append(
                f"{label}: action {action.get('id')!r} names undeclared permission {permission!r}"
            )
    return problems


def validate_module(module, sdk_version, port_types, categories, rules):
    """Return a list of contract problems for one manifest module record."""
    problems = []
    kind = module.get("kind")
    schema = module.get("schema")
    label = f"{kind}@{schema}"
    if not isinstance(kind, str) or not KIND_RE.match(kind):
        problems.append(f"{label}: kind must match {KIND_RE.pattern}")
    if not isinstance(schema, int) or isinstance(schema, bool) or schema <= 0:
        problems.append(f"{label}: schema must be a positive integer")
    if not isinstance(module.get("title"), str) or not module["title"].strip():
        problems.append(f"{label}: title must be a non-empty string")
    declared_sdk = module.get("sdk")
    if declared_sdk is not None and declared_sdk != sdk_version:
        problems.append(
            f"{label}: declares sdk {declared_sdk!r}, manifest sdkVersion is {sdk_version!r}"
        )
    category = module.get("category")
    if category is not None and category not in categories:
        problems.append(f"{label}: unknown category {category!r}")

    ports = []
    for field in ("inputs", "outputs"):
        entries = module.get(field, [])
        if not isinstance(entries, list):
            problems.append(f"{label}: {field} must be a list")
            continue
        for port in entries:
            if not isinstance(port, dict):
                problems.append(f"{label}: {field}[] entries must be objects")
                continue
            ports.append((field, port))
    for field, port in ports:
        name = port.get("name")
        if not isinstance(name, str) or not name.strip():
            problems.append(f"{label}: {field} port has no name")
        port_type = port.get("type")
        if port_type not in port_types:
            problems.append(f"{label}: port {name!r} has unknown type {port_type!r}")
        item_type = port.get("item_type")
        if item_type is not None and item_type not in port_types:
            problems.append(f"{label}: port {name!r} has unknown item_type {item_type!r}")
        if (port_type == "reference" or item_type == "reference") and not str(
            port.get("target") or ""
        ).strip():
            problems.append(f"{label}: reference port {name!r} has no target")
        if port.get("origin") == "derived" and not str(port.get("calculator") or "").strip():
            problems.append(f"{label}: derived port {name!r} has no calculator")

    rules_declared = module.get("rules", [])
    if not isinstance(rules_declared, list):
        problems.append(f"{label}: rules must be a list")
        rules_declared = []
    for rule in rules_declared:
        if rule not in rules:
            problems.append(f"{label}: rule {rule!r} is not registered in Go")

    lifecycle = module.get("lifecycle")
    if lifecycle is not None:
        states = lifecycle.get("states") if isinstance(lifecycle, dict) else None
        if not isinstance(states, list) or not states or any(
            not isinstance(state, str) or not state for state in states
        ):
            problems.append(f"{label}: lifecycle.states must be non-empty strings")
    provenance = module.get("provenance")
    if isinstance(provenance, dict) and provenance.get("origin") == "recorded":
        problems.append(f"{label}: a document module cannot store recorded facts")
    return problems
