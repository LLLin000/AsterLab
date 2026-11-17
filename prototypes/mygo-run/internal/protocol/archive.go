// Package protocol archive builds the 终态归档: the frozen, exportable record
// of one Run. The archive renders the same RecordProjection the Notebook
// board and the Run history read from, plus the signed prose with its dynamic
// references flattened, so the exported package stands alone without the app.
package protocol

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/notebook/record"
	"github.com/jung-kurt/gofpdf"
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
	if arch.ExportedAt.IsZero() {
		sb.WriteString("- 导出时间：未导出（预览）\n")
	} else {
		sb.WriteString(fmt.Sprintf("- 导出时间：%s\n", arch.ExportedAt.UTC().Format(time.RFC3339)))
	}
	if arch.Sealed && arch.Signature != nil {
		sb.WriteString(fmt.Sprintf("- 签核：%s · %s · %s\n",
			arch.Signature.Actor, arch.Signature.At, arch.Signature.Hash[:16]))
	} else {
		sb.WriteString("- 签核：未签核\n")
	}
	// The count covers matrix-cell values only: the same fact can also appear
	// in per-task measurements, and the two cannot be deduplicated safely, so
	// the label stays narrow instead of claiming every measurement.
	sb.WriteString(fmt.Sprintf("- 任务完成：%d/%d · 矩阵实测值 %d 个 · 缺 %d 个 FOV 槽\n\n",
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
	if len(proj.UnassignedMeasurements) > 0 {
		sb.WriteString(fmt.Sprintf("未定位实测 %d 条（无任务认领：未知任务键，或步骤路径无歧义匹配；列出但不归入任务行）：\n\n",
			len(proj.UnassignedMeasurements)))
		for _, m := range proj.UnassignedMeasurements {
			sb.WriteString(fmt.Sprintf("- %s%s：%s %s\n",
				measureEndpoint(m), measureSample(m), m.Value, m.Unit))
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

// recordPDFFontCandidates mirrors the CJK search in internal/asterdoc/pdf.go:
// the record PDF needs the same system font for Chinese output. Unlike the
// frozen-document export, a missing font is not fatal here: the PDF still
// builds with Helvetica (Chinese glyphs degrade) so the image board and the
// parameter facts always export. Windows Gate machines carry simhei.ttf.
var recordPDFFontCandidates = []string{
	"C:/Windows/Fonts/simhei.ttf",
	"C:/Windows/Fonts/simsun.ttc",
	"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
}

// ImageLoader returns PNG bytes for one placed matrix image. The caller wires
// it to assets.Roots.Resolve plus a decode/re-encode step (TIFF masters
// decode through the shared preview decoder, gofpdf embeds PNG). Any error
// aborts the export with the asset named: images are never silently dropped.
type ImageLoader func(record.MatrixImage) ([]byte, error)

// PDF renders the same end state as Markdown plus an embedded image board
// from the same projection: provenance header, planned-vs-actual comparison,
// signed/expanded prose, and the representative Well x Timepoint x FOV
// images. Output is deterministic for identical inputs (fixed PDF stamps;
// pass the same ExportedAt): only the file bytes differ when facts change.
func (arch Archive) PDF(w io.Writer, load ImageLoader) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCreationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetModificationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetMargins(16, 16, 16)
	pdf.SetAutoPageBreak(true, 16)
	font := "Helvetica"
	for _, path := range recordPDFFontCandidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// Single-style registration only: headings are sized, not bolded,
		// so a fontless machine falls back to core Helvetica (which always
		// defines "") with no dangling family/weight reference.
		pdf.AddUTF8FontFromBytes("RecordCJK", "", data)
		if err := pdf.Error(); err != nil {
			return fmt.Errorf("record PDF: load CJK font %q: %w", path, err)
		}
		font = "RecordCJK"
		break
	}
	proj := arch.Projection
	pdf.SetTitle("Run record archive "+proj.RunID, true)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont(font, "", 8)
		pdf.CellFormat(0, 8, fmt.Sprintf("Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})
	h1 := func(text string) {
		pdf.SetFont(font, "", 15)
		pdf.MultiCell(0, 8, text, "", "", false)
		pdf.Ln(1)
	}
	h2 := func(text string) {
		pdf.SetFont(font, "", 12)
		pdf.MultiCell(0, 7, text, "", "", false)
	}
	body := func(text string) {
		pdf.SetFont(font, "", 9)
		pdf.MultiCell(0, 5, text, "", "", false)
	}
	pdf.AddPage()
	h1("实验记录归档")
	stamp := "未导出（预览）"
	if !arch.ExportedAt.IsZero() {
		stamp = arch.ExportedAt.UTC().Format(time.RFC3339)
	}
	for _, line := range []string{
		"Run：" + displayValue(proj.RunID),
		"设计：" + displayValue(proj.DesignID) + " · 冻结 rev " + fmt.Sprint(proj.DesignRevision),
		"状态：" + displayValue(proj.Status),
		"导出时间：" + stamp,
	} {
		body(line)
	}
	if arch.Sealed && arch.Signature != nil {
		body("签核：" + arch.Signature.Actor + " · " + arch.Signature.At + " · " + arch.Signature.Hash)
	} else {
		body("签核：未签核")
	}
	body(fmt.Sprintf("任务完成：%d/%d · 矩阵实测值 %d 个 · 缺 %d 个 FOV 槽",
		proj.CompletedTasks(), len(proj.Summary), proj.MeasurementCount(), proj.Gaps))
	pdf.Ln(2)
	h2("计划 vs 实际")
	if len(proj.Summary) == 0 {
		body("无冻结任务。")
	}
	for _, sum := range proj.Summary {
		pdf.SetFont(font, "", 10)
		pdf.MultiCell(0, 6, taskTitle(sum), "", "", false)
		if strings.TrimSpace(sum.Timepoint) != "" {
			body("时间点：" + sum.Timepoint)
		}
		body("状态：" + displayValue(sum.Status))
		if sum.Skipped {
			body("条件跳过：按规则跳过（skipped_by_rule）")
		}
		for _, dev := range sum.Deviations {
			body("偏差：" + dev)
		}
		for _, row := range paramRows(sum) {
			body(row)
		}
		for _, m := range sum.Measurements {
			body("实测：" + measureEndpoint(m) + measureSample(m) + " " + m.Value + " " + m.Unit)
		}
		pdf.Ln(1)
	}
	h2("代表视野矩阵")
	ref := record.MatrixRef{All: true}
	if len(arch.SourceRefs) > 0 {
		ref = arch.SourceRefs[0]
	}
	cells, _ := record.ResolveMatrixRef(proj, ref)
	if len(cells) == 0 {
		body("无实测孔位数据。")
	}
	for _, cell := range cells {
		body(fmt.Sprintf("%s · %s：图 %d/%d", cell.Well, cell.Timepoint, cell.FilledSlots(), record.FOVSlots))
		for fov := 1; fov <= record.FOVSlots; fov++ {
			img := cell.Images[fov-1]
			if !img.Filled {
				continue
			}
			body(fmt.Sprintf("%s · %s · FOV %d — %s", cell.Well, cell.Timepoint, fov, img.Label))
			if load == nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d): no image loader wired", img.AssetID, cell.Well, cell.Timepoint, fov)
			}
			data, err := load(img)
			if err != nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d) unavailable: %w", img.AssetID, cell.Well, cell.Timepoint, fov, err)
			}
			name := "record-" + img.AssetID
			pdf.RegisterImageReader(name, "PNG", bytes.NewReader(data))
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d) rejected: %w", img.AssetID, cell.Well, cell.Timepoint, fov, err)
			}
			cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
			imgW, imgH := 150.0, 100.0
			if err == nil && cfg.Width > 0 && cfg.Height > 0 {
				imgW = 150.0
				imgH = 150.0 * float64(cfg.Height) / float64(cfg.Width)
				if imgH > 90 {
					imgH = 90
					imgW = 90 * float64(cfg.Width) / float64(cfg.Height)
				}
			}
			if pdf.GetY()+imgH+4 > 297-16 {
				pdf.AddPage()
			}
			x := 16.0
			if imgW < 178 {
				x = 16 + (178-imgW)/2
			}
			pdf.Image(name, x, pdf.GetY(), imgW, imgH, false, "", 0, "")
			if err := pdf.Error(); err != nil {
				return fmt.Errorf("record PDF image %q (%s · %s · FOV %d) embed failed: %w", img.AssetID, cell.Well, cell.Timepoint, fov, err)
			}
			pdf.Ln(imgH + 2)
		}
	}
	if len(proj.UnassignedLabels) > 0 {
		h2("未定位图")
		for _, label := range proj.UnassignedLabels {
			body(label)
		}
	}
	h2("研判记录")
	for _, line := range strings.Split(strings.TrimSpace(stripSignatureBlock(arch.Prose)), "\n") {
		if strings.TrimSpace(line) == "" {
			pdf.Ln(2)
			continue
		}
		body(line)
	}
	h2("派生设计参数")
	if len(arch.Derived.Parameters) == 0 {
		body("无实测参数可派生。")
	}
	for _, param := range arch.Derived.Parameters {
		body(fmt.Sprintf("%s · %s：计划 %s / 实测 %s（%s）",
			param.TaskKey, param.Label, param.Planned, param.Measured, param.Source))
	}
	return pdf.Output(w)
}
