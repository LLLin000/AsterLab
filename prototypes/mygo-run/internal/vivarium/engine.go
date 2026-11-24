package vivarium

import (
	"fmt"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/schedule"
)

// MilestoneKind identifies animal colony experiment/welfare calendar milestones.
type MilestoneKind string

const (
	MilestoneDosing  MilestoneKind = "dosing"
	MilestoneWeigh   MilestoneKind = "weigh"
	MilestoneHarvest MilestoneKind = "harvest"
	MilestoneWean    MilestoneKind = "wean"
	MilestoneCheck   MilestoneKind = "check"
)

// Milestone records one scheduled operation or sample-collection date bound to an animal cage.
type Milestone struct {
	ID      string        `json:"id"`
	Kind    MilestoneKind `json:"kind"`
	DateYMD string        `json:"dateYMD"`
	Title   string        `json:"title,omitempty"`
	Note    string        `json:"note,omitempty"`
	Done    bool          `json:"done,omitempty"`
	DoneAt  string        `json:"doneAt,omitempty"`
}

// WeightPoint records one dated average/individual body-weight check in grams.
type WeightPoint struct {
	Date    string  `json:"date"`
	WeightG float64 `json:"weightG"`
	Note    string  `json:"note,omitempty"`
}

// CageInput is a UI-free snapshot of an animal housing unit for compliance & schedule computation.
type CageInput struct {
	ID             string
	CageCode       string
	Species        string
	Strain         string
	Genotype       string
	Room           string
	RackPos        string
	DOB            string
	ArrivedOn      string
	QuarantineDays int
	IACUCNo        string
	IACUCExpiresOn string
	Status         string
	ExperimentRef  string
	MaleCount      int
	FemaleCount    int
	WeightG        float64
	WeightHistory  []WeightPoint
	Milestones     []Milestone
}

// WelfareReport summarizes GB 14925 stocking density and weight-loss humane endpoint alerts.
type WelfareReport struct {
	DensityWarning string
	WeightDropPct  float64
	WeightAlert    string
	HumaneEndpoint bool
}

// MilestoneKindLabel returns a concise Chinese label for a milestone type.
func MilestoneKindLabel(k MilestoneKind) string {
	switch k {
	case MilestoneDosing:
		return "造模/给药"
	case MilestoneWeigh:
		return "计划称重"
	case MilestoneHarvest:
		return "收样取材"
	case MilestoneWean:
		return "分笼断奶"
	case MilestoneCheck:
		return "福利巡检"
	default:
		return "实验节点"
	}
}

// TotalAnimals returns total headcount (male + female) clipped to non-negative ints.
func (c CageInput) TotalAnimals() int {
	m := c.MaleCount
	if m < 0 {
		m = 0
	}
	f := c.FemaleCount
	if f < 0 {
		f = 0
	}
	return m + f
}

// DensityLimitForSpecies returns reference single-cage headcount cap under GB 14925-2023.
func DensityLimitForSpecies(species string) int {
	switch species {
	case "mouse_white", "mouse_black", "mouse_nude":
		return 5
	case "rat":
		return 3
	case "rabbit", "beagle":
		return 1
	case "guinea_pig":
		return 2
	case "zebrafish":
		return 20
	default:
		return 5
	}
}

// AssessWelfare computes GB 14925 stocking density alerts plus body-weight decline
// humane-endpoint triggers (>=15% watch, >=20% humane endpoint review).
func AssessWelfare(c CageInput) WelfareReport {
	var rep WelfareReport
	if c.Status != "archived" {
		total := c.TotalAnimals()
		limit := DensityLimitForSpecies(c.Species)
		if total > limit {
			rep.DensityWarning = fmt.Sprintf("超密度 %d/限%d", total, limit)
		}
	}

	var peak float64
	for _, pt := range c.WeightHistory {
		if pt.WeightG > peak {
			peak = pt.WeightG
		}
	}
	cur := c.WeightG
	if cur <= 0 && len(c.WeightHistory) > 0 {
		cur = c.WeightHistory[len(c.WeightHistory)-1].WeightG
	}
	if peak > 0 && cur > 0 && cur < peak {
		drop := (peak - cur) / peak * 100.0
		rep.WeightDropPct = drop
		if drop >= 20.0 {
			rep.HumaneEndpoint = true
			rep.WeightAlert = fmt.Sprintf("体重较峰值 %.1f g 下降 %.1f%%（≥20%% 触发人道终点复核）", peak, drop)
		} else if drop >= 15.0 {
			rep.WeightAlert = fmt.Sprintf("体重较峰值 %.1f g 下降 %.1f%%（≥15%% 需加强福利监测）", peak, drop)
		}
	}
	return rep
}

// QuarantineEndDate computes arrivedOn + quarantineDays when both are valid.
func QuarantineEndDate(arrivedOn string, quarantineDays int) string {
	if quarantineDays <= 0 {
		return ""
	}
	arr, err := parseYMD(arrivedOn)
	if err != nil {
		return ""
	}
	return arr.AddDate(0, 0, quarantineDays).Format("2006-01-02")
}

