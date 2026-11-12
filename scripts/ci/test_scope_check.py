#!/usr/bin/env python3
"""Fixture tests for scripts/ci/scope_check.py.

Run by the `scope-check` workflow and locally:

    python3 scripts/ci/test_scope_check.py
"""

from __future__ import annotations

import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

HERE = pathlib.Path(__file__).resolve().parent
SCRIPT = HERE / "scope_check.py"
FIXTURES = HERE / "fixtures"

sys.path.insert(0, str(HERE))
import scope_check  # noqa: E402  (path bootstrap must precede the import)


def run_script(*arguments):
    return subprocess.run(
        [sys.executable, str(SCRIPT), *arguments],
        capture_output=True,
        text=True,
        cwd=str(HERE),
    )


class ScopeMatchingTest(unittest.TestCase):
    def test_directory_entry_covers_children(self):
        self.assertTrue(scope_check.matches(".github/workflows/", ".github/workflows/ci.yml"))
        self.assertFalse(scope_check.matches(".github/workflows/", ".github/CODEOWNERS"))

    def test_plain_path_covers_children_not_siblings(self):
        self.assertTrue(scope_check.matches("docs/release", "docs/release/notes.md"))
        self.assertFalse(scope_check.matches("docs/release", "docs/released/notes.md"))

    def test_glob_crosses_directories(self):
        self.assertTrue(scope_check.matches("crates/**", "crates/a/src/lib.rs"))
        self.assertFalse(scope_check.matches("crates/**", "src/lib.rs"))

    def test_forbid_wins_over_write(self):
        scope = scope_check.Scope(write=["docs/"], read=[], forbid=["docs/contracts/**"])
        self.assertEqual(("write", "docs/"), scope_check.classify("docs/README.md", scope))
        self.assertEqual(
            ("forbid", "docs/contracts/**"),
            scope_check.classify("docs/contracts/core-v1.md", scope),
        )

    def test_read_entry_is_not_a_write(self):
        scope = scope_check.Scope(write=["src/"], read=["Cargo.toml"], forbid=[])
        self.assertEqual(("read", "Cargo.toml"), scope_check.classify("Cargo.toml", scope))


class DescriptionParsingTest(unittest.TestCase):
    def test_block_is_extracted_from_surrounding_markdown(self):
        text = (FIXTURES / "out-of-scope" / "description.md").read_text(encoding="utf-8")
        scope = scope_check.parse_scope(scope_check.extract_scope_block(text))
        self.assertEqual(["crates/"], scope.write)
        self.assertEqual(["README.md"], scope.read)
        self.assertEqual(["docs/contracts/**"], scope.forbid)

    def test_duplicate_section_is_rejected(self):
        with self.assertRaises(scope_check.ScopeError):
            scope_check.parse_scope("\nwrite:\n  - a/\nwrite:\n  - b/\n")

    def test_empty_write_list_is_rejected(self):
        with self.assertRaises(scope_check.ScopeError):
            scope_check.parse_scope("\nwrite: []\nread:\n  - a\nforbid: []\n")


class CommandLineTest(unittest.TestCase):
    def test_out_of_scope_fixture_exits_nonzero(self):
        result = run_script(
            "--description-file",
            str(FIXTURES / "out-of-scope" / "description.md"),
            "--diff-file",
            str(FIXTURES / "out-of-scope" / "diff.txt"),
        )
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn("4 violation(s)", result.stdout)
        self.assertIn("out of scope: src/cli.rs (not listed in the `write` list)", result.stdout)
        self.assertIn("forbidden by `docs/contracts/**`", result.stdout)
        self.assertIn("read-only entry `README.md` must not be modified", result.stdout)
        self.assertIn(
            "::error file=docs/contracts/core-v1.md::", result.stdout
        )
        self.assertIn("crates/asterlab-core/src/lib.rs", result.stdout)

    def test_in_scope_fixture_passes(self):
        result = run_script(
            "--description-file",
            str(FIXTURES / "in-scope" / "description.md"),
            "--diff-file",
            str(FIXTURES / "in-scope" / "diff.txt"),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("2 changed path(s), 2 allowed, 0 violation(s)", result.stdout)

    def test_rename_checks_source_and_target_paths(self):
        result = run_script(
            "--description-file",
            str(FIXTURES / "rename" / "description.md"),
            "--diff-file",
            str(FIXTURES / "rename" / "diff.txt"),
        )
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertIn("out of scope: crates/moved.rs (not listed in the `write` list)", result.stdout)
        self.assertNotIn("out of scope: src/old.rs", result.stdout)

    def test_missing_scope_block_is_a_config_error(self):
        result = run_script(
            "--description-file",
            str(FIXTURES / "invalid" / "missing-block.md"),
            "--diff-file",
            str(FIXTURES / "in-scope" / "diff.txt"),
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("no `<!-- kandev-scope` block", result.stderr)

    def test_free_text_entry_is_rejected(self):
        result = run_script(
            "--description-file",
            str(FIXTURES / "invalid" / "free-text-entry.md"),
            "--diff-file",
            str(FIXTURES / "in-scope" / "diff.txt"),
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("contains whitespace", result.stderr)

    def test_pr_body_marker_falls_back_to_the_body_without_api(self):
        result = run_script(
            "--pr-body-file",
            str(FIXTURES / "pr-body" / "pr-body.md"),
            "--diff-file",
            str(FIXTURES / "in-scope" / "diff.txt"),
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("scope source = PR body", result.stdout)
        self.assertIn("KANDEV_API_URL/KANDEV_API_TOKEN are not configured", result.stderr)

    def test_no_description_source_is_a_config_error(self):
        result = run_script("--diff-file", str(FIXTURES / "in-scope" / "diff.txt"))
        self.assertEqual(result.returncode, 2)
        self.assertIn("no task description source", result.stderr)

    def test_task_id_without_credentials_is_a_config_error(self):
        env = {key: "" for key in ("KANDEV_API_URL", "KANDEV_API_TOKEN")}
        result = subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--task-id",
                "4fd7bbeb-e7e8-49a8-8323-2766f1d2f645",
                "--diff-file",
                str(FIXTURES / "in-scope" / "diff.txt"),
            ],
            capture_output=True,
            text=True,
            cwd=str(HERE),
            env={**os.environ, **env},
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("--task-id needs a Kandev base URL", result.stderr)

    def test_truncated_diff_file_is_a_config_error(self):
        with tempfile.NamedTemporaryFile("w", suffix=".txt", delete=False) as handle:
            handle.write("R100\tonly-one-path\n")
            path = handle.name
        try:
            result = run_script(
                "--description-file",
                str(FIXTURES / "in-scope" / "description.md"),
                "--diff-file",
                path,
            )
        finally:
            pathlib.Path(path).unlink()
        self.assertEqual(result.returncode, 2)
        self.assertIn("needs source and target path", result.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
