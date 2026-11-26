package vault

import (
	"path/filepath"
	"testing"
)

func TestDefaultVaultDirForOS(t *testing.T) {
	home := filepath.FromSlash("/var/mobile/Containers/Data/Application/UUID")
	gotIOS := defaultVaultDirForOS("ios", "", "", home, "")
	wantIOS := filepath.Join(home, "Documents", "AsterLab")
	if gotIOS != wantIOS {
		t.Fatalf("iOS vault path mismatch: got %q want %q", gotIOS, wantIOS)
	}

	gotAndroid := defaultVaultDirForOS("android", "", "", "/data/user/0/party.asterlab.app/files", "")
	wantAndroid := filepath.Join("/data/user/0/party.asterlab.app/files", "AsterLab")
	if gotAndroid != wantAndroid {
		t.Fatalf("Android vault path mismatch: got %q want %q", gotAndroid, wantAndroid)
	}

	gotWin := defaultVaultDirForOS("windows", "", `C:\Users\Lin\AppData\Local`, `C:\Users\Lin`, "")
	wantWin := filepath.Join(`C:\Users\Lin\AppData\Local`, "AsterLab", "vault")
	if gotWin != wantWin {
		t.Fatalf("Windows vault path mismatch: got %q want %q", gotWin, wantWin)
	}

	gotOverride := defaultVaultDirForOS("ios", "/tmp/custom-vault", "", home, "")
	if gotOverride != filepath.Clean("/tmp/custom-vault") {
		t.Fatalf("env override mismatch: got %q", gotOverride)
	}
}
