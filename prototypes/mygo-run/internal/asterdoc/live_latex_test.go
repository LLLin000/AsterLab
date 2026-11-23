package asterdoc

import (
	"strings"
	"testing"

	"asterlab-mygo-prototype/internal/theme"
)

func TestLivePreviewLaTeX_InlineAndDisplayMath(t *testing.T) {
	tok := theme.GetTokens(nil)
	src := strings.Join([]string{
		"- 理论体积：$V = \\pi r^2 h \\approx 201.1\\ \\mu\\text{L}$，价格 $10 到 $20 不误触发",
		"$$\\text{SR} = \\frac{W_t - W_0}{W_0} \\times 100\\%$$",
		"#@ 1. GelMA 前驱液配制",
	}, "\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Non-active line (activeLine = -1): both inline $...$ and display $$...$$ render replacements,
	// currency "$10 到 $20" is NOT turned into math, and `#@ 1. ` hides duplicate leading number!
	_, repls, _ := ComputeLivePreviewDecorations(src, -1, false, nil, tok, doc, 0, false, nil, nil)
	foundInline := false
	foundDisplay := false
	foundStepStripped := false
	for _, r := range repls {
		if strings.Contains(r.Label, "π") && strings.Contains(r.Label, "μL") {
			foundInline = true
		}
		if strings.Contains(r.Label, "SR") && strings.Contains(r.Label, "Wₜ") {
			foundDisplay = true
		}
		if strings.Contains(r.Label, "10 到") {
			t.Fatalf("currency span accidentally treated as LaTeX math: %+v", r)
		}
		if r.Label == "STEP 1" && strings.Contains(src[r.Start:r.End], "1.") {
			foundStepStripped = true
		}
	}
	if !foundInline {
		t.Fatalf("expected Obsidian inline LaTeX replacement for $V = \\pi r^2 h...$")
	}
	if !foundDisplay {
		t.Fatalf("expected Obsidian display LaTeX replacement for $$\\text{SR}...$$")
	}
	if !foundStepStripped {
		t.Fatalf("expected step pill replacement to strip redundant '1. ' prefix on non-active line")
	}

	// Active line (cursor on line 0): inline LaTeX reveals raw `$...$` syntax for editing!
	_, activeRepls, _ := ComputeLivePreviewDecorations(src, 0, true, nil, tok, doc, 0, false, nil, nil)
	for _, r := range activeRepls {
		if strings.Contains(r.Label, "π") {
			t.Fatalf("active cursor line should reveal raw LaTeX syntax, got replacement %+v", r)
		}
	}
}

func TestSpecGuardrails_DataTableFormulaKindAndBioUCUMUnits(t *testing.T) {
	// 1. data_table column with formula but value_kind: number MUST fail with actionable hint
	badDT := "---\ntitle: bad\n---\n\n```asterlab\nkind: data_table\nid: dt1\nschema: 1\ncolumns:\n  - {key: d1, label: d1, value_kind: number, unit: mm}\n  - {key: d2, label: d2, value_kind: number, unit: mm}\n  - {key: avg, label: avg, value_kind: number, unit: mm, formula: \"(d1 + d2) / 2\"}\n```\n"
	if _, err := Parse(badDT); err == nil || !strings.Contains(err.Error(), "value_kind: formula") {
		t.Fatalf("expected actionable error for formula column with value_kind=number, got: %v", err)
	}

	// 2. Bio UCUM units U/mL, U, g/mL must parse cleanly in param_table & data_table
	bioUnits := "---\ntitle: bio\n---\n\n```asterlab\nkind: param_table\nid: pt1\nschema: 1\nrows:\n  - {key: enz, label: 胶原酶 II, value: 2, unit: U/mL}\n  - {key: tot, label: 总酶活, value: 60, unit: U}\n  - {key: ext, label: 浸提比例, value: 0.1, unit: g/mL}\n```\n"
	if _, err := Parse(bioUnits); err != nil {
		t.Fatalf("expected bio UCUM units (U/mL, U, g/mL) to parse cleanly, got: %v", err)
	}

	// 3. groupRoleLabel localizes blank & positive_control cleanly
	if got := groupRoleLabel("blank"); got != "空白组" {
		t.Fatalf("expected blank -> 空白组, got %q", got)
	}
	if got := groupRoleLabel("positive_control"); got != "阳性对照" {
		t.Fatalf("expected positive_control -> 阳性对照, got %q", got)
	}
}

func TestNLSuggest_MastermixPreservesMLUnitAndDedupesStepModules(t *testing.T) {
	// 1. Mastermix with "每孔 2 mL" must NOT turn into 2 uL!
	rep := SuggestSentenceModules(nil, "12孔板预混液，每孔 2 mL，按 25% 余量配制")
	if len(rep.Suggestions) != 1 || rep.Suggestions[0].Kind != "calculator" {
		t.Fatalf("expected 1 calculator suggestion, got %+v", rep.Suggestions)
	}
	if !strings.Contains(rep.Suggestions[0].Summary, "mL") || strings.Contains(rep.Suggestions[0].Summary, "µL") {
		t.Fatalf("expected mL unit preserved in mastermix summary, got %q", rep.Suggestions[0].Summary)
	}

	// 2. SuggestDocumentModules skips blockquote metadata and skips timer when step already owns a timer
	src := strings.Join([]string{
		"---",
		"title: 去重测试",
		"---",
		"",
		"> **实验目标**：405 nm 紫外固化 30 s 后测定溶胀率。",
		"",
		"#@ 光固化成型",
		"- [ ] 405 nm 紫外照射固化 30 s",
		"",
		"```asterlab",
		"kind: timer",
		"id: t-uv",
		"schema: 1",
		"title: 光固化计时",
		"mode: countdown",
		"duration: {value: 30, unit: s}",
		"```",
	}, "\n")
	doc, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	docRep := SuggestDocumentModules(doc)
	if len(docRep.Suggestions) != 0 {
		t.Fatalf("expected 0 suggestions (blockquote gated & existing timer deduped), got %+v", docRep.Suggestions)
	}
}

func TestNativeScriptRunsAndScientificNotation(t *testing.T) {
	runs := parseRichScriptRuns(`10^{-3} \sim 10^{-5} + H_2O`)
	var sawSup, sawSub bool
	for _, r := range runs {
		if r.Kind == scriptSup && r.Text == "-3" {
			sawSup = true
		}
		if r.Kind == scriptSub && r.Text == "2" {
			sawSub = true
		}
	}
	if !sawSup || !sawSub {
		t.Fatalf("expected native sup(-3) and sub(2) runs, got %+v", runs)
	}

	if got := formatSciCompactText(1000000); got != "10⁶" {
		t.Fatalf("1e6 -> %q, want 10⁶", got)
	}
	if got := formatSciCompactText(14200000); got != "1.42×10⁷" {
		t.Fatalf("1.42e7 -> %q, want 1.42×10⁷", got)
	}
	if got := formatSciCompactText(0.0001); got != "10⁻⁴" {
		t.Fatalf("1e-4 -> %q, want 10⁻⁴", got)
	}
	if got := formatSciCompactText(250); got != "250" {
		t.Fatalf("250 should stay plain, got %q", got)
	}
}
