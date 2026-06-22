package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

func TestWalletRepositoryTransferIsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := NewWalletRepository()

	createTestWallet(t, repo, domain.Wallet{
		ID:           "wallet-from",
		UserID:       "user-from",
		BalanceCents: 1000,
		CreatedAt:    time.Now().UTC(),
	})
	createTestWallet(t, repo, domain.Wallet{
		ID:           "wallet-to",
		UserID:       "user-to",
		BalanceCents: 100,
		CreatedAt:    time.Now().UTC(),
	})

	firstTx, err := repo.Transfer(ctx, "wallet-from", "wallet-to", 250, "transfer-request-1")
	if err != nil {
		t.Fatalf("first transfer failed: %v", err)
	}

	secondTx, err := repo.Transfer(ctx, "wallet-from", "wallet-to", 250, "transfer-request-1")
	if err != nil {
		t.Fatalf("duplicate transfer failed: %v", err)
	}
	if secondTx.ID != firstTx.ID {
		t.Fatalf("duplicate request returned a different transaction: got %q want %q", secondTx.ID, firstTx.ID)
	}

	from, err := repo.GetWallet(ctx, "wallet-from")
	if err != nil {
		t.Fatalf("get source wallet: %v", err)
	}
	to, err := repo.GetWallet(ctx, "wallet-to")
	if err != nil {
		t.Fatalf("get destination wallet: %v", err)
	}

	if from.BalanceCents != 750 {
		t.Fatalf("source balance = %d, want 750", from.BalanceCents)
	}
	if to.BalanceCents != 350 {
		t.Fatalf("destination balance = %d, want 350", to.BalanceCents)
	}
}

func TestWalletRepositoryWithdrawInsufficientFundsDoesNotMutateBalance(t *testing.T) {
	ctx := context.Background()
	repo := NewWalletRepository()

	createTestWallet(t, repo, domain.Wallet{
		ID:           "wallet",
		UserID:       "user",
		BalanceCents: 100,
		CreatedAt:    time.Now().UTC(),
	})

	_, err := repo.Withdraw(ctx, "wallet", 101, "withdraw-request-1")
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("withdraw error = %v, want %v", err, domain.ErrInsufficientFunds)
	}

	wallet, err := repo.GetWallet(ctx, "wallet")
	if err != nil {
		t.Fatalf("get wallet: %v", err)
	}
	if wallet.BalanceCents != 100 {
		t.Fatalf("balance after failed withdraw = %d, want 100", wallet.BalanceCents)
	}
}

func TestWalletRepositoryCountsWallets(t *testing.T) {
	ctx := context.Background()
	repo := NewWalletRepository()

	createTestWallet(t, repo, domain.Wallet{ID: "wallet-1", UserID: "user-1", CreatedAt: time.Now().UTC()})
	createTestWallet(t, repo, domain.Wallet{ID: "wallet-2", UserID: "user-2", CreatedAt: time.Now().UTC()})

	count, err := repo.CountWallets(ctx)
	if err != nil {
		t.Fatalf("count wallets: %v", err)
	}
	if count != 2 {
		t.Fatalf("wallet count = %d, want 2", count)
	}

	ids, err := repo.ListWalletIDs(ctx, 1)
	if err != nil {
		t.Fatalf("list wallets: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("listed wallet count = %d, want 1", len(ids))
	}
}

func createTestWallet(t *testing.T, repo *WalletRepository, wallet domain.Wallet) {
	t.Helper()

	if err := repo.CreateWallet(context.Background(), wallet); err != nil {
		t.Fatalf("create wallet %q: %v", wallet.ID, err)
	}
}
