package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/astercore"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/diag"
	"asterlab-mygo-prototype/internal/schedule"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/tools"
	"asterlab-mygo-prototype/internal/vault"
)

const releaseBlankTemplateDoc = `---
title: 空白实验
---

`

func defaultTemplateSeeds() map[string]string {
	return map[string]string{
		"空白实验.asterml": releaseBlankTemplateDoc,
	}
}

type vaultPrefs struct {
	LastVaultRoot string   `json:"lastVaultRoot,omitempty"`
	RecentVaults  []string `json:"recentVaults,omitempty"`
	UsingSandbox  bool     `json:"usingSandbox,omitempty"`
}

func vaultPrefsPath() string {
	dir := workspaceDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "vault-prefs.json")
}

func loadVaultPrefs() vaultPrefs {
	p := vaultPrefsPath()
	if p == "" {
		return vaultPrefs{}
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return vaultPrefs{}
	}
	var prefs vaultPrefs
	if json.Unmarshal(raw, &prefs) != nil {
		return vaultPrefs{}
	}
	return prefs
}

func saveVaultPrefs(prefs vaultPrefs) {
	p := vaultPrefsPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if raw, err := json.MarshalIndent(prefs, "", "  "); err == nil {
		_ = asterdoc.WriteFileAtomic(p, raw)
	}
}

func isSandboxVaultPath(root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	return strings.EqualFold(filepath.Clean(root), filepath.Clean(vault.DefaultVaultDir()))
}

func loadLastVaultRoot() string {
	if strings.TrimSpace(os.Getenv("ASTERLAB_VAULT_DIR")) != "" {
		return ""
	}
	prefs := loadVaultPrefs()
	root := strings.TrimSpace(prefs.LastVaultRoot)
	if root == "" {
		return ""
	}
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		return root
	}
	return ""
}

func saveLastVaultRoot(root string) {
	root = strings.TrimSpace(root)
	if root == "" {
		return
	}
	clean := filepath.Clean(root)
	prefs := loadVaultPrefs()
	prefs.LastVaultRoot = clean
	prefs.UsingSandbox = isSandboxVaultPath(clean)
	next := []string{clean}
	for _, old := range prefs.RecentVaults {
		oc := filepath.Clean(strings.TrimSpace(old))
		if oc == "" || strings.EqualFold(oc, clean) {
			continue
		}
		next = append(next, oc)
		if len(next) >= 8 {
			break
		}
	}
	prefs.RecentVaults = next
	saveVaultPrefs(prefs)
}

func (a *app) syncRecentVaultsToShell() {
	if a.shell == nil {
		return
	}
	cur := ""
	if a.vault != nil {
		cur = filepath.Clean(a.vault.Root)
	}
	a.shell.IsTempSandboxVault = isSandboxVaultPath(cur)
	prefs := loadVaultPrefs()
	seen := map[string]bool{}
	var out []shell.RecentVaultEntry
	addPath := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		clean := filepath.Clean(p)
		key := strings.ToLower(clean)
		if seen[key] {
			return
		}
		if info, err := os.Stat(clean); err != nil || !info.IsDir() {
			return
		}
		seen[key] = true
		name := filepath.Base(clean)
		sandbox := isSandboxVaultPath(clean)
		if sandbox {
			name = "临时体验库"
		}
		out = append(out, shell.RecentVaultEntry{
			Name:    name,
			Path:    clean,
			Active:  cur != "" && strings.EqualFold(cur, clean),
			Sandbox: sandbox,
		})
	}
	if cur != "" {
		addPath(cur)
	}
	for _, p := range prefs.RecentVaults {
		addPath(p)
	}
	a.shell.RecentVaults = out
}

