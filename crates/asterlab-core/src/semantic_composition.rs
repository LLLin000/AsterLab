//! S1-B/B.1 typed composition validation.
//!
//! This module validates an already-expanded composition tree and its port
//! connections. It does not expand ProtocolBlockVersions, read SQLite, or
//! replace the authoritative ordered `steps` array. Step identity is always
//! Protocol + scope + path, never a bare path.

use crate::semantic_component::{ProtocolScope, SampleExpectation, SemanticIssue};
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};

pub const SEMANTIC_COMPOSITION_SCHEMA_VERSION: u32 = 2;

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
pub enum PortCompatibility {
    Compatible,
    Incompatible,
    Unknown,
}

pub fn port_compatibility(from: &PortSpec, to: &PortSpec) -> PortCompatibility {
    if from.direction != PortDirection::Output || to.direction != PortDirection::Input {
        return PortCompatibility::Incompatible;
    }
    if from.kind != to.kind {
        return PortCompatibility::Incompatible;
    }
    match (&from.unit, &to.unit) {
        (Some(from), Some(to)) if from != to => return PortCompatibility::Incompatible,
        (Some(_), None) | (None, Some(_)) => return PortCompatibility::Unknown,
        _ => {}
    }
    if from.kind == PortKind::Sample {
        return match (&from.sample_expectation, &to.sample_expectation) {
            (Some(from), Some(to)) if from == to => PortCompatibility::Compatible,
            (Some(_), Some(_)) => PortCompatibility::Incompatible,
            _ => PortCompatibility::Unknown,
        };
    }
    PortCompatibility::Compatible
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct AuthorizationRef {
    pub id: String,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum AuthorizationStatus {
    Authorized,
    Denied,
    Missing,
    Unknown,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct IterationRef {
    pub id: String,
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
    pub scope: ProtocolScope,
    pub step_path: String,
    pub port_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(deny_unknown_fields)]
pub struct PortConnection {
    pub from: StepPortRef,
    pub to: StepPortRef,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub cross_scope_authorization: Option<AuthorizationRef>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub iteration: Option<IterationRef>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CompositionProtocol {
    pub protocol_version_id: String,
    pub roots: Vec<CompositionNode>,
    #[serde(default)]
    pub connections: Vec<PortConnection>,
}

pub trait CompositionContext {
    /// Return the authoritative port declared by the OperationModule/Block
    /// version. `None` means the context cannot prove the port contract.
    fn authoritative_port(
        &self,
        protocol_version_id: &str,
        scope: &ProtocolScope,
        step_path: &str,
        port_id: &str,
    ) -> Option<PortSpec>;
    fn authorization_status(&self, authorization: &AuthorizationRef) -> AuthorizationStatus;
}

impl CompositionProtocol {
    /// Structural validation only. It cannot verify cross-scope authorization
    /// references or authoritative OperationModule port contracts.
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
        let mut order = HashMap::new();
        let mut next_order = 0usize;
        for root in &self.roots {
            visit_node(
                root,
                &self.protocol_version_id,
                &mut ancestors,
                &mut paths,
                &mut ports,
                &mut scopes,
                &mut order,
                &mut next_order,
                &mut issues,
            );
        }
        validate_connections(
            &self.protocol_version_id,
            &self.connections,
            &ports,
            &scopes,
            &order,
            &mut issues,
            None,
        );
        expect_no_issues(issues)
    }

    /// Structural checks plus authoritative port and authorization context.
    /// Missing or unprovable context is surfaced as Unknown, not Compatible.
    pub fn validate_with_context(
        &self,
        context: &dyn CompositionContext,
    ) -> Result<(), Vec<SemanticIssue>> {
        let mut issues = self.validate().err().unwrap_or_default();

        let mut paths = HashSet::new();
        let mut ports = HashMap::new();
        let mut scopes = HashMap::new();
        let mut order = HashMap::new();
        let mut ancestors = Vec::new();
        let mut next_order = 0usize;
        for root in &self.roots {
            visit_node(
                root,
                &self.protocol_version_id,
                &mut ancestors,
                &mut paths,
                &mut ports,
                &mut scopes,
                &mut order,
                &mut next_order,
                &mut issues,
            );
        }
        validate_connections(
            &self.protocol_version_id,
            &self.connections,
            &ports,
            &scopes,
            &order,
            &mut issues,
            Some(context),
        );
        expect_no_issues(issues)
    }
}

fn visit_node(
    node: &CompositionNode,
    protocol_version_id: &str,
    ancestors: &mut Vec<String>,
    paths: &mut HashSet<String>,
    ports: &mut HashMap<String, PortSpec>,
    scopes: &mut HashMap<String, ProtocolScope>,
    order: &mut HashMap<String, usize>,
    next_order: &mut usize,
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

    let identity = identity_key(protocol_version_id, &node.scope, &node.step_path);
    if !paths.insert(identity.clone()) {
        issues.push(issue(
            "duplicate_step_path",
            &node.step_path,
            "step identity (Protocol + scope + path) must be unique",
        ));
    }
    scopes.insert(identity.clone(), node.scope.clone());
    order.insert(identity.clone(), *next_order);
    *next_order += 1;

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
        let key = port_key(&node.scope, &node.step_path, &port.id);
        if ports.insert(key, port.clone()).is_some() {
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
        // Unknown remains unknown; it is not converted to false or skipped.
    }

    ancestors.push(node.block_version_id.clone());
    for child in &node.children {
        visit_node(
            child,
            protocol_version_id,
            ancestors,
            paths,
            ports,
            scopes,
            order,
            next_order,
            issues,
        );
    }
    ancestors.pop();
}

fn validate_connections(
    protocol_version_id: &str,
    connections: &[PortConnection],
    ports: &HashMap<String, PortSpec>,
    scopes: &HashMap<String, ProtocolScope>,
    order: &HashMap<String, usize>,
    issues: &mut Vec<SemanticIssue>,
    context: Option<&dyn CompositionContext>,
) {
    let mut incoming = HashSet::new();
    let mut data_edges: HashMap<String, Vec<String>> = HashMap::new();
    for connection in connections {
        let from_key = port_key(
            &connection.from.scope,
            &connection.from.step_path,
            &connection.from.port_id,
        );
        let to_key = port_key(
            &connection.to.scope,
            &connection.to.step_path,
            &connection.to.port_id,
        );
        let from = ports.get(&from_key);
        let to = ports.get(&to_key);
        let (Some(from), Some(to)) = (from, to) else {
            issues.push(issue(
                "missing_port",
                format!("{from_key} -> {to_key}"),
                "connection references a missing port",
            ));
            continue;
        };

        match port_compatibility(from, to) {
            PortCompatibility::Compatible => {}
            PortCompatibility::Incompatible => issues.push(issue(
                "port_incompatible",
                format!("{from_key} -> {to_key}"),
                "port direction, kind, unit or sample expectation is incompatible",
            )),
            PortCompatibility::Unknown => issues.push(issue(
                "port_unknown",
                format!("{from_key} -> {to_key}"),
                "port compatibility cannot be proven from the declared attributes",
            )),
        }

        if connection.from.step_path == connection.to.step_path
            && connection.from.scope == connection.to.scope
        {
            issues.push(issue(
                "self_connection",
                format!("{from_key}"),
                "a step cannot connect to itself",
            ));
        }

        if !incoming.insert(to_key.clone()) {
            issues.push(issue(
                "multiple_input_connections",
                format!("{to_key}"),
                "an input port accepts only one connection",
            ));
        }

        let from_scope = scopes.get(&identity_key(
            protocol_version_id,
            &connection.from.scope,
            &connection.from.step_path,
        ));
        let to_scope = scopes.get(&identity_key(
            protocol_version_id,
            &connection.to.scope,
            &connection.to.step_path,
        ));
        if from_scope != to_scope {
            match &connection.cross_scope_authorization {
                None => issues.push(issue(
                    "cross_scope_unauthorized",
                    format!("{from_key}"),
                    "cross-scope connection requires explicit authorization",
                )),
                Some(authorization) if authorization.id.trim().is_empty() => issues.push(issue(
                    "empty_authorization",
                    format!("{from_key}"),
                    "cross-scope authorization id must not be empty",
                )),
                Some(authorization) => {
                    if let Some(context) = context {
                        match context.authorization_status(authorization) {
                            AuthorizationStatus::Authorized => {}
                            AuthorizationStatus::Denied => issues.push(issue(
                                "cross_scope_denied",
                                format!("{from_key}"),
                                "cross-scope authorization was denied",
                            )),
                            AuthorizationStatus::Missing | AuthorizationStatus::Unknown => issues
                                .push(issue(
                                    "cross_scope_unverified",
                                    format!("{from_key}"),
                                    "cross-scope authorization cannot be verified",
                                )),
                        }
                    }
                }
            }
        }

        if from.kind == PortKind::Sample {
            let from_step = identity_key(
                protocol_version_id,
                &connection.from.scope,
                &connection.from.step_path,
            );
            let to_step = identity_key(
                protocol_version_id,
                &connection.to.scope,
                &connection.to.step_path,
            );
            if let (Some(from_order), Some(to_order)) = (order.get(&from_step), order.get(&to_step))
            {
                if from_order > to_order {
                    let has_iteration = connection
                        .iteration
                        .as_ref()
                        .is_some_and(|iteration| !iteration.id.trim().is_empty());
                    if !has_iteration {
                        issues.push(issue(
                            "sample_causal_order",
                            format!("{from_key}"),
                            "sample flow must point forward; a backward edge requires explicit iteration semantics",
                        ));
                    }
                }
            }
        }

        if from.kind == PortKind::Data && to.kind == PortKind::Data {
            data_edges
                .entry(identity_key(
                    protocol_version_id,
                    &connection.from.scope,
                    &connection.from.step_path,
                ))
                .or_default()
                .push(identity_key(
                    protocol_version_id,
                    &connection.to.scope,
                    &connection.to.step_path,
                ));
        }

        if let Some(context) = context {
            let from_authoritative = context.authoritative_port(
                protocol_version_id,
                &connection.from.scope,
                &connection.from.step_path,
                &connection.from.port_id,
            );
            let to_authoritative = context.authoritative_port(
                protocol_version_id,
                &connection.to.scope,
                &connection.to.step_path,
                &connection.to.port_id,
            );
            match (from_authoritative, to_authoritative) {
                (Some(from_authoritative), Some(to_authoritative)) => {
                    if &from_authoritative != from || &to_authoritative != to {
                        issues.push(issue(
                            "port_contract_mismatch",
                            format!("{from_key} -> {to_key}"),
                            "declared ports do not match the authoritative PortSpec",
                        ));
                    }
                }
                _ => issues.push(issue(
                    "port_contract_missing",
                    format!("{from_key} -> {to_key}"),
                    "authoritative PortSpec is unavailable; compatibility is unknown",
                )),
            }
        }
    }

    if has_cycle(&data_edges) {
        issues.push(issue(
            "data_dependency_cycle",
            "connections",
            "pure data dependencies must not form a cycle",
        ));
    }
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

fn identity_key(protocol_version_id: &str, scope: &ProtocolScope, step_path: &str) -> String {
    format!("{protocol_version_id}::{}::{step_path}", scope.key())
}

fn port_key(scope: &ProtocolScope, step_path: &str, port_id: &str) -> String {
    format!("{}::{step_path}::{port_id}", scope.key())
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct BlockUseSource {
    pub use_id: String,
    pub block_version_id: String,
    pub scope: ProtocolScope,
    #[serde(default)]
    pub ports: Vec<PortSpec>,
    #[serde(default)]
    pub children: Vec<BlockUseSource>,
}

/// Read-only bridge from the existing ProtocolBlock/BlockUse shape. The caller
/// supplies the already-resolved block version and local ports; this function
/// only builds stable scope-qualified step paths.
pub fn composition_from_block_uses(
    protocol_version_id: impl Into<String>,
    roots: Vec<BlockUseSource>,
) -> CompositionProtocol {
    fn convert(source: BlockUseSource, parent: &str) -> CompositionNode {
        let step_path = if parent.is_empty() {
            source.use_id
        } else {
            format!("{parent}/{}", source.use_id)
        };
        let children = source
            .children
            .into_iter()
            .map(|child| convert(child, &step_path))
            .collect();
        CompositionNode {
            block_version_id: source.block_version_id,
            step_path,
            scope: source.scope,
            ports: source.ports,
            children,
            condition: None,
        }
    }

    CompositionProtocol {
        protocol_version_id: protocol_version_id.into(),
        roots: roots.into_iter().map(|root| convert(root, "")).collect(),
        connections: vec![],
    }
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
    use crate::operation_module::ParameterValueType;
    use crate::semantic_component::{
        BindingOwner, BindingResolution, BindingSource, BindingTarget, ParameterTarget,
        PlannedSampleRef, QualifiedStepRef, SampleExpectation, SampleTarget, TypedBinding,
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

    fn sample_port(id: &str, direction: PortDirection) -> PortSpec {
        PortSpec {
            id: id.into(),
            direction,
            kind: PortKind::Sample,
            unit: None,
            sample_expectation: Some(SampleExpectation::Planned),
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

    fn port_ref(scope: &str, path: &str, port: &str) -> StepPortRef {
        StepPortRef {
            scope: scope_of(scope),
            step_path: path.into(),
            port_id: port.into(),
        }
    }

    fn scope_of(assay: &str) -> ProtocolScope {
        scope(assay)
    }

    fn connection(from: StepPortRef, to: StepPortRef) -> PortConnection {
        PortConnection {
            from,
            to,
            cross_scope_authorization: None,
            iteration: None,
        }
    }

    #[derive(Default)]
    struct TestContext {
        ports: HashMap<String, PortSpec>,
        authorizations: HashMap<String, AuthorizationStatus>,
    }

    impl TestContext {
        fn with_port(mut self, scope: &str, path: &str, port: &str, spec: PortSpec) -> Self {
            self.ports
                .insert(format!("{}::{path}::{port}", scope_of(scope).key()), spec);
            self
        }

        fn with_authorization(mut self, id: &str, status: AuthorizationStatus) -> Self {
            self.authorizations.insert(id.into(), status);
            self
        }
    }

    impl CompositionContext for TestContext {
        fn authoritative_port(
            &self,
            _protocol_version_id: &str,
            scope: &ProtocolScope,
            step_path: &str,
            port_id: &str,
        ) -> Option<PortSpec> {
            self.ports
                .get(&format!("{}::{step_path}::{port_id}", scope.key()))
                .cloned()
        }

        fn authorization_status(&self, authorization: &AuthorizationRef) -> AuthorizationStatus {
            self.authorizations
                .get(&authorization.id)
                .copied()
                .unwrap_or(AuthorizationStatus::Missing)
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
            connection(
                port_ref("assay-1", "root/read-a", "out"),
                port_ref("assay-1", "root/read-b", "in"),
            ),
            connection(
                port_ref("assay-1", "root/read-b", "out"),
                port_ref("assay-1", "root/read-a", "in"),
            ),
        ];
        let errors = protocol(roots, connections).validate().unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "data_dependency_cycle"));
    }

    #[test]
    fn sample_back_edge_requires_explicit_iteration() {
        let roots = vec![
            node(
                "sample-a",
                "root/a",
                "assay-1",
                vec![
                    sample_port("in", PortDirection::Input),
                    sample_port("out", PortDirection::Output),
                ],
                vec![],
            ),
            node(
                "sample-b",
                "root/b",
                "assay-1",
                vec![
                    sample_port("in", PortDirection::Input),
                    sample_port("out", PortDirection::Output),
                ],
                vec![],
            ),
        ];
        let mut connections = vec![
            connection(
                port_ref("assay-1", "root/a", "out"),
                port_ref("assay-1", "root/b", "in"),
            ),
            connection(
                port_ref("assay-1", "root/b", "out"),
                port_ref("assay-1", "root/a", "in"),
            ),
        ];
        let errors = protocol(roots.clone(), connections.clone())
            .validate()
            .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "sample_causal_order"));
        connections[1].iteration = Some(IterationRef {
            id: "passage-loop".into(),
        });
        assert!(protocol(roots, connections).validate().is_ok());
    }

    #[test]
    fn same_path_in_different_scopes_is_not_duplicate() {
        let roots = vec![
            node("seed", "root/seed", "cck8", vec![], vec![]),
            node("seed", "root/seed", "live_dead", vec![], vec![]),
        ];
        assert!(protocol(roots, vec![]).validate().is_ok());
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
            vec![connection(
                port_ref("assay-1", "root/source", "out"),
                port_ref("assay-1", "root/sink", "in"),
            )],
        )
        .validate()
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "port_incompatible"));
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
            vec![connection(
                port_ref("assay-1", "root/source", "out"),
                port_ref("assay-1", "root/sink", "in"),
            )],
        )
        .validate()
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "port_incompatible"));
    }

    #[test]
    fn missing_unit_is_unknown_not_compatible() {
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
                vec![port("in", PortDirection::Input, PortKind::Data)],
                vec![],
            ),
        ];
        let errors = protocol(
            roots,
            vec![connection(
                port_ref("assay-1", "root/source", "out"),
                port_ref("assay-1", "root/sink", "in"),
            )],
        )
        .validate()
        .unwrap_err();
        assert!(errors.iter().any(|issue| issue.code == "port_unknown"));
    }

    #[test]
    fn cross_scope_authorization_must_be_verified_by_context() {
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
        let mut connection = connection(
            port_ref("cck8", "cck8/produce", "out"),
            port_ref("elisa", "elisa/consume", "in"),
        );
        let errors = protocol(roots.clone(), vec![connection.clone()])
            .validate()
            .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "cross_scope_unauthorized"));
        connection.cross_scope_authorization = Some(AuthorizationRef {
            id: "shared-material-1".into(),
        });
        assert!(protocol(roots.clone(), vec![connection.clone()])
            .validate()
            .is_ok());

        let context = TestContext::default();
        let errors = protocol(roots.clone(), vec![connection.clone()])
            .validate_with_context(&context)
            .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "cross_scope_unverified"));
    }

    #[test]
    fn authorized_cross_scope_connection_passes() {
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
        let mut connection = connection(
            port_ref("cck8", "cck8/produce", "out"),
            port_ref("elisa", "elisa/consume", "in"),
        );
        connection.cross_scope_authorization = Some(AuthorizationRef {
            id: "shared-material-1".into(),
        });
        let context = TestContext::default()
            .with_authorization("shared-material-1", AuthorizationStatus::Authorized)
            .with_port(
                "cck8",
                "cck8/produce",
                "out",
                port("out", PortDirection::Output, PortKind::Data),
            )
            .with_port(
                "elisa",
                "elisa/consume",
                "in",
                port("in", PortDirection::Input, PortKind::Data),
            );
        assert!(protocol(roots, vec![connection])
            .validate_with_context(&context)
            .is_ok());
    }

    #[test]
    fn authoritative_port_mismatch_is_rejected() {
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
                vec![port("in", PortDirection::Input, PortKind::Data)],
                vec![],
            ),
        ];
        let connection = connection(
            port_ref("assay-1", "root/source", "out"),
            port_ref("assay-1", "root/sink", "in"),
        );
        let context = TestContext::default()
            .with_port(
                "assay-1",
                "root/source",
                "out",
                PortSpec {
                    id: "out".into(),
                    direction: PortDirection::Output,
                    kind: PortKind::Data,
                    unit: Some("mL".into()),
                    sample_expectation: None,
                },
            )
            .with_port(
                "assay-1",
                "root/sink",
                "in",
                port("in", PortDirection::Input, PortKind::Data),
            );
        let errors = protocol(roots, vec![connection])
            .validate_with_context(&context)
            .unwrap_err();
        assert!(errors
            .iter()
            .any(|issue| issue.code == "port_contract_mismatch"));
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
                        step: QualifiedStepRef {
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
                        step: QualifiedStepRef {
                            protocol_version_id: "protocol-v1".into(),
                            scope: scope(method),
                            step_path: format!("{method}/transfer"),
                        },
                        parameter_key: "volume".into(),
                        expected_type: ParameterValueType::Number,
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

    #[test]
    fn block_use_adapter_keeps_scope_qualified_paths() {
        let roots = vec![
            BlockUseSource {
                use_id: "seed".into(),
                block_version_id: "block-seed".into(),
                scope: scope("cck8"),
                ports: vec![],
                children: vec![],
            },
            BlockUseSource {
                use_id: "seed".into(),
                block_version_id: "block-seed".into(),
                scope: scope("live_dead"),
                ports: vec![],
                children: vec![],
            },
        ];
        let adapted = composition_from_block_uses("protocol-v1", roots);
        assert_eq!(adapted.roots[0].step_path, "seed");
        assert_eq!(adapted.roots[1].step_path, "seed");
        assert!(adapted.validate().is_ok());
    }
}
