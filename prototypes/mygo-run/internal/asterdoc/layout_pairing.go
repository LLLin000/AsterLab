package asterdoc

import (
	"fmt"
	"math"
	"strings"
)

const layoutFullDirectiveLine = "# @layout: full"

func findModuleByID(doc *Document, moduleID string) *Module {
	if doc == nil || moduleID == "" {
		return nil
	}
	for i := range doc.Modules {
		if doc.Modules[i].ID == moduleID {
			return &doc.Modules[i]
		}
	}
	return nil
}

func hasLayoutFullDirective(body string) bool {
	for _, ln := range strings.Split(body, "\n") {
		if strings.TrimSpace(ln) == layoutFullDirectiveLine {
			return true
		}
	}
	return false
}

// stripLayoutFullDirective removes any standalone `# @layout: full` lines while
// preserving original ordering of all other YAML lines.
func stripLayoutFullDirective(body string) string {
	lines := strings.SplitAfter(body, "\n")
	var kept []string
	for _, ln := range lines {
		if strings.TrimSpace(ln) == layoutFullDirectiveLine {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "")
}

// ToggleTopLevelModuleFullWidthInSource persists or clears `# @layout: full` on
// a top-level module fence without altering any validated AsterML YAML keys.
func ToggleTopLevelModuleFullWidthInSource(source, moduleID string, full bool) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.ID != moduleID {
			continue
		}
		cleaned := strings.TrimLeft(stripLayoutFullDirective(m.Body), "\r\n")
		nextBody := cleaned
		if full {
			nextBody = layoutFullDirectiveLine + "\n" + cleaned
		}
		nextSrc := source[:m.Start] + nextBody + source[m.End:]
		if _, err := Parse(nextSrc); err != nil {
			return "", err
		}
		return nextSrc, nil
	}
	return "", fmt.Errorf("module %q is not in this document", moduleID)
}

// isPairableModuleKind reports whether a top-level module kind can participate in
// automatic rectangular twin pairing.
func isPairableModuleKind(kind string) bool {
	switch kind {
	case "timer", "calculator", "group_table", "plate", "figure":
		return true
	default:
		return false
	}
}

func pairClassForSegment(vs visualFlowSeg) string {
	if vs.isStepHeader {
		return ""
	}
	switch vs.seg.Kind {
	case SegmentEmbed:
		return "media"
	case SegmentModule:
		if vs.seg.Module == nil {
			return ""
		}
		switch vs.seg.Module.Kind {
		case "timer", "calculator", "group_table", "plate":
			return vs.seg.Module.Kind
		case "figure":
			return "media"
		}
	}
	return ""
}

func segmentHasFullLayoutOptOut(vs visualFlowSeg) bool {
	if vs.seg.Kind == SegmentModule && vs.seg.Module != nil {
		return hasLayoutFullDirective(vs.seg.Module.Body)
	}
	return false
}

func figureDataHasGraphic(data map[string]any) bool {
	src := strings.TrimSpace(textValue(data, "src"))
	if src != "" {
		return true
	}
	trimSVG := strings.TrimSpace(textValue(data, "svg"))
	isPlaceholder := trimSVG == "" || (strings.HasPrefix(trimSVG, "<svg") && strings.HasSuffix(trimSVG, "></svg>") && strings.Count(trimSVG, "<") == 2)
	return !isPlaceholder
}

type pairSegmentMetrics struct {
	minW  float32
	estH  float32
	class string
}

