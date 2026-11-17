package runworkspace

import (
	"fmt"
	"strings"

	"asterlab-mygo-prototype/internal/notebook/record"
	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

// historyRecord returns the shared Core record projection for the history
// tab plus the frozen task skeleton, both through host callbacks. The
// projection is Core's single read-only RecordProjection, memoized by the
// host's opaque key (Run revision, asset coordinates, FOV plan, view); the
// UI never re-derives plan slots, groups or counts. A nil FetchRecord means
// the host has nothing to project, and the history tab keeps its
// entry-level rendering only.
func (s *State) historyRecord() (proj *record.Projection, tasks []record.TaskStatus) {
	if s == nil || s.Callbacks.FetchRecord == nil {
		return nil, nil
	}
	bench := s.bench()
	key, fetched, err := s.Callbacks.FetchRecord()
	if err != nil {
		bench.historyErr = err.Error()
		return nil, nil
	}
	bench.historyErr = ""
	if bench.historyProj == nil || bench.historyKey != string(key) {
		cp := fetched
		bench.historyProj = &cp
		bench.historyKey = string(key)
	}
	// Tasks come from the host's frozen view model (Core-delivered snapshot);
	// Go never re-derives them.
	if s != nil && s.VM != nil {
		for _, st := range s.VM.Steps {
			if st == nil {
				continue
			}
			tasks = append(tasks, record.TaskStatus{
				Key:       st.TaskKey,
				Title:     st.StepTitle,
				Timepoint: st.Timepoint,
				Status:    string(st.Status),
			})
		}
	}
	return bench.historyProj, tasks
}

// InvalidateRecord drops the memoized history projection so the next frame
// refetches from Core. The host calls it after transitions that never bump
// the Run revision (sign/archive flips the persisted archive state, not the
// snapshot); every snapshot or asset change already moves the host key.
func (s *State) InvalidateRecord() {
	if s == nil {
		return
	}
	bench := s.bench()
	bench.historyKey = ""
	bench.historyProj = nil
	bench.historyErr = ""
}

// renderHistoryRecord draws the shared Core projection above the entry
// timeline: planned-vs-actual parameter groups, the frozen plan skeleton,
// then the capture audit matrix. The entries below stay exactly as
// notebook.Build projects them; this section never replaces the timeline.
func renderHistoryRecord(c *ui.Context, s *State, proj *record.Projection, tasks []record.TaskStatus, t *ui.Theme, tokens theme.Tokens) {
	if proj == nil {
		return
	}
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		renderHistorySummary(c, proj, tasks, t, tokens)
		renderHistoryMatrix(c, proj, s.bench().Blind, t, tokens)
	})
}

func renderHistorySummary(c *ui.Context, proj *record.Projection, tasks []record.TaskStatus, t *ui.Theme, tokens theme.Tokens) {
	counts := record.Summarize(*proj, tasks)
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "计划 vs 实际").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			ui.Text(c, record.Headline(counts)).FontSize(t.Rem(.7)).TextColor(t.TextMuted).FontFeatures("tnum")
		})
		for _, task := range tasks {
			task := task
			title := task.Title
			if strings.TrimSpace(title) == "" {
				title = task.Key
			}
			line := title
			if strings.TrimSpace(task.Timepoint) != "" {
				line += " · " + task.Timepoint
			}
			ui.Text(c, line+" · "+record.TaskStatusWord(task.Status)).FontSize(t.Rem(.72)).TextColor(t.TextMuted)
		}
		for _, group := range record.GroupParameters(proj.Parameters) {
			group := group
			ui.Column(c).FillWidth().Padding(t.Space(1.5), t.Space(2)).Radius(6).
				Background(tokens.Elevated).Border(1, tokens.Border).Gap(t.Space(.75)).Children(func() {
				ui.Text(c, group.Title).FontSize(t.Rem(.8)).Bold().TextColor(t.Text)
				for _, row := range group.Rows {
					row := row
					ui.Text(c, fmt.Sprintf("%s：计划 %s → 实际 %s（%s）", row.Label, row.Planned, row.Actual, row.Status)).
						FontSize(t.Rem(.72)).TextColor(t.Text)
				}
			})
		}
		for _, issue := range proj.Issues {
			issue := issue
			ui.Text(c, "缺项 "+issue.Code+"："+issue.Message).FontSize(t.Rem(.68)).TextColor(tokens.WarningText)
		}
	})
}

