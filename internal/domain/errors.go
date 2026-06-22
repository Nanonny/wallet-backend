package domain

import "errors"

var (
	ErrWalletNotFound     = errors.New("wallet not found")
	ErrInvalidAmount      = errors.New("amount must be greater than zero")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrDuplicateRequestID = errors.New("duplicate request id")
	ErrNotEnoughWallets   = errors.New("not enough wallets for simulation")
)
