//! Declared composite field values for AsterDoc module inputs.
//!
//! Only the two list port types that already exist in the module manifest are
//! modeled: `string[]` and `object-list`. Scalar ports keep their existing
//! validators; this module neither adds field types nor acts as a generic JSON
//! editor.
//!
//! An `object-list` row shape is declared here because the manifest only
//! declares the port type, not the row fields. The declarations below mirror
//! the shapes that the product already writes and reads:
//!
//! | shape | source |
//! |---|---|
//! | `experiment@1.factors` | `AGENT_API.md` `factors[]` (`name`, `levels`) |
//! | `step@{1,2,3}.checks` | AsterDoc documents and `protocol` projection (`checkId`, `title`, `blocking`) |
//! | `step@{1,2,3}.tasks` | AsterDoc documents (`id`, `title`, `kind`, `readout`) |
//! | `workflow@1.edges` | AsterDoc documents and `connect_steps` (`from`, `to`, `id`) |
//!
//! Validation rejects a non-list value, a row that is not an object, a row field
//! that does not match the declared type, an unknown row field, a missing
//! required row field and a duplicate unique row. Serialization is canonical:
//! validating a legal value and serializing it again yields the same JSON.

use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashSet};

pub const COMPOSITE_FIELDS_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum RowFieldType {
    Text,
    Boolean,
    StringList,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RowField {
    pub name: &'static str,
    pub field_type: RowFieldType,
    pub required: bool,
    /// Maximum item count for a `StringList` field; `None` means unbounded.
    pub max_items: Option<usize>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ObjectListShape {
    /// Stable shape id, e.g. `experiment@1.factors`.
    pub id: &'static str,
    pub fields: &'static [RowField],
    /// Field groups that must be unique within one list.
    pub unique_sets: &'static [&'static [&'static str]],
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DeclaredShape {
    StringList,
    ObjectList(&'static ObjectListShape),
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct DeclaredField {
    pub kind: &'static str,
    pub schema: u32,
    pub field: &'static str,
    pub shape: DeclaredShape,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CompositeIssueCode {
    NotAList,
    NotAnObject,
    MissingRequiredField,
    WrongFieldType,
    UnknownField,
    EmptyText,
    DuplicateRow,
    TooManyItems,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CompositeIssue {
    pub code: CompositeIssueCode,
    /// Path relative to the module: `factors[0].name`.
    pub path: String,
    pub message: String,
}

#[derive(Debug, Clone, PartialEq)]
pub enum RowValue {
    Text(String),
    Boolean(bool),
    StringList(Vec<String>),
}

#[derive(Debug, Clone, PartialEq)]
pub struct CompositeRow {
    /// Only fields present in the source value; absent optional fields stay absent.
    pub values: BTreeMap<String, RowValue>,
}

#[derive(Debug, Clone, PartialEq)]
pub enum CompositeValue {
    StringList(Vec<String>),
    ObjectList(Vec<CompositeRow>),
}

/// Every `object-list` port declared by `spec/modules.json`.
pub const DECLARED_OBJECT_LIST_FIELDS: &[DeclaredField] = &[
    DeclaredField {
        kind: "experiment",
        schema: 1,
        field: "factors",
        shape: DeclaredShape::ObjectList(&FACTORS_SHAPE),
    },
    DeclaredField {
        kind: "step",
        schema: 1,
        field: "checks",
        shape: DeclaredShape::ObjectList(&STEP_CHECKS_SHAPE),
    },
    DeclaredField {
        kind: "step",
        schema: 1,
        field: "tasks",
        shape: DeclaredShape::ObjectList(&STEP_TASKS_SHAPE),
    },
    DeclaredField {
        kind: "step",
        schema: 2,
        field: "checks",
        shape: DeclaredShape::ObjectList(&STEP_CHECKS_SHAPE),
    },
    DeclaredField {
        kind: "step",
        schema: 2,
        field: "tasks",
        shape: DeclaredShape::ObjectList(&STEP_TASKS_SHAPE),
    },
    DeclaredField {
        kind: "step",
        schema: 3,
        field: "checks",
        shape: DeclaredShape::ObjectList(&STEP_CHECKS_SHAPE),
    },
    DeclaredField {
        kind: "step",
        schema: 3,
        field: "tasks",
        shape: DeclaredShape::ObjectList(&STEP_TASKS_SHAPE),
    },
    DeclaredField {
        kind: "workflow",
        schema: 1,
        field: "edges",
        shape: DeclaredShape::ObjectList(&WORKFLOW_EDGES_SHAPE),
    },
];

pub const FACTORS_SHAPE: ObjectListShape = ObjectListShape {
    id: "experiment@1.factors",
    fields: &[
        RowField {
            name: "name",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "levels",
            field_type: RowFieldType::StringList,
            required: true,
            max_items: Some(50),
        },
    ],
    unique_sets: &[&["name"]],
};

pub const STEP_CHECKS_SHAPE: ObjectListShape = ObjectListShape {
    id: "step.checks",
    fields: &[
        RowField {
            name: "checkId",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "title",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "blocking",
            field_type: RowFieldType::Boolean,
            required: false,
            max_items: None,
        },
    ],
    unique_sets: &[&["checkId"]],
};

pub const STEP_TASKS_SHAPE: ObjectListShape = ObjectListShape {
    id: "step.tasks",
    fields: &[
        RowField {
            name: "id",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "title",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "kind",
            field_type: RowFieldType::Text,
            required: false,
            max_items: None,
        },
        RowField {
            name: "readout",
            field_type: RowFieldType::Text,
            required: false,
            max_items: None,
        },
    ],
    unique_sets: &[&["id"]],
};

pub const WORKFLOW_EDGES_SHAPE: ObjectListShape = ObjectListShape {
    id: "workflow@1.edges",
    fields: &[
        RowField {
            name: "from",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "to",
            field_type: RowFieldType::Text,
            required: true,
            max_items: None,
        },
        RowField {
            name: "id",
            field_type: RowFieldType::Text,
            required: false,
            max_items: None,
        },
    ],
    unique_sets: &[&["from", "to"]],
};

/// Resolve the declared shape of one module input field.
pub fn declared_shape(kind: &str, schema: u32, field: &str) -> Option<DeclaredShape> {
    DECLARED_OBJECT_LIST_FIELDS
        .iter()
        .find(|entry| entry.kind == kind && entry.schema == schema && entry.field == field)
        .map(|entry| entry.shape)
}

/// Every declared object-list shape, deduplicated by shape id.
pub fn declared_object_list_shapes() -> Vec<&'static ObjectListShape> {
    let mut shapes: Vec<&'static ObjectListShape> = Vec::new();
    for entry in DECLARED_OBJECT_LIST_FIELDS {
        let DeclaredShape::ObjectList(shape) = entry.shape else {
            continue;
        };
        if !shapes.iter().any(|known| known.id == shape.id) {
            shapes.push(shape);
        }
    }
    shapes
}

/// Validate one field value against its declared shape.
pub fn validate_field_value(
    field: &str,
    shape: DeclaredShape,
    value: &Value,
) -> Result<CompositeValue, Vec<CompositeIssue>> {
    let mut issues = Vec::new();
    let parsed = match shape {
        DeclaredShape::StringList => CompositeValue::StringList(
            parse_string_list(field, value, None, &mut issues).unwrap_or_default(),
        ),
        DeclaredShape::ObjectList(shape) => {
            CompositeValue::ObjectList(parse_object_list(field, shape, value, &mut issues))
        }
    };
    if issues.is_empty() {
        Ok(parsed)
    } else {
        Err(issues)
    }
}

/// Serialize a validated value back to canonical JSON.
pub fn serialize_field_value(value: &CompositeValue) -> Value {
    match value {
        CompositeValue::StringList(items) => {
            Value::Array(items.iter().map(|item| Value::String(item.clone())).collect())
        }
        CompositeValue::ObjectList(rows) => Value::Array(
            rows.iter()
                .map(|row| {
                    let mut object = serde_json::Map::new();
                    for (name, value) in &row.values {
                        object.insert(name.clone(), row_value_json(value));
                    }
                    Value::Object(object)
                })
                .collect(),
        ),
    }
}

fn row_value_json(value: &RowValue) -> Value {
    match value {
        RowValue::Text(text) => Value::String(text.clone()),
        RowValue::Boolean(flag) => Value::Bool(*flag),
        RowValue::StringList(items) => {
            Value::Array(items.iter().map(|item| Value::String(item.clone())).collect())
        }
    }
}

fn parse_object_list(
    field: &str,
    shape: &ObjectListShape,
    value: &Value,
    issues: &mut Vec<CompositeIssue>,
) -> Vec<CompositeRow> {
    let Some(items) = value.as_array() else {
        issues.push(issue(
            CompositeIssueCode::NotAList,
            field,
            format!("{} must be a list of objects", shape.id),
        ));
        return Vec::new();
    };
    let mut rows = Vec::with_capacity(items.len());
    for (index, item) in items.iter().enumerate() {
        let path = format!("{field}[{index}]");
        let Some(object) = item.as_object() else {
            issues.push(issue(
                CompositeIssueCode::NotAnObject,
                &path,
                format!("{} row must be an object", shape.id),
            ));
            continue;
        };
        let mut values = BTreeMap::new();
        for name in object.keys() {
            if !shape.fields.iter().any(|declared| declared.name == name) {
                issues.push(issue(
                    CompositeIssueCode::UnknownField,
                    &format!("{path}.{name}"),
                    format!("{} does not declare field {name}", shape.id),
                ));
            }
        }
        for declared in shape.fields {
            let field_path = format!("{path}.{}", declared.name);
            let Some(raw) = object.get(declared.name) else {
                if declared.required {
                    issues.push(issue(
                        CompositeIssueCode::MissingRequiredField,
                        &field_path,
                        format!("{} needs {}", shape.id, declared.name),
                    ));
                }
                continue;
            };
            if let Some(parsed) = parse_row_value(declared, raw, &field_path, issues) {
                values.insert(declared.name.to_owned(), parsed);
            }
        }
        rows.push(CompositeRow { values });
    }
    check_unique_sets(field, shape, &rows, issues);
    rows
}

fn parse_row_value(
    declared: &RowField,
    raw: &Value,
    path: &str,
    issues: &mut Vec<CompositeIssue>,
) -> Option<RowValue> {
    match declared.field_type {
        RowFieldType::Text => match raw.as_str() {
            Some(text) if !text.trim().is_empty() => Some(RowValue::Text(text.to_owned())),
            Some(_) => {
                issues.push(issue(
                    CompositeIssueCode::EmptyText,
                    path,
                    format!("{} must not be empty", declared.name),
                ));
                None
            }
            None => {
                issues.push(issue(
                    CompositeIssueCode::WrongFieldType,
                    path,
                    format!("{} must be a string", declared.name),
                ));
                None
            }
        },
        RowFieldType::Boolean => match raw.as_bool() {
            Some(flag) => Some(RowValue::Boolean(flag)),
            None => {
                issues.push(issue(
                    CompositeIssueCode::WrongFieldType,
                    path,
                    format!("{} must be a boolean", declared.name),
                ));
                None
            }
        },
        RowFieldType::StringList => parse_string_list(
            path,
            raw,
            declared.max_items,
            issues,
        )
        .map(RowValue::StringList),
    }
}

fn parse_string_list(
    path: &str,
    raw: &Value,
    max_items: Option<usize>,
    issues: &mut Vec<CompositeIssue>,
) -> Option<Vec<String>> {
    let Some(items) = raw.as_array() else {
        issues.push(issue(
            CompositeIssueCode::NotAList,
            path,
            format!("{path} must be a list of strings"),
        ));
        return None;
    };
    if let Some(max) = max_items {
        if items.len() > max {
            issues.push(issue(
                CompositeIssueCode::TooManyItems,
                path,
                format!("{path} must not contain more than {max} items"),
            ));
        }
    }
    let mut parsed = Vec::with_capacity(items.len());
    for (index, item) in items.iter().enumerate() {
        match item.as_str() {
            Some(text) => parsed.push(text.to_owned()),
            None => issues.push(issue(
                CompositeIssueCode::WrongFieldType,
                &format!("{path}[{index}]"),
                format!("{path} items must be strings"),
            )),
        }
    }
    Some(parsed)
}

fn check_unique_sets(
    field: &str,
    shape: &ObjectListShape,
    rows: &[CompositeRow],
    issues: &mut Vec<CompositeIssue>,
) {
    for unique in shape.unique_sets {
        let Some(first) = unique.first() else {
            continue;
        };
        let mut seen: HashSet<Vec<String>> = HashSet::new();
        for (index, row) in rows.iter().enumerate() {
            let mut key = Vec::with_capacity(unique.len());
            let mut complete = true;
            for name in unique.iter() {
                match row.values.get(*name) {
                    Some(value) => key.push(row_value_key(value)),
                    None => {
                        complete = false;
                        break;
                    }
                }
            }
            if !complete {
                continue;
            }
            if !seen.insert(key.clone()) {
                issues.push(issue(
                    CompositeIssueCode::DuplicateRow,
                    &format!("{field}[{index}].{first}"),
                    format!("{} repeats row {}", shape.id, key.join(" / ")),
                ));
            }
        }
    }
}

fn row_value_key(value: &RowValue) -> String {
    match value {
        RowValue::Text(text) => text.clone(),
        RowValue::Boolean(flag) => flag.to_string(),
        RowValue::StringList(items) => items.join("\u{1f}"),
    }
}

fn issue(
    code: CompositeIssueCode,
    path: &str,
    message: String,
) -> CompositeIssue {
    CompositeIssue {
        code,
        path: path.to_owned(),
        message,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn codes(issues: &[CompositeIssue]) -> Vec<(CompositeIssueCode, String)> {
        issues
            .iter()
            .map(|issue| (issue.code, issue.path.clone()))
            .collect()
    }

    fn object_list(id: &str) -> &'static ObjectListShape {
        declared_object_list_shapes()
            .into_iter()
            .find(|shape| shape.id == id)
            .unwrap_or_else(|| panic!("shape {id} is not declared"))
    }

    fn step_checks() -> DeclaredShape {
        declared_shape("step", 2, "checks").expect("step@2 checks is declared")
    }

    fn step_tasks() -> DeclaredShape {
        declared_shape("step", 3, "tasks").expect("step@3 tasks is declared")
    }

    fn factors() -> DeclaredShape {
        declared_shape("experiment", 1, "factors").expect("experiment@1 factors is declared")
    }

    fn edges() -> DeclaredShape {
        declared_shape("workflow", 1, "edges").expect("workflow@1 edges is declared")
    }

    fn serialized_rows(value: &CompositeValue) -> Vec<Value> {
        match serialize_field_value(value) {
            Value::Array(rows) => rows,
            other => panic!("expected an array, got {other}"),
        }
    }

    #[test]
    fn declared_object_lists_are_the_manifest_ports() {
        let mut declared: Vec<(String, String)> = DECLARED_OBJECT_LIST_FIELDS
            .iter()
            .map(|entry| {
                (
                    format!("{}@{}", entry.kind, entry.schema),
                    entry.field.to_owned(),
                )
            })
            .collect();
        declared.sort();
        assert_eq!(
            declared,
            vec![
                ("experiment@1".to_owned(), "factors".to_owned()),
                ("step@1".to_owned(), "checks".to_owned()),
                ("step@1".to_owned(), "tasks".to_owned()),
                ("step@2".to_owned(), "checks".to_owned()),
                ("step@2".to_owned(), "tasks".to_owned()),
                ("step@3".to_owned(), "checks".to_owned()),
                ("step@3".to_owned(), "tasks".to_owned()),
                ("workflow@1".to_owned(), "edges".to_owned()),
            ]
        );
        let mut ids: Vec<&str> = declared_object_list_shapes()
            .into_iter()
            .map(|shape| shape.id)
            .collect();
        ids.sort_unstable();
        assert_eq!(
            ids,
            vec![
                "experiment@1.factors",
                "step.checks",
                "step.tasks",
                "workflow@1.edges",
            ]
        );
        assert_eq!(object_list("step.checks").fields.len(), 3);
        assert_eq!(declared_shape("step", 3, "timepoints"), None);
        assert_eq!(declared_shape("assay", 1, "checks"), None);
    }

    #[test]
    fn string_list_accepts_and_round_trips() {
        let value = json!(["0 h", "24 h", "48 h"]);
        let parsed =
            validate_field_value("timepoints", DeclaredShape::StringList, &value).unwrap();
        assert_eq!(
            parsed,
            CompositeValue::StringList(vec![
                "0 h".to_owned(),
                "24 h".to_owned(),
                "48 h".to_owned(),
            ])
        );
        assert_eq!(serialize_field_value(&parsed), value);
        let serialized = serialize_field_value(&parsed);
        assert_eq!(
            validate_field_value("timepoints", DeclaredShape::StringList, &serialized).unwrap(),
            parsed
        );
    }

    #[test]
    fn string_list_rejects_a_non_list() {
        let issues =
            validate_field_value("timepoints", DeclaredShape::StringList, &json!("24 h"))
                .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::NotAList, "timepoints".to_owned())]
        );
    }

    #[test]
    fn string_list_rejects_non_string_items() {
        let issues = validate_field_value(
            "timepoints",
            DeclaredShape::StringList,
            &json!(["24 h", 42]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::WrongFieldType,
                "timepoints[1]".to_owned()
            )]
        );
    }

    #[test]
    fn object_list_rejects_a_non_list() {
        let issues =
            validate_field_value("checks", step_checks(), &json!({"checkId": "c1"}))
                .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::NotAList, "checks".to_owned())]
        );
    }

    #[test]
    fn object_list_rejects_a_non_object_row() {
        let issues =
            validate_field_value("checks", step_checks(), &json!(["c1"])).unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::NotAnObject, "checks[0]".to_owned())]
        );
    }

    #[test]
    fn object_list_rejects_a_missing_required_field() {
        let issues = validate_field_value(
            "checks",
            step_checks(),
            &json!([{"title": "已完成"}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::MissingRequiredField,
                "checks[0].checkId".to_owned()
            )]
        );

        let issues = validate_field_value(
            "tasks",
            step_tasks(),
            &json!([{"title": "读取"}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::MissingRequiredField,
                "tasks[0].id".to_owned()
            )]
        );

        let issues =
            validate_field_value("edges", edges(), &json!([{"from": "a"}])).unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::MissingRequiredField, "edges[0].to".to_owned())]
        );

        let issues =
            validate_field_value("factors", factors(), &json!([{"name": "dose"}])).unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::MissingRequiredField,
                "factors[0].levels".to_owned()
            )]
        );
    }

    #[test]
    fn object_list_rejects_wrong_field_types() {
        let issues = validate_field_value(
            "checks",
            step_checks(),
            &json!([{"checkId": "c1", "title": "done", "blocking": "true"}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::WrongFieldType,
                "checks[0].blocking".to_owned()
            )]
        );

        let issues = validate_field_value(
            "factors",
            factors(),
            &json!([{"name": 7, "levels": ["control"]}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::WrongFieldType,
                "factors[0].name".to_owned()
            )]
        );

        let issues = validate_field_value(
            "factors",
            factors(),
            &json!([{"name": "dose", "levels": "control"}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::NotAList,
                "factors[0].levels".to_owned()
            )]
        );

        let issues = validate_field_value(
            "factors",
            factors(),
            &json!([{"name": "dose", "levels": ["control", 3]}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(
                CompositeIssueCode::WrongFieldType,
                "factors[0].levels[1]".to_owned()
            )]
        );
    }

    #[test]
    fn object_list_rejects_unknown_fields() {
        let issues = validate_field_value(
            "tasks",
            step_tasks(),
            &json!([{"id": "read", "title": "读取", "owner": "alice"}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::UnknownField, "tasks[0].owner".to_owned())]
        );
    }

    #[test]
    fn object_list_rejects_empty_text() {
        let issues =
            validate_field_value("edges", edges(), &json!([{"from": " ", "to": "b"}]))
                .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::EmptyText, "edges[0].from".to_owned())]
        );
    }

    #[test]
    fn object_list_rejects_duplicate_unique_rows() {
        let issues = validate_field_value(
            "checks",
            step_checks(),
            &json!([
                {"checkId": "c1", "title": "first"},
                {"checkId": "c1", "title": "second"},
            ]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::DuplicateRow, "checks[1].checkId".to_owned())]
        );

        let issues = validate_field_value(
            "edges",
            edges(),
            &json!([{"from": "a", "to": "b"}, {"from": "a", "to": "b"}]),
        )
        .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::DuplicateRow, "edges[1].from".to_owned())]
        );
    }

    #[test]
    fn object_list_rejects_too_many_levels() {
        let levels: Vec<String> = (0..51).map(|index| format!("level-{index}")).collect();
        let issues =
            validate_field_value("factors", factors(), &json!([{"name": "dose", "levels": levels}]))
                .unwrap_err();
        assert_eq!(
            codes(&issues),
            vec![(CompositeIssueCode::TooManyItems, "factors[0].levels".to_owned())]
        );
    }

    #[test]
    fn legal_object_lists_round_trip() {
        let cases = [
            (
                "factors",
                factors(),
                json!([
                    {"name": "treatment", "levels": ["control", "material"]},
                    {"name": "time", "levels": ["24 h"]},
                ]),
            ),
            (
                "checks",
                step_checks(),
                json!([
                    {"checkId": "c01-pellet", "title": "保留细胞沉淀；弃培养基", "blocking": true},
                    {"checkId": "c02-lysis", "title": "细胞沉淀完全分散"},
                ]),
            ),
            (
                "tasks",
                step_tasks(),
                json!([
                    {"id": "collect", "title": "转移细胞至 RNase-free 管", "kind": "procedure"},
                    {"id": "qc", "title": "记录 RNA 浓度", "kind": "measurement", "readout": "rna_qc"},
                ]),
            ),
            (
                "edges",
                edges(),
                json!([
                    {"from": "step-seed", "to": "step-read"},
                    {"id": "edge-b-c", "from": "step-read", "to": "step-analyse"},
                ]),
            ),
        ];
        for (field, shape, value) in cases {
            let parsed = validate_field_value(field, shape, &value).unwrap();
            let serialized = serialize_field_value(&parsed);
            assert_eq!(serialized, value);
            assert_eq!(
                validate_field_value(field, shape, &serialized).unwrap(),
                parsed
            );
        }
    }

    #[test]
    fn empty_lists_and_absent_optional_fields_round_trip() {
        let parsed =
            validate_field_value("checks", step_checks(), &json!([])).unwrap();
        assert_eq!(serialized_rows(&parsed), Vec::<Value>::new());

        let parsed = validate_field_value(
            "checks",
            step_checks(),
            &json!([{"checkId": "c1", "title": "done"}]),
        )
        .unwrap();
        assert_eq!(
            serialized_rows(&parsed),
            vec![json!({"checkId": "c1", "title": "done"})]
        );
    }
}
