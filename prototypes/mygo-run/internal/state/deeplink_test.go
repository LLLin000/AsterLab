package state

import "testing"

func TestParseRunWellLinkValid(t *testing.T) {
	link, err := ParseRunWellLink("asterlab://run/run-7/well/H12")
	if err != nil {
		t.Fatal(err)
	}
	if link.RunID != "run-7" || link.Well != "H12" {
		t.Fatalf("got %+v", link)
	}
	link, err = ParseRunWellLink("  ASTERLAB://RUN/run-7/well/a1  ")
	if err != nil {
		t.Fatal(err)
	}
	if link.RunID != "run-7" || link.Well != "A1" {
		t.Fatalf("normalization failed: %+v", link)
	}
}

func TestParseRunWellLinkInvalid(t *testing.T) {
	for _, raw := range []string{
		"",
		"not a link",
		"http://run/r1/well/A1",
		"asterlab://design/d1/well/A1",
		"asterlab://run//well/A1",
		"asterlab://run/r1/A1",
		"asterlab://run/r1/well/",
		"asterlab://run/r1/well/A0",
		"asterlab://run/r1/well/1A",
		"asterlab://run/r1/well/A1/extra",
	} {
		if _, err := ParseRunWellLink(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestFindWellCell(t *testing.T) {
	rows := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	cols := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}
	row, col, ok := FindWellCell("A1", rows, cols)
	if !ok || row != 0 || col != 0 {
		t.Fatalf("A1 -> (%d,%d,%v)", row, col, ok)
	}
	row, col, ok = FindWellCell("h12", rows, cols)
	if !ok || row != 7 || col != 11 {
		t.Fatalf("h12 -> (%d,%d,%v)", row, col, ok)
	}
	if _, _, ok = FindWellCell("Z99", rows, cols); ok {
		t.Fatal("expected no match for Z99")
	}
}
