package renderer

import (
	"strings"
	"testing"

	"github.com/dawidgora/gitea-twincat-viewer/internal/twincat"
)

// TestRenderPOU exercises the happy path for a POU file.
func TestRenderPOU(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Room",
		POUType: twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK FB_Room
VAR_INPUT
    bPresence : BOOL;
END_VAR`,
		Implementation: `IF bPresence THEN
    nCount := nCount + 1;
END_IF`,
	}

	html := Render(f, Options{})

	mustContain(t, html, "FB_Room")
	mustContain(t, html, "FUNCTION_BLOCK")
	// Inline styles must be used for syntax colours; classes are stripped
	// by Gitea's sanitizer and would not render.
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">FUNCTION_BLOCK</span>`)
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">VAR_INPUT</span>`)
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">IF</span>`)
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">END_IF</span>`)
	// Gitea wants a fragment, so Render must NOT emit a full document.
	if strings.Contains(html, "<!DOCTYPE") {
		t.Fatalf("Render should emit a fragment, not a full HTML document")
	}
	if !strings.Contains(html, `class="tcv"`) {
		t.Fatalf("Render should wrap content in a tcv container")
	}
}

// TestRenderStandalone verifies the standalone (full document)
// variant still works for tests, debugging, and external viewers.
func TestRenderStandalone(t *testing.T) {
	f := &twincat.File{
		Kind:        twincat.KindPOU,
		Name:        "FB_Room",
		POUType:     twincat.POUTypeFunctionBlock,
		Declaration: "FUNCTION_BLOCK FB_Room",
	}
	html := RenderStandalone(f, Options{})
	mustContain(t, html, "DOCTYPE html")
	mustContain(t, html, "prefers-color-scheme")
	mustContain(t, html, `[data-theme=dark] .tcv`)
	mustContain(t, html, `.theme-dark .tcv`)
	mustContain(t, html, "FB_Room")
}

func TestThemePaletteStates(t *testing.T) {
	if !strings.Contains(css, `[data-theme=gitea-light] .tcv`) {
		t.Fatalf("missing explicit Gitea light palette selector")
	}
	if !strings.Contains(css, `[data-theme=gitea-dark] .tcv`) {
		t.Fatalf("missing explicit Gitea dark palette selector")
	}
	if !strings.Contains(css, `@media (prefers-color-scheme: dark) {
  [data-theme=gitea-auto] .tcv`) {
		t.Fatalf("OS dark preference must be scoped to gitea-auto")
	}
	if strings.Contains(css, `@media (prefers-color-scheme: dark) {
  .tcv`) {
		t.Fatalf("OS dark preference must not override explicit themes")
	}
}

// TestRenderDUT exercises STRUCT/ENUM/UNION/ALIAS DUT rendering.
func TestRenderDUT(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindDUT,
		Name:    "ST_RoomInputs",
		DUTType: twincat.DUTTypeStruct,
		Declaration: `TYPE ST_RoomInputs :
STRUCT
    bLightOn  : BOOL;
END_STRUCT
END_TYPE`,
	}
	html := Render(f, Options{})
	mustContain(t, html, "ST_RoomInputs")
	mustContain(t, html, "STRUCT")
	mustContain(t, html, "END_STRUCT")
	mustContain(t, html, "END_TYPE")
	mustContain(t, html, `<section class="tcv-block" aria-label="ST_RoomInputs TYPE">`)
	mustContain(t, html, `<div class="tcv-label tcv-unit-label"><span class="tcv-unit-name">ST_RoomInputs</span><span class="tcv-unit-type">TYPE</span></div>`)
	if strings.Contains(html, `<div class="tcv-label tcv-section-label">Declaration</div>`) {
		t.Fatalf("DUT title must identify the named TYPE")
	}
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">STRUCT</span>`)
	mustContain(t, html, `style="color:#953800;color:var(--tcv-type);font-weight:600">BOOL</span>`)
}

