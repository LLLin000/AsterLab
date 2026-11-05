# Batch 02 — Microbiology Basics: Streak / Spread / Liquid Culture / Serial Dilution / Colony Counting

- 日期: 2026-11-05
- 状态: `research_draft`;不是 `SourceChecked`;没有 `ReviewRecord`
- 范围: 细菌划线分离、涂布铺板、液体培养、连续稀释、菌落计数/CFU
- 规则: 划线与涂布不合并;CFU 计算是 ToolAction,不是操作;不新增公共模块(见 §8);不做代码/UI/DB 变更
- 现有记录仅作 Reuse Map,不作步骤/参数证据

## 1. 来源索引与实际读取深度

| # | 来源 | URL/定位 | 读取深度 | 适用范围 |
|---|---|---|---|---|
| P1 | Katz 2008, ASM Streak Plate Protocol (peer-reviewed CUE 2008) | https://asm.org/asm/media/protocol-images/the-streak-plate-protocol.pdf | 直接读取:T-streak 详细步骤(A–D, pp.6–9)+Comments/Tips(A–G, pp.9–11);历史页仅背景浏览 | 需氧细菌平板划线分离 |
| P2 | Wise 2006, ASM Preparing Spread Plates Protocols (peer-reviewed CUE 2006) | https://asm.org/asm/media/protocol-images/preparing-spread-plates-protocols.pdf | 直接读取:Spread 全流程(pp.2–3)+Track 变体(pp.4–5)+Tips(pp.6–7) | CFU 计数用涂布/轨道铺板 |
| P3 | Reynolds 2005, ASM Serial Dilution Protocols (peer-reviewed CUE 2005) | https://asm.org/asm/media/protocol-images/serial-dilution-protocols.pdf | 直接读取:全文(Steps 1–4, pp.2–4;Comments/Tips p.6) | 细菌 CFU 稀释与计算 |
| P4 | Addgene, Inoculating a Liquid Bacterial Culture (lab protocol) | https://www.addgene.org/protocols/inoculate-bacterial-culture/ | 直接读取:Written Protocol 1–10 + 抗生素表 + Tips/FAQ 全文 | 质粒宿主 LB 液体过夜培养;非同行评议,仅限此适用范围 |
| P5 | Elbing & Brent 2018, Growth of E. coli on Solid Media, Curr Protoc Mol Biol e82 (PMC6819146) | https://pmc.ncbi.nlm.nih.gov/articles/PMC6819146/ | 直接读取:Basic Protocols 1–3 + Support Protocol 2(保存/复苏);Replica plating 仅标题浏览 | E. coli 固体培养:稀释滴定/划线/涂布的同行评议佐证 |

