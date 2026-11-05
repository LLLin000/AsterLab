# Catalog Consistency Audit — five JSON drafts vs research

- 日期:2026-11-05
- 状态:`research_draft`;这是契约一致性审查,不是 SourceChecked
- 范围:`catalog/operations/` 的五份 JSON + B01–B06 Reuse Map
- 结论:三份需要修订;修订后仍不能进入运行时,因为所有 profile/operation 仍是 `research_draft`

## 1. 状态定义

| 状态 | 含义 |
|---|---|
| Research Shared | 研究中已有 ≥2 个独立方法家族的使用证据 |
| Catalog Draft | 已有结构校验通过的 JSON,但无 `ReviewRecord` |
| Source Checked | 第二人逐字段复核 + `ReviewRecord` |
| Runtime | 已进入正式 Protocol/Run;当前没有 |

## 2. 五份 JSON 一致性

| 操作 | 研究范围 | JSON 原问题 | 修订 |
|---|---|---|---|
| `liquid_transfer` | 细胞、微生物、ELISA、3D | 目标已有样本的变化仍只在 note/条件 transition 中,未类型化 | 保留 conditional `add_to_existing`;记录“目标已有内容”缺口;不新增字段 |
| `sample_aliquot` | 上清、核酸、微生物 | 父/子 ID 有,但数量守恒未类型化 | 保留 conditional;记录数量/ID 缺口 |
| `incubation` | 细胞、微生物、染色、分子 | applicability 仅 cell_culture;CO2 必需;仪器固定为 CO2 培养箱 | 改为通用 controlled hold;CO2 可选;移除通用 instrument requirement;CO2 培养箱作为方法/仪器绑定 |
| `washing` | 细胞、ELISA、IF、生物膜、3D | `prevent_drying` 被写成普遍 true;样本保留默认 | 移除 `prevent_drying` 参数与 no-drying 阻断检查;新增 `loss_policy`(preserving/lossy_possible);不默认无损 |
| `absorbance_measurement` | CCK-8、LDH、ELISA、CV | 只允许 once;无法表达按时间点读取 | allowed `once` + `per_timepoint`;新增 `read_mode` endpoint/spectral;kinetic/continuous 保留为缺口 |

## 3. 研究“共享”但尚无 JSON

| 操作 | 研究状态 | 机器状态 | 处理 |
|---|---|---|---|
| 离心 | Research Shared | 无 JSON | 不急着建;先用 B01–B06 的端口/参数证据 |
| 原始荧光图像采集 | Research Shared | 无 JSON | 先补 raw readout 合同;不建分析 |
| 细胞浓度计数 | Research Shared | 无 JSON | 先补计数板/稀释规则 |
| 划线/涂布 | Research Shared | 无 JSON | 保持两个独立;不合并 |
| 菌落计数 | Research Shared(候选) | 无 JSON | 计数窗冲突未解决,暂缓 |
| 发光原始采集 | 缺口 | 无 JSON | 不并进 absorbance;需要独立 readout |
| PCR 热循环 | method-local(待复审) | 无 JSON | 需至少两个独立方法来源再评估 |
| 电泳分离 | method-local(待复审) | 无 JSON | 同上 |
| 原始流式采集 | method-local(待复审) | 无 JSON | 与分选/设门分离;先做 readout 合同 |

## 4. 修订后的边界

- `incubation` 不是“CO2 培养箱操作”;CO2 是可选方法条件。
- `washing` 不是“永不损伤样本”;loss policy 必须显式。
- `absorbance` 不是“只能读一次”;一次读板与 per-timepoint 分开;连续动力学尚未表达。
- 不要把 `Research Shared` 直接写成 `Catalog Draft`;两者不是同一状态。
- 不要因为修订 JSON 就升级 `SourceChecked`。

## 5. 验证

```text
cargo test -p asterlab-core --test catalog_drafts
cargo test -p asterlab-core --test operation_surface
```

五份 JSON 仍通过结构校验;草稿 profile 仍只能 `Unknown`。