func (a *app) initVaultWorkspace(vaultRoot string) error {
	firstRunNoVault := false
	if strings.TrimSpace(vaultRoot) == "" {
		if saved := loadLastVaultRoot(); saved != "" {
			vaultRoot = saved
		} else if strings.TrimSpace(os.Getenv("ASTERLAB_VAULT_DIR")) == "" && !testing.Testing() {
			firstRunNoVault = true
		}
	}
	v, err := vault.Open(vaultRoot)
	if err != nil {
		return err
	}
	_ = v.SeedTemplates(defaultTemplateSeeds())
	_ = v.SeedAgentGuide()
	// Only migrate legacy `%LOCALAPPDATA%/AsterLab/data/experiments` into the default
	// built-in Vault; never pollute user-chosen / freshly created Vault directories.
	if dataDir := workspaceDataDir(); dataDir != "" &&
		strings.EqualFold(filepath.Clean(v.Root), filepath.Clean(vault.DefaultVaultDir())) {
		_, _ = v.MigrateLegacyExperiments(dataDir)
	}
	a.vault = v
	a.resetDevFeedbackDirCache()
	a.vaultIndex = vault.NewVaultIndex(v)
	a.docStore = vault.NewDocStore(v)
	a.ensureLiveViewsMap()
	if a.tools != nil {
		a.tools.ResetPantryState()
	}

	if a.shell != nil {
		a.shell.Error = ""
		a.shell.VaultPathInput = v.Root
		a.shell.FolderTemplates = v.Config.FolderTemplates
		a.shell.VaultLauncherSeedDemo = true
		if firstRunNoVault {
			a.shell.VaultLauncherOpen = true
		}
		a.shell.HideAssetsInTree = v.Config.HideAssetsInTree
		a.shell.DefaultProjectDir = v.Config.DefaultProjectDir
		a.shell.ThemeSource = v.Config.ThemeSource
		lgVal := true
		if v.Config.LiquidGlass != nil {
			lgVal = *v.Config.LiquidGlass
		}
		a.shell.SetInitialThemeLook(v.Config.ThemePreset, lgVal)
		a.shell.TextScale = v.Config.TextScale
		a.shell.Density = v.Config.Density
		a.shell.ReduceMotion = v.Config.ReduceMotion
		a.shell.FeedbackMode = v.Config.FeedbackMode
		a.shell.DefaultOpenMode = v.Config.DefaultOpenMode
		a.shell.DefaultPlateFormat = v.Config.DefaultPlateFormat
		a.shell.DefaultExcessPct = v.Config.DefaultExcessPct
		if v.Config.AutoArchiveAssets != nil {
			a.shell.AutoArchiveAssets = *v.Config.AutoArchiveAssets
		} else {
			a.shell.AutoArchiveAssets = true
		}
		if len(v.Config.Hotkeys) > 0 {
			a.shell.Hotkeys = v.Config.Hotkeys
		}
		autoHide := true
		if v.Config.MobileAutoHideDock != nil {
			autoHide = *v.Config.MobileAutoHideDock
		}
		editTb := true
		if v.Config.MobileEditToolbar != nil {
			editTb = *v.Config.MobileEditToolbar
		}
		a.shell.SetInitialMobilePrefs(v.Config.MobilePullAction, autoHide, editTb)
		a.shell.SyncThemeLook(nil)
		a.shell.ResetWorkspaceForVault()
		var persist shell.WorkspacePersistState
		if err := v.LoadWorkspaceJSON(&persist); err == nil {
			repaired := a.shell.RestorePersistState(persist)
			pruned := a.shell.PruneMissingFileTabs(func(rel string) bool {
				_, statErr := os.Stat(v.AbsPath(rel))
				return statErr == nil
			})
			if repaired || pruned {
				a.saveVaultWorkspace()
			}
		}
	}
	a.refreshVaultTreeAndCalendar()
	a.syncRecentVaultsToShell()
	a.lastVaultSig, a.lastPantrySig = computeVaultDiskSignatures(v.Root)
	return nil
}

func (a *app) saveVaultSettingsFromShell() {
	if a.vault == nil || a.shell == nil {
		return
	}
	a.vault.Config.HideAssetsInTree = a.shell.HideAssetsInTree
	a.vault.Config.DefaultProjectDir = a.shell.DefaultProjectDir
	a.vault.Config.ThemeSource = a.shell.ThemeSource
	a.vault.Config.ThemePreset = a.shell.ThemePreset
	lg := a.shell.LiquidGlass
	a.vault.Config.LiquidGlass = &lg
	a.vault.Config.TextScale = a.shell.TextScale
	a.vault.Config.Density = a.shell.Density
	a.vault.Config.ReduceMotion = a.shell.ReduceMotion
	a.vault.Config.FeedbackMode = a.shell.FeedbackMode
	a.vault.Config.DefaultOpenMode = a.shell.DefaultOpenMode
	a.vault.Config.DefaultPlateFormat = a.shell.DefaultPlateFormat
	a.vault.Config.DefaultExcessPct = a.shell.DefaultExcessPct
	autoArch := a.shell.AutoArchiveAssets
	a.vault.Config.AutoArchiveAssets = &autoArch
	if len(a.shell.Hotkeys) > 0 {
		a.vault.Config.Hotkeys = a.shell.Hotkeys
	}
	a.vault.Config.MobilePullAction = a.shell.MobilePullAction
	autoHide := a.shell.MobileAutoHideDock
	a.vault.Config.MobileAutoHideDock = &autoHide
	editTb := a.shell.MobileEditToolbar
	a.vault.Config.MobileEditToolbar = &editTb
	_ = a.vault.SaveConfig()
}

func (a *app) refreshVaultTreeAndCalendar() {
	if a.vault == nil || a.shell == nil {
		return
	}
	tree, files, err := a.vault.ScanTree()
	if err != nil {
		return
	}
	a.shell.VaultTree = tree
	a.shell.VaultFiles = files
	a.shell.VaultFolders = a.vault.ProjectFolders()
	a.shell.TrashItems = a.vault.ListTrash()
	if a.vaultIndex != nil {
		_ = a.vaultIndex.Refresh()
		a.shell.VaultTimers = a.vaultIndex.ActiveTimers()
	}

	var exps []shell.ExperimentItem
	var designs []shell.DesignItem
	for _, f := range files {
		if f.IsTemplate || f.IsAsset {
			continue
		}
		stage := shell.StageReady
		switch f.Status {
		case "in_progress":
			stage = shell.StageInProgress
		case "completed":
			stage = shell.StageCompleted
		}
		folder := f.Folder
		if folder == "" {
			folder = "根目录"
		}
		exps = append(exps, shell.ExperimentItem{
			ID:        f.RelPath,
			DesignID:  f.RelPath,
			FilePath:  f.RelPath,
			Folder:    folder,
			Title:     f.Title,
			Date:      f.Date,
			StartHour: f.StartHour,
			Stage:     stage,
			Summary:   folder,
			FileName:  filepath.Base(f.RelPath),
		})
		designs = append(designs, shell.DesignItem{
			ID:    f.RelPath,
			Title: f.Title,
			Meta:  f.Date,
		})
	}
	a.shell.Experiments = exps
	a.shell.Designs = designs
	a.syncPantrySnapshotToShell()
}

