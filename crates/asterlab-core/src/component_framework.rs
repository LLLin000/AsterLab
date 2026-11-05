//! C0/C1.1 headless unified component framework.
//!
//! The view document is a presentation/composition layer over the scientific
//! core. It stores component organization, presentation props and typed model
//! references; it is not a second Protocol/Run and does not own execution order,
//! sample identity or scientific parameter truth.

use crate::semantic_component::{QualifiedStepRef, SemanticIssue};
use crate::semantic_tool::validate_schema_value;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::collections::{BTreeMap, HashMap, HashSet};

pub const COMPONENT_FRAMEWORK_SCHEMA_VERSION: u32 = 2;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum ComponentFamily {
    Workflow,
    Operation,
    Layout,
    Tool,
    Content,
    Data,
    Visualization,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ComponentValueType {
    Text,
    Number,
    Integer,
    Boolean,
    Json,
    Sample,
    Data,
    Resource,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum SlotCardinality {
    One,
    Many,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ComponentDefinitionRef {
    pub id: String,
    pub version: u32,
}

impl ComponentDefinitionRef {
    pub fn new(id: impl Into<String>, version: u32) -> Self {
        Self {
            id: id.into(),
            version,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ComponentPort {
    pub id: String,
    pub value_type: ComponentValueType,
    #[serde(default)]
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct SlotDefinition {
    pub id: String,
    #[serde(default)]
    pub accepts: Vec<ComponentFamily>,
    /// Optional exact-definition allow-list. Empty means any definition in an
    /// accepted family.
    #[serde(default)]
    pub accepts_definitions: Vec<String>,
    pub cardinality: SlotCardinality,
    #[serde(default)]
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ComponentDefinition {
    pub id: String,
    pub version: u32,
    pub family: ComponentFamily,
    pub title: String,
    pub props_schema: Value,
    #[serde(default)]
    pub capabilities: Vec<String>,
    #[serde(default)]
    pub inputs: Vec<ComponentPort>,
    #[serde(default)]
    pub outputs: Vec<ComponentPort>,
    #[serde(default)]
    pub slots: Vec<SlotDefinition>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub presentation: Option<String>,
    #[serde(default)]
    pub agent_operations: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ModelRef {
    DesignVersion {
        id: String,
    },
    ProtocolVersion {
        id: String,
    },
    BlockVersion {
        id: String,
    },
    /// The definition is a BlockVersion; the use location is a real Protocol
    /// step. `instance_id` in the view is not the scientific identity.
    OperationUse {
        block_version_id: String,
        step: QualifiedStepRef,
    },
    PlannedContainer {
        design_version_id: String,
        container_id: String,
    },
    Run {
        id: String,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct VisualSpec {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub symbol: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub template: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ComponentInstance {
    pub instance_id: String,
    pub definition: ComponentDefinitionRef,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_ref: Option<ModelRef>,
    #[serde(default)]
    pub props: Value,
    #[serde(default)]
    pub slots: BTreeMap<String, Vec<ComponentInstance>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub workflow: Option<WorkflowGraph>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub visual: Option<VisualSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct WorkflowGraph {
    pub nodes: Vec<WorkflowNode>,
    #[serde(default)]
    pub edges: Vec<WorkflowEdge>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct WorkflowNode {
    pub node_id: String,
    pub component: Box<ComponentInstance>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct WorkflowEdge {
    pub from: String,
    pub to: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(deny_unknown_fields)]
pub struct ComponentPortRef {
    pub instance_id: String,
    pub port_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ModelPath {
    pub model: ModelRef,
    pub path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum BindingSource {
    Model(ModelPath),
    ComponentPort(ComponentPortRef),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct DataBinding {
    pub binding_id: String,
    pub source: BindingSource,
    pub target: ComponentPortRef,
    pub value_type: ComponentValueType,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ViewDocument {
    pub schema_version: u32,
    pub document_id: String,
    pub revision: u64,
    pub root: ComponentInstance,
    #[serde(default)]
    pub bindings: Vec<DataBinding>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum ViewPatch {
    UpdateProps {
        instance_id: String,
        property: String,
        value: Value,
    },
    AddSlotChild {
        parent_instance_id: String,
        slot_id: String,
        child: ComponentInstance,
    },
    MoveSlotChild {
        instance_id: String,
        target_parent_instance_id: String,
        target_slot_id: String,
        target_index: Option<usize>,
    },
    AddWorkflowNode {
        workflow_instance_id: String,
        node: WorkflowNode,
    },
    RemoveInstance {
        instance_id: String,
    },
    Connect {
        workflow_instance_id: String,
        edge: WorkflowEdge,
    },
    Disconnect {
        workflow_instance_id: String,
        edge: WorkflowEdge,
    },
    AddBinding {
        binding: DataBinding,
    },
    ReplaceBinding {
        binding: DataBinding,
    },
    RemoveBinding {
        binding_id: String,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ViewChangeSet {
    pub changeset_id: String,
    pub document_id: String,
    pub expected_revision: u64,
    pub operations: Vec<ViewPatch>,
    #[serde(default)]
    pub provenance: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ViewChangeResult {
    pub document: ViewDocument,
    pub changed_instances: Vec<String>,
    pub affected_bindings: Vec<String>,
    pub requires_confirmation: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ModelPathResolution {
    Resolved(ComponentValueType),
    Missing,
    Unknown,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ContextValidationMode {
    Strict,
    AllowUnresolved,
}

pub trait ComponentBindingContext {
    fn resolve_model_path(&self, model: &ModelRef, path: &str) -> ModelPathResolution;
    fn operation_use_exists(&self, block_version_id: &str, step: &QualifiedStepRef) -> bool;
}

#[derive(Debug, Clone, Default)]
pub struct ComponentRegistry {
    definitions: HashMap<(String, u32), ComponentDefinition>,
}

impl ComponentRegistry {
    /// Definitions are immutable by exact ID + version. Re-registering an equal
    /// definition is idempotent; a conflicting definition is rejected.
    pub fn register(&mut self, definition: ComponentDefinition) -> Result<(), String> {
        let key = (definition.id.clone(), definition.version);
        if let Some(existing) = self.definitions.get(&key) {
            if existing == &definition {
                return Ok(());
            }
            return Err(format!(
                "definition {}@{} is already registered with different content",
                definition.id, definition.version
            ));
        }
        if definition.id.trim().is_empty()
            || definition.version == 0
            || definition.title.trim().is_empty()
            || !definition.props_schema.is_object()
        {
            return Err("component definition is missing required fields".into());
        }
        let mut port_ids = HashSet::new();
        for port in definition.inputs.iter().chain(definition.outputs.iter()) {
            if port.id.trim().is_empty() {
                return Err(format!("component {}@{} has an empty port id", definition.id, definition.version));
            }
            if !port_ids.insert(port.id.as_str()) {
                return Err(format!(
                    "component {}@{} has duplicate port id {}",
                    definition.id, definition.version, port.id
                ));
            }
        }
        let mut slot_ids = HashSet::new();
        for slot in &definition.slots {
            if slot.id.trim().is_empty() {
                return Err(format!("component {}@{} has an empty slot id", definition.id, definition.version));
            }
            if !slot_ids.insert(slot.id.as_str()) {
                return Err(format!(
                    "component {}@{} has duplicate slot id {}",
                    definition.id, definition.version, slot.id
                ));
            }
        }
        self.definitions.insert(key, definition);
        Ok(())
    }

    pub fn get(&self, reference: &ComponentDefinitionRef) -> Option<&ComponentDefinition> {
        self.definitions
            .get(&(reference.id.clone(), reference.version))
    }
}

fn reg(registry: &mut ComponentRegistry, definition: ComponentDefinition) {
    registry
        .register(definition)
        .expect("valid built-in definition");
}

pub fn default_registry() -> ComponentRegistry {
    let mut registry = ComponentRegistry::default();
    reg(
        &mut registry,
        ComponentDefinition {
            id: "workflow".into(),
            version: 1,
            family: ComponentFamily::Workflow,
            title: "Workflow".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"label":{"type":"string"},"collapsed":{"type":"boolean"},"style":{"type":"string"}}}),
            capabilities: vec!["compose".into(), "navigate".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![SlotDefinition {
                id: "detail".into(),
                accepts: vec![
                    ComponentFamily::Layout,
                    ComponentFamily::Tool,
                    ComponentFamily::Content,
                    ComponentFamily::Workflow,
                    ComponentFamily::Operation,
                ],
                accepts_definitions: vec![],
                cardinality: SlotCardinality::Many,
                required: false,
            }],
            presentation: Some("workflow.graph".into()),
            agent_operations: vec!["update".into(), "add_node".into(), "connect".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "operation".into(),
            version: 1,
            family: ComponentFamily::Operation,
            title: "Operation".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"label":{"type":"string"},"collapsed":{"type":"boolean"},"style":{"type":"string"}}}),
            capabilities: vec!["bind".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![SlotDefinition {
                id: "detail".into(),
                accepts: vec![
                    ComponentFamily::Layout,
                    ComponentFamily::Tool,
                    ComponentFamily::Content,
                    ComponentFamily::Workflow,
                ],
                accepts_definitions: vec![],
                cardinality: SlotCardinality::Many,
                required: false,
            }],
            presentation: Some("operation.node".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "layout.table".into(),
            version: 1,
            family: ComponentFamily::Layout,
            title: "Table".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"rows":{"type":"array"}}}),
            capabilities: vec!["bind".into()],
            inputs: vec![ComponentPort {
                id: "data".into(),
                value_type: ComponentValueType::Data,
                required: false,
            }],
            outputs: vec![ComponentPort {
                id: "values".into(),
                value_type: ComponentValueType::Data,
                required: false,
            }],
            slots: vec![],
            presentation: Some("table".into()),
            agent_operations: vec!["update".into(), "bind".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "lab.well_plate".into(),
            version: 1,
            family: ComponentFamily::Layout,
            title: "Well plate".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"format":{"type":"string"}}}),
            capabilities: vec!["bind".into(), "layout".into()],
            inputs: vec![
                ComponentPort {
                    id: "groups".into(),
                    value_type: ComponentValueType::Data,
                    required: false,
                },
                ComponentPort {
                    id: "volumes".into(),
                    value_type: ComponentValueType::Data,
                    required: false,
                },
            ],
            outputs: vec![ComponentPort {
                id: "plate_plan".into(),
                value_type: ComponentValueType::Data,
                required: false,
            }],
            slots: vec![],
            presentation: Some("well_plate".into()),
            agent_operations: vec!["update".into(), "bind".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "tool.dilution".into(),
            version: 1,
            family: ComponentFamily::Tool,
            title: "Dilution calculator".into(),
            props_schema: json!({"type":"object"}),
            capabilities: vec!["calculate".into()],
            inputs: vec![ComponentPort {
                id: "targets".into(),
                value_type: ComponentValueType::Data,
                required: false,
            }],
            outputs: vec![ComponentPort {
                id: "plan".into(),
                value_type: ComponentValueType::Data,
                required: false,
            }],
            slots: vec![],
            presentation: Some("tool.dilution".into()),
            agent_operations: vec!["bind".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "content.note".into(),
            version: 1,
            family: ComponentFamily::Content,
            title: "Note".into(),
            props_schema: json!({"type":"object","properties":{"text":{"type":"string"}}}),
            capabilities: vec!["content".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![],
            presentation: Some("content.note".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "layout.section".into(),
            version: 1,
            family: ComponentFamily::Layout,
            title: "Section".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"title":{"type":"string"},"collapsed":{"type":"boolean"}}}),
            capabilities: vec!["compose".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![SlotDefinition {
                id: "children".into(),
                accepts: vec![
                    ComponentFamily::Layout,
                    ComponentFamily::Content,
                    ComponentFamily::Tool,
                    ComponentFamily::Workflow,
                    ComponentFamily::Operation,
                ],
                accepts_definitions: vec![],
                cardinality: SlotCardinality::Many,
                required: false,
            }],
            presentation: Some("layout.section".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "content.markdown".into(),
            version: 1,
            family: ComponentFamily::Content,
            title: "Markdown".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"markdown":{"type":"string"}}}),
            capabilities: vec!["content".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![],
            presentation: Some("content.markdown".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "content.table".into(),
            version: 1,
            family: ComponentFamily::Content,
            title: "Table".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"mode":{"type":"string","enum":["authored","bound"]},"markdown":{"type":"string"},"columns":{"type":"array"},"rows":{"type":"array"}}}),
            capabilities: vec!["content".into(), "bind".into()],
            inputs: vec![ComponentPort { id: "data".into(), value_type: ComponentValueType::Data, required: false }],
            outputs: vec![ComponentPort { id: "values".into(), value_type: ComponentValueType::Data, required: false }],
            slots: vec![],
            presentation: Some("content.table".into()),
            agent_operations: vec!["update".into(), "bind".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "lab.container_view".into(),
            version: 1,
            family: ComponentFamily::Layout,
            title: "Container".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"label":{"type":"string"}}}),
            capabilities: vec!["layout".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![],
            presentation: Some("lab.container_view".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "lab.rack_layout".into(),
            version: 1,
            family: ComponentFamily::Layout,
            title: "Rack".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"rows":{"type":"integer"},"columns":{"type":"integer"}}}),
            capabilities: vec!["layout".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![],
            presentation: Some("lab.rack_layout".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "tool.calculator".into(),
            version: 1,
            family: ComponentFamily::Tool,
            title: "Calculator".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"tool_ref":{"type":"string"},"preset":{"type":"object"}}}),
            capabilities: vec!["calculate".into()],
            inputs: vec![ComponentPort { id: "values".into(), value_type: ComponentValueType::Data, required: false }],
            outputs: vec![ComponentPort { id: "result".into(), value_type: ComponentValueType::Data, required: false }],
            slots: vec![],
            presentation: Some("tool.calculator".into()),
            agent_operations: vec!["bind".into(), "update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "data.reference".into(),
            version: 1,
            family: ComponentFamily::Data,
            title: "Data reference".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"path":{"type":"string"}}}),
            capabilities: vec!["reference".into()],
            inputs: vec![],
            outputs: vec![ComponentPort { id: "value".into(), value_type: ComponentValueType::Data, required: false }],
            slots: vec![],
            presentation: Some("data.reference".into()),
            agent_operations: vec!["update".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "visual.chart".into(),
            version: 1,
            family: ComponentFamily::Visualization,
            title: "Chart".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"kind":{"type":"string"}}}),
            capabilities: vec!["visualize".into()],
            inputs: vec![ComponentPort { id: "data".into(), value_type: ComponentValueType::Data, required: false }],
            outputs: vec![],
            slots: vec![],
            presentation: Some("visual.chart".into()),
            agent_operations: vec!["update".into(), "bind".into()],
        },
    );
    reg(
        &mut registry,
        ComponentDefinition {
            id: "media.figure".into(),
            version: 1,
            family: ComponentFamily::Visualization,
            title: "Figure".into(),
            props_schema: json!({"type":"object","additionalProperties":false,"properties":{"src":{"type":"string"},"alt":{"type":"string"}}}),
            capabilities: vec!["visualize".into()],
            inputs: vec![],
            outputs: vec![],
            slots: vec![],
            presentation: Some("media.figure".into()),
            agent_operations: vec!["update".into()],
        },
    );
    registry
}

impl ViewDocument {
    /// Structural validation only. ModelPath existence and OperationUse
    /// existence require a ComponentBindingContext.
    pub fn validate(&self, registry: &ComponentRegistry) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = Vec::new();
        if self.schema_version != COMPONENT_FRAMEWORK_SCHEMA_VERSION {
            issues.push(issue(
                "schema_version",
                "schema_version",
                "unsupported component view schema version",
            ));
        }
        if self.document_id.trim().is_empty() {
            issues.push(issue(
                "empty_document",
                "document_id",
                "document id must not be empty",
            ));
        }
        if self.revision == 0 {
            issues.push(issue("revision", "revision", "view revision must be >= 1"));
        }
        let mut ids = HashSet::new();
        let mut owners = HashMap::new();
        validate_instance(&self.root, registry, &mut ids, &mut owners, &mut issues);

        let mut binding_ids = HashSet::new();
        let mut targets = HashSet::new();
        let mut dependency_edges: HashMap<String, Vec<String>> = HashMap::new();
        for binding in &self.bindings {
            if !binding_ids.insert(binding.binding_id.as_str()) {
                issues.push(issue(
                    "duplicate_binding",
                    &binding.binding_id,
                    "binding ids must be unique",
                ));
            }
            if !ids.contains(&binding.target.instance_id) {
                issues.push(issue(
                    "missing_target",
                    &binding.target.instance_id,
                    "binding target instance does not exist",
                ));
            }
            let target_key = format!("{}::{}", binding.target.instance_id, binding.target.port_id);
            if !targets.insert(target_key.clone()) {
                issues.push(issue(
                    "duplicate_target",
                    target_key,
                    "an input port accepts only one binding",
                ));
            }
            let target_port = owners
                .get(&binding.target.instance_id)
                .and_then(|reference| registry.get(reference))
                .and_then(|definition| {
                    definition
                        .inputs
                        .iter()
                        .find(|port| port.id == binding.target.port_id)
                });
            let Some(target_port) = target_port else {
                issues.push(issue(
                    "missing_target_port",
                    format!("{}::{}", binding.target.instance_id, binding.target.port_id),
                    "binding target is not a declared input port",
                ));
                continue;
            };
            if target_port.value_type != binding.value_type {
                issues.push(issue(
                    "binding_type",
                    &binding.binding_id,
                    "binding value type does not match target port",
                ));
            }
            match &binding.source {
                BindingSource::Model(path) => {
                    if path.path.trim().is_empty() {
                        issues.push(issue(
                            "empty_model_path",
                            &binding.binding_id,
                            "model path must not be empty",
                        ));
                    }
                    validate_model_ref(&path.model, &binding.binding_id, &mut issues);
                }
                BindingSource::ComponentPort(source) => {
                    if !ids.contains(&source.instance_id) {
                        issues.push(issue(
                            "missing_source",
                            &source.instance_id,
                            "binding source instance does not exist",
                        ));
                        continue;
                    }
                    let source_port = owners
                        .get(&source.instance_id)
                        .and_then(|reference| registry.get(reference))
                        .and_then(|definition| {
                            definition
                                .outputs
                                .iter()
                                .find(|port| port.id == source.port_id)
                        });
                    let Some(source_port) = source_port else {
                        issues.push(issue(
                            "missing_source_port",
                            format!("{}::{}", source.instance_id, source.port_id),
                            "binding source is not a declared output port",
                        ));
                        continue;
                    };
                    if source_port.value_type != binding.value_type {
                        issues.push(issue(
                            "binding_type",
                            &binding.binding_id,
                            "binding value type does not match source port",
                        ));
                    }
                    dependency_edges
                        .entry(source.instance_id.clone())
                        .or_default()
                        .push(binding.target.instance_id.clone());
                }
            }
        }
        if has_cycle(&dependency_edges) {
            issues.push(issue(
                "binding_cycle",
                "bindings",
                "component data bindings must not form a cycle",
            ));
        }
        finish(issues)
    }

    /// Structural plus contextual validation. Missing or unprovable model paths
    /// remain unresolved; they are never treated as valid data.
    pub fn validate_with_context(
        &self,
        registry: &ComponentRegistry,
        context: &dyn ComponentBindingContext,
    ) -> Result<(), Vec<SemanticIssue>> {
        self.validate_with_context_mode(registry, context, ContextValidationMode::Strict)
    }

    pub fn validate_with_context_mode(
        &self,
        registry: &ComponentRegistry,
        context: &dyn ComponentBindingContext,
        mode: ContextValidationMode,
    ) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = self.validate(registry).err().unwrap_or_default();
        let mut visited = HashSet::new();
        validate_operation_refs(&self.root, context, mode, &mut visited, &mut issues);
        for binding in &self.bindings {
            if let BindingSource::Model(path) = &binding.source {
                match context.resolve_model_path(&path.model, &path.path) {
                    ModelPathResolution::Resolved(value_type)
                        if value_type == binding.value_type => {}
                    ModelPathResolution::Resolved(_) => issues.push(issue(
                        "model_type",
                        &binding.binding_id,
                        "resolved model path type does not match binding type",
                    )),
                    ModelPathResolution::Missing => issues.push(issue(
                        "model_missing",
                        &binding.binding_id,
                        "model path does not exist in the supplied context",
                    )),
                    ModelPathResolution::Unknown => {
                        if mode == ContextValidationMode::Strict {
                            issues.push(issue(
                                "model_unknown",
                                &binding.binding_id,
                                "model path cannot be resolved",
                            ));
                        }
                    }
                }
            }
        }
        finish(issues)
    }

    pub fn apply_changeset(
        &self,
        changeset: &ViewChangeSet,
        registry: &ComponentRegistry,
    ) -> Result<ViewChangeResult, Vec<SemanticIssue>> {
        let mut issues = Vec::new();
        if changeset.changeset_id.trim().is_empty() {
            issues.push(issue(
                "empty_changeset",
                "changeset_id",
                "changeset id must not be empty",
            ));
        }
        if changeset.document_id != self.document_id {
            issues.push(issue(
                "document_mismatch",
                "document_id",
                "changeset targets a different document",
            ));
        }
        if changeset.expected_revision != self.revision {
            issues.push(issue(
                "revision_conflict",
                "expected_revision",
                "expected revision does not match the current document",
            ));
        }
        if changeset.operations.is_empty() {
            issues.push(issue("empty_changeset", "operations", "changeset has no operations"));
        }
        if !issues.is_empty() {
            return Err(issues);
        }
        self.validate(registry)?;

        let mut next = self.clone();
        let mut changed = Vec::new();
        let mut affected_bindings = Vec::new();
        let mut requires_confirmation = false;
        for patch in &changeset.operations {
            apply_patch(
                &mut next,
                patch,
                &mut changed,
                &mut affected_bindings,
                &mut requires_confirmation,
            )?;
        }
        next.revision += 1;
        next.validate(registry)?;
        Ok(ViewChangeResult {
            document: next,
            changed_instances: changed,
            affected_bindings,
            requires_confirmation,
        })
    }
}

fn validate_instance(
    instance: &ComponentInstance,
    registry: &ComponentRegistry,
    ids: &mut HashSet<String>,
    owners: &mut HashMap<String, ComponentDefinitionRef>,
    issues: &mut Vec<SemanticIssue>,
) {
    if instance.instance_id.trim().is_empty() {
        issues.push(issue(
            "empty_instance",
            "instance_id",
            "instance id must not be empty",
        ));
    }
    if !ids.insert(instance.instance_id.clone()) {
        issues.push(issue(
            "duplicate_instance",
            &instance.instance_id,
            "instance ids must be unique across the document",
        ));
    }
    owners.insert(instance.instance_id.clone(), instance.definition.clone());
    let Some(definition) = registry.get(&instance.definition) else {
        issues.push(issue(
            "missing_definition",
            &instance.instance_id,
            "component definition is not registered at the exact version",
        ));
        return;
    };
    if !instance.props.is_object() {
        issues.push(issue(
            "props_type",
            &instance.instance_id,
            "component props must be an object",
        ));
    } else {
        for error in validate_schema_value(
            &definition.props_schema,
            &instance.props,
            &format!("{}.props", instance.instance_id),
        ) {
            issues.push(issue(
                error.code,
                format!("{}.props", instance.instance_id),
                error.message,
            ));
        }
    }
    if let Some(model_ref) = &instance.model_ref {
        validate_model_ref(model_ref, &instance.instance_id, issues);
        if definition.family == ComponentFamily::Operation
            && !matches!(model_ref, ModelRef::OperationUse { .. })
        {
            issues.push(issue(
                "operation_ref",
                &instance.instance_id,
                "Operation components require an OperationUse reference",
            ));
        }
    } else if definition.family == ComponentFamily::Operation {
        issues.push(issue(
            "operation_ref",
            &instance.instance_id,
            "Operation components require a block version and step reference",
        ));
    }

    match definition.family {
        ComponentFamily::Workflow => {
            if instance.workflow.is_none() {
                issues.push(issue(
                    "workflow_body",
                    &instance.instance_id,
                    "workflow component requires a workflow graph",
                ));
            }
        }
        _ => {
            if instance.workflow.is_some() {
                issues.push(issue(
                    "workflow_body",
                    &instance.instance_id,
                    "only a workflow component may own a workflow graph",
                ));
            }
        }
    }

    for (slot_id, children) in &instance.slots {
        let Some(slot) = definition.slots.iter().find(|slot| slot.id == *slot_id) else {
            issues.push(issue(
                "unknown_slot",
                format!("{}.slots.{slot_id}", instance.instance_id),
                "slot is not declared",
            ));
            continue;
        };
        if slot.required && children.is_empty() {
            issues.push(issue(
                "required_slot",
                format!("{}.slots.{slot_id}", instance.instance_id),
                "required slot is empty",
            ));
        }
        if slot.cardinality == SlotCardinality::One && children.len() > 1 {
            issues.push(issue(
                "slot_cardinality",
                format!("{}.slots.{slot_id}", instance.instance_id),
                "single-value slot contains multiple children",
            ));
        }
        for child in children {
            if let Some(child_definition) = registry.get(&child.definition) {
                if !slot.accepts.is_empty() && !slot.accepts.contains(&child_definition.family) {
                    issues.push(issue(
                        "slot_family",
                        format!("{}.slots.{slot_id}", instance.instance_id),
                        "child family is not accepted by this slot",
                    ));
                }
                if !slot.accepts_definitions.is_empty()
                    && !slot
                        .accepts_definitions
                        .iter()
                        .any(|id| id == &child_definition.id)
                {
                    issues.push(issue(
                        "slot_definition",
                        format!("{}.slots.{slot_id}", instance.instance_id),
                        "child definition is not in the slot allow-list",
                    ));
                }
            }
            validate_instance(child, registry, ids, owners, issues);
        }
    }
    for slot in &definition.slots {
        if slot.required && !instance.slots.contains_key(&slot.id) {
            issues.push(issue(
                "required_slot",
                format!("{}.slots.{}", instance.instance_id, slot.id),
                "required slot is missing",
            ));
        }
    }

    if let Some(workflow) = &instance.workflow {
        let mut node_ids = HashSet::new();
        for node in &workflow.nodes {
            if !node_ids.insert(node.node_id.clone()) {
                issues.push(issue(
                    "duplicate_node",
                    &node.node_id,
                    "workflow node ids must be unique",
                ));
            }
            validate_instance(&node.component, registry, ids, owners, issues);
        }
        let mut edges = HashMap::<String, Vec<String>>::new();
        for edge in &workflow.edges {
            if !node_ids.contains(&edge.from) || !node_ids.contains(&edge.to) {
                issues.push(issue(
                    "missing_workflow_node",
                    format!("{}->{}", edge.from, edge.to),
                    "edge references a missing workflow node",
                ));
            }
            if edge.from == edge.to {
                issues.push(issue(
                    "self_edge",
                    &edge.from,
                    "workflow node cannot connect to itself",
                ));
            }
            edges
                .entry(edge.from.clone())
                .or_default()
                .push(edge.to.clone());
        }
        if has_cycle(&edges) {
            issues.push(issue(
                "workflow_cycle",
                &instance.instance_id,
                "workflow graph must not contain a cycle",
            ));
        }
    }
}

fn validate_model_ref(model: &ModelRef, path: &str, issues: &mut Vec<SemanticIssue>) {
    match model {
        ModelRef::DesignVersion { id }
        | ModelRef::ProtocolVersion { id }
        | ModelRef::BlockVersion { id }
        | ModelRef::Run { id } => {
            if id.trim().is_empty() {
                issues.push(issue(
                    "empty_model_ref",
                    path,
                    "model reference id must not be empty",
                ));
            }
        }
        ModelRef::OperationUse {
            block_version_id,
            step,
        } => {
            if block_version_id.trim().is_empty() {
                issues.push(issue(
                    "empty_block_ref",
                    path,
                    "block version id must not be empty",
                ));
            }
            if step.protocol_version_id.trim().is_empty() || step.step_path.trim().is_empty() {
                issues.push(issue(
                    "empty_step_ref",
                    path,
                    "operation use requires Protocol version and step path",
                ));
            }
        }
        ModelRef::PlannedContainer {
            design_version_id,
            container_id,
        } => {
            if design_version_id.trim().is_empty() || container_id.trim().is_empty() {
                issues.push(issue(
                    "empty_container_ref",
                    path,
                    "planned container reference requires design version and container id",
                ));
            }
        }
    }
}

fn validate_operation_refs(
    instance: &ComponentInstance,
    context: &dyn ComponentBindingContext,
    mode: ContextValidationMode,
    visited: &mut HashSet<String>,
    issues: &mut Vec<SemanticIssue>,
) {
    if !visited.insert(instance.instance_id.clone()) {
        return;
    }
    if let Some(ModelRef::OperationUse {
        block_version_id,
        step,
    }) = &instance.model_ref
    {
        if !context.operation_use_exists(block_version_id, step) {
            issues.push(issue(
                "operation_use_missing",
                &instance.instance_id,
                "operation use does not resolve in the supplied context",
            ));
        }
    }
    if let Some(ModelRef::PlannedContainer {
        design_version_id,
        container_id,
    }) = &instance.model_ref
    {
        match context.resolve_model_path(&instance.model_ref.clone().unwrap(), "") {
            ModelPathResolution::Missing => issues.push(issue(
                "planned_container_missing",
                &instance.instance_id,
                "planned container does not resolve in the supplied context",
            )),
            ModelPathResolution::Unknown if mode == ContextValidationMode::Strict => {
                issues.push(issue(
                    "planned_container_unknown",
                    &instance.instance_id,
                    "planned container cannot be resolved",
                ))
            }
            _ => {
                let _ = (design_version_id, container_id);
            }
        }
    }
    for children in instance.slots.values() {
        for child in children {
            validate_operation_refs(child, context, mode, visited, issues);
        }
    }
    if let Some(workflow) = &instance.workflow {
        for node in &workflow.nodes {
            validate_operation_refs(&node.component, context, mode, visited, issues);
        }
    }
}

fn apply_patch(
    document: &mut ViewDocument,
    patch: &ViewPatch,
    changed: &mut Vec<String>,
    affected_bindings: &mut Vec<String>,
    requires_confirmation: &mut bool,
) -> Result<(), Vec<SemanticIssue>> {
    match patch {
        ViewPatch::UpdateProps {
            instance_id,
            property,
            value,
        } => {
            let instance = find_instance_mut(&mut document.root, instance_id).ok_or_else(|| {
                vec![issue(
                    "missing_instance",
                    instance_id,
                    "instance does not exist",
                )]
            })?;
            let object = instance
                .props
                .as_object_mut()
                .ok_or_else(|| vec![issue("props_type", instance_id, "props must be an object")])?;
            object.insert(property.clone(), value.clone());
            changed.push(instance_id.clone());
        }
        ViewPatch::AddSlotChild {
            parent_instance_id,
            slot_id,
            child,
        } => {
            let parent =
                find_instance_mut(&mut document.root, parent_instance_id).ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        parent_instance_id,
                        "parent instance does not exist",
                    )]
                })?;
            parent
                .slots
                .entry(slot_id.clone())
                .or_default()
                .push(child.clone());
            changed.push(child.instance_id.clone());
        }
        ViewPatch::MoveSlotChild {
            instance_id,
            target_parent_instance_id,
            target_slot_id,
            target_index,
        } => {
            if document.root.instance_id == *instance_id {
                return Err(vec![issue(
                    "move_root",
                    instance_id,
                    "root instance cannot be moved",
                )]);
            }
            let descendants =
                collect_ids(find_instance(&document.root, instance_id).ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        instance_id,
                        "instance does not exist",
                    )]
                })?);
            if descendants.contains(target_parent_instance_id) {
                return Err(vec![issue(
                    "move_into_descendant",
                    instance_id,
                    "cannot move an instance into its own descendant",
                )]);
            }
            let moved = detach_slot_child(&mut document.root, instance_id).ok_or_else(|| {
                vec![issue(
                    "move_workflow_node",
                    instance_id,
                    "only slot children can be moved",
                )]
            })?;
            let parent = find_instance_mut(&mut document.root, target_parent_instance_id)
                .ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        target_parent_instance_id,
                        "target parent does not exist",
                    )]
                })?;
            let children = parent.slots.entry(target_slot_id.clone()).or_default();
            if let Some(index) = target_index {
                if *index > children.len() {
                    return Err(vec![issue(
                        "move_index",
                        instance_id,
                        "target index is outside the slot",
                    )]);
                }
            }
            let index = target_index.unwrap_or(children.len());
            children.insert(index, moved);
            changed.push(instance_id.clone());
            *requires_confirmation = true;
        }
        ViewPatch::AddWorkflowNode {
            workflow_instance_id,
            node,
        } => {
            let instance =
                find_instance_mut(&mut document.root, workflow_instance_id).ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        workflow_instance_id,
                        "workflow instance does not exist",
                    )]
                })?;
            let workflow = instance.workflow.as_mut().ok_or_else(|| {
                vec![issue(
                    "workflow_body",
                    workflow_instance_id,
                    "instance is not a workflow",
                )]
            })?;
            workflow.nodes.push(node.clone());
            changed.push(node.component.instance_id.clone());
            *requires_confirmation = true;
        }
        ViewPatch::RemoveInstance { instance_id } => {
            if document.root.instance_id == *instance_id {
                return Err(vec![issue(
                    "remove_root",
                    instance_id,
                    "root instance cannot be removed",
                )]);
            }
            let removed =
                collect_ids(find_instance(&document.root, instance_id).ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        instance_id,
                        "instance does not exist",
                    )]
                })?);
            remove_instance(&mut document.root, instance_id);
            document.bindings.retain(|binding| {
                let target_removed = removed.contains(&binding.target.instance_id);
                let source_removed = matches!(
                    &binding.source,
                    BindingSource::ComponentPort(source) if removed.contains(&source.instance_id)
                );
                if target_removed || source_removed {
                    affected_bindings.push(binding.binding_id.clone());
                    false
                } else {
                    true
                }
            });
            changed.extend(removed);
            *requires_confirmation = true;
        }
        ViewPatch::Connect {
            workflow_instance_id,
            edge,
        } => {
            let instance =
                find_instance_mut(&mut document.root, workflow_instance_id).ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        workflow_instance_id,
                        "workflow instance does not exist",
                    )]
                })?;
            let workflow = instance.workflow.as_mut().ok_or_else(|| {
                vec![issue(
                    "workflow_body",
                    workflow_instance_id,
                    "instance is not a workflow",
                )]
            })?;
            if workflow.edges.contains(edge) {
                return Err(vec![issue(
                    "duplicate_edge",
                    workflow_instance_id,
                    "workflow edge already exists",
                )]);
            }
            workflow.edges.push(edge.clone());
            *requires_confirmation = true;
        }
        ViewPatch::Disconnect {
            workflow_instance_id,
            edge,
        } => {
            let instance =
                find_instance_mut(&mut document.root, workflow_instance_id).ok_or_else(|| {
                    vec![issue(
                        "missing_instance",
                        workflow_instance_id,
                        "workflow instance does not exist",
                    )]
                })?;
            let workflow = instance.workflow.as_mut().ok_or_else(|| {
                vec![issue(
                    "workflow_body",
                    workflow_instance_id,
                    "instance is not a workflow",
                )]
            })?;
            let before = workflow.edges.len();
            workflow.edges.retain(|candidate| candidate != edge);
            if workflow.edges.len() == before {
                return Err(vec![issue(
                    "missing_edge",
                    workflow_instance_id,
                    "workflow edge does not exist",
                )]);
            }
            *requires_confirmation = true;
        }
        ViewPatch::AddBinding { binding } => {
            document.bindings.push(binding.clone());
            affected_bindings.push(binding.binding_id.clone());
        }
        ViewPatch::ReplaceBinding { binding } => {
            let Some(existing) = document
                .bindings
                .iter_mut()
                .find(|existing| existing.binding_id == binding.binding_id)
            else {
                return Err(vec![issue("missing_binding", &binding.binding_id, "binding does not exist")]);
            };
            *existing = binding.clone();
            affected_bindings.push(binding.binding_id.clone());
        }
        ViewPatch::RemoveBinding { binding_id } => {
            let before = document.bindings.len();
            document
                .bindings
                .retain(|binding| binding.binding_id != *binding_id);
            if document.bindings.len() == before {
                return Err(vec![issue(
                    "missing_binding",
                    binding_id,
                    "binding does not exist",
                )]);
            }
            affected_bindings.push(binding_id.clone());
        }
    }
    Ok(())
}

