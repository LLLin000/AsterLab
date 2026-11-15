package runworkspace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	WellID       string
	Timepoint    string
	FOVIndex     uint32
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
	RunID           string
	Kind            ViewKind
	Grid            *resultgrid.Grid
	Assets          []LinkedAsset
	Task            *RunTaskViewModel
	OnOpenAsset     func(asset LinkedAsset)
	OnRelocate      func(asset LinkedAsset)
	OnRelabelAsset  func(asset LinkedAsset)
	OnLinkFile      func(asset LinkedAsset)
	OnTimeline     func()
	// LinkErr holds the Core rejection cause for the last link attempt on this
	// task so the recording surface can show cause + retry (UX-P5).
	LinkErr string
	// AssetErr holds the failure cause for the last asset open/relocate/load
	// attempt so the asset list renders a persistent banner with retry (UX-P5).
	AssetErr        string
	OnRetryAssetErr func()
	OnKindChange    func(kind ViewKind)
	OnSaveBatch     func(cells []resultgrid.Cell)
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
		if r.Grid.State == resultgrid.StateSaving {
			return
		}
		if err := r.Grid.CommitPendingEdit(); err != nil {
			r.Grid.Error = err.Error()
			r.Grid.State = resultgrid.StateError
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
						r.Grid.State = resultgrid.StateError
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
			// UX-STATE, UX-P5: make unsaved, saving, and retryable failure states explicit beside Save.
			switch r.Grid.State {
			case resultgrid.StateError:
				ui.Text(c, "保存失败").FontSize(t.Rem(.72)).Bold().TextColor(tokens.DangerText)
			case resultgrid.StateSaving:
				ui.Text(c, "保存中…").FontSize(t.Rem(.72)).Bold().TextColor(t.TextMuted)
			default:
				ui.Text(c, "未保存").FontSize(t.Rem(.72)).Bold().TextColor(tokens.WarningText)
			}
			saveLabel := "保存读数"
			if r.Grid.State == resultgrid.StateError {
				saveLabel = "重试保存"
			}
			save := components.Button(c, saveLabel, components.ButtonPrimary)
			save.Disabled(r.Grid.State == resultgrid.StateSaving)
			if save.Clicked() && r.Grid.State != resultgrid.StateSaving {
				saveNow()
			}
		}
	})
	if r.Grid != nil && r.Grid.State == resultgrid.StateError && r.Grid.Error != "" {
		// UX-P5 / UX-STATE / UX-DONE: retain the Core rejection cause and choose guidance per cause.
		headline, guidance := gridErrorPresentation(r.Grid.Error)
		ui.Column(c).FillWidth().Padding(t.Space(.75), t.Space(1.25)).Gap(t.Space(.5)).
			Background(tokens.DangerBg).Children(func() {
			ui.Text(c, headline).FontSize(t.Rem(.76)).TextColor(tokens.DangerText)
			ui.Text(c, guidance).FontSize(t.Rem(.72)).TextColor(tokens.DangerText)
		})
	}
	if r.Kind != ViewGallery && r.AssetErr != "" {
		renderAssetErrBanner(c, r.AssetErr, r.OnRetryAssetErr, t, tokens)
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

// cellStatusLabel maps a Core cell status to operator-facing wording, never the
// raw enum value (UX-STATE).
func cellStatusLabel(status string) string {
	switch status {
	case "raw", "recorded":
		return "已记录"
	case "missing", "":
		return "缺失"
	case "excluded":
		return "排除"
	}
	return status
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
				ui.Text(c, cellStatusLabel(m.Status)).FontSize(11).TextColor(t.TextMuted)
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
								g.State = resultgrid.StateError
							}
						} else if v, err := strconv.ParseFloat(*input, 64); err != nil {
							g.Error = "请输入数字"
							g.State = resultgrid.StateError
						} else {
							cell.Value, cell.Status = &v, "raw"
							if err := g.SetCell(cell); err != nil {
								g.Error = err.Error()
								g.State = resultgrid.StateError
							}
						}
					}
					status := cell.Status
					if status == "" {
						status = "missing"
					}
					ui.Text(c, cellStatusLabel(status)).Width(44).FontSize(11).TextColor(t.TextMuted)
				})
			}
		})
	})
	if g.Error != "" {
		ui.Text(c, g.Error).FontSize(t.Rem(.75)).TextColor(t.Danger)
	}
}

