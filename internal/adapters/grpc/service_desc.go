package grpcadapter

import (
	"context"

	"google.golang.org/grpc"
)

type WalletServiceServer interface {
	CreateWallet(context.Context, *CreateWalletRequest) (*CreateWalletResponse, error)
	GetBalance(context.Context, *GetBalanceRequest) (*GetBalanceResponse, error)
	Deposit(context.Context, *DepositRequest) (*TransactionResponse, error)
	Withdraw(context.Context, *WithdrawRequest) (*TransactionResponse, error)
	Transfer(context.Context, *TransferRequest) (*TransactionResponse, error)
}

func RegisterWalletServiceServer(s grpc.ServiceRegistrar, srv WalletServiceServer) {
	s.RegisterService(&WalletService_ServiceDesc, srv)
}

var WalletService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "wallet.v1.WalletService",
	HandlerType: (*WalletServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "CreateWallet", Handler: _WalletService_CreateWallet_Handler},
		{MethodName: "GetBalance", Handler: _WalletService_GetBalance_Handler},
		{MethodName: "Deposit", Handler: _WalletService_Deposit_Handler},
		{MethodName: "Withdraw", Handler: _WalletService_Withdraw_Handler},
		{MethodName: "Transfer", Handler: _WalletService_Transfer_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "api/proto/wallet.proto",
}

func _WalletService_CreateWallet_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(CreateWalletRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WalletServiceServer).CreateWallet(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/wallet.v1.WalletService/CreateWallet"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WalletServiceServer).CreateWallet(ctx, req.(*CreateWalletRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WalletService_GetBalance_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(GetBalanceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WalletServiceServer).GetBalance(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/wallet.v1.WalletService/GetBalance"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WalletServiceServer).GetBalance(ctx, req.(*GetBalanceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WalletService_Deposit_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(DepositRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WalletServiceServer).Deposit(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/wallet.v1.WalletService/Deposit"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WalletServiceServer).Deposit(ctx, req.(*DepositRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WalletService_Withdraw_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(WithdrawRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WalletServiceServer).Withdraw(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/wallet.v1.WalletService/Withdraw"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WalletServiceServer).Withdraw(ctx, req.(*WithdrawRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _WalletService_Transfer_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(TransferRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WalletServiceServer).Transfer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/wallet.v1.WalletService/Transfer"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(WalletServiceServer).Transfer(ctx, req.(*TransferRequest))
	}
	return interceptor(ctx, in, info, handler)
}
