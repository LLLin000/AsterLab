#!/usr/bin/env python3
"""Regression tests for Kandev automation authorization and status integration."""

import importlib.util
import json
import os
import pathlib
import sqlite3
import subprocess
import tempfile
import unittest
from unittest import mock

import sys
ROOT = pathlib.Path(__file__).resolve().parents[1]


def load_script(name, path):
    spec = importlib.util.spec_from_file_location(name, ROOT / "scripts" / path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


sys.path.insert(0, str(ROOT / "scripts"))
feeder = load_script("kandev_ready_feeder_test", "kandev-ready-feeder.py")
callback = load_script("kandev_telegram_callback_test", "kandev-telegram-callback.py")


class RetroWatermarkTest(unittest.TestCase):
    def test_binding_and_watermark_are_persisted_on_success(self):
        with tempfile.TemporaryDirectory() as tmp:
            db_path = pathlib.Path(tmp) / "kandev.db"
            with sqlite3.connect(db_path) as con:
                con.execute("CREATE TABLE task_repositories (id TEXT, task_id TEXT, repository_id TEXT, workspace_relative_path TEXT, base_branch TEXT, checkout_branch TEXT, position INTEGER, metadata TEXT, created_at TEXT, updated_at TEXT)")
                con.execute("CREATE TABLE kandev_meta (key TEXT PRIMARY KEY, value TEXT)")

            feeder.bind_retro_and_persist(str(db_path), "retro-1", ["task-1"], "2026-01-02T03:04:05+00:00")

            with sqlite3.connect(db_path) as con:
                self.assertEqual(con.execute("SELECT task_id FROM task_repositories").fetchone(), ("retro-1",))
                values = dict(con.execute("SELECT key, value FROM kandev_meta"))
            self.assertEqual(values["retro.vps.last_run_at"], "2026-01-02T03:04:05+00:00")
            self.assertEqual(json.loads(values["retro.vps.last_task_ids"]), ["task-1"])

    def test_failed_binding_does_not_advance_watermark(self):
        with tempfile.TemporaryDirectory() as tmp:
            db_path = pathlib.Path(tmp) / "kandev.db"
            with sqlite3.connect(db_path) as con:
                con.execute("CREATE TABLE task_repositories (id TEXT, task_id TEXT, repository_id TEXT, workspace_relative_path TEXT, base_branch TEXT, checkout_branch TEXT, position INTEGER, metadata TEXT, created_at TEXT, updated_at TEXT)")
                con.execute("CREATE TABLE kandev_meta (key TEXT PRIMARY KEY, value TEXT)")
                con.execute("INSERT INTO kandev_meta VALUES ('retro.vps.last_run_at', 'old')")
                con.execute("DROP TABLE task_repositories")

            with self.assertRaises(sqlite3.OperationalError):
                feeder.bind_retro_and_persist(str(db_path), "retro-1", ["task-1"], "new")

            with sqlite3.connect(db_path) as con:
                self.assertEqual(con.execute("SELECT value FROM kandev_meta WHERE key='retro.vps.last_run_at'").fetchone(), ("old",))
                self.assertIsNone(con.execute("SELECT value FROM kandev_meta WHERE key='retro.vps.last_task_ids'").fetchone())


class CallbackAuthorizationTest(unittest.TestCase):
    def test_configured_user_is_allowed_and_other_user_is_rejected(self):
        with mock.patch.dict(os.environ, {"TELEGRAM_ALLOWED_USER_IDS": "123,456"}, clear=True):
            allowed = callback.configured_allowed_user_ids()
        self.assertTrue(callback.callback_user_allowed("123", allowed))
        self.assertFalse(callback.callback_user_allowed("789", allowed))

    def test_unset_allowlist_defaults_to_configured_chat(self):
        with mock.patch.dict(os.environ, {"TELEGRAM_CHAT_ID": "123"}, clear=True):
            self.assertEqual(callback.configured_allowed_user_ids(), {"123"})


class HomeStatusWrapperTest(unittest.TestCase):
    def test_status_invocation_uses_home_paths_and_required_command(self):
        script = ROOT / "scripts" / "kandev-home-status.sh"
        with tempfile.TemporaryDirectory() as tmp:
            bin_dir = pathlib.Path(tmp) / "bin"
            bin_dir.mkdir()
            capture = pathlib.Path(tmp) / "invocation.json"
            fake_python = bin_dir / "python3"
            fake_python.write_text(
                "#!/usr/bin/python3\n"
                "import json, os, sys\n"
                f"open({str(capture)!r}, 'w').write(json.dumps({{'argv': sys.argv[1:], 'env': {{k: os.environ.get(k) for k in ('ENGINEERING_HOME', 'REPOS_ROOT', 'PYTHONPATH', 'KANDEV_API_TOKEN')}}}}))\n"
            )
            fake_python.chmod(0o755)
            env = os.environ.copy()
            env.update({
                "PATH": f"{bin_dir}:{env.get('PATH', '')}",
                "ENGINEERING_HOME": "/tmp/home",
                "REPOS_ROOT": "/tmp/repos",
                "KANDEV_API_TOKEN": "test-token",
            })
            subprocess.run(["bash", str(script)], env=env, check=True, capture_output=True, text=True)
            invocation = json.loads(capture.read_text())
            self.assertEqual(invocation["argv"], ["-m", "engctl", "status", "--project", "asterlab"])
            self.assertEqual(invocation["env"], {
                "ENGINEERING_HOME": "/tmp/home",
                "REPOS_ROOT": "/tmp/repos",
                "PYTHONPATH": "/tmp/home/src",
                "KANDEV_API_TOKEN": "test-token",
            })

    def test_missing_token_fails_clearly(self):
        script = ROOT / "scripts" / "kandev-home-status.sh"
        env = os.environ.copy()
        env.pop("KANDEV_API_TOKEN", None)
        result = subprocess.run(["bash", str(script)], env=env, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("KANDEV_API_TOKEN", result.stderr)


if __name__ == "__main__":
    unittest.main()
