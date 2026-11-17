package service

import (
	"regexp"
	"strings"
	"unicode"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

var (
	synopsisCode    = regexp.MustCompile("(?s)```.*?```")
	synopsisInline  = regexp.MustCompile("`[^`]*`")
	synopsisImages  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	synopsisLinks   = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	synopsisMarkers = regexp.MustCompile(`(?m)^[#>*+-]+\s*`)
	synopsisURLs    = regexp.MustCompile(`https?://\S+`)
)

func applyArticleSummary(article *v1.Article) {
	body := article.ContentMd
	article.ContentMd = ""
	article.ContentHtml = ""
	prose := synopsisCode.ReplaceAllString(body, " ")
	prose = synopsisImages.ReplaceAllString(prose, " ")
	if strings.TrimSpace(article.Summary) == "" {
		plain := synopsisLinks.ReplaceAllString(prose, "$1")
		plain = synopsisMarkers.ReplaceAllString(plain, "")
		plain = strings.NewReplacer("`", "", "_", "", "*", "", "~", "").Replace(plain)
		runes := []rune(strings.Join(strings.Fields(plain), " "))
		if len(runes) > 160 {
			article.Summary = strings.TrimSpace(string(runes[:160])) + "…"
		} else {
			article.Summary = string(runes)
		}
	}

	if strings.TrimSpace(body) == "" {
		return
	}
	prose = synopsisInline.ReplaceAllString(prose, " ")
	prose = synopsisLinks.ReplaceAllString(prose, " ")
	prose = synopsisURLs.ReplaceAllString(prose, " ")
	han, words := 0, 0
	inWord := false
	for _, r := range prose {
		switch {
		case unicode.Is(unicode.Han, r):
			han++
			inWord = false
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if !inWord {
				words++
			}
			inWord = true
		default:
			inWord = false
		}
	}
	minutes := uint32(max(1, (han+2*words+399)/400))
	article.ReadingMinutes = &minutes
}
