# Batch 10 — Microbial Growth / Transformation / Susceptibility: OD600 Growth Curves, Competent-Cell Transformation, MIC/MBC, Disk Diffusion

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 微生物生长曲线/OD600 取样读数、化学感受态热激转化、抗生素敏感性 MIC/MBC 稀释法、Kirby-Bauer 纸片扩散法。生长测量、转化、药敏三者分立记录;MIC/MBC 与纸片扩散不合并。
- 规则: 湿实验步骤与原始读数 only;OD600 原始读数为 readout,不做生长速率推断;MIC/MBC 判读与 S/I/R 解释为 ToolAction/方法结果,不是操作;现有仓库记录只作 Reuse Map 身份对照,不作步骤/参数证据;不新增公共模块除非四问 + 晋升四门全过, domain-shared 允许但本批无晋升;不做代码/UI/DB 变更。
- 禁止声明: 本文件不含任何通用接种量、通用 OD 目标、通用热激时长、通用 MIC 断点、通用抑菌圈阈值;所有温度/时间/体积/浓度/圈径均为所引协议的绑定值,不得跨协议复制。

## 1. 来源索引与实际读取深度

| # | Protocol | 权威类型 | 版本/定位 | 读取深度 (2026-11-05) |
|---|---|---|---|---|
| P1 | Introductory Bacteriology Lab Manual, LAB 3 Bacterial Growth (eCampusOntario) | 教学实验手册 | Table 3.1 取样时刻 + Growth rate/cell density 计算 + Growth Exercise METHOD 全文 | 直接读:页面全文转写;取样 0/30/60/90/120 min 与预培养 60–180 min、空白/600 nm/>1.0 稀释、8 管 10 倍系列 drop plate 20 µL、RT 24 h 或 37 °C 18 h、≥8 菌落计数窗、TNTC/TFTC、CFU/mL 换算、g 公式 |
| P2 | Son & Taylor 2021, Growth and Maintenance of *E. coli* Laboratory Strains, Curr Protoc e20 (PMC8006063) | 同行评议 protocol 单元 | Basic Protocol 1 (冻存复苏划线) + Basic Protocol 2 (固体稀释铺板) + Basic Protocol 3 (液体接种) 全文;生长条件/培养基节为背景 | 直接读:三节步骤全文;10⁻¹–10⁻¹⁰ 系列、20–100 µL 铺板、酒精烧灼涂布棒、14–18 h 37 °C、5 mL LB 管单菌落接种、37 °C 通气、~10⁹–10¹⁰ cfu/mL 过夜密度注记 |
| P3 | Addgene Protocol — Bacterial Transformation (chemically competent, heat shock) | 实验室 protocol | Equipment + Reagents + Procedure 1–9 + Tips/FAQ 全文 (Last Update 2017-11-13) | 直接读:页面全文转写;−80 °C 冰上 20–30 min 融化、1–5 µL DNA (10 pg–100 ng) + 20–50 µL 感受态、冰 20–30 min、42 °C 30–60 s、冰 2 min、250–1000 µL LB/SOC 无抗 37 °C 摇 45 min、分两板 (50 µL + 剩余)、37 °C 过夜 |
| P4 | Andrews 2001, Determination of minimum inhibitory concentrations, J Antimicrob Chemother 48 Suppl 1 | 同行评议标准化方法论文 | Abstract (Crossref) only;全文未直接读 | 摘要级:MIC=过夜后抑制可见生长的最低浓度、MBC=转接无抗生素平板后无生长的最低浓度;用途(确证耐药/新药体外活性/断点);必须严格遵守不得改编;覆盖标准品储存、母液、培养基、接种物、孵育、判读;附 NCTC/ATCC 质控预期范围表。稀释级数/接种量/温时数值为 `source_missing`,一律 UNKNOWN |
| P5 | Field AMR SOP — *E. coli* Kirby Bauer Disk Diffusion Assay (hostmicrobe;引 Hudzicki 2009 ASM + CLSI M100 ED35:2025 Table 2A) | 实地 SOP (方法学引 ASM 教育 protocol + CLSI 断点表) | Reagents/Equipment + Prepare Tubes + Disk Diffusion Assay 1–12 + Data Interpretation 表全文 | 直接读:页面全文转写;2 mL 盐水管 + 1 mL LB 管、3–5 菌落、0.5 McFarland 目视比浊、MH II 整板三向 60° 涂布 + 沿边、晾 3–5 min、每板 6 纸片、镊子异丙醇擦拭、接触后禁移、35 °C 16–20 h、汇合草坪验收、背板黑底反射光 mm 尺(含片)上舍入、读径困难测半径×2、TMP/SMX ≤20% 轻生长忽略、12 药 S/I/R + ATCC 25922/35218 QC 范围、2 板/株 |

