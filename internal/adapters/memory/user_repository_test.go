package memory

import (
	"context"
	"testing"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

func TestMemoryUserRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewUserRepository()

	user, err := repo.CreateUser(ctx, "Demo", "abc12345")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if user.Username != "demo" {
		t.Fatalf("username = %q, want normalized demo", user.Username)
	}
	if user.PasswordHash == "abc12345" {
		t.Fatal("password hash is stored as plaintext")
	}

	if _, err := repo.CreateUser(ctx, "demo", "abc12345"); err != domain.ErrUserExists {
		t.Fatalf("duplicate create err = %v, want ErrUserExists", err)
	}

	authed, err := repo.Authenticate(ctx, "demo", "abc12345")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if authed.ID != user.ID {
		t.Fatalf("authenticated id = %q, want %q", authed.ID, user.ID)
	}
	if authed.LastLoginAt == nil {
		t.Fatal("last_login_at should be set after a successful login")
	}

	if _, err := repo.Authenticate(ctx, "demo", "wrong9999"); err != domain.ErrInvalidCredentials {
		t.Fatalf("bad password err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := repo.Authenticate(ctx, "ghost", "abc12345"); err != domain.ErrInvalidCredentials {
		t.Fatalf("unknown user err = %v, want ErrInvalidCredentials", err)
	}
}
