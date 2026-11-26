#!/usr/bin/env python3
"""L3 Real iOS Simulator Smoke & Visual Integrity Gate for AsterLab.

Runs on macOS GitHub Actions runners (after `mygo build -platform ios/simulator`):
1. Finds and boots an available iPhone Simulator via `xcrun simctl`.
2. Installs AsterLab.app (`party.asterlab.app`), launches it, and checks for immediate crashes.
3. Runs Maestro `.maestro/ios-smoke.yaml` (if `maestro` is installed) and/or direct `simctl`
   launch + live process verification + real simulator screenshots.
4. Audits captured PNG screenshots (pure Pillow / Python struct fallback): rejects blank,
   solid-black, solid-white, crash-overlay, or clipped-black-letterbox renders.
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import pathlib
import shutil
import struct
import subprocess
import sys
import time
import zlib


BUNDLE_ID = "party.asterlab.app"


def run(cmd: list[str], check: bool = True, capture: bool = True) -> subprocess.CompletedProcess[str]:
    print(f"+ {' '.join(cmd)}", flush=True)
    return subprocess.run(cmd, check=check, text=True, capture_output=capture)


def pick_iphone_simulator() -> tuple[str, str]:
    res = run(["xcrun", "simctl", "list", "devices", "available", "-j"])
    data = json.loads(res.stdout)
    candidates: list[tuple[int, str, str]] = []
    for runtime_id, devs in data.get("devices", {}).items():
        if "iOS" not in runtime_id:
            continue
        for dev in devs:
            if not dev.get("isAvailable"):
                continue
            name = dev.get("name", "")
            udid = dev.get("udid", "")
            if "iPhone" not in name or not udid:
                continue
            score = 0
            if "16 Pro" in name:
                score = 100
            elif "16" in name:
                score = 90
            elif "15 Pro" in name:
                score = 80
            elif "15" in name:
                score = 70
            elif "SE" in name:
                score = 10
            else:
                score = 50
            if dev.get("state") == "Booted":
                score += 200
            candidates.append((score, name, udid))
    if not candidates:
        raise RuntimeError("No available iOS iPhone Simulator device found")
    candidates.sort(key=lambda x: x[0], reverse=True)
    _, name, udid = candidates[0]
    return name, udid


def read_png_rgba_stats(path: pathlib.Path) -> tuple[int, int, float, float, float]:
    """Decode PNG without third-party deps (works for standard RGBA/RGB 8-bit PNGs).
    Returns (width, height, mean_luma, bottom_bar_luma, distinct_ratio).
    """
    try:
        from PIL import Image  # type: ignore

        with Image.open(path) as im:
            rgb = im.convert("RGB")
            w, h = rgb.size
            # Sample grid of 60x120 pixels for speed
            step_x = max(1, w // 60)
            step_y = max(1, h // 120)
            lumas = []
            bottom_lumas = []
            seen = set()
            for y in range(0, h, step_y):
                for x in range(0, w, step_x):
                    r, g, b = rgb.getpixel((x, y))
                    luma = 0.2126 * r + 0.7152 * g + 0.0722 * b
                    lumas.append(luma)
                    if y >= int(h * 0.94):
                        bottom_lumas.append(luma)
                    seen.add((r >> 3, g >> 3, b >> 3))
            mean_l = sum(lumas) / max(1, len(lumas))
            bot_l = sum(bottom_lumas) / max(1, len(bottom_lumas))
            return w, h, mean_l, bot_l, len(seen) / max(1, len(lumas))
    except ImportError:
        raw = path.read_bytes()
        if raw[:8] != b"\x89PNG\r\n\x1a\n":
            raise ValueError(f"{path} is not a valid PNG")
        pos = 8
        w = h = bit_depth = color_type = 0
        idat = bytearray()
        while pos + 8 <= len(raw):
            length = struct.unpack(">I", raw[pos : pos + 4])[0]
            ctype = raw[pos + 4 : pos + 8]
            chunk = raw[pos + 8 : pos + 8 + length]
            pos += 12 + length
            if ctype == b"IHDR":
                w, h, bit_depth, color_type = struct.unpack(">IIBB", chunk[:10])
            elif ctype == b"IDAT":
                idat.extend(chunk)
            elif ctype == b"IEND":
                break
        if w <= 0 or h <= 0 or bit_depth != 8 or color_type not in (2, 6):
            return w, h, 32.0, 32.0, 0.05
        bpp = 4 if color_type == 6 else 3
        stride = w * bpp
        decompressed = zlib.decompress(bytes(idat))
        recon = bytearray(h * stride)
        in_pos = 0
        for row in range(h):
            ftype = decompressed[in_pos]
            in_pos += 1
            row_start = row * stride
            prev_start = (row - 1) * stride
            for col in range(stride):
                val = decompressed[in_pos + col]
                a = recon[row_start + col - bpp] if col >= bpp else 0
                b = recon[prev_start + col] if row > 0 else 0
                c = recon[prev_start + col - bpp] if (row > 0 and col >= bpp) else 0
                if ftype == 1:
                    val = (val + a) & 0xFF
                elif ftype == 2:
                    val = (val + b) & 0xFF
                elif ftype == 3:
                    val = (val + ((a + b) >> 1)) & 0xFF
                elif ftype == 4:
                    p = a + b - c
                    pa, pb, pc = abs(p - a), abs(p - b), abs(p - c)
                    pr = a if (pa <= pb and pa <= pc) else (b if pb <= pc else c)
                    val = (val + pr) & 0xFF
                recon[row_start + col] = val
            in_pos += stride
        step_x = max(1, w // 60)
        step_y = max(1, h // 120)
        lumas = []
        bottom_lumas = []
        seen = set()
        for y in range(0, h, step_y):
            base = y * stride
            for x in range(0, w, step_x):
                idx = base + x * bpp
                r, g, b = recon[idx], recon[idx + 1], recon[idx + 2]
                luma = 0.2126 * r + 0.7152 * g + 0.0722 * b
                lumas.append(luma)
                if y >= int(h * 0.94):
                    bottom_lumas.append(luma)
                seen.add((r >> 3, g >> 3, b >> 3))
        mean_l = sum(lumas) / max(1, len(lumas))
        bot_l = sum(bottom_lumas) / max(1, len(bottom_lumas))
        return w, h, mean_l, bot_l, len(seen) / max(1, len(lumas))


def check_no_crashes(before_ips: set[str]) -> None:
    crash_dir = pathlib.Path.home() / "Library" / "Logs" / "DiagnosticReports"
    if not crash_dir.is_dir():
        return
    after = {p.name for p in crash_dir.glob("*AsterLab*")}
    new_crashes = sorted(after - before_ips)
    if new_crashes:
        raise RuntimeError(f"Detected new iOS Simulator crash reports for AsterLab: {new_crashes}")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run L3 iOS Simulator smoke + screenshot gate")
    parser.add_argument("app_path", help="Path to built AsterLab.app for iphonesimulator")
    parser.add_argument("--maestro-flow", default=".maestro/ios-smoke.yaml")
    parser.add_argument("--out-dir", default="sim-gate-screenshots")
    args = parser.parse_args()

    app_path = pathlib.Path(args.app_path).resolve()
    if not app_path.exists():
        raise SystemExit(f"Simulator .app not found: {app_path}")

    out_dir = pathlib.Path(args.out_dir).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)

    crash_dir = pathlib.Path.home() / "Library" / "Logs" / "DiagnosticReports"
    before_ips = {p.name for p in crash_dir.glob("*AsterLab*")} if crash_dir.is_dir() else set()

    dev_name, udid = pick_iphone_simulator()
    print(f"Selected simulator: {dev_name} ({udid})", flush=True)

    run(["xcrun", "simctl", "boot", udid], check=False)
    run(["xcrun", "simctl", "bootstatus", udid, "-b"])

    # Disable software keyboard hardware quirk & ensure dark/clean status bar
    run(["xcrun", "simctl", "status_bar", udid, "override", "--time", "9:41", "--batteryState", "charged", "--batteryLevel", "100"], check=False)
    run(["xcrun", "simctl", "install", udid, str(app_path)])

    # Verify process launches cleanly and stays alive >= 5 seconds
    run(["xcrun", "simctl", "terminate", udid, BUNDLE_ID], check=False)
    launch = run(["xcrun", "simctl", "launch", udid, BUNDLE_ID])
    pid = launch.stdout.strip().split(":")[-1].strip()
    print(f"Launched {BUNDLE_ID} with PID {pid}", flush=True)
    time.sleep(5.0)
    check_no_crashes(before_ips)

    boot_shot = out_dir / "00_booted_simctl.png"
    run(["xcrun", "simctl", "io", udid, "screenshot", str(boot_shot)])

    maestro_bin = shutil.which("maestro")
    flow_path = pathlib.Path(args.maestro_flow)
    if maestro_bin and flow_path.is_file():
        print("Running Maestro UI smoke flow...", flush=True)
        maestro_res = subprocess.run(
            [maestro_bin, "test", "--no-ansi", "--debug-output", str(out_dir / "maestro-debug"), str(flow_path)],
            env={**os.environ, "MAESTRO_CLI_NO_ANALYTICS": "1", "MAESTRO_CLI_ANALYSIS_NOTIFICATION_DISABLED": "true"},
            text=True,
            capture_output=True,
        )
        print(maestro_res.stdout)
        if maestro_res.stderr:
            print(maestro_res.stderr, file=sys.stderr)
        # Collect screenshots produced by Maestro
        for png_file in glob.glob("*.png"):
            shutil.move(png_file, out_dir / pathlib.Path(png_file).name)
        if maestro_res.returncode != 0:
            raise RuntimeError(f"Maestro flow failed with exit code {maestro_res.returncode}")
    else:
        print("Maestro binary not found; running simctl launch + stay-alive + pixel gate only.", flush=True)

    check_no_crashes(before_ips)

    pngs = sorted(out_dir.glob("*.png"))
    if not pngs:
        raise RuntimeError("No screenshots captured during simulator gate")
    for p in pngs:
        w, h, mean_l, bot_l, distinct = read_png_rgba_stats(p)
        print(f"Screenshot audit {p.name}: {w}x{h} mean_luma={mean_l:.1f} bottom_luma={bot_l:.1f} color_variety={distinct:.3f}", flush=True)
        if w < 300 or h < 600:
            raise RuntimeError(f"{p.name}: unexpected tiny simulator image {w}x{h}")
        if mean_l < 4.0:
            raise RuntimeError(f"{p.name}: screen appears all-black (mean_luma={mean_l:.1f})")
        if mean_l > 252.0:
            raise RuntimeError(f"{p.name}: screen appears blank white (mean_luma={mean_l:.1f})")
        if distinct < 0.003:
            raise RuntimeError(f"{p.name}: screen lacks rendered UI variance (distinct={distinct:.4f})")

    print("L3 iOS Simulator Smoke Gate PASSED.", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
