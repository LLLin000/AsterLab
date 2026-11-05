//! C0/C1 headless unified component framework.
//!
//! This is a presentation/composition layer over the existing scientific core.
//! It does not create a second Design/Protocol/Run model or a second database.
//! The document is a versioned, serializable view tree plus a workflow graph.

use crate::semantic_component::SemanticIssue;
use crate::semantic_tool::validate_schema_value;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, HashMap, HashSet};

pub const COMPONENT_FRAMEWORK_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(rename_all = "snake_case")]
pub enum ComponentKind {
    Workflow,
    Operation,
    LayoutTable,
    WellPlate,
    ToolDilution,
    ContentNote,
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
    pub accepts: Vec<ComponentKind>,
    pub cardinality: SlotCardinality,
    #[serde(default)]
    pub required: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ComponentDefinition {
    pub id: String,
    pub version: u32,
    pub kind: ComponentKind,
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

impl ComponentDefinitionRef {
    pub fn new(id: impl Into<String>, version: u32) -> Self {
        Self {
            id: id.into(),
            version,
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ModelKind {
    DesignVersion,
    ProtocolVersion,
    BlockVersion,
    Run,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct ModelRef {
    pub kind: ModelKind,
    pub id: String,
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
    RemoveInstance {
        instance_id: String,
    },
    Connect {
        workflow_instance_id: String,
        edge: WorkflowEdge,
    },
    AddBinding {
        binding: DataBinding,
    },
}

#[derive(Debug, Clone, Default)]
pub struct ComponentRegistry {
    definitions: HashMap<(String, u32), ComponentDefinition>,
}

impl ComponentRegistry {
    pub fn register(&mut self, definition: ComponentDefinition) {
        self.definitions
            .insert((definition.id.clone(), definition.version), definition);
    }

    pub fn get(&self, reference: &ComponentDefinitionRef) -> Option<&ComponentDefinition> {
        self.definitions
            .get(&(reference.id.clone(), reference.version))
    }

    pub fn definitions(&self) -> impl Iterator<Item = &ComponentDefinition> {
        self.definitions.values()
    }
}

pub fn default_registry() -> ComponentRegistry {
    let mut registry = ComponentRegistry::default();
    registry.register(ComponentDefinition {
        id: "workflow".into(),
        version: 1,
        kind: ComponentKind::Workflow,
        title: "Workflow".into(),
        props_schema: json_object(),
        capabilities: vec!["compose".into(), "navigate".into()],
        inputs: vec![],
        outputs: vec![],
        slots: vec![SlotDefinition {
            id: "detail".into(),
            accepts: vec![
                ComponentKind::LayoutTable,
                ComponentKind::WellPlate,
                ComponentKind::ToolDilution,
                ComponentKind::ContentNote,
                ComponentKind::Workflow,
                ComponentKind::Operation,
            ],
            cardinality: SlotCardinality::Many,
            required: false,
        }],
        presentation: Some("workflow.graph".into()),
        agent_operations: vec!["update".into(), "add_node".into(), "connect".into()],
    });
    registry.register(ComponentDefinition {
        id: "operation".into(),
        version: 1,
        kind: ComponentKind::Operation,
        title: "Operation".into(),
        props_schema: json_object(),
        capabilities: vec!["bind".into()],
        inputs: vec![],
        outputs: vec![],
        slots: vec![SlotDefinition {
            id: "detail".into(),
            accepts: vec![
                ComponentKind::LayoutTable,
                ComponentKind::WellPlate,
                ComponentKind::ToolDilution,
                ComponentKind::ContentNote,
                ComponentKind::Workflow,
            ],
            cardinality: SlotCardinality::Many,
            required: false,
        }],
        presentation: Some("operation.node".into()),
        agent_operations: vec!["update".into()],
    });
    registry.register(ComponentDefinition {
        id: "layout.table".into(),
        version: 1,
        kind: ComponentKind::LayoutTable,
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
    });
    registry.register(ComponentDefinition {
        id: "lab.well_plate".into(),
        version: 1,
        kind: ComponentKind::WellPlate,
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
    });
    registry.register(ComponentDefinition {
        id: "tool.dilution".into(),
        version: 1,
        kind: ComponentKind::ToolDilution,
        title: "Dilution calculator".into(),
        props_schema: json_object(),
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
    });
    registry.register(ComponentDefinition {
        id: "content.note".into(),
        version: 1,
        kind: ComponentKind::ContentNote,
        title: "Note".into(),
        props_schema: json!({"type":"object","properties":{"text":{"type":"string"}}}),
        capabilities: vec!["content".into()],
        inputs: vec![],
        outputs: vec![],
        slots: vec![],
        presentation: Some("content.note".into()),
        agent_operations: vec!["update".into()],
    });
    registry
}

fn json_object() -> Value {
    json!({"type":"object"})
}

impl ViewDocument {
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
        let mut ids = HashSet::new();
        let mut port_owner: HashMap<String, ComponentDefinitionRef> = HashMap::new();
        validate_instance(&self.root, registry, &mut ids, &mut port_owner, &mut issues);

        let mut binding_ids = HashSet::new();
        let mut targets = HashSet::new();
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
            let target_definition = port_owner.get(&binding.target.instance_id);
            let target_port = target_definition.and_then(|reference| {
                registry.get(reference).and_then(|definition| {
                    definition
                        .inputs
                        .iter()
                        .find(|port| port.id == binding.target.port_id)
                })
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
                    if path.model.id.trim().is_empty() || path.path.trim().is_empty() {
                        issues.push(issue(
                            "empty_model_path",
                            &binding.binding_id,
                            "model reference and path must not be empty",
                        ));
                    }
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
                    let source_definition = port_owner.get(&source.instance_id);
                    let source_port = source_definition.and_then(|reference| {
                        registry.get(reference).and_then(|definition| {
                            definition
                                .outputs
                                .iter()
                                .find(|port| port.id == source.port_id)
                        })
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
                }
            }
        }
        finish(issues)
    }

    pub fn apply(
        &self,
        patch: &ViewPatch,
        registry: &ComponentRegistry,
    ) -> Result<ViewDocument, Vec<SemanticIssue>> {
        self.validate(registry)?;
        let mut next = self.clone();
        apply_patch(&mut next, patch)?;
        next.validate(registry)?;
        Ok(next)
    }
}

fn validate_instance(
    instance: &ComponentInstance,
    registry: &ComponentRegistry,
    ids: &mut HashSet<String>,
    port_owner: &mut HashMap<String, ComponentDefinitionRef>,
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
    port_owner.insert(instance.instance_id.clone(), instance.definition.clone());
    let definition = registry.get(&instance.definition);
    let Some(definition) = definition else {
        issues.push(issue(
            "missing_definition",
            &instance.instance_id,
            "component definition is not registered at the exact version",
        ));
        return;
    };
    if !instance.props.is_object() && !instance.props.is_null() {
        issues.push(issue(
            "props_type",
            &instance.instance_id,
            "component props must be an object",
        ));
    }
    if instance.props.is_object() {
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

    match definition.kind {
        ComponentKind::Workflow => {
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
                "slot is not declared by the component definition",
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
                if !slot.accepts.contains(&child_definition.kind) {
                    issues.push(issue(
                        "slot_kind",
                        format!("{}.slots.{slot_id}", instance.instance_id),
                        "child component kind is not accepted by this slot",
                    ));
                }
            }
            validate_instance(child, registry, ids, port_owner, issues);
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
            validate_instance(&node.component, registry, ids, port_owner, issues);
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

fn apply_patch(document: &mut ViewDocument, patch: &ViewPatch) -> Result<(), Vec<SemanticIssue>> {
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
            Ok(())
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
            Ok(())
        }
        ViewPatch::RemoveInstance { instance_id } => {
            if document.root.instance_id == *instance_id {
                return Err(vec![issue(
                    "remove_root",
                    instance_id,
                    "root instance cannot be removed",
                )]);
            }
            remove_instance(&mut document.root, instance_id);
            document
                .bindings
                .retain(|binding| binding.target.instance_id != *instance_id);
            Ok(())
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
            workflow.edges.push(edge.clone());
            Ok(())
        }
        ViewPatch::AddBinding { binding } => {
            document.bindings.push(binding.clone());
            Ok(())
        }
    }
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
            workflow.nodes.remove(index);
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

use serde_json::json;

#[cfg(test)]
mod tests {
    use super::*;

    fn instance(
        id: &str,
        definition: &str,
        props: Value,
        slots: BTreeMap<String, Vec<ComponentInstance>>,
        workflow: Option<WorkflowGraph>,
    ) -> ComponentInstance {
        ComponentInstance {
            instance_id: id.into(),
            definition: ComponentDefinitionRef::new(definition, 1),
            model_ref: None,
            props,
            slots,
            workflow,
            visual: None,
        }
    }

    fn empty_slots() -> BTreeMap<String, Vec<ComponentInstance>> {
        BTreeMap::new()
    }

    fn fixture() -> ViewDocument {
        let detail = BTreeMap::from([(
            "detail".into(),
            vec![
                instance(
                    "concentration_table",
                    "layout.table",
                    json!({"rows": []}),
                    empty_slots(),
                    None,
                ),
                instance(
                    "plate_layout",
                    "lab.well_plate",
                    json!({"format": "96"}),
                    empty_slots(),
                    None,
                ),
                instance(
                    "dilution_tool",
                    "tool.dilution",
                    json!({}),
                    empty_slots(),
                    None,
                ),
                instance(
                    "protocol_note",
                    "content.note",
                    json!({"text": "note"}),
                    empty_slots(),
                    None,
                ),
            ],
        )]);
        let operation = instance("step_drug", "operation", json!({}), detail, None);
        let root = instance(
            "experiment_main",
            "workflow",
            json!({}),
            BTreeMap::from([(
                "detail".into(),
                vec![instance(
                    "drug_subflow",
                    "workflow",
                    json!({}),
                    empty_slots(),
                    Some(WorkflowGraph {
                        nodes: vec![],
                        edges: vec![],
                    }),
                )],
            )]),
            Some(WorkflowGraph {
                nodes: vec![WorkflowNode {
                    node_id: "drug".into(),
                    component: Box::new(operation),
                }],
                edges: vec![],
            }),
        );
        ViewDocument {
            schema_version: COMPONENT_FRAMEWORK_SCHEMA_VERSION,
            document_id: "view-1".into(),
            root,
            bindings: vec![
                DataBinding {
                    binding_id: "bind-table".into(),
                    source: BindingSource::Model(ModelPath {
                        model: ModelRef {
                            kind: ModelKind::DesignVersion,
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
    fn fixture_validates_and_round_trips() {
        let registry = default_registry();
        let document = fixture();
        document.validate(&registry).unwrap();
        let encoded = serde_json::to_string(&document).unwrap();
        let decoded: ViewDocument = serde_json::from_str(&encoded).unwrap();
        assert_eq!(decoded, document);
        decoded.validate(&registry).unwrap();
    }

    #[test]
    fn patch_updates_props_and_adds_content_note() {
        let registry = default_registry();
        let document = fixture();
        let patched = document
            .apply(
                &ViewPatch::UpdateProps {
                    instance_id: "protocol_note".into(),
                    property: "text".into(),
                    value: json!("updated"),
                },
                &registry,
            )
            .unwrap();
        let mut patched_root = patched.root.clone();
        let note = find_instance_mut(&mut patched_root, "protocol_note").unwrap();
        assert_eq!(note.props["text"], "updated");

        let added = document
            .apply(
                &ViewPatch::AddSlotChild {
                    parent_instance_id: "step_drug".into(),
                    slot_id: "detail".into(),
                    child: instance(
                        "new_note",
                        "content.note",
                        json!({"text": "new"}),
                        empty_slots(),
                        None,
                    ),
                },
                &registry,
            )
            .unwrap();
        added.validate(&registry).unwrap();
        let mut added_root = added.root.clone();
        assert!(find_instance_mut(&mut added_root, "new_note").is_some());
    }

    #[test]
    fn duplicate_instance_and_workflow_cycle_are_rejected() {
        let registry = default_registry();
        let mut document = fixture();
        document.bindings.push(DataBinding {
            binding_id: "bad".into(),
            source: BindingSource::Model(ModelPath {
                model: ModelRef {
                    kind: ModelKind::DesignVersion,
                    id: "d".into(),
                },
                path: "x".into(),
            }),
            target: ComponentPortRef {
                instance_id: "missing".into(),
                port_id: "data".into(),
            },
            value_type: ComponentValueType::Data,
        });
        assert!(document.validate(&registry).is_err());

        let mut cyclic = fixture();
        if let Some(workflow) = &mut cyclic.root.workflow {
            workflow.edges = vec![WorkflowEdge {
                from: "drug".into(),
                to: "drug".into(),
            }];
        }
        assert!(cyclic.validate(&registry).is_err());
    }

    #[test]
    fn slot_and_exact_definition_are_enforced() {
        let registry = default_registry();
        let mut document = fixture();
        document.root.workflow.as_mut().unwrap().nodes[0]
            .component
            .slots
            .insert(
                "detail".into(),
                vec![instance(
                    "wrong_kind",
                    "operation",
                    json!({}),
                    empty_slots(),
                    None,
                )],
            );
        assert!(document.validate(&registry).is_err());
        document.root.slots.clear();
        document.root.definition.version = 99;
        assert!(document.validate(&registry).is_err());
    }
}
