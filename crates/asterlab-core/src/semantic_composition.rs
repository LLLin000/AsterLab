//! S1-B recursive composition validation.
//!
//! This module validates a typed, already-expanded composition tree and its
//! port connections. It does not expand ProtocolBlockVersions, read SQLite, or
//! replace the authoritative ordered `steps` array. Containment is the tree;
//! data dependencies are explicit port connections.

use crate::semantic_component::{ProtocolScope, SampleExpectation, SemanticIssue};
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};

pub const SEMANTIC_COMPOSITION_SCHEMA_VERSION: u32 = 1;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum PortDirection {
    Input,
    Output,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum PortKind {
    Sample,
    Data,
    Parameter,
    Resource,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PortSpec {
    pub id: String,
    pub direction: PortDirection,
    pub kind: PortKind,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub unit: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sample_expectation: Option<SampleExpectation>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ConditionState {
    True,
    False,
    Unknown,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct StepCondition {
    pub condition_id: String,
    pub state: ConditionState,
}

impl StepCondition {
    /// An unknown condition is not false. Callers must not treat it as skipped
    /// or executed until they have an explicit resolution.
    pub fn is_skipped(&self) -> bool {
        self.state == ConditionState::False
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CompositionNode {
    pub block_version_id: String,
    pub step_path: String,
    pub scope: ProtocolScope,
    pub ports: Vec<PortSpec>,
    #[serde(default)]
    pub children: Vec<CompositionNode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub condition: Option<StepCondition>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Hash)]
#[serde(deny_unknown_fields)]
pub struct StepPortRef {
    pub step_path: String,
    pub port_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PortConnection {
    pub from: StepPortRef,
    pub to: StepPortRef,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cross_scope_authorization: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CompositionProtocol {
    pub protocol_version_id: String,
    pub roots: Vec<CompositionNode>,
    #[serde(default)]
    pub connections: Vec<PortConnection>,
}

impl CompositionProtocol {
    pub fn validate(&self) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = Vec::new();
        if self.protocol_version_id.trim().is_empty() {
            issues.push(issue(
                "empty_id",
                "protocol_version_id",
                "Protocol version id must not be empty",
            ));
        }

        let mut ancestors = Vec::new();
        let mut paths = HashSet::new();
        let mut ports = HashMap::new();
        let mut scopes = HashMap::new();
        for root in &self.roots {
            visit_node(
                root,
                &mut ancestors,
                &mut paths,
                &mut ports,
                &mut scopes,
                &mut issues,
            );
        }
        validate_connections(&self.connections, &ports, &scopes, &mut issues);
        expect_no_issues(issues)
    }
}

fn visit_node(
    node: &CompositionNode,
    ancestors: &mut Vec<String>,
    paths: &mut HashSet<String>,
    ports: &mut HashMap<(String, String), PortSpec>,
    scopes: &mut HashMap<String, ProtocolScope>,
    issues: &mut Vec<SemanticIssue>,
) {
    if node.block_version_id.trim().is_empty() {
        issues.push(issue(
            "empty_block",
            format!("{}.block_version_id", node.step_path),
            "block version id must not be empty",
        ));
    }
    if node.step_path.trim().is_empty() {
        issues.push(issue(
            "empty_step_path",
            "step_path",
            "step path must not be empty",
        ));
    }
    if ancestors.contains(&node.block_version_id) {
        issues.push(issue(
            "composition_cycle",
            &node.step_path,
            "a block version must not contain itself recursively",
        ));
    }
    if !paths.insert(node.step_path.clone()) {
        issues.push(issue(
            "duplicate_step_path",
            &node.step_path,
            "step paths must be unique within a Protocol",
        ));
    }
    if scopes
        .insert(node.step_path.clone(), node.scope.clone())
        .is_some()
    {
        // Duplicate paths are already reported.
    }

    let mut local_ports = HashSet::new();
    for port in &node.ports {
        if port.id.trim().is_empty() {
            issues.push(issue(
                "empty_port",
                format!("{}.ports", node.step_path),
                "port id must not be empty",
            ));
        }
        if !local_ports.insert(port.id.clone()) {
            issues.push(issue(
                "duplicate_port",
                format!("{}.ports.{}", node.step_path, port.id),
                "port ids must be unique within a step",
            ));
        }
        if ports
            .insert((node.step_path.clone(), port.id.clone()), port.clone())
            .is_some()
        {
            issues.push(issue(
                "duplicate_port",
                format!("{}.ports.{}", node.step_path, port.id),
                "port address must be unique",
            ));
        }
    }

    if let Some(condition) = &node.condition {
        if condition.condition_id.trim().is_empty() {
            issues.push(issue(
                "empty_condition",
                format!("{}.condition", node.step_path),
                "condition id must not be empty",
            ));
        }
        if condition.state == ConditionState::Unknown {
            // Keep it unknown. Do not convert it to false or skipped.
        }
    }

    ancestors.push(node.block_version_id.clone());
    for child in &node.children {
        visit_node(child, ancestors, paths, ports, scopes, issues);
    }
    ancestors.pop();
}

fn validate_connections(
    connections: &[PortConnection],
    ports: &HashMap<(String, String), PortSpec>,
    scopes: &HashMap<String, ProtocolScope>,
    issues: &mut Vec<SemanticIssue>,
) {
    let mut incoming = HashSet::new();
    let mut data_edges: HashMap<String, Vec<String>> = HashMap::new();
    for connection in connections {
        let from = ports.get(&(
            connection.from.step_path.clone(),
            connection.from.port_id.clone(),
        ));
        let to = ports.get(&(
            connection.to.step_path.clone(),
            connection.to.port_id.clone(),
        ));
        let (Some(from), Some(to)) = (from, to) else {
            issues.push(issue(
                "missing_port",
                format!(
                    "{}:{} -> {}:{}",
                    connection.from.step_path,
                    connection.from.port_id,
                    connection.to.step_path,
                    connection.to.port_id
                ),
                "connection references a missing port",
            ));
            continue;
        };
        if from.direction != PortDirection::Output {
            issues.push(issue(
                "port_direction",
                format!("{}.{}", connection.from.step_path, connection.from.port_id),
                "connection source must be an output port",
            ));
        }
        if to.direction != PortDirection::Input {
            issues.push(issue(
                "port_direction",
                format!("{}.{}", connection.to.step_path, connection.to.port_id),
                "connection target must be an input port",
            ));
        }
        if from.kind != to.kind {
            issues.push(issue(
                "port_kind",
                format!(
                    "{}:{} -> {}:{}",
                    connection.from.step_path,
                    connection.from.port_id,
                    connection.to.step_path,
                    connection.to.port_id
                ),
                "connected ports must have the same object kind",
            ));
        }
        if from.unit.is_some() && to.unit.is_some() && from.unit != to.unit {
            issues.push(issue(
                "port_unit",
                format!("{}.{}", connection.from.step_path, connection.from.port_id),
                "connected port units must match exactly",
            ));
        }
        if from.kind == PortKind::Sample
            && from.sample_expectation.is_some()
            && to.sample_expectation.is_some()
            && from.sample_expectation != to.sample_expectation
        {
            issues.push(issue(
                "sample_expectation",
                format!("{}.{}", connection.to.step_path, connection.to.port_id),
                "sample expectations must match at the port boundary",
            ));
        }
        if connection.from.step_path == connection.to.step_path {
            issues.push(issue(
                "self_connection",
                format!("{}.{}", connection.from.step_path, connection.from.port_id),
                "a step cannot connect to itself",
            ));
        }

        let target_key = format!("{}::{}", connection.to.step_path, connection.to.port_id);
        if !incoming.insert(target_key) {
            issues.push(issue(
                "multiple_input_connections",
                format!("{}.{}", connection.to.step_path, connection.to.port_id),
                "an input port accepts only one connection",
            ));
        }

        let from_scope = scopes.get(&connection.from.step_path);
        let to_scope = scopes.get(&connection.to.step_path);
        if let (Some(from_scope), Some(to_scope)) = (from_scope, to_scope) {
            if from_scope != to_scope
                && connection
                    .cross_scope_authorization
                    .as_deref()
                    .map(str::trim)
                    .is_none_or(str::is_empty)
            {
                issues.push(issue(
                    "cross_scope_unauthorized",
                    format!("{}.{}", connection.from.step_path, connection.from.port_id),
                    "cross-scope connection requires explicit authorization",
                ));
            }
        }

        if from.kind == PortKind::Data && to.kind == PortKind::Data {
            data_edges
                .entry(connection.from.step_path.clone())
                .or_default()
                .push(connection.to.step_path.clone());
        }
    }

    if has_data_cycle(&data_edges) {
        issues.push(issue(
            "data_dependency_cycle",
            "connections",
            "pure data dependencies must not form a cycle",
        ));
    }
}

fn has_data_cycle(edges: &HashMap<String, Vec<String>>) -> bool {
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

fn expect_no_issues(issues: Vec<SemanticIssue>) -> Result<(), Vec<SemanticIssue>> {
    if issues.is_empty() {
        Ok(())
    } else {
        Err(issues)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::semantic_component::{
        BindingOwner, BindingResolution, BindingSource, BindingTarget, ParameterTarget,
        PlannedSampleRef, SampleExpectation, SampleTarget, TypedBinding,
    };
    use serde_json::json;

    fn scope(assay: &str) -> ProtocolScope {
        ProtocolScope::Assay {
            assay_id: assay.into(),
        }
    }

    fn port(id: &str, direction: PortDirection, kind: PortKind) -> PortSpec {
        PortSpec {
            id: id.into(),
            direction,
            kind,
            unit: None,
            sample_expectation: None,
        }
    }

    fn node(
        block: &str,
        path: &str,
        assay: &str,
        ports: Vec<PortSpec>,
        children: Vec<CompositionNode>,
    ) -> CompositionNode {
        CompositionNode {
            block_version_id: block.into(),
            step_path: path.into(),
            scope: scope(assay),
            ports,
            children,
            condition: None,
        }
    }

    fn protocol(
        roots: Vec<CompositionNode>,
        connections: Vec<PortConnection>,
    ) -> CompositionProtocol {
        CompositionProtocol {
            protocol_version_id: "protocol-v1".into(),
            roots,
            connections,
        }
    }

    fn connection(from: (&str, &str), to: (&str, &str)) -> PortConnection {
        PortConnection {
            from: StepPortRef {
                step_path: from.0.into(),
                port_id: from.1.into(),
            },
            to: StepPortRef {
                step_path: to.0.into(),
                port_id: to.1.into(),
            },
            cross_scope_authorization: None,
        }
    }

    #[test]
    fn rejects_recursive_containment() {
        let child = node(
            "module-a",
            "root/a",
            "assay-1",
            vec![],
            vec![node(
                "module-b",
                "root/a/b",
                "assay-1",
                vec![],
                vec![node("module-a", "root/a/b/a", "assay-1", vec![], vec![])],
            )],
        );
        let errors = protocol(vec![child], vec![]).validate().unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "composition_cycle"));
    }

    #[test]
    fn allows_same_module_under_different_parents() {
        let roots = vec![
            node("module-m", "cck8/transfer", "cck8", vec![], vec![]),
            node("module-m", "elisa/transfer", "elisa", vec![], vec![]),
        ];
        assert!(protocol(roots, vec![]).validate().is_ok());
    }

    #[test]
    fn rejects_pure_data_dependency_cycle() {
        let roots = vec![
            node(
                "reader",
                "root/read-a",
                "assay-1",
                vec![
                    port("in", PortDirection::Input, PortKind::Data),
                    port("out", PortDirection::Output, PortKind::Data),
                ],
                vec![],
            ),
            node(
                "reader",
                "root/read-b",
                "assay-1",
                vec![
                    port("in", PortDirection::Input, PortKind::Data),
                    port("out", PortDirection::Output, PortKind::Data),
                ],
                vec![],
            ),
        ];
        let connections = vec![
            connection(("root/read-a", "out"), ("root/read-b", "in")),
            connection(("root/read-b", "out"), ("root/read-a", "in")),
        ];
        let errors = protocol(roots, connections).validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "data_dependency_cycle"));
    }

    #[test]
    fn sample_connections_do_not_create_data_cycle() {
        let roots = vec![
            node(
                "sample-a",
                "root/a",
                "assay-1",
                vec![
                    port("in", PortDirection::Input, PortKind::Sample),
                    port("out", PortDirection::Output, PortKind::Sample),
                ],
                vec![],
            ),
            node(
                "sample-b",
                "root/b",
                "assay-1",
                vec![
                    port("in", PortDirection::Input, PortKind::Sample),
                    port("out", PortDirection::Output, PortKind::Sample),
                ],
                vec![],
            ),
        ];
        let connections = vec![
            connection(("root/a", "out"), ("root/b", "in")),
            connection(("root/b", "out"), ("root/a", "in")),
        ];
        assert!(protocol(roots, connections).validate().is_ok());
    }

    #[test]
    fn port_unit_and_kind_mismatches_are_rejected() {
        let roots = vec![
            node(
                "source",
                "root/source",
                "assay-1",
                vec![PortSpec {
                    id: "out".into(),
                    direction: PortDirection::Output,
                    kind: PortKind::Data,
                    unit: Some("mL".into()),
                    sample_expectation: None,
                }],
                vec![],
            ),
            node(
                "sink",
                "root/sink",
                "assay-1",
                vec![PortSpec {
                    id: "in".into(),
                    direction: PortDirection::Input,
                    kind: PortKind::Data,
                    unit: Some("uL".into()),
                    sample_expectation: None,
                }],
                vec![],
            ),
        ];
        let errors = protocol(
            roots,
            vec![connection(("root/source", "out"), ("root/sink", "in"))],
        )
        .validate()
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "port_unit"));
    }

    #[test]
    fn port_kind_mismatch_is_rejected() {
        let roots = vec![
            node(
                "source",
                "root/source",
                "assay-1",
                vec![port("out", PortDirection::Output, PortKind::Data)],
                vec![],
            ),
            node(
                "sink",
                "root/sink",
                "assay-1",
                vec![port("in", PortDirection::Input, PortKind::Sample)],
                vec![],
            ),
        ];
        let errors = protocol(
            roots,
            vec![connection(("root/source", "out"), ("root/sink", "in"))],
        )
        .validate()
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "port_kind"));
    }

    #[test]
    fn cross_scope_connection_requires_authorization() {
        let roots = vec![
            node(
                "producer",
                "cck8/produce",
                "cck8",
                vec![port("out", PortDirection::Output, PortKind::Data)],
                vec![],
            ),
            node(
                "consumer",
                "elisa/consume",
                "elisa",
                vec![port("in", PortDirection::Input, PortKind::Data)],
                vec![],
            ),
        ];
        let mut connection = connection(("cck8/produce", "out"), ("elisa/consume", "in"));
        let errors = protocol(roots.clone(), vec![connection.clone()])
            .validate()
            .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "cross_scope_unauthorized"));
        connection.cross_scope_authorization = Some("shared-material-1".into());
        assert!(protocol(roots, vec![connection]).validate().is_ok());
    }

    #[test]
    fn unknown_condition_is_not_skipped() {
        let mut root = node("module-a", "root/a", "assay-1", vec![], vec![]);
        root.condition = Some(StepCondition {
            condition_id: "if-response".into(),
            state: ConditionState::Unknown,
        });
        assert!(!root.condition.as_ref().unwrap().is_skipped());
        assert!(protocol(vec![root], vec![]).validate().is_ok());
    }

    #[test]
    fn same_liquid_transfer_module_has_independent_paths_and_bindings() {
        let roots = vec![
            node(
                "op.liquid_transfer",
                "cck8/transfer",
                "cck8",
                vec![port("sample_in", PortDirection::Input, PortKind::Sample)],
                vec![],
            ),
            node(
                "op.liquid_transfer",
                "elisa/transfer",
                "elisa",
                vec![port("sample_in", PortDirection::Input, PortKind::Sample)],
                vec![],
            ),
            node(
                "op.liquid_transfer",
                "cfu/transfer",
                "cfu",
                vec![port("sample_in", PortDirection::Input, PortKind::Sample)],
                vec![],
            ),
        ];
        assert!(protocol(roots, vec![]).validate().is_ok());

        let bindings = ["cck8", "elisa", "cfu"]
            .into_iter()
            .map(|method| {
                let sample = TypedBinding {
                    binding_id: format!("{method}:sample"),
                    owner: BindingOwner::ProtocolPlan {
                        protocol_version_id: "protocol-v1".into(),
                    },
                    target: BindingTarget::Sample(SampleTarget {
                        block_version_id: "op.liquid_transfer".into(),
                        step: crate::semantic_component::QualifiedStepRef {
                            protocol_version_id: "protocol-v1".into(),
                            scope: scope(method),
                            step_path: format!("{method}/transfer"),
                        },
                        port_id: "sample_in".into(),
                        expectation: SampleExpectation::Planned,
                    }),
                    source: BindingSource::PlannedSample {
                        reference: PlannedSampleRef {
                            design_version_id: "design-v1".into(),
                            sample_key: format!("{method}-sample"),
                        },
                    },
                    unit: None,
                    resolution: BindingResolution::Resolved,
                    confirmed: true,
                    provenance: vec![],
                };
                let volume = TypedBinding {
                    binding_id: format!("{method}:volume"),
                    owner: BindingOwner::ProtocolPlan {
                        protocol_version_id: "protocol-v1".into(),
                    },
                    target: BindingTarget::Parameter(ParameterTarget {
                        block_version_id: "op.liquid_transfer".into(),
                        step: crate::semantic_component::QualifiedStepRef {
                            protocol_version_id: "protocol-v1".into(),
                            scope: scope(method),
                            step_path: format!("{method}/transfer"),
                        },
                        parameter_key: "volume".into(),
                        expected_type: crate::operation_module::ParameterValueType::Number,
                        expected_unit: Some("uL".into()),
                    }),
                    source: BindingSource::Literal { value: json!(10.0) },
                    unit: Some("uL".into()),
                    resolution: BindingResolution::Resolved,
                    confirmed: true,
                    provenance: vec![],
                };
                (sample, volume)
            })
            .collect::<Vec<_>>();

        let mut target_keys = HashSet::new();
        let mut sample_keys = HashSet::new();
        for (sample, volume) in bindings {
            sample.validate().unwrap();
            volume.validate().unwrap();
            let BindingTarget::Sample(target) = sample.target else {
                panic!("sample target");
            };
            target_keys.insert(target.step.target_key());
            let BindingSource::PlannedSample { reference } = sample.source else {
                panic!("planned sample");
            };
            sample_keys.insert(reference.sample_key);
        }
        assert_eq!(target_keys.len(), 3);
        assert_eq!(sample_keys.len(), 3);
    }
}
