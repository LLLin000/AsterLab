package devfeedback

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestExportSessionWithCrashLogAndUploadBundle(t *testing.T) {
	outDir := t.TempDir()
	sess := SessionMeta{
		SessionID:  "20261124-000100",
		TesterName: "小王",
		AppVersion: "0.1.10-beta",
		Commit:     "abc1234",
		OS:         "windows",
		Arch:       "amd64",
		CrashLog:   "panic: sample fatal crash\ngoroutine 1 [running]:\nmain.main()\n",
	}
	// Pure crash report (0 UI issues) must still export cleanly with logs/crash.log.
	_, zipPath, err := ExportSession(outDir, sess, nil, []string{"line1", "line2"})
	if err != nil {
		t.Fatalf("ExportSession with crash log failed: %v", err)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	foundCrash := false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "logs/crash.log") {
			foundCrash = true
		}
	}
	_ = zr.Close()
	if !foundCrash {
		t.Fatalf("expected logs/crash.log inside exported zip %s", zipPath)
	}

	// Verify UploadBundle sends X-Feedback-Key and multipart fields accurately.
	var gotKey, gotTester, gotCrash string
	var gotFileSize int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Feedback-Key")
		if gotKey != "test-write-only-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "unauthorized"})
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotTester = r.FormValue("tester")
		gotCrash = r.FormValue("has_crash")
		f, hdr, err := r.FormFile("bundle")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = f.Close()
		gotFileSize = hdr.Size
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":         true,
			"id":         "FB-20261124-000100",
			"repo_path":  "submissions/FB-20261124-000100-小王",
			"file_count": 4,
		})
	}))
	defer srv.Close()

	resp, err := UploadBundle(srv.URL, "test-write-only-key", zipPath, "小王", "0.1.10-beta", 0, true, "crash report")
	if err != nil {
		t.Fatalf("UploadBundle failed: %v", err)
	}
	if !resp.OK || resp.ID != "FB-20261124-000100" {
		t.Fatalf("unexpected upload response: %+v", resp)
	}
	if gotTester != "小王" || gotCrash != "true" || gotFileSize <= 0 {
		t.Fatalf("unexpected multipart fields: tester=%q crash=%q size=%d", gotTester, gotCrash, gotFileSize)
	}

	// Invalid key must fail cleanly.
	if _, err := UploadBundle(srv.URL, "wrong-key", zipPath, "小王", "0.1.10-beta", 0, true, ""); err == nil {
		t.Fatal("expected error for invalid upload key")
	}
	_ = os.Remove(zipPath)
}
