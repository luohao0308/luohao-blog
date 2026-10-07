package data

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	"github.com/redis/go-redis/v9"
)

// chatLLM calls an OpenAI-compatible /chat/completions endpoint (DeepSeek,
// OpenAI, self-hosted replicas — anything speaking the de-facto standard).
type chatLLM struct {
	baseURL string
	apiKey  string
	model   string
	// reasoningEffort is sent as the optional reasoning_effort field, only
	// when configured (empty = field omitted, provider default applies).
	reasoningEffort string
	http            *http.Client
}

// NewChatLLM builds the client from the bootstrap config. It returns nil when
// the LLM configuration is absent or incomplete: the chat usecase treats a nil
// LLM as "generation disabled" and the Chat RPC reports unavailable.
func NewChatLLM(c *conf.Bootstrap) biz.ChatLLM {
	return newChatLLM(c.GetLlm())
}

func newChatLLM(c *conf.Llm) biz.ChatLLM {
	if c == nil || c.GetBaseUrl() == "" || c.GetModel() == "" {
		return nil
	}
	timeout := c.GetTimeout().AsDuration()
	if timeout <= 0 {
		// Generation is inherently slower than embedding, and reasoning
		// models can think for a long time before the first token.
		timeout = 120 * time.Second
	}
	return &chatLLM{
		baseURL:         strings.TrimRight(c.GetBaseUrl(), "/"),
		apiKey:          c.GetApiKey(),
		model:           c.GetModel(),
		reasoningEffort: c.GetReasoningEffort(),
		http:            &http.Client{Timeout: timeout},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	// ReasoningEffort is omitted when empty (omitempty) so providers without
	// the concept, or deployments wanting the provider default, are unaffected.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Bound the complete provider response, including ignored fields and padding.
const maxChatResponseBytes = 1 << 20

// Generate returns the assistant reply for one system/user exchange.
func (c *chatLLM) Generate(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatCompletionsRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		ReasoningEffort: c.reasoningEffort,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat completions: status %d", res.StatusCode)
	}
	var out chatCompletionsResponse
	response, err := io.ReadAll(io.LimitReader(res.Body, maxChatResponseBytes+1))
	if err != nil {
		return "", err
	}
	if len(response) > maxChatResponseBytes {
		return "", fmt.Errorf("chat completions: response exceeds %d bytes", maxChatResponseBytes)
	}
	if err := json.Unmarshal(response, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || out.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("chat completions: empty response")
	}
	return out.Choices[0].Message.Content, nil
}

// NewChatRateLimiter builds the public-chat throttler from the auth config,
// applying the same defaults pattern as the login and comment throttlers.
func NewChatRateLimiter(rdb redis.UniversalClient, a *conf.Auth) biz.ChatRateLimiter {
	attempts := a.GetRateLimit().GetChatAttempts()
	if attempts <= 0 {
		attempts = biz.DefaultChatAttempts
	}
	window := a.GetRateLimit().GetChatWindow().AsDuration()
	if window <= 0 {
		window = biz.DefaultChatWindow
	}
	return &rateLimiter{rdb: rdb, attempts: attempts, window: window}
}
