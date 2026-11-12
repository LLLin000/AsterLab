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

	result, err := inspectTreeFor(ctx, designSvc, runSvc, *designID, *revision, *runID)
	if err != nil {
		return err
	}
	return encoderOutput(result)
}

// inspectTreeFor is the testable projection entry point. It returns the
// contract and never prints.
func inspectTreeFor(ctx context.Context, designSvc runs.DesignService, runSvc runs.RunService, designID string, revision uint64, runID string) (inspectTreeResult, error) {
	if runID != "" {
		result, err := inspectFrozenRun(ctx, designSvc, runSvc, designID, revision, runID)
		if err != nil {
			return inspectTreeResult{}, err
		}
		return result, nil
	}
	return inspectDesign(ctx, designSvc, designID, revision)
}

// inspectDesign builds the hierarchy from a frozen AsterDoc revision without a Run.
func inspectDesign(ctx context.Context, designSvc runs.DesignService, designID string, revision uint64) (inspectTreeResult, error) {
	doc, err := loadAsterDoc(ctx, designSvc, designID, revision)
	if err != nil {
		return inspectTreeResult{}, err
	}
	provider := &protocolBlockProvider{svc: designSvc, cache: map[string]workflowtree.BlockDefinition{}}
	tree, err := workflowtree.BuildDesign(&asterdoc.FrozenAsterDoc{
		DesignID: doc.DesignID, Revision: doc.Revision, Source: doc.Source,
	}, provider)
	if err != nil {
		return inspectTreeResult{}, fmt.Errorf("design %s: %w", designID, err)
	}
	return inspectTreeResult{
		DesignID: doc.DesignID,
		Revision: doc.Revision,
		Design:   true,
		Roots:    tree.Roots,
		Order:    tree.Order,
		Precedes: tree.Precedes,
		Issues:   tree.Issues,
	}, nil
}

// inspectFrozenRun uses the Run's frozen AsterDoc as the sole authority.
// It never reads the latest Design revision to supplement a frozen Run.
func inspectFrozenRun(ctx context.Context, designSvc runs.DesignService, runSvc runs.RunService, designID string, requestedRevision uint64, runID string) (inspectTreeResult, error) {
	snap, err := runSvc.LoadRun(ctx, runID)
	if err != nil {
		return inspectTreeResult{}, fmt.Errorf("run %s: %w", runID, err)
	}
	if snap.DesignID != designID {
		return inspectTreeResult{}, fmt.Errorf("run %s belongs to design %s, not %s", runID, snap.DesignID, designID)
	}
	if requestedRevision > 0 && snap.AsterDoc != nil && snap.AsterDoc.Revision != requestedRevision {
		return inspectTreeResult{}, fmt.Errorf("run %s frozen AsterDoc revision %d does not match requested revision %d",
			runID, snap.AsterDoc.Revision, requestedRevision)
	}
	if snap.AsterDoc == nil || snap.AsterDoc.SourceHash == "" {
		return inspectTreeResult{}, fmt.Errorf("run %s has no frozen AsterDoc reference", runID)
	}
	if snap.AsterDoc.DesignID != designID {
		return inspectTreeResult{}, fmt.Errorf("run %s frozen AsterDoc belongs to design %s, not %s",
			runID, snap.AsterDoc.DesignID, designID)
	}
	frozen, err := asterdoc.LoadFrozenAsterDocAt(ctx, designSvc,
		snap.AsterDoc.DesignID, snap.AsterDoc.Revision, snap.AsterDoc.SourceHash)
	if err != nil {
		return inspectTreeResult{}, fmt.Errorf("run %s frozen document: %w", runID, err)
	}
	if frozen.Revision != snap.AsterDoc.Revision {
		return inspectTreeResult{}, fmt.Errorf("run %s frozen document revision %d does not match reference %d",
			runID, frozen.Revision, snap.AsterDoc.Revision)
	}
	provider := &protocolBlockProvider{svc: designSvc, cache: map[string]workflowtree.BlockDefinition{}}
	designTree, err := workflowtree.BuildDesign(&frozen, provider)
	if err != nil {
		return inspectTreeResult{}, fmt.Errorf("run %s frozen design hierarchy: %w", runID, err)
	}
	runTree, err := workflowtree.Build(snap.Tasks, &frozen)
	if err != nil {
		return inspectTreeResult{}, fmt.Errorf("run %s workflow tree: %w", runID, err)
	}
	enrichRunTree(runTree, designTree, runID)
	issues := append([]string{}, designTree.Issues...)
	issues = append(issues, runTree.Issues...)
	for _, root := range runTree.Roots {
		issues = append(issues, root.Issues...)
	}
	return inspectTreeResult{
		DesignID: designID,
		Revision: frozen.Revision,
		RunID:    runID,
		Design:   false,
		Roots:    runTree.Roots,
		Order:    runTree.Order,
		Precedes: runTree.Precedes,
		Issues:   issues,
	}, nil
}

func loadAsterDoc(ctx context.Context, designSvc runs.DesignService, designID string, revision uint64) (runs.AsterDocSnapshot, error) {
	var doc runs.AsterDocSnapshot
	var err error
	if revision > 0 {
		doc, err = designSvc.LoadAsterDocAt(ctx, designID, revision)
	} else {
		doc, err = designSvc.LoadAsterDoc(ctx, designID)
	}
	if err != nil {
		return doc, fmt.Errorf("design %s: %w", designID, err)
	}
	if doc.Source == "" {
		return doc, fmt.Errorf("design %s has no AsterDoc source", designID)
	}
	return doc, nil
}

// enrichRunTree copies definition metadata from the frozen design hierarchy
// onto Run occurrence nodes. Matching is identity-based: path + assay_id.
// It never guesses from path alone.
func enrichRunTree(runTree, designTree *workflowtree.Tree, runID string) error {
	byPath := map[string][]*workflowtree.Node{}
	var walk func(*workflowtree.Node)
	walk = func(node *workflowtree.Node) {
		if node == nil {
			return
		}
		key := node.Path + "|" + node.AssayID
		byPath[key] = append(byPath[key], node)
		for _, child := range node.Children {
			walk(child)
		}
	}
	for _, root := range designTree.Roots {
		walk(root)
	}
	var apply func(*workflowtree.Node) error
	apply = func(node *workflowtree.Node) error {
		if node == nil {
			return nil
		}
		key := node.Path + "|" + node.AssayID
		defs := byPath[key]
		if len(defs) > 1 {
			node.Issues = append(node.Issues, fmt.Sprintf(
				"run %s: path %s assay %s matches %d design nodes; identity is ambiguous",
				runID, node.Path, node.AssayID, len(defs)))
		} else if len(defs) == 1 {
			def := defs[0]
			if node.Instruction == "" {
				node.Instruction = def.Instruction
			}
			if len(node.Parameters) == 0 {
				node.Parameters = def.Parameters
				node.UnresolvedParameters = def.UnresolvedParameters
			}
			if node.UseID == "" {
				node.UseID = def.UseID
			}
			if node.DefinitionRef == "" {
				node.DefinitionRef = def.DefinitionRef
			}
			if node.BlockVersionID == "" {
				node.BlockVersionID = def.BlockVersionID
			}
		}
		for _, child := range node.Children {
			if err := apply(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range runTree.Roots {
		if err := apply(root); err != nil {
			return err
		}
	}
	return nil
}

func encoderOutput(result inspectTreeResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}
