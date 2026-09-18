// Package renderer produces HTML from a parsed twincat.File. Output is
// a complete, self-contained HTML5 document with inline CSS. The
// styles use only color, layout, and typography - no JavaScript, no
// remote resources - so the document renders safely inside Gitea and
// inherits the host application's light/dark theme where possible.
package renderer

import (
	"fmt"
	"html"
	"strings"

	"github.com/dawidgora/gitea-twincat-viewer/internal/twincat"
)

// Options controls HTML output. The zero value is valid.
type Options struct {
	Title    string // page <title>; defaults to the file name
	CSSClass string // optional container class
}

// Render converts the parsed file into HTML.
//
// Gitea's external renderer contract expects a *fragment*, not a
// full HTML document: Gitea inserts the renderer's stdout inside its
// own `<pre><code>` and applies its own sanitiser. Emitting a full
// document would cause Gitea to show our <html><head> tags as text
// inside the code block. To support both the Gitea pipeline and
// standalone use (e.g. `curl ... | less`, GitHub Pages, etc.) we
// emit a fragment that Gitea can drop straight into its code view.
//
// The fragment is wrapped in a `<div class="tcv">` containing one
// code-oriented listing and subtle section labels. All user-supplied text is HTML-escaped;
// the only emitted HTML tags are the structural ones below and the
// highlight `<span>` wrappers produced by highlightST.
func Render(f *twincat.File, opts Options) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<div class=\"tcv\"%s>\n", cssClassAttr(opts.CSSClass))
	b.WriteString("<style>\n")
	b.WriteString(css)
	b.WriteString("\n</style>\n")
	b.WriteString(renderBody(f))
	b.WriteString("</div>\n")
	return b.String()
}

