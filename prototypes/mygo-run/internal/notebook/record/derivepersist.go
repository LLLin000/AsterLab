package record

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"asterlab-mygo-prototype/modules/runs"
)

// derivedChangesWhitelist is the exact key set SaveDesign (design.revise)
// accepts. Anything else in the derived draft fails loudly instead of being
// silently dropped.
var derivedChangesWhitelist = map[string]bool{
	"title": true, "objective": true, "research_question": true,
	"factors": true, "experimental_unit": true, "group_specs": true,
	"timepoints": true, "primary_endpoint": true, "endpoint_method": true,
	"endpoint_unit": true, "sample_size_rationale": true, "randomization": true,
	"blinding": true, "inclusion_criteria": true, "exclusion_criteria": true,
	"analysis_plan": true, "controls": true, "assays": true,
	"sample_allocations": true, "planned_outputs": true,
	"planned_run_count": true, "planned_run_relationship": true,
	"groups": true, "hypothesis": true, "replicate_plan": true,
	"independent_variables": true, "container_plan": true,
	"source_evidence": true, "evidence_conflicts": true,
}

// ParamUpdate is one measured parameter promoted to the next version's
// evidence, with the exact source event for review.
type ParamUpdate struct {
	ScopeID      string          `json:"scope_id"`
	StepPath     *string         `json:"step_path"`
	ParameterKey string          `json:"parameter_key"`
	Label        *string         `json:"label"`
	Unit         *string         `json:"unit"`
	Planned      json.RawMessage `json:"planned"`
	Actual       json.RawMessage `json:"actual"`
	EventID      string          `json:"event_id"`
}

// DurationUpdate is one measured timer promoted to the next version's time
// budget evidence.
type DurationUpdate struct {
	TimerID  string  `json:"timer_id"`
	TaskKey  string  `json:"task_key"`
	Phase    uint32  `json:"phase_index"`
	PlannedS *uint64 `json:"planned_s"`
	ActualS  uint64  `json:"actual_s"`
}

// DerivedEvidence is the provenance Core returns with a derived draft:
// explicit params and sources, before anything is persisted.
type DerivedEvidence struct {
	DesignID             string
	SourceDesignVersion  string
	SourceDesignRevision int64
	SourceRunID          string
	ExpectedRevision     int64
	Title                string
	Params               []ParamUpdate
	Durations            []DurationUpdate
	Note                 string
}

// DecodeDerived decodes a DeriveDesignFromRun response into reviewable
// evidence. The draft itself stays opaque JSON until ChangesForDerived shapes
// it for SaveDesign.
func DecodeDerived(derived runs.DerivedDesign) (DerivedEvidence, error) {
	var doc struct {
		Title       string `json:"title"`
		DerivedFrom *struct {
			DesignVersionID string           `json:"design_version_id"`
			DesignRevision  int64            `json:"design_revision"`
			RunID           string           `json:"run_id"`
			Params          []ParamUpdate    `json:"parameter_updates"`
			Durations       []DurationUpdate `json:"duration_updates"`
			Note            *string          `json:"note"`
		} `json:"derived_from"`
	}
	if err := json.Unmarshal([]byte(derived.DesignJSON), &doc); err != nil {
		return DerivedEvidence{}, fmt.Errorf("派生设计无法解析：%w", err)
	}
	ev := DerivedEvidence{
		DesignID:             derived.DesignID,
		SourceDesignVersion:  derived.SourceDesignVersionID,
		SourceDesignRevision: derived.SourceDesignRevision,
		SourceRunID:          derived.SourceRunID,
		ExpectedRevision:     derived.ExpectedRevision,
		Title:                doc.Title,
		Note:                 derived.Note,
	}
	if doc.DerivedFrom != nil {
		ev.Params = doc.DerivedFrom.Params
		ev.Durations = doc.DerivedFrom.Durations
		if doc.DerivedFrom.Note != nil && ev.Note == "" {
			ev.Note = *doc.DerivedFrom.Note
		}
	}
	return ev, nil
}

// existingEvidence carries the persisted source_evidence of the current
// design revision so derivation appends instead of replacing it.
type existingEvidence struct {
	SourceEvidence []json.RawMessage `json:"source_evidence"`
}

// ChangesForDerived shapes a DeriveDesignFromRun response into SaveDesign
// Changes: the derived draft minus derived_from (which design.revise
// rejects), plus one source_evidence entry carrying the explicit
// params+sources. Existing evidence is preserved. Unknown draft keys fail
// loudly: persisting a partial draft would silently lose design data.
func ChangesForDerived(designData string, derived runs.DerivedDesign, ev DerivedEvidence, derivedAt time.Time) (string, error) {
	var draft map[string]json.RawMessage
	if err := json.Unmarshal([]byte(derived.DesignJSON), &draft); err != nil {
		return "", fmt.Errorf("派生设计无法解析：%w", err)
	}
	delete(draft, "derived_from")
	var unknown []string
	for key := range draft {
		if !derivedChangesWhitelist[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		return "", fmt.Errorf("派生草案含 SaveDesign 不接受的字段（%s），未写入任何内容", strings.Join(unknown, "、"))
	}
	var existing existingEvidence
	if strings.TrimSpace(designData) != "" {
		if err := json.Unmarshal([]byte(designData), &existing); err != nil {
			return "", fmt.Errorf("当前设计无法解析：%w", err)
		}
	}
	entry, err := json.Marshal(map[string]any{
		"id":              fmt.Sprintf("derived-run-%s-r%d", ev.SourceRunID, ev.SourceDesignRevision),
		"source_document": fmt.Sprintf("run %s（设计 %s rev %d）", ev.SourceRunID, ev.DesignID, ev.SourceDesignRevision),
		"source_date":     derivedAt.UTC().Format(time.RFC3339),
		"source_locator":  "DeriveDesignFromRun parameter_updates/duration_updates",
		"claim":           evidenceClaim(ev),
		"state":           "calculated",
	})
	if err != nil {
		return "", err
	}
	merged := append(append([]json.RawMessage{}, existing.SourceEvidence...), entry)
	combined, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	draft["source_evidence"] = combined
	out, err := json.Marshal(draft)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// evidenceClaim renders the explicit params+sources one line per update, so
// the persisted entry names every measured fact the next version leans on.
func evidenceClaim(ev DerivedEvidence) string {
	var lines []string
	for _, param := range ev.Params {
		label := param.ParameterKey
		if param.Label != nil && strings.TrimSpace(*param.Label) != "" {
			label = *param.Label
		}
		lines = append(lines, fmt.Sprintf("参数 %s/%s：计划 %s → 实测 %s（事件 %s）",
			param.ScopeID, label, compactJSON(param.Planned), compactJSON(param.Actual), param.EventID))
	}
	for _, dur := range ev.Durations {
		lines = append(lines, fmt.Sprintf("计时 %s/%s：实测 %ds（事件源计时器）", dur.TaskKey, dur.TimerID, dur.ActualS))
	}
	if len(lines) == 0 {
		return "无实测漂移：草案与冻结版本一致，由研判确认后创建"
	}
	return strings.Join(lines, "；")
}

// compactJSON renders a small JSON value on one line for evidence text.
func compactJSON(raw json.RawMessage) string {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return "—"
	}
	if len(text) > 80 {
		return text[:80] + "…"
	}
	return text
}
