package biz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

var (
	ErrMiniCommentConflict  = errors.Conflict(v1.ErrorReason_MINI_COMMENT_CONFLICT.String(), "comment submission differs from its original request")
	ErrMiniCommentCheckBusy = errors.Conflict(v1.ErrorReason_MINI_COMMENT_CHECK_BUSY.String(), "comment check is already in progress")
	ErrMiniCommentDeleted   = errors.New(410, v1.ErrorReason_MINI_COMMENT_DELETED.String(), "this comment submission was removed")
)

type MiniCommentState string

const (
	MiniCommentUnknown     MiniCommentState = "unknown"
	MiniCommentChecking    MiniCommentState = "checking"
	MiniCommentPass        MiniCommentState = "pass"
	MiniCommentReview      MiniCommentState = "review"
	MiniCommentRisky       MiniCommentState = "risky"
	MiniCommentUnavailable MiniCommentState = "unavailable"
	MiniCommentDeleted     MiniCommentState = "deleted"
)

// MiniCommentActor is created only after the fresh WeChat code and JWT owner
// agree. These fields are never accepted from a public request DTO.
type MiniCommentActor struct {
	UserID uuid.UUID
	AppID  string
	OpenID string
}

type MiniComment struct {
	Comment          *Comment
	AppID            string
	IdentityID       uuid.UUID
	InputHash        string
	FingerprintValid bool
	State            MiniCommentState
	Generation       uint64
	LeaseUntil       time.Time
	NameTraceID      string
	ContentTraceID   string
	ErrorKind        string
}

type MiniCommentVerdict struct {
	State          MiniCommentState
	NameTraceID    string
	ContentTraceID string
	ErrorKind      string
	// Internal policy permission, never accepted from a client or persisted.
	// Legacy smart/manual recovery does not grant automatic publication.
	AutoApproveRecovery bool
}

func (c *MiniComment) Visible() bool {
	return c != nil && c.Comment != nil && c.Comment.UserID != nil && c.IdentityID != uuid.Nil && c.AppID != "" && c.Comment.Status == CommentStatusApproved && c.State == MiniCommentPass && c.FingerprintValid && c.NameTraceID != "" && c.ContentTraceID != ""
}

func (c *MiniComment) CanRecheck(now time.Time) bool {
	if c == nil || c.Comment == nil || c.State == MiniCommentDeleted {
		return false
	}
	if c.State == MiniCommentChecking && c.LeaseUntil.After(now) {
		return false
	}
	return !c.FingerprintValid || (c.State != MiniCommentPass && c.State != MiniCommentReview && c.State != MiniCommentRisky)
}

func MiniCommentInputHash(c *Comment, appID string, identityID uuid.UUID) string {
	if c == nil || c.UserID == nil || identityID == uuid.Nil {
		return ""
	}
	value, _ := json.Marshal([]string{"mini-comment-v1", c.ID.String(), c.UserID.String(), appID, identityID.String(), c.ArticleSlug, c.DisplayName, c.Content})
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func MiniCommentRequestHash(slug, content string) string {
	value, _ := json.Marshal([]string{"mini-comment-create-v1", strings.TrimSpace(slug), strings.TrimSpace(content)})
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

// MiniCommentRepo owns atomic pending creation, immutable author snapshots,
// idempotency, check generations and fixed mini-program projections. A negative
// next offset denotes end-of-list; positive offsets count scanned SQL rows.
type MiniCommentRepo interface {
	FindRequest(context.Context, uuid.UUID, string, string, string) (*MiniComment, error)
	CreatePending(context.Context, *Comment, MiniCommentActor, string, string, time.Duration) (*MiniComment, bool, error)
	FindOwned(context.Context, uuid.UUID, uuid.UUID, string) (*MiniComment, error)
	BeginRecheck(context.Context, uuid.UUID, MiniCommentActor, time.Duration) (*MiniComment, bool, error)
	CompleteCheck(context.Context, *MiniComment, MiniCommentVerdict, bool) (*MiniComment, error)
	ListPublic(context.Context, string, string, int, int) ([]*MiniComment, int, error)
	ListMine(context.Context, string, uuid.UUID, string, int, int) ([]*MiniComment, int, error)
}

// A separate named dependency lets the safety flow use context-aware Redis
// without changing the existing website limiter's client options.
type MiniCommentLimiter interface {
	Allow(context.Context, string) (bool, error)
}
