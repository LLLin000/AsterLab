// Package workflowtree builds a read-only containment projection over the
// frozen AsterDoc step paths and the frozen Run tasks. It does not add a
// second workflow store or a new execution engine.
package workflowtree

import (
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/modules/runs"
)

// Kind distinguishes an AsterDoc step from an executable frozen task.
type Kind string

const (
	KindStep Kind = "step"
	KindTask Kind = "task"
)

// TaskRef is one frozen Run task occurrence under a step.
type TaskRef struct {
	Key               string
	Path              string
	Title             string
	Status            string
	ProtocolVersionID string
}

// Node is one read-only node in the projection. A step node may contain child
// task nodes; a task node has no children.
type Node struct {
	ID            string
	Title         string
	Path          string
	Kind          Kind
	Order         int
	DefinitionRef string
	Status        string
	Completed     int
	Total         int
	Children      []*Node
	Issues        []string
}

// Tree is the projection of one frozen design plus one frozen Run.
type Tree struct {
	Roots    []*Node
	Precedes map[string][]string
	Issues   []string
	Orphans  []TaskRef
}

// Build projects the design steps and frozen tasks. It matches a task to a
// step by exact path or by the composite prefix step.path + "/". It never
// matches by title.
func Build(tasks []runs.RunTask, doc *asterdoc.FrozenAsterDoc) (*Tree, error) {
	if doc == nil || strings.TrimSpace(doc.Source) == "" {
		return nil, fmt.Errorf("frozen AsterDoc source is required")
	}
	parsed, err := asterdoc.Parse(doc.Source)
	if err != nil {
		return nil, fmt.Errorf("parse frozen AsterDoc: %w", err)
	}
	tree := &Tree{Precedes: map[string][]string{}}

	type stepDef struct {
		id    string
		title string
		path  string
		order int
		refs  []string
	}
	var steps []stepDef
	referenceRefs := map[string]string{}
	for _, module := range parsed.Modules {
		if module.Kind == "reference" {
			if ref, _ := module.Data["ref"].(string); ref != "" {
				referenceRefs[module.ID] = ref
			}
		}
	}
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
	}

	// A task belongs to the longest matching step path. If it matches more
	// than one step, it is ambiguous and is reported, not guessed.
	assigned := map[int]bool{}
	for i, step := range steps {
		if step.path == "" {
			continue
		}
		node := &Node{ID: step.id, Title: step.title, Path: step.path, Kind: KindStep, Order: step.order}
		var children []TaskRef
		for taskIndex, task := range tasks {
			path := task.Path
			if path == "" {
				path = taskPath(task.Key)
			}
			matches := path == step.path || strings.HasPrefix(path, step.path+"/")
			if !matches {
				continue
			}
			// Check whether a more specific step also matches this task.
			for otherIndex, other := range steps {
				if otherIndex == i || other.path == "" {
					continue
				}
				if strings.HasPrefix(path, other.path+"/") && len(other.path) > len(step.path) {
					node.Issues = append(node.Issues, fmt.Sprintf("task %s is also under step %s", path, other.id))
				}
			}
			assigned[taskIndex] = true
			children = append(children, TaskRef{Key: task.Key, Path: path, Title: task.Title, Status: task.Status, ProtocolVersionID: task.ProtocolVersionID})
		}
		sort.SliceStable(children, func(i, j int) bool { return children[i].Path < children[j].Path })
		for _, child := range children {
			node.Children = append(node.Children, &Node{
				ID: child.Key, Title: child.Title, Path: child.Path, Kind: KindTask, Status: child.Status,
				DefinitionRef: child.ProtocolVersionID,
			})
		}
		node.Total = len(children)
		node.Completed = countCompleted(children)
		node.Status = aggregate(children)
		if node.Total == 0 {
			node.Issues = append(node.Issues, "no frozen task matches this step path")
			node.Status = "unresolved"
		}
		if node.Total > 0 && hasExactAndPrefix(children, step.path) {
			node.Issues = append(node.Issues, "step mixes an exact task and child tasks")
		}
		if isComposite(node, step.path) {
			for _, ref := range step.refs {
				if blockRef, ok := referenceRefs[ref]; ok {
					node.DefinitionRef = blockRef
					break
				}
			}
			if node.DefinitionRef == "" {
				node.Issues = append(node.Issues, "composite step has no explicit reference to a child definition")
			}
		} else if len(children) == 1 {
			// A leaf's definition is the frozen parent protocol version; the
			// occurrence identity remains the frozen task key.
			node.DefinitionRef = children[0].ProtocolVersionID
		}
		tree.Roots = append(tree.Roots, node)
		tree.Issues = append(tree.Issues, node.Issues...)
	}

	for taskIndex, task := range tasks {
		if assigned[taskIndex] {
			continue
		}
		path := task.Path
		if path == "" {
			path = taskPath(task.Key)
		}
		tree.Orphans = append(tree.Orphans, TaskRef{Key: task.Key, Path: path, Title: task.Title, Status: task.Status, ProtocolVersionID: task.ProtocolVersionID})
	}
	if len(tree.Orphans) > 0 {
		tree.Issues = append(tree.Issues, fmt.Sprintf("%d frozen task(s) have no design step", len(tree.Orphans)))
	}

	// Precedes comes from the workflow edges when present; otherwise from the
	// authored step order. Containment is the tree itself.
	if edges := workflowEdges(parsed); len(edges) > 0 {
		for _, edge := range edges {
			tree.Precedes[edge[0]] = append(tree.Precedes[edge[0]], edge[1])
		}
		if cycle := findCycle(tree.Precedes); cycle != "" {
			tree.Issues = append(tree.Issues, "precedes cycle at "+cycle)
		}
	} else {
		for i := 0; i+1 < len(tree.Roots); i++ {
			tree.Precedes[tree.Roots[i].ID] = append(tree.Precedes[tree.Roots[i].ID], tree.Roots[i+1].ID)
		}
	}
	return tree, nil
}

