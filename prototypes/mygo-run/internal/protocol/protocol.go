// Package protocol presents the existing workflow and block instances as a readable protocol.
package protocol

import (
	"fmt"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/workflow"

	"github.com/egoist/mygo/ui"
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
// VersionSummary is enough to identify one listed immutable Protocol version;
// it deliberately carries no content that the Core query does not provide.
type VersionSummary struct {
	ID      string
	Title   string
	Version int
}

// RenderVersions lists available versions without exposing raw identifiers.
// A click selects the summary; the caller must resolve its content before
// rendering a document (UX-P2, UX-P3).
func RenderVersions(c *ui.Context, versions []VersionSummary, selectedID string, selectVersion func(string)) {
	for i, version := range versions {
		label := fmt.Sprintf("%s · v%d", display(version.Title, "Protocol"), version.Version)
		// Duplicate titles/versions can exist as separate Protocol records.
		for j := range i {
			if versions[j].Title == version.Title && versions[j].Version == version.Version {
				label = fmt.Sprintf("%s · 方案 %d", label, i+1)
				break
			}
		}
		if version.ID == selectedID {
			label = "✓ " + label
		}
		if ui.Button(c, label).Clicked() && selectVersion != nil {
			selectVersion(version.ID)
		}
	}
}

// RenderVersionState uses the UX contract's state vocabulary. Run identifiers
// are shown only for the frozen object (UX-STATE); preview never says frozen.
func RenderVersionState(c *ui.Context, state, runID string, revision uint64) {
	switch state {
	case "frozen":
		ui.Text(c, fmt.Sprintf("已冻结 · Run %s · revision %d", runID, revision)).FontSize(c.Theme().Rem(.75)).TextColor(c.Theme().TextMuted)
	case "preview":
		ui.Text(c, fmt.Sprintf("预览 · Design revision %d", revision)).FontSize(c.Theme().Rem(.75)).TextColor(c.Theme().TextMuted)
	case "unresolved":
		ui.Text(c, "未解析").FontSize(c.Theme().Rem(.75)).TextColor(c.Theme().TextMuted)
	}
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
