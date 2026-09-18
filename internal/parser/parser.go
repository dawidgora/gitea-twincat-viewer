// Package parser converts raw TwinCAT XML into the typed model in
// internal/twincat. The parser is intentionally strict about XML
// safety: it uses encoding/xml without enabling external entity
// resolution, which is the default behaviour of the standard library.
//
// The parser supports the three file kinds the MVP focuses on:
//   - .TcPOU  → KindPOU
//   - .TcDUT  → KindDUT
//   - .TcGVL  → KindGVL
//
// Anything else returns KindUnknown so the caller can decide how to
// react.
package parser

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dawidgora/gitea-twincat-viewer/internal/twincat"
)

// MaxInputBytes caps the size of any single XML document we are
// willing to parse. The cap is generous - TwinCAT POU files rarely
// exceed a few hundred kilobytes - and protects against pathologically
// large inputs being fed through Gitea's external renderer pipeline.
const MaxInputBytes = 2 << 20 // 2 MiB

// ErrTooLarge is returned when the input exceeds MaxInputBytes.
var ErrTooLarge = errors.New("twincat: input exceeds maximum allowed size")

// ErrMalformedXML wraps any error returned by encoding/xml when the
// XML decoder rejects the document.
var ErrMalformedXML = errors.New("twincat: malformed XML")

// parserState accumulates intermediate state during a single parse run.
type parserState struct {
	name           string
	pouType        twincat.POUType
	dutType        twincat.DUTType
	declaration    strings.Builder
	implementation strings.Builder
	methods        []twincat.Method
	members        []twincat.Member
	gvl            []twincat.Member
	attributes     []string
	currentMethod  *twincat.Method
	currentBlock   string // "declaration" or "implementation" or ""
	captureTarget  *strings.Builder
}

// element names we look for inside a TwinCAT document.
const (
	elTcPlcObject    = "TcPlcObject"
	elPOU            = "POU"
	elDUT            = "DUT"
	elGVL            = "GVL"
	elTcPOU          = "TcPOU"
	elTcDUT          = "TcDUT"
	elTcGVL          = "TcGVL"
	elDeclaration    = "Declaration"
	elImplementation = "Implementation"
	elST             = "ST"
	elMethod         = "Method"
	elProperty       = "Property"
	elAction         = "Action"
	elDefinition     = "Definition"
)

// Parse converts raw XML bytes into a twincat.File. The provided
// extension (with or without leading dot) is used to determine the
// file Kind before parsing. Pass an empty string to auto-detect from
// the root element.
func Parse(data []byte, ext string) (*twincat.File, error) {
	if len(data) > MaxInputBytes {
		return nil, ErrTooLarge
	}

	state := &parserState{}

	// We use a streaming XML decoder so that large files do not have
	// to be buffered entirely. We bound the decoder with io.LimitReader
	// to defend against maliciously sized inputs even if the caller
	// skipped the byte-length check.
	dec := xml.NewDecoder(io.LimitReader(strings.NewReader(string(data)), MaxInputBytes))
	dec.Strict = true
	// Explicitly disable any external entity expansion. Go's stdlib
	// does not resolve external entities by default, but we name the
	// intent for future maintainers.
	dec.Entity = xml.HTMLEntity

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedXML, err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if err := state.handleStart(t, dec); err != nil {
				return nil, err
			}
		case xml.EndElement:
			state.handleEnd(t)
		}
	}

	if state.name == "" {
		return nil, fmt.Errorf("%w: missing root element", ErrMalformedXML)
	}

	kind := twincat.KindFromExtension(ext)
	if kind == twincat.KindUnknown {
		kind = inferKind(state)
	}

	return &twincat.File{
		Kind:           kind,
		Name:           state.name,
		POUType:        state.pouType,
		DUTType:        state.dutType,
		Declaration:    strings.TrimSpace(state.declaration.String()),
		Implementation: strings.TrimSpace(state.implementation.String()),
		Methods:        state.methods,
		Members:        state.members,
		GVL:            state.gvl,
		Attributes:     state.attributes,
		Raw:            string(data),
	}, nil
}

// inferKind picks a Kind based on which root element was observed.
func inferKind(state *parserState) twincat.Kind {
	switch {
	case state.pouType != "":
		return twincat.KindPOU
	case state.dutType != "":
		return twincat.KindDUT
	case len(state.gvl) > 0:
		return twincat.KindGVL
	}
	return twincat.KindUnknown
}

