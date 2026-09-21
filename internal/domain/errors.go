package domain

import "errors"

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInvalidAmount       = errors.New("amount must be greater than zero")
	ErrInsufficientFunds   = errors.New("insufficient funds")
	ErrDuplicateRequestID  = errors.New("duplicate request id")
	ErrIdempotencyConflict = errors.New("request id was already used with a different request")
	ErrNotEnoughWallets    = errors.New("not enough wallets for simulation")

	ErrUserExists         = errors.New("username is already taken")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidUsername    = errors.New("username must be 3-32 characters: lowercase letters, numbers, or _")
	ErrWeakPassword       = errors.New("password must be at least 8 characters and contain a letter and a number")
	ErrUserNotFound       = errors.New("user not found")
	ErrUnauthorized       = errors.New("missing or invalid bearer token")
	ErrWalletForbidden    = errors.New("wallet does not belong to this user")
)
