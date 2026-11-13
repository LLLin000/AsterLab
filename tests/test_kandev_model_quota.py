#!/usr/bin/env python3
"""Tests for the one-way model quota fallback."""

import importlib.util
import json
import pathlib
import sqlite3
import unittest
from datetime import datetime, timezone

SCRIPT = pathlib.Path(__file__).resolve().parents[1] / "scripts" / "kandev_model_quota.py"
SPEC = importlib.util.spec_from_file_location("kandev_model_quota", SCRIPT)
rotator = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(rotator)


def fixture_db(profiles, sessions, messages):
    con = sqlite3.connect(":memory:")
    con.execute("CREATE TABLE workflow_steps (id TEXT PRIMARY KEY, agent_profile_id TEXT)")
    con.execute(
        "CREATE TABLE task_sessions "
        "(id TEXT PRIMARY KEY, state TEXT, agent_profile_snapshot TEXT)"
    )
    con.execute(
        "CREATE TABLE task_session_messages "
        "(task_session_id TEXT, content TEXT, created_at TEXT)"
    )
    for name, step_id in rotator.STEPS.items():
        con.execute(
            "INSERT INTO workflow_steps VALUES (?, ?)",
            (step_id, profiles[name]),
        )
    for session_id, state, model in sessions:
        con.execute(
            "INSERT INTO task_sessions VALUES (?, ?, ?)",
            (session_id, state, json.dumps({"model": model})),
        )
    con.executemany(
        "INSERT INTO task_session_messages VALUES (?, ?, ?)",
        messages,
    )
    return con


class QuotaFallbackTest(unittest.TestCase):
    def setUp(self):
        self.now = datetime(2026, 11, 13, 13, 30, tzinfo=timezone.utc)

    def test_active_deepseek_weekly_limit_is_detected(self):
        messages = [(
            rotator.DS_MODEL,
            "429 OpenCode Go usage limit reached (weekly, resets in 2d 10h).",
            "2026-11-13T13:00:00+00:00",
        )]
        self.assertTrue(rotator.ds_quota_exhausted(messages, self.now))

    def test_expired_deepseek_limit_does_not_trigger_fallback(self):
        messages = [(
            rotator.DS_MODEL,
            "429 OpenCode Go usage limit reached (weekly, resets in 1h).",
            "2026-11-13T10:00:00+00:00",
        )]
        self.assertFalse(rotator.ds_quota_exhausted(messages, self.now))

    def test_luna_quota_error_never_counts_as_deepseek_exhaustion(self):
        messages = [(
            rotator.LUNA_MODEL,
            "429 OpenCode Go usage limit reached (weekly, resets in 2d 10h).",
            "2026-11-13T13:00:00+00:00",
        )]
        self.assertFalse(rotator.ds_quota_exhausted(messages, self.now))

    def test_non_quota_error_does_not_trigger_fallback(self):
        messages = [(
            rotator.DS_MODEL,
            "429 temporary upstream error",
            "2026-11-13T13:00:00+00:00",
        )]
        self.assertFalse(rotator.ds_quota_exhausted(messages, self.now))

    def test_deepseek_exhaustion_selects_luna(self):
        self.assertEqual(
            rotator.fallback_target(rotator.DS, ds_quota_exhausted=True),
            rotator.LUNA,
        )

    def test_luna_remains_selected_when_luna_is_exhausted(self):
        self.assertEqual(
            rotator.fallback_target(rotator.LUNA, ds_quota_exhausted=False),
            rotator.LUNA,
        )

    def test_without_deepseek_quota_error_current_profile_is_preserved(self):
        self.assertEqual(
            rotator.fallback_target(rotator.DS, ds_quota_exhausted=False),
            rotator.DS,
        )

    def test_active_ds_quota_updates_steps_and_active_ds_sessions(self):
        quota_error = "429 OpenCode Go usage limit reached (weekly, resets in 2d 10h)."
        con = fixture_db(
            {"Agent": rotator.DS, "Review": rotator.DS},
            [
                ("ds-active", "WAITING_FOR_INPUT", rotator.DS_MODEL),
                ("luna-active", "WAITING_FOR_INPUT", rotator.LUNA_MODEL),
                ("ds-cancelled", "CANCELLED", rotator.DS_MODEL),
            ],
            [
                ("ds-active", quota_error, "2026-11-13T13:00:00+00:00"),
                ("luna-active", quota_error, "2026-11-13T13:00:00+00:00"),
            ],
        )
        changed_sessions = []
        result = rotator.reconcile(
            con,
            self.now,
            lambda session_id: changed_sessions.append(session_id) or True,
        )
        profiles = dict(
            con.execute("SELECT id, agent_profile_id FROM workflow_steps").fetchall()
        )
        self.assertEqual(result, (2, 1))
        self.assertEqual(set(profiles.values()), {rotator.LUNA})
        self.assertEqual(changed_sessions, ["ds-active"])

    def test_luna_quota_keeps_luna_profile_and_never_selects_ds(self):
        quota_error = "429 OpenCode Go usage limit reached (weekly, resets in 2d 10h)."
        con = fixture_db(
            {"Agent": rotator.LUNA, "Review": rotator.LUNA},
            [("luna-active", "WAITING_FOR_INPUT", rotator.LUNA_MODEL)],
            [("luna-active", quota_error, "2026-11-13T13:00:00+00:00")],
        )
        changed_sessions = []
        result = rotator.reconcile(
            con,
            self.now,
            lambda session_id: changed_sessions.append(session_id) or True,
        )
        profiles = dict(
            con.execute("SELECT id, agent_profile_id FROM workflow_steps").fetchall()
        )
        self.assertEqual(result, (0, 0))
        self.assertEqual(set(profiles.values()), {rotator.LUNA})
        self.assertEqual(changed_sessions, [])


if __name__ == "__main__":
    unittest.main()