fn find_instance<'a>(
    instance: &'a ComponentInstance,
    instance_id: &str,
) -> Option<&'a ComponentInstance> {
    if instance.instance_id == instance_id {
        return Some(instance);
    }
    for children in instance.slots.values() {
        for child in children {
            if let Some(found) = find_instance(child, instance_id) {
                return Some(found);
            }
        }
    }
    if let Some(workflow) = &instance.workflow {
        for node in &workflow.nodes {
            if let Some(found) = find_instance(&node.component, instance_id) {
                return Some(found);
            }
        }
    }
    None
}

fn find_instance_mut<'a>(
    instance: &'a mut ComponentInstance,
    instance_id: &str,
) -> Option<&'a mut ComponentInstance> {
    if instance.instance_id == instance_id {
        return Some(instance);
    }
    for children in instance.slots.values_mut() {
        for child in children {
            if let Some(found) = find_instance_mut(child, instance_id) {
                return Some(found);
            }
        }
    }
    if let Some(workflow) = &mut instance.workflow {
        for node in &mut workflow.nodes {
            if let Some(found) = find_instance_mut(&mut node.component, instance_id) {
                return Some(found);
            }
        }
    }
    None
}

fn collect_ids(instance: &ComponentInstance) -> HashSet<String> {
    let mut ids = HashSet::new();
    collect_ids_inner(instance, &mut ids);
    ids
}

