package shell

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/syncengine"
	"asterlab-mygo-prototype/internal/theme"
)

func renderThemeViewport(region func(*ui.Context), dark bool, w, h int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		if dark {
			c.SetTheme(theme.DarkTheme())
		} else {
			c.SetTheme(theme.LightTheme())
		}
		region(c)
	}, w, h)
}

func scrollSettingsUntilFound(tt *ui.Tester, text string, w, h int) bool {
	cx := float32(w) * 0.65
	cy := float32(h) * 0.55
	if w < 680 {
		cx = float32(w) * 0.5
	}
	prevY := float32(-1)
	for i := 0; i < 30; i++ {
		if r, ok := tt.Find(text); ok && r.W > 0 && r.H > 0 {
			if r.Y >= 100 && r.Y+r.H <= float32(h)-96 {
				return true
			}
			// Reached bottom of scroll area without clipping the dialog frame.
			if i > 0 && r.Y == prevY && r.Y >= 60 && r.Y+r.H <= float32(h)-56 {
				return true
			}
			prevY = r.Y
		}
		tt.Scroll(cx, cy, 0, 140)
		tt.Frame()
	}
	return false
}

func TestSyncSettingsCategoryWebDAVAndNutstoreBadge(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
		w, h int
	}{
		{"light_desktop_1280x800", false, 1280, 800},
		{"dark_desktop_1280x800", true, 1280, 800},
		{"light_mobile_390x844", false, 390, 844},
		{"dark_mobile_390x844", true, 390, 844},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var saveCalls, testCalls, syncNowCalls int
			s := &Shell{
				TodayDate:          "2026-11-26",
				ForceMobilePreview: tc.w < 680,
				VaultSettingsOpen:  true,
				SettingsView:       components.SettingsState{Category: "sync"},
				SyncSettings: SyncSettings{
					Enabled:          true,
					Endpoint:         "https://dav.jianguoyun.com/dav/",
					Username:         "lab@asterlab.party",
					Password:         "super-secret-app-pw",
					RemotePath:       "/AsterLab",
					AutoIntervalMin:  5,
					ImagePolicy:      SyncImagePolicyThreshold,
					ImageThresholdMB: 2,
				},
				SyncState: SyncStateSummary{
					Status:                "synced",
					LastSyncText:          "12:30",
					PendingCount:          0,
					LocalOriginalsCacheMB: 12.5,
				},
				OnSaveSyncSettings:   func() { saveCalls++ },
				OnTestSyncConnection: func() { testCalls++ },
				OnTriggerSyncNow:     func() { syncNowCalls++ },
			}

			tt := renderThemeViewport(s.RenderSplitView, tc.dark, tc.w, tc.h)
			tt.Frame()
			advanceMobileFrames(tt, 220*time.Millisecond)

			for _, want := range []string{
				"多端同步",
				"WebDAV 多端同步",
				"启用多端同步",
				"服务器地址 (WebDAV)",
				"坚果云自动限流保护 · 单线程并发",
				"应用密码",
				"测试连接",
				"立即同步",
			} {
				if !tt.HasText(want) {
					t.Fatalf("[%s] missing %q in sync settings; texts=%q", tc.name, want, tt.Texts())
				}
			}

			// Masked password must never leak plaintext when unrevealed.
			if strings.Contains(strings.Join(tt.Texts(), " "), "super-secret-app-pw") {
				t.Fatalf("[%s] password should be masked by default", tc.name)
			}

			if !scrollSettingsUntilFound(tt, "测试连接", tc.w, tc.h) {
				t.Fatalf("[%s] could not scroll to 测试连接", tc.name)
			}
			if err := tt.Click("测试连接"); err != nil {
				t.Fatalf("[%s] click 测试连接: %v", tc.name, err)
			}
			tt.Frame()
			if testCalls != 1 {
				t.Fatalf("[%s] expected OnTestSyncConnection=1, got %d", tc.name, testCalls)
			}

			if err := tt.Click("立即同步"); err != nil {
				t.Fatalf("[%s] click 立即同步: %v", tc.name, err)
			}
			tt.Frame()
			if syncNowCalls != 1 {
				t.Fatalf("[%s] expected OnTriggerSyncNow=1, got %d", tc.name, syncNowCalls)
			}
			if saveCalls < 2 {
				t.Fatalf("[%s] expected OnSaveSyncSettings saved on test/sync, got %d", tc.name, saveCalls)
			}
		})
	}
}

