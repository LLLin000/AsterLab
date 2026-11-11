package runworkspace

import (
	"fmt"

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
	Kind   ViewKind
	Grid   *resultgrid.Grid
	Assets []LinkedAsset
	Task   *RunTaskViewModel

	OnOpenAsset  func(asset LinkedAsset)
	OnRelocate   func(asset LinkedAsset)
	OnLinkFile   func(asset LinkedAsset)
	OnSaveBatch  func(cells []resultgrid.Cell)
}

// Render draws the selected view plus a compact view switcher.
func (r *RecordingView) Render(c *ui.Context) {
	if r == nil || r.Task == nil {
		return
	}
	t := c.Theme()

	views := r.availableViews()
	if len(views) > 1 {
		ui.Row(c).Gap(t.Space(1)).Children(func() {
			current := r.Kind
			for _, v := range views {
				v := v
				label := viewLabel(v)
				b := ui.ButtonBase(c).Padding(3, 10).Radius(5)
				if v == current {
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
					r.Kind = v
				}
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
		out = append(out, ViewPlate, ViewTable)
	} else if len(r.Task.Actual.Measurements) > 0 {
		out = append(out, ViewTable)
	}
	if len(r.Assets) > 0 {
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
	ui.Text(c, fmt.Sprintf("%s · %d × %d · 冻结设计",
		r.Grid.Schema.ID, r.Grid.Rows(), r.Grid.Columns())).
		FontSize(11).TextColor(c.Theme().TextMuted)
	resultgrid.View(c, r.Grid, resultgrid.Options{FullWidth: true})
}

// renderTable draws a rectangular table view (bulk paste supported by grid).
func (r *RecordingView) renderTable(c *ui.Context) {
	t := c.Theme()
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
}

// renderGallery draws a file/image gallery plus the measured indicators.
func (r *RecordingView) renderGallery(c *ui.Context) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	ui.Column(c).Gap(t.Space(2)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "原始图片与文件").FontSize(12).Bold().TextColor(t.TextMuted)
			if len(r.Assets) > 0 {
				ui.Text(c, fmt.Sprintf("%d 个文件 · 本地引用", len(r.Assets))).
					FontSize(11).TextColor(t.TextMuted)
			}
		})
		if len(r.Assets) == 0 {
			ui.Text(c, "尚未关联本地文件。").FontSize(12).TextColor(t.TextMuted)
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
			ui.Row(c).Padding(2, 7).Radius(5).Background(tokens.WarningBg).Children(func() {
				ui.Text(c, "文件不可用").FontSize(t.Rem(.68)).Bold().TextColor(tokens.WarningText)
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
	ui.Column(c).Gap(t.Space(1.5)).Children(func() {
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
