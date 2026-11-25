package syncengine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
)

// ConflictEntry describes a concurrent `.asterml` (or non-mergeable file) edit
// preserved without data loss by writing a `.sync-conflict-...` copy.
type ConflictEntry struct {
	RelPath          string `json:"relPath"`
	ConflictCopyPath string `json:"conflictCopyPath"`
	LocalSHA256      string `json:"localSha256"`
	RemoteSHA256     string `json:"remoteSha256"`
	RemoteDevice     string `json:"remoteDevice,omitempty"`
	DetectedAt       string `json:"detectedAt"`
}

// SyncReport summarizes what one Sync run transferred, deferred, merged, or flagged.
type SyncReport struct {
	VaultRevision         int64           `json:"vaultRevision"`
	UploadedFiles         []string        `json:"uploadedFiles,omitempty"`
	DownloadedOriginals   []string        `json:"downloadedOriginals,omitempty"`
	DownloadedPreviews    []string        `json:"downloadedPreviews,omitempty"`
	DeletedLocal          []string        `json:"deletedLocal,omitempty"`
	DeletedRemote         []string        `json:"deletedRemote,omitempty"`
	MergedPantry          bool            `json:"mergedPantry,omitempty"`
	Conflicts             []ConflictEntry `json:"conflicts,omitempty"`
	CaseCollisions        []CaseCollision `json:"caseCollisions,omitempty"`
	NeedsOriginalDownload []string        `json:"needsOriginalDownload,omitempty"`
	HTTPRequests          int64           `json:"httpRequests"`
	ManifestUpdated       bool            `json:"manifestUpdated"`
}

// CacheClearResult summarizes space reclaimed by ClearLocalOriginalsCache.
type CacheClearResult struct {
	FilesCleared int      `json:"filesCleared"`
	BytesFreed   int64    `json:"bytesFreed"`
	ClearedPaths []string `json:"clearedPaths,omitempty"`
}

// Engine orchestrates manifest-driven WebDAV sync for a local AsterLab Vault.
type Engine struct {
	VaultRoot string
	Client    *WebDAVClient
	Policy    DownloadPolicy
	DeviceID  string
	NowFn     func() time.Time
}

// NewEngine constructs a Sync Engine bound to vaultRoot and a configured WebDAVClient.
func NewEngine(vaultRoot string, client *WebDAVClient, policy DownloadPolicy, deviceID string) (*Engine, error) {
	if strings.TrimSpace(vaultRoot) == "" {
		return nil, errors.New("syncengine: VaultRoot is required")
	}
	if client == nil {
		return nil, errors.New("syncengine: WebDAVClient is required")
	}
	if strings.TrimSpace(deviceID) == "" {
		deviceID = "device"
	}
	return &Engine{
		VaultRoot: filepath.Clean(vaultRoot),
		Client:    client,
		Policy:    policy,
		DeviceID:  sanitizeDeviceTag(deviceID),
		NowFn:     time.Now,
	}, nil
}

