package shell

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/vault"
)

const (
	mobileBreakpointWidth      = float32(680)
	mobilePullActionThresh     = float32(56)
	mobileDrawerWidthMax       = float32(320)
	mobileEdgeSwipeZoneWidth   = float32(18)
	mobileKeyboardInsetThresh  = float32(120)
	mobileToolbarConfigFile    = "mobile_toolbar.json"
)

func mobileDockBottomMargin() float32 {
	if mobile.Current() == mobile.Material {
		return 12
	}
	return 8
}

// MobileToolbarActionSpec describes one customizable button in the mobile edit toolbar.
type MobileToolbarActionSpec struct {
	ID             string
	Group          string
	Label          string
	Icon           *ui.SVG
	Accent         bool
	DispatchAction string
}

// MobileToolbarConfig is the persisted JSON payload for mobile edit toolbar ordering and visibility.
type MobileToolbarConfig struct {
	MobileToolbarOrder  []string        `json:"mobileToolbarOrder,omitempty"`
	MobileToolbarHidden map[string]bool `json:"mobileToolbarHidden,omitempty"`
	PinnedFiles         []string        `json:"pinnedFiles,omitempty"`
}

type mobileExtraState struct {
	order                  []string
	hidden                 map[string]bool
	pinnedFiles            map[string]bool
	pullWasReady           bool
	loadedVaultPath        string
	explicitConfig         bool
	calAgenda3Day          bool
	calAgendaInited        bool
	calModeInited          bool
	calStripPage           mobile.PageState
	calStripInited         bool
	canvasSwipeOffset           ui.Point
	drawerDragOffset            ui.Point
	drawerDragOpened            string
	drawerSkipCloseAnim         bool
	mobileDrawerFilterTemplates bool
	pullRefresh                 ui.RefreshState
	mobileSettingsDetail   bool
	mobileSettingsLastOpen bool
}

var (
	mobileExtraMu  sync.Mutex
	mobileExtraMap = map[*Shell]*mobileExtraState{}
)

func (s *Shell) mobileExtra() *mobileExtraState {
	mobileExtraMu.Lock()
	defer mobileExtraMu.Unlock()
	st, ok := mobileExtraMap[s]
	if !ok {
		st = &mobileExtraState{
			hidden:        map[string]bool{},
			pinnedFiles:   map[string]bool{},
			calAgenda3Day: true,
			calStripPage:  mobile.PageState{Page: 1},
		}
		mobileExtraMap[s] = st
	}
	return st
}

// DefaultMobileToolbarSpecs returns the canonical Obsidian-grade mobile edit toolbar actions.
func DefaultMobileToolbarSpecs() []MobileToolbarActionSpec {
	return []MobileToolbarActionSpec{
		// Scientific quick actions
		{ID: "at_entity", Group: "科研快捷", Label: "插入实体 @", Icon: lucide.AtSign()},
		{ID: "slash_module", Group: "科研快捷", Label: "插入模块 /", Icon: lucide.Slash()},
		{ID: "footnote", Group: "科研快捷", Label: "批注 [^]", Icon: lucide.MessageSquareQuote()},
		{ID: "nl_structure", Group: "科研快捷", Label: "自然语言转结构", Icon: lucide.Sparkles()},
		{ID: "camera_photo", Group: "科研快捷", Label: "现场拍照", Icon: lucide.Camera()},
		{ID: "timer", Group: "科研快捷", Label: "快捷计时", Icon: lucide.Timer(), DispatchAction: "open_timer"},
		{ID: "add_step", Group: "科研快捷", Label: "+步骤", Icon: lucide.ListPlus()},
		// Formatting & Structure
		{ID: "undo", Group: "排版与结构", Label: "撤销", Icon: lucide.Undo2()},
		{ID: "redo", Group: "排版与结构", Label: "重做", Icon: lucide.Redo2()},
		{ID: "heading", Group: "排版与结构", Label: "标题 H1–H3", Icon: lucide.Heading()},
		{ID: "bold", Group: "排版与结构", Label: "加粗", Icon: lucide.Bold()},
		{ID: "checklist", Group: "排版与结构", Label: "待办清单", Icon: lucide.ListChecks()},
		{ID: "bullet_list", Group: "排版与结构", Label: "无序列表", Icon: lucide.List()},
		{ID: "indent", Group: "排版与结构", Label: "增加缩进", Icon: lucide.ListIndentIncrease()},
		{ID: "outdent", Group: "排版与结构", Label: "减少缩进", Icon: lucide.ListIndentDecrease()},
		{ID: "move_block_up", Group: "排版与结构", Label: "上移块", Icon: lucide.ArrowUp()},
		{ID: "move_block_down", Group: "排版与结构", Label: "下移块", Icon: lucide.ArrowDown()},
		{ID: "fold_block", Group: "排版与结构", Label: "折叠块", Icon: lucide.FoldVertical()},
		// Mobile-native cursor/nav
		{ID: "cursor_left", Group: "光标与导航", Label: "光标左移", Icon: lucide.ChevronLeft()},
		{ID: "cursor_right", Group: "光标与导航", Label: "光标右移", Icon: lucide.ChevronRight()},
		{ID: "hide_keyboard", Group: "光标与导航", Label: "收起键盘", Icon: lucide.KeyboardOff()},
		{ID: "switch_nav", Group: "光标与导航", Label: "切换底部导航栏", Icon: lucide.LayoutGrid()},
	}
}

func normalizeToolbarActionID(id string) string {
	id = strings.TrimSpace(id)
	if id == "open_timer" {
		return "timer"
	}
	return id
}

func defaultToolbarSpecMap() map[string]MobileToolbarActionSpec {
	specs := DefaultMobileToolbarSpecs()
	m := make(map[string]MobileToolbarActionSpec, len(specs))
	for _, sp := range specs {
		m[sp.ID] = sp
	}
	return m
}

