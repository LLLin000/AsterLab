#!/usr/bin/env python3
"""REL-04: enforce a Kandev card's `kandev-scope` block against a PR diff.

The block is machine-readable YAML inside the task description:

    <!-- kandev-scope
    write:
      - path/or/dir/
    read:
      - path/to/reference
    forbid:
      - path/glob/**
    -->

Semantics
---------
* Every entry is a path or a shell-style glob. `*`, `?` and `[` are glob
  characters and `*` also crosses `/` (so `crates/**` covers every file below
  `crates/`). A trailing `/`, or a plain path, also covers that path's
  children.
* `forbid` wins over `write`, `write` wins over `read`, and a path matched by
  nothing is out of scope.
* `read` entries are references: they may never appear in the diff.
* Entries must be paths. Free text such as `all source files` is rejected
  instead of silently matching nothing.

Exit codes: 0 = in scope, 1 = scope violation, 2 = unusable input.

Usage examples are in scripts/ci/README.md.
"""

from __future__ import annotations

import argparse
import fnmatch
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.request

SCOPE_MARKER_OPEN = "<!-- kandev-scope"
SCOPE_MARKER_CLOSE = "-->"
TASK_MARKER_RE = re.compile(r"<!--\s*kandev-task:\s*([0-9a-fA-F-]{36})\s*-->")
SECTION_RE = re.compile(r"^(write|read|forbid)\s*:\s*$")
EMPTY_SECTION_RE = re.compile(r"^(write|read|forbid)\s*:\s*\[\s*\]\s*$")
ENTRY_RE = re.compile(r"^-\s+(.*)$")
SECTIONS = ("write", "read", "forbid")


class ScopeError(Exception):
    """Input that cannot be enforced; always exit code 2."""


class Scope:
    def __init__(self, write, read, forbid):
        self.write = list(write)
        self.read = list(read)
        self.forbid = list(forbid)

    def __repr__(self):
        return (
            f"Scope(write={self.write!r}, read={self.read!r}, "
            f"forbid={self.forbid!r})"
        )


def extract_scope_block(text):
    """Return the raw YAML body of the first `kandev-scope` block."""
    start = text.find(SCOPE_MARKER_OPEN)
    if start < 0:
        raise ScopeError(
            "no `<!-- kandev-scope` block in the task description"
        )
    body_start = start + len(SCOPE_MARKER_OPEN)
    end = text.find(SCOPE_MARKER_CLOSE, body_start)
    if end < 0:
        raise ScopeError("`kandev-scope` block is not closed with `-->`")
    if text.find(SCOPE_MARKER_OPEN, end) >= 0:
        print(
            "scope-check: warning: multiple kandev-scope blocks; using the first",
            file=sys.stderr,
        )
    return text[body_start:end]


def _normalize_entry(entry, line_no):
    value = entry.strip()
    if not value:
        raise ScopeError(f"line {line_no}: empty scope entry")
    if any(ch.isspace() for ch in value):
        raise ScopeError(
            f"line {line_no}: scope entry {value!r} contains whitespace; "
            "the block may only list paths and globs"
        )
    if value.startswith("/") or value.startswith("../"):
        raise ScopeError(
            f"line {line_no}: scope entry {value!r} must be repository-relative"
        )
    while value.startswith("./"):
        value = value[2:]
    if value in ("", "."):
        raise ScopeError(
            f"line {line_no}: scope entry {entry.strip()!r} is not a path"
        )
    return value


def parse_scope(block):
    """Parse the YAML body into a Scope; unknown or sloppy syntax is an error."""
    sections = {name: [] for name in SECTIONS}
    seen = set()
    current = None
    for line_no, raw in enumerate(block.splitlines(), start=2):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        empty = EMPTY_SECTION_RE.match(line)
        if empty:
            name = empty.group(1)
            if name in seen:
                raise ScopeError(f"line {line_no}: duplicate `{name}:` section")
            seen.add(name)
            current = None
            continue
        header = SECTION_RE.match(line)
        if header:
            name = header.group(1)
            if name in seen:
                raise ScopeError(f"line {line_no}: duplicate `{name}:` section")
            seen.add(name)
            current = name
            continue
        entry = ENTRY_RE.match(line)
        if entry and current is not None:
            sections[current].append(_normalize_entry(entry.group(1), line_no))
            continue
        raise ScopeError(
            f"line {line_no}: cannot parse {line!r}; expected "
            "`write:`, `read:`, `forbid:` or `- <path>`"
        )
    if not seen:
        raise ScopeError("`kandev-scope` block declares no write/read/forbid")
    if not sections["write"]:
        raise ScopeError("`kandev-scope` block has an empty `write` list")
    return Scope(sections["write"], sections["read"], sections["forbid"])