// Sync runs one complete manifest-driven synchronization pass:
//  1. Scans local syncable vault files, normalizes paths, and checks case collisions.
//  2. Fetches `.asterlab/sync-manifest.json` in 1 GET (no recursive PROPFIND!).
//  3. Reconciles changed files using SHA-256 + logical revisions, enforcing
//     lossless original upload + preview sidecar generation, mobile threshold
//     preview-only downloads, `.asterml` conflict copy preservation, and
//     conflict-free `pantry.jsonl` append-only merge.
//  4. Uploads `.asterlab/sync-manifest.json` once only if changes occurred.
func (e *Engine) Sync(ctx context.Context) (SyncReport, error) {
	startReqs := e.Client.RequestCount()
	var report SyncReport

	localFiles, err := e.scanLocalVault()
	if err != nil {
		return report, err
	}

	allLocalPaths := make([]string, 0, len(localFiles))
	for p := range localFiles {
		allLocalPaths = append(allLocalPaths, p)
	}
	if collisions := CheckCaseCollisions(allLocalPaths); len(collisions) > 0 {
		report.CaseCollisions = collisions
		return report, fmt.Errorf("syncengine: case collision detected for %q", collisions[0].LowerPath)
	}

	state, err := LoadLocalSyncState(e.VaultRoot, e.DeviceID)
	if err != nil {
		return report, err
	}

	remoteManifest, err := e.fetchRemoteManifest(ctx)
	if err != nil {
		return report, err
	}

	manifestDirty := false
	pathSet := map[string]struct{}{}
	for p := range localFiles {
		pathSet[p] = struct{}{}
	}
	for p := range remoteManifest.Entries {
		if normP, nErr := NormalizeVaultPath(p); nErr == nil {
			pathSet[normP] = struct{}{}
		}
	}
	for p := range state.Files {
		pathSet[p] = struct{}{}
	}

	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, relPath := range paths {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if IsSyncIgnored(relPath) || IsConflictCopyPath(relPath) {
			continue
		}

		localInfo, hasLocal := localFiles[relPath]
		remoteEntry, hasRemote := remoteManifest.Entries[relPath]
		baseState, hasBase := state.Files[relPath]

		// Determine local and remote changes relative to last synced base SHA-256.
		// Note: When a mobile client intentionally deferred an original (NeedsOriginalDownload),
		// missing local original is NOT treated as a local deletion.
		localChanged := false
		if hasLocal {
			if !hasBase || baseState.BaseSHA256 != localInfo.sha256 {
				localChanged = true
			}
		} else if hasBase && baseState.BaseSHA256 != "" && !baseState.NeedsOriginalDownload {
			localChanged = true // local deletion
		}

		remoteChanged := false
		if hasRemote {
			if !hasBase || baseState.BaseSHA256 != remoteEntry.SHA256 || (remoteEntry.Deleted && baseState.BaseSHA256 != "") {
				remoteChanged = true
			}
		}

		// Fast path: identical content on local and remote (or initial adoption of identical files).
		if hasLocal && hasRemote && !remoteEntry.Deleted && localInfo.sha256 == remoteEntry.SHA256 {
			if err := e.ensureAssetSidecarsSyncedLocally(ctx, relPath, localInfo, remoteEntry, &report); err != nil {
				return report, err
			}
			state.Files[relPath] = LocalFileState{
				RelPath:               relPath,
				BaseSHA256:            remoteEntry.SHA256,
				BaseRevision:          remoteEntry.Revision,
				NeedsOriginalDownload: false,
				PreviewCached:         e.hasLocalPreview(remoteEntry.PreviewPath),
				OriginalCached:        IsAssetPath(relPath),
				RemoteOriginalSize:    remoteEntry.Size,
				OriginalAssetPath:     remoteEntry.OriginalAssetPath,
				PreviewPath:           remoteEntry.PreviewPath,
				PreviewSHA256:         remoteEntry.PreviewSHA256,
			}
			continue
		}

		// Fast path for mobile preview-only already cached and remote unchanged.
		if !hasLocal && hasBase && baseState.NeedsOriginalDownload && hasRemote && !remoteEntry.Deleted && baseState.BaseSHA256 == remoteEntry.SHA256 {
			if e.Policy.ShouldDownloadOriginal(remoteEntry.Size, IsAssetPath(relPath)) {
				// User changed policy (e.g. switched to AlwaysOriginal or raised threshold): pull original now.
				if err := e.pullRemoteEntry(ctx, relPath, remoteEntry, state, &report); err != nil {
					return report, err
				}
			} else if !e.hasLocalPreview(remoteEntry.PreviewPath) && remoteEntry.PreviewPath != "" {
				if err := e.pullPreviewOnly(ctx, relPath, remoteEntry, state, &report); err != nil {
					return report, err
				}
			}
			continue
		}

		// Case 1: Both local & remote missing (clean up stale state).
		if !hasLocal && (!hasRemote || remoteEntry.Deleted) {
			delete(state.Files, relPath)
			continue
		}

		// Case 2: Both local and remote changed concurrently.
		if localChanged && remoteChanged {
			if !hasLocal && (!hasRemote || remoteEntry.Deleted) {
				delete(state.Files, relPath)
				continue
			}
			if hasLocal && hasRemote && !remoteEntry.Deleted {
				if relPath == PantryLedgerRelPath {
					updatedEntry, err := e.mergeAndUploadPantry(ctx, localInfo, remoteEntry, &report)
					if err != nil {
						return report, err
					}
					remoteManifest.Entries[relPath] = updatedEntry
					manifestDirty = true
					state.Files[relPath] = LocalFileState{
						RelPath:      relPath,
						BaseSHA256:   updatedEntry.SHA256,
						BaseRevision: updatedEntry.Revision,
					}
					continue
				}
				// Non-mergeable concurrent edit (e.g. .asterml): preserve local + write conflict copy.
				if err := e.handleConcurrentConflict(ctx, relPath, localInfo, remoteEntry, state, &report); err != nil {
					return report, err
				}
				continue
			}
			// If one side deleted and other modified, preserve existing content rather than silently deleting.
			if !hasLocal && hasRemote && !remoteEntry.Deleted {
				if err := e.pullRemoteEntry(ctx, relPath, remoteEntry, state, &report); err != nil {
					return report, err
				}
				continue
			}
		}

		// Case 3: Local changed only -> push to remote.
		if localChanged && !remoteChanged {
			if hasLocal {
				entry, err := e.pushLocalFile(ctx, relPath, localInfo, remoteEntry, &report)
				if err != nil {
					return report, err
				}
				remoteManifest.Entries[relPath] = entry
				manifestDirty = true
				origCached := IsAssetPath(relPath)
				needsOrig := false
				if IsAssetPath(relPath) && e.Policy.AutoPruneMobileOriginalOnUpload && e.Policy.IsMobile &&
					!e.Policy.ShouldDownloadOriginal(entry.Size, true) && entry.PreviewPath != "" {
					if rmErr := os.Remove(localInfo.absPath); rmErr == nil {
						origCached = false
						needsOrig = true
					}
				}
				state.Files[relPath] = LocalFileState{
					RelPath:               relPath,
					BaseSHA256:            entry.SHA256,
					BaseRevision:          entry.Revision,
					NeedsOriginalDownload: needsOrig,
					PreviewCached:         e.hasLocalPreview(entry.PreviewPath),
					OriginalCached:        origCached,
					RemoteOriginalSize:    entry.Size,
					OriginalAssetPath:     entry.OriginalAssetPath,
					PreviewPath:           entry.PreviewPath,
					PreviewSHA256:         entry.PreviewSHA256,
				}
			} else {
				// Local file was deleted.
				if err := e.pushLocalDelete(ctx, relPath, remoteEntry, remoteManifest, &report); err != nil {
					return report, err
				}
				manifestDirty = true
				delete(state.Files, relPath)
			}
			continue
		}

		// Case 4: Remote changed only -> pull from remote.
		if remoteChanged && !localChanged {
			if remoteEntry.Deleted {
				if hasLocal {
					if err := os.Remove(localInfo.absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
						return report, err
					}
					report.DeletedLocal = append(report.DeletedLocal, relPath)
				}
				delete(state.Files, relPath)
				continue
			}
			if err := e.pullRemoteEntry(ctx, relPath, remoteEntry, state, &report); err != nil {
				return report, err
			}
		}
	}

	if manifestDirty {
		remoteManifest.VaultRevision++
		remoteManifest.UpdatedAt = e.nowUTC().Format(time.RFC3339)
		remoteManifest.UpdatedByDevice = e.DeviceID
		manifestBytes, err := MarshalManifest(remoteManifest)
		if err != nil {
			return report, err
		}
		if err := e.Client.Put(ctx, ManifestRelPath, manifestBytes, "application/json; charset=utf-8"); err != nil {
			return report, err
		}
		report.ManifestUpdated = true
	}

	state.LastSyncedVaultRev = remoteManifest.VaultRevision
	state.LastSyncAt = e.nowUTC().Format(time.RFC3339)
	if err := SaveLocalSyncStateAtomic(e.VaultRoot, state); err != nil {
		return report, err
	}

	for _, fs := range state.Files {
		if fs.NeedsOriginalDownload {
			report.NeedsOriginalDownload = append(report.NeedsOriginalDownload, fs.RelPath)
		}
	}
	sort.Strings(report.NeedsOriginalDownload)
	report.VaultRevision = remoteManifest.VaultRevision
	report.HTTPRequests = e.Client.RequestCount() - startReqs
	return report, nil
}

