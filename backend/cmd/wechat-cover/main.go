// wechat-cover uploads only a default permanent cover. It never creates drafts,
// publishes, sends messages, connects to the database or starts the sync worker.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/conf"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
)

type coverState struct {
	Key       string `json:"key"`
	Status    string `json:"status"`
	MediaID   string `json:"media_id,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

func writeState(path string, state coverState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cover-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := json.NewEncoder(tmp).Encode(state); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	// The rename must survive a host crash before a remote write can begin.
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return dir.Sync()
}

func validCoverJournal(state coverState) bool {
	switch state.Status {
	case "uploaded":
		return state.MediaID != ""
	case "failed", "uploading", "unknown":
		return true
	default:
		return false
	}
}

func writeSetup(configDir, site, id, appID string) error {
	// Only non-secret generated fields. A later-sorting override leaves the
	// user's JWT, LLM and WeChat secret file untouched and wins over empty fields.
	body, err := json.Marshal(map[string]any{"official_account": map[string]string{"site_url": site, "default_thumb_media_id": id, "app_id": appID}})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, "zz-official-account-setup.local.yaml"), body, 0600)
}

func setup(ctx context.Context, bc *conf.Bootstrap, cover []byte, site, statePath, configDir string, upload func(context.Context, *conf.Bootstrap, []byte, string) (string, error)) (coverState, error) {
	if bc.GetOfficialAccount().GetAppId() == "" {
		return coverState{}, fmt.Errorf("official_account_app_id_missing")
	}
	if bc.GetOfficialAccount().GetAppSecret() == "" {
		return coverState{}, fmt.Errorf("official_account_app_secret_missing")
	}
	key := sha256.Sum256(append([]byte(bc.GetOfficialAccount().GetAppId()+"\x00"), cover...))
	state := coverState{Key: hex.EncodeToString(key[:]), Status: "uploading"}
	raw, err := os.ReadFile(statePath)
	if err == nil {
		var previous coverState
		if json.Unmarshal(raw, &previous) != nil {
			return state, fmt.Errorf("invalid_upload_journal")
		}
		if !validCoverJournal(previous) {
			return state, fmt.Errorf("invalid_upload_journal")
		}
		if previous.Key != state.Key {
			return state, fmt.Errorf("journal_account_or_cover_mismatch")
		}
		if previous.Status == "uploaded" && previous.MediaID != "" {
			if err := writeSetup(configDir, site, previous.MediaID, bc.GetOfficialAccount().GetAppId()); err != nil {
				return previous, fmt.Errorf("setup_write_failed")
			}
			return previous, nil
		}
		if previous.Status == "uploading" || previous.Status == "unknown" {
			return previous, fmt.Errorf("previous_upload_unknown_check_wechat")
		}
	} else if !os.IsNotExist(err) {
		return state, fmt.Errorf("upload_journal_read_failed")
	}
	// O_EXCL lock prevents two independent invocations racing past the journal.
	if err := os.MkdirAll(filepath.Dir(statePath), 0700); err != nil {
		return state, fmt.Errorf("upload_journal_write_failed")
	}
	lock, err := os.OpenFile(statePath+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return state, fmt.Errorf("setup_already_running_or_interrupted")
	}
	_ = lock.Close()
	defer func() { _ = os.Remove(statePath + ".lock") }()
	// Re-read after locking: another invocation may have completed while this
	// command was acquiring the lock. Never overwrite an uploaded/unknown row.
	if raw, err := os.ReadFile(statePath); err == nil {
		var previous coverState
		if json.Unmarshal(raw, &previous) != nil {
			return state, fmt.Errorf("invalid_upload_journal")
		}
		if !validCoverJournal(previous) {
			return state, fmt.Errorf("invalid_upload_journal")
		}
		if previous.Key != state.Key {
			return state, fmt.Errorf("journal_account_or_cover_mismatch")
		}
		if previous.Status == "uploaded" && previous.MediaID != "" {
			if err := writeSetup(configDir, site, previous.MediaID, bc.GetOfficialAccount().GetAppId()); err != nil {
				return previous, fmt.Errorf("setup_write_failed")
			}
			return previous, nil
		}
		if previous.Status == "uploading" || previous.Status == "unknown" {
			return previous, fmt.Errorf("previous_upload_unknown_check_wechat")
		}
	}
	if _, err := os.Stat(statePath); err == nil {
		if _, readErr := os.ReadFile(statePath); readErr != nil {
			return state, fmt.Errorf("upload_journal_read_failed")
		}
	} else if !os.IsNotExist(err) {
		return state, fmt.Errorf("upload_journal_read_failed")
	}
	if err := writeState(statePath, state); err != nil {
		return state, fmt.Errorf("upload_journal_write_failed")
	}
	id, err := upload(ctx, bc, cover, site)
	if err != nil {
		state.Status = "failed"
		state.ErrorCode = "cover_setup_failed"
		var remote *biz.WechatRemoteError
		if stderrors.As(err, &remote) {
			state.ErrorCode = remote.Code
			if remote.Ambiguous {
				state.Status = "unknown"
			}
		}
		if err := writeState(statePath, state); err != nil {
			return state, fmt.Errorf("upload_result_write_failed")
		}
		return state, fmt.Errorf("cover_upload_failed_code_%s", state.ErrorCode)
	}
	if id == "" {
		state.Status = "unknown"
		_ = writeState(statePath, state)
		return state, fmt.Errorf("empty_media_id")
	}
	state.Status = "uploaded"
	state.MediaID = id
	if err := writeState(statePath, state); err != nil {
		return state, fmt.Errorf("upload_result_write_failed")
	}
	if err := writeSetup(configDir, site, id, bc.GetOfficialAccount().GetAppId()); err != nil {
		return state, fmt.Errorf("setup_write_failed")
	}
	return state, nil
}

func main() {
	configDir := flag.String("conf", "./configs", "backend configuration directory")
	site := flag.String("site-url", "", "verified public blog origin")
	coverPath := flag.String("cover", "./internal/data/assets/official-cover.jpg", "default JPEG cover")
	statePath := flag.String("state", "./data/uploads/wechat-cover-setup.json", "durable local upload journal")
	checkOnly := flag.Bool("check-config", false, "print only credential presence, never values")
	flag.Parse()
	log.SetDefault(log.NewLogger(slog.NewTextHandler(io.Discard, nil)))
	if *site == "" && !*checkOnly {
		fmt.Fprintln(os.Stderr, "site_url_missing")
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
	if *checkOnly {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]bool{"official_account_app_id_present": bc.GetOfficialAccount().GetAppId() != "", "official_account_app_secret_present": bc.GetOfficialAccount().GetAppSecret() != ""})
		return
	}
	cover, err := os.ReadFile(*coverPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cover_file_read_failed")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	state, err := setup(ctx, &bc, cover, *site, *statePath, *configDir, data.UploadOfficialAccountCover)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// No token, AppSecret, response body or authenticated URL is emitted.
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"status": state.Status, "media_id": state.MediaID, "site_url": *site})
}
