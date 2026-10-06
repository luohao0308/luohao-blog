package render

import (
	"strings"
	"testing"
)

func TestHTMLHeadingsAndParagraph(t *testing.T) {
	out, err := HTML("# 标题一\n\n这是一段正文。")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if !strings.Contains(out, "<h1") || !strings.Contains(out, "标题一") {
		t.Fatalf("HTML() = %q, want h1 with heading text", out)
	}
	if !strings.Contains(out, "<p>这是一段正文。</p>") {
		t.Fatalf("HTML() = %q, want paragraph", out)
	}
}

func TestHTMLCodeBlockGetsChromaClasses(t *testing.T) {
	out, err := HTML("```go\nfunc main() {}\n```\n")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if !strings.Contains(out, `class="chroma"`) {
		t.Fatalf("HTML() = %q, want chroma highlight wrapper", out)
	}
	// WithClasses(true) means no inline styles: theming is the frontend's job.
	if strings.Contains(out, "style=") {
		t.Fatalf("HTML() = %q, want class-based highlighting without inline styles", out)
	}
	if !strings.Contains(out, "<span") {
		t.Fatalf("HTML() = %q, want highlighted tokens", out)
	}
}

func TestHTMLGFMFeatures(t *testing.T) {
	out, err := HTML("| a | b |\n|---|---|\n| 1 | 2 |\n\n~~删除~~\n")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if !strings.Contains(out, "<table>") {
		t.Fatalf("HTML() = %q, want GFM table", out)
	}
	if !strings.Contains(out, "<del>删除</del>") {
		t.Fatalf("HTML() = %q, want strikethrough", out)
	}
}

func TestHTMLHardWraps(t *testing.T) {
	out, err := HTML("第一行\n第二行")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if !strings.Contains(out, "<br>") {
		t.Fatalf("HTML() = %q, want hard-wrapped line break", out)
	}
}

func TestHTMLAutoHeadingID(t *testing.T) {
	out, err := HTML("## 安装\n\n内容")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if !strings.Contains(out, "id=") {
		t.Fatalf("HTML() = %q, want auto heading id", out)
	}
}

func TestHTMLEmptySource(t *testing.T) {
	out, err := HTML("")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("HTML(empty) = %q, want empty fragment", out)
	}
}

// Raw HTML in the Markdown source must not pass through: content_html is
// embedded as-is by the frontend (v-html), so the renderer is the XSS
// boundary for everything an author (or a compromised author account) can
// put into an article. Goldmark's safe default omits raw HTML entirely.
func TestHTMLRawHTMLOmitted(t *testing.T) {
	out, err := HTML("前文\n\n<script>alert(1)</script>\n\n<img src=x onerror=alert(1)>\n\n<a href=\"javascript:alert(1)\">链接</a>\n")
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	for _, danger := range []string{"<script>", "<img", "<a href=\"javascript:"} {
		if strings.Contains(out, danger) {
			t.Fatalf("HTML() = %q, want raw HTML tag %q omitted", out, danger)
		}
	}
	if !strings.Contains(out, "raw HTML omitted") {
		t.Fatalf("HTML() = %q, want goldmark's raw-HTML omission comment", out)
	}
	if !strings.Contains(out, "链接") {
		t.Fatalf("HTML() = %q, want raw-HTML anchor text to survive as text", out)
	}
}

// Markdown's own link/image syntax must not smuggle executable schemes past
// the renderer: unsafe destinations are unwrapped to their text, while
// allowlisted schemes and relative URLs render normally.
func TestHTMLUnsafeDestinationsUnwrapped(t *testing.T) {
	out, err := HTML(
		"[点我](javascript:alert(1))\n\n"+
			"[数据](data:text/html;base64,AAAA)\n\n"+
			"[正常](https://example.com)\n\n"+
			"[相对](/posts/other)\n\n"+
			"[锚点](#section)\n\n"+
			"![图](javascript:alert(2))\n",
	)
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	if strings.Contains(out, "javascript:") || strings.Contains(out, "data:text") {
		t.Fatalf("HTML() = %q, want unsafe destinations stripped", out)
	}
	if strings.Contains(out, "<img") {
		t.Fatalf("HTML() = %q, want unsafe image unwrapped", out)
	}
	// Unwrapping keeps the link text as content.
	if !strings.Contains(out, "点我") || !strings.Contains(out, "数据") || !strings.Contains(out, "图") {
		t.Fatalf("HTML() = %q, want unwrapped link text to survive", out)
	}
	for _, want := range []string{`href="https://example.com"`, `href="/posts/other"`, `href="#section"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("HTML() = %q, want %q to render", out, want)
		}
	}
}