// FetchOriginalOnDemand downloads the 100% lossless original for an asset whose
// download was previously deferred to preview-only on mobile, verifies its SHA-256,
// stores it at `<vault>/<relPath>`, and clears NeedsOriginalDownload.
func (e *Engine) FetchOriginalOnDemand(ctx context.Context, relPath string) error {
	normPath, err := NormalizeVaultPath(relPath)
	if err != nil {
		return err
	}
	state, err := LoadLocalSyncState(e.VaultRoot, e.DeviceID)
	if err != nil {
		return err
	}
	var wantSHA, remoteOrigPath, previewPath, previewSHA string
	var rev, size int64
	if fs, ok := state.Files[normPath]; ok && fs.BaseSHA256 != "" {
		wantSHA = fs.BaseSHA256
		remoteOrigPath = fs.OriginalAssetPath
		previewPath = fs.PreviewPath
		previewSHA = fs.PreviewSHA256
		rev = fs.BaseRevision
		size = fs.RemoteOriginalSize
	} else {
		manifest, mErr := e.fetchRemoteManifest(ctx)
		if mErr != nil {
			return mErr
		}
		entry, ok := manifest.Entries[normPath]
		if !ok || entry.Deleted {
			return fmt.Errorf("syncengine: %q not found in remote manifest", normPath)
		}
		wantSHA = entry.SHA256
		remoteOrigPath = entry.OriginalAssetPath
		previewPath = entry.PreviewPath
		previewSHA = entry.PreviewSHA256
		rev = entry.Revision
		size = entry.Size
	}
	if remoteOrigPath == "" {
		remoteOrigPath = normPath
	}

	data, err := e.Client.GetVerified(ctx, remoteOrigPath, wantSHA)
	if err != nil {
		return err
	}
	if err := asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(normPath)), data); err != nil {
		return err
	}
	state.Files[normPath] = LocalFileState{
		RelPath:               normPath,
		BaseSHA256:            wantSHA,
		BaseRevision:          rev,
		NeedsOriginalDownload: false,
		PreviewCached:         e.hasLocalPreview(previewPath),
		OriginalCached:        true,
		RemoteOriginalSize:    size,
		OriginalAssetPath:     remoteOrigPath,
		PreviewPath:           previewPath,
		PreviewSHA256:         previewSHA,
	}
	return SaveLocalSyncStateAtomic(e.VaultRoot, state)
}

