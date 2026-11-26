package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/astercmd"
	"asterlab-mygo-prototype/internal/astercore"
	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/labio"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/vault"
)

type cachedAssayResult struct {
	modSig    string
	summaries []astercore.AssaySummary
}

func analysisModuleSignature(doc *asterdoc.Document) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	for _, m := range doc.Modules {
		switch m.Kind {
		case "group_table", "plate", "data_table", "step":
			b.WriteString(m.Kind)
			b.WriteByte(':')
			b.WriteString(m.ID)
			b.WriteByte(':')
			b.WriteString(m.Body)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (a *app) cachedDocumentAnalysis(relPath, _ string, doc *asterdoc.Document) []astercore.AssaySummary {
	if doc == nil {
		return nil
	}
	if a.cachedAssays == nil {
		a.cachedAssays = map[string]cachedAssayResult{}
	}
	sig := analysisModuleSignature(doc)
	if prev, ok := a.cachedAssays[relPath]; ok && prev.modSig == sig {
		return prev.summaries
	}
	res := astercore.AnalyzeDocument(doc, relPath)
	a.cachedAssays[relPath] = cachedAssayResult{
		modSig:    sig,
		summaries: res,
	}
	return res
}

func (a *app) ensureLiveViewsMap() {
	if a.liveViews == nil {
		a.liveViews = map[string]*asterdoc.View{}
	}
	if a.liveEditors == nil {
		a.liveEditors = map[string]*editor.State{}
	}
}

func (a *app) buildPantryLocationMap() map[string]string {
	out := map[string]string{}
	if a.tools == nil || len(a.tools.Pantry.Items) == 0 {
		return out
	}
	for _, it := range a.tools.Pantry.Items {
		loc := a.tools.FormatPantryDisplayLocation(it)
		if loc == "" {
			loc = it.Location
		}
		if loc == "" {
			continue
		}
		if it.LabelCode != "" {
			out[it.LabelCode] = loc
		}
		if it.ID != "" {
			out[it.ID] = loc
		}
		if it.Name != "" {
			out[it.Name] = loc
		}
	}
	return out
}

func (a *app) buildPantryMentionItems() []asterdoc.PantryMentionItem {
	if a.tools == nil || len(a.tools.Pantry.Items) == 0 {
		return nil
	}
	out := make([]asterdoc.PantryMentionItem, 0, len(a.tools.Pantry.Items))
	for _, it := range a.tools.Pantry.Items {
		loc := a.tools.FormatPantryDisplayLocation(it)
		if loc == "" {
			loc = it.Location
		}
		meta := a.tools.MetaForPantryItem(it)
		out = append(out, asterdoc.PantryMentionItem{
			ID:        it.ID,
			Name:      it.Name,
			LabelCode: it.LabelCode,
			Location:  loc,
			ItemType:  meta.ItemType,
			Status:    it.Status,
		})
	}
	return out
}

func (a *app) ensureLiveSourceEditor(relPath, src string) *editor.State {
	a.ensureLiveViewsMap()
	normSrc := strings.ReplaceAll(src, "\r\n", "\n")
	ed := a.liveEditors[relPath]
	if ed == nil {
		buf := editor.NewBuffer(normSrc)
		if lang := editor.LanguageFor("design.md"); lang != nil {
			buf.SetLanguage(lang)
		}
		ed = &editor.State{
			Buffer:   buf,
			DiffBase: editor.NewBuffer(normSrc),
		}
		a.liveEditors[relPath] = ed
		a.liveEditorSynced[relPath] = normSrc
		return ed
	}
	// If doc changed externally or via visual pane, keep buffer synchronized when it wasn't just edited
	if prev, ok := a.liveEditorSynced[relPath]; !ok || (prev != normSrc && strings.ReplaceAll(ed.Buffer.String(), "\r\n", "\n") == prev) {
		ed.Buffer = editor.NewBuffer(normSrc)
		if lang := editor.LanguageFor("design.md"); lang != nil {
			ed.Buffer.SetLanguage(lang)
		}
		ed.DiffBase = editor.NewBuffer(normSrc)
		a.liveEditorSynced[relPath] = normSrc
	}
	return ed
}

func (a *app) renderVaultTabContent(c *ui.Context, pane *shell.PaneNode, tab shell.WorkspaceTab) {
	tok := theme.GetTokens(c.Theme())
	if a.vault == nil || a.docStore == nil {
		ui.Column(c).Fill().Center().Padding(float32(tok.Spacing24)).Children(func() {
			ui.Text(c, "本地实验库尚未就绪").FontSize(float32(tok.TextSection)).TextColor(tok.TextMuted)
		})
		return
	}
	if strings.TrimSpace(tab.FilePath) == "" {
		ui.Column(c).Fill().Center().Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing8)).Children(func() {
			ui.Text(c, "请选择或新建实验文件").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
			ui.Text(c, "在左侧文件树或日历中选择实验，也可以拖拽左侧文件到此处分屏对照。").
				FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
		})
		return
	}

	ld, err := a.docStore.Open(tab.FilePath)
	if err != nil {
		ui.Column(c).Fill().Center().Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing12)).Children(func() {
			ui.Text(c, "无法读取实验文件："+tab.FilePath).FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Danger)
			ui.Text(c, err.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
			if components.Button(c, "关闭此标签页", components.ButtonOutline).Clicked() && a.shell != nil {
				a.shell.CloseTabInPane(pane.ID, tab.ID)
			}
		})
		return
	}
	if changed, _ := ld.ReloadIfExternalChanged(); changed {
		a.refreshVaultTreeAndCalendar()
	}

	fm, _, _ := asterdoc.ScanFrontmatter(ld.Source)
	docTitle := strings.TrimSpace(fm.Title)
	if docTitle == "" {
		docTitle = strings.TrimSuffix(filepath.Base(tab.FilePath), ".asterml")
	}
	docDate := asterdoc.NormalizeDateYMD(fm.ScheduledDate)
	docStatus := firstNonEmpty(strings.TrimSpace(fm.Stage), "planned")
	if docStatus != "planned" && docStatus != "in_progress" && docStatus != "completed" {
		docStatus = "planned"
	}
	folderRel := filepath.ToSlash(filepath.Dir(tab.FilePath))
	if folderRel == "." {
		folderRel = ""
	}
	folderLabel := folderRel
	if folderLabel == "" {
		folderLabel = "根目录"
	}

	setTabMode := func(m shell.ViewMode) {
		if a.shell == nil {
			return
		}
		for i := range pane.Tabs {
			if pane.Tabs[i].ID == tab.ID {
				pane.Tabs[i].Mode = m
				break
			}
		}
		a.saveVaultWorkspace()
	}

	applyLiveSource := func(next string) {
		if next == ld.Source {
			return
		}
		a.stageEmergencyDraft(tab.FilePath, next, 0)
		_ = ld.ApplyEdit(next)
		ld.NotifySaved(a.vault, false)
		_, _, _ = a.snapshotDocFile(tab.FilePath, "edit", false)
		if a.vaultIndex != nil {
			a.vaultIndex.UpsertSource(tab.FilePath, ld.Source)
		}
		if a.liveEditorSynced != nil {
			a.liveEditorSynced[tab.FilePath] = strings.ReplaceAll(ld.Source, "\r\n", "\n")
		}
		a.lastCompanionKey = ""
		a.refreshVaultTreeAndCalendar()
		a.syncActiveDocCompanionState(tab.FilePath, ld.Source, ld.Parsed)
	}

	mobileLayout := a.shell != nil && a.shell.IsMobileLayout(c)

	if a.shell != nil {
		a.shell.OnMobileDocToolbarAction = func(curTab shell.WorkspaceTab, action string) {
			a.handleMobileDocToolbarAction(c, curTab, action)
		}
	}

	pageCol := ui.Column(c.Key("vault-page:"+pane.ID+":"+tab.ID+":"+tab.FilePath)).Fill()
	theme.PaintCanvasBackground(pageCol, tok)
	findOpen := ui.Local(c.Root(), "doc-find-open:"+pane.ID+":"+tab.ID, func() bool { return false })
	findQuery := ui.Local(c.Root(), "doc-find-q:"+pane.ID+":"+tab.ID, func() string { return "" })
	if a.pendingFindQuery != nil {
		if pq := a.pendingFindQuery[tab.FilePath]; pq != "" {
			*findQuery = pq
			*findOpen = true
			delete(a.pendingFindQuery, tab.FilePath)
		}
	}
	if pane.ID == a.shell.ActivePaneID && c.Shortcut(ui.Cmd, ui.KeyF) {
		*findOpen = !*findOpen
	}
	if a.shell != nil && pane.ID == a.shell.ActivePaneID {
		a.syncActiveDocCompanionState(tab.FilePath, ld.Source, ld.Parsed)
		if ld.ParseErr != nil {
			a.shell.StatusSaveLabel = "待语法闭合"
			a.shell.StatusWarnLabel = "AsterML 语法待修复"
		} else {
			a.shell.StatusSaveLabel = "实时已保存"
			a.shell.StatusWarnLabel = ""
		}
		if ld.Parsed != nil {
			stepTree := asterdoc.CachedParseStepTree(ld.Source, ld.Parsed, -1)
			a.shell.StatusMetaLabel = fmt.Sprintf("%d 步骤 · %d 模块", len(stepTree.Flat), len(ld.Parsed.Modules))
		}
	}

	pageCol.Children(func() {
		// 1. Clean Obsidian 1-row Document Header on Desktop/Tablet (Breadcrumbs + single Read/Edit toggle + Find + More)
		if !mobileLayout {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing16)).
				Background(tok.Surface.Alpha(0.86)).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				chosenCrumb := -1
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					bc := components.Breadcrumbs(c, []string{folderLabel, docTitle}, &chosenCrumb)
					if bc.Changed() && chosenCrumb == 0 && a.shell != nil {
						a.shell.SelectedTreePath = folderRel
					}
				})

				if ld.ParseErr != nil {
					components.Badge(c, "待语法闭合", components.BadgeOptions{Tone: components.BadgeWarning}).AlignSelf(ui.Center)
				}

				if tab.Mode == shell.ModeSource {
					components.Badge(c, "高级源码视图", components.BadgeOptions{Tone: components.BadgeWarning}).AlignSelf(ui.Center)
					if components.Button(c, "返回编辑", components.ButtonSecondary).Clicked() {
						setTabMode(shell.ModeEdit)
						tab.Mode = shell.ModeEdit
					}
				} else {
					// Calm compact 2-segment mode switch: [ 编辑 | 实验 ]
					ui.Row(c.Key("doc-mode-seg:"+tab.ID)).AlignSelf(ui.Center).
						Padding(float32(tok.Spacing4)*0.5).Radius(float32(tok.ControlRadius)).
						Background(tok.Elevated).Border(1, tok.Border).Gap(2).Children(func() {
						edAct := tab.Mode == shell.ModeEdit
						edSeg := ui.ButtonBase(c.Key("doc-mode-ed:"+tab.ID)).
							Height(float32(tok.Spacing24)-2).Padding(0, float32(tok.Spacing8)).
							AlignItems(ui.Center).Radius(float32(tok.ControlRadius)).Label("编辑设计模式 (Ctrl+E)")
						theme.ApplyTactilePress(edSeg)
						edBg := ui.Transparent
						if edAct {
							edBg = tok.Surface
						}
						edSeg.Background(edSeg.AnimateColor("edseg-bg", edBg, theme.MotionInstant))
						edSeg.Children(func() {
							col := tok.TextMuted
							if edAct {
								col = tok.Text
							}
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Icon(c, lucide.Pencil()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(col)
								ui.Text(c, "编辑").FontSize(float32(tok.TextMeta)).Bold().TextColor(col).SingleLine()
							})
						})
						if edSeg.Clicked() && !edAct {
							setTabMode(shell.ModeEdit)
							tab.Mode = shell.ModeEdit
						}

						labAct := tab.Mode == shell.ModeLab
						labSeg := ui.ButtonBase(c.Key("doc-mode-lab:"+tab.ID)).
							Height(float32(tok.Spacing24)-2).Padding(0, float32(tok.Spacing8)).
							AlignItems(ui.Center).Radius(float32(tok.ControlRadius)).Label("实验执行模式 · 防误触 (Ctrl+E)")
						theme.ApplyTactilePress(labSeg)
						labBg := ui.Transparent
						if labAct {
							labBg = tok.AccentSoftBg
						}
						labSeg.Background(labSeg.AnimateColor("labseg-bg", labBg, theme.MotionInstant))
						labSeg.Children(func() {
							col := tok.TextMuted
							if labAct {
								col = tok.Accent
							}
							ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
								ui.Icon(c, lucide.FlaskConical()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(col)
								ui.Text(c, "实验").FontSize(float32(tok.TextMeta)).Bold().TextColor(col).SingleLine()
							})
						})
						if labSeg.Clicked() && !labAct {
							_, _, _ = a.snapshotDocFile(tab.FilePath, "freeze", true)
							setTabMode(shell.ModeLab)
							tab.Mode = shell.ModeLab
						}
					})
				}

				findBtn := components.IconButton(c, lucide.Search(), "页内查找 (Ctrl+F)", components.IconButtonPlain)
				if *findOpen {
					findBtn.Background(tok.AccentSoftBg)
				}
				if findBtn.Clicked() {
					*findOpen = !*findOpen
				}

				moreBtn := components.IconButton(c, lucide.Ellipsis(), "更多文档操作", components.IconButtonPlain)
				components.MenuButton(c, moreBtn, func(m *components.Menu) {
					if m.Item("导出 PDF 执行单…").Chosen() && ld.Parsed != nil {
						a.startDocumentPDFExport(c, docTitle, "", "", a.vault.Root, ld.Parsed)
					}
					if m.Item("导出 Word 报告 (.docx)…").Chosen() && ld.Parsed != nil {
						_ = a.exportDocumentDocx(c, tab.FilePath, ld.Parsed)
					}
					if m.Item("导出 Tidy CSV (Prism / R)…").Chosen() && ld.Parsed != nil {
						_ = a.exportDocumentTidyCSV(c, tab.FilePath, ld.Parsed)
					}
					if m.Item("导入酶标仪矩阵 (CSV / XLSX)…").Chosen() {
						_ = a.importPlateDataIntoDoc(c, tab.FilePath)
					}
					if m.Item("分享实验文档…").Chosen() && ld.Parsed != nil {
						_ = a.shareDocument(c, tab.FilePath, ld.Parsed)
					}
					m.Separator()
					if m.Item("另存为模板").Chosen() {
						if tpl, err := a.vault.SaveAsTemplate(tab.FilePath, docTitle); err == nil {
							a.refreshVaultTreeAndCalendar()
							c.AddToast(ui.Toast{Title: "已保存为模板：" + tpl.Name, Type: components.ToastSuccess})
						}
					}
					if m.Item("创建副本").Chosen() {
						if dup, err := a.vault.DuplicateFile(tab.FilePath, docDate); err == nil {
							a.refreshVaultTreeAndCalendar()
							a.openVaultFileInActivePane(dup.RelPath, dup.Title)
						}
					}
					m.Separator()
					if m.Item("复制纯文本原文").Chosen() {
						c.Services().WriteClipboard(ld.Source)
						c.AddToast(ui.Toast{Title: "已复制文档原文", Type: components.ToastSuccess})
					}
					srcLbl := "查看源码（高级）"
					if tab.Mode == shell.ModeSource {
						srcLbl = "返回可视化编辑"
					}
					if m.Item(srcLbl).Chosen() {
						if tab.Mode == shell.ModeSource {
							setTabMode(shell.ModeEdit)
							tab.Mode = shell.ModeEdit
						} else {
							setTabMode(shell.ModeSource)
							tab.Mode = shell.ModeSource
						}
					}
					m.Separator()
					if m.Item("在文件管理器中显示").Chosen() {
						a.revealPathInOS(tab.FilePath)
					}
					if m.Item("移入回收站").Chosen() {
						a.deleteVaultFile(tab.FilePath)
					}
				})
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.Border)
		}

		// In-page Find Bar (Ctrl+F)
		if *findOpen && tab.Mode != shell.ModeSource {
			ui.Row(c.Key("doc-find-bar")).FillWidth().Padding(float32(tok.Spacing4), float32(tok.Spacing16)).
				Background(tok.Elevated).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				ui.Icon(c, lucide.Search()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.Accent)
				ui.Box(c).Grow(1).MinWidth(0).Children(func() {
					tb := components.TextBox(c, findQuery, components.TextBoxOptions{}).Placeholder("在当前实验步骤与模块中查找…").FillWidth()
					tb.AutoFocus()
					if tb.Shortcut(0, ui.KeyEscape) {
						*findOpen = false
						*findQuery = ""
					}
				})
				if strings.TrimSpace(*findQuery) != "" {
					qLower := strings.ToLower(strings.TrimSpace(*findQuery))
					hits := strings.Count(strings.ToLower(ld.Source), qLower)
					components.Badge(c, fmt.Sprintf("%d 处匹配", hits))
				}
				ui.Spacer(c)
				if components.IconButton(c, lucide.X(), "关闭查找", components.IconButtonCompact).Clicked() {
					*findOpen = false
					*findQuery = ""
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.Border)
		}

		// 2. Source Mode
		if tab.Mode == shell.ModeSource {
			if a.liveEditorSynced == nil {
				a.liveEditorSynced = map[string]string{}
			}
			srcEd := a.ensureLiveSourceEditor(tab.FilePath, ld.Source)
			bufText := strings.ReplaceAll(srcEd.Buffer.String(), "\r\n", "\n")
			curSource := strings.ReplaceAll(ld.Source, "\r\n", "\n")
			if bufText != curSource {
				a.liveEditorSynced[tab.FilePath] = bufText
				a.stageEmergencyDraft(tab.FilePath, bufText, srcEd.Cursor())
				ld.ApplySourceDraft(bufText)
				if ld.ParseErr == nil {
					_, _, _ = a.snapshotDocFile(tab.FilePath, "source-edit", false)
					a.refreshVaultTreeAndCalendar()
				}
			}
			ui.Column(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
				if ld.ParseErr != nil {
					ui.Row(c).FillWidth().Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
						Background(tok.DangerBg).Children(func() {
						ui.Text(c, ld.ParseErr.Error()).FontSize(float32(tok.TextMeta)).TextColor(tok.DangerText)
					})
				}
				ui.Box(c).FillWidth().Grow(1).MinHeight(0).Background(tok.CodeBg).Children(func() {
					editor.View(c, srcEd).Fill()
				})
			})
			return
		}

		// 3. Parse failure fallback in Edit/Read
		if ld.ParseErr != nil || ld.Parsed == nil {
			ui.Column(c).FillWidth().Grow(1).Padding(float32(tok.Spacing24)).Gap(float32(tok.Spacing12)).Children(func() {
				errTxt := "文档语法待修复"
				if ld.ParseErr != nil {
					errTxt = ld.ParseErr.Error()
				}
				ui.Row(c).FillWidth().Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
					Background(tok.DangerBg).Border(1, tok.Danger).Children(func() {
					ui.Text(c, errTxt).FontSize(float32(tok.TextBody)).TextColor(tok.DangerText)
				})
				if components.Button(c, "切换到源码模式修复", components.ButtonSecondary).Clicked() {
					setTabMode(shell.ModeSource)
				}
			})
			return
		}

		// 4. Unified Edit / Read Document Canvas (Single Source of Truth)
		a.ensureLiveViewsMap()
		doc := ld.Parsed
		viewKey := pane.ID + "|" + tab.ID + "|" + tab.FilePath
		v := a.liveViews[viewKey]
		if v == nil {
			v = &asterdoc.View{
				Registry:    asterdoc.BuiltinRegistry(),
				UnifiedLive: true,
				Allowed:     map[string]bool{asterdoc.PermDocumentEdit: true},
				AsyncDecode: a.windowReady,
				OnAsyncInvalidate: func() {
					if a.shell != nil {
						a.shell.Invalidate()
					}
				},
			}
			a.liveViews[viewKey] = v
		}
		if v.Allowed == nil {
			v.Allowed = map[string]bool{asterdoc.PermDocumentEdit: true}
		}
		v.Doc = doc
		v.DocumentTitle = docTitle
		v.SearchQuery = *findQuery
		v.BaseDir = a.vault.Root
		v.MobileCompact = mobileLayout
		v.AltHoverPreviewQuery = a.altHoverPreviewQuery
		if a.shell != nil {
			v.DefaultPlateFormat = a.shell.DefaultPlateFormat
			v.DefaultExcessPct = a.shell.DefaultExcessPct
		}
		if a.scrollToStepID != "" && pane.ID == a.shell.ActivePaneID {
			v.ScrollToStepID = a.scrollToStepID
			a.scrollToStepID = ""
		}
		v.PantryLocationMap = a.buildPantryLocationMap()
		v.PantryCompletionItems = a.buildPantryMentionItems()
		v.OnOpenVaultLink = func(relPath string) {
			resolved := strings.TrimSpace(relPath)
			if a.shell != nil && resolved != "" {
				for _, f := range a.shell.VaultFiles {
					if f.RelPath == resolved {
						resolved = f.RelPath
						break
					}
					noExt := strings.TrimSuffix(f.RelPath, filepath.Ext(f.RelPath))
					base := strings.TrimSuffix(filepath.Base(f.RelPath), filepath.Ext(f.RelPath))
					if noExt == resolved || base == resolved || f.Title == resolved {
						resolved = f.RelPath
						break
					}
				}
			}
			a.openVaultFileInActivePane(resolved, "")
		}
		if tab.Mode == shell.ModeLab {
			v.Mode = asterdoc.ModeProtocol
			v.LabMode = true
			v.ReadOnly = false // Reserved execution slots & blank note areas stay interactive in Lab Mode!
		} else {
			v.Mode = asterdoc.ModeDesign
			v.LabMode = false
			v.ReadOnly = false
		}
		// Wire unified Semantic Command Bus (astercmd) for Slash menus, `+` buttons, and well photo linking
		cmdMode := astercmd.ModeEdit
		if v.LabMode {
			cmdMode = astercmd.ModeLab
		}
		regEntries := astercmd.DefaultRegistry().SlashEntries("top_level", cmdMode)
		slashItems := make([]asterdoc.SlashCommandItem, 0, len(regEntries))
		for _, re := range regEntries {
			slashItems = append(slashItems, asterdoc.SlashCommandItem{
				CommandID:   re.CommandID,
				Label:       re.Label,
				Detail:      re.Detail,
				Slash:       re.Slash,
				Kind:        re.Kind,
				Preset:      re.Preset,
				SearchTerms: re.SearchTerms,
			})
		}
		v.SlashItems = slashItems
		if a.shell != nil {
			var targets []string
			for _, f := range a.shell.VaultFiles {
				targets = append(targets, f.RelPath)
			}
			v.VaultCompletionTargets = targets
		}
		runSlashCmd := func(item asterdoc.SlashCommandItem, stepID string, insertOffset int) {
			if item.CommandID == "asset.embed" {
				paths, pErr := mygo.Dialog.Open(mygo.OpenDialogOptions{
					Parent: mygo.FocusedWindow(),
					Title:  "选择要嵌入的图片或实验附件",
				})
				if pErr != nil || len(paths) == 0 {
					return
				}
				refPath := paths[0]
				if a.shell == nil || a.shell.AutoArchiveAssets {
					if rel, impErr := a.vault.ImportAssetFile(paths[0]); impErr == nil {
						refPath = rel
					}
				}
				res := astercmd.ExecuteBatch(ld.Source, astercmd.BatchRequest{
					Mode: cmdMode,
					Operations: []astercmd.Operation{{
						CommandID: "asset.embed",
						Args:      map[string]any{"asset": refPath, "at_offset": insertOffset},
					}},
				})
				if res.OK {
					applyLiveSource(res.NextSource)
				}
				return
			}
			args := map[string]any{}
			if stepID != "" {
				args["step_id"] = stepID
			} else if insertOffset >= 0 {
				args["at_offset"] = insertOffset
			}
			if item.Kind != "" && item.CommandID == "block.insert" {
				args["kind"] = item.Kind
			}
			if item.Preset != "" && item.CommandID == "step.create" {
				args["preset"] = item.Preset
			}
			if item.CommandID == "doc.append_note" {
				args["text"] = "现场记录："
			}
			res := astercmd.ExecuteBatch(ld.Source, astercmd.BatchRequest{
				Mode:        cmdMode,
				PlateFormat: v.DefaultPlateFormat,
				ExcessPct:   v.DefaultExcessPct,
				Operations:  []astercmd.Operation{{CommandID: item.CommandID, Args: args}},
			})
			if res.OK {
				applyLiveSource(res.NextSource)
				if stepID == "" {
					v.RequestFocusTailProse()
				}
			} else if res.Error != nil {
				c.AddToast(ui.Toast{Title: res.Error.Message, Type: components.ToastWarning})
			}
		}
		v.OnExecuteSlash = runSlashCmd
		v.OnAttachPhotoToWell = func(plateID string, wells []string) {
			paths, pErr := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: mygo.FocusedWindow(),
				Title:  fmt.Sprintf("为 %s (%s) 选择实验照片", plateID, strings.Join(wells, ",")),
			})
			if pErr != nil || len(paths) == 0 {
				return
			}
			refPath := paths[0]
			if a.shell == nil || a.shell.AutoArchiveAssets {
				if rel, impErr := a.vault.ImportAssetFile(paths[0]); impErr == nil {
					refPath = rel
				}
			}
			targetSpec := fmt.Sprintf("@%s:%s", plateID, strings.Join(wells, ","))
			res := astercmd.ExecuteBatch(ld.Source, astercmd.BatchRequest{
				Mode: cmdMode,
				Operations: []astercmd.Operation{{
					CommandID: "asset.embed",
					Args: map[string]any{
						"asset":   refPath,
						"targets": []string{targetSpec},
						"role":    "obs",
					},
				}},
			})
			if res.OK {
				applyLiveSource(res.NextSource)
				c.AddToast(ui.Toast{Title: fmt.Sprintf("已关联照片到 %s", targetSpec), Type: components.ToastSuccess})
			}
		}
		v.OnChange = func(nextSrc string) {
			applyLiveSource(nextSrc)
		}
		v.OnInsertStepBlock = func(stepID string, afterIdx int, block map[string]any) {
			if next, err := asterdoc.InsertStepBlockInSource(ld.Source, stepID, afterIdx, block); err == nil {
				applyLiveSource(next)
			}
		}
		v.OnUpdateStepBlock = func(stepID, blockID string, props map[string]any) {
			if next, err := asterdoc.UpdateStepBlockInSource(ld.Source, stepID, blockID, props); err == nil {
				applyLiveSource(next)
			}
		}
		v.OnRemoveStepBlock = func(stepID, blockID string) {
			if next, err := asterdoc.RemoveStepBlockInSource(ld.Source, stepID, blockID); err == nil {
				applyLiveSource(next)
			}
		}
		v.OnDuplicateBlock = func(stepID, blockID string) {
			if next, err := asterdoc.DuplicateStepBlockInSource(ld.Source, stepID, blockID); err == nil {
				applyLiveSource(next)
			}
		}
		v.OnMoveStepBlock = func(stepID, blockID, dir string) {
			if next, err := asterdoc.MoveStepBlockInSource(ld.Source, stepID, blockID, dir); err == nil {
				applyLiveSource(next)
			}
		}
		v.OnMoveStepBlockIndex = func(stepID, blockID string, toIndex int) {
			if next, err := asterdoc.MoveStepBlockToIndex(ld.Source, stepID, blockID, toIndex); err == nil {
				applyLiveSource(next)
			}
		}
		v.OnImportAssetBytes = func(data []byte, origName string) (string, error) {
			return a.vault.ImportAssetBytes(data, origName)
		}
		attachFilesToBlock := func(stepID, blockID, wellID string, paths []string) {
			if len(paths) == 0 {
				return
			}
			var existing []any
			if ld.Parsed != nil {
				for _, m := range ld.Parsed.Modules {
					if m.ID == blockID {
						if arr, ok := m.Data["items"].([]any); ok {
							existing = append(existing, arr...)
						}
					}
					if m.Kind == "step" && m.ID == stepID {
						if blks, ok := m.Data["blocks"].([]any); ok {
							for _, raw := range blks {
								if bm, ok := raw.(map[string]any); ok && fmt.Sprint(bm["id"]) == blockID {
									if arr, ok := bm["items"].([]any); ok {
										existing = append(existing, arr...)
									}
								}
							}
						}
					}
				}
			}
			wellCount := 0
			for _, raw := range existing {
				if wm, ok := raw.(map[string]any); ok && wellID != "" && strings.EqualFold(fmt.Sprint(wm["well_id"]), wellID) {
					wellCount++
				}
			}
			for _, picked := range paths {
				if strings.TrimSpace(picked) == "" {
					continue
				}
				refPath := picked
				if a.shell == nil || a.shell.AutoArchiveAssets {
					if rel, impErr := a.vault.ImportAssetFile(picked); impErr == nil {
						refPath = rel
					}
				}
				entry := map[string]any{
					"id":    fmt.Sprintf("att-%d", len(existing)+1),
					"label": filepath.Base(picked),
					"src":   refPath,
				}
				if wellID != "" {
					wellCount++
					entry["well_id"] = strings.ToUpper(strings.TrimSpace(wellID))
					entry["fov_index"] = wellCount
				}
				existing = append(existing, entry)
			}
			if stepID != "" {
				if next, uErr := asterdoc.UpdateStepBlockInSource(ld.Source, stepID, blockID, map[string]any{"items": existing}); uErr == nil {
					applyLiveSource(next)
				}
			} else if ld.Parsed != nil {
				if next, uErr := ld.Parsed.UpdateModule(blockID, map[string]any{"items": existing}); uErr == nil {
					applyLiveSource(next)
				}
			}
		}
		v.OnPickAttachmentFile = func(stepID, blockID string) {
			paths, pErr := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: mygo.FocusedWindow(),
				Title:  "选择实验照片或原始附件",
			})
			if pErr == nil && len(paths) > 0 {
				attachFilesToBlock(stepID, blockID, "", paths)
			}
		}
		v.OnPickAttachmentForWell = func(stepID, blockID, wellID string) {
			title := "选择实验照片或原始附件"
			if wellID != "" {
				title = fmt.Sprintf("为 %s 选择实验照片或附件", wellID)
			}
			paths, pErr := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: mygo.FocusedWindow(),
				Title:  title,
			})
			if pErr == nil && len(paths) > 0 {
				attachFilesToBlock(stepID, blockID, wellID, paths)
			}
		}
		v.OnAttachFilesForWell = func(stepID, blockID, wellID string, paths []string) {
			attachFilesToBlock(stepID, blockID, wellID, paths)
		}
		v.OnPickFigureFile = func(stepID, blockID string) {
			paths, pErr := mygo.Dialog.Open(mygo.OpenDialogOptions{
				Parent: mygo.FocusedWindow(),
				Title:  "选择示意图或照片（PNG / JPG / SVG）",
			})
			if pErr != nil || len(paths) == 0 {
				return
			}
			picked := paths[0]
			refPath := picked
			if a.shell == nil || a.shell.AutoArchiveAssets {
				if rel, impErr := a.vault.ImportAssetFile(picked); impErr == nil {
					refPath = rel
				}
			}
			props := map[string]any{"src": refPath}
			if strings.EqualFold(filepath.Ext(picked), ".svg") {
				if raw, rErr := os.ReadFile(picked); rErr == nil && strings.Contains(string(raw), "<svg") {
					props["svg"] = string(raw)
				}
			}
			if stepID == "" && ld.Parsed != nil {
				if next, uErr := ld.Parsed.UpdateModule(blockID, props); uErr == nil {
					applyLiveSource(next)
				}
			} else if next, uErr := asterdoc.UpdateStepBlockInSource(ld.Source, stepID, blockID, props); uErr == nil {
				applyLiveSource(next)
			}
		}
		v.OnCopyImage = func(pngBytes []byte) error {
			if len(pngBytes) == 0 {
				return nil
			}
			return mygo.Clipboard.WriteImage(pngBytes)
		}
		v.OnSavePlateExport = func(defaultName, format string, data []byte) error {
			if len(data) == 0 {
				return nil
			}
			title := "导出组件文件"
			switch format {
			case "csv":
				title = "导出孔板 CSV 布局"
			case "png":
				title = "导出组件 PNG 图像"
			case "svg":
				title = "导出组件 SVG 矢量图"
			case "md":
				title = "导出组件 Markdown"
			}
			path, sErr := mygo.Dialog.Save(mygo.SaveDialogOptions{
				Parent:      mygo.FocusedWindow(),
				Title:       title,
				DefaultPath: defaultName,
			})
			if sErr == nil && path != "" {
				if wErr := os.WriteFile(path, data, 0o644); wErr != nil {
					return wErr
				}
				c.AddToast(ui.Toast{Title: "已导出：" + filepath.Base(path), Type: components.ToastSuccess})
			}
			return nil
		}
		v.OnLocatePantryCode = func(code string) {
			if a.tools == nil {
				a.tools = a.newToolsWorkspace()
			}
			a.tools.LocatePantryItem(code)
			if a.shell != nil {
				a.shell.OpenPantryTab()
			}
		}
		v.OnTimerAction = func(stepID, blockID, action string) {
			var curProps map[string]any
			if ld.Parsed != nil {
				for _, m := range ld.Parsed.Modules {
					if m.Kind == "step" && m.ID == stepID {
						if blks, ok := m.Data["blocks"].([]any); ok {
							for _, raw := range blks {
								if bm, ok := raw.(map[string]any); ok && fmt.Sprint(bm["id"]) == blockID {
									curProps = bm
									break
								}
							}
						}
					}
				}
			}
			nowRFC := time.Now().UTC().Format(time.RFC3339)
			patch := map[string]any{}
			switch action {
			case "start":
				patch["status"] = "running"
				patch["started_at"] = nowRFC
			case "pause":
				elapsed := 0
				if curProps != nil {
					if n, ok := curProps["elapsed_sec"].(int); ok {
						elapsed = n
					}
					if sa, _ := curProps["started_at"].(string); sa != "" {
						if started, err := time.Parse(time.RFC3339, sa); err == nil {
							elapsed += int(time.Since(started).Seconds())
						}
					}
				}
				patch["status"] = "paused"
				patch["elapsed_sec"] = elapsed
			case "complete":
				patch["status"] = "completed"
				patch["completed_at"] = nowRFC
			}
			if len(patch) > 0 {
				if next, err := asterdoc.UpdateStepBlockInSource(ld.Source, stepID, blockID, patch); err == nil {
					applyLiveSource(next)
				}
			}
		}

		// Sync unified Document Outline (plain chapters + orthogonal nested steps)
		if pane.ID == a.shell.ActivePaneID {
			docOutline := asterdoc.CachedParseDocumentOutline(ld.Source, doc)
			var outline []shell.OutlineItem
			for _, on := range docOutline {
				meta := ""
				if on.IsStep {
					if on.RunningTimers > 0 {
						meta = "计时中"
					} else if v.LabMode && on.ChecksTotal > 0 {
						if on.ChecksDone >= on.ChecksTotal {
							meta = "✓"
						} else {
							meta = fmt.Sprintf("%d/%d", on.ChecksDone, on.ChecksTotal)
						}
					}
				}
				outline = append(outline, shell.OutlineItem{
					ID:        on.ID,
					Title:     on.Title,
					IsStep:    on.IsStep,
					Number:    on.Number,
					Depth:     on.Depth,
					StepDepth: on.StepDepth,
					Meta:      meta,
					Active:    on.ID == a.designSelected,
				})
			}
			a.shell.OutlineSteps = outline
		}

		// Docked centered pure-icon toolbar rail on desktop/tablet so scrolled STEP headings never clip under it
		if !mobileLayout {
			ui.Row(c.Key("doc-ftb-rail")).FillWidth().
				Padding(float32(tok.Spacing8), float32(tok.Spacing16)).
				Justify(ui.Center).Background(tok.Canvas).Children(func() {
				v.RenderFloatingToolbar(c, tab.Mode == shell.ModeLab)
			})
			ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
		}

		ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			docScroll := ui.Scroll(c).Fill()
			if mobileLayout && a.shell != nil {
				docScroll.TrackScroll(a.shell.MobileContentScrollState()).
					ScrollbarInsets(a.shell.MobileTopReserve(c), 0, a.shell.MobileBottomReserve(c), 0)
				a.shell.HandleMobilePullRefresh(c, docScroll)
			}
			padTop := float32(tok.Spacing16)
			if !mobileLayout {
				padTop = float32(tok.Spacing24)
			}
			padH := float32(tok.Spacing24)
			padBot := float32(tok.Spacing48)
			if mobileLayout && a.shell != nil {
				padTop = a.shell.MobileTopReserve(c) + float32(tok.Spacing4)
				padH = float32(tok.Spacing12) + 2
				padBot = a.shell.MobileBottomReserve(c) + float32(tok.Spacing8)
			}
			docScroll.Children(func() {
				// Readable line length adapts smoothly to viewport width (widens gently on 27"+ monitors)
				winW, _ := c.Size()
				paneW := docScroll.Bounds().W
				if paneW <= 0 {
					paneW = winW - float32(tok.Spacing48)*5
				}
				maxColW := float32(780)
				wideColW := float32(960)
				if winW >= 1800 {
					maxColW = 860
					wideColW = 1060
				}
				if doc != nil {
					for _, m := range doc.Modules {
						if m.Kind == "plate" || m.Kind == "data_table" {
							maxColW = wideColW
							break
						}
						if m.Kind == "step" {
							if blks, ok := m.Data["blocks"].([]any); ok {
								for _, raw := range blks {
									if bm, ok := raw.(map[string]any); ok {
										k := fmt.Sprint(bm["kind"])
										if k == "plate" || k == "data_table" {
											maxColW = wideColW
											break
										}
									}
								}
							}
						}
					}
				}
				sidenoteW := float32(tok.Spacing48)*4 + float32(tok.Spacing32)
				sidenoteGap := float32(tok.Spacing24)
				showSidenotes := !mobileLayout && asterdoc.HasFootnotes(ld.Source) && paneW >= 920
				v.ShowRightSidenotes = showSidenotes
				v.SidenoteWidth = sidenoteW
				outerMaxW := maxColW
				if showSidenotes {
					outerMaxW = maxColW + sidenoteW + sidenoteGap
					if paneW > 0 && outerMaxW > paneW-padH {
						outerMaxW = paneW - padH
					}
				}
				ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(outerMaxW).
						Padding(padTop, padH, padBot, padH).
						Gap(float32(tok.Spacing16)).Children(func() {
						// Clean unframed `# Title` (Obsidian/Notion borderless display heading)
						if tab.Mode == shell.ModeEdit {
							if a.liveTitleDrafts == nil {
								a.liveTitleDrafts = map[string]*string{}
							}
							tBuf := a.liveTitleDrafts[tab.FilePath]
							if tBuf == nil {
								s := docTitle
								tBuf = &s
								a.liveTitleDrafts[tab.FilePath] = tBuf
							}
							ti := asterdoc.BorderlessTextInput(c, tok, tBuf, float32(tok.TextPage), true, "输入实验名称…")
							if ti.Submitted() || (!ti.Focused() && strings.TrimSpace(*tBuf) != "" && strings.TrimSpace(*tBuf) != docTitle) {
								next := asterdoc.SetFrontmatterMetadata(ld.Source, strings.TrimSpace(*tBuf), docDate, docStatus)
								applyLiveSource(next)
							}
						} else {
							ui.Text(c, docTitle).FontSize(float32(tok.TextPage)).Bold().TextColor(tok.Text)
						}

						// Obsidian Properties: borderless soft gray card on Mobile (matching iOS Obsidian), quiet single-line strip on Desktop
						renderPropsFields := func() {
							// Date property
							if a.liveDateDrafts == nil {
								a.liveDateDrafts = map[string]*string{}
							}
							dBuf := a.liveDateDrafts[tab.FilePath]
							if dBuf == nil {
								s := docDate
								dBuf = &s
								a.liveDateDrafts[tab.FilePath] = dBuf
							}
							dateBadge := firstNonEmpty(docDate, "未排期")
							projFolders := append([]string{""}, a.vault.ProjectFolders()...)
							folderVal := folderRel
							statusLabels := map[string]string{
								"planned":     "计划中",
								"in_progress": "进行中",
								"completed":   "已完成",
							}
							sVal := docStatus

							propRow := func(icon *ui.SVG, label string, valueFn func()) {
								ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Padding(float32(tok.Spacing4)*0.5, 0).Children(func() {
									labelW := float32(tok.Spacing48) * 1.8
									if mobileLayout {
										labelW = float32(tok.Spacing48) * 1.35
									}
									ui.Row(c).Width(labelW).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
										ui.Icon(c, icon).Size(float32(tok.Spacing16)*0.9, float32(tok.Spacing16)*0.9).TextColor(tok.TextMuted)
										ui.Text(c, label).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
									})
									ui.Box(c).Grow(1).MinWidth(0).Children(valueFn)
								})
							}

							if mobileLayout {
								ui.Column(c).FillWidth().Gap(float32(tok.Spacing8)).Children(func() {
									ui.Text(c, "实验属性").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
									ui.Column(c).FillWidth().Padding(float32(tok.Spacing12), float32(tok.Spacing16)).
										Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
										propRow(lucide.Calendar(), "日期", func() {
											if tab.Mode == shell.ModeEdit {
												dt := asterdoc.BorderlessTextInput(c, tok, dBuf, float32(tok.TextBody), false, "YYYY-MM-DD").
													FontFeatures("tnum")
												if dt.Submitted() || (!dt.Focused() && strings.TrimSpace(*dBuf) != docDate) {
													rawDate := strings.TrimSpace(*dBuf)
													if rawDate == "" {
														next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, "", docStatus)
														applyLiveSource(next)
													} else if normDate := asterdoc.NormalizeDateYMD(rawDate); normDate != "" {
														*dBuf = normDate
														if normDate != docDate {
															next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, normDate, docStatus)
															applyLiveSource(next)
														}
													} else {
														*dBuf = docDate
													}
												}
											} else {
												ui.Text(c, dateBadge).FontSize(float32(tok.TextBody)).TextColor(tok.Text).FontFeatures("tnum")
											}
										})
										propRow(lucide.Folder(), "项目", func() {
											if tab.Mode == shell.ModeEdit {
												pSel := components.Select(c, &folderVal)
												asterdoc.StyleSelectQuiet(c, tok, pSel)
												pSel.Trigger.Children(func() {
													ui.Text(c, folderLabel).FontSize(float32(tok.TextBody)).TextColor(tok.Text)
												})
												pSel.Popup(func(panel ui.Element) {
													for _, pf := range projFolders {
														pf := pf
														lbl := firstNonEmpty(pf, "根目录")
														pSel.Item(pf).Children(func() {
															ui.Text(c, lbl).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
														})
													}
												})
												if pSel.Trigger.Changed() && folderVal != folderRel {
													a.moveVaultFile(tab.FilePath, folderVal)
												}
											} else {
												ui.Text(c, folderLabel).FontSize(float32(tok.TextBody)).TextColor(tok.Text)
											}
										})
										propRow(lucide.Tag(), "状态", func() {
											sSel := components.Select(c, &sVal)
											asterdoc.StyleSelectQuiet(c, tok, sSel)
											sSel.Trigger.Children(func() {
												components.Badge(c, statusLabels[sVal], components.BadgeOptions{Tone: components.BadgeAccent})
											})
											sSel.Popup(func(panel ui.Element) {
												for _, opt := range []string{"planned", "in_progress", "completed"} {
													opt := opt
													sSel.Item(opt).Children(func() {
														ui.Text(c, statusLabels[opt]).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
													})
												}
											})
											if sSel.Trigger.Changed() && sVal != docStatus {
												next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, docDate, sVal)
												applyLiveSource(next)
											}
										})
									})
								})
								return
							}

							ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
									ui.Icon(c, lucide.Calendar()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
									if tab.Mode == shell.ModeEdit {
										ui.Box(c).Width(float32(tok.Spacing48)*1.9).Children(func() {
											dt := asterdoc.BorderlessTextInput(c, tok, dBuf, float32(tok.TextMeta), false, "YYYY-MM-DD").
												FontFeatures("tnum")
											if dt.Submitted() || (!dt.Focused() && strings.TrimSpace(*dBuf) != docDate) {
												rawDate := strings.TrimSpace(*dBuf)
												if rawDate == "" {
													next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, "", docStatus)
													applyLiveSource(next)
												} else if normDate := asterdoc.NormalizeDateYMD(rawDate); normDate != "" {
													*dBuf = normDate
													if normDate != docDate {
														next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, normDate, docStatus)
														applyLiveSource(next)
													}
												} else {
													*dBuf = docDate
												}
											}
										})
									} else {
										ui.Text(c, dateBadge).FontSize(float32(tok.TextData)).TextColor(tok.TextMuted).FontFeatures("tnum")
									}
								})

								if tab.Mode == shell.ModeEdit {
									pSel := components.Select(c, &folderVal)
									asterdoc.StyleSelectQuiet(c, tok, pSel)
									pSel.Trigger.Children(func() {
										ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
											ui.Icon(c, lucide.Folder()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
											ui.Text(c, folderLabel).FontSize(float32(tok.TextData)).TextColor(tok.Text)
										})
									})
									pSel.Popup(func(panel ui.Element) {
										for _, pf := range projFolders {
											pf := pf
											lbl := firstNonEmpty(pf, "根目录")
											pSel.Item(pf).Children(func() {
												ui.Text(c, lbl).FontSize(float32(tok.TextData)).TextColor(tok.Text)
											})
										}
									})
									if pSel.Trigger.Changed() && folderVal != folderRel {
										a.moveVaultFile(tab.FilePath, folderVal)
									}
								} else {
									ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ui.Icon(c, lucide.Folder()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
										ui.Text(c, folderLabel).FontSize(float32(tok.TextData)).TextColor(tok.TextMuted)
									})
								}

								sSel := components.Select(c, &sVal)
								asterdoc.StyleSelectQuiet(c, tok, sSel)
								sSel.Trigger.Children(func() {
									ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ui.Box(c).Size(float32(tok.Spacing4)*1.5, float32(tok.Spacing4)*1.5).Radius(float32(tok.ControlRadius)).Background(tok.Accent)
										ui.Text(c, statusLabels[sVal]).FontSize(float32(tok.TextData)).TextColor(tok.Accent)
									})
								})
								sSel.Popup(func(panel ui.Element) {
									for _, opt := range []string{"planned", "in_progress", "completed"} {
										opt := opt
										sSel.Item(opt).Children(func() {
											ui.Text(c, statusLabels[opt]).FontSize(float32(tok.TextData)).TextColor(tok.Text)
										})
									}
								})
								if sSel.Trigger.Changed() && sVal != docStatus {
									next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, docDate, sVal)
									applyLiveSource(next)
								}
							})
							ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.45))
						}
						renderPropsFields()

						// Render the single-source document body & step modules
						v.Render(c)

						// Deterministic Pure-Go Scientific Analysis Projection (CCK-8 / RT-qPCR ΔΔCq / Descriptive)
						if summaries := a.cachedDocumentAnalysis(tab.FilePath, ld.Source, doc); len(summaries) > 0 {
							analOpen := ui.Local(c.Root(), "doc-analysis-open:"+pane.ID+":"+tab.ID, func() bool { return true })
							ui.Column(c.Key("doc-analysis-card:"+tab.FilePath)).FillWidth().
								Padding(float32(tok.Spacing12)).Radius(float32(tok.PanelRadius)).
								Background(tok.Surface).Border(1, tok.Border).Gap(float32(tok.Spacing8)).Children(func() {
								hdrBtn := ui.Row(c.Key("doc-analysis-hdr")).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8))
								chev := lucide.ChevronRight()
								if *analOpen {
									chev = lucide.ChevronDown()
								}
								hdrBtn.Children(func() {
									ui.Icon(c, chev).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
									ui.Text(c, "读数自动汇总").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text)
									ui.Spacer(c)
									components.Badge(c, fmt.Sprintf("%d 项统计", len(summaries)))
								})
								if hdrBtn.Clicked() {
									*analOpen = !*analOpen
								}
								if *analOpen {
									for sIdx, sum := range summaries {
										sum := sum
										ui.Column(c.Key(fmt.Sprintf("sum-%d-%s", sIdx, sum.Kind))).FillWidth().
											Padding(float32(tok.Spacing12)).Radius(float32(tok.ControlRadius)).
											Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
											ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
												ui.Text(c, asterdoc.FormatDisplayText(sum.Title)).FontSize(float32(tok.TextData)).Bold().TextColor(tok.Text)
												if sum.Control != "" {
													ui.Text(c, "对照："+asterdoc.FormatDisplayText(sum.Control)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
												}
												if sum.BlankN > 0 && sum.BlankMean != nil {
													ui.Text(c, fmt.Sprintf("空白本底 %s（%d 孔）", asterdoc.FormatSciStat(*sum.BlankMean), sum.BlankN)).
														FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
												}
												if sum.ExcludedN > 0 {
													ui.Text(c, fmt.Sprintf("已剔除 %d 异常孔", sum.ExcludedN)).FontSize(float32(tok.TextMeta)).TextColor(tok.Warning).FontFeatures("tnum")
												}
											})
											switch sum.Kind {
											case "cck8":
												if sum.CCK8 != nil {
													for _, g := range sum.CCK8.Groups {
														ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
															lblEl := ui.Text(c, asterdoc.FormatDisplayText(g.Group)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
															if !mobileLayout {
																lblEl.MinWidth(160)
															}
															ui.Text(c, fmt.Sprintf("均值 %s ± %s（%d 复孔）", asterdoc.FormatSciStat(g.Mean), asterdoc.FormatSciStat(g.SD), g.N)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").Grow(1)
															ui.Text(c, fmt.Sprintf("相对活力 %.1f%%", g.RelativeActivity*100)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
														})
													}
												}
											case "qpcr":
												if sum.QPCR != nil {
													for _, g := range sum.QPCR.Groups {
														ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
															lblEl := ui.Text(c, asterdoc.FormatDisplayText(g.SampleID)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
															if !mobileLayout {
																lblEl.MinWidth(160)
															}
															ui.Text(c, fmt.Sprintf("ΔΔCq %.2f", g.DeltaDeltaCq)).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").Grow(1)
															ui.Text(c, fmt.Sprintf("相对表达 %.2f×", g.FoldChange)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
														})
													}
												}
											default:
												for _, g := range sum.Groups {
													ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing12)).Children(func() {
														lblEl := ui.Text(c, asterdoc.FormatDisplayText(g.Group)).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Text)
														if !mobileLayout {
															lblEl.MinWidth(160)
														}
														repTxt := fmt.Sprintf("%d 复孔", g.TechReplicates)
														if g.BioSamples > 0 {
															repTxt = fmt.Sprintf("%d 复孔 · 独立 n=%d", g.TechReplicates, g.BioSamples)
														}
														statTxt := fmt.Sprintf("均值 %s ± %s（%s）", asterdoc.FormatSciStat(g.Mean), asterdoc.FormatSciStat(g.SD), repTxt)
														ui.Text(c, statTxt).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum").Grow(1)
														if g.BlankCorrMean != nil {
															ui.Text(c, fmt.Sprintf("扣空白 %s", asterdoc.FormatSciStat(*g.BlankCorrMean))).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent).FontFeatures("tnum")
														}
													})
												}
											}
										})
									}
								}
							})
						}

					})
				})
			})
			v.RenderPlateOverlay(c)
		})
	})
}

