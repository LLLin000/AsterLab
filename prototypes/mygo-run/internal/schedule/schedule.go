package schedule

import (
	"sort"
	"strings"
	"time"
)

// Domain identifies which laboratory subsystem owns a scheduled calendar event.
type Domain string

const (
	DomainExperiment Domain = "experiment"
	DomainReagent    Domain = "reagent"
	DomainVivarium   Domain = "vivarium"
)

// EventKind classifies the exact biological, operational, or compliance milestone.
type EventKind string

const (
	KindExpPlanned  EventKind = "exp_planned"
	KindExpRunning  EventKind = "exp_running"
	KindExpDone     EventKind = "exp_done"

	KindReagentPurchase   EventKind = "reagent_purchase"
	KindReagentOpened     EventKind = "reagent_opened"
	KindReagentExpirySoon EventKind = "reagent_expiry_soon"
	KindReagentExpired    EventKind = "reagent_expired"
	KindReagentRestock    EventKind = "reagent_restock"

	KindAnimalArrival  EventKind = "animal_arrival"
	KindQuarantineEnd  EventKind = "quarantine_end"
	KindAnimalDosing   EventKind = "animal_dosing"
	KindAnimalWeigh    EventKind = "animal_weigh"
	KindAnimalHarvest  EventKind = "animal_harvest"
	KindIACUCExpiry    EventKind = "iacuc_expiry"
)

// Severity grades urgency for sorting and visual badge contrast.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeveritySuccess Severity = "success"
	SeverityWarning Severity = "warning"
	SeverityDanger  Severity = "danger"
)

// Event is a lightweight read-only projection rendered onto the unified calendar.
type Event struct {
	ID          string
	Domain      Domain
	Kind        EventKind
	DateYMD     string // YYYY-MM-DD
	Title       string
	Subtitle    string
	Severity    Severity
	SourceID    string
	SourceType  string // "experiment" | "reagent" | "cage"
	MilestoneID string
	Done        bool
	ActionLabel string
}

// Filter controls which domains and search query appear in calendar views.
type Filter struct {
	ShowExperiment bool
	ShowReagent    bool
	ShowVivarium   bool
	Query          string
}

// DefaultFilter enables all three laboratory domains with an empty search query.
func DefaultFilter() Filter {
	return Filter{
		ShowExperiment: true,
		ShowReagent:    true,
		ShowVivarium:   true,
	}
}

// UpcomingSummary aggregates critical events over a lookahead horizon for the companion drawer.
type UpcomingSummary struct {
	ReagentExpiring int
	ReagentExpired  int
	AnimalHarvests  int
	AnimalTasks     int
	IACUCAlerts     int
	NextEvents      []Event
}

// KindLabel returns the localized Chinese label for a schedule event kind.
func KindLabel(k EventKind) string {
	switch k {
	case KindExpPlanned:
		return "实验排期"
	case KindExpRunning:
		return "实验进行"
	case KindExpDone:
		return "实验完成"
	case KindReagentPurchase:
		return "试剂购入"
	case KindReagentOpened:
		return "开封启用"
	case KindReagentExpirySoon:
		return "临期预警"
	case KindReagentExpired:
		return "试剂到期"
	case KindReagentRestock:
		return "计划补货"
	case KindAnimalArrival:
		return "动物入栏"
	case KindQuarantineEnd:
		return "检疫解封"
	case KindAnimalDosing:
		return "造模/给药"
	case KindAnimalWeigh:
		return "计划称重"
	case KindAnimalHarvest:
		return "收样取材"
	case KindIACUCExpiry:
		return "伦理到期"
	default:
		return "日程事项"
	}
}

func severityRank(s Severity) int {
	switch s {
	case SeverityDanger:
		return 0
	case SeverityWarning:
		return 1
	case SeverityInfo:
		return 2
	case SeveritySuccess:
		return 3
	default:
		return 4
	}
}

func kindPriorityRank(k EventKind) int {
	switch k {
	case KindReagentExpired, KindIACUCExpiry:
		return 0
	case KindAnimalHarvest:
		return 1
	case KindReagentExpirySoon:
		return 2
	case KindAnimalDosing, KindAnimalWeigh, KindQuarantineEnd:
		return 3
	case KindReagentRestock, KindReagentPurchase, KindAnimalArrival:
		return 4
	default:
		return 5
	}
}

