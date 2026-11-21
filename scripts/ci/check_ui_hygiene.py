#!/usr/bin/env python3
"""CI UI & Design-System Hygiene Gate for AsterLab Pure-Go desktop."""

from __future__ import annotations

import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
PROTO = ROOT / "prototypes" / "mygo-run"

TARGET_DIRS = [
    PROTO / "internal" / "shell",
    PROTO / "internal" / "asterdoc",
    PROTO / "internal" / "tools",
]
TARGET_FILES = [
    PROTO / "experiment_page.go",
    PROTO / "vault_host.go",
    PROTO / "update_host.go",
]

LITERAL_COLOR_RE = re.compile(r"\bui\.(Hex|RGBA)\(")
TINY_REM_RE = re.compile(r"\bt\.Rem\(\s*0?\.(?:[0-7]\d*|8[0-4])\s*\)")


def iter_go_files():
    for d in TARGET_DIRS:
        if d.is_dir():
            for p in sorted(d.glob("*.go")):
                if not p.name.endswith("_test.go"):
                    yield p
    for p in TARGET_FILES:
        if p.is_file():
            yield p


def main() -> int:
    violations: list[str] = []
    for path in iter_go_files():
        rel = path.relative_to(ROOT).as_posix()
        for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            stripped = line.strip()
            if stripped.startswith("//"):
                continue
            if m := LITERAL_COLOR_RE.search(line):
                violations.append(f"{rel}:{lineno}: literal color {m.group(0)} outside internal/theme")
            if m := TINY_REM_RE.search(line):
                violations.append(f"{rel}:{lineno}: sub-12px {m.group(0)}; use theme Tokens Text* instead")
    if violations:
        print("UI hygiene check failed:", file=sys.stderr)
        for v in violations:
            print("  " + v, file=sys.stderr)
        return 1
    print("ui hygiene check: ok (0 literal colors or tiny Rem fonts in Vault/Shell/AsterDoc)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
