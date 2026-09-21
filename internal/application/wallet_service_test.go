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

func TestWalletOwnershipIsEnforcedForAuthenticatedSubjects(t *testing.T) {
	repo := memory.NewWalletRepository()
	svc := application.NewWalletService(repo)

	ownerCtx := application.WithSubject(context.Background(), "owner")
	strangerCtx := application.WithSubject(context.Background(), "stranger")

	wallet, err := svc.CreateWallet(ownerCtx, "owner", 1000)
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	// Owner keeps full access.
	if _, err := svc.GetBalance(ownerCtx, wallet.ID); err != nil {
		t.Fatalf("owner GetBalance: %v", err)
	}
	if _, err := svc.Withdraw(ownerCtx, wallet.ID, 10, "req-1"); err != nil {
		t.Fatalf("owner Withdraw: %v", err)
	}

	// A stranger cannot read or move this wallet, and the API must not leak
	// that the wallet exists.
	if _, err := svc.GetBalance(strangerCtx, wallet.ID); !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("stranger GetBalance err = %v, want %v", err, domain.ErrWalletNotFound)
	}
	if _, err := svc.Deposit(strangerCtx, wallet.ID, 5, "req-2"); !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("stranger Deposit err = %v, want %v", err, domain.ErrWalletNotFound)
	}
	if _, err := svc.Withdraw(strangerCtx, wallet.ID, 5, "req-3"); !errors.Is(err, domain.ErrWalletNotFound) {
		t.Fatalf("stranger Withdraw err = %v, want %v", err, domain.ErrWalletNotFound)
	}
}

func TestCreateWalletRejectsForeignOwnerForAuthenticatedSubject(t *testing.T) {
	svc := application.NewWalletService(memory.NewWalletRepository())
	ctx := application.WithSubject(context.Background(), "alice")

	if _, err := svc.CreateWallet(ctx, "mallory", 100); !errors.Is(err, domain.ErrWalletForbidden) {
		t.Fatalf("err = %v, want %v", err, domain.ErrWalletForbidden)
	}
}

func TestSimulateTransferBypassesOwnership(t *testing.T) {
	repo := memory.NewWalletRepository()
	svc := application.NewWalletService(repo)

	// Seed wallets owned by "load-user" (simulated by a system call).
	for i := 0; i < 4; i++ {
		if _, err := svc.SeedWallets(context.Background(), 1, 1000); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// An authenticated subject still runs a system simulation; it must
	// transfer between wallets the user does not own.
	ctx := application.WithSubject(context.Background(), "load-runner")
	_, err := svc.SimulateTransfer(ctx, 4, 10)
	if err != nil {
		t.Fatalf("SimulateTransfer as subject: %v", err)
	}
}
