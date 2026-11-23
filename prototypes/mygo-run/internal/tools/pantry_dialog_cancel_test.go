package tools

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPantryDialogCancelDoesNotPanic reproduces the exact UI-15 flow:
// open Pantry box view -> open "新增储藏室试剂" inside a box -> attempt save -> click "取消"
// and tick through the full modal exit transition without panicking.
func TestPantryDialogCancelDoesNotPanic(t *testing.T) {
	w := New(Callbacks{})
	w.EnsurePantryBoxesLoaded()
	box := PantryBox{
		ID:      "box-test-1",
		Name:    "123",
		ZoneKey: "-80",
		Kind:    BoxCryo9x9,
		Rows:    9,
		Cols:    9,
	}
	w.PantryBoxes = append(w.PantryBoxes, box)
	w.OpenPantryBox(box.ID, "A1")
	w.OpenNewPantryDialogInBox(box, "A1")

	tt := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			View(c, w)
		})
	}, 1280, 820)

	// Open & stabilize enter animation
	for i := 0; i < 6; i++ {
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
	}
	if !w.PantryDialogOpen {
		t.Fatal("expected PantryDialogOpen to stay open before cancel")
	}

	// Simulate failed save error banner (like UI-15)
	w.pantryDialogErr = "保存试剂台账失败：access is denied"
	tt.Frame()

	// Click "取消" button inside the dialog
	tt.Click("取消")
	for i := 0; i < 12; i++ {
		tt.Frame()
		tt.Advance(30 * time.Millisecond)
	}
	if w.PantryDialogOpen {
		t.Fatal("expected PantryDialogOpen to be false after clicking 取消")
	}

	// Re-open and close Box dialog via "取消" as well
	w.OpenNewBoxDialog("-80")
	for i := 0; i < 6; i++ {
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
	}
	if !w.PantryBoxDialogOpen {
		t.Fatal("expected PantryBoxDialogOpen to stay open before cancel")
	}
	tt.Click("取消")
	for i := 0; i < 12; i++ {
		tt.Frame()
		tt.Advance(30 * time.Millisecond)
	}
	if w.PantryBoxDialogOpen {
		t.Fatal("expected PantryBoxDialogOpen to be false after clicking 取消")
	}
}
