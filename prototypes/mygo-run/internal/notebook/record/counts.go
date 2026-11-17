package record

import (
	"fmt"

	"asterlab-mygo-prototype/modules/runs"
)

// TaskStatus is the frozen plan skeleton of one task, adapted from the
// Core-delivered snapshot. Statuses arrive from Core (LoadRun/CompleteTask);
// Go never re-derives them.
type TaskStatus struct {
	Key       string
	Title     string
	Timepoint string
	Status    string
}

// SnapshotTasks adapts the Core-delivered frozen tasks for counting. It
// carries no derivation: titles and statuses are Core's.
func SnapshotTasks(tasks []runs.RunTask) []TaskStatus {
	out := make([]TaskStatus, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, TaskStatus{
			Key:       task.Key,
			Title:     task.Title,
			Timepoint: task.Timepoint,
			Status:    task.Status,
		})
	}
	return out
}

// Counts is the one counting of a Record projection. Notebook, Run history,
// the Protocol archive and the PDF all read these numbers, so the same Run
// can never disagree with itself. Every field traces to a Core member:
//
//   - measurements: typed readout records plus raw readings that carry a value
//   - missing: valueless readings plus unfilled planned image slots
//   - excluded: facts Core kept out of the plan or the matrix (unplanned
//     parameters, unplanned/unlocated assets); each carries an issue
//   - skips: Core's rule-skip count from the timeline
//   - timers: projected durable timers
//   - asset groups: frozen assay groups plus located assets
//   - gaps: unfilled planned image slots
type Counts struct {
	TasksTotal     int
	TasksCompleted int
	Measurements   int
	Missing        int
	Excluded       int
	SkippedByRule  int
	Deviations     int
	Timers         int
	TimersRunning  int
	Groups         int
	Assets         int
	ImagesPlanned  int
	ImagesFilled   int
	ImagesMissing  int
	Issues         int
}

// Summarize counts one Core projection. Tasks come from the Core-delivered
// snapshot; everything else comes from the projection itself.
func Summarize(proj Projection, tasks []TaskStatus) Counts {
	counts := Counts{
		TasksTotal:    len(tasks),
		SkippedByRule: int(proj.Timeline.SkippedByRule),
		Timers:        len(proj.Timers),
		Groups:        len(proj.Design.Groups),
		Assets:        len(proj.Assets),
		ImagesPlanned: int(proj.Images.PlannedSlots),
		ImagesFilled:  int(proj.Images.FilledSlots),
		ImagesMissing: int(proj.Images.MissingSlots),
		Issues:        len(proj.Issues),
	}
	for _, task := range tasks {
		if task.Status == "completed" {
			counts.TasksCompleted++
		}
	}
	for _, reading := range proj.Readings {
		if reading.Value == nil {
			counts.Missing++
		} else {
			counts.Measurements++
		}
	}
	counts.Measurements += len(proj.Readouts)
	for _, param := range proj.Parameters {
		if param.Status == "unplanned" {
			counts.Excluded++
		}
	}
	counts.Excluded += len(proj.Images.UnplannedAssetIDs) + len(proj.Images.UnlocatedAssetIDs)
	counts.Missing += int(proj.Images.MissingSlots)
	for _, event := range proj.Timeline.Events {
		if event.Deviation != nil && *event.Deviation != "" {
			counts.Deviations++
		}
	}
	for _, timer := range proj.Timers {
		if timer.Status == "running" {
			counts.TimersRunning++
		}
	}
	return counts
}

// Headline renders the one summary line every record surface shows. Notebook,
// Run history, the Protocol archive and the PDF embed this exact string, so
// the same Run can never disagree with itself across surfaces.
func Headline(c Counts) string {
	return fmt.Sprintf("任务完成 %d/%d · 实测 %d 个 · 缺 %d 个 · 排除 %d 个 · 跳过 %d 次 · 计时 %d 个 · 缺 %d 个槽",
		c.TasksCompleted, c.TasksTotal, c.Measurements, c.Missing, c.Excluded,
		c.SkippedByRule, c.Timers, c.ImagesMissing)
}

// TaskStatusWord names a frozen task status in the §3 vocabulary
// (docs/product/ux-contract.md). Unknown Core statuses pass through so a new
// status is visible instead of silently blank.
func TaskStatusWord(status string) string {
	switch status {
	case "not_started":
		return "未开始"
	case "in_progress":
		return "进行中"
	case "completed":
		return "已完成"
	case "blocked":
		return "阻塞"
	case "skipped", "skipped_by_rule":
		return "已跳过"
	default:
		return status
	}
}

// ParamStatusWord names a Core parameter comparison status. MissingActual
// and unplanned are the explicit缺/计划外 the sign flow asks the operator to
// acknowledge.
func ParamStatusWord(status string) string {
	switch status {
	case "matches":
		return "一致"
	case "differs":
		return "不同"
	case "missing_actual":
		return "缺实测"
	case "unplanned":
		return "计划外"
	default:
		return status
	}
}

// ArchiveStatusWord names the persisted Core record lifecycle. Only the
// persistent state row is authority; the prose signature trailer is history.
func ArchiveStatusWord(status string) string {
	switch status {
	case "recording":
		return "未签核"
	case "summarizing", "signed":
		return "签核处理中"
	case "archived":
		return "已签核归档"
	default:
		return status
	}
}
