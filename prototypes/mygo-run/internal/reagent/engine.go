package reagent

import (
	"fmt"
	"strings"
	"time"

	"asterlab-mygo-prototype/internal/schedule"
)

// TxKind classifies inventory ledger movements for a reagent batch.
type TxKind string

const (
	TxIn      TxKind = "in"
	TxOut     TxKind = "out"
	TxAliquot TxKind = "aliquot"
	TxDiscard TxKind = "discard"
)

// StockTx records one immutable stock入库/领用/分装 transaction on a reagent.
type StockTx struct {
	ID            string  `json:"id"`
	DateYMD       string  `json:"dateYMD"`
	Kind          TxKind  `json:"kind"`
	Delta         float64 `json:"delta"`
	Unit          string  `json:"unit,omitempty"`
	ExperimentRef string  `json:"experimentRef,omitempty"`
	Note          string  `json:"note,omitempty"`
}

// LifecycleInput captures all persisted date & quantity fields needed for
// pure reagent expiry evaluation and calendar event projection.
type LifecycleInput struct {
	ID                  string
	Name                string
	Location            string
	PurchasedOn         string
	OpenedOn            string
	ExpiresOn           string
	RestockBy           string
	Status              string
	ShelfLifeDays       *uint32
	OpenedShelfLifeDays *uint32
	Exhausted           bool
	RemainingQty        float64
	LowStockThreshold   float64
	QtyUnit             string
}

// LifecycleStatus holds computed dual-expiry and low-stock flags.
type LifecycleStatus struct {
	ExpiresOn     string
	DaysRemaining *int
	IsNearExpiry  bool
	IsExpired     bool
	IsLowStock    bool
}

const nearExpiryWindowDays = 30

// EvaluateLifecycle computes effective expiration date (earlier of unopened vs opened shelf life)
// and stock-alert flags relative to today.
func EvaluateLifecycle(in LifecycleInput, today time.Time) LifecycleStatus {
	todayDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	var out LifecycleStatus
	if in.LowStockThreshold > 0 && in.RemainingQty <= in.LowStockThreshold && !in.Exhausted {
		out.IsLowStock = true
	}

	var candidates []time.Time
	if t, err := parseYMD(in.ExpiresOn); err == nil {
		candidates = append(candidates, t)
	} else if in.ShelfLifeDays != nil && *in.ShelfLifeDays > 0 {
		if p, pErr := parseYMD(in.PurchasedOn); pErr == nil {
			candidates = append(candidates, p.AddDate(0, 0, int(*in.ShelfLifeDays)))
		}
	}
	if in.OpenedShelfLifeDays != nil && *in.OpenedShelfLifeDays > 0 {
		if o, oErr := parseYMD(in.OpenedOn); oErr == nil {
			candidates = append(candidates, o.AddDate(0, 0, int(*in.OpenedShelfLifeDays)))
		}
	}
	if len(candidates) > 0 {
		earliest := candidates[0]
		for _, c := range candidates[1:] {
			if c.Before(earliest) {
				earliest = c
			}
		}
		out.ExpiresOn = earliest.Format("2006-01-02")
		days := int(earliest.Sub(todayDay).Hours() / 24)
		out.DaysRemaining = &days
		if !in.Exhausted {
			if days < 0 || strings.EqualFold(in.Status, "expired") {
				out.IsExpired = true
			} else if days <= nearExpiryWindowDays || strings.EqualFold(in.Status, "near_expiry") {
				out.IsNearExpiry = true
			}
		}
	}
	return out
}

// ApplyTransaction applies one StockTx onto *remaining (clamped >= 0) and appends to *ledger.
func ApplyTransaction(remaining *float64, ledger *[]StockTx, tx StockTx) StockTx {
	if tx.ID == "" {
		tx.ID = fmt.Sprintf("rtx-%d", time.Now().UnixNano())
	}
	if strings.TrimSpace(tx.DateYMD) == "" {
		tx.DateYMD = time.Now().Format("2006-01-02")
	}
	if remaining != nil {
		next := *remaining + tx.Delta
		if next < 0 {
			next = 0
		}
		*remaining = next
	}
	if ledger != nil {
		*ledger = append(*ledger, tx)
	}
	return tx
}

