# EXP-15 — 三类实验语义合成验收（synthetic-only）

> SYNTHETIC-ONLY：本页所有数值均为为覆盖契约而编造的合成输入，不是湿实验
> 结果，不声称 assay 有效性、仪器兼容或真实样本准确性。

## 测试矩阵

每组在独立临时数据目录 `<tmp>/exp15-three-semantics-*/{cck8,qpcr,biofilm}` 下执行；
不碰用户主库。夹具启动时若环境残留 `LABWORKBENCH_DATA_DIR` 直接拒绝运行。

| 类别 | CCK-8 | RT-qPCR | Biofilm CV+CFU |
|---|---|---|---|
| 正常 | raw OD450 + blank + control 三行分离；viability 派生 `derived_from` 指回三者 | raw target Cq + 参考基因 Cq + NTC Cq 伴随数据；dCq 派生依赖显式参考源 | CV OD 与 CFU 分属不同 endpoint；calculator 聚合同组重复；CFU/mL + CFU/sample 派生行指回 raw 板 ID；well→tube→plate lineage 落库 |
| 边界 | `above_uloq` 以 qualifier 保留，不转数字 | 缺参考源的 dCq 写入被拒 | 计数窗 30/300 含边界收录；窗外板 `below_window`/`above_window` 排除且不估算 |
| 失败回滚 | 未知来源派生被拒；事件数与 DB 校验和不变 | NTC 单独派生不断言通过（未知来源被拒）；校验和不变 | 重复 `raw_id` 计算输入被拒；DB 不变 |
| 重启恢复 | 独立进程重读 Run（3 事件 / 2 任务） | 同左（3 事件 / 2 任务） | 同左（3 事件 / 3 任务） |
| 并发 | 2 并行只读 workspace 查询 | 同左 | 同左 |
| SQLite SHA-256 | 每组输出（见下） | 每组输出（见下） | 每组输出（见下） |

复现步骤：

```bash
python3 tests/fixtures/three_experiment_semantics.py
```

备注：标量 `readout.submit` 的 `readout_id` 为 null（实现现状），凡被派生
`derived_from` 引用的原始读数均经 `execution_event.create` 显式 `readout_id`
写入；这是写法选择，不是运行时修改。`calculator.biofilm_cfu` 为纯计算，
不写库；派生 CFU 行经 `execution_event.create` 落库。

## 验收命令原始输出

（run id 与 sha256 每次重跑会变；语义断言不变。临时目录路径已脱敏，
运行后即删除。）

```bash
python3 tests/fixtures/three_experiment_semantics.py
```

```
== cck8 (synthetic) ==
{"case": "CCK-8 synthetic", "assertions": 9, "restart": {"events": 3, "tasks": 2}, "concurrent": 2, "checksum": "78cc85afdb9d650f783159a52978e3dcaece1bc35b4482b4bf8435043b98af84"}
== qpcr (synthetic) ==
{"case": "RT-qPCR synthetic", "assertions": 7, "restart": {"events": 3, "tasks": 2}, "concurrent": 2, "checksum": "d12c2625ea0c973ae991c33604598d4f5d002b67b42e3d6e73f9d3a69eb5788e"}
== biofilm (synthetic) ==
{"case": "Biofilm CV+CFU synthetic", "assertions": 13, "restart": {"events": 3, "tasks": 3}, "concurrent": 2, "checksum": "215a56e77a964e27ae378280f24a72bbaf7649332a56e08d8e12ee5c20bcebfb"}
EXP-15 SYNTHETIC ACCEPTANCE PASS (synthetic-only; isolated data dir <tmp>/exp15-three-semantics-*, removed after run)
CCK-8: raw_od_separate, raw_no_derivation, derived_kind, derived_sources, derived_method, saturated_qualifier, unknown_source_refused, rollback_event_count, rollback_checksum
qPCR: raw_cq_present, not_detected_qualifier, ntc_companion_data, dcq_derived, missing_ref_refused, rollback_checksum, ntc_no_autopass
Biofilm: cv_cfu_separate, no_endpoint_merge, group_averaging, boundary_inclusive, window_exclusion, no_estimation, cfu_method, cfu_lineage, cfu_units, cfu_persisted, lineage_trace, duplicate_raw_refused, rollback_checksum
```

exit code：0。

## 隔离说明与边界

- 数据目录：每次运行新建 `mktemp` 临时目录做 `LABWORKBENCH_DATA_DIR`，
  三组各用子目录；结束时整个删除。用户主库零接触。
- 数据库/文件校验：每组 SQLite SHA-256 见上；被拒写入前后比对文件集合
  与校验和，无部分写入。
- synthetic-only 边界：不执行湿实验；不声称 CCK-8/qPCR/CFU 的生物学有效性、
  仪器兼容或真实样本准确性；编造值仅用于证明 raw/derived、qualifier、
  来源与 lineage 语义可由现有 CLI/event/readout 契约表达。
- 缺口登记：无。全部 29 断言由现有契约表达，未发现需另开修复卡的缺陷。