func (a *app) syncPantrySnapshotToShell() {
	if a.shell == nil {
		return
	}
	defer a.syncCalendarEventsToShell()
	if a.tools == nil {
		a.shell.PantryItems = nil
		return
	}
	if !a.tools.PantryLoaded {
		a.tools.ReloadPantry()
	}
	if len(a.tools.Pantry.Items) == 0 {
		a.shell.PantryItems = nil
		return
	}
	qItems := make([]shell.QuickPantryItem, 0, len(a.tools.Pantry.Items))
	for _, p := range a.tools.Pantry.Items {
		loc := a.tools.FormatPantryDisplayLocation(p)
		if loc == "" {
			loc = p.Location
		}
		qItems = append(qItems, shell.QuickPantryItem{
			ID:        p.ID,
			Name:      p.Name,
			Location:  loc,
			Status:    p.Status,
			ExpiresOn: p.ExpiresOn,
			LotNumber: p.LabelCode,
		})
	}
	a.shell.PantryItems = qItems
}

func (a *app) syncCalendarEventsToShell() {
	if a.shell == nil || a.tools == nil {
		return
	}
	now := time.Now()
	evs := make([]schedule.Event, 0, len(a.tools.Pantry.Items)*2+len(a.tools.VivariumCages)*3)
	evs = append(evs, a.tools.ProjectReagentCalendarEvents(now)...)
	evs = append(evs, a.tools.ProjectVivariumCalendarEvents(now)...)
	a.shell.CalendarEvents = evs
}

func (a *app) saveVaultWorkspace() {
	if a.vault == nil || a.shell == nil {
		return
	}
	st := a.shell.ExportPersistState()
	_ = a.vault.SaveWorkspaceJSON(st)
}

func (a *app) openVaultFileInActivePane(relPath, title string) {
	if a.shell == nil || strings.TrimSpace(relPath) == "" {
		return
	}
	a.shell.EnsureTabs()
	paneID := a.shell.ActivePaneID
	if title == "" && a.vault != nil {
		for _, f := range a.shell.VaultFiles {
			if f.RelPath == relPath {
				title = f.Title
				break
			}
		}
	}
	mode := a.shell.DefaultFileOpenMode()
	a.shell.OpenFileInPane(paneID, relPath, title, mode, false)
	a.saveVaultWorkspace()
}

func (a *app) openNewExperimentDialog(folderRel, dateYMD string) {
	a.openNewExperimentDialogAt(folderRel, dateYMD, "")
}

func (a *app) openNewExperimentDialogAt(folderRel, dateYMD, startTimeHHMM string) {
	if a.vault != nil && strings.TrimSpace(folderRel) == "" {
		switch a.vault.Config.DefaultProjectDir {
		case "/":
			folderRel = ""
		case "":
			folderRel = a.vault.Config.LastProjectDir
		default:
			folderRel = a.vault.Config.DefaultProjectDir
		}
	}
	a.newExpFolder = folderRel
	a.newExpTitle = ""
	a.newExpDate = strings.TrimSpace(dateYMD)
	a.newExpStartTime = asterdoc.NormalizeStartTimeHHMM(startTimeHHMM)
	// UI-10: Auto-apply folder's bound default template from `.asterlab/config.json` when configured.
	if a.vault != nil && a.vault.Config.FolderTemplates != nil {
		if boundTpl := strings.TrimSpace(a.vault.Config.FolderTemplates[folderRel]); boundTpl != "" {
			a.newExpTemplate = boundTpl
		}
	}
	if a.newExpTemplate == "" {
		a.newExpTemplate = "templates/空白实验.asterml"
	}
	a.newExpTplQuery = ""
	a.newExpErr = ""
	a.newExpOpen = true
}

