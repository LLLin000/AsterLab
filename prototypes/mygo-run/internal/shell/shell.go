// Package shell is the AsterLab natively drawn desktop frame: an Obsidian-grade
// peer split-pane workspace built on Onevra's sidebar shell with Ribbon + File Tree,
// Back/Forward history, drag-to-split TabGroups, companion drawer, and Ctrl+O/K palette.
package shell

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"asterlab-mygo-prototype/internal/schedule"
	"asterlab-mygo-prototype/internal/syncengine"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/vault"
	"asterlab-mygo-prototype/modules/runs"
)

type Page int

const (
	Project Page = iota
	Design
	Protocol
	Run
	Notebook
	Tools
)

func (p Page) ShortLabel() string {
	switch p {
	case Tools:
		return "储藏室"
	case Design, Protocol, Run, Notebook:
		return "实验"
	}
	return "日程"
}

func (s *Shell) PageName() string {
	switch s.Page {
	case Tools:
		return "tools"
	case Design, Protocol, Run, Notebook:
		return "design"
	}
	return "project"
}

func (p Page) Label() string {
	switch p {
	case Tools:
		return "储藏室"
	case Design, Protocol, Run, Notebook:
		return "实验文档"
	}
	return "实验日程"
}

type ExperimentStage string

const (
	StageDraft      ExperimentStage = "draft"
	StageReady      ExperimentStage = "ready"
	StageInProgress ExperimentStage = "in_progress"
	StageCompleted  ExperimentStage = "completed"
)

type ExperimentItem struct {
	ID          string
	DesignID    string
	ActiveRunID string
	FilePath    string
	Folder      string
	Title       string
	Date        string // YYYY-MM-DD
	Stage       ExperimentStage
	Summary     string
	GapHint     string
	StartHour   float32
	DurationMin int
	FileName    string
}

type DesignItem struct{ ID, Title, Meta string }
type RecentRun struct{ ID, Title, Meta string }

const (
	SyncImagePolicyPreviewOnly    = string(syncengine.PolicyPreviewOnly)
	SyncImagePolicyThreshold      = "threshold"
	SyncImagePolicyAlwaysOriginal = string(syncengine.PolicyAlwaysOriginal)
)

const (
	SyncProviderWebDAV   = "webdav"
	SyncProviderOneDrive = "onedrive"
	SyncProviderGDrive   = "gdrive"
)

// NormalizeSyncProvider maps empty/unknown sync provider strings to canonical keys.
func NormalizeSyncProvider(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SyncProviderOneDrive:
		return SyncProviderOneDrive
	case SyncProviderGDrive, "google_drive", "google":
		return SyncProviderGDrive
	default:
		return SyncProviderWebDAV
	}
}

// SyncSettings holds WebDAV / OneDrive / Google Drive connection parameters and the
// dual-track scientific image download policy.
type SyncSettings struct {
	Provider          string // "webdav" | "onedrive" | "gdrive"
	Enabled           bool
	Endpoint          string
	Username          string
	Password          string
	RemotePath        string
	AutoIntervalMin   int
	ImagePolicy       string // "preview_only" | "threshold" | "always_original"
	ImageThresholdMB  int    // 1 | 2 | 5 | 10
	WiFiOnlyOriginals bool
	OAuthClientID     string
	OAuthAuthorized   bool
	OAuthAccountHint  string
	OAuthUserCode     string
	OAuthVerifyURL    string
	OAuthPending      bool
}

// NormalizeImagePolicy maps syncengine.DownloadPolicyMode or UI policy values into canonical UI keys.
func NormalizeImagePolicy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case SyncImagePolicyPreviewOnly:
		return SyncImagePolicyPreviewOnly
	case SyncImagePolicyAlwaysOriginal:
		return SyncImagePolicyAlwaysOriginal
	case SyncImagePolicyThreshold, string(syncengine.PolicyThresholdBytes), "":
		return SyncImagePolicyThreshold
	default:
		return SyncImagePolicyThreshold
	}
}

// IsNutstore reports whether Endpoint points to Nutstore (jianguoyun.com), which triggers auto-throttling.
func (cfg SyncSettings) IsNutstore() bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(cfg.Endpoint)), "jianguoyun.com")
}

// ToDownloadPolicy converts UI SyncSettings into the syncengine.DownloadPolicy contract.
func (cfg SyncSettings) ToDownloadPolicy(isMobile bool) syncengine.DownloadPolicy {
	var mode syncengine.DownloadPolicyMode
	switch NormalizeImagePolicy(cfg.ImagePolicy) {
	case SyncImagePolicyPreviewOnly:
		mode = syncengine.PolicyPreviewOnly
	case SyncImagePolicyAlwaysOriginal:
		mode = syncengine.PolicyAlwaysOriginal
	default:
		mode = syncengine.PolicyThresholdBytes
	}
	mb := cfg.ImageThresholdMB
	switch mb {
	case 1, 2, 5, 10:
	default:
		mb = 2
	}
	return syncengine.DownloadPolicy{
		Mode:                 mode,
		ThresholdBytes:       int64(mb) << 20,
		WiFiOnlyForOriginals: cfg.WiFiOnlyOriginals,
		IsMobile:             isMobile,
	}
}

// SyncStateSummary exposes live WebDAV sync telemetry and local originals cache footprint.
type SyncStateSummary struct {
	Status                string // "" | "idle" | "syncing" | "synced" | "error" | "conflict"
	LastSyncText          string
	PendingCount          int
	LocalOriginalsCacheMB float64
	ErrorText             string
}

// SyncConflictItem describes one preserved `.sync-conflict-*` copy awaiting user resolution.
type SyncConflictItem struct {
	RelPath      string
	ConflictPath string
	DetectedAt   string
}

// DiagnosticItem renders one graded scientific or structural diagnostic in the companion drawer.
type DiagnosticItem struct {
	Code         string
	Severity     string // "blocking" | "needs_confirmation" | "advisory"
	Path         string
	Message      string
	NextAction   string
	PantryItemID string
}

type tabDragPayload struct {
	PaneID string
	TabID  string
	Tab    WorkspaceTab
}

