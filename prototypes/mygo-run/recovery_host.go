package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/crashguard"
	"asterlab-mygo-prototype/internal/recovery"
	"asterlab-mygo-prototype/internal/updater"
	"asterlab-mygo-prototype/internal/vault"
	"github.com/egoist/mygo/ui/editor"
)

func (a *app) historyManager() *recovery.HistoryManager {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return nil
	}
	root := filepath.Clean(a.vault.Root)
	if a.recoveryHistory == nil || filepath.Clean(a.recoveryHistory.VaultRoot) != root {
		a.recoveryHistory = recovery.NewHistoryManager(root, recovery.DefaultSnapshotPolicy())
	}
	return a.recoveryHistory
}

func (a *app) crashLogDir() string {
	if a != nil && strings.TrimSpace(a.crashGuardLogDir) != "" {
		return a.crashGuardLogDir
	}
	if dir := workspaceDataDir(); strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "logs")
	}
	if a != nil && a.vault != nil && strings.TrimSpace(a.vault.Root) != "" {
		return filepath.Join(a.vault.Root, ".asterlab", "cache")
	}
	return ""
}

func (a *app) emergencyDraftBaseDir() string {
	if a != nil && a.vault != nil && strings.TrimSpace(a.vault.Root) != "" {
		return a.vault.Root
	}
	return workspaceDataDir()
}

// installCrashGuard configures secret-sanitized crash logging via `internal/crashguard`.
func (a *app) installCrashGuard() {
	if a == nil {
		return
	}
	logDir := a.crashLogDir()
	if logDir != "" {
		_ = os.MkdirAll(logDir, 0o755)
		a.crashGuardLogDir = logDir
	}
	a.crashGuardInstalled = true
}

func (a *app) recordCrashGuardPanic(tag string, panicVal any) string {
	if panicVal == nil {
		return ""
	}
	logDir := a.crashLogDir()
	if logDir == "" {
		return ""
	}
	meta := map[string]string{
		"tag":     tag,
		"version": updater.Version,
	}
	if a != nil && a.vault != nil && a.vault.Root != "" {
		meta["vault"] = filepath.Base(a.vault.Root)
	}
	p, _ := crashguard.WriteCrashReport(logDir, panicVal, debug.Stack(), meta)
	return p
}

// stageEmergencyDraft atomically stages unsaved document edits via `crashguard.SaveEmergencyDraft`.
func (a *app) stageEmergencyDraft(relPath, content string, cursorOffset int) {
	if a == nil || strings.TrimSpace(relPath) == "" || content == "" {
		return
	}
	baseDir := a.emergencyDraftBaseDir()
	if baseDir == "" {
		return
	}
	_, _ = crashguard.SaveEmergencyDraft(baseDir, relPath, content, cursorOffset, updater.Version)
}

func (a *app) clearEmergencyDraft(relPath string) {
	if a == nil || strings.TrimSpace(relPath) == "" {
		return
	}
	if baseDir := a.emergencyDraftBaseDir(); baseDir != "" {
		_ = crashguard.ClearEmergencyDraft(baseDir, relPath)
	}
}

// snapshotDocFile records a SHA-256 deduplicated File Recovery snapshot via `internal/recovery`.
// Returns (entry, created, err) where created is false when throttled or identical.
func (a *app) snapshotDocFile(relPath, reason string, force bool) (recovery.SnapshotEntry, bool, error) {
	hm := a.historyManager()
	if hm == nil {
		return recovery.SnapshotEntry{}, false, errors.New("recovery: no active vault")
	}
	entry, created, err := hm.SnapshotFile(relPath, reason, force)
	if err == nil && created && a.shell != nil && a.shell.CurrentDocRelPath == relPath {
		a.syncRecoverySnapshotsToShell(relPath)
	}
	return entry, created, err
}

// backupVaultToZip exports the active Vault into a checksum-verified `.zip` archive.
func (a *app) backupVaultToZip(destZipPath string, opts ...recovery.BackupOptions) (recovery.BackupResult, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return recovery.BackupResult{}, errors.New("recovery: no active vault")
	}
	if a.docStore != nil {
		for _, ld := range a.docStore.Docs {
			if ld != nil {
				_ = ld.Flush()
			}
		}
	}
	return recovery.BackupVaultZip(a.vault.Root, destZipPath, opts...)
}

// restoreVaultFromZip verifies and atomically restores a Vault backup `.zip` with rollback safety,
// then reloads the live workspace state cleanly.
func (a *app) restoreVaultFromZip(zipPath string) (recovery.RestoreResult, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return recovery.RestoreResult{}, errors.New("recovery: no active vault")
	}
	root := a.vault.Root
	res, err := recovery.RestoreVaultZip(zipPath, root)
	if err != nil {
		return res, err
	}
	a.liveViews = map[string]*asterdoc.View{}
	a.liveEditors = map[string]*editor.State{}
	if a.liveEditorSynced != nil {
		clear(a.liveEditorSynced)
	}
	if a.liveTitleDrafts != nil {
		clear(a.liveTitleDrafts)
	}
	if a.liveDateDrafts != nil {
		clear(a.liveDateDrafts)
	}
	a.recoveryHistory = nil
	if initErr := a.initVaultWorkspace(root); initErr != nil {
		return res, initErr
	}
	a.wireRecoveryHost()
	return res, nil
}

