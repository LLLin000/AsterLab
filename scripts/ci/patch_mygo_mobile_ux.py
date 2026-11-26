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
            (
                "\t\tif s.longPress || s.drags() || s.flags&(flagContextMenu|flagEditable|flagSelectable) != 0 {",
                "\t\tif s.longPress || s.drags() || s.flags&(flagContextMenu|flagEditable|flagSelectable) != 0 || s.textClient != nil || s.role == RoleTextField {",
            ),
            (
                "\tcase s.flags&flagEditable != 0 || s.flags&flagSelectable != 0:\n"
                "\t\t// A word chosen, as a double click does.\n"
                "\t\trt.cancelPress()\n"
                "\t\t// As the second press of two on it, whatever it was pressed before.\n"
                "\t\trt.lastPress.prev, rt.lastPress.base = s.id, 0\n"
                "\t\trt.pointerDown(x, y, 0, rt.mods, 2)\n"
                "\t\trt.pointerUp(0, 2, false)\n"
                "\t\tt.swallow = true\n"
                "\t\trt.touchText.id = s.id\n"
                "\t\trt.showEditMenu(s)",
                "\tcase s.flags&flagEditable != 0 || s.flags&flagSelectable != 0 || s.textClient != nil || s.role == RoleTextField:\n"
                "\t\t// A word chosen, as a double click does.\n"
                "\t\trt.cancelPress()\n"
                "\t\t// As the second press of two on it, whatever it was pressed before.\n"
                "\t\trt.lastPress.prev, rt.lastPress.base = s.id, 0\n"
                "\t\trt.pointerDown(x, y, 0, rt.mods, 2)\n"
                "\t\trt.pointerUp(0, 2, false)\n"
                "\t\tt.swallow = true\n"
                "\t\trt.touchText.id = s.id\n"
                "\t\trt.showEditMenuAt(s, x, y)",
            ),
        ],
    )


def patch_touch_text_go(root: pathlib.Path) -> None:
    target = root / "ui" / "touch_text.go"
    apply_replacements(
        target,
        [
            (
                "func (rt *engine) showEditMenu(s *state) {\n"
                "\tvar actions []string\n"
                "\tfor _, name := range [...]string{\"cut\", \"copy\", \"paste\", \"selectAll\"} {\n"
                "\t\tif rt.commandEnabled(name) {\n"
                "\t\t\tactions = append(actions, name)\n"
                "\t\t}\n"
                "\t}\n"
                "\tif len(actions) == 0 {\n"
                "\t\treturn\n"
                "\t}\n"
                "\tvar x, y float64\n"
                "\tif _, a, b, _, ok := rt.handles(); ok {\n"
                "\t\tx, y = (a.X+b.X)/2, a.Y\n"
                "\t} else {\n"
                "\t\tr := transformRect(s.world, Rect{s.x, s.y, s.w, s.h})\n"
                "\t\tw := rt.toWindow(platform.RectF{X: float64(r.X + r.W/2), Y: float64(r.Y)})\n"
                "\t\tx, y = w.X, w.Y\n"
                "\t}\n"
                "\trt.touchSurface(func(ts platform.TouchSurface) { ts.ShowEditMenu(x, y, actions) })\n"
                "}",
                "func (rt *engine) showEditMenu(s *state) {\n"
                "\trt.showEditMenuAt(s, 0, 0)\n"
                "}\n\n"
                "func (rt *engine) showEditMenuAt(s *state, px, py float32) {\n"
                "\tif s == nil {\n"
                "\t\treturn\n"
                "\t}\n"
                "\tisReadOnly := (s.editor != nil && s.editor.readOnly) || (s.role == RoleTextField && s.textClient == nil)\n"
                "\tvar actions []string\n"
                "\tif isReadOnly {\n"
                "\t\tactions = []string{\"copy\", \"selectAll\"}\n"
                "\t} else {\n"
                "\t\tfor _, name := range [...]string{\"cut\", \"copy\", \"paste\", \"selectAll\"} {\n"
                "\t\t\tif rt.commandEnabled(name) {\n"
                "\t\t\t\tactions = append(actions, name)\n"
                "\t\t\t}\n"
                "\t\t}\n"
                "\t\tif s.role == RoleTextField {\n"
                "\t\t\tactions = append(actions, \"formatBold\", \"formatItalic\", \"formatCode\", \"formatFootnote\")\n"
                "\t\t}\n"
                "\t}\n"
                "\tif len(actions) == 0 {\n"
                "\t\treturn\n"
                "\t}\n"
                "\tvar x, y float64\n"
                "\tif _, a, b, _, ok := rt.handles(); ok {\n"
                "\t\tx, y = (a.X+b.X)/2, a.Y\n"
                "\t} else if px > 0 || py > 0 {\n"
                "\t\tx, y = rt.windowPoint(px, max(float32(0), py-16))\n"
                "\t} else {\n"
                "\t\tr := transformRect(s.world, Rect{s.x, s.y, s.w, s.h})\n"
                "\t\tw := rt.toWindow(platform.RectF{X: float64(r.X + r.W/2), Y: float64(r.Y)})\n"
                "\t\tx, y = w.X, w.Y\n"
                "\t}\n"
                "\trt.touchSurface(func(ts platform.TouchSurface) { ts.ShowEditMenu(x, y, actions) })\n"
                "}",
            ),
        ],
    )


