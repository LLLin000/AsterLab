# 结构化实验记录 v1.1：ResultGrid 与批量读数

- 日期：2026-11-09
- 状态：产品候选，不是已实现规格
- 前置：MyGo Desktop v1 RC 候选基线 `47a9aea`

## 结论

以 **ResultGrid** 作为通用记录引擎，而不是为 CCK-8、RT-qPCR 各写一套表格：

- Designer 定义孔位/样本/分组/时间点/重复/检测类型。
- Run 只接收原始数值、缺失/无效标记，并保存到既有 Run 版本。
- 孔板视图和表格视图是同一份结果数据的不同投影。
- 批量写入由 Rust Core 的 `SaveReadoutsBatch` 在一个事务中完成，不能 UI 循环调用 96 次 `SaveReadout`。
- 派生统计（均值、SD、相对活性、ΔCq/ΔΔCq）属于分析层，永不覆盖原始 Cq/OD 读数。

## 通用模型

```
run_id + assay_id + task_key/timepoint
  └─ ResultGrid schema
       ├─ rows/columns (plate coordinates or sample×gene)
       ├─ cell type, unit, required/missing/invalid
       ├─ source range + group/replicate metadata
       └─ cells[] (raw value, status, provenance)
```

Schema 由检测类型选择，不由 UI 硬编码：

| 检测 | 默认视图 | 关键字段 |
| --- | --- | --- |
| CCK-8 | 8×12 孔板 / 分组表 | OD450、时间点、重复 |
| RT-qPCR | 孔板 / 样本×基因表 | 样本、基因、Cq、重复 |
| 溶血 | 分组表 | OD、阴阳性对照 |
| CFU | 稀释梯度表 | 稀释倍数、菌落数、体积 |
| 活死染色 | 图片 + 计数表 | 细胞计数、存活率 |
| SEM/EDS | 图片 + 参数表 | 样品、倍率、检测信息 |
| 动物 | 时间线 + 测量表 | 个体、时间点、测量值 |

## Excel/仪器粘贴

必须支持从 Excel、酶标仪、qPCR 软件复制二维数据：

1. 选中起始单元格，Ctrl+V。
2. 解析 TSV/CSV，识别行列、空值、单位、表头。
3. 预览将要覆盖的孔位；已有值覆盖必须明确确认。
4. 草稿先在 UI 高亮，确认后一次事务提交。
5. 空白单元格不得自动解释为 0；缺失与无效是不同状态。
6. 支持反向 Ctrl+C 导出选区为 TSV。

## Rust Core 契约（候选）

`SaveReadoutsBatch`：

- 输入：`run_id`、`operation_id`、`task_key`、`assay_id`、`timepoint`、`cells[]`（row/column/sample/gene/value/status）。
- 同一事务内校验：Run 冻结设计、assay/timepoint、孔位映射、值类型、幂等键。
- 返回：权威 revision、保存数量、冲突/无效明细。
- 相同 operation + 相同 payload 返回同一结果；不同 payload 返回冲突。
- 失败整批回滚，不允许部分提交。

## 验收

- CCK-8：从 Excel 粘贴 8×12，预览、确认、保存、重启、CLI 读回。
- RT-qPCR：样本×基因×3 重复，原始 Cq 不被派生结果覆盖。
- 空白、非数字、超范围值有明确错误，且不写入。
- 覆盖已有值有确认，不静默覆盖。
- 96 个单元格一次事务提交；不允许 96 次单条写入。
- Designer 的孔位分组与 Run 记录位置一致。

## 边界

- 先做 CCK-8 + RT-qPCR，验证通用 Grid；CFU、溶血等主要通过 schema 配置扩展。
- 不在 v1.1 重做 MyGo/UI 框架，不改变 v1 Run 的既有单条读取接口。
- 分析视图只读派生，不覆盖原始数据。
