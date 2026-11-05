# Batch-06 Flow Cytometry & Immunofluorescence — Wet-Lab Discovery (prep / stain / acquire-sort / fix-perm-block / image)

- 日期: 2026-11-05
- 状态: `research_draft`；不是 `SourceChecked`。第二人逐字段复核前不得晋升、不得进入 JSON/运行时。
- 范围: 仅湿实验步骤与原始读数。流式：样本分装、表面/胞内/活力染色、获取与分选；IF：固定/通透/封闭/染色/封片/原始成像。设门、补偿、群体统计、图像分割/定量一律 out-of-scope，不建操作。
- 规则: 现有仓库记录（`k2-operations/flow-cytometry-and-immunofluorescence.md` 等）只用于 Reuse Map，不作步骤/数值证据；每条数值/行为证据只引本文件 §1 的原始 protocol；不提出任何新公共模块；平台能力不等于单机构建。
- 颗粒度: 通用 `stain` / `fix` / `perm` / `block` 不建模块（effect 跨染料/缓冲/终点不稳定，G2 不过），一律 method-local；洗涤/离心/转移/孵育/原始成像按参数化复用。

## 1. 来源索引

| # | Protocol | 权威类型 | 版本/定位 | 读取程度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Thermo Fisher BestProtocols: Staining Cell Surface Targets for Flow Cytometry (Protocol A cell suspensions; Protocol B lysed whole blood) https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/staining-cell-surface-targets-flow-cytometry.html | 厂商手册 | 当前页；General notes + Protocol A 步骤 1–14 + 材料表 + polymer-dye/CellBlox 表；Protocol B 步骤 1–16 | 直接读：A 全文转写；B 只记存在与分支差异，不作主绑定 |
| S2 | Thermo Fisher BestProtocols: Staining Intracellular Antigens for Flow Cytometry (Protocol A cytoplasmic; B nuclear Foxp3; C Fixation/Methanol) https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/staining-intracellular-antigens-flow-cytometry.html | 厂商手册 | 当前页；General Notes 1–4 + A 管/板步骤 + B 管/板步骤 + C 管/板步骤 + MeOH 耐受表 | 直接读：三协议全文转写 |
| S3 | Thermo Fisher BestProtocols: Viability Staining Protocol for Flow Cytometry (A PI/7-AAD; B calcein; C1–C5 FVD) https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/viability-staining-flow-cytometry.html | 厂商手册 | 当前页；A 步骤 1–5 + B 步骤 1–8 + C 总注 + C1–C5 步骤 | 直接读：全文转写 |
| S4 | Thermo Fisher BestProtocols: Immunofluorescent Staining of Intracellular Antigens on Cultured Cells, 步骤 1–15 + 材料表 https://www.thermofisher.com/us/en/home/references/protocols/cell-and-tissue-analysis/protocols/immunofluorescence-staining-intracellular-antigens-cultured-cells.html | 厂商手册 | 当前页；步骤 1–15 + PBS/TBS 配方 + 注记 | 直接读：全文转写 |
| S5 | Thermo Fisher Fix, Perm, & Block (Molecular Probes School of Fluorescence), 步骤 1–10 + 体积/时间表 https://www.thermofisher.com/us/en/home/life-science/cell-analysis/cell-analysis-learning-center/molecular-probes-school-of-fluorescence/imaging-basics/protocols-troubleshooting/protocols/fix-perm-block.html | 厂商手册 | 当前页；步骤表 1–10 + What-you-need | 直接读：全文转写 |

未用作证据：BD "Flow Cytometry Controls" 博客、FACSAria 分选 facility/SOP PDF 群、厂商分析仪/分选仪包络页——均为 `search-summary-only`，不引数值；任何单机构建表、转子映射、lot 稀释单均未读到。

## 2. P-FLOW-SURFACE (S1-A 主路径；S1-B 只记分支)

