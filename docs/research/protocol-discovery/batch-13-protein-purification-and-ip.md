# Batch 13 — Protein Preparation: Lysis / Total-Protein Quant / Affinity Purification / Immunoprecipitation / Dialysis-Concentration

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 蛋白制备链 5 个湿实验 protocol 组:裂解 (RIPA, 贴壁/悬浮双分支)、总蛋白定量 (BCA 微孔板主程序 + Bradford 对照变体)、His 标签亲和纯化 (Ni-NTA spin/batch)、免疫沉淀 (native IP, Protein A/G)、透析/缓冲液置换/浓缩 (Slide-A-Lyzer 盒式透析)。只记录湿实验步骤与原始读数;制备止于澄清裂解液/定量值/洗脱组分/透析后样品。
- 规则: 制备与检测分离 — ELISA/WB 检测链已由 B05 覆盖,本批下游 SDS-PAGE/WB/激酶测定/标准曲线拟合/densitometry/统计一律记为 out-of-scope 或 ToolAction,不建操作;亲和纯化与免疫沉淀分立记录,不合并;现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;不新增公共模块除非四问 + 晋升四门全过,domain-shared 允许但本批无晋升;不做代码/UI/DB/JSON 变更。
- 禁止声明: 本文件不含任何通用裂解体积、通用蛋白浓度、通用咪唑浓度、通用抗体量、通用透析时间;所有温度/时间/体积/浓度/离心条件均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| S1 | Thermo Fisher RIPA Lysis and Extraction Buffer 产品说明书 MAN0011565 (89900/89901) + WB Sample Preparation 页 | 厂商手册 | RIPA 组成/贴壁与悬浮双程序/抑制剂注记 | 检索转写级:贴壁洗涤→冷 RIPA→冰 5 min→刮取→14,000×g 15 min;悬浮 2,500×g 5 min 集菌→PBS 洗→RIPA 重悬→冰 15 min→14,000×g 15 min;可选超声 30 s (50% pulse);全文 PDF 未逐页直读,exact 离心转子/超声探头型号 UNKNOWN |
| S2a | Thermo Pierce BCA Protein Assay Kit 用户指南 MAN0011430 (23225) 微孔板程序 | 厂商手册 | WR 配制/标准系列/微孔板 25 µL + 200 µL WR/37 °C 30 min/562 nm | 检索转写级:样品:WR=1:8 (25 µL + 200 µL),样品受限 1:20 (10 µL) 范围收窄;标准 20–2,000 µg/mL,增强管式 5–250 µg/mL;板振 30 s;冷却至 RT 后 562 nm (540–590 可接受);blank 扣除 + 标准曲线为 ToolAction |
| S2b | Bio-Rad Protein Assay (Bradford, LIT33) 标准程序 + Thermo Bradford 页 | 厂商手册 | 染料 1:4 稀释/595 nm/标准系列 | 检索转写级:浓缩染料 1:4 水稀释,Whatman #1 过滤,2 周内 RT 用完;BSA 线性 0.2–0.9 mg/mL,IgG 0.2–1.5 mg/mL;RT ≥5 min 且 ≤1 h 内读 595 nm (575–615 可接受);exact 微孔板体积配比按 lot 说明,本批 UNKNOWN |
| S3 | Thermo HisPur Ni-NTA Purification Kit MAN0011702 + Spin Columns MAN0011699 + Resin MAN0011700 (batch 法) | 厂商手册 | Spin 柱 0.2/1/3 mL + batch 管式;平衡/结合/洗 3×/洗脱 3× | 检索转写级:贮存液 700×g 2 min 去除;2 树脂床体积平衡;上样 (extract + 等体积平衡液,总量 ≥2 床体积),可选结合至 30 min;洗 2 床体积×3 组分;洗脱 1 床体积×3 组分 (batch 法 2 床体积);A280 或 Bradford 监测;咪唑去除另立透析/凝胶过滤 |
| S4 | Cell Signaling Technology Immunoprecipitation Protocol for Native Proteins (Protein A/G agarose) + Product 9863 注记 | 厂商手册/protocol | 预澄清/一抗 4 °C overnight/20 µL 50% beads 1–3 h 4 °C/5×500 µL 洗 | 检索转写级:200 µL 裂解液 (1 mg/mL) 预澄清 30–60 min 4 °C;一抗 4 °C overnight (易降解 2–4 h);20 µL 50% Protein A/G slurry 1–3 h 4 °C;沉淀后 1×裂解液 500 µL×5 洗;3× SDS 上样缓冲液洗脱接 WB/激酶 (下游 out-of-scope);isotype 对照必备 |
| S5 | Thermo Slide-A-Lyzer 透析盒技术支持页 (dialysis/desalting/concentration getting-started) + 盒页 | 厂商技术页 | 盒式透析/脱盐/浓缩:上样→3×1 h 换液 (200 倍体积比) + 可选 4 °C overnight | 检索转写级:第二代盒 0.5–70 mL,注射器/移液器上样,打气展开腔体,浮标透析;典型 3×1 h RT + 200 倍样品:缓冲液比;exact MWCO/膜再生/浓缩器型号按 lot,本批 UNKNOWN;自旋脱盐 (Zeba) 仅记存在,不拆执行 |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 取样/分样 sampling atomic (谱系,守恒缺口);R3 `op.incubation` (elapsed/条件 only,不断言裂解/结合/生长完成);R4 `op.washing` (保留语义) + lossy 变体 (潜在损失显式);R5 `op.centrifugation` (RCF/时间/转子绑定);R6 连续稀释=Composite (多次转移);R7 `absorbance_measurement` (raw OD only, blank/λ 方法绑定);R8 `FLUO_IMAGE`/发光读数与本批无关;审计 `#23` 通用染色、`#24` 解离、`#26` 固定/封闭、`#27` 定量不单原子 (blank/单位/方法约束不同,作方法绑定读数) 与本批直接相关。

