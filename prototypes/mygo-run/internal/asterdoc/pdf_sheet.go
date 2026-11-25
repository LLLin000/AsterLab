package asterdoc

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/jung-kurt/gofpdf"
	_ "golang.org/x/image/tiff"

	"asterlab-mygo-prototype/internal/theme"
	plateui "asterlab-mygo-prototype/internal/uikit/plate"
)

// DocumentPDFOptions configures full component-level A4 PDF rendering for both
// draft Design documents and hydrated Run experiment records.
type DocumentPDFOptions struct {
	Title            string
	Revision         string
	SourceHash       string
	RunID            string
	IsRunRecord      bool
	StatusLabel      string
	RecordedAt       string
	Plan             string
	RunRecords       string
	BaseDir          string
	HeaderText       string
	IncludeFootnotes bool
	ActualParams     map[string]string
	CalcActuals      *CalcActualContext
	CheckedItems     map[string]bool
	SkippedItems     map[string]string
	TableCells       map[string]map[string]string
	LinkedAssets     map[string][]AssetThumb
	NotebookMarkdown string
}

type fontPairCandidate struct {
	regular string
	bold    string
}

var cjkFontPairCandidates = []fontPairCandidate{
	// Windows
	{regular: "C:/Windows/Fonts/Deng.ttf", bold: "C:/Windows/Fonts/Dengb.ttf"},
	{regular: "C:/Windows/Fonts/simhei.ttf", bold: ""},
	{regular: "C:/Windows/Fonts/simsunb.ttf", bold: ""},
	{regular: "C:/Windows/Fonts/NotoSansSC-VF.ttf", bold: ""},
	{regular: "C:/Windows/Fonts/NotoSansSC-Regular.ttf", bold: "C:/Windows/Fonts/NotoSansSC-Bold.ttf"},
	// macOS
	{regular: "/System/Library/Fonts/Supplemental/Arial Unicode.ttf", bold: ""},
	{regular: "/Library/Fonts/Arial Unicode.ttf", bold: ""},
	{regular: "/Library/Fonts/NotoSansSC-Regular.ttf", bold: "/Library/Fonts/NotoSansSC-Bold.ttf"},
	{regular: "/Library/Fonts/NotoSansCJKsc-Regular.otf", bold: "/Library/Fonts/NotoSansCJKsc-Bold.otf"},
	// Linux
	{regular: "/usr/share/fonts/google-noto-cjk/NotoSansCJKsc-Regular.otf", bold: "/usr/share/fonts/google-noto-cjk/NotoSansCJKsc-Bold.otf"},
	{regular: "/usr/share/fonts/truetype/noto/NotoSansSC-Regular.ttf", bold: "/usr/share/fonts/truetype/noto/NotoSansSC-Bold.ttf"},
	{regular: "/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf", bold: ""},
	{regular: "/usr/share/fonts/wenquanyi/wqy-zenhei/wqy-zenhei.ttf", bold: ""},
	// Android
	{regular: "/system/fonts/NotoSansSC-Regular.ttf", bold: "/system/fonts/NotoSansSC-Bold.ttf"},
	{regular: "/system/fonts/DroidSansFallback.ttf", bold: ""},
}

func isValidSingleSFNT(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	magic := string(data[:4])
	return magic == "\x00\x01\x00\x00" || magic == "OTTO" || magic == "true" || magic == "typ1"
}

func probeFontLoadsCleanly(data []byte) bool {
	if !isValidSingleSFNT(data) {
		return false
	}
	probe := gofpdf.New("P", "mm", "A4", "")
	probe.AddUTF8FontFromBytes("Probe", "", data)
	return probe.Error() == nil
}

func loadSafeCJKFont(pdf *gofpdf.Fpdf) (string, error) {
	registerPair := func(regPath, boldPath string) (string, error) {
		regBytes, err := os.ReadFile(regPath)
		if err != nil || !probeFontLoadsCleanly(regBytes) {
			return "", nil
		}
		pdf.AddUTF8FontFromBytes("AsterCJK", "", regBytes)
		boldBytes := regBytes
		if boldPath != "" {
			if b, bErr := os.ReadFile(boldPath); bErr == nil && probeFontLoadsCleanly(b) {
				boldBytes = b
			}
		}
		pdf.AddUTF8FontFromBytes("AsterCJK", "B", boldBytes)
		if pdf.Error() != nil {
			return "", fmt.Errorf("load CJK font %q: %w", regPath, pdf.Error())
		}
		return "AsterCJK", nil
	}
	for _, cand := range cjkFontPairCandidates {
		loaded, err := registerPair(cand.regular, cand.bold)
		if err != nil {
			return "", err
		}
		if loaded != "" {
			return loaded, nil
		}
	}
	for _, cand := range chineseFontCandidates {
		loaded, err := registerPair(cand, "")
		if err != nil {
			return "", err
		}
		if loaded != "" {
			return loaded, nil
		}
	}
	return "", fmt.Errorf("cannot export Chinese text: no valid CJK TrueType font found")
}

// ensurePDFRoom inserts a page break when neededMM vertical space is not
// available above pageBreakY, preventing headers, tables, and plates from
// splitting awkwardly across page boundaries.
func ensurePDFRoom(pdf *gofpdf.Fpdf, neededMM, pageBreakY float64) bool {
	if neededMM <= 0 {
		return false
	}
	const maxUsableHeight = 250.0
	if neededMM > maxUsableHeight {
		neededMM = maxUsableHeight
	}
	if pdf.GetY()+neededMM > pageBreakY {
		pdf.AddPage()
		return true
	}
	return false
}

