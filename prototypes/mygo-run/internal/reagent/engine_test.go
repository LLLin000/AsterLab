package reagent

import (
	"testing"
	"time"

	"asterlab-mygo-prototype/internal/schedule"
)

func TestReagentLifecycleDualExpiryAndCalendarProjection(t *testing.T) {
	now := time.Date(2026, 11, 24, 9, 0, 0, 0, time.Local)
	unopened := uint32(365)
	opened := uint32(14)

	it := LifecycleInput{
		ID:                  "rg-1",
		Name:                "Trypsin-EDTA (0.25%)",
		Location:            "-20°C 酶盒",
		PurchasedOn:         "2026-10-01",
		OpenedOn:            "2026-11-15",
		RestockBy:           "2026-11-28",
		ShelfLifeDays:       &unopened,
		OpenedShelfLifeDays: &opened,
		RemainingQty:        5.0,
		LowStockThreshold:   10.0,
		QtyUnit:             "mL",
	}

	lc := EvaluateLifecycle(it, now)
	if lc.ExpiresOn != "2026-11-29" || !lc.IsNearExpiry || lc.IsExpired || !lc.IsLowStock {
		t.Fatalf("unexpected lifecycle status: %+v", lc)
	}

	rem := it.RemainingQty
	var ledger []StockTx
	ApplyTransaction(&rem, &ledger, StockTx{DateYMD: "2026-11-24", Kind: TxOut, Delta: -2.5, Unit: "mL", ExperimentRef: "HUVEC"})
	if rem != 2.5 || len(ledger) != 1 || ledger[0].Kind != TxOut {
		t.Fatalf("unexpected ApplyTransaction result: rem=%.2f ledger=%+v", rem, ledger)
	}

	evs := ProjectCalendarEvents([]LifecycleInput{it}, now)
	kinds := map[schedule.EventKind]string{}
	for _, ev := range evs {
		kinds[ev.Kind] = ev.DateYMD
	}
	if kinds[schedule.KindReagentPurchase] != "2026-10-01" ||
		kinds[schedule.KindReagentOpened] != "2026-11-15" ||
		kinds[schedule.KindReagentRestock] != "2026-11-28" ||
		kinds[schedule.KindReagentExpirySoon] != "2026-11-29" {
		t.Fatalf("unexpected projected reagent calendar events: %+v", kinds)
	}
}