// ClearLocalOriginalsCache removes locally cached lossless original assets that
// are already safely backed by a synced remote copy (and preserves their local
// preview sidecars), freeing mobile disk space without touching remote data.
func (e *Engine) ClearLocalOriginalsCache() (CacheClearResult, error) {
	state, err := LoadLocalSyncState(e.VaultRoot, e.DeviceID)
	if err != nil {
		return CacheClearResult{}, err
	}
	var res CacheClearResult
	for relPath, fs := range state.Files {
		if !IsAssetPath(relPath) || fs.BaseSHA256 == "" {
			continue
		}
		absPath := filepath.Join(e.VaultRoot, filepath.FromSlash(relPath))
		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			continue
		}
		if err := os.Remove(absPath); err != nil {
			return res, err
		}
		res.FilesCleared++
		res.BytesFreed += info.Size()
		res.ClearedPaths = append(res.ClearedPaths, relPath)
		fs.OriginalCached = false
		fs.NeedsOriginalDownload = true
		state.Files[relPath] = fs
	}
	sort.Strings(res.ClearedPaths)
	if err := SaveLocalSyncStateAtomic(e.VaultRoot, state); err != nil {
		return res, err
	}
	return res, nil
}

// LocalOriginalsCacheUsage returns total bytes and file count currently occupied by
// synced local original assets that can be safely freed.
func (e *Engine) LocalOriginalsCacheUsage() (int64, int, error) {
	state, err := LoadLocalSyncState(e.VaultRoot, e.DeviceID)
	if err != nil {
		return 0, 0, err
	}
	var totalBytes int64
	var count int
	for relPath, fs := range state.Files {
		if !IsAssetPath(relPath) || fs.BaseSHA256 == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(e.VaultRoot, filepath.FromSlash(relPath)))
		if err == nil && !info.IsDir() {
			totalBytes += info.Size()
			count++
		}
	}
	return totalBytes, count, nil
}

