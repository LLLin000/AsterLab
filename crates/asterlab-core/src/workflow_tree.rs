//! Read-only Design/Run workflow hierarchy projection.
//!
//! This is the Rust Core counterpart of the Go `internal/workflowtree` engine.
//! It consumes a platform-neutral parsed document and explicit Run tasks; the
//! CLI adapter loads the document and ProtocolBlock definitions.
//!
//! Identity is explicit: a definition is an immutable ProtocolBlock version, a
//! use is an AsterDoc step, and an occurrence is a frozen Run task. Paths locate
//! nodes; titles never establish identity.

use serde_json::{json, Map, Value};
use std::collections::{BTreeMap, HashMap, HashSet};

#[derive(Clone, Debug, Default)]
pub struct Document {
    pub steps: Vec<DesignStep>,
    pub reference_blocks: HashMap<String, String>,
    pub order: Vec<String>,
    pub edges: Vec<(String, String)>,
}

#[derive(Clone, Debug, Default)]
pub struct DesignStep {
    pub id: String,
    pub title: String,
    pub path: String,
    pub order: i64,
    pub organizational: bool,
    pub refs: Vec<String>,
}

#[derive(Clone, Debug, Default)]
pub struct BlockDefinition {
    pub steps: Vec<BlockStep>,
}

#[derive(Clone, Debug, Default)]
pub struct BlockStep {
    pub use_id: String,
    pub path: String,
    pub title: String,
    pub instruction: String,
    pub source_block_version_id: String,
    pub parameters: Vec<BlockParameter>,
    pub unresolved_parameters: Vec<String>,
}

#[derive(Clone, Debug)]
pub struct BlockParameter {
    pub key: String,
    pub label: String,
    pub value: Value,
    pub unit: Option<String>,
}

#[derive(Clone, Debug, Default)]
pub struct RunTask {
    pub key: String,
    pub path: String,
    pub title: String,
    pub status: String,
    pub protocol_version_id: String,
    pub source_block_version_id: String,
    pub assay_id: String,
    pub timepoint: String,
    pub design_step_id: String,
}

#[derive(Clone, Debug, Default)]
struct Node {
    id: String,
    title: String,
    path: String,
    kind: String,
    identity: String,
    composite: bool,
    order: i64,
    use_id: String,
    block_version_id: String,
    definition_ref: String,
    protocol_version_id: String,
    assay_id: String,
    timepoint: String,
    instruction: String,
    parameters: Vec<BlockParameter>,
    unresolved_parameters: Vec<String>,
    organizational: bool,
    status: String,
    completed: i64,
    total: i64,
    task_key: String,
    children: Vec<Node>,
    issues: Vec<String>,
}

impl Node {
    fn to_value(&self) -> Value {
        let mut out = Map::new();
        out.insert("id".into(), json!(self.id));
        out.insert("title".into(), json!(self.title));
        out.insert("path".into(), json!(self.path));
        out.insert("kind".into(), json!(self.kind));
        if !self.identity.is_empty() {
            out.insert("identity".into(), json!(self.identity));
        }
        if self.composite {
            out.insert("composite".into(), json!(true));
        }
        if self.order != 0 {
            out.insert("order".into(), json!(self.order));
        }
        if !self.use_id.is_empty() {
            out.insert("useId".into(), json!(self.use_id));
        }
        if !self.block_version_id.is_empty() {
            out.insert("blockVersionId".into(), json!(self.block_version_id));
        }
        if !self.definition_ref.is_empty() {
            out.insert("definitionRef".into(), json!(self.definition_ref));
        }
        if !self.protocol_version_id.is_empty() {
            out.insert("protocolVersionId".into(), json!(self.protocol_version_id));
        }
        if !self.assay_id.is_empty() {
            out.insert("assayId".into(), json!(self.assay_id));
        }
        if !self.timepoint.is_empty() {
            out.insert("timepoint".into(), json!(self.timepoint));
        }
        if !self.instruction.is_empty() {
            out.insert("instruction".into(), json!(self.instruction));
        }
        if !self.parameters.is_empty() {
            let parameters = self
                .parameters
                .iter()
                .map(|parameter| {
                    let mut item = Map::new();
                    item.insert("key".into(), json!(parameter.key));
                    item.insert("label".into(), json!(parameter.label));
                    item.insert("value".into(), parameter.value.clone());
                    if let Some(unit) = &parameter.unit {
                        item.insert("unit".into(), json!(unit));
                    }
                    Value::Object(item)
                })
                .collect::<Vec<_>>();
            out.insert("parameters".into(), json!(parameters));
        }
        if !self.unresolved_parameters.is_empty() {
            out.insert(
                "unresolvedParameters".into(),
                json!(self.unresolved_parameters),
            );
        }
        if self.organizational {
            out.insert("organizational".into(), json!(true));
        }
        if !self.status.is_empty() {
            out.insert("status".into(), json!(self.status));
        }
        if self.completed != 0 {
            out.insert("completed".into(), json!(self.completed));
        }
        if self.total != 0 {
            out.insert("total".into(), json!(self.total));
        }
        if !self.task_key.is_empty() {
            out.insert("taskKey".into(), json!(self.task_key));
        }
        if !self.children.is_empty() {
            out.insert(
                "children".into(),
                json!(self.children.iter().map(Node::to_value).collect::<Vec<_>>()),
            );
        }
        if !self.issues.is_empty() {
            out.insert("issues".into(), json!(self.issues));
        }
        Value::Object(out)
    }
}