// ExportDocumentPDF renders an AsterML v2 document into a publication-grade A4
// multi-page PDF with real vector tables, callout boxes, checklist checkboxes,
// deterministic calculator recipes, high-DPI plate layouts, and inline SVGs/images.
func ExportDocumentPDF(w io.Writer, doc *Document, opts DocumentPDFOptions) error {
	if doc == nil {
		return fmt.Errorf("cannot export PDF: nil AsterML document")
	}
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = extractExperimentTitle(doc)
	}
	if title == "" {
		title = "AsterLab 实验方案"
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCatalogSort(true)
	pdf.SetCreationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	pdf.SetModificationDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	const marginL, marginT, marginR = 15.0, 16.0, 15.0
	const contentW = 180.0
	const pageBreakY = 275.0
	pdf.SetMargins(marginL, marginT, marginR)
	pdf.SetAutoPageBreak(true, 16.0)
	pdf.AliasNbPages("{nb}")

	font := "Helvetica"
	headerText := strings.TrimSpace(opts.HeaderText)
	needsCJK := containsCJK(title, opts.Revision, opts.SourceHash, opts.RunID, []string{doc.Source, opts.Plan, opts.RunRecords, opts.StatusLabel, opts.RecordedAt, opts.NotebookMarkdown, headerText})
	if loaded, err := loadSafeCJKFont(pdf); err == nil {
		font = loaded
	} else if needsCJK {
		return err
	}

	traceability := "Document Revision: " + opts.Revision + "; Source Hash: " + opts.SourceHash
	if opts.RunID != "" {
		traceability += "; Linked Run: " + opts.RunID
	}
	pdf.SetTitle(title, true)
	pdf.SetSubject(traceability, false)

	pdf.SetHeaderFunc(func() {
		if pdf.PageNo() <= 1 && headerText == "" {
			return
		}
		pdf.SetY(7.5)
		pdf.SetFont(font, "", 8)
		pdf.SetTextColor(115, 110, 102)
		hdrTag := "Protocol Design"
		if font != "Helvetica" {
			hdrTag = "实验设计方案"
		}
		if opts.IsRunRecord || opts.RunID != "" {
			if font != "Helvetica" {
				hdrTag = "实验实测记录单"
			} else {
				hdrTag = "Run Record"
			}
		}
		leftHdr := title
		rightHdr := hdrTag
		if headerText != "" {
			if pdf.PageNo() <= 1 {
				leftHdr = headerText
			} else {
				rightHdr = headerText
			}
		}
		pdf.CellFormat(120, 5, leftHdr, "", 0, "L", false, 0, "")
		pdf.CellFormat(60, 5, rightHdr, "", 0, "R", false, 0, "")
		pdf.SetDrawColor(231, 227, 219)
		pdf.SetLineWidth(0.25)
		pdf.Line(marginL, 13.5, marginL+contentW, 13.5)
		pdf.SetY(marginT)
	})

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetDrawColor(231, 227, 219)
		pdf.SetLineWidth(0.25)
		pdf.Line(marginL, 283.5, marginL+contentW, 283.5)
		pdf.SetFont(font, "", 8)
		pdf.SetTextColor(115, 110, 102)
		leftFooter := "AsterLab · AsterML v2 Protocol & Run Sheet"
		if font != "Helvetica" {
			leftFooter = "AsterLab · AsterML v2 实验方案与记录单"
		}
		pdf.CellFormat(110, 6, leftFooter, "", 0, "L", false, 0, "")
		pdf.CellFormat(70, 6, fmt.Sprintf("Page %d / {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	imgSeq := 0

	var hasBrandSeal bool
	if wm := theme.LogoWatermarkPNG(128); len(wm) > 0 {
		info := pdf.RegisterImageOptionsReader("asterlab-brand-seal", gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(wm))
		hasBrandSeal = info != nil
	}

	ensureSpace := func(neededMM float64) {
		ensurePDFRoom(pdf, neededMM, pageBreakY)
	}

	// 1. Top Title & Provenance Card (Bench-sheet ready with printable operator blank if draft)
	titleW := contentW
	if hasBrandSeal {
		titleW = contentW - 11
		pdf.ImageOptions("asterlab-brand-seal", marginL+contentW-6.8, marginT-0.5, 6.4, 5.6, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	}
	pdf.SetTextColor(28, 27, 24)
	pdf.SetFont(font, "B", 16)
	pdf.MultiCell(titleW, 7.5, title, "", "L", false)
	pdf.Ln(1.5)

	metaLines := extractExperimentSummaryLines(doc, opts)
	if len(metaLines) > 0 {
		drawPDFKeyValueCard(pdf, font, marginL, contentW, pageBreakY, metaLines)
		pdf.Ln(3)
	}

	// Collect step block_refs so referenced top-level modules render inside their owning step without duplication.
	stepOwnedRefs := map[string]bool{}
	for _, m := range doc.Modules {
		if m.Kind == "step" {
			for _, ref := range stringList(m.Data, "block_refs") {
				stepOwnedRefs[ref] = true
			}
		}
	}

	// 2. Global Overview (Groups + Materials + Unowned Top-level Blocks)
	renderedGlobalHeader := false
	ensureGlobalHeader := func() {
		if !renderedGlobalHeader {
			ensurePDFRoom(pdf, 26, pageBreakY)
			drawPDFSectionHeading(pdf, font, marginL, contentW, "一、实验设计与材料总览", "")
			renderedGlobalHeader = true
		}
	}

	var standaloneGroups []map[string]any
	var materials []map[string]any
	for _, m := range doc.Modules {
		switch m.Kind {
		case "group":
			standaloneGroups = append(standaloneGroups, m.Data)
		case "material":
			materials = append(materials, m.Data)
		}
	}

	for _, m := range doc.Modules {
		switch m.Kind {
		case "experiment", "assay", "sample", "readout", "instrument_profile", "reference", "workflow", "step", "group", "material":
			continue
		}
		if stepOwnedRefs[m.ID] {
			continue
		}
		ensureGlobalHeader()
		renderModuleToPDF(pdf, font, doc, opts, marginL, contentW, pageBreakY, ensureSpace, &imgSeq, "", m.Kind, m.Data)
	}

	if len(standaloneGroups) > 0 {
		ensureGlobalHeader()
		renderStandaloneGroupsPDF(pdf, font, marginL, contentW, pageBreakY, standaloneGroups)
	}
	if len(materials) > 0 {
		ensureGlobalHeader()
		renderMaterialsTablePDF(pdf, font, marginL, contentW, pageBreakY, materials)
	}

	// 3. Ordered Step Sequence
	var steps []Module
	for _, m := range doc.Modules {
		if m.Kind == "step" {
			steps = append(steps, m)
		}
	}
	if len(steps) > 0 {
		ensurePDFRoom(pdf, 28, pageBreakY)
		stepHeading := "二、实验操作步骤与台账记录表"
		if opts.IsRunRecord {
			stepHeading = "二、实验执行步骤与实测记录"
		}
		drawPDFSectionHeading(pdf, font, marginL, contentW, stepHeading, fmt.Sprintf("共 %d 步", len(steps)))
		for idx, step := range steps {
			execMode := firstNonEmpty(textValue(step.Data, "execution"), "executable")
			stepTitle := firstNonEmpty(textValue(step.Data, "title"), step.ID)
			tps := strings.Join(stringList(step.Data, "timepoints"), " / ")
			ensurePDFRoom(pdf, 26, pageBreakY)
			drawPDFStepBanner(pdf, font, marginL, contentW, idx+1, stepTitle, execMode, tps)

			blocks := mapSlice(step.Data["blocks"])
			for _, bm := range blocks {
				renderModuleToPDF(pdf, font, doc, opts, marginL, contentW, pageBreakY, ensureSpace, &imgSeq, step.ID, textValue(bm, "kind"), bm)
			}
			for _, refID := range stringList(step.Data, "block_refs") {
				for _, tm := range doc.Modules {
					if tm.ID == refID {
						renderModuleToPDF(pdf, font, doc, opts, marginL, contentW, pageBreakY, ensureSpace, &imgSeq, step.ID, tm.Kind, tm.Data)
						break
					}
				}
			}

			// If printable checklist items live directly on step.checks, render them too.
			if checks := mapSlice(step.Data["checks"]); len(checks) > 0 && len(blocks) == 0 {
				drawPDFChecklist(pdf, font, opts, step.ID, marginL, contentW, pageBreakY, ensureSpace, "步骤核对项", checks)
			}
			pdf.Ln(2)
		}
	}

	// 4. Optional Lab Notebook Observations & Embedded Components (Run records)
	if strings.TrimSpace(opts.NotebookMarkdown) != "" {
		ensureSpace(16)
		drawPDFSectionHeading(pdf, font, marginL, contentW, "三、实验观察笔记与阶段结论", "")
		renderNotebookMarkdownToPDF(pdf, font, opts, marginL, contentW, pageBreakY, ensureSpace, &imgSeq, opts.NotebookMarkdown)
		pdf.Ln(2)
	}

	// 5. Optional Plain Appendices (legacy callers)
	if strings.TrimSpace(opts.Plan) != "" {
		ensureSpace(16)
		secTitle := "三、执行计划摘要"
		if strings.TrimSpace(opts.NotebookMarkdown) != "" {
			secTitle = "四、执行计划摘要"
		}
		drawPDFSectionHeading(pdf, font, marginL, contentW, secTitle, "")
		for _, line := range strings.Split(opts.Plan, "\n") {
			renderPDFLine(pdf, font, line)
		}
		pdf.Ln(2)
	}
	if strings.TrimSpace(opts.RunRecords) != "" {
		ensurePDFRoom(pdf, 24, pageBreakY)
		drawPDFSectionHeading(pdf, font, marginL, contentW, "执行明细记录", "")
		for _, line := range strings.Split(opts.RunRecords, "\n") {
			renderPDFLine(pdf, font, line)
		}
	}

	// 6. Optional Parsed Markdown Footnotes Appendix
	if opts.IncludeFootnotes {
		if footnotes := collectPDFFootnotes(doc.Source, opts.NotebookMarkdown); len(footnotes) > 0 {
			renderFootnotesAppendixPDF(pdf, font, marginL, contentW, pageBreakY, footnotes)
		}
	}

	if pdf.Error() != nil {
		return pdf.Error()
	}
	return pdf.Output(w)
}

func extractExperimentTitle(doc *Document) string {
	for _, m := range doc.Modules {
		if m.Kind == "experiment" {
			if t := strings.TrimSpace(textValue(m.Data, "title")); t != "" {
				return t
			}
		}
	}
	return ""
}

func extractExperimentSummaryLines(doc *Document, opts DocumentPDFOptions) [][2]string {
	var rows [][2]string
	for _, m := range doc.Modules {
		if m.Kind != "experiment" {
			continue
		}
		if v := strings.TrimSpace(textValue(m.Data, "objective")); v != "" {
			rows = append(rows, [2]string{"实验目的", v})
		}
		if v := strings.TrimSpace(textValue(m.Data, "hypothesis")); v != "" {
			rows = append(rows, [2]string{"科学假说", v})
		}
		if v := strings.TrimSpace(textValue(m.Data, "experimental_unit")); v != "" {
			rows = append(rows, [2]string{"实验单元", v})
		}
		if v := strings.TrimSpace(textValue(m.Data, "primary_endpoint")); v != "" {
			unit := strings.TrimSpace(textValue(m.Data, "endpoint_unit"))
			method := strings.TrimSpace(textValue(m.Data, "endpoint_method"))
			val := v
			if unit != "" && unit != "1" {
				val += " (" + unit + ")"
			}
			if method != "" {
				val += " · " + method
			}
			rows = append(rows, [2]string{"测定终点", val})
		}
		if sched := NormalizeDateYMD(textValue(m.Data, "scheduled_date")); sched != "" {
			rows = append(rows, [2]string{"计划日期", sched})
		}
		if tps := stringList(m.Data, "timepoints"); len(tps) > 0 {
			rows = append(rows, [2]string{"时间节点", strings.Join(tps, "、")})
		}
		break
	}
	b := ComputeBudgetSummary(doc)
	budgetVal := fmt.Sprintf("%d 步 (%d 可执行) · %d 组 · %d 孔/位", b.Steps, b.ExecutableSteps, b.Groups, b.Wells)
	if b.HandsOnSec > 0 || b.TotalSpanSec > 0 {
		budgetVal += fmt.Sprintf(" · 预计操作 %d min / 总跨度 %d min", max(1, b.HandsOnSec/60), max(1, b.TotalSpanSec/60))
	}
	rows = append(rows, [2]string{"规模概览", budgetVal})
	if opts.IsRunRecord || opts.RunID != "" {
		stVal := firstNonEmpty(opts.StatusLabel, "实验中") + " · 方案快照已锁定"
		if strings.TrimSpace(opts.RecordedAt) != "" {
			stVal += " · 更新于 " + strings.TrimSpace(opts.RecordedAt)
		}
		rows = append(rows, [2]string{"记录状态", stVal})
	} else {
		rows = append(rows, [2]string{"台面签署", "实验日期：____________   操作人：____________"})
	}
	return rows
}

func drawPDFKeyValueCard(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, rows [][2]string) {
	labelW := 28.0
	valW := w - labelW - 6.0
	rowH := make([]float64, len(rows))
	totalH := 4.0
	pdf.SetFont(font, "", 9.0)
	for i, kv := range rows {
		lines := pdf.SplitText(kv[1], valW)
		n := max(1, len(lines))
		h := float64(n)*4.4 + 1.2
		rowH[i] = h
		totalH += h
	}
	if pdf.GetY()+totalH > pageBreakY {
		pdf.AddPage()
	}
	y0 := pdf.GetY()
	pdf.SetFillColor(247, 246, 243)
	pdf.SetDrawColor(214, 208, 197)
	pdf.SetLineWidth(0.3)
	pdf.Rect(x, y0, w, totalH, "FD")
	cy := y0 + 2.2
	for i, kv := range rows {
		pdf.SetXY(x+3, cy)
		pdf.SetFont(font, "B", 8.8)
		pdf.SetTextColor(95, 90, 82)
		pdf.CellFormat(labelW, 4.4, kv[0], "", 0, "L", false, 0, "")
		pdf.SetXY(x+3+labelW, cy)
		pdf.SetFont(font, "", 9.0)
		pdf.SetTextColor(28, 27, 24)
		pdf.MultiCell(valW, 4.4, kv[1], "", "L", false)
		cy += rowH[i]
	}
	pdf.SetY(y0 + totalH)
}

func drawPDFSectionHeading(pdf *gofpdf.Fpdf, font string, x, w float64, title, badge string) {
	y := pdf.GetY()
	pdf.SetFillColor(91, 69, 201)
	pdf.Rect(x, y+0.6, 1.8, 5.2, "F")
	pdf.SetXY(x+4, y)
	pdf.SetFont(font, "B", 11.5)
	pdf.SetTextColor(28, 27, 24)
	pdf.CellFormat(w-40, 6.2, title, "", 0, "L", false, 0, "")
	if badge != "" {
		pdf.SetFont(font, "", 8.5)
		pdf.SetTextColor(95, 90, 82)
		pdf.CellFormat(36, 6.2, badge, "", 0, "R", false, 0, "")
	}
	pdf.SetDrawColor(214, 208, 197)
	pdf.SetLineWidth(0.3)
	pdf.Line(x, y+7.0, x+w, y+7.0)
	pdf.SetY(y + 9.0)
}

func drawPDFStepBanner(pdf *gofpdf.Fpdf, font string, x, w float64, num int, stepTitle, execMode, tps string) {
	y := pdf.GetY()
	pdf.SetFillColor(243, 241, 250)
	pdf.SetDrawColor(214, 208, 197)
	pdf.SetLineWidth(0.25)
	pdf.Rect(x, y, w, 8.0, "FD")

	pdf.SetFillColor(91, 69, 201)
	pdf.Rect(x+1.5, y+1.3, 8.5, 5.4, "F")
	pdf.SetXY(x+1.5, y+1.3)
	pdf.SetFont(font, "B", 8.8)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(8.5, 5.4, fmt.Sprintf("%02d", num), "", 0, "C", false, 0, "")

	stepTitle = stripLeadingStepNumber(stepTitle)
	pdf.SetXY(x+12, y+1.2)
	pdf.SetFont(font, "B", 10.2)
	pdf.SetTextColor(28, 27, 24)
	pdf.CellFormat(w-56, 5.6, stepTitle, "", 0, "L", false, 0, "")

	tag := tps
	if execMode == "organizational" {
		if tag != "" {
			tag = "组织阶段 · " + tag
		} else {
			tag = "组织阶段"
		}
	}
	if tag != "" {
		pdf.SetFont(font, "", 8.2)
		pdf.SetTextColor(91, 69, 201)
		pdf.CellFormat(42, 5.6, tag, "", 0, "R", false, 0, "")
	}
	pdf.SetY(y + 9.8)
}

func renderModuleToPDF(
	pdf *gofpdf.Fpdf,
	font string,
	doc *Document,
	opts DocumentPDFOptions,
	x, w, pageBreakY float64,
	ensureSpace func(float64),
	imgSeq *int,
	stepID string,
	kind string,
	data map[string]any,
) {
	if data == nil {
		return
	}
	switch kind {
	case "prose":
		drawPDFProseCallout(pdf, font, x, w, ensureSpace, textValue(data, "title"), textValue(data, "tone"), textValue(data, "body"))
	case "group_table":
		renderGroupTablePDF(pdf, font, x, w, pageBreakY, data)
	case "param_table":
		renderParamTablePDF(pdf, font, opts, stepID, x, w, pageBreakY, data)
	case "data_table":
		renderDataTablePDF(pdf, font, opts, x, w, pageBreakY, data)
	case "calculator":
		drawPDFCalculator(pdf, font, doc, opts, x, w, pageBreakY, ensureSpace, data)
	case "plate":
		drawPDFPlate(pdf, font, doc, x, w, pageBreakY, imgSeq, data)
	case "figure":
		drawPDFFigure(pdf, font, opts, x, w, pageBreakY, ensureSpace, imgSeq, data)
	case "checklist":
		drawPDFChecklist(pdf, font, opts, stepID, x, w, pageBreakY, ensureSpace, firstNonEmpty(textValue(data, "title"), "操作核对清单"), mapSlice(data["items"]))
	case "subflow":
		renderSubflowPDF(pdf, font, opts, stepID, x, w, pageBreakY, ensureSpace, data)
	case "timer":
		renderTimerPDF(pdf, font, x, w, ensureSpace, data)
	case "attachment":
		renderAttachmentPDF(pdf, font, opts, stepID, x, w, pageBreakY, ensureSpace, imgSeq, data)
	}
}

func drawPDFProseCallout(pdf *gofpdf.Fpdf, font string, x, w float64, ensureSpace func(float64), title, tone, body string) {
	if strings.TrimSpace(title) == "" && strings.TrimSpace(body) == "" {
		return
	}
	pdf.SetFont(font, "", 9.2)
	bodyLines := pdf.SplitText(strings.TrimSpace(body), w-8)
	hasTitle := strings.TrimSpace(title) != ""
	boxH := 3.6 + float64(max(1, len(bodyLines)))*4.4
	if hasTitle {
		boxH += 5.0
	}
	ensureSpace(boxH + 3)

	bgR, bgG, bgB := 250, 249, 247
	barR, barG, barB := 158, 151, 139
	switch tone {
	case "warning":
		bgR, bgG, bgB = 255, 248, 235
		barR, barG, barB = 196, 127, 23
	case "tip":
		bgR, bgG, bgB = 238, 251, 243
		barR, barG, barB = 47, 122, 79
	case "note":
		bgR, bgG, bgB = 243, 240, 255
		barR, barG, barB = 91, 69, 201
	}

	y0 := pdf.GetY()
	pdf.SetFillColor(bgR, bgG, bgB)
	pdf.SetDrawColor(231, 227, 219)
	pdf.SetLineWidth(0.2)
	pdf.Rect(x, y0, w, boxH, "FD")
	pdf.SetFillColor(barR, barG, barB)
	pdf.Rect(x, y0, 1.4, boxH, "F")

	cy := y0 + 2.0
	if hasTitle {
		pdf.SetXY(x+4.2, cy)
		pdf.SetFont(font, "B", 9.4)
		pdf.SetTextColor(barR, barG, barB)
		pdf.CellFormat(w-6, 4.6, title, "", 0, "L", false, 0, "")
		cy += 5.0
	}
	if strings.TrimSpace(body) != "" {
		pdf.SetXY(x+4.2, cy)
		pdf.SetFont(font, "", 9.2)
		pdf.SetTextColor(28, 27, 24)
		pdf.MultiCell(w-7, 4.4, strings.TrimSpace(body), "", "L", false)
	}
	pdf.SetY(y0 + boxH + 2.4)
}

func drawPDFTable(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, title string, headers []string, colWidths []float64, rows [][]string, alignRight []bool) {
	if len(headers) == 0 || len(colWidths) != len(headers) {
		return
	}
	// Pre-measure every row so compact tables keep together and large tables
	// never orphan their title/header from the first row.
	pdf.SetFont(font, "", 8.6)
	splitAll := make([][][]string, len(rows))
	rowHeights := make([]float64, len(rows))
	totalRowsH := 0.0
	for rIdx, row := range rows {
		maxLines := 1
		splitCells := make([][]string, len(headers))
		for cIdx := range headers {
			val := ""
			if cIdx < len(row) {
				val = row[cIdx]
			}
			if val == "" {
				val = "—"
			}
			lines := pdf.SplitText(val, colWidths[cIdx]-3.2)
			if len(lines) > maxLines {
				maxLines = len(lines)
			}
			splitCells[cIdx] = lines
		}
		rh := float64(maxLines)*4.1 + 2.4
		splitAll[rIdx] = splitCells
		rowHeights[rIdx] = rh
		totalRowsH += rh
	}

	hasTitle := strings.TrimSpace(title) != ""
	titleH := 0.0
	if hasTitle {
		titleH = 5.2
	}
	const headerH = 6.4
	firstRowH := 0.0
	if len(rowHeights) > 0 {
		firstRowH = rowHeights[0]
	}
	totalH := titleH + headerH + totalRowsH + 2.5
	minKeepH := titleH + headerH + firstRowH + 2.5
	if totalH <= 175.0 {
		ensurePDFRoom(pdf, totalH, pageBreakY)
	} else {
		ensurePDFRoom(pdf, minKeepH, pageBreakY)
	}

	if hasTitle {
		pdf.SetFont(font, "B", 9.5)
		pdf.SetTextColor(28, 27, 24)
		pdf.CellFormat(w, titleH, title, "", 1, "L", false, 0, "")
	}
	drawHeader := func() {
		y := pdf.GetY()
		pdf.SetFillColor(239, 236, 230)
		pdf.SetDrawColor(214, 208, 197)
		pdf.SetLineWidth(0.25)
		cx := x
		pdf.SetFont(font, "B", 8.6)
		pdf.SetTextColor(58, 54, 49)
		for i, h := range headers {
			cw := colWidths[i]
			pdf.Rect(cx, y, cw, headerH, "FD")
			align := "L"
			if i < len(alignRight) && alignRight[i] {
				align = "R"
			}
			pdf.SetXY(cx+1.6, y+1.0)
			pdf.CellFormat(cw-3.2, 4.4, h, "", 0, align, false, 0, "")
			cx += cw
		}
		pdf.SetY(y + headerH)
	}
	drawHeader()

	for rIdx := range rows {
		rh := rowHeights[rIdx]
		splitCells := splitAll[rIdx]
		if ensurePDFRoom(pdf, rh, pageBreakY) {
			drawHeader()
		}
		y := pdf.GetY()
		cx := x
		if rIdx%2 == 1 {
			pdf.SetFillColor(250, 249, 247)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		pdf.SetDrawColor(214, 208, 197)
		pdf.SetLineWidth(0.22)
		pdf.SetFont(font, "", 8.6)
		pdf.SetTextColor(28, 27, 24)
		for cIdx := range headers {
			cw := colWidths[cIdx]
			pdf.Rect(cx, y, cw, rh, "FD")
			align := "L"
			if cIdx < len(alignRight) && alignRight[cIdx] {
				align = "R"
			}
			ly := y + 1.2
			for _, ln := range splitCells[cIdx] {
				pdf.SetXY(cx+1.6, ly)
				pdf.CellFormat(cw-3.2, 4.1, ln, "", 0, align, false, 0, "")
				ly += 4.1
			}
			cx += cw
		}
		pdf.SetY(y + rh)
	}
	pdf.Ln(2.5)
}

func renderGroupTablePDF(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "实验分组表")
	rawRows := mapSlice(data["rows"])
	rows := make([][]string, 0, len(rawRows))
	for _, rm := range rawRows {
		role := groupRoleCN(textValue(rm, "role"))
		cond := firstNonEmpty(textValue(rm, "condition"), textValue(rm, "dose"), "—")
		if d := textValue(rm, "dose"); d != "" && cond != d && !strings.Contains(cond, d) {
			cond = d + " · " + cond
		}
		rep := "—"
		indN, hasInd := numericValue(rm["independent_n"])
		techR, hasTech := numericValue(rm["technical_repeats"])
		if !hasTech {
			techR, hasTech = numericValue(rm["replicate_count"])
		}
		if hasInd && hasTech {
			rep = fmt.Sprintf("n=%d × %d", int(indN), int(techR))
		} else if hasInd {
			rep = fmt.Sprintf("n=%d", int(indN))
		} else if hasTech {
			rep = fmt.Sprintf("rep=%d", int(techR))
		}
		rows = append(rows, []string{firstNonEmpty(textValue(rm, "name"), textValue(rm, "id")), role, cond, rep})
	}
	drawPDFTable(pdf, font, x, w, pageBreakY, title,
		[]string{"分组名称", "角色", "处理条件与剂量", "样本与重复"},
		[]float64{44, 30, 80, 26},
		rows, []bool{false, false, false, true},
	)
}

func renderStandaloneGroupsPDF(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, groups []map[string]any) {
	synthetic := map[string]any{"title": "实验分组表", "rows": make([]any, 0, len(groups))}
	var rowList []any
	for _, g := range groups {
		rowList = append(rowList, g)
	}
	synthetic["rows"] = rowList
	renderGroupTablePDF(pdf, font, x, w, pageBreakY, synthetic)
}

func renderMaterialsTablePDF(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, mats []map[string]any) {
	rows := make([][]string, 0, len(mats))
	for _, m := range mats {
		conc := formatAnyValueWithUnit(m["concentration"], "")
		cond := firstNonEmpty(textValue(m, "condition"), textValue(m, "storage"), textValue(m, "pantry_code"), "—")
		rows = append(rows, []string{firstNonEmpty(textValue(m, "name"), textValue(m, "id")), textValue(m, "role"), conc, cond})
	}
	drawPDFTable(pdf, font, x, w, pageBreakY, "关键试剂与材料清单",
		[]string{"材料 / 试剂名称", "类别", "规格/浓度", "备注/储藏短码"},
		[]float64{58, 26, 36, 60},
		rows, []bool{false, false, true, false},
	)
}

func lookupActualParam(actuals map[string]string, stepID, moduleID, key string) string {
	if len(actuals) == 0 || key == "" {
		return ""
	}
	for _, k := range []string{
		stepID + ":" + moduleID + ":" + key,
		moduleID + ":" + key,
		key,
	} {
		if v := strings.TrimSpace(actuals[k]); v != "" {
			return v
		}
	}
	return ""
}

func renderParamTablePDF(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, stepID string, x, w, pageBreakY float64, data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "操作参数表")
	moduleID := textValue(data, "id")
	rawRows := mapSlice(data["rows"])
	needsActual := false
	hasActualData := opts.IsRunRecord || len(opts.ActualParams) > 0
	for _, rm := range rawRows {
		if b, _ := rm["record_actual"].(bool); b {
			needsActual = true
			break
		}
		key := textValue(rm, "key")
		if lookupActualParam(opts.ActualParams, stepID, moduleID, key) != "" {
			needsActual = true
			break
		}
	}
	var rows [][]string
	for _, rm := range rawRows {
		key := textValue(rm, "key")
		name := firstNonEmpty(textValue(rm, "label"), textValue(rm, "name"), key)
		val := formatAnyScalar(rm["value"])
		unit := textValue(rm, "unit")
		note := textValue(rm, "note")
		if needsActual {
			actualCol := "—"
			recorded := lookupActualParam(opts.ActualParams, stepID, moduleID, key)
			if recorded != "" {
				actualCol = recorded
			} else if !hasActualData {
				if b, _ := rm["record_actual"].(bool); b {
					actualCol = "____________"
				}
			}
			rows = append(rows, []string{name, val, unit, actualCol, note})
		} else {
			rows = append(rows, []string{name, val, unit, note})
		}
	}
	if needsActual {
		drawPDFTable(pdf, font, x, w, pageBreakY, title,
			[]string{"参数名称", "计划值", "单位", "实测记录", "说明"},
			[]float64{46, 26, 20, 32, 56},
			rows, []bool{false, true, false, true, false},
		)
		return
	}
	drawPDFTable(pdf, font, x, w, pageBreakY, title,
		[]string{"参数名称", "设定值", "单位", "备注"},
		[]float64{54, 28, 22, 76},
		rows, []bool{false, true, false, false},
	)
}

func renderDataTablePDF(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, x, w, pageBreakY float64, data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "实验测定记录表")
	moduleID := textValue(data, "id")
	cols := mapSlice(data["columns"])
	if len(cols) == 0 {
		return
	}
	headers := make([]string, len(cols))
	colKeys := make([]string, len(cols))
	colKinds := make([]string, len(cols))
	colFormulas := make([]string, len(cols))
	alignRight := make([]bool, len(cols))
	for i, cm := range cols {
		lbl := firstNonEmpty(textValue(cm, "label"), textValue(cm, "key"))
		if u := textValue(cm, "unit"); u != "" {
			lbl += " (" + u + ")"
		}
		headers[i] = lbl
		colKeys[i] = textValue(cm, "key")
		vk := textValue(cm, "value_kind")
		colKinds[i] = vk
		colFormulas[i] = textValue(cm, "formula")
		alignRight[i] = vk == "numeric" || vk == "number" || vk == "formula"
	}
	colWidths := make([]float64, len(cols))
	eachW := w / float64(len(cols))
	for i := range colWidths {
		colWidths[i] = eachW
	}

	authoredRows := mapSlice(data["rows"])
	hydratedMap := map[string]string{}
	if opts.TableCells != nil && moduleID != "" {
		hydratedMap = opts.TableCells[moduleID]
	}
	rowCount := len(authoredRows)
	prefix := "dt:" + moduleID + ":"
	for ck := range hydratedMap {
		if strings.HasPrefix(ck, prefix) {
			rest := strings.TrimPrefix(ck, prefix)
			if colon := strings.IndexByte(rest, ':'); colon > 0 {
				if idx, err := strconv.Atoi(rest[:colon]); err == nil && idx+1 > rowCount {
					rowCount = idx + 1
				}
			}
		}
	}

	var rows [][]string
	for rIdx := 0; rIdx < rowCount; rIdx++ {
		var pre map[string]any
		if rIdx < len(authoredRows) {
			pre = authoredRows[rIdx]
		}
		r := make([]string, len(cols))
		rowVars := map[string]float64{}
		for i, k := range colKeys {
			if colKinds[i] == "formula" {
				continue
			}
			cellKey := fmt.Sprintf("dt:%s:%d:%s", moduleID, rIdx, k)
			rawStr := strings.TrimSpace(hydratedMap[cellKey])
			if rawStr == "" && pre != nil {
				rawStr = formatAnyScalar(pre[k])
			}
			if rawStr == "" {
				rawStr = "—"
			}
			r[i] = rawStr
			if f, err := strconv.ParseFloat(strings.TrimSpace(rawStr), 64); err == nil {
				rowVars[k] = f
			}
		}
		for i := range cols {
			if colKinds[i] != "formula" {
				continue
			}
			val, err := EvalFormula(colFormulas[i], rowVars)
			if err != nil {
				r[i] = "—"
			} else {
				r[i] = formatNumber(round4(val))
			}
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		for r := 0; r < 4; r++ {
			blank := make([]string, len(cols))
			for i := range blank {
				blank[i] = " "
			}
			rows = append(rows, blank)
		}
	}
	drawPDFTable(pdf, font, x, w, pageBreakY, title, headers, colWidths, rows, alignRight)
}

func drawPDFCalculator(pdf *gofpdf.Fpdf, font string, doc *Document, opts DocumentPDFOptions, x, w, pageBreakY float64, ensureSpace func(float64), data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "配方与用量换算")
	res := EvalCalculatorWithActuals(doc, data, opts.CalcActuals)
	if res.Err != nil {
		drawPDFProseCallout(pdf, font, x, w, ensureSpace, title, "warning", res.Summary+"\n换算提示："+res.Err.Error())
		return
	}
	if len(res.Rows) == 0 {
		drawPDFProseCallout(pdf, font, x, w, ensureSpace, title, "note", res.Summary)
		return
	}
	rows := make([][]string, 0, len(res.Rows))
	for _, cr := range res.Rows {
		amt := strconv.FormatFloat(cr.Amount, 'f', -1, 64)
		if math.Abs(cr.Amount-math.Round(cr.Amount)) > 1e-4 {
			amt = fmt.Sprintf("%.2f", cr.Amount)
		}
		rows = append(rows, []string{cr.Label, amt, cr.Unit, firstNonEmpty(cr.Note, "—")})
	}
	heading := title
	if res.Summary != "" {
		heading += " · " + res.Summary
	}
	if len(res.UsedActuals) > 0 {
		heading += "（已按实测值换算）"
	}
	drawPDFTable(pdf, font, x, w, pageBreakY, heading,
		[]string{"组分 / 项目", "计算用量", "单位", "配制说明"},
		[]float64{56, 28, 22, 74},
		rows, []bool{false, true, false, false},
	)
}

func drawPDFPlate(pdf *gofpdf.Fpdf, font string, doc *Document, x, w, pageBreakY float64, imgSeq *int, data map[string]any) {
	format := int(numOrZero(data["format"]))
	if format <= 0 {
		format = 96
	}
	topology := firstNonEmpty(textValue(data, "topology"), "plate")
	rows, cols := int(numOrZero(data["rows"])), int(numOrZero(data["columns"]))
	geom, ok := geometryForPlate(format, topology, rows, cols)
	if !ok {
		return
	}
	plateID := textValue(data, "id")
	fallbackTitle := fmt.Sprintf("%d 孔板排布", geom.Rows*geom.Columns)
	switch topology {
	case "dish":
		fallbackTitle = fmt.Sprintf("培养皿分区（%d 区）", geom.Rows*geom.Columns)
	case "rack_1d":
		fallbackTitle = fmt.Sprintf("%d 位联管 / 样本架", geom.Rows*geom.Columns)
	}
	title := firstNonEmpty(textValue(data, "title"), fallbackTitle)

	rawGroups, _ := data["groups"].(map[string]any)
	groupsMap := map[string][]string{}
	wellToGroup := map[string]string{}
	for g, rawList := range rawGroups {
		if items, ok := rawList.([]any); ok {
			var list []string
			for _, it := range items {
				if ws, ok := it.(string); ok {
					list = append(list, ws)
					wellToGroup[strings.ToUpper(strings.TrimSpace(ws))] = g
				}
			}
			groupsMap[g] = list
		}
	}
	samples := stringMapAny(data["samples"])
	roles := stringMapAny(data["roles"])
	replicates := map[string]int{}
	if repMap, ok := data["replicates"].(map[string]any); ok {
		for wk, rv := range repMap {
			if n, ok := numericValue(rv); ok {
				replicates[strings.ToUpper(strings.TrimSpace(wk))] = int(n)
			}
		}
	}
	factors := map[string]map[string]string{}
	if fRaw, ok := data["factors"].(map[string]any); ok {
		for dim, mRaw := range fRaw {
			factors[dim] = stringMapAny(mRaw)
		}
	}

	wellStates := map[string]plateui.WellState{}
	for well, g := range wellToGroup {
		wf := map[string]string{}
		for dim, wm := range factors {
			if v := wm[well]; v != "" {
				wf[dim] = v
			}
		}
		wellStates[well] = plateui.WellState{
			Group:     g,
			SampleID:  samples[well],
			Replicate: replicates[well],
			Role:      roles[well],
			Factors:   wf,
		}
	}
	tempPlate := &Plate{ID: plateID, Groups: groupsMap}
	pngBytes, err := plateui.RenderPNG(plateui.Props{
		Wells:       wellStates,
		Rows:        geom.Rows,
		Cols:        geom.Columns,
		Topology:    topology,
		Roles:       roles,
		GroupIndex:  plateGroupPaletteMap(doc, tempPlate),
		GroupLabels: PlateGroupLabelMap(doc),
	}, title, false)
	if err != nil || len(pngBytes) == 0 {
		return
	}

	drawEmbeddedPNG(pdf, font, x, w, pageBreakY, imgSeq, "plate", pngBytes, 174, 94, "", "")
}

func drawPDFFigure(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, x, w, pageBreakY float64, ensureSpace func(float64), imgSeq *int, data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "示意图 / 附图")
	caption := strings.TrimSpace(textValue(data, "caption"))
	svgRaw := strings.TrimSpace(textValue(data, "svg"))
	src := strings.TrimSpace(textValue(data, "src"))

	var annotations []svgTextLabel
	for _, am := range mapSlice(data["annotations"]) {
		ax, _ := numericValue(am["x"])
		ay, _ := numericValue(am["y"])
		if lbl := textValue(am, "label"); lbl != "" {
			annotations = append(annotations, svgTextLabel{X: ax, Y: ay, Anchor: "middle", Text: lbl, Size: 11})
		}
	}

	var pngBytes []byte
	if svgRaw != "" {
		pngBytes = rasterizeFigureSVGToPNG(svgRaw, annotations)
	} else if src != "" {
		resolved := resolveFigurePath(opts.BaseDir, src)
		if raw, err := os.ReadFile(resolved); err == nil {
			pngBytes = ensureImagePNGBytes(raw)
		}
	}
	if len(pngBytes) == 0 {
		if caption != "" || src != "" {
			drawPDFProseCallout(pdf, font, x, w, ensureSpace, title, "plain", firstNonEmpty(caption, "图片引用："+src))
		}
		return
	}

	drawEmbeddedPNG(pdf, font, x, w, pageBreakY, imgSeq, "fig", pngBytes, 152, 82, title, caption)
}

