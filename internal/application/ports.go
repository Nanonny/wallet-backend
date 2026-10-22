package application

import (
	"context"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

// UserRepository is the port for authenticated user accounts.
// Implementations must store only bcrypt password hashes, never plaintext,
// and must map credential failures to domain.ErrInvalidCredentials.
type UserRepository interface {
	// CreateUser stores a new account. Returns domain.ErrUserExists when the
	// username is already taken.
	CreateUser(ctx context.Context, username, password string) (domain.User, error)
	// Authenticate verifies the password, refreshes last_login_at, and
	// returns the user. Returns domain.ErrInvalidCredentials on failure.
	Authenticate(ctx context.Context, username, password string) (domain.User, error)
}

type WalletRepository interface {
	CreateWallet(ctx context.Context, wallet domain.Wallet) error
	CreateWalletWithBalance(ctx context.Context, wallet domain.Wallet) error
	GetWallet(ctx context.Context, walletID string) (domain.Wallet, error)
	Deposit(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error)
	Withdraw(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error)
	Transfer(ctx context.Context, fromWalletID string, toWalletID string, amountCents int64, requestID string) (domain.Transaction, error)
	ListWalletIDs(ctx context.Context, limit int64) ([]string, error)
	CountWallets(ctx context.Context) (int64, error)
}
