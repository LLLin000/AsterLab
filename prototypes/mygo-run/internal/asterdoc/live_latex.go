package asterdoc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"asterlab-mygo-prototype/internal/theme"

	"github.com/egoist/mygo/ui"
)

var (
	reLatexTextCmd  = regexp.MustCompile(`\\(?:text|mathrm|operatorname)\{([^{}]*)\}`)
	reLatexFracCmd  = regexp.MustCompile(`\\d?frac\{([^{}]*)\}\{([^{}]*)\}`)
	reLatexSqrtCmd  = regexp.MustCompile(`\\sqrt\{([^{}]*)\}`)
	reLatexSupBrace = regexp.MustCompile(`\^\{([^{}]+)\}`)
	reLatexSubBrace = regexp.MustCompile(`_\{([^{}]+)\}`)
	reLatexSupOne   = regexp.MustCompile(`\^([0-9+\-n])`)
	reLatexSubOne   = regexp.MustCompile(`_([0-9+\-a-zA-Z])`)
)

// looksLikeInlineLatex guards `$...$` against accidental currency spans like "$10 to $20".
// Matches Obsidian/MathJax inline math rules: no leading/trailing space next to `$`,
// and contains at least one LaTeX command, variable letter, or mathematical operator.
func looksLikeInlineLatex(inner string) bool {
	if len(inner) == 0 {
		return false
	}
	if strings.HasPrefix(inner, " ") || strings.HasSuffix(inner, " ") || strings.HasPrefix(inner, "\t") || strings.HasSuffix(inner, "\t") {
		return false
	}
	hasAlphaOrCmd := false
	for _, r := range inner {
		if r == '\\' || r == '^' || r == '_' || r == '=' || unicode.IsLetter(r) {
			hasAlphaOrCmd = true
			break
		}
	}
	return hasAlphaOrCmd
}

// formatLatexInlinePretty turns scientific LaTeX expressions such as
// `V = \pi r^2 h \approx 201.1\ \mu\text{L}` or `\text{SR} = \frac{W_t - W_0}{W_0} \times 100\%`
// into clean, readable Unicode mathematical typography for labels, widths, and accessibility.
func formatLatexInlinePretty(expr string) string {
	s := strings.TrimSpace(expr)
	if s == "" {
		return ""
	}
	for pass := 0; pass < 3; pass++ {
		prev := s
		s = reLatexTextCmd.ReplaceAllString(s, "$1")
		s = reLatexFracCmd.ReplaceAllString(s, "($1)/($2)")
		s = reLatexSqrtCmd.ReplaceAllString(s, "√($1)")
		if s == prev {
			break
		}
	}
	s = latexSymbolReplacer.Replace(s)

	s = reLatexSupBrace.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLatexSupBrace.FindStringSubmatch(m)
		return toUnicodeSuperscript(sub[1])
	})
	s = reLatexSupOne.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLatexSupOne.FindStringSubmatch(m)
		return toUnicodeSuperscript(sub[1])
	})
	s = reLatexSubBrace.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLatexSubBrace.FindStringSubmatch(m)
		return toUnicodeSubscriptFull(sub[1])
	})
	s = reLatexSubOne.ReplaceAllStringFunc(s, func(m string) string {
		sub := reLatexSubOne.FindStringSubmatch(m)
		return toUnicodeSubscriptFull(sub[1])
	})
	return strings.Join(strings.Fields(s), " ")
}

var latexSymbolReplacer = strings.NewReplacer(
	`\times`, "×",
	`\cdot`, "·",
	`\approx`, "≈",
	`\sim`, "~",
	`\pm`, "±",
	`\mp`, "∓",
	`\leq`, "≤",
	`\le`, "≤",
	`\geq`, "≥",
	`\ge`, "≥",
	`\neq`, "≠",
	`\ne`, "≠",
	`\to`, "→",
	`\rightarrow`, "→",
	`\infty`, "∞",
	`\pi`, "π",
	`\mu`, "μ",
	`\alpha`, "α",
	`\beta`, "β",
	`\gamma`, "γ",
	`\delta`, "δ",
	`\Delta`, "Δ",
	`\lambda`, "λ",
	`\rho`, "ρ",
	`\sigma`, "σ",
	`\theta`, "θ",
	`\omega`, "ω",
	`\Omega`, "Ω",
	`\degree`, "°",
	`\circ`, "°",
	`\%`, "%",
	`\_`, "_",
	`\quad`, " ",
	`\qquad`, "  ",
	`\,`, " ",
	`\;`, " ",
	`\:`, " ",
	`\ `, " ",
	`\left`, "",
	`\right`, "",
)