// CaptureLabPhoto captures a lab photo via mobile camera (or test hook).
func (a *app) CaptureLabPhoto(ctx context.Context) (string, error) {
	if a != nil && a.capturePhotoFn != nil {
		return a.capturePhotoFn(ctx)
	}
	return CaptureLabPhoto(ctx)
}

// ShareFile opens the OS native share sheet for path (or test hook).
func (a *app) ShareFile(path, text string) error {
	if a != nil && a.shareFileFn != nil {
		return a.shareFileFn(path, text)
	}
	return ShareFile(path, text)
}

func (a *app) activeLiveViewForTab(tab shell.WorkspaceTab) *asterdoc.View {
	if a == nil || len(a.liveViews) == 0 {
		return nil
	}
	if a.shell != nil {
		if pane := a.shell.ActivePane(); pane != nil {
			if v := a.liveViews[pane.ID+"|"+tab.ID+"|"+tab.FilePath]; v != nil {
				return v
			}
		}
	}
	for k, v := range a.liveViews {
		if v == nil {
			continue
		}
		if (tab.ID != "" && strings.HasSuffix(k, "|"+tab.ID+"|"+tab.FilePath)) ||
			(tab.FilePath != "" && strings.HasSuffix(k, "|"+tab.FilePath)) {
			return v
		}
	}
	return nil
}