func TestSyncImageThresholdPolicyAndCacheClear(t *testing.T) {
	var saveCalls, clearCalls int
	s := &Shell{
		TodayDate:         "2026-11-26",
		VaultSettingsOpen: true,
		SettingsView:      components.SettingsState{Category: "sync"},
		SyncSettings: SyncSettings{
			Enabled:           true,
			Endpoint:          "https://nas.lab.local/dav/",
			ImagePolicy:       SyncImagePolicyThreshold,
			ImageThresholdMB:  2,
			WiFiOnlyOriginals: true,
		},
		SyncState: SyncStateSummary{
			Status:                "synced",
			LocalOriginalsCacheMB: 42.5,
		},
		OnSaveSyncSettings:    func() { saveCalls++ },
		OnClearOriginalsCache: func() { clearCalls++ },
	}

	tt := renderThemeViewport(s.RenderSplitView, false, 1280, 800)
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)

	if !scrollSettingsUntilFound(tt, "科学图像双轨同步策略", 1280, 800) {
		t.Fatalf("expected 科学图像双轨同步策略 section; texts=%q", tt.Texts())
	}
	if !tt.HasText("上传端始终保留 100% 科学原图，手机端按阈值拉取预览图") {
		t.Fatalf("missing explicit scientific image sync policy statement; texts=%q", tt.Texts())
	}

	// Switch threshold to 10 MB.
	if !scrollSettingsUntilFound(tt, "10 MB", 1280, 800) {
		t.Fatalf("expected 10 MB threshold pill")
	}
	if err := tt.Click("10 MB"); err != nil {
		t.Fatalf("click 10 MB: %v", err)
	}
	tt.Frame()
	if s.SyncSettings.ImageThresholdMB != 10 {
		t.Fatalf("expected ImageThresholdMB=10, got %d", s.SyncSettings.ImageThresholdMB)
	}
	pol := s.SyncSettings.ToDownloadPolicy(true)
	if pol.Mode != syncengine.PolicyThresholdBytes || pol.ThresholdBytes != syncengine.Threshold10MB || !pol.WiFiOnlyForOriginals {
		t.Fatalf("unexpected DownloadPolicy after threshold click: %+v", pol)
	}

	// Switch to preview_only.
	if err := tt.Click("仅同步预览图"); err != nil {
		t.Fatalf("click 仅同步预览图: %v", err)
	}
	tt.Frame()
	if s.SyncSettings.ImagePolicy != SyncImagePolicyPreviewOnly {
		t.Fatalf("expected ImagePolicy=preview_only, got %q", s.SyncSettings.ImagePolicy)
	}
	if pol := s.SyncSettings.ToDownloadPolicy(true); pol.Mode != syncengine.PolicyPreviewOnly {
		t.Fatalf("expected PolicyPreviewOnly, got %+v", pol)
	}

	// Switch to always_original.
	if err := tt.Click("始终拉原图"); err != nil {
		t.Fatalf("click 始终拉原图: %v", err)
	}
	tt.Frame()
	if s.SyncSettings.ImagePolicy != SyncImagePolicyAlwaysOriginal {
		t.Fatalf("expected ImagePolicy=always_original, got %q", s.SyncSettings.ImagePolicy)
	}
	if pol := s.SyncSettings.ToDownloadPolicy(true); pol.Mode != syncengine.PolicyAlwaysOriginal {
		t.Fatalf("expected PolicyAlwaysOriginal, got %+v", pol)
	}

	// Switch back to threshold and pick 5 MB.
	if err := tt.Click("阈值内拉原图 (1/2/5/10 MB)"); err != nil {
		t.Fatalf("click threshold mode: %v", err)
	}
	tt.Frame()
	if err := tt.Click("5 MB"); err != nil {
		t.Fatalf("click 5 MB: %v", err)
	}
	tt.Frame()
	if s.SyncSettings.ImagePolicy != SyncImagePolicyThreshold || s.SyncSettings.ImageThresholdMB != 5 {
		t.Fatalf("expected threshold=5MB, got policy=%q mb=%d", s.SyncSettings.ImagePolicy, s.SyncSettings.ImageThresholdMB)
	}

	// Toggle Wi-Fi switch.
	if !scrollSettingsUntilFound(tt, "切换仅 Wi-Fi 自动拉原图", 1280, 800) {
		t.Fatalf("expected Wi-Fi switch")
	}
	if err := tt.Click("切换仅 Wi-Fi 自动拉原图"); err != nil {
		t.Fatalf("click Wi-Fi switch: %v", err)
	}
	tt.Frame()
	if s.SyncSettings.WiFiOnlyOriginals {
		t.Fatalf("expected WiFiOnlyOriginals toggled to false")
	}

	// Trigger local originals cache clear.
	cacheBtnLabel := "本地原图缓存 42.5 MB · 一键释放"
	if !scrollSettingsUntilFound(tt, cacheBtnLabel, 1280, 800) {
		t.Fatalf("expected cache clear button %q", cacheBtnLabel)
	}
	if err := tt.Click(cacheBtnLabel); err != nil {
		t.Fatalf("click cache clear button: %v", err)
	}
	tt.Frame()
	if clearCalls != 1 {
		t.Fatalf("expected OnClearOriginalsCache=1, got %d", clearCalls)
	}
	if saveCalls == 0 {
		t.Fatalf("expected OnSaveSyncSettings called on policy/switch changes")
	}
}

