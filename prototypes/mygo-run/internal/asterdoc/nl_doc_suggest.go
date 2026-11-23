package asterdoc

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DocumentSuggestion represents one actionable interactive module suggestion
// extracted from a natural-language line in an `.asterml` document by the
// built-in offline SciGraph + NL router + deterministic module compiler.
// Shared between desktop Alt+Hover and the `asterlab asterdoc.suggest` CLI.
type DocumentSuggestion struct {
	Line          int              `json:"line"`
	ByteOffset    int              `json:"byteOffset"`
	StepTitle     string           `json:"stepTitle,omitempty"`
	ChapterTitle  string           `json:"chapterTitle,omitempty"`
	Zone          string           `json:"zone"`
	OriginalLine  string           `json:"originalLine"`
	Kind          string           `json:"kind"`
	Mode          string           `json:"mode,omitempty"`
	KindLabel     string           `json:"kindLabel,omitempty"`
	Summary       string           `json:"summary"`
	InheritedFrom string           `json:"inheritedFrom,omitempty"`
	Confidence    float64          `json:"confidence"`
	Source        string           `json:"source"`
	MissingSlots  []string         `json:"missingSlots,omitempty"`
	Params        []NLParamKV      `json:"params,omitempty"`
	Modules       []map[string]any `json:"modules"`
	YAML          string           `json:"yaml"`
}

// DocumentSuggestReport summarizes all non-intrusive module upgrade suggestions
// discovered across a Markdown-first `.asterml` document.
type DocumentSuggestReport struct {
	Engine       string               `json:"engine"`
	ScannedLines int                  `json:"scannedLines"`
	SkippedLines int                  `json:"skippedLines"`
	Suggestions  []DocumentSuggestion `json:"suggestions"`
}

// SuggestDocumentModules scans all non-fenced, non-frontmatter Markdown prose
// lines in doc using the exact same SciGraph zone gating, offline ONNX/local
// semantic router, and deterministic module compiler used by Alt+Hover in the UI.
//
// Lines outside actionable zones (headings, metadata, materials, review, fenced
// code blocks) or routed to `none` are cleanly skipped so agents receive only
// high-signal, spec-validated interactive module suggestions.
func SuggestDocumentModules(doc *Document) DocumentSuggestReport {
	engine := "heuristic"
	if DefaultNLOnnxRouter() != nil {
		engine = "onnx"
	}
	rep := DocumentSuggestReport{
		Engine:      engine,
		Suggestions: []DocumentSuggestion{},
	}
	if doc == nil {
		return rep
	}

	src := doc.Source
	_, bodyStart, _ := ScanFrontmatter(src)
	graph := BuildSciGraph(doc)

	// Track existing + newly suggested IDs so every proposed block has a unique ID.
	usedIDs := map[string]bool{}
	for _, m := range doc.Modules {
		if m.ID != "" {
			usedIDs[m.ID] = true
		}
		if m.Kind == "step" {
			for _, bm := range mapSlice(m.Data["blocks"]) {
				if id := textValue(bm, "id"); id != "" {
					usedIDs[id] = true
				}
			}
		}
	}
	ensureUnique := func(base string) string {
		if base == "" {
			base = "blk-nl"
		}
		if !usedIDs[base] {
			usedIDs[base] = true
			return base
		}
		for i := 2; i < 1000; i++ {
			cand := fmt.Sprintf("%s-%d", base, i)
			if !usedIDs[cand] {
				usedIDs[cand] = true
				return cand
			}
		}
		return base
	}

	lines := strings.SplitAfter(src, "\n")
	offset := 0
	inFence := false
	fenceTicks := 0

	for idx, rawLine := range lines {
		lineNo := idx + 1
		lineStart := offset
		offset += len(rawLine)

		if lineStart < bodyStart {
			continue
		}

		trimmed := strings.TrimSpace(strings.TrimRight(rawLine, "\r\n"))
		if !inFence {
			if ticks := countLeadingBackticks(trimmed); ticks >= 3 {
				inFence = true
				fenceTicks = ticks
				rep.SkippedLines++
				continue
			}
		} else {
			if isClosingBacktickFence(trimmed, fenceTicks) {
				inFence = false
				fenceTicks = 0
			}
			rep.SkippedLines++
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") || reThematicBreakLine.MatchString(trimmed) {
			rep.SkippedLines++
			continue
		}

		rep.ScannedLines++
		var ctx ScientificLineContext
		if graph != nil {
			ctx = graph.ContextAt(lineStart)
			if !ctx.AllowedForNL() {
				rep.SkippedLines++
				continue
			}
		}

		sugs := AnalyzeLabSentenceWithContext(doc, trimmed, lineStart, nil)
		if len(sugs) == 0 {
			rep.SkippedLines++
			continue
		}
		for _, sug := range sugs {
			if sug == nil || len(sug.Modules) == 0 {
				continue
			}
			clonedMods := make([]map[string]any, 0, len(sug.Modules))
			var yamlParts []string
			for _, b := range sug.Modules {
				cp := cloneAnyMap(b)
				if id := textValue(cp, "id"); id != "" {
					cp["id"] = ensureUnique(id)
				}
				clonedMods = append(clonedMods, cp)
				if rawYAML, err := yaml.Marshal(cp); err == nil {
					yamlParts = append(yamlParts, "```asterlab\n"+strings.TrimRight(string(rawYAML), "\n")+"\n```")
				}
			}
			rep.Suggestions = append(rep.Suggestions, DocumentSuggestion{
				Line:          lineNo,
				ByteOffset:    lineStart,
				StepTitle:     ctx.StepTitle,
				ChapterTitle:  ctx.ChapterTitle,
				Zone:          string(ctx.Zone),
				OriginalLine:  trimmed,
				Kind:          sug.Kind,
				Mode:          sug.Mode,
				KindLabel:     sug.KindLabel,
				Summary:       sug.Summary,
				InheritedFrom: sug.InheritedFrom,
				Confidence:    sug.Confidence,
				Source:        sug.Source,
				MissingSlots:  sug.MissingSlots,
				Params:        sug.Params,
				Modules:       clonedMods,
				YAML:          strings.Join(yamlParts, "\n\n"),
			})
		}
	}

	return rep
}

