package vault

import (
	"os"
	"path/filepath"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const AgentGuideFileName = "AGENTS.md"

// DefaultVaultAgentGuide is seeded once into new Vault roots (never overwriting
// existing user edits) so any zero-context AI Agent (Claude Code, Cursor, Codex,
// Antigravity, etc.) immediately knows how to read, author, and validate `.asterml`
// experiments and inspect Vault-local Pantry inventory.
const DefaultVaultAgentGuide = `# AsterLab Vault · AI Agent 协作指南

本目录是一个 **AsterLab 本地单库（Vault）**。所有实验文档均以明文 ` + "`.asterml`" + `（Markdown + YAML 结构化科学模块）存储，桌面端每 ~1.2 秒自动监听本目录变更——外部 Agent 新增、修改或重命名 ` + "`.asterml`" + ` 文件后，AsterLab 会像 Obsidian 一样自动刷新左侧文件树、日历与已打开的标签页。

## 1. 目录结构与数据格式

- ` + "`<项目文件夹>/*.asterml`" + `：实验设计与执行单源文档（可直接用任意编辑器或 Agent 读写）。
- ` + "`templates/*.asterml`" + `：无日期复用模板（不会出现在实验日程日历中）。
- ` + "`assets/`" + `：实验图片与附件（文档内使用相对路径引用）。
- ` + "`.asterlab/`" + `：本 Vault 独立配置与**储藏室试剂台账**（随 Vault 同步与隔离）：
  - ` + "`.asterlab/pantry.jsonl`" + `：单文件 JSONL 储藏室总账（每行一个 JSON 对象，按 ` + "`type + id`" + ` 排序，云盘/Git 同步友好）：
    - ` + "`{\"type\":\"schema\",\"version\":1}`" + `
    - ` + "`{\"type\":\"box\",\"id\":\"...\",\"name\":\"冻存盒 #1\",\"kind\":\"cryobox_9x9\",\"zoneKey\":\"-80\",\"rows\":9,\"cols\":9}`" + `
    - ` + "`{\"type\":\"reagent\",\"id\":\"...\",\"labelCode\":\"PN-001\",\"name\":\"Vancomycin\",\"location\":\"-20°C\",\"purchasedOn\":\"2026-11-23\",\"shelfLifeDays\":365,\"revision\":1,\"meta\":{\"catalogNo\":\"...\",\"lotNo\":\"...\",\"boxId\":\"...\",\"boxSlot\":\"A1\"}}`" + `
  - ` + "`.asterlab/pantry-manuals/`" + `：上传的试剂说明书 PDF / COA / SDS 与自定义贴纸图片。

## 2. AsterML v2.1 编写规范（Markdown 优先 + 渐进结构化）

每份 ` + "`.asterml`" + ` 文件遵循 **80% 自由 Markdown + 20% 关键交互卡片** 原则：给人看和记笔记的内容全部用标准 Markdown 写，只在需要计算、排板、倒计时、数据汇总时放置 ` + "```asterlab" + ` 交互模块。

### 2.1 Frontmatter（唯一文档头）

` + "```yaml" + `
---
title: 实验标题
scheduled_date: 2026-11-24
stage: planned
tags:
  - 抗菌实验
---
` + "```" + `

- ` + "`stage`" + ` 合法枚举：` + "`planned`" + `（计划中）| ` + "`in_progress`" + `（进行中）| ` + "`completed`" + `（已完成）。
- 模板文件省略或留空 ` + "`scheduled_date`" + `。
- 实验目的与背景直接用 Markdown 引用块 ` + "`> **实验目标**：...`" + ` 与普通列表说明，**不要再手写老式 ` + "`kind: experiment`" + `、` + "`kind: assay`" + `、` + "`kind: workflow`" + ` 块**。

### 2.2 顶部实验蓝图（分组表 + 孔板/平皿）

需要分组对照或孔板排布时，在顶部放 ` + "`group_table`" + ` 与 ` + "`plate`" + `（供下方计算器 ` + "`@plate.sample_wells`" + ` 与数据表自动联动）：

` + "````markdown" + `
` + "```asterlab" + `
kind: group_table
id: gt-groups
schema: 1
title: 实验分组
rows:
  - id: g-ctrl
    name: 阴性对照组
    role: control
    condition: 空白缓冲液
    independent_n: 3
  - id: g-treat
    name: 处理组
    role: treatment
    condition: 10 ug/mL 样品
    independent_n: 3
` + "```" + `
` + "````" + `

- ` + "`group_table.rows[].role`" + ` 常用值：` + "`control`" + ` | ` + "`positive_control`" + ` | ` + "`treatment`" + ` | ` + "`blank`" + ` | ` + "`reference`" + `。

### 2.3 步骤骨架与 7 类核心交互卡片（统一标准）

- **步骤一律使用正交标题语法**：` + "`#@ 1. 主步骤标题`" + `、` + "`##@ 1.1 子步骤标题`" + `（可选末尾加 ` + "`{#t0}`" + ` 时间锚点）。桌面端会自动识别步骤树、大纲、勾选进度以及 **` + "`Alt+悬停`" + ` 智能解析作用域**。
- **操作说明与核对清单直接写普通 Markdown 段落和 ` + "`- [ ]`" + ` 复选框**。
- **严禁在新文档中使用老式容器/冗余块**：不要生成 ` + "`kind: step`" + `、` + "`kind: workflow`" + `、` + "`kind: assay`" + `、` + "`kind: experiment`" + `、` + "`kind: prose`" + `、` + "`kind: subflow`" + `（引擎仅保留对历史旧文件的静默兼容）。
- 仅在真正需要交互计算、排布或读数汇总时，在 ` + "`#@`" + ` 步骤下按需插入以下 **7 类交互模块**（严禁自造新 ` + "`kind`" + `）：

| ` + "`kind`" + ` | 用途 | 核心字段 |
|---|---|---|
| ` + "`group_table`" + ` | 分组设计（对照、处理、独立重复数） | ` + "`title, rows[{id, name, role, condition, independent_n}]`" + ` |
| ` + "`plate`" + ` | 孔板 / 平皿 / 管架排布映射 | ` + "`title, format (6|12|24|48|96|384), topology (plate|dish|rack_1d), groups`" + ` |
| ` + "`param_table`" + ` | 需要记录“计划 vs 实测”的关键条件 | ` + "`title, rows[{key, label, value, unit}]`" + ` |
| ` + "`calculator`" + ` | 稀释、称量、MasterMix 或公式自动计算 | ` + "`title, mode (dilution|serial_dilution|group_mastermix|formula), ...`" + ` |
| ` + "`timer`" + ` | 孵育 / 离心 / 光固化倒计时、秒表或循环计时 | ` + "`title, mode (countdown|stopwatch|cycle), status (idle), duration: {value, unit}`" + ` |
| ` + "`data_table`" + ` | 计划测量表、自动换算公式与实测读数表 | ` + "`title, columns[{key, label, value_kind, unit, formula}], rows[...]`" + ` |
| ` + "`figure`" + ` / ` + "`attachment`" + ` | 物理比例 SVG 示意图 / 按孔位采图网格（日常插图直接用 ` + "`![[./assets/图.png]]`" + `） | ` + "`title, svg|src|scope, container_ref`" + ` |

每个模块都必须包含 ` + "`kind`" + `、` + "`id`" + `（文档内唯一）和 ` + "`schema: 1`" + `。

### 2.4 关键硬约束（写前必查）

1. **UCUM 合法单位**：` + "`param_table`" + ` 与 ` + "`data_table`" + ` 的 ` + "`unit`" + ` 字段必须使用标准 UCUM 记法，例如：
   - 温度：` + "`Cel`" + `（不要写 ` + "`°C`" + `）
   - 浓度：` + "`ug/mL`" + `、` + "`mg/mL`" + `、` + "`ng/mL`" + `、` + "`mmol/L`" + `、` + "`uM`" + `、` + "`mM`" + `、` + "`%`" + `
   - 体积 / 质量：` + "`uL`" + `、` + "`mL`" + `、` + "`L`" + `、` + "`ug`" + `、` + "`mg`" + `、` + "`g`" + `
   - 时间 / 长度 / 微生物 / 吸光度：` + "`s`" + `、` + "`min`" + `、` + "`h`" + `、` + "`d`" + `、` + "`mm`" + `、` + "`cm`" + `、` + "`CFU/mL`" + `、` + "`AU`" + `、` + "`rpm`" + `
   - **无量纲比例 / 倍数 / 计数**：必须写 ` + "`\"1\"`" + ` 或留空 ` + "`\"\"`" + `，**严禁**写 ` + "`\"×\"`" + `、` + "`\"倍\"`" + `、` + "`\"次\"`" + `！
2. **严禁伪造实测数据**：编写实验计划时，` + "`data_table`" + ` 只预填样本/组别/计划浓度等自变量列，**所有待测读数列必须留空（` + "`null`" + ` 或省略）**，等实验执行时由实验人员录入。
3. **不修改历史冻结记录**：除非用户明确要求，不要篡改已完成实验的原始读数。

## 3. 内置校验与智能卡片编译（CLI）

若环境已安装 ` + "`asterlab`" + ` CLI，推荐采用 **“先写清爽 ` + "`#@`" + ` Markdown → 调 ` + "`asterdoc.suggest`" + ` 获取确定性卡片建议 → ` + "`asterdoc.validate`" + ` 校验”** 工作流：

` + "```bash" + `
# 1. 调用内置离线语义引擎 + 确定性编译器，检查文档中哪些步骤句适合升格为计时器/计算器/孔板卡片（附带合规 YAML）
asterlab asterdoc.suggest ./项目目录/实验计划.asterml

#    也可直接对单句自然语言编译合规模块 YAML（零单位错、自动联动已有孔板）
asterlab asterdoc.suggest --text "37°C 倒置培养 24 h" ./项目目录/实验计划.asterml

# 2. 校验 AsterML 语法、模块 Schema 与 UCUM 单位
asterlab asterdoc.validate ./项目目录/实验计划.asterml

# 3. 评估实验方案就绪度（检查对照、重复数、缺失参数等）
asterlab asterdoc.assess ./项目目录/实验计划.asterml

# 4. 查看全部可用语义命令与参数说明
asterlab cmd.list
` + "```" + `
`

// SeedAgentGuide writes AGENTS.md into the vault root if absent so external
// AI Agents have self-contained instructions without prior conversation context.
func (v *Vault) SeedAgentGuide() error {
	if v == nil || v.Root == "" {
		return nil
	}
	target := filepath.Join(v.Root, AgentGuideFileName)
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	return asterdoc.WriteFileAtomic(target, []byte(DefaultVaultAgentGuide))
}
