# Scientific Fact-Aware Components 研究记录

- 日期:2026-11-06
- 状态:研究记录,待审;不是实现记录,不是已冻结规范。
- 范围:记录用户提出的“科学事实直接参与组件交互、计算和校验”方向,以及它与 Component Definition / LabwareProfile / ContainerPlan / ToolAction 的关系。
- 前置:[组件体系与 LabwareProfile 设计评审](2026-11-05-component-profile-and-labware-design-review.md)、[Unified Component Framework v1](../contracts/component-framework-v1.md)。

## 0. 一句话

组件不只需要“显示什么”,还需要在明确、可追溯、带单位的科学事实基础上参与计算和检查;但事实、建议、设计参数、派生值和实测值必须分开,缺少事实时保持 unknown。

## 1. 五层对象不能混用

| 层 | 例子 | 权威归属 |
|---|---|---|
| Component Definition | `lab.well_plate`、`content.table`、`tool.calculator` | 组件注册表 |
| Profile | 具体 96 孔培养板、PCR 板、Transwell Insert 的规格 | LabwareProfile Registry |
| Plan | Plate-01、A1 分配给某组/样本/反应 | DesignVersion / ContainerPlan |
| Derived | 每孔细胞数、总培养基、Master Mix 总量 | ToolAction / 计算结果 |
| Actual | 实际加液量、实测 OD、实际使用的孔 | Run / ExecutionEvent |

`Profile` 不是组件实例;`Plan` 不是 ViewDocument;`Derived` 不是科学事实;`Actual` 不能由 View 自动生成。

## 2. Scientific Property:属性必须有类型、单位、来源、适用范围

`ProfileField` 的 `known / unknown / not_applicable / conflicted` 继续保留,但科学量值应逐步从任意 JSON 变为:

```text
ScientificQuantity {
  value, unit
}

ProfileField {
  state,
  quantity?,
  range?,
  evidence[],
  applies_to?,
  note?
}
```

属性分类:

| 类别 | 例子 | 使用方式 |
|---|---|---|
| 结构几何 | 孔行列、孔底直径、孔距 | 坐标/几何验证 |
| 产品物理属性 | 培养面积、总容量、材料、表面处理 | 材料/容量约束 |
| 厂商建议 | 工作体积范围、温度范围 | 提醒或条件检查,不是强制 SOP |
| 实验设计参数 | 每孔接种密度、计划加液量 | 研究者设定,DesignVersion |
| 派生结果 | 每孔细胞数、试剂总量 | 确定性 ToolAction 计算 |
| 实际测量 | 实际加液量、实测 OD | Run/ExecutionEvent |

用户提出的具体参考(仅代表对应产品类别,不是全行业默认):

| 耗材规格 | 单孔培养面积 | 单孔总容量 | 建议工作体积 |
|---|---:|---:|---:|
| Corning 6 孔培养板† | 9.5 cm² | 16.8 mL | 1.9–2.9 mL |
| Corning 24 孔培养板† | 1.9 cm² | 3.4 mL | 0.38–0.57 mL |
| Corning 96 孔平底微孔板(3596 类) | 0.32 cm² | 360 μL | 75–200 μL;培养常用 100–200 μL |
| Corning 96 孔 Half-Area 板† | 0.16 cm² | 190 μL | 50–100 μL |

这些值必须进入**具体产品 Profile**,不能写成“96 孔通用值”。普通 96 孔、圆底孔、Half-Area、不同表面处理/品牌不能互相替代。来源需在 Profile 中逐字段记录;本表来自用户提供的 Corning 资料,正式采用前需重新核对型号与文档。

### 2.1 外部资料核对结果(2026-11-06)

已抽查 Corning 3596 产品页与 Corning CLS-AN-209:

- 96 孔平底 TC 处理板 3596:总孔容约 **360 μL**,推荐工作体积 **75–200 μL**;细胞培养常用 **100–200 μL**;生长面积约 **0.32 cm²/孔**。
- 因此“96 孔平底 = 100–200 μL”不能作为所有 96 孔板的通用值;同一族产品也有 75–200 μL 的表述,且不同型号/孔底/表面不同。
- 6 孔、24 孔、Half-Area 的数值同样必须绑定具体产品;本次未逐项完成所有型号核对。

