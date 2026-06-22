package grpcadapter

import (
	"context"

	"github.com/anon/wallet-devops-lab/internal/application"
)

type WalletGRPCServer struct {
	svc *application.WalletService
}

func NewWalletGRPCServer(svc *application.WalletService) *WalletGRPCServer {
	return &WalletGRPCServer{svc: svc}
}

func (s *WalletGRPCServer) CreateWallet(ctx context.Context, req *CreateWalletRequest) (*CreateWalletResponse, error) {
	wallet, err := s.svc.CreateWallet(ctx, req.UserId, req.InitialBalanceCents)
	if err != nil {
		return nil, err
	}
	return &CreateWalletResponse{WalletId: wallet.ID, UserId: wallet.UserID, BalanceCents: wallet.BalanceCents}, nil
}

func (s *WalletGRPCServer) GetBalance(ctx context.Context, req *GetBalanceRequest) (*GetBalanceResponse, error) {
	wallet, err := s.svc.GetBalance(ctx, req.WalletId)
	if err != nil {
		return nil, err
	}
	return &GetBalanceResponse{WalletId: wallet.ID, BalanceCents: wallet.BalanceCents}, nil
}

func (s *WalletGRPCServer) Deposit(ctx context.Context, req *DepositRequest) (*TransactionResponse, error) {
	tx, err := s.svc.Deposit(ctx, req.WalletId, req.AmountCents, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &TransactionResponse{TransactionId: tx.ID, Status: tx.Status, Message: tx.Message}, nil
}

func (s *WalletGRPCServer) Withdraw(ctx context.Context, req *WithdrawRequest) (*TransactionResponse, error) {
	tx, err := s.svc.Withdraw(ctx, req.WalletId, req.AmountCents, req.RequestId)
	if err != nil {
		return nil, err
	}
	return &TransactionResponse{TransactionId: tx.ID, Status: tx.Status, Message: tx.Message}, nil
}

func (s *WalletGRPCServer) Transfer(ctx context.Context, req *TransferRequest) (*TransactionResponse, error) {
	tx, err := s.svc.Transfer(ctx, req.FromWalletId, req.ToWalletId, req.AmountCents, req.RequestId)
	// metrics.RecordTransfer(err == nil)
	if err != nil {
		return nil, err
	}
	return &TransactionResponse{TransactionId: tx.ID, Status: tx.Status, Message: tx.Message}, nil
}
