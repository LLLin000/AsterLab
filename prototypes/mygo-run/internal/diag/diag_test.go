package diag

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeTextAndBundlePrivacy(t *testing.T) {
	home, _ := os.UserHomeDir()
	raw := "failed reading " + home + `\secret-vault\exp.asterml with token ghp_1234567890abcdef123456 and C:\Users\Alice\vault`
	clean := SanitizeText(raw)
	if home != "" && strings.Contains(clean, home) {
		t.Fatalf("home dir leaked in sanitized text: %q", clean)
	}
	if strings.Contains(clean, "Alice") || strings.Contains(clean, "ghp_1234567890abcdef123456") {
		t.Fatalf("username or token leaked in sanitized text: %q", clean)
	}
	if !strings.Contains(clean, "<REDACTED_TOKEN>") {
		t.Fatalf("expected token redaction marker, got: %q", clean)
	}

	logDir := filepath.Join(t.TempDir(), "logs")
	dataDir := filepath.Join(t.TempDir(), "data")
	_ = os.MkdirAll(dataDir, 0o755)
	_ = os.WriteFile(filepath.Join(dataDir, "update-prefs.json"), []byte(`{"autoCheck":true}`), 0o644)
	_ = os.WriteFile(filepath.Join(dataDir, "secret.asterml"), []byte("CONFIDENTIAL SCIENCE DATA"), 0o644)

	if err := Init(logDir); err != nil {
		t.Fatalf("diag.Init: %v", err)
	}
	defer Close()

	crashPath := RecordPanic("test-worker", "nil pointer dereference at C:\\Users\\Alice\\exp.asterml", []byte("goroutine 1"))
	if crashPath == "" {
		t.Fatal("expected crash log file written")
	}
	pending := PendingCrash(logDir)
	if pending == nil || pending.Tag != "test-worker" || strings.Contains(pending.Summary, "Alice") {
		t.Fatalf("unexpected pending crash record: %+v", pending)
	}

	zipOut := filepath.Join(t.TempDir(), "diag-bundle.zip")
	if err := ExportBundle(zipOut, EnvironmentMeta{
		AppVersion: "0.1.4-test", BuildCommit: "abc1234", VaultStats: "3 实验", DataDir: dataDir,
	}); err != nil {
		t.Fatalf("ExportBundle: %v", err)
	}

	zr, err := zip.OpenReader(zipOut)
	if err != nil {
		t.Fatalf("open exported bundle: %v", err)
	}
	defer zr.Close()

	hasEnv, hasCrash := false, false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".asterml") {
			t.Fatalf("diagnostic bundle must never include .asterml file %q", f.Name)
		}
		if f.Name == "environment.json" {
			hasEnv = true
		}
		if strings.HasPrefix(f.Name, "crashes/crash-") {
			hasCrash = true
		}
	}
	if !hasEnv || !hasCrash {
		t.Fatalf("missing expected diagnostic files: hasEnv=%v hasCrash=%v", hasEnv, hasCrash)
	}

	DismissPendingCrash(logDir)
	if PendingCrash(logDir) != nil {
		t.Fatal("expected pending crash dismissed")
	}

	issueURL := BuildGitHubIssueURL("LLLin000/AsterLab", "程序异常", "", "在 C:\\Users\\Alice 打开实验崩溃", EnvironmentMeta{
		AppVersion: "0.1.4", BuildCommit: "deadbeef",
	}, true)
	if !strings.HasPrefix(issueURL, "https://github.com/LLLin000/AsterLab/issues/new?") || strings.Contains(issueURL, "Alice") {
		t.Fatalf("unexpected github issue URL: %q", issueURL)
	}
}
