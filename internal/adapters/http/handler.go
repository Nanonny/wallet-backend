package httpadapter

import (
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/anon/wallet-devops-lab/internal/adapters/metrics"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

// Content-Security-Policy values. The console loads demo avatars/styles from
// CDNs, so its policy allows those origins; the login page is fully
// self-contained and gets a closed policy.
const (
	walletConsoleCSP = "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' data: https://fonts.gstatic.com; img-src 'self' data: https://images.unsplash.com https://api.dicebear.com; connect-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'"
	loginCSP         = "default-src 'none'; style-src 'unsafe-inline'; script-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'"
)

func setSecurityHeaders(w http.ResponseWriter, csp string) {
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	w.Header().Set("X-Frame-Options", "DENY")
}

//go:embed static/index.html
var frontendHTML []byte

//go:embed static/login.html
var loginHTML []byte

//go:embed static/login.js
var loginJS []byte

type Handler struct {
	svc          *application.WalletService
	userSvc      *application.UserService
	tokenSvc     *application.TokenService
	loginLimiter *loginLimiter
}

func NewHandler(svc *application.WalletService, userSvc *application.UserService, tokenSvc *application.TokenService) *Handler {
	return &Handler{
		svc:          svc,
		userSvc:      userSvc,
		tokenSvc:     tokenSvc,
		loginLimiter: newLoginLimiter(loginLimitPerWindow, loginWindow),
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.frontend)
	mux.HandleFunc("/login", h.loginPage)
	mux.HandleFunc("/assets/login.js", h.loginAsset)
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/readyz", h.readyz)
	// Public account endpoints.
	mux.HandleFunc("/v1/auth/register", h.register)
	mux.HandleFunc("/v1/auth/login", h.login)
	mux.HandleFunc("/v1/auth/logout", h.logout)
	// Everything else requires a valid bearer token.
	mux.HandleFunc("/v1/auth/me", h.requireAuth(http.HandlerFunc(h.me)).ServeHTTP)
	mux.HandleFunc("/v1/wallets", h.requireAuth(http.HandlerFunc(h.createWallet)).ServeHTTP)
	mux.HandleFunc("/v1/wallets/transfer", h.requireAuth(http.HandlerFunc(h.transfer)).ServeHTTP)
	mux.HandleFunc("/v1/wallets/", h.requireAuth(http.HandlerFunc(h.walletAction)).ServeHTTP)
	mux.HandleFunc("/v1/simulate/seed", h.requireAuth(http.HandlerFunc(h.seed)).ServeHTTP)
	mux.HandleFunc("/v1/simulate/transfer", h.requireAuth(http.HandlerFunc(h.simulateTransfer)).ServeHTTP)
	return metrics.HTTPMiddleware(mux)
}

func (h *Handler) frontend(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	setSecurityHeaders(w, walletConsoleCSP)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(frontendHTML)
}

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	// The login page ships no external scripts at all, so the CSP can be
	// stricter than the wallet console's.
	setSecurityHeaders(w, loginCSP)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(loginHTML)
}

func (h *Handler) loginAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	setSecurityHeaders(w, loginCSP)
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(loginJS)
}

func (h *Handler) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) readyz(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.CountWallets(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	metrics.SetActiveWallets(count)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "wallets": count})
}

func (h *Handler) createWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		UserID              string `json:"user_id"`
		InitialBalanceCents int64  `json:"initial_balance_cents"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	wallet, err := h.svc.CreateWallet(r.Context(), req.UserID, req.InitialBalanceCents)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, wallet)
}

func (h *Handler) walletAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/wallets/"), "/")
	if len(parts) < 2 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	walletID := parts[0]
	action := parts[1]

	switch action {
	case "balance":
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		wallet, err := h.svc.GetBalance(r.Context(), walletID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"wallet_id": wallet.ID, "balance_cents": wallet.BalanceCents})

	case "deposit":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req amountRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		tx, err := h.svc.Deposit(r.Context(), walletID, req.AmountCents, req.RequestID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tx)

	case "withdraw":
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req amountRequest
		if err := decodeJSON(w, r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		tx, err := h.svc.Withdraw(r.Context(), walletID, req.AmountCents, req.RequestID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tx)

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *Handler) transfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		FromWalletID string `json:"from_wallet_id"`
		ToWalletID   string `json:"to_wallet_id"`
		AmountCents  int64  `json:"amount_cents"`
		RequestID    string `json:"request_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.svc.Transfer(r.Context(), req.FromWalletID, req.ToWalletID, req.AmountCents, req.RequestID)
	metrics.RecordTransfer(err == nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

func (h *Handler) seed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Users               int   `json:"users"`
		InitialBalanceCents int64 `json:"initial_balance_cents"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ids, err := h.svc.SeedWallets(r.Context(), req.Users, req.InitialBalanceCents)
	if err != nil {
		writeError(w, err)
		return
	}
	metrics.SetActiveWallets(int64(len(ids)))
	writeJSON(w, http.StatusOK, map[string]any{"created": len(ids), "sample_wallet_ids": first(ids, 5)})
}

func (h *Handler) simulateTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Users       int   `json:"users"`
		AmountCents int64 `json:"amount_cents"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.svc.SimulateTransfer(r.Context(), req.Users, req.AmountCents)
	metrics.RecordTransfer(err == nil)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

type amountRequest struct {
	AmountCents int64  `json:"amount_cents"`
	RequestID   string `json:"request_id"`
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrWalletNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrWalletForbidden):
		status = http.StatusForbidden
	case errors.Is(err, domain.ErrInvalidAmount):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInsufficientFunds):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrIdempotencyConflict):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNotEnoughWallets):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// writeInternalError hides driver/infrastructure details from clients.
// The full error is left in server logs for operators.
func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain exactly one JSON object")
		}
		return err
	}
	return nil
}

func first(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