func (a *app) renderVaultNewExperimentDialog(c *ui.Context) {
	if !a.newExpOpen || a.vault == nil {
		return
	}
	tok := theme.GetTokens(c.Theme())
	open := a.newExpOpen
	folders := append([]string{""}, a.vault.ProjectFolders()...)
	var templates []vault.FileEntry
	if a.shell != nil {
		for _, f := range a.shell.VaultFiles {
			if f.IsTemplate {
				templates = append(templates, f)
			}
		}
	}

	ui.Box(c.Key("vault-new-exp-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: "新建实验"}, func(panel ui.Element) {
		panel.Width(float32(tok.Spacing48)*11).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.FlaskConical()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
				ui.Text(c, "新建实验").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			})

			// Title
			titleCol := ui.Column(c.Key("new-exp-title-col")).FillWidth().Gap(float32(tok.Spacing4))
			theme.ApplyNudge(titleCol, a.newExpNudgeSeq)
			titleCol.Children(func() {
				ui.Text(c, "实验名称").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted)
				tb := components.TextBox(c, &a.newExpTitle, components.TextBoxOptions{}).Placeholder("例如：CCK-8 药物敏感性测定")
				if tb.Submitted() {
					if strings.TrimSpace(a.newExpTitle) == "" {
						a.newExpErr = "请输入实验名称"
						a.newExpNudgeSeq++
					} else {
						a.confirmCreateVaultExperiment()
					}
				}
			})

			// Location (standard path input + quick-pick existing folder dropdown) + Date + Optional Start Time
			ui.Row(c).FillWidth().AlignItems(ui.End).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "保存位置（路径）").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted).SingleLine()
					ui.Row(c).FillWidth().Gap(float32(tok.Spacing4)).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Grow(1).MinWidth(0).Children(func() {
							components.TextBox(c, &a.newExpFolder, components.TextBoxOptions{}).
								Placeholder("留空存入根目录；支持 Fig4 或 Fig4/子目录")
						})
						folderPick := a.newExpFolder
						ui.Box(c).Width(float32(tok.Spacing48)*2.05).Shrink(0).Children(func() {
							sel := components.Select(c, &folderPick)
							sel.Trigger.FillWidth().Children(func() {
								ui.Text(c, "已有目录").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
							})
							sel.Popup(func(p ui.Element) {
								ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48) * 5.2).Children(func() {
									ui.Column(c).FillWidth().Gap(2).Children(func() {
										for _, f := range folders {
											f := f
											lbl := f
											if lbl == "" {
												lbl = "根目录 (/)"
											}
											sel.Item(f).Children(func() {
												ui.Text(c, lbl).FontSize(float32(tok.TextData)).TextColor(tok.Text)
											})
										}
									})
								})
							})
						})
						if folderPick != a.newExpFolder {
							a.newExpFolder = folderPick
						}
					})
				})
				ui.Column(c).Width(float32(tok.Spacing48)*2.7).Shrink(0).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "日期").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted).SingleLine()
					components.TextBox(c, &a.newExpDate, components.TextBoxOptions{}).Placeholder("YYYY-MM-DD")
				})
				ui.Column(c).Width(float32(tok.Spacing48)*1.8).Shrink(0).Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "时段").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted).SingleLine()
					components.TextBox(c, &a.newExpStartTime, components.TextBoxOptions{}).Placeholder("如 14:00")
				})
			})

			if strings.TrimSpace(a.newExpFolder) != "" {
				if norm, normErr := vault.NormalizeFolderPath(a.newExpFolder); normErr != nil {
					ui.Text(c, normErr.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.Danger)
				} else if norm != "" {
					exists := false
					for _, f := range folders {
						if f == norm {
							exists = true
							break
						}
					}
					if !exists {
						ui.Text(c, "将自动新建目录："+norm).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent)
					}
				}
			}

			// Template picker
			if len(templates) > 0 {
				ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
					ui.Text(c, "选用模板").FontSize(float32(tok.TextData)).Bold().TextColor(tok.TextMuted)
					tSel := components.Select(c, &a.newExpTemplate)
					tSel.Trigger.Children(func() {
						lbl := strings.TrimSuffix(filepath.Base(a.newExpTemplate), ".asterml")
						if lbl == "" || lbl == "." {
							lbl = "空白实验"
						}
						tSel.Trigger.Label(lbl)
						ui.Text(c, lbl).FontSize(float32(tok.TextData)).TextColor(tok.Text)
					})
					tSel.Popup(func(p ui.Element) {
						if len(templates) > 8 {
							tSel.SearchField(&a.newExpTplQuery).Label("筛选模板").Placeholder("搜索模板…")
						}
						q := strings.ToLower(strings.TrimSpace(a.newExpTplQuery))
						ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48) * 5.8).Children(func() {
							ui.Column(c).FillWidth().Gap(2).Children(func() {
								matched := 0
								hasBlank := false
								for _, tpl := range templates {
									if tpl.Name == "空白实验" {
										hasBlank = true
										break
									}
								}
								if !hasBlank && (q == "" || strings.Contains("空白实验", q)) {
									matched++
									tSel.Item("").Children(func() {
										ui.Text(c, "空白实验").FontSize(float32(tok.TextData)).TextColor(tok.Text)
									})
								}
								for _, tpl := range templates {
									tpl := tpl
									if q != "" && !strings.Contains(strings.ToLower(tpl.Name), q) {
										continue
									}
									matched++
									tSel.Item(tpl.RelPath).Children(func() {
										ui.Text(c, tpl.Name).FontSize(float32(tok.TextData)).TextColor(tok.Text).SingleLine().Ellipsis("…")
									})
								}
								if matched == 0 {
									ui.Text(c, "无匹配模板").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
										Padding(float32(tok.Spacing8), float32(tok.Spacing8))
								}
							})
						})
					})
				})
			}
			if a.newExpErr != "" {
				ui.Text(c, a.newExpErr).FontSize(float32(tok.TextData)).TextColor(tok.Danger)
			}

			ui.Row(c).FillWidth().Justify(ui.End).Gap(float32(tok.Spacing8)).Children(func() {
				if components.Button(c, "取消", components.ButtonGhost).Clicked() {
					a.newExpOpen = false
				}
				if components.Button(c, "创建并打开", components.ButtonPrimary).Clicked() {
					a.confirmCreateVaultExperiment()
				}
			})
		})
	})
	})
	if !open {
		a.newExpOpen = false
	}
}

