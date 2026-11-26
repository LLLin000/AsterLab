package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/syncengine"
	mygo "github.com/egoist/mygo"
)

const syncConfigRelPath = ".asterlab/sync_config.json"

// syncHostConfig holds Vault-scoped WebDAV sync settings persisted to
// `<Vault>/.asterlab/sync_config.json`. Passwords are NEVER written to disk;
// they are stored exclusively via mygo.Secrets (with a memory-only test fallback).
type syncHostConfig struct {
	Enabled                         bool                          `json:"enabled"`
	BaseURL                         string                        `json:"baseUrl"`
	Username                        string                        `json:"username"`
	Password                        string                        `json:"-"`
	PolicyMode                      syncengine.DownloadPolicyMode `json:"policyMode"`
	ThresholdBytes                  int64                         `json:"thresholdBytes"`
	WiFiOnlyForOriginals            bool                          `json:"wifiOnlyForOriginals"`
	AutoPruneMobileOriginalOnUpload bool                          `json:"autoPruneMobileOriginalOnUpload,omitempty"`
	ForceNutstoreMode               bool                          `json:"forceNutstoreMode,omitempty"`
	DeviceID                        string                        `json:"deviceId"`
	LastSyncAt                      string                        `json:"lastSyncAt,omitempty"`
	LastVaultRevision               int64                         `json:"lastVaultRevision,omitempty"`
	LastError                       string                        `json:"lastError,omitempty"`
}

func defaultSyncHostConfig() syncHostConfig {
	pol := syncengine.DefaultDesktopDownloadPolicy()
	if isMobileRuntime() {
		pol = syncengine.DefaultMobileDownloadPolicy()
	}
	return syncHostConfig{
		PolicyMode:                      pol.Mode,
		ThresholdBytes:                  pol.ThresholdBytes,
		WiFiOnlyForOriginals:            pol.WiFiOnlyForOriginals,
		AutoPruneMobileOriginalOnUpload: pol.AutoPruneMobileOriginalOnUpload,
		DeviceID:                        defaultSyncDeviceID(),
	}
}

func defaultSyncDeviceID() string {
	if host, err := os.Hostname(); err == nil && strings.TrimSpace(host) != "" {
		return strings.TrimSpace(host) + "-" + runtime.GOOS
	}
	return "asterlab-" + runtime.GOOS
}

func syncSecretAccountKey(vaultRoot string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(vaultRoot)))
	return "asterlab-webdav-" + hex.EncodeToString(sum[:8])
}

func (a *app) syncConfigPath() (string, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return "", errors.New("sync: no active vault")
	}
	return filepath.Join(a.vault.Root, filepath.FromSlash(syncConfigRelPath)), nil
}

func (a *app) storeWebDAVPassword(key, password string) error {
	a.syncSecretsMu.Lock()
	if a.syncSecretsFallback == nil {
		a.syncSecretsFallback = map[string]string{}
	}
	if password == "" {
		delete(a.syncSecretsFallback, key)
	} else {
		a.syncSecretsFallback[key] = password
	}
	a.syncSecretsMu.Unlock()

	if testing.Testing() {
		return nil
	}
	var err error
	if password == "" {
		err = mygo.Secrets.Delete(key)
		if errors.Is(err, mygo.ErrSecretNotFound) || errors.Is(err, errors.ErrUnsupported) {
			return nil
		}
		return err
	}
	err = mygo.Secrets.Set(key, []byte(password))
	if errors.Is(err, errors.ErrUnsupported) {
		return nil
	}
	return err
}

func (a *app) loadWebDAVPassword(key string) string {
	a.syncSecretsMu.Lock()
	memVal, hasMem := a.syncSecretsFallback[key]
	a.syncSecretsMu.Unlock()
	if hasMem {
		return memVal
	}
	if testing.Testing() {
		return ""
	}
	raw, err := mygo.Secrets.Get(key)
	if err != nil {
		return ""
	}
	return string(raw)
}

func (cfg syncHostConfig) downloadPolicy() syncengine.DownloadPolicy {
	pol := syncengine.DefaultDesktopDownloadPolicy()
	if isMobileRuntime() {
		pol = syncengine.DefaultMobileDownloadPolicy()
	}
	if cfg.PolicyMode != "" {
		pol.Mode = cfg.PolicyMode
	}
	if cfg.ThresholdBytes > 0 {
		pol.ThresholdBytes = cfg.ThresholdBytes
	}
	pol.WiFiOnlyForOriginals = cfg.WiFiOnlyForOriginals
	pol.AutoPruneMobileOriginalOnUpload = cfg.AutoPruneMobileOriginalOnUpload
	return pol
}

