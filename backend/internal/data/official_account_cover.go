package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"google.golang.org/protobuf/proto"
)

const (
	maxOfficialAccountCoverSize = 64 << 10
	setupCoverPlaceholder       = "setup-only-placeholder"
)

// UploadOfficialAccountCover uploads one JPEG as a permanent WeChat thumb
// material. It clones the bootstrap config so setup-only defaults never alter
// the application's live configuration.
func UploadOfficialAccountCover(ctx context.Context, bc *conf.Bootstrap, image []byte, siteURL string) (string, error) {
	publisher, err := officialAccountCoverPublisher(bc, siteURL)
	if err != nil {
		return "", err
	}
	return publisher.uploadPermanentCover(ctx, image)
}

func officialAccountCoverPublisher(bc *conf.Bootstrap, siteURL string) (*officialAccountPublisher, error) {
	if bc == nil {
		return nil, errors.New("official account bootstrap configuration is required")
	}
	cloned, ok := proto.Clone(bc).(*conf.Bootstrap)
	if !ok || cloned == nil {
		return nil, errors.New("official account bootstrap configuration is invalid")
	}
	cfg := cloned.GetOfficialAccount()
	if cfg == nil {
		cfg = &conf.OfficialAccount{}
		cloned.OfficialAccount = cfg
	}
	cfg.Enabled = true
	cfg.SiteUrl = strings.TrimSpace(siteURL)
	if strings.TrimSpace(cfg.GetDefaultThumbMediaId()) == "" {
		// NewOfficialAccountPublisher requires a cover for article publishing.
		// This value only passes constructor validation and is never submitted.
		cfg.DefaultThumbMediaId = setupCoverPlaceholder
	}
	publisher, err := NewOfficialAccountPublisher(cloned)
	if err != nil {
		return nil, err
	}
	client, ok := publisher.(*officialAccountPublisher)
	if !ok || client == nil {
		return nil, errors.New("official account publisher is unavailable")
	}
	return client, nil
}

func (p *officialAccountPublisher) uploadPermanentCover(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 || len(image) > maxOfficialAccountCoverSize {
		if len(image) > maxOfficialAccountCoverSize {
			return "", localRemoteError("cover_too_large")
		}
		return "", localRemoteError("cover_invalid")
	}
	if len(image) < 3 || image[0] != 0xff || image[1] != 0xd8 || image[2] != 0xff {
		return "", localRemoteError("cover_invalid")
	}
	if _, err := jpeg.Decode(bytes.NewReader(image)); err != nil {
		return "", localRemoteError("cover_invalid")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "cover.jpg")
	if err != nil {
		return "", localRemoteError("cover_upload_prepare_failed")
	}
	if _, err := part.Write(image); err != nil {
		return "", localRemoteError("cover_upload_prepare_failed")
	}
	if err := writer.Close(); err != nil {
		return "", localRemoteError("cover_upload_prepare_failed")
	}

	token, err := p.token(ctx)
	if err != nil {
		return "", err
	}
	endpoint := p.baseURL + "/cgi-bin/material/add_material?access_token=" + url.QueryEscape(token) + "&type=thumb"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", localRemoteError("cover_upload_prepare_failed")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	var out struct {
		wechatEnvelope
		MediaID json.RawMessage `json:"media_id"`
	}
	if err := p.doJSON(req, &out, true); err != nil {
		return "", err
	}
	if out.ErrCode != 0 {
		return "", p.apiError(out.ErrCode)
	}
	mediaID := rawID(out.MediaID)
	if mediaID == "" {
		return "", unknownWriteError()
	}
	return mediaID, nil
}
