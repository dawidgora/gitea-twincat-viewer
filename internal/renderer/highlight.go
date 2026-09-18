package renderer

import (
	"html"
	"strings"
)

// Style describes the inline presentation for a highlighted token.
// Color is a concrete hex fallback; Var is the name of a CSS custom
// property that may override the fallback via the embedded stylesheet.
// Bold and Italic map to font-weight/font-style declarations.
type Style struct {
	Color  string
	Var    string
	Bold   bool
	Italic bool
}

// String returns the semicolon-separated CSS declarations for the style.
func (s Style) String() string {
	var parts []string
	if s.Var != "" {
		// Emit the concrete fallback first, then the variable override.
		// If the variable is undefined (e.g. the stylesheet is stripped)
		// the concrete color is still rendered.
		parts = append(parts, "color:"+s.Color)
		parts = append(parts, "color:var("+s.Var+")")
	} else {
		parts = append(parts, "color:"+s.Color)
	}
	if s.Bold {
		parts = append(parts, "font-weight:600")
	}
	if s.Italic {
		parts = append(parts, "font-style:italic")
	}
	return strings.Join(parts, ";")
}

// Gitea-syntax color palette.  The light-theme hex values are used as
// fallbacks; the embedded stylesheet redefines the corresponding custom
// properties for dark mode via prefers-color-scheme.
var (
	keywordStyle  = Style{Color: "#cf222e", Var: "--tcv-kw", Bold: true}
	typeStyle     = Style{Color: "#953800", Var: "--tcv-type", Bold: true}
	literalStyle  = Style{Color: "#0550ae", Var: "--tcv-lit"}
	stringStyle   = Style{Color: "#0a3069", Var: "--tcv-str"}
	commentStyle  = Style{Color: "#6e7781", Var: "--tcv-com", Italic: true}
	functionStyle = Style{Color: "#8250df", Var: "--tcv-fn"}
	numberStyle   = Style{Color: "#0550ae", Var: "--tcv-num"}
)

// highlightST applies minimal Structured Text / IEC 61131-3 syntax
// highlighting. Output is HTML-safe: every input byte is either
// emitted as escaped character data or wrapped in a <span style="...">
// tag. There is no path that emits raw HTML from the source.
func highlightST(src string) string {
	var b strings.Builder
	b.Grow(len(src) * 2)

	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], "(*") {
			end := strings.Index(src[i:], "*)")
			if end == -1 {
				writeSpan(&b, commentStyle, src[i:])
				return b.String()
			}
			writeSpan(&b, commentStyle, src[i:i+end+2])
			i += end + 2
			continue
		}
		if strings.HasPrefix(src[i:], "//") {
			j := strings.IndexByte(src[i:], '\n')
			if j == -1 {
				j = len(src) - i
			}
			writeSpan(&b, commentStyle, src[i:i+j])
			i += j
			continue
		}
		if src[i] == '{' {
			j := i + 1
			depth := 1
			for j < len(src) && depth > 0 {
				switch src[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				j++
			}
			writeSpan(&b, commentStyle, src[i:j])
			i = j
			continue
		}
		if src[i] == '\'' {
			j := i + 1
			for j < len(src) {
				if src[j] == '\'' {
					if j+1 < len(src) && src[j+1] == '\'' {
						j += 2
						continue
					}
					j++
					break
				}
				j++
			}
			writeSpan(&b, stringStyle, src[i:j])
			i = j
			continue
		}
		if isDigit(src[i]) || (src[i] == '.' && i+1 < len(src) && isDigit(src[i+1])) {
			j := i + 1
			for j < len(src) && (isAlnum(src[j]) || src[j] == '_' || src[j] == '.' || src[j] == '#') {
				if isLetter(src[j]) && !(j > i && (isDigit(src[j-1]) || src[j-1] == '.' || src[j-1] == '#')) {
					break
				}
				j++
			}
			writeSpan(&b, numberStyle, src[i:j])
			i = j
			continue
		}
		if isIdentStart(src[i]) {
			j := i + 1
			for j < len(src) && isIdentCont(src[j]) {
				j++
			}
			word := src[i:j]
			upper := strings.ToUpper(word)
			style, ok := classifyToken(upper)
			if !ok && isFunctionCall(src, j) {
				style = functionStyle
				ok = true
			}
			if ok {
				writeSpan(&b, style, word)
			} else {
				b.WriteString(html.EscapeString(word))
			}
			i = j
			continue
		}
		b.WriteString(html.EscapeString(string(src[i])))
		i++
	}
	return b.String()
}