// assetMatrix groups linked assets strictly by Core coordinates. Missing any
// coordinate stays in the explicit unlabelled bucket; paths are never parsed.
type assetMatrixCell struct {
	WellID     string
	Timepoint  string
	Assets     []LinkedAsset
	FOVIndices []uint32
	Gaps       []uint32
}

func assetMatrix(assets []LinkedAsset) []assetMatrixCell {
	type key struct{ well, timepoint string }
	groups := map[key][]LinkedAsset{}
	for _, asset := range assets {
		k := key{asset.WellID, asset.Timepoint}
		if k.well == "" || k.timepoint == "" || asset.FOVIndex == 0 {
			k = key{"未标注", ""}
		}
		groups[k] = append(groups[k], asset)
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].well != keys[j].well {
			return keys[i].well < keys[j].well
		}
		return keys[i].timepoint < keys[j].timepoint
	})
	cells := make([]assetMatrixCell, 0, len(keys))
	for _, k := range keys {
		cell := assetMatrixCell{WellID: k.well, Timepoint: k.timepoint, Assets: groups[k]}
		if k.well != "未标注" {
			seen, max := map[uint32]bool{}, uint32(0)
			for _, asset := range cell.Assets {
				if !seen[asset.FOVIndex] {
					cell.FOVIndices = append(cell.FOVIndices, asset.FOVIndex)
					seen[asset.FOVIndex] = true
				}
				if asset.FOVIndex > max {
					max = asset.FOVIndex
				}
			}
			sort.Slice(cell.FOVIndices, func(i, j int) bool {
				return cell.FOVIndices[i] < cell.FOVIndices[j]
			})
			for i := uint32(1); i < max; i++ {
				if !seen[i] {
					cell.Gaps = append(cell.Gaps, i)
				}
			}
		}
		sort.Slice(cell.Assets, func(i, j int) bool {
			return cell.Assets[i].FOVIndex < cell.Assets[j].FOVIndex
		})
		cells = append(cells, cell)
	}
	return cells
}