// Shell owns AsterLab's window-level Obsidian workspace state.
type Shell struct {
	Project     string
	Projects    []string
	Designs     []DesignItem
	RecentRuns  []RecentRun
	Experiments []ExperimentItem
	Selected    string
	Page        Page
	Loading     bool
	Error       string

	ActiveTimers   []runs.ActiveTimer
	TimersExpanded bool
	OnTimerSelect  func(runID string)
	OnSelect       func(string)
	OnProject      func(string)
	OnRunSelect    func(string)
	OnPage         func(Page)
	OnSidebar      func(bool)

	Query          string
	SelectedDate   string
	TodayDate      string
	CurrentHour    float32
	CalendarMode              int    // 0 = Month Grid, 1 = Week Time Grid
	CalendarCursor            string // YYYY-MM-DD (month / week anchor)
	CalendarEvents            []schedule.Event
	CalendarDomainFilter      schedule.Filter
	calFilterInited           bool
	OnOpenScheduleEvent       func(ev schedule.Event)
	OnToggleScheduleEventDone func(ev schedule.Event)
	OnCreateReagentForDate    func(dateYMD string)
	OnCreateAnimalForDate     func(dateYMD string)
	OnOpenLabGallery          func()
	DayPopoverOpen            bool
	DayPopoverDate  string
	calAnimDir      int
	lastDayClickYMD string
	lastDayClickAt  time.Time
	weekHoverOn     bool
	weekHoverY      float32
	weekHoverDay    int

	ShowAllRuns          bool
	OnContinue           func()
	OnCreate             func()
	OnCreateForDate      func(dateYMD string)
	OnCreateForDateTime  func(dateYMD, startTimeHHMM string)
	OnOpenTemplatePicker func(dateYMD string)
	OnOpenExperiment     func(ExperimentItem)
	OnQuickStartRun      func(designID string)
	OnInspectDesign      func(designID string)
	OnImport             func()
	Content              func(*ui.Context)

	// Peer split-tree state
	RootPane     *PaneNode
	ActivePaneID string
	Tabs         []WorkspaceTab
	ActiveTabID  string
	History      []NavEntry
	HistoryIdx   int
	SplitOpen    bool
	SplitPct     float32
	SecondaryTab *WorkspaceTab
	paneSeq      uint64
	tabSeq       uint64
	middleHeld   map[string]bool

	// Right companion drawer (Outline + Calc/Timers, collapsed by default)
	RightOpen        bool
	RightTab         int
	RightCalc        RightCalcState
	BenchTimer       BenchTimerState
	PantryItems      []QuickPantryItem
	OutlineSteps     []OutlineItem
	OutlineOnlySteps bool
	PaletteOpen      bool
	PaletteMode      string // "" (all) | "files" (Ctrl+O files+pantry) | "commands" (Ctrl+P/K actions)
	PaletteQuery     string
	PaletteCursor    int
	PaletteCommands  []PaletteCommand

	// Vault tree & file lifecycle callbacks
	VaultTree           *vault.TreeNode
	VaultFiles          []vault.FileEntry
	VaultFolders        []string
	FolderTemplates     map[string]string
	OnSetFolderTemplate func(folderRel, templateRel string)
	SelectedTreePath     string
	CreateTargetFolder   string
	CreateTargetTemplate string
	VaultSettingsOpen     bool
	Layers                kit.LayerStack
	Commands              *kit.CommandRegistry
	Toasts                *kit.ToastQueue
	RenderPantryInspector func(c *ui.Context, tok theme.Tokens)
	SettingsView          components.SettingsState
	AboutMonochromeLogo bool
	ThemeSource         string // system | light | dark
	TextScale           float32
	Density             string // comfortable | compact
	ReduceMotion        bool
	DefaultOpenMode     string // edit | read
	DefaultProjectDir   string
	DefaultPlateFormat  int
	DefaultExcessPct    int
	AutoArchiveAssets   bool
	Hotkeys             map[string]string
	HotkeyFilter        string
	ThemePreset         string // aurora | zinc | warm
	LiquidGlass         bool
	themeLookInited     bool
	ForceMobilePreview  bool
	MobileDrawer        string // "" | "tree" | "outline"
	MobilePullAction    string // palette | new_exp | calc | tree
	MobileAutoHideDock  bool
	MobileEditToolbar   bool
	MobileTabSwitcherOpen bool
	MobileMenuSheetOpen   bool
	MobileKeyboardOpen    bool
	MobileDockForceNav    bool
	MobileDockHidden      bool
	MobileDrawerProgress  float32
	MobileSwipeKind       string
	MobilePullOffset      float32
	mobileLastDrawerKind  string
	mobilePrefsInited     bool
	mobileScrollStates    map[string]*ui.ScrollState
	mobileLastScrollY     map[string]float32
	VaultPathInput        string
	HideAssetsInTree      bool
	TrashItems            []vault.TrashItem
	VaultLauncherOpen     bool
	VaultLauncherName     string
	VaultLauncherParent   string
	VaultLauncherSeedDemo bool
	VaultLauncherErr      string
	IsTempSandboxVault    bool
	RecentVaults          []RecentVaultEntry
	folderOpen            map[string]bool
	renamingRelPath       string
	renameDraft           string
	newFolderDialogOpen   bool
	newFolderParent       string
	newFolderName         string

	// Status Bar & Companion Diagnostics / Search fed by host
	StatusSaveLabel       string
	StatusMetaLabel       string
	StatusWarnLabel       string
	AppVersion            string
	BuildCommit           string
	UpdateStatus          string // "" | "checking" | "up_to_date" | "available" | "downloading" | "restarting" | "error"
	UpdateLatestVersion   string
	UpdateNotes           string
	UpdateDownloadURL     string
	UpdateErr             string
	UpdateIncludePre      bool
	UpdateAutoCheck       bool
	OnCheckUpdate         func()
	OnInstallUpdate       func()
	OnOpenUpdatePage      func()
	CrashNotice           string
	OnDismissCrashNotice  func()
	FeedbackOpen          bool
	FeedbackMode          bool
	FeedbackCategory      string
	FeedbackText          string
	FeedbackIncludeLogs   bool
	FeedbackStatus        string
	OnExportDiagBundle    func()
	OnCopyFeedbackReport  func()
	OnSubmitFeedback      func()
	OnStartFeedbackPick   func()
	OnOpenFeedbackQueue   func()
	WelcomeOpen           bool
	OnOnboardingChoice    func(choice string)
	TourOpen              bool
	TourStep              int
	tourAutoOpenedRight   bool
	OnOpenUserGuide       func()
	inSplitViewRoot       bool
	GlobalSearchOpen      bool
	GlobalSearchQuery     string
	GlobalSearchLastQuery string
	GlobalSearchCursor    int
	GlobalSearchHits      []vault.SearchHit
	OnGlobalSearch        func(string) []vault.SearchHit
	OnOpenSearchHit       func(relPath, query string)
	VaultTimers            []vault.VaultTimerEntry
	CurrentDocRelPath      string
	CurrentBacklinks       []vault.LinkRef
	CurrentDiagnostics     []DiagnosticItem
	CurrentDocSuggestions  []DocSuggestionItem
	CurrentRelatedDocs     []RelatedDocItem
	CurrentMatchedReagents []MatchedReagentItem
	OnApplyDocSuggestion   func(DocSuggestionItem)
	drawerExpandAllSugs    bool
	CurrentSnapshots       []vault.SnapshotMeta
	OnRestoreSnapshot     func(relPath, snapshotID string)

	OnActivateTab          func(WorkspaceTab)
	OnSelectOutlineStep    func(stepID string)
	OnLocatePantry         func(itemID string)
	OnVaultCreateFile      func(folderRel, dateYMD string)
	OnVaultCreateFolder    func(parentRel, name string)
	OnVaultRename          func(relPath, newName string)
	OnVaultMove            func(srcRel, destFolderRel string)
	OnVaultDuplicate       func(relPath, targetDate string)
	OnVaultSaveTemplate    func(relPath string)
	OnVaultDelete          func(relPath string)
	OnVaultRestoreTrash    func(trashName string)
	OnSetExperimentDate    func(relPath, dateYMD string)
	OnRevealInExplorer     func(relPath string)
	OnPickVaultPath        func() string
	OnSwitchVaultPath      func(newPath string)
	OnSwitchVaultPathErr   func(newPath string) error
	OnCreateVault          func(targetPath string, seedDemo bool) error
	OnOpenSandboxVault     func()
	OnDismissVaultLauncher func()
	OnRemoveRecentVault    func(vaultPath string)
	OnToggleHideAssets     func(hide bool)
	SyncSettings           SyncSettings
	SyncState              SyncStateSummary
	SyncConflicts          []SyncConflictItem
	syncPrefsInited        bool
	syncPasswordVisible    bool
	OnSaveSyncSettings     func()
	OnTestSyncConnection   func()
	OnTriggerSyncNow       func()
	OnStartCloudOAuth      func(provider string)
	OnDisconnectCloudOAuth func(provider string)
	OnClearOriginalsCache  func()
	OnResolveSyncConflict  func(relPath, conflictPath, choice string)
	OnVaultBackupZip       func()
	OnVaultRestoreZip      func()
	OnSettingsChanged         func()
	OnUndo                    func()
	OnRedo                    func()
	OnMobileDocToolbarAction  func(tab WorkspaceTab, action string)
	RenderTabContent          func(*ui.Context, *PaneNode, WorkspaceTab)
	RenderPantryContent    func(*ui.Context)
	RenderSecondaryContent func(*ui.Context, WorkspaceTab)
	RenderOverlays         func(*ui.Context)
	ExternalModalOpen      bool
	IsExternalModalOpen    func() bool

	window    *sidebar.Shell
	mobileWin *mygo.Window
}

