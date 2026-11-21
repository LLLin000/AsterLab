package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartupWatchdogAutoRollbackAndHealthyClear(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	exeDir := filepath.Join(t.TempDir(), "bin")
	_ = os.MkdirAll(exeDir, 0o755)
	_ = os.MkdirAll(dataDir, 0o755)

	// Simulate an update from v0.1.3 -> v0.1.4 leaving v0.1.3 under .old.<stamp>.
	stamp := "179999999"
	_ = os.WriteFile(filepath.Join(exeDir, "mygo-run.exe"), []byte("MZ-broken-0.1.4"), 0o755)
	_ = os.WriteFile(filepath.Join(exeDir, "mygo-run.exe.old."+stamp), []byte("MZ-stable-0.1.3"), 0o755)
	_ = os.WriteFile(filepath.Join(dataDir, "pantry-ledger.json"), []byte(`{"items":[]}`), 0o644)

	BackupLocalData(dataDir, "v0.1.4")
	if matches, _ := filepath.Glob(filepath.Join(dataDir, "backups", "pre-0.1.4-*", "pantry-ledger.json")); len(matches) != 1 {
		t.Fatalf("expected pre-update data backup created, got %v", matches)
	}

	SaveUpdateState(dataDir, UpdateState{
		PendingVersion:  "v0.1.4",
		PreviousVersion: "v0.1.3",
		BackupStamp:     stamp,
		BootAttempts:    0,
	})

	// Boot 1 & Boot 2 increment counter without deleting .old backup yet.
	if msg := EvaluateStartupWatchdog(dataDir, "0.1.4", exeDir); msg != "" {
		t.Fatalf("boot 1 unexpected rollback: %s", msg)
	}
	if msg := EvaluateStartupWatchdog(dataDir, "0.1.4", exeDir); msg != "" {
		t.Fatalf("boot 2 unexpected rollback: %s", msg)
	}
	if _, err := os.Stat(filepath.Join(exeDir, "mygo-run.exe.old."+stamp)); err != nil {
		t.Fatal("expected .old backup preserved until healthy boot")
	}

	// Boot 3 (BootAttempts >= 2) triggers automatic rollback to 0.1.3 and skips v0.1.4.
	notice := EvaluateStartupWatchdog(dataDir, "0.1.4", exeDir)
	if !strings.Contains(notice, "自动回滚") {
		t.Fatalf("expected automatic rollback notice on repeated startup crash, got %q", notice)
	}
	gotExe, _ := os.ReadFile(filepath.Join(exeDir, "mygo-run.exe"))
	if string(gotExe) != "MZ-stable-0.1.3" {
		t.Fatalf("expected restored 0.1.3 binary, got %q", string(gotExe))
	}
	st := LoadUpdateState(dataDir)
	if st.SkippedVersion != "v0.1.4" || st.PendingVersion != "" {
		t.Fatalf("unexpected post-rollback state: %+v", st)
	}

	// Verify healthy boot clears pending state and sweeps .old files.
	_ = os.WriteFile(filepath.Join(exeDir, "mygo-run.exe.old.999"), []byte("MZ-old"), 0o755)
	SaveUpdateState(dataDir, UpdateState{PendingVersion: "v0.1.5", BackupStamp: "999", BootAttempts: 1})
	MarkBootHealthy(dataDir, exeDir)
	if _, err := os.Stat(filepath.Join(exeDir, "mygo-run.exe.old.999")); !os.IsNotExist(err) {
		t.Fatal("expected .old backup removed after MarkBootHealthy")
	}
}