绑定摘要：单细胞悬液 → 可选 Fc 封闭（mouse 0.5–1 µg anti-CD16/32 / 100 µL，10–20 min 2–25 °C；human 20 µL Fc 抑制剂 / 100 µL；或 2 µL 2% 血清 / 100 µL 15 min，染前不洗）→ 分装 50 µL/管或孔（稀有事件数公式只引不设值 `r=(100/CV)×2`）→ polymer 染料配伍封闭（Super Bright Complete 5 µL/样，或 Brilliant 50 µL/样，或 Plus 10 µL/样；NovaFluor/花青配 CellBlox (Plus) 5 µL/样，抗体混合液现配现用）→ 抗体混合液补足至终体积 100 µL（50 µL 细胞 + 50 µL 抗体），轻脉冲涡旋 → 30 min 2–8 °C 避光（纯化/生物素化一抗 1 h）→ 2 mL/管或 200 µL/孔洗液，400–600×g 5 min RT，弃上清，×2 → 二抗（非直标时）100 µL 稀释液 ≥30 min 2–8 °C/冰上避光 + 同规格洗 → 可选活力染料（路由到 P-FLOW-VIAB）→ 可选储存（100 µL 样品 + 100 µL IC Fixation Buffer，或 2 mL 1-step Fix/Lyse，≤3 d 2–8 °C 暗）→ 上机分析或分选（本批只记原始 FCS/分选谱系，不定仪器值）。S1-B 全血分支：100 µL 全血/管、20–30 min 2–25 °C、1X RBC Lysis 2 mL 10–20 min（>20 min 禁止）、Fix/Lyse 前须确认克隆耐固定；细节不展开。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 单细胞悬液制备（引用前置 protocol） | 前置条件，不拆解 | 上游悬液 → 待染悬液 | 方法绑定：单细胞确认是进入条件 | S1-A 步骤 1（引用） |
| 可选 Fc 封闭三选一（mouse/human/血清），染前不洗 | `op.liquid_transfer` 参数化 + method-local `block-Fc` | 待染悬液 → 已封闭悬液 | 方法绑定：0.5–1 µg/100 µL 或 20 µL/100 µL 或 2 µL 2%/100 µL；10–20 min / 15 min；2–25 °C；NovaFluor + mouse 需双血清 | S1-A 步骤 2 + 注记 |
| 分装 50 µL/管或孔；稀有事件公式 | `op.sample_aliquot` 参数化 | 母悬液 → 分装管/孔（父保留） | 方法绑定：50 µL/管孔；`r=(100/CV)×2` 只引公式，输入系别依赖 | S1-A 步骤 3 + Tip |
| polymer/花青配伍封闭（SB/Brilliant/Plus/CellBlox 四选），现配现用 | `op.liquid_transfer` 参数化 + method-local `block-dye` | 分装管 → 已配伍封闭管 | 方法绑定：5 / 50 / 10 / 5 µL/样；>1 polymer 染料才必需 SB；CellBlox 逢 NovaFluor 必加 | S1-A 步骤 4 + 表 + 注记 |
| 抗体混合液补足至 100 µL 终体积，轻脉冲涡旋 | `op.liquid_transfer` 参数化 | 已封闭管 → 已加抗体管 | 方法绑定：50 + 50 µL = 100 µL；抗体量按 lot TDS；涡旋禁令（抗体瓶禁涡旋，快 spin） | S1-A 步骤 5 + General notes |
| 30 min 2–8 °C 避光（纯化/生物素化 1 h；冰上需延长） | `op.incubation` 参数化 | 已加抗体管 → 已孵育管 | 方法绑定：30 min / 1 h；2–8 °C；避光；冰上延长无值 | S1-A 步骤 6 + 注记 |
| 洗：2 mL/管或 200 µL/孔，400–600×g 5 min RT，弃上清，×2 | `op.washing` 参数化 + `op.centrifugation` 参数化 | 已孵育管 → 已洗管 + 废液 | 方法绑定：2 mL / 200 µL；400–600×g 5 min RT；×2 | S1-A 步骤 7–8 |
| 二抗 100 µL ≥30 min 2–8 °C/冰上避光 + 同规格洗 | `op.liquid_transfer` + `op.incubation` + `op.washing`/`op.centrifugation` 参数化 | 已洗管 → 已二染管 | 方法绑定：100 µL；≥30 min；直标全标时跳过（直达储存/活力） | S1-A 步骤 9–11 + 注记 |
| 可选活力染料 | 路由到 P-FLOW-VIAB（method-local） | 已染管 → 活力标记管 | 方法绑定：FVD 逢固定必选；NovaFluor 禁 DAPI/PI/SYTOX | S1-A 步骤 12 + 注记 |
| 可选储存：100 + 100 µL IC Fix 或 2 mL Fix/Lyse，≤3 d | `op.liquid_transfer` 参数化 + `op.incubation`（保持窗） | 已染管 → 已固定储存管 | 方法绑定：100 + 100 µL 或 2 mL；≤3 d 2–8 °C 暗；tandem 亮度影响须逐克隆验证 | S1-A General notes + 步骤 13 |
| 上机分析或分选 | 方法模板读数/分选（原始 FCS + 分选谱系），不建共享 op | 已染管 → 原始 FCS（分析）/ 子群 + FCS（分选） | UNKNOWN：事件数/流速/激光-滤光-增益-阈值/喷嘴/压/延迟；逐机构建绑定 | S1-A 步骤 14（仅动作存在） |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（封闭/加抗体/加洗液/储存液） | 细胞培养、微生物、生化读数板系 | shared (`op.liquid_transfer`) | S1-A 步骤 2/4/5/9/13；只绑体积 |
| 取样/分样（50 µL 分装） | 细胞上清、LDH harvest、qPCR 分装 | shared conditional (`op.sample_aliquot` + 谱系注记) | S1-A 步骤 3；父→管谱系 |
| 孵育/保持（30 min / 1 h / 储存 ≤3 d） | 多家族 | shared (`op.incubation`) | S1-A 步骤 6/9/13；只断言 elapsed/条件 |
| 洗涤（保留语义） | 多家族 | shared (`op.washing`) | S1-A 步骤 7–8/10–11；去液力 UNKNOWN |
| 离心 | 传代、计数、Transwell | shared (`op.centrifugation`) | S1-A 400–600×g 5 min RT；转子/RPM 映射 UNKNOWN |
| Fc/染料配伍封闭 | 仅流式 panel | method-local | S1-A 步骤 2/4；effect 染料/受体依赖，不晋升 |
| 通用 "staining" | 表面多见但 effect 不稳定 | 不建通用模块 | 抗体/TDS 依赖；留方法内 |
| 获取/分选 | 仅流式 | method template 内步骤；不建共享 op | S1-A 步骤 14 无值；分选谱系/得率缺口见 §9 |