已抽查 Thermo Fisher PCR 耗材资料:

- 96 孔 PCR 板分标准型/Low-Profile、非裙边/半裙边/全裙边等;
- 兼容性取决于热循环仪型号、板高、裙边、封膜/自动化;
- 不能从“96 孔”推导 PCR 仪器兼容性;必须使用厂商 Selection Tool 或明确兼容表。

结论:方案可行,但表中的数值只能作为**候选 Profile 数据**;正式 Profile 必须逐型号、逐字段记录来源与核对状态。† 6/24 孔与 Half-Area 数值本次未独立核对,不能直接进入正式 Profile。未知或冲突不能补默认值。

标准尺寸(ANSI/SLAS footprint/height/well position)只证明几何标准,不证明培养面积、表面处理、工作体积或仪器兼容性。

## 3. Scientific Constraint:规则必须有级别和依据

规则不是“模型觉得应该如此”,而是 Rust Core 中明确、测试过的确定性检查。

输出建议:

```text
ScientificAssessment {
  status: valid | warning | blocked | unresolved,
  rule_id,
  subject_ref,
  observed_quantity?,
  applicable_limit?,
  explanation,
  evidence_refs[],
  affected_components[]
}
```

语义:

- `blocked`:物理/定义上的明确冲突,不能保存或必须修正。
- `warning`:超出厂商建议/实验条件范围,允许保存但要求复核。
- `unresolved`:缺少必要事实,不能宣称通过。
- `valid`:已有足够事实且落在规则允许范围。

不能把 `valid` 解释为“实验一定成功”;它只表示该规则在当前事实下通过。

第一版只做三条规则:

1. **培养面积 → 铺板细胞数**
   - 输入:Profile 的 `growth_area_per_well`(cm²),用户目标密度(cells/cm²)。
   - 输出:每孔理论细胞数(cells/well)。
   - 缺少面积或密度 → unresolved;单位不兼容 → blocked。
   - 不自动考虑贴壁率、损失、细胞类型差异。

2. **工作体积范围**
   - 输入:Profile 的 `working_volume_range`,用户计划体积。
   - 超出 → warning;单位不符 → blocked;范围未知 → unresolved。
   - 总容量、工作体积、实验实际加液量是三个不同概念。

3. **PCR 板/仪器兼容性**
   - 输入:PCR Profile 的板高、裙边、封膜/兼容设备,以及具体仪器型号。
   - 未知 → unresolved;明确不兼容 → blocked;匹配 → valid。
   - 不能因为都是 96 孔就认为兼容。

后续可加 Transwell 膜面积/孔径/上下室、细胞爬片、管架、离心兼容性,但必须逐条有来源与测试。

## 4. Component Interactions:事实如何参与组件

```text
PlannedContainer + Profile
        ↓
WellPlate 组件(只读展示计划孔位)
        ↓
Calculator 组件(Pure ToolAction,读取明确输入)
        ↓
Table 组件(展示结果、单位、规则、来源、未知项)
        ↓
Protocol/Operation(需要科学变更时走正式 Core 命令)
```

规则:

- WellPlate 不能从 View Props 覆盖 Profile。
- Calculator 只能使用明确输入;缺少每孔体积/密度时返回 unresolved,不能填默认值。
- Table 是表达层;authored 表可用于 draft 计算,但不能自动成为 DesignVersion。
- 计算结果是 derived;不能覆盖计划孔位或实验参数。
- 组件可以共享同一 `PlannedContainer` 引用;显示多次不等于多个实体板。
- 科学冲突由 Core 规则评估,不由前端“看起来合理”判定。

## 5. 现有代码的落点

已有:

- `LabwareProfile` / `ProfileField` 四态、精确 ProfileRef;
- `PlannedContainer` / `PositionAssignment` / `ContainerPlacement`;
- DesignVersion `container_plan`、Run 的 `container_plan + labware_profiles` 冻结;
- `tool.calculator` / `content.table` Definition;
- `ViewChangeSet` 与 `DataBinding`。

需要补:

