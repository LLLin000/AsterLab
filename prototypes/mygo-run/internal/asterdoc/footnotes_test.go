package asterdoc

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestParseFootnotesOrderingTrailingAndCodeMasking(t *testing.T) {
	src := `---
title: 脚注解析测试
---

按每孔 100 uL 加入 DMEM[^2]，边缘孔用 PBS 封闭[^1]。
行内代码中的 ` + "`[^ignore]`" + ` 不应被当成脚注。

` + "```asterlab" + `
kind: prose
id: p-1
schema: 1
text: "代码围栏内 [^also_ignore]"
` + "```" + `

[^1]: 边缘 36 孔防蒸发。
  第二行缩进续写说明。
[^2]: 避光分装配制。
`
	if !HasFootnotes(src) {
		t.Fatalf("expected HasFootnotes=true")
	}
	doc := ParseFootnotes(src)
	if len(doc.Entries) != 2 {
		t.Fatalf("expected 2 footnotes (ignoring code spans/fences), got %d: %+v", len(doc.Entries), doc.Entries)
	}
	// First referenced is [^2] before [^1]
	if doc.Entries[0].ID != "2" || doc.Entries[0].Number != 1 {
		t.Fatalf("expected [^2] first (#1), got %+v", doc.Entries[0])
	}
	if doc.Entries[1].ID != "1" || doc.Entries[1].Number != 2 {
		t.Fatalf("expected [^1] second (#2), got %+v", doc.Entries[1])
	}
	if !strings.Contains(doc.Entries[1].Body, "第二行缩进续写说明") {
		t.Fatalf("expected multiline footnote body, got %q", doc.Entries[1].Body)
	}
	wantIdx := strings.Index(src, "[^1]:")
	if doc.TrailingDefStart != wantIdx {
		t.Fatalf("TrailingDefStart = %d, want %d", doc.TrailingDefStart, wantIdx)
	}
}

func TestFootnoteMutationsInsertUpsertDelete(t *testing.T) {
	src := "# 实验步骤\n吹打混匀避免气泡。\n"
	if got := NextFootnoteID(src); got != "1" {
		t.Fatalf("NextFootnoteID = %q, want 1", got)
	}
	insertAt := strings.Index(src, "。")
	s1, id1 := InsertFootnoteAtOffset(src, insertAt, "建议沿孔壁缓慢加液")
	if id1 != "1" || !strings.Contains(s1, "避免气泡[^1]。") || !strings.Contains(s1, "[^1]: 建议沿孔壁缓慢加液") {
		t.Fatalf("unexpected insert output:\n%s", s1)
	}
	s2 := UpsertFootnoteBody(s1, "1", "改写后的两行提示\n保留空白对照")
	d2 := ParseFootnotes(s2)
	if len(d2.Entries) != 1 || !strings.Contains(d2.Entries[0].Body, "保留空白对照") {
		t.Fatalf("UpsertFootnoteBody failed: %+v\nsrc:\n%s", d2.Entries, s2)
	}
	insertAt2 := strings.Index(s2, "步骤") + len("步骤")
	s3, id2 := InsertFootnoteAtOffset(s2, insertAt2, "注意冰上操作")
	if id2 != "2" || !strings.Contains(s3, "[^2]: 注意冰上操作") {
		t.Fatalf("second insert failed (id=%q):\n%s", id2, s3)
	}
	s4 := DeleteFootnote(s3, "1")
	if strings.Contains(s4, "[^1]") || !strings.Contains(s4, "[^2]: 注意冰上操作") {
		t.Fatalf("DeleteFootnote failed:\n%s", s4)
	}
}

func TestLivePreviewFootnoteSuperscriptAndCursorReveal(t *testing.T) {
	src := "---\ntitle: Live 脚注视图测试\n---\n\n#@ 1. 加药\n避光孵育不超过 2h[^noteA]。\n\n[^noteA]: 超过 2h 会导致 OD 饱和。\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	fnDoc := ParseFootnotes(src)
	th := ui.LightTheme()
	tok := theme.GetTokens(th)

	prose := "避光孵育不超过 2h[^noteA]。"
	hovered := ""
	hooks := &liveFootnoteHooks{
		doc:      fnDoc,
		showSide: true,
		editable: true,
		onHover: func(id string) {
			hovered = id
		},
	}
	_, replsUnfocused, _ := computeLivePreviewDecorationsFull(prose, 99, false, th, tok, doc, 0, true, nil, hooks, nil, nil, nil, nil)
	foundSup := false
	for _, r := range replsUnfocused {
		if r.Label == "脚注 "+strconv.Itoa(1) && r.Interactive {
			foundSup = true
		}
	}
	if !foundSup {
		t.Fatalf("expected interactive footnote superscript pill on unfocused line, got %+v", replsUnfocused)
	}
	_ = hovered

	// Focused cursor on the active line reveals raw `[^noteA]` syntax for smooth editing.
	_, replsFocused, _ := computeLivePreviewDecorationsFull(prose, 0, true, th, tok, doc, 0, true, nil, hooks, nil, nil, nil, nil)
	for _, r := range replsFocused {
		if r.Label == "脚注 1" {
			t.Fatalf("expected raw footnote marker on active cursor line, got replacement %+v", r)
		}
	}
}
