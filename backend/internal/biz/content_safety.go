package biz

import (
	"context"

	"github.com/go-kratos/kratos/v3/errors"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

type ContentSafetyScene int

const (
	ContentSafetyProfile ContentSafetyScene = 1
	ContentSafetyComment ContentSafetyScene = 2
)

type ContentSafetyDecision string

const (
	ContentSafetyPass   ContentSafetyDecision = "pass"
	ContentSafetyReview ContentSafetyDecision = "review"
	ContentSafetyRisky  ContentSafetyDecision = "risky"
)

type ContentSafetyInput struct {
	OpenID, Content string
	Scene           ContentSafetyScene
}
type ContentSafetyResult struct {
	Decision ContentSafetyDecision
	TraceID  string
	Label    int
}
type ContentSafetyChecker interface {
	AppID() string
	Configured() bool
	CheckText(context.Context, ContentSafetyInput) (ContentSafetyResult, error)
}

var (
	ErrContentSafetyDisabled    = errors.New(412, v1.ErrorReason_CONTENT_SAFETY_DISABLED.String(), "content safety is not configured")
	ErrContentSafetyInput       = errors.BadRequest(v1.ErrorReason_CONTENT_SAFETY_INPUT.String(), "content safety input is invalid")
	ErrContentSafetyIdentity    = errors.New(412, v1.ErrorReason_CONTENT_SAFETY_IDENTITY.String(), "wechat identity must be renewed")
	ErrContentSafetyUnavailable = errors.New(503, v1.ErrorReason_CONTENT_SAFETY_UNAVAILABLE.String(), "content safety service unavailable")
)
