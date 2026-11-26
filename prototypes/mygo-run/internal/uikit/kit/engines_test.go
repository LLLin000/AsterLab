package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestLayerStackEscapeOrder(t *testing.T) {
	var stack LayerStack
	aOpen, bOpen, cOpen := true, true, false

	stack.BeginFrame()
	stack.Register("settings", aOpen, func() { aOpen = false })
	stack.Register("palette", bOpen, func() { bOpen = false })
	stack.Register("tour", cOpen, func() { cOpen = false })

	if stack.TopID() != "palette" {
		t.Fatalf("expected top layer palette, got %q", stack.TopID())
	}
	if !stack.DismissTop() || bOpen || !aOpen {
		t.Fatalf("expected DismissTop to close only palette: a=%v b=%v", aOpen, bOpen)
	}
	if stack.TopID() != "settings" {
		t.Fatalf("expected top layer settings after palette popped, got %q", stack.TopID())
	}
	if !stack.DismissTop() || aOpen || stack.HasOpen() {
		t.Fatalf("expected second DismissTop to close settings and empty stack: a=%v hasOpen=%v", aOpen, stack.HasOpen())
	}
}

func TestCommandRegistryWhenClausesAndOverrides(t *testing.T) {
	reg := NewCommandRegistry()
	reg.Register(CommandSpec{
		ID:              "doc.undo",
		Title:           "撤销",
		Category:        "编辑器",
		DefaultShortcut: "Ctrl+Z",
		When:            func(ctx CommandContext) bool { return ctx.IsFileTab && !ctx.TextInputFocused },
	})
	reg.Register(CommandSpec{
		ID:              "nav.calendar",
		Title:           "切换到实验日程",
		Category:        "导航",
		DefaultShortcut: "Ctrl+1",
	})
	reg.SetOverrides(map[string]string{"nav.calendar": "Alt+1"})

	if got := reg.ShortcutFor("nav.calendar"); got != "Alt+1" {
		t.Fatalf("expected override Alt+1, got %q", got)
	}
	if got := reg.ShortcutFor("doc.undo"); got != "Ctrl+Z" {
		t.Fatalf("expected default Ctrl+Z, got %q", got)
	}

	resTextBusy := reg.Search("", CommandContext{IsFileTab: true, TextInputFocused: true})
	if len(resTextBusy) != 1 || resTextBusy[0].ID != "nav.calendar" {
		t.Fatalf("expected doc.undo gated when TextInputFocused=true, got %+v", resTextBusy)
	}

	resAll := reg.Search("日程", CommandContext{IsFileTab: true, TextInputFocused: false})
	if len(resAll) != 1 || resAll[0].ID != "nav.calendar" {
		t.Fatalf("expected query 日程 to match nav.calendar, got %+v", resAll)
	}
}

func TestScrubNumberDeadzoneShiftEscapeAndCommit(t *testing.T) {
	val := 20.0
	st := &scrubState{}
	opts := ScrubOptions{Min: 0, Max: 100, Step: 1, Decimals: 1, Sensitivity: 5}

	// 1. Sub-3px pointer move stays pointing and enters inline editing on mouseup
	applyScrubDelta(st, ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Button: 0}, &val, opts)
	r1 := applyScrubDelta(st, ui.InputEvent{Kind: ui.InputPointerMove, X: 52}, &val, opts)
	if st.mode != scrubPointing || r1.Changed {
		t.Fatalf("expected <3px move to stay in deadzone without changing val, mode=%v changed=%v", st.mode, r1.Changed)
	}
	rUpEdit := applyScrubDelta(st, ui.InputEvent{Kind: ui.InputPointerUp, X: 52, Button: 0}, &val, opts)
	if st.mode != scrubEditing || rUpEdit.Committed {
		t.Fatalf("expected click within deadzone to enter inline editing without commit, mode=%v committed=%v", st.mode, rUpEdit.Committed)
	}

	// 2. Drag >= 3px enters scrubbing, applies Shift 0.1x precision, commits cleanly on PointerUp
	*st = scrubState{}
	applyScrubDelta(st, ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Button: 0}, &val, opts)
	rDrag := applyScrubDelta(st, ui.InputEvent{Kind: ui.InputPointerMove, X: 75, Mods: ui.Shift}, &val, opts) // dx=25 -> 25/5 * 0.1 = +0.5
	if st.mode != scrubDragging || !rDrag.Changed || val != 20.5 {
		t.Fatalf("expected Shift fine scrub to 20.5, mode=%v val=%.2f changed=%v", st.mode, val, rDrag.Changed)
	}
	rCommit := applyScrubDelta(st, ui.InputEvent{Kind: ui.InputPointerUp, X: 75, Button: 0}, &val, opts)
	if st.mode != scrubIdle || !rCommit.Committed {
		t.Fatalf("expected single atomic commit on PointerUp, mode=%v committed=%v", st.mode, rCommit.Committed)
	}

	// 3. Mobile 8px touch deadzone ignores micro-jitter (< 8px), then commits cleanly beyond 8px.
	*st = scrubState{}
	val = 20.0
	applyScrubDeltaDeadzone(st, ui.InputEvent{Kind: ui.InputPointerDown, X: 50, Button: 0}, &val, opts, 8)
	rJitter := applyScrubDeltaDeadzone(st, ui.InputEvent{Kind: ui.InputPointerMove, X: 56}, &val, opts, 8) // 6px < 8px
	if st.mode != scrubPointing || rJitter.Changed || val != 20.0 {
		t.Fatalf("expected 6px jitter inside 8px mobile deadzone to be ignored, mode=%v val=%.1f", st.mode, val)
	}
	rMobileDrag := applyScrubDeltaDeadzone(st, ui.InputEvent{Kind: ui.InputPointerMove, X: 65}, &val, opts, 8) // +15px / 5 = +3
	if st.mode != scrubDragging || !rMobileDrag.Changed || val != 23.0 {
		t.Fatalf("expected drag beyond 8px deadzone to scrub to 23.0, mode=%v val=%.1f", st.mode, val)
	}

	// 4. Inline edit mode shows ✓ confirm button and commits typed value cleanly.
	liveVal := 25.0
	var committed float64
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Padding(12).Children(func() {
			r := ScrubNumber(c, "mobile-scrub", tok, &liveVal, ScrubOptions{
				Label: "体积", Unit: "mL", Min: 0, Max: 500, Step: 1, Decimals: 0, Width: 180,
			})
			if r.Committed {
				committed = liveVal
			}
		})
	}, 402, 200)
	tt.Frame()
	if err := tt.Click("体积 25 mL"); err != nil {
		t.Fatalf("tap scrub pill to enter edit mode: %v", err)
	}
	tt.Frame()
	rOk, ok := tt.Find("确认")
	if !ok {
		t.Fatalf("expected inline 确认 button in scrub edit mode, texts=%v", tt.Texts())
	}
	// Focus the inline TextBox to the left of the confirm button before typing.
	tt.ClickAt(48, rOk.Y+rOk.H/2)
	tt.Frame()
	tt.Command("selectAll")
	tt.Type("42")
	tt.Frame()
	if err := tt.Click("确认"); err != nil {
		t.Fatalf("click inline 确认 button: %v", err)
	}
	tt.Frame()
	if liveVal != 42 || committed != 42 {
		t.Fatalf("expected inline confirm button to commit 42, liveVal=%.1f committed=%.1f", liveVal, committed)
	}
}

