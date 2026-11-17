package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
)

// chatRecordingIndex implements ArticleSearchIndex, recording search calls so
// tests can assert the retrieval parameters the chat usecase passes through.
type chatRecordingIndex struct {
	query  string
	limit  int
	offset int
	slugs  []string
	err    error
}

func (c *chatRecordingIndex) IndexArticle(context.Context, *Article) error { return nil }
func (c *chatRecordingIndex) RemoveArticle(context.Context, string) error  { return nil }
func (c *chatRecordingIndex) RecreateIndex(context.Context) error          { return nil }
func (c *chatRecordingIndex) Search(_ context.Context, query string, limit, offset int) ([]string, error) {
	c.query, c.limit, c.offset = query, limit, offset
	return c.slugs, c.err
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

	// Hybrid retrieval can call a paid embedding endpoint. Skip retrieval and
	// generation when the limiter cannot verify the budget.
	index := &chatRecordingIndex{slugs: []string{"pub-a"}}
	llm := &chatFakeLLM{answer: "a"}
	uc, _ = newChatTestUsecase(t, index, llm, &chatFakeLimiter{err: context.DeadlineExceeded})
	answer, err := uc.Ask(context.Background(), "问题", 0, "ip")
	if err != nil || answer == nil || answer.Outcome != ChatOutcomeGenerationUnavailable || index.query != "" || llm.calls != 0 {
		t.Fatalf("limiter failure Ask() = (%+v, %v), search=%q model calls=%d", answer, err, index.query, llm.calls)
	}
	answer, err = uc.Ask(context.Background(), "你是谁", 0, "ip")
	if err != nil || answer == nil || answer.Outcome != ChatOutcomeAnswered || index.query != "" || llm.calls != 0 {
		t.Fatalf("free introduction unavailable during limiter failure: (%+v, %v)", answer, err)
	}
}

type blockingChatLLM struct {
	started chan struct{}
}

func (llm blockingChatLLM) Generate(ctx context.Context, _, _ string) (string, error) {
	llm.started <- struct{}{}
	<-ctx.Done()
	return "", ctx.Err()
}