// AnyModalOpen reports whether any shell or host modal/dialog/popover currently covers the canvas,
// so underlying hover-tracking widgets (e.g. week-grid timeline) stay frozen behind overlays.
func (s *Shell) AnyModalOpen() bool {
	if s.ExternalModalOpen || (s.IsExternalModalOpen != nil && s.IsExternalModalOpen()) {
		return true
	}
	return s.VaultSettingsOpen || s.PaletteOpen || s.MobileMenuSheetOpen ||
		s.DayPopoverOpen || s.VaultLauncherOpen ||
		s.newFolderDialogOpen || s.FeedbackOpen || s.WelcomeOpen || s.GlobalSearchOpen
}

// NewWindow creates the AsterLab window with Ribbon + File Tree on the left
// and the slim companion drawer (collapsed by default) on the right.
// On native iOS / Android, it mounts a chromeless mobile window without desktop sidebar chrome.
func (s *Shell) NewWindow() *sidebar.Shell {
	s.EnsureTabs()
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		s.window = nil
		s.mobileWin = mygo.NewWindow(mygo.WindowOptions{
			Title:   "AsterLab",
			Content: ui.View(s.mobileRootView),
		})
		return nil
	}
	s.window = sidebar.NewShell(sidebar.ShellOptions{
		Title:          "AsterLab",
		Width:          1380,
		Height:         880,
		MinWidth:       900,
		MinHeight:      600,
		StateKey:       "asterlab-desk",
		Theme:          theme.Window,
		LeftWidth:      284,
		LeftMinWidth:   220,
		LeftMaxWidth:   400,
		Left:           s.navigation,
		Content:        s.page,
		ContentTitle:   s.topChromeBar,
		LeftHeader:     s.navigationHeader,
		LeftToolbar:    s.navigationFooter,
		Right:          s.rightDrawer,
		RightHeader:    s.rightDrawerHeader,
		RightWidth:     380,
		RightMinWidth:  300,
		RightMaxWidth:  560,
		RightCollapsed: !s.RightOpen,
	})
	return s.window
}

func (s *Shell) mobileRootView(c *ui.Context) {
	c.SetTheme(theme.Window(c))
	s.page(c)
	components.Toasts(c, components.ToastOptions{})
}

func (s *Shell) Invalidate() {
	if s.window != nil {
		s.window.Window().Invalidate()
	} else if s.mobileWin != nil {
		s.mobileWin.Invalidate()
	}
}

// MyGoWindow returns the underlying native mygo.Window when mounted (nil in headless tests).
func (s *Shell) MyGoWindow() *mygo.Window {
	if s.window != nil {
		return s.window.Window()
	}
	return s.mobileWin
}

// LeftExpanded reports whether the left navigation sidebar is currently visible.
func (s *Shell) LeftExpanded() bool {
	if s.window == nil {
		return true
	}
	return s.window.LeftExpanded()
}

// topChromeBar draws the clean Obsidian window chrome bar:
// Back/Forward arrows + Search (Ctrl+K/O) + Companion Drawer toggle.
// Each peer split pane draws its own interactive tab bar cleanly right above its surface.
func (s *Shell) topChromeBar(c *ui.Context) {
	s.EnsureTabs()
	s.syncTourRightDrawer()
	tok := theme.GetTokens(c.Theme())
	if s.leafCount() == 1 {
		if leaf := s.firstLeaf(s.RootPane); leaf != nil {
			s.renderPaneTabBar(c, tok, leaf)
			return
		}
	}
	if s.window != nil {
		s.RightOpen = s.window.RightExpanded()
	}

	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
		backBtn := components.IconButton(c, lucide.ArrowLeft(), "后退 (Alt+←)", components.IconButtonPlain)
		backBtn.Disabled(!s.CanBack())
		if backBtn.Clicked() {
			s.GoBack()
		}
		fwdBtn := components.IconButton(c, lucide.ArrowRight(), "前进 (Alt+→)", components.IconButtonPlain)
		fwdBtn.Disabled(!s.CanForward())
		if fwdBtn.Clicked() {
			s.GoForward()
		}

		ui.Spacer(c)

		if s.Loading {
			components.Text(c, "加载中…").TextColor(tok.TextMuted).FontSize(float32(tok.TextMeta)).Shrink(0)
		}
		if s.Error != "" {
			components.Text(c, s.Error).TextColor(tok.Danger).FontSize(float32(tok.TextMeta)).
				SingleLine().Ellipsis("…").MaxWidth(float32(tok.Spacing48) * 5)
		}

		cmdBtn := components.IconButton(c, lucide.Search(), "快速打开 (Ctrl+O)", components.IconButtonPlain)
		components.Tooltip(c, cmdBtn, "快速打开 (Ctrl+O)")
		if cmdBtn.Clicked() {
			s.PaletteOpen = true
			s.PaletteCursor = 0
		}
		if s.window == nil {
			rdBtn := components.IconButton(c, lucide.PanelRight(), "大纲与速算抽屉", components.IconButtonPlain)
			if s.RightOpen {
				rdBtn.Background(tok.AccentSoftBg)
			}
			if rdBtn.Clicked() {
				s.ToggleRightDrawer()
			}
		}
	})
}

func (s *Shell) navigationHeader(c *ui.Context) {
	ui.Spacer(c)
}

