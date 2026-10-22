package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

func TestRedisTransferAtomicityAndIdempotency(t *testing.T) {
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := NewWalletRepository(addr, "")
	defer repo.Close()
	if err := repo.Ping(ctx); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	suffix := time.Now().UnixNano()
	fromID := fmt.Sprintf("redis-test-from-%d", suffix)
	toID := fmt.Sprintf("redis-test-to-%d", suffix)
	requestID := fmt.Sprintf("redis-test-request-%d", suffix)
	now := time.Now().UTC()
	if err := repo.CreateWalletWithBalance(ctx, domain.Wallet{ID: fromID, UserID: "from", BalanceCents: 1000, CreatedAt: now}); err != nil {
		t.Fatalf("create source wallet: %v", err)
	}
	if err := repo.CreateWalletWithBalance(ctx, domain.Wallet{ID: toID, UserID: "to", CreatedAt: now}); err != nil {
		t.Fatalf("create destination wallet: %v", err)
	}

	first, err := repo.Transfer(ctx, fromID, toID, 250, requestID)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	duplicate, err := repo.Transfer(ctx, fromID, toID, 250, requestID)
	if err != nil {
		t.Fatalf("duplicate transfer: %v", err)
	}
	if duplicate.ID != first.ID || !duplicate.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("duplicate transaction changed: first=%+v duplicate=%+v", first, duplicate)
	}

	_, err = repo.Transfer(ctx, fromID, toID, 251, requestID)
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("payload mismatch error = %v, want %v", err, domain.ErrIdempotencyConflict)
	}
	from, err := repo.GetWallet(ctx, fromID)
	if err != nil {
		t.Fatalf("get source wallet: %v", err)
	}
	to, err := repo.GetWallet(ctx, toID)
	if err != nil {
		t.Fatalf("get destination wallet: %v", err)
	}
	if from.BalanceCents != 750 || to.BalanceCents != 250 {
		t.Fatalf("unexpected balances: from=%d to=%d", from.BalanceCents, to.BalanceCents)
	}
}
