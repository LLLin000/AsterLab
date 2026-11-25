package asterdoc

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// FootnoteEntry represents one Markdown footnote (`[^id]` + optional `[^id]: body`)
// ordered by first inline reference (followed by any unreferenced definitions).
type FootnoteEntry struct {
	ID             string
	Number         int
	Body           string
	DefStart       int
	DefEnd         int
	FirstRefOffset int
}

// FootnoteDoc holds all parsed Markdown footnotes and the trailing EOF definition cluster offset.
type FootnoteDoc struct {
	Entries          []FootnoteEntry
	ByID             map[string]*FootnoteEntry
	TrailingDefStart int // Byte offset of contiguous trailing `[^id]: ...` block at EOF, or -1
}

var (
	reFootnoteDefLine   = regexp.MustCompile(`^\[\^([^\s\]\r\n]+)\]:[ \t]*(.*)$`)
	reFootnoteContLine  = regexp.MustCompile(`^(?:  |\t)(.+)$`)
	reFootnoteRefInline = regexp.MustCompile(`\[\^([^\s\]\r\n]+)\]`)
)

// HasFootnotes reports fast whether source likely contains Markdown footnote syntax outside code.
func HasFootnotes(source string) bool {
	if !strings.Contains(source, "[^") {
		return false
	}
	doc := ParseFootnotes(source)
	return doc != nil && len(doc.Entries) > 0
}

