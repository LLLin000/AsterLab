package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const (
	TrashDirRel     = ".asterlab/trash"
	TrashMetaRel    = ".asterlab/trash_meta.json"
)

// TrashEntry mirrors `.asterlab/trash_meta.json` records for soft-deleted vault items.
type TrashEntry struct {
	TrashName string    `json:"trashName"`
	OrigRel   string    `json:"origRel"`
	DeletedAt time.Time `json:"deletedAt"`
	Size      int64     `json:"size,omitempty"`
}

// MoveToTrash soft-deletes relPath into `.asterlab/trash/` and records its metadata
// in `.asterlab/trash_meta.json`.
func MoveToTrash(vaultRoot, relPath string) (TrashEntry, error) {
	hm := NewHistoryManager(vaultRoot, DefaultSnapshotPolicy())
	normRel, srcAbs, err := hm.resolveVaultFile(relPath)
	if err != nil {
		return TrashEntry{}, err
	}
	info, err := os.Stat(srcAbs)
	if err != nil {
		return TrashEntry{}, err
	}
	trashDir := filepath.Join(hm.VaultRoot, filepath.FromSlash(TrashDirRel))
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return TrashEntry{}, err
	}

	now := time.Now().UTC()
	stamp := now.Format("20060102-150405")
	base := filepath.Base(srcAbs)
	trashName := fmt.Sprintf("%s_%s", stamp, base)
	destAbs := filepath.Join(trashDir, trashName)
	for n := 2; ; n++ {
		if _, err := os.Stat(destAbs); os.IsNotExist(err) {
			break
		}
		trashName = fmt.Sprintf("%s_%d_%s", stamp, n, base)
		destAbs = filepath.Join(trashDir, trashName)
	}
	if err := os.Rename(srcAbs, destAbs); err != nil {
		return TrashEntry{}, err
	}
	item := TrashEntry{
		TrashName: trashName,
		OrigRel:   normRel,
		DeletedAt: now,
		Size:      info.Size(),
	}
	items := loadTrashMeta(hm.VaultRoot)
	items = append(items, item)
	if err := saveTrashMeta(hm.VaultRoot, items); err != nil {
		return item, err
	}
	return item, nil
}

// ListTrash returns all existing soft-deleted items under `.asterlab/trash/`, newest first.
func ListTrash(vaultRoot string) ([]TrashEntry, error) {
	if strings.TrimSpace(vaultRoot) == "" {
		return nil, errors.New("recovery: vaultRoot is required")
	}
	root := filepath.Clean(vaultRoot)
	items := loadTrashMeta(root)
	out := make([]TrashEntry, 0, len(items))
	for _, it := range items {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(TrashDirRel), filepath.Base(it.TrashName))); err == nil {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].DeletedAt.After(out[j].DeletedAt)
	})
	return out, nil
}

// RestoreFromTrash restores trashName back to its original vault-relative path.
// When overwrite is false and the destination already exists, a collision-safe
// `(restored ...)` suffix is appended rather than clobbering existing work.
func RestoreFromTrash(vaultRoot, trashName string, overwrite bool) (string, error) {
	if strings.TrimSpace(vaultRoot) == "" || strings.TrimSpace(trashName) == "" {
		return "", errors.New("recovery: vaultRoot and trashName are required")
	}
	root := filepath.Clean(vaultRoot)
	safeName := filepath.Base(strings.TrimSpace(trashName))
	if safeName == "" || safeName == "." || safeName == ".." {
		return "", errors.New("recovery: invalid trashName")
	}
	trashAbs := filepath.Join(root, filepath.FromSlash(TrashDirRel), safeName)
	if _, err := os.Stat(trashAbs); err != nil {
		return "", err
	}

	items := loadTrashMeta(root)
	var target *TrashEntry
	var remaining []TrashEntry
	for i := range items {
		if items[i].TrashName == safeName {
			cp := items[i]
			target = &cp
		} else {
			remaining = append(remaining, items[i])
		}
	}

	origRel := safeName
	if idx := strings.IndexByte(safeName, '_'); idx >= 0 && idx+1 < len(safeName) {
		origRel = safeName[idx+1:]
	}
	if target != nil && strings.TrimSpace(target.OrigRel) != "" {
		origRel = target.OrigRel
	}

	hm := NewHistoryManager(root, DefaultSnapshotPolicy())
	normRel, destAbs, err := hm.resolveVaultFile(origRel)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
		return "", err
	}

	if _, statErr := os.Stat(destAbs); statErr == nil {
		if overwrite {
			if err := os.RemoveAll(destAbs); err != nil {
				return "", err
			}
		} else {
			ext := filepath.Ext(normRel)
			stem := strings.TrimSuffix(normRel, ext)
			for n := 1; ; n++ {
				candRel := fmt.Sprintf("%s (restored %d)%s", stem, n, ext)
				candAbs := filepath.Join(root, filepath.FromSlash(candRel))
				if _, err := os.Stat(candAbs); os.IsNotExist(err) {
					normRel = candRel
					destAbs = candAbs
					break
				}
			}
		}
	}

	if err := os.Rename(trashAbs, destAbs); err != nil {
		return "", err
	}
	_ = saveTrashMeta(root, remaining)
	return normRel, nil
}

// PurgeTrashItem permanently deletes a single item from `.asterlab/trash/`.
func PurgeTrashItem(vaultRoot, trashName string) error {
	root := filepath.Clean(vaultRoot)
	safeName := filepath.Base(strings.TrimSpace(trashName))
	if safeName == "" || safeName == "." || safeName == ".." {
		return errors.New("recovery: invalid trashName")
	}
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(TrashDirRel), safeName)); err != nil {
		return err
	}
	items := loadTrashMeta(root)
	var remaining []TrashEntry
	for _, it := range items {
		if it.TrashName != safeName {
			remaining = append(remaining, it)
		}
	}
	return saveTrashMeta(root, remaining)
}

// PurgeExpiredTrash permanently removes trash entries older than olderThan.
func PurgeExpiredTrash(vaultRoot string, olderThan time.Duration) (int, error) {
	if olderThan <= 0 {
		return 0, nil
	}
	items, err := ListTrash(vaultRoot)
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	purged := 0
	for _, it := range items {
		if it.DeletedAt.Before(cutoff) {
			if pErr := PurgeTrashItem(vaultRoot, it.TrashName); pErr == nil {
				purged++
			}
		}
	}
	return purged, nil
}

func loadTrashMeta(vaultRoot string) []TrashEntry {
	raw, err := os.ReadFile(filepath.Join(vaultRoot, filepath.FromSlash(TrashMetaRel)))
	if err != nil {
		return nil
	}
	var out []TrashEntry
	_ = json.Unmarshal(raw, &out)
	return out
}

func saveTrashMeta(vaultRoot string, items []TrashEntry) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return asterdoc.WriteFileAtomic(filepath.Join(vaultRoot, filepath.FromSlash(TrashMetaRel)), data)
}
