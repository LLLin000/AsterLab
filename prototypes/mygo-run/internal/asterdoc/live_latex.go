package asterdoc

import (
	"regexp"
	"strings"
	"unicode"
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
// into clean, readable Unicode mathematical typography for Obsidian-style Live Preview.
func formatLatexInlinePretty(expr string) string {
	s := strings.TrimSpace(expr)
	if s == "" {
		return ""
	}
	// Resolve nested \text{...}, \frac{a}{b}, \sqrt{x} up to 3 passes
	for pass := 0; pass < 3; pass++ {
		prev := s
		s = reLatexTextCmd.ReplaceAllString(s, "$1")
		s = reLatexFracCmd.ReplaceAllString(s, "($1)/($2)")
		s = reLatexSqrtCmd.ReplaceAllString(s, "√($1)")
		if s == prev {
			break
		}
	}
	replacer := strings.NewReplacer(
		`\times`, "×",
		`\cdot`, "·",
		`\approx`, "≈",
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
	s = replacer.Replace(s)

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
	// Collapse excessive whitespace
	return strings.Join(strings.Fields(s), " ")
}

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
		case 's':
			b.WriteRune('ₛ')
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
