package main

import (
	"context"
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
	redisrepo "github.com/anon/wallet-devops-lab/internal/adapters/redis"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/config"
)

func main() {
	cfg := config.Load()
	metrics.MustRegister()
	grpcadapter.RegisterJSONCodec()

	ctx := context.Background()
	var repo application.WalletRepository

	if strings.EqualFold(cfg.Storage, "memory") {
		log.Println("storage=memory")
		repo = memory.NewWalletRepository()
	} else {
		log.Printf("storage=redis redis_addr=%s", cfg.RedisAddr)
		r := redisrepo.NewWalletRepository(cfg.RedisAddr)
		if err := r.Ping(ctx); err != nil {
			log.Fatalf("redis ping failed: %v", err)
		}
		repo = r
	}

	walletSvc := application.NewWalletService(repo)

	metricsServer := &http.Server{Addr: ":" + cfg.MetricsPort, Handler: promhttp.Handler()}
	httpServer := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: httpadapter.NewHandler(walletSvc).Routes()}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(metrics.GRPCUnaryServerInterceptor()))
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
