package runworkspace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/resultgrid"
)

const draftFlushInterval = time.Second

// GridDraft is the atomic local recovery snapshot for one Run task grid.
type GridDraft struct {
	RunID         string            `json:"runId"`
	TaskKey       string            `json:"taskKey"`
	SchemaID      string            `json:"schemaId"`
	PendingOpID   string            `json:"pendingOpId,omitempty"`
	PendingOpHash string            `json:"pendingOpHash,omitempty"`
	Cells         []resultgrid.Cell `json:"cells"`
	UpdatedAt     string            `json:"updatedAt"`
}

type draftTracker struct {
	restored       bool
	recovered      bool
	lastDigest     string
	lastFlushed    time.Time
	baselineDigest string
}

var gridDraftTrackers = map[string]*draftTracker{}

func draftDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("LABWORKBENCH_DATA_DIR")); dir != "" {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "AsterLab")
	}
	return ""
}

func draftPath(runID, taskKey string) string {
	dir := draftDataDir()
	if dir == "" || strings.TrimSpace(runID) == "" || strings.TrimSpace(taskKey) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(runID + "\x00" + taskKey))
	return filepath.Join(dir, "drafts", hex.EncodeToString(sum[:16])+".json")
}

// SaveGridDraft atomically writes the current unsaved grid cells and pending
// idempotency token to disk so a process crash or kill never loses work.
func SaveGridDraft(runID, taskKey string, grid *resultgrid.Grid) error {
	if grid == nil {
		return nil
	}
	p := draftPath(runID, taskKey)
	if p == "" {
		return nil
	}
	payload := GridDraft{
		RunID:         runID,
		TaskKey:       taskKey,
		SchemaID:      grid.Schema.ID,
		PendingOpID:   grid.PendingOpID,
		PendingOpHash: grid.PendingOpHash,
		Cells:         grid.Cells,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// LoadGridDraft reads an unsubmitted local draft if one exists for (runID, taskKey).
func LoadGridDraft(runID, taskKey string) (*GridDraft, bool) {
	p := draftPath(runID, taskKey)
	if p == "" {
		return nil, false
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var draft GridDraft
	if err := json.Unmarshal(raw, &draft); err != nil || draft.RunID != runID || draft.TaskKey != taskKey {
		return nil, false
	}
	return &draft, true
}

// ClearGridDraft deletes any saved local draft after Core confirms persistence
// or the operator explicitly discards it.
func ClearGridDraft(runID, taskKey string) {
	p := draftPath(runID, taskKey)
	if p != "" {
		_ = os.Remove(p)
	}
	delete(gridDraftTrackers, runID+"\x00"+taskKey)
}

func syncGridDraft(runID, taskKey string, grid *resultgrid.Grid, now time.Time) (recovered bool) {
	if grid == nil || runID == "" || taskKey == "" {
		return false
	}
	k := runID + "\x00" + taskKey
	tr := gridDraftTrackers[k]
	if tr == nil {
		tr = &draftTracker{baselineDigest: grid.CellsDigest()}
		gridDraftTrackers[k] = tr
	}
	if !tr.restored {
		tr.restored = true
		if draft, ok := LoadGridDraft(runID, taskKey); ok && draft.SchemaID == grid.Schema.ID {
			if err := grid.SetCells(draft.Cells); err == nil {
				grid.PendingOpID = draft.PendingOpID
				grid.PendingOpHash = draft.PendingOpHash
				grid.State = resultgrid.StateDraft
				tr.recovered = true
				tr.lastDigest = grid.CellsDigest()
				tr.lastFlushed = now
			}
		}
	}
	digest := grid.CellsDigest()
	switch grid.State {
	case resultgrid.StateSaved:
		if digest == tr.baselineDigest {
			ClearGridDraft(runID, taskKey)
			tr.recovered = false
		}
	case resultgrid.StateDraft, resultgrid.StateError:
		dirty := digest != tr.baselineDigest || grid.PendingOpID != ""
		if dirty && (digest != tr.lastDigest || now.Sub(tr.lastFlushed) >= draftFlushInterval) {
			if SaveGridDraft(runID, taskKey, grid) == nil {
				tr.lastDigest = digest
				tr.lastFlushed = now
			}
		}
	}
	return tr.recovered
}

func discardGridDraft(runID, taskKey string, grid *resultgrid.Grid) {
	p := draftPath(runID, taskKey)
	if p != "" {
		_ = os.Remove(p)
	}
	k := runID + "\x00" + taskKey
	tr := gridDraftTrackers[k]
	if tr != nil {
		tr.recovered = false
		tr.lastDigest = ""
		if grid != nil {
			tr.baselineDigest = grid.CellsDigest()
		}
	}
}
