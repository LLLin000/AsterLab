package notebook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/modules/runs"
)

// Citation is an immutable text snapshot of one authoritative Run record.
type Citation struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	Snapshot string `json:"snapshot,omitempty"`
	Digest   string `json:"digest,omitempty"`
	Revision uint64 `json:"revision,omitempty"`
	Value    string `json:"value,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

type ResolvedCitation struct {
	Citation Citation
	Status   string
}

type CatalogItem struct {
	Citation Citation
	Label    string
	Detail   string
}

type CatalogGroup struct {
	Key   string
	Label string
	Items []CatalogItem
}

type Catalog struct {
	Project string
	Run     string
	Groups  []CatalogGroup
}

// citationPattern matches Markdown link citations [@label](cite://kind/id?d=...&s=...),
// raw/escaped [[cite:...]] tokens, and legacy [event id] / [batch id] tokens.
var citationPattern = regexp.MustCompile(
	`\[@?([^\]]*)\]\(cite://([^/]+)/([^?\)]+)\?d=([^&\)]*)&s=([^\)]*)\)` +
		`|\\?\[\\?\[cite:([^|\]]+)\|([^|\]]+)\|([^|\]]*)\|([^\]\\]*)\\?\]\\?\]` +
		`|\\?\[(event|batch) ([^\]\\]+)\\?\]`,
)

// NewEventCitation snapshots an event's content and identity-bearing fields.
func NewEventCitation(event runs.RunEvent) Citation {
	c := Citation{Kind: "event", ID: event.ID, Title: event.EventType, Snapshot: event.Content}
	c.Digest = digest(event)
	return c
}

func NewAssetCitation(asset runs.Asset) Citation {
	title := asset.RelativePath
	c := Citation{Kind: "asset", ID: asset.ID, Title: title, Snapshot: title, SHA256: asset.SHA256}
	c.Digest = digest(asset)
	return c
}

func NewReadoutCitation(m runs.Measurement) Citation {
	snapshot := m.Endpoint + ": " + jsonNumber(m.Value)
	if m.Unit != "" {
		snapshot += " " + m.Unit
	}
	c := Citation{Kind: "readout", ID: m.SampleID, Title: m.Endpoint, Snapshot: snapshot, Value: jsonNumber(m.Value)}
	c.Digest = digest(m)
	return c
}

func NewPlateCitation(id, title, snapshot string, payload any) Citation {
	return Citation{
		Kind:     "plate",
		ID:       id,
		Title:    title,
		Snapshot: snapshot,
		Digest:   digest(payload),
	}
}

func NewSummaryCitation(field, sampleID string, recorded int, mean, sd float64, unit string) Citation {
	id := field + ":" + sampleID
	snap := fmt.Sprintf("%s · %s (n=%d, 均值=%.4g", field, sampleID, recorded, mean)
	if recorded > 1 {
		snap += fmt.Sprintf(", SD=%.4g", sd)
	}
	if unit != "" {
		snap += " " + unit
	}
	snap += ")"
	return Citation{
		Kind:     "summary",
		ID:       id,
		Title:    field + " · " + sampleID,
		Snapshot: snap,
		Digest:   digest(struct {
			Field, Sample string
			N             int
			Mean, SD      string
		}{field, sampleID, recorded, fmt.Sprintf("%.6g", mean), fmt.Sprintf("%.6g", sd)}),
	}
}

func NewStepCitation(task runs.RunTask) Citation {
	title := strings.TrimSpace(task.Title)
	if title == "" {
		title = task.Path
	}
	if title == "" {
		title = task.Key
	}
	tp := strings.TrimSpace(task.Timepoint)
	if tp == "" {
		tp = "默认时间点"
	}
	snap := fmt.Sprintf("%s (%s · %s · %s)", title, task.Path, tp, task.Status)
	return Citation{
		Kind:     "step",
		ID:       task.Key,
		Title:    title,
		Snapshot: snap,
		Digest:   digest(task),
	}
}

