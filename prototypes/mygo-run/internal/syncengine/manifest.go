package syncengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

const ManifestSchemaVersion = 1

// ManifestEntry records the authoritative content identity and media sidecar
// metadata for a single synced vault path inside `.asterlab/sync-manifest.json`.
// Content equality uses SHA256 and monotonic logical Revision — never ETag or mtime.
type ManifestEntry struct {
	RelPath           string `json:"relPath"`
	SHA256            string `json:"sha256"`
	Size              int64  `json:"size"`
	Revision          int64  `json:"revision"`
	ModTimeUTC        string `json:"modTimeUtc,omitempty"`
	Deleted           bool   `json:"deleted,omitempty"`
	DeviceID          string `json:"deviceId,omitempty"`
	OriginalAssetPath string `json:"originalAssetPath,omitempty"`
	PreviewPath       string `json:"previewPath,omitempty"`
	PreviewSHA256     string `json:"previewSha256,omitempty"`
	PreviewSize       int64  `json:"previewSize,omitempty"`
}

// SyncManifest is the single-file remote index stored at `.asterlab/sync-manifest.json`.
// Reading this single file replaces recursive WebDAV PROPFIND directory crawls.
type SyncManifest struct {
	SchemaVersion   int                      `json:"schemaVersion"`
	VaultRevision   int64                    `json:"vaultRevision"`
	UpdatedAt       string                   `json:"updatedAt"`
	UpdatedByDevice string                   `json:"updatedByDevice,omitempty"`
	Entries         map[string]ManifestEntry `json:"entries"`
}

// LocalFileState tracks the last synced baseline and local cache state per vault path.
type LocalFileState struct {
	RelPath                string `json:"relPath"`
	BaseSHA256             string `json:"baseSha256"`
	BaseRevision           int64  `json:"baseRevision"`
	NeedsOriginalDownload  bool   `json:"needsOriginalDownload,omitempty"`
	PreviewCached          bool   `json:"previewCached,omitempty"`
	OriginalCached         bool   `json:"originalCached,omitempty"`
	RemoteOriginalSize     int64  `json:"remoteOriginalSize,omitempty"`
	OriginalAssetPath      string `json:"originalAssetPath,omitempty"`
	PreviewPath            string `json:"previewPath,omitempty"`
	PreviewSHA256          string `json:"previewSha256,omitempty"`
	LastConflictRemoteSHA  string `json:"lastConflictRemoteSha,omitempty"`
}

// LocalSyncState persists per-device sync state under `.asterlab/sync-state.json`.
type LocalSyncState struct {
	SchemaVersion      int                       `json:"schemaVersion"`
	DeviceID           string                    `json:"deviceId"`
	LastSyncedVaultRev int64                     `json:"lastSyncedVaultRev"`
	LastSyncAt         string                    `json:"lastSyncAt,omitempty"`
	Files              map[string]LocalFileState `json:"files"`
}

// NewSyncManifest constructs an empty versioned SyncManifest.
func NewSyncManifest(deviceID string) *SyncManifest {
	return &SyncManifest{
		SchemaVersion:   ManifestSchemaVersion,
		VaultRevision:   0,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
		UpdatedByDevice: deviceID,
		Entries:         map[string]ManifestEntry{},
	}
}

// ParseManifest unmarshals a `.asterlab/sync-manifest.json` payload safely.
func ParseManifest(data []byte) (*SyncManifest, error) {
	var m SyncManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = ManifestSchemaVersion
	}
	if m.Entries == nil {
		m.Entries = map[string]ManifestEntry{}
	}
	return &m, nil
}

// MarshalManifest serializes a SyncManifest deterministically for remote PUT.
func MarshalManifest(m *SyncManifest) ([]byte, error) {
	if m == nil {
		return nil, errors.New("nil manifest")
	}
	if m.SchemaVersion == 0 {
		m.SchemaVersion = ManifestSchemaVersion
	}
	if m.Entries == nil {
		m.Entries = map[string]ManifestEntry{}
	}
	return json.MarshalIndent(m, "", "  ")
}

// LoadLocalSyncState reads `.asterlab/sync-state.json`, returning an initialized
// empty state if it does not yet exist.
func LoadLocalSyncState(vaultRoot, deviceID string) (*LocalSyncState, error) {
	path := filepath.Join(vaultRoot, filepath.FromSlash(LocalSyncStateRelPath))
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &LocalSyncState{
			SchemaVersion: ManifestSchemaVersion,
			DeviceID:      deviceID,
			Files:         map[string]LocalFileState{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	var st LocalSyncState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	if st.SchemaVersion == 0 {
		st.SchemaVersion = ManifestSchemaVersion
	}
	if st.DeviceID == "" {
		st.DeviceID = deviceID
	}
	if st.Files == nil {
		st.Files = map[string]LocalFileState{}
	}
	return &st, nil
}

// SaveLocalSyncStateAtomic persists `.asterlab/sync-state.json` atomically.
func SaveLocalSyncStateAtomic(vaultRoot string, st *LocalSyncState) error {
	if st == nil {
		return errors.New("nil sync state")
	}
	if st.SchemaVersion == 0 {
		st.SchemaVersion = ManifestSchemaVersion
	}
	if st.Files == nil {
		st.Files = map[string]LocalFileState{}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return asterdoc.WriteFileAtomic(filepath.Join(vaultRoot, filepath.FromSlash(LocalSyncStateRelPath)), data)
}

// SHA256Bytes returns the lowercase hex SHA-256 digest of data.
func SHA256Bytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// SHA256File streams a file on disk and returns its lowercase hex SHA-256 digest and byte size.
func SHA256File(absPath string) (string, int64, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}
