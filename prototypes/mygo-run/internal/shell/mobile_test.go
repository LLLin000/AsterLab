package shell

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"

	"asterlab-mygo-prototype/internal/schedule"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/vault"
)

func advanceMobileFrames(tt *ui.Tester, d time.Duration) {
	steps := int(d / (16 * time.Millisecond))
	if steps < 1 {
		steps = 1
	}
	for range steps {
		tt.Advance(16 * time.Millisecond)
		tt.Frame()
	}
}

func TestMobileToolbarCustomizationAndPersistence(t *testing.T) {
	tmpVault := t.TempDir()
	var fired []string

	s := &Shell{
		TodayDate:          "2026-11-20",
		SelectedDate:       "2026-11-20",
		ForceMobilePreview: true,
		MobileKeyboardOpen: true,
		VaultPathInput:     tmpVault,
		OnMobileDocToolbarAction: func(tab WorkspaceTab, action string) {
			fired = append(fired, action)
		},
	}
	s.OpenFileInPane(defaultPaneID, "proj/pcr.asterml", "qPCR 扩增验证", ModeEdit, false)

	// Verify all required scientific, formatting/structure, and mobile-native nav actions exist.
	required := []string{
		"at_entity", "slash_module", "footnote", "nl_structure", "camera_photo", "timer", "add_step",
		"undo", "redo", "heading", "bold", "checklist", "bullet_list", "indent", "outdent",
		"move_block_up", "move_block_down", "fold_block",
		"cursor_left", "cursor_right", "hide_keyboard", "switch_nav",
	}
	order := s.MobileToolbarOrder()
	seen := map[string]bool{}
	for _, id := range order {
		seen[id] = true
	}
	for _, req := range required {
		if !seen[req] {
			t.Fatalf("missing required mobile toolbar action %q in default order %v", req, order)
		}
	}

	tt := render(s.RenderSplitView, 390, 844)
	tt.Frame()
	advanceMobileFrames(tt, 200*time.Millisecond)

	for _, lbl := range []string{"插入实体 @", "插入模块 /", "自然语言转结构", "+步骤"} {
		if err := tt.Click(lbl); err != nil {
			t.Fatalf("click toolbar button %q: %v", lbl, err)
		}
		tt.Frame()
	}
	if len(fired) != 4 || fired[0] != "at_entity" || fired[1] != "slash_module" || fired[2] != "nl_structure" || fired[3] != "add_step" {
		t.Fatalf("unexpected dispatched mobile toolbar actions: %v", fired)
	}

	// Customize visibility & ordering and confirm disk persistence.
	s.SetMobileToolbarActionVisible("at_entity", false)
	if !s.MoveMobileToolbarAction("bold", -1) {
		t.Fatalf("expected MoveMobileToolbarAction(bold, -1) to succeed")
	}

	cfgFile := filepath.Join(tmpVault, vault.SystemDirName, mobileToolbarConfigFile)
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected persisted toolbar config at %s: %v", cfgFile, err)
	}

	// Reload into a fresh Shell instance from disk.
	s2 := &Shell{
		TodayDate:          "2026-11-20",
		SelectedDate:       "2026-11-20",
		ForceMobilePreview: true,
		VaultPathInput:     tmpVault,
	}
	s2.OpenFileInPane(defaultPaneID, "proj/pcr.asterml", "qPCR 扩增验证", ModeEdit, false)
	if s2.IsMobileToolbarActionVisible("at_entity") {
		t.Fatalf("expected at_entity to stay hidden after reload from disk")
	}
	order2 := s2.MobileToolbarOrder()
	boldIdx, headingIdx := -1, -1
	for i, id := range order2 {
		if id == "bold" {
			boldIdx = i
		} else if id == "heading" {
			headingIdx = i
		}
	}
	if boldIdx < 0 || headingIdx < 0 || boldIdx >= headingIdx {
		t.Fatalf("expected bold moved before heading after reload, got order=%v", order2)
	}

	tt2 := render(s2.RenderSplitView, 390, 844)
	tt2.Frame()
	if _, ok := tt2.Find("插入实体 @"); ok {
		t.Fatalf("expected hidden toolbar button 插入实体 @ not to be rendered")
	}

	// Reset restores defaults.
	s2.ResetMobileToolbarConfig()
	if !s2.IsMobileToolbarActionVisible("at_entity") {
		t.Fatalf("expected ResetMobileToolbarConfig to restore at_entity visibility")
	}
}