func renderHistoryMatrix(c *ui.Context, proj *record.Projection, blind bool, t *ui.Theme, tokens theme.Tokens) {
	ui.Column(c).FillWidth().Gap(t.Space(1)).Children(func() {
		ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
			ui.Text(c, "采图对账").FontSize(float32(tokens.TextSection)).FontWeight(600).TextColor(t.Text)
			ui.Spacer(c)
			counts := record.Summarize(*proj, nil)
			ui.Text(c, fmt.Sprintf("%d 孔位时间点 · 缺 %d 视野", len(record.MatrixCells(*proj)), counts.ImagesMissing)).
				FontSize(t.Rem(.7)).TextColor(t.TextMuted).FontFeatures("tnum")
		})
		cells := record.MatrixCells(*proj)
		if len(cells) == 0 {
			ui.Text(c, "暂无采图记录。").FontSize(t.Rem(.74)).TextColor(t.TextMuted)
		}
		for _, cell := range cells {
			cell := cell
			label := fmt.Sprintf("%s · %s %d/%d", cell.Well, cell.Timepoint, cell.Filled, cell.Planned)
			bg, fg := tokens.Elevated, t.TextMuted
			if cell.Planned > 0 && cell.Filled == cell.Planned {
				bg, fg = tokens.SuccessBg, tokens.SuccessText
			} else if cell.Filled > 0 {
				bg, fg = tokens.WarningBg, tokens.WarningText
			}
			ui.Column(c).FillWidth().Padding(t.Space(1), t.Space(1.5)).Radius(6).
				Background(bg).Border(1, tokens.Border).Gap(t.Space(.5)).Children(func() {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(t.Space(1.5)).Children(func() {
					ui.Text(c, label).FontSize(t.Rem(.74)).Bold().TextColor(fg).FontFeatures("tnum")
					ui.Spacer(c)
					ui.Text(c, matrixGroups(*proj, cell.Well, blind)).
						FontSize(t.Rem(.68)).TextColor(t.TextMuted)
				})
			})
		}
		if len(proj.Images.UnlocatedAssetIDs) > 0 {
			ui.Text(c, fmt.Sprintf("未定位图 %d 张：%s", len(proj.Images.UnlocatedAssetIDs),
				strings.Join(proj.Images.UnlocatedAssetIDs, "、"))).
				FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
		}
		if len(proj.Images.UnplannedAssetIDs) > 0 {
			ui.Text(c, fmt.Sprintf("计划外图 %d 张：%s", len(proj.Images.UnplannedAssetIDs),
				strings.Join(proj.Images.UnplannedAssetIDs, "、"))).
				FontSize(t.Rem(.7)).TextColor(tokens.WarningText)
		}
	})
}

// matrixGroups names the group of one well, masked under blinding. Well
// identity is never masked: wells are coordinates, not group identity.
func matrixGroups(proj record.Projection, well string, blind bool) string {
	if blind || proj.View == "blind" {
		return "已设盲"
	}
	for _, plate := range proj.Images.Plates {
		for _, row := range plate.Wells {
			if strings.ToUpper(strings.TrimSpace(row.Coordinate)) != well {
				continue
			}
			if row.GroupLabel == nil || strings.TrimSpace(*row.GroupLabel) == "" {
				return "未分组"
			}
			return *row.GroupLabel
		}
	}
	return "未分组"
}
