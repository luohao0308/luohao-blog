package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatidentity"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"github.com/redis/go-redis/v9"
)

// Only the outbound WeChat exchange is fake. HTTP policy, SQL credentials and
// identity ownership, tickets and sessions all exercise production components.
type httpWechatClient struct {
	app     string
	enabled bool
}

func (c *httpWechatClient) AppID() string    { return c.app }
func (c *httpWechatClient) Configured() bool { return c.enabled }
func (*httpWechatClient) Code2Session(_ context.Context, code string) (string, error) {
	return code, nil
}

type wechatHTTPFixture struct {
	handler  http.Handler
	db       *ent.Client
	users    biz.UserRepository
	bindings biz.WechatBindingStore
	client   *httpWechatClient
}

func newWechatHTTPFixture(t *testing.T, rdb redis.UniversalClient, sessions biz.SessionRepo, mysql bool) *wechatHTTPFixture {
	t.Helper()
	var db *ent.Client
	if mysql {
		db, _ = identityMySQL(t)
	} else {
		_, db = newTestArticleRepo(t)
	}
	d := &Data{db: db}
	articleRepo := NewArticleRepo(d, rdb)
	users := NewUserRepo(d)
	usersUC := biz.NewUserUsecase(users)
	cfg := jwtTestConfig("wechat-http-fixture-signing-key", time.Hour)
	issuer, err := NewTokenIssuer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	auth := biz.NewAuthUsecase(usersUC, sessions, issuer, nil, nil, time.Hour)
	client := &httpWechatClient{app: "wx-formal-http", enabled: true}
	bindings := NewWechatBindingStore(rdb)
	wechat := biz.NewWechatUsecase(usersUC, auth, client, bindings, nil)
	articles := biz.NewArticleUsecase(articleRepo, nil)
	comments := biz.NewCommentUsecase(NewCommentRepo(d), articles, usersUC, NewCommentRateLimiter(rdb, cfg), NewCommentModerationPolicy(cfg))
	media, err := NewMediaStore(&conf.Data{UploadsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz,
		service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))), service.NewProjectService(biz.NewProjectUsecase(NewProjectRepo(d))), service.NewAboutService(biz.NewAboutUsecase(NewAboutRepo(d))), service.NewCollectionService(biz.NewCollectionUsecase(NewCollectionRepo(d))), service.NewAuthService(auth, wechat, cfg), service.NewUserService(usersUC, nil), service.NewMediaService(biz.NewMediaUsecase(media)), service.NewCommentService(comments), service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(nil))
	if err != nil {
		t.Fatal(err)
	}
	return &wechatHTTPFixture{handler: srv, db: db, users: users, bindings: bindings, client: client}
}
func (f *wechatHTTPFixture) request(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}
func (f *wechatHTTPFixture) account(t *testing.T, email string) *biz.User {
	t.Helper()
	hash, err := biz.HashPassword("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	u, err := f.users.Create(context.Background(), &biz.User{Email: uuid.NewString() + "-" + email, PasswordHash: hash, DisplayName: "Original profile", Role: biz.UserRoleAdmin})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.users.UpdateAvatar(context.Background(), u.ID, "/v1/assets/avatars/0123456789abcdef0123456789abcdef.png"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.db.WechatIdentity.Delete().Where(wechatidentity.UserIDEQ(u.ID)).Exec(ctx)
		_, _ = f.db.User.Delete().Where(user.IDEQ(u.ID)).Exec(ctx)
	})
	u, err = f.users.FindByID(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
func (f *wechatHTTPFixture) ticket(t *testing.T, app, openid string) string {
	t.Helper()
	ticket, err := f.bindings.Save(context.Background(), app, openid, biz.DefaultWechatBindingTTL)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}
func wechatBindBody(ticket, email, password string) map[string]string {
	return map[string]string{"binding_ticket": ticket, "email": email, "password": password}
}
func requireWechatHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("HTTP status %d, want %d: %s", response.Code, status, response.Body.String())
	}
}

