# QA-07 — 三组 fixture 回归

运行：

```bash
python3 tests/fixtures/run_qa07.py --all
python3 tests/fixtures/run_qa07.py --all --json
```

测试矩阵（每组在独立临时数据目录中执行，退出时清理）：

| 类别 | CCK-8 | RNA/RT-qPCR | 非孔板 |
|---|---|---|---|
| 正常：创建 Design/Protocol/Run 并记录原始读数 | 自动验证 | 自动验证 | 自动验证 |
| 边界：互斥的 value 与 qualifier 输入被拒绝 | 自动验证 | 自动验证 | 自动验证 |
| 查询：`record.list` / `record.get` / `run.workspace` | 自动验证 | 自动验证 | 自动验证 |
| 重启恢复：独立 CLI 进程重开库并重读事件与冻结步骤 | 自动验证 | 自动验证 | 自动验证 |
| 并发：并行只读查询 | 两个并发读者 | 两个并发读者 | 两个并发读者 |
| SQLite SHA-256 | 每组输出 | 每组输出 | 每组输出 |

脚本输出记录在 `run-output.txt` 与 `results.json`；每次启动都会以当次运行结果覆盖对应证据文件。所有读数均为合成测试值，不代表实验结果。数据目录位于 `target/` 下的临时目录，fixture 不访问用户主库。