fn collect_ids_inner(instance: &ComponentInstance, ids: &mut HashSet<String>) {
    ids.insert(instance.instance_id.clone());
    for children in instance.slots.values() {
        for child in children {
            collect_ids_inner(child, ids);
        }
    }
    if let Some(workflow) = &instance.workflow {
        for node in &workflow.nodes {
            collect_ids_inner(&node.component, ids);
        }
    }
}

fn detach_slot_child(
    instance: &mut ComponentInstance,
    instance_id: &str,
) -> Option<ComponentInstance> {
    for children in instance.slots.values_mut() {
        if let Some(index) = children
            .iter()
            .position(|child| child.instance_id == instance_id)
        {
            return Some(children.remove(index));
        }
        for child in children {
            if let Some(found) = detach_slot_child(child, instance_id) {
                return Some(found);
            }
        }
    }
    if let Some(workflow) = &mut instance.workflow {
        for node in &mut workflow.nodes {
            if let Some(found) = detach_slot_child(&mut node.component, instance_id) {
                return Some(found);
            }
        }
    }
    None
}

fn remove_instance(instance: &mut ComponentInstance, instance_id: &str) -> bool {
    for children in instance.slots.values_mut() {
        if let Some(index) = children
            .iter()
            .position(|child| child.instance_id == instance_id)
        {
            children.remove(index);
            return true;
        }
        for child in children {
            if remove_instance(child, instance_id) {
                return true;
            }
        }
    }
    if let Some(workflow) = &mut instance.workflow {
        if let Some(index) = workflow
            .nodes
            .iter()
            .position(|node| node.component.instance_id == instance_id)
        {
            let node_id = workflow.nodes[index].node_id.clone();
            workflow.nodes.remove(index);
            workflow
                .edges
                .retain(|edge| edge.from != node_id && edge.to != node_id);
            return true;
        }
        for node in &mut workflow.nodes {
            if remove_instance(&mut node.component, instance_id) {
                return true;
            }
        }
    }
    false
}

