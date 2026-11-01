package service_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/service"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type chatRepo struct{ biz.ArticleRepo }

func (chatRepo) FindBySlug(_ context.Context, slug string) (*biz.Article, error) {
	status := biz.ArticleStatusPublished
	if slug == "draft" {
		status = biz.ArticleStatusDraft
	}
	if slug == "missing" {
		return nil, biz.ErrArticleNotFound
	}
	return &biz.Article{Slug: slug, Title: "测试", Status: status, ContentMD: "## 恢复\n\nrecovery 使用日志恢复。"}, nil
}

type chatModel struct{}

func (chatModel) Generate(context.Context, string, string) (string, error) { return "答案", nil }
func TestChatScopedHTTPContract(t *testing.T) {
	srv := khttp.NewServer(khttp.ResponseEncoder(func(w http.ResponseWriter, _ *http.Request, value any) error {
		data, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(value.(proto.Message))
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(data)
		return err
	}))
	v1.RegisterChatServiceHTTPServer(srv, service.NewChatService(biz.NewChatUsecase(biz.NewArticleUsecase(chatRepo{}, nil), chatModel{}, nil)))
	ts := httptest.NewServer(srv)
	defer ts.Close()
	for _, tc := range []struct {
		body string
		code int
		want string
	}{
		{`{"query":"recovery","article_slug":"published"}`, 200, `"section_title":"恢复"`},
		{`{"query":"recovery","article_slug":"draft"}`, 404, `ARTICLE_NOT_FOUND`},
		{`{"query":"recovery","article_slug":"missing"}`, 404, `ARTICLE_NOT_FOUND`},
		{`{"query":"recovery"}`, 200, `"article_slug":""`},
	} {
		res, err := http.Post(ts.URL+"/v1/chat", "application/json", strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != tc.code || !strings.Contains(string(data), tc.want) {
			t.Fatalf("%s => %d %s", tc.body, res.StatusCode, data)
		}
		if tc.code == 200 && strings.Contains(tc.body, "article_slug") {
			for _, field := range []string{`"anchor":`, `"excerpt":`, `"article_slug":"published"`} {
				if !strings.Contains(string(data), field) {
					t.Fatalf("missing %s in %s", field, data)
				}
			}
		}
	}
}