func rasterizeFigureSVGToPNG(svgRaw string, annotations []svgTextLabel) []byte {
	parsedSVG, err := ui.ParseSVG([]byte(svgRaw))
	if err != nil || parsedSVG == nil {
		return nil
	}
	vbX, vbY, vbW, vbH, labels := parseSVGMetricsAndLabels(svgRaw)
	allLabels := append(labels, annotations...)
	drawW := 420
	drawH := int(math.Round(float64(drawW) * vbH / vbW))
	if drawH < 180 {
		drawH = 180
	}
	if drawH > 380 {
		drawH = 380
		drawW = int(math.Round(float64(drawH) * vbW / vbH))
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Background(tok.Surface).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Box(c).Size(float32(drawW-16), float32(drawH-16)).DrawOver(func(p *ui.Painter, r ui.Rect) {
				sx := float64(r.W) / vbW
				sy := float64(r.H) / vbH
				for _, lbl := range allLabels {
					px := r.X + float32((lbl.X-vbX)*sx)
					py := r.Y + float32((lbl.Y-vbY)*sy)
					fsize := float32(lbl.Size * sx)
					if fsize < 11 {
						fsize = 11
					}
					if fsize > 16 {
						fsize = 16
					}
					textW := estimateSVGTextWidth(lbl.Text, fsize)
					if lbl.Anchor == "middle" {
						px -= textW * 0.5
					} else if lbl.Anchor == "end" {
						px -= textW
					}
					p.Text(px, py-fsize*0.35, lbl.Text, fsize, tok.Text)
				}
			}).Children(func() {
				ui.Image(c, parsedSVG).Size(float32(drawW-16), float32(drawH-16))
			})
		})
	}, drawW, drawH)
	tt.SetScale(2.0)
	tt.Frame()
	img := tt.Image()
	if img == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func ensureImagePNGBytes(raw []byte) []byte {
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func drawEmbeddedPNG(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, imgSeq *int, prefix string, pngBytes []byte, maxW, maxH float64, title, caption string) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return
	}
	aspect := float64(cfg.Width) / float64(cfg.Height)
	drawW := maxW
	if drawW > w {
		drawW = w
	}
	drawH := drawW / aspect
	if drawH > maxH {
		drawH = maxH
		drawW = drawH * aspect
	}
	titleH := 0.0
	if strings.TrimSpace(title) != "" {
		titleH = 5.2
	}
	extra := 4.0
	if caption != "" {
		extra = 10.0
	}
	ensurePDFRoom(pdf, titleH+drawH+extra, pageBreakY)
	if titleH > 0 {
		pdf.SetFont(font, "B", 9.5)
		pdf.SetTextColor(28, 27, 24)
		pdf.CellFormat(w, titleH, title, "", 1, "L", false, 0, "")
	}
	*imgSeq++
	imgName := fmt.Sprintf("%s-%d", prefix, *imgSeq)
	info := pdf.RegisterImageOptionsReader(imgName, gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: false}, bytes.NewReader(pngBytes))
	if info == nil || pdf.Error() != nil {
		return
	}
	y := pdf.GetY()
	drawX := x + (w-drawW)/2
	pdf.SetDrawColor(231, 227, 219)
	pdf.SetLineWidth(0.25)
	pdf.ImageOptions(imgName, drawX, y, drawW, drawH, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
	pdf.Rect(drawX, y, drawW, drawH, "D")
	pdf.SetY(y + drawH + 1.8)
	if caption != "" {
		pdf.SetFont(font, "", 8.4)
		pdf.SetTextColor(95, 90, 82)
		pdf.MultiCell(w, 4.0, caption, "", "C", false)
	}
	pdf.Ln(2.0)
}

