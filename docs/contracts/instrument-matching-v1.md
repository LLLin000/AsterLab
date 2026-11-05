# Instrument Matching v1(K2 匹配引擎草案)

- 日期:2026-11-05
- 阶段:K2;纯 Core 匹配引擎 + 合成测试夹具。
- **没有真实 InstrumentProfile 被标为 SourceChecked。** 当前测试只用合成 profile,不能当仪器规格。
- 关联:[OperationModule v1.1](operation-module-v1.md)、[K0 审计](../research/2026-11-05-k0-knowledge-coverage-audit.md)。
- 代码:[`crates/asterlab-core/src/instrument_matching.rs`](../../crates/asterlab-core/src/instrument_matching.rs)

## 0. 目标

判断“这个操作要求”与“这个仪器能力档案”之间是:

- `Compatible`:已有、已核验的证据满足全部已知要求。
- `Unknown`:资料不足、型号未知、单位不能换算或某项能力未记录。
- `Incompatible`:存在明确的、有来源的冲突。

不返回裸布尔值;每个判定必须带 `MatchReason`。`Compatible` 也有原因。

## 1. 当前实现

```rust
pub struct InstrumentProfile {
    pub schema_version: u32,
    pub profile_id: String,
    pub version: u32,
    pub class: String,
    pub model: Option<String>,
    pub capabilities: Vec<InstrumentCapability>,
    pub container_kinds: Vec<String>,
    pub source: ProfileSource, // built_in | lab_imported
    pub evidence: Vec<EvidenceRef>,
    pub review_status: ReviewStatus,
}

pub struct InstrumentCapability {
    pub key: String,
    pub value: CapabilityValue,
    pub unit: Option<String>,
    pub conditions: Vec<String>,
    pub evidence: Vec<EvidenceRef>,
}

pub enum CapabilityValue {
    Exists,
    Unsupported,
    Number { value: f64 },
    Range { min: f64, max: f64 },
    Text { value: String },
    Set { values: Vec<String> },
}
```

`InstrumentProfile::validate()` 做结构校验:

- schema/version/ID/class;
- capability key 唯一;
- 数值与范围有限、`min <= max`;
- 数字/范围才允许单位;
- set 非空、text 非空;
- 非 `research_draft` 至少有 evidence。

它**不**证明厂商规格正确;真实 profile 仍需 K2 的来源审核。

## 2. 匹配接口

```rust
pub fn match_requirement(
    requirement: &InstrumentRequirement,
    profile: Option<&InstrumentProfile>,
    requested: Option<&ProfileRef>,
    container: Option<&ContainerRequirement>,
) -> MatchResult;
```

```rust
pub struct MatchResult {
    pub status: MatchStatus, // compatible | unknown | incompatible
    pub reasons: Vec<MatchReason>,
}

pub struct MatchReason {
    pub code: String,
    pub path: String,
    pub message: String,
    pub source: MatchSource, // requirement | profile | profile_absent | unit_unknown | evidence | container
}
```

## 3. 判定顺序

1. **profile 缺失** → `Unknown` `profile_missing`。
2. **profile 结构无效** → `Unknown` `profile_invalid`。
3. **精确引用**
   - requirement 带 `profile_id + profile_version` 时,必须与 profile 相同;版本不同 → `Unknown` `profile_version_mismatch`。
   - `profile_id` 没有版本 → `Unknown` `profile_version_missing`。
   - 调用方另给 `requested` 时也做精确匹配。
4. **类别**
   - class 不同:
     - profile 已 source-checked → `Incompatible` `class_mismatch`;
     - profile 未 source-checked → 仍最多 `Unknown` `class_mismatch_unverified`。
   - 当前没有 class 别名映射;字符串不同即不同,不猜别名。
5. **审核门**
   - profile 低于 `source_checked` 或没有 evidence → 总体最多 `Unknown` `profile_not_source_checked`。
   - 单个 capability 没有 evidence → `Unknown` `capability_evidence_missing`,不能判 `Compatible`。
   - 不能因为能力值“看起来满足”就判 `Compatible`。
