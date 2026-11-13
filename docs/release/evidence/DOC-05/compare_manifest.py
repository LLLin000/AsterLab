#!/usr/bin/env python3
"""DOC-05: compare docs/sdk/module-catalog-v1.md §1 table with spec/modules.json."""
import json
import os
import re

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", "..", ".."))
MANIFEST = os.path.join(ROOT, "prototypes/mygo-run/internal/asterdoc/spec/modules.json")
CATALOG = os.path.join(ROOT, "docs/sdk/module-catalog-v1.md")


def main() -> int:
    modules = json.load(open(MANIFEST))["modules"]
    rows = {}
    for line in open(CATALOG):
        if not line.startswith("| ") or line.startswith("|---"):
            continue
        cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
        if len(cells) != 6 or "@" not in cells[0]:
            continue
        rows[cells[0]] = cells

    errors = []
    for mod in modules:
        key = f"{mod['kind']}@{mod['schema']}"
        cells = rows.get(key)
        if cells is None:
            errors.append(f"{key}: missing table row")
            continue
        renderers = mod.get("renderers", {})
        expected = [
            mod.get("category", ""),
            ", ".join(mod.get("capabilities", [])),
            ", ".join(mod.get("permissions", [])) or "(none)",
            " / ".join(renderers.get(k, "—") for k in ("design", "document", "run", "static")),
            ", ".join(mod.get("rules", [])) or "(none)",
        ]
        actual = cells[1:]
        actual[3] = re.sub(r"\s*\(.*?\)", "", actual[3]).strip()
        for index, (want, got) in enumerate(zip(expected, actual), start=1):
            if want != got:
                errors.append(f"{key} col{index}: expected {want!r} got {got!r}")

    extra = set(rows) - {f"{m['kind']}@{m['schema']}" for m in modules}
    print(f"manifest entries: {len(modules)} | table rows: {len(rows)} | extra rows: {sorted(extra)}")
    print(f"mismatches: {len(errors)}")
    for error in errors:
        print(" -", error)
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main())