func drawPDFChecklist(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, stepID string, x, w, pageBreakY float64, ensureSpace func(float64), title string, items []map[string]any) {
	if len(items) == 0 {
		return
	}
	ensurePDFRoom(pdf, 18, pageBreakY)
	pdf.SetFont(font, "B", 9.4)
	pdf.SetTextColor(28, 27, 24)
	pdf.CellFormat(w, 5.2, title, "", 1, "L", false, 0, "")

	for _, it := range items {
		cid := firstNonEmpty(textValue(it, "id"), textValue(it, "check_id"))
		label := firstNonEmpty(textValue(it, "label"), textValue(it, "title"), cid)
		if b, _ := it["blocking"].(bool); b {
			label = "[关键必检] " + label
		}
		checked := cid != "" && (opts.CheckedItems[cid] || (stepID != "" && opts.CheckedItems[stepID+":"+cid]))
		skipReason := ""
		if opts.SkippedItems != nil && cid != "" {
			skipReason = firstNonEmpty(opts.SkippedItems[cid], opts.SkippedItems[stepID+":"+cid])
		}
		if skipReason != "" {
			label += " [已豁免: " + skipReason + "]"
		}
		pdf.SetFont(font, "", 9.0)
		lines := pdf.SplitText(label, w-9)
		rh := float64(max(1, len(lines)))*4.4 + 1.2
		ensurePDFRoom(pdf, rh, pageBreakY)
		y := pdf.GetY()
		if checked {
			pdf.SetFillColor(238, 251, 243)
			pdf.SetDrawColor(47, 122, 79)
			pdf.SetLineWidth(0.3)
			pdf.Rect(x+1.5, y+0.8, 3.2, 3.2, "FD")
			pdf.SetLineWidth(0.45)
			pdf.Line(x+2.0, y+2.4, x+2.85, y+3.25)
			pdf.Line(x+2.85, y+3.25, x+4.25, y+1.45)
		} else {
			pdf.SetDrawColor(120, 114, 104)
			pdf.SetLineWidth(0.3)
			pdf.Rect(x+1.5, y+0.8, 3.2, 3.2, "D")
		}
		pdf.SetLineWidth(0.25)
		pdf.SetXY(x+6.5, y+0.2)
		pdf.SetTextColor(28, 27, 24)
		pdf.MultiCell(w-7, 4.4, label, "", "L", false)
		pdf.SetY(y + rh)
	}
	pdf.Ln(1.5)
}