#[derive(Clone, Debug, Default)]
struct TaskRef {
    key: String,
    path: String,
    title: String,
    status: String,
    protocol_version_id: String,
    source_block_version_id: String,
    assay_id: String,
    timepoint: String,
}

/// Build the Design/Use/Definition tree. No Run occurrence is present.
fn find_design_parent_mut<'a>(nodes: &'a mut [Node], path: &str) -> Option<&'a mut Node> {
    for node in nodes {
        if node.identity == "use" && node.path == path {
            return Some(node);
        }
        if node.identity == "use" {
            if let Some(parent) = find_design_parent_mut(&mut node.children, path) {
                return Some(parent);
            }
        }
    }
    None
}

fn sort_design_nodes(nodes: &mut [Node], ranks: &HashMap<String, usize>) {
    nodes.sort_by_key(|node| (ranks.get(&node.id).copied().unwrap_or(usize::MAX), node.order));
    for node in nodes {
        sort_design_nodes(&mut node.children, ranks);
    }
}

pub fn build_design(
    document: &Document,
    blocks: &HashMap<String, Result<BlockDefinition, String>>,
) -> Value {
    let ranks: HashMap<String, usize> = document.order.iter().enumerate()
        .map(|(rank, id)| (id.clone(), rank)).collect();
    let mut steps = document.steps.clone();
    steps.sort_by(|a, b| {
        let depth = |path: &str| path.split('/').count();
        depth(&a.path).cmp(&depth(&b.path))
            .then(ranks.get(&a.id).copied().unwrap_or(usize::MAX)
                .cmp(&ranks.get(&b.id).copied().unwrap_or(usize::MAX)))
            .then(a.order.cmp(&b.order))
    });

    let mut flat_nodes = Vec::new();
    let mut issues = Vec::new();
    let mut seen_paths: HashMap<String, String> = HashMap::new();

    for step in steps {
        if step.path.is_empty() {
            issues.push(format!("step {} has no path", step.id));
            continue;
        }
        if let Some(previous) = seen_paths.get(&step.path) {
            issues.push(format!(
                "steps {} and {} share path {}",
                previous, step.id, step.path
            ));
            continue;
        }
        seen_paths.insert(step.path.clone(), step.id.clone());

        let mut node = Node {
            id: step.id.clone(),
            title: step.title.clone(),
            path: step.path.clone(),
            kind: "step".into(),
            identity: "use".into(),
            order: step.order,
            use_id: step.path.clone(),
            total: 1,
            organizational: step.organizational,
            ..Node::default()
        };
        let references = resolve_step_blocks(&step, document);
        match references.len() {
            0 => {
                if step.organizational {
                    node.status = "organizational".into();
                    node.total = 0;
                }
                flat_nodes.push(node);
            }
            1 => {
                let version = references[0].clone();
                node.block_version_id = version.clone();
                node.definition_ref = version.clone();
                match blocks.get(&version) {
                    None => {
                        node.issues
                            .push("ProtocolBlock provider is unavailable".into());
                    }
                    Some(Err(error)) => {
                        node.issues
                            .push(format!("ProtocolBlock {version}: {error}"));
                    }
                    Some(Ok(definition)) => {
                        let mut definition_issues = Vec::new();
                        let children = convert_definition_trie(
                            &build_definition_trie(&step.path, &definition.steps),
                            &mut definition_issues,
                        );
                        if children.is_empty() {
                            node.issues
                                .push("ProtocolBlock has no definition steps".into());
                        } else {
                            node.composite = true;
                            node.total = count_definition_leaves(&children);
                            node.children = children;
                        }
                        node.issues.extend(definition_issues);
                    }
                }
                flat_nodes.push(node);
            }
            _ => {
                node.issues
                    .push("step references multiple ProtocolBlock definitions".into());
                flat_nodes.push(node);
            }
        }
    }
    issues.extend(flat_nodes.iter().flat_map(|node| node.issues.clone()));

    let mut roots = Vec::new();
    for node in flat_nodes {
        let parent_path = node.path.rsplit_once('/').map(|(parent, _)| parent);
        if let Some(parent) = parent_path.and_then(|path| find_design_parent_mut(&mut roots, path)) {
            parent.children.push(node);
        } else {
            roots.push(node);
        }
    }
    sort_design_nodes(&mut roots, &ranks);
    finalize(document, roots, issues, true)
}

