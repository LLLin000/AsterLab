package runworkspace

import (
	"fmt"
	"strconv"
	"time"

	"asterlab-mygo-prototype/internal/assays"
	"asterlab-mygo-prototype/internal/resultgrid"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
)

// ViewKind identifies the presentation of the recording module. The data
// source is always the same; the kind only selects how it is drawn.
type ViewKind string

const (
	ViewPlate   ViewKind = "plate"
	ViewTable   ViewKind = "table"
	ViewGallery ViewKind = "gallery"
	ViewForm    ViewKind = "form"
)

// LinkedAsset is a reference to a local file. Encoding never stores an absolute
// path in AsterML; root_id + relative_path is the durable identity.
type LinkedAsset struct {
	AssetID      string
	RootID       string
	RelativePath string
	MediaType    string
	Size         int64
	ModTime      string
	SHA256       string
	Available    bool
	LinkedTo     AssetLink
}

// AssetLink names the experiment object an asset belongs to.
type AssetLink struct {
	RunID    string
	TaskKey  string
	SampleID string
}

// SelectKind chooses the recording view for a task. The grid is the Core-backed
// plate/table record when one exists; a plate schema wins because it is the
// richest record for that task. Otherwise a measurement profile maps to a
// table, and any task with linked files can offer a gallery.
func SelectKind(task *RunTaskViewModel, grid *resultgrid.Grid, assets []LinkedAsset) ViewKind {
	if grid != nil && grid.Rows() > 0 && grid.Columns() > 0 {
		if assays.DefaultViewForSchema(grid.Schema) == "table" {
			return ViewTable
		}
		return ViewPlate
	}
	if task == nil {
		return ViewForm
	}
	if len(assets) > 0 {
		return ViewGallery
	}
	for _, m := range task.Modules {
		if m.Kind == "measurement_profile" {
			return ViewTable
		}
	}
	if len(task.Actual.Measurements) > 0 {
		return ViewTable
	}
	return ViewForm
}

// RecordingView renders the unified recording module: the same data backed by
// plate, table, gallery, or form. The host supplies the write callbacks.
type RecordingView struct {
	RunID       string
	Kind        ViewKind
	Grid        *resultgrid.Grid
	Assets      []LinkedAsset
	Task        *RunTaskViewModel
	OnOpenAsset func(asset LinkedAsset)
	OnRelocate  func(asset LinkedAsset)
	OnLinkFile  func(asset LinkedAsset)
	// LinkErr holds the Core rejection cause for the last link attempt on this
	// task so the recording surface can show cause + retry (UX-P5).
	LinkErr      string
	OnKindChange func(kind ViewKind)
	OnSaveBatch  func(cells []resultgrid.Cell)
}

