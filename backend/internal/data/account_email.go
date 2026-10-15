package data

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"

	"github.com/redis/go-redis/v9"
)

type emailRateLimiter struct{ rdb redis.UniversalClient }

func NewEmailRateLimiter(rdb redis.UniversalClient) biz.EmailRateLimiter {
	return &emailRateLimiter{rdb: rdb}
}
func (r *emailRateLimiter) Allow(ctx context.Context, key string, attempts int64, window time.Duration) (bool, error) {
	if attempts <= 0 || window <= 0 {
		return false, fmt.Errorf("invalid email rate limit")
	}
	k := rateKeyPrefix + "email:" + key
	pipe := r.rdb.TxPipeline()
	incr := pipe.Incr(ctx, k)
	pipe.ExpireNX(ctx, k, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return incr.Val() <= attempts, nil
}

type disabledAccountMailSender struct{}

func (*disabledAccountMailSender) Configured() bool { return false }
func (*disabledAccountMailSender) Send(context.Context, biz.AccountEmailMessage) error {
	return fmt.Errorf("mail disabled")
}

type smtpAccountMailSender struct {
	host      string
	port      int
	username  string
	password  string
	from      string
	fromName  string
	timeout   time.Duration
	tlsConfig *tls.Config
}

func (s *smtpAccountMailSender) Configured() bool { return true }

func NewAccountMailSender(bc *conf.Bootstrap) (biz.AccountMailSender, error) {
	c := bc.GetMail()
	if c == nil || !c.GetEnabled() {
		return &disabledAccountMailSender{}, nil
	}
	if c.GetSmtpPort() != 465 || c.GetSmtpHost() == "" || c.GetUsername() == "" || c.GetPassword() == "" || c.GetFromAddress() == "" || c.GetUsername() != c.GetFromAddress() || hasHeaderBreak(c.GetFromName()) {
		return nil, fmt.Errorf("invalid account mail configuration")
	}
	if _, err := mail.ParseAddress(c.GetFromAddress()); err != nil {
		return nil, fmt.Errorf("invalid account mail sender")
	}
	origin, err := validatedSiteOrigin(c.GetPublicSiteOrigin())
	if err != nil {
		return nil, err
	}
	_ = origin
	timeout := c.GetSendTimeout().AsDuration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &smtpAccountMailSender{host: c.GetSmtpHost(), port: int(c.GetSmtpPort()), username: c.GetUsername(), password: c.GetPassword(), from: c.GetFromAddress(), fromName: c.GetFromName(), timeout: timeout, tlsConfig: &tls.Config{ServerName: c.GetSmtpHost(), MinVersion: tls.VersionTLS12}}, nil
}

func NewAccountEmailSettings(bc *conf.Bootstrap) (biz.AccountEmailSettings, error) {
	c := bc.GetMail()
	if c == nil || !c.GetEnabled() {
		return biz.AccountEmailSettings{}, nil
	}
	origin, err := validatedSiteOrigin(c.GetPublicSiteOrigin())
	if err != nil {
		return biz.AccountEmailSettings{}, err
	}
	return biz.AccountEmailSettings{SiteOrigin: origin, VerificationTTL: biz.DefaultVerificationTTL, ResetTTL: biz.DefaultPasswordResetTTL}, nil
}

func validatedSiteOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("invalid mail public site origin")
	}
	loopback := u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()
	if u.Scheme != "https" && (u.Scheme != "http" || !loopback) {
		return "", fmt.Errorf("mail public site origin must use https")
	}
	return strings.TrimSuffix(u.String(), "/"), nil
}

func hasHeaderBreak(value string) bool { return strings.ContainsAny(value, "\r\n") }

func (s *smtpAccountMailSender) Send(ctx context.Context, message biz.AccountEmailMessage) error {
	if hasHeaderBreak(message.To) || hasHeaderBreak(message.Subject) || hasHeaderBreak(s.from) {
		return fmt.Errorf("invalid mail header")
	}
	if _, err := mail.ParseAddress(message.To); err != nil {
		return fmt.Errorf("invalid recipient")
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: s.timeout}, Config: s.tlsConfig.Clone()}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(s.host, strconv.Itoa(s.port)))
	if err != nil {
		return fmt.Errorf("smtp connection failed")
	}
	deadline := time.Now().Add(s.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err = conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp deadline failed")
	}
	cancelWatch := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetDeadline(time.Now())
		case <-cancelWatch:
		}
	}()
	defer close(cancelWatch)
	defer func() { _ = conn.Close() }()
	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp negotiation failed")
	}
	defer func() { _ = client.Close() }()
	if ok, mechanisms := client.Extension("AUTH"); !ok || !strings.Contains(" "+mechanisms+" ", " PLAIN ") {
		return fmt.Errorf("smtp plain auth unavailable")
	}
	if err = client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
		return fmt.Errorf("smtp authentication failed")
	}
	if err = client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp sender rejected")
	}
	if err = client.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp recipient rejected")
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data rejected")
	}
	from := s.from
	if s.fromName != "" {
		from = mime.BEncoding.Encode("UTF-8", s.fromName) + " <" + s.from + ">"
	}
	b := bufio.NewWriter(w)
	_, err = fmt.Fprintf(b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", from, message.To, mime.BEncoding.Encode("UTF-8", message.Subject), message.Body)
	if err == nil {
		err = b.Flush()
	}
	closeErr := w.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("smtp write failed")
	}
	if err = client.Quit(); err != nil {
		return fmt.Errorf("smtp quit failed")
	}
	return nil
}