func estimatePairMetrics(vs visualFlowSeg) (pairSegmentMetrics, bool) {
	cls := pairClassForSegment(vs)
	if cls == "" || segmentHasFullLayoutOptOut(vs) {
		return pairSegmentMetrics{}, false
	}
	if vs.seg.Kind == SegmentEmbed {
		return pairSegmentMetrics{minW: 300, estH: 240, class: "media"}, true
	}
	m := vs.seg.Module
	if m == nil {
		return pairSegmentMetrics{}, false
	}
	data := m.Data
	switch m.Kind {
	case "timer":
		return pairSegmentMetrics{minW: 280, estH: 54, class: "timer"}, true
	case "calculator":
		if strings.EqualFold(textValue(data, "mode"), "serial_dilution") ||
			strings.EqualFold(textValue(data, "formula"), "serial_dilution") {
			return pairSegmentMetrics{}, false
		}
		nRows := len(mapSlice(data["rows"]))
		if inN := len(mapSlice(data["inputs"])); inN > nRows {
			nRows = inN
		}
		if nRows < 1 {
			nRows = 1
		}
		return pairSegmentMetrics{
			minW:  310,
			estH:  148 + float32(nRows)*34,
			class: "calculator",
		}, true
	case "group_table":
		rows := len(mapSlice(data["rows"]))
		if rows > 8 {
			return pairSegmentMetrics{}, false
		}
		effRows := rows
		if effRows < 1 {
			effRows = 1
		}
		return pairSegmentMetrics{
			minW:  330,
			estH:  88 + float32(effRows)*36,
			class: "group_table",
		}, true
	case "plate":
		p, err := plateFromData(data)
		if err != nil {
			return pairSegmentMetrics{}, false
		}
		geom, ok := geometryForPlate(p.Format, p.Topology, p.Rows, p.Columns)
		if !ok {
			return pairSegmentMetrics{}, false
		}
		if p.Topology == "rack_1d" || p.Format > 24 || geom.Columns > 6 || geom.Rows > 4 {
			return pairSegmentMetrics{}, false
		}
		var minW, rowPitch float32 = 340, 46
		switch {
		case p.Format >= 24 || geom.Columns >= 6:
			minW, rowPitch = 380, 40
		case p.Format <= 6:
			minW, rowPitch = 310, 52
		}
		estH := 106 + float32(geom.Rows)*rowPitch
		if len(p.Layers) > 0 {
			estH += 36
		}
		if len(p.Groups) > 0 {
			estH += 24
		}
		return pairSegmentMetrics{minW: minW, estH: estH, class: "plate"}, true
	case "figure":
		if !figureDataHasGraphic(data) {
			return pairSegmentMetrics{minW: 280, estH: 86, class: "media"}, true
		}
		estH := float32(250)
		if mh, hasH := numericValue(data["max_height"]); hasH && mh >= 40 {
			estH = float32(mh) + 64
		}
		return pairSegmentMetrics{minW: 300, estH: estH, class: "media"}, true
	}
	return pairSegmentMetrics{}, false
}

// canPairVisualSegments determines whether two adjacent visual segments form a
// safe, rectangular twin pair at availW (both width-fit and height-balanced <= 1.40).
func canPairVisualSegments(doc *Document, a, b visualFlowSeg, availW float32) bool {
	_ = doc
	if availW < 640 {
		return false
	}
	ma, okA := estimatePairMetrics(a)
	mb, okB := estimatePairMetrics(b)
	if !okA || !okB {
		return false
	}
	pairAllowed := false
	switch {
	case ma.class == "timer" && mb.class == "timer":
		pairAllowed = true
	case ma.class == "calculator" && mb.class == "calculator":
		pairAllowed = true
	case (ma.class == "group_table" && mb.class == "plate") || (ma.class == "plate" && mb.class == "group_table"):
		pairAllowed = true
	case ma.class == "media" && mb.class == "media":
		pairAllowed = true
	}
	if !pairAllowed {
		return false
	}
	const gap = float32(12)
	if availW < ma.minW+mb.minW+gap {
		return false
	}
	maxH := float32(math.Max(float64(ma.estH), float64(mb.estH)))
	minH := float32(math.Min(float64(ma.estH), float64(mb.estH)))
	if minH <= 0 || maxH/minH > 1.40 {
		return false
	}
	return true
}

// nextNonBlankVisualSegIndex returns the index of the next renderable segment after
// `afterIdx` (skipping whitespace-only prose segments rendered as nothing), or -1 if
// the immediately following segment is non-blank prose or step header, or endIdx is reached.
func nextNonBlankVisualSegIndex(visualSegs []visualFlowSeg, afterIdx, endIdx int) int {
	for idx := afterIdx + 1; idx < endIdx; idx++ {
		vs := visualSegs[idx]
		if vs.isStepHeader {
			return -1
		}
		if vs.seg.Kind == SegmentProse {
			if strings.TrimSpace(vs.seg.Text) == "" {
				continue
			}
			return -1
		}
		return idx
	}
	return -1
}
