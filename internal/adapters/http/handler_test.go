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

	assertJSONStatus(t, handler, http.MethodGet, "/healthz", "", "", http.StatusOK)
	assertJSONStatus(t, handler, http.MethodGet, "/readyz", "", "", http.StatusOK)
}

func TestHandlerServesFrontendAndLoginPage(t *testing.T) {
	handler := newTestHandler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("frontend status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Wallet Console") {
		t.Fatal("frontend response does not contain Wallet Console")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("frontend response is missing a Content-Security-Policy header")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("login page status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Sign in") {
		t.Fatal("login page does not contain sign-in form")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if strings.Contains(csp, "unpkg") || strings.Contains(csp, "tailwind") {
		t.Fatalf("login page CSP must not allow console CDNs: %q", csp)
	}
}

func TestProtectedRoutesRequireBearerToken(t *testing.T) {
	handler := newTestHandler()
	assertJSONStatus(t, handler, http.MethodPost, "/v1/wallets", `{"initial_balance_cents":100}`, "", http.StatusUnauthorized)
	assertJSONStatus(t, handler, http.MethodPost, "/v1/simulate/seed", `{"users":10}`, "", http.StatusUnauthorized)
	assertJSONStatus(t, handler, http.MethodGet, "/v1/auth/me", "", "", http.StatusUnauthorized)
}

func TestRegisterLoginMeRoundTrip(t *testing.T) {
	handler := newTestHandler()

	rec := postJSON(t, handler, "/v1/auth/register", `{"username":"demo","password":"abc12345"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	dup := postJSON(t, handler, "/v1/auth/register", `{"username":"DEMO","password":"abc12345"}`)
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want %d", dup.Code, http.StatusConflict)
	}

	weak := postJSON(t, handler, "/v1/auth/register", `{"username":"weakling","password":"short"}`)
	if weak.Code != http.StatusBadRequest {
		t.Fatalf("weak password register status = %d, want %d", weak.Code, http.StatusBadRequest)
	}

	badLogin := postJSON(t, handler, "/v1/auth/login", `{"username":"demo","password":"wrong9999"}`)
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, want %d", badLogin.Code, http.StatusUnauthorized)
	}

	login := postJSON(t, handler, "/v1/auth/login", `{"username":"demo","password":"abc12345"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}
	var loginBody struct {
		Token string `json:"token"`
		User  struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := json.NewDecoder(login.Body).Decode(&loginBody); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if loginBody.Token == "" {
		t.Fatal("login response has no token")
	}
	if loginBody.User.Username != "demo" {
		t.Fatalf("login user = %q, want demo", loginBody.User.Username)
	}

	me := getWithToken(t, handler, "/v1/auth/me", loginBody.Token)
	if me.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d", me.Code, http.StatusOK)
	}
	if !strings.Contains(me.Body.String(), "demo") {
		t.Fatalf("me body = %q, want username demo", me.Body.String())
	}

	// A token minted for another server must not validate here.
	forger, _ := application.NewTokenService("rogue-secret-01234567890a")
	forged, _ := forger.Issue("demo")
	if rec := getWithToken(t, handler, "/v1/auth/me", forged); rec.Code != http.StatusUnauthorized {
		t.Fatalf("forged token status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	handler := newTestHandler()
	body := `{"username":"nobody","password":"abc12345"}`

	var last *httptest.ResponseRecorder
	for i := 0; i < 12; i++ {
		last = postRaw(t, handler, "/v1/auth/login", body)
		if i >= 10 {
			break
		}
		if last.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status = %d, want %d", i+1, last.Code, http.StatusUnauthorized)
		}
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 11: status = %d, want %d: %s", last.Code, http.StatusTooManyRequests, last.Body.String())
	}
	if last.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response is missing Retry-After")
	}
}

func TestHandlerCreatesWalletAndReturnsBalance(t *testing.T) {
	handler := newTestHandler()
	token := login(t, handler, "owner", "abc12345")

	createBody := `{"initial_balance_cents":1500}`
	createResponse := postWithTokenRaw(t, handler, "/v1/wallets", createBody, token)
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

	balanceResponse := getWithToken(t, handler, "/v1/wallets/"+created.ID+"/balance", token)
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

func TestWalletIsNotVisibleToAnotherUser(t *testing.T) {
	handler := newTestHandler()
	ownerToken := login(t, handler, "alice", "abc12345")
	strangerToken := login(t, handler, "bob", "abc12345")

	createResponse := postWithTokenRaw(t, handler, "/v1/wallets", `{"initial_balance_cents":1500}`, ownerToken)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create wallet status = %d, want %d: %s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	var created domain.Wallet
	_ = json.NewDecoder(createResponse.Body).Decode(&created)

	// Same wallet, different authenticated user: not found (no existence leak).
	rec := getWithToken(t, handler, "/v1/wallets/"+created.ID+"/balance", strangerToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("stranger balance status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// Creating a wallet for a foreign owner is refused.
	rec = postWithTokenRaw(t, handler, "/v1/wallets", `{"user_id":"alice","initial_balance_cents":1}`, strangerToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign owner create status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	// Sending to a foreign wallet is allowed (payments go to other people).
	bobWallet := postWithTokenRaw(t, handler, "/v1/wallets", `{"initial_balance_cents":10}`, strangerToken)
	var bob domain.Wallet
	_ = json.NewDecoder(bobWallet.Body).Decode(&bob)
	transfer := postWithTokenRaw(t, handler, "/v1/wallets/transfer",
		`{"from_wallet_id":"`+created.ID+`","to_wallet_id":"`+bob.ID+`","amount_cents":5}`, ownerToken)
	if transfer.Code != http.StatusOK {
		t.Fatalf("transfer to foreign wallet status = %d, want %d: %s", transfer.Code, http.StatusOK, transfer.Body.String())
	}

	// But sending from a foreign wallet is not.
	transfer = postWithTokenRaw(t, handler, "/v1/wallets/transfer",
		`{"from_wallet_id":"`+bob.ID+`","to_wallet_id":"`+created.ID+`","amount_cents":5}`, ownerToken)
	if transfer.Code != http.StatusNotFound {
		t.Fatalf("transfer from foreign wallet status = %d, want %d", transfer.Code, http.StatusNotFound)
	}
}

func TestHandlerRejectsInvalidCreateWalletPayload(t *testing.T) {
	handler := newTestHandler()
	token := login(t, handler, "owner", "abc12345")

	assertJSONStatus(t, handler, http.MethodPost, "/v1/wallets", `{"initial_balance_cents":-1}`, token, http.StatusBadRequest)
}

func TestHandlerRejectsUnknownJSONFields(t *testing.T) {
	handler := newTestHandler()
	token := login(t, handler, "owner", "abc12345")

	assertJSONStatus(t, handler, http.MethodPost, "/v1/wallets", `{"user_id":"owner","unexpected":true}`, token, http.StatusBadRequest)
}

func TestSimulateEndpointsRequireAuthAndStillWork(t *testing.T) {
	handler := newTestHandler()
	token := login(t, handler, "loader", "abc12345")

	seed := postWithTokenRaw(t, handler, "/v1/simulate/seed", `{"users":5,"initial_balance_cents":1000}`, token)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed status = %d, want %d: %s", seed.Code, http.StatusOK, seed.Body.String())
	}

	moved := postWithTokenRaw(t, handler, "/v1/simulate/transfer", `{"users":5,"amount_cents":10}`, token)
	if moved.Code != http.StatusOK {
		t.Fatalf("simulate transfer status = %d, want %d: %s", moved.Code, http.StatusOK, moved.Body.String())
	}
}

// ---------- helpers ----------

func newTestHandler() http.Handler {
	userRepo := memory.NewUserRepository()
	tokenSvc, err := application.NewTokenService("test-secret-0123456789")
	if err != nil {
		panic(err)
	}
	return httpadapter.NewHandler(
		application.NewWalletService(memory.NewWalletRepository()),
		application.NewUserService(userRepo),
		tokenSvc,
	).Routes()
}

func login(t *testing.T, handler http.Handler, username, password string) string {
	t.Helper()
	postJSON(t, handler, "/v1/auth/register", `{"username":"`+username+`","password":"`+password+`"}`)
	login := postJSON(t, handler, "/v1/auth/login", `{"username":"`+username+`","password":"`+password+`"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d: %s", login.Code, http.StatusOK, login.Body.String())
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(login.Body).Decode(&body); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	return body.Token
}

func postJSON(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return postRaw(t, handler, path, body)
}

func postRaw(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func postWithTokenRaw(t *testing.T, handler http.Handler, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func getWithToken(t *testing.T, handler http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func assertJSONStatus(t *testing.T, handler http.Handler, method, path, body, token string, wantStatus int) {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, rec.Code, wantStatus, rec.Body.String())
	}
}
