# Instrument Matching v1(K2/K2.1 匹配引擎)

- 日期:2026-11-05
- 阶段:K2.1;纯 Core 匹配引擎 + 两个结构校验 JSON 草稿 + 合成测试。
- **没有真实 InstrumentProfile 被标为 SourceChecked。** 所有真实记录保持 `research_draft`,不能产生 `Compatible`。
- 关联:[OperationModule v1.1](operation-module-v1.md)、[K0 审计](../research/2026-11-05-k0-knowledge-coverage-audit.md)。
- 代码:[`crates/asterlab-core/src/instrument_matching.rs`](../../crates/asterlab-core/src/instrument_matching.rs)
- JSON 草稿:`catalog/operations/`、`catalog/instruments/`
- 来源矩阵:[field-evidence-matrix](../research/k2-operations/field-evidence-matrix.md)

## 0. 目标

判断“这个操作要求”与“这个仪器能力档案”之间是:

- `Compatible`:已有、经审核记录覆盖的证据满足全部已知要求。
- `Unknown`:资料不足、型号/配置未知、单位不能换算、能力未记录、条件未验证。
- `Incompatible`:存在明确的、有来源的冲突。

不返回裸布尔值;每个判定必须带 `MatchReason`。`Compatible` 也有原因。

## 1. 类型

```rust
pub struct InstrumentProfile {
    pub schema_version: u32,
    pub profile_id: String,
    pub version: u32,
    pub class: String,
    pub model: Option<String>,
    pub capabilities: Vec<InstrumentCapability>,
    pub container_support: ContainerSupport,
    pub source: ProfileSource,
    pub evidence: Vec<EvidenceRef>,
    pub review_status: ReviewStatus,
    pub review_record: Option<ReviewRecord>,
}

pub struct InstrumentCapability {
    pub key: String,
    pub value: CapabilityValue,
    pub unit: Option<String>,
    pub conditions: Vec<String>,
    pub evidence: Vec<EvidenceRef>,
}

pub enum ContainerSupport {
    Unspecified,
    Partial { listed: Vec<String> },
    Complete { listed: Vec<String> },
    ExplicitlyUnsupported { kinds: Vec<String> },
}

pub struct ReviewRecord {
    pub review_id: String,
    pub reviewer: String,
    pub reviewed_at: String,
    pub subject: ProfileRef,
    pub scope: Vec<String>,
    pub evidence_ids: Vec<String>,
    pub outcome: ReviewOutcome, // approved | changes_requested | rejected
}
```

`InstrumentProfile::validate()` 检查结构、ID、数值范围、单位、容器支持列表、证据和审核记录;它不证明厂商规格正确。

## 2. 审核门:声明不等于证明

`review_status` 只是模板声明。真正允许 `Compatible` 的判定必须满足 `InstrumentProfile::is_review_verified()`:

- `review_status >= source_checked`;
- 至少一条 profile evidence;
- 存在 `ReviewRecord`,且:
  - `outcome = approved`;
  - `subject.profile_id + version` 与 profile 完全一致;
  - `reviewer`, `reviewed_at`, `scope`, `evidence_ids` 非空;
  - 引用的 evidence id 存在于 profile。

没有 `ReviewRecord` 的 `source_checked` profile 在 `validate()` 阶段即报 `review_record_required`;即使绕过结构校验,匹配仍返回 `Unknown` `profile_not_review_verified`。

`ResearchDraft` 永远不能产生 `Compatible`;即使 capability 看起来满足,总体最多 `Unknown`。

## 3. 匹配顺序

1. **profile 缺失** → `Unknown` `profile_missing`。
2. **profile 结构无效** → `Unknown` `profile_invalid`。
3. **精确引用**
   - requirement 带 `profile_id + profile_version` 时必须匹配;不同 → `Unknown` `profile_version_mismatch`。
   - 只有 version 没有 id → `Unknown` `profile_version_missing`。
4. **审核门**
   - `is_review_verified()` 为 false → 总体最多 `Unknown`,reason `profile_not_review_verified`。
5. **类别**
   - class 不同:
     - 已审核 profile → `Incompatible` `class_mismatch`;
     - 未审核 profile → `Unknown` `class_mismatch_unverified`。
   - 当前没有 class 别名映射;字符串不同即不同。