// Render draws the selected view plus a compact view switcher and the single
// Core write action. Save lives in the header so it stays visible while the
// plate or table is scrolled.
func (r *RecordingView) Render(c *ui.Context) {
	if r == nil || r.Task == nil {
		return
	}
	t := c.Theme()
	tokens := theme.GetTokens(t)
	recovered := syncGridDraft(r.RunID, r.Task.TaskKey, r.Grid, time.Now())
	saveNow := func() {
		if r.Grid == nil || r.OnSaveBatch == nil {
			return
		}
		if err := r.Grid.CommitPendingEdit(); err != nil {
			r.Grid.Error = err.Error()
			return
		}
		r.Grid.Error = ""
		r.OnSaveBatch(r.Grid.Cells)
	}
	if r.Grid != nil && r.OnSaveBatch != nil {
		c.OnShortcut(ui.Cmd, ui.KeyS, saveNow)
	}
	views := r.availableViews()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		for _, v := range views {
			v := v
			label := viewLabel(v)
			b := ui.ButtonBase(c).Padding(3, 10).Radius(5)
			if v == r.Kind {
				b.Background(t.Accent)
				b.Children(func() {
					ui.Text(c, label).FontSize(t.Rem(.76)).Bold().TextColor(t.AccentText)
				})
			} else {
				b.Children(func() {
					ui.Text(c, label).FontSize(t.Rem(.76)).TextColor(t.TextMuted)
				})
			}
			if b.Clicked() {
				blocked := false
				if r.Grid != nil {
					if err := r.Grid.CommitPendingEdit(); err != nil {
						r.Grid.Error = err.Error()
						blocked = true
					}
				}
				if !blocked {
					r.Kind = v
					if r.OnKindChange != nil {
						r.OnKindChange(v)
					}
				}
			}
		}
		if r.Grid != nil && r.Grid.State == resultgrid.StateSaved {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "已保存").FontSize(t.Rem(.72)).Bold().TextColor(tokens.SuccessText)
			})
		} else if r.Grid != nil && r.OnSaveBatch != nil {
			// UX-STATE, UX-P5: make unsaved and retryable failure states explicit beside Save.
			if r.Grid.State == resultgrid.StateError {
				ui.Text(c, "保存失败").FontSize(t.Rem(.72)).Bold().TextColor(tokens.DangerText)
			} else {
				ui.Text(c, "未保存").FontSize(t.Rem(.72)).Bold().TextColor(tokens.WarningText)
			}
			saveLabel := "保存读数"
			if r.Grid.State == resultgrid.StateError {
				saveLabel = "重试保存"
			}
			save := components.Button(c, saveLabel, components.ButtonPrimary)
			save.Disabled(r.Grid.State == resultgrid.StateSaving)
			if r.Grid.State == resultgrid.StateSaving {
				ui.Text(c, "保存中…").FontSize(t.Rem(.72)).TextColor(t.TextMuted)
			}
			if save.Clicked() {
				saveNow()
			}
		}
	})
	if r.Grid != nil && r.Grid.State == resultgrid.StateError && r.Grid.Error != "" {
		// UX-P5 / UX-STATE / UX-DONE: retain the Core rejection cause and give a concrete recovery action.
		ui.Column(c).FillWidth().Padding(t.Space(.75), t.Space(1.25)).Gap(t.Space(.5)).
			Background(tokens.DangerBg).Children(func() {
			ui.Text(c, "未保存 · "+r.Grid.Error).FontSize(t.Rem(.76)).TextColor(tokens.DangerText)
			ui.Text(c, "检查目标目录权限后，点击重试保存。").
				FontSize(t.Rem(.72)).TextColor(tokens.DangerText)
		})
	}

	if recovered && r.Grid != nil && r.Grid.State != resultgrid.StateSaved {
		ui.Row(c).FillWidth().Padding(t.Space(.75), t.Space(1.25)).Radius(6).
			Background(tokens.WarningBg).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
			ui.Text(c, "已从上次未保存草稿恢复（Ctrl+S 保存到 Core）").FontSize(t.Rem(.72)).TextColor(tokens.WarningText)
			ui.Spacer(c)
			if components.Button(c, "丢弃草稿", components.ButtonGhost).Clicked() {
				discardGridDraft(r.RunID, r.Task.TaskKey, r.Grid)
			}
		})
	}
	switch r.Kind {
	case ViewPlate:
		r.renderPlate(c)
	case ViewTable:
		r.renderTable(c)
	case ViewGallery:
		r.renderGallery(c)
	default:
		r.renderForm(c)
	}
}

func (r *RecordingView) availableViews() []ViewKind {
	out := []ViewKind{}
	if r.Grid != nil && r.Grid.Rows() > 0 && r.Grid.Columns() > 0 {
		if assays.DefaultViewForSchema(r.Grid.Schema) == "table" {
			out = append(out, ViewTable, ViewPlate)
		} else {
			out = append(out, ViewPlate, ViewTable)
		}
	} else if len(r.Task.Actual.Measurements) > 0 {
		out = append(out, ViewTable)
	}
	if len(r.Assets) > 0 || r.OnLinkFile != nil {
		out = append(out, ViewGallery)
	}
	if len(out) == 0 {
		out = append(out, ViewForm)
	}
	return out
}

func viewLabel(kind ViewKind) string {
	switch kind {
	case ViewPlate:
		return "孔板"
	case ViewTable:
		return "表格"
	case ViewGallery:
		return "图片与文件"
	default:
		return "表单"
	}
}

// renderPlate draws the plate view fed by the result grid.
func (r *RecordingView) renderPlate(c *ui.Context) {
	if r.Grid == nil {
		ui.Text(c, "当前任务没有孔板记录 schema。").FontSize(12).TextColor(c.Theme().TextMuted)
		return
	}
	ui.Column(c).Key("recording-plate").FillWidth().Children(func() {
		resultgrid.PlateView(c, r.Grid)
	})
}

