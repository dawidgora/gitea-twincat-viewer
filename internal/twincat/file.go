// Package twincat defines the in-memory model produced by the parser.
//
// The model is intentionally small and framework-free: it only carries
// the TwinCAT concepts we render. It is decoupled from the Gitea
// renderer and from XML, so additional renderers (Forgejo, standalone
// web viewer, etc.) can reuse the same model.
package twincat

import "strings"

// Kind enumerates the supported TwinCAT XML file kinds.
type Kind int

const (
	KindUnknown Kind = iota
	KindPOU
	KindDUT
	KindGVL
)

// String returns a stable lower-case label used in output and logs.
func (k Kind) String() string {
	switch k {
	case KindPOU:
		return "pou"
	case KindDUT:
		return "dut"
	case KindGVL:
		return "gvl"
	default:
		return "unknown"
	}
}

// KindFromExtension maps a file extension (with or without leading dot)
// to the corresponding Kind. Unknown extensions return KindUnknown.
func KindFromExtension(ext string) Kind {
	e := strings.ToLower(strings.TrimPrefix(ext, "."))
	switch e {
	case "tcpou":
		return KindPOU
	case "tcdut":
		return KindDUT
	case "tcgvl":
		return KindGVL
	}
	return KindUnknown
}

// POUType identifies the structural type of a POU (Function Block,
// Program, Function, etc.).
type POUType string

const (
	POUTypeFunctionBlock POUType = "FUNCTION_BLOCK"
	POUTypeProgram       POUType = "PROGRAM"
	POUTypeFunction      POUType = "FUNCTION"
	POUTypeMethod        POUType = "METHOD"
	POUTypeProperty      POUType = "PROPERTY"
	POUTypeAction        POUType = "ACTION"
)

// DUTType identifies the structural type of a DUT.
type DUTType string

const (
	DUTTypeStruct DUTType = "STRUCT"
	DUTTypeEnum   DUTType = "ENUM"
	DUTTypeUnion  DUTType = "UNION"
	DUTTypeAlias  DUTType = "ALIAS"
)

// Member is a single declaration line (var / field / constant /
// attribute etc.) as it appears inside a Declaration section.
// Code holds raw Structured Text text and may span multiple lines.
type Member struct {
	Code string
}

// Method represents a METHOD, PROPERTY or ACTION inside a POU.
type Method struct {
	Name           string
	Type           POUType
	Declaration    string
	Implementation string
}

// File is the unified parsed representation of a TwinCAT XML file.
// Only fields relevant to the current file Kind are populated.
type File struct {
	Kind           Kind
	Name           string
	POUType        POUType
	DUTType        DUTType
	Declaration    string
	Implementation string
	Methods        []Method
	Members        []Member
	GVL            []Member
	Attributes     []string // raw <Attribute> entries
	Raw            string   // original document for diagnostics
}
