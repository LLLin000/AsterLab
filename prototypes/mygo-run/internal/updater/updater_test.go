package updater

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0-unsigned", "v0.1.0", 0},
		{"0.1.0-dev", "v0.1.0", -1},
		{"v0.1.1", "0.1.0-unsigned", 1},
		{"v0.1.0-rc.2", "v0.1.0-rc.1", 1},
		{"v0.1.0", "v0.1.0-rc.2", 1},
		{"v0.2.0", "v0.1.9", 1},
	}
	for _, tc := range cases {
		if got := CompareVersions(tc.a, tc.b); got != tc.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCheckForUpdateAndAssetMatching(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]ReleaseInfo{
			{
				TagName:    "v0.2.0-rc.1",
				Name:       "v0.2.0-rc.1 Pre-release",
				Prerelease: true,
				HTMLURL:    "https://github.com/LLLin000/AsterLab/releases/tag/v0.2.0-rc.1",
				Body:       "Preview release",
				Assets: []ReleaseAsset{
					{Name: "AsterLab-MyGo-0.2.0-rc.1-win-x64.zip", BrowserDownloadURL: "https://example.com/rc.zip"},
					{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://example.com/SHA256SUMS.txt"},
				},
			},
			{
				TagName:    "v0.1.5",
				Name:       "v0.1.5 Stable",
				Prerelease: false,
				HTMLURL:    "https://github.com/LLLin000/AsterLab/releases/tag/v0.1.5",
				Assets: []ReleaseAsset{
					{Name: "AsterLab-MyGo-0.1.5-win-x64-setup.exe", BrowserDownloadURL: "https://example.com/setup.exe"},
				},
			},
		})
	}))
	defer srv.Close()

	prevBase := APIBaseURL
	APIBaseURL = srv.URL
	defer func() { APIBaseURL = prevBase }()

	stableRes, err := CheckForUpdate(context.Background(), "0.1.0-unsigned", false)
	if err != nil {
		t.Fatalf("CheckForUpdate stable: %v", err)
	}
	if !stableRes.HasUpdate || stableRes.Latest == nil || stableRes.Latest.TagName != "v0.1.5" {
		t.Fatalf("expected v0.1.5 stable update, got %+v", stableRes)
	}
	if stableRes.Latest.SetupURL == "" {
		t.Fatal("expected SetupURL matched for v0.1.5")
	}

	preRes, err := CheckForUpdate(context.Background(), "0.1.0-unsigned", true)
	if err != nil {
		t.Fatalf("CheckForUpdate prerelease: %v", err)
	}
	if !preRes.HasUpdate || preRes.Latest == nil || preRes.Latest.TagName != "v0.2.0-rc.1" {
		t.Fatalf("expected v0.2.0-rc.1 prerelease update, got %+v", preRes)
	}
	if preRes.Latest.ZipURL == "" || preRes.Latest.SumsURL == "" {
		t.Fatalf("expected ZipURL and SumsURL matched, got %+v", preRes.Latest)
	}

	sameRes, err := CheckForUpdate(context.Background(), "v0.2.0-rc.1", true)
	if err != nil {
		t.Fatalf("CheckForUpdate same version: %v", err)
	}
	if sameRes.HasUpdate {
		t.Fatalf("expected HasUpdate=false when already at %s", sameRes.Latest.TagName)
	}
}