func TestMobileSettingsToolbarCustomizationAndPullActionUI(t *testing.T) {
	tmpVault := t.TempDir()
	changedCalls := 0
	s := &Shell{
		TodayDate:          "2026-11-20",
		ForceMobilePreview: true,
		VaultPathInput:     tmpVault,
		VaultSettingsOpen:  true,
		SettingsView:       components.SettingsState{Category: "appearance"},
		OnSettingsChanged:  func() { changedCalls++ },
	}
	tt := render(s.RenderSplitView, 390, 844)
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)

	if !tt.HasText("顶部下拉快捷动作") || !tt.HasText("自定义移动端编辑工具栏") {
		t.Fatalf("expected mobile settings controls in appearance settings, got: %q", tt.Texts())
	}

	// Scroll Settings pane so the toolbar customization row is inside the 390x844 viewport.
	for i := 0; i < 8; i++ {
		if r, ok := tt.Find("上移 插入模块 /"); ok && r.Y >= 80 && r.Y+r.H <= 760 {
			break
		}
		tt.Scroll(200, 450, 0, 240)
		tt.Frame()
	}

	// Move second item up via Settings UI button.
	if err := tt.Click("上移 插入模块 /"); err != nil {
		t.Fatalf("click move up in settings: %v", err)
	}
	tt.Frame()
	order := s.MobileToolbarOrder()
	if len(order) < 2 || order[0] != "slash_module" {
		t.Fatalf("expected slash_module moved to index 0, got %v", order)
	}
	if changedCalls == 0 {
		t.Fatalf("expected OnSettingsChanged to be notified")
	}

	// Reset via Settings UI button.
	if err := tt.Click("恢复默认工具栏"); err != nil {
		t.Fatalf("click reset toolbar in settings: %v", err)
	}
	tt.Frame()
	if order := s.MobileToolbarOrder(); order[0] != "at_entity" {
		t.Fatalf("expected default order restored, got %v", order)
	}
}

func TestMobileBackDismissalStack(t *testing.T) {
	s := &Shell{
		TodayDate:             "2026-11-20",
		SelectedDate:          "2026-11-20",
		ForceMobilePreview:    true,
		PaletteOpen:           true,
		DayPopoverOpen:        true,
		MobileTabSwitcherOpen: true,
		MobileDrawer:          "tree",
	}
	s.OpenCalendarTab()
	s.OpenFileInPane(defaultPaneID, "proj/a.asterml", "实验 A", ModeEdit, false)

	// 1. Palette closes first
	if !s.HandleMobileBack() || s.PaletteOpen {
		t.Fatalf("step 1: expected PaletteOpen dismissed")
	}
	// 2. Day popover closes
	if !s.HandleMobileBack() || s.DayPopoverOpen {
		t.Fatalf("step 2: expected DayPopoverOpen dismissed")
	}
	// 3. Tab switcher closes
	if !s.HandleMobileBack() || s.MobileTabSwitcherOpen {
		t.Fatalf("step 3: expected MobileTabSwitcherOpen dismissed")
	}
	// 4. Mobile drawer closes
	if !s.HandleMobileBack() || s.MobileDrawer != "" {
		t.Fatalf("step 4: expected MobileDrawer dismissed")
	}
	// 5. History navigation goes back to calendar tab
	if !s.HandleMobileBack() {
		t.Fatalf("step 5: expected history GoBack consumed")
	}
	if cur := s.ActiveTab(); cur == nil || cur.Kind != TabCalendar {
		t.Fatalf("step 5: expected back to Calendar tab, got %+v", cur)
	}
	// 6. Root reached -> returns false so OS can background app
	if s.HandleMobileBack() {
		t.Fatalf("step 6: expected false when dismissal stack is empty")
	}
}