- `ProfileField` 的 typed `quantity` / `range`;
- `ScientificAssessment` 规则评估;
- 具体产品 Profile(新建身份/版本,不改已有 research_draft@1;Corning 3596 类已抽查,6/24/Half-Area 待核对);
- WellPlate/Calculator/Table 的真实数据适配;
- 校验状态在 UI/API 中的显示;
- 具体型号/文档的来源核对。

## 6. 不建议做的事

- 不建立通用科学本体/推理引擎/规则 DSL。
- 不把厂家建议直接当强制 SOP。
- 不把“96 孔”当所有板共有的培养面积/体积。
- 不把派生值写成实测值。
- 不让 Agent 用 View Props 或 authored table 自动覆盖科学事实。
- 不在缺少事实时猜默认体积/面积/兼容性。

## 7. 建议首个最小闭环

1. 新建一个有明确来源的具体 96 孔培养板 Profile(新 ID 或新版本,保留旧 Profile)。
2. 给 `growth_area_per_well`、`working_volume_range` 增加 typed quantity/range。
3. 实现两条规则:铺板细胞数、工作体积范围;PCR 兼容性随后。
4. `view.data_preview`/WellPlate 读取 Profile 和 ContainerPlan,Calculator 只使用明确输入,Table 显示结果、单位、规则、来源、unknown。
5. 测试:
   - 15 个孔、每孔 10 μL → 150 μL;
   - 缺少每孔体积 → unresolved;
   - 超出 100–200 μL → warning;
   - 没有培养面积 → unresolved;
   - 切换 Profile/型号 → 明确提示复核,不自动继承。

## 8. 来源与核对状态

- 用户提供的 Corning 细胞培养耗材参考表:仅作候选数据,正式 Profile 需核对产品型号、文档和页面日期。
- 已抽查:[Corning 3596 产品页](https://ecatalog.corning.com/life-sciences/b2c/US/en/Microplates/Assay-Microplates/96-Well-Microplates/Corning%C2%AE-96-well-Clear-Polystyrene-Microplates/p/3596)、[Corning CLS-AN-209](https://www.corning.com/catalog/cls/documents/application-notes/CLS-AN-209.pdf)、[Corning Microplate Selection Guide](https://www.corning.com/media/worldwide/cls/documents/CLS-C-DL-MP-014REV9.pdf)。
- 用户提供的 Thermo Fisher PCR 耗材说明:仅作候选,需核对标准型/Low-Profile、裙边和仪器兼容表。已抽查:[Thermo PCR plates](https://www.thermofisher.com/us/en/home/brands/thermo-scientific/molecular-biology/thermo-scientific-pcr/pcr-plates-and-qpcr-plates.html)、[Thermo PCR plastics selection guide](https://assets.thermofisher.com/TFS-Assets/LSG/brochures/thermo-scientific-pcr-plastics-selection-guide.pdf)、[AB0800](https://www.thermofisher.com/order/catalog/product/AB0800)。
- [SLAS Microplate Standards](https://www.slas.org/education/ansi-slas-microplate-standards/)
- [ANSI/SLAS 1-2004 footprint](https://www.slas.org/slas/assets/file/public/standards/ansi_slas_1-2004_footprintdimensions.pdf)
- [ANSI/SLAS 2-2004 height](https://www.slas.org/SLAS/assets/File/public/standards/ANSI_SLAS_2-2004_HeightDimensions.pdf)
- 现有代码事实以仓库当前提交为准;本文件不把用户表格当作已独立复核的厂商事实。

## 9. 评审问题

1. 首个具体 Profile 选择哪个品牌/型号?必须有可核对的官方表格。
2. `growth_area_per_well` 是否只对贴壁培养板适用?悬浮/圆底孔应保持 not_applicable 或 unknown。
3. 工作体积 warning 是否允许保存草稿?建议允许,但未确认前不宣称 ready。
4. PCR 兼容性用“设备型号 + 板型号”规则,还是先只记录兼容声明?
5. Calculator 的 `tool_ref` 与 `ToolActionSpec` 如何绑定到具体算法版本?
6. Table 的规则/来源行如何避免成为第二份科学事实?