// TxKindLabel returns localized Chinese text for a stock transaction kind.
func TxKindLabel(k TxKind) string {
	switch k {
	case TxIn:
		return "入库"
	case TxOut:
		return "领用"
	case TxAliquot:
		return "分装"
	case TxDiscard:
		return "报废/核销"
	default:
		return "变动"
	}
}

// ProjectCalendarEvents derives read-only schedule.Event items from reagent batches
// covering purchase, open date, restock date, and near-expiry / expiration alerts.
func ProjectCalendarEvents(items []LifecycleInput, today time.Time) []schedule.Event {
	out := make([]schedule.Event, 0, len(items)*2)
	for _, it := range items {
		loc := strings.TrimSpace(it.Location)
		if pDate, err := parseYMD(it.PurchasedOn); err == nil {
			out = append(out, schedule.Event{
				ID:          "reagent-buy-" + it.ID,
				Domain:      schedule.DomainReagent,
				Kind:        schedule.KindReagentPurchase,
				DateYMD:     pDate.Format("2006-01-02"),
				Title:       fmt.Sprintf("购入 · %s", it.Name),
				Subtitle:    loc,
				Severity:    schedule.SeverityInfo,
				SourceID:    it.ID,
				SourceType:  "reagent",
				ActionLabel: "查看试剂",
			})
		}
		if oDate, err := parseYMD(it.OpenedOn); err == nil && it.OpenedOn != it.PurchasedOn {
			out = append(out, schedule.Event{
				ID:          "reagent-open-" + it.ID,
				Domain:      schedule.DomainReagent,
				Kind:        schedule.KindReagentOpened,
				DateYMD:     oDate.Format("2006-01-02"),
				Title:       fmt.Sprintf("开封 · %s", it.Name),
				Subtitle:    loc,
				Severity:    schedule.SeverityInfo,
				SourceID:    it.ID,
				SourceType:  "reagent",
				ActionLabel: "查看试剂",
			})
		}
		if rDate, err := parseYMD(it.RestockBy); err == nil {
			out = append(out, schedule.Event{
				ID:          "reagent-restock-" + it.ID,
				Domain:      schedule.DomainReagent,
				Kind:        schedule.KindReagentRestock,
				DateYMD:     rDate.Format("2006-01-02"),
				Title:       fmt.Sprintf("补货 · %s", it.Name),
				Subtitle:    loc,
				Severity:    schedule.SeverityWarning,
				SourceID:    it.ID,
				SourceType:  "reagent",
				Done:        it.Exhausted,
				ActionLabel: "处理补货",
			})
		}

		lc := EvaluateLifecycle(it, today)
		if lc.ExpiresOn != "" && !it.Exhausted {
			kind := schedule.KindReagentExpirySoon
			sev := schedule.SeverityWarning
			title := fmt.Sprintf("效期提醒 · %s", it.Name)
			if lc.IsExpired {
				kind = schedule.KindReagentExpired
				sev = schedule.SeverityDanger
				title = fmt.Sprintf("已过期 · %s", it.Name)
			} else if lc.IsNearExpiry {
				title = fmt.Sprintf("临期 (%d天) · %s", *lc.DaysRemaining, it.Name)
			} else {
				sev = schedule.SeverityInfo
			}
			out = append(out, schedule.Event{
				ID:          "reagent-exp-" + it.ID,
				Domain:      schedule.DomainReagent,
				Kind:        kind,
				DateYMD:     lc.ExpiresOn,
				Title:       title,
				Subtitle:    loc,
				Severity:    sev,
				SourceID:    it.ID,
				SourceType:  "reagent",
				ActionLabel: "检查库存",
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
