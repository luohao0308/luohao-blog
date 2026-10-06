package biz

import (
	"context"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
)

// fakeUserRepo keeps users in memory for usecase tests.
type fakeUserRepo struct {
	users map[string]*User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]*User{}}
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*User, error) {
	u, ok := f.users[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id uuid.UUID) (*User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (f *fakeUserRepo) FindByIDs(_ context.Context, ids []uuid.UUID) ([]*User, error) {
	want := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	var out []*User
	for _, u := range f.users {
		if _, ok := want[u.ID]; ok {
			out = append(out, u)
		}
	}
	return out, nil
}

func (f *fakeUserRepo) Create(_ context.Context, u *User) (*User, error) {
	if _, ok := f.users[u.Email]; ok {
		return nil, ErrUserEmailConflict
	}
	u.ID = uuid.Must(uuid.NewV7())
	f.users[u.Email] = u
	return u, nil
}

func (f *fakeUserRepo) UpdatePassword(_ context.Context, id uuid.UUID, passwordHash string) error {
	for _, u := range f.users {
		if u.ID == id {
			u.PasswordHash = passwordHash
			return nil
		}
	}
	return ErrUserNotFound
}

func (f *fakeUserRepo) UpdateProfile(_ context.Context, id uuid.UUID, displayName string) error {
	for _, u := range f.users {
		if u.ID == id {
			u.DisplayName = displayName
			return nil
		}
	}
	return ErrUserNotFound
}

func (f *fakeUserRepo) UpdateAvatar(_ context.Context, id uuid.UUID, avatarURL string) error {
	for _, u := range f.users {
		if u.ID == id {
			u.AvatarURL = avatarURL
			return nil
		}
	}
	return ErrUserNotFound
}

func (f *fakeUserRepo) FindByWechatOpenID(_ context.Context, openid string) (*User, error) {
	if openid == "" {
		return nil, ErrUserNotFound
	}
	for _, u := range f.users {
		if u.WechatOpenID == openid {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}

func (f *fakeUserRepo) BindWechat(_ context.Context, id uuid.UUID, openid string) error {
	for _, u := range f.users {
		if u.WechatOpenID == openid && u.ID != id {
			return ErrUserWechatConflict
		}
	}
	for _, u := range f.users {
		if u.ID == id {
			u.WechatOpenID = openid
			return nil
		}
	}
	return ErrUserNotFound
}

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash = %q, want argon2id PHC string", hash)
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Fatal("VerifyPassword(correct) = false, want true")
	}
	if VerifyPassword("wrong password", hash) {
		t.Fatal("VerifyPassword(wrong) = true, want false")
	}
	// Salts are random: the same password hashes differently every time.
	other, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword(second) error = %v", err)
	}
	if other == hash {
		t.Fatal("two hashes of one password are identical; salt is not random")
	}
	if VerifyPassword("anything", "$argon2id$garbage") {
		t.Fatal("VerifyPassword(malformed) = true, want false")
	}
}

func TestUserUsecaseCreateAuthorValidation(t *testing.T) {
	uc := NewUserUsecase(newFakeUserRepo())

	if _, err := uc.CreateAuthor(context.Background(), "not-an-email", "longenough1", "x"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("CreateAuthor(bad email) error = %v, want bad request", err)
	}
	if _, err := uc.CreateAuthor(context.Background(), "author@example.com", "short", "x"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("CreateAuthor(short password) error = %v, want bad request", err)
	}
	created, err := uc.CreateAuthor(context.Background(), "  Author@Example.COM ", "longenough1", "作者")
	if err != nil {
		t.Fatalf("CreateAuthor(ok) error = %v", err)
	}
	if created.Email != "author@example.com" {
		t.Fatalf("email = %q, want lowercased and trimmed", created.Email)
	}
	if created.Role != UserRoleAdmin {
		t.Fatalf("role = %d, want admin", created.Role)
	}
	if strings.Contains(created.PasswordHash, "longenough1") {
		t.Fatal("plaintext password leaked into the stored record")
	}
}

func TestUserUsecaseAuthenticate(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	uc := NewUserUsecase(repo)

	if _, err := uc.CreateAuthor(ctx, "author@example.com", "longenough1", "作者"); err != nil {
		t.Fatalf("CreateAuthor() error = %v", err)
	}

	got, err := uc.Authenticate(ctx, "Author@Example.COM", "longenough1")
	if err != nil {
		t.Fatalf("Authenticate(ok) error = %v", err)
	}
	if got.Email != "author@example.com" {
		t.Fatalf("Authenticate() = %+v, want the author", got)
	}

	// Unknown email and wrong password are indistinguishable.
	if _, err := uc.Authenticate(ctx, "ghost@example.com", "longenough1"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Authenticate(unknown) error = %v, want unauthorized", err)
	}
	if _, err := uc.Authenticate(ctx, "author@example.com", "wrongpassword"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("Authenticate(wrong password) error = %v, want unauthorized", err)
	}
}

func TestUpdatePassword(t *testing.T) {
	ctx := context.Background()
	repo := newFakeUserRepo()
	uc := NewUserUsecase(repo)
	u, err := uc.CreateAuthor(ctx, "admin@example.com", "old-password-1", "罗")
	if err != nil {
		t.Fatal(err)
	}

	if err := uc.UpdatePassword(ctx, u.ID, "wrong-old-password", "new-password-1"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("wrong old password = %v, want unauthorized", err)
	}
	if err := uc.UpdatePassword(ctx, u.ID, "old-password-1", "short"); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("short new password = %v, want bad request", err)
	}
	if err := uc.UpdatePassword(ctx, u.ID, "old-password-1", "new-password-1"); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	// The stored hash must verify against the new password and reject the old.
	if _, err := uc.Authenticate(ctx, "admin@example.com", "old-password-1"); !kratoserrors.IsUnauthorized(err) {
		t.Fatalf("old password after rotation = %v, want unauthorized", err)
	}
	if _, err := uc.Authenticate(ctx, "admin@example.com", "new-password-1"); err != nil {
		t.Fatalf("new password login: %v", err)
	}
}

