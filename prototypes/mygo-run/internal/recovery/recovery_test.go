package recovery

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistorySnapshotsDedupRetentionAndRestore(t *testing.T) {
	vaultRoot := t.TempDir()
	rel := "2026/cck8.asterml"
	abs := filepath.Join(vaultRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	hm := NewHistoryManager(vaultRoot, SnapshotPolicy{
		MaxCopiesPerFile: 2,
		RetentionDays:    7,
		MinInterval:      2 * time.Minute,
	})
	hm.NowFn = func() time.Time { return now }

	v1 := []byte("---\ntitle: CCK8 v1\n---\n")
	if err := os.WriteFile(abs, v1, 0o644); err != nil {
		t.Fatal(err)
	}

	snap1, created, err := hm.SnapshotFile(rel, "auto", false)
	if err != nil || !created {
		t.Fatalf("first SnapshotFile failed: created=%v err=%v", created, err)
	}

	// Duplicate identical content must dedup even with force=true.
	snapDup, createdDup, err := hm.SnapshotFile(rel, "auto", true)
	if err != nil || createdDup || snapDup.ID != snap1.ID {
		t.Fatalf("expected SHA-256 dedup on identical content, got created=%v id=%s", createdDup, snapDup.ID)
	}

	// Modified content within MinInterval with force=false is throttled.
	v2 := []byte("---\ntitle: CCK8 v2\n---\n")
	_ = os.WriteFile(abs, v2, 0o644)
	if _, createdThrottled, err := hm.SnapshotFile(rel, "auto", false); err != nil || createdThrottled {
		t.Fatalf("expected MinInterval throttle when force=false, got created=%v err=%v", createdThrottled, err)
	}

	// Force snapshot v2, then revert content back to v1 and force snapshot v3 (shares v1 blob).
	now = now.Add(10 * time.Second)
	snap2, created2, err := hm.SnapshotFile(rel, "pre-freeze", true)
	if err != nil || !created2 {
		t.Fatalf("force snapshot v2 failed: %v", err)
	}

	// Current disk has v2; restore snap1 -> must create pre-restore snapshot of v2 and write v1 to disk.
	now = now.Add(10 * time.Second)
	if err := hm.RestoreSnapshot(rel, snap1.ID); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	gotDisk, _ := os.ReadFile(abs)
	if !bytes.Equal(gotDisk, v1) {
		t.Fatalf("RestoreSnapshot content mismatch: got %q want %q", gotDisk, v1)
	}

	// MaxCopiesPerFile=2 pruning: snapshot v3 forces retention pruning to keep 2 newest.
	now = now.Add(10 * time.Second)
	v3 := []byte("---\ntitle: CCK8 v3\n---\n")
	_ = os.WriteFile(abs, v3, 0o644)
	snap3, created3, err := hm.SnapshotFile(rel, "manual", true)
	if err != nil || !created3 {
		t.Fatalf("snapshot v3 failed: %v", err)
	}
	snaps, err := hm.ListSnapshots(rel)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 || snaps[0].ID != snap3.ID || snaps[1].ID != snap2.ID {
		t.Fatalf("expected 2 newest snapshots [%s, %s], got %+v", snap3.ID, snap2.ID, snaps)
	}

	// RetentionDays pruning: advance clock by 10 days -> snapshots older than 7 days pruned on Prune().
	now = now.AddDate(0, 0, 10)
	pruned, err := hm.Prune(rel)
	if err != nil || pruned != 2 {
		t.Fatalf("expected 2 expired snapshots pruned, got pruned=%d err=%v", pruned, err)
	}
	remaining, _ := hm.ListSnapshots(rel)
	if len(remaining) != 0 {
		t.Fatalf("expected 0 snapshots after expiry prune, got %+v", remaining)
	}

	// Integrity check failure when blob is tampered.
	now = now.Add(time.Minute)
	_ = os.WriteFile(abs, v1, 0o644)
	snapTamper, _, _ := hm.SnapshotFile(rel, "manual", true)
	blobPath := filepath.Join(hm.snapshotBucketDir(rel), snapTamper.SHA256+".blob")
	_ = os.WriteFile(blobPath, []byte("corrupted"), 0o644)
	if _, err := hm.ReadSnapshot(rel, snapTamper.ID); err == nil {
		t.Fatalf("expected ReadSnapshot to fail integrity check on corrupted blob")
	}
}

func TestBackupZipVerifyRestoreAndChaosRollback(t *testing.T) {
	vaultRoot := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(filepath.Join(vaultRoot, "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vaultRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vaultRoot, ".asterlab"), 0o755); err != nil {
		t.Fatal(err)
	}

	origFiles := map[string][]byte{
		"2026/exp1.asterml":      []byte("---\ntitle: Exp1\n---\n# Original\n"),
		"assets/gel.png":         []byte{0x89, 'P', 'N', 'G', 1, 2, 3, 4, 5},
		".asterlab/pantry.jsonl": []byte(`{"type":"schema","version":1}` + "\n"),
		".asterlab/config.json":  []byte(`{"themeSource":"light"}`),
	}
	for rel, data := range origFiles {
		if err := os.WriteFile(filepath.Join(vaultRoot, filepath.FromSlash(rel)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	zipPath := filepath.Join(t.TempDir(), "asterlab-backup.zip")
	bkRes, err := BackupVaultZip(vaultRoot, zipPath)
	if err != nil {
		t.Fatalf("BackupVaultZip failed: %v", err)
	}
	if bkRes.FileCount != len(origFiles) {
		t.Fatalf("BackupResult FileCount = %d, want %d", bkRes.FileCount, len(origFiles))
	}

	manifest, err := VerifyBackupZip(zipPath)
	if err != nil {
		t.Fatalf("VerifyBackupZip failed: %v", err)
	}
	if manifest.FileCount != len(origFiles) {
		t.Fatalf("manifest FileCount = %d, want %d", manifest.FileCount, len(origFiles))
	}

	// Chaos 1: Corrupt zip bytes -> VerifyBackupZip and RestoreVaultZip must reject and leave target untouched.
	corruptZip := filepath.Join(t.TempDir(), "corrupt.zip")
	zipBytes, _ := os.ReadFile(zipPath)
	if len(zipBytes) > 40 {
		zipBytes[len(zipBytes)/2] ^= 0xff
		zipBytes[len(zipBytes)-10] ^= 0xff
	}
	_ = os.WriteFile(corruptZip, zipBytes, 0o644)

	liveSentinel := []byte("DO NOT DESTROY LIVE DATA")
	sentinelPath := filepath.Join(vaultRoot, "2026/exp1.asterml")
	_ = os.WriteFile(sentinelPath, liveSentinel, 0o644)

	if _, err := VerifyBackupZip(corruptZip); err == nil {
		t.Fatalf("expected VerifyBackupZip to reject corrupt archive")
	}
	if _, err := RestoreVaultZip(corruptZip, vaultRoot); err == nil {
		t.Fatalf("expected RestoreVaultZip to reject corrupt archive")
	}
	stillLive, _ := os.ReadFile(sentinelPath)
	if !bytes.Equal(stillLive, liveSentinel) {
		t.Fatalf("corrupt zip restore modified live vault! got %q", stillLive)
	}

	// Chaos 2: Malicious zip-slip traversal entry -> rejected before extraction.
	zipSlipPath := filepath.Join(t.TempDir(), "zipslip.zip")
	writeRawZip(t, zipSlipPath, map[string][]byte{
		"../../evil.txt":               []byte("pwned"),
		".asterlab/backup-manifest.json": []byte(`{"schemaVersion":1,"fileCount":1,"checksums":{"../../evil.txt":"abc"}}`),
	})
	if _, err := RestoreVaultZip(zipSlipPath, vaultRoot); err == nil {
		t.Fatalf("expected zip-slip archive to be rejected")
	}

	// Chaos 3: Mid-restore failure after swap prep -> automatic rollback preserves liveSentinel!
	rbRes, err := restoreVaultZipWithHook(zipPath, vaultRoot, func(stageDir string) error {
		return errors.New("simulated disk/permission failure during restore commit")
	})
	if err == nil || !rbRes.RolledBack {
		t.Fatalf("expected rollback on mid-restore error, got res=%+v err=%v", rbRes, err)
	}
	afterRollback, _ := os.ReadFile(sentinelPath)
	if !bytes.Equal(afterRollback, liveSentinel) {
		t.Fatalf("rollback failed to preserve live vault! got %q want %q", afterRollback, liveSentinel)
	}

	// Clean full restore onto live vault restores original backed-up contents.
	res, err := RestoreVaultZip(zipPath, vaultRoot)
	if err != nil {
		t.Fatalf("RestoreVaultZip failed: %v", err)
	}
	if res.FileCount != len(origFiles) {
		t.Fatalf("RestoreResult FileCount = %d, want %d", res.FileCount, len(origFiles))
	}
	for rel, want := range origFiles {
		got, err := os.ReadFile(filepath.Join(vaultRoot, filepath.FromSlash(rel)))
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("restored file %s mismatch: err=%v", rel, err)
		}
	}
}

func TestTrashMoveListCollisionSafeRestoreAndPurge(t *testing.T) {
	vaultRoot := t.TempDir()
	rel := "2026/sample.asterml"
	abs := filepath.Join(vaultRoot, filepath.FromSlash(rel))
	_ = os.MkdirAll(filepath.Dir(abs), 0o755)
	_ = os.WriteFile(abs, []byte("trashed-content"), 0o644)

	entry, err := MoveToTrash(vaultRoot, rel)
	if err != nil {
		t.Fatalf("MoveToTrash failed: %v", err)
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatalf("expected original path removed after MoveToTrash")
	}

	list, err := ListTrash(vaultRoot)
	if err != nil || len(list) != 1 || list[0].TrashName != entry.TrashName {
		t.Fatalf("ListTrash mismatch: list=%+v err=%v", list, err)
	}

	// Create a new file at origRel -> collision-safe RestoreFromTrash(overwrite=false) preserves both!
	_ = os.WriteFile(abs, []byte("newer-content"), 0o644)
	restoredRel, err := RestoreFromTrash(vaultRoot, entry.TrashName, false)
	if err != nil {
		t.Fatalf("RestoreFromTrash failed: %v", err)
	}
	if restoredRel == rel {
		t.Fatalf("expected collision-safe restored path != %q", rel)
	}
	gotNewer, _ := os.ReadFile(abs)
	gotRestored, _ := os.ReadFile(filepath.Join(vaultRoot, filepath.FromSlash(restoredRel)))
	if string(gotNewer) != "newer-content" || string(gotRestored) != "trashed-content" {
		t.Fatalf("unexpected contents: newer=%q restored=%q", gotNewer, gotRestored)
	}

	// Move again and purge.
	entry2, err := MoveToTrash(vaultRoot, restoredRel)
	if err != nil {
		t.Fatal(err)
	}
	if err := PurgeTrashItem(vaultRoot, entry2.TrashName); err != nil {
		t.Fatal(err)
	}
	if rem, _ := ListTrash(vaultRoot); len(rem) != 0 {
		t.Fatalf("expected empty trash after PurgeTrashItem, got %+v", rem)
	}
}

func writeRawZip(t *testing.T, zipPath string, entries map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
