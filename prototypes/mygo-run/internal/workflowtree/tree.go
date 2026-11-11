// Package workflowtree builds a read-only containment projection over the
// frozen AsterDoc steps and the frozen Run tasks. It does not add a second
// workflow store or execution engine.
package workflowtree

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

// Kind distinguishes a step/composite from an executable frozen task.
type Kind string

const (
	KindStep Kind = "step"
	KindTask Kind = "task"
)

// Identity separates the three layers of the hierarchy. A definition is an
// immutable ProtocolBlock version; a use is a design step referencing it; an
// occurrence is a frozen Run task. Paths locate nodes; identity does not.
type Identity string

const (
	IdentityDefinition Identity = "definition"
	IdentityUse        Identity = "use"
	IdentityOccurrence Identity = "occurrence"
)

// TaskRef is one frozen Run task occurrence under a step.
type TaskRef struct {
	Key                  string `json:"key"`
	Path                 string `json:"path"`
	Title                string `json:"title"`
	Status               string `json:"status,omitempty"`
	ProtocolVersionID    string `json:"protocolVersionId,omitempty"`
	SourceBlockVersionID string `json:"sourceBlockVersionId,omitempty"`
}

// Node is one read-only node. A step node may contain child tasks or nested
// composites; a task node has no children.
type Node struct {
	ID                   string           `json:"id"`
	Title                string           `json:"title"`
	Path                 string           `json:"path"`
	Kind                 Kind             `json:"kind"`
	Composite            bool             `json:"composite,omitempty"`
	Order                int              `json:"order,omitempty"`
	Identity             Identity         `json:"identity,omitempty"`
	UseID                string           `json:"useId,omitempty"`
	BlockVersionID       string           `json:"blockVersionId,omitempty"`
	DefinitionRef        string           `json:"definitionRef,omitempty"`
	Instruction          string           `json:"instruction,omitempty"`
	Parameters           []BlockParameter `json:"parameters,omitempty"`
	UnresolvedParameters []string         `json:"unresolvedParameters,omitempty"`
	Status               string           `json:"status,omitempty"`
	Completed            int              `json:"completed,omitempty"`
	Total                int              `json:"total,omitempty"`
	TaskKey              string           `json:"taskKey,omitempty"`
	Children             []*Node          `json:"children,omitempty"`
	Issues               []string         `json:"issues,omitempty"`
}

// Tree is the projection of one frozen design plus one frozen Run.
type Tree struct {
	Roots []*Node `json:"roots"`
	// Order is the business display order from workflow.business_order. It is
	// independent of Precedes and never implies execution dependency.
	Order []string `json:"order"`
	// Precedes is the explicit dependency graph from workflow.edges. Missing
	// edges are not synthesized from Order or authored position.
	Precedes map[string][]string `json:"precedes"`
	// Design is true when the tree was projected from AsterDoc/BlockVersion
	// definitions without a frozen Run. Execution fields are then empty.
	Design    bool      `json:"design"`
	Issues    []string  `json:"issues,omitempty"`
	Orphans   []TaskRef `json:"orphans,omitempty"`
	Ambiguous []TaskRef `json:"ambiguous,omitempty"`
}