func mapMobileToolbarActionToAsterDoc(action string) (string, bool) {
	switch action {
	case "at_entity", "trigger_mention":
		return "trigger_mention", true
	case "slash_module", "trigger_slash":
		return "trigger_slash", true
	case "footnote", "add_footnote":
		return "add_footnote", true
	case "nl_structure", "trigger_nl":
		return "trigger_nl", true
	case "heading", "format_heading":
		return "format_heading", true
	case "bold", "format_bold":
		return "format_bold", true
	case "checklist", "format_checklist":
		return "format_checklist", true
	case "bullet_list", "format_bullet":
		return "format_bullet", true
	case "indent":
		return "indent", true
	case "outdent":
		return "outdent", true
	case "move_block_up":
		return "move_block_up", true
	case "move_block_down":
		return "move_block_down", true
	case "fold_block":
		return "fold_block", true
	case "cursor_left":
		return "cursor_left", true
	case "cursor_right":
		return "cursor_right", true
	default:
		return "", false
	}
}

func addPageToast(c *ui.Context, title string, tone string) {
	if c != nil && strings.TrimSpace(title) != "" {
		c.AddToast(ui.Toast{Title: title, Type: tone})
	}
}

func (a *app) handleMobileDocToolbarAction(c *ui.Context, curTab shell.WorkspaceTab, action string) {
	if a == nil || a.docStore == nil || a.vault == nil || strings.TrimSpace(curTab.FilePath) == "" {
		return
	}
	curLD, err := a.docStore.Open(curTab.FilePath)
	if err != nil {
		return
	}
	applyDoc := func(next string) {
		if next == curLD.Source {
			return
		}
		a.stageEmergencyDraft(curTab.FilePath, next, 0)
		_ = curLD.ApplyEdit(next)
		curLD.NotifySaved(a.vault, false)
		_, _, _ = a.snapshotDocFile(curTab.FilePath, "toolbar-edit", false)
		if a.vaultIndex != nil {
			a.vaultIndex.UpsertSource(curTab.FilePath, curLD.Source)
		}
		if a.liveEditorSynced != nil {
			a.liveEditorSynced[curTab.FilePath] = strings.ReplaceAll(curLD.Source, "\r\n", "\n")
		}
		a.lastCompanionKey = ""
		a.refreshVaultTreeAndCalendar()
		a.syncActiveDocCompanionState(curTab.FilePath, curLD.Source, curLD.Parsed)
	}

	if mapped, ok := mapMobileToolbarActionToAsterDoc(action); ok {
		if v := a.activeLiveViewForTab(curTab); v != nil {
			v.DispatchMobileAction(mapped)
			if a.shell != nil {
				a.shell.Invalidate()
			}
		}
		return
	}

	fm, _, _ := asterdoc.ScanFrontmatter(curLD.Source)
	docTitle := strings.TrimSpace(fm.Title)
	if docTitle == "" {
		docTitle = strings.TrimSuffix(filepath.Base(curTab.FilePath), ".asterml")
	}
	docDate := asterdoc.NormalizeDateYMD(fm.ScheduledDate)

	lastStepID := ""
	if curLD.Parsed != nil {
		for _, m := range curLD.Parsed.Modules {
			if m.Kind == "step" {
				lastStepID = m.ID
			}
		}
	}
	runMobileCmd := func(cmdID string, args map[string]any, toastTitle string) {
		var platFmt, excPct int
		if a.shell != nil {
			platFmt = a.shell.DefaultPlateFormat
			excPct = a.shell.DefaultExcessPct
		}
		res := astercmd.ExecuteBatch(curLD.Source, astercmd.BatchRequest{
			Mode:        astercmd.ModeEdit,
			PlateFormat: platFmt,
			ExcessPct:   excPct,
			Operations:  []astercmd.Operation{{CommandID: cmdID, Args: args}},
		})
		if res.OK {
			applyDoc(res.NextSource)
			addPageToast(c, toastTitle, components.ToastSuccess)
		} else if res.Error != nil {
			addPageToast(c, res.Error.Message, components.ToastWarning)
		}
	}

	switch action {
	case "undo":
		if a.shell != nil && a.shell.OnUndo != nil {
			a.shell.OnUndo()
		}
	case "redo":
		if a.shell != nil && a.shell.OnRedo != nil {
			a.shell.OnRedo()
		}
	case "add_step":
		runMobileCmd("step.create", map[string]any{"preset": "blank"}, "已添加空白步骤")
	case "add_checklist":
		args := map[string]any{"kind": "checklist"}
		if lastStepID != "" {
			args["step_id"] = lastStepID
		}
		runMobileCmd("block.insert", args, "已插入检查清单")
	case "add_param":
		args := map[string]any{"kind": "param_table"}
		if lastStepID != "" {
			args["step_id"] = lastStepID
		}
		runMobileCmd("block.insert", args, "已插入参数表")
	case "add_plate":
		platFmt := 12
		if a.shell != nil && a.shell.DefaultPlateFormat > 0 {
			platFmt = a.shell.DefaultPlateFormat
		}
		args := map[string]any{"format": platFmt}
		if lastStepID != "" {
			args["step_id"] = lastStepID
		}
		runMobileCmd("plate.create", args, "已插入孔板")
	case "camera_photo":
		a.importCameraPhotoIntoDoc(c, curTab, curLD, applyDoc)
	case "export_tidy_csv":
		if err := a.exportDocumentTidyCSV(c, curTab.FilePath, curLD.Parsed); err != nil {
			addPageToast(c, "导出 Tidy CSV 失败："+err.Error(), components.ToastWarning)
		}
	case "export_docx":
		if err := a.exportDocumentDocx(c, curTab.FilePath, curLD.Parsed); err != nil {
			addPageToast(c, "导出 Word 失败："+err.Error(), components.ToastWarning)
		}
	case "import_plate_data":
		if err := a.importPlateDataIntoDoc(c, curTab.FilePath); err != nil {
			addPageToast(c, "导入孔板数据失败："+err.Error(), components.ToastWarning)
		}
	case "share_doc":
		if err := a.shareDocument(c, curTab.FilePath, curLD.Parsed); err != nil {
			addPageToast(c, "分享实验文档失败："+err.Error(), components.ToastWarning)
		}
	case "open_timer", "timer":
		if a.shell != nil {
			a.shell.RightTab = 1
			a.shell.MobileDrawer = "outline"
		}
	case "source_mode":
		if a.shell != nil {
			if curTab.Mode == shell.ModeSource {
				a.shell.SetActiveTabMode(shell.ModeEdit)
			} else {
				a.shell.SetActiveTabMode(shell.ModeSource)
			}
		}
	case "copy_source":
		if c != nil {
			c.Services().WriteClipboard(curLD.Source)
			addPageToast(c, "已复制 AsterML 原文", components.ToastSuccess)
		}
	case "export_pdf":
		if curLD.Parsed != nil {
			a.startDocumentPDFExport(c, docTitle, "", "", a.vault.Root, curLD.Parsed)
		}
	case "save_template":
		if tpl, tErr := a.vault.SaveAsTemplate(curTab.FilePath, docTitle); tErr == nil {
			a.refreshVaultTreeAndCalendar()
			addPageToast(c, "已保存为模板："+tpl.Name, components.ToastSuccess)
		}
	case "duplicate":
		if dup, dErr := a.vault.DuplicateFile(curTab.FilePath, docDate); dErr == nil {
			a.refreshVaultTreeAndCalendar()
			if a.shell != nil {
				if pane := a.shell.ActivePane(); pane != nil {
					a.shell.OpenFileInPane(pane.ID, dup.RelPath, dup.Title, shell.ModeEdit, true)
				}
			}
		}
	case "delete":
		a.deleteVaultFile(curTab.FilePath)
	}
}

