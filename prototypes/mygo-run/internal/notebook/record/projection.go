package record

// Projection is the Go decode of Core's single read-only RecordProjection
// JSON, fetched through RunService.RecordProjection. Field names follow the
// Core serde exactly (snake_case); unknown fields are ignored so a newer
// Core stays readable. Go never rebuilds the nested readings, timeline or
// archive members: surfaces adapt these types for presentation only.
//
// The authoritative shape lives in crates/asterlab-core/src/record_projection.rs
// (EXP-05); this file mirrors it read-only.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/modules/runs"
)

// FetchInput selects the Core projection. View is "blind" or empty
// (unblinded); FOVPerWell is the planned FOV slots per well (0 means Core's
// default of one planned slot); NowUTC is the caller-supplied RFC 3339
// watermark a running timer's elapsed time is measured at.
type FetchInput struct {
	RunID      string
	View       string
	FOVPerWell uint32
	NowUTC     string
}

// Projection is Core's single read-only Record projection for one frozen Run.
type Projection struct {
	SchemaVersion uint32          `json:"schema_version"`
	RunID         string          `json:"run_id"`
	View          string          `json:"view"`
	Design        DesignSummary   `json:"design"`
	Parameters    []ParamCompare  `json:"parameters"`
	Timers        []TimerObserve  `json:"timers"`
	Images        ImageMatrix     `json:"images"`
	Assets        []RecordAsset   `json:"assets"`
	Readings      []Reading       `json:"readings"`
	ReadingIssues []ReadingIssue  `json:"reading_issues"`
	Readouts      []ReadoutRecord `json:"readouts"`
	Timeline      TimelineSummary `json:"timeline"`
	Notes         []NoteView      `json:"notes"`
	NotesLocked   bool            `json:"notes_locked"`
	Archive       ArchiveState    `json:"archive"`
	Issues        []Issue         `json:"issues"`
}

// DesignSummary is the frozen design snapshot; it never follows later edits.
type DesignSummary struct {
	DesignID        string  `json:"design_id"`
	DesignVersionID string  `json:"design_version_id"`
	DesignRevision  int64   `json:"design_revision"`
	Title           string  `json:"title"`
	Objective       string  `json:"objective"`
	Groups          []Group `json:"groups"`
	Assays          []Assay `json:"assays"`
}

// Group is one frozen assay group. Under blinding the id and name are blank.
type Group struct {
	GroupID          *string `json:"group_id"`
	Name             string  `json:"name"`
	Role             *string `json:"role"`
	Condition        *string `json:"condition"`
	IndependentN     *uint32 `json:"independent_n"`
	TechnicalRepeats *uint32 `json:"technical_repeats"`
	Blinded          bool    `json:"blinded"`
}

// Assay is one frozen assay with its planned timepoints.
type Assay struct {
	AssayID           string   `json:"assay_id"`
	Title             string   `json:"title"`
	Endpoint          string   `json:"endpoint"`
	Method            string   `json:"method"`
	Unit              string   `json:"unit"`
	GroupIDs          []string `json:"group_ids"`
	Timepoints        []string `json:"timepoints"`
	TimepointRelation *string  `json:"timepoint_relation"`
	ProtocolVersionID *string  `json:"protocol_version_id"`
}

// ParamCompare is one planned Protocol binding next to the actual Run values
// Core recorded for the same (scope, slot). Status is matches, differs,
// missing_actual or unplanned.
type ParamCompare struct {
	ScopeID  string          `json:"scope_id"`
	StepPath *string         `json:"step_path"`
	Slot     string          `json:"slot"`
	Label    *string         `json:"label"`
	Unit     *string         `json:"unit"`
	Planned  json.RawMessage `json:"planned"`
	Actual   []ActualParam   `json:"actual"`
	Status   string          `json:"status"`
}

// ActualParam is one recorded actual parameter value.
type ActualParam struct {
	Value      json.RawMessage `json:"value"`
	Unit       *string         `json:"unit"`
	EventID    string          `json:"event_id"`
	RecordedAt *string         `json:"recorded_at"`
	Source     *string         `json:"source"`
}

