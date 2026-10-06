// sanitize.go post-processes the parsed Markdown AST before rendering: the
// raw-HTML omission (goldmark's default) already removes pasted HTML, but
// Markdown's own link and image syntax still emits arbitrary destination
// schemes — [x](javascript:...) would render a clickable executable href.
// Destinations are restricted to an allowlist of schemes (plus relative and
// fragment URLs); anything else is unwrapped to its plain text content.
package render

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// safeLinkSchemes are the destination schemes that survive rendering.
// Autolinks (<https://…>) are already parser-limited to these; the check
// exists for author-written [text](destination) and image syntax.
var safeLinkSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"mailto": true,
}

type safeLinkTransformer struct{}

func (t *safeLinkTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	// Collect first, mutate after: unwrapping nodes while Walk is inside
	// their subtree would confuse the traversal.
	var unsafe []ast.Node
	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch ln := n.(type) {
		case *ast.Link:
			if !safeDestination(ln.Destination) {
				unsafe = append(unsafe, ln)
			}
		case *ast.Image:
			if !safeDestination(ln.Destination) {
				unsafe = append(unsafe, ln)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, n := range unsafe {
		unwrapNode(n)
	}
}

// safeDestination reports whether a link/image destination may be rendered.
// A destination without a scheme (relative path, anchor, query) is allowed;
// with a scheme, only the allowlist passes. A colon that is not part of a
// well-formed scheme does not make the URL unsafe.
func safeDestination(dest []byte) bool {
	i := bytes.IndexByte(dest, ':')
	if i < 0 {
		return true
	}
	scheme := dest[:i]
	for _, c := range scheme {
		if !util.IsAlphaNumeric(c) && c != '+' && c != '-' && c != '.' {
			return true // not a scheme (e.g. a relative path containing ':')
		}
	}
	return safeLinkSchemes[strings.ToLower(string(scheme))]
}

// unwrapNode replaces n with its children (keeping the text) and removes n.
func unwrapNode(n ast.Node) {
	parent := n.Parent()
	if parent == nil {
		return
	}
	for c := n.FirstChild(); c != nil; {
		next := c.NextSibling()
		parent.InsertBefore(parent, n, c)
		c = next
	}
	parent.RemoveChild(parent, n)
}
