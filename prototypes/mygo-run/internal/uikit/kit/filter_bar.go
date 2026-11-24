package kit

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// FilterOption defines one selectable option within a FilterField.
type FilterOption struct {
	Value string
	Label string
}

// FilterField defines a filterable dimension in a Twenty/Plane-style FilterBar.
type FilterField struct {
	Key     string
	Label   string
	Options []FilterOption
}

// ActiveFilter represents one active Field=Value pill filter.
type ActiveFilter struct {
	FieldKey string
	Value    string
}

// SavedFilterView defines a reusable quick-view preset (e.g., "SPF 屏障", "密度预警").
type SavedFilterView struct {
	ID      string
	Label   string
	Filters []ActiveFilter
}

// FilterBarState holds search text, active multi-condition pills, and popover UI state.
type FilterBarState struct {
	Query       string
	Active      []ActiveFilter
	SavedViewID string
	pickerOpen  bool
	pickerField string
}

// HasFilter reports whether fieldKey=value is currently active.
func (s *FilterBarState) HasFilter(fieldKey, value string) bool {
	if s == nil {
		return false
	}
	for _, f := range s.Active {
		if f.FieldKey == fieldKey && f.Value == value {
			return true
		}
	}
	return false
}

// ToggleFilter adds or removes a Field=Value condition pill (clearing active SavedViewID).
func (s *FilterBarState) ToggleFilter(fieldKey, value string) {
	if s == nil {
		return
	}
	s.SavedViewID = ""
	for i, f := range s.Active {
		if f.FieldKey == fieldKey && f.Value == value {
			s.Active = append(s.Active[:i], s.Active[i+1:]...)
			return
		}
	}
	s.Active = append(s.Active, ActiveFilter{FieldKey: fieldKey, Value: value})
}

// ClearFilters removes all active condition pills and saved view selection.
func (s *FilterBarState) ClearFilters() {
	if s == nil {
		return
	}
	s.Active = nil
	s.SavedViewID = ""
}

// ApplySavedView installs a saved quick view's filter list onto FilterBarState.
func (s *FilterBarState) ApplySavedView(v SavedFilterView) {
	if s == nil {
		return
	}
	s.SavedViewID = v.ID
	if len(v.Filters) == 0 {
		s.Active = nil
		return
	}
	cp := make([]ActiveFilter, len(v.Filters))
	copy(cp, v.Filters)
	s.Active = cp
}

func resolveFilterLabels(fields []FilterField, f ActiveFilter) (fieldLbl, optLbl string) {
	fieldLbl, optLbl = f.FieldKey, f.Value
	for _, fd := range fields {
		if fd.Key != f.FieldKey {
			continue
		}
		fieldLbl = fd.Label
		for _, opt := range fd.Options {
			if opt.Value == f.Value {
				optLbl = opt.Label
				break
			}
		}
		break
	}
	return fieldLbl, optLbl
}