func renderSubflowPDF(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, stepID string, x, w, pageBreakY float64, ensureSpace func(float64), data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "细分子操作")
	items := mapSlice(data["items"])
	if len(items) == 0 {
		return
	}
	rows := make([]map[string]any, 0, len(items))
	for i, it := range items {
		lbl := fmt.Sprintf("%d. %s", i+1, firstNonEmpty(textValue(it, "title"), textValue(it, "id")))
		if d := formatAnyValueWithUnit(it["duration"], ""); d != "—" {
			lbl += "（" + d + "）"
		}
		if n := textValue(it, "note"); n != "" {
			lbl += " — " + n
		}
		rows = append(rows, map[string]any{"id": textValue(it, "id"), "label": lbl})
	}
	drawPDFChecklist(pdf, font, opts, stepID, x, w, pageBreakY, ensureSpace, title, rows)
}

func renderTimerPDF(pdf *gofpdf.Fpdf, font string, x, w float64, ensureSpace func(float64), data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "过程计时")
	dur := formatAnyValueWithUnit(data["duration"], "")
	mode := textValue(data, "mode")
	line := fmt.Sprintf("计时模式：%s   ｜   设定时长：%s   ｜   实际开始：____:____   结束：____:____", firstNonEmpty(mode, "倒计时"), dur)
	drawPDFProseCallout(pdf, font, x, w, ensureSpace, "计时器 · "+title, "note", line)
}

