package vivarium

import (
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/schedule"
)

func TestVivariumWelfareSplitAndCalendarProjection(t *testing.T) {
	now := time.Date(2026, 11, 24, 9, 0, 0, 0, time.Local)
	cg := CageInput{
		ID:             "viv-1",
		CageCode:       "SPF-M101",
		Species:        "mouse_white",
		Strain:         "BALB/c",
		Room:           "屏障 302",
		ArrivedOn:      "2026-11-17",
		QuarantineDays: 7,
		IACUCNo:        "IACUC-2026-088",
		IACUCExpiresOn: "2026-12-10",
		Status:         "in_study",
		MaleCount:      4,
		FemaleCount:    2,
		WeightG:        19.0, // down 24% from peak 25.0
		WeightHistory: []WeightPoint{
			{Date: "2026-11-17", WeightG: 25.0},
			{Date: "2026-11-24", WeightG: 19.0},
		},
		Milestones: []Milestone{
			{ID: "m1", Kind: MilestoneHarvest, DateYMD: "2026-11-28", Title: "肿瘤取材 + 血清冻存"},
		},
	}

	rep := AssessWelfare(cg)
	if rep.DensityWarning == "" || !rep.HumaneEndpoint || rep.WeightDropPct < 20 {
		t.Fatalf("expected both GB 14925 density warning & >=20%% humane endpoint alert, got %+v", rep)
	}

	upd, child, err := ComputeSplit(cg, "SPF-M102", 0, 2)
	if err != nil || upd.TotalAnimals() != 4 || child.TotalAnimals() != 2 || child.IACUCNo != cg.IACUCNo {
		t.Fatalf("unexpected ComputeSplit: upd=%+v child=%+v err=%v", upd, child, err)
	}

	evs := ProjectCalendarEvents([]CageInput{cg}, now)
	kinds := map[schedule.EventKind]string{}
	for _, ev := range evs {
		kinds[ev.Kind] = ev.DateYMD
	}
	if kinds[schedule.KindAnimalArrival] != "2026-11-17" ||
		kinds[schedule.KindQuarantineEnd] != "2026-11-24" ||
		kinds[schedule.KindAnimalHarvest] != "2026-11-28" ||
		kinds[schedule.KindIACUCExpiry] != "2026-12-10" {
		t.Fatalf("unexpected vivarium calendar projection: %+v", kinds)
	}
}
