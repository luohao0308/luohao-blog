package biz

import (
	"context"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// chatRecordingIndex implements ArticleSearchIndex, recording search calls so
// tests can assert the retrieval parameters the chat usecase passes through.
type chatRecordingIndex struct {
	query  string
	limit  int
	offset int
	slugs  []string
}

func (c *chatRecordingIndex) IndexArticle(context.Context, *Article) error { return nil }
func (c *chatRecordingIndex) RemoveArticle(context.Context, string) error  { return nil }
func (c *chatRecordingIndex) RecreateIndex(context.Context) error          { return nil }
func (c *chatRecordingIndex) Search(_ context.Context, query string, limit, offset int) ([]string, error) {
	c.query, c.limit, c.offset = query, limit, offset
	return c.slugs, nil
}

// chatFakeLLM records prompts and returns a canned answer or error.
type chatFakeLLM struct {
	system string
	user   string
	calls  int
	answer string
	err    error
}

func (f *chatFakeLLM) Generate(_ context.Context, system, user string) (string, error) {
	f.calls++
	f.system, f.user = system, user
	return f.answer, f.err
}

// chatFakeLimiter records keys and returns a canned decision.
type chatFakeLimiter struct {
	key   string
	allow bool
	err   error
}

func (f *chatFakeLimiter) Allow(_ context.Context, key string) (bool, error) {
	f.key = key
	return f.allow, f.err
}

func newChatTestUsecase(t *testing.T, index ArticleSearchIndex, llm ChatLLM, limiter ChatRateLimiter) (*ChatUsecase, *fakeArticleRepo) {
	t.Helper()
	repo := newFakeArticleRepo()
	for slug, status := range map[string]ArticleStatus{
		"pub-a": ArticleStatusPublished,
		"pub-b": ArticleStatusPublished,
		"draft": ArticleStatusDraft,
	} {
		a := &Article{Slug: slug, Title: "标题" + slug, ContentMD: "内容" + slug, Status: status}
		if _, err := repo.CreateArticle(context.Background(), a); err != nil {
			t.Fatalf("seed %s: %v", slug, err)
		}
	}
	return NewChatUsecase(NewArticleUsecase(repo, index), llm, limiter), repo
}

func TestChatAskDisabled(t *testing.T) {
	uc, _ := newChatTestUsecase(t, &chatRecordingIndex{}, nil, nil)
	if _, err := uc.Ask(context.Background(), "问题", 0, "ip"); !kratoserrors.IsServiceUnavailable(err) {
		t.Fatalf("Ask() without llm error = %v, want unavailable", err)
	}
}

func TestChatAskValidation(t *testing.T) {
	uc, _ := newChatTestUsecase(t, &chatRecordingIndex{}, &chatFakeLLM{answer: "a"}, nil)
	if _, err := uc.Ask(context.Background(), "", 0, "ip"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("empty query error = %v, want bad request", err)
	}
	if _, err := uc.Ask(context.Background(), strings.Repeat("长", ChatQueryMaxRunes+1), 0, "ip"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("overlong query error = %v, want bad request", err)
	}
}

func TestChatAskRateLimited(t *testing.T) {
	limiter := &chatFakeLimiter{allow: false}
	uc, _ := newChatTestUsecase(t, &chatRecordingIndex{}, &chatFakeLLM{answer: "a"}, limiter)
	if _, err := uc.Ask(context.Background(), "问题", 0, "1.2.3.4"); !kratoserrors.IsTooManyRequests(err) {
		t.Fatalf("throttled Ask error = %v, want too many requests", err)
	}
	if limiter.key != "chat:1.2.3.4" {
		t.Fatalf("limiter key = %q, want chat-namespaced ip", limiter.key)
	}

	// A limiter infrastructure error fails open: the question goes through.
	uc, _ = newChatTestUsecase(t, &chatRecordingIndex{slugs: []string{"pub-a"}}, &chatFakeLLM{answer: "a"}, &chatFakeLimiter{err: context.DeadlineExceeded})
	answer, err := uc.Ask(context.Background(), "问题", 0, "ip")
	if err != nil || answer.Answer != "a" {
		t.Fatalf("limiter failure Ask() = (%q, %v), want fail-open answer", answer.Answer, err)
	}
}

func TestChatAskNoResults(t *testing.T) {
	index := &chatRecordingIndex{slugs: nil}
	llm := &chatFakeLLM{answer: "不应被调用"}
	uc, _ := newChatTestUsecase(t, index, llm, nil)
	answer, err := uc.Ask(context.Background(), "冷门问题", 0, "ip")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if llm.calls != 0 {
		t.Fatalf("llm called %d times with no retrieval hits, want 0", llm.calls)
	}
	if answer.Answer != noResultAnswer || len(answer.Citations) != 0 {
		t.Fatalf("no-result answer = %+v", answer)
	}
}

func TestChatAskHappyPath(t *testing.T) {
	index := &chatRecordingIndex{slugs: []string{"pub-b", "pub-a", "draft"}}
	llm := &chatFakeLLM{answer: "生成的回答\n来源：标题pub-b"}
	uc, _ := newChatTestUsecase(t, index, llm, nil)
	answer, err := uc.Ask(context.Background(), "怎么建模", 99, "ip")
	if err != nil {
		t.Fatalf("Ask() error = %v", err)
	}
	if index.limit != 8 || index.query != "怎么建模" || index.offset != 0 {
		t.Fatalf("search args = (%q, %d, %d), want topK clamped to 8", index.query, index.limit, index.offset)
	}
	if answer.Answer != llm.answer {
		t.Fatalf("answer = %q, want the llm output", answer.Answer)
	}
	// Drafts never reach citations even if the index returned one.
	if len(answer.Citations) != 2 || answer.Citations[0].Slug != "pub-b" || answer.Citations[1].Slug != "pub-a" {
		t.Fatalf("citations = %+v, want published slugs in search order", answer.Citations)
	}
	// The prompt grounds the model: system rules, both article titles and
	// contents, the question, and the citation instruction.
	if !strings.Contains(llm.system, "不要编造") || !strings.Contains(llm.system, "纯文本") {
		t.Fatalf("system prompt missing grounding rules: %q", llm.system)
	}
	for _, want := range []string{"标题pub-a", "内容pub-a", "标题pub-b", "内容pub-b", "怎么建模", "来源：文章标题"} {
		if !strings.Contains(llm.user, want) {
			t.Fatalf("user prompt missing %q", want)
		}
	}
}

func TestChatAskLLMFailureKeepsCitations(t *testing.T) {
	index := &chatRecordingIndex{slugs: []string{"pub-a"}}
	llm := &chatFakeLLM{err: context.DeadlineExceeded}
	uc, _ := newChatTestUsecase(t, index, llm, nil)
	answer, err := uc.Ask(context.Background(), "问题", 0, "ip")
	if err != nil {
		t.Fatalf("Ask() error = %v, want degraded answer", err)
	}
	if answer.Answer != llmFailureAnswer {
		t.Fatalf("degraded answer = %q", answer.Answer)
	}
	if len(answer.Citations) != 1 || answer.Citations[0].Slug != "pub-a" {
		t.Fatalf("degraded citations = %+v", answer.Citations)
	}
}
