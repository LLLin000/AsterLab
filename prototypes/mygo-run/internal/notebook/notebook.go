// Package notebook projects a stored Run into a read-only timeline. It copies
// no experimental fact: every entry names the record it came from, keeps the
// recorded and occurred times apart, and states when a source is missing.
package notebook

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"asterlab-mygo-prototype/modules/runs"
)

// Category groups entries for reading. Defined categories do not imply the
// product records them yet: only stored events produce entries.
type Category string

const (
	Execution   Category = "execution"
	Checklist   Category = "checklist"
	Measurement Category = "measurement"
	Observation Category = "observation"
	Deviation   Category = "deviation"
	System      Category = "system"
)

// Field is one displayed detail, already human-readable.
type Field struct{ Label, Value string }

// Reference points at the stored record an entry projects.
type Reference struct{ Kind, ID, Label string }

// TraceState is derived from the entry's references and the stored snapshot.
// It is never stored as an independent fact: any reference that cannot be
// resolved makes the entry invalid instead of silently trusted.
type TraceState string

const (
	// TraceRecorded means every reference resolves to a stored record.
	TraceRecorded TraceState = "recorded"
	// TraceInvalid means a reference names a record this snapshot cannot find.
	TraceInvalid TraceState = "invalid"
	// TraceMissing means the entry has no usable source information at all.
	TraceMissing TraceState = "missing"
)

// Entry is one read-only timeline record. OccurredAt and RecordedAt are RFC
// 3339 strings or empty when the stored record has none; they are never filled
// in from the other one.
type Entry struct {
	ID           string
	RunID        string
	SourceType   string
	SourceID     string
	TaskID       string
	DesignStepID string
	OccurredAt   string
	RecordedAt   string
	Category     Category
	Title        string
	// Note is a short, always-visible statement for records whose stored
	// context matters before expanding (for example a creation audit).
	Note       string
	Details    []Field
	References []Reference
	// Trace is computed by Build from the snapshot, not supplied by callers.
	Trace TraceState
}

// Notebook is the projection of one Run.
type Notebook struct {
	RunID          string
	DesignID       string
	DesignRevision uint64
	Entries        []Entry
}

// Build projects a stored snapshot. It is deterministic for one snapshot and
// performs no writes.
func Build(snap runs.RunSnapshot) Notebook {
	out := Notebook{RunID: snap.RunID, DesignID: snap.DesignID, DesignRevision: snap.DesignRevision}
	stepForTask := make(map[string]string, len(snap.Tasks))
	for _, task := range snap.Tasks {
		stepForTask[task.Key] = task.DesignStepID
	}

	runEntry := Entry{
		ID:         "run:" + snap.RunID,
		RunID:      snap.RunID,
		SourceType: "run",
		SourceID:   snap.RunID,
		Category:   System,
		Title:      "创建 Run",
		Details: []Field{
			{Label: "设计", Value: snap.DesignID},
			{Label: "冻结设计版本", Value: fmt.Sprintf("rev %d", snap.DesignRevision)},
			{Label: "状态", Value: snap.Status},
			{Label: "记录时间", Value: "未记录"},
		},
		References: []Reference{{Kind: "run", ID: snap.RunID, Label: "实验运行"}},
	}
	if audit := snap.CreationAudit; audit != nil {
		actor := audit.ActorID
		if actor == "" {
			actor = "未记录"
		}
		note := "创建时无警告 · 操作者 " + actor
		if len(audit.Warnings) > 0 {
			note = fmt.Sprintf("创建时有 %d 条警告 · 操作者 %s", len(audit.Warnings), actor)
		}
		runEntry.Note = note
		runEntry.Details = append(runEntry.Details,
			Field{Label: "创建时间", Value: valueOrUnrecorded(audit.RecordedAt)},
			Field{Label: "操作者", Value: actor},
			Field{Label: "创建检查", Value: audit.Decision})
		for _, warning := range audit.Warnings {
			runEntry.Details = append(runEntry.Details, Field{Label: "创建警告", Value: warning})
		}
	}
	var events []Entry

	seen := map[string]bool{}
	for _, event := range snap.Events {
		if event.ID != "" && seen[event.ID] {
			continue
		}
		seen[event.ID] = true
		category := categoryFor(event)
		title := titleFor(event, category)
		details := []Field{}
		if event.TaskKey != "" {
			details = append(details, Field{Label: "任务", Value: event.TaskKey})
		}
		if event.StepStatus != "" {
			details = append(details, Field{Label: "步骤状态", Value: event.StepStatus})
		}
		if event.Timepoint != "" {
			details = append(details, Field{Label: "时间点", Value: event.Timepoint})
		}
		references := []Reference{}
		if event.ID != "" {
			references = append(references, Reference{Kind: "execution_event", ID: event.ID, Label: "执行事件"})
		}
		if event.TaskKey != "" {
			references = append(references, Reference{Kind: "run_task", ID: event.TaskKey, Label: "执行任务"})
		}
		// A readout batch is one timeline record: its cells expand in the
		// source detail instead of becoming ninety-six top-level entries.
		if len(event.ResultCells) > 0 {
			category = Measurement
			title = "保存读数批次"
			details = append(details, Field{Label: "单元格", Value: fmt.Sprintf("%d 个", len(event.ResultCells))})
			if event.SchemaID != "" {
				details = append(details, Field{Label: "结果表", Value: fmt.Sprintf("%s · v%d", event.SchemaID, event.SchemaVersion)})
			}
			if event.BatchID != "" {
				details = append(details, Field{Label: "批次", Value: event.BatchID})
				references = append([]Reference{{Kind: "readout_batch", ID: event.BatchID, Label: "读数批次"}}, references...)
			} else {
				details = append(details, Field{Label: "批次", Value: "未记录"})
			}
		}
		if event.Content != "" && event.Stage != "notebook" {
			details = append(details, Field{Label: "内容", Value: event.Content})
		}
		events = append(events, Entry{
			ID:           "event:" + event.ID,
			RunID:        snap.RunID,
			SourceType:   "execution_event",
			SourceID:     event.ID,
			TaskID:       event.TaskKey,
			DesignStepID: stepForTask[event.TaskKey],
			OccurredAt:   event.OccurredAt,
			RecordedAt:   event.RecordedAt,
			Category:     category,
			Title:        title,
			Details:      details,
			References:   references,
		})
	}

	// The run is the root of the timeline; stored events follow by their real
	// times, and records without any stored time stay last by ID.
	sort.SliceStable(events, func(i, j int) bool {
		ti, oki := sortTime(events[i])
		tj, okj := sortTime(events[j])
		switch {
		case oki && okj && !ti.Equal(tj):
			return ti.Before(tj)
		case oki != okj:
			return oki
		}
		return events[i].ID < events[j].ID
	})
	out.Entries = append([]Entry{runEntry}, events...)
	known := knownSources(snap)
	for i := range out.Entries {
		out.Entries[i].Trace = resolveTrace(out.Entries[i], known)
	}
	return out
}