// renderGallery draws the coordinate matrix, one asset preview, and file actions.
// The preview uses Core coordinates only, never path-derived data.
func (r *RecordingView) renderGallery(c *ui.Context) {
	t := c.Theme()
	tokens := theme.GetTokens(t)
	cells := assetMatrix(r.Assets)
	ui.Column(c).Key("recording-gallery").Gap(t.Space(2)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "孔位 × 时间点").FontSize(float32(tokens.TextBody)).Bold().TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("%d 个文件 · 本地引用", len(r.Assets))).
				FontSize(float32(tokens.TextMeta)).TextColor(t.TextMuted)
			if r.OnLinkFile != nil && components.Button(c, "关联本地文件", components.ButtonSecondary).Clicked() {
				r.OnLinkFile(LinkedAsset{LinkedTo: AssetLink{TaskKey: r.Task.TaskKey}})
			}
		})
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
		if r.AssetErr != "" {
			renderAssetErrBanner(c, r.AssetErr, r.OnRetryAssetErr, t, tokens)
		}
		if len(r.Assets) == 0 {
			ui.Text(c, "尚未关联本地文件。").FontSize(12).TextColor(t.TextMuted)
		}
		if len(cells) > 0 {
			byKey, wells, times := map[[2]string]assetMatrixCell{}, []string{}, []string{}
			wellSet, timeSet := map[string]bool{}, map[string]bool{}
			var unlabelled *assetMatrixCell
			for i := range cells {
				cell := cells[i]
				if cell.WellID == "未标注" {
					unlabelled = &cell
					continue
				}
				byKey[[2]string{cell.WellID, cell.Timepoint}] = cell
				if !wellSet[cell.WellID] {
					wellSet[cell.WellID] = true
					wells = append(wells, cell.WellID)
				}
				if !timeSet[cell.Timepoint] {
					timeSet[cell.Timepoint] = true
					times = append(times, cell.Timepoint)
				}
			}
			sort.Strings(wells)
			sort.Strings(times)
			ui.Scroll(c).FillWidth().Children(func() {
				ui.Column(c).MinWidth(float32(180 + 184*len(times))).Gap(t.Space(.5)).Children(func() {
					ui.Row(c).Gap(t.Space(.5)).Children(func() {
						matrixCellBox(c, "孔位", tokens, t, 180, func() {})
						for _, timepoint := range times {
							matrixCellBox(c, timepoint, tokens, t, 180, func() {})
						}
					})
					for _, well := range wells {
						well := well
						ui.Row(c).Gap(t.Space(.5)).Children(func() {
							matrixCellBox(c, well, tokens, t, 180, func() {})
							for _, timepoint := range times {
								cell, exists := byKey[[2]string{well, timepoint}]
								matrixCellBox(c, "", tokens, t, 180, func() {
									if !exists {
										ui.Text(c, "1 个缺口 · 未关联视野").
											FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
										if r.OnLinkFile != nil && components.Button(c, "补齐", components.ButtonSecondary).Clicked() {
											r.OnLinkFile(LinkedAsset{
												WellID: well, Timepoint: timepoint, FOVIndex: 1,
												LinkedTo: AssetLink{TaskKey: r.Task.TaskKey},
											})
										}
										return
									}
									if len(cell.Gaps) > 0 {
										ui.Text(c, fmt.Sprintf("%d 个缺口 · FOV %v", len(cell.Gaps), cell.Gaps)).
											FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
										for _, gap := range cell.Gaps {
											gap := gap
											if r.OnLinkFile != nil && components.Button(c, fmt.Sprintf("补齐 FOV %d", gap), components.ButtonSecondary).Clicked() {
												r.OnLinkFile(LinkedAsset{
													WellID: well, Timepoint: timepoint, FOVIndex: gap,
													LinkedTo: AssetLink{TaskKey: r.Task.TaskKey},
												})
											}
										}
									}
									ui.Text(c, fmt.Sprintf("FOV %v", cell.FOVIndices)).
										FontSize(t.Rem(.7)).TextColor(t.Text)
								})
							}
						})
					}
					if unlabelled != nil {
						matrixCellBox(c, "未标注", tokens, t, 180, func() {
							ui.Text(c, fmt.Sprintf("%d 个未标注文件", len(unlabelled.Assets))).
								FontSize(t.Rem(.7)).TextColor(t.TextMuted)
						})
					}
				})
			})
		}
		if len(r.Assets) > 0 {
			selected := r.Assets[0]
			ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).
				Background(tokens.Surface).Border(1, tokens.Border).Gap(t.Space(.75)).Children(func() {
				ui.Text(c, "资产预览").FontSize(t.Rem(.78)).Bold().TextColor(t.Text)
				ui.Text(c, selected.RelativePath).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				ui.Text(c, fmt.Sprintf("孔位：%s · 时间点：%s · 视野：%s",
					labelOr(selected.WellID, "未标注"), labelOr(selected.Timepoint, "未标注"),
					fovLabel(selected.FOVIndex))).FontSize(t.Rem(.72)).TextColor(t.Text)
				if r.OnTimeline != nil && components.Button(c, "返回 Run 时间线", components.ButtonGhost).Clicked() {
					r.OnTimeline()
				}
			})
		}
		for _, a := range r.Assets {
			assetRow(c, a, r.OnOpenAsset, r.OnRelocate, r.OnRelabelAsset, t, tokens)
		}
		if len(r.Task.Actual.Measurements) > 0 {
			ui.Divider(c)
			ui.Text(c, "定量指标").FontSize(12).Bold().TextColor(t.TextMuted)
			r.renderTable(c)
		}
	})
}