func TestDownloadVerifyAndSafeExtractZip(t *testing.T) {
	tmp := t.TempDir()
	zipSrc := filepath.Join(tmp, "payload.zip")
	func() {
		f, err := os.Create(zipSrc)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		zw := zip.NewWriter(f)
		w, _ := zw.Create("AsterLab-MyGo-0.1.1-win-x64/mygo-run.exe")
		_, _ = w.Write([]byte("new-binary-bytes"))
		_ = zw.Close()
	}()
	zipBytes, err := os.ReadFile(zipSrc)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(zipBytes)
	goodHash := hex.EncodeToString(h[:])

	badSums := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pkg.zip":
			_, _ = w.Write(zipBytes)
		case "/SHA256SUMS.txt":
			sum := goodHash
			if badSums {
				sum = strings.Repeat("0", 64)
			}
			_, _ = w.Write([]byte(sum + "  AsterLab-MyGo-0.1.1-win-x64.zip (16 bytes)\n"))
		}
	}))
	defer srv.Close()

	downloaded, err := DownloadAndVerifyAsset(
		context.Background(),
		srv.URL+"/pkg.zip",
		srv.URL+"/SHA256SUMS.txt",
		"AsterLab-MyGo-0.1.1-win-x64.zip",
		filepath.Join(tmp, "stage-ok"),
	)
	if err != nil {
		t.Fatalf("DownloadAndVerifyAsset failed: %v", err)
	}
	unpacked := filepath.Join(tmp, "unpacked")
	if err := extractZipSafely(downloaded, unpacked); err != nil {
		t.Fatalf("extractZipSafely failed: %v", err)
	}
	gotExe, err := os.ReadFile(filepath.Join(unpacked, "mygo-run.exe"))
	if err != nil || string(gotExe) != "new-binary-bytes" {
		t.Fatalf("expected stripped top-folder mygo-run.exe, err=%v got=%q", err, string(gotExe))
	}

	badSums = true
	if _, err := DownloadAndVerifyAsset(
		context.Background(),
		srv.URL+"/pkg.zip",
		srv.URL+"/SHA256SUMS.txt",
		"AsterLab-MyGo-0.1.1-win-x64.zip",
		filepath.Join(tmp, "stage-bad"),
	); err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatalf("expected SHA256 mismatch error, got: %v", err)
	}

	if !canWriteDir(tmp) {
		t.Fatalf("expected temp dir to be writable for in-place update")
	}
}

func TestApplyDirectorySwapWithLockedExeAndCleanup(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "AsterLab MyGo")
	unpackedDir := filepath.Join(t.TempDir(), "unpacked")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(unpackedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	exeDst := filepath.Join(installDir, "mygo-run.exe")
	cliDst := filepath.Join(installDir, "asterlab.exe")
	if err := os.WriteFile(exeDst, []byte("MZ-old-v0.1.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cliDst, []byte("MZ-old-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unpackedDir, "mygo-run.exe"), []byte("MZ-new-v0.1.3"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unpackedDir, "asterlab.exe"), []byte("MZ-new-cli"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := applyDirectorySwap(unpackedDir, installDir); err != nil {
		t.Fatalf("applyDirectorySwap failed: %v", err)
	}

	gotExe, err := os.ReadFile(exeDst)
	if err != nil || string(gotExe) != "MZ-new-v0.1.3" {
		t.Fatalf("expected swapped exe bytes, err=%v got=%q", err, string(gotExe))
	}
	cleanupOldUpdatesDir(installDir)
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if strings.Contains(ent.Name(), ".old") || strings.Contains(ent.Name(), ".new.") {
			t.Fatalf("expected leftover temp file %q to be cleaned on startup", ent.Name())
		}
	}

	// Verify PE magic guard aborts before mutating installDir.
	badUnpacked := filepath.Join(t.TempDir(), "bad-unpacked")
	_ = os.MkdirAll(badUnpacked, 0o755)
	_ = os.WriteFile(filepath.Join(badUnpacked, "mygo-run.exe"), []byte("CORRUPT-TEXT"), 0o755)
	if _, err := applyDirectorySwap(badUnpacked, installDir); err == nil {
		t.Fatal("expected invalid PE header error")
	}
	stillGood, _ := os.ReadFile(exeDst)
	if string(stillGood) != "MZ-new-v0.1.3" {
		t.Fatalf("installDir mutated after failed PE validation: %q", string(stillGood))
	}
}
