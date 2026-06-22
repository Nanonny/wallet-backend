package domain

import "time"

type Wallet struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	BalanceCents int64     `json:"balance_cents"`
	CreatedAt    time.Time `json:"created_at"`
}
