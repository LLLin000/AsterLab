package protocol

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/blocks"
	"asterlab-mygo-prototype/internal/blockui"
	"asterlab-mygo-prototype/internal/workflow"
)

// Document is a read-only projection of one frozen design revision: what a
// reader needs to understand and repeat the experiment, with every gap named.
// It copies no fact and owns no persistence.
type Document struct {
	DesignID          string
	Revision          uint64
	Title             string
	Objective         string
	ResearchQuestion  string
	ExperimentalUnit  string
	Groups            []string
	Steps             []Step
	Missing           []string
	PlateWellCount    int
	MeasurementFields []string
}

// Step is one workflow step as a reader sees it.
type Step struct {
	ID         string
	Title      string
	Kind       string
	AssayID    string
	Timepoints []string
	Params     []string
	Modules    []string
	Notes      []string
}

type extension struct {
	Workflow   workflow.WorkflowDocument    `json:"workflow"`
	Instances  map[string][]blocks.Instance `json:"instances"`
	StepAssays map[string]string            `json:"stepAssays"`
	StepTasks  []struct {
		StepID    string `json:"stepId"`
		AssayID   string `json:"assayId"`
		Timepoint string `json:"timepoint"`
		Path      string `json:"path"`
	} `json:"stepTasks"`
	Assays map[string]assays.Design `json:"assays"`
}