func (a *app) confirmCreateVaultExperiment() {
	if a.vault == nil {
		return
	}
	folder, folderErr := vault.NormalizeFolderPath(a.newExpFolder)
	if folderErr != nil {
		a.newExpErr = folderErr.Error()
		a.newExpNudgeSeq++
		return
	}
	if strings.TrimSpace(a.newExpDate) != "" && asterdoc.NormalizeDateYMD(a.newExpDate) == "" {
		a.newExpErr = "日期格式请填写 YYYY-MM-DD（或留空）"
		a.newExpNudgeSeq++
		return
	}
	if strings.TrimSpace(a.newExpStartTime) != "" && asterdoc.NormalizeStartTimeHHMM(a.newExpStartTime) == "" {
		a.newExpErr = "开始时间格式请填写 HH:MM（如 09:30，或留空）"
		a.newExpNudgeSeq++
		return
	}
	a.newExpErr = ""
	title := strings.TrimSpace(a.newExpTitle)
	tplSrc := releaseBlankTemplateDoc
	if a.newExpTemplate != "" {
		if ld, err := a.docStore.Open(a.newExpTemplate); err == nil && strings.TrimSpace(ld.Source) != "" {
			tplSrc = ld.Source
		}
	}
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(a.newExpTemplate), ".asterml")
		if title == "" || title == "." {
			title = "新实验"
		}
	}
	entry, err := a.vault.CreateExperiment(folder, title, strings.TrimSpace(a.newExpDate), tplSrc)
	a.newExpOpen = false
	if err != nil {
		a.syncErr = fmt.Sprintf("创建实验失败：%v", err)
		return
	}
	if stNorm := asterdoc.NormalizeStartTimeHHMM(a.newExpStartTime); stNorm != "" {
		_ = a.vault.SetExperimentStartTime(entry.RelPath, stNorm)
	}
	a.vault.Config.LastProjectDir = folder
	_ = a.vault.SaveConfig()
	a.refreshVaultTreeAndCalendar()
	a.openVaultFileInActivePane(entry.RelPath, entry.Title)
}

func (a *app) moveVaultFile(srcRel, destFolderRel string) {
	if a.vault == nil || srcRel == "" {
		return
	}
	newRel, err := a.vault.Move(srcRel, destFolderRel)
	if err != nil {
		a.syncErr = fmt.Sprintf("移动失败：%v", err)
		return
	}
	if a.docStore != nil {
		a.docStore.RenameKey(srcRel, newRel)
	}
	if a.shell != nil {
		a.shell.SyncFileRename(srcRel, newRel, "")
	}
	a.refreshVaultTreeAndCalendar()
	a.saveVaultWorkspace()
}

func (a *app) deleteVaultFile(relPath string) {
	if a.vault == nil || relPath == "" {
		return
	}
	if _, err := a.vault.Trash(relPath); err != nil {
		a.syncErr = fmt.Sprintf("移入回收站失败：%v", err)
		return
	}
	if a.docStore != nil {
		a.docStore.Evict(relPath)
	}
	if a.shell != nil {
		a.shell.CloseTabsForFile(relPath)
	}
	a.refreshVaultTreeAndCalendar()
	a.saveVaultWorkspace()
}

func (a *app) restoreVaultTrash(trashName string) {
	if a.vault == nil || trashName == "" {
		return
	}
	restoredRel, err := a.vault.RestoreFromTrash(trashName)
	if err != nil {
		a.syncErr = fmt.Sprintf("恢复文件失败：%v", err)
		return
	}
	a.refreshVaultTreeAndCalendar()
	if strings.HasSuffix(strings.ToLower(restoredRel), ".asterml") {
		a.openVaultFileInActivePane(restoredRel, "")
	}
}