func normalizeMobileToolbarOrder(order []string) []string {
	defaults := DefaultMobileToolbarSpecs()
	specMap := defaultToolbarSpecMap()
	seen := make(map[string]bool, len(defaults))
	out := make([]string, 0, len(defaults))
	for _, raw := range order {
		id := normalizeToolbarActionID(raw)
		if _, ok := specMap[id]; ok && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, sp := range defaults {
		if !seen[sp.ID] {
			seen[sp.ID] = true
			out = append(out, sp.ID)
		}
	}
	return out
}

func (s *Shell) ensureToolbarConfigLoaded() {
	st := s.mobileExtra()
	vaultRoot := strings.TrimSpace(s.VaultPathInput)
	if vaultRoot != "" && st.loadedVaultPath != vaultRoot && !st.explicitConfig {
		_ = s.LoadMobileToolbarConfig()
	}
}

func (s *Shell) mobileToolbarConfigPath() string {
	root := strings.TrimSpace(s.VaultPathInput)
	if root == "" {
		return ""
	}
	return filepath.Join(root, vault.SystemDirName, mobileToolbarConfigFile)
}

// SaveMobileToolbarConfig persists the shell's mobile toolbar order, hidden actions,
// and pinned file paths into `<VaultRoot>/.asterlab/mobile_toolbar.json`.
func (s *Shell) SaveMobileToolbarConfig() error {
	p := s.mobileToolbarConfigPath()
	if p == "" {
		return nil
	}
	st := s.mobileExtra()
	order := normalizeMobileToolbarOrder(st.order)
	hidden := map[string]bool{}
	for k, v := range st.hidden {
		if v {
			hidden[normalizeToolbarActionID(k)] = true
		}
	}
	var pinned []string
	for k, v := range st.pinnedFiles {
		if v && k != "" {
			pinned = append(pinned, k)
		}
	}
	sort.Strings(pinned)
	payload := MobileToolbarConfig{
		MobileToolbarOrder:  order,
		MobileToolbarHidden: hidden,
		PinnedFiles:         pinned,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return os.WriteFile(p, data, 0o644)
	}
	return nil
}

// LoadMobileToolbarConfig reloads mobile toolbar ordering, visibility, and pinned file state from disk.
func (s *Shell) LoadMobileToolbarConfig() error {
	p := s.mobileToolbarConfigPath()
	if p == "" {
		return nil
	}
	st := s.mobileExtra()
	st.loadedVaultPath = strings.TrimSpace(s.VaultPathInput)
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	var payload MobileToolbarConfig
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	st.order = normalizeMobileToolbarOrder(payload.MobileToolbarOrder)
	st.hidden = map[string]bool{}
	for k, v := range payload.MobileToolbarHidden {
		if v {
			st.hidden[normalizeToolbarActionID(k)] = true
		}
	}
	st.pinnedFiles = map[string]bool{}
	for _, rel := range payload.PinnedFiles {
		if rel != "" {
			st.pinnedFiles[rel] = true
		}
	}
	return nil
}

// MobileToolbarOrder returns a copy of the ordered mobile toolbar action IDs.
func (s *Shell) MobileToolbarOrder() []string {
	s.ensureToolbarConfigLoaded()
	st := s.mobileExtra()
	return normalizeMobileToolbarOrder(st.order)
}

// SetMobileToolbarOrder updates the mobile edit toolbar order and persists it.
func (s *Shell) SetMobileToolbarOrder(order []string) {
	st := s.mobileExtra()
	st.order = normalizeMobileToolbarOrder(order)
	st.explicitConfig = true
	_ = s.SaveMobileToolbarConfig()
	s.notifySettingsChanged()
}

// MobileToolbarHidden returns a copy of the hidden action ID map.
func (s *Shell) MobileToolbarHidden() map[string]bool {
	s.ensureToolbarConfigLoaded()
	st := s.mobileExtra()
	out := make(map[string]bool, len(st.hidden))
	for k, v := range st.hidden {
		if v {
			out[k] = true
		}
	}
	return out
}

// SetMobileToolbarHidden replaces the hidden action map and persists it.
func (s *Shell) SetMobileToolbarHidden(hidden map[string]bool) {
	st := s.mobileExtra()
	st.hidden = map[string]bool{}
	for k, v := range hidden {
		if v {
			st.hidden[normalizeToolbarActionID(k)] = true
		}
	}
	st.explicitConfig = true
	_ = s.SaveMobileToolbarConfig()
	s.notifySettingsChanged()
}

// MobileToolbarOrderedSpecs returns all known toolbar actions in current customized order.
func (s *Shell) MobileToolbarOrderedSpecs() []MobileToolbarActionSpec {
	order := s.MobileToolbarOrder()
	specMap := defaultToolbarSpecMap()
	out := make([]MobileToolbarActionSpec, 0, len(order))
	for _, id := range order {
		if sp, ok := specMap[id]; ok {
			out = append(out, sp)
		}
	}
	return out
}

// IsMobileToolbarActionVisible reports whether an action ID is currently shown in the toolbar.
func (s *Shell) IsMobileToolbarActionVisible(id string) bool {
	s.ensureToolbarConfigLoaded()
	st := s.mobileExtra()
	return !st.hidden[normalizeToolbarActionID(id)]
}

// SetMobileToolbarActionVisible shows or hides a toolbar action and persists the change.
func (s *Shell) SetMobileToolbarActionVisible(id string, visible bool) {
	st := s.mobileExtra()
	norm := normalizeToolbarActionID(id)
	if st.hidden == nil {
		st.hidden = map[string]bool{}
	}
	if visible {
		delete(st.hidden, norm)
	} else {
		st.hidden[norm] = true
	}
	st.explicitConfig = true
	_ = s.SaveMobileToolbarConfig()
	s.notifySettingsChanged()
}

// MoveMobileToolbarAction moves an action up (delta < 0) or down (delta > 0) in toolbar order.
func (s *Shell) MoveMobileToolbarAction(id string, delta int) bool {
	if delta == 0 {
		return false
	}
	order := s.MobileToolbarOrder()
	norm := normalizeToolbarActionID(id)
	idx := -1
	for i, cur := range order {
		if cur == norm {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	target := idx + delta
	if target < 0 {
		target = 0
	}
	if target >= len(order) {
		target = len(order) - 1
	}
	if target == idx {
		return false
	}
	order[idx], order[target] = order[target], order[idx]
	s.SetMobileToolbarOrder(order)
	return true
}

// ResetMobileToolbarConfig restores the default action order and visibility.
func (s *Shell) ResetMobileToolbarConfig() {
	st := s.mobileExtra()
	st.order = normalizeMobileToolbarOrder(nil)
	st.hidden = map[string]bool{}
	st.explicitConfig = true
	_ = s.SaveMobileToolbarConfig()
	s.notifySettingsChanged()
}

// IsFilePinned reports whether a vault file is pinned in the mobile file tree.
func (s *Shell) IsFilePinned(relPath string) bool {
	s.ensureToolbarConfigLoaded()
	st := s.mobileExtra()
	return st.pinnedFiles[relPath]
}

// TogglePinFile toggles the pinned state for a vault file and persists it.
func (s *Shell) TogglePinFile(relPath string) bool {
	if relPath == "" {
		return false
	}
	st := s.mobileExtra()
	if st.pinnedFiles == nil {
		st.pinnedFiles = map[string]bool{}
	}
	next := !st.pinnedFiles[relPath]
	if next {
		st.pinnedFiles[relPath] = true
	} else {
		delete(st.pinnedFiles, relPath)
	}
	st.explicitConfig = true
	_ = s.SaveMobileToolbarConfig()
	return next
}

// HandleMobileBack dismisses one active mobile overlay/drawer/modal layer at a time,
// falling back to navigation history before returning false so the OS can background the app.
func (s *Shell) HandleMobileBack() bool {
	if s == nil {
		return false
	}
	if s.Layers.HasOpen() {
		return s.Layers.DismissTop()
	}
	switch {
	case s.PaletteOpen:
		s.PaletteOpen = false
		return true
	case s.GlobalSearchOpen:
		s.GlobalSearchOpen = false
		return true
	case s.MobileMenuSheetOpen:
		s.MobileMenuSheetOpen = false
		return true
	case s.VaultSettingsOpen:
		extra := s.mobileExtra()
		if extra.mobileSettingsDetail {
			extra.mobileSettingsDetail = false
			return true
		}
		s.VaultSettingsOpen = false
		extra.mobileSettingsLastOpen = false
		return true
	case s.newFolderDialogOpen:
		s.newFolderDialogOpen = false
		return true
	case s.VaultLauncherOpen:
		s.VaultLauncherOpen = false
		return true
	case s.FeedbackOpen:
		s.FeedbackOpen = false
		return true
	case s.WelcomeOpen:
		s.WelcomeOpen = false
		return true
	case s.TourOpen:
		s.TourOpen = false
		return true
	case s.DayPopoverOpen:
		s.DayPopoverOpen = false
		return true
	case s.MobileTabSwitcherOpen:
		s.MobileTabSwitcherOpen = false
		return true
	case s.MobileDrawer != "":
		s.MobileDrawer = ""
		s.MobileDrawerProgress = 0
		s.MobileSwipeKind = ""
		return true
	case s.CanBack():
		s.GoBack()
		return true
	default:
		return false
	}
}

// SetInitialMobilePrefs loads persisted mobile shell preferences without letting
// EnsureMobileDefaults overwrite explicitly saved false booleans back to true.
func (s *Shell) SetInitialMobilePrefs(pullAction string, autoHideDock, editToolbar bool) {
	if pullAction == "" {
		pullAction = "palette"
	}
	s.MobilePullAction = pullAction
	s.MobileAutoHideDock = autoHideDock
	s.MobileEditToolbar = editToolbar
	s.mobilePrefsInited = true
}

// EnsureMobileDefaults initializes Obsidian Mobile v2 shell defaults once without
// overriding explicit user preferences loaded from disk.
func (s *Shell) EnsureMobileDefaults() {
	if s.MobilePullAction == "" {
		s.MobilePullAction = "palette"
	}
	if !s.mobilePrefsInited {
		s.mobilePrefsInited = true
		if !s.MobileAutoHideDock {
			s.MobileAutoHideDock = true
		}
		if !s.MobileEditToolbar {
			s.MobileEditToolbar = true
		}
	}
}

// IsMobileLayout reports whether AsterLab should adapt its workspace chrome to
// single-pane touch-friendly mobile ergonomics (narrow viewport < 680dp, mobile OS,
// or user-toggled mobile preview in Settings).
func (s *Shell) IsMobileLayout(c *ui.Context) bool {
	if s.ForceMobilePreview || runtime.GOOS == "android" || runtime.GOOS == "ios" {
		return true
	}
	w, _ := c.Size()
	return w > 0 && w < mobileBreakpointWidth
}

// MobileContentScrollState returns the tracked ui.ScrollState for the currently
// active mobile tab so upward/downward scrolling can drive floating dock auto-hide
// and top pull-down Quick Action cleanly.
func (s *Shell) MobileContentScrollState() *ui.ScrollState {
	if s.mobileScrollStates == nil {
		s.mobileScrollStates = map[string]*ui.ScrollState{}
	}
	key := "default"
	if t := s.ActiveTab(); t != nil && t.ID != "" {
		key = t.ID
	}
	st, ok := s.mobileScrollStates[key]
	if !ok {
		st = &ui.ScrollState{}
		s.mobileScrollStates[key] = st
	}
	return st
}

// MobileTopReserve returns top clearance below the floating iOS/Android Liquid Glass controls
// (left drawer orb + right mode/more pill) when Liquid Glass is active.
// MyGo's root already applies system status/cutout SafeArea padding via padInsets, so this excludes SafeArea.
func (s *Shell) MobileTopReserve(c *ui.Context) float32 {
	tok := theme.GetTokens(c.Theme())
	if !tok.LiquidGlass {
		return 0
	}
	return 48
}

// MobileBottomReserve returns the bottom clearance inside the root SafeArea needed so scrolled
// content clears the floating bottom capsule cleanly without double-counting system bottom SafeArea.
func (s *Shell) MobileBottomReserve(c *ui.Context) float32 {
	_ = c
	return 48 + mobileDockBottomMargin() + 8
}

func (s *Shell) updateMobileScrollBehavior() {
	st := s.MobileContentScrollState()
	if s.mobileLastScrollY == nil {
		s.mobileLastScrollY = map[string]float32{}
	}
	key := "default"
	if t := s.ActiveTab(); t != nil && t.ID != "" {
		key = t.ID
	}
	prevY := s.mobileLastScrollY[key]
	curY := st.Y
	dy := curY - prevY
	s.mobileLastScrollY[key] = curY

	if !s.MobileAutoHideDock || s.MobileDrawer != "" || s.MobileTabSwitcherOpen {
		if !s.MobileAutoHideDock {
			s.MobileDockHidden = false
		}
		return
	}
	if curY <= 12 || dy < -4 {
		s.MobileDockHidden = false
	} else if dy > 6 && curY > 36 {
		s.MobileDockHidden = true
	}
}

func (s *Shell) mobilePullActionLabel() string {
	switch s.MobilePullAction {
	case "new_exp":
		return "新建今日实验"
	case "calc":
		return "速算与计时器"
	case "tree":
		return "展开实验库"
	default:
		return "命令面板 / 快速切换"
	}
}

func (s *Shell) executeMobilePullAction() {
	switch s.MobilePullAction {
	case "new_exp":
		s.MobileDrawer = ""
		s.MobileTabSwitcherOpen = false
		s.triggerVaultNewFile("")
	case "calc":
		s.RightTab = 1
		s.MobileDrawer = "outline"
	case "tree":
		s.MobileDrawer = "tree"
	default:
		s.MobileDrawer = ""
		s.MobileTabSwitcherOpen = false
		s.MobileMenuSheetOpen = false
		s.PaletteOpen = true
		s.PaletteCursor = 0
	}
}

// HandleMobilePullRefresh wires native OS pull-to-refresh (iOS rubber band / Android overscroll)
// on top-level mobile scroll views to trigger the configured Quick Action.
func (s *Shell) HandleMobilePullRefresh(c *ui.Context, el ui.Element) {
	if s == nil || c == nil || !s.IsMobileLayout(c) {
		return
	}
	extra := s.mobileExtra()
	if el.PullToRefresh(&extra.pullRefresh) {
		extra.pullRefresh.Refreshing = false
		mygo.Haptics.Success()
		s.executeMobilePullAction()
	}
}

type mobileSwipeBackKey struct{ id string }
type mobileSwipeBackShownKey struct{ id string }

// renderMobileSwipeBackContainer wraps a full-screen mobile page in ui.Overlay with
// native horizontal right-swipe-to-go-back physics.
func (s *Shell) renderMobileSwipeBackContainer(
	c *ui.Context,
	key string,
	tok theme.Tokens,
	onBack func(),
	content func(),
) {
	vw, _ := c.Size()
	if vw <= 0 {
		vw = 402
	}
	offset := ui.Local(c.Root(), mobileSwipeBackKey{key}, func() ui.Point { return ui.Point{} })
	lastShownX := ui.Local(c.Root(), mobileSwipeBackShownKey{key}, func() float32 { return 0 })
	// Emit backdrop scrim BEFORE panel in Z-order so full-screen overlays keep full brightness.
	scrimAlpha := float32(1) - max(0, min(1, (*lastShownX)/max(vw, 1)))
	ui.Box(c.Key(key + "-scrim")).Absolute().Fill().PassThrough().
		Background(tok.Scrim.Alpha(scrimAlpha)).
		Transition(theme.MobileOverlayFadeTransition())

	panel := ui.Column(c.Key(key)).Absolute().Fill().Modal().Background(tok.Canvas).
		Shadow(0, 8, 32, 0, tok.ShadowColor).
		Transition(theme.MobilePagePushTransition())
	if onBack != nil {
		panel.Draggable(offset, ui.DraggableOptions{
			Axis:      ui.Horizontal,
			Bounds:    &ui.Rect{X: 0, W: vw},
			Rigid:     true,
			Animation: theme.MotionPagePush,
			Snap:      ui.Nearest(ui.Point{}, ui.Point{X: vw}),
		})
		shownX := panel.DragOffset().X
		*lastShownX = shownX
		if !panel.Dragging() && offset.X > vw*0.45 {
			*offset = ui.Point{}
			*lastShownX = 0
			mygo.Haptics.Light()
			onBack()
			return
		}
		if !panel.Dragging() && !panel.Transitioning() && offset.X != 0 {
			*offset = ui.Point{}
		}
	}
	panel.Children(content)
}

func (s *Shell) renderMobileWorkbench(c *ui.Context, tok theme.Tokens) {
	s.EnsureTabs()
	s.EnsureMobileDefaults()
	if ex := s.mobileExtra(); !ex.calModeInited {
		ex.calModeInited = true
		if s.CalendarMode == 0 {
			s.CalendarMode = 1
		}
	}
	s.ensureToolbarConfigLoaded()
	tok = s.SyncThemeLook(c)
	s.updateMobileScrollBehavior()

	if c.Shortcut(0, ui.KeyEscape) {
		s.HandleMobileBack()
	}

	// MyGo's runtime padInsets already pads the window root for OS SafeArea bars/cutouts/IME.
	// Only add sidebar.HeaderHeight when a desktop window is narrowed into mobile preview.
	var topInset float32
	if s.window != nil {
		topInset = sidebar.HeaderHeight(c)
	}
	activeTab := s.ActiveTab()
	activePane := s.findPane(s.ActivePaneID)
	if activePane == nil {
		activePane = s.firstLeaf(s.RootPane)
	}
	vw, _ := c.Size()
	sheetMaxW := mobileDrawerWidthMax
	if vw > 0 && vw*0.84 < sheetMaxW {
		sheetMaxW = vw * 0.84
	}
	if sheetMaxW < 240 {
		sheetMaxW = 240
	}

	if s.MobilePullOffset >= mobilePullActionThresh {
		s.MobilePullOffset = 0
		s.executeMobilePullAction()
	}
	extra := s.mobileExtra()

	rootBox := ui.Box(c.Key("mobile-workbench-root")).Fill().Padding(topInset, 0, 0, 0)
	c.Root().Background(tok.Canvas)
	theme.PaintCanvasBackground(rootBox, tok)
	rootBox.Children(func() {
		ui.Column(c).Fill().Children(func() {
			if !tok.LiquidGlass {
				s.renderMobileDockedTopChrome(c, tok, activeTab)
			}

			bodyRoot := ui.Box(c.Key("mobile-body-stage")).FillWidth().Grow(1).MinHeight(0)
			canSwipeCanvas := s.MobileDrawer == "" && !s.MobileTabSwitcherOpen && !s.MobileMenuSheetOpen && !s.AnyModalOpen()
			var shownX float32
			if !canSwipeCanvas {
				extra.canvasSwipeOffset = ui.Point{}
			} else {
				bodyRoot.Draggable(&extra.canvasSwipeOffset, ui.DraggableOptions{
					Axis:      ui.Horizontal,
					Bounds:    &ui.Rect{X: -sheetMaxW, W: 2 * sheetMaxW},
					Rigid:     true,
					Animation: theme.MotionDrawer,
					Snap:      ui.Nearest(ui.Point{X: -sheetMaxW}, ui.Point{}, ui.Point{X: sheetMaxW}),
				})
				shownX = bodyRoot.DragOffset().X
				if bodyRoot.Dragging() {
					switch {
					case shownX > 6:
						s.MobileSwipeKind = "tree"
						s.MobileDrawerProgress = min(1, shownX/sheetMaxW)
					case shownX < -6:
						s.MobileSwipeKind = "outline"
						s.MobileDrawerProgress = min(1, (-shownX)/sheetMaxW)
					default:
						s.MobileSwipeKind = ""
						s.MobileDrawerProgress = 0
					}
				} else if s.MobileSwipeKind != "" || extra.canvasSwipeOffset.X != 0 {
					switch {
					case extra.canvasSwipeOffset.X > sheetMaxW*0.5 || shownX > sheetMaxW*0.32:
						s.MobileDrawer = "tree"
						s.MobileDrawerProgress = min(1, max(0, shownX/sheetMaxW))
						extra.canvasSwipeOffset = ui.Point{}
						s.MobileSwipeKind = ""
						extra.drawerSkipCloseAnim = false
						mygo.Haptics.Light()
					case extra.canvasSwipeOffset.X < -sheetMaxW*0.5 || shownX < -sheetMaxW*0.32:
						s.MobileDrawer = "outline"
						s.MobileDrawerProgress = min(1, max(0, (-shownX)/sheetMaxW))
						extra.canvasSwipeOffset = ui.Point{}
						s.MobileSwipeKind = ""
						extra.drawerSkipCloseAnim = false
						mygo.Haptics.Light()
					case !bodyRoot.Transitioning():
						s.MobileSwipeKind = ""
						s.MobileDrawerProgress = 0
						extra.canvasSwipeOffset = ui.Point{}
					}
				}
			}
			bodyRoot.Children(func() {
				// Counter-translate horizontal drag offset so document text stays stationary
				// while the drawer overlay tracks the finger 1:1 from millimeter 1.
				stage := ui.Box(c.Key("mb-body-static")).Fill()
				if shownX != 0 {
					stage.Translate(-shownX, 0)
				}
				stage.Children(func() {
					// 1A. Primary Tab Content
					if activeTab != nil {
						switch activeTab.Kind {
						case TabCalendar:
							s.calendarHomeView(c)
						case TabPantry:
							if s.RenderPantryContent != nil {
								s.RenderPantryContent(c)
							} else if s.Content != nil {
								s.Content(c)
							}
						default:
							if s.RenderTabContent != nil && activePane != nil {
								s.RenderTabContent(c, activePane, *activeTab)
							} else if s.Content != nil {
								s.Content(c)
							}
						}
					}
				})

				// 1B. Floating Obsidian iOS 26 Top Bar (Open center, left circular orb + right twin-icon pill)
				if tok.LiquidGlass {
					topBar := ui.Box(c.Key("mb-top-bar-static")).Absolute().Top(0).Left(0).Right(0).PassThrough()
					if shownX != 0 {
						topBar.Translate(-shownX, 0)
					}
					topBar.Children(func() {
						s.renderMobileGlassTopBar(c, tok, activeTab)
					})
				}

				// 1C. Centered Compact Icon-Only Liquid Glass Bottom Capsule
				dockStatic := ui.Box(c.Key("mb-dock-static")).Absolute().Fill().PassThrough()
				if shownX != 0 {
					dockStatic.Translate(-shownX, 0)
				}
				dockStatic.Children(func() {
					s.renderMobileFloatingBottomBar(c, tok, bodyRoot, activePane, activeTab)
				})
			})
		})

		// 2. Interactive Symmetric Full-Canvas / Swipe-Close Drawers ("tree" or "outline")
		s.renderMobileSwipeDrawers(c, tok, sheetMaxW)

		// 3. Obsidian-style Tab Switcher Card Overview
		if s.MobileTabSwitcherOpen {
			s.renderMobileTabSwitcherOverlay(c, tok, activePane, activeTab)
		}

		// 4. Obsidian iOS Ribbon Quick Options Bottom Sheet (≡ button)
		s.renderMobileRibbonMenuSheet(c, tok)
	})

	s.renderCommandPalette(c)
	s.renderGlobalSearchModal(c)
	s.renderVaultDialogs(c)
	s.renderFeedbackDialog(c)
	s.renderWelcomeDialog(c)
	s.renderSpotlightTour(c, tok)
	if s.RenderOverlays != nil {
		s.RenderOverlays(c)
	}
	if s.Toasts != nil {
		s.Toasts.Render(c, tok)
	}
}

func (s *Shell) renderMobileGlassTopBar(c *ui.Context, tok theme.Tokens, activeTab *WorkspaceTab) {
	// Soft atmospheric top scroll feather so scrolled headings melt gently at top edge
	components.ScrollEdge(c, components.ScrollEdgeOptions{
		Style:      components.ScrollEdgeSoft,
		Bottom:     false,
		Extent:     48,
		Background: tok.Canvas,
	})

	// Left: Standalone 40x40 circular Liquid Glass sidebar button (matches Obsidian iOS)
	leftOrb := ui.ButtonBase(c.Key("mb-top-sidebar")).Absolute().Top(float32(tok.Spacing4)).Left(14).
		Size(40, 40).Radius(float32(tok.PillRadius)).Center().Label("打开实验库抽屉")
	theme.ApplyCapsuleGlass(leftOrb, tok, true)
	theme.ApplyTactilePress(leftOrb)
	leftOrb.Children(func() {
		ui.Icon(c, lucide.PanelLeft()).Size(18, 18).TextColor(tok.Text)
	})
	if leftOrb.Clicked() {
		if s.MobileDrawer == "tree" {
			s.MobileDrawer = ""
		} else {
			s.MobileDrawer = "tree"
		}
	}

	// Right: Clean twin-icon merged Liquid Glass pill (Zero nested boxes!)
	rightPill := ui.Row(c.Key("mb-top-right-pill")).Absolute().Top(float32(tok.Spacing4)).Right(14).
		Height(40).Padding(0, float32(tok.Spacing8)).Radius(float32(tok.PillRadius)).AlignItems(ui.Center).Gap(2)
	theme.ApplyCapsuleGlass(rightPill, tok, false)
	rightPill.Children(func() {
		pillIcon := func(key string, icon *ui.SVG, label string, active bool, onClick func()) ui.Element {
			btn := ui.ButtonBase(c.Key("mb-tr-"+key)).Size(36, 36).Radius(float32(tok.PillRadius)).Center().Label(label)
			if btn.Hovered() {
				btn.Background(tok.Text.Alpha(0.05))
			}
			theme.ApplyTactilePress(btn)
			fg := tok.Text
			if active {
				fg = tok.Accent
			}
			btn.Children(func() {
				ui.Icon(c, icon).Size(18, 18).TextColor(fg)
			})
			if btn.Clicked() {
				onClick()
			}
			return btn
		}

		if activeTab != nil && activeTab.Kind == TabFile {
			if activeTab.Mode == ModeEdit {
				pillIcon("mode-lab", lucide.FlaskConical(), "切换到实验模式（防误触）", false, func() {
					s.MobileDockForceNav = false
					s.SetActiveTabMode(ModeLab)
				})
			} else {
				pillIcon("mode-edit", lucide.PenLine(), "切换到编辑态", false, func() {
					s.MobileDockForceNav = false
					s.SetActiveTabMode(ModeEdit)
				})
			}
			moreB := ui.ButtonBase(c.Key("mb-tr-more")).Size(36, 36).Radius(float32(tok.PillRadius)).Center().Label("文档与更多操作")
			if moreB.Hovered() {
				moreB.Background(tok.Text.Alpha(0.05))
			}
			theme.ApplyTactilePress(moreB)
			moreB.Children(func() {
				ui.Icon(c, lucide.Ellipsis()).Size(18, 18).TextColor(tok.Text)
			})
			s.attachMobileDocMenu(c, moreB, activeTab)
		} else {
			pillIcon("calc", lucide.Calculator(), "速算与计时器", s.MobileDrawer == "outline", func() {
				s.RightTab = 1
				if s.MobileDrawer == "outline" {
					s.MobileDrawer = ""
				} else {
					s.MobileDrawer = "outline"
				}
			})
			pillIcon("search", lucide.Search(), "快速打开 (Ctrl+K)", false, func() {
				s.PaletteOpen = true
				s.PaletteCursor = 0
			})
		}
	})
}

func (s *Shell) attachMobileDocMenu(c *ui.Context, trigger ui.Element, activeTab *WorkspaceTab) {
	components.MenuButton(c, trigger, func(m *components.Menu) {
		if m.Item("步骤大纲与速算").Chosen() {
			s.MobileDrawer = "outline"
		}
		if m.Item("快速打开命令 (Ctrl+K)").Chosen() {
			s.PaletteOpen = true
			s.PaletteCursor = 0
		}
		m.Separator()
		srcLbl := "查看 AsterML 源码"
		if activeTab.Mode == ModeSource {
			srcLbl = "返回可视化编辑"
		}
		if m.Item(srcLbl).Chosen() {
			if s.OnMobileDocToolbarAction != nil {
				s.OnMobileDocToolbarAction(*activeTab, "source_mode")
			} else if activeTab.Mode == ModeSource {
				s.SetActiveTabMode(ModeEdit)
			} else {
				s.SetActiveTabMode(ModeSource)
			}
		}
		if s.OnMobileDocToolbarAction != nil {
			if m.Item("复制 AsterML 原文").Chosen() {
				s.OnMobileDocToolbarAction(*activeTab, "copy_source")
			}
			if m.Item("导出 PDF…").Chosen() {
				s.OnMobileDocToolbarAction(*activeTab, "export_pdf")
			}
			if m.Item("另存为模板").Chosen() {
				s.OnMobileDocToolbarAction(*activeTab, "save_template")
			}
			if m.Item("创建实验副本").Chosen() {
				s.OnMobileDocToolbarAction(*activeTab, "duplicate")
			}
			m.Separator()
			if m.Item("移入回收站").Chosen() {
				s.OnMobileDocToolbarAction(*activeTab, "delete")
			}
		}
	})
}

func (s *Shell) renderMobileDockedTopChrome(c *ui.Context, tok theme.Tokens, activeTab *WorkspaceTab) {
	title := "实验日程"
	if activeTab != nil && activeTab.Title != "" {
		title = activeTab.Title
	}
	ui.Row(c).FillWidth().Height(46).Padding(float32(tok.Spacing4), float32(tok.Spacing12)).
		Background(tok.Surface).
		AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
		treeBtn := components.IconButton(c, lucide.PanelLeft(), "打开实验库抽屉", components.IconButtonCompact)
		if treeBtn.Clicked() {
			if s.MobileDrawer == "tree" {
				s.MobileDrawer = ""
			} else {
				s.MobileDrawer = "tree"
			}
		}

		components.Text(c, title).FontSize(float32(tok.TextBody)).Bold().
			TextColor(tok.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)

		if activeTab != nil && activeTab.Kind == TabFile {
			if activeTab.Mode == ModeEdit {
				labBtn := components.IconButton(c, lucide.FlaskConical(), "切换到实验模式（防误触）", components.IconButtonCompact)
				if labBtn.Clicked() {
					s.MobileDockForceNav = false
					s.SetActiveTabMode(ModeLab)
				}
			} else {
				editBtn := components.IconButton(c, lucide.PenLine(), "切换到编辑态", components.IconButtonCompact)
				if editBtn.Clicked() {
					s.MobileDockForceNav = false
					s.SetActiveTabMode(ModeEdit)
				}
			}
			moreBtn := components.IconButton(c, lucide.Ellipsis(), "文档与更多操作", components.IconButtonCompact)
			s.attachMobileDocMenu(c, moreBtn, activeTab)
		} else {
			if components.IconButton(c, lucide.Search(), "快速打开 (Ctrl+K)", components.IconButtonCompact).Clicked() {
				s.PaletteOpen = true
				s.PaletteCursor = 0
			}
		}
	})
	ui.Box(c).FillWidth().Height(1).Background(tok.Border)
}

func (s *Shell) renderMobileRibbonMenuSheet(c *ui.Context, tok theme.Tokens) {
	mobile.Sheet(c, &s.MobileMenuSheetOpen, mobile.SheetOptions{
		Detents:    []float32{0.68, 0.90},
		Background: &tok.Surface,
		Label:      "功能菜单",
	}, func() {
		type ribbonAction struct {
			key   string
			icon  *ui.SVG
			label string
			run   func()
		}
		items := []ribbonAction{
			{
				key: "quick-switcher", icon: lucide.FileSearch(), label: "打开快速切换",
				run: func() { s.PaletteMode = "files"; s.PaletteOpen = true; s.PaletteCursor = 0 },
			},
			{
				key: "global-search", icon: lucide.Search(), label: "全局全文检索",
				run: func() { s.GlobalSearchOpen = true },
			},
			{
				key: "calendar", icon: lucide.Calendar(), label: "实验日程与日历",
				run: func() { s.OpenCalendarTab() },
			},
			{
				key: "new-exp", icon: lucide.FilePlus(), label: "新建今日实验",
				run: func() { s.triggerVaultNewFile("") },
			},
			{
				key: "pantry", icon: lucide.Package(), label: "储藏室与动物房",
				run: func() { s.OpenPantryTab() },
			},
			{
				key: "outline-calc", icon: lucide.Calculator(), label: "步骤大纲与速算计时",
				run: func() { s.RightTab = 1; s.MobileDrawer = "outline" },
			},
			{
				key: "palette", icon: lucide.Terminal(), label: "打开命令面板",
				run: func() { s.PaletteMode = "commands"; s.PaletteOpen = true; s.PaletteCursor = 0 },
			},
			{
				key: "vault-switcher", icon: lucide.FolderKanban(), label: "切换实验库",
				run: func() { s.VaultLauncherOpen = true },
			},
			{
				key: "settings", icon: lucide.Settings(), label: "云同步与设置",
				run: func() { s.VaultSettingsOpen = true },
			},
			{
				key: "feedback", icon: lucide.LifeBuoy(), label: "反馈与问题诊断",
				run: func() { s.FeedbackOpen = true },
			},
		}

		ui.Scroll(c.Key("mb-ribbon-sheet-scroll")).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().
				Padding(float32(tok.Spacing8), float32(tok.Spacing16), float32(tok.Spacing24), float32(tok.Spacing16)).
				Children(func() {
				ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Clip().Children(func() {
					for i, item := range items {
						item := item
						if i > 0 {
							ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
						}
						row := ui.ButtonBase(c.Key("mb-ribbon-"+item.key)).FillWidth().
							MinHeight(float32(tok.Spacing48)).
							Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
							Justify(ui.Start).Label(item.label).
							OnClick(func() {
								mygo.Haptics.Selection()
								s.MobileMenuSheetOpen = false
								item.run()
							})
						theme.ApplyTactilePress(row)
						row.Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
								kit.IconBox(c, tok, item.icon, tok.TextMuted)
								ui.Text(c, item.label).FontSize(float32(tok.TextBody)).TextColor(tok.Text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
							})
						})
					}
				})
			})
		})
	})
}