## 3. P-FLOW-IC (S2-A/B/C)

绑定摘要：FVD 可选（路由 P-FLOW-VIAB）+ 表面染（路由 P-FLOW-SURFACE）→ 固定：A：100 µL IC Fixation（管）/ 200 µL（板），20–60 min RT 暗；B：Foxp3 工作液（1 份浓缩 + 3 份稀释）1 mL/管 / 200 µL/孔，30–60 min 2–8 °C 或 RT 暗（mouse ≤18 h 2–8 °C）；C：等体积 IC Fix 直接加样终止刺激（管）/ 200 µL（板），10–60 min RT 暗 → 通透洗：A/B 用 1X Perm（10X 1:9 现配，每样 8.5 mL 管装量级）2 mL/管或 200 µL/孔，400–600×g 5 min RT ×2；C 用冰冷 90–100% MeOH 1 mL/管或 100 µL/孔，≥30 min 2–8 °C（MeOH 中 ≤−20 °C 存 ≤4 周；板印刷 "≤20 °C" 存疑，见缺口）→ 可选 2% 血清封闭 2 µL 15 min RT（B 板；A 管未列）→ 胞内染（须在 Perm 存在下）：A 100 µL Perm + 直标一抗 20–60 min RT；B ≥30 min RT；C 30–60 min RT（表面可同步染，须耐固定/耐 MeOH 克隆）→ Perm 洗 ×2 → 重悬上机。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| FVD 可选 + 表面染 | 路由 P-FLOW-VIAB / P-FLOW-SURFACE | 悬液 → 已表面染色管 | 方法绑定：FVD 逢固定必选 | S2-A/B 管/板步骤 2–3 |
| 固定 A：100 µL（管）/ 200 µL（板）IC Fix，20–60 min RT 暗 | method-local `fix-IC` + `op.incubation` 参数化 | 已表面染色 → 已固定 | 方法绑定：100 / 200 µL；20–60 min RT 暗 | S2-A 管步骤 5–6 / 板步骤 5–6 |
| 固定 B：Foxp3 工作液 1 mL/管 / 200 µL/孔，30–60 min（mouse ≤18 h） | method-local `fix-perm-B` + `op.incubation` 参数化 | 已表面染色 → 已固定通透 | 方法绑定：1:3 现配；1 mL / 200 µL；30–60 min 2–8 °C/RT；mouse ≤18 h | S2-B 缓冲配制 + 管步骤 5–6 / 板步骤 5–6 |
| 固定 C：等体积 IC Fix 终止刺激，10–60 min RT | method-local `fix-stop` + `op.incubation` 参数化 | 已刺激细胞 → 已固定 | 方法绑定：等体积；10–60 min RT；未刺激 37 °C 阴性对照 | S2-C 管步骤 5–6 / 板步骤 6–7 |
| 通透洗 A：1X Perm 2 mL/管或 200 µL/孔，400–600×g 5 min ×2 | `op.washing` + `op.centrifugation` 参数化 | 已固定 → 已通透 | 方法绑定：1:9 现配（每样 8.5 mL 量级）；2 mL / 200 µL；400–600×g 5 min；×2 | S2-A 管步骤 7–8 / 板步骤 7–9 |
| 通透 B：同上（1 mL 工作液 + Perm 洗 ×1–2） | 同上 | 已固定通透 → 待染 | 方法绑定：1 mL/管 / 200 µL/孔；洗 ×1–2 | S2-B 管步骤 7–8 / 板步骤 7–9 |
| 通透 C：冰冷 90–100% MeOH 1 mL/管或 100 µL/孔，≥30 min 2–8 °C | method-local `perm-MeOH` + `op.incubation` 参数化 | 已固定 → MeOH 通透 | 方法绑定：90–100% HPLC 级；1 mL / 100 µL；≥30 min；储存 ≤4 周（温度印刷冲突见缺口） | S2-C 管步骤 8 / 板步骤 9 + 注记 |
| 可选血清封闭 2 µL 15 min RT（B 板；A 板同） | method-local `block-serum` + `op.liquid_transfer` | 已通透 → 已封闭 | 方法绑定：2 µL 2% 血清；15 min RT；洗前不洗直接加抗体 | S2-A 板步骤 10；S2-B 管步骤 10 / 板步骤 11 |
| 胞内染：A 20–60 min / B ≥30 min / C 30–60 min RT 暗（Perm/染色缓冲中） | method-local `stain-IC` + `op.incubation` 参数化 | 已通透 → 已胞内染色 | 方法绑定：~100 µL 体积；抗体量按 lot；C 表面可同步（耐 MeOH/耐固定克隆表） | S2-A 管步骤 9 / 板步骤 11；S2-B 管步骤 11 / 板步骤 12；S2-C 管步骤 13 / 板步骤 13 + MeOH 表 |
| Perm/染色缓冲洗 ×2 → 重悬 → 上机 | `op.washing` + `op.centrifugation` 参数化；上机同 P-FLOW-SURFACE | 已染 → 待测悬液 + 原始 FCS | 方法绑定：2 mL/管或 200 µL/孔 ×2（C 用染色缓冲 2 mL/200 µL） | S2-A 管步骤 10–12 / 板步骤 12–14；S2-C 管步骤 14–16 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/洗涤/离心 | 多家族 | shared/参数化 | S2 三协议体积/×g/时间绑定；MeOH 量另立 |
| 固定/通透/封闭（IC/Foxp3/MeOH/血清） | 仅流式胞内分支 | method-local（三协议各留方法内） | 缓冲系靶点依赖（胞质→A；核→B；磷酸→C）；effect/终点不同，不合并、不晋升 |
| 通用 "fix/perm/block/stain" | 表面多见但 effect 不稳定 | 不建通用模块 | 与 IF 的固定/通透/封闭不合并（试剂/终点/可逆性不同），见 §8 |
| 胞内 panel 选择（A/B/C 路由） | 仅流式 | 方法模板内分支 | S2 引言靶点-缓冲映射；错配 = 重做 |