// Build projects the design steps and frozen tasks. A task has exactly one
// authoritative step: the unique matching step path. If it matches more than
// one step, it is reported as ambiguous and is not assigned. Titles are never
// used for matching.
func Build(tasks []runs.RunTask, doc *asterdoc.FrozenAsterDoc) (*Tree, error) {
	if doc == nil || strings.TrimSpace(doc.Source) == "" {
		return nil, fmt.Errorf("frozen AsterDoc source is required")
	}
	parsed, err := asterdoc.Parse(doc.Source)
	if err != nil {
		return nil, fmt.Errorf("parse frozen AsterDoc: %w", err)
	}
	tree := &Tree{Precedes: map[string][]string{}}

	referenceRefs := map[string]string{}
	for _, module := range parsed.Modules {
		if module.Kind == "reference" {
			if ref, _ := module.Data["ref"].(string); ref != "" {
				referenceRefs[module.ID] = ref
			}
		}
	}

	type stepDef struct {
		id    string
		title string
		path  string
		order int
		refs  []string
	}
	var steps []stepDef
	for _, module := range parsed.Modules {
		if module.Kind != "step" {
			continue
		}
		title, _ := module.Data["title"].(string)
		path, _ := module.Data["path"].(string)
		order := intValue(module.Data["order"])
		var refs []string
		if uses, ok := module.Data["uses"].([]any); ok {
			for _, raw := range uses {
				use, _ := raw.(map[string]any)
				ref, _ := use["ref"].(string)
				if ref != "" {
					refs = append(refs, ref)
				}
			}
		}
		steps = append(steps, stepDef{id: module.ID, title: title, path: path, order: order, refs: refs})
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].order != steps[j].order {
			return steps[i].order < steps[j].order
		}
		return steps[i].path < steps[j].path
	})

	// Duplicate step paths make containment ambiguous.
	seenPath := map[string]string{}
	validSteps := make([]stepDef, 0, len(steps))
	for _, step := range steps {
		if step.path == "" {
			tree.Issues = append(tree.Issues, fmt.Sprintf("step %s has no path", step.id))
			continue
		}
		if prev, ok := seenPath[step.path]; ok {
			tree.Issues = append(tree.Issues, fmt.Sprintf("steps %s and %s share path %s", prev, step.id, step.path))
			continue
		}
		seenPath[step.path] = step.id
		validSteps = append(validSteps, step)
	}

	// Assign every task to exactly one step before building any node.
	assignments := map[string][]TaskRef{}
	for _, task := range tasks {
		path := taskPath(task)
		ref := TaskRef{
			Key: task.Key, Path: path, Title: task.Title, Status: task.Status,
			ProtocolVersionID: task.ProtocolVersionID, SourceBlockVersionID: task.SourceBlockVersionID,
		}
		var matches []stepDef
		for _, step := range validSteps {
			if path == step.path || strings.HasPrefix(path, step.path+"/") {
				matches = append(matches, step)
			}
		}
		switch len(matches) {
		case 0:
			tree.Orphans = append(tree.Orphans, ref)
		case 1:
			assignments[matches[0].id] = append(assignments[matches[0].id], ref)
		default:
			tree.Ambiguous = append(tree.Ambiguous, ref)
			tree.Issues = append(tree.Issues, fmt.Sprintf("task path %s matches %d design steps", path, len(matches)))
		}
	}
	if len(tree.Orphans) > 0 {
		tree.Issues = append(tree.Issues, fmt.Sprintf("%d frozen task(s) have no design step", len(tree.Orphans)))
	}

	for _, step := range validSteps {
		node := &Node{ID: step.id, Title: step.title, Path: step.path, Kind: KindStep, Order: step.order, Identity: IdentityUse, UseID: step.path}
		var exact, descendants []TaskRef
		for _, task := range assignments[step.id] {
			if task.Path == step.path {
				exact = append(exact, task)
			} else if strings.HasPrefix(task.Path, step.path+"/") {
				descendants = append(descendants, task)
			}
		}
		if len(exact) > 0 && len(descendants) > 0 {
			node.Issues = append(node.Issues, "step mixes an exact task and child tasks")
		}
		switch {
		case len(descendants) > 0:
			node.Composite = true
			node.Children = buildTrie(step.path, descendants)
			node.DefinitionRef = resolveDefinition(step, referenceRefs)
			node.BlockVersionID = node.DefinitionRef
			validateComposite(node, step.path, node.DefinitionRef, descendants)
		case len(exact) > 0:
			for _, task := range exact {
				node.Children = append(node.Children, taskNode(task))
			}
			if len(exact) > 1 {
				node.Issues = append(node.Issues, "multiple exact tasks share one step path")
			}
			node.DefinitionRef = exact[0].ProtocolVersionID
			node.BlockVersionID = exact[0].SourceBlockVersionID
		default:
			node.Issues = append(node.Issues, "no frozen task matches this step path")
		}
		aggregateNode(node)
		tree.Roots = append(tree.Roots, node)
		tree.Issues = append(tree.Issues, node.Issues...)
	}

	// Display order and dependency are independent contracts. business_order
	// orders the rail/canvas; only workflow.edges declares execution
	// dependency. Never synthesize edges from display order or authored
	// position, especially for branched or parallel workflows.
	tree.Order = businessOrder(parsed)
	tree.Roots = orderRoots(tree.Roots, tree.Order)
	for _, edge := range workflowEdges(parsed) {
		tree.Precedes[edge[0]] = append(tree.Precedes[edge[0]], edge[1])
	}
	if cycle := findCycle(tree.Precedes); cycle != "" {
		tree.Issues = append(tree.Issues, "precedes cycle at "+cycle)
	}
	return tree, nil
}

