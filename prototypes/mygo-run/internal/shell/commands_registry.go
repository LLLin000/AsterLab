package shell

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
)

// PushToast enqueues an optimistic non-blocking notification / undo snackbar.
func (s *Shell) PushToast(t kit.Toast) {
	if s == nil {
		return
	}
	if s.Toasts == nil {
		s.Toasts = kit.NewToastQueue()
	}
	s.Toasts.Push(t)
}

func (s *Shell) activeCommandContext() kit.CommandContext {
	tab := s.ActiveTab()
	activePage := ""
	isFile := false
	modeLab := false
	modeSource := false
	if tab != nil {
		switch tab.Kind {
		case TabCalendar:
			activePage = "calendar"
		case TabPantry:
			activePage = "pantry"
		case TabFile:
			activePage = "file"
			isFile = true
			modeLab = tab.Mode == ModeLab
			modeSource = tab.Mode == ModeSource
		}
	}
	return kit.CommandContext{
		ModalOpen:        s.Layers.HasOpen(),
		ActivePage:       activePage,
		IsFileTab:        isFile,
		ModeLab:          modeLab,
		ModeSource:       modeSource,
		TextInputFocused: false,
	}
}

// ensureCommandRegistry keeps the single-source VS Code/Zed-style CommandRegistry synchronized
// with hotkeys, command palette entries, and `when` context rules.
func (s *Shell) ensureCommandRegistry() *kit.CommandRegistry {
	if s == nil {
		return nil
	}
	if s.Commands == nil {
		s.Commands = kit.NewCommandRegistry()
	}
	s.Commands.SetOverrides(s.Hotkeys)

	specs := []kit.CommandSpec{
		{
			ID: "vault.new_exp", Title: "新建实验", Subtitle: "在当前项目与日期新建 .asterml 实验文档",
			Category: "导航", DefaultShortcut: "Ctrl+N", Icon: lucide.Plus(),
			Run: func() { s.triggerCreateForDate(s.ensureSelectedDate()) },
		},
		{
			ID: "nav.global_search", Title: "Vault 全局全文搜索", Subtitle: "跨所有 .asterml 实验步骤、配方、试剂编号与笔记全文检索",
			Category: "导航", DefaultShortcut: "Ctrl+Shift+F", Icon: lucide.Search(),
			Run: func() {
				s.GlobalSearchOpen = true
				s.refreshGlobalSearch()
			},
		},
		{
			ID: "nav.right_drawer", Title: "打开 / 收起右侧步骤大纲与速算抽屉", Subtitle: "查看当前实验步骤大纲、动物房检查器或稀释速算器",
			Category: "工作区", DefaultShortcut: "Ctrl+Shift+R", Icon: lucide.PanelRight(),
			Run: func() { s.ToggleRightDrawer() },
		},
		{
			ID: "pane.split_right", Title: "向右分屏打开当前标签页", Subtitle: "并排对照两份实验、同一实验的不同区域或储藏室",
			Category: "工作区", DefaultShortcut: "Ctrl+\\", Icon: lucide.Columns2(),
			Run: func() { s.ToggleSplit() },
		},
		{
			ID: "nav.calendar", Title: "切换到实验日程", Subtitle: "查看月/周实验安排与排期",
			Category: "导航", DefaultShortcut: "Ctrl+1", Icon: lucide.CalendarDays(),
			Run: func() { s.OpenCalendarTab() },
		},
		{
			ID: "nav.pantry", Title: "打开储藏室与动物房", Subtitle: "管理冰箱货架、冻存盒孔位、试剂库存与实验动物笼卡",
			Category: "导航", Icon: lucide.FlaskConical(),
			Run: func() { s.OpenPantryTab() },
		},
		{
			ID: "doc.toggle_mode", Title: "切换编辑 / 实验操作模式", Subtitle: "在结构编辑与实时实验记录态间无损切换",
			Category: "编辑器", DefaultShortcut: "Ctrl+E", Icon: lucide.FlaskConical(),
			When: func(ctx kit.CommandContext) bool { return ctx.IsFileTab && !ctx.ModalOpen },
			Run: func() {
				if t := s.ActiveTab(); t != nil && t.Kind == TabFile {
					if t.Mode == ModeEdit {
						s.SetActiveTabMode(ModeLab)
					} else {
						s.SetActiveTabMode(ModeEdit)
					}
				}
			},
		},
		{
			ID: "app.settings", Title: "打开设置", Subtitle: "配置外观主题、界面密度、快捷键与更新",
			Category: "系统", DefaultShortcut: "Ctrl+,", Icon: lucide.Settings(),
			Run: func() { s.VaultSettingsOpen = true },
		},
		{
			ID: "theme.preset.paper", Title: "切换配色：纯净白纸 (默认)", Subtitle: "100% 纯净白纸底搭配冷灰层次",
			Category: "外观",
			Run: func() { s.ThemePreset = theme.PresetPaper; s.notifySettingsChanged(); s.Invalidate() },
		},
		{
			ID: "theme.preset.zinc", Title: "切换配色：冷银极简 (Cool Zinc)", Subtitle: "中性冷灰底配纯白卡片",
			Category: "外观",
			Run: func() { s.ThemePreset = theme.PresetZinc; s.notifySettingsChanged(); s.Invalidate() },
		},
		{
			ID: "theme.preset.warm", Title: "切换配色：经典暖润实验纸 (Warm Paper)", Subtitle: "温润米白护眼纸面 (#f7f6f3)",
			Category: "外观",
			Run: func() { s.ThemePreset = theme.PresetWarm; s.notifySettingsChanged(); s.Invalidate() },
		},
	}
	for _, sp := range specs {
		s.Commands.Register(sp)
	}
	return s.Commands
}

func (s *Shell) registerShellOverlays(c *ui.Context) {
	if s == nil {
		return
	}
	s.Layers.BeginFrame()
	s.Layers.Register("welcome", s.WelcomeOpen, func() { s.WelcomeOpen = false })
	s.Layers.Register("vault-launcher", s.VaultLauncherOpen, func() { s.VaultLauncherOpen = false })
	s.Layers.Register("feedback", s.FeedbackOpen, func() { s.FeedbackOpen = false })
	s.Layers.Register("vault-settings", s.VaultSettingsOpen, func() { s.VaultSettingsOpen = false })
	s.Layers.Register("global-search", s.GlobalSearchOpen, func() { s.GlobalSearchOpen = false })
	s.Layers.Register("command-palette", s.PaletteOpen, func() { s.PaletteOpen = false })
	s.Layers.Register("spotlight-tour", s.TourOpen, func() {
		s.TourOpen = false
		s.syncTourRightDrawer()
	})
}
