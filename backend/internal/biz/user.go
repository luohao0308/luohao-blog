package biz

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
)

var (
	// ErrUserNotFound is returned when a user does not exist.
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	// ErrUserInvalidArgument is returned when a user request is invalid.
	ErrUserInvalidArgument = errors.BadRequest(v1.ErrorReason_USER_INVALID_ARGUMENT.String(), "invalid user argument")
	// ErrUserInvalidCredentials is returned when authentication fails. It is
	// deliberately indistinguishable between unknown email and wrong
	// password so the endpoint cannot be used to enumerate accounts.
	ErrUserInvalidCredentials = errors.Unauthorized(v1.ErrorReason_USER_INVALID_CREDENTIALS.String(), "invalid email or password")
	// ErrUserEmailConflict is returned when an email is already registered.
	ErrUserEmailConflict = errors.Conflict(v1.ErrorReason_USER_EMAIL_CONFLICT.String(), "email already registered")
)

// UserRole is the account role. Values match the api enum introduced with
// the auth contract in S2.
type UserRole int32

const (
	// UserRoleUnspecified is the zero value; it is never persisted.
	UserRoleUnspecified UserRole = 0
	// UserRoleAdmin can author and manage articles.
	UserRoleAdmin UserRole = 1
	// UserRoleReader is reserved for future reader-facing features.
	UserRoleReader UserRole = 2
)

// argon2id parameters: 64 MiB, 1 iteration, 4 threads, 32-byte key. Memory
// cost dominates GPU cracking resistance; these are OWASP-recommended
// baselines. The PHC string is self-contained, so parameters can be raised
// over time and old hashes still verify.
const (
	argon2Memory  = 64 * 1024
	argon2Time    = 1
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// emailPattern is a pragmatic RFC5322 subset; the real verification for
// author accounts is the seed command's operator, not a regex.
var emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// User is a User model.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	Role         UserRole
	// AvatarURL is the site-relative avatar path served by the GetAvatar
	// route (/v1/assets/avatars/<name>), or "" when no avatar was uploaded.
	AvatarURL string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserRepository is a user repo.
type UserRepository interface {
	FindByEmail(context.Context, string) (*User, error)
	FindByID(context.Context, uuid.UUID) (*User, error)
	// FindByIDs returns the accounts present in the input; missing ids are
	// simply absent from the result (comment avatar decoration tolerates
	// deleted authors).
	FindByIDs(context.Context, []uuid.UUID) ([]*User, error)
	Create(context.Context, *User) (*User, error)
	UpdatePassword(context.Context, uuid.UUID, string) error
	// UpdateProfile replaces the public display name.
	UpdateProfile(context.Context, uuid.UUID, string) error
	// UpdateAvatar replaces the site-relative avatar path ("" clears it).
	UpdateAvatar(context.Context, uuid.UUID, string) error
}

// HashPassword derives an argon2id PHC string for a plaintext password:
// $argon2id$v=19$m=<KiB>,t=<iters>,p=<threads>$<salt>$<hash> (raw std b64).
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

type phcParams struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
	Salt        []byte
	Key         []byte
}

// decodePHC parses a self-contained argon2id PHC string back into parameters.
func decodePHC(encoded string) (*phcParams, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, fmt.Errorf("unsupported hash format")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, fmt.Errorf("unsupported argon2 version")
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return nil, fmt.Errorf("invalid argon2 params")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, err
	}
	return &phcParams{MemoryKiB: m, Iterations: t, Parallelism: p, Salt: salt, Key: key}, nil
}

// VerifyPassword checks a plaintext password against a PHC-encoded hash,
// re-deriving with the parameters stored in the string.
func VerifyPassword(password, encoded string) bool {
	params, err := decodePHC(encoded)
	if err != nil {
		return false
	}
	key := argon2.IDKey([]byte(password), params.Salt, params.Iterations, params.MemoryKiB, params.Parallelism, uint32(len(params.Key)))
	return subtle.ConstantTimeCompare(key, params.Key) == 1
}

