package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	grpcadapter "github.com/anon/wallet-devops-lab/internal/adapters/grpc"
)

// The gRPC wallet API requires a bearer token. This demo client obtains one
// through the HTTP login endpoint, then sends it as gRPC metadata.

func main() {
	grpcadapter.RegisterJSONCodec()

	username := envOr("GRPC_CLIENT_USERNAME", "demo")
	password := envOr("GRPC_CLIENT_PASSWORD", "wallet-demo-1234")
	httpBase := strings.TrimSuffix(envOr("WALLET_HTTP_BASE", "http://localhost:8080"), "/")

	token, err := login(httpBase, username, password)
	if err != nil {
		log.Fatalf("login failed: %v", err)
	}
	log.Printf("authenticated as %s (token %s...)", username, token[:min(12, len(token))])

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
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)

	var created grpcadapter.CreateWalletResponse
	if err := conn.Invoke(ctx, "/wallet.v1.WalletService/CreateWallet", &grpcadapter.CreateWalletRequest{
		UserId:              username,
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

func login(base, username, password string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})

	// Best-effort self-registration so the demo works against a fresh store;
	// 409 (already exists) and 400 (weak) are acceptable here.
	if res, err := http.Post(base+"/v1/auth/register", "application/json", bytes.NewReader(body)); err == nil {
		res.Body.Close()
	}

	res, err := http.Post(base+"/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var out struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", res.StatusCode, out.Error)
	}
	return out.Token, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
