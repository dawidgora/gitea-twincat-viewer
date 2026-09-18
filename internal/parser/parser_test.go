package parser

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/dawidgora/gitea-twincat-viewer/internal/twincat"
)

// TestParseTcPOU exercises a complete generic TwinCAT POU document with a
// FUNCTION_BLOCK header, declarations, an assignment implementation, and one
// method.
func TestParseTcPOU(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8"?>
<TcPlcObject Version="1.1.0.1">
  <POU Name="FB_Boolean" KindOfPOU="Element">
    <Declaration><![CDATA[FUNCTION_BLOCK FB_Boolean
VAR_INPUT
    bInput  : BOOL;
END_VAR
VAR_OUTPUT
    bResult : BOOL;
END_VAR
]]></Declaration>
    <Implementation>
      <ST><![CDATA[bResult := NOT bInput;
]]></ST>
    </Implementation>
    <Method Name="Reset" KindOfMethod="Element">
      <Declaration><![CDATA[METHOD Reset : BOOL
VAR_INPUT
END_VAR
]]></Declaration>
      <Implementation><![CDATA[bResult := FALSE;]]></Implementation>
    </Method>
  </POU>
</TcPlcObject>`

	f, err := Parse([]byte(src), ".TcPOU")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if f.Name != "FB_Boolean" {
		t.Fatalf("Name = %q, want FB_Boolean", f.Name)
	}
	if f.Kind != twincat.KindPOU {
		t.Fatalf("Kind = %v, want KindPOU", f.Kind)
	}
	if f.POUType != twincat.POUTypeFunctionBlock {
		t.Fatalf("POUType = %v, want FUNCTION_BLOCK", f.POUType)
	}
	if !strings.Contains(f.Declaration, "FUNCTION_BLOCK FB_Boolean") {
		t.Fatalf("Declaration missing header: %q", f.Declaration)
	}
	if !strings.Contains(f.Declaration, "bInput  : BOOL;") {
		t.Fatalf("Declaration missing body: %q", f.Declaration)
	}
	if !strings.Contains(f.Implementation, "bResult := NOT bInput;") {
		t.Fatalf("Implementation missing assignment: %q", f.Implementation)
	}
	if len(f.Methods) != 1 {
		t.Fatalf("len(Methods) = %d, want 1", len(f.Methods))
	}
	if f.Methods[0].Name != "Reset" {
		t.Fatalf("Method[0].Name = %q, want Reset", f.Methods[0].Name)
	}
}

// TestParseTcDUT verifies a generic DUT with STRUCT.
func TestParseTcDUT(t *testing.T) {
	src := `<?xml version="1.0"?>
<TcPlcObject Version="1.1.0.1">
  <DUT Name="ST_Record">
    <Declaration><![CDATA[TYPE ST_Record :
STRUCT
    nValue : INT;
END_STRUCT
END_TYPE
]]></Declaration>
  </DUT>
</TcPlcObject>`

	f, err := Parse([]byte(src), ".TcDUT")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Name != "ST_Record" {
		t.Fatalf("Name = %q", f.Name)
	}
	if f.Kind != twincat.KindDUT {
		t.Fatalf("Kind = %v", f.Kind)
	}
	if f.DUTType != twincat.DUTTypeStruct {
		t.Fatalf("DUTType = %v, want STRUCT", f.DUTType)
	}
	if !strings.Contains(f.Declaration, "END_STRUCT") {
		t.Fatalf("Declaration missing END_STRUCT: %q", f.Declaration)
	}
}

// TestParseTcGVL exercises a generic Global Variable List.
func TestParseTcGVL(t *testing.T) {
	src := `<?xml version="1.0"?>
<TcPlcObject Version="1.1.0.1">
  <GVL Name="GVL_Shared">
    <Declaration><![CDATA[VAR_GLOBAL
    bReady : BOOL := TRUE;
END_VAR
]]></Declaration>
  </GVL>
</TcPlcObject>`

	f, err := Parse([]byte(src), ".TcGVL")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Name != "GVL_Shared" {
		t.Fatalf("Name = %q", f.Name)
	}
	if f.Kind != twincat.KindGVL {
		t.Fatalf("Kind = %v", f.Kind)
	}
	if !strings.Contains(f.Declaration, "VAR_GLOBAL") {
		t.Fatalf("Declaration missing VAR_GLOBAL: %q", f.Declaration)
	}
	if !strings.Contains(f.Declaration, "bReady : BOOL := TRUE;") {
		t.Fatalf("Declaration missing bReady: %q", f.Declaration)
	}
}

// TestParseCDATA verifies that a CDATA section inside an implementation
// is preserved verbatim, including characters that would otherwise be
// XML special characters.
func TestParseCDATA(t *testing.T) {
	src := `<?xml version="1.0"?>
<TcPlcObject Version="1.1.0.1">
  <POU Name="FB_X" KindOfPOU="Element">
    <Declaration><![CDATA[FUNCTION_BLOCK FB_X
VAR
    s : STRING := 'a < b & c > d';
END_VAR
]]></Declaration>
    <Implementation><![CDATA[IF s <> '' THEN
    ;
END_IF
]]></Implementation>
  </POU>
</TcPlcObject>`

	f, err := Parse([]byte(src), ".TcPOU")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(f.Declaration, "'a < b & c > d'") {
		t.Fatalf("Declaration CDATA not preserved: %q", f.Declaration)
	}
}

// TestParseSTComments verifies that comments inside the ST block are
// preserved in the captured declaration text.
func TestParseSTComments(t *testing.T) {
	src := `<?xml version="1.0"?>
<TcPlcObject Version="1.1.0.1">
  <POU Name="FB_C" KindOfPOU="Element">
    <Declaration><![CDATA[FUNCTION_BLOCK FB_C
VAR
    // line comment
    (* block comment *)
    n : INT;
END_VAR
]]></Declaration>
  </POU>
</TcPlcObject>`

	f, err := Parse([]byte(src), ".TcPOU")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(f.Declaration, "// line comment") {
		t.Fatalf("Declaration missing line comment: %q", f.Declaration)
	}
	if !strings.Contains(f.Declaration, "(* block comment *)") {
		t.Fatalf("Declaration missing block comment: %q", f.Declaration)
	}
}

// TestParseProperty verifies that PROPERTY elements are picked up as
// methods with the correct type label.
func TestParseProperty(t *testing.T) {
	src := `<?xml version="1.0"?>
<TcPlcObject Version="1.1.0.1">
  <POU Name="FB_P" KindOfPOU="Element">
    <Declaration><![CDATA[FUNCTION_BLOCK FB_P
VAR
    _n : INT;
END_VAR
]]></Declaration>
    <Implementation><![CDATA[bParent := TRUE;]]></Implementation>
    <Property Name="Value">
      <Declaration><![CDATA[PROPERTY Value : INT]]></Declaration>
      <Implementation><![CDATA[Value := _n;]]></Implementation>
    </Property>
  </POU>
</TcPlcObject>`

	f, err := Parse([]byte(src), ".TcPOU")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(f.Methods) != 1 {
		t.Fatalf("len(Methods) = %d, want 1", len(f.Methods))
	}
	if f.Methods[0].Type != twincat.POUTypeProperty {
		t.Fatalf("Method.Type = %v, want PROPERTY", f.Methods[0].Type)
	}
	if f.Methods[0].Name != "Value" {
		t.Fatalf("Method.Name = %q, want Value", f.Methods[0].Name)
	}
	if f.Methods[0].Declaration != "PROPERTY Value : INT" {
		t.Fatalf("Method.Declaration = %q, want property declaration", f.Methods[0].Declaration)
	}
	if f.Methods[0].Implementation != "Value := _n;" {
		t.Fatalf("Method.Implementation = %q, want property implementation", f.Methods[0].Implementation)
	}
	if strings.Contains(f.Declaration, "PROPERTY Value : INT") {
		t.Fatalf("POU declaration contaminated by property declaration: %q", f.Declaration)
	}
	if f.Implementation != "bParent := TRUE;" {
		t.Fatalf("POU implementation = %q, want parent implementation", f.Implementation)
	}
}

// TestParseCombinedFixtureMembers verifies the generic combined fixture
// exposes its method, property, and action as distinct POU members.
func TestParseCombinedFixtureMembers(t *testing.T) {
	data, err := os.ReadFile("../../examples/twincat/FB_Combined.TcPOU")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	f, err := Parse(data, ".TcPOU")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []struct {
		name string
		typ  twincat.POUType
	}{
		{name: "Status", typ: twincat.POUTypeProperty},
		{name: "Reset", typ: twincat.POUTypeMethod},
		{name: "Apply", typ: twincat.POUTypeAction},
	}
	if len(f.Methods) != len(want) {
		t.Fatalf("len(Methods) = %d, want %d", len(f.Methods), len(want))
	}
	for i, member := range want {
		got := f.Methods[i]
		if got.Name != member.name || got.Type != member.typ {
			t.Fatalf("Methods[%d] = (%q, %q), want (%q, %q)", i, got.Name, got.Type, member.name, member.typ)
		}
	}
	if !strings.Contains(f.Methods[1].Declaration, "METHOD Reset : BOOL") {
		t.Fatalf("method declaration missing header: %q", f.Methods[1].Declaration)
	}
	if !strings.Contains(f.Methods[2].Implementation, "sStatus := CONCAT(sLabel, ' applied');") {
		t.Fatalf("action implementation missing output assignment: %q", f.Methods[2].Implementation)
	}
}

// TestParseMalformedXML checks that invalid XML surfaces a parse error.
func TestParseMalformedXML(t *testing.T) {
	src := `<TcPlcObject><POU Name="FB_X"><Declaration><![CDATA[not closed`
	_, err := Parse([]byte(src), ".TcPOU")
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
	if !errors.Is(err, ErrMalformedXML) {
		t.Fatalf("expected ErrMalformedXML, got %v", err)
	}
}

// TestParseMissingRoot verifies that an empty document fails.
func TestParseMissingRoot(t *testing.T) {
	_, err := Parse([]byte(""), ".TcPOU")
	if err == nil {
		t.Fatal("expected error for empty document, got nil")
	}
}

// TestParseTooLarge verifies the size cap.
func TestParseTooLarge(t *testing.T) {
	big := make([]byte, MaxInputBytes+1)
	_, err := Parse(big, ".TcPOU")
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}

// TestParseUnknownExtension ensures a non-twinCAT extension still parses
// when content is provided, but maps to KindUnknown.
func TestParseUnknownExtension(t *testing.T) {
	src := `<TcPlcObject><GVL Name="GVL"><Declaration>VAR_GLOBAL x : INT; END_VAR</Declaration></GVL></TcPlcObject>`
	f, err := Parse([]byte(src), ".txt")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.Kind != twincat.KindGVL {
		t.Fatalf("Kind = %v, want KindGVL (inferred)", f.Kind)
	}
}
