//! Readout schema v1: one structure for numeric, table, plate, categorical and
//! file readouts.
//!
//! A [`ReadoutSchema`] describes shape, unit, missing-value semantics,
//! provenance and timepoint/sample association. A [`Readout`] is one value
//! recorded against a schema.
//!
//! Nothing here writes a record. [`to_legacy_readouts`] is a pure projection
//! onto the frozen `execution_event.create` `readouts[]` form, and existing
//! events are never rewritten.

use crate::reading_facts::ReadingTarget;
use serde::{Deserialize, Serialize};

/// Schema version of the readout contract. Bump only for a breaking field change.
pub const READOUT_SCHEMA_VERSION: u32 = 1;

/// The five readout shapes. `table`, `plate`, `categorical` and `file` describe
/// a value; `numeric` is the scalar case the frozen `readouts[]` form covers.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ValueKind {
    Numeric,
    Table,
    Plate,
    Categorical,
    File,
}

/// Why a readout is missing. `None` (no reason) means "not recorded"; it is a
/// different state from every variant here and the two are never merged.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MissingReason {
    NotDetected,
    BelowLod,
    AboveUloq,
    Invalid,
    Other,
}

/// Unit rules for a schema. `canonical` is the recorded unit; it is never
/// inferred from the endpoint name.
#[derive(Debug, Clone, Default, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct UnitSpec {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canonical: Option<String>,
    pub required: bool,
}

/// Which explicit missing reasons a schema admits, and whether an unrecorded
/// value (`null`) is admissible at write time.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MissingSpec {
    #[serde(default)]
    pub allowed: Vec<MissingReason>,
    /// `false` means a readout with neither a value nor a reason is rejected:
    /// the writer must record the value or name the reason.
    pub null_allowed: bool,
}

/// Which provenance the schema admits. `derived` requires `raw`: a derived
/// readout always names the raw readouts it was computed from.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SourcePolicy {
    pub raw: bool,
    pub derived: bool,
}

/// Whether the recorded readout must be tied to a frozen timepoint or sample.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct LinkPolicy {
    pub timepoint_required: bool,
    pub sample_required: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TableSpec {
    pub columns: Vec<TableColumn>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TableColumn {
    pub key: String,
    pub label: String,
    /// Only `numeric` and `categorical` columns exist; a nested table is not a column.
    pub value_kind: ValueKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
}

/// Closed plate formats; `custom` has no fixed dimensions.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PlateFormat {
    #[serde(rename = "6")]
    Six,
    #[serde(rename = "12")]
    Twelve,
    #[serde(rename = "24")]
    TwentyFour,
    #[serde(rename = "48")]
    FortyEight,
    #[serde(rename = "96")]
    NinetySix,
    #[serde(rename = "384")]
    ThreeEightyFour,
    #[serde(rename = "custom")]
    Custom,
}