既有复用对照(仅身份,不复述契约):
R1 `op.liquid_transfer`(record-liquid-transfer.md);R2 取样/分样(sampling atomic, granularity-freeze #2);
R3 `op.incubation`(elapsed-only, freeze #5);R4 涂布/划线接种(streak atomic + spread variant, freeze #10–11);
R5 菌落计数读数原子(freeze #12);R6 连续稀释=Composite(freeze #14);R7 `absorbance_measurement`(raw OD);
R8 OP-INOC/OP-INCUB(哺乳动物,inoculation-and-incubation.md,范围不同仅对照);R9 OP-CFU-DILUTION-PLATE/CV(biofilm-cv-cfu.md,生物膜路径对照)。

---

## 2. P1 — ASM 划线分离 (T-streak)

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| A. 皿底边缘标记(菌名/琼脂/日期/姓名);无菌拭子去可见水滴; мысленно分三区 | `method_local`(plate-label-prep) | 干琼脂皿 → 已标记分区皿 | 方法绑定 | P1 p.6–7, §A |
| B. 金属环烧至红热 / 灭菌器 5–7 s (815 °C);静置冷却,不吹不挥;塑料环每区换新 | `method_local`(loop-sterilize);冷却检查为父操作备注 | 染菌环 → 无菌冷环 | 方法绑定(815 °C/5–7 s 仅灭菌器型号条件,UNKNOWN 通用) | P1 p.7, §B; Tips B p.9–10 |
| C. 开管取一环菌液(含管口/管帽无菌手法) | `sampling`(R2)参数化复用;谱系:母管 → 环上样品 | 混合肉汤管 → 环上接种物 | 方法绑定 | P1 p.7–8, §C; Tips E p.11 |
| D1. 第一区 zig-zag 密划,环滑过不挖琼脂 | `streak_inoculation`(R4 atomic) | 环上接种物 → 第一区分布菌 | 方法绑定(密度);UNKNOWN 定量标准 | P1 p.8, §D1 |
| D2. 区之间重新灭菌/换环;冷环 touching 未接种区无滋滋/无烙痕才继续 | `method_local`(同 B);完成检查 | 热环 → 无菌冷环 | 方法绑定 | P1 p.8, §D2 |
| D3. 冷环点第一区一次带菌入第二区,较疏 zig-zag | `streak_inoculation`(同区划内稀释分支) | 第一区菌 → 第二区稀疏分布 | 方法绑定 | P1 p.8, §D3 |
| D4–D5. 换无菌环;点第二区一次入第三区,行距最宽;勿碰第一区 | `streak_inoculation`(同上) | 第二区菌 → 第三区分离分布 | 方法绑定 | P1 pp.8–9, §D4–D5 |
| D6. 终末烧环冷却;倒置(琼脂面朝上)孵育 | `method_local`(终末灭菌)+ `op.incubation`(R3,仅 elapsed/条件保持) | 划线皿 → 孵育中皿 | 倒置方向方法绑定;温度/时间 UNKNOWN(本 protocol 未给) | P1 p.9, §D6; Tips D p.10–11 |
| Tips A. 重接种用四象限;选择/鉴别培养基(MacConkey 例) | 方法分支 + 方法绑定(培养基) | — | 方法绑定 | P1 p.9, Tips A |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 划线接种 streak_inoculation | 划线分离(P1);E. coli 划线单菌落(P5 Basic 2);冻存复苏划线(P5 Support 2) | 复用 frozen atomic(R4);本次为第三证据源,不新建 | P1 §D; P5 Basic 2 全文 |
| 取样/环取接种物 | 划线取样(P1 §C);菌落挑取(P4 step 4–5);血球计数取样(既有 Morimoto) | 复用 sampling atomic(R2);新增微生物家族证据 | 谱系:管/菌落 → 新培养;数量守恒 UNKNOWN(既有缺口) |
| 孵育(平板倒置) | 划线/涂布/液体培养(P1/P2/P4);哺乳动物(R8) | 复用 R3;倒置/湿度容器为方法绑定 | 温度/时间全部方法依赖,本批无通用值 |
| 环/涂布器灭菌 | 仅平板接种内(环烧灼 P1;酒精烧涂布器 P2) | `method_local`;不建公共模块 | 四问中"有意义复用"不足:仅同一技术家族内出现,无 ≥2 独立方法家族;设备条件化严重 |

---

## 3. P2 — ASM 涂布铺板 (Spread + Track)

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| 琼脂 45–50 °C 冷却后倒板;100×15 mm 板 15–20 mL(~0.3 cm);室温干 ~24 h 或层流罩加速;冷藏袋存,用前回温 | `method_local`(plate-pour-prep) | 融化琼脂 → 干可用平板 | 方法绑定(45–50 °C/15–20 mL/24 h 均为本方法值,不通用) | P2 p.1–2 |
| 稀释系列(常 10 倍,PBS);计数窗 20–300(或 25–250);取 0.1 mL;尽快涂布;稀→浓免换头 | 连续稀释 Composite(R6)=多次 `op.liquid_transfer`;0.1 mL 点板 = `liquid_transfer/add_to_existing` | 原样 → 稀释系列 → 板上液滴 | 方法绑定;稀→浓免换头为本方法污染控制,不通用 | P2 p.2 |
| 涂布器蘸酒精烧灼冷却;接触菌液均匀施压转板;勿涂到边缘;防积液;静置 10–20 min | `spread_inoculation`(R4 variant)+ `method_local`(spreader-sterilize) | 板上液滴 → 均匀表面分布 | 方法绑定;10–20 min 方法值 | P2 p.2–3, Fig.2–7 |
| 吸收后倒置孵育;提前观察防菌落融合;慢生菌用保湿容器 | `op.incubation`(R3) | 涂布皿 → 孵育中皿 | 方法绑定;保湿为方法分支 | P2 p.3 |
| 计数:重复 2–3 板,30–300 CFU/板算 CFU/mL;挑单菌落建新培养 | `colony_count`(R5 readout)+ ToolAction(CFU 公式)+ `sampling`(挑菌) | 菌落板 → 原始计数 + 派生 CFU | 30–300 vs 20–300 冲突:各自保留,不平均(见缺口 G3) | P2 pp.3–4; Tips p.6 |
| Track 变体:10 µL(或 5 µL);方板 6 道/圆板 3–4 道;倾 ~60°(45–90°);距边缘 0.5 cm 回倾防积液;静置 5–10 min;20–200 CFU/道 | 同 spread 方法模板内分支,不另立操作 | 稀释液滴 → 轨道分布 | 方法绑定 | P2 pp.4–5 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 涂布接种 spread_inoculation | 涂布计数(P2);E. coli 均匀铺板(P5 Basic 3,0.05–1 mL);生物膜 CFU 铺板对照(R9) | 复用 frozen variant(R4);不与划线合并 | P2 p.2; P5 Basic 3; R9 §7.3 |
| 液体转移(系列稀释+点板) | 涂布稀释(P2);CFU 稀释(P3);生物膜 CFU(R9);哺乳动物分装(R8/Morimoto) | 复用 R1;稀释整体=Composite(R6),不建"稀释"原子 | 稀释倍数/体积全方法绑定 |
| 菌落计数 colony_count | 涂布计数(P2);稀释计数(P3);生物膜 CFU(R9) | 复用 readout atomic(R5);计数 ≠ CFU 计算 | 计数窗冲突见 G3 |
| 孵育 | 同 P1 表 | 复用 R3 | — |
| Track/Drop 法 | 仅 P2 内部分支 | 方法内分支,不建模块 | 四问失败:无跨方法复用证据 |

---

## 4. P3 — ASM 连续稀释 + CFU (Reynolds)

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| Step 1: 选 30–300 菌落板;同稀释度复板取平均 | `colony_count`(R5)+ ToolAction(平均) | 菌落板 → 可用原始计数 | 方法绑定(30–300) | P3 p.2 |
| Step 2: 单管稀释倍数=转移量/(转移量+稀释液);总倍数=连乘;例 0.5→4.5 再 1→9 =1/100 | 多次 `op.liquid_transfer`(R1,partial)+ ToolAction(连乘) | 原样 → 稀释系列(谱系) | 方法绑定;公式为确定性计算 | P3 pp.2–3 |
| Step 3: 接种量直接给定(例 0.1 mL→1/10),无计算 | `liquid_transfer` 参数记录 | 稀释管 → 板 | run 记录实际值 | P3 p.3 |
| Step 4: CFU/mL=菌落数/(总稀释倍数×接种 mL);例 51/(10⁻²×1/10)=5.1×10⁴ | ToolAction(CFU 公式);**不是操作** | 原始计数 → 派生 CFU 数据集 | 方法要求;科学计数+原始值并存 | P3 p.4 |
| 无菌操作:每级换无菌吸头;充分振荡;禁口吸;生物危害处置 | 前置条件/完成检查/失败模式(方法绑定) | — | 方法绑定 | P3 p.6 |
| Tips: <30(杂菌/抽样误差主导);>300(重叠/漏数) | 方法判读依据,非操作参数 | — | 方法绑定 | P3 p.6 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 液体转移(分级转移) | 稀释(P3);涂布(P2);生物膜 CFU(R9);细胞培养(R8) | 复用 R1;不建"稀释"原子(freeze #14) | 换头/振荡为方法绑定,非模块默认 |
| 菌落计数 | 同 P2 表 | 复用 R5 | — |
| CFU 计算 | 稀释滴定(P3);E. coli 滴定(P5 Basic 1,22×10×10⁶=2.2×10⁸/mL);生物膜 CFU(R9 §7.1) | ToolAction;**永不建 OperationModule**(record-standard §1.5) | 三源公式一致,均为确定性换算 |

---

## 5. P4 — 液体培养接种 (Addgene; P5 佐证)

### 表 1:步骤拆解

| 原文步骤 | 操作语义(现有模块/候选) | 输入 → 输出 | 参数归属 | 来源定位 |
|---|---|---|---|---|
| LB 配制:400 mL 含 4 g NaCl/4 g 胰蛋白胨/2 g 酵母+dH₂O;松盖+锡纸;高压灭菌;室温冷存 | `method_local`(media-prep) | 干粉+水 → 无菌 LB | 方法绑定(配方/灭菌条件) | P4 steps 1–2 |
| 按量分装 LB + 抗生素至终浓度(1000× 母液,-20 °C;如 Amp 100 µg/mL) | `op.liquid_transfer`(R1)参数化复用 | LB + 抗生素母液 → 选择性培养基 | 方法绑定(浓度表) | P4 step 3 + 抗生素表 |
| 无菌头/牙签挑单菌落,投入摇匀(可取出牙签) | `sampling`(R2)+ `liquid_transfer`(full/add_to_existing) | 琼脂单菌落 → 接种的液体管(谱系新建) | 方法绑定 | P4 steps 4–5 |
| 松盖/锡纸/透气管/透气膜覆盖保证通气 | 父操作备注(通气条件),非独立操作 | — | 方法绑定 | P4 step 6 |
| 37 °C 摇床 12–18 h(高拷贝 12–16 h;低拷贝 18–30 h;30 °C 菌株延长) | `op.incubation`(R3)+ 振荡参数槽 | 接种管 → 饱和培养物 | 方法绑定;rpm UNKNOWN(FAQ 150–250/350–400 仅本方法经验,未立规) | P4 step 7 + FAQ |
| 浑浊雾状=生长;阴性对照(LB+抗生素无菌)应无生长;对数期需测 OD600 | 方法完成检查(目视,operational)+ `absorbance_measurement`(R7,仅 OD600 时) | 培养物 → 生长确认/OD 原始值 | 方法绑定 | P4 step 8 + Notes |
| 可选甘油保藏;转质粒提取 | 下游方法入口,本批不拆 | — | — | P4 steps 9–10 |
| 佐证:饱和 E. coli ~10⁹/mL;5 µL→5 mL 三管 10³ 系列;各取 100 µL 铺板;37 °C 过夜;22 菌落@10⁻⁶→2.2×10⁸/mL | 同 R6/R5/ToolAction(P3);跨源一致性证据 | — | — | P5 Basic 1 |

### 表 2:公共操作复用

| 公共操作候选 | 出现过的独立方法家族 | 处理决定 | 证据/缺口 |
|---|---|---|---|
| 取样(菌落挑取,谱系新建) | 液体接种(P4);划线取样(P1);冻存复苏取样(P5 Support 2) | 复用 R2;微生物家族为第二证据方向 | 挑取工具(环/牙签/头)为方法绑定 |
| 液体转移(抗生素添加/接种) | 液体培养(P4);稀释/涂布(P2/P3) | 复用 R1 | 体积/浓度全方法绑定 |
| 孵育(摇床,通气) | 液体培养(P4);平板(P1/P2);哺乳动物(R8) | 复用 R3;振荡/通气为参数槽,不另立"摇床孵育"模块 | rpm/通气量 UNKNOWN 通用值 |
| 吸光度读取(OD600) | 对数期定量(P4 Note);CV 定量(R9) | 复用 R7(raw only);blank/标准曲线方法绑定 | P4 未给波长外参数,UNKNOWN |
| LB 配制/灭菌 | 仅本方法 | `method_local` | 单方法,无跨家族证据 |

---

## 6. Reuse Map 汇总

| 实验步骤 | 复用对象 | 处理 |
|---|---|---|
| 划线分布(三区/T) | `streak_inoculation`(R4 atomic) | 直接复用;区数/行距方法绑定 |
| 涂布分布(含 Track) | `spread_inoculation`(R4 variant) | 复用 variant;Track 为模板内分支 |
| 环/涂布器烧灼灭菌 | 无 → `method_local` | 不新建(单技术家族,设备条件化) |
| 标皿/倒板/晾干/培养基配制灭菌 | 无 → `method_local` | 方法模板自有步骤 |
| 分级稀释转移 | `op.liquid_transfer` ×N = Composite(R6) | 参数化复用;不建"稀释"原子 |
| 点板 0.1 mL / 加抗生素 | `op.liquid_transfer` | 参数化复用 |
| 环取样/挑单菌落 | `sampling`(R2) | 参数化复用;谱系新建 |
| 平板倒置/摇床过夜 | `op.incubation` | 复用;仅 elapsed/条件保持,不推断生长 |
| 菌落计数 | `colony_count`(R5 readout) | 复用;计数窗方法绑定 |
| CFU/mL 换算、复板平均、稀释连乘 | ToolAction | 永不建模块 |
| OD600 对数期确认 | `absorbance_measurement`(R7) | 仅需 OD600 时复用 raw 读数 |

## 7. 跨方法公共操作覆盖矩阵

| 操作 | 划线 P1 | 涂布 P2 | 稀释/CFU P3 | 液体培养 P4 | 既有家族证据 | 结论 |
|---|---|---|---|---|---|---|
| `streak_inoculation` | ● | ○(明确区分) | — | — | P5 Basic 2/Support 2 | 保持独立 atomic |
| `spread_inoculation` | ○(明确区分) | ●(含 Track 分支) | — | — | P5 Basic 3; R9 铺板 | 保持 variant,不拆移动 |
| `op.liquid_transfer` | — | ●稀释+点板 | ●分级转移 | ●抗生素+接种 | R8/R9/Morimoto | shared,最多复用 |
| `sampling`谱系 | ●环取 | ●挑菌 | — | ●挑菌落 | Morimoto/计数取样 | shared(微生物第二证据) |
| `op.incubation` | ●倒置 | ●倒置/保湿 | — | ●摇床通气 | R8/R9 | shared;振荡=参数槽 |
| `colony_count` readout | — | ● | ● | — | R9 CFU | shared readout |
| CFU 计算 | — | ● | ● | — | P5; R9 | ToolAction,非操作 |
| `absorbance_measurement` | — | — | — | △(OD600 按需) | R9 CV | 按需复用 raw |
| `method_local`(灭菌/倒板/配基) | ● | ● | — | ● | 各方法模板 | 不晋升 |

● 直接使用;△ 条件使用;○ 刻意不用(防合并);— 本方法无此步。

## 8. 缺口清单(不能表达 / UNKNOWN)

| # | 缺口 | 影响步骤 | 现状 |
|---|---|---|---|
| G1 | 划线"分离成功"的定量标准(行距/菌量/单菌落率) | P1 D1–D5 | UNKNOWN;原文只有定性"越来越稀";完成检查只能是"有无孤立菌落"(人工判断,record-standard §1.3) |
| G2 | 涂布均匀性验收标准(CV/边缘排除量化) | P2 涂布 | UNKNOWN;"勿涂边缘""均匀施压"定性;边缘菌落计数困难仅警告 |
| G3 | 计数窗冲突:20–300 vs 25–250(P2) vs 30–300(P2 重复板/P3) vs Track 20–200 | P2/P3 计数 | 冲突保留,各守其 SOP;不取平均(record-standard §3) |
| G4 | 摇床 rpm/通气量通用值 | P4 step 7 | UNKNOWN(`method_dependent`);150–250/350–400 仅 P4 FAQ 经验 |
| G5 | 稀释后放置时限(P2 "30 min 内接种"仅涂布;稀释管增殖/死亡漂移) | P2/P3 | 部分已知(P2 Tips);通用保持时限 UNKNOWN |
| G6 | 选择/鉴别培养基、McFarland 预估、复平板 | P1 Tips A; P2 Tips; P5 Support 1 | 方法绑定/范围外;不建模块 |
| G7 | 环上取样量的体积/细胞数 | P1 §C; P4 steps 4–5 | UNKNOWN;谱系有,数量守恒无(沿用既有缺口) |
| G8 | 厌氧/微需氧/高温菌的孵育气氛与琼脂替代(Gelrite 等,P1 Theory 提及) | 全部平板法 | 范围外;`op.incubation` 参数槽可挂,无证据值 |

## 9. 审核与诚实标记

- 本文件全部表格由 AI 2026-11-05 直接读取 P1–P5 写成,属 `agent-verified`,**不是** `SourceChecked`;未做第二人逐字段复核,未做湿实验验证。
- P1–P3 页码定位指向 PDF 内页眉页(ASM 在线转址后的 getcontentasset 副本);页号为该副本页,复核时以步骤号(§A–D/Step 1–4)为准。
- 未引用任何本地实验记录作步骤/参数证据;Morimoto(M1–M4)仅在 R8 行 No local experiment records as evidence — 本批遵守。
- JSON/运行时:无;Catalog 变更:无(§8 结论:零新模块,全部复用 frozen atoms + method-local + ToolAction)。