type photoRouteOption struct {
	Kind     string // "figure_slot" | "plate_embed" | "inline_embed"
	Label    string
	Detail   string
	ModuleID string
	StepID   string
	PlateID  string
}

type photoRouteRequest struct {
	open     bool
	asset    string
	tabPath  string
	options  []photoRouteOption
	applyDoc func(string)
}

func sanitizeWikiAssetPath(relAsset string) string {
	clean := filepath.ToSlash(strings.TrimSpace(relAsset))
	clean = strings.ReplaceAll(clean, "[", "")
	clean = strings.ReplaceAll(clean, "]", "")
	clean = strings.ReplaceAll(clean, "|", "")
	clean = strings.ReplaceAll(clean, "\r", "")
	clean = strings.ReplaceAll(clean, "\n", "")
	clean = strings.TrimLeft(clean, "/")
	if clean == "" {
		return ""
	}
	if !strings.HasPrefix(clean, "./") {
		clean = "./" + clean
	}
	return clean
}

func isEmptyPlaceholderSVG(svg string) bool {
	s := strings.ToLower(strings.TrimSpace(svg))
	if s == "" {
		return true
	}
	for _, shape := range []string{"<path", "<rect", "<circle", "<ellipse", "<line", "<polyline", "<polygon", "<text", "<image"} {
		if strings.Contains(s, shape) {
			return false
		}
	}
	return true
}

