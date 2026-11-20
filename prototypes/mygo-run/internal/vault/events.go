package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

// EventKind identifies a Vault filesystem lifecycle event.
type EventKind string

const (
	EventCreate EventKind = "create"
	EventModify EventKind = "modify"
	EventRename EventKind = "rename"
	EventDelete EventKind = "delete"
)

// VaultEvent describes a change to a tracked file or folder inside the Vault.
type VaultEvent struct {
	Kind    EventKind
	RelPath string
	OldPath string
	At      time.Time
}

// EventBus implements an Obsidian-style publish/subscribe bus for Vault changes.
type EventBus struct {
	mu   sync.RWMutex
	seq  int
	subs map[int]func(VaultEvent)
}

// Subscribe registers a listener and returns an unsubscribe closure.
func (b *EventBus) Subscribe(fn func(VaultEvent)) func() {
	if b == nil || fn == nil {
		return func() {}
	}
	b.mu.Lock()
	b.seq++
	id := b.seq
	if b.subs == nil {
		b.subs = map[int]func(VaultEvent){}
	}
	b.subs[id] = fn
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
	}
}

// Emit broadcasts ev synchronously to registered subscribers.
func (b *EventBus) Emit(ev VaultEvent) {
	if b == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now()
	}
	b.mu.RLock()
	fns := make([]func(VaultEvent), 0, len(b.subs))
	for _, fn := range b.subs {
		fns = append(fns, fn)
	}
	b.mu.RUnlock()
	for _, fn := range fns {
		fn(ev)
	}
}

// SnapshotMeta records one File Recovery snapshot inside `.asterlab/snapshots/`.
type SnapshotMeta struct {
	ID        string    `json:"id"`
	RelPath   string    `json:"relPath"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
}

const (
	SnapshotsDirName      = "snapshots"
	maxSnapshotsPerFile   = 20
	defaultSnapshotMinGap = 2 * time.Minute
)

// RecordSnapshot saves a deduplicated point-in-time recovery snapshot of relPath
// inside `.asterlab/snapshots/`, throttling repeated writes unless force is true.
func (v *Vault) RecordSnapshot(relPath, content string, force bool) error {
	if v == nil || strings.TrimSpace(relPath) == "" || strings.TrimSpace(content) == "" {
		return nil
	}
	sumBytes := sha256.Sum256([]byte(content))
	sum := hex.EncodeToString(sumBytes[:])
	dir := v.snapshotDirFor(relPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	existing := v.ListSnapshots(relPath)
	if len(existing) > 0 {
		latest := existing[0]
		if latest.SHA256 == sum {
			return nil
		}
		if !force && time.Since(latest.CreatedAt) < defaultSnapshotMinGap {
			return nil
		}
	}
	id := fmt.Sprintf("%s_%s", time.Now().Format("20060102-150405"), sum[:8])
	snapPath := filepath.Join(dir, id+".asterml")
	if err := asterdoc.WriteFileAtomic(snapPath, []byte(content)); err != nil {
		return err
	}
	meta := SnapshotMeta{
		ID:        id,
		RelPath:   relPath,
		CreatedAt: time.Now(),
		Size:      int64(len(content)),
		SHA256:    sum,
	}
	metaBytes, _ := json.Marshal(meta)
	_ = asterdoc.WriteFileAtomic(filepath.Join(dir, id+".json"), metaBytes)

	// Prune old snapshots beyond retention limit.
	updated := v.ListSnapshots(relPath)
	for i := maxSnapshotsPerFile; i < len(updated); i++ {
		_ = os.Remove(filepath.Join(dir, updated[i].ID+".asterml"))
		_ = os.Remove(filepath.Join(dir, updated[i].ID+".json"))
	}
	return nil
}

// ListSnapshots returns all recovery snapshots for relPath, newest first.
func (v *Vault) ListSnapshots(relPath string) []SnapshotMeta {
	if v == nil || strings.TrimSpace(relPath) == "" {
		return nil
	}
	dir := v.snapshotDirFor(relPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []SnapshotMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m SnapshotMeta
		if json.Unmarshal(raw, &m) == nil && m.ID != "" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// ReadSnapshot loads the saved source of snapshotID for relPath.
func (v *Vault) ReadSnapshot(relPath, snapshotID string) (string, error) {
	if v == nil || relPath == "" || snapshotID == "" {
		return "", os.ErrInvalid
	}
	p := filepath.Join(v.snapshotDirFor(relPath), filepath.Base(snapshotID)+".asterml")
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (v *Vault) snapshotDirFor(relPath string) string {
	h := sha256.Sum256([]byte(filepath.ToSlash(strings.TrimSpace(relPath))))
	return v.systemPath(SnapshotsDirName, hex.EncodeToString(h[:8]))
}

var wikilinkRe = regexp.MustCompile(`\[\[([^\]|#]+)(#[^\]|]+)?(\|[^\]]+)?\]\]`)

// ExtractWikiLinks returns normalized target note names/paths referenced via [[...]].
func ExtractWikiLinks(source string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(source, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		target := strings.TrimSpace(m[1])
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		out = append(out, target)
	}
	return out
}

// RewriteWikiAndRepeatLinks updates cross-file references (`[[old]]` and `repeat_of`)
// across all vault `.asterml` files whenever a file is renamed or moved.
func (v *Vault) RewriteWikiAndRepeatLinks(oldRel, newRel string) (int, error) {
	if v == nil || oldRel == "" || newRel == "" || oldRel == newRel {
		return 0, nil
	}
	oldBase := strings.TrimSuffix(filepath.Base(oldRel), filepath.Ext(oldRel))
	newBase := strings.TrimSuffix(filepath.Base(newRel), filepath.Ext(newRel))
	oldRelNoExt := strings.TrimSuffix(filepath.ToSlash(oldRel), ".asterml")
	newRelNoExt := strings.TrimSuffix(filepath.ToSlash(newRel), ".asterml")

	_, files, err := v.ScanTree()
	if err != nil {
		return 0, err
	}
	updatedCount := 0
	for _, f := range files {
		if f.IsAsset {
			continue
		}
		rawBytes, rErr := os.ReadFile(f.AbsPath)
		if rErr != nil {
			continue
		}
		orig := string(rawBytes)
		next := wikilinkRe.ReplaceAllStringFunc(orig, func(full string) string {
			sub := wikilinkRe.FindStringSubmatch(full)
			if len(sub) < 2 {
				return full
			}
			tgt := strings.TrimSpace(sub[1])
			tgtNorm := strings.TrimSuffix(filepath.ToSlash(tgt), ".asterml")
			var replaced string
			switch {
			case tgtNorm == oldRelNoExt || tgt == oldRel:
				replaced = newRelNoExt
			case oldBase != newBase && strings.EqualFold(tgtNorm, oldBase):
				replaced = newBase
			default:
				return full
			}
			return "[[" + replaced + sub[2] + sub[3] + "]]"
		})
		if fm, _, hasFM := asterdoc.ScanFrontmatter(next); hasFM && fm.RepeatOf != "" {
			if fm.RepeatOf == oldRel || fm.RepeatOf == oldRelNoExt || (oldBase != newBase && fm.RepeatOf == oldBase) {
				next = strings.Replace(next, "repeat_of: "+fm.RepeatOf, "repeat_of: "+newRelNoExt, 1)
			}
		}
		if next != orig {
			if asterdoc.WriteFileAtomic(f.AbsPath, []byte(next)) == nil {
				updatedCount++
				if v.Events != nil {
					v.Events.Emit(VaultEvent{Kind: EventModify, RelPath: f.RelPath})
				}
			}
		}
	}
	return updatedCount, nil
}
