// Package recovery provides AsterLab's local File Recovery snapshot history
// (`.asterlab/history/`), full-vault `.zip` backup and verified atomic restore
// with rollback safety, and `.asterlab/trash/` recovery helpers.
package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const (
	HistoryDirRel           = ".asterlab/history"
	DefaultMaxCopiesPerFile = 30
	DefaultRetentionDays    = 30
	DefaultMinInterval      = 2 * time.Minute
)

// SnapshotPolicy configures SHA-256 deduplication and retention pruning for `.asterlab/history/`.
type SnapshotPolicy struct {
	MaxCopiesPerFile int
	RetentionDays    int
	MinInterval      time.Duration
}

// DefaultSnapshotPolicy returns the standard retention policy (30 copies, 30 days, 2m throttle).
func DefaultSnapshotPolicy() SnapshotPolicy {
	return SnapshotPolicy{
		MaxCopiesPerFile: DefaultMaxCopiesPerFile,
		RetentionDays:    DefaultRetentionDays,
		MinInterval:      DefaultMinInterval,
	}
}

// SnapshotEntry describes one point-in-time file recovery snapshot.
type SnapshotEntry struct {
	ID        string    `json:"id"`
	RelPath   string    `json:"relPath"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	Reason    string    `json:"reason,omitempty"`
}

// HistoryManager manages automatic and manual File Recovery snapshots under `.asterlab/history/`.
type HistoryManager struct {
	VaultRoot string
	Policy    SnapshotPolicy
	NowFn     func() time.Time
	seq       atomic.Uint64
}

// NewHistoryManager creates a HistoryManager rooted at vaultRoot.
func NewHistoryManager(vaultRoot string, policy SnapshotPolicy) *HistoryManager {
	if policy.MaxCopiesPerFile <= 0 {
		policy.MaxCopiesPerFile = DefaultMaxCopiesPerFile
	}
	if policy.RetentionDays < 0 {
		policy.RetentionDays = DefaultRetentionDays
	}
	if policy.MinInterval < 0 {
		policy.MinInterval = DefaultMinInterval
	}
	return &HistoryManager{
		VaultRoot: filepath.Clean(vaultRoot),
		Policy:    policy,
		NowFn:     time.Now,
	}
}

// SnapshotFile reads `<vaultRoot>/<relPath>` and stores a SHA-256 deduplicated
// recovery snapshot under `.asterlab/history/`.
// Returns (entry, created, err) where created is false when skipped due to dedup or throttle.
func (h *HistoryManager) SnapshotFile(relPath, reason string, force bool) (SnapshotEntry, bool, error) {
	normRel, absPath, err := h.resolveVaultFile(relPath)
	if err != nil {
		return SnapshotEntry{}, false, err
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return SnapshotEntry{}, false, err
	}
	return h.SnapshotBytes(normRel, data, reason, force)
}

// SnapshotBytes saves data as a SHA-256 deduplicated snapshot for relPath.
// Content blobs are stored once per SHA-256 inside the file's history bucket so
// repeated or reverted states share storage cleanly.
func (h *HistoryManager) SnapshotBytes(relPath string, data []byte, reason string, force bool) (SnapshotEntry, bool, error) {
	normRel, _, err := h.resolveVaultFile(relPath)
	if err != nil {
		return SnapshotEntry{}, false, err
	}
	now := h.nowUTC()
	sumBytes := sha256.Sum256(data)
	sum := hex.EncodeToString(sumBytes[:])

	existing, err := h.ListSnapshots(normRel)
	if err != nil {
		return SnapshotEntry{}, false, err
	}
	if len(existing) > 0 {
		latest := existing[0]
		if latest.SHA256 == sum {
			return latest, false, nil
		}
		if !force && h.Policy.MinInterval > 0 && now.Sub(latest.CreatedAt) < h.Policy.MinInterval {
			return latest, false, nil
		}
	}

	dir := h.snapshotBucketDir(normRel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return SnapshotEntry{}, false, err
	}

	blobPath := filepath.Join(dir, sum+".blob")
	if _, statErr := os.Stat(blobPath); errors.Is(statErr, os.ErrNotExist) {
		if err := asterdoc.WriteFileAtomic(blobPath, data); err != nil {
			return SnapshotEntry{}, false, err
		}
	}

	n := h.seq.Add(1)
	id := fmt.Sprintf("%s-%04d_%s", now.Format("20060102-150405.000"), n%10000, sum[:8])
	entry := SnapshotEntry{
		ID:        id,
		RelPath:   normRel,
		CreatedAt: now,
		Size:      int64(len(data)),
		SHA256:    sum,
		Reason:    strings.TrimSpace(reason),
	}
	metaBytes, err := json.Marshal(entry)
	if err != nil {
		return SnapshotEntry{}, false, err
	}
	if err := asterdoc.WriteFileAtomic(filepath.Join(dir, id+".json"), metaBytes); err != nil {
		return SnapshotEntry{}, false, err
	}

	if _, err := h.Prune(normRel); err != nil {
		return entry, true, err
	}
	return entry, true, nil
}

// ListSnapshots returns all recorded recovery snapshots for relPath, newest first.
func (h *HistoryManager) ListSnapshots(relPath string) ([]SnapshotEntry, error) {
	normRel, _, err := h.resolveVaultFile(relPath)
	if err != nil {
		return nil, err
	}
	dir := h.snapshotBucketDir(normRel)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []SnapshotEntry
	for _, de := range entries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".json") {
			continue
		}
		raw, rErr := os.ReadFile(filepath.Join(dir, de.Name()))
		if rErr != nil {
			continue
		}
		var m SnapshotEntry
		if json.Unmarshal(raw, &m) == nil && m.ID != "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out, nil
}

// ReadSnapshot loads and verifies the SHA-256 integrity of snapshotID for relPath.
func (h *HistoryManager) ReadSnapshot(relPath, snapshotID string) ([]byte, error) {
	normRel, _, err := h.resolveVaultFile(relPath)
	if err != nil {
		return nil, err
	}
	safeID := filepath.Base(strings.TrimSpace(snapshotID))
	if safeID == "" || safeID == "." || safeID == ".." {
		return nil, errors.New("recovery: invalid snapshot id")
	}
	dir := h.snapshotBucketDir(normRel)
	metaBytes, err := os.ReadFile(filepath.Join(dir, safeID+".json"))
	if err != nil {
		return nil, err
	}
	var meta SnapshotEntry
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, err
	}
	blobBytes, err := os.ReadFile(filepath.Join(dir, meta.SHA256+".blob"))
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(blobBytes)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, meta.SHA256) {
		return nil, fmt.Errorf("recovery: snapshot %s integrity check failed: got %s want %s", safeID, got, meta.SHA256)
	}
	return blobBytes, nil
}

// RestoreSnapshot verifies snapshotID, captures a safety `pre-restore` snapshot
// of the current file on disk if present, and atomically restores the snapshot.
func (h *HistoryManager) RestoreSnapshot(relPath, snapshotID string) error {
	normRel, absPath, err := h.resolveVaultFile(relPath)
	if err != nil {
		return err
	}
	data, err := h.ReadSnapshot(normRel, snapshotID)
	if err != nil {
		return err
	}
	if curData, rErr := os.ReadFile(absPath); rErr == nil {
		_, _, _ = h.SnapshotBytes(normRel, curData, "pre-restore", true)
	}
	return asterdoc.WriteFileAtomic(absPath, data)
}

// Prune enforces RetentionDays and MaxCopiesPerFile on relPath's history and
// removes unreferenced content blobs. Returns the number of pruned snapshots.
func (h *HistoryManager) Prune(relPath string) (int, error) {
	normRel, _, err := h.resolveVaultFile(relPath)
	if err != nil {
		return 0, err
	}
	snaps, err := h.ListSnapshots(normRel)
	if err != nil || len(snaps) == 0 {
		return 0, err
	}
	dir := h.snapshotBucketDir(normRel)
	cutoff := time.Time{}
	if h.Policy.RetentionDays > 0 {
		cutoff = h.nowUTC().AddDate(0, 0, -h.Policy.RetentionDays)
	}

	pruned := 0
	keptHashes := map[string]bool{}
	for i, s := range snaps {
		tooOld := !cutoff.IsZero() && s.CreatedAt.Before(cutoff)
		tooMany := h.Policy.MaxCopiesPerFile > 0 && i >= h.Policy.MaxCopiesPerFile
		if tooOld || tooMany {
			_ = os.Remove(filepath.Join(dir, s.ID+".json"))
			pruned++
			continue
		}
		keptHashes[s.SHA256] = true
	}

	if pruned > 0 {
		entries, rErr := os.ReadDir(dir)
		if rErr == nil {
			for _, de := range entries {
				if !de.IsDir() && strings.HasSuffix(de.Name(), ".blob") {
					hash := strings.TrimSuffix(de.Name(), ".blob")
					if !keptHashes[hash] {
						_ = os.Remove(filepath.Join(dir, de.Name()))
					}
				}
			}
		}
	}
	return pruned, nil
}

func (h *HistoryManager) snapshotBucketDir(normRel string) string {
	sum := sha256.Sum256([]byte(normRel))
	return filepath.Join(h.VaultRoot, filepath.FromSlash(HistoryDirRel), hex.EncodeToString(sum[:8]))
}

func (h *HistoryManager) resolveVaultFile(relPath string) (string, string, error) {
	if strings.TrimSpace(h.VaultRoot) == "" {
		return "", "", errors.New("recovery: VaultRoot is required")
	}
	norm := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relPath)))
	norm = strings.TrimPrefix(norm, "./")
	if norm == "" || norm == "." || strings.HasPrefix(norm, "..") || filepath.IsAbs(relPath) {
		return "", "", fmt.Errorf("recovery: invalid relative path %q", relPath)
	}
	abs := filepath.Join(h.VaultRoot, filepath.FromSlash(norm))
	relCheck, err := filepath.Rel(h.VaultRoot, abs)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("recovery: path %q escapes vault root", relPath)
	}
	return norm, abs, nil
}

func (h *HistoryManager) nowUTC() time.Time {
	if h.NowFn != nil {
		return h.NowFn().UTC()
	}
	return time.Now().UTC()
}
