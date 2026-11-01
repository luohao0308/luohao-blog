package render

import (
	"strings"
	"testing"
)

func TestSectionsMatchRenderedAnchors(t *testing.T) {
	source := "# 中文标题\n\n## 恢复\n\n第一段\n\n## 恢复\n\n第二段\n\n```go\nconst x = 1\n```"
	sections := Sections(source)
	html, err := HTML(source)
	if err != nil || len(sections) != 3 {
		t.Fatal(sections, err)
	}
	for _, s := range sections[1:] {
		if s.Anchor == "" || !strings.Contains(html, `id="`+s.Anchor+`"`) {
			t.Fatalf("anchor %+v not in %s", s, html)
		}
	}
	if sections[1].Anchor == sections[2].Anchor || !strings.Contains(sections[2].Content, "const x = 1") {
		t.Fatal(sections)
	}
}