// TestRenderGVL exercises the global variable list rendering.
func TestRenderGVL(t *testing.T) {
	f := &twincat.File{
		Kind:        twincat.KindGVL,
		Name:        "GVL_Home",
		Declaration: "VAR_GLOBAL\n    n : INT;\nEND_VAR",
	}
	html := Render(f, Options{})
	mustContain(t, html, `<section class="tcv-block" aria-label="GVL_Home GLOBAL_VARIABLE_LIST">`)
	mustContain(t, html, `<div class="tcv-label tcv-unit-label"><span class="tcv-unit-name">GVL_Home</span><span class="tcv-unit-type">GLOBAL_VARIABLE_LIST</span></div>`)
	if strings.Contains(html, `<div class="tcv-label tcv-section-label">Declaration</div>`) {
		t.Fatalf("GVL title must identify the named global variable list")
	}
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">VAR_GLOBAL</span>`)
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">END_VAR</span>`)
	mustContain(t, html, `style="color:#953800;color:var(--tcv-type);font-weight:600">INT</span>`)
}

// TestRenderHTMLEscaping verifies that POU names containing HTML
// metacharacters are escaped and cannot break out of attribute or
// text contexts.
func TestRenderHTMLEscaping(t *testing.T) {
	f := &twincat.File{
		Kind:        twincat.KindPOU,
		Name:        `<script>alert("xss")</script>`,
		POUType:     twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK "<img onerror=x>"`,
	}
	html := Render(f, Options{})
	if strings.Contains(html, "<script>alert") {
		t.Fatalf("renderer emitted raw script tag: %q", html)
	}
	if !strings.Contains(html, "&lt;img onerror=x&gt;") {
		t.Fatalf("expected escaped img tag in declaration: %q", html)
	}
}

// TestRenderNoJavaScript ensures the renderer never emits <script>.
func TestRenderNoJavaScript(t *testing.T) {
	f := &twincat.File{
		Kind:        twincat.KindPOU,
		Name:        "FB_X",
		POUType:     twincat.POUTypeFunctionBlock,
		Declaration: "FUNCTION_BLOCK FB_X\nVAR END_VAR",
	}
	html := Render(f, Options{})
	if strings.Contains(html, "<script") {
		t.Fatalf("renderer emitted script tag")
	}
}

// TestRenderStringsAndComments verifies string literals and comments
// are wrapped in inline-styled spans.
func TestRenderStringsAndComments(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_S",
		POUType: twincat.POUTypeFunctionBlock,
		Implementation: `// comment line
s := 'a string';
(* block comment *)
IF TRUE THEN
    ;
END_IF`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `style="color:#6e7781;color:var(--tcv-com);font-style:italic">// comment line</span>`)
	mustContain(t, html, `style="color:#6e7781;color:var(--tcv-com);font-style:italic">(* block comment *)</span>`)
	mustContain(t, html, `style="color:#0a3069;color:var(--tcv-str)">&#39;a string&#39;</span>`)
	mustContain(t, html, `style="color:#0550ae;color:var(--tcv-lit)">TRUE</span>`)
	mustContain(t, html, `style="color:#cf222e;color:var(--tcv-kw);font-weight:600">END_IF</span>`)
}

// TestRenderNumbers verifies numeric literals get inline number styles.
func TestRenderNumbers(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_N",
		POUType: twincat.POUTypeFunctionBlock,
		Implementation: `n := 42;
x := 3.14;`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `style="color:#0550ae;color:var(--tcv-num)">42</span>`)
	mustContain(t, html, `style="color:#0550ae;color:var(--tcv-num)">3.14</span>`)
}

// TestRenderUnknownKind ensures unknown file kinds still produce a valid
// empty fragment without inventing a renderer-owned title or badge.
func TestRenderUnknownKind(t *testing.T) {
	f := &twincat.File{Name: "FB_X"}
	html := Render(f, Options{})
	if strings.Contains(html, "Unknown") || strings.Contains(html, "FB_X") {
		t.Fatalf("unknown file metadata should not be rendered: %q", html)
	}
}

