#!/usr/bin/env python3
"""CI UI & Design-System Hygiene Gate for AsterLab Pure-Go desktop & mobile."""

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
    PROTO / "internal" / "uikit" / "kit",
    PROTO / "internal" / "storybook",
    PROTO / "cmd" / "storybook",
]
TARGET_FILES = [
    PROTO / "experiment_page.go",
    PROTO / "vault_host.go",
    PROTO / "update_host.go",
]

LEGACY_DIRS = [
    PROTO / "internal" / "protocol",
    PROTO / "internal" / "runworkspace",
    PROTO / "internal" / "runui",
    PROTO / "internal" / "resultgrid",
    PROTO / "internal" / "designer",
    PROTO / "internal" / "blockui",
    PROTO / "internal" / "blocks",
    PROTO / "internal" / "uikit" / "plate",
]
LEGACY_FILES = [
    PROTO / "main.go",
    PROTO / "design_page.go",
    PROTO / "design_canvas.go",
    PROTO / "notebook_page.go",
]

# Legacy debt ratchet: counts may only stay equal or decrease over time.
LEGACY_MAX_BASELINE = {
    "FontSize": 64,
    "Padding": 70,
    "Radius": 64,
}

LITERAL_COLOR_RE = re.compile(r"\bui\.(Hex|RGBA)\(")
TINY_REM_RE = re.compile(r"\bt\.Rem\(\s*0?\.(?:[0-7]\d*|8[0-4])\s*\)")
RAW_CTX_MENU_RE = re.compile(r"\bcomponents\.ContextMenu\(")
LITERAL_FONT_RE = re.compile(r"\bFontSize\(\s*[0-9]")
LITERAL_RADIUS_RE = re.compile(r"\bRadius\(\s*[0-9]")
PADDING_CALL_RE = re.compile(r"\bPadding\(([^)]+)\)")
NUMERIC_ARG_RE = re.compile(r"^[0-9]+(?:\.[0-9]+)?$")


def iter_go_files(dirs: list[pathlib.Path], files: list[pathlib.Path]):
    for d in dirs:
        if d.is_dir():
            for p in sorted(d.glob("*.go")):
                if not p.name.endswith("_test.go"):
                    yield p
    for p in files:
        if p.is_file():
            yield p


def has_nonzero_literal_padding(line: str) -> str | None:
    for m in PADDING_CALL_RE.finditer(line):
        args = [a.strip() for a in m.group(1).split(",")]
        if any(NUMERIC_ARG_RE.match(a) and float(a) != 0 for a in args):
            return m.group(0)
    return None


def main() -> int:
    violations: list[str] = []
    for path in iter_go_files(TARGET_DIRS, TARGET_FILES):
        rel = path.relative_to(ROOT).as_posix()
        for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            stripped = line.strip()
            if stripped.startswith("//"):
                continue
            if m := LITERAL_COLOR_RE.search(line):
                violations.append(f"{rel}:{lineno}: literal color {m.group(0)} outside internal/theme")
            if m := TINY_REM_RE.search(line):
                violations.append(f"{rel}:{lineno}: sub-12px {m.group(0)}; use theme Tokens Text* instead")
            if RAW_CTX_MENU_RE.search(line):
                violations.append(f"{rel}:{lineno}: raw components.ContextMenu(...) banned; use internal/uikit/kit.ContextMenu")
            if LITERAL_FONT_RE.search(line):
                violations.append(f"{rel}:{lineno}: literal FontSize(...) banned; use theme Tokens Text* or t.Rem(...)")
            if pad := has_nonzero_literal_padding(line):
                violations.append(f"{rel}:{lineno}: literal {pad} banned; use tok.Spacing* or t.Space(...)")
            if LITERAL_RADIUS_RE.search(line):
                violations.append(f"{rel}:{lineno}: literal Radius(...) banned; use tok.ControlRadius/PanelRadius/DialogRadius/PillRadius")

    legacy_counts = {"FontSize": 0, "Padding": 0, "Radius": 0}
    for path in iter_go_files(LEGACY_DIRS, LEGACY_FILES):
        for line in path.read_text(encoding="utf-8").splitlines():
            if line.strip().startswith("//"):
                continue
            if LITERAL_FONT_RE.search(line):
                legacy_counts["FontSize"] += 1
            if has_nonzero_literal_padding(line):
                legacy_counts["Padding"] += 1
            if LITERAL_RADIUS_RE.search(line):
                legacy_counts["Radius"] += 1

    for kind, count in legacy_counts.items():
        cap = LEGACY_MAX_BASELINE[kind]
        if count > cap:
            violations.append(
                f"legacy ratchet exceeded for {kind}: got {count} > baseline {cap} (do not add new literals to legacy packages)"
            )

    if violations:
        print("UI hygiene check failed:", file=sys.stderr)
        for v in violations:
            print("  " + v, file=sys.stderr)
        return 1

    print(
        "ui hygiene check: ok (0 violations in active UI packages; "
        f"legacy ratchet FontSize={legacy_counts['FontSize']}/{LEGACY_MAX_BASELINE['FontSize']}, "
        f"Padding={legacy_counts['Padding']}/{LEGACY_MAX_BASELINE['Padding']}, "
        f"Radius={legacy_counts['Radius']}/{LEGACY_MAX_BASELINE['Radius']})"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