// loadSyncConfig reads non-secret sync settings from `<Vault>/.asterlab/sync_config.json`
// and hydrates the WebDAV password from `mygo.Secrets` (with safe headless fallback).
func (a *app) loadSyncConfig() (syncHostConfig, error) {
	cfgPath, err := a.syncConfigPath()
	if err != nil {
		return syncHostConfig{}, err
	}
	cfg := defaultSyncHostConfig()
	raw, err := os.ReadFile(cfgPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if err == nil && len(raw) > 0 {
		if uErr := json.Unmarshal(raw, &cfg); uErr != nil {
			return cfg, fmt.Errorf("sync: parse %s: %w", syncConfigRelPath, uErr)
		}
	}
	if cfg.PolicyMode == "" {
		def := defaultSyncHostConfig()
		cfg.PolicyMode = def.PolicyMode
	}
	if cfg.ThresholdBytes <= 0 {
		cfg.ThresholdBytes = syncengine.Threshold2MB
	}
	if strings.TrimSpace(cfg.DeviceID) == "" {
		cfg.DeviceID = defaultSyncDeviceID()
	}
	cfg.Password = a.loadWebDAVPassword(syncSecretAccountKey(a.vault.Root))
	return cfg, nil
}

// saveSyncConfig atomically persists non-secret sync config to `<Vault>/.asterlab/sync_config.json`
// and stores cfg.Password in `mygo.Secrets` (never serialized to JSON).
func (a *app) saveSyncConfig(cfg syncHostConfig) error {
	cfgPath, err := a.syncConfigPath()
	if err != nil {
		return err
	}
	if cfg.PolicyMode == "" {
		cfg.PolicyMode = defaultSyncHostConfig().PolicyMode
	}
	if cfg.ThresholdBytes <= 0 {
		cfg.ThresholdBytes = syncengine.Threshold2MB
	}
	if strings.TrimSpace(cfg.DeviceID) == "" {
		cfg.DeviceID = defaultSyncDeviceID()
	}
	if err := a.storeWebDAVPassword(syncSecretAccountKey(a.vault.Root), cfg.Password); err != nil {
		return err
	}
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.Username = strings.TrimSpace(cfg.Username)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return asterdoc.WriteFileAtomic(cfgPath, data)
}

func (a *app) newSyncClient(cfg syncHostConfig) (*syncengine.WebDAVClient, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("sync: WebDAV URL is required")
	}
	return syncengine.NewWebDAVClient(syncengine.WebDAVConfig{
		BaseURL:           cfg.BaseURL,
		Username:          cfg.Username,
		Password:          cfg.Password,
		ForceNutstoreMode: cfg.ForceNutstoreMode,
		HTTPClient:        a.syncHTTPClient,
	})
}

func (a *app) newSyncEngine(cfg syncHostConfig) (*syncengine.Engine, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return nil, errors.New("sync: no active vault")
	}
	client, err := a.newSyncClient(cfg)
	if err != nil {
		return nil, err
	}
	pol := cfg.downloadPolicy()
	if a.syncPolicyOverride != nil {
		pol = *a.syncPolicyOverride
	}
	return syncengine.NewEngine(a.vault.Root, client, pol, cfg.DeviceID)
}

// testSyncConnection verifies WebDAV connectivity and credentials via a bounded Depth-0 PROPFIND.
func (a *app) testSyncConnection(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := a.loadSyncConfig()
	if err != nil {
		return err
	}
	client, err := a.newSyncClient(cfg)
	if err != nil {
		return err
	}
	_, err = client.Propfind(ctx, "", "0")
	return err
}

// runSyncNow executes one full manifest-driven sync pass against the configured WebDAV endpoint.
func (a *app) runSyncNow(ctx context.Context) (syncengine.SyncReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := a.loadSyncConfig()
	if err != nil {
		return syncengine.SyncReport{}, err
	}
	if !cfg.Enabled && strings.TrimSpace(cfg.BaseURL) == "" {
		return syncengine.SyncReport{}, errors.New("sync: synchronization is not configured")
	}
	eng, err := a.newSyncEngine(cfg)
	if err != nil {
		return syncengine.SyncReport{}, err
	}
	report, syncErr := eng.Sync(ctx)
	cfg.LastSyncAt = time.Now().UTC().Format(time.RFC3339)
	if syncErr != nil {
		cfg.LastError = syncErr.Error()
		_ = a.saveSyncConfig(cfg)
		return report, syncErr
	}
	cfg.LastError = ""
	cfg.LastVaultRevision = report.VaultRevision
	_ = a.saveSyncConfig(cfg)

	if len(report.DownloadedOriginals) > 0 || len(report.DeletedLocal) > 0 || report.MergedPantry || len(report.Conflicts) > 0 {
		if a.docStore != nil {
			a.docStore.ReloadAllExternalChanged()
		}
		if report.MergedPantry && a.tools != nil {
			a.tools.ReloadPantry()
		}
		a.refreshVaultTreeAndCalendar()
	}
	return report, nil
}