// Validate returns an error when the projection has ambiguity, orphans,
// unresolved steps, a missing definition, or a precedence cycle.
func (t *Tree) Validate() error {
	if t == nil {
		return fmt.Errorf("nil workflow tree")
	}
	if len(t.Issues) > 0 {
		return fmt.Errorf("workflow tree has issues: %s", strings.Join(t.Issues, "; "))
	}
	return nil
}

// HasComposite reports whether the tree has at least one real containment
// level. A single-layer run (one task per step) keeps the legacy rail.
func (t *Tree) HasComposite() bool {
	if t == nil {
		return false
	}
	var walk func(*Node) bool
	walk = func(node *Node) bool {
		if node.Composite {
			return true
		}
		for _, child := range node.Children {
			if walk(child) {
				return true
			}
		}
		return false
	}
	for _, root := range t.Roots {
		if walk(root) {
			return true
		}
	}
	return false
}

type trieNode struct {
	segment  string
	path     string
	tasks    []TaskRef
	children map[string]*trieNode
}

func buildTrie(stepPath string, tasks []TaskRef) []*Node {
	root := &trieNode{children: map[string]*trieNode{}}
	for _, task := range tasks {
		suffix := strings.TrimPrefix(task.Path, stepPath+"/")
		parts := strings.Split(suffix, "/")
		current := root
		for i, part := range parts {
			if part == "" {
				continue
			}
			if current.children == nil {
				current.children = map[string]*trieNode{}
			}
			next := current.children[part]
			if next == nil {
				next = &trieNode{segment: part, path: stepPath + "/" + strings.Join(parts[:i+1], "/"), children: map[string]*trieNode{}}
				current.children[part] = next
			}
			current = next
		}
		current.tasks = append(current.tasks, task)
	}
	return convertTrie(root)
}

func convertTrie(node *trieNode) []*Node {
	segments := make([]string, 0, len(node.children))
	for segment := range node.children {
		segments = append(segments, segment)
	}
	sort.Strings(segments)
	out := make([]*Node, 0, len(segments)+len(node.tasks))
	for _, task := range node.tasks {
		out = append(out, taskNode(task))
	}
	for _, segment := range segments {
		child := node.children[segment]
		if len(child.tasks) > 0 && len(child.children) == 0 {
			for _, task := range child.tasks {
				out = append(out, taskNode(task))
			}
			continue
		}
		childNode := &Node{
			ID: child.path, Title: child.segment, Path: child.path,
			Kind: KindStep, Composite: true, Identity: IdentityUse, UseID: child.path,
		}
		childNode.Children = convertTrie(child)
		if len(child.tasks) > 0 && len(child.children) > 0 {
			childNode.Issues = append(childNode.Issues, "nested node mixes a task and child tasks")
		}
		aggregateNode(childNode)
		out = append(out, childNode)
	}
	return out
}

func taskNode(task TaskRef) *Node {
	title := task.Title
	if title == "" {
		title = lastSegment(task.Path)
	}
	return &Node{
		ID: task.Key, Title: title, Path: task.Path, Kind: KindTask,
		Identity: IdentityOccurrence, BlockVersionID: task.SourceBlockVersionID,
		TaskKey: task.Key,
		Status:  task.Status, DefinitionRef: task.ProtocolVersionID,
		Completed: completedCount(task.Status), Total: 1,
	}
}