// RenderStandalone converts the parsed file into a complete HTML
// document with inline CSS. This is used by tests and by callers who
// need a viewable artifact (PDF preview, file:// usage, etc.). The
// output is safe to open in a browser; it is not what we send to
// Gitea's external renderer pipeline.
func RenderStandalone(f *twincat.File, opts Options) string {
	title := opts.Title
	if title == "" {
		title = f.Name
	}
	if title == "" {
		title = "TwinCAT"
	}

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n")
	fmt.Fprintf(&b, "<html lang=\"en\"%s>\n", cssClassAttr(opts.CSSClass))
	b.WriteString("<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString("<style>\n")
	b.WriteString(css)
	b.WriteString("\n</style>\n")
	b.WriteString("</head>\n<body>\n")
	b.WriteString("<div class=\"tcv\">\n")
	b.WriteString(renderBody(f))
	b.WriteString("</div>\n")
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

func cssClassAttr(c string) string {
	if c == "" {
		return ""
	}
	return " class=\"" + html.EscapeString(c) + "\""
}

// renderBody keeps the model's logical blocks separate while retaining the
// order in which TwinCAT presents them.
func renderBody(f *twincat.File) string {
	type block struct {
		label, code string
		unitType    string
		rendered    string
	}
	var blocks []block
	switch f.Kind {
	case twincat.KindPOU:
		blocks = append(blocks, block{rendered: renderPOU(f)})
	case twincat.KindDUT:
		if strings.TrimSpace(f.Declaration) != "" {
			blocks = append(blocks, block{label: f.Name, unitType: fileUnitType(f.Kind), code: f.Declaration})
		}
		if len(f.Members) > 0 {
			var mb strings.Builder
			for _, m := range f.Members {
				mb.WriteString(m.Code)
				mb.WriteString("\n")
			}
			blocks = append(blocks, block{label: "Members", code: strings.TrimSuffix(mb.String(), "\n")})
		}
	case twincat.KindGVL:
		if strings.TrimSpace(f.Declaration) != "" {
			blocks = append(blocks, block{label: f.Name, unitType: fileUnitType(f.Kind), code: f.Declaration})
		}
		if f.Declaration == "" && len(f.GVL) > 0 {
			var mb strings.Builder
			for _, m := range f.GVL {
				mb.WriteString(m.Code)
				mb.WriteString("\n")
			}
			blocks = append(blocks, block{label: "Declarations", code: strings.TrimSuffix(mb.String(), "\n")})
		}
	}
	if len(f.Attributes) > 0 {
		var ab strings.Builder
		for _, a := range f.Attributes {
			ab.WriteString(a)
			ab.WriteString("\n")
		}
		blocks = append(blocks, block{label: "Metadata", code: strings.TrimSuffix(ab.String(), "\n")})
	}
	var b strings.Builder
	for _, part := range blocks {
		if part.rendered != "" {
			b.WriteString(part.rendered)
			continue
		}
		if strings.TrimSpace(part.code) != "" {
			if part.unitType != "" {
				b.WriteString(renderNamedBlock(part.label, part.unitType, part.code))
			} else {
				b.WriteString(renderBlock(part.label, part.code))
			}
		}
	}
	return b.String()
}

func fileUnitType(kind twincat.Kind) string {
	switch kind {
	case twincat.KindDUT:
		return "TYPE"
	case twincat.KindGVL:
		return "GLOBAL_VARIABLE_LIST"
	default:
		return ""
	}
}

// renderPOU gives the POU its own unit heading and keeps each member's
// declaration and implementation visibly grouped beneath it.
func renderPOU(f *twincat.File) string {
	var b strings.Builder
	label := strings.TrimSpace(string(f.POUType) + " " + f.Name)
	fmt.Fprintf(&b, "<section class=\"tcv-block tcv-pou\" aria-label=\"%s\">\n", html.EscapeString(label))
	b.WriteString(`<div class="tcv-label tcv-unit-label">`)
	b.WriteString(renderUnitLabel(f.Name, string(f.POUType)))
	b.WriteString("</div>\n")
	if strings.TrimSpace(f.Declaration) != "" {
		b.WriteString(renderUnitPart("Declaration", f.Declaration, "tcv-pou-part"))
	}
	if strings.TrimSpace(f.Implementation) != "" {
		b.WriteString(renderUnitPart("Implementation", f.Implementation, "tcv-pou-part"))
	}
	for _, m := range f.Methods {
		if strings.TrimSpace(m.Declaration) == "" && strings.TrimSpace(m.Implementation) == "" {
			continue
		}
		if m.Type == twincat.POUTypeProperty {
			b.WriteString(renderProperty(m))
			continue
		}
		b.WriteString(renderMember(m))
	}
	b.WriteString("</section>\n")
	return b.String()
}

func renderMember(m twincat.Method) string {
	var b strings.Builder
	label := string(m.Type) + " " + m.Name
	fmt.Fprintf(&b, "<details class=\"tcv-block tcv-member\" open aria-label=\"%s\">\n", html.EscapeString(label))
	b.WriteString(`<summary class="tcv-label tcv-unit-label">`)
	b.WriteString(`<span class="tcv-disclosure" aria-hidden="true">&#9656;</span>`)
	b.WriteString(renderUnitLabel(m.Name, string(m.Type)))
	b.WriteString("</summary>\n")
	if strings.TrimSpace(m.Declaration) != "" {
		b.WriteString(renderUnitPart("Declaration", m.Declaration, "tcv-member-part"))
	}
	if strings.TrimSpace(m.Implementation) != "" {
		b.WriteString(renderUnitPart("Implementation", m.Implementation, "tcv-member-part"))
	}
	b.WriteString("</details>\n")
	return b.String()
}

func renderUnitLabel(name, typ string) string {
	return `<span class="tcv-unit-name">` + html.EscapeString(name) +
		`</span><span class="tcv-unit-type">` + html.EscapeString(typ) + `</span>`
}

func renderUnitPart(label, code, className string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<div class=\"tcv-subsection %s\" aria-label=\"%s\">\n", className, html.EscapeString(label))
	b.WriteString(renderCodeBlock(code))
	b.WriteString("</div>\n")
	return b.String()
}

// renderProperty keeps a property's declaration and implementation as two
// independently numbered listings beneath one compact property label. A
// property is not a method with two source fragments: keeping these listings
// separate mirrors the way TwinCAT presents the property in the POU.
func renderProperty(m twincat.Method) string {
	var b strings.Builder
	label := string(m.Type) + " " + m.Name
	fmt.Fprintf(&b, "<details class=\"tcv-block tcv-property\" open aria-label=\"%s\">\n", html.EscapeString(label))
	b.WriteString(`<summary class="tcv-label tcv-unit-label">`)
	b.WriteString(`<span class="tcv-disclosure" aria-hidden="true">&#9656;</span>`)
	b.WriteString(renderUnitLabel(m.Name, string(m.Type)))
	b.WriteString("</summary>\n")
	if strings.TrimSpace(m.Declaration) != "" {
		b.WriteString(renderPropertyPart("Declaration", m.Declaration))
	}
	if strings.TrimSpace(m.Implementation) != "" {
		b.WriteString(renderPropertyPart("Implementation", m.Implementation))
	}
	b.WriteString("</details>\n")
	return b.String()
}

func renderPropertyPart(label, code string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<div class=\"tcv-subsection tcv-property-part\" aria-label=\"%s\">\n", html.EscapeString(label))
	b.WriteString(renderCodeBlock(code))
	b.WriteString("</div>\n")
	return b.String()
}