// sourceSet is the set of records this snapshot can actually resolve.
type sourceSet struct {
	runID   string
	events  map[string]bool
	tasks   map[string]bool
	batches map[string]bool
}

func knownSources(snap runs.RunSnapshot) sourceSet {
	known := sourceSet{
		runID:   snap.RunID,
		events:  map[string]bool{},
		tasks:   map[string]bool{},
		batches: map[string]bool{},
	}
	for _, task := range snap.Tasks {
		if task.Key != "" {
			known.tasks[task.Key] = true
		}
	}
	for _, event := range snap.Events {
		if event.ID != "" {
			known.events[event.ID] = true
		}
		if event.BatchID != "" {
			known.batches[event.BatchID] = true
		}
	}
	return known
}

// resolveTrace derives the state from what is stored. An entry with no source
// fields is missing; a reference to a record outside the snapshot is invalid;
// only fully resolvable references are recorded.
func resolveTrace(entry Entry, known sourceSet) TraceState {
	if entry.SourceID == "" {
		return TraceMissing
	}
	for _, ref := range entry.References {
		var ok bool
		switch ref.Kind {
		case "run":
			ok = ref.ID != "" && ref.ID == known.runID
		case "execution_event":
			ok = known.events[ref.ID]
		case "run_task":
			ok = known.tasks[ref.ID]
		case "readout_batch":
			ok = known.batches[ref.ID]
		default:
			ok = false
		}
		if !ok {
			return TraceInvalid
		}
	}
	if len(entry.References) == 0 {
		return TraceMissing
	}
	return TraceRecorded
}

// sortTime prefers the recorded time and falls back to the occurred time, but
// never invents one. ok reports whether either is a real timestamp.
func valueOrUnrecorded(value string) string {
	if strings.TrimSpace(value) == "" {
		return "未记录"
	}
	return value
}

