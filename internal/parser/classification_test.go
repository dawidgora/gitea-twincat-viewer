package parser

import (
	"encoding/xml"
	"os"
	"strings"
	"testing"

	"github.com/dawidgora/gitea-twincat-viewer/internal/twincat"
)

func TestInferKindUsesXMLObject(t *testing.T) {
	cases := []struct {
		xml, ext string
		want     twincat.Kind
	}{
		{`<TcPlcObject><POU Name="P"><Declaration>THING x;</Declaration></POU></TcPlcObject>`, "tmp", twincat.KindPOU},
		{`<TcPlcObject><DUT Name="D"><Declaration></Declaration></DUT></TcPlcObject>`, "", twincat.KindDUT},
		{`<TcPlcObject><GVL Name="G"/></TcPlcObject>`, "", twincat.KindGVL},
		{`<TcPlcObject><TcGVL Name="G"/></TcPlcObject>`, "", twincat.KindGVL},
		{`<TcPlcObject><POU Name="P"><Declaration></Declaration><Method Name="M"/></POU></TcPlcObject>`, "TcPOU", twincat.KindPOU},
	}
	for _, tc := range cases {
		f, err := Parse([]byte(tc.xml), tc.ext)
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != tc.want {
			t.Errorf("%s ext=%q: got %v want %v", tc.xml, tc.ext, f.Kind, tc.want)
		}
	}
}

func TestDetectPOUTypePrefixes(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want twincat.POUType
	}{
		{"\n// hi\n(* outer (* nested *) *) {attribute 'x}' } fUnCtIoN_bLoCk X", twincat.POUTypeFunctionBlock},
		{"{a := '(* not comment }'} PROGRAM P", twincat.POUTypeProgram},
		{"FUNCTIONAL", ""}, {"METHOD_EXTRA", ""}, {"(* unfinished PROGRAM", ""},
	} {
		if got := detectPOUType(tc.s); got != tc.want {
			t.Errorf("detectPOUType(%q)=%q want %q", tc.s, got, tc.want)
		}
	}
	for keyword, want := range map[string]twincat.POUType{"FUNCTION_BLOCK": twincat.POUTypeFunctionBlock, "PROGRAM": twincat.POUTypeProgram, "FUNCTION": twincat.POUTypeFunction, "METHOD": twincat.POUTypeMethod, "PROPERTY": twincat.POUTypeProperty, "ACTION": twincat.POUTypeAction} {
		decl := "{attribute 'note' := '$} (* //'}\n// ignored\n(* outer (* nested *) comment *)\n" + strings.ToLower(keyword) + " Name"
		if got := detectPOUType(decl); got != want {
			t.Errorf("keyword %s: got %q want %q", keyword, got, want)
		}
	}
}

func TestIssue9FixtureAndDeclarationMatrix(t *testing.T) {
	data, err := os.ReadFile("../../examples/twincat/Disposable_Issue9.TcPOU")
	if err != nil {
		t.Fatal(err)
	}
	var expected struct {
		POU struct {
			Declaration    string `xml:"Declaration"`
			Implementation struct {
				ST string `xml:"ST"`
			} `xml:"Implementation"`
			Methods []struct {
				Name           string `xml:"Name,attr"`
				Declaration    string `xml:"Declaration"`
				Implementation struct {
					ST string `xml:"ST"`
				} `xml:"Implementation"`
			} `xml:"Method"`
		} `xml:"POU"`
	}
	if err := xml.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"TcPOU", "", "tmp"} {
		f, err := Parse(data, ext)
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != twincat.KindPOU || f.POUType != twincat.POUTypeFunctionBlock || len(f.Methods) != 2 {
			t.Fatalf("fixture classification ext=%q: kind=%v pou=%v methods=%d", ext, f.Kind, f.POUType, len(f.Methods))
		}
		if f.Raw != string(data) || f.Declaration != strings.TrimSpace(expected.POU.Declaration) || f.Implementation != strings.TrimSpace(expected.POU.Implementation.ST) || len(f.GVL) != 0 {
			t.Fatalf("fixture fields changed ext=%q: declaration=%q implementation=%q gvl=%v", ext, f.Declaration, f.Implementation, f.GVL)
		}
		for i, want := range expected.POU.Methods {
			got := f.Methods[i]
			if got.Name != want.Name || got.Type != twincat.POUTypeMethod || got.Declaration != strings.TrimSpace(want.Declaration) || got.Implementation != strings.TrimSpace(want.Implementation.ST) {
				t.Errorf("method %d changed: got %#v want name=%q declaration=%q implementation=%q", i, got, want.Name, strings.TrimSpace(want.Declaration), strings.TrimSpace(want.Implementation.ST))
			}
		}
	}
	for _, tc := range []struct {
		src  string
		want twincat.POUType
	}{
		{"FUNCTION_BLOCK X", twincat.POUTypeFunctionBlock}, {"PROGRAM P", twincat.POUTypeProgram}, {"FUNCTION F", twincat.POUTypeFunction}, {"METHOD M", twincat.POUTypeMethod}, {"PROPERTY P", twincat.POUTypeProperty}, {"ACTION A", twincat.POUTypeAction},
		{"(* c *) {x := '(* }'} function_block X", twincat.POUTypeFunctionBlock}, {"(* broken PROGRAM", ""}, {"@ PROGRAM", ""},
	} {
		if got := detectPOUType(tc.src); got != tc.want {
			t.Errorf("%q: %q want %q", tc.src, got, tc.want)
		}
	}
}

