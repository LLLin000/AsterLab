package asterdoc

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// SetFrontmatterMetadata updates title, date (scheduled_date), and status on
// a single-source AsterML file cleanly while preserving the body untouched.
func SetFrontmatterMetadata(source, title, dateYMD, status string) string {
	fm, fmEnd, hasFM := ScanFrontmatter(source)
	body := source
	if hasFM {
		body = source[fmEnd:]
	}
	if strings.TrimSpace(fm.AsterML) == "" {
		fm.AsterML = EnvelopeFormatVersion
	}
	if strings.TrimSpace(title) != "" {
		fm.Title = strings.TrimSpace(title)
	}
	if dateYMD != "" {
		fm.ScheduledDate = NormalizeDateYMD(dateYMD)
	} else if hasFM {
		fm.ScheduledDate = ""
	}
	if strings.TrimSpace(status) != "" {
		fm.Stage = strings.TrimSpace(status)
	}
	// Strip legacy seal so single-source editing remains unlocked.
	fm.Seal = nil

	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return source
	}
	cleanBody := strings.TrimLeft(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := "---\n" + strings.TrimRight(string(fmBytes), "\r\n") + "\n---\n\n" + cleanBody
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	// Also synchronize experiment@1 module fields when present so both match.
	if doc, err := Parse(out); err == nil {
		for _, m := range doc.Modules {
			if m.Kind == "experiment" {
				props := map[string]any{}
				if strings.TrimSpace(title) != "" {
					props["title"] = strings.TrimSpace(title)
				}
				if dateYMD != "" {
					if norm := NormalizeDateYMD(dateYMD); norm != "" {
						props["scheduled_date"] = norm
					}
				} else {
					props["scheduled_date"] = ""
				}
				if strings.TrimSpace(status) != "" {
					switch status {
					case "planned", "in_progress", "completed":
						props["status"] = status
					}
				}
				if updated, uErr := doc.UpdateModule(m.ID, props); uErr == nil {
					out = updated
				}
				break
			}
		}
	}
	return out
}

// SetFrontmatterStartTime updates start_time (HH:MM) in the YAML frontmatter without altering body.
func SetFrontmatterStartTime(source, startTimeHHMM string) string {
	fm, fmEnd, hasFM := ScanFrontmatter(source)
	body := source
	if hasFM {
		body = source[fmEnd:]
	}
	if strings.TrimSpace(fm.AsterML) == "" {
		fm.AsterML = EnvelopeFormatVersion
	}
	fm.StartTime = NormalizeStartTimeHHMM(startTimeHHMM)
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return source
	}
	cleanBody := strings.TrimLeft(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := "---\n" + strings.TrimRight(string(fmBytes), "\r\n") + "\n---\n\n" + cleanBody
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}

// ClearExecutionDataForTemplate creates a clean date-free template copy from
// an experiment `.asterml` file by removing frontmatter date/status and clearing
// inline execution values (done flags, actuals, timer runs, table row readouts).
func ClearExecutionDataForTemplate(source, templateTitle string) (string, error) {
	flat, err := FlattenEnvelopeToSingleSource(source)
	if err != nil {
		flat = source
	}
	doc, err := Parse(flat)
	if err != nil {
		return "", err
	}
	working := doc.Source
	for _, m := range doc.Modules {
		curDoc, pErr := Parse(working)
		if pErr != nil {
			break
		}
		switch m.Kind {
		case "experiment":
			props := map[string]any{"scheduled_date": "", "status": "planned", "conclusion": "", "outcome": ""}
			if strings.TrimSpace(templateTitle) != "" {
				props["title"] = strings.TrimSpace(templateTitle)
			}
			if next, uErr := curDoc.UpdateModule(m.ID, props); uErr == nil {
				working = next
			}
		case "step":
			blocks := mapSlice(m.Data["blocks"])
			changed := false
			for bi, bm := range blocks {
				switch textValue(bm, "kind") {
				case "checklist":
					items := mapSlice(bm["items"])
					for ii, it := range items {
						delete(it, "done")
						delete(it, "note")
						items[ii] = it
					}
					bm["items"] = items
					changed = true
				case "param_table":
					rows := mapSlice(bm["rows"])
					for ri, r := range rows {
						delete(r, "actual")
						delete(r, "note")
						rows[ri] = r
					}
					bm["rows"] = rows
					changed = true
				case "timer":
					delete(bm, "status")
					delete(bm, "started_at")
					delete(bm, "completed_at")
					delete(bm, "elapsed_sec")
					delete(bm, "phase_index")
					delete(bm, "round_index")
					changed = true
				}
				blocks[bi] = bm
			}
			if changed {
				if next, uErr := curDoc.UpdateModule(m.ID, map[string]any{"blocks": blocks}); uErr == nil {
					working = next
				}
			}
		}
	}
	fm, fmEnd, hasFM := ScanFrontmatter(working)
	if hasFM {
		working = working[fmEnd:]
		fm.ScheduledDate = ""
		fm.Stage = ""
		fm.Seal = nil
		if strings.TrimSpace(templateTitle) != "" {
			fm.Title = strings.TrimSpace(templateTitle)
		}
		if fmBytes, err := yaml.Marshal(fm); err == nil {
			working = "---\n" + strings.TrimRight(string(fmBytes), "\r\n") + "\n---\n\n" + strings.TrimLeft(working, "\r\n")
		}
	}
	return working, nil
}

