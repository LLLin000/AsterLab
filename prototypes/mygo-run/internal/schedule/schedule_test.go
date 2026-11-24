package schedule

import "testing"

func TestScheduleFilterSortAndUpcomingSummary(t *testing.T) {
	events := []Event{
		{ID: "1", Domain: DomainReagent, Kind: KindReagentPurchase, DateYMD: "2026-11-24", Title: "PBS 购入", Severity: SeverityInfo, SourceID: "r1"},
		{ID: "2", Domain: DomainVivarium, Kind: KindAnimalHarvest, DateYMD: "2026-11-26", Title: "SPF-M101 收样取材", Severity: SeverityWarning, SourceID: "v1"},
		{ID: "3", Domain: DomainReagent, Kind: KindReagentExpired, DateYMD: "2026-11-24", Title: "胰酶过期", Severity: SeverityDanger, SourceID: "r2"},
		{ID: "4", Domain: DomainVivarium, Kind: KindAnimalWeigh, DateYMD: "2026-12-20", Title: "远期称重", Severity: SeverityInfo, SourceID: "v1"},
		{ID: "5", Domain: DomainVivarium, Kind: KindAnimalDosing, DateYMD: "2026-11-25", Title: "已完成给药", Severity: SeverityInfo, SourceID: "v1", Done: true},
	}

	flt := DefaultFilter()
	flt.ShowReagent = false
	idx := IndexByDate(events, flt)
	if len(idx["2026-11-24"]) != 0 || len(idx["2026-11-26"]) != 1 {
		t.Fatalf("expected reagent events excluded cleanly, got idx=%+v", idx)
	}

	allIdx := IndexByDate(events, DefaultFilter())
	if len(allIdx["2026-11-24"]) != 2 || allIdx["2026-11-24"][0].Kind != KindReagentExpired {
		t.Fatalf("expected danger expired event sorted before purchase info on same date, got %+v", allIdx["2026-11-24"])
	}

	sum := SummarizeUpcoming(events, "2026-11-24", 7)
	if sum.ReagentExpired != 1 || sum.AnimalHarvests != 1 {
		t.Fatalf("unexpected upcoming summary counts: %+v", sum)
	}
	for _, ev := range sum.NextEvents {
		if ev.ID == "4" || ev.ID == "5" {
			t.Fatalf("did not expect out-of-horizon or done event in NextEvents: %+v", ev)
		}
	}
}
