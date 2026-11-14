package runworkspace

import (
	"sort"
	"strings"
	"time"
)

// BuildTimeline projects only persisted Core events; it never reads or derives task status.
// UX-STATE, UX-P3, UX-P4, UX-P5: retain explicit event outcomes in the shared component language.
func BuildTimeline(vm *RunWorkspaceViewModel) []EventViewModel {
	if vm == nil {
		return nil
	}
	events := make([]EventViewModel, 0, len(vm.UnassociatedEvents))
	for _, event := range vm.UnassociatedEvents {
		events = append(events, event)
	}
	for _, task := range vm.TaskByKey {
		if task != nil {
			events = append(events, task.Events...)
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		a, b := timelineTime(events[i]), timelineTime(events[j])
		if a == "" || b == "" {
			if a == b {
				return events[i].Sequence < events[j].Sequence
			}
			return b == ""
		}
		at, aErr := parseTimelineTime(a)
		bt, bErr := parseTimelineTime(b)
		if aErr == nil && bErr == nil {
			if at.Equal(bt) {
				return events[i].Sequence < events[j].Sequence
			}
			return at.Before(bt)
		}
		if a != b {
			return a < b
		}
		return events[i].Sequence < events[j].Sequence
	})
	attempts := map[string]int{}
	failed := map[string]bool{}
	for i := range events {
		event := &events[i]
		for _, check := range event.Checks {
			key := event.TaskKey + "\x00" + check.CheckID
			attempts[key]++
			if attempts[key] > event.Attempt {
				event.Attempt = attempts[key]
			}
			if check.Status == "failed" || check.Status == "blocked" {
				failed[key] = true
			}
			if check.Status == "passed" && failed[key] {
				event.Retry = true
			}
		}
		event.Badge = timelineBadge(*event)
	}
	return events
}

func timelineTime(event EventViewModel) string {
	if event.RecordedAt != "" {
		return event.RecordedAt
	}
	return event.OccurredAt
}

func parseTimelineTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02", value)
}

func timelineBadge(event EventViewModel) string {
	if event.Retry {
		return "重试"
	}
	for _, check := range event.Checks {
		if check.Status == "failed" || check.Status == "blocked" {
			return "失败"
		}
	}
	for _, check := range event.Checks {
		if check.Status == "passed" {
			return "已确认"
		}
	}
	switch event.StepStatus {
	case "completed":
		return "已完成"
	case "blocked":
		return "失败"
	}
	if len(event.Measurements) > 0 || event.Type == "measurement" {
		return "测量"
	}
	if event.Deviation != "" || event.Type == "deviation" {
		return "偏差"
	}
	return ""
}

func timelineCheckStatus(status string) string {
	switch strings.ToLower(status) {
	case "passed":
		return "已确认"
	case "failed":
		return "失败"
	case "blocked":
		return "阻塞"
	case "skipped":
		return "跳过"
	default:
		return status
	}
}
