package recovery

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	BackupManifestPath    = ".asterlab/backup-manifest.json"
	BackupSchemaVersion   = 1
	maxBackupEntryBytes   = 2 << 30 // 2 GiB safety limit per entry against zip bombs
)

// BackupOptions configures what auxiliary directories are included in a full-vault zip backup.
type BackupOptions struct {
	IncludeHistory  bool
	IncludeTrash    bool
	IncludePreviews bool
}

// DefaultBackupOptions includes vault notes, assets, pantry, config, trash, and previews,
// while excluding ephemeral caches and redundant history blobs unless requested.
func DefaultBackupOptions() BackupOptions {
	return BackupOptions{
		IncludeHistory:  false,
		IncludeTrash:    true,
		IncludePreviews: true,
	}
}

// BackupManifest is embedded inside every backup zip at `.asterlab/backup-manifest.json`
// to guarantee full cryptographic integrity verification on restore.
type BackupManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	CreatedAt     string            `json:"createdAt"`
	VaultName     string            `json:"vaultName"`
	FileCount     int               `json:"fileCount"`
	TotalBytes    int64             `json:"totalBytes"`
	Checksums     map[string]string `json:"checksums"`
}

// BackupResult summarizes a created vault backup archive.
type BackupResult struct {
	ZipPath    string `json:"zipPath"`
	FileCount  int    `json:"fileCount"`
	TotalBytes int64  `json:"totalBytes"`
}

// RestoreResult summarizes a verified vault restore operation.
type RestoreResult struct {
	VaultRoot  string `json:"vaultRoot"`
	FileCount  int    `json:"fileCount"`
	TotalBytes int64  `json:"totalBytes"`
	RolledBack bool   `json:"rolledBack,omitempty"`
}

// BackupVaultZip exports the entire vault at vaultRoot into destZipPath atomically,
// embedding `.asterlab/backup-manifest.json` with SHA-256 checksums for every file.
func BackupVaultZip(vaultRoot, destZipPath string, opts ...BackupOptions) (BackupResult, error) {
	if strings.TrimSpace(vaultRoot) == "" || strings.TrimSpace(destZipPath) == "" {
		return BackupResult{}, errors.New("recovery: vaultRoot and destZipPath are required")
	}
	info, err := os.Stat(vaultRoot)
	if err != nil || !info.IsDir() {
		return BackupResult{}, fmt.Errorf("recovery: vaultRoot %q is not an accessible directory", vaultRoot)
	}
	opt := DefaultBackupOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	type fileEntry struct {
		rel string
		abs string
	}
	var files []fileEntry
	err = filepath.WalkDir(vaultRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == vaultRoot {
			return nil
		}
		rel, rErr := filepath.Rel(vaultRoot, p)
		if rErr != nil {
			return rErr
		}
		relSlash := filepath.ToSlash(rel)
		lower := strings.ToLower(relSlash)
		if d.IsDir() {
			if lower == ".asterlab/cache" {
				return filepath.SkipDir
			}
			if !opt.IncludeHistory && lower == ".asterlab/history" {
				return filepath.SkipDir
			}
			if !opt.IncludeTrash && lower == ".asterlab/trash" {
				return filepath.SkipDir
			}
			if !opt.IncludePreviews && lower == ".asterlab/previews" {
				return filepath.SkipDir
			}
			return nil
		}
		base := strings.ToLower(filepath.Base(relSlash))
		if strings.HasSuffix(base, ".tmp") || strings.HasPrefix(base, ".asterlab-") {
			return nil
		}
		if relSlash == BackupManifestPath {
			return nil
		}
		files = append(files, fileEntry{rel: relSlash, abs: p})
		return nil
	})
	if err != nil {
		return BackupResult{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })

	if err := os.MkdirAll(filepath.Dir(destZipPath), 0o755); err != nil {
		return BackupResult{}, err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(destZipPath), "asterlab-backup-*.zip.tmp")
	if err != nil {
		return BackupResult{}, err
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	zw := zip.NewWriter(tmpFile)
	manifest := BackupManifest{
		SchemaVersion: BackupSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		VaultName:     filepath.Base(filepath.Clean(vaultRoot)),
		Checksums:     make(map[string]string, len(files)),
	}

	for _, fe := range files {
		fi, sErr := os.Stat(fe.abs)
		if sErr != nil {
			_ = zw.Close()
			cleanupTmp()
			return BackupResult{}, sErr
		}
		hdr, hErr := zip.FileInfoHeader(fi)
		if hErr != nil {
			_ = zw.Close()
			cleanupTmp()
			return BackupResult{}, hErr
		}
		hdr.Name = fe.rel
		hdr.Method = zip.Deflate

		w, cErr := zw.CreateHeader(hdr)
		if cErr != nil {
			_ = zw.Close()
			cleanupTmp()
			return BackupResult{}, cErr
		}
		src, oErr := os.Open(fe.abs)
		if oErr != nil {
			_ = zw.Close()
			cleanupTmp()
			return BackupResult{}, oErr
		}
		h := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(w, h), src)
		_ = src.Close()
		if copyErr != nil {
			_ = zw.Close()
			cleanupTmp()
			return BackupResult{}, copyErr
		}
		manifest.Checksums[fe.rel] = hex.EncodeToString(h.Sum(nil))
		manifest.TotalBytes += written
		manifest.FileCount++
	}

	manifestBytes, mErr := json.MarshalIndent(manifest, "", "  ")
	if mErr != nil {
		_ = zw.Close()
		cleanupTmp()
		return BackupResult{}, mErr
	}
	mw, mwErr := zw.Create(BackupManifestPath)
	if mwErr != nil {
		_ = zw.Close()
		cleanupTmp()
		return BackupResult{}, mwErr
	}
	if _, wErr := mw.Write(manifestBytes); wErr != nil {
		_ = zw.Close()
		cleanupTmp()
		return BackupResult{}, wErr
	}
	if err := zw.Close(); err != nil {
		cleanupTmp()
		return BackupResult{}, err
	}
	if err := tmpFile.Sync(); err != nil {
		cleanupTmp()
		return BackupResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return BackupResult{}, err
	}
	if err := os.Rename(tmpPath, destZipPath); err != nil {
		_ = os.Remove(tmpPath)
		return BackupResult{}, err
	}

	return BackupResult{
		ZipPath:    destZipPath,
		FileCount:  manifest.FileCount,
		TotalBytes: manifest.TotalBytes,
	}, nil
}

