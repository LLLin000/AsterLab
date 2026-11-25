// Package syncengine implements AsterLab's rate-limit-safe Pure-Go WebDAV
// synchronization engine, single-file manifest index, scientific original +
// preview dual-track media policy, conflict-copy preservation, and
// conflict-free pantry.jsonl append-only merge.
package syncengine

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	SystemDirName         = ".asterlab"
	AssetsDirName         = "assets"
	TemplatesDirName      = "templates"
	ManifestRelPath       = ".asterlab/sync-manifest.json"
	LocalSyncStateRelPath = ".asterlab/sync-state.json"
	PreviewsDirRel        = ".asterlab/previews"
	OriginalsDirRel       = "assets/originals"
	PantryLedgerRelPath   = ".asterlab/pantry.jsonl"
)

// CaseCollision describes two or more normalized paths that differ only by ASCII/Unicode case
// and would collide on case-insensitive filesystems (Windows, macOS, iOS).
type CaseCollision struct {
	LowerPath string   `json:"lowerPath"`
	Paths     []string `json:"paths"`
}

var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// NormalizeVaultPath converts a vault-relative path to canonical slash-separated
// UTF-8 NFC form and validates it against Windows illegal characters, reserved
// device names, control characters, and directory traversal segments.
func NormalizeVaultPath(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("empty vault path")
	}
	if strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("vault path %q contains null byte", raw)
	}
	s := strings.ReplaceAll(raw, "\\", "/")
	// Reject Windows drive prefixes or UNC-like prefixes.
	if len(s) >= 2 && s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z')) {
		return "", fmt.Errorf("absolute Windows path not allowed: %q", raw)
	}
	s = strings.Trim(s, "/")
	if s == "" || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("empty vault path")
	}
	s = norm.NFC.String(s)

	parts := strings.Split(s, "/")
	cleaned := make([]string, 0, len(parts))
	for _, seg := range parts {
		if seg == "" || seg == "." || seg == ".." {
			return "", fmt.Errorf("invalid path segment %q in %q", seg, raw)
		}
		if strings.HasSuffix(seg, " ") || strings.HasSuffix(seg, ".") {
			return "", fmt.Errorf("segment %q has trailing space or dot (forbidden on Windows)", seg)
		}
		for _, r := range seg {
			if r < 0x20 || unicode.IsControl(r) {
				return "", fmt.Errorf("segment %q contains control character", seg)
			}
			switch r {
			case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
				return "", fmt.Errorf("segment %q contains Windows illegal character %q", seg, r)
			}
		}
		base := seg
		if dot := strings.IndexByte(seg, '.'); dot >= 0 {
			base = seg[:dot]
		}
		if windowsReservedNames[strings.ToUpper(base)] {
			return "", fmt.Errorf("segment %q uses reserved Windows device name", seg)
		}
		cleaned = append(cleaned, seg)
	}
	return path.Join(cleaned...), nil
}

// CheckCaseCollisions scans normalized vault paths and returns any groups that
// would collide on case-insensitive filesystems (Windows / macOS / iOS).
func CheckCaseCollisions(paths []string) []CaseCollision {
	byLower := make(map[string]map[string]struct{}, len(paths))
	for _, raw := range paths {
		normPath, err := NormalizeVaultPath(raw)
		if err != nil {
			continue
		}
		lower := strings.ToLower(normPath)
		set, ok := byLower[lower]
		if !ok {
			set = map[string]struct{}{}
			byLower[lower] = set
		}
		set[normPath] = struct{}{}
	}
	var collisions []CaseCollision
	for lower, set := range byLower {
		if len(set) > 1 {
			variants := make([]string, 0, len(set))
			for p := range set {
				variants = append(variants, p)
			}
			sortStrings(variants)
			collisions = append(collisions, CaseCollision{
				LowerPath: lower,
				Paths:     variants,
			})
		}
	}
	return collisions
}

// IsSyncIgnored reports whether a normalized vault-relative path is local-only
// or managed out-of-band (such as the sync manifest itself, local snapshots,
// trash, cache, or machine-specific paths).
func IsSyncIgnored(rel string) bool {
	rel = strings.TrimPrefix(filepath.ToSlash(strings.TrimSpace(rel)), "./")
	if rel == "" {
		return true
	}
	lower := strings.ToLower(rel)
	base := path.Base(lower)
	if strings.HasPrefix(base, ".asterlab-") && strings.HasSuffix(base, ".tmp") {
		return true
	}
	if strings.HasPrefix(base, "asterlab-save-") {
		return true
	}
	if strings.HasSuffix(base, ".tmp") || base == ".ds_store" || base == "thumbs.db" {
		return true
	}
	if !strings.HasPrefix(lower, ".asterlab/") && lower != ".asterlab" {
		// Previews and originals inside assets/originals are managed by the sync engine's
		// dual-track media pipeline rather than treated as generic loose files when inside
		// .asterlab/previews or assets/originals.
		if strings.HasPrefix(lower, OriginalsDirRel+"/") {
			return true
		}
		return false
	}
	switch lower {
	case ".asterlab",
		".asterlab/sync-manifest.json",
		".asterlab/sync-state.json",
		".asterlab/local.json",
		".asterlab/workspace.json",
		".asterlab/trash_meta.json",
		".asterlab/index_cache.json",
		".asterlab/migrated_legacy.flag":
		return true
	}
	for _, prefix := range []string{
		".asterlab/history/",
		".asterlab/snapshots/",
		".asterlab/trash/",
		".asterlab/cache/",
		".asterlab/previews/",
	} {
		if strings.HasPrefix(lower, prefix) || lower == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}

// IsAssetPath reports whether rel lies under the vault `assets/` folder.
func IsAssetPath(rel string) bool {
	lower := strings.ToLower(strings.TrimPrefix(filepath.ToSlash(rel), "./"))
	return strings.HasPrefix(lower, AssetsDirName+"/") && !strings.HasPrefix(lower, OriginalsDirRel+"/")
}

// IsAsterMLPath reports whether rel is an `.asterml` document.
func IsAsterMLPath(rel string) bool {
	return strings.HasSuffix(strings.ToLower(rel), ".asterml")
}

// IsConflictCopyPath reports whether rel is already a generated `.sync-conflict-` artifact.
func IsConflictCopyPath(rel string) bool {
	return strings.Contains(strings.ToLower(path.Base(rel)), ".sync-conflict-")
}

func sortStrings(xs []string) {
	for i := 1; i < len(xs); i++ {
		for j := i; j > 0 && xs[j] < xs[j-1]; j-- {
			xs[j], xs[j-1] = xs[j-1], xs[j]
		}
	}
}