func TestWechatHTTPDisabledPreservesWebsiteAuthAndRead(t *testing.T) {
	exerciseWechatHTTPDisabled(t, false)
}
func TestWechatHTTPDisabledMySQLPreservesWebsiteAuthAndRead(t *testing.T) {
	exerciseWechatHTTPDisabled(t, true)
}
func exerciseWechatHTTPDisabled(t *testing.T, mysql bool) {
	t.Helper()
	rdb, _ := newTestRedis(t)
	f := newWechatHTTPFixture(t, rdb, NewSessionRepo(rdb), mysql)
	u := f.account(t, "disabled@example.invalid")
	ticket := f.ticket(t, f.client.app, "disabled-openid")
	f.client.enabled = false
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat", map[string]string{"code": "disabled-openid"}), http.StatusPreconditionFailed)
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(ticket, u.Email, "fixture-password")), http.StatusPreconditionFailed)
	if openid, err := f.bindings.Consume(context.Background(), f.client.app, ticket); err != nil || openid != "disabled-openid" {
		t.Fatalf("disabled bind consumed ticket: %v", err)
	}
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/login", map[string]string{"email": u.Email, "password": "fixture-password"}), http.StatusOK)
	requireWechatHTTPStatus(t, f.request(t, http.MethodGet, "/v1/articles/list", nil), http.StatusOK)
}

