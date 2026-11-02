package biz

import (
	"context"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

const miniCommentCheckLease = 30 * time.Second

type MiniCommentUsecase struct {
	base     *CommentUsecase
	identity *MiniContentIdentityUsecase
	checker  ContentSafetyChecker
	repo     MiniCommentRepo
	limiter  MiniCommentLimiter
}

func NewMiniCommentUsecase(base *CommentUsecase, identity *MiniContentIdentityUsecase, checker ContentSafetyChecker, repo MiniCommentRepo, limiter MiniCommentLimiter) *MiniCommentUsecase {
	return &MiniCommentUsecase{base: base, identity: identity, checker: checker, repo: repo, limiter: limiter}
}

func (uc *MiniCommentUsecase) appID() string {
	if uc == nil || uc.base == nil || uc.base.users == nil || uc.base.articles == nil || uc.identity == nil || uc.checker == nil || uc.repo == nil || uc.identity.AppID() == "" || uc.identity.AppID() != uc.checker.AppID() {
		return ""
	}
	return uc.identity.AppID()
}

func (uc *MiniCommentUsecase) Ready() bool {
	return uc.appID() != "" && uc.checker.Configured()
}

func (uc *MiniCommentUsecase) CanRecheck(c *MiniComment) bool {
	return uc.Ready() && c.CanRecheck(time.Now())
}

func (uc *MiniCommentUsecase) account(ctx context.Context, claims *Claims) (*User, error) {
	if claims == nil || claims.UserID == uuid.Nil {
		return nil, ErrAuthUnauthorized
	}
	if uc == nil || uc.base == nil || uc.base.users == nil {
		return nil, ErrContentSafetyDisabled
	}
	user, err := uc.base.users.ByID(ctx, claims.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return nil, ErrAuthUnauthorized
	}
	if err != nil || user == nil {
		return nil, ErrContentSafetyUnavailable
	}
	if user.ID != claims.UserID {
		return nil, ErrAuthForbidden
	}
	return user, nil
}

func validMiniRequestID(key string) bool {
	if key == "" || len(key) > 64 {
		return false
	}
	for _, r := range key {
		switch {
		case r == '-', r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func (uc *MiniCommentUsecase) budget(ctx context.Context, userID uuid.UUID, ip string) error {
	if uc.limiter == nil {
		return ErrContentSafetyUnavailable
	}
	keys := []string{"comment:user:" + userID.String()}
	if ip != "" {
		keys = append(keys, "comment:"+ip)
	}
	for _, key := range keys {
		ok, err := uc.limiter.Allow(ctx, key)
		if err != nil {
			return ErrContentSafetyUnavailable
		}
		if !ok {
			return ErrCommentTooManyAttempts
		}
	}
	return nil
}

func miniStoreError(err error) error {
	for _, fixed := range []error{ErrCommentNotFound, ErrMiniCommentConflict, ErrMiniCommentCheckBusy, ErrMiniCommentDeleted, ErrAuthForbidden, ErrContentSafetyIdentity, ErrCommentInvalidArgument} {
		if errors.Is(err, fixed) {
			return fixed
		}
	}
	if err == nil {
		return nil
	}
	return ErrContentSafetyUnavailable
}

func miniSafeRecord(c *MiniComment) *MiniComment {
	if c == nil || c.Comment == nil {
		return nil
	}
	copyRecord, copyComment := *c, *c.Comment
	copyComment.AvatarURL = ""
	copyRecord.Comment = &copyComment
	return &copyRecord
}

func (uc *MiniCommentUsecase) Create(ctx context.Context, claims *Claims, slug, content, code, requestID, ip string) (*MiniComment, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	user, err := uc.account(ctx, claims)
	if err != nil {
		return nil, err
	}
	appID := uc.appID()
	if appID == "" {
		return nil, ErrContentSafetyDisabled
	}
	if !validMiniRequestID(requestID) {
		return nil, ErrCommentInvalidArgument
	}
	slug, content = strings.TrimSpace(slug), strings.TrimSpace(content)
	requestHash := MiniCommentRequestHash(slug, content)
	previous, err := uc.repo.FindRequest(ctx, user.ID, appID, requestID, requestHash)
	if err == nil {
		if previous == nil || previous.Comment == nil {
			return nil, ErrContentSafetyUnavailable
		}
		if previous.Comment.UserID == nil || *previous.Comment.UserID != user.ID || previous.AppID != appID {
			return nil, ErrAuthForbidden
		}
		return miniSafeRecord(previous), nil
	}
	if !errors.Is(err, ErrCommentNotFound) {
		return nil, miniStoreError(err)
	}
	if !uc.Ready() {
		return nil, ErrContentSafetyDisabled
	}
	c := &Comment{ArticleSlug: slug, Content: content, DisplayName: user.DisplayName, UserID: &user.ID, Status: CommentStatusPending}
	if err := validateComment(c); err != nil {
		return nil, err
	}
	if _, err := uc.base.articles.GetPublicArticle(ctx, slug); err != nil {
		return nil, err
	}
	if err := uc.budget(ctx, user.ID, ip); err != nil {
		return nil, err
	}
	openid, err := uc.identity.Resolve(ctx, claims, code)
	if err != nil {
		return nil, err
	}
	r, created, err := uc.repo.CreatePending(ctx, c, MiniCommentActor{UserID: user.ID, AppID: appID, OpenID: openid}, requestID, requestHash, miniCommentCheckLease)
	if err != nil {
		return nil, miniStoreError(err)
	}
	if r == nil || r.Comment == nil {
		return nil, ErrContentSafetyUnavailable
	}
	if r.Comment.UserID == nil || *r.Comment.UserID != user.ID || r.AppID != appID {
		return nil, ErrAuthForbidden
	}
	if !created {
		return miniSafeRecord(r), nil
	}
	approve := uc.base.moderation.status(r.Comment.DisplayName, r.Comment.Content) == CommentStatusApproved
	return uc.check(ctx, r, openid, approve)
}

func safetyErrorKind(err error) string {
	if errors.Is(err, ErrContentSafetyIdentity) {
		return "identity"
	}
	if errors.Is(err, ErrContentSafetyInput) {
		return "input"
	}
	if errors.Is(err, ErrContentSafetyDisabled) {
		return "disabled"
	}
	return "unavailable"
}

func (uc *MiniCommentUsecase) check(ctx context.Context, record *MiniComment, openid string, approve bool) (*MiniComment, error) {
	v := MiniCommentVerdict{State: MiniCommentUnavailable, ErrorKind: "unavailable"}
	for i, in := range []ContentSafetyInput{{OpenID: openid, Content: record.Comment.DisplayName, Scene: ContentSafetyProfile}, {OpenID: openid, Content: record.Comment.Content, Scene: ContentSafetyComment}} {
		result, err := uc.checker.CheckText(ctx, in)
		if err != nil {
			v.ErrorKind = safetyErrorKind(err)
			break
		}
		if result.TraceID == "" && result.Decision == ContentSafetyPass {
			break
		}
		if i == 0 {
			v.NameTraceID = result.TraceID
		} else {
			v.ContentTraceID = result.TraceID
		}
		switch result.Decision {
		case ContentSafetyRisky:
			v.State = MiniCommentRisky
			v.ErrorKind = ""
		case ContentSafetyReview:
			v.State = MiniCommentReview
			v.ErrorKind = ""
		case ContentSafetyPass:
			if i == 1 {
				v.State = MiniCommentPass
				v.ErrorKind = ""
			}
			continue
		}
		break
	}
	updated, err := uc.repo.CompleteCheck(ctx, record, v, approve && v.State == MiniCommentPass)
	if err == nil && updated != nil && updated.Comment != nil {
		return miniSafeRecord(updated), nil
	}
	if errors.Is(err, ErrMiniCommentDeleted) || errors.Is(err, ErrCommentNotFound) {
		return nil, ErrMiniCommentDeleted
	}
	// Creation is already durable. Keep its receipt instead of encouraging a
	// second submission; the author list/recheck can resolve a lost check ack.
	current, readErr := uc.repo.FindOwned(ctx, record.Comment.ID, *record.Comment.UserID, record.AppID)
	if readErr == nil && current != nil && current.Comment != nil {
		return miniSafeRecord(current), nil
	}
	if errors.Is(readErr, ErrMiniCommentDeleted) || errors.Is(readErr, ErrCommentNotFound) {
		return nil, ErrMiniCommentDeleted
	}
	return miniSafeRecord(record), nil
}

func (uc *MiniCommentUsecase) Recheck(ctx context.Context, claims *Claims, id uuid.UUID, code, ip string) (*MiniComment, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	user, err := uc.account(ctx, claims)
	if err != nil {
		return nil, err
	}
	appID := uc.appID()
	if appID == "" {
		return nil, ErrContentSafetyDisabled
	}
	r, err := uc.repo.FindOwned(ctx, id, user.ID, appID)
	if err != nil {
		return nil, miniStoreError(err)
	}
	if r == nil || r.Comment == nil {
		return nil, ErrCommentNotFound
	}
	if !r.CanRecheck(time.Now()) {
		if r.State == MiniCommentChecking {
			return nil, ErrMiniCommentCheckBusy
		}
		return miniSafeRecord(r), nil
	}
	if !uc.Ready() {
		return nil, ErrContentSafetyDisabled
	}
	if _, err := uc.base.articles.GetPublicArticle(ctx, r.Comment.ArticleSlug); err != nil {
		return nil, err
	}
	if err := uc.budget(ctx, user.ID, ip); err != nil {
		return nil, err
	}
	openid, err := uc.identity.Resolve(ctx, claims, code)
	if err != nil {
		return nil, err
	}
	r, begun, err := uc.repo.BeginRecheck(ctx, id, MiniCommentActor{UserID: user.ID, AppID: appID, OpenID: openid}, miniCommentCheckLease)
	if err != nil {
		return nil, miniStoreError(err)
	}
	if r == nil || r.Comment == nil {
		return nil, ErrCommentNotFound
	}
	if !begun {
		return miniSafeRecord(r), nil
	}
	return uc.check(ctx, r, openid, false)
}

func (uc *MiniCommentUsecase) ListPublic(ctx context.Context, slug string, limit, offset int) ([]*MiniComment, int, error) {
	if !ValidSlug(slug) {
		return nil, -1, ErrCommentInvalidArgument
	}
	if uc.appID() == "" {
		return []*MiniComment{}, -1, nil
	}
	if _, err := uc.base.articles.GetPublicArticle(ctx, slug); err != nil {
		if errors.IsNotFound(err) {
			return []*MiniComment{}, -1, nil
		}
		return nil, -1, err
	}
	rows, next, err := uc.repo.ListPublic(ctx, slug, uc.appID(), limit, offset)
	if err != nil {
		return nil, -1, miniStoreError(err)
	}
	result := make([]*MiniComment, 0, len(rows))
	for _, r := range rows {
		if r.Visible() && r.AppID == uc.appID() {
			result = append(result, miniSafeRecord(r))
		}
	}
	return result, next, nil
}

func (uc *MiniCommentUsecase) ListMine(ctx context.Context, claims *Claims, slug string, limit, offset int) ([]*MiniComment, int, error) {
	user, err := uc.account(ctx, claims)
	if err != nil {
		return nil, -1, err
	}
	if !ValidSlug(slug) {
		return nil, -1, ErrCommentInvalidArgument
	}
	if uc.appID() == "" {
		return []*MiniComment{}, -1, nil
	}
	if _, err := uc.base.articles.GetPublicArticle(ctx, slug); err != nil {
		if errors.IsNotFound(err) {
			return []*MiniComment{}, -1, nil
		}
		return nil, -1, err
	}
	rows, next, err := uc.repo.ListMine(ctx, slug, user.ID, uc.appID(), limit, offset)
	if err != nil {
		return nil, -1, miniStoreError(err)
	}
	result := make([]*MiniComment, 0, len(rows))
	for _, r := range rows {
		if r != nil && r.Comment != nil && r.Comment.UserID != nil && *r.Comment.UserID == user.ID && r.AppID == uc.appID() {
			result = append(result, miniSafeRecord(r))
		}
	}
	return result, next, nil
}
