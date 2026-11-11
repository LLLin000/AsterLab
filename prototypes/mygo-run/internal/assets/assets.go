// Package assets defines the Linked Local Asset contract: a local file is
// referenced by a logical root plus a relative path, never by an absolute
// Windows path embedded in AsterML. The authoritative link (run/task/sample)
// lives in the Core record store; this package only computes and validates
// the reference shape and the file's observable state.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MediaType is a coarse classification used for display and open behaviour.
type MediaType string

const (
	MediaImage   MediaType = "image"
	MediaTable   MediaType = "table"
	MediaDocument MediaType = "document"
	MediaBinary  MediaType = "binary"
	MediaUnknown MediaType = "unknown"
)

// Availability states whether the referenced file can be read right now.
type Availability string

const (
	Available   Availability = "available"
	Unavailable Availability = "unavailable"
)

// Link names the experiment object an asset belongs to.
type Link struct {
	RunID    string `json:"runId"`
	TaskKey  string `json:"taskKey"`
	SampleID string `json:"sampleId,omitempty"`
}

// Asset is the durable reference plus the observable state captured when the
// link was created. Encoding stores root_id and relative_path; size, mtime and
// sha256 are evidence that the file has not been swapped silently.
type Asset struct {
	ID           string       `json:"id"`
	RootID       string       `json:"rootId"`
	RelativePath string       `json:"relativePath"`
	MediaType    MediaType    `json:"mediaType"`
	Size         int64        `json:"size"`
	ModTime      string       `json:"modTime"`
	SHA256       string       `json:"sha256,omitempty"`
	Availability Availability `json:"availability"`
	Link         Link         `json:"link"`
	LinkedAt     string       `json:"linkedAt"`
	ActorID      string       `json:"actorId,omitempty"`
}

// Roots maps a logical root ID to a local directory. The directory may move
// between machines; only the root mapping changes, never each asset.
type Roots map[string]string

// Resolve returns the absolute path for an asset via its root mapping.
func (r Roots) Resolve(a Asset) (string, error) {
	base, ok := r[a.RootID]
	if !ok || strings.TrimSpace(base) == "" {
		return "", fmt.Errorf("root %q is not configured", a.RootID)
	}
	rel := strings.TrimSpace(a.RelativePath)
	if rel == "" {
		return "", fmt.Errorf("asset %q has no relative path", a.ID)
	}
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("asset %q relative path escapes its root", a.ID)
	}
	return filepath.Join(base, clean), nil
}

// Classify maps a file extension to a coarse media type.
func Classify(path string) MediaType {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".tif", ".tiff", ".bmp", ".gif", ".webp":
		return MediaImage
	case ".csv", ".tsv", ".xlsx", ".xls", ".txt":
		return MediaTable
	case ".pdf", ".doc", ".docx", ".md":
		return MediaDocument
	case ".bin", ".dat", ".zip", ".raw":
		return MediaBinary
	}
	return MediaUnknown
}

// Describe reads the file's size, modification time and (optionally) hash, and
// reports whether it is currently readable. A missing file is not an error: it
// is an Unavailable asset that keeps its original link.
func Describe(path string, withHash bool) (size int64, modTime string, hash string, availability Availability) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0, "", "", Unavailable
	}
	size = info.Size()
	modTime = info.ModTime().UTC().Format(time.RFC3339)
	availability = Available
	if withHash && size <= 64<<20 {
		if sum, err := hashFile(path); err == nil {
			hash = sum
		}
	}
	return size, modTime, hash, availability
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Verify reports whether a currently available file still matches the recorded
// identity. Large files are compared by size+mtime only; only small files carry
// a full hash. A mismatch means the path was silently repointed.
func Verify(a Asset, path string, withHash bool) (bool, string) {
	size, modTime, hash, availability := Describe(path, withHash)
	if availability == Unavailable {
		return false, "file is unavailable"
	}
	if a.Size > 0 && size != a.Size {
		return false, "file size changed"
	}
	if a.ModTime != "" && modTime != a.ModTime {
		return false, "file modification time changed"
	}
	if withHash && a.SHA256 != "" && hash != "" && hash != a.SHA256 {
		return false, "file content hash changed"
	}
	return true, ""
}

// NewLink builds a validated asset link from a logical root and a path inside it.
// The relative path is derived from the root, so the caller passes a directory
// root and the concrete file; nothing absolute is persisted.
func NewLink(rootID, rootDir, filePath string, link Link, actorID string, withHash bool) (Asset, error) {
	rel, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		return Asset{}, fmt.Errorf("asset is outside root %q: %w", rootID, err)
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return Asset{}, fmt.Errorf("asset %q escapes root %q", filePath, rootID)
	}
	size, modTime, hash, availability := Describe(filePath, withHash)
	if availability == Unavailable {
		return Asset{}, fmt.Errorf("file %q is not readable", filePath)
	}
	rel = filepath.ToSlash(rel)
	return Asset{
		ID:           fmt.Sprintf("asset-%s-%x", slugify(rel), time.Now().UnixNano()),
		RootID:       rootID,
		RelativePath: rel,
		MediaType:    Classify(rel),
		Size:         size,
		ModTime:      modTime,
		SHA256:       hash,
		Availability: availability,
		Link:         link,
		LinkedAt:     time.Now().UTC().Format(time.RFC3339),
		ActorID:      actorID,
	}, nil
}

func slugify(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		}
		if r == '-' || r == '_' {
			// keep separators out of the ID; use hyphen
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "file"
	}
	return out
}