// clearLocalOriginalsCache removes synced lossless local originals while keeping
// preview sidecars intact to free mobile storage without deleting remote data.
func (a *app) clearLocalOriginalsCache() (syncengine.CacheClearResult, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return syncengine.CacheClearResult{}, errors.New("sync: no active vault")
	}
	cfg, err := a.loadSyncConfig()
	if err != nil {
		cfg = defaultSyncHostConfig()
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		// ClearLocalOriginalsCache operates purely on local `.asterlab/sync-state.json` and `assets/`.
		baseURL = "http://127.0.0.1/offline"
	}
	client, err := syncengine.NewWebDAVClient(syncengine.WebDAVConfig{
		BaseURL:    baseURL,
		HTTPClient: a.syncHTTPClient,
	})
	if err != nil {
		return syncengine.CacheClearResult{}, err
	}
	eng, err := syncengine.NewEngine(a.vault.Root, client, cfg.downloadPolicy(), cfg.DeviceID)
	if err != nil {
		return syncengine.CacheClearResult{}, err
	}
	return eng.ClearLocalOriginalsCache()
}

// fetchAssetOriginalOnDemand downloads and SHA-256-verifies a deferred lossless asset
// original into `<Vault>/<relPath>`.
func (a *app) fetchAssetOriginalOnDemand(ctx context.Context, relPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := a.loadSyncConfig()
	if err != nil {
		return err
	}
	eng, err := a.newSyncEngine(cfg)
	if err != nil {
		return err
	}
	if err := eng.FetchOriginalOnDemand(ctx, relPath); err != nil {
		return err
	}
	a.refreshVaultTreeAndCalendar()
	return nil
}

// resolveSyncConflict resolves a `.sync-conflict-...` file created during concurrent edits:
//   - "local" / "keep_local": keep the current local file and discard the conflict copy.
//   - "remote" / "keep_remote": snapshot the current local file, promote the conflict copy over it, and remove the conflict copy.
//   - "both" / "keep_both": rename the conflict copy to a regular non-conflict `.asterml` file so both are kept.
func (a *app) resolveSyncConflict(relPath, conflictCopyPath, keepSide string) error {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return errors.New("sync: no active vault")
	}
	normTarget, err := syncengine.NormalizeVaultPath(relPath)
	if err != nil {
		return err
	}
	normConflict, err := syncengine.NormalizeVaultPath(conflictCopyPath)
	if err != nil {
		return err
	}
	targetAbs := filepath.Join(a.vault.Root, filepath.FromSlash(normTarget))
	conflictAbs := filepath.Join(a.vault.Root, filepath.FromSlash(normConflict))

	switch strings.ToLower(strings.TrimSpace(keepSide)) {
	case "local", "keep_local":
		if err := os.Remove(conflictAbs); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	case "remote", "keep_remote", "use_remote":
		remoteData, err := os.ReadFile(conflictAbs)
		if err != nil {
			return err
		}
		_, _, _ = a.snapshotDocFile(normTarget, "pre-conflict-resolve", true)
		if err := asterdoc.WriteFileAtomic(targetAbs, remoteData); err != nil {
			return err
		}
		if err := os.Remove(conflictAbs); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if a.docStore != nil {
			a.docStore.Evict(normTarget)
		}
		a.resetLiveDraftCachesForFile(normTarget)
	case "open_copy":
		a.openVaultFileInActivePane(normConflict, filepath.Base(normConflict))
		return nil
	case "both", "keep_both":
		ext := filepath.Ext(normTarget)
		stem := strings.TrimSuffix(normTarget, ext)
		var destAbs string
		for n := 1; ; n++ {
			candRel := fmt.Sprintf("%s (冲突副本 %d)%s", stem, n, ext)
			candAbs := filepath.Join(a.vault.Root, filepath.FromSlash(candRel))
			if _, stErr := os.Stat(candAbs); errors.Is(stErr, os.ErrNotExist) {
				destAbs = candAbs
				break
			}
		}
		if err := os.Rename(conflictAbs, destAbs); err != nil {
			return err
		}
	default:
		return fmt.Errorf("sync: unsupported conflict resolution %q", keepSide)
	}

	if a.vaultIndex != nil {
		if raw, rErr := os.ReadFile(targetAbs); rErr == nil {
			a.vaultIndex.UpsertSource(normTarget, string(raw))
		}
	}
	a.refreshVaultTreeAndCalendar()
	return nil
}

