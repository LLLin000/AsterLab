package kit

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"asterlab-mygo-prototype/internal/theme"
)

func TestElevationDarkSpecularRimAndNoLiquidGlass(t *testing.T) {
	for _, glass := range []bool{true, false} {
		tok := theme.TokensFor(theme.PresetPaper, true, glass)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.DarkTheme())
			ui.Column(c).Fill().Padding(float32(tok.Spacing12)).Gap(float32(tok.Spacing8)).Children(func() {
				for _, lvl := range []Elevation{ElevationCard, ElevationFloating, ElevationModal} {
					box := ui.Box(c).Size(120, 48)
					ApplyElevation(box, tok, lvl)
					box.Children(func() {
						ui.Text(c, "elev").TextColor(tok.Text)
					})
				}
			})
		}, 320, 260)
		tt.Frame()
		if !tt.HasText("elev") {
			t.Fatalf("expected elevation tiles to render (glass=%v)", glass)
		}
	}
	if got := theme.ConcentricRadius(14, 4); got != 10 {
		t.Fatalf("expected ConcentricRadius(14,4)=10, got %.1f", got)
	}
	if got := theme.ConcentricRadius(12, 4); got != 8 {
		t.Fatalf("expected ConcentricRadius(12,4)=8, got %.1f", got)
	}
}

func TestDarkroomLightboxChromeZoomAndSwipeDismiss(t *testing.T) {
	open := true
	st := DarkroomLightboxState{Zoom: 1.0}
	actionFired := false

	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightTheme())
		tok := theme.GetTokens(c.Theme())
		ui.Box(c).Fill().Children(func() {
			DarkroomLightbox(c, tok, &open, &st, DarkroomLightboxProps{
				Key:         "test-lb",
				Title:       "HUVEC 共定位成像",
				FormatBadge: "DNG",
				MetaSummary: "ISO 400 · 1/125 s · 5600K",
				Caption:     "孔 A1 · 10 uM",
				Actions: []DarkroomLightboxAction{
					{ID: "ins", Label: "插入实验", Primary: true, OnClick: func() { actionFired = true }},
				},
			})
		})
	}, 402, 874)

	tt.Frame()
	if !tt.HasText("HUVEC 共定位成像") || !tt.HasText("100%") || !tt.HasText("插入实验") {
		t.Fatalf("expected lightbox title, zoom pill and action visible, texts=%v", tt.Texts())
	}

	// Click footer action
	if err := tt.Click("插入实验"); err != nil {
		t.Fatalf("click footer action: %v", err)
	}
	tt.Frame()
	if !actionFired {
		t.Fatalf("expected footer action callback to fire")
	}

	// Single tap on canvas toggles ChromeHidden
	tt.ClickAt(201, 420)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if !st.ChromeHidden {
		t.Fatalf("expected single tap on canvas to hide chrome")
	}
	// Tap again restores chrome
	tt.ClickAt(201, 420)
	tt.Advance(200 * time.Millisecond)
	tt.Frame()
	if st.ChromeHidden {
		t.Fatalf("expected second tap on canvas to restore chrome")
	}

	// Double-click toggles 1.0x <-> 2.0x zoom
	tt.ClickAt(201, 420)
	tt.Advance(40 * time.Millisecond)
	tt.ClickAt(201, 420)
	tt.Frame()
	if st.Zoom != 2.0 {
		t.Fatalf("expected double-click to zoom to 2.0x, got %.2f", st.Zoom)
	}

	// Double-click again (after double-click window resets) resets back to 1.0x
	tt.Advance(600 * time.Millisecond)
	tt.ClickAt(201, 420)
	tt.Advance(40 * time.Millisecond)
	tt.ClickAt(201, 420)
	tt.Frame()
	if st.Zoom != 1.0 {
		t.Fatalf("expected second double-click to reset zoom to 1.0x, got %.2f", st.Zoom)
	}

	// Vertical swipe down past dismiss threshold closes *open = false
	tt.Press(201, 300)
	tt.Move(201, 360)
	tt.Frame()
	tt.Move(201, 480)
	tt.Frame()
	tt.Release(201, 480)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if open {
		t.Fatalf("expected vertical swipe-down release past threshold to close lightbox, dragOffset=%+v", st.DragOffset)
	}
}

func TestContactSheetGridBatchesBadgesSelectionAndDarkroom(t *testing.T) {
	selected := map[string]bool{"item-1": true}
	var clickedID string
	var batchActionFired bool

	batches := []ContactSheetBatch{
		{
			ID:          "b1",
			Title:       "2026-11-27 · B20261127-01",
			ActionLabel: "选择本批",
			OnAction:    func() { batchActionFired = true },
			Items: []ContactSheetItem{
				{ID: "item-1", Title: "样本 A1", FormatBadge: "DNG", RoleBadge: "A1"},
				{ID: "item-2", Title: "样本 A2", FormatBadge: "RAW+JPG", RoleBadge: "A2"},
			},
		},
	}

	for _, darkroom := range []bool{true, false} {
		clickedID = ""
		batchActionFired = false
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightTheme())
			tok := theme.GetTokens(c.Theme())
			ContactSheetGrid(c, tok, batches, ContactSheetOptions{
				Darkroom:    darkroom,
				SelectedIDs: selected,
				SelectionOn: true,
				OnItemClick: func(it ContactSheetItem) {
					clickedID = it.ID
				},
			})
		}, 402, 600)
		tt.Frame()

		if !tt.HasText("2026-11-27 · B20261127-01") || !tt.HasText("2 张") {
			t.Fatalf("expected batch header & tnum count (darkroom=%v), texts=%v", darkroom, tt.Texts())
		}
		if !tt.HasText("A1 · DNG") || !tt.HasText("A2 · RAW+JPG") {
			t.Fatalf("expected corner role/format badges (darkroom=%v), texts=%v", darkroom, tt.Texts())
		}
		if err := tt.Click("选择本批"); err != nil {
			t.Fatalf("click batch action (darkroom=%v): %v", darkroom, err)
		}
		tt.Frame()
		if !batchActionFired {
			t.Fatalf("expected batch action to fire (darkroom=%v)", darkroom)
		}
		if err := tt.Click("样本 A2"); err != nil {
			t.Fatalf("click item-2 (darkroom=%v): %v", darkroom, err)
		}
		tt.Frame()
		if clickedID != "item-2" {
			t.Fatalf("expected OnItemClick item-2, got %q (darkroom=%v)", clickedID, darkroom)
		}
	}
}