// FindFootnoteRefAtOffset returns the footnote ID when byte offset lies inside an
// inline `[^id]` reference outside code spans/fences and definition blocks.
func FindFootnoteRefAtOffset(source string, offset int) (string, bool) {
	if source == "" || !strings.Contains(source, "[^") {
		return "", false
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	doc := ParseFootnotes(source)
	inDef := func(pos int) bool {
		if doc == nil {
			return false
		}
		for _, e := range doc.Entries {
			if e.DefStart >= 0 && pos >= e.DefStart && pos <= e.DefEnd {
				return true
			}
		}
		return false
	}
	masked := maskCodeRegions(source)
	for _, m := range reFootnoteRefInline.FindAllStringSubmatchIndex(masked, -1) {
		s, e := m[0], m[1]
		if inDef(s) || (e < len(source) && source[e] == ':') {
			continue
		}
		if offset >= s && offset <= e {
			id := strings.TrimSpace(source[m[2]:m[3]])
			if id != "" {
				return id, true
			}
		}
	}
	return "", false
}

type rawFootnoteDef struct {
	id       string
	body     string
	defStart int
	defEnd   int
}

// ParseFootnotes extracts Markdown `[^id]` references and `[^id]: body` definitions
// outside YAML frontmatter, fenced code blocks, and inline code spans.
func ParseFootnotes(source string) *FootnoteDoc {
	out := &FootnoteDoc{
		ByID:             map[string]*FootnoteEntry{},
		TrailingDefStart: -1,
	}
	if source == "" || !strings.Contains(source, "[^") {
		return out
	}
	masked := maskCodeRegions(source)

	var defs []rawFootnoteDef
	defByID := map[string]rawFootnoteDef{}
	lineStart := 0
	for lineStart < len(source) {
		nl := strings.IndexByte(source[lineStart:], '\n')
		var lineEnd, nextStart int
		if nl >= 0 {
			lineEnd = lineStart + nl
			nextStart = lineEnd + 1
		} else {
			lineEnd = len(source)
			nextStart = len(source)
		}
		cleanEnd := lineEnd
		if cleanEnd > lineStart && source[cleanEnd-1] == '\r' {
			cleanEnd--
		}
		maskedLine := masked[lineStart:cleanEnd]
		if m := reFootnoteDefLine.FindStringSubmatchIndex(maskedLine); m != nil {
			id := strings.TrimSpace(source[lineStart+m[2] : lineStart+m[3]])
			firstBody := strings.TrimRight(source[lineStart+m[4]:cleanEnd], " \t")
			bodyLines := []string{firstBody}
			defStart := lineStart
			defEnd := cleanEnd

			scan := nextStart
			for scan < len(source) {
				snl := strings.IndexByte(source[scan:], '\n')
				var sEnd, sNext int
				if snl >= 0 {
					sEnd = scan + snl
					sNext = sEnd + 1
				} else {
					sEnd = len(source)
					sNext = len(source)
				}
				sCleanEnd := sEnd
				if sCleanEnd > scan && source[sCleanEnd-1] == '\r' {
					sCleanEnd--
				}
				mCont := masked[scan:sCleanEnd]
				if cm := reFootnoteContLine.FindStringSubmatchIndex(mCont); cm != nil {
					contTxt := strings.TrimRight(source[scan+cm[2]:sCleanEnd], " \t")
					bodyLines = append(bodyLines, contTxt)
					defEnd = sCleanEnd
					scan = sNext
					continue
				}
				break
			}
			body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
			if id != "" {
				rd := rawFootnoteDef{id: id, body: body, defStart: defStart, defEnd: defEnd}
				defs = append(defs, rd)
				defByID[id] = rd
			}
			lineStart = scan
			continue
		}
		lineStart = nextStart
	}

	// Trailing definition cluster detection: contiguous definitions at EOF with nothing non-blank after them
	if len(defs) > 0 {
		last := defs[len(defs)-1]
		if strings.TrimSpace(masked[last.defEnd:]) == "" {
			i := len(defs) - 1
			for i > 0 && strings.TrimSpace(masked[defs[i-1].defEnd:defs[i].defStart]) == "" {
				i--
			}
			out.TrailingDefStart = defs[i].defStart
		}
	}

	inDefBlock := func(pos int) bool {
		for _, d := range defs {
			if pos >= d.defStart && pos <= d.defEnd {
				return true
			}
		}
		return false
	}

	firstRef := map[string]int{}
	var refOrder []string
	for _, m := range reFootnoteRefInline.FindAllStringSubmatchIndex(masked, -1) {
		s, e := m[0], m[1]
		if inDefBlock(s) || (e < len(source) && source[e] == ':') {
			continue
		}
		id := strings.TrimSpace(source[m[2]:m[3]])
		if id == "" {
			continue
		}
		if _, seen := firstRef[id]; !seen {
			firstRef[id] = s
			refOrder = append(refOrder, id)
		}
	}

	sort.SliceStable(refOrder, func(i, j int) bool {
		return firstRef[refOrder[i]] < firstRef[refOrder[j]]
	})

	added := map[string]bool{}
	num := 1
	for _, id := range refOrder {
		added[id] = true
		rd, hasDef := defByID[id]
		entry := FootnoteEntry{
			ID:             id,
			Number:         num,
			FirstRefOffset: firstRef[id],
			DefStart:       -1,
			DefEnd:         -1,
		}
		if hasDef {
			entry.Body = rd.body
			entry.DefStart = rd.defStart
			entry.DefEnd = rd.defEnd
		}
		out.Entries = append(out.Entries, entry)
		num++
	}
	for _, d := range defs {
		if added[d.id] {
			continue
		}
		added[d.id] = true
		out.Entries = append(out.Entries, FootnoteEntry{
			ID:             d.id,
			Number:         num,
			Body:           d.body,
			DefStart:       d.defStart,
			DefEnd:         d.defEnd,
			FirstRefOffset: -1,
		})
		num++
	}
	for i := range out.Entries {
		e := &out.Entries[i]
		out.ByID[e.ID] = e
	}
	return out
}

// NextFootnoteID picks the smallest unused positive integer string ("1", "2", ...).
func NextFootnoteID(source string) string {
	doc := ParseFootnotes(source)
	for n := 1; ; n++ {
		cand := strconv.Itoa(n)
		if doc == nil || doc.ByID[cand] == nil {
			return cand
		}
	}
}

// InsertFootnoteAtOffset inserts `[^id]` at byte offset inside prose and appends its definition.
func InsertFootnoteAtOffset(source string, offset int, initialBody string) (nextSource, id string) {
	id = NextFootnoteID(source)
	fnDoc := ParseFootnotes(source)
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	for offset > 0 && offset < len(source) && (source[offset]&0xC0) == 0x80 {
		offset--
	}
	if fnDoc != nil && fnDoc.TrailingDefStart >= 0 && offset > fnDoc.TrailingDefStart {
		offset = fnDoc.TrailingDefStart
		for offset > 0 && (source[offset-1] == '\n' || source[offset-1] == '\r') {
			offset--
		}
	}
	withRef := source[:offset] + "[^" + id + "]" + source[offset:]
	return UpsertFootnoteBody(withRef, id, initialBody), id
}

func formatFootnoteDefLine(id, body string) string {
	norm := strings.ReplaceAll(strings.TrimRight(body, " \t\r\n"), "\r\n", "\n")
	norm = strings.TrimLeft(norm, "\r\n")
	if norm == "" {
		return fmt.Sprintf("[^%s]:", id)
	}
	lines := strings.Split(norm, "\n")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[^%s]: %s", id, strings.TrimSpace(lines[0])))
	for _, ln := range lines[1:] {
		b.WriteString("\n  ")
		b.WriteString(strings.TrimRight(ln, " \t"))
	}
	return b.String()
}

// UpsertFootnoteBody updates an existing `[^id]: ...` definition block or appends it at EOF.
func UpsertFootnoteBody(source, id, body string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return source
	}
	formatted := formatFootnoteDefLine(id, body)
	doc := ParseFootnotes(source)
	if doc != nil {
		if e := doc.ByID[id]; e != nil && e.DefStart >= 0 && e.DefEnd >= e.DefStart && e.DefEnd <= len(source) {
			return source[:e.DefStart] + formatted + source[e.DefEnd:]
		}
	}
	base := strings.TrimRight(source, "\r\n")
	if base == "" {
		return formatted + "\n"
	}
	sep := "\n\n"
	if doc != nil && doc.TrailingDefStart >= 0 {
		sep = "\n"
	}
	return base + sep + formatted + "\n"
}