func TestChatProviderLimitSharedAcrossScopes(t *testing.T) {
	const slots = 4
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	blocking := blockingChatLLM{started: make(chan struct{}, slots)}
	articleChat, _ := newChatTestUsecase(t, nil, blocking, nil)
	index := &chatRecordingIndex{slugs: []string{"pub-a"}}
	llm := &chatFakeLLM{answer: "答案"}
	siteChat, _ := newChatTestUsecase(t, index, llm, nil)
	finished := make(chan error, slots)
	for range slots {
		go func() {
			_, err := articleChat.AskArticle(ctx, "总结本文", "pub-a", "ip")
			finished <- err
		}()
	}
	for range slots {
		select {
		case <-blocking.started:
		case <-ctx.Done():
			t.Fatal("four requests did not reach generation")
		}
	}
	answer, err := siteChat.Ask(ctx, "问题", 0, "ip")
	if err != nil || answer == nil || answer.Outcome != ChatOutcomeGenerationUnavailable || index.query != "" || llm.calls != 0 {
		t.Errorf("site-wide saturation = (%+v, %v), search=%q model calls=%d", answer, err, index.query, llm.calls)
	}
	answer, err = siteChat.AskArticle(ctx, "总结本文", "pub-a", "ip")
	if err != nil || answer == nil || answer.Outcome != ChatOutcomeGenerationUnavailable || len(answer.Sections) == 0 || llm.calls != 0 {
		t.Errorf("article saturation = (%+v, %v), model calls=%d", answer, err, llm.calls)
	}
	cancel()
	for range slots {
		if err := <-finished; err != nil {
			t.Errorf("in-flight article chat: %v", err)
		}
	}
	answer, err = siteChat.Ask(context.Background(), "问题", 0, "ip")
	if err != nil || answer == nil || answer.Outcome != ChatOutcomeAnswered || llm.calls != 1 {
		t.Errorf("generation slot was not released after cancellation: (%+v, %v)", answer, err)
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

func TestChatOutcomes(t *testing.T) {
	for _, tt := range []struct {
		name  string
		index ArticleSearchIndex
		llm   *chatFakeLLM
		want  ChatOutcome
	}{
		{"answered", &chatRecordingIndex{slugs: []string{"pub-a"}}, &chatFakeLLM{answer: "answer"}, ChatOutcomeAnswered},
		{"no match", &chatRecordingIndex{}, &chatFakeLLM{}, ChatOutcomeNoMatch},
		{"missing index", nil, &chatFakeLLM{}, ChatOutcomeRetrievalUnavailable},
		{"failed index", &chatRecordingIndex{err: context.DeadlineExceeded}, &chatFakeLLM{}, ChatOutcomeRetrievalUnavailable},
		{"generation failed", &chatRecordingIndex{slugs: []string{"pub-a"}}, &chatFakeLLM{err: context.DeadlineExceeded}, ChatOutcomeGenerationUnavailable},
		{"empty generation", &chatRecordingIndex{slugs: []string{"pub-a"}}, &chatFakeLLM{answer: "  "}, ChatOutcomeGenerationUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			uc, _ := newChatTestUsecase(t, tt.index, tt.llm, nil)
			answer, err := uc.Ask(context.Background(), "question", 4, "ip")
			if err != nil || answer.Outcome != tt.want {
				t.Fatalf("got %+v, %v; want %v", answer, err, tt.want)
			}
			if tt.want == ChatOutcomeRetrievalUnavailable && tt.llm.calls != 0 {
				t.Fatal("generation must not run without retrieval")
			}
		})
	}
}

type failingHydrationRepo struct{ ArticleRepo }

func (f failingHydrationRepo) FindBySlug(context.Context, string) (*Article, error) {
	return nil, context.DeadlineExceeded
}
func TestSearchAvailabilityAndLegacyWrapper(t *testing.T) {
	uc := NewArticleUsecase(failingHydrationRepo{}, &chatRecordingIndex{slugs: []string{"pub-a"}})
	result := uc.SearchArticlesWithOutcome(context.Background(), "q", 4, 0)
	if result.Available || len(result.Articles) != 0 {
		t.Fatalf("hydration outage: %+v", result)
	}
	legacy, err := uc.SearchArticles(context.Background(), "q", 4, 0)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("legacy degraded page: %+v, %v", legacy, err)
	}
	uc = NewArticleUsecase(newFakeArticleRepo(), &chatRecordingIndex{slugs: []string{"deleted"}})
	result = uc.SearchArticlesWithOutcome(context.Background(), "q", 4, 0)
	if !result.Available || len(result.Articles) != 0 {
		t.Fatalf("stale index should remain available: %+v", result)
	}
}

func TestChatIntroductionDoesNotRetrieveOrCite(t *testing.T) {
	ctx := context.Background()
	index := &chatRecordingIndex{slugs: []string{"pub-a"}}
	llm := &chatFakeLLM{answer: "文章回答"}
	uc, _ := newChatTestUsecase(t, index, llm, nil)
	for _, slug := range []string{"", "pub-a"} {
		var answer *ChatAnswer
		var err error
		if slug == "" {
			answer, err = uc.Ask(ctx, " 你是谁？ ", 0, "ip")
		} else {
			answer, err = uc.AskArticle(ctx, " 你是谁？ ", slug, "ip")
		}
		if err != nil || answer == nil || answer.Outcome != ChatOutcomeAnswered || answer.ArticleSlug != slug ||
			!strings.Contains(answer.Answer, "我是小乌") || len(answer.Citations) != 0 || len(answer.Sections) != 0 {
			t.Fatalf("scope %q: answer=%+v error=%v", slug, answer, err)
		}
	}
	if index.query != "" || llm.calls != 0 {
		t.Fatalf("introduction queried the index (%q) or model (%d calls)", index.query, llm.calls)
	}
	if _, err := uc.AskArticle(ctx, "你是谁", "draft", "ip"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("introduction must still validate article visibility: %v", err)
	}
	answer, err := uc.Ask(ctx, "文章里提到的作者是谁？", 0, "ip")
	if err != nil || answer == nil || len(answer.Citations) != 1 || llm.calls != 1 {
		t.Fatalf("article question should still retrieve sources: answer=%+v error=%v", answer, err)
	}
}
