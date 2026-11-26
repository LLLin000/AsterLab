#!/usr/bin/env python3
"""Deterministically patch a writable copy of github.com/egoist/mygo@v0.10.0
for responsive mobile touch scrolling, keyboard dismissal hygiene, and optional
unsigned arm64 .ipa packaging.
"""

import argparse
import pathlib
import sys


def apply_replacements(path: pathlib.Path, repls: list[tuple[str, str]]) -> None:
    src = path.read_text(encoding="utf-8")
    for old, new in repls:
        if old not in src:
            raise AssertionError(f"{path}: missing expected snippet:\n{old}")
        if src.count(old) != 1:
            raise AssertionError(f"{path}: expected 1 occurrence, found {src.count(old)} for:\n{old}")
        src = src.replace(old, new, 1)
    path.write_text(src, encoding="utf-8")


def patch_touch_go(root: pathlib.Path) -> None:
    target = root / "ui" / "touch.go"
    apply_replacements(
        target,
        [
            (
                "// flingStop is the speed, in DIPs a second, under which a fling stops.\n"
                "const flingStop = 20",
                "// flingStop is the speed, in DIPs a second, under which a fling stops.\n"
                "const flingStop = 45",
            ),
            (
                "\t// swallow drops what the finger does: it went down to stop a fling,\n"
                "\t// which takes no tap.\n"
                "\tswallow bool",
                "\t// swallow drops everything until the finger lifts; swallowTap drops\n"
                "\t// only an accidental tap when interrupting a fling, so chained swipes scroll immediately.\n"
                "\tswallow    bool\n"
                "\tswallowTap bool",
            ),
            (
                "\t\tif rt.flinging.on {\n"
                "\t\t\t// A finger stops content flinging, and taps nothing.\n"
                "\t\t\trt.flinging.on = false\n"
                "\t\t\tt.swallow = true\n"
                "\t\t\treturn true\n"
                "\t\t}",
                "\t\tif rt.flinging.on {\n"
                "\t\t\t// A finger stops content flinging, suppressing tap while allowing chained swipes.\n"
                "\t\t\trt.flinging.on = false\n"
                "\t\t\tt.swallowTap = true\n"
                "\t\t\treturn true\n"
                "\t\t}",
            ),
            (
                "\t\t\tt.panning = true\n"
                "\t\t\tif t.waiting {\n"
                "\t\t\t\tt.waiting = false // it never pressed\n"
                "\t\t\t} else {\n"
                "\t\t\t\trt.cancelPress()\n"
                "\t\t\t}\n"
                "\t\t\tt.target, t.axes = rt.panTarget(t.x0, t.y0, axis)",
                "\t\t\tt.panning = true\n"
                "\t\t\tt.swallowTap = false\n"
                "\t\t\tif t.waiting {\n"
                "\t\t\t\tt.waiting = false // it never pressed\n"
                "\t\t\t} else {\n"
                "\t\t\t\trt.cancelPress()\n"
                "\t\t\t}\n"
                "\t\t\tt.target, t.axes = rt.panTarget(t.x0, t.y0, axis)\n"
                "\t\t\t// Interactive scroll-to-dismiss keyboard: scrolling blurs any focused text input.\n"
                "\t\t\tif t.axes&Vertical != 0 && rt.focused != 0 && rt.focused != t.target {\n"
                "\t\t\t\tif fs := rt.states[rt.focused]; fs != nil && fs.textClient != nil {\n"
                "\t\t\t\t\trt.focused = 0\n"
                "\t\t\t\t\trt.touchText.id = 0\n"
                "\t\t\t\t\trt.requestFrame()\n"
                "\t\t\t\t}\n"
                "\t\t\t}",
            ),
            (
                "\t\tif was.swallow {\n"
                "\t\t\treturn true\n"
                "\t\t}",
                "\t\tif was.swallow || (was.swallowTap && !was.panning) {\n"
                "\t\t\treturn true\n"
                "\t\t}",
            ),
        ],
    )


def patch_surface_window_swift(root: pathlib.Path) -> None:
    target = root / "internal" / "mobile" / "ios" / "Sources" / "MyGoHost" / "SurfaceWindow.swift"
    apply_replacements(
        target,
        [
            (
                "        if editing && (!isFirstResponder || c.show != shows) {",
                "        if editing && (!wasEditing || another || c.show != shows) {",
            ),
        ],
    )


def patch_sheet_go(root: pathlib.Path) -> None:
    target = root / "ui" / "mobile" / "sheet.go"
    apply_replacements(
        target,
        [
            (
                "\t\tback := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Modal()",
                "\t\tback := ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0)\n"
                "\t\tif *open {\n"
                "\t\t\tback.Modal()\n"
                "\t\t} else {\n"
                "\t\t\tback.PassThrough()\n"
                "\t\t}",
            ),
            (
                "\t\tif !*open && !born && shown.Y >= closedY-0.5 {",
                "\t\tif !*open && !born && shown.Y >= closedY-2.0 {",
            ),
        ],
    )


def patch_ios_unsigned(root: pathlib.Path) -> None:
    target = root / "cmd" / "mygo" / "ios.go"
    apply_replacements(
        target,
        [
            (
                "\tvar prof *profile\n\tif !simulator {",
                "\tunsigned := !simulator && os.Getenv(\"MYGO_IOS_UNSIGNED\") == \"1\"\n"
                "\tvar prof *profile\n\tif !simulator && !unsigned {",
            ),
            (
                "\t\tif simulator {\n\t\t\terr = signForSimulator(bundle)",
                "\t\tif simulator || unsigned {\n\t\t\terr = signForSimulator(bundle)",
            ),
            (
                "\t\tartifact = filepath.Join(stage, c.slug()+\"-\"+c.Version+\".ipa\")",
                "\t\tsuffix := \"\"\n\t\tif unsigned {\n\t\t\tsuffix = \"-unsigned\"\n\t\t}\n"
                "\t\tartifact = filepath.Join(stage, c.slug()+\"-\"+c.Version+suffix+\".ipa\")",
            ),
        ],
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Patch MyGo v0.10.0 source for mobile UX fixes.")
    parser.add_argument("mygo_dir", help="Path to writable copy of github.com/egoist/mygo source tree")
    parser.add_argument("--unsigned-ipa", action="store_true", help="Also enable MYGO_IOS_UNSIGNED=1 arm64 .ipa builds")
    args = parser.parse_args()

    root = pathlib.Path(args.mygo_dir).resolve()
    if not (root / "go.mod").is_file():
        raise SystemExit(f"Not a Go module root: {root}")

    patch_touch_go(root)
    patch_sheet_go(root)
    patch_surface_window_swift(root)
    if args.unsigned_ipa:
        patch_ios_unsigned(root)
    print(f"Patched MyGo mobile UX in {root} (unsigned_ipa={args.unsigned_ipa})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