// renderTable draws a table projection of the same Core-backed grid. It never
// becomes a second authoritative dataset: identity and values come from Grid.
func (r *RecordingView) renderTable(c *ui.Context) {
	t := c.Theme()
	if r.Grid == nil {
		if len(r.Task.Actual.Measurements) == 0 {
			ui.Text(c, "尚无已记录的实际读数。").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Grid(c).Columns(4).Gap(8).Children(func() {
			ui.Text(c, "样本").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Text(c, "字段").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Text(c, "数值").FontSize(11).Bold().TextColor(t.TextMuted)
			ui.Text(c, "状态").FontSize(11).Bold().TextColor(t.TextMuted)
			for _, m := range r.Task.Actual.Measurements {
				ui.Text(c, m.SampleID).FontSize(12).TextColor(t.Text)
				ui.Text(c, m.Field).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, m.Value+" "+m.Unit).FontSize(12).FontFeatures("tnum")
				status := m.Status
				if status == "" {
					status = "raw"
				}
				ui.Text(c, status).FontSize(11).TextColor(t.TextMuted)
			}
		})
		return
	}
	g := r.Grid
	// UX-P1, UX-P3, UX-P4, UX-P5, UX-LAYOUT: keep the themed record table aligned and readable at both breakpoints.
	ui.Scroll(c).Key("recording-table").Fill().Children(func() {
		ui.Column(c).FillWidth().Gap(t.Space(.75)).Children(func() {
			posHeader := "孔位"
			if g.Schema.Layout != "" && g.Schema.Layout != "plate96" {
				posHeader = "位置"
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Padding(t.Space(.5), t.Space(1)).Children(func() {
				ui.Text(c, posHeader+" / 样本").Grow(1).MinWidth(0).FontSize(11).Bold().TextColor(t.TextMuted)
				ui.Text(c, "字段").Width(68).FontSize(11).Bold().TextColor(t.TextMuted)
				ui.Text(c, "数值").Width(88).FontSize(11).Bold().TextColor(t.TextMuted)
				ui.Text(c, "状态").Width(44).FontSize(11).Bold().TextColor(t.TextMuted)
			})
			for _, cell := range g.Cells {
				cell := cell
				rr, cc := cell.Row, cell.Column
				well := ""
				if rr < len(g.Schema.RowLabels) && cc < len(g.Schema.ColumnLabels) {
					if g.Schema.Layout == "" || g.Schema.Layout == "plate96" {
						well = g.Schema.RowLabels[rr] + g.Schema.ColumnLabels[cc]
					} else {
						well = g.Schema.RowLabels[rr] + " · " + g.Schema.ColumnLabels[cc]
					}
				}
				input, ok := g.InputAt(rr, cc)
				if !ok {
					continue
				}
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).
					Padding(t.Space(.75), t.Space(1)).Radius(6).
					Background(t.Surface).Border(1, t.Border).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, well).FontSize(12).Bold().TextColor(t.Text).SingleLine().Ellipsis("…")
						meta := cell.SampleID
						if cell.Group != "" && cell.Group != cell.SampleID {
							meta += " · " + cell.Group
						}
						meta += fmt.Sprintf(" · 重复 %d", cell.Replicate)
						ui.Text(c, meta).FontSize(10).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
					})
					ui.Text(c, cell.Field).Width(68).FontSize(11).TextColor(t.TextMuted).SingleLine().Ellipsis("…")
					e := ui.TextInput(c, input).Width(88).Padding(4).Placeholder("—").Label(well+" 读数").
						FocusBind(&g.FocusInput, well)
					if well == g.FocusInput {
						// Focus follows the same well when a breakpoint rebuilds
						// the field (UX-03).
						e.AutoFocus()
					}
					if e.Changed() {
						if *input == "" {
							cell.Value, cell.Status = nil, "missing"
							if err := g.SetCell(cell); err != nil {
								g.Error = err.Error()
							}
						} else if v, err := strconv.ParseFloat(*input, 64); err != nil {
							g.Error = "请输入数字"
							g.State = resultgrid.StateError
						} else {
							cell.Value, cell.Status = &v, "raw"
							if err := g.SetCell(cell); err != nil {
								g.Error = err.Error()
							}
						}
					}
					status := cell.Status
					if status == "" {
						status = "missing"
					}
					ui.Text(c, status).Width(44).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
	})
	if g.Error != "" {
		ui.Text(c, g.Error).FontSize(t.Rem(.75)).TextColor(t.Danger)
	}
}