既有复用对照(只记身份,不复述契约,不作证据):
R1 `op.liquid_transfer`;R2 取样/分样 sampling atomic(谱系,守恒缺口);R3 `op.incubation`(elapsed/条件 only,不断言生长/附着/凝胶);R4 涂布/划线接种(spread atomic variant / streak narrow exception);R5 菌落计数 readout + CFU ToolAction;R6 连续稀释=Composite(多次转移);R7 `absorbance_measurement`(raw OD only);R8 `op.centrifugation`(本批仅 P3 可选浓缩语境);审计 `#23` 通用染色、`#24` 解离、`G-1` 发光读数与本批无关。

检索摘要(`search-summary-only`,非证据):Caltech Bi1x W6 生长曲线讲义、N2BVJY microtiter OD 方案摘要、NEB C2988/C3016 热激参数摘要、Hardy Kirby-Bauer 步骤摘要仅用于立项定位;凡数值以 §1 直接读为准。

---

## 2. P1 — 生长曲线 OD600 + Drop Plate CFU (Ontario LAB 3)

绑定摘要:指数期 *E. coli* LB 摇瓶;每 30 min 取样共 5 次(0/30/60/90/120 min;预培养组 60–180 min);分光光度计组与稀释铺板组分工(Table 3.2);OD600 空白 1 mL 液体、600 nm 调零、无菌转 1 mL 立即读数、>1.0 则 1:1 肉汤稀释读数×2、摇瓶回孵;drop plate 仅第 3/5 次(1 h/2 h):8 管各 900 µL 无菌水、100 µL 逐级 10 倍至 10⁻⁸、同头由稀到浓每区 20 µL、火焰旁晾干倒置、RT 24 h 或 37 °C 18 h、≥8 菌落可数、TNTC/TFTC 规则、CFU/mL=菌落/接种 mL×稀释倒数;指数期 doubling 由 OD 翻倍时间差(Method 1/2)或 g=t/(3.32(logN₂−logN₁))。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 空白 1 mL 液体装比色皿;600 nm 调零,全程沿用 | method-local(blank-zero,读数前置) | 空比色皿 → 已调零仪器 | 方法绑定:1 mL、600 nm | P1 Growth Exercise METHOD 1.1–1.2 |
| 无菌转 1 mL 摇瓶培养物至比色皿,立即读数,每 30 min 一次共 5 次 | `sampling`(R2,父瓶保留+子样新建)+ `absorbance_measurement`(R7 raw only) | 摇瓶培养物 → OD 原始值 + 剩余培养物 | 方法绑定:1 mL、30 min×5、立即读 | P1 METHOD 1.3 + Table 3.1 |
| 读数>1.0 则 1 mL 培养物 + 1 mL 肉汤,读数×2 | `op.liquid_transfer`(R1,1:1 稀释)+ ToolAction(×2 换算) | 超量程样 → 稀释样 + 派生 OD | 方法绑定:1:1、阈值 1.0 | P1 METHOD 1.3 注记 |
| 回瓶至孵箱继续生长 | `op.incubation`(R3) | 取样后摇瓶 → 孵育中摇瓶 | 方法绑定;温/摇速本节未给 | P1 METHOD 1.4 |
| 8 管各 900 µL 无菌水,100 µL 逐级传至 10⁻⁸,每级换头+涡旋 | 连续稀释 Composite(R6)=多次 `op.liquid_transfer` | 定时子样 → 稀释系列 | 方法绑定:900/100 µL、换头 | P1 METHOD 2.1–2.5 |
| 平板 4 线分 8 区,由稀到浓每区滴 20 µL(同头),火焰旁晾干后倒置 | method-local(drop-spot enumeration,不并入 spread) | 稀释系列 → 滴板 | 方法绑定:20 µL、由稀到浓顺序 | P1 METHOD 2.6–2.9 + Fig 3.4 |
| RT 24 h 或 37 °C 18 h;≥8 菌落可数,TNTC/TFTC | `colony_count`(R5 readout)+ `op.incubation`(R3) | 滴板 → 原始计数 | 方法绑定:两条温时路线、≥8 窗 | P1 METHOD 2.10 + Fig 3.3 |
| CFU/mL=菌落/接种 mL×稀释倒数;OD 翻倍时间差/g 公式 | ToolAction(换算/拟合);**不是操作** | 原始计数/OD → 派生速率 + 数据集 | 方法要求 | P1 Cell density + Growth rate 节 |
| 台面消毒/Bunsen/BSC 气流/持盖手法 | 方法前置(非操作) | — → 就绪工作区 | 方法绑定 | P1 Aseptic Technique 节 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 取样/分样(定时子样,父瓶保留) | 生长曲线定时取样(P1);划线/液体培养取样(B02) | 复用 R2;微生物第二证据方向 | 体积 1 mL/100 µL 方法绑定;数量守恒 UNKNOWN(沿用缺口) |
| 吸光度原始读数 OD600 | 生长曲线(P1);CV 定量/CFU 按需(B02) | 复用 R7 raw only;blank/稀释阈值方法绑定 | 速率推断为 ToolAction,不进读数模块 |
| 液体转移(稀释/滴板) | 生长曲线(P1);稀释/涂布(B02) | 复用 R1;稀释整体=Composite(R6) | 900/100/20 µL 全方法绑定 |
| 菌落计数 readout | 滴板(P1);涂布/稀释(B02) | 复用 R5;计数≠CFU 计算 | ≥8 窗 vs 30–300 冲突保留,各守其 SOP |
| 孵育(摇瓶保持/平板) | 生长曲线(P1);划线/涂布/液体(B02) | 复用 R3 elapsed-only | 通气/摇速 UNKNOWN 通用值 |
| Drop-spot 滴板分布 | 仅 P1 内 | method-local;不建模块 | 四问有意义复用不足:无 ≥2 独立家族;不与 spread 合并(见 §7) |
| 生成时间/拟合换算 | 生长曲线(P1);CFU 公式(B02) | ToolAction;**永不建模块** | OD 翻倍 vs log 公式均为确定性派生 |