// generateAssetPreviewSidecar builds a compact JPEG preview sidecar inside
// `<Vault>/.asterlab/previews/<sha256>.jpg` for a Vault-relative raster asset
// so mobile clients and sync previews stay lightweight while keeping the lossless original.
func (a *app) generateAssetPreviewSidecar(relAssetPath string) (string, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return "", errors.New("sync: no active vault")
	}
	abs := a.vault.AbsPath(relAssetPath)
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	prevBytes, _, ok, err := syncengine.GeneratePreviewJPEG(data, syncengine.DefaultPreviewMaxDim, syncengine.DefaultPreviewQuality)
	if err != nil || !ok {
		return "", err
	}
	sha := syncengine.SHA256Bytes(data)
	prevRel := syncengine.CanonicalPreviewPath(sha)
	prevAbs := filepath.Join(a.vault.Root, filepath.FromSlash(prevRel))
	if err := asterdoc.WriteFileAtomic(prevAbs, prevBytes); err != nil {
		return "", err
	}
	return prevRel, nil
}

// hydrateSyncShellState pushes persisted sync settings, cache usage, and conflict items into `a.shell`.
func (a *app) hydrateSyncShellState() {
	if a == nil || a.shell == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return
	}
	cfg, err := a.loadSyncConfig()
	if err != nil {
		cfg = defaultSyncHostConfig()
	}
	thMB := int(cfg.ThresholdBytes >> 20)
	switch thMB {
	case 1, 2, 5, 10:
	default:
		thMB = 2
	}
	a.shell.SyncSettings = shell.SyncSettings{
		Enabled:           cfg.Enabled,
		Endpoint:          cfg.BaseURL,
		Username:          cfg.Username,
		Password:          cfg.Password,
		RemotePath:        "/asterlab",
		AutoIntervalMin:   15,
		ImagePolicy:       shell.NormalizeImagePolicy(string(cfg.PolicyMode)),
		ImageThresholdMB:  thMB,
		WiFiOnlyOriginals: cfg.WiFiOnlyForOriginals,
	}

	var cacheMB float64
	if client, cErr := syncengine.NewWebDAVClient(syncengine.WebDAVConfig{BaseURL: "http://127.0.0.1/offline"}); cErr == nil {
		if eng, eErr := syncengine.NewEngine(a.vault.Root, client, cfg.downloadPolicy(), cfg.DeviceID); eErr == nil {
			if usedBytes, _, uErr := eng.LocalOriginalsCacheUsage(); uErr == nil {
				cacheMB = float64(usedBytes) / (1024 * 1024)
			}
		}
	}

	conflicts := a.scanVaultSyncConflicts()
	a.shell.SyncConflicts = conflicts

	status := "idle"
	if cfg.LastError != "" {
		status = "error"
	} else if len(conflicts) > 0 {
		status = "conflict"
	} else if cfg.LastSyncAt != "" {
		status = "synced"
	}
	lastSyncText := "从未同步"
	if cfg.LastSyncAt != "" {
		if tParsed, pErr := time.Parse(time.RFC3339, cfg.LastSyncAt); pErr == nil {
			lastSyncText = tParsed.Local().Format("01-02 15:04")
		} else {
			lastSyncText = cfg.LastSyncAt
		}
	}
	a.shell.SyncState = shell.SyncStateSummary{
		Status:                status,
		LastSyncText:          lastSyncText,
		LocalOriginalsCacheMB: cacheMB,
		ErrorText:             cfg.LastError,
	}
}

