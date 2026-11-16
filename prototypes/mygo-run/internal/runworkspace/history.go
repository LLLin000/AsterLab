package runworkspace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"
	"asterlab-mygo-prototype/modules/runs"

	"github.com/egoist/mygo/ui"
)

// historyProjection returns the shared record projection for the history
// tab, memoized by Run revision plus every linked asset's identity and
// coordinates. Asset link/update/unlink never bump the Run revision, so a
// bare count would reuse a stale projection after a coordinate relink with
// an unchanged asset count; the tokens below make the relink a new key.
// Inputs are re-read every call (the host getter only hands over the stored
// snapshot and asset list), but the derivation itself runs only when the key
// moves; the UI never re-derives it. A nil callback (or nil inputs) means
// the host has nothing to project, and the history tab keeps its
// entry-level rendering only.
func (s *State) historyProjection() *record.RecordProjection {
	if s == nil || s.Callbacks.HistoryInputs == nil {
		return nil
	}
	snap, assets := s.Callbacks.HistoryInputs()
	if strings.TrimSpace(snap.RunID) == "" {
		return nil
	}
	bench := s.bench()
	key := historyMemoKey(snap, assets)
	if bench.historyProj == nil || bench.historyKey != key {
		proj := record.Project(snap, assets)
		bench.historyProj = &proj
		bench.historyKey = key
	}
	return bench.historyProj
}

// historyMemoKey keeps the RunID|Revision prefix and folds every linked
// asset's identity plus its recorded coordinates into the key, sorted so
// link-list order never matters. A relinked well, timepoint or FOV is a new
// key even when the asset count is unchanged.
func historyMemoKey(snap runs.RunSnapshot, assets []runs.Asset) string {
	tokens := make([]string, 0, len(assets))
	for _, asset := range assets {
		tokens = append(tokens, strings.Join([]string{
			asset.ID,
			asset.Link.WellID,
			asset.Link.Timepoint,
			strconv.FormatUint(uint64(asset.Link.FOVIndex), 10),
		}, "\x00"))
	}
	sort.Strings(tokens)
	return snap.RunID + "|" + strconv.FormatUint(snap.Revision, 10) + "|" + strings.Join(tokens, "\x01")
}

// renderHistoryRecord draws the shared projection above the entry timeline:
// planned-vs-actual summary rows, then the capture audit matrix. The entries
// below stay exactly as notebook.Build projects them; this section never
// replaces the timeline.
func renderHistoryRecord(c *ui.Context, s *State, proj *record.RecordProjection, t *ui.Theme, tokens theme.Tokens) {
	if proj == nil {
		return
	}
	blind := s.bench().Blind
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		renderHistorySummary(c, proj, t, tokens)
		renderHistoryMatrix(c, proj, blind, t, tokens)
	})
}

func renderHistorySummary(c *ui.Context, proj *record.RecordProjection, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "计划 vs 实际").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("共 %d 步", len(proj.Summary))).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
		})
		if len(proj.Summary) == 0 {
			ui.Text(c, "暂无任务摘要。").FontSize(t.Rem(.74)).TextColor(t.TextMuted)
			return
		}
		for _, sum := range proj.Summary {
			sum := sum
			status := buildTaskStatus(runs.RunTask{Status: sum.Status})
			ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
				Background(tokens.Elevated).Border(1, tokens.Border).Gap(t.Space(.75)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Text(c, fallback(sum.Title, sum.TaskKey)).FontSize(t.Rem(.8)).Bold().TextColor(t.Text)
					if sum.Timepoint != "" {
						ui.Text(c, sum.Timepoint).FontSize(t.Rem(.7)).TextColor(t.TextMuted)
					}
					taskStatusPill(c, status, t, tokens)
				})
				if len(sum.Planned) > 0 {
					ui.Text(c, "计划："+paramsLine(sum.Planned)).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
				}
				if len(sum.Actual) > 0 {
					ui.Text(c, "实际："+paramsLine(sum.Actual)).FontSize(t.Rem(.72)).TextColor(t.Text)
				}
				for _, m := range sum.Measurements {
					ui.Text(c, fmt.Sprintf("读数 %s：%s %s", m.Endpoint, m.Value, m.Unit)).
						FontSize(t.Rem(.72)).TextColor(t.Text)
				}
				for _, deviation := range sum.Deviations {
					ui.Text(c, "偏差："+deviation).FontSize(t.Rem(.72)).TextColor(tokens.WarningText)
				}
			})
		}
	})
}

func paramsLine(params []record.Param) string {
	parts := make([]string, 0, len(params))
	for _, p := range params {
		label := p.Label
		if label == "" {
			label = p.Key
		}
		value := strings.TrimSpace(p.Value + " " + strings.TrimSpace(p.Unit))
		if value == "" {
			parts = append(parts, label)
		} else {
			parts = append(parts, label+" "+value)
		}
	}
	return strings.Join(parts, "；")
}

func renderHistoryMatrix(c *ui.Context, proj *record.RecordProjection, blind bool, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "采图对账").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%d 孔位 · 缺 %d 视野", len(proj.Matrix), proj.Gaps)).
				FontSize(t.Rem(.7)).TextColor(t.TextMuted).FontFeatures("tnum")
		})
		if len(proj.Matrix) == 0 {
			ui.Text(c, "暂无采图记录。").FontSize(t.Rem(.74)).TextColor(t.TextMuted)
		}
		for _, cell := range proj.Matrix {
			cell := cell
			filled := cell.FilledSlots()
			label := fmt.Sprintf("%s · %s %d/%d", cell.Well, cell.Timepoint, filled, record.FOVSlots)
			bg, fg := tokens.Elevated, t.TextMuted
			if filled == record.FOVSlots {
				bg, fg = tokens.SuccessBg, tokens.SuccessText
			} else if filled > 0 {
				bg, fg = tokens.WarningBg, tokens.WarningText
			}
			ui.Column(c).FillWidth().Padding(t.Space(1), t.Space(1.5)).Radius(6).
				Background(bg).Border(1, tokens.Border).Gap(t.Space(.5)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Text(c, label).FontSize(t.Rem(.74)).Bold().TextColor(fg).FontFeatures("tnum")
					ui.Spacer(c)
					ui.Text(c, strings.Join(matrixGroups(cell, blind), " · ")).
						FontSize(t.Rem(.68)).TextColor(t.TextMuted)
				})
				if cell.Overflow > 0 {
					ui.Text(c, fmt.Sprintf("多出 %d 张未归槽图", cell.Overflow)).
						FontSize(t.Rem(.68)).TextColor(tokens.WarningText)
				}
			})
		}
		if proj.UnassignedImages > 0 {
			ui.Text(c, fmt.Sprintf("未定位图 %d 张：%s", proj.UnassignedImages,
				strings.Join(proj.UnassignedLabels, "、"))).
				FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
		}
	})
}

// matrixGroups names the groups measured in a cell, masked under blinding.
// Well identity is never masked: wells are coordinates, not group identity.
func matrixGroups(cell record.MatrixCell, blind bool) []string {
	if blind && len(cell.Groups) > 0 {
		return []string{"已设盲"}
	}
	if len(cell.Groups) == 0 {
		return []string{"未分组"}
	}
	return cell.Groups
}