---

## 3. P2 — *E. coli* 生长与维持:复苏/铺板/液体接种 (Son & Taylor, Basic 1–3)

绑定摘要:37 °C 有氧最适、兼性厌氧、pH 7.0、LB 丰富培养 doubling ~20 min、过夜 >10⁹ cfu/mL;Basic 1:无菌木签取冻存小冰块、最小融化、划线求单菌落、37 °C 14–18 h(含抗生素平板维持质粒按 Appendix 4A);Basic 2:10⁻¹–10⁻¹⁰ 系列(100 µL + 900 µL LB,涡旋)、每板 20–100 µL 由稀到浓同头点板、玻璃涂布棒 70% 乙醇 1 min + 火焰烧 + 空冷 15–30 s(琼脂空区试凉)、涂匀至干、37 °C 14–18 h,过夜 LB 只需铺 10⁻⁷–10⁻⁹;Basic 3:5 mL LB 无菌管(大体积转三角瓶)、木签/烧灼环触单菌落重悬、盖紧 37 °C 通气需时培养。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 木签取冻存冰块,速划 LB 平板求单菌落,37 °C 14–18 h | `sampling`(R2,谱系新建)+ `streak_inoculation`(R4)+ `op.incubation`(R3) | 冻存管 → 划线皿 → 孵育中皿 | 方法绑定:最小融化、14–18 h | P2 Basic 1 steps 1–3 |
| 100 µL + 900 µL LB 逐级 10 倍至 10⁻¹⁰,涡旋混匀 | Composite(R6)=多次 `op.liquid_transfer` | 过夜培养物 → 稀释系列 | 方法绑定:100/900 µL | P2 Basic 2 step 1 |
| 20–100 µL 点板(由稀到浓同头),涂布棒乙醇烧灼冷却涂匀至干 | `spread_inoculation`(R4 variant)+ `op.liquid_transfer`(点板) | 稀释液滴 → 均匀表面分布 | 方法绑定:20–100 µL、乙醇 1 min、空冷 15–30 s | P2 Basic 2 steps 2–7 |
| 37 °C 14–18 h;过夜 LB 只需 10⁻⁷–10⁻⁹ | `op.incubation`(R3)+ 方法完成检查 | 铺板 → 孵育中皿 | 方法绑定:14–18 h | P2 Basic 2 step 8 + 注记 |
| 5 mL LB 管 + 单菌落触取重悬,盖紧 37 °C 通气培养 | `sampling`(R2)+ `op.liquid_transfer`(接种)+ `op.incubation`(R3,通气参数槽) | 单菌落 + LB → 接种液体管 | 方法绑定:5 mL;时长按需 | P2 Basic 3 steps 1–4 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 划线接种 | 冻存复苏划线(P2 Basic 1);ASM 划线(B02 P1) | 复用 frozen atomic(R4) | 工具(木签/环)方法绑定 |
| 涂布接种 | 铺板计数(P2 Basic 2);ASM 涂布(B02 P2) | 复用 variant(R4) | 涂布棒灭菌为父操作备注 |
| 液体转移/连续稀释 | 稀释铺板(P2);生长曲线/涂布(P1/B02) | 复用 R1;整体=Composite | 体积全方法绑定 |
| 取样(菌落挑取/冰块取样) | 复苏/接种(P2);划线/液体(B02) | 复用 R2 | 取样量 UNKNOWN |
| 孵育 | 平板/液体(P2);生长曲线(P1) | 复用 R3;通气=参数槽 | pH/气氛通用值无 |

