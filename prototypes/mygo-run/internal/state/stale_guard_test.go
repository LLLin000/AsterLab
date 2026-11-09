package state

import (
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestStaleGuardAcceptsNewer(t *testing.T) {
	g := &StaleGuard{}
	if !g.Accept(5) {
		t.Fatal("expected accept")
	}
	if g.Latest() != 5 {
		t.Fatalf("latest=%d", g.Latest())
	}
	if g.Accept(3) {
		t.Fatal("expected reject older")
	}
	if !g.Accept(5) {
		t.Fatal("expected accept equal")
	}
}

func TestApplySnapshotStaleIgnored(t *testing.T) {
	cur := runs.RunSnapshot{Revision: 2}
	in := runs.RunSnapshot{Revision: 1}
	got, err := ApplySnapshot(cur, in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 2 {
		t.Fatalf("stale not ignored: %d", got.Revision)
	}
	got2, _ := ApplySnapshot(cur, runs.RunSnapshot{Revision: 3})
	if got2.Revision != 3 {
		t.Fatalf("newer not applied: %d", got2.Revision)
	}
}

func TestConflictIfDifferent(t *testing.T) {
	a := runs.RunSnapshot{RunID: "r", Revision: 1, Status: "x"}
	b := runs.RunSnapshot{RunID: "r", Revision: 1, Status: "y"}
	if err := ConflictIfDifferent(a, b); err == nil {
		t.Fatal("expected conflict")
	}
}