package data

import (
	"context"
	stdsql "database/sql"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	_ "modernc.org/sqlite"
)

// newTestUserRepo opens an in-memory SQLite database with the schema applied,
// so repo tests exercise real ent queries at the storage boundary. Test
// databases are created through ent directly; the versioned MySQL migrations
// are exercised by the boot-time rehearsal instead.
func newTestUserRepo(t *testing.T) (biz.UserRepository, *ent.Client) {
	t.Helper()
	db, err := stdsql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() {
		_ = client.Close()
	})
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return NewUserRepo(&Data{db: client}), client
}

func TestUserRepoCreateAndFind(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestUserRepo(t)

	created, err := repo.Create(ctx, &biz.User{
		Email:        "author@example.com",
		PasswordHash: "$argon2id$fake",
		DisplayName:  "作者",
		Role:         biz.UserRoleAdmin,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID.String() == "" {
		t.Fatal("Create() did not assign an id")
	}

	got, err := repo.FindByEmail(ctx, "author@example.com")
	if err != nil {
		t.Fatalf("FindByEmail() error = %v", err)
	}
	if got.DisplayName != "作者" || got.Role != biz.UserRoleAdmin {
		t.Fatalf("FindByEmail() = %+v, want created user", got)
	}
}

func TestUserRepoEmailConflict(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestUserRepo(t)

	first := &biz.User{Email: "dup@example.com", PasswordHash: "h", Role: biz.UserRoleAdmin}
	if _, err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if _, err := repo.Create(ctx, &biz.User{Email: "dup@example.com", PasswordHash: "h", Role: biz.UserRoleAdmin}); !kratoserrors.IsConflict(err) {
		t.Fatalf("Create(duplicate) error = %v, want email conflict", err)
	}
}

func TestUserRepoNotFound(t *testing.T) {
	ctx := context.Background()
	repo, _ := newTestUserRepo(t)

	if _, err := repo.FindByEmail(ctx, "missing@example.com"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("FindByEmail(missing) error = %v, want not found", err)
	}
}