func matrixCellBox(c *ui.Context, title string, tokens theme.Tokens, t *ui.Theme, width float32, body func()) {
	ui.Column(c).Width(width).MinHeight(76).Padding(t.Space(1), t.Space(1.25)).
		Background(tokens.Surface).Border(1, tokens.Border).Gap(t.Space(.5)).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(t.Rem(.75)).Bold().TextColor(t.Text)
		}
		body()
	})
}

func fovLabel(index uint32) string {
	if index == 0 {
		return "未标注"
	}
	return strconv.FormatUint(uint64(index), 10)
}

func assetRow(c *ui.Context, a LinkedAsset, open, relocate, relabel func(LinkedAsset), t *ui.Theme, tokens theme.Tokens) {
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
			if a.WellID == "" || a.Timepoint == "" || a.FOVIndex == 0 {
				ui.Text(c, "未标注 · 需要 Core 提供坐标更新/解除关联接口").
					FontSize(t.Rem(.68)).Bold().TextColor(tokens.WarningText)
			}
		})
		if a.WellID == "" || a.Timepoint == "" || a.FOVIndex == 0 {
			if relabel != nil {
				if components.Button(c, "重设坐标", components.ButtonGhost).Clicked() {
					relabel(a)
				}
			}
			return
		}
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

func gridErrorPresentation(errMsg string) (string, string) {
	lower := strings.ToLower(errMsg)
	switch {
	case strings.Contains(errMsg, "刷新") && (strings.Contains(errMsg, "已写入 Core") || strings.Contains(errMsg, "已保存到 Core")):
		return errMsg, "读数已写入 Core，仅最新视图刷新失败；请点击重试刷新或重新打开当前 Run。"
	case strings.Contains(lower, "conflict") || strings.Contains(errMsg, "冲突") || strings.Contains(lower, "revision"):
		return "未保存 · " + errMsg, "Core 修订版本已发生变化（修订冲突）；请重新加载当前 Run 核对最新数据后再重试保存。"
	case strings.Contains(errMsg, "未接受全部单元格") || strings.Contains(errMsg, "被 Core 拒绝") || strings.Contains(lower, "rejected") || strings.Contains(lower, "invalid"):
		return "未保存 · " + errMsg, "Core 拒绝了部分或全部单元格；请核对单元格数值、状态与孔位定义后，点击重试保存。"
	case strings.Contains(lower, "permission") || strings.Contains(lower, "read-only") || strings.Contains(lower, "denied") || strings.Contains(errMsg, "权限") || strings.Contains(errMsg, "只读"):
		return "未保存 · " + errMsg, "检查目标目录权限后，点击重试保存。"
	case strings.Contains(errMsg, "请输入数字") || strings.Contains(lower, "cell coordinate") || strings.Contains(lower, "must be finite") || strings.Contains(lower, "missing status"):
		return "未保存 · " + errMsg, "请修正无效的单元格输入后，重新提交并保存。"
	default:
		return "未保存 · " + errMsg, "请根据上方错误原因排查后，点击重试保存。"
	}
}

func renderAssetErrBanner(c *ui.Context, message string, onRetry func(), t *ui.Theme, tokens theme.Tokens) {
	if message == "" {
		return
	}
	ui.Row(c).FillWidth().Padding(6, 10).Radius(6).
		Background(tokens.DangerBg).Border(1, tokens.DangerText).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, message).FontSize(t.Rem(.72)).Bold().TextColor(tokens.DangerText)
		})
		if onRetry != nil {
			if components.Button(c, "重试", components.ButtonPrimary).Clicked() {
				onRetry()
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