---

## 4. P3 — 化学感受态热激转化 (Addgene)

绑定摘要:设备 37 °C 摇/静孵箱 + 42 °C 水浴 + 冰 + 管 + 无菌涂布器;试剂 LB 琼脂+相应抗生素、LB/SOC、感受态、DNA;流程见表;效率注记:连接产物比完整质粒低 ~10 倍;复苏让抗性蛋白表达,Amp 可跳过、其他抗性建议 ≥20–30 min;省时捷径(手融/冰 2 min/跳过复苏)以降效为代价;大质粒 >10 kb/BAC 转电转;无转化子先查抗生素匹配 + 阳性对照;高感受态时少 DNA 反而高效(连接产物取 1:5/1:10 稀释)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| −80 °C 感受态冰上融 20–30 min;平板 RT 回温(可选 37 °C) | method-local(thaw/plate-warm,前置) | 冻存感受态 + 冷平板 → 可用细胞 + 可用平板 | 方法绑定:20–30 min | P3 Procedure 1–2 |
| 1–5 µL DNA (10 pg–100 ng) + 20–50 µL 感受态,指弹管底轻混 | `op.liquid_transfer`(R1,DNA 加入)+ method-local(DNA–细胞接触,无稳定转化效应) | DNA + 感受态 → 混合物 | 方法绑定:体积/DNA 量 | P3 Procedure 3 |
| 冰上 20–30 min | method-local(cold-contact hold);**不复用 `op.incubation`** | 混合物 → 待热激混合物 | 方法绑定:20–30 min | P3 Procedure 4 |
| 42 °C 水浴热激 30–60 s(常 45 s,随细胞而变),管底 1/2–2/3 入水 | method-local(heat-shock permeabilization);**不并入孵育/洗涤/转移** | 待热激混合物 → 通透化混合物 | 方法绑定:42 °C、30–60 s、浸没深度 | P3 Procedure 5 |
| 回冰 2 min | method-local(骤冷) | 热激后混合物 → 待复苏混合物 | 方法绑定:2 min | P3 Procedure 6 |
| 加 250–1000 µL 无抗 LB/SOC,37 °C 摇 45 min | `op.liquid_transfer`(R1,复苏液加入)+ method-local(recovery hold,抗性表达窗) | 混合物 → 复苏培养物 | 方法绑定:体积、45 min | P3 Procedure 7 + Pro-Tip |
| 取部分/全部涂 10 cm LB+抗生素板(推荐 50 µL + 剩余分两板;液多先离心浓缩重悬防流淌) | `op.liquid_transfer`(R1,涂板)+ `spread_inoculation`(R4,涂布器具未指定按 variant 复用语境) | 复苏培养物 → 选择平板 | 方法绑定:50 µL/分板;离心浓缩为条件分支 | P3 Procedure 8 + Pro-Tips |
| 37 °C 过夜 | `op.incubation`(R3) | 选择平板 → 孵育中平板 | 方法绑定:过夜 | P3 Procedure 9 |
| 捷径/大质粒电转/阳性对照/DNA 稀释注记 | 方法分支 + 完成检查/失败模式(非操作) | — | 方法绑定 | P3 Tips/FAQ |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移(DNA/复苏液/涂板) | 转化(P3);稀释/涂布/生长(P1/P2/B02) | 复用 R1 | 体积/DNA 量全方法绑定 |
| 孵育(过夜平板) | 转化(P3);生长/铺板(P1/P2) | 复用 R3 elapsed-only | 复苏 45 min 摇床为方法保持窗,不复用 R3 作生长断言(见缺口 G4) |
| 涂布接种(转化子铺板) | 转化(P3);铺板计数(P2) | 复用 variant 语境;器具 UNKNOWN | 涂布均匀标准 UNKNOWN |
| 可选离心浓缩(液多时) | 仅 P3 条件分支 | 条件复用 `op.centrifugation`(R8);RCF/时间 UNKNOWN | 单方法分支,不立新模块 |
| 融化/冰接触/热激/骤冷/复苏保持 | 仅转化家族 | method-local;不建模块 | 四问复用失败 + 晋升 G1/G2 不过(见 §9) |
| 效率注记/对照/稀释优化 | 转化方法 | 方法结果/ToolAction 语境,非操作 | 连接 vs 质粒 ~10 倍为本方法经验值 |

---

## 5. P4 — MIC/MBC 标准化稀释法 (Andrews 2001;摘要级)