// Validate returns an error when the projection contains ambiguity, orphans,
// unresolved steps, or a precedence cycle.
func (t *Tree) Validate() error {
	if t == nil {
		return fmt.Errorf("nil workflow tree")
	}
	if len(t.Issues) > 0 {
		return fmt.Errorf("workflow tree has issues: %s", strings.Join(t.Issues, "; "))
	}
	return nil
}

func isComposite(node *Node, stepPath string) bool {
	if len(node.Children) == 0 {
		return false
	}
	for _, child := range node.Children {
		if strings.HasPrefix(child.Path, stepPath+"/") {
			return true
		}
	}
	return false
}

func hasExactAndPrefix(children []TaskRef, stepPath string) bool {
	exact, prefix := false, false
	for _, child := range children {
		if child.Path == stepPath {
			exact = true
		}
		if strings.HasPrefix(child.Path, stepPath+"/") {
			prefix = true
		}
	}
	return exact && prefix
}

func countCompleted(tasks []TaskRef) int {
	count := 0
	for _, task := range tasks {
		if task.Status == "completed" {
			count++
		}
	}
	return count
}

func aggregate(tasks []TaskRef) string {
	if len(tasks) == 0 {
		return "unresolved"
	}
	completed, current, blocked := 0, false, false
	for _, task := range tasks {
		switch task.Status {
		case "completed":
			completed++
		case "in_progress", "current", "started", "active":
			current = true
		case "blocked":
			blocked = true
		}
	}
	switch {
	case blocked:
		return "blocked"
	case completed == len(tasks):
		return "completed"
	case current:
		return "current"
	}
	return "pending"
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

func taskPath(key string) string {
	const marker = `"path":"`
	start := strings.Index(key, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(key[start:], `"`)
	if end < 0 {
		return ""
	}
	return key[start : start+end]
}

func workflowEdges(doc *asterdoc.Document) [][2]string {
	for _, module := range doc.Modules {
		if module.Kind != "workflow" {
			continue
		}
		edges, _ := module.Data["edges"].([]any)
		out := make([][2]string, 0, len(edges))
		for _, raw := range edges {
			edge, _ := raw.(map[string]any)
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
