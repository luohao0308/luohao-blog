package data

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
)

func mailBootstrap() *conf.Bootstrap {
	return &conf.Bootstrap{Mail: &conf.Mail{Enabled: true, SmtpHost: "smtpdm.aliyun.com", SmtpPort: 465, Username: "sender@example.com", Password: "fixture", FromAddress: "sender@example.com", PublicSiteOrigin: "https://example.com"}}
}

func TestAccountMailConfigurationGuards(t *testing.T) {
	disabled, err := NewAccountMailSender(&conf.Bootstrap{})
	if err != nil || disabled.Configured() {
		t.Fatalf("disabled sender = %v/%v", disabled.Configured(), err)
	}
	badPort := mailBootstrap()
	badPort.Mail.SmtpPort = 80
	if _, err = NewAccountMailSender(badPort); err == nil {
		t.Fatal("plaintext SMTP accepted")
	}
	badOrigin := mailBootstrap()
	badOrigin.Mail.PublicSiteOrigin = "http://example.com"
	if _, err = NewAccountMailSender(badOrigin); err == nil {
		t.Fatal("non-loopback HTTP origin accepted")
	}
	loopback := mailBootstrap()
	loopback.Mail.PublicSiteOrigin = "http://127.0.0.1:3000"
	sender, err := NewAccountMailSender(loopback)
	if err != nil || !sender.Configured() {
		t.Fatalf("loopback config = %v/%v", sender.Configured(), err)
	}
	if sender.(*smtpAccountMailSender).tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatal("TLS minimum is below 1.2")
	}
}

func TestEmailRateLimiterFailsClosedAndKeepsWindow(t *testing.T) {
	ctx := context.Background()
	rdb, mr := newTestRedis(t)
	limiter := NewEmailRateLimiter(rdb)
	for i := 0; i < 2; i++ {
		if ok, err := limiter.Allow(ctx, "reset:email:hash", 2, time.Hour); err != nil || !ok {
			t.Fatalf("allow %d = %v/%v", i, ok, err)
		}
	}
	if ok, err := limiter.Allow(ctx, "reset:email:hash", 2, time.Hour); err != nil || ok {
		t.Fatalf("third = %v/%v", ok, err)
	}
	mr.SetError("offline")
	if _, err := limiter.Allow(ctx, "reset:ip:127", 5, time.Hour); err == nil {
		t.Fatal("Redis failure was not surfaced")
	}
}

func TestSMTPStallRespectsTotalTimeout(t *testing.T) {
	listener, roots := newTestTLSListener(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err == nil {
			defer func() { _ = conn.Close() }()
			time.Sleep(time.Second)
		}
	}()
	host, port := splitListener(t, listener)
	sender := &smtpAccountMailSender{host: host, port: port, username: "sender@example.com", password: "secret", from: "sender@example.com", timeout: 50 * time.Millisecond, tlsConfig: &tls.Config{ServerName: "localhost", RootCAs: roots, MinVersion: tls.VersionTLS12}}
	start := time.Now()
	err := sender.Send(context.Background(), biz.AccountEmailMessage{To: "to@example.com", Subject: "subject", Body: "body"})
	if err == nil || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("stalled SMTP = %v after %v", err, time.Since(start))
	}
	_ = listener.Close()
	<-done
}

func TestSMTPCancellationInterruptsStalledConnection(t *testing.T) {
	listener, roots := newTestTLSListener(t)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer func() { _ = conn.Close() }()
			time.Sleep(time.Second)
		}
	}()
	host, port := splitListener(t, listener)
	sender := &smtpAccountMailSender{host: host, port: port, username: "sender@example.com", password: "secret", from: "sender@example.com", timeout: time.Second, tlsConfig: &tls.Config{ServerName: "localhost", RootCAs: roots, MinVersion: tls.VersionTLS12}}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	err := sender.Send(ctx, biz.AccountEmailMessage{To: "to@example.com", Subject: "subject", Body: "body"})
	if err == nil || time.Since(start) > 500*time.Millisecond {
		t.Fatalf("canceled SMTP = %v after %v", err, time.Since(start))
	}
}

func TestSMTPRequiresPlainAuthCapability(t *testing.T) {
	listener, roots := newTestTLSListener(t)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		reader := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 fixture\r\n"))
		_, _ = reader.ReadString('\n')
		_, _ = conn.Write([]byte("250-fixture\r\n250 SIZE 1024\r\n"))
	}()
	host, port := splitListener(t, listener)
	sender := &smtpAccountMailSender{host: host, port: port, username: "sender@example.com", password: "secret", from: "sender@example.com", timeout: time.Second, tlsConfig: &tls.Config{ServerName: "localhost", RootCAs: roots, MinVersion: tls.VersionTLS12}}
	err := sender.Send(context.Background(), biz.AccountEmailMessage{To: "to@example.com", Subject: "subject", Body: "body"})
	_ = listener.Close()
	if err == nil || !strings.Contains(err.Error(), "plain auth unavailable") {
		t.Fatalf("AUTH guard = %v", err)
	}
}

func splitListener(t *testing.T, listener net.Listener) (string, int) {
	t.Helper()
	host, rawPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	return host, port
}

func newTestTLSListener(t *testing.T) (net.Listener, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certPEM)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener, roots
}
