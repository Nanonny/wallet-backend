package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcadapter "github.com/anon/wallet-devops-lab/internal/adapters/grpc"
)

func main() {
	grpcadapter.RegisterJSONCodec()

	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")),
	)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var created grpcadapter.CreateWalletResponse
	if err := conn.Invoke(ctx, "/wallet.v1.WalletService/CreateWallet", &grpcadapter.CreateWalletRequest{
		UserId:              "grpc-demo-user",
		InitialBalanceCents: 100000,
	}, &created); err != nil {
		log.Fatalf("CreateWallet failed: %v", err)
	}
	log.Printf("created wallet: %+v", created)

	var balance grpcadapter.GetBalanceResponse
	if err := conn.Invoke(ctx, "/wallet.v1.WalletService/GetBalance", &grpcadapter.GetBalanceRequest{
		WalletId: created.WalletId,
	}, &balance); err != nil {
		log.Fatalf("GetBalance failed: %v", err)
	}
	log.Printf("balance: %+v", balance)
}
