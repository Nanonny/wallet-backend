package memory

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/anon/wallet-devops-lab/internal/adapters/credential"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

// UserRepository keeps authenticated accounts in process memory. It is used
// by `make run-memory` (zero-dependency demo) and by unit tests. It shares
// the same bcrypt hashing as the Postgres adapter via credential.Hash.
type UserRepository struct {
	mu    sync.RWMutex
	users map[string]domain.User // keyed by normalized username
}

func NewUserRepository() *UserRepository {
	return &UserRepository{users: map[string]domain.User{}}
}

func (r *UserRepository) CreateUser(ctx context.Context, username, password string) (domain.User, error) {
	hash, err := credential.HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.users[username]; exists {
		return domain.User{}, domain.ErrUserExists
	}
	user := domain.User{
		ID:           "usr_" + newMemoryID(),
		Username:     application.NormalizeUsername(username),
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	r.users[user.Username] = user
	return user, nil
}

func newMemoryID() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return hex.EncodeToString(b[:])
}

func (r *UserRepository) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	username = application.NormalizeUsername(username)

	r.mu.RLock()
	user, exists := r.users[username]
	r.mu.RUnlock()

	if !exists {
		// Burn the same bcrypt time for unknown users (user-enumeration guard).
		credential.CompareHashAndPassword(credential.DummyHash, password)
		return domain.User{}, domain.ErrInvalidCredentials
	}
	if !credential.CompareHashAndPassword(user.PasswordHash, password) {
		return domain.User{}, domain.ErrInvalidCredentials
	}

	r.mu.Lock()
	now := time.Now().UTC()
	user.LastLoginAt = &now
	r.users[username] = user
	r.mu.Unlock()

	return user, nil
}
