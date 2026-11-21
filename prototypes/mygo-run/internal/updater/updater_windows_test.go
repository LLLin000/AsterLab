//go:build windows

package updater

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWindowsInPlaceSwapWhileExecutableMapped(t *testing.T) {
	installDir := filepath.Join(t.TempDir(), "AsterLab MyGo")
	unpackedDir := filepath.Join(t.TempDir(), "unpacked")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(unpackedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	exeDst := filepath.Join(installDir, "mygo-run.exe")
	if err := os.WriteFile(exeDst, []byte("MZ-old-running"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unpackedDir, "mygo-run.exe"), []byte("MZ-new-swapped"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Windows PE loader holds an image mapping handle with FILE_SHARE_READ | FILE_SHARE_DELETE,
	// preventing overwrite while permitting Rename -> .old.
	p16, err := syscall.UTF16PtrFromString(exeDst)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(
		p16,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile lock simulation: %v", err)
	}

	swapErr := applyDirectorySwap(unpackedDir, installDir)
	_ = syscall.CloseHandle(h)
	if swapErr != nil {
		t.Fatalf("applyDirectorySwap while PE-mapped failed: %v", swapErr)
	}

	got, err := os.ReadFile(exeDst)
	if err != nil || string(got) != "MZ-new-swapped" {
		t.Fatalf("expected new binary on disk, err=%v got=%q", err, string(got))
	}
	cleanupOldUpdatesDir(installDir)
}