// renderGallery draws a file/image gallery plus the measured indicators.
func (r *RecordingView) renderGallery(c *ui.Context) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	ui.Column(c).Key("recording-gallery").Gap(t.Space(2)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "原始图片与文件").FontSize(12).Bold().TextColor(t.TextMuted)
			if len(r.Assets) > 0 {
				ui.Text(c, fmt.Sprintf("%d 个文件 · 本地引用", len(r.Assets))).
					FontSize(11).TextColor(t.TextMuted)
			}
			if r.OnLinkFile != nil && components.Button(c, "关联本地文件", components.ButtonSecondary).Clicked() {
				r.OnLinkFile(LinkedAsset{LinkedTo: AssetLink{TaskKey: r.Task.TaskKey}})
			}
		})
		if len(r.Assets) == 0 {
			ui.Text(c, "尚未关联本地文件。").FontSize(12).TextColor(t.TextMuted)
		}
		if r.LinkErr != "" {
			ui.Row(c).Padding(6, 10).Radius(6).
				Background(tokens.DangerBg).Border(1, tokens.DangerText).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
				ui.Text(c, "关联失败："+r.LinkErr).FontSize(t.Rem(.72)).Bold().TextColor(tokens.DangerText)
				ui.Spacer(c)
				retry := components.Button(c, "重新关联", components.ButtonPrimary)
				if retry.Clicked() && r.OnLinkFile != nil {
					r.OnLinkFile(LinkedAsset{LinkedTo: AssetLink{TaskKey: r.Task.TaskKey}})
				}
			})
		}
		for _, a := range r.Assets {
			assetRow(c, a, r.OnOpenAsset, r.OnRelocate, t, tokens)
		}
		if len(r.Task.Actual.Measurements) > 0 {
			ui.Divider(c)
			ui.Text(c, "定量指标").FontSize(12).Bold().TextColor(t.TextMuted)
			r.renderTable(c)
		}
	})
}

func assetRow(c *ui.Context, a LinkedAsset, open, relocate func(LinkedAsset), t *ui.Theme, tokens theme.Tokens) {
	ui.Row(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
		Background(tokens.Elevated).Border(1, tokens.Border).
		Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
		name := a.RelativePath
		if name == "" {
			name = a.AssetID
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, name).FontSize(t.Rem(.8)).FontWeight(600).TextColor(t.Text).
				SingleLine().Ellipsis("…")
			meta := a.MediaType
			if a.Size > 0 {
				meta += " · " + humanSize(a.Size)
			}
			if meta != "" {
				ui.Text(c, meta).FontSize(t.Rem(.7)).TextColor(t.TextMuted).
					SingleLine().Ellipsis("…")
			}
		})
		if !a.Available {
			// UX-P5 / UX-STATE: keep a missing asset link visible and offer relocation.
			ui.Row(c).Padding(2, 7).Radius(5).Background(tokens.WarningBg).Children(func() {
				ui.Text(c, "未解析 · 文件不可用").FontSize(t.Rem(.68)).Bold().TextColor(tokens.WarningText)
			})
			if relocate != nil {
				if components.Button(c, "重新定位", components.ButtonGhost).Clicked() {
					relocate(a)
				}
			}
			return
		}
		if open != nil {
			if components.Button(c, "打开", components.ButtonGhost).Clicked() {
				open(a)
			}
		}
	})
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/float64(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// renderForm draws the actual-parameter form, comparing planned vs actual.
func (r *RecordingView) renderForm(c *ui.Context) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	planned := r.Task.Planned
	if planned == nil || len(planned.Parameters) == 0 {
		ui.Text(c, "此步骤没有计划参数。").FontSize(12).TextColor(t.TextMuted)
		return
	}
	ui.Column(c).Key("recording-form").Gap(t.Space(1.5)).Children(func() {
		ui.Text(c, "计划与实际参数").FontSize(12).Bold().TextColor(t.TextMuted)
		for _, p := range planned.Parameters {
			p := p
			actual, hasActual := r.Task.Actual.Parameters[p.Key]
			plannedText := p.Value
			if p.Unit != "" {
				plannedText += " " + p.Unit
			}
			ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, p.Label).FontSize(t.Rem(.8)).TextColor(t.TextMuted).Width(120)
				ui.Text(c, plannedText).FontSize(t.Rem(.82)).FontWeight(600).TextColor(t.Text)
				if hasActual && actual != plannedText {
					ui.Row(c).Padding(2, 7).Radius(5).Background(tokens.WarningBg).Children(func() {
						ui.Text(c, "实际 "+actual).FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
					})
				}
			})
		}
	})
}
