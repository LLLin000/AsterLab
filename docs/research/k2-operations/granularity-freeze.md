# 粒度冻结:最小可审计科学操作

- 日期:2026-11-05
- 状态:**原则冻结**,不是最终模块清单;所有记录仍为 `research_draft`
- 依据:[record-standard §1.1–1.3](../record-standard.md#11-粒度判定最小可审计科学操作)、[候选审查](granularity-candidates.md)、[独立复审](granularity-candidate-review.md)、[五场景](granularity-scenarios.md)
- 结论:不拆到最小物理动作,拆到“有独立科学意义、边界稳定、可独立确认、可复用”的最小操作。

## 1. 四条判定 + 停止规则

| 维度 | 判断 |
|---|---|
| 独立目的 | 是否有明确、独立的实验意图? |
| 稳定边界 | 输入、输出、样本变化能否说明白? |
| 独立确认 | 能否判断完成、失败、需复核? |
| 有意义复用 | 换方法后基本含义是否不变? |

停止规则:继续拆只得到设备动作、操作姿势、无独立科学意义的子动作时,停止。

## 2. 四种粒度 + Custom Step

| 层 | 表达 |
|---|---|
| Global Operation | `OperationModule`(跨多个湿实验领域) |
| Domain-Shared Operation | `OperationModule` + 技术领域适用范围(同一领域多个方法复用) |
| Composite Block | 递归 `BlockUse` |
| Method Template | `ProtocolVersion` + 组合块 |
| Assay Model / Design | Design/Assay |

Custom Step 保留:

- 新染色/新材料/新处理,没有稳定模块;
- 有标题、说明、时间、参数、目标、来源;
- 样本变化可 `Unknown`;
- 反复出现且边界稳定后再提炼。

**姿势/设备动作不是 Custom Step**;它们应作为父操作的备注/参数。

## 3. 复审争议的最终裁决

| 候选 | 复审意见 | 冻结决定 | 理由 |
|---|---|---|---|
| 取样/分样 | 降级为 Custom | **保留 Atomic Operation** | 独立科学目的是“建立父子样本谱系”,不是单纯移液;谱系/数量缺口是模型扩展问题,不是取消操作 |
| 洗涤 | 拆 lossy 变体 | **基础洗涤原子 + lossy 变体** | 基础液体交换可复用;生物膜洗涤必须声明潜在样本损失,不能默认无损 |
| 细胞计数/活力 | 拆分 | **拆** | 计数是读数原子;活力依赖染色/方法,归 method template/analysis |
| 核酸/蛋白定量 | 不单原子 | **接受** | blank/单位/方法约束不同;作为方法绑定读数,不建通用“quantification”模块 |
| 孵育 | 降确认等级 | **原子,确认仅 elapsed/偏差** | 时间到不等于培养完成;附着/生长是方法前置或结果,不由孵育自动断言 |
| 荧光成像 | 仅原始采集 | **接受** | raw image acquisition 是原子;设门/活死/分析归 method |
| 涂布接种 | 列为变体 | **接受** | 表面接种是划线/接种的 variant;不拆每次涂布器移动 |
| 轻敲/旋转混匀等姿势 | 进 Custom | **拒绝** | 无独立科学目的/边界;作为父操作备注 |

## 4. 候选冻结表(初轮)

| # | 候选 | 类型 | 继续拆? | 备注 |
|---|---|---|---|---|
| 1 | 液体转移 | Atomic | 否,不拆吸/排液 | `transfer_mode` 条件 |
| 2 | 取样/分样 | Atomic | 否 | 父子谱系;数量守恒待模型扩展 |
| 3 | 孵育/保持 | Atomic | 否 | 确认 elapsed/偏差,不推断生长 |
| 4 | 基础洗涤 | Atomic + parameter slots | 否 | 试剂/体积/次数/浸泡/搅拌是参数;样本保留语义需显式 |
| 5 | lossy 洗涤 | Atomic variant | 否 | 生物膜/易脱落样本;损失量 UNKNOWN;不能塞进普通洗涤参数 |
| 6 | 离心 | Atomic | 否,不拆加速/减速 | 转子/RCF 绑定 |
| 7 | 离心后取上清 | Atomic(转移) | 与离心分开 | 新意图/new material transfer |
| 8 | 吸光度读取 | Atomic | 否 | raw OD only |
| 9 | 荧光图像采集 | Atomic | 否 | raw image only |
| 10 | 划线接种 | Atomic | 否,不拆每一笔 | 空间分布 |
| 11 | 涂布接种 | Atomic variant | 否,不拆移动 | 表面接种 |
| 12 | 菌落计数 | Atomic readout | 否 | 计数≠CFU 计算 |
| 13 | 细胞浓度计数 | Atomic readout | 否 | 活力判读另立 |
| 14 | 连续稀释 | Composite | 拆成多次液体转移 | 不建“稀释”原子 |
| 15 | 细胞传代 | Composite Block | 拆 | 洗涤/解离/离心/计数/接种/孵育 |
| 16 | 静态生物膜培养 | Composite/method phase | 拆 | 接种+孵育+换液;附着/浮游 fractions |
| 17 | 生物膜 CV | Method Template | 拆 | 洗涤/染色/溶解/读数 |
| 18 | 生物膜 CFU | Method Template | 拆 | 洗涤/分散/稀释/铺板/培养/计数/CFU |
| 19 | CCK-8 / ELISA / WB / qPCR / 流式 / IF | Method Template | 拆 | 方法特定绑定和终点 |
| 20 | 生物膜形成 / 细胞活力 / 蛋白表达 | Assay Model | 不拆成模块 | 科学问题/measurand |

## 5. 五场景验证

[granularity-scenarios.md](granularity-scenarios.md) 用五类场景验证:

1. L929 CCK-8:多时间点、参数绑定、重复测量、读数。
2. 细胞传代:复合块、谱系、培养阶段。
3. 细菌划线/涂布:空间分布、来源、接种结果。
4. 生物膜 CV/CFU:共享前期、不同终点、不同样本处理。
5. qPCR/ELISA:多级复合、方法约束、结果计算。

结论:同一套四层结构可以描述五类场景;不能表达的部分集中到数量守恒、谱系生成、条件执行、状态类型、损失、跨字段校验、方法对照、时间锚点。

## 5.1 具体实验的复用优先流程

新增实验记录必须附 Reuse Map:先列已有的 Atomic Operation/Composite/Method,再列“参数化复用/变体/新操作/Custom Step/ToolAction”。只有不能由已有操作+参数表达时,才新建原子操作。详见 [record-standard §1.5](../record-standard.md#15-具体实验的拆分流程必须先查复用)。

## 5.2 原子性 ≠ 公共 Catalog

原子操作只是步骤建模粒度;进入全局 Catalog 还要过“晋升门槛”:

| 维度 | 标准 |
|---|---|
| 复用广度 | ≥2 个独立方法有真实使用;可在同一技术领域内,不必跨实验大类;同一方法变体不算 |
| 语义稳定 | 核心输入/输出/作用跨方法不变 |
| 抽象收益 | 减少重复定义/校验/记录 |
| 差异成本 | 不需要大量专用分支 |

不满足 → Method-local Step(继续 `ProtocolStep::Text`),不建公共模块。涉及样本谱系/安全/审计的关键操作可例外,但需记录理由。

## 5.3 方法内部步骤

| 对象 | 决定 |
|---|---|
| `CAND-GAP-CREATE`(划痕/Insert 形成间隙) | Method-local Step;划痕与 Insert 机制不同,不合并为公共模块 |
| `CAND-APICAL-CLEAR`(Transwell 上表面清除) | Method-local Step;方法特定,暂不注册 |
| 3D 基质 dome 点胶/包埋 | Method-local Step;留在 3D 方法模板 |
| ULA 球体形成 | 方法阶段/确认条件,不是原子模块 |
| 自动移液吸头/吹出/气隙 | 设备参数/指令,不是科学操作模块 |
| 离心后取上清 | 复用取样/液体转移,不新增模块 |

## 6. 冻结规则

1. 新候选先过四问;失败进 Composite/Method/Assay/Custom,不硬塞 Atomic。
2. 通过四问的原子操作仍不等于公共模块;必须过 §5.2 晋升门槛,否则留在 Method-local。
3. 不因为新实验出现就新增通用字段;先看参数/变体/自定义步骤能否表达。
4. 不把所有实验强行拆成已有模块;允许条件分支、人工判断、Unknown。
5. 前端可以显示 Composite 任务;内部引用多个 Atomic,不要求用户看技术组件。
6. 方法模板只记录与通用操作的差异和绑定,不重复通用操作研究。
7. 同一原子操作内的可变量优先参数化,不要按试剂/次数/吹打方式复制模块;只有样本 effect/意图改变时才建 variant 或新操作。
8. 任何模块/方法升级前需第二人来源复核;AI 预审不算。

## 7. 未决

- 取样/分样的数量守恒是否进入 Core 类型;当前用 note/record fields。
- lossy wash 的损失量是否需要 typed `loss` effect。
- `Custom Step` 最终 Schema 与 Run 记录字段。
- 甲基化/测序/动物/临床等未覆盖领域是否适用四层结构。
