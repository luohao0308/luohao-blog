package data

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	blogserver "github.com/luohao0308/luohao-blog/backend/internal/server"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
)

type capturedAccountMailer struct {
	mu       sync.Mutex
	messages []biz.AccountEmailMessage
	fail     bool
}

func (*capturedAccountMailer) Configured() bool { return true }
func (m *capturedAccountMailer) Send(_ context.Context, message biz.AccountEmailMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
	if m.fail {
		return context.DeadlineExceeded
	}
	return nil
}
func (m *capturedAccountMailer) latest(to string, purpose biz.EmailPurpose) (biz.AccountEmailMessage, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].To == to && m.messages[i].Purpose == purpose {
			return m.messages[i], true
		}
	}
	return biz.AccountEmailMessage{}, false
}
func (m *capturedAccountMailer) reset() { m.mu.Lock(); m.messages = nil; m.fail = false; m.mu.Unlock() }

func TestAccountEmailHTTPLifecycle(t *testing.T) {
	ctx := context.Background()
	articleRepo, db := newTestArticleRepo(t)
	d := &Data{db: db}
	rdb, mr := newTestRedis(t)
	users := biz.NewUserUsecase(NewUserRepo(d))
	sessions := NewSessionRepo(rdb)
	tokens := NewAccountEmailTokenRepo(d)
	mailer := &capturedAccountMailer{}
	accountEmail := biz.NewAccountEmailUsecase(users, tokens, NewEmailRateLimiter(rdb), mailer, sessions, biz.AccountEmailSettings{SiteOrigin: "http://127.0.0.1:3000"})
	if err := accountEmail.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = accountEmail.Stop(context.Background()) }()
	authConfig := jwtTestConfig("account-email-fixture-signing-key", time.Hour)
	issuer, err := NewTokenIssuer(authConfig)
	if err != nil {
		t.Fatal(err)
	}
	authz, err := NewAuthorizer()
	if err != nil {
		t.Fatal(err)
	}
	authUC := biz.NewAuthUsecaseWithEmail(users, sessions, issuer, NewRateLimiter(rdb, authConfig), NewRegisterRateLimiter(rdb, authConfig), time.Hour, tokens)
	articles := biz.NewArticleUsecase(articleRepo, nil)
	commentUC := biz.NewCommentUsecase(NewCommentRepo(d), articles, users, NewCommentRateLimiter(rdb, authConfig), NewCommentModerationPolicy(authConfig))
	mediaStore, err := NewMediaStore(&conf.Data{UploadsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := blogserver.NewHTTPServer(&conf.Server{Http: &conf.Server_HTTP{}}, issuer, authz,
		service.NewArticleService(articles), service.NewCategoryService(biz.NewCategoryUsecase(NewCategoryRepo(d))), service.NewProjectService(biz.NewProjectUsecase(NewProjectRepo(d))), service.NewAboutService(biz.NewAboutUsecase(NewAboutRepo(d))),
		service.NewCollectionService(biz.NewCollectionUsecase(NewCollectionRepo(d))),
		service.NewAuthServiceWithEmail(authUC, nil, accountEmail, authConfig), service.NewUserService(users, nil), service.NewMediaService(biz.NewMediaUsecase(mediaStore)), service.NewCommentService(commentUC), service.NewSubscriberService(nil), service.NewArticleSearchService(articles, nil), service.NewChatService(nil), service.NewOfficialAccountService(nil))
	if err != nil {
		t.Fatal(err)
	}
	srv.HandleFunc("/fixture/emails/latest", func(w http.ResponseWriter, r *http.Request) {
		purpose := biz.EmailPurpose(r.URL.Query().Get("purpose"))
		message, ok := mailer.latest(r.URL.Query().Get("to"), purpose)
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"available":false}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"available": true, "link": message.Link, "purpose": message.Purpose})
	})
	server := httptest.NewServer(srv)
	defer server.Close()

	type result struct {
		status int
		body   []byte
		header http.Header
	}
	request := func(method, path, bearer, cookie string, body any) result {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, reqErr := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
		if reqErr != nil {
			t.Fatal(reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		resp, doErr := server.Client().Do(req)
		if doErr != nil {
			t.Fatal(doErr)
		}
		defer func() { _ = resp.Body.Close() }()
		data := new(bytes.Buffer)
		_, _ = data.ReadFrom(resp.Body)
		return result{status: resp.StatusCode, body: data.Bytes(), header: resp.Header.Clone()}
	}
	register := request(http.MethodPost, "/v1/auth/register", "", "", map[string]string{"email": "email-flow@example.com", "password": "oldpassword1", "display_name": "Email Flow"})
	if register.status != http.StatusOK {
		t.Fatalf("register = %d %s", register.status, register.body)
	}
	var login map[string]any
	if err = json.Unmarshal(register.body, &login); err != nil {
		t.Fatal(err)
	}
	access := login["access_token"].(string)
	cookie := register.header.Get("Set-Cookie")
	if user := login["user"].(map[string]any); user["email_verified"] != false {
		t.Fatalf("new user = %#v", user)
	}
	if got := request(http.MethodPost, "/v1/auth/email-verification/request", "", "", map[string]any{}); got.status != http.StatusUnauthorized {
		t.Fatalf("anon verify request = %d", got.status)
	}
	if got := request(http.MethodPost, "/v1/auth/email-verification/request", access, "", map[string]any{}); got.status != http.StatusOK {
		t.Fatalf("verify request = %d %s", got.status, got.body)
	}
	verify := waitCapturedMail(t, mailer, "email-flow@example.com", biz.EmailPurposeVerify)
	verifyToken := fragmentToken(t, verify.Link)
	if got := request(http.MethodPost, "/v1/auth/password-reset/confirm", "", "", map[string]string{"token": verifyToken, "new_password": "newpassword1"}); got.status != http.StatusBadRequest {
		t.Fatalf("cross-purpose = %d", got.status)
	}
	if got := request(http.MethodPost, "/v1/auth/email-verification/confirm", "", "", map[string]string{"token": verifyToken}); got.status != http.StatusOK {
		t.Fatalf("verify confirm = %d %s", got.status, got.body)
	}
	me := request(http.MethodGet, "/v1/auth/me", access, "", nil)
	if me.status != http.StatusOK || !bytes.Contains(me.body, []byte(`"email_verified":true`)) {
		t.Fatalf("verified me = %d %s", me.status, me.body)
	}

	known := request(http.MethodPost, "/v1/auth/password-reset/request", "", "", map[string]string{"email": "email-flow@example.com"})
	unknown := request(http.MethodPost, "/v1/auth/password-reset/request", "", "", map[string]string{"email": "missing@example.com"})
	if known.status != http.StatusOK || unknown.status != known.status || !bytes.Equal(known.body, unknown.body) {
		t.Fatalf("enumeration responses differ known=%d/%s unknown=%d/%s", known.status, known.body, unknown.status, unknown.body)
	}
	reset := waitCapturedMail(t, mailer, "email-flow@example.com", biz.EmailPurposeReset)
	resetToken := fragmentToken(t, reset.Link)
	if got := request(http.MethodPost, "/v1/auth/password-reset/confirm", "", "", map[string]string{"token": resetToken, "new_password": "short"}); got.status != http.StatusBadRequest {
		t.Fatalf("weak reset = %d", got.status)
	}
	if got := request(http.MethodPost, "/v1/auth/password-reset/confirm", "", "", map[string]string{"token": resetToken, "new_password": "newpassword1"}); got.status != http.StatusOK {
		t.Fatalf("reset confirm = %d %s", got.status, got.body)
	}
	if got := request(http.MethodPost, "/v1/auth/password-reset/confirm", "", "", map[string]string{"token": resetToken, "new_password": "anotherpass1"}); got.status != http.StatusBadRequest {
		t.Fatalf("reset replay = %d", got.status)
	}
	if got := request(http.MethodPost, "/v1/auth/refresh", "", cookie, map[string]any{}); got.status != http.StatusUnauthorized {
		t.Fatalf("old refresh = %d", got.status)
	}
	if got := request(http.MethodPost, "/v1/auth/login", "", "", map[string]string{"email": "email-flow@example.com", "password": "oldpassword1"}); got.status != http.StatusUnauthorized {
		t.Fatalf("old password = %d", got.status)
	}
	if got := request(http.MethodPost, "/v1/auth/login", "", "", map[string]string{"email": "email-flow@example.com", "password": "newpassword1"}); got.status != http.StatusOK {
		t.Fatalf("new password = %d %s", got.status, got.body)
	}

	if file := os.Getenv("BLOG_ACCOUNT_EMAIL_BROWSER_FIXTURE"); file != "" {
		mr.FlushAll()
		mailer.reset()
		if err = os.WriteFile(file, []byte(server.URL), 0o600); err != nil {
			t.Fatal(err)
		}
		deadline := time.After(10 * time.Minute)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-deadline:
				return
			case <-ticker.C:
				if _, statErr := os.Stat(file + ".stop"); statErr == nil {
					return
				}
			}
		}
	}
}

func waitCapturedMail(t *testing.T, mailer *capturedAccountMailer, to string, purpose biz.EmailPurpose) biz.AccountEmailMessage {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if message, ok := mailer.latest(to, purpose); ok {
			return message
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("mail not captured for purpose %s", purpose)
	return biz.AccountEmailMessage{}
}

func fragmentToken(t *testing.T, link string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		t.Fatal(err)
	}
	token := values.Get("token")
	if token == "" {
		t.Fatal("mail link has no fragment token")
	}
	return token
}