// checkAndRecoverEmergencyDrafts inspects staged `crashguard` emergency drafts on startup,
// discards stale identical drafts, and safely recovers any unsaved edits while taking a
// `pre-draft-recovery` snapshot first.
func (a *app) checkAndRecoverEmergencyDrafts() ([]crashguard.EmergencyDraft, error) {
	if a == nil || a.vault == nil || strings.TrimSpace(a.vault.Root) == "" {
		return nil, nil
	}
	baseDir := a.emergencyDraftBaseDir()
	drafts, err := crashguard.ListEmergencyDrafts(baseDir)
	if err != nil || len(drafts) == 0 {
		return nil, err
	}

	var recovered []crashguard.EmergencyDraft
	var titles []string
	for _, d := range drafts {
		rel := filepath.ToSlash(strings.TrimSpace(d.DocRelPath))
		if rel == "" {
			continue
		}
		abs := a.vault.AbsPath(rel)
		if diskBytes, rErr := os.ReadFile(abs); rErr == nil {
			sum := sha256.Sum256(diskBytes)
			if strings.EqualFold(hex.EncodeToString(sum[:]), d.SHA256) {
				_ = crashguard.ClearEmergencyDraft(baseDir, rel)
				continue
			}
			// Preserve current disk content before applying emergency draft.
			_, _, _ = a.snapshotDocFile(rel, "pre-draft-recovery", true)
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			continue
		}
		if err := asterdoc.WriteFileAtomic(abs, []byte(d.Content)); err != nil {
			continue
		}
		if a.docStore != nil {
			a.docStore.Evict(rel)
		}
		if a.vaultIndex != nil {
			a.vaultIndex.UpsertSource(rel, d.Content)
		}
		a.resetLiveDraftCachesForFile(rel)
		_ = crashguard.ClearEmergencyDraft(baseDir, rel)
		recovered = append(recovered, d)
		titles = append(titles, filepath.Base(rel))
	}

	if len(recovered) > 0 {
		a.refreshVaultTreeAndCalendar()
		if a.shell != nil {
			msg := fmt.Sprintf("检测到上次异常退出，已从应急草稿恢复 %d 份实验文档：%s", len(recovered), strings.Join(titles, "、"))
			if strings.TrimSpace(a.shell.CrashNotice) == "" {
				a.shell.CrashNotice = msg
			} else {
				a.shell.CrashNotice += "；" + msg
			}
		}
	}
	return recovered, nil
}

// wireRecoveryHost connects `a.shell.OnRestoreSnapshot` and `a.shell.OnVaultRestoreTrash`
// to `internal/recovery` with collision-safe trash restoration and history snapshot rollback.
func (a *app) wireRecoveryHost() {
	if a == nil || a.shell == nil {
		return
	}
	a.shell.OnVaultRestoreTrash = func(trashName string) {
		a.restoreVaultTrashSafely(trashName)
	}
	a.shell.OnRestoreSnapshot = func(relPath, snapshotID string) {
		a.restoreDocSnapshotSafely(relPath, snapshotID)
	}
}

func (a *app) restoreVaultTrashSafely(trashName string) {
	if a == nil || a.vault == nil || strings.TrimSpace(trashName) == "" {
		return
	}
	restoredRel, err := recovery.RestoreFromTrash(a.vault.Root, trashName, false)
	if err != nil {
		a.syncErr = fmt.Sprintf("恢复回收站文件失败：%v", err)
		return
	}
	a.refreshVaultTreeAndCalendar()
	if strings.HasSuffix(strings.ToLower(restoredRel), ".asterml") {
		a.openVaultFileInActivePane(restoredRel, "")
	}
}

func (a *app) restoreDocSnapshotSafely(relPath, snapshotID string) {
	if a == nil || a.vault == nil || relPath == "" || snapshotID == "" {
		return
	}
	hm := a.historyManager()
	if hm != nil {
		if err := hm.RestoreSnapshot(relPath, snapshotID); err == nil {
			if a.docStore != nil {
				a.docStore.Evict(relPath)
			}
			if a.vaultIndex != nil {
				if raw, rErr := os.ReadFile(a.vault.AbsPath(relPath)); rErr == nil {
					a.vaultIndex.UpsertSource(relPath, string(raw))
				}
			}
			a.resetLiveDraftCachesForFile(relPath)
			a.lastCompanionKey = ""
			a.refreshVaultTreeAndCalendar()
			if a.docStore != nil {
				if ld, oErr := a.docStore.Open(relPath); oErr == nil {
					a.syncActiveDocCompanionState(relPath, ld.Source, ld.Parsed)
				}
			}
			return
		}
	}
	// Fall back to legacy `.asterlab/snapshots/` if snapshotID belongs to an older vault snapshot.
	a.restoreSnapshotFromHistory(relPath, snapshotID)
}

func (a *app) syncRecoverySnapshotsToShell(relPath string) {
	if a == nil || a.shell == nil || a.vault == nil || relPath == "" {
		return
	}
	seen := map[string]bool{}
	var merged []vault.SnapshotMeta
	if hm := a.historyManager(); hm != nil {
		if list, err := hm.ListSnapshots(relPath); err == nil {
			for _, sn := range list {
				seen[sn.ID] = true
				merged = append(merged, vault.SnapshotMeta{
					ID:        sn.ID,
					RelPath:   sn.RelPath,
					CreatedAt: sn.CreatedAt,
					Size:      sn.Size,
					SHA256:    sn.SHA256,
				})
			}
		}
	}
	for _, sn := range a.vault.ListSnapshots(relPath) {
		if !seen[sn.ID] {
			merged = append(merged, sn)
		}
	}
	a.shell.CurrentSnapshots = merged
}
