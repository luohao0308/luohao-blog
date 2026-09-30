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
