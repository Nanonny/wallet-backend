package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

// Integration test: skips unless TEST_POSTGRES_DSN is set, e.g.
//
//	TEST_POSTGRES_DSN=postgres://wallet:wallet@localhost:5432/wallet?sslmode=disable \
//	  go test ./internal/adapters/postgres/ -v
func TestPostgresUserRepositoryLifecycle(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN not set (skipping Postgres integration test)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := Open(ctx, dsn, 10*time.Second)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer Close(db)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewUserRepository(db)
	username := fmt.Sprintf("itest_%d", time.Now().UnixNano())

	user, err := repo.CreateUser(ctx, username, "abc12345")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(user.PasswordHash) < 60 {
		t.Fatal("password hash does not look like bcrypt output")
	}

	if _, err := repo.CreateUser(ctx, username, "abc12345"); err != domain.ErrUserExists {
		t.Fatalf("duplicate create err = %v, want ErrUserExists", err)
	}

	authed, err := repo.Authenticate(ctx, username, "abc12345")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if authed.ID != user.ID {
		t.Fatalf("id = %q, want %q", authed.ID, user.ID)
	}
	if authed.LastLoginAt == nil {
		t.Fatal("last_login_at should be set after a successful login")
	}

	if _, err := repo.Authenticate(ctx, username, "wrong9999"); err != domain.ErrInvalidCredentials {
		t.Fatalf("bad password err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := repo.Authenticate(ctx, "ghost_user_123", "abc12345"); err != domain.ErrInvalidCredentials {
		t.Fatalf("unknown user err = %v, want ErrInvalidCredentials", err)
	}
}
