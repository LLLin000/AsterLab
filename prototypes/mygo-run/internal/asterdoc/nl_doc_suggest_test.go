package asterdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuggestDocumentModules_FiltersNonActionableAndCompilesValidYAML(t *testing.T) {
	src := strings.Join([]string{
		"---",
		"title: CFU 现代写法智能建议测试",
		"scheduled_date: \"2026-11-24\"",
		"stage: planned",
		"---",
		"",
		"## 实验材料",
		"- VEGF 母液 100 ug/mL（-80°C 冻存）",
		"",
		"#@ 1. 稀释与恒温培养 {#t0}",
		"- [ ] 用无菌 PBS 轻柔吹打混匀并弃去上清",
		"- [ ] 将 VEGF 母液从 100 ug/mL 稀释至 50 ng/mL，配制 8 mL 工作液",
		"- [ ] 置于 37°C 恒温培养箱倒置培养 24 h",
		"",
		"## 复盘与讨论",
		"- 上一轮培养 24 h 后菌落偏密，本轮增加 10^-5 稀释度。",
		"",
	}, "\n")

	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	rep := SuggestDocumentModules(doc)
	if rep.ScannedLines == 0 {
		t.Fatalf("expected ScannedLines > 0")
	}
	if len(rep.Suggestions) < 2 {
		t.Fatalf("expected >= 2 actionable suggestions (dilution + timer), got %d: %+v", len(rep.Suggestions), rep.Suggestions)
	}

	seenTimer := false
	seenCalc := false
	for _, s := range rep.Suggestions {
		if strings.Contains(s.OriginalLine, "弃去上清") {
			t.Fatalf("negative washing line should not be suggested: %+v", s)
		}
		if strings.Contains(s.OriginalLine, "上一轮") {
			t.Fatalf("review section line must be gated out by SciGraph: %+v", s)
		}
		if s.StepTitle == "" {
			t.Fatalf("expected step title populated from #@ heading, got empty on %+v", s)
		}
		if !strings.Contains(s.YAML, "```asterlab") {
			t.Fatalf("expected fenced asterlab YAML in suggestion, got %q", s.YAML)
		}
		// Every compiled suggestion YAML must pass asterdoc.Parse cleanly!
		if _, pErr := Parse("# test\n\n" + s.YAML + "\n"); pErr != nil {
			t.Fatalf("compiled suggestion YAML failed asterdoc.Parse: %v\nYAML:\n%s", pErr, s.YAML)
		}
		if s.Kind == "timer" {
			seenTimer = true
		}
		if s.Kind == "calculator" {
			seenCalc = true
		}
	}
	if !seenTimer || !seenCalc {
		t.Fatalf("expected both timer and calculator suggestions, got timer=%v calc=%v (%+v)", seenTimer, seenCalc, rep.Suggestions)
	}
}

func TestSuggestSentenceModules_SingleSentenceWithDocContext(t *testing.T) {
	rep := SuggestSentenceModules(nil, "37°C 倒置培养 24 h")
	if len(rep.Suggestions) != 1 || rep.Suggestions[0].Kind != "timer" {
		t.Fatalf("expected 1 timer suggestion, got %+v", rep.Suggestions)
	}
	if _, err := Parse("# test\n\n" + rep.Suggestions[0].YAML + "\n"); err != nil {
		t.Fatalf("compiled single-sentence YAML failed Parse: %v", err)
	}
}

func TestModernSOPTemplates_UseHeadingStepsAndNoLegacyBoilerplate(t *testing.T) {
	templateDir := filepath.Join("..", "..", "templates")
	files := []string{
		"sop-cfu.asterml",
		"sop-inhibition-zone.asterml",
		"sop-huvec-tube.asterml",
		"sop-rt-qpcr.asterml",
	}
	forbiddenKinds := map[string]bool{
		"step":       true,
		"workflow":   true,
		"assay":      true,
		"experiment": true,
		"prose":      true,
		"subflow":    true,
	}

	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(templateDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		src := string(raw)
		fm, _, ok := ScanFrontmatter(src)
		if !ok || strings.TrimSpace(fm.Title) == "" {
			t.Fatalf("%s must have valid YAML frontmatter with title", name)
		}
		doc, err := Parse(src)
		if err != nil {
			t.Fatalf("%s failed asterdoc.Parse: %v", name, err)
		}
		for _, m := range doc.Modules {
			if forbiddenKinds[m.Kind] {
				t.Fatalf("%s must not contain legacy module kind %q", name, m.Kind)
			}
			// Verify planned templates never pre-fill fake measured readings in data_table
			if m.Kind == "data_table" {
				for _, row := range mapSlice(m.Data["rows"]) {
					for _, fakeKey := range []string{"colonies", "d1", "d2", "ct", "total_length"} {
						if _, has := row[fakeKey]; has {
							t.Fatalf("%s data_table %s must not pre-fill fake measurement %q", name, m.ID, fakeKey)
						}
					}
				}
			}
		}
		tree := ParseStepTree(src, doc)
		if tree == nil || len(tree.Roots) < 3 {
			t.Fatalf("%s expected >= 3 #@ heading steps, got %+v", name, tree)
		}
		for _, sn := range tree.Flat {
			if sn.SourceKind != "heading" {
				t.Fatalf("%s step %q expected SourceKind=heading, got %q", name, sn.Title, sn.SourceKind)
			}
		}
	}
}
