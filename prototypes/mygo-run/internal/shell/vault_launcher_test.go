package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestVaultLauncherDialogOpenAndCloseWithoutPanic(t *testing.T) {
	s := &Shell{
		TodayDate:             "2026-11-24",
		SelectedDate:          "2026-11-24",
		VaultPathInput:        `D:\L\AsterLab`,
		VaultLauncherOpen:     true,
		VaultLauncherName:     "MyLab",
		VaultLauncherParent:   `D:\`,
		VaultLauncherSeedDemo: true,
		RecentVaults: []RecentVaultEntry{
			{Name: "AsterLab", Path: `D:\L\AsterLab`, Active: true},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			s.renderVaultLauncherDialog(c)
		})
	}, 1280, 820)

	for i := 0; i < 6; i++ {
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
	}
	if !tt.HasText("新建本地实验库") || !tt.HasText("打开电脑已有文件夹作为实验库") {
		t.Fatalf("expected Vault Launcher cards, got texts: %q", tt.Texts())
	}

	// Close launcher via Escape / state toggle and tick through exit animation
	s.VaultLauncherOpen = false
	for i := 0; i < 12; i++ {
		tt.Frame()
		tt.Advance(30 * time.Millisecond)
	}
}
