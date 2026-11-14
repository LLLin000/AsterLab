#!/usr/bin/env python3
"""Regression tests for the feeder's lane membership and Inbox admission rules.

Two production stalls motivate these tests:
- DES-00 sat in Ready without a `lane:vps` label and was invisible to the
  feeder while Ready forbids manual moves.
- QA-07 was bounced back to Inbox and kept state TODO, so the old
  `state == "CREATED"` gate never promoted it.
"""

import importlib.util
import pathlib
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

spec = importlib.util.spec_from_file_location(
    "kandev_ready_feeder_admission", ROOT / "scripts" / "kandev-ready-feeder.py"
)
feeder = importlib.util.module_from_spec(spec)
spec.loader.exec_module(feeder)


class LaneMembershipTest(unittest.TestCase):
    def test_lane_membership_follows_workflow_not_label(self):
        tasks = [
            {"id": "no-label", "workflow_id": feeder.WORKFLOW, "labels": []},
            {"id": "labelled", "workflow_id": feeder.WORKFLOW, "labels": ["lane:vps"]},
            {"id": "other-workflow", "workflow_id": "other", "labels": ["lane:vps"]},
        ]
        self.assertEqual(
            [t["id"] for t in feeder.lane_tasks(tasks)], ["no-label", "labelled"]
        )


class InboxAdmissionTest(unittest.TestCase):
    def test_admits_created_and_todo_but_not_blocked_or_foreign_steps(self):
        def card(cid, state, blocked=None, step=feeder.INBOX):
            return {
                "id": cid,
                "workflow_step_id": step,
                "state": state,
                "blocked": blocked,
            }

        detailed = [
            card("created", "CREATED"),
            card("todo", "TODO"),
            card("blocked", "CREATED", blocked=True),
            card("review", "REVIEW"),
            card("ready", "CREATED", step=feeder.READY),
        ]
        self.assertEqual(
            [t["id"] for t in feeder.inbox_candidates(detailed)],
            ["created", "todo"],
        )


class MergedPrCycleGuardTest(unittest.TestCase):
    def test_merge_before_the_current_cycle_is_not_accepted(self):
        cycle = "2026-11-14 01:21:20.121452+00:00"
        self.assertFalse(
            feeder.merged_in_current_cycle("2026-11-13T18:22:14Z", cycle)
        )

    def test_merge_inside_the_current_cycle_is_accepted(self):
        cycle = "2026-11-14 01:21:20.121452+00:00"
        self.assertTrue(
            feeder.merged_in_current_cycle("2026-11-14T02:00:00Z", cycle)
        )

    def test_missing_timestamps_keep_the_previous_behaviour(self):
        self.assertTrue(feeder.merged_in_current_cycle(None, "2026-11-14 01:21:20+00:00"))
        self.assertTrue(feeder.merged_in_current_cycle("2026-11-13T18:22:14Z", None))


class SessionLivenessTest(unittest.TestCase):
    START = 1_000_000.0

    @staticmethod
    def iso(ts):
        from datetime import datetime, timezone

        return datetime.fromtimestamp(ts, timezone.utc).isoformat()

    def test_process_must_match_the_session_worktree(self):
        procs = {101: ("/var/lib/kandev/tasks/a", self.START)}
        self.assertTrue(
            feeder._session_process_live(procs, "/var/lib/kandev/tasks/a", self.iso(self.START + 60))
        )
        self.assertFalse(
            feeder._session_process_live(procs, "/var/lib/kandev/tasks/b", self.iso(self.START + 60))
        )

    def test_start_time_must_be_within_the_match_window(self):
        procs = {101: ("/var/lib/kandev/tasks/a", self.START)}
        self.assertFalse(
            feeder._session_process_live(procs, "/var/lib/kandev/tasks/a", self.iso(self.START + 3600))
        )

    def test_unreadable_process_table_stays_conservative(self):
        self.assertTrue(feeder._session_process_live({}, "/var/lib/kandev/tasks/a", "1970-01-01T00:00:00+00:00"))


if __name__ == "__main__":
    unittest.main()