绑定摘要(仅摘要可断言):MIC=过夜孵育后抑制可见生长的最低浓度(慢生/厌氧延长);MBC=转接无抗生素平板后无生长的最低浓度;用途为临床确证耐药 + 新药体外活性研究 + 断点制定;MBC 较少用,主要用于心内膜炎血 isolate;方法必须严格遵守不得改编;覆盖标准抗生素粉末储存、母液配制、培养基、接种物制备、孵育条件、判读解释;附对照 NCTC/ATCC 菌株预期 MIC 范围表(质控 ±1 两倍稀释容差见检索摘要,未直接读,不作证据)。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 标准粉末储存 + 母液配制 | method-local(antibiotic-stock-prep);细节 UNKNOWN | 粉末 → 母液系列 | 方法绑定;温时/溶剂 UNKNOWN | P4 Abstract 方法覆盖句 |
| 培养基 + 接种物制备 | method-local(inoculum-prep);细节 UNKNOWN | 菌 + 培养基 → 接种体系 | 方法绑定;密度/体积 UNKNOWN | P4 Abstract 同上 |
| 孵育(过夜;慢生延长) | `op.incubation`(R3,仅 elapsed/条件保持) | 接种体系 → 孵育后系列 | 方法绑定;温时 UNKNOWN | P4 Abstract 定义句 |
| MIC 判读:抑制可见生长的最低浓度 | 方法结果/ToolAction;**不是操作**(OD600 原始读数规则的对应物) | 孵育后系列 → MIC 数据集 | 方法要求 | P4 Abstract 定义句 |
| MBC:转接无抗生素平板后无生长的最低浓度 | `spread_inoculation`(R4,转接语境,细节 UNKNOWN)+ 方法结果/ToolAction | MIC 孔/管 → 无抗平板 → MBC 数据集 | 方法绑定;转接体积 UNKNOWN | P4 Abstract 定义句 |
| 对照 NCTC/ATCC 预期范围表 | 方法对照(非操作) | — → 验收窗 | 方法绑定 | P4 Abstract 表注记 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 孵育 | MIC/MBC(P4);生长/转化/纸片(P1–P3/P5) | 复用 R3 elapsed-only | 温时全 UNKNOWN,不得外推 |
| 转接铺板(MBC) | MBC(P4);铺板家族(P2) | 复用 variant 语境;体积 UNKNOWN | 全文未读,转接细节 `source_missing` |
| MIC/MBC 判读与断点 | 药敏稀释法(P4);纸片 S/I/R(P5) | ToolAction + 方法模板结果;**永不建模块** | 判读≠操作(record-standard §1.5) |
| 粉末/母液/接种物制备 | 仅 P4 摘要提及 | method-local;全文未读不展开 | G5 缺口 |

---

## 6. P5 — 纸片扩散法 Kirby-Bauer (Field SOP;Hudzicki 谱系 + CLSI 断点)

