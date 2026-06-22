package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/anon/wallet-devops-lab/internal/adapters/metrics"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

type Handler struct {
	svc *application.WalletService
}

func NewHandler(svc *application.WalletService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.healthz)
	mux.HandleFunc("/readyz", h.readyz)
	mux.HandleFunc("/v1/wallets", h.createWallet)
	mux.HandleFunc("/v1/wallets/transfer", h.transfer)
	mux.HandleFunc("/v1/wallets/", h.walletAction)
	mux.HandleFunc("/v1/simulate/seed", h.seed)
	mux.HandleFunc("/v1/simulate/transfer", h.simulateTransfer)
	return metrics.HTTPMiddleware(mux)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	case errors.Is(err, domain.ErrInvalidAmount):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrInsufficientFunds):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrNotEnoughWallets):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func first(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	return items[:n]
}
