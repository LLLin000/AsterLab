package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/asterdoc"
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
	case "remote", "keep_remote":
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
