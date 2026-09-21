package application

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

type WalletService struct {
	repo WalletRepository
}

func NewWalletService(repo WalletRepository) *WalletService {
	return &WalletService{repo: repo}
}

func (s *WalletService) CreateWallet(ctx context.Context, userID string, initialBalanceCents int64) (domain.Wallet, error) {
	subject := SubjectFromContext(ctx)
	if userID == "" {
		// Default the owner to the authenticated subject; fall back to a
		// generated id for unauthenticated internal calls (unit tests).
		userID = subject
		if userID == "" {
			userID = "user-" + newID()
		}
	} else if subject != "" && userID != subject {
		return domain.Wallet{}, domain.ErrWalletForbidden
	}
	if initialBalanceCents < 0 {
		return domain.Wallet{}, domain.ErrInvalidAmount
	}

	wallet := domain.Wallet{
		ID:           "wlt_" + newID(),
		UserID:       userID,
		BalanceCents: initialBalanceCents,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.CreateWalletWithBalance(ctx, wallet); err != nil {
		return domain.Wallet{}, err
	}
	return wallet, nil
}

func (s *WalletService) GetBalance(ctx context.Context, walletID string) (domain.Wallet, error) {
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return domain.Wallet{}, err
	}
	if err := s.assertOwnership(ctx, wallet); err != nil {
		return domain.Wallet{}, err
	}
	return wallet, nil
}

func (s *WalletService) Deposit(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	if amountCents <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}
	if err := s.requireWalletForSubject(ctx, walletID); err != nil {
		return domain.Transaction{}, err
	}
	if requestID == "" {
		requestID = "deposit_" + newID()
	}
	return s.repo.Deposit(ctx, walletID, amountCents, requestID)
}

func (s *WalletService) Withdraw(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	if amountCents <= 0 {
		return domain.Transaction{}, domain.ErrInvalidAmount
	}
	if err := s.requireWalletForSubject(ctx, walletID); err != nil {
		return domain.Transaction{}, err
	}
	if requestID == "" {
		requestID = "withdraw_" + newID()
	}
	return s.repo.Withdraw(ctx, walletID, amountCents, requestID)
}

// assertOwnership enforces that an authenticated subject may only access
// wallets they own. When no subject is present (unit tests, system calls)
// the check is skipped. A foreign wallet is reported as "not found" so the
// API does not reveal that the wallet exists at all.
func (s *WalletService) assertOwnership(ctx context.Context, wallet domain.Wallet) error {
	subject := SubjectFromContext(ctx)
	if subject == "" || wallet.UserID == subject {
		return nil
	}
	return domain.ErrWalletNotFound
}

// requireWalletForSubject is the ownership gate applied before mutation.
func (s *WalletService) requireWalletForSubject(ctx context.Context, walletID string) error {
	if SubjectFromContext(ctx) == "" {
		return nil
	}
	wallet, err := s.repo.GetWallet(ctx, walletID)
	if err != nil {
		return err
	}
	return s.assertOwnership(ctx, wallet)
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

	// The sender must own the source wallet; the destination may belong to
	// anyone (a payment is sent to another account's wallet).
	if err := s.requireWalletForSubject(ctx, fromWalletID); err != nil {
		return domain.Transaction{}, err
	}

	if requestID == "" {
		requestID = "transfer_" + newID()
	}

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

	// Seeding creates fleets of load-test wallets on behalf of any
	// authenticated operator; it is a system operation, not personal funds.
	ctx = WithoutSubject(ctx)

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

	// Simulation is a system-level load operation: pick any wallets in the
	// fleet, regardless of which authenticated account ran it.
	ctx = WithoutSubject(ctx)

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

	return s.Transfer(ctx, walletIDs[fromIndex], walletIDs[toIndex], amountCents, "sim_"+newID())
}

func newID() string {
	var value [16]byte
	if _, err := cryptorand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate secure id: %v", err))
	}
	return hex.EncodeToString(value[:])
}

func (s *WalletService) CountWallets(ctx context.Context) (int64, error) {
	return s.repo.CountWallets(ctx)
}