func (s *Shell) renderMobileFloatingBottomBar(
	c *ui.Context,
	tok theme.Tokens,
	stage ui.Element,
	activePane *PaneNode,
	activeTab *WorkspaceTab,
) {
	_, _, safeBottom, _ := c.SafeArea()
	keyboardOpen := safeBottom > mobileKeyboardInsetThresh || s.MobileKeyboardOpen
	editingBar := s.MobileEditToolbar &&
		activeTab != nil &&
		activeTab.Kind == TabFile &&
		activeTab.Mode == ModeEdit &&
		!s.MobileDockForceNav &&
		keyboardOpen

	targetHide := float32(0)
	if s.MobileDockHidden && !editingBar {
		targetHide = 1
	}
	hideT := stage.AnimateTo("mb-dock-hide", targetHide, theme.MotionInline)

	if safeBottom > mobileKeyboardInsetThresh && !editingBar {
		return
	}
	bottomMargin := mobileDockBottomMargin()
	if hideT > 0.85 {
		pill := ui.ButtonBase(c.Key("mb-dock-reveal")).Absolute().
			Left(120).Right(120).Bottom(4).Height(18).Center().Label("显示导航栏")
		pill.Children(func() {
			ui.Box(c).Width(42).Height(4).Radius(float32(tok.PillRadius)).Background(tok.TextMuted.Alpha(0.45))
		})
		if pill.Clicked() {
			s.MobileDockHidden = false
		}
	}

	tabCount := 1
	if activePane != nil && len(activePane.Tabs) > 0 {
		tabCount = len(activePane.Tabs)
	}

	// Centered compact floating capsule positioned by platform HIG/M3 spacing above root SafeArea.
	// Keep mounted while translated offscreen so glass pipelines are not torn down mid-scroll.
	vw, _ := c.Size()
	maxCapsuleW := float32(360)
	if vw > 48 {
		maxCapsuleW = vw - 32
	}
	ui.Row(c.Key("mb-dock-wrap")).Absolute().Left(16).Right(16).Bottom(bottomMargin).Center().PassThrough().Children(func() {
		capsule := ui.Row(c.Key("mb-dock-capsule")).Height(48).MaxWidth(maxCapsuleW).Radius(float32(tok.PillRadius)).
			Padding(0, float32(tok.Spacing12)).AlignItems(ui.Center).Gap(4).Clip().
			Translate(0, hideT*78).
			Opacity(1 - hideT*0.92)
		if hideT > 0.85 {
			capsule.PassThrough()
		}
		theme.ApplyCapsuleGlass(capsule, tok, false)
		capsule.Children(func() {
			if editingBar {
				ui.Row(c.Key("mb-dock-edit-inner")).AlignItems(ui.Center).Gap(4).
					Transition(theme.MobileDockCrossfadeTransition()).Children(func() {
					s.renderMobileEditToolbarCapsule(c, tok, maxCapsuleW-24, activeTab)
				})
				return
			}
			ui.Row(c.Key("mb-dock-nav-inner")).AlignItems(ui.Center).Gap(4).
				Transition(theme.MobileDockCrossfadeTransition()).Children(func() {
				s.renderMobileNavDockItems(c, tok, activeTab, tabCount)
			})
		})
	})
}