// TestNoHighlightClasses verifies that highlightST never emits
// class-based spans that Gitea's sanitizer would strip.
func TestNoHighlightClasses(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_ClassProbe",
		POUType: twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK FB_ClassProbe
VAR_INPUT
    b : BOOL;
END_VAR`,
		Implementation: `// comment
IF TRUE THEN
    x := 42;
    s := 'hi';
END_IF`,
	}
	html := Render(f, Options{})
	if strings.Contains(html, `class="tck-`) {
		t.Fatalf("highlight output must not contain tck-* classes: %q", html)
	}
	if strings.Contains(html, `class='tck-`) {
		t.Fatalf("highlight output must not contain tck-* classes: %q", html)
	}
}

// TestLineNumbers verifies that code blocks use Gitea's native line gutter.
func TestLineNumbers(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Lines",
		POUType: twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK FB_Lines
VAR
    x : INT;
END_VAR`,
		Implementation: `x := 1;
x := 2;
x := 3;`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `<div class="file-view code-view"><table><tbody>`)
	mustContain(t, html, `<tr><td class="lines-num"><span data-line-number="1"></span></td>`)
	mustContain(t, html, `<span data-line-number="4"></span>`)
	if strings.Count(html, `<tr><td class="lines-num">`) != 7 {
		t.Fatalf("expected one native table row per source line across both listings")
	}
	for _, number := range []string{"1", "2", "3"} {
		if strings.Count(html, `data-line-number="`+number+`"`) != 2 {
			t.Fatalf("expected line number %s in each separate listing", number)
		}
	}
	if strings.Count(html, `data-line-number="4"`) != 1 {
		t.Fatalf("expected declaration listing to contain line number 4 once")
	}
	if strings.Contains(html, `class="tcv-line"`) {
		t.Fatalf("line gutters must use per-listing data attributes without custom wrappers")
	}
}

// TestCodeViewMinimal verifies the fragment is a native code listing rather
// than a renderer-owned title/detail page.
func TestCodeViewMinimal(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Meta",
		POUType: twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK FB_Meta
VAR_INPUT
    b : BOOL;
END_VAR`,
		Implementation: `b := TRUE;`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `<div class="tcv">`)
	mustContain(t, html, `<div class="file-view code-view"><table><tbody>`)
	mustContain(t, html, `<td class="lines-code chroma"><code class="code-inner">`)
	if strings.Contains(html, "<pre") || strings.Contains(html, `class="tcv-line"`) {
		t.Fatalf("code view must use native table rows, not pre/custom line wrappers")
	}
	if strings.Contains(html, `<h1`) || strings.Contains(html, `<aside`) {
		t.Fatalf("code view should not emit a title bar or summary aside")
	}
	for _, forbidden := range []string{"tcv-header", "tcv-name", "tcv-badge", "tcv-summary", "tcv-chip", "446 B", "29 lines", "TwinCAT Structured Text"} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("code view should not emit %q", forbidden)
		}
	}
}

// TestSectionHeader verifies logical blocks have compact labels and listings.
func TestSectionHeader(t *testing.T) {
	f := &twincat.File{
		Kind:           twincat.KindPOU,
		Name:           "FB_Section",
		POUType:        twincat.POUTypeFunctionBlock,
		Declaration:    "FUNCTION_BLOCK FB_Section\nVAR END_VAR",
		Implementation: `;`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `<section class="tcv-block tcv-pou" aria-label="FUNCTION_BLOCK FB_Section">`)
	mustContain(t, html, `<div class="tcv-label tcv-unit-label"><span class="tcv-unit-name">FB_Section</span><span class="tcv-unit-type">FUNCTION_BLOCK</span></div>`)
	if strings.Contains(html, "tcv-pou-header") || strings.Contains(html, "tcv-pou-name") || strings.Contains(html, "tcv-pou-type") {
		t.Fatalf("POU heading must use the shared unit-label markup")
	}
	mustContain(t, html, `<div class="file-view code-view"><table><tbody>`)
	if strings.Count(html, `<div class="file-view code-view">`) != 2 {
		t.Fatalf("expected separate declaration and implementation listings")
	}
}