type localFileInfo struct {
	relPath string
	absPath string
	sha256  string
	size    int64
	modUTC  string
}

func (e *Engine) scanLocalVault() (map[string]localFileInfo, error) {
	files := map[string]localFileInfo{}
	err := filepath.WalkDir(e.VaultRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == e.VaultRoot {
			return nil
		}
		relSlash := filepath.ToSlash(mustRel(e.VaultRoot, p))
		if d.IsDir() {
			if IsSyncIgnored(relSlash + "/") {
				return filepath.SkipDir
			}
			return nil
		}
		if IsSyncIgnored(relSlash) || IsConflictCopyPath(relSlash) {
			return nil
		}
		normPath, err := NormalizeVaultPath(relSlash)
		if err != nil {
			return err
		}
		sum, size, err := SHA256File(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files[normPath] = localFileInfo{
			relPath: normPath,
			absPath: p,
			sha256:  sum,
			size:    size,
			modUTC:  info.ModTime().UTC().Format(time.RFC3339),
		}
		return nil
	})
	return files, err
}

func (e *Engine) fetchRemoteManifest(ctx context.Context) (*SyncManifest, error) {
	raw, err := e.Client.Get(ctx, ManifestRelPath)
	if errors.Is(err, ErrNotFound) {
		return NewSyncManifest(e.DeviceID), nil
	}
	if err != nil {
		return nil, err
	}
	return ParseManifest(raw)
}

func (e *Engine) pushLocalFile(ctx context.Context, relPath string, info localFileInfo, prev ManifestEntry, report *SyncReport) (ManifestEntry, error) {
	data, err := os.ReadFile(info.absPath)
	if err != nil {
		return ManifestEntry{}, err
	}
	entry := ManifestEntry{
		RelPath:    relPath,
		SHA256:     info.sha256,
		Size:       info.size,
		Revision:   prev.Revision + 1,
		ModTimeUTC: info.modUTC,
		DeviceID:   e.DeviceID,
	}

	if IsAssetPath(relPath) {
		origRemotePath := CanonicalOriginalPath(info.sha256, relPath)
		entry.OriginalAssetPath = origRemotePath
		if err := e.Client.Put(ctx, origRemotePath, data, "application/octet-stream"); err != nil {
			return ManifestEntry{}, err
		}
		if prevBytes, _, ok, pErr := GeneratePreviewJPEG(data, DefaultPreviewMaxDim, DefaultPreviewQuality); pErr == nil && ok {
			prevPath := CanonicalPreviewPath(info.sha256)
			prevSHA := SHA256Bytes(prevBytes)
			if err := asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(prevPath)), prevBytes); err != nil {
				return ManifestEntry{}, err
			}
			if err := e.Client.Put(ctx, prevPath, prevBytes, "image/jpeg"); err != nil {
				return ManifestEntry{}, err
			}
			entry.PreviewPath = prevPath
			entry.PreviewSHA256 = prevSHA
			entry.PreviewSize = int64(len(prevBytes))
		}
	} else {
		if err := e.Client.Put(ctx, relPath, data, ""); err != nil {
			return ManifestEntry{}, err
		}
	}

	report.UploadedFiles = append(report.UploadedFiles, relPath)
	return entry, nil
}

func (e *Engine) pushLocalDelete(ctx context.Context, relPath string, prev ManifestEntry, manifest *SyncManifest, report *SyncReport) error {
	if !IsAssetPath(relPath) {
		_ = e.Client.Delete(ctx, relPath)
	}
	manifest.Entries[relPath] = ManifestEntry{
		RelPath:  relPath,
		SHA256:   "",
		Size:     0,
		Revision: prev.Revision + 1,
		Deleted:  true,
		DeviceID: e.DeviceID,
	}
	report.DeletedRemote = append(report.DeletedRemote, relPath)
	return nil
}

