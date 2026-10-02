package biz

import (
	"context"
	"strings"
	"time"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
)

// Chat domain errors.
var (
	ErrChatUnavailable = errors.ServiceUnavailable(v1.ErrorReason_CHAT_UNAVAILABLE.String(), "chat is not configured")
	ErrChatInvalidArgument = errors.BadRequest(v1.ErrorReason_CHAT_INVALID_ARGUMENT.String(), "invalid chat argument")
	// ErrChatTooManyAttempts guards the LLM budget: chat is public but every
	// call costs tokens on a paid endpoint.
	ErrChatTooManyAttempts = errors.TooManyRequests(v1.ErrorReason_CHAT_TOO_MANY_ATTEMPTS.String(), "too many chat requests, try again later")
	// ChatQueryMaxRunes bounds one question; retrieval context, not the
	// question length, drives answer quality.
	ChatQueryMaxRunes = 500
	// DefaultChatAttempts/DefaultChatWindow bound public questions per client
	// IP when the config leaves them unset.
	DefaultChatAttempts int64 = 10
	DefaultChatWindow         = 5 * time.Minute
)

// ChatRateLimiter throttles public chat questions per client.
type ChatRateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// ChatLLM generates an answer from a system instruction and one user turn.
// Implementations are OpenAI-compatible chat endpoints; nil disables the
// generation side and the Chat RPC reports unavailable.
type ChatLLM interface {
	Generate(ctx context.Context, system, user string) (string, error)
}

// ChatAnswer is a retrieval-augmented reply: the generated text plus the
// published articles it drew from, best-match first.
type ChatAnswer struct {
	Answer    string
	Citations []*Article
}

// ChatUsecase answers visitor questions over the published articles:
// hybrid retrieval feeds an LLM prompt, the reply carries citations.
type ChatUsecase struct {
	search  *ArticleUsecase
	llm     ChatLLM
	limiter ChatRateLimiter
}

// NewChatUsecase builds the chat usecase. llm nil disables generation; the
// limiter is optional and fails open on infrastructure errors.
func NewChatUsecase(search *ArticleUsecase, llm ChatLLM, limiter ChatRateLimiter) *ChatUsecase {
	return &ChatUsecase{search: search, llm: llm, limiter: limiter}
}

// noResultAnswer is returned without an LLM call when retrieval finds
// nothing: the acceptance contract requires saying so explicitly, and no
// generation can ground an answer without sources.
const noResultAnswer = "没有找到与你的问题相关的文章。这个问答基于博客已发布的文章内容回答，换个说法再试试，或者先浏览文章列表。"

// llmFailureAnswer keeps the chat window useful when generation fails: the
// retrieval already succeeded, so the citations are still shown.
const llmFailureAnswer = "AI 生成暂时不可用，先看看这几篇相关文章："

// Ask answers one question. Flow: validate → throttle → retrieve → generate.
// The answer is plain text; citations list the retrieved articles in
// best-match order regardless of which ones the answer text mentions.
func (uc *ChatUsecase) Ask(ctx context.Context, query string, topK int, clientIP string) (*ChatAnswer, error) {
	if uc.llm == nil {
		return nil, ErrChatUnavailable
	}
	runes := []rune(query)
	if len(runes) == 0 || len(runes) > ChatQueryMaxRunes {
		return nil, ErrChatInvalidArgument
	}
	if topK <= 0 {
		topK = 4
	}
	if topK > 8 {
		topK = 8
	}
	if uc.limiter != nil {
		ok, err := uc.limiter.Allow(ctx, "chat:"+clientIP)
		if err == nil && !ok {
			return nil, ErrChatTooManyAttempts
		}
		// limiter failure fails open: an outage in the counter must not take
		// the chat down, mirroring the login and comment throttlers.
	}
	articles, err := uc.search.SearchArticles(ctx, query, topK, 0)
	if err != nil {
		return nil, err
	}
	if len(articles) == 0 {
		return &ChatAnswer{Answer: noResultAnswer}, nil
	}
	answer, err := uc.llm.Generate(ctx, chatSystemPrompt(), chatUserPrompt(query, articles))
	if err != nil {
		log.Warn("chat: llm generation failed, answering with citations only", err)
		return &ChatAnswer{Answer: llmFailureAnswer, Citations: articles}, nil
	}
	return &ChatAnswer{Answer: answer, Citations: articles}, nil
}

// chatSystemPrompt fixes the assistant's role and output shape: grounded in
// the provided articles only, plain text (the client does not render
// Markdown), Chinese, and honest about knowledge limits.
func chatSystemPrompt() string {
	return "你是这个技术博客的读者助手。只依据我提供的文章内容回答问题；文章里没有的，就直说博客中没有覆盖，不要编造。用简体中文、口语化的纯文本回答，不要使用任何 Markdown 标记（不要 **、#、-、代码围栏）。回答控制在 200 字以内，除非用户要求更详细的解释。"
}

// chatUserPrompt packs the question with the retrieved articles. Each
// article's content is truncated: the vector only needs the topic and the
// model's focus decays with context length anyway.
func chatUserPrompt(query string, articles []*Article) string {
	const maxContentRunes = 2000
	var b strings.Builder
	b.WriteString("参考文章：")
	for i, a := range articles {
		content := []rune(a.ContentMD)
		if len(content) > maxContentRunes {
			content = content[:maxContentRunes]
		}
		b.WriteString("\n\n【文章")
		b.WriteRune(rune('A' + i))
		b.WriteString("】标题：")
		b.WriteString(a.Title)
		b.WriteString("\n内容：")
		b.WriteString(string(content))
	}
	b.WriteString("\n\n用户问题：")
	b.WriteString(query)
	b.WriteString("\n\n请基于以上文章回答。如果引用了某篇，请在回答末尾用一行「来源：文章标题」注明。")
	return b.String()
}