## 4. P-FLOW-VIAB (S3-A/B/C1–C5)

绑定摘要：A（PI/7-AAD，非固定表面染专用）：表面染后 1–2× 洗 → 5 µL/100 µL 细胞，5–15 min 冰/RT，**禁洗（染料须留存至上机）**，≤4 h 内分析，2–8 °C 避光保存。B（calcein 活细胞）：1–5×10⁶ / 0.1–1 mL + lot 浓度染料 → 30 min RT 暗 → 2 mL 洗 400–600×g 5 min ×2 → 可选表面染 → 上机（可复染 PI/7-AAD；禁胞内流程）。C（FVD，固定兼容）：C1 管：2× 无叠氮无蛋白 PBS 洗 → 1–10×10⁶/mL（≥0.5 mL）→ 1 µL FVD/mL 立即涡旋 → 30 min 2–8 °C 暗 → 1–2× 染色缓冲洗；C2 板：1:1,000 工作液 100 µL/孔同孵同洗；C3 全血：1 µL/100 µL 血 + 表面抗体同孵 30 min；C4 含蛋白/叠氮缓冲可染但减亮/加背景；C5 鸡尾酒现加 0.5–1 µL/样。FVD 补偿用单染细胞样；<5% 死细胞预期时 65 °C 1 min 热杀 + 冰 1 min 对半掺入作对照。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| A：表面染后 1–2× 洗 → 重悬 | `op.washing` 参数化（路由 surface） | 已表面染色 → 待活力染 | 方法绑定：1–2× 染色缓冲洗 | S3-A 步骤 1–2 |
| A：5 µL/100 µL，5–15 min 冰/RT，禁洗，≤4 h 上机 | method-local `stain-PI/7-AAD` + `op.incubation` 参数化 | 待染 → 死染标记悬液（染料留存） | 方法绑定：5 µL/100 µL；5–15 min；禁洗是硬约束；≤4 h 时钟 | S3-A 步骤 3–5 + 注记 |
| B：1–5×10⁶ / 0.1–1 mL + lot 浓度 calcein，30 min RT 暗 | method-local `stain-calcein` + `op.incubation` 参数化 | 悬液 → 活染标记悬液 | 方法绑定：1–5×10⁶ / 0.1–1 mL；30 min；浓度按 TDS 滴定 | S3-B 步骤 2–4 + 总注 |
| B：2 mL 洗 400–600×g 5 min ×2 → 可选表面染 | `op.washing` + `op.centrifugation` 参数化 | 活染 → 已洗（→表面染） | 方法绑定：2 mL；400–600×g 5 min；×2 | S3-B 步骤 5–7 |
| C1：2× PBS 洗 → 1–10×10⁶/mL（≥0.5 mL）→ 1 µL/mL 立即涡旋 → 30 min 2–8 °C → 1–2× 洗 | method-local `stain-FVD` + `op.washing`/`op.incubation` 参数化 | 悬液 → FVD 标记悬液 | 方法绑定：无叠氮无蛋白 PBS 最亮；1 µL/mL；30 min 2–8 °C；≥0.5 mL 一致性 | S3-C 总注 + C1 步骤 1–7 |
| C2 板：1:1,000 工作液 100 µL/孔同孵同洗 | 同上（板变体） | 板孔 → FVD 标记孔 | 方法绑定：1:1,000；100 µL/孔；例 96 孔 10 µL/10 mL | S3-C2 步骤 1–7 |
| C3–C5 变体（全血/含蛋白缓冲/鸡尾酒现加） | 同上变体（method-local） | 各输入 → FVD 标记 | 方法绑定（变体）：1 µL/100 µL 血；C4 减亮警告；C5 0.5–1 µL/样现加 | S3-C3/C4/C5 步骤 |
| 热杀对照（<5% 死细胞预期） | 方法模板内对照 | 活悬液 → 掺入对照 | 方法绑定：65 °C 1 min → 冰 1 min → 1:1 掺入 | S3-C 总注 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/洗涤/离心/孵育 | 多家族 | shared/参数化 | S3-A/B/C1–C5 体积/×g/时间绑定 |
| 活力染色（PI/7-AAD / calcein / FVD） | 仅流式活力分支 | method-local（三分立，不合并） | effect/可逆性/固定兼容不同：A 禁固定+禁洗；B 禁胞内；C 固定兼容；G2 不过 |
| 通用 "viability stain/count" | — | 不建模块；判读另立 | 浓度/稀释/TDS 滴定系别依赖；上机门控 out-of-scope |