func (e *Engine) pullRemoteEntry(ctx context.Context, relPath string, entry ManifestEntry, state *LocalSyncState, report *SyncReport) error {
	if IsAssetPath(relPath) {
		if e.Policy.ShouldDownloadOriginal(entry.Size, true) {
			remoteSrc := entry.OriginalAssetPath
			if remoteSrc == "" {
				remoteSrc = relPath
			}
			data, err := e.Client.GetVerified(ctx, remoteSrc, entry.SHA256)
			if err != nil {
				return err
			}
			if err := asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(relPath)), data); err != nil {
				return err
			}
			report.DownloadedOriginals = append(report.DownloadedOriginals, relPath)

			prevCached := false
			if entry.PreviewPath != "" {
				if e.hasLocalPreview(entry.PreviewPath) {
					prevCached = true
				} else if prevBytes, pErr := e.Client.GetVerified(ctx, entry.PreviewPath, entry.PreviewSHA256); pErr == nil {
					if asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(entry.PreviewPath)), prevBytes) == nil {
						prevCached = true
						report.DownloadedPreviews = append(report.DownloadedPreviews, entry.PreviewPath)
					}
				}
			}

			state.Files[relPath] = LocalFileState{
				RelPath:               relPath,
				BaseSHA256:            entry.SHA256,
				BaseRevision:          entry.Revision,
				NeedsOriginalDownload: false,
				PreviewCached:         prevCached,
				OriginalCached:        true,
				RemoteOriginalSize:    entry.Size,
				OriginalAssetPath:     remoteSrc,
				PreviewPath:           entry.PreviewPath,
				PreviewSHA256:         entry.PreviewSHA256,
			}
			return nil
		}
		return e.pullPreviewOnly(ctx, relPath, entry, state, report)
	}

	data, err := e.Client.GetVerified(ctx, relPath, entry.SHA256)
	if err != nil {
		return err
	}
	if err := asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(relPath)), data); err != nil {
		return err
	}
	report.DownloadedOriginals = append(report.DownloadedOriginals, relPath)
	state.Files[relPath] = LocalFileState{
		RelPath:      relPath,
		BaseSHA256:   entry.SHA256,
		BaseRevision: entry.Revision,
	}
	return nil
}

func (e *Engine) pullPreviewOnly(ctx context.Context, relPath string, entry ManifestEntry, state *LocalSyncState, report *SyncReport) error {
	prevCached := false
	if entry.PreviewPath != "" {
		if e.hasLocalPreview(entry.PreviewPath) {
			prevCached = true
		} else {
			prevData, err := e.Client.GetVerified(ctx, entry.PreviewPath, entry.PreviewSHA256)
			if err != nil {
				return err
			}
			if err := asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(entry.PreviewPath)), prevData); err != nil {
				return err
			}
			prevCached = true
			report.DownloadedPreviews = append(report.DownloadedPreviews, entry.PreviewPath)
		}
	}
	remoteOrig := entry.OriginalAssetPath
	if remoteOrig == "" {
		remoteOrig = relPath
	}
	state.Files[relPath] = LocalFileState{
		RelPath:               relPath,
		BaseSHA256:            entry.SHA256,
		BaseRevision:          entry.Revision,
		NeedsOriginalDownload: true,
		PreviewCached:         prevCached,
		OriginalCached:        false,
		RemoteOriginalSize:    entry.Size,
		OriginalAssetPath:     remoteOrig,
		PreviewPath:           entry.PreviewPath,
		PreviewSHA256:         entry.PreviewSHA256,
	}
	return nil
}

func (e *Engine) ensureAssetSidecarsSyncedLocally(ctx context.Context, relPath string, info localFileInfo, entry ManifestEntry, report *SyncReport) error {
	if !IsAssetPath(relPath) || entry.PreviewPath == "" {
		return nil
	}
	if e.hasLocalPreview(entry.PreviewPath) {
		return nil
	}
	data, err := os.ReadFile(info.absPath)
	if err != nil {
		return err
	}
	if prevBytes, _, ok, pErr := GeneratePreviewJPEG(data, DefaultPreviewMaxDim, DefaultPreviewQuality); pErr == nil && ok {
		return asterdoc.WriteFileAtomic(filepath.Join(e.VaultRoot, filepath.FromSlash(entry.PreviewPath)), prevBytes)
	}
	return nil
}