func collectPhotoRouteOptions(doc *asterdoc.Document) []photoRouteOption {
	if doc == nil {
		return nil
	}
	var slots []photoRouteOption
	var plates []photoRouteOption
	inspectMod := func(kind, id, stepID, stepTitle string, data map[string]any) {
		switch kind {
		case "figure":
			src := strings.TrimSpace(fmt.Sprint(data["src"]))
			if src == "<nil>" {
				src = ""
			}
			svg := strings.TrimSpace(fmt.Sprint(data["svg"]))
			if svg == "<nil>" {
				svg = ""
			}
			if src == "" && isEmptyPlaceholderSVG(svg) && id != "" {
				title := strings.TrimSpace(fmt.Sprint(data["title"]))
				if title == "" || title == "<nil>" {
					title = "待拍照片槽位 (" + id + ")"
				}
				detail := "填入该照片槽位（figure@1）"
				if stepTitle != "" {
					detail = "步骤「" + stepTitle + "」照片槽位"
				}
				slots = append(slots, photoRouteOption{
					Kind:     "figure_slot",
					Label:    "填入照片槽位 · " + title,
					Detail:   detail,
					ModuleID: id,
					StepID:   stepID,
				})
			}
		case "plate":
			if id != "" {
				title := strings.TrimSpace(fmt.Sprint(data["title"]))
				if title == "" || title == "<nil>" {
					title = id
				}
				plates = append(plates, photoRouteOption{
					Kind:    "plate_embed",
					Label:   "以 ![[]] 嵌入并关联孔板 · " + title,
					Detail:  "插入 ![[...]]{@" + id + "}，可在卡片中再点选具体孔位",
					PlateID: id,
				})
			}
		}
	}
	for _, m := range doc.Modules {
		inspectMod(m.Kind, m.ID, "", "", m.Data)
		if m.Kind == "step" {
			stTitle := strings.TrimSpace(fmt.Sprint(m.Data["title"]))
			if stTitle == "<nil>" {
				stTitle = ""
			}
			if blks, ok := m.Data["blocks"].([]any); ok {
				for _, raw := range blks {
					if bm, ok := raw.(map[string]any); ok {
						inspectMod(
							strings.TrimSpace(fmt.Sprint(bm["kind"])),
							strings.TrimSpace(fmt.Sprint(bm["id"])),
							m.ID,
							stTitle,
							bm,
						)
					}
				}
			}
		}
	}
	out := make([]photoRouteOption, 0, len(slots)+len(plates)+1)
	out = append(out, slots...)
	out = append(out, plates...)
	if len(out) > 0 {
		out = append(out, photoRouteOption{
			Kind:   "inline_embed",
			Label:  "直接以 ![[]] 插入文档正文",
			Detail: "生成标准可关联图片卡片，不占用既有槽位",
		})
	}
	return out
}

