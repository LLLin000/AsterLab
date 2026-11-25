package crashguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoverAndWriteCrashLogSanitizesSecrets(t *testing.T) {
	logDir := t.TempDir()

	// Normal non-panicking defer returns (false, "").
	func() {
		rec, p := RecoverAndWriteCrashLog(logDir)
		if rec || p != "" {
			t.Fatalf("expected no recovery without panic, got rec=%v path=%q", rec, p)
		}
	}()

	runPanickingBody(logDir)

	logPath := filepath.Join(logDir, CrashLogFileName)
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read crash.log: %v", err)
	}
	text := string(raw)

	// Must contain safe diagnostic context and stack trace.
	if !strings.Contains(text, "AsterLab Crash Report") || !strings.Contains(text, "runPanickingBody") {
		t.Fatalf("crash.log missing expected panic context:\n%s", text)
	}
	// Must NEVER leak any secret/token/password/credential.
	for _, secret := range []string{
		"super-secret-bearer-token-12345",
		"my-nutstore-pass",
		"kandev_tok_987654",
		"beta_key_xyz",
	} {
		if strings.Contains(text, secret) {
			t.Fatalf("crash.log leaked secret %q:\n%s", secret, text)
		}
	}
	if !strings.Contains(text, "<redacted>") {
		t.Fatalf("expected <redacted> markers in sanitized crash.log:\n%s", text)
	}
}

func runPanickingBody(logDir string) {
	defer RecoverAndWriteCrashLog(logDir, map[string]string{
		"endpoint":          "https://lin_lab:my-nutstore-pass@dav.jianguoyun.com/dav",
		"authHeader":        "Bearer super-secret-bearer-token-12345",
		"BETA_FEEDBACK_KEY": "beta_key_xyz",
	})
	panic("unexpected nil plate KANDEV_API_TOKEN=kandev_tok_987654")
}

func TestEmergencyDraftAtomicSaveLoadListAndTamperReject(t *testing.T) {
	baseDir := t.TempDir()
	rel := "2026/western_blot.asterml"
	content := "---\ntitle: WB\n---\n# Unsaved step notes\n"

	saved, err := SaveEmergencyDraft(baseDir, rel, content, 42, "v1.0.0")
	if err != nil {
		t.Fatalf("SaveEmergencyDraft failed: %v", err)
	}
	if saved.SHA256 == "" || saved.CursorOffset != 42 {
		t.Fatalf("unexpected saved draft: %+v", saved)
	}

	loaded, ok, err := LoadEmergencyDraft(baseDir, rel)
	if err != nil || !ok {
		t.Fatalf("LoadEmergencyDraft failed: ok=%v err=%v", ok, err)
	}
	if loaded.Content != content || loaded.CursorOffset != 42 {
		t.Fatalf("loaded draft mismatch: %+v", loaded)
	}

	list, err := ListEmergencyDrafts(baseDir)
	if err != nil || len(list) != 1 || list[0].DocRelPath != rel {
		t.Fatalf("ListEmergencyDrafts mismatch: list=%+v err=%v", list, err)
	}

	// Tamper draft file -> LoadEmergencyDraft must reject SHA-256 mismatch.
	draftPath := draftFilePath(baseDir, rel)
	raw, _ := os.ReadFile(draftPath)
	tampered := strings.Replace(string(raw), "Unsaved step notes", "CORRUPTED", 1)
	_ = os.WriteFile(draftPath, []byte(tampered), 0o644)
	if _, ok, err := LoadEmergencyDraft(baseDir, rel); err == nil || ok {
		t.Fatalf("expected LoadEmergencyDraft to reject tampered draft")
	}

	if err := ClearEmergencyDraft(baseDir, rel); err != nil {
		t.Fatalf("ClearEmergencyDraft failed: %v", err)
	}
	if _, ok, err := LoadEmergencyDraft(baseDir, rel); err != nil || ok {
		t.Fatalf("expected draft cleared, got ok=%v err=%v", ok, err)
	}
}