func (s *Shell) navigationFooter(c *ui.Context) {
	tok := theme.GetTokens(c.Theme())
	label := fmt.Sprintf("AsterLab · %d 份实验", len(s.VaultFiles))
	if len(s.VaultFiles) == 0 && len(s.Experiments) > 0 {
		label = fmt.Sprintf("AsterLab · %d 份实验", len(s.Experiments))
	}
	components.Text(c, label).TextColor(tok.TextMuted).FontSize(float32(tok.TextMeta)).
		SingleLine().Ellipsis("…")
}

func (s *Shell) handleGlobalShortcuts(c *ui.Context) {
	if s.matchHotkey(c, "nav.left_sidebar", parsedHotkey{ui.Cmd, ui.KeyB}) && s.window != nil {
		s.window.SetLeftExpanded(!s.window.LeftExpanded())
	}
	// UI-14: Obsidian-style split shortcuts:
	// Ctrl+O -> Quick Switcher (files + pantry two-group lookup)
	// Ctrl+P / Ctrl+K -> Command Palette (workspace & tool actions)
	if c.Shortcut(ui.Cmd, ui.KeyO) {
		if s.PaletteOpen && s.PaletteMode == "files" {
			s.PaletteOpen = false
		} else {
			s.PaletteMode = "files"
			s.PaletteOpen = true
			s.PaletteCursor = 0
		}
	} else if s.matchHotkey(c, "cmd.palette", parsedHotkey{ui.Cmd, ui.KeyK}) || c.Shortcut(ui.Cmd, ui.KeyP) {
		if s.PaletteOpen && s.PaletteMode == "commands" {
			s.PaletteOpen = false
		} else {
			s.PaletteMode = "commands"
			s.PaletteOpen = true
			s.PaletteCursor = 0
		}
	}
	if s.matchHotkey(c, "nav.global_search", parsedHotkey{ui.Cmd | ui.Shift, ui.KeyF}) {
		s.GlobalSearchOpen = !s.GlobalSearchOpen
		if s.GlobalSearchOpen {
			s.refreshGlobalSearch()
		}
	}
	if s.matchHotkey(c, "vault.new_exp", parsedHotkey{ui.Cmd, ui.KeyN}) {
		s.triggerVaultNewFile("")
	}
	if s.matchHotkey(c, "nav.right_drawer", parsedHotkey{ui.Cmd | ui.Shift, ui.KeyR}) {
		s.ToggleRightDrawer()
	}
	if s.matchHotkey(c, "nav.back", parsedHotkey{ui.Alt, ui.KeyLeft}) || c.Shortcut(0, ui.KeyBack) {
		s.GoBack()
	}
	if s.matchHotkey(c, "nav.forward", parsedHotkey{ui.Alt, ui.KeyRight}) || c.Shortcut(0, ui.KeyForward) {
		s.GoForward()
	}
	if s.matchHotkey(c, "tab.close", parsedHotkey{ui.Cmd, ui.KeyW}) {
		if t := s.ActiveTab(); t != nil && !t.Pinned {
			s.CloseTabInPane(s.ActivePaneID, t.ID)
		}
	}
	if s.matchHotkey(c, "pane.split_right", parsedHotkey{ui.Cmd, ui.KeyBackslash}) {
		if t := s.ActiveTab(); t != nil {
			cp := *t
			cp.ID = ""
			s.SplitPaneWithTab(s.ActivePaneID, cp, "right")
		}
	}
	if s.matchHotkey(c, "doc.toggle_mode", parsedHotkey{ui.Cmd, ui.KeyE}) {
		if t := s.ActiveTab(); t != nil && t.Kind == TabFile {
			if t.Mode == ModeEdit {
				s.SetActiveTabMode(ModeLab)
			} else {
				s.SetActiveTabMode(ModeEdit)
			}
		}
	}
	// Let native text editors (Source mode & focused text inputs) handle their own Cmd+Z / Cmd+Y
	// instead of hijacking and resetting the cursor position.
	activeIsSource := false
	if t := s.ActiveTab(); t != nil && t.Kind == TabFile && t.Mode == ModeSource {
		activeIsSource = true
	}
	if !activeIsSource && !c.FocusTakesText() {
		if s.matchHotkey(c, "doc.undo", parsedHotkey{ui.Cmd, ui.KeyZ}) && s.OnUndo != nil {
			s.OnUndo()
		}
		if (s.matchHotkey(c, "doc.redo", parsedHotkey{ui.Cmd, ui.KeyY}) || c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ)) && s.OnRedo != nil {
			s.OnRedo()
		}
	}
	if s.matchHotkey(c, "nav.calendar", parsedHotkey{ui.Cmd, ui.Key1}) {
		s.OpenCalendarTab()
	}
	if s.matchHotkey(c, "app.settings", parsedHotkey{ui.Cmd, ui.KeyComma}) {
		s.VaultSettingsOpen = !s.VaultSettingsOpen
	}
	if c.Shortcut(ui.Cmd, ui.KeyEqual) || c.Shortcut(ui.Cmd|ui.Shift, ui.KeyEqual) {
		s.StepUIScale(c, 0.10)
	}
	if c.Shortcut(ui.Cmd, ui.KeyMinus) {
		s.StepUIScale(c, -0.10)
	}
	if c.Shortcut(ui.Cmd, ui.Key0) {
		s.TextScale = 0
		s.SyncThemeLook(c)
		s.notifySettingsChanged()
		c.Invalidate()
	}
}

// StepUIScale adjusts the unified font+icon UI zoom by delta (clamped to [0.85, 1.50])
// and persists the choice into Vault settings (standard Ctrl+= / Ctrl+- desktop zoom).
func (s *Shell) StepUIScale(c *ui.Context, delta float32) {
	base := s.TextScale
	if base <= 0 {
		base = theme.EffectiveScale()
	}
	next := float32(int((base+delta)*100+0.5)) / 100.0
	if next < 0.85 {
		next = 0.85
	}
	if next > 1.50 {
		next = 1.50
	}
	s.TextScale = next
	s.SyncThemeLook(c)
	s.notifySettingsChanged()
	if c != nil {
		c.Invalidate()
	}
}

// SetInitialThemeLook loads persisted appearance settings without letting first-frame
// defaults overwrite an explicitly saved LiquidGlass=false preference.
func (s *Shell) SetInitialThemeLook(preset string, liquidGlass bool) {
	if preset == "" || preset == "aurora" || preset == "obsidian" {
		preset = theme.PresetPaper
	}
	s.ThemePreset = preset
	s.LiquidGlass = liquidGlass
	s.themeLookInited = true
	theme.SetActiveLook(s.ThemePreset, s.LiquidGlass)
}

// SyncThemeLook ensures Shell's ThemePreset ("paper" | "zinc" | "warm"),
// LiquidGlass, TextScale, and ReduceMotion are synced to package theme and installed on c.
func (s *Shell) SyncThemeLook(c *ui.Context) theme.Tokens {
	if !s.themeLookInited {
		s.themeLookInited = true
		if s.ThemePreset == "" || s.ThemePreset == "aurora" || s.ThemePreset == "obsidian" {
			s.ThemePreset = theme.PresetPaper
		}
		if !s.LiquidGlass {
			s.LiquidGlass = true
		}
	}
	if s.ThemePreset == "" || s.ThemePreset == "aurora" || s.ThemePreset == "obsidian" {
		s.ThemePreset = theme.PresetPaper
	}
	theme.SetActiveLook(s.ThemePreset, s.LiquidGlass)
	theme.SetActiveDensity(s.Density)
	theme.SetAccessibilityPrefs(s.TextScale, s.ReduceMotion)
	if c != nil {
		c.SetTheme(theme.Window(c))
		tok := theme.GetTokens(c.Theme())
		c.Root().Background(tok.Canvas)
		return tok
	}
	return theme.LightTokens()
}