// VerifyBackupZip validates that zipPath is a well-formed AsterLab backup archive:
// rejects path-traversal/zip-slip entries, requires `.asterlab/backup-manifest.json`,
// and verifies CRC32 + SHA-256 of every archived file against the manifest.
func VerifyBackupZip(zipPath string) (BackupManifest, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return BackupManifest{}, fmt.Errorf("recovery: invalid backup zip: %w", err)
	}
	defer zr.Close()

	var manifest *BackupManifest
	actualHashes := make(map[string]string, len(zr.File))
	for _, f := range zr.File {
		if err := validateZipEntryPath(f.Name); err != nil {
			return BackupManifest{}, err
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return BackupManifest{}, fmt.Errorf("recovery: corrupt zip entry %q: %w", f.Name, err)
		}
		h := sha256.New()
		limited := io.LimitReader(rc, maxBackupEntryBytes+1)
		var buf strings.Builder
		if f.Name == BackupManifestPath {
			n, copyErr := io.Copy(&buf, limited)
			_ = rc.Close()
			if copyErr != nil || n > maxBackupEntryBytes {
				return BackupManifest{}, fmt.Errorf("recovery: unreadable backup manifest: %v", copyErr)
			}
			var m BackupManifest
			if uErr := json.Unmarshal([]byte(buf.String()), &m); uErr != nil {
				return BackupManifest{}, fmt.Errorf("recovery: corrupt backup manifest JSON: %w", uErr)
			}
			manifest = &m
			continue
		}
		n, copyErr := io.Copy(h, limited)
		closeErr := rc.Close()
		if copyErr != nil {
			return BackupManifest{}, fmt.Errorf("recovery: corrupt entry %q: %w", f.Name, copyErr)
		}
		if closeErr != nil {
			return BackupManifest{}, fmt.Errorf("recovery: corrupt entry %q on close: %w", f.Name, closeErr)
		}
		if n > maxBackupEntryBytes {
			return BackupManifest{}, fmt.Errorf("recovery: zip entry %q exceeds size cap", f.Name)
		}
		actualHashes[f.Name] = hex.EncodeToString(h.Sum(nil))
	}

	if manifest == nil {
		return BackupManifest{}, errors.New("recovery: missing .asterlab/backup-manifest.json in backup archive")
	}
	if len(actualHashes) != len(manifest.Checksums) {
		return BackupManifest{}, fmt.Errorf("recovery: backup file count mismatch (archive=%d manifest=%d)", len(actualHashes), len(manifest.Checksums))
	}
	for rel, wantHash := range manifest.Checksums {
		gotHash, ok := actualHashes[rel]
		if !ok {
			return BackupManifest{}, fmt.Errorf("recovery: backup missing expected file %q", rel)
		}
		if !strings.EqualFold(gotHash, wantHash) {
			return BackupManifest{}, fmt.Errorf("recovery: checksum mismatch for %q (got %s want %s)", rel, gotHash, wantHash)
		}
	}
	return *manifest, nil
}

// RestoreVaultZip safely restores a verified backup zip to targetVaultRoot using:
//  1. Pre-flight cryptographic archive verification (never touches live vault if corrupt).
//  2. Full extraction + verification into a sibling staging directory.
//  3. Atomic directory swap with automatic rollback to the previous live vault on any failure.
func RestoreVaultZip(zipPath, targetVaultRoot string) (RestoreResult, error) {
	return restoreVaultZipWithHook(zipPath, targetVaultRoot, nil)
}