// FilterBar renders a composable Twenty/Plane-style search + saved quick views +
// removable multi-condition filter pills bar.
func FilterBar(c *ui.Context, key any, tok theme.Tokens, st *FilterBarState, fields []FilterField, savedViews []SavedFilterView, searchPlaceholder string) {
	if c == nil || st == nil {
		return
	}
	t := c.Theme()

	ui.Column(c.Key(key)).FillWidth().Gap(t.Space(2)).Children(func() {
		ui.Row(c).FillWidth().Wrap().Padding(t.Space(2), t.Space(3)).
			Radius(float32(tok.PanelRadius)).Background(tok.Elevated).Border(1, tok.Border).
			AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			ui.Box(c).Width(float32(tok.Spacing48) * 5.2).Children(func() {
				components.TextBox(c, &st.Query, components.TextBoxOptions{
					LeadingIcon: lucide.Search(),
				}).Placeholder(searchPlaceholder)
			})

			if len(savedViews) > 0 {
				ui.Box(c).Width(1).Height(t.Space(4)).Background(tok.BorderStrong)
				for _, sv := range savedViews {
					sv := sv
					active := st.SavedViewID == sv.ID || (st.SavedViewID == "" && len(sv.Filters) == 0 && len(st.Active) == 0)
					variant := components.ButtonGhost
					if active {
						variant = components.ButtonToolbar
					}
					if components.Button(c, sv.Label, variant).FontFeatures("tnum").Clicked() {
						st.ApplySavedView(sv)
					}
				}
			}

			if len(fields) > 0 {
				addBtn := components.Button(c, "+ 筛选", components.ButtonSecondary)
				if addBtn.Clicked() {
					st.pickerOpen = !st.pickerOpen
					if st.pickerField == "" {
						st.pickerField = fields[0].Key
					}
				}
				components.Popover(c, addBtn, &st.pickerOpen, func(panel ui.Element) {
					panel.Column().Width(float32(tok.Spacing48)*6.8).Padding(float32(tok.Spacing8)).Gap(float32(tok.Spacing8))
					ApplyElevation(panel, tok, ElevationFloating)
					panel.Children(func() {
						ui.Text(c, "组合条件筛选").FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.TextMuted)
						ui.Row(c).FillWidth().Wrap().Gap(float32(tok.Spacing4)).Children(func() {
							for _, fd := range fields {
								fd := fd
								var variant components.ButtonVariant = components.ButtonGhost
								if st.pickerField == fd.Key {
									variant = components.ButtonToolbar
								}
								if components.Button(c, fd.Label, variant).Clicked() {
									st.pickerField = fd.Key
								}
							}
						})
						ui.Box(c).FillWidth().Height(1).Background(tok.BorderSubtle)
						for _, fd := range fields {
							if fd.Key != st.pickerField {
								continue
							}
							ui.Column(c).FillWidth().Gap(2).Children(func() {
								for _, opt := range fd.Options {
									opt := opt
									on := st.HasFilter(fd.Key, opt.Value)
									ListItem(c, fmt.Sprintf("fopt-%s-%s", fd.Key, opt.Value), tok, ListItemOptions{
										Title:    opt.Label,
										Selected: on,
										StartSlot: func() {
											ico := lucide.Circle()
											col := tok.TextMuted
											if on {
												ico = lucide.CircleCheck()
												col = tok.Accent
											}
											ui.Icon(c, ico).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(col)
										},
										OnClick: func() {
											st.ToggleFilter(fd.Key, opt.Value)
										},
									})
								}
							})
						}
					})
				})
			}

			if len(st.Active) > 0 || st.Query != "" {
				if components.Button(c, "清空筛选", components.ButtonGhost).Clicked() {
					st.Query = ""
					st.ClearFilters()
				}
			}
		})

		if len(st.Active) > 0 {
			ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
				ui.Text(c, "已启用条件：").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted)
				for idx, f := range st.Active {
					idx, f := idx, f
					fLbl, oLbl := resolveFilterLabels(fields, f)
					chip := ui.ButtonBase(c.Key(fmt.Sprintf("fchip-%d-%s-%s", idx, f.FieldKey, f.Value))).
						Label(fmt.Sprintf("%s: %s", fLbl, oLbl)).
						Padding(float32(tok.Spacing4), float32(tok.Spacing8)).
						Radius(float32(tok.ControlRadius)).
						Background(tok.AccentSoftBg).
						Border(1, tok.Accent.Alpha(0.45))
					chip.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(float32(tok.Spacing4)).Children(func() {
							ui.Text(c, fLbl+":").FontSize(float32(tok.TextMeta)).TextColor(tok.TextMuted).SingleLine()
							ui.Text(c, oLbl).FontSize(float32(tok.TextMeta)).Bold().TextColor(tok.AccentText).SingleLine()
							ui.Icon(c, lucide.X()).Size(float32(tok.Spacing12), float32(tok.Spacing12)).TextColor(tok.AccentText)
						})
					})
					if chip.Clicked() {
						st.ToggleFilter(f.FieldKey, f.Value)
					}
				}
			})
		}
	})
}