func (s *Shell) renderMobileNavDockItems(c *ui.Context, tok theme.Tokens, activeTab *WorkspaceTab, tabCount int) {
	navBtn := func(key, label string, enabled, active bool, custom func(fg ui.Color), onClick func()) {
		b := ui.ButtonBase(c.Key("mb-nav-"+key)).Size(40, 40).Radius(float32(tok.PillRadius)).Center().
			Disabled(!enabled).Label(label)
		if b.Hovered() && enabled {
			b.Background(tok.Text.Alpha(0.06))
		} else if active {
			b.Background(tok.Text.Alpha(0.07))
		}
		theme.ApplyTactilePress(b)
		fg := tok.Text
		if !enabled {
			if tok.Dark {
				fg = tok.TextMuted.Alpha(0.45)
			} else {
				fg = tok.TextMuted.Alpha(0.35)
			}
		} else if active {
			fg = tok.Accent
		}
		b.Children(func() {
			custom(fg)
		})
		if enabled && b.Clicked() {
			onClick()
		}
	}

	// 1. Back (<)
	navBtn("back", "后退", s.CanBack(), false, func(fg ui.Color) {
		ui.Icon(c, lucide.ChevronLeft()).Size(20, 20).TextColor(fg)
	}, func() { s.GoBack() })

	// 2. Forward (>)
	navBtn("fwd", "前进", s.CanForward(), false, func(fg ui.Color) {
		ui.Icon(c, lucide.ChevronRight()).Size(20, 20).TextColor(fg)
	}, func() { s.GoForward() })

	// 3. Search / Quick Switcher (🔍)
	navBtn("search", "搜索", true, s.PaletteOpen, func(fg ui.Color) {
		ui.Icon(c, lucide.Search()).Size(19, 19).TextColor(fg)
	}, func() {
		s.MobileDrawer = ""
		s.MobileTabSwitcherOpen = false
		s.PaletteOpen = true
		s.PaletteCursor = 0
	})

	// 4. New Experiment (+)
	navBtn("new", "新建", true, false, func(fg ui.Color) {
		ui.Icon(c, lucide.Plus()).Size(21, 21).TextColor(fg)
	}, func() {
		s.MobileDrawer = ""
		s.MobileTabSwitcherOpen = false
		s.MobileDockForceNav = false
		s.triggerVaultNewFile("")
	})

	// 5. Tab Counter Box ([N]) - faithful to Obsidian iOS Tab Switcher glyph!
	navBtn("tabs", "标签", true, s.MobileTabSwitcherOpen, func(fg ui.Color) {
		ui.Box(c).Size(20, 20).Radius(float32(tok.ControlRadius)).Border(1.6, fg).Center().Children(func() {
			components.Text(c, fmt.Sprintf("%d", tabCount)).
				FontSize(float32(tok.TextMeta)).Bold().TextColor(fg).FontFeatures("tnum").SingleLine()
		})
	}, func() {
		s.MobileDrawer = ""
		s.MobileTabSwitcherOpen = !s.MobileTabSwitcherOpen
	})

	// 6. Ribbon Quick Options Sheet (≡) - matches Obsidian iOS bottom quick actions menu
	navBtn("menu", "功能菜单", true, s.MobileMenuSheetOpen, func(fg ui.Color) {
		ui.Icon(c, lucide.Menu()).Size(20, 20).TextColor(fg)
	}, func() {
		s.MobileDrawer = ""
		s.MobileTabSwitcherOpen = false
		s.MobileMenuSheetOpen = !s.MobileMenuSheetOpen
	})
}

