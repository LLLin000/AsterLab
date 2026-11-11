package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/workflowtree"
	"asterlab-mygo-prototype/modules/runs"
)

// inspectTreeResult is the shared read-only hierarchy contract. The GUI and
// the Agent CLI consume this shape; the Go projection remains the single
// implementation until the contract is moved into Rust Core.
type inspectTreeResult struct {
	DesignID string               `json:"designId"`
	Revision uint64               `json:"revision"`
	RunID    string               `json:"runId,omitempty"`
	Design   bool                 `json:"design"`
	Roots    []*workflowtree.Node `json:"roots"`
	Order    []string             `json:"order"`
	Precedes map[string][]string  `json:"precedes"`
	Issues   []string             `json:"issues"`
}

// runInspectTree is the hidden read-only mode used by the Agent CLI adapter.
// It never writes and never creates a Run.
func runInspectTree(args []string) error {
	fs := flag.NewFlagSet("inspect-tree", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	designID := fs.String("design", "", "design id")
	revision := fs.Uint64("revision", 0, "AsterDoc revision; 0 means latest")
	runID := fs.String("run", "", "optional Run id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *designID == "" {
		return fmt.Errorf("--design is required")
	}
	ctx := context.Background()
	designSvc, err := runs.NewDesignService()
	if err != nil {
		return err
	}
	defer designSvc.Close()
	runSvc, err := runs.NewRunService()
	if err != nil {
		return err
	}
	defer runSvc.Close()

	var doc runs.AsterDocSnapshot
	if *revision > 0 {
		doc, err = designSvc.LoadAsterDocAt(ctx, *designID, *revision)
	} else {
		doc, err = designSvc.LoadAsterDoc(ctx, *designID)
	}
	if err != nil {
		return err
	}
	if doc.Source == "" {
		return fmt.Errorf("design %s has no AsterDoc source", *designID)
	}
	provider := &protocolBlockProvider{svc: designSvc, cache: map[string]workflowtree.BlockDefinition{}}
	designTree, err := workflowtree.BuildDesign(&asterdoc.FrozenAsterDoc{
		DesignID: doc.DesignID, Revision: doc.Revision, Source: doc.Source,
	}, provider)
	if err != nil {
		return err
	}

	tree := designTree
	designMode := true
	resultRevision := doc.Revision
	if *runID != "" {
		snap, loadErr := runSvc.LoadRun(ctx, *runID)
		if loadErr != nil {
			return loadErr
		}
		if snap.AsterDoc == nil || snap.AsterDoc.SourceHash == "" {
			return fmt.Errorf("run %s has no frozen AsterDoc reference", *runID)
		}
		frozen, loadErr := asterdoc.LoadFrozenAsterDocAt(ctx, designSvc,
			snap.AsterDoc.DesignID, snap.AsterDoc.Revision, snap.AsterDoc.SourceHash)
		if loadErr != nil {
			return loadErr
		}
		tree, err = workflowtree.Build(snap.Tasks, &frozen)
		if err != nil {
			return err
		}
		enrichRunTree(tree, designTree)
		designMode = false
		resultRevision = frozen.Revision
	}
	issues := append([]string{}, designTree.Issues...)
	issues = append(issues, tree.Issues...)
	result := inspectTreeResult{
		DesignID: doc.DesignID, Revision: resultRevision, RunID: *runID,
		Design: designMode, Roots: tree.Roots, Order: tree.Order,
		Precedes: tree.Precedes, Issues: issues,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}

// enrichRunTree copies definition metadata onto the matching Run overlay
// nodes. Occurrence identity and status stay owned by the Run.
func enrichRunTree(runTree, designTree *workflowtree.Tree) {
	byPath := map[string]*workflowtree.Node{}
	var walk func(*workflowtree.Node)
	walk = func(node *workflowtree.Node) {
		if node == nil {
			return
		}
		byPath[node.Path] = node
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range designTree.Roots {
		walk(root)
	}
	var apply func(*workflowtree.Node)
	apply = func(node *workflowtree.Node) {
		if node == nil {
			return
		}
		if definition := byPath[node.Path]; definition != nil {
			if node.Instruction == "" {
				node.Instruction = definition.Instruction
			}
			if len(node.Parameters) == 0 {
				node.Parameters = definition.Parameters
				node.UnresolvedParameters = definition.UnresolvedParameters
			}
			if node.UseID == "" {
				node.UseID = definition.UseID
			}
			if node.DefinitionRef == "" {
				node.DefinitionRef = definition.DefinitionRef
			}
		}
		for _, child := range node.Children {
			apply(child)
		}
	}
	for _, root := range runTree.Roots {
		apply(root)
	}
}
