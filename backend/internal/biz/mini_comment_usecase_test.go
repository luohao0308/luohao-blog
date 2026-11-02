package biz

import (
	"context"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type miniRepoFixture struct {
	record, replay     *MiniComment
	storedName         string
	creates, completes int
	approve            bool
	recoveryApproval   bool
	completeErr        error
}

func (f *miniRepoFixture) FindRequest(context.Context, uuid.UUID, string, string, string) (*MiniComment, error) {
	if f.replay != nil {
		return f.replay, nil
	}
	return nil, ErrCommentNotFound
}
func (f *miniRepoFixture) CreatePending(_ context.Context, c *Comment, a MiniCommentActor, _, _ string, lease time.Duration) (*MiniComment, bool, error) {
	f.creates++
	snapshot := *c
	snapshot.ID = uuid.New()
	snapshot.Status = CommentStatusPending
	if f.storedName != "" {
		snapshot.DisplayName = f.storedName
	}
	f.record = &MiniComment{Comment: &snapshot, AppID: a.AppID, IdentityID: uuid.New(), State: MiniCommentChecking, Generation: 1, LeaseUntil: time.Now().Add(lease), FingerprintValid: true}
	f.record.InputHash = MiniCommentInputHash(&snapshot, a.AppID, f.record.IdentityID)
	return f.record, true, nil
}
func (f *miniRepoFixture) FindOwned(_ context.Context, _, _ uuid.UUID, _ string) (*MiniComment, error) {
	return f.record, nil
}
func (f *miniRepoFixture) BeginRecheck(_ context.Context, _ uuid.UUID, _ MiniCommentActor, _ time.Duration) (*MiniComment, bool, error) {
	return f.record, true, nil
}
func (f *miniRepoFixture) CompleteCheck(_ context.Context, c *MiniComment, v MiniCommentVerdict, approve bool) (*MiniComment, error) {
	f.completes++
	f.approve = approve
	f.recoveryApproval = v.AutoApproveRecovery
	if f.completeErr != nil {
		return nil, f.completeErr
	}
	c.State = v.State
	c.NameTraceID = v.NameTraceID
	c.ContentTraceID = v.ContentTraceID
	c.ErrorKind = v.ErrorKind
	if approve && v.State == MiniCommentPass {
		c.Comment.Status = CommentStatusApproved
	}
	return c, nil
}
func (f *miniRepoFixture) ListPublic(context.Context, string, string, int, int) ([]*MiniComment, int, error) {
	return []*MiniComment{f.record}, -1, nil
}
func (f *miniRepoFixture) ListMine(context.Context, string, uuid.UUID, string, int, int) ([]*MiniComment, int, error) {
	return []*MiniComment{f.record}, -1, nil
}

type miniCheckerFixture struct {
	decision     ContentSafetyDecision
	err          error
	inputs       []ContentSafetyInput
	enabled      bool
	withoutTrace bool
}

func (*miniCheckerFixture) AppID() string      { return "wx-fixture" }
func (f *miniCheckerFixture) Configured() bool { return f.enabled }
func (f *miniCheckerFixture) CheckText(_ context.Context, in ContentSafetyInput) (ContentSafetyResult, error) {
	f.inputs = append(f.inputs, in)
	trace := "trace-fixture"
	if f.withoutTrace {
		trace = ""
	}
	return ContentSafetyResult{Decision: f.decision, TraceID: trace}, f.err
}

func newMiniUsecaseFixture(t *testing.T) (*MiniCommentUsecase, *miniRepoFixture, *miniCheckerFixture, *Claims) {
	t.Helper()
	base, articles, author, claims := newCommentTestUsecase(t, &allowAllLimiter{})
	publishCommentableArticle(t, articles)
	if err := base.users.repo.BindWechatIdentity(context.Background(), author.ID, 0, "wx-fixture", "openid-fixture"); err != nil {
		t.Fatal(err)
	}
	identity := NewMiniContentIdentityUsecase(base.users, &fakeWechatClient{codes: map[string]string{"code-fixture": "openid-fixture"}})
	repo := &miniRepoFixture{}
	checker := &miniCheckerFixture{decision: ContentSafetyPass, enabled: true}
	return NewMiniCommentUsecase(base, identity, checker, repo, &allowAllLimiter{}), repo, checker, claims
}

func TestMiniCommentChecksStoredSnapshotAndBothScenes(t *testing.T) {
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	repo.storedName = "实际存储昵称"
	r, err := uc.Create(context.Background(), claims, "commentable", " 正文 ", "code-fixture", "intent-01", "127.0.0.1")
	if err != nil || !r.Visible() || repo.creates != 1 || repo.completes != 1 || !repo.approve {
		t.Fatalf("create result: %v %v", r, err)
	}
	if len(checker.inputs) != 2 || checker.inputs[0].Scene != ContentSafetyProfile || checker.inputs[0].Content != repo.storedName || checker.inputs[1].Scene != ContentSafetyComment || checker.inputs[1].Content != "正文" {
		t.Fatalf("not checking persisted fields: %+v", checker.inputs)
	}
	if r.Comment.AvatarURL != "" {
		t.Fatal("unverified avatar escaped projection")
	}
}

func TestMiniCommentNonPassNeverAutoApproves(t *testing.T) {
	for _, decision := range []ContentSafetyDecision{ContentSafetyReview, ContentSafetyRisky, "unexpected"} {
		t.Run(string(decision), func(t *testing.T) {
			uc, repo, checker, claims := newMiniUsecaseFixture(t)
			checker.decision = decision
			r, err := uc.Create(context.Background(), claims, "commentable", "正文", "code-fixture", "intent-01", "")
			if err != nil || r.Visible() || repo.approve || r.Comment.Status != CommentStatusPending {
				t.Fatalf("non-pass escaped: %v %v", r, err)
			}
		})
	}
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	checker.err = ErrContentSafetyIdentity
	r, err := uc.Create(context.Background(), claims, "commentable", "正文", "code-fixture", "intent-01", "")
	if err != nil || r.State != MiniCommentUnavailable || r.ErrorKind != "identity" || repo.approve {
		t.Fatalf("identity outage lost saved comment: %v %v", r, err)
	}
}

func TestMiniCommentReplayDoesNotConsumeWechatCode(t *testing.T) {
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	first, err := uc.Create(context.Background(), claims, "commentable", "正文", "code-fixture", "intent-01", "")
	if err != nil {
		t.Fatal(err)
	}
	repo.replay = first
	checker.enabled = false
	again, err := uc.Create(context.Background(), claims, "commentable", "正文", "invalid-used-code", "intent-01", "")
	if err != nil || again.Comment.ID != first.Comment.ID || repo.creates != 1 || len(checker.inputs) != 2 {
		t.Fatalf("idempotent read repeated creation/check: %v %v", again, err)
	}
}

func TestMiniCommentSavedReceiptSurvivesCheckCommitFailure(t *testing.T) {
	uc, repo, _, claims := newMiniUsecaseFixture(t)
	repo.completeErr = ErrContentSafetyUnavailable
	r, err := uc.Create(context.Background(), claims, "commentable", "正文", "code-fixture", "intent-01", "")
	if err != nil || r == nil || r.Comment.ID == uuid.Nil || r.Visible() {
		t.Fatalf("lost durable receipt after check failure: %v %v", r, err)
	}
}

func TestMiniCommentMissingIdentityNeverPersists(t *testing.T) {
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	_, err := uc.Create(context.Background(), claims, "commentable", "正文", "bad-code", "intent-01", "")
	if !errors.Is(err, ErrContentSafetyIdentity) || repo.creates != 0 || len(checker.inputs) != 0 {
		t.Fatalf("unproved identity persisted: %v", err)
	}
}

func TestMiniCommentDeletionDuringCheckDoesNotReturnStaleReceipt(t *testing.T) {
	uc, repo, _, claims := newMiniUsecaseFixture(t)
	repo.completeErr = ErrMiniCommentDeleted
	record, err := uc.Create(context.Background(), claims, "commentable", "正文", "code-fixture", "intent-01", "")
	if record != nil || !errors.Is(err, ErrMiniCommentDeleted) {
		t.Fatalf("deleted comment echoed as pending: %v / %v", record, err)
	}
}

func TestMiniCommentKnownRiskWithoutTraceCannotBeRetried(t *testing.T) {
	uc, repo, checker, claims := newMiniUsecaseFixture(t)
	checker.decision, checker.withoutTrace = ContentSafetyRisky, true
	record, err := uc.Create(context.Background(), claims, "commentable", "正文", "code-fixture", "intent-01", "")
	if err != nil || record.State != MiniCommentRisky || record.CanRecheck(time.Now()) || repo.approve {
		t.Fatalf("known risk became a retryable outage: %v / %v", record, err)
	}
}
