package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	grpcadapter "github.com/anon/wallet-devops-lab/internal/adapters/grpc"
	httpadapter "github.com/anon/wallet-devops-lab/internal/adapters/http"
	"github.com/anon/wallet-devops-lab/internal/adapters/memory"
	"github.com/anon/wallet-devops-lab/internal/adapters/metrics"
	postgresadapter "github.com/anon/wallet-devops-lab/internal/adapters/postgres"
	redisrepo "github.com/anon/wallet-devops-lab/internal/adapters/redis"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/config"
)

func main() {
	cfg := config.Load()
	metrics.MustRegister()
	grpcadapter.RegisterJSONCodec()

	ctx := context.Background()

	// Wallet state (Redis or in-memory for the zero-dependency demo).
	var walletRepo application.WalletRepository
	if strings.EqualFold(cfg.Storage, "memory") {
		log.Println("storage=memory (wallets)")
		walletRepo = memory.NewWalletRepository()
	} else {
		log.Printf("storage=redis redis_addr=%s", cfg.RedisAddr)
		r := redisrepo.NewWalletRepository(cfg.RedisAddr, cfg.RedisPassword)
		if err := r.Ping(ctx); err != nil {
			log.Fatalf("redis ping failed: %v", err)
		}
		defer func() {
			if err := r.Close(); err != nil {
				log.Printf("close redis: %v", err)
			}
		}()
		walletRepo = r
	}

	// User accounts (PostgreSQL, or in-memory in zero-dependency demo mode).
	var userRepo application.UserRepository
	if strings.EqualFold(cfg.Storage, "memory") {
		log.Println("storage=memory (users)")
		userRepo = memory.NewUserRepository()
	} else {
		dsn := cfg.PostgresDSN()
		db, err := postgresadapter.Open(ctx, dsn, 30*time.Second)
		if err != nil {
			log.Fatalf("connect postgres: %v (start it with: docker compose up -d postgres)", err)
		}
		defer postgresadapter.Close(db)
		if err := postgresadapter.Migrate(ctx, db); err != nil {
			log.Fatalf("postgres migrate: %v", err)
		}
		log.Printf("storage=postgres host=%s user=%s", cfg.PGHost, cfg.PGUser)
		userRepo = postgresadapter.NewUserRepository(db)
	}

	walletSvc := application.NewWalletService(walletRepo)
	userSvc := application.NewUserService(userRepo)

	jwtSecret := cfg.JWTSecret
	if jwtSecret == "" {
		var b [32]byte
		if _, err := cryptorand.Read(b[:]); err != nil {
			log.Fatalf("generate jwt secret: %v", err)
		}
		jwtSecret = hex.EncodeToString(b[:])
		log.Println("WARNING: JWT_SECRET is not set; using an ephemeral secret. Sessions will not survive a restart and shared replicas will disagree. Set JWT_SECRET for real deployments.")
	}
	tokenSvc, err := application.NewTokenService(jwtSecret)
	if err != nil {
		log.Fatalf("initialize token service: %v", err)
	}

	metricsServer := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           promhttp.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           httpadapter.NewHandler(walletSvc, userSvc, tokenSvc).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpcadapter.AuthUnaryServerInterceptor(tokenSvc),
			metrics.GRPCUnaryServerInterceptor(),
		),
	)
	grpcadapter.RegisterWalletServiceServer(grpcServer, grpcadapter.NewWalletGRPCServer(walletSvc))

	go func() {
		log.Printf("metrics listening on :%s", cfg.MetricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("metrics server failed: %v", err)
		}
	}()

	go func() {
		log.Printf("http listening on :%s", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	go func() {
		listener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			log.Fatalf("grpc listen failed: %v", err)
		}
		log.Printf("grpc listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("grpc server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down wallet-api")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = metricsServer.Shutdown(shutdownCtx)
	grpcServer.GracefulStop()
}
