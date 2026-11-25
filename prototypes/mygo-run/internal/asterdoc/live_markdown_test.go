package asterdoc

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestShouldTriggerSlashAtSafety(t *testing.T) {
	cases := []struct {
		text   string
		cursor int
		wantOK bool
		wantQ  string
	}{
		{"/", 1, true, ""},
		{"/group", 6, true, "group"},
		{"  /plate", 8, true, "plate"},
		{"第一行\n/step", len("第一行\n/step"), true, "step"},
		{"稀释比 1/1000", len("稀释比 1/1000"), false, ""},
		{"路径 ./assets/fig.png", len("路径 ./assets/fig.png"), false, ""},
		{"https://example.com", len("https://example.com"), false, ""},
		{"/group extra", len("/group extra"), false, ""},
	}
	for _, tc := range cases {
		_, q, ok := ShouldTriggerSlashAt(tc.text, tc.cursor)
		if ok != tc.wantOK || q != tc.wantQ {
			t.Errorf("ShouldTriggerSlashAt(%q, %d) = (ok=%v, q=%q), want (ok=%v, q=%q)", tc.text, tc.cursor, ok, q, tc.wantOK, tc.wantQ)
		}
	}
}

func TestComputeLivePreviewDecorationsCursorRevealAndRelations(t *testing.T) {
	src := `---
title: Live Preview 测试
---

# 实验背景
这是一段 **加粗** 文本，附带任务：
- [ ] 配制培养基
关联图：![[assets/a1.png]]{@plate-1:A1 6h} 与断链 ![[assets/bad.png]]{@plate-1:H12}

` + "```asterlab" + `
kind: group_table
id: gt-1
schema: 1
rows:
  - {id: g1, name: 对照组, role: control}
` + "```" + `

` + "```asterlab" + `
kind: plate
id: plate-1
schema: 1
format: 12
groups:
  g1: [A1]
` + "```" + `
`
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	segs := SplitDocumentSegments(src, doc)
	proseSeg := segs[0]
	th := ui.LightTheme()
	tok := theme.GetTokens(th)

	// Cursor away (unfocused): heading style applied, `# ` & `**` hidden, checkbox interactive, 2 relation chips, 1 warning diagnostic for H12
	styles, repls, diags := ComputeLivePreviewDecorations(proseSeg.Text, 0, false, th, tok, doc, proseSeg.Start, true, nil, nil)
	if len(styles) == 0 {
		t.Fatalf("expected heading LineStyle")
	}
	if len(repls) < 5 {
		t.Fatalf("expected live preview replacements for heading/bold/task/relations, got %d", len(repls))
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic for out-of-bounds well H12, got %+v", diags)
	}

	// Cursor focused on the `# 实验背景` line (line 0 or 1 depending on leading newline):
	// hiding replacements on that active line must be suppressed so raw syntax is revealed!
	activeLine := 1 // line 0 is leading `\n`, line 1 is `# 实验背景`
	_, replsFocused, _ := ComputeLivePreviewDecorations(proseSeg.Text, activeLine, true, th, tok, doc, proseSeg.Start, true, nil, nil)
	if len(replsFocused) >= len(repls) {
		t.Fatalf("expected fewer replacements when active heading line reveals raw syntax (unfocused=%d, focused=%d)", len(repls), len(replsFocused))
	}
}

func TestFindRelationAtTriggerBoundaryAndBraceModes(t *testing.T) {
	cases := []struct {
		text       string
		cursor     int
		wantOK     bool
		wantBraces bool
		wantAt     int
	}{
		{"@", 1, true, false, 0},
		{"加入@PBS", len("加入@PBS"), true, false, len("加入")},
		{"加入，@培养基", len("加入，@培养基"), true, false, len("加入，")},
		{"#@ 主步骤", len("#@ 主步骤"), false, false, 0},
		{"#@步骤", len("#@步骤"), false, false, 0},
		{"test@example.com", len("test@example.com"), false, false, 0},
		{"![[a.png]]{@plate", len("![[a.png]]{@plate"), true, true, len("![[a.png]]{")},
		{"加入 @PBS 结束", len("加入 @PBS 结束"), false, false, 0},
	}
	for _, tc := range cases {
		at, braces, ok := findRelationAtTrigger(tc.text, tc.cursor)
		if ok != tc.wantOK || braces != tc.wantBraces || (ok && at != tc.wantAt) {
			t.Errorf("findRelationAtTrigger(%q, %d) = (at=%d, braces=%v, ok=%v), want (at=%d, braces=%v, ok=%v)",
				tc.text, tc.cursor, at, braces, ok, tc.wantAt, tc.wantBraces, tc.wantOK)
		}
	}
}

