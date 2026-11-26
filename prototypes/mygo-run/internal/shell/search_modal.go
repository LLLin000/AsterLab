package shell

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/internal/uikit/kit"
	"asterlab-mygo-prototype/internal/vault"
)

func (s *Shell) refreshGlobalSearch() {
	q := strings.TrimSpace(s.GlobalSearchQuery)
	s.GlobalSearchLastQuery = s.GlobalSearchQuery
	s.GlobalSearchCursor = 0
	if q == "" {
		s.GlobalSearchHits = nil
		return
	}
	if s.OnGlobalSearch != nil {
		s.GlobalSearchHits = s.OnGlobalSearch(q)
		return
	}
	lq := strings.ToLower(q)
	var hits []vault.SearchHit
	for _, exp := range s.resolvedExperiments() {
		if strings.Contains(strings.ToLower(exp.Title+" "+exp.Summary+" "+exp.FilePath), lq) {
			hits = append(hits, vault.SearchHit{
				RelPath: exp.FilePath,
				Title:   exp.Title,
				Folder:  exp.Folder,
				Snippet: firstNonEmptyStr(exp.Summary, string(exp.Stage)),
				Date:    exp.Date,
				Status:  string(exp.Stage),
			})
		}
	}
	s.GlobalSearchHits = hits
}

func (s *Shell) openGlobalSearchHit(hit vault.SearchHit) {
	q := strings.TrimSpace(s.GlobalSearchQuery)
	if s.OnOpenSearchHit != nil {
		s.OnOpenSearchHit(hit.RelPath, q)
		return
	}
	for _, exp := range s.resolvedExperiments() {
		if exp.FilePath == hit.RelPath {
			s.openExperiment(exp)
			return
		}
	}
}