func jsonNumber(value float64) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func digest(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func displayLabel(c Citation) string {
	label := strings.TrimSpace(c.Title)
	if label == "" || c.Kind == "event" || c.Kind == "summary" || c.Kind == "plate" || c.Kind == "step" {
		if strings.TrimSpace(c.Snapshot) != "" {
			label = strings.TrimSpace(c.Snapshot)
		}
	}
	if label == "" {
		label = c.ID
	}
	r := strings.NewReplacer("[", "(", "]", ")", "\r", " ", "\n", " ")
	label = strings.TrimSpace(r.Replace(label))
	runes := []rune(label)
	if len(runes) > 32 {
		label = string(runes[:31]) + "…"
	}
	return label
}

// SerializeCitation emits a deterministic, Markdown-link citation token that
// richtext editors and Markdown renderers preserve without bracket escaping.
func SerializeCitation(c Citation) string {
	return "[@" + displayLabel(c) + "](cite://" + url.PathEscape(c.Kind) + "/" + url.PathEscape(c.ID) +
		"?d=" + url.QueryEscape(c.Digest) + "&s=" + url.QueryEscape(c.Snapshot) + ")"
}

// ParseCitations reads structured Markdown-link citations, legacy [[cite:...]]
// tokens (including escaped brackets), and legacy [event id]/[batch id] tokens.
func ParseCitations(markdown string) []Citation {
	matches := citationPattern.FindAllStringSubmatch(markdown, -1)
	out := make([]Citation, 0, len(matches))
	for _, match := range matches {
		if match[2] != "" {
			kind, errKind := url.PathUnescape(match[2])
			id, errID := url.PathUnescape(match[3])
			d, errD := url.QueryUnescape(match[4])
			snapshot, errSnapshot := url.QueryUnescape(match[5])
			if errKind != nil || errID != nil || errD != nil || errSnapshot != nil {
				continue
			}
			out = append(out, Citation{Kind: kind, ID: id, Title: strings.TrimSpace(match[1]), Digest: d, Snapshot: snapshot})
			continue
		}
		if match[6] != "" {
			id, errID := url.QueryUnescape(match[7])
			snapshot, errSnapshot := url.QueryUnescape(match[9])
			if errID != nil || errSnapshot != nil {
				continue
			}
			out = append(out, Citation{Kind: match[6], ID: id, Digest: match[8], Snapshot: snapshot})
			continue
		}
		kind := "event"
		if match[10] == "batch" {
			kind = "batch"
		}
		out = append(out, Citation{Kind: kind, ID: strings.TrimSpace(match[11])})
	}
	return out
}

// ResolveCitations compares saved citation snapshots against authoritative Run data.
func ResolveCitations(markdown string, snap runs.RunSnapshot, assets []runs.Asset) []ResolvedCitation {
	return ResolveCitationsWith(markdown, snap, assets, nil, "")
}

// ResolveCitationsWith resolves all Run citations plus optional ResultGrid / frozen AsterML plates.
func ResolveCitationsWith(markdown string, snap runs.RunSnapshot, assets []runs.Asset, grid *resultgrid.Grid, frozenSource string) []ResolvedCitation {
	citations := ParseCitations(markdown)
	catalog := BuildCitationCatalog("", snap, assets, grid, frozenSource, "")
	byKindID := map[string]Citation{}
	for _, grp := range catalog.Groups {
		for _, item := range grp.Items {
			byKindID[item.Citation.Kind+"\x00"+item.Citation.ID] = item.Citation
		}
	}
	out := make([]ResolvedCitation, 0, len(citations))
	for _, c := range citations {
		status := "missing"
		switch c.Kind {
		case "event":
			for _, event := range snap.Events {
				if event.ID == c.ID {
					status = compareCitation(c, NewEventCitation(event))
					break
				}
			}
		case "batch": // Legacy token: resolve the batch to its authoritative event.
			for _, event := range snap.Events {
				if event.BatchID == c.ID {
					c.Kind, c.ID = "event", event.ID
					status = compareCitation(c, NewEventCitation(event))
					break
				}
			}
		case "readout":
			for _, m := range snap.Measurements {
				if m.SampleID == c.ID {
					status = compareCitation(c, NewReadoutCitation(m))
					break
				}
			}
		case "asset":
			for _, asset := range assets {
				if asset.ID != c.ID {
					continue
				}
				if asset.Availability != "available" {
					status = "missing"
				} else if asset.Link.RunID == "" || asset.Link.RunID != snap.RunID {
					status = "stale"
				} else {
					status = compareCitation(c, NewAssetCitation(asset))
				}
				break
			}
		case "step":
			for _, task := range snap.Tasks {
				if task.Key == c.ID {
					status = compareCitation(c, NewStepCitation(task))
					break
				}
			}
		case "summary", "plate":
			if cur, ok := byKindID[c.Kind+"\x00"+c.ID]; ok {
				status = compareCitation(c, cur)
			}
		}
		out = append(out, ResolvedCitation{Citation: c, Status: status})
	}
	return out
}

func compareCitation(saved, current Citation) string {
	if saved.Digest == "" {
		return "current" // Legacy references carry no snapshot.
	}
	if saved.Digest == current.Digest {
		return "current"
	}
	return "stale"
}

// RemoveCitation removes only the zero-based citation token in Markdown.
func RemoveCitation(markdown string, index int) string {
	matches := citationPattern.FindAllStringIndex(markdown, -1)
	if index < 0 || index >= len(matches) {
		return markdown
	}
	match := matches[index]
	return markdown[:match[0]] + markdown[match[1]:]
}

// ExtractAtQuery detects an active trailing @mention query at the end of prose
// (e.g., "@", "@板图", "@OD450"). Returns ("", false) when no trigger is active.
func ExtractAtQuery(markdown string) (string, bool) {
	trimmed := strings.TrimRight(markdown, "\r\n")
	if trimmed == "" {
		return "", false
	}
	lastNL := strings.LastIndexByte(trimmed, '\n')
	line := trimmed[lastNL+1:]
	at := strings.LastIndexByte(line, '@')
	if at < 0 {
		return "", false
	}
	if at > 0 {
		prev := line[at-1]
		if prev != ' ' && prev != '\t' {
			return "", false
		}
	}
	tail := line[at+1:]
	if strings.ContainsAny(tail, " \t[]()") {
		return "", false
	}
	return tail, true
}

// ApplyAtCitation replaces an active trailing @query with the serialized citation,
// or appends it to the note when triggered from the toolbar button.
func ApplyAtCitation(markdown string, c Citation) string {
	token := SerializeCitation(c)
	trailing := ""
	trimmed := strings.TrimRight(markdown, "\r\n")
	if len(trimmed) < len(markdown) {
		trailing = markdown[len(trimmed):]
	}
	if q, ok := ExtractAtQuery(trimmed); ok {
		cut := len(trimmed) - (1 + len(q))
		return trimmed[:cut] + token + " " + trailing
	}
	if strings.TrimSpace(trimmed) != "" {
		return trimmed + " " + token + trailing
	}
	return token + trailing
}

// BuildCitationCatalog projects the hierarchical citable workspace tree:
// Project -> Plates / Statistical Summaries / Summary Statements / Steps / Assets / Readouts.
func BuildCitationCatalog(projectName string, snap runs.RunSnapshot, assets []runs.Asset, grid *resultgrid.Grid, frozenSource string, query string) Catalog {
	proj := strings.TrimSpace(projectName)
	if proj == "" {
		proj = strings.TrimSpace(snap.Title)
	}
	if proj == "" {
		proj = snap.RunID
	}
	q := strings.ToLower(strings.TrimSpace(query))
	matches := func(fields ...string) bool {
		if q == "" {
			return true
		}
		for _, f := range fields {
			if strings.Contains(strings.ToLower(f), q) {
				return true
			}
		}
		return false
	}

	var groups []CatalogGroup

	// 1. 板图 (Plates from frozen AsterML, ResultGrid schema, or unique BatchIDs)
	var plateItems []CatalogItem
	seenPlates := map[string]bool{}
	if strings.TrimSpace(frozenSource) != "" {
		if doc, err := asterdoc.Parse(frozenSource); err == nil {
			for _, m := range doc.Modules {
				if m.Kind != "plate" || m.Plate == nil {
					continue
				}
				p := m.Plate
				wells := 0
				for _, list := range p.Groups {
					wells += len(list)
				}
				fmtSize := p.Format
				if fmtSize == 0 {
					fmtSize = 96
				}
				snapText := fmt.Sprintf("板图 %s (%d孔 · %d组/%d孔位", p.ID, fmtSize, len(p.Groups), wells)
				if p.Assay != "" {
					snapText += " · " + p.Assay
				}
				if p.Timepoint != "" {
					snapText += " · " + p.Timepoint
				}
				snapText += ")"
				cit := NewPlateCitation(p.ID, p.ID, snapText, p)
				seenPlates[p.ID] = true
				if matches("板图", "plate", p.ID, snapText) {
					plateItems = append(plateItems, CatalogItem{Citation: cit, Label: p.ID, Detail: snapText})
				}
			}
		}
	}
	if grid != nil && grid.Schema.ID != "" && !seenPlates[grid.Schema.ID] {
		snapText := fmt.Sprintf("板图 %s (%d行×%d列 · %d格)", grid.Schema.ID, grid.Rows(), grid.Columns(), len(grid.Cells))
		cit := NewPlateCitation(grid.Schema.ID, grid.Schema.ID, snapText, struct {
			ID    string
			Cells []resultgrid.Cell
		}{grid.Schema.ID, grid.Cells})
		seenPlates[grid.Schema.ID] = true
		if matches("板图", "plate", grid.Schema.ID, snapText) {
			plateItems = append(plateItems, CatalogItem{Citation: cit, Label: grid.Schema.ID, Detail: snapText})
		}
	}
	for _, ev := range snap.Events {
		if ev.BatchID == "" || seenPlates[ev.BatchID] {
			continue
		}
		seenPlates[ev.BatchID] = true
		snapText := fmt.Sprintf("板图批次 %s (%s)", ev.BatchID, strings.TrimSpace(ev.Content))
		cit := NewPlateCitation(ev.BatchID, "板图 "+ev.BatchID, snapText, ev.BatchID+":"+ev.Content)
		if matches("板图", "plate", "batch", ev.BatchID, snapText) {
			plateItems = append(plateItems, CatalogItem{Citation: cit, Label: "批次 " + ev.BatchID, Detail: snapText})
		}
	}
	if len(plateItems) > 0 {
		groups = append(groups, CatalogGroup{Key: "plate", Label: "板图", Items: plateItems})
	}

	// 2. 统计结果 (Summary statistics over ResultGrid or Run measurements)
	var summaryItems []CatalogItem
	if grid != nil && len(grid.Cells) > 0 {
		rev := resultgrid.Summarize(grid)
		for _, row := range rev.Rows {
			if row.Recorded == 0 {
				continue
			}
			cit := NewSummaryCitation(row.Field, row.SampleID, row.Recorded, row.Mean, row.SD, "")
			if matches("统计结果", "summary", "均值", row.Field, row.SampleID, cit.Snapshot) {
				summaryItems = append(summaryItems, CatalogItem{Citation: cit, Label: cit.Title, Detail: cit.Snapshot})
			}
		}
	} else if len(snap.Measurements) > 0 {
		type sumKey struct{ field, sample string }
		vals := map[sumKey][]float64{}
		units := map[sumKey]string{}
		var keys []sumKey
		for _, m := range snap.Measurements {
			if strings.TrimSpace(m.SampleID) == "" {
				continue
			}
			k := sumKey{m.Endpoint, m.SampleID}
			if _, ok := vals[k]; !ok {
				keys = append(keys, k)
			}
			vals[k] = append(vals[k], m.Value)
			if units[k] == "" {
				units[k] = m.Unit
			}
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].field != keys[j].field {
				return keys[i].field < keys[j].field
			}
			return keys[i].sample < keys[j].sample
		})
		for _, k := range keys {
			xs := vals[k]
			var sum float64
			for _, x := range xs {
				sum += x
			}
			mean := sum / float64(len(xs))
			var sd float64
			if len(xs) > 1 {
				var sq float64
				for _, x := range xs {
					d := x - mean
					sq += d * d
				}
				sd = math.Sqrt(sq / float64(len(xs)-1))
			}
			cit := NewSummaryCitation(k.field, k.sample, len(xs), mean, sd, units[k])
			if matches("统计结果", "summary", "均值", k.field, k.sample, cit.Snapshot) {
				summaryItems = append(summaryItems, CatalogItem{Citation: cit, Label: cit.Title, Detail: cit.Snapshot})
			}
		}
	}
	if len(summaryItems) > 0 {
		groups = append(groups, CatalogGroup{Key: "summary", Label: "统计结果", Items: summaryItems})
	}

	// 3. 总结语句 / 执行事件 (Summary statements & recorded events, newest first)
	var eventItems []CatalogItem
	for i := len(snap.Events) - 1; i >= 0 && len(eventItems) < 12; i-- {
		ev := snap.Events[i]
		if strings.TrimSpace(ev.ID) == "" || ev.Stage == "notebook" {
			continue
		}
		label := strings.TrimSpace(ev.Content)
		if label == "" {
			label = ev.ID
		}
		cit := NewEventCitation(ev)
		if matches("总结语句", "事件", "statement", "event", ev.ID, label, ev.EventType, ev.StepStatus) {
			eventItems = append(eventItems, CatalogItem{Citation: cit, Label: label, Detail: ev.EventType + " · " + ev.OccurredAt})
		}
	}
	if len(eventItems) > 0 {
		groups = append(groups, CatalogGroup{Key: "event", Label: "总结语句 / 事件", Items: eventItems})
	}

	// 4. 步骤板块 (Frozen Run tasks / protocol steps)
	var stepItems []CatalogItem
	for _, task := range snap.Tasks {
		cit := NewStepCitation(task)
		if matches("步骤板块", "步骤", "step", task.Key, task.Title, task.Path, task.Timepoint, task.Status) {
			stepItems = append(stepItems, CatalogItem{Citation: cit, Label: cit.Title, Detail: cit.Snapshot})
		}
	}
	if len(stepItems) > 0 {
		groups = append(groups, CatalogGroup{Key: "step", Label: "步骤板块", Items: stepItems})
	}

	// 5. 关联文件 (Linked Run assets)
	var assetItems []CatalogItem
	for _, asset := range assets {
		if strings.TrimSpace(asset.ID) == "" {
			continue
		}
		cit := NewAssetCitation(asset)
		if matches("关联文件", "文件", "asset", asset.ID, asset.RelativePath) {
			assetItems = append(assetItems, CatalogItem{Citation: cit, Label: asset.RelativePath, Detail: asset.Availability})
		}
	}
	if len(assetItems) > 0 {
		groups = append(groups, CatalogGroup{Key: "asset", Label: "关联文件", Items: assetItems})
	}

	// 6. 读数明细 (Individual measurements)
	var readoutItems []CatalogItem
	for _, m := range snap.Measurements {
		if strings.TrimSpace(m.SampleID) == "" {
			continue
		}
		cit := NewReadoutCitation(m)
		if matches("读数", "readout", m.SampleID, m.Endpoint, cit.Snapshot) {
			readoutItems = append(readoutItems, CatalogItem{Citation: cit, Label: m.Endpoint + " · " + m.SampleID, Detail: cit.Snapshot})
		}
	}
	if len(readoutItems) > 0 {
		groups = append(groups, CatalogGroup{Key: "readout", Label: "读数明细", Items: readoutItems})
	}

	return Catalog{Project: proj, Run: snap.Title, Groups: groups}
}
