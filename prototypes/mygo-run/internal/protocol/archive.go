// Package protocol archive builds the 终态归档: the frozen, exportable record
// of one Run. The archive renders the same RecordProjection the Notebook
// board and the Run history read from, plus the signed prose with its dynamic
// references flattened, so the exported package stands alone without the app.
package protocol

import (
	"fmt"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
)

// Archive is the exportable end state of one Run's record.
type Archive struct {
	Projection record.RecordProjection
	Prose      string
	// SourceRefs remembers the dynamic references the prose cited, so the
	// representative matrix slice follows what the author actually referenced.
	SourceRefs []record.MatrixRef
	Signature  *record.Signature
	Derived    record.DerivedDesign
	Sealed     bool
	ExportedAt time.Time
}

// BuildArchive assembles the archive from the shared projection and the
// current prose. Dynamic `@matrix(...)` references are expanded against the
// projection; prose without references passes through untouched.
func BuildArchive(proj record.RecordProjection, prose string, now time.Time) Archive {
	sig, ok := record.Verify(prose)
	arch := Archive{
		Projection: proj,
		Prose:      record.ExpandMatrixRefs(prose, proj),
		SourceRefs: record.FindMatrixRefs(prose),
		Derived:    record.DeriveDesign(proj),
		Sealed:     ok,
		ExportedAt: now,
	}
	if ok {
		arch.Signature = &sig
	}
	return arch
}