检索摘要 (`search-summary-only`,非 `SourceChecked` 证据):Fortis RIPA 页、Duke Bradford 微孔板页、Warwick Bio-Rad PDF 摘要、protocols.io HisPur/Bradford 页仅用于立项定位;凡数值以 §1 直接转写为准,未见值一律 UNKNOWN。

---

## 2. P-LYS — RIPA 裂解:贴壁与悬浮双分支 (S1)

绑定摘要:贴壁:弃培养基→冷 PBS 洗 2×→冷 RIPA (75 cm² 瓶 1 mL,约 5×10⁶ 细胞) →冰 5 min + 轻摇→刮取转管→~14,000×g 15 min→收上清;可选沉淀超声 30 s (50% pulse) 增产。悬浮:2,500×g 5 min 集菌→弃上清→冷 PBS 洗→再 2,500×g 5 min→RIPA (1 mL / ~40 mg 湿重,约 5×10⁶ 细胞) 重悬→可选超声→冰 15 min 轻摇→~14,000×g 15 min→收上清。抑制剂 (Halt 蛋白酶/磷酸酶) 现配现加;需酶活时改用无脱氧胆酸/SDS 的温和配方 (本批只记存在,不给配方)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 弃培养基/集菌;贴壁冷 PBS 洗 2×;悬浮 2,500×g 5 min + PBS 洗 + 再离心 | `op.washing` 参数化 (洗) + `op.centrifugation` 参数化 (集菌) | 培养物 → 待裂解细胞/沉淀 | 方法绑定:2× PBS;2,500×g 5 min;exact PBS 体积/转子 UNKNOWN | S1 贴壁 1–2;悬浮 1–2 |
| 加冷 RIPA:贴壁 1 mL/75 cm² (~5×10⁶);悬浮 1 mL/~40 mg 湿重;抑制剂现加 | `op.liquid_transfer` 参数化 | 待裂解细胞 + RIPA → 裂解混合物 | 方法绑定:体积/细胞量比;抑制剂品种按 datasheet,exact 倍数 UNKNOWN | S1 两分支加液行 + 抑制剂注记 |
| 冰上保持:贴壁 5 min 轻摇;悬浮 15 min 轻摇;全程 on ice | `op.incubation` 参数化 (定时冷保持) | 裂解混合物 → 已作用混合物 | 方法绑定:5 vs 15 min 分支;只断言 elapsed/低温条件,不推断裂解完成 | S1 两分支 incubate 行 |
| 刮取收集 (贴壁);重悬/吹打 (悬浮);可选超声 30 s 50% pulse | method-local (`LYS-HARVEST`,机械收集/超声分支) | 已作用混合物 → 待澄清匀浆 | 方法绑定:刮取 vs 重悬分支;超声可选,exact 振幅/探头 UNKNOWN | S1 贴壁 3;悬浮 3 + sonicate 注记 |
| 澄清离心 ~14,000×g 15 min;收上清作分析;沉淀弃/留作复检 | `op.centrifugation` 参数化 + `sampling` (R2,上清新建,沉淀谱系) | 待澄清匀浆 → 澄清裂解液 (上清) + 沉淀 | 方法绑定:~14,000×g 15 min;exact 温度/转子 UNKNOWN | S1 两分支末离心行 |
| BCA 相容性注记;酶活需求改温和配方 | 方法约束,非步骤 | — | 方法绑定:相容性声明,exact 干扰阈值 UNKNOWN | S1 Notes + WB 页注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移 (PBS/抑制剂/RIPA 加样) | 多家族 | shared (R1) | S1 两分支;仅体积/品种绑定 |
| 离心 (集菌/澄清) | 多家族 | shared (R5) | 2,500×g vs 14,000×g 分支,转子/温度 UNKNOWN |
| 孵育/冷保持 | 多家族 | shared (R3) | 5 vs 15 min;只 elapsed,不推断裂解/得率 |
| 洗涤 (PBS 预洗) | 多家族 | shared (R4 保留语义) | 2×/1× 方法绑定;裂解后损失不记入洗涤 |
| 裂解/去污剂增溶 (detergent solubilization) | 本批 + S4 IP 裂解/S3 提取背景 | method-local,不新增 | 四问有独立目的/边界/确认,但 G2 不过:RIPA 变性 vs IP 非变性 vs 包涵体变性 effect 不兼容 (见 §9) |
| 超声辅助/刮取收集 | 仅本分支 | method-local 分支 | 单方法手法;G1 不过;作父操作备注,不建模块 |
| 得率/活性推断 | — (分析) | 非操作 (out-of-scope) | 无完成检查可断言产量/活性 |