func executePhotoRouteChoice(source string, mode astercmd.ExecMode, platFmt, excPct int, relAsset string, opt photoRouteOption) astercmd.BatchResult {
	safeAsset := sanitizeWikiAssetPath(relAsset)
	if safeAsset == "" {
		return astercmd.BatchResult{Error: &astercmd.CommandError{Code: "INVALID_ARGS", Message: "无效的照片路径"}}
	}
	switch opt.Kind {
	case "figure_slot":
		return astercmd.ExecuteBatch(source, astercmd.BatchRequest{
			Mode:        astercmd.ModeEdit,
			PlateFormat: platFmt,
			ExcessPct:   excPct,
			Operations: []astercmd.Operation{{
				CommandID: "block.update",
				Args: map[string]any{
					"module_id": opt.ModuleID,
					"step_id":   opt.StepID,
					"props": map[string]any{
						"src":     safeAsset,
						"svg":     "",
						"caption": "现场实验照片（原图已无损归档）",
					},
				},
			}},
		})
	default:
		atOff := -1
		if fnDoc := asterdoc.ParseFootnotes(source); fnDoc != nil && fnDoc.TrailingDefStart >= 0 {
			atOff = fnDoc.TrailingDefStart
			for atOff > 0 && (source[atOff-1] == '\n' || source[atOff-1] == '\r') {
				atOff--
			}
		}
		args := map[string]any{
			"asset":     safeAsset,
			"at_offset": atOff,
		}
		if opt.Kind == "plate_embed" && opt.PlateID != "" {
			args["targets"] = []string{opt.PlateID}
		}
		return astercmd.ExecuteBatch(source, astercmd.BatchRequest{
			Mode:        mode,
			PlateFormat: platFmt,
			ExcessPct:   excPct,
			Operations: []astercmd.Operation{{
				CommandID: "asset.embed",
				Args:      args,
			}},
		})
	}
}

func (a *app) importCameraPhotoIntoDoc(c *ui.Context, curTab shell.WorkspaceTab, curLD *vault.LiveDocument, applyDoc func(string)) {
	photoPath, err := a.CaptureLabPhoto(context.Background())
	if errors.Is(err, errors.ErrUnsupported) {
		paths, dErr := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  mygo.FocusedWindow(),
			Title:   "选择实验拍照或显微照片",
			Filters: []mygo.FileFilter{{Name: "图像文件", Extensions: []string{"png", "jpg", "jpeg", "webp", "tif", "tiff"}}},
		})
		if dErr != nil || len(paths) == 0 {
			return
		}
		photoPath = paths[0]
		err = nil
	}
	if err != nil {
		addPageToast(c, "拍照失败："+err.Error(), components.ToastWarning)
		return
	}
	if strings.TrimSpace(photoPath) == "" {
		return
	}
	relAsset, err := a.vault.ImportAssetFile(photoPath)
	if err != nil {
		addPageToast(c, "导入照片原图失败："+err.Error(), components.ToastWarning)
		return
	}
	_, _ = a.generateAssetPreviewSidecar(relAsset)

	safeAsset := sanitizeWikiAssetPath(relAsset)
	var platFmt, excPct int
	if a.shell != nil {
		platFmt = a.shell.DefaultPlateFormat
		excPct = a.shell.DefaultExcessPct
	}
	opts := collectPhotoRouteOptions(curLD.Parsed)
	if len(opts) > 0 {
		a.pendingPhotoRoute = &photoRouteRequest{
			open:     true,
			asset:    safeAsset,
			tabPath:  curTab.FilePath,
			options:  opts,
			applyDoc: applyDoc,
		}
		return
	}

	res := executePhotoRouteChoice(curLD.Source, astercmd.ModeEdit, platFmt, excPct, safeAsset, photoRouteOption{Kind: "inline_embed"})
	if res.OK {
		applyDoc(res.NextSource)
		addPageToast(c, "已无损归档照片并插入 ![[...]] 图片卡片", components.ToastSuccess)
	} else if res.Error != nil {
		addPageToast(c, res.Error.Message, components.ToastWarning)
	}
}

