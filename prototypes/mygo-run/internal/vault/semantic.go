package vault

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"asterlab-mygo-prototype/internal/asterdoc"
)

type semanticDocEntry struct {
	modTime int64
	size    int64
	snippet string
	vec     []float32
	tokens  map[string]float32
}

// RelatedDocHit describes a semantically related sibling experiment in the Vault.
type RelatedDocHit struct {
	RelPath string
	Title   string
	Date    string
	Folder  string
	Score   float32
	Reason  string
}

func docSemanticText(d IndexedDoc) (text, preview string) {
	var parts []string
	if strings.TrimSpace(d.Title) != "" {
		parts = append(parts, strings.TrimSpace(d.Title))
	}
	if strings.TrimSpace(d.Folder) != "" {
		parts = append(parts, strings.TrimSpace(d.Folder))
	}
	if len(d.Tags) > 0 {
		parts = append(parts, strings.Join(d.Tags, " "))
	}
	if len(d.Steps) > 0 {
		parts = append(parts, strings.Join(d.Steps, " "))
	}
	inFence := false
	addedLines := 0
	for _, raw := range d.Lines {
		tr := strings.TrimSpace(raw)
		if strings.HasPrefix(tr, "```") {
			inFence = !inFence
			continue
		}
		if inFence || tr == "" || tr == "---" || strings.HasPrefix(tr, "asterml:") || strings.HasPrefix(tr, "id:") || strings.HasPrefix(tr, "schema:") {
			continue
		}
		if preview == "" && !strings.HasPrefix(tr, "#") {
			preview = tr
			if len([]rune(preview)) > 96 {
				preview = string([]rune(preview)[:96]) + "…"
			}
		}
		parts = append(parts, tr)
		addedLines++
		if addedLines >= 6 {
			break
		}
	}
	joined := strings.Join(parts, " ")
	runes := []rune(joined)
	if len(runes) > 220 {
		joined = string(runes[:220])
	}
	return joined, preview
}

func buildCharBigramTokens(text string) map[string]float32 {
	out := map[string]float32{}
	clean := strings.ToLower(strings.TrimSpace(text))
	if clean == "" {
		return out
	}
	// ASCII/numeric words + CJK bigrams & unigrams so both English scientific terms (GelMA, CCK-8, PCR)
	// and Chinese biomedical phrases match smoothly even without the optional ONNX DLL loaded.
	var wordBuf []rune
	var cjkSeq []rune
	flushWord := func() {
		if len(wordBuf) >= 2 {
			w := string(wordBuf)
			out["w:"+w] += 1.0
		}
		wordBuf = wordBuf[:0]
	}
	flushCJK := func() {
		for i := 0; i < len(cjkSeq); i++ {
			if i+1 < len(cjkSeq) {
				out["bg:"+string(cjkSeq[i:i+2])] += 1.5
			}
			out["ch:"+string(cjkSeq[i])] += 0.4
		}
		cjkSeq = cjkSeq[:0]
	}
	for _, r := range clean {
		if unicode.Is(unicode.Han, r) {
			flushWord()
			cjkSeq = append(cjkSeq, r)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			flushCJK()
			wordBuf = append(wordBuf, r)
		} else {
			flushWord()
			flushCJK()
		}
	}
	flushWord()
	flushCJK()

	var sumSq float64
	for _, v := range out {
		sumSq += float64(v * v)
	}
	if sumSq > 0 {
		norm := float32(math.Sqrt(sumSq))
		for k, v := range out {
			out[k] = v / norm
		}
	}
	return out
}

func tokenCosine(a, b map[string]float32) float32 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	var dot float32
	for k, va := range a {
		if vb, ok := b[k]; ok {
			dot += va * vb
		}
	}
	return dot
}

func vecCosine(a, b []float32) float32 {
	n := len(a)
	if n == 0 || len(b) != n {
		return 0
	}
	var dot float32
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
	}
	return dot
}

func (idx *VaultIndex) ensureSemanticEntry(d IndexedDoc, router *asterdoc.NLOnnxRouter) semanticDocEntry {
	idx.semMu.RLock()
	if idx.semCache != nil {
		if cached, ok := idx.semCache[d.RelPath]; ok && cached.modTime == d.ModTimeUnix && cached.size == d.Size && (router == nil || len(cached.vec) > 0) {
			idx.semMu.RUnlock()
			return cached
		}
	}
	idx.semMu.RUnlock()

	semTxt, prev := docSemanticText(d)
	entry := semanticDocEntry{
		modTime: d.ModTimeUnix,
		size:    d.Size,
		snippet: prev,
		tokens:  buildCharBigramTokens(semTxt),
	}
	if router != nil && router.Engine() != nil && semTxt != "" {
		if vec, err := router.Engine().Encode("retrieve", semTxt); err == nil && len(vec) > 0 {
			entry.vec = vec
		}
	}

	idx.semMu.Lock()
	if idx.semCache == nil {
		idx.semCache = map[string]semanticDocEntry{}
	}
	idx.semCache[d.RelPath] = entry
	idx.semMu.Unlock()
	return entry
}