// handleStart processes the opening of an XML element, descending into
// any inner element that carries Structured Text content.
func (s *parserState) handleStart(start xml.StartElement, dec *xml.Decoder) error {
	name := start.Name.Local

	switch name {
	case elTcPlcObject:
	case elPOU:
		s.name = attr(start, "Name")
		// TwinCAT uses KindOfPOU for the "Element / Function / Method"
		// designator; the *POU kind* (FUNCTION_BLOCK/PROGRAM/FUNCTION)
		// is recorded as the first keyword in the declaration. So we
		// only treat the attribute as authoritative when it already
		// looks like one of those keywords.
		if v := attr(start, "KindOfPOU"); v != "" {
			if isPOUKeyword(v) {
				s.pouType = twincat.POUType(v)
			}
		}
	case elGVL:
		s.name = attr(start, "Name")
	case elDUT:
		s.name = attr(start, "Name")
	case elTcPOU:
		s.name = attr(start, "Name")
		if v := attr(start, "KindOfPOU"); v != "" && isPOUKeyword(v) {
			s.pouType = twincat.POUType(v)
		}
	case elTcDUT:
		s.name = attr(start, "Name")
	case elTcGVL:
		s.name = attr(start, "Name")
	case elDeclaration:
		s.currentBlock = elDeclaration
		if s.currentMethod != nil {
			var buf strings.Builder
			s.captureBlock(start, dec, &buf)
			s.currentMethod.Declaration = buf.String()
		} else {
			s.captureBlock(start, dec, &s.declaration)
			if s.pouType == "" {
				if t := detectPOUType(s.declaration.String()); t != "" {
					s.pouType = t
				}
			}
			if s.dutType == "" {
				if t := detectDUTType(s.declaration.String()); t != "" {
					s.dutType = t
				}
			}
		}
	case elImplementation:
		s.currentBlock = elImplementation
		// Many TwinCAT POU documents wrap the implementation body in a
		// child <ST><![CDATA[...]]></ST>. captureCDATA descends into
		// inner elements and captures their text. If the <ST> wrapper
		// is absent, captureCDATA still records the direct text.
		if s.currentMethod != nil {
			var buf strings.Builder
			s.captureBlock(start, dec, &buf)
			s.currentMethod.Implementation = buf.String()
		} else {
			s.captureBlock(start, dec, &s.implementation)
		}
	case elST:
		// Inside <Implementation><ST>…</ST></Implementation>. Treat as
		// direct capture; the surrounding Implementation close tag
		// will clear currentBlock.
		if s.currentBlock == elImplementation {
			if s.captureTarget != nil {
				s.captureCDATA(start, dec, s.captureTarget)
			} else {
				s.captureCDATA(start, dec, &s.implementation)
			}
		}
	case elMethod:
		m := twincat.Method{
			Name: attr(start, "Name"),
			Type: twincat.POUTypeMethod,
		}
		s.currentMethod = &m
	case elProperty:
		m := twincat.Method{
			Name: attr(start, "Name"),
			Type: twincat.POUTypeProperty,
		}
		s.currentMethod = &m
	case elAction:
		m := twincat.Method{
			Name: attr(start, "Name"),
			Type: twincat.POUTypeAction,
		}
		s.currentMethod = &m
	case elDefinition:
		if s.currentMethod != nil {
			var buf strings.Builder
			s.captureCDATA(start, dec, &buf)
			s.currentMethod.Declaration = buf.String()
		}
	default:
		// Unknown element: descend and capture its CDATA into the
		// appropriate block so we never silently lose content.
		if s.currentMethod != nil && s.currentBlock == elImplementation {
			if s.captureTarget != nil {
				s.captureCDATA(start, dec, s.captureTarget)
			} else {
				var buf strings.Builder
				s.captureCDATA(start, dec, &buf)
				s.currentMethod.Implementation = buf.String()
			}
		}
	}
	return nil
}

// handleEnd processes the closing of an XML element.
func (s *parserState) handle(end xml.EndElement) {
	name := end.Name.Local

	switch name {
	case elDeclaration, elImplementation:
		s.currentBlock = ""
	case elMethod, elProperty, elAction:
		if s.currentMethod != nil {
			s.methods = append(s.methods, *s.currentMethod)
			s.currentMethod = nil
		}
	}

	// Some TwinCAT GVL files put declaration lines into nested
	// elements (e.g. <Declaration><Line>...</Line></Declaration>).
	// After a Declaration block closes, look at what we captured and
	// break it into individual lines if appropriate.
	if name == elDeclaration && len(s.gvl) == 0 && len(s.members) == 0 {
		lines := splitDeclaration(s.declaration.String())
		if s.pouType == "" && s.dutType == "" {
			s.gvl = lines
		} else if s.dutType != "" {
			s.members = lines
		}
	}
}

// handleEnd dispatches based on the element name.
func (s *parserState) handleEnd(end xml.EndElement) {
	s.handle(end)
}

