// Package protocol presents the existing workflow and block instances as a readable protocol.
package protocol

import (
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/workflow"
)

// State references the Designer's existing document and block instances; it does
// not introduce a second protocol model or claim persistence of protocol edits.
type State struct {
	Document       workflow.WorkflowDocument
	Instances      map[string][]blocks.Instance
	DesignID       string
	Revision       uint64
	SelectedStepID string
	OnSelectStep   func(string)
}

// StepIDs returns valid workflow node IDs in business order, then includes any
// unlisted nodes in their stored order.
func (s *State) StepIDs() []string {
	if s == nil {
		return nil
	}
	valid := make(map[string]bool, len(s.Document.Nodes))
	for _, node := range s.Document.Nodes {
		valid[node.ID] = true
	}
	seen := make(map[string]bool, len(valid))
	ids := make([]string, 0, len(valid))
	for _, id := range s.Document.BusinessOrder {
		if valid[id] && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, node := range s.Document.Nodes {
		if node.ID != "" && !seen[node.ID] {
			ids = append(ids, node.ID)
			seen[node.ID] = true
		}
	}
	return ids
}

func (s *State) selectStep(id string) {
	s.SelectedStepID = id
	if s.OnSelectStep != nil {
		s.OnSelectStep(id)
	}
}

func (s *State) selectedNode() *workflow.Node {
	if s == nil || s.SelectedStepID == "" {
		return nil
	}
	for i := range s.Document.Nodes {
		if s.Document.Nodes[i].ID == s.SelectedStepID {
			return &s.Document.Nodes[i]
		}
	}
	return nil
}