func (s *Shell) renderMobileEditToolbarCapsule(c *ui.Context, tok theme.Tokens, innerW float32, activeTab *WorkspaceTab) {
	showSwitchNav := s.IsMobileToolbarActionVisible("switch_nav")
	scrollMaxW := innerW
	if showSwitchNav && innerW > float32(tok.Spacing48) {
		avail := innerW - float32(tok.Spacing48) - float32(tok.Spacing4)
		// Align scroll viewport width to full 40dp button strides so the next icon never peeks as a 3px dot.
		stride := float32(36 + tok.Spacing4)
		fit := float32(int(avail/stride)) * stride
		if fit >= stride*3 {
			scrollMaxW = fit + float32(tok.Spacing4)
		} else {
			scrollMaxW = avail
		}
	}
	ui.ScrollHorizontal(c.Key("mb-edit-tb-scroll")).MaxWidth(scrollMaxW).Height(40).Scrollbars(false).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			s.renderMobileEditToolbarItems(c, tok, activeTab)
		})
	})
	if showSwitchNav {
		ui.Box(c).Width(1).Height(float32(tok.Spacing16)).Background(tok.BorderStrong.Alpha(0.55))
		s.renderMobileToolbarButton(c, tok, activeTab, MobileToolbarActionSpec{
			ID: "switch_nav", Label: "切换底部导航栏", Icon: lucide.LayoutGrid(),
		})
	}
}