def matches(entry, path):
    """True when a repo-relative `path` is covered by one scope `entry`."""
    if entry.endswith("/"):
        prefix = entry.rstrip("/")
        return path == prefix or path.startswith(prefix + "/")
    if any(ch in entry for ch in "*?["):
        return fnmatch.fnmatchcase(path, entry)
    return path == entry or path.startswith(entry + "/")


def classify(path, scope):
    """Return (verdict, pattern) where verdict is forbid|write|read|unlisted."""
    for pattern in scope.forbid:
        if matches(pattern, path):
            return "forbid", pattern
    for pattern in scope.write:
        if matches(pattern, path):
            return "write", pattern
    for pattern in scope.read:
        if matches(pattern, path):
            return "read", pattern
    return "unlisted", None


def parse_name_status_text(text):
    """Parse `git diff --name-status` output; renames yield both paths."""
    changes = []
    for line_no, line in enumerate(text.splitlines(), start=1):
        if not line.strip():
            continue
        fields = line.split("\t")
        status = fields[0].strip()
        if not status:
            raise ScopeError(f"diff line {line_no}: empty status field")
        if status[0] in ("R", "C"):
            if len(fields) < 3:
                raise ScopeError(
                    f"diff line {line_no}: {status} needs source and target path"
                )
            changes.append((status, fields[1]))
            changes.append((status, fields[2]))
        else:
            if len(fields) < 2:
                raise ScopeError(f"diff line {line_no}: missing path")
            changes.append((status, fields[1]))
    return changes


def parse_name_status_z(payload):
    """Parse NUL-separated `git diff --name-status -z` output."""
    fields = payload.split("\0")
    if fields and fields[-1] == "":
        fields.pop()
    changes = []
    index = 0
    while index < len(fields):
        status = fields[index]
        index += 1
        if not status:
            raise ScopeError("`git diff --name-status -z` contained an empty status")
        if status[0] in ("R", "C"):
            if index + 1 >= len(fields):
                raise ScopeError(f"truncated {status} entry in git diff output")
            source, target = fields[index], fields[index + 1]
            index += 2
            if status[0] == "R":
                changes.append((status, source))
            changes.append((status, target))
        else:
            if index >= len(fields):
                raise ScopeError(f"truncated {status} entry in git diff output")
            changes.append((status, fields[index]))
            index += 1
    return changes


def git_changes(repo, base, head):
    command = [
        "git",
        "-C",
        repo,
        "diff",
        "--name-status",
        "--find-renames",
        "--no-color",
        "-z",
        f"{base}...{head}",
    ]
    result = subprocess.run(command, capture_output=True, text=True)
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip()
        raise ScopeError(f"{' '.join(command)} failed: {detail}")
    return parse_name_status_z(result.stdout)


