#!/usr/bin/env python3
"""Hash the MyGo generator's committed binding output, on every platform.

The pinned MyGo generator writes two files that Git tracks:

    prototypes/mygo-run/modules/runs/runs_native.go
    prototypes/mygo-run/modules/runs/rust/src/mygo.rs

They are generated from `modules/runs/runs.go` (`mygo:native` interfaces) by
`github.com/egoist/mygo`, whose version is pinned in three places: the
`require` line of `prototypes/mygo-run/go.mod`, `mygo_pin` in
`scripts/mygo-native-linux.sh`, and `$mygoPin` in `scripts/mygo-build.ps1`.
A Linux and a Windows checkout must produce the same bytes for the same input;
this script is how that is checked instead of assumed.

Drift sources and what the check does about each:

    line endings      canonicalised (CRLF and lone CR -> LF) before hashing and
                      reported as `drift: crlf`; a Windows `core.autocrlf`
                      checkout therefore cannot change the hash
    timestamps        rejected: an ISO date or a 10-digit epoch in generated
                      output fails the check
    path separators   rejected: a drive letter, a UNC path or an absolute
                      POSIX home/temp path in generated output fails the check
    map ordering      the generator sorts map keys before writing, and
                      `--regen N` runs it N times and requires byte-identical
                      canonical output every time

Hashes (all over canonical bytes, all hex SHA-256):

    input   sha256 of "mygo-generator-input-v1\\0<pin>\\0" followed by each
            input as "<repo path>\\0<sha256>\\n", sorted by path, then the
            language directories the module implements ("rust=1")
    output  sha256 of "mygo-generator-output-v1\\0" followed by each generated
            file as "<repo path>\\0<sha256>\\n", in GENERATED order
    file    sha256 of one file's canonical bytes

`scripts/ci/native_generator_hashes.json` records the expected values. Both CI
platforms compare against the same golden file, so equal hashes across
platforms is what the gate proves, not what this docstring claims.

Usage:
    python3 scripts/ci/native_generator_hash.py --check
    python3 scripts/ci/native_generator_hash.py --regen 2
    python3 scripts/ci/native_generator_hash.py --update-golden
    python3 scripts/ci/native_generator_hash.py --json /tmp/hashes.json

Exit codes: 0 the check passed; 1 drift, a pin mismatch or a golden mismatch;
2 the tree or a tool needed for the check is missing.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import platform
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

MODULE = "prototypes/mygo-run/modules/runs"
PROTO = "prototypes/mygo-run"
GO_MOD = f"{PROTO}/go.mod"
PIN_SOURCES = {
    "go.mod": GO_MOD,
    "linux script": "scripts/mygo-native-linux.sh",
    "windows script": "scripts/mygo-build.ps1",
}
# The generator's inputs, in hash order. Language directories that make the
# module buildable are part of the input: without rust/Cargo.toml the generator
# writes no Rust file.
INPUT_FILES = [f"{MODULE}/runs.go"]
LANG_MANIFESTS = {"rust": f"{MODULE}/rust/Cargo.toml"}
GENERATED = [
    f"{MODULE}/runs_native.go",
    f"{MODULE}/rust/src/mygo.rs",
]
GOLDEN = "scripts/ci/native_generator_hashes.json"
GENERATOR_MODULE = "github.com/egoist/mygo"
PREFIX = "native-generator-hash:"

ISO_DATE = re.compile(r"\b(?:19|20)\d{2}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])\b")
EPOCH = re.compile(r"\b(?:1[0-9]{9})\b")
DRIVE_PATH = re.compile(r"(?:^|[^A-Za-z0-9_])[A-Za-z]:[\\/]")
UNC_PATH = re.compile(r"\\\\[A-Za-z0-9_.$-]")
ABS_HOME = re.compile(r"(?:^|[\s\"'(=])/(?:home|Users|root|tmp|var)/")
HEADER = re.compile(r"^// mygo:native (\d+) ([0-9a-f]{16})$", re.MULTILINE)


class CheckError(Exception):
    """The tree cannot be checked as it stands."""


def repo_root() -> Path:
    return Path(__file__).resolve().parent.parent.parent


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def canonical_bytes(data: bytes) -> tuple[bytes, list[str]]:
    """Return the platform-neutral form of a generated file and its drift."""
    drift: list[str] = []
    if data.startswith(b"\xef\xbb\xbf"):
        data = data[3:]
        drift.append("utf8-bom")
    if b"\r\n" in data:
        data = data.replace(b"\r\n", b"\n")
        drift.append("crlf")
    if b"\r" in data:
        data = data.replace(b"\r", b"\n")
        drift.append("cr")
    return data, drift


def volatile_findings(text: str) -> list[str]:
    """Report generated content that cannot be identical on two platforms."""
    out: list[str] = []
    checks = [
        ("timestamp:iso-date", ISO_DATE),
        ("timestamp:epoch", EPOCH),
        ("path:drive-letter", DRIVE_PATH),
        ("path:unc", UNC_PATH),
        ("path:absolute", ABS_HOME),
    ]
    for kind, pattern in checks:
        for match in pattern.finditer(text):
            line = text.count("\n", 0, match.start()) + 1
            sample = text[match.start() : match.start() + 60].splitlines()[0]
            out.append(f"{kind} at line {line}: {sample!r}")
            break
    return out


def parse_headers(texts: dict[str, str]) -> tuple[int, str]:
    """Return the (format, interface hash) pair every generated file carries."""
    seen: dict[str, tuple[int, str]] = {}
    for path, text in texts.items():
        match = HEADER.search(text)
        if not match:
            raise CheckError(f"{path}: no '// mygo:native <format> <hash>' header")
        seen[path] = (int(match.group(1)), match.group(2))
    first_path, first = next(iter(seen.items()))
    for path, value in seen.items():
        if value != first:
            raise CheckError(
                f"generated files disagree on the interface hash: "
                f"{first_path} {first} vs {path} {value}"
            )
    return first


def pins_from_texts(texts: dict[str, str]) -> dict[str, str]:
    """Read the MyGo pin from every place that pins it."""
    patterns = {
        "go.mod": r"(?m)^\s*(?:require\s+)?" + re.escape(GENERATOR_MODULE) + r"\s+(v\S+)",
        "linux script": r"mygo_pin=\"([^\"]+)\"",
        "windows script": r"\$mygoPin\s*=\s*'([^']+)'",
    }
    pins: dict[str, str] = {}
    for name, path in PIN_SOURCES.items():
        match = re.search(patterns[name], texts.get(name, ""))
        if not match:
            raise CheckError(f"{path}: no MyGo version pin; the generator would float")
        pins[name] = match.group(1)
    if len(set(pins.values())) != 1:
        detail = ", ".join(f"{name}={pin}" for name, pin in pins.items())
        raise CheckError(f"the MyGo pin disagrees between files: {detail}")
    return pins


def pin_from_tree(root: Path) -> dict[str, str]:
    texts = {}
    for name, path in PIN_SOURCES.items():
        target = root / path
        if not target.is_file():
            raise CheckError(f"{path}: missing; cannot read the MyGo pin")
        texts[name] = target.read_text(encoding="utf-8")
    return pins_from_texts(texts)


def hash_input(root: Path, pin: str) -> str:
    parts = [f"mygo-generator-input-v1\0{pin}\0".encode()]
    for rel in sorted(INPUT_FILES):
        data, _ = canonical_bytes((root / rel).read_bytes())
        parts.append(f"{rel}\0{sha256(data)}\n".encode())
    for lang, rel in sorted(LANG_MANIFESTS.items()):
        present = "1" if (root / rel).is_file() else "0"
        parts.append(f"{lang}={present}\n".encode())
    return sha256(b"".join(parts))


def hash_generated(files: dict[str, bytes]) -> str:
    parts = [b"mygo-generator-output-v1\0"]
    for rel in GENERATED:
        parts.append(f"{rel}\0{sha256(files[rel])}\n".encode())
    return sha256(b"".join(parts))


def collect(root: Path) -> dict:
    """Read the committed tree and describe its generator hashes."""
    pins = pin_from_tree(root)
    pin = pins["go.mod"]
    for rel in INPUT_FILES + GENERATED:
        if not (root / rel).is_file():
            raise CheckError(f"{rel}: missing")

    files: dict[str, bytes] = {}
    drift: dict[str, list[str]] = {}
    texts: dict[str, str] = {}
    for rel in GENERATED:
        raw = (root / rel).read_bytes()
        data, found = canonical_bytes(raw)
        files[rel] = data
        drift[rel] = found
        try:
            texts[rel] = data.decode("utf-8")
        except UnicodeDecodeError as exc:
            raise CheckError(f"{rel}: generated file is not UTF-8: {exc}") from exc

    volatile: list[str] = []
    for rel, text in texts.items():
        volatile.extend(f"{rel}: {item}" for item in volatile_findings(text))

    fmt, iface = parse_headers(texts)
    return {
        "generator": {
            "module": GENERATOR_MODULE,
            "pin": pin,
            "pin_sources": sorted(PIN_SOURCES.values()),
            "format": fmt,
            "interface_hash": iface,
        },
        "input_sha256": hash_input(root, pin),
        "output_sha256": hash_generated(files),
        "files": {rel: sha256(files[rel]) for rel in GENERATED},
        "drift": {rel: found for rel, found in drift.items() if found},
        "volatile": volatile,
    }


def compare(expected: dict, actual: dict) -> list[str]:
    """Return one line per field that differs between golden and tree."""
    problems: list[str] = []
    for key in ("input_sha256", "output_sha256"):
        if expected.get(key) != actual.get(key):
            problems.append(f"{key}: golden {expected.get(key)} != tree {actual.get(key)}")
    want_files = expected.get("files") or {}
    for rel in sorted(set(want_files) | set(actual["files"])):
        want = want_files.get(rel)
        got = actual["files"].get(rel)
        if want != got:
            problems.append(f"{rel}: golden {want} != tree {got}")
    want_gen = expected.get("generator") or {}
    got_gen = actual["generator"]
    for key in ("module", "pin", "format", "interface_hash"):
        if want_gen.get(key) != got_gen.get(key):
            problems.append(f"generator.{key}: golden {want_gen.get(key)} != tree {got_gen.get(key)}")
    return problems


def load_golden(path: Path) -> dict:
    if not path.is_file():
        raise CheckError(
            f"{path}: missing; record the expected hashes with --update-golden in a reviewed commit"
        )
    return json.loads(path.read_text(encoding="utf-8"))


def display_path(path: Path, root: Path) -> str:
    try:
        return str(path.resolve().relative_to(root))
    except ValueError:
        return str(path)


def platform_line() -> str:
    return f"platform={platform.system().lower()} {platform.machine()} python={platform.python_version()}"


def commit_line(root: Path) -> str:
    try:
        out = subprocess.run(
            ["git", "-C", str(root), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return "commit=unknown"
    return f"commit={out.stdout.strip()}" if out.returncode == 0 else "commit=unknown"


def report_lines(report: dict) -> list[str]:
    gen = report["generator"]
    lines = [
        f"{PREFIX} {platform_line()}",
        f"{PREFIX} generator {gen['module']} {gen['pin']} "
        f"(pinned in {', '.join(gen['pin_sources'])})",
        f"{PREFIX} interface hash {gen['interface_hash']} format {gen['format']}",
        f"{PREFIX} input sha256 {report['input_sha256']}",
        f"{PREFIX} output sha256 {report['output_sha256']}",
    ]
    for rel, digest in report["files"].items():
        drift = ",".join(report["drift"].get(rel, [])) or "none"
        lines.append(f"{PREFIX}   {rel} sha256 {digest} drift {drift}")
    for item in report["volatile"]:
        lines.append(f"{PREFIX} volatile {item}")
    return lines


def write_golden(root: Path, golden_path: Path, report: dict) -> None:
    payload = {
        "generator": report["generator"],
        "input_sha256": report["input_sha256"],
        "output_sha256": report["output_sha256"],
        "files": report["files"],
        "note": (
            "Written by scripts/ci/native_generator_hash.py --update-golden. "
            "Linux and Windows CI must both reproduce these hashes; see "
            "docs/engineering/cross-platform-builds.md and "
            "docs/release/evidence/REL-05/."
        ),
    }
    golden_path.parent.mkdir(parents=True, exist_ok=True)
    golden_path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")


def temp_path(rel: str) -> Path:
    """Map a repository path below PROTOTYPES to its path in a temp copy."""
    prefix = f"{PROTO}/"
    if not rel.startswith(prefix):
        raise CheckError(f"{rel}: not below {prefix}")
    return Path(rel[len(prefix):])


def regen_once(root: Path, pin: str, index: int) -> dict:
    go = os.environ.get("GO", "go")
    if shutil.which(go) is None:
        raise CheckError("go: not found; --regen runs the pinned MyGo generator")
    tmp = Path(tempfile.mkdtemp(prefix=f"native-generator-{index}-"))
    try:
        (tmp / "modules/runs/rust/src").mkdir(parents=True)
        for rel in ("go.mod", "go.sum"):
            shutil.copy2(root / PROTO / rel, tmp / rel)
        for rel in INPUT_FILES + [LANG_MANIFESTS["rust"], f"{MODULE}/rust/src/lib.rs"]:
            shutil.copy2(root / rel, tmp / temp_path(rel))
        proc = subprocess.run(
            [go, "run", f"{GENERATOR_MODULE}/cmd/mygo@{pin}", "native", "check", "./modules/runs"],
            cwd=tmp,
            capture_output=True,
            text=True,
            timeout=1800,
        )
        files: dict[str, bytes] = {}
        for rel in GENERATED:
            target = tmp / temp_path(rel)
            if not target.is_file():
                raise CheckError(
                    f"regen {index}: the generator wrote no {rel}\n"
                    f"stdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
                )
            data, drift = canonical_bytes(target.read_bytes())
            if drift:
                raise CheckError(f"regen {index}: {rel} came back with drift {drift}")
            files[rel] = data
        texts = {rel: files[rel].decode("utf-8") for rel in GENERATED}
        fmt, iface = parse_headers(texts)
        # The generator builds the module after writing the files. The copy has
        # no Rust crate tree, so that build cannot succeed and is not what this
        # check hashes; the generated bytes are complete before it. Report the
        # build's last line so a failure of the check is readable.
        build_tail = proc.stdout.strip().splitlines()[-1:] or proc.stderr.strip().splitlines()[-1:]
        build_note = f"(post-generation build: {build_tail[0]})" if proc.returncode and build_tail else ""
        return {
            "index": index,
            "output_sha256": hash_generated(files),
            "files": {rel: sha256(files[rel]) for rel in GENERATED},
            "generator": {"format": fmt, "interface_hash": iface, "pin": pin},
            "note": build_note,
        }
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def run_regen(root: Path, runs: int, report: dict) -> list[str]:
    problems: list[str] = []
    pin = report["generator"]["pin"]
    results = [regen_once(root, pin, index) for index in range(1, runs + 1)]
    for result in results:
        suffix = f" {result['note']}" if result["note"] else ""
        print(f"{PREFIX} regen {result['index']} output sha256 {result['output_sha256']}{suffix}")
        for rel, digest in result["files"].items():
            print(f"{PREFIX}   regen {result['index']} {rel} sha256 {digest}")
        if result["generator"]["pin"] != pin:
            problems.append(
                f"regen {result['index']}: generator pin {result['generator']['pin']} != {pin}"
            )
        if result["output_sha256"] != report["output_sha256"]:
            problems.append(
                f"regen {result['index']}: output {result['output_sha256']} "
                f"!= committed {report['output_sha256']}"
            )
            for rel in GENERATED:
                if result["files"][rel] != report["files"][rel]:
                    problems.append(f"regen {result['index']}: {rel} differs from the committed file")
    if len({result["output_sha256"] for result in results}) > 1:
        problems.append(f"the generator is not deterministic across {runs} runs")
    return problems


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--root", default=None, help="repository root (default: this script's repo)")
    parser.add_argument("--golden", default=None, help=f"golden file (default: {GOLDEN})")
    parser.add_argument("--check", action="store_true", help="compare the tree with the golden file (default)")
    parser.add_argument("--update-golden", action="store_true", help="rewrite the golden file from this tree")
    parser.add_argument("--regen", type=int, default=0, metavar="N",
                        help="run the pinned generator N times and require identical output")
    parser.add_argument("--json", default=None, metavar="PATH", help="write the report as JSON")
    args = parser.parse_args(argv)

    root = Path(args.root).resolve() if args.root else repo_root()
    golden_path = Path(args.golden) if args.golden else root / GOLDEN

    try:
        report = collect(root)
    except CheckError as exc:
        print(f"{PREFIX} FAILED {exc}", file=sys.stderr)
        return 2

    print(f"{PREFIX} {commit_line(root)}")
    for line in report_lines(report):
        print(line)

    if report["volatile"]:
        print(
            f"{PREFIX} FAILED generated output carries values that cannot be identical on "
            f"two platforms; make the generator deterministic instead of hashing around it",
            file=sys.stderr,
        )
        return 1

    if args.update_golden:
        write_golden(root, golden_path, report)
        print(f"{PREFIX} wrote {display_path(golden_path, root)}")
        return 0

    problems: list[str] = []
    try:
        problems.extend(compare(load_golden(golden_path), report))
    except CheckError as exc:
        print(f"{PREFIX} FAILED {exc}", file=sys.stderr)
        return 2
    if problems:
        for problem in problems:
            print(f"{PREFIX} drift {problem}", file=sys.stderr)
        print(
            f"{PREFIX} FAILED the committed generator output no longer matches "
            f"{display_path(golden_path, root)}",
            file=sys.stderr,
        )
    else:
        print(f"{PREFIX} OK golden {display_path(golden_path, root)}")

    if args.regen:
        regen_problems = run_regen(root, args.regen, report)
        for problem in regen_problems:
            print(f"{PREFIX} drift {problem}", file=sys.stderr)
        if regen_problems:
            print(f"{PREFIX} FAILED the pinned generator is not reproducible", file=sys.stderr)
        else:
            print(
                f"{PREFIX} OK {args.regen} regeneration(s) identical, "
                f"generator {report['generator']['pin']}"
            )
        problems.extend(regen_problems)

    if args.json:
        payload = {
            "commit": commit_line(root).split("=", 1)[-1],
            "platform": platform_line(),
            "report": report,
            "problems": problems,
        }
        Path(args.json).write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8", newline="\n")
        print(f"{PREFIX} wrote {args.json}")

    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())