func TestMobileCalendarAgendaAndSwipeableDateStrip(t *testing.T) {
	var opened []string
	s := &Shell{
		TodayDate:          "2026-11-20",
		SelectedDate:       "2026-11-20",
		CalendarCursor:     "2026-11-20",
		CalendarMode:       1, // Mobile Agenda (replaces cramped 7-column week grid)
		ForceMobilePreview: true,
		Experiments: []ExperimentItem{
			{ID: "e1", Title: "上午细胞传代", Date: "2026-11-20", StartHour: 9.5, DurationMin: 45, Stage: StageInProgress, Folder: "细胞室"},
			{ID: "e2", Title: "明日流式检测", Date: "2026-11-21", Stage: StageReady, Folder: "免疫组"},
			{ID: "e3", Title: "后日切片染色", Date: "2026-11-22", Stage: StageDraft, Folder: "病理组"},
		},
		CalendarEvents: []schedule.Event{
			{ID: "ev1", Domain: schedule.DomainReagent, Kind: schedule.KindReagentRestock, Title: "补充 DMEM 培养基", DateYMD: "2026-11-20", Severity: schedule.SeverityWarning},
		},
		OnOpenExperiment: func(exp ExperimentItem) {
			opened = append(opened, exp.ID)
		},
	}
	s.OpenCalendarTab()

	tt := render(s.RenderSplitView, 390, 844)
	tt.Frame()
	advanceMobileFrames(tt, 220*time.Millisecond)

	// Cramped 7-col hourly gutter must NOT render on mobile.
	if tt.HasText("07:00") {
		t.Fatalf("expected desktop 7-column 07:00 week grid replaced on mobile, got: %q", tt.Texts())
	}
	// 3-Day agenda shows all 3 consecutive days + timed chip + domain event.
	for _, expected := range []string{"3日日程", "09:30", "上午细胞传代", "明日流式检测", "后日切片染色", "补充 DMEM 培养基"} {
		if !tt.HasText(expected) {
			t.Fatalf("expected %q in mobile 3-day agenda, got: %q", expected, tt.Texts())
		}
	}

	// Tap an experiment card to open it.
	if err := tt.Click("上午细胞传代"); err != nil {
		t.Fatalf("click mobile agenda experiment: %v", err)
	}
	tt.Frame()
	if len(opened) != 1 || opened[0] != "e1" {
		t.Fatalf("expected e1 opened, got %v", opened)
	}

	// Switch to Single-Day ("单日") mode.
	if err := tt.Click("单日"); err != nil {
		t.Fatalf("click 单日 tab: %v", err)
	}
	tt.Frame()
	if !tt.HasText("上午细胞传代") || tt.HasText("后日切片染色") {
		t.Fatalf("expected single-day view to show only selected day, got: %q", tt.Texts())
	}

	// Swipe the horizontal date strip left -> advances week by +7 days.
	stripBox, ok := tt.Find("日期 2026-11-20")
	if !ok {
		t.Fatalf("expected date strip cell for 2026-11-20")
	}
	y := stripBox.Y + stripBox.H/2
	tt.FingerDown(340, y)
	tt.Advance(16 * time.Millisecond)
	for x := float32(320); x >= 40; x -= 35 {
		tt.FingerMove(x, y)
		tt.Frame()
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(200 * time.Millisecond)
	tt.FingerUp(40, y)
	advanceMobileFrames(tt, 2*time.Second)
	if s.SelectedDate != "2026-11-27" {
		t.Fatalf("expected swiping date strip left to advance SelectedDate to 2026-11-27, got %q (stripBox=%+v)", s.SelectedDate, stripBox)
	}
}

func TestMobileSafeAreaAndFileTreeSwipeActions(t *testing.T) {
	tmpVault := t.TempDir()
	var duplicated, deleted []string
	s := &Shell{
		TodayDate:          "2026-11-20",
		SelectedDate:       "2026-11-20",
		ForceMobilePreview: true,
		MobileKeyboardOpen: true,
		VaultPathInput:     tmpVault,
		MobileDrawer:       "tree",
		VaultTree: &vault.TreeNode{
			IsDir: true,
			Children: []*vault.TreeNode{
				{
					RelPath: "proj/exp1.asterml",
					Name:    "exp1.asterml",
					Entry:   &vault.FileEntry{RelPath: "proj/exp1.asterml", Name: "exp1.asterml", Title: "细胞活力测定", Date: "2026-11-20"},
				},
			},
		},
		OnVaultDuplicate: func(relPath, targetDate string) { duplicated = append(duplicated, relPath) },
		OnVaultDelete:    func(relPath string) { deleted = append(deleted, relPath) },
	}
	s.OpenCalendarTab()

	var bottomReserve float32
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		bottomReserve = s.MobileBottomReserve(c)
		s.RenderSplitView(c)
	}, 390, 844)
	tt.SetInsets(44, 0, 34, 0)
	tt.Frame()
	advanceMobileFrames(tt, 260*time.Millisecond)

	if bottomReserve < 60 || bottomReserve > 72 {
		t.Fatalf("expected MobileBottomReserve inside root SafeArea to be in [60, 72] without 2x safeBottom, got %v", bottomReserve)
	}

	hdrR, ok := tt.Find("实验库")
	if !ok {
		t.Fatalf("expected 实验库 drawer header, texts=%v", tt.Texts())
	}
	if hdrR.Y < 44 || hdrR.Y > 72 {
		t.Fatalf("expected Left Drawer header to apply 1x top SafeArea (44..72), got %+v", hdrR)
	}

	rowRect, ok := tt.Find("exp1.asterml")
	if !ok {
		t.Fatalf("expected file row exp1.asterml in mobile drawer, texts=%v", tt.Texts())
	}
	y := rowRect.Y + rowRect.H/2
	// Swipe right to reveal Leading "置顶" action.
	tt.FingerDown(40, y)
	tt.Advance(16 * time.Millisecond)
	for x := float32(60); x <= 180; x += 25 {
		tt.FingerMove(x, y)
		tt.Frame()
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(150 * time.Millisecond)
	tt.FingerUp(180, y)
	advanceMobileFrames(tt, 600*time.Millisecond)

	if err := tt.Click("置顶"); err != nil {
		t.Fatalf("click SwipeRow 置顶 action: %v", err)
	}
	tt.Frame()
	if !s.IsFilePinned("proj/exp1.asterml") {
		t.Fatalf("expected proj/exp1.asterml to be pinned after SwipeRow tap")
	}

	// Swipe left to reveal Trailing "副本" / "删除" actions.
	rowRect, ok = tt.Find("exp1.asterml")
	if !ok {
		t.Fatalf("expected pinned file row exp1.asterml")
	}
	y = rowRect.Y + rowRect.H/2
	tt.FingerDown(260, y)
	tt.Advance(16 * time.Millisecond)
	for x := float32(240); x >= 60; x -= 30 {
		tt.FingerMove(x, y)
		tt.Frame()
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(150 * time.Millisecond)
	tt.FingerUp(60, y)
	advanceMobileFrames(tt, 600*time.Millisecond)

	if err := tt.Click("副本"); err != nil {
		t.Fatalf("click SwipeRow 副本 action: %v", err)
	}
	tt.Frame()
	if len(duplicated) != 1 || duplicated[0] != "proj/exp1.asterml" {
		t.Fatalf("expected duplicate callback for proj/exp1.asterml, got %v", duplicated)
	}
	_ = deleted
}

func TestMobileContextMenuLongPressActionSheet(t *testing.T) {
	chosen := ""
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		card := ui.Box(c.Key("mobile-ctx-target")).FillWidth().Height(80).Padding(16)
		card.Children(func() {
			ui.Text(c, "长按测试卡片")
		})
		kit.ContextMenu(c, card, func(m *kit.Menu) {
			if m.Item("复制实验").Chosen() {
				chosen = "copy"
			}
			if m.Item("移入回收站").Chosen() {
				chosen = "trash"
			}
		})
	}, 390, 844)
	tt.Frame()

	r, ok := tt.Find("长按测试卡片")
	if !ok {
		t.Fatalf("expected target card")
	}
	tt.FingerDown(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	tt.FingerUp(r.X+r.W/2, r.Y+r.H/2)
	advanceMobileFrames(tt, 600*time.Millisecond)

	if !tt.HasText("复制实验") || !tt.HasText("取消") {
		t.Fatalf("expected bottom ActionSheet to open on LongPressed, got texts=%v", tt.Texts())
	}
	itemRect, ok := tt.Find("复制实验")
	if !ok {
		t.Fatalf("expected 复制实验 row in sheet")
	}
	if itemRect.H < 48 {
		t.Fatalf("expected touch row height >= 48dp, got %v", itemRect.H)
	}
	tt.FingerDown(itemRect.X+itemRect.W/2, itemRect.Y+itemRect.H/2)
	tt.FingerUp(itemRect.X+itemRect.W/2, itemRect.Y+itemRect.H/2)
	tt.Frame()
	if chosen != "copy" {
		t.Fatalf("expected chosen='copy' on sheet tap, got %q", chosen)
	}
}

func TestMobileDefaultFileOpenModeLab(t *testing.T) {
	s := &Shell{}
	s.ForceMobilePreview = true
	if got := s.DefaultFileOpenMode(); got != ModeLab {
		t.Fatalf("expected mobile default file open mode ModeLab, got %q", got)
	}
	s.DefaultOpenMode = "edit"
	if got := s.DefaultFileOpenMode(); got != ModeEdit {
		t.Fatalf("expected explicit edit override to return ModeEdit, got %q", got)
	}
	s.ForceMobilePreview = false
	s.DefaultOpenMode = ""
	if got := s.DefaultFileOpenMode(); got != ModeEdit {
		t.Fatalf("expected desktop default file open mode ModeEdit, got %q", got)
	}

	s.ForceMobilePreview = true
	s.MobileMenuSheetOpen = true
	if !s.HandleMobileBack() {
		t.Fatal("expected HandleMobileBack to dismiss MobileMenuSheetOpen")
	}
	if s.MobileMenuSheetOpen {
		t.Fatal("expected MobileMenuSheetOpen to be false after HandleMobileBack")
	}
}