func renderAttachmentPDF(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, stepID string, x, w, pageBreakY float64, ensureSpace func(float64), imgSeq *int, data map[string]any) {
	title := firstNonEmpty(textValue(data, "title"), "实拍照片 / 仪器文件附件槽")
	moduleID := textValue(data, "id")
	hint := firstNonEmpty(textValue(data, "media_hint"), "请在实验运行期关联原始图像或测量文件")

	var thumbs []AssetThumb
	if opts.LinkedAssets != nil {
		thumbs = opts.LinkedAssets[moduleID]
		if len(thumbs) == 0 && stepID != "" {
			thumbs = opts.LinkedAssets[stepID]
		}
		if len(thumbs) == 0 && textValue(data, "scope") != "design_ref" {
			thumbs = opts.LinkedAssets[""]
		}
	}
	if len(thumbs) == 0 {
		drawPDFProseCallout(pdf, font, x, w, ensureSpace, "采图/附件 · "+title, "plain", hint)
		return
	}

	ensurePDFRoom(pdf, 24, pageBreakY)
	pdf.SetFont(font, "B", 9.5)
	pdf.SetTextColor(28, 27, 24)
	pdf.CellFormat(w, 5.2, fmt.Sprintf("采图/附件 · %s（共 %d 份）", title, len(thumbs)), "", 1, "L", false, 0, "")
	for _, th := range thumbs {
		capLabel := firstNonEmpty(th.Label, th.WellID, "实拍附件")
		if th.WellID != "" && !strings.Contains(capLabel, th.WellID) {
			capLabel = th.WellID + " · " + capLabel
		}
		if th.FovIndex > 0 {
			capLabel += fmt.Sprintf(" · 视野 %d", th.FovIndex)
		}
		resolved := resolveFigurePath(opts.BaseDir, th.FilePath)
		if raw, err := os.ReadFile(resolved); err == nil {
			if pngBytes := ensureImagePNGBytes(raw); len(pngBytes) > 0 {
				drawEmbeddedPNG(pdf, font, x, w, pageBreakY, imgSeq, "att", pngBytes, 132, 62, "", capLabel)
				continue
			}
		}
		pdf.SetFont(font, "", 8.8)
		pdf.SetTextColor(58, 54, 49)
		pdf.MultiCell(w, 4.4, "• 附件："+capLabel, "", "L", false)
	}
}