// TimerObserve is one durable timer with the elapsed time measured at the
// projection watermark. ElapsedS is nil for a running timer without one.
type TimerObserve struct {
	TimerID    string  `json:"timer_id"`
	TaskKey    string  `json:"task_key"`
	PhaseIndex uint32  `json:"phase_index"`
	Mode       string  `json:"mode"`
	Status     string  `json:"status"`
	PlannedS   *uint64 `json:"planned_s"`
	ElapsedS   *uint64 `json:"elapsed_s"`
	RemainingS *uint64 `json:"remaining_s"`
	RoundIndex uint32  `json:"round_index"`
	RoundCount *uint32 `json:"round_count"`
}

// ImageSlot is one planned slot; AssetID is nil until an asset fills it.
type ImageSlot struct {
	Timepoint *string `json:"timepoint"`
	FOVIndex  uint32  `json:"fov_index"`
	AssetID   *string `json:"asset_id"`
}

// ImageWellRow is one well row of the completion matrix.
type ImageWellRow struct {
	Coordinate   string      `json:"coordinate"`
	GroupLabel   *string     `json:"group_label"`
	Slots        []ImageSlot `json:"slots"`
	FilledSlots  uint32      `json:"filled_slots"`
	PlannedSlots uint32      `json:"planned_slots"`
}

// ImagePlateRow is one plate of the completion matrix.
type ImagePlateRow struct {
	PlateID      string         `json:"plate_id"`
	Name         string         `json:"name"`
	Timepoints   []*string      `json:"timepoints"`
	Wells        []ImageWellRow `json:"wells"`
	PlannedSlots uint32         `json:"planned_slots"`
	FilledSlots  uint32         `json:"filled_slots"`
}

// ImageMatrix is the planned-slot completion matrix. Slots derive from the
// frozen plate layout and the assay plan times fov_per_well, never from
// already uploaded assets.
type ImageMatrix struct {
	FOVPerWell        uint32          `json:"fov_per_well"`
	Plates            []ImagePlateRow `json:"plates"`
	PlannedSlots      uint32          `json:"planned_slots"`
	FilledSlots       uint32          `json:"filled_slots"`
	MissingSlots      uint32          `json:"missing_slots"`
	UnlocatedAssetIDs []string        `json:"unlocated_asset_ids"`
	UnplannedAssetIDs []string        `json:"unplanned_asset_ids"`
}

// RecordAsset is a located asset with its recorded coordinates.
type RecordAsset struct {
	AssetID      string  `json:"asset_id"`
	TaskKey      string  `json:"task_key"`
	SampleID     *string `json:"sample_id"`
	WellID       *string `json:"well_id"`
	Timepoint    *string `json:"timepoint"`
	FOVIndex     *uint32 `json:"fov_index"`
	CoordinateID *string `json:"coordinate_id"`
}

// Reading is one raw reading; Value is nil for an explicitly missing cell.
type Reading struct {
	RunID           string          `json:"run_id"`
	StepPath        string          `json:"step_path"`
	AssayID         *string         `json:"assay_id"`
	Timepoint       *string         `json:"timepoint"`
	Endpoint        string          `json:"endpoint"`
	Target          json.RawMessage `json:"target"`
	Value           *float64        `json:"value"`
	Unit            *string         `json:"unit"`
	Status          string          `json:"status"`
	SourceEventID   string          `json:"source_event_id"`
	Revision        uint32          `json:"revision"`
	SupersedesEvent *string         `json:"supersedes_event_id"`
	RecordedAt      *string         `json:"recorded_at"`
}

// ReadingIssue is one problem found while projecting readings.
type ReadingIssue struct {
	Code     string  `json:"code"`
	Message  string  `json:"message"`
	EventID  *string `json:"event_id"`
	StepPath *string `json:"step_path"`
}

// ReadoutRecord is one typed readout read back from a stored event.
type ReadoutRecord struct {
	Readout    json.RawMessage `json:"readout"`
	EventID    string          `json:"event_id"`
	StepPath   string          `json:"step_path"`
	AssayID    *string         `json:"assay_id"`
	Timepoint  *string         `json:"timepoint"`
	RecordedAt *string         `json:"recorded_at"`
}

