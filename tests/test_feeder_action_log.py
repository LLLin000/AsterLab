import importlib.util
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load_feeder():
    spec = importlib.util.spec_from_file_location(
        "kandev_ready_feeder_action_log", ROOT / "scripts" / "kandev-ready-feeder.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class FeederActionLogTest(unittest.TestCase):
    def test_action_record_is_structured_and_private(self):
        module = load_feeder()
        with tempfile.TemporaryDirectory() as directory:
            module.ACTION_LOG = str(Path(directory) / "actions.jsonl")
            module.log_action(
                "move_step",
                {"id": "task-1", "workflow_step_id": "review", "state": "REVIEW", "labels": []},
                reason="ci_green",
                result="attempted",
                from_step="review",
                to_step="review",
            )
            lines = Path(module.ACTION_LOG).read_text().splitlines()
            self.assertEqual(len(lines), 1)
            record = json.loads(lines[0])
            self.assertEqual(record["source"], "legacy_feeder")
            self.assertEqual(record["task_id"], "task-1")
            self.assertEqual(record["action"], "move_step")
            self.assertEqual(record["result"], "attempted")
            self.assertTrue(record["snapshot_id"])
            self.assertEqual(stat.S_IMODE(os.stat(module.ACTION_LOG).st_mode), 0o600)


if __name__ == "__main__":
    unittest.main()
