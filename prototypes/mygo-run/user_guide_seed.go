package main

import (
	"log"
	"net/url"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/modules/runs"
)

// openOrSeedUserGuideDoc opens (or creates once) an Obsidian-style interactive
// Chinese wet-lab onboarding SOP inside the user's Vault showcasing a 96-well
// HUVEC cell assay, `@` reagent mention with expiry badge, Tufte `[^1]` sidenote,
// linked dilution/premix calculators, and local-ownership + WebDAV sync intro.
func (a *app) openOrSeedUserGuideDoc() {
	if a.vault == nil {
		return
	}
	if a.shell != nil {
		for _, f := range a.shell.VaultFiles {
			if strings.Contains(f.Title, "新手入门指南") || strings.Contains(f.RelPath, "新手入门指南") {
				a.openVaultFileInActivePane(f.RelPath, f.Title)
				a.shell.SetActiveTabMode(shell.ModeEdit)
				return
			}
		}
	}

	// Ensure at least one realistic Pantry reagent exists (including a near-expiry badge)
	// so the `@VEGF-165 重组蛋白` inline mention renders an authentic expiry status pill.
	vegfCode := "VEGF-165 重组蛋白"
	ecmCode := "ECM 基底膜胶"
	if ledger, err := a.localListPantry(); err == nil {
		if len(ledger.Items) == 0 {
			nearPurchased := time.Now().AddDate(0, 0, -350).Format("2006-01-02")
			nearShelf := uint32(365)
			normPurchased := time.Now().AddDate(0, 0, -14).Format("2006-01-02")
			normShelf := uint32(180)
			if it1, uErr1 := a.localUpsertPantry(runs.PantryUpsert{
				Name:          "VEGF-165 重组蛋白",
				Location:      "-80°C 冰箱 A2 盒",
				PurchasedOn:   nearPurchased,
				ShelfLifeDays: &nearShelf,
			}); uErr1 == nil {
				vegfCode = firstNonEmpty(it1.LabelCode, it1.ID, it1.Name)
			}
			if it2, uErr2 := a.localUpsertPantry(runs.PantryUpsert{
				Name:          "ECM 基底膜胶",
				Location:      "-20°C 试剂盒 B1",
				PurchasedOn:   normPurchased,
				ShelfLifeDays: &normShelf,
			}); uErr2 == nil {
				ecmCode = firstNonEmpty(it2.LabelCode, it2.ID, it2.Name)
			}
			if a.tools != nil {
				a.tools.ReloadPantry()
			}
		} else {
			vegfCode = firstNonEmpty(ledger.Items[0].LabelCode, ledger.Items[0].ID, ledger.Items[0].Name)
			if len(ledger.Items) > 1 {
				ecmCode = firstNonEmpty(ledger.Items[1].LabelCode, ledger.Items[1].ID, ledger.Items[1].Name)
			}
		}
	}

	today := time.Now().Format("2006-01-02")
	guideSrc := "---\n" +
		"title: 新手入门指南（HUVEC 细胞测定交互示范）\n" +
		"scheduled_date: " + today + "\n" +
		"stage: in_progress\n" +
		"tags:\n  - 新手指南\n  - HUVEC\n" +
		"---\n\n" +
		"欢迎来到 **AsterLab**！这份文档是一份可直接上手试玩的 **HUVEC 细胞血管成管与 CCK-8 活力测定 SOP**——你可以直接在下方拖拽 96 孔板、点按 `@` 试剂标签、查看右侧 Tufte 边栏批注、修改稀释配液数值，或粘贴酶标仪读数，随时可删可改。\n\n" +
		"## 1. 本地所有权与多端同步（3 步上手）\n\n" +
		"- **第 1 步 · 本地优先与明文所有权**：每份实验方案都是纯文本 `.asterml` 单一源文件，直接保存在你本地的实验库目录中；随时可用任意文本编辑器打开，或一键导出为 **Word (`.docx`)**、**GraphPad/R Tidy CSV** 与 **A4 PDF 执行单**。\n" +
		"- **第 2 步 · 原图无损上传 + 移动端省流预览**：在「设置 → 多端同步 (WebDAV)」填入坚果云 / 自建 WebDAV 账号（密码加密存入系统凭据库，绝不写入明文文件）。实验显微原图 100% 无损上传归档，手机端默认仅自动拉取轻量预览图，需要高精度测量时再按需点击下载原图。\n" +
		"- **第 3 步 · 历史快照、回收站与崩溃应急保障**：编辑时自动记录去重历史快照与本地应急草稿，支持整库 `.zip` 校验备份与回滚还原。\n\n" +
		"## 2. 细胞测定 SOP：试剂 `@` 引用、Tufte 批注与快捷语法\n\n" +
		"- 本次 HUVEC 诱导体系采用 [@VEGF-165 重组蛋白](asterlab://pantry?code=" + url.QueryEscape(vegfCode) + ") 与 [@ECM 基底膜胶](asterlab://pantry?code=" + url.QueryEscape(ecmCode) + ")，点击上方药丸可直达储藏室位置并实时查看临期/过期预警。\n" +
		"- 96 孔板接种密度建议控制在 `8,000 cells/well`，并在外周空白孔加入无菌 PBS[^1] 以减少边缘蒸发导致的 OD450 漂移。\n" +
		"- 输入 `/` 插入模块，输入 `@` 引用储藏室试剂、孔板或关联实验 `[[另一份实验]]`，输入 `[^1]` 创建学术边栏批注。\n" +
		"- [ ] 试着点一下我前面的复选框（支持记录完成状态）\n" +
		"- [ ] 试着点击顶部「实验」切换到防误触模式，再切回「编辑」\n\n" +
		"## 3. 实验分组与 96 孔板排布（动手拖选试试）\n\n" +
		"```asterlab\n" +
		"kind: group_table\n" +
		"id: gt-guide\n" +
		"schema: 1\n" +
		"title: HUVEC 分组设计\n" +
		"rows:\n" +
		"  - id: g_ctrl\n" +
		"    name: 空白培养基对照\n" +
		"    role: control\n" +
		"    condition: 0 ng/mL VEGF\n" +
		"    independent_n: 3\n" +
		"  - id: g_low\n" +
		"    name: 低浓度 VEGF\n" +
		"    role: treatment\n" +
		"    condition: 10 ng/mL VEGF\n" +
		"    independent_n: 3\n" +
		"  - id: g_high\n" +
		"    name: 高浓度 VEGF\n" +
		"    role: treatment\n" +
		"    condition: 50 ng/mL VEGF\n" +
		"    independent_n: 3\n" +
		"  - id: g_pbs\n" +
		"    name: PBS 空白防蒸发\n" +
		"    role: control\n" +
		"    condition: 外周空白孔\n" +
		"    independent_n: 3\n" +
		"```\n\n" +
		"下面这块 96 孔板已预选内圈 9 个有效实验孔与第 5 列 PBS 空白孔：**拖选任意孔位**即可涂色或标为异常孔；**右键孔板**可体验「按行横排」「按列竖排（8 道排枪）」「外周留空防蒸发」与导出 PNG/SVG：\n\n" +
		"```asterlab\n" +
		"kind: plate\n" +
		"id: plate-guide\n" +
		"schema: 1\n" +
		"title: HUVEC 96 孔测定板（试试拖选与右键菜单）\n" +
		"format: 96\n" +
		"groups:\n" +
		"  g_ctrl: [B2, B3, B4]\n" +
		"  g_low: [C2, C3, C4]\n" +
		"  g_high: [D2, D3, D4]\n" +
		"  g_pbs: [B5, C5, D5]\n" +
		"roles:\n" +
		"  B5: blank\n" +
		"  C5: blank\n" +
		"  D5: blank\n" +
		"```\n\n" +
		"#@ 4. 稀释配液与孔板联动预混计算 {#t0}\n\n" +
		"下方两个计算器分别演示 **C₁V₁ = C₂V₂ 试剂稀释** 与 **自动联动 96 孔板有效实验孔数（9 孔，已自动剔除 PBS 空白孔）**：\n\n" +
		"```asterlab\n" +
		"kind: calculator\n" +
		"id: calc-guide-dilution\n" +
		"schema: 1\n" +
		"title: VEGF 工作液梯度稀释计算 (C₁V₁ = C₂V₂)\n" +
		"mode: dilution\n" +
		"stock:\n" +
		"  value: 100\n" +
		"  unit: ug/mL\n" +
		"target:\n" +
		"  value: 50\n" +
		"  unit: ng/mL\n" +
		"final_volume:\n" +
		"  value: 5\n" +
		"  unit: mL\n" +
		"```\n\n" +
		"```asterlab\n" +
		"kind: calculator\n" +
		"id: calc-guide-mm\n" +
		"schema: 1\n" +
		"title: 孔板 CCK-8 预混液计算（已自动联动孔板有效孔）\n" +
		"mode: formula\n" +
		"rows:\n" +
		"  - key: N\n" +
		"    label: 有效实验孔数\n" +
		"    kind: input\n" +
		"    value: \"@plate.sample_wells\"\n" +
		"    unit: 孔\n" +
		"  - key: V_0\n" +
		"    label: 单孔工作液体积\n" +
		"    kind: input\n" +
		"    value: 110\n" +
		"    unit: uL\n" +
		"  - key: r\n" +
		"    label: 移液富余比例 (20%)\n" +
		"    kind: input\n" +
		"    value: 0.2\n" +
		"    unit: \"\"\n" +
		"  - key: V_tot\n" +
		"    label: 预混液总体积\n" +
		"    kind: formula\n" +
		"    formula: 'N \\times V_{0} \\times (1 + r)'\n" +
		"    unit: uL\n" +
		"```\n\n" +
		"```asterlab\n" +
		"kind: timer\n" +
		"id: timer-guide-1\n" +
		"schema: 1\n" +
		"title: CCK-8 显色孵育计时（37°C 避光）\n" +
		"mode: countdown\n" +
		"status: idle\n" +
		"duration:\n" +
		"  value: 60\n" +
		"  unit: min\n" +
		"```\n\n" +
		"#@ 5. 一键把实验描述转成结构化模块\n\n" +
		"把鼠标点到下面任意一行，点击行尾微亮的 `✦` 图标，或按住 `Alt` 键悬停在句子上（也可按 `Alt + Enter`）：\n" +
		"- 将 VEGF 母液 100 ug/mL 稀释至 50 ng/mL 工作液，终体积 5 mL\n" +
		"- 37°C 5% CO2 避光孵育 60 min 后测定 OD450\n\n" +
		"#@ 6. 酶标仪读数导入、Excel 粘贴与 Tidy CSV / Word 导出\n\n" +
		"点进下方表格任意单元格可直接 `Ctrl + V` 粘贴酶标仪矩阵；也支持从文档 `···` 菜单直接导入 CSV/TSV/XLSX 酶标仪矩阵，或导出为 GraphPad/R Tidy CSV 与 Word `.docx` 报告：\n\n" +
		"```asterlab\n" +
		"kind: data_table\n" +
		"id: dt-guide\n" +
		"schema: 1\n" +
		"title: HUVEC OD450 测定读数（支持 Ctrl+V 粘贴与 Tidy CSV 导出）\n" +
		"container_ref: plate-guide\n" +
		"columns:\n" +
		"  - key: well\n" +
		"    label: 孔位\n" +
		"    value_kind: text\n" +
		"  - key: value\n" +
		"    label: OD450\n" +
		"    value_kind: number\n" +
		"    unit: AU\n" +
		"rows:\n" +
		"  - well: B2\n" +
		"    value: 1.02\n" +
		"  - well: B3\n" +
		"    value: 0.98\n" +
		"  - well: C2\n" +
		"    value: 1.38\n" +
		"  - well: C3\n" +
		"    value: 1.42\n" +
		"  - well: D2\n" +
		"    value: 1.76\n" +
		"  - well: D3\n" +
		"    value: 1.81\n" +
		"  - well: B5\n" +
		"    value: 0.05\n" +
		"```\n\n" +
		"## 7. 常用快捷键与移动端快捷操作\n\n" +
		"- `Ctrl + K`：快速跳转实验或功能；`Ctrl + Shift + F`：全库搜索\n" +
		"- `Ctrl + E`：切换「编辑」与「实验（防误触）」模式\n" +
		"- `Alt + 悬停` / `Alt + Enter`：把操作句一键转为交互组件\n" +
		"- `Ctrl + V`：在数据表粘贴 Excel / 酶标仪矩阵；在正文直接粘贴截图\n" +
		"- 移动端底部悬浮工具栏支持自定义排序，一键触发 `@` 引用、`/` 模块、`[^1]` 批注、现场拍照与缩进排版\n\n" +
		"[^1]: 96 孔板最外圈孔位（A/H 行与 1/12 列）在 37°C 长时间培养时蒸发速率高于内圈孔，建议注入无菌 PBS 150 µL 作为防蒸发空白屏障，确保 CCK-8 读数稳健。\n"

	entry, err := a.vault.CreateExperiment("", "新手入门指南（可动手试玩）", today, guideSrc)
	if err != nil {
		log.Printf("seed user guide failed: %v", err)
		return
	}
	a.refreshVaultTreeAndCalendar()
	a.openVaultFileInActivePane(entry.RelPath, entry.Title)
	if a.shell != nil {
		a.shell.SetActiveTabMode(shell.ModeEdit)
	}
}
