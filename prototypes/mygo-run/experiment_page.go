package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/asterdoc"
	"asterlab-mygo-prototype/internal/shell"
	"asterlab-mygo-prototype/internal/theme"
)

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
		_ = ld.ApplyEdit(next)
		if a.liveEditorSynced != nil {
			a.liveEditorSynced[tab.FilePath] = strings.ReplaceAll(ld.Source, "\r\n", "\n")
		}
		a.refreshVaultTreeAndCalendar()
	}

	mobileLayout := a.shell != nil && a.shell.IsMobileLayout(c)

	if a.shell != nil {
		a.shell.OnMobileDocToolbarAction = func(curTab shell.WorkspaceTab, action string) {
			curLD, err := a.docStore.Open(curTab.FilePath)
			if err != nil {
				return
			}
			applyDoc := func(next string) {
				if next == curLD.Source {
					return
				}
				_ = curLD.ApplyEdit(next)
				if a.liveEditorSynced != nil {
					a.liveEditorSynced[curTab.FilePath] = strings.ReplaceAll(curLD.Source, "\r\n", "\n")
				}
				a.refreshVaultTreeAndCalendar()
			}
			lastStepID := ""
			afterIdx := -1
			if curLD.Parsed != nil {
				for _, m := range curLD.Parsed.Modules {
					if m.Kind == "step" {
						lastStepID = m.ID
						if blks, ok := m.Data["blocks"].([]any); ok {
							afterIdx = len(blks) - 1
						}
					}
				}
			}
			ensureTargetStep := func() string {
				if lastStepID != "" {
					return lastStepID
				}
				if next, createdID, err := asterdoc.AddStepInSource(curLD.Source, "步骤 1"); err == nil {
					applyDoc(next)
					return createdID
				}
				return ""
			}
			switch action {
			case "undo":
				if a.shell.OnUndo != nil {
					a.shell.OnUndo()
				}
			case "redo":
				if a.shell.OnRedo != nil {
					a.shell.OnRedo()
				}
			case "add_step":
				cnt := 1
				if curLD.Parsed != nil {
					for _, m := range curLD.Parsed.Modules {
						if m.Kind == "step" {
							cnt++
						}
					}
				}
				if next, _, err := asterdoc.AddStepInSource(curLD.Source, fmt.Sprintf("步骤 %d", cnt)); err == nil {
					applyDoc(next)
					c.AddToast(ui.Toast{Title: fmt.Sprintf("已添加步骤 %d", cnt), Type: components.ToastSuccess})
				}
			case "add_checklist":
				sid := ensureTargetStep()
				if sid != "" {
					blk := map[string]any{
						"kind": "checklist", "schema": 1,
						"title": "关键核对",
						"items": []map[string]any{{"id": "chk-new", "text": "确认条件与耗材就绪", "checked": false}},
					}
					if next, err := asterdoc.InsertStepBlockInSource(curLD.Source, sid, afterIdx, blk); err == nil {
						applyDoc(next)
						c.AddToast(ui.Toast{Title: "已插入检查清单", Type: components.ToastSuccess})
					}
				}
			case "add_param":
				sid := ensureTargetStep()
				if sid != "" {
					blk := map[string]any{
						"kind": "param_table", "schema": 1,
						"title": "条件参数",
						"rows":  []map[string]any{{"key": "conc", "label": "工作浓度", "value": "10", "unit": "uM", "record_actual": true}},
					}
					if next, err := asterdoc.InsertStepBlockInSource(curLD.Source, sid, afterIdx, blk); err == nil {
						applyDoc(next)
						c.AddToast(ui.Toast{Title: "已插入参数表", Type: components.ToastSuccess})
					}
				}
			case "add_plate":
				sid := ensureTargetStep()
				if sid != "" {
					blk := map[string]any{
						"kind": "plate", "schema": 1,
						"title": "96 孔板布板", "plate_format": 96,
					}
					if next, err := asterdoc.InsertStepBlockInSource(curLD.Source, sid, afterIdx, blk); err == nil {
						applyDoc(next)
						c.AddToast(ui.Toast{Title: "已插入 96 孔板", Type: components.ToastSuccess})
					}
				}
			case "open_timer":
				a.shell.RightTab = 1
				a.shell.MobileDrawer = "outline"
			case "source_mode":
				if curTab.Mode == shell.ModeSource {
					a.shell.SetActiveTabMode(shell.ModeEdit)
				} else {
					a.shell.SetActiveTabMode(shell.ModeSource)
				}
			case "copy_source":
				c.Services().WriteClipboard(curLD.Source)
				c.AddToast(ui.Toast{Title: "已复制 AsterML 原文", Type: components.ToastSuccess})
			case "export_pdf":
				if curLD.Parsed != nil {
					t := docTitle
					if fm2, _, _ := asterdoc.ScanFrontmatter(curLD.Source); strings.TrimSpace(fm2.Title) != "" {
						t = strings.TrimSpace(fm2.Title)
					}
					a.startDocumentPDFExport(c, t, "", "", a.vault.Root, curLD.Parsed)
				}
			case "save_template":
				if tpl, tErr := a.vault.SaveAsTemplate(curTab.FilePath, docTitle); tErr == nil {
					a.refreshVaultTreeAndCalendar()
					c.AddToast(ui.Toast{Title: "已保存为模板：" + tpl.Name, Type: components.ToastSuccess})
				}
			case "duplicate":
				if dup, dErr := a.vault.DuplicateFile(curTab.FilePath, docDate); dErr == nil {
					a.refreshVaultTreeAndCalendar()
					a.shell.OpenFileInPane(pane.ID, dup.RelPath, dup.Title, shell.ModeEdit, true)
				}
			case "delete":
				a.deleteVaultFile(curTab.FilePath)
			}
		}
	}

	pageCol := ui.Column(c.Key("vault-page:"+pane.ID+":"+tab.ID)).Fill()
	theme.PaintCanvasBackground(pageCol, tok)
	pageCol.Children(func() {
		// 1. Clean 1-row Document Header on Desktop/Tablet (collapsed on Mobile so zero duplicate double-headers)
		if !mobileLayout {
			ui.Row(c).FillWidth().Padding(float32(tok.Spacing4)*1.5, float32(tok.Spacing16)).
				Background(tok.Surface.Alpha(0.86)).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
				components.Text(c, folderLabel+" / "+docTitle).
					FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
					SingleLine().Ellipsis("…").Grow(1).MinWidth(0)

				if ld.ParseErr != nil {
					components.Badge(c, "待语法闭合", components.BadgeOptions{Tone: components.BadgeWarning})
				}

				modeInt := int(tab.Mode)
				components.Tabs(c, &modeInt, components.TabsSegmented, "编辑", "阅读", "源码")
				if shell.ViewMode(modeInt) != tab.Mode {
					setTabMode(shell.ViewMode(modeInt))
					tab.Mode = shell.ViewMode(modeInt)
				}

				copiedAt := ui.Local(c.Root(), "doc-copy-ok:"+tab.FilePath, func() time.Time { return time.Time{} })
				justCopied := !copiedAt.IsZero() && time.Since(*copiedAt) < 1500*time.Millisecond
				ico := lucide.Copy()
				if justCopied {
					ico = lucide.Check()
				}
				copyBtn := components.IconButton(c, ico, "复制 Markdown / AsterML 原文", components.IconButtonCompact)
				if justCopied {
					copyBtn.TextColor(tok.Success)
				}
				if copyBtn.Clicked() {
					c.Services().WriteClipboard(ld.Source)
					now := time.Now()
					*copiedAt = now
					if a.shell != nil {
						inv := a.shell.Invalidate
						time.AfterFunc(1550*time.Millisecond, inv)
					}
				}

				moreBtn := components.IconButton(c, lucide.Ellipsis(), "文档操作菜单", components.IconButtonCompact)
				components.MenuButton(c, moreBtn, func(m *components.Menu) {
					if m.Item("导出 PDF…").Chosen() && ld.Parsed != nil {
						a.startDocumentPDFExport(c, docTitle, "", "", a.vault.Root, ld.Parsed)
					}
					if m.Item("导出 Markdown").Chosen() {
						c.Services().WriteClipboard(ld.Source)
						*copiedAt = time.Now()
						if a.shell != nil {
							inv := a.shell.Invalidate
							time.AfterFunc(1550*time.Millisecond, inv)
						}
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
							if a.shell != nil {
								a.shell.OpenFileInPane(pane.ID, dup.RelPath, dup.Title, shell.ModeEdit, true)
							}
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
				ld.ApplySourceDraft(bufText)
				if ld.ParseErr == nil {
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
		v.BaseDir = a.vault.Root
		v.MobileCompact = mobileLayout
		v.PantryLocationMap = a.buildPantryLocationMap()
		if tab.Mode == shell.ModeRead {
			v.Mode = asterdoc.ModeProtocol
			v.ReadOnly = false // Checkboxes and timers stay interactive in Read mode!
		} else {
			v.Mode = asterdoc.ModeDesign
			v.ReadOnly = false
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
		v.OnImportAssetBytes = func(data []byte, origName string) (string, error) {
			return a.vault.ImportAssetBytes(data, origName)
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
			if rel, impErr := a.vault.ImportAssetFile(picked); impErr == nil {
				props := map[string]any{"src": rel}
				if strings.EqualFold(filepath.Ext(picked), ".svg") {
					if raw, rErr := os.ReadFile(picked); rErr == nil && strings.Contains(string(raw), "<svg") {
						props["svg"] = string(raw)
					}
				}
				if next, uErr := asterdoc.UpdateStepBlockInSource(ld.Source, stepID, blockID, props); uErr == nil {
					applyLiveSource(next)
				}
			}
		}
		v.OnSavePlateExport = func(defaultName, format string, data []byte) error {
			if format == "csv" && len(data) > 0 {
				path, sErr := mygo.Dialog.Save(mygo.SaveDialogOptions{
					Parent:      mygo.FocusedWindow(),
					Title:       "导出孔板 CSV 布局",
					DefaultPath: defaultName,
				})
				if sErr == nil && path != "" {
					return os.WriteFile(path, data, 0o644)
				}
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

		// Sync outline steps for focused pane
		if pane.ID == a.shell.ActivePaneID {
			var outline []shell.OutlineItem
			for _, m := range doc.Modules {
				if m.Kind == "step" {
					outline = append(outline, shell.OutlineItem{
						ID:     m.ID,
						Title:  firstNonEmpty(moduleString(m.Data, "title"), m.ID),
						Active: m.ID == a.designSelected,
					})
				}
			}
			a.shell.OutlineSteps = outline
		}

		ui.Box(c).FillWidth().Grow(1).MinHeight(0).Children(func() {
			docScroll := ui.Scroll(c).Fill()
			if mobileLayout && a.shell != nil {
				docScroll.TrackScroll(a.shell.MobileContentScrollState()).
					ScrollbarInsets(a.shell.MobileTopReserve(c), 0, a.shell.MobileBottomReserve(c), 0)
			}
			padTop := float32(tok.Spacing24)
			padH := float32(tok.Spacing24)
			padBot := float32(tok.Spacing48)
			if mobileLayout && a.shell != nil {
				padTop = a.shell.MobileTopReserve(c) + float32(tok.Spacing12)
				padH = float32(tok.Spacing12) + 2
				padBot = a.shell.MobileBottomReserve(c) + float32(tok.Spacing24)
			}
			docScroll.Children(func() {
				ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
					ui.Column(c).FillWidth().MaxWidth(920).
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
									ui.Row(c).Width(float32(tok.Spacing48)*1.8).Shrink(0).AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
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
										Radius(14).Background(tok.Elevated).Gap(float32(tok.Spacing8)).Children(func() {
										propRow(lucide.Calendar(), "date", func() {
											if tab.Mode == shell.ModeEdit {
												dt := asterdoc.BorderlessTextInput(c, tok, dBuf, float32(tok.TextBody), false, "YYYY-MM-DD").
													FontFeatures("tnum")
												if dt.Submitted() || (!dt.Focused() && strings.TrimSpace(*dBuf) != docDate) {
													next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, strings.TrimSpace(*dBuf), docStatus)
													applyLiveSource(next)
												}
											} else {
												ui.Text(c, dateBadge).FontSize(float32(tok.TextBody)).TextColor(tok.Text).FontFeatures("tnum")
											}
										})
										propRow(lucide.Folder(), "project", func() {
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
										propRow(lucide.Tag(), "status", func() {
											sSel := components.Select(c, &sVal)
											asterdoc.StyleSelectQuiet(c, tok, sSel)
											sSel.Trigger.Children(func() {
												ui.Box(c).Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing8)).
													Radius(12).Background(tok.AccentSoftBg).Children(func() {
													ui.Text(c, statusLabels[sVal]).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.Accent)
												})
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
												next := asterdoc.SetFrontmatterMetadata(ld.Source, docTitle, strings.TrimSpace(*dBuf), docStatus)
												applyLiveSource(next)
											}
										})
									} else {
										ui.Text(c, dateBadge).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
									}
								})

								if tab.Mode == shell.ModeEdit {
									pSel := components.Select(c, &folderVal)
									asterdoc.StyleSelectQuiet(c, tok, pSel)
									pSel.Trigger.Children(func() {
										ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
											ui.Icon(c, lucide.Folder()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
											ui.Text(c, folderLabel).FontSize(float32(tok.TextMeta)).TextColor(tok.Text)
										})
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
									ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ui.Icon(c, lucide.Folder()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.TextMuted)
										ui.Text(c, folderLabel).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
									})
								}

								sSel := components.Select(c, &sVal)
								asterdoc.StyleSelectQuiet(c, tok, sSel)
								sSel.Trigger.Children(func() {
									ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
										ui.Box(c).Size(float32(tok.Spacing4)*1.5, float32(tok.Spacing4)*1.5).Radius(float32(tok.ControlRadius)).Background(tok.Accent)
										ui.Text(c, statusLabels[sVal]).FontSize(float32(tok.TextMeta)).TextColor(tok.Accent)
									})
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
							ui.Box(c).FillWidth().Height(1).Background(tok.Border.Alpha(0.45))
						}
						renderPropsFields()

						// Render the single-source document body & step modules
						v.Render(c)

						// Bottom add-step control in Edit mode
						if tab.Mode == shell.ModeEdit {
							ui.Row(c).FillWidth().Justify(ui.Start).Gap(float32(tok.Spacing8)).Children(func() {
								if components.Button(c, "+ 添加实验步骤", components.ButtonGhost).Clicked() {
									n := len(doc.Modules) + 1
									if next, _, err := asterdoc.AddStepInSource(ld.Source, fmt.Sprintf("步骤 %d", n)); err == nil {
										applyLiveSource(next)
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