func (a *app) renderPhotoRouteSheet(c *ui.Context) {
	if a == nil || a.pendingPhotoRoute == nil || a.docStore == nil {
		return
	}
	req := a.pendingPhotoRoute
	tok := theme.GetTokens(c.Theme())
	mobile.Sheet(c, &req.open, mobile.SheetOptions{
		Detents:    []float32{0.50, 0.80},
		Background: &tok.Surface,
		Label:      "选择照片归档位置",
	}, func() {
		ui.Column(c.Key("photo-route-sheet")).FillWidth().
			Padding(float32(tok.Spacing8), float32(tok.Spacing16), float32(tok.Spacing24), float32(tok.Spacing16)).
			Gap(float32(tok.Spacing12)).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				kit.IconBox(c, tok, lucide.Camera(), tok.Accent)
				ui.Text(c, "选择照片归档位置").FontSize(float32(tok.TextSection)).Bold().TextColor(tok.Text).Grow(1).MinWidth(0)
			})
			ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Clip().Children(func() {
				for i, opt := range req.options {
					opt := opt
					if i > 0 {
						ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
					}
					row := ui.ButtonBase(c.Key(fmt.Sprintf("photo-route-%d-%s-%s", i, opt.Kind, opt.ModuleID+opt.PlateID))).
						FillWidth().MinHeight(float32(tok.Spacing48)).
						Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
						Justify(ui.Start).Label(opt.Label).
						OnClick(func() {
							ld, lErr := a.docStore.Open(req.tabPath)
							if lErr != nil || ld == nil {
								req.open = false
								return
							}
							var platFmt, excPct int
							if a.shell != nil {
								platFmt = a.shell.DefaultPlateFormat
								excPct = a.shell.DefaultExcessPct
							}
							res := executePhotoRouteChoice(ld.Source, astercmd.ModeEdit, platFmt, excPct, req.asset, opt)
							req.open = false
							if res.OK {
								if req.applyDoc != nil {
									req.applyDoc(res.NextSource)
								}
								mygo.Haptics.Success()
								addPageToast(c, "已归档现场照片", components.ToastSuccess)
							} else if res.Error != nil {
								addPageToast(c, res.Error.Message, components.ToastWarning)
							}
						})
					theme.ApplyTactilePress(row)
					row.Children(func() {
						ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)*0.5).Children(func() {
							ui.Text(c, opt.Label).FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
							if opt.Detail != "" {
								ui.Text(c, opt.Detail).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
							}
						})
					})
				}
			})
		})
	})
}

func (a *app) saveExportBytes(c *ui.Context, defaultName, title string, filter mygo.FileFilter, data []byte) (string, error) {
	if a != nil && a.saveExportFileFn != nil {
		return a.saveExportFileFn(defaultName, title, data)
	}
	home, _ := os.UserHomeDir()
	defPath := filepath.Join(home, defaultName)
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      mygo.FocusedWindow(),
		Title:       title,
		DefaultPath: defPath,
		Filters:     []mygo.FileFilter{filter},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	if wErr := asterdoc.WriteFileAtomic(path, data); wErr != nil {
		return "", wErr
	}
	addPageToast(c, "已导出："+filepath.Base(path), components.ToastSuccess)
	return path, nil
}

func docDisplayTitle(doc *asterdoc.Document, relPath string) string {
	if doc != nil {
		if fm, _, ok := asterdoc.ScanFrontmatter(doc.Source); ok && strings.TrimSpace(fm.Title) != "" {
			return strings.TrimSpace(fm.Title)
		}
	}
	base := strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	if base == "" {
		return "AsterLab 实验记录"
	}
	return base
}

func sanitizeExportStem(name string) string {
	s := vault.SanitizeFileName(name)
	if s == "" {
		return "experiment"
	}
	return s
}

func (a *app) exportDocumentTidyCSV(c *ui.Context, relPath string, doc *asterdoc.Document) error {
	if doc == nil {
		return errors.New("文档尚未解析")
	}
	title := docDisplayTitle(doc, relPath)
	obs := buildTidyObservationsFromDoc(doc, title)
	csvBytes, err := labio.ExportTidyCSV(obs, labio.TidyCSVOptions{IncludeUTF8BOM: true})
	if err != nil {
		return err
	}
	_, err = a.saveExportBytes(c, sanitizeExportStem(title)+"-tidy.csv", "导出 Tidy 长表 CSV", mygo.FileFilter{Name: "CSV", Extensions: []string{"csv"}}, csvBytes)
	return err
}

func (a *app) exportDocumentDocx(c *ui.Context, relPath string, doc *asterdoc.Document) error {
	if doc == nil {
		return errors.New("文档尚未解析")
	}
	title := docDisplayTitle(doc, relPath)
	vaultRoot := ""
	if a != nil && a.vault != nil {
		vaultRoot = a.vault.Root
	}
	docxDoc := buildDocxDocumentFromAsterDoc(doc, title, vaultRoot)
	docxBytes, err := labio.ExportDocx(docxDoc)
	if err != nil {
		return err
	}
	_, err = a.saveExportBytes(c, sanitizeExportStem(title)+".docx", "导出 Word 实验报告 (.docx)", mygo.FileFilter{Name: "Word 文档", Extensions: []string{"docx"}}, docxBytes)
	return err
}

func (a *app) importPlateDataIntoDoc(c *ui.Context, relPath string) error {
	if a == nil || a.docStore == nil {
		return errors.New("文档存储未就绪")
	}
	var picked string
	if a.openImportFileFn != nil {
		p, err := a.openImportFileFn("选择酶标仪读数文件 (CSV / TSV / XLSX)")
		if err != nil {
			return err
		}
		picked = p
	} else {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  mygo.FocusedWindow(),
			Title:   "选择酶标仪读数文件 (CSV / TSV / XLSX)",
			Filters: []mygo.FileFilter{{Name: "表格与酶标仪导出", Extensions: []string{"csv", "tsv", "txt", "xlsx"}}},
		})
		if err != nil {
			return err
		}
		if len(paths) > 0 {
			picked = paths[0]
		}
	}
	if strings.TrimSpace(picked) == "" {
		return nil
	}
	raw, err := os.ReadFile(picked)
	if err != nil {
		return err
	}
	nextSrc, err := a.importPlateMatrixBytesIntoDoc(relPath, raw, strings.TrimSuffix(filepath.Base(picked), filepath.Ext(picked)))
	if err != nil {
		return err
	}
	if nextSrc != "" {
		addPageToast(c, "已导入酶标仪矩阵并生成数据表", components.ToastSuccess)
	}
	return nil
}

func (a *app) importPlateMatrixBytesIntoDoc(relPath string, rawBytes []byte, sourceTitle string) (string, error) {
	if a == nil || a.docStore == nil || a.vault == nil {
		return "", errors.New("vault doc store unavailable")
	}
	ld, err := a.docStore.Open(relPath)
	if err != nil {
		return "", err
	}
	parsedRes, err := labio.ImportPlateMatrix(rawBytes)
	if err != nil {
		return "", err
	}
	containerRef := ""
	if ld.Parsed != nil {
		for _, m := range ld.Parsed.Modules {
			if m.Kind == "plate" {
				containerRef = m.ID
				break
			}
			if m.Kind == "step" {
				if blks, ok := m.Data["blocks"].([]any); ok {
					for _, raw := range blks {
						if bm, ok := raw.(map[string]any); ok && fmt.Sprint(bm["kind"]) == "plate" {
							containerRef = fmt.Sprint(bm["id"])
							break
						}
					}
				}
			}
			if containerRef != "" {
				break
			}
		}
	}
	rows := make([]any, 0, len(parsedRes.Ordered))
	for _, w := range parsedRes.Ordered {
		if w.Value == nil && w.Raw == "" {
			continue
		}
		row := map[string]any{
			"well": w.Coord,
		}
		if w.Value != nil {
			row["value"] = *w.Value
		} else {
			row["value"] = w.Raw
		}
		if w.Group != "" {
			row["group"] = w.Group
		}
		if w.Timepoint != "" {
			row["timepoint"] = w.Timepoint
		}
		if w.Replicate > 0 {
			row["replicate"] = w.Replicate
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return "", errors.New("未识别到有效孔位读数")
	}
	tblTitle := "酶标仪导入读数"
	if strings.TrimSpace(sourceTitle) != "" {
		tblTitle = strings.TrimSpace(sourceTitle) + " 读数"
	}
	props := map[string]any{
		"columns": []any{
			map[string]any{"key": "well", "label": "孔位", "value_kind": "text"},
			map[string]any{"key": "value", "label": "测定读数", "value_kind": "number"},
		},
		"rows": rows,
	}
	if containerRef != "" {
		props["container_ref"] = containerRef
	}
	var platFmt, excPct int
	if a.shell != nil {
		platFmt = a.shell.DefaultPlateFormat
		excPct = a.shell.DefaultExcessPct
	}
	res := astercmd.ExecuteBatch(ld.Source, astercmd.BatchRequest{
		Mode:        astercmd.ModeEdit,
		PlateFormat: platFmt,
		ExcessPct:   excPct,
		Operations: []astercmd.Operation{{
			CommandID: "block.insert",
			Args: map[string]any{
				"kind":  "data_table",
				"title": tblTitle,
				"props": props,
			},
		}},
	})
	if !res.OK {
		if res.Error != nil {
			return "", res.Error
		}
		return "", errors.New("插入数据表失败")
	}
	a.stageEmergencyDraft(relPath, res.NextSource, 0)
	if err := ld.ApplyEdit(res.NextSource); err != nil {
		return "", err
	}
	ld.NotifySaved(a.vault, false)
	_, _, _ = a.snapshotDocFile(relPath, "import-plate-data", false)
	if a.vaultIndex != nil {
		a.vaultIndex.UpsertSource(relPath, ld.Source)
	}
	if a.liveEditorSynced != nil {
		a.liveEditorSynced[relPath] = strings.ReplaceAll(ld.Source, "\r\n", "\n")
	}
	a.lastCompanionKey = ""
	a.refreshVaultTreeAndCalendar()
	a.syncActiveDocCompanionState(relPath, ld.Source, ld.Parsed)
	return res.NextSource, nil
}

func (a *app) shareDocument(c *ui.Context, relPath string, doc *asterdoc.Document) error {
	if a == nil || a.vault == nil || doc == nil {
		return errors.New("文档未就绪")
	}
	title := docDisplayTitle(doc, relPath)
	shareDir := filepath.Join(a.vault.Root, ".asterlab", "cache", "share")
	_ = os.MkdirAll(shareDir, 0o755)
	pdfPath := filepath.Join(shareDir, sanitizeExportStem(title)+".pdf")
	f, err := os.Create(pdfPath)
	exportBytesPath := ""
	exportFilter := mygo.FileFilter{Name: "PDF", Extensions: []string{"pdf"}}
	if err == nil {
		pdfErr := asterdoc.ExportDocumentPDF(f, doc, asterdoc.DocumentPDFOptions{
			Title:   title,
			BaseDir: a.vault.Root,
		})
		_ = f.Close()
		if pdfErr == nil {
			exportBytesPath = pdfPath
		} else {
			_ = os.Remove(pdfPath)
		}
	}
	if exportBytesPath == "" {
		// Fallback to sharing raw .asterml if headless environment has no CJK PDF font.
		asterPath := filepath.Join(shareDir, sanitizeExportStem(title)+".asterml")
		if wErr := asterdoc.WriteFileAtomic(asterPath, []byte(doc.Source)); wErr != nil {
			return wErr
		}
		exportBytesPath = asterPath
		exportFilter = mygo.FileFilter{Name: "AsterML", Extensions: []string{"asterml"}}
	}
	err = a.ShareFile(exportBytesPath, title)
	if errors.Is(err, errors.ErrUnsupported) {
		raw, rErr := os.ReadFile(exportBytesPath)
		if rErr != nil {
			return rErr
		}
		_, err = a.saveExportBytes(c, filepath.Base(exportBytesPath), "保存分享文件", exportFilter, raw)
		return err
	}
	return err
}

func buildTidyObservationsFromDoc(doc *asterdoc.Document, docTitle string) []labio.TidyObservation {
	if doc == nil {
		return nil
	}
	groupNames := map[string]string{}
	groupRoles := map[string]string{}
	for _, g := range doc.GroupList() {
		groupNames[g.ID] = firstNonEmpty(g.Name, g.ID)
		groupRoles[g.ID] = g.Role
	}
	graph := asterdoc.BuildSciGraph(doc)
	defaultPlateID := ""
	if graph != nil {
		if pl := graph.PrimaryPlate(); pl != nil {
			defaultPlateID = pl.ID
		}
	}

	var dataTables []map[string]any
	for _, m := range doc.Modules {
		if m.Kind == "data_table" {
			dataTables = append(dataTables, m.Data)
		} else if m.Kind == "step" {
			if blks, ok := m.Data["blocks"].([]any); ok {
				for _, raw := range blks {
					if bm, ok := raw.(map[string]any); ok && fmt.Sprint(bm["kind"]) == "data_table" {
						dataTables = append(dataTables, bm)
					}
				}
			}
		}
	}

	var out []labio.TidyObservation
	for _, dt := range dataTables {
		metricName := firstNonEmpty(stringField(dt, "title"), "value")
		metricUnit := ""
		valueKey := ""
		if cols, ok := dt["columns"].([]any); ok {
			for _, rc := range cols {
				cm, _ := rc.(map[string]any)
				if cm == nil {
					continue
				}
				k := stringField(cm, "key")
				vk := stringField(cm, "value_kind")
				if (vk == "number" || k == "value" || k == "reading") && valueKey == "" {
					valueKey = k
					if lbl := stringField(cm, "label"); lbl != "" {
						metricName = lbl
					}
					metricUnit = stringField(cm, "unit")
				}
			}
		}
		if valueKey == "" {
			valueKey = "value"
		}
		plateID := firstNonEmpty(stringField(dt, "container_ref"), stringField(dt, "plate_id"), defaultPlateID)
		rows, _ := dt["rows"].([]any)
		for _, rr := range rows {
			rm, _ := rr.(map[string]any)
			if rm == nil {
				continue
			}
			well := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringField(rm, "well"), stringField(rm, "well_id"))))
			var grp, role, sampleID string
			var rep int
			if well != "" && graph != nil {
				if unit, ok := graph.UnitForWell(plateID, well); ok {
					grp = firstNonEmpty(unit.GroupName, unit.GroupID)
					role = groupRoles[unit.GroupID]
					if unit.Blank && role == "" {
						role = "blank"
					}
					sampleID = unit.SampleID
					rep = unit.RepIndex
				}
			}
			if g := stringField(rm, "group"); g != "" {
				grp = firstNonEmpty(groupNames[g], g)
				if role == "" {
					role = groupRoles[g]
				}
			}
			if sid := firstNonEmpty(stringField(rm, "sample_id"), stringField(rm, "sample")); sid != "" {
				sampleID = sid
			}
			if rVal, ok := asFloat(rm["replicate"]); ok && rVal > 0 {
				rep = int(rVal)
			}
			tp := stringField(rm, "timepoint")
			rawVal, hasVal := rm[valueKey]
			if !hasVal {
				for _, alt := range []string{"value", "reading", "od450", "ct"} {
					if v, ok := rm[alt]; ok {
						rawVal, hasVal = v, true
						break
					}
				}
			}
			obs := labio.TidyObservation{
				Experiment: docTitle,
				Group:      grp,
				Role:       role,
				Timepoint:  tp,
				Replicate:  rep,
				Well:       well,
				SampleID:   sampleID,
				Metric:     metricName,
				Unit:       metricUnit,
			}
			if !hasVal || rawVal == nil {
				obs.Missing = true
			} else if num, ok := asFloat(rawVal); ok {
				obs.Value = num
			} else {
				obs.Missing = true
				obs.RawValue = fmt.Sprint(rawVal)
			}
			out = append(out, obs)
		}
	}
	return out
}