// ComputeSplit splits splitMale/splitFemale animals from parent into a new child cage
// inheriting strain, genotype, DOB, arrival date, facility location, and IACUC metadata.
func ComputeSplit(parent CageInput, newCageCode string, splitMale, splitFemale int) (CageInput, CageInput, error) {
	if splitMale < 0 || splitFemale < 0 || splitMale+splitFemale <= 0 {
		return CageInput{}, CageInput{}, fmt.Errorf("分笼数量须大于 0")
	}
	if splitMale > parent.MaleCount || splitFemale > parent.FemaleCount {
		return CageInput{}, CageInput{}, fmt.Errorf("分笼数量超出当前笼存栏 (%d♂/%d♀)", parent.MaleCount, parent.FemaleCount)
	}
	remainingTotal := (parent.MaleCount - splitMale) + (parent.FemaleCount - splitFemale)
	if remainingTotal <= 0 {
		return CageInput{}, CageInput{}, fmt.Errorf("原笼须至少保留 1 只实验动物")
	}
	code := strings.TrimSpace(newCageCode)
	if code == "" {
		code = strings.TrimSpace(parent.CageCode) + "-S"
	}

	updParent := parent
	updParent.MaleCount -= splitMale
	updParent.FemaleCount -= splitFemale

	child := parent
	child.ID = fmt.Sprintf("viv-%d", time.Now().UnixNano())
	child.CageCode = code
	child.MaleCount = splitMale
	child.FemaleCount = splitFemale
	child.Status = "active"
	child.Milestones = nil
	if len(parent.WeightHistory) > 0 {
		child.WeightHistory = append([]WeightPoint(nil), parent.WeightHistory...)
	}
	return updParent, child, nil
}

// ProjectCalendarEvents converts animal cages into read-only schedule.Event items
// covering arrival, quarantine clearance, scheduled weigh/dosing/harvest milestones, and IACUC expiry.
func ProjectCalendarEvents(cages []CageInput, today time.Time) []schedule.Event {
	todayDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	out := make([]schedule.Event, 0, len(cages)*3)

	for _, cg := range cages {
		loc := strings.TrimSpace(strings.TrimSpace(cg.Room) + " " + strings.TrimSpace(cg.RackPos))
		subParts := []string{cg.Strain}
		if loc != "" {
			subParts = append(subParts, loc)
		}
		sub := strings.Join(subParts, " · ")

		if arr, err := parseYMD(cg.ArrivedOn); err == nil {
			out = append(out, schedule.Event{
				ID:          "viv-arr-" + cg.ID,
				Domain:      schedule.DomainVivarium,
				Kind:        schedule.KindAnimalArrival,
				DateYMD:     arr.Format("2006-01-02"),
				Title:       fmt.Sprintf("动物入栏 · %s", cg.CageCode),
				Subtitle:    sub,
				Severity:    schedule.SeveritySuccess,
				SourceID:    cg.ID,
				SourceType:  "cage",
				ActionLabel: "打开笼卡",
			})
		}
		if qEnd := QuarantineEndDate(cg.ArrivedOn, cg.QuarantineDays); qEnd != "" {
			qDate, _ := parseYMD(qEnd)
			done := !qDate.After(todayDay) && cg.Status != "quarantine"
			out = append(out, schedule.Event{
				ID:          "viv-quar-" + cg.ID,
				Domain:      schedule.DomainVivarium,
				Kind:        schedule.KindQuarantineEnd,
				DateYMD:     qEnd,
				Title:       fmt.Sprintf("检疫解封 · %s", cg.CageCode),
				Subtitle:    sub,
				Severity:    schedule.SeveritySuccess,
				SourceID:    cg.ID,
				SourceType:  "cage",
				Done:        done,
				ActionLabel: "核查笼卡",
			})
		}
		if iacucExp, err := parseYMD(cg.IACUCExpiresOn); err == nil {
			days := int(iacucExp.Sub(todayDay).Hours() / 24)
			sev := schedule.SeverityInfo
			if days <= 0 {
				sev = schedule.SeverityDanger
			} else if days <= 30 {
				sev = schedule.SeverityWarning
			}
			out = append(out, schedule.Event{
				ID:          "viv-iacuc-" + cg.ID,
				Domain:      schedule.DomainVivarium,
				Kind:        schedule.KindIACUCExpiry,
				DateYMD:     iacucExp.Format("2006-01-02"),
				Title:       fmt.Sprintf("伦理到期 · %s", cg.CageCode),
				Subtitle:    strings.TrimSpace(cg.IACUCNo + " · " + sub),
				Severity:    sev,
				SourceID:    cg.ID,
				SourceType:  "cage",
				ActionLabel: "续期复核",
			})
		}

		for _, ms := range cg.Milestones {
			d, err := parseYMD(ms.DateYMD)
			if err != nil {
				continue
			}
			kind := schedule.KindAnimalWeigh
			sev := schedule.SeverityInfo
			switch ms.Kind {
			case MilestoneHarvest:
				kind = schedule.KindAnimalHarvest
				sev = schedule.SeverityWarning
				if !ms.Done && !d.After(todayDay) {
					sev = schedule.SeverityDanger
				}
			case MilestoneDosing:
				kind = schedule.KindAnimalDosing
				sev = schedule.SeverityInfo
			case MilestoneWean, MilestoneCheck:
				kind = schedule.KindAnimalWeigh
				sev = schedule.SeverityInfo
			}
			title := strings.TrimSpace(ms.Title)
			if title == "" {
				title = MilestoneKindLabel(ms.Kind)
			}
			out = append(out, schedule.Event{
				ID:          "viv-ms-" + cg.ID + "-" + ms.ID,
				Domain:      schedule.DomainVivarium,
				Kind:        kind,
				DateYMD:     d.Format("2006-01-02"),
				Title:       fmt.Sprintf("%s · %s", title, cg.CageCode),
				Subtitle:    sub,
				Severity:    sev,
				SourceID:    cg.ID,
				SourceType:  "cage",
				MilestoneID: ms.ID,
				Done:        ms.Done,
				ActionLabel: "打开笼卡",
			})
		}
	}
	return out
}

func parseYMD(raw string) (time.Time, error) {
	s := strings.TrimSpace(raw)
	if len(s) >= 10 {
		s = s[:10]
	}
	return time.Parse("2006-01-02", s)
}