---

## 3. P-QUANT — 总蛋白定量:BCA 微孔板 + Bradford 对照 (S2a/S2b)

绑定摘要:BCA 微孔板:标准系列 BSA (工作 20–2,000 µg/mL) + blank;25 µL 标准/未知 + 200 µL WR (样品受限 10 µL,范围收窄至 125–2,000);板振 30 s→37 °C 30 min→冷至 RT→562 nm (540–590 可接受);blank 扣除 + 标准曲线求浓度。Bradford 对照:染料 1:4 水稀释过滤;3–5 点标准 (BSA 线性 0.2–0.9 mg/mL,IgG 0.2–1.5 mg/mL);RT ≥5 min 且 ≤1 h 内 595 nm;两曲线法 (样品缓冲液 vs 水) 评估干扰为方法 QC。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| WR 配制 (BCA A:B=50:1,按需现配);染料 1:4 稀释过滤 (Bradford) | 方法内试剂准备,试剂加样复用 `op.liquid_transfer` | 母液 → 工作试剂 | 方法绑定:比例/过滤/2 周有效期;exact 总体积按板数,UNKNOWN | S2a WR 节;S2b LIT33 试剂节 |
| 标准系列 + blank:每 assay 现配;BCA duplicate/triplicate,管式/板式点数不同 | 方法内标准准备 (`sampling` 谱系 + 转移) | BSA 母液 → 标准系列 + blank | 方法绑定:浓度窗按程序 (标准/增强/限定样品三档);exact 稀释级数按 lot 表 | S2a Table 1/增强表;S2b 3–5 点节 |
| 加样:25 µL/孔 + 200 µL WR (1:8);受限 10 µL (1:20);Bradford 按程序加染料 | `op.liquid_transfer` 参数化 | 标准/未知 + WR/染料 → 待显色板 | 方法绑定:25/200 vs 10 µL 分支;板型 clear flat-bottom | S2a 微孔板 1–3;S2b 加样行 |
| 显色保持:BCA 37 °C 30 min + 冷至 RT;Bradford RT ≥5 min (≤1 h 上限) | `op.incubation` 参数化 (定时保持) | 待显色板 → 已显色板 | 方法绑定:37 °C/30 min vs RT/5 min–1 h;只 elapsed,不推断显色完成 | S2a 孵育行;S2b 时间窗行 |
| 读数:BCA 562 nm;Bradford 595 nm;均为原始吸光度矩阵 | `absorbance_measurement` (R7 raw only) | 已显色板 → 原始 OD 矩阵 | 方法绑定:λ/板色/读数窗;exact 读数延迟上限按 lot | S2a 562 nm 行;S2b 595 nm 行 |
| blank 扣除 + 标准曲线 → 样品浓度;干扰排查 (稀释/透析/沉淀/双曲线) | ToolAction (确定性计算) + 方法 QC | 原始矩阵 + 同板标准 → 派生浓度 | 方法模板内公式,非操作;无曲线不作浓度声明 | S2a 数据处理节;S2b 干扰节 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/吸光度原始读数 | 多家族 (B05 ELISA-TMB, B10 OD600, 本批) | shared (R1/R3/R7) | 本批仅绑定体积/时长/λ;派生另立 |
| 通用 "protein quantification" | 表面多见 (BCA/Bradford/A260,见冻结 #27) | 不建通用模块 | blank/单位/化学/干扰效应异质,G2/G4 不过;留方法绑定读数 (见 §9) |
| 双曲线干扰检查 | 仅定量 QC | method-local QC | 单方法质控;G1 不过 |

---

## 4. P-AFF — His 标签亲和纯化 Ni-NTA spin/batch (S3)

绑定摘要:柱平衡至 RT/4 °C;extract + 等体积平衡液 (总量 ≥2 床体积) 上样;去底塞,700×g 2 min 去贮存液;2 床体积平衡液平衡→离心;上样,可选混合结合至 30 min→离心收 flowthrough (超量分次上样);2 床体积洗液洗×3 组分;1 床体积洗脱液洗脱×3 组分 (batch 法 2 床体积);A280 或 Bradford/Coomassie 监测,SDS-PAGE 确证 (下游 out-of-scope);咪唑去除另立透析/凝胶过滤;树脂 MES/水洗后 50% 浆 20% 乙醇竖放存,至多 5 次复用。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 柱/树脂平衡至 RT 或 4 °C;去底塞置管;贮存液 700×g 2 min 去除 | `op.centrifugation` + `op.liquid_transfer` 参数化 | 干柱 → 已排空柱 | 方法绑定:700×g 2 min;温度二选一 | S3 spin 1–4 |
| 2 床体积平衡液平衡→离心去除 | `op.liquid_transfer` + `op.centrifugation` 参数化 | 已排空柱 → 已平衡柱 | 方法绑定:2 床体积;exact 缓冲液 pH/盐按 native/变性表 | S3 spin 5 |
| 上样:extract + 等体积平衡液 (总量 ≥2 床体积);可选混合结合至 30 min;大体积超量分次 | 方法内亲和结合分支 (`AFF-BIND`,见 §9) + 转移/保持参数化 | 澄清 extract + 已平衡柱 → 已结合柱 + flowthrough | 方法绑定:体积比/时长/温度;分次规则为方法注记 | S3 spin 6–7 + batch 注记 |
| 洗:2 床体积洗液×3,逐组分收集 | 方法内亲和洗涤分支 (`AFF-WASH`) | 已结合柱 → 已洗柱 + 洗组分×3 | 方法绑定:咪唑浓度按 native/变性表;exact 值 UNKNOWN | S3 spin 8–9 |
| 洗脱:1 床体积洗脱液×3 (batch 2 床体积),逐组分收集 | 方法内亲和洗脱分支 (`AFF-ELUTE`) | 已洗柱 → 洗脱组分×3 + 用后柱 | 方法绑定:高咪唑;exact mM 按表优化 | S3 spin 10 |
| A280/Bradford 监测;SDS-PAGE (下游);咪唑去除 (透析/凝胶过滤,见 §6) | 原始读数 + 下游 out-of-scope | 洗脱组分 → 原始 A280/OD + 派生判断 | 方法绑定;下游程序另立 | S3 监测行 + 去咪唑注记 |
| 树脂再生:MES 10 床体积→水 10 床体积→50% 浆 20% 乙醇存 (≤5 次) | 方法内再生分支 | 用后柱 → 可复用柱 | 方法绑定:体积/存液/次数上限 | S3 再生节 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/离心/孵育 (平衡/上样/结合保持) | 多家族 | shared (R1/R5/R3) | 仅体积/RCF/时长绑定 |
| 吸光度/染料监测 (A280/Bradford) | 多家族 | shared (R7) + 方法绑定读数 | 只到原始值;纯度/得率判读另立 |
| 通用 "affinity bind-wash-elute" | 仅 His-Ni-NTA 一家族 (本批) | method-local,不新增 | G1 不过;且与 IP effect/端口不同,不合并 (见 §9) |
| 树脂再生/复用 | 仅本方法 | method-local | 单方法维护;G1 不过 |

---

## 5. P-IP — Native 免疫沉淀 Protein A/G (S4)

绑定摘要:非变性裂解 (CST 建议 Cell Lysis Buffer #9803 而非 RIPA,保相互作用),冰上操作,短超声,高速澄清,−80 °C 可存;200 µL 裂解液 (1 mg/mL) + beads 预澄清 30–60 min 4 °C;一抗按 datasheet,4 °C overnight (易降解 2–4 h);20 µL 50% Protein A (兔 IgG) / G (鼠 IgG) slurry (生物素化抗体改 streptavidin 10 µL),1–3 h 4 °C 捕获;沉淀后 1×裂解液 500 µL×5 洗,冰上间隔;3× SDS 上样缓冲液洗脱接 WB/激酶 (下游 out-of-scope);同种 isotype IgG 对照必备。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 非变性裂解:冰冷裂解液裂解→短超声→高速澄清→定量至 1 mg/mL | §2 同类操作参数化 + method-local 裂解化学 (`IP-LYSIS`,非变性) | 细胞 → IP 级裂解液 | 方法绑定:非变性缓冲液品种;RIPA 禁用于保相互作用场景;exact 超声/RCF UNKNOWN | S4 裂解节 + troubleshooting RIPA 注记 |
| 预澄清:200 µL (1 mg/mL) + beads,30–60 min 4 °C,去珠 | 方法内预澄清分支 (`IP-PRECLEAR`) | IP 级裂解液 → 已预澄清裂解液 + 非特异组分 | 方法绑定:体积/浓度/时长;beads 品种按 IgG 种属 | S4 预澄清行 |
| 一抗结合:一抗 + 200 µL 裂解液,4 °C overnight (易降解 2–4 h) | 方法内抗体结合分支 (`IP-BIND`) + 保持参数化 | 已预澄清液 + 一抗 → 免疫复合物液 | 方法绑定:抗体按 datasheet;时长分支 | S4 一抗行 |
| 捕获:20 µL 50% Protein A/G slurry (生物素化改 10 µL streptavidin),1–3 h 4 °C | 方法内捕获分支 (`IP-CAPTURE`) | 免疫复合物液 + beads → 已捕获珠 | 方法绑定:bead 体积/品种/时长;种属匹配表 | S4 捕获行 + 9863 注记 |
| 洗:沉淀 + 1×裂解液 500 µL×5,冰上间隔 | `op.washing` 参数化 (保留语义;脱落场景 lossy 另记) | 已捕获珠 → 已洗珠 + 洗液 | 方法绑定:500 µL×5;exact 离心 (6,000 rpm ~1 min) 按珠说明 | S4 洗涤行 |
| 洗脱:3× SDS 上样缓冲液重悬→加热→上样 4–20% gel (WB/激酶下游) | 下游入口,本批不拆 | 已洗珠 → 洗脱物 (接检测) | 方法绑定;加热/胶浓度按下游 | S4 洗脱行 |
| isotype IgG 对照 (兔/鼠匹配) | 方法对照,非操作 | — | 方法绑定;缺即无意义 | S4 对照行 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/离心/洗涤 (裂解/结合/捕获/洗珠) | 多家族 | shared (R1/R3/R5/R4) | 仅体积/时长/缓冲液绑定 |
| 抗体–抗原结合/珠捕获/预澄清 | 仅 IP 一家族 | method-local,不新增 | G1 不过;与亲和纯化 effect/端口不同,不合并 (见 §9) |
| 通用 "capture"/"pull-down" | 表面多见但 effect 不稳定 | 不建通用模块 | His–Ni 配位 vs 抗体–Protein A/G vs streptavidin–biotin 机理/洗脱/对照异质,G2/G4 不过 |

---

## 6. P-DIA — 透析/缓冲液置换/浓缩 (S5)

绑定摘要:第二代盒 0.5–70 mL,注射器/移液器经角膜口上样,打气展开腔体后排气,浮标透析;典型 3×1 h RT + 200 倍样品:缓冲液比,可选 4 °C overnight 至完全;多余角膜口可作反应加料 (减样品转移损失);膜为再生纤维素,ABS 壳,硅胶垫,75% 乙腈/1% TFA 相容边界按说明;自旋脱盐 (Zeba) 仅记存在;浓缩器型号/截留量按 lot,本批 UNKNOWN。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 盒准备:打气展开腔体,浮标/台架就位;MWCO 按样品选 | 方法内准备分支 | 干盒 → 待上样盒 | 方法绑定:MWCO 按样品,exact 值 UNKNOWN | S5 上样节 |
| 上样:注射器/移液器经角膜口注入,排多余空气 | `op.liquid_transfer` 参数化 | 洗脱组分/样品 → 已上样盒 | 方法绑定:0.5–70 mL 按盒型 | S5 上样行 |
| 透析:3×1 h RT + 200 倍缓冲液比,换液;可选 4 °C overnight | `op.incubation` 参数化 (换液保持) + 转移 (换液) | 已上样盒 → 已置换样品 | 方法绑定:次数/时长/体积比;只 elapsed,不推断置换完全 | S5 条件行 |
| 回收:抽出样品,测体积/浓度 (BCA/Bradford,见 §3) | `sampling` (R2) + 方法绑定读数 | 已置换样品 → 回收样品 + 原始浓度 | 方法绑定;回收率 UNKNOWN | S5 回收注记 |
| 自旋脱盐/浓缩 (Zeba/超滤) | 存在注记,本批不拆 | — | 方法绑定;需独立 protocol 才可执行 | S5 脱盐节 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移/孵育/取样 (上样/换液/回收) | 多家族 | shared (R1/R3/R2) | 仅体积/时长/体积比绑定 |
| 透析/缓冲液置换 (膜交换) | 仅本家族 | method-local,不新增 | G1 不过 (单家族);G2 待第二家族 (超滤/凝胶过滤) 才重审 |
| 通用 "desalt/concentrate" | 表面多见但机理不同 | 不建通用模块 | 透析 vs 自旋凝胶过滤 vs 超滤 effect/时间/回收率异质 |

---

## 7. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| PBS 洗/抑制剂/RIPA/WR/染料/平衡/上样/抗体/beads/透析液加样 | `op.liquid_transfer` | 参数化复用;绑定体积/浓度/缓冲液/板色/床体积倍数 (§§2–6) |
| 冷裂解/显色/结合/捕获/透析保持 | `op.incubation` | 参数化复用;绑定时长/温度/振荡;只断言 elapsed/偏差,不推断裂解/结合/置换完成 |
| PBS 预洗/板洗模式/亲和洗/IP 珠洗/透析换液 | `op.washing` | 参数化复用;cycles/缓冲液/soak 方法绑定;脱落/珠损失场景 lossy 显式 |
| 集菌/澄清/排空/平衡/收组分/珠沉淀 | `op.centrifugation` | 参数化复用;绑定 RCF/时长 (2,500×g 5 min;14,000×g 15 min;700×g 2 min) |
| 上清/标准/洗脱组分/透析回收分样 | sampling atomic | 参数化复用;父样保留 + 子样新建;数量守恒 UNKNOWN (沿用缺口) |
| BCA 562 nm / Bradford 595 nm / A280 原始读数 | `absorbance_measurement` | 参数化复用;绑定 λ/板色/读数窗;blank/曲线方法绑定 |
| 裂解化学 (RIPA vs 非变性 vs 变性) | method-local (`LYS-HARVEST`/`IP-LYSIS`) | 留方法内;不合并、不晋升 (G2 不过) |
| 亲和结合/洗/洗脱;IP 预澄清/结合/捕获;透析膜交换;定量标准准备 | method-local 分支 | 各留方法模板内;不合并、不晋升 (G1 不过) |
| 标准曲线拟合/浓度换算/纯度/得率/酶活/亲和力、SDS-PAGE/WB/激酶下游、densitometry/统计 | ToolAction / out-of-scope | 一律非操作;无曲线/无下游不作浓度/表达声明 |

## 8. 跨方法公共操作覆盖矩阵

| 公共操作 | P-LYS RIPA 双分支 (S1) | P-QUANT BCA (S2a) | P-QUANT Bradford (S2b) | P-AFF Ni-NTA (S3) | P-IP native (S4) | P-DIA 透析 (S5) | 备注 |
|---|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● PBS/RIPA/抑制剂 | ● WR/标准 25+200 | ● 染料/标准 | ● 平衡/上样/洗/洗脱 | ● 裂解液/抗体/beads | ● 上样/换液 | 最高复用;全参数化 |
| `op.incubation` | ● 冰 5/15 min | ● 37 °C 30 min | ● RT 5 min–1 h | ● 结合至 30 min | ● overnight/1–3 h 4 °C | ● 3×1 h + 可选 overnight | 只 elapsed/偏差 |
| `op.washing` | ● PBS 预洗 | — (板洗见 B05) | — | ○ 方法内亲和洗分支 | ● 500 µL×5 珠洗 | ○ 换液 (转移+保持) | cycles/缓冲液方法绑定 |
| `op.centrifugation` | ● 2,500×g/14,000×g | — | — | ● 700×g 排空/平衡/收组分 | ● 珠沉淀 (~6,000 rpm 注记) | — | RCF/转子方法绑定 |
| sampling | ● 上清新建 | ● 标准/未知分样 | ● 标准/未知分样 | ● flowthrough/洗/洗脱组分 | ● 预澄清去珠/洗脱 | ● 回收集 | 守恒缺口沿用 |
| `absorbance_measurement` | — (相容性注记) | ● 562 nm | ● 595 nm | ○ A280/Bradford 监测 | — (下游 WB) | ○ 回收浓度 | 原始值止 |
| method-local (裂解/结合/捕获/透析/标准) | ● 双分支 + 超声 | ● WR/标准三档 | ● 染料/双曲线 QC | ● 结合/洗/洗脱/再生 | ● 预澄清/结合/捕获/isotype | ● MWCO/盒型/体积比 | 各留方法内;不合并 |
| ToolAction/下游 | ● 得率/活性另立 | ● 曲线拟合 | ● 曲线拟合 | ● 纯度/SDS-PAGE/去咪唑 | ● WB/激酶/isotype 判读 | ● 回收率 | 全部非操作 |

●=主路径使用;○=方法内分支/监测;—=本方法无此步。

方向警示:变性裂解 (RIPA) vs 非变性裂解 (IP #9803)——同一 "lysis" 词在不同方法下对相互作用/酶活的含义相反,不得共用配方。读数警示:BCA-562 (30 min 37 °C 时钟) vs Bradford-595 (RT 5 min–1 h 窗) vs A280——三者 blank/线性/干扰谱不同,相互不可换算。捕获警示:His–Ni 咪唑洗脱 (可再生复用) vs 抗体–Protein A/G SDS 洗脱 (isotype 对照定向)——同一 "pull-down" 值在不同机理下含义相反,不得共用解释。

## 9. 候选与四问判定 + 晋升审查 (本批结论:零新公共模块)

| 候选 | 独立目的 | 稳定边界 | 独立确认 | 跨方法复用 (≥2 独立家族) | 结论 |
|---|---|---|---|---|---|
| 去污剂裂解/增溶 (RIPA) | ✓ | ✓ | ✓ (澄清上清可确认) | ✗ (RIPA vs IP 非变性 vs 包涵体变性;G2 不过。S4 明确禁 RIPA 保相互作用) | method-local,不新增 |
| 总蛋白定量 (BCA/Bradford) | ✓ | ✓ | ✓ | ✗ (冻结 #27 已裁决:blank/单位/化学不同;G2/G4 不过) | 方法绑定读数,不新增 |
| 亲和结合–洗–洗脱 (Ni-NTA) | ✓ | ✓ | ✓ (组分可确认) | ✗ (仅 His 一家族) | method-local,不新增 |
| 免疫沉淀 (预澄清–结合–捕获–珠洗) | ✓ | ✓ | ✓ | ✗ (仅 IP 一家族) | method-local,不新增 |
| 透析/缓冲液置换 | ✓ | ✓ | ✓ | ✗ (仅盒式透析一家族;超滤/凝胶过滤未拆) | method-local,不新增 |
| 通用 "capture/pull-down" (亲和 + IP 合并) | ✓ | ✗ (配位 vs 抗体–Fc vs biotin–streptavidin;洗脱/对照/可逆性异质) | △ | ✗ (G2/G4 不过) | **拒绝合并**;分立记录 (题设要求) |
| 树脂再生/复用 | ✓ | ✓ | ✓ | ✗ (仅 His) | method-local |

晋升四门 (G1 复用广度 ≥2 独立方法,域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):本批无候选同时通过。亲和 vs IP 不合并的独立理由:端口不同 (Ni-NTA 柱/树脂 + 咪唑 vs agarose 珠 + 抗体 + isotype 对照)、样本 effect 不同 (重组标签可逆配位洗脱、可复用 vs 天然复合物抗体捕获、SDS 终点洗脱、预澄清去非特异)、完成检查不同 (A280 组分 vs WB/激酶下游 + isotype)。合并将隐藏机理差异,违反 G2/G4。

结论:本批零新公共模块;对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer` (+裂解/定量/纯化/IP/透析五类绑定)、`op.incubation` (+冷裂解/显色/结合/捕获/透析保持)、`op.washing` (+PBS 预洗/IP 珠洗)、`op.centrifugation` (+集菌/澄清/排空/收组分)、sampling (+上清/组分/回收谱系)、`absorbance_measurement` (+562/595/A280 三绑定)。

## 10. 缺口清单 (不能表达/UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 类别 | 说明 |
|---|---|---|---|
| G-1 | 对照矩阵不可省 | 方法绑定缺口 | 定量:同板标准系列 + blank (每 assay 现配);Bradford 双曲线 (样品缓冲液 vs 水);亲和:flowthrough/洗组分/SDS-PAGE 参照;IP:isotype IgG + no-Ab 珠对照 + input;透析:透析前后体积/浓度对照。任一缺即无意义 |
| G-2 | exact 配方/浓度未取 | `source_missing` | RIPA 抑制剂倍数、Bradford 微孔板 exact 体积配比、BCA 增强/限定三档 exact 稀释级数、Ni-NTA native/变性咪唑 exact mM、IP 抗体量/超声/RCF exact 值、透析 MWCO/浓缩器型号——按 lot/datasheet,本批 UNKNOWN |
| G-3 | 力学/手法量 | `source_missing` | 刮取力度、重悬吹打强度、板振之外的混匀、tap dry/珠重悬轻柔度、盒排气手法——原文定性,归 SOP 操作者判断 |
| G-4 | 派生公式归属 | ToolAction,非操作 | 标准曲线拟合、blank 扣除、CFU 式类比的浓度换算、纯度/得率/回收率、酶活/亲和力、densitometry/归一化——全部下游;无曲线不作浓度声明 |
| G-5 | 系别/批依赖值 | `method_dependent` + `configuration_dependent` | 裂解体积/细胞量比按系优化;BCA/Bradford 干扰谱按缓冲液 (去污剂/还原剂/螯合剂);Ni-NTA 平衡/洗咪唑按蛋白滴定;IP 抗体/bead 量按 datasheet;透析时间按样品实测 |
| G-6 | 不可逆/终点约束 | 跃迁约束 | Bradford >1 h 不读;BCA 延迟读数漂移;chemi 类比不适用本批;SDS 洗脱后珠不可复用 (vs Ni-NTA 可再生 ≤5 次);azide/螯合剂/强还原剂禁用于 Ni-NTA;AP/酸失活类比见 B05 |
| G-7 | 时间锚点 | 锚点缺口 | 裂解 `t₀`=RIPA 加入;定量 `t₀`=WR/染料加入 (BCA 37 °C 30 min / Bradford RT 窗);亲和 `t₀`=上样结合开始;IP `t₀`=一抗加入 (预澄清为前置);透析 `t₀`=上样入盒。孵育不断言裂解/结合/置换完成 |
| G-8 | 范围外 (误建模块风险) | `not_applicable` | SDS-PAGE/转膜优化、WB 检测 (B05)、激酶测定、质谱/测序、病毒/临床/动物、图像分割/统计管线、通用 "lysis/quantify/purify/IP/dialyze" 模块——本批一律不建模 |
| G-9 | 损失/守恒未量化 | 合同缺口 (沿用) | 裂解不完全/膜蛋白丢失、珠/柱非特异吸附、透析回收损失、取样守恒——量级 UNKNOWN;lossy 显式,不塞进普通洗涤参数 |

## 11. 核查与状态

- 已验证 (检索转写,2026-11-05,§1 定位节):S1 RIPA 双程序 + 抑制剂/相容性注记;S2a BCA 微孔板体积/孵育/波长三元组 + 三档范围;S2b Bradford 稀释/线性/时间窗;S3 spin 平衡/结合/洗×3/洗脱×3 + 再生;S4 预澄清/一抗/beads/5 洗/isotype + 非变性约束;S5 盒上样/3×1 h/200 倍比。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何 PDF 全文逐页、per-lot 片 (抗体/标准/咪唑/膜 MWCO)、仪器操作手册、实验室 SOP 改编、已跑数据派生应用、自旋脱盐/超滤执行程序。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离 (§§2–6 表 1 vs 读数/仪器绑定行);样本跃迁有端口 (裂解液/板/柱/珠/盒/原始矩阵);时间锚显式 (§10 G-7);部分列表标部分 (盒型/缓冲液选项);plan/actual/raw/derived 分离 (本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
