// Package labio provides Pure-Go, stdlib-only scientific data import/export:
//   - CSV, TSV, and minimal XLSX plate-reader & tabular parsing (`ImportPlateMatrix`, `ParseTabularData`)
//   - Tidy long-format CSV export for GraphPad Prism, R, and Python (`ExportTidyCSV`)
//   - Standards-compliant OOXML `.docx` report generation (`ExportDocx`)
package labio

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TabularData represents a normalized 2D table parsed from CSV, TSV, or XLSX.
type TabularData struct {
	Format string     `json:"format"` // csv | tsv | xlsx
	Sheet  string     `json:"sheet,omitempty"`
	Rows   [][]string `json:"rows"`
}

// TabularOptions configures how raw CSV/TSV/XLSX bytes are parsed.
type TabularOptions struct {
	Format    string // optional hint: "csv", "tsv", "xlsx"; auto-detected when empty
	Delimiter rune   // optional override for delimited text
	SheetName string // optional worksheet name for XLSX (defaults to first sheet)
}

// PlateWellReading holds one parsed well value from a 96-well or 384-well import.
type PlateWellReading struct {
	Coord     string   `json:"coord"`
	Row       int      `json:"row"` // 0-based
	Col       int      `json:"col"` // 0-based
	Value     *float64 `json:"value,omitempty"`
	Raw       string   `json:"raw"`
	Group     string   `json:"group,omitempty"`
	Timepoint string   `json:"timepoint,omitempty"`
	Replicate int      `json:"replicate,omitempty"`
}

// PlateImportOptions configures plate matrix import.
type PlateImportOptions struct {
	TabularOptions
	ExpectedFormat int // 0 (auto), 96, or 384
}

// PlateImportResult contains the structured plate grid and ordered well readings.
type PlateImportResult struct {
	Format  int                         `json:"format"` // 96 or 384
	Rows    int                         `json:"rows"`
	Cols    int                         `json:"cols"`
	Wells   map[string]PlateWellReading `json:"wells"`
	Ordered []PlateWellReading          `json:"ordered"`
	Table   *TabularData                `json:"table,omitempty"`
}