// DeleteFootnote removes both the `[^id]: ...` definition and every inline `[^id]` reference.
func DeleteFootnote(source, id string) string {
	id = strings.TrimSpace(id)
	if id == "" || source == "" {
		return source
	}
	doc := ParseFootnotes(source)
	next := source
	if doc != nil {
		if e := doc.ByID[id]; e != nil && e.DefStart >= 0 && e.DefEnd >= e.DefStart && e.DefEnd <= len(next) {
			rmEnd := e.DefEnd
			if rmEnd < len(next) && next[rmEnd] == '\r' {
				rmEnd++
			}
			if rmEnd < len(next) && next[rmEnd] == '\n' {
				rmEnd++
			}
			next = next[:e.DefStart] + next[rmEnd:]
		}
	}

	masked := maskCodeRegions(next)
	doc2 := ParseFootnotes(next)
	inDef := func(pos int) bool {
		if doc2 == nil {
			return false
		}
		for _, e := range doc2.Entries {
			if e.DefStart >= 0 && pos >= e.DefStart && pos <= e.DefEnd {
				return true
			}
		}
		return false
	}
	var ranges [][]int
	for _, m := range reFootnoteRefInline.FindAllStringSubmatchIndex(masked, -1) {
		s, e := m[0], m[1]
		if inDef(s) || (e < len(next) && next[e] == ':') {
			continue
		}
		if strings.TrimSpace(next[m[2]:m[3]]) == id {
			ranges = append(ranges, []int{s, e})
		}
	}
	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		next = next[:r[0]] + next[r[1]:]
	}
	for strings.Contains(next, "\n\n\n") {
		next = strings.ReplaceAll(next, "\n\n\n", "\n\n")
	}
	trimmed := strings.TrimRight(next, "\r\n")
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}
