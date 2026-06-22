package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/anon/wallet-devops-lab/internal/domain"
)

type WalletRepository struct {
	client *goredis.Client
}

func NewWalletRepository(addr string) *WalletRepository {
	return &WalletRepository{
		client: goredis.NewClient(&goredis.Options{Addr: addr}),
	}
}

func (r *WalletRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *WalletRepository) CreateWallet(ctx context.Context, wallet domain.Wallet) error {
	key := walletKey(wallet.ID)
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, key, map[string]any{
		"id":            wallet.ID,
		"user_id":       wallet.UserID,
		"balance_cents": wallet.BalanceCents,
		"created_at":    wallet.CreatedAt.Format(time.RFC3339Nano),
	})
	pipe.SAdd(ctx, "wallets", wallet.ID)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *WalletRepository) GetWallet(ctx context.Context, walletID string) (domain.Wallet, error) {
	m, err := r.client.HGetAll(ctx, walletKey(walletID)).Result()
	if err != nil {
		return domain.Wallet{}, err
	}
	if len(m) == 0 {
		return domain.Wallet{}, domain.ErrWalletNotFound
	}
	balance, _ := strconv.ParseInt(m["balance_cents"], 10, 64)
	createdAt, _ := time.Parse(time.RFC3339Nano, m["created_at"])
	return domain.Wallet{
		ID:           m["id"],
		UserID:       m["user_id"],
		BalanceCents: balance,
		CreatedAt:    createdAt,
	}, nil
}

func (r *WalletRepository) Deposit(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	txID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	res, err := r.client.Eval(ctx, depositLua, []string{requestKey(requestID), walletKey(walletID), transactionKey(txID)}, amountCents, txID, time.Now().UTC().Format(time.RFC3339Nano)).Slice()
	if err != nil {
		return domain.Transaction{}, err
	}
	return parseLuaResult(res, domain.TransactionDeposit, "", walletID, amountCents)
}

func (r *WalletRepository) Withdraw(ctx context.Context, walletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	txID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	res, err := r.client.Eval(ctx, withdrawLua, []string{requestKey(requestID), walletKey(walletID), transactionKey(txID)}, amountCents, txID, time.Now().UTC().Format(time.RFC3339Nano)).Slice()
	if err != nil {
		return domain.Transaction{}, err
	}
	return parseLuaResult(res, domain.TransactionWithdraw, walletID, "", amountCents)
}

func (r *WalletRepository) Transfer(ctx context.Context, fromWalletID string, toWalletID string, amountCents int64, requestID string) (domain.Transaction, error) {
	txID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
	res, err := r.client.Eval(ctx, transferLua, []string{requestKey(requestID), walletKey(fromWalletID), walletKey(toWalletID), transactionKey(txID)}, amountCents, txID, time.Now().UTC().Format(time.RFC3339Nano)).Slice()
	if err != nil {
		return domain.Transaction{}, err
	}
	return parseLuaResult(res, domain.TransactionTransfer, fromWalletID, toWalletID, amountCents)
}

func (r *WalletRepository) ListWalletIDs(ctx context.Context, limit int64) ([]string, error) {
	ids, err := r.client.SMembers(ctx, "wallets").Result()
	if err != nil {
		return nil, err
	}
	if limit > 0 && int64(len(ids)) > limit {
		return ids[:limit], nil
	}
	return ids, nil
}

func (r *WalletRepository) CountWallets(ctx context.Context) (int64, error) {
	return r.client.SCard(ctx, "wallets").Result()
}

func parseLuaResult(res []any, txType, fromWalletID, toWalletID string, amountCents int64) (domain.Transaction, error) {
	if len(res) < 3 {
		return domain.Transaction{}, errors.New("unexpected redis lua result")
	}
	status := fmt.Sprint(res[0])
	txID := fmt.Sprint(res[1])
	message := fmt.Sprint(res[2])

	switch status {
	case "OK", "DUPLICATE":
		return domain.Transaction{
			ID:           txID,
			Type:         txType,
			FromWalletID: fromWalletID,
			ToWalletID:   toWalletID,
			AmountCents:  amountCents,
			Status:       domain.StatusSuccess,
			Message:      message,
			CreatedAt:    time.Now().UTC(),
		}, nil
	case "INSUFFICIENT_FUNDS":
		return domain.Transaction{}, domain.ErrInsufficientFunds
	case "NOT_FOUND", "FROM_NOT_FOUND", "TO_NOT_FOUND":
		return domain.Transaction{}, domain.ErrWalletNotFound
	default:
		return domain.Transaction{}, fmt.Errorf("redis lua status %s: %s", status, message)
	}
}

func walletKey(id string) string      { return "wallet:" + id }
func transactionKey(id string) string { return "transaction:" + id }
func requestKey(id string) string     { return "request:" + id }
