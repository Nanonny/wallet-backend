package application

import (
	"context"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

type WalletRepository interface {
	CreateWallet(ctx context.Context, wallet domain.Wallet) error
	GetWallet(ctx context.Context, walletID string) (domain.Wallet, error)
	Deposit(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error)
	Withdraw(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error)
	Transfer(ctx context.Context, fromWalletID string, toWalletID string, amountCents int64, requestID string) (domain.Transaction, error)
	ListWalletIDs(ctx context.Context, limit int64) ([]string, error)
	CountWallets(ctx context.Context) (int64, error)
}
