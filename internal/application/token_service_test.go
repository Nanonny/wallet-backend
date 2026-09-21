package application

import (
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	svc, err := NewTokenService("0123456789abcdef-FORBIDDEN-LENGTH")
	if err != nil {
		t.Fatalf("new token service: %v", err)
	}
	token, err := svc.Issue("demo_user")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	subject, err := svc.Validate(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if subject != "demo_user" {
		t.Fatalf("subject = %q, want demo_user", subject)
	}
}

func TestTokenRejectsWrongSecret(t *testing.T) {
	issuer, err := NewTokenService("issuer-secret-0123456789")
	if err != nil {
		t.Fatalf("issuer: %v", err)
	}
	token, err := issuer.Issue("demo")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	other, err := NewTokenService("other-secret-0123456789")
	if err != nil {
		t.Fatalf("validator: %v", err)
	}
	if _, err := other.Validate(token); err == nil {
		t.Fatal("token signed by a different secret must be rejected")
	}
}

func TestTokenExpiry(t *testing.T) {
	const secret = "clock-secret-0123456789"

	// Happy path: a token inside its TTL validates.
	fresh, err := NewTokenService(secret, WithTTL(1*time.Hour))
	if err != nil {
		t.Fatalf("new token service: %v", err)
	}
	token, err := fresh.Issue("demo")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := fresh.Validate(token); err != nil {
		t.Fatalf("fresh token should validate: %v", err)
	}

	// Expiry path: a token older than its TTL must be rejected.
	// (jwt/v5 checks exp against the real clock, so use a short real TTL.)
	short, err := NewTokenService(secret, WithTTL(20*time.Millisecond))
	if err != nil {
		t.Fatalf("new token service: %v", err)
	}
	expiring, err := short.Issue("demo")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := short.Validate(expiring); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

func TestTokenRejectsAlgorithmNone(t *testing.T) {
	svc, err := NewTokenService("alg-attack-secret-01")
	if err != nil {
		t.Fatalf("new token service: %v", err)
	}

	// Hand-built "alg=none" token with a valid-looking payload.
	token := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJkZW1vIiwiaXNzIjoid2FsbGV0LWFwaSJ9."
	if _, err := svc.Validate(token); err == nil {
		t.Fatal("alg=none token must be rejected")
	}
}

func TestTokenServiceRejectsShortSecret(t *testing.T) {
	if _, err := NewTokenService("tiny"); err == nil {
		t.Fatal("short secrets must be rejected at construction")
	}
}