## 5. P-IF-PRE 固定/通透/封闭 (S4 步骤 1–7 + S5 步骤 1–10)

绑定摘要：贴壁细胞 60–80% 汇合（例 8 腔室玻片 30,000/腔，前日铺板）→ 固定二选一：4% 甲醛/PFA 15 min RT（S4：4% Formaldehyde；S5：1 mL 4% w/v / 6 孔单孔或 35 mm 皿），或 100% 冰 MeOH −20 °C 10 min；固定剂/时间须按抗原优化（S4 起始点声明）→ 3× PBS 洗 5 min/次（S4 滴管+抽吸；S5 侧壁加液+轻摇+倾斜去液），**此后禁干燥**（S4：每次最多抽 2 孔；S5 同），固定样 5 °C 存数天（天数未量化）→ 通透二选一（不合并）：S4：1% Triton X-100 / PBS，Parafilm 封 + 湿盒，37 °C 30 min + RT 10 min；S5：1 mL 0.5% Triton X-100 / PBS，15 min RT → 3× PBS 洗 → 封闭二选一：S4：100 µL/腔 10% 正常血清（二抗宿主种属）+ 0.1% Triton 于 PBS/TBS，湿盒 RT 1 h；S5：2 mL 3% w/v BSA / PBS，RT ≥60 min（可过夜；robust 系可缩至 30 min）。PBS 配方 S4：137 mM NaCl / 2.7 mM KCl / 10 mM Na₂HPO₄ / 1.76 mM KH₂PO₄ pH 7.4；TBS：50 mM Tris / 150 mM NaCl pH 7.4。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 铺板至 60–80%（例 30,000/腔） | 前置条件（培养），不拆解 | 培养细胞 → 待固定单层/贴壁 | 方法绑定：60–80%；例 30,000/腔；前日铺板 | S4 步骤 1 |
| 去培养基 → 加固定液（S5 1 mL 4% FA；S4 同浓度/MeOH 选项） | `op.liquid_transfer` 参数化 + method-local `fix` | 待固定 → 已加固定液 | 方法绑定：1 mL（S5 体积；S4 腔室体积未给）；4% FA 或 100% 冰 MeOH | S4 步骤 2 + 材料；S5 步骤 1–2 |
| 固定保持 15 min RT 或 −20 °C 10 min（抗原依赖优化） | `op.incubation` 参数化 + method-local `fix` | 已加固定液 → 已固定 | 方法绑定：15 min RT / 10 min −20 °C；最优须逐抗原优化 | S4 步骤 2 + 注记；S5 步骤 3 |
| 3× PBS 洗 5 min/次 + 禁干燥（≤2 孔/次） | `op.washing` 参数化（禁干约束） | 已固定 → 已洗固定样 | 方法绑定：3×；5 min/次；滴管/侧壁手法；禁干 | S4 步骤 3 + 注记；S5 步骤 4 + 视频 |
| 通透 S4：1% Triton，湿盒 37 °C 30 min + RT 10 min / S5：1 mL 0.5% Triton 15 min RT | method-local `perm`（两选项分立）+ `op.incubation` 参数化 | 已洗固定样 → 已通透 | 方法绑定：1% vs 0.5%；37 °C 30+10 min vs RT 15 min；Parafilm/湿盒；冲突不合并 | S4 步骤 4 + 注记；S5 步骤 5–6 |
| 3× PBS 洗 | `op.washing` 参数化 | 已通透 → 待封闭 | 方法绑定：3×（S5 同手法） | S4 步骤 5；S5 步骤 7 |
| 封闭 S4：100 µL/腔 10% 血清 + 0.1% Triton 1 h RT 湿盒 / S5：2 mL 3% BSA ≥60 min RT | method-local `block`（两选项分立）+ `op.incubation` 参数化 | 已通透 → 已封闭 | 方法绑定：100 µL/腔 vs 2 mL；10% 血清（二抗宿主）vs 3% BSA；1 h vs ≥60 min（robust 30 min） | S4 步骤 6 + 材料；S5 步骤 8–9 + tip |
| 3× PBS 洗（S4） | `op.washing` 参数化 | 已封闭 → 待染 | 方法绑定：3× 5 min/次 | S4 步骤 7 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（去液/加固定/通透/封闭/洗液） | 多家族 | shared (`op.liquid_transfer`) | S4/S5 体积绑定；S4 腔室外体积 UNKNOWN |
| 孵育/保持（固定/通透/封闭时钟） | 多家族 | shared (`op.incubation`) | S4/S5 时间/温度绑定；成胶/生长类推断不在内 |
| 洗涤（3× PBS） | 多家族 | shared (`op.washing`) | S4/S5 3× 5 min；干燥 = 失效约束 |
| 固定/通透/封闭 | IF 内两选项各自成立；跨流式不稳定 | method-local（S4/S5 选项分立） | Triton 1% vs 0.5%、血清 vs BSA effect/后果不同；G2/G4 不过，不建通用模块 |
| 通用 "fix/perm/block" | 表面多见但 effect 不稳定 | 不建通用模块 | 终点不可逆性 + 下游后果见 §9 |