/// Build the frozen Run occurrence tree layered over the Design/Use tree.
pub fn build_run(
    document: &Document,
    blocks: &HashMap<String, Result<BlockDefinition, String>>,
    tasks: &[RunTask],
) -> Value {
    let _ = blocks;
    let mut steps = document.steps.clone();
    steps.sort_by(|a, b| a.order.cmp(&b.order).then(a.path.cmp(&b.path)));

    let mut valid_steps = Vec::new();
    let mut issues = Vec::new();
    let mut duplicate_paths = HashSet::new();
    let mut seen_paths: HashMap<String, String> = HashMap::new();
    for step in steps {
        if step.path.is_empty() {
            issues.push(format!("step {} has no path", step.id));
            continue;
        }
        if let Some(previous) = seen_paths.get(&step.path) {
            issues.push(format!(
                "steps {} and {} share path {}",
                previous, step.id, step.path
            ));
            duplicate_paths.insert(step.path.clone());
            continue;
        }
        seen_paths.insert(step.path.clone(), step.id.clone());
        valid_steps.push(step);
    }

    let mut assignments: HashMap<String, Vec<TaskRef>> = HashMap::new();
    let mut orphans = Vec::new();
    let mut ambiguous = Vec::new();
    for task in tasks {
        let path = if task.path.is_empty() {
            task_path_from_key(&task.key)
        } else {
            task.path.clone()
        };
        let reference = TaskRef {
            key: task.key.clone(),
            path: path.clone(),
            title: task.title.clone(),
            status: task.status.clone(),
            protocol_version_id: task.protocol_version_id.clone(),
            source_block_version_id: task.source_block_version_id.clone(),
            assay_id: task.assay_id.clone(),
            timepoint: task.timepoint.clone(),
        };
        if duplicate_paths
            .iter()
            .any(|duplicate| path == *duplicate || path.starts_with(&format!("{duplicate}/")))
        {
            issues.push(format!(
                "task path {path} matches a duplicated design path"
            ));
            ambiguous.push(reference);
            continue;
        }
        let path_matches = valid_steps
            .iter()
            .filter(|step| {
                !step.organizational
                    && (path == step.path || path.starts_with(&format!("{}/", step.path)))
            })
            .collect::<Vec<_>>();
        let id_matches = if task.design_step_id.is_empty() {
            Vec::new()
        } else {
            valid_steps
                .iter()
                .filter(|step| !step.organizational && step.id == task.design_step_id)
                .collect::<Vec<_>>()
        };
        let selected = if !task.design_step_id.is_empty() {
            if id_matches.len() != 1 {
                issues.push(format!(
                    "task path {} names DesignStepID {} but it resolves to {} design steps",
                    path,
                    task.design_step_id,
                    id_matches.len()
                ));
                ambiguous.push(reference);
                continue;
            }
            let target = id_matches[0];
            if !path_matches.is_empty()
                && (path_matches.len() != 1 || path_matches[0].id != target.id)
            {
                issues.push(format!(
                    "task path {} matches a different design step than DesignStepID {}",
                    path, task.design_step_id
                ));
                ambiguous.push(reference);
                continue;
            }
            vec![target]
        } else {
            path_matches
        };
        match selected.len() {
            0 => orphans.push(reference),
            1 => assignments
                .entry(selected[0].id.clone())
                .or_default()
                .push(reference),
            _ => {
                issues.push(format!(
                    "task path {} matches {} design steps",
                    path,
                    selected.len()
                ));
                ambiguous.push(reference);
            }
        }
    }
    if !orphans.is_empty() {
        issues.push(format!(
            "{} frozen task(s) have no design step",
            orphans.len()
        ));
    }

    let mut roots = Vec::new();
    for step in valid_steps {
        let assigned = assignments.remove(&step.id).unwrap_or_default();
        let mut node = Node {
            id: step.id.clone(),
            title: step.title.clone(),
            path: step.path.clone(),
            kind: "step".into(),
            identity: "use".into(),
            order: step.order,
            use_id: step.path.clone(),
            organizational: step.organizational,
            ..Node::default()
        };
        let exact = assigned
            .iter()
            .filter(|task| task.path == step.path)
            .cloned()
            .collect::<Vec<_>>();
        let descendants = assigned
            .iter()
            .filter(|task| task.path != step.path)
            .cloned()
            .collect::<Vec<_>>();
        if !exact.is_empty() && !descendants.is_empty() {
            node.issues
                .push("step mixes an exact task and child tasks".into());
        }
        if !descendants.is_empty() {
            node.composite = true;
            node.children = build_task_trie(&step.path, &descendants);
            let definition = resolve_definition(&step, document);
            node.definition_ref = definition.clone();
            node.block_version_id = definition.clone();
            if let Some(protocol) = descendants
                .iter()
                .find(|task| !task.protocol_version_id.is_empty())
            {
                node.protocol_version_id = protocol.protocol_version_id.clone();
            }
            validate_composite(&mut node, &step.path, &definition, &descendants);
        } else if !exact.is_empty() {
            node.children = exact.iter().map(task_node).collect();
            if exact.len() > 1 {
                node.issues
                    .push("multiple exact tasks share one step path".into());
            }
            node.protocol_version_id = exact[0].protocol_version_id.clone();
            node.definition_ref = exact[0].source_block_version_id.clone();
            node.block_version_id = exact[0].source_block_version_id.clone();
        } else if !step.organizational {
            node.issues
                .push("no frozen task matches this step path".into());
        }
        aggregate_node(&mut node);
        roots.push(node);
    }
    for root in &roots {
        issues.extend(root.issues.clone());
    }

    let mut value = finalize(document, roots, issues, false);
    if !orphans.is_empty() {
        value["orphans"] = json!(orphans.iter().map(task_ref_value).collect::<Vec<_>>());
    }
    if !ambiguous.is_empty() {
        value["ambiguous"] =
            json!(ambiguous.iter().map(task_ref_value).collect::<Vec<_>>());
    }
    value
}

