# QA-04 evidence

PR: https://github.com/LLLin000/AsterLab/pull/18
Tested implementation commit: `48782d985e0f9868306f07731108493cfb65defc`

## Matrix

| Case | Evidence test | Expected result |
|---|---|---|
| Single confirmed proposal | `commit_appends_one_version_and_records_the_confirmation` | revision 2 appended; `replayed=false`; revision 1 unchanged |
| Same-key replay | `replay_returns_the_original_result_without_a_second_version` | original result returned; no second row |
| Same key, different digest | `same_key_with_another_digest_is_an_idempotency_mismatch` | `IDEMPOTENCY_MISMATCH`; no extra row |
| Stale expected version | `stale_proposal_is_a_version_conflict` | `VERSION_CONFLICT`; no write |
| Old key after head advance | `replay_of_an_old_key_returns_the_original_after_the_head_advances` | original result returned; store checksum unchanged |
| Append failure and retry | `failed_append_rolls_back_and_retry_appends_once` | rollback; retry commits once; later replay does not append |
| Reopened store | `idempotent_replay_survives_a_store_restart` | replay survives reconstruction from persisted records |
| Eight distinct concurrent keys | `concurrent_distinct_keys_leave_exactly_one_new_version` | one success, seven version conflicts, two protocol-version rows total |
| Eight same-key submissions | `concurrent_same_key_writes_once_and_replays_the_rest` | one commit, seven replays, two protocol-version rows total |
| History checksum | `replay_and_failed_attempts_do_not_change_history_checksum` | all existing rows byte-identical across replay, failure and second commit |

## Rust test output

The required command was reported passing in the implementation session. Its PR description initially included only an excerpt. This review shell does not have Cargo installed (`cargo: command not found`), so the command could not be re-run here. The PR's Linux CI job ran the locked equivalent and passed all three Core test groups; complete job output is available at:

https://github.com/LLLin000/AsterLab/actions/runs/43375499648/job/131720929972

The captured test output for the changed test cases and summaries:

```text
running 256 tests
test agent_edit_commit::tests::concurrent_same_key_writes_once_and_replays_the_rest ... ok
test agent_edit_commit::tests::concurrent_distinct_keys_leave_exactly_one_new_version ... ok
test agent_edit_commit::tests::failed_append_rolls_back_and_retry_appends_once ... ok
test agent_edit_commit::tests::replay_and_failed_attempts_do_not_change_history_checksum ... ok
test agent_edit_commit::tests::idempotent_replay_survives_a_store_restart ... ok
test agent_edit_commit::tests::replay_of_an_old_key_returns_the_original_after_the_head_advances ... ok
test result: ok. 256 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s

running 3 tests
test result: ok. 3 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s

running 6 tests
test result: ok. 6 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s
```

## Isolated SQLite smoke output

The implementation session used an isolated temporary `LABWORKBENCH_DATA_DIR`; it did not use the user database. The original temporary directory name is intentionally omitted. Transcript recorded in the PR:

```text
records=6 checksum=sha256:3df90aab546997273dda04979f50bc51f09e9204f244b2f22c4acf4f4b30151b
protocol_versions=1

qa04-concurrent-a exit= 0
{"ok":true,"result":{"new_revision":2,"replayed":false}}
qa04-concurrent-b exit= 4
{"error":{"code":"VERSION_CONFLICT","current_version":2}}
records=7 checksum=sha256:e039518f698d941920f938b603a9021bf3b5e0182776884b0530500b98d396c9
protocol_versions=2

# replay of the winning key
{"replayed":true,"new_revision":2,"idempotency_key":"qa04-concurrent-a"}
records=7 checksum=sha256:e039518f698d941920f938b603a9021bf3b5e0182776884b0530500b98d396c9
protocol_versions=2

# stale expected_version rejection
{"code":"VERSION_CONFLICT","path":"expected_version"}
records=7 checksum=sha256:e039518f698d941920f938b603a9021bf3b5e0182776884b0530500b98d396c9
protocol_versions=2

# existing six-record history subset after revision 2
history_records=6 history_checksum=sha256:3df90aab546997273dda04979f50bc51f09e9204f244b2f22c4acf4f4b30151b
```

The existing six-record subset checksum is unchanged. Concurrent distinct-key submissions produced one successful revision 2 and one `VERSION_CONFLICT`; replay and stale rejection left the seven-row database checksum unchanged.

The in-memory history checksum test independently reports:

```text
history_checksum_before=sha256:542a7358055bda1f88b0e876bcf593270706c3d71b13a18a6b0374debc5b9dec
history_checksum_after_commit=sha256:542a7358055bda1f88b0e876bcf593270706c3d71b13a18a6b0374debc5b9dec
store_checksum_after_commit=sha256:13096c1a0e028c9fb62f56b1c333a4cf95a10d71523867b4588405fa2f857ed7
history_checksum_after_second_commit=sha256:542a7358055bda1f88b0e876bcf593270706c3d71b13a18a6b0374debc5b9dec
```