func renderNotebookMarkdownToPDF(pdf *gofpdf.Fpdf, font string, opts DocumentPDFOptions, x, w, pageBreakY float64, ensureSpace func(float64), imgSeq *int, md string) {
	normalized := strings.ReplaceAll(strings.TrimSpace(md), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	inAsterLab := false
	inCode := false
	var asterBuf strings.Builder
	var mdBuf []string

	flushMarkdown := func() {
		for _, rawLine := range mdBuf {
			ln := strings.TrimRight(rawLine, " \t")
			trimmed := strings.TrimSpace(ln)
			if trimmed == "" {
				pdf.Ln(2.5)
				continue
			}
			if strings.HasPrefix(trimmed, ">") {
				qBody := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
				qBody = strings.ReplaceAll(qBody, "**", "")
				drawPDFProseCallout(pdf, font, x, w, ensureSpace, "", "note", qBody)
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				lvl := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
				heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
				sz := 12.2 - float64(max(0, lvl-1))*1.1
				if sz < 10.0 {
					sz = 10.0
				}
				ensurePDFRoom(pdf, 16, pageBreakY)
				pdf.SetFont(font, "B", sz)
				pdf.SetTextColor(28, 27, 24)
				pdf.SetX(x)
				pdf.MultiCell(w, 5.4, heading, "", "L", false)
				pdf.Ln(1)
				continue
			}
			clean := strings.ReplaceAll(trimmed, "**", "")
			clean = strings.ReplaceAll(clean, "`", "")
			if strings.HasPrefix(clean, "- ") || strings.HasPrefix(clean, "* ") {
				clean = "• " + strings.TrimSpace(clean[2:])
			}
			ensureSpace(6)
			pdf.SetFont(font, "", 9.4)
			pdf.SetTextColor(36, 34, 30)
			pdf.SetX(x)
			pdf.MultiCell(w, 4.6, clean, "", "L", false)
		}
		mdBuf = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			if !inAsterLab && !inCode {
				if lang == "asterlab" {
					flushMarkdown()
					inAsterLab = true
					asterBuf.Reset()
				} else {
					inCode = true
				}
				continue
			}
			if inAsterLab {
				src := "```asterlab\n" + asterBuf.String() + "\n```"
				if parsed, err := Parse(src); err == nil {
					for _, m := range parsed.Modules {
						renderModuleToPDF(pdf, font, parsed, opts, x, w, pageBreakY, ensureSpace, imgSeq, "", m.Kind, m.Data)
					}
				}
				inAsterLab = false
				asterBuf.Reset()
				continue
			}
			if inCode {
				inCode = false
				continue
			}
		}
		if inAsterLab {
			asterBuf.WriteString(line)
			asterBuf.WriteByte('\n')
		} else {
			mdBuf = append(mdBuf, line)
		}
	}
	flushMarkdown()
}