func (s *Shell) renderGlobalSearchModal(c *ui.Context) {
	if !s.GlobalSearchOpen {
		ui.Box(c.Key("global-search-dlg")).Children(func() {
			components.Dialog(c, &s.GlobalSearchOpen, components.DialogOptions{Label: "Vault 全局全文搜索"}, nil)
		})
		return
	}
	if s.GlobalSearchQuery != s.GlobalSearchLastQuery {
		s.refreshGlobalSearch()
	}
	tok := theme.GetTokens(c.Theme())
	hits := s.GlobalSearchHits
	if len(hits) == 0 {
		s.GlobalSearchCursor = 0
	} else {
		if s.GlobalSearchCursor < 0 {
			s.GlobalSearchCursor = 0
		}
		if s.GlobalSearchCursor >= len(hits) {
			s.GlobalSearchCursor = len(hits) - 1
		}
	}
	if s.IsMobileLayout(c) {
		s.renderMobileGlobalSearchPage(c, tok, hits)
		return
	}
	open := s.GlobalSearchOpen

	ui.Box(c.Key("global-search-dlg")).Children(func() {
		components.Dialog(c, &open, components.DialogOptions{Label: "Vault 全局全文搜索"}, func(panel ui.Element) {
			panel.Width(float32(tok.Spacing48)*12.5).Padding(float32(tok.Spacing16)).Gap(float32(tok.Spacing8)).
				MaxHeight(float32(tok.Spacing48) * 12.0)
			kit.ApplyElevation(panel, tok, kit.ElevationModal)
			panel.Children(func() {
				if len(hits) > 0 {
					if panel.Shortcut(0, ui.KeyDown) {
						s.GlobalSearchCursor = (s.GlobalSearchCursor + 1) % len(hits)
					}
					if panel.Shortcut(0, ui.KeyUp) {
						s.GlobalSearchCursor = (s.GlobalSearchCursor - 1 + len(hits)) % len(hits)
					}
					if panel.Shortcut(0, ui.KeyEnter) {
						s.openGlobalSearchHit(hits[s.GlobalSearchCursor])
						open = false
						return
					}
				}
				if panel.Shortcut(0, ui.KeyEscape) {
					open = false
					return
				}

				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
					ui.Icon(c, lucide.FileSearch()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
					inp := ui.TextInputBase(c, &s.GlobalSearchQuery).FillWidth().FontSize(float32(tok.TextBody)).TextColor(tok.Text).
						Padding(float32(tok.Spacing4)*0.5, float32(tok.Spacing4)).
						Placeholder("全文检索所有 .asterml 实验步骤、分组、试剂编号或笔记片段…").
						Grow(1).MinWidth(0)
					inp.AutoFocus()
					if strings.TrimSpace(s.GlobalSearchQuery) != "" {
						components.Text(c, fmt.Sprintf("%d 条命中", len(hits))).
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
					}
				})

				ui.Divider(c)

				if strings.TrimSpace(s.GlobalSearchQuery) == "" {
					ui.Column(c).FillWidth().Padding(float32(tok.Spacing24)).Center().Gap(float32(tok.Spacing4)).Children(func() {
						components.Text(c, "输入关键词跨 Vault 全文检索").FontSize(float32(tok.TextBody)).Bold().TextColor(tok.TextMuted)
						components.Text(c, "支持搜索步骤标题、孔板配方、Pantry 编号与实验笔记；选中后直接打开并高亮匹配行。").
							FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
					})
					return
				}

				if len(hits) == 0 {
					kit.EmptyState(c, tok, lucide.FileSearch(), "未找到包含该关键词的实验文档", "尝试更换试剂名、细胞系名称或实验日期。", "", nil)
					return
				}

				ui.Scroll(c).FillWidth().MaxHeight(float32(tok.Spacing48) * 9.0).Children(func() {
					ui.Column(c).FillWidth().Gap(float32(tok.Spacing4)).Children(func() {
						for i, hit := range hits {
							hit := hit
							selected := i == s.GlobalSearchCursor
							targetBg := ui.Transparent
							if selected {
								targetBg = tok.AccentSoftBg
							}
							row := ui.ButtonBase(c.Key(fmt.Sprintf("gsearch-%d-%d-%s", i, hit.LineNum, hit.RelPath))).
								Label(firstNonEmptyStr(hit.Title, hit.RelPath)).
								FillWidth().Justify(ui.Start).
								Padding(float32(tok.Spacing8), float32(tok.Spacing8)).
								Radius(float32(tok.ControlRadius))
							if row.Hovered() && !selected {
								targetBg = tok.Elevated
							}
							row.Background(row.AnimateColor("gs-bg", targetBg, theme.MotionMicroFade))
							theme.ApplyTactilePress(row)
							row.Children(func() {
								ui.Row(c).FillWidth().Gap(float32(tok.Spacing8)).AlignItems(ui.Center).Children(func() {
									ui.Icon(c, lucide.FileText()).Size(float32(tok.Spacing16), float32(tok.Spacing16)).TextColor(tok.Accent)
									ui.Column(c).Grow(1).MinWidth(0).Gap(float32(tok.Spacing4) * 0.5).Children(func() {
										ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(float32(tok.Spacing8)).Children(func() {
											components.Text(c, firstNonEmptyStr(hit.Title, hit.RelPath)).
												FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).SingleLine().Ellipsis("…")
											if hit.Date != "" {
												components.Text(c, hit.Date).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).FontFeatures("tnum")
											}
										})
										snip := hit.Snippet
										if hit.LineNum > 0 {
											snip = fmt.Sprintf("L%d: %s", hit.LineNum, snip)
										}
										components.Text(c, snip).FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine().Ellipsis("…")
									})
								})
							})
							if row.Clicked() {
								s.openGlobalSearchHit(hit)
								open = false
							}
						}
					})
				})
			})
		})
	})
	s.GlobalSearchOpen = open
}