func (s *Shell) renderMobileToolbarButton(c *ui.Context, tok theme.Tokens, activeTab *WorkspaceTab, spec MobileToolbarActionSpec) {
	b := ui.ButtonBase(c.Key("mb-ed-"+spec.ID)).Size(36, 36).Radius(float32(tok.PillRadius)).Center().Label(spec.Label)
	if b.Hovered() {
		b.Background(tok.Text.Alpha(0.06))
	}
	theme.ApplyTactilePress(b)
	fg := tok.Text
	if spec.Accent {
		fg = tok.Accent
	}
	b.Children(func() {
		ui.Icon(c, spec.Icon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(fg)
	})
	if b.Clicked() {
		mygo.Haptics.Selection()
		s.dispatchMobileToolbarAction(c, activeTab, spec)
	}
}

func (s *Shell) dispatchMobileToolbarAction(c *ui.Context, activeTab *WorkspaceTab, spec MobileToolbarActionSpec) {
	act := spec.DispatchAction
	if act == "" {
		act = spec.ID
	}
	switch spec.ID {
	case "switch_nav":
		s.MobileDockForceNav = true
		return
	case "hide_keyboard":
		if c != nil {
			c.Root().Focus(ui.FocusOptions{PreventScroll: true})
		}
		if s.OnMobileDocToolbarAction != nil && activeTab != nil {
			s.OnMobileDocToolbarAction(*activeTab, act)
		}
		return
	case "undo":
		if s.OnMobileDocToolbarAction != nil && activeTab != nil {
			s.OnMobileDocToolbarAction(*activeTab, act)
		} else if s.OnUndo != nil {
			s.OnUndo()
		}
		return
	case "redo":
		if s.OnMobileDocToolbarAction != nil && activeTab != nil {
			s.OnMobileDocToolbarAction(*activeTab, act)
		} else if s.OnRedo != nil {
			s.OnRedo()
		}
		return
	case "timer":
		if s.OnMobileDocToolbarAction != nil && activeTab != nil {
			s.OnMobileDocToolbarAction(*activeTab, act)
		} else {
			s.RightTab = 1
			s.MobileDrawer = "outline"
		}
		return
	default:
		if s.OnMobileDocToolbarAction != nil && activeTab != nil {
			s.OnMobileDocToolbarAction(*activeTab, act)
		}
	}
}

func (s *Shell) renderMobileEditToolbarItems(c *ui.Context, tok theme.Tokens, activeTab *WorkspaceTab) {
	specs := s.MobileToolbarOrderedSpecs()
	prevGroup := ""
	renderedAny := false
	for _, spec := range specs {
		if spec.ID == "switch_nav" {
			continue
		}
		if !s.IsMobileToolbarActionVisible(spec.ID) {
			continue
		}
		if renderedAny && prevGroup != "" && spec.Group != prevGroup {
			ui.Box(c).Width(1).Height(float32(tok.Spacing16)).Background(tok.BorderStrong.Alpha(0.55))
		}
		s.renderMobileToolbarButton(c, tok, activeTab, spec)
		prevGroup = spec.Group
		renderedAny = true
	}
}

func (s *Shell) renderMobileSwipeDrawers(c *ui.Context, tok theme.Tokens, sheetMaxW float32) {
	extra := s.mobileExtra()
	activeKind := s.MobileDrawer
	if activeKind == "" && s.MobileSwipeKind != "" && s.MobileDrawerProgress > 0.01 {
		activeKind = s.MobileSwipeKind
	}
	if activeKind != "" {
		s.mobileLastDrawerKind = activeKind
		if s.MobileDrawer != "" {
			extra.drawerSkipCloseAnim = false
		}
	}
	targetP := float32(0)
	anim := theme.MotionDrawer
	if s.MobileDrawer != "" {
		targetP = 1
	} else if s.MobileSwipeKind != "" {
		targetP = s.MobileDrawerProgress
		anim = ui.Animation{Duration: time.Millisecond}
	} else if extra.drawerSkipCloseAnim {
		anim = ui.Animation{Duration: time.Millisecond}
	}

	animHost := ui.Box(c.Key("mb-drawer-host")).Absolute().Size(0, 0).PassThrough()
	pAnim := animHost.AnimateTo("mb-drawer-p", targetP, anim)
	if s.MobileSwipeKind != "" {
		pAnim = targetP
	} else if extra.drawerSkipCloseAnim && s.MobileDrawer == "" {
		pAnim = 0
	}
	if activeKind == "" && pAnim <= 0.01 {
		s.mobileLastDrawerKind = ""
		s.MobileDrawerProgress = 0
		extra.drawerDragOffset = ui.Point{}
		extra.drawerDragOpened = ""
		extra.drawerSkipCloseAnim = false
		return
	}
	if activeKind == "" {
		activeKind = s.mobileLastDrawerKind
		if activeKind == "" {
			activeKind = "tree"
		}
	}

	ui.Overlay(c, func() {
		root := ui.Box(c.Key("mb-drawer-overlay")).Absolute().Fill().PassThrough().Clip()
		root.Children(func() {
			isLeft := activeKind == "tree"
			backdrop := ui.Box(c.Key("mb-drawer-scrim")).Absolute().Fill()
			if s.MobileDrawer != "" {
				if extra.drawerDragOpened != s.MobileDrawer {
					extra.drawerDragOffset = ui.Point{}
					extra.drawerDragOpened = s.MobileDrawer
				}
				if backdrop.Clicked() {
					s.MobileDrawer = ""
					s.MobileDrawerProgress = 0
					extra.drawerDragOffset = ui.Point{}
					extra.drawerDragOpened = ""
					extra.drawerSkipCloseAnim = false
				}
			}

			sheet := ui.Column(c.Key("mb-sheet-"+activeKind)).Absolute().Top(0).Bottom(0).Width(sheetMaxW).
				Shadow(0, 8, 32, 0, tok.ShadowColor).Clip()
			theme.ApplyPanelGlass(sheet, tok, 0.45)
			if isLeft {
				sheet.Left(0)
			} else {
				sheet.Right(0)
			}

			var sheetDragX float32
			if s.MobileDrawer != "" && pAnim >= 0.985 {
				gone := -sheetMaxW
				bounds := ui.Rect{X: -sheetMaxW, W: sheetMaxW}
				if !isLeft {
					gone = sheetMaxW
					bounds = ui.Rect{X: 0, W: sheetMaxW}
				}
				sheet.Draggable(&extra.drawerDragOffset, ui.DraggableOptions{
					Axis:      ui.Horizontal,
					Bounds:    &bounds,
					Rigid:     true,
					Animation: theme.MotionDrawer,
					Snap:      ui.Nearest(ui.Point{}, ui.Point{X: gone}),
				})
				sheetDragX = sheet.DragOffset().X
				if !sheet.Dragging() && math.Abs(float64(extra.drawerDragOffset.X-gone)) < 1.0 {
					s.MobileDrawer = ""
					s.MobileDrawerProgress = 0
					extra.drawerDragOffset = ui.Point{}
					extra.drawerDragOpened = ""
					extra.drawerSkipCloseAnim = true
					mygo.Haptics.Light()
					return
				}
			} else {
				sheetOffsetX := (1 - pAnim) * sheetMaxW
				if isLeft {
					sheetOffsetX = -(1 - pAnim) * sheetMaxW
				}
				sheet.Translate(sheetOffsetX, 0)
			}
			dragProgress := float32(math.Min(1, math.Abs(float64(sheetDragX))/float64(max(sheetMaxW, 1))))
			backdrop.Background(tok.Scrim.Alpha(max(0, pAnim*(1-dragProgress))))

			safeTop, safeRight, safeBottom, safeLeft := c.SafeArea()
			t := c.Theme()
			sheet.Children(func() {
				if isLeft {
					// Left Drawer: compact header + clipped scrollable file tree + 3-tier bottom control island.
					ui.Row(c).FillWidth().
						Padding(safeTop+t.Space(2), t.Space(3)+safeRight, t.Space(2), t.Space(3)+safeLeft).
						AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						ui.Icon(c, lucide.FolderOpen()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
						components.Text(c, "实验库").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
						if components.IconButton(c.Key("mb-left-close"), lucide.X(), "关闭实验库抽屉", components.IconButtonPlain).Clicked() {
							s.MobileDrawer = ""
							s.MobileDrawerProgress = 0
							extra.drawerDragOffset = ui.Point{}
							extra.drawerDragOpened = ""
						}
					})
					ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
					ui.Box(c).FillWidth().Grow(1).MinHeight(0).Clip().
						Padding(t.Space(1), t.Space(2)+safeRight, 0, t.Space(2)+safeLeft).Children(func() {
						s.renderMobileFileTreeScrollBody(c, tok)
					})
					ui.Box(c).FillWidth().Padding(0, safeRight, safeBottom, safeLeft).Children(func() {
						s.renderMobileDrawerBottomIsland(c, tok, "tree")
					})
				} else {
					// Dedicated Right Companion Drawer: clean header with drawer switch + close, and full-height clipped body
					// (NO bottom island overlapping timers or calculator controls).
					ui.Row(c).FillWidth().
						Padding(safeTop+t.Space(2), t.Space(3)+safeRight, t.Space(2), t.Space(3)+safeLeft).
						AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
						ui.Icon(c, lucide.Calculator()).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Accent)
						components.Text(c, "步骤与速算").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
						if components.IconButton(c.Key("mb-right-close"), lucide.X(), "关闭右侧抽屉", components.IconButtonPlain).Clicked() {
							s.MobileDrawer = ""
							s.MobileDrawerProgress = 0
							extra.drawerDragOffset = ui.Point{}
							extra.drawerDragOpened = ""
						}
					})
					ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
					ui.Box(c).FillWidth().Grow(1).MinHeight(0).Clip().
						Padding(0, safeRight, safeBottom, safeLeft).Children(func() {
						s.rightDrawer(c)
					})
				}
			})
		})
	})
}

