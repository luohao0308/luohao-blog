package biz

import (
	"context"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

type fakeCollectionRepo struct {
	items map[uuid.UUID]map[string]*Collection
}

func newFakeCollectionRepo() *fakeCollectionRepo {
	return &fakeCollectionRepo{items: map[uuid.UUID]map[string]*Collection{}}
}
func (r *fakeCollectionRepo) Save(_ context.Context, userID uuid.UUID, slug string) (*Collection, error) {
	if slug == "draft" {
		return nil, ErrArticleNotFound
	}
	if r.items[userID] == nil {
		r.items[userID] = map[string]*Collection{}
	}
	if existing := r.items[userID][slug]; existing != nil {
		return existing, nil
	}
	c := &Collection{Article: &Article{Slug: slug, Status: ArticleStatusPublished}, SavedAt: time.Now()}
	r.items[userID][slug] = c
	return c, nil
}
func (r *fakeCollectionRepo) List(_ context.Context, userID uuid.UUID, _, _ int) ([]*Collection, error) {
	out := []*Collection{}
	for _, c := range r.items[userID] {
		out = append(out, c)
	}
	return out, nil
}

type collectionListSpy struct {
	*fakeCollectionRepo
	limit, offset int
}

func (s *collectionListSpy) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Collection, error) {
	s.limit, s.offset = limit, offset
	return s.fakeCollectionRepo.List(ctx, userID, limit, offset)
}
func (r *fakeCollectionRepo) Delete(_ context.Context, userID uuid.UUID, slug string) error {
	if r.items[userID] != nil {
		delete(r.items[userID], slug)
	}
	return nil
}

func TestCollectionUsecaseIsolationIdempotencyAndPublicBoundary(t *testing.T) {
	repo := newFakeCollectionRepo()
	uc := NewCollectionUsecase(repo)
	a, b := uuid.New(), uuid.New()
	first, err := uc.Save(context.Background(), a, "published")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Save(context.Background(), a, "published")
	if err != nil || !second.SavedAt.Equal(first.SavedAt) {
		t.Fatalf("duplicate=%+v %v", second, err)
	}
	if _, err = uc.Save(context.Background(), a, "draft"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("draft save=%v", err)
	}
	if list, _ := uc.List(context.Background(), b, 20, 0); len(list) != 0 {
		t.Fatal("cross-account leak")
	}
	if err = uc.Delete(context.Background(), b, "published"); err != nil {
		t.Fatal(err)
	}
	if list, _ := uc.List(context.Background(), a, 20, 0); len(list) != 1 {
		t.Fatal("other user delete removed item")
	}
	if err = uc.Delete(context.Background(), a, "published"); err != nil {
		t.Fatal(err)
	}
	if err = uc.Delete(context.Background(), a, "published"); err != nil {
		t.Fatal("delete not idempotent")
	}
}

func TestCollectionUsecaseValidatesIdentityAndSlug(t *testing.T) {
	uc := NewCollectionUsecase(newFakeCollectionRepo())
	if _, err := uc.Save(context.Background(), uuid.Nil, "ok"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("nil user=%v", err)
	}
	if _, err := uc.Save(context.Background(), uuid.New(), "BAD SLUG"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("bad slug=%v", err)
	}
}

func TestCollectionUsecasePassesLimitOffsetInOrder(t *testing.T) {
	spy := &collectionListSpy{fakeCollectionRepo: newFakeCollectionRepo()}
	if _, err := NewCollectionUsecase(spy).List(context.Background(), uuid.New(), 20, 40); err != nil {
		t.Fatal(err)
	}
	if spy.limit != 20 || spy.offset != 40 {
		t.Fatalf("limit/offset=%d/%d", spy.limit, spy.offset)
	}
}