// FlattenEnvelopeToSingleSource folds any legacy ```asterlab-exec``` ledger and
// ````asterlab-record```` markdown section into the single authoritative `.asterml`
// module tree & document body, guaranteeing zero duplicated design copies.
func FlattenEnvelopeToSingleSource(raw string) (string, error) {
	env, err := ParseEnvelope(raw)
	if err != nil {
		return raw, err
	}
	if env.Execution == nil && env.Record == nil && (env.Frontmatter.Seal == nil) {
		return raw, nil
	}
	designSrc := strings.TrimSpace(env.DesignSource)
	if designSrc == "" {
		designSrc = raw
	}
	doc, err := Parse(designSrc)
	if err != nil {
		return raw, err
	}
	working := doc.Source
	if env.Execution != nil {
		for _, m := range doc.Modules {
			if m.Kind != "step" {
				continue
			}
			curDoc, pErr := Parse(working)
			if pErr != nil {
				break
			}
			var stepMod *Module
			for i := range curDoc.Modules {
				if curDoc.Modules[i].ID == m.ID {
					stepMod = &curDoc.Modules[i]
					break
				}
			}
			if stepMod == nil {
				continue
			}
			blocks := mapSlice(stepMod.Data["blocks"])
			changed := false
			for bi, bm := range blocks {
				bid := textValue(bm, "id")
				switch textValue(bm, "kind") {
				case "checklist":
					items := mapSlice(bm["items"])
					for ii, it := range items {
						cid := textValue(it, "id")
						if st := firstNonEmpty(env.Execution.Checks[m.ID+":"+cid], env.Execution.Checks[cid]); st == "checked" || st == "true" {
							it["done"] = true
							items[ii] = it
							changed = true
						}
					}
					if changed {
						bm["items"] = items
					}
				case "param_table":
					rows := mapSlice(bm["rows"])
					for ri, r := range rows {
						k := textValue(r, "key")
						if val, ok := env.Execution.ActualParams[m.ID+":"+bid+":"+k]; ok {
							r["actual"] = val
							rows[ri] = r
							changed = true
						} else if val, ok := env.Execution.ActualParams[k]; ok {
							r["actual"] = val
							rows[ri] = r
							changed = true
						}
					}
					if changed {
						bm["rows"] = rows
					}
				}
				blocks[bi] = bm
			}
			if changed {
				if next, uErr := curDoc.UpdateModule(m.ID, map[string]any{"blocks": blocks}); uErr == nil {
					working = next
				}
			}
		}
	}
	if env.Record != nil && strings.TrimSpace(env.Record.Markdown) != "" {
		working = strings.TrimRight(working, "\r\n") + "\n\n## 结果分析与实验结论\n\n" + strings.TrimSpace(env.Record.Markdown) + "\n"
	}
	fm := env.Frontmatter
	fm.Seal = nil
	if fm.AsterML == "" {
		fm.AsterML = EnvelopeFormatVersion
	}
	if env.Execution != nil && fm.Stage == "" {
		fm.Stage = env.Execution.Status
	}
	fmBytes, mErr := yaml.Marshal(fm)
	if mErr != nil {
		return working, nil
	}
	return "---\n" + strings.TrimRight(string(fmBytes), "\r\n") + "\n---\n\n" + strings.TrimLeft(working, "\r\n"), nil
}

// UpdateStepBlockInSource merges props into a single inline block inside stepID.
func UpdateStepBlockInSource(source, stepID, blockID string, props map[string]any) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind != "step" || m.ID != stepID {
			continue
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(m.Body), &data); err != nil {
			return "", err
		}
		seq := rawAnySlice(data["blocks"])
		found := false
		for bi := range seq {
			bm, _ := seq[bi].(map[string]any)
			if textValue(bm, "id") == blockID {
				for k, pv := range props {
					if k != "id" && k != "kind" && k != "schema" {
						if pv == nil {
							delete(bm, k)
						} else {
							bm[k] = pv
						}
					}
				}
				seq[bi] = bm
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("block %q not found in step %q", blockID, stepID)
		}
		data["blocks"] = seq
		raw, err := yaml.Marshal(data)
		if err != nil {
			return "", err
		}
		next := doc.Source[:m.Start] + string(raw) + doc.Source[m.End:]
		if _, err := Parse(next); err != nil {
			return "", err
		}
		return next, nil
	}
	return "", fmt.Errorf("step %q not found", stepID)
}