// TestNoBackgroundPanels verifies the listing does not create a competing card.
func TestNoBackgroundPanels(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Panels",
		POUType: twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK FB_Panels
VAR_INPUT
    b : BOOL;
END_VAR`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `<div class="file-view code-view"><table><tbody>`)

	if strings.Contains(css, ".tcv-code") || strings.Contains(css, ".tcv-line") || strings.Contains(css, "border-right") {
		t.Fatalf("renderer must not recreate Gitea's code surface or gutter CSS")
	}
}

// TestMethodsRemainNavigable verifies method names remain semantic headings.
func TestMethodsRemainNavigable(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Summary",
		POUType: twincat.POUTypeFunctionBlock,
		Declaration: `FUNCTION_BLOCK FB_Summary
VAR_INPUT
    b : BOOL;
END_VAR
VAR_OUTPUT
    o : BOOL;
END_VAR`,
		Methods: []twincat.Method{
			{Name: "Reset", Type: twincat.POUTypeMethod, Declaration: "METHOD Reset", Implementation: "Reset();"},
			{Name: "IsOn", Type: twincat.POUTypeProperty, Implementation: "IsOn := TRUE;"},
			{Name: "ApplyOutput", Type: twincat.POUTypeAction, Implementation: "rActual := rBrightness;"},
		},
	}
	html := Render(f, Options{})
	for _, label := range []string{"METHOD Reset", "PROPERTY IsOn", "ACTION ApplyOutput"} {
		mustContain(t, html, `<details class="tcv-block`)
		mustContain(t, html, `aria-label="`+label+`"`)
		parts := strings.SplitN(label, " ", 2)
		mustContain(t, html, `<summary class="tcv-label tcv-unit-label"><span class="tcv-disclosure" aria-hidden="true">&#9656;</span><span class="tcv-unit-name">`+parts[1]+`</span><span class="tcv-unit-type">`+parts[0]+`</span></summary>`)
	}
	mustContain(t, html, `<details class="tcv-block tcv-member" open aria-label="METHOD Reset">`)
	mustContain(t, html, `<details class="tcv-block tcv-property" open aria-label="PROPERTY IsOn">`)
	mustContain(t, html, `<details class="tcv-block tcv-member" open aria-label="ACTION ApplyOutput">`)
	if !strings.Contains(html, "Reset") || !strings.Contains(html, "IsOn") || !strings.Contains(html, "ApplyOutput") {
		t.Fatalf("member source should remain in the listing")
	}
	if strings.Count(html, `<div class="file-view code-view">`) != 5 {
		t.Fatalf("declaration and each method should have separate listings")
	}
	methodStart := strings.Index(html, `aria-label="METHOD Reset"`)
	methodEnd := strings.Index(html[methodStart+1:], `<details class="tcv-block`)
	if methodStart == -1 || methodEnd == -1 {
		t.Fatalf("method unit should be rendered as a section")
	}
	methodHTML := html[methodStart : methodStart+1+methodEnd]
	if !strings.Contains(methodHTML, `<div class="tcv-subsection tcv-member-part" aria-label="Declaration">`) ||
		!strings.Contains(methodHTML, `<div class="tcv-subsection tcv-member-part" aria-label="Implementation">`) {
		t.Fatalf("method declaration and implementation must be separate subsections")
	}
	if strings.Contains(html, "tcv-subtitle") {
		t.Fatalf("method blocks should not have nested subsection chrome")
	}
	actionStart := strings.Index(html, `aria-label="ACTION ApplyOutput"`)
	actionEnd := strings.Index(html[actionStart:], `</details>`)
	if actionStart == -1 || actionEnd == -1 || strings.Contains(html[actionStart:actionStart+actionEnd], `aria-label="Declaration"`) {
		t.Fatalf("actions without declarations must show implementation only")
	}
}