## 6. P-IF-STAIN 染色/封片/原始成像 (S4 步骤 8–15)

绑定摘要：一抗按厂商推荐稀释于封闭液，湿盒 RT 2 h（低丰度 ≤12 h 4 °C）→ 3× TBS 洗 5 min → 二抗路由：一抗非标记 → 3-step（生物素化二抗 1 h + 链霉亲和素 30 min）；一抗生物素化/荧光直标 → 2-step（二抗 1 h，**荧光二抗须避光且封闭液去 Triton**；eFluor 纳米晶体配 TBS 信号最强）→ 3× PBS/TBS 洗 5 min → 链霉亲和素（仅 3-step）：eFluor 605NC/650NC/FITC 于无 Triton 10% 血清/TBS，30 min RT（605NC 起始 1:100）→ 3× TBS 洗 5 min → Fluoromount-G/Fluoro-Gel 封片 → 4 °C 避光存 → 原始荧光采集（每（视野，通道）记通道/物镜/曝光/cube/仪器 ID；只到原始帧）。

### 表 1：步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 一抗（推荐稀释，封闭液配）湿盒 RT 2 h（低丰度 ≤12 h 4 °C） | method-local `stain-primary` + `op.incubation` 参数化 | 已封闭 → 已一抗 | 方法绑定：2 h RT；低丰度 ≤12 h 4 °C；稀释按 lot | S4 步骤 8 + 材料 + 注记 |
| 3× TBS 洗 5 min | `op.washing` 参数化 | 已一抗 → 待二抗 | 方法绑定：3× TBS 5 min/次 | S4 步骤 9 |
| 二抗路由（生物素化/荧光直标二抗，推荐稀释）1 h RT 湿盒；荧光须避光 + 去 Triton | method-local `stain-secondary` + `op.incubation` 参数化 | 已一抗 → 已二抗 | 方法绑定：1 h RT；荧光避光 + 封闭液去 Triton；TBS 配 eFluor 条件注记 | S4 步骤 9–10 + 注记 |
| 3× PBS/TBS 洗 5 min | `op.washing` 参数化 | 已二抗 → 待显色/封片 | 方法绑定：3× 5 min | S4 步骤 11 |
| 链霉亲和素（仅 3-step）无 Triton 血清/TBS 30 min RT（605NC 起始 1:100） | method-local `stain-streptavidin` + `op.incubation` 参数化 | 生物素化二抗 → 已标记 | 方法绑定：30 min RT；1:100 起始（605NC）；2-step 禁走此步 | S4 步骤 12 |
| 3× TBS 洗 5 min → 封片（Fluoromount-G/Fluoro-Gel）→ 4 °C 避光存 | `op.washing` 参数化 + method-local `mount` | 已标记 → 已封片存档 | 方法绑定：3× TBS 5 min；封片剂二选一；4 °C 避光 | S4 步骤 13–15 |
| 原始荧光采集（每视野×通道） | `FLUO_IMAGE` 参数化（固定终点） | 已封片 → 原始图像集 + 存档片 | 方法绑定：通道 = 实际缀合物；曝光/物镜/cube 逐帧记录；无默认值 | S4 步骤 15 + 通道条件（S4 材料/步骤 10/12）；仪器值 UNKNOWN |
| 二抗缺失/自发荧光对照 + 抗原固定优化矩阵 | 方法模板内对照 | 同批片 → 可比基线 | 方法绑定：secondary-only + unstained 同条件采集；曝光/聚焦可比 | S4 步骤 2 注记（优化）+ 通用对照要求 |

