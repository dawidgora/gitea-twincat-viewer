package parser

import (
	"strings"

	"github.com/dawidgora/gitea-twincat-viewer/internal/twincat"
)

// firstDeclarationToken skips whitespace, comments and TwinCAT attributes,
// but fails closed on an incomplete prefix rather than guessing later text.
func firstDeclarationToken(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for len(s) > 0 {
		if strings.HasPrefix(s, "{") {
			end := attributeEnd(s)
			if end < 0 {
				return "", false
			}
			s = strings.TrimSpace(s[end:])
			continue
		}
		if strings.HasPrefix(s, "//") {
			if n := strings.IndexByte(s, '\n'); n >= 0 {
				s = strings.TrimSpace(s[n+1:])
				continue
			}
			return "", false
		}
		if strings.HasPrefix(s, "(*") {
			end := commentEnd(s, 0)
			if end < 0 {
				return "", false
			}
			s = strings.TrimSpace(s[end:])
			continue
		}
		break
	}
	start := 0
	for start < len(s) && isIdent(s[start]) {
		start++
	}
	if start == 0 {
		return "", false
	}
	return s[:start], true
}

func quotedEnd(s string, start int) (int, bool) {
	q := s[start]
	for i := start + 1; i < len(s); i++ {
		if s[i] == '$' {
			i++
			continue
		}
		if s[i] == q {
			i++
			if i < len(s) && s[i] == q {
				continue
			}
			return i, true
		}
	}
	return len(s), false
}

func attributeEnd(s string) int {
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '\'' || c == '"' {
			end, ok := quotedEnd(s, i)
			if !ok {
				return -1
			}
			i = end - 1
			continue
		}
		if c == '}' {
			return i + 1
		}
	}
	return -1
}

func commentEnd(s string, start int) int {
	depth := 1
	for i := start + 2; i < len(s)-1; i++ {
		if strings.HasPrefix(s[i:], "(*") {
			depth++
			i++
			continue
		}
		if strings.HasPrefix(s[i:], "*)") {
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// cleanST removes comments, attributes, and quoted strings while retaining token spacing.
func cleanST(s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\'' || s[i] == '"' {
			end, ok := quotedEnd(s, i)
			if !ok {
				return b.String(), false
			}
			i = end
			b.WriteByte(' ')
			continue
		}
		if s[i] == '{' {
			end := attributeEnd(s[i:])
			if end < 0 {
				return b.String(), false
			}
			i += end
			b.WriteByte(' ')
			continue
		}
		if strings.HasPrefix(s[i:], "//") {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
			continue
		}
		if strings.HasPrefix(s[i:], "(*") {
			e := commentEnd(s, i)
			if e < 0 {
				return b.String(), false
			}
			i = e
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String(), true
}

// classifyDUT is a display classifier, not a full ST grammar validator;
// malformed lexical spans and unrecognized declarations remain unclassified.
func classifyDUT(src string) twincat.DUTType {
	clean, ok := cleanST(src)
	if !ok {
		return ""
	}
	s := strings.TrimSpace(clean)
	fields := strings.Fields(strings.ToUpper(s))
	if len(fields) == 0 || fields[0] != "TYPE" {
		return ""
	}
	colon := strings.IndexByte(s, ':')
	if colon < 0 {
		return ""
	}
	body := strings.TrimSpace(s[colon+1:])
	if body == "" {
		return ""
	}
	word := func(v string) bool {
		u := strings.ToUpper(body)
		return strings.HasPrefix(u, v) && (len(u) == len(v) || !isIdent(u[len(v)]))
	}
	switch {
	case word("STRUCT"):
		return twincat.DUTTypeStruct
	case word("UNION"):
		return twincat.DUTTypeUnion
	case strings.HasPrefix(body, "("):
		return twincat.DUTTypeEnum
	default:
		if len(body) > 0 && (body[0] == '_' || body[0] >= 'A' && body[0] <= 'Z' || body[0] >= 'a' && body[0] <= 'z') {
			return twincat.DUTTypeAlias
		}
		return ""
	}
}

func isIdent(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}
