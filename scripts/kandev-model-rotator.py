#!/usr/bin/env python3
"""Rotate Agent/Review step agent_profile_id by hour of day.

Profile IDs:
  DS   = 29ea4051-cbfb-46b3-9474-27331d8841e8 (opencode-go/deepseek-v4.1-flash)
  LUNA = e930e6a0-835f-4e84-834e-09dea519280c (opencode-go/gpt-6-luna)

Schedule (local time, VPS = CST):
  0-8    DS
  8-12   LUNA
  12-14  DS
  14-18  LUNA
  18-24  DS
"""

import os
import re
import sqlite3
import urllib.request
import json
import sys
from datetime import datetime

DB = os.environ.get("KANDEV_DB", "/var/lib/kandev/data/kandev.db")
DS = "29ea4051-cbfb-46b3-9474-27331d8841e8"
LUNA = "e930e6a0-835f-4e84-834e-09dea519280c"

WORKFLOW = "d1a08c74-42ab-4e8d-aee3-a67b459537f8"
STEPS = {
    "Agent": "47b1aac7-2bb9-4b26-b8e3-412c424b4f46",
    "Review": "2b6d9951-e9fd-4578-8bf4-6f3e7aa7ce31",
}


def profile_for_hour(hour: int) -> str:
    if 0 <= hour < 8:
        return DS
    if 8 <= hour < 12:
        return LUNA
    if 12 <= hour < 14:
        return DS
    if 14 <= hour < 18:
        return LUNA
    return DS  # 18-24


def main():
    now = datetime.now()
    hour = now.hour
    target = profile_for_hour(hour)
    target_name = "DS" if target == DS else "LUNA"

    con = sqlite3.connect(DB)
    con.row_factory = sqlite3.Row
    changes = 0
    for name, step_id in STEPS.items():
        row = con.execute(
            "SELECT agent_profile_id FROM workflow_steps WHERE id=?", (step_id,)
        ).fetchone()
        if row is None:
            print(f"step {name} ({step_id[:8]}) not found; skipping")
            continue
        current = row["agent_profile_id"]
        if current == target:
            print(f"{name}: already {target_name}; no change")
        else:
            con.execute(
                "UPDATE workflow_steps SET agent_profile_id=? WHERE id=?",
                (target, step_id),
            )
            changes += 1
            print(f"{name}: switched -> {target_name}")
    con.commit()
    con.close()
    print(f"done: hour={hour} target={target_name} changes={changes}")


if __name__ == "__main__":
    main()
