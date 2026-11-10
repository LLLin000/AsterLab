package resultgrid

import (
	"math"
	"testing"
)

// TestSummarizeCountsMissingAndExcluded guards the Review derivation: only
// recorded technical replicates feed the mean and SD, while missing and
// excluded cells stay visible as counts.
func TestSummarizeCountsMissingAndExcluded(t *testing.T) {
	v1, v2, v3 := 0.8, 1.0, 0.2
	g := &Grid{Schema: Schema{ID: "cck8", Version: 2}, Cells: []Cell{
		{SampleID: "S1", Group: "Control", Field: "OD450", Value: &v1, Status: "raw"},
		{SampleID: "S1", Group: "Control", Field: "OD450", Value: nil, Status: "missing"},
		{SampleID: "S1", Group: "Control", Field: "OD450", Value: &v2, Status: "raw"},
		{SampleID: "S1", Group: "Control", Field: "OD450", Value: &v3, Status: "excluded"},
	}}
	rev := Summarize(g)
	if rev.SchemaID != "cck8" || rev.SchemaVersion != 2 || len(rev.Rows) != 1 {
		t.Fatalf("review = %+v", rev)
	}
	row := rev.Rows[0]
	if row.Recorded != 2 || row.Missing != 1 || row.Excluded != 1 {
		t.Fatalf("counts = %+v", row)
	}
	if math.Abs(row.Mean-0.9) > 1e-9 || math.Abs(row.SD-0.141421356) > 1e-6 {
		t.Fatalf("mean/sd = %v/%v", row.Mean, row.SD)
	}
}