fn resolve_step_blocks(step: &DesignStep, document: &Document) -> Vec<String> {
    step.refs
        .iter()
        .filter_map(|reference| document.reference_blocks.get(reference))
        .filter(|version| !version.is_empty())
        .cloned()
        .collect()
}

fn resolve_definition(step: &DesignStep, document: &Document) -> String {
    resolve_step_blocks(step, document)
        .into_iter()
        .next()
        .unwrap_or_default()
}

fn finalize(document: &Document, roots: Vec<Node>, mut issues: Vec<String>, design: bool) -> Value {
    let roots = order_roots(roots, &document.order);
    let precedes = precedes_map(&document.edges, &mut issues);
    let mut out = Map::new();
    out.insert("design".into(), json!(design));
    out.insert(
        "roots".into(),
        json!(roots.iter().map(Node::to_value).collect::<Vec<_>>()),
    );
    out.insert("order".into(), json!(document.order));
    out.insert("precedes".into(), Value::Object(precedes));
    out.insert(
        "issues".into(),
        if issues.is_empty() && design {
            Value::Null
        } else {
            json!(issues)
        },
    );
    Value::Object(out)
}

fn order_roots(roots: Vec<Node>, order: &[String]) -> Vec<Node> {
    if order.is_empty() || roots.len() < 2 {
        return roots;
    }
    let original = roots.iter().map(|root| root.id.clone()).collect::<Vec<_>>();
    let mut by_id = roots
        .into_iter()
        .map(|root| (root.id.clone(), root))
        .collect::<HashMap<_, _>>();
    let mut seen = HashSet::new();
    let mut out = Vec::new();
    for id in order {
        if seen.contains(id) {
            continue;
        }
        if let Some(root) = by_id.remove(id) {
            seen.insert(id.clone());
            out.push(root);
        }
    }
    for id in original {
        if seen.contains(&id) {
            continue;
        }
        if let Some(root) = by_id.remove(&id) {
            out.push(root);
        }
    }
    out
}

fn precedes_map(
    edges: &[(String, String)],
    issues: &mut Vec<String>,
) -> Map<String, Value> {
    let mut grouped: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for (from, to) in edges {
        if !from.is_empty() && !to.is_empty() {
            grouped.entry(from.clone()).or_default().push(to.clone());
        }
    }
    if let Some(cycle) = find_cycle(&grouped) {
        issues.push(format!("precedes cycle at {cycle}"));
    }
    grouped
        .into_iter()
        .map(|(from, to)| (from, json!(to)))
        .collect()
}

fn find_cycle(edges: &BTreeMap<String, Vec<String>>) -> Option<String> {
    fn visit(
        node: &str,
        edges: &BTreeMap<String, Vec<String>>,
        state: &mut HashMap<String, u8>,
    ) -> Option<String> {
        match state.get(node).copied() {
            Some(1) => return Some(node.to_owned()),
            Some(2) => return None,
            _ => {}
        }
        state.insert(node.to_owned(), 1);
        if let Some(next) = edges.get(node) {
            for target in next {
                if let Some(found) = visit(target, edges, state) {
                    return Some(found);
                }
            }
        }
        state.insert(node.to_owned(), 2);
        None
    }
    let mut state = HashMap::new();
    for node in edges.keys() {
        if let Some(found) = visit(node, edges, &mut state) {
            return Some(found);
        }
    }
    None
}

#[derive(Default)]
struct DefinitionTrie {
    segment: String,
    path: String,
    step: Option<BlockStep>,
    children: BTreeMap<String, DefinitionTrie>,
}

fn build_definition_trie(prefix: &str, steps: &[BlockStep]) -> DefinitionTrie {
    let mut root = DefinitionTrie::default();
    for step in steps {
        let relative = step
            .path
            .strip_prefix("root/")
            .unwrap_or(&step.path)
            .trim_matches('/')
            .to_owned();
        let relative = if relative.is_empty() {
            step.use_id
                .strip_prefix("root/")
                .unwrap_or(&step.use_id)
                .trim_matches('/')
                .to_owned()
        } else {
            relative
        };
        let parts = relative
            .split('/')
            .filter(|part| !part.is_empty())
            .collect::<Vec<_>>();
        if parts.is_empty() {
            continue;
        }
        let mut current = &mut root;
        for (index, part) in parts.iter().enumerate() {
            let path = format!("{}/{}", prefix, parts[..=index].join("/"));
            current = current
                .children
                .entry((*part).to_owned())
                .or_insert_with(|| DefinitionTrie {
                    segment: (*part).to_owned(),
                    path,
                    step: None,
                    children: BTreeMap::new(),
                });
        }
        if current.step.is_some() {
            continue;
        }
        current.step = Some(step.clone());
    }
    root
}

