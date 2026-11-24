package asterdoc

import (
	"testing"
)

func TestFastSourceHashDeterminismAndSeparation(t *testing.T) {
	h1 := FastSourceHash("doc-a", "hello asterlab")
	h2 := FastSourceHash("doc-a", "hello asterlab")
	h3 := FastSourceHash("doc-b", "hello asterlab")
	h4 := FastSourceHash("doc-a", "hello asterlab!")
	if h1 != h2 {
		t.Fatalf("expected identical hash, got %x vs %x", h1, h2)
	}
	if h1 == h3 || h1 == h4 {
		t.Fatalf("expected distinct hashes for key/content changes, got %x / %x / %x", h1, h3, h4)
	}
}

func TestRenderCacheHitsAndInvalidation(t *testing.T) {
	ResetRenderCaches()
	src := "---\ntitle: Cache Smoke\n---\n#@ Step 1: Prepare\n- Mix buffer\n\n" +
		"```asterlab:param_table@1#p1\n" +
		"rows:\n" +
		"  - name: NaCl\n" +
		"    value: 150\n" +
		"    unit: mM\n" +
		"```\n"
	doc, err := Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	s1 := CachedSplitDocumentSegments(src, doc, -1)
	s2 := CachedSplitDocumentSegments(src, doc, -1)
	if len(s1) == 0 || len(s1) != len(s2) {
		t.Fatalf("unexpected segments lengths: %d vs %d", len(s1), len(s2))
	}
	st1 := CachedParseStepTree(src, doc, -1)
	st2 := CachedParseStepTree(src, doc, -1)
	if st1 != st2 {
		t.Fatalf("expected identical StepTree pointer on cache hit")
	}
	o1 := CachedParseDocumentOutline(src, doc)
	o2 := CachedParseDocumentOutline(src, doc)
	if len(o1) != len(o2) {
		t.Fatalf("expected matching outline lengths")
	}
	r1 := CachedResolveRelations(doc)
	r2 := CachedResolveRelations(doc)
	if len(r1) != len(r2) {
		t.Fatalf("expected matching relation lengths")
	}

	svg := `<svg viewBox="0 0 240 120"><text x="10" y="20">Well A1</text></svg>`
	_, _, w1, h1, l1 := CachedSVGMetrics(svg)
	_, _, w2, h2, l2 := CachedSVGMetrics(svg)
	if w1 != 240 || h1 != 120 || w1 != w2 || h1 != h2 || len(l1) != len(l2) {
		t.Fatalf("unexpected cached svg metrics: (%v,%v) vs (%v,%v)", w1, h1, w2, h2)
	}

	_, err1 := CachedLatexAST(`\frac{C_1 V_1}{C_2}`)
	_, err2 := CachedLatexAST(`\frac{C_1 V_1}{C_2}`)
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected latex cache error: %v / %v", err1, err2)
	}

	hits, misses := RenderCacheStats()
	if misses != 6 || hits != 6 {
		t.Fatalf("expected 6 hits and 6 misses, got hits=%d misses=%d", hits, misses)
	}

	// Offset change should invalidate segment cache lookup.
	_ = CachedSplitDocumentSegments(src, doc, 32)
	hits, misses = RenderCacheStats()
	if misses != 7 || hits != 6 {
		t.Fatalf("expected offset miss increment to 7, got hits=%d misses=%d", hits, misses)
	}
}