func TestInlineAtMentionCompletionsAndLivePreviewPills(t *testing.T) {
	src := `---
title: @ 内联引用测试
---

#@ 准备体系
向每孔加入 [@DMEM 培养基](asterlab://pantry?code=R-001) 与 [@96孔板](#plate-1)，参见 [[Fig4/CCK8-实验.asterml|CCK8-实验]]。

` + "```asterlab" + `
kind: group_table
id: gt-1
schema: 1
rows:
  - {id: g1, name: 对照组, role: control}
` + "```" + `

` + "```asterlab" + `
kind: plate
id: plate-1
schema: 1
title: 96孔板
format: 96
groups:
  g1: [A1, A2, A3]
` + "```" + `
`
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	v := &View{
		Doc: doc,
		PantryCompletionItems: []PantryMentionItem{
			{ID: "p-1", Name: "DMEM 培养基", LabelCode: "R-001", Location: "4°C 货架", ItemType: "reagent", Status: "near_expiry"},
			{ID: "p-2", Name: "HeLa P12", LabelCode: "C-008", Location: "-80°C #1 · A2", ItemType: "biosample", Status: "normal"},
		},
		VaultCompletionTargets: []string{"Fig4/CCK8-实验.asterml"},
	}

	// Standalone prose `@`: includes Pantry, Doc modules/steps (WITHOUT flooding with A1/A2/A3 wells), and Vault docs.
	proseItems := v.buildRelationTargetCompletions(false)
	var hasPantry, hasPlate, hasStep, hasVaultDoc, hasIndividualWell bool
	for _, it := range proseItems {
		switch {
		case it.Label == "@DMEM 培养基":
			hasPantry = true
			if it.Insert != "[@DMEM 培养基](asterlab://pantry?code=R-001)" {
				t.Fatalf("unexpected pantry insert: %q", it.Insert)
			}
			if !strings.Contains(it.Detail, "临期") {
				t.Fatalf("expected near-expiry status badge, got %q", it.Detail)
			}
		case it.Label == "@96孔板":
			hasPlate = true
			if it.Insert != "[@96孔板](#plate-1)" {
				t.Fatalf("unexpected plate mention insert: %q", it.Insert)
			}
		case strings.Contains(it.Label, "准备体系"):
			hasStep = true
		case it.Label == "@CCK8-实验":
			hasVaultDoc = true
			if it.Insert != "[[Fig4/CCK8-实验.asterml|CCK8-实验]]" {
				t.Fatalf("unexpected vault doc insert: %q", it.Insert)
			}
		case strings.Contains(it.Label, "A1"):
			hasIndividualWell = true
		}
	}
	if !hasPantry || !hasPlate || !hasStep || !hasVaultDoc {
		t.Fatalf("missing expected standalone @ items: pantry=%v plate=%v step=%v vault=%v", hasPantry, hasPlate, hasStep, hasVaultDoc)
	}
	if hasIndividualWell {
		t.Fatalf("standalone @ menu must not flood with individual plate wells")
	}

	// Brace `{@` mode preserves raw relation target + well syntax for embeds.
	braceItems := v.buildRelationTargetCompletions(true)
	foundWell := false
	for _, it := range braceItems {
		if it.Insert == "@plate-1:A1" {
			foundWell = true
			break
		}
	}
	if !foundWell {
		t.Fatalf("expected {@ mode to retain @plate-1:A1 relation completions")
	}

	// Non-active line renders interactive pills for Pantry, Doc object, and Vault link; active line reveals raw Markdown.
	segs := SplitDocumentSegments(src, doc)
	proseIdx := -1
	for idx, s := range segs {
		if s.Kind == SegmentProse && strings.Contains(s.Text, "asterlab://pantry") {
			proseIdx = idx
			break
		}
	}
	if proseIdx < 0 {
		t.Fatalf("failed to find prose segment containing inline @ mentions")
	}
	proseSeg := segs[proseIdx]
	mentionLine := -1
	for idx, ln := range strings.Split(proseSeg.Text, "\n") {
		if strings.Contains(ln, "asterlab://pantry") {
			mentionLine = idx
			break
		}
	}
	th := ui.LightTheme()
	tok := theme.GetTokens(th)
	pantryByKey := map[string]PantryMentionItem{
		"R-001": v.PantryCompletionItems[0],
	}
	_, replsUnfocused, _ := computeLivePreviewDecorationsFull(proseSeg.Text, mentionLine, false, th, tok, doc, proseSeg.Start, true, pantryByKey, nil, func(string, string) {}, func(string) {}, func(string) {})
	var gotPantryChip, gotPlateChip, gotDocChip bool
	for _, r := range replsUnfocused {
		switch r.Label {
		case "@DMEM 培养基":
			gotPantryChip = r.Interactive
		case "@96孔板":
			gotPlateChip = r.Interactive
		case "CCK8-实验":
			gotDocChip = r.Interactive
		}
	}
	if !gotPantryChip || !gotPlateChip || !gotDocChip {
		t.Fatalf("expected interactive pills on unfocused line: pantry=%v plate=%v doc=%v", gotPantryChip, gotPlateChip, gotDocChip)
	}

	// Cursor on the mention line suppresses mention pills so user edits raw Markdown cleanly.
	_, replsFocused, _ := computeLivePreviewDecorationsFull(proseSeg.Text, mentionLine, true, th, tok, doc, proseSeg.Start, true, pantryByKey, nil, func(string, string) {}, func(string) {}, func(string) {})
	for _, r := range replsFocused {
		if r.Label == "@DMEM 培养基" || r.Label == "@96孔板" || r.Label == "CCK8-实验" {
			t.Fatalf("unexpected mention pill replacement on active cursor line: %+v", r)
		}
	}

	// focusRelationTarget resolves plate module IDs cleanly
	v.focusRelationTarget("plate-1", "")
	if v.ScrollToModuleID != "plate-1" {
		t.Fatalf("expected focusRelationTarget to set ScrollToModuleID=plate-1, got %q", v.ScrollToModuleID)
	}
}
