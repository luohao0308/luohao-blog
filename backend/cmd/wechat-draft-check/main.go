// wechat-draft-check creates one WeChat draft from an explicitly selected,
// publicly readable blog article. It never publishes or mass-sends.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
	"google.golang.org/protobuf/proto"
)

const (
	maxPublicArticleResponse = 2 << 20
	draftCheckTimeout        = time.Minute
)

var validArticleSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type publicArticle struct {
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Summary     string          `json:"summary"`
	ContentHTML string          `json:"content_html"`
	Status      int             `json:"status"`
	PublishedAt json.RawMessage `json:"published_at"`
}

type draftJournal struct {
	Key         string `json:"key"`
	ContentHash string `json:"content_hash"`
	Status      string `json:"status"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	SourceURL   string `json:"source_url"`
	MediaID     string `json:"media_id,omitempty"`
	ErrorCode   string `json:"error_code,omitempty"`
}

type draftCheckDependencies struct {
	validateOrigin func(context.Context, string) (*url.URL, *http.Client, error)
	newPublisher   func(*conf.Bootstrap) (biz.WechatPublisher, error)
}

func productionDraftDependencies() draftCheckDependencies {
	return draftCheckDependencies{
		validateOrigin: validatePublicSiteOrigin,
		newPublisher:   data.NewOfficialAccountPublisher,
	}
}

func validateDraftJournal(state draftJournal) bool {
	if state.Key == "" || state.ContentHash == "" || state.Slug == "" || state.Title == "" || state.SourceURL == "" {
		return false
	}
	switch state.Status {
	case "completed":
		return state.MediaID != ""
	case "failed", "uploading", "unknown":
		return true
	default:
		return false
	}
}

func readDraftJournal(path, key string) (draftJournal, bool, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return draftJournal{}, false, nil
	}
	if err != nil {
		return draftJournal{}, false, fmt.Errorf("draft_journal_read_failed")
	}
	var state draftJournal
	if json.Unmarshal(raw, &state) != nil || !validateDraftJournal(state) {
		return draftJournal{}, false, fmt.Errorf("invalid_draft_journal")
	}
	if state.Key != key {
		return draftJournal{}, false, fmt.Errorf("draft_journal_key_mismatch")
	}
	return state, true, nil
}

func writeDraftJournal(path string, state draftJournal) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("draft_journal_write_failed")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".draft-check-*")
	if err != nil {
		return fmt.Errorf("draft_journal_write_failed")
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("draft_journal_write_failed")
	}
	if err := json.NewEncoder(tmp).Encode(state); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("draft_journal_write_failed")
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("draft_journal_write_failed")
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("draft_journal_write_failed")
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("draft_journal_write_failed")
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("draft_journal_write_failed")
	}
	defer func() { _ = dir.Close() }()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("draft_journal_write_failed")
	}
	return nil
}

func fetchPublicArticle(ctx context.Context, client *http.Client, origin *url.URL, slug string) (publicArticle, error) {
	endpoint := *origin
	endpoint.Path = "/api/v1/articles/" + url.PathEscape(slug)
	endpoint.RawPath = ""
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return publicArticle{}, fmt.Errorf("article_request_invalid")
	}
	res, err := client.Do(req)
	if err != nil {
		return publicArticle{}, fmt.Errorf("article_fetch_failed")
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden {
			return publicArticle{}, fmt.Errorf("article_not_published_or_missing")
		}
		return publicArticle{}, fmt.Errorf("article_fetch_failed_status_%d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxPublicArticleResponse+1))
	if err != nil || len(body) == 0 || len(body) > maxPublicArticleResponse {
		return publicArticle{}, fmt.Errorf("article_response_invalid")
	}
	var article publicArticle
	if json.Unmarshal(body, &article) != nil {
		return publicArticle{}, fmt.Errorf("article_response_invalid")
	}
	if article.Status != 2 || len(bytes.TrimSpace(article.PublishedAt)) == 0 || bytes.Equal(bytes.TrimSpace(article.PublishedAt), []byte("null")) {
		return publicArticle{}, fmt.Errorf("article_not_published")
	}
	if article.Slug != slug || strings.TrimSpace(article.Title) == "" || strings.TrimSpace(article.ContentHTML) == "" {
		return publicArticle{}, fmt.Errorf("article_response_invalid")
	}
	return article, nil
}

func draftBootstrap(bc *conf.Bootstrap) (*conf.Bootstrap, error) {
	if bc == nil {
		return nil, fmt.Errorf("configuration_missing")
	}
	cloned, ok := proto.Clone(bc).(*conf.Bootstrap)
	if !ok || cloned == nil || cloned.GetOfficialAccount() == nil {
		return nil, fmt.Errorf("official_account_configuration_missing")
	}
	cloned.OfficialAccount.Enabled = true
	cloned.OfficialAccount.Mode = "draft"
	return cloned, nil
}

func executeDraftCheck(ctx context.Context, bc *conf.Bootstrap, slug, journalRoot string, deps draftCheckDependencies) (draftJournal, error) {
	if !validArticleSlug.MatchString(slug) {
		return draftJournal{}, fmt.Errorf("article_slug_required_or_invalid")
	}
	if bc == nil || bc.GetOfficialAccount() == nil {
		return draftJournal{}, fmt.Errorf("official_account_configuration_missing")
	}
	origin, client, err := deps.validateOrigin(ctx, bc.GetOfficialAccount().GetSiteUrl())
	if err != nil {
		return draftJournal{}, err
	}
	article, err := fetchPublicArticle(ctx, client, origin, slug)
	if err != nil {
		return draftJournal{}, err
	}
	sourceURL := origin.Scheme + "://" + origin.Host + "/posts/" + url.PathEscape(article.Slug)
	contentSum := sha256.Sum256([]byte(article.Slug + "\x00" + article.Title + "\x00" + article.Summary + "\x00" + article.ContentHTML + "\x00" + sourceURL))
	contentHash := hex.EncodeToString(contentSum[:])
	keySum := sha256.Sum256([]byte(bc.GetOfficialAccount().GetAppId() + "\x00" + contentHash))
	key := hex.EncodeToString(keySum[:])
	state := draftJournal{
		Key:         key,
		ContentHash: contentHash,
		Status:      "uploading",
		Slug:        article.Slug,
		Title:       article.Title,
		SourceURL:   sourceURL,
	}
	journalPath := filepath.Join(journalRoot, key+".json")
	if previous, found, err := readDraftJournal(journalPath, key); err != nil {
		return state, err
	} else if found {
		switch previous.Status {
		case "completed":
			return previous, nil
		case "uploading", "unknown":
			return previous, fmt.Errorf("previous_draft_result_unknown_check_wechat")
		}
	}
	if err := os.MkdirAll(journalRoot, 0700); err != nil {
		return state, fmt.Errorf("draft_journal_write_failed")
	}
	lockPath := journalPath + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return state, fmt.Errorf("draft_check_already_running_or_interrupted")
	}
	_ = lock.Close()
	defer func() { _ = os.Remove(lockPath) }()
	if previous, found, err := readDraftJournal(journalPath, key); err != nil {
		return state, err
	} else if found {
		switch previous.Status {
		case "completed":
			return previous, nil
		case "uploading", "unknown":
			return previous, fmt.Errorf("previous_draft_result_unknown_check_wechat")
		}
	}
	if err := writeDraftJournal(journalPath, state); err != nil {
		return state, err
	}
	cloned, err := draftBootstrap(bc)
	if err != nil {
		state.Status = "failed"
		state.ErrorCode = err.Error()
		_ = writeDraftJournal(journalPath, state)
		return state, err
	}
	publisher, err := deps.newPublisher(cloned)
	if err != nil || publisher == nil {
		state.Status = "failed"
		state.ErrorCode = "publisher_configuration_invalid"
		_ = writeDraftJournal(journalPath, state)
		return state, fmt.Errorf("publisher_configuration_invalid")
	}
	mediaID, err := publisher.CreateDraft(ctx, biz.WechatArticle{
		Title:       article.Title,
		Summary:     article.Summary,
		ContentHTML: article.ContentHTML,
		SourceURL:   sourceURL,
	})
	if err != nil {
		state.Status = "failed"
		state.ErrorCode = "remote_request_failed"
		var remote *biz.WechatRemoteError
		if stderrors.As(err, &remote) {
			state.ErrorCode = remote.Code
			if remote.Ambiguous {
				state.Status = "unknown"
			}
		}
		if writeErr := writeDraftJournal(journalPath, state); writeErr != nil {
			return state, writeErr
		}
		return state, fmt.Errorf("draft_create_failed_code_%s", state.ErrorCode)
	}
	if mediaID == "" {
		state.Status = "unknown"
		state.ErrorCode = "unknown"
		if err := writeDraftJournal(journalPath, state); err != nil {
			return state, err
		}
		return state, fmt.Errorf("draft_create_result_unknown")
	}
	state.Status = "completed"
	state.MediaID = mediaID
	if err := writeDraftJournal(journalPath, state); err != nil {
		return state, err
	}
	return state, nil
}

func validatePublicSiteOrigin(ctx context.Context, raw string) (*url.URL, *http.Client, error) {
	origin, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || origin.Host == "" || origin.User != nil || (origin.Scheme != "http" && origin.Scheme != "https") || (origin.Path != "" && origin.Path != "/") || origin.RawQuery != "" || origin.Fragment != "" {
		return nil, nil, fmt.Errorf("site_url_invalid")
	}
	origin.Path = ""
	lookup := func(ctx context.Context, host string) ([]net.IP, error) {
		return net.DefaultResolver.LookupIP(ctx, "ip", host)
	}
	if err := validatePublicHost(ctx, lookup, origin.Hostname()); err != nil {
		return nil, nil, err
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("site_url_invalid")
		}
		ips, err := lookup(ctx, host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("site_host_resolution_failed")
		}
		for _, ip := range ips {
			if !isPublicAddress(ip) {
				return nil, fmt.Errorf("site_host_not_public")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != origin.Scheme || !strings.EqualFold(req.URL.Host, origin.Host) {
			return fmt.Errorf("article_redirect_not_allowed")
		}
		return nil
	}
	return origin, client, nil
}

func validatePublicHost(ctx context.Context, lookup func(context.Context, string) ([]net.IP, error), host string) error {
	ips, err := lookup(ctx, host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("site_host_resolution_failed")
	}
	for _, ip := range ips {
		if !isPublicAddress(ip) {
			return fmt.Errorf("site_host_not_public")
		}
	}
	return nil
}

func isPublicAddress(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range blockedSitePrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var blockedSitePrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func main() {
	configDir := flag.String("conf", "./configs", "backend configuration directory")
	slug := flag.String("slug", "", "explicit slug of one published article")
	journalDir := flag.String("journal-dir", "./data/uploads/wechat-draft-check", "durable local draft journal directory")
	flag.Parse()
	log.SetDefault(log.NewLogger(slog.NewTextHandler(io.Discard, nil)))
	if !validArticleSlug.MatchString(*slug) {
		fmt.Fprintln(os.Stderr, "article_slug_required_or_invalid")
		os.Exit(1)
	}
	c := config.New(config.WithSource(file.NewSource(*configDir)))
	defer func() { _ = c.Close() }()
	if err := c.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "configuration_load_failed")
		os.Exit(1)
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		fmt.Fprintln(os.Stderr, "configuration_decode_failed")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), draftCheckTimeout)
	defer cancel()
	state, err := executeDraftCheck(ctx, &bc, *slug, *journalDir, productionDraftDependencies())
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{
		"status": state.Status, "media_id": state.MediaID, "slug": state.Slug,
		"title": state.Title, "source_url": state.SourceURL,
	})
}