// TimelineEvent is one classified execution event in recorded order.
type TimelineEvent struct {
	EventID        string            `json:"event_id"`
	EventType      *string           `json:"event_type"`
	Contributions  []string          `json:"contributions"`
	Outcome        string            `json:"outcome"`
	Content        *string           `json:"content"`
	StepPath       *string           `json:"step_path"`
	AssayID        *string           `json:"assay_id"`
	PlannedOutput  *string           `json:"planned_output_id"`
	Timepoint      *string           `json:"timepoint"`
	TaskKey        *string           `json:"task_key"`
	OccurrenceID   *string           `json:"occurrence_id"`
	Operator       *string           `json:"operator"`
	OccurredAt     *string           `json:"occurred_at"`
	RecordedAt     *string           `json:"recorded_at"`
	Checks         []json.RawMessage `json:"checks"`
	Measurements   []json.RawMessage `json:"measurements"`
	Materials      []json.RawMessage `json:"materials"`
	Deviation      *string           `json:"deviation"`
	RetryOfEventID *string           `json:"retry_of_event_id"`
	Attempt        uint32            `json:"attempt"`
}

// RetryRecord links a retry to the attempt it replaces.
type RetryRecord struct {
	EventID         string  `json:"event_id"`
	RetryOfEventID  string  `json:"retry_of_event_id"`
	StepPath        *string `json:"step_path"`
	TaskKey         *string `json:"task_key"`
	Attempt         uint32  `json:"attempt"`
	PreviousOutcome string  `json:"previous_outcome"`
	RecordedAt      *string `json:"recorded_at"`
}

// TimelineIssue is one problem found while projecting the timeline.
type TimelineIssue struct {
	Code     string  `json:"code"`
	Message  string  `json:"message"`
	EventID  *string `json:"event_id"`
	StepPath *string `json:"step_path"`
}

// TimelineSummary is the Record audit rendering's counts and entries.
type TimelineSummary struct {
	Events        []TimelineEvent `json:"events"`
	Failures      []TimelineEvent `json:"failures"`
	Retries       []RetryRecord   `json:"retries"`
	Issues        []TimelineIssue `json:"issues"`
	SkippedByRule uint32          `json:"skipped_by_rule"`
	FirstAt       *string         `json:"first_recorded_at"`
	LastAt        *string         `json:"last_recorded_at"`
}

// Reference is one @kind(args) citation resolved against the projection.
type Reference struct {
	Raw     string          `json:"raw"`
	Kind    string          `json:"kind"`
	Args    []string        `json:"args"`
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
	Message *string         `json:"message"`
}

// NoteView is one stored Markdown note with its resolved references.
type NoteView struct {
	NoteID     string      `json:"note_id"`
	Title      *string     `json:"title"`
	Markdown   string      `json:"markdown"`
	UpdatedAt  string      `json:"updated_at_utc"`
	References []Reference `json:"references"`
}

// ArchiveSignature is the sign-off evidence Core persisted. Core stays clock-free:
// the instant is caller-supplied.
type ArchiveSignature struct {
	SignedBy    string  `json:"signed_by"`
	SignedAtUTC string  `json:"signed_at_utc"`
	Statement   *string `json:"statement"`
}

// ManifestEntry is one archived member with its checksum. Kind is asset,
// note or projection; LogicalPath is never absolute.
type ManifestEntry struct {
	Kind        string  `json:"kind"`
	LogicalPath string  `json:"logical_path"`
	SHA256      string  `json:"sha256"`
	Size        *uint64 `json:"size"`
}

// Manifest is the checksum list of a complete archive package.
type Manifest struct {
	SchemaVersion   uint32          `json:"schema_version"`
	RunID           string          `json:"run_id"`
	DesignVersionID string          `json:"design_version_id"`
	GeneratedAtUTC  string          `json:"generated_at_utc"`
	Entries         []ManifestEntry `json:"entries"`
	ManifestSHA256  string          `json:"manifest_sha256"`
}