def fetch_task_description(task_id, base_url, token, timeout=20):
    endpoint = f"{base_url.rstrip('/')}/api/v1/tasks/{task_id}"
    request = urllib.request.Request(
        endpoint,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload = json.load(response)
    except (urllib.error.URLError, OSError, ValueError) as exc:
        raise ScopeError(f"cannot read Kandev task {task_id} from {endpoint}: {exc}")
    description = payload.get("description") if isinstance(payload, dict) else None
    if not isinstance(description, str):
        task = payload.get("task") if isinstance(payload, dict) else None
        description = task.get("description") if isinstance(task, dict) else None
    if not isinstance(description, str):
        raise ScopeError(
            f"Kandev task {task_id} response has no string `description` field"
        )
    return description


def _read(path, what):
    try:
        with open(path, encoding="utf-8") as handle:
            return handle.read()
    except OSError as exc:
        raise ScopeError(f"cannot read {what} {path}: {exc}")


def resolve_description(args):
    """Return (description, source label) following the documented precedence."""
    api_url = args.kandev_url or os.environ.get("KANDEV_API_URL", "")
    api_token = args.kandev_token or os.environ.get("KANDEV_API_TOKEN", "")

    if args.task_id:
        if not api_url:
            raise ScopeError(
                "--task-id needs a Kandev base URL: pass --kandev-url or set "
                "KANDEV_API_URL"
            )
        if not api_token:
            raise ScopeError(
                "--task-id needs a Kandev token: pass --kandev-token or set "
                "KANDEV_API_TOKEN"
            )
        return (
            fetch_task_description(args.task_id, api_url, api_token),
            f"Kandev API task {args.task_id}",
        )
    if args.pr_body_file:
        body = _read(args.pr_body_file, "PR body")
        marker = TASK_MARKER_RE.search(body)
        if marker and api_url and api_token:
            task_id = marker.group(1)
            return (
                fetch_task_description(task_id, api_url, api_token),
                f"Kandev API task {task_id}",
            )
        if marker:
            print(
                "scope-check: notice: KANDEV_API_URL/KANDEV_API_TOKEN are not "
                "configured; reading the scope block from the PR body instead",
                file=sys.stderr,
            )
        else:
            print(
                "scope-check: notice: PR body has no `<!-- kandev-task: <uuid> -->` "
                "marker; reading the scope block from the PR body",
                file=sys.stderr,
            )
        return body, "PR body"
    if args.description_file:
        return _read(args.description_file, "description"), args.description_file
    if args.description_stdin:
        return sys.stdin.read(), "stdin"
    raise ScopeError(
        "no task description source: pass --task-id, --pr-body-file, "
        "--description-file or --description-stdin"
    )


def load_changes(args):
    if args.diff_file:
        return (
            parse_name_status_text(_read(args.diff_file, "diff")),
            f"diff file {args.diff_file}",
        )
    return git_changes(args.repo, args.base, args.head), (
        f"git diff --name-status {args.base}...{args.head}"
    )


def evaluate(changes, scope):
    """Return a list of (status, path, verdict, pattern) for changed paths."""
    return [(status, path) + classify(path, scope) for status, path in changes]


def report(results, source, diff_source):
    violations = [row for row in results if row[2] not in ("write",)]
    print(f"scope-check: scope source = {source}")
    print(f"scope-check: diff source  = {diff_source}")
    for status, path, verdict, pattern in results:
        if verdict == "write":
            print(f"  ok    {status:>4}  {path}  <- {pattern}")
        elif verdict == "forbid":
            print(f"  BLOCK {status:>4}  {path}  <- forbid {pattern}")
        elif verdict == "read":
            print(f"  BLOCK {status:>4}  {path}  <- read-only {pattern}")
        else:
            print(f"  BLOCK {status:>4}  {path}  <- not listed in write")
    print(
        f"scope-check: {len(results)} changed path(s), "
        f"{len(results) - len(violations)} allowed, {len(violations)} violation(s)"
    )
    if not violations:
        return 0
    print("scope-check: FAILED - the pull request touches paths outside the card scope")
    for _status, path, verdict, pattern in violations:
        if verdict == "forbid":
            reason = f"forbidden by `{pattern}`"
        elif verdict == "read":
            reason = f"read-only entry `{pattern}` must not be modified"
        else:
            reason = "not listed in the `write` list"
        print(f"::error file={path}::scope-check: {reason}")
        print(f"  out of scope: {path} ({reason})")
    return 1


def build_parser():
    parser = argparse.ArgumentParser(
        description="Enforce the Kandev `kandev-scope` block on a PR diff.",
    )
    parser.add_argument("--base", default="origin/main", help="base ref (default: origin/main)")
    parser.add_argument("--head", default="HEAD", help="head ref (default: HEAD)")
    parser.add_argument("--repo", default=".", help="repository root (default: .)")
    parser.add_argument(
        "--diff-file",
        help="read `git diff --name-status` output from a file instead of running git",
    )
    parser.add_argument("--task-id", help="Kandev task id; read the description from the API")
    parser.add_argument("--pr-body-file", help="PR body; used when --task-id is absent")
    parser.add_argument("--description-file", help="file holding the task description")
    parser.add_argument(
        "--description-stdin",
        action="store_true",
        help="read the task description from stdin",
    )
    parser.add_argument("--kandev-url", help="Kandev base URL (default: $KANDEV_API_URL)")
    parser.add_argument(
        "--kandev-token", help="Kandev API token (default: $KANDEV_API_TOKEN)"
    )
    return parser


def main(argv=None):
    args = build_parser().parse_args(argv)
    try:
        description, source = resolve_description(args)
        scope = parse_scope(extract_scope_block(description))
        changes, diff_source = load_changes(args)
    except ScopeError as exc:
        print(f"scope-check: error: {exc}", file=sys.stderr)
        return 2
    return report(evaluate(changes, scope), source, diff_source)


if __name__ == "__main__":
    sys.exit(main())
