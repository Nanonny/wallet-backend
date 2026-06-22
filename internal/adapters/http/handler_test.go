package httpadapter_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpadapter "github.com/anon/wallet-devops-lab/internal/adapters/http"
	"github.com/anon/wallet-devops-lab/internal/adapters/memory"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

func TestHandlerHealthAndReadiness(t *testing.T) {
	handler := newTestHandler()

	assertJSONStatus(t, handler, http.MethodGet, "/healthz", "", http.StatusOK)
	assertJSONStatus(t, handler, http.MethodGet, "/readyz", "", http.StatusOK)
}

func TestHandlerCreatesWalletAndReturnsBalance(t *testing.T) {
	handler := newTestHandler()

	createBody := `{"user_id":"user-1","initial_balance_cents":1500}`
	createResponse := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/v1/wallets", strings.NewReader(createBody))
	createRequest.Header.Set("Content-Type", "application/json")

	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create wallet status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	var created domain.Wallet
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatalf("decode created wallet: %v", err)
	}
	if created.ID == "" {
		t.Fatal("created wallet id is empty")
	}

	balanceResponse := httptest.NewRecorder()
	balanceRequest := httptest.NewRequest(http.MethodGet, "/v1/wallets/"+created.ID+"/balance", nil)
	handler.ServeHTTP(balanceResponse, balanceRequest)
	if balanceResponse.Code != http.StatusOK {
		t.Fatalf("balance status = %d, want %d: %s", balanceResponse.Code, http.StatusOK, balanceResponse.Body.String())
	}

	var balance struct {
		WalletID     string `json:"wallet_id"`
		BalanceCents int64  `json:"balance_cents"`
	}
	if err := json.NewDecoder(balanceResponse.Body).Decode(&balance); err != nil {
		t.Fatalf("decode balance: %v", err)
	}
	if balance.WalletID != created.ID {
		t.Fatalf("balance wallet id = %q, want %q", balance.WalletID, created.ID)
	}
	if balance.BalanceCents != 1500 {
		t.Fatalf("balance = %d, want 1500", balance.BalanceCents)
	}
}

func TestHandlerRejectsInvalidCreateWalletPayload(t *testing.T) {
	handler := newTestHandler()

	assertJSONStatus(t, handler, http.MethodPost, "/v1/wallets", `{"initial_balance_cents":-1}`, http.StatusBadRequest)
}

func newTestHandler() http.Handler {
	repo := memory.NewWalletRepository()
	svc := application.NewWalletService(repo)
	return httpadapter.NewHandler(svc).Routes()
}

func assertJSONStatus(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
}