// InsertStepBlockInSource inserts a block into stepID.blocks[] at index (clamped).
func InsertStepBlockInSource(source, stepID string, index int, block map[string]any) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	flat, _ := flattenModules(doc.Modules)
	used := make(map[string]bool, len(flat))
	for _, fm := range flat {
		used[fm.ID] = true
	}
	blkCopy := make(map[string]any, len(block))
	for k, v := range block {
		blkCopy[k] = v
	}
	if id, _ := blkCopy["id"].(string); strings.TrimSpace(id) == "" || used[id] {
		kind := textValue(blkCopy, "kind")
		if kind == "" {
			kind = "blk"
		}
		base := "blk-" + sanitizeIDFrag(kind)
		cand := base
		for n := 2; used[cand]; n++ {
			cand = fmt.Sprintf("%s-%d", base, n)
		}
		blkCopy["id"] = cand
	}
	if blkCopy["schema"] == nil {
		blkCopy["schema"] = 1
	}
	for _, m := range doc.Modules {
		if m.Kind != "step" || m.ID != stepID {
			continue
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(m.Body), &data); err != nil {
			return "", err
		}
		seq := rawAnySlice(data["blocks"])
		if index < 0 || index > len(seq) {
			index = len(seq)
		}
		nextSeq := make([]any, 0, len(seq)+1)
		nextSeq = append(nextSeq, seq[:index]...)
		nextSeq = append(nextSeq, blkCopy)
		nextSeq = append(nextSeq, seq[index:]...)
		data["blocks"] = nextSeq
		raw, err := yaml.Marshal(data)
		if err != nil {
			return "", err
		}
		next := doc.Source[:m.Start] + string(raw) + doc.Source[m.End:]
		if _, err := Parse(next); err != nil {
			return "", err
		}
		return next, nil
	}
	return "", fmt.Errorf("step %q not found", stepID)
}

// RemoveStepBlockInSource removes blockID from stepID.blocks[].
func RemoveStepBlockInSource(source, stepID, blockID string) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind != "step" || m.ID != stepID {
			continue
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(m.Body), &data); err != nil {
			return "", err
		}
		seq := rawAnySlice(data["blocks"])
		nextSeq := make([]any, 0, len(seq))
		for _, rawBlk := range seq {
			bm, _ := rawBlk.(map[string]any)
			if textValue(bm, "id") == blockID {
				continue
			}
			nextSeq = append(nextSeq, rawBlk)
		}
		data["blocks"] = nextSeq
		raw, err := yaml.Marshal(data)
		if err != nil {
			return "", err
		}
		next := doc.Source[:m.Start] + string(raw) + doc.Source[m.End:]
		if _, err := Parse(next); err != nil {
			return "", err
		}
		return next, nil
	}
	return "", fmt.Errorf("step %q not found", stepID)
}

// MoveStepBlockInSource moves blockID up or down within stepID.blocks[].
func MoveStepBlockInSource(source, stepID, blockID, direction string) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind != "step" || m.ID != stepID {
			continue
		}
		var data map[string]any
		if err := yaml.Unmarshal([]byte(m.Body), &data); err != nil {
			return "", err
		}
		seq := rawAnySlice(data["blocks"])
		idx := -1
		for i, rawBlk := range seq {
			bm, _ := rawBlk.(map[string]any)
			if textValue(bm, "id") == blockID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return "", fmt.Errorf("block %q not found in step %q", blockID, stepID)
		}
		target := idx + 1
		if strings.EqualFold(direction, "up") {
			target = idx - 1
		}
		if target < 0 || target >= len(seq) {
			return source, nil
		}
		seq[idx], seq[target] = seq[target], seq[idx]
		data["blocks"] = seq
		raw, err := yaml.Marshal(data)
		if err != nil {
			return "", err
		}
		next := doc.Source[:m.Start] + string(raw) + doc.Source[m.End:]
		if _, err := Parse(next); err != nil {
			return "", err
		}
		return next, nil
	}
	return "", fmt.Errorf("step %q not found", stepID)
}

// DuplicateStepBlockInSource clones blockID immediately below itself with a fresh ID.
func DuplicateStepBlockInSource(source, stepID, blockID string) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind != "step" || m.ID != stepID {
			continue
		}
		seq := rawAnySlice(m.Data["blocks"])
		for idx, rawBlk := range seq {
			bm, _ := rawBlk.(map[string]any)
			if textValue(bm, "id") == blockID {
				clone := cloneAnyMap(bm)
				delete(clone, "id")
				return InsertStepBlockInSource(source, stepID, idx+1, clone)
			}
		}
	}
	return "", fmt.Errorf("block %q not found in step %q", blockID, stepID)
}

