package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

type WalletRepository struct {
	mu             sync.RWMutex
	wallets        map[string]domain.Wallet
	transactions   map[string]domain.Transaction
	requestIDToTxn map[string]string
	requestHashes  map[string]string
}

func NewWalletRepository() *WalletRepository {
	return &WalletRepository{
		wallets:        map[string]domain.Wallet{},
		transactions:   map[string]domain.Transaction{},
		requestIDToTxn: map[string]string{},
		requestHashes:  map[string]string{},
	}
}

func (r *WalletRepository) CreateWalletWithBalance(ctx context.Context, wallet domain.Wallet) error {
	return r.CreateWallet(ctx, wallet)
}

func (r *WalletRepository) CreateWallet(ctx context.Context, wallet domain.Wallet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.wallets[wallet.ID] = wallet
	return nil
}

func (r *WalletRepository) GetWallet(ctx context.Context, walletID string) (domain.Wallet, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	w, ok := r.wallets[walletID]
	if !ok {
		return domain.Wallet{}, domain.ErrWalletNotFound
	}
	return w, nil
}

func (r *WalletRepository) Deposit(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fingerprint := requestFingerprint(domain.TransactionDeposit, "", walletID, amountCents)
	if tx, handled, err := r.duplicateRequest(requestID, fingerprint); handled {
		return tx, err
	}
	w, ok := r.wallets[walletID]
	if !ok {
		return domain.Transaction{}, domain.ErrWalletNotFound
	}
	w.BalanceCents += amountCents
	r.wallets[walletID] = w
	tx := newTransaction(domain.TransactionDeposit, "", walletID, amountCents, "deposit success")
	r.transactions[tx.ID] = tx
	r.requestIDToTxn[requestID] = tx.ID
	r.requestHashes[requestID] = fingerprint
	return tx, nil
}

func (r *WalletRepository) Withdraw(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fingerprint := requestFingerprint(domain.TransactionWithdraw, walletID, "", amountCents)
	if tx, handled, err := r.duplicateRequest(requestID, fingerprint); handled {
		return tx, err
	}
	w, ok := r.wallets[walletID]
	if !ok {
		return domain.Transaction{}, domain.ErrWalletNotFound
	}
	if w.BalanceCents < amountCents {
		return domain.Transaction{}, domain.ErrInsufficientFunds
	}
	w.BalanceCents -= amountCents
	r.wallets[walletID] = w
	tx := newTransaction(domain.TransactionWithdraw, walletID, "", amountCents, "withdraw success")
	r.transactions[tx.ID] = tx
	r.requestIDToTxn[requestID] = tx.ID
	r.requestHashes[requestID] = fingerprint
	return tx, nil
}

func (r *WalletRepository) Transfer(ctx context.Context, fromWalletID string, toWalletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fingerprint := requestFingerprint(domain.TransactionTransfer, fromWalletID, toWalletID, amountCents)
	if tx, handled, err := r.duplicateRequest(requestID, fingerprint); handled {
		return tx, err
	}
	from, ok := r.wallets[fromWalletID]
	if !ok {
		return domain.Transaction{}, domain.ErrWalletNotFound
	}
	to, ok := r.wallets[toWalletID]
	if !ok {
		return domain.Transaction{}, domain.ErrWalletNotFound
	}
	if from.BalanceCents < amountCents {
		return domain.Transaction{}, domain.ErrInsufficientFunds
	}
	from.BalanceCents -= amountCents
	to.BalanceCents += amountCents
	r.wallets[fromWalletID] = from
	r.wallets[toWalletID] = to
	tx := newTransaction(domain.TransactionTransfer, fromWalletID, toWalletID, amountCents, "transfer success")
	r.transactions[tx.ID] = tx
	r.requestIDToTxn[requestID] = tx.ID
	r.requestHashes[requestID] = fingerprint
	return tx, nil
}

func (r *WalletRepository) duplicateRequest(requestID, fingerprint string) (domain.Transaction, bool, error) {
	txID, ok := r.requestIDToTxn[requestID]
	if !ok {
		return domain.Transaction{}, false, nil
	}
	if r.requestHashes[requestID] != fingerprint {
		return domain.Transaction{}, true, domain.ErrIdempotencyConflict
	}
	return r.transactions[txID], true, nil
}

func requestFingerprint(txType, from, to string, amount int64) string {
	return fmt.Sprintf("%s|%s|%s|%d", txType, from, to, amount)
}

func (r *WalletRepository) ListWalletIDs(ctx context.Context, limit int64) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.wallets))
	for id := range r.wallets {
		ids = append(ids, id)
		if limit > 0 && int64(len(ids)) >= limit {
			break
		}
	}
	return ids, nil
}

func (r *WalletRepository) CountWallets(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.wallets)), nil
}

func newTransaction(txType, from, to string, amount int64, message string) domain.Transaction {
	return domain.Transaction{
		ID:           fmt.Sprintf("txn_%d", time.Now().UnixNano()),
		Type:         txType,
		FromWalletID: from,
		ToWalletID:   to,
		AmountCents:  amount,
		Status:       domain.StatusSuccess,
		Message:      message,
		CreatedAt:    time.Now().UTC(),
	}
}
