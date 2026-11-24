package kit

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"asterlab-mygo-prototype/internal/theme"
)

// ToastTone classifies toast severity semantics.
type ToastTone uint8

const (
	ToastInfo ToastTone = iota
	ToastSuccess
	ToastWarning
	ToastDanger
)

// Toast describes one optimistic non-blocking feedback item with optional undo callback.
type Toast struct {
	ID        string
	Tone      ToastTone
	Message   string
	UndoLabel string
	OnUndo    func()
	Duration  time.Duration
	createdAt time.Time
}

// ToastQueue manages Linear-style stacked optimistic notifications & undo snackbars.
type ToastQueue struct {
	items []Toast
	seq   int
}

// NewToastQueue creates an empty ToastQueue.
func NewToastQueue() *ToastQueue {
	return &ToastQueue{}
}

// Push appends a toast notification, defaulting Duration to 5s, and caps queue length at 6.
func (q *ToastQueue) Push(t Toast) string {
	if q == nil {
		return ""
	}
	q.seq++
	if t.ID == "" {
		t.ID = fmt.Sprintf("toast-%d", q.seq)
	}
	if t.Duration <= 0 {
		t.Duration = 5 * time.Second
	}
	if t.createdAt.IsZero() {
		t.createdAt = time.Now()
	}
	if t.UndoLabel == "" && t.OnUndo != nil {
		t.UndoLabel = "撤销"
	}
	q.items = append(q.items, t)
	if len(q.items) > 6 {
		q.items = q.items[len(q.items)-6:]
	}
	return t.ID
}

// Dismiss removes a toast by ID immediately.
func (q *ToastQueue) Dismiss(id string) {
	if q == nil {
		return
	}
	kept := q.items[:0]
	for _, it := range q.items {
		if it.ID != id {
			kept = append(kept, it)
		}
	}
	q.items = kept
}

// Prune drops expired toasts whose createdAt + Duration <= now.
func (q *ToastQueue) Prune(now time.Time) {
	if q == nil || len(q.items) == 0 {
		return
	}
	kept := q.items[:0]
	for _, it := range q.items {
		if now.Sub(it.createdAt) < it.Duration {
			kept = append(kept, it)
		}
	}
	q.items = kept
}

// Len returns the count of active queued toasts.
func (q *ToastQueue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.items)
}

// Items returns a copy of queued toasts (for tests or inspection).
func (q *ToastQueue) Items() []Toast {
	if q == nil {
		return nil
	}
	out := make([]Toast, len(q.items))
	copy(out, q.items)
	return out
}

// Render paints up to 3 active toasts stacked at the bottom-right corner without blocking background input.
func (q *ToastQueue) Render(c *ui.Context, tok theme.Tokens) {
	if q == nil || c == nil {
		return
	}
	q.Prune(time.Now())
	if len(q.items) == 0 {
		return
	}
	visible := q.items
	if len(visible) > 3 {
		visible = visible[len(visible)-3:]
	}
	stack := ui.Column(c.Key("kit-toast-stack")).
		Absolute().Bottom(float32(tok.Spacing24)).Right(float32(tok.Spacing24)).
		Width(float32(tok.Spacing48) * 7.6).
		Gap(float32(tok.Spacing8))
	stack.Children(func() {
		for _, it := range visible {
			it := it
			bg, fg, icon := tok.Surface, tok.Accent, lucide.Info()
			switch it.Tone {
			case ToastSuccess:
				fg, icon = tok.Success, lucide.CircleCheck()
			case ToastWarning:
				fg, icon = tok.Warning, lucide.TriangleAlert()
			case ToastDanger:
				fg, icon = tok.Danger, lucide.CircleAlert()
			}
			card := ui.Row(c.Key("toast-" + it.ID)).
				FillWidth().
				Padding(float32(tok.Spacing8), float32(tok.Spacing12)).
				AlignItems(ui.Center).
				Gap(float32(tok.Spacing8))
			ApplyElevation(card, tok, ElevationFloating)
			if !tok.LiquidGlass {
				card.Background(bg)
			}
			card.Children(func() {
				ui.Icon(c, icon).Size(float32(tok.IconSm), float32(tok.IconSm)).TextColor(fg).Shrink(0)
				ui.Text(c, it.Message).FontSize(float32(tok.TextData)).TextColor(tok.Text).Grow(1).MinWidth(0)
				if it.OnUndo != nil {
					lbl := it.UndoLabel
					if lbl == "" {
						lbl = "撤销"
					}
					if components.Button(c, lbl, components.ButtonSecondary).Clicked() {
						fn := it.OnUndo
						id := it.ID
						q.Dismiss(id)
						fn()
					}
				}
				if components.IconButton(c, lucide.X(), "关闭提示", components.IconButtonCompact).Clicked() {
					q.Dismiss(it.ID)
				}
			})
		}
	})
}