func restoreVaultZipWithHook(zipPath, targetVaultRoot string, postStageHook func(stageDir string) error) (RestoreResult, error) {
	if strings.TrimSpace(targetVaultRoot) == "" {
		return RestoreResult{}, errors.New("recovery: targetVaultRoot is required")
	}
	targetVaultRoot = filepath.Clean(targetVaultRoot)
	manifest, err := VerifyBackupZip(zipPath)
	if err != nil {
		return RestoreResult{}, err
	}

	parent := filepath.Dir(targetVaultRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return RestoreResult{}, err
	}
	stageDir, err := os.MkdirTemp(parent, ".asterlab-restore-stage-*")
	if err != nil {
		return RestoreResult{}, err
	}
	cleanupStage := true
	defer func() {
		if cleanupStage {
			_ = os.RemoveAll(stageDir)
		}
	}()

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return RestoreResult{}, err
	}
	var totalBytes int64
	var fileCount int
	for _, f := range zr.File {
		if err := validateZipEntryPath(f.Name); err != nil {
			_ = zr.Close()
			return RestoreResult{}, err
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") || f.Name == BackupManifestPath {
			continue
		}
		dest := filepath.Join(stageDir, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			_ = zr.Close()
			return RestoreResult{}, err
		}
		rc, oErr := f.Open()
		if oErr != nil {
			_ = zr.Close()
			return RestoreResult{}, oErr
		}
		outF, cErr := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if cErr != nil {
			_ = rc.Close()
			_ = zr.Close()
			return RestoreResult{}, cErr
		}
		h := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(outF, h), io.LimitReader(rc, maxBackupEntryBytes+1))
		_ = rc.Close()
		closeErr := outF.Close()
		if copyErr != nil {
			_ = zr.Close()
			return RestoreResult{}, copyErr
		}
		if closeErr != nil {
			_ = zr.Close()
			return RestoreResult{}, closeErr
		}
		wantHash := manifest.Checksums[f.Name]
		gotHash := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(gotHash, wantHash) {
			_ = zr.Close()
			return RestoreResult{}, fmt.Errorf("recovery: staged checksum mismatch for %q", f.Name)
		}
		totalBytes += n
		fileCount++
	}
	_ = zr.Close()

	if fileCount != manifest.FileCount {
		return RestoreResult{}, fmt.Errorf("recovery: staged file count mismatch (%d vs %d)", fileCount, manifest.FileCount)
	}

	// Move existing vault aside to a rollback directory, swap in stageDir, and run
	// post-swap checks. Any failure triggers automatic rollback.
	var rollbackDir string
	hadExisting := false
	if _, statErr := os.Stat(targetVaultRoot); statErr == nil {
		hadExisting = true
		rollbackDir = filepath.Join(parent, fmt.Sprintf(".asterlab-rollback-%d", time.Now().UTC().UnixNano()))
		if err := os.Rename(targetVaultRoot, rollbackDir); err != nil {
			return RestoreResult{}, fmt.Errorf("recovery: prepare rollback backup: %w", err)
		}
	}

	rollback := func(cause error) (RestoreResult, error) {
		if hadExisting && rollbackDir != "" {
			_ = os.RemoveAll(targetVaultRoot)
			if rbErr := os.Rename(rollbackDir, targetVaultRoot); rbErr != nil {
				return RestoreResult{}, fmt.Errorf("recovery: restore failed (%v) AND rollback failed (%v)", cause, rbErr)
			}
			return RestoreResult{VaultRoot: targetVaultRoot, RolledBack: true}, cause
		}
		return RestoreResult{}, cause
	}

	if postStageHook != nil {
		if err := postStageHook(stageDir); err != nil {
			return rollback(err)
		}
	}

	if err := os.Rename(stageDir, targetVaultRoot); err != nil {
		return rollback(fmt.Errorf("recovery: promote staged restore: %w", err))
	}
	cleanupStage = false

	// Post-swap integrity check on live targetVaultRoot.
	for rel, wantHash := range manifest.Checksums {
		gotHash, _, hErr := hashVaultFile(filepath.Join(targetVaultRoot, filepath.FromSlash(rel)))
		if hErr != nil || !strings.EqualFold(gotHash, wantHash) {
			return rollback(fmt.Errorf("recovery: post-swap verification failed for %q", rel))
		}
	}

	if hadExisting && rollbackDir != "" {
		_ = os.RemoveAll(rollbackDir)
	}
	return RestoreResult{
		VaultRoot:  targetVaultRoot,
		FileCount:  fileCount,
		TotalBytes: totalBytes,
	}, nil
}

func validateZipEntryPath(name string) error {
	if strings.TrimSpace(name) == "" || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("recovery: invalid empty/null zip entry %q", name)
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") || strings.Contains(name, "\\") {
		return fmt.Errorf("recovery: unsafe zip path %q", name)
	}
	if len(name) >= 2 && name[1] == ':' {
		return fmt.Errorf("recovery: drive-prefixed zip path %q", name)
	}
	cleaned := filepath.ToSlash(filepath.Clean(name))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return fmt.Errorf("recovery: zip-slip traversal detected in %q", name)
	}
	return nil
}

func hashVaultFile(absPath string) (string, int64, error) {
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
