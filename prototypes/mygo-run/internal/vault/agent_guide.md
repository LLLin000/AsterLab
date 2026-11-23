# AsterLab Vault · AI Agent 协作指南

本目录是 **AsterLab 本地单库（Vault）**，所有实验均为明文 `.asterml`（Markdown + YAML 结构化科学卡片），桌面端每 ~1.2 秒监听目录变更并热刷新。

- `<项目文件夹>/*.asterml`：实验设计与执行单源文档
- `templates/*.asterml`：无日期复用模板；`assets/`：图片与附件
- `.asterlab/pantry.jsonl`：本库独立储藏室台账（JSONL，按 `type + id` 排序）；`.asterlab/pantry-manuals/`：说明书与贴纸

---

## 1. 人类友好排布 5 条原则（让科学家进台一眼能用）

1. **首屏克制**：顶部只放 `> **实验目标**：...` + 2–4 条纯文字要点 + 最多 1 张 `group_table`；**孔板（`plate`）就近放在首次加样的步骤下**，让第一屏直接看到第一步。
2. **卡片克制 + 单一表达**：整篇控在 **5–8 张卡片**、单步骤 ≤ 2 张；同一条件不要在正文、参数表、计时器里重复写三遍。
3. **步骤骨架用 `#@` / `##@`**：直接写 `#@ 配制前驱液`、`##@ 称量与溶解`（可加 `{#t0}` 锚点），**不必手写数字序号**（软件会自动编号）；操作细则与防呆提醒直接写 `- [ ]` 复选框。
4. **正文格式**：支持标准 Markdown 与 Obsidian 风格行内/块级 LaTeX（`$...$` / `$$...$$`）；**暂不支持 Markdown 管道表**（`|---|---|`），普通数值直接加粗即可，不必乱包反引号。
5. **严禁伪造数据 & 不用老式块**：`data_table` 待测列留 `null` 或省略，**公式列必须写 `value_kind: formula`**（列数建议 ≤ 8 列）；不要生成老式 `step` / `workflow` / `assay` / `experiment` / `prose` / `subflow` 块。

---

## 2. 7 类核心模块最小可抄 YAML 速查

每份文档以 YAML Frontmatter 开头（`stage`: `planned | in_progress | completed`；模板留空 `scheduled_date`）：

```yaml
---
title: 实验标题
scheduled_date: 2026-11-24
stage: planned
tags: [水凝胶, CFU]
---
```

每个模块必须含 `kind`、文档内唯一 `id` 和 `schema: 1`：

### ① `group_table`（实验分组表）
- `role` 合法值：`control` | `positive_control` | `negative_control` | `blank` | `vehicle` | `treatment` | `reference`

```asterlab
kind: group_table
id: gt-groups
schema: 1
title: 实验分组
rows:
  - {id: g-ctrl, name: 阴性对照, role: control, condition: PBS, independent_n: 3}
  - {id: g-treat, name: 处理组, role: treatment, condition: 10 ug/mL, independent_n: 3}
```

### ② `plate`（孔板 / 培养皿 / 管架）
- `format`: `4 | 6 | 12 | 24 | 48 | 96 | 384`；`topology`: `plate | dish | rack_1d | cohort`
- `roles` 标注 `blank` 后，下方计算器 `@plate.sample_wells` 会自动排除空白孔

```asterlab
kind: plate
id: plate-main
schema: 1
title: 12 孔板排布
format: 12
topology: plate
groups:
  g-ctrl: [A1, A2, A3]
  g-treat: [B1, B2, B3]
roles:
  A4: blank
```

### ③ `param_table`（仅记录需要对照“计划 vs 实测”的关键条件）
```asterlab
kind: param_table
id: pt-cond
schema: 1
title: 关键孵育条件
rows:
  - {key: temp, label: 培养温度, value: 37, unit: Cel}
  - {key: enzyme, label: 胶原酶 II, value: 2, unit: U/mL}
```

### ④ `calculator`（统一 `mode: formula`，支持 LaTeX 与 `@plate.sample_wells`）
- 公式变量下标用单下划线（如 `V_{0}` 对应 `key: V_0`，不要写 `\_`）

```asterlab
kind: calculator
id: calc-mix
schema: 1
title: 预混液配制计算
mode: formula
rows:
  - {key: N, kind: input, label: 有效实验孔数, value: "@plate.sample_wells", unit: "1"}
  - {key: V_0, kind: input, label: 单孔体积, value: 100, unit: uL}
  - {key: r, kind: input, label: 余量比例, value: 0.15, unit: "1"}
  - {key: V_tot, kind: formula, label: 配制总体积, formula: 'N \times V_{0} \times (1 + r)', unit: uL}
```

### ⑤ `timer`（倒计时 / 秒表 / 循环 / 锚点窗口）
- `mode`: `countdown | stopwatch | cycle | window`

```asterlab
kind: timer
id: timer-inc
schema: 1
title: 37 °C 恒温培养
mode: countdown
status: idle
duration: {value: 24, unit: h}
```

### ⑥ `data_table`（实验读数与自动公式表）
- `value_kind` 合法值：`text` | `number` | `categorical` | `formula`（**带 `formula` 的列必须写 `value_kind: formula`**，且公式只引用非公式原始列）

```asterlab
kind: data_table
id: dt-readout
schema: 1
title: 抑菌圈双径记录表
columns:
  - {key: sample, label: 样本, value_kind: text}
  - {key: d1, label: 水平径 d1, value_kind: number, unit: mm}
  - {key: d2, label: 垂直径 d2, value_kind: number, unit: mm}
  - {key: d_avg, label: 平均直径, value_kind: formula, formula: "(d1 + d2) / 2", unit: mm}
rows:
  - {sample: Q1-PBS}
  - {sample: Q2-HAMA}
```

### ⑦ `figure` / `attachment`（SVG 示意图 / 采图网格；日常插图直接用 `![[./assets/图.png]]`）
```asterlab
kind: attachment
id: att-fov
schema: 1
title: 每孔 3 视野采图网格
scope: container_grid
container_ref: plate-main
slots_per_cell: 3
```

---

## 3. UCUM 常用合法单位

- 温度：`Cel`（不要写 `°C`）
- 浓度 / 效价：`g/mL`、`mg/mL`、`ug/mL`、`ng/mL`、`M`、`mM`、`uM`、`nM`、`%`、`% w/v`、`U/mL`、`U`、`CFU/mL`、`cells/mL`、`cells/well`
- 体积 / 质量 / 长度 / 时间 / 仪器：`L`、`mL`、`uL`、`g`、`mg`、`ug`、`mm`、`um`、`nm`、`s`、`min`、`h`、`d`、`AU`、`OD`、`rpm`、`xg`
- 无量纲倍数 / 计数：写 `"1"` 或 `""`（不要写 `"倍"`、`"×"`、`"孔"`）

---

## 4. CLI 校验与离线智能卡片建议

```bash
# 1. 校验语法、模块 Schema、公式列类型与 UCUM 单位（支持库内相对路径或绝对路径）
asterlab asterdoc.validate ./项目目录/实验计划.asterml

# 2. 评估实验就绪度（对照、独立 n、孔板分配等）
asterlab asterdoc.assess ./项目目录/实验计划.asterml

# 3. 离线语义建议与确定性模块编译（可对整文档或单句使用）
asterlab asterdoc.suggest ./项目目录/实验计划.asterml
asterlab asterdoc.suggest --text "37°C 倒置培养 24 h" ./项目目录/实验计划.asterml

# 4. 查看全部 CLI 子命令与语义命令
asterlab --help
asterlab cmd.list
```