// SuggestSentenceModules compiles one standalone natural-language lab sentence
// (optionally enriched by an existing Document's plates/groups/reagents) into
// spec-validated AsterML module suggestion(s).
func SuggestSentenceModules(doc *Document, sentence string) DocumentSuggestReport {
	engine := "heuristic"
	if DefaultNLOnnxRouter() != nil {
		engine = "onnx"
	}
	rep := DocumentSuggestReport{
		Engine:       engine,
		ScannedLines: 1,
		Suggestions:  []DocumentSuggestion{},
	}
	trimmed := strings.TrimSpace(sentence)
	if trimmed == "" {
		rep.SkippedLines = 1
		return rep
	}
	sugs := AnalyzeLabSentenceWithContext(doc, trimmed, -1, nil)
	if len(sugs) == 0 {
		rep.SkippedLines = 1
		return rep
	}
	for _, sug := range sugs {
		if sug == nil || len(sug.Modules) == 0 {
			continue
		}
		clonedMods := make([]map[string]any, 0, len(sug.Modules))
		var yamlParts []string
		for _, b := range sug.Modules {
			cp := cloneAnyMap(b)
			clonedMods = append(clonedMods, cp)
			if rawYAML, err := yaml.Marshal(cp); err == nil {
				yamlParts = append(yamlParts, "```asterlab\n"+strings.TrimRight(string(rawYAML), "\n")+"\n```")
			}
		}
		rep.Suggestions = append(rep.Suggestions, DocumentSuggestion{
			Line:          1,
			ByteOffset:    0,
			Zone:          string(ZoneStepBody),
			OriginalLine:  trimmed,
			Kind:          sug.Kind,
			Mode:          sug.Mode,
			KindLabel:     sug.KindLabel,
			Summary:       sug.Summary,
			InheritedFrom: sug.InheritedFrom,
			Confidence:    sug.Confidence,
			Source:        sug.Source,
			MissingSlots:  sug.MissingSlots,
			Params:        sug.Params,
			Modules:       clonedMods,
			YAML:          strings.Join(yamlParts, "\n\n"),
		})
	}
	return rep
}