func (s *Shell) page(c *ui.Context) {
	s.EnsureTabs()
	tok := s.SyncThemeLook(c)
	s.ensureCommandRegistry()
	s.registerShellOverlays(c)
	if !s.Layers.HandleEscape(c) {
		s.handleGlobalShortcuts(c)
	}

	t := c.Theme()
	if s.IsMobileLayout(c) {
		s.renderMobileWorkbench(c, tok)
		return
	}
	var inset float32
	if s.window != nil {
		inset = sidebar.HeaderHeight(c)
	}

	rootCol := ui.Column(c).Fill().Padding(inset, 0, 0, 0)
	if !c.Vibrancy() {
		theme.PaintCanvasBackground(rootCol, tok)
	}
	rootCol.Children(func() {
		s.renderCrashOrWatchdogBanner(c, tok)
		s.timerStrip(c)
		if s.Error != "" {
			ui.Column(c).FillWidth().Padding(t.Space(1), t.Space(2)).Gap(t.Space(.5)).
				Background(t.Surface).Children(func() {
				components.Text(c, "未保存 · "+s.Error).FontSize(float32(tok.TextData)).TextColor(t.Danger)
				components.Text(c, "检查目标目录权限后，返回当前操作并重试保存。").
					TextColor(t.TextMuted).FontSize(float32(tok.TextMeta))
			})
		}
		ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			s.renderPaneNode(c, tok, s.RootPane)
		})
		s.renderDesktopStatusBar(c, tok)
	})

	s.renderCommandPalette(c)
	s.renderGlobalSearchModal(c)
	s.renderVaultDialogs(c)
	s.renderFeedbackDialog(c)
	s.renderWelcomeDialog(c)
	if !s.inSplitViewRoot {
		s.renderSpotlightTour(c, tok)
	}
	if s.RenderOverlays != nil {
		s.RenderOverlays(c)
	}
	if s.Toasts != nil {
		s.Toasts.Render(c, tok)
	}
}

func (s *Shell) renderDesktopStatusBar(c *ui.Context, tok theme.Tokens) {
	tab := s.ActiveTab()
	isFileTab := tab != nil && tab.Kind == TabFile
	saveLbl := "实时已保存"
	warnLbl := ""
	metaLbl := ""
	if isFileTab {
		saveLbl = firstNonEmptyStr(s.StatusSaveLabel, "实时已保存")
		warnLbl = s.StatusWarnLabel
		metaLbl = s.StatusMetaLabel
	}
	modeLbl := ""
	if tab != nil {
		switch tab.Kind {
		case TabCalendar:
			modeLbl = "实验日程"
		case TabPantry:
			modeLbl = "储藏室货架"
		default:
			switch tab.Mode {
			case ModeLab:
				modeLbl = "实验模式（防误触）"
			case ModeSource:
				modeLbl = "源码模式"
			default:
				modeLbl = "编辑模式"
			}
		}
	}
	ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.55))
	ui.Row(c.Key("desktop-status-bar")).FillWidth().Height(28).
		Padding(0, float32(tok.Spacing12)).
		Background(tok.SidebarBackground).AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
			dotCol := tok.Success
			if warnLbl != "" {
				dotCol = tok.Warning
			}
			ui.Box(c).Size(6, 6).Radius(float32(tok.PillRadius)).Background(dotCol)
			ui.Text(c, saveLbl).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
		})
		if warnLbl != "" {
			ui.Text(c, "· "+warnLbl).FontSize(float32(tok.TextMeta)).TextColor(tok.Warning).SingleLine()
		}
		if metaLbl != "" {
			ui.Text(c, metaLbl).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").SingleLine()
		}
		if isFileTab && s.CurrentDocRelPath != "" {
			blockingCnt, warnCnt := 0, 0
			for _, d := range s.CurrentDiagnostics {
				if d.Severity == "blocking" {
					blockingCnt++
				} else if d.Severity == "needs_confirmation" {
					warnCnt++
				}
			}
			alertCnt := blockingCnt + warnCnt
			sugCnt := len(s.CurrentDocSuggestions)
			pill := ui.ButtonBase(c.Key("status-steward-pill")).
				Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).
				Label("方案体检与关联建议")
			if pill.Hovered() {
				pill.Background(tok.Text.Alpha(0.05))
			}
			pill.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					switch {
					case alertCnt > 0:
						dotCol := tok.Warning
						if blockingCnt > 0 {
							dotCol = tok.Danger
						}
						ui.Box(c).Size(6, 6).Radius(float32(tok.PillRadius)).Background(dotCol)
						ui.Text(c, fmt.Sprintf("%d 项提醒", alertCnt)).FontSize(float32(tok.TextMeta)).TextColor(dotCol).FontFeatures("tnum").SingleLine()
					case sugCnt > 0:
						ui.Icon(c, lucide.Sparkles()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
						ui.Text(c, fmt.Sprintf("%d 处可结构化", sugCnt)).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
					default:
						ui.Box(c).Size(6, 6).Radius(float32(tok.PillRadius)).Background(tok.Success)
						ui.Text(c, "方案就绪").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
					}
				})
			})
			components.Tooltip(c, pill, "查看方案体检、关联试剂与可结构化步骤")
			if pill.Clicked() {
				s.SetRightDrawer(true, 2)
			}
		}
		ui.Spacer(c)
		if s.UpdateStatus == "available" && s.UpdateLatestVersion != "" {
			updBtn := ui.ButtonBase(c).Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).Label("发现新版本")
			updBtn.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Icon(c, lucide.CircleArrowUp()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
					ui.Text(c, "新版本 "+s.UpdateLatestVersion).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).SingleLine()
				})
			})
			if updBtn.Clicked() {
				s.SettingsView.Category = "about"
				s.VaultSettingsOpen = true
			}
		}
		timerTotal := len(s.ActiveTimers) + len(s.VaultTimers)
		if timerTotal > 0 {
			btn := ui.ButtonBase(c).Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).Label("查看计时器")
			btn.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Icon(c, lucide.Timer()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
					ui.Text(c, fmt.Sprintf("%d 个计时中", timerTotal)).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent).FontFeatures("tnum").SingleLine()
				})
			})
			if btn.Clicked() {
				s.SetRightDrawer(true, 1)
			}
		}
		if modeLbl != "" {
			ui.Text(c, modeLbl).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
		}
		guideBtn := ui.ButtonBase(c.Key("status-guide-btn")).Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).Label("新手指南与界面导览")
		guideBtn.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Icon(c, lucide.BookOpen()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
				ui.Text(c, "指南").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
			})
		})
		components.MenuButton(c, guideBtn, func(m *components.Menu) {
			if m.Item("1 分钟界面区域导览").Chosen() {
				s.StartTour()
			}
			if s.OnOpenUserGuide != nil && m.Item("打开「新手入门指南」互动文档").Chosen() {
				s.OnOpenUserGuide()
			}
		})
		fbBtn := ui.ButtonBase(c).Padding(0, float32(tok.Spacing8)).Radius(float32(tok.ControlRadius)).Label("反馈与诊断")
		fbBtn.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Icon(c, lucide.LifeBuoy()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
				ui.Text(c, "反馈").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
			})
		})
		if fbBtn.Clicked() {
			s.FeedbackOpen = true
		}
		ui.Text(c, s.HotkeyDisplay("cmd.palette")).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
	})
}