// Markdown renders the archive package: provenance header, planned-vs-actual
// summary, representative matrix slice, signed prose, derived parameters.
func (arch Archive) Markdown() string {
	proj := arch.Projection
	var sb strings.Builder
	sb.WriteString("# 实验记录归档\n\n")
	sb.WriteString(fmt.Sprintf("- Run：%s\n", displayValue(proj.RunID)))
	sb.WriteString(fmt.Sprintf("- 设计：%s · 冻结 rev %d\n", displayValue(proj.DesignID), proj.DesignRevision))
	sb.WriteString(fmt.Sprintf("- 状态：%s\n", displayValue(proj.Status)))
	sb.WriteString(fmt.Sprintf("- 导出时间：%s\n", arch.ExportedAt.UTC().Format(time.RFC3339)))
	if arch.Sealed && arch.Signature != nil {
		sb.WriteString(fmt.Sprintf("- 签核：%s · %s · %s\n",
			arch.Signature.Actor, arch.Signature.At, arch.Signature.Hash[:16]))
	} else {
		sb.WriteString("- 签核：未签核\n")
	}
	sb.WriteString(fmt.Sprintf("- 任务完成：%d/%d · 实测值 %d 个 · 缺 %d 个 FOV 槽\n\n",
		proj.CompletedTasks(), len(proj.Summary), proj.MeasurementCount(), proj.Gaps))

	sb.WriteString("## 计划 vs 实际\n\n")
	if len(proj.Summary) == 0 {
		sb.WriteString("无冻结任务。\n\n")
	}
	for _, sum := range proj.Summary {
		sb.WriteString(fmt.Sprintf("### %s\n\n", taskTitle(sum)))
		if strings.TrimSpace(sum.Timepoint) != "" {
			sb.WriteString(fmt.Sprintf("- 时间点：%s\n", sum.Timepoint))
		}
		sb.WriteString(fmt.Sprintf("- 状态：%s\n", displayValue(sum.Status)))
		if sum.Skipped {
			sb.WriteString("- 条件跳过：按规则跳过（skipped_by_rule）\n")
		}
		for _, dev := range sum.Deviations {
			sb.WriteString(fmt.Sprintf("- 偏差：%s\n", dev))
		}
		if len(sum.Planned) > 0 || len(sum.Actual) > 0 {
			sb.WriteString("\n| 参数 | 计划 | 实际 |\n| --- | --- | --- |\n")
			rows := paramRows(sum)
			for _, row := range rows {
				sb.WriteString(row + "\n")
			}
			sb.WriteString("\n")
		}
		if len(sum.Measurements) > 0 {
			sb.WriteString("实测：\n\n")
			for _, m := range sum.Measurements {
				sb.WriteString(fmt.Sprintf("- %s%s：%s %s\n",
					measureEndpoint(m), measureSample(m), m.Value, m.Unit))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("## 代表视野矩阵\n\n")
	if len(proj.Matrix) == 0 {
		sb.WriteString("无实测孔位数据。\n\n")
	} else {
		ref := record.MatrixRef{All: true}
		if len(arch.SourceRefs) > 0 {
			ref = arch.SourceRefs[0]
		}
		sb.WriteString(record.RenderMatrixTable(proj, ref))
		sb.WriteString("\n")
		for _, cell := range proj.Matrix {
			if filled := cell.FilledSlots(); filled > 0 || cell.Overflow > 0 {
				sb.WriteString(fmt.Sprintf("- %s · %s：图 %d/%d",
					cell.Well, cell.Timepoint, filled, record.FOVSlots))
				if cell.Overflow > 0 {
					sb.WriteString(fmt.Sprintf("（另有 %d 张超出 6 视野槽）", cell.Overflow))
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	if len(proj.UnassignedLabels) > 0 {
		sb.WriteString(fmt.Sprintf("未定位图 %d 张（未命名可解析的任务与样本，不归入视野槽）：\n\n",
			proj.UnassignedImages))
		for _, label := range proj.UnassignedLabels {
			sb.WriteString("- " + label + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 研判记录\n\n")
	if strings.TrimSpace(stripSignatureBlock(arch.Prose)) == "" {
		sb.WriteString("（无研判正文）\n\n")
	} else {
		sb.WriteString(strings.TrimSpace(stripSignatureBlock(arch.Prose)) + "\n\n")
	}

	sb.WriteString("## 派生设计参数\n\n")
	if len(arch.Derived.Parameters) == 0 {
		sb.WriteString("无实测参数可派生。\n")
	} else {
		sb.WriteString(fmt.Sprintf("草案：%s（源自 %s · 设计 rev %d）\n\n",
			arch.Derived.Title, arch.Derived.SourceRunID, arch.Derived.SourceDesignRevision))
		sb.WriteString("| 任务 | 参数 | 计划 | 实测 | 来源 |\n| --- | --- | --- | --- | --- |\n")
		for _, param := range arch.Derived.Parameters {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				param.TaskKey, param.Label, param.Planned, param.Measured, param.Source))
		}
	}
	return sb.String()
}

func stripSignatureBlock(prose string) string {
	return record.StripSignature(prose)
}

func displayValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未记录"
	}
	return value
}

func taskTitle(sum record.TaskSummary) string {
	title := strings.TrimSpace(sum.Title)
	if title == "" {
		title = strings.TrimSpace(sum.TaskKey)
	}
	if title == "" {
		title = "未命名任务"
	}
	return title
}

func measureEndpoint(m record.Measure) string {
	if strings.TrimSpace(m.Endpoint) != "" {
		return m.Endpoint
	}
	return "实测值"
}

func measureSample(m record.Measure) string {
	if strings.TrimSpace(m.SampleID) != "" {
		return " · " + m.SampleID
	}
	return ""
}

func paramRows(sum record.TaskSummary) []string {
	actual := map[string]record.Param{}
	order := []string{}
	for _, p := range sum.Actual {
		if _, ok := actual[p.Key]; !ok {
			order = append(order, p.Key)
		}
		actual[p.Key] = p
	}
	planned := map[string]record.Param{}
	for _, p := range sum.Planned {
		planned[p.Key] = p
		if _, ok := actual[p.Key]; !ok {
			order = append(order, p.Key)
		}
	}
	rows := make([]string, 0, len(order))
	for _, key := range order {
		a, p := actual[key], planned[key]
		label := p.Label
		if strings.TrimSpace(label) == "" {
			label = a.Label
		}
		if strings.TrimSpace(label) == "" {
			label = key
		}
		plan, act := p.Value, a.Value
		if strings.TrimSpace(plan) == "" {
			plan = "—"
		}
		if strings.TrimSpace(act) == "" {
			act = "—"
		}
		unit := a.Unit
		if unit == "" {
			unit = p.Unit
		}
		if unit != "" {
			act += " " + unit
		}
		rows = append(rows, fmt.Sprintf("| %s | %s | %s |", label, plan, act))
	}
	return rows
}