func TestRenderPropertyKeepsDeclarationAndImplementationSeparate(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Property",
		POUType: twincat.POUTypeFunctionBlock,
		Methods: []twincat.Method{{
			Name:           "Value",
			Type:           twincat.POUTypeProperty,
			Declaration:    "PROPERTY Value : INT",
			Implementation: "Value := nValue;",
		}},
	}
	html := Render(f, Options{})
	mustContain(t, html, `<details class="tcv-block tcv-property" open aria-label="PROPERTY Value">`)
	mustContain(t, html, `<summary class="tcv-label tcv-unit-label"><span class="tcv-disclosure" aria-hidden="true">&#9656;</span><span class="tcv-unit-name">Value</span><span class="tcv-unit-type">PROPERTY</span></summary>`)
	mustContain(t, html, `<div class="tcv-subsection tcv-property-part" aria-label="Declaration">`)
	mustContain(t, html, `<div class="tcv-subsection tcv-property-part" aria-label="Implementation">`)
	if strings.Contains(html, `<div class="tcv-label tcv-section-label">Declaration</div>`) ||
		strings.Contains(html, `<div class="tcv-label tcv-section-label">Implementation</div>`) {
		t.Fatalf("code areas must not have visible Declaration or Implementation labels")
	}
	if strings.Contains(html, "Property Declaration") || strings.Contains(html, "Property Implementation") {
		t.Fatalf("property child labels should inherit context from their parent")
	}
	if strings.Count(html, `<div class="file-view code-view">`) != 2 {
		t.Fatalf("property declaration and implementation must have separate listings")
	}
	if strings.Index(html, "PROPERTY Value") > strings.Index(html, "Declaration") ||
		strings.Index(html, "Declaration") > strings.Index(html, "Implementation") {
		t.Fatalf("property parts are out of source order")
	}
}

func TestCompactCodeViewDensity(t *testing.T) {
	if strings.Contains(css, "min-height:") || strings.Contains(css, ".tcv-code") || strings.Contains(css, ".tcv-line") {
		t.Fatalf("renderer must defer code density to Gitea's native code-view CSS")
	}
}

