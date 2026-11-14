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


if __name__ == "__main__":
    unittest.main()