6. **容器**
   - profile 没记录容器 → `Unknown` `container_unknown`。
   - `container_kinds` 视为封闭列表;非空但未列出请求容器 → `Incompatible` `container_mismatch`。
7. **能力**
   - profile 没有该 capability → `Unknown` `capability_missing`。
   - profile 明确 `Unsupported` → `Incompatible` `capability_unsupported`。
   - 数值/范围按单位完全一致比较;单位不同 → `Unknown` `unit_conversion_unknown`,不判不兼容。
   - 范围包含/相交规则:
     - `Min`:profile 最大可达值 ≥ 要求下限;
     - `Max`:profile 最小可达值 ≤ 要求上限;
     - `Range`:profile 范围包含要求范围;或 profile 数值落在要求范围内。
   - `OneOf`:profile set/text 与请求值有交集;否则明确不匹配。
8. **汇总**
   - 任一 `Incompatible` → 总体 `Incompatible`。
   - 否则任一 `Unknown` → 总体 `Unknown`。
   - 全部有原因且通过 → `Compatible`。

## 4. 当前不做的

- 不做单位换算;`µL`/`mL` 无换算依据时返回 `Unknown`。
- 不做 class 别名映射;未知类别不猜。
- 不生成 Run 实际设置;匹配通过不代表校准、耗材或当前状态已确认。
- 不把 `Compatible` 写成科学结论。
- 不创建真实 profile;真实型号资料必须逐页核对。

## 5. 首批真实 profile 范围

只研究四类,每类先选一个明确型号:

| 类别 | 候选型号 | 必要记录 |
|---|---|---|
| 手动移液器 | Eppendorf Research plus | 量程、通道、吸头、液体类型、来源页码 |
| CO₂ 培养箱 | Thermo Heracell VIOS 160i | 温度、CO₂、湿度、容器/透气盖、来源页码 |
| 吸光度读板机 | Thermo Multiskan SkyHigh | 波长、板型、模式、温度、线性范围、来源页码 |
| 荧光显微镜 | Leica DMi8 | 物镜、滤光片/通道、工作距离、曝光/相机、来源页码 |

真实 profile 必须同时记录:

- 厂商、型号、配置/版本;
- 每个能力值的单位和适用范围;
- 手册/规格表 URL、页码/章节、检索日期;
- 明确未知项;
- `review_status: research_draft`,直到逐页核对完成。

## 6. K2 验收

- 同一要求 + 同一 profile 在不同项目得到同一结果。
- 缺少 profile → `Unknown`,不是 `Compatible`。
- 明确冲突 → `Incompatible`,带原因。
- 单位未知 → `Unknown`,不误判不兼容。
- 档案未记录能力 → `Unknown`,不判不支持。
- `research_draft` 档案不能产生 `Compatible`。
- 每条结果都有 `MatchReason`;没有裸布尔值。
- profile 结构校验拒绝重复 capability、非法范围、无证据的 `source_checked`。
- 不修改 CLI/Protocol/Run/Slint;先做纯 Core 类型与测试。

当前 19 个合成测试覆盖以上状态;它们只证明匹配逻辑,不证明任何真实仪器。

## 7. 未决项

1. 四类真实 profile 的来源核对;下一轮按 K0 审核计划逐页做。
2. class 别名/映射的来源与审核;不能凭 Agent 记忆。
3. 容器/耗材是否独立 `LabwareProfile`;首版只用 `container_kinds` 字符串。
4. 单位换算表是否进入 K2;首版只做“未知”。
5. `InstrumentProfile` 与 `ProtocolBlockVersion.module_contract` 的存储位置;K3 决定。
6. 审核记录(审核者、时间、内容摘要)的存储;K3。

## 8. 明确不做

- 不批量创建十几个 profile。
- 不根据几个数值范围写“万能比较器”。
- 不实现通用仪器控制。
- 不把匹配结果写进 Run 实际设置。
- 不修改 Slint。