func (a *app) syncActiveDocCompanionState(relPath, source string, doc *asterdoc.Document) {
	if a.shell == nil || a.vault == nil || relPath == "" {
		return
	}
	cacheKey := fmt.Sprintf("%s|%d|%x", relPath, len(source), asterdoc.FastSourceHash(relPath, source))
	if a.lastCompanionKey == cacheKey && a.shell.CurrentDocRelPath == relPath {
		return
	}
	a.lastCompanionKey = cacheKey
	if a.shell.CurrentDocRelPath != relPath {
		a.shell.CurrentRelatedDocs = nil
	}
	a.shell.CurrentDocRelPath = relPath
	a.shell.CurrentSnapshots = a.vault.ListSnapshots(relPath)
	if a.vaultIndex != nil {
		a.shell.CurrentBacklinks = a.vaultIndex.BacklinksFor(relPath)
		a.shell.VaultTimers = a.vaultIndex.ActiveTimers()
	}
	knownPantry := map[string]bool{}
	var matchedReagents []shell.MatchedReagentItem
	var pantryDiags []shell.DiagnosticItem
	if a.tools != nil {
		srcLower := strings.ToLower(source)
		seenReag := map[string]bool{}
		for _, p := range a.tools.Pantry.Items {
			if p.LabelCode != "" {
				knownPantry[strings.TrimSpace(p.LabelCode)] = true
			}
			pName := strings.TrimSpace(p.Name)
			if len([]rune(pName)) < 2 || seenReag[pName] {
				continue
			}
			if !strings.Contains(srcLower, strings.ToLower(pName)) {
				continue
			}
			seenReag[pName] = true
			matchedReagents = append(matchedReagents, shell.MatchedReagentItem{
				ID:        p.ID,
				Name:      pName,
				Location:  p.Location,
				Status:    p.Status,
				ExpiresOn: p.ExpiresOn,
			})
			switch p.Status {
			case "expired":
				pantryDiags = append(pantryDiags, shell.DiagnosticItem{
					Code:         "PANTRY_EXPIRED",
					Severity:     string(astercore.SeverityBlocking),
					Path:         "pantry." + p.ID,
					Message:      fmt.Sprintf("文中提及试剂「%s」在储藏室中已过期（%s）", pName, p.ExpiresOn),
					PantryItemID: p.ID,
				})
			case "exhausted":
				pantryDiags = append(pantryDiags, shell.DiagnosticItem{
					Code:         "PANTRY_EXHAUSTED",
					Severity:     string(astercore.SeverityNeedsConfirmation),
					Path:         "pantry." + p.ID,
					Message:      fmt.Sprintf("文中提及试剂「%s」在储藏室中已用尽", pName),
					PantryItemID: p.ID,
				})
			case "near_expiry":
				pantryDiags = append(pantryDiags, shell.DiagnosticItem{
					Code:         "PANTRY_NEAR_EXPIRY",
					Severity:     string(astercore.SeverityNeedsConfirmation),
					Path:         "pantry." + p.ID,
					Message:      fmt.Sprintf("文中提及试剂「%s」即将过期（%s）", pName, p.ExpiresOn),
					PantryItemID: p.ID,
				})
			}
		}
		sort.SliceStable(matchedReagents, func(i, j int) bool {
			rank := func(st string) int {
				switch st {
				case "expired":
					return 0
				case "exhausted":
					return 1
				case "near_expiry":
					return 2
				default:
					return 3
				}
			}
			ri, rj := rank(matchedReagents[i].Status), rank(matchedReagents[j].Status)
			if ri != rj {
				return ri < rj
			}
			return matchedReagents[i].Name < matchedReagents[j].Name
		})
		if len(matchedReagents) > 8 {
			matchedReagents = matchedReagents[:8]
		}
	}
	a.shell.CurrentMatchedReagents = matchedReagents

	rep := astercore.AssessDocument(doc, knownPantry)
	diags := make([]shell.DiagnosticItem, 0, len(pantryDiags)+len(rep.Diagnostics))
	diags = append(diags, pantryDiags...)
	for _, d := range rep.Diagnostics {
		diags = append(diags, shell.DiagnosticItem{
			Code:       d.Code,
			Severity:   string(d.Severity),
			Path:       d.Path,
			Message:    d.Message,
			NextAction: d.NextAction,
		})
	}
	a.shell.CurrentDiagnostics = diags

	var docSugs []shell.DocSuggestionItem
	if doc != nil {
		sRep := asterdoc.SuggestDocumentModules(doc)
		for _, sg := range sRep.Suggestions {
			if strings.TrimSpace(sg.YAML) == "" {
				continue
			}
			docSugs = append(docSugs, shell.DocSuggestionItem{
				Line:         sg.Line,
				ByteOffset:   sg.ByteOffset,
				Kind:         sg.Kind,
				KindLabel:    sg.KindLabel,
				Summary:      sg.Summary,
				OriginalLine: sg.OriginalLine,
				YAML:         sg.YAML,
			})
		}
	}
	a.shell.CurrentDocSuggestions = docSugs

	if a.vaultIndex != nil {
		idx := a.vaultIndex
		targetRel := relPath
		toShellRelated := func(hits []vault.RelatedDocHit) []shell.RelatedDocItem {
			out := make([]shell.RelatedDocItem, 0, len(hits))
			for _, h := range hits {
				out = append(out, shell.RelatedDocItem{
					RelPath: h.RelPath,
					Title:   h.Title,
					Date:    h.Date,
					Folder:  h.Folder,
					Score:   h.Score,
					Reason:  h.Reason,
				})
			}
			return out
		}
		if a.windowReady {
			go func() {
				defer diag.RecoverAndLog("related-docs")
				relHits := toShellRelated(idx.RelatedDocs(targetRel, 3))
				mygo.RunOnMain(func() {
					if a.shell != nil && a.shell.CurrentDocRelPath == targetRel {
						a.shell.CurrentRelatedDocs = relHits
						a.shell.Invalidate()
					}
				})
			}()
		} else {
			a.shell.CurrentRelatedDocs = toShellRelated(idx.RelatedDocs(targetRel, 3))
		}
	}
}

