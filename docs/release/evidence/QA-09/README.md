# QA-09 — 三类真实实验 CLI 回归夹具

测试矩阵（每组在隔离临时数据目录下执行，`<target>/qa09-three-protocol-*/{huvec,rtqpcr,invivo}`；不碰用户主库）：

| 类别 | HUVEC 成管 | RT-qPCR | 体内药效 + 下游 |
|---|---|---|---|
| 正常 | 官方样板 validate→compile→create_run；7 冻结任务；27 孔×时间点；162 图像坐标 | Nanodrop 配平不足 1；Ct CSV 4 行；S2 重试 attempt 2；S3 淘汰 | D0/D3/D6/D10/D14 周期窗口；盲法视图；4 次盲法给药；1-to-3 分样；3 条 1D 泳道；2 条非标量读数 |
| 边界 | `fov_index: 0` 与无孔位时间点/FOV 拒绝 | cells+csv 混传拒绝；无单位 CSV 拒绝 | 跳号 attempt 拒绝；`well.set Tube13` 越界拒绝；未知 view 拒绝 |
| 失败回滚 | 重复步骤 id 的 save 被拒且 revision 不变 | 被拒批次不追加事件且校验和不变 | 未知检查项事件被拒且校验和不变 |
| 重启恢复 | 独立进程重读 Run 与事件 | 同左 | 同左 |
| 并发 | 两个并行只读查询 | 同左 | 同左 |
| SQLite SHA-256 | 每组输出 | 每组输出 | 每组输出 |

`results.json` 记录每组 run id、protocol id、计数与校验和；每次启动都会以当次运行结果覆盖。

测试矩阵覆盖说明（QA 工作方法要求正常、边界、失败、重启、并发五列，见上表）。

复现步骤：

```bash
python3 tests/fixtures/three_protocol_cli_acceptance.py
cargo test --locked --bin labworkbench
```

下面是验收命令的原始输出（run id 与 sha256 每次重跑会变；语义断言不变）：

```
== huvec ==
{"case": "HUVEC tube formation", "asterdoc_revision": 1, "run_id": "941f8ffb-e03a-48cc-9197-9f0eecdc5445", "frozen_tasks": 7, "planned_well_timepoints": 27, "image_links": 162, "asset_coverage_cells": 27, "rollback": "PASS", "boundary": "PASS", "restart": {"events": 1, "tasks": 7, "steps": 7}, "concurrent_queries": 2, "checksum": "48fd7d61184a8f7227c0a797cf2f03b7a4ff2ca655553d22866ca9b4e32de928"}
== rtqpcr ==
{"case": "RT-qPCR cross plate", "run_id": "671e614a-bed2-47a5-bbc4-da3d27ef4fb5", "plate_id": "e6506fed-81e2-4d8d-ac2f-67a299cc15fe", "normalization_insufficient": 1, "qc_dropped_event": "ee702e6e-5602-44df-bd00-0f3577f8bb1a", "retry_event": "6be9ccdc-e477-4b74-8190-8e9ac350ef42", "ct_rows": 4, "rollback": "PASS", "boundary": "PASS", "restart": {"events": 7, "tasks": 3, "steps": 3}, "concurrent_queries": 2, "checksum": "5a5dcdbf615bcbb53133abbc71f440d649e442071202c2658dcbead6b8ed1a0e"}
== invivo ==
{"case": "in-vivo efficacy + WB/BCA-ELISA", "protocol_versions": {"efficacy": "4b6c62f8-9b13-4ef0-85e1-b648613d5d59", "wb": "82fc67df-b1c3-4c78-b94d-c93a80ba44e0", "bca_elisa": "2e9eb1c6-5156-421b-af61-29e79f6d9a7c"}, "run_id": "6af27d12-69ba-4a72-ae4e-b80c5f031a61", "scheduled_tasks": 5, "dosing_events": 4, "censored_dead_event": "m-04", "aliquot_event": "af0874e9-713e-4d2b-8d6a-467264528fa5", "wb_lanes": 3, "non_scalar_readouts": 2, "rollback": "PASS", "boundary": "PASS", "restart": {"events": 8, "tasks": 7, "steps": 3}, "concurrent_queries": 2, "checksum": "b59deb577d353473c78ff219a788809a1e794cb274a1d81de843f2c60250173c"}
QA-09 three-protocol CLI acceptance: PASS
```

```bash
cargo test --locked --bin labworkbench
```

```
test result: ok. 62 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 1.79s
```

样板修复说明：`docs/sdk/huvec-tube-formation-case/huvec-tube-formation.asterml`
把已退役枚举改成现行值（`timepoint_relation: same_experimental_unit_repeated`、
`planned_run_relationship: same_source`），把 `step@2` 升到显式 `execution: executable`
的 `step@3`，新增 7 个 `reference(view: protocol_block)` 模块并去掉跨时间点共用的
`plate.timepoint`（回退到 assay 自带 6h/12h/24h）。样板内的 `reference.ref` 与
`assay.protocol_version_id` 写成占位词：ProtocolBlock 版本是建库时才生成的不可变
UUID，静态文档无法预填；fixture 在隔离库里建块后绑定实际 id 再走
validate→compile→create_run。