func (s *Shell) renderMobileDrawerBottomIsland(c *ui.Context, tok theme.Tokens, activeMode string) {
	expCnt, tplCnt := 0, 0
	for _, f := range s.VaultFiles {
		if f.IsTemplate {
			tplCnt++
		} else if !f.IsAsset {
			expCnt++
		}
	}
	if expCnt == 0 && len(s.Experiments) > 0 {
		expCnt = len(s.Experiments)
	}

	ui.Column(c.Key("mb-drawer-island")).FillWidth().
		Padding(float32(tok.Spacing8), float32(tok.Spacing16), float32(tok.Spacing16), float32(tok.Spacing16)).
		Gap(float32(tok.Spacing12)).Children(func() {
		// Tier 1: Quiet centered icon-only action toolbar (New, Folder, Calendar, Pantry, Collapse)
		ui.Row(c).FillWidth().Justify(ui.SpaceEvenly).AlignItems(ui.Center).Children(func() {
			miniAction := func(icon *ui.SVG, label string, onClick func()) {
				b := ui.ButtonBase(c).Size(36, 36).Radius(float32(tok.PillRadius)).Center().Label(label)
				theme.ApplyTactilePress(b)
				b.Children(func() {
					ui.Icon(c, icon).Size(18, 18).TextColor(tok.Text)
				})
				if b.Clicked() {
					onClick()
				}
			}
			miniAction(lucide.SquarePen(), "新建实验", func() {
				s.MobileDrawer = ""
				s.triggerVaultNewFile("")
			})
			miniAction(lucide.FolderPlus(), "新建文件夹", func() {
				s.startNewFolderPrompt("")
			})
			miniAction(lucide.CalendarDays(), "日程主页", func() {
				s.MobileDrawer = ""
				s.OpenCalendarTab()
			})
			miniAction(lucide.Package(), "储藏室", func() {
				s.MobileDrawer = ""
				s.OpenPantryTab()
			})
			miniAction(lucide.ChevronsDownUp(), "折叠全部", func() {
				for k := range s.folderOpen {
					s.folderOpen[k] = false
				}
			})
		})

		// Tier 2: Pill view filter switcher inside Left Drawer (实验列表 ↕ / 仅看模板 ↕ — NEVER jumps to right drawer!)
		extra := s.mobileExtra()
		modeIcon := lucide.Folder()
		modeText := "实验列表"
		if extra.mobileDrawerFilterTemplates {
			modeIcon = lucide.BookOpen()
			modeText = "仅看模板"
		}
		if activeMode == "outline" {
			modeIcon = lucide.ListTree()
			modeText = "步骤与速算"
		}
		pillSwitch := ui.ButtonBase(c.Key("mb-drawer-mode-pill")).FillWidth().Height(float32(tok.Spacing32)+float32(tok.Spacing8)).
			Radius(float32(tok.PillRadius)).Padding(0, float32(tok.Spacing16)).Background(tok.Elevated).Label(modeText)
		theme.ApplyTactilePress(pillSwitch)
		pillSwitch.Children(func() {
			ui.Row(c).Fill().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, modeIcon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.Text)
				components.Text(c, modeText).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
				ui.Icon(c, lucide.ChevronsUpDown()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
			})
		})
		if pillSwitch.Clicked() {
			mygo.Haptics.Selection()
			if activeMode == "tree" {
				extra.mobileDrawerFilterTemplates = !extra.mobileDrawerFilterTemplates
			} else {
				s.MobileDrawer = "tree"
			}
		}

		// Tier 3: Clickable Vault Manager trigger (Vault title/stats on left) + Sync status pill + Settings Gear!
		vaultTitle := "AsterLab"
		if vp := strings.TrimSpace(s.VaultPathInput); vp != "" {
			if base := filepath.Base(vp); base != "" && base != "." && base != "/" && base != `\` {
				vaultTitle = base
			}
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			vaultBtn := ui.ButtonBase(c.Key("mb-drawer-vault-mgr")).Grow(1).MinWidth(0).
				Radius(float32(tok.ControlRadius)).Padding(float32(tok.Spacing4)).
				Justify(ui.Start).Label("管理实验库 " + vaultTitle)
			theme.ApplyTactilePress(vaultBtn)
			vaultBtn.Children(func() {
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
						components.Text(c, vaultTitle).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
						ui.Icon(c, lucide.ChevronDown()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
					})
					components.Text(c, fmt.Sprintf("%d 份实验 · %d 模板", expCnt, tplCnt)).
						FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
				})
			})
			if vaultBtn.Clicked() {
				mygo.Haptics.Selection()
				s.MobileDrawer = ""
				s.MobileDrawerProgress = 0
				extra.drawerDragOffset = ui.Point{}
				extra.drawerDragOpened = ""
				s.VaultLauncherOpen = true
			}

			s.renderMobileDrawerSyncIndicator(c, tok)

			gearBtn := ui.ButtonBase(c.Key("mb-drawer-settings")).Size(float32(tok.Spacing32)+float32(tok.Spacing8), float32(tok.Spacing32)+float32(tok.Spacing8)).Radius(float32(tok.PillRadius)).Center().Label("打开设置")
			theme.ApplyCapsuleGlass(gearBtn, tok, true)
			theme.ApplyTactilePress(gearBtn)
			gearBtn.Children(func() {
				ui.Icon(c, lucide.Settings()).Size(18, 18).TextColor(tok.Text)
			})
			if gearBtn.Clicked() {
				s.MobileDrawer = ""
				s.SettingsView.Category = ""
				s.SettingsView.Query = ""
				extra := s.mobileExtra()
				extra.mobileSettingsLastOpen = false
				extra.mobileSettingsDetail = false
				s.VaultSettingsOpen = true
			}
		})
	})
}

func (s *Shell) renderMobileDrawerSyncIndicator(c *ui.Context, tok theme.Tokens) {
	dotColor := tok.TextMuted
	bgColor := tok.Elevated
	borderColor := tok.BorderSubtle
	textColor := tok.TextMuted
	label := "未同步"
	hasConflict := len(s.SyncConflicts) > 0

	switch {
	case hasConflict:
		dotColor = tok.Warning
		bgColor = tok.WarningBg
		borderColor = tok.Warning
		textColor = tok.WarningText
		label = fmt.Sprintf("冲突 %d", len(s.SyncConflicts))
	case s.SyncState.Status == "error":
		dotColor = tok.Danger
		bgColor = tok.DangerBg
		borderColor = tok.Danger
		textColor = tok.DangerText
		label = "同步异常"
	case s.SyncState.Status == "syncing":
		dotColor = tok.Accent
		bgColor = tok.InfoBg
		borderColor = tok.Border
		textColor = tok.InfoText
		label = "同步中"
	case s.SyncSettings.Enabled:
		dotColor = tok.Success
		bgColor = tok.Elevated
		borderColor = tok.BorderSubtle
		textColor = tok.TextMuted
		if s.SyncState.PendingCount > 0 {
			dotColor = tok.Warning
			label = fmt.Sprintf("待同步 %d", s.SyncState.PendingCount)
		} else {
			label = "已同步"
		}
	}

	syncPill := ui.ButtonBase(c.Key("mb-drawer-sync-status")).
		Height(float32(tok.Spacing24)).
		Padding(0, float32(tok.Spacing8)).
		Radius(float32(tok.ControlRadius)).
		Background(bgColor).
		Border(1, borderColor).
		Shrink(0).
		Center().
		Label("同步状态 " + label)
	theme.ApplyTactilePress(syncPill)
	syncPill.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			ui.Box(c).Size(float32(tok.Spacing8), float32(tok.Spacing8)).
				Radius(float32(tok.Spacing4)).Background(dotColor).Shrink(0)
			components.Text(c, label).FontSize(float32(tok.TextMeta)).
				TextColor(textColor).SingleLine().FontFeatures("tnum")
		})
	})
	if syncPill.Clicked() {
		mygo.Haptics.Selection()
		if hasConflict || !s.SyncSettings.Enabled || s.SyncState.Status == "error" || s.OnTriggerSyncNow == nil {
			s.MobileDrawer = ""
			s.SettingsView.Category = "sync"
			s.VaultSettingsOpen = true
		} else {
			s.OnTriggerSyncNow()
		}
	}
}

func (s *Shell) renderMobileFileTreeScrollBody(c *ui.Context, tok theme.Tokens) {
	if s.folderOpen == nil {
		s.folderOpen = map[string]bool{}
	}
	s.ensureToolbarConfigLoaded()

	scrollArea := ui.Scroll(c).FillWidth().Grow(1).MinHeight(0)
	scrollArea.Children(func() {
		ui.Column(c).FillWidth().Padding(float32(tok.Spacing4)).Gap(float32(tok.Spacing4)).Children(func() {
			if s.VaultTree == nil || len(s.VaultTree.Children) == 0 {
				ui.Column(c).FillWidth().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing4)).Children(func() {
					components.Text(c, "暂无实验文件").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					if components.Button(c, "新建实验", components.ButtonGhost).Clicked() {
						s.MobileDrawer = ""
						s.triggerVaultNewFile("")
					}
				})
				return
			}
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing4) * 0.5).Children(func() {
				s.renderMobileTreeChildren(c, tok, s.VaultTree.Children, 0)
			})
		})
	})
}

func (s *Shell) renderMobileTreeChildren(c *ui.Context, tok theme.Tokens, children []*vault.TreeNode, depth int) {
	filterTemplates := s.mobileExtra().mobileDrawerFilterTemplates
	var nodeHasTemplate func(n *vault.TreeNode) bool
	nodeHasTemplate = func(n *vault.TreeNode) bool {
		if n == nil {
			return false
		}
		if !n.IsDir {
			return n.Entry != nil && n.Entry.IsTemplate
		}
		for _, ch := range n.Children {
			if nodeHasTemplate(ch) {
				return true
			}
		}
		return false
	}

	var dirs, pinnedFiles, otherFiles []*vault.TreeNode
	for _, ch := range children {
		if ch == nil {
			continue
		}
		if filterTemplates && !nodeHasTemplate(ch) {
			continue
		}
		if ch.IsDir {
			dirs = append(dirs, ch)
		} else if s.IsFilePinned(ch.RelPath) {
			pinnedFiles = append(pinnedFiles, ch)
		} else {
			otherFiles = append(otherFiles, ch)
		}
	}
	for _, node := range pinnedFiles {
		s.renderMobileTreeNode(c, tok, node, depth)
	}
	for _, node := range dirs {
		s.renderMobileTreeNode(c, tok, node, depth)
	}
	for _, node := range otherFiles {
		s.renderMobileTreeNode(c, tok, node, depth)
	}
}

func (s *Shell) renderMobileTreeNode(c *ui.Context, tok theme.Tokens, node *vault.TreeNode, depth int) {
	if node == nil {
		return
	}
	if node.IsDir {
		if _, seen := s.folderOpen[node.RelPath]; !seen {
			s.folderOpen[node.RelPath] = node.Name != vault.AssetsDirName
		}
		open := s.folderOpen[node.RelPath]
		selected := s.SelectedTreePath == node.RelPath
		if s.renamingRelPath == node.RelPath {
			s.renderInlineTreeRename(c, tok, node.RelPath, node.Name)
			return
		}
		icon := lucide.Folder()
		if open {
			icon = lucide.FolderOpen()
		}
		if node.Name == vault.TemplatesDirName {
			icon = lucide.BookOpen()
		} else if node.Name == vault.AssetsDirName {
			icon = lucide.Paperclip()
		}
		row := ui.ButtonBase(c.Key("mb-tree-dir:"+node.RelPath)).Label(node.Name).
			FillWidth().MinWidth(0).MinHeight(float32(tok.Spacing48)*0.85).
			Justify(ui.Start).Radius(float32(tok.ControlRadius)).
			Padding(0, float32(tok.Spacing8), 0, float32(tok.Spacing4)).
			Selected(selected).TextColor(tok.Text)
		if selected {
			row.Background(tok.SelectedRow)
		}
		theme.ApplyTactilePress(row)
		row.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				renderTreeIndentGuides(c, tok, depth)
				chev := lucide.ChevronRight()
				if open {
					chev = lucide.ChevronDown()
				}
				ui.Box(c).Size(float32(tok.Spacing16), float32(tok.Spacing16)).Center().Shrink(0).Children(func() {
					ui.Icon(c, chev).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.TextMuted)
				})
				ui.Icon(c, icon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.TextMuted)
				dispName := node.Name
				if node.IsSystem {
					switch strings.ToLower(node.Name) {
					case "templates":
						dispName = "实验模板"
					case "assets":
						dispName = "图片与附件"
					}
				}
				components.Text(c, dispName).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text).
					SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			})
		})
		if row.Clicked() {
			s.SelectedTreePath = node.RelPath
			s.folderOpen[node.RelPath] = !open
			open = !open
		}
		kit.ContextMenu(c, row, func(m *kit.Menu) {
			if m.Item("在此文件夹新建实验").Chosen() {
				s.MobileDrawer = ""
				s.triggerVaultNewFile(node.RelPath)
			}
			if m.Item("新建子文件夹").Chosen() {
				s.startNewFolderPrompt(node.RelPath)
			}
			m.Separator()
			if m.Item("重命名").Disabled(node.IsSystem).Chosen() {
				s.renamingRelPath = node.RelPath
				s.renameDraft = node.Name
			}
			if m.Item("删除").Disabled(node.IsSystem).Chosen() && s.OnVaultDelete != nil {
				s.OnVaultDelete(node.RelPath)
			}
		})
		if open && len(node.Children) > 0 {
			ui.Column(c.Key("mb-tree-dir-body:"+node.RelPath)).FillWidth().
				Gap(float32(tok.Spacing4) * 0.25).Children(func() {
				s.renderMobileTreeChildren(c, tok, node.Children, depth+1)
			})
		}
		return
	}

	if s.renamingRelPath == node.RelPath {
		s.renderInlineTreeRename(c, tok, node.RelPath, node.Name)
		return
	}
	entry := node.Entry
	isTemplate := entry != nil && entry.IsTemplate
	isAsset := entry != nil && entry.IsAsset
	activeTab := s.ActiveTab()
	selected := s.SelectedTreePath == node.RelPath || (activeTab != nil && activeTab.Kind == TabFile && activeTab.FilePath == node.RelPath)
	pinned := s.IsFilePinned(node.RelPath)

	icon := lucide.FlaskConical()
	if isTemplate {
		icon = lucide.FileText()
	} else if isAsset {
		icon = lucide.Image()
	}
	shortDate := ""
	if entry != nil && !isTemplate && len(entry.Date) >= 10 {
		shortDate = entry.Date[5:10]
	}

	pinLbl := "置顶"
	if pinned {
		pinLbl = "取消置顶"
	}
	pinColor := tok.Accent
	dupColor := tok.Success
	var leading []mobile.SwipeAction
	var trailing []mobile.SwipeAction
	if !isAsset {
		leading = append(leading, mobile.SwipeAction{
			Label: pinLbl,
			Color: &pinColor,
			OnTap: func() {
				s.TogglePinFile(node.RelPath)
				mygo.Haptics.Selection()
			},
		})
		if s.OnVaultDuplicate != nil {
			trailing = append(trailing, mobile.SwipeAction{
				Label: "副本",
				Color: &dupColor,
				OnTap: func() {
					s.OnVaultDuplicate(node.RelPath, "")
					mygo.Haptics.Light()
				},
			})
		}
	}
	if s.OnVaultDelete != nil {
		trailing = append(trailing, mobile.SwipeAction{
			Label:       "删除",
			Destructive: true,
			OnTap: func() {
				s.OnVaultDelete(node.RelPath)
				mygo.Haptics.Warning()
			},
		})
	}

	mobile.SwipeRow(c.Key("mb-swipe-"+node.RelPath), nil, mobile.SwipeOptions{
		Leading:  leading,
		Trailing: trailing,
	}, func() {
		rowBg := tok.Surface
		if selected {
			rowBg = tok.SelectedRow
		}
		row := ui.ButtonBase(c.Key("mb-tree-file:"+node.RelPath)).Label(node.Name).
			FillWidth().MinWidth(0).MinHeight(float32(tok.Spacing48)*0.85).
			Justify(ui.Start).Radius(float32(tok.ControlRadius)).
			Padding(0, float32(tok.Spacing8), 0, float32(tok.Spacing4)).
			Background(rowBg).Selected(selected).TextColor(tok.Text)
		theme.ApplyTactilePress(row)
		row.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				renderTreeIndentGuides(c, tok, depth)
				ui.Box(c).Size(float32(tok.Spacing16), float32(tok.Spacing16)).Center().Shrink(0).Children(func() {
					if pinned {
						ui.Icon(c, lucide.Pin()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent)
					}
				})
				ui.Icon(c, icon).Size(float32(tok.IconMd), float32(tok.IconMd)).TextColor(tok.TextMuted)
				components.Text(c, node.Name).FontSize(float32(tok.TextData)).TextColor(tok.Text).
					SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
				if shortDate != "" {
					ui.Text(c, shortDate).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
						FontFeatures("tnum").Shrink(0)
				}
			})
		})
		if row.Clicked() {
			s.SelectedTreePath = node.RelPath
			if entry != nil && !isAsset {
				s.OpenFileInPane(s.ActivePaneID, entry.RelPath, entry.Title, s.DefaultFileOpenMode(), false)
				s.MobileDrawer = ""
			} else if isAsset && s.OnRevealInExplorer != nil {
				s.OnRevealInExplorer(node.RelPath)
			}
		}
		kit.ContextMenu(c, row, func(m *kit.Menu) {
			if entry != nil && !isAsset {
				if m.Item("打开").Chosen() {
					s.OpenFileInPane(s.ActivePaneID, entry.RelPath, entry.Title, s.DefaultFileOpenMode(), false)
					s.MobileDrawer = ""
				}
				if m.Item(pinLbl).Chosen() {
					s.TogglePinFile(node.RelPath)
				}
				if m.Item("创建副本").Chosen() && s.OnVaultDuplicate != nil {
					s.OnVaultDuplicate(entry.RelPath, "")
				}
				if !isTemplate && m.Item("存为模板").Chosen() && s.OnVaultSaveTemplate != nil {
					s.OnVaultSaveTemplate(entry.RelPath)
				}
				m.Separator()
			}
			if m.Item("重命名").Chosen() {
				s.renamingRelPath = node.RelPath
				s.renameDraft = node.Name
			}
			if m.Item("删除").Chosen() && s.OnVaultDelete != nil {
				s.OnVaultDelete(node.RelPath)
			}
		})
	})
}

func (s *Shell) renderMobileTabSwitcherOverlay(c *ui.Context, tok theme.Tokens, activePane *PaneNode, activeTab *WorkspaceTab) {
	if activePane == nil {
		s.MobileTabSwitcherOpen = false
		return
	}
	ui.Overlay(c, func() {
		safeTop, safeRight, safeBottom, safeLeft := c.SafeArea()
		overlay := ui.Column(c.Key("mb-tab-switcher")).Absolute().Fill().Modal().
			Padding(float32(tok.Spacing16)+safeTop, float32(tok.Spacing16)+safeRight, float32(tok.Spacing16)+safeBottom, float32(tok.Spacing16)+safeLeft).
			Gap(float32(tok.Spacing12)).
			Transition(theme.MobileTabOverviewTransition())
		theme.ApplyPanelGlass(overlay, tok, 0.35)
		overlay.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
			components.Text(c, fmt.Sprintf("标签页概览 (%d)", len(activePane.Tabs))).
				FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
			if components.Button(c, "+ 新建实验", components.ButtonGhost).Clicked() {
				s.MobileTabSwitcherOpen = false
				s.triggerVaultNewFile("")
			}
			if components.Button(c, "完成", components.ButtonPrimary).Clicked() {
				s.MobileTabSwitcherOpen = false
			}
		})

		ui.Scroll(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Padding(0, 0, float32(tok.Spacing24), 0).Children(func() {
				for idx := 0; idx < len(activePane.Tabs); idx += 2 {
					ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
						for j := 0; j < 2; j++ {
							pos := idx + j
							if pos >= len(activePane.Tabs) {
								ui.Box(c).Grow(1).MinWidth(0)
								continue
							}
							t := activePane.Tabs[pos]
							isActive := activeTab != nil && activeTab.ID == t.ID
							cardBg := tok.Elevated
							var borderW float32 = 0
							cardBorder := ui.Transparent
							if isActive {
								cardBg = tok.AccentSoftBg
								cardBorder = tok.Accent
								borderW = 1.5
							}

							card := ui.ButtonBase(c.Key("mb-ts-"+t.ID)).Grow(1).MinWidth(0).Height(96).
								Padding(float32(tok.Spacing12)).Radius(float32(tok.DialogRadius)).
								Background(cardBg).Border(borderW, cardBorder).
								Label(t.Title)
							theme.ApplyTactilePress(card)
							closedCard := false
							card.Children(func() {
								ui.Column(c).Fill().Gap(float32(tok.Spacing4)).Children(func() {
									ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ico := lucide.FlaskConical()
										switch t.Kind {
										case TabCalendar:
											ico = lucide.CalendarDays()
										case TabPantry:
											ico = lucide.Package()
										}
										ui.Icon(c, ico).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
										ui.Spacer(c)
										if isActive {
											components.Badge(c, "当前", components.BadgeOptions{Tone: components.BadgeAccent})
										}
										if !t.Pinned && len(activePane.Tabs) > 1 {
											xBtn := components.IconButton(c, lucide.X(), "关闭标签页", components.IconButtonCompact)
											if xBtn.Clicked() {
												closedCard = true
												s.CloseTabInPane(activePane.ID, t.ID)
											}
										}
									})
									components.Text(c, firstNonEmptyStr(t.Title, "未命名")).
										FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
									sub := "实验日程"
									switch t.Kind {
									case TabPantry:
										sub = "试剂耗材储藏室"
									case TabFile:
										modeLbl := "编辑态"
										if t.Mode == ModeRead {
											modeLbl = "阅读态"
										} else if t.Mode == ModeSource {
											modeLbl = "源码态"
										}
										dir := filepath.ToSlash(filepath.Dir(t.FilePath))
										if dir == "" || dir == "." {
											dir = "根目录"
										}
										sub = fmt.Sprintf("%s · %s", dir, modeLbl)
									}
									components.Text(c, sub).FontSize(float32(tok.TextMeta)*0.88).
										TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
								})
							})
							if !closedCard && card.Clicked() {
								s.SwitchTabInPane(activePane.ID, t.ID, true)
								s.MobileTabSwitcherOpen = false
							}
						}
					})
				}
			})
		})
	})
	})
}
