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
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserRepository is a user repo.
type UserRepository interface {
	FindByEmail(context.Context, string) (*User, error)
	FindByID(context.Context, uuid.UUID) (*User, error)
	Create(context.Context, *User) (*User, error)
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

// CreateAuthor creates an author account (seed path only). The password is
// hashed before it reaches the repo.
func (uc *UserUsecase) CreateAuthor(ctx context.Context, email, password, displayName string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !emailPattern.MatchString(email) {
		return nil, ErrUserInvalidArgument
	}
	if len(password) < 8 {
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
		Role:         UserRoleAdmin,
	})
}

// ByID returns the account with the given id.
func (uc *UserUsecase) ByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return uc.repo.FindByID(ctx, id)
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
