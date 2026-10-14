package data

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/accountemailtoken"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/user"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

func TestAccountEmailTokenRepoMySQL(t *testing.T) {
	dsn := os.Getenv("BLOG_ACCOUNT_EMAIL_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BLOG_ACCOUNT_EMAIL_MYSQL_DSN is not set")
	}
	cfg, err := gomysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:13316" || !regexp.MustCompile(`^website_completion_[0-9a-f_]+$`).MatchString(cfg.DBName) {
		t.Fatal("refusing non-fixture MySQL target")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.MySQL, db)))
	defer func() { _ = client.Close() }()
	ctx := context.Background()
	data := &Data{db: client}
	repo := NewAccountEmailTokenRepo(data)
	uid := uuid.Must(uuid.NewV7())
	email := "fixture-" + uid.String() + "@example.com"
	hash, _ := biz.HashPassword("longenough1")
	_, err = client.User.Create().SetID(uid).SetEmail(email).SetPasswordHash(hash).SetDisplayName("Fixture").SetRole(biz.UserRoleReader).SetEmailVerified(true).SetCredentialVersion(0).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = client.AccountEmailToken.Delete().Where(accountemailtoken.UserIDEQ(uid)).Exec(ctx)
		_, _ = client.User.Delete().Where(user.IDEQ(uid)).Exec(ctx)
	}()
	first, err := repo.Issue(ctx, biz.EmailPurposeVerify, uid, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Issue(ctx, biz.EmailPurposeVerify, uid, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if current, _ := repo.IsCurrent(ctx, biz.EmailPurposeVerify, uid, 0, first); current {
		t.Fatal("old same-second token remains current")
	}
	if _, ok, err := repo.ConfirmVerification(ctx, second); err != nil || !ok {
		t.Fatalf("already-verified confirm = %v/%v", ok, err)
	}
	_, _ = repo.Issue(ctx, biz.EmailPurposeVerify, uid, 0, time.Hour)
	reset, err := repo.Issue(ctx, biz.EmailPurposeReset, uid, 0, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	newHash, _ := biz.HashPassword("newpassword1")
	_, version, ok, err := repo.ConfirmPasswordReset(ctx, reset, newHash)
	if err != nil || !ok || version != 1 {
		t.Fatalf("reset = %d/%v/%v", version, ok, err)
	}
	if count, _ := client.AccountEmailToken.Query().Where(accountemailtoken.UserIDEQ(uid)).Count(ctx); count != 0 {
		t.Fatalf("tokens remain = %d", count)
	}
	row, err := client.User.Query().Where(user.IDEQ(uid)).Only(ctx)
	if err != nil || !row.EmailVerified || row.CredentialVersion != 1 {
		t.Fatalf("user flags/version invalid")
	}
	if _, err := repo.Issue(ctx, biz.EmailPurposeVerify, uid, 0, time.Hour); err == nil {
		t.Fatal("old version issued after reset")
	}

	uid2 := uuid.Must(uuid.NewV7())
	email2 := "fixture-" + uid2.String() + "@example.com"
	_, err = client.User.Create().SetID(uid2).SetEmail(email2).SetPasswordHash(hash).SetDisplayName("Fixture2").SetRole(biz.UserRoleReader).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = client.AccountEmailToken.Delete().Where(accountemailtoken.UserIDEQ(uid2)).Exec(ctx)
		_, _ = client.User.Delete().Where(user.IDEQ(uid2)).Exec(ctx)
	}()
	userRepo := NewUserRepo(data)
	if _, updated, err := userRepo.UpdatePasswordState(ctx, uid2, 0, newHash, false); err != nil || !updated {
		t.Fatalf("ordinary password update = %v/%v", updated, err)
	}
	row2, _ := client.User.Query().Where(user.IDEQ(uid2)).Only(ctx)
	if row2.EmailVerified || row2.CredentialVersion != 1 {
		t.Fatal("ordinary password change altered verification/version incorrectly")
	}
}
