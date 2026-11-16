// Package record is the single shared read model for the record surfaces:
// Notebook's automatic board, the Run history audit rendering, and the
// protocol archive all project from the same RecordProjection, so the
// renderings can never disagree. It derives only from the stored RunSnapshot
// plus already-linked assets; it performs no writes and invents no
// coordinates. The package stays a leaf (stdlib plus modules/runs only) so
// every surface can import it without a cycle.
package record

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// DerivedParameter is one measured value promoted to a planned default for a
// new Design. Planned holds the frozen value the Run executed against (empty
// when the Run recorded no plan for the key); Measured holds what the Run
// actually recorded. Source names the exact record the value came from, so a
// reviewer can trace every derived default back to its measurement.
type DerivedParameter struct {
	TaskKey  string `json:"taskKey"`
	Key      string `json:"key"`
	Label    string `json:"label"`
	Planned  string `json:"planned"`
	Measured string `json:"measured"`
	Unit     string `json:"unit"`
	Source   string `json:"source"`
}

// DerivedDesign is a new-design draft grown from measured data. It owns no
// persistence: the Design surface decides whether and how to adopt it. It
// carries its provenance (source Run and frozen design revision) so the new
// design never silently inherits numbers.
type DerivedDesign struct {
	Title                string             `json:"title"`
	SourceRunID          string             `json:"sourceRunId"`
	SourceDesignID       string             `json:"sourceDesignId"`
	SourceDesignRevision uint64             `json:"sourceDesignRevision"`
	Parameters           []DerivedParameter `json:"parameters"`
}

// DeriveDesign grows a new-design draft from measured data: every
// actually-recorded parameter becomes a planned default, and every endpoint
// with a recorded value becomes a reference measurement. Planned-only values
// with no measurement are carried as context, marked unmeasured.
func DeriveDesign(proj RecordProjection) DerivedDesign {
	out := DerivedDesign{
		Title:                fmt.Sprintf("源自 Run %s 的实测参数", shortRunID(proj.RunID)),
		SourceRunID:          proj.RunID,
		SourceDesignID:       proj.DesignID,
		SourceDesignRevision: proj.DesignRevision,
	}
	for _, sum := range proj.Summary {
		planned := map[string]Param{}
		for _, p := range sum.Planned {
			planned[p.Key] = p
		}
		for _, actual := range sum.Actual {
			pp := planned[actual.Key]
			value := actual.Value
			if strings.TrimSpace(value) == "" {
				continue
			}
			unit := actual.Unit
			if unit == "" {
				unit = pp.Unit
			}
			out.Parameters = append(out.Parameters, DerivedParameter{
				TaskKey: sum.TaskKey, Key: actual.Key, Label: labelOrKey(actual.Label, actual.Key),
				Planned: pp.Value, Measured: value, Unit: unit,
				Source: fmt.Sprintf("run %s · task %s · actual_parameters",
					shortRunID(proj.RunID), shortTaskKey(sum.TaskKey)),
			})
			delete(planned, actual.Key)
		}
		for _, key := range sortedKeys(planned) {
			pp := planned[key]
			out.Parameters = append(out.Parameters, DerivedParameter{
				TaskKey: sum.TaskKey, Key: pp.Key, Label: labelOrKey(pp.Label, pp.Key),
				Planned: pp.Value, Measured: "", Unit: pp.Unit,
				Source: fmt.Sprintf("run %s · task %s · planned (unmeasured)",
					shortRunID(proj.RunID), shortTaskKey(sum.TaskKey)),
			})
		}
		seen := map[string]bool{}
		for _, m := range sum.Measurements {
			key := m.Endpoint + "\x00" + m.SampleID
			if seen[key] || strings.TrimSpace(m.Value) == "" || m.Value == "未记录" {
				continue
			}
			seen[key] = true
			label := m.Endpoint
			if strings.TrimSpace(m.SampleID) != "" {
				label = m.Endpoint + " · " + m.SampleID
			}
			out.Parameters = append(out.Parameters, DerivedParameter{
				TaskKey: sum.TaskKey, Key: "measure:" + m.Endpoint, Label: label,
				Planned: "", Measured: m.Value, Unit: m.Unit,
				Source: fmt.Sprintf("run %s · task %s · %s",
					shortRunID(proj.RunID), shortTaskKey(sum.TaskKey), m.Source),
			})
		}
	}
	return out
}

// JSON renders the derived draft for handoff to the Design surface.
func (design DerivedDesign) JSON() string {
	data, err := json.MarshalIndent(design, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func shortRunID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func shortTaskKey(key string) string {
	if len(key) > 24 {
		return key[:24] + "…"
	}
	return key
}

func labelOrKey(label, key string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return key
}

func sortedKeys(params map[string]Param) []string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