// TidyObservation is one normalized long-format data row for Prism / R / Python.
type TidyObservation struct {
	Experiment string            `json:"experiment,omitempty"`
	Group      string            `json:"group"`
	Role       string            `json:"role,omitempty"`
	Timepoint  string            `json:"timepoint,omitempty"`
	Replicate  int               `json:"replicate,omitempty"`
	Well       string            `json:"well,omitempty"`
	SampleID   string            `json:"sampleId,omitempty"`
	Metric     string            `json:"metric"`
	Value      float64           `json:"value"`
	Missing    bool              `json:"missing,omitempty"`
	RawValue   string            `json:"rawValue,omitempty"`
	Unit       string            `json:"unit,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

// TidyCSVOptions configures ExportTidyCSV formatting.
type TidyCSVOptions struct {
	IncludeUTF8BOM bool
	Precision      int // decimal places for float64 (<=0 uses shortest representation 'g')
}

var wellCoordRe = regexp.MustCompile(`^([A-Pa-p])0?([1-9]|1[0-9]|2[0-4])$`)

// ParseTabularData parses CSV, TSV, or XLSX bytes into normalized string rows.
func ParseTabularData(data []byte, opts ...TabularOptions) (*TabularData, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("labio: empty tabular input")
	}
	var opt TabularOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	fmtHint := strings.ToLower(strings.TrimSpace(opt.Format))
	if fmtHint == "xlsx" || (fmtHint == "" && len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 0x03 && data[3] == 0x04) {
		return parseXLSX(data, opt.SheetName)
	}

	cleaned := bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	text := strings.ReplaceAll(string(cleaned), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	delim := opt.Delimiter
	detectedFmt := "csv"
	if delim == 0 {
		if fmtHint == "tsv" {
			delim = '\t'
			detectedFmt = "tsv"
		} else {
			delim = detectDelimiter(text)
			if delim == '\t' {
				detectedFmt = "tsv"
			}
		}
	} else if delim == '\t' {
		detectedFmt = "tsv"
	}

	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = false

	rawRows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("labio: parse %s: %w", detectedFmt, err)
	}
	rows := make([][]string, 0, len(rawRows))
	for _, row := range rawRows {
		if isAllEmptyCells(row) {
			continue
		}
		trimmed := make([]string, len(row))
		for i, c := range row {
			trimmed[i] = strings.TrimSpace(c)
		}
		rows = append(rows, trimmed)
	}
	if len(rows) == 0 {
		return nil, errors.New("labio: no non-empty rows found")
	}
	return &TabularData{
		Format: detectedFmt,
		Rows:   rows,
	}, nil
}

// ImportPlateMatrix parses a 96-well (8x12) or 384-well (16x24) plate-reader export
// from CSV, TSV, or XLSX — supporting labeled matrices (rows A..H/P, cols 1..12/24
// with optional instrument header rows), bare numeric grids, and long-table formats
// with a `Well` column.
func ImportPlateMatrix(data []byte, opts ...PlateImportOptions) (*PlateImportResult, error) {
	var opt PlateImportOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	tbl, err := ParseTabularData(data, opt.TabularOptions)
	if err != nil {
		return nil, err
	}

	// Strategy 1: Try labeled matrix detection (e.g. header row 1..12 + row labels A..H).
	if res, ok := tryParseLabeledMatrix(tbl, opt.ExpectedFormat); ok {
		return res, nil
	}

	// Strategy 2: Try long-table format with a "Well" column.
	if res, ok := tryParseLongWellTable(tbl, opt.ExpectedFormat); ok {
		return res, nil
	}

	// Strategy 3: Try bare unlabeled 8x12 or 16x24 numeric matrix.
	if res, ok := tryParseBareMatrix(tbl, opt.ExpectedFormat); ok {
		return res, nil
	}

	return nil, errors.New("labio: could not recognize 96-well or 384-well plate matrix or Well column")
}

// ExportTidyCSV writes normalized long-format observations as RFC-4180 CSV suitable
// for GraphPad Prism, R (tidyverse), and Python (pandas).
func ExportTidyCSV(rows []TidyObservation, opts ...TidyCSVOptions) ([]byte, error) {
	var opt TidyCSVOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	extraSet := map[string]struct{}{}
	for _, r := range rows {
		for k := range r.Extra {
			if strings.TrimSpace(k) != "" {
				extraSet[k] = struct{}{}
			}
		}
	}
	extraKeys := make([]string, 0, len(extraSet))
	for k := range extraSet {
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)

	var buf bytes.Buffer
	if opt.IncludeUTF8BOM {
		buf.Write([]byte{0xef, 0xbb, 0xbf})
	}
	w := csv.NewWriter(&buf)

	header := []string{"experiment", "group", "role", "timepoint", "replicate", "well", "sample_id", "metric", "value", "unit"}
	header = append(header, extraKeys...)
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, r := range rows {
		repStr := ""
		if r.Replicate > 0 {
			repStr = strconv.Itoa(r.Replicate)
		}
		valStr := ""
		if r.Missing {
			if r.RawValue != "" {
				valStr = r.RawValue
			} else {
				valStr = "NA"
			}
		} else if opt.Precision > 0 {
			valStr = strconv.FormatFloat(r.Value, 'f', opt.Precision, 64)
		} else {
			valStr = strconv.FormatFloat(r.Value, 'g', -1, 64)
		}

		rec := make([]string, 0, len(header))
		rec = append(rec,
			r.Experiment,
			r.Group,
			r.Role,
			r.Timepoint,
			repStr,
			r.Well,
			r.SampleID,
			r.Metric,
			valStr,
			r.Unit,
		)
		for _, ek := range extraKeys {
			rec = append(rec, r.Extra[ek])
		}
		if err := w.Write(rec); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func tryParseLabeledMatrix(tbl *TabularData, expectedFmt int) (*PlateImportResult, bool) {
	for rIdx, row := range tbl.Rows {
		if len(row) == 0 {
			continue
		}
		first := strings.TrimSpace(row[0])
		rowLetter, ok := parsePlateRowLetter(first)
		if !ok {
			continue
		}
		// Check if previous row was a numeric column header (1..12 or 1..24) to determine startCol.
		startCol := 1
		maxCols := 12
		if rIdx > 0 {
			prev := tbl.Rows[rIdx-1]
			if detectedCols, sCol, hasHdr := detectColumnHeaderRow(prev); hasHdr {
				startCol = sCol
				maxCols = detectedCols
			}
		}
		// Collect consecutive row-letter rows (A..H or A..P).
		var matrixRows []struct {
			rowIdx int
			cells  []string
		}
		for j := rIdx; j < len(tbl.Rows); j++ {
			cur := tbl.Rows[j]
			if len(cur) == 0 {
				break
			}
			rl, ok := parsePlateRowLetter(cur[0])
			if !ok {
				break
			}
			if len(cur)-startCol > maxCols {
				maxCols = len(cur) - startCol
			}
			var vals []string
			if startCol < len(cur) {
				vals = cur[startCol:]
			}
			matrixRows = append(matrixRows, struct {
				rowIdx int
				cells  []string
			}{rowIdx: rl, cells: vals})
		}
		if len(matrixRows) < 2 {
			continue
		}
		// Check if this is at least a plate-like slice starting at row A (0).
		if rowLetter != 0 && len(matrixRows) < 4 {
			continue
		}
		format, rowsN, colsN := inferPlateDimensions(expectedFmt, maxRowIndex(matrixRows)+1, maxCols)
		res := newPlateResult(format, rowsN, colsN, tbl)
		for _, mr := range matrixRows {
			if mr.rowIdx >= rowsN {
				continue
			}
			for c := 0; c < len(mr.cells) && c < colsN; c++ {
				setPlateWell(res, mr.rowIdx, c, mr.cells[c], "", "", 0)
			}
		}
		finalizePlateOrder(res)
		return res, true
	}
	return nil, false
}

func tryParseLongWellTable(tbl *TabularData, expectedFmt int) (*PlateImportResult, bool) {
	if len(tbl.Rows) < 2 {
		return nil, false
	}
	hdr := tbl.Rows[0]
	wellCol, valCol, grpCol, tpCol, repCol := -1, -1, -1, -1, -1
	for i, h := range hdr {
		lh := strings.ToLower(strings.TrimSpace(h))
		switch lh {
		case "well", "coord", "coordinate", "position", "孔位":
			if wellCol == -1 {
				wellCol = i
			}
		case "value", "od", "od450", "ct", "readout", "intensity", "signal", "读数", "数值":
			if valCol == -1 {
				valCol = i
			}
		case "group", "sample", "condition", "分组":
			if grpCol == -1 {
				grpCol = i
			}
		case "timepoint", "time", "时间点":
			if tpCol == -1 {
				tpCol = i
			}
		case "replicate", "rep", "复孔":
			if repCol == -1 {
				repCol = i
			}
		}
	}
	if wellCol < 0 {
		return nil, false
	}
	if valCol < 0 {
		// Fall back to the first non-well column that parses numerically in data rows.
		for c := 0; c < len(hdr); c++ {
			if c == wellCol || c == grpCol || c == tpCol || c == repCol {
				continue
			}
			if _, err := parseFloatCell(tbl.Rows[1][c]); err == nil {
				valCol = c
				break
			}
		}
	}
	if valCol < 0 {
		return nil, false
	}

	type parsedItem struct {
		r, c int
		raw  string
		grp  string
		tp   string
		rep  int
	}
	var items []parsedItem
	maxR, maxC := 0, 0
	for _, row := range tbl.Rows[1:] {
		if wellCol >= len(row) {
			continue
		}
		r, c, ok := parseWellCoord(row[wellCol])
		if !ok {
			continue
		}
		if r > maxR {
			maxR = r
		}
		if c > maxC {
			maxC = c
		}
		rawVal := ""
		if valCol < len(row) {
			rawVal = row[valCol]
		}
		var grp, tp string
		var rep int
		if grpCol >= 0 && grpCol < len(row) {
			grp = row[grpCol]
		}
		if tpCol >= 0 && tpCol < len(row) {
			tp = row[tpCol]
		}
		if repCol >= 0 && repCol < len(row) {
			rep, _ = strconv.Atoi(strings.TrimSpace(row[repCol]))
		}
		items = append(items, parsedItem{r: r, c: c, raw: rawVal, grp: grp, tp: tp, rep: rep})
	}
	if len(items) == 0 {
		return nil, false
	}
	format, rowsN, colsN := inferPlateDimensions(expectedFmt, maxR+1, maxC+1)
	res := newPlateResult(format, rowsN, colsN, tbl)
	for _, it := range items {
		if it.r < rowsN && it.c < colsN {
			setPlateWell(res, it.r, it.c, it.raw, it.grp, it.tp, it.rep)
		}
	}
	finalizePlateOrder(res)
	return res, true
}

func tryParseBareMatrix(tbl *TabularData, expectedFmt int) (*PlateImportResult, bool) {
	rowsN := len(tbl.Rows)
	colsN := 0
	for _, r := range tbl.Rows {
		if len(r) > colsN {
			colsN = len(r)
		}
	}
	is96 := rowsN == 8 && colsN == 12
	is384 := rowsN == 16 && colsN == 24
	if !is96 && !is384 {
		return nil, false
	}
	format := 96
	if is384 || expectedFmt == 384 {
		format = 384
	}
	res := newPlateResult(format, rowsN, colsN, tbl)
	for r := 0; r < rowsN; r++ {
		for c := 0; c < len(tbl.Rows[r]) && c < colsN; c++ {
			setPlateWell(res, r, c, tbl.Rows[r][c], "", "", 0)
		}
	}
	finalizePlateOrder(res)
	return res, true
}

func newPlateResult(format, rowsN, colsN int, tbl *TabularData) *PlateImportResult {
	return &PlateImportResult{
		Format: format,
		Rows:   rowsN,
		Cols:   colsN,
		Wells:  make(map[string]PlateWellReading, rowsN*colsN),
		Table:  tbl,
	}
}

func setPlateWell(res *PlateImportResult, r, c int, raw, grp, tp string, rep int) {
	coord := fmt.Sprintf("%c%d", 'A'+r, c+1)
	reading := PlateWellReading{
		Coord:     coord,
		Row:       r,
		Col:       c,
		Raw:       strings.TrimSpace(raw),
		Group:     strings.TrimSpace(grp),
		Timepoint: strings.TrimSpace(tp),
		Replicate: rep,
	}
	if v, err := parseFloatCell(raw); err == nil {
		reading.Value = &v
	}
	res.Wells[coord] = reading
}

func finalizePlateOrder(res *PlateImportResult) {
	res.Ordered = make([]PlateWellReading, 0, len(res.Wells))
	for r := 0; r < res.Rows; r++ {
		for c := 0; c < res.Cols; c++ {
			coord := fmt.Sprintf("%c%d", 'A'+r, c+1)
			if w, ok := res.Wells[coord]; ok {
				res.Ordered = append(res.Ordered, w)
			}
		}
	}
}

func parsePlateRowLetter(s string) (int, bool) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) != 1 || s[0] < 'A' || s[0] > 'P' {
		return 0, false
	}
	return int(s[0] - 'A'), true
}

func parseWellCoord(raw string) (int, int, bool) {
	m := wellCoordRe.FindStringSubmatch(strings.TrimSpace(raw))
	if len(m) != 3 {
		return 0, 0, false
	}
	row := int(strings.ToUpper(m[1])[0] - 'A')
	col, err := strconv.Atoi(m[2])
	if err != nil || col < 1 || col > 24 {
		return 0, 0, false
	}
	return row, col - 1, true
}

func detectColumnHeaderRow(row []string) (colCount int, startCol int, ok bool) {
	if len(row) < 3 {
		return 0, 0, false
	}
	start := 0
	if _, err := strconv.Atoi(strings.TrimSpace(row[0])); err != nil {
		start = 1
	}
	expected := 1
	matched := 0
	for i := start; i < len(row); i++ {
		s := strings.TrimSpace(row[i])
		if s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil || n != expected {
			return 0, 0, false
		}
		expected++
		matched++
	}
	if matched < 2 {
		return 0, 0, false
	}
	return matched, start, true
}

func inferPlateDimensions(expectedFmt, seenRows, seenCols int) (format, rowsN, colsN int) {
	if expectedFmt == 384 || seenRows > 8 || seenCols > 12 {
		return 384, 16, 24
	}
	return 96, 8, 12
}

func maxRowIndex(rows []struct {
	rowIdx int
	cells  []string
}) int {
	maxR := 0
	for _, r := range rows {
		if r.rowIdx > maxR {
			maxR = r.rowIdx
		}
	}
	return maxR
}

func parseFloatCell(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "na") || strings.EqualFold(s, "nan") || strings.EqualFold(s, "ovrflw") {
		return 0, errors.New("non-numeric cell")
	}
	s = strings.ReplaceAll(s, ",", "")
	return strconv.ParseFloat(s, 64)
}

func detectDelimiter(text string) rune {
	lines := strings.Split(text, "\n")
	tabHits, commaHits, semiHits := 0, 0, 0
	checked := 0
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		tabHits += strings.Count(ln, "\t")
		commaHits += strings.Count(ln, ",")
		semiHits += strings.Count(ln, ";")
		checked++
		if checked >= 5 {
			break
		}
	}
	if tabHits > 0 && tabHits >= commaHits && tabHits >= semiHits {
		return '\t'
	}
	if semiHits > commaHits && commaHits == 0 {
		return ';'
	}
	return ','
}

func isAllEmptyCells(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// --- Minimal Pure-Go XLSX Reader (stdlib archive/zip + encoding/xml) ---

type xlsxSST struct {
	Items []xlsxSI `xml:"si"`
}

type xlsxSI struct {
	Text string      `xml:"t"`
	Runs []xlsxRun   `xml:"r"`
}

type xlsxRun struct {
	Text string `xml:"t"`
}

type xlsxWorkbook struct {
	Sheets []xlsxSheetRef `xml:"sheets>sheet"`
}

type xlsxSheetRef struct {
	Name    string `xml:"name,attr"`
	SheetID string `xml:"sheetId,attr"`
	RelID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type xlsxRels struct {
	Rels []xlsxRel `xml:"Relationship"`
}

type xlsxRel struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
}

type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	RowNum int        `xml:"r,attr"`
	Cells  []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref    string      `xml:"r,attr"`
	Type   string      `xml:"t,attr"`
	Value  string      `xml:"v"`
	Inline *xlsxInline `xml:"is"`
}

type xlsxInline struct {
	Text string    `xml:"t"`
	Runs []xlsxRun `xml:"r"`
}

func parseXLSX(data []byte, wantSheet string) (*TabularData, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("labio: open xlsx zip: %w", err)
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[strings.TrimPrefix(filepath.ToSlash(f.Name), "/")] = f
	}

	var shared []string
	if f, ok := files["xl/sharedStrings.xml"]; ok {
		raw, rErr := readZipBytes(f)
		if rErr != nil {
			return nil, rErr
		}
		var sst xlsxSST
		if uErr := xml.Unmarshal(raw, &sst); uErr != nil {
			return nil, fmt.Errorf("labio: parse sharedStrings.xml: %w", uErr)
		}
		shared = make([]string, len(sst.Items))
		for i, si := range sst.Items {
			if len(si.Runs) > 0 {
				var b strings.Builder
				for _, run := range si.Runs {
					b.WriteString(run.Text)
				}
				shared[i] = b.String()
			} else {
				shared[i] = si.Text
			}
		}
	}

	sheetPath, sheetName := resolveXLSXSheet(files, wantSheet)
	sf, ok := files[sheetPath]
	if !ok {
		return nil, fmt.Errorf("labio: worksheet %q not found in xlsx", sheetPath)
	}
	rawSheet, err := readZipBytes(sf)
	if err != nil {
		return nil, err
	}
	var ws xlsxWorksheet
	if err := xml.Unmarshal(rawSheet, &ws); err != nil {
		return nil, fmt.Errorf("labio: parse %s: %w", sheetPath, err)
	}

	maxRow, maxCol := 0, 0
	type cellPos struct {
		r, c int
		val  string
	}
	var cells []cellPos
	for rowOrder, row := range ws.Rows {
		rIdx := rowOrder
		if row.RowNum > 0 {
			rIdx = row.RowNum - 1
		}
		if rIdx > maxRow {
			maxRow = rIdx
		}
		for colOrder, c := range row.Cells {
			cIdx := colOrder
			if parsedR, parsedC, ok := parseExcelCellRef(c.Ref); ok {
				rIdx = parsedR
				cIdx = parsedC
				if rIdx > maxRow {
					maxRow = rIdx
				}
			}
			if cIdx > maxCol {
				maxCol = cIdx
			}
			val := resolveXLSXCellValue(c, shared)
			cells = append(cells, cellPos{r: rIdx, c: cIdx, val: val})
		}
	}
	if len(cells) == 0 {
		return nil, errors.New("labio: xlsx worksheet has no cells")
	}

	grid := make([][]string, maxRow+1)
	for i := range grid {
		grid[i] = make([]string, maxCol+1)
	}
	for _, cp := range cells {
		grid[cp.r][cp.c] = strings.TrimSpace(cp.val)
	}

	nonEmpty := make([][]string, 0, len(grid))
	for _, row := range grid {
		if !isAllEmptyCells(row) {
			nonEmpty = append(nonEmpty, row)
		}
	}
	if len(nonEmpty) == 0 {
		return nil, errors.New("labio: xlsx worksheet contains only empty cells")
	}
	return &TabularData{
		Format: "xlsx",
		Sheet:  sheetName,
		Rows:   nonEmpty,
	}, nil
}

func resolveXLSXSheet(files map[string]*zip.File, wantSheet string) (string, string) {
	defaultPath := "xl/worksheets/sheet1.xml"
	wbFile, ok := files["xl/workbook.xml"]
	if !ok {
		return defaultPath, ""
	}
	wbRaw, err := readZipBytes(wbFile)
	if err != nil {
		return defaultPath, ""
	}
	var wb xlsxWorkbook
	if xml.Unmarshal(wbRaw, &wb) != nil || len(wb.Sheets) == 0 {
		return defaultPath, ""
	}

	relsMap := map[string]string{}
	if rf, rOk := files["xl/_rels/workbook.xml.rels"]; rOk {
		if rRaw, rErr := readZipBytes(rf); rErr == nil {
			var rels xlsxRels
			if xml.Unmarshal(rRaw, &rels) == nil {
				for _, rel := range rels.Rels {
					target := strings.TrimPrefix(filepath.ToSlash(rel.Target), "/")
					if !strings.HasPrefix(target, "xl/") {
						target = "xl/" + target
					}
					relsMap[rel.ID] = target
				}
			}
		}
	}

	chosen := wb.Sheets[0]
	if strings.TrimSpace(wantSheet) != "" {
		for _, s := range wb.Sheets {
			if strings.EqualFold(s.Name, strings.TrimSpace(wantSheet)) {
				chosen = s
				break
			}
		}
	}
	if target, ok := relsMap[chosen.RelID]; ok {
		return target, chosen.Name
	}
	return defaultPath, chosen.Name
}

func resolveXLSXCellValue(c xlsxCell, shared []string) string {
	switch c.Type {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(c.Value))
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return c.Value
	case "inlineStr":
		if c.Inline != nil {
			if len(c.Inline.Runs) > 0 {
				var b strings.Builder
				for _, r := range c.Inline.Runs {
					b.WriteString(r.Text)
				}
				return b.String()
			}
			return c.Inline.Text
		}
	case "b":
		if strings.TrimSpace(c.Value) == "1" {
			return "TRUE"
		}
		return "FALSE"
	}
	return c.Value
}

func parseExcelCellRef(ref string) (row, col int, ok bool) {
	ref = strings.TrimSpace(strings.ToUpper(ref))
	if ref == "" {
		return 0, 0, false
	}
	split := 0
	for split < len(ref) && ref[split] >= 'A' && ref[split] <= 'Z' {
		split++
	}
	if split == 0 || split == len(ref) {
		return 0, 0, false
	}
	colVal := 0
	for i := 0; i < split; i++ {
		colVal = colVal*26 + int(ref[i]-'A'+1)
	}
	rowVal, err := strconv.Atoi(ref[split:])
	if err != nil || rowVal < 1 || colVal < 1 {
		return 0, 0, false
	}
	return rowVal - 1, colVal - 1, true
}

func readZipBytes(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 64<<20))
}
