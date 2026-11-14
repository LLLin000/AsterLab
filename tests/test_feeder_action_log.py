import importlib.util
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]


def load_feeder():
    spec = importlib.util.spec_from_file_location(
        "kandev_ready_feeder_action_log", ROOT / "scripts" / "kandev-ready-feeder.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


TASK = {
    "id": "task-1",
    "title": "private task",
    "workflow_step_id": "review",
    "state": "REVIEW",
    "labels": ["lane:vps"],
}


class FeederActionLogTest(unittest.TestCase):
    def setUp(self):
        self.module = load_feeder()
        self.temp = tempfile.TemporaryDirectory()
        self.module.ACTION_LOG = str(Path(self.temp.name) / "actions.jsonl")

    def tearDown(self):
        self.temp.cleanup()

    def records(self):
        return [json.loads(line) for line in Path(self.module.ACTION_LOG).read_text().splitlines()]

    def test_action_record_is_structured_and_private(self):
        self.module.log_action(
            "move_step", TASK, reason="ci_green", result="attempted",
            from_step="review", to_step="review",
        )
        record = self.records()[0]
        self.assertEqual(record["source"], "legacy_feeder")
        self.assertEqual(record["task_id"], "task-1")
        self.assertEqual(record["action"], "move_step")
        self.assertEqual(record["result"], "attempted")
        self.assertEqual(record["session_id"], "unknown")
        self.assertEqual(record["head_sha"], "unknown")
        self.assertTrue(record["action_id"])
        self.assertTrue(record["snapshot_id"])
        self.assertNotIn("private task", Path(self.module.ACTION_LOG).read_text())
        self.assertEqual(stat.S_IMODE(os.stat(self.module.ACTION_LOG).st_mode), 0o600)

    def test_attempted_and_result_share_action_and_snapshot_ids(self):
        self.module.log_action("move_step", TASK, reason="r", result="attempted",
                               from_step="review", to_step="review")
        self.module.log_action("move_step", TASK, reason="r", result="ok",
                               from_step="review", to_step="review")
        first, second = self.records()
        self.assertEqual(first["action_id"], second["action_id"])
        self.assertEqual(first["snapshot_id"], second["snapshot_id"])

    def test_move_success_logs_attempt_and_ok(self):
        with mock.patch.object(self.module, "api", return_value=(200, {})):
            self.assertTrue(self.module.move(TASK, "review", "wake"))
        records = self.records()
        self.assertEqual([r["result"] for r in records], ["attempted", "ok"])
        self.assertEqual(records[0]["action_id"], records[1]["action_id"])
        self.assertEqual(records[0]["snapshot_id"], records[1]["snapshot_id"])

    def test_move_failure_is_logged_without_changing_return(self):
        with mock.patch.object(self.module, "api", return_value=(500, {"error": "x"})):
            self.assertFalse(self.module.move(TASK, "agent", "retry"))
        records = self.records()
        self.assertEqual([r["result"] for r in records], ["attempted", "failed:500"])
        self.assertEqual(records[0]["action_id"], records[1]["action_id"])

    def test_log_write_failure_does_not_raise(self):
        self.module.ACTION_LOG = "/proc/1/no-such-action-log.jsonl"
        self.module.log_action("move_step", TASK, result="attempted")


if __name__ == "__main__":
    unittest.main()