### 表 2：公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移（加一抗/二抗/链霉亲和素/洗液/封片） | 多家族 | shared (`op.liquid_transfer`) | S4 步骤 8/10/12/14；稀释按 lot |
| 孵育/保持（2 h / 1 h / 30 min / 储存） | 多家族 | shared (`op.incubation`) | S4 时间/温度绑定；只断言 elapsed |
| 洗涤（3× TBS/PBS） | 多家族 | shared (`op.washing`) | S4 3× 5 min 多轮 |
| 一抗/二抗/链霉亲和素/封片 | 仅 IF | method-local（四步分立） | 结合 effect/路由/去 Triton 约束不同；G1/G2 不过 |
| 荧光图像采集（原始） | LIVE/DEAD、划痕、Transwell、IF、3D | shared (`FLUO_IMAGE`，固定终点注记) | S4 通道/封片绑定；判读/定量 out-of-scope |
| 通用 "staining"/"mount" | 表面多见但 effect 不稳定 | 不建通用模块 | CV/DAPI/抗体/活力染料 effect 不同；封片存档语义另立 |

## 7. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 分装/加封闭/加抗体/加洗液/加固定·通透·封闭·染料/封片 | `op.liquid_transfer` | 参数化复用；绑定体积/试剂/孔板（§§2–6） |
| 表面/胞内/活力/固定/通透/封闭/一抗/二抗保持 + 储存窗 | `op.incubation` | 参数化复用；绑定温度/时长/锚点；生物学完成不断言 |
| 管/板/膜/片洗涤（PBS/TBS/Perm/染色缓冲） | `op.washing`（保留语义） | 参数化复用；禁干/禁洗（PI/7-AAD）为硬约束；脱落 loss 显式 |
| 离心（400–600×g 5 min RT 档） | `op.centrifugation` | 参数化复用；转子/RPM 映射 UNKNOWN |
| 分装到管/孔（50 µL 档） | `op.sample_aliquot` | 参数化复用；父→管谱系 |
| IF 原始荧光采集 | `FLUO_IMAGE`（固定终点） | 参数化复用；只到原始帧；通道/曝光/物镜逐帧记录 |
| 流式获取/分选 | 方法模板内步骤 | 不建共享 op；原始 FCS + 分选谱系；仪器值 UNKNOWN |
| 表面/胞内/活力/Fc/染料配伍/固定/通透/封闭/一抗/二抗/链霉亲和素/封片 | method-local | 各留方法内；跨流式×IF 不合并、不晋升 |
| 稀有事件数/活力%/阳性%/补偿/设门/图像定量/统计 | ToolAction 或 out-of-scope | 一律非操作 |

## 8. 跨方法公共操作覆盖矩阵

| 公共操作 | P-FLOW-SURFACE | P-FLOW-IC | P-FLOW-VIAB | P-IF-PRE | P-IF-STAIN | 备注 |
|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 分装/封闭/抗体/洗液/储存 | ● 固定/Perm/抗体 | ● 染料/洗液/工作液 | ● 固定/通透/封闭/洗液 | ● 一抗/二抗/亲和素/封片 | 最高复用；全参数化 |
| `op.incubation` | ● 30 min/1 h/≤3 d | ● 10–60 min + ≥30 min | ● 5–15 min/30 min | ● 10–15 min + 15–60+ min | ● 2 h/1 h/30 min | 只断言 elapsed/条件 |
| `op.washing` | ● 2 mL/200 µL ×2 | ● Perm ×2 | ● PBS/染色缓冲 1–2× | ● 3× PBS 5 min | ● 3× TBS/PBS 5 min | 禁干/禁洗硬约束保留 |
| `op.centrifugation` | ● 400–600×g 5 min | ● 同左 | ● 同左 | —（贴壁无离心） | — | 转子映射 UNKNOWN |
| `op.sample_aliquot` | ● 50 µL 分装 | ○ 板孔分装 | ○ 管/板/血分装 | — | — | 流式为谱系关键步 |
| `FLUO_IMAGE`（原始） | — | — | — | — | ● 固定片多通道 | 分析另立；活/固定不合并 |
| method-local 染色/固定/封闭 | ● Fc/染料/抗体 | ● IC/Foxp3/MeOH | ● PI/calcein/FVD | ● FA-MeOH/Trition/血清-BSA | ● 一抗/二抗/亲和素/封片 | 各留方法内；见 §9 G-5 |
| 获取/分选/派生 | ● FCS/分选（模板） | ● FCS（模板） | — | — | ● 原始帧（共享）+ 定量 out | 补偿/设门/定量一律非操作 |

