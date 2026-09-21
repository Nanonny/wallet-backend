package application

import (
	"context"
	"regexp"
	"strings"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)

// NormalizeUsername lowercases and trims usernames so lookups are
// case-insensitive, matching how usernames are validated.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// ValidateUsername enforces the shared username policy.
func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return domain.ErrInvalidUsername
	}
	return nil
}

// ValidatePassword enforces the minimum strength for new credentials.
func ValidatePassword(password string) error {
	if len(password) < 8 || len(password) > 128 {
		return domain.ErrWeakPassword
	}
	hasLetter, hasNumber := false, false
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			hasLetter = true
		case r >= '0' && r <= '9':
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return domain.ErrWeakPassword
	}
	return nil
}

// UserService applies the account domain rules (username/password policy)
// and delegates credential storage to the UserRepository port, so the
// application layer stays storage-agnostic.
type UserService struct {
	users UserRepository
}

func NewUserService(users UserRepository) *UserService {
	return &UserService{users: users}
}

// Register creates a new account after validating the credentials.
func (s *UserService) Register(ctx context.Context, username, password string) (domain.User, error) {
	username = NormalizeUsername(username)
	if err := ValidateUsername(username); err != nil {
		return domain.User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return domain.User{}, err
	}
	return s.users.CreateUser(ctx, username, password)
}

// Login verifies credentials via the repository port.
func (s *UserService) Login(ctx context.Context, username, password string) (domain.User, error) {
	return s.users.Authenticate(ctx, NormalizeUsername(username), password)
}