func TestToastQueuePushUndoAndPrune(t *testing.T) {
	q := NewToastQueue()
	undone := false
	now := time.Now()
	id := q.Push(Toast{
		Tone:      ToastInfo,
		Message:   "已删除笼卡 C-01",
		OnUndo:    func() { undone = true },
		Duration:  4 * time.Second,
		createdAt: now,
	})
	if q.Len() != 1 || q.Items()[0].UndoLabel != "撤销" {
		t.Fatalf("unexpected queued toast: %+v", q.Items())
	}
	q.Prune(now.Add(2 * time.Second))
	if q.Len() != 1 {
		t.Fatalf("expected toast still alive before expiration")
	}
	q.Items()[0].OnUndo()
	q.Dismiss(id)
	if !undone || q.Len() != 0 {
		t.Fatalf("expected undo + dismiss to clear queue, undone=%v len=%d", undone, q.Len())
	}
	q.Push(Toast{Message: "expired", Duration: time.Second, createdAt: now})
	q.Prune(now.Add(2 * time.Second))
	if q.Len() != 0 {
		t.Fatalf("expected expired toast pruned")
	}
}

func TestFilterBarAndSidePeekInteraction(t *testing.T) {
	var fs FilterBarState
	sv := SavedFilterView{
		ID:      "spf",
		Label:   "SPF",
		Filters: []ActiveFilter{{FieldKey: "grade", Value: "SPF"}},
	}
	fs.ApplySavedView(sv)
	if !fs.HasFilter("grade", "SPF") || fs.SavedViewID != "spf" {
		t.Fatalf("expected saved view applied, state=%+v", fs)
	}
	fs.ToggleFilter("status", "in_study")
	if len(fs.Active) != 2 || fs.SavedViewID != "" {
		t.Fatalf("expected custom pill to detach saved view and add condition, state=%+v", fs)
	}
	fs.ClearFilters()
	if len(fs.Active) != 0 {
		t.Fatalf("expected empty active filters after ClearFilters")
	}

	open := true
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		SidePeek(c, "peek-test", tok, &open, SidePeekOptions{Title: "侧滑笼卡 · C-01"}, func() {
			ui.Text(c, "BALB/c 品系详情")
		})
	}, 900, 600)
	tt.Frame()
	if !tt.HasText("侧滑笼卡 · C-01") || !tt.HasText("BALB/c 品系详情") {
		t.Fatalf("expected SidePeek body & header visible, texts=%v", tt.Texts())
	}
	if err := tt.Click("关闭侧滑面板"); err != nil {
		// Fallback to scrim area on left half
		tt.ClickAt(60, 300)
	}
	tt.Frame()
	if open || tt.HasText("BALB/c 品系详情") {
		t.Fatalf("expected SidePeek to close on button/scrim click, open=%v", open)
	}
}
