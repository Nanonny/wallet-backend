package postgres

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/anon/wallet-devops-lab/internal/adapters/credential"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

// UserRepository implements application.UserRepository on PostgreSQL.
// All queries are parameterized; the password is never stored or logged in
// plaintext (bcrypt hash only).
type UserRepository struct {
	db *sql.DB
}

// NewUserRepository wraps a pooled database handle.
func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

type userRow struct {
	id           string
	username     string
	passwordHash string
	createdAt    time.Time
	lastLoginAt  sql.NullTime
}

func (r *UserRepository) CreateUser(ctx context.Context, username, password string) (domain.User, error) {
	hash, err := credential.HashPassword(password)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}
	username = application.NormalizeUsername(username)
	user := domain.User{
		ID:           "usr_" + r.newID(),
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, created_at)
		 VALUES ($1, $2, $3, $4)`,
		user.ID, user.Username, user.PasswordHash, user.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, domain.ErrUserExists
		}
		return domain.User{}, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at, last_login_at
		 FROM users WHERE username = $1`,
		application.NormalizeUsername(username),
	)

	var u userRow
	err := row.Scan(&u.id, &u.username, &u.passwordHash, &u.createdAt, &u.lastLoginAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Burn the same bcrypt time for unknown users (enumeration guard).
			credential.CompareHashAndPassword(credential.DummyHash, password)
			return domain.User{}, domain.ErrInvalidCredentials
		}
		return domain.User{}, fmt.Errorf("lookup user: %w", err)
	}

	if !credential.CompareHashAndPassword(u.passwordHash, password) {
		return domain.User{}, domain.ErrInvalidCredentials
	}

	if _, err := r.db.ExecContext(ctx,
		`UPDATE users SET last_login_at = $2 WHERE id = $1`,
		u.id, time.Now().UTC(),
	); err != nil {
		// Bookkeeping failure must not break a valid login.
	}

	user := domain.User{
		ID:           u.id,
		Username:     u.username,
		PasswordHash: u.passwordHash,
		CreatedAt:    u.createdAt,
	}
	if u.lastLoginAt.Valid {
		l := u.lastLoginAt.Time
		user.LastLoginAt = &l
	}
	return user, nil
}

func (r *UserRepository) newID() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return hex.EncodeToString(b[:])
}