func (s *Shell) renderPaneNode(c *ui.Context, tok theme.Tokens, node *PaneNode) {
	if node == nil {
		return
	}
	if node.isLeaf() {
		s.renderLeafPane(c, tok, node)
		return
	}
	if node.Ratio <= 0 || node.Ratio >= 100 {
		node.Ratio = 50
	}
	orient := components.Horizontal
	if node.Vertical {
		orient = components.Vertical
	}
	components.SplitPercent(c.Key("split-"+node.ID), &node.Ratio,
		func() { s.renderPaneNode(c, tok, node.First) },
		func() { s.renderPaneNode(c, tok, node.Second) },
		components.SplitOptions{
			Orientation: orient,
			FirstMin:    float32(tok.Spacing48) * 4.5,
			SecondMin:   float32(tok.Spacing48) * 4.5,
			Default:     50,
		},
	).Label("工作区分屏分隔线").Fill()
}

func (s *Shell) renderLeafPane(c *ui.Context, tok theme.Tokens, pane *PaneNode) {
	if pane == nil {
		return
	}
	isFocusedPane := pane.ID == s.ActivePaneID

	container := ui.Column(c.Key("leaf-"+pane.ID)).Fill().MinWidth(0).MinHeight(0).Clip()
	theme.PaintCanvasBackground(container, tok)
	container.Children(func() {
		if isFocusedPane && s.leafCount() > 1 {
			ui.Box(c).FillWidth().Height(2).Background(tok.Accent.Alpha(.75))
		}
		// 1. Per-Pane Obsidian Tab Bar (skipped when hosted inside native window ContentTitle in single-pane mode)
		if !(s.window != nil && s.leafCount() == 1) {
			s.renderPaneTabBar(c, tok, pane)
			ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(.55))
		}

		// 2. Active Tab Content Surface + Edge Drag-to-Split Drop Zones
		var activeTab *WorkspaceTab
		for i := range pane.Tabs {
			if pane.Tabs[i].ID == pane.ActiveTabID {
				activeTab = &pane.Tabs[i]
				break
			}
		}
		if activeTab == nil && len(pane.Tabs) > 0 {
			pane.ActiveTabID = pane.Tabs[0].ID
			activeTab = &pane.Tabs[0]
		}

		bodyBox := ui.Box(c).FillWidth().Grow(1).MinHeight(0)
		bodyBox.Children(func() {
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
					if s.RenderTabContent != nil {
						s.RenderTabContent(c, pane, *activeTab)
					} else if s.Content != nil {
						s.Content(c)
					}
				}
			}
			// Render split drop-zone preview highlight when hovering drag
			if zone := s.detectPaneDropZone(bodyBox, pane.ID); zone != "" {
				s.renderDropZoneOverlay(c, tok, pane.ID, zone)
			}
		})
		if dragged, dropped := ui.Drop[tabDragPayload](bodyBox); dropped {
			zone := detectDropZone(bodyBox)
			s.MoveOrSplitTab(dragged.PaneID, dragged.TabID, pane.ID, zone)
		}
		if dragged, dropped := ui.Drop[fileDragPayload](bodyBox); dropped && !dragged.IsDir && dragged.RelPath != "" {
			zone := detectDropZone(bodyBox)
			if zone == "" || zone == "center" {
				s.OpenFileInPane(pane.ID, dragged.RelPath, dragged.Title, s.DefaultFileOpenMode(), false)
			} else {
				s.SplitPaneWithTab(pane.ID, WorkspaceTab{
					Kind: TabFile, Title: dragged.Title, FilePath: dragged.RelPath, Mode: s.DefaultFileOpenMode(),
				}, zone)
			}
		}
	})
	// Clicking or interacting inside an unfocused split pane activates it so shortcuts,
	// file-tree clicks, and the companion outline drawer target the pane the user focused.
	if !isFocusedPane && (container.Pressed() || container.Clicked()) {
		s.ActivePaneID = pane.ID
		s.syncLegacyMirror()
		s.Invalidate()
	}
}