func validateComposite(node *Node, stepPath, definitionRef string, descendants []TaskRef) {
	if definitionRef == "" {
		node.Issues = append(node.Issues, "composite step has no explicit reference to a child definition")
	}
	protocolVersions := map[string]bool{}
	for _, task := range descendants {
		if task.ProtocolVersionID != "" {
			protocolVersions[task.ProtocolVersionID] = true
		}
		// Direct child tasks must come from the referenced block version. A
		// deeper task comes from a nested block, whose parent version is not
		// exposed by the current frozen task model; do not guess it.
		if strings.Count(strings.TrimPrefix(task.Path, stepPath+"/"), "/") != 0 {
			continue
		}
		if definitionRef != "" && task.SourceBlockVersionID != "" && task.SourceBlockVersionID != definitionRef {
			node.Issues = append(node.Issues, fmt.Sprintf(
				"child task %s source block %s does not match referenced block %s",
				task.Path, task.SourceBlockVersionID, definitionRef))
		}
	}
	if len(protocolVersions) > 1 {
		node.Issues = append(node.Issues, "composite children reference multiple Protocol versions")
	}
}

func resolveDefinition(step struct {
	id    string
	title string
	path  string
	order int
	refs  []string
}, referenceRefs map[string]string) string {
	for _, ref := range step.refs {
		if block, ok := referenceRefs[ref]; ok {
			return block
		}
	}
	return ""
}

func aggregateNode(node *Node) {
	completed, total := 0, 0
	blocked, current := false, false
	for _, child := range node.Children {
		completed += child.Completed
		total += child.Total
		switch child.Status {
		case "blocked":
			blocked = true
		case "current", "in_progress", "started", "active":
			current = true
		}
	}
	node.Completed, node.Total = completed, total
	switch {
	case total == 0:
		node.Status = "unresolved"
	case blocked:
		node.Status = "blocked"
	case completed == total:
		node.Status = "completed"
	case current:
		node.Status = "current"
	default:
		node.Status = "pending"
	}
}

func completedCount(status string) int {
	if status == "completed" {
		return 1
	}
	return 0
}

func lastSegment(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func taskPath(task runs.RunTask) string {
	if task.Path != "" {
		return task.Path
	}
	var parsed struct {
		Path string `json:"path"`
	}
	if json.Unmarshal([]byte(task.Key), &parsed) != nil {
		return ""
	}
	return parsed.Path
}

func orderRoots(roots []*Node, order []string) []*Node {
	if len(order) == 0 || len(roots) < 2 {
		return roots
	}
	byID := make(map[string]*Node, len(roots))
	for _, root := range roots {
		byID[root.ID] = root
	}
	ordered := make([]*Node, 0, len(roots))
	seen := make(map[string]bool, len(roots))
	for _, id := range order {
		if root := byID[id]; root != nil && !seen[id] {
			ordered = append(ordered, root)
			seen[id] = true
		}
	}
	for _, root := range roots {
		if !seen[root.ID] {
			ordered = append(ordered, root)
		}
	}
	return ordered
}

func businessOrder(doc *asterdoc.Document) []string {
	for _, module := range doc.Modules {
		if module.Kind != "workflow" {
			continue
		}
		raw, _ := module.Data["business_order"].([]any)
		out := make([]string, 0, len(raw))
		for _, value := range raw {
			if id, ok := value.(string); ok && id != "" {
				out = append(out, id)
			}
		}
		return out
	}
	return nil
}

func workflowEdges(doc *asterdoc.Document) [][2]string {
	for _, module := range doc.Modules {
		if module.Kind != "workflow" {
			continue
		}
		raw, _ := module.Data["edges"].([]any)
		out := make([][2]string, 0, len(raw))
		for _, value := range raw {
			edge, _ := value.(map[string]any)
			from, _ := edge["from"].(string)
			to, _ := edge["to"].(string)
			if from != "" && to != "" {
				out = append(out, [2]string{from, to})
			}
		}
		return out
	}
	return nil
}

func findCycle(edges map[string][]string) string {
	state := map[string]int{}
	var visit func(string) string
	visit = func(node string) string {
		switch state[node] {
		case 1:
			return node
		case 2:
			return ""
		}
		state[node] = 1
		for _, next := range edges[node] {
			if found := visit(next); found != "" {
				return found
			}
		}
		state[node] = 2
		return ""
	}
	for node := range edges {
		if found := visit(node); found != "" {
			return found
		}
	}
	return ""
}

func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}