func (a *app) scanVaultSyncConflicts() []shell.SyncConflictItem {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return nil
	}
	var out []shell.SyncConflictItem
	_ = filepath.WalkDir(a.vault.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".asterlab" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rErr := filepath.Rel(a.vault.Root, path)
		if rErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !syncengine.IsConflictCopyPath(rel) {
			return nil
		}
		baseRel := rel
		if idx := strings.Index(baseRel, ".sync-conflict-"); idx > 0 {
			ext := filepath.Ext(baseRel)
			baseRel = baseRel[:idx] + ext
		}
		det := ""
		if info, stErr := d.Info(); stErr == nil {
			det = info.ModTime().Format("2006-01-02 15:04")
		}
		out = append(out, shell.SyncConflictItem{
			RelPath:      baseRel,
			ConflictPath: rel,
			DetectedAt:   det,
		})
		return nil
	})
	return out
}

// wireSyncAndBackupShellCallbacks connects Shell's M2 Sync, Conflict Resolution,
// Originals Cache, and Full-Vault Zip Backup/Restore actions to `sync_host` & `recovery_host`.
func (a *app) wireSyncAndBackupShellCallbacks() {
	if a == nil || a.shell == nil {
		return
	}
	applySettingsFromShell := func() error {
		cur, _ := a.loadSyncConfig()
		ss := a.shell.SyncSettings
		pol := ss.ToDownloadPolicy(isMobileRuntime())
		cur.Enabled = ss.Enabled
		cur.BaseURL = strings.TrimSpace(ss.Endpoint)
		cur.Username = strings.TrimSpace(ss.Username)
		cur.Password = ss.Password
		cur.PolicyMode = pol.Mode
		cur.ThresholdBytes = pol.ThresholdBytes
		cur.WiFiOnlyForOriginals = pol.WiFiOnlyForOriginals
		return a.saveSyncConfig(cur)
	}

	a.shell.OnSaveSyncSettings = func() {
		if err := applySettingsFromShell(); err != nil {
			a.shell.SyncState.Status = "error"
			a.shell.SyncState.ErrorText = err.Error()
			return
		}
		a.hydrateSyncShellState()
	}
	a.shell.OnTestSyncConnection = func() {
		if err := applySettingsFromShell(); err != nil {
			a.shell.SyncState.Status = "error"
			a.shell.SyncState.ErrorText = err.Error()
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := a.testSyncConnection(ctx); err != nil {
			a.shell.SyncState.Status = "error"
			a.shell.SyncState.ErrorText = err.Error()
			return
		}
		a.shell.SyncState.ErrorText = ""
		if a.shell.SyncState.Status == "error" || a.shell.SyncState.Status == "" {
			a.shell.SyncState.Status = "idle"
		}
	}
	a.shell.OnTriggerSyncNow = func() {
		_ = applySettingsFromShell()
		a.shell.SyncState.Status = "syncing"
		a.shell.SyncState.ErrorText = ""
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		report, err := a.runSyncNow(ctx)
		a.hydrateSyncShellState()
		if err != nil {
			a.shell.SyncState.Status = "error"
			a.shell.SyncState.ErrorText = err.Error()
		} else if len(report.Conflicts) > 0 {
			a.shell.SyncState.Status = "conflict"
		} else {
			a.shell.SyncState.Status = "synced"
		}
	}
	a.shell.OnClearOriginalsCache = func() {
		if _, err := a.clearLocalOriginalsCache(); err != nil {
			a.shell.SyncState.ErrorText = err.Error()
			return
		}
		a.hydrateSyncShellState()
	}
	a.shell.OnResolveSyncConflict = func(relPath, conflictPath, choice string) {
		if err := a.resolveSyncConflict(relPath, conflictPath, choice); err != nil {
			a.shell.SyncState.ErrorText = err.Error()
			return
		}
		a.hydrateSyncShellState()
	}
	a.shell.OnVaultBackupZip = func() {
		dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Title:       "导出整库 .zip 备份",
			DefaultPath: fmt.Sprintf("AsterLab-Backup-%s.zip", time.Now().Format("20060102-1504")),
		})
		if err != nil || strings.TrimSpace(dest) == "" {
			return
		}
		if _, err := a.backupVaultToZip(dest); err != nil {
			a.syncErr = fmt.Sprintf("整库备份失败：%v", err)
		}
	}
	a.shell.OnVaultRestoreZip = func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title: "选择整库 .zip 备份文件恢复",
		})
		if err != nil || len(paths) == 0 || strings.TrimSpace(paths[0]) == "" {
			return
		}
		if _, err := a.restoreVaultFromZip(paths[0]); err != nil {
			a.syncErr = fmt.Sprintf("整库恢复失败：%v", err)
			return
		}
		a.hydrateSyncShellState()
		a.wireSyncAndBackupShellCallbacks()
	}
	a.hydrateSyncShellState()
}