func renderBlock(label, code string) string {
	return renderNamedBlock(label, "", code)
}

func renderNamedBlock(name, typ, code string) string {
	var b strings.Builder
	label := strings.TrimSpace(name + " " + typ)
	fmt.Fprintf(&b, "<section class=\"tcv-block\" aria-label=\"%s\">\n", html.EscapeString(label))
	if typ == "" {
		fmt.Fprintf(&b, "<div class=\"tcv-label tcv-section-label\">%s</div>\n", html.EscapeString(name))
	} else {
		b.WriteString(`<div class="tcv-label tcv-unit-label">`)
		b.WriteString(renderUnitLabel(name, typ))
		b.WriteString("</div>\n")
	}
	b.WriteString(renderCodeBlock(code))
	b.WriteString("</section>\n")
	return b.String()
}

// renderCodeBlock highlights src using the DOM shape emitted by Gitea's
// native source view. Each listing owns its line-number sequence; line
// numbers are data attributes rather than IDs so separate listings never
// create duplicate document IDs.
func renderCodeBlock(src string) string {
	highlighted := highlightST(src)
	lines := strings.Split(highlighted, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		lines = []string{""}
	}

	var b strings.Builder
	b.WriteString(`<div class="file-view code-view"><table><tbody>`)
	for i, line := range lines {
		fmt.Fprintf(&b, `<tr><td class="lines-num"><span data-line-number="%d"></span></td><td class="lines-code chroma"><code class="code-inner">`, i+1)
		b.WriteString(line)
		b.WriteString(`</code></td></tr>`)
	}
	b.WriteString("</tbody></table></div>\n")
	return b.String()
}