func buildDocxDocumentFromAsterDoc(doc *asterdoc.Document, title, vaultRoot string) labio.DocxDocument {
	out := labio.DocxDocument{Title: title}
	if doc == nil {
		return out
	}
	if fm, _, ok := asterdoc.ScanFrontmatter(doc.Source); ok {
		if fm.ScheduledDate != "" {
			out.Metadata = append(out.Metadata, labio.DocxMetaKV{Key: "计划日期", Value: fm.ScheduledDate})
		}
		if fm.Stage != "" {
			out.Metadata = append(out.Metadata, labio.DocxMetaKV{Key: "阶段状态", Value: fm.Stage})
		}
	}
	fnDoc := asterdoc.ParseFootnotes(doc.Source)
	fnNumByID := map[string]int{}
	if fnDoc != nil {
		for _, e := range fnDoc.Entries {
			out.Footnotes = append(out.Footnotes, e.Body)
			fnNumByID[e.ID] = len(out.Footnotes)
		}
	}

	segs := asterdoc.SplitDocumentSegments(doc.Source, doc)
	var curSec *labio.DocxSection
	ensureSec := func() *labio.DocxSection {
		if curSec == nil {
			out.Sections = append(out.Sections, labio.DocxSection{Heading: "实验内容", Level: 1})
			curSec = &out.Sections[len(out.Sections)-1]
		}
		return curSec
	}
	for _, seg := range segs {
		switch seg.Kind {
		case asterdoc.SegmentProse:
			for _, ln := range strings.Split(seg.Text, "\n") {
				tr := strings.TrimSpace(ln)
				if tr == "" || strings.HasPrefix(tr, "[^") && strings.Contains(tr, "]:") {
					continue
				}
				if hs, isStep := asterdoc.ParseHeadingStepLine(tr); isStep {
					lvl := 1
					if hs.Level > 1 {
						lvl = 2
					}
					out.Sections = append(out.Sections, labio.DocxSection{Heading: hs.Title, Level: lvl})
					curSec = &out.Sections[len(out.Sections)-1]
					continue
				}
				if strings.HasPrefix(tr, "#") {
					lvl := len(tr) - len(strings.TrimLeft(tr, "#"))
					if lvl < 1 {
						lvl = 1
					} else if lvl > 3 {
						lvl = 3
					}
					heading := strings.TrimSpace(strings.TrimLeft(tr, "#"))
					if heading != "" {
						out.Sections = append(out.Sections, labio.DocxSection{Heading: heading, Level: lvl})
						curSec = &out.Sections[len(out.Sections)-1]
					}
					continue
				}
				sec := ensureSec()
				if strings.HasPrefix(tr, "- ") || strings.HasPrefix(tr, "* ") {
					sec.Bullets = append(sec.Bullets, strings.TrimSpace(tr[2:]))
					continue
				}
				var fnIDs []int
				for id, num := range fnNumByID {
					if strings.Contains(tr, "[^"+id+"]") {
						fnIDs = append(fnIDs, num)
					}
				}
				sec.Paragraphs = append(sec.Paragraphs, labio.DocxPara{Text: tr, FootnoteIDs: fnIDs})
			}
		case asterdoc.SegmentModule:
			if seg.Module == nil {
				continue
			}
			sec := ensureSec()
			appendModuleToDocxSection(sec, seg.Module.Kind, seg.Module.Data, seg.Module.Plate, vaultRoot)
		}
	}
	return out
}

func appendModuleToDocxSection(sec *labio.DocxSection, kind string, data map[string]any, plate *asterdoc.Plate, vaultRoot string) {
	switch kind {
	case "step":
		title := stringField(data, "title")
		if title != "" {
			sec.Paragraphs = append(sec.Paragraphs, labio.DocxPara{Text: title})
		}
		if blks, ok := data["blocks"].([]any); ok {
			for _, raw := range blks {
				if bm, ok := raw.(map[string]any); ok {
					appendModuleToDocxSection(sec, stringField(bm, "kind"), bm, nil, vaultRoot)
				}
			}
		}
	case "group_table":
		tbl := labio.DocxTable{Caption: stringField(data, "title"), Headers: []string{"组别", "角色", "条件", "样本量"}}
		if rows, ok := data["rows"].([]any); ok {
			for _, rr := range rows {
				if rm, ok := rr.(map[string]any); ok {
					nStr := firstNonEmpty(fmt.Sprint(rm["independent_n"]), fmt.Sprint(rm["technical_repeats"]))
					if nStr == "<nil>" {
						nStr = ""
					}
					tbl.Rows = append(tbl.Rows, []string{stringField(rm, "name"), stringField(rm, "role"), stringField(rm, "condition"), nStr})
				}
			}
		}
		sec.Tables = append(sec.Tables, tbl)
	case "param_table":
		tbl := labio.DocxTable{Caption: stringField(data, "title"), Headers: []string{"参数", "设定值", "单位", "实测值"}}
		if rows, ok := data["rows"].([]any); ok {
			for _, rr := range rows {
				if rm, ok := rr.(map[string]any); ok {
					tbl.Rows = append(tbl.Rows, []string{stringField(rm, "label"), stringField(rm, "value"), stringField(rm, "unit"), stringField(rm, "actual")})
				}
			}
		}
		sec.Tables = append(sec.Tables, tbl)
	case "data_table":
		var headers, keys []string
		if cols, ok := data["columns"].([]any); ok {
			for _, rc := range cols {
				if cm, ok := rc.(map[string]any); ok {
					keys = append(keys, stringField(cm, "key"))
					headers = append(headers, firstNonEmpty(stringField(cm, "label"), stringField(cm, "key")))
				}
			}
		}
		if len(headers) == 0 {
			headers = []string{"well", "value"}
			keys = []string{"well", "value"}
		}
		tbl := labio.DocxTable{Caption: stringField(data, "title"), Headers: headers}
		if rows, ok := data["rows"].([]any); ok {
			for _, rr := range rows {
				if rm, ok := rr.(map[string]any); ok {
					rowVals := make([]string, len(keys))
					for i, k := range keys {
						rowVals[i] = stringField(rm, k)
					}
					tbl.Rows = append(tbl.Rows, rowVals)
				}
			}
		}
		sec.Tables = append(sec.Tables, tbl)
	case "plate":
		if plate != nil {
			tbl := labio.DocxTable{Caption: firstNonEmpty(plate.Title, fmt.Sprintf("%d 孔板布局", plate.Format)), Headers: []string{"组别", "分配孔位"}}
			for g, ws := range plate.Groups {
				tbl.Rows = append(tbl.Rows, []string{g, strings.Join(ws, ", ")})
			}
			if len(tbl.Rows) > 0 {
				sec.Tables = append(sec.Tables, tbl)
			}
		}
	case "figure":
		src := stringField(data, "src")
		capText := firstNonEmpty(stringField(data, "caption"), stringField(data, "title"))
		if src != "" && vaultRoot != "" && strings.EqualFold(filepath.Ext(src), ".png") {
			abs := filepath.Join(vaultRoot, filepath.FromSlash(strings.TrimPrefix(src, "./")))
			if b, err := os.ReadFile(abs); err == nil {
				sec.Figures = append(sec.Figures, labio.DocxFigure{Caption: capText, PNGBytes: b})
			}
		} else if capText != "" {
			sec.Paragraphs = append(sec.Paragraphs, labio.DocxPara{Text: "[附图] " + capText})
		}
	}
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
