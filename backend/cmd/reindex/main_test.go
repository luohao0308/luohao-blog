package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
)

func TestReindexArticlesPagesPast1000(t *testing.T) {
	const total = 2005
	indexed := map[string]bool{}
	list := func(_ context.Context, opts ...biz.ListOption) ([]*biz.Article, error) {
		var o biz.ListOptions
		for _, opt := range opts {
			opt(&o)
		}
		if !o.Public || o.Limit != 1000 {
			t.Fatalf("options: %+v", o)
		}
		var out []*biz.Article
		for i := o.Offset; i < min(total, o.Offset+o.Limit); i++ {
			out = append(out, &biz.Article{Slug: fmt.Sprintf("a-%d", i)})
		}
		return out, nil
	}
	count, err := reindexArticles(context.Background(), list, func(_ context.Context, a *biz.Article) error {
		if indexed[a.Slug] {
			t.Fatalf("duplicate %s", a.Slug)
		}
		indexed[a.Slug] = true
		return nil
	})
	if err != nil || count != total || len(indexed) != total {
		t.Fatalf("count=%d indexed=%d err=%v", count, len(indexed), err)
	}
}

func TestReindexArticlesStopsOnErrors(t *testing.T) {
	want := fmt.Errorf("storage unavailable")
	list := func(context.Context, ...biz.ListOption) ([]*biz.Article, error) { return nil, want }
	if _, err := reindexArticles(context.Background(), list, nil); err == nil {
		t.Fatal("list error swallowed")
	}
	list = func(context.Context, ...biz.ListOption) ([]*biz.Article, error) {
		return []*biz.Article{{Slug: "one"}}, nil
	}
	if count, err := reindexArticles(context.Background(), list, func(context.Context, *biz.Article) error { return want }); err == nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
