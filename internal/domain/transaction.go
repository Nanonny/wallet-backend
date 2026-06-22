package domain

import "time"

const (
	TransactionDeposit  = "DEPOSIT"
	TransactionWithdraw = "WITHDRAW"
	TransactionTransfer = "TRANSFER"
	StatusSuccess       = "SUCCESS"
	StatusFailed        = "FAILED"
)

type Transaction struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	FromWalletID string    `json:"from_wallet_id,omitempty"`
	ToWalletID   string    `json:"to_wallet_id,omitempty"`
	AmountCents  int64     `json:"amount_cents"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}