func (s *Shell) renderPaneTabBar(c *ui.Context, tok theme.Tokens, pane *PaneNode) {
	if s.middleHeld == nil {
		s.middleHeld = map[string]bool{}
	}
	if s.window != nil {
		s.RightOpen = s.window.RightExpanded()
	}
	isFirstPane := s.firstLeaf(s.RootPane) == pane
	isLastPane := s.lastLeaf(s.RootPane) == pane
	isFocusedPane := pane.ID == s.ActivePaneID
	multiPane := s.leafCount() > 1

	wrapCol := ui.Column(c).FillWidth()
	wrapCol.Children(func() {
		if multiPane {
			accentBarCol := tok.BorderSubtle
			if isFocusedPane {
				accentBarCol = tok.Accent
			}
			ui.Box(c).FillWidth().Height(2).Background(wrapCol.AnimateColor("pane-ind", accentBarCol, theme.MotionInstant))
		}

		bar := ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
			AlignItems(ui.Center).Gap(float32(tok.Spacing4))
		// Avoid double-glass vertical clip seams when hosted inside sidebar.Shell's native ContentTitle chrome
		if !(s.window != nil && s.leafCount() == 1) {
			if multiPane && !isFocusedPane {
				bar.Background(tok.Canvas)
			} else {
				theme.ApplyChromeGlass(bar, tok, false)
			}
		}
		bar.Children(func() {
		// Unified Obsidian Top Bar: compact Back/Forward history controls inline on the left of the tab strip
		if isFirstPane {
			backBtn := components.IconButton(c, lucide.ArrowLeft(), "后退 (Alt+←)", components.IconButtonPlain)
			backBtn.Disabled(!s.CanBack())
			if backBtn.Clicked() {
				s.GoBack()
			}
			fwdBtn := components.IconButton(c, lucide.ArrowRight(), "前进 (Alt+→)", components.IconButtonPlain)
			fwdBtn.Disabled(!s.CanForward())
			if fwdBtn.Clicked() {
				s.GoForward()
			}
		}

		ui.ScrollHorizontal(c.Key("tab-scroll-"+pane.ID)).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				for idx, tab := range pane.Tabs {
					idx, tab := idx, tab
					active := tab.ID == pane.ActiveTabID
					targetBg := ui.Transparent
					targetFg := tok.TextMuted
					if active {
						if tok.LiquidGlass {
							targetBg = tok.Surface.Alpha(0.72)
						} else {
							targetBg = tok.Canvas
						}
						targetFg = tok.Text
					}
					pill := ui.Row(c.Key("ptab-"+pane.ID+"-"+tab.ID)).AlignItems(ui.Center).
						Padding(float32(tok.Spacing4)*0.75, float32(tok.Spacing8)).
						Radius(float32(tok.ControlRadius)).
						Gap(float32(tok.Spacing4)).Shrink(0)
					if active && tok.LiquidGlass {
						pill.Border(1, tok.Border.Alpha(0.65)).Shadow(0, 1, 6, 0, tok.Text.Alpha(0.06))
					} else if !active && pill.Hovered() {
						targetBg = tok.Text.Alpha(0.05)
					}
					pill.Background(pill.AnimateColor("ptab-bg", targetBg, theme.MotionInstant))
					fg := pill.AnimateColor("ptab-fg", targetFg, theme.MotionInstant)
					pill.Children(func() {
						selBtn := ui.ButtonBase(c).Justify(ui.Start).AlignItems(ui.Center).Label("标签:" + tab.Title)
						theme.ApplyTactilePress(selBtn)
						components.Tooltip(c, selBtn, fmtTabTooltip(tab))
						selBtn.Children(func() {
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								var icon *ui.SVG
								switch tab.Kind {
								case TabCalendar:
									icon = lucide.CalendarDays()
								case TabPantry:
									icon = lucide.Package()
								default:
									icon = lucide.FlaskConical()
								}
								icoCol := tok.TextMuted
								if active {
									icoCol = tok.Accent
								}
								ui.Icon(c, icon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(icoCol)
								// Pinned calendar tab renders compactly when space is tight
								if !(tab.Pinned && tab.Kind == TabCalendar && len(pane.Tabs) > 3) {
									tabMaxW := float32(tok.Spacing48) * 3.6
									if len(pane.Tabs) >= 4 {
										tabMaxW = float32(tok.Spacing48) * 2.8
									}
									components.Text(c, tab.Title).FontSize(float32(tok.TextData)).Bold().
										TextColor(fg).SingleLine().Ellipsis("…").MaxWidth(tabMaxW)
								}
								if tab.Pinned {
									ui.Icon(c, lucide.Pin()).Size(float32(tok.IconSm)*0.85, float32(tok.IconSm)*0.85).TextColor(tok.TextMuted)
								}
							})
						})
						if selBtn.Clicked() {
							s.SwitchTabInPane(pane.ID, tab.ID, true)
						}
						// Middle-mouse click closes non-pinned tab cleanly
						if !tab.Pinned {
							if btn, pressed := selBtn.PressedButton(); pressed && btn == 2 {
								s.middleHeld[tab.ID] = true
							} else if s.middleHeld[tab.ID] {
								s.middleHeld[tab.ID] = false
								s.CloseTabInPane(pane.ID, tab.ID)
							}
							clsBtn := components.IconButton(c, lucide.X(), "关闭 "+tab.Title, components.IconButtonCompact)
							if clsBtn.Clicked() {
								s.CloseTabInPane(pane.ID, tab.ID)
							}
						}
					})

					// Drag tab to reorder or drop onto split edge
					pill.Drag(tabDragPayload{PaneID: pane.ID, TabID: tab.ID, Tab: tab})
					if dragged, dropped := ui.Drop[tabDragPayload](pill); dropped && dragged.PaneID == pane.ID && dragged.TabID != tab.ID {
						s.ReorderTabInPane(pane.ID, dragged.TabID, idx)
					}

					// Right-click tab context menu
					kit.ContextMenu(c, pill, func(m *kit.Menu) {
						if m.Item("关闭").Shortcut("Ctrl+W").Disabled(tab.Pinned).Chosen() {
							s.CloseTabInPane(pane.ID, tab.ID)
						}
						if m.Item("关闭其他标签页").Chosen() {
							s.CloseOtherTabs(pane.ID, tab.ID)
						}
						if m.Item("关闭右侧标签页").Chosen() {
							s.CloseTabsToRight(pane.ID, tab.ID)
						}
						m.Separator()
						if m.Item("向右分屏").Shortcut("Ctrl+\\").Chosen() {
							cp := tab
							cp.ID = ""
							s.SplitPaneWithTab(pane.ID, cp, "right")
						}
						if m.Item("向下分屏").Chosen() {
							cp := tab
							cp.ID = ""
							s.SplitPaneWithTab(pane.ID, cp, "down")
						}
						pinLbl := "固定标签页"
						if tab.Pinned {
							pinLbl = "取消固定"
						}
						if m.Item(pinLbl).Chosen() {
							s.TogglePinTab(pane.ID, tab.ID)
						}
						if tab.Kind == TabFile && tab.FilePath != "" {
							m.Separator()
							if m.Item("在文件树中定位").Chosen() {
								s.SelectedTreePath = tab.FilePath
								if s.window != nil && !s.window.LeftExpanded() {
									s.window.SetLeftExpanded(true)
								}
							}
							if m.Item("复制相对路径").Chosen() {
								c.WriteClipboard(tab.FilePath)
							}
							if s.OnRevealInExplorer != nil && m.Item("在资源管理器中显示").Chosen() {
								s.OnRevealInExplorer(tab.FilePath)
							}
						}
					})
				}
			})
		})
		// Keep trailing '+' new-tab button pinned immediately after the horizontal tab scroll
		// so long Chinese tab titles can never push '+' off screen.
		addBtn := components.IconButton(c, lucide.Plus(), "新建实验 (Ctrl+N)", components.IconButtonPlain)
		if addBtn.Clicked() {
			s.ActivePaneID = pane.ID
			s.triggerVaultNewFile("")
		}
		spacerArea := ui.Box(c).Width(float32(tok.Spacing8)).Height(float32(tok.Spacing24)).Shrink(0)
		if spacerArea.DoubleClicked() {
			s.ActivePaneID = pane.ID
			s.triggerVaultNewFile("")
		}

		// Unified Obsidian Top Bar: right-side compact Quick Open & Right Drawer toggle
		if isLastPane {
			if s.Loading {
				components.Text(c, "加载中…").TextColor(tok.TextMuted).FontSize(float32(tok.TextMeta)).Shrink(0)
			}
			if s.Error != "" {
				components.Text(c, s.Error).TextColor(tok.Danger).FontSize(float32(tok.TextMeta)).
					SingleLine().Ellipsis("…").MaxWidth(float32(tok.Spacing48) * 4)
			}
			cmdBtn := components.IconButton(c, lucide.Search(), "快速打开 (Ctrl+O)", components.IconButtonPlain)
			components.Tooltip(c, cmdBtn, "快速打开 (Ctrl+O)")
			if cmdBtn.Clicked() {
				s.PaletteOpen = true
				s.PaletteCursor = 0
			}
			if s.window == nil {
				rdBtn := components.IconButton(c, lucide.PanelRight(), "大纲与速算抽屉", components.IconButtonPlain)
				components.Tooltip(c, rdBtn, "大纲与速算抽屉")
				if s.RightOpen {
					rdBtn.Background(tok.AccentSoftBg)
				}
				if rdBtn.Clicked() {
					s.ToggleRightDrawer()
				}
			}
		}
		})
	})
}