绑定摘要见 §1 P5 行;关键与 P4 的分离:纸片法终点是汇合草坪上的抑菌圈直径 + CLSI Table 2A S/I/R 查表,不是 MIC µg/mL;0.5 McFarland 目视比浊是接种物标准化手段,不是生长曲线 OD;盘位/圈径/质控窗全为本 SOP 绑定值。

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| n+3 管组:5 mL 管 + 2 mL 无菌盐水(药敏) + 2 mL 旋盖管 + 1 mL LB(建库);3–5 玫瑰红菌落先后搅入两管,涡旋;加 ATCC 35218/25922 + 空环阴性 | `sampling`(R2,多菌落汇集取样,谱系新建)+ `op.liquid_transfer`(盐水/LB 分装语境) | MacConkey II 菌落 → 盐水悬液 + LB 建库存 | 方法绑定:2 mL/1 mL、3–5 菌落 | P5 Prepare Tubes 1–6 |
| LB 管松盖 35 °C 1000 rpm 过夜(建库/提 DNA 前置) | `op.incubation`(R3,建库保持) | LB 管 → 过夜建库管 | 方法绑定:35 °C、1000 rpm | P5 Prepare Tubes 6 |
| 盐水管加盐水调至 0.5 McFarland(白底黑线卡目视比浊+涡旋) | method-local(turbidity-standardization);**不复用吸光度读数**(标准比浊 intent 不同) | 盐水悬液 → 标准化接种悬液 | 方法绑定:0.5 McFarland 目视 | P5 Disk Assay 1 |
| 无菌棉签蘸取、管壁压去多余,整板涂抹×3(每次转 ~60°)+ 沿边;共 2 板/株 | method-local(lawn inoculation);**不并入 streak/spread**(草坪 intent 不同) | 标准化悬液 → MH II 草坪板 | 方法绑定:三向/沿边/2 板 | P5 Disk Assay 2–6 |
| 开盖晾 3–5 min;异丙醇擦拭镊子取 6 纸片/板按模板布放,接触后禁移,压实接触 | method-local(disk-application) | 草坪板 + 纸片 → 布片板 | 方法绑定:6/板、3–5 min | P5 Disk Assay 7–9 |
| 倒置 35 °C 16–20 h;汇合草坪验收(散在单菌落=接种过稀重做) | `op.incubation`(R3)+ 方法完成检查 | 布片板 → 孵育后草坪板 | 方法绑定:35 °C、16–20 h | P5 Disk Assay 10–11 |
| 背板黑底反射光 mm 尺测圈径(含片,上舍入);读径困难测半径×2;TMP/SMX ≤20% 轻生长忽略取清朗边缘 | method-local(zone readout);圈径解释为 ToolAction/方法结果 | 孵育后板 → 圈径数据集 | 方法绑定:含片、上舍入、半径×2、20% 规则 | P5 Disk Assay 12 + ⚠️注记 |
| CLSI Table 2A 查 S/I/R (12 药含量/阈值/QC 窗) | ToolAction + 方法模板结果;**不是操作** | 圈径数据集 → S/I/R 报告 | 方法要求(CLSI M100 ED35:2025) | P5 Data Interpretation 表 |
| Day 1–2 水样 MC pad 25–250/垫计数 + MacConkey II 纯度传代;Day 4 拍照/甘油保藏/DNA | 上游/下游方法入口,本批不拆 | — | — | P5 Overview 1–6 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 取样(3–5 菌落汇集) | 纸片(P5);划线/接种(P2/B02) | 复用 R2 语境;汇集行为方法绑定 | 取样量/代表性 UNKNOWN |
| 孵育(建库/药敏) | 纸片(P5);生长/转化(P1–P4) | 复用 R3 elapsed-only | 35 °C/16–20 h 方法绑定 |
| 液体转移(盐水/LB 分装/调浊加盐水) | 纸片(P5);稀释/转化(P1–P4) | 复用 R1 | 体积全方法绑定 |
| McFarland 比浊标准化 | 仅药敏家族 | method-local;不建模块 | G1/G2 不过;不与 OD600 读数合并(见 §7) |
| 草坪涂布 | 仅纸片法 | method-local;不建模块 | intent 与单菌落分离/计数铺板不同 |
| 纸片布放 | 仅纸片法 | method-local;不建模块 | 无跨家族复用 |
| 圈径测量 readout | 仅纸片法 | method-local readout;解释另立 ToolAction | 与菌落计数/OD 读数不合并 |
| S/I/R 查表解释 | 纸片(P5);MIC 判读(P4) | ToolAction/方法结果;**永不建模块** | CLSI 版本绑定(ED35:2025) |

---

## 7. Reuse Map 汇总(本批)

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 定时取样 1 mL/100 µL/20–100 µL/50 µL;DNA/复苏液/盐水/LB 加入;分级稀释转移 | `op.liquid_transfer` | 参数化复用;稀释整体=Composite,不建"稀释"原子 |
| 摇瓶定时子样/单菌落挑取/冻存冰块取样/3–5 菌落汇集 | `sampling`(R2) | 参数化复用;谱系新建;数量守恒 UNKNOWN |
| 摇瓶/平板/复苏管/药敏板保持(37 °C/35 °C/RT,通气/倒置/振荡) | `op.incubation` | 复用 elapsed-only;通气/倒置/振荡为参数槽,不推断生长 |
| OD600 原始读数(含 1 mL 空白调零语境) | `absorbance_measurement`(R7 raw only) | 按需复用;阈值/稀释/速率拟合为方法绑定 + ToolAction |
| 涂布铺板(玻璃棒/转化子/铺板计数) | `spread_inoculation`(R4 variant) | 复用 variant;drop-spot 不并入 |
| 划线求单菌落(冻存复苏) | `streak_inoculation`(R4 narrow) | 直接复用;不与涂布/草坪合并 |
| MBC 转接无抗平板 | `spread_inoculation` 语境复用 | 细节 UNKNOWN(摘要级);判读另立 |
| 菌落/滴板计数 | `colony_count`(R5 readout) | 复用;计数窗各守其 SOP(≥8 vs 30–300) |
| 转化可选离心浓缩 | `op.centrifugation`(R8) | 条件复用;RCF/时间 UNKNOWN |
| CFU/mL、OD×2、g 公式、MIC/MBC 判定、圈径 S/I/R 查表 | ToolAction + 方法结果 | **永不建模块** |
| 融化/冰接触/热激/骤冷/复苏保持;drop-spot;McFarland 目视比浊;草坪涂布;纸片布放;圈径测量;粉末/母液/接种物制备(摘要级);溶解/灭菌/储存语境 | 无 → `method_local` | 方法模板自有步骤;不晋升(§9) |

