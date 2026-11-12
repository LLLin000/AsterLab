package workflowtree

import (
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
)

// BlockStep is one immutable definition step inside a ProtocolBlock version.
// Path is relative to the block root; SourceBlockVersionID is the exact block
// version that owns the step.
type BlockParameter struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type BlockStep struct {
	UseID                string
	Path                 string
	Title                string
	Instruction          string
	SourceBlockVersionID string
	Parameters           []BlockParameter
	UnresolvedParameters []string
}

// BlockDefinition is the read-only expansion of one ProtocolBlock version.
// It is a definition, not a design use and not a Run occurrence.
type BlockDefinition struct {
	BlockVersionID string
	Ready          bool
	Steps          []BlockStep
}

// BlockProvider resolves an immutable ProtocolBlock version. Implementations
// may read Core, but must never mutate the definition.
type BlockProvider interface {
	BlockDefinition(versionID string) (BlockDefinition, error)
}

// BlockProviderFunc adapts a function to BlockProvider.
type BlockProviderFunc func(versionID string) (BlockDefinition, error)

func (f BlockProviderFunc) BlockDefinition(versionID string) (BlockDefinition, error) {
	return f(versionID)
}

// BuildDesign projects the definition/use hierarchy from a frozen AsterDoc and
// its referenced ProtocolBlock versions. It does not need a Run and does not
// copy block steps into the document. Occurrence status is absent by design.
func BuildDesign(doc *asterdoc.FrozenAsterDoc, provider BlockProvider) (*Tree, error) {
	if doc == nil || strings.TrimSpace(doc.Source) == "" {
		return nil, fmt.Errorf("frozen AsterDoc source is required")
	}
	parsed, err := asterdoc.Parse(doc.Source)
	if err != nil {
		return nil, fmt.Errorf("parse frozen AsterDoc: %w", err)
	}
	tree := &Tree{Precedes: map[string][]string{}, Design: true}

	referenceBlocks := map[string]string{}
	for _, module := range parsed.Modules {
		if module.Kind != "reference" {
			continue
		}
		view, _ := module.Data["view"].(string)
		if view != "protocol_block" {
			continue
		}
		if ref, _ := module.Data["ref"].(string); ref != "" {
			referenceBlocks[module.ID] = ref
		}
	}

	type designStep struct {
		id, title, path string
		order           int
		blockRefs       []string
		organizational  bool
	}
	var steps []designStep
	for _, module := range parsed.Modules {
		if module.Kind != "step" {
			continue
		}
		title, _ := module.Data["title"].(string)
		path, _ := module.Data["path"].(string)
		order := intValue(module.Data["order"])
		execution, _ := module.Data["execution"].(string)
		organizational := module.Schema == 3 && execution == "organizational"
		if organizational && path == "" {
			path = module.ID
		}
		var blockRefs []string
		if !organizational {
			if uses, ok := module.Data["uses"].([]any); ok {
				for _, raw := range uses {
					use, _ := raw.(map[string]any)
					ref, _ := use["ref"].(string)
					if version := referenceBlocks[ref]; version != "" {
						blockRefs = append(blockRefs, version)
					}
				}
			}
		}
		steps = append(steps, designStep{id: module.ID, title: title, path: path, order: order, blockRefs: blockRefs, organizational: organizational})
	}
	sort.SliceStable(steps, func(i, j int) bool {
		if steps[i].order != steps[j].order {
			return steps[i].order < steps[j].order
		}
		return steps[i].path < steps[j].path
	})

	seenPath := map[string]string{}
	for _, step := range steps {
		if step.path == "" {
			tree.Issues = append(tree.Issues, fmt.Sprintf("step %s has no path", step.id))
			continue
		}
		if previous, ok := seenPath[step.path]; ok {
			tree.Issues = append(tree.Issues, fmt.Sprintf("steps %s and %s share path %s", previous, step.id, step.path))
			continue
		}
		seenPath[step.path] = step.id

		node := &Node{
			ID: step.id, Title: step.title, Path: step.path, Kind: KindStep,
			Order: step.order, Identity: IdentityUse, UseID: step.path,
			Status: "", Total: 1, Organizational: step.organizational,
		}
		switch len(step.blockRefs) {
		case 0:
			if step.organizational {
				node.Status = "organizational"
				node.Total = 0
			}
			tree.Roots = append(tree.Roots, node)
			continue
		case 1:
			// Continue below.
		default:
			node.Issues = append(node.Issues, "step references multiple ProtocolBlock definitions")
			tree.Roots = append(tree.Roots, node)
			tree.Issues = append(tree.Issues, node.Issues...)
			continue
		}
		versionID := step.blockRefs[0]
		node.BlockVersionID = versionID
		node.DefinitionRef = versionID
		if provider == nil {
			node.Issues = append(node.Issues, "ProtocolBlock provider is unavailable")
			tree.Roots = append(tree.Roots, node)
			tree.Issues = append(tree.Issues, node.Issues...)
			continue
		}
		definition, err := provider.BlockDefinition(versionID)
		if err != nil {
			node.Issues = append(node.Issues, fmt.Sprintf("ProtocolBlock %s: %v", versionID, err))
			tree.Roots = append(tree.Roots, node)
			tree.Issues = append(tree.Issues, node.Issues...)
			continue
		}
		children, issues := buildDefinitionTrie(step.path, definition.Steps)
		if len(children) == 0 {
			node.Issues = append(node.Issues, "ProtocolBlock has no definition steps")
		} else {
			node.Composite = true
			node.Children = children
			node.Total = countDefinitionLeaves(children)
		}
		node.Issues = append(node.Issues, issues...)
		tree.Roots = append(tree.Roots, node)
		tree.Issues = append(tree.Issues, node.Issues...)
	}

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

type definitionTrie struct {
	segment  string
	path     string
	step     *BlockStep
	children map[string]*definitionTrie
}

func buildDefinitionTrie(prefix string, steps []BlockStep) ([]*Node, []string) {
	root := &definitionTrie{children: map[string]*definitionTrie{}}
	var issues []string
	for i := range steps {
		step := steps[i]
		relative := strings.Trim(strings.TrimPrefix(step.Path, "root/"), "/")
		if relative == "" {
			relative = strings.Trim(strings.TrimPrefix(step.UseID, "root/"), "/")
		}
		if relative == "" {
			issues = append(issues, "ProtocolBlock step has no path or use id")
			continue
		}
		parts := strings.Split(relative, "/")
		current := root
		for j, part := range parts {
			if part == "" {
				continue
			}
			if current.children == nil {
				current.children = map[string]*definitionTrie{}
			}
			next := current.children[part]
			if next == nil {
				next = &definitionTrie{
					segment:  part,
					path:     prefix + "/" + strings.Join(parts[:j+1], "/"),
					children: map[string]*definitionTrie{},
				}
				current.children[part] = next
			}
			current = next
		}
		if current.step != nil {
			issues = append(issues, fmt.Sprintf("ProtocolBlock has duplicate step path %s", step.Path))
			continue
		}
		current.step = &step
	}
	children, childIssues := convertDefinitionTrie(root)
	return children, append(issues, childIssues...)
}

func convertDefinitionTrie(node *definitionTrie) ([]*Node, []string) {
	segments := make([]string, 0, len(node.children))
	for segment := range node.children {
		segments = append(segments, segment)
	}
	sort.Strings(segments)
	out := make([]*Node, 0, len(segments))
	var issues []string
	for _, segment := range segments {
		child := node.children[segment]
		switch {
		case child.step != nil && len(child.children) == 0:
			step := child.step
			title := strings.TrimSpace(step.Title)
			if title == "" {
				title = child.segment
			}
			out = append(out, &Node{
				ID: child.path, Title: title, Path: child.path, Kind: KindStep,
				Identity: IdentityDefinition, UseID: child.path,
				BlockVersionID:       step.SourceBlockVersionID,
				DefinitionRef:        step.SourceBlockVersionID,
				Instruction:          step.Instruction,
				Parameters:           step.Parameters,
				UnresolvedParameters: step.UnresolvedParameters,
				Status:               "", Total: 1,
			})
		case child.step == nil && len(child.children) > 0:
			nested, nestedIssues := convertDefinitionTrie(child)
			out = append(out, &Node{
				ID: child.path, Title: child.segment, Path: child.path,
				Kind: KindStep, Composite: true, Identity: IdentityDefinition,
				UseID: child.path, Total: countDefinitionLeaves(nested),
				Children: nested,
			})
			issues = append(issues, nestedIssues...)
		default:
			issues = append(issues, fmt.Sprintf("definition %s mixes a step and child steps", child.path))
			nested, nestedIssues := convertDefinitionTrie(child)
			out = append(out, &Node{
				ID: child.path, Title: child.segment, Path: child.path,
				Kind: KindStep, Composite: true, Identity: IdentityDefinition,
				UseID: child.path, Total: countDefinitionLeaves(nested),
				Children: nested,
			})
			issues = append(issues, nestedIssues...)
		}
	}
	return out, issues
}

func countDefinitionLeaves(nodes []*Node) int {
	total := 0
	for _, node := range nodes {
		if len(node.Children) == 0 {
			total++
			continue
		}
		total += countDefinitionLeaves(node.Children)
	}
	return total
}