func sortTime(entry Entry) (time.Time, bool) {
	for _, raw := range []string{entry.RecordedAt, entry.OccurredAt} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func categoryFor(event runs.RunEvent) Category {
	switch event.EventType {
	case "measurement":
		return Measurement
	case "observation":
		return Observation
	case "deviation":
		return Deviation
	case "operation", "material_use":
		return Execution
	}
	return Execution
}

func titleFor(event runs.RunEvent, category Category) string {
	if event.Stage == "notebook" {
		return "更新 Notebook"
	}
	if event.EventType == "operation" || event.EventType == "material_use" {
		if event.Content != "" {
			return event.Content
		}
		return "执行操作"
	}
	if event.Content != "" {
		return event.Content
	}
	switch category {
	case Measurement:
		return "记录测量"
	case Observation:
		return "记录观察"
	case Deviation:
		return "记录偏差"
	}
	return "执行记录"
}

// CitationTarget identifies one timeline entry that a citation can jump to.
// It never copies event content: it carries only references into the stored
// snapshot, so a citation is a pointer, never a second copy of a fact.
type CitationTarget struct {
	EntryID    string
	SourceID   string
	SourceKind string
	Label      string
}

// CitationTargetOf resolves a parsed citation to a timeline entry id when the
// entry the citation names exists in this snapshot. The target carries only
// references; it never copies the cited record's content, so a citation stays
// a pointer into the authoritative run instead of a second copy of it.
//
// An event or batch resolves to its own entry. A readout or step resolves to
// the entry of the task it was recorded under, and a file asset to the entry
// of the task the asset is linked to. Anything without a timeline entry is
// unresolved and the surface must say so instead of inventing one.
func CitationTargetOf(c Citation, snap runs.RunSnapshot, assets []runs.Asset) (CitationTarget, bool) {
	entries := Build(snap).Entries
	findEntry := func(id string) (Entry, bool) {
		for _, entry := range entries {
			if entry.ID == id {
				return entry, true
			}
		}
		return Entry{}, false
	}
	findTask := func(taskKey string) (Entry, bool) {
		if taskKey == "" {
			return Entry{}, false
		}
		for _, entry := range entries {
			if entry.TaskID == taskKey && entry.SourceType == "execution_event" {
				return entry, true
			}
		}
		return Entry{}, false
	}
	// A task-linked citation points at the task's own record; when the task
	// has no stored event yet the citation is unresolved rather than guessed.
	assetTarget := func(assetID, label string) (CitationTarget, bool) {
		for _, asset := range assets {
			if asset.ID != assetID || asset.Link.RunID != snap.RunID {
				continue
			}
			entry, ok := findTask(asset.Link.TaskKey)
			if !ok {
				return CitationTarget{}, false
			}
			return CitationTarget{EntryID: entry.ID, SourceID: asset.ID,
				SourceKind: "asset", Label: label}, true
		}
		return CitationTarget{}, false
	}

	switch c.Kind {
	case "event":
		if entry, ok := findEntry("event:" + c.ID); ok {
			return CitationTarget{EntryID: entry.ID, SourceID: c.ID,
				SourceKind: entry.SourceType, Label: entry.Title}, true
		}
	case "batch":
		for _, event := range snap.Events {
			if event.BatchID == c.ID {
				if entry, ok := findEntry("event:" + event.ID); ok {
					return CitationTarget{EntryID: entry.ID, SourceID: event.ID,
						SourceKind: entry.SourceType, Label: entry.Title}, true
				}
			}
		}
	case "readout":
		for _, event := range snap.Events {
			for _, cell := range event.ResultCells {
				if cell.SampleID != c.ID {
					continue
				}
				if entry, ok := findEntry("event:" + event.ID); ok {
					return CitationTarget{EntryID: entry.ID, SourceID: event.ID,
						SourceKind: entry.SourceType, Label: entry.Title}, true
				}
				break
			}
		}
		for _, m := range snap.Measurements {
			if m.SampleID != c.ID {
				continue
			}
			if entry, ok := findTask(m.TaskKey); ok {
				return CitationTarget{EntryID: entry.ID, SourceID: m.SampleID,
					SourceKind: "measurement", Label: entry.Title}, true
			}
			break
		}
	case "step":
		for _, task := range snap.Tasks {
			if task.Key != c.ID {
				continue
			}
			if entry, ok := findTask(task.Key); ok {
				return CitationTarget{EntryID: entry.ID, SourceID: task.Key,
					SourceKind: "run_task", Label: entry.Title}, true
			}
			break
		}
	case "asset":
		return assetTarget(c.ID, c.Title)
	}
	return CitationTarget{}, false
}

// DisplayTime is the short label for an entry's stored time: the recorded
// time when present, else the occurred time, else an explicit "时间未记录".
// It never invents a time, and it is the one place both projections word it.
func (e Entry) DisplayTime() string {
	raw := e.RecordedAt
	if strings.TrimSpace(raw) == "" {
		raw = e.OccurredAt
	}
	if strings.TrimSpace(raw) == "" {
		return "时间未记录"
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.Format("15:04")
	}
	return raw
}