func (s *Shell) lastLeaf(n *PaneNode) *PaneNode {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		return n
	}
	if r := s.lastLeaf(n.Second); r != nil {
		return r
	}
	return s.lastLeaf(n.First)
}

func detectDropZone(box ui.Element) string {
	x, y, over := box.PointerPosition()
	if !over {
		return ""
	}
	r := box.Bounds()
	w, h := r.W, r.H
	if w <= 0 || h <= 0 {
		return "center"
	}
	rx, ry := x/w, y/h
	switch {
	case rx < 0.25:
		return "left"
	case rx > 0.75:
		return "right"
	case ry < 0.25:
		return "up"
	case ry > 0.75:
		return "down"
	default:
		return "center"
	}
}

func (s *Shell) detectPaneDropZone(box ui.Element, _ string) string {
	if _, over := ui.DragOver[tabDragPayload](box); over {
		return detectDropZone(box)
	}
	if dragged, over := ui.DragOver[fileDragPayload](box); over && !dragged.IsDir {
		return detectDropZone(box)
	}
	return ""
}

type splitPreviewMatch struct{ PaneID string }

func (s *Shell) renderDropZoneOverlay(c *ui.Context, tok theme.Tokens, paneID, zone string) {
	pad := float32(tok.Spacing8)
	ov := ui.Box(c.Key("split-ov-"+paneID+"-"+zone)).Absolute().
		Radius(float32(tok.PanelRadius)).
		Background(tok.AccentSoftBg).Border(2, tok.Accent).
		Transition(theme.LiquidSplitTransition(splitPreviewMatch{PaneID: paneID}))
	switch zone {
	case "left":
		ov.Left(pad).Top(pad).Bottom(pad).WidthPercent(48)
	case "right":
		ov.Right(pad).Top(pad).Bottom(pad).WidthPercent(48)
	case "up":
		ov.Left(pad).Right(pad).Top(pad).HeightPercent(48)
	case "down":
		ov.Left(pad).Right(pad).Bottom(pad).HeightPercent(48)
	default:
		ov.Left(pad).Right(pad).Top(pad).Bottom(pad)
	}
}

// RenderSplitView renders the complete headless Obsidian frame (auto-switching to
// the responsive mobile shell when viewport < 680dp or ForceMobilePreview is set).
func (s *Shell) RenderSplitView(c *ui.Context) {
	s.EnsureTabs()
	tok := s.SyncThemeLook(c)
	if s.IsMobileLayout(c) {
		s.renderMobileWorkbench(c, tok)
		return
	}
	s.inSplitViewRoot = true
	defer func() { s.inSplitViewRoot = false }()
	s.syncTourRightDrawer()
	root := ui.Row(c).Fill()
	theme.PaintCanvasBackground(root, tok)
	root.Children(func() {
		// Responsive sidebar clamping on narrow viewports (< 1100 DIP) so the center
		// document editor always retains >= 460 DIP instead of clipping title/tables.
		winW, _ := c.Size()
		leftW := float32(284)
		rightW := float32(tok.Spacing48) * 6.2
		if winW > 0 && winW < 1100 {
			leftW = 220
			if s.RightOpen {
				rightW = 220
			}
		}
		// Content/Sidebar panels stay crisp solid structural surfaces (no stacked glass-on-glass washout)
		leftCol := ui.Column(c).Width(leftW).FillHeight().
			Background(tok.RailBackground).Border(1, tok.Border.Alpha(0.75))
		leftCol.Children(func() {
			ui.Box(c).Grow(1).FillWidth().Children(func() {
				s.navigation(c)
			})
			ui.Box(c).Padding(float32(tok.Spacing8)).Children(func() {
				s.navigationFooter(c)
			})
		})
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
			// Unified Obsidian-style 2-bar header: tabs + back/forward + quick-open live in ONE top tab bar
			ui.Box(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
				s.page(c)
			})
		})
		if s.RightOpen {
			rightCol := ui.Column(c).Width(rightW).FillHeight().
				Background(tok.SidebarBackground).Border(1, tok.Border.Alpha(0.75))
			rightCol.Children(func() {
				hdrRow := ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
					Border(1, tok.Border.Alpha(0.65)).AlignItems(ui.Center)
				theme.ApplyChromeGlass(hdrRow, tok, false)
				hdrRow.Children(func() {
					s.rightDrawerHeader(c)
					ui.Spacer(c)
					if components.IconButton(c, lucide.X(), "收起抽屉", components.IconButtonCompact).Clicked() {
						s.ToggleRightDrawer()
					}
				})
				ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
					s.rightDrawer(c)
				})
			})
		}
		s.renderSpotlightTour(c, tok)
	})
}

const timerStripCompactN = 2

func orderedTimers(timers []runs.ActiveTimer) []runs.ActiveTimer {
	out := make([]runs.ActiveTimer, len(timers))
	copy(out, timers)
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := out[i].RemainingS, out[j].RemainingS
		if ri == nil || rj == nil {
			return rj == nil && ri != nil
		}
		return *ri < *rj
	})
	return out
}

func (s *Shell) timerStrip(c *ui.Context) {
	if len(s.ActiveTimers) == 0 {
		return
	}
	timers := orderedTimers(s.ActiveTimers)
	shown := timers
	hidden := 0
	if !s.TimersExpanded && len(timers) > timerStripCompactN {
		shown = timers[:timerStripCompactN]
		hidden = len(timers) - timerStripCompactN
	}
	t := c.Theme()
	tok := theme.GetTokens(t)
	ui.Row(c).FillWidth().Wrap().Padding(t.Space(1), t.Space(2)).Gap(t.Space(2)).
		Background(t.Surface).Children(func() {
		components.Text(c, "计时中").TextColor(t.TextMuted).FontSize(float32(tok.TextMeta))
		for _, timer := range shown {
			timer := timer
			label := timer.RunTitle + " · " + formatTimerClock(timer)
			if components.Button(c, label, components.ButtonGhost).Clicked() && s.OnTimerSelect != nil {
				s.OnTimerSelect(timer.RunID)
			}
		}
		if hidden > 0 {
			if components.Button(c, fmt.Sprintf("＋%d 更多", hidden), components.ButtonGhost).Clicked() {
				s.TimersExpanded = true
			}
		}
		if s.TimersExpanded && len(timers) > timerStripCompactN {
			if components.Button(c, "收起", components.ButtonGhost).Clicked() {
				s.TimersExpanded = false
			}
		}
	})
}

func formatTimerClock(timer runs.ActiveTimer) string {
	if timer.RemainingS != nil {
		return "剩余 " + formatDuration(*timer.RemainingS)
	}
	return "已计时 " + formatDuration(timer.ElapsedS)
}

func formatDuration(total uint64) string {
	h, m, sec := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

func statusLabel(status string) string {
	switch status {
	case "in_progress":
		return "进行中"
	case "planned":
		return "计划中"
	case "completed":
		return "已完成"
	case "blocked":
		return "阻塞"
	case "cancelled":
		return "已取消"
	}
	return status
}

func (s *Shell) Quit() {
	if w := s.MyGoWindow(); w != nil {
		w.OnClosed(func() { mygo.App.Quit() })
	}
}