func TestExplicitExtensionAndGVLClassification(t *testing.T) {
	for _, tc := range []struct {
		xml, ext string
		kind     twincat.Kind
	}{
		{`<TcPlcObject><POU Name="P"><Declaration>NOT_A_TYPE</Declaration><Method Name="M"/></POU></TcPlcObject>`, "", twincat.KindPOU},
		{`<TcPlcObject><DUT Name="D"/></TcPlcObject>`, "", twincat.KindDUT},
		{"<TcPlcObject><GVL Name=\"G\"><Declaration>// STRUCT END_STRUCT\nx : STRING := 'FUNCTION';</Declaration></GVL></TcPlcObject>", "", twincat.KindGVL},
		{`<TcPlcObject><POU Name="P"/></TcPlcObject>`, "TcGVL", twincat.KindGVL},
		{`<TcPlcObject><TcDUT Name="D"/></TcPlcObject>`, "TcPOU", twincat.KindPOU},
	} {
		f, err := Parse([]byte(tc.xml), tc.ext)
		if err != nil {
			t.Fatal(err)
		}
		if f.Kind != tc.kind {
			t.Errorf("ext %q got %v want %v", tc.ext, f.Kind, tc.kind)
		}
	}
}

func TestCanonicalKindOfPOUAndUnclassifiedKinds(t *testing.T) {
	for _, tc := range []struct {
		attr, decl string
		want       twincat.POUType
	}{
		{"  fUnCtIoN  ", "PROGRAM P", twincat.POUTypeFunction},
		{" FUNCTION_BLOCK ", "", twincat.POUTypeFunctionBlock},
	} {
		xml := `<TcPlcObject><POU Name="P" KindOfPOU="` + tc.attr + `"><Declaration>` + tc.decl + `</Declaration></POU></TcPlcObject>`
		f, err := Parse([]byte(xml), "")
		if err != nil {
			t.Fatal(err)
		}
		if f.POUType != tc.want {
			t.Errorf("KindOfPOU %q got %q want %q", tc.attr, f.POUType, tc.want)
		}
		if len(f.GVL) != 0 {
			t.Errorf("POU declaration contaminated GVL: %#v", f.GVL)
		}
	}
	for _, xml := range []string{`<TcPlcObject><POU Name="P"><Declaration>NOT_A_KEYWORD x;</Declaration></POU></TcPlcObject>`, `<TcPlcObject><DUT Name="D"><Declaration>NOT_A_TYPE x;</Declaration></DUT></TcPlcObject>`} {
		f, err := Parse([]byte(xml), "")
		if err != nil {
			t.Fatal(err)
		}
		if len(f.GVL) != 0 {
			t.Errorf("unclassified %v contaminated GVL: %#v", f.Kind, f.GVL)
		}
	}
}

func TestDetectDUTTypeIgnoresCommentsAndStrings(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want twincat.DUTType
	}{
		{"(* END_STRUCT *) TYPE X : STRUCT a : INT; END_STRUCT END_TYPE", twincat.DUTTypeStruct},
		{"TYPE X : UNION a : INT; END_UNION END_TYPE", twincat.DUTTypeUnion},
		{"TYPE X : (A, B, C); END_TYPE", twincat.DUTTypeEnum},
		{"(* prefix *) TYPE X : STRING := 'END_STRUCT'; END_TYPE", twincat.DUTTypeAlias},
		{"TYPE X : STRING; (* END_UNION *) END_TYPE", twincat.DUTTypeAlias},
		{"// { comment with newline\nTYPE X : STRUCT END_STRUCT", twincat.DUTTypeStruct},
		{"TYPE {attribute 'x'} X : {attribute 'y'} UNION END_UNION", twincat.DUTTypeUnion},
		{"(* { nested (* brace *) *) TYPE X : (A,B);", twincat.DUTTypeEnum},
		{"TYPE X : STRING := 'brace } $''); END_TYPE", twincat.DUTTypeAlias},
		{"TYPE X : STRING := 'brace } $\"'; END_TYPE", twincat.DUTTypeAlias},
		{"TYPE X : STRING := 'dollar $$ and \\\\'; END_TYPE", twincat.DUTTypeAlias},
		{"TYPE X : STRING := 'one backslash \\'; END_TYPE", twincat.DUTTypeAlias},
		{"TYPE X : STRING := 'unterminated", ""},
		{"(* unterminated TYPE X : STRUCT", ""},
		{"{attribute 'unterminated} TYPE X : STRUCT", ""},
		{"TYPE X :", ""},
	} {
		if got := detectDUTType(tc.s); got != tc.want {
			t.Errorf("detectDUTType(%q)=%q want %q", tc.s, got, tc.want)
		}
	}
	for _, tc := range []struct {
		s, want string
		ok      bool
	}{
		{`{attribute '$}'} FUNCTION X`, "FUNCTION", true},
		{`{attribute '$"}'} PROGRAM X`, "PROGRAM", true},
		{`{attribute '$$'} ACTION X`, "ACTION", true},
		{`{attribute 'literal \'} METHOD X`, "METHOD", true},
	} {
		got, ok := firstDeclarationToken(tc.s)
		if got != tc.want || ok != tc.ok {
			t.Errorf("firstDeclarationToken(%q)=(%q,%v) want (%q,%v)", tc.s, got, ok, tc.want, tc.ok)
		}
	}
}