impl PlateFormat {
    /// Fixed `(rows, columns)` for a standard format; `None` for `custom`.
    pub fn dimensions(self) -> Option<(u32, u32)> {
        Some(match self {
            Self::Six => (2, 3),
            Self::Twelve => (3, 4),
            Self::TwentyFour => (4, 6),
            Self::FortyEight => (6, 8),
            Self::NinetySix => (8, 12),
            Self::ThreeEightyFour => (16, 24),
            Self::Custom => return None,
        })
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PlateSpec {
    pub format: PlateFormat,
    pub rows: u32,
    pub columns: u32,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CategoricalSpec {
    pub options: Vec<String>,
    /// When `false` a category outside `options` is rejected instead of stored.
    pub allow_other: bool,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct FileSpec {
    pub media_types: Vec<String>,
    pub require_checksum: bool,
}

/// One readout shape. Exactly the spec matching `value_kind` may be present.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutSchema {
    pub schema_id: String,
    pub schema_version: u32,
    pub endpoint: String,
    pub value_kind: ValueKind,
    pub unit: UnitSpec,
    pub missing: MissingSpec,
    pub source: SourcePolicy,
    pub link: LinkPolicy,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub table: Option<TableSpec>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub plate: Option<PlateSpec>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub categorical: Option<CategoricalSpec>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub file: Option<FileSpec>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WellRole {
    Unknown,
    Ntc,
    Standard,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PlateCell {
    pub coordinate: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub number: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub missing: Option<MissingReason>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sample_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target_gene: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub well_role: Option<WellRole>,
}

/// One table cell. `numeric` columns hold a number; `categorical` columns hold
/// a label such as a WB grade (`+`, `++`, `+++`). Untagged so v1 records that
/// wrote `[[1.98, null]]` keep deserializing and reserializing byte-identically.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(untagged)]
pub enum TableCell {
    Number(f64),
    Category(String),
}

/// The recorded value. The tag is `kind`, mirroring [`ReadingTarget`]; the
/// schema side names the same concept `value_kind`.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ReadoutValue {
    Numeric {
        #[serde(default, skip_serializing_if = "Option::is_none")]
        number: Option<f64>,
    },
    Table {
        columns: Vec<String>,
        /// Row-major cells; each cell's shape must match its column's
        /// `value_kind`. A table cell carries no missing reason; use numeric
        /// or plate readouts when the reason must be recorded per cell.
        rows: Vec<Vec<Option<TableCell>>>,
    },
    Plate {
        plate_id: String,
        rows: u32,
        columns: u32,
        cells: Vec<PlateCell>,
    },
    Categorical {
        #[serde(default, skip_serializing_if = "Option::is_none")]
        category: Option<String>,
    },
    File {
        asset_id: String,
        media_type: String,
        bytes: u64,
        #[serde(default, skip_serializing_if = "Option::is_none")]
        sha256: Option<String>,
    },
}

impl ReadoutValue {
    pub fn value_kind(&self) -> ValueKind {
        match self {
            Self::Numeric { .. } => ValueKind::Numeric,
            Self::Table { .. } => ValueKind::Table,
            Self::Plate { .. } => ValueKind::Plate,
            Self::Categorical { .. } => ValueKind::Categorical,
            Self::File { .. } => ValueKind::File,
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum SourceKind {
    Raw,
    Derived,
}

/// Provenance. A derived readout names its method and the raw readouts it was
/// computed from; a raw readout carries neither.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutSource {
    pub kind: SourceKind,
    /// `tool@version`-style method identifier; required when `kind = derived`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub method: Option<String>,
    #[serde(default)]
    pub derived_from: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default, deny_unknown_fields)]
pub struct ReadoutLink {
    /// Frozen design timepoint label, matched to the Run; never a free-form value.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timepoint: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sample_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub target: Option<ReadingTarget>,
}

/// One value recorded against a [`ReadoutSchema`].
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Readout {
    /// Assigned on commit; `None` is an uncommitted draft.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub readout_id: Option<String>,
    pub schema_id: String,
    pub schema_version: u32,
    pub value: ReadoutValue,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub missing: Option<MissingReason>,
    pub source: ReadoutSource,
    #[serde(default)]
    pub link: ReadoutLink,
}

/// How a schema projects onto the frozen `execution_event.create` `readouts[]`.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum LegacyReadoutForm {
    /// One legacy entry per readout.
    Single,
    /// One legacy entry per cell; the cell coordinate is appended to the id.
    PerCell,
    /// No lossless legacy form exists; the value lives in a table, asset or
    /// observation record and `readouts[]` stays empty.
    Unsupported,
}

impl ReadoutSchema {
    /// Legacy projection shape implied by `value_kind`. Only numeric scalars and
    /// numeric plate cells have a lossless `readouts[]` form.
    pub fn legacy_form(&self) -> LegacyReadoutForm {
        match self.value_kind {
            ValueKind::Numeric => LegacyReadoutForm::Single,
            ValueKind::Plate => LegacyReadoutForm::PerCell,
            ValueKind::Table | ValueKind::Categorical | ValueKind::File => {
                LegacyReadoutForm::Unsupported
            }
        }
    }
}

/// One entry of the frozen `readouts[]` array.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct LegacyReadout {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub readout_id: Option<String>,
    pub endpoint: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub value: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub qualifier: Option<MissingReason>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sample_id: Option<String>,
    pub result_kind: SourceKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub method: Option<String>,
    #[serde(default)]
    pub derived_from: Vec<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ReadoutIssueCode {
    // Schema shape.
    InvalidSchemaVersion,
    EmptyIdentifier,
    UnitRequiredWithoutCanonical,
    UnitNotApplicable,
    DuplicateMissingReason,
    KindSpecMissing,
    KindSpecUnexpected,
    InvalidTableColumnKind,
    DuplicateTableColumn,
    EmptyTableColumns,
    InvalidPlateShape,
    EmptyCategoricalOptions,
    DuplicateCategoricalOption,
    EmptyFileMediaTypes,
    DerivedRequiresRaw,
    // Readout value.
    SchemaIdMismatch,
    SchemaVersionMismatch,
    ValueKindMismatch,
    UnitMismatch,
    MissingUnit,
    NonFiniteNumber,
    ReasonWithValue,
    MissingWithoutReason,
    MissingReasonNotAllowed,
    TopLevelMissingNotApplicable,
    DuplicatePlateCoordinate,
    InvalidPlateMapping,
    TableColumnsMismatch,
    TableRowWidthMismatch,
    TableCellKindMismatch,
    CategoryNotAllowed,
    FileMediaTypeNotAllowed,
    FileChecksumRequired,
    // Provenance and association.
    DerivedWithoutMethod,
    DerivedWithoutSources,
    RawWithDerivation,
    RawNotAllowed,
    DerivedNotAllowed,
    TimepointMissing,
    SampleMissing,
    TargetNotApplicable,
    DerivedSourceUnknown,
    DerivedSourceNotRaw,
    // Legacy projection.
    LegacyFormUnsupported,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ReadoutIssue {
    pub code: ReadoutIssueCode,
    pub path: String,
    pub message: String,
}

fn issue(code: ReadoutIssueCode, path: &str, message: impl Into<String>) -> ReadoutIssue {
    ReadoutIssue {
        code,
        path: path.to_owned(),
        message: message.into(),
    }
}

fn blank(value: &str) -> bool {
    value.trim().is_empty()
}

/// Check a schema's own invariants. Call this before [`validate_readout`].
pub fn validate_schema(schema: &ReadoutSchema) -> Vec<ReadoutIssue> {
    let mut issues = Vec::new();

    if schema.schema_version != READOUT_SCHEMA_VERSION {
        issues.push(issue(
            ReadoutIssueCode::InvalidSchemaVersion,
            "schema_version",
            format!(
                "schema_version={}，当前契约只接受 {READOUT_SCHEMA_VERSION}",
                schema.schema_version
            ),
        ));
    }
    if blank(&schema.schema_id) {
        issues.push(issue(
            ReadoutIssueCode::EmptyIdentifier,
            "schema_id",
            "schema_id 不能为空",
        ));
    }
    if blank(&schema.endpoint) {
        issues.push(issue(
            ReadoutIssueCode::EmptyIdentifier,
            "endpoint",
            "endpoint 不能为空",
        ));
    }

    let canonical = schema.unit.canonical.as_deref().filter(|unit| !blank(unit));
    if schema.unit.required && canonical.is_none() {
        issues.push(issue(
            ReadoutIssueCode::UnitRequiredWithoutCanonical,
            "unit",
            "unit.required=true 但未声明 canonical 单位",
        ));
    }
    if matches!(schema.value_kind, ValueKind::File | ValueKind::Categorical)
        && (canonical.is_some() || schema.unit.required)
    {
        issues.push(issue(
            ReadoutIssueCode::UnitNotApplicable,
            "unit",
            "file/categorical 读数不携带标量单位",
        ));
    }

    let mut seen = Vec::new();
    for reason in &schema.missing.allowed {
        if seen.contains(reason) {
            issues.push(issue(
                ReadoutIssueCode::DuplicateMissingReason,
                "missing.allowed",
                format!("重复的缺失原因: {reason:?}"),
            ));
        } else {
            seen.push(*reason);
        }
    }

    check_kind_spec(
        schema.value_kind,
        ValueKind::Table,
        schema.table.is_some(),
        "table",
        &mut issues,
    );
    check_kind_spec(
        schema.value_kind,
        ValueKind::Plate,
        schema.plate.is_some(),
        "plate",
        &mut issues,
    );
    check_kind_spec(
        schema.value_kind,
        ValueKind::Categorical,
        schema.categorical.is_some(),
        "categorical",
        &mut issues,
    );
    check_kind_spec(
        schema.value_kind,
        ValueKind::File,
        schema.file.is_some(),
        "file",
        &mut issues,
    );

    if let Some(table) = &schema.table {
        if table.columns.is_empty() {
            issues.push(issue(
                ReadoutIssueCode::EmptyTableColumns,
                "table.columns",
                "表格读数必须声明至少一列",
            ));
        }
        let mut keys: Vec<&str> = Vec::new();
        for column in &table.columns {
            if keys.contains(&column.key.as_str()) {
                issues.push(issue(
                    ReadoutIssueCode::DuplicateTableColumn,
                    "table.columns",
                    format!("重复的列: {}", column.key),
                ));
            } else {
                keys.push(&column.key);
            }
            if !matches!(
                column.value_kind,
                ValueKind::Numeric | ValueKind::Categorical
            ) {
                issues.push(issue(
                    ReadoutIssueCode::InvalidTableColumnKind,
                    "table.columns",
                    format!(
                        "列 {} 的 value_kind 只能是 numeric 或 categorical",
                        column.key
                    ),
                ));
            }
        }
    }

    if let Some(plate) = &schema.plate {
        let dimensions = plate.format.dimensions();
        let mismatch = dimensions
            .is_some_and(|(rows, columns)| plate.rows != rows || plate.columns != columns);
        if plate.rows == 0 || plate.columns == 0 || mismatch {
            issues.push(issue(
                ReadoutIssueCode::InvalidPlateShape,
                "plate",
                format!(
                    "板型 {:?} 与声明的 {}×{} 不一致",
                    plate.format, plate.rows, plate.columns
                ),
            ));
        }
    }

    if let Some(categorical) = &schema.categorical {
        if categorical.options.iter().all(|option| blank(option)) {
            issues.push(issue(
                ReadoutIssueCode::EmptyCategoricalOptions,
                "categorical.options",
                "分类读数必须声明至少一个类别",
            ));
        }
        let mut seen_options: Vec<&str> = Vec::new();
        for option in &categorical.options {
            if seen_options.contains(&option.as_str()) {
                issues.push(issue(
                    ReadoutIssueCode::DuplicateCategoricalOption,
                    "categorical.options",
                    format!("重复的类别: {option}"),
                ));
            } else {
                seen_options.push(option);
            }
        }
    }

    if let Some(file) = &schema.file {
        if file.media_types.is_empty() {
            issues.push(issue(
                ReadoutIssueCode::EmptyFileMediaTypes,
                "file.media_types",
                "文件读数必须声明允许的 media type",
            ));
        }
    }

    if schema.source.derived && !schema.source.raw {
        issues.push(issue(
            ReadoutIssueCode::DerivedRequiresRaw,
            "source",
            "允许 derived 时必须同时允许 raw",
        ));
    }

    issues
}

fn check_kind_spec(
    kind: ValueKind,
    spec_kind: ValueKind,
    present: bool,
    field: &str,
    issues: &mut Vec<ReadoutIssue>,
) {
    let matching = kind == spec_kind;
    if matching && !present {
        issues.push(issue(
            ReadoutIssueCode::KindSpecMissing,
            field,
            format!("value_kind={kind:?} 必须声明 {field} 形状"),
        ));
    }
    if !matching && present {
        issues.push(issue(
            ReadoutIssueCode::KindSpecUnexpected,
            field,
            format!("value_kind={kind:?} 不接受 {field} 形状"),
        ));
    }
}

fn push_missing_pair(
    has_value: bool,
    reason: Option<MissingReason>,
    schema: &ReadoutSchema,
    path: &str,
    issues: &mut Vec<ReadoutIssue>,
) {
    match (has_value, reason) {
        (true, Some(reason)) => issues.push(issue(
            ReadoutIssueCode::ReasonWithValue,
            path,
            format!("已记录数值，不能同时标记缺失原因 {reason:?}"),
        )),
        (false, None) if !schema.missing.null_allowed => issues.push(issue(
            ReadoutIssueCode::MissingWithoutReason,
            path,
            "schema 不允许 null；必须记录数值或缺失原因",
        )),
        (false, Some(reason)) if !schema.missing.allowed.contains(&reason) => issues.push(issue(
            ReadoutIssueCode::MissingReasonNotAllowed,
            path,
            format!("缺失原因 {reason:?} 未在 missing.allowed 中声明"),
        )),
        _ => {}
    }
}

/// Check a readout against a schema. Schema-level problems are reported by
/// [`validate_schema`]; only readout/schema agreement is checked here.
pub fn validate_readout(schema: &ReadoutSchema, readout: &Readout) -> Vec<ReadoutIssue> {
    let mut issues = Vec::new();

    if readout.schema_id != schema.schema_id {
        issues.push(issue(
            ReadoutIssueCode::SchemaIdMismatch,
            "schema_id",
            format!(
                "读数引用 {}，但校验的 schema 是 {}",
                readout.schema_id, schema.schema_id
            ),
        ));
    }
    if readout.schema_version != schema.schema_version {
        issues.push(issue(
            ReadoutIssueCode::SchemaVersionMismatch,
            "schema_version",
            format!(
                "读数 schema_version={}，schema 为 {}",
                readout.schema_version, schema.schema_version
            ),
        ));
    }

    let kind = readout.value.value_kind();
    if kind != schema.value_kind {
        issues.push(issue(
            ReadoutIssueCode::ValueKindMismatch,
            "value.kind",
            format!(
                "读数 value_kind={kind:?}，schema 为 {:?}",
                schema.value_kind
            ),
        ));
    }

    match (&readout.unit, schema.unit.canonical.as_deref()) {
        (Some(unit), Some(canonical)) if unit != canonical => issues.push(issue(
            ReadoutIssueCode::UnitMismatch,
            "unit",
            format!("读数单位 {unit} 与 schema 单位 {canonical} 不一致"),
        )),
        (None, Some(canonical)) if schema.unit.required => issues.push(issue(
            ReadoutIssueCode::MissingUnit,
            "unit",
            format!("schema 要求单位 {canonical}"),
        )),
        _ => {}
    }

    match &readout.value {
        ReadoutValue::Numeric { number } => {
            if let Some(number) = number {
                if !number.is_finite() {
                    issues.push(issue(
                        ReadoutIssueCode::NonFiniteNumber,
                        "value.number",
                        "数值必须是有限数",
                    ));
                }
            }
            push_missing_pair(
                number.is_some(),
                readout.missing,
                schema,
                "value",
                &mut issues,
            );
        }
        ReadoutValue::Categorical { category } => {
            push_missing_pair(
                category.is_some(),
                readout.missing,
                schema,
                "value",
                &mut issues,
            );
            if let (Some(category), Some(spec)) = (category, &schema.categorical) {
                if !spec.allow_other && !spec.options.iter().any(|option| option == category) {
                    issues.push(issue(
                        ReadoutIssueCode::CategoryNotAllowed,
                        "value.category",
                        format!("类别 {category} 未在 categorical.options 中声明"),
                    ));
                }
            }
        }
        ReadoutValue::Plate {
            rows,
            columns,
            cells,
            ..
        } => {
            if readout.missing.is_some() {
                issues.push(issue(
                    ReadoutIssueCode::TopLevelMissingNotApplicable,
                    "missing",
                    "plate 读数按孔记录缺失原因，不使用顶层 missing",
                ));
            }
            if let Some(spec) = &schema.plate {
                if spec.rows != *rows || spec.columns != *columns {
                    issues.push(issue(
                        ReadoutIssueCode::InvalidPlateShape,
                        "value",
                        format!(
                            "读数板型 {rows}×{columns} 与 schema {}×{} 不一致",
                            spec.rows, spec.columns
                        ),
                    ));
                }
            }
            let mut coordinates: Vec<&str> = Vec::new();
            for cell in cells {
                if coordinates.contains(&cell.coordinate.as_str()) {
                    issues.push(issue(
                        ReadoutIssueCode::DuplicatePlateCoordinate,
                        "value.cells",
                        format!("重复孔位: {}", cell.coordinate),
                    ));
                } else {
                    coordinates.push(&cell.coordinate);
                }
                if let Some(number) = cell.number {
                    if !number.is_finite() {
                        issues.push(issue(
                            ReadoutIssueCode::NonFiniteNumber,
                            "value.cells",
                            format!("孔位 {} 的数值不是有限数", cell.coordinate),
                        ));
                    }
                }
                if cell.sample_id.as_deref().is_some_and(blank)
                    || cell.target_gene.as_deref().is_some_and(blank)
                {
                    issues.push(issue(
                        ReadoutIssueCode::InvalidPlateMapping,
                        "value.cells",
                        format!("孔位 {} 的样本和目标基因标识不得为空", cell.coordinate),
                    ));
                }
                push_missing_pair(
                    cell.number.is_some(),
                    cell.missing,
                    schema,
                    "value.cells",
                    &mut issues,
                );
            }
        }
        ReadoutValue::Table { columns, rows } => {
            if readout.missing.is_some() {
                issues.push(issue(
                    ReadoutIssueCode::TopLevelMissingNotApplicable,
                    "missing",
                    "table 读数不使用顶层 missing",
                ));
            }
            if let Some(spec) = &schema.table {
                let expected: Vec<&str> = spec
                    .columns
                    .iter()
                    .map(|column| column.key.as_str())
                    .collect();
                let actual: Vec<&str> = columns.iter().map(String::as_str).collect();
                if expected != actual {
                    issues.push(issue(
                        ReadoutIssueCode::TableColumnsMismatch,
                        "value.columns",
                        format!("读数列 {actual:?} 与 schema 列 {expected:?} 不一致"),
                    ));
                }
            }
            for row in rows {
                if row.len() != columns.len() {
                    issues.push(issue(
                        ReadoutIssueCode::TableRowWidthMismatch,
                        "value.rows",
                        format!("行宽 {} 与列数 {} 不一致", row.len(), columns.len()),
                    ));
                }
                if !schema.missing.null_allowed && row.iter().any(Option::is_none) {
                    issues.push(issue(
                        ReadoutIssueCode::MissingWithoutReason,
                        "value.rows",
                        "schema 不允许 null，但表格含未记录单元格",
                    ));
                }
                let Some(spec) = &schema.table else {
                    continue;
                };
                for (index, cell) in row.iter().enumerate() {
                    let (Some(cell), Some(column)) = (cell, spec.columns.get(index)) else {
                        continue;
                    };
                    match (column.value_kind, cell) {
                        (ValueKind::Numeric, TableCell::Number(number)) => {
                            if !number.is_finite() {
                                issues.push(issue(
                                    ReadoutIssueCode::NonFiniteNumber,
                                    "value.rows",
                                    format!("列 {} 的数值必须是有限数", column.key),
                                ));
                            }
                        }
                        (ValueKind::Categorical, TableCell::Category(category))
                            if !blank(category) => {}
                        (_, cell) => issues.push(issue(
                            ReadoutIssueCode::TableCellKindMismatch,
                            "value.rows",
                            format!(
                                "列 {} 的 value_kind 为 {:?}，不接受单元格 {cell:?}",
                                column.key, column.value_kind
                            ),
                        )),
                    }
                }
            }
        }
        ReadoutValue::File {
            media_type, sha256, ..
        } => {
            if readout.missing.is_some() {
                issues.push(issue(
                    ReadoutIssueCode::TopLevelMissingNotApplicable,
                    "missing",
                    "file 读数不使用顶层 missing；缺失文件应以未记录表达",
                ));
            }
            if let Some(spec) = &schema.file {
                if !spec.media_types.iter().any(|allowed| allowed == media_type) {
                    issues.push(issue(
                        ReadoutIssueCode::FileMediaTypeNotAllowed,
                        "value.media_type",
                        format!("media type {media_type} 未在 file.media_types 中声明"),
                    ));
                }
                if spec.require_checksum && sha256.is_none() {
                    issues.push(issue(
                        ReadoutIssueCode::FileChecksumRequired,
                        "value.sha256",
                        "schema 要求文件校验和",
                    ));
                }
            }
        }
    }

    match readout.source.kind {
        SourceKind::Derived => {
            let has_method = readout
                .source
                .method
                .as_deref()
                .is_some_and(|method| !blank(method));
            if !has_method {
                issues.push(issue(
                    ReadoutIssueCode::DerivedWithoutMethod,
                    "source.method",
                    "派生读数必须记录计算方法",
                ));
            }
            if readout.source.derived_from.is_empty() {
                issues.push(issue(
                    ReadoutIssueCode::DerivedWithoutSources,
                    "source.derived_from",
                    "派生读数必须引用原始读数",
                ));
            }
            if !schema.source.derived {
                issues.push(issue(
                    ReadoutIssueCode::DerivedNotAllowed,
                    "source.kind",
                    "schema 未允许派生读数",
                ));
            }
        }
        SourceKind::Raw => {
            if readout.source.method.is_some() || !readout.source.derived_from.is_empty() {
                issues.push(issue(
                    ReadoutIssueCode::RawWithDerivation,
                    "source",
                    "原始读数不能带 method 或 derived_from",
                ));
            }
            if !schema.source.raw {
                issues.push(issue(
                    ReadoutIssueCode::RawNotAllowed,
                    "source.kind",
                    "schema 未允许原始读数",
                ));
            }
        }
    }

    if schema.link.timepoint_required && readout.link.timepoint.is_none() {
        issues.push(issue(
            ReadoutIssueCode::TimepointMissing,
            "link.timepoint",
            "schema 要求记录时间点",
        ));
    }
    if schema.link.sample_required && readout.link.sample_id.is_none() {
        issues.push(issue(
            ReadoutIssueCode::SampleMissing,
            "link.sample_id",
            "schema 要求记录样本",
        ));
    }
    if kind == ValueKind::Plate && readout.link.target.is_some() {
        issues.push(issue(
            ReadoutIssueCode::TargetNotApplicable,
            "link.target",
            "plate 读数由孔位坐标承载目标，不再带单一 target",
        ));
    }

    issues
}

/// Resolve `derived_from` against ids that already exist in the same Run. A
/// derived readout may only cite raw readouts.
pub fn check_derived_sources(
    readout: &Readout,
    raw_readout_ids: &[&str],
    derived_readout_ids: &[&str],
) -> Vec<ReadoutIssue> {
    let mut issues = Vec::new();
    for source in &readout.source.derived_from {
        if derived_readout_ids.contains(&source.as_str()) {
            issues.push(issue(
                ReadoutIssueCode::DerivedSourceNotRaw,
                "source.derived_from",
                format!("{source} 是派生读数；派生只能引用原始读数"),
            ));
        } else if !raw_readout_ids.contains(&source.as_str()) {
            issues.push(issue(
                ReadoutIssueCode::DerivedSourceUnknown,
                "source.derived_from",
                format!("{source} 不是本次 Run 已存在的原始读数"),
            ));
        }
    }
    issues
}

/// Project a readout onto the frozen `execution_event.create` `readouts[]`
/// entries. Pure: nothing is written, and a plate yields one entry per cell.
pub fn to_legacy_readouts(
    schema: &ReadoutSchema,
    readout: &Readout,
) -> Result<Vec<LegacyReadout>, ReadoutIssue> {
    let entry = |readout_id: Option<String>, value: Option<f64>, qualifier| LegacyReadout {
        readout_id,
        endpoint: schema.endpoint.clone(),
        value,
        qualifier,
        unit: readout.unit.clone(),
        sample_id: readout.link.sample_id.clone(),
        result_kind: readout.source.kind,
        method: readout.source.method.clone(),
        derived_from: readout.source.derived_from.clone(),
    };

    match &readout.value {
        ReadoutValue::Numeric { number } => Ok(vec![entry(
            readout.readout_id.clone(),
            *number,
            readout.missing,
        )]),
        ReadoutValue::Plate { cells, .. } => Ok(cells
            .iter()
            .map(|cell| {
                let id = readout
                    .readout_id
                    .as_ref()
                    .map(|base| format!("{base}-{}", cell.coordinate));
                entry(id, cell.number, cell.missing)
            })
            .collect()),
        ReadoutValue::Table { .. }
        | ReadoutValue::Categorical { .. }
        | ReadoutValue::File { .. } => Err(issue(
            ReadoutIssueCode::LegacyFormUnsupported,
            "value.kind",
            format!(
                "{:?} 读数没有无损的 readouts[] 形式；保持 readouts[] 为空",
                readout.value.value_kind()
            ),
        )),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::{json, Value};

    fn schema_from(value: Value) -> ReadoutSchema {
        serde_json::from_value(value).expect("schema 示例必须可反序列化")
    }

    fn readout_from(value: Value) -> Readout {
        serde_json::from_value(value).expect("readout 示例必须可反序列化")
    }

    fn codes(issues: &[ReadoutIssue]) -> Vec<ReadoutIssueCode> {
        issues.iter().map(|issue| issue.code).collect()
    }

    fn cck8_schema() -> ReadoutSchema {
        schema_from(json!({
            "schema_id": "cck8_od450",
            "schema_version": 1,
            "endpoint": "od450",
            "value_kind": "plate",
            "unit": {"canonical": "AU", "required": true},
            "missing": {"allowed": ["not_detected", "above_uloq"], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": true, "sample_required": false},
            "plate": {"format": "96", "rows": 8, "columns": 12}
        }))
    }

    fn cck8_readout() -> Readout {
        readout_from(json!({
            "readout_id": "ro-cck8-1",
            "schema_id": "cck8_od450",
            "schema_version": 1,
            "value": {
                "kind": "plate",
                "plate_id": "plate-1",
                "rows": 8,
                "columns": 12,
                "cells": [
                    {"coordinate": "A1", "number": 1.234},
                    {"coordinate": "A2", "missing": "above_uloq"},
                    {"coordinate": "A3"}
                ]
            },
            "unit": "AU",
            "source": {"kind": "raw", "derived_from": []},
            "link": {"timepoint": "处理后 24 h"}
        }))
    }

    fn numeric_schema(unit: &str, required: bool) -> ReadoutSchema {
        schema_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "endpoint": "value",
            "value_kind": "numeric",
            "unit": {"canonical": unit, "required": required},
            "missing": {"allowed": ["not_detected", "below_lod"], "null_allowed": false},
            "source": {"raw": true, "derived": true},
            "link": {"timepoint_required": false, "sample_required": false}
        }))
    }

    #[test]
    fn schema_round_trips_all_five_value_kinds() {
        let values = [
            (
                numeric_schema("ng/µL", true),
                json!({"kind": "numeric", "number": 214.5}),
            ),
            (
                schema_from(json!({
                    "schema_id": "plate_counts",
                    "schema_version": 1,
                    "endpoint": "cells",
                    "value_kind": "plate",
                    "unit": {"canonical": "count", "required": true},
                    "missing": {"allowed": ["not_detected"], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "plate": {"format": "6", "rows": 2, "columns": 3}
                })),
                json!({
                    "kind": "plate",
                    "plate_id": "p6",
                    "rows": 2,
                    "columns": 3,
                    "cells": [{"coordinate": "A1", "number": 42.0}]
                }),
            ),
            (
                schema_from(json!({
                    "schema_id": "qc_table",
                    "schema_version": 1,
                    "endpoint": "qc",
                    "value_kind": "table",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "table": {"columns": [
                        {"key": "a260", "label": "A260", "value_kind": "numeric", "unit": "AU"},
                        {"key": "pass", "label": "判定", "value_kind": "categorical"}
                    ]}
                })),
                json!({
                    "kind": "table",
                    "columns": ["a260", "pass"],
                    "rows": [[1.98, null]]
                }),
            ),
            (
                schema_from(json!({
                    "schema_id": "gram_stain",
                    "schema_version": 1,
                    "endpoint": "gram",
                    "value_kind": "categorical",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": ["not_detected"], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": true},
                    "categorical": {"options": ["positive", "negative"], "allow_other": false}
                })),
                json!({"kind": "categorical", "category": "positive"}),
            ),
            (
                schema_from(json!({
                    "schema_id": "raw_trace",
                    "schema_version": 1,
                    "endpoint": "trace",
                    "value_kind": "file",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "file": {"media_types": ["text/csv"], "require_checksum": true}
                })),
                json!({
                    "kind": "file",
                    "asset_id": "asset-1",
                    "media_type": "text/csv",
                    "bytes": 2048,
                    "sha256": "abc"
                }),
            ),
        ];

        for (schema, value) in values {
            assert!(validate_schema(&schema).is_empty(), "{schema:?}");
            let readout = readout_from(json!({
                "schema_id": schema.schema_id,
                "schema_version": 1,
                "value": value,
                "source": {"kind": "raw", "derived_from": []},
                "link": {}
            }));
            assert_eq!(readout.value.value_kind(), schema.value_kind);
            let encoded = serde_json::to_value(&readout).unwrap();
            let decoded: Readout = serde_json::from_value(encoded).unwrap();
            assert_eq!(decoded, readout);
        }
    }

    #[test]
    fn cck8_plate_example_is_valid_and_maps_per_cell() {
        let schema = cck8_schema();
        let readout = cck8_readout();
        assert!(validate_schema(&schema).is_empty());
        assert_eq!(validate_readout(&schema, &readout), Vec::new());
        assert_eq!(schema.legacy_form(), LegacyReadoutForm::PerCell);

        let legacy = to_legacy_readouts(&schema, &readout).unwrap();
        assert_eq!(legacy.len(), 3);
        assert_eq!(legacy[0].readout_id.as_deref(), Some("ro-cck8-1-A1"));
        assert_eq!(legacy[0].value, Some(1.234));
        assert_eq!(legacy[1].qualifier, Some(MissingReason::AboveUloq));
        assert_eq!(legacy[2].value, None);
        assert_eq!(legacy[2].qualifier, None);
        assert_eq!(legacy[0].endpoint, "od450");
        assert_eq!(legacy[0].result_kind, SourceKind::Raw);
    }

    #[test]
    fn rna_purity_is_marked_derived_from_raw_readouts() {
        let concentration = schema_from(json!({
            "schema_id": "rna_concentration",
            "schema_version": 1,
            "endpoint": "concentration",
            "value_kind": "numeric",
            "unit": {"canonical": "ng/µL", "required": true},
            "missing": {"allowed": ["below_lod"], "null_allowed": false},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": true, "sample_required": true}
        }));
        let purity = schema_from(json!({
            "schema_id": "rna_purity",
            "schema_version": 1,
            "endpoint": "a260_a280",
            "value_kind": "numeric",
            "unit": {"canonical": "ratio", "required": true},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": true},
            "link": {"timepoint_required": true, "sample_required": true}
        }));
        let raw = readout_from(json!({
            "readout_id": "ro-rna-conc-1",
            "schema_id": "rna_concentration",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 214.5},
            "unit": "ng/µL",
            "source": {"kind": "raw", "derived_from": []},
            "link": {"timepoint": "裂解后立即", "sample_id": "S-1"}
        }));
        let ratio = readout_from(json!({
            "readout_id": "ro-rna-ratio-1",
            "schema_id": "rna_purity",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 2.06},
            "unit": "ratio",
            "source": {
                "kind": "derived",
                "method": "a260_a280_ratio@1",
                "derived_from": ["ro-rna-a260-1", "ro-rna-a280-1"]
            },
            "link": {"timepoint": "裂解后立即", "sample_id": "S-1"}
        }));

        assert!(validate_schema(&concentration).is_empty());
        assert!(validate_schema(&purity).is_empty());
        assert_eq!(validate_readout(&concentration, &raw), Vec::new());
        assert_eq!(validate_readout(&purity, &ratio), Vec::new());
        assert_eq!(purity.legacy_form(), LegacyReadoutForm::Single);

        let ok = check_derived_sources(
            &ratio,
            &["ro-rna-a260-1", "ro-rna-a280-1", "ro-rna-conc-1"],
            &["ro-rna-ratio-1"],
        );
        assert!(ok.is_empty(), "{ok:?}");

        let legacy = to_legacy_readouts(&purity, &ratio).unwrap();
        assert_eq!(legacy.len(), 1);
        assert_eq!(legacy[0].result_kind, SourceKind::Derived);
        assert_eq!(legacy[0].method.as_deref(), Some("a260_a280_ratio@1"));
        assert_eq!(legacy[0].derived_from.len(), 2);
    }

    #[test]
    fn cfu_count_keeps_unit_and_well_target() {
        let schema = schema_from(json!({
            "schema_id": "cfu_count",
            "schema_version": 1,
            "endpoint": "cfu",
            "value_kind": "numeric",
            "unit": {"canonical": "CFU", "required": true},
            "missing": {"allowed": ["not_detected", "above_uloq"], "null_allowed": false},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": true, "sample_required": true}
        }));
        let readout = readout_from(json!({
            "readout_id": "ro-cfu-1",
            "schema_id": "cfu_count",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 143.0},
            "unit": "CFU",
            "source": {"kind": "raw", "derived_from": []},
            "link": {
                "timepoint": "处理后 48 h",
                "sample_id": "S-3",
                "target": {"kind": "well", "plate_id": "plate-cfu", "coordinate": "B2"}
            }
        }));

        assert!(validate_schema(&schema).is_empty());
        assert_eq!(validate_readout(&schema, &readout), Vec::new());
        let legacy = to_legacy_readouts(&schema, &readout).unwrap();
        assert_eq!(legacy[0].unit.as_deref(), Some("CFU"));
        assert_eq!(legacy[0].sample_id.as_deref(), Some("S-3"));
        assert_eq!(legacy[0].value, Some(143.0));
    }

    #[test]
    fn null_and_explicit_missing_reason_stay_distinct() {
        let schema = numeric_schema("CFU", true);

        let unrecorded = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric"},
            "unit": "CFU",
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &unrecorded)),
            vec![ReadoutIssueCode::MissingWithoutReason]
        );

        let explicit = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric"},
            "unit": "CFU",
            "missing": "not_detected",
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert!(validate_readout(&schema, &explicit).is_empty());

        let contradictory = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 12.0},
            "unit": "CFU",
            "missing": "below_lod",
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &contradictory)),
            vec![ReadoutIssueCode::ReasonWithValue]
        );

        let undeclared = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric"},
            "unit": "CFU",
            "missing": "above_uloq",
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &undeclared)),
            vec![ReadoutIssueCode::MissingReasonNotAllowed]
        );

        assert_eq!(
            serde_json::to_value(MissingReason::AboveUloq).unwrap(),
            json!("above_uloq")
        );
        let typo = serde_json::from_value::<MissingReason>(json!("above_ulog"));
        assert!(typo.is_err(), "above_ulog 不是契约值");
    }

    #[test]
    fn derived_readout_requires_method_and_sources() {
        let schema = numeric_schema("ratio", true);
        let missing_both = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 2.06},
            "unit": "ratio",
            "source": {"kind": "derived", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &missing_both)),
            vec![
                ReadoutIssueCode::DerivedWithoutMethod,
                ReadoutIssueCode::DerivedWithoutSources
            ]
        );

        let references_derived = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 2.06},
            "unit": "ratio",
            "source": {
                "kind": "derived",
                "method": "ratio@1",
                "derived_from": ["ro-ratio-0", "ro-unknown"]
            }
        }));
        assert_eq!(
            codes(&check_derived_sources(
                &references_derived,
                &["ro-a260-1"],
                &["ro-ratio-0"]
            )),
            vec![
                ReadoutIssueCode::DerivedSourceNotRaw,
                ReadoutIssueCode::DerivedSourceUnknown
            ]
        );
    }

    #[test]
    fn schema_requires_the_matching_kind_spec() {
        let without_plate = schema_from(json!({
            "schema_id": "cck8_od450",
            "schema_version": 1,
            "endpoint": "od450",
            "value_kind": "plate",
            "unit": {"canonical": "AU", "required": true},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false}
        }));
        assert_eq!(
            codes(&validate_schema(&without_plate)),
            vec![ReadoutIssueCode::KindSpecMissing]
        );

        let mut with_table = without_plate.clone();
        with_table.table = Some(TableSpec {
            columns: vec![TableColumn {
                key: "od450".into(),
                label: "OD450".into(),
                value_kind: ValueKind::Numeric,
                unit: Some("AU".into()),
            }],
        });
        assert_eq!(
            codes(&validate_schema(&with_table)),
            vec![
                ReadoutIssueCode::KindSpecUnexpected,
                ReadoutIssueCode::KindSpecMissing
            ]
        );

        let mut derived_only = numeric_schema("CFU", true);
        derived_only.source = SourcePolicy {
            raw: false,
            derived: true,
        };
        assert_eq!(
            codes(&validate_schema(&derived_only)),
            vec![ReadoutIssueCode::DerivedRequiresRaw]
        );
    }

    #[test]
    fn standard_plate_format_fixes_its_dimensions() {
        let mut schema = cck8_schema();
        schema.plate = Some(PlateSpec {
            format: PlateFormat::NinetySix,
            rows: 6,
            columns: 8,
        });
        assert_eq!(
            codes(&validate_schema(&schema)),
            vec![ReadoutIssueCode::InvalidPlateShape]
        );

        let readout = cck8_readout();
        assert_eq!(
            codes(&validate_readout(&schema, &readout)),
            vec![ReadoutIssueCode::InvalidPlateShape]
        );

        let custom = PlateSpec {
            format: PlateFormat::Custom,
            rows: 3,
            columns: 5,
        };
        assert_eq!(custom.format.dimensions(), None);
    }

    #[test]
    fn table_and_file_and_categorical_have_no_legacy_form() {
        let kinds = [
            (
                json!({
                    "schema_id": "qc_table",
                    "schema_version": 1,
                    "endpoint": "qc",
                    "value_kind": "table",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "table": {"columns": [
                        {"key": "a260", "label": "A260", "value_kind": "numeric"}
                    ]}
                }),
                json!({"kind": "table", "columns": ["a260"], "rows": [[1.98]]}),
            ),
            (
                json!({
                    "schema_id": "gram_stain",
                    "schema_version": 1,
                    "endpoint": "gram",
                    "value_kind": "categorical",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "categorical": {"options": ["positive", "negative"], "allow_other": false}
                }),
                json!({"kind": "categorical", "category": "positive"}),
            ),
            (
                json!({
                    "schema_id": "raw_trace",
                    "schema_version": 1,
                    "endpoint": "trace",
                    "value_kind": "file",
                    "unit": {"canonical": null, "required": false},
                    "missing": {"allowed": [], "null_allowed": true},
                    "source": {"raw": true, "derived": false},
                    "link": {"timepoint_required": false, "sample_required": false},
                    "file": {"media_types": ["text/csv"], "require_checksum": false}
                }),
                json!({
                    "kind": "file",
                    "asset_id": "asset-1",
                    "media_type": "text/csv",
                    "bytes": 10
                }),
            ),
        ];

        for (schema, value) in kinds {
            let schema = schema_from(schema);
            assert!(validate_schema(&schema).is_empty(), "{schema:?}");
            assert_eq!(schema.legacy_form(), LegacyReadoutForm::Unsupported);
            let readout = readout_from(json!({
                "schema_id": schema.schema_id,
                "schema_version": 1,
                "value": value,
                "source": {"kind": "raw", "derived_from": []}
            }));
            assert!(validate_readout(&schema, &readout).is_empty());
            let error = to_legacy_readouts(&schema, &readout).unwrap_err();
            assert_eq!(error.code, ReadoutIssueCode::LegacyFormUnsupported);
        }
    }

    #[test]
    fn categorical_and_file_rules_are_enforced() {
        let categorical = schema_from(json!({
            "schema_id": "gram_stain",
            "schema_version": 1,
            "endpoint": "gram",
            "value_kind": "categorical",
            "unit": {"canonical": null, "required": false},
            "missing": {"allowed": [], "null_allowed": false},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false},
            "categorical": {"options": ["positive", "negative"], "allow_other": false}
        }));
        let unknown = readout_from(json!({
            "schema_id": "gram_stain",
            "schema_version": 1,
            "value": {"kind": "categorical", "category": "variable"},
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&categorical, &unknown)),
            vec![ReadoutIssueCode::CategoryNotAllowed]
        );

        let file = schema_from(json!({
            "schema_id": "raw_trace",
            "schema_version": 1,
            "endpoint": "trace",
            "value_kind": "file",
            "unit": {"canonical": null, "required": false},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false},
            "file": {"media_types": ["text/csv"], "require_checksum": true}
        }));
        let bad = readout_from(json!({
            "schema_id": "raw_trace",
            "schema_version": 1,
            "value": {
                "kind": "file",
                "asset_id": "asset-1",
                "media_type": "image/tiff",
                "bytes": 10
            },
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&file, &bad)),
            vec![
                ReadoutIssueCode::FileMediaTypeNotAllowed,
                ReadoutIssueCode::FileChecksumRequired
            ]
        );
    }

    #[test]
    fn required_unit_timepoint_and_sample_are_reported() {
        let mut schema = numeric_schema("ng/µL", true);
        schema.link = LinkPolicy {
            timepoint_required: true,
            sample_required: true,
        };
        let readout = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 214.5},
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &readout)),
            vec![
                ReadoutIssueCode::MissingUnit,
                ReadoutIssueCode::TimepointMissing,
                ReadoutIssueCode::SampleMissing
            ]
        );

        let wrong_unit = readout_from(json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 214.5},
            "unit": "µg/mL",
            "source": {"kind": "raw", "derived_from": []},
            "link": {"timepoint": "裂解后立即", "sample_id": "S-1"}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &wrong_unit)),
            vec![ReadoutIssueCode::UnitMismatch]
        );
    }

    #[test]
    fn closed_structs_reject_unknown_fields() {
        let unknown_schema_field = json!({
            "schema_id": "cck8_od450",
            "schema_version": 1,
            "endpoint": "od450",
            "value_kind": "plate",
            "unit": {"canonical": "AU", "required": true},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false},
            "plate": {"format": "96", "rows": 8, "columns": 12},
            "renderer": "plate-grid"
        });
        assert!(serde_json::from_value::<ReadoutSchema>(unknown_schema_field).is_err());

        let unknown_readout_field = json!({
            "schema_id": "numeric",
            "schema_version": 1,
            "value": {"kind": "numeric", "number": 1.0},
            "source": {"kind": "raw", "derived_from": []},
            "confidence": 0.9
        });
        assert!(serde_json::from_value::<Readout>(unknown_readout_field).is_err());
    }

    #[test]
    fn plate_readouts_preserve_sample_gene_well_role_mapping() {
        let mut readout = cck8_readout();
        if let ReadoutValue::Plate { cells, .. } = &mut readout.value {
            cells[0].sample_id = Some("sample-1".into());
            cells[0].target_gene = Some("Gapdh".into());
            cells[0].well_role = Some(WellRole::Unknown);
            cells[1].well_role = Some(WellRole::Ntc);
            cells[2].well_role = Some(WellRole::Standard);
        } else {
            unreachable!();
        }
        let encoded = serde_json::to_value(&readout).unwrap();
        let decoded: Readout = serde_json::from_value(encoded).unwrap();
        let ReadoutValue::Plate { cells, .. } = decoded.value else {
            unreachable!();
        };
        assert_eq!(cells[0].sample_id.as_deref(), Some("sample-1"));
        assert_eq!(cells[0].target_gene.as_deref(), Some("Gapdh"));
        assert_eq!(cells[0].well_role, Some(WellRole::Unknown));
        assert_eq!(cells[1].well_role, Some(WellRole::Ntc));
        assert_eq!(cells[2].well_role, Some(WellRole::Standard));
        assert!(validate_readout(&cck8_schema(), &readout).is_empty());
    }

    fn wb_band_schema() -> ReadoutSchema {
        schema_from(json!({
            "schema_id": "wb_band",
            "schema_version": 1,
            "endpoint": "band",
            "value_kind": "table",
            "unit": {"canonical": null, "required": false},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false},
            "table": {"columns": [
                {"key": "target", "label": "目标蛋白", "value_kind": "categorical"},
                {"key": "grade", "label": "条带等级", "value_kind": "categorical"},
                {"key": "normalized", "label": "归一化强度", "value_kind": "numeric"}
            ]}
        }))
    }

    #[test]
    fn table_cells_carry_categories_alongside_numbers() {
        let schema = wb_band_schema();
        let readout = readout_from(json!({
            "readout_id": "ro-wb-1",
            "schema_id": "wb_band",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["target", "grade", "normalized"],
                "rows": [["GAPDH", "+++", 1.0], ["ACTB", "++", null], [null, null, 0.42]]
            },
            "source": {"kind": "raw", "derived_from": []}
        }));

        assert!(validate_schema(&schema).is_empty());
        assert_eq!(validate_readout(&schema, &readout), Vec::new());
        assert_eq!(schema.legacy_form(), LegacyReadoutForm::Unsupported);

        let encoded = serde_json::to_value(&readout).unwrap();
        assert_eq!(
            encoded["value"]["rows"][0],
            json!(["GAPDH", "+++", 1.0])
        );
        let decoded: Readout = serde_json::from_value(encoded).unwrap();
        assert_eq!(decoded, readout);
    }

    #[test]
    fn table_cell_kind_must_match_its_column() {
        let schema = wb_band_schema();
        let category_in_numeric = readout_from(json!({
            "schema_id": "wb_band",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["target", "grade", "normalized"],
                "rows": [["GAPDH", "+++", "+++"]]
            },
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &category_in_numeric)),
            vec![ReadoutIssueCode::TableCellKindMismatch]
        );

        let number_in_categorical = readout_from(json!({
            "schema_id": "wb_band",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["target", "grade", "normalized"],
                "rows": [["GAPDH", 3.0, 1.0]]
            },
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &number_in_categorical)),
            vec![ReadoutIssueCode::TableCellKindMismatch]
        );

        let blank_category = readout_from(json!({
            "schema_id": "wb_band",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["target", "grade", "normalized"],
                "rows": [["GAPDH", " ", 1.0]]
            },
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &blank_category)),
            vec![ReadoutIssueCode::TableCellKindMismatch]
        );

        let mut non_finite = readout_from(json!({
            "schema_id": "wb_band",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["target", "grade", "normalized"],
                "rows": [["GAPDH", "+++", 1.0]]
            },
            "source": {"kind": "raw", "derived_from": []}
        }));
        let ReadoutValue::Table { rows, .. } = &mut non_finite.value else {
            unreachable!();
        };
        rows[0][2] = Some(TableCell::Number(f64::INFINITY));
        assert_eq!(
            codes(&validate_readout(&schema, &non_finite)),
            vec![ReadoutIssueCode::NonFiniteNumber]
        );
    }

    #[test]
    fn categorical_kind_accepts_declared_wb_grades() {
        let schema = schema_from(json!({
            "schema_id": "wb_grade",
            "schema_version": 1,
            "endpoint": "band_grade",
            "value_kind": "categorical",
            "unit": {"canonical": null, "required": false},
            "missing": {"allowed": ["not_detected"], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false},
            "categorical": {"options": ["-", "+", "++", "+++"], "allow_other": false}
        }));
        let strong = readout_from(json!({
            "schema_id": "wb_grade",
            "schema_version": 1,
            "value": {"kind": "categorical", "category": "+++"},
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert!(validate_schema(&schema).is_empty());
        assert_eq!(validate_readout(&schema, &strong), Vec::new());

        let out_of_scale = readout_from(json!({
            "schema_id": "wb_grade",
            "schema_version": 1,
            "value": {"kind": "categorical", "category": "++++"},
            "source": {"kind": "raw", "derived_from": []}
        }));
        assert_eq!(
            codes(&validate_readout(&schema, &out_of_scale)),
            vec![ReadoutIssueCode::CategoryNotAllowed]
        );
    }

    #[test]
    fn numeric_table_rows_from_v1_deserialize_and_reserialize_unchanged() {
        let encoded = json!({
            "kind": "table",
            "columns": ["a260", "pass"],
            "rows": [[1.98, null]]
        });
        let value: ReadoutValue = serde_json::from_value(encoded.clone()).unwrap();
        assert_eq!(
            value,
            ReadoutValue::Table {
                columns: vec!["a260".to_owned(), "pass".to_owned()],
                rows: vec![vec![Some(TableCell::Number(1.98)), None]],
            }
        );
        assert_eq!(serde_json::to_value(&value).unwrap(), encoded);
    }

    #[test]
    fn standard_curve_point_table_round_trips() {
        let schema = schema_from(json!({
            "schema_id": "bca_standard_curve",
            "schema_version": 1,
            "endpoint": "a562",
            "value_kind": "table",
            "unit": {"canonical": "AU", "required": false},
            "missing": {"allowed": [], "null_allowed": true},
            "source": {"raw": true, "derived": false},
            "link": {"timepoint_required": false, "sample_required": false},
            "table": {"columns": [
                {"key": "concentration", "label": "标准浓度", "value_kind": "numeric", "unit": "µg/mL"},
                {"key": "a562", "label": "A562", "value_kind": "numeric", "unit": "AU"}
            ]}
        }));
        let readout = readout_from(json!({
            "readout_id": "ro-bca-std-1",
            "schema_id": "bca_standard_curve",
            "schema_version": 1,
            "value": {
                "kind": "table",
                "columns": ["concentration", "a562"],
                "rows": [[0.0, 0.0], [25.0, 0.121], [125.0, 0.587], [250.0, 1.148], [500.0, 2.201]]
            },
            "source": {"kind": "raw", "derived_from": []}
        }));

        assert!(validate_schema(&schema).is_empty());
        assert_eq!(validate_readout(&schema, &readout), Vec::new());
        assert_eq!(schema.legacy_form(), LegacyReadoutForm::Unsupported);
        let encoded = serde_json::to_value(&readout).unwrap();
        assert_eq!(serde_json::from_value::<Readout>(encoded).unwrap(), readout);
    }
}
