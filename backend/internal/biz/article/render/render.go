// Package render turns article Markdown into the HTML that the frontend
// displays. Rendering is a domain concern of the article model: content_html
// is a write-time cache produced here and read by the API layer verbatim.
package render

import (
	"bytes"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// md is the shared goldmark instance. Options are fixed so that every article
// renders with the same feature set:
//   - GFM: tables, strikethrough, task lists, autolinks;
//   - auto heading IDs for anchor links;
//   - hard wraps: single newlines become <br>, which matches how Chinese
//     prose is usually written in Markdown editors;
//   - raw HTML in the source is kept (Unsafe): articles are authored by the
//     site owner; the assumption is revisited when M2 adds authentication;
//   - code blocks highlight through chroma with CSS classes (no inline
//     styles), so the frontend owns the theme.
var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithLineNumbers(false),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		html.WithHardWraps(),
		html.WithUnsafe(),
	),
)

// HTML renders Markdown source into the HTML fragment stored in
// content_html. The result is a fragment (no <html> wrapper) and is safe to
// embed in an SSR page as-is.
func HTML(source string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