fn has_cycle(edges: &HashMap<String, Vec<String>>) -> bool {
    fn visit(
        node: &str,
        edges: &HashMap<String, Vec<String>>,
        state: &mut HashMap<String, u8>,
    ) -> bool {
        match state.get(node).copied() {
            Some(1) => return true,
            Some(2) => return false,
            _ => {}
        }
        state.insert(node.to_owned(), 1);
        if let Some(next) = edges.get(node) {
            for child in next {
                if visit(child, edges, state) {
                    return true;
                }
            }
        }
        state.insert(node.to_owned(), 2);
        false
    }
    let mut state = HashMap::new();
    edges.keys().any(|node| visit(node, edges, &mut state))
}

fn issue(
    code: impl Into<String>,
    path: impl Into<String>,
    message: impl Into<String>,
) -> SemanticIssue {
    SemanticIssue {
        code: code.into(),
        path: path.into(),
        message: message.into(),
    }
}

fn finish(issues: Vec<SemanticIssue>) -> Result<(), Vec<SemanticIssue>> {
    if issues.is_empty() {
        Ok(())
    } else {
        Err(issues)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::semantic_component::ProtocolScope;

    fn instance(id: &str, definition: &str) -> ComponentInstance {
        ComponentInstance {
            instance_id: id.into(),
            definition: ComponentDefinitionRef {
                id: definition.into(),
                version: 1,
            },
            model_ref: None,
            props: json!({}),
            slots: BTreeMap::new(),
            workflow: None,
            visual: None,
        }
    }

    fn operation(id: &str, block: &str, protocol: &str, step: &str) -> ComponentInstance {
        let mut value = instance(id, "operation");
        value.model_ref = Some(ModelRef::OperationUse {
            block_version_id: block.into(),
            step: QualifiedStepRef {
                protocol_version_id: protocol.into(),
                scope: ProtocolScope::Assay {
                    assay_id: "cck8".into(),
                },
                step_path: step.into(),
            },
        });
        value
    }

    fn fixture() -> ViewDocument {
        let table = instance("concentration_table", "layout.table");
        let plate = instance("plate_layout", "lab.well_plate");
        let dilution = instance("dilution_tool", "tool.dilution");
        let note = instance("protocol_note", "content.note");
        let mut detail = BTreeMap::new();
        detail.insert(
            "detail".into(),
            vec![
                table,
                plate,
                dilution,
                note,
                ComponentInstance {
                    instance_id: "drug_subflow".into(),
                    definition: ComponentDefinitionRef {
                        id: "workflow".into(),
                        version: 1,
                    },
                    model_ref: Some(ModelRef::ProtocolVersion {
                        id: "drug-prep-v1".into(),
                    }),
                    props: json!({}),
                    slots: BTreeMap::new(),
                    workflow: Some(WorkflowGraph {
                        nodes: vec![
                            WorkflowNode {
                                node_id: "prepare".into(),
                                component: Box::new(operation(
                                    "prepare",
                                    "drug-treatment-v1",
                                    "drug-prep-v1",
                                    "root/prepare",
                                )),
                            },
                            WorkflowNode {
                                node_id: "treat".into(),
                                component: Box::new(operation(
                                    "treat",
                                    "drug-treatment-v1",
                                    "drug-prep-v1",
                                    "root/treat",
                                )),
                            },
                            WorkflowNode {
                                node_id: "incubate".into(),
                                component: Box::new(operation(
                                    "incubate",
                                    "drug-treatment-v1",
                                    "drug-prep-v1",
                                    "root/incubate",
                                )),
                            },
                        ],
                        edges: vec![
                            WorkflowEdge {
                                from: "prepare".into(),
                                to: "treat".into(),
                            },
                            WorkflowEdge {
                                from: "treat".into(),
                                to: "incubate".into(),
                            },
                        ],
                    }),
                    visual: None,
                },
            ],
        );
        let mut step = operation(
            "step_drug",
            "drug-treatment-v1",
            "protocol-v1",
            "root/step_drug",
        );
        step.slots = detail;
        let root = ComponentInstance {
            instance_id: "experiment_main".into(),
            definition: ComponentDefinitionRef {
                id: "workflow".into(),
                version: 1,
            },
            model_ref: Some(ModelRef::ProtocolVersion {
                id: "protocol-v1".into(),
            }),
            props: json!({}),
            slots: BTreeMap::new(),
            workflow: Some(WorkflowGraph {
                nodes: vec![WorkflowNode {
                    node_id: "drug".into(),
                    component: Box::new(step),
                }],
                edges: vec![],
            }),
            visual: None,
        };
        ViewDocument {
            schema_version: COMPONENT_FRAMEWORK_SCHEMA_VERSION,
            document_id: "view-1".into(),
            revision: 1,
            root,
            bindings: vec![
                DataBinding {
                    binding_id: "bind-table".into(),
                    source: BindingSource::Model(ModelPath {
                        model: ModelRef::DesignVersion {
                            id: "design-v1".into(),
                        },
                        path: "concentrations".into(),
                    }),
                    target: ComponentPortRef {
                        instance_id: "concentration_table".into(),
                        port_id: "data".into(),
                    },
                    value_type: ComponentValueType::Data,
                },
                DataBinding {
                    binding_id: "bind-dilution".into(),
                    source: BindingSource::ComponentPort(ComponentPortRef {
                        instance_id: "concentration_table".into(),
                        port_id: "values".into(),
                    }),
                    target: ComponentPortRef {
                        instance_id: "dilution_tool".into(),
                        port_id: "targets".into(),
                    },
                    value_type: ComponentValueType::Data,
                },
            ],
        }
    }

    #[test]
    fn catalog_definitions_are_registered_without_core_enum_changes() {
        let registry = default_registry();
        for id in [
            "workflow",
            "operation",
            "layout.section",
            "content.markdown",
            "content.table",
            "lab.well_plate",
            "lab.container_view",
            "lab.rack_layout",
            "tool.calculator",
            "data.reference",
            "visual.chart",
            "media.figure",
        ] {
            assert!(
                registry.get(&ComponentDefinitionRef { id: id.into(), version: 1 }).is_some(),
                "missing definition {id}"
            );
        }
    }

    #[test]
    fn fixture_round_trips_and_resolves_operation_uses() {
        let registry = default_registry();
        let document = fixture();
        document.validate(&registry).unwrap();
        let decoded: ViewDocument =
            serde_json::from_str(&serde_json::to_string(&document).unwrap()).unwrap();
        decoded.validate(&registry).unwrap();

        let operations = find_instance(&decoded.root, "prepare");
        let treat = find_instance(&decoded.root, "treat");
        assert!(operations.is_some() && treat.is_some());
    }

    #[test]
    fn custom_component_extends_registry_without_core_enum_change() {
        let registry = default_registry();
        let mut registry = registry;
        registry
            .register(ComponentDefinition {
                id: "lab.solution_calculator".into(),
                version: 1,
                family: ComponentFamily::Tool,
                title: "Solution calculator".into(),
                props_schema: json!({"type":"object"}),
                capabilities: vec!["calculate".into()],
                inputs: vec![],
                outputs: vec![],
                slots: vec![],
                presentation: None,
                agent_operations: vec!["update".into()],
            })
            .unwrap();
        let document = fixture();
        document
            .apply_changeset(
                &ViewChangeSet {
                    changeset_id: "add-tool".into(),
                    document_id: document.document_id.clone(),
                    expected_revision: 1,
                    operations: vec![ViewPatch::AddSlotChild {
                        parent_instance_id: "step_drug".into(),
                        slot_id: "detail".into(),
                        child: instance("solution_tool", "lab.solution_calculator"),
                    }],
                    provenance: vec!["test".into()],
                },
                &registry,
            )
            .unwrap();
        document.validate(&registry).unwrap();
    }

    #[test]
    fn changeset_is_atomic_and_checks_revision() {
        let registry = default_registry();
        let document = fixture();
        let conflict = ViewChangeSet {
            changeset_id: "bad-rev".into(),
            document_id: document.document_id.clone(),
            expected_revision: 2,
            operations: vec![ViewPatch::UpdateProps {
                instance_id: "protocol_note".into(),
                property: "text".into(),
                value: json!("x"),
            }],
            provenance: vec![],
        };
        assert!(document.apply_changeset(&conflict, &registry).is_err());

        let invalid = ViewChangeSet {
            changeset_id: "atomic".into(),
            document_id: document.document_id.clone(),
            expected_revision: 1,
            operations: vec![
                ViewPatch::UpdateProps {
                    instance_id: "protocol_note".into(),
                    property: "text".into(),
                    value: json!("changed"),
                },
                ViewPatch::MoveSlotChild {
                    instance_id: "missing".into(),
                    target_parent_instance_id: "step_drug".into(),
                    target_slot_id: "detail".into(),
                    target_index: None,
                },
            ],
            provenance: vec![],
        };
        assert!(document.apply_changeset(&invalid, &registry).is_err());
        assert_eq!(document.revision, 1);
    }

    #[test]
    fn move_connect_and_binding_cycles_are_checked() {
        let registry = default_registry();
        let document = fixture();
        let result = document
            .apply_changeset(
                &ViewChangeSet {
                    changeset_id: "move".into(),
                    document_id: document.document_id.clone(),
                    expected_revision: 1,
                    operations: vec![ViewPatch::MoveSlotChild {
                        instance_id: "protocol_note".into(),
                        target_parent_instance_id: "drug_subflow".into(),
                        target_slot_id: "detail".into(),
                        target_index: None,
                    }],
                    provenance: vec![],
                },
                &registry,
            )
            .unwrap();
        assert_eq!(result.document.revision, 2);
        assert!(result.requires_confirmation);

        let mut cycle = fixture();
        cycle.bindings.push(DataBinding {
            binding_id: "cycle".into(),
            source: BindingSource::ComponentPort(ComponentPortRef {
                instance_id: "dilution_tool".into(),
                port_id: "plan".into(),
            }),
            target: ComponentPortRef {
                instance_id: "concentration_table".into(),
                port_id: "data".into(),
            },
            value_type: ComponentValueType::Data,
        });
        assert!(cycle.validate(&registry).is_err());
    }

    #[test]
    fn remove_cleans_bindings_and_definition_versions_are_exact() {
        let registry = default_registry();
        let document = fixture();
        let result = document
            .apply_changeset(
                &ViewChangeSet {
                    changeset_id: "remove".into(),
                    document_id: document.document_id.clone(),
                    expected_revision: 1,
                    operations: vec![ViewPatch::RemoveInstance {
                        instance_id: "concentration_table".into(),
                    }],
                    provenance: vec![],
                },
                &registry,
            )
            .unwrap();
        assert!(!result
            .document
            .bindings
            .iter()
            .any(|binding| binding.binding_id == "bind-table"));
        assert!(find_instance(&result.document.root, "concentration_table").is_none());

        let mut document = fixture();
        document.root.definition.version = 99;
        assert!(document.validate(&registry).is_err());
    }

    #[test]
    fn strict_patch_targets_and_props_boundaries_are_enforced() {
        let registry = default_registry();
        let document = fixture();
        let empty = ViewChangeSet {
            changeset_id: "empty".into(),
            document_id: document.document_id.clone(),
            expected_revision: 1,
            operations: vec![],
            provenance: vec![],
        };
        assert!(document.apply_changeset(&empty, &registry).is_err());

        let missing_binding = ViewChangeSet {
            changeset_id: "missing-binding".into(),
            document_id: document.document_id.clone(),
            expected_revision: 1,
            operations: vec![ViewPatch::RemoveBinding {
                binding_id: "does-not-exist".into(),
            }],
            provenance: vec![],
        };
        assert!(document.apply_changeset(&missing_binding, &registry).is_err());

        let scientific_prop = ViewChangeSet {
            changeset_id: "scientific-prop".into(),
            document_id: document.document_id.clone(),
            expected_revision: 1,
            operations: vec![ViewPatch::UpdateProps {
                instance_id: "step_drug".into(),
                property: "temperature".into(),
                value: json!(37),
            }],
            provenance: vec![],
        };
        assert!(document.apply_changeset(&scientific_prop, &registry).is_err());

        let index = ViewChangeSet {
            changeset_id: "index".into(),
            document_id: document.document_id.clone(),
            expected_revision: 1,
            operations: vec![ViewPatch::MoveSlotChild {
                instance_id: "protocol_note".into(),
                target_parent_instance_id: "drug_subflow".into(),
                target_slot_id: "detail".into(),
                target_index: Some(99),
            }],
            provenance: vec![],
        };
        assert!(document.apply_changeset(&index, &registry).is_err());
    }

    #[test]
    fn contextual_binding_is_separate_from_structural_validation() {
        struct Context;
        impl ComponentBindingContext for Context {
            fn resolve_model_path(&self, _model: &ModelRef, _path: &str) -> ModelPathResolution {
                ModelPathResolution::Resolved(ComponentValueType::Data)
            }
            fn operation_use_exists(&self, _block: &str, _step: &QualifiedStepRef) -> bool {
                true
            }
        }
        let registry = default_registry();
        let document = fixture();
        document.validate_with_context(&registry, &Context).unwrap();

        struct Missing;
        impl ComponentBindingContext for Missing {
            fn resolve_model_path(&self, _model: &ModelRef, _path: &str) -> ModelPathResolution {
                ModelPathResolution::Missing
            }
            fn operation_use_exists(&self, _block: &str, _step: &QualifiedStepRef) -> bool {
                false
            }
        }
        assert!(document.validate_with_context(&registry, &Missing).is_err());
    }
}