fn convert_definition_trie(node: &DefinitionTrie, issues: &mut Vec<String>) -> Vec<Node> {
    let mut out = Vec::new();
    for child in node.children.values() {
        match (&child.step, child.children.is_empty()) {
            (Some(step), true) => {
                let title = if step.title.trim().is_empty() {
                    child.segment.clone()
                } else {
                    step.title.clone()
                };
                out.push(Node {
                    id: child.path.clone(),
                    title,
                    path: child.path.clone(),
                    kind: "step".into(),
                    identity: "definition".into(),
                    use_id: child.path.clone(),
                    block_version_id: step.source_block_version_id.clone(),
                    definition_ref: step.source_block_version_id.clone(),
                    instruction: step.instruction.clone(),
                    parameters: step.parameters.clone(),
                    unresolved_parameters: step.unresolved_parameters.clone(),
                    total: 1,
                    ..Node::default()
                });
            }
            (None, false) => {
                let nested = convert_definition_trie(child, issues);
                out.push(Node {
                    id: child.path.clone(),
                    title: child.segment.clone(),
                    path: child.path.clone(),
                    kind: "step".into(),
                    identity: "definition".into(),
                    composite: true,
                    use_id: child.path.clone(),
                    total: count_definition_leaves(&nested),
                    children: nested,
                    ..Node::default()
                });
            }
            (Some(_), false) => {
                issues.push(format!(
                    "definition {} mixes a step and child steps",
                    child.path
                ));
                let nested = convert_definition_trie(child, issues);
                out.push(Node {
                    id: child.path.clone(),
                    title: child.segment.clone(),
                    path: child.path.clone(),
                    kind: "step".into(),
                    identity: "definition".into(),
                    composite: true,
                    use_id: child.path.clone(),
                    total: count_definition_leaves(&nested),
                    children: nested,
                    ..Node::default()
                });
            }
            (None, true) => {}
        }
    }
    out
}

fn count_definition_leaves(nodes: &[Node]) -> i64 {
    nodes
        .iter()
        .map(|node| {
            if node.children.is_empty() {
                1
            } else {
                count_definition_leaves(&node.children)
            }
        })
        .sum()
}

#[derive(Default)]
struct TaskTrie {
    segment: String,
    path: String,
    tasks: Vec<TaskRef>,
    children: BTreeMap<String, TaskTrie>,
}

fn build_task_trie(step_path: &str, tasks: &[TaskRef]) -> Vec<Node> {
    let prefix = format!("{step_path}/");
    let mut foreign = Vec::new();
    for task in tasks {
        if !task.path.starts_with(&prefix) {
            let parts = task.path.split('/').collect::<Vec<_>>();
            if parts.len() > 1 {
                foreign.push(parts[..parts.len() - 1].to_vec());
            }
        }
    }
    let mut base = String::new();
    if let Some(first) = foreign.first() {
        let mut common = first.clone();
        for parts in &foreign[1..] {
            let mut length = 0;
            while length < common.len()
                && length < parts.len()
                && common[length] == parts[length]
            {
                length += 1;
            }
            common.truncate(length);
        }
        if !common.is_empty() {
            base = format!("{}/", common.join("/"));
        }
    }

    let mut root = TaskTrie::default();
    for task in tasks {
        let mut suffix = task
            .path
            .strip_prefix(&prefix)
            .unwrap_or(&task.path)
            .to_owned();
        if suffix == task.path && !base.is_empty() {
            suffix = task.path.strip_prefix(&base).unwrap_or(&task.path).to_owned();
        }
        let parts = suffix
            .split('/')
            .filter(|part| !part.is_empty())
            .collect::<Vec<_>>();
        let mut current = &mut root;
        for (index, part) in parts.iter().enumerate() {
            let path = format!("{step_path}/{}", parts[..=index].join("/"));
            current = current
                .children
                .entry((*part).to_owned())
                .or_insert_with(|| TaskTrie {
                    segment: (*part).to_owned(),
                    path,
                    tasks: Vec::new(),
                    children: BTreeMap::new(),
                });
        }
        current.tasks.push(task.clone());
    }
    convert_task_trie(&root)
}

fn convert_task_trie(node: &TaskTrie) -> Vec<Node> {
    let mut out = node.tasks.iter().map(task_node).collect::<Vec<_>>();
    for child in node.children.values() {
        if !child.tasks.is_empty() && child.children.is_empty() {
            out.extend(child.tasks.iter().map(task_node));
            continue;
        }
        let mut node = Node {
            id: child.path.clone(),
            title: child.segment.clone(),
            path: child.path.clone(),
            kind: "step".into(),
            composite: true,
            identity: "use".into(),
            use_id: child.path.clone(),
            ..Node::default()
        };
        node.children = convert_task_trie(child);
        if !child.tasks.is_empty() && !child.children.is_empty() {
            node.issues
                .push("nested node mixes a task and child tasks".into());
        }
        aggregate_node(&mut node);
        out.push(node);
    }
    out
}

fn task_ref_value(task: &TaskRef) -> Value {
    let mut out = Map::new();
    out.insert("key".into(), json!(task.key));
    out.insert("path".into(), json!(task.path));
    out.insert("title".into(), json!(task.title));
    for (key, value) in [
        ("status", &task.status),
        ("protocolVersionId", &task.protocol_version_id),
        ("sourceBlockVersionId", &task.source_block_version_id),
        ("assayId", &task.assay_id),
        ("timepoint", &task.timepoint),
    ] {
        if !value.is_empty() {
            out.insert(key.into(), json!(value));
        }
    }
    Value::Object(out)
}