// css deliberately mirrors Gitea's code surface rather than creating a
// second application shell. Gitea exposes these variables on the page and
// changes them when its manual theme selector changes. Palette overrides are
// scoped to Gitea's explicit theme states so an OS dark preference cannot
// accidentally override a manually selected light theme.
const css = `.tcv {
  --tcv-fg: var(--color-text, #24292f);
  --tcv-muted: var(--color-text-light-2, #6a737d);
  --tcv-kw: #cf222e;
  --tcv-type: #953800;
  --tcv-lit: #0550ae;
  --tcv-str: #0a3069;
  --tcv-com: #6e7781;
  --tcv-fn: #8250df;
  --tcv-num: #0550ae;
  color: var(--tcv-fg);
  margin: 0;
  padding: 0;
}
/* Gitea 1.20+ sets data-theme; older Gitea/Forgejo use theme-dark classes. */
[data-theme=gitea-light] .tcv {
  --tcv-kw: #cf222e; --tcv-type: #953800; --tcv-lit: #0550ae;
  --tcv-str: #0a3069; --tcv-com: #6e7781; --tcv-fn: #8250df; --tcv-num: #0550ae;
}
[data-theme=gitea-dark] .tcv,
[data-theme=dark] .tcv, .theme-dark .tcv, .theme-darkness .tcv {
  --tcv-kw: #ff7b72; --tcv-type: #ffa657; --tcv-lit: #79c0ff;
  --tcv-str: #a5d6ff; --tcv-com: #8b949e; --tcv-fn: #d2a8ff; --tcv-num: #79c0ff;
}
/* Only Gitea's automatic mode follows the user's OS preference. */
@media (prefers-color-scheme: dark) {
  [data-theme=gitea-auto] .tcv { --tcv-kw: #ff7b72; --tcv-type: #ffa657; --tcv-lit: #79c0ff; --tcv-str: #a5d6ff; --tcv-com: #8b949e; --tcv-fn: #d2a8ff; --tcv-num: #79c0ff; }
}
.tcv-block { margin: 0; padding: 0; }
.tcv .tcv-member summary,
.tcv .tcv-property summary {
  cursor: pointer; list-style: none; list-style-position: outside;
}
.tcv .tcv-member summary::-webkit-details-marker,
.tcv .tcv-property summary::-webkit-details-marker { display: none; }
.tcv .tcv-disclosure {
  color: var(--tcv-muted); display: inline-block; margin-right: .35rem; user-select: none;
}
.tcv .tcv-member[open] .tcv-disclosure,
.tcv .tcv-property[open] .tcv-disclosure { transform: rotate(90deg); }
.tcv .tcv-block + .tcv-block {
  border-top: 1px solid var(--color-secondary, #d0d7de);
  margin-top: 8px; padding-top: 3px;
}
.tcv .tcv-pou .tcv-member,
.tcv .tcv-pou .tcv-property {
  border-top: 1px solid var(--color-secondary, #d0d7de);
  margin-top: 8px; padding-top: 3px;
}
.tcv .file-view.code-view { overflow-x: auto; }
/* External renderers are padded by the surrounding Markdown file view. */
.file-view.markup.twincat:has(.tcv) { padding: 0 !important; }
/* Gitea places renderer output in .markup, whose table defaults are prose-sized. */
.markup .tcv .file-view.code-view table {
  border-collapse: collapse; border-spacing: 0; margin: 0; padding: 0;
}
.markup .tcv .file-view.code-view tbody,
.markup .tcv .file-view.code-view tr {
  margin: 0; padding: 0;
}
.markup .tcv .file-view.code-view tr {
  border: 0 !important; height: 20px; line-height: 20px;
}
.markup .tcv .file-view.code-view td {
  border: 0 !important; font-size: 12px; line-height: 20px; margin: 0;
}
.markup .tcv .file-view.code-view td.lines-num { padding: 0 8px !important; }
.markup .tcv .file-view.code-view td.lines-code { padding: 0 1px 0 5px !important; }
.tcv .file-view.code-view td.lines-num { user-select: none; }
.markup .tcv .file-view.code-view code.code-inner {
  background: transparent !important; border: 0 !important; border-radius: 0 !important;
  font-family: var(--font-family-monospace, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
  font-size: inherit; line-height: inherit; padding: 0 !important; white-space: pre;
}
.markup .tcv .file-view.code-view tr:nth-child(even),
.markup .tcv .file-view.code-view tr:nth-child(even) td {
  background: transparent !important;
}
.markup .tcv .file-view.code-view tr:hover,
.markup .tcv .file-view.code-view tr:hover td {
  background: transparent !important;
}
.tcv .tcv-label {
  color: var(--tcv-muted); font-size: 11px; font-weight: 600;
  line-height: 1.25; margin: 0; padding: 4px 12px 2px; user-select: none;
}
.tcv .tcv-section-label {
  color: var(--tcv-muted); font-size: 12px; font-weight: 600;
  letter-spacing: .01em; padding-top: 4px; padding-bottom: 3px;
}
.tcv .tcv-unit-label {
  letter-spacing: .01em; padding-top: 5px; padding-bottom: 4px;
}
.tcv .tcv-unit-name { color: var(--tcv-fg); font-size: 12px; font-weight: 700; }
.tcv .tcv-unit-type {
  color: var(--tcv-muted); font-size: 11px; font-weight: 400; margin-left: .6rem;
}
.tcv-property { margin-top: 7px; }
.tcv .tcv-subsection { margin-left: 1rem; }
.tcv .tcv-subsection + .tcv-subsection {
  border-top: 1px solid var(--color-secondary, #d0d7de);
  margin-top: 4px; padding-top: 4px;
}
`
