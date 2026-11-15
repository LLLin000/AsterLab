# LB-09 evidence：空 step path 的嵌套与提升

契约：[`docs/contracts/asterdoc-step-path-v1.md`](../../../contracts/asterdoc-step-path-v1.md)。

## 验收命令（原始输出）

| 文件 | 命令 | 结果 |
|---|---|---|
| `cargo-test-bin-labworkbench.txt` | `cargo test --bin labworkbench` | 46 passed; 0 failed; exit=0 |
| `cargo-test-asterlab-core.txt` | `cargo test --manifest-path crates/asterlab-core/Cargo.toml` | 424 + 3 + 6 + 2 passed; 0 failed; exit=0 |
| `cli-smoke.txt` | 真实 CLI 跑 LB-07 的空 path HUVEC 设计 | 见下 |

构建缓存绝对路径在证据里已脱敏为 `<target-dir>`。

## 新增测试

- `crates/asterlab-core/src/asterdoc_path.rs::tests`：派生规则、循环、已嵌套、顶层提升、路径冲突 11 条。
- `src/cli.rs::tests::asterdoc_derives_missing_step_paths_when_nesting_and_promoting`：派生成功分支（`nest_step` 同时补全父步骤）。
- `src/cli.rs::tests::asterdoc_rejects_a_derived_step_path_that_collides`：派生路径冲突的明确拒绝分支。
- `src/cli.rs::tests::asterdoc_rejects_promoting_a_step_that_is_already_top_level`：已在最外层的拒绝。
- `src/cli.rs::tests::asterdoc_add_step_derives_a_path_when_the_module_omits_one`：创建侧派生。

## CLI 走查（`cli-smoke.txt`）

1. `asterdoc.save` 写入 7 个 `path: ""` 的 HUVEC 步骤 → revision 1。
2. `nest_step step-04-seed` under `step-03-soak` → `root/step-03-soak` + `root/step-03-soak/step-04-seed`，revision 2。
3. `promote_step step-04-seed` → `root/step-04-seed`，revision 3。
4. 再次 `promote_step` → `INVALID_INPUT`（已在最外层），exit 2，无新 revision。
5. 人为制造 `root/step-05a-img-6h` 占用后 `promote_step step-05a-img-6h` → `INVALID_INPUT` 并指出两个步骤 id、冲突路径与改名示例，exit 2。

冻结 Run 未改动：所有命令只追加新的 `asterdoc_version` revision。
