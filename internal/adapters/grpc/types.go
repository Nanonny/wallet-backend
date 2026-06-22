package grpcadapter

type CreateWalletRequest struct {
	UserId              string `json:"user_id"`
	InitialBalanceCents int64  `json:"initial_balance_cents"`
}

type CreateWalletResponse struct {
	WalletId     string `json:"wallet_id"`
	UserId       string `json:"user_id"`
	BalanceCents int64  `json:"balance_cents"`
}

type GetBalanceRequest struct {
	WalletId string `json:"wallet_id"`
}

type GetBalanceResponse struct {
	WalletId     string `json:"wallet_id"`
	BalanceCents int64  `json:"balance_cents"`
}

type DepositRequest struct {
	WalletId    string `json:"wallet_id"`
	AmountCents int64  `json:"amount_cents"`
	RequestId   string `json:"request_id"`
}

type WithdrawRequest struct {
	WalletId    string `json:"wallet_id"`
	AmountCents int64  `json:"amount_cents"`
	RequestId   string `json:"request_id"`
}

type TransferRequest struct {
	FromWalletId string `json:"from_wallet_id"`
	ToWalletId   string `json:"to_wallet_id"`
	AmountCents  int64  `json:"amount_cents"`
	RequestId    string `json:"request_id"`
}

type TransactionResponse struct {
	TransactionId string `json:"transaction_id"`
	Status        string `json:"status"`
	Message       string `json:"message"`
}