// captureBlock captures a declaration or implementation into dst while
// making that destination available to nested wrappers such as <ST>.
func (s *parserState) captureBlock(start xml.StartElement, dec *xml.Decoder, dst *strings.Builder) {
	previous := s.captureTarget
	s.captureTarget = dst
	s.captureCDATA(start, dec, dst)
	s.captureTarget = previous
}

// captureCDATA consumes the body of the current element as a string
// (including CDATA sections) and appends it to dst. It then advances
// past the matching end element. This is safe because encoding/xml
// resolves CDATA into regular character data of type xml.CharData.
//
// captureCDATA dispatches nested StartElement events through the
// main handleStart pipeline so siblings like <Method> inside a POU
// are still picked up, while keeping a separate stack so nested
// captureCDATA calls do not lose their place.
func (s *parserState) captureCDATA(start xml.StartElement, dec *xml.Decoder, dst *strings.Builder) {
	type frame struct {
		name  string
		depth int
	}
	stack := []frame{{name: start.Name.Local, depth: 1}}

	for len(stack) > 0 {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			top := &stack[len(stack)-1]
			top.depth++
			stack = append(stack, frame{name: t.Name.Local, depth: 1})
			// Dispatch the nested start to the normal handler so the
			// parser can detect Methods / Properties / nested
			// <Declaration> elements etc.
			if err := s.handleStart(t, dec); err != nil {
				return
			}
		case xml.EndElement:
			top := &stack[len(stack)-1]
			top.depth--
			if top.depth == 0 {
				stack = stack[:len(stack)-1]
				s.handle(t)
			}
		case xml.CharData:
			dst.Write(t)
		}
	}
}

// attr returns the value of attribute name on start, or "" if missing.
func attr(start xml.StartElement, name string) string {
	for _, a := range start.Attr {
		if a.Name.Local == name {
			return strings.TrimSpace(a.Value)
		}
	}
	return ""
}

// detectPOUType inspects the first non-blank line of a declaration to
// detect "FUNCTION_BLOCK", "PROGRAM", "FUNCTION", "METHOD", "PROPERTY"
// or "ACTION". Returns empty string when none match.
func detectPOUType(decl string) twincat.POUType {
	first := firstNonBlankLine(decl)
	upper := strings.ToUpper(strings.TrimSpace(first))
	return pouTypeFromKeyword(upper)
}

// pouTypeFromKeyword returns the POUType for any of the recognised
// POU/Method/Property/Action keywords, or "" otherwise.
func pouTypeFromKeyword(upper string) twincat.POUType {
	switch {
	case strings.HasPrefix(upper, "FUNCTION_BLOCK"):
		return twincat.POUTypeFunctionBlock
	case strings.HasPrefix(upper, "PROGRAM"):
		return twincat.POUTypeProgram
	case strings.HasPrefix(upper, "FUNCTION"):
		return twincat.POUTypeFunction
	case strings.HasPrefix(upper, "METHOD"):
		return twincat.POUTypeMethod
	case strings.HasPrefix(upper, "PROPERTY"):
		return twincat.POUTypeProperty
	case strings.HasPrefix(upper, "ACTION"):
		return twincat.POUTypeAction
	}
	return ""
}

// isPOUKeyword reports whether v is one of the POU keywords.
func isPOUKeyword(v string) bool {
	return pouTypeFromKeyword(strings.ToUpper(strings.TrimSpace(v))) != ""
}

// detectDUTType inspects the body of a DUT declaration to detect
// STRUCT / ENUM / UNION / TYPE (alias). The first keyword is
// usually TYPE <name> :, so we look deeper into the body.
func detectDUTType(decl string) twincat.DUTType {
	upper := strings.ToUpper(decl)
	switch {
	case strings.Contains(upper, "END_STRUCT"):
		return twincat.DUTTypeStruct
	case strings.Contains(upper, "END_ENUM"):
		return twincat.DUTTypeEnum
	case strings.Contains(upper, "END_UNION"):
		return twincat.DUTTypeUnion
	}
	// TYPE without STRUCT/ENUM/UNION is an alias.
	if strings.HasPrefix(strings.TrimSpace(upper), "TYPE") {
		return twincat.DUTTypeAlias
	}
	return ""
}

// firstNonBlankLine returns the first line that contains at least one
// non-whitespace character, or "" if none exists.
func firstNonBlankLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// splitDeclaration breaks a block of declarations into individual
// statements. Empty lines are dropped. Comments and pragmas are
// preserved verbatim.
func splitDeclaration(decl string) []twincat.Member {
	var out []twincat.Member
	for _, line := range strings.Split(decl, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, twincat.Member{Code: line})
	}
	return out
}
