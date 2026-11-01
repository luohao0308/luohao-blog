package render

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"strings"
)

// Section refers to an actual heading ID produced by the article renderer.
// Parse the complete document so repeated headings keep their original IDs.
type Section struct{ Title, Anchor, Content string }

func Sections(source string) []Section {
	data := []byte(source)
	doc := md.Parser().Parse(text.NewReader(data))
	out := []Section{{Title: "正文"}}
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*ast.Heading); ok && (heading.Level == 2 || heading.Level == 3) {
			anchor := ""
			if id, ok := heading.AttributeString("id"); ok {
				if value, ok := id.([]byte); ok {
					anchor = string(value)
				}
			}
			out = append(out, Section{Title: sectionText(node, data), Anchor: anchor})
			continue
		}
		out[len(out)-1].Content += sectionText(node, data) + "\n"
	}
	result := make([]Section, 0, len(out))
	for _, section := range out {
		section.Content = strings.TrimSpace(section.Content)
		if section.Content != "" {
			result = append(result, section)
		}
	}
	return result
}
func sectionText(node ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(source))
			if v.SoftLineBreak() || v.HardLineBreak() {
				b.WriteByte('\n')
			}
		case *ast.String:
			b.Write(v.Value)
		case *ast.FencedCodeBlock:
			for i := 0; i < v.Lines().Len(); i++ {
				line := v.Lines().At(i)
				b.Write(line.Value(source))
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			for i := 0; i < v.Lines().Len(); i++ {
				line := v.Lines().At(i)
				b.Write(line.Value(source))
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}