// AddStepInSource appends a new executable step module and wires workflow.steps if present.
func AddStepInSource(source, title string) (nextSource, stepID string, err error) {
	doc, err := Parse(source)
	if err != nil {
		return "", "", err
	}
	used := map[string]bool{}
	stepCount := 0
	var wf *Module
	for i := range doc.Modules {
		used[doc.Modules[i].ID] = true
		if doc.Modules[i].Kind == "step" {
			stepCount++
		}
		if doc.Modules[i].Kind == "workflow" && wf == nil {
			wf = &doc.Modules[i]
		}
	}
	n := stepCount + 1
	stepID = fmt.Sprintf("step-%02d", n)
	for used[stepID] {
		n++
		stepID = fmt.Sprintf("step-%02d", n)
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("%02d 新实验步骤", stepCount+1)
	}
	mod := map[string]any{
		"kind":      "step",
		"id":        stepID,
		"schema":    1,
		"execution": "executable",
		"title":     strings.TrimSpace(title),
		"blocks":    []any{},
	}
	raw, err := yaml.Marshal(mod)
	if err != nil {
		return "", "", err
	}
	fence := "\n```asterlab\n" + string(raw) + "```\n"
	insertPos := len(doc.Source)
	if wf != nil {
		if idx := strings.LastIndex(doc.Source[:wf.Start], "```asterlab"); idx >= 0 {
			insertPos = idx
		}
	}
	nextSrc := doc.Source[:insertPos] + fence + doc.Source[insertPos:]
	nextDoc, err := Parse(nextSrc)
	if err != nil {
		return "", "", err
	}
	if wf != nil {
		for _, m := range nextDoc.Modules {
			if m.ID == wf.ID {
				steps := append([]string{}, stringList(m.Data, "steps")...)
				steps = append(steps, stepID)
				if updated, uErr := nextDoc.UpdateModule(wf.ID, map[string]any{"steps": steps}); uErr == nil {
					nextSrc = updated
				}
				break
			}
		}
	}
	return nextSrc, stepID, nil
}

// MoveStepInSource moves stepID by delta (-1 up, +1 down) in workflow/document order.
func MoveStepInSource(source, stepID string, delta int) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind == "workflow" {
			steps := append([]string{}, stringList(m.Data, "steps")...)
			idx := -1
			for i, sid := range steps {
				if sid == stepID {
					idx = i
					break
				}
			}
			if idx < 0 {
				continue
			}
			target := idx + delta
			if target < 0 || target >= len(steps) {
				return source, nil
			}
			steps[idx], steps[target] = steps[target], steps[idx]
			return doc.UpdateModule(m.ID, map[string]any{"steps": steps})
		}
	}
	return source, nil
}

// RemoveStepInSource removes a step fence and detaches it from workflow.steps.
func RemoveStepInSource(source, stepID string) (string, error) {
	doc, err := Parse(source)
	if err != nil {
		return "", err
	}
	working := doc.Source
	for _, m := range doc.Modules {
		if m.Kind == "workflow" {
			var filtered []string
			for _, sid := range stringList(m.Data, "steps") {
				if sid != stepID {
					filtered = append(filtered, sid)
				}
			}
			if len(filtered) > 0 {
				if next, uErr := doc.UpdateModule(m.ID, map[string]any{"steps": filtered}); uErr == nil {
					working = next
				}
			}
		}
	}
	doc, err = Parse(working)
	if err != nil {
		return "", err
	}
	for _, m := range doc.Modules {
		if m.Kind == "step" && m.ID == stepID {
			openIdx := strings.LastIndex(doc.Source[:m.Start], "```asterlab")
			closeIdx := strings.Index(doc.Source[m.End:], "```")
			if openIdx >= 0 && closeIdx >= 0 {
				endPos := m.End + closeIdx + 3
				if endPos < len(doc.Source) && doc.Source[endPos] == '\n' {
					endPos++
				}
				next := doc.Source[:openIdx] + doc.Source[endPos:]
				if _, pErr := Parse(next); pErr == nil {
					return next, nil
				}
			}
		}
	}
	return working, nil
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	raw, err := yaml.Marshal(src)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := yaml.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

// EnsureUniqueBlockID generates a unique block ID inside doc for kind.
func EnsureUniqueBlockID(doc *Document, kind string) string {
	used := map[string]bool{}
	if doc != nil {
		flat, _ := flattenModules(doc.Modules)
		for _, m := range flat {
			used[m.ID] = true
		}
	}
	base := "blk-" + sanitizeIDFrag(kind)
	if base == "blk-" {
		base = "blk-item"
	}
	cand := base
	for n := 2; used[cand]; n++ {
		cand = fmt.Sprintf("%s-%d", base, n)
	}
	_ = time.Now()
	return cand
}