func toUnicodeSuperscript(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '0':
			b.WriteRune('⁰')
		case '1':
			b.WriteRune('¹')
		case '2':
			b.WriteRune('²')
		case '3':
			b.WriteRune('³')
		case '4':
			b.WriteRune('⁴')
		case '5':
			b.WriteRune('⁵')
		case '6':
			b.WriteRune('⁶')
		case '7':
			b.WriteRune('⁷')
		case '8':
			b.WriteRune('⁸')
		case '9':
			b.WriteRune('⁹')
		case '+':
			b.WriteRune('⁺')
		case '-':
			b.WriteRune('⁻')
		case 'n':
			b.WriteRune('ⁿ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func toUnicodeSubscriptFull(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '0':
			b.WriteRune('₀')
		case '1':
			b.WriteRune('₁')
		case '2':
			b.WriteRune('₂')
		case '3':
			b.WriteRune('₃')
		case '4':
			b.WriteRune('₄')
		case '5':
			b.WriteRune('₅')
		case '6':
			b.WriteRune('₆')
		case '7':
			b.WriteRune('₇')
		case '8':
			b.WriteRune('₈')
		case '9':
			b.WriteRune('₉')
		case '+':
			b.WriteRune('₊')
		case '-':
			b.WriteRune('₋')
		case 'a':
			b.WriteRune('ₐ')
		case 'e':
			b.WriteRune('ₑ')
		case 'i':
			b.WriteRune('ᵢ')
		case 'n':
			b.WriteRune('ₙ')
		case 'o':
			b.WriteRune('ₒ')
		case 'r':
			b.WriteRune('ᵣ')
		case 't':
			b.WriteRune('ₜ')
		case 'x':
			b.WriteRune('ₓ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

type scriptRunKind uint8

const (
	scriptNormal scriptRunKind = iota
	scriptSup
	scriptSub
)

type scriptRun struct {
	Kind scriptRunKind
	Text string
}

// parseRichScriptRuns parses LaTeX/scientific inline text into native normal,
// superscript (^), and subscript (_) runs so the UI renders real raised/lowered
// smaller spans (UI-01) instead of uneven Unicode superscript/subscript glyphs.
func parseRichScriptRuns(expr string) []scriptRun {
	s := strings.TrimSpace(expr)
	if s == "" {
		return nil
	}
	for pass := 0; pass < 3; pass++ {
		prev := s
		s = reLatexTextCmd.ReplaceAllString(s, "$1")
		s = reLatexFracCmd.ReplaceAllString(s, "($1)/($2)")
		s = reLatexSqrtCmd.ReplaceAllString(s, "√($1)")
		if s == prev {
			break
		}
	}
	s = latexSymbolReplacer.Replace(s)

	var runs []scriptRun
	var normBuf strings.Builder
	flushNormal := func() {
		if normBuf.Len() > 0 {
			runs = append(runs, scriptRun{Kind: scriptNormal, Text: normBuf.String()})
			normBuf.Reset()
		}
	}

	for i := 0; i < len(s); {
		ch := s[i]
		if (ch == '^' || ch == '_') && i+1 < len(s) {
			kind := scriptSup
			if ch == '_' {
				kind = scriptSub
			}
			j := i + 1
			var content string
			if s[j] == '{' {
				depth := 1
				k := j + 1
				for k < len(s) && depth > 0 {
					if s[k] == '{' {
						depth++
					} else if s[k] == '}' {
						depth--
					}
					k++
				}
				if depth == 0 {
					content = s[j+1 : k-1]
					i = k
				} else {
					normBuf.WriteByte(ch)
					i++
					continue
				}
			} else if kind == scriptSup && (s[j] == '-' || s[j] == '+') && j+1 < len(s) && s[j+1] >= '0' && s[j+1] <= '9' {
				k := j + 1
				for k < len(s) && s[k] >= '0' && s[k] <= '9' {
					k++
				}
				content = s[j:k]
				i = k
			} else {
				content = s[j : j+1]
				i = j + 1
			}
			flushNormal()
			if content != "" {
				runs = append(runs, scriptRun{Kind: kind, Text: content})
			}
			continue
		}
		normBuf.WriteByte(ch)
		i++
	}
	flushNormal()
	return runs
}

// renderNativeScriptRow renders scientific expressions with native raised superscript
// and lowered subscript ui.Text spans (UI-01).
func renderNativeScriptRow(c *ui.Context, tok theme.Tokens, expr string, baseSize float32, col ui.Color, italic bool, bold bool) {
	runs := parseRichScriptRuns(expr)
	if len(runs) == 0 {
		return
	}
	scriptSize := baseSize * 0.72
	if scriptSize < 10 {
		scriptSize = 10
	}
	offset := baseSize * 0.30
	ui.Row(c).AlignItems(ui.Center).Gap(0).Children(func() {
		for _, r := range runs {
			switch r.Kind {
			case scriptSup:
				ui.Box(c).Padding(0, 0, offset, 0).AlignSelf(ui.Start).Children(func() {
					t := ui.Text(c, r.Text).FontSize(scriptSize).TextColor(col).FontFeatures("tnum").SingleLine()
					if bold {
						t.Bold()
					}
				})
			case scriptSub:
				ui.Box(c).Padding(offset, 0, 0, 0).AlignSelf(ui.End).Children(func() {
					t := ui.Text(c, r.Text).FontSize(scriptSize).TextColor(col).FontFeatures("tnum").SingleLine()
					if bold {
						t.Bold()
					}
				})
			default:
				t := ui.Text(c, r.Text).FontSize(baseSize).TextColor(col).FontFeatures("tnum").SingleLine()
				if italic {
					t.Italic()
				}
				if bold {
					t.Bold()
				}
			}
		}
	})
}

// formatSciParts formats numbers with >= 3 zeros (|v| >= 1000 or 0 < |v| <= 0.001)
// into scientific notation parts (mantissa, exponent) per UI-05.
func formatSciParts(v float64) (mantissa string, exp int, isSci bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v == 0 {
		return formatNumber(v), 0, false
	}
	absV := math.Abs(v)
	if absV < 1000 && absV > 0.001 {
		return formatNumber(round4(v)), 0, false
	}
	exp = int(math.Floor(math.Log10(absV)))
	m := v / math.Pow(10, float64(exp))
	m = math.Round(m*100) / 100
	if math.Abs(m) >= 10 {
		m /= 10
		exp++
	}
	mantissa = strconv.FormatFloat(m, 'f', -1, 64)
	return mantissa, exp, true
}

// formatSciCompactText formats a value using scientific notation when |v| >= 1000
// or 0 < |v| <= 0.001 (e.g. 1000000 -> "10⁶", 14200000 -> "1.42×10⁷", 0.0001 -> "10⁻⁴").
func formatSciCompactText(v float64) string {
	m, exp, isSci := formatSciParts(v)
	if !isSci {
		return m
	}
	if m == "1" {
		return "10" + toUnicodeSuperscript(strconv.Itoa(exp))
	}
	return m + "×10" + toUnicodeSuperscript(strconv.Itoa(exp))
}

// renderSciNumber renders a number with native raised exponent when |v| >= 1000 or 0 < |v| <= 0.001 (UI-05).
func renderSciNumber(c *ui.Context, tok theme.Tokens, v float64, unit string, baseSize float32, col ui.Color, bold bool) {
	m, exp, isSci := formatSciParts(v)
	var expr string
	if !isSci {
		expr = m
	} else if m == "1" {
		expr = fmt.Sprintf("1×10^{%d}", exp)
	} else {
		expr = fmt.Sprintf("%s×10^{%d}", m, exp)
	}
	if strings.TrimSpace(unit) != "" {
		expr += " " + DisplayUnit(unit)
	}
	renderNativeScriptRow(c, tok, expr, baseSize, col, false, bold)
}

var (
	reStandaloneCel = regexp.MustCompile(`\bCel\b`)
	reTextSciPower  = regexp.MustCompile(`(\b(?:10|[a-zA-Z]{1,3}))\^(?:\{([+-]?[0-9a-zA-Z]+)\}|([+-]?[0-9]+))`)
)

// FormatDisplayText converts UCUM unit tokens like "37 Cel" -> "37 °C" and inline powers
// like "10^-3 ~ 10^-5" -> "10⁻³ ~ 10⁻⁵" for clean display in titles, chips, and summaries (UI-04, UI-06).
func FormatDisplayText(s string) string {
	if s == "" {
		return s
	}
	out := reStandaloneCel.ReplaceAllString(s, "°C")
	out = reTextSciPower.ReplaceAllStringFunc(out, func(m string) string {
		sub := reTextSciPower.FindStringSubmatch(m)
		exp := sub[2]
		if exp == "" {
			exp = sub[3]
		}
		return sub[1] + toUnicodeSuperscript(exp)
	})
	return out
}

// FormatSciStat formats statistical metrics (mean, SD, blank) using scientific notation
// whenever |v| >= 1000 or 0 < |v| <= 0.001 (UI-07), otherwise clean 3-decimal notation.
func FormatSciStat(v float64) string {
	if _, _, isSci := formatSciParts(v); isSci {
		return formatSciCompactText(v)
	}
	return fmt.Sprintf("%.3f", v)
}
