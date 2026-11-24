package asterdoc

import (
	"sync"
	"sync/atomic"
)

const renderCacheMaxEntries = 64

type segmentCacheKey struct {
	hash          uint64
	length        int
	modCount      int
	excludeOffset int
}

type stepTreeCacheKey struct {
	hash          uint64
	length        int
	modCount      int
	excludeOffset int
}

type docCacheKey struct {
	hash     uint64
	length   int
	modCount int
}

type svgMetricsCacheVal struct {
	vbX, vbY, vbW, vbH float64
	labels             []svgTextLabel
}

type latexCacheVal struct {
	ast MathNode
	err error
}

var (
	renderCacheMu   sync.RWMutex
	segCache        = make(map[segmentCacheKey][]DocSegment, 16)
	stepCache       = make(map[stepTreeCacheKey]*StepTree, 16)
	outlineCache    = make(map[docCacheKey][]*OutlineNode, 16)
	relCache        = make(map[docCacheKey][]ResolvedRelation, 16)
	svgMetricsCache = make(map[uint64]svgMetricsCacheVal, 32)
	latexASTCache   = make(map[string]latexCacheVal, 32)

	renderCacheHits   atomic.Uint64
	renderCacheMisses atomic.Uint64
)

// FastSourceHash computes a zero-allocation 64-bit FNV-1a digest over an
// optional namespace key and source text for per-frame render memoization.
func FastSourceHash(key, source string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for i := 0; i < len(key); i++ {
		h ^= uint64(key[i])
		h *= prime64
	}
	if key != "" {
		h ^= 0xff
		h *= prime64
	}
	for i := 0; i < len(source); i++ {
		h ^= uint64(source[i])
		h *= prime64
	}
	return h
}

// ResetRenderCaches clears all memoized document/SVG/LaTeX caches and counters.
func ResetRenderCaches() {
	renderCacheMu.Lock()
	clear(segCache)
	clear(stepCache)
	clear(outlineCache)
	clear(relCache)
	clear(svgMetricsCache)
	clear(latexASTCache)
	renderCacheMu.Unlock()
	renderCacheHits.Store(0)
	renderCacheMisses.Store(0)
}

// RenderCacheStats returns cumulative (hits, misses) across render caches.
func RenderCacheStats() (hits, misses uint64) {
	return renderCacheHits.Load(), renderCacheMisses.Load()
}

func docKeyParts(source string, doc *Document) (uint64, int, int) {
	if source == "" && doc != nil {
		source = doc.Source
	}
	mods := 0
	if doc != nil {
		mods = len(doc.Modules)
	}
	return FastSourceHash("", source), len(source), mods
}

// CachedSplitDocumentSegments memoizes SplitDocumentSegmentsExcludingLine across
// frames when source, module count, and active byte offset are unchanged.
func CachedSplitDocumentSegments(source string, doc *Document, activeByteOffset int) []DocSegment {
	h, ln, mods := docKeyParts(source, doc)
	key := segmentCacheKey{hash: h, length: ln, modCount: mods, excludeOffset: activeByteOffset}

	renderCacheMu.RLock()
	if cached, ok := segCache[key]; ok {
		renderCacheMu.RUnlock()
		renderCacheHits.Add(1)
		return cached
	}
	renderCacheMu.RUnlock()

	renderCacheMisses.Add(1)
	res := SplitDocumentSegmentsExcludingLine(source, doc, activeByteOffset)

	renderCacheMu.Lock()
	if len(segCache) >= renderCacheMaxEntries {
		clear(segCache)
	}
	segCache[key] = res
	renderCacheMu.Unlock()
	return res
}

