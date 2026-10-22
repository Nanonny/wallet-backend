package httpadapter

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/anon/wallet-devops-lab/internal/adapters/metrics"
	"github.com/anon/wallet-devops-lab/internal/application"
	"github.com/anon/wallet-devops-lab/internal/domain"
)

const (
	loginLimitPerWindow = 10
	loginWindow         = 15 * time.Minute
)

// register binds the account lifecycle endpoints.
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, err := h.userSvc.Register(r.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, domain.ErrUserExists):
		metrics.RecordAuth("register", "rejected")
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username is already taken"})
		return
	case errors.Is(err, domain.ErrInvalidUsername):
		metrics.RecordAuth("register", "rejected")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	case errors.Is(err, domain.ErrWeakPassword):
		metrics.RecordAuth("register", "rejected")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	case err != nil:
		metrics.RecordAuth("register", "error")
		writeInternalError(w, err)
		return
	}
	metrics.RecordAuth("register", "success")
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

// login verifies credentials and issues a short-lived bearer token.
// Failed attempts are rate limited per (client IP, username) to slow
// online brute-forcing; bcrypt cost 12 already raises the per-guess cost.
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	key := clientIP(r) + "|" + application.NormalizeUsername(req.Username)
	if !h.loginLimiter.allow(key) {
		metrics.RecordAuth("login", "rate_limited")
		w.Header().Set("Retry-After", "900")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, try again in 15 minutes"})
		return
	}

	user, err := h.userSvc.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		metrics.RecordAuth("login", "invalid_credentials")
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": domain.ErrInvalidCredentials.Error()})
		return
	}

	token, err := h.tokenSvc.Issue(user.Username)
	if err != nil {
		metrics.RecordAuth("login", "token_error")
		writeInternalError(w, err)
		return
	}
	metrics.RecordAuth("login", "success")
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": user})
}

// me returns the authenticated user for the current bearer token.
func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	username := application.SubjectFromContext(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"username": username})
}

// logout is a client-side convenience endpoint: sessions are stateless JWTs,
// so the server has nothing to revoke. The client discards its token.
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// requireAuth validates the Authorization bearer token and attaches the
// authenticated subject to the request context for the application layer.
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			metrics.RecordAuth("api_request", "unauthorized")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		subject, err := h.tokenSvc.Validate(token)
		if err != nil {
			metrics.RecordAuth("api_request", "unauthorized")
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
			return
		}
		next.ServeHTTP(w, r.WithContext(application.WithSubject(r.Context(), subject)))
	})
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return strings.TrimSpace(header[len(prefix):])
	}
	return ""
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// loginLimiter is a fixed-window in-memory rate limiter. It is per-process;
// with 3 replicas the effective limit is 3x the configured value, which is
// acceptable for a lab (Envoy local_ratelimit tightens it cluster-wide).
type loginLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]*loginAttempts
}

type loginAttempts struct {
	count int
	since time.Time
}

func newLoginLimiter(limit int, window time.Duration) *loginLimiter {
	return &loginLimiter{limit: limit, window: window, entries: map[string]*loginAttempts{}}
}

func (l *loginLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry, ok := l.entries[key]
	if !ok || now.Sub(entry.since) >= l.window {
		l.entries[key] = &loginAttempts{count: 1, since: now}
		l.prune(now)
		return true
	}
	entry.count++
	return entry.count <= l.limit
}

func (l *loginLimiter) prune(now time.Time) {
	for key, entry := range l.entries {
		if now.Sub(entry.since) >= l.window {
			delete(l.entries, key)
		}
	}
}