func TestNativeCodeViewMetricsOverrideMarkup(t *testing.T) {
	for _, selector := range []string{
		`.markup .tcv .file-view.code-view table {`,
		`.markup .tcv .file-view.code-view tr {`,
		`.markup .tcv .file-view.code-view td.lines-num {`,
		`.markup .tcv .file-view.code-view td.lines-code {`,
	} {
		if !strings.Contains(css, selector) {
			t.Fatalf("missing scoped native metric selector %q", selector)
		}
	}
	if !strings.Contains(css, "height: 20px; line-height: 20px") {
		t.Fatalf("code rows must match native 20px metrics")
	}
	if !strings.Contains(css, "font-size: 12px; line-height: 20px") {
		t.Fatalf("code cells must match native typography metrics")
	}
	if !strings.Contains(css, "td.lines-num { padding: 0 8px !important; }") ||
		!strings.Contains(css, "td.lines-code { padding: 0 1px 0 5px !important; }") {
		t.Fatalf("code cells must not inherit Markdown table padding")
	}
	if !strings.Contains(css, "border: 0 !important") {
		t.Fatalf("code cells must override Markdown table borders")
	}
	if !strings.Contains(css, ".markup .tcv .file-view.code-view tr {\n  border: 0 !important;") {
		t.Fatalf("code rows must override Markdown table borders")
	}
	if !strings.Contains(css, ".markup .tcv .file-view.code-view code.code-inner {") ||
		!strings.Contains(css, "background: transparent !important") ||
		!strings.Contains(css, "border-radius: 0 !important") ||
		!strings.Contains(css, "padding: 0 !important") {
		t.Fatalf("code contents must reset inherited Markdown styling")
	}
	if !strings.Contains(css, "font-family: var(--font-family-monospace,") ||
		!strings.Contains(css, "white-space: pre;") {
		t.Fatalf("code contents must use a monospaced font and preserve authored whitespace")
	}
	if !strings.Contains(css, "border-collapse: collapse; border-spacing: 0") {
		t.Fatalf("code table must not add Markdown spacing")
	}
	if !strings.Contains(css, `.file-view.markup.twincat:has(.tcv) { padding: 0 !important; }`) {
		t.Fatalf("renderer must only remove padding from its external TwinCAT host")
	}
	if !strings.Contains(css, `.markup .tcv .file-view.code-view tr:nth-child(even)`) ||
		!strings.Contains(css, "background: transparent !important") {
		t.Fatalf("renderer code tables must clear alternating row backgrounds")
	}
	if strings.Contains(css, ">") {
		t.Fatalf("renderer CSS selectors must not contain encoded child combinators")
	}
	if strings.Contains(css, ".tcv .tcv-property-label") {
		t.Fatalf("property headers must use the shared unit-label class")
	}
	if !strings.Contains(css, `.tcv .tcv-block + .tcv-block {`) ||
		!strings.Contains(css, "border-top: 1px solid var(--color-secondary") {
		t.Fatalf("logical sections need restrained visual boundaries")
	}
	if !strings.Contains(css, `.tcv .tcv-section-label {`) ||
		!strings.Contains(css, "color: var(--tcv-muted); font-size: 12px") {
		t.Fatalf("all subsection labels must use the same muted style")
	}
	if !strings.Contains(css, `.tcv .tcv-subsection { margin-left: 1rem; }`) {
		t.Fatalf("all unit subsections must share the same nested offset")
	}
	if !strings.Contains(css, `.tcv .tcv-subsection + .tcv-subsection {`) {
		t.Fatalf("adjacent code areas need one internal divider")
	}
	if !strings.Contains(css, `::-webkit-details-marker`) {
		t.Fatalf("native summary markers must be hidden before rendering the compact chevron")
	}
	if strings.Contains(css, "content:") || !strings.Contains(css, `.tcv .tcv-disclosure {`) ||
		!strings.Contains(css, `.tcv .tcv-member[open] .tcv-disclosure`) ||
		!strings.Contains(css, `.tcv .tcv-property[open] .tcv-disclosure`) {
		t.Fatalf("disclosure icons must use static markup with CSS open-state rotation")
	}
	if !strings.Contains(css, `.markup .tcv .file-view.code-view tr:hover,`) {
		t.Fatalf("native row hover must be explicitly neutralized")
	}
	if !strings.Contains(css, `.tcv .tcv-label {`) ||
		!strings.Contains(css, "user-select: none") {
		t.Fatalf("logical section labels must not be selectable")
	}
	if !strings.Contains(css, `.tcv .file-view.code-view td.lines-num { user-select: none; }`) {
		t.Fatalf("line-number gutters must remain non-selectable")
	}
}

// TestFunctionCallHighlighting verifies identifiers followed by '(' are
// styled as function calls.
func TestFunctionCallHighlighting(t *testing.T) {
	f := &twincat.File{
		Kind:    twincat.KindPOU,
		Name:    "FB_Fn",
		POUType: twincat.POUTypeFunctionBlock,
		Implementation: `MyFunc();
MyFunc ();`,
	}
	html := Render(f, Options{})
	mustContain(t, html, `style="color:#8250df;color:var(--tcv-fn)">MyFunc</span>`)
}

// mustContain fails the test if substr is not found in s.
func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected output to contain %q\n\nactual:\n%s", substr, s)
	}
}