func TestSyncConflictResolutionAndZipBackupRestore(t *testing.T) {
	type resolvedCall struct {
		rel, conflict, choice string
	}
	var resolved []resolvedCall
	var backupCalls, restoreCalls int

	s := &Shell{
		TodayDate:         "2026-11-26",
		VaultSettingsOpen: true,
		SettingsView:      components.SettingsState{Category: "sync"},
		SyncConflicts: []SyncConflictItem{
			{
				RelPath:      "肿瘤耐药/wb_blot.asterml",
				ConflictPath: "肿瘤耐药/wb_blot.sync-conflict-ipad-20261126-001200.asterml",
				DetectedAt:   "2026-11-26 00:12",
			},
		},
		OnResolveSyncConflict: func(relPath, conflictPath, choice string) {
			resolved = append(resolved, resolvedCall{relPath, conflictPath, choice})
		},
		OnVaultBackupZip:  func() { backupCalls++ },
		OnVaultRestoreZip: func() { restoreCalls++ },
	}

	tt := renderThemeViewport(s.RenderSplitView, true, 1280, 800)
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)

	if !scrollSettingsUntilFound(tt, "冲突副本处理", 1280, 800) {
		t.Fatalf("expected 冲突副本处理 group; texts=%q", tt.Texts())
	}
	for _, act := range []struct {
		label  string
		choice string
	}{
		{"保留本地", "keep_local"},
		{"采用远端", "use_remote"},
		{"打开副本", "open_copy"},
	} {
		if !scrollSettingsUntilFound(tt, act.label, 1280, 800) {
			t.Fatalf("expected conflict button %q", act.label)
		}
		if err := tt.Click(act.label); err != nil {
			t.Fatalf("click %q: %v", act.label, err)
		}
		tt.Frame()
	}
	if len(resolved) != 3 ||
		resolved[0].choice != "keep_local" ||
		resolved[1].choice != "use_remote" ||
		resolved[2].choice != "open_copy" {
		t.Fatalf("unexpected conflict resolutions: %+v", resolved)
	}
	if resolved[0].rel != "肿瘤耐药/wb_blot.asterml" || resolved[0].conflict != "肿瘤耐药/wb_blot.sync-conflict-ipad-20261126-001200.asterml" {
		t.Fatalf("unexpected conflict paths: %+v", resolved[0])
	}

	// Switch to vault category and verify backup/restore buttons in 文件恢复 group.
	s.SettingsView.Category = "vault"
	tt.Frame()
	advanceMobileFrames(tt, 120*time.Millisecond)

	if !scrollSettingsUntilFound(tt, "导出整库 .zip 备份", 1280, 800) {
		t.Fatalf("expected 导出整库 .zip 备份 button in vault settings; texts=%q", tt.Texts())
	}
	if err := tt.Click("导出整库 .zip 备份"); err != nil {
		t.Fatalf("click 导出整库 .zip 备份: %v", err)
	}
	tt.Frame()
	if err := tt.Click("从 .zip 恢复整库"); err != nil {
		t.Fatalf("click 从 .zip 恢复整库: %v", err)
	}
	tt.Frame()
	if backupCalls != 1 || restoreCalls != 1 {
		t.Fatalf("expected backupCalls=1 restoreCalls=1, got %d, %d", backupCalls, restoreCalls)
	}
}

func TestMobileDrawerSyncIndicatorLightAndDark390x844(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			syncCalls := 0
			s := &Shell{
				TodayDate:          "2026-11-26",
				ForceMobilePreview: true,
				MobileDrawer:       "tree",
				SyncSettings: SyncSettings{
					Enabled: true,
				},
				SyncState: SyncStateSummary{
					Status:       "synced",
					LastSyncText: "刚刚",
					PendingCount: 2,
				},
				OnTriggerSyncNow: func() { syncCalls++ },
			}
			s.OpenCalendarTab()

			tt := renderThemeViewport(s.RenderSplitView, dark, 390, 844)
			tt.Frame()
			advanceMobileFrames(tt, 260*time.Millisecond)

			if !tt.HasText("待同步 2") {
				t.Fatalf("[%s] expected 待同步 2 indicator in mobile drawer; texts=%q", name, tt.Texts())
			}
			if err := tt.Click("同步状态 待同步 2"); err != nil {
				t.Fatalf("[%s] tap sync status pill: %v", name, err)
			}
			tt.Frame()
			if syncCalls != 1 {
				t.Fatalf("[%s] expected tap on enabled sync pill to trigger OnTriggerSyncNow, got %d", name, syncCalls)
			}

			// When conflicts exist, indicator switches to warning copy and tapping opens Sync settings.
			s.SyncConflicts = []SyncConflictItem{{RelPath: "a.asterml", ConflictPath: "a.sync-conflict-m-1.asterml"}}
			tt.Frame()
			if !tt.HasText("冲突 1") {
				t.Fatalf("[%s] expected 冲突 1 warning indicator; texts=%q", name, tt.Texts())
			}
			if err := tt.Click("同步状态 冲突 1"); err != nil {
				t.Fatalf("[%s] tap conflict sync pill: %v", name, err)
			}
			tt.Frame()
			if s.MobileDrawer != "" || !s.VaultSettingsOpen || s.SettingsView.Category != "sync" {
				t.Fatalf("[%s] expected tapping conflict pill to open sync settings, got drawer=%q open=%v cat=%q",
					name, s.MobileDrawer, s.VaultSettingsOpen, s.SettingsView.Category)
			}
		})
	}
}