●=主路径使用；○=变体语境使用；—=本批未使用。

## 9. 缺口清单(不能表达/UNKNOWN；分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | 流式：unstained + 每荧光单染（spillover/补偿）+ 每门控通道 FMO（门边界）+ 每 run 活力（固定 run 用 FVD；PI/7-AAD 仅未固定表面 run）+ Fc 封闭验证群；isotype 只作有限背景语境，禁代 FMO。IF：secondary-only + unstained + 逐抗原固定/通透优化矩阵；曝光/聚焦跨对照可比否则比较无效。任一缺即判读无效 |
| G-2 | 获取/分选仪器值 | `configuration_dependent` + `source_missing` | 激光线/滤光/增益/阈值/事件数/质控限、喷嘴/压力/drop-delay/收集液/纯度标准、分选 ≤4 群/无菌路径/计数器——S1–S5 均无值；facility SOP + build sheet 未读；plan 前须逐 run 绑定，否则禁排 |
| G-3 | 印刷冲突（未裁决） | `source_missing` | S5 通透 0.5% vs S4 1%（37 °C 30+10 min vs RT 15 min）分立保留，不合并；S2-C 板 "≤20 °C" vs 管 "<−20 °C" 疑缺负号，不猜，按 kit/lot 手册核对 |
| G-4 | 系别/lot 依赖值 | `method_dependent` | 全部抗体克隆/稀释/滴定终点、calcein 工作浓度、FVD lot µM、polymer 互作矩阵、刺激/磷酸化动力学、事件数目标、MeOH 耐受表外荧光——按 lot/TDS 绑定，本批无默认值 |
| G-5 | 终点不可逆/不兼容 | 跃迁约束 | 固定终结回培养；PI/7-AAD 染后禁固定；FVD 逢固定必选；NovaFluor 禁 DAPI/PI/SYTOX；MeOH 敏感染料禁走 C；polymer 无 blocker 呈欠补偿貌；胞内染须 Perm 存在；荧光二抗须避光 + 封闭液去 Triton；2-step 禁加亲和素 |
| G-6 | 力学/手法/设备态 | `source_missing` / `device_state` | 吹打/涡旋强度、弃液残留、抽吸遍数、转子 ×g↔RPM、离心机冷藏档、山羊/鼠血清种属匹配、"数天"储存上限、封片固化时间——原文无量化，归 SOP/操作者判断 |
| G-7 | 时间锚点 | 锚点缺口 | 表面 30 min `per_assay`；IC 固定+染 clocks 累加；PI/7-AAD 加样→上机 ≤4 h 硬时钟；储存 ≤3 d 独立保持窗；MeOH ≥30 min + ≤4 周储存窗；孵育不断言结合/通透/封闭完成 |
| G-8 | 范围外（误建模块风险） | `not_applicable` | 补偿矩阵、设门策略、%阳性/MFI/纯度/得率、图像分割/共定位/计数、统计/拟合、RBC 全血深分支（S1-B）、组织切片、活细胞 IF（走 `FLUO_IMAGE (live)`，不并入本批固定像）、流式前处理/解离上游——本批一律不记录、不建模 |
| G-9 | 晋升结论 | 审计结论 | 四维粒度 + 晋升门（≥2 独立家族/语义稳定/抽象收益/分支成本）全检：通用 stain/fix/perm/block 止于 method-local（G2/G4 不过）；流式获取/分选止于 method template；本批新增公共模块数 = 0 |

## 10. 核查与状态

- 已验证（直接读，2026-11-05，§1 定位节）：S1-A 全文；S2-A/B/C 管+板；S3-A/B/C1–C5；S4 步骤 1–15 + 配方；S5 步骤表 1–10 + 体积。单人 `agent-verified`，仍需第二人逐字段复核；未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证：S1-B 全血深分支、BD/FACSAria 二手摘要、任何仪器手册/单机构建表、转子映射、per-lot 片、实验室 SOP 改编、滴定终点、任何已跑数据的派生应用。
- 记录清单：每值有定位或标 UNKNOWN；方法要求与仪器规格分离（§§2–6 仅能力类别 + 用过体积作例）；样本跃迁有端口（§§2–6 表 1）；时间锚显式；工作值标协议归属；部分列表标部分；plan/actual/raw/derived 分离（原始 FCS/图像保留，派生下游）；无 JSON；UNKNOWN 永不填零。