fn task_node(task: &TaskRef) -> Node {
    let title = if task.title.is_empty() {
        task.path.split('/').last().unwrap_or("").to_owned()
    } else {
        task.title.clone()
    };
    Node {
        id: task.key.clone(),
        title,
        path: task.path.clone(),
        kind: "task".into(),
        identity: "occurrence".into(),
        block_version_id: task.source_block_version_id.clone(),
        definition_ref: task.source_block_version_id.clone(),
        task_key: task.key.clone(),
        protocol_version_id: task.protocol_version_id.clone(),
        assay_id: task.assay_id.clone(),
        timepoint: task.timepoint.clone(),
        status: task.status.clone(),
        completed: if task.status == "completed" { 1 } else { 0 },
        total: 1,
        ..Node::default()
    }
}

fn validate_composite(node: &mut Node, step_path: &str, definition: &str, tasks: &[TaskRef]) {
    if definition.is_empty() {
        node.issues
            .push("composite step has no explicit reference to a child definition".into());
    }
    let mut protocols = HashSet::new();
    for task in tasks {
        if !task.protocol_version_id.is_empty() {
            protocols.insert(task.protocol_version_id.clone());
        }
        let relative = task
            .path
            .strip_prefix(&format!("{step_path}/"))
            .unwrap_or(&task.path);
        if relative.contains('/') {
            continue;
        }
        if !definition.is_empty()
            && !task.source_block_version_id.is_empty()
            && task.source_block_version_id != definition
        {
            node.issues.push(format!(
                "child task {} source block {} does not match referenced block {}",
                task.path, task.source_block_version_id, definition
            ));
        }
    }
    if protocols.len() > 1 {
        node.issues
            .push("composite children reference multiple Protocol versions".into());
    }
}

fn aggregate_node(node: &mut Node) {
    let mut completed = 0;
    let mut total = 0;
    let mut blocked = false;
    let mut current = false;
    for child in &node.children {
        completed += child.completed;
        total += child.total;
        match child.status.as_str() {
            "blocked" => blocked = true,
            "current" | "in_progress" | "started" | "active" => current = true,
            _ => {}
        }
    }
    node.completed = completed;
    node.total = total;
    node.status = if node.organizational {
        "organizational".into()
    } else if total == 0 {
        "unresolved".into()
    } else if blocked {
        "blocked".into()
    } else if completed == total {
        "completed".into()
    } else if current {
        "current".into()
    } else {
        "pending".into()
    };
}

