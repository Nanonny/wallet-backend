package application

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/anon/wallet-devops-lab/internal/adapters/metrics"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

type WalletService struct {
	repo WalletRepository
}

func NewWalletService(repo WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

func (s *WalletService) CreateWallet(ctx context.Context, userID string, initialBalanceCents int64) (domain.Wallet, error) {
	if userID == "" {
		userID = fmt.Sprintf("user-%d", time.Now().UnixNano())
	}
	if initialBalanceCents < 0 {
		return domain.Wallet{}, domain.ErrInvalidAmount
	}

	wallet := domain.Wallet{
		ID:           fmt.Sprintf("wlt_%d_%d", time.Now().UnixNano(), rand.Intn(100000)),
		UserID:       userID,
		BalanceCents: 0,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.CreateWallet(ctx, wallet); err != nil {
		return domain.Wallet{}, err
	}
	if initialBalanceCents > 0 {
		_, err := s.repo.Deposit(ctx, wallet.ID, initialBalanceCents, fmt.Sprintf("seed_%s", wallet.ID))
		if err != nil {
			return domain.Wallet{}, err
		}
		wallet.BalanceCents = initialBalanceCents
	}
	return wallet, nil
}

func (s *WalletService) GetBalance(ctx context.Context, walletID string) (domain.Wallet, error) {
	return s.repo.GetWallet(ctx, walletID)
}

func (s *WalletService) Deposit(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	if amountCents <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}
	if requestID == "" {
		requestID = fmt.Sprintf("deposit_%d", time.Now().UnixNano())
	}
	return s.repo.Deposit(ctx, walletID, amountCents, requestID)
}

func (s *WalletService) Withdraw(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	if amountCents <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}
	if requestID == "" {
		requestID = fmt.Sprintf("withdraw_%d", time.Now().UnixNano())
	}
	return s.repo.Withdraw(ctx, walletID, amountCents, requestID)
}

func (s *WalletService) Transfer(
	ctx context.Context,
	fromWalletID string,
	toWalletID string,
	amountCents int64,
	requestID string,
) (domain.Transaction, error) {

	if amountCents <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}

	if fromWalletID == "" || toWalletID == "" || fromWalletID == toWalletID {
		return domain.Transaction{},
			fmt.Errorf("from_wallet_id and to_wallet_id must be different")
	}

	if requestID == "" {
		requestID = fmt.Sprintf(
			"transfer_%d",
			time.Now().UnixNano(),
		)
	}

	// 20% slow request
	if rand.Intn(100) < 20 {
		time.Sleep(300 * time.Millisecond)
	}

	// 5% error
	if rand.Intn(100) < 5 {
		metrics.RecordTransfer(false)
		return domain.Transaction{},
			fmt.Errorf("simulated transfer failure")
	}

	metrics.RecordTransfer(true)

	return s.repo.Transfer(
		ctx,
		fromWalletID,
		toWalletID,
		amountCents,
		requestID,
	)
}

func (s *WalletService) SeedWallets(ctx context.Context, users int, initialBalanceCents int64) ([]string, error) {
	if users <= 0 {
		users = 100
	}
	if initialBalanceCents <= 0 {
		initialBalanceCents = 100000
	}

	ids := make([]string, 0, users)
	prefix := time.Now().UnixNano()
	for i := 0; i < users; i++ {
		wallet, err := s.CreateWallet(ctx, fmt.Sprintf("load-user-%d-%d", prefix, i), initialBalanceCents)
		if err != nil {
			return ids, err
		}
		ids = append(ids, wallet.ID)
	}
	return ids, nil
}

func (s *WalletService) SimulateTransfer(ctx context.Context, users int, amountCents int64) (domain.Transaction, error) {
	if users <= 0 {
		users = 1000
	}
	if amountCents <= 0 {
		amountCents = 100
	}

	walletIDs, err := s.repo.ListWalletIDs(ctx, int64(users))
	if err != nil {
		return domain.Transaction{}, err
	}
	if len(walletIDs) < 2 {
		return domain.Transaction{}, domain.ErrNotEnoughWallets
	}

	fromIndex := rand.Intn(len(walletIDs))
	toIndex := rand.Intn(len(walletIDs))
	for toIndex == fromIndex {
		toIndex = rand.Intn(len(walletIDs))
	}

	return s.Transfer(ctx, walletIDs[fromIndex], walletIDs[toIndex], amountCents, fmt.Sprintf("sim_%d_%d", time.Now().UnixNano(), rand.Intn(1000000)))
}

func (s *WalletService) CountWallets(ctx context.Context) (int64, error) {
	return s.repo.CountWallets(ctx)
}