// ArchiveState is the persisted lifecycle of a Run record: recording,
// summarizing, signed or archived.
type ArchiveState struct {
	Status    string            `json:"status"`
	Signature *ArchiveSignature `json:"signature"`
	Manifest  *Manifest         `json:"manifest"`
}

// Issue is one problem Core found while projecting. Code is stable; Path
// locates the note, asset or slot that caused it.
type Issue struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Path    *string `json:"path"`
}

// Fetch asks Core for the single read-only Record projection and decodes it.
// It performs no derivation: the JSON is Core's exact text.
func Fetch(ctx context.Context, svc runs.RunService, in FetchInput) (Projection, error) {
	raw, err := svc.RecordProjection(ctx, runs.RecordProjectionInput{
		RunID:      in.RunID,
		View:       in.View,
		FOVPerWell: in.FOVPerWell,
		NowUTC:     in.NowUTC,
	})
	if err != nil {
		return Projection{}, err
	}
	var proj Projection
	if err := json.Unmarshal([]byte(raw), &proj); err != nil {
		return Projection{}, fmt.Errorf("record projection 不是合法 JSON：%w", err)
	}
	if proj.SchemaVersion != 1 {
		return Projection{}, fmt.Errorf("record projection 版本 %d 不受支持", proj.SchemaVersion)
	}
	if strings.TrimSpace(in.RunID) != "" && proj.RunID != in.RunID {
		return Projection{}, fmt.Errorf("record projection 属于 %q，不是 %q", proj.RunID, in.RunID)
	}
	return proj, nil
}

// DecodedReadout is a presentation-friendly decode of a stored Core ReadoutRecord.
type DecodedReadout struct {
	SchemaID  string
	Kind      string
	StepPath  string
	Timepoint string
	Category  string
	Number    *float64
	Unit      string
	Columns   []string
	Rows      [][]string
}

// DecodeReadouts extracts table, categorical, and scalar readouts from Core's projection verbatim.
func DecodeReadouts(records []ReadoutRecord) []DecodedReadout {
	out := make([]DecodedReadout, 0, len(records))
	for _, rec := range records {
		if len(rec.Readout) == 0 {
			continue
		}
		var parsed struct {
			SchemaID string  `json:"schema_id"`
			Unit     *string `json:"unit"`
			Missing  *string `json:"missing"`
			Value    struct {
				Kind     string   `json:"kind"`
				Category *string  `json:"category"`
				Number   *float64 `json:"number"`
				Columns  []string `json:"columns"`
				Rows     [][]any  `json:"rows"`
			} `json:"value"`
		}
		if json.Unmarshal(rec.Readout, &parsed) != nil || parsed.Value.Kind == "" {
			continue
		}
		item := DecodedReadout{
			SchemaID: parsed.SchemaID,
			Kind:     parsed.Value.Kind,
			StepPath: rec.StepPath,
			Columns:  parsed.Value.Columns,
		}
		if rec.Timepoint != nil {
			item.Timepoint = *rec.Timepoint
		}
		if parsed.Unit != nil {
			item.Unit = *parsed.Unit
		}
		if parsed.Value.Category != nil {
			item.Category = *parsed.Value.Category
		} else if parsed.Missing != nil && parsed.Value.Kind == "categorical" {
			item.Category = *parsed.Missing
		}
		item.Number = parsed.Value.Number
		if len(parsed.Value.Rows) > 0 {
			item.Rows = make([][]string, 0, len(parsed.Value.Rows))
			for _, row := range parsed.Value.Rows {
				strRow := make([]string, len(row))
				for i, cell := range row {
					switch v := cell.(type) {
					case nil:
						strRow[i] = "—"
					case float64:
						strRow[i] = strconv.FormatFloat(v, 'g', -1, 64)
					default:
						s := strings.TrimSpace(fmt.Sprint(v))
						if s == "" {
							s = "—"
						}
						strRow[i] = s
					}
				}
				item.Rows = append(item.Rows, strRow)
			}
		}
		out = append(out, item)
	}
	return out
}