fn task_path_from_key(key: &str) -> String {
    serde_json::from_str::<Value>(key)
        .ok()
        .and_then(|value| {
            value
                .get("path")
                .and_then(Value::as_str)
                .map(str::to_owned)
        })
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn block_step(path: &str, title: &str, source: &str) -> BlockStep {
        BlockStep {
            use_id: path.split('/').last().unwrap_or("").to_owned(),
            path: path.into(),
            title: title.into(),
            instruction: format!("do {title}"),
            source_block_version_id: source.into(),
            parameters: vec![],
            unresolved_parameters: vec![],
        }
    }

    #[test]
    fn design_projection_uses_definition_identity_and_order() {
        let document = Document {
            steps: vec![
                DesignStep {
                    id: "step-parent".into(),
                    title: "Parent".into(),
                    path: "root/parent".into(),
                    order: 1,
                    organizational: false,
                    refs: vec!["ref-block".into()],
                },
                DesignStep {
                    id: "step-leaf".into(),
                    title: "Leaf".into(),
                    path: "root/leaf".into(),
                    order: 2,
                    organizational: false,
                    refs: vec![],
                },
            ],
            reference_blocks: HashMap::from([("ref-block".into(), "block-v1".into())]),
            order: vec!["step-parent".into(), "step-leaf".into()],
            edges: vec![("step-parent".into(), "step-leaf".into())],
        };
        let blocks = HashMap::from([(
            "block-v1".into(),
            Ok(BlockDefinition {
                steps: vec![
                    block_step("root/a", "A", "block-v1"),
                    block_step("root/group/b", "B", "block-v1"),
                ],
            }),
        )]);
        let value = build_design(&document, &blocks);
        assert_eq!(value["design"], true);
        assert_eq!(value["roots"][0]["id"], "step-parent");
        assert_eq!(value["roots"][0]["identity"], "use");
        assert_eq!(value["roots"][0]["total"], 2);
        let child = &value["roots"][0]["children"][0];
        assert_eq!(child["identity"], "definition");
        assert_eq!(child["id"], "root/parent/a");
        assert_eq!(value["roots"][1]["id"], "step-leaf");
        assert_eq!(value["precedes"]["step-parent"][0], "step-leaf");
    }

    #[test]
    fn design_projection_nests_step_paths_and_uses_business_order_for_siblings() {
        let document = Document {
            steps: vec![
                DesignStep { id: "parent".into(), title: "Parent".into(), path: "root/parent".into(), order: 1, organizational: true, refs: vec![] },
                DesignStep { id: "child-b".into(), title: "B".into(), path: "root/parent/b".into(), order: 2, organizational: true, refs: vec![] },
                DesignStep { id: "child-a".into(), title: "A".into(), path: "root/parent/a".into(), order: 3, organizational: true, refs: vec![] },
                DesignStep { id: "sibling".into(), title: "Sibling".into(), path: "root/sibling".into(), order: 4, organizational: true, refs: vec![] },
            ],
            order: vec!["parent".into(), "child-a".into(), "child-b".into(), "sibling".into()],
            ..Document::default()
        };
        let value = build_design(&document, &HashMap::new());
        assert_eq!(value["roots"][0]["id"], "parent");
        assert_eq!(value["roots"][0]["children"][0]["id"], "child-a");
        assert_eq!(value["roots"][0]["children"][1]["id"], "child-b");
        assert_eq!(value["roots"][1]["id"], "sibling");
        assert_eq!(value["order"], json!(["parent","child-a","child-b","sibling"]));
    }

    #[test]
    fn run_projection_aggregates_tasks_and_reports_orphans() {
        let document = Document {
            steps: vec![DesignStep {
                id: "step-parent".into(),
                title: "Parent".into(),
                path: "root/parent".into(),
                order: 1,
                organizational: false,
                refs: vec!["ref-block".into()],
            }],
            reference_blocks: HashMap::from([("ref-block".into(), "block-v1".into())]),
            order: vec![],
            edges: vec![],
        };
        let tasks = vec![
            RunTask {
                key: "task-a".into(),
                path: "root/parent/a".into(),
                title: "A".into(),
                status: "completed".into(),
                protocol_version_id: "protocol-v1".into(),
                source_block_version_id: "block-v1".into(),
                assay_id: "assay-1".into(),
                timepoint: "D1".into(),
                design_step_id: "step-parent".into(),
            },
            RunTask {
                key: "task-b".into(),
                path: "root/parent/b".into(),
                title: "B".into(),
                status: "blocked".into(),
                protocol_version_id: "protocol-v1".into(),
                source_block_version_id: "block-v1".into(),
                assay_id: "assay-1".into(),
                timepoint: "D1".into(),
                design_step_id: "step-parent".into(),
            },
            RunTask {
                key: "task-orphan".into(),
                path: "root/elsewhere".into(),
                title: "Orphan".into(),
                status: "not_started".into(),
                protocol_version_id: "protocol-v1".into(),
                source_block_version_id: "block-v1".into(),
                assay_id: "assay-1".into(),
                timepoint: "D1".into(),
                design_step_id: "".into(),
            },
        ];
        let value = build_run(&document, &HashMap::new(), &tasks);
        assert_eq!(value["design"], false);
        assert_eq!(value["roots"][0]["status"], "blocked");
        assert_eq!(value["roots"][0]["completed"], 1);
        assert_eq!(value["roots"][0]["total"], 2);
        assert_eq!(value["orphans"][0]["key"], "task-orphan");
    }

    #[test]
    fn run_projection_keeps_multiple_timepoints_as_occurrences() {
        let document = Document {
            steps: vec![DesignStep {
                id: "step-parent".into(),
                title: "Parent".into(),
                path: "root/parent".into(),
                order: 1,
                organizational: false,
                refs: vec!["ref-block".into()],
            }],
            reference_blocks: HashMap::from([("ref-block".into(), "block-v1".into())]),
            order: vec![],
            edges: vec![],
        };
        let task = |key: &str, timepoint: &str| RunTask {
            key: key.into(),
            path: "root/parent/op".into(),
            title: "Op".into(),
            status: "pending".into(),
            protocol_version_id: "protocol-v1".into(),
            source_block_version_id: "block-v1".into(),
            assay_id: "assay-1".into(),
            timepoint: timepoint.into(),
            design_step_id: "step-parent".into(),
        };
        let tasks = vec![task("task-d1", "D1"), task("task-d3", "D3")];
        let value = build_run(&document, &HashMap::new(), &tasks);
        let children = value["roots"][0]["children"].as_array().unwrap();
        assert_eq!(children.len(), 2);
        assert_eq!(value["roots"][0]["total"], 2);
        assert_eq!(children[0]["timepoint"], "D1");
        assert_eq!(children[1]["timepoint"], "D3");
    }

    #[test]
    fn design_projection_reports_a_missing_block_provider() {
        let document = Document {
            steps: vec![DesignStep {
                id: "step-parent".into(),
                title: "Parent".into(),
                path: "root/parent".into(),
                order: 1,
                organizational: false,
                refs: vec!["ref-block".into()],
            }],
            reference_blocks: HashMap::from([("ref-block".into(), "block-v1".into())]),
            order: vec![],
            edges: vec![],
        };
        let value = build_design(&document, &HashMap::new());
        assert!(value["roots"][0]["children"].is_null());
        assert!(
            value["roots"][0]["issues"]
                .as_array()
                .unwrap()
                .iter()
                .any(|issue| issue.as_str().unwrap_or("").contains("provider is unavailable"))
        );
    }

    #[test]
    fn design_projection_reports_duplicate_step_paths() {
        let document = Document {
            steps: vec![
                DesignStep {
                    id: "step-a".into(),
                    title: "A".into(),
                    path: "root/shared".into(),
                    order: 1,
                    organizational: false,
                    refs: vec![],
                },
                DesignStep {
                    id: "step-b".into(),
                    title: "B".into(),
                    path: "root/shared".into(),
                    order: 2,
                    organizational: false,
                    refs: vec![],
                },
            ],
            reference_blocks: HashMap::new(),
            order: vec![],
            edges: vec![],
        };
        let value = build_design(&document, &HashMap::new());
        assert_eq!(value["roots"].as_array().unwrap().len(), 1);
        assert!(
            value["issues"]
                .as_array()
                .unwrap()
                .iter()
                .any(|issue| issue.as_str().unwrap_or("").contains("share path"))
        );
    }

    #[test]
    fn run_projection_does_not_bind_a_duplicated_design_path() {
        let document = Document {
            steps: vec![
                DesignStep {
                    id: "step-a".into(),
                    title: "A".into(),
                    path: "root/shared".into(),
                    order: 1,
                    organizational: false,
                    refs: vec![],
                },
                DesignStep {
                    id: "step-b".into(),
                    title: "B".into(),
                    path: "root/shared".into(),
                    order: 2,
                    organizational: false,
                    refs: vec![],
                },
            ],
            reference_blocks: HashMap::new(),
            order: vec![],
            edges: vec![],
        };
        let tasks = vec![RunTask {
            key: "task-shared".into(),
            path: "root/shared/op".into(),
            title: "Shared".into(),
            status: "pending".into(),
            protocol_version_id: "protocol-v1".into(),
            source_block_version_id: "block-v1".into(),
            assay_id: "assay-1".into(),
            timepoint: "D1".into(),
            design_step_id: "".into(),
        }];
        let value = build_run(&document, &HashMap::new(), &tasks);
        assert_eq!(value["ambiguous"][0]["key"], "task-shared");
        assert!(value["roots"][0]["children"].is_null());
    }

    #[test]
    fn run_projection_rejects_a_conflicting_design_step_identity() {
        let document = Document {
            steps: vec![
                DesignStep {
                    id: "step-a".into(),
                    title: "A".into(),
                    path: "root/a".into(),
                    order: 1,
                    organizational: false,
                    refs: vec![],
                },
                DesignStep {
                    id: "step-b".into(),
                    title: "B".into(),
                    path: "root/b".into(),
                    order: 2,
                    organizational: false,
                    refs: vec![],
                },
            ],
            reference_blocks: HashMap::new(),
            order: vec![],
            edges: vec![],
        };
        let tasks = vec![RunTask {
            key: "task-conflict".into(),
            path: "root/a".into(),
            title: "A".into(),
            status: "pending".into(),
            protocol_version_id: "protocol-v1".into(),
            source_block_version_id: "block-v1".into(),
            assay_id: "assay-1".into(),
            timepoint: "D1".into(),
            design_step_id: "step-b".into(),
        }];
        let value = build_run(&document, &HashMap::new(), &tasks);
        assert_eq!(value["ambiguous"][0]["key"], "task-conflict");
        assert!(
            value["issues"]
                .as_array()
                .unwrap()
                .iter()
                .any(|issue| issue.as_str().unwrap_or("").contains("different design step"))
        );
        assert!(value["roots"][0]["children"].is_null());
        assert!(value["roots"][1]["children"].is_null());
    }

    #[test]
    fn design_projection_keeps_repeated_block_uses_independent() {
        let document = Document {
            steps: vec![
                DesignStep {
                    id: "use-one".into(),
                    title: "One".into(),
                    path: "root/one".into(),
                    order: 1,
                    organizational: false,
                    refs: vec!["ref-block".into()],
                },
                DesignStep {
                    id: "use-two".into(),
                    title: "Two".into(),
                    path: "root/two".into(),
                    order: 2,
                    organizational: false,
                    refs: vec!["ref-block".into()],
                },
            ],
            reference_blocks: HashMap::from([("ref-block".into(), "block-v1".into())]),
            order: vec!["use-one".into(), "use-two".into()],
            edges: vec![],
        };
        let blocks = HashMap::from([(
            "block-v1".into(),
            Ok(BlockDefinition {
                steps: vec![block_step("root/op", "Op", "block-v1")],
            }),
        )]);
        let value = build_design(&document, &blocks);
        assert_eq!(value["roots"][0]["children"][0]["id"], "root/one/op");
        assert_eq!(value["roots"][1]["children"][0]["id"], "root/two/op");
        assert_eq!(
            value["roots"][0]["children"][0]["blockVersionId"],
            value["roots"][1]["children"][0]["blockVersionId"]
        );
    }

    #[test]
    fn design_projection_reports_a_precedes_cycle() {
        let document = Document {
            steps: vec![],
            reference_blocks: HashMap::new(),
            order: vec![],
            edges: vec![
                ("step-a".into(), "step-b".into()),
                ("step-b".into(), "step-a".into()),
            ],
        };
        let value = build_design(&document, &HashMap::new());
        assert!(
            value["issues"]
                .as_array()
                .unwrap()
                .iter()
                .any(|issue| issue.as_str().unwrap_or("").starts_with("precedes cycle"))
        );
    }
}