func groupRoleCN(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "control":
		return "阴性/空白对照"
	case "reference", "positive_control":
		return "阳性参照"
	case "treatment":
		return "干预实验组"
	case "vehicle":
		return "溶剂对照"
	default:
		return firstNonEmpty(role, "实验组")
	}
}

func formatAnyScalar(v any) string {
	if v == nil {
		return "—"
	}
	if m, ok := v.(map[string]any); ok {
		return formatAnyValueWithUnit(m, "")
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if s == "" || s == "<nil>" {
		return "—"
	}
	return s
}

func formatAnyValueWithUnit(v any, fallbackUnit string) string {
	if v == nil {
		return "—"
	}
	if m, ok := v.(map[string]any); ok {
		val := formatAnyScalar(m["value"])
		u := firstNonEmpty(textValue(m, "unit"), fallbackUnit)
		if u != "" && val != "—" {
			return val + " " + u
		}
		return val
	}
	val := formatAnyScalar(v)
	if fallbackUnit != "" && val != "—" {
		return val + " " + fallbackUnit
	}
	return val
}

func stringMapAny(v any) map[string]string {
	out := map[string]string{}
	if m, ok := v.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out[strings.ToUpper(strings.TrimSpace(k))] = fmt.Sprint(m[k])
		}
	}
	return out
}

func stripLeadingStepNumber(title string) string {
	runes := []rune(strings.TrimSpace(title))
	i := 0
	for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
		i++
	}
	if i > 0 && i <= 3 && i < len(runes) && (runes[i] == ' ' || runes[i] == '.' || runes[i] == '、') {
		rest := strings.TrimSpace(string(runes[i+1:]))
		if rest != "" {
			return rest
		}
	}
	return string(runes)
}

func collectPDFFootnotes(docSource, notebookMD string) []FootnoteEntry {
	var out []FootnoteEntry
	seen := map[string]bool{}
	appendDoc := func(src string) {
		if strings.TrimSpace(src) == "" {
			return
		}
		fnDoc := ParseFootnotes(src)
		if fnDoc == nil {
			return
		}
		for _, e := range fnDoc.Entries {
			if seen[e.ID] {
				continue
			}
			seen[e.ID] = true
			e.Number = len(out) + 1
			out = append(out, e)
		}
	}
	appendDoc(docSource)
	appendDoc(notebookMD)
	return out
}

func renderFootnotesAppendixPDF(pdf *gofpdf.Fpdf, font string, x, w, pageBreakY float64, entries []FootnoteEntry) {
	if len(entries) == 0 {
		return
	}
	heading := "附录：实验批注与参考文献"
	badge := fmt.Sprintf("%d 条", len(entries))
	emptyBody := "（未填写批注正文）"
	if font == "Helvetica" {
		heading = "Appendix: Footnotes & Notes"
		badge = fmt.Sprintf("%d notes", len(entries))
		emptyBody = "(no note body)"
	}

	ensurePDFRoom(pdf, 26, pageBreakY)
	drawPDFSectionHeading(pdf, font, x, w, heading, badge)

	numColW := 14.0
	bodyColW := w - numColW - 2.0
	for _, fn := range entries {
		body := strings.TrimSpace(fn.Body)
		if body == "" {
			body = emptyBody
		}
		prefix := fmt.Sprintf("[%d]", fn.Number)
		if fn.ID != "" && fn.ID != strconv.Itoa(fn.Number) {
			prefix = fmt.Sprintf("[%d · %s]", fn.Number, fn.ID)
		}
		pdf.SetFont(font, "", 8.8)
		lines := pdf.SplitText(body, bodyColW)
		rh := float64(max(1, len(lines)))*4.2 + 2.4
		ensurePDFRoom(pdf, rh, pageBreakY)

		y0 := pdf.GetY()
		pdf.SetDrawColor(214, 208, 197)
		pdf.SetLineWidth(0.35)
		pdf.Line(x+1.0, y0+0.6, x+1.0, y0+rh-1.2)

		pdf.SetXY(x+3.0, y0+0.4)
		pdf.SetFont(font, "B", 8.8)
		pdf.SetTextColor(91, 69, 201)
		pdf.CellFormat(numColW, 4.2, prefix, "", 0, "L", false, 0, "")

		pdf.SetXY(x+3.0+numColW, y0+0.4)
		pdf.SetFont(font, "", 8.8)
		pdf.SetTextColor(36, 34, 30)
		pdf.MultiCell(bodyColW, 4.2, body, "", "L", false)
		pdf.SetY(y0 + rh)
	}
	pdf.Ln(2.0)
}