func applyDocSuggestionYAML(src string, byteOffset int, origLine string, fencedYAML string) (string, bool) {
	if strings.TrimSpace(fencedYAML) == "" {
		return src, false
	}
	insertPos := -1
	if byteOffset >= 0 && byteOffset < len(src) {
		lineEnd := len(src)
		if nl := strings.IndexByte(src[byteOffset:], '\n'); nl >= 0 {
			lineEnd = byteOffset + nl
		}
		seg := strings.TrimSpace(src[byteOffset:lineEnd])
		if origLine == "" || strings.Contains(seg, strings.TrimSpace(origLine)) || seg == strings.TrimSpace(origLine) {
			insertPos = lineEnd
		}
	}
	if insertPos < 0 && strings.TrimSpace(origLine) != "" {
		if idx := strings.Index(src, strings.TrimSpace(origLine)); idx >= 0 {
			lineEnd := len(src)
			if nl := strings.IndexByte(src[idx:], '\n'); nl >= 0 {
				lineEnd = idx + nl
			}
			insertPos = lineEnd
		}
	}
	if insertPos < 0 {
		return src, false
	}
	prefix := "\n\n"
	if insertPos < len(src) && src[insertPos] == '\n' {
		insertPos++
		prefix = "\n"
	}
	suffix := "\n\n"
	if insertPos < len(src) && strings.HasPrefix(src[insertPos:], "\n") {
		suffix = "\n"
	}
	return src[:insertPos] + prefix + strings.TrimSpace(fencedYAML) + suffix + src[insertPos:], true
}

func (a *app) restoreSnapshotFromHistory(relPath, snapshotID string) {
	if a.vault == nil || a.docStore == nil || relPath == "" || snapshotID == "" {
		return
	}
	snapSrc, err := a.vault.ReadSnapshot(relPath, snapshotID)
	if err != nil {
		a.syncErr = fmt.Sprintf("读取历史快照失败：%v", err)
		return
	}
	ld, err := a.docStore.Open(relPath)
	if err != nil {
		a.syncErr = fmt.Sprintf("打开文档失败：%v", err)
		return
	}
	_ = a.vault.RecordSnapshot(relPath, ld.Source, true)
	if err := ld.ApplyEdit(snapSrc); err != nil {
		a.syncErr = fmt.Sprintf("恢复快照失败：%v", err)
		return
	}
	ld.NotifySaved(a.vault, true)
	if a.vaultIndex != nil {
		a.vaultIndex.UpsertSource(relPath, ld.Source)
	}
	a.resetLiveDraftCachesForFile(relPath)
	a.lastCompanionKey = ""
	a.refreshVaultTreeAndCalendar()
	a.syncActiveDocCompanionState(relPath, ld.Source, ld.Parsed)
}

func (a *app) resetLiveDraftCachesForFile(relPath string) {
	if a.liveEditorSynced != nil {
		delete(a.liveEditorSynced, relPath)
	}
	if a.liveTitleDrafts != nil {
		delete(a.liveTitleDrafts, relPath)
	}
	if a.liveDateDrafts != nil {
		delete(a.liveDateDrafts, relPath)
	}
	for key, v := range a.liveViews {
		if strings.HasSuffix(key, "|"+relPath) {
			v.ClearTextDrafts()
		}
	}
}

func (a *app) undoActiveLiveDoc() {
	if a.shell == nil || a.docStore == nil {
		return
	}
	tab := a.shell.ActiveTab()
	if tab == nil || tab.Kind != shell.TabFile || tab.FilePath == "" {
		a.designUndo()
		return
	}
	if ld, err := a.docStore.Open(tab.FilePath); err == nil {
		if ld.Undo() {
			a.resetLiveDraftCachesForFile(tab.FilePath)
			a.refreshVaultTreeAndCalendar()
		}
	}
}

func (a *app) redoActiveLiveDoc() {
	if a.shell == nil || a.docStore == nil {
		return
	}
	tab := a.shell.ActiveTab()
	if tab == nil || tab.Kind != shell.TabFile || tab.FilePath == "" {
		a.designRedo()
		return
	}
	if ld, err := a.docStore.Open(tab.FilePath); err == nil {
		if ld.Redo() {
			a.resetLiveDraftCachesForFile(tab.FilePath)
			a.refreshVaultTreeAndCalendar()
		}
	}
}

func (a *app) revealPathInOS(relPath string) {
	if a.vault == nil {
		return
	}
	abs := a.vault.AbsPath(relPath)
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("explorer", "/select,", filepath.FromSlash(abs)).Start()
	case "darwin":
		_ = exec.Command("open", "-R", abs).Start()
	default:
		_ = exec.Command("xdg-open", filepath.Dir(abs)).Start()
	}
}

func (a *app) switchVaultRoot(newPath string) {
	if strings.TrimSpace(newPath) == "" {
		return
	}
	a.liveViews = nil
	a.liveEditors = nil
	a.liveEditorSynced = nil
	a.liveTitleDrafts = nil
	a.liveDateDrafts = nil
	a.lastCompanionKey = ""
	if err := a.initVaultWorkspace(newPath); err != nil {
		a.syncErr = fmt.Sprintf("切换实验库失败：%v", err)
		return
	}
	if a.vault != nil {
		saveLastVaultRoot(a.vault.Root)
		a.syncRecentVaultsToShell()
	}
	a.syncErr = ""
}

func (a *app) createVaultAt(targetPath string, seedDemo bool) error {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return fmt.Errorf("实验库路径不能为空")
	}
	clean := filepath.Clean(targetPath)
	if err := os.MkdirAll(clean, 0o755); err != nil {
		return fmt.Errorf("创建实验库目录失败：%w", err)
	}
	a.switchVaultRoot(clean)
	if a.syncErr != "" {
		return fmt.Errorf("%s", a.syncErr)
	}
	if seedDemo && a.vault != nil {
		a.openOrSeedUserGuideDoc()
	}
	return nil
}

func (a *app) openSandboxVault() {
	defDir := vault.DefaultVaultDir()
	if defDir == "" {
		return
	}
	a.switchVaultRoot(defDir)
}