func (s *Shell) renderMobileGlobalSearchPage(c *ui.Context, tok theme.Tokens, hits []vault.SearchHit) {
	ui.Overlay(c, func() {
		t := c.Theme()
		topSafe, rightSafe, bottomSafe, leftSafe := c.SafeArea()
	ui.Column(c.Key("mb-gsearch-root")).Absolute().Fill().Modal().Background(tok.Canvas).Children(func() {
		// Top Obsidian Search Nav Bar: Search Field + Cancel
		ui.Row(c).FillWidth().
			Padding(t.Space(2)+topSafe, t.Space(4)+rightSafe, t.Space(2), t.Space(4)+leftSafe).
			MinHeight(t.Space(12)+topSafe).
			AlignItems(ui.Center).Gap(t.Space(2)).Background(tok.Surface).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).
				Padding(t.Space(1), t.Space(3)).
				Radius(float32(tok.ControlRadius)).Background(tok.Elevated).Border(1, tok.Border).
				AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
				ui.Icon(c, lucide.FileSearch()).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(tok.Accent).Shrink(0)
				inp := ui.TextInputBase(c.Key("mb-gsearch-inp"), &s.GlobalSearchQuery).
					Grow(1).MinWidth(0).
					FontSize(float32(tok.TextBody)).TextColor(tok.Text).
					Placeholder("搜索全部实验步骤、配方或笔记…")
				inp.AutoFocus()
				if len(hits) > 0 && inp.Shortcut(0, ui.KeyEnter) {
					s.openGlobalSearchHit(hits[0])
					s.GlobalSearchOpen = false
					return
				}
			})
			if components.Button(c.Key("mb-gsearch-cancel"), "取消", components.ButtonGhost).Shrink(0).Clicked() {
				s.GlobalSearchOpen = false
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)

		ui.Scroll(c.Key("mb-gsearch-scroll")).FillWidth().Grow(1).Basis(0).MinHeight(0).Clip().Children(func() {
			ui.Column(c).FillWidth().
				Padding(t.Space(3), t.Space(4)+rightSafe, t.Space(8)+bottomSafe, t.Space(4)+leftSafe).
				Gap(t.Space(3)).Children(func() {
				if strings.TrimSpace(s.GlobalSearchQuery) == "" {
					kit.EmptyState(c, tok, lucide.FileSearch(), "全库全文检索", "输入关键词搜索步骤标题、孔板配方、试剂编号或实验笔记。", "", nil)
					return
				}
				if len(hits) == 0 {
					kit.EmptyState(c, tok, lucide.FileSearch(), "未找到匹配文档", "尝试更换试剂名、细胞系名称或实验日期。", "", nil)
					return
				}
				ui.Text(c, fmt.Sprintf("共 %d 条命中", len(hits))).
					FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted).FontFeatures("tnum")
				ui.Column(c).FillWidth().Radius(float32(tok.PanelRadius)).
					Background(tok.Surface).Border(1, tok.Border).Clip().Children(func() {
					for i, hit := range hits {
						hit := hit
						if i > 0 {
							ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
						}
						row := ui.ButtonBase(c.Key(fmt.Sprintf("mb-gs-%d-%d-%s", i, hit.LineNum, hit.RelPath))).
							Label(firstNonEmptyStr(hit.Title, hit.RelPath)).
							FillWidth().MinHeight(float32(tok.Spacing48)).
							Padding(t.Space(2), t.Space(3)).
							Background(tok.Surface).Justify(ui.Start).
							OnClick(func() {
								s.openGlobalSearchHit(hit)
								s.GlobalSearchOpen = false
							})
						theme.ApplyTactilePress(row)
						row.Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
								kit.IconBox(c, tok, lucide.FileText(), tok.Accent)
								ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(1)).Children(func() {
									ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
										components.Text(c, firstNonEmptyStr(hit.Title, hit.RelPath)).
											FontSize(float32(tok.TextBody)).Bold().TextColor(tok.Text).
											Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
										if hit.Date != "" {
											components.Text(c, hit.Date).
												FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
												FontFeatures("tnum").Shrink(0)
										}
									})
									snip := hit.Snippet
									if hit.LineNum > 0 {
										snip = fmt.Sprintf("L%d: %s", hit.LineNum, snip)
									}
									components.Text(c, snip).
										FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).
										SingleLine().Ellipsis("…")
								})
							})
						})
					}
				})
			})
		})
	})
	})
}