// RelatedDocs returns up to limit semantically similar non-template sibling experiments.
// Uses the 512-dim BGE ONNX encoder when loaded, with deterministic CJK-bigram + token fallback.
func (idx *VaultIndex) RelatedDocs(targetRel string, limit int) []RelatedDocHit {
	if idx == nil || strings.TrimSpace(targetRel) == "" {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}
	idx.mu.RLock()
	targetDoc, ok := idx.docs[targetRel]
	if !ok {
		idx.mu.RUnlock()
		return nil
	}
	docs := make([]IndexedDoc, 0, len(idx.docs))
	for rel, d := range idx.docs {
		if rel == targetRel || d.IsTemplate {
			continue
		}
		docs = append(docs, d)
	}
	idx.mu.RUnlock()
	if len(docs) == 0 {
		return nil
	}

	router := asterdoc.DefaultNLOnnxRouter()
	targetEnt := idx.ensureSemanticEntry(targetDoc, router)

	var out []RelatedDocHit
	for _, d := range docs {
		ent := idx.ensureSemanticEntry(d, router)
		var score float32
		minThresh := float32(0.18)
		if len(targetEnt.vec) > 0 && len(ent.vec) == len(targetEnt.vec) {
			score = vecCosine(targetEnt.vec, ent.vec)
			minThresh = 0.50
		} else {
			score = tokenCosine(targetEnt.tokens, ent.tokens)
		}
		if score < minThresh {
			continue
		}
		out = append(out, RelatedDocHit{
			RelPath: d.RelPath,
			Title:   d.Title,
			Date:    d.Date,
			Folder:  d.Folder,
			Score:   score,
			Reason:  "语义相近",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].RelPath < out[j].RelPath
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// HybridSearch combines exact full-text matches from SearchFullText with semantic similarity
// recall using BGE 512-dim vectors (or CJK-bigram tokens in lightweight builds).
func (idx *VaultIndex) HybridSearch(query string, limit int) []SearchHit {
	q := strings.TrimSpace(query)
	if idx == nil || q == "" {
		return nil
	}
	if limit <= 0 {
		limit = 50
	}
	hits := idx.SearchFullText(q, limit)
	if len(hits) >= limit || len([]rune(q)) < 2 {
		return hits
	}

	seen := make(map[string]bool, len(hits))
	for _, h := range hits {
		seen[h.RelPath] = true
	}

	idx.mu.RLock()
	docs := make([]IndexedDoc, 0, len(idx.docs))
	for rel, d := range idx.docs {
		if !d.IsTemplate && !seen[rel] {
			docs = append(docs, d)
		}
	}
	idx.mu.RUnlock()
	if len(docs) == 0 {
		return hits
	}

	router := asterdoc.DefaultNLOnnxRouter()
	var qVec []float32
	if router != nil && router.Engine() != nil {
		if v, err := router.Engine().Encode("retrieve", q); err == nil && len(v) > 0 {
			qVec = v
		}
	}
	qTok := buildCharBigramTokens(q)

	type cand struct {
		doc     IndexedDoc
		score   float32
		snippet string
	}
	var cands []cand
	for _, d := range docs {
		ent := idx.ensureSemanticEntry(d, router)
		var score float32
		minThresh := float32(0.16)
		if len(qVec) > 0 && len(ent.vec) == len(qVec) {
			score = vecCosine(qVec, ent.vec)
			minThresh = 0.45
		} else {
			score = tokenCosine(qTok, ent.tokens)
		}
		if score >= minThresh {
			snip := firstNonEmptyStr(ent.snippet, fmt.Sprintf("%s · %s", firstNonEmptyStr(d.Folder, "根目录"), firstNonEmptyStr(d.Date, "未排期")))
			cands = append(cands, cand{doc: d, score: score, snippet: "语义相近 · " + snip})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score == cands[j].score {
			return cands[i].doc.RelPath < cands[j].doc.RelPath
		}
		return cands[i].score > cands[j].score
	})
	for _, cd := range cands {
		if len(hits) >= limit {
			break
		}
		hits = append(hits, SearchHit{
			RelPath: cd.doc.RelPath,
			Title:   cd.doc.Title,
			Folder:  cd.doc.Folder,
			LineNum: 0,
			Snippet: cd.snippet,
			Date:    cd.doc.Date,
			Status:  cd.doc.Status,
		})
	}
	return hits
}
