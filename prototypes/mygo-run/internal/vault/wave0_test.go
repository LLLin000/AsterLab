package vault

import (
	"testing"
)

func TestLiveDocumentSourceUndoCoalescing(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open vault: %v", err)
	}
	entry, err := v.CreateExperiment("proj", "UndoTest", "2026-11-20", "---\ntitle: UndoTest\n---\n\n# UndoTest\n")
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	ds := NewDocStore(v)
	ld, err := ds.Open(entry.RelPath)
	if err != nil {
		t.Fatalf("Open doc: %v", err)
	}
	orig := ld.Source
	// Rapid burst of 15 keystrokes should coalesce into a single undo snapshot.
	cur := orig
	for i := 0; i < 15; i++ {
		cur += "x"
		ld.ApplySourceDraft(cur)
	}
	if !ld.Undo() {
		t.Fatalf("expected Undo to succeed")
	}
	if ld.Source != orig {
		t.Fatalf("expected single coalesced Undo to restore original text, got %q", ld.Source)
	}
	if ld.CanUndo() {
		t.Fatalf("expected undo stack to be empty after coalesced burst undo")
	}
}

func TestVaultTrashListAndRestore(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatalf("Open vault: %v", err)
	}
	entry, err := v.CreateExperiment("proj", "TrashMe", "2026-11-20", "---\ntitle: TrashMe\n---\n\n# TrashMe\n")
	if err != nil {
		t.Fatalf("CreateExperiment: %v", err)
	}
	item, err := v.Trash(entry.RelPath)
	if err != nil {
		t.Fatalf("Trash: %v", err)
	}
	list := v.ListTrash()
	if len(list) != 1 || list[0].TrashName != item.TrashName {
		t.Fatalf("expected 1 item in ListTrash, got %+v", list)
	}
	restored, err := v.RestoreFromTrash(item.TrashName)
	if err != nil {
		t.Fatalf("RestoreFromTrash: %v", err)
	}
	if restored != entry.RelPath {
		t.Fatalf("expected restored rel=%q, got %q", entry.RelPath, restored)
	}
	if rem := v.ListTrash(); len(rem) != 0 {
		t.Fatalf("expected empty ListTrash after restore, got %+v", rem)
	}
}