func TestUserUsecaseUpdateProfile(t *testing.T) {
	ctx := context.Background()
	uc := NewUserUsecase(newFakeUserRepo())
	u, err := uc.CreateAuthor(ctx, "author@example.com", "longenough1", "作者")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := uc.UpdateProfile(ctx, u.ID, "   "); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("UpdateProfile(blank) error = %v, want bad request", err)
	}
	if _, err := uc.UpdateProfile(ctx, u.ID, strings.Repeat("名", MaxDisplayNameLen+1)); !kratoserrors.IsBadRequest(err) {
		t.Fatalf("UpdateProfile(too long) error = %v, want bad request", err)
	}
	// The bound counts runes: 32 Chinese characters fit just like 32 Latin ones.
	got, err := uc.UpdateProfile(ctx, u.ID, "  罗浩 LuoHao ")
	if err != nil {
		t.Fatalf("UpdateProfile(ok) error = %v", err)
	}
	if got.DisplayName != "罗浩 LuoHao" {
		t.Fatalf("UpdateProfile() name = %q, want trimmed value", got.DisplayName)
	}
}

func TestUserUsecaseUpdateAvatar(t *testing.T) {
	ctx := context.Background()
	uc := NewUserUsecase(newFakeUserRepo())
	u, err := uc.CreateAuthor(ctx, "author@example.com", "longenough1", "作者")
	if err != nil {
		t.Fatal(err)
	}

	got, err := uc.UpdateAvatar(ctx, u.ID, "/v1/assets/avatars/0123456789abcdef0123456789abcdef.png")
	if err != nil {
		t.Fatalf("UpdateAvatar() error = %v", err)
	}
	if got.AvatarURL == "" {
		t.Fatal("UpdateAvatar() did not record the avatar path")
	}
	if _, err := uc.UpdateAvatar(ctx, uuid.Must(uuid.NewV7()), "x"); !kratoserrors.IsNotFound(err) {
		t.Fatalf("UpdateAvatar(unknown account) error = %v, want not found", err)
	}
}
