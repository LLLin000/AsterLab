# Instrument Matching v1(K2 接口提案)

- 日期:2026-11-05
- 阶段:K2 前置提案;不实现引擎、不创建批量 InstrumentProfile。
- 关联:[OperationModule v1.1](operation-module-v1.md)、[K0 审计](../research/2026-11-05-k0-knowledge-coverage-audit.md)。
- 状态:待 K1.1 审查通过后再实现。

## 0. 目标

判断“这个操作要求”与“这个仪器能力档案”之间是:

- `Compatible`:已有证据满足全部已知要求。
- `Unknown`:资料不足、型号未知、单位不能换算或某项能力未记录。
- `Incompatible`:存在明确的、有来源的冲突。

不返回裸布尔值;每个判定必须带原因。

## 1. 输入

```rust
pub struct MatchRequest {
    pub requirement: InstrumentRequirement, // OperationModule.execution
    pub profile: Option<InstrumentProfile>,
    pub requested_profile: Option<ProfileRef>,
    pub container: Option<ContainerRequirement>,
}
```

`InstrumentProfile` 仍沿用 Core Contract 的方向:

```rust
pub struct InstrumentProfile {
    pub profile_id: String,
    pub version: u32,
    pub class: String,
    pub model: Option<String>,
    pub capabilities: Vec<Capability>,
    pub constraints: Vec<Constraint>,
    pub source: ProfileSource,
    pub evidence: Vec<EvidenceRef>,
    pub review_status: ReviewStatus,
}
```

`Capability` 至少要有 `key`、`value`/范围、`unit`、适用条件、证据。不能用一个无来源的数值范围。

## 2. 输出

```rust
pub enum MatchStatus {
    Compatible,
    Unknown,
    Incompatible,
}

pub struct MatchResult {
    pub status: MatchStatus,
    pub reasons: Vec<MatchReason>,
}

pub struct MatchReason {
    pub code: String,
    pub path: String,
    pub message: String,
    pub source: MatchSource,
}

pub enum MatchSource {
    Requirement,
    Profile,
    ProfileAbsent,
    UnitUnknown,
    Evidence,
}
```

`Compatible` 也必须带原因;`Unknown`/`Incompatible` 不能只有一个空状态。

## 3. 判定顺序

1. **精确 profile 引用**
   - 要求有 `profile_id + profile_version` 时,必须匹配同一个 `profile_id + version`。
   - 版本不同 → `Unknown`(不能证明兼容),并给出 `profile_version_mismatch`。
   - 档案缺失 → `Unknown`(`profile_missing`)。
   - 未指定 profile(只有 class)→ 走类别能力检查,不假设某型号。

2. **类别**
   - class 相同或存在明确、有来源的别名 → 继续。
   - class 明确不同 → `Incompatible`(`class_mismatch`)。
   - 别名/类别映射缺失 → `Unknown`(`class_unmapped`)。

3. **能力存在性**
   - 要求 `exists`:档案明确声明能力 → `Compatible`;明确声明不支持 → `Incompatible`;未记录 → `Unknown`。
   - 不能因为档案没写就当作不支持。

4. **数值范围**
   - 单位完全相同:比较 min/max。
   - 已知不重叠:要求下限 > 档案上限,或要求上限 < 档案下限 → `Incompatible`。
   - 单位不同但没有换算依据 → `Unknown`(`unit_conversion_unknown`),不能判不兼容。
   - 档案范围缺失 → `Unknown`(`capability_range_missing`)。

5. **容器/耗材**
   - 明确支持 → 通过。
   - 明确不支持 → `Incompatible`。
   - 未记录 → `Unknown`;不能从板型名称推断。

6. **证据/审核**
   - 档案 `research_draft` 或没有来源:匹配结果最多 `Unknown`;不能因为“看起来满足”判 `Compatible`。
   - 只有 `source_checked` 以上、且能力字段有定位证据,才可判 `Compatible`。

7. **汇总**
   - 任一 `Incompatible` → 总体 `Incompatible`。
   - 否则任一 `Unknown` → 总体 `Unknown`。
   - 全部 `Compatible` → `Compatible`。

## 4. 首批范围

只做四类:

| 类别 | 需要验证的能力 |
|---|---|
| 手动移液器 | 体积范围、通道、吸头、液体类型 |
| CO₂ 培养箱 | 温度、CO₂、湿度、容器/透气盖 |
| 吸光度读板机 | 波长、板型、读数模式、温度、线性范围 |
| 荧光显微镜 | 物镜、通道/滤光片、容器工作距离、曝光 |

首批不做:离心机全型号、qPCR 全通道、流式全 panel、自动液体处理。等前四类匹配稳定后再扩。

## 5. 与 OperationModule 的关系

- 输入是 `InstrumentRequirement`,不是实验名称。
- 输出只说明“能力是否匹配”,不生成 Run 实际设置。
- 匹配通过不等于校准/耗材/当前状态已确认;这些仍需 Run 前检查。
- `Compatible` 不能自动填波长/转速/体积;只证明能力边界满足。

## 6. K2 验收

- 同一要求 + 同一 profile 在不同项目得到同一结果。
- 缺少 profile → `Unknown`,不是 `Compatible`。
- 明确冲突 → `Incompatible`,带原因。
- 单位未知 → `Unknown`,不误判不兼容。
- 档案未记录能力 → `Unknown`,不判不支持。
- `research_draft` 档案不能产生 `Compatible`。
- 每条结果都有 `MatchReason`;没有裸布尔值。
- 不修改 CLI/Protocol/Run/Slint;先做纯 Core 类型与测试。

## 7. 未决项

1. `InstrumentProfile` 的最终 Schema 与 profile 版本存储;K2 定义。
2. 单位换算表是否进入 K2;首版只做“未知”,不做自动换算。
3. class 别名/映射的来源与审核;不能凭 Agent 记忆。
4. 容器兼容是否合并进 profile 或独立 `LabwareProfile`;K2 决定。
5. 已有 `docs/prototypes/instrument-step-modules.md` 的型号摘录仍需逐页核对,不能直接当 `Source Checked`。

## 8. 明确不做

- 不批量创建十几个 profile。
- 不根据几个数值范围写“万能比较器”。
- 不实现通用仪器控制。
- 不把匹配结果写进 Run 实际设置。
- 不修改 Slint。