func (e *Engine) mergeAndUploadPantry(ctx context.Context, localInfo localFileInfo, remoteEntry ManifestEntry, report *SyncReport) (ManifestEntry, error) {
	localBytes, err := os.ReadFile(localInfo.absPath)
	if err != nil {
		return ManifestEntry{}, err
	}
	remoteBytes, err := e.Client.GetVerified(ctx, PantryLedgerRelPath, remoteEntry.SHA256)
	if err != nil {
		return ManifestEntry{}, err
	}
	merged, _, err := MergePantryJSONL(localBytes, remoteBytes)
	if err != nil {
		return ManifestEntry{}, err
	}
	if err := asterdoc.WriteFileAtomic(localInfo.absPath, merged); err != nil {
		return ManifestEntry{}, err
	}
	if err := e.Client.Put(ctx, PantryLedgerRelPath, merged, "application/x-ndjson; charset=utf-8"); err != nil {
		return ManifestEntry{}, err
	}
	report.MergedPantry = true
	report.UploadedFiles = append(report.UploadedFiles, PantryLedgerRelPath)
	return ManifestEntry{
		RelPath:    PantryLedgerRelPath,
		SHA256:     SHA256Bytes(merged),
		Size:       int64(len(merged)),
		Revision:   remoteEntry.Revision + 1,
		ModTimeUTC: e.nowUTC().Format(time.RFC3339),
		DeviceID:   e.DeviceID,
	}, nil
}

func (e *Engine) handleConcurrentConflict(ctx context.Context, relPath string, localInfo localFileInfo, remoteEntry ManifestEntry, state *LocalSyncState, report *SyncReport) error {
	// Never emit duplicate conflict copies for the exact same remote SHA-256 already captured.
	if prev, ok := state.Files[relPath]; ok && prev.LastConflictRemoteSHA == remoteEntry.SHA256 {
		return nil
	}

	remoteSrc := relPath
	if IsAssetPath(relPath) && remoteEntry.OriginalAssetPath != "" {
		remoteSrc = remoteEntry.OriginalAssetPath
	}
	remoteBytes, err := e.Client.GetVerified(ctx, remoteSrc, remoteEntry.SHA256)
	if err != nil {
		return err
	}

	stamp := e.nowUTC().Format("20060102-150405")
	dev := sanitizeDeviceTag(remoteEntry.DeviceID)
	if dev == "" {
		dev = "remote"
	}
	conflictRel := buildConflictCopyPath(relPath, dev, stamp)
	conflictAbs := filepath.Join(e.VaultRoot, filepath.FromSlash(conflictRel))
	if err := asterdoc.WriteFileAtomic(conflictAbs, remoteBytes); err != nil {
		return err
	}

	ce := ConflictEntry{
		RelPath:          relPath,
		ConflictCopyPath: conflictRel,
		LocalSHA256:      localInfo.sha256,
		RemoteSHA256:     remoteEntry.SHA256,
		RemoteDevice:     remoteEntry.DeviceID,
		DetectedAt:       e.nowUTC().Format(time.RFC3339),
	}
	report.Conflicts = append(report.Conflicts, ce)

	cur := state.Files[relPath]
	cur.RelPath = relPath
	cur.LastConflictRemoteSHA = remoteEntry.SHA256
	state.Files[relPath] = cur
	return nil
}

func (e *Engine) hasLocalPreview(previewRel string) bool {
	if previewRel == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(e.VaultRoot, filepath.FromSlash(previewRel)))
	return err == nil && !info.IsDir()
}

func (e *Engine) nowUTC() time.Time {
	if e.NowFn != nil {
		return e.NowFn().UTC()
	}
	return time.Now().UTC()
}

func buildConflictCopyPath(relPath, device, timestamp string) string {
	dir := path.Dir(relPath)
	base := path.Base(relPath)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".asterml"
	}
	name := fmt.Sprintf("%s.sync-conflict-%s-%s%s", stem, device, timestamp, ext)
	if dir == "." || dir == "" {
		return name
	}
	return dir + "/" + name
}

func sanitizeDeviceTag(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func mustRel(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return target
	}
	return rel
}
