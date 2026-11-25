package shell

import (
	"image/png"
	"os"
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/vault"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

func render(region func(*ui.Context), w, h int) *ui.Tester {
	return ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		region(c)
	}, w, h)
}

func TestRibbonAndVaultFileTreeNavigation(t *testing.T) {
	root := t.TempDir()
	v, err := vault.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := v.CreateFolder("", "肿瘤耐药")
	if err != nil {
		t.Fatal(err)
	}
	src := "```asterlab\nkind: experiment\nid: e1\nschema: 1\ntitle: L929 生物相容性\nscheduled_date: 2026-11-19\n```\n"
	_, err = v.CreateExperiment(proj, "L929 生物相容性", "2026-11-19", src)
	if err != nil {
		t.Fatal(err)
	}
	tree, files, err := v.ScanTree()
	if err != nil {
		t.Fatal(err)
	}

	s := &Shell{
		VaultTree:    tree,
		VaultFiles:   files,
		VaultFolders: v.ProjectFolders(),
	}
	tt := render(s.navigation, 280, 700)
	tt.Frame()
	for _, label := range []string{"实验库", "肿瘤耐药", "L929 生物相容性", "11-19"} {
		if !tt.HasText(label) {
			t.Fatalf("missing %q in file tree: %q", label, tt.Texts())
		}
	}
	for _, forbidden := range []string{"实验方案", "Notebook", "实验运行", "实验记录", "资料与工具", "全部实验"} {
		if tt.HasText(forbidden) {
			t.Fatalf("removed legacy nav label %q should not appear: %q", forbidden, tt.Texts())
		}
	}
	if err := tt.Click("储藏室"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if cur := s.ActiveTab(); cur == nil || cur.Kind != TabPantry {
		t.Fatalf("expected Pantry tab active, got %+v", cur)
	}
}

func TestPageShortcuts(t *testing.T) {
	s := &Shell{TodayDate: "2026-11-20"}
	s.OpenFileInPane(defaultPaneID, "proj/exp.asterml", "测试实验", ModeEdit, false)
	tt := render(s.page, 1200, 760)
	tt.Frame()

	// Cmd+E toggles Edit <-> Read mode
	tt.Key(ui.Cmd, ui.KeyE)
	tt.Frame()
	if cur := s.ActiveTab(); cur == nil || cur.Mode != ModeRead {
		t.Fatalf("expected ModeRead after Cmd+E, got %+v", cur)
	}
	tt.Key(ui.Cmd, ui.KeyE)
	tt.Frame()
	if cur := s.ActiveTab(); cur == nil || cur.Mode != ModeEdit {
		t.Fatalf("expected ModeEdit after 2nd Cmd+E, got %+v", cur)
	}

	// Cmd+\ splits current pane cleanly
	tt.Key(ui.Cmd, ui.KeyBackslash)
	tt.Frame()
	if s.leafCount() != 2 || !s.SplitOpen {
		t.Fatalf("expected 2 peer split leaves after Cmd+\\, got %d", s.leafCount())
	}

	// Cmd+W closes non-pinned split tab and collapses split branch back to 1 leaf
	tt.Key(ui.Cmd, ui.KeyW)
	tt.Frame()
	if s.leafCount() != 1 || s.SplitOpen {
		t.Fatalf("expected split collapse to 1 leaf after Cmd+W, got %d", s.leafCount())
	}
}

func TestShellThemes(t *testing.T) {
	s := &Shell{TodayDate: "2026-11-20"}
	for _, dark := range []bool{false, true} {
		tt := ui.NewTesterWith(func(c *ui.Context) { s.page(c) }, 1200, 760, ui.TesterOptions{Dark: dark})
		tt.Frame()
		if tt.Image() == nil {
			t.Fatalf("dark=%v image nil", dark)
		}
	}
}

func TestCalendarHomeScheduleAgendaAndRouting(t *testing.T) {
	createdDate := ""
	opened := ExperimentItem{}
	s := &Shell{
		TodayDate:      "2026-11-19",
		SelectedDate:   "2026-11-19",
		CalendarCursor: "2026-11-19",
		Experiments: []ExperimentItem{
			{ID: "e-draft", FilePath: "p/mtt.asterml", Title: "MTT 细胞毒性初筛", Date: "2026-11-19", Stage: StageDraft},
			{ID: "e-ready", FilePath: "p/qpcr.asterml", Title: "RT-qPCR 多靶点定量", Date: "2026-11-20", Stage: StageReady, StartHour: 9.5, DurationMin: 90},
			{ID: "e-run", FilePath: "p/iz.asterml", Title: "四象限抑菌圈实验", Date: "2026-11-20", Stage: StageInProgress},
		},
		OnCreateForDate:  func(d string) { createdDate = d },
		OnOpenExperiment: func(exp ExperimentItem) { opened = exp },
	}
	tt := render(s.page, 1280, 820)
	tt.Frame()
	for _, want := range []string{"月", "周", "今天", "2026 年 11 月", "MTT 细胞毒性初筛", "RT-qPCR 多靶点定量", "四象限抑菌圈实验"} {
		if !tt.HasText(want) {
			t.Fatalf("calendar missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("2026-11-20"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.SelectedDate != "2026-11-20" || !s.DayPopoverOpen || s.DayPopoverDate != "2026-11-20" {
		t.Fatalf("day click did not open popover: date=%q open=%v popDate=%q", s.SelectedDate, s.DayPopoverOpen, s.DayPopoverDate)
	}
	if err := tt.Click("打开 四象限抑菌圈实验"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if opened.FilePath != "p/iz.asterml" {
		t.Fatalf("opened = %+v", opened)
	}
	if err := tt.Click("2026-11-23"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("2026-11-23"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if createdDate != "2026-11-23" {
		t.Fatalf("double-click create date = %q, want 2026-11-23", createdDate)
	}
	if err := tt.Click("周"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.CalendarMode != 1 || !tt.HasText("全天") || !tt.HasText("08:00") {
		t.Fatalf("weekly grid failed: mode=%d texts=%q", s.CalendarMode, tt.Texts())
	}
}

func TestEmptyCalendarHomeOffersDirectCreation(t *testing.T) {
	createdDate := ""
	s := &Shell{
		TodayDate:       "2026-11-19",
		SelectedDate:    "2026-11-19",
		CalendarCursor:  "2026-11-19",
		OnCreateForDate: func(d string) { createdDate = d },
	}
	tt := render(s.page, 900, 600)
	tt.Frame()
	if !tt.HasText("2026 年 11 月") || !tt.HasText("双击任意日期格新建实验") {
		t.Fatalf("empty calendar missing guidance: %q", tt.Texts())
	}
	if err := tt.Click("2026-11-19"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("＋ 在此日新建实验"); err != nil {
		t.Fatal(err)
	}
	if createdDate != "2026-11-19" {
		t.Fatalf("createdDate = %q, want 2026-11-19", createdDate)
	}
}

func TestPeerSplitTreeHistoryAndSameFileInTwoPanes(t *testing.T) {
	var activated []WorkspaceTab
	s := &Shell{
		TodayDate: "2026-11-19",
		OnActivateTab: func(tab WorkspaceTab) { activated = append(activated, tab) },
	}
	tt := render(s.RenderSplitView, 1360, 880)
	tt.Frame()

	s.OpenFileInPane(defaultPaneID, "proj/a.asterml", "抑菌圈实验", ModeEdit, false)
	s.SetActiveTabMode(ModeRead)
	s.OpenFileInPane(defaultPaneID, "proj/b.asterml", "RT-qPCR 定量", ModeEdit, false)
	tt.Frame()

	if len(s.Tabs) != 3 || !s.CanBack() || s.CanForward() {
		t.Fatalf("unexpected tabs/history: tabs=%d back=%v fwd=%v", len(s.Tabs), s.CanBack(), s.CanForward())
	}
	if err := tt.Click("后退 (Alt+←)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if cur := s.ActiveTab(); cur == nil || cur.FilePath != "proj/a.asterml" || cur.Mode != ModeRead {
		t.Fatalf("after 1st Back active=%+v", cur)
	}
	s.GoBack()
	tt.Frame()
	if cur := s.ActiveTab(); cur == nil || cur.FilePath != "proj/a.asterml" || cur.Mode != ModeEdit {
		t.Fatalf("after 2nd Back active=%+v", cur)
	}
	s.GoForward()
	tt.Frame()
	if cur := s.ActiveTab(); cur == nil || cur.FilePath != "proj/a.asterml" || cur.Mode != ModeRead {
		t.Fatalf("after GoForward active=%+v", cur)
	}

	// Open the SAME file in a right split pane with independent ModeSource
	s.SplitPaneWithTab(s.ActivePaneID, WorkspaceTab{
		Kind: TabFile, Title: "抑菌圈实验", FilePath: "proj/a.asterml", Mode: ModeSource,
	}, "right")
	tt.Frame()
	if s.leafCount() != 2 || !s.SplitOpen {
		t.Fatalf("expected 2 peer split panes, got %d", s.leafCount())
	}
	if len(activated) == 0 {
		t.Fatal("expected OnActivateTab invocations")
	}
}

func TestSlimRightCompanionDrawerOutlineAndCalc(t *testing.T) {
	jumpedStep := ""
	s := &Shell{
		TodayDate:    "2026-11-18",
		RightOpen:    true,
		OutlineSteps: []OutlineItem{{ID: "step-2", Title: "02 滤纸片贴敷", Meta: "3 块", Active: true}},
		OnSelectOutlineStep: func(id string) { jumpedStep = id },
	}
	s.EnsureTabs()
	s.OpenFileInPane(s.ActivePaneID, "proj/disc.asterml", "抑菌圈实验", ModeEdit, false)
	tt := render(s.RenderSplitView, 1400, 880)
	tt.Frame()

	// Tab 0: Outline
	if err := tt.Click("跳至 02 滤纸片贴敷"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if jumpedStep != "step-2" {
		t.Fatalf("jumpedStep = %q, want step-2", jumpedStep)
	}

	// Tab 1: Quick Calculator & Timers (defaults empty with placeholder, bottom timer always visible)
	if err := tt.Click("速算与计时"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("取母液 V₁") || !tt.HasText("—") || !tt.HasText("快捷倒计时") || !tt.HasText("15:00") {
		t.Fatalf("expected blank default calc + bottom countdown timer: %q", tt.Texts())
	}

	// Populated dilution calc + standalone preset switch + toggle into Molar without cross-contamination
	s.RightCalc.Stock = "100"
	s.RightCalc.Target = "50"
	s.RightCalc.FinalVol = "4"
	tt.Frame()
	if !tt.HasText("2.00 µL") {
		t.Fatalf("quick dilution calculation mismatch: %q", tt.Texts())
	}
	if err := tt.Click("30分"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("30:00") {
		t.Fatalf("expected timer preset 30:00 after click: %q", tt.Texts())
	}
	if err := tt.Click("摩尔称量"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.RightCalc.TargetUnit != "ng/mL" || s.RightCalc.MolarTargetUnit != "mM" {
		t.Fatalf("mode toggle leaked unit across tabs: %+v", s.RightCalc)
	}
}

func TestCommandPaletteSearchAndAction(t *testing.T) {
	cmdRan := false
	s := &Shell{
		TodayDate: "2026-11-18",
		Experiments: []ExperimentItem{
			{ID: "e1", FilePath: "p/qpcr.asterml", Title: "RT-qPCR 多靶点定量", Date: "2026-11-18", Stage: StageReady},
		},
		PantryItems: []QuickPantryItem{
			{ID: "p1", Name: "SYBR Green 预混液", Location: "-20°C A2", Status: "near_expiry"},
		},
		PaletteCommands: []PaletteCommand{
			{ID: "custom-pdf", Title: "导出当前实验 PDF", Subtitle: "生成 A4 组件打印件", OnRun: func() { cmdRan = true }},
		},
	}
	tt := render(s.RenderSplitView, 1360, 880)
	tt.Frame()
	if err := tt.Click("快速打开 (Ctrl+O)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !s.PaletteOpen || !tt.HasText("实验 (.asterml)") || !tt.HasText("储藏室试剂") || !tt.HasText("导出当前实验 PDF") {
		t.Fatalf("command palette did not render groups: open=%v texts=%q", s.PaletteOpen, tt.Texts())
	}
	s.PaletteQuery = "导出"
	tt.Frame()
	if tt.HasText("SYBR Green 预混液") || !tt.HasText("导出当前实验 PDF") {
		t.Fatalf("palette query filtering failed: texts=%q", tt.Texts())
	}
	if err := tt.Click("导出当前实验 PDF"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !cmdRan || s.PaletteOpen {
		t.Fatalf("palette command failed: cmdRan=%v open=%v", cmdRan, s.PaletteOpen)
	}
}

func TestCoreWriteErrorOffersRecoveryAction(t *testing.T) {
	s := &Shell{Error: "permission denied: asset root is read-only"}
	tt := render(s.page, 900, 600)
	tt.Frame()
	for _, text := range []string{"未保存", "permission denied: asset root is read-only", "检查目标目录权限", "重试保存"} {
		if !tt.HasText(text) {
			t.Fatalf("failed-write state missing %q: %q", text, tt.Texts())
		}
	}
}

func TestTimerStripShowsAndCollapsesRunningTimers(t *testing.T) {
	rSoon, rLater := uint64(30), uint64(3600)
	selected := ""
	s := &Shell{OnTimerSelect: func(id string) { selected = id }, ActiveTimers: []runs.ActiveTimer{
		{RunID: "r1", RunTitle: "L929 夜间孵育", TimerID: "t1", ElapsedS: 300, RemainingS: &rLater},
		{RunID: "r2", RunTitle: "CCK-8 读板", TimerID: "t2", ElapsedS: 61},
		{RunID: "r3", RunTitle: "RT-qPCR 扩增", TimerID: "t3", ElapsedS: 10, RemainingS: &rSoon},
	}}
	tt := render(s.page, 900, 600)
	tt.Frame()
	for _, text := range []string{"RT-qPCR 扩增 · 剩余 0:30", "L929 夜间孵育 · 剩余 1:00:00", "＋1 更多"} {
		if !tt.HasText(text) {
			t.Fatalf("compact strip missing %q: %q", text, tt.Texts())
		}
	}
	if err := tt.Click("＋1 更多"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("CCK-8 读板 · 已计时 1:01"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if selected != "r2" {
		t.Fatalf("timer click selected run %q, want r2", selected)
	}
}

func TestShellResponsiveThemeScreenshots(t *testing.T) {
	for _, size := range []struct {
		w, h  int
		label string
	}{{1186, 753, "1186x753"}, {900, 600, "900x600"}} {
		for _, dark := range []bool{false, true} {
			name := "light"
			if dark {
				name = "dark"
			}
			tt := ui.NewTesterWith(func(c *ui.Context) { (&Shell{}).page(c) }, size.w, size.h, ui.TesterOptions{Dark: dark})
			tt.Frame()
			f, err := os.Create("evidence-shell-" + size.label + "-" + name + ".png")
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestObsidianMobileV2FloatingDockTabSwitcherAndEditToolbar(t *testing.T) {
	var toolbarActs []string
	var exps []ExperimentItem
	for i := 1; i <= 10; i++ {
		exps = append(exps, ExperimentItem{
			ID:     "e-" + string(rune('0'+i)),
			Title:  "细胞增殖实验 #" + string(rune('0'+i)),
			Date:   "2026-11-18",
			Folder: "肿瘤筛选",
			Stage:  StageReady,
		})
	}
	s := &Shell{
		TodayDate:          "2026-11-20",
		SelectedDate:       "2026-11-18",
		ForceMobilePreview: true,
		Experiments:        exps,
		OnMobileDocToolbarAction: func(tab WorkspaceTab, action string) {
			toolbarActs = append(toolbarActs, action)
		},
	}
	s.OpenFileInPane(defaultPaneID, "proj/l929.asterml", "L929 药物梯度测定", ModeEdit, false)
	s.OpenCalendarTab()

	tt := render(s.RenderSplitView, 390, 844)
	tt.Frame()
	tt.Advance(220 * time.Millisecond)
	tt.Frame()

	// 1. Centered icon-only Liquid Glass navigation capsule on calendar (accessible labels present; zero cluttered text)
	if !tt.HasText("2026 年 11 月") || !tt.HasText("2") {
		t.Fatalf("expected calendar & tab badge count 2 in mobile capsule: %q", tt.Texts())
	}

	// 2. Scroll downward triggers auto-hide on dock; scroll back up reveals dock
	st := s.MobileContentScrollState()
	st.Y = 64
	tt.Frame()
	if !s.MobileDockHidden {
		t.Fatalf("expected MobileDockHidden=true after downward scroll")
	}
	st.Y = 0
	tt.Frame()
	if s.MobileDockHidden {
		t.Fatalf("expected MobileDockHidden=false after scrolling back to top")
	}

	// 3. Open Tab Switcher Card Overview via [2] badge button and switch to File Tab in Edit mode
	if err := tt.Click("标签"); err != nil {
		t.Fatalf("click tab switcher button: %v", err)
	}
	tt.Frame()
	tt.Advance(220 * time.Millisecond)
	tt.Frame()
	if !s.MobileTabSwitcherOpen || !tt.HasText("标签页概览") {
		t.Fatalf("expected Tab Switcher overlay open: %q", tt.Texts())
	}
	if err := tt.Click("L929 药物梯度测定"); err != nil {
		t.Fatalf("click tab card in switcher: %v", err)
	}
	tt.Frame()
	tt.Advance(220 * time.Millisecond)
	tt.Frame()
	if s.MobileTabSwitcherOpen {
		t.Fatalf("expected Tab Switcher to close after picking tab")
	}
	if cur := s.ActiveTab(); cur == nil || cur.FilePath != "proj/l929.asterml" {
		t.Fatalf("expected active tab L929, got %+v", cur)
	}

	// 4. Edit mode shows icon-only Mobile Edit Toolbar instead of Nav Dock
	if err := tt.Click("+步骤"); err != nil {
		t.Fatalf("click +步骤 in mobile edit toolbar: %v", err)
	}
	tt.Frame()
	if len(toolbarActs) == 0 || toolbarActs[0] != "add_step" {
		t.Fatalf("expected add_step callback, got %+v", toolbarActs)
	}

	// 5. Toggle temporarily back to Navigation Dock while staying in Edit mode
	if err := tt.Click("切换底部导航栏"); err != nil {
		t.Fatalf("click nav dock toggle: %v", err)
	}
	tt.Frame()
	tt.Advance(220 * time.Millisecond)
	tt.Frame()

	// 6. Open File Tree Drawer via ≡ button and verify Obsidian 3-tier bottom control island
	if err := tt.Click("实验库与工具"); err != nil {
		t.Fatalf("click tree drawer button: %v", err)
	}
	tt.Frame()
	tt.Advance(240 * time.Millisecond)
	tt.Frame()
	if s.MobileDrawer != "tree" || !tt.HasText("实验列表") || !tt.HasText("AsterLab") {
		t.Fatalf("expected mobile tree drawer with bottom island, got drawer=%q texts=%q", s.MobileDrawer, tt.Texts())
	}

	// 7. Execute Pull-Down Quick Action (default: command palette)
	s.MobileDrawer = ""
	s.MobilePullOffset = 72
	tt.Frame()
	if !s.PaletteOpen {
		t.Fatalf("expected PaletteOpen after pull-down quick action past threshold")
	}
}

func TestThemePresetAndLiquidGlassSwitching(t *testing.T) {
	s := &Shell{TodayDate: "2026-11-20", ThemePreset: theme.PresetObsidian, LiquidGlass: true}
	var gotTok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		gotTok = s.SyncThemeLook(c)
		s.RenderSplitView(c)
	}, 1100, 720)
	tt.Frame()
	if gotTok.Preset != theme.PresetObsidian || !gotTok.LiquidGlass || gotTok.Canvas != ui.Hex("#ffffff") {
		t.Fatalf("expected obsidian liquid glass default, got %+v", gotTok)
	}

	s.ThemePreset = theme.PresetZinc
	tt.Frame()
	if gotTok.Preset != theme.PresetZinc || gotTok.Canvas != ui.Hex("#f4f5f7") {
		t.Fatalf("expected crisp zinc preset after live switch, got %+v", gotTok)
	}

	s.ThemePreset = theme.PresetWarm
	s.LiquidGlass = false
	tt.Frame()
	if gotTok.Preset != theme.PresetWarm || gotTok.LiquidGlass || gotTok.Canvas != ui.Hex("#f7f6f3") {
		t.Fatalf("expected warm non-glass fallback, got %+v", gotTok)
	}
}

func TestWeekGridYToMinutesPrecisionAndSnap(t *testing.T) {
	// rowH = 56px -> 30min = 28px. StartHr = 8, EndHr = 21.
	rowH := float32(56)
	startHr, endHr := 8, 21

	// Within ±7px of 10:00 (y=112): snaps cleanly to 10:00
	mins, snapped, dispY := weekGridYToMinutes(117, rowH, startHr, endHr)
	if !snapped || mins != 10*60 || dispY != 112 {
		t.Fatalf("near-boundary snap got (%d, %v, %.1f), want (600, true, 112.0)", mins, snapped, dispY)
	}

	// Quarter past (y = 2*56 + 14 = 126, >7px from 112 and 140): preserves 5-min precision 10:15
	mins, snapped, dispY = weekGridYToMinutes(126, rowH, startHr, endHr)
	if snapped || mins != 10*60+15 || dispY != 126 {
		t.Fatalf("5-min precision got (%d, %v, %.1f), want (615, false, 126.0)", mins, snapped, dispY)
	}
}

func TestWeekCalendarSuppressesHoverBehindModalAndSettingsSegmentedFits(t *testing.T) {
	extModal := false
	s := &Shell{
		TodayDate:           "2026-11-25",
		CalendarMode:        1,
		weekHoverOn:         true,
		weekHoverY:          120,
		IsExternalModalOpen: func() bool { return extModal },
	}
	s.EnsureTabs()
	s.OpenCalendarTab()
	if s.AnyModalOpen() {
		t.Fatal("expected AnyModalOpen false initially")
	}

	extModal = true
	if !s.AnyModalOpen() {
		t.Fatal("expected AnyModalOpen true when host dialog is open")
	}
	tt := render(s.RenderSplitView, 1365, 872)
	tt.Frame()
	if s.weekHoverOn {
		t.Fatalf("expected weekHoverOn cleared when modal dialog is open")
	}

	// Open Settings -> 编辑器 tab and verify 阅读态 segmented pill fits without clipping
	extModal = false
	s.VaultSettingsOpen = true
	s.SettingsView.Category = "editor"
	tt.Frame()
	if !tt.HasText("默认打开视图") || !tt.HasText("阅读态") {
		t.Fatalf("expected editor settings with 默认打开视图 / 阅读态, got %q", tt.Texts())
	}
	el, ok := tt.Find("阅读态")
	if !ok {
		t.Fatal("missing 阅读态 button element")
	}
	if el.W < 40 || el.X+el.W > 1365 {
		t.Fatalf("unexpected 阅读态 bounds in 1365dp window: %+v", el)
	}
}