type designSnapshot struct {
	Title            string `json:"title"`
	Objective        string `json:"objective"`
	ResearchQuestion string `json:"research_question"`
	ExperimentalUnit string `json:"experimental_unit"`
	Groups           []struct {
		Name string `json:"name"`
	} `json:"groups"`
	Assays []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"assays"`
	MyGo extension `json:"mygo"`
}

// Build projects a design snapshot's Data at the given revision. The output is
// deterministic for one input; missing facts become Missing entries instead of
// being filled from names, counts or assumptions.
func Build(data, designID string, revision uint64) (Document, error) {
	doc := Document{DesignID: designID, Revision: revision}
	if strings.TrimSpace(data) == "" {
		doc.Missing = append(doc.Missing, "没有冻结设计数据")
		return doc, nil
	}
	var snap designSnapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return doc, fmt.Errorf("无法读取冻结设计: %w", err)
	}
	doc.Title = strings.TrimSpace(snap.Title)
	doc.Objective = strings.TrimSpace(snap.Objective)
	doc.ResearchQuestion = strings.TrimSpace(snap.ResearchQuestion)
	doc.ExperimentalUnit = strings.TrimSpace(snap.ExperimentalUnit)
	for _, group := range snap.Groups {
		if name := strings.TrimSpace(group.Name); name != "" {
			doc.Groups = append(doc.Groups, name)
		}
	}
	sort.Strings(doc.Groups)

	assayTitles := map[string]string{}
	for _, assay := range snap.Assays {
		assayTitles[assay.ID] = assay.Title
	}
	ordered := snap.MyGo.Workflow.BusinessOrder
	if len(ordered) == 0 {
		for _, node := range snap.MyGo.Workflow.Nodes {
			ordered = append(ordered, node.ID)
		}
	}
	nodes := map[string]workflow.Node{}
	for _, node := range snap.MyGo.Workflow.Nodes {
		nodes[node.ID] = node
	}
	for _, id := range ordered {
		node, ok := nodes[id]
		if !ok {
			continue
		}
		step := Step{
			ID:         node.ID,
			Title:      strings.TrimSpace(node.Title),
			Kind:       string(node.Kind),
			AssayID:    snap.MyGo.StepAssays[node.ID],
			Timepoints: append([]string(nil), snap.MyGo.Assays[snap.MyGo.StepAssays[node.ID]].Timepoints...),
		}
		if step.Title == "" {
			step.Title = "未命名步骤"
			doc.Missing = append(doc.Missing, "步骤 "+node.ID+" 没有名称")
		}
		if step.AssayID == "" {
			doc.Missing = append(doc.Missing, fmt.Sprintf("步骤「%s」未关联检测", step.Title))
		} else {
			assay := snap.MyGo.Assays[step.AssayID]
			if len(assay.Timepoints) == 0 {
				doc.Missing = append(doc.Missing, fmt.Sprintf("检测 %s 未设置时间点", step.AssayID))
			}
			if assay.PlateInstanceID != "" {
				step.Params = append(step.Params, "孔板实例 "+assay.PlateInstanceID)
			}
			if assay.TechnicalReplicates > 0 {
				step.Params = append(step.Params, fmt.Sprintf("技术重复 %d", assay.TechnicalReplicates))
			}
			if assay.ControlGroup != "" {
				step.Params = append(step.Params, "对照 "+assay.ControlGroup)
			}
			if assay.CalibratorGroup != "" {
				step.Params = append(step.Params, "校准 "+assay.CalibratorGroup)
			}
			if len(assay.Wells) > 0 && doc.PlateWellCount == 0 {
				doc.PlateWellCount = uniqueWells(assay.Wells)
			}
		}
		// Modules are listed by their definition, not by guessed science.
		for _, inst := range snap.MyGo.Instances[node.ID] {
			step.Modules = append(step.Modules, blockui.TypeTitle(inst.Type))
			if inst.Type == blocks.TypeNote {
				var note blocks.NoteProps
				if inst.DecodeProps(&note) == nil {
					text := strings.TrimSpace(note.Text)
					if note.Title != "" {
						text = strings.TrimSpace(note.Title + "：" + text)
					}
					if text != "" {
						step.Notes = append(step.Notes, text)
					}
				}
			}
		}
		doc.Steps = append(doc.Steps, step)
	}
	if len(doc.Steps) == 0 {
		doc.Missing = append(doc.Missing, "设计没有工作流步骤")
	}
	if doc.Objective == "" {
		doc.Missing = append(doc.Missing, "未填写实验目的")
	}
	if doc.ExperimentalUnit == "" {
		doc.Missing = append(doc.Missing, "未填写实验单位")
	}
	if len(doc.Groups) == 0 {
		doc.Missing = append(doc.Missing, "未记录分组")
	}
	if doc.PlateWellCount == 0 {
		doc.Missing = append(doc.Missing, "未记录孔板布局")
	}
	sort.Strings(doc.Missing)
	return doc, nil
}

func uniqueWells(wells []assays.WellAssignment) int {
	seen := map[string]bool{}
	for _, well := range wells {
		seen[strings.ToUpper(strings.TrimSpace(well.WellID))] = true
	}
	return len(seen)
}

// Markdown renders the projection as a deterministic document for MyGo's
// markdown view. It formats only recorded fields; missing facts appear in the
// 缺失 section and are never inferred.
func (d Document) Markdown() string {
	var b strings.Builder
	title := strings.TrimSpace(d.Title)
	if title == "" {
		title = "未命名实验"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "**冻结设计 · rev %d**", d.Revision)
	if d.DesignID != "" {
		fmt.Fprintf(&b, " · `%s`", d.DesignID)
	}
	b.WriteString("\n\n")
	if d.Objective != "" {
		fmt.Fprintf(&b, "%s\n\n", d.Objective)
	}
	if d.ResearchQuestion != "" {
		fmt.Fprintf(&b, "研究问题：%s\n\n", d.ResearchQuestion)
	}
	if d.ExperimentalUnit != "" {
		fmt.Fprintf(&b, "实验单位：%s\n\n", d.ExperimentalUnit)
	}
	if len(d.Missing) > 0 {
		b.WriteString("## 缺失与未记录\n\n")
		for _, missing := range d.Missing {
			fmt.Fprintf(&b, "- %s\n", missing)
		}
		b.WriteString("\n")
	}
	if len(d.Groups) > 0 || d.PlateWellCount > 0 {
		b.WriteString("## 分组与板布局\n\n")
		if len(d.Groups) > 0 {
			fmt.Fprintf(&b, "- 分组：%s\n", strings.Join(d.Groups, "、"))
		}
		if d.PlateWellCount > 0 {
			fmt.Fprintf(&b, "- 孔板布局：%d 个计划孔位\n", d.PlateWellCount)
		}
		b.WriteString("\n")
	}
	if len(d.Steps) > 0 {
		b.WriteString("## 步骤\n\n")
	}
	for i, step := range d.Steps {
		fmt.Fprintf(&b, "### %02d  %s\n\n", i+1, step.Title)
		meta := []string{}
		if step.Kind != "" {
			meta = append(meta, step.Kind)
		}
		if step.AssayID != "" {
			meta = append(meta, "检测 "+step.AssayID)
		}
		if len(step.Timepoints) > 0 {
			meta = append(meta, "时间点 "+strings.Join(step.Timepoints, ", "))
		}
		if len(meta) > 0 {
			fmt.Fprintf(&b, "%s\n\n", strings.Join(meta, " · "))
		}
		for _, param := range step.Params {
			fmt.Fprintf(&b, "- %s\n", param)
		}
		if len(step.Params) > 0 {
			b.WriteString("\n")
		}
		if len(step.Modules) > 0 {
			fmt.Fprintf(&b, "模块：%s\n\n", strings.Join(step.Modules, "、"))
		}
		for _, note := range step.Notes {
			fmt.Fprintf(&b, "> %s\n>\n", note)
		}
		if len(step.Notes) > 0 {
			b.WriteString("\n")
		}
		if step.AssayID == "" {
			b.WriteString("> 未关联检测；该步骤没有可列出的测量计划。\n\n")
		}
	}
	return b.String()
}
