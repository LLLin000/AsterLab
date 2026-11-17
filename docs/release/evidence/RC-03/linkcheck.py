"""RC-03 relative-link checker: every relative doc/file reference in the six
Beta docs must resolve on disk. Covers markdown links [t](target) and
code-span paths like `evidence/RC-01/README.md` or `../../beta-scope.md`.
Run from the repo root:

    python docs/release/evidence/RC-03/linkcheck.py
"""
import re, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
DOCS = [
    "docs/release/README-BETA.md",
    "docs/release/START_HERE.md",
    "docs/release/BACKUP_RESTORE.md",
    "docs/release/CHANGELOG.md",
    "docs/release/TROUBLESHOOTING.md",
    "docs/release/evidence/RC-03/README.md",
]
LINK = re.compile(r"\[[^\]]*\]\(([^)#\s]+)(?:#[^)\s]*)?\)")
CODE = re.compile(
    r"`((?:\.\./)*(?:evidence/|beta-scope\.md|pdf-export\.md|MYGO_[A-Z_0-9]+\.md"
    r"|README-BETA\.md|START_HERE(?:\.bat|\.md)|BACKUP_RESTORE\.md"
    r"|CHANGELOG\.md|TROUBLESHOOTING\.md)[^`\s]*?)`"
)

def targets(text):
    for m in LINK.finditer(text):
        yield m.group(1)
    for m in CODE.finditer(text):
        yield m.group(1).rstrip(".,;:")

failures = []
checked = 0
for doc in DOCS:
    text = (ROOT / doc).read_text(encoding="utf-8")
    base = (ROOT / doc).parent
    for target in targets(text):
        if re.match(r"^[a-zA-Z][a-zA-Z0-9+.-]*:", target):
            continue  # absolute URI, out of scope
        if target.startswith("evidence/"):
            resolved = ROOT / "docs/release" / target
        else:
            resolved = base / target
        checked += 1
        if not resolved.exists():
            failures.append(f"{doc} -> {target} (missing)")

print(f"checked {checked} relative links in {len(DOCS)} files")
if failures:
    print("FAILURES:")
    for f in failures:
        print("  " + f)
    sys.exit(1)
print("all relative links resolve: OK")
