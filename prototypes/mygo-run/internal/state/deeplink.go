package state

import (
	"fmt"
	"net/url"
	"strings"
)

// RunWellLink is a Run-page navigation intent: open the Run and select the
// well on its recording grid. UX-IA-3, UX-P5.
type RunWellLink struct {
	RunID string
	Well  string
}

// ParseRunWellLink parses asterlab://run/<runID>/well/<well> deep links.
// Invalid links return visible errors, never panic.
func ParseRunWellLink(raw string) (RunWellLink, error) {
	trimmed := strings.TrimSpace(raw)
	u, err := url.Parse(trimmed)
	if err != nil || u == nil {
		return RunWellLink{}, fmt.Errorf("打不开实验链接：链接格式不正确")
	}
	if !strings.EqualFold(u.Scheme, "asterlab") || !strings.EqualFold(u.Host, "run") {
		return RunWellLink{}, fmt.Errorf("打不开实验链接：应为 asterlab://run/<实验ID>/well/<孔位>")
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) != 3 || segments[0] == "" || !strings.EqualFold(segments[1], "well") || segments[2] == "" {
		return RunWellLink{}, fmt.Errorf("打不开实验链接：应为 asterlab://run/<实验ID>/well/<孔位>")
	}
	well := strings.ToUpper(strings.TrimSpace(segments[2]))
	if !validWell(well) {
		return RunWellLink{}, fmt.Errorf("打不开实验链接：孔位 %q 不正确", segments[2])
	}
	return RunWellLink{RunID: segments[0], Well: well}, nil
}

// validWell accepts plate coordinates such as A1 or H12: 1-2 row letters
func validWell(well string) bool {
	letters, digits := 0, 0
	for i := range well {
		switch {
		case well[i] >= 'A' && well[i] <= 'Z':
			if digits > 0 {
				return false
			}
			letters++
		case well[i] >= '0' && well[i] <= '9':
			digits++
		default:
			return false
		}
	}
	if letters < 1 || letters > 2 || digits < 1 || len(well) > 4 {
		return false
	}
	return well[letters] != '0'
}

// FindWellCell maps a parsed well onto grid coordinates by matching the
// schema's row+column labels, so any plate format resolves without a
// hardcoded matrix. UX-P4.
func FindWellCell(well string, rowLabels, colLabels []string) (row, col int, ok bool) {
	for r, rl := range rowLabels {
		for c, cl := range colLabels {
			if strings.EqualFold(rl+cl, well) {
				return r, c, true
			}
		}
	}
	return 0, 0, false
}
