package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anon/wallet-devops-lab/internal/adapters/memory"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

func TestCreateWalletDepositsInitialBalance(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewWalletRepository()
	svc := application.NewWalletService(repo)

	wallet, err := svc.CreateWallet(ctx, "user-1", 500)
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	stored, err := repo.GetWallet(ctx, wallet.ID)
	if err != nil {
		t.Fatalf("get wallet: %v", err)
	}
	if stored.UserID != "user-1" {
		t.Fatalf("user id = %q, want user-1", stored.UserID)
	}
	if stored.BalanceCents != 500 {
		t.Fatalf("balance = %d, want 500", stored.BalanceCents)
	}
}

func TestCreateWalletRejectsNegativeInitialBalance(t *testing.T) {
	svc := application.NewWalletService(memory.NewWalletRepository())

	_, err := svc.CreateWallet(context.Background(), "user-1", -1)
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("error = %v, want %v", err, domain.ErrInvalidAmount)
	}
}

func TestDepositRejectsInvalidAmount(t *testing.T) {
	svc := application.NewWalletService(memory.NewWalletRepository())

	_, err := svc.Deposit(context.Background(), "wallet-1", 0, "deposit-request-1")
	if !errors.Is(err, domain.ErrInvalidAmount) {
		t.Fatalf("error = %v, want %v", err, domain.ErrInvalidAmount)
	}
}
