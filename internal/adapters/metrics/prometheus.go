package metrics

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"net/http"
	"strconv"
	"time"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_http_requests_total",
			Help: "Total HTTP requests handled by wallet-api.",
		},
		[]string{"method", "path", "status"},
	)

	GRPCRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_grpc_requests_total",
			Help: "Total gRPC requests handled by wallet-api.",
		},
		[]string{"method", "status"},
	)

	WalletRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "wallet_request_duration_seconds",
			Help:    "Wallet API request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"protocol", "method", "status"},
	)

	WalletTransfersTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wallet_transfer_total",
			Help: "Total wallet transfer attempts.",
		},
		[]string{"status"},
	)

	WalletActiveWallets = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "wallet_active_wallets_total",
			Help: "Number of active wallets known by repository.",
		},
	)
)

func MustRegister() {
	prometheus.MustRegister(HTTPRequestsTotal)
	prometheus.MustRegister(GRPCRequestsTotal)
	prometheus.MustRegister(WalletRequestDurationSeconds)
	prometheus.MustRegister(WalletTransfersTotal)
	prometheus.MustRegister(WalletActiveWallets)

}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		status := strconv.Itoa(rec.status)
		path := normalizePath(r.URL.Path)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		WalletRequestDurationSeconds.WithLabelValues("http", r.Method+" "+path, status).Observe(time.Since(start).Seconds())
	})
}

func GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		status := "OK"
		if err != nil {
			status = "ERROR"
		}
		GRPCRequestsTotal.WithLabelValues(info.FullMethod, status).Inc()
		WalletRequestDurationSeconds.WithLabelValues("grpc", info.FullMethod, status).Observe(time.Since(start).Seconds())
		return resp, err
	}
}

func RecordTransfer(ok bool) {
	if ok {
		WalletTransfersTotal.WithLabelValues("success").Inc()
		return
	}
	WalletTransfersTotal.WithLabelValues("failed").Inc()
}

func SetActiveWallets(n int64) {
	WalletActiveWallets.Set(float64(n))
}

func normalizePath(path string) string {
	if path == "/" {
		return "/"
	}
	if len(path) > 16 && path[:12] == "/v1/wallets/" {
		return "/v1/wallets/{wallet_id}/*"
	}
	return path
}