def patch_editor_mobile(root: pathlib.Path) -> None:
    paint_go = root / "ui" / "editor" / "paint.go"
    apply_replacements(
        paint_go,
        [
            (
                "\ted.prepare(&f)\n"
                "\tif ed.focused {",
                "\ted.prepare(&f)\n"
                "\tif ed.focused && !ed.o.readOnly {",
            ),
        ],
    )
    build_go = root / "ui" / "editor" / "build.go"
    apply_replacements(
        build_go,
        [
            (
                "import (\n"
                "\t\"slices\"\n"
                "\t\"time\"\n"
                "\t\"unicode/utf8\"",
                "import (\n"
                "\t\"runtime\"\n"
                "\t\"slices\"\n"
                "\t\"time\"\n"
                "\t\"unicode/utf8\"",
            ),
            (
                "\te.Draw(ed.paint)\n"
                "\te.ContextMenu(ed.menu)",
                "\te.Draw(ed.paint)\n"
                "\tif runtime.GOOS != \"ios\" && runtime.GOOS != \"android\" {\n"
                "\t\te.ContextMenu(ed.menu)\n"
                "\t}",
            ),
        ],
    )


def patch_surface_window_swift(root: pathlib.Path) -> None:
    target = root / "internal" / "mobile" / "ios" / "Sources" / "MyGoHost" / "SurfaceWindow.swift"
    apply_replacements(
        target,
        [
            (
                "final class SurfaceWindow: HostWindow, UITextInput, UIPointerInteractionDelegate {",
                "final class SurfaceWindow: HostWindow, UITextInput, UIPointerInteractionDelegate, UIEditMenuInteractionDelegate {",
            ),
            (
                "    override var canBecomeFirstResponder: Bool { true }",
                "    private static let emptyInputView = UIView(frame: .zero)\n"
                "    override var canBecomeFirstResponder: Bool { true }\n"
                "    override var inputView: UIView? { editing ? nil : SurfaceWindow.emptyInputView }",
            ),
            (
                "        if editing && (!isFirstResponder || c.show != shows) {",
                "        if editing != wasEditing {\n"
                "            reloadInputViews()\n"
                "        }\n"
                "        if editing && (!wasEditing || another || c.show != shows) {",
            ),
            (
                "    private lazy var editMenu: UIEditMenuInteraction = {\n"
                "        let i = UIEditMenuInteraction(delegate: nil)\n"
                "        addInteraction(i)\n"
                "        return i\n"
                "    }()\n\n"
                "    func showMenu(_ c: ShowMenuCall) {\n"
                "        menuActions = c.actions\n"
                "        if !isFirstResponder { becomeFirstResponder() }\n"
                "        let cfg = UIEditMenuConfiguration(identifier: nil, sourcePoint: CGPoint(x: c.x, y: c.y))\n"
                "        cfg.preferredArrowDirection = .down\n"
                "        editMenu.presentEditMenu(with: cfg)\n"
                "    }",
                "    private lazy var editMenu: UIEditMenuInteraction = {\n"
                "        let i = UIEditMenuInteraction(delegate: self)\n"
                "        addInteraction(i)\n"
                "        return i\n"
                "    }()\n\n"
                "    func showMenu(_ c: ShowMenuCall) {\n"
                "        menuActions = c.actions\n"
                "        if !isFirstResponder { becomeFirstResponder() }\n"
                "        reloadInputViews()\n"
                "        let cfg = UIEditMenuConfiguration(identifier: nil, sourcePoint: CGPoint(x: c.x, y: c.y))\n"
                "        cfg.preferredArrowDirection = .down\n"
                "        editMenu.presentEditMenu(with: cfg)\n"
                "    }\n\n"
                "    func editMenuInteraction(\n"
                "        _ interaction: UIEditMenuInteraction,\n"
                "        menuFor configuration: UIEditMenuConfiguration,\n"
                "        suggestedActions: [UIMenuElement]\n"
                "    ) -> UIMenu? {\n"
                "        var items = suggestedActions\n"
                "        if menuActions.contains(\"formatBold\") {\n"
                "            let fmtMenu = UIMenu(\n"
                "                title: \"格式\",\n"
                "                image: UIImage(systemName: \"bold.italic.underline\"),\n"
                "                children: [\n"
                "                    UIAction(title: \"加粗\", image: UIImage(systemName: \"bold\")) { [weak self] _ in\n"
                "                        guard let self = self else { return }\n"
                "                        self.edit(Event.textCommand(id: self.id, command: \"formatBold\"))\n"
                "                    },\n"
                "                    UIAction(title: \"斜体\", image: UIImage(systemName: \"italic\")) { [weak self] _ in\n"
                "                        guard let self = self else { return }\n"
                "                        self.edit(Event.textCommand(id: self.id, command: \"formatItalic\"))\n"
                "                    },\n"
                "                    UIAction(title: \"代码\", image: UIImage(systemName: \"chevron.left.forwardslash.chevron.right\")) { [weak self] _ in\n"
                "                        guard let self = self else { return }\n"
                "                        self.edit(Event.textCommand(id: self.id, command: \"formatCode\"))\n"
                "                    },\n"
                "                    UIAction(title: \"批注\", image: UIImage(systemName: \"quote.bubble\")) { [weak self] _ in\n"
                "                        guard let self = self else { return }\n"
                "                        self.edit(Event.textCommand(id: self.id, command: \"formatFootnote\"))\n"
                "                    },\n"
                "                ]\n"
                "            )\n"
                "            items.append(fmtMenu)\n"
                "        }\n"
                "        return UIMenu(children: items)\n"
                "    }",
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
    patch_touch_text_go(root)
    patch_editor_mobile(root)
    patch_sheet_go(root)
    patch_surface_window_swift(root)
    if args.unsigned_ipa:
        patch_ios_unsigned(root)
    print(f"Patched MyGo mobile UX in {root} (unsigned_ipa={args.unsigned_ipa})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
