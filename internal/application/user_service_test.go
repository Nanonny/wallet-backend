package application

import (
	"context"
	"errors"
	"testing"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

type fakeUserRepo struct {
	users       map[string]domain.User
	createErr   error
	lastCreated domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: map[string]domain.User{}}
}

func (f *fakeUserRepo) CreateUser(ctx context.Context, username, password string) (domain.User, error) {
	if f.createErr != nil {
		return domain.User{}, f.createErr
	}
	if _, exists := f.users[username]; exists {
		return domain.User{}, domain.ErrUserExists
	}
	user := domain.User{ID: "usr-test", Username: username, PasswordHash: "hash:" + password}
	f.users[username] = user
	f.lastCreated = user
	return user, nil
}

func (f *fakeUserRepo) Authenticate(ctx context.Context, username, password string) (domain.User, error) {
	user, ok := f.users[username]
	if !ok || user.PasswordHash != "hash:"+password {
		return domain.User{}, domain.ErrInvalidCredentials
	}
	return user, nil
}

func TestRegisterValidAccount(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)

	user, err := svc.Register(context.Background(), "Demo_User", "abc12345")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if user.Username != "demo_user" {
		t.Fatalf("username = %q, want normalized demo_user", user.Username)
	}
	if len(repo.users) != 1 {
		t.Fatalf("stored users = %d, want 1", len(repo.users))
	}
}

func TestRegisterRejectsWeakPasswords(t *testing.T) {
	svc := NewUserService(newFakeUserRepo())

	cases := []string{
		"short1",                 // too short
		"onlyletters",            // no number
		"12345678",               // no letter
		"abcdefgh",               // no number
		strings_repeat("a1", 65), // 130 chars: over the 128 max
	}
	for _, password := range cases {
		if _, err := svc.Register(context.Background(), "user1", password); !errors.Is(err, domain.ErrWeakPassword) {
			t.Fatalf("password %q: err = %v, want %v", password, err, domain.ErrWeakPassword)
		}
	}
}

func TestRegisterRejectsInvalidUsernames(t *testing.T) {
	svc := NewUserService(newFakeUserRepo())

	cases := []string{"ab", "has space", "has.dot", "user-name", strings_repeat("a", 33)}
	for _, username := range cases {
		if _, err := svc.Register(context.Background(), username, "abc12345"); !errors.Is(err, domain.ErrInvalidUsername) {
			t.Fatalf("username %q: err = %v, want %v", username, err, domain.ErrInvalidUsername)
		}
	}
}

func TestRegisterDuplicateUsername(t *testing.T) {
	svc := NewUserService(newFakeUserRepo())

	if _, err := svc.Register(context.Background(), "demo", "abc12345"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := svc.Register(context.Background(), "DEMO", "abc12345"); !errors.Is(err, domain.ErrUserExists) {
		t.Fatalf("duplicate register err = %v, want %v", err, domain.ErrUserExists)
	}
}

func TestLoginSuccessAndFailure(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewUserService(repo)
	if _, err := svc.Register(context.Background(), "demo", "abc12345"); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := svc.Login(context.Background(), "demo", "abc12345"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.Login(context.Background(), "demo", "wrong9999"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("bad login err = %v, want %v", err, domain.ErrInvalidCredentials)
	}
	if _, err := svc.Login(context.Background(), "ghost", "abc12345"); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("unknown user err = %v, want %v", err, domain.ErrInvalidCredentials)
	}
}

func strings_repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