func (a *app) removeRecentVault(vaultPath string) {
	clean := filepath.Clean(strings.TrimSpace(vaultPath))
	if clean == "" {
		return
	}
	prefs := loadVaultPrefs()
	var kept []string
	for _, p := range prefs.RecentVaults {
		if !strings.EqualFold(filepath.Clean(strings.TrimSpace(p)), clean) {
			kept = append(kept, p)
		}
	}
	prefs.RecentVaults = kept
	saveVaultPrefs(prefs)
	a.syncRecentVaultsToShell()
}

func (a *app) setFolderDefaultTemplate(folderRel, templateRel string) {
	if a.vault == nil {
		return
	}
	folderRel = strings.TrimSpace(folderRel)
	if folderRel == "" {
		return
	}
	if a.vault.Config.FolderTemplates == nil {
		a.vault.Config.FolderTemplates = map[string]string{}
	}
	templateRel = strings.TrimSpace(templateRel)
	if templateRel == "" {
		delete(a.vault.Config.FolderTemplates, folderRel)
	} else {
		a.vault.Config.FolderTemplates[folderRel] = templateRel
	}
	_ = a.vault.SaveConfig()
	if a.shell != nil {
		a.shell.FolderTemplates = a.vault.Config.FolderTemplates
	}
}

// computeVaultDiskSignatures hashes the directory structure + file mtimes/sizes of
// user-visible `.asterml`/assets files and Vault-local `.asterlab/pantry-*.json` files.
// Runs safely off the main UI goroutine so external Agent/editor writes appear automatically.
func computeVaultDiskSignatures(vaultRoot string) (vaultSig, pantrySig string) {
	if strings.TrimSpace(vaultRoot) == "" {
		return "", ""
	}
	var entries []string
	_ = filepath.WalkDir(vaultRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == vaultRoot {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == vault.SystemDirName || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			rel, rErr := filepath.Rel(vaultRoot, path)
			if rErr == nil {
				entries = append(entries, "D:"+filepath.ToSlash(rel))
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		rel, rErr := filepath.Rel(vaultRoot, path)
		if rErr != nil {
			return nil
		}
		slash := filepath.ToSlash(rel)
		lower := strings.ToLower(name)
		inAssets := strings.HasPrefix(slash, vault.AssetsDirName+"/")
		if !inAssets && !strings.HasSuffix(lower, ".asterml") {
			return nil
		}
		info, iErr := d.Info()
		if iErr != nil {
			return nil
		}
		entries = append(entries, fmt.Sprintf("F:%s:%d:%d", slash, info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	sort.Strings(entries)
	vh := sha256.New()
	for _, e := range entries {
		vh.Write([]byte(e))
		vh.Write([]byte{'\n'})
	}
	vaultSig = hex.EncodeToString(vh.Sum(nil))

	ph := sha256.New()
	for _, fname := range []string{pantryJSONLFileName, "pantry-ledger.json", "pantry-meta.json", "pantry-boxes.json"} {
		p := filepath.Join(vaultRoot, vault.SystemDirName, fname)
		if info, err := os.Stat(p); err == nil {
			fmt.Fprintf(ph, "%s:%d:%d\n", fname, info.Size(), info.ModTime().UnixNano())
		} else {
			fmt.Fprintf(ph, "%s:missing\n", fname)
		}
	}
	pantrySig = hex.EncodeToString(ph.Sum(nil))
	return vaultSig, pantrySig
}

// syncVaultDiskChangesNow compares current disk signatures against cached signatures
// and refreshes the left file tree, calendar, open LiveDocuments, and Vault Pantry on change.
// Must be called on the main UI thread (or directly in single-threaded tests).
func (a *app) syncVaultDiskChangesNow() bool {
	if a.vault == nil || a.vault.Root == "" {
		return false
	}
	nextVaultSig, nextPantrySig := computeVaultDiskSignatures(a.vault.Root)
	return a.applyVaultDiskSignatures(nextVaultSig, nextPantrySig)
}

func (a *app) applyVaultDiskSignatures(nextVaultSig, nextPantrySig string) bool {
	if a.vault == nil || a.vault.Root == "" {
		return false
	}
	vaultChanged := nextVaultSig != "" && nextVaultSig != a.lastVaultSig
	pantryChanged := nextPantrySig != "" && nextPantrySig != a.lastPantrySig
	if !vaultChanged && !pantryChanged {
		return false
	}
	a.lastVaultSig = nextVaultSig
	a.lastPantrySig = nextPantrySig

	if vaultChanged {
		if a.docStore != nil {
			for _, rel := range a.docStore.ReloadAllExternalChanged() {
				a.resetLiveDraftCachesForFile(rel)
			}
		}
		a.lastCompanionKey = ""
		a.refreshVaultTreeAndCalendar()
	}
	if pantryChanged {
		if a.tools != nil && !a.tools.PantryDialogOpen && !a.tools.PantryBoxDialogOpen {
			a.tools.ResetPantryState()
		}
		a.syncPantrySnapshotToShell()
	}
	return true
}

func (a *app) renderStandalonePantry(c *ui.Context) {
	if a.tools == nil {
		a.tools = a.newToolsWorkspace()
	}
	tools.ViewPantry(c, a.tools)
}
