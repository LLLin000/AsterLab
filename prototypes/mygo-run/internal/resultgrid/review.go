package resultgrid

import (
	"math"
	"sort"
)

// ReviewRow is one sample's recorded values for one field. Values are technical
// replicates only: their count is never an independent biological n.
type ReviewRow struct {
	SampleID string
	Group    string
	Field    string
	Recorded int
	Missing  int
	Excluded int
	Mean     float64
	SD       float64
}

// Review is a read-only derivation over a grid. It never writes cells.
type Review struct {
	SchemaID      string
	SchemaVersion int
	Rows          []ReviewRow
}

// Summarize derives per-sample recorded counts, mean and sample standard
// deviation for each field. Missing and excluded cells are counted, never
// treated as zero. The result is a projection: the grid remains the raw source.
func Summarize(g *Grid) Review {
	out := Review{}
	if g == nil {
		return out
	}
	out.SchemaID, out.SchemaVersion = g.Schema.ID, g.Schema.Version
	type key struct{ sample, field string }
	groups := map[key]*ReviewRow{}
	values := map[key][]float64{}
	for _, cell := range g.Cells {
		k := key{cell.SampleID, cell.Field}
		row, ok := groups[k]
		if !ok {
			row = &ReviewRow{SampleID: cell.SampleID, Group: cell.Group, Field: cell.Field}
			groups[k] = row
		}
		if row.Group == "" {
			row.Group = cell.Group
		}
		switch {
		case cell.Status == "excluded":
			row.Excluded++
		case cell.Value == nil:
			if cell.Status == "missing" || cell.Status == "" {
				row.Missing++
			}
			// "planned" and similar statuses are neither recorded nor missing.
		default:
			values[k] = append(values[k], *cell.Value)
		}
	}
	for k, row := range groups {
		vals := values[k]
		row.Recorded = len(vals)
		if len(vals) > 0 {
			var sum float64
			for _, v := range vals {
				sum += v
			}
			row.Mean = sum / float64(len(vals))
			if len(vals) > 1 {
				var sq float64
				for _, v := range vals {
					d := v - row.Mean
					sq += d * d
				}
				row.SD = math.Sqrt(sq / float64(len(vals)-1))
			}
		}
		out.Rows = append(out.Rows, *row)
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		if out.Rows[i].Field != out.Rows[j].Field {
			return out.Rows[i].Field < out.Rows[j].Field
		}
		if out.Rows[i].Group != out.Rows[j].Group {
			return out.Rows[i].Group < out.Rows[j].Group
		}
		return out.Rows[i].SampleID < out.Rows[j].SampleID
	})
	return out
}