func writeSpan(b *strings.Builder, style Style, text string) {
	b.WriteString(`<span style="`)
	b.WriteString(style.String())
	b.WriteString(`">`)
	b.WriteString(html.EscapeString(text))
	b.WriteString(`</span>`)
}

func isFunctionCall(src string, j int) bool {
	for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
		j++
	}
	return j < len(src) && src[j] == '('
}

func classifyToken(upper string) (Style, bool) {
	if stKeywords[upper] {
		return keywordStyle, true
	}
	if stTypes[upper] {
		return typeStyle, true
	}
	if stLiterals[upper] {
		return literalStyle, true
	}
	return Style{}, false
}

func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func isLetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isAlnum(c byte) bool  { return isDigit(c) || isLetter(c) }
func isIdentStart(c byte) bool {
	return isLetter(c) || c == '_'
}
func isIdentCont(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '#'
}

// stKeywords is the IEC 61131-3 / TwinCAT Structured Text keyword set.
// Matching is case-insensitive.
var stKeywords = map[string]bool{
	"AND":            true,
	"ANY":            true,
	"ARRAY":          true,
	"AT":             true,
	"CASE":           true,
	"CONSTANT":       true,
	"DO":             true,
	"ELSE":           true,
	"ELSIF":          true,
	"END_CASE":       true,
	"END_FOR":        true,
	"END_FUNCTION":   true,
	"END_IF":         true,
	"END_PROGRAM":    true,
	"END_REPEAT":     true,
	"END_STRUCT":     true,
	"END_TYPE":       true,
	"END_UNION":      true,
	"END_VAR":        true,
	"END_WHILE":      true,
	"EXIT":           true,
	"FOR":            true,
	"FUNCTION":       true,
	"FUNCTION_BLOCK": true,
	"GOTO":           true,
	"IF":             true,
	"IMPLEMENTS":     true,
	"INTERFACE":      true,
	"ENDINTERFACE":   true,
	"END_IMPLEMENTS": true,
	"END_NAMESPACE":  true,
	"EXTENDS":        true,
	"MOD":            true,
	"NOT":            true,
	"OF":             true,
	"OR":             true,
	"PRIVATE":        true,
	"PROGRAM":        true,
	"PUBLIC":         true,
	"PROTECTED":      true,
	"REPEAT":         true,
	"RETURN":         true,
	"STRUCT":         true,
	"THEN":           true,
	"TO":             true,
	"TYPE":           true,
	"UNION":          true,
	"UNTIL":          true,
	"VAR":            true,
	"VAR_INPUT":      true,
	"VAR_OUTPUT":     true,
	"VAR_IN_OUT":     true,
	"VAR_GLOBAL":     true,
	"VAR_TEMP":       true,
	"VAR_STAT":       true,
	"VAR_INSTANCE":   true,
	"VAR_CONFIG":     true,
	"VAR_EXTERNAL":   true,
	"WHILE":          true,
	"XOR":            true,
	"WITH":           true,
	"ENUM":           true,
	"END_ENUM":       true,
	"NAMESPACE":      true,
	"USING":          true,
	"METHOD":         true,
	"PROPERTY":       true,
	"GET":            true,
	"SET":            true,
	"ACTION":         true,
	"POINTER":        true,
	"REFERENCE":      true,
}

// stTypes is the standard IEC 61131-3 elementary type set extended
// with common TwinCAT types.
var stTypes = map[string]bool{
	"BOOL":          true,
	"BYTE":          true,
	"WORD":          true,
	"DWORD":         true,
	"LWORD":         true,
	"SINT":          true,
	"INT":           true,
	"DINT":          true,
	"LINT":          true,
	"USINT":         true,
	"UINT":          true,
	"UDINT":         true,
	"ULINT":         true,
	"REAL":          true,
	"LREAL":         true,
	"STRING":        true,
	"WSTRING":       true,
	"CHAR":          true,
	"WCHAR":         true,
	"TIME":          true,
	"TIME_OF_DAY":   true,
	"TOD":           true,
	"DATE":          true,
	"DATE_AND_TIME": true,
	"DT":            true,
	"LTIME":         true,
	// TwinCAT / Beckhoff specific
	"ARRAY":     true,
	"POINTER":   true,
	"REFERENCE": true,
}

var stLiterals = map[string]bool{
	"TRUE":  true,
	"FALSE": true,
	"NULL":  true,
}