6. **容器**
   - `Unspecified` → `Unknown` `container_unspecified`;
   - `Partial` 未列出 → `Unknown` `container_not_listed`(不能把部分列表当完整列表);
   - `Complete` 未列出 → `Incompatible` `container_mismatch`;
   - `ExplicitlyUnsupported` 命中 → `Incompatible` `container_explicitly_unsupported`。
7. **能力**
   - profile 无该 capability → `Unknown` `capability_missing`;
   - capability 没有 evidence → `Unknown` `capability_evidence_missing`;
   - `conditions` 非空 → `Unknown` `capability_conditions_unverified`;第一版不假装知道条件已满足;
   - 明确 `Unsupported` → `Incompatible` `capability_unsupported`;
   - 数值/范围按单位完全一致比较;单位不同 → `Unknown` `unit_conversion_unknown`;
   - `Min`:profile 最大可达值 ≥ 要求下限;
   - `Max`:profile 最小可达值 ≤ 要求上限;
   - `Range`:profile 范围包含要求范围,或 profile 数值落在要求范围内;
   - `OneOf`:profile set/text 与请求值有交集。
8. **汇总**
   - 未审核 → `Unknown`(即使有冲突也不升级);
   - 否则任一 `Incompatible` → `Incompatible`;
   - 否则任一 `Unknown` → `Unknown`;
   - 全部通过且原因非空 → `Compatible`。

## 4. 当前 JSON 草稿

五个操作、两个 profile:

| Artifact | Path | 状态 |
|---|---|---|
| `op.liquid_transfer` v1 | `catalog/operations/liquid-transfer.v1.json` | structural valid; conditional transfer modes |
| `op.sample_aliquot` v1 | `catalog/operations/sample-aliquot.v1.json` | structural valid; parent/child |
| `op.incubation` v1 | `catalog/operations/incubation.v1.json` | structural valid; time anchor/state change |
| `op.washing` v1 | `catalog/operations/washing.v1.json` | structural valid; state change/waste |
| `op.absorbance_measurement` v1 | `catalog/operations/absorbance-measurement.v1.json` | structural valid; generic readout |
| Research plus 100–1000 µL single-channel | `catalog/instruments/liquid-handler.eppendorf-research-plus.single-channel.100-1000.v1.json` | structural valid; no review record |
| Multiskan SkyHigh | `catalog/instruments/plate-reader.multiskan-skyhigh.v1.json` | structural valid; no review record; one capability has conditions |

这些 JSON 只经过结构校验,没有通过第二人来源复核。集成测试证明:

- 五个操作通过 `OperationModule::validate_contract()`;
- 两个 profile 通过 `InstrumentProfile::validate()`;
- 两个 profile 都不能与草稿操作产生 `Compatible`(只能 `Unknown`)。

## 5. 测试

匹配模块 23 个测试;操作模块 26 个测试;新增集成测试 3 个。覆盖:

- 缺失/无效/版本不匹配 profile;
- 审核记录缺失 → 不能 Compatible;
- `ResearchDraft` 即使能力满足也最多 Unknown;
- class 冲突优先于版本冲突;
- capability 缺证据、conditions 非空 → Unknown;
- 容器 `Partial`/`Complete`/`ExplicitlyUnsupported`/`Unspecified` 四态;
- 单位不同 → Unknown;
- 范围不重叠 → Incompatible;
- profile 结构校验拒绝重复 capability、非法范围、空单位;
- 五条草稿操作/两条草稿 profile 的结构校验。

## 6. 明确不做

- 不做单位换算;首版无换算依据就 Unknown。
- 不做 class 别名映射。
- 不做条件规则引擎;有 conditions 即 Unknown。
- 不生成 Run 实际设置;匹配通过不代表校准、耗材或当前状态已确认。
- 不批量创建 profile;只做两个代表性 JSON 草稿。
- 不进入 K3;不改 Slint/CLI/正式数据库。

## 7. 未决项

1. 两份 JSON 草稿的第二人逐字段来源复核。
2. 条件类型化(安装组件、滤光片、光源、相机)与可验证满足记录。
3. 单位换算表;首版只做“未知”。
4. `ReviewRecord` 的持久化与审批流程;K3 前完成。
5. 容器/耗材是否独立 `LabwareProfile`。
6. 其他操作/仪器的来源审计。