// UserUsecase is a User usecase.
type UserUsecase struct {
	repo UserRepository
}

// NewUserUsecase new a User usecase.
func NewUserUsecase(repo UserRepository) *UserUsecase {
	return &UserUsecase{repo: repo}
}

// CreateAuthor creates an ADMIN account; the common seed path.
func (uc *UserUsecase) CreateAuthor(ctx context.Context, email, password, displayName string) (*User, error) {
	return uc.CreateAccount(ctx, email, password, displayName, UserRoleAdmin)
}

// CreateAccount creates an account with the given role (seed path only).
// The password is hashed before it reaches the repo.
func (uc *UserUsecase) CreateAccount(ctx context.Context, email, password, displayName string, role UserRole) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailPattern.MatchString(email) {
		return nil, ErrUserInvalidArgument
	}
	if len(password) < 8 {
		return nil, ErrUserInvalidArgument
	}
	if role != UserRoleAdmin && role != UserRoleReader {
		return nil, ErrUserInvalidArgument
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return uc.repo.Create(ctx, &User{
		Email:        email,
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(displayName),
		Role:         role,
	})
}

// ByID returns the account with the given id.
func (uc *UserUsecase) ByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return uc.repo.FindByID(ctx, id)
}

// ByIDs returns the accounts found for the given ids, deduplicating the
// input and skipping empty requests entirely.
func (uc *UserUsecase) ByIDs(ctx context.Context, ids []uuid.UUID) ([]*User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return uc.repo.FindByIDs(ctx, unique)
}

// Authenticate verifies credentials and returns the account on success.
func (uc *UserUsecase) Authenticate(ctx context.Context, email, password string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.IsNotFound(err) {
			// Burn comparable time so unknown emails are not distinguishable
			// by response latency.
			_, _ = HashPassword(password)
			return nil, ErrUserInvalidCredentials
		}
		return nil, err
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return nil, ErrUserInvalidCredentials
	}
	return user, nil
}

// UpdatePassword rotates the calling account's password: the current
// password is verified first, the new one follows the creation rules, and
// the stored hash is replaced. The account identity comes from the verified
// access token, never from the payload.
func (uc *UserUsecase) UpdatePassword(ctx context.Context, id uuid.UUID, oldPassword, newPassword string) error {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !VerifyPassword(oldPassword, user.PasswordHash) {
		return ErrUserInvalidCredentials
	}
	if len(newPassword) < 8 {
		return ErrUserInvalidArgument
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return ErrUserInvalidArgument
	}
	return uc.repo.UpdatePassword(ctx, id, hash)
}

// UpdateProfile renames the calling account. The name is the comment/header
// identity, so it follows the same bounds as a comment nickname: 1-32
// characters after trimming.
func (uc *UserUsecase) UpdateProfile(ctx context.Context, id uuid.UUID, displayName string) (*User, error) {
	name, err := NormalizeDisplayName(displayName)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.UpdateProfile(ctx, id, name); err != nil {
		return nil, err
	}
	return uc.repo.FindByID(ctx, id)
}

// UpdateAvatar records the site-relative avatar path for the calling
// account. The path is produced by the avatar usecase; nothing here trusts
// it beyond storing it verbatim.
func (uc *UserUsecase) UpdateAvatar(ctx context.Context, id uuid.UUID, avatarURL string) (*User, error) {
	if err := uc.repo.UpdateAvatar(ctx, id, avatarURL); err != nil {
		return nil, err
	}
	return uc.repo.FindByID(ctx, id)
}

// MaxDisplayNameLen bounds the public display name, matching the comment
// nickname limit so identities stay consistent across surfaces.
const MaxDisplayNameLen = 32

// NormalizeDisplayName trims a display name and enforces the 1-32 character
// bound. The limit counts runes, not bytes, so Chinese names get the same
// room as Latin ones.
func NormalizeDisplayName(displayName string) (string, error) {
	name := strings.TrimSpace(displayName)
	if name == "" || utf8.RuneCountInString(name) > MaxDisplayNameLen {
		return "", ErrUserInvalidArgument
	}
	return name, nil
}