func TestWechatHTTPFormalBindingPreservesLegacyAndWebsiteAccount(t *testing.T) {
	ctx := context.Background()
	rdb, _ := newTestRedis(t)
	f := newWechatHTTPFixture(t, rdb, NewSessionRepo(rdb), true)
	u := f.account(t, "binding@example.invalid")
	if _, err := f.db.User.UpdateOneID(u.ID).SetWechatOpenid("same-openid").Save(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.users.BindWechatIdentity(ctx, u.ID, u.CredentialVersion, "wx-legacy-http", "same-openid"); err != nil {
		t.Fatal(err)
	}
	login := f.request(t, http.MethodPost, "/v1/auth/wechat", map[string]string{"code": "same-openid"})
	requireWechatHTTPStatus(t, login, http.StatusOK)
	var payload struct {
		Status        v1.WechatLoginStatus `json:"status"`
		BindingTicket string               `json:"binding_ticket"`
		Login         json.RawMessage      `json:"login"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != v1.WechatLoginStatus_WECHAT_LOGIN_STATUS_BINDING_REQUIRED || payload.BindingTicket == "" || len(payload.Login) != 0 && string(payload.Login) != "null" {
		t.Fatal("formal identity incorrectly auto-logged into legacy account")
	}
	bound := f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(payload.BindingTicket, u.Email, "fixture-password"))
	requireWechatHTTPStatus(t, bound, http.StatusOK)
	var reply struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID          string      `json:"id"`
			Role        v1.UserRole `json:"role"`
			Avatar      string      `json:"avatar_url"`
			DisplayName string      `json:"display_name"`
		} `json:"user"`
	}
	if err := json.Unmarshal(bound.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	if reply.AccessToken == "" || reply.User.ID != u.ID.String() || reply.User.Role != v1.UserRole_USER_ROLE_ADMIN || reply.User.Avatar != u.AvatarURL || reply.User.DisplayName != u.DisplayName || len(bound.Result().Cookies()) != 1 {
		t.Fatal("binding changed public account or did not issue session")
	}
	row, err := f.db.User.Get(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if row.PasswordHash != u.PasswordHash || row.WechatOpenid != "same-openid" || row.CredentialVersion != u.CredentialVersion {
		t.Fatal("website credentials or legacy column changed")
	}
	for _, app := range []string{"wx-legacy-http", f.client.app} {
		identity, err := f.users.FindByWechatIdentity(ctx, app, "same-openid")
		if err != nil || identity.ID != u.ID {
			t.Fatalf("identity %s not retained: %v", app, err)
		}
	}
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(payload.BindingTicket, u.Email, "fixture-password")), http.StatusUnauthorized)
	again := f.request(t, http.MethodPost, "/v1/auth/wechat", map[string]string{"code": "same-openid"})
	requireWechatHTTPStatus(t, again, http.StatusOK)
	if err := json.Unmarshal(again.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Status != v1.WechatLoginStatus_WECHAT_LOGIN_STATUS_OK {
		t.Fatal("formal identity did not log in after binding")
	}
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/login", map[string]string{"email": u.Email, "password": "fixture-password"}), http.StatusOK)
}

func TestWechatHTTPBindingNegativeCases(t *testing.T) {
	ctx := context.Background()
	rdb, mr := newTestRedis(t)
	f := newWechatHTTPFixture(t, rdb, NewSessionRepo(rdb), true)
	u := f.account(t, "negative@example.invalid")
	other := f.account(t, "other@example.invalid")
	cross := f.ticket(t, "wx-other-http", "cross-openid")
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(cross, u.Email, "fixture-password")), http.StatusUnauthorized)
	wrong := f.ticket(t, f.client.app, "wrong-password-openid")
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(wrong, u.Email, "wrong-password")), http.StatusUnauthorized)
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(wrong, u.Email, "fixture-password")), http.StatusUnauthorized)
	expired := f.ticket(t, f.client.app, "expired-openid")
	mr.FastForward(11 * time.Minute)
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(expired, u.Email, "fixture-password")), http.StatusUnauthorized)
	legacy, err := newOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := rdb.Set(ctx, "blog:wechatbind:"+legacy, "legacy-openid", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(legacy, u.Email, "fixture-password")), http.StatusUnauthorized)
	if err := f.users.BindWechatIdentity(ctx, other.ID, other.CredentialVersion, f.client.app, "owned-openid"); err != nil {
		t.Fatal(err)
	}
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(f.ticket(t, f.client.app, "owned-openid"), u.Email, "fixture-password")), http.StatusConflict)
	if err := f.users.BindWechatIdentity(ctx, u.ID, u.CredentialVersion, f.client.app, "first-openid"); err != nil {
		t.Fatal(err)
	}
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(f.ticket(t, f.client.app, "replacement-openid"), u.Email, "fixture-password")), http.StatusConflict)
	requireWechatHTTPStatus(t, f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(f.ticket(t, f.client.app, "first-openid"), u.Email, "fixture-password")), http.StatusOK)
	owner, err := f.users.FindByWechatIdentity(ctx, f.client.app, "owned-openid")
	if err != nil || owner.ID != other.ID {
		t.Fatal("conflict changed identity owner")
	}
	retained, err := f.users.FindByWechatIdentity(ctx, f.client.app, "first-openid")
	if err != nil || retained.ID != u.ID {
		t.Fatal("conflict replaced existing identity")
	}
}

// Reset after the actual Redis Create commits: the final version recheck must
// reject the response and delete this precise refresh token and index member.
type resetAfterWechatSessionCreate struct {
	biz.SessionRepo
	users biz.UserRepository
	token string
}

func (s *resetAfterWechatSessionCreate) Create(ctx context.Context, id uuid.UUID, version uint64, ttl time.Duration) (*biz.RefreshSession, error) {
	session, err := s.SessionRepo.Create(ctx, id, version, ttl)
	if err != nil {
		return nil, err
	}
	s.token = session.Token
	_, changed, err := s.users.UpdatePasswordState(ctx, id, version, "reset-password-hash", false)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, biz.ErrUserInvalidCredentials
	}
	return session, nil
}
func TestWechatHTTPPostIssueResetRealRedis(t *testing.T) {
	addr := os.Getenv("REDIS_INTEGRATION_ADDR")
	if addr == "" {
		t.Skip("set REDIS_INTEGRATION_ADDR for the required real Redis release gate")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"login", "bind"} {
		t.Run(operation, func(t *testing.T) {
			ctx := context.Background()
			base := NewSessionRepo(rdb)
			sessions := &resetAfterWechatSessionCreate{SessionRepo: base}
			f := newWechatHTTPFixture(t, rdb, sessions, true)
			sessions.users = f.users
			u := f.account(t, operation+"-reset@example.invalid")
			t.Cleanup(func() { _ = base.RevokeUser(ctx, u.ID) })
			var response *httptest.ResponseRecorder
			if operation == "login" {
				if err := f.users.BindWechatIdentity(ctx, u.ID, u.CredentialVersion, f.client.app, "race-openid"); err != nil {
					t.Fatal(err)
				}
				response = f.request(t, http.MethodPost, "/v1/auth/wechat", map[string]string{"code": "race-openid"})
			} else {
				response = f.request(t, http.MethodPost, "/v1/auth/wechat/bind", wechatBindBody(f.ticket(t, f.client.app, "race-openid"), u.Email, "fixture-password"))
			}
			requireWechatHTTPStatus(t, response, http.StatusUnauthorized)
			if sessions.token == "" {
				t.Fatal("did not reach real session creation")
			}
			if bytes.Contains(response.Body.Bytes(), []byte("access_token")) || response.Header().Get("Set-Cookie") != "" {
				t.Fatal("reset race leaked HTTP credentials")
			}
			if _, err := base.Consume(ctx, sessions.token); !kratoserrors.Is(err, biz.ErrAuthInvalidRefreshToken) {
				t.Fatalf("refresh remains redeemable: %v", err)
			}
			if rdb.SIsMember(ctx, refreshUserPrefix+u.ID.String(), sessions.token).Val() {
				t.Fatal("revoked session remains in refresh index")
			}
		})
	}
}
