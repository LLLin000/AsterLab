package protocol

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/workflow"

	"github.com/egoist/mygo/ui"
)

// View renders the existing workflow in protocol order with read-only details.
func View(c *ui.Context, s *State) {
	if s == nil {
		return
	}
	t := c.Theme()
	ids := s.StepIDs()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		ui.Row(c).Height(34).Padding(0, 12).Gap(8).Children(func() {
			ui.Text(c, "Protocol").FontSize(20).Bold().TextColor(t.Text)
			ui.Text(c, fmt.Sprintf("Design %s · revision %d", display(s.DesignID, "unlinked"), s.Revision)).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Divider(c)
		ui.Row(c).Grow(1).Children(func() {
			ui.Column(c).Width(224).FillHeight().Padding(12).Gap(4).Background(t.Surface).Children(func() {
				ui.Text(c, "STEPS").FontSize(11).TextColor(t.TextMuted)
				if len(ids) == 0 {
					ui.Text(c, "No workflow steps in this design.").FontSize(13).TextColor(t.TextMuted)
				}
				for i, id := range ids {
					node := findNode(s.Document.Nodes, id)
					if node == nil {
						continue
					}
					label := fmt.Sprintf("%02d  %s", i+1, display(node.Title, string(node.Kind)))
					if id == s.SelectedStepID {
						label = "• " + label
					}
					if ui.Button(c, label).Clicked() {
						s.selectStep(id)
					}
				}
			})
			ui.Divider(c)
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).FillWidth().Padding(20).Gap(12).Children(func() {
				node := s.selectedNode()
				if node == nil {
					ui.Text(c, "Select a step to inspect its protocol details.").FontSize(14).TextColor(t.TextMuted)
					return
				}
				instances := s.Instances[node.ID]
				ui.Text(c, display(node.Title, "Untitled step")).FontSize(20).Bold().TextColor(t.Text)
				ui.Text(c, fmt.Sprintf("%s · step %s", display(string(node.Kind), "step"), node.ID)).FontSize(12).TextColor(t.TextMuted)
				section(c, "Purpose")
				hasPurpose := false
				for _, inst := range instances {
					if inst.Type != blocks.TypeNote {
						continue
					}
					var p blocks.NoteProps
					if inst.DecodeProps(&p) == nil && p.Text != "" {
						if p.Title != "" {
							ui.Text(c, p.Title).FontSize(13).Bold()
						}
						ui.Text(c, p.Text).FontSize(13)
						hasPurpose = true
					}
				}
				if !hasPurpose {
					ui.Text(c, "Purpose is not recorded for this step.").FontSize(13).TextColor(t.TextMuted)
				}

				section(c, "Parameters")
				hasParameters := false
				for _, inst := range instances {
					if inst.Type != blocks.TypeParameterTable {
						continue
					}
					var p blocks.ParameterTableProps
					if inst.DecodeProps(&p) != nil {
						continue
					}
					for _, row := range p.Rows {
						ui.Row(c).Gap(12).Children(func() {
							ui.Text(c, row.Key).Width(180).FontSize(13)
							ui.Text(c, strings.TrimSpace(row.Value+" "+row.Unit)).FontSize(13).Bold()
						})
						hasParameters = true
					}
				}
				if !hasParameters {
					ui.Text(c, "No structured parameters recorded.").FontSize(13).TextColor(t.TextMuted)
				}

				section(c, "Materials, reagents & instruments")
				ui.Text(c, fmt.Sprintf("Workflow item: %s (%s)", display(node.Title, "Untitled"), display(string(node.Kind), "untyped"))).FontSize(13)
				if len(node.ComponentRefs) > 0 {
					ui.Text(c, "Linked component IDs: "+strings.Join(node.ComponentRefs, ", ")).FontSize(13)
				} else {
					ui.Text(c, "No component IDs linked to this step.").FontSize(13).TextColor(t.TextMuted)
				}
				ui.Text(c, "本步骤未单独列出结构化试剂或仪器条目。").FontSize(12).TextColor(t.TextMuted)

				section(c, "Checks")
				hasChecks := false
				for _, inst := range instances {
					if inst.Type != blocks.TypeChecklist {
						continue
					}
					var p blocks.ChecklistProps
					if inst.DecodeProps(&p) != nil {
						continue
					}
					for _, item := range p.Items {
						mark := "○"
						if item.Checked {
							mark = "✓"
						}
						ui.Text(c, mark+"  "+item.Text).FontSize(13)
						hasChecks = true
					}
				}
				if !hasChecks {
					ui.Text(c, "No checks recorded.").FontSize(13).TextColor(t.TextMuted)
				}

				section(c, "Expected outputs")
				ui.Text(c, "本步骤未声明结构化预期产物。").FontSize(13).TextColor(t.TextMuted)

				section(c, "Recorded measurements")
				hasMeasurements := false
				for _, inst := range instances {
					if inst.Type != blocks.TypeMeasurementTable {
						continue
					}
					var p blocks.MeasurementTableProps
					if inst.DecodeProps(&p) != nil {
						continue
					}
					for _, row := range p.Rows {
						ui.Text(c, fmt.Sprintf("%s: %g %s", row.SampleID, row.Value, row.Unit)).FontSize(13)
						hasMeasurements = true
					}
				}
				if !hasMeasurements {
					ui.Text(c, "No measurements recorded.").FontSize(13).TextColor(t.TextMuted)
				}

				section(c, "Design link & version")
				ui.Text(c, fmt.Sprintf("%s · revision %d · workflow node %s", display(s.DesignID, "No linked design ID"), s.Revision, node.ID)).FontSize(13)
				ui.Text(c, "实验方案页为只读视图；如需调整方案，请前往「实验设计」页。").FontSize(12).TextColor(t.TextMuted)
			})
		})
		})
	})
}

func section(c *ui.Context, title string) {
	ui.Text(c, title).FontSize(15).Bold()
}

func findNode(nodes []workflow.Node, id string) *workflow.Node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func display(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
