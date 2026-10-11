package biz

import (
	"regexp"
	"strings"
	"unicode"
)

// CommentModerationPolicy uses local spam signals, not semantic safety scoring.
// Empty mode defaults to smart. Unknown modes retain manual review.
type CommentModerationPolicy struct {
	Mode           string
	ReviewKeywords []string
}

var commentContactPattern = regexp.MustCompile(`(?i)(https?://|www\.|[a-z0-9][a-z0-9._%+\-]*@[a-z0-9.\-]+\.[a-z]{2,}|\b1[3-9][0-9]{9}\b|[a-z0-9][a-z0-9\-]*\.(com|cn|net|org|io|dev|app|xyz|top|info|site|blog|cloud|tech|vip|me|cc|edu|gov)\b)`)
var commentReviewKeywords = []string{"加微信", "加vx", "加v信", "兼职刷单", "博彩", "裸聊", "代开发票", "贷款秒批"}

func (p CommentModerationPolicy) status(name, content string) CommentStatus {
	mode := strings.ToLower(strings.TrimSpace(p.Mode))
	if mode != "" && mode != "smart" {
		return CommentStatusPending
	}
	text := normalizeCommentRiskText(name + "\n" + content)
	if commentContactPattern.MatchString(text) {
		return CommentStatusPending
	}
	for _, keywords := range [][]string{commentReviewKeywords, p.ReviewKeywords} {
		for _, keyword := range keywords {
			needle := normalizeCommentRiskText(keyword)
			if needle != "" && strings.Contains(text, needle) {
				return CommentStatusPending
			}
		}
	}
	var previous rune
	repeated := 0
	for _, r := range normalizeCommentRiskText(content) {
		if r == previous {
			repeated++
		} else {
			previous = r
			repeated = 1
		}
		if repeated >= 20 {
			return CommentStatusPending
		}
	}
	return CommentStatusApproved
}

// Normalize only the inspection copy; preserve the submitted text for readers.
func normalizeCommentRiskText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		if r >= '\uff01' && r <= '\uff5e' {
			r -= 0xfee0
		}
		return unicode.ToLower(r)
	}, s)
}