---

## 8. 跨方法公共操作覆盖矩阵

● 直接使用;△ 条件/分支使用;○ 刻意不用(防合并);— 本方法无此步。

| 操作 | 生长曲线 P1 | 生长维持 P2 | 转化 P3 | MIC/MBC P4 | 纸片 P5 | 既有家族证据 | 结论 |
|---|---|---|---|---|---|---|---|
| `op.liquid_transfer` | ● 稀释/滴板 | ● 稀释/点板 | ● DNA/复苏液/涂板 | —(摘要未展开) | ● 盐水/LB/调浊 | B02/R9/Morimoto | shared,最多复用 |
| `sampling` 谱系 | ● 定时子样 | ● 挑菌/冰块 | ○(DNA 加入为转移+接触) | — | ● 3–5 菌落汇集 | B02/Morimoto | shared |
| `op.incubation` | ● 回孵/平板 | ● 14–18 h/通气 | ● 过夜板(复苏保持除外) | ● 过夜(值 UNKNOWN) | ● 建库/35 °C 16–20 h | B02/R8/R9 | shared elapsed-only |
| `absorbance_measurement` raw | ● OD600 | — | — | — | ○(比浊不用 OD) | B02 R7 | 按需复用 raw |
| `streak_inoculation` | — | ● 复苏 | — | — | ○(草坪≠划线) | B02 P1/P5 | 保持独立 |
| `spread_inoculation` | ○(滴板≠涂布) | ● | ● 转化子铺板 | △ MBC 转接语境 | ○(草坪≠涂布) | B02 P2/R9 | 保持 variant |
| `colony_count` readout | ● 滴板≥8 | ●(铺板语境) | — | — | — | B02 P2/P3 | shared readout |
| CFU/OD/g ToolAction | ● | —(密度注记) | ○(效率注记) | ● MIC/MBC 判定 | ● S/I/R 查表 | B02 P3 | 非操作 |
| `op.centrifugation` | — | — | △ 液多浓缩 | — | — | R8 | 条件复用 |
| method-local 转化五件套 | — | — | ● 融/冰/热激/冷/复苏 | — | — | 无 | 不晋升 |
| method-local 药敏四件套 | — | — | — | — | ● 比浊/草坪/布片/圈径 | 无 | 不晋升 |
| method-local 制备 | — | — | — | ● 粉/母液/接种(摘要) | — | B08 对照 | 不晋升 |

防合并线(显式拒绝):drop-spot ≠ spread(点滴分布 vs 均匀涂布);lawn ≠ streak/spread(汇合草坪 vs 分离/计数);McFarland 目视比浊 ≠ OD600 原始读数(标准悬液 intent vs 密度读数);heat-shock ≠ incubation(通透化效应 vs elapsed 保持);recovery hold ≠ growth incubation(抗性表达窗 vs 生长断言,见 G4);zone readout ≠ colony_count/absorbance(方法终点不同);MIC/MBC 与纸片 S/I/R 互不替代(定量浓度 vs 圈径查表)。

---

## 9. 缺口清单(不能表达 / UNKNOWN;分类按 record-standard §4)

| # | 缺口 | 影响步骤 | 现状 |
|---|---|---|---|
| G1 | P4 全文未读:稀释级数/接种量/培养基/温时/质控窗全部 UNKNOWN | P4 全表 | `source_missing`;摘要外任何数值禁引;复核须取 JAC 全文 |
| G2 | OD 生长速率推断链缺失:OD 翻倍≠细胞翻倍、波长/死活/团聚/色素效应 | P1 速率 | 方法 ToolAction;RD 校准等外部讨论未纳入证据;原文仅给两种课堂算法 |
| G3 | 计数窗冲突:≥8/滴(P1) vs 20–100 µL 板(B02/P2) vs 30–300 vs 25–250(垫) | P1/P2/P5 上游 | 冲突保留,各守其 SOP;不平均(record-standard §3) |
| G4 | 复苏/保持 ≠ 生长断言:45 min 抗性表达、过夜"饱和"、35 °C 草坪汇合均不能由 elapsed 推出 | P3/P5 | 模型边界;完成检查只能是可见生长/草坪汇合(人工判断) |
| G5 | 转化感受态制备链缺失:自制/商品效率谱、基因型/保存/冻融、质粒大小阈值(>10 kb 电转注记为经验) | P3 | `source_missing` + `method_dependent`;本批只记 Addgene 化转终点用法 |
| G6 | 热激分支未解:菌株/细胞量/质粒形态/45 s 可变性;冷接触 20–30 min vs 捷径 2 min 降效无量化 | P3 | `method_dependent`;不得立通用"热激"模块 |
| G7 | McFarland 目视链:标准品批号/光线/观察者/盐水 vs 肉汤悬液、与分光 OD 的换算 | P5 | `configuration_dependent`;本 SOP 目视卡法绑定,不作通用值 |
| G8 | 纸片链:琼脂深度/干燥度/纸片含量/布放模板/叠圈/边缘效应;6/板为本模板值 | P5 | `method_dependent`;CLSI 版本绑定,换版须重审 |
| G9 | 接种物代表性:单菌落 vs 3–5 菌落汇集 vs 定时子样;环/签/头取样量 | P2/P3/P5 | UNKNOWN;谱系有,数量守恒无(沿用既有缺口) |
| G10 | 时间锚点:生长 `t₀`=转接/调零时刻;转化 `t₀`=DNA 接触;药敏 `t₀`=调浊布片;储存/建库为独立保持窗 | 全部 | 锚点缺口;调后密闭/晾干/回孵为防漂移约束,非孵育断言 |
| G11 | 范围外(误建模块风险) | — | 电转/接合/转导、厌氧/高温菌、发酵罐、MIC 自动化仪器、基因型耐药解释、PK/PD 断点外推一律不建模(`not_applicable`) |

