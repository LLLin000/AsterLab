package asterdoc

import "fmt"

// Geometry is the fixed matrix a plate format describes. The document's
// format selects it; the document never supplies its own row/column counts.
type Geometry struct {
	Format  int
	Rows    int
	Columns int
}

var plateFormats = map[int]Geometry{
	6:   {Format: 6, Rows: 2, Columns: 3},
	12:  {Format: 12, Rows: 3, Columns: 4},
	24:  {Format: 24, Rows: 4, Columns: 6},
	48:  {Format: 48, Rows: 6, Columns: 8},
	96:  {Format: 96, Rows: 8, Columns: 12},
	384: {Format: 384, Rows: 16, Columns: 24},
}

func geometryFor(format int) (Geometry, bool) {
	if format == 0 {
		format = 96
	}
	g, ok := plateFormats[format]
	return g, ok
}

func rowLabel(index int) string {
	// A..Z then AA.. for formats beyond 26 rows; current formats stop at P.
	if index < 26 {
		return string(rune('A' + index))
	}
	return fmt.Sprintf("A%c", rune('A'+index-26))
}