// CachedParseStepTree memoizes ParseStepTreeExcludingLine across frames.
func CachedParseStepTree(source string, doc *Document, excludeOff int) *StepTree {
	h, ln, mods := docKeyParts(source, doc)
	key := stepTreeCacheKey{hash: h, length: ln, modCount: mods, excludeOffset: excludeOff}

	renderCacheMu.RLock()
	if cached, ok := stepCache[key]; ok {
		renderCacheMu.RUnlock()
		renderCacheHits.Add(1)
		return cached
	}
	renderCacheMu.RUnlock()

	renderCacheMisses.Add(1)
	res := ParseStepTreeExcludingLine(source, doc, excludeOff)

	renderCacheMu.Lock()
	if len(stepCache) >= renderCacheMaxEntries {
		clear(stepCache)
	}
	stepCache[key] = res
	renderCacheMu.Unlock()
	return res
}

// CachedParseDocumentOutline memoizes ParseDocumentOutline across frames.
func CachedParseDocumentOutline(source string, doc *Document) []*OutlineNode {
	h, ln, mods := docKeyParts(source, doc)
	key := docCacheKey{hash: h, length: ln, modCount: mods}

	renderCacheMu.RLock()
	if cached, ok := outlineCache[key]; ok {
		renderCacheMu.RUnlock()
		renderCacheHits.Add(1)
		return cached
	}
	renderCacheMu.RUnlock()

	renderCacheMisses.Add(1)
	res := ParseDocumentOutline(source, doc)

	renderCacheMu.Lock()
	if len(outlineCache) >= renderCacheMaxEntries {
		clear(outlineCache)
	}
	outlineCache[key] = res
	renderCacheMu.Unlock()
	return res
}

// CachedResolveRelations memoizes ResolveRelations across frames.
func CachedResolveRelations(doc *Document) []ResolvedRelation {
	if doc == nil {
		return nil
	}
	h, ln, mods := docKeyParts(doc.Source, doc)
	key := docCacheKey{hash: h, length: ln, modCount: mods}

	renderCacheMu.RLock()
	if cached, ok := relCache[key]; ok {
		renderCacheMu.RUnlock()
		renderCacheHits.Add(1)
		return cached
	}
	renderCacheMu.RUnlock()

	renderCacheMisses.Add(1)
	res := ResolveRelations(doc)

	renderCacheMu.Lock()
	if len(relCache) >= renderCacheMaxEntries {
		clear(relCache)
	}
	relCache[key] = res
	renderCacheMu.Unlock()
	return res
}

// CachedSVGMetrics memoizes SVG viewBox and label extraction across frames.
func CachedSVGMetrics(rawSVG string) (vbX, vbY, vbW, vbH float64, labels []svgTextLabel) {
	h := FastSourceHash("svg", rawSVG) ^ uint64(len(rawSVG))*1099511628211

	renderCacheMu.RLock()
	if cached, ok := svgMetricsCache[h]; ok {
		renderCacheMu.RUnlock()
		renderCacheHits.Add(1)
		return cached.vbX, cached.vbY, cached.vbW, cached.vbH, cached.labels
	}
	renderCacheMu.RUnlock()

	renderCacheMisses.Add(1)
	vbX, vbY, vbW, vbH, labels = parseSVGMetricsAndLabels(rawSVG)

	renderCacheMu.Lock()
	if len(svgMetricsCache) >= renderCacheMaxEntries {
		clear(svgMetricsCache)
	}
	svgMetricsCache[h] = svgMetricsCacheVal{
		vbX:    vbX,
		vbY:    vbY,
		vbW:    vbW,
		vbH:    vbH,
		labels: labels,
	}
	renderCacheMu.Unlock()
	return vbX, vbY, vbW, vbH, labels
}

// CachedLatexAST memoizes ParseLatexMath results across frames.
func CachedLatexAST(src string) (MathNode, error) {
	renderCacheMu.RLock()
	if cached, ok := latexASTCache[src]; ok {
		renderCacheMu.RUnlock()
		renderCacheHits.Add(1)
		return cached.ast, cached.err
	}
	renderCacheMu.RUnlock()

	renderCacheMisses.Add(1)
	ast, err := ParseLatexMath(src)

	renderCacheMu.Lock()
	if len(latexASTCache) >= renderCacheMaxEntries {
		clear(latexASTCache)
	}
	latexASTCache[src] = latexCacheVal{ast: ast, err: err}
	renderCacheMu.Unlock()
	return ast, err
}