---

## 10. 晋升审查结论:零新模块

四问(独立目的/稳定边界/独立确认/有意义复用) + 晋升四门(G1 复用广度 ≥2 独立方法、域内成立;G2 语义稳定;G3 抽象收益;G4 分支成本):

| 候选 | 四问 | G1 | G2 | G3 | G4 | 决定 |
|---|---|---|---|---|---|---|
| Drop-spot 滴板分布 | ✓✓✓△ | ✗(仅 P1) | △ | ✗ | ✓ | method-local;不晋升 |
| 热激通透/冷接触/骤冷/复苏保持 | ✓✓✓△ | ✗(仅转化) | ✗(通透 vs 保持 effect 不同) | ✗ | ✗ | method-local;不并入 `op.incubation`;不晋升 |
| McFarland 比浊标准化 | ✓✓✓✓ | ✗(仅药敏) | ✓ | ✗ | ✓ | method-local;不与吸光度合并;不晋升 |
| 草坪接种 lawn | ✓✓✓✓ | ✗(仅纸片) | ✓ | ✗ | ✓ | method-local;不与 streak/spread 合并 |
| 纸片布放 | ✓✓✓✓ | ✗ | ✓ | ✗ | ✓ | method-local |
| 圈径测量 readout | ✓✓✓✓ | ✗ | ✓ | ✗ | ✓ | method-local readout;解释另立 ToolAction |
| 抗生素母液/接种物制备(摘要级) | 边界不明 | ✗(全文未读) | — | — | — | method-local deferred;全文复核前不议晋升 |
| 高效/亚克隆热激变体、电转 | 范围外 | — | — | — | — | 不建模(G11) |

因此本批全部候选留在 method-local / ToolAction / 方法约束,不进入公共 Catalog,不写 PortSpec/Transition 全文,不做代码/UI/DB 变更。本批对公共 Catalog 的唯一贡献是复用证据增量:`op.liquid_transfer`( + 转化 DNA/复苏液/药敏盐水语境)、`sampling`( + 定时子样/汇集取样语境)、`op.incubation`( + 转化过夜板/药敏 35 °C 板语境)、`spread_inoculation`( + 转化子铺板语境)、`colony_count`( + 滴板≥8 窗语境)、`absorbance_measurement`( + 生长曲线定时 OD600 语境)。

---

## 11. 核查与状态

- 已验证(直接读,2026-11-05,§1 定位节):P1 取样时刻/空白/600 nm/稀释/滴板/温时/公式;P2 Basic 1–3 步骤与 37 °C/pH/密度背景;P3 Equipment/Reagents/1–9/Tips-FAQ;P5 管组/比浊/涂布/布片/温时/圈径规则/CLSI 表。单人 `agent-verified`,仍需第二人逐字段复核;未作 `SourceChecked` / `ExpertReviewed` / `LabValidated` 声明。
- 未验证:任何高压釜/分光光度计/pH 计/纸片批号操作手册、CLSI 全文断点、JAC 全文稀释细节、试剂批间差、已跑数据的派生应用;检索摘要一律未转写为证据。
- 记录清单:每值有定位或标 UNKNOWN;方法要求与仪器规格分离;样本跃迁有端口(§§2–6 表 1);时间锚显式;部分列表标部分;plan/actual/raw/derived 分离(本批只有 plan + 方法约束,无 run 值);无 JSON;UNKNOWN 永不填零。