func matchesFilter(ev Event, flt Filter) bool {
	if len(strings.TrimSpace(ev.DateYMD)) < 10 {
		return false
	}
	switch ev.Domain {
	case DomainExperiment:
		if !flt.ShowExperiment {
			return false
		}
	case DomainReagent:
		if !flt.ShowReagent {
			return false
		}
	case DomainVivarium:
		if !flt.ShowVivarium {
			return false
		}
	}
	q := strings.ToLower(strings.TrimSpace(flt.Query))
	if q == "" {
		return true
	}
	hay := strings.ToLower(ev.Title + " " + ev.Subtitle + " " + KindLabel(ev.Kind))
	return strings.Contains(hay, q)
}

// FilterEvents returns matching events sorted by DateYMD ascending then urgency descending.
func FilterEvents(events []Event, flt Filter) []Event {
	out := make([]Event, 0, len(events))
	for _, ev := range events {
		if matchesFilter(ev, flt) {
			ev.DateYMD = ev.DateYMD[:10]
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DateYMD != out[j].DateYMD {
			return out[i].DateYMD < out[j].DateYMD
		}
		if out[i].Done != out[j].Done {
			return !out[i].Done
		}
		ri, rj := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if ri != rj {
			return ri < rj
		}
		pi, pj := kindPriorityRank(out[i].Kind), kindPriorityRank(out[j].Kind)
		if pi != pj {
			return pi < pj
		}
		return out[i].Title < out[j].Title
	})
	return out
}

// IndexByDate groups filtered, sorted events into a YYYY-MM-DD map.
func IndexByDate(events []Event, flt Filter) map[string][]Event {
	grouped := map[string][]Event{}
	for _, ev := range FilterEvents(events, flt) {
		grouped[ev.DateYMD] = append(grouped[ev.DateYMD], ev)
	}
	return grouped
}

// SummarizeUpcoming counts upcoming reagent & vivarium alerts/milestones within horizonDays.
func SummarizeUpcoming(events []Event, todayYMD string, horizonDays int) UpcomingSummary {
	var sum UpcomingSummary
	if horizonDays <= 0 {
		horizonDays = 7
	}
	today, err := time.Parse("2006-01-02", strings.TrimSpace(todayYMD))
	if err != nil {
		now := time.Now()
		today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	}
	cutoff := today.AddDate(0, 0, horizonDays).Format("2006-01-02")
	todayStr := today.Format("2006-01-02")

	sorted := FilterEvents(events, DefaultFilter())
	for _, ev := range sorted {
		if ev.Domain == DomainExperiment || ev.Done {
			continue
		}
		inPastOrToday := ev.DateYMD <= todayStr
		inHorizon := ev.DateYMD >= todayStr && ev.DateYMD <= cutoff

		switch ev.Kind {
		case KindReagentExpired:
			if ev.DateYMD <= cutoff {
				sum.ReagentExpired++
				sum.NextEvents = append(sum.NextEvents, ev)
			}
		case KindReagentExpirySoon, KindReagentRestock:
			if inHorizon || inPastOrToday {
				sum.ReagentExpiring++
				sum.NextEvents = append(sum.NextEvents, ev)
			}
		case KindAnimalHarvest:
			if inHorizon {
				sum.AnimalHarvests++
				sum.NextEvents = append(sum.NextEvents, ev)
			}
		case KindAnimalDosing, KindAnimalWeigh, KindQuarantineEnd:
			if inHorizon {
				sum.AnimalTasks++
				sum.NextEvents = append(sum.NextEvents, ev)
			}
		case KindIACUCExpiry:
			if inHorizon || inPastOrToday {
				sum.IACUCAlerts++
				sum.NextEvents = append(sum.NextEvents, ev)
			}
		default:
			if inHorizon {
				sum.NextEvents = append(sum.NextEvents, ev)
			}
		}
	}
	if len(sum.NextEvents) > 10 {
		sum.NextEvents = sum.NextEvents[:10]
	}
	return sum
}
